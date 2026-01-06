"""Test a FCFG"""

from pyformlang.cfg import Variable, Terminal
from pyformlang.cfg.cfg import NotParsableException
from pyformlang.cfg.parse_tree import ParseTree
from pyformlang.fcfg.fcfg import FCFG
from pyformlang.fcfg.feature_production import FeatureProduction
from pyformlang.fcfg.feature_structure import FeatureStructure
from pyformlang.fcfg.state import State, StateProcessed
import pytest


class TestFCFG:
    """Test a FCFG"""

    def test_contains(self):
        """Test containment"""
        # 1st: S -> NP VP
        agreement = FeatureStructure()
        np_feat = FeatureStructure()
        np_feat.add_content("AGREEMENT", agreement)
        vp_feat = FeatureStructure()
        vp_feat.add_content("AGREEMENT", agreement)
        fp1 = FeatureProduction(Variable("S"),
                                [Variable("NP"), Variable("VP")],
                                FeatureStructure(),
                                [np_feat, vp_feat])
        # Second: S -> Aux NP VP
        agreement = FeatureStructure()
        aux_feat = FeatureStructure()
        aux_feat.add_content("AGREEMENT", agreement)
        np_feat = FeatureStructure()
        np_feat.add_content("AGREEMENT", agreement)
        fp2 = FeatureProduction(Variable("S"),
                                [Variable("Aux"), Variable("NP"), Variable("VP")],
                                FeatureStructure(),
                                [aux_feat, np_feat, FeatureStructure()])
        # Third: NP -> Det Nominal
        agreement = FeatureStructure()
        det_feat = FeatureStructure()
        det_feat.add_content("AGREEMENT", agreement)
        np_feat = FeatureStructure()
        np_feat.add_content("AGREEMENT", agreement)
        nominal_feat = FeatureStructure()
        nominal_feat.add_content("AGREEMENT", agreement)
        fp3 = FeatureProduction(Variable("NP"),
                                [Variable("Det"), Variable("Nominal")],
                                np_feat,
                                [det_feat, nominal_feat])
        # Forth: Aux -> do
        agreement = FeatureStructure()
        agreement.add_content("NUMBER", FeatureStructure("pl"))
        agreement.add_content("PERSON", FeatureStructure("3rd"))
        feat = FeatureStructure()
        feat.add_content("AGREEMENT", agreement)
        fp4 = FeatureProduction(Variable("Aux"),
                                [Terminal("do")],
                                feat,
                                [FeatureStructure()])
        # 5: Aux -> does
        agreement = FeatureStructure()
        agreement.add_content("NUMBER", FeatureStructure("sg"))
        agreement.add_content("PERSON", FeatureStructure("3rd"))
        feat = FeatureStructure()
        feat.add_content("AGREEMENT", agreement)
        fp5 = FeatureProduction(Variable("Aux"),
                                [Terminal("does")],
                                feat,
                                [FeatureStructure()])
        # 6: Det -> this
        agreement = FeatureStructure()
        agreement.add_content("NUMBER", FeatureStructure("sg"))
        feat = FeatureStructure()
        feat.add_content("AGREEMENT", agreement)
        fp6 = FeatureProduction(Variable("Det"),
                                [Terminal("this")],
                                feat,
                                [FeatureStructure()])
        # 7: Det -> these
        agreement = FeatureStructure()
        agreement.add_content("NUMBER", FeatureStructure("pl"))
        feat = FeatureStructure()
        feat.add_content("AGREEMENT", agreement)
        fp7 = FeatureProduction(Variable("Det"),
                                [Terminal("these")],
                                feat,
                                [FeatureStructure()])
        # 8: VP -> Verb
        agreement = FeatureStructure()
        vp_feat = FeatureStructure()
        vp_feat.add_content("AGREEMENT", agreement)
        verb_feat = FeatureStructure()
        verb_feat.add_content("AGREEMENT", agreement)
        fp8 = FeatureProduction(Variable("VP"),
                                [Variable("Verb")],
                                vp_feat,
                                [verb_feat])
        assert "AGREEMENT" in str(fp8)
        # 9: Verb -> serve
        agreement = FeatureStructure()
        agreement.add_content("NUMBER", FeatureStructure("pl"))
        feat = FeatureStructure()
        feat.add_content("AGREEMENT", agreement)
        fp9 = FeatureProduction(Variable("Verb"),
                                [Terminal("serve")],
                                feat,
                                [FeatureStructure()])
        # 10: Verb -> serves
        agreement = FeatureStructure()
        agreement.add_content("NUMBER", FeatureStructure("sg"))
        agreement.add_content("PERSON", FeatureStructure("3rd"))
        feat = FeatureStructure()
        feat.add_content("AGREEMENT", agreement)
        fp10 = FeatureProduction(Variable("Verb"),
                                 [Terminal("serves")],
                                 feat,
                                 [FeatureStructure()])
        # 11: Noun -> flight
        agreement = FeatureStructure()
        agreement.add_content("NUMBER", FeatureStructure("sg"))
        feat = FeatureStructure()
        feat.add_content("AGREEMENT", agreement)
        fp11 = FeatureProduction(Variable("Noun"),
                                 [Terminal("flight")],
                                 feat,
                                 [FeatureStructure()])
        # 12: Noun -> flight
        agreement = FeatureStructure()
        agreement.add_content("NUMBER", FeatureStructure("pl"))
        feat = FeatureStructure()
        feat.add_content("AGREEMENT", agreement)
        fp12 = FeatureProduction(Variable("Noun"),
                                 [Terminal("flights")],
                                 feat,
                                 [FeatureStructure()])
        # 13: Nominal -> Noun
        agreement = FeatureStructure()
        nominal_feat = FeatureStructure()
        nominal_feat.add_content("AGREEMENT", agreement)
        noun_feat = FeatureStructure()
        noun_feat.add_content("AGREEMENT", agreement)
        fp13 = FeatureProduction(Variable("Nominal"),
                                 [Variable("Noun")],
                                 nominal_feat,
                                 [noun_feat])
        productions = [fp1, fp2, fp3, fp4, fp5, fp6, fp7, fp8, fp9, fp10, fp11,
                       fp12, fp13]
        fcfg = FCFG(start_symbol=Variable("S"), productions=productions)
        self._sub_tests_contains1(fcfg)

    def _sub_tests_contains1(self, fcfg):
        assert fcfg.contains(["this", "flight", "serves"])
        assert ["this", "flight", "serves"] in fcfg
        assert fcfg.contains(["these", "flights", "serve"])
        assert not fcfg.contains(["this", "flights", "serves"])
        assert not fcfg.contains(["this", "flight", "serve"])
        assert not fcfg.contains(["this", "flights", "serve"])

    def test_contains2(self):
        """Test containment"""
        # 1st: S -> NP
        number = FeatureStructure("sg")
        np_feat = FeatureStructure()
        np_feat.add_content("NUMBER", number)
        fp1 = FeatureProduction(Variable("S"),
                                [Variable("NP")],
                                FeatureStructure(),
                                [np_feat])
        # 2nd: NP -> flights
        number = FeatureStructure("pl")
        np_feat = FeatureStructure()
        np_feat.add_content("NUMBER", number)
        fp2 = FeatureProduction(Variable("NP"),
                                [Terminal("flights")],
                                np_feat,
                                [FeatureStructure()])
        assert "NUMBER" in str(fp2)
        fcfg = FCFG(start_symbol=Variable("S"), productions=[fp1, fp2])
        assert not fcfg.contains(["flights"])

    def test_state(self):
        """Test functions on states"""
        fs1 = FeatureStructure()
        fs1.add_content("NUMBER", FeatureStructure("sg"))
        state0 = State(FeatureProduction(Variable("S"), [], fs1, []), (0, 0, 0), fs1, ParseTree("S"))
        processed = StateProcessed(1)
        state1 = State(FeatureProduction(Variable("S"), [], fs1, []), (0, 0, 0), fs1, ParseTree("S"))
        assert processed.add(0, state0)
        assert not processed.add(0, state1)

    def test_from_text(self):
        """Test containment from a text description"""
        fcfg = FCFG.from_text("""
             S -> NP[AGREEMENT=?a] VP[AGREEMENT=?a]
             S -> Aux[AGREEMENT=?a] NP[AGREEMENT=?a] VP
             NP[AGREEMENT=?a] -> Det[AGREEMENT=?a] Nominal[AGREEMENT=?a]
             Aux[AGREEMENT=[NUMBER=pl, PERSON=3rd]] -> do
             Aux[AGREEMENT=[NUMBER=sg, PERSON=3rd]] -> does
             Det[AGREEMENT=[NUMBER=sg]] -> this
             Det[AGREEMENT=[NUMBER=pl]] -> these
             "VAR:VP[AGREEMENT=?a]" -> Verb[AGREEMENT=?a]
             Verb[AGREEMENT=[NUMBER=pl]] -> serve
             Verb[AGREEMENT=[NUMBER=sg, PERSON=3rd]] -> "TER:serves"
             Noun[AGREEMENT=[NUMBER=sg]] -> flight
             Noun[AGREEMENT=[NUMBER=pl]] -> flights
             Nominal[AGREEMENT=?a] -> Noun[AGREEMENT=?a]
        """)
        self._sub_tests_contains1(fcfg)
        parse_tree = fcfg.get_parse_tree(["this", "flight", "serves"])
        with pytest.raises(NotParsableException):
            fcfg.get_parse_tree(["these", "flight", "serves"])
        assert "Det" in str(parse_tree)
