from __future__ import annotations

import logging
import struct

from cle.backends.symbol import SymbolType

from .common import PointTo
from .simdata import SimData, register

log = logging.getLogger(name=__name__)

#
# Here, we define a specific structure (part of it at least) for the FILE structure.
# These offsets are copied from glibc for maximum compatibility, but we are effectively
# implementing SOME libc with these symbols, so we need SOME implementation of FILE.
#
# this is supposed to be an opaque structure, the internals of which are only cared about
# by an angr simprocedure or whatever implements the fread/fwrite/etc we're linking to. And since we're linking to
# this crap instead of a real stdin/stdout/etc, someone in python land will probably be the guy which needs ABI
# compatibility with us.
#
# however, it is also a desirable property that this is abi-compatible with glibc or something so the someone in python
# land could use this to interface with the "real" structure, which would be filled out by someone other than the
# below code. To this end we so far only have the fileno, but we could add more things like buffers
#

_IO_FILE = {
    "MIPS32": {
        "size": 148,
        "fd": 0x38,
    },
    "X86": {
        "size": 148,
        "fd": 0x38,
    },
    "AMD64": {
        "size": 216,
        "fd": 0x70,
    },
    # Bionic libc does not use __IO_FILE
    # Refer to http://androidxref.com/5.1.1_r6/xref/bionic/libc/include/stdio.h
    # __sFILE replaces __IO_FILE
    # _file replaces _fileno
    "ARM": {
        "size": 84,
        "fd": 0x0E,
    },
    "AARCH64": {
        "size": 152,
        "fd": 0x14,
    },
}

_IO_FILE["ARMEL"] = _IO_FILE["ARM"]
_IO_FILE["ARMHF"] = _IO_FILE["ARM"]


def io_file_data_for_arch(arch):
    if arch.name not in _IO_FILE:
        log.error("missing _IO_FILE offsets for arch: %s", arch.name)
        return _IO_FILE["AMD64"]
    return _IO_FILE[arch.name]


class IoFilePointer(PointTo):
    libname = "libc.so.6"
    pointto_type = SymbolType.TYPE_OBJECT


class IoStdinPointer(IoFilePointer):
    name = "stdin"
    pointto_name = "_io_stdin"


class IoStdoutPointer(IoFilePointer):
    name = "stdout"
    pointto_name = "_io_stdout"


class IoStderrPointer(IoFilePointer):
    name = "stderr"
    pointto_name = "_io_stderr"


class IoFile(SimData):
    libname = "libc.so.6"
    type = SymbolType.TYPE_OBJECT
    fd: int = NotImplemented

    @classmethod
    def static_size(cls, owner):
        return io_file_data_for_arch(owner.arch)["size"]

    # the canonical verision of this should be the FILEBUF_LITERAL macro from glibc
    # for maximum hyperrealism we could have a dependency on the IO_jumps table which would have dependencies on
    # all the functions we could care about which would be implemented by simprocedures
    # but that's way overkill. see above discussion.
    def value(self):
        val = bytearray(self.size)
        struct.pack_into(self.owner.arch.struct_fmt(size=4), val, io_file_data_for_arch(self.owner.arch)["fd"], self.fd)
        struct.pack_into(self.owner.arch.struct_fmt(size=4), val, 0, 0xFBAD2088)
        return bytes(val)


class IoStdin(IoFile):
    name = "_io_stdin"
    fd = 0


class IoStdout(IoFile):
    name = "_io_stdout"
    fd = 1


class IoStderr(IoFile):
    name = "_io_stderr"
    fd = 2


register(IoStdinPointer)
register(IoStdoutPointer)
register(IoStderrPointer)
register(IoStdin)
register(IoStdout)
register(IoStderr)
