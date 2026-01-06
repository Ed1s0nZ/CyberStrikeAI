"""
:mod:`pyformlang.finite_automaton`
==================================

This module deals with finite state automata.

Available Classes
-----------------

:class:`~pyformlang.finite_automaton.FiniteAutomaton`
    A general representation of automata. Cannot be used directly.
:class:`~pyformlang.finite_automaton.DeterministicFiniteAutomaton`
    A deterministic finite automaton
:class:`~pyformlang.finite_automaton.NondeterministicFiniteAutomaton`
    A non-deterministic finite automaton, without epsilon transitions
:class:`~pyformlang.finite_automaton.EpsilonNFA`
    A non-deterministic finite automaton, with epsilon transitions
:class:`~pyformlang.finite_automaton.TransitionFunction`
    A deterministic transition function
:class:`~pyformlang.finite_automaton.NondeterministicTransitionFunction`
    A non-deterministic transition function
:class:`~pyformlang.finite_automaton.State`
    A state (or node) in an automaton
:class:`~pyformlang.finite_automaton.Symbol`
    A symbol (part of the alphabet) in an automaton
:class:`~pyformlang.finite_automaton.Epsilon`
    The epsilon (or empty) symbol
:class:`~pyformlang.finite_automaton.DuplicateTransitionError`
    An error that occurs when trying to add a non-deterministic edge to a \
    deterministic automaton
:class:`~pyformlang.finite_automaton.InvalidEpsilonTransition`
    An exception that occurs when adding an epsilon transition to a \
    non-epsilon NFA.

"""

from .finite_automaton import FiniteAutomaton
from .deterministic_finite_automaton import DeterministicFiniteAutomaton
from .nondeterministic_finite_automaton import NondeterministicFiniteAutomaton
from .epsilon_nfa import EpsilonNFA
from .state import State
from .symbol import Symbol
from .epsilon import Epsilon
from .transition_function import (TransitionFunction,
                                  DuplicateTransitionError,
                                  InvalidEpsilonTransition)
from .nondeterministic_transition_function import \
    NondeterministicTransitionFunction

__all__ = ["FiniteAutomaton",
           "DeterministicFiniteAutomaton",
           "NondeterministicFiniteAutomaton",
           "EpsilonNFA",
           "State",
           "Symbol",
           "Epsilon",
           "TransitionFunction",
           "NondeterministicTransitionFunction",
           "DuplicateTransitionError",
           "InvalidEpsilonTransition"]
