# pylint:disable=duplicate-value,missing-class-docstring
from __future__ import annotations

import logging
import numbers
import operator
from functools import reduce

import claripy
from claripy.ast import BV, Bool
from claripy.backends.backend import Backend
from claripy.backends.backend_concrete import bv, fp, strings
from claripy.errors import BackendError, UnsatError
from claripy.operations import backend_fp_operations, backend_operations, backend_strings_operations

log = logging.getLogger(__name__)


# pylint: disable=too-many-positional-arguments


class BackendConcrete(Backend):
    __slots__ = ()

    def __init__(self):
        Backend.__init__(self)
        self._make_raw_ops(set(backend_operations) - {"If"}, op_module=bv)
        self._make_raw_ops(backend_strings_operations, op_module=strings)
        self._make_raw_ops(backend_fp_operations, op_module=fp)
        self._op_raw["If"] = self._If
        self._op_raw["BVV"] = self.BVV
        self._op_raw["StringV"] = self.StringV
        self._op_raw["FPV"] = self.FPV

        # reduceable
        self._op_raw["__add__"] = self._op_add
        self._op_raw["__sub__"] = self._op_sub
        self._op_raw["__mul__"] = self._op_mul
        self._op_raw["__or__"] = self._op_or
        self._op_raw["__xor__"] = self._op_xor
        self._op_raw["__and__"] = self._op_and

        # unary
        self._op_raw["__invert__"] = self._op_not
        self._op_raw["__neg__"] = self._op_neg
        self._op_raw["fpSqrt"] = self._op_fpSqrt

        # boolean ops
        self._op_raw["And"] = self._op_and
        self._op_raw["Or"] = self._op_or
        self._op_raw["Xor"] = self._op_xor
        self._op_raw["Not"] = self._op_boolnot

        self._cache_objects = False

    @staticmethod
    def BVV(value, size):
        return bv.BVV(value, size)

    @staticmethod
    def StringV(value):
        return strings.StringV(value)

    @staticmethod
    def FPV(op, sort):
        return fp.FPV(op, sort)

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
    def _op_not(arg):
        return ~arg

    @staticmethod
    def _op_neg(arg):
        return -arg

    @staticmethod
    def _op_boolnot(arg):
        return not arg

    @staticmethod
    def _op_fpSqrt(rm, a):  # pylint:disable=unused-argument
        return a.fpSqrt()

    def convert(self, expr):
        """
        Override Backend.convert() to add fast paths for BVVs and BoolVs.
        """
        if type(expr) is BV and expr.op == "BVV":
            cached_obj = self._object_cache.get(expr.hash(), None)
            if cached_obj is None:
                cached_obj = self.BVV(*expr.args)
                self._object_cache[expr.hash()] = cached_obj
            return cached_obj
        if type(expr) is Bool and expr.op == "BoolV":
            return expr.args[0]
        return super().convert(expr)

    def _If(self, b, t, f):  # pylint:disable=no-self-use,unused-argument
        if not isinstance(b, bool):
            raise BackendError("BackendConcrete can't handle non-bool condition in If.")
        return t if b else f

    def _name(self, o):  # pylint:disable=unused-argument,no-self-use
        return None

    def _identical(self, a, b):
        if type(a) is bv.BVV and type(b) is bv.BVV and a.size() != b.size():
            return False
        return a == b

    def _convert(self, r):
        if type(r) in {int, str, bytes}:
            return r
        if isinstance(r, numbers.Number | bv.BVV | fp.FPV | fp.RM | fp.FSort | strings.StringV):
            return r
        raise BackendError(f"can't handle AST of type {type(r)}")

    def _abstract(self, e):  # pylint:disable=no-self-use
        if isinstance(e, bv.BVV):
            return claripy.BVV(e.value, e.size())
        if isinstance(e, bool):
            return claripy.BoolV(e)
        if isinstance(e, fp.FPV):
            return claripy.FPV(e.value, e.sort)
        if isinstance(e, strings.StringV):
            return claripy.StringV(e.value)
        raise BackendError(f"Couldn't abstract object of type {type(e)}")

    def _cardinality(self, a):  # pylint:disable=unused-argument
        # if we got here, it's a cardinality of 1
        return 1

    #
    # Evaluation functions
    #

    @staticmethod
    def _to_primitive(expr):
        if isinstance(expr, bv.BVV | fp.FPV | strings.StringV):
            return expr.value
        if isinstance(expr, bool | numbers.Number):
            return expr
        raise BackendError("idk how to turn this into a primitive")

    def _eval(self, expr, n, extra_constraints=(), solver=None, model_callback=None):
        if not all(extra_constraints):
            raise UnsatError("concrete False constraint in extra_constraints")

        return (self._to_primitive(expr),)

    def _batch_eval(self, exprs, n, extra_constraints=(), solver=None, model_callback=None):
        if not all(extra_constraints):
            raise UnsatError("concrete False constraint in extra_constraints")

        return [tuple(self._to_primitive(ex) for ex in exprs)]

    def _max(self, expr, extra_constraints=(), signed=False, solver=None, model_callback=None):
        if not all(extra_constraints):
            raise UnsatError("concrete False constraint in extra_constraints")
        return self._to_primitive(expr)

    def _min(self, expr, extra_constraints=(), signed=False, solver=None, model_callback=None):
        if not all(extra_constraints):
            raise UnsatError("concrete False constraint in extra_constraints")
        return self._to_primitive(expr)

    def _solution(self, expr, v, extra_constraints=(), solver=None, model_callback=None):
        if not all(extra_constraints):
            raise UnsatError("concrete False constraint in extra_constraints")
        return self.convert(expr) == v

    # Override Backend.is_true() for a better performance
    def is_true(self, e, extra_constraints=(), solver=None, model_callback=None):
        if isinstance(e, numbers.Number):
            return bool(e)
        if e is claripy.true():
            return True
        return super().is_true(e, extra_constraints=extra_constraints, solver=solver, model_callback=model_callback)

    # Override Backend.is_false() for a better performance
    def is_false(self, e, extra_constraints=(), solver=None, model_callback=None):
        if isinstance(e, numbers.Number):
            return not bool(e)
        if e is claripy.false():
            return True
        return super().is_false(e, extra_constraints=extra_constraints, solver=solver, model_callback=model_callback)

    # pylint:disable=singleton-comparison
    def _is_true(self, e, extra_constraints=(), solver=None, model_callback=None):
        return e == True  # noqa: E712

    def _is_false(self, e, extra_constraints=(), solver=None, model_callback=None):
        return e == False  # noqa: E712

    def _has_true(self, e, extra_constraints=(), solver=None, model_callback=None):
        return e == True  # noqa: E712

    def _has_false(self, e, extra_constraints=(), solver=None, model_callback=None):
        return e == False  # noqa: E712
