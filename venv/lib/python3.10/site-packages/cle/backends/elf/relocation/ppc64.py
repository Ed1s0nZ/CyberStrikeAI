"""Relocation types for PPC64.

Reference: http://refspecs.linuxfoundation.org/ELF/ppc64/PPC-elf64abi-1.9.pdf pages 57-59
"""

from __future__ import annotations

import logging

from .elfreloc import ELFReloc
from .generic import (
    GenericAbsoluteAddendReloc,
    GenericIRelativeReloc,
    GenericJumpslotReloc,
    GenericRelativeReloc,
    GenericTLSDoffsetReloc,
    GenericTLSModIdReloc,
    GenericTLSOffsetReloc,
)

log = logging.getLogger(name=__name__)

# pylint: disable=missing-class-docstring


class R_PPC64_JMP_SLOT(ELFReloc):
    def relocate(self):
        if self.owner.is_ppc64_abiv1:
            # R_PPC64_JMP_SLOT
            # http://osxr.org/glibc/source/sysdeps/powerpc/powerpc64/dl-machine.h?v=glibc-2.15#0405
            # copy an entire function descriptor struct
            addr = self.resolvedby.owner.memory.unpack_word(self.resolvedby.relative_addr)
            toc = self.resolvedby.owner.memory.unpack_word(self.resolvedby.relative_addr + 8)
            aux = self.resolvedby.owner.memory.unpack_word(self.resolvedby.relative_addr + 16)
            self.owner.memory.pack_word(self.relative_addr, addr)
            self.owner.memory.pack_word(self.relative_addr + 8, toc)
            self.owner.memory.pack_word(self.relative_addr + 16, aux)
        else:
            self.owner.memory.pack_word(self.relative_addr, self.resolvedby.rebased_addr)
        return True


class R_PPC64_RELATIVE(GenericRelativeReloc):
    pass


class R_PPC64_IRELATIVE(GenericIRelativeReloc):
    pass


class R_PPC64_ADDR64(GenericAbsoluteAddendReloc):
    pass


class R_PPC64_GLOB_DAT(GenericJumpslotReloc):
    pass


class R_PPC64_DTPMOD64(GenericTLSModIdReloc):
    pass


class R_PPC64_DTPREL64(GenericTLSDoffsetReloc):
    pass


class R_PPC64_TPREL64(GenericTLSOffsetReloc):
    pass


class R_PPC64_REL24(ELFReloc):
    """
    Relocation Type: 10
    Calculation: (S + A - P) >> 2
    Field: low24*
    """

    @property
    def value(self):
        A = self.addend
        S = self.resolvedby.rebased_addr
        P = self.rebased_addr
        return (S + A - P) >> 2

    def relocate(self):
        if not self.resolved:
            return False
        instr = self.owner.memory.unpack_word(self.relative_addr, size=4) & 0b11111100000000000000000000000011
        imm = self.value & 0xFFFFFF
        self.owner.memory.pack_word(self.relative_addr, instr | (imm << 2), size=4)
        return True


class R_PPC64_TOC16_LO(ELFReloc):
    """
    Relocation Type: 48
    Calculation: #lo(S + A - .TOC.)
    Field: half16
    """

    @property
    def value(self):
        A = self.addend
        S = self.resolvedby.rebased_addr
        if self.owner.ppc64_initial_rtoc is None:
            log.warning(".TOC. value not found")
            return (S + A) & 0xFFFF
        TOC = self.owner.ppc64_initial_rtoc
        return (S + A - TOC) & 0xFFFF

    def relocate(self):
        if not self.resolved:
            return False
        self.owner.memory.pack_word(self.relative_addr, self.value, size=2)
        return True


class R_PPC64_TOC16_HI(ELFReloc):
    """
    Relocation Type: 49
    Calculation: #hi(S + A - .TOC.)
    Field: half16
    """

    @property
    def value(self):
        A = self.addend
        S = self.resolvedby.rebased_addr
        if self.owner.ppc64_initial_rtoc is None:
            log.warning(".TOC. value not found")
            return ((S + A) >> 16) & 0xFFFF
        TOC = self.owner.ppc64_initial_rtoc
        return ((S + A - TOC) >> 16) & 0xFFFF

    def relocate(self):
        if not self.resolved:
            return False
        self.owner.memory.pack_word(self.relative_addr, self.value, size=2)
        return True


class R_PPC64_TOC16_HA(ELFReloc):
    """
    Relocation Type: 50
    Calculation: #ha(S + A - .TOC.)
    Field: half16
    """

    @property
    def value(self):
        A = self.addend
        S = self.resolvedby.rebased_addr
        if self.owner.ppc64_initial_rtoc is None:
            log.warning(".TOC. value not found")
            return (((S + A) >> 16) + (1 if ((S + A) & 0x8000) else 0)) & 0xFFFF
        TOC = self.owner.ppc64_initial_rtoc
        return (((S + A - TOC) >> 16) + (1 if ((S + A - TOC) & 0x8000) else 0)) & 0xFFFF

    def relocate(self):
        if not self.resolved:
            return False
        self.owner.memory.pack_word(self.relative_addr, self.value, size=2)
        return True


class R_PPC64_TOC(ELFReloc):
    """
    Relocation Type: 51
    Calculation: .TOC.
    Field: doubleword64
    """

    @property
    def value(self):
        if self.owner.ppc64_initial_rtoc is None:
            log.warning(".TOC. value not found")
            return 0
        return self.owner.ppc64_initial_rtoc


relocation_table_ppc64 = {
    # 1: R_PPC64_ADDR32,
    # 2: R_PPC64_ADDR24,
    # 3: R_PPC64_ADDR16,
    # 4: R_PPC64_ADDR16_LO,
    # 5: R_PPC64_ADDR16_HI,
    # 6: R_PPC64_ADDR16_HA,
    # 7: R_PPC64_ADDR14,
    # 8: R_PPC64_ADDR14_BRTAKEN,
    # 9: R_PPC64_ADDR14_BRNTAKEN,
    10: R_PPC64_REL24,
    # 11: R_PPC64_REL14,
    # 12: R_PPC64_REL14_BRTAKEN,
    # 13: R_PPC64_REL14_BRNTAKEN,
    # 14: R_PPC64_GOT16,
    # 15: R_PPC64_GOT16_LO,
    # 16: R_PPC64_GOT16_HI,
    # 17: R_PPC64_GOT16_HA,
    # No 18 in doc
    # 19: R_PPC64_COPY,
    20: R_PPC64_GLOB_DAT,
    21: R_PPC64_JMP_SLOT,
    22: R_PPC64_RELATIVE,
    # No 23 in doc
    # 24: R_PPC64_UADDR32,
    # 25: R_PPC64_UADDR16,
    # 26: R_PPC64_REL32,
    # 27: R_PPC64_PLT32,
    # 28: R_PPC64_PLTREL32,
    # 29: R_PPC64_PLT16_LO,
    # 30: R_PPC64_PLT16_HI,
    # 31: R_PPC64_PLT16_HA,
    # No 32 in doc
    # 33: R_PPC64_SECTOFF,
    # 34: R_PPC64_SECTOFF_LO,
    # 35: R_PPC64_SECTOFF_HI,
    # 36: R_PPC64_SECTOFF_HA,
    # 37: R_PPC64_ADDR30,
    38: R_PPC64_ADDR64,
    # 39: R_PPC64_ADDR16_HIGHER,
    # 40: R_PPC64_ADDR16_HIGHERA,
    # 41: R_PPC64_ADDR16_HIGHEST,
    # 42: R_PPC64_ADDR16_HIGHESTA,
    # 43: R_PPC64_UADDR64,
    # 44: R_PPC64_REL64,
    # 45: R_PPC64_PLT64,
    # 46: R_PPC64_PLTREL64,
    # 47: R_PPC64_TOC16,
    48: R_PPC64_TOC16_LO,
    49: R_PPC64_TOC16_HI,
    50: R_PPC64_TOC16_HA,
    51: R_PPC64_TOC,
    # 52: R_PPC64_PLTGOT16,
    # 53: R_PPC64_PLTGOT16_LO,
    # 54: R_PPC64_PLTGOT16_HI,
    # 55: R_PPC64_PLTGOT16_HA,
    # 56: R_PPC64_ADDR16_DS,
    # 57: R_PPC64_ADDR16_LO_DS,
    # 58: R_PPC64_GOT16_DS,
    # 59: R_PPC64_GOT16_LO_DS,
    # 60: R_PPC64_PLT16_LO_DS,
    # 61: R_PPC64_SECTOFF_DS,
    # 62: R_PPC64_SECTOFF_LO_DS,
    # 63: R_PPC64_TOC16_DS,
    # 64: R_PPC64_TOC16_LO_DS,
    # 65: R_PPC64_PLTGOT16_DS,
    # 66: R_PPC64_PLTGOT16_LO_DS,
    # 67: R_PPC64_TLS,
    68: R_PPC64_DTPMOD64,
    # 69: R_PPC64_TPREL16,
    # 70: R_PPC64_TPREL16_LO,
    # 71: R_PPC64_TPREL16_HI,
    # 72: R_PPC64_TPREL16_HA,
    73: R_PPC64_TPREL64,
    # 74: R_PPC64_DTPREL16,
    # 75: R_PPC64_DTPREL16_LO,
    # 76: R_PPC64_DTPREL16_HI,
    # 77: R_PPC64_DTPREL16_HA,
    78: R_PPC64_DTPREL64,
    # 79: R_PPC64_GOT_TLSGD16,
    # 80: R_PPC64_GOT_TLSGD16_LO,
    # 81: R_PPC64_GOT_TLSGD16_HI,
    # 82: R_PPC64_GOT_TLSGD16_HA,
    # 83: R_PPC64_GOT_TLSLD16,
    # 84: R_PPC64_GOT_TLSLD16_LO,
    # 85: R_PPC64_GOT_TLSLD16_HI,
    # 86: R_PPC64_GOT_TLSLD16_HA,
    # 87: R_PPC64_GOT_TPREL16_DS,
    # 88: R_PPC64_GOT_TPREL16_LO_DS,
    # 89: R_PPC64_GOT_TPREL16_HI,
    # 90: R_PPC64_GOT_TPREL16_HA,
    # 91: R_PPC64_GOT_DTPREL16_DS,
    # 92: R_PPC64_GOT_DTPREL16_LO_DS,
    # 93: R_PPC64_GOT_DTPREL16_HI,
    # 94: R_PPC64_GOT_DTPREL16_HA,
    # 95: R_PPC64_TPREL16_DS,
    # 96: R_PPC64_TPREL16_LO_DS,
    # 97: R_PPC64_TPREL16_HIGHER,
    # 98: R_PPC64_TPREL16_HIGHERA,
    # 99: R_PPC64_TPREL16_HIGHEST,
    # 100: R_PPC64_TPREL16_HIGHESTA,
    # 101: R_PPC64_DTPREL16_DS,
    # 102: R_PPC64_DTPREL16_LO_DS,
    # 103: R_PPC64_DTPREL16_HIGHER,
    # 104: R_PPC64_DTPREL16_HIGHERA,
    # 105: R_PPC64_DTPREL16_HIGHEST,
    # 106: R_PPC64_DTPREL16_HIGHESTA,
    # Not in spec
    248: R_PPC64_IRELATIVE,
}

__all__ = ("relocation_table_ppc64",)
