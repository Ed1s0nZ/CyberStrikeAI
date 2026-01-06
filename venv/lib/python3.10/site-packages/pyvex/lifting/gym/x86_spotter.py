import logging

from pyvex.lifting.util import GymratLifter, Instruction, JumpKind, Type

log = logging.getLogger(__name__)

# pylint: disable=missing-class-docstring


class Instruction_SWAPGS(Instruction):
    name = "SWAPGS"
    bin_format = "000011110000000111111000"  # 0f 01 f8

    def compute_result(self, *args):
        pass  # TODO check for priv mode


class Instruction_SYSRET(Instruction):
    name = "SYSRET"
    bin_format = "010010000000111100000111"  # 48 04 07

    def compute_result(self, *args):
        result = self.dirty(Type.int_64, "%sg_dirtyhelper_SYSRET" % self.arch.name.lower(), ())
        self.jump(None, result, JumpKind.Ret)


class Instruction_IRETQ(Instruction):
    name = "IRETQ"
    bin_format = "0100100011001111"  # 48 cf

    def compute_result(self, *args):
        result = self.dirty(Type.int_64, "%sg_dirtyhelper_IRETQ" % self.arch.name.lower(), ())
        self.jump(None, result, JumpKind.Ret)


class Instruction_RDMSR(Instruction):
    name = "RDMSR"
    bin_format = "0000111100110010"  # 0f 32

    def compute_result(self, *args):
        ecx = self.get("ecx", Type.int_32)
        result = self.dirty(Type.int_64, "%sg_dirtyhelper_RDMSR" % self.arch.name.lower(), (ecx,))
        edx = result.narrow_high(Type.int_32)
        eax = result.narrow_low(Type.int_32)
        if self.arch.bits == 32:
            self.put(eax, "eax")
            self.put(edx, "edx")
        else:
            self.put(eax.widen_unsigned(Type.int_64), "rax")
            self.put(edx.widen_unsigned(Type.int_64), "rdx")


class Instruction_XGETBV(Instruction):
    name = "XGETBV"
    bin_format = "000011110000000111010000"  # 0f 01 d0

    def compute_result(self, *args):
        ecx = self.get("ecx", Type.int_32)
        result = self.dirty(Type.int_64, "%sg_dirtyhelper_XGETBV" % self.arch.name.lower(), (ecx,))
        edx = result.narrow_high(Type.int_32)
        eax = result.narrow_low(Type.int_32)
        if self.arch.bits == 32:
            self.put(eax, "eax")
            self.put(edx, "edx")
        else:
            self.put(eax.widen_unsigned(Type.int_64), "rax")
            self.put(edx.widen_unsigned(Type.int_64), "rdx")


class Instruction_AAM(Instruction):
    name = "AAM"
    bin_format = "11010100iiiiiiii"

    # From https://www.felixcloutier.com/x86/aam
    def compute_result(self):  # pylint: disable=arguments-differ
        base = self.constant(int(self.data["i"], 2), Type.int_8)
        temp_al = self.get("al", Type.int_8)
        temp_ah = temp_al // base
        temp_al = temp_al % base
        self.put(temp_ah, "ah")
        self.put(temp_al, "al")
        log.debug(
            "The generalized AAM instruction is not supported by VEX, and is handled specially by pyvex."
            " It has no flag handling at present.  See pyvex/lifting/gym/x86_spotter.py for details"
        )

    # TODO: Flags


class Instruction_AAD(Instruction):
    name = "AAD"
    bin_format = "11010101iiiiiiii"

    # From https://www.felixcloutier.com/x86/aad
    def compute_result(self):  # pylint: disable=arguments-differ
        base = self.constant(int(self.data["i"], 2), Type.int_8)
        temp_al = self.get("al", Type.int_8)
        temp_ah = self.get("ah", Type.int_8)
        temp_al = (temp_al + (temp_ah * base)) & 0xFF
        temp_ah = self.constant(0, Type.int_8)
        self.put(temp_ah, "ah")
        self.put(temp_al, "al")
        log.debug(
            "The generalized AAD instruction is not supported by VEX, and is handled specially by pyvex."
            " It has no flag handling at present.  See pyvex/lifting/gym/x86_spotter.py for details"
        )

    # TODO: Flags


class AMD64Spotter(GymratLifter):
    instrs = [
        Instruction_RDMSR,
        Instruction_XGETBV,
        Instruction_AAD,
        Instruction_AAM,
        Instruction_SWAPGS,
        Instruction_IRETQ,
        Instruction_SYSRET,
    ]


class X86Spotter(GymratLifter):
    instrs = [
        Instruction_RDMSR,
        Instruction_XGETBV,
        Instruction_AAD,
        Instruction_AAM,
    ]
