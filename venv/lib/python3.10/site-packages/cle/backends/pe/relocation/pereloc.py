from __future__ import annotations

import logging

from cle.backends.relocation import Relocation

log = logging.getLogger(name=__name__)


# Reference: https://msdn.microsoft.com/en-us/library/ms809762.aspx
class PEReloc(Relocation):
    AUTO_HANDLE_NONE = True

    def __init__(self, owner, symbol, addr, resolvewith=None):  # pylint: disable=unused-argument
        super().__init__(owner, symbol, addr)

        self.resolvewith = resolvewith
        if self.resolvewith is not None:
            self.resolvewith = self.resolvewith.lower()

    def resolve_symbol(self, solist, bypass_compatibility=False, extern_object=None, **kwargs):
        if not bypass_compatibility:
            solist = [x for x in solist if self.resolvewith == x.provides]
        super().resolve_symbol(solist, bypass_compatibility=bypass_compatibility, extern_object=extern_object, **kwargs)

        if self.resolvedby is None:
            return

        # handle symbol forwarders
        newsym = self.resolvedby.resolve_forwarder()
        if newsym is None:
            new_symbol = extern_object.make_extern(self.symbol.name, sym_type=self.symbol.type)
            self.resolvedby.resolvedby = new_symbol
            self.resolve(new_symbol)
            return

        self.resolvedby = newsym
        self.symbol.resolvedby = newsym

    def relocate(self):
        if self.symbol is None:  # relocation described in the DIRECTORY_ENTRY_BASERELOC table
            value = self.value
            if value is None:
                log.debug("Unresolved relocation with no symbol.")
                return
            self.owner.memory.store(self.relative_addr, value)
        else:
            super().relocate()

    @property
    def value(self):
        if self.resolved:
            return self.resolvedby.rebased_addr

    @property
    def is_base_reloc(self):
        """
        These relocations are ignored by the linker if the executable
        is loaded at its preferred base address. There is no associated
        symbol with base relocations.
        """
        return True if self.symbol is None else False

    @property
    def is_import(self):
        return not self.is_base_reloc
