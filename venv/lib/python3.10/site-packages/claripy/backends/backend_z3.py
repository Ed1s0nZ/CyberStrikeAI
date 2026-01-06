from __future__ import annotations

import ctypes
import logging
import numbers
import operator
import os
import signal
import sys
import threading
import weakref
from decimal import Decimal
from functools import reduce
from typing import TYPE_CHECKING, Any

import z3
from cachetools import LRUCache

import claripy
from claripy.ast import BV, FP, Bool
from claripy.backends.backend import Backend
from claripy.errors import (
    BackendError,
    ClaripyError,
    ClaripyOperationError,
    ClaripySolverInterruptError,
    ClaripyZ3Error,
)
from claripy.fp import RM, FSort
from claripy.operations import backend_fp_operations, backend_operations, backend_strings_operations, bound_ops

if TYPE_CHECKING:
    from types import FrameType

    from claripy.annotation import Annotation


log = logging.getLogger(__name__)

# pylint:disable=unidiomatic-typecheck

ALL_Z3_CONTEXTS = weakref.WeakSet()
INT_STRING_CHUNK_SIZE: int | None = None  # will be updated later if we are on CPython 3.11+


def handle_sigint(signals: int, frametype: FrameType | None) -> Any:
    if old_handler == signal.SIG_IGN:
        return

    contexts = list(ALL_Z3_CONTEXTS)
    for context in contexts:
        context.interrupt()

    if callable(old_handler):
        old_handler(signals, frametype)
    else:
        print("*** CRITICAL ERROR - THIS SHOULD NEVER HAPPEN", sys.stderr, flush=True)
        raise KeyboardInterrupt


if threading.current_thread() == threading.main_thread():  # Signal only works in the main thread
    old_handler = signal.getsignal(signal.SIGINT)
    if old_handler is None or old_handler == signal.SIG_DFL:
        # there is a signal handler installed by someone other than python. we cannot handle this.
        old_handler = True
    else:
        signal.signal(signal.SIGINT, handle_sigint)

try:
    import __pypy__

    _is_pypy = True
except ImportError:
    _is_pypy = False


def z3_expr_to_smt2(f, status="unknown", name="benchmark", logic=""):
    # from https://stackoverflow.com/a/14629021/9719920
    v = (z3.Ast * 0)()
    return z3.Z3_benchmark_to_smtlib_string(f.ctx_ref(), name, logic, status, "", 0, v, f.as_ast())


def claripy_solver_to_smt2(s):
    return s._get_solver().to_smt2()


def _add_memory_pressure(p):
    """
    PyPy's garbage collector is not aware of memory uses happening inside native code. When performing memory-intensive
    tasks in native code, the memory pressure that PyPy observes can greatly deviate from the actual memory pressure.
    We must manually add sufficient memory pressure to account for the "missing" portion.

    This is not a problem for CPython since its GC is based on reference counting.
    """

    global _is_pypy  # pylint:disable=global-variable-not-assigned
    if _is_pypy:
        __pypy__.add_memory_pressure(p)


def int_to_str_unlimited(v: int) -> str:
    """
    Convert an integer to a decimal string, without any size limit.

    :param v: The integer to convert.
    :return: The string.
    """

    if INT_STRING_CHUNK_SIZE is None:
        return str(v)

    if v == 0:
        return "0"

    MOD = 10**INT_STRING_CHUNK_SIZE
    v_str = ""
    if v < 0:
        is_negative = True
        v = -v
    else:
        is_negative = False
    while v > 0:
        v_chunk = str(v % MOD)
        v //= MOD
        if v > 0:
            v_chunk = v_chunk.zfill(INT_STRING_CHUNK_SIZE)
        v_str = v_chunk + v_str
    return v_str if not is_negative else "-" + v_str


def Z3_to_int_str(val):
    # we will monkey-patch Z3 and replace Z3._to_int_str with this version, which is free of integer size limits.

    if isinstance(val, float):
        return str(int(val))
    if isinstance(val, bool):
        return "1" if val else "0"
    return int_to_str_unlimited(val)


if hasattr(sys, "get_int_max_str_digits"):
    # CPython 3.11+
    # monkey-patch Z3 so that it can accept long integers
    z3.z3._to_int_str = Z3_to_int_str
    # update INT_STRING_CHUNK_SIZE
    INT_STRING_CHUNK_SIZE = sys.get_int_max_str_digits()


def str_to_int_unlimited(s: str) -> int:
    """
    Convert a decimal string to an integer, without any size limit.

    :param s: The string to convert.
    :return: The integer.
    """
    if INT_STRING_CHUNK_SIZE is None:
        return int(s)

    if not s:
        return int(s)  # an exception will be raised, which is intentional

    v = 0
    if s[0] == "-":
        is_negative = True
        s = s[1:]
    else:
        is_negative = False

    for i in range(0, len(s), INT_STRING_CHUNK_SIZE):
        start = i
        end = min(i + INT_STRING_CHUNK_SIZE, len(s))
        v *= 10 ** (end - start)
        v += int(s[start:end], 10)
    return v if not is_negative else -v


#
# Some global variables
#

# as of z3 4.8.7.0 this seems to matter
z3.set_param("rewriter.hi_fp_unspecified", "true")

#
# Utility functions
#


def condom(f):
    def z3_condom(*args, **kwargs):
        """
        The Z3 condom intercepts Z3Exceptions and throws a ClaripyZ3Error instead.
        """
        try:
            return f(*args, **kwargs)
        except z3.Z3Exception as ze:
            raise ClaripyZ3Error from ze

    return z3_condom


def _z3_decl_name_str(ctx, decl):
    decl_name = z3.Z3_get_decl_name(ctx, decl)
    return z3.Z3_get_symbol_string_bytes(ctx, decl_name)


def z3_solver_sat(solver, extra_constraints, occasion):
    log.debug("Doing a check! (%s)", occasion)

    result = solver.check(extra_constraints)

    if result == z3.unknown:
        reason = solver.reason_unknown()
        if reason in ("interrupted from keyboard", "interrupted"):
            raise KeyboardInterrupt
        if reason in ("timeout", "max. resource limit exceeded", "max. memory exceeded"):
            raise ClaripySolverInterruptError(reason)
        raise ClaripyZ3Error("solver unknown: " + reason)
    return result == z3.sat


class SmartLRUCache(LRUCache):
    def __init__(self, maxsize, getsizeof=None, evict=None):
        LRUCache.__init__(self, maxsize, getsizeof=getsizeof)
        self._evict = evict

    def popitem(self):
        key, val = LRUCache.popitem(self)
        if self._evict:
            self._evict(key, val)
        return key, val


#
# And the (ugh) magic
#


class BackendZ3(Backend):
    _split_on = ("And", "Or")

    def __init__(self, reuse_z3_solver=None, ast_cache_size=10000):
        Backend.__init__(self, solver_required=True)

        # Per-thread Z3 solver
        # This setting is treated as a global setting and is not supposed to be changed during runtime, unless you know
        # what you are doing.
        if reuse_z3_solver is None:
            reuse_z3_solver = os.environ.get("REUSE_Z3_SOLVER", "False").lower() in {"1", "true", "yes", "y"}
        self.reuse_z3_solver = reuse_z3_solver

        self._ast_cache_size = ast_cache_size

        # and the operations
        all_ops = backend_fp_operations | backend_operations
        all_ops |= backend_strings_operations - {"StrIsDigit"}
        for o in all_ops - {"BVV", "BoolV", "FPV", "FPS", "BitVec", "StringV"}:
            self._op_raw[o] = getattr(self, "_op_raw_" + o)
        self._op_raw["Xor"] = self._op_raw_Xor

        self._op_raw["UGE"] = self._op_raw_UGE
        self._op_raw["UGT"] = self._op_raw_UGT
        self._op_raw["ULE"] = self._op_raw_ULE
        self._op_raw["ULT"] = self._op_raw_ULT

        self._op_raw["Reverse"] = self._op_raw_Reverse
        self._op_raw["fpToSBV"] = self._op_raw_fpToSBV
        self._op_raw["fpToUBV"] = self._op_raw_fpToUBV

        self._op_expr["BVS"] = self.BVS
        self._op_expr["BVV"] = self.BVV
        self._op_expr["FPV"] = self.FPV
        self._op_expr["FPS"] = self.FPS
        self._op_expr["BoolV"] = self.BoolV
        self._op_expr["BoolS"] = self.BoolS
        self._op_expr["StringV"] = self.StringV
        self._op_expr["StringS"] = self.StringS

        self._op_raw["__floordiv__"] = self._op_div
        self._op_raw["__mod__"] = self._op_mod

        # reduceable
        self._op_raw["__add__"] = self._op_add
        self._op_raw["__sub__"] = self._op_sub
        self._op_raw["__mul__"] = self._op_mul
        self._op_raw["__or__"] = self._op_or
        self._op_raw["__xor__"] = self._op_xor
        self._op_raw["__and__"] = self._op_and

        self.solve_count = 0

    # This is necessary to be able to carry over annotations when round-tripping through Z3
    @property
    def bvs_annotations(self) -> dict[bytes, tuple[Annotation, ...]]:
        if not hasattr(self._tls, "bvs_annotations"):
            self._tls.bvs_annotations = {}
        return self._tls.bvs_annotations

    @property
    def _c_uint64_p(self):
        try:
            return self._tls.c_uint64_p
        except AttributeError:
            # a pointer to get values out of Z3
            self._tls.c_uint64_p = ctypes.pointer(ctypes.c_uint64())

            return self._tls.c_uint64_p

    @property
    def _context(self):
        try:
            return self._tls.context
        except AttributeError:
            main_thread = threading.current_thread() == threading.main_thread()
            self._tls.context = z3.Context() if not main_thread else z3.main_ctx()
            ALL_Z3_CONTEXTS.add(self._tls.context)
            return self._tls.context

    @property
    def _boolref_tactics(self):
        try:
            return self._tls.boolref_tactics
        except AttributeError:
            tactics = z3.Then(
                z3.Tactic("simplify", ctx=self._context),
                z3.Tactic("propagate-ineqs", ctx=self._context),
                z3.Tactic("propagate-values", ctx=self._context),
                z3.Tactic("unit-subsume-simplify", ctx=self._context),
                z3.Tactic("aig", ctx=self._context),
                ctx=self._context,
            )
            self._tls.boolref_tactics = tactics
            return self._tls.boolref_tactics

    @property
    def _ast_cache(self):
        try:
            return self._tls.ast_cache
        except AttributeError:
            self._tls.ast_cache = SmartLRUCache(self._ast_cache_size, evict=self._pop_from_ast_cache)
            return self._tls.ast_cache

    @property
    def _var_cache(self):
        try:
            return self._tls.var_cache
        except AttributeError:
            self._tls.var_cache = weakref.WeakValueDictionary()
            return self._tls.var_cache

    @property
    def _sym_cache(self):
        try:
            return self._tls.sym_cache
        except AttributeError:
            self._tls.sym_cache = weakref.WeakValueDictionary()
            return self._tls.sym_cache

    def downsize(self):
        Backend.downsize(self)

        self._ast_cache.clear()
        self._var_cache.clear()
        self._sym_cache.clear()

    def _name(self, o):  # pylint:disable=unused-argument
        log.warning("BackendZ3.name() called. This is weird.")
        raise BackendError("name is not implemented yet")

    def _pop_from_ast_cache(self, _, tpl):
        _, raw_ast = tpl
        z3.Z3_dec_ref(self._context.ctx, raw_ast)

    #
    # Core creation methods
    #

    @condom
    def BVS(self, ast):
        name = ast._encoded_name
        self.bvs_annotations[name] = ast.annotations
        size = ast.size()
        # TODO: Here we can use low level APIs because the check performed by the high level API always results in
        #       the else branch of the check. This evidence although comes from the execution of the angr and claripy
        #       test suite so I'm not sure if this assumption would hold on 100% of the cases
        bv = z3.BitVecSortRef(z3.Z3_mk_bv_sort(self._context.ref(), size), self._context)
        return z3.BitVecRef(
            z3.Z3_mk_const(self._context.ref(), z3.to_symbol(name, self._context), bv.ast), self._context
        )

    @condom
    def BVV(self, ast):
        if ast.args[0] is None:
            raise BackendError("Z3 can't handle empty BVVs")

        size = ast.size()
        # TODO: Here there is no need to use low level API since teh high level API just perform some conversions which
        #       are mandatory to fix the types of the arguments requested by the low level API
        return z3.BitVecVal(ast.args[0], size, ctx=self._context)

    @condom
    def FPS(self, ast):
        sort_z3 = self._convert(ast.args[1])
        # TODO: Here is safe to use low level API since the only condition the hig level API check is if the context is
        #       None and this never happens here
        return z3.FPRef(
            z3.Z3_mk_const(self._context.ref(), z3.to_symbol(ast._encoded_name, self._context), sort_z3.ast),
            self._context,
        )

    @condom
    def FPV(self, ast):
        val = str(ast.args[0])
        sort = self._convert(ast.args[1])
        if val in ("+oo", "+inf", "+Inf", "inf"):
            return z3.FPNumRef(z3.Z3_mk_fpa_inf(sort.ctx_ref(), sort.ast, False), sort.ctx)
        if val in ("-oo", "-inf", "-Inf"):
            return z3.FPNumRef(z3.Z3_mk_fpa_inf(sort.ctx_ref(), sort.ast, True), sort.ctx)
        if val in ("0.0", "+0.0"):
            return z3.FPNumRef(z3.Z3_mk_fpa_zero(sort.ctx_ref(), sort.ast, False), sort.ctx)
        if val == "-0.0":
            return z3.FPNumRef(z3.Z3_mk_fpa_zero(sort.ctx_ref(), sort.ast, True), sort.ctx)
        if val in ("NaN", "nan"):
            return z3.FPNumRef(z3.Z3_mk_fpa_nan(sort.ctx_ref(), sort.ast), sort.ctx)
        better_val = str(Decimal(ast.args[0]))
        return z3.FPNumRef(z3.Z3_mk_numeral(self._context.ref(), better_val, sort.ast), self._context)

    @condom
    def BoolS(self, ast):
        return z3.BoolRef(
            z3.Z3_mk_const(
                self._context.ref(), z3.to_symbol(ast._encoded_name, self._context), z3.BoolSort(self._context).ast
            ),
            self._context,
        )

    @condom
    def BoolV(self, ast):  # pylint:disable=unused-argument
        # TODO: Here the checks performed by the high level API are mandatory before calling the low level API
        #       So we can keep the high level API call here
        return z3.BoolVal(ast.args[0], ctx=self._context)

    @condom
    def StringV(self, ast):
        return z3.StringVal(ast.args[0], ctx=self._context)

    @condom
    def StringS(self, ast):
        return z3.String(ast.args[0], ctx=self._context)

    #
    # Conversions
    #

    @condom
    def _convert(self, obj):  # pylint:disable=arguments-renamed
        if isinstance(obj, FSort):
            return z3.FPSortRef(z3.Z3_mk_fpa_sort(self._context.ref(), obj.exp, obj.mantissa), self._context)
        if isinstance(obj, RM):
            if obj == RM.RM_NearestTiesEven:
                return z3.FPRMRef(z3.Z3_mk_fpa_round_nearest_ties_to_even(self._context.ref()), self._context)
            if obj == RM.RM_NearestTiesAwayFromZero:
                return z3.FPRMRef(z3.Z3_mk_fpa_round_nearest_ties_to_away(self._context.ref()), self._context)
            if obj == RM.RM_TowardsPositiveInf:
                return z3.FPRMRef(z3.Z3_mk_fpa_round_toward_positive(self._context.ref()), self._context)
            if obj == RM.RM_TowardsNegativeInf:
                return z3.FPRMRef(z3.Z3_mk_fpa_round_toward_negative(self._context.ref()), self._context)
            if obj == RM.RM_TowardsZero:
                return z3.FPRMRef(z3.Z3_mk_fpa_round_toward_zero(self._context.ref()), self._context)
            raise BackendError("unrecognized rounding mode")
        if obj is True:
            return z3.BoolRef(z3.Z3_mk_true(self._context.ref()), self._context)
        if obj is False:
            return z3.BoolRef(z3.Z3_mk_false(self._context.ref()), self._context)
        if isinstance(obj, numbers.Number | str) or (hasattr(obj, "__module__") and obj.__module__ in ("z3", "z3.z3")):
            return obj
        log.debug("BackendZ3 encountered unexpected type %s", type(obj))
        raise BackendError(f"unexpected type {type(obj)} encountered in BackendZ3")

    def call(self, *args, **kwargs):  # pylint;disable=arguments-renamed
        return Backend.call(self, *args, **kwargs)

    @condom
    def _abstract(self, e):
        return self._abstract_internal(e.ctx.ctx, e.ast)

    @staticmethod
    def _z3_ast_hash(ast):
        """
        This is a better hashing function for z3 Ast objects. Z3_get_ast_hash() creates too many hash collisions.

        :param ast: A z3 Ast object.
        :return:    An integer - the hash.
        """

        return ast.value

    def _abstract_internal(self, ctx, ast, split_on=None):
        h = self._z3_ast_hash(ast)
        try:
            cached_ast, _ = self._ast_cache[h]
            return cached_ast
        except KeyError:
            pass

        decl = z3.Z3_get_app_decl(ctx, ast)
        decl_num = z3.Z3_get_decl_kind(ctx, decl)
        z3_sort = z3.Z3_get_sort(ctx, ast)

        if decl_num not in z3_op_nums:
            raise ClaripyError(f"unknown decl kind {decl_num}")
        if op_map.get(z3_op_nums[decl_num], None) is None:
            raise ClaripyError(f"unknown decl op {z3_op_nums[decl_num]}")
        op_name = op_map[z3_op_nums[decl_num]]

        num_args = z3.Z3_get_app_num_args(ctx, ast)
        split_on = self._split_on if split_on is None else split_on
        new_split_on = split_on if op_name in split_on else set()
        children = [self._abstract_internal(ctx, z3.Z3_get_app_arg(ctx, ast, i), new_split_on) for i in range(num_args)]

        append_children = True

        if op_name == "True":
            return claripy.true()
        if op_name == "False":
            return claripy.false()
        if op_name.startswith("RM_"):
            return RM(op_name)
        if op_name == "INTERNAL":
            return claripy.StringV(z3.SeqRef(ast).as_string())
        if op_name == "BitVecVal":
            bv_size = z3.Z3_get_bv_sort_size(ctx, z3_sort)
            if z3.Z3_get_numeral_uint64(ctx, ast, self._c_uint64_p):
                return claripy.BVV(self._c_uint64_p.contents.value, bv_size)
            bv_num = str_to_int_unlimited(z3.Z3_get_numeral_string(ctx, ast))
            return claripy.BVV(bv_num, bv_size)
        if op_name in ("FPVal", "MinusZero", "MinusInf", "PlusZero", "PlusInf", "NaN"):
            ebits = z3.Z3_fpa_get_ebits(ctx, z3_sort)
            sbits = z3.Z3_fpa_get_sbits(ctx, z3_sort)
            sort = FSort.from_params(ebits, sbits)
            val = self._abstract_fp_val(ctx, ast, op_name)
            return claripy.FPV(val, sort)
        if op_name == "fpToUBV":
            bv_size = z3.Z3_get_bv_sort_size(ctx, z3_sort)
            return claripy.fpToUBV(*children, bv_size)
        if op_name == "fpToSBV":
            bv_size = z3.Z3_get_bv_sort_size(ctx, z3_sort)
            return claripy.fpToSBV(*children, bv_size)
        if op_name == "SignExt":
            bv_size = z3.Z3_get_decl_int_parameter(ctx, decl, 0)
            return claripy.SignExt(bv_size, children[0])
        if op_name == "ZeroExt":
            bv_size = z3.Z3_get_decl_int_parameter(ctx, decl, 0)
            return claripy.ZeroExt(bv_size, children[0])
        if op_name == "RotateLeft":
            return claripy.RotateLeft(children[0], children[1])
        if op_name == "RotateRight":
            return claripy.RotateRight(children[0], children[1])

        if op_name == "UNINTERPRETED" and num_args == 0:  # symbolic value
            symbol_name = _z3_decl_name_str(ctx, decl)
            symbol_str = symbol_name.decode()
            symbol_ty = z3.Z3_get_sort_kind(ctx, z3_sort)

            if symbol_ty == z3.Z3_BV_SORT:
                bv_size = z3.Z3_get_bv_sort_size(ctx, z3_sort)
                annots = self.bvs_annotations.get(symbol_name, ())
                return claripy.BVS(symbol_str, bv_size, explicit_name=True, annotations=annots)
            if symbol_ty == z3.Z3_BOOL_SORT:
                return claripy.BoolS(symbol_str, explicit_name=True)
            if symbol_ty == z3.Z3_FLOATING_POINT_SORT:
                ebits = z3.Z3_fpa_get_ebits(ctx, z3_sort)
                sbits = z3.Z3_fpa_get_sbits(ctx, z3_sort)
                sort = FSort.from_params(ebits, sbits)
                return claripy.FPS(symbol_str, sort, explicit_name=True)
            if z3.Z3_is_string_sort(ctx, z3_sort):
                raise BackendError("Z3 backend does not support string symbols")
            raise BackendError(f"Unknown z3 term type {symbol_ty}...?")

        if op_name == "UNINTERPRETED":
            mystery_name = z3.Z3_get_symbol_string(ctx, z3.Z3_get_decl_name(ctx, decl))
            log.error("Mystery operation %s in BackendZ3._abstract_internal. Please report this.", mystery_name)
        elif op_name == "Extract":
            hi = z3.Z3_get_decl_int_parameter(ctx, decl, 0)
            lo = z3.Z3_get_decl_int_parameter(ctx, decl, 1)
            args = [hi, lo]
        elif op_name in ("SignExt", "ZeroExt"):
            num = z3.Z3_get_decl_int_parameter(ctx, decl, 0)
            args = [num]
        elif op_name in ("fpToFP", "fpToFPSigned", "fpToFPUnsigned"):
            exp = z3.Z3_fpa_get_ebits(ctx, z3_sort)
            mantissa = z3.Z3_fpa_get_sbits(ctx, z3_sort)
            sort = FSort.from_params(exp, mantissa)
            args = [*children, sort]
            append_children = False
        else:
            args = []

        if append_children:
            args.extend(children)

        # hmm.... honestly not sure what to do here
        result_ty = op_type_map[z3_op_nums[decl_num]]
        ty = type(args[-1])

        if isinstance(result_ty, str):
            err = (
                f"Unknown Z3 error in abstraction (result_ty == '{result_ty}'). "
                "Update your version of Z3, and, if the problem persists, open a claripy issue."
            )
            log.error(err)
            raise BackendError(err)

        if op_name == "If":
            # If is polymorphic and thus must be handled specially
            ty = type(args[1])

            a = ty("If", tuple(args), length=args[1].length)
        elif (
            op := getattr(ty, op_name, None)
            or getattr(result_ty, op_name, None)
            or (op_name in bound_ops and getattr(ty, bound_ops[op_name], None))
        ):
            if op.calc_length is not None:
                length = op.calc_length(*args)
                a = result_ty(op_name, tuple(args), length=length)
            else:
                a = result_ty(op_name, tuple(args))
        else:
            a = result_ty(op_name, tuple(args))

        self._ast_cache[h] = (a, ast)
        z3.Z3_inc_ref(ctx, ast)
        return a

    def _abstract_to_primitive(self, ctx, ast):
        decl = z3.Z3_get_app_decl(ctx, ast)
        decl_num = z3.Z3_get_decl_kind(ctx, decl)

        if decl_num not in z3_op_nums:
            raise ClaripyError(f"unknown decl kind {decl_num}")
        if op_map.get(z3_op_nums[decl_num], None) is None:
            raise ClaripyError(f"unknown decl op {z3_op_nums[decl_num]}")
        op_name = op_map[z3_op_nums[decl_num]]

        if op_name == "BitVecVal":
            return self._abstract_bv_val(ctx, ast)
        if op_name == "True":
            return True
        if op_name == "False":
            return False
        if op_name in ("FPVal", "MinusZero", "MinusInf", "PlusZero", "PlusInf", "NaN"):
            return self._abstract_fp_val(ctx, ast, op_name)
        if op_name == "Concat":
            # Quirk in how z3 might handle NaN encodings - it will not give us a fully evaluated model
            # https://github.com/Z3Prover/z3/issues/518
            # this case will be triggered if the z3 rewriter.hi_fp_unspecified is set to true
            # update 6 jan 2020: idk if this is true anymore
            nargs = z3.Z3_get_app_num_args(ctx, ast)
            res = 0
            for i in range(nargs):
                arg_ast = z3.Z3_get_app_arg(ctx, ast, i)
                arg_decl = z3.Z3_get_app_decl(ctx, arg_ast)
                arg_decl_num = z3.Z3_get_decl_kind(ctx, arg_decl)
                arg_size = z3.Z3_get_bv_sort_size(ctx, z3.Z3_get_sort(ctx, arg_ast))

                neg = False
                if arg_decl_num == z3.Z3_OP_BNEG:
                    arg_ast = z3.Z3_get_app_arg(ctx, arg_ast, 0)
                    arg_decl = z3.Z3_get_app_decl(ctx, arg_ast)
                    arg_decl_num = z3.Z3_get_decl_kind(ctx, arg_decl)
                    neg = True
                if arg_decl_num != z3.Z3_OP_BNUM:
                    raise BackendError("Weird z3 model")

                arg_int = self._abstract_bv_val(ctx, arg_ast)
                if neg:
                    arg_int = (1 << arg_size) - arg_int
                res <<= arg_size
                res |= arg_int
            return res
        if op_name == "fpToIEEEBV":
            # Another quirk in the way z3 might handle nan encodings. see above
            # this case will be triggered if the z3 rewriter.hi_fp_unspecified is set to false
            # update 6 jan 2020: idk if this is true anymore
            arg_ast = z3.Z3_get_app_arg(ctx, ast, 0)
            return self._abstract_fp_encoded_val(ctx, arg_ast)
        if op_name == "INTERNAL":
            seq = z3.SeqRef(ast)
            if seq.is_string():
                return seq.as_string()
        raise BackendError("Unable to abstract Z3 object to primitive")

    def _abstract_bv_val(self, ctx, ast):
        if z3.Z3_get_numeral_uint64(ctx, ast, self._c_uint64_p):
            return self._c_uint64_p.contents.value
        return str_to_int_unlimited(z3.Z3_get_numeral_string(ctx, ast))

    @staticmethod
    def _abstract_fp_val(ctx, ast, op_name):
        if op_name == "FPVal":
            # TODO: do better than this
            fp_mantissa = float(z3.Z3_fpa_get_numeral_significand_string(ctx, ast))
            fp_exp = int(z3.Z3_fpa_get_numeral_exponent_string(ctx, ast, False))
            fp_sign_c = ctypes.c_int()  # pylint: disable=no-value-for-parameter
            z3.Z3_fpa_get_numeral_sign(ctx, ast, ctypes.byref(fp_sign_c))
            fp_sign = -1 if fp_sign_c.value != 0 else 1
            return fp_sign * fp_mantissa * (2**fp_exp)
        if op_name == "MinusZero":
            return -0.0
        if op_name == "MinusInf":
            return float("-inf")
        if op_name == "PlusZero":
            return 0.0
        if op_name == "PlusInf":
            return float("inf")
        if op_name == "NaN":
            return float("nan")
        raise BackendError("Called _abstract_fp_val with unknown type")

    @staticmethod
    def _abstract_fp_encoded_val(ctx, ast):
        decl = z3.Z3_get_app_decl(ctx, ast)
        decl_num = z3.Z3_get_decl_kind(ctx, decl)
        op_name = op_map[z3_op_nums[decl_num]]
        sort = z3.Z3_get_sort(ctx, ast)
        ebits = z3.Z3_fpa_get_ebits(ctx, sort)
        sbits = z3.Z3_fpa_get_sbits(ctx, sort) - 1  # includes sign bit

        if op_name == "FPVal":
            # TODO: do better than this
            fp_mantissa = int(z3.Z3_fpa_get_numeral_significand_string(ctx, ast))
            fp_exp = int(z3.Z3_fpa_get_numeral_exponent_string(ctx, ast, True))
            fp_sign_c = ctypes.c_int()  # pylint: disable=no-value-for-parameter
            z3.Z3_fpa_get_numeral_sign(ctx, ast, ctypes.byref(fp_sign_c))
            fp_sign = 1 if fp_sign_c.value != 0 else 0
        elif op_name == "MinusZero":
            fp_sign = 1
            fp_exp = 0
            fp_mantissa = 0
        elif op_name == "MinusInf":
            fp_sign = 1
            fp_exp = (1 << ebits) - 1
            fp_mantissa = 0
        elif op_name == "PlusZero":
            fp_sign = 0
            fp_exp = 0
            fp_mantissa = 0
        elif op_name == "PlusInf":
            fp_sign = 0
            fp_exp = (1 << ebits) - 1
            fp_mantissa = 0
        elif op_name == "NaN":
            fp_sign = 0
            fp_exp = (1 << ebits) - 1
            fp_mantissa = 1
        else:
            raise BackendError("Called _abstract_fp_val with unknown type")

        return (fp_sign << (ebits + sbits)) | (fp_exp << sbits) | fp_mantissa

    def solver(self, timeout=None, max_memory=None):
        if not self.reuse_z3_solver or getattr(self._tls, "solver", None) is None:
            s = z3.Solver(ctx=self._context)  # , logFile="claripy.smt2")
            if threading.current_thread() != threading.main_thread():
                s.set(ctrl_c=False)
            _add_memory_pressure(1024 * 1024 * 10)
            if self.reuse_z3_solver:
                # Store the Z3 solver to a thread-local storage if the reuse-solver option is enabled
                self._tls.solver = s
        else:
            # Load the existing Z3 solver for this thread
            s = self._tls.solver
            s.reset()

        # Configure timeouts
        if timeout is not None:
            if "soft_timeout" in str(s.param_descrs()):
                s.set("soft_timeout", timeout)
                s.set("solver2_timeout", timeout)
            else:
                s.set("timeout", timeout)
        if max_memory is not None:
            s.set("max_memory", max_memory)
        return s

    def clone_solver(self, s):
        # This clones the solver.
        # See https://github.com/Z3Prover/z3/issues/556
        return s.translate(self._context)

    def _add(self, s, c, track=False):
        if track:
            already_tracked = {str(impl.children()[0]) for impl in s.assertions()}
            for constraint in c:
                name = str(hash(constraint))
                if name not in already_tracked:
                    s.assert_and_track(constraint, name)
                    already_tracked.add(name)
        else:
            s.add(*c)

    def add(self, s, c, track=False):
        converted = self.convert_list(c)
        if track:
            for a, nice_ast in zip(c, converted, strict=False):
                ast = nice_ast.ast
                h = self._z3_ast_hash(ast)
                self._ast_cache[h] = (a, ast)
        return self._add(s, converted, track=track)

    def _unsat_core(self, s):
        cores = s.unsat_core()
        return [impl.children()[1] for impl in s.assertions() if impl.children()[0] in cores]

    @condom
    def _primitive_from_model(self, model, expr):
        v = model.eval(expr, model_completion=True)
        return self._abstract_to_primitive(v.ctx.ctx, v.ast)

    #
    # New, model-driven solves
    #

    def _generic_model(self, z3_model):
        """
        Converts a Z3 model to a name->primitive dict.
        """
        model = {}
        for m_f in z3_model:
            n = _z3_decl_name_str(m_f.ctx.ctx, m_f.ast).decode()
            m = m_f()
            me = z3_model.eval(m, model_completion=True)
            model[n] = self._abstract_to_primitive(me.ctx.ctx, me.ast)

        return model

    def _satisfiable(self, extra_constraints=(), solver=None, model_callback=None):
        self.solve_count += 1

        log.debug("Doing a check! (satisfiable)")
        if not z3_solver_sat(solver, extra_constraints, "satisfiable"):
            return False

        if model_callback is not None:
            model_callback(self._generic_model(solver.model()))
        return True

    def _eval(self, expr, n, extra_constraints=(), solver=None, model_callback=None):
        results = self._batch_eval(
            [expr], n, extra_constraints=extra_constraints, solver=solver, model_callback=model_callback
        )

        # Unpack it
        return [v[0] for v in results]

    @condom
    def _batch_eval(self, exprs, n, extra_constraints=(), solver=None, model_callback=None):
        result_values = []

        if n > 1:
            solver.push()

        for i in range(n):
            self.solve_count += 1
            if not z3_solver_sat(solver, extra_constraints, "batch_eval"):
                break
            model = solver.model()

            # construct results
            r = []
            for expr in exprs:
                if not isinstance(expr, numbers.Number | str | bool):
                    v = self._primitive_from_model(model, expr)
                    r.append(v)
                else:
                    r.append(expr)

            # Append the solution to the result list
            if model_callback is not None:
                model_callback(self._generic_model(solver.model()))
            result_values.append(tuple(r))

            # Construct the extra constraint so we don't get the same result anymore
            if i + 1 != n:
                if len(exprs) == 1:
                    solver.add(exprs[0] != r[0])
                else:
                    solver.add(
                        self._op_raw_Not(self._op_raw_And(*[(ex == ex_v) for ex, ex_v in zip(exprs, r, strict=False)]))
                    )
                model = None

        if n > 1:
            solver.pop()

        return result_values

    def _extrema(self, is_max: bool, expr, extra_constraints, signed, solver, model_callback):
        """
        _max if is_max else _min
        """
        lo = -(2 ** (expr.size() - 1)) if signed else 0
        hi = 2 ** (expr.size() - 1) - 1 if signed else 2 ** expr.size() - 1

        constraints = [self.convert(e) for e in extra_constraints]
        comment = "max" if is_max else "min"

        GE = operator.ge if signed else z3.UGE
        LE = operator.le if signed else z3.ULE

        # TODO: Can only deal with bitvectors, not floats
        while hi - lo > 1:
            middle = (lo + hi) // 2
            # l.debug("h/m/l/d: %d %d %d %d", hi, middle, lo, hi-lo)

            # TODO: is this assumption correct?
            # here it's not safe to call directly the z3 low level API since it might happen that the argument is an
            # integer and not a BV
            constraints.append(
                z3.And(GE(expr, middle), LE(expr, hi)) if is_max else z3.And(GE(expr, lo), LE(expr, middle))
            )

            self.solve_count += 1
            sat = z3_solver_sat(solver, constraints, comment)
            constraints.pop()
            if sat:
                log.debug("... still sat")
                if model_callback is not None:
                    model_callback(self._generic_model(solver.model()))
            else:
                log.debug("... now unsat")
            if sat == is_max:
                lo = middle
            else:
                hi = middle

        constraints.append(expr == (hi if is_max else lo))
        sat = z3_solver_sat(solver, constraints, comment)
        if sat and model_callback is not None:
            model_callback(self._generic_model(solver.model()))
        return hi if sat == is_max else lo

    @condom
    def _min(self, expr, extra_constraints=(), signed=False, solver=None, model_callback=None):
        return self._extrema(False, expr, extra_constraints, signed, solver, model_callback)

    @condom
    def _max(self, expr, extra_constraints=(), signed=False, solver=None, model_callback=None):
        return self._extrema(True, expr, extra_constraints, signed, solver, model_callback)

    @condom
    def simplify(self, expr):
        expr_raw = self.convert(expr)

        if isinstance(expr_raw, z3.BoolRef):
            boolref_tactics = self._boolref_tactics
            simplified = boolref_tactics(expr_raw).as_expr()
        else:
            simplified = z3.simplify(expr_raw)

        return self._abstract(simplified)

    def _is_false(self, e, extra_constraints=(), solver=None, model_callback=None):
        return z3.simplify(e).eq(z3.BoolVal(False, ctx=self._context))

    def _is_true(self, e, extra_constraints=(), solver=None, model_callback=None):
        return z3.simplify(e).eq(z3.BoolVal(True, ctx=self._context))

    def _solution(self, expr, v, extra_constraints=(), solver=None, model_callback=None):
        return self._satisfiable(
            extra_constraints=(expr == v, *tuple(extra_constraints)), solver=solver, model_callback=model_callback
        )

    #
    # Some Z3 passthroughs
    #

    # these require the context or special treatment

    @staticmethod
    def _op_div(a, b):
        return z3.BitVecRef(z3.Z3_mk_bvudiv(a.ctx_ref(), a.as_ast(), b.as_ast()), a.ctx)

    @staticmethod
    def _op_mod(a, b):
        return z3.BitVecRef(z3.Z3_mk_bvurem(a.ctx_ref(), a.as_ast(), b.as_ast()), a.ctx)

    @staticmethod
    def _op_add(*args):
        return reduce(operator.__add__, args)

    @staticmethod
    def _op_sub(*args):
        return reduce(operator.__sub__, args)

    @staticmethod
    def _op_mul(*args):
        return reduce(operator.__mul__, args)

    @staticmethod
    def _op_or(*args):
        return reduce(operator.__or__, args)

    @staticmethod
    def _op_xor(*args):
        return reduce(operator.__xor__, args)

    @staticmethod
    def _op_and(*args):
        return reduce(operator.__and__, args)

    def _op_raw_And(self, *args):
        # copied from z3._to_ast_array
        sz = len(args)
        _args = (z3.Ast * sz)()
        for i in range(sz):
            _args[i] = args[i].as_ast()
        return z3.BoolRef(z3.Z3_mk_and(self._context.ref(), sz, _args), self._context)

    def _op_raw_Xor(self, *args):
        return z3.BoolRef(z3.Z3_mk_xor(self._context.ref(), *(arg.as_ast() for arg in args)), self._context)

    def _op_raw_Or(self, *args):
        # copied from z3._to_ast_array
        sz = len(args)
        _args = (z3.Ast * sz)()
        for i in range(sz):
            _args[i] = args[i].as_ast()
        return z3.BoolRef(z3.Z3_mk_or(self._context.ref(), sz, _args), self._context)

    def _op_raw_Not(self, a):
        return z3.BoolRef(z3.Z3_mk_not(self._context.ref(), a.as_ast()), self._context)

    def _op_raw_If(self, i, t, e):
        # partially copied from z3._to_expr_ref
        ctx_ref = self._context.ref()
        ast = z3.Z3_mk_ite(ctx_ref, i.as_ast(), t.as_ast(), e.as_ast())
        k = z3.Z3_get_ast_kind(ctx_ref, ast)
        sk = z3.Z3_get_sort_kind(ctx_ref, z3.Z3_get_sort(ctx_ref, ast))
        if sk == z3.Z3_BOOL_SORT:
            return z3.BoolRef(ast, self._context)
        if sk == z3.Z3_BV_SORT:
            if k == z3.Z3_NUMERAL_AST:
                return z3.BitVecNumRef(ast, self._context)
            return z3.BitVecRef(ast, self._context)
        if sk == z3.Z3_FLOATING_POINT_SORT:
            if k == z3.Z3_APP_AST and z3.Z3_is_numeral_ast(ctx_ref, ast):
                return z3.FPNumRef(ast, self._context)
            return z3.FPRef(ast, self._context)
        return z3.ExprRef(ast, self._context)

    @condom
    def _op_raw_fpAbs(self, a):
        return z3.FPRef(z3.Z3_mk_fpa_abs(self._context.ref(), a.as_ast()), self._context)

    @condom
    def _op_raw_fpNeg(self, a):
        return z3.FPRef(z3.Z3_mk_fpa_neg(self._context.ref(), a.as_ast()), self._context)

    @condom
    def _op_raw_fpAdd(self, rm, a, b):
        return z3.FPRef(z3.Z3_mk_fpa_add(self._context.ref(), rm.as_ast(), a.as_ast(), b.as_ast()), self._context)

    @condom
    def _op_raw_fpSub(self, rm, a, b):
        return z3.FPRef(z3.Z3_mk_fpa_sub(self._context.ref(), rm.as_ast(), a.as_ast(), b.as_ast()), self._context)

    @condom
    def _op_raw_fpMul(self, rm, a, b):
        return z3.FPRef(z3.Z3_mk_fpa_mul(self._context.ref(), rm.as_ast(), a.as_ast(), b.as_ast()), self._context)

    @condom
    def _op_raw_fpDiv(self, rm, a, b):
        return z3.FPRef(z3.Z3_mk_fpa_div(self._context.ref(), rm.as_ast(), a.as_ast(), b.as_ast()), self._context)

    @condom
    def _op_raw_fpSqrt(self, rm, a):
        return z3.FPRef(z3.Z3_mk_fpa_sqrt(self._context.ref(), rm.as_ast(), a.as_ast()), self._context)

    @condom
    def _op_raw_fpLT(self, a, b):
        return z3.BoolRef(z3.Z3_mk_fpa_lt(self._context.ref(), a.as_ast(), b.as_ast()), self._context)

    @condom
    def _op_raw_fpLEQ(self, a, b):
        return z3.BoolRef(z3.Z3_mk_fpa_leq(self._context.ref(), a.as_ast(), b.as_ast()), self._context)

    @condom
    def _op_raw_fpGT(self, a, b):
        return z3.BoolRef(z3.Z3_mk_fpa_gt(self._context.ref(), a.as_ast(), b.as_ast()), self._context)

    @condom
    def _op_raw_fpGEQ(self, a, b):
        return z3.BoolRef(z3.Z3_mk_fpa_geq(self._context.ref(), a.as_ast(), b.as_ast()), self._context)

    @condom
    def _op_raw_fpEQ(self, a, b):
        return z3.BoolRef(z3.Z3_mk_fpa_eq(self._context.ref(), a.as_ast(), b.as_ast()), self._context)

    @condom
    def _op_raw_fpNEQ(self, a, b):
        return z3.Not(z3.BoolRef(z3.Z3_mk_fpa_eq(self._context.ref(), a.as_ast(), b.as_ast()), self._context))

    @condom
    def _op_raw_fpIsNaN(self, a):
        return z3.BoolRef(z3.Z3_mk_fpa_is_nan(self._context.ref(), a.as_ast()), self._context)

    @condom
    def _op_raw_fpIsInf(self, a):
        return z3.BoolRef(z3.Z3_mk_fpa_is_infinite(self._context.ref(), a.as_ast()), self._context)

    @condom
    def _op_raw_fpFP(self, sgn, exp, sig):
        return z3.FPRef(z3.Z3_mk_fpa_fp(self._context.ref(), sgn.ast, exp.ast, sig.ast), self._context)

    @condom
    def _op_raw_fpToSBV(self, rm, fp, bv_len):
        return z3.BitVecRef(z3.Z3_mk_fpa_to_sbv(self._context.ref(), rm.ast, fp.ast, bv_len), self._context)

    @condom
    def _op_raw_fpToUBV(self, rm, fp, bv_len):
        return z3.BitVecRef(z3.Z3_mk_fpa_to_ubv(self._context.ref(), rm.ast, fp.ast, bv_len), self._context)

    @condom
    def _op_raw_fpToFP(self, a1, a2=None, a3=None):
        # TODO: lots of mandatory checks are performed by the high level API here. we shouldn't use low level APIs here
        return z3.fpToFP(a1, a2=a2, a3=a3, ctx=self._context)

    @condom
    def _op_raw_fpToFPUnsigned(self, a1, a2, a3):
        # as above
        return z3.fpToFPUnsigned(a1, a2, a3, ctx=self._context)

    @condom
    def _op_raw_fpToIEEEBV(self, x):
        return z3.BitVecRef(z3.Z3_mk_fpa_to_ieee_bv(self._context.ref(), x.ast), self._context)

    # and these do not
    @staticmethod
    @condom
    def _op_raw_Concat(*args):
        sz = len(args)
        ctx = args[0].ctx
        # TODO: I don't think this is needed for us, we don't deal with Seq or Regex
        # if z3.is_seq(args[0]) or isinstance(args[0], str):
        #     v = (z3.Ast * sz)()
        #     for i in range(sz):
        #         v[i] = args[i].as_ast()
        #     return z3.SeqRef(z3.Z3_mk_seq_concat(ctx.ref(), sz, v), ctx)
        #
        # if z3.is_re(args[0]):
        #     v = (z3.Ast * sz)()
        #     for i in range(sz):
        #         v[i] = args[i].as_ast()
        #     return z3.ReRef(z3.Z3_mk_re_concat(ctx.ref(), sz, v), ctx)

        r = args[0]
        for i in range(sz - 1):
            r = z3.BitVecRef(z3.Z3_mk_concat(ctx.ref(), r.as_ast(), args[i + 1].as_ast()), ctx)
        return r

    @staticmethod
    @condom
    def _op_raw_Extract(high, low, a):
        # TODO: I don't think this is needed for us, we don't deal with Seq or Regex
        # if isinstance(high, str):
        #     high = z3.StringVal(high)
        # if z3.is_seq(high):
        #     s = high
        #     offset, length = _coerce_exprs(low, a, s.ctx)
        #     return z3.SeqRef(z3.Z3_mk_seq_extract(s.ctx_ref(), s.as_ast(), offset.as_ast(), length.as_ast()), s.ctx)
        return z3.BitVecRef(z3.Z3_mk_extract(a.ctx_ref(), high, low, a.as_ast()), a.ctx)

    @staticmethod
    @condom
    def _op_raw_LShR(a, b):
        return z3.BitVecRef(z3.Z3_mk_bvlshr(a.ctx_ref(), a.as_ast(), b.as_ast()), a.ctx)

    @staticmethod
    @condom
    def _op_raw_RotateLeft(a, b):
        return z3.BitVecRef(z3.Z3_mk_ext_rotate_left(a.ctx_ref(), a.as_ast(), b.as_ast()), a.ctx)

    @staticmethod
    @condom
    def _op_raw_RotateRight(a, b):
        return z3.BitVecRef(z3.Z3_mk_ext_rotate_right(a.ctx_ref(), a.as_ast(), b.as_ast()), a.ctx)

    @staticmethod
    @condom
    def _op_raw_SignExt(n, a):
        return z3.BitVecRef(z3.Z3_mk_sign_ext(a.ctx_ref(), n, a.as_ast()), a.ctx)

    @staticmethod
    @condom
    def _op_raw_UGE(a, b):
        return z3.BoolRef(z3.Z3_mk_bvuge(a.ctx_ref(), a.as_ast(), b.as_ast()), a.ctx)

    @staticmethod
    @condom
    def _op_raw_UGT(a, b):
        return z3.BoolRef(z3.Z3_mk_bvugt(a.ctx_ref(), a.as_ast(), b.as_ast()), a.ctx)

    @staticmethod
    @condom
    def _op_raw_ULE(a, b):
        return z3.BoolRef(z3.Z3_mk_bvule(a.ctx_ref(), a.as_ast(), b.as_ast()), a.ctx)

    @staticmethod
    @condom
    def _op_raw_ULT(a, b):
        return z3.BoolRef(z3.Z3_mk_bvult(a.ctx_ref(), a.as_ast(), b.as_ast()), a.ctx)

    @staticmethod
    @condom
    def _op_raw_ZeroExt(n, a):
        return z3.BitVecRef(z3.Z3_mk_zero_ext(a.ctx_ref(), n, a.as_ast()), a.ctx)

    @staticmethod
    @condom
    def _op_raw_SMod(a, b):
        return z3.BitVecRef(z3.Z3_mk_bvsrem(a.ctx_ref(), a.as_ast(), b.as_ast()), a.ctx)

    @staticmethod
    @condom
    def _op_raw_Reverse(a):
        if a.size() == 8:
            return a
        if a.size() % 8 != 0:
            raise ClaripyOperationError("can't reverse non-byte sized bitvectors")
        return BackendZ3._op_raw_Concat(*[BackendZ3._op_raw_Extract(i + 7, i, a) for i in range(0, a.size(), 8)])

    @staticmethod
    @condom
    def _op_raw_SLT(a, b):
        return a < b

    @staticmethod
    @condom
    def _op_raw_SLE(a, b):
        return a <= b

    @staticmethod
    @condom
    def _op_raw_SGT(a, b):
        return a > b

    @staticmethod
    @condom
    def _op_raw_SGE(a, b):
        return a >= b

    @staticmethod
    @condom
    def _op_raw_SDiv(a, b):
        return a / b

    def _identical(self, a, b):
        return a.eq(b)

    # String operations:

    @staticmethod
    @condom
    def _op_raw_StrConcat(*args):
        return z3.Concat(*args)

    @staticmethod
    @condom
    def _op_raw_StrSubstr(start_idx, count, initial_string):
        return z3.SubString(initial_string, z3.BV2Int(start_idx), z3.BV2Int(count))

    @staticmethod
    @condom
    def _op_raw_StrLen(input_string):
        return z3.Int2BV(z3.Length(input_string), 64)

    @staticmethod
    @condom
    def _op_raw_StrReplace(input_string, target, replacement):
        return z3.Replace(input_string, target, replacement)

    @staticmethod
    @condom
    def _op_raw_StrContains(input_string, substring):
        return z3.Contains(input_string, substring)

    @staticmethod
    @condom
    def _op_raw_StrPrefixOf(prefix, input_string):
        return z3.PrefixOf(prefix, input_string)

    @staticmethod
    @condom
    def _op_raw_StrSuffixOf(suffix, input_string):
        return z3.SuffixOf(suffix, input_string)

    @staticmethod
    @condom
    def _op_raw_StrIndexOf(string, pattern, start_idx):
        return z3.Int2BV(z3.IndexOf(string, pattern, z3.BV2Int(start_idx)), 64)

    @staticmethod
    @condom
    def _op_raw_StrToInt(input_string):
        return z3.Int2BV(z3.StrToInt(input_string), 64)

    @staticmethod
    @condom
    def _op_raw_IntToStr(input_bv):
        return z3.IntToStr(z3.BV2Int(input_bv))


#
# this is for the actual->abstract conversion above
#

# these are Z3 operation names for abstraction
z3_op_names = [_ for _ in dir(z3) if _.startswith("Z3_OP")]
z3_op_nums = {getattr(z3, _): _ for _ in z3_op_names}

op_map = {
    "Z3_OP_ADD": "__add__",
    "Z3_OP_AGNUM": None,
    "Z3_OP_AND": "And",
    "Z3_OP_ANUM": None,
    "Z3_OP_ARRAY_DEFAULT": None,
    "Z3_OP_ARRAY_EXT": None,
    "Z3_OP_ARRAY_MAP": None,
    "Z3_OP_AS_ARRAY": None,
    "Z3_OP_BADD": "__add__",
    "Z3_OP_BAND": "__and__",
    "Z3_OP_BASHR": "__rshift__",
    "Z3_OP_BCOMP": None,
    "Z3_OP_BIT0": None,
    "Z3_OP_BIT1": None,
    "Z3_OP_BIT2BOOL": None,
    "Z3_OP_BLSHR": "LShR",
    "Z3_OP_BMUL": "__mul__",
    "Z3_OP_BNAND": None,
    "Z3_OP_BNEG": "__neg__",
    "Z3_OP_BNOR": None,
    "Z3_OP_BNOT": "__invert__",
    "Z3_OP_BNUM": "BitVecVal",
    "Z3_OP_BOR": "__or__",
    "Z3_OP_BREDAND": None,
    "Z3_OP_BREDOR": None,
    "Z3_OP_BSDIV": "SDiv",
    "Z3_OP_BSDIV0": None,
    "Z3_OP_BSDIV_I": "SDiv",
    "Z3_OP_BSHL": "__lshift__",
    "Z3_OP_BSMOD": "SMod",
    "Z3_OP_BSMOD0": None,
    "Z3_OP_BSMOD_I": "SMod",
    "Z3_OP_BSMUL_NO_OVFL": None,
    "Z3_OP_BSMUL_NO_UDFL": None,
    "Z3_OP_BSREM": "SMod",
    "Z3_OP_BSREM0": None,
    "Z3_OP_BSREM_I": "SMod",
    "Z3_OP_BSUB": "__sub__",
    "Z3_OP_BUDIV": "__floordiv__",
    "Z3_OP_BUDIV0": None,
    "Z3_OP_BUDIV_I": "__floordiv__",
    "Z3_OP_BUMUL_NO_OVFL": None,
    "Z3_OP_BUREM": "__mod__",
    "Z3_OP_BUREM0": None,
    "Z3_OP_BUREM_I": "__mod__",
    "Z3_OP_BV2INT": None,
    "Z3_OP_BXNOR": None,
    "Z3_OP_BXOR": "__xor__",
    "Z3_OP_CARRY": None,
    "Z3_OP_CHAR_CONST": None,
    "Z3_OP_CHAR_FROM_BV": None,
    "Z3_OP_CHAR_IS_DIGIT": None,
    "Z3_OP_CHAR_LE": None,
    "Z3_OP_CHAR_TO_BV": None,
    "Z3_OP_CHAR_TO_INT": None,
    "Z3_OP_CONCAT": "Concat",
    "Z3_OP_CONST_ARRAY": None,
    "Z3_OP_DISTINCT": "__ne__",
    "Z3_OP_DIV": "SDiv",
    "Z3_OP_DT_ACCESSOR": None,
    "Z3_OP_DT_CONSTRUCTOR": None,
    "Z3_OP_DT_IS": None,
    "Z3_OP_DT_RECOGNISER": None,
    "Z3_OP_DT_UPDATE_FIELD": None,
    "Z3_OP_EQ": "__eq__",
    "Z3_OP_EXTRACT": "Extract",
    "Z3_OP_EXT_ROTATE_LEFT": "RotateLeft",
    "Z3_OP_EXT_ROTATE_RIGHT": "RotateRight",
    "Z3_OP_FALSE": "False",
    "Z3_OP_FD_CONSTANT": None,
    "Z3_OP_FD_LT": None,
    "Z3_OP_FPA_ABS": "fpAbs",
    "Z3_OP_FPA_ADD": "fpAdd",
    "Z3_OP_FPA_BV2RM": None,
    "Z3_OP_FPA_BVWRAP": None,
    "Z3_OP_FPA_DIV": "fpDiv",
    "Z3_OP_FPA_EQ": "fpEQ",
    "Z3_OP_FPA_FMA": None,
    "Z3_OP_FPA_FP": None,
    "Z3_OP_FPA_GE": "fpGEQ",
    "Z3_OP_FPA_GT": "fpGT",
    "Z3_OP_FPA_IS_INF": None,
    "Z3_OP_FPA_IS_NAN": None,
    "Z3_OP_FPA_IS_NEGATIVE": None,
    "Z3_OP_FPA_IS_NORMAL": None,
    "Z3_OP_FPA_IS_POSITIVE": None,
    "Z3_OP_FPA_IS_SUBNORMAL": None,
    "Z3_OP_FPA_IS_ZERO": None,
    "Z3_OP_FPA_LE": "fpLEQ",
    "Z3_OP_FPA_LT": "fpLT",
    "Z3_OP_FPA_MAX": None,
    "Z3_OP_FPA_MIN": None,
    "Z3_OP_FPA_MINUS_INF": "MinusInf",
    "Z3_OP_FPA_MINUS_ZERO": "MinusZero",
    "Z3_OP_FPA_MUL": "fpMul",
    "Z3_OP_FPA_NAN": "NaN",
    "Z3_OP_FPA_NEG": "fpNeg",
    "Z3_OP_FPA_NUM": "FPVal",
    "Z3_OP_FPA_PLUS_INF": "PlusInf",
    "Z3_OP_FPA_PLUS_ZERO": "PlusZero",
    "Z3_OP_FPA_REM": None,
    "Z3_OP_FPA_RM_NEAREST_TIES_TO_AWAY": "RM_RNA",
    "Z3_OP_FPA_RM_NEAREST_TIES_TO_EVEN": "RM_RNE",
    "Z3_OP_FPA_RM_TOWARD_NEGATIVE": "RM_RTN",
    "Z3_OP_FPA_RM_TOWARD_POSITIVE": "RM_RTP",
    "Z3_OP_FPA_RM_TOWARD_ZERO": "RM_RTZ",
    "Z3_OP_FPA_ROUND_TO_INTEGRAL": None,
    "Z3_OP_FPA_SQRT": "fpSqrt",
    "Z3_OP_FPA_SUB": "fpSub",
    "Z3_OP_FPA_TO_FP": "fpToFP",
    "Z3_OP_FPA_TO_FP_UNSIGNED": "fpToFPUnsigned",
    "Z3_OP_FPA_TO_IEEE_BV": "fpToIEEEBV",
    "Z3_OP_FPA_TO_REAL": None,
    "Z3_OP_FPA_TO_SBV": "fpToSBV",
    "Z3_OP_FPA_TO_UBV": "fpToUBV",
    "Z3_OP_GE": "SGE",
    "Z3_OP_GT": "SGT",
    "Z3_OP_IDIV": "SDiv",
    "Z3_OP_IFF": "__eq__",
    "Z3_OP_IMPLIES": None,
    "Z3_OP_INT2BV": None,
    "Z3_OP_INTERNAL": "INTERNAL",
    "Z3_OP_INT_TO_STR": None,
    "Z3_OP_IS_INT": None,
    "Z3_OP_ITE": "If",
    "Z3_OP_LABEL": None,
    "Z3_OP_LABEL_LIT": None,
    "Z3_OP_LE": "SLE",
    "Z3_OP_LT": "SLT",
    "Z3_OP_MOD": "__mod__",
    "Z3_OP_MUL": "__mul__",
    "Z3_OP_NOT": "Not",
    "Z3_OP_OEQ": None,
    "Z3_OP_OR": "Or",
    "Z3_OP_PB_AT_LEAST": None,
    "Z3_OP_PB_AT_MOST": None,
    "Z3_OP_PB_EQ": None,
    "Z3_OP_PB_GE": None,
    "Z3_OP_PB_LE": None,
    "Z3_OP_POWER": None,
    "Z3_OP_PR_AND_ELIM": None,
    "Z3_OP_PR_APPLY_DEF": None,
    "Z3_OP_PR_ASSERTED": None,
    "Z3_OP_PR_ASSUMPTION_ADD": None,
    "Z3_OP_PR_BIND": None,
    "Z3_OP_PR_CLAUSE_TRAIL": None,
    "Z3_OP_PR_COMMUTATIVITY": None,
    "Z3_OP_PR_DEF_AXIOM": None,
    "Z3_OP_PR_DEF_INTRO": None,
    "Z3_OP_PR_DER": None,
    "Z3_OP_PR_DISTRIBUTIVITY": None,
    "Z3_OP_PR_ELIM_UNUSED_VARS": None,
    "Z3_OP_PR_GOAL": None,
    "Z3_OP_PR_HYPER_RESOLVE": None,
    "Z3_OP_PR_HYPOTHESIS": None,
    "Z3_OP_PR_IFF_FALSE": None,
    "Z3_OP_PR_IFF_OEQ": None,
    "Z3_OP_PR_IFF_TRUE": None,
    "Z3_OP_PR_LEMMA": None,
    "Z3_OP_PR_LEMMA_ADD": None,
    "Z3_OP_PR_MODUS_PONENS": None,
    "Z3_OP_PR_MODUS_PONENS_OEQ": None,
    "Z3_OP_PR_MONOTONICITY": None,
    "Z3_OP_PR_NNF_NEG": None,
    "Z3_OP_PR_NNF_POS": None,
    "Z3_OP_PR_NOT_OR_ELIM": None,
    "Z3_OP_PR_PULL_QUANT": None,
    "Z3_OP_PR_PUSH_QUANT": None,
    "Z3_OP_PR_QUANT_INST": None,
    "Z3_OP_PR_QUANT_INTRO": None,
    "Z3_OP_PR_REDUNDANT_DEL": None,
    "Z3_OP_PR_REFLEXIVITY": None,
    "Z3_OP_PR_REWRITE": None,
    "Z3_OP_PR_REWRITE_STAR": None,
    "Z3_OP_PR_SKOLEMIZE": None,
    "Z3_OP_PR_SYMMETRY": None,
    "Z3_OP_PR_TH_LEMMA": None,
    "Z3_OP_PR_TRANSITIVITY": None,
    "Z3_OP_PR_TRANSITIVITY_STAR": None,
    "Z3_OP_PR_TRUE": None,
    "Z3_OP_PR_UNDEF": None,
    "Z3_OP_PR_UNIT_RESOLUTION": None,
    "Z3_OP_RA_CLONE": None,
    "Z3_OP_RA_COMPLEMENT": None,
    "Z3_OP_RA_EMPTY": None,
    "Z3_OP_RA_FILTER": None,
    "Z3_OP_RA_IS_EMPTY": None,
    "Z3_OP_RA_JOIN": None,
    "Z3_OP_RA_NEGATION_FILTER": None,
    "Z3_OP_RA_PROJECT": None,
    "Z3_OP_RA_RENAME": None,
    "Z3_OP_RA_SELECT": None,
    "Z3_OP_RA_STORE": None,
    "Z3_OP_RA_UNION": None,
    "Z3_OP_RA_WIDEN": None,
    "Z3_OP_RECURSIVE": None,
    "Z3_OP_REM": "__mod__",
    "Z3_OP_REPEAT": "RepeatBitVec",
    "Z3_OP_RE_COMPLEMENT": None,
    "Z3_OP_RE_CONCAT": None,
    "Z3_OP_RE_DERIVATIVE": None,
    "Z3_OP_RE_DIFF": None,
    "Z3_OP_RE_EMPTY_SET": None,
    "Z3_OP_RE_FULL_CHAR_SET": None,
    "Z3_OP_RE_FULL_SET": None,
    "Z3_OP_RE_INTERSECT": None,
    "Z3_OP_RE_LOOP": None,
    "Z3_OP_RE_OF_PRED": None,
    "Z3_OP_RE_OPTION": None,
    "Z3_OP_RE_PLUS": None,
    "Z3_OP_RE_POWER": None,
    "Z3_OP_RE_RANGE": None,
    "Z3_OP_RE_REVERSE": None,
    "Z3_OP_RE_STAR": None,
    "Z3_OP_RE_UNION": None,
    "Z3_OP_ROTATE_LEFT": None,
    "Z3_OP_ROTATE_RIGHT": None,
    "Z3_OP_SBV_TO_STR": None,
    "Z3_OP_SELECT": None,
    "Z3_OP_SEQ_AT": None,
    "Z3_OP_SEQ_CONCAT": None,
    "Z3_OP_SEQ_CONTAINS": None,
    "Z3_OP_SEQ_EMPTY": None,
    "Z3_OP_SEQ_EXTRACT": None,
    "Z3_OP_SEQ_INDEX": None,
    "Z3_OP_SEQ_IN_RE": None,
    "Z3_OP_SEQ_LAST_INDEX": None,
    "Z3_OP_SEQ_LENGTH": None,
    "Z3_OP_SEQ_NTH": None,
    "Z3_OP_SEQ_PREFIX": None,
    "Z3_OP_SEQ_REPLACE": None,
    "Z3_OP_SEQ_REPLACE_ALL": None,
    "Z3_OP_SEQ_REPLACE_RE": None,
    "Z3_OP_SEQ_REPLACE_RE_ALL": None,
    "Z3_OP_SEQ_SUFFIX": None,
    "Z3_OP_SEQ_TO_RE": None,
    "Z3_OP_SEQ_UNIT": None,
    "Z3_OP_SET_CARD": None,
    "Z3_OP_SET_COMPLEMENT": None,
    "Z3_OP_SET_DIFFERENCE": None,
    "Z3_OP_SET_HAS_SIZE": None,
    "Z3_OP_SET_INTERSECT": None,
    "Z3_OP_SET_SUBSET": None,
    "Z3_OP_SET_UNION": None,
    "Z3_OP_SGEQ": "SGE",
    "Z3_OP_SGT": "SGT",
    "Z3_OP_SIGN_EXT": "SignExt",
    "Z3_OP_SLEQ": "SLE",
    "Z3_OP_SLT": "SLT",
    "Z3_OP_SPECIAL_RELATION_LO": None,
    "Z3_OP_SPECIAL_RELATION_PLO": None,
    "Z3_OP_SPECIAL_RELATION_PO": None,
    "Z3_OP_SPECIAL_RELATION_TC": None,
    "Z3_OP_SPECIAL_RELATION_TO": None,
    "Z3_OP_SPECIAL_RELATION_TRC": None,
    "Z3_OP_STORE": None,
    "Z3_OP_STRING_LE": None,
    "Z3_OP_STRING_LT": None,
    "Z3_OP_STR_FROM_CODE": None,
    "Z3_OP_STR_TO_CODE": None,
    "Z3_OP_STR_TO_INT": None,
    "Z3_OP_SUB": "__sub__",
    "Z3_OP_TO_INT": None,
    "Z3_OP_TO_REAL": None,
    "Z3_OP_TRUE": "True",
    "Z3_OP_UBV_TO_STR": None,
    "Z3_OP_UGEQ": "UGE",
    "Z3_OP_UGT": "UGT",
    "Z3_OP_ULEQ": "ULE",
    "Z3_OP_ULT": "ULT",
    "Z3_OP_UMINUS": "__neg__",
    "Z3_OP_UNINTERPRETED": "UNINTERPRETED",
    "Z3_OP_XOR": "Xor",
    "Z3_OP_XOR3": None,
    "Z3_OP_ZERO_EXT": "ZeroExt",
}

op_type_map = {
    "Z3_OP_ADD": None,
    "Z3_OP_AGNUM": None,
    "Z3_OP_AND": Bool,
    "Z3_OP_ANUM": None,
    "Z3_OP_ARRAY_DEFAULT": None,
    "Z3_OP_ARRAY_EXT": None,
    "Z3_OP_ARRAY_MAP": None,
    "Z3_OP_AS_ARRAY": None,
    "Z3_OP_BADD": BV,
    "Z3_OP_BAND": BV,
    "Z3_OP_BASHR": BV,
    "Z3_OP_BCOMP": None,
    "Z3_OP_BIT0": None,
    "Z3_OP_BIT1": None,
    "Z3_OP_BIT2BOOL": None,
    "Z3_OP_BLSHR": BV,
    "Z3_OP_BMUL": BV,
    "Z3_OP_BNAND": None,
    "Z3_OP_BNEG": BV,
    "Z3_OP_BNOR": None,
    "Z3_OP_BNOT": BV,
    "Z3_OP_BNUM": "BitVecVal",
    "Z3_OP_BOR": BV,
    "Z3_OP_BREDAND": None,
    "Z3_OP_BREDOR": None,
    "Z3_OP_BSDIV": BV,
    "Z3_OP_BSDIV0": None,
    "Z3_OP_BSDIV_I": BV,
    "Z3_OP_BSHL": BV,
    "Z3_OP_BSMOD": BV,
    "Z3_OP_BSMOD0": None,
    "Z3_OP_BSMOD_I": BV,
    "Z3_OP_BSMUL_NO_OVFL": None,
    "Z3_OP_BSMUL_NO_UDFL": None,
    "Z3_OP_BSREM": BV,
    "Z3_OP_BSREM0": None,
    "Z3_OP_BSREM_I": BV,
    "Z3_OP_BSUB": BV,
    "Z3_OP_BUDIV": BV,
    "Z3_OP_BUDIV0": None,
    "Z3_OP_BUDIV_I": BV,
    "Z3_OP_BUMUL_NO_OVFL": None,
    "Z3_OP_BUREM": BV,
    "Z3_OP_BUREM0": None,
    "Z3_OP_BUREM_I": BV,
    "Z3_OP_BV2INT": None,
    "Z3_OP_BXNOR": None,
    "Z3_OP_BXOR": BV,
    "Z3_OP_CARRY": None,
    "Z3_OP_CHAR_CONST": None,
    "Z3_OP_CHAR_FROM_BV": None,
    "Z3_OP_CHAR_IS_DIGIT": None,
    "Z3_OP_CHAR_LE": None,
    "Z3_OP_CHAR_TO_BV": None,
    "Z3_OP_CHAR_TO_INT": None,
    "Z3_OP_CONCAT": BV,
    "Z3_OP_CONST_ARRAY": None,
    "Z3_OP_DISTINCT": Bool,
    "Z3_OP_DIV": None,
    "Z3_OP_DT_ACCESSOR": None,
    "Z3_OP_DT_CONSTRUCTOR": None,
    "Z3_OP_DT_IS": None,
    "Z3_OP_DT_RECOGNISER": None,
    "Z3_OP_DT_UPDATE_FIELD": None,
    "Z3_OP_EQ": Bool,
    "Z3_OP_EXTRACT": BV,
    "Z3_OP_EXT_ROTATE_LEFT": BV,
    "Z3_OP_EXT_ROTATE_RIGHT": BV,
    "Z3_OP_FALSE": Bool,
    "Z3_OP_FD_CONSTANT": None,
    "Z3_OP_FD_LT": None,
    "Z3_OP_FPA_ABS": FP,
    "Z3_OP_FPA_ADD": FP,
    "Z3_OP_FPA_BV2RM": None,
    "Z3_OP_FPA_BVWRAP": None,
    "Z3_OP_FPA_DIV": FP,
    "Z3_OP_FPA_EQ": Bool,
    "Z3_OP_FPA_FMA": None,
    "Z3_OP_FPA_FP": None,
    "Z3_OP_FPA_GE": Bool,
    "Z3_OP_FPA_GT": Bool,
    "Z3_OP_FPA_IS_INF": None,
    "Z3_OP_FPA_IS_NAN": None,
    "Z3_OP_FPA_IS_NEGATIVE": None,
    "Z3_OP_FPA_IS_NORMAL": None,
    "Z3_OP_FPA_IS_POSITIVE": None,
    "Z3_OP_FPA_IS_SUBNORMAL": None,
    "Z3_OP_FPA_IS_ZERO": None,
    "Z3_OP_FPA_LE": Bool,
    "Z3_OP_FPA_LT": Bool,
    "Z3_OP_FPA_MAX": None,
    "Z3_OP_FPA_MIN": None,
    "Z3_OP_FPA_MINUS_INF": FP,
    "Z3_OP_FPA_MINUS_ZERO": FP,
    "Z3_OP_FPA_MUL": FP,
    "Z3_OP_FPA_NAN": FP,
    "Z3_OP_FPA_NEG": FP,
    "Z3_OP_FPA_NUM": FP,
    "Z3_OP_FPA_PLUS_INF": FP,
    "Z3_OP_FPA_PLUS_ZERO": FP,
    "Z3_OP_FPA_REM": None,
    "Z3_OP_FPA_RM_NEAREST_TIES_TO_AWAY": None,
    "Z3_OP_FPA_RM_NEAREST_TIES_TO_EVEN": None,
    "Z3_OP_FPA_RM_TOWARD_NEGATIVE": None,
    "Z3_OP_FPA_RM_TOWARD_POSITIVE": None,
    "Z3_OP_FPA_RM_TOWARD_ZERO": None,
    "Z3_OP_FPA_ROUND_TO_INTEGRAL": None,
    "Z3_OP_FPA_SQRT": FP,
    "Z3_OP_FPA_SUB": FP,
    "Z3_OP_FPA_TO_FP": FP,
    "Z3_OP_FPA_TO_FP_UNSIGNED": FP,
    "Z3_OP_FPA_TO_IEEE_BV": BV,
    "Z3_OP_FPA_TO_REAL": None,
    "Z3_OP_FPA_TO_SBV": BV,
    "Z3_OP_FPA_TO_UBV": BV,
    "Z3_OP_GE": None,
    "Z3_OP_GT": None,
    "Z3_OP_IDIV": None,
    "Z3_OP_IFF": Bool,
    "Z3_OP_IMPLIES": None,
    "Z3_OP_INT2BV": None,
    "Z3_OP_INTERNAL": None,
    "Z3_OP_INT_TO_STR": None,
    "Z3_OP_IS_INT": None,
    "Z3_OP_ITE": Bool,
    "Z3_OP_LABEL": None,
    "Z3_OP_LABEL_LIT": None,
    "Z3_OP_LE": None,
    "Z3_OP_LT": None,
    "Z3_OP_MOD": None,
    "Z3_OP_MUL": None,
    "Z3_OP_NOT": Bool,
    "Z3_OP_OEQ": None,
    "Z3_OP_OR": Bool,
    "Z3_OP_PB_AT_LEAST": None,
    "Z3_OP_PB_AT_MOST": None,
    "Z3_OP_PB_EQ": None,
    "Z3_OP_PB_GE": None,
    "Z3_OP_PB_LE": None,
    "Z3_OP_POWER": None,
    "Z3_OP_PR_AND_ELIM": None,
    "Z3_OP_PR_APPLY_DEF": None,
    "Z3_OP_PR_ASSERTED": None,
    "Z3_OP_PR_ASSUMPTION_ADD": None,
    "Z3_OP_PR_BIND": None,
    "Z3_OP_PR_CLAUSE_TRAIL": None,
    "Z3_OP_PR_COMMUTATIVITY": None,
    "Z3_OP_PR_DEF_AXIOM": None,
    "Z3_OP_PR_DEF_INTRO": None,
    "Z3_OP_PR_DER": None,
    "Z3_OP_PR_DISTRIBUTIVITY": None,
    "Z3_OP_PR_ELIM_UNUSED_VARS": None,
    "Z3_OP_PR_GOAL": None,
    "Z3_OP_PR_HYPER_RESOLVE": None,
    "Z3_OP_PR_HYPOTHESIS": None,
    "Z3_OP_PR_IFF_FALSE": None,
    "Z3_OP_PR_IFF_OEQ": None,
    "Z3_OP_PR_IFF_TRUE": None,
    "Z3_OP_PR_LEMMA": None,
    "Z3_OP_PR_LEMMA_ADD": None,
    "Z3_OP_PR_MODUS_PONENS": None,
    "Z3_OP_PR_MODUS_PONENS_OEQ": None,
    "Z3_OP_PR_MONOTONICITY": None,
    "Z3_OP_PR_NNF_NEG": None,
    "Z3_OP_PR_NNF_POS": None,
    "Z3_OP_PR_NOT_OR_ELIM": None,
    "Z3_OP_PR_PULL_QUANT": None,
    "Z3_OP_PR_PUSH_QUANT": None,
    "Z3_OP_PR_QUANT_INST": None,
    "Z3_OP_PR_QUANT_INTRO": None,
    "Z3_OP_PR_REDUNDANT_DEL": None,
    "Z3_OP_PR_REFLEXIVITY": None,
    "Z3_OP_PR_REWRITE": None,
    "Z3_OP_PR_REWRITE_STAR": None,
    "Z3_OP_PR_SKOLEMIZE": None,
    "Z3_OP_PR_SYMMETRY": None,
    "Z3_OP_PR_TH_LEMMA": None,
    "Z3_OP_PR_TRANSITIVITY": None,
    "Z3_OP_PR_TRANSITIVITY_STAR": None,
    "Z3_OP_PR_TRUE": None,
    "Z3_OP_PR_UNDEF": None,
    "Z3_OP_PR_UNIT_RESOLUTION": None,
    "Z3_OP_RA_CLONE": None,
    "Z3_OP_RA_COMPLEMENT": None,
    "Z3_OP_RA_EMPTY": None,
    "Z3_OP_RA_FILTER": None,
    "Z3_OP_RA_IS_EMPTY": None,
    "Z3_OP_RA_JOIN": None,
    "Z3_OP_RA_NEGATION_FILTER": None,
    "Z3_OP_RA_PROJECT": None,
    "Z3_OP_RA_RENAME": None,
    "Z3_OP_RA_SELECT": None,
    "Z3_OP_RA_STORE": None,
    "Z3_OP_RA_UNION": None,
    "Z3_OP_RA_WIDEN": None,
    "Z3_OP_RECURSIVE": None,
    "Z3_OP_REM": None,
    "Z3_OP_REPEAT": BV,
    "Z3_OP_RE_COMPLEMENT": None,
    "Z3_OP_RE_CONCAT": None,
    "Z3_OP_RE_DERIVATIVE": None,
    "Z3_OP_RE_DIFF": None,
    "Z3_OP_RE_EMPTY_SET": None,
    "Z3_OP_RE_FULL_CHAR_SET": None,
    "Z3_OP_RE_FULL_SET": None,
    "Z3_OP_RE_INTERSECT": None,
    "Z3_OP_RE_LOOP": None,
    "Z3_OP_RE_OF_PRED": None,
    "Z3_OP_RE_OPTION": None,
    "Z3_OP_RE_PLUS": None,
    "Z3_OP_RE_POWER": None,
    "Z3_OP_RE_RANGE": None,
    "Z3_OP_RE_REVERSE": None,
    "Z3_OP_RE_STAR": None,
    "Z3_OP_RE_UNION": None,
    "Z3_OP_ROTATE_LEFT": None,
    "Z3_OP_ROTATE_RIGHT": None,
    "Z3_OP_SBV_TO_STR": None,
    "Z3_OP_SELECT": None,
    "Z3_OP_SEQ_AT": None,
    "Z3_OP_SEQ_CONCAT": None,
    "Z3_OP_SEQ_CONTAINS": None,
    "Z3_OP_SEQ_EMPTY": None,
    "Z3_OP_SEQ_EXTRACT": None,
    "Z3_OP_SEQ_INDEX": None,
    "Z3_OP_SEQ_IN_RE": None,
    "Z3_OP_SEQ_LAST_INDEX": None,
    "Z3_OP_SEQ_LENGTH": None,
    "Z3_OP_SEQ_NTH": None,
    "Z3_OP_SEQ_PREFIX": None,
    "Z3_OP_SEQ_REPLACE": None,
    "Z3_OP_SEQ_REPLACE_ALL": None,
    "Z3_OP_SEQ_REPLACE_RE": None,
    "Z3_OP_SEQ_REPLACE_RE_ALL": None,
    "Z3_OP_SEQ_SUFFIX": None,
    "Z3_OP_SEQ_TO_RE": None,
    "Z3_OP_SEQ_UNIT": None,
    "Z3_OP_SET_CARD": None,
    "Z3_OP_SET_COMPLEMENT": None,
    "Z3_OP_SET_DIFFERENCE": None,
    "Z3_OP_SET_HAS_SIZE": None,
    "Z3_OP_SET_INTERSECT": None,
    "Z3_OP_SET_SUBSET": None,
    "Z3_OP_SET_UNION": None,
    "Z3_OP_SGEQ": Bool,
    "Z3_OP_SGT": Bool,
    "Z3_OP_SIGN_EXT": BV,
    "Z3_OP_SLEQ": Bool,
    "Z3_OP_SLT": Bool,
    "Z3_OP_SPECIAL_RELATION_LO": None,
    "Z3_OP_SPECIAL_RELATION_PLO": None,
    "Z3_OP_SPECIAL_RELATION_PO": None,
    "Z3_OP_SPECIAL_RELATION_TC": None,
    "Z3_OP_SPECIAL_RELATION_TO": None,
    "Z3_OP_SPECIAL_RELATION_TRC": None,
    "Z3_OP_STORE": None,
    "Z3_OP_STRING_LE": None,
    "Z3_OP_STRING_LT": None,
    "Z3_OP_STR_FROM_CODE": None,
    "Z3_OP_STR_TO_CODE": None,
    "Z3_OP_STR_TO_INT": None,
    "Z3_OP_SUB": None,
    "Z3_OP_TO_INT": None,
    "Z3_OP_TO_REAL": None,
    "Z3_OP_TRUE": Bool,
    "Z3_OP_UBV_TO_STR": None,
    "Z3_OP_UGEQ": Bool,
    "Z3_OP_UGT": Bool,
    "Z3_OP_ULEQ": Bool,
    "Z3_OP_ULT": Bool,
    "Z3_OP_UMINUS": None,
    "Z3_OP_UNINTERPRETED": "UNINTERPRETED",
    "Z3_OP_XOR": Bool,
    "Z3_OP_XOR3": None,
    "Z3_OP_ZERO_EXT": BV,
}
