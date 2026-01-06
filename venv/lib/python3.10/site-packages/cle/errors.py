from __future__ import annotations

__all__ = (
    "CLEError",
    "CLEUnknownFormatError",
    "CLEFileNotFoundError",
    "CLEInvalidBinaryError",
    "CLEOperationError",
    "CLECompatibilityError",
    "CLEMemoryError",
)


class CLEError(Exception):
    """
    Base class for errors raised by CLE.
    """

    pass


class CLEUnknownFormatError(CLEError):
    """
    Error raised when CLE encounters an unknown executable file format.
    """

    pass


class CLEFileNotFoundError(CLEError):
    """
    Error raised when a file does not exist.
    """

    pass


class CLEInvalidBinaryError(CLEError):
    """
    Error raised when an executable file is invalid or corrupted.
    """

    pass


class CLEOperationError(CLEError):
    """
    Error raised when a problem is encountered in the process of loading an executable.
    """

    pass


class CLECompatibilityError(CLEError):
    """
    Error raised when loading an executable that is not currently supported by CLE.
    """

    pass


class CLEMemoryError(CLEError):
    """
    Error raised when performing memory operations on unmapped addresses
    """

    pass


class CLEInvalidFileFormatError(CLEError):
    """
    Error raised when loading a file with an invalid format.
    """


class CLEInvalidEncryptionError(CLEError):
    """
    Error raised when loading an encrypted file (e.g., CART) with an incorrect encryption key.
    """

    def __init__(self, backend=None, enckey_argname=None):
        self.backend = backend
        self.enckey_argname = enckey_argname
