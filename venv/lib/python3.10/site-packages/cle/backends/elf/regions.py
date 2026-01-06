from __future__ import annotations

from cle.backends.region import Section, Segment


def maybedecode(string):
    return string if isinstance(string, str) else string.decode()


class ELFSegment(Segment):
    """
    Represents a segment for the ELF format.
    """

    def __init__(self, readelf_seg, relro=False):
        self.flags = readelf_seg.header.p_flags
        self.relro = relro
        super().__init__(
            readelf_seg.header.p_offset,
            readelf_seg.header.p_vaddr,
            readelf_seg.header.p_filesz,
            readelf_seg.header.p_memsz,
        )

    @property
    def is_readable(self):
        return self.flags & 4 != 0

    @property
    def is_writable(self):
        return self.flags & 2 != 0

    @property
    def is_executable(self):
        return self.flags & 1 != 0

    @property
    def is_relro(self):
        return self.relro


class ELFSection(Section):
    SHF_WRITE = 0x1
    SHF_ALLOC = 0x2
    SHF_EXECINSTR = 0x4
    SHF_STRINGS = 0x20
    SHT_NULL = "SHT_NULL"

    def __init__(self, readelf_sec, remap_offset=0):
        super().__init__(
            maybedecode(readelf_sec.name),
            readelf_sec.header.sh_offset,
            readelf_sec.header.sh_addr + remap_offset,
            readelf_sec.header.sh_size,
        )

        self.type = readelf_sec.header.sh_type
        self.entsize = readelf_sec.header.sh_entsize
        self.flags = readelf_sec.header.sh_flags
        self.link = readelf_sec.header.sh_link
        self.info = readelf_sec.header.sh_info
        self.align = readelf_sec.header.sh_addralign
        self.remap_offset = remap_offset

    @property
    def is_readable(self):
        return True

    @property
    def is_active(self):
        return self.type != self.SHT_NULL

    @property
    def is_writable(self):
        return self.flags & self.SHF_WRITE != 0

    @property
    def occupies_memory(self):
        return self.flags & self.SHF_ALLOC != 0 and self.memsize > 0

    @property
    def is_executable(self):
        return self.flags & self.SHF_EXECINSTR != 0

    @property
    def is_strings(self):
        return self.flags & self.SHF_STRINGS != 0

    @property
    def only_contains_uninitialized_data(self):
        return self.type == "SHT_NOBITS"
