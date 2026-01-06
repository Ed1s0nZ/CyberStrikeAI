from __future__ import annotations

import itertools
import logging
import math
import pickle
import struct
from collections import deque
from contextlib import suppress
from enum import IntEnum
from hashlib import blake2b
from itertools import chain
from typing import TYPE_CHECKING, NoReturn, TypeVar, Union, cast
from weakref import WeakValueDictionary

from typing_extensions import Self

import claripy
from claripy import operations
from claripy.errors import BackendError, ClaripyOperationError
from claripy.fp import FSort

if TYPE_CHECKING:
    from collections.abc import Iterable, Iterator

    from claripy.annotation import Annotation
    from claripy.backends import Backend

log = logging.getLogger(__name__)

blake2b_unpacker = struct.Struct("Q")

# pylint:disable=unused-argument,too-many-boolean-expressions,too-many-positional-arguments

ArgType = Union["Base", bool, int, float, str, FSort, tuple["ArgType"], None]

T = TypeVar("T", bound="Base")
A = TypeVar("A", bound="Annotation")


class ReprLevel(IntEnum):
    """
    Representation levels for ASTs.
    """

    LITE_REPR = 0
    MID_REPR = 1
    FULL_REPR = 2


#
# AST variable naming
#

var_counter = itertools.count()


def _make_name(name: str, size: int, explicit_name: bool = False, prefix: str = "") -> str:
    if not explicit_name:
        return f"{prefix}{name}_{next(var_counter)}_{size}"
    return name


def _d(h, cls, state):
    """
    This function is the deserializer for ASTs.
    It exists to work around the fact that pickle will (normally) call __new__() with no arguments during
    deserialization. For ASTs, this does not work.
    """
    op, args, length, variables, symbolic, annotations = state
    return cls.__new__(
        cls, op, args, length=length, variables=variables, symbolic=symbolic, annotations=annotations, hash=h
    )


class Base:
    """
    This is the base class of all claripy ASTs. An AST tracks a tree of operations on arguments.

    This class should not be instanciated directly - instead, use one of the constructor functions (BVS, BVV, FPS,
    FPV...) to construct a leaf node and then build more complicated expressions using operations.

    AST objects have *hash identity*. This means that an AST that has the same hash as another AST will be the *same*
    object. This is critical for efficient memory usage. As an example, the following is true::

        a, b = two different ASTs
        c = b + a
        d = b + a
        assert c is d

    :ivar op:                       The operation that is being done on the arguments
    :ivar args:                     The arguments that are being used
    """

    # Hashcons
    op: str
    args: tuple[ArgType, ...]
    length: int | None
    variables: frozenset[str]
    symbolic: bool
    annotations: tuple[Annotation, ...]

    # Derived information
    depth: int
    _hash: int
    _relocatable_annotations: frozenset[Annotation]
    _uneliminatable_annotations: frozenset[Annotation]

    # Backend information
    _errored: set[Backend]

    # Caching
    _cached_encoded_name: bytes | None

    __slots__ = [
        "__weakref__",
        "_cached_encoded_name",
        "_errored",
        "_hash",
        "_relocatable_annotations",
        "_uneliminatable_annotations",
        "annotations",
        "args",
        "depth",
        "length",
        "op",
        "symbolic",
        "variables",
    ]

    _hash_cache: WeakValueDictionary[int, Base] = WeakValueDictionary()

    def __new__(  # pylint:disable=redefined-builtin
        cls: type[Base],
        op: str,
        args: Iterable[ArgType],
        add_variables: Iterable[str] | None = None,
        hash: int | None = None,
        symbolic: bool | None = None,
        variables: frozenset[str] | None = None,
        errored: set[Backend] | None = None,
        annotations: tuple[Annotation, ...] = (),
        skip_child_annotations: bool = False,
        length: int | None = None,
        encoded_name: bytes | None = None,
    ):
        """
        This is called when you create a new Base object, whether directly or through an operation.
        It finalizes the arguments (see the _finalize function, above) and then computes
        a hash. If an AST of this hash already exists, it returns that AST. Otherwise,
        it creates, initializes, and returns the AST.

        :param op:                  The AST operation ('__add__', 'Or', etc)
        :param args:                The arguments to the AST operation (i.e., the objects to add)
        :param variables:           The symbolic variables present in the AST (default: empty set)
        :param symbolic:            A flag saying whether or not the AST is symbolic (default: False)
        :param length:              An integer specifying the bit length of this AST (default: None)
        :param simplified:          A measure of how simplified this AST is. 0 means unsimplified,
                                        1 means fast-simplified (basically, just undoing the Reverse
                                        op), and 2 means simplified through z3.
        :param errored:             A set of backends that are known to be unable to handle this AST.
        :param annotations:         A frozenset of annotations applied onto this AST.
        """

        if hash is not None and (self := cls._hash_cache.get(hash, None)) is not None:
            return self

        a_args = args if type(args) is tuple else tuple(args)
        b_args = tuple(a for a in a_args if isinstance(a, Base))

        if symbolic is None:
            symbolic = any(a.symbolic for a in b_args)
        if variables is None:
            variables = frozenset(chain.from_iterable(a.variables for a in b_args))
        if errored is None:
            errored = set(chain.from_iterable(a._errored for a in b_args))
        arg_max_depth = max((a.depth for a in b_args), default=0)

        if add_variables:
            variables = frozenset.union(variables, add_variables)

        if not symbolic and op not in operations.leaf_operations:
            with suppress(BackendError):
                r = operations._handle_annotations(
                    claripy.backends.concrete._abstract(claripy.backends.concrete.call(op, args)), args
                )
                if r is not None:
                    return r

        uneliminatable_annotations = frozenset(a for a in annotations if not (a.eliminatable or a.relocatable))
        relocatable_annotations = frozenset(a for a in annotations if not a.eliminatable and a.relocatable)

        if not skip_child_annotations:
            for a in b_args:
                uneliminatable_annotations |= a._uneliminatable_annotations
                relocatable_annotations |= a._relocatable_annotations

            annotations = tuple(frozenset((*annotations, *relocatable_annotations)))

        hash_ = Base._calc_hash(op, a_args, annotations, length)
        self = cls._hash_cache.get(hash_, None)
        if self is None:
            self = super().__new__(cls)
            depth = arg_max_depth + 1
            self.__a_init__(
                op,
                a_args,
                variables,
                symbolic=symbolic,
                length=length,
                errored=errored,
                annotations=annotations,
                encoded_name=encoded_name,
                depth=depth,
                uneliminatable_annotations=uneliminatable_annotations,
                relocatable_annotations=relocatable_annotations,
            )
            self._hash = hash_
            cls._hash_cache[hash_] = self
        # else:
        #     if not self._check_args_same(a_args) or self.op != op or self.annotations != annotations:
        #         raise Exception("CRAP -- hash collision")

        return self

    def make_like(
        self,
        op: str,
        args: Iterable[ArgType],
        simplify: bool = False,
        annotations: tuple[Annotation, ...] | None = None,
        variables: frozenset[str] | None = None,
        symbolic: bool | None = None,
        skip_child_annotations: bool = False,
        length: int | None = None,
    ) -> Self:
        # Try to simplify the expression again
        simplified, annotated = claripy.simplifications.simplify(op, args) if simplify else (None, False)
        if simplified is not None:
            op = simplified.op
        if (  # fast path
            simplified is None
            and annotations
            and variables is None
            and symbolic is None
            and skip_child_annotations
            and length is not None
        ):
            uneliminatable_annotations = frozenset(
                anno for anno in annotations if not anno.eliminatable and not anno.relocatable
            )
            relocatable_annotations = frozenset(
                anno for anno in annotations if not anno.eliminatable and anno.relocatable
            )

            cache = type(self)._hash_cache
            h = Base._calc_hash(op, args, annotations, length)
            cached_ast = cast("T | None", cache.get(h, None))
            if cached_ast is not None:
                return cached_ast

            result: T = super().__new__(type(self))
            result.__a_init__(
                op,
                args,
                self.variables,
                depth=self.depth,
                uneliminatable_annotations=uneliminatable_annotations,
                relocatable_annotations=relocatable_annotations,
                symbolic=self.symbolic,
                annotations=annotations,
                length=length,
            )

            result._hash = h
            cache[h] = result

            return result

        all_operations = operations.leaf_operations_symbolic_with_union
        # special case: if self is one of the args, we do not copy annotations over from self since child
        # annotations will be re-processed during AST creation.
        if annotations is None:
            if not annotated:
                annotations = self.annotations if not args or not any(self is arg for arg in args) else ()
            else:
                annotations = simplified.annotations
        if variables is None and op in all_operations:
            variables = self.variables
        if symbolic is None and op in all_operations:
            symbolic = self.symbolic

        return type(self)(
            op,
            args if simplified is None else simplified.args,
            annotations=annotations,
            variables=variables,
            symbolic=symbolic,
            skip_child_annotations=skip_child_annotations,
            length=length,
        )

    # pylint:disable=attribute-defined-outside-init
    def __a_init__(
        self: Self,
        op: str,
        args: tuple[ArgType, ...],
        variables: frozenset[str],
        symbolic: bool | None = None,
        length: int | None = None,
        errored: set[Backend] | None = None,
        annotations: tuple[Annotation, ...] | None = None,
        encoded_name: bytes | None = None,
        depth: int | None = None,
        uneliminatable_annotations: frozenset[Annotation] | None = None,
        relocatable_annotations: frozenset[Annotation] | None = None,
    ) -> None:
        """
        Initializes an AST. Takes the same arguments as ``Base.__new__()``

        We use this instead of ``__init__`` due to python's undesirable behavior w.r.t. automatically calling it on
        return from ``__new__``.
        """

        # HASHCONS: these attributes key the cache
        # BEFORE CHANGING THIS, SEE ALL OTHER INSTANCES OF "HASHCONS" IN THIS FILE
        self.op = op
        self.args = args if type(args) is tuple else tuple(args)
        self.length = length
        self.variables = variables
        self.symbolic = symbolic
        self.annotations: tuple[Annotation] = annotations
        self._uneliminatable_annotations = uneliminatable_annotations
        self._relocatable_annotations = relocatable_annotations

        self.depth = depth if depth is not None else 1

        self._cached_encoded_name = encoded_name

        self._errored = errored if errored is not None else set()

        if len(self.args) == 0:
            raise ClaripyOperationError("AST with no arguments!")

    # pylint:enable=attribute-defined-outside-init
    def __reduce__(self):
        # HASHCONS: these attributes key the cache
        # BEFORE CHANGING THIS, SEE ALL OTHER INSTANCES OF "HASHCONS" IN THIS FILE
        return _d, (
            self._hash,
            self.__class__,
            (self.op, self.args, self.length, self.variables, self.symbolic, self.annotations),
        )

    def __init__(self, *args, **kwargs):
        pass

    @staticmethod
    def _calc_hash(
        op: str,
        args: tuple[Base, ...],
        annotations: tuple[Annotation, ...],
        length: int | None,
    ) -> int:
        """
        Calculates the hash of an AST, given the operation, args, and kwargs.

        :param op:                  The operation.
        :param args:                The args tuple.
        :param annotations:         The annotations tuple.
        :returns:                   a hash, as bytes.

        We do it using md5 to avoid hash collisions.
        (hash(-1) == hash(-2), for example)
        """
        # HASHCONS: these attributes key the cache
        # BEFORE CHANGING THIS, SEE ALL OTHER INSTANCES OF "HASHCONS" IN THIS FILE

        to_hash = Base._ast_serialize(op, args, annotations, length)

        hd = blake2b(to_hash, digest_size=8).digest()
        return blake2b_unpacker.unpack(hd)[0]

    @staticmethod
    def _ast_serialize(
        op: str,
        args: tuple[ArgType, ...],
        annotations: tuple[Annotation, ...],
        length: int | None,
    ) -> bytes:
        """
        Serialize the AST and get a bytestring for hashing.

        :param op:          The operator.
        :param args:        The args tuple.
        :param annotations: The annotations tuple.
        :return:            The serialized bytestring.
        """

        serialized_args = b"".join(b"<" + Base._arg_serialize(a) + b">" for a in args)
        serialized_annotations = b"".join(b"(" + Base._arg_serialize(a) + b")" for a in annotations)
        serialized_length = b"" if length is None else length.to_bytes(8, "little", signed=False)

        return b"{" + op.encode() + serialized_args + serialized_annotations + serialized_length + b"}"

    @staticmethod
    def _arg_serialize(arg: ArgType | Annotation) -> bytes:
        """Serialize one argument."""

        if isinstance(arg, Base):
            return arg._hash.to_bytes(8, "little", signed=False)
        if arg is None:
            return b"\x0f"
        if arg is True:
            return b"\x1f"
        if arg is False:
            return b"\x2e"
        if isinstance(arg, int):
            return arg.to_bytes((arg.bit_length() + 15) // 8, "little", signed=True)
        if isinstance(arg, str):
            return arg.encode()
        if isinstance(arg, float):
            if math.isnan(arg):
                return b"nan"
            if math.isinf(arg):
                return b"inf" if arg > 0 else b"-inf"
            if arg == 0.0 and math.copysign(1, arg) < 0:
                return b"-0.0"
            return struct.pack("d", arg)
        if isinstance(arg, tuple):
            return b"".join(b"<" + Base._arg_serialize(a) + b">" for a in arg)
        if hasattr(arg, "__hash__"):
            return hash(arg).to_bytes(8, "little", signed=True)

        log.debug("Don't know how to serialize %s, consider implementing __hash__", arg)
        return pickle.dumps(arg)

    def __hash__(self) -> int:
        return self._hash

    def hash(self) -> int:
        """Python's built in hash function is not collision resistant, so we use our own.
        When you call `hash(ast)`, the value is derived from the claripy hash, but it gets
        passed through python's non-resistent hash function first. This skips that step,
        allowing the claripy hash to be used directly, eg as a cache key.
        """
        return self._hash

    @property
    def _encoded_name(self) -> bytes:
        if self._cached_encoded_name is None:
            self._cached_encoded_name = self.args[0].encode()
        return self._cached_encoded_name

    def identical(self, other: Self) -> bool:
        """
        Check if two ASTs are identical. If `strict` is False, the comparison
        will be lenient on the names of the ASTs.
        """
        a = self.canonicalize()
        b = other.canonicalize()
        return a[:2] == b[:2] and a[2] is b[2]

    #
    # Annotations
    #

    def _apply_to_annotations(self, f):
        return self.make_like(self.op, self.args, annotations=f(self.annotations), skip_child_annotations=True)

    def has_annotation_type(self, annotation_type: type[Annotation]) -> bool:
        """
        Check if this AST has an annotation of a given type.

        :param annotation_type:     The type of the annotation.
        :return:                    True if the AST has an annotation of the given type.
        """
        return any(isinstance(a, annotation_type) for a in self.annotations)

    def get_annotations_by_type(self, annotation_type: type[A]) -> tuple[A, ...]:
        """
        Get all annotations of a given type.

        :param annotation_type:     The type of the annotation.
        :return:                    A tuple of annotations of the given type.
        """
        return tuple(a for a in self.annotations if isinstance(a, annotation_type))

    def get_annotation(self, annotation_type: type[A]) -> A | None:
        """
        Get the first annotation of a given type.

        :param annotation_type:     The type of the annotation.
        :return:                    The annotation of the given type, or None if not found.
        """
        for a in self.annotations:
            if isinstance(a, annotation_type):
                return a
        return None

    def append_annotation(self, a: Annotation) -> Self:
        """
        Appends an annotation to this AST.

        :param a:                   the annotation to append
        :returns:                   a new AST, with the annotation added
        """
        return self._apply_to_annotations(lambda alist: (*alist, a))

    def append_annotations(self, new_tuple: tuple[Annotation, ...]) -> Self:
        """
        Appends several annotations to this AST.

        :param new_tuple:           the tuple of annotations to append
        :returns:                   a new AST, with the annotations added
        """
        return self._apply_to_annotations(lambda alist: alist + new_tuple)

    def annotate(self, *args: Annotation, remove_annotations: Iterable[Annotation] | None = None) -> Self:
        """
        Appends annotations to this AST.

        :param args:                the tuple of annotations to append (variadic positional args)
        :param remove_annotations:  annotations to remove
        :returns:                   a new AST, with the annotations added
        """
        if not remove_annotations:
            return self._apply_to_annotations(lambda alist: alist + args)
        return self._apply_to_annotations(
            lambda alist: tuple(arg for arg in alist if arg not in remove_annotations) + args
        )

    def insert_annotation(self, a: Annotation) -> Self:
        """
        Inserts an annotation to this AST.

        :param a:                   the annotation to insert
        :returns:                   a new AST, with the annotation added
        """
        return self._apply_to_annotations(lambda alist: (a, *alist))

    def insert_annotations(self, new_tuple: tuple[Annotation, ...]) -> Self:
        """
        Inserts several annotations to this AST.

        :param new_tuple:           the tuple of annotations to insert
        :returns:                   a new AST, with the annotations added
        """
        return self._apply_to_annotations(lambda alist: new_tuple + alist)

    def replace_annotations(self, new_tuple: tuple[Annotation, ...]) -> Self:
        """
        Replaces annotations on this AST.

        :param new_tuple:           the tuple of annotations to replace the old annotations with
        :returns:                   a new AST, with the annotations added
        """
        return self._apply_to_annotations(lambda alist: new_tuple)

    def remove_annotation(self, a: Annotation) -> Self:
        """
        Removes an annotation from this AST.

        :param a:                   the annotation to remove
        :returns:                   a new AST, with the annotation removed
        """
        return self._apply_to_annotations(lambda alist: tuple(oa for oa in alist if oa != a))

    def remove_annotations(self, remove_sequence: Iterable[Annotation]) -> Self:
        """
        Removes several annotations from this AST.

        :param remove_sequence:     a sequence/set of the annotations to remove
        :returns:                   a new AST, with the annotations removed
        """
        return self._apply_to_annotations(lambda alist: tuple(oa for oa in alist if oa not in remove_sequence))

    def clear_annotations(self) -> Self:
        """
        Removes all annotations from this AST.

        :returns:                   a new AST, with all annotations removed
        """
        return self._apply_to_annotations(lambda _: ())

    def clear_annotation_type(self, annotation_type: type[Annotation]) -> Self:
        """
        Removes all annotations of a given type from this AST.

        :param annotation_type:     the type of annotations to remove
        :returns:                   a new AST, with the annotations removed
        """
        return self._apply_to_annotations(
            lambda alist: tuple(oa for oa in alist if not isinstance(oa, annotation_type))
        )

    #
    # Viewing and debugging
    #

    def dbg_repr(self, prefix=None) -> str:  # pylint:disable=unused-argument
        """
        Returns a debug representation of this AST.
        """
        return self.shallow_repr(max_depth=None, details=ReprLevel.FULL_REPR)

    def _type_name(self) -> str:
        return self.__class__.__name__

    def __repr__(self, inner=False, max_depth=None, explicit_length=False):
        return self.shallow_repr(max_depth=max_depth, explicit_length=explicit_length, inner=inner)

    def shallow_repr(
        self,
        max_depth: int = 8,
        explicit_length: bool = False,
        details: ReprLevel = ReprLevel.LITE_REPR,
        inner: bool = False,
        parent_prec: int = 15,
        left: bool = True,
    ) -> str:
        """
        Returns a string representation of this AST, but with a maximum depth to
        prevent floods of text being printed.

        :param max_depth:           The maximum depth to print.
        :param explicit_length:     Print lengths of BVV arguments.
        :param details:             An integer value specifying how detailed the output should be:
                                        LITE_REPR - print short repr for both operations and BVs,
                                        MID_REPR  - print full repr for operations and short for BVs,
                                        FULL_REPR - print full repr of both operations and BVs.
        :param inner:               whether or not it is an inner AST
        :param parent_prec:         parent operation precedence level
        :param left:                whether or not it is a left AST
        :returns:                   A string representing the AST
        """
        if max_depth is not None and max_depth <= 0:
            return "<...>"

        if self.op in operations.reversed_ops:
            op = operations.reversed_ops[self.op]
            args = reversed(self.args)
        else:
            op = self.op
            args = self.args

        next_max_depth = max_depth - 1 if max_depth is not None else None
        length = self.length if explicit_length else None
        # if operation is not in op_precedence, assign the "least operation precedence"
        op_prec = operations.op_precedence.get(op, 15)

        args = [
            (
                arg.shallow_repr(next_max_depth, explicit_length, details, True, op_prec, idx == 0)
                if isinstance(arg, Base)
                else arg
            )
            for idx, arg in enumerate(args)
        ]

        prec_diff = parent_prec - op_prec
        inner_infix_use_par = prec_diff < 0 or (prec_diff == 0 and not left)
        inner_repr = self._op_repr(op, args, inner, length, details, inner_infix_use_par)

        if not inner:
            return f"<{self._type_name()} {inner_repr}>"
        return inner_repr

    @staticmethod
    def _op_repr(
        op: str,
        args: tuple[ArgType, ...],
        inner: ArgType,
        length: int | None,
        details: ReprLevel,
        inner_infix_use_par: bool,
    ):
        if details < ReprLevel.FULL_REPR:
            if op == "BVS":
                return f"{args[0]}"

            if op == "BoolV":
                return str(args[0])

            if op == "BVV":
                if args[0] is None:
                    value = "!"
                elif args[1] < 10:
                    value = format(args[0], "")
                else:
                    value = format(args[0], "#x")
                return f"{value}#{length}" if length is not None else value

        if details < ReprLevel.MID_REPR:
            if op == "If":
                value = f"if {args[0]} then {args[1]} else {args[2]}"
                return f"({value})" if inner else value

            if op == "Not":
                return f"!{args[0]}"

            if op == "Extract":
                return f"{args[2]}[{args[0]}:{args[1]}]"

            if op == "ZeroExt":
                value = f"0#{args[0]} .. {args[1]}"
                return f"({value})" if inner else value

            if op in operations.prefix:
                assert len(args) == 1
                value = f"{operations.prefix[op]}{args[0]}"
                return f"({value})" if inner and inner_infix_use_par else value

            if op in operations.infix:
                value = f" {operations.infix[op]} ".join(args)
                return f"({value})" if inner and inner_infix_use_par else value

        return "{}({})".format(op, ", ".join(str(arg) for arg in args))

    def children_asts(self) -> Iterator[Base]:
        """
        Return an iterator over the nested children ASTs.
        """
        ast_queue = deque([iter(self.args)])
        while ast_queue:
            try:
                ast = next(ast_queue[-1])
            except StopIteration:
                ast_queue.pop()
                continue

            if isinstance(ast, Base):
                ast_queue.append(iter(ast.args))

                log.debug("Yielding AST %s with hash %s with %d children", ast, ast.hash(), len(ast.args))
                yield ast

    def leaf_asts(self) -> Iterator[Base]:
        """
        Return an iterator over the leaf ASTs.
        """
        seen = set()

        ast_queue = deque([self])
        while ast_queue:
            ast = ast_queue.pop()
            if isinstance(ast, Base) and ast.hash() not in seen:
                seen.add(ast.hash())

                if ast.depth == 1:
                    yield ast
                    continue

                ast_queue.extend(ast.args)
                continue

    def is_leaf(self) -> bool:
        """
        Check if this AST is a leaf node.
        """
        return self.depth == 1

    def dbg_is_looped(self) -> Base | bool:  # TODO: this return type is bad
        log.debug("Checking AST with hash %s for looping", self.hash())

        seen = set()
        for child_ast in self.children_asts():
            if child_ast.hash() in seen:
                return child_ast
            seen.add(child_ast.hash())

        return False

    #
    # Other helper functions
    #

    # we don't support iterating over Base objects
    def __iter__(self) -> NoReturn:
        """
        This prevents people from iterating over ASTs.
        """
        raise ClaripyOperationError("Please don't iterate over, or split, AST nodes!")

    def __bool__(self) -> NoReturn:
        """
        This prevents people from accidentally using an AST as a condition. For
        example, the following was previously common::

            a,b = two ASTs
            if a == b:
                do something

        The problem is that `a == b` would return an AST, because an AST can be symbolic
        and there could be no way to actually know the value of that without a
        constraint solve. This caused tons of issues.
        """
        raise ClaripyOperationError(
            "testing Expressions for truthiness does not do what you want, as these expressions can be symbolic"
        )

    def structurally_match(self, o: Base) -> bool:
        """
        Structurally compares two A objects, and check if their corresponding leaves are definitely the same A object
        (name-wise or hash-identity wise).

        :param o:                   the other claripy A object
        :returns:                   True/False
        """

        # TODO: Convert a and b into canonical forms

        if self.op != o.op:
            return False

        if len(self.args) != len(o.args):
            return False

        for arg_a, arg_b in zip(self.args, o.args, strict=False):
            if not isinstance(arg_a, Base):
                if type(arg_a) != type(arg_b):  # noqa: E721
                    return False
                # They are not ASTs
                if arg_a != arg_b:
                    return False
                continue

            if arg_a.is_leaf():
                if arg_a is not arg_b:
                    return False

            else:
                if not arg_a.structurally_match(arg_b):
                    return False

        return True

    def canonicalize(self, var_map=None, counter: int | None = None) -> tuple[dict[int, Base], int, Base]:
        ctr = itertools.count(0 if counter is None else counter)
        var_map = {} if var_map is None else var_map

        for v in self.leaf_asts():
            if v.hash() not in var_map and v.is_leaf():
                new_name = f"canonical_{next(ctr)}"
                match v.op:
                    case "BVS":
                        var_map[v.hash()] = claripy.BVS(new_name, v.length, explicit_name=True)
                    case "BoolS":
                        var_map[v.hash()] = claripy.BoolS(new_name, explicit_name=True)
                    case "FPS":
                        var_map[v.hash()] = claripy.FPS(new_name, v.args[1], explicit_name=True)
                    case "StringS":
                        var_map[v.hash()] = claripy.StringS(new_name, explicit_name=True)

        return var_map, next(ctr), claripy.replace_dict(self, var_map)

    #
    # these are convenience operations
    #

    @property
    def concrete_value(self):
        try:
            raw = claripy.backends.concrete.convert(self)
            if isinstance(raw, bool):
                return raw
            return raw.value
        except BackendError:
            return self

    @property
    def singlevalued(self) -> bool:
        return claripy.backends.any_backend.singlevalued(self)

    @property
    def multivalued(self) -> bool:
        return claripy.backends.any_backend.multivalued(self)

    @property
    def cardinality(self) -> int:
        return claripy.backends.any_backend.cardinality(self)

    @property
    def concrete(self) -> bool:
        return not self.symbolic
