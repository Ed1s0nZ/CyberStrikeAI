# noinspection PyPep8Naming
# This file takes some liberties to match the naming from the dyld implementation
from __future__ import annotations

import ctypes
import enum
from ctypes import Structure, c_uint16, c_uint32, c_uint64

# Some type aliases
FilePointer = int  # Offset into a raw binary file
FileOffset = int  # Offset to another FilePointer
MemoryPointer = int  # Offset into the mapped memory space

# All of this is based on the dyld-852.2 source code


class HelperStruct(Structure):
    """
    Subclass of ctypes.Structure that adds a helpful repr method for debugging
    """

    def __repr__(self):
        r = [f"<struct {self.__class__.__name__}"]
        for name, *_ in self._fields_:
            r.append(f"{name}={hex(getattr(self, name))}")
        r.append(">")
        return " ".join(r)


class DyldImportFormats(enum.IntEnum):
    """
    https://github.com/apple-opensource/dyld/blob/852.2/include/mach-o/fixup-chains.h#L249-L254
    """

    DYLD_CHAINED_IMPORT = (1,)
    DYLD_CHAINED_IMPORT_ADDEND = (2,)
    DYLD_CHAINED_IMPORT_ADDEND64 = (3,)


class DyldChainedPtrFormats(enum.IntEnum):
    """
    https://github.com/apple-opensource/dyld/blob/852.2/include/mach-o/fixup-chains.h#L89-L104

    """

    DYLD_CHAINED_PTR_ARM64E = 1  # stride 8, unauth target is vmaddr
    DYLD_CHAINED_PTR_64 = 2  # target is vmaddr
    DYLD_CHAINED_PTR_32 = 3
    DYLD_CHAINED_PTR_32_CACHE = 4
    DYLD_CHAINED_PTR_32_FIRMWARE = 5
    DYLD_CHAINED_PTR_64_OFFSET = 6  # target is vm offset
    # DYLD_CHAINED_PTR_ARM64E_OFFSET          =  7 # old name
    DYLD_CHAINED_PTR_ARM64E_KERNEL = 7  # stride 4, unauth target is vm offset
    DYLD_CHAINED_PTR_64_KERNEL_CACHE = 8
    DYLD_CHAINED_PTR_ARM64E_USERLAND = 9  # stride 8, unauth target is vm offset
    DYLD_CHAINED_PTR_ARM64E_FIRMWARE = 10  # stride 4, unauth target is vmaddr
    DYLD_CHAINED_PTR_X86_64_KERNEL_CACHE = 11  # stride 1, x86_64 kernel caches
    DYLD_CHAINED_PTR_ARM64E_USERLAND24 = 12  # stride 8, unauth target is vm offset, 24-bit bind


# noinspection PyPep8Naming
class dyld_chained_ptr_arm64e_auth_rebase(HelperStruct):
    """
    https://github.com/apple-opensource/dyld/blob/852.2/include/mach-o/fixup-chains.h#L128-L138
    """

    _fields_ = [
        ("target", c_uint64, 32),
        ("diversity", c_uint64, 16),
        ("addrDiv", c_uint64, 1),
        ("key", c_uint64, 2),
        ("next", c_uint64, 11),
        ("bind", c_uint64, 1),
        ("auth", c_uint64, 1),
    ]


# noinspection PyPep8Naming
class dyld_chained_ptr_arm64e_auth_bind(HelperStruct):
    """
    https://github.com/apple-opensource/dyld/blob/852.2/include/mach-o/fixup-chains.h#L140-L151
    """

    _fields_ = [
        ("ordinal", c_uint64, 16),
        ("zero", c_uint64, 16),
        ("diversity", c_uint64, 16),
        ("addrDiv", c_uint64, 1),
        ("key", c_uint64, 2),
        ("next", c_uint64, 11),
        ("bind", c_uint64, 1),
        ("auth", c_uint64, 1),
    ]


# noinspection PyPep8Naming
class dyld_chained_ptr_arm64e_rebase(HelperStruct):
    """
    https://github.com/apple-opensource/dyld/blob/852.2/include/mach-o/fixup-chains.h#L107-L115
    """

    _fields_ = [
        ("target", c_uint64, 43),
        ("high8", c_uint64, 8),
        ("next", c_uint64, 11),
        ("bind", c_uint64, 1),
        ("auth", c_uint64, 1),
    ]


# noinspection PyPep8Naming
class dyld_chained_ptr_arm64e_bind(HelperStruct):
    """
    https://github.com/apple-opensource/dyld/blob/852.2/include/mach-o/fixup-chains.h#L117-L126
    """

    _fields_ = [
        ("ordinal", c_uint64, 16),
        ("zero", c_uint64, 16),
        ("addend", c_uint64, 19),
        ("next", c_uint64, 11),
        ("bind", c_uint64, 1),
        ("auth", c_uint64, 1),
    ]


# noinspection PyPep8Naming
class dyld_chained_ptr_arm64e_bind24(HelperStruct):
    """
    https://github.com/apple-opensource/dyld/blob/852.2/include/mach-o/fixup-chains.h#L164-L173
    """


# noinspection PyPep8Naming
class dyld_chained_ptr_arm64e_auth_bind24(HelperStruct):
    """
    https://github.com/apple-opensource/dyld/blob/852.2/include/mach-o/fixup-chains.h#L175-L186
    """

    _fields_ = [
        ("ordinal", c_uint64, 24),
        ("zero", c_uint64, 8),
        ("diversity", c_uint64, 16),
        ("addrDiv", c_uint64, 1),
        ("key", c_uint64, 2),
        ("next", c_uint64, 11),
        ("bind", c_uint64, 1),
        ("auth", c_uint64, 1),
    ]


class Arm64e(ctypes.Union):
    """
    named after the Union `Arm64e` from dyld MachOLoaded.h
    https://github.com/apple-opensource/dyld/blob/852.2/dyld3/MachOLoaded.h#L89-L103
    """

    authRebase: dyld_chained_ptr_arm64e_auth_rebase
    authBind: dyld_chained_ptr_arm64e_auth_bind
    rebase: dyld_chained_ptr_arm64e_rebase
    bind: dyld_chained_ptr_arm64e_bind
    bind24: dyld_chained_ptr_arm64e_bind24
    authBind24: dyld_chained_ptr_arm64e_auth_bind24

    _fields_ = [
        ("authRebase", dyld_chained_ptr_arm64e_auth_rebase),
        ("authBind", dyld_chained_ptr_arm64e_auth_bind),
        ("rebase", dyld_chained_ptr_arm64e_rebase),
        ("bind", dyld_chained_ptr_arm64e_bind),
        ("bind24", dyld_chained_ptr_arm64e_bind24),
        ("authBind24", dyld_chained_ptr_arm64e_auth_bind24),
    ]

    @property
    def sign_extended_addend(self):
        assert self.authBind.bind == 1
        assert self.authBind.auth == 0
        addend19 = self.bind.addend
        if addend19 & 0x40000:
            return addend19 | 0xFFFFFFFFFFFC0000
        else:
            return addend19

    @property
    def unpack_target(self):
        assert self.authBind.bind == 0
        assert self.authBind.auth == 0
        return self.rebase.high8 << 56 | self.rebase.target

    @staticmethod
    def check_valid_pointer_format(pointer_format: DyldChainedPtrFormats) -> bool:
        """
        helper to check if a pointer format is relevant for this
        :param pointer_format:
        :return:
        """
        return pointer_format in [
            DyldChainedPtrFormats.DYLD_CHAINED_PTR_ARM64E,
            DyldChainedPtrFormats.DYLD_CHAINED_PTR_ARM64E_USERLAND,
            DyldChainedPtrFormats.DYLD_CHAINED_PTR_ARM64E_USERLAND24,
            DyldChainedPtrFormats.DYLD_CHAINED_PTR_ARM64E_KERNEL,
            DyldChainedPtrFormats.DYLD_CHAINED_PTR_ARM64E_FIRMWARE,
        ]


# noinspection PyPep8Naming
class dyld_chained_ptr_64_rebase(HelperStruct):
    """
    https://github.com/apple-opensource/dyld/blob/852.2/include/mach-o/fixup-chains.h#L153-L161
    """

    target: FilePointer | FileOffset
    high8: int
    next: int
    bind: int

    _fields_ = [
        ("target", c_uint64, 36),
        ("high8", c_uint64, 8),
        ("_reserved", c_uint64, 7),
        ("next", c_uint64, 12),
        ("bind", c_uint64, 1),
    ]

    @property
    def unpackedTarget(self):
        return self.high8 << 56 | self.target


# noinspection PyPep8Naming
class dyld_chained_ptr_64_bind(HelperStruct):
    """
    https://github.com/apple-opensource/dyld/blob/852.2/include/mach-o/fixup-chains.h#L189-L197
    """

    ordinal: int
    addend: int
    next: int
    bind: int

    _fields_ = [
        ("ordinal", c_uint64, 24),
        ("addend", c_uint64, 8),
        ("_reserved", c_uint64, 19),
        ("next", c_uint64, 12),
        ("bind", c_uint64, 1),
    ]


class Generic64(ctypes.Union):
    """
    named after the Union `Generic64` from dyld MachOLoaded.h
    https://github.com/apple-opensource/dyld/blob/852.2/dyld3/MachOLoaded.h#L105-L111
    """

    rebase: dyld_chained_ptr_64_rebase
    bind: dyld_chained_ptr_64_bind

    _fields_ = [
        ("rebase", dyld_chained_ptr_64_rebase),
        ("bind", dyld_chained_ptr_64_bind),
    ]

    @staticmethod
    def check_valid_pointer_format(pointer_format: DyldChainedPtrFormats) -> bool:
        return pointer_format in [
            DyldChainedPtrFormats.DYLD_CHAINED_PTR_64,
            DyldChainedPtrFormats.DYLD_CHAINED_PTR_64_OFFSET,
        ]


# noinspection PyPep8Naming
class ChainedFixupPointerOnDisk(ctypes.Union):
    """
    the ChainedFixupPointerOnDisk union from dyld MachOLoaded.h
    https://github.com/apple-opensource/dyld/blob/852.2/dyld3/MachOLoaded.h#L87-L141
    """

    generic64: Generic64
    arm64e: Arm64e

    _fields_ = [("generic64", Generic64), ("arm64e", Arm64e)]

    def isBind(self, pointer_format: DyldChainedPtrFormats) -> tuple[int, int] | None:
        """
        Port of ChainedFixupPointerOnDisk::isBind(uint16_t pointerFormat, uint32_t& bindOrdinal, int64_t& addend)
        https://github.com/apple-opensource/dyld/blob/852.2/dyld3/MachOLoaded.cpp#L1098-L1147
        Returns None if not a bind (so `if struct.isBind()` works),
        :return:
        """
        # pylint: disable=no-else-raise
        if Arm64e.check_valid_pointer_format(pointer_format):
            # https://github.com/apple-opensource/dyld/blob/852.2/dyld3/MachOLoaded.cpp#L1107-L1124
            if self.arm64e.authBind.bind:
                if self.arm64e.authBind.auth:
                    if pointer_format == DyldChainedPtrFormats.DYLD_CHAINED_PTR_ARM64E_USERLAND24:
                        return self.arm64e.authBind24.ordinal, 0
                    else:
                        return self.arm64e.authBind.ordinal, 0
                else:
                    if pointer_format == DyldChainedPtrFormats.DYLD_CHAINED_PTR_ARM64E_USERLAND24:
                        return self.arm64e.bind24.ordinal, self.arm64e.sign_extended_addend
                    else:
                        return self.arm64e.bind.ordinal, self.arm64e.sign_extended_addend
            else:
                return None
        elif Generic64.check_valid_pointer_format(pointer_format):
            # https://github.com/apple-opensource/dyld/blob/852.2/dyld3/MachOLoaded.cpp#L1126-L1132
            if self.generic64.bind.bind:
                return self.generic64.bind.ordinal, self.generic64.bind.addend
            else:
                return None

        else:
            raise NotImplementedError(f"Not yet supported pointer format {pointer_format}")

    def isRebase(
        self, pointer_format: DyldChainedPtrFormats, preferredLoadAddress: MemoryPointer
    ) -> MemoryPointer | None:
        """
        port of ChainedFixupPointerOnDisk::isRebase(
        uint16_t pointerFormat, uint64_t preferedLoadAddress, uint64_t& targetRuntimeOffset)
        https://github.com/apple-opensource/dyld/blob/852.2/dyld3/MachOLoaded.cpp#L1046-L1096
        :param pointer_format:
        :param preferredLoadAddress: I think that's just the requested base address
        :return:
        """
        # pylint: disable=no-else-raise
        if Arm64e.check_valid_pointer_format(pointer_format):
            # https://github.com/apple-opensource/dyld/blob/852.2/dyld3/MachOLoaded.cpp#L1049-L1067
            if self.arm64e.bind.bind:
                return False
            else:
                if self.arm64e.authRebase.auth:
                    return self.arm64e.authRebase.target
                else:
                    targetRuntimeOffset = self.arm64e.unpack_target
                    if pointer_format in [
                        DyldChainedPtrFormats.DYLD_CHAINED_PTR_ARM64E,
                        DyldChainedPtrFormats.DYLD_CHAINED_PTR_ARM64E_FIRMWARE,
                    ]:
                        targetRuntimeOffset -= preferredLoadAddress

                    return targetRuntimeOffset

        elif Generic64.check_valid_pointer_format(pointer_format):
            # https://github.com/apple-opensource/dyld/blob/852.2/dyld3/MachOLoaded.cpp#L1068-L1076
            rebase = self.generic64.rebase
            if rebase.bind:
                # Then this wasn't actually a rebase
                return False
            else:
                targetRuntimeOffset = rebase.unpackedTarget
                if pointer_format == DyldChainedPtrFormats.DYLD_CHAINED_PTR_64:
                    targetRuntimeOffset -= preferredLoadAddress
                return targetRuntimeOffset
        else:
            raise NotImplementedError(f"Not yet supported pointer format {pointer_format}")


# DYLD_CHAINED_PTR_BASE: Dict[DyldChainedPtrFormats, Type[ChainedFixupPointerOnDisk]] = {
#     DyldChainedPtrFormats.DYLD_CHAINED_PTR_ARM64E: Arm64e,
#     DyldChainedPtrFormats.DYLD_CHAINED_PTR_ARM64E_USERLAND24: Arm64e,
#     DyldChainedPtrFormats.DYLD_CHAINED_PTR_64: Generic64,
#     DyldChainedPtrFormats.DYLD_CHAINED_PTR_64_OFFSET: Generic64
#
# }


class DyldImportStruct(HelperStruct):
    """
    Meta Struct for the different kind of import structs and the fields they are all guaranteed to have
    """

    lib_ordinal: int
    weak_import: bool
    name_offset: FileOffset

    @staticmethod
    def get_struct(pointer: DyldImportFormats) -> type[DyldImportStruct]:
        return {
            DyldImportFormats.DYLD_CHAINED_IMPORT: dyld_chained_import,
            DyldImportFormats.DYLD_CHAINED_IMPORT_ADDEND64: dyld_chained_import_addend64,
            DyldImportFormats.DYLD_CHAINED_IMPORT_ADDEND: dyld_chained_import_addend,
        }[pointer]


# noinspection PyPep8Naming
class dyld_chained_import(DyldImportStruct):
    """
    Struct for symbol format DYLD_CHAINED_IMPORT

    """

    _fields_ = [
        ("lib_ordinal", c_uint32, 8),
        ("weak_import", c_uint32, 1),
        ("name_offset", c_uint32, 23),
    ]


# noinspection PyPep8Naming
class dyld_chained_import_addend(DyldImportStruct):
    """
    https://github.com/apple-opensource/dyld/blob/852.2/include/mach-o/fixup-chains.h#L264-L271
    """

    addend: int
    _fields_ = [
        ("lib_ordinal", c_uint32, 8),
        ("weak_import", c_uint32, 1),
        ("name_offset", c_uint32, 23),
        ("addend", c_uint32),
    ]


# DYLD_CHAINED_IMPORT_ADDEND64
# noinspection PyPep8Naming
class dyld_chained_import_addend64(DyldImportStruct):
    """
    https://github.com/apple-opensource/dyld/blob/852.2/include/mach-o/fixup-chains.h#L273-L281
    """

    addend: int
    _fields_ = [
        ("lib_ordinal", c_uint64, 16),
        ("weak_import", c_uint64, 1),
        ("reserved", c_uint64, 15),
        ("name_offset", c_uint64, 32),
        ("addend", c_uint64),
    ]


# noinspection PyPep8Naming
class dyld_chained_fixups_header(HelperStruct):
    """
    https://github.com/apple-opensource/dyld/blob/852.2/include/mach-o/fixup-chains.h#L36-L46
    """

    fixups_version: int
    starts_offset: FileOffset
    imports_offset: FileOffset
    symbols_offset: FileOffset
    imports_count: int
    imports_format: DyldImportFormats
    symbols_format: int

    _fields_ = [
        ("fixups_version", c_uint32),
        # offset of dyld_chained_starts_in_image in chain_data
        ("starts_offset", c_uint32),
        # offset of imports table in chain_data
        ("imports_offset", c_uint32),
        # offset of symbol strings in chain_data
        ("symbols_offset", c_uint32),
        # number of imported symbol names
        ("imports_count", c_uint32),
        # DYLD_CHAINED_IMPORT*
        ("imports_format", c_uint32),
        # 0 => uncompressed, 1 => zlib compressed
        ("symbols_format", c_uint32),
    ]


# noinspection PyPep8Naming
class dyld_chained_starts_in_image(ctypes.Structure):
    """
    https://github.com/apple-opensource/dyld/blob/852.2/include/mach-o/fixup-chains.h#L48-L54
    """

    seg_count: int
    seg_info_offset: ctypes.Array

    _fields_ = [("seg_count", c_uint32), ("seg_info_offset", c_uint32 * 1)]


DYLD_CHAINED_PTR_START_NONE = 0xFFFF  # used in page_start[] to denote a page with no fixups
DYLD_CHAINED_PTR_START_MULTI = 0x8000  # used in page_start[] to denote a page which has multiple starts
DYLD_CHAINED_PTR_START_LAST = 0x8000  # used in chain_starts[] to denote last start in list for page


# noinspection PyPep8Naming
class dyld_chained_starts_in_segment(HelperStruct):
    """
    https://github.com/apple-opensource/dyld/blob/852.2/include/mach-o/fixup-chains.h#L56-L72
    """

    # size: int
    page_size: int
    _pointer_format: int
    segment_offset: int
    max_valid_pointer: int
    page_count: int
    page_start: int

    _fields_ = [
        # size of this (amount kernel needs to copy)
        ("_size", c_uint32),
        # 0x1000 or 0x4000
        ("page_size", c_uint16),
        # DYLD_CHAINED_PTR_*
        ("_pointer_format", c_uint16),
        # offset in memory to start of segment
        ("segment_offset", c_uint64),
        ("max_valid_pointer", c_uint32),
        ("page_count", c_uint16),
        ("page_start", c_uint16),
    ]

    @property
    def pointer_format(self) -> DyldChainedPtrFormats:
        return DyldChainedPtrFormats(self._pointer_format)


BIND_SPECIAL_DYLIB_WEAK_LOOKUP = -3
