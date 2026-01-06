from __future__ import annotations

import logging
from enum import Enum
from typing import TYPE_CHECKING

from cle.address_translator import AT

if TYPE_CHECKING:
    from .backend import Backend

log = logging.getLogger(name=__name__)

__all__ = [
    "SymbolType",
    "SymbolSubType",
    "Symbol",
]


class SymbolType(Enum):
    """
    ABI-agnostic symbol types
    """

    TYPE_OTHER = 0
    TYPE_NONE = 1
    TYPE_FUNCTION = 2
    TYPE_OBJECT = 3
    TYPE_SECTION = 4
    TYPE_TLS_OBJECT = 5
    TYPE_FUNCTION_OR_OBJECT = 6


class SymbolSubType(Enum):
    """
    Abstract base class for ABI-specific symbol types
    """

    def to_base_type(self) -> SymbolType:  # pylint: disable=no-self-use
        """
        A subclass' ABI-specific mapping to :SymbolType:
        """
        raise ValueError("Abstract base class SymbolSubType has no base_type")


class Symbol:
    """
    Representation of a symbol from a binary file. Smart enough to rebase itself.

    There should never be more than one Symbol instance representing a single symbol. To make sure of this, only use
    the :meth:`cle.backends.Backend.get_symbol()` to create new symbols.

    :ivar owner:            The object that contains this symbol
    :vartype owner:         cle.backends.Backend
    :ivar str name:         The name of this symbol
    :ivar int addr:         The un-based address of this symbol, an RVA
    :ivar int size:         The size of this symbol
    :ivar _type:            The ABI-agnostic type of this symbol
    :ivar bool resolved:    Whether this import symbol has been resolved to a real symbol
    :ivar resolvedby:       The real symbol this import symbol has been resolve to
    :vartype resolvedby:    None or cle.backends.Symbol
    :ivar str resolvewith:  The name of the library we must use to resolve this symbol, or None if none is required.
    """

    def __init__(self, owner: Backend, name: str, relative_addr: int, size: int, sym_type: SymbolType):
        """
        Not documenting this since if you try calling it, you're wrong.
        """
        self.owner: Backend = owner
        self.name = name
        self.relative_addr = relative_addr
        self.size = size
        self._type: SymbolType = SymbolType(sym_type)
        self.resolved = False
        self.resolvedby = None

    def __repr__(self):
        if self.is_import:
            return f'<Symbol "{self.name}" in {self.owner.binary_basename} (import)>'
        else:
            return f'<Symbol "{self.name}" in {self.owner.binary_basename} at {self.rebased_addr:#x}>'

    def resolve(self, obj):
        self.resolved = True
        self.resolvedby = obj
        self.owner.resolved_imports.append(self)

    @property
    def type(self) -> SymbolType:
        """
        The ABI-agnostic SymbolType. Must be overridden by derived types.
        """
        return self._type

    @property
    def subtype(self) -> SymbolSubType:
        """
        A subclass' ABI-specific types
        """
        raise ValueError("Base class Symbol has no subtype")

    @property
    def rebased_addr(self):
        """
        The address of this symbol in the global memory space
        """
        return AT.from_rva(self.relative_addr, self.owner).to_mva()

    @property
    def linked_addr(self):
        return AT.from_rva(self.relative_addr, self.owner).to_lva()

    @property
    def is_function(self):
        """
        Whether this symbol is a function
        """
        return self.type in {SymbolType.TYPE_FUNCTION, SymbolType.TYPE_FUNCTION_OR_OBJECT}

    # These may be overridden in subclasses
    is_static = False
    is_common = False
    is_import = False
    is_export = False
    is_local = False
    is_weak = False
    is_extern = False
    is_forward = False

    def resolve_forwarder(self):
        """
        If this symbol is a forwarding export, return the symbol the forwarding refers to, or None if it cannot be found
        """
        return self

    # compatibility layer

    _complained_owner = False

    @property
    def owner_obj(self):
        if not Symbol._complained_owner:
            Symbol._complained_owner = True
            log.critical("Deprecation warning: use symbol.owner instead of symbol.owner_obj")
        return self.owner

    def __getstate__(self):
        return {k: v for k, v in self.__dict__.items() if k != "owner"}

    def __setstate__(self, state):
        self.__dict__.update(state)
