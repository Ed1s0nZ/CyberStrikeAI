"""
Representation of a regular expression
"""
from typing import Iterable

from pyformlang import finite_automaton
# pylint: disable=cyclic-import
import pyformlang.regular_expression.regex_objects
from pyformlang import cfg
from pyformlang.finite_automaton import State
# pylint: disable=cyclic-import
from pyformlang.regular_expression.regex_reader import RegexReader
from pyformlang import regular_expression


class Regex(RegexReader):
    """ Represents a regular expression

    Pyformlang implements the operators of textbooks, which deviate slightly \
    from the operators in Python. For a representation closer to Python one, \
    please use :class:`~pyformlang.regular_expression.PythonRegex`

    * The concatenation can be represented either by a space or a dot (.)
    * The union is represented either by | or +
    * The Kleene star is represented by *
    * The epsilon symbol can either be "epsilon" or $

    It is also possible to use parentheses. All symbols except the space, ., \
 |, +, *, (, ), epsilon and $ can be part of the alphabet. All \
 other common regex operators (such as []) are syntactic sugar that can be \
 reduced to the previous operators. Another main difference is that the \
 alphabet is not reduced to single characters as it is the case in Python. \
 For example, "python" is a single symbol in Pyformlang, whereas it is the \
 concatenation of six symbols in regular Python.

    All special characters except epsilon can be escaped with a backslash (\
    double backslash \\ in strings).

    Parameters
    ----------
    regex : str
        The regex represented as a string

    Raises
    ------
    MisformedRegexError
        If the regular expression is misformed.

    Examples
    --------

    >>> regex = Regex("abc|d")

    Check if the symbol "abc" is accepted

    >>> regex.accepts(["abc"])
    True

    Check if the word composed of the symbols "a", "b" and "c" is accepted

    >>> regex.accepts(["a", "b", "c"])
    False

    Check if the symbol "d" is accepted

    >>> regex.accepts(["d"])  # True

    >>> regex1 = Regex("a b")
    >>> regex_concat = regex.concatenate(regex1)
    >>> regex_concat.accepts(["d", "a", "b"])
    True

    >>> print(regex_concat.get_tree_str())
    Operator(Concatenation)
     Operator(Union)
      Symbol(abc)
      Symbol(d)
     Operator(Concatenation)
      Symbol(a)
      Symbol(b)

    Give the equivalent finite-state automaton

    >>> regex_concat.to_epsilon_nfa()

    """

    def __init__(self, regex):
        self.head = None
        self.sons = None
        super().__init__(regex)
        self._counter = 0
        self._initialize_enfa()
        self._enfa = None

    def _initialize_enfa(self):
        self._enfa = finite_automaton.EpsilonNFA()

    def get_number_symbols(self) -> int:
        """ Gives the number of symbols in the regex

        Returns
        ----------
        n_symbols : int
            The number of symbols in the regex

        Examples
        --------

        >>> regex = Regex("a|b*")
        >>> regex.get_number_symbols()
        2

        The two symbols are "a" and "b".
        """
        if self.sons:
            return sum(son.get_number_symbols() for son in self.sons)
        return 1

    def get_number_operators(self) -> int:
        """ Gives the number of operators in the regex

        Returns
        ----------
        n_operators : int
            The number of operators in the regex

        Examples
        --------

        >>> regex = Regex("a|b*")
        >>> regex.get_number_operators()
        2

        The two operators are "|" and "*".

        """
        if self.sons:
            return 1 + sum(son.get_number_operators() for son in self.sons)
        return 0

    def to_epsilon_nfa(self):
        """ Transforms the regular expression into an epsilon NFA

        Returns
        ----------
        enfa : :class:`~pyformlang.finite_automaton.EpsilonNFA`
            An epsilon NFA equivalent to the regex

        Examples
        --------

        >>> regex = Regex("abc|d")
        >>> regex.to_epsilon_nfa()

        """
        self._initialize_enfa()
        s_initial = self._set_and_get_initial_state_in_enfa()
        s_final = self._set_and_get_final_state_in_enfa()
        self._process_to_enfa(s_initial, s_final)
        return self._enfa

    def _set_and_get_final_state_in_enfa(self):
        s_final = self._get_next_state_enfa()
        self._enfa.add_final_state(s_final)
        return s_final

    def _get_next_state_enfa(self):
        s_final = finite_automaton.State(self._counter)
        self._counter += 1
        return s_final

    def _set_and_get_initial_state_in_enfa(self):
        s_initial = self._get_next_state_enfa()
        self._enfa.add_start_state(s_initial)
        return s_initial

    def _process_to_enfa(self, s_from: State, s_to: State):
        """ Internal function to add a regex to a given epsilon NFA

        Parameters
        ----------
        s_from : :class:`~pyformlang.finite_automaton.State`
            The source state
        s_to : :class:`~pyformlang.finite_automaton.State`
            The destination state
        """
        if self.sons:
            self._process_to_enfa_when_sons(s_from, s_to)
        else:
            self._process_to_enfa_when_no_son(s_from, s_to)

    def _process_to_enfa_when_no_son(self, s_from, s_to):
        if isinstance(self.head,
                      pyformlang.regular_expression.regex_objects.Epsilon):
            self._add_epsilon_transition_in_enfa_between(s_from, s_to)
        elif not isinstance(self.head,
                            pyformlang.regular_expression.regex_objects.Empty):
            symbol = finite_automaton.Symbol(self.head.value)
            self._enfa.add_transition(s_from, symbol, s_to)

    def _process_to_enfa_when_sons(self, s_from, s_to):
        if isinstance(
                self.head,
                pyformlang.regular_expression.regex_objects.Concatenation):
            self._process_to_enfa_concatenation(s_from, s_to)
        elif isinstance(self.head,
                        pyformlang.regular_expression.regex_objects.Union):
            self._process_to_enfa_union(s_from, s_to)
        elif isinstance(
                self.head,
                pyformlang.regular_expression.regex_objects.KleeneStar):
            self._process_to_enfa_kleene_star(s_from, s_to)

    def _process_to_enfa_kleene_star(self, s_from, s_to):
        # pylint: disable=protected-access
        state_first = self._get_next_state_enfa()
        state_second = self._get_next_state_enfa()
        self._add_epsilon_transition_in_enfa_between(state_second, state_first)
        self._add_epsilon_transition_in_enfa_between(s_from, s_to)
        self._add_epsilon_transition_in_enfa_between(s_from, state_first)
        self._add_epsilon_transition_in_enfa_between(state_second, s_to)
        self._process_to_enfa_son(state_first, state_second, 0)

    def _process_to_enfa_union(self, s_from, s_to):
        son_number = 0
        self._create_union_branch_in_enfa(s_from, s_to, son_number)
        son_number = 1
        self._create_union_branch_in_enfa(s_from, s_to, son_number)

    def _create_union_branch_in_enfa(self, s_from, s_to, son_number):
        state0 = self._get_next_state_enfa()
        state2 = self._get_next_state_enfa()
        self._add_epsilon_transition_in_enfa_between(s_from, state0)
        self._add_epsilon_transition_in_enfa_between(state2, s_to)
        self._process_to_enfa_son(state0, state2, son_number)

    def _process_to_enfa_concatenation(self, s_from, s_to):
        state0 = self._get_next_state_enfa()
        state1 = self._get_next_state_enfa()
        self._add_epsilon_transition_in_enfa_between(state0, state1)
        self._process_to_enfa_son(s_from, state0, 0)
        self._process_to_enfa_son(state1, s_to, 1)

    def _add_epsilon_transition_in_enfa_between(self, state0, state1):
        self._enfa.add_transition(state0, finite_automaton.Epsilon(), state1)

    def _process_to_enfa_son(self, s_from, s_to, index_son):
        # pylint: disable=protected-access
        self.sons[index_son]._counter = self._counter
        self.sons[index_son]._enfa = self._enfa
        self.sons[index_son]._process_to_enfa(s_from, s_to)
        self._counter = self.sons[index_son]._counter

    def get_tree_str(self, depth: int = 0) -> str:
        """ Get a string representation of the tree behind the regex

        Parameters
        ----------
        depth: int
            The current depth, 0 by default
        Returns
        -------
        representation: str
            The tree representation

        Examples
        --------

        >>> regex = Regex("abc|d*")
        >>> print(regex.get_tree_str())
        Operator(Union)
         Symbol(abc)
         Operator(Kleene Star)
          Symbol(d)

        """
        temp = " " * depth + str(self.head) + "\n"
        for son in self.sons:
            temp += son.get_tree_str(depth + 1)
        return temp

    def to_cfg(self, starting_symbol="S") -> "CFG":
        """
        Turns the regex into a context-free grammar

        Parameters
        ----------
        starting_symbol : :class:`~pyformlang.cfg.Variable`, optional
            The starting symbol

        Returns
        -------
        cfg : :class:`~pyformlang.cfg.CFG`
            An equivalent context-free grammar

        Examples
        --------

        >>> regex = Regex("(a|b)* c")
        >>> my_cfg = regex.to_cfg()
        >>> my_cfg.contains(["c"])
        True

        """
        productions, _ = self._get_production(starting_symbol)
        cfg_res = cfg.CFG(start_symbol=cfg.utils.to_variable(starting_symbol),
                          productions=set(productions))
        return cfg_res

    def _get_production(self, current_symbol, count=0):
        next_symbols = []
        next_productions = []
        for son in self.sons:
            next_symbol = "A" + str(count)
            count += 1
            # pylint: disable=protected-access
            new_prods, count = son._get_production(next_symbol, count)
            next_symbols.append(next_symbol)
            next_productions += new_prods
        new_prods = self.head.get_cfg_rules(current_symbol, next_symbols)
        next_productions += new_prods
        return next_productions, count

    def __repr__(self):
        return self.head.get_str_repr([str(son) for son in self.sons])

    def union(self, other: "Regex") -> "Regex":
        """ Makes the union with another regex

        Equivalent to:
          >>> regex0 or regex1

        Parameters
        ----------
        other : :class:`~pyformlang.regular_expression.Regex`
            The other regex

        Returns
        ----------
        regex : :class:`~pyformlang.regular_expression.Regex`
            The union of the two regex

        Examples
        --------

        >>> regex0 = Regex("a b")
        >>> regex1 = Regex("c")
        >>> regex_union = regex0.union(regex1)
        >>> regex_union.accepts(["a", "b"])
        >>> regex_union.accepts(["c"])

        Or equivalently:

        >>> regex_union = regex0 or regex1
        >>> regex_union.accepts(["a", "b"])

        """
        regex = Regex("")
        regex.head = pyformlang.regular_expression.regex_objects.Union()
        regex.sons = [self, other]
        return regex

    def __or__(self, other):
        """ Makes the union with another regex

        Parameters
        ----------
        other : :class:`~pyformlang.regular_expression.Regex`
            The other regex

        Returns
        ----------
        regex : :class:`~pyformlang.regular_expression.Regex`
            The union of the two regex

        Examples
        --------

        >>> regex0 = Regex("a b")
        >>> regex1 = Regex("c")
        >>> regex_union = regex0.union(regex1)
        >>> regex_union.accepts(["a", "b"])
        True
        >>> regex_union.accepts(["c"])
        True

        Or equivalently:

        >>> regex_union = regex0 or regex1
        >>> regex_union.accepts(["a", "b"])
        True
        """
        return self.union(other)

    def concatenate(self, other: "Regex") -> "Regex":
        """ Concatenates a regular expression with an other one

        Equivalent to:
          >>> regex0 + regex1

        Parameters
        ----------
        other : :class:`~pyformlang.regular_expression.Regex`
            The other regex

        Returns
        ----------
        regex : :class:`~pyformlang.regular_expression.Regex`
            The concatenation of the two regex

        Examples
        --------

        >>> regex0 = Regex("a b")
        >>> regex1 = Regex("c")
        >>> regex_union = regex0.concatenate(regex1)
        >>> regex_union.accepts(["a", "b"])
        False
        >>> regex_union.accepts(["a", "b", "c"])
        True

        Or equivalently:

        >>> regex_union = regex0 + regex1
        >>> regex_union.accepts(["a", "b", "c"])
        True
        """
        regex = Regex("")
        regex.head = \
            pyformlang.regular_expression.regex_objects.Concatenation()
        regex.sons = [self, other]
        return regex

    def __add__(self, other):
        """ Concatenates a regular expression with an other one

        Parameters
        ----------
        other : :class:`~pyformlang.regular_expression.Regex`
            The other regex

        Returns
        ----------
        regex : :class:`~pyformlang.regular_expression.Regex`
            The concatenation of the two regex

        Examples
        --------

        >>> regex0 = Regex("a b")
        >>> regex1 = Regex("c")
        >>> regex_union = regex0.concatenate(regex1)
        >>> regex_union.accepts(["a", "b"])
        False
        >>> regex_union.accepts(["a", "b", "c"])
        True

        Or equivalently:

        >>> regex_union = regex0 + regex1
        >>> regex_union.accepts(["a", "b", "c"])
        True

        """
        return self.concatenate(other)

    def kleene_star(self) -> "Regex":
        """ Makes the kleene star of the current regex

        Returns
        ----------
        regex : :class:`~pyformlang.regular_expression.Regex`
            The kleene star of the current regex

        Examples
        --------

        >>> regex = Regex("a")
        >>> regex_kleene = regex.kleene_star()
        >>> regex_kleene.accepts([])
        True
        >>> regex_kleene.accepts(["a", "a", "a"])
        True

        """
        regex = Regex("")
        regex.head = pyformlang.regular_expression.regex_objects.KleeneStar()
        regex.sons = [self]
        return regex

    def from_string(self, regex_str: str):
        """ Construct a regex from a string. For internal usage.

        Equivalent to the constructor of Regex

        Parameters
        ----------
        regex_str : str
            The string representation of the regex

        Returns
        -------
        regex : :class:`~pyformlang.regular_expression.Regex`
            The regex

        Examples
        --------
        >>> Regex.from_string("a b c")

        , which is equivalent to:

        >>> Regex("a b c")

        """
        return Regex(regex_str)

    def accepts(self, word: Iterable[str]) -> bool:
        """
        Check if a word matches (completely) the regex

        Parameters
        ----------
        word : iterable of str
            The word to check

        Returns
        -------
        is_accepted : bool
            Whether the word is recognized or not

        Examples
        --------

        >>> regex = Regex("abc|d")

        Check if the symbol "abc" is accepted

        >>> regex.accepts(["abc"])
        True

        """
        if self._enfa is None:
            self._enfa = self.to_epsilon_nfa()
        return self._enfa.accepts(word)
