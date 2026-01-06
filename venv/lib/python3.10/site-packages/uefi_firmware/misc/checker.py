"""
These are misc functions/classes to implement several type checkers.
The TypeTester may be useful if parsing a large number of UEFI-related binaries.
"""


from builtins import bytes
import re

from ..uefi import FirmwareVolume, FirmwareCapsule, FirmwareFile
from ..pfs import PFSFile, PFHeader
from ..flash import FlashDescriptor
from ..me import MeContainer, MeManifestHeader


class TypeTester(object):
    parser = None
    static = b"MZ"

    def match(self, data):
        if data[:self.size] == self.static:
            return True
        return False

    @property
    def size(self):
        return len(self.static)

    @property
    def name(self):
        return self.__class__.__name__.replace("Tester", "")


class UEFIFirmwareVolumeTester(TypeTester):
    parser = FirmwareVolume

    def match(self, data):
        fv = FirmwareVolume(data)
        return fv.valid_header


class FlashDescriptorTester(TypeTester):
    parser = FlashDescriptor

    def match(self, data):
        fd = FlashDescriptor(data)
        return fd.valid_header


class EFICapsuleTester(TypeTester):
    static = bytes.fromhex("".join(
        "BD 86 66 3B 76 0D 30 40 B7 0E B5 51 9E 2F C5 A0".split()))
    parser = FirmwareCapsule


class UEFICapsuleTester(TypeTester):
    static = bytes.fromhex("".join(
        "B9 82 91 53 B5 AB 91 43 B6 9A E3 A9 43 F7 2F CC".split()))
    parser = FirmwareCapsule


class IntelMEPartitionManifestTester(TypeTester):
    static = bytes.fromhex("".join("04 00 00 00 A1 00 00 00".split()))
    parser = MeManifestHeader


class IntelMETester(TypeTester):
    parser = MeContainer

    def match(self, data):
        me = MeContainer(data)
        return me.valid_header


class PFHeaderTester(TypeTester):
    parser = PFHeader

    def match(self, data):
        pfh = PFHeader(data)
        return pfh.valid_header


class DellPFSTester(TypeTester):
    static = b"PFS.HDR"
    parser = PFSFile


class DellUpdateBinaryTester(TypeTester):
    hdr_pattern = re.compile(
        b'.{4}\xAA\xEE\xAA\x76\x1B\xEC\xBB\x20\xF1\xE6\x51.{1}\x78\x9C')
    static = b"\x00" * 100

    def match(self, data):
        hdr_match = self.hdr_pattern.search(data)
        if hdr_match is None:
            return False
        return True

TESTERS = [
    FlashDescriptorTester,
    UEFICapsuleTester,
    EFICapsuleTester,
    IntelMEPartitionManifestTester,
    IntelMETester,
    DellPFSTester,
    DellUpdateBinaryTester,
    PFHeaderTester,
    UEFIFirmwareVolumeTester,
]
