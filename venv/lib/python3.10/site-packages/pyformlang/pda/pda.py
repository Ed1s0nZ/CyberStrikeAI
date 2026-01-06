""" We represent here a push-down automaton """
import json
from itertools import product
from typing import AbstractSet, List, Iterable, Any

import networkx as nx
import numpy as np
from networkx.drawing.nx_pydot import write_dot

from pyformlang import cfg
from pyformlang import finite_automaton
from pyformlang import regular_expression
from pyformlang.pda.cfg_variable_converter import CFGVariableConverter
from .epsilon import Epsilon
from .stack_symbol import StackSymbol
from .state import State
from .transition_function import TransitionFunction
from .utils import PDAObjectCreator
from ..finite_automaton import FiniteAutomaton
from ..finite_automaton.finite_automaton import add_start_state_to_graph

INPUT_SYMBOL = 1

STACK_FROM = 2

INPUT = 0

STATE = 0

NEW_STACK = 1

OUTPUT = 1


class PDA:
    """ Representation of a pushdown automaton

    Parameters
    ----------
    states : set of :class:`~pyformlang.pda.State`, optional
        A finite set of states
    input_symbols : set of :class:`~pyformlang.pda.Symbol`, optional
        A finite set of input symbols
    stack_alphabet : set of :class:`~pyformlang.pda.StackSymbol`, optional
        A finite stack alphabet
    transition_function : :class:`~pyformlang.pda.TransitionFunction`, optional
        Takes as arguments a state, an input symbol and a stack symbol and
        returns a state and a string of stack symbols push on the stacked to
        replace X
    start_state : :class:`~pyformlang.pda.State`, optional
        A start state, element of states
    start_stack_symbol : :class:`~pyformlang.pda.StackSymbol`, optional
        The stack is initialized with this stack symbol
    final_states : set of :class:`~pyformlang.pda.State`, optional
        A set of final or accepting states. It is a subset of states.
    """

    # pylint: disable=too-many-instance-attributes

    def __init__(self,
                 states: AbstractSet[Any] = None,
                 input_symbols: AbstractSet[Any] = None,
                 stack_alphabet: AbstractSet[Any] = None,
                 transition_function: TransitionFunction = None,
                 start_state: Any = None,
                 start_stack_symbol: Any = None,
                 final_states: AbstractSet[Any] = None):
        # pylint: disable=too-many-arguments
        self._pda_obj_creator = PDAObjectCreator()
        if states is not None:
            states = {self._pda_obj_creator.to_state(x) for x in states}
        if input_symbols is not None:
            input_symbols = {self._pda_obj_creator.to_symbol(x)
                             for x in input_symbols}
        if stack_alphabet is not None:
            stack_alphabet = {self._pda_obj_creator.to_stack_symbol(x)
                              for x in stack_alphabet}
        if start_state is not None:
            start_state = self._pda_obj_creator.to_state(start_state)
        if start_stack_symbol is not None:
            start_stack_symbol = \
                self._pda_obj_creator.to_stack_symbol(start_stack_symbol)
        if final_states is not None:
            final_states = {self._pda_obj_creator.to_state(x)
                            for x in final_states}
        self._states = states or set()
        self._states = set(self._states)
        self._input_symbols = input_symbols or set()
        self._input_symbols = set(self._input_symbols)
        self._stack_alphabet = stack_alphabet or set()
        self._stack_alphabet = set(self._stack_alphabet)
        self._transition_function = transition_function or TransitionFunction()
        self._start_state = start_state
        if start_state is not None:
            self._states.add(start_state)
        self._start_stack_symbol = start_stack_symbol
        if start_stack_symbol is not None:
            self._stack_alphabet.add(start_stack_symbol)
        self._final_states = final_states or set()
        self._final_states = set(self._final_states)
        for state in self._final_states:
            self._states.add(state)
        self._cfg_variable_converter = None

    def set_start_state(self, start_state: Any):
        """ Sets the start state to the automaton

        Parameters
        ----------
        start_state : :class:`~pyformlang.pda.State`
            The start state
        """
        start_state = self._pda_obj_creator.to_state(start_state)
        self._states.add(start_state)
        self._start_state = start_state

    def set_start_stack_symbol(self, start_stack_symbol: Any):
        """ Sets the start stack symbol to the automaton

        Parameters
        ----------
        start_stack_symbol : :class:`~pyformlang.pda.StackSymbol`
            The start stack symbol
        """
        start_stack_symbol = self._pda_obj_creator.to_stack_symbol(
            start_stack_symbol)
        self._stack_alphabet.add(start_stack_symbol)
        self._start_stack_symbol = start_stack_symbol

    def add_final_state(self, state: Any):
        """ Adds a final state to the automaton

        Parameters
        ----------
        state : :class:`~pyformlang.pda.State`
            The state to add
        """
        state = self._pda_obj_creator.to_state(state)
        self._final_states.add(state)

    @property
    def start_state(self):
        """ Get start state """
        return self._start_state

    @property
    def states(self):
        """
        Get the states fo the PDA
        Returns
        -------
        states : iterable of :class:`~pyformlang.pda.State`
            The states of the PDA
        """
        return self._states

    @property
    def final_states(self):
        """
        The final states of the PDA
        Returns
        -------
        final_states : iterable of :class:`~pyformlang.pda.State`
            The final states of the PDA

        """
        return self._final_states

    @property
    def input_symbols(self):
        """
        The input symbols of the PDA

        Returns
        -------
        input_symbols : iterable of :class:`~pyformlang.pda.Symbol`
            The input symbols of the PDA
        """
        return self._input_symbols

    @property
    def stack_symbols(self):
        """
        The stack symbols of the PDA

        Returns
        -------
        stack_symbols : iterable of :class:`~pyformlang.pda.StackSymbol`
            The stack symbols of the PDA
        """
        return self._stack_alphabet

    def get_number_transitions(self) -> int:
        """ Gets the number of transitions in the PDA

        Returns
        ----------
        n_transitions : int
            The number of transitions
        """
        return self._transition_function.get_number_transitions()

    def add_transitions(self, transitions):
        """
        Adds several transitions

        Parameters
        ----------
        transitions :
            Transitions as they would be given to add_transition
        """
        for s_from, input_symbol, stack_from, s_to, stack_to in transitions:
            self.add_transition(s_from, input_symbol, stack_from,
                                s_to, stack_to)

    def add_transition(self,
                       s_from: Any,
                       input_symbol: Any,
                       stack_from: Any,
                       s_to: Any,
                       stack_to: Iterable[Any]):
        """ Add a transition to the PDA

        Parameters
        ----------
        s_from : :class:`~pyformlang.pda.State`
            The starting symbol
        input_symbol : :class:`~pyformlang.pda.Symbol`
            The input symbol for the transition
        stack_from : :class:`~pyformlang.pda.StackSymbol`
            The stack symbol of the transition
        s_to : :class:`~pyformlang.pda.State`
            The new state
        stack_to : list of :class:`~pyformlang.pda.StackSymbol`
            The string of stack symbol which replace the stack_from
        """
        # pylint: disable=too-many-arguments
        s_from = self._pda_obj_creator.to_state(s_from)
        input_symbol = self._pda_obj_creator.to_symbol(input_symbol)
        stack_from = self._pda_obj_creator.to_stack_symbol(stack_from)
        s_to = self._pda_obj_creator.to_state(s_to)
        stack_to = [self._pda_obj_creator.to_stack_symbol(x) for x in stack_to]
        self._states.add(s_from)
        self._states.add(s_to)
        if input_symbol != Epsilon():
            self._input_symbols.add(input_symbol)
        self._stack_alphabet.add(stack_from)
        for stack_symbol in stack_to:
            if stack_symbol != Epsilon():
                self._stack_alphabet.add(stack_symbol)
        self._transition_function.add_transition(s_from,
                                                 input_symbol,
                                                 stack_from,
                                                 s_to,
                                                 stack_to)

    def to_final_state(self) -> "PDA":
        """ Turns the current PDA that accepts a language L by empty stack \
        to another PDA that accepts the same language L by final state

        Returns
        ----------
        new_pda : :class:`~pyformlang.pda.PDA`
            The new PDA which accepts by final state the language that \
            was accepted by empty stack
        """
        new_start = get_next_free("#STARTTOFINAL#", State, self._states)
        new_end = get_next_free("#ENDTOFINAL#", State, self._states)
        new_stack_symbol = get_next_free("#BOTTOMTOFINAL#",
                                         StackSymbol,
                                         self._stack_alphabet)
        new_states = self._states.copy()
        new_states.add(new_start)
        new_states.add(new_end)
        new_stack_alphabet = self._stack_alphabet.copy()
        new_stack_alphabet.add(new_stack_symbol)
        new_tf = self._transition_function.copy()
        new_tf.add_transition(new_start, Epsilon(), new_stack_symbol,
                              self._start_state, [self._start_stack_symbol,
                                                  new_stack_symbol])
        for state in self._states:
            new_tf.add_transition(state, Epsilon(), new_stack_symbol,
                                  new_end, [])
        return PDA(new_states,
                   self._input_symbols.copy(),
                   new_stack_alphabet,
                   new_tf,
                   new_start,
                   new_stack_symbol,
                   {new_end})

    def to_empty_stack(self) -> "PDA":
        """ Turns the current PDA that accepts a language L by final state to \
        another PDA that accepts the same language L by empty stack

        Returns
        ----------
        new_pda : :class:`~pyformlang.pda.PDA`
            The new PDA which accepts by empty stack the language that was \
            accepted by final state
        """
        new_start = get_next_free("#STARTEMPTYS#", State, self._states)
        new_end = get_next_free("#ENDEMPTYS#", State, self._states)
        new_stack_symbol = get_next_free("#BOTTOMEMPTYS#",
                                         StackSymbol,
                                         self._stack_alphabet)
        new_states = self._states.copy()
        new_states.add(new_start)
        new_states.add(new_end)
        new_stack_alphabet = self._stack_alphabet.copy()
        new_stack_alphabet.add(new_stack_symbol)
        new_tf = self._transition_function.copy()
        new_tf.add_transition(new_start, Epsilon(), new_stack_symbol,
                              self._start_state, [self._start_stack_symbol,
                                                  new_stack_symbol])
        for state in self._final_states:
            for stack_symbol in new_stack_alphabet:
                new_tf.add_transition(state, Epsilon(), stack_symbol,
                                      new_end, [])
        for stack_symbol in new_stack_alphabet:
            new_tf.add_transition(new_end, Epsilon(), stack_symbol,
                                  new_end, [])
        return PDA(new_states,
                   self._input_symbols.copy(),
                   new_stack_alphabet,
                   new_tf,
                   new_start,
                   new_stack_symbol)

    def to_cfg(self) -> "cfg.CFG":
        """ Turns the language L generated by this PDA when accepting \
        on empty \
        stack into a CFG that accepts the same language L

        Returns
        ----------
        new_cfg : :class:`~pyformlang.cfg.CFG`
            The equivalent CFG
        """
        self._cfg_variable_converter = \
            CFGVariableConverter(self._states, self._stack_alphabet)
        start = cfg.Variable("#StartCFG#")
        productions = self._initialize_production_from_start_in_to_cfg(start)
        states = self._states
        for transition in self._transition_function:
            for state in states:
                self._cfg_variable_converter.set_valid(
                    transition[INPUT][STATE],
                    transition[INPUT][STACK_FROM],
                    state)
        for transition in self._transition_function:
            for state in states:
                self._process_transition_and_state_to_cfg(productions,
                                                          state,
                                                          transition)
        return cfg.CFG(start_symbol=start, productions=productions)

    def _process_transition_and_state_to_cfg(self,
                                             productions,
                                             state,
                                             transition):
        current_state_has_empty_new_stack = \
            len(transition[OUTPUT][NEW_STACK]) == 0 and \
            state != transition[OUTPUT][STATE]
        if not current_state_has_empty_new_stack:
            self._process_transition_and_state_to_cfg_safe(productions, state,
                                                           transition)

    def _process_transition_and_state_to_cfg_safe(self, productions, state,
                                                  transition):
        head = self._get_head_from_state_and_transition(state, transition)
        bodies = self._get_all_bodies_from_state_and_transition(state,
                                                                transition)
        if transition[INPUT][INPUT_SYMBOL] != Epsilon():
            _prepend_input_symbol_to_the_bodies(bodies, transition)
        for body in bodies:
            productions.append(cfg.Production(head, body, filtering=False))

    def _get_all_bodies_from_state_and_transition(self, state, transition):
        return self._generate_all_rules(transition[OUTPUT][STATE],
                                        state,
                                        transition[OUTPUT][NEW_STACK])

    def _generate_all_rules(self, s_from: State, s_to: State,
                            ss_by: List[StackSymbol]) \
            -> Iterable[Iterable["cfg.Variable"]]:
        """ Generates the rules in the CFG conversion """
        if not ss_by:
            return [[]]
        if len(ss_by) == 1:
            return self._generate_length_one_rules(s_from, s_to, ss_by)
        res = []
        is_valid_and_get = self._cfg_variable_converter.is_valid_and_get
        append_to_res = res.append
        length_ss_by_minus_one = len(ss_by) - 1
        for states in product(self._states, repeat=length_ss_by_minus_one):
            last_one = s_from
            temp = []
            stopped = False
            for i in range(length_ss_by_minus_one):
                new_variable = is_valid_and_get(last_one,
                                                ss_by[i],
                                                states[i])
                if new_variable is None:
                    stopped = True
                    break
                temp.append(new_variable)
                last_one = states[i]
            if stopped:
                continue
            new_variable = is_valid_and_get(last_one, ss_by[-1], s_to)
            if new_variable is None:
                continue
            temp.append(new_variable)
            append_to_res(temp)
        return res

    def _generate_length_one_rules(self, s_from, s_to, ss_by):
        state = self._cfg_variable_converter.is_valid_and_get(s_from, ss_by[0],
                                                              s_to)
        if state is not None:
            return [[state]]
        return []

    def _get_head_from_state_and_transition(self, state, transition):
        return self._cfg_variable_converter.to_cfg_combined_variable(
            transition[INPUT][STATE],
            transition[INPUT][STACK_FROM],
            state)

    def _initialize_production_from_start_in_to_cfg(self, start):
        productions = []
        for state in self._states:
            productions.append(
                cfg.Production(
                    start,
                    [self._cfg_variable_converter.to_cfg_combined_variable(
                        self._start_state,
                        self._start_stack_symbol,
                        state)]))
        return productions

    def intersection(self, other: Any) -> "PDA":
        """ Gets the intersection of the language L generated by the \
        current PDA when accepting by final state with something else

        Currently, it only works for regular languages (represented as \
        regular expressions or finite automata) as the intersection \
        between two PDAs is not context-free (it cannot be represented \
        with a PDA).

        Equivalent to:
            >> pda and regex

        Parameters
        ----------
        other : any
            The other part of the intersection

        Returns
        ----------
        new_pda : :class:`~pyformlang.pda.PDA`
            The pda resulting of the intersection

        Raises
        ----------
        NotImplementedError
            When intersecting with something else than a regex or a finite
            automaton
        """
        if isinstance(other, regular_expression.Regex):
            enfa = other.to_epsilon_nfa()
            other = enfa.to_deterministic()
        elif isinstance(other, FiniteAutomaton):
            is_deterministic = other.is_deterministic()
            if not is_deterministic:
                other = other.to_deterministic()
        else:
            raise NotImplementedError
        start_state_other = other.start_states
        if len(start_state_other) == 0:
            return PDA()
        pda_state_converter = _PDAStateConverter(self._states, other.states)
        start_state_other = list(start_state_other)[0]
        final_state_other = other.final_states
        start = pda_state_converter.to_pda_combined_state(self._start_state,
                                                          start_state_other)
        pda = PDA(start_state=start,
                  start_stack_symbol=self._start_stack_symbol)
        symbols = self._input_symbols.copy()
        symbols.add(Epsilon())
        to_process = [(self._start_state, start_state_other)]
        processed = {(self._start_state, start_state_other)}
        while to_process:
            state_in, state_dfa = to_process.pop()
            if (state_in in self._final_states and state_dfa in
                    final_state_other):
                pda.add_final_state(
                    pda_state_converter.to_pda_combined_state(state_in,
                                                              state_dfa))
            for symbol in symbols:
                if symbol == Epsilon():
                    symbol_dfa = finite_automaton.Epsilon()
                else:
                    symbol_dfa = finite_automaton.Symbol(symbol.value)
                if symbol == Epsilon():
                    next_states_dfa = [state_dfa]
                else:
                    next_states_dfa = other(state_dfa, symbol_dfa)
                if len(next_states_dfa) == 0:
                    continue
                for stack_symbol in self._stack_alphabet:
                    next_states_self = self._transition_function(state_in,
                                                                 symbol,
                                                                 stack_symbol)
                    for next_state, next_stack in next_states_self:
                        for next_state_dfa in next_states_dfa:
                            pda.add_transition(
                                pda_state_converter.to_pda_combined_state(
                                    state_in,
                                    state_dfa),
                                symbol,
                                stack_symbol,
                                pda_state_converter.to_pda_combined_state(
                                    next_state,
                                    next_state_dfa),
                                next_stack)
                            if (next_state, next_state_dfa) not in processed:
                                to_process.append((next_state, next_state_dfa))
                                processed.add((next_state, next_state_dfa))
        return pda

    def __and__(self, other):
        """ Gets the intersection of the current PDA with something else

        Equivalent to:
            >> pda and regex

        Parameters
        ----------
        other : any
            The other part of the intersection

        Returns
        ----------
        new_pda : :class:`~pyformlang.pda.PDA`
            The pda resulting of the intersection

        Raises
        ----------
        NotImplementedError
            When intersecting with something else than a regex or a finite
            automaton
        """
        return self.intersection(other)

    def to_dict(self):
        """
        Get the transitions of the PDA as a dictionary
        Returns
        -------
        transitions : dict
            The transitions
        """
        return self._transition_function.to_dict()

    def to_networkx(self) -> nx.MultiDiGraph:
        """
        Transform the current pda into a networkx graph

        Returns
        -------
        graph :  networkx.MultiDiGraph
            A networkx MultiDiGraph representing the pda

        """
        graph = nx.MultiDiGraph()
        for state in self._states:
            graph.add_node(state.value,
                           is_start=state == self._start_state,
                           is_final=state in self.final_states,
                           peripheries=2 if state in self.final_states else 1,
                           label=state.value)
            if state == self._start_state:
                add_start_state_to_graph(graph, state)
        if self._start_stack_symbol is not None:
            graph.add_node("INITIAL_STACK_HIDDEN",
                           label=json.dumps(self._start_stack_symbol.value),
                           shape=None,
                           height=.0,
                           width=.0)
        for key, value in self._transition_function:
            s_from, in_symbol, stack_from = key
            s_to, stack_to = value
            graph.add_edge(
                s_from.value,
                s_to.value,
                label=(json.dumps(in_symbol.value) + " -> " +
                       json.dumps(stack_from.value) + " / " +
                       json.dumps([x.value for x in stack_to])))
        return graph

    @classmethod
    def from_networkx(cls, graph):
        """
        Import a networkx graph into a PDA. \
        The imported graph requires to have the good format, i.e. to come \
        from the function to_networkx

        Parameters
        ----------
        graph :
            The graph representation of the PDA

        Returns
        -------
        pda :
            A PDA automaton read from the graph

        TODO
        -------
        * Explain the format
        """
        pda = PDA()
        for s_from in graph:
            if isinstance(s_from, str) and s_from.startswith("starting_"):
                continue
            for s_to in graph[s_from]:
                for transition in graph[s_from][s_to].values():
                    if "label" in transition:
                        in_symbol, stack_info = transition["label"].split(
                            " -> ")
                        in_symbol = json.loads(in_symbol)
                        stack_from, stack_to = stack_info.split(" / ")
                        stack_from = json.loads(stack_from)
                        stack_to = json.loads(stack_to)
                        pda.add_transition(s_from,
                                           in_symbol,
                                           stack_from,
                                           s_to,
                                           stack_to)
        for node in graph.nodes:
            if graph.nodes[node].get("is_start", False):
                pda.set_start_state(node)
            if graph.nodes[node].get("is_final", False):
                pda.add_final_state(node)
        if "INITIAL_STACK_HIDDEN" in graph.nodes:
            pda.set_start_stack_symbol(
                json.loads(graph.nodes["INITIAL_STACK_HIDDEN"]["label"]))
        return pda

    def write_as_dot(self, filename):
        """
        Write the PDA in dot format into a file

        Parameters
        ----------
        filename : str
            The filename where to write the dot file

        """
        write_dot(self.to_networkx(), filename)


def _prepend_input_symbol_to_the_bodies(bodies, transition):
    to_prepend = cfg.Terminal(transition[INPUT][INPUT_SYMBOL].value)
    for body in bodies:
        body.insert(0, to_prepend)


class _PDAStateConverter:
    # pylint: disable=too-few-public-methods

    def __init__(self, states_pda, states_dfa):
        self._inverse_state_pda = {}
        for i, state in enumerate(states_pda):
            self._inverse_state_pda[state] = i
        self._inverse_state_dfa = {}
        for i, state in enumerate(states_dfa):
            self._inverse_state_dfa[state] = i
        self._conversions = np.empty((len(states_pda), len(states_dfa)),
                                     dtype=object)

    def to_pda_combined_state(self, state_pda, state_other):
        """ To PDA state in the intersection function """
        i_state_pda = self._inverse_state_pda[state_pda]
        i_state_other = self._inverse_state_dfa[state_other]
        if self._conversions[i_state_pda, i_state_other] is None:
            self._conversions[i_state_pda, i_state_other] = State(
                (state_pda, state_other))
        return self._conversions[i_state_pda, i_state_other]


def get_next_free(prefix, type_generating, to_check):
    """ Get free next state or symbol """
    idx = 0
    new_var = type_generating(prefix)
    while new_var in to_check:
        new_var = type_generating(prefix + str(idx))
        idx += 1
    return new_var
