# Copyright (c) 2018 Fox-IT Security Research Team <srt@fox-it.com>
#
# Permission is hereby granted, free of charge, to any person obtaining a copy
# of this software and associated documentation files (the "Software"), to deal
# in the Software without restriction, including without limitation the rights
# to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
# copies of the Software, and to permit persons to whom the Software is
# furnished to do so, subject to the following conditions:
#
# The above copyright notice and this permission notice shall be included in all
# copies or substantial portions of the Software.
#
# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
# FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
# AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
# LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
# OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
# SOFTWARE.

# TODO:
# - Rework definition parsing, maybe pycparser?
# - Change expression implementation
# - Lazy reading?
import re
import sys
import ast
import pprint
import string
import struct
import ctypes as _ctypes
from io import BytesIO
from collections import OrderedDict

try:
    from builtins import bytes as newbytes
except ImportError:
    newbytes = bytes

PY3 = sys.version_info > (3,)
if PY3:
    long = int
    xrange = range

DEBUG = False

COLOR_RED = '\033[1;31m'
COLOR_GREEN = '\033[1;32m'
COLOR_YELLOW = '\033[1;33m'
COLOR_BLUE = '\033[1;34m'
COLOR_PURPLE = '\033[1;35m'
COLOR_CYAN = '\033[1;36m'
COLOR_WHITE = '\033[1;37m'
COLOR_NORMAL = '\033[1;0m'

COLOR_BG_RED = '\033[1;41m\033[1;37m'
COLOR_BG_GREEN = '\033[1;42m\033[1;37m'
COLOR_BG_YELLOW = '\033[1;43m\033[1;37m'
COLOR_BG_BLUE = '\033[1;44m\033[1;37m'
COLOR_BG_PURPLE = '\033[1;45m\033[1;37m'
COLOR_BG_CYAN = '\033[1;46m\033[1;37m'
COLOR_BG_WHITE = '\033[1;47m\033[1;30m'

PRINTABLE = string.digits + string.ascii_letters + string.punctuation + " "

COMPILE_TEMPL = """
class {name}(Structure):
    def __init__(self, cstruct, structure, source=None):
        self.structure = structure
        self.source = source
        super({name}, self).__init__(cstruct, structure.name, structure.fields)

    def _read(self, stream):
        r = OrderedDict()
        sizes = {{}}
        bitreader = BitBuffer(stream, self.cstruct.endian)

{read_code}

        return Instance(self, r, sizes)

    def add_fields(self, name, type_, offset=None):
        raise NotImplementedError("Can't add fields to a compiled structure")

    def __repr__(self):
        return '<Structure {name} +compiled>'
"""


class Error(Exception):
    pass


class ParserError(Error):
    pass


class CompilerError(Error):
    pass


class ResolveError(Error):
    pass


class NullPointerDereference(Error):
    pass


def log(line, *args, **kwargs):
    if not DEBUG:
        return

    print(line.format(*args, **kwargs), file=sys.stderr)


class cstruct(object):
    """Main class of cstruct. All types are registered in here.

    Args:
        endian: The endianness to use when parsing.
        pointer: The pointer type to use for Pointers.
    """

    DEF_CSTYLE = 1

    def __init__(self, endian='<', pointer='uint64'):
        self.endian = endian

        self.consts = {}
        self.lookups = {}
        self.typedefs = {
            'byte': 'int8',
            'ubyte': 'uint8',
            'uchar': 'uint8',
            'short': 'int16',
            'ushort': 'uint16',
            'long': 'int32',
            'ulong': 'uint32',
            'ulong64': 'uint64',

            'u1': 'uint8',
            'u2': 'uint16',
            'u4': 'uint32',
            'u8': 'uint64',

            'word': 'uint16',
            'dword': 'uint32',

            'longlong': 'int64',
            'ulonglong': 'uint64',

            'int': 'int32',
            'unsigned int': 'uint32',

            'int8': PackedType(self, 'int8', 1, 'b'),
            'uint8': PackedType(self, 'uint8', 1, 'B'),
            'int16': PackedType(self, 'int16', 2, 'h'),
            'uint16': PackedType(self, 'uint16', 2, 'H'),
            'int32': PackedType(self, 'int32', 4, 'i'),
            'uint32': PackedType(self, 'uint32', 4, 'I'),
            'int64': PackedType(self, 'int64', 8, 'q'),
            'uint64': PackedType(self, 'uint64', 8, 'Q'),
            'float': PackedType(self, 'float', 4, 'f'),
            'double': PackedType(self, 'double', 8, 'd'),
            'char': CharType(self),
            'wchar': WcharType(self),

            'int24': BytesInteger(self, 'int24', 3, True),
            'uint24': BytesInteger(self, 'uint24', 3, False),
            'int48': BytesInteger(self, 'int48', 6, True),
            'uint48': BytesInteger(self, 'uint48', 6, False),

            'void': VoidType(),
        }

        self.pointer = self.resolve(pointer)

    def addtype(self, name, t, replace=False):
        """Add a type or type reference.

        Args:
            name: Name of the type to be added.
            t: The type to be added. Can be a str reference to another type
                or a compatible type class.

        Raises:
            ValueError: If the type already exists.
        """
        name = name.lower()
        if not replace and name.lower() in self.typedefs:
            raise ValueError("Duplicate type: %s" % name)

        self.typedefs[name] = t

    def load(self, s, deftype=None, **kwargs):
        """Parse structures from the given definitions using the given definition type.

        Definitions can be parsed using different parsers. Currently, there's
        only one supported parser - DEF_CSTYLE. Parsers can add types and
        modify this cstruct instance. Arguments can be passed to parsers
        using kwargs.

        Args:
            s: The definition to parse.
            deftype: The definition type to parse the definitions with.
            **kwargs: Keyword arguments for parsers.
        """
        deftype = deftype or cstruct.DEF_CSTYLE

        if deftype == cstruct.DEF_CSTYLE:
            parser = CStyleParser(self, **kwargs)

        parser.parse(s)

    def loadfile(self, s, deftype=None, **kwargs):
        """Load structure definitions from a file.

        The given path will be read and parsed using the .load() function.

        Args:
            s: The path to load definitions from.
            deftype: The definition type to parse the definitions with.
            **kwargs: Keyword arguments for parsers.
        """
        with open(s, 'r') as fh:
            self.load(fh.read(), deftype, **kwargs)

    def read(self, name, s):
        """Parse data using a given type.

        Args:
            name: Type name to read.
            s: File-like object or byte string to parse.

        Returns:
            The parsed data.
        """
        return self.resolve(name).read(s)

    def resolve(self, name):
        """Resolve a type name to get the actual type object.

        Types can be referenced using different names. When we want
        the actual type object, we need to resolve these references.

        Args:
            name: Type name to resolve.

        Returns:
            The resolved type object.

        Raises:
            ResolveError: If the type can't be resolved.
        """
        t = name
        if not isinstance(t, str):
            return t

        for i in xrange(10):
            if t.lower() not in self.typedefs:
                raise ResolveError("Unknown type %s" % name)

            t = self.typedefs[t.lower()]

            if not isinstance(t, str):
                return t

        raise ResolveError("Recursion limit exceeded while resolving type %s" % name)

    def __getattr__(self, attr):
        if attr.lower() in self.typedefs:
            return self.typedefs[attr.lower()]

        if attr in self.consts:
            return self.consts[attr]

        raise AttributeError("Invalid Attribute: %s" % attr)


class Parser(object):
    """Base class for definition parsers.

    Args:
        cstruct: An instance of cstruct.
    """

    def __init__(self, cstruct):
        self.cstruct = cstruct

    def parse(self, data):
        """This function should parse definitions to cstruct types.

        Args:
            data: Data to parse definitions from, usually a string.
        """
        raise NotImplementedError()


class CStyleParser(Parser):
    """Definition parser for C-like structure syntax.

    Args:
        cstruct: An instance of cstruct
        compiled: Whether structs should be compiled or not.
    """

    def __init__(self, cstruct, compiled=True):
        self.compiled = compiled
        super(CStyleParser, self).__init__(cstruct)

    # TODO: Implement proper parsing
    def parse(self, data):
        self._constants(data)
        self._enums(data)
        self._structs(data)
        self._lookups(data, self.cstruct.consts)

    def _constants(self, data):
        r = re.finditer(r'#define\s+(?P<name>[^\s]+)\s+(?P<value>[^\r\n]+)\s*\n', data)
        for t in r:
            d = t.groupdict()
            v = d['value'].rsplit('//')[0]

            try:
                v = ast.literal_eval(v)
            except (ValueError, SyntaxError):
                pass

            self.cstruct.consts[d['name']] = v

    def _enums(self, data):
        r = re.finditer(
            r'enum\s+(?P<name>[^\s:{]+)\s*(:\s*(?P<type>[^\s]+)\s*)?\{(?P<values>[^}]+)\}\s*;',
            data,
        )
        for t in r:
            d = t.groupdict()

            nextval = 0
            values = {}
            for line in d['values'].split('\n'):
                line, sep, comment = line.partition("//")
                for v in line.split(","):
                    key, sep, val = v.partition("=")
                    key = key.strip()
                    val = val.strip()
                    if not key:
                        continue
                    if not val:
                        val = nextval
                    else:
                        val = Expression(self.cstruct, val).evaluate({})

                    nextval = val + 1

                    values[key] = val

            if not d['type']:
                d['type'] = 'uint32'

            enum = Enum(
                self.cstruct, d['name'], self.cstruct.resolve(d['type']), values
            )
            self.cstruct.addtype(enum.name, enum)

    def _structs(self, data):
        compiler = Compiler(self.cstruct)
        r = re.finditer(
            r'(#(?P<flags>(?:compile))\s+)?((?P<typedef>typedef)\s+)?(?P<type>[^\s]+)\s+(?P<name>[^\s]+)?(?P<fields>\s*\{[^}]+\}(?P<defs>\s+[^;\n]+)?)?\s*;',
            data,
        )
        for t in r:
            d = t.groupdict()

            if d['name']:
                name = d['name']
            elif d['defs']:
                name = d['defs'].strip().split(',')[0].strip()
            else:
                raise ParserError("No name for struct")

            if d['type'] == 'struct':
                data = self._parse_fields(d['fields'][1:-1].strip())
                st = Structure(self.cstruct, name, data)
                if d['flags'] == 'compile' or self.compiled:
                    st = compiler.compile(st)
            elif d['typedef'] == 'typedef':
                st = d['type']
            else:
                continue

            if d['name']:
                self.cstruct.addtype(d['name'], st)

            if d['defs']:
                for td in d['defs'].strip().split(','):
                    td = td.strip()
                    self.cstruct.addtype(td, st)

    def _parse_fields(self, s):
        fields = re.finditer(
            r'(?P<type>[^\s]+)\s+(?P<name>[^\s\[:]+)(\s*:\s*(?P<bits>\d+))?(\[(?P<count>[^;\n]*)\])?;',
            s,
        )
        r = []
        for f in fields:
            d = f.groupdict()
            if d['type'].startswith('//'):
                continue

            type_ = self.cstruct.resolve(d['type'])

            d['name'] = d['name'].replace('(', '').replace(')', '')

            # Maybe reimplement lazy type references later
            # _type = TypeReference(self, d['type'])
            if d['count'] is not None:
                if d['count'] == '':
                    count = None
                else:
                    count = Expression(self.cstruct, d['count'])
                    try:
                        count = count.evaluate()
                    except Exception:
                        pass

                type_ = Array(self.cstruct, type_, count)

            if d['name'].startswith('*'):
                d['name'] = d['name'][1:]
                type_ = Pointer(self.cstruct, type_)

            field = Field(d['name'], type_, int(d['bits']) if d['bits'] else None)
            r.append(field)

        return r

    def _lookups(self, data, consts):
        r = re.finditer(r'\$(?P<name>[^\s]+) = ({[^}]+})\w*\n', data)

        for t in r:
            d = ast.literal_eval(t.group(2))
            self.cstruct.lookups[t.group(1)] = dict(
                [(self.cstruct.consts[k], v) for k, v in d.items()]
            )


class Instance(object):
    """Holds parsed structure data."""

    def __init__(self, type_, values, sizes=None):
        object.__setattr__(self, '_type', type_)
        object.__setattr__(self, '_values', values)
        object.__setattr__(self, '_sizes', sizes)

    def write(self, fh):
        """Write this structure to a writable file-like object.

        Args:
            fh: File-like objects that supports writing.

        Returns:
            The amount of bytes written.
        """
        return self.__dict__['_type'].write(fh, self)

    def dumps(self):
        """Dump this structure to a byte string.

        Returns:
            The raw bytes of this structure.
        """
        s = BytesIO()
        self.write(s)
        return s.getvalue()

    def __getattr__(self, attr):
        if attr not in self.__dict__['_type'].lookup:
            raise AttributeError("Invalid attribute: %r" % attr)

        return self.__dict__['_values'][attr]

    def __setattr__(self, attr, value):
        if attr not in self.__dict__['_type'].lookup:
            raise AttributeError("Invalid attribute: %r" % attr)

        self.__dict__['_values'][attr] = value

    def __getitem__(self, item):
        return self.__dict__['_values'][item]

    def __contains__(self, attr):
        return attr in self.__dict__['_values']

    def __repr__(self):
        return '<%s %s>' % (
            self.__dict__['_type'].name,
            ', '.join(
                [
                    '%s=%s' % (k, hex(v) if isinstance(v, (int, long)) else repr(v))
                    for k, v in self.__dict__['_values'].items()
                ]
            ),
        )

    def __len__(self):
        return len(self.dumps())

    def _size(self, field):
        return self.__dict__['_sizes'][field]


class PointerInstance(object):
    """Like the Instance class, but for structures referenced by a pointer."""

    def __init__(self, t, stream, addr, ctx):
        self._stream = stream
        self._type = t
        self._addr = addr
        self._ctx = ctx
        self._value = None

    def _get(self):
        log("Dereferencing pointer -> 0x{:016x} [{!r}]", self._addr, self._stream)
        if self._addr == 0:
            raise NullPointerDereference()

        if self._value is None:
            pos = self._stream.tell()
            self._stream.seek(self._addr)
            if isinstance(self._type, Array):
                r = self._type._read(self._stream, self._ctx)
            else:
                r = self._type._read(self._stream)
            self._stream.seek(pos)
            self._value = r

        return self._value

    def __getattr__(self, attr):
        return getattr(self._get(), attr)

    def __str__(self):
        return str(self._get())

    def __nonzero__(self):
        return self._addr != 0

    def __repr__(self):
        return "<Pointer {!r} @ 0x{:x}>".format(self._type, self._addr)


class Expression(object):
    """Expression parser for simple calculations in definitions."""

    operators = [
        ('+', lambda a, b: a + b),
        ('-', lambda a, b: a - b),
        ('*', lambda a, b: a * b),
        ('/', lambda a, b: a / b),
        ('&', lambda a, b: a & b),
        ('|', lambda a, b: a | b),
        ('>>', lambda a, b: a >> b),
        ('<<', lambda a, b: a << b),
    ]

    def __init__(self, cstruct, expr):
        self.cstruct = cstruct
        self.expr = expr

    def evaluate(self, context=None):
        context = context if context else {}
        level = 0
        levels = []
        buf = ''

        for i in xrange(len(self.expr)):
            if self.expr[i] == '(':
                level += 1
                levels.append(buf)
                buf = ''
                continue

            if self.expr[i] == ')':
                level -= 1
                val = self.evaluate_part(buf, context)
                buf = levels.pop()
                buf += str(val)
                continue

            buf += self.expr[i]

        return self.evaluate_part(buf, context)

    def evaluate_part(self, e, v):
        e = e.strip()

        for o in self.operators:
            if o[0] in e:
                a, b = e.rsplit(o[0], 1)
                return o[1](self.evaluate_part(a, v), self.evaluate_part(b, v))

        if e in v:
            return v[e]

        if e.startswith('0x'):
            return int(e, 16)

        if e in self.cstruct.consts:
            return self.cstruct.consts[e]

        return int(e)

    def __repr__(self):
        return self.expr


class BaseType(object):
    """Base class for cstruct type classes."""

    def __init__(self, cstruct):
        self.cstruct = cstruct

    def reads(self, data):
        """Parse the given data according to the type that implements this class.

        Args:
            data: Byte string to parse.

        Returns:
            The parsed value of this type.
        """
        data = BytesIO(data)
        return self._read(data)

    def dumps(self, data):
        """Dump the given data according to the type that implements this class.

        Args:
            data: Data to dump.

        Returns:
            The resulting bytes.
        """
        out = BytesIO()
        self._write(out, data)
        return out.getvalue()

    def read(self, obj, *args, **kwargs):
        """Parse the given data according to the type that implements this class.

        Args:
            obj: Data to parse. Can be a (byte) string or a file-like object.

        Returns:
            The parsed value of this type.
        """
        if isinstance(obj, (str, bytes, newbytes)):
            return self.reads(obj)

        return self._read(obj)

    def write(self, stream, data):
        """Write the given data to a writable file-like object according to the
        type that implements this class.

        Args:
            stream: Writable file-like object to write to.
            data: Data to write.

        Returns:
            The amount of bytes written.
        """
        return self._write(stream, data)

    def _read(self, stream):
        raise NotImplementedError()

    def _read_array(self, stream, count):
        return [self._read(stream) for i in xrange(count)]

    def _read_0(self, stream):
        raise NotImplementedError()

    def _write(self, stream, data):
        raise NotImplementedError()

    def _write_array(self, stream, data):
        num = 0
        for i in data:
            num += self._write(stream, i)
        return num

    def _write_0(self, stream, data):
        raise NotImplementedError()

    def default(self):
        """Return a default value of this type."""
        raise NotImplementedError()

    def default_array(self):
        """Return a default array of this type."""
        raise NotImplementedError()

    def __getitem__(self, count):
        return Array(self.cstruct, self, count)

    def __call__(self, *args, **kwargs):
        if len(args) > 0:
            return self.read(*args, **kwargs)

        r = self.default()
        if kwargs:
            for k, v in kwargs.items():
                setattr(r, k, v)

        return r


class RawType(BaseType):
    """Base class for raw types that have a name and size."""

    def __init__(self, cstruct, name=None, size=0):
        self.name = name
        self.size = size
        super(RawType, self).__init__(cstruct)

    def __len__(self):
        return self.size

    def __repr__(self):
        if self.name:
            return self.name

        return BaseType.__repr__(self)


class Structure(BaseType):
    """Type class for structures."""

    def __init__(self, cstruct, name, fields=None):
        self.name = name
        self.size = None
        self.lookup = OrderedDict()
        self.fields = fields if fields else []

        for f in self.fields:
            self.lookup[f.name] = f

        self._calc_offsets()
        super(Structure, self).__init__(cstruct)

    def _calc_offsets(self):
        offset = 0
        bitstype = None
        bitsremaining = 0

        for field in self.fields:
            if field.bits:
                if bitsremaining == 0 or field.type != bitstype:
                    bitstype = field.type
                    bitsremaining = bitstype.size * 8
                    if offset is not None:
                        field.offset = offset
                        offset += bitstype.size
                else:
                    field.offset = None

                bitsremaining -= field.bits
                continue

            field.offset = offset
            if offset is not None:
                try:
                    offset += len(field.type)
                except TypeError:
                    offset = None

    def _calc_size(self):
        size = 0
        bitstype = None
        bitsremaining = 0

        for field in self.fields:
            if field.bits:
                if bitsremaining == 0 or field.type != bitstype:
                    bitstype = field.type
                    bitsremaining = bitstype.size * 8
                    size += bitstype.size

                bitsremaining -= field.bits
                continue

            fieldlen = len(field.type)
            size += fieldlen

            if field.offset is not None:
                size = max(size, field.offset + fieldlen)

        return size

    def _read(self, stream, *args, **kwargs):
        log("[Structure::read] {} {}", self.name, self.size)
        bitbuffer = BitBuffer(stream, self.cstruct.endian)

        struct_start = stream.tell()

        r = OrderedDict()
        sizes = {}
        for field in self.fields:
            start = stream.tell()
            ft = self.cstruct.resolve(field.type)

            if field.offset:
                if start != struct_start + field.offset:
                    log(
                        "+ seeking to 0x{:x}+0x{:x} for {}".format(
                            struct_start, field.offset, field.name
                        )
                    )
                    stream.seek(struct_start + field.offset)
                    start = struct_start + field.offset

            if field.bits:
                r[field.name] = bitbuffer.read(ft, field.bits)
                continue
            else:
                bitbuffer.reset()

            if isinstance(ft, (Array, Pointer)):
                v = ft._read(stream, r)
            else:
                v = ft._read(stream)

            sizes[field.name] = stream.tell() - start
            r[field.name] = v

        return Instance(self, r, sizes)

    def _write(self, stream, data):
        bitbuffer = BitBuffer(stream, self.cstruct.endian)
        num = 0

        for field in self.fields:
            if field.bits:
                bitbuffer.write(field.type, getattr(data, field.name), field.bits)
                continue

            if bitbuffer._type:
                bitbuffer.flush()

            num += field.type._write(stream, getattr(data, field.name))

        # Flush bitbuffer
        if bitbuffer._type:
            bitbuffer.flush()

        return num

    def add_field(self, name, type_, offset=None):
        """Add a field to this structure.

        Args:
            name: The field name.
            type_: The field type.
            offset: The field offset.
        """
        field = Field(name, type_, offset=offset)
        self.fields.append(field)
        self.lookup[name] = field
        self.size = None
        setattr(self, name, field)

    def default(self):
        """Create and return an empty Instance from this structure.

        Returns:
            An empty Instance from this structure.
        """
        r = OrderedDict()
        for field in self.fields:
            r[field.name] = field.type.default()

        return Instance(self, r)

    def __len__(self):
        if self.size is None:
            self.size = self._calc_size()

        return self.size

    def __repr__(self):
        return '<Structure {}>'.format(self.name)

    def show(self, indent=0):
        """Pretty print this structure."""
        if indent == 0:
            print("struct {}".format(self.name))

        for field in self.fields:
            if field.offset is None:
                offset = '0x??'
            else:
                offset = '0x{:02x}'.format(field.offset)

            print("{}+{} {} {}".format(' ' * indent, offset, field.name, field.type))

            if isinstance(field.type, Structure):
                field.type.show(indent + 1)


class BitBuffer(object):
    """Implements a bit buffer that can read and write bit fields."""

    def __init__(self, stream, endian):
        self.stream = stream
        self.endian = endian

        self._type = None
        self._buffer = 0
        self._remaining = 0

    def read(self, field_type, bits):
        if self._remaining < 1 or self._type != field_type:
            self._type = field_type
            self._remaining = field_type.size * 8
            self._buffer = field_type._read(self.stream)

        if self.endian != '>':
            v = self._buffer & ((1 << bits) - 1)
            self._buffer >>= bits
            self._remaining -= bits
        else:
            v = self._buffer & (
                ((1 << (self._remaining - bits)) - 1) ^ ((1 << self._remaining) - 1)
            )
            v >>= self._remaining - bits
            self._remaining -= bits

        return v

    def write(self, field_type, data, bits):
        if self._remaining == 0:
            self._remaining = field_type.size * 8
            self._type = field_type

        if self.endian != '>':
            self._buffer |= data << (self._type.size * 8 - self._remaining)
        else:
            self._buffer |= data << (self._remaining - bits)

        self._remaining -= bits

    def flush(self):
        self._type._write(self.stream, self._buffer)
        self._type = None
        self._remaining = 0
        self._buffer = 0

    def reset(self):
        self._type = None
        self._buffer = 0
        self._remaining = 0


class Field(object):
    """Holds a structure field."""

    def __init__(self, name, type_, bits=None, offset=None):
        self.name = name
        self.type = type_
        self.bits = bits
        self.offset = offset

    def __repr__(self):
        return '<Field {} {}>'.format(self.name, self.type)


class Array(BaseType):
    """Implements a fixed or dynamically sized array type.

    Example:
        When using the default C-style parser, the following syntax is supported:

            x[3] -> 3 -> static length.
            x[] -> None -> null-terminated.
            x[expr] -> expr -> dynamic length.
    """

    def __init__(self, cstruct, type_, count):
        self.type = type_
        self.count = count
        self.dynamic = isinstance(self.count, Expression) or self.count is None

        super(Array, self).__init__(cstruct)

    def _read(self, stream, context=None):
        if self.count is None:
            return self.type._read_0(stream)

        if self.dynamic:
            count = self.count.evaluate(context)
        else:
            count = self.count

        return self.type._read_array(stream, max(0, count))

    def _write(self, f, data):
        if self.count is None:
            return self.type._write_0(f, data)

        return self.type._write_array(f, data)

    def default(self):
        if self.dynamic or self.count is None:
            return []

        return [self.type.default() for i in xrange(self.count)]

    def __repr__(self):
        if self.count is None:
            return '{0!r}[]'.format(self.type)

        return '{0!r}[{1}]'.format(self.type, self.count)

    def __len__(self):
        if self.dynamic:
            raise TypeError("Dynamic size")

        return len(self.type) * self.count


class PackedType(RawType):
    """Implements a packed type that uses Python struct packing characters."""

    def __init__(self, cstruct, name, size, packchar):
        self.packchar = packchar
        super(PackedType, self).__init__(cstruct, name, size)

    def _read(self, stream):
        return self._read_array(stream, 1)[0]

    def _read_array(self, stream, count):
        length = self.size * count
        data = stream.read(length)
        fmt = self.cstruct.endian + str(count) + self.packchar
        if len(data) != length:
            raise EOFError("Read %d bytes, but expected %d" % (len(data), length))

        return list(struct.unpack(fmt, data))

    def _read_0(self, stream):
        r = []
        while True:
            d = stream.read(self.size)
            v = struct.unpack(self.cstruct.endian + self.packchar, d)[0]

            if v == 0:
                break

            r.append(v)

        return r

    def _write(self, stream, data):
        return self._write_array(stream, [data])

    def _write_array(self, stream, data):
        fmt = self.cstruct.endian + str(len(data)) + self.packchar
        return stream.write(struct.pack(fmt, *data))

    def _write_0(self, stream, data):
        return self._write_array(stream, data + [0])

    def default(self):
        return 0

    def default_array(self, count):
        return [0] * count


class CharType(RawType):
    """Implements a character type that can properly handle strings."""

    def __init__(self, cstruct):
        super(CharType, self).__init__(cstruct, 'char', 1)

    def _read(self, stream):
        return stream.read(1)

    def _read_array(self, stream, count):
        if count == 0:
            return b''

        return stream.read(count)

    def _read_0(self, stream):
        r = []
        while True:
            c = stream.read(1)
            if c == b'':
                raise EOFError()

            if c == b'\x00':
                break

            r.append(c)

        return b''.join(r)

    def _write(self, stream, data):
        if isinstance(data, int):
            data = chr(data)

        if PY3 and isinstance(data, str):
            data = data.encode('latin-1')

        return stream.write(data)

    def _write_array(self, stream, data):
        return self._write(stream, data)

    def _write_0(self, stream, data):
        return self._write(stream, data + b'\x00')

    def default(self):
        return b'\x00'

    def default_array(self, count):
        return b'\x00' * count


class WcharType(RawType):
    """Implements a wide-character type."""

    def __init__(self, cstruct):
        super(WcharType, self).__init__(cstruct, 'wchar', 2)

    @property
    def encoding(self):
        if self.cstruct.endian == '<':
            return 'utf-16-le'
        elif self.cstruct.endian == '>':
            return 'utf-16-be'

    def _read(self, stream):
        return stream.read(2).decode(self.encoding)

    def _read_array(self, stream, count):
        if count == 0:
            return u''

        data = stream.read(2 * count)
        return data.decode(self.encoding)

    def _read_0(self, stream):
        r = b''
        while True:
            c = stream.read(2)

            if len(c) != 2:
                raise EOFError()

            if c == b'\x00\x00':
                break

            r += c

        return r.decode(self.encoding)

    def _write(self, stream, data):
        return stream.write(data.encode(self.encoding))

    def _write_array(self, stream, data):
        return self._write(stream, data)

    def _write_0(self, stream, data):
        return self._write(stream, data + u'\x00')

    def default(self):
        return u'\x00'

    def default_array(self, count):
        return u'\x00' * count


class BytesInteger(RawType):
    """Implements an integer type that can span an arbitrary amount of bytes."""

    def __init__(self, cstruct, name, size, signed):
        self.signed = signed
        super(BytesInteger, self).__init__(cstruct, name, size)

    @staticmethod
    def parse(buf, size, count, signed, endian):
        nums = []

        for c in xrange(count):
            num = 0
            data = buf[c * size:(c + 1) * size]
            if endian == '<':
                data = b''.join(data[i:i + 1] for i in reversed(xrange(len(data))))

            ints = list(data) if PY3 else map(ord, data)
            for i in ints:
                num = (num << 8) | i

            if signed and num & 1 << (size * 8 - 1):
                bias = 1 << (size * 8 - 1)
                num -= bias * 2

            nums.append(num)

        return nums

    @staticmethod
    def pack(data, size, endian):
        buf = []
        for i in data:
            num = int(i)
            if num < 0:
                num += 1 << (size * 8)

            d = [b'\x00'] * size
            i = size - 1

            while i >= 0:
                b = num & 255
                d[i] = bytes((b,)) if PY3 else chr(b)
                num >>= 8
                i -= 1

            if endian == '<':
                d = b''.join(d[i:i + 1][0] for i in reversed(xrange(len(d))))
            else:
                d = b''.join(d)

            buf.append(d)

        return b''.join(buf)

    def _read(self, stream):
        return self.parse(stream.read(self.size * 1), self.size, 1, self.signed, self.cstruct.endian)[0]

    def _read_array(self, stream, count):
        return self.parse(stream.read(self.size * count), self.size, count, self.signed, self.cstruct.endian)

    def _read_0(self, stream):
        r = []
        while True:
            v = self._read(stream)
            if v == 0:
                break
            r.append(v)

        return r

    def _write(self, stream, data):
        return stream.write(self.pack([data], self.size, self.cstruct.endian))

    def _write_array(self, stream, data):
        return stream.write(self.pack(data, self.size, self.cstruct.endian))

    def _write_0(self, stream, data):
        return self._write_array(stream, data + [0])

    def default(self):
        return 0

    def default_array(self, count):
        return [0] * count


class Enum(RawType):
    """Implements an Enum type.

    Enums can be made using any type. The API for accessing enums and their
    values is very similar to Python 3 native enums.

    Example:
        When using the default C-style parser, the following syntax is supported:

            enum <name> [: <type>] {
                <values>
            };

        For example, an enum that has A=1, B=5 and C=6 could be written like so:

            enum Test : uint16 {
                A, B=5, C
            };
    """

    def __init__(self, cstruct, name, type_, values):
        self.type = type_
        self.values = values
        self.reverse = {}

        for k, v in values.items():
            self.reverse[v] = k

        super(Enum, self).__init__(cstruct, name, len(self.type))

    def __call__(self, value):
        return EnumInstance(self, value)

    def _read(self, stream):
        v = self.type._read(stream)
        return self(v)

    def _read_array(self, stream, count):
        return list(map(self, self.type._read_array(stream, count)))

    def _read_0(self, stream):
        return list(map(self, self.type._read_0(stream)))

    def _write(self, stream, data):
        data = data.value if isinstance(data, EnumInstance) else data
        return self.type._write(stream, data)

    def _write_array(self, stream, data):
        data = [d.value if isinstance(d, EnumInstance) else d for d in data]
        return self.type._write_array(stream, data)

    def _write_0(self, stream, data):
        data = [d.value if isinstance(d, EnumInstance) else d for d in data]
        return self.type._write_0(stream, data)

    def default(self):
        return self(0)

    def __getitem__(self, attr):
        if attr in self.values:
            return self(self.values[attr])

        raise KeyError(attr)

    def __getattr__(self, attr):
        if attr in self.values:
            return self(self.values[attr])

        raise AttributeError(attr)

    def __contains__(self, attr):
        return attr in self.values


class EnumInstance(object):
    """Implements a value instance of an Enum"""

    def __init__(self, enum, value):
        self.enum = enum
        self.value = value

    @property
    def name(self):
        if self.value not in self.enum.reverse:
            return '{}_{}'.format(self.enum.name, self.value)
        return self.enum.reverse[self.value]

    def __eq__(self, value):
        if isinstance(value, EnumInstance) and value.enum is not self.enum:
            return False

        if hasattr(value, 'value'):
            value = value.value

        return self.value == value

    def __ne__(self, value):
        return self.__eq__(value) is False

    def __hash__(self):
        return hash((self.enum, self.value))

    def __str__(self):
        return '{}.{}'.format(self.enum.name, self.name)

    def __repr__(self):
        return '<{}.{}: {}>'.format(self.enum.name, self.name, self.value)


class Union(RawType):
    def __init__(self, cstruct):
        self.cstruct = cstruct
        super(Union, self).__init__(cstruct)

    def _read(self, stream):
        raise NotImplementedError()


class Pointer(RawType):
    """Implements a pointer to some other type."""

    def __init__(self, cstruct, target):
        self.cstruct = cstruct
        self.type = target
        super(Pointer, self).__init__(cstruct)

    def _read(self, stream, ctx):
        addr = self.cstruct.pointer(stream)
        return PointerInstance(self.type, stream, addr, ctx)

    def __len__(self):
        return len(self.cstruct.pointer)

    def __repr__(self):
        return '<Pointer {!r}>'.format(self.type)


class VoidType(RawType):
    """Implements a void type."""

    def __init__(self):
        super(VoidType, self).__init__(None, 'void', 0)

    def _read(self, stream):
        return None


def ctypes(structure):
    """Create ctypes structures from cstruct structures."""
    fields = []
    for field in structure.fields:
        t = ctypes_type(field.type)
        fields.append((field.name, t))

    tt = type(structure.name, (_ctypes.Structure, ), {"_fields_": fields})
    return tt


def ctypes_type(t):
    mapping = {
        "I": _ctypes.c_ulong,
        "i": _ctypes.c_long,
        "b": _ctypes.c_int8,
    }

    if isinstance(t, PackedType):
        return mapping[t.packchar]

    if isinstance(t, CharType):
        return _ctypes.c_char

    if isinstance(t, Array):
        subtype = ctypes_type(t._type)
        return subtype * t.count

    if isinstance(t, Pointer):
        subtype = ctypes_type(t._target)
        return ctypes.POINTER(subtype)

    raise NotImplementedError("Type not implemented: %s" % t.__class__.__name__)


class Compiler(object):
    """Compiler for cstruct structures. Creates somewhat optimized parsing code."""

    def __init__(self, cstruct):
        self.cstruct = cstruct

    def compile(self, structure):
        source = self.gen_struct_class(structure)
        c = compile(source, '<compiled>', 'exec')

        env = {
            'OrderedDict': OrderedDict,
            'Structure': Structure,
            'Instance': Instance,
            'Expression': Expression,
            'EnumInstance': EnumInstance,
            'PointerInstance': PointerInstance,
            'BytesInteger': BytesInteger,
            'BitBuffer': BitBuffer,
            'struct': struct,
            'xrange': xrange,
        }

        exec(c, env)
        sc = env[structure.name](self.cstruct, structure, source)

        return sc

    def gen_struct_class(self, structure):
        blocks = []
        classes = []
        cur_block = []
        read_size = 0
        prev_was_bits = False

        for field in structure.fields:
            ft = self.cstruct.resolve(field.type)

            if not isinstance(
                ft,
                (
                    Structure,
                    Pointer,
                    Enum,
                    Array,
                    PackedType,
                    CharType,
                    WcharType,
                    BytesInteger,
                ),
            ):
                raise CompilerError("Unsupported type for compiler: {}".format(ft))

            if isinstance(ft, Structure) or (
                isinstance(ft, Array) and isinstance(ft.type, Structure)
            ):
                if cur_block:
                    blocks.append(self.gen_read_block(read_size, cur_block))

                struct_read = 's = stream.tell()\n'
                if isinstance(ft, Array):
                    num = ft.count

                    if isinstance(num, Expression):
                        num = 'max(0, Expression(self.cstruct, "{expr}").evaluate(r))'.format(
                            expr=num.expr
                        )

                    struct_read += (
                        'r["{name}"] = []\n'
                        'for _ in xrange({num}):\n'
                        '    r["{name}"].append(self.cstruct.{struct_name}._read(stream))\n'.format(
                            name=field.name, num=num, struct_name=ft.type.name
                        )
                    )
                else:
                    struct_read += 'r["{name}"] = self.cstruct.{struct_name}._read(stream)\n'.format(
                        name=field.name, struct_name=ft.name
                    )

                struct_read += 'sizes["{name}"] = stream.tell() - s'.format(
                    name=field.name
                )
                blocks.append(struct_read)
                read_size = 0
                cur_block = []
                continue

            if field.bits:
                if cur_block:
                    blocks.append(self.gen_read_block(read_size, cur_block))

                blocks.append(
                    'r["{name}"] = bitreader.read(self.cstruct.{type_name}, {bits})'.format(
                        name=field.name, type_name=field.type.name, bits=field.bits
                    )
                )
                read_size = 0
                cur_block = []
                prev_was_bits = True
                continue
            elif prev_was_bits:
                blocks.append('bitreader.reset()')
                prev_was_bits = False

            try:
                count = len(ft)
                read_size += count
                cur_block.append(field)
            except Exception:
                if cur_block:
                    blocks.append(self.gen_read_block(read_size, cur_block))
                blocks.append(self.gen_dynamic_block(field))
                read_size = 0
                cur_block = []

        if len(cur_block):
            blocks.append(self.gen_read_block(read_size, cur_block))

        read_code = '\n\n'.join(blocks)
        read_code = '\n'.join(['    ' * 2 + line for line in read_code.split('\n')])

        classes.append(COMPILE_TEMPL.format(name=structure.name, read_code=read_code))
        return '\n\n'.join(classes)

    def gen_read_block(self, size, block):
        templ = (
            'buf = stream.read({size})\n'
            'if len(buf) != {size}: raise EOFError()\n'
            'data = struct.unpack(self.cstruct.endian + "{{}}", buf)\n'
            '{{}}'.format(size=size)
        )
        readcode = []
        fmt = []

        curtype = None
        curcount = 0

        buf_offset = 0
        data_offset = 0

        for field in block:
            ft = self.cstruct.resolve(field.type)
            t = ft
            count = 1
            data_count = 1
            read_slice = ''

            if isinstance(t, Enum):
                t = t.type
            elif isinstance(t, Pointer):
                t = self.cstruct.pointer

            if isinstance(ft, Array):
                count = t.count
                data_count = count
                t = t.type

                if isinstance(t, Enum):
                    t = t.type
                elif isinstance(t, Pointer):
                    t = self.cstruct.pointer

                if isinstance(t, (CharType, WcharType, BytesInteger)):
                    read_slice = '{}:{}'.format(
                        buf_offset, buf_offset + (count * t.size)
                    )
                else:
                    read_slice = '{}:{}'.format(data_offset, data_offset + count)
            elif isinstance(t, CharType):
                read_slice = str(buf_offset)
            elif isinstance(t, (WcharType, BytesInteger)):
                read_slice = '{}:{}'.format(buf_offset, buf_offset + t.size)
            else:
                read_slice = str(data_offset)

            if not curtype:
                if isinstance(t, PackedType):
                    curtype = t.packchar
                else:
                    curtype = 'x'

            if isinstance(t, (PackedType, CharType, WcharType, BytesInteger, Enum)):
                charcount = count

                if isinstance(t, (CharType, WcharType, BytesInteger)):
                    data_count = 0
                    packchar = 'x'
                    charcount *= t.size
                else:
                    packchar = t.packchar

                if curtype != packchar:
                    fmt.append('{}{}'.format(curcount, curtype))
                    curcount = 0

                curcount += charcount
                curtype = packchar

            getter = ''
            if isinstance(t, BytesInteger):
                getter = 'BytesInteger.parse(buf[{slice}], {size}, {count}, {signed}, self.cstruct.endian){data_slice}'.format(
                    slice=read_slice,
                    size=t.size,
                    count=count,
                    signed=t.signed,
                    data_slice='[0]' if count == 1 else '',
                )
            elif isinstance(t, (CharType, WcharType)):
                getter = 'buf[{}]'.format(read_slice)
                if isinstance(t, WcharType):
                    getter += ".decode('utf-16-le' if self.cstruct.endian == '<' else 'utf-16-be')"
            else:
                getter = 'data[{}]'.format(read_slice)

            if isinstance(ft, Enum):
                getter = 'EnumInstance(self.cstruct.{type_name}, {getter})'.format(
                    type_name=ft.name, getter=getter
                )
            elif isinstance(ft, Array) and isinstance(ft.type, Enum):
                getter = '[EnumInstance(self.cstruct.{type_name}, d) for d in {getter}]'.format(
                    type_name=ft.type.name, getter=getter
                )
            elif isinstance(ft, Pointer):
                getter = 'PointerInstance(self.cstruct.{type_name}, stream, {getter}, r)'.format(
                    type_name=ft.type.name, getter=getter
                )
            elif isinstance(ft, Array) and isinstance(ft.type, Pointer):
                getter = '[PointerInstance(self.cstruct.{type_name}, stream, d, r) for d in {getter}]'.format(
                    type_name=ft.type.name, getter=getter
                )
            elif isinstance(ft, Array) and isinstance(t, PackedType):
                getter = 'list({})'.format(getter)

            readcode.append(
                'r["{name}"] = {getter}'.format(name=field.name, getter=getter)
            )
            readcode.append(
                'sizes["{name}"] = {size}'.format(name=field.name, size=count * t.size)
            )

            data_offset += data_count
            buf_offset += count * t.size

        if curcount:
            fmt.append('{}{}'.format(curcount, curtype))

        return templ.format(''.join(fmt), '\n'.join(readcode))

    def gen_dynamic_block(self, field):
        if not isinstance(field.type, Array):
            raise CompilerError(
                "Only Array can be dynamic, got {!r}".format(field.type)
            )

        t = field.type.type
        reader = None

        if not field.type.count:  # Null terminated
            if isinstance(t, PackedType):
                reader = (
                    't = []\nwhile True:\n'
                    '    d = stream.read({size})\n'
                    '    if len(d) != {size}: raise EOFError()\n'
                    '    v = struct.unpack(self.cstruct.endian + "{packchar}", d)[0]\n'
                    '    if v == 0: break\n'
                    '    t.append(v)'.format(size=t.size, packchar=t.packchar)
                )

            elif isinstance(t, (CharType, WcharType)):
                reader = (
                    't = []\n'
                    'while True:\n'
                    '    c = stream.read({size})\n'
                    '    if len(c) != {size}: raise EOFError()\n'
                    '    if c == b"{null}": break\n'
                    '    t.append(c)\nt = b"".join(t)'.format(
                        size=t.size, null='\\x00' * t.size
                    )
                )

                if isinstance(t, WcharType):
                    reader += ".decode('utf-16-le' if self.cstruct.endian == '<' else 'utf-16-be')"
            elif isinstance(t, BytesInteger):
                reader = (
                    't = []\n'
                    'while True:\n'
                    '    d = stream.read({size})\n'
                    '    if len(d) != {size}: raise EOFError()\n'
                    '    v = BytesInteger.parse(d, {size}, 1, {signed}, self.cstruct.endian)\n'
                    '    if v == 0: break\n'
                    '    t.append(v)'.format(size=t.size, signed=t.signed)
                )

            return '{reader}\nr["{name}"] = t\nsizes["{name}"] = len(t)'.format(
                reader=reader, name=field.name
            )
        else:
            expr = field.type.count.expr
            expr_read = (
                'dynsize = max(0, Expression(self.cstruct, "{expr}").evaluate(r))\n'
                'buf = stream.read(dynsize * {type_size})\n'
                'if len(buf) != dynsize * {type_size}: raise EOFError()\n'
                'r["{name}"] = {{reader}}\n'
                'sizes["{name}"] = dynsize * {type_size}'.format(
                    expr=expr, name=field.name, type_size=t.size
                )
            )

            if isinstance(t, PackedType):
                reader = 'list(struct.unpack(self.cstruct.endian + "{{:d}}{packchar}".format(dynsize), buf))'.format(
                    packchar=t.packchar, type_size=t.size
                )
            elif isinstance(t, (CharType, WcharType)):
                reader = 'buf'
                if isinstance(t, WcharType):
                    reader += ".decode('utf-16-le' if self.cstruct.endian == '<' else 'utf-16-be')"
            elif isinstance(t, BytesInteger):
                reader = 'BytesInteger.parse(buf, {size}, dynsize, {signed}, self.cstruct.endian)'.format(
                    size=t.size, signed=t.signed
                )

            return expr_read.format(reader=reader, size=None)


def hexdump(s, palette=None, offset=0, prefix=""):
    """Hexdump some data.

    Args:
        s: Bytes to hexdump.
        palette: Colorize the hexdump using this color pattern.
        offset: Byte offset of the hexdump.
        prefix: Optional prefix.
    """
    if palette:
        palette = palette[::-1]

    remaining = 0
    active = None

    for i in xrange(0, len(s), 16):
        vals = ""
        chars = []
        for j in xrange(16):
            if not active and palette:
                remaining, active = palette.pop()
                vals += active
            elif active and j == 0:
                vals += active

            if i + j >= len(s):
                vals += "  "
            else:
                c = s[i + j]
                c = chr(c) if PY3 else c
                p = c if c in PRINTABLE else "."

                if active:
                    vals += "{:02x}".format(ord(c))
                    chars.append(active + p + COLOR_NORMAL)
                else:
                    vals += "{:02x}".format(ord(c))
                    chars.append(p)

                remaining -= 1
                if remaining == 0:
                    active = None

                    if palette is not None:
                        vals += COLOR_NORMAL

                if j == 15:
                    if palette is not None:
                        vals += COLOR_NORMAL

            vals += " "

            if j == 7:
                vals += " "

        chars = "".join(chars)
        print("{}{:08x}  {:48s}  {}".format(prefix, offset + i, vals, chars))


def dumpstruct(t, data=None, offset=0):
    """Dump a structure or parsed structure instance.

    Prints a colorized hexdump and parsed structure output.

    Args:
        t: Structure or Instance to dump.
        data: Bytes to parse the Structure on, if t is not a parsed Instance.
        offset: Byte offset of the hexdump.
    """
    colors = [
        (COLOR_RED, COLOR_BG_RED),
        (COLOR_GREEN, COLOR_BG_GREEN),
        (COLOR_YELLOW, COLOR_BG_YELLOW),
        (COLOR_BLUE, COLOR_BG_BLUE),
        (COLOR_PURPLE, COLOR_BG_PURPLE),
        (COLOR_CYAN, COLOR_BG_CYAN),
        (COLOR_WHITE, COLOR_BG_WHITE),
    ]

    if isinstance(t, Instance):
        g = t
        t = t._type
        data = g.dumps()
    elif isinstance(t, Structure) and data:
        g = t(data)
    else:
        raise ValueError("Invalid arguments")

    palette = []
    ci = 0
    out = "struct {}".format(t.name) + ":\n"
    for field in g._type.fields:
        fg, bg = colors[ci % len(colors)]
        palette.append((g._size(field.name), bg))
        ci += 1

        v = getattr(g, field.name)
        if isinstance(v, str):
            v = repr(v)
        elif isinstance(v, int):
            v = hex(v)
        elif isinstance(v, list):
            v = pprint.pformat(v)
            if '\n' in v:
                v = v.replace('\n', '\n{}'.format(' ' * (len(field.name) + 4)))

        out += "- {}{}{}: {}\n".format(fg, field.name, COLOR_NORMAL, v)

    print()
    hexdump(data, palette, offset=offset)
    print()
    print(out)
