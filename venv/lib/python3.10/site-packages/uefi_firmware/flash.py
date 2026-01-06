# -*- coding: utf-8 -*-

import os
import struct

from .base import FirmwareObject, BaseObject, StructuredObject
from .me import MeContainer
from .utils import *
from .structs.flash_structs import *


class RegionSection(StructuredObject):
    size = 20

    def __init__(self, data):
        self.parse_structure(data, FlashRegionSectionType)


class MasterSection(StructuredObject):
    size = 12

    def __init__(self, data):
        self.parse_structure(data, FlashMasterSectionType)


class DescriptorMap(StructuredObject):
    size = 16

    def __init__(self, data):
        self.parse_structure(data, FlashDescriptorMapType)


class FlashRegion(FirmwareObject, BaseObject):

    def __init__(self, data, region_name, region_details):
        self.sections = []
        self.data = data
        self.attrs = region_details
        self.name = region_name

    @property
    def objects(self):
        return self.sections

    def process(self):
        from .uefi import FirmwareVolume

        if self.name == "bios":
            data = self.data
            while True:
                volume_index = search_firmware_volumes(data, limit=1)
                if len(volume_index) == 0:
                    break
                fv = FirmwareVolume(data[volume_index[0] - 40:])
                if fv.valid_header:
                    self.sections.append(fv)
                    data = data[volume_index[0] - 40 + fv.size:]
                else:
                    data = data[volume_index[0] + 8:]
        if self.name == "me":
            data = self.data
            me = MeContainer(data)
            if me.valid_header:
                self.sections.append(me)
        for section in self.sections:
            section.process()
        return True

    def showinfo(self, ts='', index=None):
        print("%s%s type= %s, size= 0x%x (%d bytes) details[ %s ]" % (
            ts, blue("Flash Region"), green(self.name),
            len(self.data), len(self.data),
            ", ".join(["%s: 0x%02x" % (k, v) for k, v in list(self.attrs.items())])
        ))
        for section in self.sections:
            section.showinfo(ts="%s  " % ts)
        pass

    def dump(self, parent=""):
        dump_data(os.path.join(parent, "region-%s.fd" % self.name), self.data)

        parent = os.path.join(parent, "region-%s" % self.name)
        for section in self.sections:
            section.dump(parent)
        pass
    pass


class FlashDescriptor(FirmwareObject):

    def __init__(self, data):
        self.valid_header = False
        self.size = len(data)
        if self.size < 20:
            return

        self.padding, self.header = struct.unpack("<16s4s", data[:16 + 4])
        if self.header != FLASH_HEADER:
            return

        self.valid_header = True
        self.regions = []
        self.data = data

    @property
    def objects(self):
        return self.regions

    def process(self):
        def _region_size(base, limit):
            if limit:
                return (limit + 1 - base) * 0x1000
            return 0

        def _region_offset(base):
            return base * 0x1000

        self.map = DescriptorMap(self.data[20:20 + DescriptorMap.size])
        region_offset = (self.map.structure.RegionBase * 0x10)
        self.region = RegionSection(
            self.data[region_offset:region_offset + RegionSection.size])
        master_offset = (self.map.structure.MasterBase * 0x10)
        self.master = MasterSection(
            self.data[master_offset:master_offset + MasterSection.size])

        bios_base = self.region.structure.BiosBase
        bios_limit = self.region.structure.BiosLimit
        bios_size = _region_offset(
            bios_base) + _region_size(bios_base, bios_limit)
        bios = self.data[_region_offset(bios_base): bios_size]

        bios_region = FlashRegion(bios, "bios", {
            "base": bios_base,
            "limit": bios_limit,
            "id": self.master.structure.BiosId,
            "read": self.master.structure.BiosRead,
            "write": self.master.structure.BiosWrite
        })
        bios_region.process()
        self.regions.append(bios_region)

        me_base = self.region.structure.MeBase
        me_limit = self.region.structure.MeLimit
        me_size = _region_offset(me_base) + _region_size(me_base, me_limit)
        me = self.data[_region_offset(me_base): me_size]

        me_region = FlashRegion(me, "me", {
            "base": me_base,
            "limit": me_limit,
            "id": self.master.structure.MeId,
            "read": self.master.structure.MeRead,
            "write": self.master.structure.MeWrite
        })
        me_region.process()
        self.regions.append(me_region)

        gbe_base = self.region.structure.GbeBase
        gbe_limit = self.region.structure.GbeLimit
        gbe_size = _region_offset(gbe_base) + _region_size(gbe_base, gbe_limit)
        gbe = self.data[_region_offset(gbe_base): gbe_size]

        gbe_region = FlashRegion(gbe, "gbe", {
            "base": gbe_base,
            "limit": gbe_limit,
            "id": self.master.structure.GbeId,
            "read": self.master.structure.GbeRead,
            "write": self.master.structure.GbeWrite
        })
        gbe_region.process()
        self.regions.append(gbe_region)

        pdr_base = self.region.structure.PdrBase
        pdr_limit = self.region.structure.PdrLimit
        pdr_size = _region_offset(pdr_base) + _region_size(pdr_base, pdr_limit)
        pdr = self.data[_region_offset(pdr_base): pdr_size]

        pdr_region = FlashRegion(pdr, "pdr", {
            "base": pdr_base,
            "limit": pdr_limit,
        })
        pdr_region.process()
        self.regions.append(pdr_region)
        return True

    def showinfo(self, ts='', index=None):
        print("%s%s chips 0x%02x, regions 0x%02x, masters 0x%02x, PCH straps 0x%02x, "
                "PROC straps 0x%02x, ICC entries 0x%02x" % (
            ts, blue("Flash Descriptor (Intel PCH)"),
            self.map.structure.NumberOfFlashChips,
            self.map.structure.NumberOfRegions,
            self.map.structure.NumberOfMasters,
            self.map.structure.NumberOfPchStraps,
            self.map.structure.NumberOfProcStraps,
            self.map.structure.NumberOfIccTableEntries))
        for region in self.regions:
            region.showinfo(ts="%s  " % ts)

    def to_dict(self):
        res = []
        # TODO: ME, PSP, ...
        for region in self.regions:
            if region.name == "bios":
                fvs = []
                for s in region.sections:
                    fvs.append(s.to_dict())
                res.append({
                    'type': region.name,
                    'data': { 'firmwareVolumes': fvs },
                })
        return { 'regions': res }

    def dump(self, parent, index=None):
        dump_data(os.path.join(parent, "flash.fd"), self.data)

        parent = os.path.join(parent, "regions")
        for region in self.regions:
            region.dump(parent)

    pass
