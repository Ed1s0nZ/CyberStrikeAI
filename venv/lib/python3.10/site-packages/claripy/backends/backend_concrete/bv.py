from __future__ import annotations

import functools
import numbers

from claripy import debug as _d
from claripy.errors import ClaripyOperationError, ClaripyTypeError, ClaripyZeroDivisionError


def compare_bits(f):
    @functools.wraps(f)
    def compare_guard(self, o):
        if self.bits == 0 or o.bits == 0:
            raise ClaripyTypeError("The operation is not allowed on zero-length bitvectors.")

        if self.bits != o.bits:
            raise ClaripyTypeError(f"bitvectors are differently-sized ({self.bits} and {o.bits})")
        return f(self, o)

    return compare_guard


def compare_bits_0_length(f):
    @functools.wraps(f)
    def compare_guard(self, o):
        if self.bits != o.bits:
            raise ClaripyTypeError(f"bitvectors are differently-sized ({self.bits} and {o.bits})")
        return f(self, o)

    return compare_guard


def normalize_types(f):
    @functools.wraps(f)
    def normalize_helper(self, o):
        if _d._DEBUG and hasattr(o, "__module__") and o.__module__ == "z3":
            raise ValueError("this should no longer happen")
        if isinstance(o, numbers.Number):
            o = BVV(o, self.bits)
        if isinstance(self, numbers.Number):
            self = BVV(self, self.bits)

        if not isinstance(self, BVV) or not isinstance(o, BVV):
            return NotImplemented
        return f(self, o)

    return normalize_helper


class BVV:
    """A concrete bitvector value. Used in the concrete backend for calculations.
    Any use outside of claripy should use `claripy.BVV` instead.
    """

    __slots__ = ["_value", "bits", "mod"]

    def __init__(self, value, bits):
        if _d._DEBUG:
            if not isinstance(bits, numbers.Number) or bits < 0:
                raise ClaripyOperationError("BVV needs a non-negative length and an int value")
            if not isinstance(value, numbers.Number | None):
                raise ClaripyOperationError("BVV needs a number or None value")

            if bits == 0 and value not in (0, "", None):
                raise ClaripyOperationError("Zero-length BVVs cannot have a meaningful value.")

        self.bits = bits
        self._value = 0
        self.mod = 1 << bits
        self.value = value

    def __hash__(self):
        return hash((str(self.value), self.bits))

    def __getstate__(self):
        return (self.bits, self.value)

    def __setstate__(self, s):
        self.bits = s[0]
        self.mod = 1 << self.bits
        self.value = s[1]

    @property
    def value(self):
        return self._value

    @value.setter
    def value(self, v):
        self._value = v & (self.mod - 1) if v is not None else None

    @property
    def signed(self):
        return self._value if self._value < self.mod // 2 else self._value % (self.mod // 2) - (self.mod // 2)

    @signed.setter
    def signed(self, v):
        self._value = v % -self.mod

    #
    # Arithmetic stuff
    #

    @normalize_types
    @compare_bits
    def __add__(self, o):
        return BVV(self.value + o.value, self.bits)

    @normalize_types
    @compare_bits
    def __sub__(self, o):
        return BVV(self.value - o.value, self.bits)

    @normalize_types
    @compare_bits
    def __mul__(self, o):
        return BVV(self.value * o.value, self.bits)

    @normalize_types
    @compare_bits
    def __mod__(self, o):
        if o.value == 0:
            raise ClaripyZeroDivisionError
        return BVV(self.value % o.value, self.bits)

    @normalize_types
    @compare_bits
    def __floordiv__(self, o):
        if o.value == 0:
            raise ClaripyZeroDivisionError
        return BVV(self.value // o.value, self.bits)

    def __truediv__(self, other):
        return self // other  # decline to implicitly have anything to do with floats

    #
    # Reverse arithmetic stuff
    #

    @normalize_types
    @compare_bits
    def __radd__(self, o):
        return BVV(self.value + o.value, self.bits)

    @normalize_types
    @compare_bits
    def __rsub__(self, o):
        return BVV(o.value - self.value, self.bits)

    @normalize_types
    @compare_bits
    def __rmul__(self, o):
        return BVV(self.value * o.value, self.bits)

    @normalize_types
    @compare_bits
    def __rmod__(self, o):
        if self.value == 0:
            raise ClaripyZeroDivisionError
        return BVV(o.value % self.value, self.bits)

    @normalize_types
    @compare_bits
    def __rfloordiv__(self, o):
        if self.value == 0:
            raise ClaripyZeroDivisionError
        return BVV(o.value // self.value, self.bits)

    def __rtruediv__(self, o):
        return self.__rfloordiv__(o)

    #
    # Bit operations
    #

    @normalize_types
    @compare_bits
    def __and__(self, o):
        return BVV(self.value & o.value, self.bits)

    @normalize_types
    @compare_bits
    def __or__(self, o):
        return BVV(self.value | o.value, self.bits)

    @normalize_types
    @compare_bits
    def __xor__(self, o):
        return BVV(self.value ^ o.value, self.bits)

    @normalize_types
    @compare_bits
    def __lshift__(self, o):
        return BVV(self.value << o.value, self.bits)

    @normalize_types
    @compare_bits
    def __rshift__(self, o):
        # arithmetic shift uses the signed version
        return BVV(self.signed >> o.value, self.bits)

    def __invert__(self):
        return BVV(self.value ^ self.mod - 1, self.bits)

    def __neg__(self):
        return BVV((-self.value) % self.mod, self.bits)

    #
    # Reverse bit operations
    #

    @normalize_types
    @compare_bits
    def __rand__(self, o):
        return BVV(self.value & o.value, self.bits)

    @normalize_types
    @compare_bits
    def __ror__(self, o):
        return BVV(self.value | o.value, self.bits)

    @normalize_types
    @compare_bits
    def __rxor__(self, o):
        return BVV(self.value ^ o.value, self.bits)

    @normalize_types
    @compare_bits
    def __rlshift__(self, o):
        return BVV(o.value << self.signed, self.bits)

    @normalize_types
    @compare_bits
    def __rrshift__(self, o):
        return BVV(o.signed >> self.signed, self.bits)

    #
    # Boolean stuff
    #

    @normalize_types
    @compare_bits_0_length
    def __eq__(self, o):
        return self.value == o.value

    @normalize_types
    @compare_bits_0_length
    def __ne__(self, o):
        return self.value != o.value

    @normalize_types
    @compare_bits
    def ULT(self, o):
        return self.value < o.value

    @normalize_types
    @compare_bits
    def UGT(self, o):
        return self.value > o.value

    @normalize_types
    @compare_bits
    def ULE(self, o):
        return self.value <= o.value

    @normalize_types
    @compare_bits
    def UGE(self, o):
        return self.value >= o.value

    #
    # Conversions
    #

    def size(self):
        return self.bits

    def __repr__(self):
        return f"BVV(0x{self.value:x}, {self.bits})"


#
# External stuff
#


def BitVecVal(value, bits):
    return BVV(value, bits)


def ZeroExt(num, o):
    return BVV(o.value, o.bits + num)


def SignExt(num, o):
    return BVV(o.signed, o.bits + num)


def Extract(f, t, o):
    return BVV((o.value >> t) & ((1 << (f + 2 - t)) - 1), f + 1 - t)


def Concat(*args):
    total_bits = 0
    total_value = 0

    for o in args:
        total_value = (total_value << o.bits) | o.value
        total_bits += o.bits
    return BVV(total_value, total_bits)


def RotateRight(self, bits):
    bits_smaller = bits % self.size()
    return LShR(self, bits_smaller) | (self << (self.size() - bits_smaller))


def RotateLeft(self, bits):
    bits_smaller = bits % self.size()
    return (self << bits_smaller) | (LShR(self, (self.size() - bits_smaller)))


def Reverse(a):
    size = a.size()
    if size == 8:
        return a
    if size % 8 != 0:
        raise ClaripyOperationError("can't reverse non-byte sized bitvectors")
    value = a.value
    out = 0
    if size == 64:
        out = _reverse_64(value)
    elif size == 32:
        out = _reverse_32(value)
    elif size == 16:
        out = _reverse_16(value)
    else:
        for i in range(0, size, 8):
            out |= ((value & (0xFF << i)) >> i) << (size - 8 - i)
    return BVV(out, size)

    # the RIGHT way to do it:
    # return BVV(int(("%x" % a.value).rjust(size/4, '0').decode('hex')[::-1].encode('hex'), 16), size)


def _reverse_16(v):
    return ((v & 0xFF) << 8) | ((v & 0xFF00) >> 8)


def _reverse_32(v):
    return ((v & 0xFF) << 24) | ((v & 0xFF00) << 8) | ((v & 0xFF0000) >> 8) | ((v & 0xFF000000) >> 24)


def _reverse_64(v):
    return (
        ((v & 0xFF) << 56)
        | ((v & 0xFF00) << 40)
        | ((v & 0xFF0000) << 24)
        | ((v & 0xFF000000) << 8)
        | ((v & 0xFF00000000) >> 8)
        | ((v & 0xFF0000000000) >> 24)
        | ((v & 0xFF000000000000) >> 40)
        | ((v & 0xFF00000000000000) >> 56)
    )


@normalize_types
@compare_bits
def ULT(self, o):
    return self.value < o.value


@normalize_types
@compare_bits
def UGT(self, o):
    return self.value > o.value


@normalize_types
@compare_bits
def ULE(self, o):
    return self.value <= o.value


@normalize_types
@compare_bits
def UGE(self, o):
    return self.value >= o.value


@normalize_types
@compare_bits
def SLT(self, o):
    return self.signed < o.signed


@normalize_types
@compare_bits
def SGT(self, o):
    return self.signed > o.signed


@normalize_types
@compare_bits
def SLE(self, o):
    return self.signed <= o.signed


@normalize_types
@compare_bits
def SGE(self, o):
    return self.signed >= o.signed


@normalize_types
@compare_bits
def SMod(self, o):
    # compute the remainder like the % operator in C
    a = self.signed
    b = o.signed
    if b == 0:
        raise ClaripyZeroDivisionError
    division_result = a // b if a * b > 0 else (a + (-a % b)) // b
    val = a - division_result * b
    return BVV(val, self.bits)


@normalize_types
@compare_bits
def SDiv(self, o):
    # compute the round towards 0 division
    a = self.signed
    b = o.signed
    if b == 0:
        raise ClaripyZeroDivisionError
    val = a // b if a * b > 0 else (a + (-a % b)) // b
    return BVV(val, self.bits)


#
# Pure boolean stuff
#


def BoolV(b):
    return b


def And(*args):
    return all(args)


def Or(*args):
    return any(args)


def Not(b):
    return not b


@normalize_types
def normalizer(*args):
    return args


def If(c, t, f):
    t, f = normalizer(t, f)  # pylint:disable=unbalanced-tuple-unpacking
    if c:
        return t
    return f


@normalize_types
@compare_bits
def LShR(a, b):
    return BVV(a.value >> b.value, a.bits)
