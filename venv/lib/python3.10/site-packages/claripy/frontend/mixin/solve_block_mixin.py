from __future__ import annotations


class SolveBlockMixin:
    """SolveBlockMixin that adds the ability to block solving by setting can_solve to False."""

    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.can_solve = True

    def _blank_copy(self, c):
        super()._blank_copy(c)
        c.can_solve = True

    def _copy(self, c):
        super()._copy(c)
        c.can_solve = self.can_solve

    def __getstate__(self):
        return self.can_solve, super().__getstate__()

    def __setstate__(self, s):
        self.can_solve, base_state = s
        super().__setstate__(base_state)

    def eval(self, e, n, extra_constraints=(), exact=None):
        assert self.can_solve
        return super().eval(e, n, extra_constraints=extra_constraints, exact=exact)

    def batch_eval(self, exprs, n, extra_constraints=(), exact=None):
        assert self.can_solve
        return super().batch_eval(exprs, n, extra_constraints=extra_constraints, exact=exact)

    def min(self, e, extra_constraints=(), signed=False, exact=None):
        assert self.can_solve
        return super().min(e, extra_constraints=extra_constraints, signed=signed, exact=exact)

    def max(self, e, extra_constraints=(), signed=False, exact=None):
        assert self.can_solve
        return super().max(e, extra_constraints=extra_constraints, signed=signed, exact=exact)

    def satisfiable(self, extra_constraints=(), exact=None):
        assert self.can_solve
        return super().satisfiable(extra_constraints=extra_constraints, exact=exact)

    def solution(self, e, v, extra_constraints=(), exact=None):
        assert self.can_solve
        return super().solution(e, v, extra_constraints=extra_constraints, exact=exact)
