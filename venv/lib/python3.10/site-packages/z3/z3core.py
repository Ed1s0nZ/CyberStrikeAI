
# Automatically generated file
import atexit
import sys, os
import contextlib
import ctypes
if sys.version_info >= (3, 9):
    import importlib.resources as importlib_resources
else:
    import importlib_resources
from .z3types import *
from .z3consts import *

_file_manager = contextlib.ExitStack()
atexit.register(_file_manager.close)
_ext = 'dll' if sys.platform in ('win32', 'cygwin') else 'dylib' if sys.platform == 'darwin' else 'so'
_lib = None
_z3_lib_resource = importlib_resources.files('z3').joinpath('lib')
_z3_lib_resource_path = _file_manager.enter_context(
    importlib_resources.as_file(_z3_lib_resource)
)
_default_dirs = ['.',
                 os.path.dirname(os.path.abspath(__file__)),
                 _z3_lib_resource_path,
                 os.path.join(sys.prefix, 'lib'),
                 None]
_all_dirs = []
# search the default dirs first
_all_dirs.extend(_default_dirs)

if sys.version < '3':
  import __builtin__
  if hasattr(__builtin__, "Z3_LIB_DIRS"):
    _all_dirs = __builtin__.Z3_LIB_DIRS
else:
  import builtins
  if hasattr(builtins, "Z3_LIB_DIRS"):
    _all_dirs = builtins.Z3_LIB_DIRS

for v in ('Z3_LIBRARY_PATH', 'PATH', 'PYTHONPATH'):
  if v in os.environ:
    lp = os.environ[v];
    lds = lp.split(';') if sys.platform in ('win32') else lp.split(':')
    _all_dirs.extend(lds)

_failures = []
for d in _all_dirs:
  try:
    d = os.path.realpath(d)
    if os.path.isdir(d):
      d = os.path.join(d, 'libz3.%s' % _ext)
      if os.path.isfile(d):
        _lib = ctypes.CDLL(d)
        break
  except Exception as e:
    _failures += [e]
    pass

if _lib is None:
  # If all else failed, ask the system to find it.
  try:
    _lib = ctypes.CDLL('libz3.%s' % _ext)
  except Exception as e:
    _failures += [e]
    pass

if _lib is None:
  print("Could not find libz3.%s; consider adding the directory containing it to" % _ext)
  print("  - your system's PATH environment variable,")
  print("  - the Z3_LIBRARY_PATH environment variable, or ")
  print("  - to the custom Z3_LIB_DIRS Python-builtin before importing the z3 module, e.g. via")
  if sys.version < '3':
    print("    import __builtin__")
    print("    __builtin__.Z3_LIB_DIRS = [ '/path/to/z3/lib/dir' ] # directory containing libz3.%s" % _ext)
  else:
    print("    import builtins")
    print("    builtins.Z3_LIB_DIRS = [ '/path/to/z3/lib/dir' ] # directory containing libz3.%s" % _ext)
  print(_failures)
  raise Z3Exception("libz3.%s not found." % _ext)


if sys.version < '3':
  def _str_to_bytes(s):
    return s
  def _to_pystr(s):
     return s
else:
  def _str_to_bytes(s):
    if isinstance(s, str):
        enc = sys.getdefaultencoding()
        return s.encode(enc if enc != None else 'latin-1')
    else:
        return s

  def _to_pystr(s):
     if s != None:
        enc = sys.getdefaultencoding()
        return s.decode(enc if enc != None else 'latin-1')
     else:
        return ""

_error_handler_type  = ctypes.CFUNCTYPE(None, ctypes.c_void_p, ctypes.c_uint)

_lib.Z3_set_error_handler.restype  = None
_lib.Z3_set_error_handler.argtypes = [ContextObj, _error_handler_type]

Z3_on_clause_eh = ctypes.CFUNCTYPE(None, ctypes.c_void_p, ctypes.c_void_p, ctypes.c_uint, ctypes.POINTER(ctypes.c_uint), ctypes.c_void_p)
Z3_push_eh  = ctypes.CFUNCTYPE(None, ctypes.c_void_p, ctypes.c_void_p)
Z3_pop_eh   = ctypes.CFUNCTYPE(None, ctypes.c_void_p, ctypes.c_void_p, ctypes.c_uint)
Z3_fresh_eh = ctypes.CFUNCTYPE(ctypes.c_void_p, ctypes.c_void_p, ctypes.c_void_p)

Z3_fixed_eh = ctypes.CFUNCTYPE(None, ctypes.c_void_p, ctypes.c_void_p, ctypes.c_void_p, ctypes.c_void_p)
Z3_final_eh = ctypes.CFUNCTYPE(None, ctypes.c_void_p, ctypes.c_void_p)
Z3_eq_eh    = ctypes.CFUNCTYPE(None, ctypes.c_void_p, ctypes.c_void_p, ctypes.c_void_p, ctypes.c_void_p)

Z3_created_eh = ctypes.CFUNCTYPE(None, ctypes.c_void_p, ctypes.c_void_p, ctypes.c_void_p)
Z3_decide_eh = ctypes.CFUNCTYPE(None, ctypes.c_void_p, ctypes.c_void_p, ctypes.c_void_p, ctypes.c_uint, ctypes.c_int)

_lib.Z3_solver_register_on_clause.restype = None
_lib.Z3_solver_propagate_init.restype = None
_lib.Z3_solver_propagate_final.restype = None
_lib.Z3_solver_propagate_fixed.restype = None
_lib.Z3_solver_propagate_eq.restype = None
_lib.Z3_solver_propagate_diseq.restype = None
_lib.Z3_solver_propagate_decide.restype = None

on_model_eh_type = ctypes.CFUNCTYPE(None, ctypes.c_void_p)
_lib.Z3_optimize_register_model_eh.restype = None
_lib.Z3_optimize_register_model_eh.argtypes = [ContextObj, OptimizeObj, ModelObj, ctypes.c_void_p, on_model_eh_type]

_lib.Z3_global_param_set.argtypes = [ctypes.c_char_p, ctypes.c_char_p]
_lib.Z3_global_param_reset_all.argtypes = []
_lib.Z3_global_param_get.restype = ctypes.c_bool
_lib.Z3_global_param_get.argtypes = [ctypes.c_char_p, ctypes.POINTER(ctypes.c_char_p)]
_lib.Z3_mk_config.restype = Config
_lib.Z3_mk_config.argtypes = []
_lib.Z3_del_config.argtypes = [Config]
_lib.Z3_set_param_value.argtypes = [Config, ctypes.c_char_p, ctypes.c_char_p]
_lib.Z3_mk_context.restype = ContextObj
_lib.Z3_mk_context.argtypes = [Config]
_lib.Z3_mk_context_rc.restype = ContextObj
_lib.Z3_mk_context_rc.argtypes = [Config]
_lib.Z3_del_context.argtypes = [ContextObj]
_lib.Z3_inc_ref.argtypes = [ContextObj, Ast]
_lib.Z3_dec_ref.argtypes = [ContextObj, Ast]
_lib.Z3_update_param_value.argtypes = [ContextObj, ctypes.c_char_p, ctypes.c_char_p]
_lib.Z3_get_global_param_descrs.restype = ParamDescrs
_lib.Z3_get_global_param_descrs.argtypes = [ContextObj]
_lib.Z3_interrupt.argtypes = [ContextObj]
_lib.Z3_enable_concurrent_dec_ref.argtypes = [ContextObj]
_lib.Z3_mk_params.restype = Params
_lib.Z3_mk_params.argtypes = [ContextObj]
_lib.Z3_params_inc_ref.argtypes = [ContextObj, Params]
_lib.Z3_params_dec_ref.argtypes = [ContextObj, Params]
_lib.Z3_params_set_bool.argtypes = [ContextObj, Params, Symbol, ctypes.c_bool]
_lib.Z3_params_set_uint.argtypes = [ContextObj, Params, Symbol, ctypes.c_uint]
_lib.Z3_params_set_double.argtypes = [ContextObj, Params, Symbol, ctypes.c_double]
_lib.Z3_params_set_symbol.argtypes = [ContextObj, Params, Symbol, Symbol]
_lib.Z3_params_to_string.restype = ctypes.c_char_p
_lib.Z3_params_to_string.argtypes = [ContextObj, Params]
_lib.Z3_params_validate.argtypes = [ContextObj, Params, ParamDescrs]
_lib.Z3_param_descrs_inc_ref.argtypes = [ContextObj, ParamDescrs]
_lib.Z3_param_descrs_dec_ref.argtypes = [ContextObj, ParamDescrs]
_lib.Z3_param_descrs_get_kind.restype = ctypes.c_uint
_lib.Z3_param_descrs_get_kind.argtypes = [ContextObj, ParamDescrs, Symbol]
_lib.Z3_param_descrs_size.restype = ctypes.c_uint
_lib.Z3_param_descrs_size.argtypes = [ContextObj, ParamDescrs]
_lib.Z3_param_descrs_get_name.restype = Symbol
_lib.Z3_param_descrs_get_name.argtypes = [ContextObj, ParamDescrs, ctypes.c_uint]
_lib.Z3_param_descrs_get_documentation.restype = ctypes.c_char_p
_lib.Z3_param_descrs_get_documentation.argtypes = [ContextObj, ParamDescrs, Symbol]
_lib.Z3_param_descrs_to_string.restype = ctypes.c_char_p
_lib.Z3_param_descrs_to_string.argtypes = [ContextObj, ParamDescrs]
_lib.Z3_mk_int_symbol.restype = Symbol
_lib.Z3_mk_int_symbol.argtypes = [ContextObj, ctypes.c_int]
_lib.Z3_mk_string_symbol.restype = Symbol
_lib.Z3_mk_string_symbol.argtypes = [ContextObj, ctypes.c_char_p]
_lib.Z3_mk_uninterpreted_sort.restype = Sort
_lib.Z3_mk_uninterpreted_sort.argtypes = [ContextObj, Symbol]
_lib.Z3_mk_type_variable.restype = Sort
_lib.Z3_mk_type_variable.argtypes = [ContextObj, Symbol]
_lib.Z3_mk_bool_sort.restype = Sort
_lib.Z3_mk_bool_sort.argtypes = [ContextObj]
_lib.Z3_mk_int_sort.restype = Sort
_lib.Z3_mk_int_sort.argtypes = [ContextObj]
_lib.Z3_mk_real_sort.restype = Sort
_lib.Z3_mk_real_sort.argtypes = [ContextObj]
_lib.Z3_mk_bv_sort.restype = Sort
_lib.Z3_mk_bv_sort.argtypes = [ContextObj, ctypes.c_uint]
_lib.Z3_mk_finite_domain_sort.restype = Sort
_lib.Z3_mk_finite_domain_sort.argtypes = [ContextObj, Symbol, ctypes.c_ulonglong]
_lib.Z3_mk_array_sort.restype = Sort
_lib.Z3_mk_array_sort.argtypes = [ContextObj, Sort, Sort]
_lib.Z3_mk_array_sort_n.restype = Sort
_lib.Z3_mk_array_sort_n.argtypes = [ContextObj, ctypes.c_uint, ctypes.POINTER(Sort), Sort]
_lib.Z3_mk_tuple_sort.restype = Sort
_lib.Z3_mk_tuple_sort.argtypes = [ContextObj, Symbol, ctypes.c_uint, ctypes.POINTER(Symbol), ctypes.POINTER(Sort), ctypes.POINTER(FuncDecl), ctypes.POINTER(FuncDecl)]
_lib.Z3_mk_enumeration_sort.restype = Sort
_lib.Z3_mk_enumeration_sort.argtypes = [ContextObj, Symbol, ctypes.c_uint, ctypes.POINTER(Symbol), ctypes.POINTER(FuncDecl), ctypes.POINTER(FuncDecl)]
_lib.Z3_mk_list_sort.restype = Sort
_lib.Z3_mk_list_sort.argtypes = [ContextObj, Symbol, Sort, ctypes.POINTER(FuncDecl), ctypes.POINTER(FuncDecl), ctypes.POINTER(FuncDecl), ctypes.POINTER(FuncDecl), ctypes.POINTER(FuncDecl), ctypes.POINTER(FuncDecl)]
_lib.Z3_mk_constructor.restype = Constructor
_lib.Z3_mk_constructor.argtypes = [ContextObj, Symbol, Symbol, ctypes.c_uint, ctypes.POINTER(Symbol), ctypes.POINTER(Sort), ctypes.POINTER(ctypes.c_uint)]
_lib.Z3_constructor_num_fields.restype = ctypes.c_uint
_lib.Z3_constructor_num_fields.argtypes = [ContextObj, Constructor]
_lib.Z3_del_constructor.argtypes = [ContextObj, Constructor]
_lib.Z3_mk_datatype.restype = Sort
_lib.Z3_mk_datatype.argtypes = [ContextObj, Symbol, ctypes.c_uint, ctypes.POINTER(Constructor)]
_lib.Z3_mk_datatype_sort.restype = Sort
_lib.Z3_mk_datatype_sort.argtypes = [ContextObj, Symbol]
_lib.Z3_mk_constructor_list.restype = ConstructorList
_lib.Z3_mk_constructor_list.argtypes = [ContextObj, ctypes.c_uint, ctypes.POINTER(Constructor)]
_lib.Z3_del_constructor_list.argtypes = [ContextObj, ConstructorList]
_lib.Z3_mk_datatypes.argtypes = [ContextObj, ctypes.c_uint, ctypes.POINTER(Symbol), ctypes.POINTER(Sort), ctypes.POINTER(ConstructorList)]
_lib.Z3_query_constructor.argtypes = [ContextObj, Constructor, ctypes.c_uint, ctypes.POINTER(FuncDecl), ctypes.POINTER(FuncDecl), ctypes.POINTER(FuncDecl)]
_lib.Z3_mk_func_decl.restype = FuncDecl
_lib.Z3_mk_func_decl.argtypes = [ContextObj, Symbol, ctypes.c_uint, ctypes.POINTER(Sort), Sort]
_lib.Z3_mk_app.restype = Ast
_lib.Z3_mk_app.argtypes = [ContextObj, FuncDecl, ctypes.c_uint, ctypes.POINTER(Ast)]
_lib.Z3_mk_const.restype = Ast
_lib.Z3_mk_const.argtypes = [ContextObj, Symbol, Sort]
_lib.Z3_mk_fresh_func_decl.restype = FuncDecl
_lib.Z3_mk_fresh_func_decl.argtypes = [ContextObj, ctypes.c_char_p, ctypes.c_uint, ctypes.POINTER(Sort), Sort]
_lib.Z3_mk_fresh_const.restype = Ast
_lib.Z3_mk_fresh_const.argtypes = [ContextObj, ctypes.c_char_p, Sort]
_lib.Z3_mk_rec_func_decl.restype = FuncDecl
_lib.Z3_mk_rec_func_decl.argtypes = [ContextObj, Symbol, ctypes.c_uint, ctypes.POINTER(Sort), Sort]
_lib.Z3_add_rec_def.argtypes = [ContextObj, FuncDecl, ctypes.c_uint, ctypes.POINTER(Ast), Ast]
_lib.Z3_mk_true.restype = Ast
_lib.Z3_mk_true.argtypes = [ContextObj]
_lib.Z3_mk_false.restype = Ast
_lib.Z3_mk_false.argtypes = [ContextObj]
_lib.Z3_mk_eq.restype = Ast
_lib.Z3_mk_eq.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_distinct.restype = Ast
_lib.Z3_mk_distinct.argtypes = [ContextObj, ctypes.c_uint, ctypes.POINTER(Ast)]
_lib.Z3_mk_not.restype = Ast
_lib.Z3_mk_not.argtypes = [ContextObj, Ast]
_lib.Z3_mk_ite.restype = Ast
_lib.Z3_mk_ite.argtypes = [ContextObj, Ast, Ast, Ast]
_lib.Z3_mk_iff.restype = Ast
_lib.Z3_mk_iff.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_implies.restype = Ast
_lib.Z3_mk_implies.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_xor.restype = Ast
_lib.Z3_mk_xor.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_and.restype = Ast
_lib.Z3_mk_and.argtypes = [ContextObj, ctypes.c_uint, ctypes.POINTER(Ast)]
_lib.Z3_mk_or.restype = Ast
_lib.Z3_mk_or.argtypes = [ContextObj, ctypes.c_uint, ctypes.POINTER(Ast)]
_lib.Z3_mk_add.restype = Ast
_lib.Z3_mk_add.argtypes = [ContextObj, ctypes.c_uint, ctypes.POINTER(Ast)]
_lib.Z3_mk_mul.restype = Ast
_lib.Z3_mk_mul.argtypes = [ContextObj, ctypes.c_uint, ctypes.POINTER(Ast)]
_lib.Z3_mk_sub.restype = Ast
_lib.Z3_mk_sub.argtypes = [ContextObj, ctypes.c_uint, ctypes.POINTER(Ast)]
_lib.Z3_mk_unary_minus.restype = Ast
_lib.Z3_mk_unary_minus.argtypes = [ContextObj, Ast]
_lib.Z3_mk_div.restype = Ast
_lib.Z3_mk_div.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_mod.restype = Ast
_lib.Z3_mk_mod.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_rem.restype = Ast
_lib.Z3_mk_rem.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_power.restype = Ast
_lib.Z3_mk_power.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_lt.restype = Ast
_lib.Z3_mk_lt.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_le.restype = Ast
_lib.Z3_mk_le.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_gt.restype = Ast
_lib.Z3_mk_gt.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_ge.restype = Ast
_lib.Z3_mk_ge.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_divides.restype = Ast
_lib.Z3_mk_divides.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_int2real.restype = Ast
_lib.Z3_mk_int2real.argtypes = [ContextObj, Ast]
_lib.Z3_mk_real2int.restype = Ast
_lib.Z3_mk_real2int.argtypes = [ContextObj, Ast]
_lib.Z3_mk_is_int.restype = Ast
_lib.Z3_mk_is_int.argtypes = [ContextObj, Ast]
_lib.Z3_mk_bvnot.restype = Ast
_lib.Z3_mk_bvnot.argtypes = [ContextObj, Ast]
_lib.Z3_mk_bvredand.restype = Ast
_lib.Z3_mk_bvredand.argtypes = [ContextObj, Ast]
_lib.Z3_mk_bvredor.restype = Ast
_lib.Z3_mk_bvredor.argtypes = [ContextObj, Ast]
_lib.Z3_mk_bvand.restype = Ast
_lib.Z3_mk_bvand.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_bvor.restype = Ast
_lib.Z3_mk_bvor.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_bvxor.restype = Ast
_lib.Z3_mk_bvxor.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_bvnand.restype = Ast
_lib.Z3_mk_bvnand.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_bvnor.restype = Ast
_lib.Z3_mk_bvnor.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_bvxnor.restype = Ast
_lib.Z3_mk_bvxnor.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_bvneg.restype = Ast
_lib.Z3_mk_bvneg.argtypes = [ContextObj, Ast]
_lib.Z3_mk_bvadd.restype = Ast
_lib.Z3_mk_bvadd.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_bvsub.restype = Ast
_lib.Z3_mk_bvsub.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_bvmul.restype = Ast
_lib.Z3_mk_bvmul.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_bvudiv.restype = Ast
_lib.Z3_mk_bvudiv.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_bvsdiv.restype = Ast
_lib.Z3_mk_bvsdiv.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_bvurem.restype = Ast
_lib.Z3_mk_bvurem.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_bvsrem.restype = Ast
_lib.Z3_mk_bvsrem.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_bvsmod.restype = Ast
_lib.Z3_mk_bvsmod.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_bvult.restype = Ast
_lib.Z3_mk_bvult.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_bvslt.restype = Ast
_lib.Z3_mk_bvslt.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_bvule.restype = Ast
_lib.Z3_mk_bvule.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_bvsle.restype = Ast
_lib.Z3_mk_bvsle.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_bvuge.restype = Ast
_lib.Z3_mk_bvuge.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_bvsge.restype = Ast
_lib.Z3_mk_bvsge.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_bvugt.restype = Ast
_lib.Z3_mk_bvugt.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_bvsgt.restype = Ast
_lib.Z3_mk_bvsgt.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_concat.restype = Ast
_lib.Z3_mk_concat.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_extract.restype = Ast
_lib.Z3_mk_extract.argtypes = [ContextObj, ctypes.c_uint, ctypes.c_uint, Ast]
_lib.Z3_mk_sign_ext.restype = Ast
_lib.Z3_mk_sign_ext.argtypes = [ContextObj, ctypes.c_uint, Ast]
_lib.Z3_mk_zero_ext.restype = Ast
_lib.Z3_mk_zero_ext.argtypes = [ContextObj, ctypes.c_uint, Ast]
_lib.Z3_mk_repeat.restype = Ast
_lib.Z3_mk_repeat.argtypes = [ContextObj, ctypes.c_uint, Ast]
_lib.Z3_mk_bit2bool.restype = Ast
_lib.Z3_mk_bit2bool.argtypes = [ContextObj, ctypes.c_uint, Ast]
_lib.Z3_mk_bvshl.restype = Ast
_lib.Z3_mk_bvshl.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_bvlshr.restype = Ast
_lib.Z3_mk_bvlshr.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_bvashr.restype = Ast
_lib.Z3_mk_bvashr.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_rotate_left.restype = Ast
_lib.Z3_mk_rotate_left.argtypes = [ContextObj, ctypes.c_uint, Ast]
_lib.Z3_mk_rotate_right.restype = Ast
_lib.Z3_mk_rotate_right.argtypes = [ContextObj, ctypes.c_uint, Ast]
_lib.Z3_mk_ext_rotate_left.restype = Ast
_lib.Z3_mk_ext_rotate_left.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_ext_rotate_right.restype = Ast
_lib.Z3_mk_ext_rotate_right.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_int2bv.restype = Ast
_lib.Z3_mk_int2bv.argtypes = [ContextObj, ctypes.c_uint, Ast]
_lib.Z3_mk_bv2int.restype = Ast
_lib.Z3_mk_bv2int.argtypes = [ContextObj, Ast, ctypes.c_bool]
_lib.Z3_mk_bvadd_no_overflow.restype = Ast
_lib.Z3_mk_bvadd_no_overflow.argtypes = [ContextObj, Ast, Ast, ctypes.c_bool]
_lib.Z3_mk_bvadd_no_underflow.restype = Ast
_lib.Z3_mk_bvadd_no_underflow.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_bvsub_no_overflow.restype = Ast
_lib.Z3_mk_bvsub_no_overflow.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_bvsub_no_underflow.restype = Ast
_lib.Z3_mk_bvsub_no_underflow.argtypes = [ContextObj, Ast, Ast, ctypes.c_bool]
_lib.Z3_mk_bvsdiv_no_overflow.restype = Ast
_lib.Z3_mk_bvsdiv_no_overflow.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_bvneg_no_overflow.restype = Ast
_lib.Z3_mk_bvneg_no_overflow.argtypes = [ContextObj, Ast]
_lib.Z3_mk_bvmul_no_overflow.restype = Ast
_lib.Z3_mk_bvmul_no_overflow.argtypes = [ContextObj, Ast, Ast, ctypes.c_bool]
_lib.Z3_mk_bvmul_no_underflow.restype = Ast
_lib.Z3_mk_bvmul_no_underflow.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_select.restype = Ast
_lib.Z3_mk_select.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_select_n.restype = Ast
_lib.Z3_mk_select_n.argtypes = [ContextObj, Ast, ctypes.c_uint, ctypes.POINTER(Ast)]
_lib.Z3_mk_store.restype = Ast
_lib.Z3_mk_store.argtypes = [ContextObj, Ast, Ast, Ast]
_lib.Z3_mk_store_n.restype = Ast
_lib.Z3_mk_store_n.argtypes = [ContextObj, Ast, ctypes.c_uint, ctypes.POINTER(Ast), Ast]
_lib.Z3_mk_const_array.restype = Ast
_lib.Z3_mk_const_array.argtypes = [ContextObj, Sort, Ast]
_lib.Z3_mk_map.restype = Ast
_lib.Z3_mk_map.argtypes = [ContextObj, FuncDecl, ctypes.c_uint, ctypes.POINTER(Ast)]
_lib.Z3_mk_array_default.restype = Ast
_lib.Z3_mk_array_default.argtypes = [ContextObj, Ast]
_lib.Z3_mk_as_array.restype = Ast
_lib.Z3_mk_as_array.argtypes = [ContextObj, FuncDecl]
_lib.Z3_mk_set_has_size.restype = Ast
_lib.Z3_mk_set_has_size.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_set_sort.restype = Sort
_lib.Z3_mk_set_sort.argtypes = [ContextObj, Sort]
_lib.Z3_mk_empty_set.restype = Ast
_lib.Z3_mk_empty_set.argtypes = [ContextObj, Sort]
_lib.Z3_mk_full_set.restype = Ast
_lib.Z3_mk_full_set.argtypes = [ContextObj, Sort]
_lib.Z3_mk_set_add.restype = Ast
_lib.Z3_mk_set_add.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_set_del.restype = Ast
_lib.Z3_mk_set_del.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_set_union.restype = Ast
_lib.Z3_mk_set_union.argtypes = [ContextObj, ctypes.c_uint, ctypes.POINTER(Ast)]
_lib.Z3_mk_set_intersect.restype = Ast
_lib.Z3_mk_set_intersect.argtypes = [ContextObj, ctypes.c_uint, ctypes.POINTER(Ast)]
_lib.Z3_mk_set_difference.restype = Ast
_lib.Z3_mk_set_difference.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_set_complement.restype = Ast
_lib.Z3_mk_set_complement.argtypes = [ContextObj, Ast]
_lib.Z3_mk_set_member.restype = Ast
_lib.Z3_mk_set_member.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_set_subset.restype = Ast
_lib.Z3_mk_set_subset.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_array_ext.restype = Ast
_lib.Z3_mk_array_ext.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_numeral.restype = Ast
_lib.Z3_mk_numeral.argtypes = [ContextObj, ctypes.c_char_p, Sort]
_lib.Z3_mk_real.restype = Ast
_lib.Z3_mk_real.argtypes = [ContextObj, ctypes.c_int, ctypes.c_int]
_lib.Z3_mk_real_int64.restype = Ast
_lib.Z3_mk_real_int64.argtypes = [ContextObj, ctypes.c_longlong, ctypes.c_longlong]
_lib.Z3_mk_int.restype = Ast
_lib.Z3_mk_int.argtypes = [ContextObj, ctypes.c_int, Sort]
_lib.Z3_mk_unsigned_int.restype = Ast
_lib.Z3_mk_unsigned_int.argtypes = [ContextObj, ctypes.c_uint, Sort]
_lib.Z3_mk_int64.restype = Ast
_lib.Z3_mk_int64.argtypes = [ContextObj, ctypes.c_longlong, Sort]
_lib.Z3_mk_unsigned_int64.restype = Ast
_lib.Z3_mk_unsigned_int64.argtypes = [ContextObj, ctypes.c_ulonglong, Sort]
_lib.Z3_mk_bv_numeral.restype = Ast
_lib.Z3_mk_bv_numeral.argtypes = [ContextObj, ctypes.c_uint, ctypes.POINTER(ctypes.c_bool)]
_lib.Z3_mk_seq_sort.restype = Sort
_lib.Z3_mk_seq_sort.argtypes = [ContextObj, Sort]
_lib.Z3_is_seq_sort.restype = ctypes.c_bool
_lib.Z3_is_seq_sort.argtypes = [ContextObj, Sort]
_lib.Z3_get_seq_sort_basis.restype = Sort
_lib.Z3_get_seq_sort_basis.argtypes = [ContextObj, Sort]
_lib.Z3_mk_re_sort.restype = Sort
_lib.Z3_mk_re_sort.argtypes = [ContextObj, Sort]
_lib.Z3_is_re_sort.restype = ctypes.c_bool
_lib.Z3_is_re_sort.argtypes = [ContextObj, Sort]
_lib.Z3_get_re_sort_basis.restype = Sort
_lib.Z3_get_re_sort_basis.argtypes = [ContextObj, Sort]
_lib.Z3_mk_string_sort.restype = Sort
_lib.Z3_mk_string_sort.argtypes = [ContextObj]
_lib.Z3_mk_char_sort.restype = Sort
_lib.Z3_mk_char_sort.argtypes = [ContextObj]
_lib.Z3_is_string_sort.restype = ctypes.c_bool
_lib.Z3_is_string_sort.argtypes = [ContextObj, Sort]
_lib.Z3_is_char_sort.restype = ctypes.c_bool
_lib.Z3_is_char_sort.argtypes = [ContextObj, Sort]
_lib.Z3_mk_string.restype = Ast
_lib.Z3_mk_string.argtypes = [ContextObj, ctypes.c_char_p]
_lib.Z3_mk_lstring.restype = Ast
_lib.Z3_mk_lstring.argtypes = [ContextObj, ctypes.c_uint, ctypes.c_char_p]
_lib.Z3_mk_u32string.restype = Ast
_lib.Z3_mk_u32string.argtypes = [ContextObj, ctypes.c_uint, ctypes.POINTER(ctypes.c_uint)]
_lib.Z3_is_string.restype = ctypes.c_bool
_lib.Z3_is_string.argtypes = [ContextObj, Ast]
_lib.Z3_get_string.restype = ctypes.c_char_p
_lib.Z3_get_string.argtypes = [ContextObj, Ast]
_lib.Z3_get_lstring.restype = ctypes.POINTER(ctypes.c_char)
_lib.Z3_get_lstring.argtypes = [ContextObj, Ast, ctypes.POINTER(ctypes.c_uint)]
_lib.Z3_get_string_length.restype = ctypes.c_uint
_lib.Z3_get_string_length.argtypes = [ContextObj, Ast]
_lib.Z3_get_string_contents.argtypes = [ContextObj, Ast, ctypes.c_uint, ctypes.POINTER(ctypes.c_uint)]
_lib.Z3_mk_seq_empty.restype = Ast
_lib.Z3_mk_seq_empty.argtypes = [ContextObj, Sort]
_lib.Z3_mk_seq_unit.restype = Ast
_lib.Z3_mk_seq_unit.argtypes = [ContextObj, Ast]
_lib.Z3_mk_seq_concat.restype = Ast
_lib.Z3_mk_seq_concat.argtypes = [ContextObj, ctypes.c_uint, ctypes.POINTER(Ast)]
_lib.Z3_mk_seq_prefix.restype = Ast
_lib.Z3_mk_seq_prefix.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_seq_suffix.restype = Ast
_lib.Z3_mk_seq_suffix.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_seq_contains.restype = Ast
_lib.Z3_mk_seq_contains.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_str_lt.restype = Ast
_lib.Z3_mk_str_lt.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_str_le.restype = Ast
_lib.Z3_mk_str_le.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_seq_extract.restype = Ast
_lib.Z3_mk_seq_extract.argtypes = [ContextObj, Ast, Ast, Ast]
_lib.Z3_mk_seq_replace.restype = Ast
_lib.Z3_mk_seq_replace.argtypes = [ContextObj, Ast, Ast, Ast]
_lib.Z3_mk_seq_at.restype = Ast
_lib.Z3_mk_seq_at.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_seq_nth.restype = Ast
_lib.Z3_mk_seq_nth.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_seq_length.restype = Ast
_lib.Z3_mk_seq_length.argtypes = [ContextObj, Ast]
_lib.Z3_mk_seq_index.restype = Ast
_lib.Z3_mk_seq_index.argtypes = [ContextObj, Ast, Ast, Ast]
_lib.Z3_mk_seq_last_index.restype = Ast
_lib.Z3_mk_seq_last_index.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_str_to_int.restype = Ast
_lib.Z3_mk_str_to_int.argtypes = [ContextObj, Ast]
_lib.Z3_mk_int_to_str.restype = Ast
_lib.Z3_mk_int_to_str.argtypes = [ContextObj, Ast]
_lib.Z3_mk_string_to_code.restype = Ast
_lib.Z3_mk_string_to_code.argtypes = [ContextObj, Ast]
_lib.Z3_mk_string_from_code.restype = Ast
_lib.Z3_mk_string_from_code.argtypes = [ContextObj, Ast]
_lib.Z3_mk_ubv_to_str.restype = Ast
_lib.Z3_mk_ubv_to_str.argtypes = [ContextObj, Ast]
_lib.Z3_mk_sbv_to_str.restype = Ast
_lib.Z3_mk_sbv_to_str.argtypes = [ContextObj, Ast]
_lib.Z3_mk_seq_to_re.restype = Ast
_lib.Z3_mk_seq_to_re.argtypes = [ContextObj, Ast]
_lib.Z3_mk_seq_in_re.restype = Ast
_lib.Z3_mk_seq_in_re.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_re_plus.restype = Ast
_lib.Z3_mk_re_plus.argtypes = [ContextObj, Ast]
_lib.Z3_mk_re_star.restype = Ast
_lib.Z3_mk_re_star.argtypes = [ContextObj, Ast]
_lib.Z3_mk_re_option.restype = Ast
_lib.Z3_mk_re_option.argtypes = [ContextObj, Ast]
_lib.Z3_mk_re_union.restype = Ast
_lib.Z3_mk_re_union.argtypes = [ContextObj, ctypes.c_uint, ctypes.POINTER(Ast)]
_lib.Z3_mk_re_concat.restype = Ast
_lib.Z3_mk_re_concat.argtypes = [ContextObj, ctypes.c_uint, ctypes.POINTER(Ast)]
_lib.Z3_mk_re_range.restype = Ast
_lib.Z3_mk_re_range.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_re_allchar.restype = Ast
_lib.Z3_mk_re_allchar.argtypes = [ContextObj, Sort]
_lib.Z3_mk_re_loop.restype = Ast
_lib.Z3_mk_re_loop.argtypes = [ContextObj, Ast, ctypes.c_uint, ctypes.c_uint]
_lib.Z3_mk_re_power.restype = Ast
_lib.Z3_mk_re_power.argtypes = [ContextObj, Ast, ctypes.c_uint]
_lib.Z3_mk_re_intersect.restype = Ast
_lib.Z3_mk_re_intersect.argtypes = [ContextObj, ctypes.c_uint, ctypes.POINTER(Ast)]
_lib.Z3_mk_re_complement.restype = Ast
_lib.Z3_mk_re_complement.argtypes = [ContextObj, Ast]
_lib.Z3_mk_re_diff.restype = Ast
_lib.Z3_mk_re_diff.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_re_empty.restype = Ast
_lib.Z3_mk_re_empty.argtypes = [ContextObj, Sort]
_lib.Z3_mk_re_full.restype = Ast
_lib.Z3_mk_re_full.argtypes = [ContextObj, Sort]
_lib.Z3_mk_char.restype = Ast
_lib.Z3_mk_char.argtypes = [ContextObj, ctypes.c_uint]
_lib.Z3_mk_char_le.restype = Ast
_lib.Z3_mk_char_le.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_char_to_int.restype = Ast
_lib.Z3_mk_char_to_int.argtypes = [ContextObj, Ast]
_lib.Z3_mk_char_to_bv.restype = Ast
_lib.Z3_mk_char_to_bv.argtypes = [ContextObj, Ast]
_lib.Z3_mk_char_from_bv.restype = Ast
_lib.Z3_mk_char_from_bv.argtypes = [ContextObj, Ast]
_lib.Z3_mk_char_is_digit.restype = Ast
_lib.Z3_mk_char_is_digit.argtypes = [ContextObj, Ast]
_lib.Z3_mk_linear_order.restype = FuncDecl
_lib.Z3_mk_linear_order.argtypes = [ContextObj, Sort, ctypes.c_uint]
_lib.Z3_mk_partial_order.restype = FuncDecl
_lib.Z3_mk_partial_order.argtypes = [ContextObj, Sort, ctypes.c_uint]
_lib.Z3_mk_piecewise_linear_order.restype = FuncDecl
_lib.Z3_mk_piecewise_linear_order.argtypes = [ContextObj, Sort, ctypes.c_uint]
_lib.Z3_mk_tree_order.restype = FuncDecl
_lib.Z3_mk_tree_order.argtypes = [ContextObj, Sort, ctypes.c_uint]
_lib.Z3_mk_transitive_closure.restype = FuncDecl
_lib.Z3_mk_transitive_closure.argtypes = [ContextObj, FuncDecl]
_lib.Z3_mk_pattern.restype = Pattern
_lib.Z3_mk_pattern.argtypes = [ContextObj, ctypes.c_uint, ctypes.POINTER(Ast)]
_lib.Z3_mk_bound.restype = Ast
_lib.Z3_mk_bound.argtypes = [ContextObj, ctypes.c_uint, Sort]
_lib.Z3_mk_forall.restype = Ast
_lib.Z3_mk_forall.argtypes = [ContextObj, ctypes.c_uint, ctypes.c_uint, ctypes.POINTER(Pattern), ctypes.c_uint, ctypes.POINTER(Sort), ctypes.POINTER(Symbol), Ast]
_lib.Z3_mk_exists.restype = Ast
_lib.Z3_mk_exists.argtypes = [ContextObj, ctypes.c_uint, ctypes.c_uint, ctypes.POINTER(Pattern), ctypes.c_uint, ctypes.POINTER(Sort), ctypes.POINTER(Symbol), Ast]
_lib.Z3_mk_quantifier.restype = Ast
_lib.Z3_mk_quantifier.argtypes = [ContextObj, ctypes.c_bool, ctypes.c_uint, ctypes.c_uint, ctypes.POINTER(Pattern), ctypes.c_uint, ctypes.POINTER(Sort), ctypes.POINTER(Symbol), Ast]
_lib.Z3_mk_quantifier_ex.restype = Ast
_lib.Z3_mk_quantifier_ex.argtypes = [ContextObj, ctypes.c_bool, ctypes.c_uint, Symbol, Symbol, ctypes.c_uint, ctypes.POINTER(Pattern), ctypes.c_uint, ctypes.POINTER(Ast), ctypes.c_uint, ctypes.POINTER(Sort), ctypes.POINTER(Symbol), Ast]
_lib.Z3_mk_forall_const.restype = Ast
_lib.Z3_mk_forall_const.argtypes = [ContextObj, ctypes.c_uint, ctypes.c_uint, ctypes.POINTER(Ast), ctypes.c_uint, ctypes.POINTER(Pattern), Ast]
_lib.Z3_mk_exists_const.restype = Ast
_lib.Z3_mk_exists_const.argtypes = [ContextObj, ctypes.c_uint, ctypes.c_uint, ctypes.POINTER(Ast), ctypes.c_uint, ctypes.POINTER(Pattern), Ast]
_lib.Z3_mk_quantifier_const.restype = Ast
_lib.Z3_mk_quantifier_const.argtypes = [ContextObj, ctypes.c_bool, ctypes.c_uint, ctypes.c_uint, ctypes.POINTER(Ast), ctypes.c_uint, ctypes.POINTER(Pattern), Ast]
_lib.Z3_mk_quantifier_const_ex.restype = Ast
_lib.Z3_mk_quantifier_const_ex.argtypes = [ContextObj, ctypes.c_bool, ctypes.c_uint, Symbol, Symbol, ctypes.c_uint, ctypes.POINTER(Ast), ctypes.c_uint, ctypes.POINTER(Pattern), ctypes.c_uint, ctypes.POINTER(Ast), Ast]
_lib.Z3_mk_lambda.restype = Ast
_lib.Z3_mk_lambda.argtypes = [ContextObj, ctypes.c_uint, ctypes.POINTER(Sort), ctypes.POINTER(Symbol), Ast]
_lib.Z3_mk_lambda_const.restype = Ast
_lib.Z3_mk_lambda_const.argtypes = [ContextObj, ctypes.c_uint, ctypes.POINTER(Ast), Ast]
_lib.Z3_get_symbol_kind.restype = ctypes.c_uint
_lib.Z3_get_symbol_kind.argtypes = [ContextObj, Symbol]
_lib.Z3_get_symbol_int.restype = ctypes.c_int
_lib.Z3_get_symbol_int.argtypes = [ContextObj, Symbol]
_lib.Z3_get_symbol_string.restype = ctypes.c_char_p
_lib.Z3_get_symbol_string.argtypes = [ContextObj, Symbol]
_lib.Z3_get_sort_name.restype = Symbol
_lib.Z3_get_sort_name.argtypes = [ContextObj, Sort]
_lib.Z3_get_sort_id.restype = ctypes.c_uint
_lib.Z3_get_sort_id.argtypes = [ContextObj, Sort]
_lib.Z3_sort_to_ast.restype = Ast
_lib.Z3_sort_to_ast.argtypes = [ContextObj, Sort]
_lib.Z3_is_eq_sort.restype = ctypes.c_bool
_lib.Z3_is_eq_sort.argtypes = [ContextObj, Sort, Sort]
_lib.Z3_get_sort_kind.restype = ctypes.c_uint
_lib.Z3_get_sort_kind.argtypes = [ContextObj, Sort]
_lib.Z3_get_bv_sort_size.restype = ctypes.c_uint
_lib.Z3_get_bv_sort_size.argtypes = [ContextObj, Sort]
_lib.Z3_get_finite_domain_sort_size.restype = ctypes.c_bool
_lib.Z3_get_finite_domain_sort_size.argtypes = [ContextObj, Sort, ctypes.POINTER(ctypes.c_ulonglong)]
_lib.Z3_get_array_sort_domain.restype = Sort
_lib.Z3_get_array_sort_domain.argtypes = [ContextObj, Sort]
_lib.Z3_get_array_sort_domain_n.restype = Sort
_lib.Z3_get_array_sort_domain_n.argtypes = [ContextObj, Sort, ctypes.c_uint]
_lib.Z3_get_array_sort_range.restype = Sort
_lib.Z3_get_array_sort_range.argtypes = [ContextObj, Sort]
_lib.Z3_get_tuple_sort_mk_decl.restype = FuncDecl
_lib.Z3_get_tuple_sort_mk_decl.argtypes = [ContextObj, Sort]
_lib.Z3_get_tuple_sort_num_fields.restype = ctypes.c_uint
_lib.Z3_get_tuple_sort_num_fields.argtypes = [ContextObj, Sort]
_lib.Z3_get_tuple_sort_field_decl.restype = FuncDecl
_lib.Z3_get_tuple_sort_field_decl.argtypes = [ContextObj, Sort, ctypes.c_uint]
_lib.Z3_get_datatype_sort_num_constructors.restype = ctypes.c_uint
_lib.Z3_get_datatype_sort_num_constructors.argtypes = [ContextObj, Sort]
_lib.Z3_get_datatype_sort_constructor.restype = FuncDecl
_lib.Z3_get_datatype_sort_constructor.argtypes = [ContextObj, Sort, ctypes.c_uint]
_lib.Z3_get_datatype_sort_recognizer.restype = FuncDecl
_lib.Z3_get_datatype_sort_recognizer.argtypes = [ContextObj, Sort, ctypes.c_uint]
_lib.Z3_get_datatype_sort_constructor_accessor.restype = FuncDecl
_lib.Z3_get_datatype_sort_constructor_accessor.argtypes = [ContextObj, Sort, ctypes.c_uint, ctypes.c_uint]
_lib.Z3_datatype_update_field.restype = Ast
_lib.Z3_datatype_update_field.argtypes = [ContextObj, FuncDecl, Ast, Ast]
_lib.Z3_get_relation_arity.restype = ctypes.c_uint
_lib.Z3_get_relation_arity.argtypes = [ContextObj, Sort]
_lib.Z3_get_relation_column.restype = Sort
_lib.Z3_get_relation_column.argtypes = [ContextObj, Sort, ctypes.c_uint]
_lib.Z3_mk_atmost.restype = Ast
_lib.Z3_mk_atmost.argtypes = [ContextObj, ctypes.c_uint, ctypes.POINTER(Ast), ctypes.c_uint]
_lib.Z3_mk_atleast.restype = Ast
_lib.Z3_mk_atleast.argtypes = [ContextObj, ctypes.c_uint, ctypes.POINTER(Ast), ctypes.c_uint]
_lib.Z3_mk_pble.restype = Ast
_lib.Z3_mk_pble.argtypes = [ContextObj, ctypes.c_uint, ctypes.POINTER(Ast), ctypes.POINTER(ctypes.c_int), ctypes.c_int]
_lib.Z3_mk_pbge.restype = Ast
_lib.Z3_mk_pbge.argtypes = [ContextObj, ctypes.c_uint, ctypes.POINTER(Ast), ctypes.POINTER(ctypes.c_int), ctypes.c_int]
_lib.Z3_mk_pbeq.restype = Ast
_lib.Z3_mk_pbeq.argtypes = [ContextObj, ctypes.c_uint, ctypes.POINTER(Ast), ctypes.POINTER(ctypes.c_int), ctypes.c_int]
_lib.Z3_func_decl_to_ast.restype = Ast
_lib.Z3_func_decl_to_ast.argtypes = [ContextObj, FuncDecl]
_lib.Z3_is_eq_func_decl.restype = ctypes.c_bool
_lib.Z3_is_eq_func_decl.argtypes = [ContextObj, FuncDecl, FuncDecl]
_lib.Z3_get_func_decl_id.restype = ctypes.c_uint
_lib.Z3_get_func_decl_id.argtypes = [ContextObj, FuncDecl]
_lib.Z3_get_decl_name.restype = Symbol
_lib.Z3_get_decl_name.argtypes = [ContextObj, FuncDecl]
_lib.Z3_get_decl_kind.restype = ctypes.c_uint
_lib.Z3_get_decl_kind.argtypes = [ContextObj, FuncDecl]
_lib.Z3_get_domain_size.restype = ctypes.c_uint
_lib.Z3_get_domain_size.argtypes = [ContextObj, FuncDecl]
_lib.Z3_get_arity.restype = ctypes.c_uint
_lib.Z3_get_arity.argtypes = [ContextObj, FuncDecl]
_lib.Z3_get_domain.restype = Sort
_lib.Z3_get_domain.argtypes = [ContextObj, FuncDecl, ctypes.c_uint]
_lib.Z3_get_range.restype = Sort
_lib.Z3_get_range.argtypes = [ContextObj, FuncDecl]
_lib.Z3_get_decl_num_parameters.restype = ctypes.c_uint
_lib.Z3_get_decl_num_parameters.argtypes = [ContextObj, FuncDecl]
_lib.Z3_get_decl_parameter_kind.restype = ctypes.c_uint
_lib.Z3_get_decl_parameter_kind.argtypes = [ContextObj, FuncDecl, ctypes.c_uint]
_lib.Z3_get_decl_int_parameter.restype = ctypes.c_int
_lib.Z3_get_decl_int_parameter.argtypes = [ContextObj, FuncDecl, ctypes.c_uint]
_lib.Z3_get_decl_double_parameter.restype = ctypes.c_double
_lib.Z3_get_decl_double_parameter.argtypes = [ContextObj, FuncDecl, ctypes.c_uint]
_lib.Z3_get_decl_symbol_parameter.restype = Symbol
_lib.Z3_get_decl_symbol_parameter.argtypes = [ContextObj, FuncDecl, ctypes.c_uint]
_lib.Z3_get_decl_sort_parameter.restype = Sort
_lib.Z3_get_decl_sort_parameter.argtypes = [ContextObj, FuncDecl, ctypes.c_uint]
_lib.Z3_get_decl_ast_parameter.restype = Ast
_lib.Z3_get_decl_ast_parameter.argtypes = [ContextObj, FuncDecl, ctypes.c_uint]
_lib.Z3_get_decl_func_decl_parameter.restype = FuncDecl
_lib.Z3_get_decl_func_decl_parameter.argtypes = [ContextObj, FuncDecl, ctypes.c_uint]
_lib.Z3_get_decl_rational_parameter.restype = ctypes.c_char_p
_lib.Z3_get_decl_rational_parameter.argtypes = [ContextObj, FuncDecl, ctypes.c_uint]
_lib.Z3_app_to_ast.restype = Ast
_lib.Z3_app_to_ast.argtypes = [ContextObj, Ast]
_lib.Z3_get_app_decl.restype = FuncDecl
_lib.Z3_get_app_decl.argtypes = [ContextObj, Ast]
_lib.Z3_get_app_num_args.restype = ctypes.c_uint
_lib.Z3_get_app_num_args.argtypes = [ContextObj, Ast]
_lib.Z3_get_app_arg.restype = Ast
_lib.Z3_get_app_arg.argtypes = [ContextObj, Ast, ctypes.c_uint]
_lib.Z3_is_eq_ast.restype = ctypes.c_bool
_lib.Z3_is_eq_ast.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_get_ast_id.restype = ctypes.c_uint
_lib.Z3_get_ast_id.argtypes = [ContextObj, Ast]
_lib.Z3_get_ast_hash.restype = ctypes.c_uint
_lib.Z3_get_ast_hash.argtypes = [ContextObj, Ast]
_lib.Z3_get_sort.restype = Sort
_lib.Z3_get_sort.argtypes = [ContextObj, Ast]
_lib.Z3_is_well_sorted.restype = ctypes.c_bool
_lib.Z3_is_well_sorted.argtypes = [ContextObj, Ast]
_lib.Z3_get_bool_value.restype = ctypes.c_int
_lib.Z3_get_bool_value.argtypes = [ContextObj, Ast]
_lib.Z3_get_ast_kind.restype = ctypes.c_uint
_lib.Z3_get_ast_kind.argtypes = [ContextObj, Ast]
_lib.Z3_is_app.restype = ctypes.c_bool
_lib.Z3_is_app.argtypes = [ContextObj, Ast]
_lib.Z3_is_numeral_ast.restype = ctypes.c_bool
_lib.Z3_is_numeral_ast.argtypes = [ContextObj, Ast]
_lib.Z3_is_algebraic_number.restype = ctypes.c_bool
_lib.Z3_is_algebraic_number.argtypes = [ContextObj, Ast]
_lib.Z3_to_app.restype = Ast
_lib.Z3_to_app.argtypes = [ContextObj, Ast]
_lib.Z3_to_func_decl.restype = FuncDecl
_lib.Z3_to_func_decl.argtypes = [ContextObj, Ast]
_lib.Z3_get_numeral_string.restype = ctypes.c_char_p
_lib.Z3_get_numeral_string.argtypes = [ContextObj, Ast]
_lib.Z3_get_numeral_binary_string.restype = ctypes.c_char_p
_lib.Z3_get_numeral_binary_string.argtypes = [ContextObj, Ast]
_lib.Z3_get_numeral_decimal_string.restype = ctypes.c_char_p
_lib.Z3_get_numeral_decimal_string.argtypes = [ContextObj, Ast, ctypes.c_uint]
_lib.Z3_get_numeral_double.restype = ctypes.c_double
_lib.Z3_get_numeral_double.argtypes = [ContextObj, Ast]
_lib.Z3_get_numerator.restype = Ast
_lib.Z3_get_numerator.argtypes = [ContextObj, Ast]
_lib.Z3_get_denominator.restype = Ast
_lib.Z3_get_denominator.argtypes = [ContextObj, Ast]
_lib.Z3_get_numeral_small.restype = ctypes.c_bool
_lib.Z3_get_numeral_small.argtypes = [ContextObj, Ast, ctypes.POINTER(ctypes.c_longlong), ctypes.POINTER(ctypes.c_longlong)]
_lib.Z3_get_numeral_int.restype = ctypes.c_bool
_lib.Z3_get_numeral_int.argtypes = [ContextObj, Ast, ctypes.POINTER(ctypes.c_int)]
_lib.Z3_get_numeral_uint.restype = ctypes.c_bool
_lib.Z3_get_numeral_uint.argtypes = [ContextObj, Ast, ctypes.POINTER(ctypes.c_uint)]
_lib.Z3_get_numeral_uint64.restype = ctypes.c_bool
_lib.Z3_get_numeral_uint64.argtypes = [ContextObj, Ast, ctypes.POINTER(ctypes.c_ulonglong)]
_lib.Z3_get_numeral_int64.restype = ctypes.c_bool
_lib.Z3_get_numeral_int64.argtypes = [ContextObj, Ast, ctypes.POINTER(ctypes.c_longlong)]
_lib.Z3_get_numeral_rational_int64.restype = ctypes.c_bool
_lib.Z3_get_numeral_rational_int64.argtypes = [ContextObj, Ast, ctypes.POINTER(ctypes.c_longlong), ctypes.POINTER(ctypes.c_longlong)]
_lib.Z3_get_algebraic_number_lower.restype = Ast
_lib.Z3_get_algebraic_number_lower.argtypes = [ContextObj, Ast, ctypes.c_uint]
_lib.Z3_get_algebraic_number_upper.restype = Ast
_lib.Z3_get_algebraic_number_upper.argtypes = [ContextObj, Ast, ctypes.c_uint]
_lib.Z3_pattern_to_ast.restype = Ast
_lib.Z3_pattern_to_ast.argtypes = [ContextObj, Pattern]
_lib.Z3_get_pattern_num_terms.restype = ctypes.c_uint
_lib.Z3_get_pattern_num_terms.argtypes = [ContextObj, Pattern]
_lib.Z3_get_pattern.restype = Ast
_lib.Z3_get_pattern.argtypes = [ContextObj, Pattern, ctypes.c_uint]
_lib.Z3_get_index_value.restype = ctypes.c_uint
_lib.Z3_get_index_value.argtypes = [ContextObj, Ast]
_lib.Z3_is_quantifier_forall.restype = ctypes.c_bool
_lib.Z3_is_quantifier_forall.argtypes = [ContextObj, Ast]
_lib.Z3_is_quantifier_exists.restype = ctypes.c_bool
_lib.Z3_is_quantifier_exists.argtypes = [ContextObj, Ast]
_lib.Z3_is_lambda.restype = ctypes.c_bool
_lib.Z3_is_lambda.argtypes = [ContextObj, Ast]
_lib.Z3_get_quantifier_weight.restype = ctypes.c_uint
_lib.Z3_get_quantifier_weight.argtypes = [ContextObj, Ast]
_lib.Z3_get_quantifier_skolem_id.restype = Symbol
_lib.Z3_get_quantifier_skolem_id.argtypes = [ContextObj, Ast]
_lib.Z3_get_quantifier_id.restype = Symbol
_lib.Z3_get_quantifier_id.argtypes = [ContextObj, Ast]
_lib.Z3_get_quantifier_num_patterns.restype = ctypes.c_uint
_lib.Z3_get_quantifier_num_patterns.argtypes = [ContextObj, Ast]
_lib.Z3_get_quantifier_pattern_ast.restype = Pattern
_lib.Z3_get_quantifier_pattern_ast.argtypes = [ContextObj, Ast, ctypes.c_uint]
_lib.Z3_get_quantifier_num_no_patterns.restype = ctypes.c_uint
_lib.Z3_get_quantifier_num_no_patterns.argtypes = [ContextObj, Ast]
_lib.Z3_get_quantifier_no_pattern_ast.restype = Ast
_lib.Z3_get_quantifier_no_pattern_ast.argtypes = [ContextObj, Ast, ctypes.c_uint]
_lib.Z3_get_quantifier_num_bound.restype = ctypes.c_uint
_lib.Z3_get_quantifier_num_bound.argtypes = [ContextObj, Ast]
_lib.Z3_get_quantifier_bound_name.restype = Symbol
_lib.Z3_get_quantifier_bound_name.argtypes = [ContextObj, Ast, ctypes.c_uint]
_lib.Z3_get_quantifier_bound_sort.restype = Sort
_lib.Z3_get_quantifier_bound_sort.argtypes = [ContextObj, Ast, ctypes.c_uint]
_lib.Z3_get_quantifier_body.restype = Ast
_lib.Z3_get_quantifier_body.argtypes = [ContextObj, Ast]
_lib.Z3_simplify.restype = Ast
_lib.Z3_simplify.argtypes = [ContextObj, Ast]
_lib.Z3_simplify_ex.restype = Ast
_lib.Z3_simplify_ex.argtypes = [ContextObj, Ast, Params]
_lib.Z3_simplify_get_help.restype = ctypes.c_char_p
_lib.Z3_simplify_get_help.argtypes = [ContextObj]
_lib.Z3_simplify_get_param_descrs.restype = ParamDescrs
_lib.Z3_simplify_get_param_descrs.argtypes = [ContextObj]
_lib.Z3_update_term.restype = Ast
_lib.Z3_update_term.argtypes = [ContextObj, Ast, ctypes.c_uint, ctypes.POINTER(Ast)]
_lib.Z3_substitute.restype = Ast
_lib.Z3_substitute.argtypes = [ContextObj, Ast, ctypes.c_uint, ctypes.POINTER(Ast), ctypes.POINTER(Ast)]
_lib.Z3_substitute_vars.restype = Ast
_lib.Z3_substitute_vars.argtypes = [ContextObj, Ast, ctypes.c_uint, ctypes.POINTER(Ast)]
_lib.Z3_substitute_funs.restype = Ast
_lib.Z3_substitute_funs.argtypes = [ContextObj, Ast, ctypes.c_uint, ctypes.POINTER(FuncDecl), ctypes.POINTER(Ast)]
_lib.Z3_translate.restype = Ast
_lib.Z3_translate.argtypes = [ContextObj, Ast, ContextObj]
_lib.Z3_mk_model.restype = ModelObj
_lib.Z3_mk_model.argtypes = [ContextObj]
_lib.Z3_model_inc_ref.argtypes = [ContextObj, ModelObj]
_lib.Z3_model_dec_ref.argtypes = [ContextObj, ModelObj]
_lib.Z3_model_eval.restype = ctypes.c_bool
_lib.Z3_model_eval.argtypes = [ContextObj, ModelObj, Ast, ctypes.c_bool, ctypes.POINTER(Ast)]
_lib.Z3_model_get_const_interp.restype = Ast
_lib.Z3_model_get_const_interp.argtypes = [ContextObj, ModelObj, FuncDecl]
_lib.Z3_model_has_interp.restype = ctypes.c_bool
_lib.Z3_model_has_interp.argtypes = [ContextObj, ModelObj, FuncDecl]
_lib.Z3_model_get_func_interp.restype = FuncInterpObj
_lib.Z3_model_get_func_interp.argtypes = [ContextObj, ModelObj, FuncDecl]
_lib.Z3_model_get_num_consts.restype = ctypes.c_uint
_lib.Z3_model_get_num_consts.argtypes = [ContextObj, ModelObj]
_lib.Z3_model_get_const_decl.restype = FuncDecl
_lib.Z3_model_get_const_decl.argtypes = [ContextObj, ModelObj, ctypes.c_uint]
_lib.Z3_model_get_num_funcs.restype = ctypes.c_uint
_lib.Z3_model_get_num_funcs.argtypes = [ContextObj, ModelObj]
_lib.Z3_model_get_func_decl.restype = FuncDecl
_lib.Z3_model_get_func_decl.argtypes = [ContextObj, ModelObj, ctypes.c_uint]
_lib.Z3_model_get_num_sorts.restype = ctypes.c_uint
_lib.Z3_model_get_num_sorts.argtypes = [ContextObj, ModelObj]
_lib.Z3_model_get_sort.restype = Sort
_lib.Z3_model_get_sort.argtypes = [ContextObj, ModelObj, ctypes.c_uint]
_lib.Z3_model_get_sort_universe.restype = AstVectorObj
_lib.Z3_model_get_sort_universe.argtypes = [ContextObj, ModelObj, Sort]
_lib.Z3_model_translate.restype = ModelObj
_lib.Z3_model_translate.argtypes = [ContextObj, ModelObj, ContextObj]
_lib.Z3_is_as_array.restype = ctypes.c_bool
_lib.Z3_is_as_array.argtypes = [ContextObj, Ast]
_lib.Z3_get_as_array_func_decl.restype = FuncDecl
_lib.Z3_get_as_array_func_decl.argtypes = [ContextObj, Ast]
_lib.Z3_add_func_interp.restype = FuncInterpObj
_lib.Z3_add_func_interp.argtypes = [ContextObj, ModelObj, FuncDecl, Ast]
_lib.Z3_add_const_interp.argtypes = [ContextObj, ModelObj, FuncDecl, Ast]
_lib.Z3_func_interp_inc_ref.argtypes = [ContextObj, FuncInterpObj]
_lib.Z3_func_interp_dec_ref.argtypes = [ContextObj, FuncInterpObj]
_lib.Z3_func_interp_get_num_entries.restype = ctypes.c_uint
_lib.Z3_func_interp_get_num_entries.argtypes = [ContextObj, FuncInterpObj]
_lib.Z3_func_interp_get_entry.restype = FuncEntryObj
_lib.Z3_func_interp_get_entry.argtypes = [ContextObj, FuncInterpObj, ctypes.c_uint]
_lib.Z3_func_interp_get_else.restype = Ast
_lib.Z3_func_interp_get_else.argtypes = [ContextObj, FuncInterpObj]
_lib.Z3_func_interp_set_else.argtypes = [ContextObj, FuncInterpObj, Ast]
_lib.Z3_func_interp_get_arity.restype = ctypes.c_uint
_lib.Z3_func_interp_get_arity.argtypes = [ContextObj, FuncInterpObj]
_lib.Z3_func_interp_add_entry.argtypes = [ContextObj, FuncInterpObj, AstVectorObj, Ast]
_lib.Z3_func_entry_inc_ref.argtypes = [ContextObj, FuncEntryObj]
_lib.Z3_func_entry_dec_ref.argtypes = [ContextObj, FuncEntryObj]
_lib.Z3_func_entry_get_value.restype = Ast
_lib.Z3_func_entry_get_value.argtypes = [ContextObj, FuncEntryObj]
_lib.Z3_func_entry_get_num_args.restype = ctypes.c_uint
_lib.Z3_func_entry_get_num_args.argtypes = [ContextObj, FuncEntryObj]
_lib.Z3_func_entry_get_arg.restype = Ast
_lib.Z3_func_entry_get_arg.argtypes = [ContextObj, FuncEntryObj, ctypes.c_uint]
_lib.Z3_open_log.restype = ctypes.c_int
_lib.Z3_open_log.argtypes = [ctypes.c_char_p]
_lib.Z3_append_log.argtypes = [ctypes.c_char_p]
_lib.Z3_close_log.argtypes = []
_lib.Z3_toggle_warning_messages.argtypes = [ctypes.c_bool]
_lib.Z3_set_ast_print_mode.argtypes = [ContextObj, ctypes.c_uint]
_lib.Z3_ast_to_string.restype = ctypes.c_char_p
_lib.Z3_ast_to_string.argtypes = [ContextObj, Ast]
_lib.Z3_pattern_to_string.restype = ctypes.c_char_p
_lib.Z3_pattern_to_string.argtypes = [ContextObj, Pattern]
_lib.Z3_sort_to_string.restype = ctypes.c_char_p
_lib.Z3_sort_to_string.argtypes = [ContextObj, Sort]
_lib.Z3_func_decl_to_string.restype = ctypes.c_char_p
_lib.Z3_func_decl_to_string.argtypes = [ContextObj, FuncDecl]
_lib.Z3_model_to_string.restype = ctypes.c_char_p
_lib.Z3_model_to_string.argtypes = [ContextObj, ModelObj]
_lib.Z3_benchmark_to_smtlib_string.restype = ctypes.c_char_p
_lib.Z3_benchmark_to_smtlib_string.argtypes = [ContextObj, ctypes.c_char_p, ctypes.c_char_p, ctypes.c_char_p, ctypes.c_char_p, ctypes.c_uint, ctypes.POINTER(Ast), Ast]
_lib.Z3_parse_smtlib2_string.restype = AstVectorObj
_lib.Z3_parse_smtlib2_string.argtypes = [ContextObj, ctypes.c_char_p, ctypes.c_uint, ctypes.POINTER(Symbol), ctypes.POINTER(Sort), ctypes.c_uint, ctypes.POINTER(Symbol), ctypes.POINTER(FuncDecl)]
_lib.Z3_parse_smtlib2_file.restype = AstVectorObj
_lib.Z3_parse_smtlib2_file.argtypes = [ContextObj, ctypes.c_char_p, ctypes.c_uint, ctypes.POINTER(Symbol), ctypes.POINTER(Sort), ctypes.c_uint, ctypes.POINTER(Symbol), ctypes.POINTER(FuncDecl)]
_lib.Z3_eval_smtlib2_string.restype = ctypes.c_char_p
_lib.Z3_eval_smtlib2_string.argtypes = [ContextObj, ctypes.c_char_p]
_lib.Z3_mk_parser_context.restype = ParserContextObj
_lib.Z3_mk_parser_context.argtypes = [ContextObj]
_lib.Z3_parser_context_inc_ref.argtypes = [ContextObj, ParserContextObj]
_lib.Z3_parser_context_dec_ref.argtypes = [ContextObj, ParserContextObj]
_lib.Z3_parser_context_add_sort.argtypes = [ContextObj, ParserContextObj, Sort]
_lib.Z3_parser_context_add_decl.argtypes = [ContextObj, ParserContextObj, FuncDecl]
_lib.Z3_parser_context_from_string.restype = AstVectorObj
_lib.Z3_parser_context_from_string.argtypes = [ContextObj, ParserContextObj, ctypes.c_char_p]
_lib.Z3_get_error_code.restype = ctypes.c_uint
_lib.Z3_get_error_code.argtypes = [ContextObj]
_lib.Z3_set_error.argtypes = [ContextObj, ctypes.c_uint]
_lib.Z3_get_error_msg.restype = ctypes.c_char_p
_lib.Z3_get_error_msg.argtypes = [ContextObj, ctypes.c_uint]
_lib.Z3_get_version.argtypes = [ctypes.POINTER(ctypes.c_uint), ctypes.POINTER(ctypes.c_uint), ctypes.POINTER(ctypes.c_uint), ctypes.POINTER(ctypes.c_uint)]
_lib.Z3_get_full_version.restype = ctypes.c_char_p
_lib.Z3_get_full_version.argtypes = []
_lib.Z3_enable_trace.argtypes = [ctypes.c_char_p]
_lib.Z3_disable_trace.argtypes = [ctypes.c_char_p]
_lib.Z3_reset_memory.argtypes = []
_lib.Z3_finalize_memory.argtypes = []
_lib.Z3_mk_goal.restype = GoalObj
_lib.Z3_mk_goal.argtypes = [ContextObj, ctypes.c_bool, ctypes.c_bool, ctypes.c_bool]
_lib.Z3_goal_inc_ref.argtypes = [ContextObj, GoalObj]
_lib.Z3_goal_dec_ref.argtypes = [ContextObj, GoalObj]
_lib.Z3_goal_precision.restype = ctypes.c_uint
_lib.Z3_goal_precision.argtypes = [ContextObj, GoalObj]
_lib.Z3_goal_assert.argtypes = [ContextObj, GoalObj, Ast]
_lib.Z3_goal_inconsistent.restype = ctypes.c_bool
_lib.Z3_goal_inconsistent.argtypes = [ContextObj, GoalObj]
_lib.Z3_goal_depth.restype = ctypes.c_uint
_lib.Z3_goal_depth.argtypes = [ContextObj, GoalObj]
_lib.Z3_goal_reset.argtypes = [ContextObj, GoalObj]
_lib.Z3_goal_size.restype = ctypes.c_uint
_lib.Z3_goal_size.argtypes = [ContextObj, GoalObj]
_lib.Z3_goal_formula.restype = Ast
_lib.Z3_goal_formula.argtypes = [ContextObj, GoalObj, ctypes.c_uint]
_lib.Z3_goal_num_exprs.restype = ctypes.c_uint
_lib.Z3_goal_num_exprs.argtypes = [ContextObj, GoalObj]
_lib.Z3_goal_is_decided_sat.restype = ctypes.c_bool
_lib.Z3_goal_is_decided_sat.argtypes = [ContextObj, GoalObj]
_lib.Z3_goal_is_decided_unsat.restype = ctypes.c_bool
_lib.Z3_goal_is_decided_unsat.argtypes = [ContextObj, GoalObj]
_lib.Z3_goal_translate.restype = GoalObj
_lib.Z3_goal_translate.argtypes = [ContextObj, GoalObj, ContextObj]
_lib.Z3_goal_convert_model.restype = ModelObj
_lib.Z3_goal_convert_model.argtypes = [ContextObj, GoalObj, ModelObj]
_lib.Z3_goal_to_string.restype = ctypes.c_char_p
_lib.Z3_goal_to_string.argtypes = [ContextObj, GoalObj]
_lib.Z3_goal_to_dimacs_string.restype = ctypes.c_char_p
_lib.Z3_goal_to_dimacs_string.argtypes = [ContextObj, GoalObj, ctypes.c_bool]
_lib.Z3_mk_tactic.restype = TacticObj
_lib.Z3_mk_tactic.argtypes = [ContextObj, ctypes.c_char_p]
_lib.Z3_tactic_inc_ref.argtypes = [ContextObj, TacticObj]
_lib.Z3_tactic_dec_ref.argtypes = [ContextObj, TacticObj]
_lib.Z3_mk_probe.restype = ProbeObj
_lib.Z3_mk_probe.argtypes = [ContextObj, ctypes.c_char_p]
_lib.Z3_probe_inc_ref.argtypes = [ContextObj, ProbeObj]
_lib.Z3_probe_dec_ref.argtypes = [ContextObj, ProbeObj]
_lib.Z3_tactic_and_then.restype = TacticObj
_lib.Z3_tactic_and_then.argtypes = [ContextObj, TacticObj, TacticObj]
_lib.Z3_tactic_or_else.restype = TacticObj
_lib.Z3_tactic_or_else.argtypes = [ContextObj, TacticObj, TacticObj]
_lib.Z3_tactic_par_or.restype = TacticObj
_lib.Z3_tactic_par_or.argtypes = [ContextObj, ctypes.c_uint, ctypes.POINTER(TacticObj)]
_lib.Z3_tactic_par_and_then.restype = TacticObj
_lib.Z3_tactic_par_and_then.argtypes = [ContextObj, TacticObj, TacticObj]
_lib.Z3_tactic_try_for.restype = TacticObj
_lib.Z3_tactic_try_for.argtypes = [ContextObj, TacticObj, ctypes.c_uint]
_lib.Z3_tactic_when.restype = TacticObj
_lib.Z3_tactic_when.argtypes = [ContextObj, ProbeObj, TacticObj]
_lib.Z3_tactic_cond.restype = TacticObj
_lib.Z3_tactic_cond.argtypes = [ContextObj, ProbeObj, TacticObj, TacticObj]
_lib.Z3_tactic_repeat.restype = TacticObj
_lib.Z3_tactic_repeat.argtypes = [ContextObj, TacticObj, ctypes.c_uint]
_lib.Z3_tactic_skip.restype = TacticObj
_lib.Z3_tactic_skip.argtypes = [ContextObj]
_lib.Z3_tactic_fail.restype = TacticObj
_lib.Z3_tactic_fail.argtypes = [ContextObj]
_lib.Z3_tactic_fail_if.restype = TacticObj
_lib.Z3_tactic_fail_if.argtypes = [ContextObj, ProbeObj]
_lib.Z3_tactic_fail_if_not_decided.restype = TacticObj
_lib.Z3_tactic_fail_if_not_decided.argtypes = [ContextObj]
_lib.Z3_tactic_using_params.restype = TacticObj
_lib.Z3_tactic_using_params.argtypes = [ContextObj, TacticObj, Params]
_lib.Z3_mk_simplifier.restype = SimplifierObj
_lib.Z3_mk_simplifier.argtypes = [ContextObj, ctypes.c_char_p]
_lib.Z3_simplifier_inc_ref.argtypes = [ContextObj, SimplifierObj]
_lib.Z3_simplifier_dec_ref.argtypes = [ContextObj, SimplifierObj]
_lib.Z3_solver_add_simplifier.restype = SolverObj
_lib.Z3_solver_add_simplifier.argtypes = [ContextObj, SolverObj, SimplifierObj]
_lib.Z3_simplifier_and_then.restype = SimplifierObj
_lib.Z3_simplifier_and_then.argtypes = [ContextObj, SimplifierObj, SimplifierObj]
_lib.Z3_simplifier_using_params.restype = SimplifierObj
_lib.Z3_simplifier_using_params.argtypes = [ContextObj, SimplifierObj, Params]
_lib.Z3_get_num_simplifiers.restype = ctypes.c_uint
_lib.Z3_get_num_simplifiers.argtypes = [ContextObj]
_lib.Z3_get_simplifier_name.restype = ctypes.c_char_p
_lib.Z3_get_simplifier_name.argtypes = [ContextObj, ctypes.c_uint]
_lib.Z3_simplifier_get_help.restype = ctypes.c_char_p
_lib.Z3_simplifier_get_help.argtypes = [ContextObj, SimplifierObj]
_lib.Z3_simplifier_get_param_descrs.restype = ParamDescrs
_lib.Z3_simplifier_get_param_descrs.argtypes = [ContextObj, SimplifierObj]
_lib.Z3_simplifier_get_descr.restype = ctypes.c_char_p
_lib.Z3_simplifier_get_descr.argtypes = [ContextObj, ctypes.c_char_p]
_lib.Z3_probe_const.restype = ProbeObj
_lib.Z3_probe_const.argtypes = [ContextObj, ctypes.c_double]
_lib.Z3_probe_lt.restype = ProbeObj
_lib.Z3_probe_lt.argtypes = [ContextObj, ProbeObj, ProbeObj]
_lib.Z3_probe_gt.restype = ProbeObj
_lib.Z3_probe_gt.argtypes = [ContextObj, ProbeObj, ProbeObj]
_lib.Z3_probe_le.restype = ProbeObj
_lib.Z3_probe_le.argtypes = [ContextObj, ProbeObj, ProbeObj]
_lib.Z3_probe_ge.restype = ProbeObj
_lib.Z3_probe_ge.argtypes = [ContextObj, ProbeObj, ProbeObj]
_lib.Z3_probe_eq.restype = ProbeObj
_lib.Z3_probe_eq.argtypes = [ContextObj, ProbeObj, ProbeObj]
_lib.Z3_probe_and.restype = ProbeObj
_lib.Z3_probe_and.argtypes = [ContextObj, ProbeObj, ProbeObj]
_lib.Z3_probe_or.restype = ProbeObj
_lib.Z3_probe_or.argtypes = [ContextObj, ProbeObj, ProbeObj]
_lib.Z3_probe_not.restype = ProbeObj
_lib.Z3_probe_not.argtypes = [ContextObj, ProbeObj]
_lib.Z3_get_num_tactics.restype = ctypes.c_uint
_lib.Z3_get_num_tactics.argtypes = [ContextObj]
_lib.Z3_get_tactic_name.restype = ctypes.c_char_p
_lib.Z3_get_tactic_name.argtypes = [ContextObj, ctypes.c_uint]
_lib.Z3_get_num_probes.restype = ctypes.c_uint
_lib.Z3_get_num_probes.argtypes = [ContextObj]
_lib.Z3_get_probe_name.restype = ctypes.c_char_p
_lib.Z3_get_probe_name.argtypes = [ContextObj, ctypes.c_uint]
_lib.Z3_tactic_get_help.restype = ctypes.c_char_p
_lib.Z3_tactic_get_help.argtypes = [ContextObj, TacticObj]
_lib.Z3_tactic_get_param_descrs.restype = ParamDescrs
_lib.Z3_tactic_get_param_descrs.argtypes = [ContextObj, TacticObj]
_lib.Z3_tactic_get_descr.restype = ctypes.c_char_p
_lib.Z3_tactic_get_descr.argtypes = [ContextObj, ctypes.c_char_p]
_lib.Z3_probe_get_descr.restype = ctypes.c_char_p
_lib.Z3_probe_get_descr.argtypes = [ContextObj, ctypes.c_char_p]
_lib.Z3_probe_apply.restype = ctypes.c_double
_lib.Z3_probe_apply.argtypes = [ContextObj, ProbeObj, GoalObj]
_lib.Z3_tactic_apply.restype = ApplyResultObj
_lib.Z3_tactic_apply.argtypes = [ContextObj, TacticObj, GoalObj]
_lib.Z3_tactic_apply_ex.restype = ApplyResultObj
_lib.Z3_tactic_apply_ex.argtypes = [ContextObj, TacticObj, GoalObj, Params]
_lib.Z3_apply_result_inc_ref.argtypes = [ContextObj, ApplyResultObj]
_lib.Z3_apply_result_dec_ref.argtypes = [ContextObj, ApplyResultObj]
_lib.Z3_apply_result_to_string.restype = ctypes.c_char_p
_lib.Z3_apply_result_to_string.argtypes = [ContextObj, ApplyResultObj]
_lib.Z3_apply_result_get_num_subgoals.restype = ctypes.c_uint
_lib.Z3_apply_result_get_num_subgoals.argtypes = [ContextObj, ApplyResultObj]
_lib.Z3_apply_result_get_subgoal.restype = GoalObj
_lib.Z3_apply_result_get_subgoal.argtypes = [ContextObj, ApplyResultObj, ctypes.c_uint]
_lib.Z3_mk_solver.restype = SolverObj
_lib.Z3_mk_solver.argtypes = [ContextObj]
_lib.Z3_mk_simple_solver.restype = SolverObj
_lib.Z3_mk_simple_solver.argtypes = [ContextObj]
_lib.Z3_mk_solver_for_logic.restype = SolverObj
_lib.Z3_mk_solver_for_logic.argtypes = [ContextObj, Symbol]
_lib.Z3_mk_solver_from_tactic.restype = SolverObj
_lib.Z3_mk_solver_from_tactic.argtypes = [ContextObj, TacticObj]
_lib.Z3_solver_translate.restype = SolverObj
_lib.Z3_solver_translate.argtypes = [ContextObj, SolverObj, ContextObj]
_lib.Z3_solver_import_model_converter.argtypes = [ContextObj, SolverObj, SolverObj]
_lib.Z3_solver_get_help.restype = ctypes.c_char_p
_lib.Z3_solver_get_help.argtypes = [ContextObj, SolverObj]
_lib.Z3_solver_get_param_descrs.restype = ParamDescrs
_lib.Z3_solver_get_param_descrs.argtypes = [ContextObj, SolverObj]
_lib.Z3_solver_set_params.argtypes = [ContextObj, SolverObj, Params]
_lib.Z3_solver_inc_ref.argtypes = [ContextObj, SolverObj]
_lib.Z3_solver_dec_ref.argtypes = [ContextObj, SolverObj]
_lib.Z3_solver_interrupt.argtypes = [ContextObj, SolverObj]
_lib.Z3_solver_push.argtypes = [ContextObj, SolverObj]
_lib.Z3_solver_pop.argtypes = [ContextObj, SolverObj, ctypes.c_uint]
_lib.Z3_solver_reset.argtypes = [ContextObj, SolverObj]
_lib.Z3_solver_get_num_scopes.restype = ctypes.c_uint
_lib.Z3_solver_get_num_scopes.argtypes = [ContextObj, SolverObj]
_lib.Z3_solver_assert.argtypes = [ContextObj, SolverObj, Ast]
_lib.Z3_solver_assert_and_track.argtypes = [ContextObj, SolverObj, Ast, Ast]
_lib.Z3_solver_from_file.argtypes = [ContextObj, SolverObj, ctypes.c_char_p]
_lib.Z3_solver_from_string.argtypes = [ContextObj, SolverObj, ctypes.c_char_p]
_lib.Z3_solver_get_assertions.restype = AstVectorObj
_lib.Z3_solver_get_assertions.argtypes = [ContextObj, SolverObj]
_lib.Z3_solver_get_units.restype = AstVectorObj
_lib.Z3_solver_get_units.argtypes = [ContextObj, SolverObj]
_lib.Z3_solver_get_trail.restype = AstVectorObj
_lib.Z3_solver_get_trail.argtypes = [ContextObj, SolverObj]
_lib.Z3_solver_get_non_units.restype = AstVectorObj
_lib.Z3_solver_get_non_units.argtypes = [ContextObj, SolverObj]
_lib.Z3_solver_get_levels.argtypes = [ContextObj, SolverObj, AstVectorObj, ctypes.c_uint, ctypes.POINTER(ctypes.c_uint)]
_lib.Z3_solver_congruence_root.restype = Ast
_lib.Z3_solver_congruence_root.argtypes = [ContextObj, SolverObj, Ast]
_lib.Z3_solver_congruence_next.restype = Ast
_lib.Z3_solver_congruence_next.argtypes = [ContextObj, SolverObj, Ast]
_lib.Z3_solver_register_on_clause.argtypes = [ContextObj, SolverObj, ctypes.c_void_p, Z3_on_clause_eh]
_lib.Z3_solver_propagate_init.argtypes = [ContextObj, SolverObj, ctypes.c_void_p, Z3_push_eh, Z3_pop_eh, Z3_fresh_eh]
_lib.Z3_solver_propagate_fixed.argtypes = [ContextObj, SolverObj, Z3_fixed_eh]
_lib.Z3_solver_propagate_final.argtypes = [ContextObj, SolverObj, Z3_final_eh]
_lib.Z3_solver_propagate_eq.argtypes = [ContextObj, SolverObj, Z3_eq_eh]
_lib.Z3_solver_propagate_diseq.argtypes = [ContextObj, SolverObj, Z3_eq_eh]
_lib.Z3_solver_propagate_created.argtypes = [ContextObj, SolverObj, Z3_created_eh]
_lib.Z3_solver_propagate_decide.argtypes = [ContextObj, SolverObj, Z3_decide_eh]
_lib.Z3_solver_next_split.restype = ctypes.c_bool
_lib.Z3_solver_next_split.argtypes = [ContextObj, SolverCallbackObj, Ast, ctypes.c_uint, ctypes.c_int]
_lib.Z3_solver_propagate_declare.restype = FuncDecl
_lib.Z3_solver_propagate_declare.argtypes = [ContextObj, Symbol, ctypes.c_uint, ctypes.POINTER(Sort), Sort]
_lib.Z3_solver_propagate_register.argtypes = [ContextObj, SolverObj, Ast]
_lib.Z3_solver_propagate_register_cb.argtypes = [ContextObj, SolverCallbackObj, Ast]
_lib.Z3_solver_propagate_consequence.restype = ctypes.c_bool
_lib.Z3_solver_propagate_consequence.argtypes = [ContextObj, SolverCallbackObj, ctypes.c_uint, ctypes.POINTER(Ast), ctypes.c_uint, ctypes.POINTER(Ast), ctypes.POINTER(Ast), Ast]
_lib.Z3_solver_check.restype = ctypes.c_int
_lib.Z3_solver_check.argtypes = [ContextObj, SolverObj]
_lib.Z3_solver_check_assumptions.restype = ctypes.c_int
_lib.Z3_solver_check_assumptions.argtypes = [ContextObj, SolverObj, ctypes.c_uint, ctypes.POINTER(Ast)]
_lib.Z3_get_implied_equalities.restype = ctypes.c_int
_lib.Z3_get_implied_equalities.argtypes = [ContextObj, SolverObj, ctypes.c_uint, ctypes.POINTER(Ast), ctypes.POINTER(ctypes.c_uint)]
_lib.Z3_solver_get_consequences.restype = ctypes.c_int
_lib.Z3_solver_get_consequences.argtypes = [ContextObj, SolverObj, AstVectorObj, AstVectorObj, AstVectorObj]
_lib.Z3_solver_cube.restype = AstVectorObj
_lib.Z3_solver_cube.argtypes = [ContextObj, SolverObj, AstVectorObj, ctypes.c_uint]
_lib.Z3_solver_get_model.restype = ModelObj
_lib.Z3_solver_get_model.argtypes = [ContextObj, SolverObj]
_lib.Z3_solver_get_proof.restype = Ast
_lib.Z3_solver_get_proof.argtypes = [ContextObj, SolverObj]
_lib.Z3_solver_get_unsat_core.restype = AstVectorObj
_lib.Z3_solver_get_unsat_core.argtypes = [ContextObj, SolverObj]
_lib.Z3_solver_get_reason_unknown.restype = ctypes.c_char_p
_lib.Z3_solver_get_reason_unknown.argtypes = [ContextObj, SolverObj]
_lib.Z3_solver_get_statistics.restype = StatsObj
_lib.Z3_solver_get_statistics.argtypes = [ContextObj, SolverObj]
_lib.Z3_solver_to_string.restype = ctypes.c_char_p
_lib.Z3_solver_to_string.argtypes = [ContextObj, SolverObj]
_lib.Z3_solver_to_dimacs_string.restype = ctypes.c_char_p
_lib.Z3_solver_to_dimacs_string.argtypes = [ContextObj, SolverObj, ctypes.c_bool]
_lib.Z3_stats_to_string.restype = ctypes.c_char_p
_lib.Z3_stats_to_string.argtypes = [ContextObj, StatsObj]
_lib.Z3_stats_inc_ref.argtypes = [ContextObj, StatsObj]
_lib.Z3_stats_dec_ref.argtypes = [ContextObj, StatsObj]
_lib.Z3_stats_size.restype = ctypes.c_uint
_lib.Z3_stats_size.argtypes = [ContextObj, StatsObj]
_lib.Z3_stats_get_key.restype = ctypes.c_char_p
_lib.Z3_stats_get_key.argtypes = [ContextObj, StatsObj, ctypes.c_uint]
_lib.Z3_stats_is_uint.restype = ctypes.c_bool
_lib.Z3_stats_is_uint.argtypes = [ContextObj, StatsObj, ctypes.c_uint]
_lib.Z3_stats_is_double.restype = ctypes.c_bool
_lib.Z3_stats_is_double.argtypes = [ContextObj, StatsObj, ctypes.c_uint]
_lib.Z3_stats_get_uint_value.restype = ctypes.c_uint
_lib.Z3_stats_get_uint_value.argtypes = [ContextObj, StatsObj, ctypes.c_uint]
_lib.Z3_stats_get_double_value.restype = ctypes.c_double
_lib.Z3_stats_get_double_value.argtypes = [ContextObj, StatsObj, ctypes.c_uint]
_lib.Z3_get_estimated_alloc_size.restype = ctypes.c_ulonglong
_lib.Z3_get_estimated_alloc_size.argtypes = []
_lib.Z3_mk_ast_vector.restype = AstVectorObj
_lib.Z3_mk_ast_vector.argtypes = [ContextObj]
_lib.Z3_ast_vector_inc_ref.argtypes = [ContextObj, AstVectorObj]
_lib.Z3_ast_vector_dec_ref.argtypes = [ContextObj, AstVectorObj]
_lib.Z3_ast_vector_size.restype = ctypes.c_uint
_lib.Z3_ast_vector_size.argtypes = [ContextObj, AstVectorObj]
_lib.Z3_ast_vector_get.restype = Ast
_lib.Z3_ast_vector_get.argtypes = [ContextObj, AstVectorObj, ctypes.c_uint]
_lib.Z3_ast_vector_set.argtypes = [ContextObj, AstVectorObj, ctypes.c_uint, Ast]
_lib.Z3_ast_vector_resize.argtypes = [ContextObj, AstVectorObj, ctypes.c_uint]
_lib.Z3_ast_vector_push.argtypes = [ContextObj, AstVectorObj, Ast]
_lib.Z3_ast_vector_translate.restype = AstVectorObj
_lib.Z3_ast_vector_translate.argtypes = [ContextObj, AstVectorObj, ContextObj]
_lib.Z3_ast_vector_to_string.restype = ctypes.c_char_p
_lib.Z3_ast_vector_to_string.argtypes = [ContextObj, AstVectorObj]
_lib.Z3_mk_ast_map.restype = AstMapObj
_lib.Z3_mk_ast_map.argtypes = [ContextObj]
_lib.Z3_ast_map_inc_ref.argtypes = [ContextObj, AstMapObj]
_lib.Z3_ast_map_dec_ref.argtypes = [ContextObj, AstMapObj]
_lib.Z3_ast_map_contains.restype = ctypes.c_bool
_lib.Z3_ast_map_contains.argtypes = [ContextObj, AstMapObj, Ast]
_lib.Z3_ast_map_find.restype = Ast
_lib.Z3_ast_map_find.argtypes = [ContextObj, AstMapObj, Ast]
_lib.Z3_ast_map_insert.argtypes = [ContextObj, AstMapObj, Ast, Ast]
_lib.Z3_ast_map_erase.argtypes = [ContextObj, AstMapObj, Ast]
_lib.Z3_ast_map_reset.argtypes = [ContextObj, AstMapObj]
_lib.Z3_ast_map_size.restype = ctypes.c_uint
_lib.Z3_ast_map_size.argtypes = [ContextObj, AstMapObj]
_lib.Z3_ast_map_keys.restype = AstVectorObj
_lib.Z3_ast_map_keys.argtypes = [ContextObj, AstMapObj]
_lib.Z3_ast_map_to_string.restype = ctypes.c_char_p
_lib.Z3_ast_map_to_string.argtypes = [ContextObj, AstMapObj]
_lib.Z3_algebraic_is_value.restype = ctypes.c_bool
_lib.Z3_algebraic_is_value.argtypes = [ContextObj, Ast]
_lib.Z3_algebraic_is_pos.restype = ctypes.c_bool
_lib.Z3_algebraic_is_pos.argtypes = [ContextObj, Ast]
_lib.Z3_algebraic_is_neg.restype = ctypes.c_bool
_lib.Z3_algebraic_is_neg.argtypes = [ContextObj, Ast]
_lib.Z3_algebraic_is_zero.restype = ctypes.c_bool
_lib.Z3_algebraic_is_zero.argtypes = [ContextObj, Ast]
_lib.Z3_algebraic_sign.restype = ctypes.c_int
_lib.Z3_algebraic_sign.argtypes = [ContextObj, Ast]
_lib.Z3_algebraic_add.restype = Ast
_lib.Z3_algebraic_add.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_algebraic_sub.restype = Ast
_lib.Z3_algebraic_sub.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_algebraic_mul.restype = Ast
_lib.Z3_algebraic_mul.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_algebraic_div.restype = Ast
_lib.Z3_algebraic_div.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_algebraic_root.restype = Ast
_lib.Z3_algebraic_root.argtypes = [ContextObj, Ast, ctypes.c_uint]
_lib.Z3_algebraic_power.restype = Ast
_lib.Z3_algebraic_power.argtypes = [ContextObj, Ast, ctypes.c_uint]
_lib.Z3_algebraic_lt.restype = ctypes.c_bool
_lib.Z3_algebraic_lt.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_algebraic_gt.restype = ctypes.c_bool
_lib.Z3_algebraic_gt.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_algebraic_le.restype = ctypes.c_bool
_lib.Z3_algebraic_le.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_algebraic_ge.restype = ctypes.c_bool
_lib.Z3_algebraic_ge.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_algebraic_eq.restype = ctypes.c_bool
_lib.Z3_algebraic_eq.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_algebraic_neq.restype = ctypes.c_bool
_lib.Z3_algebraic_neq.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_algebraic_roots.restype = AstVectorObj
_lib.Z3_algebraic_roots.argtypes = [ContextObj, Ast, ctypes.c_uint, ctypes.POINTER(Ast)]
_lib.Z3_algebraic_eval.restype = ctypes.c_int
_lib.Z3_algebraic_eval.argtypes = [ContextObj, Ast, ctypes.c_uint, ctypes.POINTER(Ast)]
_lib.Z3_algebraic_get_poly.restype = AstVectorObj
_lib.Z3_algebraic_get_poly.argtypes = [ContextObj, Ast]
_lib.Z3_algebraic_get_i.restype = ctypes.c_uint
_lib.Z3_algebraic_get_i.argtypes = [ContextObj, Ast]
_lib.Z3_polynomial_subresultants.restype = AstVectorObj
_lib.Z3_polynomial_subresultants.argtypes = [ContextObj, Ast, Ast, Ast]
_lib.Z3_rcf_del.argtypes = [ContextObj, RCFNumObj]
_lib.Z3_rcf_mk_rational.restype = RCFNumObj
_lib.Z3_rcf_mk_rational.argtypes = [ContextObj, ctypes.c_char_p]
_lib.Z3_rcf_mk_small_int.restype = RCFNumObj
_lib.Z3_rcf_mk_small_int.argtypes = [ContextObj, ctypes.c_int]
_lib.Z3_rcf_mk_pi.restype = RCFNumObj
_lib.Z3_rcf_mk_pi.argtypes = [ContextObj]
_lib.Z3_rcf_mk_e.restype = RCFNumObj
_lib.Z3_rcf_mk_e.argtypes = [ContextObj]
_lib.Z3_rcf_mk_infinitesimal.restype = RCFNumObj
_lib.Z3_rcf_mk_infinitesimal.argtypes = [ContextObj]
_lib.Z3_rcf_mk_roots.restype = ctypes.c_uint
_lib.Z3_rcf_mk_roots.argtypes = [ContextObj, ctypes.c_uint, ctypes.POINTER(RCFNumObj), ctypes.POINTER(RCFNumObj)]
_lib.Z3_rcf_add.restype = RCFNumObj
_lib.Z3_rcf_add.argtypes = [ContextObj, RCFNumObj, RCFNumObj]
_lib.Z3_rcf_sub.restype = RCFNumObj
_lib.Z3_rcf_sub.argtypes = [ContextObj, RCFNumObj, RCFNumObj]
_lib.Z3_rcf_mul.restype = RCFNumObj
_lib.Z3_rcf_mul.argtypes = [ContextObj, RCFNumObj, RCFNumObj]
_lib.Z3_rcf_div.restype = RCFNumObj
_lib.Z3_rcf_div.argtypes = [ContextObj, RCFNumObj, RCFNumObj]
_lib.Z3_rcf_neg.restype = RCFNumObj
_lib.Z3_rcf_neg.argtypes = [ContextObj, RCFNumObj]
_lib.Z3_rcf_inv.restype = RCFNumObj
_lib.Z3_rcf_inv.argtypes = [ContextObj, RCFNumObj]
_lib.Z3_rcf_power.restype = RCFNumObj
_lib.Z3_rcf_power.argtypes = [ContextObj, RCFNumObj, ctypes.c_uint]
_lib.Z3_rcf_lt.restype = ctypes.c_bool
_lib.Z3_rcf_lt.argtypes = [ContextObj, RCFNumObj, RCFNumObj]
_lib.Z3_rcf_gt.restype = ctypes.c_bool
_lib.Z3_rcf_gt.argtypes = [ContextObj, RCFNumObj, RCFNumObj]
_lib.Z3_rcf_le.restype = ctypes.c_bool
_lib.Z3_rcf_le.argtypes = [ContextObj, RCFNumObj, RCFNumObj]
_lib.Z3_rcf_ge.restype = ctypes.c_bool
_lib.Z3_rcf_ge.argtypes = [ContextObj, RCFNumObj, RCFNumObj]
_lib.Z3_rcf_eq.restype = ctypes.c_bool
_lib.Z3_rcf_eq.argtypes = [ContextObj, RCFNumObj, RCFNumObj]
_lib.Z3_rcf_neq.restype = ctypes.c_bool
_lib.Z3_rcf_neq.argtypes = [ContextObj, RCFNumObj, RCFNumObj]
_lib.Z3_rcf_num_to_string.restype = ctypes.c_char_p
_lib.Z3_rcf_num_to_string.argtypes = [ContextObj, RCFNumObj, ctypes.c_bool, ctypes.c_bool]
_lib.Z3_rcf_num_to_decimal_string.restype = ctypes.c_char_p
_lib.Z3_rcf_num_to_decimal_string.argtypes = [ContextObj, RCFNumObj, ctypes.c_uint]
_lib.Z3_rcf_get_numerator_denominator.argtypes = [ContextObj, RCFNumObj, ctypes.POINTER(RCFNumObj), ctypes.POINTER(RCFNumObj)]
_lib.Z3_rcf_is_rational.restype = ctypes.c_bool
_lib.Z3_rcf_is_rational.argtypes = [ContextObj, RCFNumObj]
_lib.Z3_rcf_is_algebraic.restype = ctypes.c_bool
_lib.Z3_rcf_is_algebraic.argtypes = [ContextObj, RCFNumObj]
_lib.Z3_rcf_is_infinitesimal.restype = ctypes.c_bool
_lib.Z3_rcf_is_infinitesimal.argtypes = [ContextObj, RCFNumObj]
_lib.Z3_rcf_is_transcendental.restype = ctypes.c_bool
_lib.Z3_rcf_is_transcendental.argtypes = [ContextObj, RCFNumObj]
_lib.Z3_rcf_extension_index.restype = ctypes.c_uint
_lib.Z3_rcf_extension_index.argtypes = [ContextObj, RCFNumObj]
_lib.Z3_rcf_transcendental_name.restype = Symbol
_lib.Z3_rcf_transcendental_name.argtypes = [ContextObj, RCFNumObj]
_lib.Z3_rcf_infinitesimal_name.restype = Symbol
_lib.Z3_rcf_infinitesimal_name.argtypes = [ContextObj, RCFNumObj]
_lib.Z3_rcf_num_coefficients.restype = ctypes.c_uint
_lib.Z3_rcf_num_coefficients.argtypes = [ContextObj, RCFNumObj]
_lib.Z3_rcf_coefficient.restype = RCFNumObj
_lib.Z3_rcf_coefficient.argtypes = [ContextObj, RCFNumObj, ctypes.c_uint]
_lib.Z3_rcf_interval.restype = ctypes.c_int
_lib.Z3_rcf_interval.argtypes = [ContextObj, RCFNumObj, ctypes.POINTER(ctypes.c_int), ctypes.POINTER(ctypes.c_int), ctypes.POINTER(RCFNumObj), ctypes.POINTER(ctypes.c_int), ctypes.POINTER(ctypes.c_int), ctypes.POINTER(RCFNumObj)]
_lib.Z3_rcf_num_sign_conditions.restype = ctypes.c_uint
_lib.Z3_rcf_num_sign_conditions.argtypes = [ContextObj, RCFNumObj]
_lib.Z3_rcf_sign_condition_sign.restype = ctypes.c_int
_lib.Z3_rcf_sign_condition_sign.argtypes = [ContextObj, RCFNumObj, ctypes.c_uint]
_lib.Z3_rcf_num_sign_condition_coefficients.restype = ctypes.c_uint
_lib.Z3_rcf_num_sign_condition_coefficients.argtypes = [ContextObj, RCFNumObj, ctypes.c_uint]
_lib.Z3_rcf_sign_condition_coefficient.restype = RCFNumObj
_lib.Z3_rcf_sign_condition_coefficient.argtypes = [ContextObj, RCFNumObj, ctypes.c_uint, ctypes.c_uint]
_lib.Z3_mk_fixedpoint.restype = FixedpointObj
_lib.Z3_mk_fixedpoint.argtypes = [ContextObj]
_lib.Z3_fixedpoint_inc_ref.argtypes = [ContextObj, FixedpointObj]
_lib.Z3_fixedpoint_dec_ref.argtypes = [ContextObj, FixedpointObj]
_lib.Z3_fixedpoint_add_rule.argtypes = [ContextObj, FixedpointObj, Ast, Symbol]
_lib.Z3_fixedpoint_add_fact.argtypes = [ContextObj, FixedpointObj, FuncDecl, ctypes.c_uint, ctypes.POINTER(ctypes.c_uint)]
_lib.Z3_fixedpoint_assert.argtypes = [ContextObj, FixedpointObj, Ast]
_lib.Z3_fixedpoint_query.restype = ctypes.c_int
_lib.Z3_fixedpoint_query.argtypes = [ContextObj, FixedpointObj, Ast]
_lib.Z3_fixedpoint_query_relations.restype = ctypes.c_int
_lib.Z3_fixedpoint_query_relations.argtypes = [ContextObj, FixedpointObj, ctypes.c_uint, ctypes.POINTER(FuncDecl)]
_lib.Z3_fixedpoint_get_answer.restype = Ast
_lib.Z3_fixedpoint_get_answer.argtypes = [ContextObj, FixedpointObj]
_lib.Z3_fixedpoint_get_reason_unknown.restype = ctypes.c_char_p
_lib.Z3_fixedpoint_get_reason_unknown.argtypes = [ContextObj, FixedpointObj]
_lib.Z3_fixedpoint_update_rule.argtypes = [ContextObj, FixedpointObj, Ast, Symbol]
_lib.Z3_fixedpoint_get_num_levels.restype = ctypes.c_uint
_lib.Z3_fixedpoint_get_num_levels.argtypes = [ContextObj, FixedpointObj, FuncDecl]
_lib.Z3_fixedpoint_get_cover_delta.restype = Ast
_lib.Z3_fixedpoint_get_cover_delta.argtypes = [ContextObj, FixedpointObj, ctypes.c_int, FuncDecl]
_lib.Z3_fixedpoint_add_cover.argtypes = [ContextObj, FixedpointObj, ctypes.c_int, FuncDecl, Ast]
_lib.Z3_fixedpoint_get_statistics.restype = StatsObj
_lib.Z3_fixedpoint_get_statistics.argtypes = [ContextObj, FixedpointObj]
_lib.Z3_fixedpoint_register_relation.argtypes = [ContextObj, FixedpointObj, FuncDecl]
_lib.Z3_fixedpoint_set_predicate_representation.argtypes = [ContextObj, FixedpointObj, FuncDecl, ctypes.c_uint, ctypes.POINTER(Symbol)]
_lib.Z3_fixedpoint_get_rules.restype = AstVectorObj
_lib.Z3_fixedpoint_get_rules.argtypes = [ContextObj, FixedpointObj]
_lib.Z3_fixedpoint_get_assertions.restype = AstVectorObj
_lib.Z3_fixedpoint_get_assertions.argtypes = [ContextObj, FixedpointObj]
_lib.Z3_fixedpoint_set_params.argtypes = [ContextObj, FixedpointObj, Params]
_lib.Z3_fixedpoint_get_help.restype = ctypes.c_char_p
_lib.Z3_fixedpoint_get_help.argtypes = [ContextObj, FixedpointObj]
_lib.Z3_fixedpoint_get_param_descrs.restype = ParamDescrs
_lib.Z3_fixedpoint_get_param_descrs.argtypes = [ContextObj, FixedpointObj]
_lib.Z3_fixedpoint_to_string.restype = ctypes.c_char_p
_lib.Z3_fixedpoint_to_string.argtypes = [ContextObj, FixedpointObj, ctypes.c_uint, ctypes.POINTER(Ast)]
_lib.Z3_fixedpoint_from_string.restype = AstVectorObj
_lib.Z3_fixedpoint_from_string.argtypes = [ContextObj, FixedpointObj, ctypes.c_char_p]
_lib.Z3_fixedpoint_from_file.restype = AstVectorObj
_lib.Z3_fixedpoint_from_file.argtypes = [ContextObj, FixedpointObj, ctypes.c_char_p]
_lib.Z3_mk_optimize.restype = OptimizeObj
_lib.Z3_mk_optimize.argtypes = [ContextObj]
_lib.Z3_optimize_inc_ref.argtypes = [ContextObj, OptimizeObj]
_lib.Z3_optimize_dec_ref.argtypes = [ContextObj, OptimizeObj]
_lib.Z3_optimize_assert.argtypes = [ContextObj, OptimizeObj, Ast]
_lib.Z3_optimize_assert_and_track.argtypes = [ContextObj, OptimizeObj, Ast, Ast]
_lib.Z3_optimize_assert_soft.restype = ctypes.c_uint
_lib.Z3_optimize_assert_soft.argtypes = [ContextObj, OptimizeObj, Ast, ctypes.c_char_p, Symbol]
_lib.Z3_optimize_maximize.restype = ctypes.c_uint
_lib.Z3_optimize_maximize.argtypes = [ContextObj, OptimizeObj, Ast]
_lib.Z3_optimize_minimize.restype = ctypes.c_uint
_lib.Z3_optimize_minimize.argtypes = [ContextObj, OptimizeObj, Ast]
_lib.Z3_optimize_push.argtypes = [ContextObj, OptimizeObj]
_lib.Z3_optimize_pop.argtypes = [ContextObj, OptimizeObj]
_lib.Z3_optimize_check.restype = ctypes.c_int
_lib.Z3_optimize_check.argtypes = [ContextObj, OptimizeObj, ctypes.c_uint, ctypes.POINTER(Ast)]
_lib.Z3_optimize_get_reason_unknown.restype = ctypes.c_char_p
_lib.Z3_optimize_get_reason_unknown.argtypes = [ContextObj, OptimizeObj]
_lib.Z3_optimize_get_model.restype = ModelObj
_lib.Z3_optimize_get_model.argtypes = [ContextObj, OptimizeObj]
_lib.Z3_optimize_get_unsat_core.restype = AstVectorObj
_lib.Z3_optimize_get_unsat_core.argtypes = [ContextObj, OptimizeObj]
_lib.Z3_optimize_set_params.argtypes = [ContextObj, OptimizeObj, Params]
_lib.Z3_optimize_get_param_descrs.restype = ParamDescrs
_lib.Z3_optimize_get_param_descrs.argtypes = [ContextObj, OptimizeObj]
_lib.Z3_optimize_get_lower.restype = Ast
_lib.Z3_optimize_get_lower.argtypes = [ContextObj, OptimizeObj, ctypes.c_uint]
_lib.Z3_optimize_get_upper.restype = Ast
_lib.Z3_optimize_get_upper.argtypes = [ContextObj, OptimizeObj, ctypes.c_uint]
_lib.Z3_optimize_get_lower_as_vector.restype = AstVectorObj
_lib.Z3_optimize_get_lower_as_vector.argtypes = [ContextObj, OptimizeObj, ctypes.c_uint]
_lib.Z3_optimize_get_upper_as_vector.restype = AstVectorObj
_lib.Z3_optimize_get_upper_as_vector.argtypes = [ContextObj, OptimizeObj, ctypes.c_uint]
_lib.Z3_optimize_to_string.restype = ctypes.c_char_p
_lib.Z3_optimize_to_string.argtypes = [ContextObj, OptimizeObj]
_lib.Z3_optimize_from_string.argtypes = [ContextObj, OptimizeObj, ctypes.c_char_p]
_lib.Z3_optimize_from_file.argtypes = [ContextObj, OptimizeObj, ctypes.c_char_p]
_lib.Z3_optimize_get_help.restype = ctypes.c_char_p
_lib.Z3_optimize_get_help.argtypes = [ContextObj, OptimizeObj]
_lib.Z3_optimize_get_statistics.restype = StatsObj
_lib.Z3_optimize_get_statistics.argtypes = [ContextObj, OptimizeObj]
_lib.Z3_optimize_get_assertions.restype = AstVectorObj
_lib.Z3_optimize_get_assertions.argtypes = [ContextObj, OptimizeObj]
_lib.Z3_optimize_get_objectives.restype = AstVectorObj
_lib.Z3_optimize_get_objectives.argtypes = [ContextObj, OptimizeObj]
_lib.Z3_mk_fpa_rounding_mode_sort.restype = Sort
_lib.Z3_mk_fpa_rounding_mode_sort.argtypes = [ContextObj]
_lib.Z3_mk_fpa_round_nearest_ties_to_even.restype = Ast
_lib.Z3_mk_fpa_round_nearest_ties_to_even.argtypes = [ContextObj]
_lib.Z3_mk_fpa_rne.restype = Ast
_lib.Z3_mk_fpa_rne.argtypes = [ContextObj]
_lib.Z3_mk_fpa_round_nearest_ties_to_away.restype = Ast
_lib.Z3_mk_fpa_round_nearest_ties_to_away.argtypes = [ContextObj]
_lib.Z3_mk_fpa_rna.restype = Ast
_lib.Z3_mk_fpa_rna.argtypes = [ContextObj]
_lib.Z3_mk_fpa_round_toward_positive.restype = Ast
_lib.Z3_mk_fpa_round_toward_positive.argtypes = [ContextObj]
_lib.Z3_mk_fpa_rtp.restype = Ast
_lib.Z3_mk_fpa_rtp.argtypes = [ContextObj]
_lib.Z3_mk_fpa_round_toward_negative.restype = Ast
_lib.Z3_mk_fpa_round_toward_negative.argtypes = [ContextObj]
_lib.Z3_mk_fpa_rtn.restype = Ast
_lib.Z3_mk_fpa_rtn.argtypes = [ContextObj]
_lib.Z3_mk_fpa_round_toward_zero.restype = Ast
_lib.Z3_mk_fpa_round_toward_zero.argtypes = [ContextObj]
_lib.Z3_mk_fpa_rtz.restype = Ast
_lib.Z3_mk_fpa_rtz.argtypes = [ContextObj]
_lib.Z3_mk_fpa_sort.restype = Sort
_lib.Z3_mk_fpa_sort.argtypes = [ContextObj, ctypes.c_uint, ctypes.c_uint]
_lib.Z3_mk_fpa_sort_half.restype = Sort
_lib.Z3_mk_fpa_sort_half.argtypes = [ContextObj]
_lib.Z3_mk_fpa_sort_16.restype = Sort
_lib.Z3_mk_fpa_sort_16.argtypes = [ContextObj]
_lib.Z3_mk_fpa_sort_single.restype = Sort
_lib.Z3_mk_fpa_sort_single.argtypes = [ContextObj]
_lib.Z3_mk_fpa_sort_32.restype = Sort
_lib.Z3_mk_fpa_sort_32.argtypes = [ContextObj]
_lib.Z3_mk_fpa_sort_double.restype = Sort
_lib.Z3_mk_fpa_sort_double.argtypes = [ContextObj]
_lib.Z3_mk_fpa_sort_64.restype = Sort
_lib.Z3_mk_fpa_sort_64.argtypes = [ContextObj]
_lib.Z3_mk_fpa_sort_quadruple.restype = Sort
_lib.Z3_mk_fpa_sort_quadruple.argtypes = [ContextObj]
_lib.Z3_mk_fpa_sort_128.restype = Sort
_lib.Z3_mk_fpa_sort_128.argtypes = [ContextObj]
_lib.Z3_mk_fpa_nan.restype = Ast
_lib.Z3_mk_fpa_nan.argtypes = [ContextObj, Sort]
_lib.Z3_mk_fpa_inf.restype = Ast
_lib.Z3_mk_fpa_inf.argtypes = [ContextObj, Sort, ctypes.c_bool]
_lib.Z3_mk_fpa_zero.restype = Ast
_lib.Z3_mk_fpa_zero.argtypes = [ContextObj, Sort, ctypes.c_bool]
_lib.Z3_mk_fpa_fp.restype = Ast
_lib.Z3_mk_fpa_fp.argtypes = [ContextObj, Ast, Ast, Ast]
_lib.Z3_mk_fpa_numeral_float.restype = Ast
_lib.Z3_mk_fpa_numeral_float.argtypes = [ContextObj, ctypes.c_float, Sort]
_lib.Z3_mk_fpa_numeral_double.restype = Ast
_lib.Z3_mk_fpa_numeral_double.argtypes = [ContextObj, ctypes.c_double, Sort]
_lib.Z3_mk_fpa_numeral_int.restype = Ast
_lib.Z3_mk_fpa_numeral_int.argtypes = [ContextObj, ctypes.c_int, Sort]
_lib.Z3_mk_fpa_numeral_int_uint.restype = Ast
_lib.Z3_mk_fpa_numeral_int_uint.argtypes = [ContextObj, ctypes.c_bool, ctypes.c_int, ctypes.c_uint, Sort]
_lib.Z3_mk_fpa_numeral_int64_uint64.restype = Ast
_lib.Z3_mk_fpa_numeral_int64_uint64.argtypes = [ContextObj, ctypes.c_bool, ctypes.c_longlong, ctypes.c_ulonglong, Sort]
_lib.Z3_mk_fpa_abs.restype = Ast
_lib.Z3_mk_fpa_abs.argtypes = [ContextObj, Ast]
_lib.Z3_mk_fpa_neg.restype = Ast
_lib.Z3_mk_fpa_neg.argtypes = [ContextObj, Ast]
_lib.Z3_mk_fpa_add.restype = Ast
_lib.Z3_mk_fpa_add.argtypes = [ContextObj, Ast, Ast, Ast]
_lib.Z3_mk_fpa_sub.restype = Ast
_lib.Z3_mk_fpa_sub.argtypes = [ContextObj, Ast, Ast, Ast]
_lib.Z3_mk_fpa_mul.restype = Ast
_lib.Z3_mk_fpa_mul.argtypes = [ContextObj, Ast, Ast, Ast]
_lib.Z3_mk_fpa_div.restype = Ast
_lib.Z3_mk_fpa_div.argtypes = [ContextObj, Ast, Ast, Ast]
_lib.Z3_mk_fpa_fma.restype = Ast
_lib.Z3_mk_fpa_fma.argtypes = [ContextObj, Ast, Ast, Ast, Ast]
_lib.Z3_mk_fpa_sqrt.restype = Ast
_lib.Z3_mk_fpa_sqrt.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_fpa_rem.restype = Ast
_lib.Z3_mk_fpa_rem.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_fpa_round_to_integral.restype = Ast
_lib.Z3_mk_fpa_round_to_integral.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_fpa_min.restype = Ast
_lib.Z3_mk_fpa_min.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_fpa_max.restype = Ast
_lib.Z3_mk_fpa_max.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_fpa_leq.restype = Ast
_lib.Z3_mk_fpa_leq.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_fpa_lt.restype = Ast
_lib.Z3_mk_fpa_lt.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_fpa_geq.restype = Ast
_lib.Z3_mk_fpa_geq.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_fpa_gt.restype = Ast
_lib.Z3_mk_fpa_gt.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_fpa_eq.restype = Ast
_lib.Z3_mk_fpa_eq.argtypes = [ContextObj, Ast, Ast]
_lib.Z3_mk_fpa_is_normal.restype = Ast
_lib.Z3_mk_fpa_is_normal.argtypes = [ContextObj, Ast]
_lib.Z3_mk_fpa_is_subnormal.restype = Ast
_lib.Z3_mk_fpa_is_subnormal.argtypes = [ContextObj, Ast]
_lib.Z3_mk_fpa_is_zero.restype = Ast
_lib.Z3_mk_fpa_is_zero.argtypes = [ContextObj, Ast]
_lib.Z3_mk_fpa_is_infinite.restype = Ast
_lib.Z3_mk_fpa_is_infinite.argtypes = [ContextObj, Ast]
_lib.Z3_mk_fpa_is_nan.restype = Ast
_lib.Z3_mk_fpa_is_nan.argtypes = [ContextObj, Ast]
_lib.Z3_mk_fpa_is_negative.restype = Ast
_lib.Z3_mk_fpa_is_negative.argtypes = [ContextObj, Ast]
_lib.Z3_mk_fpa_is_positive.restype = Ast
_lib.Z3_mk_fpa_is_positive.argtypes = [ContextObj, Ast]
_lib.Z3_mk_fpa_to_fp_bv.restype = Ast
_lib.Z3_mk_fpa_to_fp_bv.argtypes = [ContextObj, Ast, Sort]
_lib.Z3_mk_fpa_to_fp_float.restype = Ast
_lib.Z3_mk_fpa_to_fp_float.argtypes = [ContextObj, Ast, Ast, Sort]
_lib.Z3_mk_fpa_to_fp_real.restype = Ast
_lib.Z3_mk_fpa_to_fp_real.argtypes = [ContextObj, Ast, Ast, Sort]
_lib.Z3_mk_fpa_to_fp_signed.restype = Ast
_lib.Z3_mk_fpa_to_fp_signed.argtypes = [ContextObj, Ast, Ast, Sort]
_lib.Z3_mk_fpa_to_fp_unsigned.restype = Ast
_lib.Z3_mk_fpa_to_fp_unsigned.argtypes = [ContextObj, Ast, Ast, Sort]
_lib.Z3_mk_fpa_to_ubv.restype = Ast
_lib.Z3_mk_fpa_to_ubv.argtypes = [ContextObj, Ast, Ast, ctypes.c_uint]
_lib.Z3_mk_fpa_to_sbv.restype = Ast
_lib.Z3_mk_fpa_to_sbv.argtypes = [ContextObj, Ast, Ast, ctypes.c_uint]
_lib.Z3_mk_fpa_to_real.restype = Ast
_lib.Z3_mk_fpa_to_real.argtypes = [ContextObj, Ast]
_lib.Z3_fpa_get_ebits.restype = ctypes.c_uint
_lib.Z3_fpa_get_ebits.argtypes = [ContextObj, Sort]
_lib.Z3_fpa_get_sbits.restype = ctypes.c_uint
_lib.Z3_fpa_get_sbits.argtypes = [ContextObj, Sort]
_lib.Z3_fpa_is_numeral_nan.restype = ctypes.c_bool
_lib.Z3_fpa_is_numeral_nan.argtypes = [ContextObj, Ast]
_lib.Z3_fpa_is_numeral_inf.restype = ctypes.c_bool
_lib.Z3_fpa_is_numeral_inf.argtypes = [ContextObj, Ast]
_lib.Z3_fpa_is_numeral_zero.restype = ctypes.c_bool
_lib.Z3_fpa_is_numeral_zero.argtypes = [ContextObj, Ast]
_lib.Z3_fpa_is_numeral_normal.restype = ctypes.c_bool
_lib.Z3_fpa_is_numeral_normal.argtypes = [ContextObj, Ast]
_lib.Z3_fpa_is_numeral_subnormal.restype = ctypes.c_bool
_lib.Z3_fpa_is_numeral_subnormal.argtypes = [ContextObj, Ast]
_lib.Z3_fpa_is_numeral_positive.restype = ctypes.c_bool
_lib.Z3_fpa_is_numeral_positive.argtypes = [ContextObj, Ast]
_lib.Z3_fpa_is_numeral_negative.restype = ctypes.c_bool
_lib.Z3_fpa_is_numeral_negative.argtypes = [ContextObj, Ast]
_lib.Z3_fpa_get_numeral_sign_bv.restype = Ast
_lib.Z3_fpa_get_numeral_sign_bv.argtypes = [ContextObj, Ast]
_lib.Z3_fpa_get_numeral_significand_bv.restype = Ast
_lib.Z3_fpa_get_numeral_significand_bv.argtypes = [ContextObj, Ast]
_lib.Z3_fpa_get_numeral_sign.restype = ctypes.c_bool
_lib.Z3_fpa_get_numeral_sign.argtypes = [ContextObj, Ast, ctypes.POINTER(ctypes.c_int)]
_lib.Z3_fpa_get_numeral_significand_string.restype = ctypes.c_char_p
_lib.Z3_fpa_get_numeral_significand_string.argtypes = [ContextObj, Ast]
_lib.Z3_fpa_get_numeral_significand_uint64.restype = ctypes.c_bool
_lib.Z3_fpa_get_numeral_significand_uint64.argtypes = [ContextObj, Ast, ctypes.POINTER(ctypes.c_ulonglong)]
_lib.Z3_fpa_get_numeral_exponent_string.restype = ctypes.c_char_p
_lib.Z3_fpa_get_numeral_exponent_string.argtypes = [ContextObj, Ast, ctypes.c_bool]
_lib.Z3_fpa_get_numeral_exponent_int64.restype = ctypes.c_bool
_lib.Z3_fpa_get_numeral_exponent_int64.argtypes = [ContextObj, Ast, ctypes.POINTER(ctypes.c_longlong), ctypes.c_bool]
_lib.Z3_fpa_get_numeral_exponent_bv.restype = Ast
_lib.Z3_fpa_get_numeral_exponent_bv.argtypes = [ContextObj, Ast, ctypes.c_bool]
_lib.Z3_mk_fpa_to_ieee_bv.restype = Ast
_lib.Z3_mk_fpa_to_ieee_bv.argtypes = [ContextObj, Ast]
_lib.Z3_mk_fpa_to_fp_int_real.restype = Ast
_lib.Z3_mk_fpa_to_fp_int_real.argtypes = [ContextObj, Ast, Ast, Ast, Sort]
_lib.Z3_fixedpoint_query_from_lvl.restype = ctypes.c_int
_lib.Z3_fixedpoint_query_from_lvl.argtypes = [ContextObj, FixedpointObj, Ast, ctypes.c_uint]
_lib.Z3_fixedpoint_get_ground_sat_answer.restype = Ast
_lib.Z3_fixedpoint_get_ground_sat_answer.argtypes = [ContextObj, FixedpointObj]
_lib.Z3_fixedpoint_get_rules_along_trace.restype = AstVectorObj
_lib.Z3_fixedpoint_get_rules_along_trace.argtypes = [ContextObj, FixedpointObj]
_lib.Z3_fixedpoint_get_rule_names_along_trace.restype = Symbol
_lib.Z3_fixedpoint_get_rule_names_along_trace.argtypes = [ContextObj, FixedpointObj]
_lib.Z3_fixedpoint_add_invariant.argtypes = [ContextObj, FixedpointObj, FuncDecl, Ast]
_lib.Z3_fixedpoint_get_reachable.restype = Ast
_lib.Z3_fixedpoint_get_reachable.argtypes = [ContextObj, FixedpointObj, FuncDecl]
_lib.Z3_qe_model_project.restype = Ast
_lib.Z3_qe_model_project.argtypes = [ContextObj, ModelObj, ctypes.c_uint, ctypes.POINTER(Ast), Ast]
_lib.Z3_qe_model_project_skolem.restype = Ast
_lib.Z3_qe_model_project_skolem.argtypes = [ContextObj, ModelObj, ctypes.c_uint, ctypes.POINTER(Ast), Ast, AstMapObj]
_lib.Z3_model_extrapolate.restype = Ast
_lib.Z3_model_extrapolate.argtypes = [ContextObj, ModelObj, Ast]
_lib.Z3_qe_lite.restype = Ast
_lib.Z3_qe_lite.argtypes = [ContextObj, AstVectorObj, Ast]

class Elementaries:
  def __init__(self, f):
    self.f = f
    self.get_error_code = _lib.Z3_get_error_code
    self.get_error_message = _lib.Z3_get_error_msg
    self.OK = Z3_OK
    self.Exception = Z3Exception

  def Check(self, ctx):
    err = self.get_error_code(ctx)
    if err != self.OK:
        raise self.Exception(self.get_error_message(ctx, err))

def Z3_set_error_handler(ctx, hndlr, _elems=Elementaries(_lib.Z3_set_error_handler)):
  ceh = _error_handler_type(hndlr)
  _elems.f(ctx, ceh)
  _elems.Check(ctx)
  return ceh

def Z3_solver_register_on_clause(ctx, s, user_ctx, on_clause_eh, _elems = Elementaries(_lib.Z3_solver_register_on_clause)):
    _elems.f(ctx, s, user_ctx, on_clause_eh)
    _elems.Check(ctx)
    
def Z3_solver_propagate_init(ctx, s, user_ctx, push_eh, pop_eh, fresh_eh, _elems = Elementaries(_lib.Z3_solver_propagate_init)):
    _elems.f(ctx, s, user_ctx, push_eh, pop_eh, fresh_eh)
    _elems.Check(ctx)

def Z3_solver_propagate_final(ctx, s, final_eh, _elems = Elementaries(_lib.Z3_solver_propagate_final)):
    _elems.f(ctx, s, final_eh)
    _elems.Check(ctx)

def Z3_solver_propagate_fixed(ctx, s, fixed_eh, _elems = Elementaries(_lib.Z3_solver_propagate_fixed)):
    _elems.f(ctx, s, fixed_eh)
    _elems.Check(ctx)

def Z3_solver_propagate_eq(ctx, s, eq_eh, _elems = Elementaries(_lib.Z3_solver_propagate_eq)):
    _elems.f(ctx, s, eq_eh)
    _elems.Check(ctx)

def Z3_solver_propagate_diseq(ctx, s, diseq_eh, _elems = Elementaries(_lib.Z3_solver_propagate_diseq)):
    _elems.f(ctx, s, diseq_eh)
    _elems.Check(ctx)

def Z3_optimize_register_model_eh(ctx, o, m, user_ctx, on_model_eh, _elems = Elementaries(_lib.Z3_optimize_register_model_eh)):
    _elems.f(ctx, o, m, user_ctx, on_model_eh)
    _elems.Check(ctx)

def Z3_global_param_set(a0, a1, _elems=Elementaries(_lib.Z3_global_param_set)):
  _elems.f(_str_to_bytes(a0), _str_to_bytes(a1))

def Z3_global_param_reset_all(_elems=Elementaries(_lib.Z3_global_param_reset_all)):
  _elems.f()

def Z3_global_param_get(a0, a1, _elems=Elementaries(_lib.Z3_global_param_get)):
  r = _elems.f(_str_to_bytes(a0), _str_to_bytes(a1))
  return r

def Z3_mk_config(_elems=Elementaries(_lib.Z3_mk_config)):
  r = _elems.f()
  return r

def Z3_del_config(a0, _elems=Elementaries(_lib.Z3_del_config)):
  _elems.f(a0)

def Z3_set_param_value(a0, a1, a2, _elems=Elementaries(_lib.Z3_set_param_value)):
  _elems.f(a0, _str_to_bytes(a1), _str_to_bytes(a2))

def Z3_mk_context(a0, _elems=Elementaries(_lib.Z3_mk_context)):
  r = _elems.f(a0)
  return r

def Z3_mk_context_rc(a0, _elems=Elementaries(_lib.Z3_mk_context_rc)):
  r = _elems.f(a0)
  return r

def Z3_del_context(a0, _elems=Elementaries(_lib.Z3_del_context)):
  _elems.f(a0)

def Z3_inc_ref(a0, a1, _elems=Elementaries(_lib.Z3_inc_ref)):
  _elems.f(a0, a1)
  _elems.Check(a0)

def Z3_dec_ref(a0, a1, _elems=Elementaries(_lib.Z3_dec_ref)):
  _elems.f(a0, a1)

def Z3_update_param_value(a0, a1, a2, _elems=Elementaries(_lib.Z3_update_param_value)):
  _elems.f(a0, _str_to_bytes(a1), _str_to_bytes(a2))
  _elems.Check(a0)

def Z3_get_global_param_descrs(a0, _elems=Elementaries(_lib.Z3_get_global_param_descrs)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_interrupt(a0, _elems=Elementaries(_lib.Z3_interrupt)):
  _elems.f(a0)
  _elems.Check(a0)

def Z3_enable_concurrent_dec_ref(a0, _elems=Elementaries(_lib.Z3_enable_concurrent_dec_ref)):
  _elems.f(a0)
  _elems.Check(a0)

def Z3_mk_params(a0, _elems=Elementaries(_lib.Z3_mk_params)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_params_inc_ref(a0, a1, _elems=Elementaries(_lib.Z3_params_inc_ref)):
  _elems.f(a0, a1)
  _elems.Check(a0)

def Z3_params_dec_ref(a0, a1, _elems=Elementaries(_lib.Z3_params_dec_ref)):
  _elems.f(a0, a1)

def Z3_params_set_bool(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_params_set_bool)):
  _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)

def Z3_params_set_uint(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_params_set_uint)):
  _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)

def Z3_params_set_double(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_params_set_double)):
  _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)

def Z3_params_set_symbol(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_params_set_symbol)):
  _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)

def Z3_params_to_string(a0, a1, _elems=Elementaries(_lib.Z3_params_to_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_params_to_string_bytes(a0, a1, _elems=Elementaries(_lib.Z3_params_to_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_params_validate(a0, a1, a2, _elems=Elementaries(_lib.Z3_params_validate)):
  _elems.f(a0, a1, a2)
  _elems.Check(a0)

def Z3_param_descrs_inc_ref(a0, a1, _elems=Elementaries(_lib.Z3_param_descrs_inc_ref)):
  _elems.f(a0, a1)
  _elems.Check(a0)

def Z3_param_descrs_dec_ref(a0, a1, _elems=Elementaries(_lib.Z3_param_descrs_dec_ref)):
  _elems.f(a0, a1)

def Z3_param_descrs_get_kind(a0, a1, a2, _elems=Elementaries(_lib.Z3_param_descrs_get_kind)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_param_descrs_size(a0, a1, _elems=Elementaries(_lib.Z3_param_descrs_size)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_param_descrs_get_name(a0, a1, a2, _elems=Elementaries(_lib.Z3_param_descrs_get_name)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_param_descrs_get_documentation(a0, a1, a2, _elems=Elementaries(_lib.Z3_param_descrs_get_documentation)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_param_descrs_get_documentation_bytes(a0, a1, a2, _elems=Elementaries(_lib.Z3_param_descrs_get_documentation)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_param_descrs_to_string(a0, a1, _elems=Elementaries(_lib.Z3_param_descrs_to_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_param_descrs_to_string_bytes(a0, a1, _elems=Elementaries(_lib.Z3_param_descrs_to_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_int_symbol(a0, a1, _elems=Elementaries(_lib.Z3_mk_int_symbol)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_string_symbol(a0, a1, _elems=Elementaries(_lib.Z3_mk_string_symbol)):
  r = _elems.f(a0, _str_to_bytes(a1))
  _elems.Check(a0)
  return r

def Z3_mk_uninterpreted_sort(a0, a1, _elems=Elementaries(_lib.Z3_mk_uninterpreted_sort)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_type_variable(a0, a1, _elems=Elementaries(_lib.Z3_mk_type_variable)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_bool_sort(a0, _elems=Elementaries(_lib.Z3_mk_bool_sort)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_mk_int_sort(a0, _elems=Elementaries(_lib.Z3_mk_int_sort)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_mk_real_sort(a0, _elems=Elementaries(_lib.Z3_mk_real_sort)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_mk_bv_sort(a0, a1, _elems=Elementaries(_lib.Z3_mk_bv_sort)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_finite_domain_sort(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_finite_domain_sort)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_array_sort(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_array_sort)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_array_sort_n(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_mk_array_sort_n)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_mk_tuple_sort(a0, a1, a2, a3, a4, a5, a6, _elems=Elementaries(_lib.Z3_mk_tuple_sort)):
  r = _elems.f(a0, a1, a2, a3, a4, a5, a6)
  _elems.Check(a0)
  return r

def Z3_mk_enumeration_sort(a0, a1, a2, a3, a4, a5, _elems=Elementaries(_lib.Z3_mk_enumeration_sort)):
  r = _elems.f(a0, a1, a2, a3, a4, a5)
  _elems.Check(a0)
  return r

def Z3_mk_list_sort(a0, a1, a2, a3, a4, a5, a6, a7, a8, _elems=Elementaries(_lib.Z3_mk_list_sort)):
  r = _elems.f(a0, a1, a2, a3, a4, a5, a6, a7, a8)
  _elems.Check(a0)
  return r

def Z3_mk_constructor(a0, a1, a2, a3, a4, a5, a6, _elems=Elementaries(_lib.Z3_mk_constructor)):
  r = _elems.f(a0, a1, a2, a3, a4, a5, a6)
  _elems.Check(a0)
  return r

def Z3_constructor_num_fields(a0, a1, _elems=Elementaries(_lib.Z3_constructor_num_fields)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_del_constructor(a0, a1, _elems=Elementaries(_lib.Z3_del_constructor)):
  _elems.f(a0, a1)
  _elems.Check(a0)

def Z3_mk_datatype(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_mk_datatype)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_mk_datatype_sort(a0, a1, _elems=Elementaries(_lib.Z3_mk_datatype_sort)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_constructor_list(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_constructor_list)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_del_constructor_list(a0, a1, _elems=Elementaries(_lib.Z3_del_constructor_list)):
  _elems.f(a0, a1)
  _elems.Check(a0)

def Z3_mk_datatypes(a0, a1, a2, a3, a4, _elems=Elementaries(_lib.Z3_mk_datatypes)):
  _elems.f(a0, a1, a2, a3, a4)
  _elems.Check(a0)

def Z3_query_constructor(a0, a1, a2, a3, a4, a5, _elems=Elementaries(_lib.Z3_query_constructor)):
  _elems.f(a0, a1, a2, a3, a4, a5)
  _elems.Check(a0)

def Z3_mk_func_decl(a0, a1, a2, a3, a4, _elems=Elementaries(_lib.Z3_mk_func_decl)):
  r = _elems.f(a0, a1, a2, a3, a4)
  _elems.Check(a0)
  return r

def Z3_mk_app(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_mk_app)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_mk_const(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_const)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_fresh_func_decl(a0, a1, a2, a3, a4, _elems=Elementaries(_lib.Z3_mk_fresh_func_decl)):
  r = _elems.f(a0, _str_to_bytes(a1), a2, a3, a4)
  _elems.Check(a0)
  return r

def Z3_mk_fresh_const(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_fresh_const)):
  r = _elems.f(a0, _str_to_bytes(a1), a2)
  _elems.Check(a0)
  return r

def Z3_mk_rec_func_decl(a0, a1, a2, a3, a4, _elems=Elementaries(_lib.Z3_mk_rec_func_decl)):
  r = _elems.f(a0, a1, a2, a3, a4)
  _elems.Check(a0)
  return r

def Z3_add_rec_def(a0, a1, a2, a3, a4, _elems=Elementaries(_lib.Z3_add_rec_def)):
  _elems.f(a0, a1, a2, a3, a4)
  _elems.Check(a0)

def Z3_mk_true(a0, _elems=Elementaries(_lib.Z3_mk_true)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_mk_false(a0, _elems=Elementaries(_lib.Z3_mk_false)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_mk_eq(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_eq)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_distinct(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_distinct)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_not(a0, a1, _elems=Elementaries(_lib.Z3_mk_not)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_ite(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_mk_ite)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_mk_iff(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_iff)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_implies(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_implies)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_xor(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_xor)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_and(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_and)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_or(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_or)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_add(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_add)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_mul(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_mul)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_sub(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_sub)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_unary_minus(a0, a1, _elems=Elementaries(_lib.Z3_mk_unary_minus)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_div(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_div)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_mod(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_mod)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_rem(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_rem)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_power(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_power)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_lt(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_lt)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_le(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_le)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_gt(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_gt)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_ge(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_ge)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_divides(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_divides)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_int2real(a0, a1, _elems=Elementaries(_lib.Z3_mk_int2real)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_real2int(a0, a1, _elems=Elementaries(_lib.Z3_mk_real2int)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_is_int(a0, a1, _elems=Elementaries(_lib.Z3_mk_is_int)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_bvnot(a0, a1, _elems=Elementaries(_lib.Z3_mk_bvnot)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_bvredand(a0, a1, _elems=Elementaries(_lib.Z3_mk_bvredand)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_bvredor(a0, a1, _elems=Elementaries(_lib.Z3_mk_bvredor)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_bvand(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_bvand)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_bvor(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_bvor)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_bvxor(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_bvxor)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_bvnand(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_bvnand)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_bvnor(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_bvnor)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_bvxnor(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_bvxnor)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_bvneg(a0, a1, _elems=Elementaries(_lib.Z3_mk_bvneg)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_bvadd(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_bvadd)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_bvsub(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_bvsub)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_bvmul(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_bvmul)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_bvudiv(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_bvudiv)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_bvsdiv(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_bvsdiv)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_bvurem(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_bvurem)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_bvsrem(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_bvsrem)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_bvsmod(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_bvsmod)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_bvult(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_bvult)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_bvslt(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_bvslt)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_bvule(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_bvule)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_bvsle(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_bvsle)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_bvuge(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_bvuge)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_bvsge(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_bvsge)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_bvugt(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_bvugt)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_bvsgt(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_bvsgt)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_concat(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_concat)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_extract(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_mk_extract)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_mk_sign_ext(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_sign_ext)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_zero_ext(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_zero_ext)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_repeat(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_repeat)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_bit2bool(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_bit2bool)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_bvshl(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_bvshl)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_bvlshr(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_bvlshr)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_bvashr(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_bvashr)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_rotate_left(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_rotate_left)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_rotate_right(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_rotate_right)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_ext_rotate_left(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_ext_rotate_left)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_ext_rotate_right(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_ext_rotate_right)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_int2bv(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_int2bv)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_bv2int(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_bv2int)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_bvadd_no_overflow(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_mk_bvadd_no_overflow)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_mk_bvadd_no_underflow(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_bvadd_no_underflow)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_bvsub_no_overflow(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_bvsub_no_overflow)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_bvsub_no_underflow(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_mk_bvsub_no_underflow)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_mk_bvsdiv_no_overflow(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_bvsdiv_no_overflow)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_bvneg_no_overflow(a0, a1, _elems=Elementaries(_lib.Z3_mk_bvneg_no_overflow)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_bvmul_no_overflow(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_mk_bvmul_no_overflow)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_mk_bvmul_no_underflow(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_bvmul_no_underflow)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_select(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_select)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_select_n(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_mk_select_n)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_mk_store(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_mk_store)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_mk_store_n(a0, a1, a2, a3, a4, _elems=Elementaries(_lib.Z3_mk_store_n)):
  r = _elems.f(a0, a1, a2, a3, a4)
  _elems.Check(a0)
  return r

def Z3_mk_const_array(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_const_array)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_map(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_mk_map)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_mk_array_default(a0, a1, _elems=Elementaries(_lib.Z3_mk_array_default)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_as_array(a0, a1, _elems=Elementaries(_lib.Z3_mk_as_array)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_set_has_size(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_set_has_size)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_set_sort(a0, a1, _elems=Elementaries(_lib.Z3_mk_set_sort)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_empty_set(a0, a1, _elems=Elementaries(_lib.Z3_mk_empty_set)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_full_set(a0, a1, _elems=Elementaries(_lib.Z3_mk_full_set)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_set_add(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_set_add)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_set_del(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_set_del)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_set_union(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_set_union)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_set_intersect(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_set_intersect)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_set_difference(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_set_difference)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_set_complement(a0, a1, _elems=Elementaries(_lib.Z3_mk_set_complement)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_set_member(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_set_member)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_set_subset(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_set_subset)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_array_ext(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_array_ext)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_numeral(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_numeral)):
  r = _elems.f(a0, _str_to_bytes(a1), a2)
  _elems.Check(a0)
  return r

def Z3_mk_real(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_real)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_real_int64(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_real_int64)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_int(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_int)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_unsigned_int(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_unsigned_int)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_int64(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_int64)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_unsigned_int64(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_unsigned_int64)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_bv_numeral(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_bv_numeral)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_seq_sort(a0, a1, _elems=Elementaries(_lib.Z3_mk_seq_sort)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_is_seq_sort(a0, a1, _elems=Elementaries(_lib.Z3_is_seq_sort)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_seq_sort_basis(a0, a1, _elems=Elementaries(_lib.Z3_get_seq_sort_basis)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_re_sort(a0, a1, _elems=Elementaries(_lib.Z3_mk_re_sort)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_is_re_sort(a0, a1, _elems=Elementaries(_lib.Z3_is_re_sort)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_re_sort_basis(a0, a1, _elems=Elementaries(_lib.Z3_get_re_sort_basis)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_string_sort(a0, _elems=Elementaries(_lib.Z3_mk_string_sort)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_mk_char_sort(a0, _elems=Elementaries(_lib.Z3_mk_char_sort)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_is_string_sort(a0, a1, _elems=Elementaries(_lib.Z3_is_string_sort)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_is_char_sort(a0, a1, _elems=Elementaries(_lib.Z3_is_char_sort)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_string(a0, a1, _elems=Elementaries(_lib.Z3_mk_string)):
  r = _elems.f(a0, _str_to_bytes(a1))
  _elems.Check(a0)
  return r

def Z3_mk_lstring(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_lstring)):
  r = _elems.f(a0, a1, _str_to_bytes(a2))
  _elems.Check(a0)
  return r

def Z3_mk_u32string(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_u32string)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_is_string(a0, a1, _elems=Elementaries(_lib.Z3_is_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_string(a0, a1, _elems=Elementaries(_lib.Z3_get_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_get_string_bytes(a0, a1, _elems=Elementaries(_lib.Z3_get_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_lstring(a0, a1, a2, _elems=Elementaries(_lib.Z3_get_lstring)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_get_string_length(a0, a1, _elems=Elementaries(_lib.Z3_get_string_length)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_string_contents(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_get_string_contents)):
  _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)

def Z3_mk_seq_empty(a0, a1, _elems=Elementaries(_lib.Z3_mk_seq_empty)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_seq_unit(a0, a1, _elems=Elementaries(_lib.Z3_mk_seq_unit)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_seq_concat(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_seq_concat)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_seq_prefix(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_seq_prefix)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_seq_suffix(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_seq_suffix)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_seq_contains(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_seq_contains)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_str_lt(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_str_lt)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_str_le(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_str_le)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_seq_extract(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_mk_seq_extract)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_mk_seq_replace(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_mk_seq_replace)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_mk_seq_at(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_seq_at)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_seq_nth(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_seq_nth)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_seq_length(a0, a1, _elems=Elementaries(_lib.Z3_mk_seq_length)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_seq_index(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_mk_seq_index)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_mk_seq_last_index(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_seq_last_index)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_str_to_int(a0, a1, _elems=Elementaries(_lib.Z3_mk_str_to_int)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_int_to_str(a0, a1, _elems=Elementaries(_lib.Z3_mk_int_to_str)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_string_to_code(a0, a1, _elems=Elementaries(_lib.Z3_mk_string_to_code)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_string_from_code(a0, a1, _elems=Elementaries(_lib.Z3_mk_string_from_code)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_ubv_to_str(a0, a1, _elems=Elementaries(_lib.Z3_mk_ubv_to_str)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_sbv_to_str(a0, a1, _elems=Elementaries(_lib.Z3_mk_sbv_to_str)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_seq_to_re(a0, a1, _elems=Elementaries(_lib.Z3_mk_seq_to_re)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_seq_in_re(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_seq_in_re)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_re_plus(a0, a1, _elems=Elementaries(_lib.Z3_mk_re_plus)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_re_star(a0, a1, _elems=Elementaries(_lib.Z3_mk_re_star)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_re_option(a0, a1, _elems=Elementaries(_lib.Z3_mk_re_option)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_re_union(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_re_union)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_re_concat(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_re_concat)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_re_range(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_re_range)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_re_allchar(a0, a1, _elems=Elementaries(_lib.Z3_mk_re_allchar)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_re_loop(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_mk_re_loop)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_mk_re_power(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_re_power)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_re_intersect(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_re_intersect)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_re_complement(a0, a1, _elems=Elementaries(_lib.Z3_mk_re_complement)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_re_diff(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_re_diff)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_re_empty(a0, a1, _elems=Elementaries(_lib.Z3_mk_re_empty)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_re_full(a0, a1, _elems=Elementaries(_lib.Z3_mk_re_full)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_char(a0, a1, _elems=Elementaries(_lib.Z3_mk_char)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_char_le(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_char_le)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_char_to_int(a0, a1, _elems=Elementaries(_lib.Z3_mk_char_to_int)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_char_to_bv(a0, a1, _elems=Elementaries(_lib.Z3_mk_char_to_bv)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_char_from_bv(a0, a1, _elems=Elementaries(_lib.Z3_mk_char_from_bv)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_char_is_digit(a0, a1, _elems=Elementaries(_lib.Z3_mk_char_is_digit)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_linear_order(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_linear_order)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_partial_order(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_partial_order)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_piecewise_linear_order(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_piecewise_linear_order)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_tree_order(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_tree_order)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_transitive_closure(a0, a1, _elems=Elementaries(_lib.Z3_mk_transitive_closure)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_pattern(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_pattern)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_bound(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_bound)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_forall(a0, a1, a2, a3, a4, a5, a6, a7, _elems=Elementaries(_lib.Z3_mk_forall)):
  r = _elems.f(a0, a1, a2, a3, a4, a5, a6, a7)
  _elems.Check(a0)
  return r

def Z3_mk_exists(a0, a1, a2, a3, a4, a5, a6, a7, _elems=Elementaries(_lib.Z3_mk_exists)):
  r = _elems.f(a0, a1, a2, a3, a4, a5, a6, a7)
  _elems.Check(a0)
  return r

def Z3_mk_quantifier(a0, a1, a2, a3, a4, a5, a6, a7, a8, _elems=Elementaries(_lib.Z3_mk_quantifier)):
  r = _elems.f(a0, a1, a2, a3, a4, a5, a6, a7, a8)
  _elems.Check(a0)
  return r

def Z3_mk_quantifier_ex(a0, a1, a2, a3, a4, a5, a6, a7, a8, a9, a10, a11, a12, _elems=Elementaries(_lib.Z3_mk_quantifier_ex)):
  r = _elems.f(a0, a1, a2, a3, a4, a5, a6, a7, a8, a9, a10, a11, a12)
  _elems.Check(a0)
  return r

def Z3_mk_forall_const(a0, a1, a2, a3, a4, a5, a6, _elems=Elementaries(_lib.Z3_mk_forall_const)):
  r = _elems.f(a0, a1, a2, a3, a4, a5, a6)
  _elems.Check(a0)
  return r

def Z3_mk_exists_const(a0, a1, a2, a3, a4, a5, a6, _elems=Elementaries(_lib.Z3_mk_exists_const)):
  r = _elems.f(a0, a1, a2, a3, a4, a5, a6)
  _elems.Check(a0)
  return r

def Z3_mk_quantifier_const(a0, a1, a2, a3, a4, a5, a6, a7, _elems=Elementaries(_lib.Z3_mk_quantifier_const)):
  r = _elems.f(a0, a1, a2, a3, a4, a5, a6, a7)
  _elems.Check(a0)
  return r

def Z3_mk_quantifier_const_ex(a0, a1, a2, a3, a4, a5, a6, a7, a8, a9, a10, a11, _elems=Elementaries(_lib.Z3_mk_quantifier_const_ex)):
  r = _elems.f(a0, a1, a2, a3, a4, a5, a6, a7, a8, a9, a10, a11)
  _elems.Check(a0)
  return r

def Z3_mk_lambda(a0, a1, a2, a3, a4, _elems=Elementaries(_lib.Z3_mk_lambda)):
  r = _elems.f(a0, a1, a2, a3, a4)
  _elems.Check(a0)
  return r

def Z3_mk_lambda_const(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_mk_lambda_const)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_get_symbol_kind(a0, a1, _elems=Elementaries(_lib.Z3_get_symbol_kind)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_symbol_int(a0, a1, _elems=Elementaries(_lib.Z3_get_symbol_int)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_symbol_string(a0, a1, _elems=Elementaries(_lib.Z3_get_symbol_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_get_symbol_string_bytes(a0, a1, _elems=Elementaries(_lib.Z3_get_symbol_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_sort_name(a0, a1, _elems=Elementaries(_lib.Z3_get_sort_name)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_sort_id(a0, a1, _elems=Elementaries(_lib.Z3_get_sort_id)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_sort_to_ast(a0, a1, _elems=Elementaries(_lib.Z3_sort_to_ast)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_is_eq_sort(a0, a1, a2, _elems=Elementaries(_lib.Z3_is_eq_sort)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_get_sort_kind(a0, a1, _elems=Elementaries(_lib.Z3_get_sort_kind)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_bv_sort_size(a0, a1, _elems=Elementaries(_lib.Z3_get_bv_sort_size)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_finite_domain_sort_size(a0, a1, a2, _elems=Elementaries(_lib.Z3_get_finite_domain_sort_size)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_get_array_sort_domain(a0, a1, _elems=Elementaries(_lib.Z3_get_array_sort_domain)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_array_sort_domain_n(a0, a1, a2, _elems=Elementaries(_lib.Z3_get_array_sort_domain_n)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_get_array_sort_range(a0, a1, _elems=Elementaries(_lib.Z3_get_array_sort_range)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_tuple_sort_mk_decl(a0, a1, _elems=Elementaries(_lib.Z3_get_tuple_sort_mk_decl)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_tuple_sort_num_fields(a0, a1, _elems=Elementaries(_lib.Z3_get_tuple_sort_num_fields)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_tuple_sort_field_decl(a0, a1, a2, _elems=Elementaries(_lib.Z3_get_tuple_sort_field_decl)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_get_datatype_sort_num_constructors(a0, a1, _elems=Elementaries(_lib.Z3_get_datatype_sort_num_constructors)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_datatype_sort_constructor(a0, a1, a2, _elems=Elementaries(_lib.Z3_get_datatype_sort_constructor)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_get_datatype_sort_recognizer(a0, a1, a2, _elems=Elementaries(_lib.Z3_get_datatype_sort_recognizer)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_get_datatype_sort_constructor_accessor(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_get_datatype_sort_constructor_accessor)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_datatype_update_field(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_datatype_update_field)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_get_relation_arity(a0, a1, _elems=Elementaries(_lib.Z3_get_relation_arity)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_relation_column(a0, a1, a2, _elems=Elementaries(_lib.Z3_get_relation_column)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_atmost(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_mk_atmost)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_mk_atleast(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_mk_atleast)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_mk_pble(a0, a1, a2, a3, a4, _elems=Elementaries(_lib.Z3_mk_pble)):
  r = _elems.f(a0, a1, a2, a3, a4)
  _elems.Check(a0)
  return r

def Z3_mk_pbge(a0, a1, a2, a3, a4, _elems=Elementaries(_lib.Z3_mk_pbge)):
  r = _elems.f(a0, a1, a2, a3, a4)
  _elems.Check(a0)
  return r

def Z3_mk_pbeq(a0, a1, a2, a3, a4, _elems=Elementaries(_lib.Z3_mk_pbeq)):
  r = _elems.f(a0, a1, a2, a3, a4)
  _elems.Check(a0)
  return r

def Z3_func_decl_to_ast(a0, a1, _elems=Elementaries(_lib.Z3_func_decl_to_ast)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_is_eq_func_decl(a0, a1, a2, _elems=Elementaries(_lib.Z3_is_eq_func_decl)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_get_func_decl_id(a0, a1, _elems=Elementaries(_lib.Z3_get_func_decl_id)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_decl_name(a0, a1, _elems=Elementaries(_lib.Z3_get_decl_name)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_decl_kind(a0, a1, _elems=Elementaries(_lib.Z3_get_decl_kind)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_domain_size(a0, a1, _elems=Elementaries(_lib.Z3_get_domain_size)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_arity(a0, a1, _elems=Elementaries(_lib.Z3_get_arity)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_domain(a0, a1, a2, _elems=Elementaries(_lib.Z3_get_domain)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_get_range(a0, a1, _elems=Elementaries(_lib.Z3_get_range)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_decl_num_parameters(a0, a1, _elems=Elementaries(_lib.Z3_get_decl_num_parameters)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_decl_parameter_kind(a0, a1, a2, _elems=Elementaries(_lib.Z3_get_decl_parameter_kind)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_get_decl_int_parameter(a0, a1, a2, _elems=Elementaries(_lib.Z3_get_decl_int_parameter)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_get_decl_double_parameter(a0, a1, a2, _elems=Elementaries(_lib.Z3_get_decl_double_parameter)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_get_decl_symbol_parameter(a0, a1, a2, _elems=Elementaries(_lib.Z3_get_decl_symbol_parameter)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_get_decl_sort_parameter(a0, a1, a2, _elems=Elementaries(_lib.Z3_get_decl_sort_parameter)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_get_decl_ast_parameter(a0, a1, a2, _elems=Elementaries(_lib.Z3_get_decl_ast_parameter)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_get_decl_func_decl_parameter(a0, a1, a2, _elems=Elementaries(_lib.Z3_get_decl_func_decl_parameter)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_get_decl_rational_parameter(a0, a1, a2, _elems=Elementaries(_lib.Z3_get_decl_rational_parameter)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_get_decl_rational_parameter_bytes(a0, a1, a2, _elems=Elementaries(_lib.Z3_get_decl_rational_parameter)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_app_to_ast(a0, a1, _elems=Elementaries(_lib.Z3_app_to_ast)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_app_decl(a0, a1, _elems=Elementaries(_lib.Z3_get_app_decl)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_app_num_args(a0, a1, _elems=Elementaries(_lib.Z3_get_app_num_args)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_app_arg(a0, a1, a2, _elems=Elementaries(_lib.Z3_get_app_arg)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_is_eq_ast(a0, a1, a2, _elems=Elementaries(_lib.Z3_is_eq_ast)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_get_ast_id(a0, a1, _elems=Elementaries(_lib.Z3_get_ast_id)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_ast_hash(a0, a1, _elems=Elementaries(_lib.Z3_get_ast_hash)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_sort(a0, a1, _elems=Elementaries(_lib.Z3_get_sort)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_is_well_sorted(a0, a1, _elems=Elementaries(_lib.Z3_is_well_sorted)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_bool_value(a0, a1, _elems=Elementaries(_lib.Z3_get_bool_value)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_ast_kind(a0, a1, _elems=Elementaries(_lib.Z3_get_ast_kind)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_is_app(a0, a1, _elems=Elementaries(_lib.Z3_is_app)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_is_numeral_ast(a0, a1, _elems=Elementaries(_lib.Z3_is_numeral_ast)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_is_algebraic_number(a0, a1, _elems=Elementaries(_lib.Z3_is_algebraic_number)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_to_app(a0, a1, _elems=Elementaries(_lib.Z3_to_app)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_to_func_decl(a0, a1, _elems=Elementaries(_lib.Z3_to_func_decl)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_numeral_string(a0, a1, _elems=Elementaries(_lib.Z3_get_numeral_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_get_numeral_string_bytes(a0, a1, _elems=Elementaries(_lib.Z3_get_numeral_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_numeral_binary_string(a0, a1, _elems=Elementaries(_lib.Z3_get_numeral_binary_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_get_numeral_binary_string_bytes(a0, a1, _elems=Elementaries(_lib.Z3_get_numeral_binary_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_numeral_decimal_string(a0, a1, a2, _elems=Elementaries(_lib.Z3_get_numeral_decimal_string)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_get_numeral_decimal_string_bytes(a0, a1, a2, _elems=Elementaries(_lib.Z3_get_numeral_decimal_string)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_get_numeral_double(a0, a1, _elems=Elementaries(_lib.Z3_get_numeral_double)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_numerator(a0, a1, _elems=Elementaries(_lib.Z3_get_numerator)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_denominator(a0, a1, _elems=Elementaries(_lib.Z3_get_denominator)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_numeral_small(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_get_numeral_small)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_get_numeral_int(a0, a1, a2, _elems=Elementaries(_lib.Z3_get_numeral_int)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_get_numeral_uint(a0, a1, a2, _elems=Elementaries(_lib.Z3_get_numeral_uint)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_get_numeral_uint64(a0, a1, a2, _elems=Elementaries(_lib.Z3_get_numeral_uint64)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_get_numeral_int64(a0, a1, a2, _elems=Elementaries(_lib.Z3_get_numeral_int64)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_get_numeral_rational_int64(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_get_numeral_rational_int64)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_get_algebraic_number_lower(a0, a1, a2, _elems=Elementaries(_lib.Z3_get_algebraic_number_lower)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_get_algebraic_number_upper(a0, a1, a2, _elems=Elementaries(_lib.Z3_get_algebraic_number_upper)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_pattern_to_ast(a0, a1, _elems=Elementaries(_lib.Z3_pattern_to_ast)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_pattern_num_terms(a0, a1, _elems=Elementaries(_lib.Z3_get_pattern_num_terms)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_pattern(a0, a1, a2, _elems=Elementaries(_lib.Z3_get_pattern)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_get_index_value(a0, a1, _elems=Elementaries(_lib.Z3_get_index_value)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_is_quantifier_forall(a0, a1, _elems=Elementaries(_lib.Z3_is_quantifier_forall)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_is_quantifier_exists(a0, a1, _elems=Elementaries(_lib.Z3_is_quantifier_exists)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_is_lambda(a0, a1, _elems=Elementaries(_lib.Z3_is_lambda)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_quantifier_weight(a0, a1, _elems=Elementaries(_lib.Z3_get_quantifier_weight)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_quantifier_skolem_id(a0, a1, _elems=Elementaries(_lib.Z3_get_quantifier_skolem_id)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_quantifier_id(a0, a1, _elems=Elementaries(_lib.Z3_get_quantifier_id)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_quantifier_num_patterns(a0, a1, _elems=Elementaries(_lib.Z3_get_quantifier_num_patterns)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_quantifier_pattern_ast(a0, a1, a2, _elems=Elementaries(_lib.Z3_get_quantifier_pattern_ast)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_get_quantifier_num_no_patterns(a0, a1, _elems=Elementaries(_lib.Z3_get_quantifier_num_no_patterns)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_quantifier_no_pattern_ast(a0, a1, a2, _elems=Elementaries(_lib.Z3_get_quantifier_no_pattern_ast)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_get_quantifier_num_bound(a0, a1, _elems=Elementaries(_lib.Z3_get_quantifier_num_bound)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_quantifier_bound_name(a0, a1, a2, _elems=Elementaries(_lib.Z3_get_quantifier_bound_name)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_get_quantifier_bound_sort(a0, a1, a2, _elems=Elementaries(_lib.Z3_get_quantifier_bound_sort)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_get_quantifier_body(a0, a1, _elems=Elementaries(_lib.Z3_get_quantifier_body)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_simplify(a0, a1, _elems=Elementaries(_lib.Z3_simplify)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_simplify_ex(a0, a1, a2, _elems=Elementaries(_lib.Z3_simplify_ex)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_simplify_get_help(a0, _elems=Elementaries(_lib.Z3_simplify_get_help)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_simplify_get_help_bytes(a0, _elems=Elementaries(_lib.Z3_simplify_get_help)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_simplify_get_param_descrs(a0, _elems=Elementaries(_lib.Z3_simplify_get_param_descrs)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_update_term(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_update_term)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_substitute(a0, a1, a2, a3, a4, _elems=Elementaries(_lib.Z3_substitute)):
  r = _elems.f(a0, a1, a2, a3, a4)
  _elems.Check(a0)
  return r

def Z3_substitute_vars(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_substitute_vars)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_substitute_funs(a0, a1, a2, a3, a4, _elems=Elementaries(_lib.Z3_substitute_funs)):
  r = _elems.f(a0, a1, a2, a3, a4)
  _elems.Check(a0)
  return r

def Z3_translate(a0, a1, a2, _elems=Elementaries(_lib.Z3_translate)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_model(a0, _elems=Elementaries(_lib.Z3_mk_model)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_model_inc_ref(a0, a1, _elems=Elementaries(_lib.Z3_model_inc_ref)):
  _elems.f(a0, a1)
  _elems.Check(a0)

def Z3_model_dec_ref(a0, a1, _elems=Elementaries(_lib.Z3_model_dec_ref)):
  _elems.f(a0, a1)

def Z3_model_eval(a0, a1, a2, a3, a4, _elems=Elementaries(_lib.Z3_model_eval)):
  r = _elems.f(a0, a1, a2, a3, a4)
  _elems.Check(a0)
  return r

def Z3_model_get_const_interp(a0, a1, a2, _elems=Elementaries(_lib.Z3_model_get_const_interp)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_model_has_interp(a0, a1, a2, _elems=Elementaries(_lib.Z3_model_has_interp)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_model_get_func_interp(a0, a1, a2, _elems=Elementaries(_lib.Z3_model_get_func_interp)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_model_get_num_consts(a0, a1, _elems=Elementaries(_lib.Z3_model_get_num_consts)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_model_get_const_decl(a0, a1, a2, _elems=Elementaries(_lib.Z3_model_get_const_decl)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_model_get_num_funcs(a0, a1, _elems=Elementaries(_lib.Z3_model_get_num_funcs)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_model_get_func_decl(a0, a1, a2, _elems=Elementaries(_lib.Z3_model_get_func_decl)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_model_get_num_sorts(a0, a1, _elems=Elementaries(_lib.Z3_model_get_num_sorts)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_model_get_sort(a0, a1, a2, _elems=Elementaries(_lib.Z3_model_get_sort)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_model_get_sort_universe(a0, a1, a2, _elems=Elementaries(_lib.Z3_model_get_sort_universe)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_model_translate(a0, a1, a2, _elems=Elementaries(_lib.Z3_model_translate)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_is_as_array(a0, a1, _elems=Elementaries(_lib.Z3_is_as_array)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_as_array_func_decl(a0, a1, _elems=Elementaries(_lib.Z3_get_as_array_func_decl)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_add_func_interp(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_add_func_interp)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_add_const_interp(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_add_const_interp)):
  _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)

def Z3_func_interp_inc_ref(a0, a1, _elems=Elementaries(_lib.Z3_func_interp_inc_ref)):
  _elems.f(a0, a1)
  _elems.Check(a0)

def Z3_func_interp_dec_ref(a0, a1, _elems=Elementaries(_lib.Z3_func_interp_dec_ref)):
  _elems.f(a0, a1)

def Z3_func_interp_get_num_entries(a0, a1, _elems=Elementaries(_lib.Z3_func_interp_get_num_entries)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_func_interp_get_entry(a0, a1, a2, _elems=Elementaries(_lib.Z3_func_interp_get_entry)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_func_interp_get_else(a0, a1, _elems=Elementaries(_lib.Z3_func_interp_get_else)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_func_interp_set_else(a0, a1, a2, _elems=Elementaries(_lib.Z3_func_interp_set_else)):
  _elems.f(a0, a1, a2)
  _elems.Check(a0)

def Z3_func_interp_get_arity(a0, a1, _elems=Elementaries(_lib.Z3_func_interp_get_arity)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_func_interp_add_entry(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_func_interp_add_entry)):
  _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)

def Z3_func_entry_inc_ref(a0, a1, _elems=Elementaries(_lib.Z3_func_entry_inc_ref)):
  _elems.f(a0, a1)
  _elems.Check(a0)

def Z3_func_entry_dec_ref(a0, a1, _elems=Elementaries(_lib.Z3_func_entry_dec_ref)):
  _elems.f(a0, a1)

def Z3_func_entry_get_value(a0, a1, _elems=Elementaries(_lib.Z3_func_entry_get_value)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_func_entry_get_num_args(a0, a1, _elems=Elementaries(_lib.Z3_func_entry_get_num_args)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_func_entry_get_arg(a0, a1, a2, _elems=Elementaries(_lib.Z3_func_entry_get_arg)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_open_log(a0, _elems=Elementaries(_lib.Z3_open_log)):
  r = _elems.f(_str_to_bytes(a0))
  return r

def Z3_append_log(a0, _elems=Elementaries(_lib.Z3_append_log)):
  _elems.f(_str_to_bytes(a0))

def Z3_close_log(_elems=Elementaries(_lib.Z3_close_log)):
  _elems.f()

def Z3_toggle_warning_messages(a0, _elems=Elementaries(_lib.Z3_toggle_warning_messages)):
  _elems.f(a0)

def Z3_set_ast_print_mode(a0, a1, _elems=Elementaries(_lib.Z3_set_ast_print_mode)):
  _elems.f(a0, a1)
  _elems.Check(a0)

def Z3_ast_to_string(a0, a1, _elems=Elementaries(_lib.Z3_ast_to_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_ast_to_string_bytes(a0, a1, _elems=Elementaries(_lib.Z3_ast_to_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_pattern_to_string(a0, a1, _elems=Elementaries(_lib.Z3_pattern_to_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_pattern_to_string_bytes(a0, a1, _elems=Elementaries(_lib.Z3_pattern_to_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_sort_to_string(a0, a1, _elems=Elementaries(_lib.Z3_sort_to_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_sort_to_string_bytes(a0, a1, _elems=Elementaries(_lib.Z3_sort_to_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_func_decl_to_string(a0, a1, _elems=Elementaries(_lib.Z3_func_decl_to_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_func_decl_to_string_bytes(a0, a1, _elems=Elementaries(_lib.Z3_func_decl_to_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_model_to_string(a0, a1, _elems=Elementaries(_lib.Z3_model_to_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_model_to_string_bytes(a0, a1, _elems=Elementaries(_lib.Z3_model_to_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_benchmark_to_smtlib_string(a0, a1, a2, a3, a4, a5, a6, a7, _elems=Elementaries(_lib.Z3_benchmark_to_smtlib_string)):
  r = _elems.f(a0, _str_to_bytes(a1), _str_to_bytes(a2), _str_to_bytes(a3), _str_to_bytes(a4), a5, a6, a7)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_benchmark_to_smtlib_string_bytes(a0, a1, a2, a3, a4, a5, a6, a7, _elems=Elementaries(_lib.Z3_benchmark_to_smtlib_string)):
  r = _elems.f(a0, _str_to_bytes(a1), _str_to_bytes(a2), _str_to_bytes(a3), _str_to_bytes(a4), a5, a6, a7)
  _elems.Check(a0)
  return r

def Z3_parse_smtlib2_string(a0, a1, a2, a3, a4, a5, a6, a7, _elems=Elementaries(_lib.Z3_parse_smtlib2_string)):
  r = _elems.f(a0, _str_to_bytes(a1), a2, a3, a4, a5, a6, a7)
  _elems.Check(a0)
  return r

def Z3_parse_smtlib2_file(a0, a1, a2, a3, a4, a5, a6, a7, _elems=Elementaries(_lib.Z3_parse_smtlib2_file)):
  r = _elems.f(a0, _str_to_bytes(a1), a2, a3, a4, a5, a6, a7)
  _elems.Check(a0)
  return r

def Z3_eval_smtlib2_string(a0, a1, _elems=Elementaries(_lib.Z3_eval_smtlib2_string)):
  r = _elems.f(a0, _str_to_bytes(a1))
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_eval_smtlib2_string_bytes(a0, a1, _elems=Elementaries(_lib.Z3_eval_smtlib2_string)):
  r = _elems.f(a0, _str_to_bytes(a1))
  _elems.Check(a0)
  return r

def Z3_mk_parser_context(a0, _elems=Elementaries(_lib.Z3_mk_parser_context)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_parser_context_inc_ref(a0, a1, _elems=Elementaries(_lib.Z3_parser_context_inc_ref)):
  _elems.f(a0, a1)
  _elems.Check(a0)

def Z3_parser_context_dec_ref(a0, a1, _elems=Elementaries(_lib.Z3_parser_context_dec_ref)):
  _elems.f(a0, a1)
  _elems.Check(a0)

def Z3_parser_context_add_sort(a0, a1, a2, _elems=Elementaries(_lib.Z3_parser_context_add_sort)):
  _elems.f(a0, a1, a2)
  _elems.Check(a0)

def Z3_parser_context_add_decl(a0, a1, a2, _elems=Elementaries(_lib.Z3_parser_context_add_decl)):
  _elems.f(a0, a1, a2)
  _elems.Check(a0)

def Z3_parser_context_from_string(a0, a1, a2, _elems=Elementaries(_lib.Z3_parser_context_from_string)):
  r = _elems.f(a0, a1, _str_to_bytes(a2))
  _elems.Check(a0)
  return r

def Z3_get_error_code(a0, _elems=Elementaries(_lib.Z3_get_error_code)):
  r = _elems.f(a0)
  return r

def Z3_set_error(a0, a1, _elems=Elementaries(_lib.Z3_set_error)):
  _elems.f(a0, a1)
  _elems.Check(a0)

def Z3_get_error_msg(a0, a1, _elems=Elementaries(_lib.Z3_get_error_msg)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_get_error_msg_bytes(a0, a1, _elems=Elementaries(_lib.Z3_get_error_msg)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_version(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_get_version)):
  _elems.f(a0, a1, a2, a3)

def Z3_get_full_version(_elems=Elementaries(_lib.Z3_get_full_version)):
  r = _elems.f()
  return _to_pystr(r)

def Z3_get_full_version_bytes(_elems=Elementaries(_lib.Z3_get_full_version)):
  r = _elems.f()
  return r

def Z3_enable_trace(a0, _elems=Elementaries(_lib.Z3_enable_trace)):
  _elems.f(_str_to_bytes(a0))

def Z3_disable_trace(a0, _elems=Elementaries(_lib.Z3_disable_trace)):
  _elems.f(_str_to_bytes(a0))

def Z3_reset_memory(_elems=Elementaries(_lib.Z3_reset_memory)):
  _elems.f()

def Z3_finalize_memory(_elems=Elementaries(_lib.Z3_finalize_memory)):
  _elems.f()

def Z3_mk_goal(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_mk_goal)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_goal_inc_ref(a0, a1, _elems=Elementaries(_lib.Z3_goal_inc_ref)):
  _elems.f(a0, a1)
  _elems.Check(a0)

def Z3_goal_dec_ref(a0, a1, _elems=Elementaries(_lib.Z3_goal_dec_ref)):
  _elems.f(a0, a1)

def Z3_goal_precision(a0, a1, _elems=Elementaries(_lib.Z3_goal_precision)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_goal_assert(a0, a1, a2, _elems=Elementaries(_lib.Z3_goal_assert)):
  _elems.f(a0, a1, a2)
  _elems.Check(a0)

def Z3_goal_inconsistent(a0, a1, _elems=Elementaries(_lib.Z3_goal_inconsistent)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_goal_depth(a0, a1, _elems=Elementaries(_lib.Z3_goal_depth)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_goal_reset(a0, a1, _elems=Elementaries(_lib.Z3_goal_reset)):
  _elems.f(a0, a1)
  _elems.Check(a0)

def Z3_goal_size(a0, a1, _elems=Elementaries(_lib.Z3_goal_size)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_goal_formula(a0, a1, a2, _elems=Elementaries(_lib.Z3_goal_formula)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_goal_num_exprs(a0, a1, _elems=Elementaries(_lib.Z3_goal_num_exprs)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_goal_is_decided_sat(a0, a1, _elems=Elementaries(_lib.Z3_goal_is_decided_sat)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_goal_is_decided_unsat(a0, a1, _elems=Elementaries(_lib.Z3_goal_is_decided_unsat)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_goal_translate(a0, a1, a2, _elems=Elementaries(_lib.Z3_goal_translate)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_goal_convert_model(a0, a1, a2, _elems=Elementaries(_lib.Z3_goal_convert_model)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_goal_to_string(a0, a1, _elems=Elementaries(_lib.Z3_goal_to_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_goal_to_string_bytes(a0, a1, _elems=Elementaries(_lib.Z3_goal_to_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_goal_to_dimacs_string(a0, a1, a2, _elems=Elementaries(_lib.Z3_goal_to_dimacs_string)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_goal_to_dimacs_string_bytes(a0, a1, a2, _elems=Elementaries(_lib.Z3_goal_to_dimacs_string)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_tactic(a0, a1, _elems=Elementaries(_lib.Z3_mk_tactic)):
  r = _elems.f(a0, _str_to_bytes(a1))
  _elems.Check(a0)
  return r

def Z3_tactic_inc_ref(a0, a1, _elems=Elementaries(_lib.Z3_tactic_inc_ref)):
  _elems.f(a0, a1)
  _elems.Check(a0)

def Z3_tactic_dec_ref(a0, a1, _elems=Elementaries(_lib.Z3_tactic_dec_ref)):
  _elems.f(a0, a1)

def Z3_mk_probe(a0, a1, _elems=Elementaries(_lib.Z3_mk_probe)):
  r = _elems.f(a0, _str_to_bytes(a1))
  _elems.Check(a0)
  return r

def Z3_probe_inc_ref(a0, a1, _elems=Elementaries(_lib.Z3_probe_inc_ref)):
  _elems.f(a0, a1)
  _elems.Check(a0)

def Z3_probe_dec_ref(a0, a1, _elems=Elementaries(_lib.Z3_probe_dec_ref)):
  _elems.f(a0, a1)

def Z3_tactic_and_then(a0, a1, a2, _elems=Elementaries(_lib.Z3_tactic_and_then)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_tactic_or_else(a0, a1, a2, _elems=Elementaries(_lib.Z3_tactic_or_else)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_tactic_par_or(a0, a1, a2, _elems=Elementaries(_lib.Z3_tactic_par_or)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_tactic_par_and_then(a0, a1, a2, _elems=Elementaries(_lib.Z3_tactic_par_and_then)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_tactic_try_for(a0, a1, a2, _elems=Elementaries(_lib.Z3_tactic_try_for)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_tactic_when(a0, a1, a2, _elems=Elementaries(_lib.Z3_tactic_when)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_tactic_cond(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_tactic_cond)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_tactic_repeat(a0, a1, a2, _elems=Elementaries(_lib.Z3_tactic_repeat)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_tactic_skip(a0, _elems=Elementaries(_lib.Z3_tactic_skip)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_tactic_fail(a0, _elems=Elementaries(_lib.Z3_tactic_fail)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_tactic_fail_if(a0, a1, _elems=Elementaries(_lib.Z3_tactic_fail_if)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_tactic_fail_if_not_decided(a0, _elems=Elementaries(_lib.Z3_tactic_fail_if_not_decided)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_tactic_using_params(a0, a1, a2, _elems=Elementaries(_lib.Z3_tactic_using_params)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_simplifier(a0, a1, _elems=Elementaries(_lib.Z3_mk_simplifier)):
  r = _elems.f(a0, _str_to_bytes(a1))
  _elems.Check(a0)
  return r

def Z3_simplifier_inc_ref(a0, a1, _elems=Elementaries(_lib.Z3_simplifier_inc_ref)):
  _elems.f(a0, a1)
  _elems.Check(a0)

def Z3_simplifier_dec_ref(a0, a1, _elems=Elementaries(_lib.Z3_simplifier_dec_ref)):
  _elems.f(a0, a1)

def Z3_solver_add_simplifier(a0, a1, a2, _elems=Elementaries(_lib.Z3_solver_add_simplifier)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_simplifier_and_then(a0, a1, a2, _elems=Elementaries(_lib.Z3_simplifier_and_then)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_simplifier_using_params(a0, a1, a2, _elems=Elementaries(_lib.Z3_simplifier_using_params)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_get_num_simplifiers(a0, _elems=Elementaries(_lib.Z3_get_num_simplifiers)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_get_simplifier_name(a0, a1, _elems=Elementaries(_lib.Z3_get_simplifier_name)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_get_simplifier_name_bytes(a0, a1, _elems=Elementaries(_lib.Z3_get_simplifier_name)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_simplifier_get_help(a0, a1, _elems=Elementaries(_lib.Z3_simplifier_get_help)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_simplifier_get_help_bytes(a0, a1, _elems=Elementaries(_lib.Z3_simplifier_get_help)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_simplifier_get_param_descrs(a0, a1, _elems=Elementaries(_lib.Z3_simplifier_get_param_descrs)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_simplifier_get_descr(a0, a1, _elems=Elementaries(_lib.Z3_simplifier_get_descr)):
  r = _elems.f(a0, _str_to_bytes(a1))
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_simplifier_get_descr_bytes(a0, a1, _elems=Elementaries(_lib.Z3_simplifier_get_descr)):
  r = _elems.f(a0, _str_to_bytes(a1))
  _elems.Check(a0)
  return r

def Z3_probe_const(a0, a1, _elems=Elementaries(_lib.Z3_probe_const)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_probe_lt(a0, a1, a2, _elems=Elementaries(_lib.Z3_probe_lt)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_probe_gt(a0, a1, a2, _elems=Elementaries(_lib.Z3_probe_gt)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_probe_le(a0, a1, a2, _elems=Elementaries(_lib.Z3_probe_le)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_probe_ge(a0, a1, a2, _elems=Elementaries(_lib.Z3_probe_ge)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_probe_eq(a0, a1, a2, _elems=Elementaries(_lib.Z3_probe_eq)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_probe_and(a0, a1, a2, _elems=Elementaries(_lib.Z3_probe_and)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_probe_or(a0, a1, a2, _elems=Elementaries(_lib.Z3_probe_or)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_probe_not(a0, a1, _elems=Elementaries(_lib.Z3_probe_not)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_num_tactics(a0, _elems=Elementaries(_lib.Z3_get_num_tactics)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_get_tactic_name(a0, a1, _elems=Elementaries(_lib.Z3_get_tactic_name)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_get_tactic_name_bytes(a0, a1, _elems=Elementaries(_lib.Z3_get_tactic_name)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_get_num_probes(a0, _elems=Elementaries(_lib.Z3_get_num_probes)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_get_probe_name(a0, a1, _elems=Elementaries(_lib.Z3_get_probe_name)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_get_probe_name_bytes(a0, a1, _elems=Elementaries(_lib.Z3_get_probe_name)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_tactic_get_help(a0, a1, _elems=Elementaries(_lib.Z3_tactic_get_help)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_tactic_get_help_bytes(a0, a1, _elems=Elementaries(_lib.Z3_tactic_get_help)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_tactic_get_param_descrs(a0, a1, _elems=Elementaries(_lib.Z3_tactic_get_param_descrs)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_tactic_get_descr(a0, a1, _elems=Elementaries(_lib.Z3_tactic_get_descr)):
  r = _elems.f(a0, _str_to_bytes(a1))
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_tactic_get_descr_bytes(a0, a1, _elems=Elementaries(_lib.Z3_tactic_get_descr)):
  r = _elems.f(a0, _str_to_bytes(a1))
  _elems.Check(a0)
  return r

def Z3_probe_get_descr(a0, a1, _elems=Elementaries(_lib.Z3_probe_get_descr)):
  r = _elems.f(a0, _str_to_bytes(a1))
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_probe_get_descr_bytes(a0, a1, _elems=Elementaries(_lib.Z3_probe_get_descr)):
  r = _elems.f(a0, _str_to_bytes(a1))
  _elems.Check(a0)
  return r

def Z3_probe_apply(a0, a1, a2, _elems=Elementaries(_lib.Z3_probe_apply)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_tactic_apply(a0, a1, a2, _elems=Elementaries(_lib.Z3_tactic_apply)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_tactic_apply_ex(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_tactic_apply_ex)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_apply_result_inc_ref(a0, a1, _elems=Elementaries(_lib.Z3_apply_result_inc_ref)):
  _elems.f(a0, a1)
  _elems.Check(a0)

def Z3_apply_result_dec_ref(a0, a1, _elems=Elementaries(_lib.Z3_apply_result_dec_ref)):
  _elems.f(a0, a1)

def Z3_apply_result_to_string(a0, a1, _elems=Elementaries(_lib.Z3_apply_result_to_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_apply_result_to_string_bytes(a0, a1, _elems=Elementaries(_lib.Z3_apply_result_to_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_apply_result_get_num_subgoals(a0, a1, _elems=Elementaries(_lib.Z3_apply_result_get_num_subgoals)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_apply_result_get_subgoal(a0, a1, a2, _elems=Elementaries(_lib.Z3_apply_result_get_subgoal)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_solver(a0, _elems=Elementaries(_lib.Z3_mk_solver)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_mk_simple_solver(a0, _elems=Elementaries(_lib.Z3_mk_simple_solver)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_mk_solver_for_logic(a0, a1, _elems=Elementaries(_lib.Z3_mk_solver_for_logic)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_solver_from_tactic(a0, a1, _elems=Elementaries(_lib.Z3_mk_solver_from_tactic)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_solver_translate(a0, a1, a2, _elems=Elementaries(_lib.Z3_solver_translate)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_solver_import_model_converter(a0, a1, a2, _elems=Elementaries(_lib.Z3_solver_import_model_converter)):
  _elems.f(a0, a1, a2)
  _elems.Check(a0)

def Z3_solver_get_help(a0, a1, _elems=Elementaries(_lib.Z3_solver_get_help)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_solver_get_help_bytes(a0, a1, _elems=Elementaries(_lib.Z3_solver_get_help)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_solver_get_param_descrs(a0, a1, _elems=Elementaries(_lib.Z3_solver_get_param_descrs)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_solver_set_params(a0, a1, a2, _elems=Elementaries(_lib.Z3_solver_set_params)):
  _elems.f(a0, a1, a2)
  _elems.Check(a0)

def Z3_solver_inc_ref(a0, a1, _elems=Elementaries(_lib.Z3_solver_inc_ref)):
  _elems.f(a0, a1)
  _elems.Check(a0)

def Z3_solver_dec_ref(a0, a1, _elems=Elementaries(_lib.Z3_solver_dec_ref)):
  _elems.f(a0, a1)

def Z3_solver_interrupt(a0, a1, _elems=Elementaries(_lib.Z3_solver_interrupt)):
  _elems.f(a0, a1)
  _elems.Check(a0)

def Z3_solver_push(a0, a1, _elems=Elementaries(_lib.Z3_solver_push)):
  _elems.f(a0, a1)
  _elems.Check(a0)

def Z3_solver_pop(a0, a1, a2, _elems=Elementaries(_lib.Z3_solver_pop)):
  _elems.f(a0, a1, a2)
  _elems.Check(a0)

def Z3_solver_reset(a0, a1, _elems=Elementaries(_lib.Z3_solver_reset)):
  _elems.f(a0, a1)
  _elems.Check(a0)

def Z3_solver_get_num_scopes(a0, a1, _elems=Elementaries(_lib.Z3_solver_get_num_scopes)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_solver_assert(a0, a1, a2, _elems=Elementaries(_lib.Z3_solver_assert)):
  _elems.f(a0, a1, a2)
  _elems.Check(a0)

def Z3_solver_assert_and_track(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_solver_assert_and_track)):
  _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)

def Z3_solver_from_file(a0, a1, a2, _elems=Elementaries(_lib.Z3_solver_from_file)):
  _elems.f(a0, a1, _str_to_bytes(a2))
  _elems.Check(a0)

def Z3_solver_from_string(a0, a1, a2, _elems=Elementaries(_lib.Z3_solver_from_string)):
  _elems.f(a0, a1, _str_to_bytes(a2))
  _elems.Check(a0)

def Z3_solver_get_assertions(a0, a1, _elems=Elementaries(_lib.Z3_solver_get_assertions)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_solver_get_units(a0, a1, _elems=Elementaries(_lib.Z3_solver_get_units)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_solver_get_trail(a0, a1, _elems=Elementaries(_lib.Z3_solver_get_trail)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_solver_get_non_units(a0, a1, _elems=Elementaries(_lib.Z3_solver_get_non_units)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_solver_get_levels(a0, a1, a2, a3, a4, _elems=Elementaries(_lib.Z3_solver_get_levels)):
  _elems.f(a0, a1, a2, a3, a4)
  _elems.Check(a0)

def Z3_solver_congruence_root(a0, a1, a2, _elems=Elementaries(_lib.Z3_solver_congruence_root)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_solver_congruence_next(a0, a1, a2, _elems=Elementaries(_lib.Z3_solver_congruence_next)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_solver_register_on_clause(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_solver_register_on_clause)):
  _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)

def Z3_solver_propagate_init(a0, a1, a2, a3, a4, a5, _elems=Elementaries(_lib.Z3_solver_propagate_init)):
  _elems.f(a0, a1, a2, a3, a4, a5)
  _elems.Check(a0)

def Z3_solver_propagate_fixed(a0, a1, a2, _elems=Elementaries(_lib.Z3_solver_propagate_fixed)):
  _elems.f(a0, a1, a2)
  _elems.Check(a0)

def Z3_solver_propagate_final(a0, a1, a2, _elems=Elementaries(_lib.Z3_solver_propagate_final)):
  _elems.f(a0, a1, a2)
  _elems.Check(a0)

def Z3_solver_propagate_eq(a0, a1, a2, _elems=Elementaries(_lib.Z3_solver_propagate_eq)):
  _elems.f(a0, a1, a2)
  _elems.Check(a0)

def Z3_solver_propagate_diseq(a0, a1, a2, _elems=Elementaries(_lib.Z3_solver_propagate_diseq)):
  _elems.f(a0, a1, a2)
  _elems.Check(a0)

def Z3_solver_propagate_created(a0, a1, a2, _elems=Elementaries(_lib.Z3_solver_propagate_created)):
  _elems.f(a0, a1, a2)
  _elems.Check(a0)

def Z3_solver_propagate_decide(a0, a1, a2, _elems=Elementaries(_lib.Z3_solver_propagate_decide)):
  _elems.f(a0, a1, a2)
  _elems.Check(a0)

def Z3_solver_next_split(a0, a1, a2, a3, a4, _elems=Elementaries(_lib.Z3_solver_next_split)):
  r = _elems.f(a0, a1, a2, a3, a4)
  _elems.Check(a0)
  return r

def Z3_solver_propagate_declare(a0, a1, a2, a3, a4, _elems=Elementaries(_lib.Z3_solver_propagate_declare)):
  r = _elems.f(a0, a1, a2, a3, a4)
  _elems.Check(a0)
  return r

def Z3_solver_propagate_register(a0, a1, a2, _elems=Elementaries(_lib.Z3_solver_propagate_register)):
  _elems.f(a0, a1, a2)
  _elems.Check(a0)

def Z3_solver_propagate_register_cb(a0, a1, a2, _elems=Elementaries(_lib.Z3_solver_propagate_register_cb)):
  _elems.f(a0, a1, a2)
  _elems.Check(a0)

def Z3_solver_propagate_consequence(a0, a1, a2, a3, a4, a5, a6, a7, _elems=Elementaries(_lib.Z3_solver_propagate_consequence)):
  r = _elems.f(a0, a1, a2, a3, a4, a5, a6, a7)
  _elems.Check(a0)
  return r

def Z3_solver_check(a0, a1, _elems=Elementaries(_lib.Z3_solver_check)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_solver_check_assumptions(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_solver_check_assumptions)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_get_implied_equalities(a0, a1, a2, a3, a4, _elems=Elementaries(_lib.Z3_get_implied_equalities)):
  r = _elems.f(a0, a1, a2, a3, a4)
  _elems.Check(a0)
  return r

def Z3_solver_get_consequences(a0, a1, a2, a3, a4, _elems=Elementaries(_lib.Z3_solver_get_consequences)):
  r = _elems.f(a0, a1, a2, a3, a4)
  _elems.Check(a0)
  return r

def Z3_solver_cube(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_solver_cube)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_solver_get_model(a0, a1, _elems=Elementaries(_lib.Z3_solver_get_model)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_solver_get_proof(a0, a1, _elems=Elementaries(_lib.Z3_solver_get_proof)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_solver_get_unsat_core(a0, a1, _elems=Elementaries(_lib.Z3_solver_get_unsat_core)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_solver_get_reason_unknown(a0, a1, _elems=Elementaries(_lib.Z3_solver_get_reason_unknown)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_solver_get_reason_unknown_bytes(a0, a1, _elems=Elementaries(_lib.Z3_solver_get_reason_unknown)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_solver_get_statistics(a0, a1, _elems=Elementaries(_lib.Z3_solver_get_statistics)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_solver_to_string(a0, a1, _elems=Elementaries(_lib.Z3_solver_to_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_solver_to_string_bytes(a0, a1, _elems=Elementaries(_lib.Z3_solver_to_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_solver_to_dimacs_string(a0, a1, a2, _elems=Elementaries(_lib.Z3_solver_to_dimacs_string)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_solver_to_dimacs_string_bytes(a0, a1, a2, _elems=Elementaries(_lib.Z3_solver_to_dimacs_string)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_stats_to_string(a0, a1, _elems=Elementaries(_lib.Z3_stats_to_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_stats_to_string_bytes(a0, a1, _elems=Elementaries(_lib.Z3_stats_to_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_stats_inc_ref(a0, a1, _elems=Elementaries(_lib.Z3_stats_inc_ref)):
  _elems.f(a0, a1)
  _elems.Check(a0)

def Z3_stats_dec_ref(a0, a1, _elems=Elementaries(_lib.Z3_stats_dec_ref)):
  _elems.f(a0, a1)

def Z3_stats_size(a0, a1, _elems=Elementaries(_lib.Z3_stats_size)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_stats_get_key(a0, a1, a2, _elems=Elementaries(_lib.Z3_stats_get_key)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_stats_get_key_bytes(a0, a1, a2, _elems=Elementaries(_lib.Z3_stats_get_key)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_stats_is_uint(a0, a1, a2, _elems=Elementaries(_lib.Z3_stats_is_uint)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_stats_is_double(a0, a1, a2, _elems=Elementaries(_lib.Z3_stats_is_double)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_stats_get_uint_value(a0, a1, a2, _elems=Elementaries(_lib.Z3_stats_get_uint_value)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_stats_get_double_value(a0, a1, a2, _elems=Elementaries(_lib.Z3_stats_get_double_value)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_get_estimated_alloc_size(_elems=Elementaries(_lib.Z3_get_estimated_alloc_size)):
  r = _elems.f()
  return r

def Z3_mk_ast_vector(a0, _elems=Elementaries(_lib.Z3_mk_ast_vector)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_ast_vector_inc_ref(a0, a1, _elems=Elementaries(_lib.Z3_ast_vector_inc_ref)):
  _elems.f(a0, a1)
  _elems.Check(a0)

def Z3_ast_vector_dec_ref(a0, a1, _elems=Elementaries(_lib.Z3_ast_vector_dec_ref)):
  _elems.f(a0, a1)

def Z3_ast_vector_size(a0, a1, _elems=Elementaries(_lib.Z3_ast_vector_size)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_ast_vector_get(a0, a1, a2, _elems=Elementaries(_lib.Z3_ast_vector_get)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_ast_vector_set(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_ast_vector_set)):
  _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)

def Z3_ast_vector_resize(a0, a1, a2, _elems=Elementaries(_lib.Z3_ast_vector_resize)):
  _elems.f(a0, a1, a2)
  _elems.Check(a0)

def Z3_ast_vector_push(a0, a1, a2, _elems=Elementaries(_lib.Z3_ast_vector_push)):
  _elems.f(a0, a1, a2)
  _elems.Check(a0)

def Z3_ast_vector_translate(a0, a1, a2, _elems=Elementaries(_lib.Z3_ast_vector_translate)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_ast_vector_to_string(a0, a1, _elems=Elementaries(_lib.Z3_ast_vector_to_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_ast_vector_to_string_bytes(a0, a1, _elems=Elementaries(_lib.Z3_ast_vector_to_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_ast_map(a0, _elems=Elementaries(_lib.Z3_mk_ast_map)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_ast_map_inc_ref(a0, a1, _elems=Elementaries(_lib.Z3_ast_map_inc_ref)):
  _elems.f(a0, a1)
  _elems.Check(a0)

def Z3_ast_map_dec_ref(a0, a1, _elems=Elementaries(_lib.Z3_ast_map_dec_ref)):
  _elems.f(a0, a1)

def Z3_ast_map_contains(a0, a1, a2, _elems=Elementaries(_lib.Z3_ast_map_contains)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_ast_map_find(a0, a1, a2, _elems=Elementaries(_lib.Z3_ast_map_find)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_ast_map_insert(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_ast_map_insert)):
  _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)

def Z3_ast_map_erase(a0, a1, a2, _elems=Elementaries(_lib.Z3_ast_map_erase)):
  _elems.f(a0, a1, a2)
  _elems.Check(a0)

def Z3_ast_map_reset(a0, a1, _elems=Elementaries(_lib.Z3_ast_map_reset)):
  _elems.f(a0, a1)
  _elems.Check(a0)

def Z3_ast_map_size(a0, a1, _elems=Elementaries(_lib.Z3_ast_map_size)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_ast_map_keys(a0, a1, _elems=Elementaries(_lib.Z3_ast_map_keys)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_ast_map_to_string(a0, a1, _elems=Elementaries(_lib.Z3_ast_map_to_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_ast_map_to_string_bytes(a0, a1, _elems=Elementaries(_lib.Z3_ast_map_to_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_algebraic_is_value(a0, a1, _elems=Elementaries(_lib.Z3_algebraic_is_value)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_algebraic_is_pos(a0, a1, _elems=Elementaries(_lib.Z3_algebraic_is_pos)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_algebraic_is_neg(a0, a1, _elems=Elementaries(_lib.Z3_algebraic_is_neg)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_algebraic_is_zero(a0, a1, _elems=Elementaries(_lib.Z3_algebraic_is_zero)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_algebraic_sign(a0, a1, _elems=Elementaries(_lib.Z3_algebraic_sign)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_algebraic_add(a0, a1, a2, _elems=Elementaries(_lib.Z3_algebraic_add)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_algebraic_sub(a0, a1, a2, _elems=Elementaries(_lib.Z3_algebraic_sub)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_algebraic_mul(a0, a1, a2, _elems=Elementaries(_lib.Z3_algebraic_mul)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_algebraic_div(a0, a1, a2, _elems=Elementaries(_lib.Z3_algebraic_div)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_algebraic_root(a0, a1, a2, _elems=Elementaries(_lib.Z3_algebraic_root)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_algebraic_power(a0, a1, a2, _elems=Elementaries(_lib.Z3_algebraic_power)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_algebraic_lt(a0, a1, a2, _elems=Elementaries(_lib.Z3_algebraic_lt)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_algebraic_gt(a0, a1, a2, _elems=Elementaries(_lib.Z3_algebraic_gt)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_algebraic_le(a0, a1, a2, _elems=Elementaries(_lib.Z3_algebraic_le)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_algebraic_ge(a0, a1, a2, _elems=Elementaries(_lib.Z3_algebraic_ge)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_algebraic_eq(a0, a1, a2, _elems=Elementaries(_lib.Z3_algebraic_eq)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_algebraic_neq(a0, a1, a2, _elems=Elementaries(_lib.Z3_algebraic_neq)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_algebraic_roots(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_algebraic_roots)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_algebraic_eval(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_algebraic_eval)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_algebraic_get_poly(a0, a1, _elems=Elementaries(_lib.Z3_algebraic_get_poly)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_algebraic_get_i(a0, a1, _elems=Elementaries(_lib.Z3_algebraic_get_i)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_polynomial_subresultants(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_polynomial_subresultants)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_rcf_del(a0, a1, _elems=Elementaries(_lib.Z3_rcf_del)):
  _elems.f(a0, a1)
  _elems.Check(a0)

def Z3_rcf_mk_rational(a0, a1, _elems=Elementaries(_lib.Z3_rcf_mk_rational)):
  r = _elems.f(a0, _str_to_bytes(a1))
  _elems.Check(a0)
  return r

def Z3_rcf_mk_small_int(a0, a1, _elems=Elementaries(_lib.Z3_rcf_mk_small_int)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_rcf_mk_pi(a0, _elems=Elementaries(_lib.Z3_rcf_mk_pi)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_rcf_mk_e(a0, _elems=Elementaries(_lib.Z3_rcf_mk_e)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_rcf_mk_infinitesimal(a0, _elems=Elementaries(_lib.Z3_rcf_mk_infinitesimal)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_rcf_mk_roots(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_rcf_mk_roots)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_rcf_add(a0, a1, a2, _elems=Elementaries(_lib.Z3_rcf_add)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_rcf_sub(a0, a1, a2, _elems=Elementaries(_lib.Z3_rcf_sub)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_rcf_mul(a0, a1, a2, _elems=Elementaries(_lib.Z3_rcf_mul)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_rcf_div(a0, a1, a2, _elems=Elementaries(_lib.Z3_rcf_div)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_rcf_neg(a0, a1, _elems=Elementaries(_lib.Z3_rcf_neg)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_rcf_inv(a0, a1, _elems=Elementaries(_lib.Z3_rcf_inv)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_rcf_power(a0, a1, a2, _elems=Elementaries(_lib.Z3_rcf_power)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_rcf_lt(a0, a1, a2, _elems=Elementaries(_lib.Z3_rcf_lt)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_rcf_gt(a0, a1, a2, _elems=Elementaries(_lib.Z3_rcf_gt)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_rcf_le(a0, a1, a2, _elems=Elementaries(_lib.Z3_rcf_le)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_rcf_ge(a0, a1, a2, _elems=Elementaries(_lib.Z3_rcf_ge)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_rcf_eq(a0, a1, a2, _elems=Elementaries(_lib.Z3_rcf_eq)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_rcf_neq(a0, a1, a2, _elems=Elementaries(_lib.Z3_rcf_neq)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_rcf_num_to_string(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_rcf_num_to_string)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_rcf_num_to_string_bytes(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_rcf_num_to_string)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_rcf_num_to_decimal_string(a0, a1, a2, _elems=Elementaries(_lib.Z3_rcf_num_to_decimal_string)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_rcf_num_to_decimal_string_bytes(a0, a1, a2, _elems=Elementaries(_lib.Z3_rcf_num_to_decimal_string)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_rcf_get_numerator_denominator(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_rcf_get_numerator_denominator)):
  _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)

def Z3_rcf_is_rational(a0, a1, _elems=Elementaries(_lib.Z3_rcf_is_rational)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_rcf_is_algebraic(a0, a1, _elems=Elementaries(_lib.Z3_rcf_is_algebraic)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_rcf_is_infinitesimal(a0, a1, _elems=Elementaries(_lib.Z3_rcf_is_infinitesimal)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_rcf_is_transcendental(a0, a1, _elems=Elementaries(_lib.Z3_rcf_is_transcendental)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_rcf_extension_index(a0, a1, _elems=Elementaries(_lib.Z3_rcf_extension_index)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_rcf_transcendental_name(a0, a1, _elems=Elementaries(_lib.Z3_rcf_transcendental_name)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_rcf_infinitesimal_name(a0, a1, _elems=Elementaries(_lib.Z3_rcf_infinitesimal_name)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_rcf_num_coefficients(a0, a1, _elems=Elementaries(_lib.Z3_rcf_num_coefficients)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_rcf_coefficient(a0, a1, a2, _elems=Elementaries(_lib.Z3_rcf_coefficient)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_rcf_interval(a0, a1, a2, a3, a4, a5, a6, a7, _elems=Elementaries(_lib.Z3_rcf_interval)):
  r = _elems.f(a0, a1, a2, a3, a4, a5, a6, a7)
  _elems.Check(a0)
  return r

def Z3_rcf_num_sign_conditions(a0, a1, _elems=Elementaries(_lib.Z3_rcf_num_sign_conditions)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_rcf_sign_condition_sign(a0, a1, a2, _elems=Elementaries(_lib.Z3_rcf_sign_condition_sign)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_rcf_num_sign_condition_coefficients(a0, a1, a2, _elems=Elementaries(_lib.Z3_rcf_num_sign_condition_coefficients)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_rcf_sign_condition_coefficient(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_rcf_sign_condition_coefficient)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_mk_fixedpoint(a0, _elems=Elementaries(_lib.Z3_mk_fixedpoint)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_fixedpoint_inc_ref(a0, a1, _elems=Elementaries(_lib.Z3_fixedpoint_inc_ref)):
  _elems.f(a0, a1)
  _elems.Check(a0)

def Z3_fixedpoint_dec_ref(a0, a1, _elems=Elementaries(_lib.Z3_fixedpoint_dec_ref)):
  _elems.f(a0, a1)

def Z3_fixedpoint_add_rule(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_fixedpoint_add_rule)):
  _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)

def Z3_fixedpoint_add_fact(a0, a1, a2, a3, a4, _elems=Elementaries(_lib.Z3_fixedpoint_add_fact)):
  _elems.f(a0, a1, a2, a3, a4)
  _elems.Check(a0)

def Z3_fixedpoint_assert(a0, a1, a2, _elems=Elementaries(_lib.Z3_fixedpoint_assert)):
  _elems.f(a0, a1, a2)
  _elems.Check(a0)

def Z3_fixedpoint_query(a0, a1, a2, _elems=Elementaries(_lib.Z3_fixedpoint_query)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_fixedpoint_query_relations(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_fixedpoint_query_relations)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_fixedpoint_get_answer(a0, a1, _elems=Elementaries(_lib.Z3_fixedpoint_get_answer)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_fixedpoint_get_reason_unknown(a0, a1, _elems=Elementaries(_lib.Z3_fixedpoint_get_reason_unknown)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_fixedpoint_get_reason_unknown_bytes(a0, a1, _elems=Elementaries(_lib.Z3_fixedpoint_get_reason_unknown)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_fixedpoint_update_rule(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_fixedpoint_update_rule)):
  _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)

def Z3_fixedpoint_get_num_levels(a0, a1, a2, _elems=Elementaries(_lib.Z3_fixedpoint_get_num_levels)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_fixedpoint_get_cover_delta(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_fixedpoint_get_cover_delta)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_fixedpoint_add_cover(a0, a1, a2, a3, a4, _elems=Elementaries(_lib.Z3_fixedpoint_add_cover)):
  _elems.f(a0, a1, a2, a3, a4)
  _elems.Check(a0)

def Z3_fixedpoint_get_statistics(a0, a1, _elems=Elementaries(_lib.Z3_fixedpoint_get_statistics)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_fixedpoint_register_relation(a0, a1, a2, _elems=Elementaries(_lib.Z3_fixedpoint_register_relation)):
  _elems.f(a0, a1, a2)
  _elems.Check(a0)

def Z3_fixedpoint_set_predicate_representation(a0, a1, a2, a3, a4, _elems=Elementaries(_lib.Z3_fixedpoint_set_predicate_representation)):
  _elems.f(a0, a1, a2, a3, a4)
  _elems.Check(a0)

def Z3_fixedpoint_get_rules(a0, a1, _elems=Elementaries(_lib.Z3_fixedpoint_get_rules)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_fixedpoint_get_assertions(a0, a1, _elems=Elementaries(_lib.Z3_fixedpoint_get_assertions)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_fixedpoint_set_params(a0, a1, a2, _elems=Elementaries(_lib.Z3_fixedpoint_set_params)):
  _elems.f(a0, a1, a2)
  _elems.Check(a0)

def Z3_fixedpoint_get_help(a0, a1, _elems=Elementaries(_lib.Z3_fixedpoint_get_help)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_fixedpoint_get_help_bytes(a0, a1, _elems=Elementaries(_lib.Z3_fixedpoint_get_help)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_fixedpoint_get_param_descrs(a0, a1, _elems=Elementaries(_lib.Z3_fixedpoint_get_param_descrs)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_fixedpoint_to_string(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_fixedpoint_to_string)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_fixedpoint_to_string_bytes(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_fixedpoint_to_string)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_fixedpoint_from_string(a0, a1, a2, _elems=Elementaries(_lib.Z3_fixedpoint_from_string)):
  r = _elems.f(a0, a1, _str_to_bytes(a2))
  _elems.Check(a0)
  return r

def Z3_fixedpoint_from_file(a0, a1, a2, _elems=Elementaries(_lib.Z3_fixedpoint_from_file)):
  r = _elems.f(a0, a1, _str_to_bytes(a2))
  _elems.Check(a0)
  return r

def Z3_mk_optimize(a0, _elems=Elementaries(_lib.Z3_mk_optimize)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_optimize_inc_ref(a0, a1, _elems=Elementaries(_lib.Z3_optimize_inc_ref)):
  _elems.f(a0, a1)
  _elems.Check(a0)

def Z3_optimize_dec_ref(a0, a1, _elems=Elementaries(_lib.Z3_optimize_dec_ref)):
  _elems.f(a0, a1)

def Z3_optimize_assert(a0, a1, a2, _elems=Elementaries(_lib.Z3_optimize_assert)):
  _elems.f(a0, a1, a2)
  _elems.Check(a0)

def Z3_optimize_assert_and_track(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_optimize_assert_and_track)):
  _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)

def Z3_optimize_assert_soft(a0, a1, a2, a3, a4, _elems=Elementaries(_lib.Z3_optimize_assert_soft)):
  r = _elems.f(a0, a1, a2, _str_to_bytes(a3), a4)
  _elems.Check(a0)
  return r

def Z3_optimize_maximize(a0, a1, a2, _elems=Elementaries(_lib.Z3_optimize_maximize)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_optimize_minimize(a0, a1, a2, _elems=Elementaries(_lib.Z3_optimize_minimize)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_optimize_push(a0, a1, _elems=Elementaries(_lib.Z3_optimize_push)):
  _elems.f(a0, a1)
  _elems.Check(a0)

def Z3_optimize_pop(a0, a1, _elems=Elementaries(_lib.Z3_optimize_pop)):
  _elems.f(a0, a1)
  _elems.Check(a0)

def Z3_optimize_check(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_optimize_check)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_optimize_get_reason_unknown(a0, a1, _elems=Elementaries(_lib.Z3_optimize_get_reason_unknown)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_optimize_get_reason_unknown_bytes(a0, a1, _elems=Elementaries(_lib.Z3_optimize_get_reason_unknown)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_optimize_get_model(a0, a1, _elems=Elementaries(_lib.Z3_optimize_get_model)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_optimize_get_unsat_core(a0, a1, _elems=Elementaries(_lib.Z3_optimize_get_unsat_core)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_optimize_set_params(a0, a1, a2, _elems=Elementaries(_lib.Z3_optimize_set_params)):
  _elems.f(a0, a1, a2)
  _elems.Check(a0)

def Z3_optimize_get_param_descrs(a0, a1, _elems=Elementaries(_lib.Z3_optimize_get_param_descrs)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_optimize_get_lower(a0, a1, a2, _elems=Elementaries(_lib.Z3_optimize_get_lower)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_optimize_get_upper(a0, a1, a2, _elems=Elementaries(_lib.Z3_optimize_get_upper)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_optimize_get_lower_as_vector(a0, a1, a2, _elems=Elementaries(_lib.Z3_optimize_get_lower_as_vector)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_optimize_get_upper_as_vector(a0, a1, a2, _elems=Elementaries(_lib.Z3_optimize_get_upper_as_vector)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_optimize_to_string(a0, a1, _elems=Elementaries(_lib.Z3_optimize_to_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_optimize_to_string_bytes(a0, a1, _elems=Elementaries(_lib.Z3_optimize_to_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_optimize_from_string(a0, a1, a2, _elems=Elementaries(_lib.Z3_optimize_from_string)):
  _elems.f(a0, a1, _str_to_bytes(a2))
  _elems.Check(a0)

def Z3_optimize_from_file(a0, a1, a2, _elems=Elementaries(_lib.Z3_optimize_from_file)):
  _elems.f(a0, a1, _str_to_bytes(a2))
  _elems.Check(a0)

def Z3_optimize_get_help(a0, a1, _elems=Elementaries(_lib.Z3_optimize_get_help)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_optimize_get_help_bytes(a0, a1, _elems=Elementaries(_lib.Z3_optimize_get_help)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_optimize_get_statistics(a0, a1, _elems=Elementaries(_lib.Z3_optimize_get_statistics)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_optimize_get_assertions(a0, a1, _elems=Elementaries(_lib.Z3_optimize_get_assertions)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_optimize_get_objectives(a0, a1, _elems=Elementaries(_lib.Z3_optimize_get_objectives)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_rounding_mode_sort(a0, _elems=Elementaries(_lib.Z3_mk_fpa_rounding_mode_sort)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_round_nearest_ties_to_even(a0, _elems=Elementaries(_lib.Z3_mk_fpa_round_nearest_ties_to_even)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_rne(a0, _elems=Elementaries(_lib.Z3_mk_fpa_rne)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_round_nearest_ties_to_away(a0, _elems=Elementaries(_lib.Z3_mk_fpa_round_nearest_ties_to_away)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_rna(a0, _elems=Elementaries(_lib.Z3_mk_fpa_rna)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_round_toward_positive(a0, _elems=Elementaries(_lib.Z3_mk_fpa_round_toward_positive)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_rtp(a0, _elems=Elementaries(_lib.Z3_mk_fpa_rtp)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_round_toward_negative(a0, _elems=Elementaries(_lib.Z3_mk_fpa_round_toward_negative)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_rtn(a0, _elems=Elementaries(_lib.Z3_mk_fpa_rtn)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_round_toward_zero(a0, _elems=Elementaries(_lib.Z3_mk_fpa_round_toward_zero)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_rtz(a0, _elems=Elementaries(_lib.Z3_mk_fpa_rtz)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_sort(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_fpa_sort)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_sort_half(a0, _elems=Elementaries(_lib.Z3_mk_fpa_sort_half)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_sort_16(a0, _elems=Elementaries(_lib.Z3_mk_fpa_sort_16)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_sort_single(a0, _elems=Elementaries(_lib.Z3_mk_fpa_sort_single)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_sort_32(a0, _elems=Elementaries(_lib.Z3_mk_fpa_sort_32)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_sort_double(a0, _elems=Elementaries(_lib.Z3_mk_fpa_sort_double)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_sort_64(a0, _elems=Elementaries(_lib.Z3_mk_fpa_sort_64)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_sort_quadruple(a0, _elems=Elementaries(_lib.Z3_mk_fpa_sort_quadruple)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_sort_128(a0, _elems=Elementaries(_lib.Z3_mk_fpa_sort_128)):
  r = _elems.f(a0)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_nan(a0, a1, _elems=Elementaries(_lib.Z3_mk_fpa_nan)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_inf(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_fpa_inf)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_zero(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_fpa_zero)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_fp(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_mk_fpa_fp)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_numeral_float(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_fpa_numeral_float)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_numeral_double(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_fpa_numeral_double)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_numeral_int(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_fpa_numeral_int)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_numeral_int_uint(a0, a1, a2, a3, a4, _elems=Elementaries(_lib.Z3_mk_fpa_numeral_int_uint)):
  r = _elems.f(a0, a1, a2, a3, a4)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_numeral_int64_uint64(a0, a1, a2, a3, a4, _elems=Elementaries(_lib.Z3_mk_fpa_numeral_int64_uint64)):
  r = _elems.f(a0, a1, a2, a3, a4)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_abs(a0, a1, _elems=Elementaries(_lib.Z3_mk_fpa_abs)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_neg(a0, a1, _elems=Elementaries(_lib.Z3_mk_fpa_neg)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_add(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_mk_fpa_add)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_sub(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_mk_fpa_sub)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_mul(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_mk_fpa_mul)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_div(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_mk_fpa_div)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_fma(a0, a1, a2, a3, a4, _elems=Elementaries(_lib.Z3_mk_fpa_fma)):
  r = _elems.f(a0, a1, a2, a3, a4)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_sqrt(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_fpa_sqrt)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_rem(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_fpa_rem)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_round_to_integral(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_fpa_round_to_integral)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_min(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_fpa_min)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_max(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_fpa_max)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_leq(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_fpa_leq)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_lt(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_fpa_lt)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_geq(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_fpa_geq)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_gt(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_fpa_gt)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_eq(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_fpa_eq)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_is_normal(a0, a1, _elems=Elementaries(_lib.Z3_mk_fpa_is_normal)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_is_subnormal(a0, a1, _elems=Elementaries(_lib.Z3_mk_fpa_is_subnormal)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_is_zero(a0, a1, _elems=Elementaries(_lib.Z3_mk_fpa_is_zero)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_is_infinite(a0, a1, _elems=Elementaries(_lib.Z3_mk_fpa_is_infinite)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_is_nan(a0, a1, _elems=Elementaries(_lib.Z3_mk_fpa_is_nan)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_is_negative(a0, a1, _elems=Elementaries(_lib.Z3_mk_fpa_is_negative)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_is_positive(a0, a1, _elems=Elementaries(_lib.Z3_mk_fpa_is_positive)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_to_fp_bv(a0, a1, a2, _elems=Elementaries(_lib.Z3_mk_fpa_to_fp_bv)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_to_fp_float(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_mk_fpa_to_fp_float)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_to_fp_real(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_mk_fpa_to_fp_real)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_to_fp_signed(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_mk_fpa_to_fp_signed)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_to_fp_unsigned(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_mk_fpa_to_fp_unsigned)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_to_ubv(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_mk_fpa_to_ubv)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_to_sbv(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_mk_fpa_to_sbv)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_to_real(a0, a1, _elems=Elementaries(_lib.Z3_mk_fpa_to_real)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_fpa_get_ebits(a0, a1, _elems=Elementaries(_lib.Z3_fpa_get_ebits)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_fpa_get_sbits(a0, a1, _elems=Elementaries(_lib.Z3_fpa_get_sbits)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_fpa_is_numeral_nan(a0, a1, _elems=Elementaries(_lib.Z3_fpa_is_numeral_nan)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_fpa_is_numeral_inf(a0, a1, _elems=Elementaries(_lib.Z3_fpa_is_numeral_inf)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_fpa_is_numeral_zero(a0, a1, _elems=Elementaries(_lib.Z3_fpa_is_numeral_zero)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_fpa_is_numeral_normal(a0, a1, _elems=Elementaries(_lib.Z3_fpa_is_numeral_normal)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_fpa_is_numeral_subnormal(a0, a1, _elems=Elementaries(_lib.Z3_fpa_is_numeral_subnormal)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_fpa_is_numeral_positive(a0, a1, _elems=Elementaries(_lib.Z3_fpa_is_numeral_positive)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_fpa_is_numeral_negative(a0, a1, _elems=Elementaries(_lib.Z3_fpa_is_numeral_negative)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_fpa_get_numeral_sign_bv(a0, a1, _elems=Elementaries(_lib.Z3_fpa_get_numeral_sign_bv)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_fpa_get_numeral_significand_bv(a0, a1, _elems=Elementaries(_lib.Z3_fpa_get_numeral_significand_bv)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_fpa_get_numeral_sign(a0, a1, a2, _elems=Elementaries(_lib.Z3_fpa_get_numeral_sign)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_fpa_get_numeral_significand_string(a0, a1, _elems=Elementaries(_lib.Z3_fpa_get_numeral_significand_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_fpa_get_numeral_significand_string_bytes(a0, a1, _elems=Elementaries(_lib.Z3_fpa_get_numeral_significand_string)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_fpa_get_numeral_significand_uint64(a0, a1, a2, _elems=Elementaries(_lib.Z3_fpa_get_numeral_significand_uint64)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_fpa_get_numeral_exponent_string(a0, a1, a2, _elems=Elementaries(_lib.Z3_fpa_get_numeral_exponent_string)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return _to_pystr(r)

def Z3_fpa_get_numeral_exponent_string_bytes(a0, a1, a2, _elems=Elementaries(_lib.Z3_fpa_get_numeral_exponent_string)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_fpa_get_numeral_exponent_int64(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_fpa_get_numeral_exponent_int64)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_fpa_get_numeral_exponent_bv(a0, a1, a2, _elems=Elementaries(_lib.Z3_fpa_get_numeral_exponent_bv)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_to_ieee_bv(a0, a1, _elems=Elementaries(_lib.Z3_mk_fpa_to_ieee_bv)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_mk_fpa_to_fp_int_real(a0, a1, a2, a3, a4, _elems=Elementaries(_lib.Z3_mk_fpa_to_fp_int_real)):
  r = _elems.f(a0, a1, a2, a3, a4)
  _elems.Check(a0)
  return r

def Z3_fixedpoint_query_from_lvl(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_fixedpoint_query_from_lvl)):
  r = _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)
  return r

def Z3_fixedpoint_get_ground_sat_answer(a0, a1, _elems=Elementaries(_lib.Z3_fixedpoint_get_ground_sat_answer)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_fixedpoint_get_rules_along_trace(a0, a1, _elems=Elementaries(_lib.Z3_fixedpoint_get_rules_along_trace)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_fixedpoint_get_rule_names_along_trace(a0, a1, _elems=Elementaries(_lib.Z3_fixedpoint_get_rule_names_along_trace)):
  r = _elems.f(a0, a1)
  _elems.Check(a0)
  return r

def Z3_fixedpoint_add_invariant(a0, a1, a2, a3, _elems=Elementaries(_lib.Z3_fixedpoint_add_invariant)):
  _elems.f(a0, a1, a2, a3)
  _elems.Check(a0)

def Z3_fixedpoint_get_reachable(a0, a1, a2, _elems=Elementaries(_lib.Z3_fixedpoint_get_reachable)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_qe_model_project(a0, a1, a2, a3, a4, _elems=Elementaries(_lib.Z3_qe_model_project)):
  r = _elems.f(a0, a1, a2, a3, a4)
  _elems.Check(a0)
  return r

def Z3_qe_model_project_skolem(a0, a1, a2, a3, a4, a5, _elems=Elementaries(_lib.Z3_qe_model_project_skolem)):
  r = _elems.f(a0, a1, a2, a3, a4, a5)
  _elems.Check(a0)
  return r

def Z3_model_extrapolate(a0, a1, a2, _elems=Elementaries(_lib.Z3_model_extrapolate)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r

def Z3_qe_lite(a0, a1, a2, _elems=Elementaries(_lib.Z3_qe_lite)):
  r = _elems.f(a0, a1, a2)
  _elems.Check(a0)
  return r


# Clean up
del _lib
del _default_dirs
del _all_dirs
del _ext
