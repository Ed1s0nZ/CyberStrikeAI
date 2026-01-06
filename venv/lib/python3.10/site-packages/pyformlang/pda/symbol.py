""" A Symbol in a pushdown automaton """


class Symbol:
    """ A Symbol in a pushdown automaton

    Parameters
    ----------
    value : any
        The value of the state

    """

    def __init__(self, value):
        self._value = value

    def __hash__(self):
        return hash(str(self._value))

    @property
    def value(self):
        """ Returns the value of the symbol

        Returns
        ----------
        value: The value
            any
        """
        return self._value

    def __eq__(self, other):
        if isinstance(other, Symbol):
            return self._value == other.value
        return False

    def __repr__(self):
        return "Symbol(" + str(self._value) + ")"
