from __future__ import annotations

import logging
import weakref
from contextlib import suppress

from typing_extensions import Self

import claripy
from claripy import operations
from claripy.ast.base import _make_name
from claripy.errors import BackendError, ClaripyValueError

from .bits import Bits
from .bool import Bool, If

log = logging.getLogger(__name__)

_bvv_cache = weakref.WeakValueDictionary()

# pylint: disable=too-many-positional-arguments


class BV(Bits):
    """
    A class representing an AST of operations culminating in a bitvector.
    Do not instantiate this class directly, instead use BVS or BVV to construct a symbol or value, and then use
    operations to construct more complicated expressions.

    Individual sub-bits and bit-ranges can be extracted from a bitvector using index and slice notation.
    Bits are indexed weirdly. For a 32-bit AST:

        a[31] is the *LEFT* most bit, so it'd be the 0 in

            01111111111111111111111111111111

        a[0] is the *RIGHT* most bit, so it'd be the 0 in

            11111111111111111111111111111110

        a[31:30] are the two leftmost bits, so they'd be the 0s in:

            00111111111111111111111111111111

        a[1:0] are the two rightmost bits, so they'd be the 0s in:

            11111111111111111111111111111100
    """

    __slots__ = ()

    def chop(self, bits=1):
        """
        Chops a BV into consecutive sub-slices. Obviously, the length of this BV must be a multiple of bits.

        :returns:   A list of smaller bitvectors, each ``bits`` in length. The first one will be the left-most (i.e.
                    most significant) bits.
        """
        s = len(self)
        if s % bits != 0:
            raise ValueError(f"expression length ({len(self)}) should be a multiple of 'bits' ({bits})")
        if s == bits:
            return [self]
        return list(reversed([self[(n + 1) * bits - 1 : n * bits] for n in range(s // bits)]))

    def __getitem__(self, rng):
        if type(rng) is slice:
            left = rng.start if rng.start is not None else len(self) - 1
            right = rng.stop if rng.stop is not None else 0
            if left < 0:
                left = len(self) + left
            if right < 0:
                right = len(self) + right
            return Extract(left, right, self)
        return Extract(int(rng), int(rng), self)

    def get_byte(self, index):
        """
        Extracts a byte from a BV, where the index refers to the byte in a big-endian order

        :param index: the byte to extract
        :return: An 8-bit BV
        """
        return self.get_bytes(index, 1)

    def get_bytes(self, index, size):
        """
        Extracts several bytes from a bitvector, where the index refers to the byte in a big-endian order

        :param index: the byte index at which to start extracting
        :param size: the number of bytes to extract
        :return: A BV of size ``size * 8``
        """
        pos = (self.size() + 7) // 8 - 1 - index
        if pos < 0:
            raise ValueError(f"Incorrect index {index}. Your index must be between 0 and {self.size() // 8 - 1}.")
        if size == 0:
            return BVV(0, 0)
        r = self[min(pos * 8 + 7, self.size() - 1) : (pos - size + 1) * 8]
        if r.size() % 8 != 0:  # pylint:disable=no-member
            r = r.zero_extend(8 - r.size() % 8)  # pylint:disable=no-member
        return r

    def zero_extend(self, n):
        """
        Zero-extends the bitvector by n bits. So:

            a = BVV(0b1111, 4)
            b = a.zero_extend(4)
            b is BVV(0b00001111)
        """
        return ZeroExt(n, self)

    def sign_extend(self, n):
        """
        Sign-extends the bitvector by n bits. So:

            a = BVV(0b1111, 4)
            b = a.sign_extend(4)
            b is BVV(0b11111111)
        """
        return SignExt(n, self)

    def concat(self, *args):
        """
        Concatenates this bitvector with the bitvectors provided.
        This bitvector will be on the far-left, i.e. the most significant bits.
        """
        return Concat(self, *args)

    @staticmethod
    def _from_int(like, value):
        return BVV(value, like.length or 64)

    @staticmethod
    def _from_Bool(like, value):
        return If(value, BVV(1, like.length), BVV(0, like.length))

    @staticmethod
    def _from_bytes(like, value):  # pylint:disable=unused-argument
        return BVV(value)

    @staticmethod
    def _from_str(like, value):  # pylint:disable=unused-argument
        log.warning("BVV value is being coerced from a unicode string, encoding as utf-8")
        value = value.encode("utf-8")
        return BVV(value)

    @staticmethod
    def _from_BVV(like, value):  # pylint:disable=unused-argument
        return BVV(value.value, value.size())

    def val_to_fp(self, sort, signed=True, rm=None):
        """
        Interpret this bitvector as an integer, and return the floating-point representation of that integer.

        :param sort:    The sort of floating point value to return
        :param signed:  Optional: whether this value is a signed integer
        :param rm:      Optional: the rounding mode to use
        :return:        An FP AST whose value is the same as this BV
        """
        if rm is None:
            rm = claripy.fp.RM.default()
        if sort is None:
            sort = claripy.fp.FSort.from_size(self.length)

        op = claripy.ast.fp.fpToFP if signed else claripy.ast.fp.fpToFPUnsigned
        return op(rm, self, sort)

    def raw_to_fp(self):
        """
        Interpret the bits of this bitvector as an IEEE754 floating point number.
        The inverse of this function is raw_to_bv.

        :return:        An FP AST whose bit-pattern is the same as this BV
        """
        sort = claripy.fp.FSort.from_size(self.length)
        return claripy.ast.fp.fpToFP(self, sort)

    def raw_to_bv(self):
        """
        A counterpart to FP.raw_to_bv - does nothing and returns itself.
        """
        return self

    def to_bv(self):
        return self.raw_to_bv()

    def identical(self, other: Self) -> bool:
        with suppress(BackendError):
            return claripy.backends.vsa.convert(self).identical(claripy.backends.vsa.convert(other))
        return super().identical(other)


def BVS(  # pylint:disable=redefined-builtin
    name,
    size,
    explicit_name=None,
    **kwargs,
) -> BV:
    """
    Creates a bit-vector symbol (i.e., a variable).

    If you want to specify the maximum or minimum value of a normal symbol that is not part of value-set analysis, you
    should manually add constraints to that effect. **Do not use ``min`` and ``max`` for symbolic execution.**

    :param name:            The name of the symbol.
    :param size:            The size (in bits) of the bit-vector.
    :param bool explicit_name:   If False, an identifier is appended to the name to ensure uniqueness.

    :returns:               a BV object representing this symbol.
    """

    if isinstance(name, bytes):
        name = name.decode()
    if not isinstance(name, str):
        raise TypeError(f"Name value for BVS must be a str, got {type(name)!r}")
    if size is None or not isinstance(size, int):
        raise TypeError("Size value for BVS must be an integer")

    n = _make_name(name, size, False if explicit_name is None else explicit_name)
    encoded_name = n.encode()

    return BV(
        "BVS",
        (n, size),
        variables=frozenset((n,)),
        length=size,
        symbolic=True,
        encoded_name=encoded_name,
        **kwargs,
    )


def BVV(value, size=None, **kwargs) -> BV:
    """
    Creates a bit-vector value (i.e., a concrete value).

    :param value:   The value. Either an integer or a bytestring. If it's the latter, it will be interpreted as the
                    bytes of a big-endian constant.
    :param size:    The size (in bits) of the bit-vector. Optional if you provide a string, required for an integer.

    :returns:       A BV object representing this value.
    """

    if type(value) in (bytes, bytearray, memoryview, str):
        if isinstance(value, str):
            log.warning("BVV value is a unicode string, encoding as utf-8")
            value = value.encode("utf-8")

        if size is None:
            size = len(value) * 8
        elif not isinstance(size, int):
            raise TypeError("Bitvector size  must be either absent (implicit) or an integer")
        elif size != len(value) * 8:
            raise ClaripyValueError("string/size mismatch for BVV creation")

        value = int.from_bytes(value, "big")

    elif size is None or (not isinstance(value, int) and value is not None):
        raise TypeError("BVV() takes either an integer value and a size or a string of bytes")

    # ensure the 0 <= value < (1 << size)
    # FIXME hack to handle None which is used for an Empty Strided Interval (ESI)
    if value is not None:
        value &= (1 << size) - 1

    if not kwargs:
        try:
            return _bvv_cache[(value, size)]
        except KeyError:
            pass

    result = BV("BVV", (value, size), length=size, **kwargs)
    _bvv_cache[(value, size)] = result
    return result


def SI(
    name="unnamed",
    bits=0,
    lower_bound=None,
    upper_bound=None,
    stride=None,
    explicit_name=None,
):
    return BVS(name, bits, explicit_name=explicit_name).annotate(
        claripy.annotation.StridedIntervalAnnotation(stride, lower_bound, upper_bound)
    )


def TSI(bits, name=None, explicit_name=None):
    name = "unnamed" if name is None else name
    return BVS(name, bits, explicit_name=explicit_name)


def ESI(bits, **kwargs):
    return BVV(None, bits, **kwargs)


def ValueSet(bits: int, region: str, region_base_addr: int, value: BV | int):
    if isinstance(value, int):
        value = BVV(value, bits)
    value = value + region_base_addr
    # Annotate the bvs and return the new AST
    return value.annotate(claripy.annotation.RegionAnnotation(region, region_base_addr))


VS = ValueSet


#
# Unbound operations
#


# comparisons
ULT = operations.op("ULT", (BV, BV), Bool, extra_check=operations.length_same_check)
ULE = operations.op("ULE", (BV, BV), Bool, extra_check=operations.length_same_check)
UGT = operations.op("UGT", (BV, BV), Bool, extra_check=operations.length_same_check)
UGE = operations.op("UGE", (BV, BV), Bool, extra_check=operations.length_same_check)
SLT = operations.op("SLT", (BV, BV), Bool, extra_check=operations.length_same_check)
SLE = operations.op("SLE", (BV, BV), Bool, extra_check=operations.length_same_check)
SGT = operations.op("SGT", (BV, BV), Bool, extra_check=operations.length_same_check)
SGE = operations.op("SGE", (BV, BV), Bool, extra_check=operations.length_same_check)

# division
SDiv = operations.op(
    "SDiv",
    (BV, BV),
    BV,
    extra_check=operations.length_same_check,
    calc_length=operations.basic_length_calc,
)
SMod = operations.op(
    "SMod",
    (BV, BV),
    BV,
    extra_check=operations.length_same_check,
    calc_length=operations.basic_length_calc,
)

# bit stuff
LShR = operations.op(
    "LShR",
    (BV, BV),
    BV,
    extra_check=operations.length_same_check,
    calc_length=operations.basic_length_calc,
)
SignExt = operations.op(
    "SignExt", (int, BV), BV, extra_check=operations.extend_check, calc_length=operations.ext_length_calc
)
ZeroExt = operations.op(
    "ZeroExt", (int, BV), BV, extra_check=operations.extend_check, calc_length=operations.ext_length_calc
)
Extract = operations.op(
    "Extract",
    (int, int, BV),
    BV,
    extra_check=operations.extract_check,
    calc_length=operations.extract_length_calc,
)

Concat = operations.op("Concat", BV, BV, calc_length=operations.concat_length_calc)

RotateLeft = operations.op(
    "RotateLeft",
    (BV, BV),
    BV,
    extra_check=operations.length_same_check,
    calc_length=operations.basic_length_calc,
)
RotateRight = operations.op(
    "RotateRight",
    (BV, BV),
    BV,
    extra_check=operations.length_same_check,
    calc_length=operations.basic_length_calc,
)
Reverse = operations.op("Reverse", (BV,), BV, calc_length=operations.basic_length_calc)

union = operations.op(
    "union",
    (BV, BV),
    BV,
    extra_check=operations.length_same_check,
    calc_length=operations.basic_length_calc,
)
widen = operations.op(
    "widen",
    (BV, BV),
    BV,
    extra_check=operations.length_same_check,
    calc_length=operations.basic_length_calc,
)
intersection = operations.op(
    "intersection",
    (BV, BV),
    BV,
    extra_check=operations.length_same_check,
    calc_length=operations.basic_length_calc,
)

#
# Bound operations
#

BV.__add__ = operations.op(
    "__add__", (BV, BV), BV, extra_check=operations.length_same_check, calc_length=operations.basic_length_calc
)
BV.__radd__ = operations.reversed_op(BV.__add__)
BV.__floordiv__ = operations.op(
    "__floordiv__", (BV, BV), BV, extra_check=operations.length_same_check, calc_length=operations.basic_length_calc
)
BV.__rfloordiv__ = operations.reversed_op(BV.__floordiv__)
BV.__truediv__ = BV.__floordiv__
BV.__rtruediv__ = BV.__rfloordiv__
BV.__mul__ = operations.op(
    "__mul__", (BV, BV), BV, extra_check=operations.length_same_check, calc_length=operations.basic_length_calc
)
BV.__rmul__ = operations.reversed_op(BV.__mul__)
BV.__sub__ = operations.op(
    "__sub__", (BV, BV), BV, extra_check=operations.length_same_check, calc_length=operations.basic_length_calc
)
BV.__rsub__ = operations.reversed_op(BV.__sub__)
BV.__mod__ = operations.op(
    "__mod__", (BV, BV), BV, extra_check=operations.length_same_check, calc_length=operations.basic_length_calc
)
BV.__rmod__ = operations.reversed_op(BV.__mod__)
BV.SDiv = operations.op(
    "SDiv",
    (BV, BV),
    BV,
    extra_check=operations.length_same_check,
    calc_length=operations.basic_length_calc,
)
BV.SMod = operations.op(
    "SMod",
    (BV, BV),
    BV,
    extra_check=operations.length_same_check,
    calc_length=operations.basic_length_calc,
)

BV.__neg__ = operations.op("__neg__", (BV,), BV, calc_length=operations.basic_length_calc)
BV.__pos__ = lambda x: x

BV.SLT = operations.op("SLT", (BV, BV), Bool, extra_check=operations.length_same_check)
BV.SGT = operations.op("SGT", (BV, BV), Bool, extra_check=operations.length_same_check)
BV.SLE = operations.op("SLE", (BV, BV), Bool, extra_check=operations.length_same_check)
BV.SGE = operations.op("SGE", (BV, BV), Bool, extra_check=operations.length_same_check)
BV.ULT = operations.op("ULT", (BV, BV), Bool, extra_check=operations.length_same_check)
BV.UGT = operations.op("UGT", (BV, BV), Bool, extra_check=operations.length_same_check)
BV.ULE = operations.op("ULE", (BV, BV), Bool, extra_check=operations.length_same_check)
BV.UGE = operations.op("UGE", (BV, BV), Bool, extra_check=operations.length_same_check)
BV.__eq__ = operations.op("__eq__", (BV, BV), Bool, extra_check=operations.length_same_check)
BV.__ne__ = operations.op("__ne__", (BV, BV), Bool, extra_check=operations.length_same_check)
BV.__ge__ = BV.UGE
BV.__le__ = BV.ULE
BV.__gt__ = BV.UGT
BV.__lt__ = BV.ULT

BV.__invert__ = operations.op("__invert__", (BV,), BV, calc_length=operations.basic_length_calc)
BV.__or__ = operations.op(
    "__or__", (BV, BV), BV, extra_check=operations.length_same_check, calc_length=operations.basic_length_calc
)
BV.__ror__ = operations.reversed_op(BV.__or__)
BV.__and__ = operations.op(
    "__and__", (BV, BV), BV, extra_check=operations.length_same_check, calc_length=operations.basic_length_calc
)
BV.__rand__ = operations.reversed_op(BV.__and__)
BV.__xor__ = operations.op(
    "__xor__", (BV, BV), BV, extra_check=operations.length_same_check, calc_length=operations.basic_length_calc
)
BV.__rxor__ = operations.reversed_op(BV.__xor__)
BV.__lshift__ = operations.op(
    "__lshift__", (BV, BV), BV, extra_check=operations.length_same_check, calc_length=operations.basic_length_calc
)
BV.__rlshift__ = operations.reversed_op(BV.__lshift__)
BV.__rshift__ = operations.op(
    "__rshift__", (BV, BV), BV, extra_check=operations.length_same_check, calc_length=operations.basic_length_calc
)
BV.__rrshift__ = operations.reversed_op(BV.__rshift__)
BV.LShR = operations.op(
    "LShR", (BV, BV), BV, extra_check=operations.length_same_check, calc_length=operations.basic_length_calc
)

BV.Extract = staticmethod(
    operations.op(
        "Extract",
        (int, int, BV),
        BV,
        extra_check=operations.extract_check,
        calc_length=operations.extract_length_calc,
    )
)
BV.Concat = staticmethod(operations.op("Concat", BV, BV, calc_length=operations.concat_length_calc))
BV.reversed = property(operations.op("Reverse", (BV,), BV, calc_length=operations.basic_length_calc))

BV.union = operations.op(
    "union", (BV, BV), BV, extra_check=operations.length_same_check, calc_length=operations.basic_length_calc
)
BV.widen = operations.op(
    "widen", (BV, BV), BV, extra_check=operations.length_same_check, calc_length=operations.basic_length_calc
)
BV.intersection = operations.op(
    "intersection", (BV, BV), BV, extra_check=operations.length_same_check, calc_length=operations.basic_length_calc
)
