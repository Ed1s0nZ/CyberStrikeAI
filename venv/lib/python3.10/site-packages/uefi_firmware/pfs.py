import struct
import os

from .base import FirmwareObject, RawObject, BaseObject, AutoRawObject
from .uefi import FirmwareVolume
from .utils import print_error, dump_data, sguid, green, blue


PFS_GUIDS = {
    "FIRMWARE_VOLUMES": "7ec6c2b0-3fe3-42a0-a316-22dd0517c1e8",
    "INTEL_ME":         "7439ed9e-70d3-4b65-9e33-1963a7ad3c37",
    "BIOS_ROMS_1":      "08e56a30-62ed-41c6-9240-b7455ee653d7",
    "BIOS_ROMS_2":      "492261e4-0659-424c-82b6-73274389e7a7"
}


def _discover_volumes(data):
    # Assume a series of firmware volumes
    volumes = []
    fv_offset = 0
    while fv_offset < len(data):
        fv = FirmwareVolume(data[fv_offset:], hex(fv_offset))
        if not fv.valid_header:
            break
        if not fv.process():
            return False
        volumes.append(fv)
        fv_offset += fv.size
    return volumes


class PFRegion(FirmwareObject):
    def __init__(self, header, data):
        self.offset, self.size, self.address, self.name_offset = struct.unpack(
            '<IIQI', header[:20])
        self.header = header
        self.data = data[:self.size]

    @property
    def objects(self):
        return [self.obj]

    def process(self):
        self.obj = AutoRawObject(self.data)
        if not self.obj.process():
            self.obj = RawObject(self.data)
        else:
            self.obj = self.obj.object
        return True

    def showinfo(self, ts='', index=0):
        print('%s%s size= 0x%x offset= 0x%x address= 0x%x name= 0x%x' % (
            ts, blue("PHRegionEntry:"),
            self.size, self.offset, self.address, self.name_offset
        ))
        self.obj.showinfo('%s  ' % (ts), index)

    def dump(self, parent='', index=None):
        if self.obj is not None:
            self.obj.dump(parent, index=index)


class PFHeader(FirmwareObject, BaseObject):
    '''A PFHeader: gh://skochinsky/181e6e338d90bb7f2693098dc43c6d54


    struct PFRegionEntry
    {
      UINT32 FileOffset;
      UINT32 Size;
      UINT64 FlashAddress;
      UINT32 NameOffset; //absolute offset
    };

    struct PFHeader
    {
      /* 00 */char Signature[4]; //$PFH
      /* 04 */UINT32 dwVersion;
      /* 08 */UINT32 dwHeaderSize;
      /* 0C */UINT16 wHeaderChecksum;
      /* 0E */UINT32 dwTotalImageSize;
      /* 12 */UINT16 wTotalImageChecksum;
      /* 04 */UINT32 dwNumberOfImages;
      /* 18 */UINT32 imagetableOffset;
      /* 1C */UINT32 unknown[48];
      /* DC */PFImageEntry rgtable[1];
    };
    '''

    def __init__(self, data):
        self.valid_header = False
        self.data = data
        self.size = len(data)

        if self.size < 32:
            return
        if self.data[:4] == b'$PFH':
            self.valid_header = True
        version, hdr_size, checksum, image_size, image_checksum, image_count, \
            image_offset = struct.unpack('<IIHIHII', self.data[4:28])
        self.hdr_size = hdr_size
        self.image_count = image_count
        self.image_offset = image_offset

    @property
    def objects(self):
        return self.objs

    def process(self):
        if not self.valid_header:
            return False

        self.objs = []
        data_offset = self.hdr_size
        for i in range(self.image_count):
            region_offset = 0xDC + (i * 20)
            entry = PFRegion(self.data[region_offset:region_offset + 20],
                self.data[data_offset:])
            data_offset += entry.size
            if entry.process():
                self.objs.append(entry)
        return True

    def showinfo(self, ts='', index=None):
        print('%s%s header size 0x%x images= %d offset= %s' % (
            ts, blue("PFHeader:"),
            self.hdr_size,  self.image_count, self.image_offset
        ))
        for i in range(len(self.objs)):
            self.objs[i].showinfo('%s  ' % (ts), index=i)
            pass

    def dump(self, parent='', index=None):
        path = os.path.join(parent, "pfheader.pfh")
        dump_data("%s" % path, self.data)
        images = os.path.join(parent, "pfheader")
        for i in range(len(self.objs)):
            self.objs[i].dump(images, i)


class PFSPartitionedSection(FirmwareObject, BaseObject):
    '''A PFSSection with embedded PFSFiles (with additional sections) that
    split the content of the section across multiple chunks. The chunks and the
    encompassing embedded PFSFile are named as 'partitioned' section.

    Search within a PFSFile, similar to handling sets of PFSSections without
    recording information/objects for each section. Instead, strip all data
    and content, save for the defined body. The object then represents the
    single, concatenated body.
    '''

    HEADER_SIZE = 0x48
    DATA_OFFSET = 0x248

    def __init__(self, data):
        self.data = data
        self.size = len(data)
        self.section_objects = []
        self.partitions = 0
        self.section_data = b""

    def process(self):
        # The end removes the PFS trailer.
        body_end = self.size - 0x10
        # This data is the content of a section, with stripped PFS header.
        # The first line will be the UUID.
        self.uuid = self.data[0x0:0x10]
        body_step = 0x10

        # The stepping is equivilent to a PFSSection save for a 0x200-sized
        # set of variables.
        while body_step < body_end:
            # The UUID for partitioned section is useless.
            header = self.data[body_step:body_step + self.HEADER_SIZE]
            if len(header) < self.HEADER_SIZE:
                return False
            self.partitions += 1
            size = struct.unpack("<I", header[0x28:0x28 + 0x04])[0]
            # Advance the seek pointer past the header.
            body_step += self.HEADER_SIZE
            # The section data seeks past an offset of variables.
            data = self.data[body_step + self.DATA_OFFSET:body_step + size]
            self.section_data += data
            sig1_size, trp_size, sig2_size = struct.unpack("<III", header[0x2C:0x2C + 0x0C])
            body_step += size + sig1_size + trp_size + sig2_size

        # Now that section partitions are reconstructed, search for volumes.
        volumes = _discover_volumes(self.section_data)
        if volumes is False:
            return False
        self.section_objects = volumes
        return True

    @property
    def objects(self):
        return self.section_objects

    def showinfo(self, ts='', index=None):
        print("%s%s %s partitions %d size 0x%x (%d bytes)" % (
            ts, blue("Dell PFSPartitionedSection:"), green(sguid(self.uuid)),
            self.partitions, len(self.section_data), len(self.section_data)))
        for sub_object in self.section_objects:
            sub_object.showinfo("%s  " % ts)

    def dump(self, parent="", index=None):
        path = os.path.join(parent, "%s" % sguid(self.uuid))
        dump_data("%s.data" % path, self.section_data)

        path = os.path.join(parent, "section-%s" % sguid(self.uuid))
        for sub_object in self.section_objects:
            sub_object.dump(path)
        pass


class PFSSection(FirmwareObject, BaseObject):
    HEADER_SIZE = 0x48

    def __init__(self, data):
        self.data = data
        self.size = -1

        # Store parsed objects (if any)
        self.section_objects = []

    def process(self):
        hdr = self.data[:self.HEADER_SIZE]
        self.uuid = hdr[:0x10]
        self.header = hdr

        # Spec seems to be a consistent 1, what I thought was a timestamp is not.
        # Version is static except for the first section in a PFS
        ##spec, ts, ctype, version, _u1 = struct.unpack("<I4shh4s", hdr[0x10:0x20])
        spec, version_type = struct.unpack("<I4s", hdr[0x10:0x10 + 0x08])
        self.spec = spec
        self.version = ""
        self.type = 0
        self.ts = 0
        for i in range(4):
            group_offset = 0x18 + (i * 2)
            if version_type[i:i+1] == b'A':
                self.version += "%X" % struct.unpack("<h", hdr[group_offset:group_offset + 2])
            elif version_type[i:i+1] == b'N':
                self.version += ".%d" % struct.unpack("<h", hdr[group_offset:group_offset + 2])

        # U1, U2 might be flag containers
        _u2, section_size, rsa1_size, pmim_size, rsa2_size = struct.unpack(
            "<8sIIII", hdr[0x20:0x20 + 0x18])

        # This seems to be a set of 8byte CRCs for each chunk (4 total)
        self.crcs = hdr[0x20 + 0x18:self.HEADER_SIZE]
        self.section_data = self.data[
            self.HEADER_SIZE:self.HEADER_SIZE + section_size]

        rsa1_offset = self.HEADER_SIZE + section_size
        self.rsa1 = RawObject(self.data[rsa1_offset:rsa1_offset + rsa1_size])
        pmim_offset = rsa1_offset + rsa1_size
        self.pmim = RawObject(self.data[pmim_offset:pmim_offset + pmim_size])
        rsa2_offset = pmim_offset + pmim_size
        self.rsa2 = RawObject(self.data[rsa2_offset:rsa2_offset + rsa2_size])

        # Unknown 8byte variable
        # _u3 = self.data[64+total_chunk_size:64+total_chunk_size+8]
        self.unknowns = [_u2]

        # Size of header, data, and footer
        total_size = section_size + rsa1_size + pmim_size + rsa2_size
        self.section_size = self.HEADER_SIZE + total_size
        self.data = None

        if self.section_data[:0x08] == b"PFS.HDR.":
            # Partitioned ROM
            rom = PFSPartitionedSection(self.section_data)
            if not rom.process():
                return False
            self.section_objects.append(rom)
        elif sguid(self.uuid) == PFS_GUIDS["FIRMWARE_VOLUMES"]:
            volumes = _discover_volumes(self.section_data)
            if volumes is False:
                return False
            self.section_objects += volumes
        else:
            raw = AutoRawObject(self.section_data)
            raw.process()
            self.section_objects.append(raw)

    @property
    def objects(self):
        return self.section_objects + [self.rsa1, self.pmim, self.rsa2]

    def info(self, include_content=False):
        return {
            "_self": self,
            "guid": sguid(self.uuid),
            "type": "PFSSection",
            "content": self.section_data if include_content else "",
            "attrs": {
                "size": self.section_size,
                "crcs": self.crcs,
                "unknowns": [u for u in self.unknowns],
                "version": self.version
            },
            "chunks": [self.rsa1, self.pmim, self.rsa2] if include_content else []
        }
        pass

    def build(self, generate_checksum=False, debug=False):
        body = ""
        for sub_object in self.section_objects:
            body += sub_object.build(generate_checksum, debug=debug)
        return self.header + body + \
            self.rsa1.build(generate_checksum) + \
            self.pmim.build(generate_checksum) + \
            self.rsa2.build(generate_checksum)
        pass

    def showinfo(self, ts=0, index=None):
        print("%s%s %s spec 0x%02x ts 0x%02x type 0x%02x version %s size 0x%x (%d bytes)" % (
            ts, blue("Dell PFSSection:"), green(sguid(self.uuid)),
            self.spec, self.ts, self.type, self.version,
            self.section_size, self.section_size
        ))

        for sub_object in self.section_objects:
            sub_object.showinfo("%s  " % ts)
        pass

    def dump(self, parent="", index=None):
        path = os.path.join(parent, "%s" % sguid(self.uuid))
        dump_data("%s.data" % path, self.section_data)

        # Instead of calling dump on each chunk RawObject, dump with a better
        # name.
        if len(self.rsa1.data) > 0:
            dump_data("%s.rsa1" % path, self.rsa1.data)
        if len(self.pmim.data) > 0:
            dump_data("%s.pmim" % path, self.pmim.data)
        if len(self.rsa2.data) > 0:
            dump_data("%s.rsa2" % path, self.rsa2.data)

        path = os.path.join(parent, "section-%s" % sguid(self.uuid))
        for sub_object in self.section_objects:
            sub_object.dump(path)
        pass


class PFSFile(FirmwareObject):
    PFS_HEADER = b"PFS.HDR."
    PFS_FOOTER = b"PFS.FTR."

    def __init__(self, data):
        self.sections = []
        self.data = data
        self.valid_header = False
        if self.check_header():
            self.valid_header = True

    def check_header(self):
        if len(self.data) < 32:
            print_error("Data does not contain a header.")
            return False

        header = self.data[:0x10]
        magic, spec, size = struct.unpack("<8sII", header)

        self.spec = spec
        self.size = size

        if magic != self.PFS_HEADER:
            print_error(
                "Data does not contain the header magic (%s)." % self.PFS_HEADER)
            return False

        footer_offset = self.size + 0x10
        footer = self.data[footer_offset:footer_offset + 0x10]
        # Footer size is a repeated body size.
        footer_size, _u2, footer_magic = struct.unpack("<II8s", footer)
        if footer_magic != self.PFS_FOOTER:
            print_error(
                "Data does not container the footer magic (%s)." % self.PFS_FOOTER)
            return False

        return True

    def process(self):
        '''Chunks are assumed to contain a chunk header.'''
        data = self.data[16:-16]
        if not self.valid_header:
            return False

        chunk_num = 0
        offset = 16
        while True:
            section = PFSSection(data)
            section.process()
            self.sections.append(section)

            chunk_num += 1
            offset += section.section_size
            data = data[section.section_size:]

            if len(data) < 64:
                break
        return True

    @property
    def objects(self):
        return self.sections

    def build(self, generate_checksum=False, debug=False):
        body = ""
        for section in self.sections:
            body += section.build(generate_checksum, debug=debug)
        return self.data[:16] + body + self.data[-16:]
        pass

    def showinfo(self, ts='', index=None):
        print("%s%s spec 0x%x size 0x%x (%d bytes)" % (
            ts, blue("DellPFS:"),
            self.spec, self.size, self.size
        ))
        for section in self.sections:
            section.showinfo("%s  " % ts)

    def dump(self, parent='', index=None):
        path = os.path.join(parent, "pfsobject.pfs")
        dump_data(path, self.data)

        path = os.path.join(parent, "pfsobject")
        for section in self.sections:
            section.dump(path)
