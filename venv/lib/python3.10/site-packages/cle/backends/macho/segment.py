# This file is part of Mach-O Loader for CLE.
# Contributed December 2016 by Fraunhofer SIT (https://www.sit.fraunhofer.de/en/).
from __future__ import annotations

from cle.backends.region import Segment


class MachOSegment(Segment):
    """
    Mach-O Segment

        - offset is the offset into the file the region starts
        - vaddr (or just addr) is the virtual address
        - filesize (or just size) is the size of the region in the file
        - memsize (or vsize) is the size of the region when loaded into memory
        - segname is the segment's name without padding
        - nsect is the number of sections contained in this segment
        - sections is an array of MachOSections
        - flags is a bit vector containing per-segment flags
        - initprot and maxprot are initial and maximum permissions respectively
    """

    def __init__(self, offset, vaddr, size, vsize, segname, nsect, sections, flags, initprot, maxprot):
        super().__init__(offset, vaddr, size, vsize)

        self.segname = segname.decode()
        self.nsect = nsect
        self.sections = sections
        self.flags = flags
        self.initprot = initprot
        self.maxprot = maxprot

    def get_section_by_name(self, name):
        """
        Searches for a section by name within this segment
        :param name: Name of the section
        :return: MachOSection or None
        """
        for sec in self.sections:
            if sec.sectname == name:
                return sec
        return None

    def __getitem__(self, item):
        """
        Syntactic sugar for get_section_by_name
        """
        return self.get_section_by_name(item)

    @property
    def is_readable(self):
        return ((self.initprot | self.maxprot) & 0x01) != 0

    @property
    def is_writable(self):
        return ((self.initprot | self.maxprot) & 0x02) != 0

    @property
    def is_executable(self):
        return ((self.initprot | self.maxprot) & 0x04) != 0

    def __repr__(self):
        return "<{} | offset {:#x}, vaddr {:#x}, size {:#x}>".format(
            self.segname if self.segname else "Unnamed", self.offset, self.vaddr, self.memsize
        )
