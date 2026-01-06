from archinfo.types import RegisterOffset

from .arch import Arch, Endness, Register, register_arch
from .tls import TLSArchInfo

try:
    import capstone as _capstone
except ImportError:
    _capstone = None

try:
    import keystone as _keystone
except ImportError:
    _keystone = None

try:
    import unicorn as _unicorn
    from unicorn import riscv_const
except ImportError:
    _unicorn = None


def is_riscv_arch(a):
    return a.name.startswith("RISCV")


class ArchRISCV64(Arch):
    def __init__(self, endness=Endness.LE):
        super().__init__(endness)
        if endness == Endness.BE:
            self.function_prologs = set()
            self.function_epilogs = set()

    bits = 64
    vex_arch = "VexArchRISCV64"
    name = "RISCV64"
    qemu_name = "riscv64"
    linux_name = "riscv64"
    triplet = "riscv64-linux-gnu"
    max_inst_bytes = 4
    ret_offset = RegisterOffset(4)
    vex_conditional_helpers = True
    syscall_num_offset = 132
    call_pushes_ret = False
    memory_endness = Endness.LE
    register_endness = Endness.LE
    instruction_endness = Endness.LE
    sizeof = {"short": 16, "int": 32, "long": 64, "long long": 64}
    if _capstone and hasattr(_capstone, "CS_ARCH_RISCV"):
        cs_arch = _capstone.CS_ARCH_RISCV
        cs_mode = _capstone.CS_MODE_RISCV64
    # if _keystone:
    #     ks_arch = _keystone.KS_ARCH_RISCV
    #     ks_mode = _keystone.KS_MODE_LITTLE_ENDIAN
    uc_arch = _unicorn.UC_ARCH_RISCV if _unicorn else None
    uc_mode = _unicorn.UC_MODE_RISCV64 if _unicorn else None
    uc_const = riscv_const if _unicorn else None
    uc_prefix = "UC_RISCV_" if _unicorn else None

    ret_instruction = b"\x00\x00\x80\x67"  # ret
    nop_instruction = b"\x00\x00\x00\x13"  # nop
    function_prologs = set()
    function_epilogs = set()
    instruction_alignment = 4
    register_list = [
        Register(name="x0", size=8, alias_names=("zero",)),
        Register(
            name="x1",
            size=8,
            alias_names=(
                "ra",
                "lr",
            ),
            general_purpose=True,
        ),
        Register(name="x2", size=8, alias_names=("sp",), general_purpose=True),
        Register(name="x3", size=8, alias_names=("gp",), general_purpose=True),
        Register(name="x4", size=8, alias_names=("tp",), general_purpose=True),
        Register(name="x5", size=8, alias_names=("t0",), general_purpose=True),
        Register(name="x6", size=8, alias_names=("t1",), general_purpose=True),
        Register(name="x7", size=8, alias_names=("t2",), general_purpose=True),
        Register(name="x8", size=8, alias_names=("s0", "fp", "bp"), general_purpose=True),
        Register(name="x9", size=8, alias_names=("s1",), general_purpose=True),
        Register(name="x10", size=8, alias_names=("a0",), general_purpose=True, argument=True),
        Register(name="x11", size=8, alias_names=("a1",), general_purpose=True, argument=True),
        Register(name="x12", size=8, alias_names=("a2",), general_purpose=True, argument=True),
        Register(name="x13", size=8, alias_names=("a3",), general_purpose=True, argument=True),
        Register(name="x14", size=8, alias_names=("a4",), general_purpose=True, argument=True),
        Register(name="x15", size=8, alias_names=("a5",), general_purpose=True, argument=True),
        Register(name="x16", size=8, alias_names=("a6",), general_purpose=True, argument=True),
        Register(name="x17", size=8, alias_names=("a7",), general_purpose=True, argument=True),
        Register(name="x18", size=8, alias_names=("s2",), general_purpose=True),
        Register(name="x19", size=8, alias_names=("s3",), general_purpose=True),
        Register(name="x20", size=8, alias_names=("s4",), general_purpose=True),
        Register(name="x21", size=8, alias_names=("s5",), general_purpose=True),
        Register(name="x22", size=8, alias_names=("s6",), general_purpose=True),
        Register(name="x23", size=8, alias_names=("s7",), general_purpose=True),
        Register(name="x24", size=8, alias_names=("s8",), general_purpose=True),
        Register(name="x25", size=8, alias_names=("s9",), general_purpose=True),
        Register(name="x26", size=8, alias_names=("s10",), general_purpose=True),
        Register(name="x27", size=8, alias_names=("s11",), general_purpose=True),
        Register(name="x28", size=8, alias_names=("t3",), general_purpose=True),
        Register(name="x29", size=8, alias_names=("t4",), general_purpose=True),
        Register(name="x30", size=8, alias_names=("t5",), general_purpose=True),
        Register(name="x31", size=8, alias_names=("t6",), general_purpose=True),
        Register(name="pc", size=8, alias_names=("ip",)),
        Register(
            name="f0",
            size=8,
            alias_names=("ft0",),
            floating_point=True,
        ),
        Register(name="f1", size=8, alias_names=("ft1",), floating_point=True),
        Register(name="f2", size=8, alias_names=("ft2",), floating_point=True),
        Register(name="f3", size=8, alias_names=("ft3",), floating_point=True),
        Register(name="f4", size=8, alias_names=("ft4",), floating_point=True),
        Register(name="f5", size=8, alias_names=("ft5",), floating_point=True),
        Register(name="f6", size=8, alias_names=("ft6",), floating_point=True),
        Register(name="f7", size=8, alias_names=("ft7",), floating_point=True),
        Register(name="f8", size=8, alias_names=("fs0",), floating_point=True),
        Register(name="f9", size=8, alias_names=("fs1",), floating_point=True),
        Register(name="f10", size=8, alias_names=("fa0",), floating_point=True),
        Register(name="f11", size=8, alias_names=("fa1",), floating_point=True),
        Register(name="f12", size=8, alias_names=("fa2",), floating_point=True),
        Register(name="f13", size=8, alias_names=("fa3",), floating_point=True),
        Register(name="f14", size=8, alias_names=("fa4",), floating_point=True),
        Register(name="f15", size=8, alias_names=("fa5",), floating_point=True),
        Register(name="f16", size=8, alias_names=("fa6",), floating_point=True),
        Register(name="f17", size=8, alias_names=("fa7",), floating_point=True),
        Register(name="f18", size=8, alias_names=("fs2",), floating_point=True),
        Register(name="f19", size=8, alias_names=("fs3",), floating_point=True),
        Register(name="f20", size=8, alias_names=("fs4",), floating_point=True),
        Register(name="f21", size=8, alias_names=("fs5",), floating_point=True),
        Register(name="f22", size=8, alias_names=("fs6",), floating_point=True),
        Register(name="f23", size=8, alias_names=("fs7",), floating_point=True),
        Register(name="f24", size=8, alias_names=("fs8",), floating_point=True),
        Register(name="f25", size=8, alias_names=("fs9",), floating_point=True),
        Register(name="f26", size=8, alias_names=("fs10",), floating_point=True),
        Register(name="f27", size=8, alias_names=("fs11",), floating_point=True),
        Register(name="f28", size=8, alias_names=("ft8",), floating_point=True),
        Register(name="f29", size=8, alias_names=("ft9",), floating_point=True),
        Register(name="f30", size=8, alias_names=("ft10",), floating_point=True),
        Register(name="f31", size=8, alias_names=("ft11",), floating_point=True),
        Register(name="ip_at_syscall", size=8, artificial=True),
    ]

    got_section_name = ".got"
    ld_linux_name = "ld-linux-riscv64-lp64d.so.1"
    elf_tls = TLSArchInfo(1, 32, [], [0], [], 0, 0)


register_arch([r".*riscv.*"], 64, Endness.ANY, ArchRISCV64)
