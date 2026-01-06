"""Internal usage states"""
from typing import Tuple

from pyformlang.cfg import Variable
from pyformlang.cfg.parse_tree import ParseTree
from pyformlang.fcfg.feature_production import FeatureProduction
from pyformlang.fcfg.feature_structure import FeatureStructure


class State:
    """For internal usage"""

    def __init__(self,
                 production: FeatureProduction,
                 positions: Tuple[int, int, int],
                 feature_stucture: FeatureStructure,
                 parse_tree: ParseTree):
        self.production = production
        self.positions = positions
        self.feature_stucture = feature_stucture
        self.parse_tree = parse_tree

    def get_key(self):
        """Get the key of the state"""
        return self.production, self.positions

    def is_incomplete(self):
        """Check if a state is incomplete"""
        return self.positions[2] < len(self.production.body)

    def next_is_variable(self):
        """Check if the next symbol to process is a variable"""
        return isinstance(self.production.body[self.positions[2]], Variable)

    def next_is_word(self, word):
        """Check if the next symbol matches a given word"""
        return self.production.body[self.positions[2]] == word


class StateProcessed:
    """For internal usage"""

    def __init__(self, size: int):
        self.processed = [{} for _ in range(size)]

    def add(self, i: int, element: State):
        """Add a state to the processed states. Returns if the insertion was successful or not."""
        key = element.get_key()
        if key not in self.processed[i]:
            self.processed[i][key] = []
        for other in self.processed[i][key]:
            if other.feature_stucture.subsumes(element.feature_stucture):
                return False
        self.processed[i][key].append(element)
        return True

    def generator(self, i: int):
        """Generates a collection of all the states at a given position"""
        for states in self.processed[i].values():
            for state in states:
                yield state
