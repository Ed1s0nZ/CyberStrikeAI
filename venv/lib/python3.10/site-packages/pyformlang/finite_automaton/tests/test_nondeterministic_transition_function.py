"""
Test the nondeterministic transition functions
"""
from pyformlang.finite_automaton import State, Symbol, \
    NondeterministicTransitionFunction, Epsilon


class TestNondeterministicTransitionFunction:
    """ Tests the nondeterministic transitions functions
    """

    def test_creation(self):
        """ Tests the creation of nondeterministic transition functions
        """
        transition_function = NondeterministicTransitionFunction()
        assert transition_function is not None

    def test_add_transitions(self):
        """ Tests the addition of transitions
        """
        transition_function = NondeterministicTransitionFunction()
        s_from = State(0)
        s_to = State(1)
        s_to_bis = State(2)
        symb_by = Symbol("a")
        transition_function.add_transition(s_from, symb_by, s_to)
        transition_function.add_transition(s_from, symb_by, s_to)
        transition_function.add_transition(s_from, symb_by, s_to_bis)
        transition_function.add_transition(s_to, symb_by, s_to)
        assert transition_function.get_number_transitions() == 3

    def test_number_transitions(self):
        """ Tests the number of transitions
        """
        transition_function = NondeterministicTransitionFunction()
        assert transition_function.get_number_transitions() == 0
        s_from = State(0)
        s_to = State(1)
        s_to_bis = State(2)
        symb_by = Symbol("a")
        symb_by2 = Symbol("b")
        transition_function.add_transition(s_from, symb_by, s_to)
        assert transition_function.get_number_transitions() == 1
        transition_function.add_transition(s_from, symb_by, s_to)
        assert transition_function.get_number_transitions() == 1
        transition_function.add_transition(s_from, symb_by2, s_to_bis)
        assert transition_function.get_number_transitions() == 2
        transition_function.add_transition(s_to, symb_by, s_to_bis)
        assert transition_function.get_number_transitions() == 3
        transition_function.add_transition(s_from, symb_by, s_from)
        assert transition_function.get_number_transitions() == 4

    def test_remove_transitions(self):
        """ Tests the removal of transitions
        """
        transition_function = NondeterministicTransitionFunction()
        s_from = State(0)
        s_to = State(1)
        symb_by = Symbol("a")
        transition_function.add_transition(s_from, symb_by, s_to)
        assert transition_function.remove_transition(s_from,
                                                               symb_by,
                                                               s_to) == 1
        assert len(transition_function(s_to, symb_by)) == 0
        assert transition_function.get_number_transitions() == 0
        assert len(transition_function(s_from, symb_by)) == 0
        assert transition_function.remove_transition(s_from,
                                                               symb_by,
                                                               s_to) == 0
        transition_function.add_transition(s_from, symb_by, s_to)
        transition_function.add_transition(s_from, symb_by, s_from)
        assert transition_function.remove_transition(s_from,
                                                               symb_by,
                                                               s_to) == 1
        assert transition_function.get_number_transitions() == 1
        assert len(transition_function(s_from, symb_by)) == 1

    def test_call(self):
        """ Tests the call of a transition function
        """
        transition_function = NondeterministicTransitionFunction()
        s_from = State(0)
        s_to = State(1)
        symb_by = Symbol("a")
        transition_function.add_transition(s_from, symb_by, s_to)
        assert transition_function(s_from, symb_by) == {s_to}
        assert len(transition_function(s_to, symb_by)) == 0
        transition_function.add_transition(s_from, symb_by, s_from)
        assert transition_function(s_from, symb_by) == {s_to, s_from}

    def test_get_transitions_from(self):
        """ Tests iteration of transitions from specified state """
        transition_function = NondeterministicTransitionFunction()
        states = [State(x) for x in range(0, 5)]
        symbol_a = Symbol("a")
        symbol_b = Symbol("b")
        symbol_c = Symbol("c")
        epsilon = Epsilon()
        transition_function.add_transition(states[0], symbol_a, states[1])
        transition_function.add_transition(states[1], symbol_b, states[2])
        transition_function.add_transition(states[1], symbol_c, states[2])
        transition_function.add_transition(states[1], symbol_c, states[3])
        transition_function.add_transition(states[1], epsilon, states[4])
        transitions = list(transition_function.get_transitions_from(states[1]))
        assert (symbol_b, states[2]) in transitions
        assert (symbol_c, states[2]) in transitions
        assert (symbol_c, states[3]) in transitions
        assert (epsilon, states[4]) in transitions
        assert len(transitions) == 4
