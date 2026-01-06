from __future__ import annotations

import contextlib
import functools
import itertools
import logging
import math
import numbers
from functools import reduce
from itertools import chain, product

import claripy
import claripy.backends.backend_vsa as vsa
from claripy.errors import ClaripyOperationError

from .bool_result import FalseResult, MaybeResult, TrueResult
from .errors import ClaripyVSAError
from .warren_methods import max_or, min_or

log = logging.getLogger(__name__)


def reversed_processor(f):
    def processor(self, *args, **kwargs):
        if self._reversed:
            # Reverse it for real. We have to accept the precision penalty.
            reversed_thing = self._reverse()
            return f(reversed_thing, *args, **kwargs)
        return f(self, *args, **kwargs)

    return processor


def normalize_types(f):
    @functools.wraps(f)
    def normalizer(self, o):
        """
        Convert any object to an object that we can process.
        """
        # Special handler for union
        if f.__name__ == "union" and isinstance(o, vsa.DiscreteStridedIntervalSet):
            return o.union(self)

        if isinstance(o, vsa.ValueSet | vsa.DiscreteStridedIntervalSet):
            # if it's singlevalued, we can convert it to a StridedInterval
            if o.cardinality == 1:
                o = o.stridedinterval()
            else:
                # It should be put to o.__radd__(self) when o is a ValueSet
                return NotImplemented

        if isinstance(o, numbers.Number) and isinstance(self, StridedInterval):
            o = StridedInterval(bits=self.bits, stride=0, lower_bound=o, upper_bound=o)

        if isinstance(self, numbers.Number) and isinstance(o, StridedInterval):
            self = StridedInterval(bits=o.bits, stride=0, lower_bound=self, upper_bound=self)

        if f.__name__ != "concat":
            # Make sure they have the same length
            common_bits = max(o.bits, self.bits)
            if o.bits < common_bits:
                o = o.agnostic_extend(common_bits)
            if self.bits < common_bits:
                self = self.agnostic_extend(common_bits)

        #
        # Handling cases where one or both operands are reversed
        #
        # Assumption: Other than a few operations ("Concat" is one of them), reverse only comes from endianness
        #             conversion.
        #
        # if reverse(a) is lossless, reverse(a) op b -> a._reverse() op b
        #                            a op reverse(b) -> reverse(a._reverse() op b)
        # if reverse(b) is lossless, reverse(a) op b -> reverse(a op b._reverse())
        #                            a op reverse(b) -> a op b._reverse()
        # else:
        #       Force reverse and bear the loss in precision

        def _lossless_reverse(a):
            return a.uninitialized or a.is_top or a.is_integer

        reverse_back = False

        if f.__name__ == "concat":
            # TODO: Some optimizations can be applied to concat
            if self._reversed:
                self = self._reverse()
            if o._reversed:
                o = o._reverse()

        else:
            if not self._reversed and not o._reversed:
                pass

            elif self._reversed and o._reversed:
                reverse_back = True
                self = self.copy()
                self._reversed = False
                o = o.copy()
                o._reversed = False

            else:
                # one of the operands is reversed
                if _lossless_reverse(self):
                    self = self._reverse()
                    if o._reversed:
                        reverse_back = True
                elif _lossless_reverse(o):
                    o = o._reverse()
                    if self._reversed:
                        reverse_back = True
                else:
                    # Force reverse
                    if self._reversed:
                        self = self._reverse()
                    if o._reversed:
                        o = o._reverse()

        ret = f(self, o)
        if isinstance(ret, StridedInterval):
            if isinstance(self, StridedInterval) and self.uninitialized:
                ret.uninitialized = True
            if isinstance(o, StridedInterval) and o.uninitialized:
                ret.uninitialized = True
        if reverse_back and isinstance(ret, StridedInterval):
            ret = ret.reverse()
        return ret

    return normalizer


si_id_ctr = itertools.count()

# Whether DiscreteStridedIntervalSet should be used or not. Sometimes we manually
# set it to False to allow easy implementation of test cases.
_allow_dsis_flag = False


@contextlib.contextmanager
def _allow_dsis(flag):
    global _allow_dsis_flag
    saved = _allow_dsis_flag
    _allow_dsis_flag = flag
    claripy.backends.vsa.downsize()
    try:
        yield
    finally:
        _allow_dsis_flag = saved
        claripy.backends.vsa.downsize()


class StridedInterval:
    """
    A Strided Interval is represented in the following form::

        <bits> stride[lower_bound, upper_bound]

    For more details, please refer to relevant papers like TIE and WYSINWYE.

    This implementation is signedness-agostic, please refer to [1]
    *Signedness-Agnostic Program Analysis: Precise Integer Bounds for Low-Level
    Code* by Jorge A. Navas, etc. for more details.

    Note that this implementation only takes hint from [1]. Such a work has been
    improved to be more precise (and still sound) when dealing with strided
    intervals. DO NOT expect to see a 1-to-1 reproduction of [1].

    Thanks all corresponding authors for their outstanding works.
    """

    # pylint: disable=too-many-positional-arguments

    def __init__(
        self, name=None, bits=0, stride=None, lower_bound=None, upper_bound=None, uninitialized=False, bottom=False
    ):
        self._name = name

        if self._name is None:
            self._name = f"SI_{next(si_id_ctr)}"

        self._bits = bits
        self._stride = stride if stride is not None else 1
        self._lower_bound = lower_bound if lower_bound is not None else 0
        self._upper_bound = upper_bound if upper_bound is not None else (2**bits - 1)

        if lower_bound is not None and not isinstance(lower_bound, numbers.Number):
            raise ClaripyVSAError(f"'lower_bound' must be an int. {type(lower_bound)} is not supported.")

        if upper_bound is not None and not isinstance(upper_bound, numbers.Number):
            raise ClaripyVSAError(f"'upper_bound' must be an int. {type(upper_bound)} is not supported.")

        self._reversed = False

        self._is_bottom = bottom

        self.uninitialized = uninitialized

        if self._upper_bound is not None and bits == 0:
            self._bits = self._min_bits()

        if self._upper_bound is None:
            self._upper_bound = StridedInterval.max_int(self.bits)

        if self._lower_bound is None:
            self._lower_bound = StridedInterval.min_int(self.bits)

        # For lower bound and upper bound, we always store the unsigned version
        self._lower_bound &= 2**bits - 1
        self._upper_bound &= 2**bits - 1

        self.normalize()

    def copy(self):
        si = StridedInterval(
            name=self._name,
            bits=self.bits,
            stride=self.stride,
            lower_bound=self.lower_bound,
            upper_bound=self.upper_bound,
            uninitialized=self.uninitialized,
            bottom=self._is_bottom,
        )
        si._reversed = self._reversed
        return si

    def nameless_copy(self):
        si = StridedInterval(
            name=None,
            bits=self.bits,
            stride=self.stride,
            lower_bound=self.lower_bound,
            upper_bound=self.upper_bound,
            uninitialized=self.uninitialized,
            bottom=self._is_bottom,
        )
        si._reversed = self._reversed
        return si

    def normalize(self):
        if self.bits == 8 and self.reversed:
            self._reversed = False

        if self.is_empty:
            return self

        if self.lower_bound == self.upper_bound:
            self._stride = 0

        if self.lower_bound < 0:
            self.lower_bound &= 2**self.bits - 1

        self._normalize_top()

        if self._stride < 0:
            raise ClaripyVSAError("Why does this happen?")

        return self

    def eval(self, n, signed=False):
        """
        Evaluate this StridedInterval to obtain a list of concrete integers.

        :param n: Upper bound for the number of concrete integers
        :param signed: Treat this StridedInterval as signed or unsigned
        :return: A list of at most `n` concrete integers
        """

        if self.is_empty:
            # no value is available
            return []

        if self._reversed:
            return self._reverse().eval(n, signed=signed)

        results = []

        if self.stride == 0 and n > 0:
            results.append(self.lower_bound)
        else:
            bounds = self._signed_bounds() if signed else self._unsigned_bounds()

            for lb, ub in bounds:
                while len(results) < n and lb <= ub:
                    results.append(lb)
                    lb += self.stride  # It will not overflow

        return results

    def solution(self, b):
        """
        Checks whether an integer is solution of the current strided Interval
        :param b: integer to check
        :return: True if b belongs to the current Strided Interval, False otherwhise
        """

        if isinstance(b, numbers.Number):
            b = StridedInterval(lower_bound=b, upper_bound=b, stride=0, bits=self.bits)
        else:
            raise ClaripyOperationError(
                "Oops, Strided intervals cannot be passed as parameter to function solution. To implement"
            )

        return not self.intersection(b).is_empty

    #
    # Private methods
    #

    def __hash__(self):
        return hash(
            (
                f"{self.bits:x} {self.lower_bound:x} {self.upper_bound:x} {self.stride:x}",
                self._reversed,
                self.uninitialized,
            )
        )

    def _normalize_top(self):
        if self.lower_bound == self._modular_add(self.upper_bound, 1, self.bits) and self.stride == 1:
            # This is a TOP!
            # Normalize it
            self.lower_bound = 0
            self.upper_bound = self.max_int(self.bits)

    def _ssplit(self):
        """
        Split `self` at the south pole, which is the same as in unsigned
        arithmetic. When returning two StridedIntervals (which means a
        splitting occurred), it is guaranteed that the first StridedInterval is
        on the right side of the south pole.

        :return: a list of split StridedIntervals, that contains either one or
            two StridedIntervals
        """

        south_pole_right = self.max_int(self.bits)  # 111...1
        # south_pole_left = 0

        # Is `self` straddling the south pole?
        if self.upper_bound < self.lower_bound:
            # It straddles the south pole!

            a_upper_bound = south_pole_right - ((south_pole_right - self.lower_bound) % self.stride)
            a = StridedInterval(
                bits=self.bits,
                stride=self.stride,
                lower_bound=self.lower_bound,
                upper_bound=a_upper_bound,
                uninitialized=self.uninitialized,
            )

            b_lower_bound = self._modular_add(a_upper_bound, self.stride, self.bits)
            b = StridedInterval(
                bits=self.bits,
                stride=self.stride,
                lower_bound=b_lower_bound,
                upper_bound=self.upper_bound,
                uninitialized=self.uninitialized,
            )

            return [a, b]

        return [self.copy()]

    def _nsplit(self):
        """
        Split `self` at the north pole, which is the same as in signed arithmetic.

        :return: A list of split StridedIntervals
        """

        north_pole_left = self.max_int(self.bits - 1)  # 01111...1
        north_pole_right = 2 ** (self.bits - 1)  # 1000...0

        # Is `self` straddling the north pole?
        straddling = False
        if self.upper_bound >= north_pole_right:
            if self.lower_bound > self.upper_bound:
                # Yes it does!
                straddling = True
            elif self.lower_bound <= north_pole_left:
                straddling = True

        else:
            if self.lower_bound > self.upper_bound and self.lower_bound <= north_pole_left:
                straddling = True

        if straddling:
            a_upper_bound = north_pole_left - ((north_pole_left - self.lower_bound) % self.stride)
            a = StridedInterval(
                bits=self.bits,
                stride=self.stride,
                lower_bound=self.lower_bound,
                upper_bound=a_upper_bound,
                uninitialized=self.uninitialized,
            )

            b_lower_bound = a_upper_bound + self.stride
            b = StridedInterval(
                bits=self.bits,
                stride=self.stride,
                lower_bound=b_lower_bound,
                upper_bound=self.upper_bound,
                uninitialized=self.uninitialized,
            )

            return [a, b]

        return [self.copy()]

    def _psplit(self):
        """
        Split `self` at both north and south poles.

        :return: A list of split StridedIntervals
        """

        nsplit_list = self._nsplit()
        psplit_list = []

        for si in nsplit_list:
            psplit_list.extend(si._ssplit())

        return psplit_list

    def _signed_bounds(self):
        """
        Get lower bound and upper bound for `self` in signed arithmetic.

        :return: a list of (lower_bound, upper_bound) tuples
        """

        nsplit = self._nsplit()
        if len(nsplit) == 1:
            lb = nsplit[0].lower_bound
            ub = nsplit[0].upper_bound

            lb = self._unsigned_to_signed(lb, self.bits)
            ub = self._unsigned_to_signed(ub, self.bits)

            return [(lb, ub)]

        if len(nsplit) == 2:
            # nsplit[0] is on the left hemisphere, and nsplit[1] is on the right hemisphere

            # The left one
            lb_1 = nsplit[0].lower_bound
            ub_1 = nsplit[0].upper_bound

            # The right one
            lb_2 = nsplit[1].lower_bound
            ub_2 = nsplit[1].upper_bound
            # Then convert them to negative numbers
            lb_2 = self._unsigned_to_signed(lb_2, self.bits)
            ub_2 = self._unsigned_to_signed(ub_2, self.bits)

            return [(lb_1, ub_1), (lb_2, ub_2)]

        raise ClaripyVSAError("WTF")

    def _unsigned_bounds(self):
        """
        Get lower bound and upper bound for `self` in unsigned arithmetic.

        :return: a list of (lower_bound, upper_bound) tuples.
        """

        ssplit = self._ssplit()
        if len(ssplit) == 1:
            lb = ssplit[0].lower_bound
            ub = ssplit[0].upper_bound

            return [(lb, ub)]
        if len(ssplit) == 2:
            # ssplit[0] is on the left hemisphere, and ssplit[1] is on the right hemisphere

            lb_1 = ssplit[0].lower_bound
            ub_1 = ssplit[0].upper_bound

            lb_2 = ssplit[1].lower_bound
            ub_2 = ssplit[1].upper_bound

            return [(lb_1, ub_1), (lb_2, ub_2)]

        raise ClaripyVSAError("WTF")

    def _rshift_logical(self, shift_amount):
        """
        Logical shift right with a concrete shift amount

        :param int shift_amount: Number of bits to shift right.
        :return: The new StridedInterval after right shifting
        :rtype: StridedInterval
        """

        if self.is_empty:
            return self

        # If straddling the south pole, we'll have to split it into two, perform
        # logical right shift on them individually, then union the result back
        # together for better precision. Note that it's an improvement from the
        # original WrappedIntervals paper.

        ssplit = self._ssplit()
        if len(ssplit) == 1:
            lower = self.lower_bound >> shift_amount
            upper = self.upper_bound >> shift_amount
            stride = max(self.stride >> shift_amount, 1)

            return StridedInterval(
                bits=self.bits, lower_bound=lower, upper_bound=upper, stride=stride, uninitialized=self.uninitialized
            )
        a = ssplit[0]._rshift_logical(shift_amount)
        b = ssplit[1]._rshift_logical(shift_amount)

        return a.union(b)

    def _rshift_arithmetic(self, shift_amount):
        """
        Arithmetic shift right with a concrete shift amount

        :param int shift_amount: Number of bits to shift right.
        :return: The new StridedInterval after right shifting
        :rtype: StridedInterval
        """

        if self.is_empty:
            return self

        # If straddling the north pole, we'll have to split it into two, perform
        # arithmetic right shift on them individually, then union the result
        # back together for better precision. Note that it's an improvement from
        # the original WrappedIntervals paper.

        nsplit = self._nsplit()
        if len(nsplit) == 1:
            # preserve the highest bit :-)
            highest_bit_set = self.lower_bound > StridedInterval.signed_max_int(nsplit[0].bits)

            lower = self.lower_bound >> shift_amount
            upper = self.upper_bound >> shift_amount
            stride = max(self.stride >> shift_amount, 1)
            mask = (2**shift_amount - 1) << (self.bits - shift_amount)

            if highest_bit_set:
                lower = lower | mask
                upper = upper | mask
            if lower == upper:
                stride = 0
            return StridedInterval(
                bits=self.bits, lower_bound=lower, upper_bound=upper, stride=stride, uninitialized=self.uninitialized
            )
        a = nsplit[0]._rshift_arithmetic(shift_amount)
        b = nsplit[1]._rshift_arithmetic(shift_amount)

        return a.union(b)

    #
    # Comparison operations
    #

    def identical(self, o):
        """
        Used to make exact comparisons between two StridedIntervals. Usually it
        is only used in test cases.

        :param o: The other StridedInterval to compare with.
        :return: True if they are exactly same, False otherwise.
        """
        return (
            self.bits == o.bits
            and self.stride == o.stride
            and self.lower_bound == o.lower_bound
            and self.upper_bound == o.upper_bound
        )

    @normalize_types
    def SLT(self, o):
        """
        Signed less than

        :param o: The other operand
        :return: TrueResult(), FalseResult(), or MaybeResult()
        """

        signed_bounds_1 = self._signed_bounds()
        signed_bounds_2 = o._signed_bounds()

        ret = []
        for lb_1, ub_1 in signed_bounds_1:
            for lb_2, ub_2 in signed_bounds_2:
                if ub_1 < lb_2:
                    ret.append(TrueResult())
                elif lb_1 >= ub_2:
                    ret.append(FalseResult())
                else:
                    ret.append(MaybeResult())

        if all(r.identical(TrueResult()) for r in ret):
            return TrueResult()
        if all(r.identical(FalseResult()) for r in ret):
            return FalseResult()
        return MaybeResult()

    @normalize_types
    def SLE(self, o):
        """
        Signed less than or equal to.

        :param o: The other operand.
        :return: TrueResult(), FalseResult(), or MaybeResult()
        """

        signed_bounds_1 = self._signed_bounds()
        signed_bounds_2 = o._signed_bounds()

        ret = []
        for lb_1, ub_1 in signed_bounds_1:
            for lb_2, ub_2 in signed_bounds_2:
                if ub_1 <= lb_2:
                    ret.append(TrueResult())
                elif lb_1 > ub_2:
                    ret.append(FalseResult())
                else:
                    ret.append(MaybeResult())

        if all(r.identical(TrueResult()) for r in ret):
            return TrueResult()
        if all(r.identical(FalseResult()) for r in ret):
            return FalseResult()
        return MaybeResult()

    @normalize_types
    def SGT(self, o):
        """
        Signed greater than.

        :param o: The other operand
        :return: TrueResult(), FalseResult(), or MaybeResult()
        """

        signed_bounds_1 = self._signed_bounds()
        signed_bounds_2 = o._signed_bounds()

        ret = []
        for lb_1, ub_1 in signed_bounds_1:
            for lb_2, ub_2 in signed_bounds_2:
                if lb_1 > ub_2:
                    ret.append(TrueResult())
                elif ub_1 <= lb_2:
                    ret.append(FalseResult())
                else:
                    ret.append(MaybeResult())

        if all(r.identical(TrueResult()) for r in ret):
            return TrueResult()
        if all(r.identical(FalseResult()) for r in ret):
            return FalseResult()
        return MaybeResult()

    @normalize_types
    def SGE(self, o):
        """
        Signed greater than or equal to.

        :param o: The other operand
        :return: TrueResult(), FalseResult(), or MaybeResult()
        """

        signed_bounds_1 = self._signed_bounds()
        signed_bounds_2 = o._signed_bounds()

        ret = []
        for lb_1, ub_1 in signed_bounds_1:
            for lb_2, ub_2 in signed_bounds_2:
                if lb_1 >= ub_2:
                    ret.append(TrueResult())
                elif ub_1 < lb_2:
                    ret.append(FalseResult())
                else:
                    ret.append(MaybeResult())

        if all(r.identical(TrueResult()) for r in ret):
            return TrueResult()
        if all(r.identical(FalseResult()) for r in ret):
            return FalseResult()
        return MaybeResult()

    @normalize_types
    def ULT(self, o):
        """
        Unsigned less than.

        :param o: The other operand
        :return: TrueResult(), FalseResult(), or MaybeResult()
        """

        unsigned_bounds_1 = self._unsigned_bounds()
        unsigned_bounds_2 = o._unsigned_bounds()

        ret = []
        for lb_1, ub_1 in unsigned_bounds_1:
            for lb_2, ub_2 in unsigned_bounds_2:
                if ub_1 < lb_2:
                    ret.append(TrueResult())
                elif lb_1 >= ub_2:
                    ret.append(FalseResult())
                else:
                    ret.append(MaybeResult())

        if all(r.identical(TrueResult()) for r in ret):
            return TrueResult()
        if all(r.identical(FalseResult()) for r in ret):
            return FalseResult()
        return MaybeResult()

    @normalize_types
    def ULE(self, o):
        """
        Unsigned less than or equal to.

        :param o: The other operand
        :return: TrueResult(), FalseResult(), or MaybeResult()
        """

        unsigned_bounds_1 = self._unsigned_bounds()
        unsigned_bounds_2 = o._unsigned_bounds()

        ret = []
        for lb_1, ub_1 in unsigned_bounds_1:
            for lb_2, ub_2 in unsigned_bounds_2:
                if ub_1 <= lb_2:
                    ret.append(TrueResult())
                elif lb_1 > ub_2:
                    ret.append(FalseResult())
                else:
                    ret.append(MaybeResult())

        if all(r.identical(TrueResult()) for r in ret):
            return TrueResult()
        if all(r.identical(FalseResult()) for r in ret):
            return FalseResult()
        return MaybeResult()

    @normalize_types
    def UGT(self, o):
        """
        Signed greater than.

        :param o: The other operand
        :return: TrueResult(), FalseResult(), or MaybeResult()
        """

        unsigned_bounds_1 = self._unsigned_bounds()
        unsigned_bounds_2 = o._unsigned_bounds()

        ret = []
        for lb_1, ub_1 in unsigned_bounds_1:
            for lb_2, ub_2 in unsigned_bounds_2:
                if lb_1 > ub_2:
                    ret.append(TrueResult())
                elif ub_1 <= lb_2:
                    ret.append(FalseResult())
                else:
                    ret.append(MaybeResult())

        if all(r.identical(TrueResult()) for r in ret):
            return TrueResult()
        if all(r.identical(FalseResult()) for r in ret):
            return FalseResult()
        return MaybeResult()

    @normalize_types
    def UGE(self, o):
        """
        Unsigned greater than or equal to.

        :param o: The other operand
        :return: TrueResult(), FalseResult(), or MaybeResult()
        """

        unsigned_bounds_1 = self._unsigned_bounds()
        unsigned_bounds_2 = o._unsigned_bounds()

        ret = []
        for lb_1, ub_1 in unsigned_bounds_1:
            for lb_2, ub_2 in unsigned_bounds_2:
                if lb_1 >= ub_2:
                    ret.append(TrueResult())
                elif ub_1 < lb_2:
                    ret.append(FalseResult())
                else:
                    ret.append(MaybeResult())

        if all(r.identical(TrueResult()) for r in ret):
            return TrueResult()
        if all(r.identical(FalseResult()) for r in ret):
            return FalseResult()
        return MaybeResult()

    @normalize_types
    def eq(self, o):
        """
        Equal

        :param o: The ohter operand
        :return: TrueResult(), FalseResult(), or MaybeResult()
        """

        if self.is_integer and o.is_integer:
            # Two integers
            if self.lower_bound == o.lower_bound:
                # They are equal
                return TrueResult()
            # They are not equal
            return FalseResult()

        if self.name == o.name:
            return TrueResult()  # They are the same guy

        si_intersection = self.intersection(o)
        if si_intersection.is_empty:
            return FalseResult()

        return MaybeResult()

    #
    # Overriding default operators in Python
    #

    def __len__(self):
        """
        Get the length in bits of this variable.
        :return:
        """
        return self._bits

    def __eq__(self, o):
        return self.eq(o)

    def __ne__(self, o):
        return ~(self.eq(o))

    def __gt__(self, other):
        """
        Unsigned greater than

        :param other: The other operand
        :return: TrueResult(), FalseResult(), or MaybeResult()
        """
        return self.UGT(other)

    def __ge__(self, other):
        """
        Unsigned greater than or equal to

        :param other: The other operand
        :return: TrueResult(), FalseResult(), or MaybeResult()
        """

        return self.UGE(other)

    def __lt__(self, other):
        """
        Unsigned less than

        :param other: The other operand
        :return: TrueResult(), FalseResult(), or MaybeResult()
        """
        return self.ULT(other)

    def __le__(self, other):
        """
        Unsigned less than or equal to

        :param other: The other operand
        :return: TrueResult(), FalseResult(), or MaybeResult()
        """
        return self.ULE(other)

    def __add__(self, o):
        return self.add(o)

    def __radd__(self, o):
        return self.add(o)

    def __sub__(self, o):
        return self.sub(o)

    def __rsub__(self, o):
        return StridedInterval(bits=self.bits, stride=0, lower_bound=o, upper_bound=o).sub(self)

    @normalize_types
    def __mul__(self, o):
        return self.mul(o)

    @normalize_types
    def __mod__(self, o):
        # TODO: Make a better approximatiom
        # FIXME: this is the implementation of the unsigned modulo
        # implement also the signed one.
        if o.is_integer and o.lower_bound == 0:
            return StridedInterval.empty(o.bits)
        if self.is_integer and o.is_integer:
            r = self.lower_bound % o.lower_bound
            return StridedInterval(bits=self.bits, stride=0, lower_bound=r, upper_bound=r)

        all_resulting_intervals = []
        for s in self._ssplit():
            for t in o._ssplit():
                card = s.udiv(t).cardinality
                if card == 1:
                    tmp = s.sub(s.udiv(t)).mul(t)
                else:
                    tmp = StridedInterval(bits=self.bits, stride=1, lower_bound=0, upper_bound=o.upper_bound - 1)
                all_resulting_intervals.append(tmp)

        return StridedInterval.least_upper_bound(*all_resulting_intervals).normalize()

    @normalize_types
    def __floordiv__(self, o):
        """
        Unsigned division

        :param o: The divisor
        :return: The quotient (self / o)
        """

        return self.udiv(o)

    def __truediv__(self, other):
        return self // other  # decline to involve floating point numbers at ALL

    def __neg__(self):
        return self.bitwise_not()

    def __invert__(self):
        return self.bitwise_not()

    @normalize_types
    def __or__(self, other):
        return self.bitwise_or(other)

    @normalize_types
    def __and__(self, other):
        return self.bitwise_and(other)

    def __rand__(self, other):
        return self.__and__(other)

    @normalize_types
    def __xor__(self, other):
        return self.bitwise_xor(other)

    def __rxor__(self, other):
        return self.__xor__(other)

    def __lshift__(self, other):
        return self.lshift(other)

    def __rshift__(self, shift_amount):
        """
        Arithmetic shift right.

        :param StridedInterval shift_amount: Number of bits to shift right.
        :return: The shifted StridedInterval object
        :rtype: StridedInterval
        """
        return self.rshift_arithmetic(shift_amount)

    def __repr__(self):
        if self.is_empty:
            s = f"<{self._bits}>[EmptySI]"
        else:
            lower_bound = self._lower_bound if isinstance(self._lower_bound, str) else f"{self._lower_bound:#x}"
            upper_bound = self._upper_bound if isinstance(self._upper_bound, str) else f"{self._upper_bound:#x}"
            s = f"<{self._bits}>0x{self._stride:x}[{lower_bound}, {upper_bound}]{('R' if self._reversed else '')}"

        if self.uninitialized:
            s += "(uninit)"

        return s

    #
    # Other operations
    #

    def LShR(self, shift_amount):
        """
        Logical shift right.
        :param StridedInterval shift_amount: The amount of shifting
        :return: The shifted StridedInterval object
        :rtype: StridedInterval
        """

        return self.rshift_logical(shift_amount)

    #
    # Properties
    #

    @property
    def name(self):
        return self._name

    @property
    def reversed(self):
        return self._reversed

    @property
    def cardinality(self):
        if self.is_bottom:
            return 0
        if self.is_integer:
            return 1
        return (self._modular_sub(self._upper_bound, self._lower_bound, self.bits) + self._stride) // self._stride

    @property
    def complement(self):
        """
        Return the complement of the interval
        Refer section 3.1 augmented for managing strides

        :return:
        """
        # case 1
        if self.is_empty:
            return StridedInterval.top(self.bits)
        # case 2
        if self.is_top:
            return StridedInterval.empty(self.bits)
        # case 3
        y_plus_1 = StridedInterval._modular_add(self.upper_bound, 1, self.bits)
        x_minus_1 = StridedInterval._modular_sub(self.lower_bound, 1, self.bits)

        # the new stride has to be the GCD between the old stride and the distance
        # between the new lower bound and the new upper bound. This assure that in
        # the new interval the boundaries are valid solution when the SI is
        # evaluated.
        dist = StridedInterval._wrapped_cardinality(y_plus_1, x_minus_1, self.bits) - 1

        # the new SI is an integer
        if dist < 0:
            new_stride = 0
        elif self._stride == 0:
            new_stride = 1
        else:
            new_stride = math.gcd(self._stride, dist)
        return StridedInterval(
            lower_bound=y_plus_1,
            upper_bound=x_minus_1,
            bits=self.bits,
            stride=new_stride,
            uninitialized=self.uninitialized,
        )

    @property
    def lower_bound(self):
        return self._lower_bound

    @lower_bound.setter
    def lower_bound(self, value):
        self._lower_bound = value

    @property
    def upper_bound(self):
        return self._upper_bound

    @upper_bound.setter
    def upper_bound(self, value):
        self._upper_bound = value

    @property
    def bits(self):
        return self._bits

    @property
    def stride(self):
        return self._stride

    @stride.setter
    def stride(self, value):
        self._stride = value

    @property
    @reversed_processor
    def max(self):
        """
        Treat this StridedInterval as a set of unsigned numbers, and return the
        greatest one

        :return: the greatest number in this StridedInterval when evaluated as
            unsigned, or None if empty
        """
        if not self.is_empty:
            splitted = self._ssplit()
            return splitted[0].upper_bound
        # It is empty!
        return None

    @property
    @reversed_processor
    def min(self):
        """
        Treat this StridedInterval as a set of unsigned numbers, and return the
        smallest one

        :return: the smallest number in this StridedInterval when evaluated as
            unsigned, or None if empty
        """
        if not self.is_empty:
            splitted = self._ssplit()
            return splitted[-1].lower_bound
        # It is empty
        return None

    @property
    def unique(self):
        return self.lower_bound is not None and self.lower_bound == self.upper_bound

    def _min_bits(self):
        v = self._upper_bound
        assert v >= 0
        return StridedInterval.min_bits(v)

    @property
    def is_empty(self):
        """
        The same as is_bottom
        :return: True/False
        """
        return self.is_bottom

    @property
    def is_top(self):
        """
        If this is a TOP value.

        :return: True if this is a TOP
        """
        return self.stride == 1 and self.lower_bound == self._modular_add(self.upper_bound, 1, self.bits)

    @property
    def is_bottom(self):
        """
        Whether this StridedInterval is a BOTTOM, in other words, describes an
        empty set of integers.

        :return: True/False
        """
        return self._is_bottom

    @property
    def is_integer(self):
        """
        If this is an integer, i.e. self.lower_bound == self.upper_bound.

        :return: True if this is an integer, False otherwise
        """
        return self.lower_bound == self.upper_bound

    @property
    def is_interval(self):
        return not self.is_integer

    @property
    def n_values(self):
        return (StridedInterval._wrapped_cardinality(self.lower_bound, self.upper_bound, self.bits) // self.stride) + 1

    #
    # Modular arithmetic
    #

    @staticmethod
    def _modular_add(a, b, bits):
        return (a + b) % (2**bits)

    @staticmethod
    def _modular_sub(a, b, bits):
        return (a - b) % (2**bits)

    @staticmethod
    def _modular_mul(a, b, bits):
        return (a * b) % (2**bits)

    #
    # Helper methods
    #

    @staticmethod
    def lcm(a, b):
        """
        Get the least common multiple.

        :param a: The first operand (integer)
        :param b: The second operand (integer)
        :return: Their LCM
        """
        return a * b // math.gcd(a, b)

    @staticmethod
    def gcd(a, b):
        """
        Get the greatest common divisor.

        :param a: The first operand (integer)
        :param b: The second operand (integer)
        :return: Their GCD
        """

        return math.gcd(a, b)

    @staticmethod
    def highbit(k):
        return 1 << (k - 1)

    @staticmethod
    def min_bits(val, max_bits=None):
        if val == 0:
            return 1
        if val < 0:
            if max_bits is None:
                return int(math.log2(-val) + 1) + 1
            assert isinstance(max_bits, int)
            return int(math.log2((((1 << max_bits) - 1) & ~(-val)) + 1) + 1)
        # FIXME: Support other bits
        # Here we assume the maximum val is 64 bits
        # Special case to deal with the floating-point imprecision
        if 0xFFFFFFFFFFFE0000 < val <= 0x10000000000000000:
            return 64
        return int(math.log2(val) + 1)

    @staticmethod
    def max_int(k):
        return StridedInterval.highbit(k + 1) - 1

    @staticmethod
    def min_int(k):
        return -StridedInterval.highbit(k)

    @staticmethod
    def signed_max_int(k):
        return 2 ** (k - 1) - 1

    @staticmethod
    def signed_min_int(k):
        return -(2 ** (k - 1))

    @staticmethod
    def _to_negative(a, bits):
        return -((1 << bits) - a)

    @staticmethod
    def upper(bits, i, stride):
        """

        :return:
        """
        if stride >= 1:
            offset = i % stride
            max = StridedInterval.max_int(bits)  # pylint:disable=redefined-builtin
            max_offset = max % stride

            return max - (max_offset - offset) if max_offset >= offset else max - (max_offset + stride - offset)
        return StridedInterval.max_int(bits)

    @staticmethod
    def lower(bits, i, stride):
        """

        :return:
        """
        if stride >= 1:
            offset = i % stride
            min = StridedInterval.min_int(bits)  # pylint:disable=redefined-builtin
            min_offset = min % stride

            return min + (offset - min_offset) if offset >= min_offset else min + (offset + stride - min_offset)
        return StridedInterval.min_int(bits)

    @staticmethod
    def _gap(src_interval, tar_interval):
        """
        Refer section 3.1; gap function.

        :param src_interval: first argument or interval 1
        :param tar_interval: second argument or interval 2
        :return: Interval representing gap between two intervals
        """
        assert src_interval.bits == tar_interval.bits, "Number of bits should be same for operands"
        # use the same variable names as in paper
        s = src_interval
        t = tar_interval
        (_, b) = (s.lower_bound, s.upper_bound)
        (c, _) = (t.lower_bound, t.upper_bound)

        w = s.bits
        # case 1
        if (not t._surrounds_member(b)) and (not s._surrounds_member(c)):
            # FIXME: maybe we can do better here and to not fix the stride to 1
            # FIXME: found the first common integer for more precision
            return StridedInterval(lower_bound=c, upper_bound=b, bits=w, stride=1).complement
        # otherwise
        return StridedInterval.empty(w)

    @staticmethod
    def top(bits, name=None, uninitialized=False):
        """
        Get a TOP StridedInterval.

        :return:
        """
        return StridedInterval(
            name=name,
            bits=bits,
            stride=1,
            lower_bound=0,
            upper_bound=StridedInterval.max_int(bits),
            uninitialized=uninitialized,
        )

    @staticmethod
    def empty(bits):
        return StridedInterval(bits=bits, bottom=True)

    @staticmethod
    def _wrapped_cardinality(x, y, bits):
        """
        Return the cardinality for a set of number (| x, y |) on the wrapped-interval domain.

        :param x: The first operand (an integer)
        :param y: The second operand (an integer)
        :return: The cardinality
        """

        if x == ((y + 1) % (2**bits)):
            return 2**bits

        return ((y - x) + 1) & (2**bits - 1)

    @staticmethod
    def _is_msb_zero(v, bits):
        """
        Checks if the most significant bit is zero (i.e. is the integer positive
        under signed arithmetic).

        :param v: The integer to check with
        :param bits: Bits of the integer
        :return: True or False
        """
        return (v & (2**bits - 1)) & (2 ** (bits - 1)) == 0

    @staticmethod
    def _is_msb_one(v, bits):
        """
        Checks if the most significant bit is one (i.e. is the integer negative
        under signed arithmetic).

        :param v: The integer to check with
        :param bits: Bits of the integer
        :return: True or False
        """
        return not StridedInterval._is_msb_zero(v, bits)

    @staticmethod
    def _get_msb(v, bits):
        """
        Get the MSB (most significant bit).

        :param v: The integer
        :param bits: Bits of the integer
        :return: the MSB
        """
        if StridedInterval._is_msb_zero(v, bits):
            return 0
        return 1

    @staticmethod
    def _unsigned_to_signed(v, bits):
        """
        Convert an unsigned integer to a signed integer.

        :param v: The unsigned integer
        :param bits: How many bits this integer should be
        :return: The converted signed integer
        """
        if StridedInterval._is_msb_zero(v, bits):
            return v
        return -(2**bits - v)

    @staticmethod
    def _wrapped_overflow_add(a, b):
        """
        Determines if an overflow happens during the addition of `a` and `b`.

        :param a: The first operand (StridedInterval)
        :param b: The other operand (StridedInterval)
        :return: True if overflows, False otherwise
        """

        if a.is_integer and a.lower_bound == 0:
            # Special case: if `a` or `b` is a zero
            card_self = 0
        else:
            card_self = StridedInterval._wrapped_cardinality(a.lower_bound, a.upper_bound, a.bits)

        if b.is_integer and b.lower_bound == 0:
            # Special case: if `a` or `b` is a zero
            card_b = 0
        else:
            card_b = StridedInterval._wrapped_cardinality(b.lower_bound, b.upper_bound, b.bits)

        return (card_self + card_b) > (StridedInterval.max_int(a.bits) + 1)

    @staticmethod
    def _wrapped_overflow_sub(a, b):
        """
        Determines if an overflow happens during the subtraction of `a` and `b`.

        :param a: The first operand (StridedInterval)
        :param b: The other operand (StridedInterval)
        :return: True if overflows, False otherwise
        """

        return StridedInterval._wrapped_overflow_add(a, b)

    @staticmethod
    def _wrapped_unsigned_mul(a, b):
        """
        Perform wrapped unsigned multiplication on two StridedIntervals.

        :param a: The first operand (StridedInterval)
        :param b: The second operand (StridedInterval)
        :return: The multiplication result
        """
        if a.bits != b.bits:
            log.warning("Signed mul: two parameters have different bit length")

        bits = max(a.bits, b.bits)
        lb = a.lower_bound * b.lower_bound
        ub = a.upper_bound * b.upper_bound
        uninit_flag = a.uninitialized | b.uninitialized

        if (ub - lb) < (2**bits):
            if b.is_integer:
                # Multiplication with an integer, and it does not overflow!
                stride = abs(a.stride * b.lower_bound)
            elif a.is_integer:
                stride = abs(a.lower_bound * b.stride)
            else:
                stride = math.gcd(a.stride, b.stride)
            return StridedInterval(
                bits=bits,
                stride=stride,
                lower_bound=lb,
                upper_bound=ub,
                uninitialized=uninit_flag,
            )
        # Overflow occurred
        return StridedInterval.top(bits, uninitialized=False)

    @staticmethod
    def _wrapped_signed_mul(a, b):
        """
        Perform wrapped signed multiplication on two StridedIntervals.

        :param a: The first operand (StridedInterval)
        :param b: The second operand (StridedInterval)
        :return: The product
        """

        # NOTE: interval here should never straddle poles
        # FIXME: add assert to be sure of it!

        if a.bits != b.bits:
            log.warning("Signed mul: two parameters have different bit length")

        bits = max(a.bits, b.bits)

        # shorter SI
        a_lb_positive = StridedInterval._is_msb_zero(a.lower_bound, bits)
        a_ub_positive = StridedInterval._is_msb_zero(a.upper_bound, bits)
        b_lb_positive = StridedInterval._is_msb_zero(b.lower_bound, bits)
        b_ub_positive = StridedInterval._is_msb_zero(b.upper_bound, bits)
        uninit_flag = a.uninitialized | b.uninitialized

        if b.is_integer:
            if b_lb_positive:
                stride = abs(a.stride * b.lower_bound)
            else:
                # if the number is negative we have to get its value first
                stride = abs(a.stride * StridedInterval._unsigned_to_signed(b.lower_bound, bits))
        elif a.is_integer:
            if a_lb_positive:
                stride = abs(b.stride * a.lower_bound)
            else:
                # if the number is negative we have to get its value first:
                stride = abs(b.stride * StridedInterval._unsigned_to_signed(a.lower_bound, bits))
        else:
            stride = math.gcd(a.stride, b.stride)

        if a_lb_positive and a_ub_positive and b_lb_positive and b_ub_positive:
            # [2, 5] * [10, 20] = [20, 100]
            lb = a.lower_bound * b.lower_bound
            ub = a.upper_bound * b.upper_bound

            if ub - lb < (2**bits):
                return StridedInterval(
                    bits=bits, stride=stride, lower_bound=lb, upper_bound=ub, uninitialized=uninit_flag
                )
            return StridedInterval.top(bits, uninitialized=uninit_flag)

        if not a_lb_positive and not a_ub_positive and not b_lb_positive and not b_ub_positive:
            # [-5, -2] * [-20, -10] = [20, 100]
            lb = StridedInterval._unsigned_to_signed(a.upper_bound, bits) * StridedInterval._unsigned_to_signed(
                b.upper_bound, bits
            )
            ub = StridedInterval._unsigned_to_signed(a.lower_bound, bits) * StridedInterval._unsigned_to_signed(
                b.lower_bound, bits
            )

            if ub - lb < (2**bits):
                return StridedInterval(
                    bits=bits, stride=stride, lower_bound=lb, upper_bound=ub, uninitialized=uninit_flag
                )
            return StridedInterval.top(bits, uninitialized=uninit_flag)

        if not a_lb_positive and not a_ub_positive and b_lb_positive and b_ub_positive:
            # [-10, -2] * [2, 5] = [-50, -4]
            lb = StridedInterval._unsigned_to_signed(a.lower_bound, bits) * b.upper_bound
            ub = StridedInterval._unsigned_to_signed(a.upper_bound, bits) * b.lower_bound
            # since the intervals do not straddle the poles, ub is greater than lb
            if ub - lb < (2**bits):
                lb &= 2**bits - 1
                ub &= 2**bits - 1
                return StridedInterval(
                    bits=bits, stride=stride, lower_bound=lb, upper_bound=ub, uninitialized=uninit_flag
                )
            return StridedInterval.top(bits, uninitialized=uninit_flag)

        if a_lb_positive and a_ub_positive and not b_lb_positive and not b_ub_positive:
            # [2, 10] * [-5, -2] = [-50, -4]
            lb = a.upper_bound * StridedInterval._unsigned_to_signed(b.lower_bound, bits)
            ub = a.lower_bound * StridedInterval._unsigned_to_signed(b.upper_bound, bits)
            # since the intervals do not straddle the poles, ub is greater than lb
            if ub - lb < (2**bits):
                lb &= 2**bits - 1
                ub &= 2**bits - 1
                return StridedInterval(
                    bits=bits, stride=stride, lower_bound=lb, upper_bound=ub, uninitialized=uninit_flag
                )
            return StridedInterval.top(bits, uninitialized=uninit_flag)

        raise ClaripyVSAError(f"We shouldn't see this case: {a} * {b}")

    @staticmethod
    def _wrapped_unsigned_div(a, b):
        """
        Perform wrapped unsigned division on two StridedIntervals.

        :param a: The dividend (StridedInterval)
        :param b: The divisor (StridedInterval)
        :return: The quotient
        """

        bits = max(a.bits, b.bits)

        divisor_lb, divisor_ub = b.lower_bound, b.upper_bound
        uninit_flag = a.uninitialized | b.uninitialized

        # Make sure divisor_lb and divisor_ub is not 0
        if divisor_lb == 0:
            # Can we increment it?
            if divisor_ub == 0:
                # We can't :-(
                return StridedInterval.empty(bits)
            divisor_lb += 1
        # If divisor_ub is 0, decrement it to get last but one element
        if divisor_ub == 0:
            divisor_ub = (divisor_ub - 1) & (2**bits - 1)

        lb = a.lower_bound // divisor_ub
        ub = a.upper_bound // divisor_lb

        # TODO: Can we make a more precise estimate of the stride?
        stride = 1

        return StridedInterval(
            bits=bits,
            stride=stride,
            lower_bound=lb,
            upper_bound=ub,
            uninitialized=uninit_flag,
        )

    @staticmethod
    def _wrapped_signed_div(a, b):
        """
        Perform wrapped unsigned division on two StridedIntervals.

        :param a: The dividend (StridedInterval)
        :param b: The divisor (StridedInterval)
        :return: The quotient
        """

        bits = max(a.bits, b.bits)

        # Make sure the divisor is not 0
        divisor_lb = b.lower_bound
        divisor_ub = b.upper_bound
        uninit_flag = a.uninitialized | b.uninitialized

        if divisor_lb == 0:
            # Try to increment it
            if divisor_ub == 0:
                return StridedInterval.empty(bits)
            divisor_lb = 1
        # If divisor_ub is 0, decrement it to get last but one element
        if divisor_ub == 0:
            divisor_ub = (divisor_ub - 1) & (2**bits - 1)

        dividend_positive = StridedInterval._is_msb_zero(a.lower_bound, bits)
        divisor_positive = StridedInterval._is_msb_zero(b.lower_bound, bits)

        # TODO: Can we make a more precise estimate of the stride?
        stride = 1
        if dividend_positive and divisor_positive:
            # They are all positive numbers!
            lb = a.lower_bound // divisor_ub
            ub = a.upper_bound // divisor_lb

        elif dividend_positive and not divisor_positive:
            # + / -
            lb = a.upper_bound // StridedInterval._unsigned_to_signed(divisor_ub, bits)
            ub = a.lower_bound // StridedInterval._unsigned_to_signed(divisor_lb, bits)

        elif not dividend_positive and divisor_positive:
            # - / +
            lb = StridedInterval._unsigned_to_signed(a.lower_bound, bits) // divisor_lb
            ub = StridedInterval._unsigned_to_signed(a.upper_bound, bits) // divisor_ub

        else:
            # - / -
            lb = StridedInterval._unsigned_to_signed(a.upper_bound, bits) // StridedInterval._unsigned_to_signed(
                b.lower_bound, bits
            )
            ub = StridedInterval._unsigned_to_signed(a.lower_bound, bits) // StridedInterval._unsigned_to_signed(
                b.upper_bound, bits
            )

        return StridedInterval(
            bits=bits,
            stride=stride,
            lower_bound=lb,
            upper_bound=ub,
            uninitialized=uninit_flag,
        )

    #
    # Membership testing and poset ordering
    #

    @staticmethod
    def _lex_lte(x, y, bits):
        """
        Lexicographical LTE comparison

        :param x: The first operand (integer)
        :param y: The second operand (integer)
        :param bits: bit-width of the operands
        :return: True or False
        """

        return (x & (2**bits - 1)) <= (y & (2**bits - 1))

    @staticmethod
    def _lex_lt(x, y, bits):
        """
        Lexicographical LT comparison

        :param x: The first operand (integer)
        :param y: The second operand (integer)
        :param bits: bit-width of the operands
        :return: True or False
        """

        return (x & (2**bits - 1)) < (y & (2**bits - 1))

    def _surrounds_member(self, v):
        s = self
        return self._lex_lte(v - s.lower_bound, s.upper_bound - s.lower_bound, s.bits)

    def _is_surrounded(self, b):
        """
        Perform a wrapped LTE comparison only considering the SI bounds

        :param a: The first operand
        :param b: The second operand
        :return: True if a <= b, False otherwise
        """

        a = self
        if a.is_empty:
            return True

        if a.is_top and b.is_top:
            return True

        if a.is_top:
            return False

        if b.is_top:
            return True

        return bool(
            b._surrounds_member(a.lower_bound)
            and b._surrounds_member(a.upper_bound)
            and (
                (b.lower_bound == a.lower_bound and b.upper_bound == a.upper_bound)
                or not a._surrounds_member(b.lower_bound)
                or not a._surrounds_member(b.upper_bound)
            )
        )

    #
    # Arithmetic operations
    #

    @reversed_processor
    def neg(self):
        """
        Unary operation: neg

        :return: 0 - self
        """

        si = StridedInterval(bits=self.bits, stride=0, lower_bound=0, upper_bound=0).sub(self)
        si.uninitialized = self.uninitialized
        return si

    @normalize_types
    def add(self, b):
        """
        Binary operation: add

        :param b: The other operand
        :return: self + b
        """
        new_bits = max(self.bits, b.bits)

        # TODO: Some improvements can be made here regarding the following case
        # TODO: SI<16>0xff[0x0, 0xff] + 3
        # TODO: In current implementation, it overflows, but it doesn't have to

        # optimization
        # case: SI<16>0xff[0x0, 0xff] + 3
        # if self.is_top and b.is_integer:
        #     si = self.copy()
        #     si.lower_bound = b.lower_bound
        #     return si
        # elif b.is_top and self.is_integer:
        #     si = b.copy()
        #     si.lower_bound = self.lower_bound
        #     return si
        # FIXME

        overflow = self._wrapped_overflow_add(self, b)
        if overflow:
            return StridedInterval.top(self.bits)

        lb = self._modular_add(self.lower_bound, b.lower_bound, new_bits)
        ub = self._modular_add(self.upper_bound, b.upper_bound, new_bits)

        # Is it initialized?
        uninitialized = self.uninitialized or b.uninitialized

        # Take the GCD of two operands' strides
        stride = math.gcd(self.stride, b.stride)

        return StridedInterval(
            bits=new_bits, stride=stride, lower_bound=lb, upper_bound=ub, uninitialized=uninitialized
        ).normalize()

    @normalize_types
    def sub(self, b):
        """
        Binary operation: sub

        :param b: The other operand
        :return: self - b
        """
        new_bits = max(self.bits, b.bits)

        overflow = self._wrapped_overflow_sub(self, b)
        if overflow:
            return StridedInterval.top(self.bits)

        lb = self._modular_sub(self.lower_bound, b.upper_bound, new_bits)
        ub = self._modular_sub(self.upper_bound, b.lower_bound, new_bits)

        # Is it initialized?
        uninitialized = self.uninitialized or b.uninitialized

        # Take the GCD of two operands' strides
        stride = math.gcd(self.stride, b.stride)

        return StridedInterval(
            bits=new_bits, stride=stride, lower_bound=lb, upper_bound=ub, uninitialized=uninitialized
        ).normalize()

    @normalize_types
    def mul(self, o):
        """
        Binary operation: multiplication

        :param o: The other operand
        :return: self * o
        """
        if self.is_integer and o.is_integer:
            # Two integers!
            a, b = self.lower_bound, o.lower_bound
            ret = StridedInterval(bits=self.bits, stride=0, lower_bound=a * b, upper_bound=a * b)

            if a * b > (2**self.bits - 1):
                log.warning("Overflow in multiplication detected.")

            return ret.normalize()

        # All other cases

        # Cut from both north pole and south pole
        si1_psplit = self._psplit()
        si2_psplit = o._psplit()
        all_resulting_intervals = list(
            chain(
                *[
                    self._wrapped_unsigned_mul(si1, si2)._multi_valued_intersection(self._wrapped_signed_mul(si1, si2))
                    for si1, si2 in product(si1_psplit, si2_psplit)
                ]
            )
        )

        return StridedInterval.least_upper_bound(*all_resulting_intervals).normalize()

    @normalize_types
    def sdiv(self, o):
        """
        Binary operation: signed division

        :param o: The divisor
        :return: (self / o) in signed arithmetic
        """
        # TODO: copy the code from wrapped interval
        splitted_dividends = self._psplit()
        splitted_divisors = o._psplit()

        resulting_intervals = set()
        for dividend in splitted_dividends:
            for divisor in splitted_divisors:
                tmp = self._wrapped_signed_div(dividend, divisor)
                resulting_intervals.add(tmp)

        return StridedInterval.least_upper_bound(*resulting_intervals).normalize()

    @normalize_types
    def udiv(self, o):
        """
        Binary operation: unsigned division

        :param o: The divisor
        :return: (self / o) in unsigned arithmetic
        """
        # FIXME: copy the code fromm wrapped interval
        splitted_dividends = self._ssplit()
        splitted_divisors = o._ssplit()

        resulting_intervals = set()
        for dividend in splitted_dividends:
            for divisor in splitted_divisors:
                tmp = self._wrapped_unsigned_div(dividend, divisor)
                resulting_intervals.add(tmp)

        return StridedInterval.least_upper_bound(*resulting_intervals).normalize()

    # FIXME: preserve uninitialized flag?
    @reversed_processor
    def bitwise_not(self):
        """
        Unary operation: bitwise not

        :return: ~self
        """
        splitted_si = self._ssplit()
        if len(splitted_si) == 0:
            return StridedInterval.empty(self.bits)

        result_interval = []
        for si in splitted_si:
            lb = ~si.upper_bound
            ub = ~si.lower_bound
            stride = self.stride

            tmp = StridedInterval(bits=self.bits, stride=stride, lower_bound=lb, upper_bound=ub)
            result_interval.append(tmp)

        si = StridedInterval.least_upper_bound(*result_interval).normalize()
        # preserve the uninitialized flag
        si.uninitialized = self.uninitialized
        return si

    @normalize_types
    def bitwise_or(self, t):
        """
        Binary operation: logical or

        :param b: The other operand
        :return: self | b

        This implementation combines the approaches used by 'WYSINWYX: what you see is not what you execute'
        paper and 'Signedness-Agnostic Program Analysis: Precise Integer Bounds for Low-Level Code'. The
        first paper provides an sound way to approximate the stride, whereas the second provides a way
        to calculate the or operation using wrapping intervals.
        Note that, even though according Warren's work 'Hacker's delight', one should follow different
        approaches to calculate the minimun/maximum values of an or operations according on the type
        of the operands (signed/unsigned). On the other other hand, by splitting the wrapping-intervals
        at the south pole, we can safely and soundly only use the Warren's functions for unsigned
        integers.
        """
        s = self
        result_interval = []

        for u in s._ssplit():
            for v in t._ssplit():
                w = u.bits
                # u |w v
                if u.is_integer:
                    s_t = StridedInterval._ntz(v.stride)
                elif v.is_integer:
                    s_t = StridedInterval._ntz(u.stride)
                else:
                    s_t = min(StridedInterval._ntz(u.stride), StridedInterval._ntz(v.stride))

                if u.is_integer and u.lower_bound == 0:
                    new_stride = v.stride
                elif v.is_integer and v.lower_bound == 0:
                    new_stride = u.stride
                else:
                    new_stride = 2**s_t
                mask = (1 << s_t) - 1
                r = (u.lower_bound & mask) | (v.lower_bound & mask)
                m = (2**w) - 1
                low_bound = min_or(
                    u.lower_bound & (~mask & m),
                    u.upper_bound & (~mask & m),
                    v.lower_bound & (~mask & m),
                    v.upper_bound & (~mask & m),
                    w,
                )
                upper_bound = max_or(
                    u.lower_bound & (~mask & m),
                    u.upper_bound & (~mask & m),
                    v.lower_bound & (~mask & m),
                    v.upper_bound & (~mask & m),
                    w,
                )
                if low_bound == upper_bound:
                    new_stride = 0

                new_interval = StridedInterval(
                    lower_bound=((low_bound & (~mask & m)) | r),
                    upper_bound=((upper_bound & (~mask & m)) | r),
                    bits=w,
                    stride=new_stride,
                )
                result_interval.append(new_interval)

        return StridedInterval.least_upper_bound(*result_interval).normalize()

    @normalize_types
    def bitwise_and(self, t):
        """
        Binary operation: logical and

        :param b: The other operand
        :return:

        The following code implements the and operations as presented in the paper
        'Signedness-Agnostic Program Analysis: Precise Integer Bounds for Low-Level Code'
        """
        s = self

        def number_of_ones(n):
            ctr = 0
            while n > 0:
                ctr += 1
                n &= n - 1
            return ctr

        # Optimization: if one of the two intervals is an integer and contains only one one we can be precise
        for a, b in [[s, t], [t, s]]:
            if a.is_integer and number_of_ones(a.lower_bound) == 1 and a.lower_bound == (1 << (t.bits - 1)):
                # It's testing the sign bit
                stride = 1 << (a.bits - 1)
                if b.is_integer:
                    if b.lower_bound == stride:
                        return StridedInterval(bits=b.bits, stride=0, lower_bound=stride, upper_bound=stride)
                    return StridedInterval(bits=b.bits, stride=0, lower_bound=0, upper_bound=0)
                is_sol = (
                    a.lower_bound - b.lower_bound
                ) % b.stride == 0 and b.lower_bound <= a.lower_bound <= b.upper_bound
                if is_sol:
                    return StridedInterval(bits=b.bits, stride=stride, lower_bound=0, upper_bound=stride)
                return StridedInterval(bits=b.bits, stride=0, lower_bound=0, upper_bound=0)
            # FIXME: implement case only one 1 not in first position

        # paper's and
        new_interval = s.bitwise_not().bitwise_or(t.bitwise_not()).bitwise_not()
        return new_interval.normalize()

    @normalize_types
    def bitwise_xor(self, t):
        """
        Operation xor

        :param t:   The other operand.
        """

        # Using same variables as in paper
        s = self
        new_interval = (
            (s.bitwise_not().bitwise_or(t)).bitwise_not().bitwise_or(s.bitwise_or(t.bitwise_not()).bitwise_not())
        )
        return new_interval.normalize()

    def _pre_shift(self, shift_amount):
        def get_range(expr):
            """
            Get the range of bits for shifting

            :param expr:
            :return: A tuple of maximum and minimum bits to shift
            """

            def round(max, x):  # pylint:disable=redefined-builtin
                if x < 0 or x > max:
                    return max
                return x

            if isinstance(expr, numbers.Number):
                return (expr, expr)

            assert type(expr) is StridedInterval

            if expr.is_integer:
                return (round(self.bits, expr.lower_bound), round(self.bits, expr.lower_bound))
            if expr.lower_bound < 0:
                if expr.upper_bound >= 0:
                    return (0, self.bits)
                return (self.bits, self.bits)
            return (round(self.bits, self.lower_bound), round(self.bits, self.upper_bound))

        lower, upper = get_range(shift_amount)
        # TODO: Is trancating necessary?

        return lower, upper

    @reversed_processor
    def rshift_logical(self, shift_amount):
        """
        Logical shift right.

        :param StridedInterval shift_amount: The amount of shifting
        :return: The shifted StridedInterval
        :rtype: StridedInterval
        """

        lower, upper = self._pre_shift(shift_amount)

        # Shift the lower_bound and upper_bound by all possible amounts, and union all possible results

        ret = None

        for amount in range(lower, upper + 1):
            si_ = self._rshift_logical(amount)

            ret = si_ if ret is None else ret.union(si_)

        if ret is None:
            return StridedInterval.top(self.bits)

        ret.normalize()
        ret.uninitialized = self.uninitialized
        return ret

    def _unrev_rshift_logical(self, shift_amount):
        """
        Logical shift right.

        :param StridedInterval shift_amount: The amount of shifting
        :return: The shifted StridedInterval
        :rtype: StridedInterval
        """

        lower, upper = self._pre_shift(shift_amount)

        # Shift the lower_bound and upper_bound by all possible amounts, and union all possible results

        ret = None

        for amount in range(lower, upper + 1):
            si_ = self._rshift_logical(amount)

            ret = si_ if ret is None else ret.union(si_)

        if ret is None:
            return StridedInterval.top(self.bits)

        ret.normalize()
        ret.uninitialized = self.uninitialized
        return ret

    @reversed_processor
    def rshift_arithmetic(self, shift_amount):
        """
        Arithmetic shift right.

        :param StridedInterval shift_amount: The amount of shifting
        :return: The shifted StridedInterval
        :rtype: StridedInterval
        """

        lower, upper = self._pre_shift(shift_amount)

        # Shift the lower_bound and upper_bound by all possible amounts, and union all possible results

        ret = None

        for amount in range(lower, upper + 1):
            si_ = self._rshift_arithmetic(amount)

            ret = si_ if ret is None else ret.union(si_)

        if ret is None:
            return StridedInterval.top(self.bits)

        ret.normalize()
        ret.uninitialized = self.uninitialized
        return ret

    @reversed_processor
    def lshift(self, shift_amount):
        lower, upper = self._pre_shift(shift_amount)

        # Shift the lower_bound and upper_bound by all possible amounts, and
        # get min/max values from all the resulting values

        new_lower_bound = None
        new_upper_bound = None
        for amount in range(lower, upper + 1):
            lower_shifted = self.lower_bound << amount
            if new_lower_bound is None or lower_shifted < new_lower_bound:
                new_lower_bound = lower_shifted
            upper_shifted = self.upper_bound << amount
            if new_upper_bound is None or upper_shifted > new_upper_bound:
                new_upper_bound = upper_shifted

        # NOTE: If this is an arithmetic operation, we should take care
        # of sign-changes.

        ret = StridedInterval(
            bits=self.bits,
            stride=max(self.stride << lower, 1),
            lower_bound=new_lower_bound,
            upper_bound=new_upper_bound,
            uninitialized=self.uninitialized,
        )
        ret.normalize()

        return ret

    @reversed_processor
    def cast_low(self, tok):
        assert tok <= self.bits

        mask = (1 << tok) - 1

        if self.stride >= (1 << tok):
            log.warning("Tried to cast_low an interval to an interval shorter than its stride.")

        if tok == self.bits:
            return self.copy()

        # the interval can be represented in tok bits
        if (self.lower_bound & mask) == self.lower_bound and (self.upper_bound & mask) == self.upper_bound:
            return StridedInterval(
                bits=tok,
                stride=self.stride,
                lower_bound=self.lower_bound,
                upper_bound=self.upper_bound,
                uninitialized=self.uninitialized,
            )

        # the range between lower bound and upper bound can be represented
        # in the new SI
        if 0 <= (self.upper_bound - self.lower_bound) <= mask:
            lower = self.lower_bound & mask
            upper = self.upper_bound & mask
            return StridedInterval(
                bits=tok, stride=self.stride, lower_bound=lower, upper_bound=upper, uninitialized=self.uninitialized
            )

        if (self.upper_bound & mask == self.lower_bound & mask) and ((self.upper_bound - self.lower_bound) & mask == 0):
            # This operation doesn't affect the stride. Stride should be 0 then.

            bound = self.lower_bound & mask

            return StridedInterval(
                bits=tok, stride=0, lower_bound=bound, upper_bound=bound, uninitialized=self.uninitialized
            )

        ntz = StridedInterval._ntz(self.stride)

        if tok > ntz:
            new_lower = self.lower_bound & ((2**ntz) - 1)
            stride = 2**ntz
            ret = self.top(tok, uninitialized=self.uninitialized)
            ret.stride = stride
            ret.lower_bound = new_lower
            k = (ret.upper_bound - ret.lower_bound) // ret.stride
            ret.upper_bound = ret.stride * k + ret.lower_bound
        else:
            ret = StridedInterval(
                bits=tok,
                stride=0,
                lower_bound=(self.lower_bound & ((2**tok) - 1)),
                upper_bound=(self.upper_bound & ((2**tok) - 1)),
            )
        return ret

    def _unrev_cast_low(self, tok):
        assert tok <= self.bits

        mask = (1 << tok) - 1

        if self.stride >= (1 << tok):
            log.warning("Tried to cast_low an interval to a an interval shorter than its stride.")

        if tok == self.bits:
            return self.copy()

        # the interval can be represented in tok bits
        if (self.lower_bound & mask) == self.lower_bound and (self.upper_bound & mask) == self.upper_bound:
            return StridedInterval(
                bits=tok,
                stride=self.stride,
                lower_bound=self.lower_bound,
                upper_bound=self.upper_bound,
                uninitialized=self.uninitialized,
            )

        # the range between lower bound and upper bound can be represented
        # in the new SI
        if self.upper_bound - self.lower_bound <= mask:
            lower = self.lower_bound & mask
            upper = self.upper_bound & mask
            # Keep the signs!
            if self.lower_bound < 0:
                # how this should happen ?
                log.warning("Lower bound values is less than 0")
                lower = StridedInterval._to_negative(lower, tok)
            if self.upper_bound < 0:
                # how this should happen ?
                log.warning("Upper bound value is less than 0")
                upper = StridedInterval._to_negative(upper, tok)
            return StridedInterval(
                bits=tok, stride=self.stride, lower_bound=lower, upper_bound=upper, uninitialized=self.uninitialized
            )

        if (self.upper_bound & mask == self.lower_bound & mask) and ((self.upper_bound - self.lower_bound) & mask == 0):
            # This operation doesn't affect the stride. Stride should be 0 then.

            bound = self.lower_bound & mask

            return StridedInterval(
                bits=tok, stride=0, lower_bound=bound, upper_bound=bound, uninitialized=self.uninitialized
            )

        # TODO: How can we do better here? For example, keep the stride information?
        return self.top(tok, uninitialized=self.uninitialized)

    @normalize_types
    def concat(self, b):
        # Zero-extend
        a = self.nameless_copy()
        a._bits += b.bits

        new_si = a.lshift(b.bits)
        new_b = b.copy()
        # Zero-extend b
        new_b._bits = new_si.bits

        if new_si.is_integer:
            # We can be more precise!
            new_si._bits = new_b.bits
            new_si._stride = new_b.stride
            new_si._lower_bound = new_si.lower_bound + b.lower_bound
            new_si._upper_bound = new_si.upper_bound + b.upper_bound
            return new_si
        return new_si.bitwise_or(new_b)

    @reversed_processor
    def extract(self, high_bit, low_bit):
        assert low_bit >= 0

        bits = high_bit - low_bit + 1

        ret = self.rshift_logical(low_bit) if low_bit != 0 else self.copy()
        if bits != self.bits:
            ret = ret.cast_low(bits)

        ret.uninitialized = self.uninitialized
        return ret.normalize()

    def _unrev_extract(self, high_bit, low_bit):
        assert low_bit >= 0

        bits = high_bit - low_bit + 1

        ret = self._unrev_rshift_logical(low_bit) if low_bit != 0 else self.copy()
        if bits != self.bits:
            ret = ret._unrev_cast_low(bits)

        ret.uninitialized = self.uninitialized
        return ret.normalize()

    @reversed_processor
    def agnostic_extend(self, new_length):
        """
        Unary operation: SignExtend

        :param new_length: New length after sign-extension
        :return: A new StridedInterval

        In a sign-agnostic implementation of strided-intervals a number can be
        signed or unsigned both. Given a SI, we must pay attention how we extend
        its lower bound and upper bound. Assuming that the lower bound is in
        the left emishpere (positive number). Let's assume first that the SI is
        signed and its upper bound is in the right emisphere. Extending it with
        leading 1s (i.e., its MSB) is correct given that its values would be
        preserved. On the other hand if the number is unsigned we should not
        replicate its MSB, since this would increase the value of the upper
        bound in the new interval. In this case the correct approach would be to
        add 0 in front of the number, i.e., moving it to the left emisphere.
        But this approach wouldn't be correct in the first scenario (signed SI).
        The solution in this case is extend the upper bound with 1s. This gives
        us an overapproximation of the original SI.

        Extending this intuition, the implementation follows the below rules:
        (UB: upper bound, LB: lower bound, RE: right emisphere, LE: left emisphere)
        1* UB:LE and LB:LE: add leading 0s (sound).
        2* UB:RE and LB:RE and the LB is closer to the north pole: add leading 0s to LB and leading 1s to the UB (sound)
        3* UB:RE and LB:RE and UB is closer to the north pole: add leading 1s to LB and UB both (sound).
        4* UB:LE and LB:RE: add leading 0s to UB and leading 0s to LB (sound).
        5* UB:RE and LB:LE: add leading 0s to LB and leading 1s to UB (sound).
        6* UB:RE and LB:RE and LB = UB: add leading 0s to LB and 1s to UB and add stride from LB to UB ****
        """

        si = self.copy()
        si._bits = new_length

        leading_1_lb = False
        leading_1_ub = False

        ub_msb = self._get_msb(self.upper_bound, self.bits)
        lb_msb = self._get_msb(self.lower_bound, self.bits)
        # the only one which chages the stride
        case_6 = False

        # LB:RE cases
        if lb_msb == 1:
            # 2
            if ub_msb == 1 and self.upper_bound > self.lower_bound:
                leading_1_ub = True
            # 3
            if ub_msb == 1 and self.lower_bound > self.upper_bound:
                leading_1_ub = True
                leading_1_lb = True
            # 6
            if ub_msb == 1 and self.lower_bound == self.upper_bound:
                leading_1_ub = True
                case_6 = True
        # 5
        elif ub_msb == 1:
            leading_1_ub = True

        if leading_1_lb:
            mask = (2**new_length - 1) - (2**self.bits - 1)
            si._lower_bound |= mask
        if leading_1_ub:
            mask = (2**new_length - 1) - (2**self.bits - 1)
            si._upper_bound |= mask
        if case_6:
            si.stride = si.upper_bound - si.lower_bound

        return si

    @reversed_processor
    def zero_extend(self, new_length):
        """
        Unary operation: ZeroExtend

        :param new_length: New length after zero-extension
        :return: A new StridedInterval
        """
        si = self.copy()
        si._bits = new_length

        return si

    @reversed_processor
    def sign_extend(self, new_length):
        """
        Unary operation: SignExtend

        :param new_length: New length after sign-extension
        :return: A new StridedInterval
        """

        msb = self.extract(self.bits - 1, self.bits - 1).eval(2)
        if msb == [0]:
            # All positive numbers
            return self.zero_extend(new_length)
        if msb == [1]:
            # All negative numbers
            si = self.copy()
            si._bits = new_length
            mask = (2**new_length - 1) - (2**self.bits - 1)
            si._lower_bound |= mask
            si._upper_bound |= mask

        else:
            # Both positive numbers and negative numbers
            nums = self._nsplit()
            # Since there are both positive and negative numbers, there must be two bounds after nsplit
            # assert len(numbers) == 2

            all_resulting_intervals = []

            assert len(nums) > 0

            for n in nums:
                a, b = n.lower_bound, n.upper_bound
                mask_a = 0
                mask_b = 0
                mask_n = ((1 << (new_length - n.bits)) - 1) << n.bits

                if StridedInterval._get_msb(a, n.bits) == 1:
                    mask_a = mask_n
                if StridedInterval._get_msb(b, n.bits) == 1:
                    mask_b = mask_n

                si_ = StridedInterval(bits=new_length, stride=n.stride, lower_bound=a | mask_a, upper_bound=b | mask_b)
                all_resulting_intervals.append(si_)
            si = StridedInterval.least_upper_bound(*all_resulting_intervals).normalize()

        si.uninitialized = self.uninitialized
        return si

    @normalize_types
    def union(self, b):
        """
        The union operation. It might return a DiscreteStridedIntervalSet to allow for better precision in analysis.

        :param b: Operand
        :return: A new DiscreteStridedIntervalSet, or a new StridedInterval.
        """

        if not _allow_dsis_flag:
            return StridedInterval.least_upper_bound(self, b)

        if (
            self.cardinality > vsa.DEFAULT_MAX_CARDINALITY_WITHOUT_COLLAPSING
            or b.cardinality > vsa.DEFAULT_MAX_CARDINALITY_WITHOUT_COLLAPSING
        ):
            return StridedInterval.least_upper_bound(self, b)

        dsis = vsa.DiscreteStridedIntervalSet(bits=self._bits, si_set={self})
        return dsis.union(b)

    @staticmethod
    def _bigger(interval1, interval2):
        """
        Return interval with bigger cardinality
        Refer Section 3.1

        :param interval1: first interval
        :param interval2: second interval
        :return: Interval or interval2 whichever has greater cardinality
        """
        if interval2.cardinality > interval1.cardinality:
            return interval2.copy()
        return interval1.copy()

    @staticmethod
    def _ntz(x):
        """
        Get the number of consecutive zeros
        :param x:
        :return:
        """
        if x == 0:
            return 0
        y = (~x) & (x - 1)  # There is actually a bug in BAP until 0.8

        def bits(y):
            n = 0
            while y != 0:
                n += 1
                y >>= 1
            return n

        return bits(y)

    @staticmethod
    def least_upper_bound(*intervals_to_join):
        """
        Pseudo least upper bound.
        Join the given set of intervals into a big interval. The resulting strided interval is the one which in
        all the possible joins of the presented SI, presented the least number of values.

        The number of joins to compute is linear with the number of intervals to join.

        Draft of proof:
        Considering  three generic SI (a,b, and c) ordered from their lower bounds, such that
        a.lower_bund <= b.lower_bound <= c.lower_bound, where <= is the lexicographic less or equal.
        The only joins which have sense to compute are:
        * a U b U c
        * b U c U a
        * c U a U b

        All the other combinations fall in either one of these cases. For example: b U a U c does not make make sense
        to be calculated. In fact, if one draws this union, the result is exactly either (b U c U a) or (a U b U c) or
        (c U a U b).
        :param intervals_to_join: Intervals to join
        :return: Interval that contains all intervals
        """
        assert len(intervals_to_join) > 0, "No intervals to join"
        # Check if all intervals are of same width
        all_same = all(x.bits == intervals_to_join[0].bits for x in intervals_to_join)
        assert all_same, "All intervals to join should be same"

        # Optimization: If we have only one interval, then return that interval as result
        if len(intervals_to_join) == 1:
            return intervals_to_join[0].copy()
        # Optimization: If we have only two intervals, the pseudo-join is fine and more precise
        if len(intervals_to_join) == 2:
            return StridedInterval.pseudo_join(intervals_to_join[0], intervals_to_join[1])

        # sort the intervals in increasing left bound
        sorted_intervals = sorted(intervals_to_join, key=lambda x: x.lower_bound)
        # Fig 3 of the paper
        ret = None

        # we try all possible joins (linear with the number of SI to join)
        # and we return the one with the least number of values.
        for i in range(len(sorted_intervals)):
            # let's join all of them
            si = reduce(
                lambda x, y: StridedInterval.pseudo_join(x, y, False), sorted_intervals[i:] + sorted_intervals[0:i]
            )

            if ret is None or ret.n_values > si.n_values:
                ret = si

        if any(x for x in intervals_to_join if x.uninitialized):
            ret.uninitialized = True

        return ret

    @normalize_types
    def _union(self, b):
        # FIXME: to remove
        # this function is here only for retro compatibility with the other parts of angr
        return StridedInterval.pseudo_join(self, b)

    @staticmethod
    def pseudo_join(s, b, smart_join=True):
        """
        It two intervals in a way that the resulting SI is the one that has the least
        SI cardinality (i.e., which represents the least number of elements) possible if the smart_join flag is enabled,
        otherwise it just joins the SI according the order they are passed to the function.

        The pseudo-join operation is not associative in wrapping intervals (please refer to section 3.1 paper
        'Signedness-Agnostic Program Analysis: Precise Integer Bounds for Low-Level Code'), Therefore the join of three
        WI may  give us different results according on the order we join them. All of the results will be sound, though.

        Please use the function least_upper_bound as a stub.

        :param s:           The first SI
        :param b:           The other SI.
        :param smart_join:  Enable the smart join behavior. If this flag is set, this function joins the two SI in a way
                            that the resulting Si has least number of elements (more precise). If it is unset, this
                            function will join the two SI according on the order they are passed to the function.
        :return:            A new StridedInterval
        """

        assert s.bits == b.bits
        w = s.bits

        if s._reversed != b._reversed:
            log.warning("Incoherent reversed flag between operands %s and %s", s, b)

        uninit_flag = s.uninitialized | b.uninitialized

        #
        # Trivial cases
        #

        if s.is_empty:
            return b
        if b.is_empty:
            return s

        if s.is_integer and b.is_integer:
            upper = max(s.upper_bound, b.upper_bound) if smart_join else b.upper_bound
            lower = min(s.lower_bound, b.lower_bound) if smart_join else s.lower_bound
            stride = abs(upper - lower)
            return StridedInterval(
                bits=w, stride=stride, lower_bound=lower, upper_bound=upper, uninitialized=uninit_flag
            )

        #
        # Other cases
        #

        if s._is_surrounded(b):
            # Containment: s <= b
            new_stride = StridedInterval.gcd(s.stride, b.stride) if s.is_interval else b.stride
            new_stride = StridedInterval.gcd(new_stride, s._modular_sub(s.lower_bound, b.lower_bound, w))
            return StridedInterval(
                bits=w,
                stride=new_stride,
                lower_bound=b.lower_bound,
                upper_bound=b.upper_bound,
                uninitialized=uninit_flag,
            )

        if b._is_surrounded(s):
            # Containment: b <= s
            # TODO: This case is missing in the original implementation. Is that a bug?
            new_stride = StridedInterval.gcd(s.stride, b.stride) if b.is_interval else s.stride
            new_stride = StridedInterval.gcd(new_stride, s._modular_sub(b.lower_bound, s.lower_bound, w))
            return StridedInterval(
                bits=w,
                stride=new_stride,
                lower_bound=s.lower_bound,
                upper_bound=s.upper_bound,
                uninitialized=uninit_flag,
            )

        if (
            s._surrounds_member(b.lower_bound)
            and s._surrounds_member(b.upper_bound)
            and b._surrounds_member(s.lower_bound)
            and b._surrounds_member(s.upper_bound)
        ):
            # The union of them covers the entire sphere
            return StridedInterval.top(w, uninitialized=uninit_flag)

        if s._surrounds_member(b.lower_bound):
            # Overlapping. Nor s or b are integer here.
            # We return the join with less values
            new_stride = StridedInterval.gcd(s.stride, b.stride)
            new_stride = StridedInterval.gcd(new_stride, s._modular_sub(b.lower_bound, s.lower_bound, w))
            return StridedInterval(
                bits=w,
                stride=new_stride,
                lower_bound=s.lower_bound,
                upper_bound=b.upper_bound,
                uninitialized=uninit_flag,
            )
        if b._surrounds_member(s.lower_bound):
            # Overlapping. Nor s or b are integer here.
            # We return the join with less values
            new_stride = StridedInterval.gcd(s.stride, b.stride)
            new_stride = StridedInterval.gcd(new_stride, s._modular_sub(s.lower_bound, b.lower_bound, w))
            return StridedInterval(
                bits=w,
                stride=new_stride,
                lower_bound=b.lower_bound,
                upper_bound=s.upper_bound,
                uninitialized=uninit_flag,
            )
        # no overlapping.
        # we join the two intervals according on the order they are given
        if not smart_join:
            if s.is_integer:
                new_stride = StridedInterval.gcd(b.stride, s._modular_sub(b.lower_bound, s.lower_bound, w))
            elif b.is_integer:
                new_stride = StridedInterval.gcd(s.stride, s._modular_sub(b.lower_bound, s.lower_bound, w))
            else:
                new_stride = StridedInterval.gcd(s.stride, b.stride)
                new_stride = StridedInterval.gcd(
                    new_stride, StridedInterval._wrapped_cardinality(s.lower_bound, b.lower_bound, w) - 1
                )
            return StridedInterval(
                bits=w,
                stride=new_stride,
                lower_bound=s.lower_bound,
                upper_bound=b.upper_bound,
                uninitialized=uninit_flag,
            )

        # Else: smart join.
        # we return the join which produce an interval with the least number of values
        if s.is_integer:
            new_stride = b.stride
        elif b.is_integer:
            new_stride = s.stride
        else:
            new_stride = StridedInterval.gcd(s.stride, b.stride)

        # from b to s
        new_stride1 = StridedInterval.gcd(
            new_stride, StridedInterval._wrapped_cardinality(b.lower_bound, s.lower_bound, w) - 1
        )
        # from s to b
        new_stride2 = StridedInterval.gcd(
            new_stride, StridedInterval._wrapped_cardinality(s.lower_bound, b.lower_bound, w) - 1
        )
        si1 = StridedInterval(
            bits=w,
            stride=new_stride1,
            lower_bound=b.lower_bound,
            upper_bound=s.upper_bound,
            uninitialized=uninit_flag,
        )
        si2 = StridedInterval(
            bits=w,
            stride=new_stride2,
            lower_bound=s.lower_bound,
            upper_bound=b.upper_bound,
            uninitialized=uninit_flag,
        )

        if si1.n_values <= si2.n_values:
            return si1
        return si2

    @staticmethod
    def _minimal_common_integer(si_0, si_1):
        """
        Calculates the minimal integer that appears in both StridedIntervals.
        As a wrapper method of _minimal_common_integer_splitted(), this method takes arbitrary StridedIntervals.
        For more information, please refer to the comment of _minimal_common_integer_splitted().

        :param si_0:   the first StridedInterval
        :type  si_0:   StridedInterval
        :param si_1:   the second StridedInterval
        :type  si_1:   StridedInterval

        :return: the minimal common integer, or None if there is no common integer
        """

        si_0_splitted = si_0._ssplit()
        si_1_splitted = si_1._ssplit()

        len_0, len_1 = len(si_0_splitted), len(si_1_splitted)

        if len_0 == 1 and len_1 == 2:
            # Swap them so we don't have to handle dual
            si_0_splitted, si_1_splitted = si_1_splitted, si_0_splitted
            len_0, len_1 = len_1, len_0

        if len_0 == 1 and len_1 == 1:
            # No splitting was necessary
            return StridedInterval._minimal_common_integer_splitted(si_0, si_1)

        if len_0 == 2 and len_1 == 1:
            int_0 = StridedInterval._minimal_common_integer_splitted(si_0_splitted[0], si_1_splitted[0])
            int_1 = StridedInterval._minimal_common_integer_splitted(si_0_splitted[1], si_1_splitted[0])

        else:
            # len_0 == 2 and len_1 == 2
            int_0 = StridedInterval._minimal_common_integer_splitted(si_0_splitted[0], si_1_splitted[0])
            int_1 = StridedInterval._minimal_common_integer_splitted(si_0_splitted[1], si_1_splitted[1])

        if int_0 is None:
            return int_1
        if int_1 is None:
            return int_0
        return int_0

    @staticmethod
    def extended_euclid(a, b):
        """
        It calculates the GCD of a and b, and two values x and y such that:
        a*x + b*y = GCD(a,b).
        This code has been taken from the project sympy.

        :param a: first integer
        :param b: second integer
        :return: x,y and the GCD of a and b
        """
        if b == 0:
            return (1, 0, a)
        x0, y0, d = StridedInterval.extended_euclid(b, a % b)
        x, y = y0, x0 - (a // b) * y0
        return x, y, d

    @staticmethod
    def sign(a):
        return -1 if a < 0 else 1

    @staticmethod
    def igcd(a, b):
        """
        :param a: First integer
        :param b: Second integer
        :return: the integer GCD between a and b
        """
        a = round(a)
        b = round(b)
        if b < 0:
            b = -b
        while b:
            a, b = b, a % b
        if a == 1 or b == 1:
            return 1
        return a

    @staticmethod
    def diop_natural_solution_linear(c, a, b):
        """
        It finds the fist natural solution of the diophantine equation
        a*x + b*y = c. Some lines of this code are taken from the project
        sympy.

        :param c: constant
        :param a: quotient of x
        :param b: quotient of y
        :return: the first natural solution of the diophatine equation
        """

        def get_intersection(a, b, a_dir, b_dir):
            # Do the intersection between two
            # ranges.
            if (a_dir, b_dir) == (">=", ">="):
                lb = max(b, a)
                ub = float("inf")
            elif (a_dir, b_dir) == ("<=", ">="):
                if a > b:
                    lb = b
                    ub = a
                else:
                    lb = None
                    ub = None
            elif (a_dir, b_dir) == (">=", "<="):
                if b > a:
                    lb = a
                    ub = b
                else:
                    lb = None
                    ub = None
            elif (a_dir, b_dir) == ("<=", "<="):
                ub = min(b, a)
                lb = float("-inf")
            else:
                raise ClaripyOperationError("Invalid direction")

            return lb, ub

        d = StridedInterval.igcd(a, StridedInterval.igcd(b, c))
        a = a // d
        b = b // d
        c = c // d

        if c == 0:
            return (0, 0)
        x0, y0, d = StridedInterval.extended_euclid(int(abs(a)), int(abs(b)))
        x0 = x0 * StridedInterval.sign(a)
        y0 = y0 * StridedInterval.sign(b)

        if c % d == 0:
            # Integer solutions are: (c*x0 + b*t, c*y0 - a*t)
            # we have to get the first positive solution, which means
            # that we have to solve the following disequations for t:
            # c*x0 + b*t >= 0 and c*y0 - a*t >= 0.
            assert b != 0
            assert a != 0

            t0 = (-c * x0) / float(b)
            t1 = (c * y0) / float(a)
            # direction of the disequation depends on b and a sign
            t0_dir = "<=" if b < 0 else ">="
            t1_dir = ">=" if a < 0 else "<="

            # calculate the intersection between the found
            # solution intervals to get the common solutions
            # for t.
            lb, ub = get_intersection(t0, t1, t0_dir, t1_dir)

            # Given that we are looking for the first value
            # which solve the diophantine equation, we have to
            # select the value of t closer to 0.
            if lb <= 0 <= ub:
                t = ub if abs(ub) < abs(lb) else lb
            elif lb == float("inf") or lb == float("-inf"):
                t = ub
            elif ub == float("inf") or ub == float("-inf"):
                t = lb
            else:
                t = ub if abs(ub) < abs(lb) else lb
            # round the value of t
            t = math.floor(t) if t == ub else math.ceil(t)

            return (c * x0 + b * t, c * y0 - a * t)
        return (None, None)

    @staticmethod
    def _minimal_common_integer_splitted(si_0, si_1):
        """
        Calculates the minimal integer that appears in both StridedIntervals.
        It's equivalent to finding an integral solution for equation `ax + b = cy + d` that makes `ax + b` minimal
        si_0.stride, si_1.stride being a and c, and si_0.lower_bound, si_1.lower_bound being b and d, respectively.
        Upper bounds are used to check whether the minimal common integer exceeds the bound or not. None is returned
        if no minimal common integers can be found within the range.

        Some assumptions:
        # - None of the StridedIntervals straddles the south pole. Consequently, we have x <= max_int(si.bits) and y <=
        #   max_int(si.bits)
        # - a, b, c, d are all positive integers
        # - x >= 0, y >= 0

        :param StridedInterval si_0: the first StridedInterval
        :param StridedInterval si_1: the second StrideInterval
        :return: the minimal common integer, or None if there is no common integer
        """

        a, c = si_0.stride, si_1.stride
        b, d = si_0.lower_bound, si_1.lower_bound

        # if any of them is an integer
        if si_0.is_integer:
            if si_1.is_integer:
                return None if si_0.lower_bound != si_1.lower_bound else si_0.lower_bound
            if (
                si_0.lower_bound >= si_1.lower_bound
                and si_0.lower_bound <= si_1.upper_bound
                and (si_0.lower_bound - si_1.lower_bound) % si_1.stride == 0
            ):
                return si_0.lower_bound
            return None
        if si_1.is_integer:
            # pylint: disable=arguments-out-of-order
            return StridedInterval._minimal_common_integer_splitted(si_1, si_0)

        # shortcut
        if si_0.upper_bound < si_1.lower_bound or si_1.upper_bound < si_0.lower_bound:
            # They don't overlap at all
            return None

        if (d - b) % StridedInterval.gcd(a, c) != 0:
            # They don't overlap
            return None

        # Given two strided intervals a = sa[lba, uba] and b = sb[lbb, ubb], the first integer shared
        # by them is found by finding the minimum values of ka and kb which solve the equation:
        #     ka * sa + lba = kb * sb + lbb
        # In particular one can solve the above diophantine equation and find the parameterized solutions
        # of ka and kb, with respect to a parameter t.
        # The minimum natural value of the parameter t which gives two positive natural values of ka and kb
        # is used to resolve ka and kb, and finally to solve the above equation and get the minimum shared integer.
        x, y = StridedInterval.diop_natural_solution_linear(-(b - d), a, -c)
        if a is None or b is None:
            return None
        first_integer = x * a + b
        assert first_integer == y * c + d
        if (
            si_0.lower_bound <= first_integer <= si_0.upper_bound
            and si_1.lower_bound <= first_integer <= si_1.upper_bound
        ):
            return first_integer

        return None

    @normalize_types
    def intersection(self, b):
        intersection = self._multi_valued_intersection(b)
        v = intersection[0]
        if len(intersection) == 2:
            v = StridedInterval.pseudo_join(v, intersection[1])

        return v

    @normalize_types
    def _multi_valued_intersection(self, b):
        if self.is_empty or b.is_empty:
            return (StridedInterval.empty(self.bits),)

        assert self.bits == b.bits
        if self.is_integer and b.is_integer:
            if self.lower_bound == b.lower_bound:
                # They are the same number!
                ret = (
                    StridedInterval(
                        bits=self.bits, stride=0, lower_bound=self.lower_bound, upper_bound=self.lower_bound
                    ),
                )
            else:
                ret = (StridedInterval.empty(self.bits),)

        elif self.is_integer:
            integer = self.lower_bound
            if (b.lower_bound - integer) % b.stride == 0 and b._surrounds_member(integer):
                ret = (StridedInterval(bits=self.bits, stride=0, lower_bound=integer, upper_bound=integer),)
            else:
                ret = (StridedInterval.empty(self.bits),)

        elif b.is_integer:
            integer = b.lower_bound
            if (integer - self.lower_bound) % self.stride == 0 and self._surrounds_member(integer):
                ret = (StridedInterval(bits=self.bits, stride=0, lower_bound=integer, upper_bound=integer),)
            else:
                ret = (StridedInterval.empty(self.bits),)

        else:
            # None of the operands is an integer
            # Note that this is not a faithful implementation of the WI paper, rather it is based on WrappedMeet() in
            # wrapped-intervals:lib/RangeAnalysis/WrappedRange.cpp . Please see wrapped-intervals on GitHub at
            # https://github.com/sav-tools/wrapped-intervals
            new_stride = self.lcm(self.stride, b.stride)
            if self._is_surrounded(b):
                # Containment case
                # `b` may fully contain `self`

                lb = StridedInterval._minimal_common_integer(self, b)
                if lb is None:
                    ret = (StridedInterval.empty(self.bits),)
                else:
                    ub = self._modular_add(
                        self._modular_sub(self.upper_bound, lb, self.bits) // new_stride * new_stride, lb, self.bits
                    )
                    ret = (StridedInterval(bits=self.bits, stride=new_stride, lower_bound=lb, upper_bound=ub),)

            elif b._is_surrounded(self):
                # Containment case 2
                # `self` contains `b`

                lb = StridedInterval._minimal_common_integer(self, b)

                if lb is None:
                    ret = (StridedInterval.empty(self.bits),)
                else:
                    ub = self._modular_add(
                        self._modular_sub(b.upper_bound, lb, self.bits) // new_stride * new_stride, lb, self.bits
                    )
                    ret = (StridedInterval(bits=self.bits, stride=new_stride, lower_bound=lb, upper_bound=ub),)

            elif (
                self._surrounds_member(b.lower_bound)
                and self._surrounds_member(b.upper_bound)
                and b._surrounds_member(self.lower_bound)
                and b._surrounds_member(self.upper_bound)
            ):
                # One cover the other

                # bounds of the two common intervals
                # among the SIs
                lb_s0 = self.lower_bound
                ub_s0 = b.upper_bound
                lb_s1 = b.lower_bound
                ub_s1 = self.upper_bound

                # Let's build the SIs
                s0 = StridedInterval(bits=self.bits, lower_bound=lb_s0, upper_bound=ub_s0, stride=self.stride)
                s1 = StridedInterval(bits=self.bits, lower_bound=lb_s1, upper_bound=ub_s1, stride=b.stride)
                # and find the first common integer
                lb_s0_new = StridedInterval._minimal_common_integer(s0, b)
                lb_s1_new = StridedInterval._minimal_common_integer(s1, self)

                if lb_s0_new is None:
                    s0 = StridedInterval.empty(self.bits)
                else:
                    ub_s0_new = self._modular_add(
                        self._modular_sub(ub_s0, lb_s0_new, self.bits) // new_stride * new_stride, lb_s0_new, self.bits
                    )
                    s0 = StridedInterval(
                        bits=self.bits, lower_bound=lb_s0_new, upper_bound=ub_s0_new, stride=new_stride
                    )

                if lb_s1_new is None:
                    s1 = StridedInterval.empty(self.bits)
                else:
                    ub_s1_new = self._modular_add(
                        self._modular_sub(ub_s1, lb_s1_new, self.bits) // new_stride * new_stride, lb_s1_new, self.bits
                    )
                    s1 = StridedInterval(
                        bits=self.bits, lower_bound=lb_s1_new, upper_bound=ub_s1_new, stride=new_stride
                    )

                ret = (s0, s1)

            # here we have four cases since the overlapping depends also on the stride
            elif self._surrounds_member(b.lower_bound):
                # Overlapping case 1a

                lb = StridedInterval._minimal_common_integer(b, self)

                if lb is None:
                    ret = (StridedInterval.empty(self.bits),)
                else:
                    ub = self._modular_add(
                        self._modular_sub(self.upper_bound, lb, self.bits) // new_stride * new_stride, lb, self.bits
                    )
                    ret = (StridedInterval(bits=self.bits, stride=new_stride, lower_bound=lb, upper_bound=ub),)

            elif self._surrounds_member(b.upper_bound):
                # Overlapping case 1b

                lb = StridedInterval._minimal_common_integer(b, self)

                if lb is None:
                    ret = (StridedInterval.empty(self.bits),)
                else:
                    ub = self._modular_add(
                        self._modular_sub(b.upper_bound, lb, self.bits) // new_stride * new_stride, lb, self.bits
                    )
                    ret = (StridedInterval(bits=self.bits, stride=new_stride, lower_bound=lb, upper_bound=ub),)

            elif b._surrounds_member(self.lower_bound):
                # Overlapping case 2a

                lb = StridedInterval._minimal_common_integer(self, b)

                if lb is None:
                    ret = (StridedInterval.empty(self.bits),)

                else:
                    ub = self._modular_add(
                        self._modular_sub(b.upper_bound, lb, self.bits) // new_stride * new_stride, lb, self.bits
                    )
                    ret = (StridedInterval(bits=self.bits, stride=new_stride, lower_bound=lb, upper_bound=ub),)

            elif b._surrounds_member(self.upper_bound):
                # Overlapping case 2b

                lb = StridedInterval._minimal_common_integer(self, b)

                if lb is None:
                    ret = (StridedInterval.empty(self.bits),)

                else:
                    ub = self._modular_add(
                        self._modular_sub(self.upper_bound, lb, self.bits) // new_stride * new_stride, lb, self.bits
                    )
                    ret = (StridedInterval(bits=self.bits, stride=new_stride, lower_bound=lb, upper_bound=ub),)

            else:
                # Disjoint case
                ret = (StridedInterval.empty(self.bits),)

        return tuple(r.normalize() for r in ret)

    @normalize_types
    def widen(self, b):
        ret = None

        if self.is_empty and not b.is_empty:
            ret = StridedInterval.top(bits=self.bits)

        elif self.is_empty:
            ret = b

        elif b.is_empty:
            ret = self

        else:
            new_stride = StridedInterval.gcd(self.stride, b.stride)
            lower = (
                StridedInterval.lower(self.bits, self.lower_bound, new_stride)
                if b.lower_bound < self.lower_bound
                else self.lower_bound
            )
            upper = (
                StridedInterval.upper(self.bits, self.upper_bound, new_stride)
                if b.upper_bound > self.upper_bound
                else self.upper_bound
            )
            if new_stride == 0:
                if self.is_integer and b.is_integer:
                    ret = StridedInterval(bits=self.bits, stride=1, lower_bound=lower, upper_bound=upper)
                else:
                    raise ClaripyOperationError("SI: operands are not reduced.")
            else:
                ret = StridedInterval(bits=self.bits, stride=new_stride, lower_bound=lower, upper_bound=upper)

        ret.normalize()
        return ret

    def reverse(self):
        """
        This is a delayed reversing function. All it really does is to invert the _reversed property of this
        StridedInterval object.

        :return: None
        """
        if self.bits == 8:
            # We cannot reverse a one-byte value
            return self

        si = self.copy()
        si._reversed = not si._reversed

        return si

    def _reverse(self):
        """
        This method reverses the StridedInterval object for real. Do expect loss of precision for most cases!

        This reverse operation is unsound and incomplete, but allows the reverse operation to be......

        :return: A new reversed StridedInterval instance
        """
        o = self.copy()
        # Clear ok reversed flag
        o._reversed = not o._reversed

        if o.bits == 8:
            # No need for reversing
            return o.copy()

        if o.is_top:
            # A TOP is still a TOP after reversing
            return o.copy()

        if not o.is_integer:
            # We really don't want to do that... but well, sometimes it just happens...
            log.warning("Reversing a real strided-interval %s is bad", self)

        # Reversing an integer is easy
        rounded_bits = ((o.bits + 7) // 8) * 8
        list_bytes = []
        si = None

        for i in range(0, rounded_bits, 8):
            b = o._unrev_extract(min(i + 7, o.bits - 1), i)
            list_bytes.append(b)

        for b in list_bytes:
            si = b if si is None else si.concat(b)
        si.uninitialized = self.uninitialized
        si._reversed = o._reversed
        return si

    def _involuted_reverse(self):
        """
        This method reverses the StridedInterval object for real. Do expect loss of precision for most cases!

        :return: A new reversed StridedInterval instance
        """

        def inv_is_top(si):
            return si.stride == 1 and self._lower_bound == StridedInterval._modular_add(self._upper_bound, 1, self.bits)

        o = self.copy()
        # Clear the reversed flag
        o._reversed = not o._reversed

        if o.bits == 8:
            # No need for reversing
            return o.copy()

        if inv_is_top(o):
            # A TOP is still a TOP after reversing
            return o.copy()

        lb = o._lower_bound
        ub = o._upper_bound

        rounded_bits = ((o.bits + 7) // 8) * 8
        lb_r = []
        ub_r = []

        for i in range(0, rounded_bits, 8):
            if i != 0:
                lb = lb >> 8
                ub = ub >> 8

            lb_r.append(lb & 0xFF)
            ub_r.append(ub & 0xFF)

        si_lb = None
        si_ub = None
        for b in lb_r:
            if si_lb is None:
                si_lb = b
            else:
                si_lb <<= 8
                si_lb |= b
        for b in ub_r:
            if si_ub is None:
                si_ub = b
            else:
                si_ub <<= 8
                si_ub |= b

        si = StridedInterval(
            bits=o.bits, lower_bound=si_lb, upper_bound=si_ub, stride=o._stride, uninitialized=o.uninitialized
        )

        si._reversed = o._reversed
        if not o.is_integer:
            # We really don't want to do that... but well, sometimes it just happens...
            log.warning("Reversing a real strided-interval %s is bad", self)

        return si
