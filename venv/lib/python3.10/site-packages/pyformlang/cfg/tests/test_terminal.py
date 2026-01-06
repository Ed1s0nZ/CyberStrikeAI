""" Tests the terminal """
from pyformlang.cfg import Terminal, Epsilon


class TestTerminal:
    """ Tests the terminal """
    # pylint: disable=missing-function-docstring

    def test_creation(self):
        terminal0 = Terminal(0)
        terminal1 = Terminal(1)
        terminal2 = Terminal(0)
        terminal3 = Terminal("0")
        assert terminal0 == terminal2
        assert terminal0 != terminal1
        assert terminal0 != terminal3
        assert hash(terminal0) == hash(terminal2)
        assert hash(terminal0) != hash(terminal1)
        assert str(terminal0) == str(terminal2)
        assert str(terminal0) == str(terminal3)
        assert str(terminal0) != str(terminal1)
        epsilon = Epsilon()
        assert epsilon.to_text() == "epsilon"
        assert Terminal("C").to_text() == '"TER:C"'
