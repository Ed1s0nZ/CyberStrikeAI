""" A StackSymbol in a pushdown automaton """


class StackSymbol:
    """ A StackSymbol in a pushdown automaton

    Parameters
    ----------
    value : any
        The value of the state

    """

    def __init__(self, value):
        self._value = value
        self._hash = None
        self.index_cfg_converter = None

    @property
    def value(self):
        """ Returns the value of the stack symbol

        Returns
        ----------
        value: The value
            any
        """
        return self._value

    def __hash__(self):
        if self._hash is None:
            self._hash = hash(self._value)
        return self._hash

    def __eq__(self, other):
        return self._value == other.value

    def __repr__(self):
        return "StackSymbol(" + str(self._value) + ")"
