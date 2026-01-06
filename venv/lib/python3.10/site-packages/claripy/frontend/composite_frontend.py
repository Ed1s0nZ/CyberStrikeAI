from __future__ import annotations

import itertools
import logging
import weakref
from typing import TYPE_CHECKING

from claripy import Or, backends
from claripy.ast import Base
from claripy.errors import BackendError, UnsatError

from .constrained_frontend import ConstrainedFrontend
from .mixin import ModelCacheMixin, SimplifySkipperMixin

if TYPE_CHECKING:
    from claripy.solvers import SolverCompositeChild


log = logging.getLogger(__name__)
symbolic_count = itertools.count()


class CompositeFrontend(ConstrainedFrontend):
    """Composite Solver splits constraints into independent sets, allowing
    caching to be done on a per-set basis. Additionally, by allowing constraints
    to be solved independently, the solver can be more efficient in some cases
    ("divide and conquer").

    For example, the constraints (a==b, b==1, c==4) would be split into two
    sets: one containing a==b and b==1, and the other containing c==4.
    """

    def __init__(self, template_frontend, track=False, **kwargs):
        super().__init__(**kwargs)
        self._solvers = {}
        self._unchecked_solvers = weakref.WeakSet()
        self._owned_solvers = weakref.WeakSet()
        self._template_frontend = template_frontend
        self._unsat = False
        self._track = track

    def _blank_copy(self, c):
        super()._blank_copy(c)
        c._unchecked_solvers = weakref.WeakSet()
        c._owned_solvers = weakref.WeakSet()
        c._solvers = {}
        c._template_frontend = self._template_frontend
        c._unsat = False
        c._track = self._track

    def _copy(self, c):
        super()._copy(c)
        c._unsat = self._unsat
        c._track = self._track

        c._solvers = dict(self._solvers)
        c._unchecked_solvers = weakref.WeakSet(self._unchecked_solvers)
        self._owned_solvers = weakref.WeakSet()  # for the COW
        return c

    #
    # Serialization stuff
    #

    def __getstate__(self):
        return self._solvers, self._template_frontend, self._unsat, self._track, super().__getstate__()

    def __setstate__(self, s):
        self._solvers, self._template_frontend, self._unsat, self._track, base_state = s
        self._owned_solvers = weakref.WeakSet(self._solver_list)
        self._unchecked_solvers = weakref.WeakSet()
        super().__setstate__(base_state)

    def downsize(self):
        for e in self._solver_list:
            e.downsize()

    #
    # Frontend management
    #

    @property
    def _solver_list(self):
        seen_solvers = set()
        solver_list = []
        for s in self._solvers.values():
            if id(s) in seen_solvers:
                continue
            seen_solvers.add(id(s))
            solver_list.append(s)
        return solver_list

    @property
    def variables(self):
        return set(self._solvers.keys())

    # this is really hacky, but we want to avoid having our variables messed with
    @variables.setter
    def variables(self, v):
        pass

    #
    # Solver list management
    #

    def _solvers_for_variables(self, names):
        seen_solvers = set()
        existing_solvers = []
        for n in names:
            if n not in self._solvers:
                continue
            s = self._solvers[n]

            if id(s) in seen_solvers:
                continue
            seen_solvers.add(id(s))
            existing_solvers.append(s)
        return existing_solvers

    @staticmethod
    def _names_for(names=None, lst=None, lst2=None, e=None, v=None) -> set[str]:
        if names is None:
            names = set()
        if e is not None and isinstance(e, Base):
            names.update(e.variables)
        if v is not None and isinstance(v, Base):
            names.update(v.variables)
        if lst is not None:
            for ee in lst:
                if isinstance(ee, Base):
                    names.update(ee.variables)
        if lst2 is not None:
            for ee in lst2:
                if isinstance(ee, Base):
                    names.update(ee.variables)
        return names

    def _merged_solver_for(self, *args, **kwargs):
        return self._solver_for_names(self._names_for(*args, **kwargs))

    def _solver_for_names(self, names: set[str]) -> SolverCompositeChild:
        """
        Get a merged child solver for variables specified in `names`.

        :param names:   A set of variable names.
        :return:        A composite child solver.
        """

        log.debug("composite_solver._merged_solver_for() running with %d names", len(names))

        # compute a transitive closure for all variable names
        all_names = set(names)
        new_names = names
        solvers = set()
        while True:
            tmp_solvers = self._solvers_for_variables(new_names)
            solvers |= set(tmp_solvers)
            all_names |= new_names
            tmp_names = set()
            for solver in solvers:
                tmp_names |= solver.variables
            new_names = tmp_names.difference(all_names)
            if not new_names:
                break

        solvers = list(solvers)

        if len(solvers) == 0:
            log.debug("... creating new solver")
            return self._template_frontend.blank_copy()
        if len(solvers) == 1:
            log.debug("... got one solver")
            return solvers[0]
        log.debug(".... combining %d solvers", len(solvers))
        return solvers[0].combine(solvers[1:])

    def _shared_solvers(self, others):
        """
        Returns a sequence of the solvers that self and others share.
        """

        solvers_by_id = {id(s): s for s in self._solver_list}
        common_solvers = set(solvers_by_id.keys())
        other_sets = [{id(s) for s in cs._solver_list} for cs in others]
        for o in other_sets:
            common_solvers &= o

        return [solvers_by_id[s] for s in common_solvers]

    def _variable_sets(self):
        return {s.variables for s in self._solver_list}

    def _shared_varsets(self, others):
        common_varsets = self._variable_sets()
        for o in others:
            common_varsets &= o.all_varsets()
        return common_varsets

    def _split_child(self, s):
        ss = s.split()
        if len(ss) == 1:
            return [s]

        log.debug("... split solver %r into %d parts", s, len(ss))
        log.debug("... variable counts: %s", [len(cs.variables) for cs in ss])

        for ns in ss:
            self._owned_solvers.add(ns)
            self._store_child(ns)

        return ss

    def _reabsorb_solver(self, s):
        try:
            if len(s.variables) == 0 or self._solvers[min(iter(s.variables))] is s:
                return
        except KeyError:
            # this happens when a variable is introduced due to constraint expansion
            return

        if isinstance(s, ModelCacheMixin):
            new_solvers = s.split()
            old_solvers = self._solvers_for_variables(s.variables)
            if len(new_solvers) == len(old_solvers):
                done = set()
                for ss in new_solvers:
                    if ss in done:
                        continue
                    done.add(ss)
                    v = min(iter(ss.variables))
                    self._solvers[v].update(ss)
            else:
                for ns in new_solvers:
                    self._owned_solvers.add(ns)
                    self._store_child(ns)

    def _store_child(self, ns, extra_names=frozenset(), invalidate_cache=True):
        for v in ns.variables | extra_names:
            # os = self._solvers[v]
            self._solvers[v] = ns
        if invalidate_cache:
            self._unchecked_solvers.add(ns)

        # if isinstance(s, ModelCacheMixin):
        #   if len(os._models) < len(ns._models):
        #       print("GOT %d NEW MODELS (before: %d)" % (
        #           len(ns._models) - len(os._models), len(os._models)
        #       ))
        #   elif len(os._models) > len(ns._models):
        #       print("WARNING: LOST %d NEW MODELS (before: %d)" % (
        #           len(os._models) - len(ns._models), len(os._models)
        #       ))
        #   else:
        #       print("Remained at %d models." % len(os._models))

    #
    # Constraints
    #

    def _claim(self, s):
        if s not in self._owned_solvers:
            sc = s.branch()
            self._owned_solvers.add(sc)
            return sc
        return s

    def _add_dependent_constraints(self, names, constraints, invalidate_cache=True, **kwargs):
        if not invalidate_cache and len(self._solvers_for_variables(names)) > 1:
            log.debug("Ignoring cross-solver helper constraints.")
            return []

        log.debug("Adding %d constraints to %d names", len(constraints), len(names))
        s = self._claim(self._merged_solver_for(names=names))
        added = s.add(constraints, invalidate_cache=invalidate_cache, **kwargs)
        self._store_child(s, invalidate_cache=invalidate_cache)
        return added

    def _add(self, constraints, invalidate_cache=True):
        split = self._split_constraints(constraints)
        child_added = []

        # l.debug("%s, solvers before: %d", self, len(self._solvers))
        unsure = []
        for names, set_constraints in split:
            if names == {"CONCRETE"}:
                try:
                    if any(backends.concrete.convert(c) is False for c in set_constraints):
                        self._unsat = True
                except BackendError:
                    unsure.extend(set_constraints)
            else:
                child_added += self._add_dependent_constraints(names, set_constraints)

        # l.debug("... solvers after add: %d", len(self._solver_list))

        if len(unsure) > 0:
            for s in self._solver_list:
                s = self._claim(s)
                s.add(unsure)
                self._store_child(s)

        return super()._add(child_added)

    #
    # Solving
    #

    def _ensure_sat(self, extra_constraints):
        if self._unsat or (len(extra_constraints) == 0 and not self.satisfiable()):
            raise UnsatError("CompositeSolver is already unsat")

    def check_satisfiability(self, extra_constraints=(), exact=None):
        if self._unsat:
            return "UNSAT"

        log.debug("%r checking satisfiability...", self)

        if len(extra_constraints) != 0:
            extra_solver = self._merged_solver_for(lst=extra_constraints)
            extra_solver_satness = extra_solver.check_satisfiability(extra_constraints=extra_constraints, exact=exact)
            if extra_solver_satness in {"UNSAT", "UNKNOWN"}:
                return extra_solver_satness
            self._reabsorb_solver(extra_solver)

        for s in self._unchecked_solvers:
            if extra_constraints and s.variables & extra_solver.variables:
                # skip solvers covered by extra constraints (they were checked above)
                continue

            if len(s.variables) == 0 or self._solvers[min(iter(s.variables))] is not s:
                # this happens when a parent solver didn't check all unchecked solvers, and we have stale
                # child solvers in the unchecked list
                continue

            satness = s.check_satisfiability(exact=exact)
            if satness in {"UNSAT", "UNKNOWN"}:
                return satness

        self._unchecked_solvers.clear()
        return "SAT"

    def satisfiable(self, extra_constraints=(), exact=None):
        return self.check_satisfiability(extra_constraints=extra_constraints, exact=exact) == "SAT"

    def eval(self, e, n, extra_constraints=(), exact=None):
        self._ensure_sat(extra_constraints=extra_constraints)

        ms = self._merged_solver_for(e=e, lst=extra_constraints)
        r = ms.eval(e, n, extra_constraints=extra_constraints, exact=exact)
        self._reabsorb_solver(ms)
        return r

    def batch_eval(self, exprs, n, extra_constraints=(), exact=None):
        self._ensure_sat(extra_constraints=extra_constraints)

        ms = self._merged_solver_for(lst2=exprs, lst=extra_constraints)
        r = ms.batch_eval(exprs, n, extra_constraints=extra_constraints, exact=exact)
        self._reabsorb_solver(ms)
        return r

    def max(self, e, extra_constraints=(), signed=False, exact=None):
        self._ensure_sat(extra_constraints=extra_constraints)

        ms = self._merged_solver_for(e=e, lst=extra_constraints)
        r = ms.max(e, extra_constraints=extra_constraints, signed=signed, exact=exact)
        self._reabsorb_solver(ms)
        return r

    def min(self, e, extra_constraints=(), signed=False, exact=None):
        self._ensure_sat(extra_constraints=extra_constraints)

        ms = self._merged_solver_for(e=e, lst=extra_constraints)
        r = ms.min(e, extra_constraints=extra_constraints, signed=signed, exact=exact)
        self._reabsorb_solver(ms)
        return r

    def solution(self, e, v, extra_constraints=(), exact=None):
        self._ensure_sat(extra_constraints=extra_constraints)

        ms = self._merged_solver_for(e=e, v=v, lst=extra_constraints)
        r = ms.solution(e, v, extra_constraints=extra_constraints, exact=exact)
        self._reabsorb_solver(ms)
        return r

    def is_true(self, e, extra_constraints=(), exact=None):
        # self._ensure_sat(extra_constraints=extra_constraints)

        ms = self._merged_solver_for(e=e, lst=extra_constraints)
        return ms.is_true(e, extra_constraints=extra_constraints, exact=exact)
        # self._reabsorb_solver(ms)

    def is_false(self, e, extra_constraints=(), exact=None):
        # self._ensure_sat(extra_constraints=extra_constraints)

        ms = self._merged_solver_for(e=e, lst=extra_constraints)
        return ms.is_false(e, extra_constraints=extra_constraints, exact=exact)
        # self._reabsorb_solver(ms)

    def unsat_core(self, extra_constraints=()):
        if self.satisfiable(extra_constraints=extra_constraints):
            return ()

        cores = []

        for solver in self._solver_list:
            cores.extend(list(solver.unsat_core(extra_constraints=extra_constraints)))

        return cores

    def simplify(self):
        if self._unsat:
            return self.constraints

        new_constraints = []

        log.debug("Simplifying %r with %d solvers", self, len(self._solver_list))
        for s in self._solver_list:
            if isinstance(s, SimplifySkipperMixin) and s._simplified:
                new_constraints += s.constraints
                continue

            log.debug("... simplifying child solver %r", s)
            s.simplify()
            results = self._split_child(s)
            for ns in results:
                if isinstance(ns, SimplifySkipperMixin):
                    ns._simplified = True
            new_constraints += s.constraints

        log.debug("... after-split, %r has %d solvers", self, len(self._solver_list))

        self.constraints = new_constraints
        return new_constraints

    #
    # Merging and splitting
    #

    def finalize(self):
        for s in self._solver_list:
            s.finalize()

    @property
    def timeout(self):
        return self._template_frontend.timeout

    @timeout.setter
    def timeout(self, t):
        self._template_frontend.timeout = t
        for s in self._solver_list:
            s.timeout = t

    @property
    def max_memory(self):
        return self._template_frontend.max_memory

    @max_memory.setter
    def max_memory(self, val):
        # this is technically wrong. we cannot enforce a memory limit for a pool shared among multiple solvers
        self._template_frontend.max_memory = val
        for s in self._solver_list:
            s.max_memory = val

    @staticmethod
    def _merge_with_ancestor(common_ancestor, merge_conditions):
        merged = common_ancestor.branch()
        merged.add([Or(*merge_conditions)])
        # import ipdb; ipdb.set_trace()
        return True, merged

    def merge(self, others, merge_conditions, common_ancestor=None):
        if common_ancestor is not None:
            return self._merge_with_ancestor(common_ancestor, merge_conditions)

        log.debug("Merging %s with %d other solvers.", self, len(others))
        merged = self.blank_copy()
        common_solvers = self._shared_solvers(others)
        common_ids = {id(s) for s in common_solvers}
        log.debug("... %s common solvers", len(common_solvers))

        for s in common_solvers:
            self._owned_solvers.discard(s)
            for o in others:
                o._owned_solvers.discard(s)

            for v in s.variables:
                merged._solvers[v] = s

        noncommon_solvers = [[s for s in cs._solver_list if id(s) not in common_ids] for cs in [self, *others]]

        log.debug("... merging noncommon solvers")
        combined_noncommons = []
        for ns in noncommon_solvers:
            log.debug("... %d", len(ns))
            if len(ns) == 0:
                s = self._template_frontend.blank_copy()
                combined_noncommons.append(s)
            elif len(ns) == 1:
                combined_noncommons.append(ns[0])
            else:
                combined_noncommons.append(ns[0].combine(ns[1:]))

        if len(combined_noncommons):
            _, merged_noncommon = combined_noncommons[0].merge(combined_noncommons[1:], merge_conditions)

            merged._owned_solvers.add(merged_noncommon)
            merged._store_child(merged_noncommon)

        merged.constraints = list(itertools.chain.from_iterable(a.constraints for a in merged._solver_list))
        return True, merged

    def split(self):
        return [s.branch() for s in self._solver_list]
