"""
:mod:`pyformlang.indexed_grammar`
==================================

This module deals with indexed grammars.

Available Classes
-----------------

IndexedGrammar
    An indexed grammar
Rules
    A representation of a set of indexed grammar rules
EndRule
    An end rule, turning a variable into a terminal
ConsumptionRule
    A consumption rule, consuming something from the stack
ProductionRule
    A production rule, pushing something on the stack
DuplicationRule
    A duplication rule, duplicating the stack

"""

from .rules import Rules
from .consumption_rule import ConsumptionRule
from .end_rule import EndRule
from .production_rule import ProductionRule
from .duplication_rule import DuplicationRule
from .indexed_grammar import IndexedGrammar

__all__ = ["Rules",
           "ConsumptionRule",
           "EndRule",
           "ProductionRule",
           "DuplicationRule",
           "IndexedGrammar"]
