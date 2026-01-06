import logging
from typing import TYPE_CHECKING

import bitstring

from pyvex.const import vex_int_class
from pyvex.errors import LiftingException
from pyvex.lifting.lifter import Lifter

from .vex_helper import IRSBCustomizer, JumpKind

if TYPE_CHECKING:
    from .instr_helper import Instruction

log = logging.getLogger(__name__)


def is_empty(bitstrm):
    try:
        bitstrm.peek(1)
        return False
    except bitstring.ReadError:
        return True


class ParseError(Exception):
    pass


class GymratLifter(Lifter):
    """
    This is a base class for lifters that use Gymrat.
    For most architectures, all you need to do is subclass this, and set the property "instructions"
    to be a list of classes that define each instruction.
    By default, a lifter will decode instructions by attempting to instantiate every class until one works.
    This will use an IRSBCustomizer, which will, if it succeeds, add the appropriate VEX instructions to a pyvex IRSB.
    pyvex, when lifting a block of code for this architecture, will call the method "lift", which will produce the IRSB
    of the lifted code.
    """

    __slots__ = (
        "bitstrm",
        "errors",
        "thedata",
        "disassembly",
    )

    REQUIRE_DATA_PY = True
    instrs: list[type["Instruction"]]

    def __init__(self, arch, addr):
        super().__init__(arch, addr)
        self.bitstrm = None
        self.errors = None
        self.thedata = None
        self.disassembly = None

    def create_bitstrm(self):
        self.bitstrm = bitstring.ConstBitStream(bytes=self.thedata)

    def _decode_next_instruction(self, addr):
        # Try every instruction until one works
        for possible_instr in self.instrs:
            try:
                log.debug("Trying %s", possible_instr.name)
                return possible_instr(self.bitstrm, self.irsb.arch, addr)
            # a ParserError signals that this instruction did not match
            # we need to try other instructions, so we ignore this error
            except ParseError:
                pass  # l.exception(repr(possible_instr))
            # if we are out of input, ignore.
            # there may be other, shorter instructions that still match,
            # so we continue with the loop
            except (bitstring.ReadError, bitstring.InterpretError):
                pass

        # If no instruction matches, log an error
        errorstr = "Unknown instruction at bit position %d" % self.bitstrm.bitpos
        log.debug(errorstr)
        log.debug("Address: %#08x" % addr)

    def decode(self):
        try:
            self.create_bitstrm()
            count = 0
            disas = []
            addr = self.irsb.addr
            log.debug("Starting block at address: " + hex(addr))
            bytepos = self.bitstrm.bytepos

            while not is_empty(self.bitstrm):
                instr = self._decode_next_instruction(addr)
                if not instr:
                    break
                disas.append(instr)
                log.debug("Matched " + instr.name)
                addr += self.bitstrm.bytepos - bytepos
                bytepos = self.bitstrm.bytepos
                count += 1
            return disas
        except Exception as e:
            self.errors = str(e)
            log.exception(f"Error decoding block at offset {bytepos:#x} (address {addr:#x}):")
            raise

    def _lift(self):
        self.thedata = (
            self.data[: self.max_bytes]
            if isinstance(self.data, (bytes, bytearray, memoryview))
            else self.data[: self.max_bytes].encode()
        )
        log.debug(repr(self.thedata))
        instructions = self.decode()

        if self.disasm:
            self.disassembly = [instr.disassemble() for instr in instructions]
        self.irsb.jumpkind = JumpKind.Invalid
        irsb_c = IRSBCustomizer(self.irsb)
        log.debug("Decoding complete.")
        for i, instr in enumerate(instructions[: self.max_inst]):
            log.debug("Lifting instruction %s", instr.name)
            instr(irsb_c, instructions[:i], instructions[i + 1 :])
            if irsb_c.irsb.jumpkind != JumpKind.Invalid:
                break
            if (i + 1) == self.max_inst:  # if we are on our last iteration
                instr.jump(None, irsb_c.irsb.addr + irsb_c.irsb.size)
                break
        else:
            if len(irsb_c.irsb.statements) == 0:
                raise LiftingException("Could not decode any instructions")
            irsb_c.irsb.jumpkind = JumpKind.NoDecode
            dst = irsb_c.irsb.addr + irsb_c.irsb.size
            dst_ty = vex_int_class(irsb_c.irsb.arch.bits).type
            irsb_c.irsb.next = irsb_c.mkconst(dst, dst_ty)
        log.debug(str(self.irsb))
        if self.dump_irsb:
            self.irsb.pp()
        return self.irsb

    def pp_disas(self):
        disasstr = ""
        insts = self.disassemble()
        for addr, name, args in insts:
            args_str = ",".join(str(a) for a in args)
            disasstr += f"{addr:#08x}:\t{name} {args_str}\n"
        print(disasstr)

    def error(self):
        return self.errors

    def disassemble(self):
        if self.disassembly is None:
            self.lift(self.data, disasm=True)
        return self.disassembly
