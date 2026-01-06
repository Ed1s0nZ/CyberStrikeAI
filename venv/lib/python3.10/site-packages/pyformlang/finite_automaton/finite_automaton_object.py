"""
Represents an object of a finite state automaton
"""

from typing import Any


class FiniteAutomatonObject:  # pylint: disable=too-few-public-methods
    """ Represents an object in a finite state automaton

    Parameters
    ----------
    value: any
        The value of the object
    """

    def __init__(self, value: Any):
        self._value = value
        self._hash = None

    def __repr__(self) -> str:
        return str(self._value)

    @property
    def value(self) -> Any:
        """ Gets the value of the object

        Returns
        ---------
        value : any
            The value of the object
        """
        return self._value
