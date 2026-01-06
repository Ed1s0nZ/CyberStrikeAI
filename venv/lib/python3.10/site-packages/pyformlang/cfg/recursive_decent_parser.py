"""
A recursive decent parser.
"""

from pyformlang.cfg import Variable, Epsilon
from pyformlang.cfg.cfg import NotParsableException
from pyformlang.cfg.parse_tree import ParseTree
from pyformlang.cfg.utils import to_terminal


def _get_index_to_extend(current_expansion, left):
    order = enumerate(current_expansion)
    if not left:
        order = reversed(list(order))
    for i, symbol in order:
        if isinstance(symbol[0], Variable):
            return i, symbol
    return -1, None


class RecursiveDecentParser:
    """
        A recursive Top-Down parser

        Parameters
        ----------
        cfg : :class:`~pyformlang.cfg.CFG`
            A context-free Grammar

    """

    def __init__(self, cfg):
        self._cfg = cfg

    def get_parse_tree(self, word, left=True):
        """
            Get a parse tree for a given word

            Parameters
            ----------
            word : list
                The word to parse
            left
                If we do the recursive from the left or the right(left by \
                default)

            Returns
            -------
            parse_tree : :class:`~pyformlang.cfg.ParseTree`
                The parse tree

            Raises
            --------
            NotParsableException
                When the word cannot be parsed

        """
        word = [to_terminal(x) for x in word if x != Epsilon()]
        parse_tree = ParseTree(self._cfg.start_symbol)
        starting_expansion = [(self._cfg.start_symbol, parse_tree)]
        if self._get_parse_tree_sub(word, starting_expansion, left):
            return parse_tree
        raise NotParsableException

    def _match(self, word, current_expansion, idx_word=0,
               idx_current_expansion=0):
        if idx_word == len(word) and \
                idx_current_expansion == len(current_expansion):
            return True
        if idx_word > len(word):
            return False
        if idx_current_expansion == len(current_expansion):
            return False
        if isinstance(current_expansion[idx_current_expansion][0], Variable):
            return self._match(word, current_expansion, idx_word + 1,
                               idx_current_expansion) or \
                   self._match(word, current_expansion, idx_word,
                               idx_current_expansion + 1)
        if idx_word < len(word) and \
                word[idx_word] == current_expansion[idx_current_expansion][0]:
            return self._match(word, current_expansion, idx_word + 1,
                               idx_current_expansion + 1)
        return False

    def _get_parse_tree_sub(self, word, current_expansion, left=True):
        if not self._match(word, current_expansion):
            return False
        extend_idx, to_expand = _get_index_to_extend(current_expansion, left)
        # Nothing else to extend, we are done
        if to_expand is None:
            return True
        begin = current_expansion[:extend_idx]
        end = current_expansion[extend_idx + 1:]
        for production in self._cfg.productions:
            if production.head == to_expand[0]:
                replacement = [(x, ParseTree(x)) for x in production.body]
                new_expansion = begin + replacement + end
                if self._get_parse_tree_sub(word, new_expansion, left):
                    to_expand[1].sons = [x[1] for x in replacement]
                    return True
        return False

    def is_parsable(self, word, left=True):
        """
        Whether a word is parsable or not

        Parameters
        ----------
        word : list
                The word to parse
        left
            If we do the recursive from the left or the right(left by \
            default)

        Returns
        -------
        is_parsable : bool
            If the word is parsable

        Raises
        --------
        NotParsableException
            When the word cannot be parsed
        RecursionError
            If the recursion goes too deep. This error occurs because some \
            the algorithm is not guaranteed to terminate with left/right \
            recursive grammars.

        """
        try:
            self.get_parse_tree(word, left)
        except NotParsableException:
            return False
        return True
