"""
Representation of a transition function
"""
import copy
from typing import List, Iterable, Tuple, Any

from pyformlang.finite_automaton.epsilon import Epsilon

from .state import State
from .symbol import Symbol


class InvalidEpsilonTransition(Exception):
    """Exception raised when an epsilon transition is created in
    deterministic automaton"""


class TransitionFunction:
    """ A transition function in a finite automaton.

    This is a deterministic transition function.

    Attributes
    ----------
    _transitions : dict
        A dictionary which contains the transitions of a finite automaton

    Examples
    --------

    >>> transition = TransitionFunction()
    >>> transition.add_transition(State(0), Symbol("a"), State(1))

    Creates a transition function and adds a transition.

    """

    def __init__(self):
        self._transitions = {}

    def add_transition(self, s_from: Any, symb_by: Any,
                       s_to: Any) -> int:
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

        Raises
        --------
        DuplicateTransitionError
            If the transition already exists

        Examples
        --------

        >>> transition = TransitionFunction()
        >>> transition.add_transition(State(0), Symbol("a"), State(1))

        """
        if symb_by == Epsilon():
            raise InvalidEpsilonTransition()
        if s_from in self._transitions:
            if symb_by in self._transitions[s_from]:
                if self._transitions[s_from][symb_by] != s_to:
                    raise DuplicateTransitionError(s_from,
                                                   symb_by,
                                                   s_to,
                                                   self._transitions[s_from][
                                                       symb_by])
            else:
                self._transitions[s_from][symb_by] = s_to
        else:
            self._transitions[s_from] = {}
            self._transitions[s_from][symb_by] = s_to
        return 1

    # pylint: disable=duplicate-code
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

        >>> transition = TransitionFunction()
        >>> transition.add_transition(State(0), Symbol("a"), State(1))
        >>> transition.remove_transition(State(0), Symbol("a"), State(1))

        """
        if s_from in self._transitions and \
                symb_by in self._transitions[s_from] and \
                s_to == self._transitions[s_from][symb_by]:
            del self._transitions[s_from][symb_by]
            return 1
        return 0

    def __call__(self, s_from: State, symb_by: Symbol = None) -> List[State]:
        """ Calls the transition function as a real function

        Parameters
        ----------
        s_from : :class:`~pyformlang.finite_automaton.State`
            The source state
        symb_by : :class:`~pyformlang.finite_automaton.Symbol`
            The transition symbol

        Returns
        ----------
        s_from : list of :class:`~pyformlang.finite_automaton.State`
            The destination state, in a list

        """
        if s_from in self._transitions:
            if symb_by is not None:
                if symb_by in self._transitions[s_from]:
                    return [self._transitions[s_from][symb_by]]
            else:
                return self._transitions[s_from].items()
        return []

    def get_number_transitions(self) -> int:
        """ Gives the number of transitions describe by the deterministic \
        function

        Returns
        ----------
        n_transitions : int
            The number of deterministic transitions

        Examples
        --------

        >>> transition = TransitionFunction()
        >>> transition.add_transition(State(0), Symbol("a"), State(1))
        >>> transition.get_number_transitions()
        1

        """
        return sum(len(x) for x in self._transitions.values())

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
                yield state, symbol, next_states

    def __len__(self):
        return self.get_number_transitions()

    def __iter__(self):
        yield from self.get_edges()

    def to_dict(self):
        """
        Get the dictionary representation of the transition function. The \
        keys of the dictionary are the source nodes. The items are \
        dictionaries where the keys are the symbols of the transitions and \
        the items are the set of target nodes.

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
            yield from self._transitions[state_from].items()


class DuplicateTransitionError(Exception):
    """ Signals a duplicated transition

    Parameters
    ----------
    s_from : :class:`~pyformlang.finite_automaton.State`
        The source state
    symb_by : :class:`~pyformlang.finite_automaton.Symbol`
        The transition symbol
    s_to : :class:`~pyformlang.finite_automaton.State`
        The wanted new destination state
    s_to_old : :class:`~pyformlang.finite_automaton.State`
        The old destination state

    """

    def __init__(self,
                 s_from: State,
                 symb_by: Symbol,
                 s_to: State,
                 s_to_old: State):
        super().__init__("Transition from " + str(s_from) +
                         " by " + str(symb_by) +
                         " goes to " + str(s_to_old) + " not " + str(s_to))
        self.s_from = s_from
        self.symb_by = symb_by
        self.s_to = s_to
        self.s_to_old = s_to_old
