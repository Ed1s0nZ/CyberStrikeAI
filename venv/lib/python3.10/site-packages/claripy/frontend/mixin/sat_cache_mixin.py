from __future__ import annotations

from typing import TYPE_CHECKING

from claripy import And, false, is_false
from claripy.errors import UnsatError

if TYPE_CHECKING:
    from collections.abc import Iterable

    from claripy.ast.bool import Bool


class SatCacheMixin:
    """SatCacheMixin is a mixin that caches the satisfiability of the constraints."""

    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self._cached_satness = None
        self._cached_unsat_core: tuple[Bool, ...] | None = None

    def _blank_copy(self, c):
        super()._blank_copy(c)
        c._cached_satness = None
        c._cached_unsat_core = None

    def _copy(self, c):
        super()._copy(c)
        c._cached_satness = self._cached_satness
        c._cached_unsat_core = self._cached_unsat_core

    def __getstate__(self):
        return self._cached_satness, self._cached_unsat_core, super().__getstate__()

    def __setstate__(self, s):
        self._cached_satness, self._cached_unsat_core, base_state = s
        super().__setstate__(base_state)

    #
    # SAT caching
    #

    def _add(self, constraints, invalidate_cache=True):
        added = super()._add(constraints, invalidate_cache=invalidate_cache)

        cached_satness = None
        if len(added) > 0:
            if any(c is false() for c in added):
                cached_satness = False
            elif len(added) == 1 and len(self.constraints) < 5:
                added_ = added[0]
                for con in self.constraints:
                    if is_false(And(con.clear_annotations(), added_.clear_annotations())):
                        cached_satness = False
                        self._cached_unsat_core = (con, added)
                        break

        if cached_satness is False:
            self._cached_satness = False
        elif self._cached_satness is True:
            self._cached_satness = None

        return added

    def simplify(self):
        new_constraints = super().simplify()
        if len(new_constraints) > 0 and any(c is false() for c in new_constraints):
            self._cached_satness = False
        return new_constraints

    def satisfiable(self, extra_constraints=(), exact=None):
        if self._cached_satness is False:
            return False
        if self._cached_satness is True and len(extra_constraints) == 0:
            return True
        r = super().satisfiable(extra_constraints=extra_constraints, exact=exact)
        if len(extra_constraints) == 0:
            self._cached_satness = r
        return r

    def check_satisfiability(self, extra_constraints=(), exact=None) -> str:
        if self._cached_satness is False:
            return "UNSAT"
        if self._cached_satness is True and not extra_constraints:
            return "SAT"
        return super().check_satisfiability(extra_constraints=extra_constraints, exact=exact)

    def eval(self, e, n, extra_constraints=(), exact=None):
        if self._cached_satness is False:
            raise UnsatError("cached unsat")
        try:
            r = super().eval(e, n, extra_constraints=extra_constraints, exact=exact)
            self._cached_satness = True
            return r
        except UnsatError:
            if len(extra_constraints) == 0:
                self._cached_satness = False
            raise

    def batch_eval(self, e, n, extra_constraints=(), exact=None):
        if self._cached_satness is False:
            raise UnsatError("cached unsat")
        try:
            r = super().batch_eval(e, n, extra_constraints=extra_constraints, exact=exact)
            self._cached_satness = True
            return r
        except UnsatError:
            if len(extra_constraints) == 0:
                self._cached_satness = False
            raise

    def max(self, e, extra_constraints=(), signed=False, exact=None):
        if self._cached_satness is False:
            raise UnsatError("cached unsat")
        try:
            r = super().max(e, extra_constraints=extra_constraints, signed=signed, exact=exact)
            self._cached_satness = True
            return r
        except UnsatError:
            if len(extra_constraints) == 0:
                self._cached_satness = False
            raise

    def min(self, e, extra_constraints=(), signed=False, exact=None):
        if self._cached_satness is False:
            raise UnsatError("cached unsat")
        try:
            r = super().min(e, extra_constraints=extra_constraints, signed=signed, exact=exact)
            self._cached_satness = True
            return r
        except UnsatError:
            if len(extra_constraints) == 0:
                self._cached_satness = False
            raise

    def solution(self, e, v, extra_constraints=(), exact=None):
        if self._cached_satness is False:
            raise UnsatError("cached unsat")
        try:
            r = super().solution(e, v, extra_constraints=extra_constraints, exact=exact)
            self._cached_satness = True
            return r
        except UnsatError:
            if len(extra_constraints) == 0:
                self._cached_satness = False
            raise

    def unsat_core(self, extra_constraints: tuple[Bool, ...] = ()) -> Iterable[Bool]:
        if self._cached_unsat_core is not None:
            return self._cached_unsat_core
        return super().unsat_core(extra_constraints)
