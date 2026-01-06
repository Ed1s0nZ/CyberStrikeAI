from __future__ import annotations

import logging

import archinfo

from cle.address_translator import AT
from cle.errors import CLEError

from .backend import Backend, register_backend
from .relocation import Relocation
from .symbol import Symbol, SymbolType

log = logging.getLogger(name=__name__)

try:
    import binaryninja as bn
except ImportError:
    bn = None
    log.debug("Unable to import binaryninja module")
    BINJA_NOT_INSTALLED_STR = (
        "Binary Ninja does not appear to be installed. Please ensure Binary Ninja "
        "and its Python API are properly installed before using this backend."
    )


class BinjaSymbol(Symbol):
    BINJA_FUNC_SYM_TYPES = (
        [bn.SymbolType.ImportedFunctionSymbol, bn.SymbolType.FunctionSymbol, bn.SymbolType.ImportAddressSymbol]
        if bn
        else []
    )

    BINJA_DATA_SYM_TYPES = [bn.SymbolType.ImportedDataSymbol, bn.SymbolType.DataSymbol] if bn else []

    BINJA_IMPORT_TYPES = (
        [bn.SymbolType.ImportedFunctionSymbol, bn.SymbolType.ImportAddressSymbol, bn.SymbolType.ImportedDataSymbol]
        if bn
        else []
    )

    def __init__(self, owner, sym):
        if not bn:
            raise CLEError(BINJA_NOT_INSTALLED_STR)

        if sym.type in self.BINJA_FUNC_SYM_TYPES:
            symtype = SymbolType.TYPE_FUNCTION
        elif sym.type in self.BINJA_DATA_SYM_TYPES:
            symtype = SymbolType.TYPE_OBJECT
        else:
            symtype = SymbolType.TYPE_OTHER

        super().__init__(owner, sym.raw_name, AT.from_rva(sym.address, owner).to_rva(), owner.bv.address_size, symtype)

        if sym.type in self.BINJA_IMPORT_TYPES:
            self.is_import = True

        # TODO: set is_weak appropriately


class BinjaReloc(Relocation):
    @property
    def value(self):
        return self.relative_addr


class BinjaBin(Backend):
    """
    Get information from binaries using Binary Ninja. Basing this on idabin.py, but will try to be more complete.
    TODO: add more features as Binary Ninja's feature set improves
    """

    is_default = True  # Tell CLE to automatically consider using the BinjaBin backend
    BINJA_ARCH_MAP = {
        "aarch64": archinfo.ArchAArch64(endness="Iend_LE"),
        "armv7": archinfo.ArchARMEL(endness="Iend_LE"),
        "thumb2": archinfo.ArchARMEL(endness="Iend_LE"),
        "armv7eb": archinfo.ArchARMEL(endness="Iend_BE"),
        "thumb2eb": archinfo.ArchARMEL(endness="Iend_BE"),
        "mipsel32": archinfo.ArchMIPS32(endness="Iend_LE"),
        "mips32": archinfo.ArchMIPS32(endness="Iend_BE"),
        "ppc": archinfo.ArchPPC32(endness="Iend_BE"),
        "ppc_le": archinfo.ArchPPC32(endness="Iend_LE"),
        "x86": archinfo.ArchX86(),
        "x86_64": archinfo.ArchAMD64(),
    }

    def __init__(self, binary, *args, **kwargs):
        super().__init__(binary, *args, **kwargs)
        if not bn:
            raise CLEError(BINJA_NOT_INSTALLED_STR)
        # get_view_of_file can take a bndb or binary - wait for autoanalysis to complete
        self.bv = bn.BinaryViewType.get_view_of_file(binary, False)
        log.info("Analyzing %s, this may take some time...", binary)
        self.bv.update_analysis_and_wait()
        log.info("Analysis complete")
        # Note may want to add option to kick off linear sweep

        try:
            self.set_arch(self.BINJA_ARCH_MAP[self.bv.arch.name])
        except KeyError:
            log.error("Architecture %s is not supported.", self.bv.arch.name)

        for seg in self.bv.segments:
            log.info("Adding memory for segment at %x.", seg.start)
            br = bn.BinaryReader(self.bv)
            br.seek(seg.start)
            data = br.read(len(seg))
            self.memory.add_backer(seg.start, data)

        self._find_got()
        self._symbol_cache = {}
        self._init_symbol_cache()
        # Note: this represents the plt stub. ImportAddressSymbol refers to .got entries
        # Since we're not trying to import and load dependencies directly, but want to run SimProcedures,
        # We should use the binaryninja.SymbolType.ImportedFunctionSymbol
        # Also this should be generalized to get data imports, too
        self.raw_imports = {
            i.name: i.address for i in self.bv.get_symbols_of_type(bn.SymbolType.ImportedFunctionSymbol)
        }
        self._process_imports()
        self.exports = {}
        self.linking = "static" if len(self.raw_imports) == 0 else "dynamic"
        # We'll look for this attribute to see if we need to do SimProcedures for any imports in this binary
        # This is an ugly hack, but will have to use this for now until Binary Ninja exposes dependencies
        self.guess_simprocs = True
        self.guess_simprocs_hint = "nix" if self.bv.get_section_by_name(".plt") else "win"
        log.warning(
            "This backend is based on idabin.py.\n"
            "You may encounter unexpected behavior if:\n"
            "\tyour target depends on library data symbol imports, or\n"
            "\tlibrary imports that don't have a guess-able SimProcedure\n"
            "Good luck!"
        )

    def _process_imports(self):
        """Process self.raw_imports into list of Relocation objects"""
        if not self.raw_imports:
            log.warning("No imports found - if this is a dynamically-linked binary, something probably went wrong.")

        for name, addr in self.raw_imports.items():
            BinjaReloc(self, self._symbol_cache[name], addr)

    def _init_symbol_cache(self):
        # Note that we could also access name, short_name, or full_name attributes
        for sym in self.bv.get_symbols():
            cle_sym = BinjaSymbol(self, sym)
            self._symbol_cache[sym.raw_name] = cle_sym
            self.symbols.add(cle_sym)

    def _find_got(self):
        """
        Locate the section (e.g. .got) that should be updated when relocating functions (that's where we want to
        write absolute addresses).
        """
        sec_name = self.arch.got_section_name
        self.got_begin = None
        self.got_end = None

        try:
            got_sec = self.bv.sections[self.arch.got_section_name]
            self.got_begin = got_sec.start
            self.got_end = got_sec.end
        except KeyError:
            log.warning("No got section mapping found!")

        # If we reach this point, we should have the addresses
        if self.got_begin is None or self.got_end is None:
            log.warning("No section %s, is this a static binary ? (or stripped)", sec_name)
            return False
        return True

    @staticmethod
    def is_compatible(stream):
        if not bn:
            return False
        magic = stream.read(100)
        stream.seek(0)
        # bndb files are SQlite 3
        if magic.startswith(b"SQLite format 3") and stream.name.endswith("bndb"):
            return True

        return False

    def in_which_segment(self, addr):
        """
        Return the segment name at address `addr`.
        """
        # WARNING: if there are overlapping sections, we choose the first name.
        # The only scenario I've seen here is a NOBITS section that "overlaps" with another one, but
        # I'm not sure if that's a heurstic that should be applied here.
        # https://stackoverflow.com/questions/25501044/gcc-ld-overlapping-sections-tbss-init-array-in-statically-linked-elf-bin#25771838
        seg = self.bv.get_sections_at(addr)[0].name
        return "unknown" if len(seg) == 0 else seg

    def get_symbol_addr(self, sym):
        """
        Get the address of the symbol `sym` from IDA.

        :returns: An address.
        """
        # sym is assumed to be the raw_name of the symbol
        return self.bv.get_symbol_by_raw_name(sym)

    def function_name(self, addr):
        """
        Return the function name at address `addr`.
        """
        func = self.bv.get_function_at(addr)
        if not func:
            return "UNKNOWN"
        return func.name

    @property
    def min_addr(self):
        """
        Get the min address of the binary. (note: this is probably not "right")
        """
        return self.bv.start

    @property
    def max_addr(self):
        """
        Get the max address of the binary.
        """
        return self.bv.end

    @property
    def entry(self):
        if self._custom_entry_point is not None:
            return self._custom_entry_point + self.mapped_base
        return self.bv.entry_point + self.mapped_base

    def get_strings(self):
        """
        Extract strings from binary (Binary Ninja).

        :returns:   An array of strings.
        """
        return self.bv.get_strings()

    def set_got_entry(self, name, newaddr):
        """
        Resolve import `name` with address `newaddr`. That is, update the GOT entry for `name` with `newaddr`.
        """
        if name not in self.imports:
            log.warning("%s not in imports", name)
            return

        addr = self.imports[name]
        self.memory.pack_word(addr, newaddr)

    def close(self):
        """
        Release the BinaryView we created in __init__
        :return: None
        """
        self.bv.file.close()


register_backend("binja", BinjaBin)
