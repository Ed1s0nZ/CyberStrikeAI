from __future__ import annotations

import logging

from cle.errors import CLEError

from .backend import Backend, register_backend
from .region import Segment

log = logging.getLogger(name=__name__)

__all__ = ("Blob",)


class Blob(Backend):
    """
    Representation of a binary blob, i.e. an executable in an unknown file format.
    """

    is_default = True  # Tell CLE to automatically consider using the Blob backend

    def __init__(self, *args, offset=None, segments=None, **kwargs):
        """
        :param arch:   (required) an :class:`archinfo.Arch` for the binary blob.
        :param offset: Skip this many bytes from the beginning of the file.
        :param segments:      List of tuples describing how to map data into memory. Tuples
                              are of ``(file_offset, mem_addr, size)``.

        You can't specify both ``offset`` and ``segments``.
        """
        if "custom_offset" in kwargs:
            offset = kwargs.pop("custom_offset")
            log.critical("Deprecation warning: the custom_offset parameter has been renamed to offset")
        super().__init__(*args, **kwargs)
        self.set_load_args(offset=offset, segments=segments)

        if self._arch is None:
            raise CLEError("Must specify arch when loading blob!")

        if self._custom_entry_point is None:
            log.warning("No entry_point was specified for blob %s, assuming 0", self.binary_basename)

        self._entry = 0
        self._max_addr = 0
        self._min_addr = 2**64

        try:
            self.linked_base = kwargs["base_addr"]
        except KeyError:
            log.warning("No base_addr was specified for blob %s, assuming 0", self.binary_basename)
        self.mapped_base = self.linked_base

        self.os = "unknown"

        if offset is not None:
            if segments is not None:
                log.error("You can't specify both offset and segments. Taking only the segments data")
            else:
                self._binary_stream.seek(0, 2)
                segments = [(offset, self.linked_base, self._binary_stream.tell() - offset)]
        else:
            if segments is not None:
                pass
            else:
                self._binary_stream.seek(0, 2)
                segments = [(0, self.linked_base, self._binary_stream.tell())]

        for file_offset, mem_addr, size in segments:
            self._load(file_offset, mem_addr, size)

    @staticmethod
    def is_compatible(stream):
        return stream == 0  # I hate pylint

    @property
    def min_addr(self):
        return self._min_addr

    @property
    def max_addr(self):
        return self._max_addr

    def _load(self, file_offset, mem_addr, size):
        """
        Load a segment into memory.
        """

        self._binary_stream.seek(file_offset)
        string = self._binary_stream.read(size)
        if string:
            self.memory.add_backer(mem_addr - self.linked_base, string)
        seg = Segment(file_offset, mem_addr, size, size)
        self.segments.append(seg)
        self._max_addr = max(len(string) + mem_addr - 1, self._max_addr)
        self._min_addr = min(mem_addr, self._min_addr)

    def function_name(self, addr):  # pylint: disable=unused-argument,no-self-use
        """
        Blobs don't support function names.
        """
        return None

    def contains_addr(self, addr):
        return addr >= self.mapped_base and (addr - self.mapped_base) in self.memory

    def in_which_segment(self, addr):  # pylint: disable=unused-argument,no-self-use
        """
        Blobs don't support segments.
        """
        return None

    @classmethod
    def check_compatibility(cls, spec, obj):  # pylint: disable=unused-argument
        return True

    def _cache_content(self):
        return

    def _checksum(self):
        return


register_backend("blob", Blob)
