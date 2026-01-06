import sys
import typing

if sys.version_info >= (3, 8):
    from typing import Protocol
else:
    Protocol = object  # pragma: no cover


from .types import (
    Concept,
    DeductionGuide,
    EnumDecl,
    Field,
    ForwardDecl,
    FriendDecl,
    Function,
    Method,
    NamespaceAlias,
    TemplateInst,
    Typedef,
    UsingAlias,
    UsingDecl,
    Variable,
    Value,
)

from .parserstate import (
    State,
    ClassBlockState,
    ExternBlockState,
    NamespaceBlockState,
    NonClassBlockState,
)


class CxxVisitor(Protocol):
    """
    Defines the interface used by the parser to emit events
    """

    def on_parse_start(self, state: NamespaceBlockState) -> None:
        """
        Called when parsing begins
        """

    def on_pragma(self, state: State, content: Value) -> None:
        """
        Called once for each ``#pragma`` directive encountered
        """

    def on_include(self, state: State, filename: str) -> None:
        """
        Called once for each ``#include`` directive encountered
        """

    def on_extern_block_start(self, state: ExternBlockState) -> typing.Optional[bool]:
        """
        .. code-block:: c++

            extern "C" {

            }

        If this function returns False, the visitor will not be called for any
        items inside this block (including on_extern_block_end)
        """

    def on_extern_block_end(self, state: ExternBlockState) -> None:
        """
        Called when an extern block ends
        """

    def on_namespace_start(self, state: NamespaceBlockState) -> typing.Optional[bool]:
        """
        Called when a ``namespace`` directive is encountered

        If this function returns False, the visitor will not be called for any
        items inside this namespace (including on_namespace_end)
        """

    def on_namespace_end(self, state: NamespaceBlockState) -> None:
        """
        Called at the end of a ``namespace`` block
        """

    def on_namespace_alias(
        self, state: NonClassBlockState, alias: NamespaceAlias
    ) -> None:
        """
        Called when a ``namespace`` alias is encountered
        """

    def on_concept(self, state: NonClassBlockState, concept: Concept) -> None:
        """
        .. code-block:: c++

            template <class T>
            concept Meowable = is_meowable<T>;
        """

    def on_forward_decl(self, state: State, fdecl: ForwardDecl) -> None:
        """
        Called when a forward declaration is encountered
        """

    def on_template_inst(self, state: State, inst: TemplateInst) -> None:
        """
        Called when an explicit template instantiation is encountered
        """

    def on_variable(self, state: State, v: Variable) -> None:
        """
        Called when a global variable is encountered
        """

    def on_function(self, state: NonClassBlockState, fn: Function) -> None:
        """
        Called when a function is encountered that isn't part of a class
        """

    def on_method_impl(self, state: NonClassBlockState, method: Method) -> None:
        """
        Called when a method implementation is encountered outside of a class
        declaration. For example:

        .. code-block:: c++

            void MyClass::fn() {
                // does something
            }

        .. note:: The above implementation is ambiguous, as it technically could
                  be a function in a namespace. We emit this instead as it's
                  more likely to be the case in common code.
        """

    def on_typedef(self, state: State, typedef: Typedef) -> None:
        """
        Called for each typedef instance encountered. For example:

        .. code-block:: c++

            typedef int T, *PT;

        Will result in ``on_typedef`` being called twice, once for ``T`` and
        once for ``*PT``
        """

    def on_using_namespace(
        self, state: NonClassBlockState, namespace: typing.List[str]
    ) -> None:
        """
        .. code-block:: c++

            using namespace std;
        """

    def on_using_alias(self, state: State, using: UsingAlias) -> None:
        """
        .. code-block:: c++

            using foo = int;

            template <typename T>
            using VectorT = std::vector<T>;

        """

    def on_using_declaration(self, state: State, using: UsingDecl) -> None:
        """
        .. code-block:: c++

            using NS::ClassName;

        """

    #
    # Enums
    #

    def on_enum(self, state: State, enum: EnumDecl) -> None:
        """
        Called after an enum is encountered
        """

    #
    # Class/union/struct
    #

    def on_class_start(self, state: ClassBlockState) -> typing.Optional[bool]:
        """
        Called when a class/struct/union is encountered

        When part of a typedef:

        .. code-block:: c++

            typedef struct { } X;

        This is called first, followed by on_typedef for each typedef instance
        encountered. The compound type object is passed as the type to the
        typedef.

        If this function returns False, the visitor will not be called for any
        items inside this class (including on_class_end)
        """

    def on_class_field(self, state: ClassBlockState, f: Field) -> None:
        """
        Called when a field of a class is encountered
        """

    def on_class_friend(self, state: ClassBlockState, friend: FriendDecl) -> None:
        """
        Called when a friend declaration is encountered
        """

    def on_class_method(self, state: ClassBlockState, method: Method) -> None:
        """
        Called when a method of a class is encountered inside of a class
        """

    def on_class_end(self, state: ClassBlockState) -> None:
        """
        Called when the end of a class/struct/union is encountered.

        When a variable like this is declared:

        .. code-block:: c++

            struct X {

            } x;

        Then ``on_class_start``, .. ``on_class_end`` are emitted, along with
        ``on_variable`` for each instance declared.
        """

    def on_deduction_guide(
        self, state: NonClassBlockState, guide: DeductionGuide
    ) -> None:
        """
        Called when a deduction guide is encountered
        """


class NullVisitor:
    """
    This visitor does nothing
    """

    def on_parse_start(self, state: NamespaceBlockState) -> None:
        return None

    def on_pragma(self, state: State, content: Value) -> None:
        return None

    def on_include(self, state: State, filename: str) -> None:
        return None

    def on_extern_block_start(self, state: ExternBlockState) -> typing.Optional[bool]:
        return None

    def on_extern_block_end(self, state: ExternBlockState) -> None:
        return None

    def on_namespace_start(self, state: NamespaceBlockState) -> typing.Optional[bool]:
        return None

    def on_namespace_end(self, state: NamespaceBlockState) -> None:
        return None

    def on_concept(self, state: NonClassBlockState, concept: Concept) -> None:
        return None

    def on_namespace_alias(
        self, state: NonClassBlockState, alias: NamespaceAlias
    ) -> None:
        return None

    def on_forward_decl(self, state: State, fdecl: ForwardDecl) -> None:
        return None

    def on_template_inst(self, state: State, inst: TemplateInst) -> None:
        return None

    def on_variable(self, state: State, v: Variable) -> None:
        return None

    def on_function(self, state: NonClassBlockState, fn: Function) -> None:
        return None

    def on_method_impl(self, state: NonClassBlockState, method: Method) -> None:
        return None

    def on_typedef(self, state: State, typedef: Typedef) -> None:
        return None

    def on_using_namespace(
        self, state: NonClassBlockState, namespace: typing.List[str]
    ) -> None:
        return None

    def on_using_alias(self, state: State, using: UsingAlias) -> None:
        return None

    def on_using_declaration(self, state: State, using: UsingDecl) -> None:
        return None

    def on_enum(self, state: State, enum: EnumDecl) -> None:
        return None

    def on_class_start(self, state: ClassBlockState) -> typing.Optional[bool]:
        return None

    def on_class_field(self, state: ClassBlockState, f: Field) -> None:
        return None

    def on_class_friend(self, state: ClassBlockState, friend: FriendDecl) -> None:
        return None

    def on_class_method(self, state: ClassBlockState, method: Method) -> None:
        return None

    def on_class_end(self, state: ClassBlockState) -> None:
        return None

    def on_deduction_guide(
        self, state: NonClassBlockState, guide: DeductionGuide
    ) -> None:
        return None


null_visitor = NullVisitor()
