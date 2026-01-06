from __future__ import annotations

import logging
import os

from cle.errors import CLEFileNotFoundError
from cle.utils import get_text_offset

log = logging.getLogger(name=__name__)


def convert_info_sharedlibrary(fname):
    """
    Convert a dump from gdb's ``info sharedlibrary`` command to a set of options that can be passed to CLE to replicate
    the address space from the gdb session

    :param fname:   The name of a file containing the dump
    :returns:       A dict appropriate to be passed as ``**kwargs`` for ``angr.Project`` or ``cle.Loader``
    """
    return _parse_gdb_map(fname, True)


def convert_info_proc_maps(fname):
    """
    Convert a dump from gdb's ``info proc maps`` command to a set of options that can be passed to CLE to replicate
    the address space from the gdb session

    :param fname:   The name of a file containing the dump
    :returns:       A dict appropriate to be passed as ``**kwargs`` for ``angr.Project`` or ``cle.Loader``
    """
    return _parse_gdb_map(fname, False)


def _parse_gdb_map(gdb_map, gdb_fix):
    if not os.path.isfile(gdb_map):
        raise CLEFileNotFoundError(f"gdb mapping file {gdb_map} does not exist")
    with open(gdb_map) as f:
        data = f.readlines()

    gmap = {}
    for line in data:
        if line in ("\n", "\r\n"):
            continue
        line_items = line.split()
        # Get rid of all metadata, just extract lines containing addresses
        if "0x" not in line_items[0]:
            continue
        if line_items[-1].startswith("["):
            continue
        try:
            int(line_items[-1], 16)
        except ValueError:
            pass
        else:
            continue

        addr, objfile = int(line_items[0], 16), line_items[-1].strip()

        # Get the smallest address of each libs' mappings
        try:
            gmap[objfile] = min(gmap[objfile], addr)
        except KeyError:
            gmap[objfile] = addr

    # Find lib names
    # libnames = filter(lambda n: '.so' in n, gmap.keys())

    # Find base addr for each lib (each lib is mapped to several segments,
    # we take the segment that is loaded at the smallest address).
    lib_opts = {}
    main_opts = {}
    force_load_libs = []
    smallest_addr = min(gmap.values())

    for lib, addr in gmap.items():
        if addr == smallest_addr and not gdb_fix:
            # this is the main binary
            opts = main_opts
        else:
            # if not os.path.exists(lib):
            #    lib = _simple_search(lib)
            force_load_libs.append(lib)
            opts = {}
            lib_opts[lib] = opts

        # address of .text -> base address of the library
        if gdb_fix:
            found = _simple_search(lib)
            if found:
                addr = addr - get_text_offset(found)

        log.info("gdb_plugin: mapped %s to %#x", lib, addr)
        opts["base_addr"] = addr

    return {"force_load_libs": force_load_libs, "main_opts": main_opts, "lib_opts": lib_opts}


GDB_SEARCH_PATH = ["/lib", "/usr/lib"]


def _simple_search(libname):
    dirs = list(GDB_SEARCH_PATH)
    while dirs:
        dirname = dirs.pop(0)
        try:
            for name in os.listdir(dirname):
                if name in (".", ".."):
                    continue
                full = os.path.join(dirname, name)
                if os.path.isdir(full):
                    if full.count("/") < 12:  # don't go too deep
                        dirs.append(full)
                if os.path.isfile(full) and name == libname:
                    return full
        except OSError:
            pass
    return libname
