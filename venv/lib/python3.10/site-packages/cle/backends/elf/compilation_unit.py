from __future__ import annotations

import os.path

from cle.address_translator import AT

from .subprogram import Subprogram
from .variable import Variable


class CompilationUnit:
    """
    CompilationUnit for DWARF
    See http://dwarfstd.org/doc/DWARF5.pdf page 60
    """

    def __init__(self, name, comp_dir, low_pc, high_pc, language, elf_object, ranges):
        self.name = name
        self.comp_dir = comp_dir
        self.file_path = os.path.join(self.comp_dir, self.name)
        self.low_pc = low_pc
        self.high_pc = high_pc
        self.ranges = ranges
        self.language = language
        self.functions: dict[int, Subprogram] = {}
        self.global_variables: list[Variable] = []
        self._elf_object = elf_object

    @property
    def min_addr(self):
        return AT.from_rva(self.low_pc, self._elf_object).to_mva()

    @property
    def max_addr(self):
        if self.high_pc is not None:
            pc = self.high_pc
        elif self.ranges is not None:
            pc = max(x for _, x in self.ranges)
        else:
            raise ValueError("Cannot compute max_addr of CompilationUnit without either high_pc or ranges")
        return AT.from_rva(pc, self._elf_object).to_mva()
