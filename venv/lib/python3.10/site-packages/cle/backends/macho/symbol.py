# This file is part of Mach-O Loader for CLE.
# Contributed December 2016 by Fraunhofer SIT (https://www.sit.fraunhofer.de/en/) and updated in September 2019.
from __future__ import annotations

import logging
from typing import TYPE_CHECKING

from cle import AT
from cle.backends.backend import Backend
from cle.backends.symbol import Symbol, SymbolType

if TYPE_CHECKING:
    from . import MachO

log = logging.getLogger(name=__name__)

# some constants:
SYMBOL_TYPE_UNDEF = 0x0
SYMBOL_TYPE_ABS = 0x2
SYMBOL_TYPE_SECT = 0xE
SYMBOL_TYPE_PBUD = 0xC
SYMBOL_TYPE_INDIR = 0xA
N_STAB = 0xE0  # actually a mask
N_EXT = 0x01  # external symbol bit

LIBRARY_ORDINAL_SELF = 0x0
LIBRARY_ORDINAL_OLD_MAX = 0xFE
LIBRARY_ORDINAL_DYN_LOOKUP = 0xFE

BIND_SPECIAL_DYLIB_SELF = 0x0
BIND_SPECIAL_DYLIB_WEAK_LOOKUP = 0xFD
BIND_SPECIAL_DYLIB_FLAT_LOOKUP = 0xFE
BIND_SPECIAL_DYLIB_MAIN_EXECUTABLE = 0xFF  # technically -1


class AbstractMachOSymbol(Symbol):
    """
    Base class for Mach-O symbols.
    Defines the minimum common properties all types of mach-o symbols must have
    """

    owner: MachO  # type: ignore[override]

    def __init__(self, owner: Backend, name: str, relative_addr: int, size: int, sym_type: SymbolType):
        super().__init__(owner, name, relative_addr, size, sym_type)

        # additional properties
        self.bind_xrefs = []  # XREFs discovered during binding of the symbol
        # starting addresses of stubs that resolve to this symbol - note that this
        # must be obtained through an analysis of some sort
        self.symbol_stubs = []

    @property
    def library_ordinal(self):
        return None

    @property
    def is_stab(self) -> bool:
        return False

    @property
    def library_name(self) -> str | None:
        return None

    @property
    def library_base_name(self) -> str | None:
        full_name = self.library_name
        if full_name is None:
            return None

        return full_name.rsplit("/", 1)[-1]


class SymbolTableSymbol(AbstractMachOSymbol):
    """
    "Regular" symbol. Made to be (somewhat) compatible with backends.Symbol.
    A SymbolTableSymbol is an entry in the binary's symbol table.

    Note that ELF-specific fields from backends.Symbol are not used and semantics of the remaining fields differ in
    many cases. As a result most stock functionality from Angr and related libraries WILL NOT WORK PROPERLY on
    MachOSymbol.

    Much of the code below is based on heuristics as official documentation is sparse, consider yourself warned!

    The relevant struct with documentation is nlist_64 defined in mach-o/nlist.h

    """

    def __init__(self, owner: MachO, symtab_offset, n_strx, n_type, n_sect, n_desc, n_value):
        # Note 1: Setting size = owner.arch.bytes has been directly taken over from the PE backend,
        # there is no meaningful definition of a symbol's size so I assume the size of an address counts here
        # Note 2: relative_addr will be the address of a symbols __got or __nl_symbol_ptr entry, not the addr of a stub
        # pointing to the symobl.
        # Stub addresses must be obtained through some sort of higher-level analysis
        # Note 3: A symbols name may not be unique!
        # Note 4: The symbol type of all symbols is SymbolType.TYPE_OTHER
        # because without docs I was unable to proplerly map Mach-O symbol types to CLE's notion of a symbol type

        # store the mach-o properties, all these are raw values straight from the binary
        self.symtab_offset = symtab_offset  # offset from the start of the symbol table
        self.n_type = n_type  # n_type field from the symbol table
        self.n_sect = n_sect  # n_sect field from the symbol table
        self.n_desc = n_desc  # n_desc  field from the symbol table
        self.n_value = n_value  # n_value field from the symbol table.
        self.n_strx = n_strx  # index into the string table

        # The meaning of n_value isn't always an address, and the logic for this is complicated
        # For now this is mostly a heuristic
        if n_value == 0:
            # This symbol doesn't have an address
            addr = 0
        elif n_value == 0x5614542:
            # This is the radr://5614542 symbol
            # The logic around this is somewhat explained here:
            # https://opensource.apple.com/source/cctools/cctools-782/misc/strip.c
            # The addr isn't really an address, but a magic value that is used to indicate that the symbol is
            addr = 0x5614542
            # addr = 0
        else:
            # The n_value is probably an address, but we need to convert it to a relative address
            addr = AT.from_lva(n_value, owner).to_rva()

        # now we may call super
        # however we cannot access any properties yet that would touch superclass-initialized attributes
        # so we have to repeat some work
        super().__init__(
            owner,
            owner.get_string(n_strx).decode("utf-8") if n_strx != 0 else "",
            addr,
            owner.arch.bytes,
            SymbolType.TYPE_OTHER,
        )

        # set further fields
        self.is_import = (
            self.sym_type == SYMBOL_TYPE_UNDEF and self.is_external and self.library_ordinal != LIBRARY_ORDINAL_SELF
        )
        self.is_export = self.name in self.owner.exports_by_name

        if self.is_export or self.is_stab or self.is_local:
            self._type = SymbolType.TYPE_FUNCTION_OR_OBJECT

    @property
    def library_name(self) -> str | None:
        if self.is_import:
            if LIBRARY_ORDINAL_DYN_LOOKUP == self.library_ordinal:
                log.warning("LIBRARY_ORDINAL_DYN_LOOKUP found, cannot handle")
                return None
            else:
                return self.owner.imported_libraries[self.library_ordinal]
        return None

    @property
    def segment_name(self):
        if self.sym_type == SYMBOL_TYPE_SECT:
            return self.owner.sections_by_ordinal[self.n_sect].segname
        else:
            return None

    @property
    def section_name(self):
        if self.sym_type == SYMBOL_TYPE_SECT:
            return self.owner.sections_by_ordinal[self.n_sect].sectname
        else:
            return None

    @property
    def value(self):
        if self.sym_type == SYMBOL_TYPE_INDIR:
            return 0
        else:
            return self.n_value

    @property
    def referenced_symbol_index(self):
        """For indirect symbols n_value contains an index into the string table indicating the referenced
        symbol's name"""
        if self.sym_type == SYMBOL_TYPE_INDIR:
            return self.n_value
        else:
            return None

    def is_weak(self):
        # compare https://developer.apple.com/library/mac/documentation/DeveloperTools/Conceptual/MachOTopics/1-Articles/executing_files.html
        return self.is_weak_referenced

    @property
    def is_function(self):
        # Incompatibility to CLE
        log.debug("It is not possible to decide wether a symbol is a function or not for MachOSymbols")
        return False

    # real symbols have properties, mach-o symbols have plenty of them:
    @property
    def is_stab(self) -> bool:
        return (self.n_type & N_STAB) != 0

    @property
    def is_private_external(self) -> bool:
        return (self.n_type & 0x10) == 0x10

    @property
    def is_external(self) -> bool:
        return (self.n_type & N_EXT) == 0x01

    @property
    def sym_type(self):  # cannot be called "type" as that shadows a builtin variable from Symbol
        if self.is_stab:
            return self.n_type
        else:
            return self.n_type & 0x0E

    @property
    def is_common(self):
        return self.is_external and self.sym_type == SYMBOL_TYPE_UNDEF and self.n_value != 0

    @property
    def common_align(self):
        return None if not self.is_common else ((self.n_desc) >> 8) & 0x0F

    @property
    def reference_type(self):
        return self.n_desc & 0x7 if self.sym_type == SYMBOL_TYPE_UNDEF else None

    @property
    def library_ordinal(self):
        return ((self.n_desc) >> 8) & 0xFF

    @property
    def is_no_dead_strip(self):
        return self.n_desc & 0x0020

    @property
    def is_desc_discarded(self):
        return self.is_no_dead_strip

    @property
    def is_weak_referenced(self):
        return self.n_desc & 0x0040

    @property
    def is_weak_defined(self):
        return self.n_desc & 0x0080

    @property
    def is_reference_to_weak(self):
        return self.n_desc & 0x0080

    @property
    def is_thumb_definition(self):
        return self.n_desc & 0x0008

    @property
    def is_symbol_resolver(self):
        return self.n_desc & 0x0100

    @property
    def is_alt_entry(self):
        return self.n_desc & 0x0200


class DyldBoundSymbol(AbstractMachOSymbol):
    """
    The new kind of symbol handling introduced with ios15
    """

    owner: MachO

    def __init__(self, owner, name, lib_ordinal):
        """Based on the constructor of BindingSymbol"""

        # store the mach-o properties, all these are raw values straight from the binary
        self.lib_ordinal = lib_ordinal

        super().__init__(owner, name, 0, owner.arch.bytes, SymbolType.TYPE_OTHER)

        # set further fields
        self.is_import = True  # TODO: this is always an import for now
        # with ios15 came a new load command LC_DYLD_EXPORTS_TRIE
        # this isn't handled yet, so for now no symbol generated by the new dyld handling is an export
        self.is_export = False

    @property
    def library_name(self) -> str | None:
        if BIND_SPECIAL_DYLIB_FLAT_LOOKUP == self.lib_ordinal:
            log.warning("BIND_SPECIAL_DYLIB_FLAT_LOOKUP found, cannot handle")
            return None
        elif BIND_SPECIAL_DYLIB_WEAK_LOOKUP == self.lib_ordinal:
            return None
        try:
            return self.owner.imported_libraries[self.lib_ordinal]
        except IndexError:
            log.error(
                "Symbol %s has library ordinal %d, but there are only %d imported libraries",
                self,
                self.lib_ordinal,
                len(self.owner.imported_libraries),
            )
            return None

    @property
    def is_function(self):
        # Incompatibility to CLE
        log.debug("It is not possible to decide wether a symbol is a function or not for MachOSymbols")
        return False

    def demangled_name(self):
        return self.name  # it is not THAT easy with Mach-O

    @property
    def library_ordinal(self):
        return self.lib_ordinal


class BindingSymbol(AbstractMachOSymbol):
    """
    "Binding" symbol. Made to be (somewhat) compatible with backends.Symbol.
    A BindingSymbol is an imported symbol discovered during the binding process.

    Note that ELF-specific fields from backends.Symbol are not used and semantics of the remaining fields differ in
    many cases. As a result most stock functionality from Angr and related libraries WILL NOT WORK PROPERLY on
    MachOSymbol.

    Much of the code below is based on heuristics as official documentation is sparse, consider yourself warned!
    """

    def __init__(self, owner, name, lib_ordinal):
        # Note 1: Setting size = owner.arch.bytes has been directly taken over from the PE backend,
        # there is no meaningful definition of a symbol's size so I assume the size of an address counts here
        # Note 2: relative_addr will be the address of a symbols __got or __nl_symbol_ptr entry, not the addr of a stub
        # pointing to the symobl.
        # Stub addresses must be obtained through some sort of higher-level analysis
        # Note 3: A symbols name may not be unique!
        # Note 4: The symbol type of all symbols is SymbolType.TYPE_OTHER because without docs I was unable to problerly
        # map Mach-O symbol types to CLE's notion of a symbol type

        # store the mach-o properties, all these are raw values straight from the binary
        self.lib_ordinal = lib_ordinal

        # now we may call super
        # however we cannot access any properties yet that would touch superclass-initialized attributes
        # so we have to repeat some work
        super().__init__(owner, name, 0, owner.arch.bytes, SymbolType.TYPE_OTHER)

        # set further fields
        self.is_import = True  # this is always an import
        self.is_export = self.name in self.owner.exports_by_name

    @property
    def library_name(self) -> str | None:
        if LIBRARY_ORDINAL_DYN_LOOKUP == self.lib_ordinal:
            log.warning("LIBRARY_ORDINAL_DYN_LOOKUP found, cannot handle")
            return None

        return self.owner.imported_libraries[self.lib_ordinal]

    @property
    def is_function(self):
        # Incompatibility to CLE
        log.debug("It is not possible to decide wether a symbol is a function or not for MachOSymbols")
        return False

    def demangled_name(self):
        return self.name  # it is not THAT easy with Mach-O

    @property
    def library_ordinal(self):
        return self.lib_ordinal


__all__ = [
    "SymbolType",
    "AbstractMachOSymbol",
    "SymbolTableSymbol",
    "BindingSymbol",
    "DyldBoundSymbol",
]
