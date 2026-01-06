from __future__ import annotations

import functools
import itertools
import numbers

from claripy.backends.backend_concrete.bv import BVV

from .bool_result import BoolResult
from .errors import ClaripyVSAOperationError
from .strided_interval import StridedInterval
from .valueset import ValueSet

DEFAULT_MAX_CARDINALITY_WITHOUT_COLLAPSING = 256  # We don't collapse until there are more than this many SIs


def apply_on_each_si(f):
    @functools.wraps(f)
    def operator(self, o=None):
        if o is None:
            # This is an unary operator.
            new_si_set = set()

            for a in self._si_set:
                new_si_set.add(getattr(a, f.__name__)())

            ret = DiscreteStridedIntervalSet(bits=self.bits, si_set=new_si_set)
            return ret.normalize()

        if isinstance(o, DiscreteStridedIntervalSet):
            # We gotta apply the operation on each single object
            new_si_set = set()

            for a in self._si_set:
                for b in o._si_set:
                    new_si_set.add(getattr(a, f.__name__)(b))

            ret = DiscreteStridedIntervalSet(bits=self.bits, si_set=new_si_set)
            return ret.normalize()

        if isinstance(o, StridedInterval | numbers.Number | BVV):
            new_si_set = set()
            for si in self._si_set:
                new_si_set.add(getattr(si, f.__name__)(o))

            ret = DiscreteStridedIntervalSet(bits=self.bits, si_set=new_si_set)
            return ret.normalize()

        raise ClaripyVSAOperationError(f"Unsupported operand type {type(o)}")

    return operator


def convert_operand_to_si(f):
    @functools.wraps(f)
    def converter(self, o):
        if isinstance(o, BVV):
            o = o.value
        if isinstance(o, numbers.Number):
            o = StridedInterval(bits=self.bits, stride=0, lower_bound=o, upper_bound=o)

        return f(self, o)

    return converter


def collapse_operand(f):
    @functools.wraps(f)
    def collapser(self, o):
        if isinstance(o, DiscreteStridedIntervalSet):
            return f(self, o.collapse())
        return f(self, o)

    return collapser


dsis_id_ctr = itertools.count()


class DiscreteStridedIntervalSet(StridedInterval):
    """
    A DiscreteStridedIntervalSet represents one or more discrete StridedInterval instances.
    """

    def __init__(self, name=None, bits=0, si_set=None, max_cardinality=None):
        if name is None:
            name = f"DSIS_{next(dsis_id_ctr)}"

        # Initialize the set for strided intervals
        if si_set is not None and len(si_set):
            self._si_set = si_set

        else:
            self._si_set = set()

        self._max_cardinality = (
            DEFAULT_MAX_CARDINALITY_WITHOUT_COLLAPSING if max_cardinality is None else max_cardinality
        )

        StridedInterval.__init__(self, name=name, bits=bits)

        # Update lower_bound and upper_bound
        for si in self._si_set:
            self._update_bounds(si)
            self._update_bits(si)

    #
    # Properties
    #

    def __repr__(self):
        representatives = ", ".join([i.__repr__() for i in list(self._si_set)[:5]])
        if self.number_of_values > 5:
            representatives += ", ..."

        return f"{self._name}<{self._bits}>({self.number_of_values}){{{representatives}}}"

    @property
    def cardinality(self):
        """
        This is an over-approximation of the cardinality of this DSIS.

        :return:
        """
        cardinality = 0
        for si in self._si_set:
            cardinality += si.cardinality
        return cardinality

    @property
    def number_of_values(self):
        return len(self._si_set)

    @property
    def stride(self):
        return self.collapse().stride

    #
    # Special methods
    #

    def should_collapse(self):
        return self.cardinality > self._max_cardinality

    def collapse(self):
        """
        Collapse into a StridedInterval instance.

        :return: A new StridedInterval instance.
        """

        if self.cardinality:
            r = None
            for si in self._si_set:
                r = r._union(si) if r is not None else si

            return r

        # This is an empty StridedInterval...
        return StridedInterval.empty(self._bits)

    def normalize(self):
        """
        Return the collapsed object if ``should_collapse()`` is True, otherwise return self.

        :return: A DiscreteStridedIntervalSet object.
        """
        if self.should_collapse():
            return self.collapse()
        if self.number_of_values == 1:
            return next(iter(self._si_set))
        for si in self._si_set:
            self._update_bits(si)
        return self

    def copy(self):
        return DiscreteStridedIntervalSet(
            bits=self._bits, si_set=self._si_set.copy(), max_cardinality=self._max_cardinality
        )

    def __hash__(self):
        return id(self)  # ...not sure how to do this. these objects are mutable.

    #
    # Operations
    #

    # Logical operations

    @convert_operand_to_si
    @collapse_operand
    def __eq__(self, o):
        """
        Operation ==

        :param o:   The other operand.
        :return:    An instance of BoolResult.
        """

        return self.collapse() == o

    @convert_operand_to_si
    @collapse_operand
    def __ne__(self, o):
        """
        Operation !=

        :param o:   The other operand.
        :return:    An instance of BoolResult.
        """

        return self.collapse() != o

    @convert_operand_to_si
    @collapse_operand
    def UGT(self, o):
        """
        Operation >

        :param o:   The other operand.
        :return:    An instance of BoolResult.
        """

        return self.collapse() > o

    @convert_operand_to_si
    @collapse_operand
    def ULE(self, o):
        """
        Operation <=

        :param o:   The other operand.
        :return:    An instance of BoolResult.
        """

        return self.collapse() <= o

    @convert_operand_to_si
    @collapse_operand
    def ULT(self, o):
        """
        Operation <

        :param o:   The other operand.
        :return:    An instance of BoolResult.
        """
        return self.collapse() < o

    @convert_operand_to_si
    @collapse_operand
    def UGE(self, o):
        """
        Operation >=

        :param o:   The other operand.
        :return:    An instance of BoolResult.
        """

        return self.collapse() >= o

    # Bitwise operations

    @convert_operand_to_si
    @apply_on_each_si
    def __and__(self, o):
        """
        Operation &

        :param o:   The other operand.
        :return:    An instance of DiscreteStridedIntervalSet.
        """

    def __rand__(self, o):
        return self.__and__(o)

    @convert_operand_to_si
    @apply_on_each_si
    def __or__(self, o):
        """
        Operation |

        :param o:   The other operand.
        :return:    An instance of DiscreteStridedIntervalSet.
        """

    def __ror__(self, o):
        return self.__or__(o)

    @convert_operand_to_si
    @apply_on_each_si
    def __xor__(self, o):
        """
        Operation ^

        :param o:   The other operand.
        :return:    An instance of DiscreteStridedIntervalSet.
        """

    def __rxor__(self, o):
        return self.__xor__(o)

    def __neg__(self):
        """
        Operation ~

        :return: The negated value.
        """
        new_si_set = set()
        for si in self._si_set:
            new_si_set.add(~si)

        r = DiscreteStridedIntervalSet(bits=self._bits, si_set=new_si_set)
        return r.normalize()

    def __invert__(self):
        """
        Operation ~

        :return: The negated value.
        """
        return self.__neg__()

    @apply_on_each_si
    def __lshift__(self, o):
        """
        Operation <<

        :param o:   The other operand.
        :return:    An instance of DiscreteStridedIntervalSet.
        """

    @apply_on_each_si
    def __rshift__(self, o):
        """
        Operation >>

        :param o:   The other operand.
        :return:    An instance of DiscreteStridedIntervalSet.
        """

    @apply_on_each_si
    def concat(self, b):
        """
        Operation concat

        :param b:   The other operand to concatenate with.
        :return:    The concatenated value.
        """

    def extract(self, high_bit, low_bit):
        """
        Operation extract

        :param high_bit:    The highest bit to begin extraction.
        :param low_bit:     The lowest bit to end extraction.
        :return:            Extracted bits.
        """
        # TODO: This method can be optimized

        ret = set()
        bits = high_bit - low_bit + 1

        for si in self._si_set:
            ret.add(si.extract(high_bit, low_bit))

        if len(ret) > 1:
            return DiscreteStridedIntervalSet(bits=bits, si_set=ret)

        return next(iter(ret))

    # Arithmetic operations

    @convert_operand_to_si
    @apply_on_each_si
    def __add__(self, o):
        """
        Operation +

        :param o:   The other operand.
        :return:
        """

    def __radd__(self, o):
        return self.__add__(o)

    @convert_operand_to_si
    @apply_on_each_si
    def __sub__(self, o):
        """
        Operation -

        :param o:   The other operand.
        :return:
        """

    def __rsub__(self, o):
        return self.__sub__(o)

    @convert_operand_to_si
    @apply_on_each_si
    def __floordiv__(self, o):
        """
        Operation /

        :param o:   The other operand.
        :return:
        """

    def __truediv__(self, o):
        return self.__floordiv__(o)  # floats not welcome

    def __rfloordiv__(self, o):
        return self.__floordiv__(o)

    def __rtruediv__(self, o):
        return self.__rfloordiv__(o)

    @convert_operand_to_si
    @apply_on_each_si
    def __mod__(self, o):
        """
        Operation %

        :param o:   The other operand.
        :return:
        """

    def __rmod__(self, o):
        return self.__mod__(o)

    # Evaluation

    def eval(self, n, signed=False):
        """

        :param n:
        :param signed:
        :return:
        """

        # FIXME: "signed" is silently ignored now

        ret = set()

        for si in self._si_set:
            ret |= set(si.eval(n))
            if len(ret) >= n:
                break

        return list(ret)[:n]

    # Set operations

    def union(self, b):
        if isinstance(b, DiscreteStridedIntervalSet):
            return self._union_with_dsis(b)

        if isinstance(b, StridedInterval):
            if not b.is_empty:
                return self._union_with_si(b)
            return self.copy()

        if isinstance(b, ValueSet):
            return b.union(self)

        raise ClaripyVSAOperationError(f"Unsupported operand type {type(b)} for operation union.")

    def intersection(self, b):
        if isinstance(b, DiscreteStridedIntervalSet):
            return self._intersection_with_dsis(b)

        if isinstance(b, StridedInterval):
            return self._intersection_with_si(b)

        raise ClaripyVSAOperationError(f"Unsupported operand type {type(b)} for operation intersection.")

    # Other operations

    @apply_on_each_si
    def reverse(self):
        """
        Operation Reverse

        :return: None
        """

    @apply_on_each_si
    def sign_extend(self, new_length):
        """
        Operation SignExt

        :param new_length:  The length to extend to.
        :return:            SignExtended value.
        """

    @apply_on_each_si
    def zero_extend(self, new_length):
        """
        Operation ZeroExt

        :param new_length:  The length to extend to.
        :return:            ZeroExtended value.
        """

    @collapse_operand
    def widen(self, b):
        """
        Widening operator.

        :param b:   The other operand.
        :return:    The widened result.
        """
        return self.collapse().widen(b)

    #
    # Private methods
    #

    def _union_with_si(self, si):
        """
        Union with another StridedInterval.

        :param si:
        :return:
        """

        dsis = self.copy()
        for si_ in dsis._si_set:
            if BoolResult.is_true(si_ == si):
                return dsis

        dsis._si_set.add(si)
        dsis._update_bounds(si)

        return dsis.normalize()

    def _union_with_dsis(self, dsis):
        """
        Union with another DiscreteStridedIntervalSet.

        :param dsis:
        :return:
        """

        copied = self.copy()

        for a in dsis._si_set:
            copied = copied.union(a)

        if isinstance(copied, DiscreteStridedIntervalSet):
            copied._update_bounds(dsis)

        return copied.normalize()

    def _intersection_with_si(self, si):
        """
        Intersection with another :class:`StridedInterval`.

        :param si: The other operand
        :return:
        """

        new_si_set = set()
        for si_ in self._si_set:
            r = si_.intersection(si)
            new_si_set.add(r)

        if len(new_si_set):
            ret = DiscreteStridedIntervalSet(bits=self.bits, si_set=new_si_set)

            if ret.should_collapse():
                return ret.collapse()
            return ret

        # There is no intersection between two operands
        return StridedInterval.empty(self.bits)

    def _intersection_with_dsis(self, dsis):
        """
        Intersection with another :class:`DiscreteStridedIntervalSet`.

        :param dsis:    The other operand.
        :return:
        """

        new_si_set = set()
        for si in dsis._si_set:
            r = self._intersection_with_si(si)

            if isinstance(r, StridedInterval):
                if not r.is_empty:
                    new_si_set.add(r)

            else:  # r is a DiscreteStridedIntervalSet
                new_si_set |= r._si_set

        if len(new_si_set):
            ret = DiscreteStridedIntervalSet(bits=self.bits, si_set=new_si_set)
            return ret.normalize()

        return StridedInterval.empty(self.bits)

    def _update_bounds(self, val):
        if not isinstance(val, StridedInterval):
            raise ClaripyVSAOperationError(f"Unsupported operand type {type(val)}.")

        if val._lower_bound is not None and (self._lower_bound is None or val.lower_bound < self._lower_bound):
            self._lower_bound = val.lower_bound

        if val._upper_bound is not None and (self._upper_bound is None or val.upper_bound > self._upper_bound):
            self._upper_bound = val.upper_bound

    def _update_bits(self, val):
        if not isinstance(val, StridedInterval):
            raise ClaripyVSAOperationError(f"Unsupported operand type {type(val)}.")

        self._bits = val.bits
