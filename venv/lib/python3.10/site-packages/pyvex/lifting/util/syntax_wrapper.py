import functools
from typing import Union

from pyvex.const import get_type_size
from pyvex.expr import Const, IRExpr, RdTmp

from .vex_helper import IRSBCustomizer, Type


def checkparams(rhstype=None):
    def decorator(fn):
        @functools.wraps(fn)
        def inner_decorator(self, *args, **kwargs):
            irsb_cs = {a.irsb_c for a in list(args) + [self] if isinstance(a, VexValue)}  # pylint: disable=no-member
            assert len(irsb_cs) == 1, "All VexValues must belong to the same irsb_c"
            args = list(args)
            for idx, arg in enumerate(args):
                if isinstance(arg, int):
                    thetype = rhstype if rhstype else self.ty
                    args[idx] = VexValue.Constant(self.irsb_c, arg, thetype)
                elif not isinstance(arg, VexValue):
                    raise Exception("Cannot convert param %s" % str(arg))
            args = tuple(args)
            return fn(self, *args, **kwargs)

        return inner_decorator

    return decorator


def vvifyresults(f):
    @functools.wraps(f)
    def decor(self, *args, **kwargs):
        returned = f(self, *args, **kwargs)
        assert isinstance(returned, RdTmp) or isinstance(returned, Const)
        return VexValue(self.irsb_c, returned)

    return decor


class VexValue:
    def __init__(self, irsb_c: "IRSBCustomizer", rdt: "Union[RdTmp, Const]", signed=False):
        self.irsb_c = irsb_c
        self.ty = self.irsb_c.get_type(rdt)
        self.rdt = rdt
        self.width = get_type_size(self.ty)
        self._is_signed = signed

    @property
    def value(self):
        if isinstance(self.rdt, Const):
            return self.rdt.con.value
        else:
            raise ValueError("Non-constant VexValue has no value property")

    @property
    def signed(self):
        return VexValue(self.irsb_c, self.rdt, True)

    @vvifyresults
    def widen_unsigned(self, ty):
        return self.irsb_c.op_widen_int_unsigned(self.rdt, ty)

    @vvifyresults
    def cast_to(self, ty, signed=False, high=False):
        return self.irsb_c.cast_to(self.rdt, ty, signed=signed, high=high)

    @vvifyresults
    def widen_signed(self, ty):
        return self.irsb_c.op_widen_int_signed(self.rdt, ty)

    @vvifyresults
    def narrow_high(self, ty):
        return self.irsb_c.op_narrow_int(self.rdt, ty, high_half=True)

    @vvifyresults
    def narrow_low(self, ty):
        return self.irsb_c.op_narrow_int(self.rdt, ty, high_half=False)

    # TODO at some point extend this to Vex nonconstants
    def __getitem__(self, idx):
        def getb(i):
            return VexValue(self.irsb_c, self.irsb_c.get_bit(self.rdt, i))

        def makeconstant(x):
            return VexValue.Constant(self.irsb_c, x, Type.int_8).rdt

        if not isinstance(idx, slice):
            actualindex = slice(idx).indices(self.width)[1]
            return getb(makeconstant(actualindex))
        else:
            return [getb(makeconstant(i)) for i in range(*idx.indices(self.width))]

    def __setitem__(self, idx, bval):
        setted = self.set_bit(idx, bval)
        self.__init__(setted.irsb_c, setted.rdt)

    @checkparams(rhstype=Type.int_8)
    @vvifyresults
    def set_bit(self, idx, bval):
        return self.irsb_c.set_bit(self.rdt, idx.rdt, bval.rdt)

    @checkparams()
    @vvifyresults
    def set_bits(self, idxsandvals):
        return self.irsb_c.set_bits(self.rdt, [(i.cast_to(Type.int_8).rdt, b.rdt) for i, b in idxsandvals])

    @checkparams()
    @vvifyresults
    def ite(self, iftrue, iffalse):
        onebitcond = self.cast_to(Type.int_1)
        return self.irsb_c.ite(onebitcond.rdt, iftrue.rdt, iffalse.rdt)

    @checkparams()
    @vvifyresults
    def sar(self, right):
        """
        `v.sar(r)` should do arithmetic shift right of `v` by `r`

        :param right:VexValue value to shift by
        :return: VexValue - result of a shift
        """
        return self.irsb_c.op_sar(self.rdt, right.rdt)

    @checkparams()
    @vvifyresults
    def __add__(self, right):
        return self.irsb_c.op_add(self.rdt, right.rdt)

    @checkparams()
    def __radd__(self, left):
        return self + left

    @checkparams()
    @vvifyresults
    def __sub__(self, right):
        return self.irsb_c.op_sub(self.rdt, right.rdt)

    @checkparams()
    def __rsub__(self, left):
        return left - self

    @checkparams()
    @vvifyresults
    def __div__(self, right):
        if self._is_signed:
            return self.irsb_c.op_sdiv(self.rdt, right.rdt)
        else:
            return self.irsb_c.op_udiv(self.rdt, right.rdt)

    @checkparams()
    def __rdiv__(self, left):
        return left // self

    @checkparams()
    def __floordiv__(self, right):  # Note: nonprimitive
        return self.__div__(right)

    @checkparams()
    def __rfloordiv__(self, left):
        return left // self

    @checkparams()
    def __truediv__(self, right):  # Note: nonprimitive
        return self / right

    @checkparams()
    def __rtruediv__(self, left):
        return left.__truediv__(self)

    @checkparams()
    @vvifyresults
    def __and__(self, right):
        return self.irsb_c.op_and(self.rdt, right.rdt)

    @checkparams()
    def __rand__(self, left):
        return left & self

    @checkparams()
    @vvifyresults
    def __eq__(self, right):
        return self.irsb_c.op_cmp_eq(self.rdt, right.rdt)

    @checkparams()
    @vvifyresults
    def __ne__(self, other):
        return self.irsb_c.op_cmp_ne(self.rdt, other.rdt)

    @checkparams()
    @vvifyresults
    def __invert__(self):
        return self.irsb_c.op_not(self.rdt)

    @checkparams()
    @vvifyresults
    def __le__(self, right):
        if self._is_signed:
            return self.irsb_c.op_cmp_sle(self.rdt, right.rdt)
        else:
            return self.irsb_c.op_cmp_ule(self.rdt, right.rdt)

    @checkparams()
    @vvifyresults
    def __gt__(self, other):
        if self._is_signed:
            return self.irsb_c.op_cmp_sgt(self.rdt, other.rdt)
        else:
            return self.irsb_c.op_cmp_ugt(self.rdt, other.rdt)

    @checkparams()
    @vvifyresults
    def __ge__(self, right):
        if self._is_signed:
            return self.irsb_c.op_cmp_sge(self.rdt, right.rdt)
        else:
            return self.irsb_c.op_cmp_uge(self.rdt, right.rdt)

    @checkparams(rhstype=Type.int_8)
    @vvifyresults
    def __lshift__(self, right):  # TODO put better type inference in irsb_c so we can have rlshift
        """
        logical shift left
        """
        return self.irsb_c.op_shl(self.rdt, right.rdt)

    @checkparams()
    @vvifyresults
    def __lt__(self, right):
        if self._is_signed:
            return self.irsb_c.op_cmp_slt(self.rdt, right.rdt)
        else:
            return self.irsb_c.op_cmp_ult(self.rdt, right.rdt)

    @checkparams()
    @vvifyresults
    def __mod__(self, right):  # Note: nonprimitive
        return self.irsb_c.op_mod(self.rdt, right.rdt)

    @checkparams()
    def __rmod__(self, left):
        return left % self

    @checkparams()
    @vvifyresults
    def __mul__(self, right):
        if self._is_signed:
            return self.irsb_c.op_smul(self.rdt, right.rdt)
        else:
            return self.irsb_c.op_umul(self.rdt, right.rdt)

    @checkparams()
    def __rmul__(self, left):
        return left * self

    @checkparams()
    @vvifyresults
    def __neg__(self):  # Note: nonprimitive
        if not self._is_signed:
            raise Exception("Number is unsigned, cannot change sign!")
        else:
            return self.rdt * -1

    @checkparams()
    @vvifyresults
    def __or__(self, right):
        return self.irsb_c.op_or(self.rdt, right.rdt)

    def __ror__(self, left):
        return self | left

    @checkparams()
    @vvifyresults
    def __pos__(self):
        return self

    @checkparams(rhstype=Type.int_8)
    @vvifyresults
    def __rshift__(self, right):
        """
        logical shift right
        """
        return self.irsb_c.op_shr(self.rdt, right.rdt)

    @checkparams()
    def __rlshift__(self, left):
        return left << self

    @checkparams()
    def __rrshift__(self, left):
        return left >> self

    @checkparams()
    @vvifyresults
    def __xor__(self, right):
        return self.irsb_c.op_xor(self.rdt, right.rdt)

    def __rxor__(self, left):
        return self ^ left

    @classmethod
    def Constant(cls, irsb_c, val, ty):
        """
        Creates a constant as a VexValue
        :param irsb_c: The IRSBCustomizer to use
        :param val: The value, as an integer
        :param ty: The type of the resulting VexValue
        :return: a VexValue
        """
        assert not (isinstance(val, VexValue) or isinstance(val, IRExpr))
        rdt = irsb_c.mkconst(val, ty)
        return cls(irsb_c, rdt)
