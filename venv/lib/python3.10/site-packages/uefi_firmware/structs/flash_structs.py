import ctypes

FLASH_HEADER = b"\x5A\xA5\xF0\x0F"

uint8_t = ctypes.c_ubyte
char = ctypes.c_char
uint32_t = ctypes.c_uint
uint64_t = ctypes.c_uint64
uint16_t = ctypes.c_ushort


class FlashDescriptorMapType(ctypes.LittleEndianStructure):
    _fields_ = [
        ("ComponentBase",       uint8_t),  #
        ("NumberOfFlashChips",  uint8_t),  #
        ("RegionBase",          uint8_t),  #
        ("NumberOfRegions",     uint8_t),  #
        ("MasterBase",          uint8_t),  #
        ("NumberOfMasters",     uint8_t),  #
        ("PchStrapsBase",       uint8_t),  #
        ("NumberOfPchStraps",   uint8_t),  #
        ("ProcStrapsBase",      uint8_t),  #
        ("NumberOfProcStraps",  uint8_t),  #
        ("IccTableBase",        uint8_t),  #
        ("NumberOfIccTableEntries", uint8_t),  #
        ("DmiTableBase",            uint8_t),  #
        ("NumberOfDmiTableEntries", uint8_t),  #
        ("ReservedZero",            uint16_t),  #
    ]


class FlashMasterSectionType(ctypes.LittleEndianStructure):
    _fields_ = [
        ("BiosId",    uint16_t),  #
        ("BiosRead",  uint8_t),   #
        ("BiosWrite", uint8_t),   #
        ("MeId",      uint16_t),  #
        ("MeRead",    uint8_t),   #
        ("MeWrite",   uint8_t),   #
        ("GbeId",     uint16_t),  #
        ("GbeRead",   uint8_t),   #
        ("GbeWrite",  uint8_t),   #
    ]


class FlashRegionSectionType(ctypes.LittleEndianStructure):
    _fields_ = [
        ("ReservedZero",        uint16_t),  #
        ("FlashBlockEraseSize", uint16_t),  #
        ("BiosBase",            uint16_t),  #
        ("BiosLimit",           uint16_t),  #
        ("MeBase",              uint16_t),  #
        ("MeLimit",             uint16_t),  #
        ("GbeBase",             uint16_t),  #
        ("GbeLimit",            uint16_t),  #
        ("PdrBase",             uint16_t),  #
        ("PdrLimit",            uint16_t),  #
    ]
