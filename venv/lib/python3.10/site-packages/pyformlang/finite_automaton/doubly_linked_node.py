"""Linked nodes in both direction"""


class DoublyLinkedNode:
    """Represents doubly linked list of nodes from a doubly linked list"""

    def __init__(self,
                 list_in,
                 next_node=None,
                 previous_node=None,
                 value=None):
        self.next_node = next_node
        self.previous_node = previous_node
        self.value = value
        self.list_in = list_in

    def delete(self):
        """Delete the current node"""
        self.list_in.delete(self)

    def append(self, value):
        """
        Append a new node with the given value

        Parameters
        ----------
        value : any
            The value if the new node

        Returns
        -------
        new_node:
            The created node

        """
        next_node = DoublyLinkedNode(self.list_in, self.next_node, self, value)
        if self.next_node is None:
            self.list_in.last = next_node
        self.next_node = next_node
        self.list_in.size += 1
        return next_node
