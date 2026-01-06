# This file is part of Mach-O Loader for CLE.
# Contributed December 2016 by Fraunhofer SIT (https://www.sit.fraunhofer.de/en/).
from __future__ import annotations

import ctypes
import logging
import struct
import sys
import typing
from collections import defaultdict
from io import BufferedReader, BytesIO
from os import SEEK_CUR, SEEK_SET

import archinfo
from sortedcontainers import SortedKeyList

from cle.backends.backend import AT, Backend, register_backend
from cle.backends.macho.binding import BindingHelper, MachOPointerRelocation, MachOSymbolRelocation, read_uleb
from cle.backends.regions import Regions
from cle.errors import CLECompatibilityError, CLEInvalidBinaryError, CLEOperationError

from .encrypted_sentinel_backer import CryptSentinel
from .macho_enums import LoadCommands as LC
from .macho_enums import MachoFiletype, MH_flags
from .section import MachOSection
from .segment import MachOSegment
from .structs import (
    DYLD_CHAINED_PTR_START_NONE,
    ChainedFixupPointerOnDisk,
    DyldChainedPtrFormats,
    DyldImportStruct,
    FileOffset,
    FilePointer,
    MemoryPointer,
    dyld_chained_fixups_header,
    dyld_chained_starts_in_segment,
)
from .symbol import AbstractMachOSymbol, DyldBoundSymbol, SymbolTableSymbol

log = logging.getLogger(name=__name__)

__all__ = ("MachO", "MachOSection", "MachOSegment", "SymbolList")


# pylint: disable=abstract-method
class SymbolList(SortedKeyList):
    """
    Special data structure that extends SortedKeyList to allow looking up a MachO library by name and ordinal quickly
    without having to iterate over the whole list
    """

    _symbol_cache: defaultdict[tuple[str, int], list[AbstractMachOSymbol]]

    def __init__(self, **kwargs):
        super().__init__(**kwargs)
        self._symbol_cache = defaultdict(list)

    def add(self, value: AbstractMachOSymbol):
        super().add(value)
        self._symbol_cache[
            (
                value.name,
                value.library_ordinal,
            )
        ].append(value)

    def get_by_name_and_ordinal(self, name: str, ordinal: int, include_stab=False) -> list[AbstractMachOSymbol]:
        if include_stab:
            return self._symbol_cache[(name, ordinal)]
        else:
            return [symbol for symbol in self._symbol_cache[(name, ordinal)] if not symbol.is_stab]


# pylint: enable =abstract-method


class MachO(Backend):
    """
    Mach-O binaries for CLE
    -----------------------

    The Mach-O format is notably different from other formats. Specifically:

    - Sections are always part of a segment, so `self.sections` will be empty.
    - Symbols cannot be categorized like in ELF.
    - Symbol resolution must be handled by the binary.
    - Rebasing in dyld is implemented by adding a small slide to addresses inside the binary, instead of
      changing the base address of the binary. Consequently, the addresses are absolute rather than relative.
      CLE requires relative addresses, leading to numerous `AT.from_lva().to_rva()` calls in this backend.
    """

    is_default = True  # Tell CLE to automatically consider using the MachO backend

    MH_MAGIC_64 = 0xFEEDFACF
    MH_CIGAM_64 = 0xCFFAEDFE
    MH_MAGIC = 0xFEEDFACE
    MH_CIGAM = 0xCEFAEDFE
    ncmds: int
    sizeofcmds: int

    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.symbols = SymbolList(key=self._get_symbol_relative_addr)

        self.struct_byteorder = None  # holds byteorder for struct.unpack(...)
        self._mapped_base = None  # temporary holder für mapped base derived via loading
        self.cputype = None
        self.cpusubtype = None
        self.filetype: int = None
        self.flags = None  # binary flags
        self.imported_libraries: list[str] = ["Self"]  # ordinal 0 = SELF_LIBRARY_ORDINAL
        self.sections_by_ordinal = [None]  # ordinal 0 = None == Self
        self.exports_by_name = {}  # note exports is currently a raw and unprocessed datastructure.
        # If we intend to use it we must first upgrade it to a class or somesuch
        self.entryoff = None
        self.unixthread_pc = None
        self.os = "macos"
        self.lc_data_in_code = []  # data from LC_DATA_IN_CODE (if encountered). Format: (offset,length,kind)
        self.lc_function_starts: list[int] | None = None  # data from LC_FUNCTION_STARTS (if encountered)
        self.mod_init_func_pointers = []  # may be TUMB interworking
        self.mod_term_func_pointers = []  # may be THUMB interworking
        self.export_blob: bytes | None = None  # exports trie
        self.binding_blob: bytes | None = None  # binding information
        self.lazy_binding_blob: bytes | None = None  # lazy binding information
        self.weak_binding_blob: bytes | None = None  # weak binidng information
        self.rebase_blob: bytes | None = None  # rebasing information
        self.symtab_offset = None  # offset to the symtab
        self.symtab_nsyms = None  # number of symbols in the symtab
        self.binding_done = False  # if true binding was already done and do_bind will be a no-op
        self.strtab: bytes | None = None
        self._indexed_strtab: dict[int, bytes] | None = None
        self._dyld_chained_fixups_offset: int | None = None
        self._dyld_imports: list[AbstractMachOSymbol] = []
        self.indirect_symtab_off: int | None = None  # offset of the indirect symbol table
        self.indirect_syms: int = 0  # number of indirect symbols
        self._stubs: dict[str, int] = {}
        self._cached_stubs = None
        self._cached_reverse_stubs = None

        # For some analysis the insertion order of the symbols is relevant and needs to be kept.
        # This is has to be separate from self.symbols because the latter is sorted by address
        self._ordered_symbols: list[AbstractMachOSymbol] = []

        # The minimum version encoded by the LC_BUILD_VERSION command
        self._minimum_version: tuple[int, int, int] | None = None

        # Begin parsing the file
        try:
            binary_file = self._binary_stream
            # get magic value and determine endianness
            self.struct_byteorder = self._detect_byteorder(struct.unpack("=I", binary_file.read(4))[0])

            # parse the mach header:
            # (ignore all irrelevant fields)
            (_, self.cputype, self.cpusubtype, self.filetype, self.ncmds, self.sizeofcmds, self.flags) = self._unpack(
                "7I", binary_file, 0, 28
            )

            # Libraries are always implicitly PIC
            self.pic = bool(self.flags & MH_flags.MH_PIE) or bool(self.filetype & MachoFiletype.MH_DYLIB)

            if not bool(self.flags & MH_flags.MH_TWOLEVEL):  # ensure MH_TWOLEVEL
                log.error(
                    "Binary is not using MH_TWOLEVEL namespacing."
                    "This isn't properly implemented yet and will degrade results in unpredictable ways."
                    "Please open an issue if you encounter this with a binary you can share"
                )

            # determine architecture
            arch_ident = self._detect_arch_ident()
            if not arch_ident:
                raise CLECompatibilityError(f"Unsupported architecture: 0x{self.cputype:X}:0x{self.cpusubtype:X}")

            # Create archinfo
            # Note that this should be customized for Apple ABI (TODO)
            self.set_arch(archinfo.arch_from_id(arch_ident, endness="lsb" if self.struct_byteorder == "<" else "msb"))

            # Determine the base address the binary was linked against
            # and set the values for the Backend and Loader accordingly
            if self.pic and self.filetype == MachoFiletype.MH_EXECUTE:
                assert self.is_main_bin, "An file of type MH_EXECUTE should be the main bin, this should not happen"
                # a Position Independent Main binary would later be loaded at 0x400000, which isn't legal for Mach-O
                # Also, its segment vaddrs are relative to 0x100000000, so we set this as the linked base
                # and the MachO Backend code uses the AdressTranslator to translate linked addresses to relative ones
                # In theory this is the place where the slide for rebasing should be added, but this isn't supported yet
                if self.arch.bits == 64:
                    self.linked_base = self.mapped_base = 2**32
                elif self.arch.bits == 32:
                    self.linked_base = self.mapped_base = 0x4000
            elif self.filetype == MachoFiletype.MH_DYLIB and self.is_main_bin:
                # the segments of dylibs are just relative to the load address, i.e. the lowest segment addr is 0
                # we need to set the load address to something because otherwise the loader will try to map the
                # file to 0x400000, which is technically illegal for Mach-O because of PAGEZERO
                #
                # The problem is that libraries also tend to have relative pointers (e.g. inside ObjC Metadata),
                # which are rebased by parsing the rebase_blob, which isn't supported yet (but coming soon)
                # so we set the base addr to 0 to make them work out without having to deal with this
                # IDA and Ghidra both seem to handle it this way too
                # AFAIU this isn't a problem with iOS15+ binaries anymore that use the new binding fixups
                # but for now we just load all libraries, that are loaded as the main object, at address 0
                #
                # We can't set the linked base to request this, because the MachO Backend implementation
                # uses this to recalculate the addresses
                self._custom_base_addr = 0
            elif self.filetype == MachoFiletype.MH_DYLIB and not self.is_main_bin:
                # A Library is loaded as a dependency, this is fine, the loader will map it to somewhere above the main
                # binary, so we don't need to do anything
                pass
            else:
                # This case is not explicitly supported yet.
                # There are various other MachoFiletypes, which might have different quirks in their loading
                raise CLECompatibilityError(
                    f"Unsupported Mach-O file type: {MachoFiletype(self.filetype)}. "
                    "Please open an issue if you need support for this"
                )

            # Start reading load commands
            lc_offset = (7 if self.arch.bits == 32 else 8) * 4

            self._parse_load_commands(lc_offset)

        except OSError as e:
            log.exception(e)
            raise CLEOperationError(e) from e

        # File is read, begin populating internal fields
        log.info("Parsing exports")
        self._parse_exports()

        if "__mh_execute_header" in self.exports_by_name:
            assert self.exports_by_name["__mh_execute_header"][1] == self.linked_base, (
                "This binary doesn't have a proper __mh_execute_header export, "
                "this breaks assumptions, please report this"
            )

        self._resolve_entry()

        log.info("Parsing %s symbols", self.symtab_nsyms)
        self._parse_symbols(binary_file)
        log.info("Parsing module init/term function pointers")
        self._parse_mod_funcs()

        if self._dyld_chained_fixups_offset:
            log.info("Parsing dyld bound symbols and fixup chains (ios15 and above)")
            self._parse_dyld_chained_fixups()
        else:
            log.info("Parsing binding bytecode stream")
            self.do_binding()

        self._load_stubs()

    @property
    def stubs(self):
        """
        Maps names to addresses.
        """
        if self._cached_stubs is None:
            self._cached_stubs = {k: AT.from_rva(v, self).to_mva() for k, v in self._stubs.items()}
        return self._cached_stubs

    plt = stubs

    @property
    def reverse_stubs(self):
        """
        Maps addresses to names.
        """
        if self._cached_reverse_stubs is None:
            self._cached_reverse_stubs = {AT.from_rva(v, self).to_mva(): k for k, v in self._stubs.items()}
        return self._cached_reverse_stubs

    reverse_plt = reverse_stubs

    def set_arch(self, arch):
        super().set_arch(arch)
        self.memory = CryptSentinel(arch=arch)

    @property
    def min_addr(self):
        return self.mapped_base

    @classmethod
    def check_compatibility(cls, spec, obj):
        # TODO: Check properly, but for now libs are just used via force load libs anyway
        return True

    def _parse_load_commands(self, lc_offset):
        # Possible optimization: Remove all unnecessary calls to seek()
        # Load commands have a common structure: First 4 bytes identify the command by a magic number
        # second 4 bytes determine the commands size. Everything after this generic "header" is command-specific
        # this makes parsing the commands easy.
        # The documentation for Mach-O is at
        # http://opensource.apple.com//source/xnu/xnu-1228.9.59/EXTERNAL_HEADERS/mach-o/loader.h
        binary_file = self._binary_stream
        count = 0
        offset = lc_offset
        while count < self.ncmds and (offset - lc_offset) < self.sizeofcmds:
            count += 1
            (cmd, size) = self._unpack("II", binary_file, offset, 8)

            # check for segments that interest us
            if cmd in [LC.LC_SEGMENT, LC.LC_SEGMENT_64]:  # LC_SEGMENT,LC_SEGMENT_64
                log.debug("Found LC_SEGMENT(_64) @ %#x", offset)
                self._load_segment(binary_file, offset)
            elif cmd == LC.LC_SYMTAB:  # LC_SYMTAB
                log.debug("Found LC_SYMTAB @ %#x", offset)
                self._load_symtab(binary_file, offset)
            elif cmd in [LC.LC_DYLD_INFO, LC.LC_DYLD_INFO_ONLY]:  # LC_DYLD_INFO(_ONLY)
                log.debug("Found LC_DYLD_INFO(_ONLY) @ %#x", offset)
                self._load_dyld_info(binary_file, offset)
            elif cmd in [LC.LC_LOAD_DYLIB, LC.LC_LOAD_WEAK_DYLIB, LC.LC_REEXPORT_DYLIB]:
                log.debug("Found LC_*_DYLIB @ %#x", offset)
                self._load_dylib_info(binary_file, offset)
            elif cmd == LC.LC_RPATH:  # LC_RPATH
                log.debug("Found LC_RPATH @ %#x", offset)
            elif cmd == LC.LC_MAIN:  # LC_MAIN
                log.debug("Found LC_MAIN @ %#x", offset)
                self._load_lc_main(binary_file, offset)
            elif cmd == LC.LC_UNIXTHREAD:  # LC_UNIXTHREAD
                log.debug("Found LC_UNIXTHREAD @ %#x", offset)
                self._load_lc_unixthread(binary_file, offset)
            elif cmd == LC.LC_FUNCTION_STARTS:  # LC_FUNCTION_STARTS
                log.debug("Found LC_FUNCTION_STARTS @ %#x", offset)
                self._load_lc_function_starts(binary_file, offset)
            elif cmd == LC.LC_DATA_IN_CODE:  # LC_DATA_IN_CODE
                log.debug("Found LC_DATA_IN_CODE @ %#x", offset)
                self._load_lc_data_in_code(binary_file, offset)
            elif cmd in [LC.LC_ENCRYPTION_INFO, LC.LC_ENCRYPTION_INFO_64]:  # LC_ENCRYPTION_INFO(_64)
                log.debug("Found LC_ENCRYPTION_INFO @ %#x", offset)
                # Store the offset and size of the encrypted section for later
                (_, _, cryptoff, cryptsize, cryptid) = self._unpack("5I", binary_file, offset, 20)
                self.memory.set_crypt_info(cryptid, cryptoff, cryptsize)

            elif cmd in [LC.LC_DYLD_CHAINED_FIXUPS]:
                log.info("Found LC_DYLD_CHAINED_FIXUPS @ %#x", offset)
                (_, _, dataoff, datasize) = self._unpack("4I", binary_file, offset, 16)
                self._dyld_chained_fixups_offset: int = dataoff
            elif cmd in [LC.LC_BUILD_VERSION]:
                log.info("Found LC_BUILD_VERSION @ %#x", offset)
                (_, _, _platform, minos, _sdk, _ntools) = self._unpack("6I", binary_file, offset, 6 * 4)
                patch = (minos >> (8 * 0)) & 0xFF
                minor = (minos >> (8 * 1)) & 0xFF
                major = (minos >> (8 * 2)) & 0xFFFF
                self._minimum_version = (major, minor, patch)
                log.info("Found minimum version %s", ".".join([str(i) for i in self._minimum_version]))
            elif cmd in [LC.LC_DYLD_EXPORTS_TRIE]:
                log.info("Found LC_DYLD_EXPORTS_TRIE @ %#x", offset)
                (_, _, dataoff, datasize) = self._unpack("4I", binary_file, offset, 16)
                self.export_blob = self._read(binary_file, dataoff, datasize)
            elif cmd in [LC.LC_DYSYMTAB]:
                log.info("Found LC_DYSYMTAB @ %#x", offset)
                self._load_dysymtab(binary_file, offset)
            else:
                try:
                    log.warning("%s is not handled yet", LC(cmd).name)
                except ValueError:
                    log.error("Command %s is not recognized!", hex(cmd))
            # update bookkeeping
            offset += size

        # Assertion to catch malformed binaries - YES this is needed!
        if count < self.ncmds or (offset - lc_offset) < self.sizeofcmds:
            raise CLEInvalidBinaryError(
                f"Assertion triggered: {count} < {self.ncmds} or {offset - lc_offset} < {self.sizeofcmds}"
            )

    @staticmethod
    def extract_soname(path):
        return path.split("/")[-1]

    @classmethod
    def is_compatible(cls, stream):
        stream.seek(0)
        identstring = stream.read(0x5)
        stream.seek(0)
        if (
            identstring.startswith(struct.pack("I", MachO.MH_MAGIC_64))
            or identstring.startswith(struct.pack("I", MachO.MH_CIGAM_64))
            or identstring.startswith(struct.pack("I", MachO.MH_MAGIC))
            or identstring.startswith(struct.pack("I", MachO.MH_CIGAM))
        ):
            return True
        return False

    def is_thumb_interworking(self, address):
        """Returns true if the given address is a THUMB interworking address"""
        # Note: Untested
        return self.arch.bits != 64 and address & 1

    def decode_thumb_interworking(self, address):
        """Decodes a thumb interworking address"""
        # Note: Untested
        return address & ~1 if self.is_thumb_interworking(address) else address

    def _parse_mod_funcs(self):
        log.debug("Parsing module init/term function pointers")

        fmt = "Q" if self.arch.bits == 64 else "I"
        size = 8 if self.arch.bits == 64 else 4

        # factoring out common code
        def parse_mod_funcs_internal(s, target):
            for i in range(s.vaddr, s.vaddr + s.memsize, size):
                rel_address = AT.from_lva(i, self).to_rva()
                addr = self._unpack_with_byteorder(fmt, self.memory.load(rel_address, size))[0]
                log.debug("Addr: %#x", addr)
                target.append(addr)

        for seg in self.segments:
            seg: MachOSection | MachOSegment
            for sec in seg.sections:
                if sec.type == 0x9:  # S_MOD_INIT_FUNC_POINTERS
                    log.debug("Section %s contains init pointers", sec.sectname)
                    parse_mod_funcs_internal(sec, self.mod_init_func_pointers)
                elif sec.type == 0xA:  # S_MOD_TERM_FUNC_POINTERS
                    log.debug("Section %s contains term pointers", sec.sectname)
                    parse_mod_funcs_internal(sec, self.mod_term_func_pointers)

        log.debug("Done parsing module init/term function pointers")

    def find_segment_by_name(self, name):
        for s in self.segments:
            s: MachOSection | MachOSegment
            if s.segname == name:
                return s
        return None

    def _resolve_entry(self):
        if self.entryoff:
            self._entry = self.linked_base + self.entryoff
        elif self.unixthread_pc:
            self._entry = self.unixthread_pc
        else:
            log.warning("No entry point found")
            self._entry = 0

    @staticmethod
    def _read(fp: BufferedReader, offset: int, size: int) -> bytes:
        """
        Simple read abstraction, reads size bytes from offset in file
        :param offset: Offset to seek() to
        :param size: number of bytes to be read
        :return: string of bytes or "" for EOF
        """
        fp.seek(offset)
        return fp.read(size)

    def _unpack_with_byteorder(self, fmt, data) -> tuple[typing.Any, ...]:
        """
        Appends self.struct_byteorder before fmt to ensure usage of correct byteorder
        :return: struct.unpack(self.struct_byteorder+fmt,input)
        """
        return struct.unpack(self.struct_byteorder + fmt, data)

    def _unpack(self, fmt: str, fp: BufferedReader, offset: FilePointer, size: int) -> tuple[typing.Any, ...]:
        """Convenience"""
        return self._unpack_with_byteorder(fmt, self._read(fp, offset, size))

    @staticmethod
    def _detect_byteorder(magic):
        """Determines the binary's byteorder"""

        log.debug("Magic is %#x", magic)

        host_is_little = sys.byteorder == "little"

        if host_is_little:
            if magic in [MachO.MH_MAGIC_64, MachO.MH_MAGIC]:
                log.debug("Detected little-endian")
                return "<"
            elif magic in [MachO.MH_CIGAM, MachO.MH_CIGAM_64]:
                log.debug("Detected big-endian")
                return ">"
            else:
                log.debug("Not a mach-o file")
                raise CLECompatibilityError()
        else:
            if magic in [MachO.MH_MAGIC_64, MachO.MH_MAGIC]:
                log.debug("Detected big-endian")
                return ">"
            elif magic in [MachO.MH_CIGAM_64, MachO.MH_CIGAM]:
                log.debug("Detected little-endian")
                return "<"
            else:
                log.debug("Not a mach-o file")
                raise CLECompatibilityError()

    def do_binding(self):
        # Perform binding

        if self.binding_done:
            log.warning("Binding already done, reset self.binding_done to override if you know what you are doing")
            return

        bh = BindingHelper(self)  # TODO: Make this configurable
        bh.do_normal_bind(self.binding_blob)
        bh.do_lazy_bind(self.lazy_binding_blob)
        bh.do_rebases(self.rebase_blob)
        if self.weak_binding_blob is not None and len(self.weak_binding_blob) > 0:
            log.info(
                "Found weak binding blob. According to current state of knowledge, weak binding "
                "is only sensible if multiple binaries are involved and is thus skipped."
            )

        self.binding_done = True

    def _load_stubs(self):
        if "__TEXT,__stubs" not in self.sections_map:
            log.debug("No __TEXT,__stubs section found, skipping stub loading")
            return

        stubs_section = self.sections_map["__TEXT,__stubs"]
        if not isinstance(stubs_section, MachOSection):
            log.debug("__TEXT,__stubs is not a MachOSection, skipping stub loading")
            return

        reloc_by_destaddr: dict[int, MachOSymbolRelocation] = {}

        for reloc in self.relocs:
            if isinstance(reloc, MachOSymbolRelocation):
                reloc_by_destaddr[reloc.dest_addr] = reloc

        if not reloc_by_destaddr:
            return

        stub_addr_by_got_addr: dict[int, int] = {}

        # attempt 1: AARCH64 stubs look like the following:
        # __stubs:000000010000061C                 ADRP            X16, #_puts_ptr@PAGE
        # __stubs:0000000100000620                 LDR             X16, [X16,#_puts_ptr@PAGEOFF]
        # __stubs:0000000100000624                 BR              X16 ; __imp__puts
        if self.arch.name == "AARCH64":
            for addr in range(stubs_section.vaddr, stubs_section.vaddr + stubs_section.memsize, 4 * 3):
                rel_addr = AT.from_lva(addr, self).to_rva()
                instr_bytes = self.memory.load(rel_addr, 4 * 3)
                if len(instr_bytes) != 4 * 3:
                    break
                adrp_instr = instr_bytes[0:4]
                ldr_instr = instr_bytes[4:8]
                br_instr = instr_bytes[8:12]
                # parse ADRP
                if adrp_instr[3] & 0x9F != 0x90:
                    continue
                adrp_imm = (
                    ((adrp_instr[3] >> 5) & 0x3)
                    | (adrp_instr[2] << 13)
                    | (adrp_instr[1] << 5)
                    | ((adrp_instr[0] >> 5) << 2)
                ) << 12
                adrp_reg = adrp_instr[0] & 0x1F
                # parse LDR
                if ldr_instr[3] & 0xBF != 0xB9 or ldr_instr[2] & 0xC0 != 0x40:
                    continue
                ldr_imm = (((ldr_instr[2] & 0x3F) << 6) | ((ldr_instr[1] >> 2) & 0x3F)) * 8
                ldr_regn = ((ldr_instr[1] & 3) << 3) | (ldr_instr[0] >> 5)
                ldr_regt = ldr_instr[0] & 0x1F
                if ldr_regn != adrp_reg:
                    continue
                # parse BR
                if br_instr[3] != 0xD6 or br_instr[2] != 0x1F or br_instr[1] & 0xFC != 0x00 or br_instr[0] & 0x1F != 0:
                    continue
                br_reg = (br_instr[0] >> 5) | ((br_instr[1] & 7) << 3)
                if ldr_regt == br_reg:
                    # likely a stub
                    got_addr = (addr & 0xFFFFFFFFFFFFF000) + adrp_imm + ldr_imm
                    stub_addr_by_got_addr[got_addr] = addr

        # attempt 2: x86_64 stubs look like the following:
        # __stubs:0000000100003F12                 jmp     cs:_fclose_ptr
        elif self.arch.name == "AMD64":
            for addr in range(stubs_section.vaddr, stubs_section.vaddr + stubs_section.memsize, 6):
                rel_addr = AT.from_lva(addr, self).to_rva()
                instr_bytes = self.memory.load(rel_addr, 6)
                if len(instr_bytes) != 6:
                    break
                if instr_bytes[0] != 0xFF or instr_bytes[1] != 0x25:
                    continue
                # parse JMP
                disp = struct.unpack("<I", instr_bytes[2:6])[0]
                got_addr = addr + 6 + disp
                stub_addr_by_got_addr[got_addr] = addr

        for got_addr, addr in stub_addr_by_got_addr.items():
            got_rva = AT.from_lva(got_addr, self).to_rva()
            if got_rva in reloc_by_destaddr:
                reloc = reloc_by_destaddr[got_rva]
                # found it!
                self._stubs[reloc.symbol.name] = AT.from_lva(addr, self).to_rva()

    def _parse_exports(self):
        """
        Parses the exports trie
        """
        log.debug("Parsing exports")
        blob = self.export_blob
        if blob is None or len(blob) == 0:
            log.debug("Parsing exports done: No exports found")
            return

        # Note some of these fields are currently not used, keep them in to make used variables explicit
        index = 0
        sym_str = b""
        # index,str
        nodes_to_do = [(0, b"")]
        blob_f = BytesIO(blob)  # easier to handle seeking here

        # constants
        # FLAGS_KIND_MASK = 0x03
        # FLAGS_KIND_REGULAR = 0x00
        # FLAGS_KIND_THREAD_LOCAL = 0x01
        # FLAGS_WEAK_DEFINITION = 0x04
        FLAGS_REEXPORT = 0x08
        FLAGS_STUB_AND_RESOLVER = 0x10

        try:
            while True:
                index, sym_str = nodes_to_do.pop()
                log.debug("Processing node %#x %r", index, sym_str)
                blob_f.seek(index, SEEK_SET)
                info_len = struct.unpack("B", blob_f.read(1))[0]
                if info_len > 127:
                    # special case
                    blob_f.seek(-1, SEEK_CUR)
                    tmp = read_uleb(blob, blob_f.tell())  # a bit kludgy
                    info_len = tmp[0]
                    blob_f.seek(tmp[1], SEEK_CUR)

                if info_len > 0:
                    # a symbol is complete
                    tmp = read_uleb(blob, blob_f.tell())
                    blob_f.seek(tmp[1], SEEK_CUR)
                    flags = tmp[0]
                    if flags & FLAGS_REEXPORT:
                        # REEXPORT: uleb:lib ordinal, zero-term str
                        tmp = read_uleb(blob, blob_f.tell())
                        blob_f.seek(tmp[1], SEEK_CUR)
                        lib_ordinal = tmp[0]
                        lib_sym_name = b""
                        char = blob_f.read(1)
                        while char != b"\0":
                            lib_sym_name += char
                            char = blob_f.read(1)
                        log.info("Found REEXPORT export %r: %d,%r", sym_str, lib_ordinal, lib_sym_name)
                        self.exports_by_name[sym_str.decode()] = (flags, lib_ordinal, lib_sym_name.decode())
                    elif flags & FLAGS_STUB_AND_RESOLVER:
                        # STUB_AND_RESOLVER: uleb: stub offset, uleb: resovler offset
                        log.warning("EXPORT: STUB_AND_RESOLVER found")
                        tmp = read_uleb(blob, blob_f.tell())
                        blob_f.seek(tmp[1], SEEK_CUR)
                        stub_offset = tmp[0]
                        tmp = read_uleb(blob, blob_f.tell())
                        blob_f.seek(tmp[1], SEEK_CUR)
                        resolver_offset = tmp[0]
                        log.info("Found STUB_AND_RESOLVER export %r: %#x,%#x'", sym_str, stub_offset, resolver_offset)
                        self.exports_by_name[sym_str.decode()] = (flags, stub_offset, resolver_offset)
                    else:
                        # normal: offset from mach header
                        tmp = read_uleb(blob, blob_f.tell())
                        blob_f.seek(tmp[1], SEEK_CUR)
                        symbol_offset = tmp[0] + self.linked_base
                        log.debug("Found normal export %r: %#x", sym_str, symbol_offset)
                        self.exports_by_name[sym_str.decode()] = (flags, symbol_offset)

                child_count = struct.unpack("B", blob_f.read(1))[0]
                for i in range(0, child_count):
                    child_str = sym_str
                    char = blob_f.read(1)
                    while char != b"\0":
                        child_str += char
                        char = blob_f.read(1)
                    tmp = read_uleb(blob, blob_f.tell())
                    blob_f.seek(tmp[1], SEEK_CUR)
                    next_node = tmp[0]
                    log.debug("%d. child: (%#x, %r)", i, next_node, child_str)
                    nodes_to_do.append((next_node, child_str))

        except IndexError:
            # List is empty we are done!
            log.debug("Done parsing exports")

    def _detect_arch_ident(self):
        """
        Determines the binary's architecture by inspecting cputype and cpusubtype.
        :return: archinfo.arch_from_id-compatible ident string
        """
        # determine architecture by major CPU type
        try:
            arch_lookup = {
                # contains all supported architectures. Note that apple deviates from standard ABI, see Apple docs
                0x100000C: "aarch64",
                0xC: "arm",
                0x7: "x86",
                0x1000007: "x64",
            }
            return arch_lookup[self.cputype]  # subtype currently not needed
        except KeyError:
            return None

    def _load_lc_data_in_code(self, f, off):
        log.debug("Parsing data in code")

        (_, _, dataoff, datasize) = self._unpack("4I", f, off, 16)
        for i in range(dataoff, datasize, 8):
            blob = self._unpack("IHH", f, i, 8)
            self.lc_data_in_code.append(blob)

        log.debug("Done parsing data in code")

    def _load_lc_function_starts(self, f, off):
        # note that the logic below is based on Apple's dyldinfo.cpp, no official docs seem to exist
        log.debug("Parsing function starts")
        (_, _, dataoff, datasize) = self._unpack("4I", f, off, 16)

        i = 0
        end = datasize
        blob = self._read(f, dataoff, datasize)
        self.lc_function_starts = []

        address = None
        for seg in self.segments:
            if seg.offset == 0 and seg.filesize != 0:
                address = seg.vaddr
                break

        if address is None:
            log.error("Could not determine base-address for function starts")
            raise CLEInvalidBinaryError()
        log.debug("Located base-address: %#x", address)

        while i < end:
            uleb = read_uleb(blob, i)

            if blob[i] == 0:
                break  # list is 0 terminated

            address += uleb[0]

            self.lc_function_starts.append(address)
            log.debug("Function start @ %#x (%#x)", uleb[0], address)
            i += uleb[1]
        log.debug("Done parsing function starts")

    def _load_lc_main(self, f, offset):
        if self.entryoff is not None or self.unixthread_pc is not None:
            log.error("More than one entry point for main detected, abort.")
            raise CLEInvalidBinaryError()

        (_, _, self.entryoff, _) = self._unpack("2I2Q", f, offset, 24)
        log.debug("LC_MAIN: entryoff=%#x", self.entryoff)

    def _load_lc_unixthread(self, f, offset):
        if self.entryoff is not None or self.unixthread_pc is not None:
            log.error("More than one entry point for main detected, abort.")
            raise CLEInvalidBinaryError()

        # parse basic structure
        # _, cmdsize, flavor, long_count
        _, _, flavor, _ = self._unpack("4I", f, offset, 16)

        # we only support 4 different types of thread state atm
        # TODO: This is the place to add x86 and x86_64 thread states
        if flavor == 1 and self.arch.bits != 64:  # ARM_THREAD_STATE or ARM_UNIFIED_THREAD_STATE or ARM_THREAD_STATE32
            blob = self._unpack("16I", f, offset + 16, 64)  # parses only until __pc
        elif flavor == 1 and self.arch.bits == 64 or flavor == 6:
            # ARM_THREAD_STATE or ARM_UNIFIED_THREAD_STATE or ARM_THREAD_STATE64
            blob = self._unpack("33Q", f, offset + 16, 264)  # parses only until __pc
        else:
            log.error("Unknown thread flavor: %d", flavor)
            raise CLECompatibilityError()

        self.unixthread_pc = blob[-1]
        log.debug("LC_UNIXTHREAD: __pc=%#x", self.unixthread_pc)

    def _load_dylib_info(self, f, offset):
        (_, _, name_offset, _, _, _) = self._unpack("6I", f, offset, 24)
        lib_path = self.parse_lc_str(f, offset + name_offset).decode("utf-8", errors="replace")
        log.debug("Adding library %r", lib_path)
        lib_base_name = lib_path.rsplit("/", 1)[-1]
        self.deps.append(lib_base_name)
        self.imported_libraries.append(lib_path)

    def _load_dyld_info(self, f: BufferedReader, offset):
        """
        Extracts information blobs for rebasing, binding and export
        """
        (_, _, roff, rsize, boff, bsize, wboff, wbsize, lboff, lbsize, eoff, esize) = self._unpack("12I", f, offset, 48)

        def blob_or_None(f: BufferedReader, off: int, size: int) -> bytes | None:  # helper
            return self._read(f, off, size) if off != 0 and size != 0 else None

        # Extract data blobs
        self.rebase_blob = blob_or_None(f, roff, rsize)
        self.binding_blob = blob_or_None(f, boff, bsize)
        self.weak_binding_blob = blob_or_None(f, wboff, wbsize)
        self.lazy_binding_blob = blob_or_None(f, lboff, lbsize)
        self.export_blob = blob_or_None(f, eoff, esize)

    def _load_symtab(self, f, offset):
        """
        Handles loading of the symbol table
        :param f: input file
        :param offset: offset to the LC_SYMTAB structure
        :return:
        """

        (_, _, symoff, nsyms, stroff, strsize) = self._unpack("6I", f, offset, 24)

        # load string table
        self.strtab = self._read(f, stroff, strsize)

        # Create Dictionary of offsets to strings for quick lookups e.g. during later symbol creation
        _indexed_strtab: dict[int, bytes] = {}
        idx = 0
        for s in self.strtab.split(b"\x00"):
            _indexed_strtab[idx] = s
            idx += len(s) + 1
        self._indexed_strtab = _indexed_strtab

        # store symtab info
        self.symtab_nsyms = nsyms
        self.symtab_offset = symoff

    def _load_dysymtab(self, f, offset: int) -> None:
        """
        Handles loading of the dynamic symbol table
        :param f: input file
        :param offset: offset to the LC_DYSYMTAB structure
        :return:
        """

        (
            _,
            _,
            _ilocalsym,
            _nlocalsym,
            _iextdefsym,
            _nextdefsym,
            _iundefsym,
            _nundefsym,
            _tocoff,
            _ntoc,
            _modtaboff,
            _nmodtab,
            _extrefsymoff,
            _nextrefsyms,
            indirectsymoff,
            nindirectsyms,
            _extreloff,
            _nextrel,
            _locreloff,
            _nlocrel,
        ) = self._unpack("20I", f, offset, 80)

        if nindirectsyms == 0:
            return

        self.indirect_symtab_off = indirectsymoff
        self.indirect_syms = nindirectsyms

    def _parse_symbols(self, f):
        # parse the symbol entries and create (unresolved) MachOSymbols.
        if self.arch.bits == 64:
            packstr = "I2BHQ"
            structsize = 16
        else:
            packstr = "I2BhI"
            structsize = 12

        for i in range(0, self.symtab_nsyms):
            # The relevant struct is nlist_64 which is defined and documented in mach-o/nlist.h
            offset_in_symtab = i * structsize
            offset = offset_in_symtab + self.symtab_offset
            (n_strx, n_type, n_sect, n_desc, n_value) = self._unpack(packstr, f, offset, structsize)
            log.debug("Adding symbol # %d @ %#x: %s,%s,%s,%s,%s", i, offset, n_strx, n_type, n_sect, n_desc, n_value)
            sym = SymbolTableSymbol(self, offset_in_symtab, n_strx, n_type, n_sect, n_desc, n_value)
            self.symbols.add(sym)
            self._ordered_symbols.append(sym)

            log.debug("Symbol # %d @ %#x is '%s'", i, offset, sym.name)

    def get_string(self, start):
        """Loads a string from the string table"""

        if start in self._indexed_strtab:
            return self._indexed_strtab[start]

        end = start
        if end > len(self.strtab):
            raise ValueError()

        while end < len(self.strtab):
            if self.strtab[end] == 0:
                return self.strtab[start:end]
            end += 1
        return self.strtab[start:]

    def parse_lc_str(self, f, start, limit: int | None = None):
        """Parses a lc_str data structure"""
        tmp = self._unpack("c", f, start, 1)[0]
        s = b""
        ctr = 0
        while tmp != b"\0" and (limit is None or ctr < limit):
            s += tmp
            ctr += 1
            tmp = self._unpack("c", f, start + ctr, 1)[0]

        return s

    def _load_segment(self, f, offset):
        """
        Handles LC_SEGMENT(_64) commands
        :param f: input file
        :param offset: starting offset of the LC_SEGMENT command
        :return:
        """
        # determine if 64 or 32 bit segment
        is64 = self.arch.bits == 64
        if not is64:
            segment_s_size = 56
            (_, _, segname, vmaddr, vmsize, fileoff, filesize, maxprot, initprot, nsects, flags) = self._unpack(
                "2I16s8I", f, offset, segment_s_size
            )
        else:
            segment_s_size = 72
            (_, _, segname, vmaddr, vmsize, fileoff, filesize, maxprot, initprot, nsects, flags) = self._unpack(
                "2I16s4Q4I", f, offset, segment_s_size
            )

        # Cleanup segname
        segname = segname.replace(b"\0", b"")
        log.debug("Processing segment %r", segname)

        # create segment
        seg = MachOSegment(fileoff, vmaddr, filesize, vmsize, segname, nsects, [], flags, initprot, maxprot)

        # Parse section datastructures
        if not is64:
            # 32 bit
            section_s_size = 68
            section_s_packstr = "16s16s9I"
        else:
            # 64 bit
            section_s_size = 80
            # The correct packstring is "16s16s2Q8I", however we use a different one that merges the last two reserved
            # fields (reserved2,reserved3) because it makes the parsing logic below easier
            section_s_packstr = "16s16s2Q6IQ"

        section_start = offset + segment_s_size
        for i in range(0, nsects):
            # Read section
            log.debug("Processing section # %d in %r", i + 1, segname)
            (
                section_sectname,
                section_segname,
                section_vaddr,
                section_vsize,
                section_foff,
                section_align,
                section_reloff,
                section_nreloc,
                section_flags,
                r1,
                r2,
            ) = self._unpack(section_s_packstr, f, (i * section_s_size) + section_start, section_s_size)

            # Clean segname and sectname
            section_sectname = section_sectname.replace(b"\0", b"")
            section_segname = section_segname.replace(b"\0", b"")

            # Create section
            sec = MachOSection(
                section_foff,
                section_vaddr,
                section_vsize,
                section_vsize,
                section_segname,
                section_sectname,
                section_align,
                section_reloff,
                section_nreloc,
                section_flags,
                r1,
                r2,
                parent_segment=seg,
            )

            # Store section
            seg.sections.append(sec)
            self.sections.append(sec)
            self.sections_map[sec.full_name] = sec

        # add to sections_by_ordinal
        self.sections_by_ordinal.extend(seg.sections)

        if segname == b"__PAGEZERO":
            # PAGEZERO is a complicated mess so we ignore it entirely
            # It would allocate 4GB of unneeded memory and also break rebasing
            # because now there is a segment that must be at address 0, while the other segments should be slid
            log.info("Found PAGEZERO, skipping backer for memory conservation")
        elif seg.filesize > 0:
            # Append segment data to memory
            blob = self._read(f, seg.offset, seg.filesize)
            if seg.filesize < seg.memsize:
                blob += b"\0" * (seg.memsize - seg.filesize)  # padding

            # The memory of the Backend itself should start at 0, where 0 is the lowest meaningful address
            # In our case this would be the Mach header magic
            # Later this will be loaded at an address like 0x1000000, but that's the job of the loader
            vaddr_offset = AT.from_lva(seg.vaddr, self).to_rva()
            self.memory.add_backer(vaddr_offset, blob)

        # Store segment
        self.segments.append(seg)

    S = typing.TypeVar("S", bound=ctypes.Structure | ctypes.Union)

    def _get_struct(self, struct_type: type[S], offset: int) -> S:
        data = self._read(self._binary_stream, offset, ctypes.sizeof(struct_type))
        return struct_type.from_buffer_copy(data)

    def _read_cstring_from_file(self, start: FilePointer, max_length=None):
        """
        This technically has unnecessary quadratic runtime behavior in `buffer.find` and `buffer+= ...`
        but this shouldn't be noticeable in practice.
        :param start:
        :param max_length:
        :return:
        """
        end = -1
        buffer = b""
        chunk_size = 1024
        while end == -1:
            buffer += self._read(self._binary_stream, start, chunk_size)
            end = buffer.find(b"\x00")
            if end == -1:
                start += chunk_size
            if max_length is not None and len(buffer) > max_length:
                raise ValueError(f"Symbol name exceeds {max_length} bytes, giving up")
        return buffer[:end]

    def _parse_dyld_imports(self, header):
        # Address of Array of dyld_chained_import* structs
        imports_start_addr: FilePointer = self._dyld_chained_fixups_offset + header.imports_offset
        symbols_start_addr: FilePointer = self._dyld_chained_fixups_offset + header.symbols_offset

        import_struct = DyldImportStruct.get_struct(header.imports_format)

        # Parse Imports
        for i in range(header.imports_count):
            import_addr = imports_start_addr + i * ctypes.sizeof(import_struct)
            imp = self._get_struct(import_struct, import_addr)
            sym_name_addr = symbols_start_addr + imp.name_offset
            try:
                sym_name_bytes = self._read_cstring_from_file(sym_name_addr, max_length=2**21)
                sym_name = sym_name_bytes.decode("utf-8")
            except ValueError as e:
                # This symbol string is probably not null terminated, so we can't read it
                log.error("Failed to read symbol name at %x: %s", sym_name_addr, e)
                sym_name = f"<Excessively long symbol name at fileoffset 0x{sym_name_addr:x}>"

            symbols = self.symbols.get_by_name_and_ordinal(sym_name, imp.lib_ordinal)
            if len(symbols) == 1:
                self._dyld_imports.append(symbols[0])
            elif len(symbols) == 0:
                try:
                    log.debug(
                        "Creating DyldBoundSymbol with name %s for library %s",
                        sym_name,
                        self.imported_libraries[imp.lib_ordinal],
                    )
                except IndexError:
                    log.debug(
                        "Creating DyldBoundSymbol with name %s and library ordinal %s (unknown library)",
                        sym_name,
                        imp.lib_ordinal,
                    )
                sym = DyldBoundSymbol(self, sym_name, imp.lib_ordinal)
                self.symbols.add(sym)
                self._dyld_imports.append(sym)
            else:
                raise NotImplementedError(
                    f"Multiple symbols with name {sym_name}" f"for library {self.imported_libraries[imp.lib_ordinal]}."
                )

    def _parse_dyld_chained_fixups(self):
        header: dyld_chained_fixups_header = self._get_struct(
            dyld_chained_fixups_header, self._dyld_chained_fixups_offset
        )

        if header.symbols_format != 0:
            raise NotImplementedError("Dyld fixup symbols are compressed, this isn't supported yet")

        self._parse_dyld_imports(header)

        # Address of the dyld_chained_starts_in_image struct
        segs_addr: FilePointer = self._dyld_chained_fixups_offset + header.starts_offset

        # The struct isn't straightforward to parse with ctypes, so we do it manually
        seg_count = self._unpack("I", self._binary_stream, segs_addr, 4)[0]

        segs: list[FileOffset] = []
        for i in range(seg_count):
            s = self._unpack("I", self._binary_stream, (i * 4) + segs_addr + 4, 4)[0]
            segs.append(s)

            if segs[i] == 0:
                continue

            starts_addr: FilePointer = segs_addr + segs[i]
            starts = self._get_struct(dyld_chained_starts_in_segment, starts_addr)

            seg = self.find_segment_containing(starts.segment_offset)
            # There are weird binaries where the offsets inside the file
            # and inside the virtual addr space don't match anymore.
            # This isn't properly supported yet, and the only known case is the __PII section inside the __ETC segment
            # of rare binaries, which isn't that important for most purposes
            shift = seg.vaddr - (seg.offset)
            if shift != 0:
                assert isinstance(seg, MachOSegment)
                assert seg.segname == "__ETC", (
                    "Only __ETC segments are known to have this shift, please open an"
                    " issue for this binary so it can be investigated"
                )
                log.error("Segment shift detected in, not handling fixups here for now")
                continue

            page_starts_data = self._read(self._binary_stream, starts_addr + 22, starts.page_count * 2)
            page_starts = struct.unpack("<" + ("H" * starts.page_count), page_starts_data)

            pointer_format: DyldChainedPtrFormats = starts.pointer_format
            log.info("Page has pointer_format: %s", pointer_format)
            for j, start in enumerate(page_starts):
                if start == DYLD_CHAINED_PTR_START_NONE:
                    continue
                chain_entry_addr = starts.segment_offset + (j * starts.page_size) + start
                current_chain_addr = chain_entry_addr
                log.info("Reading chain at %x", current_chain_addr)

                while True:
                    chained_rebase_ptr: ChainedFixupPointerOnDisk = self._get_struct(
                        ChainedFixupPointerOnDisk, current_chain_addr
                    )
                    bind = chained_rebase_ptr.isBind(pointer_format)
                    rebase = chained_rebase_ptr.isRebase(pointer_format, self.mapped_base)
                    if bind is not None:
                        libOrdinal, _addend = bind
                        import_symbol = self._dyld_imports[libOrdinal]
                        reloc = MachOSymbolRelocation(self, import_symbol, current_chain_addr, None)
                        self.relocs.append(reloc)
                        # Legacy Code uses bind_xrefs, explicitly add this to make this compatible for now
                        import_symbol.bind_xrefs.append(reloc.dest_addr + self.linked_base)
                        log.debug("Binding for %s found at %x", import_symbol, current_chain_addr)
                    elif rebase is not None:
                        target = self.linked_base + rebase
                        location: MemoryPointer = self.linked_base + current_chain_addr
                        anon_reloc = MachOPointerRelocation(owner=self, relative_addr=current_chain_addr, data=rebase)
                        self.relocs.append(anon_reloc)
                        log.debug("Rebase to %x found at %x", target, location)

                    else:
                        raise CLEInvalidBinaryError("FixupPointer was neither bind nor rebase, that shouldn't happen")

                    skip = chained_rebase_ptr.generic64.rebase.next * 4
                    current_chain_addr += skip
                    if skip == 0:
                        break

    def get_symbol_by_address_fuzzy(self, address):
        """
        Locates a symbol by checking the given address against sym.addr, sym.bind_xrefs and
        sym.symbol_stubs
        """
        for sym in self.symbols:
            if address == sym.relative_addr or address in sym.bind_xrefs or address in sym.symbol_stubs:
                return sym
        return None

    def get_symbol(self, name, include_stab=False, fuzzy=False):  # pylint: disable=arguments-differ
        """
        Returns all symbols matching name.

        Note that especially when include_stab=True there may be multiple symbols with the same
        name, therefore this method always returns an array.

        :param name: the name of the symbol
        :param include_stab: Include debugging symbols NOT RECOMMENDED
        :param fuzzy: Replace exact match with "contains"-style match
        """
        result = []
        for sym in self.symbols:
            if sym.is_stab and not include_stab:
                continue

            if fuzzy:
                if name in sym.name:
                    result.append(sym)
            else:
                if name == sym.name:
                    result.append(sym)

        return result

    def get_symbol_by_insertion_order(self, idx: int) -> AbstractMachOSymbol:
        """

        :param idx: idx when this symbol was inserted
        :return:
        """
        return self._ordered_symbols[idx]

    def get_segment_by_name(self, name):
        """
        Searches for a MachOSegment with the given name and returns it
        :param name: Name of the sought segment
        :return: MachOSegment or None
        """
        for seg in self.segments:
            seg: MachOSection | MachOSegment
            if seg.segname == name:
                return seg

        return None

    def __getitem__(self, item):
        """
        Syntactic sugar for get_segment_by_name
        """
        return self.get_segment_by_name(item)

    segments: Regions[MachOSegment]


register_backend("mach-o", MachO)
