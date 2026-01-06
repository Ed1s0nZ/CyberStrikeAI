from typing import Any

from .native import ffi, pvc
from .utils import stable_hash


class VEXObject:
    """
    The base class for Vex types.
    """

    __slots__: list[str] = []

    def __eq__(self, other):
        if not isinstance(other, type(self)):
            return False
        # compare values in slots
        for slot in self.__slots__:
            if getattr(self, slot) != getattr(other, slot):
                return False
        return True

    def __hash__(self):
        values = [getattr(self, slot) for slot in self.__slots__]
        for i, lst_val in enumerate(values):
            if isinstance(lst_val, list):
                values[i] = tuple(lst_val)
        return stable_hash(tuple([type(self)] + values))


class IRCallee(VEXObject):
    """
    Describes a helper function to call.
    """

    __slots__ = ["regparms", "name", "mcx_mask"]

    def __init__(self, regparms, name, mcx_mask):
        VEXObject.__init__(self)
        self.regparms = regparms
        self.name = name
        self.mcx_mask = mcx_mask

    def __str__(self):
        return str(self.name)

    @staticmethod
    def _from_c(c_callee):
        return IRCallee(
            c_callee.regparms,
            ffi.string(c_callee.name).decode(),
            # NO. #int(ffi.cast("unsigned long long", c_callee.addr)),
            c_callee.mcx_mask,
        )

    @staticmethod
    def _to_c(callee):  # pylint: disable=unused-argument
        raise TypeError(
            "This doesn't work! Please invent a way to get the correct address for the named function from pyvex_c."
        )
        # c_callee = pvc.mkIRCallee(callee.regparms,
        #                          callee.name.encode(),
        #                          ffi.cast("void *", callee.addr))
        # c_callee.mcx_mask = callee.mcx_mask
        # return c_callee


class IRRegArray(VEXObject):
    """
    A section of the guest state that we want te be able to index at run time, so as to be able to describe indexed or
    rotating register files on the guest.

    :ivar int base:     The offset into the state that this array starts
    :ivar str elemTy:   The types of the elements in this array, as VEX enum strings
    :ivar int nElems:   The number of elements in this array
    """

    __slots__ = ["base", "elemTy", "nElems"]

    def __init__(self, base, elemTy, nElems):
        VEXObject.__init__(self)
        self.base = base
        self.elemTy = elemTy
        self.nElems = nElems

    def __str__(self):
        return "%s:%sx%d" % (self.base, self.elemTy[4:], self.nElems)

    @staticmethod
    def _from_c(c_arr):
        return IRRegArray(c_arr.base, ints_to_enums[c_arr.elemTy], c_arr.nElems)

    @staticmethod
    def _to_c(arr):
        return pvc.mkIRRegArray(arr.base, get_int_from_enum(arr.elemTy), arr.nElems)


ints_to_enums: dict[int, str] = {}
enums_to_ints: dict[str, int] = {}
irop_enums_to_ints: dict[str, int] = {}
will_be_overwritten = ["Ircr_GT", "Ircr_LT"]


def get_enum_from_int(i):
    return ints_to_enums[i]


def get_int_from_enum(e):
    return enums_to_ints[e]


_add_enum_counter = 0


def _add_enum(s, i=None):  # TODO get rid of this
    global _add_enum_counter  # pylint: disable=global-statement
    if i is None:
        while _add_enum_counter in ints_to_enums:
            _add_enum_counter += 1
        i = _add_enum_counter
        _add_enum_counter += 1  # Update for the next iteration
    if i in ints_to_enums:
        if ints_to_enums[i] not in will_be_overwritten:
            raise ValueError("Enum with intkey %d already present" % i)
    enums_to_ints[s] = i
    ints_to_enums[i] = s
    if s.startswith("Iop_"):
        irop_enums_to_ints[s] = i


for attr in dir(pvc):
    if attr[0] in "ABCDEFGHIJKLMNOPQRSTUVWXYZ" and hasattr(pvc, attr) and isinstance(getattr(pvc, attr), int):
        _add_enum(attr, getattr(pvc, attr))


def vex_endness_from_string(endness_str):
    return getattr(pvc, endness_str)


def default_vex_archinfo() -> dict[str, Any]:
    return {
        "hwcaps": 0,
        "endness": vex_endness_from_string("VexEndnessLE"),
        "ppc_icache_line_szB": 0,
        "ppc_dcbz_szB": 0,
        "ppc_dcbzl_szB": 0,
        "arm64_dMinLine_lg2_szB": 0,
        "arm64_iMinLine_lg2_szB": 0,
        "hwcache_info": {
            "num_levels": 0,
            "num_caches": 0,
            "caches": None,
            "icaches_maintain_coherence": True,
        },
        "x86_cr0": 0xFFFFFFFF,
    }
