""" An object in a CFG (Variable and Terminal)"""

from typing import Any


class CFGObject:  # pylint: disable=too-few-public-methods
    """ An object in a CFG

    Parameters
    -----------
    value : any
        The value of the object
    """

    __slots__ = ["_value", "_hash"]

    def __init__(self, value: Any):
        self._value = value
        self._hash = None

    @property
    def value(self) -> Any:
        """Gets the value of the object"""
        return self._value

    def to_text(self) -> str:
        """ Turns the object into a text format """
        raise NotImplementedError
