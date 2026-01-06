from __future__ import annotations

import logging
from itertools import chain
from typing import TypeVar, cast
from weakref import WeakValueDictionary

import claripy
from claripy.ast import Base
from claripy.errors import BackendError

log = logging.getLogger(__name__)

T = TypeVar("T", bound=Base)

simplification_cache: WeakValueDictionary[int, Base] = WeakValueDictionary()


def simplify(expr: T) -> T:
    """
    Simplify an expression.
    """

    if expr.is_leaf():
        return expr

    if expr.hash() in simplification_cache and simplification_cache[expr.hash()] is not None:
        return cast("T", simplification_cache[expr.hash()])

    try:
        simplified = claripy.backends.any_backend.simplify(expr)
    except BackendError:
        simplified = None
    if simplified is None:
        log.debug("Unable to simplify expression")
        simplification_cache[expr.hash()] = expr
        return expr

    # dealing with annotations
    if expr.annotations:
        ast_args = tuple(a for a in expr.args if isinstance(a, Base))
        annotations = tuple(
            set(
                chain(
                    chain.from_iterable(a._relocatable_annotations for a in ast_args),
                    tuple(a for a in expr.annotations),
                )
            )
        )
        if annotations != simplified.annotations:
            simplified = simplified.remove_annotations(simplified.annotations)
            simplified = simplified.annotate(*annotations)

    simplification_cache[expr.hash()] = simplified
    return simplified
