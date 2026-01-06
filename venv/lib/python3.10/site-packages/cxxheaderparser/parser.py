from collections import deque

import inspect
import re
import typing

from . import lexer
from .errors import CxxParseError
from .lexer import LexToken, Location, PhonyEnding
from .options import ParserOptions
from .parserstate import (
    ClassBlockState,
    ExternBlockState,
    NamespaceBlockState,
    NonClassBlockState,
    ParsedTypeModifiers,
    State,
)
from .types import (
    AnonymousName,
    Array,
    AutoSpecifier,
    BaseClass,
    ClassDecl,
    Concept,
    DecltypeSpecifier,
    DecoratedType,
    DeductionGuide,
    EnumDecl,
    Enumerator,
    Field,
    ForwardDecl,
    FriendDecl,
    Function,
    FunctionType,
    FundamentalSpecifier,
    Method,
    MoveReference,
    NameSpecifier,
    NamespaceAlias,
    NamespaceDecl,
    PQNameSegment,
    Parameter,
    PQName,
    Pointer,
    Reference,
    TemplateArgument,
    TemplateDecl,
    TemplateDeclTypeVar,
    TemplateInst,
    TemplateNonTypeParam,
    TemplateParam,
    TemplateSpecialization,
    TemplateTypeParam,
    Token,
    Type,
    Typedef,
    UsingAlias,
    UsingDecl,
    Value,
    Variable,
)
from .visitor import CxxVisitor, null_visitor

LexTokenList = typing.List[LexToken]
T = typing.TypeVar("T")

PT = typing.TypeVar("PT", Parameter, TemplateNonTypeParam)


class CxxParser:
    """
    Single-use parser object
    """

    def __init__(
        self,
        filename: str,
        content: typing.Optional[str],
        visitor: CxxVisitor,
        options: typing.Optional[ParserOptions] = None,
        encoding: typing.Optional[str] = None,
    ) -> None:
        self.visitor = visitor
        self.filename = filename
        self.options = options if options else ParserOptions()

        if options and options.preprocessor is not None:
            content = options.preprocessor(filename, content)

        if content is None:
            if encoding is None:
                encoding = "utf-8-sig"

            with open(filename, "r", encoding=encoding) as fp:
                content = fp.read()

        self.lex: lexer.TokenStream = lexer.LexerTokenStream(filename, content)

        global_ns = NamespaceDecl([], False)
        self.current_namespace = global_ns

        self.state: State = NamespaceBlockState(
            None, self.lex.current_location(), global_ns
        )
        self.anon_id = 0

        self.verbose = self.options.verbose
        if self.verbose:

            def debug_print(fmt: str, *args: typing.Any) -> None:
                fmt = f"[%4d] {fmt}"
                args = (inspect.currentframe().f_back.f_lineno,) + args  # type: ignore
                print(fmt % args)

            self.debug_print = debug_print
        else:
            self.debug_print = lambda fmt, *args: None

        self.visitor.on_parse_start(self.state)

    #
    # State management
    #

    def _setup_state(self, state: State):
        state._prior_visitor = self.visitor
        self.state = state

    def _pop_state(self) -> State:
        prev_state = self.state
        prev_state._finish(self.visitor)
        self.visitor = prev_state._prior_visitor
        state = prev_state.parent
        if state is None:
            raise CxxParseError("INTERNAL ERROR: unbalanced state")

        if isinstance(state, NamespaceBlockState):
            self.current_namespace = state.namespace

        self.state = state
        return prev_state

    @property
    def _current_access(self) -> typing.Optional[str]:
        return getattr(self.state, "access", None)

    #
    # Utility parsing functions used by the rest of the code
    #

    def _parse_error(
        self, tok: typing.Optional[LexToken], expected: str = ""
    ) -> CxxParseError:
        if not tok:
            # common case after a failed token_if
            tok = self.lex.token()

        if expected:
            expected = f", expected '{expected}'"

        msg = f"unexpected '{tok.value}'{expected}"

        # TODO: better error message
        return CxxParseError(msg, tok)

    def _next_token_must_be(self, *tokenTypes: str) -> LexToken:
        tok = self.lex.token()
        if tok.type not in tokenTypes:
            raise self._parse_error(tok, "' or '".join(tokenTypes))
        return tok

    # def _next_token_in_set(self, tokenTypes: typing.Set[str]) -> LexToken:
    #     tok = self.lex.token()
    #     if tok.type not in tokenTypes:
    #         raise self._parse_error(tok, "' or '".join(sorted(tokenTypes)))
    #     return tok

    # def _consume_up_to(self, rtoks: LexTokenList, *token_types: str) -> LexTokenList:
    #     # includes the last token
    #     get_token = self.lex.token
    #     while True:
    #         tok = get_token()
    #         rtoks.append(tok)
    #         if tok.type in token_types:
    #             break
    #     return rtoks

    def _consume_until(self, rtoks: LexTokenList, *token_types: str) -> LexTokenList:
        # does not include the found token
        token_if_not = self.lex.token_if_not
        while True:
            tok = token_if_not(*token_types)
            if tok is None:
                break
            rtoks.append(tok)
        return rtoks

    def _consume_value_until(
        self, rtoks: LexTokenList, *token_types: str
    ) -> LexTokenList:
        # does not include the found token
        token_if_not = self.lex.token_if_not
        while True:
            tok = token_if_not(*token_types)
            if tok is None:
                break

            if tok.type in self._balanced_token_map:
                rtoks.extend(self._consume_balanced_tokens(tok))
            else:
                rtoks.append(tok)

        return rtoks

    _end_balanced_tokens = {">", "}", "]", ")", "DBL_RBRACKET"}
    _balanced_token_map = {
        "<": ">",
        "{": "}",
        "(": ")",
        "[": "]",
        "DBL_LBRACKET": "DBL_RBRACKET",
    }

    def _consume_balanced_tokens(
        self,
        *init_tokens: LexToken,
        token_map: typing.Optional[typing.Dict[str, str]] = None,
    ) -> LexTokenList:
        if token_map is None:
            token_map = self._balanced_token_map

        consumed = list(init_tokens)
        match_stack = deque((token_map[tok.type] for tok in consumed))
        get_token = self.lex.token

        tok: typing.Optional[LexToken]

        while True:
            tok = get_token()
            consumed.append(tok)

            if tok.type in self._end_balanced_tokens:
                expected = match_stack.pop()
                if tok.type != expected:
                    # hack: we only claim to parse correct code, so if this
                    # is less than or greater than, assume that the code is
                    # doing math and so this unexpected item is correct.
                    #
                    # If one of the other items on the stack match, pop back
                    # to that. Otherwise, ignore it and hope for the best
                    if tok.type != ">" and expected != ">":
                        raise self._parse_error(tok, expected)

                    for i, maybe in enumerate(reversed(match_stack)):
                        if tok.type == maybe:
                            for _ in range(i + 1):
                                match_stack.pop()
                            break
                    else:
                        match_stack.append(expected)
                        continue

                if len(match_stack) == 0:
                    return consumed

                continue

            next_end = token_map.get(tok.type)
            if next_end:
                match_stack.append(next_end)

    def _discard_contents(self, start_type: str, end_type: str) -> None:
        # use this instead of consume_balanced_tokens because
        # we don't care at all about the internals
        level = 1
        get_token = self.lex.token
        while True:
            tok = get_token()
            if tok.type == start_type:
                level += 1
            elif tok.type == end_type:
                level -= 1
                if level == 0:
                    break

    def _create_value(self, toks: LexTokenList) -> Value:
        return Value([Token(tok.value, tok.type) for tok in toks])

    #
    # Parsing begins here
    #

    def parse(self) -> None:
        """
        Parse the header contents
        """

        # non-ambiguous parsing functions for each token type
        _translation_unit_tokens: typing.Dict[
            str, typing.Callable[[LexToken, typing.Optional[str]], typing.Any]
        ] = {
            "__attribute__": self._consume_gcc_attribute,
            "__declspec": self._consume_declspec,
            "alignas": self._consume_attribute_specifier_seq,
            "extern": self._parse_extern,
            "friend": self._parse_friend_decl,
            "inline": self._parse_inline,
            "namespace": self._parse_namespace,
            "private": self._process_access_specifier,
            "protected": self._process_access_specifier,
            "public": self._process_access_specifier,
            "static_assert": self._consume_static_assert,
            "template": self._parse_template,
            "typedef": self._parse_typedef,
            "using": self._parse_using,
            "{": self._on_empty_block_start,
            "}": self._on_block_end,
            "DBL_LBRACKET": self._consume_attribute_specifier_seq,
            "INCLUDE_DIRECTIVE": self._process_include_directive,
            "PRAGMA_DIRECTIVE": self._process_pragma_directive,
            ";": lambda _1, _2: None,
        }

        _keep_doxygen = {"__declspec", "alignas", "__attribute__", "DBL_LBRACKET"}

        tok = None

        get_token_eof_ok = self.lex.token_eof_ok
        get_doxygen = self.lex.get_doxygen

        doxygen = None

        try:
            while True:
                if doxygen is None:
                    doxygen = get_doxygen()

                tok = get_token_eof_ok()
                if not tok:
                    break

                fn = _translation_unit_tokens.get(tok.type)
                if fn:
                    fn(tok, doxygen)

                    if tok.type not in _keep_doxygen:
                        doxygen = None
                else:
                    # this processes ambiguous declarations
                    self._parse_declarations(tok, doxygen)
                    doxygen = None

        except Exception as e:
            if self.verbose:
                raise
            context = ""
            if isinstance(e, CxxParseError):
                context = ": " + str(e)
                if e.tok:
                    tok = e.tok

            if tok:
                filename, lineno = tok.location
                msg = f"{filename}:{lineno}: parse error evaluating '{tok.value}'{context}"
            else:
                msg = f"{self.filename}: parse error{context}"

            raise CxxParseError(msg) from e

    #
    # Preprocessor directives
    #

    _preprocessor_compress_re = re.compile(r"^#[\t ]+")
    _preprocessor_split_re = re.compile(r"[\t ]+")

    def _process_include_directive(self, tok: LexToken, doxygen: typing.Optional[str]):
        value = self._preprocessor_compress_re.sub("#", tok.value)
        svalue = self._preprocessor_split_re.split(value, 1)
        if len(svalue) == 2:
            self.state.location = tok.location
            self.visitor.on_include(self.state, svalue[1])
        else:
            raise CxxParseError("incomplete #include directive", tok)

    def _process_pragma_directive(self, _: LexToken, doxygen: typing.Optional[str]):
        # consume all tokens until the end of the line
        # -- but if we find a paren, get the group
        tokens: LexTokenList = []
        while True:
            tok = self.lex.token_newline_eof_ok()
            if not tok or tok.type == "NEWLINE":
                break
            if tok.type in self._balanced_token_map:
                tokens.extend(self._consume_balanced_tokens(tok))
            else:
                tokens.append(tok)

        self.visitor.on_pragma(self.state, self._create_value(tokens))

    #
    # Various
    #

    _msvc_conventions = {
        "__cdecl",
        "__clrcall",
        "__stdcall",
        "__fastcall",
        "__thiscall",
        "__vectorcall",
    }

    def _parse_namespace(
        self, tok: LexToken, doxygen: typing.Optional[str], inline: bool = False
    ) -> None:
        """
        namespace_definition: named_namespace_definition
                            | unnamed_namespace_definition

        named_namespace_definition: ["inline"] "namespace" IDENTIFIER "{" namespace_body "}"

        unnamed_namespace_definition: ["inline"] "namespace" "{" namespace_body "}"

        namespace_alias_definition: "namespace" IDENTIFIER "=" qualified_namespace_specifier ";"
        """

        names = []
        location = tok.location
        ns_alias: typing.Union[typing.Literal[False], LexToken] = False

        tok = self._next_token_must_be("NAME", "{")
        if tok.type != "{":
            endtok = "{"
            # Check for namespace alias here
            etok = self.lex.token_if("=")
            if etok:
                ns_alias = tok
                endtok = ";"
                # They can start with ::
                maybe_tok = self.lex.token_if("DBL_COLON")
                if maybe_tok:
                    names.append(maybe_tok.value)
                tok = self._next_token_must_be("NAME")

            while True:
                names.append(tok.value)

                atok = self.lex.token_if_in_set(self._attribute_start_tokens)
                if atok:
                    self._consume_attribute(atok)

                tok = self._next_token_must_be("DBL_COLON", endtok)
                if tok.type == endtok:
                    break

                tok = self._next_token_must_be("NAME")

        if inline and len(names) > 1:
            raise CxxParseError("a nested namespace definition cannot be inline")

        state = self.state
        if not isinstance(state, (NamespaceBlockState, ExternBlockState)):
            raise CxxParseError("namespace cannot be defined in a class")

        if ns_alias:
            alias = NamespaceAlias(ns_alias.value, names)
            self.visitor.on_namespace_alias(state, alias)
            return

        ns = NamespaceDecl(names, inline, doxygen)

        state = NamespaceBlockState(state, location, ns)
        self._setup_state(state)
        self.current_namespace = state.namespace

        if self.visitor.on_namespace_start(state) is False:
            self.visitor = null_visitor

    def _parse_extern(self, tok: LexToken, doxygen: typing.Optional[str]) -> None:
        etok = self.lex.token_if("STRING_LITERAL", "template")
        if etok:
            # classes cannot contain extern blocks/templates
            state = self.state
            if isinstance(state, ClassBlockState):
                raise self._parse_error(tok)

            if etok.type == "STRING_LITERAL":
                if self.lex.token_if("{"):
                    state = ExternBlockState(state, tok.location, etok.value)
                    self._setup_state(state)

                    if self.visitor.on_extern_block_start(state) is False:
                        self.visitor = null_visitor
                    return

                # an extern variable/function with specific linkage
                self.lex.return_token(etok)
            else:
                # must be an extern template instantitation
                self._parse_template_instantiation(doxygen, True)
                return

        self._parse_declarations(tok, doxygen)

    def _parse_friend_decl(
        self,
        tok: LexToken,
        doxygen: typing.Optional[str],
        template: typing.Optional[TemplateDecl] = None,
    ) -> None:
        if not isinstance(self.state, ClassBlockState):
            raise self._parse_error(tok)

        tok = self.lex.token()
        self._parse_declarations(tok, doxygen, template, is_friend=True)

    def _parse_inline(self, tok: LexToken, doxygen: typing.Optional[str]) -> None:
        itok = self.lex.token_if("namespace")
        if itok:
            self._parse_namespace(itok, doxygen, inline=True)
        else:
            self._parse_declarations(tok, doxygen)

    def _parse_typedef(self, tok: LexToken, doxygen: typing.Optional[str]) -> None:
        tok = self.lex.token()
        self._parse_declarations(tok, doxygen, is_typedef=True)

    def _consume_static_assert(
        self, tok: LexToken, doxygen: typing.Optional[str]
    ) -> None:
        self._next_token_must_be("(")
        self._discard_contents("(", ")")

    def _on_empty_block_start(
        self, tok: LexToken, doxygen: typing.Optional[str]
    ) -> None:
        raise self._parse_error(tok)

    def _on_block_end(self, tok: LexToken, doxygen: typing.Optional[str]) -> None:
        old_state = self._pop_state()
        if isinstance(old_state, ClassBlockState):
            self._finish_class_decl(old_state)

    #
    # Template and concept parsing
    #

    def _parse_template_type_parameter(
        self, tok: LexToken, template: typing.Optional[TemplateDecl]
    ) -> TemplateTypeParam:
        """
        type_parameter: "class" ["..."] [IDENTIFIER]
                      | "class" [IDENTIFIER] "=" type_id
                      | "typename" ["..."] [IDENTIFIER]
                      | "typename" [IDENTIFIER] "=" type_id
                      | "template" "<" template_parameter_list ">" "class" ["..."] [IDENTIFIER]
                      | "template" "<" template_parameter_list ">" "class" [IDENTIFIER] "=" id_expression
        """
        # entry: token is either class or typename
        typekey = tok.type
        param_pack = True if self.lex.token_if("ELLIPSIS") else False
        name = None
        default = None

        otok = self.lex.token_if("NAME")
        if otok:
            name = otok.value

        otok = self.lex.token_if("=")
        if otok:
            default = self._create_value(self._consume_value_until([], ",", ">"))

        return TemplateTypeParam(typekey, name, param_pack, default, template)

    def _parse_template_decl(self) -> TemplateDecl:
        """
        template_declaration: "template" "<" template_parameter_list ">" declaration

        explicit_specialization: "template" "<" ">" declaration

        template_parameter_list: template_parameter
                               | template_parameter_list "," template_parameter

        template_parameter: type_parameter
                          | parameter_declaration
        """
        tok = self._next_token_must_be("<")
        params: typing.List[TemplateParam] = []

        lex = self.lex

        if not lex.token_if(">"):
            while True:
                tok = lex.token()
                tok_type = tok.type

                param: TemplateParam

                if tok_type == "template":
                    template = self._parse_template_decl()
                    tok = self._next_token_must_be("class", "typename")
                    param = self._parse_template_type_parameter(tok, template)
                elif tok_type == "class":
                    param = self._parse_template_type_parameter(tok, None)
                elif tok_type == "typename":
                    ptok = lex.token()
                    if ptok.type in ("ELLIPSIS", "=", ",", ">") or (
                        ptok.type == "NAME" and lex.token_peek_if("=", ",", ">")
                    ):
                        lex.return_token(ptok)
                        param = self._parse_template_type_parameter(tok, None)
                    else:
                        lex.return_token(ptok)
                        param, _ = self._parse_parameter(
                            tok,
                            TemplateNonTypeParam,
                            concept_ok=False,
                            deduce_this_ok=False,
                            end=">",
                        )
                else:
                    param, _ = self._parse_parameter(
                        tok,
                        TemplateNonTypeParam,
                        concept_ok=False,
                        deduce_this_ok=False,
                        end=">",
                    )

                params.append(param)

                tok = self._next_token_must_be(",", ">")
                if tok.type == ">":
                    break

        return TemplateDecl(params)

    def _parse_template(self, tok: LexToken, doxygen: typing.Optional[str]) -> None:
        if not self.lex.token_peek_if("<"):
            self._parse_template_instantiation(doxygen, False)
            return

        template = self._parse_template_decl()

        # Check for multiple specializations
        tok = self.lex.token()
        if tok.type == "template":
            templates = [template]
            while tok.type == "template":
                templates.append(self._parse_template_decl())
                tok = self.lex.token()

            # Can only be followed by declarations
            self._parse_declarations(tok, doxygen, templates)
            return

        if tok.type == "using":
            self._parse_using(tok, doxygen, template)
        elif tok.type == "friend":
            self._parse_friend_decl(tok, doxygen, template)
        elif tok.type == "concept":
            self._parse_concept(tok, doxygen, template)
        elif tok.type == "requires":
            template.raw_requires_pre = self._parse_requires(tok)
            self._parse_declarations(self.lex.token(), doxygen, template)
        else:
            self._parse_declarations(tok, doxygen, template)

    def _parse_template_specialization(self) -> TemplateSpecialization:
        """
        template_argument_list: template_argument ["..."]
                              | template_argument_list "," template_argument ["..."]

        template_argument: constant_expression
                         | type_id
                         | id_expression

        template_id: simple_template_id
                   | operator_function_id "<" [template_argument_list] ">"
                   | literal_operator_id "<" [template_argument_list] ">"

        simple_template_id: IDENTIFIER "<" [template_argument_list] ">"
        """

        args: typing.List[TemplateArgument] = []

        # On entry, < has just been consumed

        while True:
            # We don't know whether each argument will be a type or an expression.
            # Retrieve the expression first, then try to parse the name using those
            # tokens. If it succeeds we're done, otherwise we use the value

            param_pack = False

            raw_toks = self._consume_value_until([], ",", ">", "ELLIPSIS")
            val = self._create_value(raw_toks)
            dtype = None

            if raw_toks and raw_toks[0].type in self._pqname_start_tokens:
                # append a token to make other parsing components happy
                raw_toks.append(PhonyEnding)

                old_lex = self.lex
                try:
                    # set up a temporary token stream with the tokens we need to parse
                    tmp_lex = lexer.BoundedTokenStream(raw_toks)
                    self.lex = tmp_lex

                    try:
                        parsed_type, mods = self._parse_type(None)
                        if parsed_type is None:
                            raise self._parse_error(None)

                        mods.validate(var_ok=False, meth_ok=False, msg="")
                        dtype = self._parse_cv_ptr_or_fn(parsed_type, nonptr_fn=True)
                        self._next_token_must_be(PhonyEnding.type)
                    except CxxParseError:
                        dtype = None
                    else:
                        if tmp_lex.has_tokens():
                            dtype = None

                finally:
                    self.lex = old_lex

            if self.lex.token_if("ELLIPSIS"):
                param_pack = True

            if dtype:
                args.append(TemplateArgument(dtype, param_pack))
            else:
                # special case for sizeof...(thing)
                if param_pack and val.tokens[-1].value == "sizeof":
                    val.tokens.append(Token("...", "ELLIPSIS"))
                    tok = self._next_token_must_be("(")
                    raw_toks = self._consume_balanced_tokens(tok)
                    val.tokens.extend(Token(tok.value, tok.type) for tok in raw_toks)

                args.append(TemplateArgument(val, param_pack))

            tok = self._next_token_must_be(",", ">")
            if tok.type == ">":
                break

        return TemplateSpecialization(args)

    def _parse_template_instantiation(
        self, doxygen: typing.Optional[str], extern: bool
    ):
        """
        explicit-instantiation: [extern] template declaration
        """

        # entry is right after template

        tok = self.lex.token_if("class", "struct")
        if not tok:
            raise self._parse_error(tok)

        atok = self.lex.token_if_in_set(self._attribute_start_tokens)
        if atok:
            self._consume_attribute(atok)

        typename, _ = self._parse_pqname(None)

        # the last segment must have a specialization
        last_segment = typename.segments[-1]
        if (
            not isinstance(last_segment, NameSpecifier)
            or not last_segment.specialization
        ):
            raise self._parse_error(
                None, "expected extern template to have specialization"
            )

        self._next_token_must_be(";")

        self.visitor.on_template_inst(
            self.state, TemplateInst(typename, extern, doxygen)
        )

    def _parse_concept(
        self,
        tok: LexToken,
        doxygen: typing.Optional[str],
        template: TemplateDecl,
    ) -> None:
        name = self._next_token_must_be("NAME")
        self._next_token_must_be("=")

        # not trying to understand this for now
        raw_constraint = self._create_value(self._consume_value_until([], ",", ";"))

        state = self.state
        if isinstance(state, ClassBlockState):
            raise CxxParseError("concept cannot be defined in a class")

        self.visitor.on_concept(
            state,
            Concept(
                template=template,
                name=name.value,
                raw_constraint=raw_constraint,
                doxygen=doxygen,
            ),
        )

    def _parse_requires(
        self,
        tok: LexToken,
    ) -> Value:
        tok = self.lex.token()

        rawtoks: typing.List[LexToken] = []

        # The expression in a requires clause must be one of the following:
        #  1) A primary expression
        #  2) A sequence of (1) joined with &&
        #  3) A sequence of (2) joined with ||
        #
        # In terms of validity, this is equivalent to a sequence of primary expressions
        # joined with && and/or ||.
        #
        # In general, a primary expression is one of the following:
        #  1) this
        #  2) a literal
        #  3) an identifier expression
        #  4) a lambda expression
        #  5) a fold expression
        #  6) a requires expression
        #  7) any parenthesized expression
        #
        # For simplicity, we only consider the following primary expressions:
        #  1) parenthesized expressions (which includes fold expressions)
        #  2) requires expressions
        #  3) identifer expressions (possibly qualified, possibly templated)
        while True:
            if tok.type == "(":
                rawtoks.extend(self._consume_balanced_tokens(tok))
                tok = self.lex.token()
            elif tok.type == "requires":
                rawtoks.append(tok)
                for tt in ("(", "{"):
                    tok = self._next_token_must_be(tt)
                    rawtoks.extend(self._consume_balanced_tokens(tok))
                tok = self.lex.token()
            else:
                tok = self._parse_requires_segment(tok, rawtoks)

            if tok.value not in ("&&", "||"):
                break

            rawtoks.append(tok)

            tok = self.lex.token()

        self.lex.return_token(tok)

        return self._create_value(rawtoks)

    def _parse_requires_segment(
        self, tok: LexToken, rawtoks: typing.List[LexToken]
    ) -> LexToken:
        # first token could be a name or ::
        if tok.type == "DBL_COLON":
            rawtoks.append(tok)
            tok = self.lex.token()

        while True:
            # This token has to be a name or some other valid name-like thing
            if tok.value == "decltype":
                rawtoks.append(tok)
                tok = self._next_token_must_be("(")
                rawtoks.extend(self._consume_balanced_tokens(tok))
            elif tok.type == "NAME":
                rawtoks.append(tok)
            else:
                # not sure what I expected, but I didn't find it
                raise self._parse_error(tok)

            tok = self.lex.token()

            # Maybe there's a specialization
            if tok.value == "<":
                rawtoks.extend(self._consume_balanced_tokens(tok))
                tok = self.lex.token()

            # Maybe we keep trying to parse this name
            if tok.type == "DBL_COLON":
                tok = self.lex.token()
                continue

            # Let the caller decide
            return tok

    #
    # Attributes
    #

    _attribute_specifier_seq_start_types = {"DBL_LBRACKET", "alignas"}
    _attribute_start_tokens = {
        "__attribute__",
        "__declspec",
    }
    _attribute_start_tokens |= _attribute_specifier_seq_start_types

    def _consume_attribute(self, tok: LexToken) -> None:
        if tok.type == "__attribute__":
            self._consume_gcc_attribute(tok)
        elif tok.type == "__declspec":
            self._consume_declspec(tok)
        elif tok.type in self._attribute_specifier_seq_start_types:
            self._consume_attribute_specifier_seq(tok)
        else:
            raise CxxParseError("internal error")

    def _consume_gcc_attribute(
        self, tok: typing.Optional[LexToken], doxygen: typing.Optional[str] = None
    ) -> None:
        while True:
            tok1 = self._next_token_must_be("(")
            tok2 = self._next_token_must_be("(")
            self._consume_balanced_tokens(tok1, tok2)

            # Apparently you can have multiple attributes chained together?
            tok = self.lex.token_if("__attribute__")
            if tok is None:
                break

    def _consume_declspec(
        self, tok: LexToken, doxygen: typing.Optional[str] = None
    ) -> None:
        tok = self._next_token_must_be("(")
        self._consume_balanced_tokens(tok)

    def _consume_attribute_specifier_seq(
        self, tok: LexToken, doxygen: typing.Optional[str] = None
    ) -> None:
        """
        attribute_specifier_seq: attribute_specifier
                               | attribute_specifier_seq attribute_specifier

        attribute_specifier: "[[" attribute_list "]]"
                           | alignment_specifier

        alignment_specifier: "alignas" "(" type_id ["..."] ")"
                           | "alignas" "(" alignment_expression ["..."] ")"

        attribute_list: [attribute]
                      | attribute_list "," [attribute]
                      | attribute "..."
                      | attribute_list "," attribute "..."

        attribute: attribute_token [attribute_argument_clause]

        attribute_token: IDENTIFIER
                       | attribute_scoped_token

        attribute_scoped_token: attribute_namespace "::" IDENTIFIER

        attribute_namespace: IDENTIFIER

        attribute_argument_clause: "(" balanced_token_seq ")"

        balanced_token_seq: balanced_token
                          | balanced_token_seq balanced_token

        balanced_token: "(" balanced_token_seq ")"
                      | "[" balanced_token_seq "]"
                      | "{" balanced_token_seq "}"
                      | token
        """

        # TODO: retain the attributes and do something with them
        # attrs = []

        while True:
            if tok.type == "DBL_LBRACKET":
                tokens = self._consume_balanced_tokens(tok)
                # attrs.append(Attribute(tokens))
            elif tok.type == "alignas":
                next_tok = self._next_token_must_be("(")
                tokens = self._consume_balanced_tokens(next_tok)
                # attrs.append(AlignasAttribute(tokens))
            else:
                self.lex.return_token(tok)
                break

            # multiple attributes can be specified
            maybe_tok = self.lex.token_if(*self._attribute_specifier_seq_start_types)
            if maybe_tok is None:
                break

            tok = maybe_tok

        # TODO return attrs

    #
    # Using directive/declaration/typealias
    #

    def _parse_using_directive(self, state: NonClassBlockState) -> None:
        """
        using_directive: [attribute_specifier_seq] "using" "namespace" ["::"] [nested_name_specifier] IDENTIFIER ";"
        """

        names = []
        if self.lex.token_if("DBL_COLON"):
            names.append("")

        while True:
            tok = self._next_token_must_be("NAME")
            names.append(tok.value)

            if not self.lex.token_if("DBL_COLON"):
                break

        if not names:
            raise self._parse_error(None, "NAME")

        self.visitor.on_using_namespace(state, names)

    def _parse_using_declaration(
        self, tok: LexToken, doxygen: typing.Optional[str]
    ) -> None:
        """
        using_declaration: "using" ["typename"] ["::"] nested_name_specifier unqualified_id ";"
                         | "using" "::" unqualified_id ";"

        """
        if tok.type == "typename":
            tok = self.lex.token()

        typename, _ = self._parse_pqname(
            tok, fn_ok=True, compound_ok=True, fund_ok=True
        )
        decl = UsingDecl(typename, self._current_access, doxygen)

        self.visitor.on_using_declaration(self.state, decl)

    def _parse_using_typealias(
        self,
        id_tok: LexToken,
        template: typing.Optional[TemplateDecl],
        doxygen: typing.Optional[str],
    ) -> None:
        """
        alias_declaration: "using" IDENTIFIER "=" type_id ";"
        """

        parsed_type, mods = self._parse_type(None)
        if parsed_type is None:
            raise self._parse_error(None)

        mods.validate(var_ok=False, meth_ok=False, msg="parsing typealias")

        dtype = self._parse_cv_ptr(parsed_type)

        alias = UsingAlias(id_tok.value, dtype, template, self._current_access, doxygen)

        self.visitor.on_using_alias(self.state, alias)

    def _parse_using(
        self,
        tok: LexToken,
        doxygen: typing.Optional[str],
        template: typing.Optional[TemplateDecl] = None,
    ) -> None:
        self.state.location = tok.location

        tok = self._next_token_must_be(
            "NAME", "DBL_COLON", "namespace", "typename", "enum"
        )

        if tok.type == "namespace":
            if template:
                raise CxxParseError(
                    "unexpected using-directive when parsing alias-declaration", tok
                )
            state = self.state
            if not isinstance(state, (NamespaceBlockState, ExternBlockState)):
                raise self._parse_error(tok)

            self._parse_using_directive(state)
        elif tok.type in ("DBL_COLON", "typename") or not self.lex.token_if("="):
            if template:
                raise CxxParseError(
                    "unexpected using-declaration when parsing alias-declaration", tok
                )
            self._parse_using_declaration(tok, doxygen)
        else:
            self._parse_using_typealias(tok, template, doxygen)

        # All using things end with a semicolon
        self._next_token_must_be(";")

    #
    # Enum parsing
    #

    def _parse_enum_decl(
        self,
        typename: PQName,
        tok: LexToken,
        doxygen: typing.Optional[str],
        is_typedef: bool,
        location: Location,
        mods: ParsedTypeModifiers,
    ) -> None:
        """
        opaque_enum_declaration: enum_key [attribute_specifier_seq] IDENTIFIER [enum_base] ";"

        enum_specifier: enum_head "{" [enumerator_list] "}"
                      | enum_head "{" enumerator_list "," "}"

        enum_head: enum_key [attribute_specifier_seq] [IDENTIFIER] [enum_base]
                 | enum_key [attribute_specifier_seq] nested_name_specifier IDENTIFIER [enum_base]

        enum_key: "enum"
                | "enum" "class"
                | "enum" "struct"

        enum_base: ":" type_specifier_seq
        """

        self.state.location = location

        tok_type = tok.type

        # entry: tok is one of _class_enum_stage2
        if tok_type not in (":", "{"):
            raise self._parse_error(tok)

        base = None
        values: typing.List[Enumerator] = []

        if tok_type == ":":
            base, _ = self._parse_pqname(None, fund_ok=True)
            tok = self._next_token_must_be("{", ";")
            tok_type = tok.type

            if tok_type == ";":
                if is_typedef:
                    raise self._parse_error(tok)

                # enum forward declaration with base
                fdecl = ForwardDecl(
                    typename, None, doxygen, base, access=self._current_access
                )
                self.visitor.on_forward_decl(self.state, fdecl)
                return

        values = self._parse_enumerator_list()

        enum = EnumDecl(typename, values, base, doxygen, self._current_access)
        self.visitor.on_enum(self.state, enum)

        # Finish it up
        self._finish_class_or_enum(enum.typename, is_typedef, mods, "enum")

    def _parse_enumerator_list(self) -> typing.List[Enumerator]:
        """
        enumerator_list: enumerator_definition
                       | enumerator_list "," enumerator_definition

        enumerator_definition: enumerator
                             | enumerator "=" constant_expression

        enumerator: IDENTIFIER [attribute-specifier-seq]
        """

        values: typing.List[Enumerator] = []

        while True:
            doxygen = self.lex.get_doxygen()

            name_tok = self._next_token_must_be("}", "NAME")
            if name_tok.value == "}":
                break

            if doxygen is None:
                doxygen = self.lex.get_doxygen_after()

            name = name_tok.value
            value = None

            tok = self._next_token_must_be("}", ",", "=", "DBL_LBRACKET")
            if tok.type == "DBL_LBRACKET":
                self._consume_attribute_specifier_seq(tok)
                tok = self._next_token_must_be("}", ",", "=")

            if tok.type == "=":
                value = self._create_value(self._consume_value_until([], ",", "}"))
                tok = self._next_token_must_be("}", ",")

            values.append(Enumerator(name, value, doxygen))

            if tok.type == "}":
                break

        return values

    #
    # Type parsing
    #

    _base_access_virtual = {"public", "private", "protected", "virtual"}

    def _parse_class_decl_base_clause(
        self, default_access: str
    ) -> typing.List[BaseClass]:
        """
        base_clause: ":" base_specifier_list

        base_specifier_list: base_specifier ["..."]
                           | base_specifier_list "," base_specifier ["..."]

        base_specifier: [attribute_specifier_seq] base_type_specifier
                      | [attribute_specifier_seq] "virtual" [access_specifier] base_type_specifier
                      | [attribute_specifier_seq] access_specifier ["virtual"] base_type_specifier

        base_type_specifier: class_or_decltype

        class_or_decltype: ["::"] [nested_name_specifier] class_name
                         | decltype_specifier
        """

        bases = []

        while True:
            tok = self.lex.token()
            tok_type = tok.type

            # might start with attributes
            if tok.type in self._attribute_specifier_seq_start_types:
                self._consume_attribute_specifier_seq(tok)
                tok = self.lex.token()
                tok_type = tok.type

            access = default_access
            virtual = False
            parameter_pack = False

            # virtual/access specifier comes next
            while tok_type in self._base_access_virtual:
                if tok_type == "virtual":
                    virtual = True
                else:
                    access = tok_type

                tok = self.lex.token()
                tok_type = tok.type

            # Followed by the name
            typename, _ = self._parse_pqname(tok)

            # And potentially a parameter pack
            if self.lex.token_if("ELLIPSIS"):
                parameter_pack = True

            bases.append(BaseClass(access, typename, virtual, parameter_pack))

            if not self.lex.token_if(","):
                break

        return bases

    def _parse_class_decl(
        self,
        typename: PQName,
        tok: LexToken,
        doxygen: typing.Optional[str],
        template: TemplateDeclTypeVar,
        typedef: bool,
        location: Location,
        mods: ParsedTypeModifiers,
    ) -> None:
        """
        class_specifier: class_head "{" [member_specification] "}"

        class_head: class_key [attribute_specifier_seq] class_head_name [class_virt_specifier_seq] [base_clause]
                  | class_key [attribute_specifier_seq] [base_clause]

        class_key: "class"
                 | "struct"
                 | "union"

        class_head_name: [nested_name_specifier] class_name

        class_name: IDENTIFIER
                  | simple_template_id

        class_virt_specifier_seq: class_virt_specifier
                                | class_virt_specifier_seq class_virt_specifier

        class_virt_specifier: "final"
                            | "explicit"
        """

        bases = []
        explicit = False
        final = False

        default_access = "private" if typename.classkey == "class" else "public"

        #: entry: token is one of _class_enum_stage2
        tok_type = tok.type

        while True:
            if tok_type == "final":
                final = True
            elif tok_type == "explicit":
                explicit = True
            else:
                break

            tok = self.lex.token()
            tok_type = tok.type

        if tok_type == ":":
            bases = self._parse_class_decl_base_clause(default_access)
            tok = self.lex.token()
            tok_type = tok.type

        if tok_type != "{":
            raise self._parse_error(tok, "{")

        clsdecl = ClassDecl(
            typename, bases, template, explicit, final, doxygen, self._current_access
        )
        state: ClassBlockState = ClassBlockState(
            self.state, location, clsdecl, default_access, typedef, mods
        )
        self._setup_state(state)

        if self.visitor.on_class_start(state) is False:
            self.visitor = null_visitor

    def _finish_class_decl(self, state: ClassBlockState) -> None:
        self._finish_class_or_enum(
            state.class_decl.typename,
            state.typedef,
            state.mods,
            state.class_decl.classkey,
        )

    def _process_access_specifier(
        self, tok: LexToken, doxygen: typing.Optional[str]
    ) -> None:
        state = self.state
        if not isinstance(state, ClassBlockState):
            raise self._parse_error(tok)

        state._set_access(tok.value)
        self._next_token_must_be(":")

    def _discard_ctor_initializer(self) -> None:
        """
        ctor_initializer: ":" mem_initializer_list

        mem_initializer_list: mem_initializer ["..."]
                            | mem_initializer "," mem_initializer_list ["..."]

        mem_initializer: mem_initializer_id "(" [expression_list] ")"
                       | mem_initializer_id braced_init_list

        mem_initializer_id: class_or_decltype
                          | IDENTIFIER
        """
        self.debug_print("discarding ctor intializer")
        # all of this is discarded.. the challenge is to determine
        # when the initializer ends and the function starts
        get_token = self.lex.token
        while True:
            tok = get_token()
            if tok.type == "DBL_COLON":
                tok = get_token()

            if tok.type == "decltype":
                tok = self._next_token_must_be("(")
                self._consume_balanced_tokens(tok)
                tok = get_token()

            # each initializer is either foo() or foo{}, so look for that
            while True:
                if tok.type not in ("{", "("):
                    tok = get_token()
                    continue

                if tok.type == "{":
                    self._discard_contents("{", "}")
                elif tok.type == "(":
                    self._discard_contents("(", ")")

                tok = get_token()
                break

            # at the end
            if tok.type == "ELLIPSIS":
                tok = get_token()

            if tok.type == ",":
                continue
            elif tok.type == "{":
                # reached the function
                self._discard_contents("{", "}")
                return
            else:
                raise self._parse_error(tok, ",' or '{")

    #
    # Variable parsing
    #

    def _parse_bitfield(self) -> typing.Union[int, Value]:
        # Convert to integer for backwards compat
        tok = self.lex.token_if("INT_CONST_DEC")
        if tok is not None:
            return int(tok.value)

        # But also support weird things too
        return self._create_value(self._consume_value_until([], ";"))

    def _parse_field(
        self,
        mods: ParsedTypeModifiers,
        dtype: DecoratedType,
        pqname: typing.Optional[PQName],
        template: typing.Optional[TemplateDecl],
        doxygen: typing.Optional[str],
        location: Location,
        is_typedef: bool,
    ) -> None:
        state = self.state
        state.location = location
        if isinstance(state, ClassBlockState):
            is_class_block = True
            class_state = state
        else:
            is_class_block = False
        name = None
        bits = None
        default = None

        if not pqname:
            if is_typedef:
                raise CxxParseError("empty name not allowed in typedef")
            if not is_class_block:
                raise CxxParseError("variables must have names")

        else:
            last_segment = pqname.segments[-1]
            if not isinstance(last_segment, NameSpecifier):
                raise CxxParseError(f"invalid name for variable: {pqname}")

            if is_typedef or is_class_block:
                name = last_segment.name
                if len(pqname.segments) > 1:
                    raise CxxParseError(f"name cannot have multiple segments: {pqname}")

        # check for array
        tok = self.lex.token_if("[")
        if tok:
            dtype = self._parse_array_type(tok, dtype)

        # bitfield
        tok = self.lex.token_if(":")
        if tok:
            if is_typedef or not is_class_block:
                raise self._parse_error(tok)

            bits = self._parse_bitfield()

        # default values
        tok = self.lex.token_if("=")
        if tok:
            if is_typedef:
                raise self._parse_error(tok)

            default = self._create_value(self._consume_value_until([], ",", ";"))

        else:
            # default value initializer
            tok = self.lex.token_if("{")
            if tok:
                if is_typedef:
                    raise self._parse_error(tok)

                default = self._create_value(self._consume_balanced_tokens(tok))

        if doxygen is None:
            # try checking after the var
            doxygen = self.lex.get_doxygen_after()

        if is_typedef:
            if not name:
                raise self._parse_error(None)
            typedef = Typedef(dtype, name, self._current_access)
            self.visitor.on_typedef(state, typedef)
        else:
            props = dict.fromkeys(mods.both.keys(), True)
            props.update(dict.fromkeys(mods.vars.keys(), True))

            if is_class_block:
                access = self._current_access
                assert access is not None

                f = Field(
                    name=name,
                    type=dtype,
                    access=access,
                    value=default,
                    bits=bits,
                    doxygen=doxygen,
                    **props,
                )
                self.visitor.on_class_field(class_state, f)
            else:
                assert pqname is not None
                v = Variable(
                    pqname, dtype, default, doxygen=doxygen, template=template, **props
                )
                self.visitor.on_variable(state, v)

    #
    # PQName parsing
    #

    def _parse_pqname_decltype_specifier(self) -> DecltypeSpecifier:
        """
        decltype_specifier: "decltype" "(" expression ")"
        """
        # entry: decltype just consumed
        tok = self._next_token_must_be("(")
        toks = self._consume_balanced_tokens(tok)[1:-1]
        return DecltypeSpecifier([Token(tok.value, tok.type) for tok in toks])

    _name_compound_start = {"struct", "enum", "class", "union"}
    _compound_fundamentals = {
        "unsigned",
        "signed",
        "short",
        "int",
        "long",
        "float",
        "double",
        "char",
    }

    _fundamentals = _compound_fundamentals | {
        "bool",
        "char16_t",
        "char32_t",
        "nullptr_t",
        "wchar_t",
        "void",
    }

    def _parse_pqname_fundamental(self, tok_value: str) -> FundamentalSpecifier:
        fnames = [tok_value]
        _compound_fundamentals = self._compound_fundamentals

        # compound fundamentals can show up in groups in any order
        # -> TODO: const can show up in here
        if tok_value in _compound_fundamentals:
            while True:
                tok = self.lex.token_if_in_set(_compound_fundamentals)
                if not tok:
                    break
                fnames.append(tok.value)

        return FundamentalSpecifier(" ".join(fnames))

    def _parse_pqname_name_operator(self) -> LexTokenList:
        # last tok was 'operator' -- collect until ( is reached
        # - no validation done here, we assume the code is valid

        tok = self.lex.token()
        parts = [tok]

        # special case: operator()
        if tok.value == "(":
            tok = self._next_token_must_be(")")
            parts.append(tok)
            return parts

        self._consume_until(parts, "(")
        return parts

    _pqname_start_tokens = (
        {
            "auto",
            "decltype",
            "NAME",
            "operator",
            "template",
            "typename",
            "DBL_COLON",
            "final",
        }
        | _name_compound_start
        | _fundamentals
    )

    def _parse_pqname_name(
        self, tok_value: str
    ) -> typing.Tuple[NameSpecifier, typing.Optional[str]]:
        name = ""
        specialization = None
        op = None

        # parse out operators as that's generally useful
        if tok_value == "operator":
            op_parts = self._parse_pqname_name_operator()
            op = "".join(o.value for o in op_parts)
            name = f"operator{op}"

        else:
            name = tok_value

        if self.lex.token_if("<"):
            # template specialization
            specialization = self._parse_template_specialization()

        return NameSpecifier(name, specialization), op

    def _parse_pqname(
        self,
        tok: typing.Optional[LexToken],
        *,
        fn_ok: bool = False,
        compound_ok: bool = False,
        fund_ok: bool = False,
    ) -> typing.Tuple[PQName, typing.Optional[str]]:
        """
        Parses a possibly qualified function name or a type name, returns when
        unexpected item encountered (but does not consume it)

        :param fn_ok: Operator functions ok
        :param compound_ok: Compound types ok
        :param fund_ok: Fundamental types ok

        qualified_id: ["::"] nested_name_specifier ["template"] unqualified_id
                    | "::" IDENTIFIER
                    | "::" operator_function_id
                    | "::" literal_operator_id
                    | "::" template_id

        unqualified_id: IDENTIFIER
                      | operator_function_id
                      | conversion_function_id
                      | literal_operator_id
                      | "~" class_name
                      | "~" decltype_specifier
                      | template_id

        conversion_function_id: "operator" conversion_type_id

        conversion_type_id: type_specifier_seq [conversion_declarator]

        conversion_declarator: ptr_operator [conversion_declarator]

        nested_name_specifier: type_name "::"
                             | IDENTIFIER "::"
                             | decltype_specifier "::"
                             | nested_name_specifier IDENTIFIER "::"
                             | nested_name_specifier ["template"] simple_template_id "::"

        type_name: IDENTIFIER
                 | simple_template_id

        """

        classkey = None
        segments: typing.List[PQNameSegment] = []
        op = None
        has_typename = False

        if tok is None:
            tok = self.lex.token()

        if tok.type not in self._pqname_start_tokens:
            raise self._parse_error(tok)

        if tok.type == "auto":
            return PQName([AutoSpecifier()]), None

        _fundamentals = self._fundamentals

        # The pqname could start with class/enum/struct/union
        if tok.value in self._name_compound_start:
            if not compound_ok:
                raise self._parse_error(tok)
            classkey = tok.value
            if classkey == "enum":
                # check for enum class/struct
                tok = self.lex.token_if("class", "struct")
                if tok:
                    classkey = f"{classkey} {tok.value}"

            # Sometimes there's an embedded attribute
            tok = self.lex.token_if(*self._attribute_start_tokens)
            if tok:
                self._consume_attribute(tok)

            tok = self.lex.token_if("NAME", "DBL_COLON")
            if not tok:
                # Handle unnamed class/enum/struct
                self.anon_id += 1
                segments.append(AnonymousName(self.anon_id))
                return PQName(segments, classkey), None
        elif tok.type == "typename":
            has_typename = True
            tok = self.lex.token()
            if tok.type not in self._pqname_start_tokens:
                raise self._parse_error(tok)

        # First section of the name: Add empty segment if starting out with a
        # namespace specifier
        if tok.type == "DBL_COLON":
            segments.append(NameSpecifier(""))
            tok = self._next_token_must_be("NAME", "template", "operator")

        while True:
            tok_value = tok.value

            if tok_value == "decltype":
                segments.append(self._parse_pqname_decltype_specifier())

            # check for fundamentals, consume them
            elif tok_value in _fundamentals:
                if not fund_ok:
                    raise self._parse_error(tok)

                segments.append(self._parse_pqname_fundamental(tok_value))
                # no additional parts after fundamentals
                break

            else:
                if tok_value == "[[":
                    self._consume_attribute_specifier_seq(tok)

                if tok_value == "template":
                    tok = self._next_token_must_be("NAME")
                    tok_value = tok.value

                name, op = self._parse_pqname_name(tok_value)
                segments.append(name)
                if op:
                    if not fn_ok:
                        # encountered unexpected operator
                        raise self._parse_error(tok, "NAME")

                    # nothing after operator
                    break

            # If no more segments, we're done
            if not self.lex.token_if("DBL_COLON"):
                break

            tok = self._next_token_must_be("NAME", "operator", "template", "decltype")

        pqname = PQName(segments, classkey, has_typename)

        self.debug_print(
            "parse_pqname: %s op=%s",
            pqname,
            op,
        )

        return pqname, op

    #
    # Function parsing
    #

    def _parse_parameter(
        self,
        tok: typing.Optional[LexToken],
        cls: typing.Type[PT],
        concept_ok: bool,
        deduce_this_ok: bool,
        end: str = ")",
    ) -> typing.Tuple[PT, typing.Optional[Type]]:
        """
        Parses a single parameter (excluding vararg parameters). Also used
        to parse template non-type parameters

        Returns parameter type, abbreviated template type
        """

        param_name = None
        default = None
        param_pack = False
        parsed_type: typing.Optional[Type]
        at_type: typing.Optional[Type] = None
        extras: typing.Dict[str, typing.Any] = {}

        if deduce_this_ok and self.lex.token_if("this"):
            extras["deduces_this"] = True

        if not tok:
            tok = self.lex.token()

        # placeholder type, skip typename
        if tok.type == "auto":
            at_type = parsed_type = Type(PQName([AutoSpecifier()]))
        else:
            # required typename + decorators
            parsed_type, mods = self._parse_type(tok)
            if parsed_type is None:
                raise self._parse_error(None)

            mods.validate(var_ok=False, meth_ok=False, msg="parsing parameter")

            # Could be a concept
            if concept_ok and self.lex.token_if("auto"):
                at_type = Type(parsed_type.typename)
                parsed_type.typename = PQName([AutoSpecifier()])

        dtype = self._parse_cv_ptr(parsed_type)

        # optional parameter pack
        if self.lex.token_if("ELLIPSIS"):
            param_pack = True

        # name can be surrounded by parens
        tok = self.lex.token_if("(")
        if tok:
            toks = self._consume_balanced_tokens(tok)
            self.lex.return_tokens(toks[1:-1])

        # optional name
        tok = self.lex.token_if("NAME", "final")
        if tok:
            param_name = tok.value

        # optional array parameter
        tok = self.lex.token_if("[")
        if tok:
            dtype = self._parse_array_type(tok, dtype)

        # optional default value
        if self.lex.token_if("="):
            default = self._create_value(self._consume_value_until([], ",", end))

        # abbreviated template pack
        if at_type and self.lex.token_if("ELLIPSIS"):
            param_pack = True

        param = cls(
            type=dtype,
            name=param_name,
            default=default,
            param_pack=param_pack,
            **extras,
        )
        self.debug_print("parameter: %s", param)
        return param, at_type

    def _parse_parameters(
        self, concept_ok: bool, deduce_this_ok: bool
    ) -> typing.Tuple[typing.List[Parameter], bool, typing.List[TemplateParam]]:
        """
        Consumes function parameters and returns them, and vararg if found, and
        promotes abbreviated template parameters to actual template parameters
        if concept_ok is True
        """

        # starting at a (

        # special case: zero parameters
        if self.lex.token_if(")"):
            return [], False, []

        params: typing.List[Parameter] = []
        vararg = False
        at_params: typing.List[TemplateParam] = []

        while True:
            if self.lex.token_if("ELLIPSIS"):
                vararg = True
                self._next_token_must_be(")")
                break

            # Deduce-this only applicable for first function parameter
            param, at_type = self._parse_parameter(
                None,
                Parameter,
                concept_ok,
                deduce_this_ok=deduce_this_ok and len(params) == 0,
            )
            params.append(param)
            if at_type:
                at_params.append(
                    TemplateNonTypeParam(
                        type=at_type,
                        param_idx=len(params) - 1,
                        param_pack=param.param_pack,
                    )
                )

            tok = self._next_token_must_be(",", ")")
            if tok.value == ")":
                break

        # convert fn(void) to fn()
        if self.options.convert_void_to_zero_params and len(params) == 1:
            p0_type = params[0].type
            if (
                isinstance(p0_type, Type)
                and len(p0_type.typename.segments) == 1
                and getattr(p0_type.typename.segments[0], "name", None) == "void"
            ):
                params = []

        return params, vararg, at_params

    _auto_return_typename = PQName([AutoSpecifier()])

    def _parse_trailing_return_type(
        self, return_type: typing.Optional[DecoratedType]
    ) -> DecoratedType:
        # entry is "->"
        if not (
            isinstance(return_type, Type)
            and not return_type.const
            and not return_type.volatile
            and return_type.typename == self._auto_return_typename
        ):
            raise CxxParseError(
                f"function with trailing return type must specify return type of 'auto', not {return_type}"
            )

        parsed_type, mods = self._parse_type(None)
        if parsed_type is None:
            raise self._parse_error(None)

        mods.validate(var_ok=False, meth_ok=False, msg="parsing trailing return type")

        dtype = self._parse_cv_ptr(parsed_type)

        return dtype

    def _parse_fn_end(self, fn: Function) -> None:
        """
        Consumes the various keywords after the parameters in a function
        declaration, and definition if present.
        """

        if self.lex.token_if("throw"):
            tok = self._next_token_must_be("(")
            fn.throw = self._create_value(self._consume_balanced_tokens(tok)[1:-1])

        elif self.lex.token_if("noexcept"):
            toks = []
            otok = self.lex.token_if("(")
            if otok:
                toks = self._consume_balanced_tokens(otok)[1:-1]
            fn.noexcept = self._create_value(toks)
        else:
            rtok = self.lex.token_if("requires")
            if rtok:
                # requires on a function must always be accompanied by a template
                if fn.template is None:
                    raise self._parse_error(rtok)
                fn.raw_requires = self._parse_requires(rtok)

        if self.lex.token_if("ARROW"):
            return_type = self._parse_trailing_return_type(fn.return_type)
            fn.has_trailing_return = True
            fn.return_type = return_type

        if self.lex.token_if("{"):
            self._discard_contents("{", "}")
            fn.has_body = True
        elif self.lex.token_if("="):
            if self.lex.token_if_val("delete"):
                fn.deleted = True
            else:
                raise self._parse_error(None, "expected 'delete")

    def _parse_method_end(self, method: Method) -> None:
        """
        Consumes the various keywords after the parameters in a method
        declaration, and definition if present.
        """

        # various keywords at the end of a method
        get_token = self.lex.token

        while True:
            tok = get_token()
            tok_value = tok.value

            if tok_value in (":", "{"):
                method.has_body = True

                if tok_value == ":":
                    self._discard_ctor_initializer()
                elif tok_value == "{":
                    self._discard_contents("{", "}")

                break

            elif tok_value == "=":
                tok = get_token()
                tok_value = tok.value

                if tok_value == "0":
                    method.pure_virtual = True
                elif tok_value == "delete":
                    method.deleted = True
                elif tok_value == "default":
                    method.default = True
                else:
                    raise self._parse_error(tok, "0/delete/default")

                break
            elif tok_value in ("const", "volatile", "override", "final"):
                setattr(method, tok_value, True)
            elif tok_value in ("&", "&&"):
                method.ref_qualifier = tok_value
            elif tok_value == "->":
                return_type = self._parse_trailing_return_type(method.return_type)
                method.has_trailing_return = True
                method.return_type = return_type
                if self.lex.token_if("{"):
                    self._discard_contents("{", "}")
                    method.has_body = True
                    break
            elif tok_value == "throw":
                tok = self._next_token_must_be("(")
                method.throw = self._create_value(self._consume_balanced_tokens(tok))
            elif tok_value == "noexcept":
                toks = []
                otok = self.lex.token_if("(")
                if otok:
                    toks = self._consume_balanced_tokens(otok)[1:-1]
                method.noexcept = self._create_value(toks)
            elif tok_value == "requires":
                method.raw_requires = self._parse_requires(tok)
            else:
                self.lex.return_token(tok)
                break

    def _parse_function(
        self,
        mods: ParsedTypeModifiers,
        return_type: typing.Optional[DecoratedType],
        pqname: PQName,
        op: typing.Optional[str],
        template: TemplateDeclTypeVar,
        doxygen: typing.Optional[str],
        location: Location,
        constructor: bool,
        destructor: bool,
        is_friend: bool,
        is_typedef: bool,
        msvc_convention: typing.Optional[LexToken],
        is_guide: bool = False,
    ) -> bool:
        """
        Assumes the caller has already consumed the return type and name, this consumes the
        rest of the function including the definition if present

        Returns True if function has a body and it was consumed
        """

        # TODO: explicit (operator int)() in C++20

        # last segment must be NameSpecifier
        if not isinstance(pqname.segments[-1], NameSpecifier):
            raise self._parse_error(None)

        props: typing.Dict
        props = dict.fromkeys(mods.both.keys(), True)
        if msvc_convention:
            props["msvc_convention"] = msvc_convention.value

        state = self.state
        state.location = location
        is_class_block = isinstance(state, ClassBlockState)

        params, vararg, at_params = self._parse_parameters(
            True, deduce_this_ok=is_class_block
        )

        # Promote abbreviated template parameters
        if at_params:
            if template is None:
                template = TemplateDecl(at_params)
            elif isinstance(template, TemplateDecl):
                template.params.extend(at_params)
            else:
                template[-1].params.extend(at_params)

        # A method outside of a class has multiple name segments
        multiple_name_segments = len(pqname.segments) > 1

        if (is_class_block or multiple_name_segments) and not is_typedef:
            props.update(dict.fromkeys(mods.meths.keys(), True))

            method = Method(
                return_type,
                pqname,
                params,
                vararg,
                doxygen=doxygen,
                constructor=constructor,
                destructor=destructor,
                template=template,
                operator=op,
                access=self._current_access,
                **props,  # type: ignore
            )

            self._parse_method_end(method)

            if is_class_block:
                assert isinstance(state, ClassBlockState)
                if is_friend:
                    friend = FriendDecl(fn=method)
                    self.visitor.on_class_friend(state, friend)
                else:
                    # method must not have multiple segments except for operator
                    if len(pqname.segments) > 1:
                        if getattr(pqname.segments[0], "name", None) != "operator":
                            raise self._parse_error(None)

                    self.visitor.on_class_method(state, method)
            else:
                assert isinstance(state, (ExternBlockState, NamespaceBlockState))
                # only template specializations can be declared without a body here
                if not method.has_body and not method.template:
                    raise self._parse_error(None, expected="Method body")
                self.visitor.on_method_impl(state, method)

            return method.has_body or method.has_trailing_return
        elif is_guide:
            assert isinstance(state, (ExternBlockState, NamespaceBlockState))
            if not self.lex.token_if("ARROW"):
                raise self._parse_error(None, expected="Trailing return type")
            return_type = self._parse_trailing_return_type(
                Type(PQName([AutoSpecifier()]))
            )
            guide = DeductionGuide(
                return_type,
                name=pqname,
                parameters=params,
                doxygen=doxygen,
            )
            self.visitor.on_deduction_guide(state, guide)
            return False
        else:
            assert return_type is not None
            fn = Function(
                return_type,
                pqname,
                params,
                vararg,
                doxygen=doxygen,
                template=template,
                operator=op,
                **props,
            )
            self._parse_fn_end(fn)

            if is_typedef:
                if len(pqname.segments) != 1:
                    raise CxxParseError(
                        "typedef name may not be a nested-name-specifier"
                    )
                name: typing.Optional[str] = getattr(pqname.segments[0], "name", None)
                if not name:
                    raise CxxParseError("typedef function must have a name")

                if fn.constexpr:
                    raise CxxParseError("typedef function may not be constexpr")
                if fn.extern:
                    raise CxxParseError("typedef function may not be extern")
                if fn.static:
                    raise CxxParseError("typedef function may not be static")
                if fn.inline:
                    raise CxxParseError("typedef function may not be inline")
                if fn.has_body:
                    raise CxxParseError("typedef may not be a function definition")
                if fn.template:
                    raise CxxParseError("typedef function may not have a template")

                return_type = fn.return_type
                if return_type is None:
                    raise CxxParseError("typedef function must have return type")

                fntype = FunctionType(
                    return_type,
                    fn.parameters,
                    fn.vararg,
                    fn.has_trailing_return,
                    noexcept=fn.noexcept,
                    msvc_convention=fn.msvc_convention,
                )

                typedef = Typedef(fntype, name, self._current_access)
                self.visitor.on_typedef(state, typedef)
                return False
            else:
                if not isinstance(state, (ExternBlockState, NamespaceBlockState)):
                    raise CxxParseError("internal error")

                self.visitor.on_function(state, fn)
                return fn.has_body or fn.has_trailing_return

    #
    # Decorated type parsing
    #

    def _parse_array_type(self, tok: LexToken, dtype: DecoratedType) -> Array:
        assert tok.type == "["

        if isinstance(dtype, (Reference, MoveReference)):
            raise CxxParseError("arrays of references are illegal", tok)

        toks = self._consume_balanced_tokens(tok)
        otok = self.lex.token_if("[")
        if otok:
            # recurses because array types are right to left
            dtype = self._parse_array_type(otok, dtype)

        toks = toks[1:-1]
        size = None

        if toks:
            size = self._create_value(toks)

        return Array(dtype, size)

    def _parse_cv_ptr(
        self,
        dtype: DecoratedType,
    ) -> DecoratedType:
        dtype_or_fn = self._parse_cv_ptr_or_fn(dtype)
        if isinstance(dtype_or_fn, FunctionType):
            raise CxxParseError("unexpected function type")
        return dtype_or_fn

    def _parse_cv_ptr_or_fn(
        self,
        dtype: typing.Union[
            Array, Pointer, MoveReference, Reference, Type, FunctionType
        ],
        nonptr_fn: bool = False,
    ) -> typing.Union[Array, Pointer, MoveReference, Reference, Type, FunctionType]:
        # nonptr_fn is for parsing function types directly in template specialization

        while True:
            tok = self.lex.token_if(
                "*", "const", "volatile", "__restrict__", "restrict", "("
            )
            if not tok:
                break

            if tok.type == "*":
                if isinstance(dtype, (Reference, MoveReference)):
                    raise self._parse_error(tok)
                dtype = Pointer(dtype)
            elif tok.type == "const":
                if not isinstance(dtype, (Pointer, Type)):
                    raise self._parse_error(tok)
                dtype.const = True
            elif tok.type == "volatile":
                if not isinstance(dtype, (Pointer, Type)):
                    raise self._parse_error(tok)
                dtype.volatile = True
            elif tok.type in ("__restrict__", "restrict"):
                if not isinstance(dtype, (Pointer, Reference)):
                    raise self._parse_error(tok)
                dtype.restrict = True
            elif nonptr_fn:
                # remove any inner grouping parens
                while True:
                    gtok = self.lex.token_if("(")
                    if not gtok:
                        break

                    toks = self._consume_balanced_tokens(gtok)
                    self.lex.return_tokens(toks[1:-1])

                fn_params, vararg, _ = self._parse_parameters(False, False)

                assert not isinstance(dtype, FunctionType)
                dtype = dtype_fn = FunctionType(dtype, fn_params, vararg)
                if self.lex.token_if("ARROW"):
                    return_type = self._parse_trailing_return_type(dtype_fn.return_type)
                    dtype_fn.has_trailing_return = True
                    dtype_fn.return_type = return_type

            else:
                msvc_convention = None
                msvc_convention_tok = self.lex.token_if_val(*self._msvc_conventions)
                if msvc_convention_tok:
                    msvc_convention = msvc_convention_tok.value

                # Check to see if this is a grouping paren or something else
                if not self.lex.token_peek_if("*", "&"):
                    self.lex.return_token(tok)
                    break

                # this is a grouping paren, so consume it
                toks = self._consume_balanced_tokens(tok)

                # Now check to see if we have either an array or a function pointer
                aptok = self.lex.token_if("[", "(")
                if aptok:
                    if aptok.type == "[":
                        assert not isinstance(dtype, FunctionType)
                        dtype = self._parse_array_type(aptok, dtype)
                    elif aptok.type == "(":
                        fn_params, vararg, _ = self._parse_parameters(False, False)
                        # the type we already have is the return type of the function pointer

                        assert not isinstance(dtype, FunctionType)

                        dtype = FunctionType(
                            dtype, fn_params, vararg, msvc_convention=msvc_convention
                        )

                        # the inner tokens must either be a * or a pqname that ends
                        # with ::* (member function pointer)
                        # ... TODO member function pointer :(

                # return the inner toks and recurse
                # -> this could return some weird results for invalid code, but
                #    we don't support that anyways so it's fine?
                self.lex.return_tokens(toks[1:-1])
                dtype = self._parse_cv_ptr_or_fn(dtype, nonptr_fn)
                break

        tok = self.lex.token_if("&", "DBL_AMP")
        if tok:
            assert not isinstance(dtype, (Reference, MoveReference))

            if tok.type == "&":
                dtype = Reference(dtype)
            else:
                dtype = MoveReference(dtype)

            # peek at the next token and see if it's a paren. If so, it might
            # be a nasty function pointer
            if self.lex.token_peek_if("(", "__restrict__", "restrict"):
                dtype = self._parse_cv_ptr_or_fn(dtype, nonptr_fn)

        return dtype

    # Applies to variables and return values
    _type_kwd_both = {"const", "constexpr", "extern", "inline", "static"}

    # Only found on methods
    _type_kwd_meth = {"explicit", "virtual"}

    _parse_type_ptr_ref_paren = {"*", "&", "DBL_AMP", "("}

    def _parse_type(
        self,
        tok: typing.Optional[LexToken],
        operator_ok: bool = False,
    ) -> typing.Tuple[typing.Optional[Type], ParsedTypeModifiers]:
        """
        This parses a typename and stops parsing when it hits something
        that it doesn't understand. The caller uses the results to figure
        out what got parsed

        This only parses the base type, does not parse pointers, references,
        or additional const/volatile qualifiers

        The returned type will only be None if operator_ok is True and an
        operator is encountered.
        """

        const = False
        volatile = False

        # Modifiers that apply to the variable/function
        # -> key is name of modifier, value is a token so that we can emit an
        #    appropriate error

        vars: typing.Dict[str, LexToken] = {}  # only found on variables
        both: typing.Dict[str, LexToken] = {}  # found on either
        meths: typing.Dict[str, LexToken] = {}  # only found on methods

        get_token = self.lex.token

        if not tok:
            tok = get_token()

        pqname: typing.Optional[PQName] = None
        pqname_optional = False

        _pqname_start_tokens = self._pqname_start_tokens
        _attribute_start = self._attribute_start_tokens

        # This loop parses until it finds two pqname or ptr/ref
        while True:
            tok_type = tok.type
            if tok_type in _pqname_start_tokens:
                if pqname is not None:
                    # found second set of names, done here
                    break
                if operator_ok and tok_type == "operator":
                    # special case: conversion operators such as operator bool
                    pqname_optional = True
                    break
                pqname, _ = self._parse_pqname(
                    tok, compound_ok=True, fn_ok=False, fund_ok=True
                )
            elif tok_type in self._parse_type_ptr_ref_paren:
                if pqname is None:
                    raise self._parse_error(tok)
                break
            elif tok_type == "const":
                const = True
            elif tok_type in self._type_kwd_both:
                if tok_type == "extern":
                    # TODO: store linkage
                    self.lex.token_if("STRING_LITERAL")
                both[tok_type] = tok
            elif tok_type in self._type_kwd_meth:
                meths[tok_type] = tok
            elif tok_type == "mutable":
                vars["mutable"] = tok
            elif tok_type == "volatile":
                volatile = True
            elif tok_type in _attribute_start:
                self._consume_attribute(tok)
            elif tok_type in ("__inline", "__forceinline"):
                both["inline"] = tok
            else:
                break

            tok = get_token()

        if pqname is None:
            if not pqname_optional:
                raise self._parse_error(tok)
            parsed_type = None
        else:
            # Construct a type from the parsed name
            parsed_type = Type(pqname, const, volatile)

        self.lex.return_token(tok)

        # Always return the modifiers
        mods = ParsedTypeModifiers(vars, both, meths)
        return parsed_type, mods

    def _parse_decl(
        self,
        parsed_type: Type,
        mods: ParsedTypeModifiers,
        location: Location,
        doxygen: typing.Optional[str],
        template: TemplateDeclTypeVar,
        is_typedef: bool,
        is_friend: bool,
    ) -> bool:
        toks = []

        # On entry we only have the base type, decorate it
        dtype: typing.Optional[DecoratedType]
        dtype = self._parse_cv_ptr(parsed_type)

        state = self.state

        pqname = None
        constructor = False
        destructor = False
        op = None
        msvc_convention = None
        is_guide = False

        # If we have a leading (, that's either an obnoxious grouping
        # paren or it's a constructor
        tok = self.lex.token_if("(")
        if tok:
            dsegments: typing.List[PQNameSegment] = []
            if isinstance(dtype, Type):
                dsegments = dtype.typename.segments

            # Check to see if this is a constructor/destructor by matching
            # the method name to the class name
            is_class_block = isinstance(state, ClassBlockState)
            if (is_class_block or len(dsegments) > 1) and isinstance(dtype, Type):
                if not is_class_block:
                    # must be an instance of a class
                    cls_name = getattr(dsegments[-2], "name", None)

                elif not is_friend:
                    assert isinstance(state, ClassBlockState)

                    # class name to match against is this class
                    cls_name = getattr(
                        state.class_decl.typename.segments[-1], "name", None
                    )
                else:
                    # class name to match against is the other class
                    if len(dsegments) >= 2:
                        cls_name = getattr(dsegments[-2], "name", None)
                    else:
                        cls_name = None

                ret_name = getattr(dsegments[-1], "name", None)

                if cls_name:
                    if cls_name == ret_name:
                        pqname = dtype.typename
                        dtype = None
                        constructor = True
                        self.lex.return_token(tok)
                    elif f"~{cls_name}" == ret_name:
                        pqname = dtype.typename
                        dtype = None
                        destructor = True
                        self.lex.return_token(tok)

            if dtype:
                # if it's not a constructor/destructor, it could be a
                # grouping paren like "void (name(int x));"
                toks = self._consume_balanced_tokens(tok)

                # check to see if the next token is an arrow, and thus a trailing return
                if self.lex.token_peek_if("ARROW"):
                    self.lex.return_tokens(toks)
                    # the leading name of the class/ctor has been parsed as a type before the parens
                    pqname = parsed_type.typename
                    is_guide = True
                else:
                    # .. not sure what it's grouping, so put it back?
                    self.lex.return_tokens(toks[1:-1])

        if dtype:
            msvc_convention = self.lex.token_if_val(*self._msvc_conventions)

            tok = self.lex.token_if_in_set(self._pqname_start_tokens)
            if tok:
                pqname, op = self._parse_pqname(tok, fn_ok=True)

        # TODO: "type fn(x);" is ambiguous here. Because this is a header
        # parser, we assume it's a function, not a variable declaration
        # calling a constructor

        # if ( then it's a function/method
        if self.lex.token_if("("):
            if not pqname:
                raise self._parse_error(None)

            return self._parse_function(
                mods,
                dtype,
                pqname,
                op,
                template,
                doxygen,
                location,
                constructor,
                destructor,
                is_friend,
                is_typedef,
                msvc_convention,
                is_guide,
            )
        elif msvc_convention:
            raise self._parse_error(msvc_convention)

        # anything else is a field/variable
        if is_friend:
            # "friend Foo;"
            if not self.lex.token_if(";"):
                raise self._parse_error(None)

            assert isinstance(state, ClassBlockState)

            fwd = ForwardDecl(
                parsed_type.typename,
                template,
                doxygen,
                access=self._current_access,
            )
            friend = FriendDecl(cls=fwd)
            state.location = location
            self.visitor.on_class_friend(state, friend)
            return True
        if op:
            raise self._parse_error(None)

        if not dtype:
            raise CxxParseError("appear to be parsing a field without a type")

        if isinstance(template, list):
            raise CxxParseError("multiple template declarations on a field")

        self._parse_field(mods, dtype, pqname, template, doxygen, location, is_typedef)
        return False

    def _parse_operator_conversion(
        self,
        mods: ParsedTypeModifiers,
        location: Location,
        doxygen: typing.Optional[str],
        template: TemplateDeclTypeVar,
        is_typedef: bool,
        is_friend: bool,
    ) -> None:
        tok = self._next_token_must_be("operator")

        if is_typedef:
            raise self._parse_error(tok, "operator not permitted in typedef")

        # next piece must be the conversion type
        ctype, cmods = self._parse_type(None)
        if ctype is None:
            raise self._parse_error(None)

        cmods.validate(var_ok=False, meth_ok=False, msg="parsing conversion operator")

        # Check for any cv decorations for the type
        rtype = self._parse_cv_ptr(ctype)

        # then this must be a method
        self._next_token_must_be("(")

        # make our own pqname/op here
        segments: typing.List[PQNameSegment] = [NameSpecifier("operator")]
        pqname = PQName(segments)
        op = "conversion"

        if self._parse_function(
            mods,
            rtype,
            pqname,
            op,
            template,
            doxygen,
            location,
            False,
            False,
            is_friend,
            False,
            None,
        ):
            # has function body and handled it
            return

        # if this is just a declaration, next token should be ;
        self._next_token_must_be(";")

    _class_enum_stage2 = {":", "final", "explicit", "{"}

    def _parse_declarations(
        self,
        tok: LexToken,
        doxygen: typing.Optional[str],
        template: TemplateDeclTypeVar = None,
        is_typedef: bool = False,
        is_friend: bool = False,
    ) -> None:
        """
        Parses a sequence of declarations
        """

        # On entry to this function, we don't have enough context to know
        # what we're going to find, so keep trying to deduce it

        location = tok.location

        # Almost always starts out with some kind of type name or a modifier
        parsed_type, mods = self._parse_type(tok, operator_ok=True)

        # Check to see if this might be a class/enum declaration
        if (
            parsed_type is not None
            and parsed_type.typename.classkey
            and self._maybe_parse_class_enum_decl(
                parsed_type, mods, doxygen, template, is_typedef, is_friend, location
            )
        ):
            return

        # Check for an abbreviated template return type, promote it
        if not is_typedef and parsed_type is not None and self.lex.token_if_val("auto"):
            abv_ret_tmpl = TemplateNonTypeParam(type=parsed_type, param_idx=-1)
            if template is None:
                template = TemplateDecl(params=[abv_ret_tmpl])
            elif isinstance(template, TemplateDecl):
                template.params.append(abv_ret_tmpl)
            else:
                template[-1].params.append(abv_ret_tmpl)

        var_ok = True

        if is_typedef:
            var_ok = False
            meth_ok = False
            msg = "parsing typedef"
        elif isinstance(self.state, ClassBlockState):
            meth_ok = True
            msg = "parsing declaration in class"
        else:
            meth_ok = False
            msg = "parsing declaration"

        mods.validate(var_ok=var_ok, meth_ok=meth_ok, msg=msg)

        if parsed_type is None:
            # this means an operator was encountered, deal with the special case
            self._parse_operator_conversion(
                mods, location, doxygen, template, is_typedef, is_friend
            )
            return

        # Ok, dealing with a variable or function/method
        while True:
            if self._parse_decl(
                parsed_type, mods, location, doxygen, template, is_typedef, is_friend
            ):
                # if it returns True then it handled the end of the statement
                break

            # Sometimes attributes come after a function
            atok = self.lex.token_if("__attribute__")
            if atok:
                self._consume_gcc_attribute(atok)

            # Unset the doxygen, location
            doxygen = None

            # Check for multiple declarations
            tok = self._next_token_must_be(",", ";")
            location = tok.location
            if tok.type == ";":
                break

    def _maybe_parse_class_enum_decl(
        self,
        parsed_type: Type,
        mods: ParsedTypeModifiers,
        doxygen: typing.Optional[str],
        template: TemplateDeclTypeVar,
        is_typedef: bool,
        is_friend: bool,
        location: Location,
    ) -> bool:
        # check for forward declaration or friend declaration
        if self.lex.token_if(";"):
            if is_typedef:
                raise self._parse_error(None)

            classkey = parsed_type.typename.classkey
            mods.validate(
                var_ok=False,
                meth_ok=False,
                msg=f"parsing {classkey} forward declaration",
            )

            # enum cannot be forward declared, but "enum class" can
            # -> but `friend enum X` is fine
            if not classkey:
                raise self._parse_error(None)
            elif classkey == "enum" and not is_friend:
                raise self._parse_error(None)
            elif template and classkey.startswith("enum"):
                # enum class cannot have a template
                raise self._parse_error(None)

            fdecl = ForwardDecl(
                parsed_type.typename, template, doxygen, access=self._current_access
            )
            state = self.state
            state.location = location
            if is_friend:
                assert isinstance(state, ClassBlockState)
                friend = FriendDecl(cls=fdecl)
                self.visitor.on_class_friend(state, friend)
            else:
                self.visitor.on_forward_decl(state, fdecl)
            return True

        tok = self.lex.token_if_in_set(self._class_enum_stage2)
        if tok:
            classkey = parsed_type.typename.classkey
            # var is ok because it could be carried on to any variables
            mods.validate(
                var_ok=not is_typedef,
                meth_ok=False,
                msg=f"parsing {classkey} declaration",
            )
            typename = parsed_type.typename

            if is_friend:
                # friend declaration doesn't have extra context
                raise self._parse_error(tok)

            if typename.classkey in ("class", "struct", "union"):
                self._parse_class_decl(
                    typename, tok, doxygen, template, is_typedef, location, mods
                )
            else:
                if template:
                    # enum cannot have a template
                    raise self._parse_error(tok)
                self._parse_enum_decl(
                    typename, tok, doxygen, is_typedef, location, mods
                )

            return True

        # Otherwise this must be a variable or function
        return False

    def _finish_class_or_enum(
        self,
        name: PQName,
        is_typedef: bool,
        mods: ParsedTypeModifiers,
        classkey: typing.Optional[str],
    ) -> None:
        parsed_type = Type(name)

        tok = self.lex.token_if("__attribute__")
        if tok:
            self._consume_gcc_attribute(tok)

        if not is_typedef and self.lex.token_if(";"):
            # if parent scope is a class, add the anonymous
            # union or struct to the parent fields
            if isinstance(self.state, ClassBlockState):
                class_state = self.state

                access = self._current_access
                assert access is not None
                if (
                    classkey is not None
                    and classkey in ["union", "struct"]
                    and isinstance(name.segments[-1], AnonymousName)
                ):
                    f = Field(
                        type=Type(name),
                        access=access,
                    )
                    self.visitor.on_class_field(class_state, f)
            return

        while True:
            location = self.lex.current_location()
            if self._parse_decl(
                parsed_type, mods, location, None, None, is_typedef, False
            ):
                # if it returns True then it handled the end of the statement
                break

            # Check for multiple declarations
            tok = self._next_token_must_be(",", ";")
            if tok.type == ";":
                break
