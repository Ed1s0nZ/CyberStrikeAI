from __future__ import annotations

import logging

from claripy import And, Or
from claripy.algorithm import simplify
from claripy.annotation import SimplificationAvoidanceAnnotation

from .frontend import Frontend

log = logging.getLogger(__name__)


class ConstrainedFrontend(Frontend):
    """ConstrainedFrontend is a base class for all frontends that support constraints."""

    def __init__(self):
        Frontend.__init__(self)
        self.constraints = []
        self.constraints_wo_annotations: set[int] = set()
        self.variables = set()
        self._finalized = False

    def _blank_copy(self, c):
        super()._blank_copy(c)
        c.constraints = []
        c.constraints_wo_annotations = set()
        c.variables = set()
        c._finalized = False

    def _copy(self, c):
        super()._copy(c)
        c.constraints = list(self.constraints)
        c.constraints_wo_annotations = set(self.constraints_wo_annotations)
        c.variables = set(self.variables)

        # finalize both
        self.finalize()
        c.finalize()

    #
    # Serialization support
    #

    def __getstate__(self):
        return self.constraints, self.variables, self._finalized, super().__getstate__()

    def __setstate__(self, s):
        self.constraints, self.variables, self._finalized, base_state = s
        self.constraints_wo_annotations = {con.clear_annotations().hash() for con in self.constraints}
        super().__setstate__(base_state)

    #
    # Constraint management
    #

    def independent_constraints(self):
        return self._split_constraints(self.constraints)

    #
    # Serialization and such.
    #

    def downsize(self):
        Frontend.downsize(self)

    #
    # Merging and splitting
    #

    def finalize(self):
        self._finalized = True

    def merge(self, others, merge_conditions, common_ancestor=None):
        if common_ancestor is None:
            merged = self.blank_copy()
            options = []
            for s, v in zip([self, *others], merge_conditions, strict=False):
                options.append(And(*([v, *s.constraints])))
            merged.add([Or(*options)])
        else:
            merged = common_ancestor.branch()
            merged.add([Or(*merge_conditions)])

        return False, merged

    def combine(self, others):
        combined = self.blank_copy()

        combined.add(self.constraints)
        for o in others:
            combined.add(o.constraints)
        return combined

    def split(self):
        results = []
        log.debug("Splitting!")
        for variables, c_list in self.independent_constraints():
            log.debug("... got %d constraints with %d variables", len(c_list), len(variables))

            s = self.blank_copy()
            s.add(c_list)
            results.append(s)
        return results

    #
    # Light functionality
    #

    def _add(self, constraints, invalidate_cache=True):
        added = []
        for con in constraints:
            if not con.annotations and con.hash() in self.constraints_wo_annotations:
                continue
            self.constraints_wo_annotations.add(con.clear_annotations().hash())
            self.constraints.append(con)
            self.variables.update(con.variables)
            added.append(con)
        return added

    def simplify(self):
        to_simplify = [
            c
            for c in self.constraints
            if not any(isinstance(a, SimplificationAvoidanceAnnotation) for a in c.annotations)
        ]
        no_simplify = [
            c for c in self.constraints if any(isinstance(a, SimplificationAvoidanceAnnotation) for a in c.annotations)
        ]

        if len(to_simplify) == 0:
            return self.constraints

        simplified = simplify(And(*to_simplify))
        simplified_split = list(simplified.args) if simplified.op == "And" else [simplified]

        self.constraints = no_simplify + simplified_split
        return self.constraints

    #
    # Stuff that should be implemented by subclasses
    #

    def check_satisfiability(self, extra_constraints=(), exact=None):
        raise NotImplementedError("check_satisfiable() is not implemented")

    def satisfiable(self, extra_constraints=(), exact=None):
        raise NotImplementedError("satisfiable() is not implemented")

    def batch_eval(self, exprs, n, extra_constraints=(), exact=None):
        raise NotImplementedError("batch_eval() is not implemented")

    def eval(self, e, n, extra_constraints=(), exact=None):
        raise NotImplementedError("eval() is not implemented")

    def min(self, e, extra_constraints=(), signed=False, exact=None):
        raise NotImplementedError("min() is not implemented")

    def max(self, e, extra_constraints=(), signed=False, exact=None):
        raise NotImplementedError("max() is not implemented")

    def solution(self, e, v, extra_constraints=(), exact=None):
        raise NotImplementedError("solution() is not implemented")

    def is_true(self, e, extra_constraints=(), exact=None):
        raise NotImplementedError("is_true() is not implemented")

    def is_false(self, e, extra_constraints=(), exact=None):
        raise NotImplementedError("is_false() is not implemented")

    #
    # Some utility functions
    #

    @staticmethod
    def _split_constraints(constraints, concrete=True):
        """
        Returns independent constraints, split from this Frontend's `constraints`.
        """

        splitted = []
        for i in constraints:
            splitted.extend(list(i.args) if i.op == "And" else [i])

        log.debug("... splitted of size %d", len(splitted))

        concrete_constraints = []
        variable_connections = {}
        constraint_connections = {}
        for n, s in enumerate(splitted):
            log.debug("... processing constraint with %d variables", len(s.variables))

            connected_variables = set(s.variables)
            connected_constraints = {n}

            if len(connected_variables) == 0:
                concrete_constraints.append(s)

            for v in s.variables:
                if v in variable_connections:
                    connected_variables |= variable_connections[v]
                if v in constraint_connections:
                    connected_constraints |= constraint_connections[v]

            for v in connected_variables:
                variable_connections[v] = connected_variables
                constraint_connections[v] = connected_constraints

        unique_constraint_sets = set()
        for v, c in variable_connections.items():
            unique_constraint_sets.add((frozenset(c), frozenset(constraint_connections[v])))

        results = []
        for v, c_indexes in unique_constraint_sets:
            results.append((set(v), [splitted[c] for c in c_indexes]))

        if concrete and len(concrete_constraints) > 0:
            results.append(({"CONCRETE"}, concrete_constraints))

        return results
