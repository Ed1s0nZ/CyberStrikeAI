"""
Representation of a nondeterministic finite automaton
"""

from typing import Iterable, Any

# pylint: disable=cyclic-import
from pyformlang.finite_automaton import epsilon
from .epsilon_nfa import EpsilonNFA
from .finite_automaton import to_symbol
from .transition_function import InvalidEpsilonTransition


class NondeterministicFiniteAutomaton(EpsilonNFA):
    """ Represents a nondeterministic finite automaton

    This class represents a nondeterministic finite automaton, where epsilon \
    transition are forbidden.

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
    start_state : :class:`~pyformlang.finite_automaton.State`, optional
        A start state, element of states
    final_states : set of :class:`~pyformlang.finite_automaton.State`, optional
        A set of final or accepting states. It is a subset of states.

    Examples
    --------

    >>> nfa = NondeterministicFiniteAutomaton()

    Creates the NFA.

    >>> nfa.add_transitions([(0, "a", 1), (0, "a", 2)])

    Adds two transitions.

    >>> nfa.add_start_state(0)

    Adds a start state.

    >>> nfa.add_final_state(1)

    Adds a final state.

    >>> nfa.accepts(["a"])
    True

    >>> nfa.is_deterministic()
    False

    """

    def accepts(self, word: Iterable[Any]) -> bool:
        """ Checks whether the nfa accepts a given word

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

        >>> nfa = NondeterministicFiniteAutomaton()
        >>> nfa.add_transitions([(0, "a", 1), (0, "a", 2)])
        >>> nfa.add_start_state(0)
        >>> nfa.add_final_state(1)
        >>> nfa.accepts(["a"])
        True

        """
        word = [to_symbol(x) for x in word]
        current_states = self._start_state
        for symbol in word:
            current_states = self._get_next_states_iterable(current_states,
                                                            symbol)
        return any(self.is_final_state(x) for x in current_states)

    def is_deterministic(self) -> bool:
        """ Checks whether an automaton is deterministic

        Returns
        ----------
        is_deterministic : bool
           Whether the automaton is deterministic

        Examples
        --------

        >>> nfa = NondeterministicFiniteAutomaton()
        >>> nfa.add_transitions([(0, "a", 1), (0, "a", 2)])
        >>> nfa.add_start_state(0)
        >>> nfa.add_final_state(1)
        >>> nfa.is_deterministic()
        False

        """
        return len(self._start_state) <= 1 and \
            self._transition_function.is_deterministic()

    def to_deterministic(self) -> "DeterministicFiniteAutomaton":
        """ Transforms the nfa into a dfa

        Returns
        ----------
        dfa :  :class:`~pyformlang.deterministic_finite_automaton\
        .DeterministicFiniteAutomaton`
            A dfa equivalent to the current nfa

        Examples
        --------

        >>> nfa = NondeterministicFiniteAutomaton()
        >>> nfa.add_transitions([(0, "a", 1), (0, "a", 2)])
        >>> nfa.add_start_state(0)
        >>> nfa.add_final_state(1)
        >>> dfa = nfa.to_deterministic()
        >>> nfa.is_equivalent_to(dfa)
        True

        """
        return self._to_deterministic_internal(False)

    def add_transition(self,
                       s_from: Any,
                       symb_by: Any,
                       s_to: Any) -> int:
        if symb_by == epsilon.Epsilon():
            raise InvalidEpsilonTransition
        return super().add_transition(s_from, symb_by, s_to)
