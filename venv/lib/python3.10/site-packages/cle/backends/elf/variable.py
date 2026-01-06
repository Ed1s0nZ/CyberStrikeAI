from __future__ import annotations

from typing import TYPE_CHECKING

from elftools.dwarf.die import DIE

from cle.address_translator import AT

from .variable_type import VariableType

if TYPE_CHECKING:
    from .elf import ELF
    from .subprogram import LexicalBlock


class Variable:
    """
    Variable for DWARF
    from a DW_TAG_variable or DW_TAG_formal_parameter

    :ivar str name:         The name of the variable
    :ivar relative_addr:    The relative addr (base addr depends on the type)
    :ivar lexical_block:    For a local variable, the lexical block where the variable is declared
    """

    def __init__(self, elf_object: ELF):
        self._elf_object = elf_object
        # all other optional params can be set afterwards
        self.relative_addr: int | None = None
        self.name: str | None = None
        self._type_offset = None
        self.decl_line: int | None = None
        self.decl_file: str | None = None
        self.lexical_block: LexicalBlock | None = None
        self.external = False
        self.declaration_only = False

    @staticmethod
    def from_die(
        die: DIE,
        expr_parser,
        elf_object: ELF,
        lexical_block: LexicalBlock | None = None,
        namespace: list[str] | None = None,
    ):
        if namespace is None:
            namespace = []
        # first the address
        if "DW_AT_location" in die.attributes and die.attributes["DW_AT_location"].form == "DW_FORM_exprloc":
            parsed_exprs = expr_parser.parse_expr(die.attributes["DW_AT_location"].value)
            if len(parsed_exprs) == 1 and parsed_exprs[0].op_name == "DW_OP_addr":
                addr = parsed_exprs[0].args[0]
                var = MemoryVariable(elf_object, addr)
            elif len(parsed_exprs) == 1 and parsed_exprs[0].op_name == "DW_OP_fbreg":
                addr = parsed_exprs[0].args[0]
                var = StackVariable(elf_object, addr)
            elif len(parsed_exprs) == 1 and parsed_exprs[0].op_name.startswith("DW_OP_reg"):
                addr = parsed_exprs[0].op - 0x50  # 0x50 == DW_OP_reg0
                var = RegisterVariable(elf_object, addr)
            else:
                var = Variable(elf_object)
        else:
            var = Variable(elf_object)

        if "DW_AT_name" in die.attributes:
            var.name = "::".join(namespace + [die.attributes["DW_AT_name"].value.decode("utf-8")])
        if "DW_AT_type" in die.attributes:
            var._type_offset = die.attributes["DW_AT_type"].value + die.cu.cu_offset
        if "DW_AT_decl_line" in die.attributes:
            var.decl_line = die.attributes["DW_AT_decl_line"].value
        if "DW_AT_external" in die.attributes:
            var.external = True
        if "DW_AT_declaration" in die.attributes:
            var.declaration_only = True

        var.lexical_block = lexical_block

        return var

    # overwritten for stack variables
    def rebased_addr_from_cfa(self, cfa: int):
        """
        The address of this variable in the global memory.

        :param cfa:     The canonical frame address as described by the DWARF standard.
        """
        return self.rebased_addr

    @property
    def rebased_addr(self):
        return None

    @property
    def addr(self):
        """
        Please use 'relative_addr' or 'rebased_addr' instead.
        """
        return self.relative_addr

    @property
    def type(self) -> VariableType:
        try:
            return self._elf_object.type_list[self._type_offset]
        except KeyError:
            return None

    @property
    def sort(self) -> str:
        # sort = 'stack' | 'register' | 'global'
        return "unknown"


class MemoryVariable(Variable):
    """
    This includes all variables that are not on the stack and not in a register.
    So all global variables, and also local static variables in C!
    """

    def __init__(self, elf_object: ELF, relative_addr):
        super().__init__(elf_object)
        self.relative_addr = relative_addr

    @property
    def rebased_addr(self):
        return AT.from_rva(self.relative_addr, self._elf_object).to_mva()

    @property
    def sort(self) -> str:
        return "global"


class StackVariable(Variable):
    """
    Stack Variable from DWARF.
    """

    def __init__(self, elf_object: ELF, relative_addr):
        super().__init__(elf_object)
        self.relative_addr = relative_addr

    def rebased_addr_from_cfa(self, cfa: int):
        return self.relative_addr + cfa

    @property
    def sort(self) -> str:
        return "stack"


class RegisterVariable(Variable):
    """
    Register Variable from DWARF.
    """

    def __init__(self, elf_object: ELF, register_addr):
        super().__init__(elf_object)
        # FIXME should this really go into relative addr?
        self.relative_addr = register_addr

    @property
    def sort(self) -> str:
        return "register"
