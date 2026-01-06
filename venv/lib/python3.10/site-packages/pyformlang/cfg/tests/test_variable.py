""" Tests the variable """
from pyformlang.cfg import Variable


class TestVariable:
    """ Tests the variable """
    # pylint: disable=missing-function-docstring

    def test_creation(self):
        variable0 = Variable(0)
        variable1 = Variable(1)
        variable2 = Variable(0)
        variable3 = Variable("0")
        assert variable0 == variable2
        assert variable0 != variable1
        assert variable0 != variable3
        assert hash(variable0) == hash(variable2)
        assert hash(variable0) != hash(variable1)
        assert str(variable0) == str(variable2)
        assert str(variable0) == str(variable3)
        assert str(variable0) != str(variable1)
