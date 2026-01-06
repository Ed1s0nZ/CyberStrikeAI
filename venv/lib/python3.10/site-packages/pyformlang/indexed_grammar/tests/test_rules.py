"""
Testing the rules
"""
from pyformlang.indexed_grammar import ProductionRule
from pyformlang.indexed_grammar import DuplicationRule
from pyformlang.indexed_grammar import Rules
from pyformlang.indexed_grammar import ConsumptionRule
from pyformlang.indexed_grammar import EndRule
from pyformlang.indexed_grammar.tests.test_indexed_grammar \
    import get_example_rules


class TestIndexedGrammar:
    """ Tests things related to rules """

    # pylint: disable=missing-function-docstring

    def test_consumption_rules(self):
        """ Tests the consumption rules """
        conso = ConsumptionRule("end", "C", "T")
        terminals = conso.terminals
        assert terminals == {"end"}
        representation = str(conso)
        assert representation == "C [ end ] -> T"

    def test_duplication_rules(self):
        """ Tests the duplication rules """
        dupli = DuplicationRule("B0", "A0", "C")
        assert dupli.terminals == set()
        assert str(dupli) == \
                         "B0 -> A0 C"

    def test_end_rule(self):
        """ Tests the end rules """
        end_rule = EndRule("A0", "b")
        assert end_rule.terminals == {"b"}
        assert end_rule.right_term == "b"
        assert str(end_rule) == "A0 -> b"

    def test_production_rules(self):
        """ Tests the production rules """
        produ = ProductionRule("S", "C", "end")
        assert produ.terminals == {"end"}
        assert str(produ) == "S -> C[ end ]"

    def test_rules(self):
        """ Tests the rules """
        l_rules = get_example_rules()
        rules = Rules(l_rules)
        assert rules.terminals == {"b", "end", "epsilon"}
        assert rules.length == (5, 2)
        rules.remove_production("S", "Cinit", "end")
        assert rules.length == (4, 2)
        rules.add_production("S", "Cinit", "end")
        assert rules.length == (5, 2)
