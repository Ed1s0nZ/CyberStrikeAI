from __future__ import annotations

import logging

from cle.address_translator import AT
from cle.backends.backend import Backend
from cle.backends.region import Segment
from cle.backends.relocation import Relocation
from cle.backends.symbol import Symbol, SymbolType
from cle.errors import CLEError, CLEOperationError
from cle.utils import ALIGN_UP

from .simdata import SimData, lookup
from .simdata.common import PointTo, SimDataSimpleRelocation

log = logging.getLogger(name=__name__)


__all__ = [
    "ExternSegment",
    "TOCRelocation",
    "ExternObject",
    "KernelObject",
    "PointToPrecise",
]


class ExternSegment(Segment):
    def __init__(self, map_size):
        super().__init__(None, 0, None, map_size)

    def addr_to_offset(self, addr):
        raise CLEOperationError(
            "'offset' operations on the extern object are meaningless as it is not mapped from a file"
        )

    def offset_to_addr(self, offset):
        raise CLEOperationError(
            "'offset' operations on the extern object are meaningless as it is not mapped from a file"
        )

    def contains_offset(self, offset):
        return False

    is_readable = True
    is_writable = True
    is_executable = True


class TOCRelocation(Relocation):
    @property
    def value(self):
        return self.resolvedby.rebased_addr


class ExternObject(Backend):
    def __init__(self, loader, map_size=0, tls_size=0):
        super().__init__("cle##externs", None, loader=loader)
        self._next_object = None
        self._delayed_writes = []

        self.next_addr = 0
        self.map_size = map_size
        self.set_arch(loader.main_object.arch)
        self.provides = "extern-address space"
        self.pic = True
        self._import_symbols = {}
        self._warned_data_import = set()

        self.tls_data_size = tls_size
        self.tls_next_addr = 0
        self._tls_mapped = False

    def _finalize_tls(self):
        if self._is_mapped or self._tls_mapped:
            raise Exception("programming error")

        if self.tls_data_size != 0:
            self.tls_used = True
            self.tls_data_start = self._allocate(self.tls_data_size, alignment=0x10)
            self.tls_block_size = self.tls_data_size
            self._tls_mapped = True

    def rebase(self, new_base):
        if self._is_mapped:
            return

        if not self._tls_mapped:
            self._finalize_tls()

        backer = bytearray(self.map_size)
        for simdata in self._delayed_writes:
            value = simdata.value()
            start_addr = simdata.relative_addr
            if simdata.type == SymbolType.TYPE_TLS_OBJECT:
                start_addr += self.tls_data_size
            backer[start_addr : start_addr + len(value)] = value

        self.memory.add_backer(0, bytes(backer))
        self.segments.append(ExternSegment(self.map_size))
        super().rebase(new_base)

    def make_extern(
        self, name, size=0, alignment=None, thumb=False, sym_type=SymbolType.TYPE_FUNCTION, point_to=None, libname=None
    ) -> Symbol:
        try:
            return self._symbol_cache[name]
        except KeyError:
            pass

        tls = sym_type == SymbolType.TYPE_TLS_OBJECT
        SymbolCls = Symbol
        if point_to is not None:
            simdata = PointToPrecise
        else:
            simdata = lookup(name, libname)
        if simdata is not None:
            SymbolCls = simdata
            size = simdata.static_size(self)
            if sym_type != simdata.type:
                log.warning("Symbol type mismatch between export request and response for %s. What's going on?", name)

        real_size = max(size, 1)

        if alignment is None:
            alignment = self.arch.bytes

        make_toc = getattr(self.loader.main_object, "is_ppc64_abiv1", False) and sym_type == SymbolType.TYPE_FUNCTION
        toc_symbol = None
        if make_toc:
            # we make two symbols, one for the func and one for the toc
            # the one for the func ends up named with the #func suffix, the toc gets the normal name
            # we return the one for the toc
            toc_symbol = self.make_extern(name, size=0x18, alignment=8, sym_type=SymbolType.TYPE_OBJECT)
            name += "#func"

        if size == 0 and sym_type in (SymbolType.TYPE_NONE, SymbolType.TYPE_OBJECT, SymbolType.TYPE_TLS_OBJECT):
            self._warned_data_import.add(name)
            real_size = 8

        local_addr = self._allocate(real_size, alignment=alignment, thumb=thumb, tls=tls)
        if local_addr is None:
            if self._next_object is None:
                # we're at the end of the line. make a new extern object
                # this should only be hit if we're doing this outside a loading pass
                self._make_new_externs(real_size, alignment, tls)
            return self._next_object.make_extern(
                name, size=size, alignment=alignment, sym_type=sym_type, libname=libname
            )

        log.info("Created extern symbol for %s", name)

        new_symbol = SymbolCls(self, name, local_addr, size, sym_type)
        new_symbol.is_export = True
        new_symbol.is_extern = True

        if point_to is not None:
            new_symbol.pointto_name = point_to.name
            new_symbol.pointto_type = point_to.type
            new_symbol.pointto_precise = point_to

        self._symbol_cache[name] = new_symbol
        self.symbols.add(new_symbol)
        self._init_symbol(new_symbol)

        if make_toc:
            # write the pointer to the func into the toc
            # i.e. make a relocation for it
            # then if we're already mapped, apply the relocation manually
            reloc = TOCRelocation(self, toc_symbol, toc_symbol.relative_addr)
            reloc.resolve(new_symbol)
            self.relocs.append(reloc)
            if self._is_mapped:
                reloc.relocate()

            return toc_symbol
        return new_symbol

    def get_pseudo_addr(self, name) -> int:
        if not self._is_mapped:
            raise CLEError("Can't allocate with extern object before it is mapped")

        return self.make_extern(name).rebased_addr

    def allocate(self, size=1, alignment=8, thumb=False, tls=False) -> int:
        if not self._is_mapped:
            raise CLEError("Can't allocate with extern object before it is mapped")

        result = self._allocate(size=size, alignment=alignment, thumb=thumb, tls=tls)
        if result is None:
            if self._next_object is None:
                # we're at the end of the line. make a new extern object
                # this should only be hit if we're doing this outside a loading pass
                self._make_new_externs(size, alignment, tls)
            result = self._next_object.allocate(size=size, alignment=alignment, thumb=thumb, tls=tls)
        return result + (0 if tls else self.mapped_base)

    def _make_new_externs(self, size, alignment, tls):
        self._next_object = ExternObject(
            self.loader,
            map_size=max(size + alignment, 0x8000) if not tls else 0x8000,
            tls_size=max(size + alignment, 0x1000) if tls else 0x1000,
        )
        self._next_object._finalize_tls()
        self.loader._internal_load(self._next_object)

    def _allocate(self, size=1, alignment=8, thumb=False, tls=False):
        if tls:
            start = self.tls_next_addr
            limit = self.tls_data_size
        else:
            start = self.next_addr
            limit = self.map_size

        addr = ALIGN_UP(start, alignment) | thumb
        next_start = addr + size
        if next_start >= limit:
            if self._is_mapped:
                return None
            else:
                if tls:
                    self.tls_data_size += next_start - limit
                else:
                    self.map_size += next_start - limit

        if tls:
            self.tls_next_addr = next_start
            return addr
        else:
            self.next_addr = next_start
            return addr

    @property
    def max_addr(self):
        return AT.from_rva(self.map_size - 1, self).to_mva()

    def make_import(self, name, sym_type):
        if name not in self.imports:
            sym = Symbol(self, name, 0, 0, sym_type)
            sym.is_import = True
            sym.is_extern = True
            # this is kind of tricky... normally if you have an import and an export of the same name in the binary
            # the two symbols are *the same symbol*, usually with a copy relocation. but we don't know ahead of time
            # whether we will have the symbol here in externs, so we will not expose the import symbol to the rest of
            # the world.
            self._import_symbols[name] = sym
            return sym
        else:
            sym = self._import_symbols[name]
            if sym.type != sym_type:
                raise CLEOperationError(
                    "Created the same extern import %s with two different types. Something isn't right!"
                )
            return sym

    def _init_symbol(self, symbol):
        if isinstance(symbol, SimData):
            relocs = symbol.relocations()
            self.relocs.extend(relocs)

            if self._is_mapped:
                # TODO: is this right for tls?
                if symbol.type == SymbolType.TYPE_TLS_OBJECT:
                    self.memory.store(self.tls_block_size, symbol.value())
                else:
                    self.memory.store(symbol.relative_addr, symbol.value())

                for reloc in relocs:
                    reloc.relocate()
            else:
                self._delayed_writes.append(symbol)


class KernelObject(Backend):
    def __init__(self, loader, map_size=0x8000):
        super().__init__("cle##kernel", None, loader=loader)
        self.map_size = map_size
        self.set_arch(loader.main_object.arch)
        self.memory.add_backer(0, bytes(map_size))
        self.provides = "kernel space"
        self.pic = True

    def add_name(self, name, addr):
        self._symbol_cache[name] = Symbol(self, name, AT.from_mva(addr, self).to_rva(), 1, SymbolType.TYPE_FUNCTION)

    @property
    def max_addr(self):
        return AT.from_rva(self.map_size - 1, self).to_mva()


class PointToPrecise(PointTo):
    pointto_precise = None

    def relocations(self):
        return [
            SimDataSimpleRelocation(
                self.owner,
                self.pointto_precise,
                self.relative_addr,
                self.addend,
                preresolved=True,
            )
        ]
