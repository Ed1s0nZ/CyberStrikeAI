import logging
import re

from .arch import Arch, Endness, register_arch

log = logging.getLogger("archinfo.arch_soot")


class SootMethodDescriptor:
    __slots__ = ["class_name", "name", "params", "_soot_method", "ret"]

    def __init__(self, class_name, name, params, soot_method=None, ret_type=None):
        self.class_name = class_name
        self.name = name
        self.params = params
        self._soot_method = soot_method
        self.ret = ret_type

    def __repr__(self):
        return "{}.{}({})".format(self.class_name, self.name, ", ".join(self.params))

    def __hash__(self):
        return hash((self.class_name, self.name, self.params))

    def __eq__(self, other):
        return (
            isinstance(other, SootMethodDescriptor)
            and self.class_name == other.class_name
            and self.name == other.name
            and self.params == other.params
        )

    def __ne__(self, other):
        return not self == other

    def __lt__(self, other):
        return self.__repr__() < other.__repr__()

    def __gt__(self, other):
        return self.__repr__() > other.__repr__()

    def __le__(self, other):
        return self.__repr__() <= other.__repr__()

    def __ge__(self, other):
        return self.__repr__() >= other.__repr__()

    def address(self, block_idx=0, stmt_idx=0):
        """
        :return Address of the method.
        :rtype: SootAddressDescriptor
        """
        return SootAddressDescriptor(self, block_idx, stmt_idx)

    @property
    def fullname(self):
        """
        :return the full name of the method (class name + method name)
        """
        return f"{self.class_name}.{self.name}"

    @property
    def symbolic(self):
        return False

    @property
    def is_loaded(self):
        """
        :return: True, if the method is loaded in CLE and thus infos about attrs,
                 ret and exceptions are available.
        """
        return self._soot_method is not None

    @property
    def attrs(self):
        return self._soot_method.attrs if self.is_loaded else []

    @property
    def exceptions(self):
        return self._soot_method.exceptions if self.is_loaded else []

    @property
    def block_by_label(self):
        return self._soot_method.block_by_label if self.is_loaded else None

    # @property
    # def ret(self):
    #     return self._soot_method.ret if self.is_loaded else []

    @property
    def addr(self):
        """
        :return: the soot address description of the entry point of the method
        """
        return SootAddressDescriptor(self, 0, 0)

    def matches_with_native_name(self, native_method):
        """
        The name of native methods are getting encoded, s.t. they translate into
        valid C function names. This method indicates if the name of the given
        native method matches the name of the soot method.

        :return: True, if name of soot method matches the mangled native name.
        """

        if "__" in native_method:
            # if native methods are overloaded, two underscores are used
            native_method, params_sig = native_method.split("__")
            params = ArchSoot.decode_parameter_list_signature(params_sig)
            # check function signature
            if params != self.params:
                return False

        # demangle native name
        native_method = native_method.replace("_1", "_")
        # TODO unicode escaping

        method_native_name = "Java_{class_name}_{method_name}".format(
            class_name=self.class_name.replace(".", "_"), method_name=self.name
        )

        return native_method == method_native_name

    @classmethod
    def from_string(cls, tstr):
        # this should be the opposite of repr
        tstr = tstr.strip()
        class_and_method, tparams = tstr.split("(")
        params_str = tparams.split(")")[0]
        if params_str == "":
            params = ()
        else:
            params = tuple(t.strip() for t in params_str.split(","))
        class_name, _, method = class_and_method.rpartition(".")
        return cls(class_name, method, params)

    @classmethod
    def from_soot_method(cls, soot_method):
        return cls(
            class_name=soot_method.class_name,
            name=soot_method.name,
            params=soot_method.params,
            soot_method=soot_method,
            ret_type=soot_method.ret,
        )


class SootAddressDescriptor:
    __slots__ = ["method", "block_idx", "stmt_idx"]

    def __init__(self, method, block_idx, stmt_idx):
        if not isinstance(method, SootMethodDescriptor):
            raise ValueError('The parameter "method" must be an ' "instance of SootMethodDescriptor.")

        self.method = method
        self.block_idx = block_idx
        self.stmt_idx = stmt_idx

    def __repr__(self):
        return "<{!r}+({}:{})>".format(
            self.method, self.block_idx, "%d" % self.stmt_idx if self.stmt_idx is not None else "[0]"
        )

    def __hash__(self):
        return hash((self.method, self.stmt_idx))

    def __eq__(self, other):
        return (
            isinstance(other, SootAddressDescriptor)
            and self.method == other.method
            and self.block_idx == other.block_idx
            and self.stmt_idx == other.stmt_idx
        )

    def __ne__(self, other):
        return not self == other

    def __lt__(self, other):
        return self.__repr__() < other.__repr__()

    def __gt__(self, other):
        return self.__repr__() > other.__repr__()

    def __le__(self, other):
        return self.__repr__() <= other.__repr__()

    def __ge__(self, other):
        return self.__repr__() >= other.__repr__()

    def __add__(self, stmts_offset):
        if not isinstance(stmts_offset, int):
            raise TypeError("The stmts_offset must be an int or a long.")
        s = self.copy()
        s.stmt_idx += stmts_offset
        return s

    def copy(self):
        return SootAddressDescriptor(method=self.method, block_idx=self.block_idx, stmt_idx=self.stmt_idx)

    @property
    def symbolic(self):
        return False


class SootAddressTerminator(SootAddressDescriptor):
    __slots__ = []

    def __init__(self):
        dummy_method = SootMethodDescriptor("dummy", "dummy", tuple())
        super().__init__(dummy_method, 0, 0)

    def __repr__(self):
        return "<Terminator>"


class SootFieldDescriptor:
    __slots__ = ["class_name", "name", "type"]

    def __init__(self, class_name, name, type_):
        self.class_name = class_name
        self.name = name
        self.type = type_

    def __repr__(self):
        return f"{self.class_name}.{self.name}"

    def __hash__(self):
        return hash((self.class_name, self.name, self.type))

    def __eq__(self, other):
        return (
            isinstance(other, SootFieldDescriptor)
            and self.class_name == other.class_name
            and self.name == other.name
            and self.type == other.type
        )

    def __ne__(self, other):
        return not self == other


class SootClassDescriptor:
    __slots__ = ["name", "_soot_class"]

    def __init__(self, name, soot_class=None):
        self.name = name
        self._soot_class = soot_class

    def __repr__(self):
        return self.name

    def __hash__(self):
        return hash(self.name)

    def __eq__(self, other):
        return isinstance(other, SootClassDescriptor) and self.name == other.name

    def __ne__(self, other):
        return not self == other

    @property
    def is_loaded(self):
        """
        :return: True, if the class is loaded in CLE and thus info about field,
                 methods, ... are available.
        """
        return self._soot_class is not None

    @property
    def fields(self):
        return self._soot_class.fields if self.is_loaded else None

    @property
    def methods(self):
        return self._soot_class.methods if self.is_loaded else None

    @property
    def superclass_name(self):
        return self._soot_class.super_class if self.is_loaded else None

    @property
    def type(self):
        return "java.lang.Class"


class SootNullConstant:
    def __init__(self):
        pass

    def __repr__(self):
        return "null"

    def __hash__(self):
        return hash("null")

    def __eq__(self, other):
        return isinstance(other, SootNullConstant)

    def __ne__(self, other):
        return not self == other


class SootArgument:
    """
    Typed Java argument.
    """

    __slots__ = ["value", "type", "is_this_ref"]

    def __init__(self, value, type_, is_this_ref=False):
        """
        :param value:        Value of the argument
        :param type_:        Type of the argument
        :param is_this_ref:  Indicates whether the argument represents the
                             'this' reference, i.e. the object on which an
                             instance method is invoked.
        """
        self.value = value
        self.type = type_
        self.is_this_ref = is_this_ref

    def __repr__(self):
        return f"{self.value} ({self.type})"


class ArchSoot(Arch):
    def __init__(self, endness=Endness.LE):
        super().__init__(endness)

    name = "Soot"

    vex_arch = None  # No VEX support
    qemu_name = None  # No Qemu/Unicorn-engine support
    bits = 64
    address_types = (SootAddressDescriptor,)
    function_address_types = (SootMethodDescriptor,)

    # Size of native counterparts of primitive Java types
    sizeof = {"boolean": 8, "byte": 8, "char": 16, "short": 16, "int": 32, "long": 64, "float": 32, "double": 64}

    primitive_types = ["boolean", "byte", "char", "short", "int", "long", "float", "double"]

    sig_dict = {
        "Z": "boolean",
        "B": "byte",
        "C": "char",
        "S": "short",
        "I": "int",
        "J": "long",
        "F": "float",
        "D": "double",
        "V": "void",
    }

    @staticmethod
    def decode_type_signature(type_sig):
        if not type_sig:
            return None
        # try to translate signature as a primitive type
        if type_sig in ArchSoot.sig_dict:
            return ArchSoot.sig_dict[type_sig]
        # java classes are encoded as 'Lclass_name;'
        if type_sig.startswith("L") and type_sig.endswith(";"):
            return type_sig[1:-1]
        raise ValueError(type_sig)

    @staticmethod
    def decode_parameter_list_signature(param_sig):
        return tuple(
            ArchSoot.decode_type_signature(param) for param in re.findall(r"([\[]*[ZBCSIJFDV]|[\[]*L.+;)", param_sig)
        )

    @staticmethod
    def decode_method_signature(method_sig):
        # signature format follows the pattern: (param_sig)ret_sig
        match = re.search(r"\((.*)\)(.*)", method_sig)
        param_sig, ret_sig = match.group(1), match.group(2)
        # decode types
        params_types = ArchSoot.decode_parameter_list_signature(param_sig)
        ret_type = ArchSoot.decode_type_signature(ret_sig)
        log.debug("Decoded method signature '%s' as params=%s and ret=%s", method_sig, params_types, ret_type)
        return params_types, ret_type

    def library_search_path(self, pedantic=False):
        """
        Since Java is mostly system independent, we cannot return system
        specific paths.

        :return: empty list
        """
        return []


register_arch(["soot"], 8, Endness.LE, ArchSoot)
