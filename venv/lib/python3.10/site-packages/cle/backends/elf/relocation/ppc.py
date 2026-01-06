"""Relocation types for PowerPC 32-bit architecture.

Reference: http://refspecs.linux-foundation.org/elf/elfspec_ppc.pdf page 4-18

Only relocations 1-37 are described in the document. The rest are from the GNU
binutils source code. See include/elf/ppc.h in the binutils source code.
"""

from __future__ import annotations

import logging

from .elfreloc import ELFReloc
from .generic import (
    GenericAbsoluteAddendReloc,
    GenericCopyReloc,
    GenericJumpslotReloc,
    GenericRelativeReloc,
    GenericTLSDoffsetReloc,
    GenericTLSModIdReloc,
    GenericTLSOffsetReloc,
)

log = logging.getLogger(name=__name__)


# PPC constants/masks to be used in relocations
PPC_WORD32 = 0xFFFFFFFF
PPC_WORD30 = 0xFFFFFFFC
PPC_LOW24 = 0x03FFFFFC
PPC_LOW14 = 0x0020FFFC
PPC_HALF16 = 0xFFFF
PPC_BL_INST = 0x48000001


class R_PPC_ADDR32(GenericAbsoluteAddendReloc):
    pass


class R_PPC_ADDR24(ELFReloc):
    """
    Relocation Type: 0x2
    Calculation: (S + A) >> 2
    Field: low24*
    """

    @property
    def value(self):
        A = self.addend
        S = self.resolvedby.rebased_addr

        result = (S + A) >> 2
        return result


class R_PPC_ADDR16(ELFReloc):
    """
    Relocation Type: 0x3
    Calculation: S+A
    Field: half16*
    """

    @property
    def value(self):
        A = self.addend
        S = self.resolvedby.rebased_addr

        result = S + A
        return result


class R_PPC_ADDR16_LO(ELFReloc):
    """
    Relocation Type: 0x4
    Calculation: #lo(S + A)
    Field: half16
    """

    @property
    def value(self):
        A = self.addend
        S = self.resolvedby.rebased_addr

        result = S + A
        result = result & PPC_HALF16
        return result

    def relocate(self):
        if not self.resolved:
            return False
        self.owner.memory.pack_word(self.relative_addr, self.value, size=2)
        return True


class R_PPC_ADDR16_HI(ELFReloc):
    """
    Relocation Type: 0x5
    Calculation: #hi(S + A)
    Field: half16
    """

    @property
    def value(self):
        A = self.addend
        S = self.resolvedby.rebased_addr

        result = (S + A) >> 16
        result = result & PPC_HALF16
        return result

    def relocate(self):
        if not self.resolved:
            return False
        self.owner.memory.pack_word(self.relative_addr, self.value, size=2)
        return True


class R_PPC_ADDR16_HA(ELFReloc):  # pylint: disable=undefined-variable
    """
    Relocation Type: 0x6
    Calculation: #ha(S + A)
    Field: half16
    """

    @property
    def value(self):
        A = self.addend
        S = self.resolvedby.rebased_addr

        result = S + A
        result = ((result >> 16) + (1 if (result & 0x8000) else 0)) & PPC_HALF16
        return result

    def relocate(self):
        if not self.resolved:
            return False
        self.owner.memory.pack_word(self.relative_addr, self.value, size=2)
        return True


class R_PPC_ADDR14(ELFReloc):
    """
    Relocation Type: 0x7
    Calculation: (S + A) >> 2
    Field: low14*
    """

    @property
    def value(self):
        A = self.addend
        S = self.resolvedby.rebased_addr

        result = (S + A) >> 2
        return result


class R_PPC_ADDR14_BRTAKEN(ELFReloc):
    """
    Relocation Type: 0x8
    Calculation: (S + A) >> 2
    Field: low14*
    """

    @property
    def value(self):
        A = self.addend
        S = self.resolvedby.rebased_addr

        result = (S + A) >> 2
        return result


class R_PPC_ADDR14_BRNTAKEN(ELFReloc):
    """
    Relocation Type: 0x9
    Calculation: (S + A) >> 2
    Field: low14*
    """

    @property
    def value(self):
        A = self.addend
        S = self.resolvedby.rebased_addr

        result = (S + A) >> 2
        return result


class R_PPC_REL24(ELFReloc):
    """
    Relocation Type: 0xa
    Calculation: (S + A - P) >> 2
    Field: low24*
    R_PPC_REL24 is a special type of relocation.
    The instruction must be modified for this type.
    This relocation type resolves branch-and-link instructions.
    Prior to relocation, all instances of the branch-and-link instruction
    will consist of the following bytecode: 48 00 00 01.
    The problem with this is that all instances will result in calls to
    the current address - thus an infinite loop.
    After calculating the relocation result in R_PPC_REL24,
    you will have an address offset to the call.
    The result must be resolved to the correct instruction encoding.
    """

    @property
    def value(self):
        A = self.addend
        S = self.resolvedby.rebased_addr
        P = self.rebased_addr

        result = (S + A - P) >> 2
        result = (result << 2) & PPC_LOW24
        result = (A & ~PPC_LOW24) | result
        result = result | PPC_BL_INST
        return result


class R_PPC_REL14(ELFReloc):
    """
    Relocation Type: 0xb
    Calculation: (S + A - P) >> 2
    Field: low14*
    """

    @property
    def value(self):
        A = self.addend
        S = self.resolvedby.rebased_addr
        P = self.rebased_addr

        result = (S + A - P) >> 2
        result = (result << 2) & PPC_LOW14
        result = (A & ~PPC_LOW14) | result
        return result


class R_PPC_REL14_BRTAKEN(ELFReloc):
    """
    Relocation Type: 0xc
    Calculation: (S + A - P) >> 2
    Field: low14*
    """

    @property
    def value(self):
        A = self.addend
        S = self.resolvedby.rebased_addr
        P = self.rebased_addr

        result = (S + A - P) >> 2
        result = (result << 2) & PPC_LOW14
        result = (A & ~PPC_LOW14) | result
        return result


class R_PPC_REL14_BRNTAKEN(ELFReloc):
    """
    Relocation Type: 0xd
    Calculation: (S + A - P) >> 2
    Field: low14*
    """

    @property
    def value(self):
        A = self.addend
        S = self.resolvedby.rebased_addr
        P = self.rebased_addr

        result = (S + A - P) >> 2
        result = (result << 2) & PPC_LOW14
        result = (A & ~PPC_LOW14) | result
        return result


class R_PPC_COPY(GenericCopyReloc):
    pass


class R_PPC_GLOB_DAT(GenericJumpslotReloc):
    pass


class R_PPC_JMP_SLOT(GenericJumpslotReloc):
    def relocate(self):
        if "DT_PPC_GOT" not in self.owner._dynamic and "DT_LOPROC" not in self.owner._dynamic:
            # old PowerPC ABI - we overwrite this location with a jump (b, 0x12. .. .. .1) to the actual target
            val = (0x12 << 26) | ((self.value - self.rebased_addr) & 0x3FFFFFE)
            self.owner.memory.pack_word(self.dest_addr, val)
        else:
            super().relocate()


class R_PPC_RELATIVE(GenericRelativeReloc):
    pass


class R_PPC_UADDR32(ELFReloc):
    """
    Relocation Type: 0x18
    Calculation: S + A
    Field: word32
    """

    @property
    def value(self):
        A = self.addend
        S = self.resolvedby.rebased_addr

        result = S + A
        return result


class R_PPC_UADDR16(ELFReloc):
    """
    Relocation Type: 0x19
    Calculation: S + A
    Field: half16*
    """

    @property
    def value(self):
        A = self.addend
        S = self.resolvedby.rebased_addr

        result = S + A
        return result


class R_PPC_REL32(ELFReloc):  # pylint: disable=undefined-variable
    """
    Relocation Type: 0x1a
    Calculation: S + A - P
    Field: word32
    """

    @property
    def value(self):
        P = self.rebased_addr
        A = self.addend
        S = self.resolvedby.rebased_addr

        result = (S + A - P) & PPC_WORD32
        return result


class R_PPC_SECTOFF(ELFReloc):
    """
    Relocation Type: 0x21
    Calculation: R + A
    Field: half16*
    """

    @property
    def value(self):
        R = self.relative_addr
        A = self.addend

        result = R + A
        return result


class R_PPC_SECTOFF_LO(ELFReloc):
    """
    Relocation Type: 0x22
    Calculation: #lo(R + A)
    Field: half16
    """

    @property
    def value(self):
        R = self.relative_addr
        A = self.addend

        result = R + A
        result = result & PPC_HALF16
        return result


class R_PPC_SECTOFF_HI(ELFReloc):
    """
    Relocation Type: 0x23
    Calculation: #hi(R + A)
    Field: half16
    """

    @property
    def value(self):
        R = self.relative_addr
        A = self.addend

        result = (R + A) >> 16
        result = result & PPC_HALF16
        return result


class R_PPC_SECTOFF_HA(ELFReloc):
    """
    Relocation Type: 0x24
    Calculation: #ha(R + A)
    Field: half16
    """

    @property
    def value(self):
        R = self.relative_addr
        A = self.addend

        result = R + A
        result = ((result >> 16) + (1 if (result & 0x8000) else 0)) & PPC_HALF16
        return result


class R_PPC_ADDR30(ELFReloc):
    """
    Relocation Type: 0x25
    Calculation: (S + A - P) >> 2
    Field: word30
    """

    @property
    def value(self):
        S = self.resolvedby.rebased_addr
        A = self.addend
        P = self.rebased_addr

        result = (S + A - P) >> 2
        return result


class R_PPC_DTPMOD32(GenericTLSModIdReloc):
    pass


class R_PPC_DTPREL32(GenericTLSDoffsetReloc):
    pass


class R_PPC_TPREL32(GenericTLSOffsetReloc):
    pass


relocation_table_ppc = {
    1: R_PPC_ADDR32,
    2: R_PPC_ADDR24,
    3: R_PPC_ADDR16,
    4: R_PPC_ADDR16_LO,
    5: R_PPC_ADDR16_HI,
    6: R_PPC_ADDR16_HA,
    7: R_PPC_ADDR14,
    8: R_PPC_ADDR14_BRTAKEN,
    9: R_PPC_ADDR14_BRNTAKEN,
    10: R_PPC_REL24,
    11: R_PPC_REL14,
    12: R_PPC_REL14_BRTAKEN,
    13: R_PPC_REL14_BRNTAKEN,
    # 14: R_PPC_GOT16,
    # 15: R_PPC_GOT16_LO,
    # 16: R_PPC_GOT16_HI,
    # 17: R_PPC_GOT16_HA,
    # 18: R_PPC_PLTREL24,
    19: R_PPC_COPY,
    20: R_PPC_GLOB_DAT,
    21: R_PPC_JMP_SLOT,
    22: R_PPC_RELATIVE,
    # 23: R_PPC_LOCAL24PC,
    24: R_PPC_UADDR32,
    25: R_PPC_UADDR16,
    26: R_PPC_REL32,
    # 27: R_PPC_PLT32,
    # 28: R_PPC_PLTREL32,
    # 29: R_PPC_PLT16_LO,
    # 30: R_PPC_PLT16_HI,
    # 31: R_PPC_PLT16_HA,
    # 32: R_PPC_SDAREL16,
    33: R_PPC_SECTOFF,
    34: R_PPC_SECTOFF_LO,
    35: R_PPC_SECTOFF_HI,
    36: R_PPC_SECTOFF_HA,
    37: R_PPC_ADDR30,
    # 67: R_PPC_TLS,
    68: R_PPC_DTPMOD32,
    # 69: R_PPC_TPREL16,
    # 70: R_PPC_TPREL16_LO,
    # 71: R_PPC_TPREL16_HI,
    # 72: R_PPC_TPREL16_HA,
    73: R_PPC_TPREL32,
    # 74: R_PPC_DTPREL16,
    # 75: R_PPC_DTPREL16_LO,
    # 76: R_PPC_DTPREL16_HI,
    # 77: R_PPC_DTPREL16_HA,
    78: R_PPC_DTPREL32,
    # 79: R_PPC_GOT_TLSGD16,
    # 80: R_PPC_GOT_TLSGD16_LO,
    # 81: R_PPC_GOT_TLSGD16_HI,
    # 82: R_PPC_GOT_TLSGD16_HA,
    # 83: R_PPC_GOT_TLSLD16,
    # 84: R_PPC_GOT_TLSLD16_LO,
    # 85: R_PPC_GOT_TLSLD16_HI,
    # 86: R_PPC_GOT_TLSLD16_HA,
    # 87: R_PPC_GOT_TPREL16,
    # 88: R_PPC_GOT_TPREL16_LO,
    # 89: R_PPC_GOT_TPREL16_HI,
    # 90: R_PPC_GOT_TPREL16_HA,
    # 91: R_PPC_GOT_DTPREL16,
    # 92: R_PPC_GOT_DTPREL16_LO,
    # 93: R_PPC_GOT_DTPREL16_HI,
    # 94: R_PPC_GOT_DTPREL16_HA,
    # 95: R_PPC_TLSGD,
    # 96: R_PPC_TLSLD,
}

__all__ = ("relocation_table_ppc",)
