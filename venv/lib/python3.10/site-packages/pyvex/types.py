from typing import TYPE_CHECKING, Any, Protocol, Union, runtime_checkable

from cffi.api import FFI


class Register(Protocol):
    """
    A register. Pyvex should probably not have this dependency.
    """

    name: str


class Arch(Protocol):
    """
    An architecture description.
    """

    name: str
    ip_offset: int
    bits: int
    instruction_endness: str
    memory_endness: str
    byte_width: int
    register_list: list[Register]
    registers: dict[str, tuple[int, int]]

    def translate_register_name(self, offset: int, size: int | None = None) -> str | None: ...

    def get_register_offset(self, name: str) -> int: ...


@runtime_checkable
class LibvexArch(Protocol):
    """
    The description for an architecture that is usable with libvex
    """

    vex_arch: str
    vex_archinfo: dict[str, Any]


PyLiftSource = Union[bytes, bytearray, memoryview]
if TYPE_CHECKING:
    CLiftSource = FFI.CData
else:
    CLiftSource = None
LiftSource = Union[PyLiftSource, CLiftSource]
