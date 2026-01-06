from __future__ import annotations

from claripy import false
from claripy.errors import ClaripyValueError, UnsatError


class ConstraintFilterMixin:
    """ConstraintFilterMixin is a mixin that filters True and False constraints."""

    def _constraint_filter(self, constraints):
        if type(constraints) not in (tuple, list):
            raise ClaripyValueError("The extra_constraints argument should be a list of constraints.")

        if len(constraints) == 0:
            return constraints

        ccs = [self._concrete_constraint(c) for c in constraints]
        if False in ccs:
            raise UnsatError("Constraints contain False.")
        return tuple(o for o, n in zip(constraints, ccs, strict=False) if n is not True)

    def _add(self, constraints, invalidate_cache=True):
        try:
            ec = self._constraint_filter(constraints)
        except UnsatError:
            # filter out concrete False
            ec = [c for c in constraints if c not in {False, false()}] + [false()]

        if len(constraints) == 0:
            return []

        if len(ec) > 0:
            return super()._add(ec, invalidate_cache=invalidate_cache)
        return []

    def satisfiable(self, extra_constraints=(), exact=None):
        try:
            ec = self._constraint_filter(extra_constraints)
            return super().satisfiable(extra_constraints=ec, exact=exact)
        except UnsatError:
            return False

    def eval(self, e, n, extra_constraints=(), exact=None):
        ec = self._constraint_filter(extra_constraints)
        return super().eval(e, n, extra_constraints=ec, exact=exact)

    def batch_eval(self, exprs, n, extra_constraints=(), exact=None):
        ec = self._constraint_filter(extra_constraints)
        return super().batch_eval(exprs, n, extra_constraints=ec, exact=exact)

    def max(self, e, extra_constraints=(), signed=False, exact=None):
        ec = self._constraint_filter(extra_constraints)
        return super().max(e, extra_constraints=ec, signed=signed, exact=exact)

    def min(self, e, extra_constraints=(), signed=False, exact=None):
        ec = self._constraint_filter(extra_constraints)
        return super().min(e, extra_constraints=ec, signed=signed, exact=exact)

    def solution(self, e, v, extra_constraints=(), exact=None):
        ec = self._constraint_filter(extra_constraints)
        return super().solution(e, v, extra_constraints=ec, exact=exact)

    def is_true(self, e, extra_constraints=(), exact=None):
        ec = self._constraint_filter(extra_constraints)
        return super().is_true(e, extra_constraints=ec, exact=exact)

    def is_false(self, e, extra_constraints=(), exact=None):
        ec = self._constraint_filter(extra_constraints)
        return super().is_false(e, extra_constraints=ec, exact=exact)
