"""Relocations for SPARC

Reference: https://sparc.org/wp-content/uploads/2014/01/psABI3rd.pdf.gz page 4-4
"""

from __future__ import annotations

from .elfreloc import ELFReloc


class R_SPARC_HI22(ELFReloc):
    """
    Value: 9
    Field: T-imm22
    Calculation: (S + A) >> 10
    """

    @property
    def value(self):
        result = (self.resolvedby.rebased_addr + self.addend) >> 10
        instr_bytes = self.owner.memory.load(self.relative_addr, 4)
        instr = int.from_bytes(instr_bytes, byteorder="big")
        return instr & 0xFFC00000 | result & 0x3FFFFF


class R_SPARC_WDISP30(ELFReloc):
    """
    Value: 7
    Field: V-disp30
    Calculation: (S + A - P) >> 2
    """

    @property
    def value(self):
        result = (self.resolvedby.rebased_addr + self.addend - self.rebased_addr) >> 2
        instr_bytes = self.owner.memory.load(self.relative_addr, 4)
        instr = int.from_bytes(instr_bytes, byteorder="big")
        return instr & 0xC0000000 | result & 0x3FFFFFFF


class R_SPARC_LO10(ELFReloc):
    """
    Value: 12
    Field: T-simm13
    Calculation: (S + A) & 0x3ff
    """

    @property
    def value(self):
        result = (self.resolvedby.rebased_addr + self.addend) & 0x3FF
        instr_bytes = self.owner.memory.load(self.relative_addr, 4)
        instr = int.from_bytes(instr_bytes, byteorder="big")
        return instr & 0xFFFFE000 | result & 0x1FFF


relocation_table_sparc = {
    # 1: R_SPARC_8,
    # 2: R_SPARC_16,
    # 3: R_SPARC_32,
    # 4: R_SPARC_DISP8,
    # 5: R_SPARC_DISP16,
    # 6: R_SPARC_DISP32,
    7: R_SPARC_WDISP30,
    # 8: R_SPARC_WDISP22,
    9: R_SPARC_HI22,
    # 10: R_SPARC_22,
    # 11: R_SPARC_13,
    12: R_SPARC_LO10,
    # 13: R_SPARC_GOT10,
    # 14: R_SPARC_GOT13,
    # 15: R_SPARC_GOT22,
    # 16: R_SPARC_PC10,
    # 17: R_SPARC_PC22,
    # 18: R_SPARC_WPLT30,
    # 19: R_SPARC_COPY,
    # 20: R_SPARC_GLOB_DAT,
    # 21: R_SPARC_JMP_SLOT,
    # 22: R_SPARC_RELATIVE,
    # 23: R_SPARC_UA32,
}

__all__ = ("relocation_table_sparc",)
