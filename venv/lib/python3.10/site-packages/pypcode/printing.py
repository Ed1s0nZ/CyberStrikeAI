from __future__ import annotations

from .pypcode_native import (  # pylint:disable=no-name-in-module
    Disassembly,
    Instruction,
    OpCode,
    PcodeOp,
    Translation,
    Varnode,
)


class OpFormat:
    """
    General op pretty-printer.
    """

    @staticmethod
    def fmt_vn(vn: Varnode) -> str:
        if vn.space.name == "const":
            return "%#x" % vn.offset
        elif vn.space.name == "register":
            name = vn.getRegisterName()
            if name:
                return name
        return f"{vn.space.name}[{vn.offset:x}:{vn.size:d}]"

    def fmt(self, op: PcodeOp) -> str:
        return f'{op.opcode.__name__} {", ".join(self.fmt_vn(i) for i in op.inputs)}'


class OpFormatUnary(OpFormat):
    """
    General unary op pretty-printer.
    """

    __slots__ = ("operator",)

    def __init__(self, operator: str):
        super().__init__()
        self.operator = operator

    def fmt(self, op: PcodeOp) -> str:
        return f"{self.operator}{self.fmt_vn(op.inputs[0])}"


class OpFormatBinary(OpFormat):
    """
    General binary op pretty-printer.
    """

    __slots__ = ("operator",)

    def __init__(self, operator: str):
        super().__init__()
        self.operator = operator

    def fmt(self, op: PcodeOp) -> str:
        return f"{self.fmt_vn(op.inputs[0])} {self.operator} {self.fmt_vn(op.inputs[1])}"


class OpFormatFunc(OpFormat):
    """
    Function-call style op pretty-printer.
    """

    __slots__ = ("operator",)

    def __init__(self, operator: str):
        super().__init__()
        self.operator = operator

    def fmt(self, op: PcodeOp) -> str:
        return f'{self.operator}({", ".join(self.fmt_vn(i) for i in op.inputs)})'


class OpFormatSpecial(OpFormat):
    """
    Specialized op pretty-printers.
    """

    def fmt_BRANCH(self, op: PcodeOp) -> str:
        return f"goto {self.fmt_vn(op.inputs[0])}"

    def fmt_BRANCHIND(self, op: PcodeOp) -> str:
        return f"goto [{self.fmt_vn(op.inputs[0])}]"

    def fmt_CALL(self, op: PcodeOp) -> str:
        return f"call {self.fmt_vn(op.inputs[0])}"

    def fmt_CALLIND(self, op: PcodeOp) -> str:
        return f"call [{self.fmt_vn(op.inputs[0])}]"

    def fmt_CALLOTHER(self, op: PcodeOp) -> str:
        return f'{op.inputs[0].getUserDefinedOpName()}({", ".join(self.fmt_vn(i) for i in op.inputs[1:])})'

    def fmt_CBRANCH(self, op: PcodeOp) -> str:
        return f"if ({self.fmt_vn(op.inputs[1])}) goto {self.fmt_vn(op.inputs[0])}"

    def fmt_LOAD(self, op: PcodeOp) -> str:
        return f"*[{op.inputs[0].getSpaceFromConst().name}]{self.fmt_vn(op.inputs[1])}"

    def fmt_RETURN(self, op: PcodeOp) -> str:
        return f"return {self.fmt_vn(op.inputs[0])}"

    def fmt_STORE(self, op: PcodeOp) -> str:
        return f"*[{op.inputs[0].getSpaceFromConst().name}]{self.fmt_vn(op.inputs[1])} = {self.fmt_vn(op.inputs[2])}"

    def fmt(self, op: PcodeOp) -> str:
        return {
            OpCode.BRANCH: self.fmt_BRANCH,
            OpCode.BRANCHIND: self.fmt_BRANCHIND,
            OpCode.CALL: self.fmt_CALL,
            OpCode.CALLIND: self.fmt_CALLIND,
            OpCode.CALLOTHER: self.fmt_CALLOTHER,
            OpCode.CBRANCH: self.fmt_CBRANCH,
            OpCode.LOAD: self.fmt_LOAD,
            OpCode.RETURN: self.fmt_RETURN,
            OpCode.STORE: self.fmt_STORE,
        }.get(op.opcode, super().fmt)(op)


class PcodePrettyPrinter:
    """
    P-code pretty-printer.
    """

    DEFAULT_OP_FORMAT = OpFormat()

    OP_FORMATS = {
        OpCode.BOOL_AND: OpFormatBinary("&&"),
        OpCode.BOOL_NEGATE: OpFormatUnary("!"),
        OpCode.BOOL_OR: OpFormatBinary("||"),
        OpCode.BOOL_XOR: OpFormatBinary("^^"),
        OpCode.BRANCH: OpFormatSpecial(),
        OpCode.BRANCHIND: OpFormatSpecial(),
        OpCode.CALL: OpFormatSpecial(),
        OpCode.CALLIND: OpFormatSpecial(),
        OpCode.CALLOTHER: OpFormatSpecial(),
        OpCode.CBRANCH: OpFormatSpecial(),
        OpCode.COPY: OpFormatUnary(""),
        OpCode.CPOOLREF: OpFormatFunc("cpool"),
        OpCode.FLOAT_ABS: OpFormatFunc("abs"),
        OpCode.FLOAT_ADD: OpFormatBinary("f+"),
        OpCode.FLOAT_CEIL: OpFormatFunc("ceil"),
        OpCode.FLOAT_DIV: OpFormatBinary("f/"),
        OpCode.FLOAT_EQUAL: OpFormatBinary("f=="),
        OpCode.FLOAT_FLOAT2FLOAT: OpFormatFunc("float2float"),
        OpCode.FLOAT_FLOOR: OpFormatFunc("floor"),
        OpCode.FLOAT_INT2FLOAT: OpFormatFunc("int2float"),
        OpCode.FLOAT_LESS: OpFormatBinary("f<"),
        OpCode.FLOAT_LESSEQUAL: OpFormatBinary("f<="),
        OpCode.FLOAT_MULT: OpFormatBinary("f*"),
        OpCode.FLOAT_NAN: OpFormatFunc("nan"),
        OpCode.FLOAT_NEG: OpFormatUnary("f- "),
        OpCode.FLOAT_NOTEQUAL: OpFormatBinary("f!="),
        OpCode.FLOAT_ROUND: OpFormatFunc("round"),
        OpCode.FLOAT_SQRT: OpFormatFunc("sqrt"),
        OpCode.FLOAT_SUB: OpFormatBinary("f-"),
        OpCode.FLOAT_TRUNC: OpFormatFunc("trunc"),
        OpCode.INT_2COMP: OpFormatUnary("-"),
        OpCode.INT_ADD: OpFormatBinary("+"),
        OpCode.INT_AND: OpFormatBinary("&"),
        OpCode.INT_CARRY: OpFormatFunc("carry"),
        OpCode.INT_DIV: OpFormatBinary("/"),
        OpCode.INT_EQUAL: OpFormatBinary("=="),
        OpCode.INT_LEFT: OpFormatBinary("<<"),
        OpCode.INT_LESS: OpFormatBinary("<"),
        OpCode.INT_LESSEQUAL: OpFormatBinary("<="),
        OpCode.INT_MULT: OpFormatBinary("*"),
        OpCode.INT_NEGATE: OpFormatUnary("~"),
        OpCode.INT_NOTEQUAL: OpFormatBinary("!="),
        OpCode.INT_OR: OpFormatBinary("|"),
        OpCode.INT_REM: OpFormatBinary("%"),
        OpCode.INT_RIGHT: OpFormatBinary(">>"),
        OpCode.INT_SBORROW: OpFormatFunc("sborrow"),
        OpCode.INT_SCARRY: OpFormatFunc("scarry"),
        OpCode.INT_SDIV: OpFormatBinary("s/"),
        OpCode.INT_SEXT: OpFormatFunc("sext"),
        OpCode.INT_SLESS: OpFormatBinary("s<"),
        OpCode.INT_SLESSEQUAL: OpFormatBinary("s<="),
        OpCode.INT_SREM: OpFormatBinary("s%"),
        OpCode.INT_SRIGHT: OpFormatBinary("s>>"),
        OpCode.INT_SUB: OpFormatBinary("-"),
        OpCode.INT_XOR: OpFormatBinary("^"),
        OpCode.INT_ZEXT: OpFormatFunc("zext"),
        OpCode.LOAD: OpFormatSpecial(),
        OpCode.NEW: OpFormatFunc("newobject"),
        OpCode.POPCOUNT: OpFormatFunc("popcount"),
        OpCode.LZCOUNT: OpFormatFunc("lzcount"),
        OpCode.RETURN: OpFormatSpecial(),
        OpCode.STORE: OpFormatSpecial(),
    }

    @staticmethod
    def fmt_op(op: PcodeOp) -> str:
        fmt = PcodePrettyPrinter.OP_FORMATS.get(op.opcode, PcodePrettyPrinter.DEFAULT_OP_FORMAT)
        return (f"{fmt.fmt_vn(op.output)} = " if op.output else "") + fmt.fmt(op)

    @staticmethod
    def fmt_translation(tx: Translation) -> str:
        return "\n".join(PcodePrettyPrinter.fmt_op(op) for op in tx.ops)


def fmt_instruction(insn: Instruction) -> str:
    return f"{insn.addr.offset:#x}/{insn.length}: {insn.mnem} {insn.body}"


def fmt_disassembly(dx: Disassembly) -> str:
    return "\n".join(fmt_instruction(insn) for insn in dx.instructions)


# Monkey patch print handlers
Disassembly.__str__ = fmt_disassembly  # type: ignore[assignment,method-assign]
Instruction.__str__ = fmt_instruction  # type: ignore[assignment,method-assign]
PcodeOp.__str__ = PcodePrettyPrinter.fmt_op  # type: ignore[assignment,method-assign]
Translation.__str__ = PcodePrettyPrinter.fmt_translation  # type: ignore[assignment,method-assign]
Varnode.__str__ = OpFormat.fmt_vn  # type: ignore[assignment,method-assign]
