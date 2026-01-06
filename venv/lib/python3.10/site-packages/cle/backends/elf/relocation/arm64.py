"""Relocations for AARCH64

Reference: https://github.com/ARM-software/abi-aa/blob/main/aaelf64/aaelf64.rst#relocation
"""

from __future__ import annotations

import logging

from .elfreloc import ELFReloc
from .generic import (
    GenericAbsoluteAddendReloc,
    GenericCopyReloc,
    GenericIRelativeReloc,
    GenericJumpslotReloc,
    GenericPCRelativeAddendReloc,
    GenericRelativeReloc,
    GenericTLSDescriptorReloc,
    GenericTLSDoffsetReloc,
    GenericTLSModIdReloc,
    GenericTLSOffsetReloc,
)

log = logging.getLogger(name=__name__)


class R_AARCH64_ABS64(GenericAbsoluteAddendReloc):
    pass


class R_AARCH64_COPY(GenericCopyReloc):
    pass


class R_AARCH64_GLOB_DAT(GenericJumpslotReloc):
    pass


class R_AARCH64_JUMP_SLOT(GenericJumpslotReloc):
    pass


class R_AARCH64_RELATIVE(GenericRelativeReloc):
    pass


class R_AARCH64_IRELATIVE(GenericIRelativeReloc):
    pass


class R_AARCH64_TLS_DTPREL(GenericTLSDoffsetReloc):
    pass


class R_AARCH64_TLS_DTPMOD(GenericTLSModIdReloc):
    pass


class R_AARCH64_TLS_TPREL(GenericTLSOffsetReloc):
    pass


class R_AARCH64_TLSDESC(GenericTLSDescriptorReloc):
    RESOLVER_ADDR = 0xFFFF_FFFF_FFFF_FE00


class R_AARCH64_PREL32(GenericPCRelativeAddendReloc):
    """
    Relocation Type: 261
    Calculation: (S + A - P)
    """


class R_AARCH64_JUMP26(ELFReloc):
    """
    Relocation Type: 282
    Calculation: (S + A - P)
    """

    @property
    def value(self):
        A = self.addend
        S = self.resolvedby.rebased_addr
        P = self.rebased_addr
        return S + A - P

    def relocate(self):
        if not self.resolved:
            return False
        if not ((-(2**27)) <= self.value and self.value < (2**27)):
            log.warning("relocation out of range")
        instr = self.owner.memory.unpack_word(self.relative_addr, size=4) & 0b11111100000000000000000000000000
        imm = self.value >> 2 & 0b0000_0011_1111_1111_1111_1111_1111_1111  # [27:2] of the value
        self.owner.memory.pack_word(self.relative_addr, instr | imm, size=4)
        return True


class R_AARCH64_CALL26(ELFReloc):
    """
    Relocation Type: 283
    Calculation: (S + A - P)
    """

    @property
    def value(self):
        A = self.addend
        S = self.resolvedby.rebased_addr
        P = self.rebased_addr
        return S + A - P

    def relocate(self):
        if not self.resolved:
            return False
        if not ((-(2**27)) <= self.value and self.value < (2**27)):
            log.warning("relocation out of range")
        instr = self.owner.memory.unpack_word(self.relative_addr, size=4) & 0b11111100000000000000000000000000
        imm = self.value >> 2 & 0b0000_0011_1111_1111_1111_1111_1111_1111  # [27:2] of the value
        self.owner.memory.pack_word(self.relative_addr, instr | imm, size=4)
        return True


class R_AARCH64_ADR_PREL_PG_HI21(ELFReloc):
    """
    Relocation Type: 275
    Calculation: Page(S + A) - Page(P)
    """

    @property
    def value(self):
        A = self.addend
        S = self.resolvedby.rebased_addr
        P = self.rebased_addr
        return ((S + A) & ~0xFFF) - (P & ~0xFFF)

    def relocate(self):
        if not self.resolved:
            return False
        if not ((-(2**32)) <= self.value and self.value < (2**32)):
            log.warning("relocation out of range")
        instr = self.owner.memory.unpack_word(self.relative_addr, size=4) & 0b10011111000000000000000000011111
        imm = self.value >> 12 & 0x1FFFFF
        immlo = imm & 0b11
        immhi = imm >> 2
        self.owner.memory.pack_word(self.relative_addr, instr | (immhi << 5) | (immlo << 29), size=4)
        return True


class R_AARCH64_ADD_ABS_LO12_NC(ELFReloc):
    """
    Relocation Type: 277
    Calculation: (S + A)
    """

    @property
    def value(self):
        A = self.addend
        S = self.resolvedby.rebased_addr
        return S + A

    def relocate(self):
        if not self.resolved:
            return False
        instr = self.owner.memory.unpack_word(self.relative_addr, size=4) & 0b11111111110000000000001111111111
        imm = self.value & 0b1111_1111_1111
        self.owner.memory.pack_word(self.relative_addr, instr | (imm << 10), size=4)
        return True


class R_AARCH64_LDST8_ABS_LO12_NC(ELFReloc):
    """
    Relocation Type: 278
    Calculation: (S + A)
    """

    @property
    def value(self):
        A = self.addend
        S = self.resolvedby.rebased_addr
        return S + A

    def relocate(self):
        if not self.resolved:
            return False
        instr = self.owner.memory.unpack_word(self.relative_addr, size=4) & 0b11111111110000000000001111111111
        imm = self.value & 0b1111_1111_1111
        self.owner.memory.pack_word(self.relative_addr, instr | (imm << 10), size=4)
        return True


class R_AARCH64_LDST16_ABS_LO12_NC(ELFReloc):
    """
    Relocation Type: 284
    Calculation: (S + A)
    """

    @property
    def value(self):
        A = self.addend
        S = self.resolvedby.rebased_addr
        return S + A

    def relocate(self):
        if not self.resolved:
            return False
        instr = self.owner.memory.unpack_word(self.relative_addr, size=4) & 0b11111111110000000000001111111111
        imm = (self.value & 0b1111_1111_1111) >> 1
        self.owner.memory.pack_word(self.relative_addr, instr | (imm << 10), size=4)
        return True


class R_AARCH64_LDST32_ABS_LO12_NC(ELFReloc):
    """
    Relocation Type: 285
    Calculation: (S + A)
    """

    @property
    def value(self):
        A = self.addend
        S = self.resolvedby.rebased_addr
        return S + A

    def relocate(self):
        if not self.resolved:
            return False
        instr = self.owner.memory.unpack_word(self.relative_addr, size=4) & 0b11111111110000000000001111111111
        imm = (self.value & 0b1111_1111_1111) >> 2
        self.owner.memory.pack_word(self.relative_addr, instr | (imm << 10), size=4)
        return True


class R_AARCH64_LDST64_ABS_LO12_NC(ELFReloc):
    """
    Relocation Type: 286
    Calculation: (S + A)
    """

    @property
    def value(self):
        A = self.addend
        S = self.resolvedby.rebased_addr
        return S + A

    def relocate(self):
        if not self.resolved:
            return False
        instr = self.owner.memory.unpack_word(self.relative_addr, size=4) & 0b11111111110000000000001111111111
        imm = (self.value & 0b1111_1111_1111) >> 3
        self.owner.memory.pack_word(self.relative_addr, instr | (imm << 10), size=4)
        return True


class R_AARCH64_LDST128_ABS_LO12_NC(ELFReloc):
    """
    Relocation Type: 299
    Calculation: (S + A)
    """

    @property
    def value(self):
        A = self.addend
        S = self.resolvedby.rebased_addr
        return S + A

    def relocate(self):
        if not self.resolved:
            return False
        instr = self.owner.memory.unpack_word(self.relative_addr, size=4) & 0b11111111110000000000001111111111
        imm = (self.value & 0b1111_1111_1111) >> 4
        self.owner.memory.pack_word(self.relative_addr, instr | (imm << 10), size=4)
        return True


relocation_table_arm64 = {
    257: R_AARCH64_ABS64,
    258: R_AARCH64_COPY,
    # 259: R_AARCH64_ABS16,
    # 260: R_AARCH64_PREL64,
    261: R_AARCH64_PREL32,
    # 262: R_AARCH64_PREL16,
    # 263: R_AARCH64_MOVW_UABS_G0,
    # 264: R_AARCH64_MOVW_UABS_G0_NC,
    # 265: R_AARCH64_MOVW_UABS_G1,
    # 266: R_AARCH64_MOVW_UABS_G1_NC,
    # 267: R_AARCH64_MOVW_UABS_G2,
    # 268: R_AARCH64_MOVW_UABS_G2_NC,
    # 269: R_AARCH64_MOVW_UABS_G3,
    # 270: R_AARCH64_MOVW_SABS_G0,
    # 271: R_AARCH64_MOVW_SABS_G1,
    # 272: R_AARCH64_MOVW_SABS_G2,
    # 273: R_AARCH64_LD_PREL_LO19,
    # 274: R_AARCH64_ADR_PREL_LO21,
    275: R_AARCH64_ADR_PREL_PG_HI21,
    # 276: R_AARCH64_ADR_PREL_PG_HI21_NC,
    277: R_AARCH64_ADD_ABS_LO12_NC,
    278: R_AARCH64_LDST8_ABS_LO12_NC,
    # 279: R_AARCH64_TSTBR14,
    # 280: R_AARCH64_CONDBR19,
    282: R_AARCH64_JUMP26,
    283: R_AARCH64_CALL26,
    284: R_AARCH64_LDST16_ABS_LO12_NC,
    285: R_AARCH64_LDST32_ABS_LO12_NC,
    286: R_AARCH64_LDST64_ABS_LO12_NC,
    # 287: R_AARCH64_MOVW_PREL_G0,
    # 288: R_AARCH64_MOVW_PREL_G0_NC,
    # 289: R_AARCH64_MOVW_PREL_G1,
    # 290: R_AARCH64_MOVW_PREL_G1_NC,
    # 291: R_AARCH64_MOVW_PREL_G2,
    # 292: R_AARCH64_MOVW_PREL_G2_NC,
    # 293: R_AARCH64_MOVW_PREL_G3,
    299: R_AARCH64_LDST128_ABS_LO12_NC,
    # 300: R_AARCH64_MOVW_GOTOFF_G0,
    # 301: R_AARCH64_MOVW_GOTOFF_G0_NC,
    # 302: R_AARCH64_MOVW_GOTOFF_G1,
    # 303: R_AARCH64_MOVW_GOTOFF_G1_NC,
    # 304: R_AARCH64_MOVW_GOTOFF_G2,
    # 305: R_AARCH64_MOVW_GOTOFF_G2_NC,
    # 306: R_AARCH64_MOVW_GOTOFF_G3,
    # 307: R_AARCH64_GOTREL64,
    # 308: R_AARCH64_GOTREL32,
    # 309: R_AARCH64_GOT_LD_PREL19,
    # 310: R_AARCH64_LD64_GOTOFF_LO15,
    # 311: R_AARCH64_ADR_GOT_PAGE,
    # 312: R_AARCH64_LD64_GOT_LO12_NC,
    # 313: R_AARCH64_LD64_GOTPAGE_LO15,
    # 314: R_AARCH64_PLT32,
    # 315: R_AARCH64_GOTPCREL32,
    # 512: R_AARCH64_TLSGD_ADR_PREL21,
    # 513: R_AARCH64_TLSGD_ADR_PAGE21,
    # 514: R_AARCH64_TLSGD_ADD_LO12_NC,
    # 515: R_AARCH64_TLSGD_MOVW_G1,
    # 516: R_AARCH64_TLSGD_MOVW_G0_NC,
    # 517: R_AARCH64_TLSLD_ADR_PREL21,
    # 518: R_AARCH64_TLSLD_ADR_PAGE21,
    # 519: R_AARCH64_TLSLD_ADD_LO12_NC,
    # 520: R_AARCH64_TLSLD_MOVW_G1,
    # 521: R_AARCH64_TLSLD_MOVW_G0_NC,
    # 522: R_AARCH64_TLSLD_LD_PREL19,
    # 523: R_AARCH64_TLSLD_MOVW_DTPREL_G2,
    # 524: R_AARCH64_TLSLD_MOVW_DTPREL_G1,
    # 525: R_AARCH64_TLSLD_MOVW_DTPREL_G1_NC,
    # 526: R_AARCH64_TLSLD_MOVW_DTPREL_G0,
    # 527: R_AARCH64_TLSLD_MOVW_DTPREL_G0_NC,
    # 528: R_AARCH64_TLSLD_ADD_DTPREL_HI12,
    # 529: R_AARCH64_TLSLD_ADD_DTPREL_LO12,
    # 530: R_AARCH64_TLSLD_ADD_DTPREL_LO12_NC,
    # 531: R_AARCH64_TLSLD_LDST8_DTPREL_LO12,
    # 532: R_AARCH64_TLSLD_LDST8_DTPREL_LO12_NC,
    # 533: R_AARCH64_TLSLD_LDST16_DTPREL_LO12,
    # 534: R_AARCH64_TLSLD_LDST16_DTPREL_LO12_NC,
    # 535: R_AARCH64_TLSLD_LDST32_DTPREL_LO12,
    # 536: R_AARCH64_TLSLD_LDST32_DTPREL_LO12_NC,
    # 537: R_AARCH64_TLSLD_LDST64_DTPREL_LO12,
    # 538: R_AARCH64_TLSLD_LDST64_DTPREL_LO12_NC,
    # 539: R_AARCH64_TLSIE_MOVW_GOTTPREL_G1,
    # 540: R_AARCH64_TLSIE_MOVW_GOTTPREL_G0_NC,
    # 541: R_AARCH64_TLSIE_ADR_GOTTPREL_PAGE21,
    # 542: R_AARCH64_TLSIE_LD64_GOTTPREL_LO12_NC,
    # 543: R_AARCH64_TLSIE_LD_GOTTPREL_PREL19,
    # 544: R_AARCH64_TLSLE_MOVW_TPREL_G2,
    # 545: R_AARCH64_TLSLE_MOVW_TPREL_G1,
    # 546: R_AARCH64_TLSLE_MOVW_TPREL_G1_NC,
    # 547: R_AARCH64_TLSLE_MOVW_TPREL_G0,
    # 548: R_AARCH64_TLSLE_MOVW_TPREL_G0_NC,
    # 549: R_AARCH64_TLSLE_ADD_TPREL_HI12,
    # 550: R_AARCH64_TLSLE_ADD_TPREL_LO12,
    # 551: R_AARCH64_TLSLE_ADD_TPREL_LO12_NC,
    # 552: R_AARCH64_TLSLE_LDST8_TPREL_LO12,
    # 553: R_AARCH64_TLSLE_LDST8_TPREL_LO12_NC,
    # 554: R_AARCH64_TLSLE_LDST16_TPREL_LO12,
    # 555: R_AARCH64_TLSLE_LDST16_TPREL_LO12_NC,
    # 556: R_AARCH64_TLSLE_LDST32_TPREL_LO12,
    # 557: R_AARCH64_TLSLE_LDST32_TPREL_LO12_NC,
    # 558: R_AARCH64_TLSLE_LDST64_TPREL_LO12,
    # 559: R_AARCH64_TLSLE_LDST64_TPREL_LO12_NC,
    # 560: R_AARCH64_TLSDESC_LD_PREL19,
    # 561: R_AARCH64_TLSDESC_ADR_PREL21,
    # 562: R_AARCH64_TLSDESC_ADR_PAGE21,
    # 563: R_AARCH64_TLSDESC_LD64_LO12,
    # 564: R_AARCH64_TLSDESC_ADD_LO12,
    # 565: R_AARCH64_TLSDESC_OFF_G1,
    # 566: R_AARCH64_TLSDESC_OFF_G0_NC,
    # 567: R_AARCH64_TLSDESC_LDR,
    # 568: R_AARCH64_TLSDESC_ADD,
    # 569: R_AARCH64_TLSDESC_CALL,
    # 570: R_AARCH64_TLSLE_LDST128_TPREL_LO12,
    # 571: R_AARCH64_TLSLE_LDST128_TPREL_LO12_NC,
    # 572: R_AARCH64_TLSLD_LDST128_DTPREL_LO12,
    # 573: R_AARCH64_TLSLD_LDST128_DTPREL_LO12_NC,
    # 580: R_AARCH64_AUTH_ABS64,
    1024: R_AARCH64_COPY,
    1025: R_AARCH64_GLOB_DAT,
    1026: R_AARCH64_JUMP_SLOT,
    1027: R_AARCH64_RELATIVE,
    1028: R_AARCH64_TLS_DTPMOD,  # R_AARCH64_TLS_IMPDEF1
    1029: R_AARCH64_TLS_DTPREL,  # R_AARCH64_TLS_IMPDEF2
    1030: R_AARCH64_TLS_TPREL,
    1031: R_AARCH64_TLSDESC,
    1032: R_AARCH64_IRELATIVE,
    # 1040: R_AARCH64_AUTH_ABS64,
    # 1041: R_AARCH64_AUTHELATIVE,
}

__all__ = ("relocation_table_arm64",)
