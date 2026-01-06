from __future__ import annotations

from cle.backends.backend import register_backend
from cle.backends.region import Segment

from .cgc import CGC


class FakeSegment(Segment):
    def __init__(self, start, size):
        super().__init__(0, start, 0, size)
        self.is_readable = True
        self.is_writable = True
        self.is_executable = False


class BackedCGC(CGC):
    """
    This is a backend for CGC executables that allows user provide a memory backer and a register backer as the
    initial state of the running binary.
    """

    is_default = True  # Tell CLE to automatically consider using the BackedCGC backend

    def __init__(
        self,
        *args,
        memory_backer=None,
        register_backer=None,
        writes_backer=None,
        permissions_map=None,
        current_allocation_base=None,
        **kwargs,
    ):
        """
        :param path:                    File path to CGC executable.
        :param memory_backer:           A dict of memory content, with beginning address of each segment as key and
                                        actual memory content as data.
        :param register_backer:         A dict of all register contents. EIP will be used as the entry point of this
                                        executable.
        :param permissions_map:         A dict of memory region to permission flags
        :param current_allocation_base: An integer representing the current address of the top of the CGC heap.
        """
        super().__init__(*args, **kwargs)

        self.memory_backer = memory_backer
        self.register_backer = register_backer
        self.writes_backer = writes_backer
        self.permissions_map = permissions_map
        self.current_allocation_base = current_allocation_base

        for seg in self.segments:
            if seg.is_executable:
                exec_seg_addr = seg.vaddr
                break
        else:
            raise ValueError("Couldn't find executable segment?")

        for start, _ in self.memory._backers:
            if start != exec_seg_addr:
                self.memory.remove_backer(start)

        for start, data in sorted(self.memory_backer.items()):
            existing_seg = self.find_segment_containing(start)
            if existing_seg is None:  # this is the text or data segment
                new_seg = FakeSegment(start, len(data))
                self.segments.append(new_seg)

            if start == exec_seg_addr:
                continue

            if start in self.memory:
                raise ValueError("IF THIS GETS THROWN I'M GONNA JUMP OUT THE WINDOW")

            self.memory.add_backer(start, data)

        if self.register_backer is not None and "eip" in self.register_backer:
            self._entry = self.register_backer["eip"]

    @staticmethod
    def is_compatible(stream):
        return False  # Don't use this for anything unless it's manual

    @property
    def threads(self):
        return [0]

    def thread_registers(self, thread=None):
        return self.register_backer.items()


register_backend("backedcgc", BackedCGC)
