import re
import typing
import sys

from ._ply import lex
from ._ply.lex import TOKEN

from .errors import CxxParseError


class LexError(CxxParseError):
    pass


if sys.version_info >= (3, 8):
    from typing import Protocol
else:
    Protocol = object

_line_re = re.compile(r'^\#[\t ]*(line)? (\d+) "(.*)"')
_multicomment_re = re.compile("\n[\\s]+\\*")


class Location(typing.NamedTuple):
    """
    Location that something was found at, takes #line directives into account
    """

    filename: str
    lineno: int


class LexToken(Protocol):
    """
    Token as emitted by PLY and modified by our lexer
    """

    #: Lexer type for this token
    type: str

    #: Raw value for this token
    value: str

    lineno: int
    lexpos: int

    #: Location token was found at
    location: Location

    #: private
    lexer: lex.Lexer
    lexmatch: "re.Match"


PhonyEnding: LexToken = lex.LexToken()  # type: ignore
PhonyEnding.type = "PLACEHOLDER"
PhonyEnding.value = ""
PhonyEnding.lineno = 0
PhonyEnding.lexpos = 0


class PlyLexer:
    """
    This lexer is a combination of pieces from the PLY lexers that CppHeaderParser
    and pycparser have.

    This tokenizes the input into tokens. The other lexer classes do more complex
    things with the tokens.
    """

    keywords = {
        "__attribute__",
        "alignas",
        "alignof",
        "asm",
        "auto",
        "bool",
        "break",
        "case",
        "catch",
        "char",
        "char8_t",
        "char16_t",
        "char32_t",
        "class",
        "concept",
        "const",
        "constexpr",
        "const_cast",
        "continue",
        "decltype",
        "__declspec",
        "default",
        "delete",
        "do",
        "double",
        "dynamic_cast",
        "else",
        "enum",
        "explicit",
        "export",
        "extern",
        "false",
        "final",
        "float",
        "for",
        "__forceinline",
        "friend",
        "goto",
        "if",
        "__inline",
        "inline",
        "int",
        "long",
        "mutable",
        "namespace",
        "new",
        "noexcept",
        "nullptr",
        "operator",
        "private",
        "protected",
        "public",
        "register",
        "reinterpret_cast",
        "requires",
        "__restrict__",
        "restrict",
        "return",
        "short",
        "signed",
        "sizeof",
        "static",
        "static_assert",
        "static_cast",
        "struct",
        "switch",
        "template",
        "this",
        "thread_local",
        "throw",
        "true",
        "try",
        "typedef",
        "typeid",
        "typename",
        "union",
        "unsigned",
        "using",
        "virtual",
        "void",
        "volatile",
        "wchar_t",
        "while",
    }

    tokens = [
        # constants
        "FLOAT_CONST",
        "HEX_FLOAT_CONST",
        "INT_CONST_HEX",
        "INT_CONST_BIN",
        "INT_CONST_OCT",
        "INT_CONST_DEC",
        "INT_CONST_CHAR",
        "CHAR_CONST",
        "WCHAR_CONST",
        "U8CHAR_CONST",
        "U16CHAR_CONST",
        "U32CHAR_CONST",
        # String literals
        "STRING_LITERAL",
        "WSTRING_LITERAL",
        "U8STRING_LITERAL",
        "U16STRING_LITERAL",
        "U32STRING_LITERAL",
        #
        "NAME",
        # Comments
        "COMMENT_SINGLELINE",
        "COMMENT_MULTILINE",
        "PRAGMA_DIRECTIVE",
        "INCLUDE_DIRECTIVE",
        "PP_DIRECTIVE",
        # misc
        "DIVIDE",
        "NEWLINE",
        "WHITESPACE",
        "ELLIPSIS",
        "DBL_LBRACKET",
        "DBL_RBRACKET",
        "DBL_COLON",
        "DBL_AMP",
        "DBL_PIPE",
        "ARROW",
        "SHIFT_LEFT",
    ] + list(keywords)

    literals = [
        "<",
        ">",
        "(",
        ")",
        "{",
        "}",
        "[",
        "]",
        ";",
        ":",
        ",",
        "\\",
        "|",
        "%",
        "^",
        "!",
        "*",
        "-",
        "+",
        "&",
        "=",
        "'",
        ".",
        "?",
    ]

    #
    # Regexes for use in tokens (taken from pycparser)
    #

    hex_prefix = "0[xX]"
    hex_digits = "[0-9a-fA-F']+"
    bin_prefix = "0[bB]"
    bin_digits = "[01']+"

    # integer constants (K&R2: A.2.5.1)
    integer_suffix_opt = (
        r"(([uU]ll)|([uU]LL)|(ll[uU]?)|(LL[uU]?)|([uU][lL])|([lL][uU]?)|[uU])?"
    )
    decimal_constant = (
        "(0" + integer_suffix_opt + ")|([1-9][0-9']*" + integer_suffix_opt + ")"
    )
    octal_constant = "0[0-7']*" + integer_suffix_opt
    hex_constant = hex_prefix + hex_digits + integer_suffix_opt
    bin_constant = bin_prefix + bin_digits + integer_suffix_opt

    bad_octal_constant = "0[0-7]*[89]"

    # character constants (K&R2: A.2.5.2)
    # Note: a-zA-Z and '.-~^_!=&;,' are allowed as escape chars to support #line
    # directives with Windows paths as filenames (..\..\dir\file)
    # For the same reason, decimal_escape allows all digit sequences. We want to
    # parse all correct code, even if it means to sometimes parse incorrect
    # code.
    #
    # The original regexes were taken verbatim from the C syntax definition,
    # and were later modified to avoid worst-case exponential running time.
    #
    #   simple_escape = r"""([a-zA-Z._~!=&\^\-\\?'"])"""
    #   decimal_escape = r"""(\d+)"""
    #   hex_escape = r"""(x[0-9a-fA-F]+)"""
    #   bad_escape = r"""([\\][^a-zA-Z._~^!=&\^\-\\?'"x0-7])"""
    #
    # The following modifications were made to avoid the ambiguity that allowed backtracking:
    # (https://github.com/eliben/pycparser/issues/61)
    #
    # - \x was removed from simple_escape, unless it was not followed by a hex digit, to avoid ambiguity with hex_escape.
    # - hex_escape allows one or more hex characters, but requires that the next character(if any) is not hex
    # - decimal_escape allows one or more decimal characters, but requires that the next character(if any) is not a decimal
    # - bad_escape does not allow any decimals (8-9), to avoid conflicting with the permissive decimal_escape.
    #
    # Without this change, python's `re` module would recursively try parsing each ambiguous escape sequence in multiple ways.
    # e.g. `\123` could be parsed as `\1`+`23`, `\12`+`3`, and `\123`.

    simple_escape = r"""([a-wyzA-Z._~!=&\^\-\\?'"]|x(?![0-9a-fA-F]))"""
    decimal_escape = r"""(\d+)(?!\d)"""
    hex_escape = r"""(x[0-9a-fA-F]+)(?![0-9a-fA-F])"""
    bad_escape = r"""([\\][^a-zA-Z._~^!=&\^\-\\?'"x0-9])"""

    escape_sequence = (
        r"""(\\(""" + simple_escape + "|" + decimal_escape + "|" + hex_escape + "))"
    )

    # This complicated regex with lookahead might be slow for strings, so because all of the valid escapes (including \x) allowed
    # 0 or more non-escaped characters after the first character, simple_escape+decimal_escape+hex_escape got simplified to

    escape_sequence_start_in_string = r"""(\\[0-9a-zA-Z._~!=&\^\-\\?'"])"""

    cconst_char = r"""([^'\\\n]|""" + escape_sequence + ")"
    char_const = "'" + cconst_char + "'"
    wchar_const = "L" + char_const
    u8char_const = "u8" + char_const
    u16char_const = "u" + char_const
    u32char_const = "U" + char_const
    multicharacter_constant = "'" + cconst_char + "{2,4}'"
    unmatched_quote = "('" + cconst_char + "*\\n)|('" + cconst_char + "*$)"
    bad_char_const = (
        r"""('"""
        + cconst_char
        + """[^'\n]+')|('')|('"""
        + bad_escape
        + r"""[^'\n]*')"""
    )

    # string literals (K&R2: A.2.6)
    string_char = r"""([^"\\\n]|""" + escape_sequence_start_in_string + ")"
    string_literal = '"' + string_char + '*"'
    wstring_literal = "L" + string_literal
    u8string_literal = "u8" + string_literal
    u16string_literal = "u" + string_literal
    u32string_literal = "U" + string_literal
    bad_string_literal = '"' + string_char + "*" + bad_escape + string_char + '*"'

    # floating constants (K&R2: A.2.5.3)
    exponent_part = r"""([eE][-+]?[0-9]+)"""
    fractional_constant = r"""([0-9]*\.[0-9]+)|([0-9]+\.)"""
    floating_constant = (
        "(((("
        + fractional_constant
        + ")"
        + exponent_part
        + "?)|([0-9]+"
        + exponent_part
        + "))[FfLl]?)"
    )
    binary_exponent_part = r"""([pP][+-]?[0-9]+)"""
    hex_fractional_constant = (
        "(((" + hex_digits + r""")?\.""" + hex_digits + ")|(" + hex_digits + r"""\.))"""
    )
    hex_floating_constant = (
        "("
        + hex_prefix
        + "("
        + hex_digits
        + "|"
        + hex_fractional_constant
        + ")"
        + binary_exponent_part
        + "[FfLl]?)"
    )

    t_WHITESPACE = "[ \t]+"
    t_ignore = "\r"

    # The following floating and integer constants are defined as
    # functions to impose a strict order (otherwise, decimal
    # is placed before the others because its regex is longer,
    # and this is bad)
    #
    @TOKEN(floating_constant)
    def t_FLOAT_CONST(self, t: LexToken) -> LexToken:
        return t

    @TOKEN(hex_floating_constant)
    def t_HEX_FLOAT_CONST(self, t: LexToken) -> LexToken:
        return t

    @TOKEN(hex_constant)
    def t_INT_CONST_HEX(self, t: LexToken) -> LexToken:
        return t

    @TOKEN(bin_constant)
    def t_INT_CONST_BIN(self, t: LexToken) -> LexToken:
        return t

    @TOKEN(bad_octal_constant)
    def t_BAD_CONST_OCT(self, t: LexToken) -> None:
        msg = "Invalid octal constant"
        self._error(msg, t)

    @TOKEN(octal_constant)
    def t_INT_CONST_OCT(self, t: LexToken) -> LexToken:
        return t

    @TOKEN(decimal_constant)
    def t_INT_CONST_DEC(self, t: LexToken) -> LexToken:
        return t

    # Must come before bad_char_const, to prevent it from
    # catching valid char constants as invalid
    #
    @TOKEN(multicharacter_constant)
    def t_INT_CONST_CHAR(self, t: LexToken) -> LexToken:
        return t

    @TOKEN(char_const)
    def t_CHAR_CONST(self, t: LexToken) -> LexToken:
        return t

    @TOKEN(wchar_const)
    def t_WCHAR_CONST(self, t: LexToken) -> LexToken:
        return t

    @TOKEN(u8char_const)
    def t_U8CHAR_CONST(self, t: LexToken) -> LexToken:
        return t

    @TOKEN(u16char_const)
    def t_U16CHAR_CONST(self, t: LexToken) -> LexToken:
        return t

    @TOKEN(u32char_const)
    def t_U32CHAR_CONST(self, t: LexToken) -> LexToken:
        return t

    @TOKEN(unmatched_quote)
    def t_UNMATCHED_QUOTE(self, t: LexToken) -> None:
        msg = "Unmatched '"
        self._error(msg, t)

    @TOKEN(bad_char_const)
    def t_BAD_CHAR_CONST(self, t: LexToken) -> None:
        msg = "Invalid char constant %s" % t.value
        self._error(msg, t)

    @TOKEN(wstring_literal)
    def t_WSTRING_LITERAL(self, t: LexToken) -> LexToken:
        return t

    @TOKEN(u8string_literal)
    def t_U8STRING_LITERAL(self, t: LexToken) -> LexToken:
        return t

    @TOKEN(u16string_literal)
    def t_U16STRING_LITERAL(self, t: LexToken) -> LexToken:
        return t

    @TOKEN(u32string_literal)
    def t_U32STRING_LITERAL(self, t: LexToken) -> LexToken:
        return t

    # unmatched string literals are caught by the preprocessor

    @TOKEN(bad_string_literal)
    def t_BAD_STRING_LITERAL(self, t):
        msg = "String contains invalid escape code"
        self._error(msg, t)

    @TOKEN(r"[A-Za-z_~][A-Za-z0-9_]*")
    def t_NAME(self, t: LexToken) -> LexToken:
        if t.value in self.keywords:
            t.type = t.value
        return t

    @TOKEN(r"\#[\t ]*pragma")
    def t_PRAGMA_DIRECTIVE(self, t: LexToken) -> LexToken:
        return t

    @TOKEN(r"\#[\t ]*include (.*)")
    def t_INCLUDE_DIRECTIVE(self, t: LexToken) -> LexToken:
        return t

    @TOKEN(r"\#(.*)")
    def t_PP_DIRECTIVE(self, t: LexToken):
        # handle line macros
        m = _line_re.match(t.value)
        if m:
            self.filename = m.group(3)
            self.line_offset = 1 + self.lex.lineno - int(m.group(2))
            return None
        # ignore C++23 warning directive
        if t.value.startswith("#warning"):
            return
        if "define" in t.value:
            msgtype = "#define"
        else:
            msgtype = "preprocessor"
        self._error(
            "cxxheaderparser does not support "
            + msgtype
            + " directives, please use a C++ preprocessor first",
            t,
        )

    t_DIVIDE = r"/(?!/)"
    t_ELLIPSIS = r"\.\.\."
    t_DBL_LBRACKET = r"\[\["
    t_DBL_RBRACKET = r"\]\]"
    t_DBL_COLON = r"::"
    t_DBL_AMP = r"&&"
    t_DBL_PIPE = r"\|\|"
    t_ARROW = r"->"
    t_SHIFT_LEFT = r"<<"
    # SHIFT_RIGHT introduces ambiguity

    t_STRING_LITERAL = string_literal

    @TOKEN(r"\/\/.*\n?")
    def t_COMMENT_SINGLELINE(self, t: LexToken) -> LexToken:
        t.lexer.lineno += t.value.count("\n")
        return t

    # Found at http://ostermiller.org/findcomment.html
    @TOKEN(r"/\*([^*]|[\r\n]|(\*+([^*/]|[\r\n])))*\*+/\n?")
    def t_COMMENT_MULTILINE(self, t: LexToken) -> LexToken:
        t.lexer.lineno += t.value.count("\n")
        return t

    @TOKEN(r"\n+")
    def t_NEWLINE(self, t: LexToken) -> LexToken:
        t.lexer.lineno += len(t.value)
        return t

    def t_error(self, t: LexToken) -> None:
        self._error(f"Illegal character {t.value!r}", t)

    def _error(self, msg: str, tok: LexToken):
        tok.location = self.current_location()
        raise LexError(msg, tok)

    _lexer = None
    lex: lex.Lexer

    def __new__(cls, *args, **kwargs) -> "PlyLexer":
        # only build the lexer once
        inst = super().__new__(cls)
        if cls._lexer is None:
            cls._lexer = lex.lex(module=inst)

        inst.lex = cls._lexer.clone(inst)
        inst.lex.begin("INITIAL")
        return inst

    def __init__(self, filename: typing.Optional[str] = None):
        self.input: typing.Callable[[str], None] = self.lex.input
        self.token: typing.Callable[[], LexToken] = self.lex.token

        # For tracking current file/line position
        self.filename = filename
        self.line_offset = 0

    def current_location(self) -> Location:
        return Location(self.filename, self.lex.lineno - self.line_offset)


class TokenStream:
    """
    Provides access to a stream of tokens
    """

    tokbuf: typing.Deque[LexToken]

    def _fill_tokbuf(self, tokbuf: typing.Deque[LexToken]) -> bool:
        """
        Fills tokbuf with tokens from the next line. Return True if at least
        one token was added to the buffer
        """
        raise NotImplementedError

    def current_location(self) -> Location:
        raise NotImplementedError

    def get_doxygen(self) -> typing.Optional[str]:
        """
        This is called at the point that you want doxygen information
        """
        raise NotImplementedError

    def get_doxygen_after(self) -> typing.Optional[str]:
        """
        This is called to retrieve doxygen information after a statement
        """
        raise NotImplementedError

    _discard_types = {
        "NEWLINE",
        "COMMENT_SINGLELINE",
        "COMMENT_MULTILINE",
        "WHITESPACE",
    }

    _discard_types_except_newline = {
        "COMMENT_SINGLELINE",
        "COMMENT_MULTILINE",
        "WHITESPACE",
    }

    def token(self) -> LexToken:
        tokbuf = self.tokbuf
        while True:
            while tokbuf:
                tok = tokbuf.popleft()
                if tok.type not in self._discard_types:
                    return tok

            if not self._fill_tokbuf(tokbuf):
                raise EOFError("unexpected end of file")

    def token_eof_ok(self) -> typing.Optional[LexToken]:
        tokbuf = self.tokbuf
        while True:
            while tokbuf:
                tok = tokbuf.popleft()
                if tok.type not in self._discard_types:
                    return tok

            if not self._fill_tokbuf(tokbuf):
                return None

    def token_newline_eof_ok(self) -> typing.Optional[LexToken]:
        tokbuf = self.tokbuf
        while True:
            while tokbuf:
                tok = tokbuf.popleft()
                if tok.type not in self._discard_types_except_newline:
                    return tok

            if not self._fill_tokbuf(tokbuf):
                return None

    def token_if(self, *types: str) -> typing.Optional[LexToken]:
        tok = self.token_eof_ok()
        if tok is None:
            return None
        if tok.type not in types:
            self.tokbuf.appendleft(tok)
            return None
        return tok

    def token_if_in_set(self, types: typing.Set[str]) -> typing.Optional[LexToken]:
        tok = self.token_eof_ok()
        if tok is None:
            return None
        if tok.type not in types:
            self.tokbuf.appendleft(tok)
            return None
        return tok

    def token_if_val(self, *vals: str) -> typing.Optional[LexToken]:
        tok = self.token_eof_ok()
        if tok is None:
            return None
        if tok.value not in vals:
            self.tokbuf.appendleft(tok)
            return None
        return tok

    def token_if_not(self, *types: str) -> typing.Optional[LexToken]:
        tok = self.token_eof_ok()
        if tok is None:
            return None
        if tok.type in types:
            self.tokbuf.appendleft(tok)
            return None
        return tok

    def token_peek_if(self, *types: str) -> bool:
        tok = self.token_eof_ok()
        if not tok:
            return False
        self.tokbuf.appendleft(tok)
        return tok.type in types

    def return_token(self, tok: LexToken) -> None:
        self.tokbuf.appendleft(tok)

    def return_tokens(self, toks: typing.Sequence[LexToken]) -> None:
        self.tokbuf.extendleft(reversed(toks))


class LexerTokenStream(TokenStream):
    """
    Provides tokens from using PlyLexer on the given input text
    """

    _user_defined_literal_start = {
        "FLOAT_CONST",
        "HEX_FLOAT_CONST",
        "INT_CONST_HEX",
        "INT_CONST_BIN",
        "INT_CONST_OCT",
        "INT_CONST_DEC",
        "INT_CONST_CHAR",
        "CHAR_CONST",
        "WCHAR_CONST",
        "U8CHAR_CONST",
        "U16CHAR_CONST",
        "U32CHAR_CONST",
        # String literals
        "STRING_LITERAL",
        "WSTRING_LITERAL",
        "U8STRING_LITERAL",
        "U16STRING_LITERAL",
        "U32STRING_LITERAL",
    }

    def __init__(self, filename: typing.Optional[str], content: str) -> None:
        self._lex = PlyLexer(filename)
        self._lex.input(content)
        self.tokbuf = typing.Deque[LexToken]()

    def _fill_tokbuf(self, tokbuf: typing.Deque[LexToken]) -> bool:
        get_token = self._lex.token
        tokbuf = self.tokbuf

        tok = get_token()
        if tok is None:
            return False

        udl_start = self._user_defined_literal_start

        while True:
            tok.location = self._lex.current_location()
            tokbuf.append(tok)

            if tok.type == "NEWLINE":
                # detect/remove line continuations
                if len(tokbuf) > 2 and tokbuf[-2].type == "\\":
                    tokbuf.pop()
                    tokbuf.pop()
                else:
                    break

            # detect/combine user defined literals
            if tok.type in udl_start:
                tok2 = get_token()
                if tok2 is None:
                    break

                if tok2.type != "NAME" or tok2.value[0] != "_":
                    tok = tok2
                    continue

                tok.value = tok.value + tok2.value
                tok.type = f"UD_{tok.type}"

            tok = get_token()
            if tok is None:
                break

        return True

    def current_location(self) -> Location:
        if self.tokbuf:
            return self.tokbuf[0].location
        return self._lex.current_location()

    def get_doxygen(self) -> typing.Optional[str]:
        tokbuf = self.tokbuf

        # fill the token buffer if it's empty (which indicates a newline)
        if not tokbuf and not self._fill_tokbuf(tokbuf):
            return None

        comments: typing.List[LexToken] = []

        # retrieve any comments in the stream right before
        # the first non-discard element
        keep_going = True
        while True:
            while tokbuf:
                tok = tokbuf.popleft()
                if tok.type == "NEWLINE":
                    comments.clear()
                elif tok.type == "WHITESPACE":
                    pass
                elif tok.type in ("COMMENT_SINGLELINE", "COMMENT_MULTILINE"):
                    comments.append(tok)
                else:
                    tokbuf.appendleft(tok)
                    keep_going = False
                    break

            if not keep_going:
                break

            if not self._fill_tokbuf(tokbuf):
                break

        if comments:
            return self._extract_comments(comments)

        return None

    def get_doxygen_after(self) -> typing.Optional[str]:
        tokbuf = self.tokbuf

        # if there's a newline directly after a statement, we're done
        if not tokbuf:
            return None

        # retrieve comments after non-discard elements
        comments: typing.List[LexToken] = []
        new_tokbuf = typing.Deque[LexToken]()

        # This is different: we only extract tokens here
        while tokbuf:
            tok = tokbuf.popleft()
            if tok.type == "NEWLINE":
                break
            elif tok.type == "WHITESPACE":
                new_tokbuf.append(tok)
            elif tok.type in ("COMMENT_SINGLELINE", "COMMENT_MULTILINE"):
                comments.append(tok)
            else:
                new_tokbuf.append(tok)
                if comments:
                    break

        new_tokbuf.extend(tokbuf)
        self.tokbuf = new_tokbuf

        if comments:
            return self._extract_comments(comments)

        return None

    def _extract_comments(self, comments: typing.List[LexToken]):
        # Now we have comments, need to extract the text from them
        comment_lines: typing.List[str] = []
        for c in comments:
            text = c.value
            if c.type == "COMMENT_SINGLELINE":
                if text.startswith("///") or text.startswith("//!"):
                    comment_lines.append(text.rstrip("\n"))
            else:
                if text.startswith("/**") or text.startswith("/*!"):
                    # not sure why, but get double new lines
                    text = text.replace("\n\n", "\n")
                    # strip prefixing whitespace
                    text = _multicomment_re.sub("\n*", text)
                    comment_lines = text.splitlines()

        comment_str = "\n".join(comment_lines)
        if comment_str:
            return comment_str

        return None


class BoundedTokenStream(TokenStream):
    """
    Provides tokens from a fixed list of tokens.

    Intended for use when you have a group of tokens that you know
    must be consumed, such as a paren grouping or some type of
    lookahead case
    """

    def __init__(self, toks: typing.List[LexToken]) -> None:
        self.tokbuf = typing.Deque[LexToken](toks)

    def has_tokens(self) -> bool:
        return len(self.tokbuf) > 0

    def _fill_tokbuf(self, tokbuf: typing.Deque[LexToken]) -> bool:
        raise CxxParseError("no more tokens left in this group")

    def current_location(self) -> Location:
        if self.tokbuf:
            return self.tokbuf[0].location
        raise ValueError("internal error")

    def get_doxygen(self) -> typing.Optional[str]:
        # comment tokens aren't going to be in this stream
        return None

    def get_doxygen_after(self) -> typing.Optional[str]:
        return None


if __name__ == "__main__":  # pragma: no cover
    try:
        lex.runmain(lexer=PlyLexer(None))
    except EOFError:
        pass
