
class Error(Exception):
    """Base class for errors in the bitstring module."""

    def __init__(self, *params: object) -> None:
        self.msg = params[0] if params else ''
        self.params = params[1:]


class ReadError(Error, IndexError):
    """Reading or peeking past the end of a bitstring."""


InterpretError = ValueError
"""Inappropriate interpretation of binary data."""


class ByteAlignError(Error):
    """Whole-byte position or length needed."""


CreationError = ValueError
"""Inappropriate argument during bitstring creation."""
