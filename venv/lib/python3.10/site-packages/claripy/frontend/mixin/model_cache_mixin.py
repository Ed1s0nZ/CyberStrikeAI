from __future__ import annotations

import itertools
import weakref

import claripy
from claripy import backends, errors, false
from claripy.ast import Base
from claripy.errors import UnsatError


class ModelCache:
    """ModelCache is a wrapper around a model that provides some additional functionality."""

    def __init__(self, model):
        self.model = model
        self.replacements = {}
        self.constraint_only_replacements = {}

    def __hash__(self):
        if not hasattr(self, "_hash"):
            self._hash = hash(frozenset(self.model.items()))  # pylint:disable=attribute-defined-outside-init
        return self._hash

    def __eq__(self, other):
        return self.model == other.model

    def __getstate__(self):
        return (self.model,)

    def __setstate__(self, s):
        self.model = s[0]
        self.replacements = {}
        self.constraint_only_replacements = {}

    #
    # Splitting support
    #

    def filter(self, variables):
        return ModelCache({k: self.model[k] for k in self.model if k in variables})

    @staticmethod
    def combine(*models):
        return ModelCache(dict(itertools.chain.from_iterable(m.model.items() for m in models)))

    #
    # Model-driven evaluation
    #

    def _leaf_op(self, a):
        if a.op == "BVS":
            return claripy.BVV(self.model.get(a.args[0], 0), a.length)
        if a.op == "BoolS":
            return claripy.BoolV(self.model.get(a.args[0], True))
        if a.op == "FPS":
            return claripy.FPV(self.model.get(a.args[0], 0.0), a.args[1])
        if a.op == "StringS":
            return claripy.StringV(self.model.get(a.args[0], ""))
        return a

    def _leaf_op_existonly(self, a):
        if a.op == "BVS":
            return claripy.BVV(self.model[a.args[0]], a.length)
        if a.op == "BoolS":
            return claripy.BoolV(self.model[a.args[0]])
        if a.op == "FPS":
            return claripy.FPV(self.model[a.args[0]], a.args[1])
        if a.op == "StringS":
            return claripy.StringV(self.model[a.args[0]])
        return a

    def eval_ast(self, ast, allow_unconstrained: bool = True):
        """
        Eval the ast, replacing symbols by their last value in the model.

        :param ast:                 The AST to evaluate.
        :param allow_unconstrained: When set to True, we will treat non-existent variables as unconstrained variables
                                    and will use arbitrary concrete values for them during evaluation. Otherwise, raise
                                    KeyErrors for non-existent variables.
        """

        if allow_unconstrained:
            new_ast = claripy.replace_dict(ast, self.replacements, leaf_operation=self._leaf_op)
        else:
            new_ast = claripy.replace_dict(
                ast, self.constraint_only_replacements, leaf_operation=self._leaf_op_existonly
            )
        return backends.concrete.eval(new_ast, 1)[0]

    def eval_constraints(self, constraints):
        """Returns whether the constraints is satisfied trivially by using the
        last model."""
        # eval_ast is concretizing symbols and evaluating them, this can raise
        # exceptions.
        try:
            return all(self.eval_ast(c) for c in constraints)
        except errors.ClaripyZeroDivisionError:
            return False

    def eval_list(self, asts, allow_unconstrained: bool = True) -> tuple:
        """
        Evaluate a list of ASTs.

        :param asts:                A list of ASTs to evaluate.
        :param allow_unconstrained: When set to True, we will treat non-existent variables as unconstrained variables
                                    and will use arbitrary concrete values for them during evaluation. Otherwise, raise
                                    KeyErrors for non-existent variables.
        :return:                    A tuple of evaluated results, one element per AST.
        """

        return tuple(self.eval_ast(c, allow_unconstrained=allow_unconstrained) for c in asts)


class ModelCacheMixin:
    """ModelCacheMixin is a mixin that caches models for a solver."""

    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self._models = set()
        self._exhausted = False
        self._eval_exhausted = weakref.WeakValueDictionary()
        self._max_exhausted = weakref.WeakValueDictionary()
        self._min_exhausted = weakref.WeakValueDictionary()
        self._max_signed_exhausted = weakref.WeakValueDictionary()
        self._min_signed_exhausted = weakref.WeakValueDictionary()

    def _blank_copy(self, c):
        super()._blank_copy(c)
        c._models = set()
        c._exhausted = False
        c._eval_exhausted = weakref.WeakValueDictionary()
        c._max_exhausted = weakref.WeakValueDictionary()
        c._min_exhausted = weakref.WeakValueDictionary()
        c._max_signed_exhausted = weakref.WeakValueDictionary()
        c._min_signed_exhausted = weakref.WeakValueDictionary()

    def _copy(self, c):
        super()._copy(c)
        c._models = set(self._models)
        c._exhausted = self._exhausted
        c._eval_exhausted = weakref.WeakValueDictionary(self._eval_exhausted)
        c._max_exhausted = weakref.WeakValueDictionary(self._max_exhausted)
        c._min_exhausted = weakref.WeakValueDictionary(self._min_exhausted)
        c._max_signed_exhausted = weakref.WeakValueDictionary(self._max_signed_exhausted)
        c._min_signed_exhausted = weakref.WeakValueDictionary(self._min_signed_exhausted)

    def __setstate__(self, base_state):
        super().__setstate__(base_state)
        self._models = set()
        self._exhausted = False
        self._eval_exhausted = weakref.WeakValueDictionary()
        self._max_exhausted = weakref.WeakValueDictionary()
        self._min_exhausted = weakref.WeakValueDictionary()
        self._max_signed_exhausted = weakref.WeakValueDictionary()
        self._min_signed_exhausted = weakref.WeakValueDictionary()

    #
    # Model cleaning
    #

    def simplify(self):
        results = super().simplify()
        if len(results) > 0 and any(c is false() for c in results):
            self._models.clear()
        return results

    def _trivial_model_optimization(self):
        c = self.constraints[0]
        if not (
            c.depth == 2
            and c.op == "__eq__"
            and len(c.variables) == 1
            and c.args[0].symbolic
            and not c.args[1].symbolic
            and c.args[0].op == "BVS"
        ):
            return

        self._models.add(ModelCache({next(iter(c.args[0].variables)): backends.concrete.eval(c.args[1], 1)[0]}))
        self._eval_exhausted[c.args[0].hash()] = c.args[0]
        self._max_exhausted[c.args[0].hash()] = c.args[0]
        self._min_exhausted[c.args[0].hash()] = c.args[0]
        self._max_signed_exhausted[c.args[0].hash()] = c.args[0]
        self._min_signed_exhausted[c.args[0].hash()] = c.args[0]

    def _add(self, constraints, invalidate_cache=True):
        if len(constraints) == 0:
            return constraints

        old_vars = frozenset(self.variables)
        added = super()._add(constraints, invalidate_cache=invalidate_cache)
        if len(added) == 0:
            return added

        if len(self.constraints) == 1 and len(self._models) == 0:
            self._trivial_model_optimization()

        new_vars = any(a.variables - old_vars for a in added)
        if new_vars or invalidate_cache:
            # shortcut for unsat
            if any(c is false() for c in constraints):
                self._models.clear()

            still_valid = set(self._get_models(extra_constraints=added))
            if len(still_valid) != len(self._models):
                self._exhausted = False
                self._eval_exhausted.clear()
                self._max_exhausted.clear()
                self._min_exhausted.clear()
                self._max_signed_exhausted.clear()
                self._min_signed_exhausted.clear()
                self._models = still_valid

        return added

    def split(self):
        results = super().split()
        for r in results:
            r._models = {m.filter(r.variables) for m in self._models}
        return results

    def combine(self, others):
        combined = super().combine(others)

        if any(len(o._models) == 0 for o in others) or len(self._models) == 0:
            # this would need a solve anyways, so screw it
            return combined

        vars_count = len(self.variables) + sum(len(s.variables) for s in others)
        all_vars = self.variables.union(*[s.variables for s in others])
        if vars_count != len(all_vars):
            # this is the case where there are variables missing from the models.
            # We'll need more intelligence here to handle it
            return combined

        model_lists = [self._models]
        model_lists.extend(o._models for o in others)
        combined._models.update(
            itertools.starmap(ModelCache.combine, itertools.islice(itertools.product(*model_lists), len(self._models)))
        )
        return combined

    def update(self, other):
        """
        Updates this cache mixin with results discovered by the other split off one.
        """

        acceptable_models = [m for m in other._models if set(m.model.keys()) == self.variables]
        self._models.update(acceptable_models)
        self._eval_exhausted.update(other._eval_exhausted)
        self._max_exhausted.update(other._max_exhausted)
        self._min_exhausted.update(other._min_exhausted)
        self._max_signed_exhausted.update(other._max_signed_exhausted)
        self._min_signed_exhausted.update(other._min_signed_exhausted)

    #
    # Cache retrieval
    #

    def _model_hook(self, m):
        # Z3 might give us solutions for variables that we did not ask for. so we create a new dict with solutions for
        # only the variables that are under the solver's control
        m_ = {k: v for k, v in m.items() if k in self.variables}
        if m_:
            model = ModelCache(m_)
            self._models.add(model)

    def _get_models(self, extra_constraints=()):
        for m in self._models:
            if m.eval_constraints(extra_constraints):
                yield m

    def _get_batch_solutions(self, asts, n=None, extra_constraints=(), allow_unconstrained=True):
        results = set()

        for m in self._get_models(extra_constraints):
            try:
                results.add(m.eval_list(asts, allow_unconstrained=allow_unconstrained))
            except (ZeroDivisionError, KeyError):
                continue
            if len(results) == n:
                break

        return results

    def _get_solutions(self, e, n=None, extra_constraints=(), allow_unconstrained=True):
        return tuple(
            v[0]
            for v in self._get_batch_solutions(
                [e],
                n=n,
                extra_constraints=extra_constraints,
                allow_unconstrained=allow_unconstrained,
            )
        )

    #
    # Cached functions
    #

    def satisfiable(self, extra_constraints=(), exact=None):
        for _ in self._get_models(extra_constraints=extra_constraints):
            return True
        return super().satisfiable(extra_constraints=extra_constraints, exact=exact)

    def batch_eval(self, asts, n, extra_constraints=(), exact=None):
        results = self._get_batch_solutions(asts, n=n, extra_constraints=extra_constraints)

        if len(results) == n or (len(asts) == 1 and asts[0].hash() in self._eval_exhausted):
            return results

        remaining = n - len(results)

        # TODO: faster to concat?
        if len(results) != 0:
            constraints = (
                claripy.And(*[claripy.Or(*[a != v for a, v in zip(asts, r, strict=False)]) for r in results]),
                *tuple(extra_constraints),
            )
        else:
            constraints = extra_constraints

        try:
            results.update(super().batch_eval(asts, remaining, extra_constraints=constraints, exact=exact))
        except UnsatError:
            if len(results) == 0:
                raise

        if len(extra_constraints) == 0 and len(results) < n:
            for e in asts:
                # only mark an AST as eval-exhausted if e.variables is a subset of variables that the current solver
                # knows about (from its constraints)
                if self.variables.issuperset(e.variables):
                    self._eval_exhausted[e.hash()] = e

        return results

    def eval(self, e, n, extra_constraints=(), exact=None):
        return tuple(
            r[0] for r in ModelCacheMixin.batch_eval(self, [e], n, extra_constraints=extra_constraints, exact=exact)
        )

    def min(self, e, extra_constraints=(), signed=False, exact=None):
        cached = []
        if e.hash() in self._eval_exhausted or e.hash() in self._min_exhausted:
            # we set allow_unconstrained to False because we expect all returned values for e are returned by Z3,
            # instead of some arbitrarily assigned concrete values.
            cached = self._get_solutions(e, extra_constraints=extra_constraints, allow_unconstrained=False)

        if len(cached) > 0:

            def signed_key(v):
                return v if v >= 0 else v + 2 ** len(e)

            return min(cached, key=signed_key if signed else lambda v: v)

        m = super().min(e, extra_constraints=extra_constraints, signed=signed, exact=exact)
        if len(extra_constraints) == 0:
            (self._min_signed_exhausted if signed else self._min_exhausted)[e.hash()] = e
        return m

    def max(self, e, extra_constraints=(), signed=False, exact=None):
        cached = []
        if e.hash() in self._eval_exhausted or e.hash() in self._max_exhausted:
            cached = self._get_solutions(e, extra_constraints=extra_constraints, allow_unconstrained=False)

        if len(cached) > 0:

            def signed_key(v):
                return v if v < 2 ** (len(e) - 1) else v - 2 ** len(e)

            return max(cached, key=signed_key if signed else lambda v: v)

        m = super().max(e, extra_constraints=extra_constraints, signed=signed, exact=exact)
        if len(extra_constraints) == 0:
            (self._max_signed_exhausted if signed else self._max_exhausted)[e.hash()] = e
        return m

    def solution(self, e, v, extra_constraints=(), exact=None):
        if isinstance(v, Base):
            cached = self._get_batch_solutions([e, v], extra_constraints=extra_constraints)
            if any(ec == vc for ec, vc in cached):
                return True
        else:
            cached = self._get_solutions(e, extra_constraints=extra_constraints)
            if v in cached:
                return True

        return super().solution(e, v, extra_constraints=extra_constraints, exact=exact)
