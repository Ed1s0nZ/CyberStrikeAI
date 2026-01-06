from __future__ import annotations

import logging
from contextlib import suppress
from typing import TYPE_CHECKING

import claripy
from claripy.errors import BackendError

if TYPE_CHECKING:
    from claripy.ast import Bool

log = logging.getLogger(__name__)


def is_true(expr: Bool) -> bool:
    """Checks if a boolean expression is trivially True.

    A false result does not necessarily mean that the expression is False, but
    rather that it is not trivially True.
    """

    with suppress(BackendError):
        return claripy.backends.concrete.is_true(expr)

    log.debug("Unable to tell the truth-value of this expression")
    return False


def is_false(expr: Bool) -> bool:
    """Checks if a boolean expression is trivially False.

    A false result does not necessarily mean that the expression is False, but
    rather that it is not trivially False.
    """

    with suppress(BackendError):
        return claripy.backends.concrete.is_false(expr)

    log.debug("Unable to tell the truth-value of this expression")
    return False
