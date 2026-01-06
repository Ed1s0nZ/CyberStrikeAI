from __future__ import annotations

import copy
import numbers
import re
from collections import abc
from typing import Union, List, Iterable, Any, Optional
from bitstring import utils
from bitstring.exceptions import CreationError, Error
from bitstring.bits import Bits, BitsType, TBits

import bitstring.dtypes


class BitArray(Bits):
    """A container holding a mutable sequence of bits.

    Subclass of the immutable Bits class. Inherits all of its
    methods (except __hash__) and adds mutating methods.

    Mutating methods:

    append() -- Append a bitstring.
    byteswap() -- Change byte endianness in-place.
    clear() -- Remove all bits from the bitstring.
    insert() -- Insert a bitstring.
    invert() -- Flip bit(s) between one and zero.
    overwrite() -- Overwrite a section with a new bitstring.
    prepend() -- Prepend a bitstring.
    replace() -- Replace occurrences of one bitstring with another.
    reverse() -- Reverse bits in-place.
    rol() -- Rotate bits to the left.
    ror() -- Rotate bits to the right.
    set() -- Set bit(s) to 1 or 0.

    Methods inherited from Bits:

    all() -- Check if all specified bits are set to 1 or 0.
    any() -- Check if any of specified bits are set to 1 or 0.
    copy() -- Return a copy of the bitstring.
    count() -- Count the number of bits set to 1 or 0.
    cut() -- Create generator of constant sized chunks.
    endswith() -- Return whether the bitstring ends with a sub-string.
    find() -- Find a sub-bitstring in the current bitstring.
    findall() -- Find all occurrences of a sub-bitstring in the current bitstring.
    fromstring() -- Create a bitstring from a formatted string.
    join() -- Join bitstrings together using current bitstring.
    pp() -- Pretty print the bitstring.
    rfind() -- Seek backwards to find a sub-bitstring.
    split() -- Create generator of chunks split by a delimiter.
    startswith() -- Return whether the bitstring starts with a sub-bitstring.
    tobitarray() -- Return bitstring as a bitarray from the bitarray package.
    tobytes() -- Return bitstring as bytes, padding if needed.
    tofile() -- Write bitstring to file, padding if needed.
    unpack() -- Interpret bits using format string.

    Special methods:

    Mutating operators are available: [], <<=, >>=, +=, *=, &=, |= and ^=
    in addition to the inherited [], ==, !=, +, *, ~, <<, >>, &, | and ^.

    Properties:

    [GENERATED_PROPERTY_DESCRIPTIONS]

    len -- Length of the bitstring in bits.

    """

    __slots__ = ()

    # As BitArray objects are mutable, we shouldn't allow them to be hashed.
    __hash__: None = None

    def __init__(self, auto: Optional[Union[BitsType, int]] = None, /, length: Optional[int] = None,
                 offset: Optional[int] = None, **kwargs) -> None:
        """Either specify an 'auto' initialiser:
        A string of comma separated tokens, an integer, a file object,
        a bytearray, a boolean iterable or another bitstring.

        Or initialise via **kwargs with one (and only one) of:
        bin -- binary string representation, e.g. '0b001010'.
        hex -- hexadecimal string representation, e.g. '0x2ef'
        oct -- octal string representation, e.g. '0o777'.
        bytes -- raw data as a bytes object, for example read from a binary file.
        int -- a signed integer.
        uint -- an unsigned integer.
        float / floatbe -- a big-endian floating point number.
        bool -- a boolean (True or False).
        se -- a signed exponential-Golomb code.
        ue -- an unsigned exponential-Golomb code.
        sie -- a signed interleaved exponential-Golomb code.
        uie -- an unsigned interleaved exponential-Golomb code.
        floatle -- a little-endian floating point number.
        floatne -- a native-endian floating point number.
        bfloat / bfloatbe - a big-endian bfloat format 16-bit floating point number.
        bfloatle -- a little-endian bfloat format 16-bit floating point number.
        bfloatne -- a native-endian bfloat format 16-bit floating point number.
        intbe -- a signed big-endian whole byte integer.
        intle -- a signed little-endian whole byte integer.
        intne -- a signed native-endian whole byte integer.
        uintbe -- an unsigned big-endian whole byte integer.
        uintle -- an unsigned little-endian whole byte integer.
        uintne -- an unsigned native-endian whole byte integer.
        filename -- the path of a file which will be opened in binary read-only mode.

        Other keyword arguments:
        length -- length of the bitstring in bits, if needed and appropriate.
                  It must be supplied for all integer and float initialisers.
        offset -- bit offset to the data. These offset bits are
                  ignored and this is intended for use when
                  initialising using 'bytes' or 'filename'.

        """
        if self._bitstore.immutable:
            self._bitstore = self._bitstore._copy()
            self._bitstore.immutable = False

    def copy(self: TBits) -> TBits:
        """Return a copy of the bitstring."""
        return self.__copy__()

    def __setattr__(self, attribute, value) -> None:
        try:
            # First try the ordinary attribute setter
            super().__setattr__(attribute, value)
        except AttributeError:
            dtype = bitstring.dtypes.Dtype(attribute)
            x = object.__new__(Bits)
            if (set_fn := dtype.set_fn) is None:
                raise AttributeError(f"Cannot set attribute '{attribute}' as it does not have a set_fn.")
            set_fn(x, value)
            if len(x) != dtype.bitlength:
                raise CreationError(f"Can't initialise with value of length {len(x)} bits, "
                                    f"as attribute has length of {dtype.bitlength} bits.")
            self._bitstore = x._bitstore
            return

    def __iadd__(self, bs: BitsType) -> BitArray:
        """Append bs to current bitstring. Return self.

        bs -- the bitstring to append.

        """
        self._append(bs)
        return self

    def __copy__(self) -> BitArray:
        """Return a new copy of the BitArray."""
        s_copy = BitArray()
        s_copy._bitstore = self._bitstore._copy()
        assert s_copy._bitstore.immutable is False
        return s_copy

    def _setitem_int(self, key: int, value: Union[BitsType, int]) -> None:
        if isinstance(value, numbers.Integral):
            if value == 0:
                self._bitstore[key] = 0
                return
            if value in (1, -1):
                self._bitstore[key] = 1
                return
            raise ValueError(f"Cannot set a single bit with integer {value}.")
        try:
            value = self._create_from_bitstype(value)
        except TypeError:
            raise TypeError(f"Bitstring, integer or string expected. Got {type(value)}.")
        positive_key = key + len(self) if key < 0 else key
        if positive_key < 0 or positive_key >= len(self._bitstore):
            raise IndexError(f"Bit position {key} out of range.")
        self._bitstore[positive_key: positive_key + 1] = value._bitstore

    def _setitem_slice(self, key: slice, value: BitsType) -> None:
        if isinstance(value, numbers.Integral):
            value = int(value)
            if key.step not in [None, -1, 1]:
                if value in [0, 1]:
                    self.set(value, range(*key.indices(len(self))))
                    return
                else:
                    raise ValueError("Can't assign an integer except 0 or 1 to a slice with a step value.")
            # To find the length we first get the slice
            s = self._bitstore.getslice(key.start, key.stop)
            length = len(s)
            # Now create an int of the correct length
            if value >= 0:
                value = self.__class__(uint=value, length=length)
            else:
                value = self.__class__(int=value, length=length)
        else:
            try:
                value = self._create_from_bitstype(value)
            except TypeError:
                raise TypeError(f"Bitstring, integer or string expected. Got {type(value)}.")
        self._bitstore.__setitem__(key, value._bitstore)

    def __setitem__(self, key: Union[slice, int], value: BitsType) -> None:
        if isinstance(key, numbers.Integral):
            self._setitem_int(int(key), value)
        else:
            self._setitem_slice(key, value)

    def __delitem__(self, key: Union[slice, int]) -> None:
        """Delete item or range.

        >>> a = BitArray('0x001122')
        >>> del a[8:16]
        >>> print a
        0x0022

        """
        self._bitstore.__delitem__(key)
        return

    def __ilshift__(self: TBits, n: int) -> TBits:
        """Shift bits by n to the left in place. Return self.

        n -- the number of bits to shift. Must be >= 0.

        """
        if n < 0:
            raise ValueError("Cannot shift by a negative amount.")
        if not len(self):
            raise ValueError("Cannot shift an empty bitstring.")
        if not n:
            return self
        n = min(n, len(self))
        return self._ilshift(n)

    def __irshift__(self: TBits, n: int) -> TBits:
        """Shift bits by n to the right in place. Return self.

        n -- the number of bits to shift. Must be >= 0.

        """
        if n < 0:
            raise ValueError("Cannot shift by a negative amount.")
        if not len(self):
            raise ValueError("Cannot shift an empty bitstring.")
        if not n:
            return self
        n = min(n, len(self))
        return self._irshift(n)

    def __imul__(self: TBits, n: int) -> TBits:
        """Concatenate n copies of self in place. Return self.

        Called for expressions of the form 'a *= 3'.
        n -- The number of concatenations. Must be >= 0.

        """
        if n < 0:
            raise ValueError("Cannot multiply by a negative integer.")
        return self._imul(n)

    def __ior__(self: TBits, bs: BitsType) -> TBits:
        bs = self._create_from_bitstype(bs)
        self._bitstore |= bs._bitstore
        return self

    def __iand__(self: TBits, bs: BitsType) -> TBits:
        bs = self._create_from_bitstype(bs)
        self._bitstore &= bs._bitstore
        return self

    def __ixor__(self: TBits, bs: BitsType) -> TBits:
        bs = self._create_from_bitstype(bs)
        self._bitstore ^= bs._bitstore
        return self

    def _replace(self, old: Bits, new: Bits, start: int, end: int, count: int, bytealigned: Optional[bool]) -> int:
        if bytealigned is None:
            bytealigned = bitstring.options.bytealigned
        # First find all the places where we want to do the replacements
        starting_points: List[int] = []
        for x in self.findall(old, start, end, bytealigned=bytealigned):
            if not starting_points:
                starting_points.append(x)
            elif x >= starting_points[-1] + len(old):
                # Can only replace here if it hasn't already been replaced!
                starting_points.append(x)
            if count != 0 and len(starting_points) == count:
                break
        if not starting_points:
            return 0
        replacement_list = [self._bitstore.getslice(0, starting_points[0])]
        for i in range(len(starting_points) - 1):
            replacement_list.append(new._bitstore)
            replacement_list.append(
                self._bitstore.getslice(starting_points[i] + len(old), starting_points[i + 1]))
        # Final replacement
        replacement_list.append(new._bitstore)
        replacement_list.append(self._bitstore.getslice(starting_points[-1] + len(old), None))
        if bitstring.options.lsb0:
            # Addition of bitarray is always on the right, so assemble from other end
            replacement_list.reverse()
        self._bitstore.clear()
        for r in replacement_list:
            self._bitstore += r
        return len(starting_points)

    def replace(self, old: BitsType, new: BitsType, start: Optional[int] = None, end: Optional[int] = None,
                count: Optional[int] = None, bytealigned: Optional[bool] = None) -> int:
        """Replace all occurrences of old with new in place.

        Returns number of replacements made.

        old -- The bitstring to replace.
        new -- The replacement bitstring.
        start -- Any occurrences that start before this will not be replaced.
                 Defaults to 0.
        end -- Any occurrences that finish after this will not be replaced.
               Defaults to len(self).
        count -- The maximum number of replacements to make. Defaults to
                 replace all occurrences.
        bytealigned -- If True replacements will only be made on byte
                       boundaries.

        Raises ValueError if old is empty or if start or end are
        out of range.

        """
        if count == 0:
            return 0
        old = self._create_from_bitstype(old)
        new = self._create_from_bitstype(new)
        if len(old) == 0:
            raise ValueError("Empty bitstring cannot be replaced.")
        start, end = self._validate_slice(start, end)

        if new is self:
            # Prevent self assignment woes
            new = copy.copy(self)
        return self._replace(old, new, start, end, 0 if count is None else count, bytealigned)

    def insert(self, bs: BitsType, pos: int) -> None:
        """Insert bs at bit position pos.

        bs -- The bitstring to insert.
        pos -- The bit position to insert at.

        Raises ValueError if pos < 0 or pos > len(self).

        """
        bs = self._create_from_bitstype(bs)
        if len(bs) == 0:
            return
        if bs is self:
            bs = self._copy()
        if pos < 0:
            pos += len(self)
        if not 0 <= pos <= len(self):
            raise ValueError("Invalid insert position.")
        self._insert(bs, pos)

    def overwrite(self, bs: BitsType, pos: int) -> None:
        """Overwrite with bs at bit position pos.

        bs -- The bitstring to overwrite with.
        pos -- The bit position to begin overwriting from.

        Raises ValueError if pos < 0 or pos > len(self).

        """
        bs = self._create_from_bitstype(bs)
        if len(bs) == 0:
            return
        if pos < 0:
            pos += len(self)
        if pos < 0 or pos > len(self):
            raise ValueError("Overwrite starts outside boundary of bitstring.")
        self._overwrite(bs, pos)

    def append(self, bs: BitsType) -> None:
        """Append a bitstring to the current bitstring.

        bs -- The bitstring to append.

        """
        self._append(bs)

    def prepend(self, bs: BitsType) -> None:
        """Prepend a bitstring to the current bitstring.

        bs -- The bitstring to prepend.

        """
        self._prepend(bs)

    def _append_msb0(self, bs: BitsType) -> None:
        self._addright(self._create_from_bitstype(bs))

    def _append_lsb0(self, bs: BitsType) -> None:
        bs = self._create_from_bitstype(bs)
        self._addleft(bs)

    def reverse(self, start: Optional[int] = None, end: Optional[int] = None) -> None:
        """Reverse bits in-place.

        start -- Position of first bit to reverse. Defaults to 0.
        end -- One past the position of the last bit to reverse.
               Defaults to len(self).

        Using on an empty bitstring will have no effect.

        Raises ValueError if start < 0, end > len(self) or end < start.

        """
        start, end = self._validate_slice(start, end)
        if start == 0 and end == len(self):
            self._bitstore.reverse()
            return
        s = self._slice(start, end)
        s._bitstore.reverse()
        self[start:end] = s

    def set(self, value: Any, pos: Optional[Union[int, Iterable[int]]] = None) -> None:
        """Set one or many bits to 1 or 0.

        value -- If bool(value) is True bits are set to 1, otherwise they are set to 0.
        pos -- Either a single bit position or an iterable of bit positions.
               Negative numbers are treated in the same way as slice indices.
               Defaults to the entire bitstring.

        Raises IndexError if pos < -len(self) or pos >= len(self).

        """
        if pos is None:
            # Set all bits to either 1 or 0
            self._setint(-1 if value else 0)
            return
        if not isinstance(pos, abc.Iterable):
            pos = (pos,)
        v = 1 if value else 0
        if isinstance(pos, range):
            self._bitstore.__setitem__(slice(pos.start, pos.stop, pos.step), v)
            return
        for p in pos:
            self._bitstore[p] = v

    def invert(self, pos: Optional[Union[Iterable[int], int]] = None) -> None:
        """Invert one or many bits from 0 to 1 or vice versa.

        pos -- Either a single bit position or an iterable of bit positions.
               Negative numbers are treated in the same way as slice indices.

        Raises IndexError if pos < -len(self) or pos >= len(self).

        """
        if pos is None:
            self._invert_all()
            return
        if not isinstance(pos, abc.Iterable):
            pos = (pos,)
        length = len(self)

        for p in pos:
            if p < 0:
                p += length
            if not 0 <= p < length:
                raise IndexError(f"Bit position {p} out of range.")
            self._invert(p)

    def ror(self, bits: int, start: Optional[int] = None, end: Optional[int] = None) -> None:
        """Rotate bits to the right in-place.

        bits -- The number of bits to rotate by.
        start -- Start of slice to rotate. Defaults to 0.
        end -- End of slice to rotate. Defaults to len(self).

        Raises ValueError if bits < 0.

        """
        if not len(self):
            raise Error("Cannot rotate an empty bitstring.")
        if bits < 0:
            raise ValueError("Cannot rotate by negative amount.")
        self._ror(bits, start, end)

    def _ror_msb0(self, bits: int, start: Optional[int] = None, end: Optional[int] = None) -> None:
        start, end = self._validate_slice(start, end)  # the _slice deals with msb0/lsb0
        bits %= (end - start)
        if not bits:
            return
        rhs = self._slice(end - bits, end)
        self._delete(bits, end - bits)
        self._insert(rhs, start)

    def rol(self, bits: int, start: Optional[int] = None, end: Optional[int] = None) -> None:
        """Rotate bits to the left in-place.

        bits -- The number of bits to rotate by.
        start -- Start of slice to rotate. Defaults to 0.
        end -- End of slice to rotate. Defaults to len(self).

        Raises ValueError if bits < 0.

        """
        if not len(self):
            raise Error("Cannot rotate an empty bitstring.")
        if bits < 0:
            raise ValueError("Cannot rotate by negative amount.")
        self._rol(bits, start, end)

    def _rol_msb0(self, bits: int, start: Optional[int] = None, end: Optional[int] = None):
        start, end = self._validate_slice(start, end)
        bits %= (end - start)
        if bits == 0:
            return
        lhs = self._slice(start, start + bits)
        self._delete(bits, start)
        self._insert(lhs, end - bits)

    def byteswap(self, fmt: Optional[Union[int, Iterable[int], str]] = None, start: Optional[int] = None,
                 end: Optional[int] = None, repeat: bool = True) -> int:
        """Change the endianness in-place. Return number of repeats of fmt done.

        fmt -- A compact structure string, an integer number of bytes or
               an iterable of integers. Defaults to 0, which byte reverses the
               whole bitstring.
        start -- Start bit position, defaults to 0.
        end -- End bit position, defaults to len(self).
        repeat -- If True (the default) the byte swapping pattern is repeated
                  as much as possible.

        """
        start_v, end_v = self._validate_slice(start, end)
        if fmt is None or fmt == 0:
            # reverse all of the whole bytes.
            bytesizes = [(end_v - start_v) // 8]
        elif isinstance(fmt, numbers.Integral):
            if fmt < 0:
                raise ValueError(f"Improper byte length {fmt}.")
            bytesizes = [fmt]
        elif isinstance(fmt, str):
            if not (m := utils.BYTESWAP_STRUCT_PACK_RE.match(fmt)):
                raise ValueError(f"Cannot parse format string {fmt}.")
            # Split the format string into a list of 'q', '4h' etc.
            formatlist = re.findall(utils.STRUCT_SPLIT_RE, m.group('fmt'))
            # Now deal with multiplicative factors, 4h -> hhhh etc.
            bytesizes = []
            for f in formatlist:
                if len(f) == 1:
                    bytesizes.append(utils.PACK_CODE_SIZE[f])
                else:
                    bytesizes.extend([utils.PACK_CODE_SIZE[f[-1]]] * int(f[:-1]))
        elif isinstance(fmt, abc.Iterable):
            bytesizes = fmt
            for bytesize in bytesizes:
                if not isinstance(bytesize, numbers.Integral) or bytesize < 0:
                    raise ValueError(f"Improper byte length {bytesize}.")
        else:
            raise TypeError("Format must be an integer, string or iterable.")

        repeats = 0
        totalbitsize: int = 8 * sum(bytesizes)
        if not totalbitsize:
            return 0
        if repeat:
            # Try to repeat up to the end of the bitstring.
            finalbit = end_v
        else:
            # Just try one (set of) byteswap(s).
            finalbit = start_v + totalbitsize
        for patternend in range(start_v + totalbitsize, finalbit + 1, totalbitsize):
            bytestart = patternend - totalbitsize
            for bytesize in bytesizes:
                byteend = bytestart + bytesize * 8
                self._reversebytes(bytestart, byteend)
                bytestart += bytesize * 8
            repeats += 1
        return repeats

    def clear(self) -> None:
        """Remove all bits, reset to zero length."""
        self._clear()
