""" Tests for RSA """
from pyformlang.finite_automaton.symbol import Symbol
from pyformlang.regular_expression import Regex

from pyformlang.rsa.recursive_automaton import RecursiveAutomaton
from pyformlang.rsa.box import Box


class TestRSA:
    """ Test class for RSA """
    def test_creation(self):
        """ Test the creation of an RSA """
        # S -> a S b | a b
        regex = Regex("a S b | a b")
        enfa = regex.to_epsilon_nfa()
        dfa = enfa.minimize()
        box = Box(dfa, "S")
        rsa_1 = RecursiveAutomaton(box, set())

        assert rsa_1.get_number_boxes() == 1
        assert box == rsa_1.get_box_by_nonterminal("S")
        assert rsa_1.nonterminals == {Symbol("S")}
        assert rsa_1.start_nonterminal == Symbol("S")

        rsa_2 = RecursiveAutomaton.from_regex(regex, "S")

        assert rsa_2 == rsa_1

    def test_from_regex(self):
        """ Test creation of an RSA from a regex"""
        # S -> a*
        rsa_2 = RecursiveAutomaton.from_regex(Regex("a*"), "S")

        enfa = Regex("a*").to_epsilon_nfa()
        dfa = enfa.minimize()
        box = Box(dfa, "S")
        rsa_1 = RecursiveAutomaton(box, set())

        assert rsa_2 == rsa_1

    def test_is_equals_to(self):
        """ Test the equals of two RSAs"""
        # S -> a* b*
        rsa_1 = RecursiveAutomaton.from_regex(Regex("a* b*"), "S")

        # S -> a+ b+
        rsa_2 = RecursiveAutomaton.from_regex(Regex("a a* b b*"), "S")

        assert rsa_1 != rsa_2

    def test_from_ebnf(self):
        """ Test reading RSA from ebnf"""
        # g1: S -> a S b | a b
        rsa1_g1 = RecursiveAutomaton.from_ebnf("S -> a S b | a b")
        rsa2_g1 = RecursiveAutomaton.from_regex(
            Regex("a S b | a b"), "S")

        assert rsa1_g1 == rsa2_g1

        # g2: S -> a V b
        #     V -> c S d | c d
        rsa1_g2 = RecursiveAutomaton.from_ebnf("""
            S -> a V b
            V -> c S d | c d""")
        assert rsa1_g2.get_number_boxes() == 2
        assert rsa1_g2.nonterminals == {Symbol("S"), Symbol("V")}

        dfa_s = Regex("a V b").to_epsilon_nfa().minimize()
        assert rsa1_g2.get_box_by_nonterminal("S") == Box(dfa_s, "S")

        dfa_v = Regex("c S d | c d").to_epsilon_nfa().minimize()
        assert rsa1_g2.get_box_by_nonterminal("V") == Box(dfa_v, "V")
