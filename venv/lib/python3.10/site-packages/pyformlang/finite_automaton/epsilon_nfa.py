"""
Nondeterministic Automaton with epsilon transitions
"""

from typing import Set, Iterable, AbstractSet, Any

# pylint: disable=cyclic-import
from pyformlang import finite_automaton

from .epsilon import Epsilon
from .state import State
from .symbol import Symbol
from .nondeterministic_transition_function import \
    NondeterministicTransitionFunction
from .regexable import Regexable
from .finite_automaton import FiniteAutomaton
from .finite_automaton import to_state, to_symbol


class EpsilonNFA(Regexable, FiniteAutomaton):
    """ Represents an epsilon NFA


    Parameters
    ----------
    states : set of :class:`~pyformlang.finite_automaton.State`, optional
        A finite set of states
    input_symbols : set of :class:`~pyformlang.finite_automaton.Symbol`, \
     optional
        A finite set of input symbols
    transition_function :  \
    :class:`~pyformlang.finite_automaton.NondeterministicTransitionFunction`\
, optional
        Takes as arguments a state and an input symbol and returns a state.
    start_state : set of :class:`~pyformlang.finite_automaton.State`, optional
        A start state, element of states
    final_states : set of :class:`~pyformlang.finite_automaton.State`, optional
        A set of final or accepting states. It is a subset of states.

    Examples
    --------

    >>> enfa = EpsilonNFA()

    Creates an empty epsilon non-deterministic automaton.

    >>> enfa.add_transitions([(0, "abc", 1), (0, "d", 1), (0, "epsilon", 2)])

    Adds three transition, one of them being an epsilon transition.

    >>> enfa.add_start_state(0)

    Adds a start state.

    >>> enfa.add_final_state(1)

    Adds a final state.

    >>> enfa.is_deterministic()
    False

    Checks if the automaton is deterministic.

    """

    # pylint: disable=too-many-arguments
    def __init__(
            self,
            states: AbstractSet[State] = None,
            input_symbols: AbstractSet[Symbol] = None,
            transition_function: NondeterministicTransitionFunction = None,
            start_state: AbstractSet[State] = None,
            final_states: AbstractSet[State] = None):
        super().__init__()
        if states is not None:
            states = {to_state(x) for x in states}
        self._states = states or set()
        if input_symbols is not None:
            input_symbols = {to_symbol(x) for x in input_symbols}
        self._input_symbols = input_symbols or set()
        self._transition_function = \
            transition_function or NondeterministicTransitionFunction()
        if start_state is not None:
            start_state = {to_state(x) for x in start_state}
        self._start_state = start_state or set()
        if final_states is not None:
            final_states = {to_state(x) for x in final_states}
        self._final_states = final_states or set()
        for state in self._final_states:
            if state is not None and state not in self._states:
                self._states.add(state)
        for state in self._start_state:
            if state is not None and state not in self._states:
                self._states.add(state)

    def _get_next_states_iterable(self,
                                  current_states: Iterable[State],
                                  symbol: Symbol) \
            -> Set[State]:
        """ Gives the set of next states, starting from a set of states

        Parameters
        ----------
        current_states : iterable of \
        :class:`~pyformlang.finite_automaton.State`
            The considered list of states
        symbol : Symbol
            The symbol of the link

        Returns
        ----------
        next_states : set of :class:`~pyformlang.finite_automaton.State`
            The next of resulting states
        """
        next_states = set()
        for current_state in current_states:
            next_states_temp = self._transition_function(current_state,
                                                         symbol)
            next_states = next_states.union(next_states_temp)
        return next_states

    def accepts(self, word: Iterable[Any]) -> bool:
        """ Checks whether the epsilon nfa accepts a given word

        Parameters
        ----------
        word : iterable of :class:`~pyformlang.finite_automaton.Symbol`
            A sequence of input symbols

        Returns
        ----------
        is_accepted : bool
            Whether the word is accepted or not

        Examples
        --------

        >>> enfa = EpsilonNFA()
        >>> enfa.add_transitions([(0, "abc", 1), (0, "d", 1), \
        (0, "epsilon", 2)])
        >>> enfa.add_start_state(0)
        >>> enfa.add_final_state(1)
        >>> enfa.accepts(["abc", "epsilon"])
        True

        >>> enfa.accepts(["epsilon"])
        False

        """
        word = [to_symbol(x) for x in word]
        current_states = self.eclose_iterable(self._start_state)
        for symbol in word:
            if symbol == Epsilon():
                continue
            next_states = self._get_next_states_iterable(current_states,
                                                         symbol)
            current_states = self.eclose_iterable(next_states)
        return any(self.is_final_state(x) for x in current_states)

    def eclose_iterable(self, states: Iterable[Any]) -> Set[State]:
        """ Compute the epsilon closure of a collection of states

        Parameters
        ----------
        states : iterable of :class:`~pyformlang.finite_automaton.State`
            The source states

        Returns
        ---------
        states : set of :class:`~pyformlang.finite_automaton.State`
            The epsilon closure of the source state

        Examples
        --------

        >>> enfa = EpsilonNFA()
        >>> enfa.add_transitions([(0, "abc", 1), (0, "d", 1), \
        (0, "epsilon", 2)])
        >>> enfa.add_start_state(0)
        >>> enfa.add_final_state(1)
        >>> enfa.eclose_iterable([0])
        {2}
        """
        states = [to_state(x) for x in states]
        res = set()
        for state in states:
            res = res.union(self.eclose(state))
        return res

    def eclose(self, state: Any) -> Set[State]:
        """ Compute the epsilon closure of a state

        Parameters
        ----------
        state : :class:`~pyformlang.finite_automaton.State`
            The source state

        Returns
        ---------
        states : set of :class:`~pyformlang.finite_automaton.State`
            The epsilon closure of the source state

        Examples
        --------

        >>> enfa = EpsilonNFA()
        >>> enfa.add_transitions([(0, "abc", 1), (0, "d", 1), \
        (0, "epsilon", 2)])
        >>> enfa.add_start_state(0)
        >>> enfa.add_final_state(1)
        >>> enfa.eclose(0)
        {2}

        """
        state = to_state(state)
        to_process = [state]
        processed = {state}
        while to_process:
            current = to_process.pop()
            connected = self._transition_function(current, Epsilon())
            for conn_state in connected:
                if conn_state not in processed:
                    processed.add(conn_state)
                    to_process.append(conn_state)
        return processed

    def is_deterministic(self) -> bool:
        """ Checks whether an automaton is deterministic

        Returns
        ----------
        is_deterministic : bool
           Whether the automaton is deterministic

        Examples
        --------

        Examples
        --------

        >>> enfa = EpsilonNFA()
        >>> enfa.add_transitions([(0, "abc", 1), (0, "d", 1), \
        (0, "epsilon", 2)])
        >>> enfa.add_start_state(0)
        >>> enfa.add_final_state(1)
        >>> enfa.is_deterministic()
        False

        """
        return len(self._start_state) <= 1 \
            and self._transition_function.is_deterministic()\
            and all({x} == self.eclose(x) for x in self._states)

    def remove_epsilon_transitions(self) -> "NondeterministicFiniteAutomaton":
        """ Removes the epsilon transitions from the automaton

        Returns
        ----------
        dfa :  :class:`~pyformlang.finite_automaton.\
NondeterministicFiniteAutomaton`
            A non-deterministic finite automaton equivalent to the current \
nfa, with no epsilon transition
        """
        from pyformlang.finite_automaton import NondeterministicFiniteAutomaton
        nfa = NondeterministicFiniteAutomaton()
        for state in self._start_state:
            nfa.add_start_state(state)
        for state in self._final_states:
            nfa.add_final_state(state)
        start_eclose = self.eclose_iterable(self._start_state)
        for state in start_eclose:
            nfa.add_start_state(state)
        for state in self._states:
            eclose = self.eclose(state)
            for e_state in eclose:
                if e_state in self._final_states:
                    nfa.add_final_state(state)
                for symb in self._input_symbols:
                    for next_state in self._transition_function(e_state, symb):
                        nfa.add_transition(state, symb, next_state)
        return nfa

    def _to_deterministic_internal(self,
                                   eclose: bool) \
            -> "DeterministicFiniteAutomaton":
        """ Transforms the epsilon-nfa into a dfa

        Parameters
        ----------
        eclose : bool
            Whether to use the epsilon closure or not

        Returns
        ----------
        dfa :  :class:`~pyformlang.finite_automaton\
        .DeterministicFiniteAutomaton`
            A dfa equivalent to the current nfa
        """
        dfa = finite_automaton.DeterministicFiniteAutomaton()
        # Add Eclose
        if eclose:
            start_eclose = self.eclose_iterable(self._start_state)
        else:
            start_eclose = self._start_state
        start_state = to_single_state(start_eclose)
        dfa.add_start_state(start_state)
        to_process = [start_eclose]
        processed = {start_state}
        while to_process:
            current = to_process.pop()
            s_from = to_single_state(current)
            for symb in self._input_symbols:
                all_trans = [self._transition_function(x, symb)
                             for x in current]
                state = set()
                for trans in all_trans:
                    state = state.union(trans)
                if not state:
                    continue
                # Eclose added
                if eclose:
                    state = self.eclose_iterable(state)
                state_merged = to_single_state(state)
                dfa.add_transition(s_from, symb, state_merged)
                if state_merged not in processed:
                    processed.add(state_merged)
                    to_process.append(state)
            for state in current:
                if state in self._final_states:
                    dfa.add_final_state(s_from)
        return dfa

    def to_deterministic(self) -> "DeterministicFiniteAutomaton":
        """ Transforms the epsilon-nfa into a dfa

        Returns
        ----------
        dfa :  :class:`~pyformlang.finite_automaton\
        .DeterministicFiniteAutomaton`
            A dfa equivalent to the current nfa

        Examples
        --------

        >>> enfa = EpsilonNFA()
        >>> enfa.add_transitions([(0, "abc", 1), (0, "d", 1), \
        (0, "epsilon", 2)])
        >>> enfa.add_start_state(0)
        >>> enfa.add_final_state(1)
        >>> dfa = enfa.to_deterministic()
        >>> dfa.is_deterministic()
        True

        >>> enfa.is_equivalent_to(dfa)
        True

        """
        return self._to_deterministic_internal(True)

    def copy(self) -> "EpsilonNFA":
        """ Copies the current Epsilon NFA

        Returns
        ----------
        enfa : :class:`~pyformlang.finite_automaton.EpsilonNFA`
            A copy of the current Epsilon NFA

        Examples
        --------

        >>> enfa = EpsilonNFA()
        >>> enfa.add_transitions([(0, "abc", 1), (0, "d", 1), \
        (0, "epsilon", 2)])
        >>> enfa.add_start_state(0)
        >>> enfa.add_final_state(1)
        >>> enfa_copy = enfa.copy()
        >>> enfa.is_equivalent_to(enfa_copy)
        True

        """
        enfa = EpsilonNFA()
        for start in self._start_state:
            enfa.add_start_state(start)
        for final in self._final_states:
            enfa.add_final_state(final)
        for state in self._states:
            for symbol in self._input_symbols:
                states = self._transition_function(state, symbol)
                for state_to in states:
                    enfa.add_transition(state, symbol, state_to)
            states = self._transition_function(state, Epsilon())
            for state_to in states:
                enfa.add_transition(state, Epsilon(), state_to)
        return enfa

    def __copy__(self):
        return self.copy()

    def to_regex(self) -> "Regex":
        """ Transforms the EpsilonNFA to a regular expression

        Returns
        ----------
        regex : :class:`~pyformlang.regular_expression.Regex`
            A regular expression equivalent to the current Epsilon NFA

        Examples
        --------

        >>> enfa = EpsilonNFA()
        >>> enfa.add_transitions([(0, "abc", 1), (0, "d", 1), \
        (0, "epsilon", 2)])
        >>> enfa.add_start_state(0)
        >>> enfa.add_final_state(1)
        >>> regex = enfa.to_regex()
        >>> regex.accepts(["abc"])
        True

        """
        from pyformlang.regular_expression import Regex
        enfas = [self.copy() for _ in self._final_states]
        final_states = list(self._final_states)
        for i in range(len(self._final_states)):
            for j in range(len(self._final_states)):
                if i != j:
                    enfas[j].remove_final_state(final_states[i])
        regex_l = []
        for enfa in enfas:
            # pylint: disable=protected-access
            enfa._remove_all_basic_states()
            # pylint: disable=protected-access
            regex_sub = enfa._get_regex_simple()
            if regex_sub:
                regex_l.append(regex_sub)
        res = "+".join(regex_l)
        return Regex(res)

    def _get_regex_simple(self) -> str:
        """ Get the regex of an automaton when it only composed of a start and
        a final state

        CAUTION: For internal use only!

        Returns
        ----------
        regex : str
            A regex representing the automaton
        """
        if not self._final_states or not self._start_state:
            return ""
        if len(self._final_states) != 1 or len(self._start_state) != 1:
            raise ValueError("The automaton is not simple enough!")
        if self._start_state == self._final_states:
            # We are suppose to have only one good symbol
            for symbol in self._input_symbols:
                out_states = self._transition_function(
                    list(self._start_state)[0], symbol)
                if out_states:
                    return "(" + str(symbol.value) + ")*"
            return "epsilon"
        start_to_start, start_to_end, end_to_start, end_to_end = \
            self._get_bi_transitions()
        return get_regex_sub(start_to_start,
                             start_to_end,
                             end_to_start,
                             end_to_end)

    def _get_bi_transitions(self) -> (str, str, str, str):
        """ Internal method to compute the transition in the case of a \
        simple automaton

        Returns
        start_to_start : str
            The transition from the start state to the start state
        start_to_end : str
            The transition from the start state to the end state
        end_to_start : str
            The transition from the end state to the start state
        end_to_end : str
            The transition from the end state to the end state
        ----------
        """
        start = list(self._start_state)[0]
        end = list(self._final_states)[0]
        start_to_start = "epsilon"
        start_to_end = ""
        end_to_end = "epsilon"
        end_to_start = ""
        for state in self._states:
            for symbol in self._input_symbols.union({Epsilon()}):
                for out_state in self._transition_function(state, symbol):
                    symbol_str = str(symbol.value)
                    if not symbol_str.isalnum():
                        symbol_str = "(" + symbol_str + ")"
                    if state == start and out_state == start:
                        start_to_start = symbol_str
                    elif state == start and out_state == end:
                        start_to_end = symbol_str
                    elif state == end and out_state == start:
                        end_to_start = symbol_str
                    elif state == end and out_state == end:
                        end_to_end = symbol_str
        return start_to_start, start_to_end, end_to_start, end_to_end

    def get_complement(self) -> "EpsilonNFA":
        """ Get the complement of the current Epsilon NFA

        Equivalent to:

          >>> -automaton

        Returns
        ----------
        enfa : :class:`~pyformlang.finite_automaton.EpsilonNFA`
            A complement automaton

        Examples
        --------

        >>> enfa = EpsilonNFA()
        >>> enfa.add_transitions([(0, "abc", 1), (0, "d", 1), \
        (0, "epsilon", 2)])
        >>> enfa.add_start_state(0)
        >>> enfa.add_final_state(1)
        >>> enfa_complement = enfa.get_complement()
        >>> enfa_complement.accepts(["epsilon"])
        True

        >>> enfa_complement.accepts(["abc"])
        False

        """
        enfa = self.copy()
        trash = State("TrashNode")
        enfa.add_final_state(trash)
        for state in self._states:
            if state in self._final_states:
                enfa.remove_final_state(state)
            else:
                enfa.add_final_state(state)
        for state in self._states:
            for symbol in self._input_symbols:
                state_to = []
                eclose = self.eclose(state)
                for state0 in eclose:
                    state_to += self._transition_function(state0, symbol)
                if not state_to:
                    enfa.add_transition(state, symbol, trash)
        for symbol in self._input_symbols:
            enfa.add_transition(trash, symbol, trash)
        return enfa

    def __neg__(self):
        """ Get the complement of the current Epsilon NFA

        Returns
        ----------
        enfa : :class:`~pyformlang.finite_automaton.EpsilonNFA`
            A complement automaton
        """
        return self.get_complement()

    def get_intersection(self, other: "EpsilonNFA") -> "EpsilonNFA":
        """ Computes the intersection of two Epsilon NFAs

        Equivalent to:

          >>> automaton0 and automaton1

        Parameters
        ----------
        other : :class:`~pyformlang.finite_automaton.EpsilonNFA`
            The other Epsilon NFA

        Returns
        ---------
        enfa : :class:`~pyformlang.finite_automaton.EpsilonNFA`
            The intersection of the two Epsilon NFAs

        Examples
        --------

        >>> enfa = EpsilonNFA()
        >>> enfa.add_transitions([(0, "abc", 1), (0, "d", 1), \
        (0, "epsilon", 2)])
        >>> enfa.add_start_state(0)
        >>> enfa.add_final_state(1)
        >>> enfa2 = EpsilonNFA()
        >>> enfa2.add_transition(0, "d", 1)
        >>> enfa2.add_final_state(1)
        >>> enfa2.add_start_state(0)
        >>> enfa_inter = enfa.get_intersection(enfa2)
        >>> enfa_inter.accepts(["abc"])
        False

        >>> enfa_inter.accepts(["d"])
        True

        """
        enfa = EpsilonNFA()
        symbols = list(self.symbols.intersection(other.symbols))
        to_process = []
        processed = set()
        for st0 in self.eclose_iterable(self.start_states):
            for st1 in other.eclose_iterable(other.start_states):
                enfa.add_start_state(combine_state_pair(st0, st1))
                to_process.append((st0, st1))
                processed.add((st0, st1))
        for st0 in self.final_states:
            for st1 in other.final_states:
                enfa.add_final_state(combine_state_pair(st0, st1))
        while to_process:
            st0, st1 = to_process.pop()
            current_state = combine_state_pair(st0, st1)
            for symb in symbols:
                for new_s0 in self.eclose_iterable(self(st0, symb)):
                    for new_s1 in other.eclose_iterable(other(st1, symb)):
                        state = combine_state_pair(new_s0, new_s1)
                        enfa.add_transition(current_state, symb, state)
                        if (new_s0, new_s1) not in processed:
                            processed.add((new_s0, new_s1))
                            to_process.append((new_s0, new_s1))
        return enfa

    def __and__(self, other):
        """ Computes the intersection of two Epsilon NFAs

        Parameters
        ----------
        other : :class:`~pyformlang.finite_automaton.EpsilonNFA`
           The other Epsilon NFA

        Returns
        ---------
        enfa : :class:`~pyformlang.finite_automaton.EpsilonNFA`
           The intersection of the two Epsilon NFAs
        """
        return self.get_intersection(other)

    def get_difference(self, other: "EpsilonNFA") \
            -> "EpsilonNFA":
        """ Compute the difference with another Epsilon NFA

        Equivalent to:

          >>> automaton0 - automaton1

        Parameters
        ----------
        other : :class:`~pyformlang.finite_automaton.EpsilonNFA`
            The other Epsilon NFA

        Returns
        ---------
        enfa : :class:`~pyformlang.finite_automaton.EpsilonNFA`
            The difference with the other epsilon NFA

        Examples
        --------

        >>> enfa = EpsilonNFA()
        >>> enfa.add_transitions([(0, "abc", 1), (0, "d", 1), \
        (0, "epsilon", 2)])
        >>> enfa.add_start_state(0)
        >>> enfa.add_final_state(1)
        >>> enfa2 = EpsilonNFA()
        >>> enfa2.add_transition(0, "d", 1)
        >>> enfa2.add_final_state(1)
        >>> enfa2.add_start_state(0)
        >>> enfa_diff = enfa.get_difference(enfa2)
        >>> enfa_diff.accepts(["d"])
        False

        >>> enfa_diff.accepts(["abc"])
        True

        """
        other = other.copy()
        for symbol in self._input_symbols:
            other.add_symbol(symbol)
        return self.get_intersection(other.get_complement())

    def __sub__(self, other):
        """ Compute the difference with another Epsilon NFA

        Equivalent to:
          >> automaton0 - automaton1

        Parameters
        ----------
        other : :class:`~pyformlang.finite_automaton.EpsilonNFA`
            The other Epsilon NFA

        Returns
        ---------
        enfa : :class:`~pyformlang.finite_automaton.EpsilonNFA`
            The difference with the other epsilon NFA
        """
        return self.get_difference(other)

    def reverse(self) -> "EpsilonNFA":
        """ Compute the reversed EpsilonNFA

        Equivalent to:
          >> ~automaton

        Returns
        ---------
        enfa : :class:`~pyformlang.finite_automaton.EpsilonNFA`
            The reversed automaton

        Examples
        --------

        >>> enfa = EpsilonNFA()
        >>> enfa.add_transitions([(0, "abc", 1), (1, "d", 2)])
        >>> enfa.add_start_state(0)
        >>> enfa.add_final_state(2)
        >>> enfa_reverse = enfa.reverse()
        >>> enfa_reverse.accepts(["d", "abc"])
        True

        """
        enfa = EpsilonNFA()
        for state0 in self._states:
            for symbol in self._input_symbols:
                for state1 in self._transition_function(state0, symbol):
                    enfa.add_transition(state1, symbol, state0)
            for state1 in self._transition_function(state0, Epsilon()):
                enfa.add_transition(state1, Epsilon(), state0)
        for start in self._start_state:
            enfa.add_final_state(start)
        for final in self._final_states:
            enfa.add_start_state(final)
        return enfa

    def __invert__(self):
        """ Compute the reversed EpsilonNFA

        Returns
        ---------
        enfa : :class:`~pyformlang.finite_automaton.EpsilonNFA`
            The reversed automaton
        """
        return self.reverse()

    def is_empty(self) -> bool:
        """ Checks if the language represented by the FSM is empty or not

        Returns
        ----------
        is_empty : bool
            Whether the language is empty or not

        Examples
        --------

        >>> enfa = EpsilonNFA()
        >>> enfa.add_transitions([(0, "abc", 1), (0, "d", 1), \
        (0, "epsilon", 2)])
        >>> enfa.add_start_state(0)
        >>> enfa.add_final_state(1)
        >>> enfa.is_empty()
        False

        """
        to_process = []
        processed = set()
        for start in self._start_state:
            to_process.append(start)
            processed.add(start)
        while to_process:
            current = to_process.pop()
            if current in self._final_states:
                return False
            for symbol in self._input_symbols:
                for state in self._transition_function(current, symbol):
                    if state not in processed:
                        to_process.append(state)
                        processed.add(state)
            for state in self._transition_function(current, Epsilon()):
                if state not in processed:
                    to_process.append(state)
                    processed.add(state)
        return True

    def _remove_all_basic_states(self):
        """ Remove all states which are not the start state or a final state


        CAREFUL: This method modifies the current automaton, for internal usage
        only!

        The function _create_or_transitions is supposed to be called before
        calling this function
        """
        self._create_or_transitions()
        states = self._states.copy()
        for state in states:
            if (state not in self._start_state
                    and state not in self._final_states):
                self._remove_state(state)

    def _remove_state(self, state: State):
        """ Removes a given state from the epsilon NFA

        CAREFUL: This method modifies the current automaton, for internal usage
        only!

        The function _create_or_transitions is supposed to be called before
        calling this function

        Parameters
        ----------
        state : :class:`~pyformlang.finite_automaton.State`
            The state to remove

        """
        # First compute all endings
        out_transitions = {}
        for symbol in self._input_symbols.union({Epsilon()}):
            out_states = self._transition_function(state, symbol).copy()
            for out_state in out_states:
                out_transitions[out_state] = str(symbol.value)
                self.remove_transition(state, symbol, out_state)
        if state in out_transitions:
            to_itself = "(" + out_transitions[state] + ")*"
            del out_transitions[state]
            for out_state in list(out_transitions.keys()):
                out_transitions[out_state] = to_itself + "." + \
                                             out_transitions[out_state]
        input_symbols = self._input_symbols.copy().union({Epsilon()})
        for in_state in self._states:
            if in_state == state:
                continue
            for symbol in input_symbols:
                out_states = self._transition_function(in_state, symbol)
                if state not in out_states:
                    continue
                symbol_str = "(" + str(symbol.value) + ")"
                self.remove_transition(in_state, symbol, state)
                for out_state, next_symb in out_transitions.items():
                    new_symbol = Symbol(symbol_str + "." + next_symb)
                    self.add_transition(in_state, new_symbol, out_state)
        self._states.remove(state)
        # We make sure the automaton has the good structure
        self._create_or_transitions()

    def minimize(self) -> "DeterministicFiniteAutomaton":
        """ Minimize the current epsilon NFA

        Returns
        ----------
        dfa : :class:`~pyformlang.deterministic_finite_automaton\
        .DeterministicFiniteAutomaton`
            The minimal DFA

        Examples
        --------

        >>> enfa = EpsilonNFA()
        >>> enfa.add_transitions([(0, "abc", 1), (0, "d", 1), \
        (0, "epsilon", 2)])
        >>> enfa.add_start_state(0)
        >>> enfa.add_final_state(1)
        >>> dfa_minimal = enfa.minimize()
        >>> dfa_minimal.is_equivalent(enfa)
        True

        """
        return self.to_deterministic().minimize()

    def _create_or_transitions(self):
        """ Creates a OR transition instead of several connections

        CAREFUL: This method modifies the automaton and is designed for \
        internal use only!
        """
        for state in self._states:
            new_transitions = {}
            input_symbols = self._input_symbols.copy().union({Epsilon()})
            for symbol in input_symbols:
                out_states = self._transition_function(state, symbol)
                out_states = out_states.copy()
                symbol_str = str(symbol.value)
                for out_state in out_states:
                    self.remove_transition(state, symbol, out_state)
                    base = new_transitions.setdefault(out_state, "")
                    if "+" in symbol_str:
                        symbol_str = "(" + symbol_str + ")"
                    if base:
                        new_transitions[out_state] = "((" + base + ")+(" + \
                                                     symbol_str + "))"
                    else:
                        new_transitions[out_state] = symbol_str
            for out_state, next_symb in new_transitions.items():
                self.add_transition(state,
                                    next_symb,
                                    out_state)

    def __bool__(self):
        return not self.is_empty()


def get_temp(start_to_end: str, end_to_start: str, end_to_end: str) \
        -> (str, str):
    """ Gets a temp values in the computation of the simple automaton regex """
    temp = "epsilon"
    if (start_to_end != "epsilon"
            or end_to_end != "epsilon"
            or end_to_start != "epsilon"):
        temp = ""
    if start_to_end != "epsilon":
        temp = start_to_end
    if end_to_end != "epsilon":
        if temp:
            temp += "." + end_to_end + "*"
        else:
            temp = end_to_end + "*"
    part1 = temp
    if not part1:
        part1 = "epsilon"
    if end_to_start != "epsilon":
        if temp:
            temp += "." + end_to_start
        else:
            temp = end_to_start
    if not end_to_start:
        temp = ""
    return (temp, part1)


def get_regex_sub(start_to_start: str,
                  start_to_end: str,
                  end_to_start: str,
                  end_to_end: str) -> str:
    """ Combines the transitions in the regex simple function """
    if not start_to_end:
        return ""
    temp, part1 = get_temp(start_to_end, end_to_start, end_to_end)
    part0 = "epsilon"
    if start_to_start != "epsilon":
        if temp:
            part0 = "(" + start_to_start + "+" + temp + ")*"
        else:
            part0 = "(" + start_to_start + ")*"
    elif temp != "epsilon" and temp:
        part0 = "(" + temp + ")*"
    return "(" + part0 + "." + part1 + ")"


def to_single_state(l_states: Iterable[State]) -> State:
    """ Merge a list of states

    Parameters
    ----------
    l_states : list of :class:`~pyformlang.finite_automaton.State`
        A list of states

    Returns
    ----------
    state : :class:`~pyformlang.finite_automaton.State`
        The merged state
    """
    values = []
    for state in l_states:
        if state is not None:
            values.append(str(state.value))
        else:
            values.append("TRASH")
    values = sorted(values)
    return State(";".join(values))


def combine_state_pair(state0, state1):
    """ Combine two states """
    return State(str(state0.value) + "; " + str(state1.value))
