import logging

from archinfo.types import RegisterOffset

from .arch import Arch, Endness, Register, register_arch
from .tls import TLSArchInfo

log = logging.getLogger("archinfo.arch_arm")

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


# TODO: determine proper base register (if it exists)
# TODO: handle multiple return registers?
# TODO: which endianness should be default?


def is_arm_arch(a):
    return a.name.startswith("ARM")


def get_real_address_if_arm(arch, addr):
    """
    Obtain the real address of an instruction. ARM architectures are supported.

    :param archinfo.Arch arch:   The Arch object.
    :param int addr:    The instruction address.
    :return:            The real address of an instruction.
    :rtype:             int
    """

    return ((addr >> 1) << 1) if is_arm_arch(arch) else addr


class ArchARM(Arch):
    """
    ARM architecture specific subclass
    """

    def __init__(self, endness=Endness.LE):
        instruction_endness = None
        if endness == Endness.LE:
            instruction_endness = Endness.LE

        super().__init__(endness, instruction_endness=instruction_endness)
        if endness == Endness.BE:
            self.function_prologs = {
                # br"\xe9\x2d[\x40-\x7f\xc0-\xff][\x00-\xff]", # stmfd sp!, {xxxxx, lr}
                rb"\xe5\x2d\xe0\x04",  # push {lr}
                rb"\xe1\xa0\xc0\x0c\xe5\x2d\xe0\x04",
            }
            self.thumb_prologs = {
                # push.w {r4, r5, r7, r8, lr}
                rb"\xe9\x2d\x41\xb0",
                # push.w {r4-r7, r8, lr} | push.w {r4-r9, lr} | push.w {r4-r7, r9, r10, lr} | push.w {r4-r10, lr} |
                # push.w {r4-r8, r10, r11, lr} | push.w {r4-r11, lr}
                rb"\xe9\x2d[\x41\x43\x46\x47\x4d\x4f]\xf0",
                # push.w {r3-r9, lr} | push.w {r3-r7, r9, r10, lr} | push.w {r3-r11, lr}
                rb"\xe9\x2d[\x43\x46\x4f]\xf8",
                rb"[\xb4\xb5][\x00\x10\x30\x70\xf0]\xb0[\x80-\x8f\xa3\xa8]",
                # push {??, ??, ..., ??, lr}; sub sp, sp, #??
                rb"\xb4\x80\xb0[\x80-\xff]",  # push {r7}; sub sp, sp, #??
                rb"\xb4[\x00-\xff]\xb5\x00\xb0[\x80-\xff]",  # push {r?, r?}; push {lr}; sub sp, sp, #??
                rb"\xb0[\x80-\xff]\x90[\x00-\xff]",  # sub sp, sp, #??; str r0, [sp, ?]
                # stmt0: push {lr} | push {r3, lr} | push {r4, lr} | push {r4, r5, lr} | push {r3, r4, r5, lr} |
                #        push {r4, r5, r6, lr} | push {r4, r5, r6, r7, lr} | push {r3, r4, r5, r6, r7, lr}
                # stmt1: ldr r4, [pc, #??]
                # stmt2: add sp, r4
                rb"\xb5[\x00\x08\x10\x30\x38\x70\xf0\xf8]\x4c[\x00-\xff]\x44\xa5",
                # stmt0: push {lr} | push {r3, lr} | push {r4, lr} | push {r4, r5, lr} | push {r3, r4, r5, lr} |
                #        push {r4, r5, r6, lr} | push {r4, r5, r6, r7, lr} | push {r3, r4, r5, r6, r7, lr}
                # stmt1: mov r3/r4/r5/r6/r7, r0 | mov r4/r5/r6/r7, r1 | mov r6/r7, r3
                rb"\xb5[\x00\x08\x10\x30\x38\x70\xf0\xf8]\x46[\x03-\x07\x0c-\x0f\x1e-\x1f]",
                # stmt0: push {r3, lr}
                # stmt1: movs r2/r3, #0
                rb"\xb5\x08[\x22\x23]\x00",
                # ldr r3, [pc, #??]; ldr r2, [pc, #??]; add r3, pc; push {r4,r5,lr}
                rb"\x4b[\x00-\xff]\x4a[\x00-\xff]\x44\x7b\xb5\x30",
                # push {r3,r4,r5,lr}; mov r3, #0;
                rb"\xb5\x38\xf2\x40\x03\x00\xf2\xc0\x03\x00",
            }
            self.function_epilogs = {
                rb"\xe8\xbd[\x00-\xff]{2}\xe1\x2f\xff\x1e"  # pop {xxx}; bx lr
                rb"\xe4\x9d\xe0\x04\xe1\x2f\xff\x1e"  # pop {xxx}; bx lr
            }

    def __getstate__(self):
        self._cs = None
        self._cs_thumb = None
        self._ks = None
        self._ks_thumb = None
        return super().__getstate__()

    @property
    def capstone_thumb(self):
        if _capstone is None:
            log.warning("Capstone is not installed!")
            return None
        if self._cs_thumb is None:
            self._cs_thumb = _capstone.Cs(self.cs_arch, self.cs_mode + _capstone.CS_MODE_THUMB)
            self._cs_thumb.detail = True
        return self._cs_thumb

    @property
    def keystone_thumb(self):
        if _keystone is None:
            log.warning("Keystone is not installed!")
            return None
        if self._ks_thumb is None:
            self._ks_thumb = _keystone.Ks(self.ks_arch, _keystone.KS_MODE_THUMB)
        return self._ks_thumb

    @property
    def unicorn_thumb(self):
        if _unicorn is None:
            log.warning("Unicorn is not installed!")
            return None
        return _unicorn.Uc(self.uc_arch, self.uc_mode + _unicorn.UC_MODE_THUMB)

    def m_addr(self, addr, *args, **kwargs):
        """
        Given the address of some code block, convert it to the address where this block
        is stored in memory. The memory address can also be referred to as the "real" address.

        For ARM-architecture, the "real" address is always even (has its lowest bit clear).

        :param addr:    The address to convert.
        :return:        The "real" address in memory.
        :rtype:         int
        """
        return addr & ~1

    # pylint: disable=keyword-arg-before-vararg, arguments-differ
    def x_addr(self, addr, thumb=None, *args, **kwargs):
        """
        Given the address of some code block, convert it to the value that should be assigned
        to the instruction pointer register in order to execute the code in that block.

        :param addr:    The address to convert.
        :param thumb:   Set this parameter to True if you want to convert the address into the THUMB form.
                        Set this parameter to False if you want to convert the address into the ARM form.
                        Set this parameter to None (default) if you want to keep the address as is.
        :return:        The "execution" address.
        :rtype:         int
        """
        if thumb is None:
            return addr
        elif not thumb:
            return addr & ~1
        else:  # thumb
            return addr | 1

    def is_thumb(self, addr):  # pylint:disable=unused-argument
        """
        Return True, if the address is the THUMB address. False otherwise.

        :param addr:    The address to check.
        :return:        Whether the given address is the THUMB address.
        """
        return bool(addr & 1)

    bits = 32
    vex_arch = "VexArchARM"
    name = "ARMEL"
    qemu_name = "arm"
    ida_processor = "armb"
    linux_name = "arm"
    triplet = "arm-linux-gnueabihf"
    max_inst_bytes = 4
    ret_offset = RegisterOffset(8)
    fp_ret_offset = RegisterOffset(8)
    vex_conditional_helpers = True
    syscall_num_offset = 36
    call_pushes_ret = False
    stack_change = -4
    memory_endness = Endness.LE
    register_endness = Endness.LE
    sizeof = {"short": 16, "int": 32, "long": 32, "long long": 64}
    if _capstone:
        cs_arch = _capstone.CS_ARCH_ARM
        cs_mode = _capstone.CS_MODE_LITTLE_ENDIAN
    _cs_thumb = None
    if _keystone:
        ks_arch = _keystone.KS_ARCH_ARM
        ks_mode = _keystone.KS_MODE_ARM + _keystone.KS_MODE_LITTLE_ENDIAN
    _ks_thumb = None
    uc_arch = _unicorn.UC_ARCH_ARM if _unicorn else None
    uc_mode = _unicorn.UC_MODE_LITTLE_ENDIAN if _unicorn else None
    uc_mode_thumb = _unicorn.UC_MODE_LITTLE_ENDIAN + _unicorn.UC_MODE_THUMB if _unicorn else None
    uc_const = _unicorn.arm_const if _unicorn else None
    uc_prefix = "UC_ARM_" if _unicorn else None
    # self.ret_instruction = b"\x0E\xF0\xA0\xE1" # this is mov pc, lr
    ret_instruction = b"\x1e\xff\x2f\xe1"  # this is bx lr
    nop_instruction = b"\x00\x00\x00\x00"
    function_prologs = {
        # br"[\x00-\xff][\x40-\x7f\xc0-\xff]\x2d\xe9",       # stmfd sp!, {xxxxx,lr}
        rb"\x04\xe0\x2d\xe5",  # push {lr}
        rb"\r\xc0\xa0\xe1[\x00-\xff][\x40-\x7f\xc0-\xff]\x2d\xe9",  # mov r12, sp;  stmfd sp!, {xxxxx,lr}
        rb"\r\xc0\xa0\xe1\x04\xe0\x2d\xe5",  # mov r12, sp; push {lr}
    }
    thumb_prologs = {
        # push.w {r4, r5, r7, r8, lr}
        rb"\x2d\xe9\xb0\x41",
        # push.w {r4-r7, r8, lr} | push.w {r4-r9, lr} | push.w {r4-r7, r9, r10, lr} | push.w {r4-r10, lr} |
        # push.w {r4-r8, r10, r11, lr} | push.w {r4-r11, lr}
        rb"\x2d\xe9\xf0[\x41\x43\x46\x47\x4d\x4f]",
        # push.w {r3-r9, lr} | push.w {r3-r7, r9, r10, lr} | push.w {r3-r11, lr}
        rb"\x2d\xe9\xf8[\x43\x46\x4f]",
        rb"[\x00\x10\x30\x70\xf0][\xb4\xb5][\x80-\x8f\xa3\xa8]\xb0",  # push {??, ??, ..., ??, lr}; sub sp, sp, #??
        rb"\x80\xb4[\x80-\xff]\xb0",  # push {r7}; sub sp, sp, #??
        rb"[\x00-\xff]\xb4\x00\xb5[\x80-\xff]\xb0",  # push {r?, r?}; push {lr}; sub sp, sp, #??
        rb"[\x80-\xff]\xb0[\x00-\xff]\x90",  # sub sp, sp, #??; str r0, [sp, ?]
        # stmt0: push {lr} | push {r3, lr} | push {r4, lr} | push {r4, r5, lr} | push {r3, r4, r5, lr} |
        #        push {r4, r5, r6, lr} | push {r4, r5, r6, r7, lr} | push {r3, r4, r5, r6, r7, lr}
        # stmt1: ldr r4, [pc, #??]
        # stmt2: add sp, r4
        rb"[\x00\x08\x10\x30\x38\x70\xf0\xf8]\xb5[\x00-\xff]\x4c\xa5\x44",
        # stmt0: push {lr} | push {r3, lr} | push {r4, lr} | push {r4, r5, lr} | push {r3, r4, r5, lr} |
        #        push {r4, r5, r6, lr} | push {r4, r5, r6, r7, lr} | push {r3, r4, r5, r6, r7, lr}
        # stmt1: mov r3/r4/r5/r6/r7, r0 | mov r4/r5/r6/r7, r1 | mov r6/r7, r3
        rb"[\x00\x08\x10\x30\x38\x70\xf0\xf8]\xb5[\x03-\x07\x0c-\x0f\x1e-\x1f]\x46",
        # stmt0: push {r3, lr}
        # stmt1: movs r2/r3, #0
        rb"\x08\xb5\x00[\x22\x23]",
        # ldr r3, [pc, #??]; ldr r2, [pc, #??]; add r3, pc; push {r4,r5,lr}
        rb"[\x00-\xff]\x4b[\x00-\xff]\x4a\x7b\x44\x30\xb5",
        # push {r3,r4,r5,lr}; mov r3, #0;
        rb"\x38\xb5\x40\xf2\x00\x03\xc0\xf2\x00\x03",
    }
    function_epilogs = {
        rb"[\x00-\xff]{2}\xbd\xe8\x1e\xff\x2f\xe1"  # pop {xxx}; bx lr
        rb"\x04\xe0\x9d\xe4\x1e\xff\x2f\xe1"  # pop {xxx}; bx lr
    }
    instruction_alignment = 2  # cuz there is also thumb mode
    register_list = [
        Register(
            name="r0",
            size=4,
            alias_names=("a1",),
            general_purpose=True,
            argument=True,
            linux_entry_value="ld_destructor",
        ),
        Register(name="r1", size=4, alias_names=("a2",), general_purpose=True, argument=True),
        Register(name="r2", size=4, alias_names=("a3",), general_purpose=True, argument=True),
        Register(name="r3", size=4, alias_names=("a4",), general_purpose=True, argument=True),
        Register(name="r4", size=4, alias_names=("v1",), general_purpose=True),
        Register(name="r5", size=4, alias_names=("v2",), general_purpose=True),
        Register(name="r6", size=4, alias_names=("v3",), general_purpose=True),
        Register(name="r7", size=4, alias_names=("v4",), general_purpose=True),
        Register(name="r8", size=4, alias_names=("v5",), general_purpose=True),
        Register(name="r9", size=4, alias_names=("v6", "sb"), general_purpose=True),
        Register(name="r10", size=4, alias_names=("v7", "sl"), general_purpose=True),
        Register(name="r11", size=4, alias_names=("v8", "fp", "bp"), general_purpose=True),
        Register(name="r12", size=4, general_purpose=True),
        # r12 is sometimes known as "ip" (intraprocedural call scratch) but we can't have that...
        Register(
            name="sp",
            size=4,
            alias_names=("r13",),
            general_purpose=True,
            default_value=(Arch.initial_sp, True, "global"),
        ),
        Register(name="lr", size=4, alias_names=("r14",), general_purpose=True, concretize_unique=True),
        Register(name="pc", size=4, vex_name="r15t", alias_names=("r15", "ip")),
        Register(name="cc_op", size=4, default_value=(0, False, None), artificial=True, concrete=False),
        Register(name="cc_dep1", size=4, default_value=(0, False, None), artificial=True, concrete=False),
        Register(name="cc_dep2", size=4, default_value=(0, False, None), artificial=True, concrete=False),
        Register(name="cc_ndep", size=4, default_value=(0, False, None), artificial=True, concrete=False),
        Register(name="qflag32", size=4, default_value=(0, False, None), artificial=True, concrete=False),
        Register(name="geflag0", size=4, vector=True, default_value=(0, False, None), artificial=True, concrete=False),
        Register(name="geflag1", size=4, vector=True, default_value=(0, False, None), artificial=True, concrete=False),
        Register(name="geflag2", size=4, vector=True, default_value=(0, False, None), artificial=True, concrete=False),
        Register(name="geflag3", size=4, vector=True, default_value=(0, False, None), artificial=True, concrete=False),
        Register(name="emnote", size=4, vector=True, default_value=(0, False, None), artificial=True, concrete=False),
        Register(name="cmstart", size=4, artificial=True, vector=True, default_value=(0, False, None), concrete=False),
        Register(name="cmlen", size=4, artificial=True, default_value=(0, False, None), concrete=False),
        Register(name="nraddr", size=4, artificial=True, default_value=(0, False, None), concrete=False),
        Register(name="ip_at_syscall", size=4, artificial=True, concrete=False),
        Register(name="d0", size=8, floating_point=True, vector=True, subregisters=[("s0", 0, 4), ("s1", 4, 4)]),
        Register(name="d1", size=8, floating_point=True, vector=True, subregisters=[("s2", 0, 4), ("s3", 4, 4)]),
        Register(name="d2", size=8, floating_point=True, vector=True, subregisters=[("s4", 0, 4), ("s5", 4, 4)]),
        Register(name="d3", size=8, floating_point=True, vector=True, subregisters=[("s6", 0, 4), ("s7", 4, 4)]),
        Register(name="d4", size=8, floating_point=True, vector=True, subregisters=[("s8", 0, 4), ("s9", 4, 4)]),
        Register(name="d5", size=8, floating_point=True, vector=True, subregisters=[("s10", 0, 4), ("s11", 4, 4)]),
        Register(name="d6", size=8, floating_point=True, vector=True, subregisters=[("s12", 0, 4), ("s13", 4, 4)]),
        Register(name="d7", size=8, floating_point=True, vector=True, subregisters=[("s14", 0, 4), ("s15", 4, 4)]),
        Register(name="d8", size=8, floating_point=True, vector=True, subregisters=[("s16", 0, 4), ("s17", 4, 4)]),
        Register(name="d9", size=8, floating_point=True, vector=True, subregisters=[("s18", 0, 4), ("s19", 4, 4)]),
        Register(name="d10", size=8, floating_point=True, vector=True, subregisters=[("s20", 0, 4), ("s21", 4, 4)]),
        Register(name="d11", size=8, floating_point=True, vector=True, subregisters=[("s22", 0, 4), ("s23", 4, 4)]),
        Register(name="d12", size=8, floating_point=True, vector=True, subregisters=[("s24", 0, 4), ("s25", 4, 4)]),
        Register(name="d13", size=8, floating_point=True, vector=True, subregisters=[("s26", 0, 4), ("s27", 4, 4)]),
        Register(name="d14", size=8, floating_point=True, vector=True, subregisters=[("s28", 0, 4), ("s29", 4, 4)]),
        Register(name="d15", size=8, floating_point=True, vector=True, subregisters=[("s30", 0, 4), ("s31", 4, 4)]),
        Register(name="d16", size=8, floating_point=True, vector=True),
        Register(name="d17", size=8, floating_point=True, vector=True),
        Register(name="d18", size=8, floating_point=True, vector=True),
        Register(name="d19", size=8, floating_point=True, vector=True),
        Register(name="d20", size=8, floating_point=True, vector=True),
        Register(name="d21", size=8, floating_point=True, vector=True),
        Register(name="d22", size=8, floating_point=True, vector=True),
        Register(name="d23", size=8, floating_point=True, vector=True),
        Register(name="d24", size=8, floating_point=True, vector=True),
        Register(name="d25", size=8, floating_point=True, vector=True),
        Register(name="d26", size=8, floating_point=True, vector=True),
        Register(name="d27", size=8, floating_point=True, vector=True),
        Register(name="d28", size=8, floating_point=True, vector=True),
        Register(name="d29", size=8, floating_point=True, vector=True),
        Register(name="d30", size=8, floating_point=True, vector=True),
        Register(name="d31", size=8, floating_point=True, vector=True),
        Register(name="fpscr", size=4, floating_point=True, artificial=True, concrete=False),
        Register(name="tpidruro", size=4, artificial=True, concrete=False),
        Register(name="itstate", size=4, artificial=True, default_value=(0, False, None), concrete=False),
    ]

    got_section_name = ".got"
    ld_linux_name = "ld-linux.so.3"
    elf_tls = TLSArchInfo(1, 8, [], [0], [], 0, 0)
    # elf_tls = TLSArchInfo(1, 32, [], [0], [], 0, 0)
    # that line was lying in the original CLE code and I have no clue why it's different


class ArchARMHF(ArchARM):
    """
    This is an architecture description for the ARM hard-float (armhf).
    It supports at least an ARM 32-bit processor with ARMv7 architecture, Thumb-2 and VFP3D16.
    """

    name = "ARMHF"
    triplet = "arm-linux-gnueabihf"
    ld_linux_name = "ld-linux-armhf.so.3"
    fp_ret_offset = RegisterOffset(128)  # s0


class ArchARMEL(ArchARM):
    """
    This is an architecture description for ARM EABI (armel).
    It targets a range of older 32-bit ARM devices without hardware FPUs.
    """

    name = "ARMEL"
    triplet = "arm-linux-gnueabi"
    ld_linux_name = "ld-linux.so.3"
    elf_tls = TLSArchInfo(1, 8, [], [0], [], 0, 0)


class ArchARMCortexM(ArchARMEL):
    """
    This is an architecture description for ARM Cortex-M microcontroller-class CPUs.

    These CPUs have the following unusual / annoying distinctions from their relatives:
    - Explicitly only support the Thumb-2 instruction set.  Executing with the T-bit off causes the processor to fault
    instantly
    - Always little-endian
    - Coprocessors? Nope, none of that rubbish
    - Well-known standard memory map across all devices
    - Rarely use an MPU, even though this does exist on some devices
    - A built-in "NVIC" (Nested Vectored Interrupt Controller) as part of the standard.
    - Standardized "blob format" including the IVT, with initial SP and entry prepended
    - Usually don't run an OS (SimLinux? No thanks)
    - As part of the above, handle syscalls (SVC) instructions through an interrupt (now called PendSV)
    Uses its own fancy stack layout for this, which (UGH) varies by sub-sub-architecture
    - Some fancy instructions normally never seen in other uses of Thumb (CPSID, CPSIE, WFI, MRS.W, MSR.W)
    - New registers, namely:
    * FAULTMASK
    * PRIMASK
    * BASEPRI
    * CONTROL
    * SP, banked as PSP or MSP
    * PSR, now just one PSR, with a few meta-registers APSR, IPSR, and EPSR which take a chunk of that each

    """

    name = "ARMCortexM"
    triplet = "arm-none-eabi"  # ARM's own CM compilers use this triplet

    # These are the standard THUMB prologs.  We leave these off for other ARMs due to their length
    # For CM, we assume the FPs are OK, as they are almost guaranteed to appear all over the place
    function_prologs = set()

    thumb_prologs = {rb"[\x00-\xff]\xb5", rb"\x2d\xe9[\x00-\xff][\x00-\xff]"}  # push {xxx,lr}  # push.w {xxx, lr}
    function_epilogs = {
        rb"[\x00-\xff]\xbd"  # pop {xxx, pc}
        # TODO: POP.W
    }

    register_list = [
        Register(
            name="r0",
            size=4,
            alias_names=("a1",),
            general_purpose=True,
            argument=True,
            linux_entry_value="ld_destructor",
        ),
        Register(name="r1", size=4, alias_names=("a2",), general_purpose=True, argument=True),
        Register(name="r2", size=4, alias_names=("a3",), general_purpose=True, argument=True),
        Register(name="r3", size=4, alias_names=("a4",), general_purpose=True, argument=True),
        Register(name="r4", size=4, alias_names=("v1",), general_purpose=True),
        Register(name="r5", size=4, alias_names=("v2",), general_purpose=True),
        Register(name="r6", size=4, alias_names=("v3",), general_purpose=True),
        Register(name="r7", size=4, alias_names=("v4",), general_purpose=True),
        Register(name="r8", size=4, alias_names=("v5",), general_purpose=True),
        Register(name="r9", size=4, alias_names=("v6", "sb"), general_purpose=True),
        Register(name="r10", size=4, alias_names=("v7", "sl"), general_purpose=True),
        Register(name="r11", size=4, alias_names=("v8", "fp", "bp"), general_purpose=True),
        # r11 is used as frame pointer
        Register(name="r12", size=4, general_purpose=True),
        # r12 is sometimes known as "ip" (intraprocedural call scratch) but we can't have that...
        Register(
            name="sp",
            size=4,
            alias_names=("r13",),
            general_purpose=True,
            default_value=(Arch.initial_sp, True, "global"),
        ),
        Register(name="lr", size=4, alias_names=("r14",), general_purpose=True, concretize_unique=True),
        Register(name="pc", size=4, vex_name="r15t", alias_names=("r15", "ip")),
        # Control registers for Cortex-M33 with ArmV8 securtiy extension enabled (Trustzone)
        Register(name="msp", size=4, general_purpose=True),
        Register(name="msp_s", size=4, general_purpose=True),
        Register(name="msp_ns", size=4, general_purpose=True),
        Register(name="psp", size=4, general_purpose=True),
        Register(name="psp_s", size=4, general_purpose=True),
        Register(name="psp_ns", size=4, general_purpose=True),
        Register(name="msplim", size=4, general_purpose=True),
        Register(name="msplim_s", size=4, general_purpose=True),
        Register(name="msplim_ns", size=4, general_purpose=True),
        Register(name="msplim_ns", size=4, general_purpose=True),
        # additional stack pointers for secure/non_secure world
        Register(name="sp_process", size=4, general_purpose=True),
        Register(name="sp_process_s", size=4, general_purpose=True),
        Register(name="sp_process_ns", size=4, general_purpose=True),
        Register(name="sp_main", size=4, general_purpose=True),
        Register(name="sp_main_s", size=4, general_purpose=True),
        Register(name="sp_main_ns", size=4, general_purpose=True),
        Register(name="cc_op", size=4, default_value=(0, False, None), artificial=True, concrete=False),
        Register(name="cc_dep1", size=4, default_value=(0, False, None), artificial=True, concrete=False),
        Register(name="cc_dep2", size=4, default_value=(0, False, None), artificial=True, concrete=False),
        Register(name="cc_ndep", size=4, default_value=(0, False, None), artificial=True, concrete=False),
        Register(name="qflag32", size=4, default_value=(0, False, None), artificial=True, concrete=False),
        Register(name="ip_at_syscall", size=4, artificial=True, concrete=False),
        # Cortex-M Has a different FPU from all other ARMs.
        Register(name="d0", size=8, subregisters=[("s0", 0, 4), ("s1", 4, 4)], floating_point=True),
        Register(name="d1", size=8, subregisters=[("s2", 0, 4), ("s3", 4, 4)], floating_point=True),
        Register(name="d2", size=8, subregisters=[("s4", 0, 4), ("s5", 4, 4)], floating_point=True),
        Register(name="d3", size=8, subregisters=[("s6", 0, 4), ("s7", 4, 4)], floating_point=True),
        Register(name="d4", size=8, subregisters=[("s8", 0, 4), ("s9", 4, 4)], floating_point=True),
        Register(name="d5", size=8, subregisters=[("s10", 0, 4), ("s11", 4, 4)], floating_point=True),
        Register(name="d6", size=8, subregisters=[("s12", 0, 4), ("s13", 4, 4)], floating_point=True),
        Register(name="d7", size=8, subregisters=[("s14", 0, 4), ("s15", 4, 4)], floating_point=True),
        Register(name="d8", size=8, subregisters=[("s16", 0, 4), ("s17", 4, 4)], floating_point=True),
        Register(name="d9", size=8, subregisters=[("s18", 0, 4), ("s19", 4, 4)], floating_point=True),
        Register(name="d10", size=8, subregisters=[("s20", 0, 4), ("s21", 4, 4)], floating_point=True),
        Register(name="d11", size=8, subregisters=[("s22", 0, 4), ("s23", 4, 4)], floating_point=True),
        Register(name="d12", size=8, subregisters=[("s24", 0, 4), ("s25", 4, 4)], floating_point=True),
        Register(name="d13", size=8, subregisters=[("s26", 0, 4), ("s27", 4, 4)], floating_point=True),
        Register(name="d14", size=8, subregisters=[("s28", 0, 4), ("s29", 4, 4)], floating_point=True),
        Register(name="d15", size=8, subregisters=[("s30", 0, 4), ("s31", 4, 4)], floating_point=True),
        # xPSR register. Includes APSR, IPSR and EPSR.
        Register(name="cpsr", size=4, default_value=(0, False, None)),
        # TODO: NOTE: This is technically part of the EPSR, not its own register
        Register(name="fpscr", size=4, floating_point=True),
        # TODO: NOTE: This is also part of EPSR
        Register(name="itstate", size=4, artificial=True, default_value=(0, False, None), concrete=False),
        # Whether exceptions are masked or not. (e.g., everything but NMI)
        Register(name="faultmask", size=4, default_value=(0, False, None)),
        Register(name="faultmask_s", size=4, default_value=(0, False, None)),
        Register(name="faultmask_ns", size=4, default_value=(0, False, None)),
        # The one-byte priority, above which interrupts will not be handled if PRIMASK is 1.
        # Yes, you can implement an RTOS scheduler in hardware with this and the NVIC, you monster!
        Register(name="basepri", size=4, default_value=(0, False, None)),
        Register(name="basepri_s", size=4, default_value=(0, False, None)),
        Register(name="basepri_ns", size=4, default_value=(0, False, None)),
        # Only the bottom bit of PRIMASK is relevant, even though the docs say its 32bit.
        # Configures whether interrupt priorities are respected or not.
        Register(name="primask", size=4, default_value=(0, False, None)),
        Register(name="primask_s", size=4, default_value=(0, False, None)),
        Register(name="primask_ns", size=4, default_value=(0, False, None)),
        # NOTE: We specifically declare IEPSR here.  Not PSR, .... variants.for
        # VEX already handles the content of APSR, and half of EPSR (as ITSTATE) above.
        # We only keep here the data not computed via CCalls
        # The default is to have the T bit on.
        Register(name="iepsr", size=4, default_value=(0x01000000, False, None)),
        # CONTROL:
        # Bit 2: Whether the FPU is active or not
        # Bit 1: Whether we use MSP (0) or PSP (1)
        # Bit 0: Thread mode privilege level. 0 for privileged, 1 for unprivileged.
        Register(name="control", size=4, default_value=(0, False, None)),
    ]

    # Special handling of CM mode in *stone
    if _capstone:
        cs_arch = _capstone.CS_ARCH_ARM
        cs_mode = _capstone.CS_MODE_LITTLE_ENDIAN + _capstone.CS_MODE_THUMB + _capstone.CS_MODE_MCLASS
    _cs_thumb = None
    if _keystone:
        ks_arch = _keystone.KS_ARCH_ARM
        ks_mode = _keystone.KS_MODE_THUMB + _keystone.KS_MODE_LITTLE_ENDIAN
    _ks_thumb = None
    uc_arch = _unicorn.UC_ARCH_ARM if _unicorn else None
    uc_mode = _unicorn.UC_MODE_THUMB + _unicorn.UC_MODE_LITTLE_ENDIAN if _unicorn else None
    uc_mode_thumb = _unicorn.UC_MODE_THUMB + _unicorn.UC_MODE_LITTLE_ENDIAN if _unicorn else None

    dwarf_registers = [
        "r0",
        "r1",
        "r2",
        "r3",
        "r4",
        "r5",
        "r6",
        "r7",
        "r8",
        "r9",
        "r10",
        "r11",
        "r12",
        "sp",
        "lr",
        "pc",
    ]

    fp_ret_offset = RegisterOffset(128)  # s0

    @property
    def capstone_thumb(self):
        return self.capstone

    @property
    def keystone_thumb(self):
        return self.keystone

    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)

    # TODO: Make arm_spotter use these
    # TODO: Make SimOS use these.
    # TODO: Add.... the NVIC? to SimOS


register_arch([r".*cortexm|.*cortex\-m.*|.*v7\-m.*"], 32, Endness.ANY, ArchARMCortexM)
register_arch([r".*armhf.*"], 32, Endness.ANY, ArchARMHF)
register_arch([r".*armeb|.*armbe"], 32, Endness.BE, ArchARM)
register_arch([r".*armel|arm.*"], 32, Endness.LE, ArchARMEL)
register_arch([r".*arm.*|.*thumb.*"], 32, Endness.ANY, ArchARM)
