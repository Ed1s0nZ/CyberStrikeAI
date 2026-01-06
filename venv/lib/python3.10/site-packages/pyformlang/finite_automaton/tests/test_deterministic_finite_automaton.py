"""
Tests for the deterministic finite automata
"""
from pyformlang.finite_automaton import DeterministicFiniteAutomaton, Epsilon
from pyformlang.finite_automaton import State
from pyformlang.finite_automaton import Symbol
from pyformlang.finite_automaton import TransitionFunction
from pyformlang.finite_automaton.transition_function import \
    InvalidEpsilonTransition
import pytest


class TestDeterministicFiniteAutomaton:
    """ Tests for deterministic finite automata
    """

    # pylint: disable=missing-function-docstring, protected-access

    def test_can_create(self):
        """ Test the creation of dfa
        """
        state0 = State("0")
        state1 = State("1")
        symb0 = Symbol("a")
        states = {state0, state1}
        input_symbols = {symb0}
        transition_function = TransitionFunction()
        transition_function.add_transition(state0, symb0, state1)
        start_state = state0
        final_states = {state1}
        dfa = DeterministicFiniteAutomaton(states,
                                           input_symbols,
                                           transition_function,
                                           start_state,
                                           final_states)
        assert len(dfa.to_dict()) == 1
        assert len(dfa) == 1
        assert dfa is not None
        dfa = DeterministicFiniteAutomaton()
        assert dfa is not None
        dfa = DeterministicFiniteAutomaton(start_state=state1,
                                           final_states={state0, state1})
        assert dfa is not None
        assert dfa is dfa.to_deterministic()

    def test_add_transition(self):
        """ Tests the addition of transitions
        """
        dfa = DeterministicFiniteAutomaton()
        assert len(dfa.states) == 0
        state0 = State("0")
        state1 = State("1")
        symb = Symbol("a")
        dfa.add_transition(state0, symb, state1)
        assert len(dfa.states) == 2
        assert len(dfa.symbols) == 1
        assert len(list(dfa._transition_function.get_edges())) == 1

    def test_add_remove_start_final(self):
        """ Tests the addition and removal of initial state and final states
        """
        dfa = DeterministicFiniteAutomaton()
        state0 = State("0")
        state1 = State("1")
        assert dfa.add_start_state(state0) == 1
        assert len(dfa.states) == 1
        assert dfa.add_final_state(state1) == 1
        assert len(dfa.states) == 2
        assert dfa.remove_final_state(state0) == 0
        assert dfa.is_final_state(state1)
        assert not dfa.is_final_state(state0)
        assert dfa.remove_final_state(state1) == 1
        assert not dfa.is_final_state(state1)
        assert dfa.remove_final_state(state1) == 0

    def test_accepts(self):
        """ Tests the acceptance of dfa
        """
        dfa = get_example0()
        self._perform_tests_example0(dfa)
        dfa = get_example0_bis()
        self._perform_tests_example0(dfa)

    def _perform_tests_example0(self, dfa):
        """ Tests for DFA from example 0 """
        symb_a = Symbol("a")
        symb_b = Symbol("b")
        symb_c = Symbol("c")
        symb_d = Symbol("d")
        state0 = State(0)
        state1 = State(1)
        assert dfa.accepts([symb_a, symb_b, symb_c])
        assert dfa.accepts([symb_a, symb_b, symb_b, symb_b, symb_c])
        assert dfa.accepts([symb_a, symb_b, symb_d])
        assert dfa.accepts([symb_a, symb_d])
        assert not dfa.accepts([symb_a, symb_c, symb_d])
        assert not dfa.accepts([symb_d, symb_c, symb_d])
        assert not dfa.accepts([])
        assert dfa.remove_start_state(state1) == 0
        assert dfa.accepts([symb_a, symb_b, symb_c])
        assert dfa.remove_start_state(state0) == 1
        assert not dfa.accepts([symb_a, symb_b, symb_c])

        dfa.add_start_state(0)
        assert dfa.accepts(["a", "b", "c"])
        assert dfa.accepts(["a", "b", "b", "b", "c"])
        assert dfa.accepts(["a", "b", "d"])
        assert dfa.accepts(["a", "d"])
        assert not dfa.accepts(["a", "c", "d"])
        assert not dfa.accepts(["d", "c", "d"])
        assert not dfa.accepts([])
        assert dfa.remove_start_state(1) == 0
        assert dfa.accepts(["a", "b", "c"])
        assert dfa.remove_start_state(0) == 1
        assert not dfa.accepts(["a", "b", "c"])

    def test_copy(self):
        """ Test the copy of a DFA """
        dfa = get_example0().copy()
        self._perform_tests_example0(dfa)

    def test_regex(self):
        """ Tests the regex transformation """
        dfa = get_example0()
        dfa = dfa.to_regex().to_epsilon_nfa()
        self._perform_tests_example0(dfa)

    def test_complement(self):
        """ Tests the complement operation """
        dfa = DeterministicFiniteAutomaton()
        symb_a = Symbol("a")
        symb_b = Symbol("b")
        state0 = State(0)
        state1 = State(1)
        state2 = State(2)
        dfa.add_start_state(state0)
        dfa.add_final_state(state2)
        dfa.add_transition(state0, symb_a, state1)
        dfa.add_transition(state1, symb_b, state2)
        dfa_comp = dfa.get_complement()
        assert dfa_comp.accepts([symb_a])
        assert dfa_comp.accepts([symb_b])
        assert dfa_comp.accepts([symb_b, symb_a])
        assert dfa_comp.accepts([])
        assert not dfa_comp.accepts([symb_a, symb_b])

    def test_big_minimize(self):
        dfa = DeterministicFiniteAutomaton()
        size = 1000
        symb = Symbol("s")
        dfa.add_start_state(State(0))
        dfa.add_final_state(State(size))
        for i in range(size):
            dfa.add_transition(State(i), symb, State(i+1))
        dfa = dfa.minimize()
        assert len(dfa.states) == size + 1
        assert not dfa.accepts([symb])

    def test_big_minimize_reduce(self):
        dfa = DeterministicFiniteAutomaton()
        symb_0 = Symbol("0")
        symb_0_minus = Symbol("0-")
        symb_1_minus = Symbol("1-")
        symb_star = Symbol("STAR")
        start = State("start")
        states = [State(str(x)) for x in range(10)]
        dfa.add_start_state(start)
        dfa.add_final_state(states[2])
        dfa.add_final_state(states[3])
        dfa.add_final_state(states[4])
        dfa.add_final_state(states[8])
        dfa.add_final_state(states[7])
        dfa.add_final_state(states[9])
        dfa.add_transition(start, symb_0, states[0])
        dfa.add_transition(states[1], symb_0, states[2])
        dfa.add_transition(states[2], symb_0, states[0])
        dfa.add_transition(states[3], symb_0, states[0])
        dfa.add_transition(states[4], symb_0, states[0])
        dfa.add_transition(states[8], symb_0, states[0])
        dfa.add_transition(states[7], symb_0, states[0])
        dfa.add_transition(states[9], symb_0, states[0])
        dfa.add_transition(states[0], symb_star, states[1])
        dfa.add_transition(states[5], symb_star, states[6])
        dfa.add_transition(states[7], symb_star, states[9])
        dfa.add_transition(states[2], symb_star, states[3])
        dfa.add_transition(states[4], symb_star, states[8])
        dfa.add_transition(states[1], symb_0_minus, states[5])
        dfa.add_transition(states[6], symb_0_minus, states[7])
        dfa.add_transition(states[3], symb_1_minus, states[4])
        assert not dfa.accepts(["0", "STAR", "0-", "STAR", "0-", "0",
                                      "STAR", "0", "0", "STAR", "0-", "STAR",
                                      "0-", "1-"])
        dfa = dfa.minimize()
        assert not dfa.accepts(["0", "STAR", "0-", "STAR", "0-", "0",
                                      "STAR", "0", "0", "STAR", "0-", "STAR",
                                      "0-", "1-"])

    def test_minimize_repetition(self):
        dfa = DeterministicFiniteAutomaton()
        symb_a = Symbol('a')
        symb_b = Symbol("b")
        symb_star = Symbol("star")
        states = [State(x) for x in range(9)]
        dfa.add_start_state(states[0])
        dfa.add_final_state(states[3])
        dfa.add_final_state(states[4])
        dfa.add_final_state(states[7])
        dfa.add_final_state(states[8])
        dfa.add_transition(states[0], symb_a, states[1])
        dfa.add_transition(states[1], symb_star, states[2])
        dfa.add_transition(states[2], symb_a, states[3])
        dfa.add_transition(states[3], symb_star, states[4])
        dfa.add_transition(states[3], symb_a, states[1])
        dfa.add_transition(states[3], symb_b, states[5])
        dfa.add_transition(states[4], symb_a, states[1])
        dfa.add_transition(states[4], symb_b, states[5])

        dfa.add_transition(states[0], symb_b, states[5])
        dfa.add_transition(states[5], symb_star, states[6])
        dfa.add_transition(states[6], symb_b, states[7])
        dfa.add_transition(states[7], symb_star, states[8])
        dfa.add_transition(states[7], symb_a, states[1])
        dfa.add_transition(states[7], symb_b, states[5])
        dfa.add_transition(states[8], symb_a, states[1])
        dfa.add_transition(states[8], symb_b, states[5])
        dfa = dfa.minimize()
        assert dfa.accepts([symb_a, symb_star, symb_a])

    def test_not_cyclic(self):
        dfa = DeterministicFiniteAutomaton()
        state0 = State(0)
        state1 = State(1)
        symb_a = Symbol('a')
        dfa.add_start_state(state0)
        dfa.add_transition(state0, symb_a, state1)
        assert dfa.is_acyclic()

    def test_not_cyclic2(self):
        dfa = DeterministicFiniteAutomaton()
        state0 = State(0)
        state1 = State(1)
        symb_a = Symbol('a')
        symb_b = Symbol('b')
        dfa.add_start_state(state0)
        dfa.add_transition(state0, symb_a, state1)
        dfa.add_transition(state0, symb_b, state1)
        assert dfa.is_acyclic()

    def test_epsilon_refused(self):
        dfa = DeterministicFiniteAutomaton()
        state0 = State(0)
        state1 = State(1)
        with pytest.raises(InvalidEpsilonTransition):
            dfa.add_transition(state0, Epsilon(), state1)

    def test_cyclic(self):
        dfa = DeterministicFiniteAutomaton()
        state0 = State(0)
        state1 = State(1)
        symb_a = Symbol('a')
        dfa.add_start_state(state0)
        dfa.add_transition(state0, symb_a, state1)
        dfa.add_transition(state1, symb_a, state0)
        assert not dfa.is_acyclic()

    def test_equivalent(self):
        dfa1 = get_dfa_example()
        dfa2 = DeterministicFiniteAutomaton()
        dfa2.add_transitions(
            [("A", "c", "B"),
             ("C", "d", "D"),
             ("B", "S", "C"),
             ("B", "c", "D")])
        dfa2.add_start_state(State("A"))
        dfa2.add_final_state(State("D"))
        assert dfa2 != dfa1

    def test_regex_dfa(self):
        dfa1 = get_dfa_example()
        dfa_regex = dfa1.to_regex().to_epsilon_nfa()
        assert dfa1 == dfa_regex

    def test_word_generation(self):
        dfa = get_dfa_example_for_word_generation()
        accepted_words = list(dfa.get_accepted_words())
        assert [] in accepted_words
        assert [Symbol("b"), Symbol("c")] in accepted_words
        assert [Symbol("b"), Symbol("d")] in accepted_words
        assert len(accepted_words) == 3

    def test_cyclic_word_generation(self):
        dfa = get_cyclic_dfa_example()
        accepted_words = list(dfa.get_accepted_words(5))
        assert ["a", "f"] in accepted_words
        assert ["a", "b", "e", "f"] in accepted_words
        assert ["a", "b", "c", "e", "f"] in accepted_words
        assert ["a", "b", "d", "a", "f"] in accepted_words
        assert len(accepted_words) == 4

    def test_dfa_generating_no_words(self):
        dfa = get_dfa_example_without_accepted_words()
        accepted_words = list(dfa.get_accepted_words())
        assert not accepted_words


def get_example0():
    """ Gives a dfa """
    dfa = DeterministicFiniteAutomaton()
    state0 = State(0)
    state1 = State(1)
    state2 = State(2)
    state3 = State(3)
    symb_a = Symbol("a")
    symb_b = Symbol("b")
    symb_c = Symbol("c")
    symb_d = Symbol("d")
    dfa.add_start_state(state0)
    dfa.add_final_state(state2)
    dfa.add_final_state(state3)
    dfa.add_transition(state0, symb_a, state1)
    dfa.add_transition(state1, symb_b, state1)
    dfa.add_transition(state1, symb_c, state2)
    dfa.add_transition(state1, symb_d, state3)
    return dfa


def get_example0_bis():
    """ Gives a dfa """
    dfa = DeterministicFiniteAutomaton()
    dfa.add_start_state(0)
    dfa.add_final_state(2)
    dfa.add_final_state(3)
    dfa.add_transition(0, "a", 1)
    dfa.add_transition(1, "b", 1)
    dfa.add_transition(1, "c", 2)
    dfa.add_transition(1, "d", 3)
    return dfa


def get_dfa_example():
    """ An example of DFA """
    dfa1 = DeterministicFiniteAutomaton()
    dfa1.add_transitions(
        [("A", "c", "B"),
         ("C", "d", "D"),
         ("B", "S", "C"),
         ("B", "d", "D")])
    dfa1.add_start_state(State("A"))
    dfa1.add_final_state(State("D"))
    return dfa1


def get_dfa_example_for_word_generation():
    """ DFA example for the word generation test """
    dfa = DeterministicFiniteAutomaton()
    states = [State(x) for x in range(4)]
    symbol_a = Symbol("a")
    symbol_b = Symbol("b")
    symbol_c = Symbol("c")
    symbol_d = Symbol("d")
    dfa.add_transitions([
        (states[0], symbol_a, states[1]),
        (states[0], symbol_b, states[2]),
        (states[1], symbol_a, states[1]),
        (states[2], symbol_c, states[3]),
        (states[2], symbol_d, states[3]),
    ])
    dfa.add_start_state(states[0])
    dfa.add_final_state(states[0])
    dfa.add_final_state(states[3])
    return dfa


def get_cyclic_dfa_example():
    """ Gets DFA example with several cycles on path to final """
    dfa = DeterministicFiniteAutomaton(start_state=0,
                                       final_states={3})
    dfa.add_transitions([
        (0, "a", 1),
        (1, "b", 2),
        (2, "c", 2),
        (2, "d", 0),
        (2, "e", 1),
        (1, "f", 3),
    ])
    return dfa


def get_dfa_example_without_accepted_words():
    """ DFA example accepting no words """
    dfa = DeterministicFiniteAutomaton()
    states = [State(x) for x in range(4)]
    symbol_a = Symbol("a")
    symbol_b = Symbol("b")
    dfa.add_transitions([
        (states[0], symbol_a, states[1]),
        (states[2], symbol_b, states[3]),
    ])
    dfa.add_start_state(states[0])
    dfa.add_final_state(states[3])
    return dfa
