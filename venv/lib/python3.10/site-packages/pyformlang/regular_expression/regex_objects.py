"""
Representation of some objects used in regex.
"""
import pyformlang


class Node:  # pylint: disable=too-few-public-methods
    """ Represents a node in the tree representation of a regex

    Parameters
    ----------
    value : str
        The value of the node
    """

    def __init__(self, value):
        self._value = value

    @property
    def value(self):
        """ Give the value of the node

        Returns
        ----------
        value : str
            The value of the node
        """
        return self._value

    def get_str_repr(self, sons_repr):
        """
        The string representation of the node

        Parameters
        ----------
        sons_repr : iterable of str
            The sons representations

        Returns
        -------
        repr : str
            The representation of this node

        """
        raise NotImplementedError

    def get_cfg_rules(self, current_symbol, sons):
        """ Gets the rules for a context-free grammar to represent the \
        operator"""
        raise NotImplementedError


CONCATENATION_SYMBOLS = ["."]
UNION_SYMBOLS = ["|", "+"]
KLEENE_STAR_SYMBOLS = ["*"]
EPSILON_SYMBOLS = ["epsilon", "$"]
PARENTHESIS = ["(", ")"]

SPECIAL_SYMBOLS = CONCATENATION_SYMBOLS + \
                  UNION_SYMBOLS + \
                  KLEENE_STAR_SYMBOLS + \
                  EPSILON_SYMBOLS + \
                  PARENTHESIS


def to_node(value: str) -> Node:
    """ Transforms a given value into a node """
    if not value:
        res = Empty()
    elif value in CONCATENATION_SYMBOLS:
        res = Concatenation()
    elif value in UNION_SYMBOLS:
        res = Union()
    elif value in KLEENE_STAR_SYMBOLS:
        res = KleeneStar()
    elif value in EPSILON_SYMBOLS:
        res = Epsilon()
    elif value[0] == "\\":
        res = Symbol(value[1:])
    else:
        res = Symbol(value)
    return res


class Operator(Node):  # pylint: disable=too-few-public-methods
    """ Represents an operator

    Parameters
    ----------
    value : str
        The value of the operator
    """

    def __repr__(self):
        return "Operator(" + str(self._value) + ")"

    def get_str_repr(self, sons_repr):
        """ Get the string representation """
        raise NotImplementedError

    def get_cfg_rules(self, current_symbol, sons):
        """ Gets the rules for a context-free grammar to represent the \
        operator"""
        raise NotImplementedError


class Symbol(Node):  # pylint: disable=too-few-public-methods
    """ Represents a symbol

    Parameters
    ----------
    value : str
        The value of the symbol
    """

    def get_str_repr(self, sons_repr):
        return str(self.value)

    def get_cfg_rules(self, current_symbol, sons):
        """ Gets the rules for a context-free grammar to represent the \
        operator"""
        return [pyformlang.cfg.Production(
            pyformlang.cfg.utils.to_variable(current_symbol),
            [pyformlang.cfg.utils.to_terminal(self.value)])]

    def __repr__(self):
        return "Symbol(" + str(self._value) + ")"


class Concatenation(Operator):  # pylint: disable=too-few-public-methods
    """ Represents a concatenation
    """

    def get_str_repr(self, sons_repr):
        return "(" + ".".join(sons_repr) + ")"

    def get_cfg_rules(self, current_symbol, sons):
        return [pyformlang.cfg.Production(
            pyformlang.cfg.utils.to_variable(current_symbol),
            [pyformlang.cfg.utils.to_variable(son) for son in sons])]

    def __init__(self):
        super().__init__("Concatenation")


class Union(Operator):  # pylint: disable=too-few-public-methods
    """ Represents a union
    """

    def get_str_repr(self, sons_repr):
        return "(" + "|".join(sons_repr) + ")"

    def get_cfg_rules(self, current_symbol, sons):
        return [pyformlang.cfg.Production(
            pyformlang.cfg.utils.to_variable(current_symbol),
            [pyformlang.cfg.utils.to_variable(son)])
                for son in sons]

    def __init__(self):
        super().__init__("Union")


class KleeneStar(Operator):  # pylint: disable=too-few-public-methods
    """ Represents an epsilon symbol
    """

    def get_str_repr(self, sons_repr):
        return "(" + ".".join(sons_repr) + ")*"

    def get_cfg_rules(self, current_symbol, sons):
        return [
            pyformlang.cfg.Production(
                pyformlang.cfg.utils.to_variable(current_symbol),
                []),
            pyformlang.cfg.Production(
                pyformlang.cfg.utils.to_variable(current_symbol),
                [pyformlang.cfg.utils.to_variable(current_symbol),
                 pyformlang.cfg.utils.to_variable(current_symbol)]),
            pyformlang.cfg.Production(
                pyformlang.cfg.utils.to_variable(current_symbol),
                [pyformlang.cfg.utils.to_variable(son) for son in sons])
        ]

    def __init__(self):
        super().__init__("Kleene Star")


class Epsilon(Symbol):  # pylint: disable=too-few-public-methods
    """ Represents an epsilon symbol
    """

    def get_str_repr(self, sons_repr):
        return "$"

    def get_cfg_rules(self, current_symbol, sons):
        return [pyformlang.cfg.Production(
            pyformlang.cfg.utils.to_variable(current_symbol),
            [])]

    def __init__(self):
        super().__init__("Epsilon")


class Empty(Symbol):  # pylint: disable=too-few-public-methods
    """ Represents an empty symbol
    """

    def __init__(self):
        super().__init__("Empty")

    def get_cfg_rules(self, current_symbol, sons):
        return []


class MisformedRegexError(Exception):
    """ Error for misformed regex """

    def __init__(self, message: str, regex: str):
        super().__init__(message + " Regex: " + regex)
        self._regex = regex
