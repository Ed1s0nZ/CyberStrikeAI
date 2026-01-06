"""
PyVEX provides an interface that translates binary code into the VEX intermediate representation (IR).
For an introduction to VEX, take a look here: https://docs.angr.io/advanced-topics/ir
"""

__version__ = "9.2.191"

from . import const, expr, stmt
from .arches import (
    ARCH_AMD64,
    ARCH_ARM64_BE,
    ARCH_ARM64_LE,
    ARCH_ARM_BE,
    ARCH_ARM_BE_LE,
    ARCH_ARM_LE,
    ARCH_MIPS32_BE,
    ARCH_MIPS32_LE,
    ARCH_MIPS64_BE,
    ARCH_MIPS64_LE,
    ARCH_PPC32,
    ARCH_PPC64_BE,
    ARCH_PPC64_LE,
    ARCH_RISCV64_LE,
    ARCH_S390X,
    ARCH_X86,
)
from .block import IRSB, IRTypeEnv
from .const import get_type_size, get_type_spec_size, tag_to_const_class
from .enums import (
    IRCallee,
    IRRegArray,
    VEXObject,
    default_vex_archinfo,
    get_enum_from_int,
    get_int_from_enum,
    irop_enums_to_ints,
    vex_endness_from_string,
)
from .errors import PyVEXError
from .expr import get_op_retty
from .lifting import lift, lifters
from .native import ffi, pvc

# aliases....
IRStmt = stmt
IRExpr = expr
IRConst = const


__all__ = [
    "const",
    "expr",
    "stmt",
    "IRSB",
    "IRTypeEnv",
    "get_type_size",
    "get_type_spec_size",
    "irop_enums_to_ints",
    "tag_to_const_class",
    "IRCallee",
    "IRRegArray",
    "VEXObject",
    "default_vex_archinfo",
    "get_enum_from_int",
    "get_int_from_enum",
    "vex_endness_from_string",
    "PyVEXError",
    "get_op_retty",
    "lift",
    "lifters",
    "ffi",
    "pvc",
    "IRStmt",
    "IRExpr",
    "IRConst",
    "ARCH_X86",
    "ARCH_AMD64",
    "ARCH_ARM_BE",
    "ARCH_ARM_BE_LE",
    "ARCH_ARM_LE",
    "ARCH_ARM64_LE",
    "ARCH_ARM64_BE",
    "ARCH_PPC32",
    "ARCH_PPC64_BE",
    "ARCH_PPC64_LE",
    "ARCH_S390X",
    "ARCH_MIPS32_BE",
    "ARCH_MIPS32_LE",
    "ARCH_MIPS64_BE",
    "ARCH_MIPS64_LE",
    "ARCH_RISCV64_LE",
]
