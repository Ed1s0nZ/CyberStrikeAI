"""
Testing of indexed grammar, with manual rules
"""
from pyformlang.indexed_grammar import Rules
from pyformlang.indexed_grammar import ConsumptionRule
from pyformlang.indexed_grammar import EndRule
from pyformlang.indexed_grammar import ProductionRule
from pyformlang.indexed_grammar import DuplicationRule
from pyformlang.indexed_grammar import IndexedGrammar
from pyformlang.regular_expression import Regex


class TestIndexedGrammar:
    """ Tests the indexed grammar """

    # pylint: disable=missing-function-docstring

    def test_simple_ig_0(self):
        """Test"""

        l_rules = get_example_rules()

        for i in range(9):
            rules = Rules(l_rules, i)
            i_grammar = IndexedGrammar(rules)
            assert not i_grammar.is_empty()
            assert i_grammar.terminals == {"end", "b", "epsilon"}

    def test_simple_ig_1(self):
        # Write rules

        l_rules = [
            # Initialization rules
            ProductionRule("S", "Cinit", "end"),
            ProductionRule("Cinit", "C", "b"),
            ConsumptionRule("end", "C", "T"),
            EndRule("T", "epsilon"),
            # C[cm sigma] -> cm C[sigma]
            ConsumptionRule("cm", "C", "B0"),
            DuplicationRule("B0", "A0", "C"),
            EndRule("A0", "cm"),
            # C[b sigma] -> C[cm sigma] c b C[sigma]
            ConsumptionRule("b", "C", "B"),
            DuplicationRule("B", "A1", "D"),
            ConsumptionRule("b", "A1", "A1"),
            ConsumptionRule("bm", "A1", "A1"),
            ConsumptionRule("c", "A1", "A1"),
            ConsumptionRule("cm", "A1", "A1"),
            ConsumptionRule("end", "A1", "Abackm2"),
            ProductionRule("Abackm2", "Abackm1", "end"),
            ProductionRule("Abackm1", "C", "cm"),
            DuplicationRule("D", "E0", "C"),
            DuplicationRule("E0", "F0", "E1"),
            DuplicationRule("E1", "F1", "E2"),
            EndRule("E2", "epsilon"),
            EndRule("F0", "c"),
            EndRule("F1", "b")]

        rules = Rules(l_rules)
        i_grammar = IndexedGrammar(rules)
        assert not i_grammar.is_empty()

    def test_simple_ig_2(self):
        # Write rules

        l_rules = [
            # Initialization rules
            ProductionRule("S", "Cinit", "end"),
            ProductionRule("Cinit", "C", "b"),
            ConsumptionRule("end", "C", "T"),
            EndRule("T", "epsilon"),
            # C[b sigma] -> C[cm sigma] c b C[sigma]
            ConsumptionRule("b", "C", "B"),
            DuplicationRule("B", "A1", "D"),
            ConsumptionRule("b", "A1", "A1"),
            ConsumptionRule("bm", "A1", "A1"),
            ConsumptionRule("c", "A1", "A1"),
            ConsumptionRule("cm", "A1", "A1"),
            ConsumptionRule("end", "A1", "Abackm2"),
            ProductionRule("Abackm2", "Abackm1", "end"),
            ProductionRule("Abackm1", "C", "cm"),
            DuplicationRule("D", "E0", "C"),
            DuplicationRule("E0", "F0", "E1"),
            DuplicationRule("E1", "F1", "E2"),
            EndRule("E2", "epsilon"),
            EndRule("F0", "c"),
            EndRule("F1", "b")]

        rules = Rules(l_rules)
        i_grammar = IndexedGrammar(rules)
        assert i_grammar.is_empty()

    def test_simple_ig_3(self):
        # Write rules

        l_rules = []

        # Initialization rules

        l_rules.append(ProductionRule("S", "Cinit", "end"))
        l_rules.append(ProductionRule("Cinit", "C", "b"))
        l_rules.append(ConsumptionRule("end", "C", "T"))
        l_rules.append(EndRule("T", "epsilon"))

        # C[cm sigma] -> cm C[sigma]

        l_rules.append(ConsumptionRule("cm", "C", "B0"))
        l_rules.append(DuplicationRule("B0", "A0", "C"))
        l_rules.append(EndRule("A0", "cm"))

        rules = Rules(l_rules)
        i_grammar = IndexedGrammar(rules)
        assert i_grammar.is_empty()

    def test_simple_ig_4(self):
        # Write rules

        l_rules = []

        # Initialization rules

        l_rules.append(ProductionRule("S", "Cinit", "end"))
        l_rules.append(ConsumptionRule("end", "C", "T"))
        l_rules.append(EndRule("T", "epsilon"))

        # C[cm sigma] -> cm C[sigma]

        l_rules.append(ConsumptionRule("cm", "C", "B0"))
        l_rules.append(DuplicationRule("B0", "A0", "C"))
        l_rules.append(EndRule("A0", "cm"))

        # C[b sigma] -> C[cm sigma] c b C[sigma]

        l_rules.append(ConsumptionRule("b", "C", "B"))
        l_rules.append(DuplicationRule("B", "A1", "D"))
        l_rules.append(ConsumptionRule("b", "A1", "A1"))
        l_rules.append(ConsumptionRule("bm", "A1", "A1"))
        l_rules.append(ConsumptionRule("c", "A1", "A1"))
        l_rules.append(ConsumptionRule("cm", "A1", "A1"))
        l_rules.append(ConsumptionRule("end", "A1", "Abackm2"))
        l_rules.append(ProductionRule("Abackm2", "Abackm1", "end"))
        l_rules.append(ProductionRule("Abackm1", "C", "cm"))
        l_rules.append(DuplicationRule("D", "E0", "C"))
        l_rules.append(DuplicationRule("E0", "F0", "E1"))
        l_rules.append(DuplicationRule("E1", "F1", "E2"))
        l_rules.append(EndRule("E2", "epsilon"))
        l_rules.append(EndRule("F0", "c"))
        l_rules.append(EndRule("F1", "b"))

        rules = Rules(l_rules)
        i_grammar = IndexedGrammar(rules)
        assert i_grammar.is_empty()

    def test_simple_ig_5(self):
        # Write rules

        l_rules = []

        # Initialization rules

        l_rules.append(ProductionRule("S", "A", "f"))
        l_rules.append(ConsumptionRule("f", "A", "B"))
        l_rules.append(ConsumptionRule("f", "C", "F"))
        l_rules.append(ProductionRule("B", "C", "f"))
        l_rules.append(ProductionRule("D", "E", "f"))
        l_rules.append(EndRule("F", "epsilon"))
        l_rules.append(DuplicationRule("B0", "A0", "C"))

        rules = Rules(l_rules)
        i_grammar = IndexedGrammar(rules)
        assert not i_grammar.is_empty()

    def test_simple_ig_regular_expression(self):
        # Test for regular expression functions

        l_rules = []
        l_rules.append(ProductionRule("S", "Ci", "end"))
        l_rules.append(ProductionRule("Ci", "C", "q"))
        l_rules.append(ConsumptionRule("q", "C", "C0"))
        l_rules.append(ProductionRule("C0", "C0", "a-"))
        l_rules.append(DuplicationRule("C0", "T", "C"))
        l_rules.append(EndRule("T", "epsilon"))
        l_rules.append(ConsumptionRule("end", "C", "Cend"))
        l_rules.append(EndRule("Cend", "epsilon"))
        l_rules.append(ConsumptionRule("a-", "C", "C1"))
        l_rules.append(ConsumptionRule("a-", "C1", "C2"))
        l_rules.append(ConsumptionRule("a-", "C2", "C3"))
        l_rules.append(ConsumptionRule("a-", "C3", "C4"))
        l_rules.append(ConsumptionRule("a-", "C4", "C"))

        rules = Rules(l_rules)
        i_grammar = IndexedGrammar(rules)
        assert not i_grammar.is_empty()

    def test_simple_ig6(self):
        """ Test number 6 """
        l_rules = []
        l_rules.append(DuplicationRule("S", "S", "B"))
        rules = Rules(l_rules)
        i_grammar = IndexedGrammar(rules)
        assert i_grammar.is_empty()

        l_rules = []
        l_rules.append(DuplicationRule("S", "B", "S"))
        rules = Rules(l_rules)
        i_grammar = IndexedGrammar(rules)
        assert i_grammar.is_empty()

        l_rules = []
        l_rules.append(DuplicationRule("S", "A", "B"))
        l_rules.append(EndRule("A", "a"))
        l_rules.append(EndRule("B", "b"))
        rules = Rules(l_rules)
        i_grammar = IndexedGrammar(rules)
        assert not i_grammar.is_empty()

    def test_simple_ig7(self):
        """ Test 7 """
        l_rules = []
        l_rules.append(ProductionRule("S", "A", "end"))
        l_rules.append(ConsumptionRule("end", "A", "S"))
        l_rules.append(DuplicationRule("A", "B", "C"))
        l_rules.append(EndRule("B", "b"))
        l_rules.append(EndRule("C", "c"))
        rules = Rules(l_rules)
        i_grammar = IndexedGrammar(rules)
        assert not i_grammar.is_empty()

    def test_simple_ig8(self):
        """ Tests 8 """
        l_rules = []
        l_rules.append(ProductionRule("S", "Q", "end"))
        l_rules.append(ProductionRule("Q", "A", "end"))
        l_rules.append(ConsumptionRule("end", "A", "B"))
        l_rules.append(ConsumptionRule("end", "A", "C"))
        l_rules.append(ConsumptionRule("end", "A", "D"))
        l_rules.append(DuplicationRule("C", "G", "E"))
        l_rules.append(DuplicationRule("E", "G", "F"))
        l_rules.append(DuplicationRule("F", "G", "G"))
        l_rules.append(EndRule("G", "G"))
        rules = Rules(l_rules)
        i_grammar = IndexedGrammar(rules)
        assert not i_grammar.is_empty()

    def test_simple_ig9(self):
        """ Tests 9 {a^n b^n c^n}"""
        l_rules = []
        l_rules.append(ProductionRule("S", "T", "g"))
        l_rules.append(ProductionRule("T", "T", "f"))
        l_rules.append(DuplicationRule("T", "AB", "C"))
        l_rules.append(DuplicationRule("AB", "A", "B"))
        l_rules.append(ConsumptionRule("f", "A", "A2"))
        l_rules.append(ConsumptionRule("f", "B", "B2"))
        l_rules.append(ConsumptionRule("f", "C", "C2"))
        l_rules.append(DuplicationRule("A2", "Afinal", "A"))
        l_rules.append(DuplicationRule("B2", "Bfinal", "B"))
        l_rules.append(DuplicationRule("C2", "Cfinal", "C"))
        l_rules.append(EndRule("Afinal", "a"))
        l_rules.append(EndRule("Bfinal", "b"))
        l_rules.append(EndRule("Cfinal", "c"))
        l_rules.append(ConsumptionRule("g", "A", "Afinal"))
        l_rules.append(ConsumptionRule("g", "B", "Bfinal"))
        l_rules.append(ConsumptionRule("g", "C", "Cfinal"))
        rules = Rules(l_rules)
        i_grammar = IndexedGrammar(rules)
        assert not i_grammar.is_empty()

    def test_start_symbol(self):
        """ Tests the change of the start symbol """
        l_rules = []
        l_rules.append(EndRule("S", "s"))
        rules = Rules(l_rules)
        i_grammar = IndexedGrammar(rules, "S2")
        assert i_grammar.is_empty()

        i_grammar = IndexedGrammar(rules, "S")
        assert not i_grammar.is_empty()

        l_rules = []
        l_rules.append(EndRule("S2", "s"))
        rules = Rules(l_rules)
        i_grammar = IndexedGrammar(rules, start_variable="S2")
        assert not i_grammar.is_empty()

    def test_reachable(self):
        """ Tests the reachable symbols """
        l_rules = []
        l_rules.append(DuplicationRule("S", "A", "B"))
        l_rules.append(ProductionRule("A", "D", "f"))
        l_rules.append(ProductionRule("E", "D", "f"))
        l_rules.append(ProductionRule("E", "K", "f"))
        l_rules.append(EndRule("D", "d"))
        l_rules.append(DuplicationRule("D", "D", "D"))
        l_rules.append(DuplicationRule("D", "D", "A"))
        l_rules.append(ConsumptionRule("f", "B", "G"))
        l_rules.append(ConsumptionRule("f", "B", "A"))
        rules = Rules(l_rules)
        i_grammar = IndexedGrammar(rules, start_variable="S")
        reachable = i_grammar.get_reachable_non_terminals()
        assert reachable == {"S", "A", "B", "D", "G"}

    def test_generating(self):
        """ Tests the generating symbols """
        l_rules = []
        l_rules.append(DuplicationRule("S", "A", "B"))
        l_rules.append(ProductionRule("A", "D", "f"))
        l_rules.append(ProductionRule("E", "D", "f"))
        l_rules.append(EndRule("D", "d"))
        l_rules.append(ConsumptionRule("f", "B", "G"))
        l_rules.append(ConsumptionRule("f", "X", "G"))
        l_rules.append(DuplicationRule("Q", "A", "E"))
        l_rules.append(DuplicationRule("Q", "A", "D"))
        l_rules.append(DuplicationRule("Q", "D", "E"))
        rules = Rules(l_rules)
        i_grammar = IndexedGrammar(rules, start_variable="S")
        generating = i_grammar.get_generating_non_terminals()
        assert generating == {"D", "A", "E", "Q"}

    def test_removal_useless(self):
        """ Tests the removal of useless symbols """
        l_rules = []

        l_rules.append(ProductionRule("S", "D", "f"))
        l_rules.append(ConsumptionRule("f", "B", "Bfinal"))
        l_rules.append(DuplicationRule("D", "A", "B"))
        l_rules.append(ConsumptionRule("f", "A", "Afinal"))
        l_rules.append(EndRule("Afinal", "a"))
        l_rules.append(EndRule("Bfinal", "b"))
        l_rules.append(ConsumptionRule("f", "A", "Q"))
        l_rules.append(EndRule("R", "b"))

        rules = Rules(l_rules)
        i_grammar = IndexedGrammar(rules, start_variable="S")
        i_grammar2 = i_grammar.remove_useless_rules()
        assert not i_grammar.is_empty()
        assert i_grammar2.non_terminals == \
                         i_grammar2.get_generating_non_terminals()
        assert i_grammar2.non_terminals == \
                         i_grammar2.get_reachable_non_terminals()

    def test_intersection(self):
        """ Tests the intersection of indexed grammar with regex
        Long to run!
        """
        l_rules = [ProductionRule("S", "D", "f"),
                   DuplicationRule("D", "A", "B"),
                   ConsumptionRule("f", "A", "Afinal"),
                   ConsumptionRule("f", "B", "Bfinal"), EndRule("Afinal", "a"),
                   EndRule("Bfinal", "b")]
        rules = Rules(l_rules, 6)
        indexed_grammar = IndexedGrammar(rules)
        i_inter = indexed_grammar.intersection(Regex("a.b"))
        assert i_inter


def get_example_rules():
    """ Duplicate example of rules """
    l_rules = [  # Initialization rules
        ProductionRule("S", "Cinit", "end"),
        ProductionRule("Cinit", "C", "b"),
        ConsumptionRule("end", "C", "T"),
        EndRule("T", "epsilon"),
        # C[cm sigma] -> cm C[sigma]
        ConsumptionRule("b", "C", "B0"),
        DuplicationRule("B0", "A0", "C"),
        EndRule("A0", "b")]

    return l_rules
