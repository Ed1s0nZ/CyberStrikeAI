'''EFI and UEFI related structures.
This package defines firmware structures for unpacking, decompressing,
extracting, and rebuilding UEFI data.
'''
from __future__ import print_function

import os
import struct
import logging
import zlib

from .base import FirmwareObject, StructuredObject, RawObject, AutoRawObject
from .utils import *
from .guids import get_guid_name
from .structs.uefi_structs import *
from .structs.flash_structs import *

from uefi_firmware import efi_compressor

def parse_depex(input_data):
    depex = []
    offset = 0
    while offset < len(input_data):
        opcode = ord(input_data[offset:offset+1])
        offset = offset + 1
        if opcode == 0x00:
            guid = input_data[offset:offset+16]
            guid_name = get_guid_name(guid)
            offset = offset + 16
            depex.append({
                'op': "BEFORE",
                'name': guid_name,
                'guid': sguid(guid),
            })
        elif opcode == 0x01:
            guid = input_data[offset:offset+16]
            guid_name = get_guid_name(guid)
            offset = offset + 16
            depex.append({
                'op': "AFTER",
                'name': guid_name,
                'guid': sguid(guid),
            })
        elif opcode == 0x02:
            guid = input_data[offset:offset+16]
            guid_name = get_guid_name(guid)
            offset = offset + 16
            depex.append({
                'op': "PUSH",
                'name': guid_name,
                'guid': sguid(guid),
            })
        elif opcode == 0x03:
            depex.append({ 'op': "AND" })
        elif opcode == 0x04:
            depex.append({ 'op': "OR" })
        elif opcode == 0x05:
            depex.append({ 'op': "NOT" })
        elif opcode == 0x06:
            depex.append({ 'op': "TRUE" })
        elif opcode == 0x07:
            depex.append({ 'op': "FALSE" })
        elif opcode == 0x08:
            depex.append({ 'op': "END" })
        elif opcode == 0x09:
            depex.append({ 'op': "SOR" })
        else:
            depex.append({ 'op': opcode })

    return depex

def dlog(kls, name, msg=""):
    logging.info("%s %s %s" % (str(kls.__class__.__name__), name, msg))


def _get_file_type(file_type):
    return EFI_FILE_TYPES[file_type] if file_type in EFI_FILE_TYPES else (
        "unknown", "unknown")


def _get_section_type(section_type):
    if section_type in EFI_SECTION_TYPES:
        return EFI_SECTION_TYPES[section_type]
    else:
        return ("unknown", "unknown.bin")


def uefi_name(s):
    '''Return the utf-16le encoded string name for a UEFIFile.'''
    try:
        name = s.decode("utf-16le").split("\0")[0]
        if len(name) == 0:
            return None
        for c in name:
            if ord(c) > 128:
                return None
        return name
    except Exception:
        return None


def compare(data1, data2):
    from hashlib import md5
    md5_1 = md5(data1).hexdigest()
    md5_2 = md5(data2).hexdigest()
    if (md5_1 != md5_2):
        print ("%s != %s" % (red(md5_1), red(md5_2)))
        return False
    return True


def decompress(algorithms, compressed_data):
    '''Attempt to decompress using a set of algorithms.

    Args:
        algorithms (list): A set of decompression methods.
        compressed_data (binary): A compressed data stream.

    Return:
        pair (int, binary): Return the algorithm index, and decompressed stream.
    '''
    for i, algorithm in enumerate(algorithms):
        try:
            data = algorithm(compressed_data, len(compressed_data))
            if data:
                return (i, data)
            else:
                raise Exception
        except Exception:
            continue
    return None


def find_volumes(data, process=True):
    '''Search for arbitary firmware volumes within data.

    This is helpful within Raw files and sections.

    Some firmware vendors will implement custom checksums or metadata within
    UEFIFiles. It is often helpful to 'lose' this information and continue to
    discovery volumnes and other structures. In these cases it is ultimately
    helpful if folks/contributors will add support for representing each
    proprietary structure.

    Args:
        process (Optional[bool]): Call process on each discovered volumn.

    Return:
        list: The set of discovered firmware objects.
    '''
    objects = []
    while True:
        volume_index = data.find(b"_FVH")
        if volume_index < 0:
            break
        volume_index -= (8 + 16 * 2)
        fv = FirmwareVolume(data[volume_index:])
        if not fv.valid_header:
            data = data[16 * 3:]
            continue
        if volume_index > 0:
            objects.append(RawObject(data[:volume_index]))
        if process:
            fv.process()
        objects.append(fv)
        data = data[volume_index + fv.size:]
    if len(data) > 0:
        objects.append(RawObject(data))
    return objects


class FirmwareVariableStore(FirmwareObject, StructuredObject):

    '''An firmware-related variable storage structure (think NVRAM).'''
    variables = []

    @property
    def objects(self):
        return self.variables


class FirmwareVariable(FirmwareObject, StructuredObject):

    '''A firmware-related variable, found in a variable store.'''
    subsections = []

    @property
    def objects(self):
        return self.subsections


class NVARVariable(FirmwareVariable):

    @classmethod
    def valid_nvar(cls, data):
        if data[:4] != b"NVAR":
            return False
        return True

    def _get_name(self, data, is_ascii=False):
        tail = b"\x00" if is_ascii else b"\x00\x00"
        size = data.find(tail)
        if not is_ascii:
            name = uefi_name(data[:size])
        else:
            name = data[:size]
        return (name, size + len(tail))

    def __init__(self, data):
        self.subsections = []
        self.name = None
        self.guid = None
        self.data = data

    def process(self):
        dlog(self, 'NVAR')
        if not NVARVariable.valid_nvar(self.data):
            return False
        self.parse_structure(self.data, NVARVariableHeaderType)
        self.size = self.structure.TotalSize
        self.attrs = {"attrs": self.structure.Attributes}

        # Now with structure parsed, set bounds on the data
        self.data = self.data[:self.size]
        offset = self.structure_size
        if bit_set(self.structure.Attributes, NVRAM_ATTRIBUTES["GUID"]):
            self.guid = self.data[offset:offset + 16]
            offset += 16
        else:
            # Increment data by 1!
            offset += 1

        if bit_set(self.structure.Attributes, NVRAM_ATTRIBUTES["DATA"]):
            self.data_offset = offset
            # self.subsections.append(RawObject(self.data[offset:]))
            return True

        # Parse variable name.
        var_name, var_name_size = self._get_name(
            self.data[offset:],
            bit_set(self.structure.Attributes, NVRAM_ATTRIBUTES["DESC_ASCII"])
        )
        if var_name is not None:
            self.name = var_name
            offset += var_name_size

        # The variable's meta-data is the value.
        self.data_offset = offset

        # if self.data_offset+1 < self.size:
        #    self.subsections.append(RawObject(self.data[offset:]))
        return True

    def build(self, generate_checksum=False, debug=False):
        header = self.structure_data
        data = b""
        for section in self.subsections:
            data += section.build(generate_checksum, debug)
        if len(self.subsections) == 0:
            data = self.data[self.data_offset:]
        # Metadata includes optional guid/name.
        meta_data = self.data[self.structure_size:self.data_offset]
        return header + meta_data + data

    def dump(self, parent, index=0):
        path = os.path.join(parent, "variable%d.nvar" % index)
        dump_data(path, self.data)
        for i, section in enumerate(self.subsections):
            section.dump(os.path.join(parent, "variable%d-data" % index), i)

    def showinfo(self, ts="", index=0):
        '''Potential for A LOT of variables.'''
        if self.guid is not None and self.name is not None:
            print ("%s %s %s %s" % (
                blue("%sVariable:" % ts),
                green(sguid(self.guid)),
                purple(self.name),
                "attrs= %s" % self.attrs["attrs"]
            ))

    def to_dict(self):
        if self.guid is not None and self.name is not None:
            return {
                'guid': sguid(self.guid),
                'name': self.name,
                'attributes': self.attrs["attrs"]
            }

class NVARVariableStore(FirmwareVariableStore):
    '''NVAR has no header, only a series of variable headers.'''

    def __init__(self, data):
        self.variables = []
        self.valid_header = False
        if not NVARVariable.valid_nvar(data):
            return
        self.data = data
        self.valid_header = True

    def process(self):
        dlog(self, 'NVRAM')
        if not self.valid_header:
            return False

        var_offset = self.data
        total_size = 0
        while len(var_offset) > 4:
            nvar = NVARVariable(var_offset)
            if not nvar.process():
                break
            total_size += nvar.size
            self.variables.append(nvar)
            var_offset = var_offset[nvar.size:]

        # Scope data to just the parsed variables
        self.data = self.data[:total_size]
        self.attrs = {"variables": len(self.variables)}
        return True

    def build(self, generate_checksum=False, debug=False):
        data = ""
        for variable in self.variables:
            data += variable.build(generate_checksum, debug)
        return data

    def dump(self, parent, index=0):
        if not self.valid_header:
            return
        path = os.path.join(parent, "nvar.vars")
        dump_data(path, self.data)
        for i, variable in enumerate(self.variables):
            variable.dump(parent, i)
        pass

    def showinfo(self, ts="", index=0):
        if not self.valid_header:
            return
        print ("%s %s" % (
            blue("%sNVAR Variable Store:" % ts),
            "variables: %d" % self.attrs["variables"]
        ))
        for i, variable in enumerate(self.variables):
            variable.showinfo("%s  " % ts, i)

    def to_dict(self):
        if not self.valid_header:
            return None
        variables = []
        for variable in self.variables:
            variables.append(variable.to_dict())
        return {
            'variables': variables,
        }


class EfiSection(FirmwareObject):
    subsections = []

    @property
    def objects(self):
        return self.subsections

    def process_subsections(self):
        self.subsections = []

        if self.data is None:
            return False

        subsection_offset = 0
        status = True
        while subsection_offset < len(self.data):
            if subsection_offset % 4:
                subsection_offset += 4 - (subsection_offset % 4)
            if subsection_offset >= len(self.data):
                break

            try:
                subsection = FirmwareFileSystemSection(
                    self.data[subsection_offset:],
                    self.guid
                )
            except struct.error as e:
                dlog(self, 'subsections', 'Exception: %s' % (str(e)))
                return False
            if subsection.size == 0:
                break
            sub_status = subsection.process()
            if not sub_status:
                dlog(self, 'subsections', 'Could not parse subsection')
                status = False
            self.subsections.append(subsection)

            subsection_offset += subsection.size
        return status

    def build(self, generate_checksum=False, debug=False):
        raise Exception("Cannot build from unknown section type!")

    def process(self):
        pass

    def showinfo(self, ts='', index=-1):
        pass

    def to_dict(self):
        subsections = []
        for subsection in self.subsections:
            subsections.append(subsection.to_dict())
        return {
            'name': self.name,
            'subsections': subsections,
        }

    def dump(self, parent="", index=0):
        for i, subsection in enumerate(self.subsections):
            subsection.dump(parent, i)

    def _build_subsections(self, generate_checksum=False):
        data = ""
        for i, section in enumerate(self.subsections):
            subsection_size, subsection_data = section.build(generate_checksum)
            data += subsection_data
            if (i + 1 < len(self.subsections)):
                # Nibble-align inter-section subsections
                data += "\x00" * \
                    (((subsection_size + 3) & (~3)) - subsection_size)

        # Pad the pre-compression data
        trailling_bytes = len(self.data) - len(data)
        if trailling_bytes > 0:
            data += '\x00' * trailling_bytes
        return data


class CompressedSection(EfiSection):
    name = None

    ATTR_NOT_COMPRESSED = 0x00
    ATTR_STANDARD_COMPRESSION = 0x01
    ATTR_CUSTOMIZED_COMPRESSION = 0x02

    def __init__(self, data, guid):
        self.guid = guid
        self.data = None
        self.parsed_objects = []

        # http://dox.ipxe.org/PiFirmwareFile_8h_source.html
        self.decompressed_size, self.type = struct.unpack("<Ic", data[:5])
        self.type = ord(self.type)
        # A special compression type to determine (EFI/Tiano if type= 0x01).
        self.subtype = 0

        # Advance the byte pointer through the header
        self.compressed_data = data[5:]
        self.attrs = {
            "decompressed_size": self.decompressed_size, "type": self.type}

    def process(self):
        dlog(self, sguid(self.guid))
        def bf_decompress(data):
            return decompress([
                efi_compressor.LzmaDecompress,
                efi_compressor.TianoDecompress,
                efi_compressor.EfiDecompress,
            ], data)

        if self.type == 0x00:
            '''No compression.'''
            self.data = self.compressed_data

        results = None
        if self.type == 0x01:
            # Tiano or Efi compression, unfortunately these are identified by
            # the same byte
            results = decompress([
                efi_compressor.EfiDecompress,
                efi_compressor.TianoDecompress,
            ], self.compressed_data)
        if self.type == 0x02:
            results = bf_decompress(self.compressed_data)
            if results is None and len(self.compressed_data) > 4:
                # The type=2 is not spec-defined, may have an additional int
                # (Intel).
                results = bf_decompress(self.compressed_data[4:])

        if self.type > 0x00:
            if results is not None:
                self.subtype = results[0] + 1
                self.data = results[1]
            else:
                print_error(
                    "Cannot EFI decompress GUID (%s), type (%d), size (%d)" % (
                        sguid(self.guid),
                        self.type,
                        self.decompressed_size
                    )
                )
                raw = AutoRawObject(self.compressed_data)
                raw.process()
                self.subsections.append(raw)

        if self.data is None:
            '''No data was uncompressed.'''
            return True

        status = self.process_subsections()
        return status
        pass

    def build(self, generate_checksum=False, debug=False):
        data = self._build_subsections()

        if self.type == 0x01:
            if self.subtype == 0x01:
                data = str(efi_compressor.EfiCompress(data, len(data)))
            elif self.subtype == 0x02:
                data = str(efi_compressor.TianoCompress(data, len(data)))
        elif self.type == 0x02:
            data = str(efi_compressor.LzmaCompress(data, len(data)))
        elif self.type == 0x00:
            pass

        header = struct.pack("<Ic", self.decompressed_size, chr(self.type))
        return header + data
        pass

    def showinfo(self, ts):
        if self.name is not None:
            print ("%s %s" % (blue("%sCompressed Name:" % ts), purple(self.name)))
        for i, _object in enumerate(self.subsections):
            _object.showinfo(ts, i)

    def to_dict(self):
        subsections = []
        for subsection in self.subsections:
            subsections.append(subsection.to_dict())
        return {
            'name': self.name,
            'subsections': subsections,
        };


class VersionSection(EfiSection):

    def __init__(self, data):
        self.data = data
        self.build_number = struct.unpack("<16s", self.data[:16])


class FreeformGuidSection(EfiSection):
    '''A firmware file section type (free-form GUID)

    struct { UCHAR GUID[16]; }
    '''

    name = None

    def __init__(self, data):
        self.guid = struct.unpack("<16s", data[:16])[0]
        self.data = data[16:]

    def process(self):
        dlog(self, sguid(self.guid))
        if sguid(self.guid) == FIRMWARE_FREEFORM_GUIDS["CHAR_GUID"]:
            self.guid_header = self.data[:12]
            self.name = uefi_name(self.data[12:])
        return True

    def build(self, generate_checksum=False, debug=False):
        # print "Building FreeformGUID: %s" % green(sguid(self.guid))

        header = struct.pack("<16s", self.guid)
        return header + self.data

    def showinfo(self, ts='', index=-1):
        # print "%sGUID: %s" % (ts, green(sguid(self.guid)))
        if self.name is not None:
            print ("%sGUID Description: %s" % (ts, purple(self.name)))

    def to_dict(self):
        return {
            'guid': sguid(self.guid),
            'name': self.name,
        }


class GuidDefinedSection(EfiSection):
    '''A firmware file section type (GUID-defined)

    struct { UCHAR GUID[16]; short offset; short attrs; }
    '''

    ATTR_PROCESSING_REQUIRED = 0x01
    ATTR_AUTH_STATUS_VALID = 0x02

    def __init__(self, data):
        self.guid, self.offset, self.attr_mask = struct.unpack(
            "<16sHH", data[:20])

        # A guid-defined section includes an offset
        self.preamble = data[20:self.offset]
        self.data = data[self.offset:]
        self.attrs = {"attrs": self.attr_mask}
        self.subsections = []

    @property
    def objects(self):
        return self.subsections

    def process(self):
        dlog(self, sguid(self.guid))
        def parse_volume():
            fv = FirmwareVolume(self.data)
            if fv.valid_header:
                fv.process()
                self.subsections = [fv]
                return True
            return False

        def decompress_guid(alg):
            # Try to decompress the body of the section.
            results = decompress([alg], self.preamble + self.data)
            if results is None:
                # Attempt to recover by skipping the preamble.
                results = decompress([alg], self.data)
                if results is None:
                    return False
            self.subtype = results[0] + 1
            self.data = results[1]
            return self.process_subsections()

        status = True
        if sguid(self.guid) in [FIRMWARE_GUIDED_GUIDS["LZMA_COMPRESSED"], FIRMWARE_GUIDED_GUIDS["LZMA_COMPRESSED_HP"]]:
            status = decompress_guid(efi_compressor.LzmaDecompress)
        elif sguid(self.guid) == FIRMWARE_GUIDED_GUIDS["TIANO_COMPRESSED"]:
            status = decompress_guid(efi_compressor.TianoDecompress)
        elif sguid(self.guid) == FIRMWARE_GUIDED_GUIDS["ZLIB_COMPRESSED_QC"]:
            try:
                data = zlib.decompress(self.preamble + self.data, 31)
                if data is not None:
                    self.subtype = 0
                    self.data = data
                    self.process_subsections()
                else:
                    status = False
                    dlog(self, sguid(self.guid), 'error, empty zlib decompress')
            except zlib.error as err:
                status = False
                dlog(self, sguid(self.guid), 'zlib error: ' + str(err))
        # Todo: check for processing required attribute
        elif sguid(self.guid) == FIRMWARE_GUIDED_GUIDS["STATIC_GUID"]:
            # Todo: verify this (FirmwareFile hack)
            self.data = self.preamble[-4:] + self.data
            status = self.process_subsections()
            if len(self.subsections) == 0:
                # There were no subsections parsed, treat as a firmware volume
                status = parse_volume()
                if not status:
                    raw = AutoRawObject(self.data)
                    raw.process()
                    self.subsections.append(raw)
            pass
        elif sguid(self.guid) == FIRMWARE_GUIDED_GUIDS["FIRMWARE_VOLUME"]:
            status = parse_volume()
        else:
            # Undefined GUIDed-Section GUID, treat as a FV, don't require
            # status
            parse_volume()
        if not status:
            dlog(self, sguid(self.guid), 'Could not parse GUID object')
        return status
        pass

    def build(self, generate_checksum=False, debug=False):
        data = self._build_subsections(generate_checksum)

        if sguid(self.guid) in [FIRMWARE_GUIDED_GUIDS["LZMA_COMPRESSED"], FIRMWARE_GUIDED_GUIDS["LZMA_COMPRESSED_HP"]]:
            data = str(efi_compressor.LzmaCompress(data, len(data)))
            pass

        header = struct.pack(
            "<16sHH", self.guid, self.offset, self.attrs["attrs"])
        return header + self.preamble + data

    def showinfo(self, ts='', index=0):
        auth_status = "ATTR_UNKNOWN"
        if self.attrs["attrs"] == self.ATTR_AUTH_STATUS_VALID:
            auth_status = "AUTH_VALID"
        if self.attrs["attrs"] == self.ATTR_PROCESSING_REQUIRED:
            auth_status = "PROCESSING_REQUIRED"
        print ("%s%s %s offset= 0x%x attrs= 0x%x (%s)" % (
            ts, blue("Guid-Defined:"), green(sguid(self.guid)),
            self.offset, self.attrs["attrs"], purple(auth_status)
        ))
        if len(self.subsections) > 0:
            for i, section in enumerate(self.subsections):
                section.showinfo("%s  " % ts, index=i)

    def to_dict(self):
        auth_status = "ATTR_UNKNOWN"
        if self.attrs["attrs"] == self.ATTR_AUTH_STATUS_VALID:
            auth_status = "AUTH_VALID"
        if self.attrs["attrs"] == self.ATTR_PROCESSING_REQUIRED:
            auth_status = "PROCESSING_REQUIRED"

        subsections = []
        if len(self.subsections) > 0:
            for section in self.subsections:
                subsections.append(section.to_dict())

        return {
            'guid': sguid(self.guid),
            'offset': self.offset,
            'attributes': self.attrs["attrs"],
            'authStatus': auth_status,
            'subsections': subsections,
        }

    def dump(self, parent="", generate_checksum=False, debug=False):
        for i, subsection in enumerate(self.subsections):
            subsection.dump(parent, i)
        dump_data(os.path.join(parent, "guided.preamble"), self.preamble)
        dump_data(os.path.join(parent, "guided.certs"), self.preamble[172:])
        pass

    pass


class FirmwareFileSystemSection(EfiSection):
    '''A firmware file section

    struct { UINT8 Size[3]; EFI_SECTION_TYPE Type; } EFI_COMMON_SECTION_HEADER;
    struct { UINT8 Size[3]; EFI_SECTION_TYPE Type; UINT32 ExtendedSize; } EFI_COMMON_SECTION_HEADER2;
    '''

    parsed_object = None
    '''For object sections, keep track of each.'''

    def __init__(self, data, guid):
        self.guid = guid
        header = data[:0x4]

        self.valid_header = True
        try:
            self.size, self.type = struct.unpack("<3sB", header)
            self.size = struct.unpack("<I", self.size + b"\x00")[0]

            # check if ExtendedSize is used (FFSv3 only)
            if self.size == 0xffffff:
                self.size = struct.unpack("<I", data[4:8])[0]

        except Exception:
            print_error("Invalid FFS Section header, invalid length (%d)." % (
                len(header)
            ))
            self.valid_header = False
            return

        self._data = data[:self.size]
        self.data = data[0x4:self.size]
        self.name = None

    @property
    def objects(self):
        return [self.parsed_object]

    def regen(self, data):
        # Transitional method, should be adopted by other objects.
        self._data = data
        self.data = data[0x4:]

    def process(self):
        # section types, see PI spec v1.7 Errata A Volume 3, 2.1.5.1, table 3-4
        dlog(self, sguid(self.guid))
        self.parsed_object = None
        raw_object = False

        if self.type == 0x01:  # compression
            compressed_section = CompressedSection(self.data, self.guid)
            self.parsed_object = compressed_section

        elif self.type == 0x02:  # GUID-defined
            guid_defined = GuidDefinedSection(self.data)
            self.parsed_object = guid_defined

        elif self.type == 0x14:  # version string
            self.name = uefi_name(self.data)

        elif self.type == 0x15:  # user interface name
            self.name = uefi_name(self.data)

        elif self.type == 0x17:  # firmware-volume
            fv = FirmwareVolume(self.data, sguid(self.guid))
            if not fv.valid_header:
                # Could be a FFSv3 section (Kairos sample)
                fv = FirmwareVolume(self.data[4:], sguid(self.guid))
            if fv.valid_header:
                self.parsed_object = fv

        elif self.type == 0x18:  # freeform GUID
            freeform_guid = FreeformGuidSection(self.data)
            self.parsed_object = freeform_guid

        elif self.type == 0x19:  # raw
            raw_object = True
            if self.data[:10] == b"123456789A":
                # HP adds a strange header to nested FVs.
                fv = FirmwareVolume(self.data[12:], sguid(self.guid))
                self.parsed_object = fv
            else:
                # For a raw section, we can cheat and assign the parsed object
                # as the AutoRawObject's managed object
                raw = AutoRawObject(self.data)
                raw.process()
                if raw.object is not None:
                    self.parsed_object = raw.object

        self.attrs = {"type": self.type, "size": self.size}
        self.attrs["type_name"] = _get_section_type(self.type)[0]

        if self.parsed_object is None:
            return True
        status = self.parsed_object.process()
        if not status:
            dlog(self, sguid(self.guid), 'Could not parse %s' % (
                self.parsed_object.__class__.__name__))
            # Allow raw objects to fall-back.
            if raw_object:
                self.parsed_object = RawObject(self.data)
                status = True
        return status

    def build(self, generate_checksum=False, debug=False):
        data = ""
        # Add section data (either raw, or a partitioned section)
        if self.parsed_object is not None:
            data = self.parsed_object.build(generate_checksum)
        else:
            data = self.data

        # Pad the data and check for potential overflows.
        size = self.size
        trailling_bytes = (self.size - 4) - len(data)
        if trailling_bytes > 0:
            data += '\x00' * trailling_bytes
        if trailling_bytes < 0:
            size = self.size - trailling_bytes
            pass

        string_size = struct.pack("<I", size)
        header = struct.pack("<3sB", string_size[:3], self.type)
        return size, header + data
        pass

    def showinfo(self, ts='', index=-1):
        print ("%s type 0x%02x, size 0x%x (%d bytes) (%s section)" % (
            blue("%sSection %d:" % (ts, index)),
            self.type, self.size, self.size,
            _get_section_type(self.type)[0]
        ))
        if self.type == 0x15 and self.name is not None:
            print ("%sName: %s" % (ts, purple(self.name)))
        # DXE, PEI and SMM DEPEX sections
        if self.type == 0x13 or self.type == 0x1b or self.type == 0x1c:
            offset = 0
            while offset < len(self.data):
                opcode = ord(self.data[offset:offset+1])
                offset = offset + 1
                if opcode == 0x02:
                    guid = self.data[offset:offset+16]
                    guid_name = get_guid_name(guid)
                    offset = offset + 16
                    if guid_name is not None:
                            print ("%s  PUSH %s (%s)" % (ts, guid_name, sguid(guid)))
                    else:
                            print ("%s  PUSH %s" % (ts, sguid(guid)))
                elif opcode == 0x03:
                    print ("%s  AND" % (ts))
                elif opcode == 0x04:
                    print ("%s  OR" % (ts))
                elif opcode == 0x05:
                    print ("%s  NOT" % (ts))
                elif opcode == 0x06:
                    print ("%s  TRUE" % (ts))
                elif opcode == 0x06:
                    print ("%s  FALSE" % (ts))
                elif opcode == 0x08:
                    print ("%s  END" % (ts))
                else:
                    print ("%s  %02x?" % (ts, opcode))

        if self.parsed_object is not None:
            '''If this is a specific object, show that object's info.'''
            self.parsed_object.showinfo(ts + '  ')

    def to_dict(self):
        data = None
        if self.parsed_object is not None:
            data = self.parsed_object.to_dict()

        # section types see PI spec v1.7 Errata A Volume 3, 2.1.5.1, table 3-4
        # 0x13 - DXE DepEx
        # 0x1b - PEI DepEx
        # 0x1c - SMM DepEx
        if self.type == 0x13 or self.type == 0x1b or self.type == 0x1c:
            data = parse_depex(self.data)

        return {
            'type': self.type,
            'name': self.name, # only for section type 0x15 - User Interface
            'size': self.size,
            'sectionType': _get_section_type(self.type)[0],
            'data': data,
        }

    def dump(self, parent="", index=0):
        self.path = os.path.join(
            parent, "section%d.%s" % (index, _get_section_type(self.type)[1]))
        dump_data(self.path, self.data)

        if self.parsed_object is not None:
            self.parsed_object.dump(os.path.join(parent, "section%d" % index))


class FirmwareFile(FirmwareObject):
    '''A firmware file is contained within a firmware file system and is
    comprised of firmware file sections.

    struct {
        UCHAR: FileNameGUID[16]
        UINT16: Checksum (header/file)
        UINT8: Filetype
        UINT8: Attributes
        UINT8: Size[3]
        UINT8: State
    };
    '''
    _HEADER_SIZE = 0x18  # 24 byte header, always

    def __init__(self, data):
        header = data[:self._HEADER_SIZE]

        try:
            self.guid, self.checksum, self.type, self.attributes, \
                self.size, self.state = struct.unpack("<16sHBB3sB", header)
            self.size = struct.unpack("<I", self.size + b"\x00")[0]
        except Exception as e:
            print_error("Error: invalid FirmwareFile header.")
            raise e

        self.attrs = {
            "size": self.size,
            "type": self.type,
            "attributes": self.attributes,
            "state": self.state ^ 0xFF
        }
        self.attrs["type_name"] = _get_file_type(self.type)[0]

        # The size includes the header bytes.
        self._data = data[:self.size]
        self.data = data[self._HEADER_SIZE:self.size]
        self.raw_blobs = []
        self.sections = []

    @property
    def objects(self):
        invalid_types = [bytes, str, str]
        valid_blobs = [
            b for b in self.raw_blobs if type(b) not in invalid_types]
        return self.sections + valid_blobs

    def regen(self, data):
        # Transitional method, should be adopted by other objects.
        self.__init__(data)

    def process(self):
        '''Parse the file and file sections if appropriate.'''

        dlog(self, sguid(self.guid))
        if self.type == 0xf0:  # ffs padding
            dlog(self, sguid(self.guid), 'file is padding')
            return True

        status = True
        if sguid(self.guid) == FIRMWARE_VOLUME_GUIDS["NVRAM_NVAR"]:
            var_store = NVARVariableStore(self.data)
            if not var_store.valid_header:
                raw = AutoRawObject(self.data)
                raw.process()
                self.raw_blobs.append(raw)
            else:
                status = var_store.process()
                self.raw_blobs.append(var_store)
                if not status:
                    dlog(self, sguid(self.guid), 'Could not parse NVAR')
            return status

        if self.type == 0x01:  # raw file
            dlog(self, sguid(self.guid), 'file is Raw')
            status = self._find_objects()
            if not status:
                dlog(self, sguid(self.guid), 'Could not find Raw objects')
            return status

        if self.type == 0x00:  # unknown
            dlog(self, sguid(self.guid), 'file is unknown')
            raw = AutoRawObject(self.data)
            raw.process()
            self.raw_blobs.append(raw)
            return True

        section_data = self.data
        self.sections = []
        while len(section_data) >= 4:
            file_section = FirmwareFileSystemSection(section_data, self.guid)
            if not file_section.valid_header:
                dlog(self, sguid(self.guid), 'Invalid section header')
                return False
            if file_section.size <= 0:
                # This is not expected, something bad happened while parsing.
                print_error("Error: file section size <= 0 (%d)." %
                            file_section.size)
                return False

            status = file_section.process() and status
            self.sections.append(file_section)

            section_data = section_data[(file_section.size + 3) & (~3):]
        return status

    def _find_objects(self):
        '''Helper function for wacky raw type usages.'''
        has_object = False
        status = True

        # It may be a firmware volume (Lenovo or HP).
        fv = FirmwareVolume(self.data, sguid(self.guid))
        if fv.valid_header:
            has_object = True
            status = fv.process() and status
            self.raw_blobs.append(fv)
        elif self.data[0x10:0x10 + 4] == FLASH_HEADER:
            # Lenovo may also bundle a flash descriptor as raw content.
            from .flash import FlashDescriptor
            flash = FlashDescriptor(self.data)
            if flash.valid_header:
                has_object = True
                status = flash.process() and status
                self.raw_blobs.append(flash)

        # If everything is normal (according to the FV/FF spec).
        if not has_object:
            # There may be arbitrary firmware structures (Lenovo)
            objects = find_volumes(self.data)
            self.raw_blobs += objects
            return True
        return status

    def build(self, generate_checksum=False, debug=False):
        data = ""
        for i, section in enumerate(self.sections):
            section_size, section_data = section.build(generate_checksum)
            data += section_data
            if (i + 1 < len(self.sections)):
                # Nibble-align inter-file sections
                data += "\x00" * (((section_size + 3) & (~3)) - section_size)

        for blob in self.raw_blobs:
            if isinstance(blob, FirmwareObject):
                data += blob.build(generate_checksum)
            elif isinstance(blob, AutoRawObject) or isinstance(blob, RawObject):
                data += blob.data
            else:
                data += blob

        # Maining to support ffs-padding
        if len(self.raw_blobs) == 0 and len(self.sections) == 0:
            data = self.data

        if generate_checksum:
            pass

        size = self.size
        trailling_bytes = size - (len(data) + 24)
        if trailling_bytes < 0:
            print ("%s adding %s-bytes to GUID: %s" % (
                red("Warning"),
                red(trailling_bytes * -1),
                red(sguid(self.guid))
            ))
            size += (trailling_bytes * -1)

        string_size = struct.pack("<I", size)
        header = struct.pack(
            "<16sHBB3sB",
            self.guid, self.checksum, self.type, self.attributes, string_size[
                :3], self.state
        )
        return size, header + data

    def showinfo(self, ts='', index="N/A"):
        guid_name = get_guid_name(self.guid)
        if guid_name is None:
            guid_display = "%s" % green(sguid(self.guid))
        else:
            guid_display = "%s (%s)" % (
                green(sguid(self.guid)), purple(guid_name))
        print("%s %s type 0x%02x, attr 0x%02x, state 0x%02x, size 0x%x "
            "(%d bytes), (%s)" % (
            blue("%sFile %s:" % (ts, index)),
            guid_display,
            self.type,
            self.attributes,
            self.state ^ 0xFF,
            self.size,
            self.size,
            _get_file_type(self.type)[0]
        ))

        for i, blob in enumerate(self.raw_blobs):
            if type(blob) not in [str, bytes]:
                blob.showinfo(ts + "  ", index=i)
            else:
                self._guessinfo_text(ts + "  ", blob, index=i)

        if self.sections is None:
            # padding file, skip for now
            return

        for i, section in enumerate(self.sections):
            section.showinfo(ts + "  ", index=i)

    def to_dict(self):
        sections = []
        for section in self.sections:
            s = section.to_dict()
            if s is not None:
                sections.append(s)

        blobs = []
        for i, blob in enumerate(self.raw_blobs):
            if type(blob) not in [str, bytes]:
                blobs.append(blob.to_dict())
            else:
                info = {
                    'note': self._guessinfo_dict(blob),
                }
                blobs.append(info)

        # file types see PI spec v1.7 Errata A Volume 3, 2.1.4.1, table 3-3
        return {
            'guid': sguid(self.guid),
            'name': get_guid_name(self.guid),
            'type': self.type,
            'attributes': self.attributes,
            'state': self.state ^ 0xFF,
            'size': self.size,
            'fileType': _get_file_type(self.type)[0],
            'sections': sections,
            'blobs': blobs,
        }

    def _is_ucode(self, data):
        return data[:4] == "\x01\x00\x00\x00" and data[20:24] == "\x01\x00\x00\x00"

    def _guessinfo_dict(self, data):
        if _is_ucode(data):
            return "Might contain CPU microcodes"

    def _guessinfo_text(self, ts, data, index="N/A"):
        if _is_ucode(data):
            print ("%s Might contain CPU microcodes" % (
                blue("%sBlob %d:" % (ts, index))))

    def dump(self, parent=""):
        parent = os.path.join(parent, "file-%s" % sguid(self.guid))

        dump_data(os.path.join(parent, "file.obj"), self._data)
        if self.raw_blobs is not None:
            for i, blob in enumerate(self.raw_blobs):
                blob.dump(parent, index=i)

        if self.sections is not None:
            for i, section in enumerate(self.sections):
                section.dump(parent, index=i)


class FirmwareFileSystem(FirmwareObject):
    '''A potential UEFI firmware filesystem (FFS) data stream.

    The FFS is a specific GUID within the FirmwareVolume.
    '''

    def __init__(self, data):
        self.files = []
        self._data = data

        # Overflow data is non-file data within the filesystem
        self.overflow_data = ""

    @property
    def objects(self):
        return self.files or []

    def process(self):
        '''Search for a 24-byte header that does not contain all 0xFF.'''

        dlog(self, 'ffs')
        data = self._data
        status = True
        while len(data) >= 24 and data[:24] != (b"\xff" * 24):
            firmware_file = FirmwareFile(data)

            if firmware_file.size < 24:
                # This is a problem, the file was corrupted.
                break
            ff_status = firmware_file.process()
            if not ff_status:
                dlog(self, 'ffs', 'Could not parse FF')
                status = False
            self.files.append(firmware_file)
            data = data[(firmware_file.size + 7) & (~7):]

        if len(data) > 0:
            # There is overflow data
            self.overflow_data = data
        return status

    def build(self, generate_checksum=False, debug=False):

        # Generate the file system data as an unstructed set of file data.
        data = ""
        for firmware_file in self.files:
            file_size, file_data = firmware_file.build(generate_checksum)
            data += file_data
            data += "\xFF" * (((file_size + 7) & (~7)) - file_size)

        data += self.overflow_data

        if len(data) != len(self._data):
            print ("ffs size mismatch old=%d new=%d %d" % (
                len(self._data),
                len(data),
                len(self._data) - len(data)
            ))

        return data
        pass

    def showinfo(self, ts='', index=None):
        for i, firmware_file in enumerate(self.files):
            firmware_file.showinfo(ts + ' ', index=i)

    def to_dict(self):
        res = []
        for firmware_file in self.files:
            res.append(firmware_file.to_dict())
        return res

    def dump(self, parent=""):
        dump_data(os.path.join(parent, "filesystem.ffs"), self._data)

        for _file in self.files:
            _file.dump(parent)


class FirmwareVolume(FirmwareObject):
    '''Describes the features and layout of the firmware volume.

    struct EFI_FIRMWARE_VOLUME_HEADER {
        UINT8: Zeros[16]
        UCHAR: FileSystemGUID[16]
        UINT64: Length
        UINT32: Signature (_FVH)
        UINT32: Attribute mask
        UINT16: Header Length
        UINT16: Checksum
        UINT16: ExtHeaderOffset
        UINT8: Reserved[1]
        UINT8: Revision
        [<BlockMap>]+, <BlockMap(0,0)>
    };

    The block map is a set of blocks followed by a zeroed block indicating the
    end of the map set.

    struct BLOCK_MAP {
        UINT32: Block count
        UINT32: Block size
    };

    ExtHeaderOffset is an offset to a EFI_FIRMWARE_VOLUME_EXT_HEADER structure:

    struct EFI_FIRMWARE_VOLUME_EXT_HEADER {
        UCHAR: FvName[16]
        UINT32: ExtHeaderSize
    };
    '''

    _HEADER_SIZE = 0x38

    _EXT_HEADER_SIZE = 0x14

    name = None
    '''string: An optional name or offset of the firmware volume.'''

    block_map = None
    '''list: An empty block set.'''

    firmware_filesystems = []
    '''list: Set of FirmwareFileSystems discovered in volume.'''

    raw_objects = []
    '''list: Set of RawObjects discovered in volume.'''

    def __init__(self, data, name="0"):
        self.firmware_filesystems = []
        self.raw_objects = []
        self.name = name
        self.valid_header = False
        try:
            header = data[:self._HEADER_SIZE]
            self.rsvd, self.guid, self.size, self.magic, self.attributes, \
                self.hdrlen, self.checksum, self.exthdroff, self.rsvd2, \
                self.revision = struct.unpack("<16s16sQ4sIHHHsB", header)
        except Exception as e:
            dlog(self, name, "Exception in __init__: %s" % (str(e)))
            # print "Error: cannot parse FV header (%s)." % str(e)
            return

        if self.magic != b'_FVH':
            return

        if sguid(self.guid) not in list(FIRMWARE_VOLUME_GUIDS.values()):
            dlog(self, sguid(self.guid), 'Unrecognized volume GUID')
            return

        self.blocks = []
        self.block_map = ""

        try:
            data = data[:self.size]

            self._data = data
            self.data = data[self.hdrlen:]
            self.block_map = data[self._HEADER_SIZE:self.hdrlen]
        except Exception as e:
            dlog(self, name, "Exception in __init__: %s" % (str(e)))
            print_error("Error invalid FV header data (%s)." % str(e))
            return

        try:
            exthdr = self._data[self.exthdroff:self.exthdroff + self._EXT_HEADER_SIZE]
            self.fvname, self.exthdrsize = struct.unpack("<16sI", exthdr)
            assert self.exthdrsize == self._EXT_HEADER_SIZE
        except Exception as e:
            dlog(self, name, "Exception in __init__: %s" % (str(e)))
            print_error("Error invalid FV header data (%s)." % str(e))
            # not fatal

        self.valid_header = True
        pass

    @property
    def objects(self):
        return self.firmware_filesystems or []

    def process(self):
        dlog(self, self.name)
        if self.block_map is None:
            dlog(self, self.name, 'Block Map was not parsed')
            return False

        block_data = self.block_map
        while len(block_data) > 0:
            block = block_data[:8]

            block_size, block_length = struct.unpack("<II", block)
            if (block_size, block_length) == (0, 0):
                '''The block map ends with a (0, 0) block.'''
                break

            self.blocks.append((block_size, block_length))
            block_data = block_data[8:]

        if len(self.blocks) == 0:
            '''No block in the volume? This is a problem.'''
            dlog(self, self.name, 'No blocks discovered')
            return False

        data = self.data
        self.firmware_filesystems = []
        self.raw_objects = []
        status = True

        ffs_guids = [
            FIRMWARE_VOLUME_GUIDS["FFS1"],
            FIRMWARE_VOLUME_GUIDS["FFS2"],
            FIRMWARE_VOLUME_GUIDS["FFS3"],
            FIRMWARE_VOLUME_GUIDS["PFH1"],
            FIRMWARE_VOLUME_GUIDS["PFH2"],
        ]
        for block in self.blocks:
            if sguid(self.guid) in ffs_guids:
                # FIXME: there may only be a single FFS, which is the FV body
                # see https://uefi.org/sites/default/files/resources/PI_Spec_1_7_A_final_May1.pdf
                # Volume 3, section 2.1.2
                # and https://edk2-docs.gitbook.io/edk-ii-build-specification/2_design_discussion/22_uefipi_firmware_images
                firmware_filesystem = FirmwareFileSystem(
                    data[:block[0] * block[1]])
                ffs_status = firmware_filesystem.process()
                if not ffs_status:
                    dlog(self, self.name, 'Could not parse FFS')
                    status = False
                self.firmware_filesystems.append(firmware_filesystem)
            elif sguid(self.guid) == FIRMWARE_VOLUME_GUIDS["NVRAM_EVSA"]:
                # If this is an NVRAM volume, there are no FFS/FFs.
                self.raw_objects.append(
                    NVARVariableStore(data[:block[0] * block[1]]))
            else:
                self.raw_objects.append(data[:block[0] * block[1]])
            data = data[block[0] * block[1]:]
        return status

    def build(self, generate_checksum=False, debug=False):
        # Generate blocks from FirmwareFileSystems
        data = ""
        for filesystem in self.firmware_filesystems:
            # print "Building filesystem"
            data += filesystem.build(generate_checksum)

        # Generate block map from original block map (assume no size change)
        block_map = ""
        for block in self.blocks:
            block_map += struct.pack("<II", block[0], block[1])
        # Add a trailing-NULL to the block map
        block_map += "\x00" * 8

        if generate_checksum:
            pass

        # Assume no size change
        header = struct.pack(
            "<16s16sQ4sIHH3sB",
            self.rsvd, self.guid, self.size,
            self.magic, self.attributes, self.hdrlen,
            self.checksum, self.rsvd2, self.revision
        )
        return header + block_map + data
        pass

    def showinfo(self, ts='', index=None):
        if not self.valid_header or len(self.data) == 0:
            return

        print("%s %s attr 0x%08x, rev %d, cksum 0x%x, size 0x%x (%d bytes)" % (
            blue("%sFirmware Volume:" % (ts)),
            green(sguid(self.guid)),
            self.attributes,
            self.revision,
            self.checksum,
            self.size,
            self.size
        ))
        print(blue("%s  Firmware Volume Blocks: " % (ts)), end="")
        for block_size, block_length in self.blocks:
            print("(%d, 0x%x)" % (block_size, block_length), end="")
        print("")

        for _ffs in self.firmware_filesystems:
            _ffs.showinfo(ts + " ")
        for raw in self.raw_objects:
            print("%s%s NVRAM" % ("%s  " % ts, blue("Raw section:")))

    def to_dict(self):
        if not self.valid_header or len(self.data) == 0:
            return

        blocks = []
        for block_size, block_length in self.blocks:
            blocks.append({
                'size': block_size,
                'length': block_length,
            })
        ffs = []
        if len(self.firmware_filesystems) > 0:
            ffs = (self.firmware_filesystems[0].to_dict())

        # TODO? for raw in self.raw_objects:

        return {
            'guid': sguid(self.guid),
            'nameGuid': sguid(self.fvname),
            'attributes': self.attributes,
            'revision': self.revision,
            'checksum': self.checksum,
            'size': self.size,
            'blocks': blocks,
            'ffs': ffs,
        }

    def dump(self, parent="", index=None):
        if len(self.data) == 0:
            return

        path = os.path.join(parent, "volume-%s.fv" % self.name)
        dump_data(path, self._data)

        for _ffs in self.firmware_filesystems:
            _ffs.dump(os.path.join(parent, "volume-%s" % self.name))

1
class FirmwareCapsule(FirmwareObject):
    '''EFI Capsule Header.

    struct EFI_CAPSULE_HEADER {
        UCHAR:  CapsuleGUID[16]
        UINT32: HeaderSize
        UINT32: Flags
        UINT32: CapsureImageSize
        UINT32: SequenceNumber
        UCHAR:  InstanceGUID[16]
        UINT32: OffsetToSplitInformation
        UINT32: OffsetToCapsuleBody
        UINT32: OffsetToOemDefinedHeader
        UINT32: OffsetToAuthorInformation
        UINT32: OffsetToRevisionInformation
        UINT32: OffsetToShortDescription
        UINT32: OffsetToLongDescription
        UINT32: OffsetToApplicableDevices
    }
    '''

    capsule_body = None
    '''binary: Data string of the capsule content.'''

    def __init__(self, data, name="Capsule"):
        self.name = name
        self.valid_header = True
        self.data = None

        self.capsule_guid = data[:16]
        self.guid = "\x00" * 16
        if sguid(self.capsule_guid) not in FIRMWARE_CAPSULE_GUIDS:
            self.valid_header = False
            return

        try:
            self.parse_capsule_header(data[16:])
        except:
            self.valid_header = False
            return

        # Header sections
        self.header_sections = []

        # Set data (original, and body content)
        self._data = data
        self.data = data[self.header_size:]

        pass

    def parse_capsule_header(self, data):
        if sguid(self.capsule_guid) == FIRMWARE_CAPSULE_GUIDS[0]:
            # EFICapsule
            self.size, self.flags, self.image_size, self.seq_num = struct.unpack(
                "<IIII",
                data[:4 * 4]
            )
            self.guid = data[16:32]
            split_info, capsule_body, oem_header, author_info, revision_info, \
                short_desc, long_desc, compatibility = struct.unpack(
                    "<" + "I" * 8,
                    data[32:32 + 4 * 8]
                )

            # Store offsets
            self.offsets = {
                # This offset can be relative to the base of the capsule or end
                # of the header.
                "capsule_body": capsule_body,
                "split_info": split_info,
                "oem_header": oem_header,
                "author_info": author_info,
                "revision_info": revision_info,
                "short_desc": short_desc,
                "long_desc": long_desc,
                "compatibility": compatibility
            }
        elif sguid(self.capsule_guid) == FIRMWARE_CAPSULE_GUIDS[1]:
            # EFI2Capsule
            self.size, self.flags, self.image_size = struct.unpack(
                "<III", data[:4 * 3])
            fv_image, oem_header = struct.unpack("<HH", data[12:12 + 4])
            self.offsets = {
                "capsule_body": fv_image,
                "oem_header": oem_header,
                "author_info": 0
            }
        elif sguid(self.capsule_guid) == FIRMWARE_CAPSULE_GUIDS[2]:
            # UEFI Capsule
            self.size, self.flags, self.image_size = struct.unpack(
                "<III", data[:4 * 3])
            self.offsets = {
                "capsule_body": self.size,
                "oem_header": 0,
                "author_info": 0
            }
        elif sguid(self.capsule_guid) == FIRMWARE_CAPSULE_GUIDS[4]:
            # AMI Aptio Capsule
            self.size, self.flags, self.image_size = struct.unpack(
                "<III", data[:4 * 3])
            self.offsets = {
                "capsule_body": self.size,
                "oem_header": 0,
                "author_info": 0
            }


        self.header_size = self.size
        pass

    def parse_sections(self, header):
        # Parse the various pieces within the capsule header, before body
        pass

    @property
    def objects(self):
        return [self.capsule_body]

    def process(self):
        # Copy the EOH to capsule into a preamble
        self.preamble = self.data[:self.offsets["capsule_body"]]
        self.parse_sections(None)

        fv = FirmwareVolume(self.data[self.offsets["capsule_body"]:])
        if not fv.valid_header:
            # The body could be an offset from the end of the header (Intel
            # does this).
            fv = FirmwareVolume(
                self.data[self.offsets["capsule_body"] - self.header_size:])
            if not fv.valid_header:
                return False

        self.size += fv.size
        if not fv.process():
            # Todo: test code coverage
            # return False
            pass
        self.capsule_body = fv
        return True

    def build(self, generate_checksum=False, debug=False):
        if self.capsule_body is not None:
            body = self.capsule_body.build(generate_checksum, debug=debug)
        else:
            body = self.data[self.offsets["capsule_body"]:]

        # Assume no size change
        return self._data[:self.header_size] + self.preamble + body
        pass

    def showinfo(self, ts='', index=None):
        if not self.valid_header or len(self.data) == 0:
            return

        print ("%s %s flags 0x%08x, size 0x%x (%d bytes)" % (
            blue("%sFirmware Capsule:" % (ts)),
            "%s/%s" % (green(sguid(self.capsule_guid)),
                       green(sguid(self.guid))),
            self.flags, self.size, self.size
        ))
        print ("%s  Details: size= 0x%x (%d bytes) body= 0x0%x, oem= 0x0%x, author= 0x0%x" % (
            ts, self.image_size, self.image_size,
            self.offsets["capsule_body"], self.offsets[
                "oem_header"], self.offsets["author_info"]
        ))
        # print self.offsets

        if self.capsule_body is not None:
            self.capsule_body.showinfo(ts)
        pass

    def to_dict(self):
        if not self.valid_header or len(self.data) == 0:
            return

        body = None
        if self.capsule_body is not None:
            body = self.capsule_body.to_dict()

        return {
            'capsuleGuid': sguid(self.capsule_guid),
            'guid': sguid(self.guid),
            'flags': self.flags,
            'size': self.size,
            'imageSize': self.image_size,
            'offsets': {
                'capsuleBody': self.offsets["capsule_body"],
                'oemHeader': self.offsets["oem_header"],
                'authorInfo': self.offsets["author_info"],
            },
            'body': body,
        }

    def dump(self, parent="", index=None):
        if len(self.data) == 0:
            return

        path = os.path.join(parent, "capsule-%s.cap" % self.name)
        dump_data(path, self._data)

        if self.capsule_body is not None:
            self.capsule_body.dump(
                os.path.join(parent, "capsule-%s" % self.name))
        else:
            # Write the raw image data from the capsule.
            path = os.path.join(parent, "capsule-%s.image" % self.name)
            offset = self.offsets["capsule_body"]
            dump_data(path, self.data[offset:offset + self.image_size])
