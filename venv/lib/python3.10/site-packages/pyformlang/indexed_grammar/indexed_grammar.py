"""
Representation of an indexed grammar
"""

from typing import Any, Iterable, AbstractSet

import pyformlang

from .duplication_rule import DuplicationRule
from .production_rule import ProductionRule
from .rules import Rules


class IndexedGrammar:
    """ Describes an indexed grammar.

    Parameters
    ----------
    rules : :class:`~pyformlang.indexed_grammar.Rules`
        The rules of the grammar, in reduced form put into a Rule
    start_variable : Any, optional
        The start symbol of the indexed grammar
    """

    def __init__(self,
                 rules: Rules,
                 start_variable: Any = "S"):
        self.rules = rules
        self.start_variable = start_variable
        # Precompute all non-terminals
        self.non_terminals = rules.non_terminals
        self.non_terminals.append(self.start_variable)
        self.non_terminals = set(self.non_terminals)
        # We cache the marked items in case of future update of the query
        self.marked = {}
        # Initialize the marked symbols
        # Mark the identity
        for non_terminal_a in self.non_terminals:
            self.marked[non_terminal_a] = set()
            temp = frozenset({non_terminal_a})
            self.marked[non_terminal_a].add(temp)
        # Mark all end symbols
        for non_terminal_a in self.non_terminals:
            if exists(self.rules.rules,
                      lambda x: x.is_end_rule()
                      and x.left_term == non_terminal_a):
                self.marked[non_terminal_a].add(frozenset())

    @property
    def terminals(self) -> Iterable[Any]:
        """Get all the terminals in the grammar

        Returns
        ----------
        terminals : iterable of any
            The terminals used in the rules
        """
        return self.rules.terminals

    def _duplication_processing(self, rule: DuplicationRule):
        """Processes a duplication rule

        Parameters
        ----------
        rule : :class:`~pyformlang.indexed_grammar.DuplicationRule`
            The duplication rule to process
        """
        was_modified = False
        need_stop = False
        right_term_marked0 = []
        for marked_term0 in self.marked[rule.right_terms[0]]:
            right_term_marked1 = []
            for marked_term1 in self.marked[rule.right_terms[1]]:
                if marked_term0 <= marked_term1:
                    temp = marked_term1
                elif marked_term1 <= marked_term0:
                    temp = marked_term0
                else:
                    temp = marked_term0.union(marked_term1)
                # Check if it was marked before
                if temp not in self.marked[rule.left_term]:
                    was_modified = True
                    if rule.left_term == rule.right_terms[0]:
                        right_term_marked0.append(temp)
                    elif rule.left_term == rule.right_terms[1]:
                        right_term_marked1.append(temp)
                    else:
                        self.marked[rule.left_term].add(temp)
                    # Stop condition, no need to continue
                    if rule.left_term == self.start_variable and len(
                            temp) == 0:
                        need_stop = True
            for temp in right_term_marked1:
                self.marked[rule.right_terms[1]].add(temp)
        for temp in right_term_marked0:
            self.marked[rule.right_terms[0]].add(temp)

        return was_modified, need_stop

    def _production_process(self, rule: ProductionRule):
        """Processes a production rule

        Parameters
        ----------
        rule : :class:`~pyformlang.indexed_grammar.ProductionRule`
            The production rule to process
        """
        was_modified = False
        # f_rules contains the consumption rules associated with
        # the current production symbol
        f_rules = self.rules.consumption_rules.setdefault(
            rule.production, [])
        # l_rules contains the left symbol plus what is marked on
        # the right side
        l_temp = [(x.left_term,
                   self.marked[x.right]) for x in f_rules]
        marked_symbols = [x.left_term for x in f_rules]
        # Process all combinations of consumption rule
        was_modified |= addrec_bis(l_temp,
                                   self.marked[rule.left_term],
                                   self.marked[rule.right_term])
        # End condition
        if frozenset() in self.marked[self.start_variable]:
            return was_modified, True
        # Is it useful?
        if rule.right_term in marked_symbols:
            for term in [term for term in l_temp
                         if rule.right_term == term[0]]:
                for sub_term in [sub_term
                                 for sub_term in term[1]
                                 if sub_term not in
                                 self.marked[rule.left_term]]:
                    was_modified = True
                    self.marked[rule.left_term].add(sub_term)
                    if (rule.left_term == self.start_variable and
                            len(sub_term) == 0):
                        return was_modified, True
        # Edge case
        if frozenset() in self.marked[rule.right_term]:
            if frozenset() not in self.marked[rule.left_term]:
                was_modified = True
                self.marked[rule.left_term].add(frozenset())
        return was_modified, False

    def is_empty(self) -> bool:
        """Checks whether the grammar generates a word or not

        Returns
        ----------
        is_empty : bool
            Whether the grammar is empty or not
        """
        # To know when no more modification are done
        was_modified = True
        while was_modified:
            was_modified = False
            for rule in self.rules.rules:
                # If we have a duplication rule, we mark all combinations of
                # the sets marked on the right side for the symbol on the left
                # side
                if rule.is_duplication():
                    dup_res = self._duplication_processing(rule)
                    was_modified |= dup_res[0]
                    if dup_res[1]:
                        return False
                elif rule.is_production():
                    prod_res = self._production_process(rule)
                    if prod_res[1]:
                        return False
                    was_modified |= prod_res[0]
        if frozenset() in self.marked[self.start_variable]:
            return False
        return True

    def __bool__(self):
        return not self.is_empty()

    def get_reachable_non_terminals(self) -> AbstractSet[Any]:
        """ Get the reachable symbols

        Returns
        ----------
        reachables : set of any
            The reachable symbols from the start state
        """
        # Preprocess
        reachable_from = {}
        consumption_rules = self.rules.consumption_rules
        for rule in self.rules.rules:
            if rule.is_duplication():
                left = rule.left_term
                right0 = rule.right_terms[0]
                right1 = rule.right_terms[1]
                if left not in reachable_from:
                    reachable_from[left] = set()
                reachable_from[left].add(right0)
                reachable_from[left].add(right1)
            if rule.is_production():
                left = rule.left_term
                right = rule.right_term
                if left not in reachable_from:
                    reachable_from[left] = set()
                reachable_from[left].add(right)
        for key in consumption_rules:
            for rule in consumption_rules[key]:
                left = rule.left_term
                right = rule.right
                if left not in reachable_from:
                    reachable_from[left] = set()
                reachable_from[left].add(right)
        # Processing
        to_process = [self.start_variable]
        reachables = {self.start_variable}
        while to_process:
            current = to_process.pop()
            for symbol in reachable_from.get(current, []):
                if symbol not in reachables:
                    reachables.add(symbol)
                    to_process.append(symbol)
        return reachables

    def get_generating_non_terminals(self) -> AbstractSet[Any]:
        """ Get the generating symbols

        Returns
        ----------
        generating : set of any
            The generating symbols from the start state
        """
        # Preprocess
        generating_from = {}
        duplication_pointer = {}
        generating = set()
        to_process = []
        self._preprocess_rules_generating(duplication_pointer, generating,
                                          generating_from, to_process)
        self._preprocess_consumption_rules_generating(generating_from)
        # Processing
        while to_process:
            current = to_process.pop()
            for symbol in generating_from.get(current, []):
                if symbol not in generating:
                    generating.add(symbol)
                    to_process.append(symbol)
            for duplication in duplication_pointer.get(current, []):
                duplication[1] -= 1
                if duplication[1] == 0:
                    if duplication[0] not in generating:
                        generating.add(duplication[0])
                        to_process.append(duplication[0])
        return generating

    def _preprocess_consumption_rules_generating(self, generating_from):
        for key in self.rules.consumption_rules:
            for rule in self.rules.consumption_rules[key]:
                left = rule.left_term
                right = rule.right
                if right in generating_from:
                    generating_from[right].add(left)
                else:
                    generating_from[right] = {left}

    def _preprocess_rules_generating(self, duplication_pointer, generating,
                                     generating_from, to_process):
        for rule in self.rules.rules:
            if rule.is_duplication():
                left = rule.left_term
                right0 = rule.right_terms[0]
                right1 = rule.right_terms[1]
                temp = [left, 2]
                if right0 in duplication_pointer:
                    duplication_pointer[right0].append(temp)
                else:
                    duplication_pointer[right0] = [temp]
                if right1 in duplication_pointer:
                    duplication_pointer[right1].append(temp)
                else:
                    duplication_pointer[right1] = [temp]
            if rule.is_production():
                left = rule.left_term
                right = rule.right_term
                if right in generating_from:
                    generating_from[right].add(left)
                else:
                    generating_from[right] = {left}
            if rule.is_end_rule():
                left = rule.left_term
                if left not in generating:
                    generating.add(left)
                    to_process.append(left)

    def remove_useless_rules(self) -> "IndexedGrammar":
        """ Remove useless rules in the grammar

        More precisely, we remove rules which do not contain only generating \
        or  reachable non terminals.

        Returns
        ----------
        i_grammar : :class:`~pyformlang.indexed_grammar.IndexedGrammar`
            The indexed grammar which useless rules
        """
        l_rules = []
        generating = self.get_generating_non_terminals()
        reachables = self.get_reachable_non_terminals()
        consumption_rules = self.rules.consumption_rules
        for rule in self.rules.rules:
            if rule.is_duplication():
                left = rule.left_term
                right0 = rule.right_terms[0]
                right1 = rule.right_terms[1]
                if all(x in generating and x in reachables for x in
                       [left, right0, right1]):
                    l_rules.append(rule)
            if rule.is_production():
                left = rule.left_term
                right = rule.right_term
                if all(x in generating and x in reachables for x in
                       [left, right]):
                    l_rules.append(rule)
            if rule.is_end_rule():
                left = rule.left_term
                if left in generating and left in reachables:
                    l_rules.append(rule)
        for key in consumption_rules:
            for rule in consumption_rules[key]:
                left = rule.left_term
                right = rule.right
                if all(x in generating and x in reachables for x in
                       [left, right]):
                    l_rules.append(rule)
        rules = Rules(l_rules, self.rules.optim)
        return IndexedGrammar(rules)

    def intersection(self, other: Any) -> "IndexedGrammar":
        """ Computes the intersection of the current indexed grammar with the \
        other object

        Equivalent to
        --------------
          >> indexed_grammar and regex

        Parameters
        ----------
        other : any
            The object to intersect with

        Returns
        ----------
        i_grammar : :class:`~pyformlang.indexed_grammar.IndexedGrammar`
            The indexed grammar which useless rules

        Raises
        ------
        NotImplementedError
            When trying to intersection with something else than a regular
            expression or a finite automaton
        """
        if isinstance(other, pyformlang.regular_expression.Regex):
            other = other.to_epsilon_nfa()
        if isinstance(other, pyformlang.finite_automaton.FiniteAutomaton):
            fst = other.to_fst()
            return fst.intersection(self)
        raise NotImplementedError

    def __and__(self, other):
        """ Computes the intersection of the current indexed grammar with the
        other object

        Parameters
        ----------
        other : any
            The object to intersect with

        Returns
        ----------
        i_grammar : :class:`~pyformlang.indexed_grammar.IndexedGrammar`
            The indexed grammar which useless rules
        """
        return self.intersection(other)


def exists(list_elements, check_function):
    """exists
    Check whether at least an element x of l is True for f(x)
    :param list_elements: A list of elements to test
    :param check_function: The checking function (takes one parameter and  \
    return a boolean)
    """
    for element in list_elements:
        if check_function(element):
            return True
    return False


def addrec_bis(l_sets, marked_left, marked_right):
    """addrec_bis
    Optimized version of addrec
    :param l_sets: a list containing tuples (C, M) where:
        * C is a non-terminal on the left of a consumption rule
        * M is the set of the marked set for the right non-terminal in the
        production rule
    :param marked_left: Sets which are marked for the non-terminal on the
    left of the production rule
    :param marked_right: Sets which are marked for the non-terminal on the
    right of the production rule
    """
    was_modified = False
    for marked in list(marked_right):
        l_temp = [x for x in l_sets if x[0] in marked]
        s_temp = [x[0] for x in l_temp]
        # At least one symbol to consider
        if frozenset(s_temp) == marked and len(marked) > 0:
            was_modified |= addrec_ter(l_temp, marked_left)
    return was_modified


def addrec_ter(l_sets, marked_left):
    """addrec
    Explores all possible combination of consumption rules to mark a
    production rule.
    :param l_sets: a list containing tuples (C, M) where:
        * C is a non-terminal on the left of a consumption rule
        * M is the set of the marked set for the right non-terminal in the
        production rule
    :param marked_left: Sets which are marked for the non-terminal on the
    left of the production rule
    :return Whether an element was actually marked
    """
    # End condition, nothing left to process
    temp_in = [x[0] for x in l_sets]
    exists_after = [
        exists(l_sets[index + 1:], lambda x: x[0] == l_sets[index][0])
        for index in range(len(l_sets))]
    exists_before = [l_sets[index][0] in temp_in[:index]
                     for index in range(len(l_sets))]
    marked_sets = [l_sets[index][1] for index in range(len(l_sets))]
    marked_sets = [sorted(x, key=lambda x: -len(x)) for x in marked_sets]
    # Try to optimize by having an order of the sets
    sorted_zip = sorted(zip(exists_after, exists_before, marked_sets),
                        key=lambda x: -len(x[2]))
    exists_after, exists_before, marked_sets = \
        zip(*sorted_zip)
    res = False
    # contains tuples of index, temp_set
    to_process = [(0, frozenset())]
    done = set()
    while to_process:
        index, new_temp = to_process.pop()
        if index >= len(l_sets):
            # Check if at least one non-terminal was considered, then if the
            # set of non-terminals considered is marked of the right
            # non-terminal in the production rule, then if a new set is
            # marked or not
            if new_temp not in marked_left:
                marked_left.add(new_temp)
                res = True
            continue
        if exists_before[index] or exists_after[index]:
            to_append = (index + 1, new_temp)
            to_process.append(to_append)
        if not exists_before[index]:
            # For all sets which were marked for the current consumption rule
            for marked_set in marked_sets[index]:
                if marked_set <= new_temp:
                    to_append = (index + 1, new_temp)
                elif new_temp <= marked_set:
                    to_append = (index + 1, marked_set)
                else:
                    to_append = (index + 1, new_temp.union(marked_set))
                if to_append not in done:
                    done.add(to_append)
                    to_process.append(to_append)
    return res
