"""
:mod:`pyformlang.regular_expression`
====================================

This module deals with regular expression.

By default, this module does not use the standard way to write regular
expressions. Please read the documentation of Regex for more information.

Available Classes
-----------------

:class:`~pyformlang.regular_expression.Regex`
    A regular expression
:class:`~pyformlang.regular_expression.PythonRegex`
    A regular expression closer to Python format
:class:`~pyformlang.regular_expression.MisformedRegexError`
    An error occurring when the input regex is incorrect

"""


from .regex import Regex
from .regex_objects import MisformedRegexError
from .python_regex import PythonRegex

__all__ = ["Regex", "PythonRegex", "MisformedRegexError"]
