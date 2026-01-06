from ._register_info import REGISTER_OFFSETS
from .enums import default_vex_archinfo, vex_endness_from_string
from .types import Register
from .vex_ffi import guest_offsets


class PyvexArch:
    """
    An architecture definition for use with pyvex - usable version.
    """

    def __init__(self, name: str, bits: int, memory_endness: str, instruction_endness: str = "Iend_BE"):
        self.name = name
        self.bits = bits
        self.memory_endness = memory_endness
        self.instruction_endness = instruction_endness
        self.byte_width = 8
        self.register_list: list[Register] = []
        self.registers: dict[str, tuple[int, int]] = {}
        self.vex_arch = {
            "X86": "VexArchX86",
            "AMD64": "VexArchAMD64",
            "ARM": "VexArchARM",
            "ARM64": "VexArchARM64",
            "PPC32": "VexArchPPC32",
            "PPC64": "VexArchPPC64",
            "S390X": "VexArchS390X",
            "MIPS32": "VexArchMIPS32",
            "MIPS64": "VexArchMIPS64",
            "RISCV64": "VexArchRISCV64",
        }[name]
        self.ip_offset = guest_offsets[
            (
                self.vex_name_small,
                {
                    "X86": "eip",
                    "AMD64": "rip",
                    "ARM": "r15t",
                    "ARM64": "pc",
                    "PPC32": "cia",
                    "PPC64": "cia",
                    "S390X": "ia",
                    "MIPS32": "pc",
                    "MIPS64": "pc",
                    "RISCV64": "pc",
                }[name],
            )
        ]
        self.vex_archinfo = default_vex_archinfo()
        if memory_endness == "Iend_BE":
            self.vex_archinfo["endness"] = vex_endness_from_string("VexEndnessBE")

    def __repr__(self):
        return f"<PyvexArch {self.name}>"

    @property
    def vex_name_small(self):
        return self.vex_arch[7:].lower()

    def translate_register_name(self, offset, size=None):  # pylint: disable=unused-argument
        for (arch, reg), offset2 in guest_offsets.items():
            if arch == self.vex_name_small and offset2 == offset:
                return reg
        for (arch, reg), offset2 in REGISTER_OFFSETS.items():
            if arch == self.vex_name_small and offset2 == offset:
                return reg
        return str(offset)

    def get_register_offset(self, name: str) -> int:
        arch_reg_tuple = (self.vex_name_small, name)
        if arch_reg_tuple in guest_offsets:
            return guest_offsets[arch_reg_tuple]
        elif arch_reg_tuple in REGISTER_OFFSETS:
            return REGISTER_OFFSETS[arch_reg_tuple]
        else:
            raise KeyError(f"Unknown register {name} for architecture {self.name}")


ARCH_X86 = PyvexArch("X86", 32, "Iend_LE")
ARCH_AMD64 = PyvexArch("AMD64", 64, "Iend_LE")
ARCH_ARM_LE = PyvexArch("ARM", 32, "Iend_LE", instruction_endness="Iend_LE")
ARCH_ARM_BE_LE = PyvexArch("ARM", 32, "Iend_BE", instruction_endness="Iend_LE")
ARCH_ARM_BE = PyvexArch("ARM", 32, "Iend_LE")
ARCH_ARM64_LE = PyvexArch("ARM64", 64, "Iend_LE", instruction_endness="Iend_LE")
ARCH_ARM64_BE = PyvexArch("ARM64", 64, "Iend_BE")
ARCH_PPC32 = PyvexArch("PPC32", 32, "Iend_BE")
ARCH_PPC64_BE = PyvexArch("PPC64", 64, "Iend_BE")
ARCH_PPC64_LE = PyvexArch("PPC64", 64, "Iend_LE")
ARCH_S390X = PyvexArch("S390X", 64, "Iend_BE")
ARCH_MIPS32_BE = PyvexArch("MIPS32", 32, "Iend_BE")
ARCH_MIPS32_LE = PyvexArch("MIPS32", 32, "Iend_LE")
ARCH_MIPS64_BE = PyvexArch("MIPS64", 64, "Iend_BE")
ARCH_MIPS64_LE = PyvexArch("MIPS64", 64, "Iend_LE")
ARCH_RISCV64_LE = PyvexArch("RISCV64", 64, "Iend_LE", instruction_endness="Iend_LE")
