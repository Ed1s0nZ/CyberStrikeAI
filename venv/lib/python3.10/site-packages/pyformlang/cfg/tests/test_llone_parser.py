"""
Test for LL(1) parser
"""

from os import path

import pytest

from pyformlang.cfg import CFG, Variable, Terminal, Epsilon
from pyformlang.cfg.llone_parser import LLOneParser
from pyformlang.cfg.tests.test_cfg import get_example_text_duplicate
from pyformlang.regular_expression import Regex


class TestLLOneParser:
    """ Tests the LL(1) Parser """

    # pylint: disable=missing-function-docstring, too-many-public-methods

    def test_get_first_set(self):
        # Example from:
        # https://www.geeksforgeeks.org/first-set-in-syntax-analysis/
        text = get_example_text_duplicate()
        cfg = CFG.from_text(text)
        llone_parser = LLOneParser(cfg)
        first_set = llone_parser.get_first_set()
        assert first_set[Variable("E")] == \
                         {Terminal("("), Terminal("id")}
        assert first_set[Variable("E’")] == \
                         {Terminal("+"), Epsilon()}
        assert first_set[Variable("T")] == \
                         {Terminal("("), Terminal("id")}
        assert first_set[Variable("T’")] == \
                         {Terminal("*"), Epsilon()}
        assert first_set[Variable("F")] == \
                         {Terminal("("), Terminal("id")}

    def test_get_first_set2(self):
        # Example from:
        # https://www.geeksforgeeks.org/first-set-in-syntax-analysis/
        text = """
            S -> A C B | C b b | B a
            A -> d a | B C
            B -> g | Є
            C -> h | Є
        """
        cfg = CFG.from_text(text)
        llone_parser = LLOneParser(cfg)
        first_set = llone_parser.get_first_set()
        assert first_set[Variable("S")] == \
                         {Terminal(x) for x in ("d", "g", "h", "b",
                                                "a")}.union({Epsilon()})
        assert first_set[Variable("A")] == \
                         {Terminal(x) for x in ("d", "g",
                                                "h")}.union({Epsilon()})
        assert first_set[Variable("B")] == \
                         {Terminal(x) for x in ["g"]}.union({Epsilon()})
        assert first_set[Variable("C")] == \
                         {Terminal(x) for x in ["h"]}.union({Epsilon()})

    def test_get_follow_set(self):
        # Example from:
        # https://www.geeksforgeeks.org/follow-set-in-syntax-analysis/
        text = get_example_text_duplicate()
        cfg = CFG.from_text(text, start_symbol="E")
        llone_parser = LLOneParser(cfg)
        follow_set = llone_parser.get_follow_set()
        assert follow_set[Variable("E")] == \
                         {"$", Terminal(")")}
        assert follow_set[Variable("E’")] == \
                         {"$", Terminal(")")}
        assert follow_set[Variable("T")] == \
                         {"$", Terminal("+"), Terminal(")")}
        assert follow_set[Variable("T’")] == \
                         {"$", Terminal("+"), Terminal(")")}
        assert follow_set[Variable("F")] == \
                         {"$", Terminal("+"), Terminal("*"), Terminal(")")}

    def test_get_follow_set2(self):
        # Example from:
        # https://www.geeksforgeeks.org/follow-set-in-syntax-analysis/
        text = """
            S -> A C B | C b b | B a
            A -> d a | B C
            B -> g | Є
            C -> h | Є
        """
        cfg = CFG.from_text(text)
        llone_parser = LLOneParser(cfg)
        follow_set = llone_parser.get_follow_set()
        assert follow_set["S"] == \
                         {"$"}
        assert follow_set["A"] == \
                         {"$", Terminal("h"), Terminal("g")}
        assert follow_set["B"] == \
                         {"$", Terminal("h"), Terminal("g"), Terminal("a")}
        assert follow_set["C"] == \
                         {"$", Terminal("h"), Terminal("g"), Terminal("b")}

    def test_get_llone_table(self):
        # Example from:
        # https://www.geeksforgeeks.org/construction-of-ll1-parsing-table/
        text = get_example_text_duplicate()
        cfg = CFG.from_text(text, start_symbol="E")
        llone_parser = LLOneParser(cfg)
        parsing_table = llone_parser.get_llone_parsing_table()
        assert len(parsing_table.get(Variable("E"), {})
                .get(Terminal("id"), [])) == \
            1
        assert len(parsing_table.get(Variable("E"), {})
                .get(Terminal("+"), [])) == \
            0
        assert len(parsing_table.get(Variable("T’"), {})
                .get(Terminal(")"), [])) == \
            1
        assert len(parsing_table.get(Variable("F"), {})
                .get(Terminal("("), [])) == \
            1
        assert len(parsing_table.get(Variable("F"), {})
                .get(Terminal("id"), [])) == \
            1

    def test_llone_table_non_llone(self):
        text = """
        S -> A | a
        A -> a
        """
        cfg = CFG.from_text(text, start_symbol="E")
        llone_parser = LLOneParser(cfg)
        parsing_table = llone_parser.get_llone_parsing_table()
        assert len(parsing_table.get(Variable("S"), {})
                .get(Terminal("a"), [])) == \
            2
        assert len(parsing_table.get(Variable("A"), {})
                .get(Terminal("a"), [])) == \
            1
        assert len(parsing_table.get(Variable("S"), {})
                .get(Terminal("$"), [])) == \
            0
        assert len(parsing_table.get(Variable("A"), {})
                .get(Terminal("$"), [])) == \
            0

    def test_is_llone_parsable(self):
        # Example from:
        # https://www.geeksforgeeks.org/construction-of-ll1-parsing-table/
        text = get_example_text_duplicate()
        cfg = CFG.from_text(text, start_symbol="E")
        llone_parser = LLOneParser(cfg)
        assert llone_parser.is_llone_parsable()

    def test_is_not_llone_parsable(self):
        # Example from:
        # https://www.geeksforgeeks.org/construction-of-ll1-parsing-table/
        text = """
                S -> A | a
                A -> a
                """
        cfg = CFG.from_text(text, start_symbol="E")
        llone_parser = LLOneParser(cfg)
        assert not llone_parser.is_llone_parsable()

    def test_get_llone_parse_tree(self):
        text = get_example_text_duplicate()
        cfg = CFG.from_text(text, start_symbol="E")
        llone_parser = LLOneParser(cfg)
        parse_tree = llone_parser.get_llone_parse_tree(["id", "+", "id",
                                                        "*", "id"])
        assert parse_tree.value == Variable("E")
        assert len(parse_tree.sons) == 2

    def test_get_llone_leftmost_derivation(self):
        text = get_example_text_duplicate()
        cfg = CFG.from_text(text, start_symbol="E")
        llone_parser = LLOneParser(cfg)
        parse_tree = llone_parser.get_llone_parse_tree(["id", "+", "id",
                                                        "*", "id"])
        assert parse_tree.get_leftmost_derivation() == \
            [[Variable("E")],
             [Variable("T"), Variable("E’")],
             [Variable("F"), Variable("T’"), Variable("E’")],
             [Terminal("id"), Variable("T’"), Variable("E’")],
             [Terminal("id"), Variable("E’")],
             [Terminal("id"), Terminal("+"), Variable("T"), Variable("E’")],
             [Terminal("id"), Terminal("+"), Variable("F"), Variable("T’"),
              Variable("E’")],
             [Terminal("id"), Terminal("+"), Terminal("id"), Variable("T’"),
              Variable("E’")],
             [Terminal("id"), Terminal("+"), Terminal("id"), Terminal("*"),
              Variable("F"), Variable("T’"), Variable("E’")],
             [Terminal("id"), Terminal("+"), Terminal("id"), Terminal("*"),
              Terminal("id"), Variable("T’"), Variable("E’")],
             [Terminal("id"), Terminal("+"), Terminal("id"), Terminal("*"),
              Terminal("id"), Variable("E’")],
             [Terminal("id"), Terminal("+"), Terminal("id"), Terminal("*"),
              Terminal("id")]
             ]

    def test_get_llone_rightmost_derivation(self):
        text = get_example_text_duplicate()
        cfg = CFG.from_text(text, start_symbol="E")
        llone_parser = LLOneParser(cfg)
        parse_tree = llone_parser.get_llone_parse_tree(["id", "+", "id",
                                                        "*", "id"])
        assert parse_tree.get_rightmost_derivation() == \
            [[Variable("E")],
             [Variable("T"), Variable("E’")],
             [Variable("T"), Terminal("+"), Variable("T"), Variable("E’")],
             [Variable("T"), Terminal("+"), Variable("T")],
             [Variable("T"), Terminal("+"), Variable("F"), Variable("T’")],
             [Variable("T"), Terminal("+"), Variable("F"), Terminal("*"),
              Variable("F"), Variable("T’")],
             [Variable("T"), Terminal("+"), Variable("F"), Terminal("*"),
              Variable("F")],
             [Variable("T"), Terminal("+"), Variable("F"), Terminal("*"),
              Terminal("id")],
             [Variable("T"), Terminal("+"), Terminal("id"), Terminal("*"),
              Terminal("id")],
             [Variable("F"), Variable("T’"), Terminal("+"), Terminal("id"),
              Terminal("*"), Terminal("id")],
             [Variable("F"), Terminal("+"), Terminal("id"),
              Terminal("*"), Terminal("id")],
             [Terminal("id"), Terminal("+"), Terminal("id"),
              Terminal("*"), Terminal("id")],
             ]

    def test_save_tree(self):
        text = """
                    E  -> T E'
                    E' -> + T E' | epsilon
                    T  -> F T'
                    T' -> * F T' | epsilon
                    F  -> ( E ) | id
                """
        cfg = CFG.from_text(text, start_symbol="E")
        llone_parser = LLOneParser(cfg)
        parse_tree = llone_parser.get_llone_parse_tree(["id", "+", "id",
                                                        "*", "id"])
        parse_tree.write_as_dot("parse_tree.dot")
        assert path.exists("parse_tree.dot")

    def test_sentence_cfg(self):
        cfg = CFG.from_text("""
            S -> NP VP PUNC
            PUNC -> . | !
            VP -> V NP
            V -> buys | touches | sees
            NP -> georges | jacques | léo | Det N
            Det -> a | an | the
            N -> gorilla | sky | carrots
        """)
        regex = Regex("georges touches (a|an) (sky|gorilla) !")
        cfg_inter = cfg.intersection(regex)
        assert not cfg_inter.is_empty()
        assert cfg_inter.is_finite()
        assert not cfg_inter.contains(["georges", "sees", "a", "gorilla", "."])
        assert cfg_inter.contains(["georges", "touches", "a", "gorilla", "!"])
        assert not cfg_inter.is_normal_form()
        cnf = cfg.to_normal_form()
        assert cnf.is_normal_form()
        llone_parser = LLOneParser(cfg)
        parse_tree = llone_parser.get_llone_parse_tree(["georges", "sees",
                                                        "a", "gorilla", "."])
        assert parse_tree.get_leftmost_derivation() == \
            [[Variable("S")],
             [Variable("NP"), Variable("VP"), Variable("PUNC")],
             [Terminal("georges"), Variable("VP"), Variable("PUNC")],
             [Terminal("georges"), Variable("V"), Variable("NP"),
              Variable("PUNC")],
             [Terminal("georges"), Terminal("sees"),
              Variable("NP"), Variable("PUNC")],
             [Terminal("georges"), Terminal("sees"), Variable("Det"),
              Variable("N"), Variable("PUNC")],
             [Terminal("georges"), Terminal("sees"), Terminal("a"),
              Variable("N"), Variable("PUNC")],
             [Terminal("georges"), Terminal("sees"), Terminal("a"),
              Terminal("gorilla"), Variable("PUNC")],
             [Terminal("georges"), Terminal("sees"), Terminal("a"),
              Terminal("gorilla"), Terminal(".")]]
        parse_tree.write_as_dot("parse_tree.dot")


if __name__ == '__main__':
    pytest.main()
