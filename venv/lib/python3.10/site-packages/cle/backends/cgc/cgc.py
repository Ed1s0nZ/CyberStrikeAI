from __future__ import annotations

from cle.address_translator import AT
from cle.backends.backend import register_backend
from cle.backends.elf import ELF
from cle.patched_stream import PatchedStream

ELF_HEADER = bytes.fromhex("7f454c46010101000000000000000000")
CGC_HEADER = bytes.fromhex("7f43474301010143014d6572696e6f00")


class CGC(ELF):
    """
    Backend to support the CGC elf format used by the Cyber Grand Challenge competition.

    See : https://github.com/CyberGrandChallenge/libcgcef/blob/master/cgc_executable_format.md
    """

    is_default = True  # Tell CLE to automatically consider using the CGC backend

    def __init__(self, binary, binary_stream, *args, **kwargs):
        binary_stream = PatchedStream(binary_stream, [(0, ELF_HEADER)])
        super().__init__(binary, binary_stream, *args, **kwargs)
        self.memory.store(AT.from_raw(0, self).to_rva(), CGC_HEADER)  # repair the CGC header
        self.os = "cgc"
        self.execstack = True  # the stack is always executable in CGC

    @staticmethod
    def is_compatible(stream):
        stream.seek(0)
        identstring = stream.read(4)
        stream.seek(0)
        if identstring.startswith(b"\x7fCGC"):
            return True
        return False

    def _load_segment(self, seg):
        if seg.header.p_memsz > 0:
            super()._load_segment(seg)

    supported_filetypes = ["cgc"]


register_backend("cgc", CGC)
