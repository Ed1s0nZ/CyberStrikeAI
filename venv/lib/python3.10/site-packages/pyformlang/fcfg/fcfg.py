"""Feature Context-Free Grammar"""
import string
from typing import Iterable, AbstractSet, Union

from pyformlang.cfg import CFG, Terminal, Epsilon, Variable
from pyformlang.cfg.cfg import is_special_text, EPSILON_SYMBOLS, NotParsableException
from pyformlang.cfg.parse_tree import ParseTree
from pyformlang.cfg.utils import to_terminal
from pyformlang.fcfg.feature_production import FeatureProduction
from pyformlang.fcfg.feature_structure import FeatureStructure, FeatureStructuresNotCompatibleException
from pyformlang.fcfg.state import State, StateProcessed


class FCFG(CFG):
    """ A class representing a feature context-free grammar

    Parameters
    ----------
    variables : set of :class:`~pyformlang.cfg.Variable`, optional
        The variables of the FCFG
    terminals : set of :class:`~pyformlang.cfg.Terminal`, optional
        The terminals of the FCFG
    start_symbol : :class:`~pyformlang.cfg.Variable`, optional
        The start symbol
    productions : set of :class:`~pyformlang.fcfg.FeatureProduction`, optional
        The feature productions or rules of the FCFG

    Examples
    --------

    Creation of a FCFG from a textual description.

    >>> fcfg = FCFG.from_text(\"\"\"
    >>>          S -> NP[AGREEMENT=?a] VP[AGREEMENT=?a]
    >>>          S -> Aux[AGREEMENT=?a] NP[AGREEMENT=?a] VP
    >>>          NP[AGREEMENT=?a] -> Det[AGREEMENT=?a] Nominal[AGREEMENT=?a]
    >>>          Aux[AGREEMENT=[NUMBER=pl, PERSON=3rd]] -> do
    >>>          Aux[AGREEMENT=[NUMBER=sg, PERSON=3rd]] -> does
    >>>          Det[AGREEMENT=[NUMBER=sg]] -> this
    >>>          Det[AGREEMENT=[NUMBER=pl]] -> these
    >>>          "VAR:VP[AGREEMENT=?a]" -> Verb[AGREEMENT=?a]
    >>>          Verb[AGREEMENT=[NUMBER=pl]] -> serve
    >>>          Verb[AGREEMENT=[NUMBER=sg, PERSON=3rd]] -> "TER:serves"
    >>>          Noun[AGREEMENT=[NUMBER=sg]] -> flight
    >>>          Noun[AGREEMENT=[NUMBER=pl]] -> flights
    >>>          Nominal[AGREEMENT=?a] -> Noun[AGREEMENT=?a]
    >>>     \"\"\")

    Check if a word is in the FCFG

    >>> fcfg.contains(["this", "flight", "serves"])

    True

    """

    def __init__(self,
                 variables: AbstractSet[Variable] = None,
                 terminals: AbstractSet[Terminal] = None,
                 start_symbol: Variable = None,
                 productions: Iterable[FeatureProduction] = None):
        super().__init__(variables, terminals, start_symbol, productions)

    def __predictor(self, state, chart, processed):
        # We have an incomplete state and the next token is a variable
        # We must ask to process the variable with another rule
        end_idx = state.positions[1]
        next_var = state.production.body[state.positions[2]]
        for production in self.productions:
            if production.head == next_var:
                new_state = State(production, (end_idx, end_idx, 0),
                                  production.features, ParseTree(production.head))
                if processed.add(end_idx, new_state):
                    chart[end_idx].append(new_state)

    def contains(self, word: Iterable[Union[Terminal, str]]) -> bool:
        """ Gives the membership of a word to the grammar

        Parameters
        ----------
        word : iterable of :class:`~pyformlang.cfg.Terminal`
            The word to check

        Returns
        ----------
        contains : bool
            Whether word if in the FCFG or not
        """
        return self._get_final_state(word) is not None

    def get_parse_tree(self, word: Iterable[Union[Terminal, str]]) -> ParseTree:
        """ Gives the parse tree for a sentence, if possible

        Parameters
        ----------
        word : iterable of :class:`~pyformlang.cfg.Terminal`
            The word to check

        Returns
        ----------
        parse_tree : :class:`~pyformlang.cfg.ParseTree`
            The parse tree

        Raises
        ------
        NotParsableException
            When the word is not parsable.
        """
        final_state = self._get_final_state(word)
        if final_state is None:
            raise NotParsableException()
        return final_state.parse_tree

    def _get_final_state(self, word: Iterable[Terminal]):
        word = [to_terminal(x) for x in word if x != Epsilon()]
        chart = [[] for _ in range(len(word) + 1)]
        # Processed[i] contains all production rule that are currently working until i.
        processed = StateProcessed(len(word) + 1)
        gamma = Variable("Gamma")
        dummy_rule = FeatureProduction(gamma, [self.start_symbol], FeatureStructure(), [FeatureStructure()])
        # State = (rule, [begin, end, dot position, diag)
        first_state = State(dummy_rule, (0, 0, 0), dummy_rule.features, ParseTree("BEGIN"))
        chart[0].append(first_state)
        processed.add(0, first_state)
        for i in range(len(chart) - 1):
            while chart[i]:
                state = chart[i].pop()
                if state.is_incomplete() and state.next_is_variable():
                    self.__predictor(state, chart, processed)
                elif state.is_incomplete():
                    if state.next_is_word(word[i]):
                        _scanner(state, chart, processed)
                else:
                    _completer(state, chart, processed)
        while chart[len(chart) - 1]:
            state = chart[len(chart) - 1].pop()
            if not state.is_incomplete():
                _completer(state, chart, processed)
        for state in processed.generator(len(word)):
            if state.positions[0] == 0 and not state.is_incomplete() and state.production.head == self.start_symbol:
                return state
        return None

    @classmethod
    def _read_line(cls, line, productions, terminals, variables):
        structure_variables = {}
        head_s, body_s = line.split("->")
        head_text = head_s.strip()
        if is_special_text(head_text):
            head_text = head_text[5:-1]
        head_text, head_conditions = _split_text_conditions(head_text)
        head_fs = FeatureStructure.from_text(head_conditions, structure_variables)
        head = Variable(head_text)
        variables.add(head)
        all_body_fs = []
        for sub_body in body_s.split("|"):
            body = []
            for body_component in sub_body.split():
                if is_special_text(body_component):
                    type_component = body_component[1:4]
                    body_component = body_component[5:-1]
                else:
                    type_component = ""
                if body_component[0] in string.ascii_uppercase or \
                        type_component == "VAR":
                    body_component, body_conditions = _split_text_conditions(body_component)
                    body_fs = FeatureStructure.from_text(body_conditions, structure_variables)
                    all_body_fs.append(body_fs)
                    body_var = Variable(body_component)
                    variables.add(body_var)
                    body.append(body_var)
                elif body_component not in EPSILON_SYMBOLS or type_component \
                        == "TER":
                    body_ter = Terminal(body_component)
                    terminals.add(body_ter)
                    body.append(body_ter)
                    all_body_fs.append(FeatureStructure())
            production = FeatureProduction(head, body, head_fs, all_body_fs)
            productions.add(production)


def _split_text_conditions(head_text):
    if head_text[-1] != "]":
        return head_text, ""
    idx = head_text.find("[")
    if idx == -1:
        return head_text, ""
    return head_text[:idx], head_text[idx+1:-1]


def _scanner(state, chart, processed):
    # We have an incomplete state and the next token is the word given as input
    # We move the end token and the dot token by one.
    end_idx = state.positions[1]
    state.parse_tree.sons.append(ParseTree(state.production.body[state.positions[2]]))
    new_state = State(state.production, (state.positions[0], end_idx + 1, state.positions[2] + 1),
                      state.feature_stucture, state.parse_tree)
    if processed.add(end_idx + 1, new_state):
        chart[end_idx + 1].append(new_state)


def _completer(state, chart, processed):
    # We have a complete state. We must check if it helps to move another state forward.
    begin_idx = state.positions[0]
    head = state.production.head
    for next_state in processed.generator(begin_idx):
        # next_state[1][1] == begin_idx always true
        if next_state.is_incomplete() and next_state.production.body[next_state.positions[2]] == head:
            try:
                copy_left = state.feature_stucture.copy()
                copy_left = copy_left.get_feature_by_path(["head"])
                copy_right = next_state.feature_stucture.copy()
                copy_right_considered = copy_right.get_feature_by_path([str(next_state.positions[2])])
                copy_right_considered.unify(copy_left)
            except FeatureStructuresNotCompatibleException:
                continue
            parse_tree = next_state.parse_tree
            parse_tree.sons.append(state.parse_tree)
            new_state = State(next_state.production,
                              (next_state.positions[0], state.positions[1], next_state.positions[2] + 1),
                              copy_right, parse_tree)
            if processed.add(state.positions[1], new_state):
                chart[state.positions[1]].append(new_state)
