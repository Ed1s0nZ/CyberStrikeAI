from __future__ import annotations

import binascii
import logging
import re

import archinfo

from cle.errors import CLEError

from .backend import Backend, register_backend

log = logging.getLogger(name=__name__)

__all__ = ("SRec",)

srec_regex = "S([0-9])([0-9a-fA-F]{{2}})([0-9a-fA-F]{{{addr_size}}})([0-9a-fA-F]{{4,64}})([0-9a-fA-F]{{2}})"
SREC_ADDR_SIZE = {"0": 16, "1": 16, "5": 16, "9": 16, "2": 24, "6": 24, "8": 24, "3": 32, "7": 32}


SREC_HEADER = 0x00
SREC_DATA = {0x01, 0x02, 0x03}
SREC_START_EXEC = {0x7, 0x8, 0x9}
SREC_COUNT = {0x5, 0x6}
HEX_TYPE_EXTSEGADDR = 0x02
HEX_TYPE_STARTSEGADDR = 0x03
HEX_TYPE_EXTLINEARADDR = 0x04
HEX_TYPE_STARTLINEARADDR = 0x05


class SRec(Backend):
    """
    A loader for Motorola SRecord Objects
    See https://en.wikipedia.org/wiki/Intel_HEX
    """

    is_default = True  # Tell CLE to automatically consider using the Hex backend

    @staticmethod
    def calc_checksum(data):
        sum(x.to for x in data)

    @staticmethod
    def parse_record(line):
        addr_size = SREC_ADDR_SIZE[chr(line[1])]
        srec_re = re.compile(srec_regex.format(addr_size=addr_size // 4).encode())
        m = srec_re.match(line)
        if not m:
            raise CLEError(f"Invalid SRec record: {line}")
        my_cksum = 0
        rectype, count, addr, data, cksum = m.groups()
        cksum = int(cksum, 16)
        for d in binascii.unhexlify(line[2:-2]):
            my_cksum = (my_cksum + d) % 256
        my_cksum = 0xFF - my_cksum
        if my_cksum != cksum:
            raise CLEError(f"Invalid checksum: Computed {hex(my_cksum)}, found {hex(cksum)}")
        count = int(count, 16) - ((addr_size // 8) + 1)
        addr = int(addr, 16)
        rectype = int(rectype, 16)
        if data:
            data = binascii.unhexlify(data)
        if data and count != len(data):
            raise CLEError("Data length field does not match length of actual data: " + line)
        return rectype, addr, data

    @staticmethod
    def coalesce_regions(regions):
        # Lots of tiny memory regions is bad!
        # The greedy algorithm to smash them together:
        result = []
        last_addr: int | None = None
        last_data: list[bytes] | None = None
        last_size: int | None = None
        for addr, region in sorted(regions):
            if last_addr is not None and last_addr + last_size == addr:
                last_data.append(region)
                last_size += len(region)
            else:
                if last_addr is not None:
                    result.append((last_addr, b"".join(last_data)))
                last_addr, last_data, last_size = addr, [region], len(region)

        if last_addr is not None:
            result.append((last_addr, b"".join(last_data)))

        return result

    def __init__(self, *args, ignore_missing_arch: bool = False, **kwargs):
        super().__init__(*args, **kwargs)
        self.set_load_args(ignore_missing_arch=ignore_missing_arch)

        if self._arch is None:
            if ignore_missing_arch:
                # used internally for testing. we use a default architecture
                self.set_arch(archinfo.arch_from_id("amd64"))
            else:
                raise CLEError(
                    "To use the Hex binary backend, you need to specify an architecture in the loader options."
                )

        # Do the whole thing in one shot.
        self.os = "unknown"
        got_base = False
        got_entry = False
        self._binary_stream.seek(0)
        string = self._binary_stream.read()
        recs = string.splitlines()
        regions = []
        max_addr = 0
        min_addr = 0xFFFFFFFFFFFFFFFF
        self._base_address = 0
        for rec in recs:
            rectype, addr, data = SRec.parse_record(rec)
            if rectype == SREC_HEADER:
                continue
            if rectype in SREC_DATA:
                addr += self._base_address
                # l.debug("Loading %d bytes at " % len(data) + hex(addr))
                # Raw data.  Put the bytes
                regions.append((addr, data))
                # We have to be careful about the min and max addrs
                min_addr = min(min_addr, addr)
                max_addr = max(max_addr, addr + len(data) - 1)
            elif rectype in SREC_START_EXEC:
                got_entry = True
                self._entry = int.from_bytes(data, "big")
                log.debug("Found entry point at %#x", self._entry)
                self._initial_ip = self._entry
            else:
                raise CLEError("This SRec Object type is not implemented: " + hex(rectype))
        if not got_base:
            log.warning("No base address was found in this SRec object file. It is assumed to be 0")
        if not got_entry:
            log.warning(
                "No entry point was found in this SRec object file, and it is assumed to be 0. "
                "Specify one with `entry_point` to override."
            )
        # HEX specifies a ton of tiny little memory regions.  We now smash them together to make things faster.
        new_regions = SRec.coalesce_regions(regions)
        self.regions: list[tuple[int, int]] = []  # A list of (addr, size)
        for addr, data in new_regions:
            self.memory.add_backer(addr, data)
            self.regions.append((addr, len(data)))
        self._max_addr = max_addr
        self._min_addr = min_addr

    @staticmethod
    def is_compatible(stream):
        stream.seek(0)
        s = stream.read(0x10)
        stream.seek(0)
        return s.startswith(b"S")


register_backend("srec", SRec)
