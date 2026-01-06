"""

The simple parser/collector iterates over the C++ file and returns a data
structure with all elements in it. Not quite as flexible as implementing
your own parser listener, but you can accomplish most things with it.

cxxheaderparser's unit tests predominantly use the simple API for parsing,
so you can expect it to be pretty stable.

The :func:`parse_string` and :func:`parse_file` functions are a great place
to start:

.. code-block:: python

    from cxxheaderparser.simple import parse_string

    content = '''
        int x;
    '''

    parsed_data = parse_string(content)

See below for the contents of the returned :class:`ParsedData`.

"""

import os
import sys
import inspect
import typing


from dataclasses import dataclass, field

from .types import (
    ClassDecl,
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
    ClassBlockState,
    ExternBlockState,
    NamespaceBlockState,
)
from .parser import CxxParser
from .options import ParserOptions

#
# Data structure
#


@dataclass
class ClassScope:
    """
    Contains all data collected for a single C++ class
    """

    #: Information about the class declaration is here
    class_decl: ClassDecl

    #: Nested classes
    classes: typing.List["ClassScope"] = field(default_factory=list)
    enums: typing.List[EnumDecl] = field(default_factory=list)
    fields: typing.List[Field] = field(default_factory=list)
    friends: typing.List[FriendDecl] = field(default_factory=list)
    methods: typing.List[Method] = field(default_factory=list)
    typedefs: typing.List[Typedef] = field(default_factory=list)

    forward_decls: typing.List[ForwardDecl] = field(default_factory=list)
    using: typing.List[UsingDecl] = field(default_factory=list)
    using_alias: typing.List[UsingAlias] = field(default_factory=list)


@dataclass
class NamespaceScope:
    """
    Contains all data collected for a single namespace. Content for child
    namespaces are found in the ``namespaces`` attribute.
    """

    name: str = ""
    inline: bool = False
    doxygen: typing.Optional[str] = None

    classes: typing.List["ClassScope"] = field(default_factory=list)
    enums: typing.List[EnumDecl] = field(default_factory=list)

    #: Function declarations (with or without body)
    functions: typing.List[Function] = field(default_factory=list)

    #: Method implementations outside of a class (must have a body)
    method_impls: typing.List[Method] = field(default_factory=list)

    typedefs: typing.List[Typedef] = field(default_factory=list)
    variables: typing.List[Variable] = field(default_factory=list)

    forward_decls: typing.List[ForwardDecl] = field(default_factory=list)
    using: typing.List[UsingDecl] = field(default_factory=list)
    using_ns: typing.List["UsingNamespace"] = field(default_factory=list)
    using_alias: typing.List[UsingAlias] = field(default_factory=list)
    ns_alias: typing.List[NamespaceAlias] = field(default_factory=list)

    #: Concepts
    concepts: typing.List[Concept] = field(default_factory=list)

    #: Explicit template instantiations
    template_insts: typing.List[TemplateInst] = field(default_factory=list)

    #: Child namespaces
    namespaces: typing.Dict[str, "NamespaceScope"] = field(default_factory=dict)

    #: Deduction guides
    deduction_guides: typing.List[DeductionGuide] = field(default_factory=list)


Block = typing.Union[ClassScope, NamespaceScope]


@dataclass
class Pragma:
    content: Value


@dataclass
class Include:
    #: The filename includes the surrounding ``<>`` or ``"``
    filename: str


@dataclass
class UsingNamespace:
    ns: str


@dataclass
class ParsedData:
    """
    Container for information parsed by the :func:`parse_file` and
    :func:`parse_string` functions.

    .. warning:: Names are not resolved, so items are stored in the scope that
                 they are found. For example:

                 .. code-block:: c++

                    namespace N {
                        class C;
                    }

                    class N::C {
                        void fn();
                    };

                 The 'C' class would be a forward declaration in the 'N' namespace,
                 but the ClassDecl for 'C' would be stored in the global
                 namespace instead of the 'N' namespace.
    """

    #: Global namespace
    namespace: NamespaceScope = field(default_factory=lambda: NamespaceScope())

    #: Any ``#pragma`` directives encountered
    pragmas: typing.List[Pragma] = field(default_factory=list)

    #: Any ``#include`` directives encountered
    includes: typing.List[Include] = field(default_factory=list)


#
# Visitor implementation
#

# define what user data we store in each state type
SClassBlockState = ClassBlockState[ClassScope, Block]
SExternBlockState = ExternBlockState[NamespaceScope, NamespaceScope]
SNamespaceBlockState = NamespaceBlockState[NamespaceScope, NamespaceScope]

SState = typing.Union[SClassBlockState, SExternBlockState, SNamespaceBlockState]
SNonClassBlockState = typing.Union[SExternBlockState, SNamespaceBlockState]


class SimpleCxxVisitor:
    """
    A simple visitor that stores all of the C++ elements passed to it
    in an "easy" to use data structure

    You probably don't want to use this directly, use :func:`parse_file`
    or  :func:`parse_string` instead.
    """

    data: ParsedData

    def on_parse_start(self, state: SNamespaceBlockState) -> None:
        ns = NamespaceScope("")
        self.data = ParsedData(ns)
        state.user_data = ns

    def on_pragma(self, state: SState, content: Value) -> None:
        self.data.pragmas.append(Pragma(content))

    def on_include(self, state: SState, filename: str) -> None:
        self.data.includes.append(Include(filename))

    def on_extern_block_start(self, state: SExternBlockState) -> typing.Optional[bool]:
        state.user_data = state.parent.user_data
        return None

    def on_extern_block_end(self, state: SExternBlockState) -> None:
        pass

    def on_namespace_start(self, state: SNamespaceBlockState) -> typing.Optional[bool]:
        parent_ns = state.parent.user_data

        ns = None
        names = state.namespace.names
        if not names:
            # all anonymous namespaces in a translation unit are the same
            names = [""]

        for name in names:
            ns = parent_ns.namespaces.get(name)
            if ns is None:
                ns = NamespaceScope(name)
                parent_ns.namespaces[name] = ns
            parent_ns = ns

        assert ns is not None

        # only set inline/doxygen on inner namespace
        ns.inline = state.namespace.inline
        ns.doxygen = state.namespace.doxygen

        state.user_data = ns
        return None

    def on_namespace_end(self, state: SNamespaceBlockState) -> None:
        pass

    def on_concept(self, state: SNonClassBlockState, concept: Concept) -> None:
        state.user_data.concepts.append(concept)

    def on_namespace_alias(
        self, state: SNonClassBlockState, alias: NamespaceAlias
    ) -> None:
        state.user_data.ns_alias.append(alias)

    def on_forward_decl(self, state: SState, fdecl: ForwardDecl) -> None:
        state.user_data.forward_decls.append(fdecl)

    def on_template_inst(self, state: SState, inst: TemplateInst) -> None:
        assert isinstance(state.user_data, NamespaceScope)
        state.user_data.template_insts.append(inst)

    def on_variable(self, state: SState, v: Variable) -> None:
        assert isinstance(state.user_data, NamespaceScope)
        state.user_data.variables.append(v)

    def on_function(self, state: SNonClassBlockState, fn: Function) -> None:
        state.user_data.functions.append(fn)

    def on_method_impl(self, state: SNonClassBlockState, method: Method) -> None:
        state.user_data.method_impls.append(method)

    def on_typedef(self, state: SState, typedef: Typedef) -> None:
        state.user_data.typedefs.append(typedef)

    def on_using_namespace(
        self, state: SNonClassBlockState, namespace: typing.List[str]
    ) -> None:
        ns = UsingNamespace("::".join(namespace))
        state.user_data.using_ns.append(ns)

    def on_using_alias(self, state: SState, using: UsingAlias) -> None:
        state.user_data.using_alias.append(using)

    def on_using_declaration(self, state: SState, using: UsingDecl) -> None:
        state.user_data.using.append(using)

    #
    # Enums
    #

    def on_enum(self, state: SState, enum: EnumDecl) -> None:
        state.user_data.enums.append(enum)

    #
    # Class/union/struct
    #

    def on_class_start(self, state: SClassBlockState) -> typing.Optional[bool]:
        parent = state.parent.user_data
        block = ClassScope(state.class_decl)
        parent.classes.append(block)
        state.user_data = block
        return None

    def on_class_field(self, state: SClassBlockState, f: Field) -> None:
        state.user_data.fields.append(f)

    def on_class_method(self, state: SClassBlockState, method: Method) -> None:
        state.user_data.methods.append(method)

    def on_class_friend(self, state: SClassBlockState, friend: FriendDecl) -> None:
        state.user_data.friends.append(friend)

    def on_class_end(self, state: SClassBlockState) -> None:
        pass

    def on_deduction_guide(
        self, state: SNonClassBlockState, guide: DeductionGuide
    ) -> None:
        state.user_data.deduction_guides.append(guide)


def parse_string(
    content: str,
    *,
    filename: str = "<str>",
    options: typing.Optional[ParserOptions] = None,
    cleandoc: bool = False,
) -> ParsedData:
    """
    Simple function to parse a header and return a data structure
    """
    if cleandoc:
        content = inspect.cleandoc(content)

    visitor = SimpleCxxVisitor()
    parser = CxxParser(filename, content, visitor, options)
    parser.parse()

    return visitor.data


def parse_file(
    filename: typing.Union[str, os.PathLike],
    encoding: typing.Optional[str] = None,
    *,
    options: typing.Optional[ParserOptions] = None,
) -> ParsedData:
    """
    Simple function to parse a header from a file and return a data structure
    """
    filename = os.fsdecode(filename)

    if encoding is None:
        encoding = "utf-8-sig"

    if filename == "-":
        content = sys.stdin.read()
    else:
        content = None

    visitor = SimpleCxxVisitor()
    parser = CxxParser(filename, content, visitor, options)
    parser.parse()

    return visitor.data
