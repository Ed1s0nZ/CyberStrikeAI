"""
References:
    - http://www.hexblog.com/wp-content/uploads/2012/06/Recon-2012-Skochinsky-Compiler-Internals.pdf
    - https://www.airs.com/blog/archives/460
    - https://www.airs.com/blog/archives/464
"""

from __future__ import annotations

from elftools.common.utils import struct_parse
from elftools.dwarf.enums import DW_EH_encoding_flags
from elftools.dwarf.structs import DWARFStructs, Struct


class ExceptionTableHeader:
    __slots__ = (
        "lp_start",
        "ttype_encoding",
        "ttype_offset",
        "call_site_encoding",
        "call_site_table_len",
    )

    def __init__(self, lp_start, ttype_encoding, ttype_offset, call_site_encoding, call_site_table_len):
        self.lp_start = lp_start  # landing pad start offset
        self.ttype_encoding = ttype_encoding  # encoding of pointers in type table
        self.ttype_offset = ttype_offset  # Type table offset
        self.call_site_encoding = call_site_encoding  # encoding of items in call site table
        self.call_site_table_len = call_site_table_len  # total length of call site table


class CallSiteEntry:
    __slots__ = ("cs_start", "cs_len", "cs_lp", "cs_action")

    def __init__(self, cs_start, cs_len, cs_lp, cs_action):
        self.cs_start = cs_start
        self.cs_len = cs_len
        self.cs_lp = cs_lp
        self.cs_action = cs_action


class LSDAExceptionTable:
    """
    LSDA exception table parser.

    TODO: Much of this class should be eventually moved to pyelftools.
    """

    def __init__(self, stream, bits, little_endian=True):
        self.address = None
        self.base_offset = None
        self.stream = stream

        if bits in (32, 64):
            dwarf_format = bits
        else:
            raise ValueError(f"Unsupported bits value {bits}. Expect either 32 or 64.")

        self.entry_structs = DWARFStructs(
            little_endian=little_endian, dwarf_format=dwarf_format, address_size=bits // 8
        )
        self._formats = self._eh_encoding_to_field(self.entry_structs)

    @staticmethod
    def _eh_encoding_to_field(entry_structs):
        """
        Shamelessly copied from pyelftools since the original method is a bounded method.

        Return a mapping from basic encodings (DW_EH_encoding_flags) the
        corresponding field constructors (for instance
        entry_structs.Dwarf_uint32).
        """
        return {
            DW_EH_encoding_flags["DW_EH_PE_absptr"]: entry_structs.Dwarf_target_addr,
            DW_EH_encoding_flags["DW_EH_PE_uleb128"]: entry_structs.Dwarf_uleb128,
            DW_EH_encoding_flags["DW_EH_PE_udata2"]: entry_structs.Dwarf_uint16,
            DW_EH_encoding_flags["DW_EH_PE_udata4"]: entry_structs.Dwarf_uint32,
            DW_EH_encoding_flags["DW_EH_PE_udata8"]: entry_structs.Dwarf_uint64,
            DW_EH_encoding_flags["DW_EH_PE_sleb128"]: entry_structs.Dwarf_sleb128,
            DW_EH_encoding_flags["DW_EH_PE_sdata2"]: entry_structs.Dwarf_int16,
            DW_EH_encoding_flags["DW_EH_PE_sdata4"]: entry_structs.Dwarf_int32,
            DW_EH_encoding_flags["DW_EH_PE_sdata8"]: entry_structs.Dwarf_int64,
        }

    def parse_lsda(self, address, offset):
        self.address = address
        self.base_offset = offset
        self.stream.seek(offset)
        header = self._parse_lsda_header()

        csrs: list[CallSiteEntry] = []
        start_offset = self.stream.tell()
        while self.stream.tell() - start_offset < header.call_site_table_len:
            csr = self._parse_call_site_entry(header.call_site_encoding)
            if csr is not None:
                csrs.append(csr)

        return csrs

    def _parse_lsda_header(self):
        # lpstart
        lpstart_encoding = self.stream.read(1)[0]
        if lpstart_encoding != DW_EH_encoding_flags["DW_EH_PE_omit"]:
            base_encoding = lpstart_encoding & 0x0F
            modifier = lpstart_encoding & 0xF0

            lpstart = struct_parse(Struct("dummy", self._formats[base_encoding]("LPStart")), self.stream)["LPStart"]

            if modifier == 0:
                pass
            elif modifier == DW_EH_encoding_flags["DW_EH_PE_pcrel"]:
                lpstart += self.address + (self.stream.tell() - self.base_offset)
            else:
                raise NotImplementedError(f"Unsupported modifier {modifier:#x}.")

        else:
            lpstart = None

        # ttype
        ttype_encoding = self.stream.read(1)[0]
        if ttype_encoding != DW_EH_encoding_flags["DW_EH_PE_omit"]:
            ttype_offset = struct_parse(Struct("dummy", self.entry_structs.Dwarf_uleb128("TType")), self.stream)[
                "TType"
            ]
        else:
            ttype_offset = None

        # call site table length
        cstable_encoding = self.stream.read(1)[0]
        cstable_length = struct_parse(Struct("dummy", self.entry_structs.Dwarf_uleb128("CSTable")), self.stream)[
            "CSTable"
        ]

        return ExceptionTableHeader(
            lpstart,
            ttype_encoding,
            ttype_offset,
            cstable_encoding,
            cstable_length,
        )

    def _parse_call_site_entry(self, encoding):
        base_encoding = encoding & 0x0F
        modifier = encoding & 0xF0

        # header
        s = struct_parse(
            Struct(
                "CallSiteEntry",
                self._formats[base_encoding]("cs_start"),
                self._formats[base_encoding]("cs_len"),
                self._formats[base_encoding]("cs_lp"),
                self.entry_structs.Dwarf_uleb128("cs_action"),
            ),
            self.stream,
        )

        cs_start = s["cs_start"]
        cs_len = s["cs_len"]
        cs_lp = s["cs_lp"]
        cs_action = s["cs_action"]

        if modifier == 0:
            pass
        else:
            raise NotImplementedError(f"Unsupported modifier for CallSiteEntry: {modifier:#x}.")

        return CallSiteEntry(cs_start, cs_len, cs_lp, cs_action)
