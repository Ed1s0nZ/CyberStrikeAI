import copy
import re

from pyvex.const import U1, get_type_size, ty_to_const_class, vex_int_class
from pyvex.enums import IRCallee
from pyvex.expr import ITE, Binop, CCall, Const, Get, Load, RdTmp, Unop
from pyvex.stmt import Dirty, Exit, IMark, NoOp, Put, Store, WrTmp


class JumpKind:
    Boring = "Ijk_Boring"
    Call = "Ijk_Call"
    Ret = "Ijk_Ret"
    Segfault = "Ijk_SigSEGV"
    Exit = "Ijk_Exit"
    Syscall = "Ijk_Sys_syscall"
    Sysenter = "Ijk_Sys_sysenter"
    Invalid = "Ijk_INVALID"
    NoDecode = "Ijk_NoDecode"


class TypeMeta(type):
    typemeta_re = re.compile(r"int_(?P<size>\d+)$")

    def __getattr__(self, name):
        match = self.typemeta_re.match(name)
        if match:
            width = int(match.group("size"))
            return vex_int_class(width).type
        else:
            return type.__getattr__(name)


class Type(metaclass=TypeMeta):
    __metaclass__ = TypeMeta

    ieee_float_16 = "Ity_F16"
    ieee_float_32 = "Ity_F32"
    ieee_float_64 = "Ity_F64"
    ieee_float_128 = "Ity_F128"
    decimal_float_32 = "Ity_D32"
    decimal_float_64 = "Ity_D64"
    decimal_float_128 = "Ity_D128"
    simd_vector_128 = "Ity_V128"
    simd_vector_256 = "Ity_V256"


def get_op_format_from_const_ty(ty):
    return ty_to_const_class(ty).op_format


def make_format_op_generator(fmt_string):
    """
    Return a function which generates an op format (just a string of the vex instruction)

    Functions by formatting the fmt_string with the types of the arguments
    """

    def gen(arg_types):
        converted_arg_types = list(map(get_op_format_from_const_ty, arg_types))
        op = fmt_string.format(arg_t=converted_arg_types)
        return op

    return gen


def mkbinop(fstring):
    return lambda self, expr_a, expr_b: self.op_binary(make_format_op_generator(fstring))(expr_a, expr_b)


def mkunop(fstring):
    return lambda self, expr_a: self.op_unary(make_format_op_generator(fstring))(expr_a)


def mkcmpop(fstring_fragment, signedness=""):
    def cmpop(self, expr_a, expr_b):
        ty = self.get_type(expr_a)
        fstring = f"Iop_Cmp{fstring_fragment}{{arg_t[0]}}{signedness}"
        retval = mkbinop(fstring)(self, expr_a, expr_b)
        return self.cast_to(retval, ty)

    return cmpop


class IRSBCustomizer:
    op_add = mkbinop("Iop_Add{arg_t[0]}")
    op_sub = mkbinop("Iop_Sub{arg_t[0]}")
    op_umul = mkbinop("Iop_Mul{arg_t[0]}")
    op_smul = mkbinop("Iop_MullS{arg_t[0]}")
    op_sdiv = mkbinop("Iop_DivS{arg_t[0]}")
    op_udiv = mkbinop("Iop_DivU{arg_t[0]}")

    # Custom operation that does not exist in libVEX
    op_mod = mkbinop("Iop_Mod{arg_t[0]}")

    op_or = mkbinop("Iop_Or{arg_t[0]}")
    op_and = mkbinop("Iop_And{arg_t[0]}")
    op_xor = mkbinop("Iop_Xor{arg_t[0]}")

    op_shr = mkbinop("Iop_Shr{arg_t[0]}")  # Shift Right (logical)
    op_shl = mkbinop("Iop_Shl{arg_t[0]}")  # Shift Left (logical)

    op_sar = mkbinop("Iop_Sar{arg_t[0]}")  # Shift Arithmetic Right operation

    op_not = mkunop("Iop_Not{arg_t[0]}")

    op_cmp_eq = mkcmpop("EQ")
    op_cmp_ne = mkcmpop("NE")
    op_cmp_slt = mkcmpop("LT", "S")
    op_cmp_sle = mkcmpop("LE", "S")
    op_cmp_ult = mkcmpop("LT", "U")
    op_cmp_ule = mkcmpop("LE", "U")
    op_cmp_sge = mkcmpop("GE", "S")
    op_cmp_uge = mkcmpop("GE", "U")
    op_cmp_sgt = mkcmpop("GT", "S")
    op_cmp_ugt = mkcmpop("GT", "U")

    def __init__(self, irsb):
        self.arch = irsb.arch
        self.irsb = irsb

    def get_type(self, rdt):
        return rdt.result_type(self.irsb.tyenv)

    # Statements (no return value)
    def _append_stmt(self, stmt):
        self.irsb.statements += [stmt]

    def imark(self, int_addr, int_length, int_delta=0):
        self._append_stmt(IMark(int_addr, int_length, int_delta))

    def get_reg(self, regname):  # TODO move this into the lifter
        return self.arch.registers[regname][0]

    def put(self, expr_val, tuple_reg):
        self._append_stmt(Put(copy.copy(expr_val), tuple_reg))

    def store(self, addr, expr):
        self._append_stmt(Store(copy.copy(addr), copy.copy(expr), self.arch.memory_endness))

    def noop(self):
        self._append_stmt(NoOp())

    def add_exit(self, guard, dst, jk, ip):
        """
        Add an exit out of the middle of an IRSB.
        (e.g., a conditional jump)
        :param guard: An expression, the exit is taken if true
        :param dst: the destination of the exit (a Const)
        :param jk: the JumpKind of this exit (probably Ijk_Boring)
        :param ip: The address of this exit's source
        """
        self.irsb.statements.append(Exit(guard, dst.con, jk, ip))

    # end statements

    def goto(self, addr):
        self.irsb.next = addr
        self.irsb.jumpkind = JumpKind.Boring

    def ret(self, addr):
        self.irsb.next = addr
        self.irsb.jumpkind = JumpKind.Ret

    def call(self, addr):
        self.irsb.next = addr
        self.irsb.jumpkind = JumpKind.Call

    def _add_tmp(self, t):
        return self.irsb.tyenv.add(t)

    def _rdtmp(self, tmp):
        return RdTmp.get_instance(tmp)

    def _settmp(self, expr):
        ty = self.get_type(expr)
        tmp = self._add_tmp(ty)
        self._append_stmt(WrTmp(tmp, expr))
        return self._rdtmp(tmp)

    def rdreg(self, reg, ty):
        return self._settmp(Get(reg, ty))

    def load(self, addr, ty):
        return self._settmp(Load(self.arch.memory_endness, ty, copy.copy(addr)))

    def op_ccall(self, retty, funcstr, args):
        return self._settmp(CCall(retty, IRCallee(len(args), funcstr, 0xFFFF), args))

    def dirty(self, retty, funcstr, args):
        if retty is None:
            tmp = 0xFFFFFFFF
        else:
            tmp = self._add_tmp(retty)
        self._append_stmt(Dirty(IRCallee(len(args), funcstr, 0xFFFF), Const(U1(1)), args, tmp, None, None, None, None))
        return self._rdtmp(tmp)

    def ite(self, condrdt, iftruerdt, iffalserdt):
        return self._settmp(ITE(copy.copy(condrdt), copy.copy(iffalserdt), copy.copy(iftruerdt)))

    def mkconst(self, val, ty):
        cls = ty_to_const_class(ty)
        return Const(cls(val))

    # Operations
    def op_generic(self, Operation, op_generator):
        def instance(*args):  # Note: The args here are all RdTmps
            for arg in args:
                assert isinstance(arg, RdTmp) or isinstance(arg, Const)
            arg_types = [self.get_type(arg) for arg in args]
            # two operations should never share the same argument instances, copy them here to ensure that
            args = [copy.copy(a) for a in args]
            op = Operation(op_generator(arg_types), args)
            msg = "operation needs to be well typed: " + str(op)
            assert op.typecheck(self.irsb.tyenv), msg + "\ntypes: " + str(self.irsb.tyenv)
            return self._settmp(op)

        return instance

    def op_binary(self, op_format_str):
        return self.op_generic(Binop, op_format_str)

    def op_unary(self, op_format_str):
        return self.op_generic(Unop, op_format_str)

    def cast_to(self, rdt, tydest, signed=False, high=False):
        goalwidth = get_type_size(tydest)
        rdtwidth = self.get_rdt_width(rdt)

        if rdtwidth > goalwidth:
            return self.op_narrow_int(rdt, tydest, high_half=high)
        elif rdtwidth < goalwidth:
            return self.op_widen_int(rdt, tydest, signed=signed)
        else:
            return rdt

    def op_to_one_bit(self, rdt):
        rdtty = self.get_type(rdt)
        if rdtty not in [Type.int_64, Type.int_32]:
            rdt = self.op_widen_int_unsigned(rdt, Type.int_32)
        onebit = self.op_narrow_int(rdt, Type.int_1)
        return onebit

    def op_narrow_int(self, rdt, tydest, high_half=False):
        op_name = "{op}{high}to{dest}".format(
            op="Iop_{arg_t[0]}", high="HI" if high_half else "", dest=get_op_format_from_const_ty(tydest)
        )
        return self.op_unary(make_format_op_generator(op_name))(rdt)

    def op_widen_int(self, rdt, tydest, signed=False):
        op_name = "{op}{sign}to{dest}".format(
            op="Iop_{arg_t[0]}", sign="S" if signed else "U", dest=get_op_format_from_const_ty(tydest)
        )
        return self.op_unary(make_format_op_generator(op_name))(rdt)

    def op_widen_int_signed(self, rdt, tydest):
        return self.op_widen_int(rdt, tydest, signed=True)

    def op_widen_int_unsigned(self, rdt, tydest):
        return self.op_widen_int(rdt, tydest, signed=False)

    def get_msb(self, tmp, ty):
        width = get_type_size(ty)
        return self.get_bit(tmp, width - 1)

    def get_bit(self, rdt, idx):
        shifted = self.op_shr(rdt, idx)
        bit = self.op_extract_lsb(shifted)
        return bit

    def op_extract_lsb(self, rdt):
        bitmask = self.mkconst(1, self.get_type(rdt))
        return self.op_and(bitmask, rdt)

    def set_bit(self, rdt, idx, bval):
        currbit = self.get_bit(rdt, idx)
        areequalextrabits = self.op_xor(bval, currbit)
        one = self.mkconst(1, self.get_type(areequalextrabits))
        areequal = self.op_and(areequalextrabits, one)
        shifted = self.op_shl(areequal, idx)
        return self.op_xor(rdt, shifted)

    def set_bits(self, rdt, idxsandvals):
        ty = self.get_type(rdt)
        if all([isinstance(idx, Const) for idx, _ in idxsandvals]):
            relevantbits = self.mkconst(sum([1 << idx.con.value for idx, _ in idxsandvals]), ty)
        else:
            relevantbits = self.mkconst(0, ty)
            for idx, _ in idxsandvals:
                shifted = self.op_shl(self.mkconst(1, ty), idx)
                relevantbits = self.op_or(relevantbits, shifted)
        setto = self.mkconst(0, ty)
        for idx, bval in idxsandvals:
            bvalbit = self.op_extract_lsb(bval)
            shifted = self.op_shl(bvalbit, idx)
            setto = self.op_or(setto, shifted)
        shouldflip = self.op_and(self.op_xor(setto, rdt), relevantbits)
        return self.op_xor(rdt, shouldflip)

    def get_rdt_width(self, rdt):
        return rdt.result_size(self.irsb.tyenv)
