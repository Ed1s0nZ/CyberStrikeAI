from __future__ import annotations

import binascii
import logging
import re
import struct

import archinfo

from cle.errors import CLEError

from .backend import Backend, register_backend
from .region import Section, Segment

log = logging.getLogger(name=__name__)

__all__ = ("Hex",)

intel_hex_re = re.compile(
    b":([0-9a-fA-F][0-9a-fA-F])([0-9a-fA-F][0-9a-fA-F][0-9a-fA-F][0-9a-fA-F])"
    b"([0-9a-fA-F][0-9a-fA-F])([0-9a-fA-F][0-9a-fA-F]+)*([0-9a-fA-F][0-9a-fA-F])"
)

HEX_TYPE_DATA = 0x00
HEX_TYPE_EOF = 0x01
HEX_TYPE_EXTSEGADDR = 0x02
HEX_TYPE_STARTSEGADDR = 0x03
HEX_TYPE_EXTLINEARADDR = 0x04
HEX_TYPE_STARTLINEARADDR = 0x05


class HexSection(Section):
    """
    Describes a section in memory after loading a HEX file into memory. This section may come from the HEX file or
    added separately (e.g., the MMIO region).
    """

    def __init__(self, name, vaddr, size, initialized=True, readable=True, writable=False, executable=True):
        super().__init__(name, 0, vaddr, size)
        self.initialized = initialized
        self.readable = readable
        self.writable = writable
        self.executable = executable

    @property
    def is_readable(self):
        return self.readable

    @property
    def is_writable(self):
        return self.writable

    @property
    def is_executable(self):
        return self.executable

    @property
    def only_contains_uninitialized_data(self) -> bool:
        return not self.initialized


class Hex(Backend):
    """
    A loader for Intel Hex Objects
    See https://en.wikipedia.org/wiki/Intel_HEX
    """

    is_default = True  # Tell CLE to automatically consider using the Hex backend

    @staticmethod
    def parse_record(line):
        m = intel_hex_re.match(line)
        if not m:
            raise CLEError(f"Invalid HEX record: {line}")
        my_cksum = 0
        count, addr, rectype, data, cksum = m.groups()
        cksum = int(cksum, 16)
        for d in binascii.unhexlify(line[1:-2]):
            my_cksum = (my_cksum + d) % 256
        my_cksum = ((my_cksum ^ 0xFF) + 1) % 256
        if my_cksum != cksum:
            raise CLEError(f"Invalid checksum: Computed {hex(my_cksum)}, found {hex(cksum)}")
        count = int(count, 16)
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

    def __init__(
        self, *args, ignore_missing_arch: bool = False, extra_regions: list[dict[str, int | bool]] = None, **kwargs
    ):
        super().__init__(*args, **kwargs)
        self.set_load_args(ignore_missing_arch=ignore_missing_arch)
        if extra_regions:
            self.set_load_args(extra_regions=extra_regions)

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
            rectype, addr, data = Hex.parse_record(rec)
            if rectype == HEX_TYPE_DATA:
                addr += self._base_address
                # l.debug("Loading %d bytes at " % len(data) + hex(addr))
                # Raw data.  Put the bytes
                regions.append((addr, data))
                # We have to be careful about the min and max addrs
                if addr < min_addr:
                    min_addr = addr
                max_addr = max(max_addr, addr + len(data) - 1)
            elif rectype == HEX_TYPE_EOF:
                # EOF
                log.debug("Got EOF record.")
                break
            elif rectype == HEX_TYPE_EXTSEGADDR:
                # "Extended Mode" Segment address, take this value, multiply by 16, make the base
                self._base_address = struct.unpack(">H", data)[0] * 16
                got_base = True
                log.debug("Loading a segment at %#x", self._base_address)
            elif rectype == HEX_TYPE_STARTSEGADDR:
                # Four bytes, the segment and the initial IP
                got_base = True
                got_entry = True
                self._initial_cs, self._initial_ip = struct.unpack(">HH", data)
                # The whole thing is the entry, as far as angr is concerned.
                self._entry = struct.unpack(">I", data)[0]
                log.debug("Got entry point at %#x", self._entry)
            elif rectype == HEX_TYPE_EXTLINEARADDR:
                got_base = True
                # Specifies the base for all future data bytes.
                self._base_address = struct.unpack(">H", data)[0] << 16
                log.debug("Loading a segment at %#x", self._base_address)
            elif rectype == HEX_TYPE_STARTLINEARADDR:
                got_entry = True
                # The 32-bit EIP, really the same as STARTSEGADDR, but some compilers pick one over the other.
                self._entry = struct.unpack(">I", data)[0]
                log.debug("Found entry point at %#x", self._entry)
                self._initial_eip = self._entry
            else:
                raise CLEError("This HEX Object type is not implemented: " + hex(rectype))
        if not got_base:
            log.warning("No base address was found in this HEX object file. It is assumed to be 0")

        self.mapped_base = 0
        self._min_addr = min_addr
        self._max_addr = max_addr - self.mapped_base

        if self.load_args.get("entry_point") is not None:
            self._entry = self._custom_entry_point = self.load_args["entry_point"] - min_addr
            got_entry = True

        if not got_entry:
            log.warning(
                "No entry point was found in this HEX object file, and it is assumed to be 0. "
                "Specify one with `entry_point` to override."
            )

        # HEX specifies a ton of tiny little memory regions.  We now smash them together to make things faster.
        new_regions = Hex.coalesce_regions(regions)
        self.regions: list[tuple[int, int]] = []  # A list of (addr, size)
        for addr, data in new_regions:
            self.memory.add_backer(addr, data)
            self.regions.append((addr, len(data)))
            self.segments.append(Segment(0, addr, len(data), len(data)))
            self.sections.append(HexSection(f"sec_{addr:x}", addr, len(data)))

        self._create_extra_regions()

    @staticmethod
    def is_compatible(stream):
        stream.seek(0)
        s = stream.read(0x10)
        stream.seek(0)
        return s.startswith(b":")

    def _create_extra_regions(self) -> None:
        """
        Parse load_args["extra_regions"] and create segments/sections accordingly. This method should be called at the
        end of the loading process (if the backend supports loading extra regions).

        "extra_regions": [
        {
            "name": "ram",
            "addr": 0x20000000,
            "size": 0x1000,
            "readable": True,
            "writable": True,
            "executable": False,
        },
        {
            "name": "mmio",
            ...
        },
        ]
        """

        # in case special regions are specified
        if "extra_regions" not in self.load_args or not self.load_args["extra_regions"]:
            return
        for region in self.load_args["extra_regions"]:
            self.segments.append(Segment(0, region["addr"], 0, region["size"]))
            self.sections.append(
                HexSection(
                    region["name"],
                    region["addr"],
                    region["size"],
                    initialized=region.get("initialized", True),
                    readable=region.get("readable", True),
                    writable=region.get("writable", False),
                    executable=region.get("executable", True),
                )
            )


register_backend("hex", Hex)
