from __future__ import annotations

from typing import TypeVar, cast
from weakref import WeakValueDictionary

import claripy
from claripy.ast import Base

T = TypeVar("T", bound=Base)

#
# This code handles burrowing ITEs deeper into the ast and excavating
# them to shallower levels.
#

burrowed_cache: WeakValueDictionary[int, Base] = WeakValueDictionary()
excavated_cache: WeakValueDictionary[int, Base] = WeakValueDictionary()


def _burrow_ite(expr: T) -> T:
    if expr.op != "If":
        return expr.make_like(expr.op, [(burrow_ite(a) if isinstance(a, Base) else a) for a in expr.args])

    if not all(isinstance(a, Base) for a in expr.args):
        return expr

    old_true = expr.args[1]
    old_false = expr.args[2]

    if old_true.op != old_false.op or len(old_true.args) != len(old_false.args):
        return expr

    if old_true.op == "If":
        # let's no go into this right now
        return expr

    if any(a.is_leaf() for a in expr.args):
        # burrowing through these is pretty funny
        return expr

    matches = [old_true.args[i] is old_false.args[i] for i in range(len(old_true.args))]
    if matches.count(True) != 1 or all(matches):
        # TODO: handle multiple differences for multi-arg ast nodes
        # print("wrong number of matches:",matches,old_true,old_false)
        return expr

    different_idx = matches.index(False)
    inner_if = claripy.If(expr.args[0], old_true.args[different_idx], old_false.args[different_idx])
    new_args = list(old_true.args)
    new_args[different_idx] = burrow_ite(inner_if)
    return old_true.__class__(old_true.op, new_args, length=expr.length)


def _excavate_ite(expr: T) -> T:
    ast_queue = [iter([expr])]
    arg_queue = []
    op_queue = []

    while ast_queue:
        try:
            ast = next(ast_queue[-1])

            if not isinstance(ast, Base):
                arg_queue.append(ast)
                continue

            if ast.is_leaf():
                arg_queue.append(ast)
                continue

            if ast.annotations:
                arg_queue.append(ast)
                continue

            op_queue.append(ast)
            ast_queue.append(iter(ast.args))

        except StopIteration:
            ast_queue.pop()

            if op_queue:
                op = op_queue.pop()

                args = arg_queue[-len(op.args) :]
                del arg_queue[-len(op.args) :]

                ite_args = [isinstance(a, Base) and a.op == "If" for a in args]

                if op.op == "If":
                    # if we are an If, call the If handler so that we can take advantage of its simplifiers
                    excavated = claripy.If(*args)

                elif ite_args.count(True) == 0:
                    # if there are no ifs that came to the surface, there's nothing more to do
                    excavated = op.make_like(op.op, args, simplify=True)

                else:
                    # this gets called when we're *not* in an If, but there are Ifs in the args.
                    # it pulls those Ifs out to the surface.
                    cond = args[ite_args.index(True)].args[0]
                    new_true_args = []
                    new_false_args = []

                    for a in args:
                        if not isinstance(a, Base) or a.op != "If":
                            new_true_args.append(a)
                            new_false_args.append(a)
                        elif a.args[0] is cond:
                            new_true_args.append(a.args[1])
                            new_false_args.append(a.args[2])
                        elif a.args[0] is ~cond:
                            new_true_args.append(a.args[2])
                            new_false_args.append(a.args[1])
                        else:
                            # weird conditions -- giving up!
                            excavated = op.make_like(op.op, args, simplify=True)
                            break

                    else:
                        excavated = claripy.If(
                            cond,
                            op.make_like(op.op, new_true_args, simplify=True),
                            op.make_like(op.op, new_false_args, simplify=True),
                        )

                # continue
                arg_queue.append(excavated)

    assert len(op_queue) == 0, "op_queue is not empty"
    assert len(ast_queue) == 0, "ast_queue is not empty"
    assert len(arg_queue) == 1, ("arg_queue has unexpected length", len(arg_queue))

    return arg_queue.pop()


def burrow_ite(expr: T) -> T:
    """
    Returns an equivalent AST that "burrows" the ITE expressions as deep as
    possible into the ast, for simpler printing.
    """
    if expr.hash() in burrowed_cache and burrowed_cache[expr.hash()] is not None:
        return cast("T", burrowed_cache[expr.hash()])

    burrowed = _burrow_ite(expr)
    burrowed_cache[burrowed.hash()] = burrowed
    burrowed_cache[expr.hash()] = burrowed
    return burrowed


def excavate_ite(expr: T) -> T:
    """
    Returns an equivalent AST that "excavates" the ITE expressions out as far as
    possible toward the root of the AST, for processing in static analyses.
    """
    if expr.hash() in excavated_cache and excavated_cache[expr.hash()] is not None:
        return cast("T", excavated_cache[expr.hash()])

    excavated = _excavate_ite(expr)
    excavated_cache[excavated.hash()] = excavated
    excavated_cache[expr.hash()] = excavated
    return excavated
