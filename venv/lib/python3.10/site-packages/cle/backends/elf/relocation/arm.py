"""Relocation types for ARM.

Reference: https://github.com/ARM-software/abi-aa/blob/main/aaelf32/aaelf32.rst#relocation-codes
"""

from __future__ import annotations

import logging

from cle.errors import CLEOperationError

from .elfreloc import ELFReloc
from .generic import (
    GenericAbsoluteAddendReloc,
    GenericCopyReloc,
    GenericJumpslotReloc,
    GenericPCRelativeAddendReloc,
    GenericRelativeReloc,
    GenericTLSDoffsetReloc,
    GenericTLSModIdReloc,
    GenericTLSOffsetReloc,
    RelocGOTMixin,
    RelocTruncate32Mixin,
)

log = logging.getLogger(name=__name__)


def _applyReloc(inst, result, mask=0xFFFFFFFF):
    """
    Applies the specified mask to the relocation and verifies that the mask
    is valid for the given result.
    """
    try:
        if result & ~mask:
            raise ValueError("result & ~mask is not 0.")
    except ValueError as ex:
        log.warning("Relocation failed: %r", ex)
        return 0  # worst case, you hook it yourself
    return (inst & ~mask) | (result & mask)  # pylint: disable=superfluous-parens


def _isThumbFunc(symbol, addr):
    """
    Checks whether the provided symbol and address is a Thumb function by
    verifying the LSB is 1 and the symbol is STT_FUNC.
    """
    return (addr % 2 == 1) and symbol.is_function


class R_ARM_CALL(ELFReloc):
    """
    Relocate R_ARM_CALL symbols via instruction modification. It additionally
    handles R_ARM_PC24 and R_ARM_JUMP24. The former is deprecated and is now
    just the same as R_ARM_CALL.

    R_ARM_JUMP24 doesn't need the Thumb check. Technically, if the Thumb check
    succeeds on R_ARM_JUMP24, it's a bad call that shouldn't have been generated
    by the linker, so we may as well as just treat it like R_ARM_CALL.

    - Class: Static
    - Type: ARM (R_ARM_CALL, R_ARM_JUMP24); Deprecated (R_ARM_PC24)
    - Code: 1 (R_ARM_PC24), 28 (R_ARM_CALL), 29 (R_ARM_JUMP24)
    - Operation: ((S + A) | T) - P
      - S is the address of the symbol
      - A is the addend
      - P is the target location (place being relocated)
      - T is 1 if the symbol is of type STT_FUNC and addresses a Thumb instruction
    """

    @property
    def value(self):
        P = self.rebased_addr  # Location of this instruction
        A = inst = self.addend  # The instruction
        S = self.resolvedby.rebased_addr  # The symbol's "value", where it points to
        T = _isThumbFunc(self.symbol, S)

        if inst & 0x00800000:
            A |= 0xFF000000  # Sign extend to 32-bits
        result = ((S + (A << 2)) | T) - P  # Do the initial work
        imm24 = (result & 0x03FFFFFE) >> 2  # Sign_extend(inst[25:2])

        if T:  # Do Thumb relocation
            mask = 0xFF000000
            bit_h = (result & 0x02) >> 1
            result = _applyReloc(inst, (0xFA | bit_h), mask)
        else:  # Do ARM relocation
            mask = 0xFFFFFF
            result = _applyReloc(inst, imm24, mask)

        log.debug("%s relocated as R_ARM_CALL with new instruction: %#x", self.symbol.name, result)
        return result


class R_ARM_PREL31(ELFReloc):
    """
    Relocate R_ARM_PREL31 symbols via instruction modification. The difference
    between this and R_ARM_CALL/R_ARM_PC24/R_ARM_JUMP24 is that it's a data
    relocation

    - Class: Static
    - Type: Data
    - Code: 42
    - Operation: ((S + A) | T) - P
      - S is the address of the symbol
      - A is the addend
      - P is the target location (place being relocated)
      - T is 1 if the symbol is of type STT_FUNC and addresses a Thumb instruction
    """

    @property
    def value(self):
        P = self.rebased_addr  # Location of this instruction
        A = self.addend  # The instruction
        S = self.resolvedby.rebased_addr  # The symbol's "value", where it points to
        T = _isThumbFunc(self.symbol, S)

        if A & 0x01000000:
            A |= 0xF1000000  # Sign extend 31-bits
        result = ((S + A) | T) - P  # Do the initial work
        mask = 0x7FFFFFFF
        rel31 = result & mask
        result = _applyReloc(A, rel31, mask)
        log.debug("%s relocated as R_ARM_PREL31 to: 0x%x", self.symbol.name, result)
        return result


class R_ARM_REL32(ELFReloc):
    """
    Relocate R_ARM_REL32 symbols. This is essentially the same as
    GenericPCRelativeAddendReloc with the addition of a check
    for whether or not the target is Thumb.

    - Class: Static
    - Type: Data
    - Code: 3
    - Operation: ((S + A) | T) - P
      - S is the address of the symbol
      - A is the addend
      - P is the target location (place being relocated)
      - T is 1 if the symbol is of type STT_FUNC and addresses a Thumb instruction
    """

    @property
    def value(self):
        P = self.rebased_addr  # Location of this instruction
        A = self.addend  # The instruction
        S = self.resolvedby.rebased_addr  # The symbol's "value", where it points to
        T = _isThumbFunc(self.symbol, S)
        result = ((S + A) | T) - P
        log.debug("%s relocated as R_ARM_REL32 to: 0x%x", self.symbol.name, result)
        return result


class R_ARM_ABS32(ELFReloc):
    """
    Relocate R_ARM_ABS32 symbols. This is essentially the same as
    GenericAbsoluteAddendReloc with the addition of a check
    for whether or not the target is Thumb.

    - Class: Static
    - Type: Data
    - Code: 3
    - Operation: (S + A) | T
      - S is the address of the symbol
      - A is the addend
      - T is 1 if the symbol is of type STT_FUNC and addresses a Thumb instruction
    """

    @property
    def value(self):
        A = self.addend  # The instruction
        S = self.resolvedby.rebased_addr  # The symbol's "value", where it points to
        T = _isThumbFunc(self.symbol, S)
        result = (S + A) | T
        log.debug("%s relocated as R_ARM_ABS32 to: 0x%x", self.symbol.name, result)
        return result


class R_ARM_MOVW_ABS_NC(ELFReloc):
    """
    Relocate R_ARM_MOVW_ABS_NC symbols.

    - Class: Static
    - Type: Instruction
    - Code: 43
    - Operation: (S + A) | T
      - S is the address of the symbol
      - A is the addend
      - T is 1 if the symbol is of type STT_FUNC and addresses a Thumb instruction
    """

    @property
    def value(self):
        inst = self.addend  # The instruction
        S = self.resolvedby.rebased_addr  # The symbol's "value", where it points to
        T = _isThumbFunc(self.symbol, S)
        # initial addend is formed by interpreting the 16-bit literal field
        # of the instruction as a signed value
        A = ((inst & 0xF0000) >> 4) | (inst & 0xFFF)
        if A & 0x8000:
            # two's complement
            A = -((A ^ 0xFFFF) + 1)
        X = (S + A) | T
        MaskX = X & 0xFFFF
        # inst modification:
        part1 = MaskX >> 12
        part2 = MaskX & 0xFFF
        inst &= 0xFFF0F000  # clears inst[11, 0] and inst[19, 16]
        inst |= (part1 << 16) & 0xF0000  # inst[19, 16] = part1
        inst |= part2 & 0xFFF  # inst[11, 0] = part2
        log.debug("%s relocated as R_ARM_MOVW_ABS_NC to: 0x%x", self.symbol.name, inst)
        return inst


class R_ARM_MOVT_ABS(ELFReloc):
    """
    Relocate R_ARM_MOVT_ABS symbols.

    - Class: Static
    - Type: Instruction
    - Code: 44
    - Operation: S + A
      - S is the address of the symbol
      - A is the addend
    """

    @property
    def value(self):
        inst = self.addend  # The instruction
        S = self.resolvedby.rebased_addr  # The symbol's "value", where it points to
        # initial addend is formed by interpreting the 16-bit literal field
        # of the instruction as a signed value
        A = ((inst & 0xF0000) >> 4) | (inst & 0xFFF)
        if A & 0x8000:
            # two's complement
            A = -((A ^ 0xFFFF) + 1)
        X = S + A
        MaskX = X & 0xFFFF0000
        # inst modification:
        part1 = (MaskX >> 16) >> 12
        part2 = (MaskX >> 16) & 0xFFF
        inst &= 0xFFF0F000  # clears inst[11, 0] and inst[19, 16]
        inst |= (part1 << 16) & 0xF0000  # inst[19, 16] = part1
        inst |= part2 & 0xFFF  # inst[11, 0] = part2
        log.debug("%s relocated as R_ARM_MOVT_ABS to: 0x%x", self.symbol.name, inst)
        return inst


class R_ARM_THM_CALL(ELFReloc):
    """
    Relocate R_ARM_THM_CALL symbols via instruction modification.

    - Class: Static
    - Type: ARM (R_ARM_THM_CALL)
    - Code: 10
    - Operation: ((S + A) | T) - P

      - S is the address of the symbol
      - A is the addend
      - P is the target location (place being relocated)
      - T is 1 if the symbol is of type STT_FUNC and addresses a Thumb instruction (This bit is entirely irrelevant
        because the 1-bit of the address gets shifted off in the encoding)

    - Encoding: See http://hermes.wings.cs.wisc.edu/files/Thumb-2SupplementReferenceManual.pdf

      - Page 71 (3-31) has the chart
      - It appears that it mistakenly references the I1 and I2 bits as J1 and J2 in the chart (see the notes at the
        bottom of the page -- the ranges don't make sense)

      - However, the J1/J2 bits are XORed with !S bit in this case (see vex implementation:
        https://github.com/angr/vex/blob/6d1252c7ce8fe8376318b8f8bb8034058454c841/priv/guest_arm_toIR.c#L19219 )

      - Implementation appears correct with the bits placed into offset[23:22]
    """

    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self._insn_bytes = None

    def resolve_symbol(self, solist, **kwargs):
        kwargs["thumb"] = True
        super().resolve_symbol(solist, **kwargs)

    @property
    def value(self):
        P = self.rebased_addr  # Location of this instruction
        S = self.resolvedby.rebased_addr  # The symbol's "value", where it points to
        T = _isThumbFunc(self.symbol, S)
        A = 0

        # Deconstruct the instruction:
        #  Because this 4-byte instruction is treated as two 2-byte instructions,
        #  the bytes are in the order `b3 b4 b1 b2`, where b4 is the most significant.

        if self._insn_bytes is None:
            self._insn_bytes = self.owner.memory.load(self.relative_addr, 4)

        hi = (self._insn_bytes[1] << 8) | self._insn_bytes[0]
        lo = (self._insn_bytes[3] << 8) | self._insn_bytes[2]
        inst = (hi << 16) | lo

        def gen_mask(n_bits, first_bit):
            """
            Builds a mask that captures n_bits, where the first bit captured is first_bit
            """
            return ((1 << n_bits) - 1) << first_bit

        if self.is_rela:
            A = self.addend
        else:
            # Build A (the initial addend)

            A |= (inst & gen_mask(11, 0)) << 1  # A[11:1]  = inst[10:0] (inclusive)
            A |= ((inst & gen_mask(10, 16)) >> 16) << 12  # A[21:12] = inst[25:16]

            sign_bit = bool(inst & gen_mask(1, 26)) & 1  # sign_bit = inst[26]

            J1 = (bool(inst & gen_mask(1, 13)) & 1) ^ (not sign_bit)  # J1 = inst[13] ^ !sign
            J2 = (bool(inst & gen_mask(1, 11)) & 1) ^ (not sign_bit)  # J2 = inst[11] ^ !sign

            A |= J1 << 23  # A[23] = J1
            A |= J2 << 22  # A[22] = J2

            A &= 0x7FFFFF

            if sign_bit:
                A |= 0xFF800000

        # Compute X, the new offset, from the symbol addr, S, the addend, A,
        #  the thumb flag, T, and PC, P.

        x = (((S + A) | T) - P) & 0xFFFFFFFF  # Also mask to 32 bits

        # Ensure jump is in range

        if x & 0xFF800000 != 0 and x & 0xFF800000 != 0xFF800000:
            raise CLEOperationError(
                "Jump target out of range for reloc R_ARM_THM_CALL (+- 2^23). "
                "This may be due to SimProcedures being allocated outside the jump range. "
                "If you believe this is the case, set 'rebase_granularity'=0x1000 in the "
                "load options."
            )

        # Rebuild the instruction, first clearing out any previously set offset bits

        #                 offset     1 2  offset
        #          11110S [21:12]  11J?J  [11:1]     (if ? is 1, BL; if ? is 0, BLX)
        inst &= ~0b00000111111111110010111111111111
        #         |       |       |       |       |
        #        32      24      16       8       0

        sign_bit = bool(x & gen_mask(1, 24)) & 1
        J1 = (bool(x & gen_mask(1, 23)) & 1) ^ (not sign_bit)
        J2 = (bool(x & gen_mask(1, 22)) & 1) ^ (not sign_bit)

        inst |= sign_bit << 26
        inst |= J1 << 13
        inst |= J2 << 11

        inst |= (x & gen_mask(11, 1)) >> 1
        inst |= ((x & gen_mask(10, 12)) >> 12) << 16

        # Put it back into <little endian short> <little endian short> format

        raw = ((inst & 0x00FF0000) >> 16, (inst & 0xFF000000) >> 24, (inst & 0x00FF), (inst & 0xFF00) >> 8)

        # The relocation handler expects a little-endian result, so flip it around.

        result = (raw[3] << 24) | (raw[2] << 16) | (raw[1] << 8) | raw[0]

        log.debug("%s relocated as R_ARM_THM_CALL with new instruction: %#x", self.symbol.name, result)
        return result


class R_ARM_COPY(GenericCopyReloc):
    pass


class R_ARM_GLOB_DAT(GenericJumpslotReloc):
    pass


class R_ARM_JUMP_SLOT(GenericJumpslotReloc):
    pass


class R_ARM_RELATIVE(GenericRelativeReloc):
    pass


class R_ARM_ABS32_NOI(GenericAbsoluteAddendReloc):
    pass


class R_ARM_REL32_NOI(GenericPCRelativeAddendReloc):
    pass


class R_ARM_TLS_DTPMOD32(GenericTLSModIdReloc):
    pass


class R_ARM_TLS_DTPOFF32(GenericTLSDoffsetReloc):
    pass


class R_ARM_TLS_TPOFF32(GenericTLSOffsetReloc):
    pass


class R_ARM_JUMP24(R_ARM_CALL):
    pass


class R_ARM_PC24(R_ARM_CALL):
    pass


# EDG says: Implementing these the easy way.
# Inaccuracies may exist.  This is ARM, after all.
class R_ARM_THM_JUMP24(R_ARM_THM_CALL):
    pass


class R_ARM_THM_JUMP19(R_ARM_THM_CALL):
    pass


class R_ARM_THM_JUMP6(R_ARM_THM_CALL):
    pass


class R_ARM_THM_MOVW_ABS_NC(ELFReloc):
    """
    ((S + A) | T) & 0xffff
    Ref: https://github.com/ARM-software/abi-aa/blob/main/aaelf32/aaelf32.rst
    """

    @property
    def value(self):
        insn_bytes = self.owner.memory.load(self.relative_addr, 4)
        hi = (insn_bytes[1] << 8) | insn_bytes[0]
        lo = (insn_bytes[3] << 8) | insn_bytes[2]
        inst = (hi << 16) | lo
        S = self.resolvedby.rebased_addr  # The symbol's "value", where it points to
        # initial addend is formed by interpreting the 16-bit literal field
        # of the instruction as a signed value
        A = (inst & 0b0000_0100_0000_0000_0000_0000_0000_0000) >> 26 << 15
        A |= (inst & 0b0000_0000_0000_1111_0000_0000_0000_0000) >> 16 << 11
        A |= (inst & 0b0000_0000_0000_0000_0111_0000_0000_0000) >> 12 << 8
        A |= inst & 0b0000_0000_0000_0000_0000_0000_1111_1111
        if A & 0x8000:
            # two's complement
            A = -((A ^ 0xFFFF) + 1)
        T = _isThumbFunc(self.symbol, S)
        X = (S + A) | T
        MaskX = X & 0xFFFF
        # inst modification:
        part1 = MaskX >> 12  # [19:16]
        part2 = (MaskX >> 11) & 0x1  # [26]
        part3 = (MaskX >> 8) & 0x7  # [14:12]
        part4 = MaskX & 0xFF  # [7:0]
        inst &= 0b1111_1011_1111_0000_1000_1111_0000_0000
        inst |= (part1 << 16) & 0b0000_0000_0000_1111_0000_0000_0000_0000
        inst |= (part2 << 26) & 0b0000_0100_0000_0000_0000_0000_0000_0000
        inst |= (part3 << 12) & 0b0000_0000_0000_0000_0111_0000_0000_0000
        inst |= (part4 << 0) & 0b0000_0000_0000_0000_0000_0000_1111_1111
        raw = ((inst & 0x00FF0000) >> 16, (inst & 0xFF000000) >> 24, (inst & 0x00FF), (inst & 0xFF00) >> 8)
        inst = (raw[3] << 24) | (raw[2] << 16) | (raw[1] << 8) | raw[0]
        log.debug("%s relocated as R_ARM_THM_MOVW_ABS_NC to: 0x%x", self.symbol.name, inst)
        return inst


class R_ARM_THM_MOVT_ABS(ELFReloc):
    """
    (S + A) & 0xffff0000
    Ref: https://github.com/ARM-software/abi-aa/blob/main/aaelf32/aaelf32.rst
    """

    @property
    def value(self):
        insn_bytes = self.owner.memory.load(self.relative_addr, 4)
        hi = (insn_bytes[1] << 8) | insn_bytes[0]
        lo = (insn_bytes[3] << 8) | insn_bytes[2]
        inst = (hi << 16) | lo
        S = self.resolvedby.rebased_addr  # The symbol's "value", where it points to
        # initial addend is formed by interpreting the 16-bit literal field
        # of the instruction as a signed value
        A = (inst & 0b0000_0100_0000_0000_0000_0000_0000_0000) >> 26 << 15
        A |= (inst & 0b0000_0000_0000_1111_0000_0000_0000_0000) >> 16 << 11
        A |= (inst & 0b0000_0000_0000_0000_0111_0000_0000_0000) >> 12 << 8
        A |= inst & 0b0000_0000_0000_0000_0000_0000_1111_1111
        if A & 0x8000:
            # two's complement
            A = -((A ^ 0xFFFF) + 1)
        X = S + A
        MaskX = X & 0xFFFF0000
        # inst modification:
        part1 = MaskX >> 28  # [19:16]
        part2 = (MaskX >> 27) & 0x1  # [26]
        part3 = (MaskX >> 24) & 0x7  # [14:12]
        part4 = (MaskX >> 16) & 0xFF  # [7:0]
        inst &= 0b1111_1011_1111_0000_1000_1111_0000_0000
        inst |= (part1 << 16) & 0b0000_0000_0000_1111_0000_0000_0000_0000
        inst |= (part2 << 26) & 0b0000_0100_0000_0000_0000_0000_0000_0000
        inst |= (part3 << 12) & 0b0000_0000_0000_0000_0111_0000_0000_0000
        inst |= (part4 << 0) & 0b0000_0000_0000_0000_0000_0000_1111_1111
        raw = ((inst & 0x00FF0000) >> 16, (inst & 0xFF000000) >> 24, (inst & 0x00FF), (inst & 0xFF00) >> 8)
        inst = (raw[3] << 24) | (raw[2] << 16) | (raw[1] << 8) | raw[0]
        log.debug("%s relocated as R_ARM_THM_MOVT_ABS to: 0x%x", self.symbol.name, inst)
        return inst


class R_ARM_GOT_PREL(GenericPCRelativeAddendReloc, RelocTruncate32Mixin, RelocGOTMixin):
    """
    GOT(S) + A - P
    Ref: https://github.com/ARM-software/abi-aa/blob/main/aaelf32/aaelf32.rst
    """


relocation_table_arm = {
    1: R_ARM_PC24,
    2: R_ARM_ABS32,
    3: R_ARM_REL32,
    # 4: ARM_LDR_PC_G0,
    # 5: R_ARM_ABS16,
    # 6: R_ARM_ABS12,
    # 7: R_ARM_THM_ABS5,
    # 8: R_ARM_ABS8,
    # 9: R_ARM_SBREL32,
    10: R_ARM_THM_CALL,
    # 11: R_ARM_THM_PC8,
    # 12: R_ARM_BREL_ADJ,
    # 13: R_ARM_TLS_DESC,
    # 14: R_ARM_THM_SWI8,
    # 15: R_ARM_XPC25,
    # 16: R_ARM_THM_XPC22,
    17: R_ARM_TLS_DTPMOD32,
    18: R_ARM_TLS_DTPOFF32,
    19: R_ARM_TLS_TPOFF32,
    20: R_ARM_COPY,
    21: R_ARM_GLOB_DAT,
    22: R_ARM_JUMP_SLOT,
    23: R_ARM_RELATIVE,
    # 24: R_ARM_GOTOFF,
    # 25: R_ARM_BASE_PREL,
    # 26: R_ARM_GOT_BREL,
    # 27: R_ARM_PLT32,
    28: R_ARM_CALL,
    29: R_ARM_JUMP24,
    30: R_ARM_THM_JUMP24,
    # 31: R_ARM_BASE_ABS,
    # 32: R_ARM_ALU_PCREL_7_0,
    # 33: R_ARM_ALU_PCREL_15_8,
    # 34: R_ARM_ALU_PCREL_23_15,
    # 35: R_ARM_LDR_SBREL_11_0_NC,
    # 36: R_ARM_ALU_SBREL_19_12_NC,
    # 37: R_ARM_ALU_SBREL_27_20_CK,
    # 38: R_ARM_TARGET1,
    # 39: R_ARM_SBREL31,
    # 40: R_ARM_V4BX,
    # 41: R_ARM_TARGET2,
    42: R_ARM_PREL31,
    43: R_ARM_MOVW_ABS_NC,
    44: R_ARM_MOVT_ABS,
    # 45: R_ARM_MOVW_PREL_NC,
    # 46: R_ARM_MOVT_PREL,
    47: R_ARM_THM_MOVW_ABS_NC,
    48: R_ARM_THM_MOVT_ABS,
    # 49: R_ARM_THM_MOVW_PREL_NC,
    # 50: R_ARM_THM_MOVT_PREL,
    51: R_ARM_THM_JUMP19,
    52: R_ARM_THM_JUMP6,
    # 53: R_ARM_THM_ALU_PREL_11_0,
    # 54: R_ARM_THM_PC12,
    55: R_ARM_ABS32_NOI,
    56: R_ARM_REL32_NOI,
    # 57: R_ARM_ALU_PC_G0_NC,
    # 58: R_ARM_ALU_PC_G0,
    # 59: R_ARM_ALU_PC_G1_NC,
    # 60: R_ARM_ALU_PC_G1,
    # 61: R_ARM_ALU_PC_G2,
    # 62: R_ARM_LDR_PC_G1,
    # 63: R_ARM_LDR_PC_G2,
    # 64: R_ARM_LDRS_PC_G0,
    # 65: R_ARM_LDRS_PC_G1,
    # 66: R_ARM_LDRS_PC_G2,
    # 67: R_ARM_LDC_PC_G0,
    # 68: R_ARM_LDC_PC_G1,
    # 69: R_ARM_LDC_PC_G2,
    # 70: R_ARM_ALU_SB_G0_NC,
    # 71: R_ARM_ALU_SB_G0,
    # 72: R_ARM_ALU_SB_G1_NC,
    # 73: R_ARM_ALU_SB_G1,
    # 74: R_ARM_ALU_SB_G2,
    # 75: R_ARM_LDR_SB_G0,
    # 76: R_ARM_LDR_SB_G1,
    # 77: R_ARM_LDR_SB_G2,
    # 78: R_ARM_LDRS_SB_G0,
    # 79: R_ARM_LDRS_SB_G1,
    # 80: R_ARM_LDRS_SB_G2,
    # 81: R_ARM_LDC_SB_G0,
    # 82: R_ARM_LDC_SB_G1,
    # 83: R_ARM_LDC_SB_G2,
    # 84: R_ARM_MOVW_BREL_NC,
    # 85: R_ARM_MOVT_BREL,
    # 86: R_ARM_MOVW_BREL,
    # 87: R_ARM_THM_MOVW_BREL_NC,
    # 88: R_ARM_THM_MOVT_BREL,
    # 89: R_ARM_THM_MOVW_BREL,
    # 90: R_ARM_TLS_GOTDESC,
    # 91: R_ARM_TLS_CALL,
    # 92: R_ARM_TLS_DESCSEQ,
    # 93: R_ARM_THM_TLS_CALL,
    # 94: R_ARM_PLT32_ABS,
    # 95: R_ARM_GOT_ABS,
    96: R_ARM_GOT_PREL,
    # 97: R_ARM_GOT_BREL12,
    # 98: R_ARM_GOTOFF12,
    # 99: R_ARM_GOTRELAX,
    # 100: R_ARM_GNU_VTENTRY,
    # 101: R_ARM_GNU_VTINHERIT,
    # 102: R_ARM_JUMP11,
    # 103: R_ARM_THM_PC9,
    # 104: R_ARM_TLS_GD32,
    # 105: R_ARM_TLS_LDM32,
    # 106: R_ARM_TLS_LDO32,
    # 107: R_ARM_TLS_IE32,
    # 108: R_ARM_TLS_LE32,
    # 109: R_ARM_TLS_LDO12,
    # 110: R_ARM_TLS_LE12,
    # 111: R_ARM_TLS_IE12GP,
    # 112-127: R_ARM_PRIVATE_<n>
    # 128: R_ARM_ME_TOO,
    # 129: R_ARM_THM_TLS_DESCSEQ16,
    # 130: R_ARM_THM_TLS_DESCSEQ32,
    # 131: R_ARM_THM_GOT_BREL12,
    # 132: R_ARM_THM_ALU_ABS_G0_NC,
    # 133: R_ARM_THM_ALU_ABS_G1_NC,
    # 134: R_ARM_THM_ALU_ABS_G2_NC,
    # 135: R_ARM_THM_ALU_ABS_G3,
    # 136: R_ARM_THM_BF16,
    # 137: R_ARM_THM_BF12,
    # 138: R_ARM_THM_BF18,
}


__all__ = ("relocation_table_arm",)
