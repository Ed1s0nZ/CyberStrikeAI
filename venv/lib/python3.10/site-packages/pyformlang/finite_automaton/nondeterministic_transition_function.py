"""
A nondeterministic transition function
"""
import copy
from typing import Set, Iterable, Tuple

from .state import State
from .symbol import Symbol


class NondeterministicTransitionFunction:
    """ A nondeterministic transition function in a finite automaton.

    The difference with a deterministic transition is that the return value is
    a set of States

    Examples
    --------

    >>> transition = NondeterministicTransitionFunction()
    >>> transition.add_transition(State(0), Symbol("a"), State(1))

    Creates a transition function and adds a transition.

    """

    def __init__(self):
        self._transitions = {}

    def add_transition(self, s_from: State, symb_by: Symbol,
                       s_to: State) -> int:
        """ Adds a new transition to the function

        Parameters
        ----------
        s_from : :class:`~pyformlang.finite_automaton.State`
            The source state
        symb_by : :class:`~pyformlang.finite_automaton.Symbol`
            The transition symbol
        s_to : :class:`~pyformlang.finite_automaton.State`
            The destination state


        Returns
        --------
        done : int
            Always 1

        Examples
        --------

        >>> transition = NondeterministicTransitionFunction()
        >>> transition.add_transition(State(0), Symbol("a"), State(1))

        """
        if s_from in self._transitions:
            if symb_by in self._transitions[s_from]:
                self._transitions[s_from][symb_by].add(s_to)
            else:
                self._transitions[s_from][symb_by] = {s_to}
        else:
            self._transitions[s_from] = {}
            self._transitions[s_from][symb_by] = {s_to}
        return 1

    def remove_transition(self, s_from: State, symb_by: Symbol,
                          s_to: State) -> int:
        """ Removes a transition to the function

        Parameters
        ----------
        s_from : :class:`~pyformlang.finite_automaton.State`
            The source state
        symb_by : :class:`~pyformlang.finite_automaton.Symbol`
            The transition symbol
        s_to : :class:`~pyformlang.finite_automaton.State`
            The destination state


        Returns
        --------
        done : int
            1 is the transition was found, 0 otherwise

        Examples
        --------

        >>> transition = NondeterministicTransitionFunction()
        >>> transition.add_transition(State(0), Symbol("a"), State(1))
        >>> transition.remove_transition(State(0), Symbol("a"), State(1))

        """
        if s_from in self._transitions and \
                symb_by in self._transitions[s_from] and \
                s_to in self._transitions[s_from][symb_by]:
            self._transitions[s_from][symb_by].remove(s_to)
            return 1
        return 0

    def get_number_transitions(self) -> int:
        """ Gives the number of transitions describe by the function

        Returns
        ----------
        n_transitions : int
            The number of transitions

        Examples
        --------

        >>> transition = NondeterministicTransitionFunction()
        >>> transition.add_transition(State(0), Symbol("a"), State(1))
        >>> transition.get_number_transitions()
        1

        """
        counter = 0
        for transitions in self._transitions.values():
            for s_to in transitions.values():
                counter += len(s_to)
        return counter

    def __len__(self):
        return self.get_number_transitions()

    def __call__(self, s_from: State, symb_by: Symbol = None) -> Set[State]:
        """ Calls the transition function as a real function

        Parameters
        ----------
        s_from : :class:`~pyformlang.finite_automaton.State`
            The source state
        symb_by : :class:`~pyformlang.finite_automaton.Symbol`
            The transition symbol

        Returns
        ----------
        s_from : :class:`~pyformlang.finite_automaton.State` or None
            The destination state or None if it does not exists

        """
        if s_from in self._transitions:
            if symb_by is not None:
                if symb_by in self._transitions[s_from]:
                    return self._transitions[s_from][symb_by]
            else:
                return self._transitions[s_from].items()
        return set()

    def is_deterministic(self):
        """ Whether the transition function is deterministic

        Returns
        ----------
        is_deterministic : bool
            Whether the function is deterministic

        Examples
        --------

        >>> transition = NondeterministicTransitionFunction()
        >>> transition.add_transition(State(0), Symbol("a"), State(1))
        >>> transition.is_deterministic()
        True

        """
        for transitions in self._transitions.values():
            for s_to in transitions.values():
                if len(s_to) > 1:
                    return False
        return True

    def get_edges(self):
        """ Gets the edges

        Returns
        ----------
        edges : generator of (:class:`~pyformlang.finite_automaton.State`, \
            :class:`~pyformlang.finite_automaton.Symbol`,\
            :class:`~pyformlang.finite_automaton.State`)
            A generator of edges
        """
        for state, transitions in self._transitions.items():
            for symbol, next_states in transitions.items():
                for next_state in next_states:
                    yield state, symbol, next_state

    def __iter__(self):
        yield from self.get_edges()

    def to_dict(self):
        """
        Get the dictionary representation of the transition function. The keys
        of the dictionary are the source nodes. The items are dictionaries
        where the keys are the symbols of the transitions and the items are
        the set of target nodes.

        Returns
        -------
        transition_dict : dict
            The transitions as a dictionary.
        """
        return copy.deepcopy(self._transitions)

    def get_transitions_from(self, state_from: State) \
            -> Iterable[Tuple[Symbol, State]]:
        """ Gets transitions from the given state """
        if state_from in self._transitions:
            for symb_by, states_to in self._transitions[state_from].items():
                for state_to in states_to:
                    yield symb_by, state_to
