# This file is part of Mach-O Loader for CLE.
# Contributed December 2016 by Fraunhofer SIT (https://www.sit.fraunhofer.de/en/).
from __future__ import annotations

from cle.backends.region import Section

from .segment import MachOSegment

TYPE_MASK = 0x000000FF
ATTRIBUTES_MASK = 0xFFFFFF00


class MachOSection(Section):
    """
    Mach-O Section, only defined within the context of a Mach-O Segment.

        - offset is the offset into the file the region starts
        - vaddr (or just addr) is the virtual address
        - filesize (or just size) is the size of the region in the file
        - memsize (or vsize) is the size of the region when loaded into memory
        - segname is the corresponding segment's name without padding
        - sectname is the section's name without padding
        - align is the sections alignment as a power of 2
        - reloff is the file offset to the section's relocation entries
        - nreloc is the number of relocation entries for this section
        - flags is a bit vector containing per-section flags
        - r1 and r2 are values for the reserved1 and reserved2 fields respectively
    """

    def __init__(
        self,
        offset,
        vaddr,
        size,
        vsize,
        segname,
        sectname,
        align,
        reloff,
        nreloc,
        flags,
        r1,
        r2,
        parent_segment: MachOSegment | None = None,
    ):
        super().__init__(sectname.decode(), offset, vaddr, size)
        self.filesize = size
        self.memsize = vsize
        self.segname = segname.decode()
        self.sectname = sectname.decode()
        self.align = align
        self.reloff = reloff
        self.nreloc = nreloc
        self.flags = flags
        self.reserved1 = r1
        self.reserved2 = r2
        self.parent_segment = parent_segment

    @property
    def full_name(self) -> str:
        return f"{self.segname},{self.sectname}"

    @property
    def type(self):
        return self.flags & TYPE_MASK

    @property
    def attributes(self):
        return self.flags & ATTRIBUTES_MASK

    @property
    def is_readable(self):
        """
        Always true, because sections should always be readable
        :return:
        """
        return True

    @property
    def is_writable(self):
        """
        Returns the permission of the parent segment, because MachO sections simply inherit that
        :return:
        """
        return self.parent_segment.is_writable

    @property
    def is_executable(self):
        """
        Returns the permission of the parent segment, because MachO sections simply inherit that
        :return:
        """
        return self.parent_segment.is_executable

    @property
    def only_contains_uninitialized_data(self):
        """
        I actually don't know if this is true, but it seems like a saner assumption than true
        :return:
        """
        return False

    def __repr__(self):
        return "<Section: {} (part of Segment: {})| offset {:#x}, vaddr {:#x}, size {:#x}>".format(
            self.sectname if self.sectname else "Unnamed", self.segname, self.offset, self.vaddr, self.memsize
        )
