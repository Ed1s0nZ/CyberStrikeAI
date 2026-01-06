from __future__ import annotations

import itertools
from typing import TYPE_CHECKING, TypeVar

import claripy
import claripy.simplifications

from . import debug as _d
from .errors import ClaripyOperationError, ClaripyTypeError

if TYPE_CHECKING:
    from collections.abc import Callable

    from claripy.ast import Base

T = TypeVar("T", bound="Base")


def op(name, arg_types, return_type: type[T], extra_check=None, calc_length=None) -> Callable[..., T]:
    if isinstance(arg_types, tuple | list):
        expected_num_args = len(arg_types)
    elif isinstance(arg_types, type):
        expected_num_args = None
    else:
        raise ClaripyOperationError(f"op {name} got weird arg_types")

    def _type_fixer(args):
        num_args = len(args)
        if expected_num_args is not None and num_args != expected_num_args:
            if num_args + 1 == expected_num_args and arg_types[0] is claripy.fp.RM:
                args = (claripy.fp.RM.default(), *args)
            else:
                raise ClaripyTypeError(f"Operation {name} takes exactly {len(arg_types)} arguments ({len(args)} given)")

        actual_arg_types = (arg_types,) * num_args if isinstance(arg_types, type) else arg_types
        matches = list(itertools.starmap(isinstance, zip(args, actual_arg_types, strict=False)))

        # heuristically, this works!
        thing = args[matches.index(True, 1 if actual_arg_types[0] is claripy.fp.RM else 0)] if True in matches else None

        for arg, argty, match in zip(args, actual_arg_types, matches, strict=False):
            if not match:
                if hasattr(argty, "_from_" + type(arg).__name__):
                    convert = getattr(argty, "_from_" + type(arg).__name__)
                    yield convert(thing, arg)
                else:
                    yield NotImplemented
                    return
            else:
                yield arg

    def _op(*args):
        fixed_args = tuple(_type_fixer(args))
        if _d._DEBUG:
            for i in fixed_args:
                if i is NotImplemented:
                    return NotImplemented
            if extra_check is not None:
                success, msg = extra_check(*fixed_args)
                if not success:
                    raise ClaripyOperationError(msg)

        # pylint:disable=too-many-nested-blocks
        simp, annotated = claripy.simplifications.simplify(name, fixed_args)
        if simp is not None:
            if not annotated:
                simp = _handle_annotations(simp, fixed_args)
            if simp is not None:
                return simp

        kwargs = {}
        if calc_length is not None:
            kwargs["length"] = calc_length(*fixed_args)

        return return_type(name, fixed_args, **kwargs)

    _op.calc_length = calc_length
    return _op


def _handle_annotations(simp, args):
    # pylint:disable=isinstance-second-argument-not-valid-type
    ast_args = tuple(a for a in args if isinstance(a, claripy.ast.Base))
    preserved_relocatable = frozenset(simp._relocatable_annotations)
    relocated_annotations = set()
    bad_eliminated = 0

    for aa in ast_args:
        for oa in aa._relocatable_annotations:
            if oa not in preserved_relocatable and oa not in relocated_annotations:
                relocated_annotations.add(oa)
                na = oa.relocate(aa, simp)
                if na is not None:
                    simp = simp.append_annotation(na)

        bad_eliminated += len(aa._uneliminatable_annotations - simp._uneliminatable_annotations)

    if bad_eliminated == 0:
        return simp
    return None


def reversed_op(op_func):
    if type(op_func) is not type(reversed_op):
        op_func = op_func.im_func  # unwrap instancemethod into function

    def _reversed_op(*args):
        return op_func(*args[::-1])

    return _reversed_op


#
# Length checkers
#


def length_same_check(*args):
    return all(a.length == args[0].length for a in args), "args' length must all be equal"


def basic_length_calc(*args):
    return args[0].length


def extract_check(high, low, bv):
    if high < 0 or low < 0:
        return False, "Extract high and low must be nonnegative"
    if low > high:
        return False, "Extract low must be <= high"
    if high >= bv.size():
        return False, "Extract bound must be less than BV size"

    return True, ""


def extend_check(amount, _):
    return amount >= 0, "Extension length must be nonnegative"


def concat_length_calc(*args):
    return sum(arg.length for arg in args)


def extract_length_calc(high, low, _):
    return high + 1 - low


def ext_length_calc(ext, orig):
    return orig.length + ext


#
# Operation lists
#

expression_arithmetic_operations = {
    # arithmetic
    "__add__",
    "__radd__",
    "__truediv__",
    "__rtruediv__",
    "__floordiv__",
    "__rfloordiv__",
    "__mul__",
    "__rmul__",
    "__sub__",
    "__rsub__",
    "__mod__",
    "__rmod__",
    "SDiv",
    "SMod",
    "__neg__",
    "__abs__",
}

bin_ops = {
    "__add__",
    "__radd__",
    "__mul__",
    "__rmul__",
    "__or__",
    "__ror__",
    "__and__",
    "__rand__",
    "__xor__",
    "__rxor__",
}

expression_comparator_operations = {
    # comparisons
    "__eq__",
    "__ne__",
    "UGT",
    "UGE",
    "ULT",
    "ULE",
    "SGT",
    "SGE",
    "SLT",
    "SLE",
}

expression_bitwise_operations = {
    # bitwise
    "__invert__",
    "__or__",
    "__ror__",
    "__and__",
    "__rand__",
    "__xor__",
    "__rxor__",
    "__lshift__",
    "__rlshift__",
    "__rshift__",
    "__rrshift__",
}

expression_set_operations = {
    # Set operations
    "union",
    "intersection",
    "widen",
}

expression_operations = (
    expression_arithmetic_operations
    | expression_comparator_operations
    | expression_bitwise_operations
    | expression_set_operations
)

backend_comparator_operations = {
    "SGE",
    "SLE",
    "SGT",
    "SLT",
    "UGE",
    "ULE",
    "UGT",
    "ULT",
}

backend_bitwise_operations = {
    "RotateLeft",
    "RotateRight",
    "LShR",
    "Reverse",
}

backend_boolean_operations = {"And", "Or", "Not"}

backend_bitmod_operations = {"Concat", "Extract", "SignExt", "ZeroExt"}

backend_creation_operations = {"BoolV", "BVV", "FPV", "StringV"}

backend_symbol_creation_operations = {"BoolS", "BVS", "FPS", "StringS"}

backend_other_operations = {"If"}

backend_arithmetic_operations = {"SDiv", "SMod"}

backend_operations = (
    backend_comparator_operations
    | backend_bitwise_operations
    | backend_boolean_operations
    | backend_bitmod_operations
    | backend_creation_operations
    | backend_other_operations
    | backend_arithmetic_operations
)
backend_operations_vsa_compliant = (
    backend_bitwise_operations | backend_comparator_operations | backend_boolean_operations | backend_bitmod_operations
)
backend_operations_all = backend_operations | backend_operations_vsa_compliant

backend_fp_cmp_operations = {
    "fpLT",
    "fpLEQ",
    "fpGT",
    "fpGEQ",
    "fpEQ",
    "fpNEQ",
}

backend_fp_operations = {
    "FPS",
    "fpToFP",
    "fpToFPUnsigned",
    "fpToIEEEBV",
    "fpFP",
    "fpToSBV",
    "fpToUBV",
    "fpNeg",
    "fpSub",
    "fpAdd",
    "fpMul",
    "fpDiv",
    "fpAbs",
    "fpIsNaN",
    "fpIsInf",
    "fpSqrt",
} | backend_fp_cmp_operations

backend_strings_operations = {
    "StrSubstr",
    "StrReplace",
    "StrConcat",
    "StrLen",
    "StrContains",
    "StrPrefixOf",
    "StrSuffixOf",
    "StrIndexOf",
    "StrToInt",
    "StrIsDigit",
    "IntToStr",
}

opposites = {
    "__add__": "__radd__",
    "__radd__": "__add__",
    "__truediv__": "__rtruediv__",
    "__rtruediv__": "__truediv__",
    "__floordiv__": "__rfloordiv__",
    "__rfloordiv__": "__floordiv__",
    "__mul__": "__rmul__",
    "__rmul__": "__mul__",
    "__sub__": "__rsub__",
    "__rsub__": "__sub__",
    "__mod__": "__rmod__",
    "__rmod__": "__mod__",
    "__eq__": "__eq__",
    "__ne__": "__ne__",
    "ULT": "UGT",
    "UGT": "ULT",
    "ULE": "UGE",
    "UGE": "ULE",
    "SLT": "SGT",
    "SGT": "SLT",
    "SLE": "SGE",
    "SGE": "SLE",
    # '__neg__':
    # '__abs__':
    # '__invert__':
    "__or__": "__ror__",
    "__ror__": "__or__",
    "__and__": "__rand__",
    "__rand__": "__and__",
    "__xor__": "__rxor__",
    "__rxor__": "__xor__",
    "__lshift__": "__rlshift__",
    "__rlshift__": "__lshift__",
    "__rshift__": "__rrshift__",
    "__rrshift__": "__rshift__",
}

reversed_ops = {
    "__radd__": "__add__",
    "__rand__": "__and__",
    "__rfloordiv__": "__floordiv__",
    "__rlshift__": "__lshift__",
    "__rmod__": "__mod__",
    "__rmul__": "__mul__",
    "__ror__": "__or__",
    "__rrshift__": "__rshift__",
    "__rsub__": "__sub__",
    "__rtruediv__": "__truediv__",
    "__rxor__": "__xor__",
}

inverse_operations = {
    "__eq__": "__ne__",
    "__ne__": "__eq__",
    "ULT": "UGE",
    "UGE": "ULT",
    "UGT": "ULE",
    "ULE": "UGT",
    "SLT": "SGE",
    "SGE": "SLT",
    "SLE": "SGT",
    "SGT": "SLE",
}

leaf_operations = backend_symbol_creation_operations | backend_creation_operations
leaf_operations_concrete = backend_creation_operations
leaf_operations_symbolic = backend_symbol_creation_operations
leaf_operations_symbolic_with_union = leaf_operations_symbolic | {"union"}

#
# Reversibility
#

not_invertible = {"union"}
reverse_distributable = {
    "widen",
    "union",
    "intersection",
    "__invert__",
    "__or__",
    "__ror__",
    "__and__",
    "__rand__",
    "__xor__",
    "__rxor__",
}

infix = {
    "__add__": "+",
    "__sub__": "-",
    "__mul__": "*",
    "__floordiv__": "/",
    "__truediv__": "/",  # the raw / operator should use integral semantics on bitvectors
    "__mod__": "%",
    "__eq__": "==",
    "__ne__": "!=",
    "UGE": ">=",
    "ULE": "<=",
    "UGT": ">",
    "ULT": "<",
    "SGE": ">=s",
    "SLE": "<=s",
    "SGT": ">s",
    "SLT": "<s",
    "SDiv": "/s",
    "SMod": "%s",
    "__or__": "|",
    "__and__": "&",
    "__xor__": "^",
    "__lshift__": "<<",
    "__rshift__": ">>",
    "And": "&&",
    "Or": "||",
    "Concat": "..",
}

prefix = {
    "Not": "!",
    "__neg__": "-",
    "__invert__": "~",
}

op_precedence = {  # based on https://en.cppreference.com/w/c/language/operator_precedence
    # precedence: 2
    "Not": 2,
    "__neg__": 2,
    "__invert__": 2,
    # precedence: 3
    "__mul__": 3,
    "__floordiv__": 3,
    "__truediv__": 3,  # the raw / operator should use integral semantics on bitvectors
    "__mod__": 3,
    "SDiv": 3,
    "SMod": 3,
    # precedence: 4
    "__add__": 4,
    "__sub__": 4,
    # precedence: 5
    "__lshift__": 5,
    "__rshift__": 5,
    # precedence: 6
    "__ge__": 6,
    "__le__": 6,
    "__gt__": 6,
    "__lt__": 6,
    "UGE": 6,
    "ULE": 6,
    "UGT": 6,
    "ULT": 6,
    "SGE": 6,
    "SLE": 6,
    "SGT": 6,
    "SLT": 6,
    # precedence: 7
    "__eq__": 7,
    "__ne__": 7,
    # precedence: 8
    "__and__": 8,
    # precedence: 9
    "__xor__": 9,
    # precedence: 10
    "__or__": 10,
    # precedence: 11
    "And": 11,
    # precedence: 12
    "Or": 12,
    # 'Concat': '..',
}

commutative_operations = {
    "__and__",
    "__or__",
    "__xor__",
    "__add__",
    "__mul__",
    "And",
    "Or",
    "Xor",
}

bound_ops = {
    "Not": "__invert__",
    "And": "__and__",
    "Or": "__or__",
}
