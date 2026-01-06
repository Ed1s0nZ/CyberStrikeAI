"""
Representations of rules in a indexed grammar
"""

from typing import Iterable, Dict, Any, List

from .production_rule import ProductionRule
from .consumption_rule import ConsumptionRule
from .rule_ordering import RuleOrdering
from .reduced_rule import ReducedRule


class Rules:
    """Store a set of rules and manipulate them

    Parameters
    ----------
    rules : iterable of :class:`~pyformlang.indexed_grammar.ReducedRule`
        A list of all the rules
    optim : int
        Optimization of the order of the rules
        0 -> given order
        1 -> reverse order
        2 -> order by core number
        3 -> reverse order of core number
        4 -> reverse order by arborescence
        5 -> order by arborescence
        6 -> order by number of edges
        7 -> reverse order by number of edges
        8 -> random order
    """

    def __init__(self, rules: Iterable[ReducedRule], optim: int = 7):
        self._rules = []
        self._consumption_rules = {}
        self._optim = optim
        for rule in rules:
            # We separate consumption rule from other
            if rule.is_consumption():
                temp = self._consumption_rules.setdefault(rule.f_parameter, [])
                if rule not in temp:
                    temp.append(rule)
                self._consumption_rules[rule.f_parameter] = temp
            else:
                if rule not in self._rules:
                    self._rules.append(rule)
        rule_ordering = RuleOrdering(self._rules, self._consumption_rules)
        if optim == 1:
            self._rules = rule_ordering.reverse()
        elif optim == 2:
            self._rules = rule_ordering.order_by_core()
        elif optim == 3:
            self._rules = rule_ordering.order_by_core(reverse=True)
        elif optim == 4:
            self._rules = rule_ordering.order_by_arborescence(reverse=True)
        elif optim == 5:
            self._rules = rule_ordering.order_by_arborescence(reverse=False)
        elif optim == 6:
            self._rules = rule_ordering.order_by_edges()
        elif optim == 7:
            self._rules = rule_ordering.order_by_edges(reverse=True)
        elif optim == 8:
            self._rules = rule_ordering.order_random()

    @property
    def optim(self):
        """Gets the optimization number

        Returns
        ----------
        non_consumption_rules :  int
            The optimization number
        """
        return self._optim

    @property
    def rules(self) -> Iterable[ReducedRule]:
        """Gets the non consumption rules

        Returns
        ----------
        non_consumption_rules :  iterable of \
        :class:`~pyformlang.indexed_grammar.ReducedRule`
            The non consumption rules
        """
        return self._rules

    @property
    def length(self) -> (int, int):
        """Get the total number of rules

        Returns
        ---------
        number_rules : couple of int
            A couple with first the number of non consumption rules and then\
                the number of consumption rules
        """
        return len(self._rules), len(self._consumption_rules.values())

    @property
    def consumption_rules(self) -> Dict[Any, Iterable[ConsumptionRule]]:
        """Gets the consumption rules

        Returns
        ----------
        consumption_rules : dict of any to iterable of \
            :class:`~pyformlang.indexed_grammar.ConsumptionRule`
            A dictionary contains the consumption rules gathered by consumed \
            symbols
        """
        return self._consumption_rules

    @property
    def terminals(self) -> Iterable[Any]:
        """Gets all the terminals used by all the rules

        Returns
        ----------
        terminals : iterable of any
            The terminals used in the rules
        """
        terminals = set()
        for temp_rule in self._consumption_rules.values():
            for rule in temp_rule:
                terminals = terminals.union(rule.terminals)
        for rule in self._rules:
            terminals = terminals.union(rule.terminals)
        return terminals

    @property
    def non_terminals(self) -> List[Any]:
        """Gets all the non-terminals used by all the rules

        Returns
        ----------
        non_terminals : iterable of any
            The non terminals used in the rule
        """
        terminals = set()
        for temp_rule in self._consumption_rules.values():
            for rule in temp_rule:
                terminals = terminals.union(set(rule.non_terminals))
        for rule in self._rules:
            terminals = terminals.union(set(rule.non_terminals))
        return list(terminals)

    def remove_production(self, left: Any, right: Any, prod: Any):
        """Remove the production rule:
            left[sigma] -> right[prod sigma]

        Parameters
        -----------
        left : any
            The left non-terminal in the rule
        right : any
            The right non-terminal in the rule
        prod : any
            The production used in the rule
        """
        self._rules = list(filter(lambda x: not (x.is_production() and
                                                x.left_term == left and
                                                x.right_term == right and
                                                x.production == prod),
                                  self._rules))

    def add_production(self, left: Any, right: Any, prod: Any):
        """Add the production rule:
            left[sigma] -> right[prod sigma]

        Parameters
        -----------
        left : any
            The left non-terminal in the rule
        right : any
            The right non-terminal in the rule
        prod : any
            The production used in the rule
        """
        self._rules.append(ProductionRule(left, right, prod))
