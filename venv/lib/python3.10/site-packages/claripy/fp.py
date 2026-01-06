from __future__ import annotations

import decimal
from enum import Enum

from claripy.errors import ClaripyOperationError


class RM(Enum):
    """Rounding modes for floating point operations.

    See https://en.wikipedia.org/wiki/IEEE_754#Rounding_rules for more information.
    """

    RM_NearestTiesEven = "RM_RNE"
    RM_NearestTiesAwayFromZero = "RM_RNA"
    RM_TowardsZero = "RM_RTZ"
    RM_TowardsPositiveInf = "RM_RTP"
    RM_TowardsNegativeInf = "RM_RTN"

    @staticmethod
    def default():
        return RM.RM_NearestTiesEven

    def pydecimal_equivalent_rounding_mode(self):
        return {
            RM.RM_TowardsPositiveInf: decimal.ROUND_CEILING,
            RM.RM_TowardsNegativeInf: decimal.ROUND_FLOOR,
            RM.RM_TowardsZero: decimal.ROUND_DOWN,
            RM.RM_NearestTiesEven: decimal.ROUND_HALF_EVEN,
            RM.RM_NearestTiesAwayFromZero: decimal.ROUND_UP,
        }[self]


class FSort:
    """A class representing a floating point sort."""

    def __init__(self, name, exp, mantissa):
        self.name = name
        self.exp = exp
        self.mantissa = mantissa

    def __eq__(self, other):
        return self.exp == other.exp and self.mantissa == other.mantissa

    def __repr__(self):
        return self.name

    def __hash__(self):
        return hash((self.name, self.exp, self.mantissa))

    @property
    def length(self):
        return self.exp + self.mantissa

    @staticmethod
    def from_size(n):
        if n == 32:
            return FSORT_FLOAT
        if n == 64:
            return FSORT_DOUBLE
        raise ClaripyOperationError(f"{n} is not a valid FSort size")

    @staticmethod
    def from_params(exp, mantissa):
        if exp == 8 and mantissa == 24:
            return FSORT_FLOAT
        if exp == 11 and mantissa == 53:
            return FSORT_DOUBLE
        raise ClaripyOperationError("unrecognized FSort params")


FSORT_FLOAT = FSort("FLOAT", 8, 24)
FSORT_DOUBLE = FSort("DOUBLE", 11, 53)
