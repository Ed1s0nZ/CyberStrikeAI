import abc
import string

import bitstring

from pyvex.expr import IRExpr, RdTmp

from .lifter_helper import ParseError
from .syntax_wrapper import VexValue
from .vex_helper import IRSBCustomizer, JumpKind, vex_int_class


class Instruction(metaclass=abc.ABCMeta):
    """
    Base class for an Instruction.

    You should make a subclass of this for each instruction you want to lift. These classes will contain the "semantics"
    of the instruction, that is, what it _does_, in terms of the VEX IR.

    You may want to subclass this for your architecture, and add arch-specific handling for parsing, argument
    resolution, etc., and have instructions subclass that instead.

    The core parsing functionality is done via ``bin_format``. Each instruction should be a subclass of ``Instruction``
    and will be parsed by comparing bits in the provided bitstream to symbols in the ``bin_format`` member of the class.
    "Bin formats" are strings of symbols, like those you'd find in an ISA document, such as "0010rrrrddddffmm"
    0 or 1 specify hard-coded bits that must match for an instruction to match.
    Any letters specify arguments, grouped by letter, which will be parsed and provided as bitstrings in the ``data``
    member of the class as a dictionary.
    So, in our example, the bits ``0010110101101001``, applied to format string ``0010rrrrddddffmm``
    will result in the following in ``self.data``:

        {'r': '1101',
         'd': '0110',
         'f': '10',
         'm': '01'}

    Implement compute_result to provide the "meat" of what your instruction does.
    You can also implement it in your arch-specific subclass of ``Instruction``, to handle things common to all
    instructions, and provide instruction implementations elsewhere.

    We provide the ``VexValue`` syntax wrapper to make expressing instruction semantics easy.
    You first convert the bitstring arguments into ``VexValue``s using the provided convenience methods
    (``self.get/put/load/store/etc.``)
    This loads the register from the actual registers into a temporary value we can work with.
    You can then write it back to a register when you're done.
    For example, if you have the register in ``r``, as above, you can make a ``VexValue`` like this:

        r = int(self.data['r'], 2) # we get bits corresponding to `r` bits and convert it to an int
        r_vv = self.get(r, Type.int_32)

    If you then had an instruction to increment ``r``, you could simply:

        return r_vv += 1

    You could then write it back to the register like this:

        self.put(r_vv, r)

    Note that most architectures have special flags that get set differently for each instruction, make sure to
    implement those as well (override ``set_flags()`` )

    Override ``parse()`` to extend parsing.
    For example, in MSP430, this allows us to grab extra words from the bitstream
    when extra immediate words are present.

    All architectures are different enough that there's no magic recipe for how to write a lifter.
    See the examples provided by gymrat for ideas of how to use this to build your own lifters quickly and easily.
    """

    data: dict[str, str]
    irsb_c: IRSBCustomizer

    def __init__(self, bitstrm, arch, addr):
        """
        Create an instance of the instruction

        :param irsb_c: The IRSBCustomizer to put VEX instructions into
        :param bitstrm: The bitstream to decode instructions from
        :param addr: The address of the instruction to be lifted, used only for jumps and branches
        """
        self.addr = addr
        self.arch = arch
        self.bitwidth = len(self.bin_format)
        self.data = self.parse(bitstrm)

    @property
    @abc.abstractmethod
    def bin_format(self) -> str:
        """
        Read the documentation of the class to understand what a bin format string is

        :return: str bin format string
        """

    @property
    @abc.abstractmethod
    def name(self) -> str:
        """
        Name of the instruction

        Can be useful to name the instruction when there's an error related to it
        """

    def __call__(self, irsb_c, past_instructions, future_instructions):
        self.lift(irsb_c, past_instructions, future_instructions)

    def mark_instruction_start(self):
        self.irsb_c.imark(self.addr, self.bytewidth, 0)

    def fetch_operands(self):  # pylint: disable=no-self-use
        """
        Get the operands out of memory or registers
        Return a tuple of operands for the instruction
        """
        return ()

    def lift(self, irsb_c: IRSBCustomizer, past_instructions, future_instructions):  # pylint: disable=unused-argument
        """
        This is the main body of the "lifting" for the instruction.
        This can/should be overridden to provide the general flow of how instructions in your arch work.
        For example, in MSP430, this is:

        - Figure out what your operands are by parsing the addressing, and load them into temporary registers
        - Do the actual operation, and commit the result, if needed.
        - Compute the flags
        """
        self.irsb_c = irsb_c
        # Always call this first!
        self.mark_instruction_start()
        # Then do the actual stuff.
        inputs = self.fetch_operands()
        retval = self.compute_result(*inputs)  # pylint: disable=assignment-from-none
        if retval is not None:
            self.commit_result(retval)
        vals = list(inputs) + [retval]
        self.compute_flags(*vals)

    def commit_result(self, res):
        """
        This where the result of the operation is written to a destination.
        This happens only if compute_result does not return None, and happens before compute_flags is called.
        Override this to specify how to write out the result.
        The results of fetch_operands can be used to resolve various addressing modes for the write outward.
        A common pattern is to return a function from fetch_operands which will be called here to perform the write.

        :param args: A tuple of the results of fetch_operands and compute_result
        """

    def compute_result(self, *args):  # pylint: disable=unused-argument,no-self-use
        """
        This is where the actual operation performed by your instruction, excluding the calculation of flags, should be
        performed.  Return the VexValue of the "result" of the instruction, which may
        be used to calculate the flags later.
        For example, for a simple add, with arguments src and dst, you can simply write:

            return src + dst:

        :param args:
        :return: A VexValue containing the "result" of the operation.
        """
        return None

    def compute_flags(self, *args):
        """
        Most CPU architectures have "flags" that should be computed for many instructions.
        Override this to specify how that happens.  One common pattern is to define this method to call specifi methods
        to update each flag, which can then be overriden in the actual classes for each instruction.
        """

    def match_instruction(self, data, bitstrm):  # pylint: disable=unused-argument,no-self-use
        """
        Override this to extend the parsing functionality.
        This is great for if your arch has instruction "formats" that have an opcode that has to match.

        :param data:
        :param bitstrm:
        :return: data
        """
        return data

    def parse(self, bitstrm):
        if self.arch.instruction_endness == "Iend_LE":
            # This arch stores its instructions in memory endian-flipped compared to the ISA.
            # To enable natural lifter-writing, we let the user write them like in the manual, and correct for
            # endness here.
            instr_bits = self._load_le_instr(bitstrm, self.bitwidth)
        else:
            instr_bits = bitstrm.peek("bin:%d" % self.bitwidth)

        data = {c: "" for c in self.bin_format if c in string.ascii_letters}
        for c, b in zip(self.bin_format, instr_bits):
            if c in "01":
                if b != c:
                    raise ParseError("Mismatch between format bit %c and instruction bit %c" % (c, b))
            elif c in string.ascii_letters:
                data[c] += b
            else:
                raise ValueError("Invalid bin_format character %c" % c)

        # Hook here for extra matching functionality
        if hasattr(self, "match_instruction"):
            # Should raise if it's not right
            self.match_instruction(data, bitstrm)

        # Use up the bits once we're sure it's right
        self.rawbits = bitstrm.read("hex:%d" % self.bitwidth)

        # Hook here for extra parsing functionality (e.g., trailers)
        if hasattr(self, "_extra_parsing"):
            data = self._extra_parsing(data, bitstrm)  # pylint: disable=no-member

        return data

    @property
    def bytewidth(self):
        if self.bitwidth % self.arch.byte_width != 0:
            raise ValueError("Instruction is not a multiple of bytes wide!")
        return self.bitwidth // self.arch.byte_width

    def disassemble(self):
        """
        Return the disassembly of this instruction, as a string.
        Override this in subclasses.

        :return: The address (self.addr), the instruction's name, and a list of its operands, as strings
        """
        return self.addr, "UNK", [self.rawbits]

    # These methods should be called in subclasses to do register and memory operations

    def load(self, addr, ty):
        """
        Load a value from memory into a VEX temporary register.

        :param addr: The VexValue containing the addr to load from.
        :param ty: The Type of the resulting data
        :return: a VexValue
        """
        rdt = self.irsb_c.load(addr.rdt, ty)
        return VexValue(self.irsb_c, rdt)

    def constant(self, val, ty):
        """
        Creates a constant as a VexValue

        :param val: The value, as an integer
        :param ty: The type of the resulting VexValue
        :return: a VexValue
        """
        if isinstance(val, VexValue) and not isinstance(val, IRExpr):
            raise Exception("Constant cannot be made from VexValue or IRExpr")
        rdt = self.irsb_c.mkconst(val, ty)
        return VexValue(self.irsb_c, rdt)

    @staticmethod
    def _lookup_register(arch, reg):
        # TODO: This is a hack to make it work with archinfo where we use
        # register indicies instead of names
        if isinstance(reg, int):
            if hasattr(arch, "register_index"):
                reg = arch.register_index[reg]
            else:
                reg = arch.register_list[reg].name
        return arch.get_register_offset(reg)

    def get(self, reg, ty):
        """
        Load a value from a machine register into a VEX temporary register.
        All values must be loaded out of registers before they can be used with operations, etc
        and stored back into them when the instruction is over.  See Put().

        :param reg: Register number as an integer, or register string name
        :param ty: The Type to use.
        :return: A VexValue of the gotten value.
        """
        offset = self._lookup_register(self.irsb_c.irsb.arch, reg)
        if offset == self.irsb_c.irsb.arch.ip_offset:
            return self.constant(self.addr, ty)
        rdt = self.irsb_c.rdreg(offset, ty)
        return VexValue(self.irsb_c, rdt)

    def put(self, val, reg):
        """
        Puts a value from a VEX temporary register into a machine register.
        This is how the results of operations done to registers get committed to the machine's state.

        :param val: The VexValue to store (Want to store a constant? See Constant() first)
        :param reg: The integer register number to store into, or register name
        :return: None
        """
        offset = self._lookup_register(self.irsb_c.irsb.arch, reg)
        self.irsb_c.put(val.rdt, offset)

    def put_conditional(self, cond, valiftrue, valiffalse, reg):
        """
        Like put, except it checks a condition
        to decide what to put in the destination register.

        :param cond: The VexValue representing the logical expression for the condition
            (if your expression only has constants, don't use this method!)
        :param valiftrue: the VexValue to put in reg if cond evals as true
        :param validfalse: the VexValue to put in reg if cond evals as false
        :param reg: The integer register number to store into, or register name
        :return: None
        """

        val = self.irsb_c.ite(cond.rdt, valiftrue.rdt, valiffalse.rdt)
        offset = self._lookup_register(self.irsb_c.irsb.arch, reg)
        self.irsb_c.put(val, offset)

    def store(self, val, addr):
        """
        Store a VexValue in memory at the specified loaction.

        :param val: The VexValue of the value to store
        :param addr: The VexValue of the address to store into
        :return: None
        """
        self.irsb_c.store(addr.rdt, val.rdt)

    def jump(self, condition, to_addr, jumpkind=JumpKind.Boring, ip_offset=None):
        """
        Jump to a specified destination, under the specified condition.
        Used for branches, jumps, calls, returns, etc.

        :param condition: The VexValue representing the expression for the guard, or None for an unconditional jump
        :param to_addr: The address to jump to.
        :param jumpkind: The JumpKind to use.  See the VEX docs for what these are; you only need them for things
            aren't normal jumps (e.g., calls, interrupts, program exits, etc etc)
        :return: None
        """
        to_addr_ty = None
        if isinstance(to_addr, VexValue):
            # Unpack a VV
            to_addr_rdt = to_addr.rdt
            to_addr_ty = to_addr.ty
        elif isinstance(to_addr, int):
            # Direct jump to an int, make an RdT and Ty
            to_addr_ty = vex_int_class(self.irsb_c.irsb.arch.bits).type
            to_addr = self.constant(to_addr, to_addr_ty)  # TODO archinfo may be changing
            to_addr_rdt = to_addr.rdt
        elif isinstance(to_addr, RdTmp):
            # An RdT; just get the Ty of the arch's pointer type
            to_addr_ty = vex_int_class(self.irsb_c.irsb.arch.bits).type
            to_addr_rdt = to_addr
        else:
            raise TypeError("Jump destination has unknown type: " + repr(type(to_addr)))
        if not condition:
            # This is the default exit.
            self.irsb_c.irsb.jumpkind = jumpkind
            self.irsb_c.irsb.next = to_addr_rdt
        else:
            # add another exit
            # EDG says: We should make sure folks set ArchXYZ.ip_offset like they're supposed to
            if ip_offset is None:
                ip_offset = self.arch.ip_offset
            assert ip_offset is not None

            negated_condition_rdt = self.ite(condition, self.constant(0, condition.ty), self.constant(1, condition.ty))
            direct_exit_target = self.constant(self.addr + (self.bitwidth // 8), to_addr_ty)
            self.irsb_c.add_exit(negated_condition_rdt, direct_exit_target.rdt, jumpkind, ip_offset)
            self.irsb_c.irsb.jumpkind = jumpkind
            self.irsb_c.irsb.next = to_addr_rdt

    def ite(self, cond, t, f):
        return self.irsb_c.ite(cond.rdt, t.rdt, f.rdt)

    def ccall(self, ret_type, func_name, args):
        """
        Creates a CCall operation.
        A CCall is a procedure that calculates a value at *runtime*, not at lift-time.
        You can use these for flags, unresolvable jump targets, etc.
        We caution you to avoid using them when at all possible though.

        :param ret_type: The return type of the CCall
        :param func_obj: The name of the helper function to call. If you're using angr, this should be added (or
                         monkeypatched) into ``angr.engines.vex.claripy.ccall``.
        :param args: List of arguments to the function
        :return: A VexValue of the result.
        """

        # Check the args to make sure they're the right type
        list_args = list(args)
        new_args = []
        for arg in list_args:
            if isinstance(arg, VexValue):
                arg = arg.rdt
            new_args.append(arg)
        args = tuple(new_args)

        cc = self.irsb_c.op_ccall(ret_type, func_name, args)
        return VexValue(self.irsb_c, cc)

    def dirty(self, ret_type, func_name, args) -> VexValue:
        """
        Creates a dirty call operation.

        These are like ccalls (clean calls) but their implementations are theoretically allowed to read or write to or
        from any part of the state, making them a nightmare for static analysis to reason about. Avoid their use at all
        costs.

        :param ret_type:   The return type of the dirty call, or None if the dirty call doesn't return anything.
        :param func_name:  The name of the helper function to call. If you're using angr, this should be added (or
                           monkeypatched) into ``angr.engines.vex.heavy.dirty``.
        :param args: List of arguments to the function
        :return: A VexValue of the result.
        """

        # Check the args to make sure they're the right type
        list_args = list(args)
        new_args = []
        for arg in list_args:
            if isinstance(arg, VexValue):
                arg = arg.rdt
            new_args.append(arg)
        args = tuple(new_args)

        rdt = self.irsb_c.dirty(ret_type, func_name, args)
        return VexValue(self.irsb_c, rdt)

    def _load_le_instr(self, bitstream: bitstring.ConstBitStream, numbits: int) -> str:
        return bitstring.Bits(uint=bitstream.peek("uintle:%d" % numbits), length=numbits).bin
