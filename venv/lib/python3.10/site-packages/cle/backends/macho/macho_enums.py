from __future__ import annotations

from enum import IntEnum


class LoadCommands(IntEnum):
    """
    Enum for all the Load Commands used inside the MachO Binary
    """

    # https://opensource.apple.com/source/cctools/cctools-949.0.1/include/mach-o/loader.h.auto.html
    LC_REQ_DYLD = 0x80000000

    LC_SEGMENT = 0x1
    LC_SYMTAB = 0x2
    LC_SYMSEG = 0x3
    LC_THREAD = 0x4
    LC_UNIXTHREAD = 0x5
    LC_LOADFVMLIB = 0x6
    LC_IDFVMLIB = 0x7
    LC_IDENT = 0x8
    LC_FVMFILE = 0x9
    LC_PREPAGE = 0xA
    LC_DYSYMTAB = 0xB
    LC_LOAD_DYLIB = 0xC
    LC_ID_DYLIB = 0xD
    LC_LOAD_DYLINKER = 0xE
    LC_ID_DYLINKER = 0xF
    LC_PREBOUND_DYLIB = 0x10

    LC_ROUTINES = 0x11
    LC_SUB_FRAMEWORK = 0x12
    LC_SUB_UMBRELLA = 0x13
    LC_SUB_CLIENT = 0x14
    LC_SUB_LIBRARY = 0x15
    LC_TWOLEVEL_HINTS = 0x16
    LC_PREBIND_CKSUM = 0x17

    LC_LOAD_WEAK_DYLIB = 0x18 | LC_REQ_DYLD

    LC_SEGMENT_64 = 0x19
    LC_ROUTINES_64 = 0x1A
    LC_UUID = 0x1B
    LC_RPATH = 0x1C | LC_REQ_DYLD
    LC_CODE_SIGNATURE = 0x1D
    LC_SEGMENT_SPLIT_INFO = 0x1E
    LC_REEXPORT_DYLIB = 0x1F | LC_REQ_DYLD
    LC_LAZY_LOAD_DYLIB = 0x20
    LC_ENCRYPTION_INFO = 0x21
    LC_DYLD_INFO = 0x22
    LC_DYLD_INFO_ONLY = 0x22 | LC_REQ_DYLD
    LC_LOAD_UPWARD_DYLIB = 0x23 | LC_REQ_DYLD
    LC_VERSION_MIN_MACOSX = 0x24
    LC_VERSION_MIN_IPHONEOS = 0x25
    LC_FUNCTION_STARTS = 0x26
    LC_DYLD_ENVIRONMENT = 0x27

    LC_MAIN = 0x28 | LC_REQ_DYLD
    LC_DATA_IN_CODE = 0x29
    LC_SOURCE_VERSION = 0x2A
    LC_DYLIB_CODE_SIGN_DRS = 0x2B
    LC_ENCRYPTION_INFO_64 = 0x2C
    LC_LINKER_OPTION = 0x2D
    LC_LINKER_OPTIMIZATION_HINT = 0x2E
    LC_VERSION_MIN_TVOS = 0x2F
    LC_VERSION_MIN_WATCHOS = 0x30
    LC_NOTE = 0x31
    LC_BUILD_VERSION = 0x32
    LC_DYLD_EXPORTS_TRIE = 0x33 | LC_REQ_DYLD
    LC_DYLD_CHAINED_FIXUPS = 0x34 | LC_REQ_DYLD


class MachoFiletype(IntEnum):
    """
    from mach-o/loader.h

    Constants for the filetype field of the mach_header
    """

    MH_OBJECT = 1  # relocatable object file
    MH_EXECUTE = 2  # demand paged executable file
    MH_FVMLIB = 3  # fixed VM shared library file
    MH_CORE = 4  # core file
    MH_PRELOAD = 5  # preloaded executable file
    MH_DYLIB = 6  # dynamically bound shared library
    MH_DYLINKER = 7  # dynamic link editor
    MH_BUNDLE = 8  # dynamically bound bundle file
    MH_DYLIB_STUB = 9  # shared library stub for static
    MH_DSYM = 10  # companion file with only debug
    MH_KEXT_BUNDLE = 11  # x86_64 kexts
    MH_FILESET = 12  # set of mach-o's


class MH_flags(IntEnum):
    """
    from mach-o/loader.h
    Constants for the flags field of the mach_header
    """

    # the object file has no undefined references
    MH_NOUNDEFS = 0x1
    # the object file is the output of an incremental link against a base file and can't be link edited again
    MH_INCRLINK = 0x2
    # the object file is input for the dynamic linker and can't be staticly link edited again
    MH_DYLDLINK = 0x4
    # the object file's undefined references are bound by the dynamic linker when loaded.
    MH_BINDATLOAD = 0x8
    # the file has its dynamic undefined references prebound.
    MH_PREBOUND = 0x10
    # the file has its read-only and read-write segments split
    MH_SPLIT_SEGS = 0x20
    # the shared library init routine is to be run lazily via catching memory faults to its writeable segments(obsolete)
    MH_LAZY_INIT = 0x40
    # the image is using two-level name space bindings
    MH_TWOLEVEL = 0x80
    # the executable is forcing all images to use flat name space bindings
    MH_FORCE_FLAT = 0x100
    # this umbrella guarantees no multiple defintions of symbols in its sub-images
    # so the two-level namespace hints can always be used.
    MH_NOMULTIDEFS = 0x200
    # do not have dyld notify the prebinding agent about this executable
    MH_NOFIXPREBINDING = 0x400
    # the binary is not prebound but can have its prebinding redone. only used when MH_PREBOUND is not set.
    MH_PREBINDABLE = 0x800
    # indicates that this binary binds to all two-level namespace modules of its dependent libraries.
    # only used when MH_PREBINDABLE and MH_TWOLEVEL are both set.
    MH_ALLMODSBOUND = 0x1000
    # safe to divide up the sections into sub-sections via symbols for dead code stripping
    MH_SUBSECTIONS_VIA_SYMBOLS = 0x2000
    # the binary has been canonicalized via the unprebind operation
    MH_CANONICAL = 0x4000
    # the final linked image contains external weak symbols
    MH_WEAK_DEFINES = 0x8000
    # the final linked image uses weak symbols
    MH_BINDS_TO_WEAK = 0x10000
    # When this bit is set, all stacks in the task will be given stack execution privilege.
    # Only used in MH_EXECUTE filetypes.
    MH_ALLOW_STACK_EXECUTION = 0x20000
    # When this bit is set, the binary declares it is safe for use in processes with uid zero
    MH_ROOT_SAFE = 0x40000
    # When this bit is set, the binary declares it is safe for use in processes when issetugid() is true
    MH_SETUID_SAFE = 0x80000
    # When this bit is set on a dylib,
    # the static linker does not need to examine dependent dylibs to see if any are re-exported
    MH_NO_REEXPORTED_DYLIBS = 0x100000
    # When this bit is set, the OS will load the main executable at a random address. Only used in MH_EXECUTE filetypes.
    MH_PIE = 0x200000
    # Only for use on dylibs.
    # When linking against a dylib that has this bit set, the static linker will automatically not create a
    # LC_LOAD_DYLIB load command to the dylib if no symbols are being referenced from the dylib.
    MH_DEAD_STRIPPABLE_DYLIB = 0x400000
    # Contains a section of type S_THREAD_LOCAL_VARIABLES
    MH_HAS_TLV_DESCRIPTORS = 0x800000
    # When this bit is set, the OS will run the main executable with a non-executable heap
    # even on platforms (e.g. i386) that don't require it. Only used in MH_EXECUTE filetypes.
    MH_NO_HEAP_EXECUTION = 0x1000000
    # The code was linked for use in an application extension.
    MH_APP_EXTENSION_SAFE = 0x02000000
    # The nlist symbol table entries and the external relocation entries refer to the actual symbols and not
    # the dummy symbols.
    # This means that the external relocation entries don't need to be modified when symbols are added or removed.
    # This flag is only used in MH_OBJECT files.
    MH_NLIST_OUTOFSYNC_WITH_DYLDINFO = 0x04000000
    # The object file is a simulator wrapper file.
    MH_SIM_SUPPORT = 0x08000000
    # Only for use on dylibs.
    # When this bit is set, the dylib is part of the dyld shared cache, rather than loose in the filesystem
    MH_DYLIB_IN_CACHE = 0x80000000


class RebaseType(IntEnum):
    """
    from mach-o/loader.h
    """

    POINTER = 1
    TEXT_ABSOLUTE32 = 2
    TEXT_PCREL32 = 3


class RebaseOpcode(IntEnum):
    """
    from mach-o/loader.h

    #define REBASE_OPCODE_MASK					0xF0
    #define REBASE_IMMEDIATE_MASK					0x0F
    #define REBASE_OPCODE_DONE					0x00
    #define REBASE_OPCODE_SET_TYPE_IMM				0x10
    #define REBASE_OPCODE_SET_SEGMENT_AND_OFFSET_ULEB		0x20
    #define REBASE_OPCODE_ADD_ADDR_ULEB				0x30
    #define REBASE_OPCODE_ADD_ADDR_IMM_SCALED			0x40
    #define REBASE_OPCODE_DO_REBASE_IMM_TIMES			0x50
    #define REBASE_OPCODE_DO_REBASE_ULEB_TIMES			0x60
    #define REBASE_OPCODE_DO_REBASE_ADD_ADDR_ULEB			0x70
    #define REBASE_OPCODE_DO_REBASE_ULEB_TIMES_SKIPPING_ULEB	0x80
    """

    DONE = 0x00
    SET_TYPE_IMM = 0x10
    SET_SEGMENT_AND_OFFSET_ULEB = 0x20
    ADD_ADDR_ULEB = 0x30
    ADD_ADDR_IMM_SCALED = 0x40
    DO_REBASE_IMM_TIMES = 0x50
    DO_REBASE_ULEB_TIMES = 0x60
    DO_REBASE_ADD_ADDR_ULEB = 0x70
    DO_REBASE_ULEB_TIMES_SKIPPING_ULEB = 0x80

    @staticmethod
    def parse_byte(byte: int) -> tuple[RebaseOpcode, int]:
        """
        Split a byte into the RebaseOpcode and the immediate value
        :param byte:
        :return:
        """
        assert 0 <= byte <= 255
        return RebaseOpcode(byte & 0xF0), byte & 0x0F
