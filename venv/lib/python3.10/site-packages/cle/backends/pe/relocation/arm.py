from __future__ import annotations

from .pereloc import PEReloc


class IMAGE_REL_BASED_ARM_MOV32(PEReloc):
    pass


class IMAGE_REL_BASED_THUMB_MOV32(PEReloc):
    pass


relocation_table_arm = {
    5: IMAGE_REL_BASED_ARM_MOV32,
    7: IMAGE_REL_BASED_THUMB_MOV32,
}

__all__ = ("relocation_table_arm",)
