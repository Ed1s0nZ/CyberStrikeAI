from __future__ import annotations

import logging
import struct

from cle.address_translator import AT

from .pereloc import PEReloc

log = logging.getLogger(name=__name__)

__all__ = [
    "DllImport",
    "IMAGE_REL_BASED_ABSOLUTE",
    "IMAGE_REL_BASED_HIGHADJ",
    "IMAGE_REL_BASED_HIGHLOW",
    "IMAGE_REL_BASED_DIR64",
    "IMAGE_REL_BASED_HIGH",
    "IMAGE_REL_BASED_LOW",
]


class DllImport(PEReloc):
    """
    There's nothing special to be done for DLL imports but this class
    provides a unique name to the relocation type.
    """

    pass


class IMAGE_REL_BASED_ABSOLUTE(PEReloc):
    def relocate(self):
        pass


class IMAGE_REL_BASED_HIGHADJ(PEReloc):
    def __init__(self, owner, addr, next_rva):
        super().__init__(owner, None, addr)
        self.next_rva = next_rva

    @property
    def value(self):
        """
        In all the other cases, we can ignore the relocation difference part of the
        calculation because we simply use to_mva() to get our rebased address. In this
        case, however, we have to adjust the un-rebased address first.
        """
        org_bytes = self.owner.memory.load(self.relative_addr, 2)
        org_value = struct.unpack("<H", org_bytes)[0]
        adjusted_value = (org_value << 16) + self.next_rva
        adjusted_value = (AT.from_lva(adjusted_value, self.owner).to_mva() & 0xFFFF0000) >> 16
        adjusted_bytes = struct.pack("<I", adjusted_value)
        return adjusted_bytes


class IMAGE_REL_BASED_HIGHLOW(PEReloc):
    @property
    def value(self):
        org_bytes = self.owner.memory.load(self.relative_addr, 4)
        org_value = struct.unpack("<I", org_bytes)[0]
        rebased_value = AT.from_lva(org_value, self.owner).to_mva()
        rebased_bytes = struct.pack("<I", rebased_value & 0xFFFFFFFF)
        return rebased_bytes


class IMAGE_REL_BASED_DIR64(PEReloc):
    @property
    def value(self):
        org_bytes = self.owner.memory.load(self.relative_addr, 8)
        org_value = struct.unpack("<Q", org_bytes)[0]
        rebased_value = AT.from_lva(org_value, self.owner).to_mva()
        if rebased_value < 0 or rebased_value >= 0x10000000000000000:
            log.error(
                "Incorrect rebased address %x found at %s. Maybe the relocation table is broken.",
                rebased_value,
                self.owner,
            )
            return None
        rebased_bytes = struct.pack("<Q", rebased_value)
        return rebased_bytes


class IMAGE_REL_BASED_HIGH(PEReloc):
    @property
    def value(self):
        org_bytes = self.owner.memory.load(self.relative_addr, 2)
        org_value = struct.unpack("<H", org_bytes)[0]
        rebased_value = AT.from_lva(org_value, self.owner).to_mva()
        adjusted_value = (rebased_value >> 16) & 0xFFFF
        adjusted_bytes = struct.pack("<H", adjusted_value)
        return adjusted_bytes


class IMAGE_REL_BASED_LOW(PEReloc):
    @property
    def value(self):
        org_bytes = self.owner.memory.load(self.relative_addr, 2)
        org_value = struct.unpack("<H", org_bytes)[0]
        rebased_value = AT.from_lva(org_value, self.owner).to_mva()
        adjusted_value = rebased_value & 0x0000FFFF
        adjusted_bytes = struct.pack("<H", adjusted_value)
        return adjusted_bytes


relocation_table_generic = {
    1: IMAGE_REL_BASED_HIGH,
    2: IMAGE_REL_BASED_LOW,
    3: IMAGE_REL_BASED_HIGHLOW,
    4: IMAGE_REL_BASED_HIGHADJ,
    10: IMAGE_REL_BASED_DIR64,
}
