"""
Representation of a state in a finite state automaton
"""

from typing import Any
from .finite_automaton_object import FiniteAutomatonObject


class State(FiniteAutomatonObject):  # pylint: disable=too-few-public-methods
    """ A state in a finite automaton

    Parameters
    ----------
    value : any
        The value of the state

    Examples
    ----------
    >>> from pyformlang.finite_automaton import State
    >>> State("A")
    A

    """

    def __init__(self, value):
        super().__init__(value)
        self.index = None
        self.index_cfg_converter = None

    def __hash__(self) -> int:
        if self._hash is None:
            self._hash = hash(self._value)
        return self._hash

    def __eq__(self, other: Any) -> bool:
        if isinstance(other, State):
            return self._value == other._value
        return self._value == other
