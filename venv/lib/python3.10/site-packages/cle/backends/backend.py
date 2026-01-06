from __future__ import annotations

import hashlib
import logging
import os
from io import BufferedReader
from typing import TYPE_CHECKING, Any, BinaryIO

import archinfo
import sortedcontainers

from cle.address_translator import AT
from cle.errors import CLEError, CLEOperationError
from cle.memory import Clemory

from .regions import Regions
from .relocation import Relocation
from .symbol import Symbol

if TYPE_CHECKING:
    from cle.backends import Section, Segment
    from cle.loader import Loader

log = logging.getLogger(name=__name__)


class FunctionHintSource:
    """
    Enums that describe the source of function hints.
    """

    EH_FRAME = 0
    EXTERNAL_EH_FRAME = 1


class FunctionHint:
    """
    Describes a function hint.

    :ivar int addr:     Address of the function.
    :ivar int size:     Size of the function.
    :ivar source:       Source of this hint.
    :vartype source:    int
    """

    __slots__ = ("addr", "size", "source")

    def __init__(self, addr, size, source):
        self.addr = addr
        self.size = size
        self.source = source

    def __repr__(self):
        return f"<FuncHint@{self.addr:#x}, {self.size} bytes>"


class ExceptionHandling:
    """
    Describes an exception handling.

    Exception handlers are usually language-specific. In C++, it is usually implemented as try {} catch {} blocks.

    :ivar int start_addr:               The beginning of the try block.
    :ivar int size:                     Size of the try block.
    :ivar Optional[int] handler_addr:   Address of the exception handler code.
    :ivar type:                         Type of the exception handler. Optional.
    :ivar Optional[int] func_addr:      Address of the function. Optional.
    """

    __slots__ = (
        "start_addr",
        "size",
        "handler_addr",
        "type",
        "func_addr",
    )

    def __init__(self, start_addr, size, handler_addr=None, type_=None, func_addr=None):
        self.start_addr = start_addr
        self.size = size
        self.handler_addr = handler_addr
        self.type = type_
        self.func_addr = func_addr

    def __repr__(self):
        if self.handler_addr is not None:
            return (
                f"<ExceptionHandling@{self.start_addr:#x}-{self.start_addr + self.size:#x}: "
                f"handler@{self.handler_addr:#x}>"
            )
        else:
            return f"<ExceptionHandling@{self.start_addr:#x}-{self.start_addr + self.size:#x}: no handler>"


class Backend:
    """
    Main base class for CLE binary objects.

    An alternate interface to this constructor exists as the static method :meth:`cle.loader.Loader.load_object`

    :ivar binary:           The path to the file this object is loaded from
    :ivar binary_basename:  The basename of the filepath, or a short representation of the stream it was loaded from
    :ivar is_main_bin:      Whether this binary is loaded as the main executable
    :ivar segments:         A listing of all the loaded segments in this file
    :ivar sections:         A listing of all the demarked sections in the file
    :ivar sections_map:     A dict mapping from section name to section
    :ivar imports:          A mapping from symbol name to import relocation
    :ivar resolved_imports: A list of all the import symbols that are successfully resolved
    :ivar relocs:           A list of all the relocations in this binary
    :ivar irelatives:       A list of tuples representing all the irelative relocations that need to be performed. The
                            first item in the tuple is the address of the resolver function, and the second item is the
                            address of where to write the result. The destination address is an RVA.
    :ivar jmprel:           A mapping from symbol name to the address of its jump slot relocation, i.e. its GOT entry.
    :ivar arch:             The architecture of this binary
    :vartype arch:          archinfo.arch.Arch
    :ivar str os:           The operating system this binary is meant to run under
    :ivar int mapped_base:  The base address of this object in virtual memory
    :ivar deps:             A list of names of shared libraries this binary depends on
    :ivar linking:          'dynamic' or 'static'
    :ivar linked_base:      The base address this object requests to be loaded at
    :ivar bool pic:         Whether this object is position-independent
    :ivar bool execstack:   Whether this executable has an executable stack
    :ivar str provides:     The name of the shared library dependancy that this object resolves
    :ivar list symbols:     A list of symbols provided by this object, sorted by address
    :ivar has_memory:       Whether this backend is backed by a Clemory or not. As it stands now, a backend should still
                            define `min_addr` and `max_addr` even if `has_memory` is False.
    """

    is_default = False
    is_outer = False

    def __init__(
        self,
        binary,
        binary_stream,
        loader=None,
        is_main_bin=False,
        entry_point=None,
        arch=None,
        base_addr=None,
        force_rebase=False,
        has_memory=True,
        **kwargs,
    ):
        """
        :param binary:          The path to the binary to load
        :param binary_stream:   The open stream to this binary. The reference to this will be held until you call close.
        :param is_main_bin:     Whether this binary should be loaded as the main executable
        """

        self.load_args: dict[str, Any] = {} | kwargs
        self.set_load_args(
            loader=loader,
            is_main_bin=is_main_bin,
            entry_point=entry_point,
            arch=arch,
            base_addr=base_addr,
            force_rebase=force_rebase,
            has_memory=has_memory,
        )
        self.binary = binary
        self._binary_stream: BufferedReader = binary_stream
        if self.binary is not None:
            self.binary_basename = os.path.basename(self.binary)
        elif hasattr(self._binary_stream, "name"):
            self.binary_basename = os.path.basename(self._binary_stream.name)
        else:
            self.binary_basename = str(self._binary_stream)
        # if the backend unpacks another file, this field will hold the name of the file
        self.unpacked_name: str | None = None

        for k in list(kwargs.keys()):
            if k == "custom_entry_point":
                entry_point = kwargs.pop(k)
            elif k == "custom_arch":
                arch = kwargs.pop(k)
            elif k == "custom_base_addr":
                base_addr = kwargs.pop(k)
            else:
                continue
            log.critical("Deprecation warning: the %s parameter has been renamed to %s", k, k[7:])

        if kwargs != {}:
            log.warning("Unused kwargs for loading binary %s: %s", self.binary, ", ".join(kwargs.keys()))

        self.is_main_bin = is_main_bin
        self.has_memory = has_memory
        self._loader: Loader | None = loader
        self._entry = 0
        self._segments: Regions[Segment] = Regions()  # List of segments
        self._sections: Regions[Section] = Regions()  # List of sections
        self.sections_map = {}  # Mapping from section name to section
        self.symbols = sortedcontainers.SortedKeyList(key=self._get_symbol_relative_addr)
        self.imports: dict[str, Relocation] = {}
        self.resolved_imports = []
        self.relocs: list[Relocation] = []
        self.irelatives = []  # list of tuples (resolver, destination), dest w/o rebase
        self.jmprel = {}
        self._arch: archinfo.Arch | None = None
        self.os = None  # Let other stuff override this
        self.compiler = None, None  # compiler name, version
        self._symbol_cache = {}
        # a list of directories to search for libraries specified by the object
        self.extra_load_path = []
        # attributes to enable SimProcedure guessing
        self.guess_simprocs = False
        self.guess_simprocs_hint = None
        # if we want one of our children to be the main object of the loader, we set this field to the child, and the
        # loader will pick it up
        self.force_main_object = None

        # checksums
        self.md5 = None
        self.sha256 = None

        self.mapped_base_symbolic = 0
        # These are set by cle, and should not be overriden manually
        self.mapped_base = self.linked_base = 0  # not to be set manually - used by CLE

        self.deps = []  # Needed shared objects (libraries dependencies)
        self.child_objects: list[Backend] = []  # any objects loaded directly out of this
        self.parent_object = None
        self.linking = None  # Dynamic or static linking
        self.pic = force_rebase
        self.execstack = False
        self.aslr = False

        # tls info set by backend to communicate with thread manager
        self.tls_used = False
        self.tls_block_size = None
        self.tls_data_size = None
        self.tls_data_start = None
        # tls info set by thread manager
        self.tls_module_id = None
        # self.tls_block_offset = None  # this is an ELF-only attribute

        # exception handling
        # they should be rebased when .rebase() is called
        self.exception_handlings: list[ExceptionHandling] = []

        # Hints
        # they should be rebased when .rebase() is called
        self.function_hints: list[FunctionHint] = []

        # line number mapping
        self.addr_to_line = {}

        # Custom options
        self._custom_entry_point = entry_point
        self._custom_base_addr = base_addr
        self.provides = os.path.basename(self.binary) if self.binary is not None else None

        self.memory: Clemory

        # should be set inside `cle.Loader.add_object`
        self._is_mapped = False
        # cached max_addr
        self._max_addr = None
        # cached last section
        self._last_section = None
        # cached last segment
        self._last_segment = None

        self.cached_content: bytes | None = None

        if arch is None:
            pass
        elif isinstance(arch, str):
            self.set_arch(archinfo.arch_from_id(arch))
        elif isinstance(arch, archinfo.Arch):
            self.set_arch(arch)
        elif isinstance(arch, type) and issubclass(arch, archinfo.Arch):
            self.set_arch(arch())  # type: ignore
        else:
            raise CLEError(f"Bad parameter: arch={arch}")

        self._cache_content()
        self._checksum()

    @property
    def arch(self) -> archinfo.Arch:
        result = self._arch
        if result is None:
            raise ValueError("No arch is assigned yet")
        return result

    @property
    def loader(self) -> Loader:
        result = self._loader
        if result is None:
            raise ValueError("Backend does not have a loader associated")
        return result

    def close(self) -> None:
        del self._binary_stream

    def __repr__(self):
        return (
            f"<{self.__class__.__name__} Object {self.binary_basename}, maps [{self.min_addr:#x}:{self.max_addr:#x}]>"
        )

    def set_arch(self, arch):
        self._arch = arch
        self.memory = Clemory(arch)  # Private virtual address space, without relocations

    def set_load_args(self, **kwargs) -> None:
        self.load_args |= kwargs

    @property
    def image_base_delta(self):
        return self.mapped_base - self.linked_base

    @property
    def entry(self):
        if self._custom_entry_point is not None:
            return AT.from_lva(self._custom_entry_point, self).to_mva()
        return AT.from_lva(self._entry, self).to_mva()

    @property
    def segments(self) -> Regions[Segment]:
        return self._segments

    @segments.setter
    def segments(self, v: Regions[Segment] | list[Segment]):
        if isinstance(v, list):
            self._segments = Regions(lst=v)
        elif isinstance(v, Regions):
            self._segments = v
        else:
            raise ValueError(f"Unsupported type {type(v)} set as sections.")

    @property
    def sections(self) -> Regions[Section]:
        return self._sections

    @sections.setter
    def sections(self, v: Regions[Section] | list[Section]):
        if isinstance(v, list):
            self._sections = Regions(lst=v)
        elif isinstance(v, Regions):
            self._sections = v
        else:
            raise ValueError(f"Unsupported type {type(v)} set as sections.")

    @property
    def symbols_by_addr(self):
        log.critical(
            "Deprecation warning: symbols_by_addr is deprecated - use loader.find_symbol() for lookup "
            "and .symbols for enumeration"
        )
        return {s.rebased_addr: s for s in self.symbols}

    def rebase(self, new_base):
        """
        Rebase backend's regions to the new base where they were mapped by the loader
        """
        if self._is_mapped:
            # we could rebase an object twice if we really wanted... no need though, right?
            raise CLEOperationError(f"Image already rebased from {self.linked_base:#x} to {self.mapped_base:#x}")

        self.mapped_base = new_base

        if self.sections:
            self.sections._rebase(self.image_base_delta)
        if self.segments and self.sections is not self.segments:
            self.segments._rebase(self.image_base_delta)

        for handling in self.exception_handlings:
            if handling.func_addr is not None:
                handling.func_addr += self.image_base_delta
            if handling.handler_addr is not None:
                handling.handler_addr += self.image_base_delta
            handling.start_addr += self.image_base_delta

        for hint in self.function_hints:
            hint.addr = hint.addr + self.image_base_delta

    def relocate(self):
        """
        Apply all resolved relocations to memory.

        The meaning of "resolved relocations" is somewhat subtle - there is a linking step which attempts to resolve
        each relocation, currently only present in the main internal loading function since the calculation of which
        objects should be available
        """
        for reloc in self.relocs:
            if reloc.resolved:
                reloc.relocate()

    def contains_addr(self, addr):
        """
        Is `addr` in one of the binary's segments/sections we have loaded? (i.e. is it mapped into memory ?)
        """
        return self.find_loadable_containing(addr) is not None

    def find_loadable_containing(self, addr):
        lookup = self.find_segment_containing if self.segments else self.find_section_containing
        return lookup(addr)

    def find_segment_containing(self, addr: int) -> Segment | None:
        """
        Returns the segment that contains `addr`, or ``None``.
        """
        if self._last_segment is not None and self._last_segment.contains_addr(addr):
            return self._last_segment

        r = self.segments.find_region_containing(addr)
        if r is not None:
            self._last_segment = r
        return r

    def find_section_containing(self, addr: int) -> Section | None:
        """
        Returns the section that contains `addr` or ``None``.
        """
        if self._last_section is not None and self._last_section.contains_addr(addr):
            return self._last_section

        r = self.sections.find_region_containing(addr)
        if r is not None:
            self._last_section = r
        return r

    def addr_to_offset(self, addr: int) -> int | None:
        loadable = self.find_loadable_containing(addr)
        if loadable is not None:
            return loadable.addr_to_offset(addr)
        else:
            return None

    def offset_to_addr(self, offset: int) -> int | None:
        if self.segments:
            for s in self.segments:
                if s.contains_offset(offset):
                    return s.offset_to_addr(offset)
        else:
            for s in self.sections:
                if s.contains_offset(offset):
                    return s.offset_to_addr(offset)
        return None

    @property
    def min_addr(self) -> int:
        """
        This returns the lowest virtual address contained in any loaded segment of the binary.
        """
        # Loader maps the object at chosen mapped base anyway and independently of the internal structure
        return self.mapped_base

    @property
    def max_addr(self) -> int:
        """
        This returns the highest virtual address contained in any loaded segment of the binary.
        """

        if self._max_addr is None:
            out = self.mapped_base
            if self.segments or self.sections:
                out = max(x.max_addr for x in (self.segments or self.sections))
            self._max_addr = out - self.mapped_base
        return self._max_addr + self.mapped_base

    @property
    def initializers(self) -> list[int]:  # pylint: disable=no-self-use
        """
        Stub function. Should be overridden by backends that can provide initializer functions that ought to be run
        before execution reaches the entry point. Addresses should be rebased.
        """
        return []

    @property
    def finalizers(self) -> list[int]:  # pylint: disable=no-self-use
        """
        Stub function. Like initializers, but with finalizers.
        """
        return []

    @property
    def threads(self) -> list:  # pylint: disable=no-self-use
        """
        If this backend represents a dump of a running program, it may contain one or more thread contexts, i.e.
        register files. This property should contain a list of names for these threads, which should be unique.
        """
        return []

    def thread_registers(self, thread=None) -> dict[str, Any]:  # pylint: disable=no-self-use,unused-argument
        """
        If this backend represents a dump of a running program, it may contain one or more thread contexts, i.e.
        register files. This method should return the register file for a given thread (as named in ``Backend.threads``)
        as a dict mapping register names (as seen in archinfo) to numbers. If the thread is not specified, it should
        return the context for a "default" thread. If there are no threads, it should return an empty dict.
        """
        return {}

    def initial_register_values(self):
        """
        Deprecated
        """
        log.critical(
            "Deprecation warning: initial_register_values is deprecated - " "use backend.thread_registers() instead"
        )
        return self.thread_registers().items()

    def get_symbol(self, name: str) -> Symbol | None:  # pylint: disable=no-self-use,unused-argument
        """
        Stub function. Implement to find the symbol with name `name`.
        """
        if name in self._symbol_cache:
            return self._symbol_cache[name]
        for sym in self.symbols:
            if sym.name == name:
                self._symbol_cache[name] = sym
                return sym
        return None

    @staticmethod
    def extract_soname(path) -> str | None:  # pylint: disable=unused-argument
        """
        Extracts the shared object identifier from the path, or returns None if it cannot.
        """
        return None

    @classmethod
    def is_compatible(cls, stream) -> bool:  # pylint:disable=unused-argument
        """
        Determine quickly whether this backend can load an object from this stream
        """
        return False

    @classmethod
    def check_compatibility(cls, spec, obj) -> bool:  # pylint: disable=unused-argument
        """
        Performs a minimal static load of ``spec`` and returns whether it's compatible with other_obj
        """
        return False

    @classmethod
    def check_magic_compatibility(cls, stream: BinaryIO) -> bool:  # pylint: disable=unused-argument
        """
        Check if a stream of bytes contains the same magic number as the main object
        """
        return False

    @staticmethod
    def _get_symbol_relative_addr(value):
        return value.relative_addr

    def _cache_content(self):
        """
        Cache the raw content of this object.
        """
        if self._binary_stream is not None:
            self._binary_stream.seek(0)
            data = self._binary_stream.read()
            self._binary_stream.seek(0)
            self.cached_content = data

    def _checksum(self):
        """
        Calculate MD5 and SHA256 checksum for the binary.
        """

        if self._binary_stream is not None:
            self._binary_stream.seek(0)
            data = self._binary_stream.read()
            self._binary_stream.seek(0)
            self.md5 = hashlib.md5(data).digest()
            self.sha256 = hashlib.sha256(data).digest()

    def __getstate__(self):
        state = self.__dict__.copy()
        state["symbols"] = list(state["symbols"])
        return state

    def __setstate__(self, state):
        state["symbols"] = sortedcontainers.SortedKeyList(state["symbols"], key=self._get_symbol_relative_addr)
        self.__dict__.update(state)
        for sym in self.symbols:
            sym.owner = self

    def __contains__(self, thing: int) -> bool:
        """
        This serves two purposes:
        1. It's slightly more convenient than writing self.min_addr <= thing < self.max_addr yourself
        2. If a Backend implements some form of __getitem__ that always returns False for an integer, running
        `0 in backend` will run into an infinite loop. This prevents that, by just defining sensible semantics for `in`

        This could also be extended to other types, in the future, if it makes sense.
        """

        if isinstance(thing, int):
            return self.min_addr <= thing < self.max_addr
        raise ValueError(f"Unsupported type {type(thing)} for containment check")


ALL_BACKENDS: dict[str, type[Backend]] = {}


def register_backend(name, cls):
    ALL_BACKENDS.update({name: cls})
