""" Tests the FST """
# pylint: disable=duplicate-code
from os import path

import pytest

from pyformlang.fst import FST
from pyformlang.indexed_grammar import (
    DuplicationRule, ProductionRule, EndRule,
    ConsumptionRule, IndexedGrammar, Rules)


@pytest.fixture
def fst0():
    fst0 = FST()
    fst0.add_start_state("q0")
    fst0.add_transition("q0", "a", "q1", ["b"])
    fst0.add_final_state("q1")
    yield fst0


@pytest.fixture
def fst1():
    fst1 = FST()
    fst1.add_start_state("q1")
    fst1.add_transition("q1", "b", "q2", ["c"])
    fst1.add_final_state("q2")
    yield fst1


class TestFST:
    """ Tests FST """
    def test_creation(self):
        """ Test Translate """
        fst = FST()
        assert fst is not None
        assert len(fst.states) == 0
        assert len(fst.input_symbols) == 0
        assert len(fst.output_symbols) == 0
        assert fst.get_number_transitions() == 0
        assert len(fst.final_states) == 0

        fst.add_start_state("q0")
        assert len(fst.states) == 1

        fst.add_transition("q0", "a", "q1", ["bc"])
        assert len(fst.states) == 2
        assert len(fst.input_symbols) == 1
        assert len(fst.output_symbols) == 1
        assert fst.get_number_transitions() == 1
        assert len(fst.final_states) == 0

        fst.add_transition("q0", "epsilon", "q1", ["bc"])
        assert len(fst.states) == 2
        assert len(fst.input_symbols) == 1
        assert len(fst.output_symbols) == 1
        assert fst.get_number_transitions() == 2
        assert len(fst.final_states) == 0

        fst.add_final_state("q2")
        assert len(fst.states) == 3
        assert len(fst.input_symbols) == 1
        assert len(fst.output_symbols) == 1
        assert fst.get_number_transitions() == 2
        assert len(fst.final_states) == 1

        fst.add_transition("q0", "a", "q1", ["d"])
        assert len(fst.states) == 3
        assert len(fst.input_symbols) == 1
        assert len(fst.output_symbols) == 2
        assert fst.get_number_transitions() == 3
        assert len(fst.final_states) == 1

    def test_translate(self):
        """ Test a translation """
        fst = FST()
        fst.add_start_state("q0")
        translation = list(fst.translate(["a"]))
        assert len(translation) == 0

        fst.add_transition("q0", "a", "q1", ["b"])
        translation = list(fst.translate(["a"]))
        assert len(translation) == 0

        fst.add_final_state("q1")
        translation = list(fst.translate(["a"]))
        assert len(translation) == 1
        assert translation == [["b"]]

        fst.add_transition("q1", "epsilon", "q1", ["c"])
        translation = list(fst.translate(["a"], max_length=10))
        assert len(translation) == 10
        assert ["b"] in translation
        assert ["b", "c"] in translation
        assert ["b"] + ["c"] * 9 in translation

    def test_intersection_indexed_grammar(self):
        """ Test the intersection with indexed grammar """
        l_rules = []
        rules = Rules(l_rules)
        indexed_grammar = IndexedGrammar(rules)
        fst = FST()
        intersection = fst & indexed_grammar
        assert intersection.is_empty()

        l_rules.append(ProductionRule("S", "D", "f"))
        l_rules.append(DuplicationRule("D", "A", "B"))
        l_rules.append(ConsumptionRule("f", "A", "Afinal"))
        l_rules.append(ConsumptionRule("f", "B", "Bfinal"))
        l_rules.append(EndRule("Afinal", "a"))
        l_rules.append(EndRule("Bfinal", "b"))

        rules = Rules(l_rules)
        indexed_grammar = IndexedGrammar(rules)
        intersection = fst.intersection(indexed_grammar)
        assert intersection.is_empty()

        fst.add_start_state("q0")
        fst.add_final_state("final")
        fst.add_transition("q0", "a", "q1", ["a"])
        fst.add_transition("q1", "b", "final", ["b"])
        intersection = fst.intersection(indexed_grammar)
        assert not intersection.is_empty()

    def test_union(self, fst0, fst1):
        """ Tests the union"""
        fst_union = fst0.union(fst1)
        self._make_test_fst_union(fst_union)
        fst_union = fst0 | fst1
        self._make_test_fst_union(fst_union)

    def _make_test_fst_union(self, fst_union):
        assert len(fst_union.start_states) == 2
        assert len(fst_union.final_states) == 2
        assert fst_union.get_number_transitions() == 2
        translation = list(fst_union.translate(["a"]))
        assert translation == [["b"]]
        translation = list(fst_union.translate(["b"]))
        assert translation == [["c"]]
        translation = list(fst_union.translate(["a", "b"]))
        assert translation == []

    def test_concatenate(self, fst0, fst1):
        """ Tests the concatenation """
        fst_concatenate = fst0 + fst1
        translation = list(fst_concatenate.translate(["a", "b"]))
        assert translation == [["b", "c"]]
        translation = list(fst_concatenate.translate(["a"]))
        assert translation == []
        translation = list(fst_concatenate.translate(["b"]))
        assert translation == []

    def test_concatenate2(self, fst0, fst1):
        """ Tests the concatenation """
        fst_concatenate = fst0 + fst1 + fst1
        translation = list(fst_concatenate.translate(["a", "b", "b"]))
        assert translation == [["b", "c", "c"]]
        translation = list(fst_concatenate.translate(["a"]))
        assert translation == []
        translation = list(fst_concatenate.translate(["b"]))
        assert translation == []

    def test_kleene_start(self, fst0):
        """ Tests the kleene star on a fst"""
        fst_star = fst0.kleene_star()
        translation = list(fst_star.translate(["a"]))
        assert translation == [["b"]]
        translation = list(fst_star.translate(["a", "a"]))
        assert translation == [["b", "b"]]
        translation = list(fst_star.translate([]))
        assert translation == [[]]

    def test_generate_empty_word_from_nothing(self):
        """ Generate empty word from nothing """
        fst = FST()
        fst.add_start_state("q0")
        fst.add_transition("q0", "epsilon", "q1", [])
        fst.add_final_state("q1")
        translation = list(fst.translate([]))
        assert translation == [[]]

    def test_epsilon_loop(self):
        """ Test empty loop """
        fst = FST()
        fst.add_start_state("q0")
        fst.add_transition("q0", "epsilon", "q1", [])
        fst.add_final_state("q1")
        fst.add_transition("q1", "epsilon", "q0", [])
        translation = list(fst.translate([]))
        assert translation == [[]]

    def test_epsilon_loop2(self):
        """ Test empty loop bis """
        fst = FST()
        fst.add_start_state("q0")
        fst.add_transitions(
            [("q0", "epsilon", "q1", []),
             ("q1", "a", "q2", ["b"]),
             ("q1", "epsilon", "q0", [])])
        fst.add_final_state("q2")
        translation = list(fst.translate(["a"]))
        assert translation == [["b"]]

    def test_paper(self):
        """ Test for the paper """
        fst = FST()
        fst.add_transitions(
            [(0, "I", 1, ["Je"]), (1, "am", 2, ["suis"]),
             (2, "alone", 3, ["tout", "seul"]),
             (2, "alone", 3, ["seul"])])
        fst.add_start_state(0)
        fst.add_final_state(3)
        assert list(fst.translate(["I", "am", "alone"])) == \
            [['Je', 'suis', 'seul'],
             ['Je', 'suis', 'tout', 'seul']]
        fst = FST.from_networkx(fst.to_networkx())
        assert list(fst.translate(["I", "am", "alone"])) == \
            [['Je', 'suis', 'seul'],
             ['Je', 'suis', 'tout', 'seul']]
        fst.write_as_dot("fst.dot")
        assert path.exists("fst.dot")
