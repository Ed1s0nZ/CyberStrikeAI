"""Relocation types for i386.

Reference: https://github.com/hjl-tools/x86-psABI/wiki/intel386-psABI-1.1.pdf page 36
"""

from __future__ import annotations

from .generic import (
    GenericAbsoluteAddendReloc,
    GenericCopyReloc,
    GenericIRelativeReloc,
    GenericJumpslotReloc,
    GenericPCRelativeAddendReloc,
    GenericRelativeReloc,
    GenericTLSDoffsetReloc,
    GenericTLSModIdReloc,
    GenericTLSOffsetReloc,
    RelocGOTMixin,
)


class R_386_32(GenericAbsoluteAddendReloc):
    """
    Value: 1
    Field: word32
    Calculation: S + A
    """


class R_386_PC32(GenericPCRelativeAddendReloc):
    """
    Value: 2
    Field: word32
    Calculation: S + A - P
    """


class R_386_COPY(GenericCopyReloc):
    """
    Value: 5
    Field:
    Calculation:
    """


class R_386_GLOB_DAT(GenericJumpslotReloc):
    """
    Value: 6
    Field: word32
    Calculation: S
    """


class R_386_JMP_SLOT(GenericJumpslotReloc):
    """
    Value: 7
    Field: word32
    Calculation: S
    """


class R_386_RELATIVE(GenericRelativeReloc):
    """
    Value: 8
    Field: word32
    Calculation: B + A
    """


class R_386_IRELATIVE(GenericIRelativeReloc):
    """
    Value: 42
    Field: word32
    Calculation: indirect (B + A)
    """


class R_386_TLS_DTPMOD32(GenericTLSModIdReloc):
    """
    Value: 35
    Field: word32
    Calculation:
    """


class R_386_TLS_TPOFF(GenericTLSOffsetReloc):
    """
    Value: 14
    Field: word32
    Calculation:
    """


class R_386_TLS_DTPOFF32(GenericTLSDoffsetReloc):
    """
    Value: 36
    Field: word32
    Calculation:
    """


class R_386_PLT32(GenericPCRelativeAddendReloc):
    """
    Value: 4
    Field: word32
    Calculation: L + A - P
    """


class R_386_GOTPC(GenericPCRelativeAddendReloc, RelocGOTMixin):
    """
    Value: 10
    Field: word32
    Calculation: GOT + A - P
    """


relocation_table_i386 = {
    1: R_386_32,
    2: R_386_PC32,
    # 3: R_386_GOT32,
    4: R_386_PLT32,
    5: R_386_COPY,
    6: R_386_GLOB_DAT,
    7: R_386_JMP_SLOT,
    8: R_386_RELATIVE,
    # 9: R_386_GOTOFF,
    10: R_386_GOTPC,
    14: R_386_TLS_TPOFF,
    # 15: R_386_TLS_IE,
    # 16: R_386_TLS_GOTIE,
    # 17: R_386_TLS_LE,
    # 18: R_386_TLS_GD,
    # 19: R_386_TLS_LDM,
    # 20: R_386_16,
    # 21: R_386_PC16,
    # 22: R_386_8,
    # 23: R_386_PC8,
    # 24: R_386_TLS_GD_32,
    # 25: R_386_TLS_GD_PUSH,
    # 26: R_386_TLS_GD_CALL,
    # 27: R_386_TLS_GD_POP,
    # 28: R_386_TLS_LDM_32,
    # 29: R_386_TLS_LDM_PUSH,
    # 30: R_386_TLS_LDM_CALL,
    # 31: R_386_TLS_LDM_POP,
    # 32: R_386_TLS_LDO_32,
    # 33: R_386_TLS_IE_32,
    # 34: R_386_TLS_LE_32,
    35: R_386_TLS_DTPMOD32,
    36: R_386_TLS_DTPOFF32,
    # 37: R_386_TLS_TPOFF32,
    # 38: R_386_SIZE32,
    # 39: R_386_TLS_GOTDESC,
    # 40: R_386_TLS_DESC_CALL,
    # 41: R_386_TLS_DESC,
    42: R_386_IRELATIVE,
    # 43: R_386_GOT32X,
}

__all__ = ("relocation_table_i386",)
