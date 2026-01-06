try:
    import capstone as _capstone
except ImportError:
    _capstone = None

try:
    import keystone as _keystone
except ImportError:
    _keystone = None

try:
    import pyvex as _pyvex
except ImportError:
    _pyvex = None

from archinfo.types import RegisterOffset

from .arch import Arch, Endness, Register, register_arch
from .archerror import ArchError
from .tls import TLSArchInfo


class ArchS390X(Arch):
    def __init__(self, endness=Endness.BE):
        super().__init__(endness)
        if endness != Endness.BE:
            raise ArchError("Arch s390x must be big endian")
        self.argument_register_positions = (
            {
                self.registers["r2"][0]: 0,
                self.registers["r3"][0]: 1,
                self.registers["r4"][0]: 2,
                self.registers["r5"][0]: 3,
                self.registers["r6"][0]: 4,
                # fp registers
                self.registers["f0"][0]: 0,
                self.registers["f2"][0]: 1,
                self.registers["f4"][0]: 2,
                self.registers["f6"][0]: 3,
            }
            if _pyvex is not None
            else None
        )

    bits = 64
    vex_arch = "VexArchS390X"  # enum VexArch
    name = "S390X"
    default_endness = Endness.BE
    qemu_name = "s390x"  # target/s390x
    triplet = "s390x-linux-gnu"
    linux_name = "s390"  # arch/s390
    max_inst_bytes = 6
    ret_offset = RegisterOffset(584)  # offsetof(VexGuestS390XState, guest_r2)
    syscall_num_offset = 576  # offsetof(VexGuestS390XState, guest_r1)
    call_pushes_ret = False
    stack_change = -8
    initial_sp = 0x40000000000
    sizeof = {"short": 16, "int": 32, "long": 64, "long long": 64}
    if _capstone:
        cs_arch = _capstone.CS_ARCH_SYSZ
        cs_mode = _capstone.CS_MODE_BIG_ENDIAN
    if _keystone:
        ks_arch = _keystone.KS_ARCH_SYSTEMZ
        ks_mode = _keystone.KS_MODE_BIG_ENDIAN
    ret_instruction = b"\x07\xf4"  # br %r14
    nop_instruction = b"\x07\x07"  # nopr %r7
    instruction_alignment = 2
    register_list = [
        Register(name="ia", size=8, alias_names=("ip", "pc")),
        Register(name="r0", size=8, general_purpose=True),
        Register(name="r1", size=8, general_purpose=True, subregisters=[("r1_32", 4, 4)]),
        Register(name="r2", size=8, general_purpose=True, argument=True, subregisters=[("r2_32", 4, 4)]),
        Register(
            name="r3",
            size=8,
            general_purpose=True,
            argument=True,
            linux_entry_value="argc",
            subregisters=[("r3_32", 4, 4)],
        ),
        Register(
            name="r4",
            size=8,
            general_purpose=True,
            argument=True,
            linux_entry_value="argv",
            subregisters=[("r4_32", 4, 4)],
        ),
        Register(
            name="r5",
            size=8,
            general_purpose=True,
            argument=True,
            linux_entry_value="envp",
            subregisters=[("r5_32", 4, 4)],
        ),
        Register(
            name="r6", size=8, general_purpose=True, argument=True, persistent=True, subregisters=[("r6_32", 4, 4)]
        ),
        Register(name="r7", size=8, general_purpose=True, persistent=True, subregisters=[("r7_32", 4, 4)]),
        Register(name="r8", size=8, general_purpose=True, persistent=True, subregisters=[("r8_32", 4, 4)]),
        Register(name="r9", size=8, general_purpose=True, persistent=True, subregisters=[("r9_32", 4, 4)]),
        Register(name="r10", size=8, general_purpose=True, persistent=True, subregisters=[("r10_32", 4, 4)]),
        Register(
            name="r11",
            size=8,
            alias_names=("bp",),
            general_purpose=True,
            persistent=True,
            subregisters=[("r11_32", 4, 4)],
        ),
        Register(name="r12", size=8, general_purpose=True, persistent=True, subregisters=[("r12_32", 4, 4)]),
        Register(name="r13", size=8, general_purpose=True, persistent=True, subregisters=[("r13_32", 4, 4)]),
        # Strictly speaking, there is no fixed link register on s390x.
        # However, %r14 is almost always used for that, so mark it as such.
        # Situations when that's not the case (e.g. brasl %r0,X)
        # can still be handled explicitly.
        Register(name="r14", size=8, general_purpose=True, alias_names=("lr",)),
        Register(
            name="r15",
            size=8,
            alias_names=("sp",),
            general_purpose=True,
            persistent=True,
            default_value=(initial_sp, True, "global"),
        ),
        Register(name="v0", size=16, subregisters=[("f0", 0, 8)], floating_point=True),
        Register(name="v1", size=16, subregisters=[("f1", 0, 8)], floating_point=True),
        Register(name="v2", size=16, subregisters=[("f2", 0, 8)], floating_point=True),
        Register(name="v3", size=16, subregisters=[("f3", 0, 8)], floating_point=True),
        Register(name="v4", size=16, subregisters=[("f4", 0, 8)], floating_point=True),
        Register(name="v5", size=16, subregisters=[("f5", 0, 8)], floating_point=True),
        Register(name="v6", size=16, subregisters=[("f6", 0, 8)], floating_point=True),
        Register(name="v7", size=16, subregisters=[("f7", 0, 8)], floating_point=True),
        Register(name="v8", size=16, subregisters=[("f8", 0, 8)], floating_point=True),
        Register(name="v9", size=16, subregisters=[("f9", 0, 8)], floating_point=True),
        Register(name="v10", size=16, subregisters=[("f10", 0, 8)], floating_point=True),
        Register(name="v11", size=16, subregisters=[("f11", 0, 8)], floating_point=True),
        Register(name="v12", size=16, subregisters=[("f12", 0, 8)], floating_point=True),
        Register(name="v13", size=16, subregisters=[("f13", 0, 8)], floating_point=True),
        Register(name="v14", size=16, subregisters=[("f14", 0, 8)], floating_point=True),
        Register(name="v15", size=16, subregisters=[("f15", 0, 8)], floating_point=True),
        Register(name="v16", size=16, vector=True),
        Register(name="v17", size=16, vector=True),
        Register(name="v18", size=16, vector=True),
        Register(name="v19", size=16, vector=True),
        Register(name="v20", size=16, vector=True),
        Register(name="v21", size=16, vector=True),
        Register(name="v22", size=16, vector=True),
        Register(name="v23", size=16, vector=True),
        Register(name="v24", size=16, vector=True),
        Register(name="v25", size=16, vector=True),
        Register(name="v26", size=16, vector=True),
        Register(name="v27", size=16, vector=True),
        Register(name="v28", size=16, vector=True),
        Register(name="v29", size=16, vector=True),
        Register(name="v30", size=16, vector=True),
        Register(name="v31", size=16, vector=True),
        Register(name="a0", size=4),
        Register(name="a1", size=4),
        Register(name="a2", size=4),
        Register(name="a3", size=4),
        Register(name="a4", size=4),
        Register(name="a5", size=4),
        Register(name="a6", size=4),
        Register(name="a7", size=4),
        Register(name="a8", size=4),
        Register(name="a9", size=4),
        Register(name="a10", size=4),
        Register(name="a11", size=4),
        Register(name="a12", size=4),
        Register(name="a13", size=4),
        Register(name="a14", size=4),
        Register(name="a15", size=4),
        Register(name="nraddr", size=8),
        Register(name="cmstart", size=8),
        Register(name="cmlen", size=8),
        Register(name="ip_at_syscall", size=8, artificial=True),
        Register(name="emnote", size=4, artificial=True),
    ]

    function_prologs = {
        rb"\xeb.[\xf0-\xff]..\x24",  # stmg %r1,%r3,d2(%r15)
    }
    function_epilogs = {
        rb"\x07\xf4",  # br %r14
    }

    got_section_name = ".got"
    ld_linux_name = "ld64.so.1"
    elf_tls = TLSArchInfo(
        variant=2,  # 3.4.7 @ https://www.uclibc.org/docs/tls.pdf
        tcbhead_size=64,  # sizeof(tcbhead_t)
        head_offsets=[0],  # offsetof(tcbhead_t, tcb)
        dtv_offsets=[8],  # offsetof(tcbhead_t, dtv)
        pthread_offsets=[16],  # offsetof(tcbhead_t, self)
        tp_offset=0,
        dtv_entry_offset=0,
    )

    dwarf_registers = [
        "v0",
        "v1",
        "v2",
        "v3",
        "v4",
        "v5",
        "v6",
        "v7",
        "v8",
        "v9",
        "v10",
        "v11",
        "v12",
        "v13",
        "v14",
        "v15",
        "v16",
        "v17",
        "v18",
        "v19",
        "v20",
        "v21",
        "v22",
        "v23",
        "v24",
        "v25",
        "v26",
        "v27",
        "v28",
        "v29",
        "v30",
        "v31",
        "cr0",
        "cr1",
        "cr2",
        "cr3",
        "cr4",
        "cr5",
        "cr6",
        "cr7",
        "cr8",
        "cr9",
        "cr10",
        "cr11",
        "cr12",
        "cr13",
        "cr14",
        "cr15",
        "ar0",
        "ar1",
        "ar2",
        "ar3",
        "ar4",
        "ar5",
        "ar6",
        "ar7",
        "ar8",
        "ar9",
        "ar10",
        "ar11",
        "ar12",
        "ar13",
        "ar14",
        "ar15",
        "psw_mask",
        "psw_address",
    ]


register_arch(["s390"], 64, Endness.BE, ArchS390X)
