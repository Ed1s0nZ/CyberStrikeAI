import sys
from typing import NewType

if sys.version_info < (3, 11):
    from backports.strenum import StrEnum
else:
    from enum import StrEnum


class RegisterOffset(int):
    """A register offset is an integer that represents the offset of a register
    in VEX's register memory space.
    """

    def __add__(self, other):
        if isinstance(other, int):
            return RegisterOffset(int(self) + other)
        return NotImplemented


TmpVar = NewType("TmpVar", int)

# This causes too much issues as a NewType, sot is a simple alias instead
# This means that is still legal to pass any str where a RegisterName is expected.
# The downside is that PyCharm will show the type as `str` when displaying the signature
RegisterName = str


class Endness(StrEnum):
    """Endness specifies the byte order for integer values

    :cvar LE:      little endian, least significant byte is stored at lowest address
    :cvar BE:      big endian, most significant byte is stored at lowest address
    :cvar ME:      Middle-endian. Yep.
    """

    LE = "Iend_LE"
    BE = "Iend_BE"
    ME = "Iend_ME"
    ANY = "any"
    UNSURE = "unsure"

    @staticmethod
    def from_str(s: str) -> "Endness":
        for e in Endness:
            if e.value == s:
                return e
        raise ValueError("Unknown endness: %s" % s)
