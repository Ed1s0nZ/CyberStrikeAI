from __future__ import annotations


class AddressTranslator:
    __slots__ = (
        "_rva",
        "_owner",
    )

    """
    Mediates address translations between typed addresses such as RAW, RVA, LVA, MVA and VA
    including address owner and its state (linked or mapped)

    Semantics::

        owner - object associated with the address
            (any object class based on `cle.Backend`)
        owner mapping state - sparse object can be either mapped or not
            (actual object's image base VA to be considered valid)
        RAW - offset (index) inside a file stream
        VA  - address inside process flat virtual memory space
        RVA - address relative to the object's segment base
            (segment base normalized virtual address)
        LVA - linked VA (linker)
        MVA - mapped VA (loader)
    """

    def __init__(self, rva, owner):
        """
        :param rva: virtual address relative to owner's object image base
        :type rva: int
        :param owner: The object owner address relates to
        :type owner: cle.Backend
        """
        self._rva, self._owner = rva, owner

    @classmethod
    def from_lva(cls, lva, owner):
        """
        Loads address translator with LVA
        """
        return cls(lva - owner.linked_base, owner)

    @classmethod
    def from_mva(cls, mva, owner):
        """
        Loads address translator with MVA
        """
        return cls(mva - owner.mapped_base, owner)

    @classmethod
    def from_rva(cls, rva, owner):
        """
        Loads address translator with RVA
        """
        return cls(rva, owner)

    @classmethod
    def from_raw(cls, raw, owner):
        """
        Loads address translator with RAW address
        """
        return cls(owner.offset_to_addr(raw) - (owner.mapped_base if owner._is_mapped else owner.linked_base), owner)

    from_linked_va = from_lva
    from_va = from_mapped_va = from_mva
    from_relative_va = from_rva

    def to_lva(self):
        """
        VA -> LVA
        :rtype: int
        """
        return self._rva + self._owner.linked_base

    def to_mva(self):
        """
        RVA -> MVA
        :rtype: int
        """
        return self._rva + self._owner.mapped_base

    def to_rva(self):
        """
        RVA -> RVA
        :rtype: int
        """
        return self._rva

    def to_raw(self):
        """
        RVA -> RAW
        :rtype: int
        """
        return self._owner.addr_to_offset(
            self._rva + (self._owner.mapped_base if self._owner._is_mapped else self._owner.linked_base)
        )

    to_linked_va = to_lva
    to_va = to_mapped_va = to_mva
    to_relative_va = to_rva


AT = AddressTranslator
