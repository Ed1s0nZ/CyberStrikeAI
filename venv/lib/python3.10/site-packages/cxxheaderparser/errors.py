import typing

if typing.TYPE_CHECKING:
    from .lexer import LexToken


class CxxParseError(Exception):
    """
    Exception raised when a parsing error occurs
    """

    def __init__(self, msg: str, tok: typing.Optional["LexToken"] = None) -> None:
        Exception.__init__(self, msg)
        self.tok = tok
