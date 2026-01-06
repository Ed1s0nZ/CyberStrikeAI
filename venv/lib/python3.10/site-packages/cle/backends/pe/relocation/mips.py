from __future__ import annotations

from .pereloc import PEReloc


class IMAGE_REL_BASED_MIPS_JMPADDR(PEReloc):
    pass


class IMAGE_REL_BASED_MIPS_JMPADDR16(PEReloc):
    pass


relocation_table_mips = {
    5: IMAGE_REL_BASED_MIPS_JMPADDR,
    9: IMAGE_REL_BASED_MIPS_JMPADDR16,
}

__all__ = ("relocation_table_mips",)
