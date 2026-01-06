'''
  Modified version 2014-01-10 Teddy Reed
  Modified version 2013-12-29 Damien Zammit

  Based on the original work from:
    Intel ME ROM image dumper/extractor
    Copyright (c) 2012 Igor Skochinsky
    Version 0.1 2012-10-10
    Version 0.2 2013-08-15

    This software is provided 'as-is', without any express or implied warranty.
    In no event will the authors be held liable for any damages arising from the
    use of this software.

    Permission is granted to anyone to use this software for any purpose,
    including commercial applications, and to alter it and redistribute it
    freely, subject to the following restrictions:

    1. The origin of this software must not be misrepresented; you must not
    claim that you wrote the original software. If you use this software in a
    product, an acknowledgment in the product documentation would be appreciated
    but is not required.

    2. Altered source versions must be plainly marked as such, and must not be
    misrepresented as being the original software.

    3. This notice may not be removed or altered from any source distribution.
'''
from __future__ import print_function

import ctypes
import struct
import os
import array

from .structs.intel_me_structs import *
from .utils import *
from uefi_firmware import efi_compressor
from uefi_firmware.base import FirmwareObject, StructuredObject

MeModulePowerTypes = ["POWER_TYPE_RESERVED",
                      "POWER_TYPE_M0_ONLY", "POWER_TYPE_M3_ONLY", "POWER_TYPE_LIVE"]
MeCompressionTypes = ["COMP_TYPE_NOT_COMPRESSED",
                      "COMP_TYPE_HUFFMAN", "COMP_TYPE_LZMA", "<unknown>"]
COMP_TYPE_NOT_COMPRESSED = 0
COMP_TYPE_HUFFMAN = 1
COMP_TYPE_LZMA = 2
MeModuleTypes = ["DEFAULT", "PRE_ME_KERNEL",
                 "VENOM_TPM", "APPS_QST_DT", "APPS_AMT", "TEST"]
MeApiTypes = ["API_TYPE_DATA", "API_TYPE_ROMAPI",
              "API_TYPE_KERNEL", "<unknown>"]


class MeObject(StructuredObject, FirmwareObject):

    '''An ME Object is a combination of a parsing/extraction class and a ctype
    definding structure object.

    This follows the same ctor, process, showinfo, dump calling convention.
    '''

    def show_compression(self):
        if self.compression == COMP_TYPE_HUFFMAN:
            print(" (huffman)")
        elif self.compression == COMP_TYPE_LZMA:
            print(" (lzma)")
        else:
            print("")

    def dump_module(self, parent):
        if self.compression == COMP_TYPE_LZMA:
            dump_data("%s.module.lzma" %
                      os.path.join(parent, self.name), self.data)
            try:
                data = efi_compressor.LzmaDecompress(self.data, len(self.data))
                dump_data("%s.module" % os.path.join(parent, self.name), data)
            except Exception as e:
                print("Cannot extract (%s), %s" % (self.name, str(e)))
                return
        elif self.compression == COMP_TYPE_NOT_COMPRESSED:
            dump_data("%s.module" % os.path.join(parent, self.name), self.data)


class MeModule(MeObject):

    def __init__(self, data, structure_type, offset):
        self.attrs = {}
        self.parse_structure(data, structure_type)
        self.structure_type = structure_type

        if structure_type == MeModuleHeader2Type:
            self.structure.Guid = "(none)"
            self.attrs["version"] = "0.0.0.0"
        elif structure_type == MeModuleHeader1Type:
            self.attrs["version"] = "%d.%d.%d.%d" % (
                self.structure.MajorVersion, self.structure.MinorVersion,
                self.structure.HotfixVersion,
                self.structure.BuildVersion
            )

        self.guid = self.structure.Guid
        self.name = self.structure.Name
        self.tag = self.structure.Tag

        self.attrs["module_size"] = self.structure.Size
        if structure_type == MeModuleHeader1Type:
            self.attrs["load_base"] = 0
            self.offset = offset
        elif structure_type == MeModuleHeader2Type:
            self.attrs["load_base"] = self.structure.LoadBase
            # Must know the offset from given data (the start of the header) to find data
            self.offset = self.structure.Offset - offset

        self.attrs["flags"] = self.structure.Flags
        if structure_type == MeModuleHeader2Type:
            self.attrs["power_type"] = (
                self.structure.Flags >> 1) & 3  # MeModulePowerTypes
            self.attrs["compression"] = (
                self.structure.Flags >> 4) & 7  # MeCompressionTypes
            # MeModuleTypes (optional)
            self.attrs["module_stage"] = (self.structure.Flags >> 7) & 0xF
            self.attrs["api_type"] = (
                self.structure.Flags >> 11) & 7  # MeApiTypes
            self.attrs["privileged"] = ((self.structure.Flags >> 16) & 1)
        # There are unknown flags to parse, todo: revisit
        pass

        # print "Debug: module data 0x%08X - 0x%08X" % (
        #    self.offset, self.offset + self.structure.Size)
        self.data = data[self.offset:self.offset + self.structure.Size]

    def process(self):
        if self.compression == COMP_TYPE_HUFFMAN:
            # The individual modules are compressed together in a partition
            # chunk
            return True

        if self.structure_type == MeModuleHeader1Type:
            # It's possible for type 1 to include LZMA compression
            if self.data[0x50:0x55] == b'\x5D\x00\x00\x80\x00':
                raw_data = self.data[0x50:0x55]
                raw_data += struct.pack("<Q", self.structure.UncompressedSize)
                raw_data += self.data[0x55:]
                self.data = raw_data
        return True

    @property
    def size(self):
        return ctypes.sizeof(self.structure_type)

    @property
    def compression(self):
        if self.structure_type == MeModuleHeader1Type:
            return COMP_TYPE_NOT_COMPRESSED
        else:
            return (self.structure.Flags >> 4) & 7

    def showinfo(self, ts='', index=None):
        guid = self.guid
        if self.guid != "(none)":
            guid = green(sguid(self.guid))
        print("%s%s name= %s, guid= %s, version= %s, size= %s" % (
            ts, blue("ME Module"),
            purple(self.name),
            guid, self.attrs["version"], self.attrs["module_size"]), end=' ')
        self.show_compression()

    def dump(self, parent=""):
        self.dump_module(parent)


class MeVariableModule(MeObject):
    HEADER_SIZE = 8

    def __init__(self, data, structure_type):
        self.update = {}
        self.type = structure_type
        self.data = None
        self.size = 0

        hdr = data[:self.HEADER_SIZE]
        self.valid_header = True
        self.header_blank = False

        if hdr == b'\xFF' * self.HEADER_SIZE:
            self.header_blank = True
            return

        if len(hdr) < self.HEADER_SIZE or hdr[0:1] != b'$':
            # print "Debug: invalid module header."
            self.valid_header = False
            return

        self.tag = hdr[:4]
        # Note the elen size includes the header size
        self.size = struct.unpack("<I", hdr[4:])[0] * 4 - self.HEADER_SIZE
        self.data = data[self.HEADER_SIZE:self.HEADER_SIZE + self.size]
        pass

    def add_update(self, tag, name, offset, size):
        self.update["tag"] = tag
        self.update["name"] = name
        self.update["offset"] = offset
        self.update["size"] = size
        pass

    def process(self):
        if self.tag == b'$UDC':
            subtag, _hash, name, offset, size = struct.unpack(
                self.type.udc_format, self.data[:self.type.udc_length])
            self.add_update(subtag, name, offset, size)
        if self.tag in [b'$SKU', b'$UVR']:
            # SKU is not handled
            self.values = [0, 0]
            return True
        if self.size == 3:
            values = [struct.unpack("<I", self.data[:4])[0]]
        if self.size == 4:
            values = struct.unpack("<II", self.data[:8])[0]
        else:
            values = array.array("I", self.data)

        self.values = values
        return True

    def showinfo(self, ts='', index=None):
        print("%s%s tag= %s, size= %d" % (
            ts, blue("VModule"), purple(self.tag), self.size))
        if self.tag == b'$UDC':
            print("%s%s name= %s, offset= %d, size= %s" % (
                ts, blue("%s Update" % self.update["tag"]),
                purple(self.update["name"]),
                self.update["offset"], self.update["size"]))
        pass


class MeModuleFile(MeObject):

    def __init__(self, data):
        self.size = 0
        tag = data[:4]

        self.valid_header = True
        if tag != b"$MOD":
            self.valid_header = False
            return

        self.parse_structure(data, MeModuleFileHeader1Type)
        self.name = self.structure.Name.rstrip('\0')
        self.size = self.structure.Size
        pass


class MeLLUT(MeObject):

    def __init__(self, data, relative_offset):
        #self.tag = data[:4]

        self.valid_header = True
        if data[:4] != b'LLUT':
            self.valid_header = False

        #hdr = data[4:52]
        # chunkcount, decompbase, unk0c, size, start, a,b,c,d,e,f, chunksize = struct.unpack(
        #    "<IIIIIIIIIIII", hdr)
        self.parse_structure(data, HuffmanLUTHeader)
        self.size = self.structure.Size

        # The start and end addresses are relative to the manifest.
        # The relative offset references the start of the manifest data (not
        # header).
        self.start = self.structure.DataStart
        self.offset = relative_offset

        self.chunkcount = self.structure.ChunkCount
        self.chunksize = self.structure.ChunkSize
        self.decompression_base = self.structure.DecompBase

        self.data = data[
            self.start - relative_offset:self.start - relative_offset + self.size]
        # The huffman look up table is stored following the header data.
        self.lut_data = data[
            self.structure_size:self.structure_size + self.chunkcount * 4]

    def showinfo(self, ts='', index=None):
        print("%s%s chunks= %d, chunk size= %d, start= %d, size= %d, base= 0x%08X" % (
            ts, blue("LLUT"),
            self.chunkcount, self.chunksize, self.start, self.size, self.decompression_base))

    def dump(self, parent='PART'):
        # print "Debug: relative (%d) absolute start (%d) len (%d)." % (
        #    self.offset, self.start, len(self.data))
        dump_data("%s.llut.table" % parent, self.lut_data)
        dump_data("%s.llut.compressed" % parent, self.data)


class MeManifestHeader(MeObject):
    _DATA_OFFSET = 12

    def __init__(self, data, container_offset=0):
        self.attrs = {}

        self.valid_header = True
        if data[:8] != b"\x04\x00\x00\x00\xA1\x00\x00\x00":
            self.valid_header = False
            return

        self.parse_structure(data, MeManifestHeaderType)
        # Save the container offset as LLUT start is an absolute reference
        self.container_offset = container_offset
        self.data = data[self.partition_offset:]

        '''Set storage attributes.'''
        self.attrs["header_version"] = "%d.%d" % (
            self.structure.HeaderVersion >> 16, self.structure.HeaderVersion & 0xFFFF)
        self.attrs["version"] = "%d.%d.%d.%d" % (
            self.structure.MajorVersion,
            self.structure.MinorVersion,
            self.structure.HotfixVersion,
            self.structure.BuildVersion
        )
        self.attrs["flags"] = "0x%08X" % (self.structure.Flags)
        self.attrs["module_vendor"] = "0x%04X" % (self.structure.ModuleVendor)
        self.attrs["date"] = "%08X" % (self.structure.Date)
        self.structure_size = self.structure.Size
        self.size = len(data)

        '''Skipped.'''
        #ModuleType, ModuleSubType, size, tag, num_modules, keysize, scratchsize, rsa

        self.partition_name = self.structure.PartitionName.rstrip(b"\0")
        if not self.partition_name:
            self.partition_name = "(none)"

        self.modules = []
        self.variable_modules = []
        self.partition_end = 0
        self.huffman_llut = None

    @property
    def absolute_offset(self):
        return self.structure.HeaderLen * 4 + self._DATA_OFFSET + self.container_offset

    @property
    def partition_offset(self):
        return self.structure.HeaderLen * 4 + self._DATA_OFFSET

    @property
    def objects(self):
        _objects = self.modules + self.variable_modules
        if self.huffman_llut is not None:
            _objects.append(self.huffman_llut)
        return _objects

    def showinfo(self, ts='', index=None):
        print("%s%s type= %d, subtype= %d, partition name= %s" % (
            ts, blue("ME Module Manifest"),
            self.structure.ModuleType, self.structure.ModuleSubType,
            purple(self.structure.PartitionName.decode("utf-8"))))
        for module in self.modules:
            module.showinfo(ts="  %s" % ts)
        for module in self.variable_modules:
            module.showinfo(ts="  %s" % ts)
        if self.huffman_llut is not None:
            self.huffman_llut.showinfo(ts="  %s" % ts)

    def _parse_mods(self):
        # Parse the module headers (two types of headers, specified by the
        # manifest).
        module_offset = 0
        huffman_offset = 0
        for module_index in range(self.structure.NumModules):
            module = MeModule(
                self.data[module_offset:],
                self.header_type, module_offset + self.partition_offset)
            # print "Debug: found me module header (%s) at (%d)." %
            # (module.tag, module_offset)
            if module.compression == COMP_TYPE_HUFFMAN:
                # Todo: skipped precondition for huffman offsets.
                # print "Debug: Setting huffman offset: %d" %
                # module.structure.Offset
                huffman_offset = module.structure.Offset
            if not module.process():
                return False
            self.modules.append(module)
            self.module_map[module.name] = module
            module_offset += module.size

        #additional_header = self.structure.Size*4 - module_offset
        # print "Debug: Remaining header: %d - %d = %d" % (
        #    self.structure.Size*4, module_offset, additional_header)

        self.module_offset = module_offset
        self.huffman_offset = huffman_offset
        return True

    def _parse_variable_mods(self, module_offset):
        # Parse additional tagged modules.
        self.partition_end = 0
        while module_offset < self.structure.Size * 4:
            # There is more module header to process.
            module = MeVariableModule(
                self.data[module_offset:], self.header_type)
            if not module.valid_header:
                break
            module_offset += module.HEADER_SIZE
            if module.header_blank:
                continue

            # print("Debug: found module (%s) size (%d)." % (module.tag,
            # module.size))
            if not module.process():
                return False
            if module.tag == b'$MCP':
                # The end of a manifest partition is stored in MCP
                self.partition_end = module.values[0] + module.values[1]
            self.variable_modules.append(module)
            module_offset += module.size
        return True

    def _parse_module_files(self):
        file_offset = self.structure.Size * 4
        # print "Debug: looking for module files at (%08X)." % file_offset
        while True:
            module_file = MeModuleFile(self.data[file_offset:])
            if not module_file.valid_header:
                break
            if module_file.name in module_map:
                # A module file header corresponds to the module header
                self.module_map[module_file.name].file = module_file
            # print "Debug: found module file (%s) size (%d)." %
            # (module_file.name, module_file.size)
            file_offset += module_file.size
        return True

    def process(self):
        self.modules = []
        self.variable_modules = []
        self.module_map = {}

        if self.structure.Tag == b'$MN2':
            self.header_type = MeModuleHeader2Type
        elif self.structure.Tag == b'$MAN':
            self.header_type = MeModuleHeader1Type
        else:
            # Cannot parse modules...
            return False

        # Parse the module headers (two types of headers, specified by the
        # manifest).
        if not self._parse_mods():
            return False
        if self.header_type == MeModuleHeader2Type:
            if not self._parse_variable_mods(self.module_offset):
                return False
        if not self._parse_module_files():
            return False

        # Parse optional huffman LLUT.
        huffman_offset = self.huffman_offset - self.partition_offset
        huffman_llut = MeLLUT(
            self.data[huffman_offset:], huffman_offset + self.absolute_offset)
        if huffman_llut.valid_header:
            self.huffman_llut = huffman_llut
        #    print "Debug: huffman LLUT start (0x%08X) end (0x%08X)." % (
        #        huffman_llut.offset, huffman_llut.size + huffman_llut.start)
        # print "Debug: LLUT end (%08X) partition end (%08X)." % (
        #    huffman_llut.size + huffman_offset, self.partition_end)
        return True

    def dump(self, parent=""):
        #huffman_end = self.huffman_llut.size + self.huffman_llut.start
        for module in self.modules:
            # if module.compression == COMP_TYPE_HUFFMAN:
                # print "Huffman module data: %r %08X/%08X" % (
                #    module.name, self.huffman_llut.start, self.huffman_llut.size)
            # else:
                #huffman_end = (min(huffman_end, module.structure.Offset))
                # print "Debug: decrementing huffman to %d" % huffman_end
            module.dump(parent)
        if self.huffman_llut is not None:
            self.huffman_llut.dump(
                os.path.join(parent, self.structure.PartitionName.decode("utf-8")))


class CPDEntry(MeObject):

    def __init__(self, data, header_offset):
        self.parse_structure(data[header_offset:], MeCpdEntryType)
        self.valid_header = False
        if self.structure.Offset > len(data):
            # This is invalid, offset (start) out of bounds
            return
        end = self.structure.Offset + self.structure.Size
        if end > len(data):
            # This is invalid, end of data out of bounds
            return
        self.valid_header = True
        self.data = data[self.structure.Offset:end]

    def process(self):
        if not self.valid_header:
            return False
        self.name = self.structure.Name.rstrip(b'\0')

        # Not sure why the placement of data determines compression type.
        compression = self.structure.Offset >> 24
        if self.name.find(".met") > 0:
            self.compression = COMP_TYPE_NOT_COMPRESSED
        elif compression == 0x02:
            self.compression = COMP_TYPE_HUFFMAN
        elif compression == 0x00:
            self.compression = COMP_TYPE_LZMA
        else:
            self.compression = COMP_TYPE_NOT_COMPRESSED
        return True

    def showinfo(self, ts='', index=None):
        print("%s%s name= %s offset= 0x%x size= 0x%x (%d bytes) flags= 0x%x" % (
            ts, blue("ME CDP Entry"), purple(self.name),
            self.structure.Offset, self.structure.Size, self.structure.Size,
            self.structure.Flags), end=' ')
        self.show_compression()

    def dump(self, parent):
        if self.compression == COMP_TYPE_LZMA:
            # There is an odd state to check for that includes an additional
            # \x00\x00\x00 after the initial LZMA header block.
            if self.data[0x0e:0x11] == b'\x00\x00\x00':
                self.data = self.data[:0x0e] + self.data[0x11:]
        self.dump_module(parent)


class CPDManifestHeader(MeObject):

    def __init__(self, data, container_offset=0):
        self.valid_header = True
        self.parse_structure(data, MeCpdHeaderType)

        # Save the container offset as LLUT start is an absolute reference
        self.container_offset = container_offset
        self.data = data

        self.partition_name = self.structure.PartitionName.rstrip(b"\0")
        if not self.partition_name:
            self.partition_name = "(none)"

        self.modules = []

    @property
    def objects(self):
        return self.modules

    def process(self):
        offset = MeCpdHeaderType.size
        for i in range(self.structure.NumModules - 1):
            offset += MeCpdEntryType.size
            entry = CPDEntry(self.data, offset)
            if entry.process():
                self.modules.append(entry)
        return True

    def showinfo(self, ts='', index=None):
        print("%s%s name= %s modules= %d flags= 0x%x" % (
            ts, blue("ME CDP Entry"), purple(self.partition_name),
            self.structure.NumModules, self.structure.Flags))
        for entry in self.modules:
            entry.showinfo("%s  " % ts)

    def dump(self, parent):
        for entry in self.modules:
            entry.dump(parent)


class PartitionEntry(MeObject):
    size = 0x20

    def __init__(self, data, offset):
        self.manifest = None
        self.parse_structure(data[offset:], MeFptEntryType)

        self.has_content = True
        if self.structure.Owner == b"\xFF\xFF\xFF\xFF":
            # A blank owner is filled in with 0xFF.
            self.structure.Owner = b''
        if self.structure.Offset == 0xFFFFFFFF:
            # A (blank) offset usually means flags = 0x02.
            self.has_content = False
            return

        # Set the partition data based on an offset and size determined within
        # the partition entry metadata.
        if self.structure.Offset > len(data):
            # This partition is invalid
            self.has_content = False

        partition_end = self.structure.Offset + self.structure.Size
        if partition_end > len(data):
            # This partition is invalid
            self.has_content = False
        self.data = data[self.structure.Offset:partition_end]
        self.size = len(data)

    @property
    def objects(self):
        if self.manifest is not None:
            return [self.manifest]
        return []

    def process(self):
        if not self.has_content:
            return True
        if self.data[0:0x04] == b'$CPD':
            manifest = CPDManifestHeader(self.data, self.structure.Offset)
        else:
            manifest = MeManifestHeader(self.data, self.structure.Offset)
        if manifest.valid_header:
            if manifest.process():
                self.manifest = manifest
        return True

    def showinfo(self, ts='', index=None):
        print("%s%s name= %s owner= %s offset= 0x%x size= 0x%x (%d bytes) flags= 0x%x" % (
            ts, blue("ME Partition Entry"),
            purple(self.structure.Name.decode("utf-8")), purple(self.structure.Owner),
            self.structure.Offset, self.structure.Size, self.structure.Size, self.structure.Flags))
        if self.manifest is not None:
            self.manifest.showinfo("%s  " % ts)

    def dump(self, parent=""):
        if self.has_content:
            dump_data(os.path.join(parent, "%s.partition" % self.structure.Name.decode("utf-8")),
                self.data)
        if self.manifest is not None:
            self.manifest.dump(os.path.join(parent, self.structure.Name.decode("utf-8")))

class MeContainer(MeObject):

    def __init__(self, data):
        self.partitions = []
        self.data = data
        self.size = len(ME_HEADER)

        self.valid_header = False
        if data[0x0:len(ME_HEADER)] == ME_HEADER:
            self.partition_offset = 0x00
            self.valid_header = True
        if data[0x10:0x14] == ME_PARTITION_HEADER:
            self.partition_offset = 0x00
            self.valid_header = True

    @property
    def objects(self):
        return self.partitions

    def process(self):
        self.parse_structure(self.data, MePartitionTable)

        for i in range(self.structure.Entries):
            offset = self.partition_offset + 0x30
            offset += i * PartitionEntry.size
            entry = PartitionEntry(self.data, offset)
            if entry.process():
                self.size += entry.size
                self.partitions.append(entry)
        return True

    def showinfo(self, ts='', index=None):
        print("%s%s type= 0x%x version= 0x%x size= 0x%x (%d bytes) entires= %d flags= 0x%x" % (
            ts, blue("ME Container"),
            self.structure.Type, self.structure.Version,
            self.structure.Size, self.structure.Size,
            self.structure.Entries, self.structure.Flags))
        for partition in self.partitions:
            partition.showinfo("  %s" % ts)

    def dump(self, parent=""):
        dump_data(os.path.join(parent, "me-container.me"), self.data)
        for partition in self.partitions:
            partition.dump(os.path.join(parent, "partitions"))
