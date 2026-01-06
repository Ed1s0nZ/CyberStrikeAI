from __future__ import annotations

import struct


class ELFHashTable:
    """
    Functions to do lookup from a HASH section of an ELF file.

    Information: http://docs.oracle.com/cd/E23824_01/html/819-0690/chapter6-48031.html
    """

    def __init__(self, symtab, stream, offset, arch):
        """
        :param symtab:  The symbol table to perform lookups from (as a pyelftools SymbolTableSection).
        :param stream:  A file-like object to read from the ELF's memory.
        :param offset:  The offset in the object where the table starts.
        :param arch:    The ArchInfo object for the ELF file.
        """
        self.symtab = symtab
        fmt = "<" if arch.memory_endness == "Iend_LE" else ">"
        stream.seek(offset)
        self.nbuckets, self.nchains = struct.unpack(fmt + "II", stream.read(8))
        self.buckets = struct.unpack(fmt + "I" * self.nbuckets, stream.read(4 * self.nbuckets))
        self.chains = struct.unpack(fmt + "I" * self.nchains, stream.read(4 * self.nchains))

    def get(self, k):
        """
        Perform a lookup. Returns a pyelftools Symbol object, or None if there is no match.

        :param k:   The string to look up.
        """
        if self.nbuckets == 0:
            return None, None
        hval = self.elf_hash(k) % self.nbuckets
        symndx = self.buckets[hval]
        while symndx != 0:
            sym = self.symtab.get_symbol(symndx)
            if sym.name == k:
                return symndx, sym
            symndx = self.chains[symndx]
        return None, None

    # from http://www.partow.net/programming/hashfunctions/
    @staticmethod
    def elf_hash(key):
        h = 0
        x = 0
        for c in key:
            h = (h << 4) + ord(c)
            x = h & 0xF0000000
            if x != 0:
                h ^= x >> 24
            h &= ~x
        return h


class GNUHashTable:
    """
    Functions to do lookup from a GNU_HASH section of an ELF file.

    Information: https://blogs.oracle.com/ali/entry/gnu_hash_elf_sections
    """

    def __init__(self, symtab, stream, offset, arch):
        """
        :param symtab:       The symbol table to perform lookups from (as a pyelftools SymbolTableSection).
        :param stream:       A file-like object to read from the ELF's memory.
        :param offset:       The offset in the object where the table starts.
        :param arch:         The ArchInfo object for the ELF file.
        """
        self.symtab = symtab
        fmt = "<" if arch.memory_endness == "Iend_LE" else ">"
        self.c = arch.bits
        fmtsz = "I" if self.c == 32 else "Q"

        stream.seek(offset)
        data = stream.read(16)
        self.nbuckets, self.symndx, self.maskwords, self.shift2 = struct.unpack(fmt + "IIII", data)

        self.bloom = struct.unpack(fmt + fmtsz * self.maskwords, stream.read(self.c * self.maskwords // 8))
        self.buckets = struct.unpack(fmt + "I" * self.nbuckets, stream.read(4 * self.nbuckets))
        self.hash_ptr = stream.tell()
        self.stream = stream

    def _matches_bloom(self, H1):
        C = self.c
        H2 = H1 >> self.shift2
        N = (H1 // C) & (self.maskwords - 1)
        BITMASK = (1 << (H1 % C)) | (1 << (H2 % C))
        return (self.bloom[N] & BITMASK) == BITMASK

    def get(self, k):
        """
        Perform a lookup. Returns a pyelftools Symbol object, or None if there is no match.

        :param k:        The string to look up
        """
        h = self.gnu_hash(k)
        if not self._matches_bloom(h):
            return None, None
        n = self.buckets[h % self.nbuckets]
        if n == 0:
            return None, None
        while True:
            sym = self.symtab.get_symbol(n)
            if sym.name == k:
                return n, sym
            self.stream.seek(self.hash_ptr + 4 * (n - self.symndx))
            if struct.unpack("I", self.stream.read(4))[0] & 1 == 1:
                break
            n += 1
        return None, None

    @staticmethod
    def gnu_hash(key):
        h = 5381
        for c in key:
            h = h * 33 + ord(c)
        return h & 0xFFFFFFFF
