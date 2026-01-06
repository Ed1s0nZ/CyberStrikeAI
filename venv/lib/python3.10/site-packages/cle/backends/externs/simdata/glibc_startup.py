from __future__ import annotations

from cle.backends.symbol import SymbolType

from .common import PointTo, StaticWord
from .simdata import SimData, register


class DummyProgname(SimData):
    name = "_dummy_progname"
    type = SymbolType.TYPE_OBJECT
    libname = "libc.so"

    progname = b"./program\0"

    @classmethod
    def static_size(cls, owner):
        return len(cls.progname)

    def value(self):
        return self.progname


class Progname(PointTo):
    pointto_name = "_dummy_progname"
    pointto_type = SymbolType.TYPE_OBJECT
    name = "__progname"
    libname = "libc.so"
    type = SymbolType.TYPE_OBJECT
    addend = 2


class PrognameFull(PointTo):
    pointto_name = "_dummy_progname"
    pointto_type = SymbolType.TYPE_OBJECT
    name = "__progname_full"
    libname = "libc.so.6"
    type = SymbolType.TYPE_OBJECT
    addend = 0


class EnvironmentPointer(StaticWord):
    name = "__environ"
    libname = "libc.so"
    word = 0


class EnvironmentPointerAlso(StaticWord):
    name = "environ"
    libname = "libc.so"
    word = 0


class OptInd(StaticWord):
    name = "optind"
    libname = "libc.so"
    word = 1
    wordsize = 4


class OptArg(StaticWord):
    name = "optarg"
    libname = "libc.so"
    word = 0


class Errno(StaticWord):
    type = SymbolType.TYPE_TLS_OBJECT
    name = "errno"
    libname = "libc.so"
    word = 0
    wordsize = 4


class LibcStackEnd(StaticWord):
    name = "__libc_stack_end"
    libname = "ld-linux"
    word = 0


class RTLDGlobal(SimData):
    name = "_rtld_global"
    type = SymbolType.TYPE_OBJECT
    libname = "ld-linux"

    @classmethod
    def static_size(cls, owner):
        return {"AMD64": 3960, "X86": 2100}.get(owner.arch.name, 1024)

    def value(self):
        return bytes(self.static_size(self.owner))


class RTLDGlobalRO(SimData):
    name = "_rtld_global_ro"
    type = SymbolType.TYPE_OBJECT
    libname = "ld-linux"

    @classmethod
    def static_size(cls, owner):
        return {
            "AMD64": 440,
            "X86": 576,
        }.get(owner.arch.name, 256)

    def value(self):
        return bytes(self.static_size(self.owner))


register(DummyProgname)
register(Progname)
register(PrognameFull)
register(EnvironmentPointer)
register(EnvironmentPointerAlso)
register(OptInd)
register(OptArg)
register(Errno)
register(LibcStackEnd)
register(RTLDGlobal)
register(RTLDGlobalRO)
