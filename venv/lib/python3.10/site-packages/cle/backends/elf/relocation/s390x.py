"""Relocation types for the S390X architecture.

Reference: https://github.com/IBM/s390x-abi/releases/download/v1.6.1/lzsabi_s390x.pdf pages 51-52
"""

from __future__ import annotations

from .generic import (
    GenericAbsoluteAddendReloc,
    GenericCopyReloc,
    GenericIRelativeReloc,
    GenericJumpslotReloc,
    GenericRelativeReloc,
    GenericTLSOffsetReloc,
)


class R_390_GLOB_DAT(GenericJumpslotReloc):
    pass


class R_390_JMP_SLOT(GenericJumpslotReloc):
    pass


class R_390_RELATIVE(GenericRelativeReloc):
    pass


class R_390_64(GenericAbsoluteAddendReloc):
    pass


class R_390_TLS_TPOFF(GenericTLSOffsetReloc):
    pass


class R_390_IRELATIVE(GenericIRelativeReloc):
    pass


class R_390_COPY(GenericCopyReloc):
    pass


relocation_table_s390x = {
    # 1: R_390_8,
    # 2: R_390_12,
    # 3: R_390_16,
    # 4: R_390_32,
    # 5: R_390_PC32,
    # 6: R_390_GOT12,
    # 7: R_390_GOT32,
    # 8: R_390_PLT32,
    9: R_390_COPY,
    10: R_390_GLOB_DAT,
    11: R_390_JMP_SLOT,
    12: R_390_RELATIVE,
    # 13: R_390_GOTOFF32,
    # 14: R_390_GOTPC,
    # 15: R_390_GOT16,
    # 16: R_390_PC16,
    # 17: R_390_PC16DBL,
    # 18: R_390_PLT16DBL,
    # 19: R_390_PC32DBL,
    # 20: R_390_PLT32DBL,
    # 21: R_390_GOTPCDBL,
    22: R_390_64,
    # 23: R_390_PC64,
    # 24: R_390_GOT64,
    # 25: R_390_PLT64,
    # 26: R_390_GOTENT,
    # 27: R_390_GOTOFF16,
    # 28: R_390_GOTOFF64,
    # 29: R_390_GOTPLT12,
    # 30: R_390_GOTPLT16,
    # 31: R_390_GOTPLT32,
    # 32: R_390_GOTPLT64,
    # 33: R_390_GOTPLTENT,
    # 34: R_390_PLTOFF16,
    # 35: R_390_PLTOFF32,
    # 36: R_390_PLTOFF64,
    # 37: R_390_TLS_LOAD,
    # 38: R_390_TLS_GDCALL,
    # 39: R_390_TLS_LDCALL,
    # No 40 in doc
    # 41: R_390_TLS_GD64,
    # 42: R_390_TLS_GOTIE12,
    # No 43 in doc
    # 44: R_390_TLS_GOTIE64,
    # No 45 in doc
    # 46: R_390_TLS_LDM64,
    # No 47 in doc
    # 48: R_390_TLS_IE64,
    # 49: R_390_TLS_IEENT,
    # No 50 in doc
    # 51: R_390_TLS_LE64,
    # No 52 in doc
    # 53: R_390_TLS_LDO64,
    # 54: R_390_TLS_DTPMOD,
    # 55: R_390_TLS_DTPOFF,
    56: R_390_TLS_TPOFF,
    # 57: R_390_20,
    # 58: R_390_GOT20,
    # 59: R_390_GOTPLT20,
    # 60: R_390_TLS_GOTIE20,
    61: R_390_IRELATIVE,
    # 62: R_390_PC12DBL,
    # 63: R_390_PLT12DBL,
    # 64: R_390_PC24DBL,
    # 65: R_390_PLT24DBL,
}

__all__ = ("relocation_table_s390x",)
