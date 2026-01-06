""" A general finite automaton representation """

from typing import List, Iterable, Set, Optional, Union, Any
from collections import deque

import networkx as nx
from networkx.drawing.nx_pydot import write_dot

from pyformlang.fst import FST
# pylint: disable=cyclic-import
from pyformlang import finite_automaton

from .epsilon import Epsilon
from .state import State
from .symbol import Symbol


class FiniteAutomaton:
    """ Represents a general finite automaton


    Attributes
    ----------
    _states : set of :class:`~pyformlang.finite_automaton.State`, optional
        A finite set of states
    _input_symbols : set of :class:`~pyformlang.finite_automaton.Symbol`, \
     optional
        A finite set of input symbols
    _transition_function :  \
    :class:`~pyformlang.finite_automaton.NondeterministicTransitionFunction`\
    , optional
        Takes as arguments a state and an input symbol and returns a state.
    _start_state : set of :class:`~pyformlang.finite_automaton.State`, optional
        A start state, element of states
    _final_states : set of :class:`~pyformlang.finite_automaton.State`, \
     optional
        A set of final or accepting states. It is a subset of states.
    """

    def __init__(self):
        self._states = set()
        self._input_symbols = set()
        self._transition_function = None
        self._start_state = set()
        self._final_states = set()

    def add_transition(self, s_from: Any, symb_by: Any,
                       s_to: Any) -> int:
        """ Adds a transition to the nfa

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

        >>> enfa = EpsilonNFA()
        >>> enfa.add_transition(0, "abc", 1)

        """
        s_from = to_state(s_from)
        symb_by = to_symbol(symb_by)
        s_to = to_state(s_to)
        temp = self._transition_function.add_transition(s_from, symb_by, s_to)
        self._states.add(s_from)
        self._states.add(s_to)
        if symb_by != Epsilon():
            self._input_symbols.add(symb_by)
        return temp

    def add_transitions(self, transitions_list):
        """
        Adds several transitions to the automaton

        Parameters
        ----------
        transitions_list : list of triples of (s_from, symb_by, s_to)
            A list of all the transitions represented as triples as they \
            would be used in add_transition

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

        >>> enfa = EpsilonNFA()
        >>> enfa.add_transitions([(0, "abc", 1), (0, "d", 1), \
        (0, "epsilon", 2)])


        """
        temp = 0
        for s_from, symb_by, s_to in transitions_list:
            temp = self.add_transition(s_from, symb_by, s_to)
        return temp

    def remove_transition(self, s_from: State, symb_by: Symbol,
                          s_to: State) -> int:
        """ Remove a transition of the nfa

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
            1 if the transition existed, 0 otherwise

        Examples
        --------

        >>> enfa = EpsilonNFA()
        >>> enfa.add_transition(0, "abc", 1)
        >>> enfa.remove_transition(0, "abc", 1)

        """
        s_from = to_state(s_from)
        symb_by = to_symbol(symb_by)
        s_to = to_state(s_to)
        return self._transition_function.remove_transition(s_from,
                                                           symb_by,
                                                           s_to)

    @property
    def states(self):
        """ Gives the states

        Returns
        ----------
        states : set of :class:`~pyformlang.finite_automaton.State`
            The states
        """
        return self._states

    def get_number_transitions(self) -> int:
        """ Gives the number of transitions

        Returns
        ----------
        n_transitions : int
            The number of deterministic transitions

        Examples
        --------

        >>> enfa = EpsilonNFA()
        >>> enfa.add_transitions([(0, "abc", 1), (0, "d", 1), \
        (0, "epsilon", 2)])
        >>> enfa.get_number_transitions()
        3

        """
        return self._transition_function.get_number_transitions()

    @property
    def symbols(self):
        """The symbols"""
        return self._input_symbols

    @property
    def final_states(self):
        """The final states"""
        return self._final_states

    def add_start_state(self, state: Any) -> int:
        """ Set an initial state

        Parameters
        -----------
        state : :class:`~pyformlang.finite_automaton.State`
            The new initial state

        Returns
        ----------
        done : int
            1 is correctly added

        Examples
        --------

        >>> enfa = EpsilonNFA()
        >>> enfa.add_transitions([(0, "abc", 1), (0, "d", 1), \
        (0, "epsilon", 2)])
        >>> enfa.add_start_state(0)

        """
        state = to_state(state)
        self._start_state.add(state)
        self._states.add(state)
        return 1

    def remove_start_state(self, state: State) -> int:
        """ remove an initial state

        Parameters
        -----------
        state : :class:`~pyformlang.finite_automaton.State`
            The new initial state

        Returns
        ----------
        done : int
            1 is correctly added

        Examples
        --------

        >>> enfa = EpsilonNFA()
        >>> enfa.add_transitions([(0, "abc", 1), (0, "d", 1), \
        (0, "epsilon", 2)])
        >>> enfa.add_start_state(0)
        >>> enfa.remove_start_state(0)

        """
        state = to_state(state)
        if state in self._start_state:
            self._start_state.remove(state)
            return 1
        return 0

    def add_final_state(self, state: Any) -> int:
        """ Adds a new final state

        Parameters
        -----------
        state : :class:`~pyformlang.finite_automaton.State`
            A new final state

        Returns
        ----------
        done : int
            1 is correctly added

        Examples
        --------

        >>> enfa = EpsilonNFA()
        >>> enfa.add_transitions([(0, "abc", 1), (0, "d", 1), \
        (0, "epsilon", 2)])
        >>> enfa.add_start_state(0)
        >>> enfa.add_final_state(1)

        """
        state = to_state(state)
        self._final_states.add(state)
        self._states.add(state)
        return 1

    def remove_final_state(self, state: State) -> int:
        """ Remove a final state

        Parameters
        -----------
        state : :class:`~pyformlang.finite_automaton.State`
            A final state to remove

        Returns
        ----------
        done : int
            0 if it was not a final state, 1 otherwise

        Examples
        --------

        >>> enfa = EpsilonNFA()
        >>> enfa.add_transitions([(0, "abc", 1), (0, "d", 1), \
        (0, "epsilon", 2)])
        >>> enfa.add_start_state(0)
        >>> enfa.add_final_state(1)
        >>> enfa.remove_final_state(1)
        """
        state = to_state(state)
        if self.is_final_state(state):
            self._final_states.remove(state)
            return 1
        return 0

    def __call__(self, state: Any, symbol: Any = None) -> List[State]:
        """ Gives the states obtained after calling a symbol on a state
        Calls the transition function

        Parameters
        -----------
        state : :class:`~pyformlang.finite_automaton.State`
            The source state
        symbol : :class:`~pyformlang.finite_automaton.Symbol`
            The symbol, optional if we want all transitions

        Returns
        ----------
        states : list of :class:`~pyformlang.finite_automaton.State`
            The next states

        Examples
        --------

        >>> enfa = EpsilonNFA()
        >>> enfa.add_transitions([(0, "abc", 1), (0, "d", 1), \
        (0, "epsilon", 2)])
        >>> enfa(0, "abc")
        [1]

        """
        # pylint: disable=not-callable
        state = to_state(state)
        if symbol is not None:
            symbol = to_symbol(symbol)
        return self._transition_function(state, symbol)

    def is_final_state(self, state: State) -> bool:
        """ Checks if a state is final

        Parameters
        -----------
        state : :class:`~pyformlang.finite_automaton.State`
            The state to check

        Returns
        ----------
        is_final : bool
            Whether the state is final or not

        Examples
        --------

        >>> enfa = EpsilonNFA()
        >>> enfa.add_transitions([(0, "abc", 1), (0, "d", 1), \
        (0, "epsilon", 2)])
        >>> enfa.add_start_state(0)
        >>> enfa.add_final_state(1)
        >>> enfa.is_final_state(1)
        True

        """
        state = to_state(state)
        return state in self._final_states

    @property
    def start_states(self):
        """The start states"""
        return self._start_state

    def add_symbol(self, symbol: Symbol):
        """ Add a symbol

        Parameters
        -----------
        symbol : :class:`~pyformlang.finite_automaton.Symbol`
            The symbol

        Examples
        --------

        >>> enfa = EpsilonNFA()
        >>> enfa.add_symbol("a")

        """
        symbol = to_symbol(symbol)
        self._input_symbols.add(symbol)

    def to_fst(self) -> "FST":
        """ Turns the finite automaton into a finite state transducer

        The transducers accepts only the words in the language of the \
        automaton and output the input word

        Returns
        ----------
        fst : :class:`~pyformlang.fst.FST`
            The equivalent FST

        Examples
        --------

        >>> enfa = EpsilonNFA()
        >>> fst = enfa.to_fst()
        >>> fst.states
        {}

        """
        fst = FST()
        for start_state in self._start_state:
            fst.add_start_state(start_state.value)
        for final_state in self._final_states:
            fst.add_final_state(final_state.value)
        for s_from, symb_by, s_to in self._transition_function.get_edges():
            fst.add_transition(s_from.value,
                               symb_by.value,
                               s_to.value,
                               [symb_by.value])
        return fst

    def is_acyclic(self) -> bool:
        """
        Checks if the automaton is acyclic

        Returns
        -------
        is_acyclic : bool
            Whether the automaton is acyclic or not

        Examples
        --------

        >>> enfa = EpsilonNFA()
        >>> enfa.add_transitions([(0, "abc", 1), (0, "d", 1), \
        (0, "epsilon", 2)])
        >>> enfa.add_start_state(0)
        >>> enfa.add_final_state(1)
        >>> enfa.is_acyclic()
        True

        """
        to_process = []
        for state in self._start_state:
            to_process.append((state, set()))
        while to_process:
            current, visited = to_process.pop()
            if current in visited:
                return False
            visited.add(current)
            for symbol in self._input_symbols:
                for state in self(current, symbol):
                    to_process.append((state, visited.copy()))
            # Epsilon
            for state in self(current, Epsilon()):
                to_process.append((state, visited.copy()))
        return True

    def to_networkx(self) -> nx.MultiDiGraph:
        """
        Transform the current automaton into a networkx graph

        Returns
        -------
        graph :  networkx.MultiDiGraph
            A networkx MultiDiGraph representing the automaton

        Examples
        --------

        >>> enfa = EpsilonNFA()
        >>> enfa.add_transitions([(0, "abc", 1), (0, "d", 1), \
        (0, "epsilon", 2)])
        >>> enfa.add_start_state(0)
        >>> enfa.add_final_state(1)
        >>> graph = enfa.to_networkx()

        """
        graph = nx.MultiDiGraph()
        for state in self._states:
            graph.add_node(state.value,
                           is_start=state in self.start_states,
                           is_final=state in self.final_states,
                           peripheries=2 if state in self.final_states else 1,
                           label=state.value)
            if state in self.start_states:
                add_start_state_to_graph(graph, state)
        for s_from, symbol, s_to in self._transition_function.get_edges():
            label_ = symbol.value
            if label_ == 'epsilon':
                label_ = 'ɛ'
            graph.add_edge(s_from.value, s_to.value, label=label_)
        return graph

    @classmethod
    def from_networkx(cls, graph):
        """
        Import a networkx graph into an finite state automaton. \
        The imported graph requires to have the good format, i.e. to come \
        from the function to_networkx

        Parameters
        ----------
        graph :
            The graph representation of the automaton

        Returns
        -------
        enfa :
            A epsilon nondeterministic finite automaton read from the graph

        TODO
        -------
        * We lose the type of the node value if going through a dot file
        * Explain the format

        Examples
        --------

        >>> enfa = EpsilonNFA()
        >>> enfa.add_transitions([(0, "abc", 1), (0, "d", 1), \
        (0, "epsilon", 2)])
        >>> enfa.add_start_state(0)
        >>> enfa.add_final_state(1)
        >>> graph = enfa.to_networkx()
        >>> enfa_from_nx = EpsilonNFA.from_networkx(graph)

        """
        enfa = finite_automaton.EpsilonNFA()
        for s_from in graph:
            for s_to in graph[s_from]:
                for transition in graph[s_from][s_to].values():
                    if "label" in transition:
                        enfa.add_transition(s_from,
                                            transition["label"],
                                            s_to)
        for node in graph.nodes:
            if graph.nodes[node].get("is_start", False):
                enfa.add_start_state(node)
            if graph.nodes[node].get("is_final", False):
                enfa.add_final_state(node)
        return enfa

    def write_as_dot(self, filename):
        """
        Write the automaton in dot format into a file

        Parameters
        ----------
        filename : str
            The filename where to write the dot file

        Examples
        --------

        >>> enfa = EpsilonNFA()
        >>> enfa.add_transitions([(0, "abc", 1), (0, "d", 1), \
        (0, "epsilon", 2)])
        >>> enfa.add_start_state(0)
        >>> enfa.add_final_state(1)
        >>> enfa.write_as_dot("enfa.dot")
        """
        write_dot(self.to_networkx(), filename)

    def is_equivalent_to(self, other):
        """
        Checks if the current automaton is equivalent to a given one.

        Parameters
        ----------
        other :
            An other finite state automaton

        Returns
        -------
        is_equivalent : bool
            Whether the two automata are equivalent or not

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

        """
        self_dfa = self.to_deterministic()
        return self_dfa.is_equivalent_to(other)

    def get_accepted_words(self, max_length: Optional[int] = None) \
            -> Iterable[List[Symbol]]:
        """
        Gets words accepted by the finite automaton.
        """
        if max_length is not None and max_length < 0:
            return []
        states_to_visit = deque((start_state, [])
                                for start_state in self.start_states)
        states_leading_to_final = self._get_states_leading_to_final()
        words_by_state = {state: set() for state in self.states}
        yielded_words = set()
        while states_to_visit:
            current_state, current_word = states_to_visit.popleft()
            if max_length is not None and len(current_word) > max_length:
                continue
            word_to_add = tuple(current_word)
            if not self.__try_add(words_by_state[current_state], word_to_add):
                continue
            transitions = self._transition_function.get_transitions_from(
                current_state)
            for symbol, next_state in transitions:
                if next_state in states_leading_to_final:
                    temp_word = current_word.copy()
                    if symbol != Epsilon():
                        temp_word.append(symbol)
                    states_to_visit.append((next_state, temp_word))
            if self.is_final_state(current_state):
                if self.__try_add(yielded_words, word_to_add):
                    yield current_word

    def _get_states_leading_to_final(self) -> Set[State]:
        """
        Gets a set of states from which one
        of the final states can be reached.
        """
        leading_to_final = self.final_states.copy()
        visited = set()
        states_to_process = deque((None, start_state)
                                  for start_state in self.start_states)
        delayed_states = deque()
        while states_to_process:
            previous_state, current_state = states_to_process.pop()
            if previous_state and current_state in leading_to_final:
                leading_to_final.add(previous_state)
                continue
            if current_state in visited:
                delayed_states.append((previous_state, current_state))
                continue
            visited.add(current_state)
            next_states = self._get_next_states_from(current_state)
            if next_states:
                states_to_process.append((previous_state, current_state))
                for next_state in next_states:
                    states_to_process.append((current_state, next_state))
        for previous_state, current_state in delayed_states:
            if previous_state and current_state in leading_to_final:
                leading_to_final.add(previous_state)
        return leading_to_final

    def _get_reachable_states(self) -> Set[State]:
        """ Get all states which are reachable """
        visited = set()
        states_to_process = deque(self.start_states)
        while states_to_process:
            current_state = states_to_process.popleft()
            visited.add(current_state)
            for next_state in self._get_next_states_from(current_state):
                if next_state not in visited:
                    states_to_process.append(next_state)
        return visited

    def _get_next_states_from(self, state_from: State) -> Set[State]:
        """ Gets a set of states that are next to the given one """
        next_states = set()
        for _, next_state in \
                self._transition_function.get_transitions_from(state_from):
            next_states.add(next_state)
        return next_states

    def to_deterministic(self):
        """ Turns the automaton into a deterministic one"""
        raise NotImplementedError

    def is_deterministic(self):
        """ Checks if the automaton is deterministic """
        raise NotImplementedError

    def __eq__(self, other):
        return self.is_equivalent_to(other)

    def __len__(self):
        """Number of transitions"""
        return len(self._transition_function)

    def __iter__(self):
        yield from self._transition_function.__iter__()

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

        Examples
        --------

        >>> enfa = EpsilonNFA()
        >>> enfa.add_transitions([(0, "abc", 1), (0, "d", 1), \
        (0, "epsilon", 2)])
        >>> enfa.add_start_state(0)
        >>> enfa.add_final_state(1)
        >>> enfa_dict = enfa.to_dict()

        """
        return self._transition_function.to_dict()

    @staticmethod
    def __try_add(set_to_add_to: Set[Any], element_to_add: Any) -> bool:
        """
        Tries to add a given element to the given set.
        Returns True if element was added, otherwise False.
        """
        initial_length = len(set_to_add_to)
        set_to_add_to.add(element_to_add)
        return len(set_to_add_to) != initial_length


def to_state(given: Any) -> Union[State, None]:
    """ Transforms the input into a state

    Parameters
    ----------
    given : any
        What we want to transform
    """
    if given is None:
        return None
    if isinstance(given, State):
        return given
    return State(given)


def to_symbol(given: Any) -> Symbol:
    """ Transforms the input into a symbol

    Parameters
    ----------
    given : any
        What we want to transform
    """
    if isinstance(given, Symbol):
        return given
    if given in ("epsilon", "ɛ"):
        return Epsilon()
    return Symbol(given)


def add_start_state_to_graph(graph, state):
    """ Adds a starting node to a given graph """
    graph.add_node("starting_" + str(state.value),
                   label="",
                   shape=None,
                   height=.0,
                   width=.0)
    graph.add_edge("starting_" + str(state.value),
                   state.value)
