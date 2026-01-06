from __future__ import annotations

import functools
import math
import struct
from decimal import Decimal

from claripy.errors import ClaripyOperationError
from claripy.fp import FSORT_DOUBLE, FSORT_FLOAT, RM, FSort

from .bv import BVV, Concat


def compare_sorts(f):
    @functools.wraps(f)
    def compare_guard(self, o):
        if self.sort != o.sort:
            raise TypeError(f"FPVs are differently-sorted ({self.sort} and {o.sort})")
        return f(self, o)

    return compare_guard


def normalize_types(f):
    @functools.wraps(f)
    def normalize_helper(self, o):
        if isinstance(o, float):
            o = FPV(o, self.sort)

        if not isinstance(self, FPV) or not isinstance(o, FPV):
            raise TypeError("must have two FPVs")

        return f(self, o)

    return normalize_helper


class FPV:
    """A concrete floating point value. Used in the concrete backend for
    calculations.  Any use outside of claripy should use `claripy.FPV`
    instead.
    """

    __slots__ = ["sort", "value"]

    def __init__(self, value, sort):
        if not isinstance(value, float) or sort not in {FSORT_FLOAT, FSORT_DOUBLE}:
            raise ClaripyOperationError("FPV needs a sort (FSORT_FLOAT or FSORT_DOUBLE) and a float value")

        self.value = value
        self.sort = sort

    def __hash__(self):
        return hash((self.value, self.sort))

    def __getstate__(self):
        return self.value, self.sort

    def __setstate__(self, st):
        self.value, self.sort = st

    def __abs__(self):
        return FPV(abs(self.value), self.sort)

    def __neg__(self):
        return FPV(-self.value, self.sort)

    def fpSqrt(self):
        if self.value < 0:
            return FPV(float("nan"), self.sort)
        return FPV(math.sqrt(self.value), self.sort)

    @normalize_types
    @compare_sorts
    def __add__(self, o):
        return FPV(self.value + o.value, self.sort)

    @normalize_types
    @compare_sorts
    def __sub__(self, o):
        return FPV(self.value - o.value, self.sort)

    @normalize_types
    @compare_sorts
    def __mul__(self, o):
        return FPV(self.value * o.value, self.sort)

    @normalize_types
    @compare_sorts
    def __mod__(self, o):
        return FPV(self.value % o.value, self.sort)

    @normalize_types
    @compare_sorts
    def __truediv__(self, o):
        try:
            return FPV(self.value / o.value, self.sort)
        except ZeroDivisionError:
            if str(self.value * o.value)[0] == "-":
                return FPV(float("-inf"), self.sort)
            return FPV(float("inf"), self.sort)

    def __floordiv__(self, other):  # decline to involve integers in this floating point process
        return self.__truediv__(other)

    #
    # Reverse arithmetic stuff
    #

    @normalize_types
    @compare_sorts
    def __radd__(self, o):
        return FPV(o.value + self.value, self.sort)

    @normalize_types
    @compare_sorts
    def __rsub__(self, o):
        return FPV(o.value - self.value, self.sort)

    @normalize_types
    @compare_sorts
    def __rmul__(self, o):
        return FPV(o.value * self.value, self.sort)

    @normalize_types
    @compare_sorts
    def __rmod__(self, o):
        return FPV(o.value % self.value, self.sort)

    @normalize_types
    @compare_sorts
    def __rtruediv__(self, o):
        try:
            return FPV(o.value / self.value, self.sort)
        except ZeroDivisionError:
            if str(o.value * self.value)[0] == "-":
                return FPV(float("-inf"), self.sort)
            return FPV(float("inf"), self.sort)

    def __rfloordiv__(self, other):  # decline to involve integers in this floating point process
        return self.__rtruediv__(other)

    #
    # Boolean stuff
    #

    @normalize_types
    @compare_sorts
    def __eq__(self, o):
        return self.value == o.value

    @normalize_types
    @compare_sorts
    def __ne__(self, o):
        return self.value != o.value

    @normalize_types
    @compare_sorts
    def __lt__(self, o):
        return self.value < o.value

    @normalize_types
    @compare_sorts
    def __gt__(self, o):
        return self.value > o.value

    @normalize_types
    @compare_sorts
    def __le__(self, o):
        return self.value <= o.value

    @normalize_types
    @compare_sorts
    def __ge__(self, o):
        return self.value >= o.value

    def __repr__(self):
        return f"FPV({self.value:f}, {self.sort})"


def fpToFP(a1, a2, a3=None):
    """
    Returns a FP AST and has three signatures:

        fpToFP(ubvv, sort)
            Returns a FP AST whose value is the same as the unsigned BVV `a1`
            and whose sort is `a2`.

        fpToFP(rm, fpv, sort)
            Returns a FP AST whose value is the same as the floating point `a2`
            and whose sort is `a3`.

        fpToTP(rm, sbvv, sort)
            Returns a FP AST whose value is the same as the signed BVV `a2` and
            whose sort is `a3`.
    """
    if isinstance(a1, BVV) and isinstance(a2, FSort):
        sort = a2
        if sort == FSORT_FLOAT:
            pack, unpack = "I", "f"
        elif sort == FSORT_DOUBLE:
            pack, unpack = "Q", "d"
        else:
            raise ClaripyOperationError("unrecognized float sort")

        try:
            packed = struct.pack("<" + pack, a1.value)
            (unpacked,) = struct.unpack("<" + unpack, packed)
        except OverflowError as e:
            # struct.pack sometimes overflows
            raise ClaripyOperationError("OverflowError: " + str(e)) from e

        return FPV(unpacked, sort)
    if isinstance(a1, RM) and isinstance(a2, FPV) and isinstance(a3, FSort):
        return FPV(a2.value, a3)
    if isinstance(a1, RM) and isinstance(a2, BVV) and isinstance(a3, FSort):
        return FPV(float(a2.signed), a3)
    raise ClaripyOperationError("unknown types passed to fpToFP")


def fpToFPUnsigned(_rm, thing, sort):
    """
    Returns a FP AST whose value is the same as the unsigned BVV `thing` and
    whose sort is `sort`.
    """
    # thing is a BVV
    return FPV(float(thing.value), sort)


def fpToIEEEBV(fpv):
    """
    Interprets the bit-pattern of the IEEE754 floating point number `fpv` as a
    bitvector.

    :return:    A BV AST whose bit-pattern is the same as `fpv`
    """
    if fpv.sort == FSORT_FLOAT:
        pack, unpack = "f", "I"
    elif fpv.sort == FSORT_DOUBLE:
        pack, unpack = "d", "Q"
    else:
        raise ClaripyOperationError("unrecognized float sort")

    try:
        packed = struct.pack("<" + pack, fpv.value)
        (unpacked,) = struct.unpack("<" + unpack, packed)
    except OverflowError as e:
        # struct.pack sometimes overflows
        raise ClaripyOperationError("OverflowError: " + str(e)) from e

    return BVV(unpacked, fpv.sort.length)


def fpFP(sgn, exp, mantissa):
    """
    Concatenates the bitvectors `sgn`, `exp` and `mantissa` and returns the
    corresponding IEEE754 floating point number.

    :return:    A FP AST whose bit-pattern is the same as the concatenated
                bitvector
    """
    concatted = Concat(sgn, exp, mantissa)
    sort = FSort.from_size(concatted.size())

    if sort == FSORT_FLOAT:
        pack, unpack = "I", "f"
    elif sort == FSORT_DOUBLE:
        pack, unpack = "Q", "d"
    else:
        raise ClaripyOperationError("unrecognized float sort")

    try:
        packed = struct.pack("<" + pack, concatted.value)
        (unpacked,) = struct.unpack("<" + unpack, packed)
    except OverflowError as e:
        # struct.pack sometimes overflows
        raise ClaripyOperationError("OverflowError: " + str(e)) from e

    return FPV(unpacked, sort)


def fpToSBV(rm, fp, size):
    try:
        rounding_mode = rm.pydecimal_equivalent_rounding_mode()
        val = int(Decimal(fp.value).to_integral_value(rounding_mode))
        return BVV(val, size)

    except (ValueError, OverflowError):
        return BVV(0, size)
    except Exception as ex:
        print(f"Unhandled error during floating point rounding! {ex}")
        raise


def fpToUBV(rm, fp, size):
    # todo: actually make unsigned
    try:
        rounding_mode = rm.pydecimal_equivalent_rounding_mode()
        val = int(Decimal(fp.value).to_integral_value(rounding_mode))
        assert (
            val & ((1 << size) - 1) == val
        ), f"Rounding produced values outside the BV range! rounding {fp.value} with rounding mode {rm} produced {val}"
        if val < 0:
            val = (1 << size) + val
        return BVV(val, size)

    except (ValueError, OverflowError):
        return BVV(0, size)


def fpEQ(a, b):
    """
    Checks if floating point `a` is equal to floating point `b`.
    """
    return a == b


def fpNEQ(a, b):
    """
    Checks if floating point `a` is not equal to floating point `b`.
    """
    return a != b


def fpGT(a, b):
    """
    Checks if floating point `a` is greater than floating point `b`.
    """
    return a > b


def fpGEQ(a, b):
    """
    Checks if floating point `a` is greater than or equal to floating point `b`.
    """
    return a >= b


def fpLT(a, b):
    """
    Checks if floating point `a` is less than floating point `b`.
    """
    return a < b


def fpLEQ(a, b):
    """
    Checks if floating point `a` is less than or equal to floating point `b`.
    """
    return a <= b


def fpAbs(x):
    """
    Returns the absolute value of the floating point `x`. So:

        a = FPV(-3.2, FSORT_DOUBLE)
        b = fpAbs(a)
        b is FPV(3.2, FSORT_DOUBLE)
    """
    return abs(x)


def fpNeg(x):
    """
    Returns the additive inverse of the floating point `x`. So:

        a = FPV(3.2, FSORT_DOUBLE)
        b = fpAbs(a)
        b is FPV(-3.2, FSORT_DOUBLE)
    """
    return -x


def fpSub(_rm, a, b):
    """
    Returns the subtraction of the floating point `a` by the floating point `b`.
    """
    return a - b


def fpAdd(_rm, a, b):
    """
    Returns the addition of two floating point numbers, `a` and `b`.
    """
    return a + b


def fpMul(_rm, a, b):
    """
    Returns the multiplication of two floating point numbers, `a` and `b`.
    """
    return a * b


def fpDiv(_rm, a, b):
    """
    Returns the division of the floating point `a` by the floating point `b`.
    """
    return a / b


def fpIsNaN(x):
    """
    Checks whether the argument is a floating point NaN.
    """
    return math.isnan(x.value)


def fpIsInf(x):
    """
    Checks whether the argument is a floating point infinity.
    """
    return math.isinf(x.value)
