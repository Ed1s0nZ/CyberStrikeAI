from __future__ import annotations


class BoolResult:
    """A class representing the result of a boolean operation. Values can be
    True, False, or Maybe.
    """

    value: tuple[bool, ...]

    def __init__(self, value: tuple[bool, ...]):
        self.value = value

    def __eq__(self, other):
        return isinstance(other, BoolResult) and self.value == other.value

    def __and__(self, other):
        if BoolResult.is_false(self) or BoolResult.is_false(other):
            return FalseResult()
        if BoolResult.is_true(self):
            return other
        if BoolResult.is_true(other):
            return self
        return MaybeResult()

    def __invert__(self):
        if BoolResult.is_true(self):
            return FalseResult()
        if BoolResult.is_false(self):
            return TrueResult()
        return MaybeResult()

    def __or__(self, other):
        if BoolResult.is_true(self) or BoolResult.is_true(other):
            return TrueResult()
        if BoolResult.is_false(self):
            return other
        if BoolResult.is_false(other):
            return self
        return MaybeResult()

    def identical(self, other):
        return self.value == other.value

    def union(self, other):
        return BoolResult(tuple(set(self.value) | set(other.value)))

    @property
    def cardinality(self):
        return len(self.value)

    @staticmethod
    def is_maybe(o):
        return isinstance(o, BoolResult) and False in o.value and True in o.value

    @staticmethod
    def has_true(o):
        return o is True or (isinstance(o, BoolResult) and True in o.value)

    @staticmethod
    def has_false(o):
        return o is False or (isinstance(o, BoolResult) and False in o.value)

    @staticmethod
    def is_true(o):
        return o is True or (isinstance(o, BoolResult) and o.value == (True,))

    @staticmethod
    def is_false(o):
        return o is False or (isinstance(o, BoolResult) and o.value == (False,))


def TrueResult() -> BoolResult:
    """Return a BoolResult representing the value True."""

    return BoolResult((True,))


def FalseResult() -> BoolResult:
    """Return a BoolResult representing the value False."""

    return BoolResult((False,))


def MaybeResult() -> BoolResult:
    """Return a BoolResult representing the value Maybe."""

    return BoolResult((True, False))
