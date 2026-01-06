import typing

if typing.TYPE_CHECKING:
    from .visitor import CxxVisitor  # pragma: nocover

from .errors import CxxParseError
from .lexer import LexToken, Location
from .types import ClassDecl, NamespaceDecl


class ParsedTypeModifiers(typing.NamedTuple):
    vars: typing.Dict[str, LexToken]  # only found on variables
    both: typing.Dict[str, LexToken]  # found on either variables or functions
    meths: typing.Dict[str, LexToken]  # only found on methods

    def validate(self, *, var_ok: bool, meth_ok: bool, msg: str) -> None:
        # Almost there! Do any checks the caller asked for
        if not var_ok and self.vars:
            for tok in self.vars.values():
                raise CxxParseError(f"{msg}: unexpected '{tok.value}'")

        if not meth_ok and self.meths:
            for tok in self.meths.values():
                raise CxxParseError(f"{msg}: unexpected '{tok.value}'")

        if not meth_ok and not var_ok and self.both:
            for tok in self.both.values():
                raise CxxParseError(f"{msg}: unexpected '{tok.value}'")


#: custom user data for this state type
T = typing.TypeVar("T")

#: type of custom user data for a parent state
PT = typing.TypeVar("PT")


class BaseState(typing.Generic[T, PT]):
    #: Uninitialized user data available for use by visitor implementations. You
    #: should set this in a ``*_start`` method.
    user_data: T

    #: parent state
    parent: typing.Optional["State"]

    #: Approximate location that the parsed element was found at
    location: Location

    #: internal detail used by parser
    _prior_visitor: "CxxVisitor"

    def __init__(self, parent: typing.Optional["State"], location: Location) -> None:
        self.parent = parent
        self.location = location

    def _finish(self, visitor: "CxxVisitor") -> None:
        pass


class ExternBlockState(BaseState[T, PT]):
    parent: "NonClassBlockState"

    #: The linkage for this extern block
    linkage: str

    def __init__(
        self, parent: "NonClassBlockState", location: Location, linkage: str
    ) -> None:
        super().__init__(parent, location)
        self.linkage = linkage

    def _finish(self, visitor: "CxxVisitor") -> None:
        visitor.on_extern_block_end(self)


class NamespaceBlockState(BaseState[T, PT]):
    parent: "NonClassBlockState"

    #: The incremental namespace for this block
    namespace: NamespaceDecl

    def __init__(
        self,
        parent: typing.Optional["NonClassBlockState"],
        location: Location,
        namespace: NamespaceDecl,
    ) -> None:
        super().__init__(parent, location)
        self.namespace = namespace

    def _finish(self, visitor: "CxxVisitor") -> None:
        visitor.on_namespace_end(self)


class ClassBlockState(BaseState[T, PT]):
    parent: "State"

    #: class decl block being processed
    class_decl: ClassDecl

    #: Current access level for items encountered
    access: str

    #: Currently parsing as a typedef
    typedef: bool

    #: modifiers to apply to following variables
    mods: ParsedTypeModifiers

    def __init__(
        self,
        parent: typing.Optional["State"],
        location: Location,
        class_decl: ClassDecl,
        access: str,
        typedef: bool,
        mods: ParsedTypeModifiers,
    ) -> None:
        super().__init__(parent, location)
        self.class_decl = class_decl
        self.access = access
        self.typedef = typedef
        self.mods = mods

    def _set_access(self, access: str) -> None:
        self.access = access

    def _finish(self, visitor: "CxxVisitor") -> None:
        visitor.on_class_end(self)


State = typing.Union[
    NamespaceBlockState[T, PT], ExternBlockState[T, PT], ClassBlockState[T, PT]
]
NonClassBlockState = typing.Union[ExternBlockState[T, PT], NamespaceBlockState[T, PT]]
