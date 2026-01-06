"""
A representation of a duplication rule, i.e. a rule that duplicates the stack
"""

from typing import Any, Iterable, AbstractSet, Tuple

from .reduced_rule import ReducedRule


class DuplicationRule(ReducedRule):
    """Represents a duplication rule, i.e. a rule of the form:
        A[sigma] -> B[sigma] C[sigma]

    Parameters
    ----------
    left_term : any
        The non-terminal on the left of the rule (A here)
    right_term0 : any
        The first non-terminal on the right of the rule (B here)
    right_term1 : any
        The second non-terminal on the right of the rule (C here)
    """

    @property
    def production(self):
        raise NotImplementedError

    @property
    def right_term(self):
        raise NotImplementedError

    @property
    def f_parameter(self):
        raise NotImplementedError

    def __init__(self, left_term, right_term0, right_term1):
        self._left_term = left_term
        self._right_terms = (right_term0, right_term1)

    def is_duplication(self) -> bool:
        """Whether the rule is a duplication rule or not

        Returns
        ----------
        is_duplication : bool
            Whether the rule is a duplication rule or not
        """
        return True

    @property
    def right_terms(self) -> Tuple[Any, Any]:
        """Gives the non-terminals on the right of the rule

        Returns
        ---------
        right_terms : iterable of any
            The right terms of the rule
        """
        return self._right_terms

    @property
    def left_term(self) -> Any:
        """Gives the non-terminal on the left of the rule

        Returns
        ---------
        left_term : any
            The left term of the rule
        """
        return self._left_term

    @property
    def non_terminals(self) -> Iterable[Any]:
        """Gives the set of non-terminals used in this rule

        Returns
        ---------
        non_terminals : iterable of any
            The non terminals used in this rule
        """
        return [self._left_term, self._right_terms[0], self._right_terms[1]]

    @property
    def terminals(self) -> AbstractSet[Any]:
        """Gets the terminals used in the rule

        Returns
        ----------
        terminals : set of any
            The terminals used in this rule
        """
        return set()

    def __repr__(self):
        """Gives a string representation of the rule, ignoring the sigmas"""
        return self._left_term + " -> " + self._right_terms[0] + \
            " " + self._right_terms[1]

    def __eq__(self, other):
        return other.is_duplication() and other.left_term == \
               self._left_term and other.right_terms == self.right_terms
