from __future__ import annotations

from claripy import backends, frontend


class Solver(
    frontend.mixin.ConcreteHandlerMixin,
    frontend.mixin.EagerResolutionMixin,
    frontend.mixin.ConstraintFilterMixin,
    frontend.mixin.ConstraintDeduplicatorMixin,
    frontend.mixin.SimplifySkipperMixin,
    frontend.mixin.SatCacheMixin,
    frontend.mixin.ModelCacheMixin,
    frontend.mixin.ConstraintExpansionMixin,
    frontend.mixin.SimplifyHelperMixin,
    frontend.FullFrontend,
):
    """Solver is the default Claripy frontend. It uses Z3 as the backend solver by default."""

    def __init__(self, backend=backends.z3, **kwargs):
        super().__init__(backend, **kwargs)


class SolverCacheless(
    frontend.mixin.ConcreteHandlerMixin,
    frontend.mixin.EagerResolutionMixin,
    frontend.mixin.ConstraintFilterMixin,
    frontend.mixin.ConstraintDeduplicatorMixin,
    frontend.mixin.SimplifySkipperMixin,
    frontend.FullFrontend,
):
    """SolverCacheless is a Solver without caching. It uses Z3 as the backend solver by default."""

    def __init__(self, backend=backends.z3, **kwargs):
        super().__init__(backend, **kwargs)


class SolverReplacement(
    frontend.mixin.ConcreteHandlerMixin,
    frontend.mixin.ConstraintDeduplicatorMixin,
    frontend.ReplacementFrontend,
):
    """SolverReplacement is a frontend wrapper that replaces constraints with their solutions."""

    def __init__(self, actual_frontend=None, **kwargs):
        actual_frontend = Solver() if actual_frontend is None else actual_frontend
        super().__init__(actual_frontend, **kwargs)


class SolverHybrid(
    frontend.mixin.ConcreteHandlerMixin,
    frontend.mixin.EagerResolutionMixin,
    frontend.mixin.ConstraintFilterMixin,
    frontend.mixin.ConstraintDeduplicatorMixin,
    frontend.mixin.SimplifySkipperMixin,
    # TODO: frontend.mixin.ConstraintExpansionMixin,
    frontend.HybridFrontend,
):
    """SolverHybrid is a frontend that uses an exact solver and an approximate solver."""

    def __init__(  # pylint:disable=too-many-positional-arguments
        self,
        exact_frontend=None,
        approximate_frontend=None,
        complex_auto_replace=True,
        replace_constraints=True,
        track=False,
        approximate_first=False,
        **kwargs,
    ):
        exact_frontend = Solver(track=track) if exact_frontend is None else exact_frontend
        approximate_frontend = (
            SolverReplacement(
                actual_frontend=SolverVSA(),
                complex_auto_replace=complex_auto_replace,
                replace_constraints=replace_constraints,
            )
            if approximate_frontend is None
            else approximate_frontend
        )
        super().__init__(exact_frontend, approximate_frontend, approximate_first=approximate_first, **kwargs)


class SolverVSA(
    frontend.mixin.ConcreteHandlerMixin,
    frontend.mixin.ConstraintFilterMixin,
    frontend.LightFrontend,
):
    """SolverVSA is a thin frontend to the VSA backend solver."""

    def __init__(self, **kwargs):
        super().__init__(backends.vsa, **kwargs)


class SolverConcrete(
    frontend.mixin.ConcreteHandlerMixin,
    frontend.mixin.ConstraintFilterMixin,
    frontend.LightFrontend,
):
    """SolverConcrete is a thin frontend to the Concrete backend solver."""

    def __init__(self, **kwargs):
        super().__init__(backends.concrete, **kwargs)


class SolverStrings(
    # TODO: Figure ot if we need to use all these frontend.mixins
    frontend.mixin.ConcreteHandlerMixin,
    frontend.mixin.ConstraintFilterMixin,
    frontend.mixin.ConstraintDeduplicatorMixin,
    frontend.mixin.EagerResolutionMixin,
    frontend.FullFrontend,
):
    """SolverStrings is a frontend that uses Z3 to solve string constraints."""

    def __init__(self, *args, backend=backends.z3, **kwargs):
        super().__init__(backend, *args, **kwargs)


#
# Composite solving
#


class SolverCompositeChild(
    frontend.mixin.ConstraintDeduplicatorMixin,
    frontend.mixin.SatCacheMixin,
    frontend.mixin.SimplifySkipperMixin,
    frontend.mixin.ModelCacheMixin,
    frontend.FullFrontend,
):
    """SolverCompositeChild is a frontend that is used as a child in a SolverComposite."""

    def __init__(self, backend=backends.z3, **kwargs):
        super().__init__(backend, **kwargs)

    def __repr__(self):
        return f"<SolverCompositeChild with {len(self.variables)} variables>"


class SolverComposite(
    frontend.mixin.ConcreteHandlerMixin,
    frontend.mixin.EagerResolutionMixin,
    frontend.mixin.ConstraintFilterMixin,
    frontend.mixin.ConstraintDeduplicatorMixin,
    frontend.mixin.SatCacheMixin,
    frontend.mixin.SimplifySkipperMixin,
    frontend.mixin.SimplifyHelperMixin,
    frontend.mixin.ConstraintExpansionMixin,
    frontend.mixin.CompositedCacheMixin,
    frontend.CompositeFrontend,
):
    """SolverComposite is a frontend that composes multiple templated frontends."""

    def __init__(self, template_solver=None, track=False, **kwargs):
        template_solver = SolverCompositeChild(track=track) if template_solver is None else template_solver
        super().__init__(template_solver, track=track, **kwargs)

    def __repr__(self):
        return f"<SolverComposite {id(self):#x}, {len(self._solver_list)} children>"
