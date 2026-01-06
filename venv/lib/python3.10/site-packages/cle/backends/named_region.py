from __future__ import annotations

from .backend import Backend, register_backend
from .region import EmptySegment

__all__ = ("NamedRegion",)


class NamedRegion(Backend):
    """
    A NamedRegion represents a region of memory that has a name, a location, but no static content.

    This region also has permissions; with no memory, these obviously don't do anything on their own,
    but they help inform any other code that relies on CLE (e.g., angr)

    This can be used as a placeholder for memory that should exist in CLE's view, but for which it does not need data,
    like RAM, MMIO, etc
    """

    is_default = False  # This backend must be constructed manually (or by angr)
    has_memory = False  # This backend, by definition, has no memory backer

    def __init__(self, name, start, end, is_readable=True, is_writable=True, is_executable=False, **kwargs):
        """
        Create a NamedRegion.

        :param name: The name of the region
        :param start: The start address of the region
        :param end: The end address (exclusive) of the region
        :param is_readable: Whether the region is readable
        :param is_writable: Whether the region is writable
        :param is_executable: Whether the region is executable
        :param kwargs:
        """
        self.name = name
        super().__init__(name, None, **kwargs)
        self._min_addr = start
        self.linked_base = start
        self._max_addr = end
        self.has_memory = False
        s = EmptySegment(start, end - start, is_readable, is_writable, is_executable)
        self.segments.append(s)

    def __repr__(self):
        return f"<NamedRegion {self.name}, maps [{self.min_addr:#x}:{self.max_addr:#x}]>"

    @staticmethod
    def is_compatible(stream):
        return stream == 0  # I hate pylint

    @property
    def min_addr(self):
        return self._min_addr

    @property
    def max_addr(self):
        return self._max_addr

    def function_name(self, addr):  # pylint: disable=unused-argument,no-self-use
        """
        NamedRegions don't support function names.
        """
        return None

    def contains_addr(self, addr):
        return self.min_addr <= addr < self.max_addr

    @classmethod
    def check_compatibility(cls, spec, obj):  # pylint: disable=unused-argument
        return False


register_backend("named_region", NamedRegion)
