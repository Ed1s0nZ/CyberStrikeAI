from __future__ import annotations

import logging

from cle.address_translator import AT
from cle.backends.symbol import SymbolType
from cle.errors import CLEInvalidBinaryError, CLEOperationError

from .elfreloc import ELFReloc

log = logging.getLogger(name=__name__)


__all__ = [
    "GenericTLSDoffsetReloc",
    "GenericTLSOffsetReloc",
    "GenericTLSDescriptorReloc",
    "GenericTLSModIdReloc",
    "GenericIRelativeReloc",
    "GenericAbsoluteAddendReloc",
    "GenericPCRelativeAddendReloc",
    "GenericJumpslotReloc",
    "GenericRelativeReloc",
    "GenericAbsoluteReloc",
    "GenericCopyReloc",
    "MipsGlobalReloc",
    "MipsLocalReloc",
    "RelocTruncate32Mixin",
    "RelocGOTMixin",
]


class GenericTLSDoffsetReloc(ELFReloc):
    @property
    def value(self):
        return self.addend + self.symbol.relative_addr

    def resolve_symbol(self, solist, **kwargs):  # pylint: disable=unused-argument
        self.resolve(None)
        return True


class GenericTLSOffsetReloc(ELFReloc):
    AUTO_HANDLE_NONE = True

    def relocate(self):
        hell_offset = self.owner.arch.elf_tls.tp_offset

        if self.resolvedby is None:
            obj = self.owner
            addr = 0
        else:
            obj = self.resolvedby.owner
            addr = self.resolvedby.relative_addr

        if obj.tls_block_offset is None:
            raise CLEInvalidBinaryError("Illegal relocation - dynamically loaded object using static TLS")

        self.owner.memory.pack_word(self.relative_addr, obj.tls_block_offset + self.addend + addr - hell_offset)


class GenericTLSDescriptorReloc(ELFReloc):
    # Going VERY far out on a limb here
    # "TLS descriptors" are a thing I'm seeing in aarch64 binaries which seem to want to relocate by
    # sticking a pointer to a resolver function followed by some arbitrary data. The resolver function
    # is passed a pointer to the descriptor. My guess is the resolver is supposed to basically perform
    # _tls_get_addr, but the intention is probably to make it possible to work with dynamically loaded objects.

    RESOLVER_ADDR: int = NotImplemented
    AUTO_HANDLE_NONE = True

    def relocate(self):
        if self.resolvedby is None:
            obj = self.owner
        else:
            obj = self.resolvedby.owner

        if obj.tls_block_offset is None:
            raise CLEInvalidBinaryError("Illegal relocation? - dynamically loaded object using static TLS? Maybe?")

        self.owner.memory.pack_word(self.relative_addr, self.RESOLVER_ADDR)
        self.owner.memory.pack_word(
            self.relative_addr + self.arch.bytes, obj.tls_block_offset + self.addend + self.symbol.relative_addr
        )  # Should this include the hell offset?


class GenericTLSModIdReloc(ELFReloc):
    AUTO_HANDLE_NONE = True

    def relocate(self):
        if self.symbol.type == SymbolType.TYPE_NONE:
            obj = self.owner
        else:
            obj = self.resolvedby.owner

        self.owner.memory.pack_word(self.relative_addr, obj.tls_module_id)


class GenericIRelativeReloc(ELFReloc):
    AUTO_HANDLE_NONE = True

    def relocate(self):
        if self.symbol.type == SymbolType.TYPE_NONE:
            self.owner.irelatives.append((AT.from_lva(self.addend, self.owner).to_mva(), self.relative_addr))
        else:
            self.owner.irelatives.append((self.resolvedby.rebased_addr, self.relative_addr))


class GenericAbsoluteAddendReloc(ELFReloc):
    @property
    def value(self):
        return self.resolvedby.rebased_addr + self.addend


class GenericPCRelativeAddendReloc(ELFReloc):
    @property
    def value(self):
        return self.resolvedby.rebased_addr + self.addend - self.rebased_addr


class GenericJumpslotReloc(ELFReloc):
    @property
    def value(self):
        if self.is_rela:
            return self.resolvedby.rebased_addr + self.addend
        else:
            return self.resolvedby.rebased_addr


class GenericRelativeReloc(ELFReloc):
    AUTO_HANDLE_NONE = True

    @property
    def value(self):
        if self.resolvedby is not None:
            return self.resolvedby.rebased_addr
        return self.owner.mapped_base + self.addend


class GenericAbsoluteReloc(ELFReloc):
    @property
    def value(self):
        return self.resolvedby.rebased_addr


class GenericCopyReloc(ELFReloc):
    def resolve_symbol(self, solist, **kwargs):
        new_solist = [x for x in solist if x is not self.owner]
        super().resolve_symbol(new_solist, **kwargs)

    def relocate(self):
        if self.resolvedby.size != self.symbol.size and (self.resolvedby.size != 0 or not self.resolvedby.is_extern):
            log.error("Export symbol is different size than import symbol for copy relocation: %s", self.symbol.name)
        else:
            self.owner.memory.store(
                self.relative_addr,
                self.resolvedby.owner.memory.load(self.resolvedby.relative_addr, self.resolvedby.size),
            )
        return True


class MipsGlobalReloc(GenericAbsoluteReloc):
    pass


class MipsLocalReloc(ELFReloc):
    AUTO_HANDLE_NONE = True

    def resolve_symbol(self, solist, **kwargs):
        self.resolve(None)

    def relocate(self):
        if self.owner.mapped_base == 0:
            return  # don't touch local relocations on the main bin

        delta = self.owner.mapped_base - self.owner._dynamic["DT_MIPS_BASE_ADDRESS"]
        if delta == 0:
            return

        val = self.owner.memory.unpack_word(self.relative_addr)
        newval = val + delta
        self.owner.memory.pack_word(self.relative_addr, newval)


class RelocTruncate32Mixin:
    """
    A mix-in class for relocations that cover a 32-bit field regardless of the architecture's address word length.
    """

    # If True, 32-bit truncated value must equal to its original when zero-extended
    check_zero_extend = False

    # If True, 32-bit truncated value must equal to its original when sign-extended
    check_sign_extend = False

    def relocate(self):
        arch_bits = self.owner.arch.bits
        assert arch_bits >= 32  # 16-bit makes no sense here

        val = self.value % (2**arch_bits)  # we must truncate it to native range first

        if (
            self.check_zero_extend
            and val >> 32 != 0
            or self.check_sign_extend
            and val >> 32 != ((1 << (arch_bits - 32)) - 1)
            if ((val >> 31) & 1) == 1
            else 0
        ):
            raise CLEOperationError(
                f"relocation truncated to fit: {self.__class__.__name__}; consider making"
                " relevant addresses fit in the 32-bit address space."
            )

        self.owner.memory.pack_word(self.dest_addr, val, size=4, signed=False)

        return True


class RelocGOTMixin:
    """
    A mix-in class which will cause the symbol to be resolved to a pointer to the symbol instead of the symbol
    """

    def resolve(self, symbol, extern_object=None):
        assert extern_object is not None, "I have no idea how this would happen"

        got_symbol = extern_object.make_extern(f"got.{symbol.name}", sym_type=SymbolType.TYPE_OBJECT, point_to=symbol)
        super().resolve(got_symbol)
