from __future__ import annotations

from claripy import operations

from .base import Base, _make_name
from .bool import Bool
from .bv import BV


class String(Base):
    """
    Base class that represent the AST of a String object and implements all the
    operation useful to create and modify the AST.

    Do not instantiate this class directly, instead use StringS or StringV to
    construct a symbol or value, and then use operations to construct more
    complicated expressions.
    """

    __slots__ = ()

    @staticmethod
    def _from_str(like, value):  # pylint: disable=unused-argument
        return StringV(value)

    def strReplace(self, str_to_replace, replacement):
        """
        Replace the first occurence of str_to_replace with replacement

        :param claripy.ast.String str_to_replace: pattern that has to be replaced
        :param claripy.ast.String replacement: replacement pattern
        """
        return StrReplace(self, str_to_replace, replacement)

    def toInt(self):
        """
        Convert the string to a bitvector holding the integer
        representation of the string
        """
        return StrToInt(self)

    def indexOf(self, pattern, start_idx):
        """
        Return the start index of the pattern inside the input string in a
        Bitvector representation, otherwise it returns -1 (always using a BitVector)
        """
        return StrIndexOf(self, pattern, start_idx)


def StringS(name, explicit_name=False, **kwargs):
    """
    Create a new symbolic string (analogous to z3.String())

    :param name:                 The name of the symbolic string (i. e. the name of the variable)
    :param bool explicit_name:   If False, an identifier is appended to the name to ensure uniqueness.

    :returns:                    The String object representing the symbolic string
    """
    n = _make_name(name, 0, False if explicit_name is None else explicit_name)
    return String(
        "StringS",
        (n,),
        symbolic=True,
        variables=frozenset((n,)),
        **kwargs,
    )


def StringV(value, **kwargs):
    """
    Create a new Concrete string (analogous to z3.StringVal())

    :param value:  The constant value of the concrete string

    :returns:      The String object representing the concrete string
    """

    return String("StringV", (value,), **kwargs)


StrConcat = operations.op("StrConcat", String, String)
StrSubstr = operations.op("StrSubstr", (BV, BV, String), String)
StrLen = operations.op("StrLen", (String), BV, calc_length=lambda *_: 64)
StrReplace = operations.op("StrReplace", (String, String, String), String)
StrContains = operations.op("StrContains", (String, String), Bool)
StrPrefixOf = operations.op("StrPrefixOf", (String, String), Bool)
StrSuffixOf = operations.op("StrSuffixOf", (String, String), Bool)
StrIndexOf = operations.op("StrIndexOf", (String, String, BV), BV, calc_length=lambda *_: 64)
StrToInt = operations.op("StrToInt", (String), BV, calc_length=lambda *_: 64)
IntToStr = operations.op("IntToStr", (BV,), String)
StrIsDigit = operations.op("StrIsDigit", (String,), Bool)

# Equality / inequality check
String.__eq__ = operations.op("__eq__", (String, String), Bool)
String.__ne__ = operations.op("__ne__", (String, String), Bool)

# String manipulation
String.__add__ = StrConcat
String.StrSubstr = staticmethod(StrSubstr)
String.StrConcat = staticmethod(StrConcat)
String.StrLen = staticmethod(StrLen)
String.StrReplace = staticmethod(StrReplace)
String.StrContains = staticmethod(StrContains)
String.StrPrefixOf = staticmethod(StrPrefixOf)
String.StrSuffixOf = staticmethod(StrSuffixOf)
String.StrIndexOf = staticmethod(StrIndexOf)
String.StrToInt = staticmethod(StrToInt)
String.StrIsDigit = staticmethod(StrIsDigit)
String.IntToStr = staticmethod(IntToStr)
