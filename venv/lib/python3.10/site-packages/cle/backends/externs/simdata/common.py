from __future__ import annotations

import struct

from cle.backends.relocation import Relocation
from cle.backends.symbol import SymbolType

from .simdata import SimData


class StaticData(SimData):
    """
    A simple SimData utility class to use when you have a SimData which should provide just a static
    set of bytes. To use, implement the following:

    :cvar name:     The name of the symbol to provide.
    :cvar libname:  The name of the library from which the symbol originally comes (currently unused).
    :cvar data:     The bytes to provide
    """

    type = SymbolType.TYPE_OBJECT
    data: bytes = NotImplemented

    @classmethod
    def static_size(cls, owner):
        return len(cls.data)

    def value(self):
        return self.data


class StaticWord(SimData):
    """
    A simple SimData utility class to use when you have a SimData which should provide just a static
    integer. To use, implement the following:

    :cvar name:     The name of the symbol to provide.
    :cvar libname:  The name of the library from which the symbol originally comes (currently unused).
    :cvar word:     The value to provide
    :cvar wordsize: (optional) The size of the value in bytes, default the CPU wordsize
    """

    type = SymbolType.TYPE_OBJECT
    word: int = NotImplemented
    wordsize: int = None

    @classmethod
    def static_size(cls, owner):
        return owner.arch.bytes if cls.wordsize is None else cls.wordsize

    def value(self):
        return struct.pack(self.owner.arch.struct_fmt(size=self.wordsize), self.word)


class PointTo(SimData):
    """
    A simple SimData utility class to use when you have a SimData which should provide just a
    pointer to some other symbol. To use, implement the following:

    :cvar name:         The name of the symbol to provide.
    :cvar libname:      The name of the library from which the symbol originally comes (currently unused).
    :cvar pointto_name: The name of the symbol to point to
    :cvar pointto_type: The type of the symbol to point to (usually ``SymbolType.TYPE_FUNCTION`` or
                        ``SymbolType.TYPE_OBJECT``)
    :cvar addend:       (optional) an integer to be added to the symbol's address before storage
    """

    pointto_name: str = NotImplemented
    pointto_type: SymbolType = NotImplemented
    type: SymbolType = SymbolType.TYPE_OBJECT
    addend: int = 0

    @classmethod
    def static_size(cls, owner):
        return owner.arch.bytes

    def value(self):
        return bytes(self.size)

    def relocations(self):
        return [
            SimDataSimpleRelocation(
                self.owner,
                self.owner.make_import(self.pointto_name, self.pointto_type),
                self.relative_addr,
                self.addend,
            )
        ]


class SimDataSimpleRelocation(Relocation):
    """
    A relocation used to implement PointTo. Pretty simple.
    """

    def __init__(self, owner, symbol, addr, addend, preresolved=False):
        super().__init__(owner, symbol, addr)
        self.addend = addend
        self.preresolved = preresolved

    def resolve_symbol(self, solist, **kwargs):
        if self.preresolved:
            self.resolve(self.symbol)
        else:
            super().resolve_symbol(solist, **kwargs)

    @property
    def value(self):
        return self.resolvedby.rebased_addr + self.addend
