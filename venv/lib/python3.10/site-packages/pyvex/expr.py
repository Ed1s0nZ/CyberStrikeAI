from __future__ import annotations

import logging
import re
from typing import TYPE_CHECKING

from .const import U8, U16, U32, U64, IRConst, get_type_size
from .enums import IRCallee, IRRegArray, VEXObject, get_enum_from_int, get_int_from_enum
from .errors import PyVEXError
from .native import ffi, pvc

if TYPE_CHECKING:
    from .block import IRTypeEnv

log = logging.getLogger("pyvex.expr")


class IRExpr(VEXObject):
    """
    IR expressions in VEX represent operations without side effects.
    """

    __slots__ = []

    tag: str | None = None
    tag_int = 0  # set automatically at bottom of file

    def pp(self):
        print(str(self))

    def __str__(self):
        return self._pp_str()

    def _pp_str(self) -> str:
        raise NotImplementedError

    @property
    def child_expressions(self) -> list[IRExpr]:
        """
        A list of all of the expressions that this expression ends up evaluating.
        """
        expressions = []
        for k in self.__slots__:
            v = getattr(self, k)
            if isinstance(v, IRExpr):
                expressions.append(v)
                expressions.extend(v.child_expressions)
        return expressions

    @property
    def constants(self):
        """
        A list of all of the constants that this expression ends up using.
        """
        constants = []
        for k in self.__slots__:
            v = getattr(self, k)
            if isinstance(v, IRExpr):
                constants.extend(v.constants)
            elif isinstance(v, IRConst):
                constants.append(v)
        return constants

    def result_size(self, tyenv: IRTypeEnv):
        return get_type_size(self.result_type(tyenv))

    def result_type(self, tyenv: IRTypeEnv):
        raise NotImplementedError()

    def replace_expression(self, replacements):
        """
        Replace child expressions in-place.

        :param Dict[IRExpr, IRExpr] replacements:  A mapping from expression-to-find to expression-to-replace-with
        :return:                    None
        """

        for k in self.__slots__:
            v = getattr(self, k)
            if isinstance(v, IRExpr) and v in replacements:
                setattr(self, k, replacements.get(v))
            elif isinstance(v, list):
                # Replace the instance in the list
                for i, expr_ in enumerate(v):
                    if isinstance(expr_, IRExpr) and expr_ in replacements:
                        v[i] = replacements.get(expr_)
            elif type(v) is tuple:
                # Rebuild the tuple
                _lst = []
                replaced = False
                for i, expr_ in enumerate(v):
                    if isinstance(expr_, IRExpr) and expr_ in replacements:
                        _lst.append(replacements.get(expr_))
                        replaced = True
                    else:
                        _lst.append(expr_)
                if replaced:
                    setattr(self, k, tuple(_lst))
            elif isinstance(v, IRExpr):
                v.replace_expression(replacements)

    @staticmethod
    def _from_c(c_expr) -> IRExpr | None:
        if c_expr == ffi.NULL or c_expr[0] == ffi.NULL:
            return None

        try:
            return enum_to_expr_class(c_expr.tag)._from_c(c_expr)
        except KeyError:
            raise PyVEXError("Unknown/unsupported IRExprTag %s\n" % get_enum_from_int(c_expr.tag))

    _translate = _from_c

    @staticmethod
    def _to_c(expr):
        try:
            return tag_to_expr_class(expr.tag)._to_c(expr)
        except KeyError:
            raise PyVEXError("Unknown/unsupported IRExprTag %s\n" % expr.tag)

    def typecheck(self, tyenv):
        return self.result_type(tyenv)


class Binder(IRExpr):
    """
    Used only in pattern matching within Vex. Should not be seen outside of Vex.
    """

    __slots__ = ["binder"]

    tag = "Iex_Binder"

    def __init__(self, binder):
        self.binder = binder

    def _pp_str(self):
        return "Binder"

    @staticmethod
    def _from_c(c_expr):
        return Binder(c_expr.iex.Binder.binder)

    @staticmethod
    def _to_c(expr):
        return pvc.IRExpr_Binder(expr.binder)

    def result_type(self, tyenv):
        return "Ity_INVALID"


class VECRET(IRExpr):
    tag = "Iex_VECRET"

    __slots__ = []

    def _pp_str(self):
        return "VECRET"

    @staticmethod
    def _from_c(c_expr):
        return VECRET()

    @staticmethod
    def _to_c(expr):
        return pvc.IRExpr_VECRET()

    def result_type(self, tyenv):
        return "Ity_INVALID"


class GSPTR(IRExpr):
    __slots__ = []

    tag = "Iex_GSPTR"

    def _pp_str(self):
        return "GSPTR"

    @staticmethod
    def _from_c(c_expr):
        return GSPTR()

    @staticmethod
    def _to_c(expr):
        return pvc.IRExpr_GSPTR()

    def result_type(self, tyenv):
        return "Ity_INVALID"


class GetI(IRExpr):
    """
    Read a guest register at a non-fixed offset in the guest state.
    """

    __slots__ = ["descr", "ix", "bias"]

    tag = "Iex_GetI"

    def __init__(self, descr, ix, bias):
        self.descr = descr
        self.ix = ix
        self.bias = bias

    @property
    def description(self):
        return self.descr

    @property
    def index(self):
        return self.ix

    def _pp_str(self):
        return f"GetI({self.descr})[{self.ix},{self.bias}]"

    @staticmethod
    def _from_c(c_expr):
        descr = IRRegArray._from_c(c_expr.Iex.GetI.descr)
        ix = IRExpr._from_c(c_expr.Iex.GetI.ix)
        bias = c_expr.Iex.GetI.bias
        return GetI(descr, ix, bias)

    @staticmethod
    def _to_c(expr):
        return pvc.IRExpr_GetI(IRRegArray._to_c(expr.descr), IRExpr._to_c(expr.ix), expr.bias)

    def result_type(self, tyenv):
        return self.descr.elemTy


class RdTmp(IRExpr):
    """
    Read the value held by a temporary.
    """

    __slots__ = ["_tmp"]

    tag = "Iex_RdTmp"

    def __init__(self, tmp):
        self._tmp = tmp

    def _pp_str(self):
        return "t%d" % self.tmp

    @property
    def tmp(self):
        return self._tmp

    @staticmethod
    def _from_c(c_expr):
        tmp = c_expr.Iex.RdTmp.tmp
        return RdTmp.get_instance(tmp)

    @staticmethod
    def _to_c(expr):
        return pvc.IRExpr_RdTmp(expr.tmp)

    @staticmethod
    def get_instance(tmp):
        if tmp < 1024:
            # for small tmp reads, they are cached and are only created once globally
            return _RDTMP_POOL[tmp]
        return RdTmp(tmp)

    def replace_expression(self, replacements):
        # RdTmp is one of the terminal IRExprs, which cannot be replaced.
        pass

    def result_type(self, tyenv):
        return tyenv.lookup(self.tmp)

    def __hash__(self):
        return 133700 + self._tmp


_RDTMP_POOL = list(RdTmp(i) for i in range(0, 1024))


class Get(IRExpr):
    """
    Read a guest register, at a fixed offset in the guest state.
    """

    __slots__ = ["offset", "ty_int"]

    tag = "Iex_Get"

    def __init__(self, offset: int, ty: str, ty_int: int | None = None):
        self.offset = offset
        if ty_int is None:
            self.ty_int = get_int_from_enum(ty)
        else:
            self.ty_int = ty_int

    @property
    def ty(self):
        return get_enum_from_int(self.ty_int)

    @property
    def type(self):
        return get_enum_from_int(self.ty_int)

    def _pp_str(self):
        return f"GET:{self.ty[4:]}(offset={self.offset})"

    def pp_str_with_name(self, reg_name: str):
        """pp_str_with_name is used to print the expression with the name of the
        register instead of the offset"""
        return f"GET:{self.ty[4:]}({reg_name})"

    @staticmethod
    def _from_c(c_expr):
        return Get(c_expr.Iex.Get.offset, get_enum_from_int(c_expr.Iex.Get.ty))

    @staticmethod
    def _to_c(expr):
        return pvc.IRExpr_Get(expr.offset, expr.ty_int)

    def result_type(self, tyenv):
        return self.ty

    def __hash__(self):
        return (self.offset << 8) | self.ty_int


class Qop(IRExpr):
    """
    A quaternary operation (4 arguments).
    """

    __slots__ = ["op", "args"]

    tag = "Iex_Qop"

    def __init__(self, op, args):
        self.op = op
        self.args = args

    def _pp_str(self):
        return "{}({})".format(self.op[4:], ",".join(str(a) for a in self.args))

    @property
    def child_expressions(self):
        expressions = sum((a.child_expressions for a in self.args), [])
        expressions.extend(self.args)
        return expressions

    @staticmethod
    def _from_c(c_expr):
        return Qop(
            get_enum_from_int(c_expr.Iex.Qop.details.op),
            [
                IRExpr._from_c(arg)
                for arg in [
                    c_expr.Iex.Qop.details.arg1,
                    c_expr.Iex.Qop.details.arg2,
                    c_expr.Iex.Qop.details.arg3,
                    c_expr.Iex.Qop.details.arg4,
                ]
            ],
        )

    @staticmethod
    def _to_c(expr):
        return pvc.IRExpr_Qop(get_int_from_enum(expr.op), *[IRExpr._to_c(arg) for arg in expr.args])

    def result_type(self, tyenv):
        return get_op_retty(self.op)

    def typecheck(self, tyenv):  # TODO change all this to use PyvexTypeErrorException
        resty, (arg1ty, arg2ty, arg3ty, arg4ty) = op_arg_types(self.op)
        arg1ty_real = self.args[0].typecheck(tyenv)
        arg2ty_real = self.args[1].typecheck(tyenv)
        arg3ty_real = self.args[2].typecheck(tyenv)
        arg4ty_real = self.args[3].typecheck(tyenv)
        if arg1ty_real is None or arg2ty_real is None or arg3ty_real is None or arg4ty_real is None:
            return None

        if arg1ty_real != arg1ty:
            log.debug("First arg of %s must be %s", self.op, arg1ty)
            return None
        if arg2ty_real != arg2ty:
            log.debug("Second arg of %s must be %s", self.op, arg2ty)
            return None
        if arg3ty_real != arg3ty:
            log.debug("Third arg of %s must be %s", self.op, arg3ty)
            return None
        if arg4ty_real != arg4ty:
            log.debug("Fourth arg of %s must be %s", self.op, arg4ty)
            return None

        return resty


class Triop(IRExpr):
    """
    A ternary operation (3 arguments)
    """

    __slots__ = ["op", "args"]

    tag = "Iex_Triop"

    def __init__(self, op, args):
        self.op = op
        self.args = args

    def _pp_str(self):
        return "{}({})".format(self.op[4:], ",".join(str(a) for a in self.args))

    @property
    def child_expressions(self):
        expressions = sum((a.child_expressions for a in self.args), [])
        expressions.extend(self.args)
        return expressions

    @staticmethod
    def _from_c(c_expr):
        return Triop(
            get_enum_from_int(c_expr.Iex.Triop.details.op),
            [
                IRExpr._from_c(arg)
                for arg in [c_expr.Iex.Triop.details.arg1, c_expr.Iex.Triop.details.arg2, c_expr.Iex.Triop.details.arg3]
            ],
        )

    @staticmethod
    def _to_c(expr):
        return pvc.IRExpr_Triop(get_int_from_enum(expr.op), *[IRExpr._to_c(arg) for arg in expr.args])

    def result_type(self, tyenv):
        return get_op_retty(self.op)

    def typecheck(self, tyenv):
        resty, (arg1ty, arg2ty, arg3ty) = op_arg_types(self.op)
        arg1ty_real = self.args[0].typecheck(tyenv)
        arg2ty_real = self.args[1].typecheck(tyenv)
        arg3ty_real = self.args[2].typecheck(tyenv)
        if arg1ty_real is None or arg2ty_real is None or arg3ty_real is None:
            return None

        if arg1ty_real != arg1ty:
            log.debug("First arg of %s must be %s", self.op, arg1ty)
            return None
        if arg2ty_real != arg2ty:
            log.debug("Second arg of %s must be %s", self.op, arg2ty)
            return None
        if arg3ty_real != arg3ty:
            log.debug("Third arg of %s must be %s", self.op, arg3ty)
            return None

        return resty


class Binop(IRExpr):
    """
    A binary operation (2 arguments).
    """

    __slots__ = ["_op", "op_int", "args"]

    tag = "Iex_Binop"

    def __init__(self, op, args, op_int=None):
        self.op_int = op_int
        self.args = args
        self._op = op if op is not None else None

    def _pp_str(self):
        return "{}({})".format(self.op[4:], ",".join(str(a) for a in self.args))

    @property
    def op(self):
        if self._op is None:
            self._op = get_enum_from_int(self.op_int)
        return self._op

    @property
    def child_expressions(self):
        expressions = sum((a.child_expressions for a in self.args), [])
        expressions.extend(self.args)
        return expressions

    @staticmethod
    def _from_c(c_expr):
        return Binop(
            None,
            [IRExpr._from_c(arg) for arg in [c_expr.Iex.Binop.arg1, c_expr.Iex.Binop.arg2]],
            op_int=c_expr.Iex.Binop.op,
        )

    @staticmethod
    def _to_c(expr):
        return pvc.IRExpr_Binop(get_int_from_enum(expr.op), *[IRExpr._to_c(arg) for arg in expr.args])

    def result_type(self, tyenv):
        return get_op_retty(self.op)

    def typecheck(self, tyenv):
        arg1ty_real = self.args[0].typecheck(tyenv)
        arg2ty_real = self.args[1].typecheck(tyenv)

        resty, (arg1ty, arg2ty) = op_arg_types(self.op)
        if arg1ty_real is None or arg2ty_real is None:
            return None

        if arg1ty_real != arg1ty:
            log.debug("First arg of %s must be %s", self.op, arg1ty)
            return None
        if arg2ty_real != arg2ty:
            log.debug("Second arg of %s must be %s", self.op, arg2ty)
            return None

        return resty


class Unop(IRExpr):
    """
    A unary operation (1 argument).
    """

    __slots__ = ["op", "args"]

    tag = "Iex_Unop"

    def __init__(self, op: str, args: list[IRExpr]):
        self.op = op
        self.args = args

    def _pp_str(self):
        return "{}({})".format(self.op[4:], ",".join(str(a) for a in self.args))

    @property
    def child_expressions(self):
        expressions = sum((a.child_expressions for a in self.args), [])
        expressions.extend(self.args)
        return expressions

    @staticmethod
    def _from_c(c_expr):
        return Unop(get_enum_from_int(c_expr.Iex.Unop.op), [IRExpr._from_c(c_expr.Iex.Unop.arg)])

    @staticmethod
    def _to_c(expr):
        return pvc.IRExpr_Unop(get_int_from_enum(expr.op), IRExpr._to_c(expr.args[0]))

    def result_type(self, tyenv):
        return get_op_retty(self.op)

    def typecheck(self, tyenv):
        resty, (arg1ty,) = op_arg_types(self.op)
        arg1ty_real = self.args[0].typecheck(tyenv)
        if arg1ty_real is None:
            return None

        if arg1ty_real != arg1ty:
            log.debug("First arg of %s must be %s", self.op, arg1ty)
            return None

        return resty


class Load(IRExpr):
    """
    A load from memory.
    """

    __slots__ = ["end", "ty", "addr"]

    tag = "Iex_Load"

    def __init__(self, end, ty, addr):
        self.end = end
        self.ty = ty
        self.addr = addr

    @property
    def endness(self):
        return self.end

    @property
    def type(self):
        return self.ty

    def _pp_str(self):
        return f"LD{self.end[-2:].lower()}:{self.ty[4:]}({self.addr})"

    @staticmethod
    def _from_c(c_expr):
        return Load(
            get_enum_from_int(c_expr.Iex.Load.end),
            get_enum_from_int(c_expr.Iex.Load.ty),
            IRExpr._from_c(c_expr.Iex.Load.addr),
        )

    @staticmethod
    def _to_c(expr):
        return pvc.IRExpr_Load(get_int_from_enum(expr.end), get_int_from_enum(expr.ty), IRExpr._to_c(expr.addr))

    def result_type(self, tyenv):
        return self.ty

    def typecheck(self, tyenv):
        addrty = self.addr.typecheck(tyenv)
        if addrty is None:
            return None
        if addrty != tyenv.wordty:
            log.debug("Address must be word-sized")
            return None
        return self.ty


class Const(IRExpr):
    """
    A constant expression.
    """

    __slots__ = ["_con"]

    tag = "Iex_Const"

    def __init__(self, con: IRConst):
        self._con = con

    def _pp_str(self):
        return str(self.con)

    @property
    def con(self) -> IRConst:
        return self._con

    @staticmethod
    def _from_c(c_expr):
        con = IRConst._from_c(c_expr.Iex.Const.con)
        return Const.get_instance(con)

    @staticmethod
    def _to_c(expr):
        return pvc.IRExpr_Const(IRConst._to_c(expr.con))

    @staticmethod
    def get_instance(con):
        if con.value < 1024 and con.__class__ in _CONST_POOL:
            return _CONST_POOL[con.__class__][con.value]
        return Const(con)

    def result_type(self, tyenv):
        return self.con.type


_CONST_POOL = {
    U8: [Const(U8(i)) for i in range(0, 1024)],
    U16: [Const(U16(i)) for i in range(0, 1024)],
    U32: [Const(U32(i)) for i in range(0, 1024)],
    U64: [Const(U64(i)) for i in range(0, 1024)],
}


class ITE(IRExpr):
    """
    An if-then-else expression.
    """

    __slots__ = ["cond", "iffalse", "iftrue"]

    tag = "Iex_ITE"

    def __init__(self, cond, iffalse, iftrue):
        self.cond = cond
        self.iffalse = iffalse
        self.iftrue = iftrue

    def _pp_str(self):
        return f"ITE({self.cond},{self.iftrue},{self.iffalse})"

    @staticmethod
    def _from_c(c_expr):
        return ITE(
            IRExpr._from_c(c_expr.Iex.ITE.cond),
            IRExpr._from_c(c_expr.Iex.ITE.iffalse),
            IRExpr._from_c(c_expr.Iex.ITE.iftrue),
        )

    @staticmethod
    def _to_c(expr):
        return pvc.IRExpr_ITE(IRExpr._to_c(expr.cond), IRExpr._to_c(expr.iftrue), IRExpr._to_c(expr.iffalse))

    def result_type(self, tyenv):
        return self.iftrue.result_type(tyenv)

    def typecheck(self, tyenv):
        condty = self.cond.typecheck(tyenv)
        falsety = self.iffalse.typecheck(tyenv)
        truety = self.iftrue.typecheck(tyenv)

        if condty is None or falsety is None or truety is None:
            return None

        if condty != "Ity_I1":
            log.debug("guard must be Ity_I1")
            return None

        if falsety != truety:
            log.debug("false condition must be same type as true condition")
            return None

        return falsety


class CCall(IRExpr):
    """
    A call to a pure (no side-effects) helper C function.
    """

    __slots__ = ["retty", "cee", "args"]

    tag = "Iex_CCall"

    def __init__(self, retty, cee, args):
        self.retty = retty
        self.cee = cee
        self.args = tuple(args)

    @property
    def ret_type(self):
        return self.retty

    @property
    def callee(self):
        return self.cee

    def _pp_str(self):
        return "{}({}):{}".format(self.cee, ",".join(str(a) for a in self.args), self.retty)

    @property
    def child_expressions(self):
        expressions = sum((a.child_expressions for a in self.args), [])
        expressions.extend(self.args)
        return expressions

    @staticmethod
    def _from_c(c_expr):
        i = 0
        args = []
        while True:
            arg = c_expr.Iex.CCall.args[i]
            if arg == ffi.NULL:
                break
            args.append(IRExpr._from_c(arg))
            i += 1

        return CCall(get_enum_from_int(c_expr.Iex.CCall.retty), IRCallee._from_c(c_expr.Iex.CCall.cee), tuple(args))

    @staticmethod
    def _to_c(expr):
        args = [IRExpr._to_c(arg) for arg in expr.args]
        mkIRExprVec = getattr(pvc, "mkIRExprVec_%d" % len(args))
        return pvc.IRExpr_CCall(IRCallee._to_c(expr.cee), get_int_from_enum(expr.retty), mkIRExprVec(*args))

    def result_type(self, tyenv):
        return self.retty


def get_op_retty(op):
    return op_arg_types(op)[0]


op_signatures: dict[str, tuple[str, tuple[str, ...]]] = {}


def _request_op_type_from_cache(op):
    return op_signatures[op]


def _request_op_type_from_libvex(op):
    Ity_INVALID = 0x1100  # as defined in enum IRType in VEX

    res_ty = ffi.new("IRType *")
    arg_tys = [ffi.new("IRType *") for _ in range(4)]
    # initialize all IRTypes to Ity_INVALID
    for arg in arg_tys:
        arg[0] = Ity_INVALID
    pvc.typeOfPrimop(get_int_from_enum(op), res_ty, *arg_tys)
    arg_ty_vals = [a[0] for a in arg_tys]

    try:
        numargs = arg_ty_vals.index(Ity_INVALID)
    except ValueError:
        numargs = 4
    args_tys_list = [get_enum_from_int(arg_ty_vals[i]) for i in range(numargs)]

    op_ty_sig = (get_enum_from_int(res_ty[0]), tuple(args_tys_list))
    op_signatures[op] = op_ty_sig
    return op_ty_sig


class PyvexOpMatchException(Exception):
    pass


class PyvexTypeErrorException(Exception):
    pass


def int_type_for_size(size):
    return "Ity_I%d" % size


# precompiled regexes
unop_signature_re = re.compile(r"Iop_(Not|Ctz|Clz)(?P<size>\d+)$")
binop_signature_re = re.compile(r"Iop_(Add|Sub|Mul|Xor|Or|And|Div[SU]|Mod)(?P<size>\d+)$")
shift_signature_re = re.compile(r"Iop_(Shl|Shr|Sar)(?P<size>\d+)$")
cmp_signature_re_1 = re.compile(r"Iop_Cmp(EQ|NE)(?P<size>\d+)$")
cmp_signature_re_2 = re.compile(r"Iop_Cmp(GT|GE|LT|LE)(?P<size>\d+)[SU]$")
mull_signature_re = re.compile(r"Iop_Mull[SU](?P<size>\d+)$")
half_signature_re = re.compile(r"Iop_DivMod[SU](?P<fullsize>\d+)to(?P<halfsize>\d+)$")
cast_signature_re = re.compile(r"Iop_(?P<srcsize>\d+)(U|S|HI|HL)?to(?P<dstsize>\d+)")


def unop_signature(op):
    m = unop_signature_re.match(op)
    if m is None:
        raise PyvexOpMatchException()
    size = int(m.group("size"))
    size_type = int_type_for_size(size)
    return size_type, (size_type,)


def binop_signature(op):
    m = binop_signature_re.match(op)
    if m is None:
        raise PyvexOpMatchException()
    size = int(m.group("size"))
    size_type = int_type_for_size(size)
    return (size_type, (size_type, size_type))


def shift_signature(op):
    m = shift_signature_re.match(op)
    if m is None:
        raise PyvexOpMatchException()
    size = int(m.group("size"))
    if size > 255:
        raise PyvexTypeErrorException("Cannot apply shift operation to %d size int because shift index is 8-bit" % size)
    size_type = int_type_for_size(size)
    return (size_type, (size_type, int_type_for_size(8)))


def cmp_signature(op):
    m = cmp_signature_re_1.match(op)
    m2 = cmp_signature_re_2.match(op)
    if (m is None) == (m2 is None):
        raise PyvexOpMatchException()
    mfound = m if m is not None else m2
    assert mfound is not None
    size = int(mfound.group("size"))
    size_type = int_type_for_size(size)
    return (int_type_for_size(1), (size_type, size_type))


def mull_signature(op):
    m = mull_signature_re.match(op)
    if m is None:
        raise PyvexOpMatchException()
    size = int(m.group("size"))
    size_type = int_type_for_size(size)
    doubled_size_type = int_type_for_size(2 * size)
    return (doubled_size_type, (size_type, size_type))


def half_signature(op):
    m = half_signature_re.match(op)
    if m is None:
        raise PyvexOpMatchException()
    fullsize = int(m.group("fullsize"))
    halfsize = int(m.group("halfsize"))
    if halfsize * 2 != fullsize:
        raise PyvexTypeErrorException("Invalid Instruction %s: Type 1 must be twice the size of type 2" % op)
    fullsize_type = int_type_for_size(fullsize)
    halfsize_type = int_type_for_size(halfsize)
    return (fullsize_type, (fullsize_type, halfsize_type))


def cast_signature(op):
    m = cast_signature_re.match(op)
    if m is None:
        raise PyvexOpMatchException()
    src_type = int_type_for_size(int(m.group("srcsize")))
    dst_type = int_type_for_size(int(m.group("dstsize")))
    return (dst_type, (src_type,))


polymorphic_op_processors = [
    unop_signature,
    binop_signature,
    shift_signature,
    cmp_signature,
    mull_signature,
    half_signature,
    cast_signature,
]


def _request_polymorphic_op_type(op):
    for polymorphic_signature in polymorphic_op_processors:
        try:
            op_ty_sig = polymorphic_signature(op)
            break
        except PyvexOpMatchException:
            continue
    else:
        raise PyvexOpMatchException("Op %s not recognized" % op)
    return op_ty_sig


_request_funcs = [_request_op_type_from_cache, _request_op_type_from_libvex, _request_polymorphic_op_type]


def op_arg_types(op):
    for _request_func in _request_funcs:
        try:
            return _request_func(op)
        except KeyError:
            continue
    raise ValueError("Cannot find type of op %s" % op)


_globals = globals().copy()
#
# Mapping from tag strings/enums to IRExpr classes
#
tag_to_expr_mapping = {}
enum_to_expr_mapping = {}
tag_count = 0
cls = None
for cls in _globals.values():
    if type(cls) is type and issubclass(cls, IRExpr) and cls is not IRExpr:
        tag_to_expr_mapping[cls.tag] = cls
        enum_to_expr_mapping[get_int_from_enum(cls.tag)] = cls
        cls.tag_int = tag_count
        tag_count += 1
del cls


def tag_to_expr_class(tag):
    """
    Convert a tag string to the corresponding IRExpr class type.

    :param str tag: The tag string.
    :return:        A class.
    :rtype:         type
    """

    try:
        return tag_to_expr_mapping[tag]
    except KeyError:
        raise KeyError("Cannot find expression class for type %s." % tag)


def enum_to_expr_class(tag_enum):
    """
    Convert a tag enum to the corresponding IRExpr class.

    :param int tag_enum: The tag enum.
    :return:             A class.
    :rtype:              type
    """

    try:
        return enum_to_expr_mapping[tag_enum]
    except KeyError:
        raise KeyError("Cannot find expression class for type %s." % get_enum_from_int(tag_enum))
