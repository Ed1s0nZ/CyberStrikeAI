class ConstVal:
    """
    A constant value object. Indicates a constant value assignment to a VEX tmp variable.

    :ivar tmp:          The tmp variable being assigned to.
    :ivar value:        The value of the tmp variable.
    :ivar stmt_idx:     The IRSB statement index containing the data access
    """

    __slots__ = (
        "tmp",
        "value",
        "stmt_idx",
    )

    def __init__(self, tmp: int, value: int, stmt_idx: int):
        self.tmp = tmp
        self.value = value
        self.stmt_idx = stmt_idx

    def __repr__(self):
        return f"<ConstVal {self.tmp} = {self.value:#x} @ {self.stmt_idx}>"

    @classmethod
    def from_c(cls, r):
        return cls(r.tmp, r.value, r.stmt_idx)
