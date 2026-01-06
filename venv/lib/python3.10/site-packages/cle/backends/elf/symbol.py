from __future__ import annotations

from elftools.elf.enums import ENUM_ST_INFO_TYPE

from cle.address_translator import AT
from cle.backends.symbol import Symbol, SymbolType

from .symbol_type import ELFSymbolType


def maybedecode(string):
    return string if isinstance(string, str) else string.decode()


class ELFSymbol(Symbol):
    """
    Represents a symbol for the ELF format.

    :ivar str binding:      The binding of this symbol as an ELF enum string
    :ivar section:          The section associated with this symbol, or None
    :ivar _subtype:         The ELFSymbolType of this symbol
    """

    def __init__(self, owner, symb):
        subtype_num = ENUM_ST_INFO_TYPE.get(symb.entry.st_info.type, symb.entry.st_info.type)
        arch_list = [owner.arch.name, None]
        if "UNIX" in owner.os:
            arch_list.insert(1, "gnu")
        for arch in arch_list:
            try:
                self._subtype = ELFSymbolType((subtype_num, arch))
            except ValueError:
                pass
            else:
                self._type = self._subtype.to_base_type()
                break
        else:
            self._subtype = None
            self._type = SymbolType.TYPE_OTHER

        sec_ndx, value = symb.entry.st_shndx, symb.entry.st_value

        # A relocatable object's symbol's value is relative to its section's addr.
        if owner.is_relocatable and isinstance(sec_ndx, int):
            value += owner.sections[sec_ndx].remap_offset

        super().__init__(
            owner, maybedecode(symb.name), AT.from_lva(value, owner).to_rva(), symb.entry.st_size, self.type
        )

        self.version = None
        self.binding = symb.entry.st_info.bind
        self.is_hidden = symb.entry["st_other"]["visibility"] == "STV_HIDDEN"
        self.section = sec_ndx if not isinstance(sec_ndx, str) else None
        self.is_static = self._type == SymbolType.TYPE_SECTION or sec_ndx == "SHN_ABS"
        self.is_common = sec_ndx == "SHN_COMMON"
        self.is_weak = self.binding == "STB_WEAK"
        self.is_local = self.binding == "STB_LOCAL"

        self.is_import = sec_ndx == "SHN_UNDEF" and self.binding in ("STB_GLOBAL", "STB_WEAK")
        self.is_export = (self.section is not None or self.is_common) and self.binding in ("STB_GLOBAL", "STB_WEAK")

    @property
    def subtype(self) -> ELFSymbolType:
        return self._subtype
