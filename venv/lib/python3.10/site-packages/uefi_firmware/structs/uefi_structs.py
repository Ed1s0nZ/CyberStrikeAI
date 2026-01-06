# -*- coding: utf-8 -*-
import ctypes

uint8_t = ctypes.c_ubyte
char = ctypes.c_char
uint32_t = ctypes.c_uint
uint64_t = ctypes.c_uint64
uint16_t = ctypes.c_ushort
guid_t = char * 16

FIRMWARE_VOLUME_GUIDS = {
    "FFS1":        "7a9354d9-0468-444a-81ce-0bf617d890df",
    "FFS2":        "8c8ce578-8a3d-4f1c-9935-896185c32dd3",
    "FFS3":        "5473c07a-3dcb-4dca-bd6f-1e9689e7349a",
    "NVRAM_EVSA":  "fff12b8d-7696-4c8b-a985-2747075b4f50",
    "NVRAM_NVAR":  "cef5b9a3-476d-497f-9fdc-e98143e0422c",
    "NVRAM_EVSA2": "00504624-8a59-4eeb-bd0f-6b36e96128e0",
    "APPLE_BOOT":  "04adeead-61ff-4d31-b6ba-64f8bf901f5a",
    "PFH1":        "16b45da2-7d70-4aea-a58d-760e9ecb841d",
    "PFH2":        "e360bdba-c3ce-46be-8f37-b231e5cb9f35",
    #'08758b38-458d-50e8-56e8-3bffffff83c4'
}

FIRMWARE_CAPSULE_GUIDS = [
    '3b6686bd-0d76-4030-b70e-b5519e2fc5a0',  # EFI Capsule
    '4a3ca68b-7723-48fb-3d80-578cc1fec44d',  # EFI Capsule v2
    '539182b9-abb5-4391-b69a-e3a943f72fcc',  # UEFI Capsule
    '6dcbd5ed-e82d-4c44-bda1-7194199ad92a',  # Firmware Management Capsule
]

FIRMWARE_GUIDED_GUIDS = {
    "LZMA_COMPRESSED":    "ee4e5898-3914-4259-9d6e-dc7bd79403cf",
    "LZMA_COMPRESSED_HP": "0ed85e23-f253-413f-a03c-901987b04397",
    "TIANO_COMPRESSED":   "a31280ad-481e-41b6-95e8-127f4c984779",
    "FIRMWARE_VOLUME":    "24400798-3807-4a42-b413-a1ecee205dd8",
    #"VOLUME_SECTION":    "367ae684-335d-4671-a16d-899dbfea6b88",
    "STATIC_GUID":        "fc1bcdb0-7d31-49aa-936a-a4600d9dd083",
    "ZLIB_COMPRESSED_QC": "1d301fe9-be79-4353-91c2-d23bc959ae0c"
}

FIRMWARE_FREEFORM_GUIDS = {
    "CHAR_GUID": "059ef06e-c652-4a45-9fbe-5975e369461c"
}

EFI_FILE_TYPES = {
    # http://wiki.phoenix.com/wiki/index.php/EFI_FV_FILETYPE
    0x00: ("unknown",                    "none",        "0x00"),
    0x01: ("raw",                        "raw",         "RAW"),
    0x02: ("freeform",                   "freeform",    "FREEFORM"),
    0x03: ("security core",              "sec",         "SEC"),
    0x04: ("pei core",                   "pei.core",    "PEI_CORE"),
    0x05: ("dxe core",                   "dxe.core",    "DXE_CORE"),
    0x06: ("pei module",                 "peim",        "PEIM"),
    0x07: ("driver",                     "dxe",         "DRIVER"),
    0x08: ("combined pei module/driver", "peim.dxe",    "COMBO_PEIM_DRIVER"),
    0x09: ("application",                "app",         "APPLICATION"),
    0x0a: ("system management",          "smm",         "SMM"),
    0x0b: ("firmware volume image",      "vol",         "FV_IMAGE"),
    0x0c: ("combined smm/driver",        "smm.dxe",     "COMBO_SMM_DRIVER"),
    0x0d: ("smm core",                   "smm.core",    "SMM_CORE"),
    # 0xc0: ("oem min"),
    # 0xdf: ("oem max"),
    0xf0: ("ffs padding",                "pad",         "0xf0")
}

EFI_SECTION_TYPES = {
    0x01: ("Compression",               "compressed",   None),
    0x02: ("Guid Defined",              "guid",         None),
    0x03: ("Disposable",                "disposable",   None),
    0x10: ("PE32 image",                "pe",           "PE32"),
    0x11: ("PE32+ PIC image",           "pic.pe",       "PIC"),
    0x12: ("Terse executable (TE)",     "te",           "TE"),
    0x13: ("DXE dependency expression", "dxe.depex",    "DXE_DEPEX"),
    # Added from previous code (not in Phoenix spec
    0x14: ("Version section",           "version",      "VERSION"),
    0x15: ("User interface name",       "ui",           "UI"),

    0x16: ("IA-32 16-bit image",        "ia32.16bit",   "COMPAT16"),
    0x17: ("Firmware volume image",     "fv",           "FV_IMAGE"),
    # See FdfParser.py in EDKII's GenFds
    0x18: ("Free-form GUID",            "freeform.guid", "SUBTYPE_GUID"),
    0x19: ("Raw",                       "raw",          "RAW"),
    0x1b: ("PEI dependency expression", "pie.depex",    "PEI_DEPEX"),
    0x1c: ("SMM dependency expression", "smm.depex",    "SMM_DEPEX")
}

EFI_COMPRESSION_TYPES = {
    0x00: "",
    # 0x01: "EFI_STANDARD_COMPRESSION",
    0x01: "PI_STD",
    # 0x02: "EFI_CUSTOMIZED_COMPRESSION"
    0x02: "PI_STD"
}

NVRAM_ATTRIBUTES = {
    "RT":         0x01,
    "DESC_ASCII": 0x02,
    "GUID":       0x04,
    "DATA":       0x08,
    "EXTHDR":     0x10,
    "HER":        0x20,
    "AUTHWD":     0x40,
    "VLD":        0x80,
}


class UEFIVariableHeaderType(ctypes.LittleEndianStructure):
    _fields_ = [
        ("StartId", uint16_t),
        ("State", uint8_t),
        ("Reserved", uint8_t),
        ("Attributes", uint32_t),
        ("NameSize", uint32_t),
        ("DataSize", uint32_t),
        ("VendorGuid", guid_t),
    ]


class NVARVariableHeaderType(ctypes.LittleEndianStructure):
    _fields_ = [
        ("StartId", char * 4),  # NVAR
        ("TotalSize", uint16_t),
        ("Reserved", char * 3),
        ("Attributes", uint8_t),
    ]


class EFIVariableStoreType(ctypes.LittleEndianStructure):
    _fields_ = [
        ("Signature", uint32_t),
        ("Size", uint32_t),
        ("Format", uint8_t),
        ("State", uint8_t),
        ("Reserved", uint16_t),
        ("Reserved1", uint32_t),
    ]


class VSSHeaderType(ctypes.LittleEndianStructure):
    _fields_ = [
        ("VendorGuid", guid_t),
        ("StartId", uint16_t),
        ("State", uint8_t),
        ("Reserved", uint8_t),
        ("Attributes", uint32_t),
        ("NameSize", uint32_t),
        ("DataSize", uint32_t),
    ]


class VSSNewHeaderType(ctypes.LittleEndianStructure):
    _fields_ = [
        ("VendorGuid", guid_t),
        ("StartId", uint16_t),
        ("State", uint8_t),
        ("Reserved", uint8_t),
        ("Attributes", uint32_t),
        ("Unknown", char * 28),  # ???
        ("NameSize", uint32_t),
        ("DataSize", uint32_t),
    ]


class TLVHeaderType(ctypes.LittleEndianStructure):
    _fields_ = [
        ("Tag0", uint8_t),
        ("Tag1", uint8_t),
        ("Size", uint16_t),
    ]


class EVSARecordType(ctypes.LittleEndianStructure):
    _fields_ = [
        ("Signature", char * 4),
        ("Unknown", uint32_t),
        ("Length", uint32_t),
        ("Unknown1", uint32_t),
    ]


class GUIDRecordType(ctypes.LittleEndianStructure):
    _fields_ = [
        ("GuidId", uint16_t),
        ("Guid", guid_t),
    ]


class FirmwareVolumeType(ctypes.LittleEndianStructure):
    _fields_ = [
        ("Reserved",   char * 16),  # Zeros
        ("Guid",       guid_t),  #
        ("Size",       uint64_t),
        ("Magic",      char * 4),
        ("Attributes", uint8_t),
        ("HeaderSize", uint32_t),
        ("Checksum",   uint16_t),
        ("Reserved2",  uint8_t),
        ("Revision",   uint8_t)
    ]
