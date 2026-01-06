""" A transition function in a pushdown automaton """

from typing import List

from .stack_symbol import StackSymbol
from .state import State
from .symbol import Symbol


class TransitionFunction:
    """ A transition function in a pushdown automaton """

    def __init__(self):
        self._transitions = {}
        self._iter_key = None
        self._current_key = None
        self._iter_inside = None

    def get_number_transitions(self):
        """ Gets the number of transitions

        Returns
        ----------
        n_transitions : int
            The number of transitions
        """
        return sum(len(x) for x in self._transitions.values())

    # pylint: disable=too-many-arguments
    def add_transition(self,
                       s_from: State,
                       input_symbol: Symbol,
                       stack_from: StackSymbol,
                       s_to: State,
                       stack_to: List[StackSymbol]):
        """ Add a transition to the function

        Parameters
        ----------
        s_from : :class:`~pyformlang.pda.State`
            The starting symbol
        input_symbol : :class:`~pyformlang.pda.Symbol`
            The input symbol
        stack_from : :class:`~pyformlang.pda.StackSymbol`
            The stack symbol of the transition
        s_to : :class:`~pyformlang.pda.State`
            The new state
        stack_to : list of :class:`~pyformlang.pda.StackSymbol`
            The string of stack symbol which replace the stack_from
        """
        temp_in = (s_from, input_symbol, stack_from)
        temp_out = (s_to, tuple(stack_to))
        if temp_in in self._transitions:
            self._transitions[temp_in].add(temp_out)
        else:
            self._transitions[temp_in] = {temp_out}

    def copy(self) -> "TransitionFunction":
        """ Copy the current transition function

        Returns
        ----------
        new_tf : :class:`~pyformlang.pda.TransitionFunction`
            The copy of the transition function
        """
        new_tf = TransitionFunction()
        for temp_in, transition in self._transitions.items():
            for temp_out in transition:
                new_tf.add_transition(temp_in[0], temp_in[1], temp_in[2],
                                      temp_out[0], temp_out[1])
        return new_tf

    def __iter__(self):
        self._iter_key = iter(self._transitions.keys())
        self._current_key = None
        self._iter_inside = None
        return self

    def __next__(self):
        if self._iter_inside is None:
            next_key = next(self._iter_key)
            self._current_key = next_key
            self._iter_inside = iter(self._transitions[next_key])
        try:
            next_value = next(self._iter_inside)
            return self._current_key, next_value
        except StopIteration:
            next_key = next(self._iter_key)
            self._current_key = next_key
            self._iter_inside = iter(self._transitions[next_key])
            return next(self)

    def __call__(self, s_from: State,
                 input_symbol: Symbol,
                 stack_from: StackSymbol):
        return self._transitions.get((s_from, input_symbol, stack_from), {})

    def to_dict(self):
        """Get the dictionary representation of the transitions"""
        return self._transitions
