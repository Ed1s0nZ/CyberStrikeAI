from __future__ import annotations

import logging
import threading
from typing import TYPE_CHECKING, Any, overload

import claripy
from claripy import backends
from claripy.errors import BackendError, ClaripyFrontendError, UnsatError

from .constrained_frontend import ConstrainedFrontend

if TYPE_CHECKING:
    from collections.abc import Iterable

    from typing_extensions import Self

    from claripy.ast import BV, FP, Bool

log = logging.getLogger(__name__)


class FullFrontend(ConstrainedFrontend):
    """FullFrontend is a frontend that supports all claripy operations and is
    backed by a full solver backend.
    """

    def __init__(self, solver_backend, timeout=None, max_memory=None, track=False, **kwargs):
        ConstrainedFrontend.__init__(self, **kwargs)
        self._track = track
        self._solver_backend = solver_backend
        self.timeout = timeout if timeout is not None else 300000
        self.max_memory = max_memory
        self._tls = threading.local()
        self._to_add = []

    def _blank_copy(self, c):
        super()._blank_copy(c)
        c._track = self._track
        c._solver_backend = self._solver_backend
        c.timeout = self.timeout
        c.max_memory = self.max_memory
        c._tls = threading.local()
        c._to_add = []

    def _copy(self, c):
        super()._copy(c)
        c._track = self._track
        c._tls.solver = getattr(self._tls, "solver", None)  # pylint:disable=no-member
        c._to_add = list(self._to_add)

    #
    # Serialization support
    #

    def __getstate__(self):
        return (
            self._solver_backend.__class__.__name__,
            self.timeout,
            self.max_memory,
            self._track,
            super().__getstate__(),
        )

    def __setstate__(self, s):
        backend_name, self.timeout, self.max_memory, self._track, base_state = s
        self._solver_backend = backends.backends_by_type[backend_name]
        # self._tls = None
        self._tls = threading.local()
        self._to_add = []
        super().__setstate__(base_state)

    #
    # Frontend Creation
    #

    def _get_solver(self):
        if getattr(self._tls, "solver", None) is None:
            self._tls.solver = self._solver_backend.solver(timeout=self.timeout, max_memory=self.max_memory)
            self._add_constraints()
        elif self._finalized and len(self._to_add) > 0:
            if not hasattr(self._solver_backend, "clone_solver") or self._solver_backend.reuse_z3_solver:
                # this function may return a cached solver
                self._tls.solver = self._solver_backend.solver(timeout=self.timeout, max_memory=self.max_memory)
            else:
                self._tls.solver = self._solver_backend.clone_solver(self._tls.solver)
            self._add_constraints()

        if len(self._to_add) > 0:
            self._add_constraints()

        solver = self._tls.solver
        if self._solver_backend.reuse_z3_solver:
            # we must re-add all constraints
            self._add_constraints()
        return solver

    def _add_constraints(self):
        self._solver_backend.add(self._tls.solver, self.constraints, track=self._track)
        self._to_add = []

    #
    # Constraint management
    #

    def _add(self, constraints, invalidate_cache=True):
        to_add = ConstrainedFrontend._add(self, constraints)
        self._to_add += to_add
        return to_add

    def simplify(self):
        ConstrainedFrontend.simplify(self)

        # TODO: should we do this?
        self._tls.solver = None
        self._to_add = []

        return self.constraints

    def check_satisfiability(self, extra_constraints=(), exact=None) -> str:
        if not extra_constraints and len(self.constraints) == 1:
            con = self.constraints[0]
            if con.op in {"__eq__", "__ne__"}:
                op0, op1 = con.args
                if op0.op == "BVS" and op1.op == "BVV":
                    # trivially satisfiable
                    if self._model_hook is not None:
                        # create a new model
                        self._model_hook(
                            {
                                op0.args[0]: (
                                    op1.concrete_value
                                    if con.op == "__eq__"
                                    else (op1.concrete_value + 1) & ((1 << op0.size()) - 1)
                                )
                            }
                        )
                    return "SAT"
        try:
            return self._solver_backend.check_satisfiability(
                extra_constraints=extra_constraints, solver=self._get_solver(), model_callback=self._model_hook
            )
        except BackendError as e:
            raise ClaripyFrontendError("Backend error during solve") from e

    def satisfiable(self, extra_constraints: Iterable[Bool] = (), exact: bool | None = None) -> bool:
        try:
            return self._solver_backend.satisfiable(
                extra_constraints=extra_constraints, solver=self._get_solver(), model_callback=self._model_hook
            )
        except BackendError as e:
            raise ClaripyFrontendError("Backend error during solve") from e

    @overload
    def eval(
        self, e: BV, n: int, extra_constraints: tuple[Bool, ...] = ..., exact: Bool | None = ...
    ) -> tuple[int, ...]: ...

    @overload
    def eval(
        self, e: Bool, n: int, extra_constraints: tuple[Bool, ...] = ..., exact: Bool | None = ...
    ) -> tuple[bool, ...]: ...

    @overload
    def eval(
        self, e: FP, n: int, extra_constraints: tuple[Bool, ...] = ..., exact: Bool | None = ...
    ) -> tuple[float, ...]: ...

    def eval(self, e, n, extra_constraints=(), exact=None) -> tuple[Any, ...]:
        try:
            results = tuple(
                self._solver_backend.eval(
                    e,
                    n,
                    extra_constraints=extra_constraints,
                    solver=self._get_solver(),
                    model_callback=self._model_hook,
                )
            )
            if len(results) == 0:
                raise UnsatError("unsat")
            return results
        except BackendError as exc:
            raise ClaripyFrontendError("Backend error during eval") from exc

    # this is technically wrong but we cannot do better because python doesn't have associated types
    # if you need another overload, add one
    @overload
    def batch_eval(
        self, exprs: Iterable[BV], n: int, extra_constraints: tuple[Bool, ...] = ..., exact: bool | None = ...
    ) -> list[tuple[int, ...]]: ...

    @overload
    def batch_eval(
        self, exprs: Iterable[Bool], n: int, extra_constraints: tuple[Bool, ...] = ..., exact: bool | None = ...
    ) -> list[tuple[bool, ...]]: ...

    @overload
    def batch_eval(
        self, exprs: Iterable[FP], n: int, extra_constraints: tuple[Bool, ...] = ..., exact: bool | None = ...
    ) -> list[tuple[float, ...]]: ...

    def batch_eval(self, exprs, n, extra_constraints=(), exact=None):
        try:
            results = self._solver_backend.batch_eval(
                exprs,
                n,
                extra_constraints=extra_constraints,
                solver=self._get_solver(),
                model_callback=self._model_hook,
            )
            if len(results) == 0:
                raise UnsatError("unsat")
            return results
        except BackendError as e:
            raise ClaripyFrontendError("Backend error during batch_eval") from e

    @overload
    def max(
        self, e: BV, extra_constraints: tuple[Bool, ...] = ..., signed: bool = ..., exact: Bool | None = ...
    ) -> int: ...

    @overload
    def max(
        self, e: Bool, extra_constraints: tuple[Bool, ...] = ..., signed: bool = ..., exact: Bool | None = ...
    ) -> bool: ...

    @overload
    def max(
        self, e: FP, extra_constraints: tuple[Bool, ...] = ..., signed: bool = ..., exact: Bool | None = ...
    ) -> float: ...

    def max(self, e, extra_constraints=(), signed=False, exact=None):
        if not self.satisfiable(extra_constraints=extra_constraints):
            raise UnsatError("Unsat during _max()")

        log.debug("Frontend.max() with %d extra_constraints", len(extra_constraints))

        # pylint: disable=unsubscriptable-object
        two = self.eval(e, 2, extra_constraints=extra_constraints)
        if len(two) == 0:
            raise UnsatError("unsat during max()")
        if len(two) == 1:
            return two[0]

        if signed:
            c = (*tuple(extra_constraints), claripy.SGE(e, two[0]), claripy.SGE(e, two[1]))
        else:
            c = (*tuple(extra_constraints), claripy.UGE(e, two[0]), claripy.UGE(e, two[1]))
        try:
            return self._solver_backend.max(
                e,
                extra_constraints=c,
                solver=self._get_solver(),
                model_callback=self._model_hook,
                signed=signed,
            )
        except BackendError as exc:
            raise ClaripyFrontendError("Backend error during max") from exc

    @overload
    def min(
        self, e: BV, extra_constraints: tuple[Bool, ...] = ..., signed: bool = ..., exact: Bool | None = ...
    ) -> int: ...

    @overload
    def min(
        self, e: Bool, extra_constraints: tuple[Bool, ...] = ..., signed: bool = ..., exact: Bool | None = ...
    ) -> bool: ...

    @overload
    def min(
        self, e: FP, extra_constraints: tuple[Bool, ...] = ..., signed: bool = ..., exact: Bool | None = ...
    ) -> float: ...

    def min(self, e, extra_constraints=(), signed=False, exact=None):
        if not self.satisfiable(extra_constraints=extra_constraints):
            raise UnsatError("Unsat during _min()")

        log.debug("Frontend.min() with %d extra_constraints", len(extra_constraints))

        # pylint: disable=unsubscriptable-object
        two = self.eval(e, 2, extra_constraints=extra_constraints)
        if len(two) == 0:
            raise UnsatError("unsat during min()")
        if len(two) == 1:
            return two[0]

        if signed:
            c = (*tuple(extra_constraints), claripy.SLE(e, two[0]), claripy.SLE(e, two[1]))
        else:
            c = (*tuple(extra_constraints), claripy.ULE(e, two[0]), claripy.ULE(e, two[1]))
        try:
            return self._solver_backend.min(
                e,
                extra_constraints=c,
                solver=self._get_solver(),
                model_callback=self._model_hook,
                signed=signed,
            )
        except BackendError as exc:
            raise ClaripyFrontendError("Backend error during min") from exc

    @overload
    def solution(self, e: BV, v: int, extra_constraints: tuple[Bool, ...] = ..., exact: Bool | None = ...) -> bool: ...

    @overload
    def solution(
        self, e: Bool, v: bool, extra_constraints: tuple[Bool, ...] = ..., exact: Bool | None = ...
    ) -> bool: ...

    @overload
    def solution(
        self, e: FP, v: float, extra_constraints: tuple[Bool, ...] = ..., exact: Bool | None = ...
    ) -> bool: ...

    def solution(self, e, v, extra_constraints=(), exact=None):
        try:
            return self._solver_backend.solution(
                e, v, extra_constraints=extra_constraints, solver=self._get_solver(), model_callback=self._model_hook
            )
        except BackendError as exc:
            raise ClaripyFrontendError("Backend error during solution") from exc

    def is_true(self, e: Bool, extra_constraints: tuple[Bool, ...] = (), exact: bool | None = None) -> bool:
        return self._solver_backend.is_true(e, extra_constraints=extra_constraints, solver=self._get_solver())

    def is_false(self, e: Bool, extra_constraints: tuple[Bool, ...] = (), exact: bool | None = None) -> bool:
        return self._solver_backend.is_false(e, extra_constraints=extra_constraints, solver=self._get_solver())

    def unsat_core(self, extra_constraints: tuple[Bool, ...] = ()) -> Iterable[Bool]:
        if self.satisfiable(extra_constraints=extra_constraints):
            # all constraints are satisfied
            return ()

        unsat_core = self._solver_backend.unsat_core(self._get_solver())

        return tuple(unsat_core)

    #
    # Serialization and such.
    #

    def downsize(self) -> None:
        ConstrainedFrontend.downsize(self)
        self._tls.solver = None
        self._to_add = []

    #
    # Merging and splitting
    #

    def merge(self, others, merge_conditions, common_ancestor=None) -> tuple[bool, Self]:
        return (
            self._solver_backend.__class__.__name__ == "BackendZ3",
            ConstrainedFrontend.merge(self, others, merge_conditions, common_ancestor=common_ancestor)[1],
        )

    #
    # Default model hook
    #

    def _model_hook(self, m):  # pylint:disable=unused-argument,no-self-use
        return None
