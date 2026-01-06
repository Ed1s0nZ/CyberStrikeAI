from __future__ import annotations

from collections.abc import Iterable

import archinfo

from cle.address_translator import AT
from cle.backends.backend import Backend
from cle.backends.symbol import Symbol, SymbolType


class PEStubs(Backend):
    """
    A backend that synthesizes a minimal PE header, suitable for being traversed out of the PEB/LDR structures.

    Initialize it with a dll name and a list of names it should export.
    """

    def __init__(self, libname: str, exported_names: Iterable[str], arch: archinfo.Arch, **kwargs):
        super().__init__(libname, None, arch=arch, **kwargs)
        self.provides = libname
        self.pic = True
        self._string_table_indices = {}

        self._exported_names = names = list(exported_names)
        sorted_names_ordinals = sorted((name, idx) for idx, name in enumerate(names))

        plus = self.arch.bits == 64
        coff = 0x100
        optional = coff + 0x18
        directories = optional + (112 if plus else 96)
        exports = coff + 0x300
        addr_table = exports + 0x40
        addr_table_size = (len(names) + 1) * 4
        name_table = addr_table + addr_table_size
        name_table_size = (len(names) + 1) * 4
        ordinal_table = name_table + name_table_size
        ordinal_table_size = (len(names) + 1) * 2
        string_table = ordinal_table + ordinal_table_size
        string_table_content = bytearray()

        def alloc(s: str) -> int:
            if s not in self._string_table_indices:
                self._string_table_indices[s] = len(string_table_content)
                string_table_content.extend(s.encode("utf-8"))
                string_table_content.append(0)
            return string_table + self._string_table_indices[s]

        dll_name = alloc(libname)
        for name in names:
            alloc(name)

        text = string_table + len(string_table_content)
        func_size = 8  # 8 is arbitrary
        text_size = len(names) * func_size
        self.map_size = text + text_size

        for i, name in enumerate(names):
            self.symbols.add(Symbol(self, name, i * func_size + text, 8, SymbolType.TYPE_FUNCTION))

        self.memory.add_backer(0, bytes(self.map_size))
        self.memory.store(0, b"MZ")
        self.memory.pack_word(0x3C, coff, size=4)
        self.memory.store(coff, b"PE\0\0")
        self.memory.pack_word(optional + 0, 0x20B if plus else 0x10B, size=4)
        self.memory.pack_word(optional + (108 if plus else 92), 16, size=4)
        self.memory.pack_word(directories, exports, size=4)

        self.memory.pack_word(exports + 12, dll_name, size=4)
        self.memory.pack_word(exports + 16, 1, size=4)
        self.memory.pack_word(exports + 20, len(names), size=4)
        self.memory.pack_word(exports + 24, len(names), size=4)
        self.memory.pack_word(exports + 28, addr_table, size=4)
        self.memory.pack_word(exports + 32, name_table, size=4)
        self.memory.pack_word(exports + 36, ordinal_table, size=4)

        for idx, (name, ordinal) in enumerate(sorted_names_ordinals):
            self.memory.pack_word(addr_table + ordinal * 4, text + ordinal * func_size, size=4)
            self.memory.pack_word(name_table + idx * 4, alloc(name), size=4)
            self.memory.pack_word(ordinal_table + idx * 2, ordinal, size=2)
        self.memory.store(string_table, string_table_content)

    @property
    def max_addr(self):
        return AT.from_rva(self.map_size - 1, self).to_mva()
