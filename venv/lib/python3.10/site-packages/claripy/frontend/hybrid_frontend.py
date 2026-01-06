from __future__ import annotations

import logging

from claripy.errors import ClaripyFrontendError
from claripy.frontend import Frontend

log = logging.getLogger(__name__)


class HybridFrontend(Frontend):
    """HybridFrontend is a frontend that uses two backends, one exact and one
    approximate, to solve constraints.

    In practice this allows there to be a solver that can use the VSA backend or
    the Z3 backend depending on the constraints.
    """

    def __init__(self, exact_frontend, approximate_frontend, approximate_first=False, **kwargs):
        Frontend.__init__(self, **kwargs)
        self._exact_frontend = exact_frontend
        self._approximate_frontend = approximate_frontend
        self._approximate_first = approximate_first

    def _blank_copy(self, c):
        c._exact_frontend = self._exact_frontend.blank_copy()
        c._approximate_frontend = self._approximate_frontend.blank_copy()
        c._approximate_first = self._approximate_first

    def _copy(self, c):
        self._exact_frontend._copy(c._exact_frontend)
        self._approximate_frontend._copy(c._approximate_frontend)
        self._approximate_first = c._approximate_first

    #
    # Some passthroughs
    #

    @property
    def constraints(self):
        return self._exact_frontend.constraints

    @property
    def variables(self):
        return self._exact_frontend.variables

    #
    # Serialization support
    #

    def __getstate__(self):
        return (self._exact_frontend, self._approximate_frontend, super().__getstate__())

    def __setstate__(self, s):
        self._exact_frontend, self._approximate_frontend, base_state = s
        super().__setstate__(base_state)

    #
    # Hybrid solving
    #

    def _do_call(self, f_name, *args, exact=True, **kwargs):
        # if approximating, try the approximation backend
        if exact is False:
            try:
                return False, getattr(self._approximate_frontend, f_name)(*args, **kwargs)
            except ClaripyFrontendError:
                pass

        # if that fails, try the exact backend
        return True, getattr(self._exact_frontend, f_name)(*args, **kwargs)

    def _hybrid_call(self, f_name, *args, **kwargs):
        return self._do_call(f_name, *args, **kwargs)[1]

    def _approximate_first_call(self, f_name, e, n, *args, **kwargs):
        exact_used, solutions = self._do_call(f_name, e, n + 1, *args, exact=False, **kwargs)

        if (exact_used is False and len(solutions) > n) and any(
            getattr(c, "variables", set()) & e.variables for c in self.constraints
        ):
            _, _solutions = self._do_call(f_name, e, n + 1, *args, exact=True, **kwargs)
            return _solutions[:n] if len(_solutions) < len(solutions) else solutions[:n]

        return solutions[:n]

    def satisfiable(self, extra_constraints=(), exact=None):
        return self._hybrid_call("satisfiable", extra_constraints=extra_constraints, exact=exact)

    def eval_to_ast(self, e, n, extra_constraints=(), exact=None):
        if self._approximate_first and exact is None and n > 2:
            return self._approximate_first_call("eval_to_ast", e, n, extra_constraints=extra_constraints)
        return self._hybrid_call("eval_to_ast", e, n, extra_constraints=extra_constraints, exact=exact)

    def eval(self, e, n, extra_constraints=(), exact=None):
        if self._approximate_first and exact is None and n > 2:
            return self._approximate_first_call("eval", e, n, extra_constraints=extra_constraints)
        return self._hybrid_call("eval", e, n, extra_constraints=extra_constraints, exact=exact)

    def batch_eval(self, exprs, n, extra_constraints=(), exact=None):
        if self._approximate_first and exact is None and n > 2:
            return self._approximate_first_call("batch_eval", exprs, n, extra_constraints=extra_constraints)
        return self._hybrid_call("batch_eval", exprs, n, extra_constraints=extra_constraints, exact=exact)

    def max(self, e, extra_constraints=(), signed=False, exact=None):
        return self._hybrid_call("max", e, extra_constraints=extra_constraints, signed=signed, exact=exact)

    def min(self, e, extra_constraints=(), signed=False, exact=None):
        return self._hybrid_call("min", e, extra_constraints=extra_constraints, signed=signed, exact=exact)

    def solution(self, e, v, extra_constraints=(), exact=None):
        return self._hybrid_call("solution", e, v, extra_constraints=extra_constraints, exact=exact)

    def is_true(self, e, extra_constraints=(), exact=None):
        return self._hybrid_call("is_true", e, extra_constraints=extra_constraints, exact=exact)

    def is_false(self, e, extra_constraints=(), exact=None):
        return self._hybrid_call("is_false", e, extra_constraints=extra_constraints, exact=exact)

    def unsat_core(self, extra_constraints=()):
        return self._hybrid_call("unsat_core", extra_constraints=extra_constraints)

    #
    # Lifecycle
    #

    def _add(self, constraints, invalidate_cache=True):
        added = self._exact_frontend.add(constraints)
        self._approximate_frontend.add(constraints)
        return added

    def combine(self, others):
        other_exact = [o._exact_frontend for o in others]
        other_approximate = [o._approximate_frontend for o in others]
        new_exact = self._exact_frontend.combine(other_exact)
        new_approximate = self._approximate_frontend.combine(other_approximate)
        return HybridFrontend(new_exact, new_approximate)

    def merge(self, others, merge_conditions, common_ancestor=None):
        other_exact = [o._exact_frontend for o in others]
        other_approximate = [o._approximate_frontend for o in others]
        e_merged, new_exact = self._exact_frontend.merge(
            other_exact,
            merge_conditions,
            common_ancestor=common_ancestor._exact_frontend if common_ancestor is not None else None,
        )
        new_approximate = self._approximate_frontend.merge(
            other_approximate,
            merge_conditions,
            common_ancestor=common_ancestor._approximate_frontend if common_ancestor is not None else None,
        )[-1]
        return (e_merged, HybridFrontend(new_exact, new_approximate))

    def simplify(self):
        self._approximate_frontend.simplify()
        return self._exact_frontend.simplify()

    def downsize(self):
        self._exact_frontend.downsize()
        self._approximate_frontend.downsize()

    def finalize(self):
        self._exact_frontend.finalize()
        self._approximate_frontend.finalize()

    def split(self):
        results = []
        exacts = self._exact_frontend.split()

        for e in exacts:
            a = self._approximate_frontend.blank_copy()
            a.add(e.constraints)
            results.append(HybridFrontend(e, a))
        return results
