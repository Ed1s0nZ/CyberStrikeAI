from __future__ import annotations

import logging
from typing import TYPE_CHECKING

from cle.address_translator import AT

from .symbol import Symbol, SymbolType

if TYPE_CHECKING:
    from typing import Any

    from .backend import Backend


log = logging.getLogger(name=__name__)


class Relocation:
    """
    A representation of a relocation in a binary file. Smart enough to
    relocate itself.

    :ivar owner:            The binary this relocation was originally found in, as a cle object
    :ivar symbol:           The Symbol object this relocation refers to
    :ivar relative_addr:    The address in owner this relocation would like to write to
    :ivar resolvedby:       If the symbol this relocation refers to is an import symbol and that import has been
                            resolved, this attribute holds the symbol from a different binary that was used to resolve
                            the import.
    :ivar resolved:         Whether the application of this relocation was successful
    """

    def __init__(self, owner: Backend, symbol: Symbol | None, relative_addr: int):
        self.owner = owner
        self.arch = owner.arch
        self.symbol = symbol
        self.relative_addr = relative_addr
        self.resolvedby: Symbol | None = None
        self.resolved: bool = False
        self.resolvewith = None
        if self.symbol is not None and self.symbol.is_import:
            self.owner.imports[self.symbol.name] = self

    AUTO_HANDLE_NONE = False

    def resolve_symbol(
        self, solist: list[Any], thumb=False, extern_object=None, **kwargs
    ):  # pylint: disable=unused-argument
        if self.resolved:
            return

        if (self.symbol is None or self.symbol.type == SymbolType.TYPE_NONE) and self.AUTO_HANDLE_NONE:
            self.resolve(None, extern_object=extern_object)
            return

        if self.symbol.is_static or self.symbol.is_local:
            # A static or local symbol should only be resolved by itself.
            self.resolve(self.symbol, extern_object=extern_object)
            return

        weak_result = None
        for so in solist:
            symbol = so.get_symbol(self.symbol.name)
            if symbol is not None and symbol.is_export:
                if not symbol.is_weak:
                    self.resolve(symbol, extern_object=extern_object)
                    return
                elif weak_result is None:
                    weak_result = symbol
            # TODO: Was this check obsolted by the addition of is_static?
            # I think right now symbol.is_import = !symbol.is_export
            elif symbol is not None and not symbol.is_import and so is self.owner:
                if not symbol.is_weak:
                    self.resolve(symbol, extern_object=extern_object)
                    return
                elif weak_result is None:
                    weak_result = symbol

        if weak_result is not None:
            self.resolve(weak_result, extern_object=extern_object)
            return

        if self.symbol.is_weak:
            return

        new_symbol = extern_object.make_extern(self.symbol.name, sym_type=self.symbol._type, thumb=thumb)
        self.resolve(new_symbol, extern_object=extern_object)

    def resolve(self, obj, extern_object=None):  # pylint: disable=unused-argument
        self.resolvedby = obj
        self.resolved = True
        if self.symbol is not None:
            if obj is not None:
                log.debug(
                    "%s from %s resolved by %s from %s at %#x",
                    self.symbol.name,
                    self.owner.provides,
                    obj.name,
                    obj.owner.provides,
                    obj.rebased_addr,
                )
            self.symbol.resolve(obj)
        else:
            if not self.AUTO_HANDLE_NONE:
                raise TypeError("Programming error: resolving symbol with None but does not have CAN_HANDLE_NONE set")

    @property
    def rebased_addr(self):
        """
        The address in the global memory space this relocation would like to write to
        """
        return AT.from_rva(self.relative_addr, self.owner).to_mva()

    @property
    def linked_addr(self):
        return AT.from_rva(self.relative_addr, self.owner).to_lva()

    @property
    def dest_addr(self):
        return self.relative_addr

    @property
    def value(self) -> int:  # pylint: disable=no-self-use
        log.error("Value property of Relocation must be overridden by subclass!")
        return 0

    def relocate(self):
        """
        Applies this relocation. Will make changes to the memory object of the
        object it came from.

        This implementation is a generic version that can be overridden in subclasses.
        """
        if not self.resolved:
            return False

        self.owner.memory.pack_word(self.dest_addr, self.value)
        return True

    # compatibility layer

    _complained_owner = False

    @property
    def owner_obj(self):
        if not Relocation._complained_owner:
            Relocation._complained_owner = True
            log.critical("Deprecation warning: use relocation.owner instead of relocation.owner_obj")
        return self.owner
