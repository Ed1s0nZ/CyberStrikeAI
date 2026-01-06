from __future__ import annotations

import logging
from collections.abc import Iterator
from typing import TYPE_CHECKING

from . import expr
from .const import IRConst
from .enums import IRCallee, IRRegArray, VEXObject, get_enum_from_int, get_int_from_enum
from .errors import PyVEXError
from .expr import Const, Get, IRExpr
from .native import ffi, pvc

if TYPE_CHECKING:
    from .block import IRTypeEnv

log = logging.getLogger("pyvex.stmt")


class IRStmt(VEXObject):
    """
    IR statements in VEX represents operations with side-effects.
    """

    tag: str
    tag_int = 0  # set automatically at bottom of file

    __slots__ = []

    def pp(self):
        print(str(self))

    @property
    def child_expressions(self) -> Iterator[IRExpr]:
        for k in self.__slots__:
            v = getattr(self, k)
            if isinstance(v, IRExpr):
                # return itself
                yield v
                # return all the child expressions
                yield from v.child_expressions

    # ???
    @property
    def expressions(self):
        return self.child_expressions

    @property
    def constants(self):
        return sum((e.constants for e in self.expressions), [])

    @staticmethod
    def _from_c(c_stmt):
        if c_stmt[0] == ffi.NULL:
            return None

        try:
            stmt_class = enum_to_stmt_class(c_stmt.tag)
        except KeyError:
            raise PyVEXError("Unknown/unsupported IRStmtTag %s.\n" % get_enum_from_int(c_stmt.tag))
        return stmt_class._from_c(c_stmt)

    def typecheck(self, tyenv: IRTypeEnv) -> bool:  # pylint: disable=unused-argument,no-self-use
        return True

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
            elif isinstance(v, IRExpr):
                v.replace_expression(replacements)
            elif type(v) is tuple:
                # Rebuild the tuple
                _lst = []
                replaced = False
                for expr_ in v:
                    if isinstance(expr_, IRExpr) and expr_ in replacements:
                        _lst.append(replacements.get(expr_))
                        replaced = True
                    else:
                        _lst.append(expr_)
                if replaced:
                    setattr(self, k, tuple(_lst))

    def __str__(self):
        return self.pp_str(None, None, None)

    def pp_str(self, reg_name=None, arch=None, tyenv=None) -> str:
        raise NotImplementedError()


class NoOp(IRStmt):
    """
    A no-operation statement. It is usually the result of an IR optimization.
    """

    __slots__ = []

    tag = "Ist_NoOp"

    def pp_str(self, reg_name=None, arch=None, tyenv=None):
        return "IR-NoOp"

    @staticmethod
    def _from_c(c_stmt):
        return NoOp()


class IMark(IRStmt):
    """
    An instruction mark. It marks the start of the statements that represent a single machine instruction (the end of
    those statements is marked by the next IMark or the end of the IRSB).  Contains the address and length of the
    instruction.
    """

    __slots__ = ["addr", "len", "delta"]

    tag = "Ist_IMark"

    def __init__(self, addr: int, length: int, delta: int):
        self.addr = addr
        self.len = length
        self.delta = delta

    def pp_str(self, reg_name=None, arch=None, tyenv=None):
        return "------ IMark(0x%x, %d, %d) ------" % (self.addr, self.len, self.delta)

    @staticmethod
    def _from_c(c_stmt):
        return IMark(c_stmt.Ist.IMark.addr, c_stmt.Ist.IMark.len, c_stmt.Ist.IMark.delta)


class AbiHint(IRStmt):
    """
    An ABI hint, provides specific information about this platform's ABI.
    """

    __slots__ = ["base", "len", "nia"]

    tag = "Ist_AbiHint"

    def __init__(self, base, length, nia):
        self.base = base
        self.len = length
        self.nia = nia

    def pp_str(self, reg_name=None, arch=None, tyenv=None):
        return "====== AbiHint(0x%s, %d, %s) ======" % (self.base, self.len, self.nia)

    @staticmethod
    def _from_c(c_stmt):
        return AbiHint(
            IRExpr._from_c(c_stmt.Ist.AbiHint.base), c_stmt.Ist.AbiHint.len, IRExpr._from_c(c_stmt.Ist.AbiHint.nia)
        )


class Put(IRStmt):
    """
    Write to a guest register, at a fixed offset in the guest state.
    """

    __slots__ = ["data", "offset"]

    tag = "Ist_Put"

    def __init__(self, data: IRExpr, offset: int):
        self.data = data
        self.offset = offset

    ## TODO: Check if result_size and arch are available before looking of arch register name
    def pp_str(self, reg_name=None, arch=None, tyenv=None):
        if arch is not None and tyenv is not None:
            reg_name = arch.translate_register_name(self.offset, self.data.result_size(tyenv) // 8)

        if reg_name is not None:
            return f"PUT({reg_name}) = {self.data}"
        else:
            return f"PUT(offset={self.offset}) = {self.data}"

    @staticmethod
    def _from_c(c_stmt):
        return Put(IRExpr._from_c(c_stmt.Ist.Put.data), c_stmt.Ist.Put.offset)

    def typecheck(self, tyenv):
        return self.data.typecheck(tyenv)


class PutI(IRStmt):
    """
    Write to a guest register, at a non-fixed offset in the guest state.
    """

    __slots__ = ["descr", "ix", "data", "bias"]

    tag = "Ist_PutI"

    def __init__(self, descr, ix, data, bias):
        self.descr = descr
        self.ix = ix
        self.data = data
        self.bias = bias

    def pp_str(self, reg_name=None, arch=None, tyenv=None):
        return "PutI(%s)[%s,%d] = %s" % (self.descr, self.ix, self.bias, self.data)

    @staticmethod
    def _from_c(c_stmt):
        return PutI(
            IRRegArray._from_c(c_stmt.Ist.PutI.details.descr),
            IRExpr._from_c(c_stmt.Ist.PutI.details.ix),
            IRExpr._from_c(c_stmt.Ist.PutI.details.data),
            c_stmt.Ist.PutI.details.bias,
        )

    def typecheck(self, tyenv):
        dataty = self.data.typecheck(tyenv)
        if dataty is None:
            return False
        if dataty != self.descr.elemTy:
            log.debug("Expression doesn't match RegArray type")
            return False
        return True


class WrTmp(IRStmt):
    """
    Assign a value to a temporary.  Note that SSA rules require each tmp is only assigned to once.  IR sanity checking
    will reject any block containing a temporary which is not assigned to exactly once.
    """

    __slots__ = ["data", "tmp"]

    tag = "Ist_WrTmp"

    def __init__(self, tmp, data: IRExpr):
        self.tmp = tmp
        self.data = data

    def pp_str(self, reg_name=None, arch=None, tyenv=None):
        # Support for named register in string representation of expr.Get

        if arch is not None and tyenv is not None and isinstance(self.data, Get):
            reg_name = arch.translate_register_name(self.data.offset, self.data.result_size(tyenv) // 8)

        if reg_name is not None and isinstance(self.data, expr.Get):
            return "t%d = %s" % (self.tmp, self.data.pp_str_with_name(reg_name))
        else:
            return "t%d = %s" % (self.tmp, self.data)

    @staticmethod
    def _from_c(c_stmt):
        return WrTmp(c_stmt.Ist.WrTmp.tmp, IRExpr._from_c(c_stmt.Ist.WrTmp.data))

    def typecheck(self, tyenv):
        dataty = self.data.typecheck(tyenv)
        if dataty is None:
            return False
        if dataty != tyenv.lookup(self.tmp):
            log.debug("Expression doesn't match tmp type")
            return False
        return True


class Store(IRStmt):
    """
    Write a value to memory..
    """

    __slots__ = ["addr", "data", "end"]

    tag = "Ist_Store"

    def __init__(self, addr: IRExpr, data: IRExpr, end: str):
        self.addr = addr
        self.data = data
        self.end = end

    @property
    def endness(self):
        return self.end

    def pp_str(self, reg_name=None, arch=None, tyenv=None):
        return f"ST{self.endness[-2:].lower()}({self.addr}) = {self.data}"

    @staticmethod
    def _from_c(c_stmt):
        return Store(
            IRExpr._from_c(c_stmt.Ist.Store.addr),
            IRExpr._from_c(c_stmt.Ist.Store.data),
            get_enum_from_int(c_stmt.Ist.Store.end),
        )

    def typecheck(self, tyenv):
        dataty = self.data.typecheck(tyenv)
        if dataty is None:
            return False
        addrty = self.addr.typecheck(tyenv)
        if addrty is None:
            return False
        if addrty != tyenv.wordty:
            log.debug("addr must be full word for arch")
            return False
        if self.end not in ("Iend_LE", "Iend_BE"):
            log.debug("invalid endness enum")
            return False
        return True


class CAS(IRStmt):
    """
    an atomic compare-and-swap operation.
    """

    __slots__ = ["addr", "dataLo", "dataHi", "expdLo", "expdHi", "oldLo", "oldHi", "end"]

    tag = "Ist_CAS"

    def __init__(self, addr, dataLo, dataHi, expdLo, expdHi, oldLo, oldHi, end):
        self.addr = addr
        self.dataLo = dataLo
        self.dataHi = dataHi
        self.expdLo = expdLo
        self.expdHi = expdHi
        self.oldLo = oldLo
        self.oldHi = oldHi
        self.end = end

    @property
    def endness(self):
        return self.end

    def pp_str(self, reg_name=None, arch=None, tyenv=None):
        return "t({},{}) = CAS{}({} :: ({},{})->({},{}))".format(
            self.oldLo, self.oldHi, self.end[-2:].lower(), self.addr, self.expdLo, self.expdHi, self.dataLo, self.dataHi
        )

    @staticmethod
    def _from_c(c_stmt):
        return CAS(
            IRExpr._from_c(c_stmt.Ist.CAS.details.addr),
            IRExpr._from_c(c_stmt.Ist.CAS.details.dataLo),
            IRExpr._from_c(c_stmt.Ist.CAS.details.dataHi),
            IRExpr._from_c(c_stmt.Ist.CAS.details.expdLo),
            IRExpr._from_c(c_stmt.Ist.CAS.details.expdHi),
            c_stmt.Ist.CAS.details.oldLo,
            c_stmt.Ist.CAS.details.oldHi,
            get_enum_from_int(c_stmt.Ist.CAS.details.end),
        )

    def typecheck(self, tyenv):
        addrty = self.addr.typecheck(tyenv)
        if addrty is None:
            return False
        if addrty != tyenv.wordty:
            log.debug("addr must be full word for arch")
            return False
        if self.end not in ("Iend_LE", "Iend_BE"):
            log.debug("invalid endness enum")
            return False

        if self.oldHi == 0xFFFFFFFF:
            # single-element case
            if self.expdHi is not None or self.dataHi is not None:
                log.debug("expdHi and dataHi must be None")
                return False
            expdLoTy = self.expdLo.typecheck(tyenv)
            dataLoTy = self.dataLo.typecheck(tyenv)
            if expdLoTy is None or dataLoTy is None:
                return False
            if tyenv.lookup(self.oldLo) != expdLoTy or expdLoTy != dataLoTy:
                log.debug("oldLo, expdL, dataLo must all have the same type")
                return False
        else:
            # double-element case
            expdLoTy = self.expdLo.typecheck(tyenv)
            dataLoTy = self.dataLo.typecheck(tyenv)
            expdHiTy = self.expdHi.typecheck(tyenv)
            dataHiTy = self.dataHi.typecheck(tyenv)
            if expdLoTy is None or dataLoTy is None or expdHiTy is None or dataHiTy is None:
                return False
            if (
                tyenv.lookup(self.oldLo) != expdLoTy
                or expdLoTy != dataLoTy
                or tyenv.lookup(self.oldHi) != expdHiTy
                or expdHiTy != dataHiTy
                or expdLoTy != expdHiTy
            ):
                log.debug("oldLo, expdLo, dataLo, oldHi, expdHi, dataHi must all have the same type")
                return False

        return True


class LLSC(IRStmt):
    """
    Either Load-Linked or Store-Conditional, depending on STOREDATA. If STOREDATA is NULL then this is a Load-Linked,
    else it is a Store-Conditional.
    """

    __slots__ = ["addr", "storedata", "result", "end"]

    tag = "Ist_LLSC"

    def __init__(self, addr: IRExpr, storedata: IRExpr, result: int, end: str):
        self.addr = addr
        self.storedata = storedata
        self.result = result
        self.end = end

    @property
    def endness(self):
        return self.end

    def pp_str(self, reg_name=None, arch=None, tyenv=None):
        if self.storedata is None:
            return "t%d = LD%s-Linked(%s)" % (self.result, self.end[-2:].lower(), self.addr)
        else:
            return "t%d = ( ST%s-Cond(%s) = %s )" % (self.result, self.end[-2:].lower(), self.addr, self.storedata)

    @staticmethod
    def _from_c(c_stmt):
        return LLSC(
            IRExpr._from_c(c_stmt.Ist.LLSC.addr),
            IRExpr._from_c(c_stmt.Ist.LLSC.storedata),
            c_stmt.Ist.LLSC.result,
            get_enum_from_int(c_stmt.Ist.LLSC.end),
        )

    def typecheck(self, tyenv):
        addrty = self.addr.typecheck(tyenv)
        if addrty is None:
            return False
        if addrty != tyenv.wordty:
            log.debug("addr must be full word for arch")
            return False
        if self.end not in ("Iend_LE", "Iend_BE"):
            log.debug("invalid endness enum")
            return False

        if self.storedata is not None:
            # load-linked
            storety = self.storedata.typecheck(tyenv)
            if storety is None:
                return False

            if tyenv.lookup(self.result) != "Ity_I1":
                log.debug("result tmp must be Ity_I1")
                return False

        return True


class MBE(IRStmt):
    __slots__ = ["event"]

    tag = "Ist_MBE"

    def __init__(self, event):
        self.event = event

    def pp_str(self, reg_name=None, arch=None, tyenv=None):
        return "MBusEvent-" + self.event

    @staticmethod
    def _from_c(c_stmt):
        return MBE(get_enum_from_int(c_stmt.Ist.MBE.event))


class Dirty(IRStmt):
    __slots__ = ["cee", "guard", "args", "tmp", "mFx", "mAddr", "mSize", "nFxState"]

    tag = "Ist_Dirty"

    def __init__(self, cee, guard, args, tmp, mFx, mAddr, mSize, nFxState):
        self.cee = cee
        self.guard = guard
        self.args = tuple(args)
        self.tmp = tmp
        self.mFx = mFx
        self.mAddr = mAddr
        self.mSize = mSize
        self.nFxState = nFxState

    def pp_str(self, reg_name=None, arch=None, tyenv=None):
        return "t{} = DIRTY {} {} ::: {}({})".format(
            self.tmp, self.guard, "TODO(effects)", self.cee, ",".join(str(a) for a in self.args)
        )

    @property
    def child_expressions(self):
        expressions = sum((a.child_expressions for a in self.args), [])
        expressions.extend(self.args)
        expressions.append(self.guard)
        expressions.extend(self.guard.child_expressions)
        return expressions

    @staticmethod
    def _from_c(c_stmt):
        args = []
        for i in range(20):
            a = c_stmt.Ist.Dirty.details.args[i]
            if a == ffi.NULL:
                break

            args.append(IRExpr._from_c(a))

        return Dirty(
            IRCallee._from_c(c_stmt.Ist.Dirty.details.cee),
            IRExpr._from_c(c_stmt.Ist.Dirty.details.guard),
            tuple(args),
            c_stmt.Ist.Dirty.details.tmp,
            get_enum_from_int(c_stmt.Ist.Dirty.details.mFx),
            IRExpr._from_c(c_stmt.Ist.Dirty.details.mAddr),
            c_stmt.Ist.Dirty.details.mSize,
            c_stmt.Ist.Dirty.details.nFxState,
        )


class Exit(IRStmt):
    """
    A conditional exit from the middle of an IRSB.
    """

    __slots__ = ["guard", "dst", "offsIP", "jk"]

    tag = "Ist_Exit"

    def __init__(self, guard: IRExpr, dst: IRConst, jk: str, offsIP: int):
        self.guard = guard
        self.dst = dst
        self.offsIP = offsIP
        self.jk = jk

    @property
    def jumpkind(self):
        return self.jk

    def pp_str(self, reg_name=None, arch=None, tyenv=None):
        if arch is not None and tyenv is not None:
            reg_name = arch.translate_register_name(self.offsIP, arch.bits // 8)

        if reg_name is None:
            return "if (%s) { PUT(offset=%d) = %#x; %s }" % (self.guard, self.offsIP, self.dst.value, self.jumpkind)
        else:
            return f"if ({self.guard}) {{ PUT({reg_name}) = {self.dst.value:#x}; {self.jumpkind} }}"

    @property
    def child_expressions(self):
        return [self.guard] + self.guard.child_expressions + [Const(self.dst)]

    @staticmethod
    def _from_c(c_stmt):
        return Exit(
            IRExpr._from_c(c_stmt.Ist.Exit.guard),
            IRConst._from_c(c_stmt.Ist.Exit.dst),
            get_enum_from_int(c_stmt.Ist.Exit.jk),
            c_stmt.Ist.Exit.offsIP,
        )

    def typecheck(self, tyenv):
        if not self.jk.startswith("Ijk_"):
            log.debug("Jumpkind is not a jumpkind enum")
            return False
        guardty = self.guard.typecheck(tyenv)
        if guardty is None:
            return False
        if guardty != "Ity_I1":
            log.debug("guard must be Ity_I1")
            return False
        return True


class LoadG(IRStmt):
    """
    A guarded load.
    """

    __slots__ = ["addr", "alt", "guard", "dst", "cvt", "end", "cvt_types"]

    tag = "Ist_LoadG"

    def __init__(self, end: str, cvt: str, dst: int, addr: IRExpr, alt: IRExpr, guard: IRExpr):
        self.addr = addr
        self.alt = alt
        self.guard = guard
        self.dst = dst
        self.cvt = cvt
        self.end = end

        type_in = ffi.new("IRType *")  # TODO separate this from the pyvex C implementation
        type_out = ffi.new("IRType *")
        pvc.typeOfIRLoadGOp(get_int_from_enum(self.cvt), type_out, type_in)
        type_in = ffi.cast("int *", type_in)[0]
        type_out = ffi.cast("int *", type_out)[0]
        self.cvt_types = (get_enum_from_int(type_in), get_enum_from_int(type_out))

    @property
    def endness(self):
        return self.end

    def pp_str(self, reg_name=None, arch=None, tyenv=None):
        return "t%d = if (%s) %s(LD%s(%s)) else %s" % (
            self.dst,
            self.guard,
            self.cvt,
            self.end[-2:].lower(),
            self.addr,
            self.alt,
        )

    @staticmethod
    def _from_c(c_stmt):
        return LoadG(
            get_enum_from_int(c_stmt.Ist.LoadG.details.end),
            get_enum_from_int(c_stmt.Ist.LoadG.details.cvt),
            c_stmt.Ist.LoadG.details.dst,
            IRExpr._from_c(c_stmt.Ist.LoadG.details.addr),
            IRExpr._from_c(c_stmt.Ist.LoadG.details.alt),
            IRExpr._from_c(c_stmt.Ist.LoadG.details.guard),
        )

    def typecheck(self, tyenv):
        addrty = self.addr.typecheck(tyenv)
        if addrty is None:
            return False
        if addrty != tyenv.wordty:
            log.debug("addr must be full word for arch")
            return False
        if self.end not in ("Iend_LE", "Iend_BE"):
            log.debug("invalid endness enum")
            return False

        dstty = tyenv.lookup(self.dst)
        guardty = self.guard.typecheck(tyenv)
        altty = self.alt.typecheck(tyenv)

        if guardty is None or altty is None:
            return False
        if dstty != "Ity_I32" or altty != "Ity_I32":
            log.debug("dst and alt must be Ity_I32")
            return False
        if guardty != "Ity_I1":
            log.debug("guard must be Ity_I1")
            return False
        if not self.cvt.startswith("ILGop_"):
            log.debug("Invalid cvt enum")
            return False
        return True


class StoreG(IRStmt):
    """
    A guarded store.
    """

    __slots__ = ["addr", "data", "guard", "end"]

    tag = "Ist_StoreG"

    def __init__(self, end, addr, data, guard):
        self.addr = addr
        self.data = data
        self.guard = guard
        self.end = end

    @property
    def endness(self):
        return self.end

    def pp_str(self, reg_name=None, arch=None, tyenv=None):
        return f"if ({self.guard}) ST{self.end[-2:].lower()}({self.addr}) = {self.data}"

    @staticmethod
    def _from_c(c_stmt):
        return StoreG(
            get_enum_from_int(c_stmt.Ist.StoreG.details.end),
            IRExpr._from_c(c_stmt.Ist.StoreG.details.addr),
            IRExpr._from_c(c_stmt.Ist.StoreG.details.data),
            IRExpr._from_c(c_stmt.Ist.StoreG.details.guard),
        )

    def typecheck(self, tyenv):
        addrty = self.addr.typecheck(tyenv)
        if addrty is None:
            return False
        if addrty != tyenv.wordty:
            log.debug("addr must be full word for arch")
            return False
        if self.end not in ("Iend_LE", "Iend_BE"):
            log.debug("invalid endness enum")
            return False

        guardty = self.guard.typecheck(tyenv)
        dataty = self.data.typecheck(tyenv)

        if guardty is None or dataty is None:
            return False
        if guardty != "Ity_I1":
            log.debug("guard must be Ity_I1")
            return False
        return True


_globals = globals().copy()
#
# Mapping from tag strings/enums to IRStmt classes
#
tag_to_stmt_mapping = {}
enum_to_stmt_mapping = {}
tag_count = 0
cls = None
for cls in _globals.values():
    if type(cls) is type and issubclass(cls, IRStmt) and cls is not IRStmt:
        tag_to_stmt_mapping[cls.tag] = cls
        enum_to_stmt_mapping[get_int_from_enum(cls.tag)] = cls
        cls.tag_int = tag_count
        tag_count += 1
del cls


def tag_to_stmt_class(tag):
    try:
        return tag_to_stmt_mapping[tag]
    except KeyError:
        raise KeyError("No statement class for tag %s." % tag)


def enum_to_stmt_class(tag_enum):
    try:
        return enum_to_stmt_mapping[tag_enum]
    except KeyError:
        raise KeyError("No statement class for tag %s." % get_enum_from_int(tag_enum))
