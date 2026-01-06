# pylint:disable=missing-class-docstring,raise-missing-from,not-callable
import re
from abc import ABC

from .enums import VEXObject, get_enum_from_int
from .errors import PyVEXError
from .native import ffi, pvc


# IRConst hierarchy
class IRConst(VEXObject, ABC):
    __slots__ = ["_value"]

    type: str
    size: int
    tag: str
    c_constructor = None
    _value: int

    def pp(self):
        print(str(self))

    @property
    def value(self) -> int:
        return self._value

    @staticmethod
    def _from_c(c_const):
        if c_const[0] == ffi.NULL:
            return None

        tag = get_enum_from_int(c_const.tag)

        try:
            return tag_to_const_class(tag)._from_c(c_const)
        except KeyError:
            raise PyVEXError("Unknown/unsupported IRConstTag %s\n" % tag)

    _translate = _from_c

    @classmethod
    def _to_c(cls, const):
        # libvex throws an exception when constructing a U1 with a value other than 0 or 1
        if const.tag == "Ico_U1" and const.value not in (0, 1):
            raise PyVEXError("Invalid U1 value: %d" % const.value)

        try:
            return cls.c_constructor(const.value)
        except KeyError:
            raise PyVEXError("Unknown/unsupported IRConstTag %s]n" % const.tag)

    def __eq__(self, other):
        if not isinstance(other, type(self)):
            return False
        return self._value == other._value

    def __hash__(self):
        return hash((type(self), self._value))


class U1(IRConst):
    __slots__: list[str] = []

    type = "Ity_I1"
    size = 1
    tag = "Ico_U1"
    op_format = "1"
    c_constructor = pvc.IRConst_U1

    def __init__(self, value):
        self._value = value

    def __str__(self):
        return "%d" % self.value

    @staticmethod
    def _from_c(c_const):
        return U1(c_const.Ico.U1)


class U8(IRConst):
    __slots__: list[str] = []

    type = "Ity_I8"
    size = 8
    tag = "Ico_U8"
    op_format = "8"
    c_constructor = pvc.IRConst_U8

    def __init__(self, value):
        self._value = value

    def __str__(self):
        return "0x%02x" % self.value

    @staticmethod
    def _from_c(c_const):
        return _U8_POOL[c_const.Ico.U8]


_U8_POOL = [U8(i) for i in range(256)]


class U16(IRConst):
    __slots__: list[str] = []

    type = "Ity_I16"
    size = 16
    tag = "Ico_U16"
    op_format = "16"
    c_constructor = pvc.IRConst_U16

    def __init__(self, value):
        self._value = value

    def __str__(self):
        return "0x%04x" % self.value

    @staticmethod
    def _from_c(c_const):
        val = c_const.Ico.U16
        if val < 1024:
            return _U16_POOL[val]
        if val >= 0xFC00:
            return _U16_POOL[val - 0xFC00 + 1024]
        return U16(val)


_U16_POOL = [U16(i) for i in range(1024)] + [U16(i) for i in range(0xFC00, 0xFFFF + 1)]


class U32(IRConst):
    __slots__: list[str] = []

    type = "Ity_I32"
    size = 32
    tag = "Ico_U32"
    op_format = "32"
    c_constructor = pvc.IRConst_U32

    def __init__(self, value: int):
        self._value = value

    def __str__(self):
        return "0x%08x" % self.value

    @staticmethod
    def _from_c(c_const):
        val = c_const.Ico.U32
        if val < 1024:
            return _U32_POOL[val]
        if val >= 0xFFFFFC00:
            return _U32_POOL[val - 0xFFFFFC00 + 1024]
        return U32(val)


_U32_POOL = [U32(i) for i in range(1024)] + [U32(i) for i in range(0xFFFFFC00, 0xFFFFFFFF + 1)]


class U64(IRConst):
    __slots__: list[str] = []

    type = "Ity_I64"
    size = 64
    tag = "Ico_U64"
    op_format = "64"
    c_constructor = pvc.IRConst_U64

    def __init__(self, value):
        self._value = value

    def __str__(self):
        return "0x%016x" % self.value

    @staticmethod
    def _from_c(c_const):
        val = c_const.Ico.U64
        if val < 1024:
            return _U64_POOL[val]
        if val >= 0xFFFFFFFFFFFFFC00:
            return _U64_POOL[val - 0xFFFFFFFFFFFFFC00 + 1024]
        return U64(val)


_U64_POOL = [U64(i) for i in range(1024)] + [U64(i) for i in range(0xFFFFFFFFFFFFFC00, 0xFFFFFFFFFFFFFFFF + 1)]

# Integer Type Imagination
class_cache = {1: U1, 8: U8, 16: U16, 32: U32, 64: U64}


def vex_int_class(size):
    try:
        return class_cache[size]
    except KeyError:

        class VexInt(IRConst):
            type = "Ity_I%d" % size
            tag = "Ico_U%d" % size
            op_format = str(size)

            def __init__(self, value):
                IRConst.__init__(self)
                self._value = value

            def __str__(self):
                return f"(0x{self.value:x} :: {self.type})"

        VexInt.__name__ = "U%d" % size
        class_cache[size] = VexInt
        return VexInt


class F32(IRConst):
    __slots__: list[str] = []

    type = "Ity_F32"
    tag = "Ico_F32"
    op_format = "F32"
    c_constructor = pvc.IRConst_F32
    size = 32

    def __init__(self, value):
        self._value = value

    def __str__(self):
        return "%f" % self.value

    @staticmethod
    def _from_c(c_const):
        return F32(c_const.Ico.F32)


class F32i(IRConst):
    __slots__: list[str] = []

    type = "Ity_F32"
    tag = "Ico_F32i"
    op_format = "F32"
    c_constructor = pvc.IRConst_F32i
    size = 32

    def __init__(self, value):
        self._value = value

    def __str__(self):
        return "%f" % self.value

    @staticmethod
    def _from_c(c_const):
        return F32i(c_const.Ico.F32)


class F64(IRConst):
    __slots__: list[str] = []

    type = "Ity_F64"
    tag = "Ico_F64"
    op_format = "F64"
    c_constructor = pvc.IRConst_F64
    size = 64

    def __init__(self, value):
        self._value = value

    def __str__(self):
        return "%f" % self.value

    @staticmethod
    def _from_c(c_const):
        return F64(c_const.Ico.F64)


class F64i(IRConst):
    __slots__: list[str] = []

    type = "Ity_F64"
    tag = "Ico_F64i"
    op_format = "F64"
    c_constructor = pvc.IRConst_F64i
    size = 64

    def __init__(self, value):
        self._value = value

    def __str__(self):
        return "%f" % self.value

    @staticmethod
    def _from_c(c_const):
        return F64i(c_const.Ico.F64)


class V128(IRConst):
    __slots__: list[str] = []

    type = "Ity_V128"
    tag = "Ico_V128"
    op_format = "V128"
    c_constructor = pvc.IRConst_V128
    size = 128

    def __init__(self, value):
        self._value = value

    def __str__(self):
        return "%x" % self.value

    # vex doesn't store a full 128 bit constant, instead it stores 1 bit per 8 bits of data
    # and duplicates each bit 8 times
    @staticmethod
    def _from_c(c_const):
        base_const = c_const.Ico.V128
        real_const = 0
        for i in range(16):
            if (base_const >> i) & 1 == 1:
                real_const |= 0xFF << (8 * i)
        return V128(real_const)


class V256(IRConst):
    __slots__: list[str] = []

    type = "Ity_V256"
    tag = "Ico_V256"
    op_format = "V256"
    c_constructor = pvc.IRConst_V256
    size = 256

    def __init__(self, value):
        self._value = value

    def __str__(self):
        return "%x" % self.value

    # see above
    @staticmethod
    def _from_c(c_const):
        base_const = c_const.Ico.V256
        real_const = 0
        for i in range(32):
            if (base_const >> i) & 1 == 1:
                real_const |= 0xFF << (8 * i)
        return V256(real_const)


predefined_types = [U1, U8, U16, U32, U64, F32, F32i, F64, F64i, V128, V256]
predefined_types_map = {c.type: c for c in predefined_types}
predefined_classes_map = {c.tag: c for c in predefined_types}

# precompiled regexes
int_ty_re = re.compile(r"Ity_I\d+")
int_tag_re = re.compile(r"Ico_U\d+")
tag_size_re = re.compile(r"Ico_[UFV](?P<size>\d+)i?")


def is_int_ty(ty):
    m = int_ty_re.match(ty)
    return m is not None


def is_int_tag(tag):
    m = int_tag_re.match(tag)
    return m is not None


def get_tag_size(tag):
    m = tag_size_re.match(tag)
    if m is None:
        raise ValueError("Tag %s does not have size" % tag)
    return int(m.group("size"))


type_str_re = re.compile(r"Ity_[IFDV](?P<size>\d+)")
type_tag_str_re = re.compile(r"[IFDV]?(?P<size>\d+)[SU]?")


def get_type_size(ty):
    """
    Returns the size, in BITS, of a VEX type specifier
    e.g., Ity_I16 -> 16

    :param ty:
    :return:
    """
    m = type_str_re.match(ty)
    if m is None:
        raise ValueError("Type %s does not have size" % ty)
    return int(m.group("size"))


def get_type_spec_size(ty):
    """
    Get the width of a "type specifier"
    like I16U
    or F16
    or just 16
    (Yes, this really just takes the int out.  If we must special-case, do it here.
    :param tyspec:
    :return:
    """
    m = type_tag_str_re.match(ty)
    if m is None:
        raise ValueError("Type specifier %s does not have size" % ty)
    return int(m.group("size"))


def ty_to_const_class(ty):
    try:
        return predefined_types_map[ty]
    except KeyError:
        if is_int_ty(ty):
            size = get_type_size(ty)
            return vex_int_class(size)
        else:
            raise ValueError("Type %s does not exist" % ty)


def tag_to_const_class(tag):
    try:
        return predefined_classes_map[tag]
    except KeyError:
        if is_int_tag(tag):
            size = get_tag_size(tag)
            return vex_int_class(size)
        else:
            raise ValueError("Tag %s does not exist" % tag)
