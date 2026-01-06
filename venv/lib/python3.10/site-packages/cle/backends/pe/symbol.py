from __future__ import annotations

import logging

from cle.backends.symbol import Symbol, SymbolType

log = logging.getLogger(name=__name__)


class WinSymbol(Symbol):
    """
    Represents a symbol for the PE format.
    """

    def __init__(
        self, owner, name, addr, is_import, is_export, ordinal_number, forwarder, symbol_type=SymbolType.TYPE_FUNCTION
    ):
        super().__init__(owner, name, addr, owner.arch.bytes, symbol_type)
        self.is_import = is_import
        self.is_export = is_export
        self.ordinal_number = ordinal_number
        self.forwarder = forwarder
        self.is_forward = forwarder is not None

    def resolve_forwarder(self):
        if self.resolvedby is not None:
            return self.resolvedby

        sym = self
        seen = set()
        while sym is not None and sym.is_forward and sym.forwarder is not None:  # FORWARDING
            if sym.forwarder in seen:
                log.warning("Infinite forwarding loop for %s", self)
                return None
            seen.add(sym.forwarder)
            owner_name, name = sym.forwarder.split(".", 1)
            owner = self.owner.loader.find_object(owner_name)
            if owner is None:
                return None
            sym = owner.get_symbol(name)

        self.resolvedby = sym
        return sym
