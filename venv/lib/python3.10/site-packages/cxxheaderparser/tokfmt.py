from dataclasses import dataclass, field
import typing

from .lexer import LexToken, PlyLexer, LexerTokenStream

# key: token type, value: (left spacing, right spacing)
_want_spacing = {
    "FLOAT_CONST": (2, 2),
    "HEX_FLOAT_CONST": (2, 2),
    "INT_CONST_HEX": (2, 2),
    "INT_CONST_BIN": (2, 2),
    "INT_CONST_OCT": (2, 2),
    "INT_CONST_DEC": (2, 2),
    "INT_CONST_CHAR": (2, 2),
    "NAME": (2, 2),
    "CHAR_CONST": (2, 2),
    "WCHAR_CONST": (2, 2),
    "U8CHAR_CONST": (2, 2),
    "U16CHAR_CONST": (2, 2),
    "U32CHAR_CONST": (2, 2),
    "STRING_LITERAL": (2, 2),
    "WSTRING_LITERAL": (2, 2),
    "U8STRING_LITERAL": (2, 2),
    "U16STRING_LITERAL": (2, 2),
    "U32STRING_LITERAL": (2, 2),
    "ELLIPSIS": (2, 2),
    ">": (0, 2),
    ")": (0, 1),
    "(": (1, 0),
    ",": (0, 3),
    "*": (1, 2),
    "&": (0, 2),
}

_want_spacing.update(dict.fromkeys(PlyLexer.keywords, (2, 2)))


@dataclass
class Token:
    """
    In an ideal world, this Token class would not be exposed via the user
    visible API. Unfortunately, getting to that point would take a significant
    amount of effort.

    It is not expected that these will change, but they might.

    At the moment, the only supported use of Token objects are in conjunction
    with the ``tokfmt`` function. As this library matures, we'll try to clarify
    the expectations around these. File an issue on github if you have ideas!
    """

    #: Raw value of the token
    value: str

    #: Lex type of the token
    type: str = field(repr=False, compare=False, default="")


def tokfmt(toks: typing.List[Token]) -> str:
    """
    Helper function that takes a list of tokens and converts them to a string
    """
    last = 0
    vals = []
    default = (0, 0)
    ws = _want_spacing

    for tok in toks:
        value = tok.value
        # special case
        if value == "operator":
            l, r = 2, 0
        else:
            l, r = ws.get(tok.type, default)
        if l + last >= 3:
            vals.append(" ")

        last = r
        vals.append(value)

    return "".join(vals)


if __name__ == "__main__":  # pragma: no cover
    import argparse

    parser = argparse.ArgumentParser()
    parser.add_argument("header")
    args = parser.parse_args()

    filename: str = args.header
    with open(filename) as fp:
        lexer = LexerTokenStream(filename, fp.read())

    toks: typing.List[Token] = []
    while True:
        tok = lexer.token_eof_ok()
        if not tok:
            break
        if tok.type == ";":
            print(toks)
            print(tokfmt(toks))
            toks = []
        else:
            toks.append(Token(tok.value, tok.type))

    print(toks)
    print(tokfmt(toks))
