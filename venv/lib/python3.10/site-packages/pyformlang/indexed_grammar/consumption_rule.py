"""
Representation of a consumption rule, i.e. a rule that consumes something on \
the stack
"""

from typing import Any, Iterable, AbstractSet

from .reduced_rule import ReducedRule


class ConsumptionRule(ReducedRule):
    """ Contains a representation of a consumption rule, i.e. a rule of the \
    form:
            C[ r sigma] -> B[sigma]

    Parameters
    ----------
    f_param : any
        The consumed symbol
    left : any
        The non terminal on the left (here C)
    right : any
        The non terminal on the right (here B)
    """

    @property
    def right_term(self):
        raise NotImplementedError

    @property
    def right_terms(self):
        raise NotImplementedError

    def __init__(self, f_param: Any, left: Any, right: Any):
        self._f = f_param
        self._right = right
        self._left_term = left

    def is_consumption(self) -> bool:
        """Whether the rule is a consumption rule or not

        Returns
        ----------
        is_consumption : bool
            Whether the rule is a consumption rule or not
        """
        return True

    @property
    def f_parameter(self) -> Any:
        """Gets the symbol which is consumed

        Returns
        ----------
        f : any
            The symbol being consumed by the rule
        """
        return self._f

    @property
    def production(self):
        raise NotImplementedError

    @property
    def right(self) -> Any:
        """Gets the symbole on the right of the rule

        right : any
            The right symbol
        """
        return self._right

    @property
    def left_term(self) -> Any:
        """Gets the symbol on the left of the rule

        left : any
            The left symbol of the rule
        """
        return self._left_term

    @property
    def non_terminals(self) -> Iterable[Any]:
        """Gets the non-terminals used in the rule

        non_terminals : iterable of any
            The non_terminals used in the rule
        """
        return [self._left_term, self._right]

    @property
    def terminals(self) -> AbstractSet[Any]:
        """Gets the terminals used in the rule

        terminals : set of any
            The terminals used in the rule
        """
        return {self._f}

    def __repr__(self):
        return self._left_term + " [ " + self._f + " ] -> " + self._right

    def __eq__(self, other):
        return other.is_consumption() and other.left_term == \
               self.left_term and other.right == self.right and \
               other.f_parameter() == self.f_parameter
