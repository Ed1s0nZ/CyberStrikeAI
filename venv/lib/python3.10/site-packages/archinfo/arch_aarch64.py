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
except ImportError:
    _unicorn = None


class ArchAArch64(Arch):
    def __init__(self, endness=Endness.LE):
        super().__init__(endness)
        if endness == Endness.BE:
            self.ida_processor = "armb"
            self.function_prologs = set()
            self.function_epilogs = set()

    bits = 64
    vex_arch = "VexArchARM64"
    name = "AARCH64"
    qemu_name = "aarch64"
    ida_processor = "arm"
    linux_name = "aarch64"
    triplet = "aarch64-linux-gnueabihf"
    max_inst_bytes = 4
    ret_offset = RegisterOffset(16)
    vex_conditional_helpers = True
    syscall_num_offset = 80
    call_pushes_ret = False
    stack_change = -8
    memory_endness = Endness.LE
    register_endness = Endness.LE
    instruction_endness = Endness.LE
    sizeof = {"short": 16, "int": 32, "long": 64, "long long": 64}
    if _capstone:
        cs_arch = _capstone.CS_ARCH_ARM64
        cs_mode = _capstone.CS_MODE_LITTLE_ENDIAN
    if _keystone:
        ks_arch = _keystone.KS_ARCH_ARM64
        ks_mode = _keystone.KS_MODE_LITTLE_ENDIAN
    uc_arch = _unicorn.UC_ARCH_ARM64 if _unicorn else None
    uc_mode = _unicorn.UC_MODE_LITTLE_ENDIAN if _unicorn else None
    uc_const = _unicorn.arm64_const if _unicorn else None
    uc_prefix = "UC_ARM64_" if _unicorn else None
    initial_sp = 0x7FFFFFFFFFF0000

    ret_instruction = b"\xc0\x03\x5f\xd6"  # ret
    nop_instruction = b"\x1f\x20\x03\xd5"  # nop
    instruction_alignment = 4
    register_list = [
        Register(
            name="x0",
            size=8,
            subregisters=[("w0", 0, 4)],
            alias_names=("r0",),
            general_purpose=True,
            argument=True,
            linux_entry_value="ld_destructor",
        ),
        Register(
            name="x1", size=8, subregisters=[("w1", 0, 4)], alias_names=("r1",), general_purpose=True, argument=True
        ),
        Register(
            name="x2", size=8, subregisters=[("w2", 0, 4)], alias_names=("r2",), general_purpose=True, argument=True
        ),
        Register(
            name="x3", size=8, subregisters=[("w3", 0, 4)], alias_names=("r3",), general_purpose=True, argument=True
        ),
        Register(
            name="x4", size=8, subregisters=[("w4", 0, 4)], alias_names=("r4",), general_purpose=True, argument=True
        ),
        Register(
            name="x5", size=8, subregisters=[("w5", 0, 4)], alias_names=("r5",), general_purpose=True, argument=True
        ),
        Register(
            name="x6", size=8, subregisters=[("w6", 0, 4)], alias_names=("r6",), general_purpose=True, argument=True
        ),
        Register(name="x7", size=8, subregisters=[("w7", 0, 4)], alias_names=("r7",), general_purpose=True),
        Register(name="x8", size=8, subregisters=[("w8", 0, 4)], alias_names=("r8",), general_purpose=True),
        Register(name="x9", size=8, subregisters=[("w9", 0, 4)], alias_names=("r9",), general_purpose=True),
        Register(name="x10", size=8, subregisters=[("w10", 0, 4)], alias_names=("r10",), general_purpose=True),
        Register(name="x11", size=8, subregisters=[("w11", 0, 4)], alias_names=("r11",), general_purpose=True),
        Register(name="x12", size=8, subregisters=[("w12", 0, 4)], alias_names=("r12",), general_purpose=True),
        Register(name="x13", size=8, subregisters=[("w13", 0, 4)], alias_names=("r13",), general_purpose=True),
        Register(name="x14", size=8, subregisters=[("w14", 0, 4)], alias_names=("r14",), general_purpose=True),
        Register(name="x15", size=8, subregisters=[("w15", 0, 4)], alias_names=("r15",), general_purpose=True),
        Register(name="x16", size=8, subregisters=[("w16", 0, 4)], alias_names=("r16", "ip0"), general_purpose=True),
        Register(name="x17", size=8, subregisters=[("w17", 0, 4)], alias_names=("r17", "ip1"), general_purpose=True),
        Register(name="x18", size=8, subregisters=[("w18", 0, 4)], alias_names=("r18",), general_purpose=True),
        Register(name="x19", size=8, subregisters=[("w19", 0, 4)], alias_names=("r19",), general_purpose=True),
        Register(name="x20", size=8, subregisters=[("w20", 0, 4)], alias_names=("r20",), general_purpose=True),
        Register(name="x21", size=8, subregisters=[("w21", 0, 4)], alias_names=("r21",), general_purpose=True),
        Register(name="x22", size=8, subregisters=[("w22", 0, 4)], alias_names=("r22",), general_purpose=True),
        Register(name="x23", size=8, subregisters=[("w23", 0, 4)], alias_names=("r23",), general_purpose=True),
        Register(name="x24", size=8, subregisters=[("w24", 0, 4)], alias_names=("r24",), general_purpose=True),
        Register(name="x25", size=8, subregisters=[("w25", 0, 4)], alias_names=("r25",), general_purpose=True),
        Register(name="x26", size=8, subregisters=[("w26", 0, 4)], alias_names=("r26",), general_purpose=True),
        Register(name="x27", size=8, subregisters=[("w27", 0, 4)], alias_names=("r27",), general_purpose=True),
        Register(name="x28", size=8, subregisters=[("w28", 0, 4)], alias_names=("r28",), general_purpose=True),
        Register(
            name="x29", size=8, subregisters=[("w29", 0, 4)], alias_names=("r29", "fp", "bp"), general_purpose=True
        ),
        Register(name="x30", size=8, subregisters=[("w30", 0, 4)], alias_names=("r30", "lr"), general_purpose=True),
        Register(
            name="xsp",
            size=8,
            subregisters=[("wsp", 0, 4)],
            alias_names=("sp",),
            general_purpose=True,
            default_value=(initial_sp, True, "global"),
        ),
        Register(name="pc", size=8, alias_names=("ip",)),
        Register(name="cc_op", size=8, artificial=True),
        Register(name="cc_dep1", size=8, artificial=True),
        Register(name="cc_dep2", size=8, artificial=True),
        Register(name="cc_ndep", size=8, artificial=True),
        Register(name="tpidr_el0", size=8),
        Register(
            name="q0",
            size=16,
            subregisters=[("d0", 0, 8), ("s0", 0, 4), ("h0", 0, 2), ("b0", 0, 1)],
            alias_names=("v0",),
            floating_point=True,
            vector=True,
        ),
        Register(
            name="q1",
            size=16,
            subregisters=[("d1", 0, 8), ("s1", 0, 4), ("h1", 0, 2), ("b1", 0, 1)],
            alias_names=("v1",),
            floating_point=True,
            vector=True,
        ),
        Register(
            name="q2",
            size=16,
            subregisters=[("d2", 0, 8), ("s2", 0, 4), ("h2", 0, 2), ("b2", 0, 1)],
            alias_names=("v2",),
            floating_point=True,
            vector=True,
        ),
        Register(
            name="q3",
            size=16,
            subregisters=[("d3", 0, 8), ("s3", 0, 4), ("h3", 0, 2), ("b3", 0, 1)],
            alias_names=("v3",),
            floating_point=True,
            vector=True,
        ),
        Register(
            name="q4",
            size=16,
            subregisters=[("d4", 0, 8), ("s4", 0, 4), ("h4", 0, 2), ("b4", 0, 1)],
            alias_names=("v4",),
            floating_point=True,
            vector=True,
        ),
        Register(
            name="q5",
            size=16,
            subregisters=[("d5", 0, 8), ("s5", 0, 4), ("h5", 0, 2), ("b5", 0, 1)],
            alias_names=("v5",),
            floating_point=True,
            vector=True,
        ),
        Register(
            name="q6",
            size=16,
            subregisters=[("d6", 0, 8), ("s6", 0, 4), ("h6", 0, 2), ("b6", 0, 1)],
            alias_names=("v6",),
            floating_point=True,
            vector=True,
        ),
        Register(
            name="q7",
            size=16,
            subregisters=[("d7", 0, 8), ("s7", 0, 4), ("h7", 0, 2), ("b7", 0, 1)],
            alias_names=("v7",),
            floating_point=True,
            vector=True,
        ),
        Register(
            name="q8",
            size=16,
            subregisters=[("d8", 0, 8), ("s8", 0, 4), ("h8", 0, 2), ("b8", 0, 1)],
            alias_names=("v8",),
            floating_point=True,
            vector=True,
        ),
        Register(
            name="q9",
            size=16,
            subregisters=[("d9", 0, 8), ("s9", 0, 4), ("h9", 0, 2), ("b9", 0, 1)],
            alias_names=("v9",),
            floating_point=True,
            vector=True,
        ),
        Register(
            name="q10",
            size=16,
            subregisters=[("d10", 0, 8), ("s10", 0, 4), ("h10", 0, 2), ("b10", 0, 1)],
            alias_names=("v10",),
            floating_point=True,
            vector=True,
        ),
        Register(
            name="q11",
            size=16,
            subregisters=[("d11", 0, 8), ("s11", 0, 4), ("h11", 0, 2), ("b11", 0, 1)],
            alias_names=("v11",),
            floating_point=True,
            vector=True,
        ),
        Register(
            name="q12",
            size=16,
            subregisters=[("d12", 0, 8), ("s12", 0, 4), ("h12", 0, 2), ("b12", 0, 1)],
            alias_names=("v12",),
            floating_point=True,
            vector=True,
        ),
        Register(
            name="q13",
            size=16,
            subregisters=[("d13", 0, 8), ("s13", 0, 4), ("h13", 0, 2), ("b13", 0, 1)],
            alias_names=("v13",),
            floating_point=True,
            vector=True,
        ),
        Register(
            name="q14",
            size=16,
            subregisters=[("d14", 0, 8), ("s14", 0, 4), ("h14", 0, 2), ("b14", 0, 1)],
            alias_names=("v14",),
            floating_point=True,
            vector=True,
        ),
        Register(
            name="q15",
            size=16,
            subregisters=[("d15", 0, 8), ("s15", 0, 4), ("h15", 0, 2), ("b15", 0, 1)],
            alias_names=("v15",),
            floating_point=True,
            vector=True,
        ),
        Register(
            name="q16",
            size=16,
            subregisters=[("d16", 0, 8), ("s16", 0, 4), ("h16", 0, 2), ("b16", 0, 1)],
            alias_names=("v16",),
            floating_point=True,
            vector=True,
        ),
        Register(
            name="q17",
            size=16,
            subregisters=[("d17", 0, 8), ("s17", 0, 4), ("h17", 0, 2), ("b17", 0, 1)],
            alias_names=("v17",),
            floating_point=True,
            vector=True,
        ),
        Register(
            name="q18",
            size=16,
            subregisters=[("d18", 0, 8), ("s18", 0, 4), ("h18", 0, 2), ("b18", 0, 1)],
            alias_names=("v18",),
            floating_point=True,
            vector=True,
        ),
        Register(
            name="q19",
            size=16,
            subregisters=[("d19", 0, 8), ("s19", 0, 4), ("h19", 0, 2), ("b19", 0, 1)],
            alias_names=("v19",),
            floating_point=True,
            vector=True,
        ),
        Register(
            name="q20",
            size=16,
            subregisters=[("d20", 0, 8), ("s20", 0, 4), ("h20", 0, 2), ("b20", 0, 1)],
            alias_names=("v20",),
            floating_point=True,
            vector=True,
        ),
        Register(
            name="q21",
            size=16,
            subregisters=[("d21", 0, 8), ("s21", 0, 4), ("h21", 0, 2), ("b21", 0, 1)],
            alias_names=("v21",),
            floating_point=True,
            vector=True,
        ),
        Register(
            name="q22",
            size=16,
            subregisters=[("d22", 0, 8), ("s22", 0, 4), ("h22", 0, 2), ("b22", 0, 1)],
            alias_names=("v22",),
            floating_point=True,
            vector=True,
        ),
        Register(
            name="q23",
            size=16,
            subregisters=[("d23", 0, 8), ("s23", 0, 4), ("h23", 0, 2), ("b23", 0, 1)],
            alias_names=("v23",),
            floating_point=True,
            vector=True,
        ),
        Register(
            name="q24",
            size=16,
            subregisters=[("d24", 0, 8), ("s24", 0, 4), ("h24", 0, 2), ("b24", 0, 1)],
            alias_names=("v24",),
            floating_point=True,
            vector=True,
        ),
        Register(
            name="q25",
            size=16,
            subregisters=[("d25", 0, 8), ("s25", 0, 4), ("h25", 0, 2), ("b25", 0, 1)],
            alias_names=("v25",),
            floating_point=True,
            vector=True,
        ),
        Register(
            name="q26",
            size=16,
            subregisters=[("d26", 0, 8), ("s26", 0, 4), ("h26", 0, 2), ("b26", 0, 1)],
            alias_names=("v26",),
            floating_point=True,
            vector=True,
        ),
        Register(
            name="q27",
            size=16,
            subregisters=[("d27", 0, 8), ("s27", 0, 4), ("h27", 0, 2), ("b27", 0, 1)],
            alias_names=("v27",),
            floating_point=True,
            vector=True,
        ),
        Register(
            name="q28",
            size=16,
            subregisters=[("d28", 0, 8), ("s28", 0, 4), ("h28", 0, 2), ("b28", 0, 1)],
            alias_names=("v28",),
            floating_point=True,
            vector=True,
        ),
        Register(
            name="q29",
            size=16,
            subregisters=[("d29", 0, 8), ("s29", 0, 4), ("h29", 0, 2), ("b29", 0, 1)],
            alias_names=("v29",),
            floating_point=True,
            vector=True,
        ),
        Register(
            name="q30",
            size=16,
            subregisters=[("d30", 0, 8), ("s30", 0, 4), ("h30", 0, 2), ("b30", 0, 1)],
            alias_names=("v30",),
            floating_point=True,
            vector=True,
        ),
        Register(
            name="q31",
            size=16,
            subregisters=[("d31", 0, 8), ("s31", 0, 4), ("h31", 0, 2), ("b31", 0, 1)],
            alias_names=("v31",),
            floating_point=True,
            vector=True,
        ),
        Register(name="qcflag", size=16, floating_point=True),
        Register(name="emnote", size=4, artificial=True),
        Register(name="cmstart", size=8),
        Register(name="cmlen", size=8),
        Register(name="nraddr", size=8),
        Register(name="ip_at_syscall", size=8, artificial=True),
        Register(name="fpcr", size=4, floating_point=True, default_value=(initial_sp, True, "global")),
    ]

    got_section_name = ".got"
    ld_linux_name = "ld-linux-aarch64.so.1"
    elf_tls = TLSArchInfo(1, 32, [], [0], [], 0, 0)
    dwarf_registers = [
        "x0",
        "x1",
        "x2",
        "x3",
        "x4",
        "x5",
        "x6",
        "x7",
        "x8",
        "x9",
        "x10",
        "x11",
        "x12",
        "x13",
        "x14",
        "x15",
        "x16",
        "x17",
        "x18",
        "x19",
        "x20",
        "x21",
        "x22",
        "x23",
        "x24",
        "x25",
        "x26",
        "x27",
        "x28",
        "x29",
        "x30",
        "sp",
        "<none>",
        "ELR_mode",
        "RA_SIGN_STATE",
        "<none>",
        "<none>",
        "<none>",
        "<none>",
        "<none>",
        "<none>",
        "<none>",
        "<none>",
        "<none>",
        "<none>",
        "<none>",
        "VG",
        "FFR",
        "p0",
        "p1",
        "p2",
        "p3",
        "p4",
        "p5",
        "p6",
        "p7",
        "p8",
        "p9",
        "p10",
        "p11",
        "p12",
        "p13",
        "p14",
        "p15",
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
        "z0",
        "z1",
        "z2",
        "z3",
        "z4",
        "z5",
        "z6",
        "z7",
        "z8",
        "z9",
        "z10",
        "z11",
        "z12",
        "z13",
        "z14",
        "z15",
        "z16",
        "z17",
        "z18",
        "z19",
        "z20",
        "z21",
        "z22",
        "z23",
        "z24",
        "z25",
        "z26",
        "z27",
        "z28",
        "z29",
        "z30",
        "z31",
    ]


register_arch([r".*arm64.*|.*aarch64*"], 64, Endness.ANY, ArchAArch64)
