from __future__ import annotations

from cle.backends.symbol import SymbolSubType, SymbolType


class ELFSymbolType(SymbolSubType):
    """
    ELF-specific symbol types
    """

    # Enum classes cannot be inherited. Therefore, additional platform-specific
    # values should simply be added to this enumeration (e.g., STT_GNU_IFUNC)
    # with an appropriate conversion in `to_base_type()`.
    #
    # Though that could be solved with IntEnum as well, that breaks the
    # strong typing and is discouraged by Python docs.

    # Basic types
    STT_NOTYPE = (0, None)  # Symbol's type is not specified
    STT_OBJECT = (1, None)  # Symbol is a data object (variable, array, etc.)
    STT_FUNC = (2, None)  # Symbol is executable code (function, etc.)
    STT_SECTION = (3, None)  # Symbol refers to a section
    STT_FILE = (4, None)  # Local, absolute symbol that refers to a file
    STT_COMMON = (5, None)  # An uninitialized common block
    STT_TLS = (6, None)  # Thread local data object

    # ELF's generic place-holders
    STT_LOOS = (10, None)  # Lowest operating system-specific symbol type
    STT_HIOS = (12, None)  # Highest operating system-specific symbol type

    STT_LOPROC = (13, None)  # Lowest processor-specific symbol type
    STT_HIPROC = (15, None)  # Highest processor-specific symbol type

    #
    # OS- and processor-specific types. Note that the entire range
    # of values is used indiscriminantly for OS or processor.
    #
    # Try to use values that map to an `archinfo.Arch` so that `arch_from_id()`
    # is able to return a specific type. Otherwise, use something indicative
    # of its purpose.
    #

    # GNU indirect function
    #
    # HACK: It's GNU-specific, not OS-specific but GNU doesn't care. This
    # shouldn't be an issue unless someone tries analyzing an old ELF that
    # uses STT_LOOS for something else, before STT_GNU_IFUNC came about, in
    # which case angr will need a new SimOS variant anyway.
    STT_GNU_IFUNC = (STT_LOOS[0], "gnu")

    #
    # Below are examples of additional types that can be added. These are
    # commented out since they've never actually been used or tested.
    #

    # AMDGPU HSA
    #
    # https://github.com/RadeonOpenCompute/ROCR-Runtime/blob/master/src/inc/amd_hsa_elf.h
    # TODO: Update the arch name here if this arch is ever supported
    # STT_AMDGPU_HSA_KERNEL = (STT_LOOS[0], 'amdgpu_hsa')
    # STT_AMDGPU_HSA_INDIRECT_FUNCTION = (STT_LOOS[0] + 1, 'amdgpu_hsa')
    # STT_AMDGPU_HSA_METADATA = (STT_LOOS[0] + 2, 'amdgpu_hsa')

    # HP Precision Architecture (PA-RISC)
    #
    # https://github.com/lattera/glibc/blob/master/elf/elf.h
    # TODO: Update the arch name here if this arch is ever supported
    # STT_HP_OPAQUE = (STT_LOOS[0] + 1, 'hppa')
    # STT_HP_STUB = (STT_LOOS[0] + 2, 'hppa')
    # STT_PARISC_MILLICODE = (STT_LOPROC[0], 'hppa')

    def __init__(self, *args):  # pylint: disable=unused-argument
        # Essentially a static type check, this will fail on import
        # if someone defines a type that's not a `tuple`
        if not isinstance(self.value, tuple):
            raise ValueError(
                f"Symbol value '{self.value}' for member '{self.name}' is invalid. Values must be tuples."
            )  # pylint: disable=logging-format-interpolation

    def __repr__(self):
        return f"ELFSymbolType.{self.name}: (elf_value: {self.elf_value}, os_proc: {self.os_proc})"

    def __eq__(self, other):
        if type(self) is not type(other):
            return False
        return self.value[0] == other.value[0]

    def __ne__(self, other):
        return not (self == other)

    @property
    def elf_value(self):
        return self.value[0]  # pylint: disable=unsubscriptable-object

    @property
    def os_proc(self):
        return self.value[1]  # pylint: disable=unsubscriptable-object

    @property
    def is_custom_os_proc(self):
        if self.elf_value in range(self.STT_LOOS.elf_value, self.STT_HIPROC.elf_value + 1):  # pylint: disable=no-member
            return self.os_proc is not None
        return False

    def to_base_type(self):
        if self is ELFSymbolType.STT_NOTYPE:
            return SymbolType.TYPE_NONE

        elif self in [ELFSymbolType.STT_FUNC, ELFSymbolType.STT_GNU_IFUNC]:
            return SymbolType.TYPE_FUNCTION

        elif self in [ELFSymbolType.STT_OBJECT, ELFSymbolType.STT_COMMON]:
            return SymbolType.TYPE_OBJECT

        elif self is ELFSymbolType.STT_SECTION:
            return SymbolType.TYPE_SECTION

        elif self is ELFSymbolType.STT_TLS:
            return SymbolType.TYPE_TLS_OBJECT

        elif self is ELFSymbolType.STT_GNU_IFUNC:
            return SymbolType.TYPE_FUNCTION

        else:
            return SymbolType.TYPE_OTHER


def __ELFSymbolTypeArchParser(cls, value):
    """
    This is just a nice way to allow for just specifying the `int` for
    default types: `ELFSymbolType(10)` rather than `ELFSymbolType((10,None))`.

    Idea courtesy: https://stackoverflow.com/q/24105268/1137728.

    We don't need to implement the `str` parsing like the SO link above since
    `Enum` already has built-in item access: `ELFSymbolType['STT_FUNC']`.
    """
    if isinstance(value, int):
        return super(ELFSymbolType, cls).__new__(cls, (value, None))
    else:
        return super(ELFSymbolType, cls).__new__(cls, value)


setattr(ELFSymbolType, "__new__", __ELFSymbolTypeArchParser)
