from __future__ import annotations

from typing import TYPE_CHECKING, TypeVar

from claripy.ast import Base
from claripy.errors import ClaripyReplacementError

if TYPE_CHECKING:
    from collections.abc import Callable

T = TypeVar("T", bound="Base")


def replace_dict(
    expr: Base,
    replacements: dict[int, Base],
    variable_set: set[str] | None = None,
    leaf_operation: Callable[[Base], Base] = lambda x: x,
) -> Base:
    """
    Returns this AST with subexpressions replaced by those that can be found in `replacements`
    dict.

    :param variable_set:        For optimization, ast's without these variables are not checked
                                    for replacing.
    :param replacements:        A dictionary of hashes to their replacements.
    :param leaf_operation:      An operation that should be applied to the leaf nodes.
    :returns:                   An AST with all instances of ast's in replacements.
    """
    if variable_set is None:
        variable_set = set()

    arg_queue = [iter([expr])]
    rep_queue = []
    ast_queue = []

    while arg_queue:
        try:
            ast = next(arg_queue[-1])
            repl = ast

            if not isinstance(ast, Base):
                rep_queue.append(repl)
                continue

            if ast.hash() in replacements:
                repl = replacements[ast.hash()]

            elif ast.variables >= variable_set:
                if ast.is_leaf():
                    repl = leaf_operation(ast)
                    if repl is not ast:
                        replacements[ast.hash()] = repl

                elif ast.depth > 1:
                    arg_queue.append(iter(ast.args))
                    ast_queue.append(ast)
                    continue

            rep_queue.append(repl)

        except StopIteration:
            arg_queue.pop()

            if ast_queue:
                ast = ast_queue.pop()
                repl = ast

                args = rep_queue[-len(ast.args) :]
                del rep_queue[-len(ast.args) :]

                # Check if replacement occurred.
                if any((a is not b for a, b in zip(ast.args, args, strict=False))):
                    repl = ast.make_like(ast.op, tuple(args))
                    replacements[ast.hash()] = repl

                rep_queue.append(repl)

    assert len(arg_queue) == 0, "arg_queue is not empty"
    assert len(ast_queue) == 0, "ast_queue is not empty"
    assert len(rep_queue) == 1, ("rep_queue has unexpected length", len(rep_queue))

    return rep_queue.pop()


def _check_replaceability(old: T, new: T) -> None:
    if not isinstance(old, Base) or not isinstance(new, Base):
        raise ClaripyReplacementError("replacements must be AST nodes")
    if type(old) is not type(new):
        raise ClaripyReplacementError(f"cannot replace type {type(old)} ast with type {type(new)} ast")


def replace(expr: Base, old: T, new: T) -> Base:
    """
    Returns this AST but with the AST 'old' replaced with AST 'new' in its subexpressions.
    """
    _check_replaceability(old, new)
    replacements: dict[int, Base] = {old.hash(): new}
    return replace_dict(expr, replacements, variable_set=old.variables)
