"""
Basic MS COFF object loader based on https://docs.microsoft.com/en-us/windows/win32/debug/pe-format
"""

from __future__ import annotations

import ctypes
import logging
import struct
from enum import IntEnum, IntFlag

import archinfo

from cle.utils import extract_null_terminated_bytestr

from .backend import Backend, register_backend
from .region import Section, Segment
from .relocation import Relocation
from .symbol import Symbol, SymbolType

log = logging.getLogger(__name__)


class IMAGE_FILE_MACHINE(IntEnum):
    """
    Machine Types
    """

    I386 = 0x14C
    AMD64 = 0x8664


class CoffFileHeader(ctypes.LittleEndianStructure):
    """
    COFF File Header
    """

    _pack_ = 1
    _fields_ = [
        ("Machine", ctypes.c_uint16),
        ("NumberOfSections", ctypes.c_uint16),
        ("TimeDateStamp", ctypes.c_uint32),
        ("PointerToSymbolTable", ctypes.c_uint32),
        ("NumberOfSymbols", ctypes.c_uint32),
        ("SizeOfOptionalHeader", ctypes.c_uint16),
        ("Characteristics", ctypes.c_uint16),
    ]


class IMAGE_SCN(IntFlag):
    """
    Section Flags (Characteristics field)
    """

    MEM_EXECUTE = 0x20000000
    MEM_READ = 0x40000000
    MEM_WRITE = 0x80000000
    CNT_UNINITIALIZED_DATA = 0x00000080


class CoffSectionTableEntry(ctypes.LittleEndianStructure):
    """
    COFF Section Header
    """

    _pack_ = 1
    _fields_ = [
        ("Name", ctypes.c_uint8 * 8),
        ("VirtualSize", ctypes.c_uint32),
        ("VirtualAddress", ctypes.c_uint32),
        ("SizeOfRawData", ctypes.c_uint32),
        ("PointerToRawData", ctypes.c_uint32),
        ("PointerToRelocations", ctypes.c_uint32),
        ("PointerToLinenumbers", ctypes.c_uint32),
        ("NumberOfRelocations", ctypes.c_uint16),
        ("NumberOfLinenumbers", ctypes.c_uint16),
        ("Characteristics", ctypes.c_uint32),
    ]


class IMAGE_SYM_CLASS(IntEnum):
    """
    Symbol Storage Class
    """

    EXTERNAL = 2
    STATIC = 3
    LABEL = 6
    FUNCTION = 101


class CoffSymbolTableEntry(ctypes.LittleEndianStructure):
    """
    COFF Symbol Table Entry
    """

    _pack_ = 1
    _fields_ = [
        ("Name", ctypes.c_uint8 * 8),
        ("Value", ctypes.c_uint32),
        ("SectionNumber", ctypes.c_int16),
        ("Type", ctypes.c_uint16),
        ("StorageClass", ctypes.c_uint8),
        ("NumberOfAuxSymbols", ctypes.c_uint8),
    ]


class IMAGE_REL_I386(IntEnum):
    """
    i386 Relocation Types
    """

    DIR32 = 0x0006
    DIR32NB = 0x0007
    REL32 = 0x0014
    SECTION = 0x000A
    SECREL = 0x000B


class IMAGE_REL_AMD64(IntEnum):
    """
    AMD64 Relocation Types
    """

    ADDR64 = 0x0001
    ADDR32NB = 0x0003
    REL32 = 0x0004
    SECTION = 0x000A
    SECREL = 0x000B


class CoffRelocationTableEntry(ctypes.LittleEndianStructure):
    """
    COFF Relocations
    """

    _pack_ = 1
    _fields_ = [
        ("VirtualAddress", ctypes.c_uint32),
        ("SymbolTableIndex", ctypes.c_uint32),
        ("Type", ctypes.c_uint16),
    ]


class CoffParser:
    """
    Parses COFF object files.
    """

    data: bytes
    header: CoffFileHeader
    sections: list[CoffSectionTableEntry]
    relocations: list[list[CoffRelocationTableEntry]]
    symbols: list[CoffSymbolTableEntry]

    # Note: Symbols are uniquely identified by their index. It is possible for multiple symbols to have the same name so
    # in idx_to_symbol_name and symbol_name_to_idx, numeric suffixes are appended when necessary. To get the true name
    # of a symbol at index `symbol_idx`, call get_symbol_name(symbol_idx, true_name=True).
    idx_to_symbol_name: dict[int, str]
    symbol_name_to_idx: dict[str, int]

    def __init__(self, data: bytes):
        if data.startswith(b"\x00\x00\xff\xff"):
            raise ValueError(
                "This object file appears to have been compiled with whole program optimization (/GL flag)"
                " and cannot be parsed by this library"
            )
        self.data: bytes = data
        self._parse()

    def _parse(self) -> None:
        self.header = CoffFileHeader.from_buffer_copy(self.data)
        if self.header.Machine not in {
            IMAGE_FILE_MACHINE.I386,
            IMAGE_FILE_MACHINE.AMD64,
        }:
            raise NotImplementedError("Unsupported machine type")

        strings_offset = (
            self.header.PointerToSymbolTable + ctypes.sizeof(CoffSymbolTableEntry) * self.header.NumberOfSymbols
        )
        strings_size = struct.unpack("<I", self.data[strings_offset : strings_offset + 4])[0]
        self.strings: bytes = self.data[strings_offset : strings_offset + strings_size]

        self.symbols = []
        self.symbol_name_to_idx = {}
        self.idx_to_symbol_name = {}

        offset = self.header.PointerToSymbolTable
        aux = 0
        for i in range(self.header.NumberOfSymbols):
            symbol = CoffSymbolTableEntry.from_buffer_copy(self.data, offset)
            offset += ctypes.sizeof(CoffSymbolTableEntry)
            self.symbols.append(symbol)
            if aux:
                aux -= 1
                continue
            idx = len(self.symbols) - 1
            name = self.get_symbol_name(idx)
            aux = symbol.NumberOfAuxSymbols

            # Ensure unique symbol names
            i = 1
            base_name = name
            while name in self.symbol_name_to_idx:
                name = base_name + f"__{i}"
                i += 1
            self.symbol_name_to_idx[name] = idx
            self.idx_to_symbol_name[idx] = name

        self.sections = []
        self.relocations = []

        for i in range(self.header.NumberOfSections):
            offset = ctypes.sizeof(self.header) + ctypes.sizeof(CoffSectionTableEntry) * i
            section = CoffSectionTableEntry.from_buffer_copy(self.data, offset)
            self.sections.append(section)

            # Relocations
            relocs = []
            offset = section.PointerToRelocations
            for i in range(section.NumberOfRelocations):
                reloc = CoffRelocationTableEntry.from_buffer_copy(self.data, offset)
                relocs.append(reloc)
                offset += ctypes.sizeof(reloc)
            self.relocations.append(relocs)

    def get_symbol_name(self, symbol_idx: int, true_name: bool = False) -> str:
        if symbol_idx in self.idx_to_symbol_name and not true_name:
            return self.idx_to_symbol_name[symbol_idx]

        name_encoded = bytes(self.symbols[symbol_idx].Name)
        name_as_dwords = struct.unpack("<II", name_encoded)
        if name_as_dwords[0] == 0:
            name_encoded = extract_null_terminated_bytestr(self.strings, offset=name_as_dwords[1])
        return name_encoded.rstrip(b"\x00").decode("ascii")

    def get_section_name(self, section_idx: int) -> str:
        name = bytes(self.sections[section_idx].Name).rstrip(b"\x00").decode("ascii")
        if name.startswith("/"):
            return self.get_symbol_name(int(name[1:]))
        return name


class CoffSection(Section):
    """
    Section of the COFF object.
    """

    def __init__(
        self,
        name: str,
        file_offset: int,
        file_size: int,
        virtual_addr: int,
        virtual_size: int,
        coff_sec: CoffSectionTableEntry,
    ):
        super().__init__(name, file_offset, virtual_addr, virtual_size)
        self.filesize = file_size
        self._coff_sec = coff_sec

    @property
    def is_readable(self):
        return (self._coff_sec.Characteristics & IMAGE_SCN.MEM_READ) != 0

    @property
    def is_writable(self):
        return (self._coff_sec.Characteristics & IMAGE_SCN.MEM_WRITE) != 0

    @property
    def is_executable(self):
        return (self._coff_sec.Characteristics & IMAGE_SCN.MEM_EXECUTE) != 0

    @property
    def only_contains_uninitialized_data(self):
        return (self._coff_sec.Characteristics & IMAGE_SCN.CNT_UNINITIALIZED_DATA) != 0


class CoffRelocation(Relocation):
    """
    Relocation for a COFF object.
    """

    PACK_FORMAT = "<i"

    def relocate(self):
        value = self.value
        if value is None:
            log.debug("Unresolved relocation with no symbol.")
            return False
        self.owner.memory.store(self.relative_addr, struct.pack(self.PACK_FORMAT, value))
        return True


class CoffRelocationREL32(CoffRelocation):
    """
    Relocation for IMAGE_REL_*_REL32
    """

    @property
    def value(self):
        assert self.resolvedby is not None
        org_bytes = self.owner.memory.load(self.relative_addr, 4)
        org_value = struct.unpack("<I", org_bytes)[0]
        return org_value + self.resolvedby.rebased_addr - (self.rebased_addr + 4)


class CoffRelocationDIR32(CoffRelocation):
    """
    Relocation for IMAGE_REL_*_DIR32
    """

    @property
    def value(self):
        assert self.resolvedby is not None
        org_bytes = self.owner.memory.load(self.relative_addr, 4)
        org_value = struct.unpack("<I", org_bytes)[0]
        return org_value + self.resolvedby.rebased_addr


class CoffRelocationDIR32NB(CoffRelocation):
    """
    Relocation for IMAGE_REL_*_DIR32
    """

    @property
    def value(self):
        assert self.resolvedby is not None
        org_bytes = self.owner.memory.load(self.relative_addr, 4)
        org_value = struct.unpack("<I", org_bytes)[0]
        return org_value + self.resolvedby.relative_addr


class CoffRelocationADDR32NB(CoffRelocation):
    """
    Relocation for IMAGE_REL_AMD64_ADDR32NB
    """

    PACK_FORMAT = "<I"

    @property
    def value(self) -> int:
        assert self.resolvedby is not None
        return self.resolvedby.relative_addr


class CoffRelocationADDR64(CoffRelocation):
    """
    Relocation for IMAGE_REL_AMD64_ADDR64
    """

    PACK_FORMAT = "<Q"

    @property
    def value(self):
        assert self.resolvedby is not None
        return self.resolvedby.rebased_addr


class CoffRelocationSECTION(CoffRelocation):
    """
    Relocation for IMAGE_REL_*_SECTION
    """

    PACK_FORMAT = "<H"

    @property
    def value(self):
        assert isinstance(self.owner, Coff)
        section_idx = 0  # FIXME
        return section_idx


class CoffRelocationSECREL(CoffRelocation):
    """
    Relocation for IMAGE_REL_*_SECREL
    """

    PACK_FORMAT = "<I"

    @property
    def value(self):
        assert isinstance(self.owner, Coff)
        offset_to_symbol = 0  # FIXME
        return offset_to_symbol


RELOC_CLASSES: dict[IntEnum, dict[IntEnum, type[Relocation]]] = {
    IMAGE_FILE_MACHINE.I386: {
        IMAGE_REL_I386.REL32: CoffRelocationREL32,
        IMAGE_REL_I386.DIR32: CoffRelocationDIR32,
        IMAGE_REL_I386.DIR32NB: CoffRelocationDIR32NB,
        IMAGE_REL_I386.SECTION: CoffRelocationSECTION,
        IMAGE_REL_I386.SECREL: CoffRelocationSECREL,
    },
    IMAGE_FILE_MACHINE.AMD64: {
        IMAGE_REL_AMD64.ADDR64: CoffRelocationADDR64,
        IMAGE_REL_AMD64.ADDR32NB: CoffRelocationADDR32NB,
        IMAGE_REL_AMD64.REL32: CoffRelocationREL32,
        IMAGE_REL_AMD64.SECTION: CoffRelocationSECTION,
        IMAGE_REL_AMD64.SECREL: CoffRelocationSECREL,
    },
}

COFF_MACHINE_TO_ARCH_NAME = {
    IMAGE_FILE_MACHINE.I386: "x86",
    IMAGE_FILE_MACHINE.AMD64: "AMD64",
}


class Coff(Backend):
    """
    COFF object loader.
    """

    is_default = True

    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)

        if self.binary is None:
            self._data = self._binary_stream.read()
        else:
            with open(self.binary, "rb") as f:
                self._data = f.read()

        self._coff = CoffParser(self._data)

        arch = archinfo.arch_from_id(COFF_MACHINE_TO_ARCH_NAME[self._coff.header.Machine])
        self.set_arch(arch)

        # FIXME: Currently we just map the whole object file for convenience. Create a better memory map, discard object
        # file structure data.
        self._image_vmem = self._data

        # Add each section
        for section_idx, section in enumerate(self._coff.sections):
            section_name = self._coff.get_section_name(section_idx)
            vaddr = section.PointerToRawData
            vsize = section.SizeOfRawData
            self.segments.append(Segment(section.PointerToRawData, vaddr, section.SizeOfRawData, vsize))
            self.sections.append(
                CoffSection(
                    section_name,
                    section.PointerToRawData,
                    section.SizeOfRawData,
                    vaddr,
                    vsize,
                    section,
                )
            )

        self.memory.add_backer(0, bytes(self._image_vmem))
        self.mapped_base = self.linked_base = 0
        self.pic = True
        # assume windows, this can be wrong, but is more often right.
        self.os = "windows"
        self.supports_nx = False

        self._add_defined_symbols()
        self._add_relocs()

        # FIXME: Expose __imp_* symbols through self.imports

    def _add_defined_symbols(self) -> None:
        for sym_name, sym_idx in self._coff.symbol_name_to_idx.items():
            sym = self._coff.symbols[sym_idx]
            if sym.SectionNumber > 0 and sym.StorageClass in {
                IMAGE_SYM_CLASS.STATIC,
                IMAGE_SYM_CLASS.LABEL,
                IMAGE_SYM_CLASS.EXTERNAL,
            }:
                self.symbols.add(self.get_symbol(sym_name))

    def _add_relocs(self) -> None:
        for section_idx, section in enumerate(self._coff.sections):
            for reloc in self._coff.relocations[section_idx]:
                sym = self._coff.symbols[reloc.SymbolTableIndex]
                sym_name = self._coff.get_symbol_name(reloc.SymbolTableIndex)
                patch_offset = section.PointerToRawData + reloc.VirtualAddress

                if sym.StorageClass in {
                    IMAGE_SYM_CLASS.STATIC,
                    IMAGE_SYM_CLASS.LABEL,
                    IMAGE_SYM_CLASS.EXTERNAL,
                }:
                    reloc_class = RELOC_CLASSES[self._coff.header.Machine].get(reloc.Type, None)
                    if reloc_class is not None:
                        cle_symbol = self.get_symbol(sym_name, produce_extern_symbols=True)
                        self.relocs.append(reloc_class(self, cle_symbol, patch_offset))
                        continue

                log.warning("Skipped relocation type %#x at %#x for symbol %s", reloc.Type, patch_offset, sym_name)

    @classmethod
    def is_compatible(cls, stream):
        stream.seek(0)
        identstring = stream.read(2)
        stream.seek(0)
        return int.from_bytes(identstring, "little") in (IMAGE_FILE_MACHINE.I386, IMAGE_FILE_MACHINE.AMD64)

    def get_symbol(self, name: str, produce_extern_symbols: bool = False) -> Symbol | None:
        if name not in self._coff.symbol_name_to_idx:
            return None

        if name == "__ImageBase":
            return Symbol(self, name, 0, 0, SymbolType.TYPE_OTHER)

        sym = self._coff.symbols[self._coff.symbol_name_to_idx[name]]
        if sym.StorageClass in {
            IMAGE_SYM_CLASS.STATIC,
            IMAGE_SYM_CLASS.LABEL,
            IMAGE_SYM_CLASS.EXTERNAL,
        }:
            symbol_type = SymbolType.TYPE_FUNCTION if sym.Type == 0x20 else SymbolType.TYPE_OTHER
            if sym.SectionNumber > 0:
                sym_addr = self._coff.sections[sym.SectionNumber - 1].PointerToRawData + sym.Value
                return Symbol(self, name, sym_addr, 1, symbol_type)
            elif sym.SectionNumber == 0:
                if produce_extern_symbols:
                    return Symbol(self, name, 0, sym.Value, symbol_type)
                return None

        raise NotImplementedError("Unsupported symbol")


register_backend("COFF", Coff)
