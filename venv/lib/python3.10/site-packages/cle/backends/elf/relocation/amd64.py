"""Relocations for amd64/x86_64

Reference: https://gitlab.com/x86-psABIs/x86-64-ABI/-/jobs/artifacts/master/raw/x86-64-ABI/abi.pdf?job=build page 73
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
    RelocTruncate32Mixin,
)


class R_X86_64_64(GenericAbsoluteAddendReloc):
    pass


class R_X86_64_COPY(GenericCopyReloc):
    pass


class R_X86_64_RELATIVE(GenericRelativeReloc):
    pass


class R_X86_64_IRELATIVE(GenericIRelativeReloc):
    pass


class R_X86_64_GLOB_DAT(GenericJumpslotReloc):
    pass


class R_X86_64_JUMP_SLOT(GenericJumpslotReloc):
    pass


class R_X86_64_DTPMOD64(GenericTLSModIdReloc):
    pass


class R_X86_64_DTPOFF64(GenericTLSDoffsetReloc):
    pass


class R_X86_64_TPOFF64(GenericTLSOffsetReloc):
    pass


class R_X86_64_PC32(RelocTruncate32Mixin, GenericPCRelativeAddendReloc):
    check_sign_extend = True


class R_X86_64_32(RelocTruncate32Mixin, GenericAbsoluteAddendReloc):
    check_zero_extend = True


class R_X86_64_32S(RelocTruncate32Mixin, GenericAbsoluteAddendReloc):
    check_sign_extend = True


class R_X86_64_PLT32(RelocTruncate32Mixin, GenericPCRelativeAddendReloc):
    check_sign_extend = True


class R_X86_64_GOTPCREL(RelocGOTMixin, RelocTruncate32Mixin, GenericPCRelativeAddendReloc):
    check_sign_extend = True


class R_X86_64_GOTPCRELX(RelocGOTMixin, RelocTruncate32Mixin, GenericPCRelativeAddendReloc):
    check_sign_extend = True


class R_X86_64_REX_GOTPCRELX(RelocGOTMixin, RelocTruncate32Mixin, GenericPCRelativeAddendReloc):
    check_sign_extend = True


relocation_table_amd64 = {
    1: R_X86_64_64,
    2: R_X86_64_PC32,
    # 3: R_X86_64_GOT32,
    4: R_X86_64_PLT32,
    5: R_X86_64_COPY,
    6: R_X86_64_GLOB_DAT,
    7: R_X86_64_JUMP_SLOT,
    8: R_X86_64_RELATIVE,
    9: R_X86_64_GOTPCREL,
    10: R_X86_64_32,
    11: R_X86_64_32S,
    # 12: R_X86_64_16,
    # 13: R_X86_64_PC16,
    # 14: R_X86_64_8,
    # 15: R_X86_64_PC8,
    16: R_X86_64_DTPMOD64,
    17: R_X86_64_DTPOFF64,
    18: R_X86_64_TPOFF64,
    # 19: R_X86_64_TLSGD,
    # 20: R_X86_64_TLSLD,
    # 21: R_X86_64_DTPOFF32,
    # 22: R_X86_64_GOTTPOFF,
    # 23: R_X86_64_TPOFF32,
    # 24: R_X86_64_PC64,
    # 25: R_X86_64_GOTOFF64,
    # 26: R_X86_64_GOTPC32,
    # 32: R_X86_64_SIZE32,
    # 33: R_X86_64_SIZE64,
    # 34: R_X86_64_GOTPC32_TLSDESC,
    # 35: R_X86_64_TLSDESC_CALL,
    # 36: R_X86_64_TLSDESC,
    37: R_X86_64_IRELATIVE,
    # 38: R_X86_64_RELATIVE64,
    # 39, 40: Deprecated
    41: R_X86_64_GOTPCRELX,
    42: R_X86_64_REX_GOTPCRELX,
    # 43: R_X86_64_CODE_4_GOTPCRELX,
    # 44: R_X86_64_CODE_4_GOTTPOFF,
    # 45: R_X86_64_CODE_4_GOTPC32_TLSDESC,
    # 46: R_X86_64_CODE_5_GOTPCRELX,
    # 47: R_X86_64_CODE_5_GOTTPOFF,
    # 48: R_X86_64_CODE_5_GOTPC32_TLSDESC,
    # 49: R_X86_64_CODE_6_GOTPCRELX,
    # 50: R_X86_64_CODE_6_GOTTPOFF,
    # 51: R_X86_64_CODE_6_GOTPC32_TLSDESC,
}

__all__ = ("relocation_table_amd64",)
