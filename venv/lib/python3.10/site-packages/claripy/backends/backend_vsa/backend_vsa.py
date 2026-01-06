from __future__ import annotations

import functools
import logging
import numbers
import operator
from functools import reduce

import claripy
from claripy.annotation import RegionAnnotation, StridedIntervalAnnotation, UninitializedAnnotation
from claripy.ast import BV, Base
from claripy.backends.backend import Backend
from claripy.backends.backend_vsa.balancer import Balancer
from claripy.backends.backend_vsa.errors import ClaripyVSAError
from claripy.errors import BackendError
from claripy.operations import backend_operations_vsa_compliant, expression_set_operations

from .bool_result import BoolResult, FalseResult, TrueResult
from .discrete_strided_interval_set import DiscreteStridedIntervalSet
from .strided_interval import StridedInterval
from .valueset import ValueSet

log = logging.getLogger(__name__)


def arg_filter(f):
    @functools.wraps(f)
    def filter_(*args):
        if isinstance(args[0], numbers.Number):
            raise BackendError(f"Unsupported argument type {type(args[0])}")
        return f(*args)

    return filter_


def normalize_arg_order(f):
    @functools.wraps(f)
    def normalizer(*args):
        if len(args) != 2:
            raise BackendError(f"Unsupported arguments number {len(args)}")

        if not isinstance(args[0], StridedInterval | DiscreteStridedIntervalSet | ValueSet):
            if not isinstance(args[1], StridedInterval | DiscreteStridedIntervalSet | ValueSet):
                raise BackendError("Unsupported arguments")
            args = [args[1], args[0]]

        return f(*args)

    return normalizer


# pylint: disable=too-many-positional-arguments


class BackendVSA(Backend):
    """BackendVSA is a backend that uses VSA (Value Set Analysis) to represent
    and reason about values.
    """

    def __init__(self):
        Backend.__init__(self)
        self._make_expr_ops(set(expression_set_operations), op_class=self)
        self._make_raw_ops(set(backend_operations_vsa_compliant), op_module=BackendVSA)

        self._op_raw["Reverse"] = BackendVSA.Reverse
        self._op_raw["If"] = self.If
        self._op_expr["BVV"] = self.BVV
        self._op_expr["BoolV"] = self.BoolV
        self._op_expr["BVS"] = self.BVS

        # reduceable
        self._op_raw["__add__"] = self._op_add
        self._op_raw["__sub__"] = self._op_sub
        self._op_raw["__mul__"] = self._op_mul
        self._op_raw["__or__"] = self._op_or
        self._op_raw["__xor__"] = self._op_xor
        self._op_raw["__and__"] = self._op_and
        self._op_raw["__mod__"] = self._op_mod

    @staticmethod
    def _op_add(*args):
        return reduce(operator.__add__, args)

    @staticmethod
    def _op_sub(*args):
        return reduce(operator.__sub__, args)

    @staticmethod
    def _op_mul(*args):
        return reduce(operator.__mul__, args)

    @staticmethod
    def _op_or(*args):
        return reduce(operator.__or__, args)

    @staticmethod
    def _op_xor(*args):
        return reduce(operator.__xor__, args)

    @staticmethod
    def _op_and(*args):
        return reduce(operator.__and__, args)

    @staticmethod
    def _op_mod(*args):
        return reduce(operator.__mod__, args)

    def convert(self, expr):
        return Backend.convert(self, claripy.excavate_ite(expr) if isinstance(expr, Base) else expr)

    def _convert(self, r):
        if isinstance(r, numbers.Number):
            return r
        if isinstance(r, bool):
            return TrueResult() if r else FalseResult()
        if isinstance(r, StridedInterval | DiscreteStridedIntervalSet | ValueSet):
            return r
        if isinstance(r, BoolResult):
            return r

        # Not supported
        raise BackendError

    def _abstract(self, e):
        if isinstance(e, numbers.Number):
            return e
        if isinstance(e, StridedInterval):
            if e.is_top:
                return claripy.TSI(e.bits, explicit_name=e.name)
            if e.is_bottom:
                return claripy.ESI(e.bits)
            if e.stride in {0, 1} and e.lower_bound == e.upper_bound:
                return claripy.BVV(e.lower_bound, e.bits)
            return claripy.SI(
                name=e.name,
                bits=e.bits,
                lower_bound=e.lower_bound,
                upper_bound=e.upper_bound,
                stride=e.stride,
            )
        if isinstance(e, ValueSet):
            if len(e.regions) == 0:
                return claripy.VS(bits=e.bits, name=e.name)
            if len(e.regions) == 1:
                region = next(iter(e.regions))
                return claripy.VS(
                    bits=e.bits,
                    region=region,
                    region_base_addr=e._region_base_addrs[region].eval(1)[0] if e._region_base_addrs else 0,
                    value=e.regions[region].eval(1)[0],
                    name=e.name,
                )
            raise ClaripyVSAError("Cannot abstract ValueSet with multiple regions")
        raise BackendError(f"Don't know how to abstract {type(e)}")

    def _eval(self, expr, n, extra_constraints=(), solver=None, model_callback=None):
        if isinstance(expr, StridedInterval | ValueSet):
            return expr.eval(n)
        if isinstance(expr, BoolResult):
            return expr.value
        raise BackendError(f"Unsupported type {type(expr)}")

    def _min(self, expr, extra_constraints=(), signed=False, solver=None, model_callback=None):
        # TODO: signed min
        if isinstance(expr, StridedInterval):
            if expr.is_top:
                # TODO: Return
                return 0

            return expr.min

        if isinstance(expr, ValueSet):
            return expr.min

        raise BackendError(f"Unsupported expr type {type(expr)}")

    def _max(self, expr, extra_constraints=(), signed=False, solver=None, model_callback=None):
        # TODO: signed max
        if isinstance(expr, StridedInterval):
            if expr.is_top:
                # TODO:
                return StridedInterval.max_int(expr.bits)

            return expr.max

        if isinstance(expr, ValueSet):
            return expr.max

        raise BackendError(f"Unsupported expr type {type(expr)}")

    def _solution(self, expr, v, extra_constraints=(), solver=None, model_callback=None):
        if isinstance(expr, BoolResult):
            return len(set(v.value) & set(expr.value)) > 0

        if isinstance(expr, StridedInterval):
            return not expr.intersection(v).is_empty

        if isinstance(expr, ValueSet):
            return any(not si.intersection(v).is_empty for _, si in expr.items())

        raise NotImplementedError(type(expr).__name__)

    def _has_true(self, e, extra_constraints=(), solver=None, model_callback=None):
        return BoolResult.has_true(e)

    def _has_false(self, e, extra_constraints=(), solver=None, model_callback=None):
        return BoolResult.has_false(e)

    def _is_true(self, e, extra_constraints=(), solver=None, model_callback=None):
        return BoolResult.is_true(e)

    def _is_false(self, e, extra_constraints=(), solver=None, model_callback=None):
        return BoolResult.is_false(e)

    #
    # Backend Operations
    #

    def _identical(self, a, b):
        if type(a) != type(b):  # noqa: E721
            return False
        return a.identical(b)

    @staticmethod
    def _unique(obj):
        if isinstance(obj, StridedInterval | ValueSet):
            return obj.unique
        raise BackendError(f"Not supported type of operand {type(obj)}")

    def _cardinality(self, a):
        return a.cardinality

    def name(self, a):
        if isinstance(a, StridedInterval):
            return a.name

        return None

    def apply_annotation(self, o, a):
        """
        Apply an annotation on the backend object.

        :param BackendObject bo: The backend object.
        :param Annotation annotation: The annotation to be applied
        :return: A new BackendObject
        :rtype: BackendObject
        """

        if isinstance(o, StridedInterval):
            if isinstance(a, StridedIntervalAnnotation):
                return StridedInterval(
                    bits=o.bits,
                    stride=a.stride,
                    lower_bound=a.lower_bound,
                    upper_bound=a.upper_bound,
                    name=o.name,
                )

            if isinstance(a, RegionAnnotation):
                offset = o
                if isinstance(offset, numbers.Number):
                    offset = StridedInterval(bits=o.bits, stride=0, lower_bound=offset, upper_bound=offset)
                vs = ValueSet.empty(o.bits)
                if isinstance(offset, StridedInterval):
                    vs._merge_si(a.region_id, a.region_base_addr, offset)
                elif isinstance(offset, ValueSet):
                    for si in offset.regions.values():
                        vs._merge_si(a.region_id, a.region_base_addr, si)
                else:
                    raise ClaripyVSAError(f"Unsupported offset type {type(offset)}")
                return vs

            if isinstance(a, UninitializedAnnotation):
                o2 = o.copy()
                o2.uninitialized = True
                return o2

        if isinstance(o, ValueSet) and isinstance(a, StridedIntervalAnnotation):
            si = StridedInterval(
                bits=o.bits,
                stride=a.stride,
                lower_bound=a.lower_bound,
                upper_bound=a.upper_bound,
                name=o.name,
            )
            vs = o.copy()
            vs._merge_si(a.region_id, a.region_base_addr, si)
            return vs

        if isinstance(o, BoolResult) and isinstance(a, UninitializedAnnotation):
            # TODO: Do we want to do anything here?
            return o

        raise ValueError(f"Unsupported annotation type {type(a)} for object {type(o)}")

    @staticmethod
    def BVV(ast):
        if ast.args[0] is None:
            return StridedInterval.empty(ast.args[1])
        return StridedInterval(bits=ast.args[1], stride=0, lower_bound=ast.args[0], upper_bound=ast.args[0])

    @staticmethod
    def BoolV(ast):
        return TrueResult() if ast.args[0] else FalseResult()

    @staticmethod
    def And(a, *args):
        return reduce(operator.__and__, args, a)

    @staticmethod
    def Not(a):
        return ~a

    @staticmethod
    @normalize_arg_order
    def ULT(a, b):
        return a.ULT(b)

    @staticmethod
    @normalize_arg_order
    def ULE(a, b):
        return a.ULE(b)

    @staticmethod
    @normalize_arg_order
    def UGT(a, b):
        return a.UGT(b)

    @staticmethod
    @normalize_arg_order
    def UGE(a, b):
        return a.UGE(b)

    @staticmethod
    @normalize_arg_order
    def SLT(a, b):
        return a.SLT(b)

    @staticmethod
    @normalize_arg_order
    def SLE(a, b):
        return a.SLE(b)

    @staticmethod
    @normalize_arg_order
    def SGT(a, b):
        return a.SGT(b)

    @staticmethod
    @normalize_arg_order
    def SGE(a, b):
        return a.SGE(b)

    @staticmethod
    def BVS(ast: BV):
        return StridedInterval(name=ast.args[0], bits=ast.size())

    def If(self, cond, t, f):
        if not self.has_true(cond):
            return f
        if not self.has_false(cond):
            return t
        return t.union(f)

    # TODO: Implement other operations!

    @staticmethod
    def Or(*args):
        first = args[0]
        others = args[1:]

        for o in others:
            first = first.union(o)
        return first

    @staticmethod
    def __rshift__(expr, shift_amount):  # pylint:disable=unexpected-special-method-signature
        return expr.__rshift__(shift_amount)

    @staticmethod
    def LShR(expr, shift_amount):
        return expr.LShR(shift_amount)

    @staticmethod
    def Concat(*args):
        ret = None
        for expr in args:
            if not isinstance(expr, StridedInterval | DiscreteStridedIntervalSet | ValueSet):
                raise BackendError(f"Unsupported expr type {type(expr)}")

            ret = ret.concat(expr) if ret is not None else expr

        return ret

    @staticmethod
    def Extract(*args):
        low_bit = args[1]
        high_bit = args[0]
        expr = args[2]

        if not isinstance(expr, StridedInterval | DiscreteStridedIntervalSet | ValueSet):
            raise BackendError(f"Unsupported expr type {type(expr)}")

        return expr.extract(high_bit, low_bit)

    @staticmethod
    def SignExt(*args):
        new_bits = args[0]
        expr = args[1]

        if not isinstance(expr, StridedInterval | DiscreteStridedIntervalSet):
            raise BackendError(f"Unsupported expr type {type(expr)}")

        return expr.sign_extend(new_bits + expr.bits)

    @staticmethod
    def ZeroExt(*args):
        new_bits = args[0]
        expr = args[1]

        if not isinstance(expr, StridedInterval | DiscreteStridedIntervalSet):
            raise BackendError(f"Unsupported expr type {type(expr)}")

        return expr.zero_extend(new_bits + expr.bits)

    @staticmethod
    def Reverse(arg):
        if not isinstance(arg, StridedInterval | DiscreteStridedIntervalSet | ValueSet):
            raise BackendError(f"Unsupported expr type {type(arg)}")

        return arg.reverse()

    def union(self, ast):
        if len(ast.args) != 2:
            raise BackendError(f"Incorrect number of arguments ({len(ast.args)}) passed to BackendVSA.union().")

        converted_0 = self.convert(ast.args[0])
        converted_1 = self.convert(ast.args[1])

        ret = converted_0.union(converted_1)

        if ret is NotImplemented:
            log.debug("Union failed, trying the other way around.")
            ret = converted_1.union(converted_0)

        return ret

    def intersection(self, ast):
        if len(ast.args) != 2:
            raise BackendError(f"Incorrect number of arguments ({len(ast.args)}) passed to BackendVSA.intersection().")

        ret = None

        for arg in ast.args:
            arg = self.convert(arg)
            ret = arg if ret is None else ret.intersection(arg)
        return ret

    def widen(self, ast):
        if len(ast.args) != 2:
            raise BackendError(f"Incorrect number of arguments ({len(ast.args)}) passed to BackendVSA.widen().")

        converted_0 = self.convert(ast.args[0])
        converted_1 = self.convert(ast.args[1])

        ret = converted_0.widen(converted_1)
        if ret is NotImplemented:
            log.debug("Widening failed, trying the other way around.")
            ret = converted_1.widen(converted_0)

        return ret

    @staticmethod
    def CreateTopStridedInterval(bits, name=None):
        return StridedInterval.top(bits, name)

    @staticmethod
    def constraint_to_si(expr):
        return Balancer(expr).compat_ret
