def data_ref_type_str(dref_enum):
    """
    Translate an ``enum DataRefTypes`` value into a string representation.
    """
    if dref_enum == 0x9000:
        return "unknown"
    elif dref_enum == 0x9001:
        return "integer"
    elif dref_enum == 0x9002:
        return "fp"
    elif dref_enum == 0x9003:
        return "integer(store)"
    else:
        return "INVALID"


class DataRef:
    """
    A data reference object. Indicates a data access in an IRSB.

    :ivar data_addr:    The address of the data being accessed
    :ivar data_size:    The size of the data being accessed, in bytes
    :ivar data_type:    The type of the data, a DataRefTypes enum.
    :ivar stmt_idx:     The IRSB statement index containing the data access
    :ivar ins_addr:     The address of the instruction performing the data access
    """

    __slots__ = ("data_addr", "data_size", "data_type", "stmt_idx", "ins_addr")

    def __init__(self, data_addr, data_size, data_type, stmt_idx, ins_addr):
        self.data_addr = data_addr
        self.data_size = data_size
        self.data_type = data_type
        self.stmt_idx = stmt_idx
        self.ins_addr = ins_addr

    @property
    def data_type_str(self):
        """
        The data ref type as a string, "unknown" "integer" "fp" or "INVALID"
        """
        return data_ref_type_str(self.data_type)

    def __repr__(self):
        return "<DataRef accessing %#x %s:%d at %#x:%d>" % (
            self.data_addr,
            data_ref_type_str(self.data_type),
            self.data_size,
            self.ins_addr,
            self.stmt_idx,
        )

    @classmethod
    def from_c(cls, r):
        return cls(r.data_addr, r.size, r.data_type, r.stmt_idx, r.ins_addr)
