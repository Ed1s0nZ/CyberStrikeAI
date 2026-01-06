from __future__ import annotations


class ConcreteHandlerMixin:
    """ConcreteHandlerMixin is a mixin that handles concrete expressions
    evaluation to prevent invoking the backend solver.
    """

    def eval(self, e, n, extra_constraints=(), exact=None):
        c = self._concrete_value(e)
        if c is not None:
            return (c,)
        return super().eval(e, n, extra_constraints=extra_constraints, exact=exact)

    def batch_eval(self, exprs, n, extra_constraints=(), exact=None):
        concrete_exprs = [self._concrete_value(e) for e in exprs]
        symbolic_exprs = [e for e, c in zip(exprs, concrete_exprs, strict=False) if c is None]

        if len(symbolic_exprs) == 0:
            return [concrete_exprs]

        symbolic_results = [
            list(r) for r in super().batch_eval(symbolic_exprs, n, extra_constraints=extra_constraints, exact=exact)
        ]
        return [tuple((c if c is not None else r.pop(0)) for c in concrete_exprs) for r in symbolic_results]

    def max(self, e, extra_constraints=(), signed=False, exact=None):
        c = self._concrete_value(e)
        if c is not None:
            return c
        return super().max(e, extra_constraints=extra_constraints, signed=signed, exact=exact)

    def min(self, e, extra_constraints=(), signed=False, exact=None):
        c = self._concrete_value(e)
        if c is not None:
            return c
        return super().min(e, extra_constraints=extra_constraints, signed=signed, exact=exact)

    def solution(self, e, v, extra_constraints=(), exact=None):
        ce = self._concrete_value(e)
        cv = self._concrete_value(v)

        if ce is None or cv is None:
            return super().solution(e, v, extra_constraints=extra_constraints, exact=exact)
        return ce == cv

    def is_true(self, e, extra_constraints=(), exact=None):
        c = self._concrete_value(e)
        if c is not None:
            return c
        return super().is_true(e, extra_constraints=extra_constraints, exact=exact)

    def is_false(self, e, extra_constraints=(), exact=None):
        c = self._concrete_value(e)
        if c is not None:
            return not c
        return super().is_false(e, extra_constraints=extra_constraints, exact=exact)
