import logging
from typing import Union

from .arch import Arch, Endness, Register
from .archerror import ArchError
from .tls import TLSArchInfo
from .types import RegisterOffset

try:
    import pypcode

    _has_pypcode = True
except ImportError:
    _has_pypcode = False


log = logging.getLogger(__name__)


class ArchPcode(Arch):
    """
    archinfo interface to pypcode architectures. Provides minimal mapping for
    architectural info like register file map, endianness, bit width, etc.
    """

    def __init__(self, language: Union["pypcode.ArchLanguage", str]):
        if not _has_pypcode:
            raise ArchError("pypcode not installed")

        if isinstance(language, str):
            language = self._get_language_by_id(language)

        assert isinstance(language, pypcode.ArchLanguage)

        self.name = language.id
        self.pcode_arch = language.id
        self.description = language.description
        self.bits = int(language.size)
        self.endness = {"little": Endness.LE, "big": Endness.BE}[language.endian]
        self.instruction_endness = self.endness
        self.sizeof = (
            {"short": 16, "int": 32, "long": 64, "long long": 64}
            if self.bits == 64
            else {"short": 16, "int": 32, "long": 32, "long long": 64}
        )
        self.elf_tls = TLSArchInfo(1, 8, [], [0], [], 0, 0)

        # Build registers list
        ctx = pypcode.Context(language)
        archinfo_regs = {rname.lower(): Register(rname.lower(), r.size, r.offset) for rname, r in ctx.registers.items()}

        # Get program counter register
        pc_offset = None
        pc_tag = language.pspec.find("programcounter") if language.pspec is not None else None
        if pc_tag is not None:
            pc_reg = pc_tag.attrib.get("register", None)
            if pc_reg is not None:
                # FIXME: Assumes RAM space
                pc_offset = ctx.registers[pc_reg].offset
                aliases = {"pc", "ip"}
                aliases.discard(pc_reg.lower())
                for alias in aliases:
                    archinfo_regs.pop(alias, None)
                archinfo_regs[pc_reg.lower()].alias_names = tuple(aliases)

        if pc_offset is None:
            log.warning("Unknown program counter register offset?")
            pc_offset = 0x80000000

        sp_offset = None
        ret_offset = RegisterOffset(0)
        if len(language.cspecs):

            def find_matching_cid(language, desired):
                for cid in language.cspecs:
                    if cid[0] == desired:
                        return cid
                return None

            cspec_id = (
                find_matching_cid(language, "default") or find_matching_cid(language, "gcc") or list(language.cspecs)[0]
            )
            cspec = language.cspecs[cspec_id]

            # Get stack pointer register
            sp_tag = cspec.find("stackpointer")
            if sp_tag is not None:
                sp_reg = sp_tag.attrib.get("register", None)
                if sp_reg is not None:
                    # FIXME: Assumes RAM space
                    sp_offset = ctx.registers[sp_reg].offset
                    if sp_reg.lower() != "sp":
                        if "sp" in archinfo_regs:
                            log.warning("Unexpected SP conflict")
                            del archinfo_regs["sp"]
                        archinfo_regs[sp_reg.lower()].alias_names += ("sp",)

            # Get return offset
            proto_tags = cspec.find("default_proto")
            if proto_tags is not None and len(proto_tags) >= 1:
                proto_tag = proto_tags[0]
                output_tags = proto_tag.find("output")
                if output_tags is not None and len(output_tags) >= 1:
                    output_tag = output_tags[0]
                    output_register_tag = output_tag.find("register")
                    if output_register_tag is not None:
                        output_reg = output_register_tag.attrib["name"]
                        ret_offset = RegisterOffset(ctx.registers[output_reg].offset)

        if sp_offset is None:
            log.warning("Unknown stack pointer register offset?")
            sp_offset = 0x80000008

        self.instruction_alignment = 1
        self.ip_offset = RegisterOffset(pc_offset)
        self.sp_offset = RegisterOffset(sp_offset)
        self.bp_offset = RegisterOffset(sp_offset)
        self.ret_offset = RegisterOffset(ret_offset)
        self.register_list = list(archinfo_regs.values())
        self.initial_sp = (0x8000 << (self.bits - 16)) - 1
        self.linux_name = ""  # FIXME
        self.triplet = ""  # FIXME

        # TODO: Replace the following hardcoded function prologues by data sourced from patterns.xml
        if "PowerPC:BE" in self.name:
            self.function_prologs = {
                # stwu  r1, xx(r1); mfspr rx, lr
                b"\x94\x21[\xc0-\xff][\x00\x10\x20\x30\x40\x50\x60\x70\x80\x90\xa0\xb0\xc0\xd0\xe0\xf0]"
                b"[\x7c-\x7f][\x08\x28\x48\x68\x88\xa8\xc8\xe8]\x02\xa6",
            }
        if self.name.startswith("Xtensa:LE"):
            self.function_prologs = {
                # entry  a1, N
                b"\x36[\x11\x21\x31\x41\x51\x61\x71\x81\x91\xa1\xb1\xc1\xd1\xe1\xf1]\x00",
            }

        if "sparc:" in self.name.lower() and self.bits == 32:
            self.branch_delay_slot = True

        super().__init__(endness=self.endness, instruction_endness=self.instruction_endness)

    @staticmethod
    def _get_language_by_id(lang_id) -> "pypcode.ArchLanguage":
        if not _has_pypcode:
            raise ArchError("pypcode not installed")
        for arch in pypcode.Arch.enumerate():
            for lang in arch.languages:
                if lang.id == lang_id:
                    return lang
        raise ArchError("Language not found")

    def disasm(self, bytestring, addr=0, thumb=False):
        ctx = pypcode.Context(self.name)
        ctx.setVariableDefault("TMode", 1 if thumb else 0)
        instructions = ctx.disassemble(bytestring, addr, 0, len(bytestring), 0).instructions
        return "\n".join(f"{insn.addr.offset:#x}:\t{insn.mnem} {insn.body}" for insn in instructions)
