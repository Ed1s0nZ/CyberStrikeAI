from __future__ import annotations

import archinfo
from xbe import Xbe

from .backend import Backend, register_backend
from .region import Section, Segment


class XBESection(Section):
    def __init__(self, name, file_offset, file_size, virtual_addr, virtual_size, xbe_sec):
        super().__init__(name, file_offset, virtual_addr, virtual_size)
        self.filesize = file_size
        self._xbe_sec = xbe_sec

    @property
    def is_readable(self):
        """
        Whether this section has read permissions
        """
        return True

    @property
    def is_writable(self):
        """
        Whether this section has write permissions
        """
        return (self._xbe_sec.header.flags & self._xbe_sec.header.flags.WRITABLE) != 0

    @property
    def is_executable(self):
        """
        Whether this section has execute permissions
        """
        return (self._xbe_sec.header.flags & self._xbe_sec.header.flags.EXECUTABLE) != 0

    @property
    def only_contains_uninitialized_data(self):
        """
        We load every section in, they're all initialized
        """
        return False


class XBE(Backend):
    """
    The main loader class for statically loading XBE executables.
    """

    is_default = True

    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.set_arch(archinfo.arch_from_id("x86"))

        self.os = "xbox"
        if self.binary is None:
            self._xbe = Xbe(data=self._binary_stream.read())
        else:
            self._xbe = Xbe.from_file(self.binary)
        self._entry = self._xbe.entry_addr
        self._image_vmem = bytearray(self._xbe.header.image_size)
        self._min_addr = self._xbe.header.base_addr
        self._max_addr = self._xbe.header.base_addr + self._xbe.header.image_size - 1

        # Add header
        self._add_xbe_section(
            0,
            self._xbe.header.image_header_size,
            self._xbe.header.base_addr,
            self._xbe.header.image_header_size,
            self._xbe.header_data,
        )

        # Add each section
        for _, sec in self._xbe.sections.items():
            self._add_xbe_section(
                sec.header.raw_addr,
                sec.header.raw_size,
                sec.header.virtual_addr,
                sec.header.virtual_size,
                sec.data,
                sec,
            )

        self.memory.add_backer(0, bytes(self._image_vmem))
        self.mapped_base = self.linked_base = self._xbe.header.base_addr

    def _add_xbe_section(self, file_offset, file_size, virtual_addr, virtual_size, backer, sec=None):
        # Copy in section contents
        start = virtual_addr - self._xbe.header.base_addr
        end = start + file_size
        self._image_vmem[start:end] = backer

        # Create a segment and a section
        seg = Segment(file_offset, virtual_addr, file_size, virtual_size)
        self.segments.append(seg)

        if sec is not None:
            sec = XBESection(sec.name, file_offset, file_size, virtual_addr, virtual_size, sec)
            self.sections.append(sec)

    def close(self):
        super().close()
        del self._xbe

    @staticmethod
    def is_compatible(stream):
        stream.seek(0)
        identstring = stream.read(4)
        stream.seek(0)
        return identstring.startswith(b"XBEH")

    @property
    def min_addr(self):
        return self._min_addr

    @property
    def max_addr(self):
        return self._max_addr

    @classmethod
    def check_compatibility(cls, spec, obj):  # pylint: disable=unused-argument
        assert False
        return True


register_backend("xbe", XBE)
