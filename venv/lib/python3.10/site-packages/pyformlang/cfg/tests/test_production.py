""" Tests the productions """

from pyformlang.cfg import Production, Variable, Terminal


class TestProduction:
    """ Tests the production """
    # pylint: disable=missing-function-docstring

    def test_creation(self):
        prod0 = Production(Variable("S0"), [Terminal("S1"), Variable("a")])
        prod1 = Production(Variable("S0"), [Terminal("S1"), Variable("a")])
        prod2 = Production(Variable("S0'"), [Terminal("S1"), Variable("a")])
        prod3 = Production(Variable("S0"), [Terminal("S2"), Variable("a")])
        prod4 = Production(Variable("S0"), [Terminal("S2"), Variable("b")])
        assert prod0 == prod1
        assert prod0 != prod2
        assert prod0 != prod3
        assert prod0 != prod4
        assert str(prod0) == str(prod1)
        assert str(prod0) != str(prod2)
        assert str(prod0) != str(prod3)
        assert str(prod0) != str(prod4)
        assert hash(prod0) == hash(prod1)
        assert hash(prod0) != hash(prod2)
        assert hash(prod0) != hash(prod3)
        assert hash(prod0) != hash(prod4)
        assert " -> " in str(prod0)
