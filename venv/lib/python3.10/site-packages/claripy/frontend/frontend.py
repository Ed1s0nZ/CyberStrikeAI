from __future__ import annotations

import logging
import numbers

from claripy import BVV, BoolV, StringV
from claripy.ast import BV, String

log = logging.getLogger(__name__)


class Frontend:
    """Frontend is the base class for all claripy Solvers, which are the
    interfaces to the backend constraint solvers.
    """

    def __init__(self):
        pass

    def __getstate__(self):
        return True  # need to return something so that pickle calls setstate

    def __setstate__(self, s):  # pylint:disable=unused-argument
        return

    def branch(self):
        c = self.blank_copy()
        self._copy(c)
        return c

    def blank_copy(self):
        c = self.__class__.__new__(self.__class__)
        self._blank_copy(c)
        return c

    def _blank_copy(self, c):  # pylint:disable=no-self-use,unused-argument
        return

    def _copy(self, c):  # pylint:disable=no-self-use,unused-argument
        return

    #
    # Stuff that should be implemented by subclasses
    #

    def eval_to_ast(self, e, n, extra_constraints=(), exact=None):
        """
        Evaluates expression `e`, returning a list of `n` concrete ASTs.

        :param e:                       the expression
        :param n:                       the number of ASTs to return
        :param extra_constraints:       extra constraints to consider when performing the evaluation
        :param exact:                   whether or not to perform an exact evaluation. Ignored by
                                        non-approximating backends.

        :return:                        list of concrete ASTs
        """
        values = self.eval(e, n, extra_constraints=extra_constraints, exact=exact)

        if isinstance(e, BV):
            return [BVV(v, e.size()) for v in values]
        if isinstance(e, String):
            return [StringV(v) for v in values]
        # TODO: Implement support for other types
        raise NotImplementedError

    def finalize(self):
        raise NotImplementedError("finalize() is not implemented")

    def merge(self, others, merge_conditions, common_ancestor=None):
        raise NotImplementedError("merge() is not implemented")

    def combine(self, others):
        raise NotImplementedError("combine() is not implemented")

    def split(self):
        raise NotImplementedError("split() is not implemented")

    def add(self, constraints, invalidate_cache=True):
        """
        Adds constraint(s) to constraints list.

        :param constraints:             constraint(s) to add

        :return:
        """
        constraints = [constraints] if not isinstance(constraints, list | tuple | set) else constraints
        if len(constraints) == 0:
            return []
        constraints = [BoolV(c) if isinstance(c, bool) else c for c in constraints]
        return self._add(constraints, invalidate_cache=invalidate_cache)

    def _add(self, constraints, invalidate_cache=True):
        """
        Adds constraint(s) to constraints list. This version is called by add()
        with constrained constraints.

        :param constraints:             constraint(s) to add

        :return:
        """
        raise NotImplementedError

    def simplify(self):
        """
        Simplifies the stored constraints conjunction.
        """
        raise NotImplementedError

    def check_satisfiability(self, extra_constraints=(), exact=None):
        """
        Checks the satisfiability of stored constraints conjunction.

        :param extra_constraints:       extra constraints to consider when checking satisfiability
        :param exact:                   whether or not to perform exact checking. Ignored by
                                        non-approximating backends.

        :return:                        'SAT' if the conjunction is satisfiable otherwise 'UNSAT'
        """
        raise NotImplementedError

    def satisfiable(self, extra_constraints=(), exact=None):
        """
        Checks if stored constraints conjunction is satisfiable.

        :param extra_constraints:       extra constraints to consider when checking satisfiability
        :param exact:                   whether or not to perform exact checking. Ignored by
                                        non-approximating backends.

        :return:                        True if the conjunction is satisfiable otherwise False
        """
        raise NotImplementedError

    def eval(self, e, n, extra_constraints=(), exact=None):
        """
        Evaluates expression `e`, returning a tuple of `n` solutions.

        :param e:                       the expression
        :param n:                       the number of solutions to return
        :param extra_constraints:       extra constraints to consider when performing the evaluation
        :param exact:                   whether or not to perform an exact evaluation. Ignored by
                                        non-approximating backends.

        :return:                        tuple of python primitives representing results
        """
        raise NotImplementedError

    def batch_eval(self, exprs, n, extra_constraints=(), exact=None):
        """
        Evaluates `exprs`, returning a list of tuples (one tuple of `n` solutions for expression).

        :param exprs:                   expressions
        :param n:                       the number of solutions to return
        :param extra_constraints:       extra constraints to consider when performing the evaluation
        :param exact:                   whether or not to perform an exact evaluation. Ignored by
                                        non-approximating backends.

        :return:                        list of tuples of python primitives representing results
        """
        raise NotImplementedError

    def max(self, e, extra_constraints=(), signed=False, exact=None):
        """
        Evaluates `e`, returning its max possible value.

        :param e:                       the expression
        :param extra_constraints:       extra constraints to consider when performing the evaluation
        :param signed:                  whether the value should be treated as a signed integer
        :param exact:                   whether or not to perform an exact evaluation. Ignored by
                                        non-approximating backends.

        :return:                        max possible value
        """
        raise NotImplementedError

    def min(self, e, extra_constraints=(), signed=False, exact=None):
        """
        Evaluates `e`, returning its min possible value.

        :param e:                       the expression
        :param extra_constraints:       extra constraints to consider when performing the evaluation
        :param signed:                  whether the value should be treated as a signed integer
        :param exact:                   whether or not to perform an exact evaluation. Ignored by
                                        non-approximating backends.

        :return:                        min possible value
        """
        raise NotImplementedError

    def solution(self, e, v, extra_constraints=(), exact=None):
        """
        Checks if `v` is a possible solution to `e`.

        :param e:                       the expression
        :param v:                       the value
        :param extra_constraints:       extra constraints to consider when performing the evaluation
        :param exact:                   whether or not to perform an exact evaluation. Ignored by
                                        non-approximating backends.

        :return:                        True if it is a possible solution otherwise False
        """
        raise NotImplementedError

    def is_true(self, e, extra_constraints=(), exact=None):
        """
        Checks if `e` can only (and TRIVIALLY) evaluate to True. If this function returns True,
        then the expression cannot ever be False, regardless of constraints or anything else.
        If the expression returns False, then the expression might STILL not ever be False; it's just
        that we can't trivially prove it. In other words, a return value of False gives you no
        information whatsoever.


        :param e:                       the expression
        :param extra_constraints:       extra constraints to consider when performing the evaluation
        :param exact:                   whether or not to perform an exact evaluation. Ignored by
                                        non-approximating backends.

        :return:                        True if it can only evaluate to True otherwise False
        """
        raise NotImplementedError

    def is_false(self, e, extra_constraints=(), exact=None):
        """
        Checks if `e` can only (and TRIVIALLY) evaluate to False. If this function returns True,
        then the expression cannot ever be True, regardless of constraints or anything else.
        If the expression returns False, then the expression might STILL not ever be True; it's just
        that we can't trivially prove it. In other words, a return value of False gives you no
        information whatsoever.

        :param e:                       the expression
        :param extra_constraints:       extra constraints to consider when performing the evaluation
        :param exact:                   whether or not to perform an exact evaluation. Ignored by
                                        non-approximating backends.

        :return:                        True if it can only evaluate to False otherwise False
        """
        raise NotImplementedError

    def downsize(self):  # pylint:disable=no-self-use
        pass

    #
    # Some utility functions
    #

    def _concrete_value(self, e):  # pylint:disable=no-self-use
        if isinstance(e, numbers.Number):
            return e
        return None

    def _concrete_constraint(self, e):  # pylint:disable=no-self-use
        return self._concrete_value(e)
