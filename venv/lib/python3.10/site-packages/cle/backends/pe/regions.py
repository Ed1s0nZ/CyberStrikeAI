from __future__ import annotations

from cle.backends.region import Section


class PESection(Section):
    """
    Represents a section for the PE format.
    """

    def __init__(self, pe_section, remap_offset=0, name: str | None = None):
        super().__init__(
            name or pe_section.Name.decode("latin-1"),  # ensure all bytes can be decoded
            pe_section.PointerToRawData,
            pe_section.VirtualAddress + remap_offset,
            pe_section.Misc_VirtualSize,
        )

        self.characteristics = pe_section.Characteristics
        self.filesize = pe_section.SizeOfRawData

    #
    # Public properties
    #

    @property
    def is_readable(self):
        return self.characteristics & 0x40000000 != 0

    @property
    def is_writable(self):
        return self.characteristics & 0x80000000 != 0

    @property
    def is_executable(self):
        return self.characteristics & 0x20000000 != 0

    @property
    def only_contains_uninitialized_data(self):
        return self.filesize == 0
