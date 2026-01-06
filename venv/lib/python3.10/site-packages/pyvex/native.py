import getpass
import hashlib
import importlib.resources
import os
import pickle
import sys
import tempfile
from typing import Any

import cffi

from .vex_ffi import ffi_str as _ffi_str

ffi = cffi.FFI()


def _parse_ffi_str():
    hash_ = hashlib.md5(_ffi_str.encode("utf-8")).hexdigest()
    cache_location = os.path.join(tempfile.gettempdir(), f"pyvex_ffi_parser_cache.{getpass.getuser()}.{hash_}")

    if os.path.isfile(cache_location):
        # load the cache
        with open(cache_location, "rb") as f:
            cache = pickle.loads(f.read())
        ffi._parser._declarations = cache["_declarations"]
        ffi._parser._int_constants = cache["_int_constants"]
    else:
        ffi.cdef(_ffi_str)
        # cache the result
        cache = {
            "_declarations": ffi._parser._declarations,
            "_int_constants": ffi._parser._int_constants,
        }
        # atomically write cache
        with tempfile.NamedTemporaryFile(delete=False) as temp_file:
            temp_file.write(pickle.dumps(cache))
            temp_file_name = temp_file.name
        os.replace(temp_file_name, cache_location)


def _find_c_lib():
    # Load the c library for calling into VEX
    if sys.platform in ("win32", "cygwin"):
        library_file = "pyvex.dll"
    elif sys.platform == "darwin":
        library_file = "libpyvex.dylib"
    else:
        library_file = "libpyvex.so"

    pyvex_path = str(importlib.resources.files("pyvex") / "lib" / library_file)
    # parse _ffi_str and use cache if possible
    _parse_ffi_str()
    # RTLD_GLOBAL used for sim_unicorn.so
    lib = ffi.dlopen(pyvex_path)

    if not lib.vex_init():
        raise ImportError("libvex failed to initialize")
    # this looks up all the definitions (wtf)
    dir(lib)
    return lib


pvc: Any = _find_c_lib()  # This should be properly typed, but this seems non trivial
