"""
Tests for epsilon NFA
"""
import copy

import networkx

from pyformlang.finite_automaton import EpsilonNFA, State, Symbol, Epsilon
from ..regexable import Regexable
import pytest


class TestEpsilonNFA:
    """ Tests epsilon NFA """

    # pylint: disable=missing-function-docstring, protected-access
    # pylint: disable=too-many-statements, too-many-public-methods

    def test_eclose(self):
        """ Test of the epsilon closure """
        states = [State(x) for x in range(8)]
        epsilon = Epsilon()
        symb_a = Symbol("a")
        symb_b = Symbol("b")
        enfa = EpsilonNFA()
        enfa.add_transition(states[1], epsilon, states[2])
        enfa.add_transition(states[1], epsilon, states[4])
        enfa.add_transition(states[2], epsilon, states[3])
        enfa.add_transition(states[3], epsilon, states[6])
        enfa.add_transition(states[5], epsilon, states[7])
        enfa.add_transition(states[4], symb_a, states[5])
        enfa.add_transition(states[5], symb_b, states[6])
        assert len(enfa.eclose(states[1])) == 5
        assert len(enfa.eclose(states[2])) == 3
        assert len(enfa.eclose(states[5])) == 2
        assert len(enfa.eclose(states[6])) == 1
        assert len(list(enfa._transition_function.get_edges())) == 7
        assert enfa.remove_transition(states[1], epsilon, states[4]) == \
                         1
        assert not enfa.is_deterministic()

    def test_accept(self):
        """ Test the acceptance """
        self._perform_tests_digits(False)

    def test_copy(self):
        """ Tests the copy of enda """
        self._perform_tests_digits(True)

    def _perform_tests_digits(self, should_copy=False):
        enfa, digits, epsilon, plus, minus, point = get_digits_enfa()
        if should_copy:
            enfa = copy.copy(enfa)
        assert enfa.accepts([plus, digits[1], point, digits[9]])
        assert enfa.accepts([minus, digits[1], point, digits[9]])
        assert enfa.accepts([digits[1], point, digits[9]])
        assert enfa.accepts([digits[1], point])
        assert enfa.accepts([digits[1], point, epsilon])
        assert enfa.accepts([point, digits[9]])
        assert not enfa.accepts([point])
        assert not enfa.accepts([plus])
        assert not enfa.is_deterministic()

        assert enfa.accepts(["+", digits[1], ".", digits[9]])
        assert enfa.accepts(["-", digits[1], ".", digits[9]])
        assert enfa.accepts([digits[1], ".", digits[9]])
        assert enfa.accepts([digits[1], "."])
        assert enfa.accepts([digits[1], ".", "epsilon"])
        assert enfa.accepts([".", digits[9]])
        assert not enfa.accepts(["."])
        assert not enfa.accepts(["+"])

    def test_deterministic(self):
        """ Tests the transformation to a dfa"""
        enfa, digits, _, plus, minus, point = get_digits_enfa()
        dfa = enfa.to_deterministic()
        assert dfa.is_deterministic()
        assert len(dfa.states) == 6
        assert dfa.get_number_transitions() == 65
        assert len(dfa.final_states) == 2
        assert dfa.accepts([plus, digits[1], point, digits[9]])
        assert dfa.accepts([minus, digits[1], point, digits[9]])
        assert dfa.accepts([digits[1], point, digits[9]])
        assert dfa.accepts([digits[1], point])
        assert dfa.accepts([digits[1], point])
        assert dfa.accepts([point, digits[9]])
        assert not dfa.accepts([point])
        assert not dfa.accepts([plus])

    def test_remove_state(self):
        " Tests the remove of state """
        enfa = EpsilonNFA()
        state0 = State(0)
        state1 = State(1)
        state2 = State(2)
        symb02 = Symbol("a+b")
        symb01 = Symbol("c*")
        symb11 = Symbol("b+(c.d)")
        symb12 = Symbol("a.b.c")
        enfa.add_start_state(state0)
        enfa.add_final_state(state2)
        enfa.add_transition(state0, symb01, state1)
        enfa.add_transition(state0, symb02, state2)
        enfa.add_transition(state1, symb11, state1)
        enfa.add_transition(state1, symb12, state2)
        enfa._remove_all_basic_states()
        assert enfa.get_number_transitions() == 1
        assert len(enfa.states) == 2

    def test_to_regex(self):
        """ Tests the transformation to regex """
        enfa = EpsilonNFA()
        state0 = State(0)
        state1 = State(1)
        state2 = State(2)
        symb_e = Symbol("e")
        symb_f = Symbol("f")
        symb_g = Symbol("g")
        enfa.add_start_state(state0)
        enfa.add_final_state(state2)
        enfa.add_transition(state0, symb_e, state1)
        enfa.add_transition(state1, symb_f, state2)
        enfa.add_transition(state0, symb_g, state2)
        regex = enfa.to_regex()
        enfa2 = regex.to_epsilon_nfa()
        assert enfa2.accepts([symb_e, symb_f])
        assert enfa2.accepts([symb_g])
        assert not enfa2.accepts([])
        assert not enfa2.accepts([symb_e])
        assert not enfa2.accepts([symb_f])
        enfa.add_final_state(state0)
        with pytest.raises(ValueError) as _:
            enfa._get_regex_simple()
        regex = enfa.to_regex()
        enfa3 = regex.to_epsilon_nfa()
        assert enfa3.accepts([symb_e, symb_f])
        assert enfa3.accepts([symb_g])
        assert enfa3.accepts([])
        assert not enfa3.accepts([symb_e])
        assert not enfa3.accepts([symb_f])
        enfa.remove_start_state(state0)
        regex = enfa.to_regex()
        enfa3 = regex.to_epsilon_nfa()
        assert not enfa3.accepts([symb_e, symb_f])
        assert not enfa3.accepts([symb_g])
        assert not enfa3.accepts([])
        assert not enfa3.accepts([symb_e])
        assert not enfa3.accepts([symb_f])
        enfa.add_start_state(state0)
        enfa.add_transition(state0, symb_f, state0)
        regex = enfa.to_regex()
        enfa3 = regex.to_epsilon_nfa()
        assert enfa3.accepts([symb_e, symb_f])
        assert enfa3.accepts([symb_f, symb_e, symb_f])
        assert enfa3.accepts([symb_g])
        assert enfa3.accepts([symb_f, symb_f, symb_g])
        assert enfa3.accepts([])
        assert not enfa3.accepts([symb_e])
        assert enfa3.accepts([symb_f])

    def test_to_regex2(self):
        """ Tests the transformation to regex """
        enfa = EpsilonNFA()
        state0 = State(0)
        state1 = State(1)
        symb_a = Symbol("0")
        symb_b = Symbol("1")
        enfa.add_start_state(state0)
        enfa.add_final_state(state1)
        enfa.add_transition(state0, symb_a, state0)
        enfa.add_transition(state0, symb_a, state1)
        enfa.add_transition(state1, symb_b, state0)
        enfa.add_transition(state1, symb_b, state1)
        regex = enfa.to_regex()
        enfa2 = regex.to_epsilon_nfa()
        assert enfa2.accepts([symb_a])
        assert enfa2.accepts([symb_a, symb_a])
        assert enfa2.accepts([symb_a, symb_a, symb_b])
        assert enfa2.accepts([symb_a, symb_a, symb_b, symb_b])
        assert enfa2.accepts([symb_a, symb_a,
                                       symb_b, symb_b, symb_a])
        assert enfa2.accepts([symb_a, symb_a, symb_b,
                                       symb_b, symb_a, symb_b])
        assert not enfa2.accepts([symb_b])

    def test_to_regex3(self):
        """ Tests the transformation to regex """
        enfa = EpsilonNFA()
        state0 = State(0)
        state1 = State(1)
        symb_a = Symbol("0")
        symb_b = Symbol("1")
        enfa.add_start_state(state0)
        enfa.add_final_state(state1)
        enfa.add_transition(state0, symb_a, state0)
        enfa.add_transition(state1, symb_b, state0)
        enfa.add_transition(state1, symb_b, state1)
        regex = enfa.to_regex()
        enfa2 = regex.to_epsilon_nfa()
        assert not enfa2.accepts([symb_a])
        assert not enfa2.accepts([symb_a, symb_a])
        assert not enfa2.accepts([symb_a, symb_a, symb_b])
        assert not enfa2.accepts([symb_a, symb_a,
                                        symb_b, symb_b, symb_a])
        assert not enfa2.accepts([symb_a, symb_a, symb_b,
                                        symb_b, symb_a, symb_b])
        assert not enfa2.accepts([symb_b])
        epsilon = Epsilon()
        enfa.add_transition(state0, epsilon, state1)
        regex = enfa.to_regex()
        enfa2 = regex.to_epsilon_nfa()
        assert enfa.accepts([])
        assert enfa.accepts([symb_a])
        assert enfa2.accepts([symb_a])
        assert enfa2.accepts([symb_a, symb_a])
        assert enfa2.accepts([symb_a, symb_a, symb_b, symb_b])
        assert enfa2.accepts([symb_a, symb_a, symb_b, symb_b,
                                       symb_a, symb_b])
        assert enfa2.accepts([symb_b])
        assert enfa2.accepts([])
        enfa.remove_transition(state0, symb_a, state0)
        regex = enfa.to_regex()
        enfa2 = regex.to_epsilon_nfa()
        assert not enfa2.accepts([symb_a])
        assert not enfa2.accepts([symb_a, symb_a])
        assert not enfa2.accepts([symb_a, symb_a, symb_b])
        assert not enfa2.accepts([symb_a, symb_a, symb_b,
                                        symb_b, symb_a])
        assert not enfa2.accepts([symb_a, symb_a, symb_b, symb_b,
                                        symb_a, symb_b])
        assert enfa2.accepts([symb_b])
        assert enfa2.accepts([])
        enfa.remove_transition(state1, symb_b, state1)
        regex = enfa.to_regex()
        enfa2 = regex.to_epsilon_nfa()
        assert enfa2.accepts([symb_b, symb_b])
        enfa.add_transition(state0, symb_a, state0)
        regex = enfa.to_regex()
        enfa2 = regex.to_epsilon_nfa()
        assert enfa2.accepts([symb_a, symb_b])

    def test_union(self):
        """ Tests the union of two epsilon NFA """
        with pytest.raises(NotImplementedError) as _:
            Regexable().to_regex()
        enfa0 = get_enfa_example0()
        enfa1 = get_enfa_example1()
        symb_a = Symbol("a")
        symb_b = Symbol("b")
        symb_c = Symbol("c")
        enfa = enfa0.union(enfa1)
        assert enfa.accepts([symb_b])
        assert enfa.accepts([symb_a, symb_b])
        assert enfa.accepts([symb_c])
        assert not enfa.accepts([symb_a])
        assert not enfa.accepts([])

    def test_concatenate(self):
        """ Tests the concatenation of two epsilon NFA """
        enfa0 = get_enfa_example0()
        enfa1 = get_enfa_example1()
        symb_a = Symbol("a")
        symb_b = Symbol("b")
        symb_c = Symbol("c")
        enfa = enfa0.concatenate(enfa1)
        assert enfa.accepts([symb_b, symb_c])
        assert enfa.accepts([symb_a, symb_b, symb_c])
        assert enfa.accepts([symb_a, symb_a, symb_b, symb_c])
        assert not enfa.accepts([symb_c])
        assert not enfa.accepts([symb_b])
        assert not enfa.accepts([])

    def test_kleene(self):
        """ Tests the kleene star of an epsilon NFA """
        enfa0 = get_enfa_example0()
        symb_a = Symbol("a")
        symb_b = Symbol("b")
        enfa = enfa0.kleene_star()
        assert enfa.accepts([symb_b])
        assert enfa.accepts([symb_a, symb_b])
        assert enfa.accepts([symb_a, symb_b, symb_a, symb_b])
        assert enfa.accepts([])
        assert enfa.accepts([symb_b, symb_b])
        assert not enfa.accepts([symb_a])
        assert not enfa.accepts([symb_a, symb_b, symb_a])

    def test_complement(self):
        """ Tests the complement operation """
        enfa = EpsilonNFA()
        state0 = State(0)
        state1 = State(1)
        state2 = State(2)
        symb_a = Symbol("a")
        enfa.add_start_state(state0)
        enfa.add_final_state(state2)
        enfa.add_transition(state0, Epsilon(), state1)
        enfa.add_transition(state1, symb_a, state2)
        enfa_comp = -enfa
        assert not enfa_comp.accepts([symb_a])

    def test_intersection(self):
        """ Tests the intersection of two enfas """
        enfa0 = get_enfa_example0()
        symb_a = Symbol("a")
        symb_b = Symbol("b")
        eps = Epsilon()
        enfa1 = EpsilonNFA()
        state0 = State(10)
        state1 = State(11)
        state2 = State(12)
        state3 = State(13)
        state4 = State(14)
        enfa1.add_start_state(state0)
        enfa1.add_final_state(state3)
        enfa1.add_final_state(state4)
        enfa1.add_transition(state0, eps, state1)
        enfa1.add_transition(state1, symb_a, state2)
        enfa1.add_transition(state2, eps, state3)
        enfa1.add_transition(state3, symb_b, state4)
        enfa = enfa0 & enfa1
        assert len(enfa.start_states) == 4
        assert len(enfa.final_states) == 2
        assert len(enfa.symbols) == 2
        assert enfa.accepts([symb_a, symb_b])
        assert not enfa.accepts([symb_b])
        assert not enfa.accepts([symb_a])
        assert not enfa.accepts([])
        assert not enfa.accepts([symb_a, symb_a, symb_b])

    def test_difference(self):
        """ Tests the intersection of two languages """
        enfa0 = get_enfa_example0()
        enfa1 = get_enfa_example1()
        symb_a = Symbol("a")
        symb_b = Symbol("b")
        symb_c = Symbol("c")
        enfa = enfa0 - enfa1
        assert enfa.accepts([symb_a, symb_b])
        assert enfa.accepts([symb_b])
        assert not enfa.accepts([symb_c])
        assert not enfa.accepts([])
        enfa2 = EpsilonNFA()
        state0 = State(0)
        enfa2.add_start_state(state0)
        enfa2.add_final_state(state0)
        enfa2.add_transition(state0, symb_b, state0)
        enfa = enfa0.get_difference(enfa2)
        assert enfa.accepts([symb_a, symb_b])
        assert not enfa.accepts([symb_b])
        assert not enfa.accepts([symb_c])

    def test_reverse(self):
        """ Test the reversal of a language """
        enfa0 = get_enfa_example0()
        symb_a = Symbol("a")
        symb_b = Symbol("b")
        enfa = ~enfa0
        assert enfa.accepts([symb_b])
        assert enfa.accepts([symb_b, symb_a])
        assert enfa.accepts([symb_b, symb_a, symb_a])
        assert not enfa.accepts([symb_a, symb_b])
        assert not enfa.accepts([symb_a])
        assert not enfa.accepts([])

    def test_empty(self):
        """ Tests the emptiness of a finite automaton """
        assert get_enfa_example0()
        assert not get_enfa_example1().is_empty()
        enfa = EpsilonNFA()
        state0 = State(0)
        enfa.add_start_state(state0)
        assert enfa.is_empty()
        state1 = State(1)
        symb_a = Symbol('a')
        enfa.add_transition(state0, symb_a, state1)
        assert enfa.is_empty()
        enfa.add_final_state(state1)
        assert not enfa.is_empty()

    def test_minimization(self):
        """ Tests the minimization algorithm """
        enfa = get_enfa_example0_bis()
        symb_a = Symbol("a")
        symb_b = Symbol("b")
        enfa = enfa.minimize()
        assert enfa.is_deterministic()
        assert len(enfa.states) == 2
        assert enfa.accepts([symb_a, symb_b])
        assert enfa.accepts([symb_a, symb_a, symb_b])
        assert enfa.accepts([symb_b])
        assert not enfa.accepts([symb_a])
        enfa = get_example_non_minimal()
        enfa = enfa.minimize()
        assert enfa.is_deterministic()
        assert len(enfa.states) == 3
        assert enfa.accepts([symb_a, symb_b])
        assert enfa.accepts([symb_a, symb_a, symb_b])
        assert not enfa.accepts([symb_b])
        assert not enfa.accepts([symb_a])
        enfa = EpsilonNFA()
        enfa = enfa.minimize()
        assert enfa.is_deterministic()
        assert len(enfa.states) == 1
        assert not enfa.accepts([])

    def test_to_fst(self):
        """ Tests to turn a ENFA into a FST """
        enfa = EpsilonNFA()
        fst = enfa.to_fst()
        assert len(fst.states) == 0
        assert len(fst.final_states) == 0
        assert len(fst.start_states) == 0
        assert len(fst.input_symbols) == 0
        assert len(fst.output_symbols) == 0
        assert fst.get_number_transitions() == 0

        state0 = State("q0")
        s0bis = State("q0bis")
        enfa.add_start_state(state0)
        enfa.add_start_state(s0bis)
        fst = enfa.to_fst()
        assert len(fst.states) == 2
        assert len(fst.final_states) == 0
        assert len(fst.start_states) == 2
        assert len(fst.input_symbols) == 0
        assert len(fst.output_symbols) == 0
        assert fst.get_number_transitions() == 0

        sfinal = State("qfinal")
        sfinalbis = State("qfinalbis")
        enfa.add_final_state(sfinal)
        enfa.add_final_state(sfinalbis)
        fst = enfa.to_fst()
        assert len(fst.states) == 4
        assert len(fst.final_states) == 2
        assert len(fst.start_states) == 2
        assert len(fst.input_symbols) == 0
        assert len(fst.output_symbols) == 0
        assert fst.get_number_transitions() == 0

        enfa.add_transition(state0, Symbol("a"), sfinal)
        enfa.add_transition(sfinal, Symbol("b"), sfinal)
        enfa.add_transition(state0, Symbol("c"), sfinalbis)
        fst = enfa.to_fst()
        assert len(fst.states) == 4
        assert len(fst.final_states) == 2
        assert len(fst.start_states) == 2
        assert len(fst.input_symbols) == 3
        assert len(fst.output_symbols) == 3
        assert fst.get_number_transitions() == 3

        enfa.add_transition(state0, Epsilon(), sfinalbis)
        fst = enfa.to_fst()
        assert len(fst.states) == 4
        assert len(fst.final_states) == 2
        assert len(fst.start_states) == 2
        assert len(fst.input_symbols) == 3
        assert len(fst.output_symbols) == 3
        assert fst.get_number_transitions() == 4

        trans0 = list(fst.translate(["a"]))
        assert trans0 == [["a"]]

        trans0 = list(fst.translate(["a", "b", "b"]))
        assert trans0 == [["a", "b", "b"]]

        trans0 = list(fst.translate(["b", "b"]))
        assert trans0 == []

        trans0 = list(fst.translate(["c"]))
        assert trans0 == [["c"]]

    def test_cyclic(self):
        enfa = EpsilonNFA()
        state0 = State(0)
        state1 = State(1)
        symb_a = Symbol('a')
        enfa.add_start_state(state0)
        enfa.add_transition(state0, symb_a, state1)
        enfa.add_transition(state1, Epsilon(), state0)
        assert not enfa.is_acyclic()

    def test_export_networkx(self):
        enfa = EpsilonNFA()
        state0 = State("0")
        state1 = State(1)
        symb_a = Symbol('a')
        enfa.add_start_state(state0)
        enfa.add_final_state(state1)
        enfa.add_transition(state0, symb_a, state1)
        enfa.add_transition(state1, Epsilon(), state0)
        graph = enfa.to_networkx()
        assert isinstance(graph, networkx.MultiDiGraph)
        assert "0" in graph
        assert ("0", 1) in graph.edges
        assert "a" in [x["label"] for x in graph["0"][1].values()]
        assert graph.nodes["0"]["is_start"]
        assert not graph.nodes["0"]["is_final"]
        assert not graph.nodes[1]["is_start"]
        assert graph.nodes[1]["is_final"]
        enfa.write_as_dot("enfa.dot")

    def test_import_networkx(self):
        enfa = EpsilonNFA()
        state0 = State("0")
        state1 = State(1)
        symb_a = Symbol('a')
        enfa.add_start_state(state0)
        enfa.add_final_state(state1)
        enfa.add_transition(state0, symb_a, state1)
        enfa.add_transition(state1, Epsilon(), state0)
        graph = enfa.to_networkx()
        enfa_from_nx = EpsilonNFA.from_networkx(graph)
        assert enfa_from_nx.accepts([symb_a])
        assert enfa_from_nx.accepts([symb_a, symb_a])
        assert not enfa_from_nx.accepts([])

    def test_iter(self):
        enfa = EpsilonNFA()
        state0 = State("0")
        state1 = State(1)
        symb_a = Symbol('a')
        enfa.add_start_state(state0)
        enfa.add_final_state(state1)
        enfa.add_transition(state0, symb_a, state1)
        enfa.add_transition(state1, Epsilon(), state0)
        counter = 0
        for s_from, symb, s_to in enfa:
            counter += 1
            assert (s_from, symb, s_to) in enfa
        assert (state1, symb_a, state1) not in enfa
        assert ("0", "a", 1) in enfa
        assert counter == 2

    def test_equivalent(self):
        enfa0 = EpsilonNFA()
        state0 = State("0")
        state1 = State(1)
        symb_a = Symbol('a')
        enfa0.add_start_state(state0)
        enfa0.add_final_state(state1)
        enfa0.add_transition(state0, symb_a, state1)
        enfa0.add_transition(state1, Epsilon(), state0)
        enfa1 = EpsilonNFA()
        enfa1.add_start_state(state0)
        enfa1.add_final_state(state1)
        enfa1.add_transition(state0, symb_a, state1)
        enfa1.add_transition(state1, symb_a, state1)
        assert enfa0.is_equivalent_to(enfa1)

    def test_non_equivalent(self):
        enfa0 = EpsilonNFA()
        state0 = State("0")
        state1 = State(1)
        symb_a = Symbol('a')
        enfa0.add_start_state(state0)
        enfa0.add_final_state(state1)
        enfa0.add_transition(state0, symb_a, state1)
        enfa0.add_transition(state1, Epsilon(), state0)
        enfa1 = EpsilonNFA()
        enfa1.add_start_state(state0)
        enfa1.add_final_state(state1)
        enfa1.add_transition(state0, symb_a, state1)
        enfa1.add_transition(state1, symb_a, state0)
        assert not enfa0.is_equivalent_to(enfa1)

    def test_get_as_dict(self):
        enfa0 = EpsilonNFA()
        state0 = State("0")
        state1 = State(1)
        symb_a = Symbol('a')
        enfa0.add_start_state(state0)
        enfa0.add_final_state(state1)
        enfa0.add_transition(state0, symb_a, state1)
        enfa0.add_transition(state1, Epsilon(), state0)
        d_enfa = enfa0.to_dict()
        assert state0 in d_enfa
        assert symb_a in d_enfa[state0]
        assert state1 in d_enfa[state0][symb_a]

    def test_len(self):
        enfa = get_enfa_example1()
        assert len(enfa) == 1

    def test_call(self):
        enfa = get_enfa_example1()
        assert len(enfa(2)) == 1

    def test_example_doc(self):
        enfa = EpsilonNFA()
        state0 = State(0)
        state1 = State(1)
        symb_a = Symbol("0")
        symb_b = Symbol("1")
        enfa.add_start_state(state0)
        enfa.add_final_state(state1)
        enfa.add_transition(state0, symb_a, state0)
        enfa.add_transition(state1, symb_b, state0)
        enfa.add_transition(state1, symb_b, state1)

        # Turn a finite automaton into a regex...
        regex = enfa.to_regex()
        # And turn it back into an epsilon non deterministic automaton
        enfa2 = regex.to_epsilon_nfa()
        assert enfa == enfa2

    def test_remove_epsilon_transitions(self):
        enfa = EpsilonNFA()
        enfa.add_transitions([
            ("a", "epsilon", "b"),
            ("b", "t", "c"),
            ("a", "u", "c"),
            ("b", "epsilon", "d")
        ])
        assert enfa.get_number_transitions() == 4
        enfa.add_start_state("a")
        enfa.add_final_state("b")
        assert len(enfa.start_states) == 1
        nfa = enfa.remove_epsilon_transitions()
        assert len(nfa.start_states) == 3
        assert len(nfa.final_states) == 2
        assert nfa.get_number_transitions() == 3
        assert nfa.is_equivalent_to(enfa)

    def test_word_generation(self):
        enfa = get_enfa_example_for_word_generation()
        accepted_words = list(enfa.get_accepted_words())
        assert [] in accepted_words
        assert [Symbol("b")] in accepted_words
        assert [Symbol("c")] in accepted_words
        assert [Symbol("d"), Symbol("e")] in accepted_words
        assert [Symbol("d"), Symbol("e"), Symbol("f")] in accepted_words
        assert len(accepted_words) == 5

    def test_cyclic_word_generation(self):
        enfa = get_cyclic_enfa_example()
        max_length = 10
        accepted_words = [[Symbol("a")] +
                          [Symbol("b")] * (i + 1) +
                          [Symbol("c")]
                          for i in range(max_length - 2)]
        actual_accepted_words = list(enfa.get_accepted_words(max_length))
        assert accepted_words == actual_accepted_words

    def test_epsilon_cycle_word_generation(self):
        enfa = get_epsilon_cycle_enfa_example()
        max_length = 4
        accepted_words = list(enfa.get_accepted_words(max_length))
        assert [] in accepted_words
        assert [Symbol("a"), Symbol("c")] in accepted_words
        assert [Symbol("a"), Symbol("b"), Symbol("c")] in accepted_words
        assert [Symbol("a"), Symbol("b"),
                Symbol("b"), Symbol("c")] in accepted_words
        assert len(accepted_words) == 4

    def test_max_length_zero_accepting_empty_string(self):
        enfa = get_enfa_example_for_word_generation()
        accepted_words = list(enfa.get_accepted_words(0))
        assert accepted_words == [[]]

    def test_max_length_zero_not_accepting_empty_string(self):
        enfa = get_cyclic_enfa_example()
        accepted_words = list(enfa.get_accepted_words(0))
        assert not accepted_words


def get_digits_enfa():
    """ An epsilon NFA to recognize digits """
    epsilon = Epsilon()
    plus = Symbol("+")
    minus = Symbol("-")
    point = Symbol(".")
    digits = [Symbol(x) for x in range(10)]
    states = [State("q" + str(x)) for x in range(6)]
    enfa = EpsilonNFA()
    enfa.add_start_state(states[0])
    enfa.add_final_state(states[5])
    enfa.add_transition(states[0], epsilon, states[1])
    enfa.add_transition(states[0], plus, states[1])
    enfa.add_transition(states[0], minus, states[1])
    for digit in digits:
        enfa.add_transitions([
            (states[1], digit, states[1]),
            (states[1], digit, states[4]),
            (states[2], digit, states[3]),
            (states[3], digit, states[3])])
    enfa.add_transitions([
        (states[1], point, states[2]),
        (states[4], point, states[3]),
        (states[3], epsilon, states[5])])
    return enfa, digits, epsilon, plus, minus, point


def get_enfa_example0():
    """ Gives an example ENFA
    Accepts a*b
    """
    enfa0 = EpsilonNFA()
    state0 = State(0)
    state1 = State(1)
    state2 = State(2)
    symb_a = Symbol("a")
    symb_b = Symbol("b")
    enfa0.add_start_state(state0)
    enfa0.add_final_state(state2)
    enfa0.add_transition(state0, symb_a, state0)
    enfa0.add_transition(state0, Epsilon(), state1)
    enfa0.add_transition(state1, symb_b, state2)
    return enfa0


def get_enfa_example1():
    """ Gives and example ENFA
    Accepts c
    """
    enfa1 = EpsilonNFA()
    state2 = State(2)
    state3 = State(3)
    symb_c = Symbol("c")
    enfa1.add_start_state(state2)
    enfa1.add_final_state(state3)
    enfa1.add_transition(state2, symb_c, state3)
    return enfa1


def get_enfa_example0_bis():
    """ A non minimal NFA, equivalent to example0 """
    enfa0 = EpsilonNFA()
    state3 = State(3)
    state4 = State(4)
    state0 = State(0)
    state1 = State(1)
    state2 = State(2)
    symb_a = Symbol("a")
    symb_b = Symbol("b")
    enfa0.add_start_state(state0)
    enfa0.add_final_state(state2)
    enfa0.add_final_state(state4)
    enfa0.add_transition(state0, symb_a, state0)
    enfa0.add_transition(state0, Epsilon(), state1)
    enfa0.add_transition(state1, symb_b, state2)
    # New part
    enfa0.add_transition(state0, Epsilon(), state3)
    enfa0.add_transition(state3, symb_a, state3)
    enfa0.add_transition(state3, symb_b, state4)
    return enfa0


def get_example_non_minimal():
    """ A non minimal example a.a*.b"""
    enfa0 = EpsilonNFA()
    state0 = State(0)
    state3 = State(3)
    state4 = State(4)
    state5 = State(5)
    state6 = State(6)
    state1 = State(1)
    state2 = State(2)
    symb_a = Symbol("a")
    symb_b = Symbol("b")
    enfa0.add_start_state(state0)
    enfa0.add_final_state(state3)
    enfa0.add_final_state(state4)
    enfa0.add_transition(state0, symb_a, state1)
    enfa0.add_transition(state1, symb_a, state2)
    enfa0.add_transition(state2, symb_a, state5)
    enfa0.add_transition(state5, symb_a, state6)
    enfa0.add_transition(state6, symb_a, state1)
    enfa0.add_transition(state1, symb_b, state3)
    enfa0.add_transition(state2, symb_b, state4)
    enfa0.add_transition(state5, symb_b, state3)
    enfa0.add_transition(state6, symb_b, state4)
    return enfa0


def get_enfa_example_for_word_generation():
    """ ENFA example for the word generation test """
    enfa = EpsilonNFA()
    states = [State(x) for x in range(9)]
    symbol_a = Symbol("a")
    symbol_b = Symbol("b")
    symbol_c = Symbol("c")
    symbol_d = Symbol("d")
    symbol_e = Symbol("e")
    symbol_f = Symbol("f")
    epsilon = Epsilon()
    enfa.add_transitions([
        (states[0], symbol_a, states[1]),
        (states[0], epsilon, states[2]),
        (states[1], symbol_a, states[1]),
        (states[2], symbol_b, states[3]),
        (states[2], symbol_c, states[3]),
        (states[4], symbol_d, states[5]),
        (states[5], symbol_e, states[6]),
        (states[5], symbol_e, states[7]),
        (states[7], symbol_f, states[8]),
    ])
    enfa.add_start_state(states[0])
    enfa.add_start_state(states[4])
    enfa.add_final_state(states[3])
    enfa.add_final_state(states[4])
    enfa.add_final_state(states[6])
    enfa.add_final_state(states[8])
    return enfa


def get_cyclic_enfa_example():
    """ ENFA example with a cycle on the path to the final state """
    enfa = EpsilonNFA()
    states = [State(x) for x in range(4)]
    symbol_a = Symbol("a")
    symbol_b = Symbol("b")
    symbol_c = Symbol("c")
    epsilon = Epsilon()
    enfa.add_transitions([
        (states[0], symbol_a, states[1]),
        (states[1], symbol_b, states[2]),
        (states[2], epsilon, states[1]),
        (states[2], symbol_c, states[3]),
    ])
    enfa.add_start_state(states[0])
    enfa.add_final_state(states[3])
    return enfa


def get_epsilon_cycle_enfa_example():
    """ ENFA example with an epsilon cycle """
    enfa = EpsilonNFA()
    states = [State(x) for x in range(4)]
    symbol_a = Symbol("a")
    symbol_b = Symbol("b")
    symbol_c = Symbol("c")
    epsilon = Epsilon()
    enfa.add_transitions([
        (states[0], epsilon, states[0]),
        (states[0], symbol_a, states[1]),
        (states[1], symbol_b, states[1]),
        (states[1], epsilon, states[2]),
        (states[2], epsilon, states[1]),
        (states[1], symbol_c, states[3]),
    ])
    enfa.add_start_state(states[0])
    enfa.add_final_state(states[0])
    enfa.add_final_state(states[3])
    return enfa
