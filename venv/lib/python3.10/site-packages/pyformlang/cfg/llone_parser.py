""" LL(1) Parser """


from pyformlang.cfg.epsilon import Epsilon
from pyformlang.cfg.cfg import NotParsableException
from pyformlang.cfg.parse_tree import ParseTree
from pyformlang.cfg.set_queue import SetQueue
from pyformlang.cfg.utils import to_terminal
from pyformlang.cfg.utils_cfg import get_productions_d


class LLOneParser:
    """
    A LL(1) parser

    Parameters
    ----------
    cfg : :class:`~pyformlang.cfg.CFG`
        A context-free Grammar
    """

    def __init__(self, cfg):
        self._cfg = cfg

    def get_first_set(self):
        """ Used in LL(1) """
        # Algorithm from:
        # https://www.geeksforgeeks.org/first-set-in-syntax-analysis/
        triggers = self._get_triggers()
        first_set, to_process = self._initialize_first_set(triggers)
        production_by_head = get_productions_d(self._cfg.productions)
        while to_process:
            current = to_process.pop()
            for production in production_by_head[current]:
                if not production.body:
                    continue
                first_set_temp = self._get_first_set_production(production,
                                                                first_set)
                length_before = len(first_set.get(production.head, set()))
                first_set[production.head] = first_set.get(
                    production.head, set()).union(
                        first_set_temp)
                if len(first_set[production.head]) != length_before:
                    for triggered in triggers.get(production.head, []):
                        to_process.append(triggered)
        return first_set

    @staticmethod
    def _get_first_set_production(production, first_set):
        first_not_containing_epsilon = 0
        first_set_temp = set()
        for body_component in production.body:
            first_set_temp = first_set_temp.union(
                first_set.get(
                    production.body[first_not_containing_epsilon],
                    set()))
            if Epsilon() not in first_set.get(body_component, set()):
                break
            first_not_containing_epsilon += 1
        if first_not_containing_epsilon != len(production.body):
            if Epsilon() in first_set_temp:
                first_set_temp.remove(Epsilon())
        return first_set_temp

    def _initialize_first_set(self, triggers):
        to_process = SetQueue()
        first_set = {}
        # Initialisation
        for terminal in self._cfg.terminals:
            first_set[terminal] = {terminal}
            for triggered in triggers.get(terminal, []):
                to_process.append(triggered)
        # Generate only epsilon
        for production in self._cfg.productions:
            if not production.body:
                first_set[production.head] = {Epsilon()}
                for triggered in triggers.get(production.head, []):
                    to_process.append(triggered)
        return first_set, to_process

    def _get_triggers(self):
        triggers = {}
        for production in self._cfg.productions:
            for body_component in production.body:
                if body_component not in triggers:
                    triggers[body_component] = []
                triggers[body_component].append(production.head)
        return triggers

    def get_follow_set(self):
        """ Get follow set """
        first_set = self.get_first_set()
        triggers = self._get_triggers_follow_set(first_set)
        follow_set, to_process = self._initialize_follow_set(first_set)
        while to_process:
            current = to_process.pop()
            for triggered in triggers.get(current, set()):
                length_before = len(follow_set.get(triggered, set()))
                follow_set[triggered] = follow_set.get(
                    triggered, set()
                ).union(follow_set.get(current, set()))
                if length_before != len(follow_set[triggered]):
                    to_process.append(triggered)
        return follow_set

    def _initialize_follow_set(self, first_set):
        to_process = SetQueue()
        follow_set = {}
        follow_set[self._cfg.start_symbol] = {"$"}
        to_process.append(self._cfg.start_symbol)
        for production in self._cfg.productions:
            for i, component in enumerate(production.body):
                for component_next in production.body[i + 1:]:
                    follow_set[component] = follow_set.get(
                        component, set()
                    ).union(first_set.get(component_next, set()))
                    if Epsilon() not in first_set.get(component_next,
                                                      set()):
                        break
                if Epsilon() in follow_set.get(component, set()):
                    follow_set[component].remove(Epsilon())
                if follow_set.get(component, set()):
                    to_process.append(component)
        return follow_set, to_process

    def _get_triggers_follow_set(self, first_set):
        triggers = {}
        for production in self._cfg.productions:
            if production.head not in triggers:
                triggers[production.head] = set()
            for i, component in enumerate(production.body):
                all_epsilon = True
                for component_next in production.body[i + 1:]:
                    if Epsilon() not in first_set.get(component_next, set()):
                        all_epsilon = False
                        break
                if all_epsilon:
                    triggers[production.head].add(component)
        return triggers

    def get_llone_parsing_table(self):
        """ Get the LL(1) parsing table
        From:
        https://www.slideshare.net/MahbuburRahman273/ll1-parser-in-compilers
        """
        first_set = self.get_first_set()
        follow_set = self.get_follow_set()
        nullables = self._cfg.get_nullable_symbols()
        nullable_productions = []
        non_nullable_productions = []
        for production in self._cfg.productions:
            if all(x in nullables for x in production.body):
                nullable_productions.append(production)
            else:
                non_nullable_productions.append(production)
        llone_parsing_table = {}
        for production in nullable_productions:
            if production.head not in llone_parsing_table:
                llone_parsing_table[production.head] = {}
            for first in follow_set.get(production.head, set()):
                if first not in llone_parsing_table[production.head]:
                    llone_parsing_table[production.head][first] = []
                llone_parsing_table[production.head][first].append(
                    production
                )
        for production in non_nullable_productions:
            if production.head not in llone_parsing_table:
                llone_parsing_table[production.head] = {}
            for first in self._get_first_set_production(production,
                                                        first_set):
                if first not in llone_parsing_table[production.head]:
                    llone_parsing_table[production.head][first] = []
                llone_parsing_table[production.head][first].append(
                    production
                )
        return llone_parsing_table

    def is_llone_parsable(self):
        """
        Checks whether the grammar can be parse with the LL(1) parser.

        Returns
        -------
        is_parsable : bool
        """
        parsing_table = self.get_llone_parsing_table()
        for variable in parsing_table.values():
            for terminal in variable.values():
                if len(terminal) > 1:
                    return False
        return True

    def get_llone_parse_tree(self, word):
        """
        Get LL(1) parse Tree

        Parameters
        ----------
        word : list
            The word to parse

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
        word.append("$")
        word = word[::-1]
        parsing_table = self.get_llone_parsing_table()
        parse_tree = ParseTree(self._cfg.start_symbol)
        stack = ["$", parse_tree]
        while stack:
            current = stack.pop()
            if current == "$" and word[-1] == "$":
                return parse_tree
            if current.value == word[-1]:
                word.pop()
            else:
                rule_applied = list(parsing_table.get(current.value, {})
                                    .get(word[-1], []))
                if len(rule_applied) == 1:
                    for component in rule_applied[0].body[::-1]:
                        new_node = ParseTree(component)
                        current.sons.append(new_node)
                        stack.append(new_node)
                else:
                    raise NotParsableException
                current.sons = current.sons[::-1]
        raise NotParsableException
