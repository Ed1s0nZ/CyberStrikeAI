"""
Tests the states
"""
from pyformlang.finite_automaton import State


class TestState:
    """ Test the states
    """

    def test_can_create(self):
        """ Tests the creation of states
        """
        assert State("") is not None
        assert State(1) is not None

    def test_repr(self):
        """ Tests the representation of states
        """
        state1 = State("ABC")
        assert str(state1) == "ABC"
        state2 = State(1)
        assert str(state2) == "1"

    def test_eq(self):
        """ Tests the equality of states
        """
        state1 = State("ABC")
        state2 = State(1)
        state3 = State("ABC")
        assert state1 == state3
        assert state2 == 1
        assert state2 != state3
        assert state2 == 1
        assert state1 != state2

    def test_hash(self):
        """ Tests the hashing of states
        """
        state1 = hash(State("ABC"))
        state2 = hash(State(1))
        state3 = hash(State("ABC"))
        assert isinstance(state1, int)
        assert state1 == state3
        assert state2 != state3
        assert state1 != state2
