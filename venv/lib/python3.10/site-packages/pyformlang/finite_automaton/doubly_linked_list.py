"""A doubly linked list"""

from .doubly_linked_node import DoublyLinkedNode


class DoublyLinkedList:
    """  A doubly linked list """

    def __init__(self):
        self.first = None
        self.last = None
        self.size = 0
        self._current_node = None

    def append(self, value):
        """ Appends an element """
        if self.last is not None:
            self.last = self.last.append(value)
        else:
            node = DoublyLinkedNode(self, value=value)
            self.first = node
            self.last = node
            self.size += 1
        return self.last

    def delete(self, node):
        """ Delete an element """
        if node.next_node is not None:
            node.next_node.previous_node = node.previous_node
        else:
            self.last = node.previous_node
        if node.previous_node is not None:
            node.previous_node.next_node = node.next_node
        else:
            self.first = node.next_node
        self.size -= 1

    def __len__(self):
        return self.size

    def __iter__(self):
        self._current_node = self.first
        return self

    def __next__(self):
        if self._current_node is None:
            raise StopIteration
        res = self._current_node
        self._current_node = res.next_node
        return res
