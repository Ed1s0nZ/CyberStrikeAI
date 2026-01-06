"""
Tests for regular expressions
"""
from pyformlang.regular_expression import Regex, MisformedRegexError, PythonRegex
from pyformlang import finite_automaton
import pytest


class TestRegex:
    """ Tests for regex """

    # pylint: disable=missing-function-docstring,too-many-public-methods

    def test_creation(self):
        """ Try to create regex """
        regex = Regex("a|b")
        assert regex.get_number_symbols() == 2
        assert regex.get_number_operators() == 1
        regex = Regex("a b")
        assert regex.get_number_symbols() == 2
        assert regex.get_number_operators() == 1
        regex = Regex("a b c")
        assert regex.get_number_symbols() == 3
        assert regex.get_number_operators() == 2
        regex = Regex("(a b)|c")
        assert regex.get_number_symbols() == 3
        assert regex.get_number_operators() == 2
        regex = Regex("")
        assert regex.get_number_symbols() == 1
        assert regex.get_number_operators() == 0
        regex = Regex("a*")
        assert regex.get_number_symbols() == 1
        assert regex.get_number_operators() == 1
        regex = Regex("a**")
        assert regex.get_number_symbols() == 1
        assert regex.get_number_operators() == 2
        regex = Regex("a*b|c")
        assert regex.get_number_symbols() == 3
        assert regex.get_number_operators() == 3
        regex = Regex("a*(b|c)")
        assert regex.get_number_symbols() == 3
        assert regex.get_number_operators() == 3
        regex = Regex("a*.(b|c)")
        assert regex.get_number_symbols() == 3
        assert regex.get_number_operators() == 3
        regex = Regex("a*.(b|c)epsilon")
        assert regex.get_number_symbols() == 4
        assert regex.get_number_operators() == 4
        regex = Regex("$")
        assert regex.get_number_symbols() == 1
        regex = Regex("a|")
        assert regex.get_number_symbols() == 2
        assert regex.get_number_operators() == 1
        with pytest.raises(MisformedRegexError):
            Regex(")a b()")
        with pytest.raises(MisformedRegexError):
            Regex("(a b()")
        with pytest.raises(MisformedRegexError):
            Regex("(a b))")
        with pytest.raises(MisformedRegexError):
            Regex("| a b")
        regex = Regex("(a-|a a b)")
        assert regex.get_number_symbols() == 4
        assert regex.get_number_operators() == 3

    def test_to_enfa0(self):
        """ Tests the transformation to a regex """
        symb_a = finite_automaton.Symbol("a")
        symb_b = finite_automaton.Symbol("b")
        symb_c = finite_automaton.Symbol("c")
        epsilon = finite_automaton.Epsilon()
        regex = Regex("a|b")
        enfa = regex.to_epsilon_nfa()
        assert enfa.accepts([symb_a])
        assert enfa.accepts([symb_b])
        assert not enfa.accepts([symb_c])
        assert not enfa.accepts([epsilon])
        assert not enfa.accepts([symb_a, symb_b])
        regex = Regex("a b")
        enfa = regex.to_epsilon_nfa()
        assert not enfa.accepts([symb_a])
        assert not enfa.accepts([symb_b])
        assert enfa.accepts([symb_a, symb_b])
        regex = Regex("a b c")
        enfa = regex.to_epsilon_nfa()
        assert not enfa.accepts([symb_a, symb_b])
        assert enfa.accepts([symb_a, symb_b, symb_c])
        assert not enfa.accepts([symb_a, symb_b, symb_a])
        regex = Regex("(a b)|c")
        enfa = regex.to_epsilon_nfa()
        assert enfa.accepts([symb_a, symb_b])
        assert not enfa.accepts([symb_a, symb_c])
        assert not enfa.accepts([symb_b, symb_c])
        assert enfa.accepts([symb_c])
        regex = Regex("")
        enfa = regex.to_epsilon_nfa()
        assert not enfa.accepts([symb_a])
        assert not enfa.accepts([symb_b])
        assert not enfa.accepts([symb_c])
        assert not enfa.accepts([])
        regex = Regex("a*")
        enfa = regex.to_epsilon_nfa()
        assert enfa.accepts([symb_a])
        assert enfa.accepts([])
        assert enfa.accepts([symb_a, symb_a])
        assert enfa.accepts([symb_a, symb_a, symb_a])

    def test_to_enfa1(self):
        """ Tests the transformation to a regex """
        symb_a = finite_automaton.Symbol("a")
        symb_b = finite_automaton.Symbol("b")
        symb_c = finite_automaton.Symbol("c")
        regex = Regex("a**")
        enfa = regex.to_epsilon_nfa()
        assert enfa.accepts([symb_a])
        assert enfa.accepts([])
        assert enfa.accepts([symb_a, symb_a])
        assert enfa.accepts([symb_a, symb_a, symb_a])
        regex = Regex("a*b|c")
        enfa = regex.to_epsilon_nfa()
        assert enfa.accepts([symb_a, symb_a, symb_b])
        assert enfa.accepts([symb_b])
        assert enfa.accepts([symb_c])
        assert not enfa.accepts([symb_a, symb_a, symb_c])
        regex = Regex("a*(b|c)")
        enfa = regex.to_epsilon_nfa()
        assert enfa.accepts([symb_a, symb_a, symb_b])
        assert enfa.accepts([symb_b])
        assert enfa.accepts([symb_c])
        assert enfa.accepts([symb_a, symb_a, symb_c])
        regex = Regex("a*.(b|c)")
        enfa = regex.to_epsilon_nfa()
        assert enfa.accepts([symb_a, symb_a, symb_b])
        assert enfa.accepts([symb_b])
        assert enfa.accepts([symb_c])
        assert enfa.accepts([symb_a, symb_a, symb_c])
        regex = Regex("a*.(b|c)epsilon")
        enfa = regex.to_epsilon_nfa()
        assert enfa.accepts([symb_a, symb_a, symb_b])
        assert enfa.accepts([symb_b])
        assert enfa.accepts([symb_c])
        assert enfa.accepts([symb_a, symb_a, symb_c])
        regex = Regex("$")
        enfa = regex.to_epsilon_nfa()
        assert not enfa.accepts([symb_a])
        assert not enfa.accepts([symb_b])
        assert not enfa.accepts([symb_c])
        assert enfa.accepts([])

    def test_print(self):
        """ Test printing functions """
        regex = Regex("a*.(b|c)epsilon")
        tree_str = regex.get_tree_str()
        assert "Concatenation" in tree_str
        assert "Union" in tree_str
        assert "Kleene Star" in tree_str
        assert "Symbol" in tree_str
        regex = Regex("")
        tree_str = regex.get_tree_str()
        assert "Empty" in tree_str

    def test_get_repr(self):
        regex0 = Regex("a*.(b|c)epsilon")
        regex_str = str(regex0)
        regex1 = Regex(regex_str)
        dfa0 = regex0.to_epsilon_nfa().to_deterministic().minimize()
        dfa1 = regex1.to_epsilon_nfa().to_deterministic().minimize()
        assert dfa0 == dfa1

    def test_accepts(self):
        regex = Regex("a|b|c")
        assert regex.accepts(["a"])
        assert not regex.accepts(["a", "b"])

    def test_from_python_simple(self):
        regex = PythonRegex("abc")
        assert regex.accepts(["a", "b", "c"])
        assert not regex.accepts(["a", "b", "b"])
        assert not regex.accepts(["a", "b"])

    def test_from_python_brackets(self):
        regex = PythonRegex("a[bc]")
        assert regex.accepts(["a", "b"])
        assert regex.accepts(["a", "c"])
        assert not regex.accepts(["a", "b", "c"])
        assert not regex.accepts(["a", "a"])

    def test_space(self):
        regex = Regex("\\ ")
        assert regex.accepts([" "])

    def test_parenthesis_gorilla(self):
        regex = Regex("george touches (a|an) (sky|gorilla) !")
        assert regex.accepts(["george", "touches", "a", "sky", "!"])

    def test_regex_or(self):
        regex = Regex("a|b")
        assert regex.accepts(["a"])

    def test_regex_or_concat(self):
        regex = Regex("c (a|b)")
        assert regex.accepts(["c", "b"])

    def test_regex_two_or_concat(self):
        regex = Regex("c (a|b) (d|e)")
        assert regex.accepts(["c", "b", "e"])

    def test_regex_two_or_concat_parenthesis(self):
        regex = Regex("c.(a|b)(d|e)")
        assert regex.accepts(["c", "b", "e"])

    def test_regex_two_or_concat_parenthesis2(self):
        regex = Regex("c (a|(b d)|e)")
        assert regex.accepts(["c", "a"])
        assert regex.accepts(["c", "b", "d"])
        assert regex.accepts(["c", "e"])

    def test_regex_two_or_concat_parenthesis2_concat(self):
        regex = Regex("c (a|(b d)|e) !")
        assert regex.accepts(["c", "a", "!"])
        assert regex.accepts(["c", "b", "d", "!"])
        assert regex.accepts(["c", "e", "!"])

    def test_regex_or_two_concat(self):
        regex = Regex("c d (a|b)")
        assert regex.accepts(["c", "d", "b"])

    def test_after_union(self):
        regex = Regex("(a|b) !")
        assert regex.accepts(["a", "!"])

    def test_star_union(self):
        regex = Regex("a*(b|c)")
        assert regex.accepts(["a", "a", "c"])

    def test_misformed(self):
        with pytest.raises(MisformedRegexError):
            Regex(")")

    def test_misformed2(self):
        with pytest.raises(MisformedRegexError):
            Regex("(")

    def test_escaped_parenthesis(self):
        regex = Regex("\\(")
        assert regex.accepts(["("])

    def test_escaped_mid_bar(self):
        regex = Regex('a(0|1|2|3|4|5|6|7|8|9|a|b|c|d|e|f|g|h|i|j|k|l|m|n|o|p|q'
                      '|r|s|t|u|v|w|x|y|z|A|B|C|D|E|F|G|H|I|J|K|L|M|N|O|P|Q|R|'
                      'S|T|U|V|W|X|Y|Z|!|"|#|\\$|%|&|\'|\\(|\\)|\\*|\\+|,|-|'
                      '\\.|/|:|;|<|=|>|?|@|[|\\|]|^|_|`|{|\\||}|~|\\ |	|)')
        assert regex.accepts(["a", "|"])

    def test_to_cfg(self):
        regex = Regex("a")
        cfg = regex.to_cfg()
        assert cfg.contains(["a"])
        regex = Regex("a|b")
        cfg = regex.to_cfg()
        assert cfg.contains(["b"])
        regex = Regex("a b")
        cfg = regex.to_cfg()
        assert cfg.contains(["a", "b"])
        regex = Regex("a*")
        cfg = regex.to_cfg()
        assert cfg.contains([])
        assert cfg.contains(["a"])
        assert cfg.contains(["a", "a"])
        regex = Regex("epsilon")
        cfg = regex.to_cfg()
        assert cfg.contains([])
        regex = Regex("")
        cfg = regex.to_cfg()
        assert cfg.is_empty()
        regex = Regex("(a|b)* c")
        cfg = regex.to_cfg()
        assert cfg.contains(["c"])
        assert cfg.contains(["a", "c"])
        assert cfg.contains(["a", "b", "c"])

    def test_priority(self):
        assert Regex('b a* | a').accepts('a')
        assert Regex('b a* | a').accepts('b')
        assert Regex('(b a*) | a').accepts('a')

    def test_backslash_b(self):
        assert Regex("( a | \b )").accepts("\b")
        assert Regex("( a | \b )").accepts("a")
        assert not Regex("( a | \b )").accepts("b")

    def test_backslash(self):
        assert Regex("(\\\\|])").accepts("\\")
        assert Regex("(\\\\|])").accepts("]")
