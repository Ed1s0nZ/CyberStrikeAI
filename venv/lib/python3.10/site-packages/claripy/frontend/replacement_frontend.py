from __future__ import annotations

import logging
import numbers
from contextlib import suppress

import claripy
from claripy import backends
from claripy.ast import Base
from claripy.errors import BackendError, ClaripyFrontendError

from .constrained_frontend import ConstrainedFrontend

log = logging.getLogger(__name__)


class ReplacementFrontend(ConstrainedFrontend):
    """ReplacementFrontend is a frontend that allows for the replacement
    symbolic constraints with concrete solutions. This is useful for
    simplifying constraints and speeding up solving time.
    """

    # pylint: disable=too-many-positional-arguments

    def __init__(
        self,
        actual_frontend,
        allow_symbolic=None,
        replacements=None,
        replacement_cache=None,
        unsafe_replacement=None,
        complex_auto_replace=None,
        auto_replace=None,
        replace_constraints=None,
        **kwargs,
    ):
        super().__init__(**kwargs)
        self._actual_frontend = actual_frontend
        self._allow_symbolic = True if allow_symbolic is None else allow_symbolic
        self._auto_replace = True if auto_replace is None else auto_replace
        self._complex_auto_replace = False if complex_auto_replace is None else complex_auto_replace
        self._replace_constraints = False if replace_constraints is None else replace_constraints
        self._unsafe_replacement = False if unsafe_replacement is None else unsafe_replacement
        self._replacements = replacements or {}
        self._replacement_cache = replacement_cache or {}

    def _blank_copy(self, c):
        super()._blank_copy(c)
        c._actual_frontend = self._actual_frontend.blank_copy()
        c._allow_symbolic = self._allow_symbolic
        c._auto_replace = self._auto_replace
        c._complex_auto_replace = self._complex_auto_replace
        c._replace_constraints = self._replace_constraints
        c._unsafe_replacement = self._unsafe_replacement
        c._replacements = {}
        c._replacement_cache = {}

    def _copy(self, c):
        super()._copy(c)
        self._actual_frontend._copy(c._actual_frontend)

        c._replacements = dict(self._replacements)
        c._replacement_cache = dict(self._replacement_cache)

    #
    # Replacements
    #

    def add_replacement(self, old, new, invalidate_cache=True, replace=True, promote=True):
        if not isinstance(old, Base):
            return

        if old is new:
            return

        if not replace and old.hash() in self._replacements:
            return

        if not promote and old.hash() in self._replacement_cache:
            return

        if not isinstance(new, Base):
            if isinstance(new, bool):
                new = claripy.BoolV(new)
            elif isinstance(new, numbers.Number):
                new = claripy.BVV(new, old.length)
            else:
                return

        if invalidate_cache:
            self._replacements = dict(self._replacements)
            self._replacement_cache = dict(self._replacements)

        self._replacements[old.hash()] = new
        self._replacement_cache[old.hash()] = new

    def remove_replacements(self, old_entries):
        self._replacements = {k: v for k, v in self._replacements.items() if k not in old_entries}
        self._replacement_cache = dict(self._replacements)

    def clear_replacements(self):
        self._replacements = {}
        self._replacement_cache = dict(self._replacements)

    def _replacement(self, old):
        if not isinstance(old, Base):
            return old

        if old.hash() in self._replacement_cache:
            return self._replacement_cache[old.hash()]

        # not found in the cache
        new = claripy.replace_dict(old, self._replacement_cache)
        if new is not old:
            self._replacement_cache[old.hash()] = new
        return new

    def _add_solve_result(self, e, er, r):
        if not self._auto_replace:
            return
        if not isinstance(e, Base) or not e.symbolic:
            return
        if er.symbolic:
            return
        self.add_replacement(e, r, invalidate_cache=False)

    #
    # Storable support
    #

    def downsize(self):
        self._actual_frontend.downsize()
        self._replacement_cache.clear()

    def __getstate__(self):
        return (
            self._allow_symbolic,
            self._unsafe_replacement,
            self._complex_auto_replace,
            self._auto_replace,
            self._replace_constraints,
            self._replacements,
            self._actual_frontend,
            super().__getstate__(),
        )

    def __setstate__(self, s):
        (
            self._allow_symbolic,
            self._unsafe_replacement,
            self._complex_auto_replace,
            self._auto_replace,
            self._replace_constraints,
            self._replacements,
            self._actual_frontend,
            base_state,
        ) = s

        super().__setstate__(base_state)
        self._replacement_cache = dict(self._replacements)

    #
    # Replacement solving
    #

    def _replace_list(self, lst):
        return tuple(self._replacement(c) for c in lst)

    def eval(self, e, n, extra_constraints=(), exact=None):
        er = self._replacement(e)
        ecr = self._replace_list(extra_constraints)
        r = self._actual_frontend.eval(er, n, extra_constraints=ecr, exact=exact)
        if self._unsafe_replacement:
            self._add_solve_result(e, er, r[0])
        return r

    def batch_eval(self, exprs, n, extra_constraints=(), exact=None):
        er = self._replace_list(exprs)
        ecr = self._replace_list(extra_constraints)
        r = self._actual_frontend.batch_eval(er, n, extra_constraints=ecr, exact=exact)
        if self._unsafe_replacement:
            for i, original in enumerate(exprs):
                self._add_solve_result(original, er[i], r[0][i])
        return r

    def max(self, e, extra_constraints=(), signed=False, exact=None):
        er = self._replacement(e)
        ecr = self._replace_list(extra_constraints)
        r = self._actual_frontend.max(er, extra_constraints=ecr, signed=signed, exact=exact)
        if self._unsafe_replacement:
            self._add_solve_result(e, er, r)
        return r

    def min(self, e, extra_constraints=(), signed=False, exact=None):
        er = self._replacement(e)
        ecr = self._replace_list(extra_constraints)
        r = self._actual_frontend.min(er, extra_constraints=ecr, signed=signed, exact=exact)
        if self._unsafe_replacement:
            self._add_solve_result(e, er, r)
        return r

    def solution(self, e, v, extra_constraints=(), exact=None):
        er = self._replacement(e)
        vr = self._replacement(v)
        ecr = self._replace_list(extra_constraints)
        r = self._actual_frontend.solution(er, vr, extra_constraints=ecr, exact=exact)
        if self._unsafe_replacement and r and (not isinstance(vr, Base) or not vr.symbolic):
            self._add_solve_result(e, er, vr)
        return r

    def is_true(self, e, extra_constraints=(), exact=None):
        er = self._replacement(e)
        ecr = self._replace_list(extra_constraints)
        return self._actual_frontend.is_true(er, extra_constraints=ecr, exact=exact)

    def is_false(self, e, extra_constraints=(), exact=None):
        er = self._replacement(e)
        ecr = self._replace_list(extra_constraints)
        return self._actual_frontend.is_false(er, extra_constraints=ecr, exact=exact)

    def satisfiable(self, extra_constraints=(), exact=None):
        ecr = self._replace_list(extra_constraints)
        return self._actual_frontend.satisfiable(extra_constraints=ecr, exact=exact)

    def _concrete_value(self, e):
        c = super()._concrete_value(e)
        if c is not None:
            return c

        cr = self._replacement(e)
        with suppress(BackendError):
            return backends.concrete.eval(cr, 1)[0]
        return None

    def _concrete_constraint(self, e):
        c = super()._concrete_value(e)
        if c is not None:
            return c

        # if er.is_false():
        #   raise UnsatError("Replacement frontend received False constraint after replacement.")
        if self._replace_constraints:
            er = self._replacement(e)
            return super()._concrete_constraint(er)
        return super()._concrete_constraint(e)

    def _add(self, constraints, invalidate_cache=True):
        if self._auto_replace:
            for c in constraints:
                # the badass thing here would be to use the *replaced* constraint, but
                # we don't currently support chains of replacements, so we'll do a
                # less effective flat-replacement with the original constraint
                # rc = self._replacement(c)
                rc = c
                if not isinstance(rc, Base) or not rc.symbolic:
                    continue

                if not self._complex_auto_replace:
                    if rc.op == "Not":
                        self.add_replacement(
                            c.args[0], claripy.false(), replace=False, promote=True, invalidate_cache=True
                        )
                    elif rc.op == "__eq__" and rc.args[0].symbolic ^ rc.args[1].symbolic:
                        old, new = rc.args if rc.args[0].symbolic else rc.args[::-1]
                        self.add_replacement(old, new, replace=False, promote=True, invalidate_cache=True)
                else:
                    satisfiable, replacements = backends.vsa.constraint_to_si(rc)
                    if not satisfiable:
                        self.add_replacement(rc, claripy.false())
                    for old, new in replacements:
                        if old.cardinality == 1:
                            continue

                        rold = self._replacement(old)
                        if rold.cardinality == 1:
                            continue

                        self.add_replacement(old, rold.intersection(new))

        added = super()._add(constraints)
        cr = self._replace_list(added)
        if not self._allow_symbolic and any(c.symbolic for c in cr):
            raise ClaripyFrontendError(
                "symbolic constraints made it into ReplacementFrontend with allow_symbolic=False"
            )
        self._actual_frontend.add(cr)

        return added
