""" Finite State Transducer """
import json
from typing import Any, Iterable

import networkx as nx
from networkx.drawing.nx_pydot import write_dot

from pyformlang.indexed_grammar import DuplicationRule, ProductionRule, \
    EndRule, ConsumptionRule, IndexedGrammar, Rules


class FST:
    """ Representation of a Finite State Transducer"""

    def __init__(self):
        self._states = set()  # Set of states
        self._input_symbols = set()  # Set of input symbols
        self._output_symbols = set()  # Set of output symbols
        # Dict from _states x _input_symbols U {epsilon} into a subset of
        # _states X _output_symbols*
        self._delta = {}
        self._start_states = set()
        self._final_states = set()  # _final_states is final states

    @property
    def states(self):
        """ Get the states of the FST

        Returns
        ----------
        states : set of any
            The states
        """
        return self._states

    @property
    def input_symbols(self):
        """ Get the input symbols of the FST

        Returns
        ----------
        input_symbols : set of any
            The input symbols of the FST
        """
        return self._input_symbols

    @property
    def output_symbols(self):
        """ Get the output symbols of the FST

        Returns
        ----------
        output_symbols : set of any
            The output symbols of the FST
        """
        return self._output_symbols

    @property
    def start_states(self):
        """ Get the start states of the FST

        Returns
        ----------
        start_states : set of any
            The start states of the FST
        """
        return self._start_states

    @property
    def final_states(self):
        """ Get the final states of the FST

        Returns
        ----------
        final_states : set of any
            The final states of the FST
        """
        return self._final_states

    @property
    def transitions(self):
        """Gives the transitions as a dictionary"""
        return self._delta

    def get_number_transitions(self) -> int:
        """ Get the number of transitions in the FST

        Returns
        ----------
        n_transitions : int
            The number of transitions
        """
        return sum(len(x) for x in self._delta.values())

    def add_transition(self, s_from: Any,
                       input_symbol: Any,
                       s_to: Any,
                       output_symbols: Iterable[Any]):
        """ Add a transition to the FST

        Parameters
        -----------
        s_from : any
            The source state
        input_symbol : any
            The symbol to read
        s_to : any
            The destination state
        output_symbols : iterable of Any
            The symbols to output
        """
        self._states.add(s_from)
        self._states.add(s_to)
        if input_symbol != "epsilon":
            self._input_symbols.add(input_symbol)
        for output_symbol in output_symbols:
            if output_symbol != "epsilon":
                self._output_symbols.add(output_symbol)
        head = (s_from, input_symbol)
        if head in self._delta:
            self._delta[head].append((s_to, output_symbols))
        else:
            self._delta[head] = [(s_to, output_symbols)]

    def add_transitions(self, transitions_list):
        """
        Adds several transitions to the FST

        Parameters
        ----------
        transitions_list : list of tuples
            The tuples have the form (s_from, in_symbol, s_to, out_symbols)
        """
        for s_from, input_symbol, s_to, output_symbols in transitions_list:
            self.add_transition(
                s_from,
                input_symbol,
                s_to,
                output_symbols
            )

    def add_start_state(self, start_state: Any):
        """ Add a start state

        Parameters
        ----------
        start_state : any
            The start state
        """
        self._states.add(start_state)
        self._start_states.add(start_state)

    def add_final_state(self, final_state: Any):
        """ Add a final state

        Parameters
        ----------
        final_state : any
            The final state to add
        """
        self._final_states.add(final_state)
        self._states.add(final_state)

    def translate(self, input_word: Iterable[Any], max_length: int = -1) -> \
            Iterable[Any]:
        """ Translate a string into another using the FST

        Parameters
        ----------
        input_word : iterable of any
            The word to translate
        max_length : int, optional
            The maximum size of the output word, to prevent infinite \
            generation due to epsilon transitions

        Returns
        ----------
        output_word : iterable of any
            The translation of the input word
        """
        # (remaining in the input, generated so far, current_state)
        to_process = []
        seen_by_state = {state: [] for state in self.states}
        for start_state in self._start_states:
            to_process.append((input_word, [], start_state))
        while to_process:
            remaining, generated, current_state = to_process.pop()
            if (remaining, generated) in seen_by_state[current_state]:
                continue
            seen_by_state[current_state].append((remaining, generated))
            if len(remaining) == 0 and current_state in self._final_states:
                yield generated
            # We try to read an input
            if len(remaining) != 0:
                for next_state, output_string in self._delta.get(
                        (current_state, remaining[0]), []):
                    to_process.append(
                        (remaining[1:],
                         generated + output_string,
                         next_state))
            # We try to read an epsilon transition
            if max_length == -1 or len(generated) < max_length:
                for next_state, output_string in self._delta.get(
                        (current_state, "epsilon"), []):
                    to_process.append((remaining,
                                       generated + output_string,
                                       next_state))

    def intersection(self, indexed_grammar):
        """ Compute the intersection with an other object

        Equivalent to:
          >> fst and indexed_grammar
        """
        rules = indexed_grammar.rules
        new_rules = [EndRule("T", "epsilon")]
        self._extract_consumption_rules_intersection(rules, new_rules)
        self._extract_indexed_grammar_rules_intersection(rules, new_rules)
        self._extract_terminals_intersection(rules, new_rules)
        self._extract_epsilon_transitions_intersection(new_rules)
        self._extract_fst_delta_intersection(new_rules)
        self._extract_fst_epsilon_intersection(new_rules)
        self._extract_fst_duplication_rules_intersection(new_rules)
        rules = Rules(new_rules, rules.optim)
        return IndexedGrammar(rules).remove_useless_rules()

    def _extract_fst_duplication_rules_intersection(self, new_rules):
        for state_p in self._final_states:
            for start_state in self._start_states:
                new_rules.append(DuplicationRule(
                    "S",
                    str((start_state, "S", state_p)),
                    "T"))

    def _extract_fst_epsilon_intersection(self, new_rules):
        for state_p in self._states:
            new_rules.append(EndRule(
                str((state_p, "epsilon", state_p)),
                "epsilon"))

    def _extract_fst_delta_intersection(self, new_rules):
        for key, pair in self._delta.items():
            state_p = key[0]
            terminal = key[1]
            for transition in pair:
                state_q = transition[0]
                symbol = transition[1]
                new_rules.append(EndRule(str((state_p, terminal, state_q)),
                                         symbol))

    def _extract_epsilon_transitions_intersection(self, new_rules):
        for state_p in self._states:
            for state_q in self._states:
                for state_r in self._states:
                    new_rules.append(DuplicationRule(
                        str((state_p, "epsilon", state_q)),
                        str((state_p, "epsilon", state_r)),
                        str((state_r, "epsilon", state_q))))

    def _extract_indexed_grammar_rules_intersection(self, rules, new_rules):
        for rule in rules.rules:
            if rule.is_duplication():
                for state_p in self._states:
                    for state_q in self._states:
                        for state_r in self._states:
                            new_rules.append(DuplicationRule(
                                str((state_p, rule.left_term, state_q)),
                                str((state_p, rule.right_terms[0], state_r)),
                                str((state_r, rule.right_terms[1], state_q))))
            elif rule.is_production():
                for state_p in self._states:
                    for state_q in self._states:
                        new_rules.append(ProductionRule(
                            str((state_p, rule.left_term, state_q)),
                            str((state_p, rule.right_term, state_q)),
                            str(rule.production)))
            elif rule.is_end_rule():
                for state_p in self._states:
                    for state_q in self._states:
                        new_rules.append(DuplicationRule(
                            str((state_p, rule.left_term, state_q)),
                            str((state_p, rule.right_term, state_q)),
                            "T"))

    def _extract_terminals_intersection(self, rules, new_rules):
        terminals = rules.terminals
        for terminal in terminals:
            for state_p in self._states:
                for state_q in self._states:
                    for state_r in self._states:
                        new_rules.append(DuplicationRule(
                            str((state_p, terminal, state_q)),
                            str((state_p, "epsilon", state_r)),
                            str((state_r, terminal, state_q))))
                        new_rules.append(DuplicationRule(
                            str((state_p, terminal, state_q)),
                            str((state_p, terminal, state_r)),
                            str((state_r, "epsilon", state_q))))

    def _extract_consumption_rules_intersection(self, rules, new_rules):
        consumptions = rules.consumption_rules
        for consumption_rule in consumptions:
            for consumption in consumptions[consumption_rule]:
                for state_r in self._states:
                    for state_s in self._states:
                        new_rules.append(ConsumptionRule(
                            consumption.f_parameter,
                            str((state_r, consumption.left_term, state_s)),
                            str((state_r, consumption.right, state_s))))

    def __and__(self, other):
        return self.intersection(other)

    def union(self, other_fst):
        """
        Makes the union of two fst
        Parameters
        ----------
        other_fst : :class:`~pyformlang.fst.FST`
            The other FST

        Returns
        -------
        union_fst : :class:`~pyformlang.fst.FST`
            A new FST which is the union of the two given FST

        """
        state_renaming = self._get_state_renaming(other_fst)
        union_fst = FST()
        # pylint: disable=protected-access
        self._copy_into(union_fst, state_renaming, 0)
        other_fst._copy_into(union_fst, state_renaming, 1)
        return union_fst

    def __or__(self, other_fst):
        """
        Makes the union of two fst
        Parameters
        ----------
        other_fst : :class:`~pyformlang.fst.FST`
            The other FST

        Returns
        -------
        union_fst : :class:`~pyformlang.fst.FST`
            A new FST which is the union of the two given FST

        """
        return self.union(other_fst)

    def _copy_into(self, union_fst, state_renaming, idx):
        self._add_extremity_states_to(union_fst, state_renaming, idx)
        self._add_transitions_to(union_fst, state_renaming, idx)

    def _add_transitions_to(self, union_fst, state_renaming, idx):
        for head, transition in self.transitions.items():
            s_from, input_symbol = head
            for s_to, output_symbols in transition:
                union_fst.add_transition(
                    state_renaming.get_name(s_from, idx),
                    input_symbol,
                    state_renaming.get_name(s_to, idx),
                    output_symbols)

    def _add_extremity_states_to(self, union_fst, state_renaming, idx):
        self._add_start_states_to(union_fst, state_renaming, idx)
        self._add_final_states_to(union_fst, state_renaming, idx)

    def _add_final_states_to(self, union_fst, state_renaming, idx):
        for state in self.final_states:
            union_fst.add_final_state(state_renaming.get_name(state, idx))

    def _add_start_states_to(self, union_fst, state_renaming, idx):
        for state in self.start_states:
            union_fst.add_start_state(state_renaming.get_name(state, idx))

    def concatenate(self, other_fst):
        """
        Makes the concatenation of two fst
        Parameters
        ----------
        other_fst : :class:`~pyformlang.fst.FST`
            The other FST

        Returns
        -------
        fst_concatenate : :class:`~pyformlang.fst.FST`
            A new FST which is the concatenation of the two given FST

        """
        state_renaming = self._get_state_renaming(other_fst)
        fst_concatenate = FST()
        self._add_start_states_to(fst_concatenate, state_renaming, 0)
        # pylint: disable=protected-access
        other_fst._add_final_states_to(fst_concatenate, state_renaming, 1)
        self._add_transitions_to(fst_concatenate, state_renaming, 0)
        other_fst._add_transitions_to(fst_concatenate, state_renaming, 1)
        for final_state in self.final_states:
            for start_state in other_fst.start_states:
                fst_concatenate.add_transition(
                    state_renaming.get_name(final_state, 0),
                    "epsilon",
                    state_renaming.get_name(start_state, 1),
                    []
                )
        return fst_concatenate

    def __add__(self, other):
        """
        Makes the concatenation of two fst
        Parameters
        ----------
        other : :class:`~pyformlang.fst.FST`
            The other FST

        Returns
        -------
        fst_concatenate : :class:`~pyformlang.fst.FST`
            A new FST which is the concatenation of the two given FST

        """
        return self.concatenate(other)

    def _get_state_renaming(self, other_fst):
        state_renaming = FSTStateRemaining()
        state_renaming.add_states(list(self.states), 0)
        state_renaming.add_states(other_fst.states, 1)
        return state_renaming

    def kleene_star(self):
        """
        Computes the kleene star of the FST

        Returns
        -------
        fst_star : :class:`~pyformlang.fst.FST`
            A FST representing the kleene star of the FST
        """
        fst_star = FST()
        state_renaming = FSTStateRemaining()
        state_renaming.add_states(list(self.states), 0)
        self._add_extremity_states_to(fst_star, state_renaming, 0)
        self._add_transitions_to(fst_star, state_renaming, 0)
        for final_state in self.final_states:
            for start_state in self.start_states:
                fst_star.add_transition(
                    state_renaming.get_name(final_state, 0),
                    "epsilon",
                    state_renaming.get_name(start_state, 0),
                    []
                )
        for final_state in self.start_states:
            for start_state in self.final_states:
                fst_star.add_transition(
                    state_renaming.get_name(final_state, 0),
                    "epsilon",
                    state_renaming.get_name(start_state, 0),
                    []
                )
        return fst_star

    def to_networkx(self) -> nx.MultiDiGraph:
        """
        Transform the current fst into a networkx graph

        Returns
        -------
        graph :  networkx.MultiDiGraph
            A networkx MultiDiGraph representing the fst

        """
        graph = nx.MultiDiGraph()
        for state in self._states:
            graph.add_node(state,
                           is_start=state in self.start_states,
                           is_final=state in self.final_states,
                           peripheries=2 if state in self.final_states else 1,
                           label=state)
            if state in self.start_states:
                graph.add_node("starting_" + str(state),
                               label="",
                               shape=None,
                               height=.0,
                               width=.0)
                graph.add_edge("starting_" + str(state),
                               state)
        for s_from, input_symbol in self._delta:
            for s_to, output_symbols in self._delta[(s_from, input_symbol)]:
                graph.add_edge(
                    s_from,
                    s_to,
                    label=(json.dumps(input_symbol) + " -> " +
                           json.dumps(output_symbols)))
        return graph

    @classmethod
    def from_networkx(cls, graph):
        """
        Import a networkx graph into an finite state transducer. \
        The imported graph requires to have the good format, i.e. to come \
        from the function to_networkx

        Parameters
        ----------
        graph :
            The graph representation of the FST

        Returns
        -------
        enfa :
            A FST read from the graph

        TODO
        -------
        * Explain the format
        """
        fst = FST()
        for s_from in graph:
            for s_to in graph[s_from]:
                for transition in graph[s_from][s_to].values():
                    if "label" in transition:
                        in_symbol, out_symbols = transition["label"].split(
                            " -> ")
                        in_symbol = json.loads(in_symbol)
                        out_symbols = json.loads(out_symbols)
                        fst.add_transition(s_from,
                                           in_symbol,
                                           s_to,
                                           out_symbols)
        for node in graph.nodes:
            if graph.nodes[node].get("is_start", False):
                fst.add_start_state(node)
            if graph.nodes[node].get("is_final", False):
                fst.add_final_state(node)
        return fst

    def write_as_dot(self, filename):
        """
        Write the FST in dot format into a file

        Parameters
        ----------
        filename : str
            The filename where to write the dot file

        """
        write_dot(self.to_networkx(), filename)


class FSTStateRemaining:
    """Class for remaining the states in FST"""

    def __init__(self):
        self._state_renaming = {}
        self._seen_states = set()

    def add_state(self, state, idx):
        """
        Add a state
        Parameters
        ----------
        state : str
            The state to add
        idx : int
            The index of the FST
        """
        if state in self._seen_states:
            counter = 0
            new_state = state + str(counter)
            while new_state in self._seen_states:
                counter += 1
                new_state = state + str(counter)
            self._state_renaming[(state, idx)] = new_state
            self._seen_states.add(new_state)
        else:
            self._state_renaming[(state, idx)] = state
            self._seen_states.add(state)

    def add_states(self, states, idx):
        """
        Add states
        Parameters
        ----------
        states : list of str
            The states to add
        idx : int
            The index of the FST
        """
        for state in states:
            self.add_state(state, idx)

    def get_name(self, state, idx):
        """
        Get the renaming.

        Parameters
        ----------
        state : str
            The state to rename
        idx : int
            The index of the FST

        Returns
        -------
        new_name : str
            The new name of the state
        """
        return self._state_renaming[(state, idx)]
