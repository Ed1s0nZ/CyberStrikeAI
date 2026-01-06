"""
Tests for the symbols
"""
from pyformlang.finite_automaton import Symbol


class TestSymbol:
    """ Tests for the symbols
    """

    def test_can_create(self):
        """ Tests the creation of symbols
        """
        assert Symbol("") is not None
        assert Symbol(1) is not None

    def test_repr(self):
        """ Tests the representation of symbols
        """
        symbol1 = Symbol("ABC")
        assert str(symbol1) == "ABC"
        symbol2 = Symbol(1)
        assert str(symbol2) == "1"

    def test_eq(self):
        """ Tests equality of symbols
        """
        symbol1 = Symbol("ABC")
        symbol2 = Symbol(1)
        symbol3 = Symbol("ABC")
        assert symbol1 == symbol3
        assert symbol2 == 1
        assert symbol2 != symbol3
        assert symbol1 != symbol2

    def test_hash(self):
        """ Tests the hashing of symbols
        """
        symbol1 = hash(Symbol("ABC"))
        symbol2 = hash(Symbol(1))
        symbol3 = hash(Symbol("ABC"))
        assert isinstance(symbol1, int)
        assert symbol1 == symbol3
        assert symbol2 != symbol3
        assert symbol1 != symbol2
