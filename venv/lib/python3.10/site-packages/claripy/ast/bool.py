from __future__ import annotations

import logging
from functools import lru_cache
from typing import TYPE_CHECKING, TypeVar, overload

import claripy
from claripy import operations
from claripy.algorithm.bool_check import is_false, is_true
from claripy.ast.base import Base, _make_name
from claripy.errors import ClaripyTypeError

from .bits import Bits

if TYPE_CHECKING:
    from .bv import BV
    from .fp import FP

T = TypeVar("T", bound=Base)
log = logging.getLogger(__name__)


class Bool(Base):
    """Bool is the AST class for a boolean value."""

    __slots__ = ()

    @staticmethod
    def _from_bool(like, val):  # pylint:disable=unused-argument
        return BoolV(val)

    def is_true(self):
        """
        Returns True if 'self' can be easily determined to be True. Otherwise, return False. Note that the AST *might*
        still be True (i.e., if it were simplified via Z3), but it's hard to quickly tell that.
        """
        return is_true(self)

    def is_false(self):
        """
        Returns True if 'self' can be easily determined to be False. Otherwise, return False. Note that the AST *might*
        still be False (i.e., if it were simplified via Z3), but it's hard to quickly tell that.
        """
        return is_false(self)

    def size(self):  # pylint:disable=no-self-use
        """Returns the size of the AST in bits. A boolean is 1 bit."""
        return 1

    __len__ = size


def BoolS(name, explicit_name=None) -> Bool:
    """
    Creates a boolean symbol (i.e., a variable).

    :param name:            The name of the symbol
    :param explicit_name:   If False, an identifier is appended to the name to ensure uniqueness.

    :return:                A Bool object representing this symbol.
    """
    n = _make_name(name, -1, False if explicit_name is None else explicit_name)
    return Bool("BoolS", (n,), variables=frozenset((n,)), symbolic=True)


@lru_cache(maxsize=2)
def BoolV(val) -> Bool:
    return Bool("BoolV", (val,))


def true():
    return BoolV(True)


def false():
    return BoolV(False)


#
# Bound operations
#


Bool.__eq__ = operations.op("__eq__", (Bool, Bool), Bool)
Bool.__ne__ = operations.op("__ne__", (Bool, Bool), Bool)
Bool.intersection = operations.op("intersection", (Bool, Bool), Bool)

#
# Unbound operations
#


@overload
def If(cond: bool | Bool, true_value: bool | Bool, false_value: bool | Bool) -> Bool: ...
@overload
def If(cond: bool | Bool, true_value: int | BV, false_value: int | BV) -> BV: ...
@overload
def If(cond: bool | Bool, true_value: float | FP, false_value: float | FP) -> FP: ...
@overload
def If(cond: bool | Bool, true_value: T, false_value: T) -> T: ...


def If(cond, true_value, false_value):
    # the coercion here is strange enough that we'll just implement it manually
    args = [cond, true_value, false_value]

    if isinstance(args[0], bool):
        args[0] = BoolV(args[0])

    ty = None
    if isinstance(args[1], Base):
        ty = type(args[1])
    elif isinstance(args[2], Base):
        ty = type(args[2])
    else:
        raise ClaripyTypeError("true/false clause of If must have bearable types")

    if isinstance(args[1], Bits) and isinstance(args[2], Bits) and args[1].length != args[2].length:
        raise ClaripyTypeError("sized arguments to If must have the same length")

    if not isinstance(args[1], ty):
        if hasattr(ty, "_from_" + type(args[1]).__name__):
            convert = getattr(ty, "_from_" + type(args[1]).__name__)
            args[1] = convert(args[2], args[1])
        else:
            raise ClaripyTypeError(f"can't convert {type(args[1])} to {ty}")
    if not isinstance(args[2], ty):
        if hasattr(ty, "_from_" + type(args[2]).__name__):
            convert = getattr(ty, "_from_" + type(args[2]).__name__)
            args[2] = convert(args[1], args[2])
        else:
            raise ClaripyTypeError(f"can't convert {type(args[2])} to {ty}")

    if is_true(args[0]):
        return args[1].append_annotations(args[0].annotations)
    if is_false(args[0]):
        return args[2].append_annotations(args[0].annotations)

    if isinstance(args[1], Base) and args[1].op == "If" and args[1].args[0] is args[0]:
        return If(args[0], args[1].args[1], args[2])
    if isinstance(args[1], Base) and args[1].op == "If" and args[1].args[0] is Not(args[0]):
        return If(args[0], args[1].args[2], args[2])
    if isinstance(args[2], Base) and args[2].op == "If" and args[2].args[0] is args[0]:
        return If(args[0], args[1], args[2].args[2])
    if isinstance(args[2], Base) and args[2].op == "If" and args[2].args[0] is Not(args[0]):
        return If(args[0], args[1], args[2].args[1])

    if args[1] is args[2]:
        return args[1]
    if args[1] is true() and args[2] is false():
        return args[0]
    if args[1] is false() and args[2] is true():
        return ~args[0]

    if issubclass(ty, Bits):
        return ty("If", tuple(args), length=args[1].length)
    return ty("If", tuple(args))


And = operations.op("And", Bool, Bool)
Or = operations.op("Or", Bool, Bool)
Not = operations.op("Not", (Bool,), Bool)

Bool.__invert__ = Not
Bool.__and__ = And
Bool.__rand__ = And
Bool.__or__ = Or
Bool.__ror__ = Or


# For large tables, ite_dict that uses a binary search tree instead of a "linear" search tree.
# This improves Z3 search capability (eliminating branches) and decreases recursion depth:
# linear search trees make Z3 error out on tables larger than a couple hundred elements.)
def ite_dict(i, d, default):
    """
    Return an expression of if-then-else trees which expresses a switch tree
    :param i: The variable which may take on multiple values affecting the final result
    :param d: A dict mapping possible values for i to values which the result could be
    :param default: A default value that the expression should take on if `i` matches none of the keys of `d`
    :return: An expression encoding the result of the above
    """
    # for small dicts fall back to the linear implementation
    if len(d) < 4:
        return ite_cases([(i == c, v) for c, v in d.items()], default)

    # otherwise, binary search.
    # Find the median:
    keys = list(d.keys())
    keys.sort()
    split_val = keys[(len(keys) - 1) // 2]

    # split the dictionary
    dictLow = {c: v for c, v in d.items() if c <= split_val}
    dictHigh = {c: v for c, v in d.items() if c > split_val}

    valLow = ite_dict(i, dictLow, default)
    valHigh = ite_dict(i, dictHigh, default)
    return If(i <= split_val, valLow, valHigh)


def ite_cases(cases, default):
    """
    Return an expression of if-then-else trees which expresses a series of alternatives

    :param cases: A list of tuples (c, v). `c` is the condition under which `v` should be the result of the expression
    :param default: A default value that the expression should take on if none of the `c` conditions are satisfied
    :return: An expression encoding the result of the above
    """
    sofar = default
    for c, v in reversed(list(cases)):
        if is_true(v == sofar):
            continue
        sofar = If(c, v, sofar)
    return sofar


def reverse_ite_cases(ast):
    """
    Given an expression created by `ite_cases`, produce the cases that generated it
    :param ast:
    :return:
    """
    queue = [(true(), ast)]
    while queue:
        condition, ast = queue.pop(0)
        if ast.op == "If":
            queue.append((And(condition, ast.args[0]), ast.args[1]))
            queue.append((And(condition, Not(ast.args[0])), ast.args[2]))
        else:
            yield condition, ast


def constraint_to_si(expr):
    """
    Convert a constraint to SI if possible.

    :param expr:
    :return:
    """

    satisfiable = True
    replace_list = []

    satisfiable, replace_list = claripy.backends.vsa.constraint_to_si(expr)

    # Make sure the replace_list are all ast.bvs
    for i in range(len(replace_list)):  # pylint:disable=consider-using-enumerate
        ori, new = replace_list[i]
        if not isinstance(new, Base):
            new = claripy.BVS(
                new.name, new._bits, min=new._lower_bound, max=new._upper_bound, stride=new._stride, explicit_name=True
            )
            replace_list[i] = (ori, new)

    return satisfiable, replace_list
