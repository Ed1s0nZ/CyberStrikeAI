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
    rb"\x8b\xff\x55\x8b\xec",  # mov edi, edi; push ebp; mov ebp, esp
    rb"\x55\x8b\xec",  # push ebp; mov ebp, esp
    rb"\x55\x89\xe5",  # push ebp; mov ebp, esp
    rb"\x55\x57\x56",  # push ebp; push edi; push esi
    # mov eax, 0x000000??; (push ebp; push eax; push edi; push ebx; push esi; push edx; push ecx) sub esp
    rb"\xb8[\x00-\xff]\x00\x00\x00[\x50\x51\x52\x53\x55\x56\x57]{0,7}\x8b[\x00-\xff]{2}",
    # (push ebp; push eax; push edi; push ebx; push esi; push edx; push ecx) sub esp
    rb"[\x50\x51\x52\x53\x55\x56\x57]{1,7}\x83\xec[\x00-\xff]{2,4}",
    # (push ebp; push eax; push edi; push ebx; push esi; push edx; push ecx) mov xxx, xxx
    rb"[\x50\x51\x52\x53\x55\x56\x57]{1,7}\x8b[\x00-\xff]{2}",
    rb"(\x81|\x83)\xec",  # sub xxx %esp
}
# every function prolog can potentially be prefixed with endbr32
_endbr32 = b"\xf3\x0f\x1e\xfb"
_prefixed = {(_endbr32 + prolog) for prolog in _NATIVE_FUNCTION_PROLOGS}
_FUNCTION_PROLOGS = _prefixed | _NATIVE_FUNCTION_PROLOGS


class ArchX86(Arch):
    def __init__(self, endness=Endness.LE):
        if endness != Endness.LE:
            raise ArchError("Arch i386 must be little endian")
        super().__init__(endness)
        if self.vex_archinfo:
            self.vex_archinfo["x86_cr0"] = 0xFFFFFFFF

        # Register blacklist
        reg_blacklist = ("cs", "ds", "es", "fs", "gs", "ss", "gdt", "ldt")
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
        Get the current syntax Capstone uses for x86. It can be 'intel' or 'at&t'

        :return: Capstone's current x86 syntax
        :rtype: str
        """

        return self._cs_x86_syntax

    @capstone_x86_syntax.setter
    def capstone_x86_syntax(self, new_syntax):
        """
        Set the syntax that Capstone outputs for x86.
        """

        if new_syntax not in ("intel", "at&t"):
            raise ArchError('Unsupported Capstone x86 syntax. It must be either "intel" or "at&t".')

        if new_syntax != self._cs_x86_syntax:
            self._cs = None
            self._cs_x86_syntax = new_syntax

    def _configure_capstone(self):
        self._cs.syntax = (
            _capstone.CS_OPT_SYNTAX_ATT if self._cs_x86_syntax == "at&t" else _capstone.CS_OPT_SYNTAX_INTEL
        )

    @property
    def keystone_x86_syntax(self):
        """
        Get the current syntax Keystone uses for x86. It can be 'intel',
        'at&t', 'nasm', 'masm', 'gas' or 'radix16'

        :return: Keystone's current x86 syntax
        :rtype: str
        """

        return self._ks_x86_syntax

    @keystone_x86_syntax.setter
    def keystone_x86_syntax(self, new_syntax):
        """
        Set the syntax that Keystone uses for x86.
        """

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

    bits = 32
    vex_arch = "VexArchX86"
    name = "X86"
    qemu_name = "i386"
    ida_processor = "metapc"
    linux_name = "i386"
    triplet = "i386-linux-gnu"
    max_inst_bytes = 15
    call_sp_fix = -4
    ret_offset = RegisterOffset(8)
    vex_conditional_helpers = True
    syscall_num_offset = 8
    call_pushes_ret = True
    stack_change = -4
    memory_endness = Endness.LE
    register_endness = Endness.LE
    sizeof = {"short": 16, "int": 32, "long": 32, "long long": 64}
    if _capstone:
        cs_arch = _capstone.CS_ARCH_X86
        cs_mode = _capstone.CS_MODE_32 + _capstone.CS_MODE_LITTLE_ENDIAN
    _cs_x86_syntax = None  # Set it to 'att' in order to use AT&T syntax for x86
    if _keystone:
        ks_arch = _keystone.KS_ARCH_X86
        ks_mode = _keystone.KS_MODE_32 + _keystone.KS_MODE_LITTLE_ENDIAN
    _ks_x86_syntax = None
    uc_arch = _unicorn.UC_ARCH_X86 if _unicorn else None
    uc_mode = (_unicorn.UC_MODE_32 + _unicorn.UC_MODE_LITTLE_ENDIAN) if _unicorn else None
    uc_const = _unicorn.x86_const if _unicorn else None
    uc_prefix = "UC_X86_" if _unicorn else None
    function_prologs = _FUNCTION_PROLOGS
    function_epilogs = {
        rb"\xc9\xc3",  # leave; ret
        rb"([^\x41][\x50-\x5f]{1}|\x41[\x50-\x5f])\xc3",  # pop <reg>; ret
        rb"[^\x48][\x83,\x81]\xc4([\x00-\xff]{1}|[\x00-\xff]{4})\xc3",  #  add esp, <siz>; retq
    }
    ret_instruction = b"\xc3"
    nop_instruction = b"\x90"
    instruction_alignment = 1
    register_list = [
        Register(
            name="eax",
            size=4,
            subregisters=[("ax", 0, 2), ("al", 0, 1), ("ah", 1, 1)],
            general_purpose=True,
            argument=True,
            linux_entry_value=0x1C,
        ),
        Register(
            name="ecx",
            size=4,
            subregisters=[("cx", 0, 2), ("cl", 0, 1), ("ch", 1, 1)],
            general_purpose=True,
            argument=True,
        ),
        Register(
            name="edx",
            size=4,
            subregisters=[("dx", 0, 2), ("dl", 0, 1), ("dh", 1, 1)],
            general_purpose=True,
            argument=True,
            linux_entry_value="ld_destructor",
        ),
        Register(
            name="ebx",
            size=4,
            subregisters=[("bx", 0, 2), ("bl", 0, 1), ("bh", 1, 1)],
            general_purpose=True,
            argument=True,
        ),
        Register(
            name="esp",
            size=4,
            alias_names=("sp",),
            general_purpose=True,
            default_value=(Arch.initial_sp, True, "global"),
        ),
        Register(name="ebp", size=4, alias_names=("bp",), general_purpose=True, argument=True, linux_entry_value=0),
        Register(
            name="esi",
            size=4,
            subregisters=[("si", 0, 2), ("sil", 0, 1), ("sih", 1, 1)],
            general_purpose=True,
            argument=True,
        ),
        Register(
            name="edi",
            size=4,
            subregisters=[("di", 0, 2), ("dil", 0, 1), ("dih", 1, 1)],
            general_purpose=True,
            argument=True,
        ),
        Register(name="cc_op", size=4, default_value=(0, False, None), concrete=False, artificial=True),
        Register(name="cc_dep1", size=4, concrete=False, artificial=True),
        Register(name="cc_dep2", size=4, concrete=False, artificial=True),
        Register(name="cc_ndep", size=4, concrete=False, artificial=True),
        Register(name="d", size=4, alias_names=("dflag",), default_value=(1, False, None), concrete=False),
        Register(name="id", size=4, alias_names=("idflag",), default_value=(1, False, None), concrete=False),
        Register(name="ac", size=4, alias_names=("acflag",), default_value=(0, False, None), concrete=False),
        Register(name="eip", size=4, alias_names=("ip", "pc")),
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
            concrete=False,
        ),
        Register(name="fptag", size=8, alias_names=("fpu_tags",), floating_point=True, default_value=(0, False, None)),
        Register(name="fpround", size=4, floating_point=True, default_value=(0, False, None)),
        Register(name="fc3210", size=4, floating_point=True),
        Register(name="ftop", size=4, floating_point=True, default_value=(7, False, None), artificial=True),
        Register(name="sseround", size=4, vector=True, default_value=(0, False, None)),
        Register(name="xmm0", size=16, vector=True),
        Register(name="xmm1", size=16, vector=True),
        Register(name="xmm2", size=16, vector=True),
        Register(name="xmm3", size=16, vector=True),
        Register(name="xmm4", size=16, vector=True),
        Register(name="xmm5", size=16, vector=True),
        Register(name="xmm6", size=16, vector=True),
        Register(name="xmm7", size=16, vector=True),
        Register(name="cs", size=2),
        Register(name="ds", size=2),
        Register(name="es", size=2),
        Register(name="fs", size=2, default_value=(0, False, None), concrete=False),
        Register(name="gs", size=2, default_value=(0, False, None), concrete=False),
        Register(name="ss", size=2),
        Register(name="ldt", size=8, default_value=(0, False, None), concrete=False),
        Register(name="gdt", size=8, default_value=(0, False, None), concrete=False),
        Register(name="emnote", size=4, artificial=True),
        Register(name="cmstart", size=4),
        Register(name="cmlen", size=4),
        Register(name="nraddr", size=4),
        Register(name="sc_class", size=4),
        Register(name="ip_at_syscall", size=4, concrete=False, artificial=True),
    ]

    symbol_type_translation = {10: "STT_GNU_IFUNC", "STT_LOOS": "STT_GNU_IFUNC"}
    lib_paths = ["/lib32", "/usr/lib32"]
    got_section_name = ".got.plt"
    ld_linux_name = "ld-linux.so.2"
    elf_tls = TLSArchInfo(2, 56, [8], [4], [0], 0, 0)
    dwarf_registers = [
        "eax",
        "ecx",
        "edx",
        "ebx",
        "esp",
        "ebp",
        "esi",
        "edi",
        "eip",
        "eflags",
        "<none>",
        "st0",
        "st1",
        "st2",
        "st3",
        "st4",
        "st5",
        "st6",
        "st7",
        "<none>",
        "<none>",
        "xmm0",
        "xmm1",
        "xmm2",
        "xmm3",
        "xmm4",
        "xmm5",
        "xmm6",
        "xmm7",
        "mm0",
        "mm1",
        "mm2",
        "mm3",
        "mm4",
        "mm5",
        "mm6",
        "mm7",
        "fcw",
        "fsw",
        "mxcsr",
        "es",
        "cs",
        "ss",
        "ds",
        "fs",
        "gs",
        "<none>",
        "<none>",
        "tr",
        "ldtr",
    ]


register_arch([r".*i?\d86|.*x32|.*x86|.*metapc"], 32, Endness.LE, ArchX86)
