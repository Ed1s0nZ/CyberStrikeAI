"""
Representation of a reduced rule
"""
from abc import abstractmethod


class ReducedRule:
    """Representation of all possible reduced forms.
    They can be of four types :
        * Consumption
        * Production
        * End
        * Duplication
    """

    def is_consumption(self) -> bool:
        """Whether the rule is a consumption rule or not

        Returns
        ----------
        is_consumption : bool
            Whether the rule is a consumption rule or not
        """
        return False

    def is_duplication(self) -> bool:
        """Whether the rule is a duplication rule or not

        Returns
        ----------
        is_duplication : bool
            Whether the rule is a duplication rule or not
        """
        return False

    def is_production(self) -> bool:
        """Whether the rule is a production rule or not

        Returns
        ----------
        is_production : bool
            Whether the rule is a production rule or not
        """
        return False

    def is_end_rule(self) -> bool:
        """Whether the rule is an end rule or not

        Returns
        ----------
        is_end : bool
            Whether the rule is an end rule or not
        """
        return False

    @property
    @abstractmethod
    def f_parameter(self):
        """The f parameter

        Returns
        ----------
        f : any
            The f parameter
        """
        raise NotImplementedError

    @property
    @abstractmethod
    def left_term(self):
        """The left term

        Returns
        ----------
        left_term : any
            The left term of the rule
        """
        raise NotImplementedError

    @property
    @abstractmethod
    def right_terms(self):
        """The right terms

        Returns
        ----------
        right_terms : iterable of any
            The right terms of the rule
        """
        raise NotImplementedError

    @property
    @abstractmethod
    def right_term(self):
        """The unique right term

        Returns
        ----------
        right_term : iterable of any
            The unique right term of the rule
        """
        raise NotImplementedError

    @property
    @abstractmethod
    def production(self):
        """The production

        Returns
        ----------
        right_terms : any
            The production
        """
        raise NotImplementedError
