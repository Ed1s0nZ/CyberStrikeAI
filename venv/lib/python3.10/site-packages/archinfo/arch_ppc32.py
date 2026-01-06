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
    import pyvex as _pyvex
except ImportError:
    _pyvex = None

# Note: PowerPC doesn't have pc, so guest_CIA is commented as IP (no arch visible register)
# PowerPC doesn't have stack base pointer, so bp_offset is set to -1 below
# Normally r1 is used as stack pointer


class ArchPPC32(Arch):
    def __init__(self, endness=Endness.LE):
        super().__init__(endness)
        if endness == Endness.BE:
            self.function_prologs = {
                # stwu r1, -off(r1); mflr r0
                rb"\x94\x21[\x00-\xff]{2}\x7c\x08\x02\xa6"
            }
            self.function_epilogs = {
                # mtlr reg; ... ; blr
                rb"[\x00-\xff]{2}\x03\xa6([\x00-\xff]{4}){0,6}\x4e\x80\x00\x20"
            }

        self.argument_register_positions = (
            {
                self.registers["r3"][0]: 0,
                self.registers["r4"][0]: 1,
                self.registers["r5"][0]: 2,
                self.registers["r6"][0]: 3,
                self.registers["r7"][0]: 4,
                self.registers["r8"][0]: 5,
                self.registers["r9"][0]: 6,
                self.registers["r10"][0]: 7,
            }
            if _pyvex is not None
            else None
        )

    bits = 32
    vex_arch = "VexArchPPC32"
    name = "PPC32"
    qemu_name = "ppc"
    ida_processor = "ppc"
    linux_name = "ppc750"  # ?
    triplet = "powerpc-linux-gnu"
    max_inst_bytes = 4
    # https://www.ibm.com/developerworks/community/forums/html/topic?id=77777777-0000-0000-0000-000013836863
    # claims that r15 is the base pointer but that is NOT what I see in practice
    ret_offset = RegisterOffset(28)
    syscall_num_offset = 16
    call_pushes_ret = False
    stack_change = -4
    sizeof = {"short": 16, "int": 32, "long": 32, "long long": 64}
    if _capstone:
        cs_arch = _capstone.CS_ARCH_PPC
        cs_mode = _capstone.CS_MODE_32 + _capstone.CS_MODE_LITTLE_ENDIAN
    if _keystone:
        ks_arch = _keystone.KS_ARCH_PPC
        ks_mode = _keystone.KS_MODE_32 + _keystone.KS_MODE_LITTLE_ENDIAN
    # Unicorn not supported
    # uc_arch = _unicorn.UC_ARCH_PPC if _unicorn else None
    # uc_mode = (_unicorn.UC_MODE_32 + _unicorn.UC_MODE_LITTLE_ENDIAN) if _unicorn else None
    ret_instruction = b"\x20\x00\x80\x4e"
    nop_instruction = b"\x00\x00\x00\x60"
    instruction_alignment = 4
    register_list = [
        Register(name="gpr0", size=4, alias_names=("r0",), general_purpose=True),
        Register(
            name="gpr1",
            size=4,
            alias_names=("r1", "sp"),
            general_purpose=True,
            default_value=(Arch.initial_sp, True, "global"),
        ),
        Register(name="gpr2", size=4, alias_names=("r2",), general_purpose=True),
        Register(
            name="gpr3", size=4, alias_names=("r3",), general_purpose=True, argument=True, linux_entry_value="argc"
        ),
        Register(
            name="gpr4", size=4, alias_names=("r4",), general_purpose=True, argument=True, linux_entry_value="argv"
        ),
        Register(
            name="gpr5", size=4, alias_names=("r5",), general_purpose=True, argument=True, linux_entry_value="envp"
        ),
        Register(
            name="gpr6", size=4, alias_names=("r6",), general_purpose=True, argument=True, linux_entry_value="auxv"
        ),
        Register(
            name="gpr7",
            size=4,
            alias_names=("r7",),
            general_purpose=True,
            argument=True,
            linux_entry_value="ld_destructor",
        ),
        Register(name="gpr8", size=4, alias_names=("r8",), general_purpose=True, argument=True),
        Register(name="gpr9", size=4, alias_names=("r9",), general_purpose=True, argument=True),
        Register(name="gpr10", size=4, alias_names=("r10",), general_purpose=True, argument=True),
        Register(name="gpr11", size=4, alias_names=("r11",), general_purpose=True),
        Register(name="gpr12", size=4, alias_names=("r12",), general_purpose=True),
        Register(name="gpr13", size=4, alias_names=("r13",), general_purpose=True),
        Register(name="gpr14", size=4, alias_names=("r14",), general_purpose=True),
        Register(name="gpr15", size=4, alias_names=("r15",), general_purpose=True),
        Register(name="gpr16", size=4, alias_names=("r16",), general_purpose=True),
        Register(name="gpr17", size=4, alias_names=("r17",), general_purpose=True),
        Register(name="gpr18", size=4, alias_names=("r18",), general_purpose=True),
        Register(name="gpr19", size=4, alias_names=("r19",), general_purpose=True),
        Register(name="gpr20", size=4, alias_names=("r20",), general_purpose=True),
        Register(name="gpr21", size=4, alias_names=("r21",), general_purpose=True),
        Register(name="gpr22", size=4, alias_names=("r22",), general_purpose=True),
        Register(name="gpr23", size=4, alias_names=("r23",), general_purpose=True),
        Register(name="gpr24", size=4, alias_names=("r24",), general_purpose=True),
        Register(name="gpr25", size=4, alias_names=("r25",), general_purpose=True, persistent=True),
        Register(name="gpr26", size=4, alias_names=("r26",), general_purpose=True),
        Register(name="gpr27", size=4, alias_names=("r27",), general_purpose=True),
        Register(name="gpr28", size=4, alias_names=("r28",), general_purpose=True),
        Register(name="gpr29", size=4, alias_names=("r29",), general_purpose=True),
        Register(name="gpr30", size=4, alias_names=("r30",), general_purpose=True),
        Register(name="gpr31", size=4, alias_names=("r31", "bp"), general_purpose=True),
        Register(name="vsr0", size=16, subregisters=[("fpr0", 0, 8)], alias_names=("v0",), floating_point=True),
        Register(name="vsr1", size=16, subregisters=[("fpr1", 0, 8)], alias_names=("v1",), floating_point=True),
        Register(name="vsr2", size=16, subregisters=[("fpr2", 0, 8)], alias_names=("v2",), floating_point=True),
        Register(name="vsr3", size=16, subregisters=[("fpr3", 0, 8)], alias_names=("v3",), floating_point=True),
        Register(name="vsr4", size=16, subregisters=[("fpr4", 0, 8)], alias_names=("v4",), floating_point=True),
        Register(name="vsr5", size=16, subregisters=[("fpr5", 0, 8)], alias_names=("v5",), floating_point=True),
        Register(name="vsr6", size=16, subregisters=[("fpr6", 0, 8)], alias_names=("v6",), floating_point=True),
        Register(name="vsr7", size=16, subregisters=[("fpr7", 0, 8)], alias_names=("v7",), floating_point=True),
        Register(name="vsr8", size=16, subregisters=[("fpr8", 0, 8)], alias_names=("v8",), floating_point=True),
        Register(name="vsr9", size=16, subregisters=[("fpr9", 0, 8)], alias_names=("v9",), floating_point=True),
        Register(name="vsr10", size=16, subregisters=[("fpr10", 0, 8)], alias_names=("v10",), floating_point=True),
        Register(name="vsr11", size=16, subregisters=[("fpr11", 0, 8)], alias_names=("v11",), floating_point=True),
        Register(name="vsr12", size=16, subregisters=[("fpr12", 0, 8)], alias_names=("v12",), floating_point=True),
        Register(name="vsr13", size=16, subregisters=[("fpr13", 0, 8)], alias_names=("v13",), floating_point=True),
        Register(name="vsr14", size=16, subregisters=[("fpr14", 0, 8)], alias_names=("v14",), floating_point=True),
        Register(name="vsr15", size=16, subregisters=[("fpr15", 0, 8)], alias_names=("v15",), floating_point=True),
        Register(name="vsr16", size=16, subregisters=[("fpr16", 0, 8)], alias_names=("v16",), floating_point=True),
        Register(name="vsr17", size=16, subregisters=[("fpr17", 0, 8)], alias_names=("v17",), floating_point=True),
        Register(name="vsr18", size=16, subregisters=[("fpr18", 0, 8)], alias_names=("v18",), floating_point=True),
        Register(name="vsr19", size=16, subregisters=[("fpr19", 0, 8)], alias_names=("v19",), floating_point=True),
        Register(name="vsr20", size=16, subregisters=[("fpr20", 0, 8)], alias_names=("v20",), floating_point=True),
        Register(name="vsr21", size=16, subregisters=[("fpr21", 0, 8)], alias_names=("v21",), floating_point=True),
        Register(name="vsr22", size=16, subregisters=[("fpr22", 0, 8)], alias_names=("v22",), floating_point=True),
        Register(name="vsr23", size=16, subregisters=[("fpr23", 0, 8)], alias_names=("v23",), floating_point=True),
        Register(name="vsr24", size=16, subregisters=[("fpr24", 0, 8)], alias_names=("v24",), floating_point=True),
        Register(name="vsr25", size=16, subregisters=[("fpr25", 0, 8)], alias_names=("v25",), floating_point=True),
        Register(name="vsr26", size=16, subregisters=[("fpr26", 0, 8)], alias_names=("v26",), floating_point=True),
        Register(name="vsr27", size=16, subregisters=[("fpr27", 0, 8)], alias_names=("v27",), floating_point=True),
        Register(name="vsr28", size=16, subregisters=[("fpr28", 0, 8)], alias_names=("v28",), floating_point=True),
        Register(name="vsr29", size=16, subregisters=[("fpr29", 0, 8)], alias_names=("v29",), floating_point=True),
        Register(name="vsr30", size=16, subregisters=[("fpr30", 0, 8)], alias_names=("v30",), floating_point=True),
        Register(name="vsr31", size=16, subregisters=[("fpr31", 0, 8)], alias_names=("v31",), floating_point=True),
        Register(name="vsr32", size=16, alias_names=("v32",), vector=True),
        Register(name="vsr33", size=16, alias_names=("v33",), vector=True),
        Register(name="vsr34", size=16, alias_names=("v34",), vector=True),
        Register(name="vsr35", size=16, alias_names=("v35",), vector=True),
        Register(name="vsr36", size=16, alias_names=("v36",), vector=True),
        Register(name="vsr37", size=16, alias_names=("v37",), vector=True),
        Register(name="vsr38", size=16, alias_names=("v38",), vector=True),
        Register(name="vsr39", size=16, alias_names=("v39",), vector=True),
        Register(name="vsr40", size=16, alias_names=("v40",), vector=True),
        Register(name="vsr41", size=16, alias_names=("v41",), vector=True),
        Register(name="vsr42", size=16, alias_names=("v42",), vector=True),
        Register(name="vsr43", size=16, alias_names=("v43",), vector=True),
        Register(name="vsr44", size=16, alias_names=("v44",), vector=True),
        Register(name="vsr45", size=16, alias_names=("v45",), vector=True),
        Register(name="vsr46", size=16, alias_names=("v46",), vector=True),
        Register(name="vsr47", size=16, alias_names=("v47",), vector=True),
        Register(name="vsr48", size=16, alias_names=("v48",), vector=True),
        Register(name="vsr49", size=16, alias_names=("v49",), vector=True),
        Register(name="vsr50", size=16, alias_names=("v50",), vector=True),
        Register(name="vsr51", size=16, alias_names=("v51",), vector=True),
        Register(name="vsr52", size=16, alias_names=("v52",), vector=True),
        Register(name="vsr53", size=16, alias_names=("v53",), vector=True),
        Register(name="vsr54", size=16, alias_names=("v54",), vector=True),
        Register(name="vsr55", size=16, alias_names=("v55",), vector=True),
        Register(name="vsr56", size=16, alias_names=("v56",), vector=True),
        Register(name="vsr57", size=16, alias_names=("v57",), vector=True),
        Register(name="vsr58", size=16, alias_names=("v58",), vector=True),
        Register(name="vsr59", size=16, alias_names=("v59",), vector=True),
        Register(name="vsr60", size=16, alias_names=("v60",), vector=True),
        Register(name="vsr61", size=16, alias_names=("v61",), vector=True),
        Register(name="vsr62", size=16, alias_names=("v62",), vector=True),
        Register(name="vsr63", size=16, alias_names=("v63",), vector=True),
        Register(name="cia", size=4, alias_names=("ip", "pc")),
        Register(name="lr", size=4),
        Register(name="ctr", size=4),
        Register(name="xer_so", size=1),
        Register(name="xer_ov", size=1),
        Register(name="xer_ca", size=1),
        Register(name="xer_bc", size=1),
        Register(name="cr0_321", size=1),
        Register(name="cr0_0", size=1, alias_names=("cr0",)),
        Register(name="cr1_321", size=1),
        Register(name="cr1_0", size=1, alias_names=("cr1",)),
        Register(name="cr2_321", size=1),
        Register(name="cr2_0", size=1, alias_names=("cr2",)),
        Register(name="cr3_321", size=1),
        Register(name="cr3_0", size=1, alias_names=("cr3",)),
        Register(name="cr4_321", size=1),
        Register(name="cr4_0", size=1, alias_names=("cr4",)),
        Register(name="cr5_321", size=1),
        Register(name="cr5_0", size=1, alias_names=("cr5",)),
        Register(name="cr6_321", size=1),
        Register(name="cr6_0", size=1, alias_names=("cr6",)),
        Register(name="cr7_321", size=1),
        Register(name="cr7_0", size=1, alias_names=("cr7",)),
        Register(name="fpround", size=1, floating_point=True),
        Register(name="dfpround", size=1, floating_point=True),
        Register(name="c_fpcc", size=1, floating_point=True),
        Register(name="vrsave", size=4, vector=True),
        Register(name="vscr", size=4, vector=True),
        Register(name="emnote", size=4, artificial=True),
        Register(name="cmstart", size=4),
        Register(name="cmlen", size=4),
        Register(name="nraddr", size=4),
        Register(name="nraddr_gpr2", size=4),
        Register(name="redir_sp", size=4),
        Register(name="redir_stack", size=128),
        Register(name="ip_at_syscall", size=4, artificial=True),
        Register(name="sprg3_ro", size=4),
        Register(name="tfhar", size=8),
        Register(name="texasr", size=8),
        Register(name="tfiar", size=8),
        Register(name="ppr", size=8),
        Register(name="texasru", size=4),
        Register(name="pspb", size=4),
    ]

    function_prologs = {
        rb"[\x00-\xff]{2}\x21\x94\xa6\x02\x08\x7c",  # stwu r1, -off(r1); mflr r0
    }
    function_epilogs = {rb"\xa6\x03[\x00-\xff]{2}([\x00-\xff]{4}){0,6}\x20\x00\x80\x4e"}  # mtlr reg; ... ; blr

    got_section_name = ".plt"
    ld_linux_name = "ld.so.1"
    elf_tls = TLSArchInfo(1, 52, [], [48], [], 0x7000, 0x8000)

    dwarf_registers = [
        "gpr0",
        "gpr1",
        "gpr2",
        "gpr3",
        "gpr4",
        "gpr5",
        "gpr6",
        "gpr7",
        "gpr8",
        "gpr9",
        "gpr10",
        "gpr11",
        "gpr12",
        "gpr13",
        "gpr14",
        "gpr15",
        "gpr16",
        "gpr17",
        "gpr18",
        "gpr19",
        "gpr20",
        "gpr21",
        "gpr22",
        "gpr23",
        "gpr24",
        "gpr25",
        "gpr26",
        "gpr27",
        "gpr28",
        "gpr29",
        "gpr30",
        "gpr31",
        "vsr0",
        "vsr1",
        "vsr2",
        "vsr3",
        "vsr4",
        "vsr5",
        "vsr6",
        "vsr7",
        "vsr8",
        "vsr9",
        "vsr10",
        "vsr11",
        "vsr12",
        "vsr13",
        "vsr14",
        "vsr15",
        "vsr16",
        "vsr17",
        "vsr18",
        "vsr19",
        "vsr20",
        "vsr21",
        "vsr22",
        "vsr23",
        "vsr24",
        "vsr25",
        "vsr26",
        "vsr27",
        "vsr28",
        "vsr29",
        "vsr30",
        "vsr31",
        "cr",
        "fpscr",
    ]


register_arch([r".*p\w*pc.*be"], 32, Endness.BE, ArchPPC32)
register_arch([r".*p\w*pc.*"], 32, Endness.ANY, ArchPPC32)
