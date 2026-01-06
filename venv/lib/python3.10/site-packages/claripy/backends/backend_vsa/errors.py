from __future__ import annotations

from claripy.errors import ClaripyError


class ClaripyVSAError(ClaripyError):
    """ClaripyVSAError is the base class for all errors raised by claripy's
    VSA backend.
    """


class ClaripyVSAOperationError(ClaripyVSAError):
    """ClaripyVSAOperationError is raised when an operation is attempted on VSA
    objects that is not supported.
    """
