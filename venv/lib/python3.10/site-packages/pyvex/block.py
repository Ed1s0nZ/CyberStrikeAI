import copy
import itertools
import logging
from typing import Optional

from . import expr, stmt
from .const import U1, get_type_size
from .const_val import ConstVal
from .data_ref import DataRef
from .enums import VEXObject
from .errors import SkipStatementsError
from .expr import Const, RdTmp
from .native import pvc
from .stmt import (
    CAS,
    LLSC,
    Dirty,
    Exit,
    IMark,
    IRExpr,
    IRStmt,
    LoadG,
    WrTmp,
    get_enum_from_int,
    get_int_from_enum,
)
from .types import Arch

log = logging.getLogger("pyvex.block")


class IRSB(VEXObject):
    """
    The IRSB is the primary interface to pyvex. Constructing one of these will make a call into LibVEX to perform a
    translation.

    IRSB stands for *Intermediate Representation Super-Block*. An IRSB in VEX is a single-entry, multiple-exit code
    block.

    :ivar arch:             The architecture this block is lifted under. Must duck-type as :class:`archinfo.arch.Arch`
    :ivar statements:       The statements in this block
    :vartype statements:    list of :class:`IRStmt`
    :ivar next:             The expression for the default exit target of this block
    :vartype next:          :class:`IRExpr`
    :ivar int offsIP:       The offset of the instruction pointer in the VEX guest state
    :ivar int stmts_used:   The number of statements in this IRSB
    :ivar str jumpkind:     The type of this block's default jump (call, boring, syscall, etc) as a VEX enum string
    :ivar bool direct_next: Whether this block ends with a direct (not indirect) jump or branch
    :ivar int size:         The size of this block in bytes
    :ivar int addr:         The address of this basic block, i.e. the address in the first IMark
    """

    __slots__ = [
        "addr",
        "arch",
        "statements",
        "next",
        "_tyenv",
        "jumpkind",
        "is_noop_block",
        "_direct_next",
        "_size",
        "_instructions",
        "_exit_statements",
        "default_exit_target",
        "_instruction_addresses",
        "data_refs",
        "const_vals",
    ]

    # The following constants shall match the defs in pyvex.h
    MAX_EXITS = 400
    MAX_DATA_REFS = 2000
    MAX_CONST_VALS = 1000

    def __init__(
        self,
        data,
        mem_addr,
        arch: Arch,
        max_inst=None,
        max_bytes=None,
        bytes_offset=0,
        traceflags=0,
        opt_level=1,
        num_inst=None,
        num_bytes=None,
        strict_block_end=False,
        skip_stmts=False,
        collect_data_refs=False,
        cross_insn_opt=True,
    ):
        """
        :param data:                The bytes to lift. Can be either a string of bytes or a cffi buffer object.
                                    You may also pass None to initialize an empty IRSB.
        :type data:                 str or bytes or cffi.FFI.CData or None
        :param int mem_addr:        The address to lift the data at.
        :param arch:                The architecture to lift the data as.
        :param max_inst:            The maximum number of instructions to lift. (See note below)
        :param max_bytes:           The maximum number of bytes to use.
        :param num_inst:            Replaces max_inst if max_inst is None. If set to None as well, no instruction limit
                                    is used.
        :param num_bytes:           Replaces max_bytes if max_bytes is None. If set to None as well, no  byte limit is
                                    used.
        :param bytes_offset:        The offset into `data` to start lifting at. Note that for ARM THUMB mode, both
                                    `mem_addr` and `bytes_offset` must be odd (typically `bytes_offset` is set to 1).
        :param traceflags:          The libVEX traceflags, controlling VEX debug prints.
        :param opt_level:           The level of optimization to apply to the IR, -1 through 2. -1 is the strictest
                                    unoptimized level, 0 is unoptimized but will perform some lookahead/lookbehind
                                    optimizations, 1 performs constant propogation, and 2 performs loop unrolling,
                                    which honestly doesn't make much sense in the context of pyvex. The default is 1.
        :param strict_block_end:    Should the LibVEX arm-thumb split block at some instructions, for example CB{N}Z.

        .. note:: Explicitly specifying the number of instructions to lift (`max_inst`) may not always work
                  exactly as expected. For example, on MIPS, it is meaningless to lift a branch or jump
                  instruction without its delay slot. VEX attempts to Do The Right Thing by possibly decoding
                  fewer instructions than requested. Specifically, this means that lifting a branch or jump
                  on MIPS as a single instruction (`max_inst=1`) will result in an empty IRSB, and subsequent
                  attempts to run this block will raise `SimIRSBError('Empty IRSB passed to SimIRSB.')`.

        .. note:: If no instruction and byte limit is used, pyvex will continue lifting the block until the block
                  ends properly or until it runs out of data to lift.
        """
        if max_inst is None:
            max_inst = num_inst
        if max_bytes is None:
            max_bytes = num_bytes
        VEXObject.__init__(self)
        self.addr = mem_addr
        self.arch: Arch = arch

        self.statements: list[IRStmt] = []
        self.next: IRExpr = Const(U1(0))
        self._tyenv: Optional["IRTypeEnv"] = None
        self.jumpkind: str = "UNSET"
        self._direct_next: bool | None = None
        self._size: int | None = None
        self._instructions: int | None = None
        self._exit_statements: tuple[tuple[int, int, IRStmt], ...] | None = None
        self.is_noop_block: bool = False
        self.default_exit_target = None
        self.data_refs = ()
        self.const_vals = ()
        self._instruction_addresses: tuple[int, ...] = ()

        if data is not None:
            # This is the slower path (because we need to call _from_py() to copy the content in the returned IRSB to
            # the current IRSB instance. You should always call `lift()` directly. This method is kept for compatibility
            # concerns.
            from pyvex.lifting import lift

            irsb = lift(
                data,
                mem_addr,
                arch,
                max_bytes=max_bytes,
                max_inst=max_inst,
                bytes_offset=bytes_offset,
                opt_level=opt_level,
                traceflags=traceflags,
                strict_block_end=strict_block_end,
                skip_stmts=skip_stmts,
                collect_data_refs=collect_data_refs,
                cross_insn_opt=cross_insn_opt,
            )
            self._from_py(irsb)

    @staticmethod
    def empty_block(arch, addr, statements=None, nxt=None, tyenv=None, jumpkind=None, direct_next=None, size=None):
        block = IRSB(None, addr, arch)
        block._set_attributes(statements, nxt, tyenv, jumpkind, direct_next, size=size)
        return block

    @property
    def tyenv(self) -> "IRTypeEnv":
        if self._tyenv is None:
            self._tyenv = IRTypeEnv(self.arch)
        return self._tyenv

    @tyenv.setter
    def tyenv(self, v):
        self._tyenv = v

    @property
    def has_statements(self) -> bool:
        return self.statements is not None and bool(self.statements)

    @property
    def exit_statements(self) -> tuple[tuple[int, int, IRStmt], ...]:
        if self._exit_statements is not None:
            return self._exit_statements

        # Delayed process
        if not self.has_statements:
            return ()

        exit_statements = []

        ins_addr = None
        for idx, stmt_ in enumerate(self.statements):
            if type(stmt_) is IMark:
                ins_addr = stmt_.addr + stmt_.delta
            elif type(stmt_) is Exit:
                assert ins_addr is not None
                exit_statements.append((ins_addr, idx, stmt_))

        self._exit_statements = tuple(exit_statements)
        return self._exit_statements

    def copy(self) -> "IRSB":
        return copy.deepcopy(self)

    def extend(self, extendwith) -> None:
        """
        Appends an irsb to the current irsb. The irsb that is appended is invalidated. The appended irsb's jumpkind and
        default exit are used.
        :param extendwith:     The IRSB to append to this IRSB
        :vartype extendwith:   :class:`IRSB`
        """
        if self.stmts_used == 0:
            self._from_py(extendwith)
            return

        conversion_dict = {}
        invalid_vals = (0xFFFFFFFF, -1)

        new_size = self.size + extendwith.size
        new_instructions = self.instructions + extendwith.instructions
        new_direct_next = extendwith.direct_next

        def convert_tmp(tmp):
            """
            Converts a tmp from the appended-block into one in the appended-to-block. Creates a new tmp if it does not
            already exist. Prevents collisions in tmp numbers between the two blocks.
            :param tmp:       The tmp number to convert
            """
            if tmp not in conversion_dict:
                tmp_type = extendwith.tyenv.lookup(tmp)
                conversion_dict[tmp] = self.tyenv.add(tmp_type)
            return conversion_dict[tmp]

        def convert_expr(expr_):
            """
            Converts a VEX expression to use tmps in the appended-block instead of the appended-to-block. Used to
            prevent collisions in tmp numbers between the two blocks.
            :param tmp:       The VEX expression to convert
            :vartype expr:    :class:`IRExpr`
            """
            if type(expr_) is RdTmp:
                return RdTmp.get_instance(convert_tmp(expr_.tmp))
            return expr_

        for stmt_ in extendwith.statements:
            stmttype = type(stmt_)
            if stmttype is WrTmp:
                stmt_.tmp = convert_tmp(stmt_.tmp)
            elif stmttype is LoadG:
                stmt_.dst = convert_tmp(stmt_.dst)
            elif stmttype is LLSC:
                stmt_.result = convert_tmp(stmt_.result)
            elif stmttype is Dirty:
                if stmt_.tmp not in invalid_vals:
                    stmt_.tmp = convert_tmp(stmt_.tmp)
                for e in stmt_.args:
                    convert_expr(e)
            elif stmttype is CAS:
                if stmt_.oldLo not in invalid_vals:
                    stmt_.oldLo = convert_tmp(stmt_.oldLo)
                if stmt_.oldHi not in invalid_vals:
                    stmt_.oldHi = convert_tmp(stmt_.oldHi)
            # Convert all expressions
            to_replace = {}
            for expr_ in stmt_.expressions:
                replacement = convert_expr(expr_)
                if replacement is not expr_:
                    to_replace[expr_] = replacement
            stmt_.replace_expression(to_replace)
            # Add the converted statement to self.statements
            self.statements.append(stmt_)
        extendwith.next = convert_expr(extendwith.next)
        self.next = extendwith.next
        self.jumpkind = extendwith.jumpkind
        self._size = new_size
        self._instructions = new_instructions
        self._direct_next = new_direct_next

        # TODO: Change exit_statements, data_references, etc.

    def invalidate_direct_next(self) -> None:
        self._direct_next = None

    def pp(self) -> None:
        """
        Pretty-print the IRSB to stdout.
        """
        print(self._pp_str())

    def __repr__(self):
        return f"IRSB <0x{self.size:x} bytes, {self.instructions} ins., {str(self.arch)}> at 0x{self.addr:x}"

    def __str__(self):
        return self._pp_str()

    def __eq__(self, other):
        return (
            isinstance(other, IRSB)
            and self.addr == other.addr
            and self.arch.name == other.arch.name
            and self.statements == other.statements
            and self.next == other.next
            and self.jumpkind == other.jumpkind
        )

    def __hash__(self):
        return hash((IRSB, self.addr, self.arch.name, tuple(self.statements), self.next, self.jumpkind))

    def typecheck(self) -> bool:
        try:
            # existence assertions
            assert self.next is not None, "Missing next expression"
            assert self.jumpkind is not None, "Missing jumpkind"

            # Type assertions
            assert isinstance(self.next, expr.IRExpr), "Next expression is not an expression"
            assert type(self.jumpkind is str), "Jumpkind is not a string"
            assert self.jumpkind.startswith("Ijk_"), "Jumpkind is not a jumpkind enum"
            assert self.tyenv.typecheck(), "Type environment contains invalid types"

            # statement assertions
            last_imark = None
            for i, st in enumerate(self.statements):
                assert isinstance(st, stmt.IRStmt), "Statement %d is not an IRStmt" % i
                try:
                    assert st.typecheck(self.tyenv), "Statement %d failed to typecheck" % i
                except Exception:  # pylint: disable=bare-except
                    assert False, "Statement %d errored in typechecking" % i

                if type(st) is stmt.NoOp:
                    continue
                elif type(st) is stmt.IMark:
                    if last_imark is not None:
                        # pylint: disable=unsubscriptable-object
                        assert last_imark[0] + last_imark[1] == st.addr, "IMarks sizes overlap or have gaps"
                    last_imark = (st.addr, st.len)
                else:
                    assert last_imark is not None, "Operation statement appears before IMark"

            assert last_imark is not None, "No IMarks present in block"
        except AssertionError as e:
            log.debug(e.args[0])
            return False
        return True

    #
    # alternate constructors
    #

    @staticmethod
    def from_c(c_irsb, mem_addr, arch) -> "IRSB":
        irsb = IRSB(None, mem_addr, arch)
        irsb._from_c(c_irsb)
        return irsb

    @staticmethod
    def from_py(tyenv, stmts, next_expr, jumpkind, mem_addr, arch) -> "IRSB":
        irsb = IRSB(None, mem_addr, arch)

        irsb.tyenv = tyenv
        irsb.statements = stmts
        irsb.next = next_expr
        irsb.jumpkind = jumpkind
        irsb._direct_next = irsb._is_defaultexit_direct_jump()

        return irsb

    #
    # simple properties useful for analysis
    #

    @property
    def stmts_used(self) -> int:
        if self.statements is None:
            return 0
        return len(self.statements)

    @property
    def offsIP(self) -> int:
        return self.arch.ip_offset

    @property
    def direct_next(self):
        if self._direct_next is None:
            self._direct_next = self._is_defaultexit_direct_jump()
        return self._direct_next

    @property
    def expressions(self):
        """
        Return an iterator of all expressions contained in the IRSB.
        """
        for s in self.statements:
            yield from s.expressions
        yield self.next

    @property
    def instructions(self):
        """
        The number of instructions in this block
        """
        if self._instructions is None:
            if self.statements is None:
                self._instructions = 0
            else:
                self._instructions = len([s for s in self.statements if type(s) is stmt.IMark])
        return self._instructions

    @property
    def instruction_addresses(self) -> tuple[int, ...]:
        """
        Addresses of instructions in this block.
        """
        if self._instruction_addresses is None:
            if self.statements is None:
                self._instruction_addresses = ()
            else:
                self._instruction_addresses = tuple(
                    (s.addr + s.delta) for s in self.statements if type(s) is stmt.IMark
                )
        return self._instruction_addresses

    @property
    def size(self):
        """
        The size of this block, in bytes
        """
        if self._size is None:
            self._size = sum(s.len for s in self.statements if type(s) is stmt.IMark)
        return self._size

    @property
    def operations(self):
        """
        A list of all operations done by the IRSB, as libVEX enum names
        """
        ops = []
        for e in self.expressions:
            if hasattr(e, "op"):
                ops.append(e.op)
        return ops

    @property
    def all_constants(self):
        """
        Returns all constants in the block (including incrementing of the program counter) as
        :class:`pyvex.const.IRConst`.
        """
        return sum((e.constants for e in self.expressions), [])

    @property
    def constants(self):
        """
        The constants (excluding updates of the program counter) in the IRSB as :class:`pyvex.const.IRConst`.
        """
        return sum((s.constants for s in self.statements if not (type(s) is stmt.Put and s.offset == self.offsIP)), [])

    @property
    def constant_jump_targets(self):
        """
        A set of the static jump targets of the basic block.
        """
        exits = set()

        if self.exit_statements:
            for _, _, stmt_ in self.exit_statements:
                exits.add(stmt_.dst.value)

        default_target = self.default_exit_target
        if default_target is not None:
            exits.add(default_target)

        return exits

    @property
    def constant_jump_targets_and_jumpkinds(self):
        """
        A dict of the static jump targets of the basic block to their jumpkind.
        """
        exits = {}

        if self.exit_statements:
            for _, _, stmt_ in self.exit_statements:
                exits[stmt_.dst.value] = stmt_.jumpkind

        default_target = self.default_exit_target
        if default_target is not None:
            exits[default_target] = self.jumpkind

        return exits

    #
    # private methods
    #

    def _pp_str(self) -> str:
        """
        Return the pretty-printed IRSB.
        """
        sa = []
        sa.append("IRSB {")
        if self.statements is not None:
            sa.append("   %s" % self.tyenv)
        sa.append("")
        if self.statements is not None:
            for i, s in enumerate(self.statements):
                if isinstance(s, stmt.Put):
                    stmt_str = s.pp_str(
                        reg_name=self.arch.translate_register_name(s.offset, s.data.result_size(self.tyenv) // 8)
                    )
                elif isinstance(s, stmt.WrTmp) and isinstance(s.data, expr.Get):
                    stmt_str = s.pp_str(
                        reg_name=self.arch.translate_register_name(s.data.offset, s.data.result_size(self.tyenv) // 8)
                    )
                elif isinstance(s, stmt.Exit):
                    stmt_str = s.pp_str(reg_name=self.arch.translate_register_name(s.offsIP, self.arch.bits // 8))
                else:
                    stmt_str = s.pp_str()
                sa.append("   %02d | %s" % (i, stmt_str))
        else:
            sa.append("   Statements are omitted.")
        sa.append(f"   NEXT: PUT({self.arch.translate_register_name(self.offsIP)}) = {self.next}; {self.jumpkind}")
        sa.append("}")
        return "\n".join(sa)

    def _is_defaultexit_direct_jump(self):
        """
        Checks if the default of this IRSB a direct jump or not.
        """
        if not (self.jumpkind == "Ijk_InvalICache" or self.jumpkind == "Ijk_Boring" or self.jumpkind == "Ijk_Call"):
            return False

        target = self.default_exit_target
        return target is not None

    #
    # internal "constructors" to fill this block out with data from various sources
    #

    def _from_c(self, lift_r, skip_stmts=False):
        c_irsb = lift_r.irsb
        if not skip_stmts:
            self.statements = [stmt.IRStmt._from_c(c_irsb.stmts[i]) for i in range(c_irsb.stmts_used)]
            self.tyenv = IRTypeEnv._from_c(self.arch, c_irsb.tyenv)
        else:
            self.statements = None
            self.tyenv = None

        self.next = expr.IRExpr._from_c(c_irsb.next)
        self.jumpkind = get_enum_from_int(c_irsb.jumpkind)
        self._size = lift_r.size
        self.is_noop_block = lift_r.is_noop_block == 1
        self._instructions = lift_r.insts
        self._instruction_addresses = tuple(itertools.islice(lift_r.inst_addrs, lift_r.insts))

        # Conditional exits
        exit_statements = []
        if skip_stmts:
            if lift_r.exit_count > self.MAX_EXITS:
                # There are more exits than the default size of the exits array. We will need all statements
                raise SkipStatementsError("exit_count exceeded MAX_EXITS (%d)" % self.MAX_EXITS)
            for i in range(lift_r.exit_count):
                ex = lift_r.exits[i]
                exit_stmt = stmt.IRStmt._from_c(ex.stmt)
                exit_statements.append((ex.ins_addr, ex.stmt_idx, exit_stmt))

            self._exit_statements = tuple(exit_statements)
        else:
            self._exit_statements = None  # It will be generated when self.exit_statements is called
        # The default exit
        if lift_r.is_default_exit_constant == 1:
            self.default_exit_target = lift_r.default_exit
        else:
            self.default_exit_target = None

        # Data references
        self.data_refs = None
        if lift_r.data_ref_count > 0:
            if lift_r.data_ref_count > self.MAX_DATA_REFS:
                raise SkipStatementsError(f"data_ref_count exceeded MAX_DATA_REFS ({self.MAX_DATA_REFS})")
            self.data_refs = [DataRef.from_c(lift_r.data_refs[i]) for i in range(lift_r.data_ref_count)]

        # Const values
        self.const_vals = None
        if lift_r.const_val_count > 0:
            if lift_r.const_val_count > self.MAX_CONST_VALS:
                raise SkipStatementsError(f"const_val_count exceeded MAX_CONST_VALS ({self.MAX_CONST_VALS})")
            self.const_vals = [ConstVal.from_c(lift_r.const_vals[i]) for i in range(lift_r.const_val_count)]

    def _set_attributes(
        self,
        statements=None,
        nxt=None,
        tyenv=None,
        jumpkind=None,
        direct_next=None,
        size=None,
        instructions=None,
        instruction_addresses=None,
        exit_statements=None,
        default_exit_target=None,
    ):
        self.statements = statements if statements is not None else []
        self.next = nxt
        if tyenv is not None:
            self.tyenv = tyenv
        self.jumpkind = jumpkind
        self._direct_next = direct_next
        self._size = size
        self._instructions = instructions
        self._instruction_addresses = instruction_addresses
        self._exit_statements = exit_statements
        self.default_exit_target = default_exit_target

    def _from_py(self, irsb):
        self._set_attributes(
            irsb.statements,
            irsb.next,
            irsb.tyenv,
            irsb.jumpkind,
            irsb.direct_next,
            irsb.size,
            instructions=irsb._instructions,
            instruction_addresses=irsb._instruction_addresses,
            exit_statements=irsb.exit_statements,
            default_exit_target=irsb.default_exit_target,
        )


class IRTypeEnv(VEXObject):
    """
    An IR type environment.

    :ivar types:        A list of the types of all the temporaries in this block as VEX enum strings.
                        `types[3]` is the type of t3.
    :vartype types:     list of str
    """

    __slots__ = ["types", "wordty"]

    def __init__(self, arch, types=None):
        VEXObject.__init__(self)
        self.types = [] if types is None else types
        self.wordty = "Ity_I%d" % arch.bits

    def __str__(self):
        return " ".join(("t%d:%s" % (i, t)) for i, t in enumerate(self.types))

    def lookup(self, tmp: int) -> str:
        """
        Return the type of temporary variable `tmp` as an enum string
        """
        if tmp < 0 or tmp > self.types_used:
            log.debug("Invalid temporary number %d", tmp)
            raise IndexError(tmp)
        return self.types[tmp]

    def sizeof(self, tmp):
        return get_type_size(self.lookup(tmp))

    def add(self, ty):
        """
        Add a new tmp of type `ty` to the environment. Returns the number of the new tmp.
        """
        self.types.append(ty)
        return self.types_used - 1

    @property
    def types_used(self):
        return len(self.types)

    @staticmethod
    def _from_c(arch, c_tyenv):
        return IRTypeEnv(arch, [get_enum_from_int(c_tyenv.types[t]) for t in range(c_tyenv.types_used)])

    @staticmethod
    def _to_c(tyenv):
        c_tyenv = pvc.emptyIRTypeEnv()
        for ty in tyenv.types:
            pvc.newIRTemp(c_tyenv, get_int_from_enum(ty))
        return c_tyenv

    def typecheck(self):
        for ty in self.types:
            try:
                get_type_size(ty)
            except ValueError:
                return False
        return True
