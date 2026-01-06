import ctypes
import struct

uint8_t = ctypes.c_ubyte
char = ctypes.c_char
uint32_t = ctypes.c_uint
uint64_t = ctypes.c_uint64
uint16_t = ctypes.c_ushort

ME_HEADER = b"\x20\x20\x80\x0F\x40\x00\x00\x24"
ME_PARTITION_HEADER = b"$FPT"

def replace_bad(value, deletechars):
    for c in deletechars:
        value = value.replace(c, '_')
    return value


def read_struct(li, struct):
    s = struct()
    slen = ctypes.sizeof(s)
    _bytes = li.read(slen)
    fit = min(len(_bytes), slen)
    ctypes.memmove(ctypes.addressof(s), _bytes, fit)
    return s


def get_struct(str_, off, struct):
    s = struct()
    slen = ctypes.sizeof(s)
    _bytes = str_[off:off + slen]
    fit = min(len(_bytes), slen)
    ctypes.memmove(ctypes.addressof(s), _bytes, fit)
    return s


def DwordAt(f, off):
    return struct.unpack("<I", f[off:off + 4])[0]


class MeModuleFileHeader1Type(ctypes.LittleEndianStructure):
    _fields_ = [
        ("Tag",            char * 4),  # $MOD
        ("Unk04",          uint32_t),
        ("Unk08",          uint32_t),
        ("MajorVersion",   uint16_t),
        ("MinorVersion",   uint16_t),
        ("HotfixVersion",  uint16_t),
        ("BuildVersion",   uint16_t),
        ("Unk14",          uint32_t),
        ("CompressedSize", uint32_t),
        ("UncompressedSize", uint32_t),
        ("LoadAddress",    uint32_t),
        ("MappedSize",     uint32_t),
        ("Unk28",          uint32_t),
        ("Unk2C",          uint32_t),
        ("Name",           char * 16),
        ("Guid",           uint8_t * 16),
    ]


class MeModuleHeader1Type(ctypes.LittleEndianStructure):
    _fields_ = [
        ("Tag",            char * 4),  # $MME
        ("Guid",           uint8_t * 16),
        ("MajorVersion",   uint16_t),
        ("MinorVersion",   uint16_t),
        ("HotfixVersion",  uint16_t),
        ("BuildVersion",   uint16_t),
        ("Name",           char * 16),
        ("Hash",           uint8_t * 20),
        ("Size",           uint32_t),
        ("Flags",          uint32_t),
        ("Unk48",          uint32_t),
        ("Unk4C",          uint32_t),
    ]
    udc_format = "<4s20s16sII"
    udc_length = 0x30


class MeModuleHeader2Type(ctypes.LittleEndianStructure):
    _fields_ = [
        ("Tag",            char * 4),  # $MME
        ("Name",           char * 16),
        ("Hash",           uint8_t * 32),
        ("Unk34",          uint32_t),
        ("Offset",         uint32_t),  # From the manifest
        ("Unk3C",          uint32_t),
        ("Size",           uint32_t),
        ("Unk44",          uint32_t),
        ("Unk48",          uint32_t),
        ("LoadBase",       uint32_t),
        ("Flags",          uint32_t),
        ("Unk54",          uint32_t),
        ("Unk58",          uint32_t),
        ("Unk5C",          uint32_t),
    ]
    udc_format = "<4s32s16sII"
    udc_length = 0x3c


class HuffmanLUTHeader(ctypes.LittleEndianStructure):
    _fields_ = [
        ("Tag",            char * 4),  # LLUT
        ("ChunkCount",     uint32_t),
        ("DecompBase",     uint32_t),
        ("Unk0C",          uint32_t),
        ("Size",           uint32_t),
        ("DataStart",      uint32_t),  # Start of data
        ("Unk18",          uint32_t * 6),
        ("ChunkSize",      uint32_t),
        ("Unk34",          uint32_t),
        ("Chipset",        char * 8),  # PCH
    ]


class HuffmanOffsetBytes(ctypes.LittleEndianStructure):
    _fields_ = [
        ("Offset", uint32_t, 24),
        ("Length", uint8_t),
    ]


class HuffmanOffsets(ctypes.Union):
    _fields_ = [
        ("b", HuffmanOffsetBytes),
        ("asword", uint32_t),
    ]


class MeManifestHeaderType(ctypes.LittleEndianStructure):
    _fields_ = [
        ("ModuleType",     uint16_t),  # 00
        ("ModuleSubType",  uint16_t),  # 02
        ("HeaderLen",      uint32_t),  # 04 in dwords
        ("HeaderVersion",  uint32_t),  # 08
        ("Flags",          uint32_t),  # 0C 0x80000000 = Debug
        ("ModuleVendor",   uint32_t),  # 10
        ("Date",           uint32_t),  # 14 BCD yyyy.mm.dd
        ("Size",           uint32_t),  # 18 in dwords
        ("Tag",            char * 4),  # 1C $MAN or $MN2
        ("NumModules",     uint32_t),  # 20
        ("MajorVersion",   uint16_t),  # 24
        ("MinorVersion",   uint16_t),  # 26
        ("HotfixVersion",  uint16_t),  # 28
        ("BuildVersion",   uint16_t),  # 2A
        ("Unknown1",       uint32_t * 19),  # 2C
        ("KeySize",        uint32_t),  # 78
        ("ScratchSize",    uint32_t),  # 7C
        ("RsaPubKey",      uint32_t * 64),  # 80
        ("RsaPubExp",      uint32_t),  # 180
        ("RsaSig",         uint32_t * 64),  # 184
        ("PartitionName",  char * 12),  # 284
        # 290
    ]


class MePartitionTable(ctypes.LittleEndianStructure):
    _fields_ = [
        ("_blank",          char * 16),
        ("Magic",           char * 4), # $FPT
        ("Entries",         uint32_t),
        ("Version",         uint8_t),
        ("Type",            uint8_t),
        ("Size",            uint8_t),
        ("Checksum",        uint8_t),
        ("LIFE",            uint16_t),
        ("LIM",             uint16_t),
        ("UMASize",         uint32_t),
        ("Flags",           uint32_t),
        ("Unknown1",         char * 8),
    ]


class MeFptEntryType(ctypes.LittleEndianStructure):
    _fields_ = [
        ("Name",            char * 4),  # 00 partition name
        ("Owner",           char * 4),  # 04 partition owner?
        ("Offset",          uint32_t),  # 08 from the start of FPT, or 0
        ("Size",            uint32_t),  # 0C
        ("TokensOnStart",   uint32_t),  # 10
        ("MaxTokens",       uint32_t),  # 14
        ("ScratchSectors",  uint32_t),  # 18
        ("Flags",           uint32_t),  # 1C
    ]


class AcManifestHeaderType(ctypes.LittleEndianStructure):
    _fields_ = [
        ("ModuleType",     uint16_t),  # 00
        ("ModuleSubType",  uint16_t),  # 02
        ("HeaderLen",      uint32_t),  # 04 in dwords
        ("HeaderVersion",  uint32_t),  # 08
        ("ChipsetID",      uint16_t),  # 0C
        ("Flags",          uint16_t),  # 0E 0x80000000 = Debug
        ("ModuleVendor",   uint32_t),  # 10
        ("Date",           uint32_t),  # 14 BCD yyyy.mm.dd
        ("Size",           uint32_t),  # 18 in dwords
        ("Reserved1",      uint32_t),  # 1C
        ("CodeControl",    uint32_t),  # 20
        ("ErrorEntryPoint", uint32_t),  # 24
        ("GDTLimit",       uint32_t),  # 28
        ("GDTBasePtr",     uint32_t),  # 2C
        ("SegSel",         uint32_t),  # 30
        ("EntryPoint",     uint32_t),  # 34
        ("Reserved2",      uint32_t * 16),  # 38
        ("KeySize",        uint32_t),  # 78
        ("ScratchSize",    uint32_t),  # 7C
        ("RsaPubKey",      uint32_t * 64),  # 80
        ("RsaPubExp",      uint32_t),  # 180
        ("RsaSig",         uint32_t * 64),  # 184
        # 284
    ]


class MeCpdEntryType(ctypes.LittleEndianStructure):
    size = 0x18

    _fields_ = [
        ("Name",            char * 12),
        ("Offset",          uint32_t),
        ("Size",            uint32_t),
        ("Flags",           uint32_t),
    ]


class MeCpdHeaderType(ctypes.LittleEndianStructure):
    size = 0x10

    _fields_ = [
        ("Tag",             char * 4),
        ("NumModules",      uint32_t),
        ("Flags",           uint32_t),
        ("PartitionName",   char * 4),
    ]
