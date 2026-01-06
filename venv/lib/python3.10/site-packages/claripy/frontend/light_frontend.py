from __future__ import annotations

import logging

from claripy import backends
from claripy.errors import BackendError, ClaripyFrontendError

from .constrained_frontend import ConstrainedFrontend

log = logging.getLogger(__name__)


class LightFrontend(ConstrainedFrontend):
    """LightFrontend is an extremely simple frontend that is used primarily for
    the purpose of quickly evaluating expressions using the concrete and VSA
    backends.
    """

    def __init__(self, solver_backend, **kwargs):
        # since the LightFrontend mostly cannot handle extra_constriants, it horribly misuses the cache.
        # Because of this, we have to disable the caching here.
        super().__init__(**kwargs)
        self._solver_backend = solver_backend

    def _blank_copy(self, c):
        super()._blank_copy(c)
        c._solver_backend = self._solver_backend

    def _copy(self, c):
        super()._copy(c)
        c._solver_backend = self._solver_backend

    #
    # Serialization support
    #

    def __getstate__(self):
        return self._solver_backend.__class__.__name__, super().__getstate__()

    def __setstate__(self, s):
        solver_backend_name, base_state = s
        super().__setstate__(base_state)
        self._solver_backend = backends.backends_by_type[solver_backend_name]

    #
    # Light functionality
    #

    def eval(self, e, n, extra_constraints=(), exact=None):
        try:
            return tuple(self._solver_backend.eval(e, n))
        except BackendError as err:
            raise ClaripyFrontendError("Light solver can't handle this eval().") from err

    def batch_eval(self, exprs, n, extra_constraints=(), exact=None):
        try:
            return self._solver_backend._batch_eval(exprs, n)
        except BackendError as err:
            raise ClaripyFrontendError("Light solver can't handle this batch_eval().") from err

    def max(self, e, extra_constraints=(), signed=False, exact=None):
        try:
            return self._solver_backend.max(e, signed=signed)
        except BackendError as err:
            raise ClaripyFrontendError("Light solver can't handle this max().") from err

    def min(self, e, extra_constraints=(), signed=False, exact=None):
        try:
            return self._solver_backend.min(e, signed=signed)
        except BackendError as err:
            raise ClaripyFrontendError("Light solver can't handle this min().") from err

    def solution(self, e, v, extra_constraints=(), exact=None):
        try:
            return self._solver_backend.solution(e, v)
        except BackendError as err:
            raise ClaripyFrontendError("Light solver can't handle this solution().") from err

    def is_true(self, e, extra_constraints=(), exact=None):
        try:
            return self._solver_backend.is_true(e)
        except BackendError:
            log.info("Light solver can't handle this is_true().")
            return False

    def is_false(self, e, extra_constraints=(), exact=None):
        try:
            return self._solver_backend.is_false(e)
        except BackendError:
            log.info("Light solver can't handle this is_false().")
            return False

    def satisfiable(self, extra_constraints=(), exact=None):
        return not any(
            self.is_false(c, extra_constraints=extra_constraints, exact=exact)
            for c in reversed(self.constraints + list(extra_constraints))
        )

    #
    # Merging and splitting
    #

    def merge(self, others, merge_conditions, common_ancestor=None):
        return (
            self._solver_backend.__class__.__name__ == "BackendZ3",
            ConstrainedFrontend.merge(self, others, merge_conditions, common_ancestor=common_ancestor)[1],
        )
