from __future__ import annotations


class SimplifyHelperMixin:
    """SimplifyHelperMixin is a mixin that simplifies constraints before evaluation."""

    def max(self, e, extra_constraints=(), signed=False, exact=None):
        self.simplify()
        return super().max(e, extra_constraints=extra_constraints, signed=signed, exact=exact)

    def min(self, e, extra_constraints=(), signed=False, exact=None):
        self.simplify()
        return super().min(e, extra_constraints=extra_constraints, signed=signed, exact=exact)

    def eval(self, e, n, extra_constraints=(), exact=None):
        if n > 1:
            self.simplify()
        return super().eval(e, n, extra_constraints=extra_constraints, exact=exact)

    def batch_eval(self, exprs, n, extra_constraints=(), exact=None):
        if n > 1:
            self.simplify()
        return super().batch_eval(exprs, n, extra_constraints=extra_constraints, exact=exact)
