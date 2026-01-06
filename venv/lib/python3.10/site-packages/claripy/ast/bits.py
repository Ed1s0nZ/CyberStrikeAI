from __future__ import annotations

from typing import cast

from claripy.ast.base import Base
from claripy.errors import ClaripyOperationError


class Bits(Base):
    """
    A base class for AST types that can be stored as a series of bits. Currently, this is bitvectors and IEEE floats.

    :ivar length:       The length of this value in bits.
    """

    __slots__ = ()

    def make_like(self, op, args, **kwargs):
        if "length" not in kwargs:
            kwargs["length"] = self.length
        return Base.make_like(self, op, args, **kwargs)

    def size(self) -> int:
        """
        :returns: The bit length of this AST
        """
        return cast("int", self.length)

    def _type_name(self):
        return self.__class__.__name__ + str(self.length)

    def raw_to_bv(self):
        """
        Converts this data's bit-pattern to a bitvector.
        """
        raise NotImplementedError

    def raw_to_fp(self):
        """
        Converts this data's bit-pattern to an IEEE float.
        """
        raise NotImplementedError

    @staticmethod
    def _check_replaceability(old, new):
        if old.size() != new.size():
            raise ClaripyOperationError("replacements must have matching sizes")

    __len__ = size
