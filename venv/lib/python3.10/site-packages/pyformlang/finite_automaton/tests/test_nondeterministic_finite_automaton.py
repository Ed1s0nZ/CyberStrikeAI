"""
Tests for nondeterministic finite automata
"""
from pyformlang.finite_automaton import NondeterministicFiniteAutomaton,\
    Epsilon
from pyformlang.finite_automaton import State
from pyformlang.finite_automaton import Symbol
from pyformlang.finite_automaton.transition_function import \
    InvalidEpsilonTransition
import pytest


class TestNondeterministicFiniteAutomaton:
    """
    Tests for nondeterministic finite automata
    """

    # pylint: disable=missing-function-docstring, protected-access

    def test_creation(self):
        """ Test the creation of nfa
        """
        nfa = NondeterministicFiniteAutomaton()
        assert nfa is not None
        states = [State(x) for x in range(10)]
        nfa = NondeterministicFiniteAutomaton(start_state=set(states))
        assert nfa is not None

    def test_remove_initial(self):
        """ Test the remove of initial state
        """
        nfa = NondeterministicFiniteAutomaton()
        state0 = State(0)
        state1 = State(1)
        symb_a = Symbol("a")
        nfa.add_transition(state0, symb_a, state1)
        nfa.add_start_state(state0)
        nfa.add_final_state(state1)
        assert nfa.is_deterministic()
        assert nfa.accepts([symb_a])
        assert nfa.remove_start_state(state1) == 0
        assert nfa.accepts([symb_a])
        assert nfa.remove_start_state(state0) == 1
        assert not nfa.accepts([symb_a])

    def test_accepts(self):
        """ Tests the acceptance of nfa
        """
        nfa = NondeterministicFiniteAutomaton()
        state0 = State(0)
        state1 = State(1)
        state2 = State(2)
        state3 = State(3)
        state4 = State(4)
        symb_a = Symbol("a")
        symb_b = Symbol("b")
        symb_c = Symbol("c")
        symb_d = Symbol("d")
        nfa.add_start_state(state0)
        nfa.add_final_state(state4)
        nfa.add_final_state(state3)
        nfa.add_transition(state0, symb_a, state1)
        nfa.add_transition(state1, symb_b, state1)
        nfa.add_transition(state1, symb_c, state2)
        nfa.add_transition(state1, symb_d, state3)
        nfa.add_transition(state1, symb_c, state4)
        nfa.add_transition(state1, symb_b, state4)
        assert not nfa.is_deterministic()
        assert nfa.accepts([symb_a, symb_b, symb_c])
        assert nfa.accepts([symb_a, symb_b, symb_b, symb_b, symb_c])
        assert nfa.accepts([symb_a, symb_b, symb_d])
        assert nfa.accepts([symb_a, symb_d])
        assert nfa.accepts([symb_a, symb_b, symb_b, symb_b, symb_b])
        assert not nfa.accepts([symb_a, symb_c, symb_d])
        assert not nfa.accepts([symb_d, symb_c, symb_d])
        assert not nfa.accepts([])
        assert not nfa.accepts([symb_c])
        nfa.add_start_state(state1)
        assert not nfa.is_deterministic()
        assert nfa.accepts([symb_c])
        nfa.remove_start_state(state1)
        dfa = nfa.to_deterministic()
        assert dfa.is_deterministic()
        assert dfa.accepts([symb_a, symb_b, symb_c])
        assert dfa.accepts([symb_a, symb_b, symb_b, symb_b, symb_c])
        assert dfa.accepts([symb_a, symb_b, symb_d])
        assert dfa.accepts([symb_a, symb_d])
        assert dfa.accepts([symb_a, symb_b, symb_b, symb_b, symb_b])
        assert not dfa.accepts([symb_a, symb_c, symb_d])
        assert not dfa.accepts([symb_d, symb_c, symb_d])
        assert not dfa.accepts([])
        assert not dfa.accepts([symb_c])

    def test_deterministic(self):
        """ Tests the deterministic transformation """
        nfa = NondeterministicFiniteAutomaton()
        state0 = State("q0")
        state1 = State("q1")
        state2 = State("q2")
        symb0 = Symbol(0)
        symb1 = Symbol(1)
        nfa.add_start_state(state0)
        nfa.add_final_state(state1)
        nfa.add_transition(state0, symb0, state0)
        nfa.add_transition(state0, symb0, state1)
        nfa.add_transition(state0, symb1, state0)
        nfa.add_transition(state1, symb1, state2)
        dfa = nfa.to_deterministic()
        assert len(dfa.states) == 3
        assert dfa.get_number_transitions() == 6

    def test_epsilon_refused(self):
        dfa = NondeterministicFiniteAutomaton()
        state0 = State(0)
        state1 = State(1)
        with pytest.raises(InvalidEpsilonTransition):
            dfa.add_transition(state0, Epsilon(), state1)

    def test_word_generation(self):
        nfa = get_nfa_example_for_word_generation()
        accepted_words = list(nfa.get_accepted_words())
        assert [] in accepted_words
        assert [Symbol("a"), Symbol("b")] in accepted_words
        assert [Symbol("a"), Symbol("c")] in accepted_words
        assert [Symbol("d"), Symbol("e")] in accepted_words
        assert [Symbol("d"), Symbol("e"), Symbol("f")] in accepted_words
        assert len(accepted_words) == 5

    def test_for_duplicate_generation(self):
        nfa = get_nfa_example_with_duplicates()
        accepted_words = list(nfa.get_accepted_words())
        assert [Symbol("a"), Symbol("c")] in accepted_words
        assert [Symbol("b"), Symbol("c")] in accepted_words
        assert len(accepted_words) == 2

    def test_cyclic_word_generation(self):
        nfa = get_cyclic_nfa_example()
        accepted_words = list(nfa.get_accepted_words(5))
        assert ["a", "d", "g"] in accepted_words
        assert ["a", "b", "c", "d", "g"] in accepted_words
        assert ["a", "d", "e", "f", "g"] in accepted_words
        assert ["b", "f", "g"] in accepted_words
        assert ["b", "f", "e", "f", "g"] in accepted_words
        assert len(accepted_words) == 5

    def test_final_state_at_start_generation(self):
        nfa = get_nfa_example_with_final_state_at_start()
        accepted_words = list(nfa.get_accepted_words())
        assert accepted_words == [[]]

    def test_start_state_at_the_end_generation(self):
        nfa = get_nfa_example_with_start_state_at_the_end()
        accepted_words = list(nfa.get_accepted_words(5))
        assert [] in accepted_words
        assert ["a", "b", "c"] in accepted_words
        assert ["a", "b", "e", "b", "c"] in accepted_words
        assert ["d", "b", "c"] in accepted_words
        assert ["d", "b", "e", "b", "c"] in accepted_words
        assert len(accepted_words) == 5


def get_nfa_example_for_word_generation():
    """
    Gets Nondeterministic Finite Automaton \
    example for the word generation test.
    """
    nfa = NondeterministicFiniteAutomaton(start_state={0, 4},
                                          final_states={3, 4, 6, 8})
    nfa.add_transitions([
        (0, "a", 1),
        (0, "a", 2),
        (1, "a", 1),
        (2, "b", 3),
        (2, "c", 3),
        (4, "d", 5),
        (5, "e", 6),
        (5, "e", 7),
        (7, "f", 8),
    ])
    return nfa


def get_nfa_example_with_duplicates():
    """ Gets NFA example with duplicate word chains """
    nfa = NondeterministicFiniteAutomaton(start_state={0, 1, 5, 6},
                                          final_states={3, 4, 8})
    nfa.add_transitions([
        (0, "a", 2),
        (1, "a", 2),
        (2, "c", 3),
        (2, "c", 4),
        (5, "a", 7),
        (6, "b", 7),
        (7, "c", 8),
    ])
    return nfa


def get_cyclic_nfa_example():
    """ Gets NFA example with several cycles on path to final """
    nfa = NondeterministicFiniteAutomaton(start_state={0, 5},
                                          final_states={4})
    nfa.add_transitions([
        (0, "a", 1),
        (1, "b", 2),
        (2, "c", 1),
        (1, "d", 3),
        (3, "e", 6),
        (6, "f", 3),
        (3, "g", 4),
        (5, "b", 6),
    ])
    return nfa


def get_nfa_example_with_final_state_at_start():
    """ Gets NFA example with final state at start """
    nfa = NondeterministicFiniteAutomaton(start_state={0, 5},
                                          final_states={0})
    nfa.add_transitions([
        (0, "a", 1),
        (1, "b", 2),
        (2, "c", 3),
        (2, "d", 4),
        (5, "e", 1),
        (5, "e", 2),
    ])
    return nfa


def get_nfa_example_with_start_state_at_the_end():
    """ Gets NFA example with start state at the end """
    nfa = NondeterministicFiniteAutomaton(start_state={0, 3, 4},
                                          final_states={3})
    nfa.add_transitions([
        (0, "a", 1),
        (1, "b", 2),
        (2, "e", 1),
        (2, "c", 3),
        (4, "d", 1),
    ])
    return nfa
