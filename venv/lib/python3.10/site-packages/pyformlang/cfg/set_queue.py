""" A queue with non duplicate elements"""


class SetQueue:
    """ A queue with non duplicate elements"""

    def __init__(self):
        self._to_process = []
        self._processing = set()

    def append(self, value):
        """ Append an element """
        if value not in self._processing:
            self._to_process.append(value)
            self._processing.add(value)

    def pop(self):
        """ Pop an element """
        popped = self._to_process.pop()
        self._processing.remove(popped)
        return popped

    def __bool__(self):
        return bool(self._to_process)
