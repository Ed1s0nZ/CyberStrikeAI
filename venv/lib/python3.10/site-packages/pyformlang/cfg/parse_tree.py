""" A parse Tree """

import networkx as nx
from networkx.drawing.nx_pydot import write_dot

from pyformlang.cfg.variable import Variable


class ParseTree:
    """ A parse tree """

    def __init__(self, value):
        self.value = value
        self.sons = []

    def __repr__(self):
        return "ParseTree(" + str(self.value) + ", " + str(self.sons) + ")"

    def get_leftmost_derivation(self):
        """
        Get the leftmost derivation

        Returns
        -------
        derivation : list of list of :class:`~pyformlang.cfg.CFGObject`
            The derivation

        """
        if len(self.sons) == 0 and isinstance(self.value, Variable):
            return [[self.value], []]
        if len(self.sons) == 0:
            return [[self.value]]
        res = [[self.value]]
        start = []
        for i, son in enumerate(self.sons):
            end = [x.value for x in self.sons[i + 1:]]
            derivation = []
            derivations = son.get_leftmost_derivation()
            if i != 0 and derivations and derivations[0]:
                del derivations[0]
            for derivation in derivations:
                res.append(start + derivation + end)
            if derivation:
                start = start + derivation
            else:
                start.append(son.value)
        return res

    def get_rightmost_derivation(self):
        """
        Get the leftmost derivation

        Returns
        -------
        derivation : list of list of :class:`~pyformlang.cfg.CFGObject`
            The derivation

        """
        if len(self.sons) == 0 and isinstance(self.value, Variable):
            return [[self.value], []]
        if len(self.sons) == 0:
            return [[self.value]]
        res = [[self.value]]
        end = []
        for i, son in enumerate(self.sons[::-1]):
            start = [x.value for x in self.sons[:-1 - i]]
            derivation = []
            derivations = son.get_rightmost_derivation()
            if i != 0 and derivations and derivations[0]:
                del derivations[0]
            for derivation in derivations:
                res.append(start + derivation + end)
            end = derivation + end
        return res

    def to_networkx(self):
        """
        Transforms the tree into a Networkx Directed Graph

        Returns
        -------
        tree : networkx.Digraph
            The tree in Networkx format.

        """
        tree = nx.DiGraph()
        tree.add_node("ROOT", label=self.value.value)
        to_process = [("ROOT", son) for son in self.sons[::-1]]
        counter = 0
        while to_process:
            previous_node, current_node = to_process.pop()
            new_node = str(counter)
            if isinstance(current_node.value, str):
                tree.add_node(new_node, label=current_node.value)
            else:
                tree.add_node(new_node, label=current_node.value.value)
            counter += 1
            tree.add_edge(previous_node, new_node)
            to_process += [(new_node, son) for son in current_node.sons[::-1]]
        return tree

    def write_as_dot(self, filename):
        """
        Write the parse tree in dot format into a file

        Parameters
        ----------
        filename : str
            The filename where to write the dot file

        """
        write_dot(self.to_networkx(), filename)
