"""
A class to read Python format regex
"""

import re
import string
import unicodedata

# pylint: disable=cyclic-import
from pyformlang.regular_expression import regex, MisformedRegexError
from pyformlang.regular_expression.regex_reader import \
    WRONG_PARENTHESIS_MESSAGE

PRINTABLES = list(string.printable)

TRANSFORMATIONS = {
    "|": "\\|",
    "(": "\\(",
    ")": "\\)",
    "*": "\\*",
    "+": "\\+",
    ".": "\\.",
    "$": "\\$",
    "\n": "",
    " ": "\\ ",
    '\\': '\\\\',
    "?": "\\?"
}

RECOMBINE = {
    "\\b": "\b",
    "\\n": "\n",
    "\\r": "\r",
    "\\t": "\t",
    "\\f": "\f"
}

ESCAPED_PRINTABLES = [TRANSFORMATIONS.get(x, x)
                      for x in PRINTABLES
                      if TRANSFORMATIONS.get(x, x)]

DOT_REPLACEMENT = "(" + "|".join(ESCAPED_PRINTABLES) + ")"

TO_ESCAPE_IN_BRACKETS = "(+*)?"

SHORTCUTS = {
    " ": "\\ ",  # We have to do this due to how Regex separate words
    r"\d": "[0-9]",
    r"\s": "[\\ \t\n\r\f\v]",
    r"\w": "[a-zA-Z0-9_]"
}

HEXASTRING = "0123456789ABCDEF"
OCTAL = "01234567"
ESCAPED_OCTAL = ["\\0", "\\1", "\\2", "\\3", "\\4", "\\5", "\\6", "\\7"]


class PythonRegex(regex.Regex):
    """ Represents a regular expression as used in Python.

    It adds the following features to the basic regex:

    * Set of characters with []
    * Inverse set of character with [^...]
    * positive closure +
    * . for all printable characters
    * ? for optional character/group
    * Repetition of characters with {m} and {n,m}q
    * Shortcuts: \\d, \\s, \\w

    Parameters
    ----------
    python_regex : Union[str, Pattern[str]]
        The regex represented as a string or a compiled regex (
        re.compile(...))

    Raises
    ------
    MisformedRegexError
        If the regular expression is misformed.

    Examples
    --------
    Python regular expressions wrapper

    >>> from pyformlang.regular_expression import PythonRegex

    >>> p_regex = PythonRegex("a+[cd]")
    >>> p_regex.accepts(["a", "a", "d"])
    True

    As the alphabet is composed of single characters, one could also write

    >>> p_regex.accepts("aad")
    True
    >>> p_regex.accepts(["d"])
    False

    """

    def __init__(self, python_regex):
        if not isinstance(python_regex, str):
            python_regex = python_regex.pattern
        else:
            re.compile(python_regex)  # Check if it is valid

        self._python_regex = python_regex
        self._replace_shortcuts()
        self._escape_in_brackets()
        self._preprocess_brackets()
        self._preprocess_positive_closure()
        self._preprocess_optional()
        self._separate()
        self._python_regex = self._python_regex.lstrip('\b')
        super().__init__(self._python_regex)

    def _separate(self):
        regex_temp = []
        for symbol in self._python_regex:
            if self._should_escape_next_symbol(regex_temp):
                regex_temp[-1] += symbol
            else:
                regex_temp.append(symbol)
        regex_temp = self._recombine(regex_temp)
        regex_temp_dot = []
        for symbol in regex_temp:
            if symbol == ".":
                regex_temp_dot.append(DOT_REPLACEMENT)
            else:
                regex_temp_dot.append(symbol)
        self._python_regex = " ".join(regex_temp_dot)

    def _preprocess_brackets(self):
        regex_temp = []
        in_brackets = 0
        in_brackets_temp = []
        for symbol in self._python_regex:
            if symbol == "[" and not self._should_escape_next_symbol(regex_temp) and \
                    (in_brackets == 0 or not self._should_escape_next_symbol(in_brackets_temp[-1])):
                in_brackets += 1
                in_brackets_temp.append([])
            elif symbol == "]" and in_brackets >= 1 and not self._should_escape_next_symbol(in_brackets_temp[-1]):
                if len(in_brackets_temp) == 1:
                    regex_temp.append("(")
                    regex_temp += self._preprocess_brackets_content(
                        in_brackets_temp[-1])
                    regex_temp.append(")")
                else:
                    in_brackets_temp[-2].append(
                        "(" +
                        "".join(
                            self._preprocess_brackets_content(
                                in_brackets_temp[-1])) +
                        ")")
                in_brackets -= 1
                in_brackets_temp.pop()
            elif in_brackets > 0:
                if self._should_escape_next_symbol(in_brackets_temp[-1]):
                    in_brackets_temp[-1][-1] += symbol
                elif symbol == "|":
                    in_brackets_temp[-1].append("\\|")
                else:
                    in_brackets_temp[-1].append(symbol)
            else:
                if self._should_escape_next_symbol(regex_temp):
                    regex_temp[-1] += symbol
                else:
                    regex_temp.append(symbol)
        self._python_regex = "".join(regex_temp)

    @staticmethod
    def _recombine(regex_to_recombine):
        temp = []
        idx = 0
        while idx < len(regex_to_recombine):
            if regex_to_recombine[idx] == "\\x" and idx < len(regex_to_recombine) - 2 \
                    and regex_to_recombine[idx + 1] in HEXASTRING \
                    and regex_to_recombine[idx + 2] in HEXASTRING:
                next_str = "".join(regex_to_recombine[idx + 1:idx + 3])
                s_trans = chr(int(next_str, 16))
                temp.append(TRANSFORMATIONS.get(s_trans, s_trans))
                idx += 3
            elif regex_to_recombine[idx] in ESCAPED_OCTAL \
                    and idx < len(regex_to_recombine) - 2 \
                    and regex_to_recombine[idx + 1] in OCTAL \
                    and regex_to_recombine[idx + 2] in OCTAL:
                next_str = "".join(regex_to_recombine[idx:idx + 3])[1:]
                s_trans = chr(int(next_str, 8))
                temp.append(TRANSFORMATIONS.get(s_trans, s_trans))
                idx += 3
            elif regex_to_recombine[idx] == "\\N":
                idx_end = idx
                while regex_to_recombine[idx_end] != "}":
                    idx_end += 1
                name = "".join(regex_to_recombine[idx + 2: idx_end])
                name = unicodedata.lookup(name)
                temp.append(TRANSFORMATIONS.get(name, name))
                idx = idx_end + 1
            elif regex_to_recombine[idx] == "\\u":
                unicode_str = "".join(regex_to_recombine[idx + 1: idx + 5])
                decoded = chr(int(unicode_str, 16))
                temp.append(TRANSFORMATIONS.get(decoded, decoded))
                idx = idx + 5
            elif regex_to_recombine[idx] == "\\U":
                unicode_str = "".join(regex_to_recombine[idx + 1: idx + 9])
                decoded = chr(int(unicode_str, 16))
                temp.append(TRANSFORMATIONS.get(decoded, decoded))
                idx = idx + 9
            else:
                temp.append(regex_to_recombine[idx])
                idx += 1
        res = []
        for x in temp:
            if x in RECOMBINE:
                res.append(RECOMBINE[x])
            else:
                res.append(x)
        return res

    def _preprocess_brackets_content(self, bracket_content):
        bracket_content_temp = []
        previous_is_valid_for_range = False
        for i, symbol in enumerate(bracket_content):
            # We have a range
            if symbol == "-" and not self._should_escape_next_symbol(bracket_content_temp):
                if not previous_is_valid_for_range or i == len(bracket_content) - 1:
                    # False alarm, no range
                    bracket_content_temp.append("-")
                    previous_is_valid_for_range = True
                else:
                    # We insert all the characters in the range
                    bracket_content[i - 1] = self._recombine(bracket_content[i - 1])
                    for j in range(ord(bracket_content[i - 1][-1]) + 1,
                                   ord(bracket_content[i + 1][-1])):
                        next_char = chr(j)
                        if next_char in TRANSFORMATIONS:
                            bracket_content_temp.append(TRANSFORMATIONS[next_char])
                        else:
                            bracket_content_temp.append(next_char)
                    previous_is_valid_for_range = False
            else:
                if self._should_escape_next_symbol(bracket_content_temp):
                    bracket_content_temp[-1] += symbol
                else:
                    bracket_content_temp.append(symbol)
                if (i != 0 and bracket_content[i - 1] == "-"
                        and not previous_is_valid_for_range):
                    previous_is_valid_for_range = False
                else:
                    previous_is_valid_for_range = True
        bracket_content_temp = self._preprocess_negation(bracket_content_temp)
        bracket_content_temp = self._insert_or(bracket_content_temp)
        bracket_content_temp = self._recombine(bracket_content_temp)
        return bracket_content_temp

    @staticmethod
    def _preprocess_negation(bracket_content):
        if not bracket_content or bracket_content[0] != "^":
            return bracket_content
        # We inverse everything
        return [x for x in ESCAPED_PRINTABLES if x not in bracket_content]

    @staticmethod
    def _insert_or(l_to_modify):
        res = []
        for x in l_to_modify:
            res.append(x)
            res.append("|")
        if res:
            return res[:-1]
        return res

    def _find_previous_opening_parenthesis(self, split_sequence):
        counter = 0
        for i in range(len(split_sequence) - 1, -1, -1):
            temp = split_sequence[i]
            if temp == ")":
                counter += 1
            elif temp == "(" and counter == 1:
                return i
            elif temp == "(":
                counter -= 1
        raise MisformedRegexError(WRONG_PARENTHESIS_MESSAGE,
                                  self._python_regex)

    def _preprocess_positive_closure(self):
        regex_temp = []
        for symbol in self._python_regex:
            if symbol != "+" or (self._should_escape_next_symbol(regex_temp)):
                if self._should_escape_next_symbol(regex_temp):
                    regex_temp[-1] += symbol
                else:
                    regex_temp.append(symbol)
            elif regex_temp[-1] != ")":
                regex_temp.append(regex_temp[-1])
                regex_temp.append("*")
            else:
                pos_opening = \
                    self._find_previous_opening_parenthesis(regex_temp)
                for j in range(pos_opening, len(regex_temp)):
                    regex_temp.append(regex_temp[j])
                regex_temp.append("*")
        regex_temp = self._add_repetition(regex_temp)
        self._python_regex = "".join(regex_temp)

    @staticmethod
    def _is_repetition(regex_list, idx):
        if regex_list[idx] == "{":
            end = idx
            for i in range(idx + 1, len(regex_list)):
                if regex_list[i] == "}":
                    end = i
                    break
            inner = "".join(regex_list[idx + 1:end])
            if "," in inner:
                split = inner.split(",")
                if len(split) != 2 or not split[0].isdigit() or not split[1].isdigit():
                    return None
                return int(split[0]), int(split[1]), end
            if inner.isdigit():
                return int(inner), end
        return None

    @staticmethod
    def _find_repeated_sequence(regex_list):
        if regex_list[-1] != ")":
            return [regex_list[-1]]
        res = [")"]
        counter = -1
        for i in range(len(regex_list) - 2, -1, -1):
            if regex_list[i] == "(":
                counter += 1
                res.append("(")
                if counter == 0:
                    return res[::-1]
            elif regex_list[i] == ")":
                counter -= 1
                res.append(")")
            else:
                res.append(regex_list[i])
        return []

    def _add_repetition(self, regex_list):
        res = []
        idx = 0
        while idx < len(regex_list):
            rep = self._is_repetition(regex_list, idx)
            if rep is None:
                res.append(regex_list[idx])
                idx += 1
            elif len(rep) == 2:
                n_rep, end = rep
                repeated = self._find_repeated_sequence(res)
                for _ in range(n_rep - 1):
                    res.extend(repeated)
                idx = end + 1
            elif len(rep) == 3:
                min_rep, max_rep, end = rep
                repeated = self._find_repeated_sequence(res)
                for _ in range(min_rep - 1):
                    res.extend(repeated)
                for _ in range(min_rep, max_rep):
                    res.extend(repeated)
                    res.append("?")
                idx = end + 1
        return res

    def _preprocess_optional(self):
        regex_temp = []
        for symbol in self._python_regex:
            if symbol == "?":
                if regex_temp[-1] == ")":
                    regex_temp[-1] = "|$)"
                elif regex_temp[-1] == "\\":
                    regex_temp[-1] = "?"
                else:
                    regex_temp[-1] = "(" + regex_temp[-1] + "|$)"
            else:
                if self._should_escape_next_symbol(regex_temp):
                    regex_temp[-1] += symbol
                else:
                    regex_temp.append(symbol)
        self._python_regex = "".join(regex_temp)

    @staticmethod
    def _should_escape_next_symbol(regex_temp):
        return regex_temp and regex_temp[-1] == "\\"

    def _escape_in_brackets(self):
        regex_temp = []
        in_brackets = False
        for symbol in self._python_regex:
            if (symbol == "["
                    and not self._should_escape_next_symbol(regex_temp)):
                in_brackets = True
            elif (symbol == "]"
                  and not self._should_escape_next_symbol(regex_temp)):
                in_brackets = False
            if (in_brackets
                    and not self._should_escape_next_symbol(regex_temp)
                    and symbol in TO_ESCAPE_IN_BRACKETS):
                regex_temp.append("\\" + symbol)
            elif self._should_escape_next_symbol(regex_temp):
                regex_temp[-1] += symbol
            else:
                regex_temp.append(symbol)
        self._python_regex = "".join(regex_temp)

    def _replace_shortcuts(self):
        for to_replace, replacement in SHORTCUTS.items():
            self._python_regex = self._python_regex.replace(to_replace,
                                                            replacement)
