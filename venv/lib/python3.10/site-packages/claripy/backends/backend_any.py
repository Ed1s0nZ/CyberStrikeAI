from __future__ import annotations

import logging

import claripy
from claripy.backends.backend import Backend
from claripy.errors import BackendError

log = logging.getLogger(__name__)


class BackendAny(Backend):
    """
    BackendAny is a wrapper backend that tries all backends in order until
    one succeeds.
    """

    # pylint: disable=too-many-positional-arguments,unused-argument

    __slots__ = ()

    @staticmethod
    def _first_backend(obj, what):
        for b in claripy.backends.all_backends:
            if b in obj._errored:
                continue

            try:
                return getattr(b, what)(obj)
            except BackendError:
                pass
        raise BackendError(f"All backends failed to {what}")

    def convert(self, expr):
        return self._first_backend(expr, "convert")

    def simplify(self, expr):
        return self._first_backend(expr, "simplify")

    def is_true(self, e, extra_constraints=(), solver=None, model_callback=None):
        return self._first_backend(e, "is_true")

    def is_false(self, e, extra_constraints=(), solver=None, model_callback=None):
        return self._first_backend(e, "is_false")

    def has_true(self, e, extra_constraints=(), solver=None, model_callback=None):
        return self._first_backend(e, "has_true")

    def has_false(self, e, extra_constraints=(), solver=None, model_callback=None):
        return self._first_backend(e, "has_false")

    def eval(self, expr, n, extra_constraints=(), solver=None, model_callback=None):
        return self._first_backend(expr, "eval")

    def batch_eval(self, exprs, n, extra_constraints=(), solver=None, model_callback=None):
        return self._first_backend(exprs, "batch_eval")

    def min(self, expr, extra_constraints=(), signed=False, solver=None, model_callback=None):
        return self._first_backend(expr, "min")

    def max(self, expr, extra_constraints=(), signed=False, solver=None, model_callback=None):
        return self._first_backend(expr, "max")

    def satisfiable(self, extra_constraints=(), solver=None, model_callback=None):
        return self._first_backend(extra_constraints, "satisfiable")

    def name(self, a):
        return self._first_backend(a, "name")

    def identical(self, a, b):
        return self._first_backend(a, "identical")

    def cardinality(self, a):
        return self._first_backend(a, "cardinality")
