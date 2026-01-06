from __future__ import annotations


class CompositedCacheMixin:
    """CompositedCacheMixin is a mixin for frontends that preserves caches from merged solvers."""

    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self._merged_solvers = {}

    def _blank_copy(self, c):
        super()._blank_copy(c)
        c._merged_solvers = {}

    def _copy(self, c):
        super()._copy(c)
        c._merged_solvers = dict(self._merged_solvers)

    def __setstate__(self, base_state):
        super().__setstate__(base_state)
        self._merged_solvers = {}

    #
    # Cache stuff
    #

    def _remove_cached(self, names):
        self._merged_solvers = {k: v for k, v in self._merged_solvers.items() if not k & names}

    def _solver_for_names(self, names):
        n = frozenset(names)
        try:
            return self._merged_solvers[n]
        except KeyError:
            s = super()._solver_for_names(names)
            self._merged_solvers[n] = s
            return s

    def downsize(self):
        super().downsize()
        self._merged_solvers = {}

    def _store_child(self, ns, extra_names=frozenset(), invalidate_cache=True):
        self._remove_cached(ns.variables)
        return super()._store_child(ns, extra_names=extra_names, invalidate_cache=invalidate_cache)
