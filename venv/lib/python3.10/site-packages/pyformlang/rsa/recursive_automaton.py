"""
Representation of a recursive automaton
"""

from typing import AbstractSet, Union

from pyformlang.finite_automaton.finite_automaton import to_symbol
from pyformlang.finite_automaton.symbol import Symbol
from pyformlang.regular_expression import Regex
from pyformlang.cfg import Epsilon

from pyformlang.rsa.box import Box


class RecursiveAutomaton:
    """ Represents a recursive automaton

    This class represents a recursive automaton.

    Parameters
    ----------
    start_box : :class:`~pyformlang.rsa.Box`
        Start box
    boxes : set of :class:`~pyformlang.rsa.Box`
        A finite set of boxes

    """

    def __init__(self,
                 start_box: Box,
                 boxes: AbstractSet[Box]):
        self._nonterminal_to_box = {}
        if start_box not in boxes:
            self._nonterminal_to_box[to_symbol(start_box.nonterminal)] = start_box
        self._start_nonterminal = to_symbol(start_box.nonterminal)
        for box in boxes:
            self._nonterminal_to_box[to_symbol(box.nonterminal)] = box

    def get_box_by_nonterminal(self, nonterminal: Union[Symbol, str]):
        """
        Box by nonterminal

        Parameters
        ----------
        nonterminal: :class:`~pyformlang.finite_automaton.Symbol` | str
            the nonterminal of which represents a box

        Returns
        -----------
        box : :class:`~pyformlang.rsa.Box` | None
            box represented by given nonterminal
        """

        nonterminal = to_symbol(nonterminal)
        if nonterminal in self._nonterminal_to_box:
            return self._nonterminal_to_box[nonterminal]

        return None

    def get_number_boxes(self):
        """ Size of set of boxes """

        return len(self._nonterminal_to_box)

    def to_dot(self):
        """ Create dot representation of recursive automaton """
        dot_string = 'digraph "" {'
        for box in self._nonterminal_to_box.values():
            dot_string += f'\n{box.to_subgraph_dot()}'
        dot_string += "\n}"
        return dot_string

    @property
    def nonterminals(self) -> set:
        """ The set of nonterminals """

        return set(self._nonterminal_to_box.keys())

    @property
    def boxes(self) -> dict:
        """ The set of boxes """

        return self._nonterminal_to_box

    @property
    def start_nonterminal(self) -> Symbol:
        """ The start nonterminal """

        return self._start_nonterminal

    @property
    def start_box(self):
        """ The start box """

        return self.boxes[self.start_nonterminal]

    @classmethod
    def from_regex(cls, regex: Regex, start_nonterminal: Union[Symbol, str]):
        """ Create a recursive automaton from regular expression

        Parameters
        -----------
        regex : :class:`~pyformlang.regular_expression.Regex`
            The regular expression
        start_nonterminal : :class:`~pyformlang.finite_automaton.Symbol` | str
            The start nonterminal for the recursive automaton

        Returns
        -----------
        rsa : :class:`~pyformlang.rsa.RecursiveAutomaton`
            The new recursive automaton built from regular expression
        """
        start_nonterminal = to_symbol(start_nonterminal)
        box = Box(regex.to_epsilon_nfa().minimize(), start_nonterminal)
        return RecursiveAutomaton(box, {box})

    @classmethod
    def from_ebnf(cls, text, start_nonterminal: Union[Symbol, str] = Symbol("S")):
        """ Create a recursive automaton from ebnf (ebnf = Extended Backus-Naur Form)

        Parameters
        -----------
        text : str
            The text of transform
        start_nonterminal : :class:`~pyformlang.finite_automaton.Symbol` | str, optional
            The start nonterminal, S by default

        Returns
        -----------
        rsa : :class:`~pyformlang.rsa.RecursiveAutomaton`
            The new recursive automaton built from context-free grammar
        """
        start_nonterminal = to_symbol(start_nonterminal)
        productions = {}
        boxes = set()
        nonterminals = set()
        for production in text.splitlines():
            production = production.strip()
            if "->" not in production:
                continue

            head, body = production.split("->")
            head = head.strip()
            body = body.strip()
            nonterminals.add(to_symbol(head))

            if body == "":
                body = Epsilon().to_text()

            if head in productions:
                productions[head] += " | " + body
            else:
                productions[head] = body

        for head, body in productions.items():
            boxes.add(Box(Regex(body).to_epsilon_nfa().minimize(),
                          to_symbol(head)))
        start_box = Box(Regex(productions[start_nonterminal.value]).to_epsilon_nfa().minimize(), start_nonterminal)
        return RecursiveAutomaton(start_box, boxes)

    def is_equals_to(self, other):
        """
        Check whether two recursive automata are equals by boxes.
        Not equivalency in terms of formal languages theory, just mapping boxes

        Parameters
        ----------
        other : :class:`~pyformlang.rsa.RecursiveAutomaton`
            The input recursive automaton

        Returns
        ----------
        are_equivalent : bool
            Whether the two recursive automata are equals or not
        """
        if not isinstance(other, RecursiveAutomaton):
            return False
        return self.boxes == other.boxes

    def __eq__(self, other):
        return self.is_equals_to(other)
