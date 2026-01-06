"""
Represents an epsilon transition
"""

from .symbol import Symbol


class Epsilon(Symbol):  # pylint: disable=too-few-public-methods
    """ An epsilon transition

    Examples
    --------

    >>> epsilon = Epsilon()

    """

    def __init__(self):
        super().__init__("epsilon")

    def __hash__(self):
        return hash("EPSILON TRANSITION")

    def __eq__(self, other):
        if isinstance(other, Epsilon):
            return True
        return False
