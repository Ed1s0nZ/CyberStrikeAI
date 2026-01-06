from __future__ import annotations

from .bool_check import is_false, is_true
from .ite_relocation import burrow_ite, excavate_ite
from .replace import replace, replace_dict
from .simplify import simplify

__all__ = (
    "burrow_ite",
    "excavate_ite",
    "is_false",
    "is_true",
    "replace",
    "replace_dict",
    "simplify",
)
