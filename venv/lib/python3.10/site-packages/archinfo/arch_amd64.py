from archinfo.types import RegisterOffset

from .arch import Arch, Endness, Register, register_arch
from .archerror import ArchError
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

try:
    import pyvex as _pyvex
except ImportError:
    _pyvex = None


_NATIVE_FUNCTION_PROLOGS = {
    rb"\x55\x48\x89\xe5",  # push rbp; mov rbp, rsp
    rb"\x48[\x83,\x81]\xec[\x00-\xff]",  # sub rsp, xxx
}
# every function prolog can potentially be prefixed with endbr64
_endbr64 = b"\xf3\x0f\x1e\xfa"
_prefixed = {(_endbr64 + prolog) for prolog in _NATIVE_FUNCTION_PROLOGS}
_FUNCTION_PROLOGS = _prefixed | _NATIVE_FUNCTION_PROLOGS


class ArchAMD64(Arch):
    def __init__(self, endness=Endness.LE):
        if endness != Endness.LE:
            raise ArchError("Arch AMD64 must be little endian")
        super().__init__(endness)
        self.argument_register_positions = (
            {
                self.registers["rdi"][0]: 0,
                self.registers["rsi"][0]: 1,
                self.registers["rdx"][0]: 2,
                self.registers["rcx"][0]: 3,  # Used for user calls
                self.registers["r10"][0]: 3,  # Used for Linux kernel calls
                self.registers["r8"][0]: 4,
                self.registers["r9"][0]: 5,
                # fp registers
                self.registers["xmm0"][0]: 0,
                self.registers["xmm1"][0]: 1,
                self.registers["xmm2"][0]: 2,
                self.registers["xmm3"][0]: 3,
                self.registers["xmm4"][0]: 4,
                self.registers["xmm5"][0]: 5,
                self.registers["xmm6"][0]: 6,
                self.registers["xmm7"][0]: 7,
            }
            if _pyvex is not None
            else None
        )

        # Register blacklist
        reg_blacklist = ("fs", "gs")
        if self.reg_blacklist is not None and self.reg_blacklist_offsets is not None:
            for register in self.register_list:
                if register.name in reg_blacklist:
                    self.reg_blacklist.append(register.name)
                    self.reg_blacklist_offsets.append(register.vex_offset)

        if _unicorn and _pyvex:
            # CPU flag registers
            uc_flags_reg = _unicorn.x86_const.UC_X86_REG_EFLAGS
            cpu_flag_registers = {"d": 1 << 10, "ac": 1 << 18, "id": 1 << 21}
            for reg, reg_bitmask in cpu_flag_registers.items():
                reg_offset = self.get_register_offset(reg)
                self.cpu_flag_register_offsets_and_bitmasks_map[reg_offset] = (uc_flags_reg, reg_bitmask)

            mxcsr_registers = {"sseround": 1 << 14 | 1 << 13}
            uc_mxcsr_reg = _unicorn.x86_const.UC_X86_REG_MXCSR
            for reg, reg_bitmask in mxcsr_registers.items():
                reg_offset = self.get_register_offset(reg)
                self.cpu_flag_register_offsets_and_bitmasks_map[reg_offset] = (uc_mxcsr_reg, reg_bitmask)

    @property
    def capstone_x86_syntax(self):
        """
        The current syntax Capstone uses for x64. It can be 'intel' or 'at&t'
        """
        return self._cs_x86_syntax

    @capstone_x86_syntax.setter
    def capstone_x86_syntax(self, new_syntax):
        if new_syntax not in ("intel", "at&t"):
            raise ArchError('Unsupported Capstone x86 syntax. It must be either "intel" or "at&t".')

        if new_syntax != self._cs_x86_syntax:
            self._cs = None
            self._cs_x86_syntax = new_syntax

    def _configure_capstone(self):
        if self._cs_x86_syntax == "at&t":
            self._cs.syntax = _capstone.CS_OPT_SYNTAX_ATT
        else:
            self._cs.syntax = _capstone.CS_OPT_SYNTAX_INTEL

    @property
    def keystone_x86_syntax(self):
        """
        The current syntax Keystone uses for x86. It can be 'intel',
        'at&t', 'nasm', 'masm', 'gas' or 'radix16'
        """
        return self._ks_x86_syntax

    @keystone_x86_syntax.setter
    def keystone_x86_syntax(self, new_syntax):
        if new_syntax not in ("intel", "at&t", "nasm", "masm", "gas", "radix16"):
            raise ArchError(
                "Unsupported Keystone x86 syntax. It must be one of the following: "
                '"intel", "at&t", "nasm", "masm", "gas" or "radix16".'
            )

        if new_syntax != self._ks_x86_syntax:
            self._ks = None
            self._ks_x86_syntax = new_syntax

    def _configure_keystone(self):
        if self._ks_x86_syntax == "at&t":
            self._ks.syntax = _keystone.KS_OPT_SYNTAX_ATT
        elif self._ks_x86_syntax == "nasm":
            self._ks.syntax = _keystone.KS_OPT_SYNTAX_NASM
        elif self._ks_x86_syntax == "masm":
            self._ks.syntax = _keystone.KS_OPT_SYNTAX_MASM
        elif self._ks_x86_syntax == "gas":
            self._ks.syntax = _keystone.KS_OPT_SYNTAX_GAS
        elif self._ks_x86_syntax == "radix16":
            self._ks.syntax = _keystone.KS_OPT_SYNTAX_RADIX16
        else:
            self._ks.syntax = _keystone.KS_OPT_SYNTAX_INTEL

    bits = 64
    vex_arch = "VexArchAMD64"
    vex_endness = "VexEndnessLE"
    name = "AMD64"
    qemu_name = "x86_64"
    ida_processor = "metapc"
    linux_name = "x86_64"
    triplet = "x86_64-linux-gnu"
    max_inst_bytes = 15
    ret_offset = RegisterOffset(16)
    vex_conditional_helpers = True
    syscall_num_offset = 16
    call_pushes_ret = True
    stack_change = -8
    initial_sp = 0x7FFFFFFFFFF0000
    call_sp_fix = -8
    memory_endness = Endness.LE
    register_endness = Endness.LE
    sizeof = {"short": 16, "int": 32, "long": 64, "long long": 64}
    if _capstone:
        cs_arch = _capstone.CS_ARCH_X86
        cs_mode = _capstone.CS_MODE_64 + _capstone.CS_MODE_LITTLE_ENDIAN
    _cs_x86_syntax = None  # Set it to 'att' in order to use AT&T syntax for x86
    if _keystone:
        ks_arch = _keystone.KS_ARCH_X86
        ks_mode = _keystone.KS_MODE_64 + _keystone.KS_MODE_LITTLE_ENDIAN
    _ks_x86_syntax = None
    uc_arch = _unicorn.UC_ARCH_X86 if _unicorn else None
    uc_mode = (_unicorn.UC_MODE_64 + _unicorn.UC_MODE_LITTLE_ENDIAN) if _unicorn else None
    uc_const = _unicorn.x86_const if _unicorn else None
    uc_prefix = "UC_X86_" if _unicorn else None
    function_prologs = _FUNCTION_PROLOGS
    function_epilogs = {
        rb"\xc9\xc3",  # leaveq; retq
        rb"([^\x41][\x50-\x5f]{1}|\x41[\x50-\x5f])\xc3",  # pop <reg>; retq
        rb"\x48[\x83,\x81]\xc4([\x00-\xff]{1}|[\x00-\xff]{4})\xc3",  #  add rsp, <siz>; retq
    }
    ret_instruction = b"\xc3"
    nop_instruction = b"\x90"
    instruction_alignment = 1
    register_list = [
        Register(
            name="rax",
            size=8,
            subregisters=[("eax", 0, 4), ("ax", 0, 2), ("al", 0, 1), ("ah", 1, 1)],
            general_purpose=True,
            linux_entry_value=0x1C,
        ),
        Register(
            name="rcx",
            size=8,
            subregisters=[("ecx", 0, 4), ("cx", 0, 2), ("cl", 0, 1), ("ch", 1, 1)],
            general_purpose=True,
            argument=True,
        ),
        Register(
            name="rdx",
            size=8,
            subregisters=[("edx", 0, 4), ("dx", 0, 2), ("dl", 0, 1), ("dh", 1, 1)],
            general_purpose=True,
            argument=True,
            linux_entry_value="ld_destructor",
        ),
        Register(
            name="rbx",
            size=8,
            subregisters=[("ebx", 0, 4), ("bx", 0, 2), ("bl", 0, 1), ("bh", 1, 1)],
            general_purpose=True,
            linux_entry_value=0,
        ),
        Register(
            name="rsp",
            size=8,
            subregisters=[("esp", 0, 4)],
            alias_names=("sp",),
            general_purpose=True,
            default_value=(initial_sp, True, "global"),
        ),
        Register(
            name="rbp",
            size=8,
            subregisters=[("ebp", 0, 4), ("_bp", 0, 2), ("bpl", 0, 1), ("bph", 1, 1)],
            alias_names=("bp",),
            general_purpose=True,
            linux_entry_value=0,
        ),
        Register(
            name="rsi",
            size=8,
            subregisters=[("esi", 0, 4), ("si", 0, 2), ("sil", 0, 1), ("sih", 1, 1)],
            general_purpose=True,
            argument=True,
            linux_entry_value="argv",
        ),
        Register(
            name="rdi",
            size=8,
            subregisters=[("edi", 0, 4), ("di", 0, 2), ("dil", 0, 1), ("dih", 1, 1)],
            general_purpose=True,
            argument=True,
            linux_entry_value="argc",
        ),
        Register(
            name="r8",
            size=8,
            subregisters=[("r8d", 0, 4), ("r8w", 0, 2), ("r8b", 0, 1)],
            general_purpose=True,
            argument=True,
        ),
        Register(
            name="r9",
            size=8,
            subregisters=[("r9d", 0, 4), ("r9w", 0, 2), ("r9b", 0, 1)],
            general_purpose=True,
            argument=True,
        ),
        Register(
            name="r10",
            size=8,
            subregisters=[("r10d", 0, 4), ("r10w", 0, 2), ("r10b", 0, 1)],
            general_purpose=True,
            argument=True,
        ),
        Register(
            name="r11", size=8, subregisters=[("r11d", 0, 4), ("r11w", 0, 2), ("r11b", 0, 1)], general_purpose=True
        ),
        Register(
            name="r12",
            size=8,
            subregisters=[("r12d", 0, 4), ("r12w", 0, 2), ("r12b", 0, 1)],
            general_purpose=True,
            linux_entry_value=0,
        ),
        Register(
            name="r13",
            size=8,
            subregisters=[("r13d", 0, 4), ("r13w", 0, 2), ("r13b", 0, 1)],
            general_purpose=True,
            linux_entry_value=0,
        ),
        Register(
            name="r14",
            size=8,
            subregisters=[("r14d", 0, 4), ("r14w", 0, 2), ("r14b", 0, 1)],
            general_purpose=True,
            linux_entry_value=0,
        ),
        Register(
            name="r15",
            size=8,
            subregisters=[("r15d", 0, 4), ("r15w", 0, 2), ("r15b", 0, 1)],
            general_purpose=True,
            linux_entry_value=0,
        ),
        Register(name="cc_op", size=8, default_value=(0, False, None), concrete=False, artificial=True),
        Register(name="cc_dep1", size=8, concrete=False, artificial=True),
        Register(name="cc_dep2", size=8, concrete=False, artificial=True),
        Register(name="cc_ndep", size=8, concrete=False, artificial=True, linux_entry_value=0),
        Register(name="d", size=8, alias_names=("dflag",), default_value=(1, False, None), concrete=False),
        Register(name="rip", size=8, alias_names=("ip", "pc"), general_purpose=True),
        Register(name="ac", size=8, alias_names=("acflag",), concrete=False),
        Register(name="id", size=8, alias_names=("idflag",)),
        Register(
            name="fs",
            size=8,
            vex_name="fs_const",
            alias_names=("fs_const",),
            default_value=(0x9000000000000000, True, "global"),
            concrete=False,
        ),
        Register(name="sseround", size=8, vector=True, default_value=(0, False, None)),
        Register(name="cr0", size=8),
        Register(name="cr2", size=8),
        Register(name="cr3", size=8),
        Register(name="cr4", size=8),
        Register(name="cr8", size=8),
        Register(
            name="ymm0",
            size=32,
            subregisters=[("xmm0", 0, 16), ("xmm0lq", 0, 8), ("xmm0hq", 8, 8), ("ymm0hx", 16, 16)],
            vector=True,
        ),
        Register(
            name="ymm1",
            size=32,
            subregisters=[("xmm1", 0, 16), ("xmm1lq", 0, 8), ("xmm1hq", 8, 8), ("ymm1hx", 16, 16)],
            vector=True,
        ),
        Register(
            name="ymm2",
            size=32,
            subregisters=[("xmm2", 0, 16), ("xmm2lq", 0, 8), ("xmm2hq", 8, 8), ("ymm2hx", 16, 16)],
            vector=True,
        ),
        Register(
            name="ymm3",
            size=32,
            subregisters=[("xmm3", 0, 16), ("xmm3lq", 0, 8), ("xmm3hq", 8, 8), ("ymm3hx", 16, 16)],
            vector=True,
        ),
        Register(
            name="ymm4",
            size=32,
            subregisters=[("xmm4", 0, 16), ("xmm4lq", 0, 8), ("xmm4hq", 8, 8), ("ymm4hx", 16, 16)],
            vector=True,
        ),
        Register(
            name="ymm5",
            size=32,
            subregisters=[("xmm5", 0, 16), ("xmm5lq", 0, 8), ("xmm5hq", 8, 8), ("ymm5hx", 16, 16)],
            vector=True,
        ),
        Register(
            name="ymm6",
            size=32,
            subregisters=[("xmm6", 0, 16), ("xmm6lq", 0, 8), ("xmm6hq", 8, 8), ("ymm6hx", 16, 16)],
            vector=True,
        ),
        Register(
            name="ymm7",
            size=32,
            subregisters=[("xmm7", 0, 16), ("xmm7lq", 0, 8), ("xmm7hq", 8, 8), ("ymm7hx", 16, 16)],
            vector=True,
        ),
        Register(
            name="ymm8",
            size=32,
            subregisters=[("xmm8", 0, 16), ("xmm8lq", 0, 8), ("xmm8hq", 8, 8), ("ymm8hx", 16, 16)],
            vector=True,
        ),
        Register(
            name="ymm9",
            size=32,
            subregisters=[("xmm9", 0, 16), ("xmm9lq", 0, 8), ("xmm9hq", 8, 8), ("ymm9hx", 16, 16)],
            vector=True,
        ),
        Register(
            name="ymm10",
            size=32,
            subregisters=[("xmm10", 0, 16), ("xmm10lq", 0, 8), ("xmm10hq", 8, 8), ("ymm10hx", 16, 16)],
            vector=True,
        ),
        Register(
            name="ymm11",
            size=32,
            subregisters=[("xmm11", 0, 16), ("xmm11lq", 0, 8), ("xmm11hq", 8, 8), ("ymm11hx", 16, 16)],
            vector=True,
        ),
        Register(
            name="ymm12",
            size=32,
            subregisters=[("xmm12", 0, 16), ("xmm12lq", 0, 8), ("xmm12hq", 8, 8), ("ymm12hx", 16, 16)],
            vector=True,
        ),
        Register(
            name="ymm13",
            size=32,
            subregisters=[("xmm13", 0, 16), ("xmm13lq", 0, 8), ("xmm13hq", 8, 8), ("ymm13hx", 16, 16)],
            vector=True,
        ),
        Register(
            name="ymm14",
            size=32,
            subregisters=[("xmm14", 0, 16), ("xmm14lq", 0, 8), ("xmm14hq", 8, 8), ("ymm14hx", 16, 16)],
            vector=True,
        ),
        Register(
            name="ymm15",
            size=32,
            subregisters=[("xmm15", 0, 16), ("xmm15lq", 0, 8), ("xmm15hq", 8, 8), ("ymm15hx", 16, 16)],
            vector=True,
        ),
        Register(name="ftop", size=4, floating_point=True, default_value=(7, False, None), artificial=True),
        Register(
            name="fpreg",
            size=64,
            subregisters=[
                ("mm0", 0, 8),
                ("mm1", 8, 8),
                ("mm2", 16, 8),
                ("mm3", 24, 8),
                ("mm4", 32, 8),
                ("mm5", 40, 8),
                ("mm6", 48, 8),
                ("mm7", 56, 8),
            ],
            alias_names=("fpu_regs",),
            floating_point=True,
        ),
        Register(name="fptag", size=8, alias_names=("fpu_tags",), floating_point=True, default_value=(0, False, None)),
        Register(name="fpround", size=8, floating_point=True, default_value=(0, False, None)),
        Register(name="fc3210", size=8, floating_point=True),
        Register(name="emnote", size=4, artificial=True),
        Register(name="cmstart", size=8),
        Register(name="cmlen", size=8),
        Register(name="nraddr", size=8),
        Register(name="gs", size=8, vex_name="gs_const", alias_names=("gs_const",), concrete=False),
        Register(name="ip_at_syscall", size=8, concrete=False, artificial=True),
        Register(name="cs_seg", size=2, vex_name="cs"),
        Register(name="ds_seg", size=2, vex_name="ds"),
        Register(name="es_seg", size=2, vex_name="es"),
        Register(name="fs_seg", size=2, vex_name="fs"),
        Register(name="gs_seg", size=2, vex_name="gs"),
        Register(name="ss_seg", size=2, vex_name="ss"),
    ]

    # https://gitlab.com/x86-psABIs/x86-64-ABI
    dynamic_tag_translation = {
        0x70000000: "DT_X86_64_PLT",
        0x70000001: "DT_X86_64_PLTSZ",
        0x70000003: "DT_X86_64_PLTENT",
    }

    symbol_type_translation = {10: "STT_GNU_IFUNC", "STT_LOOS": "STT_GNU_IFUNC"}
    got_section_name = ".got.plt"
    ld_linux_name = "ld-linux-x86-64.so.2"
    elf_tls = TLSArchInfo(2, 704, [16], [8], [0], 0, 0)
    dwarf_registers = [
        "rax",
        "rdx",
        "rcx",
        "rbx",
        "rsi",
        "rdi",
        "rbp",
        "rsp",
        "r8",
        "r9",
        "r10",
        "r11",
        "r12",
        "r13",
        "r14",
        "r15",
        "rip",
        "xmm0",
        "xmm1",
        "xmm2",
        "xmm3",
        "xmm4",
        "xmm5",
        "xmm6",
        "xmm7",
        "xmm8",
        "xmm9",
        "xmm10",
        "xmm11",
        "xmm12",
        "xmm13",
        "xmm14",
        "xmm15",
        "st0",
        "st1",
        "st2",
        "st3",
        "st4",
        "st5",
        "st6",
        "st7",
        "mm0",
        "mm1",
        "mm2",
        "mm3",
        "mm4",
        "mm5",
        "mm6",
        "mm7",
        "rflags",
        "es",
        "cs",
        "ss",
        "ds",
        "fs",
        "gs",
        "<none>",
        "<none>",
        "fs.base",
        "gs.base",
        "<none>",
        "<none>",
        "tr",
        "ldtr",
        "mxcsr",
        "fcw",
        "fsw",
    ]


register_arch([r".*amd64|.*x64|.*x86_64|.*metapc"], 64, Endness.LE, ArchAMD64)
