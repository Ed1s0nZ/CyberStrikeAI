from __future__ import annotations

from typing import TYPE_CHECKING

from claripy.errors import ClaripyOperationError

if TYPE_CHECKING:
    import claripy


class Annotation:
    """
    Annotations are used to achieve claripy's goal of being an arithmetic instrumentation engine.
    They provide a means to pass extra information to the claripy backends.
    """

    @property
    def eliminatable(self) -> bool:  # pylint:disable=no-self-use
        """
        Returns whether this annotation can be eliminated in a simplification.

        :return: True if eliminatable, False otherwise
        """
        return True

    @property
    def relocatable(self) -> bool:  # pylint:disable=no-self-use
        """
        Returns whether this annotation can be relocated in a simplification.

        :return: True if it can be relocated, false otherwise.
        """
        return False

    def relocate(self, src: claripy.ast.Base, dst: claripy.ast.Base):  # pylint:disable=no-self-use,unused-argument
        """
        This is called when an annotation has to be relocated because of simplifications.

        Consider the following case:

            x = claripy.BVS('x', 32)
            zero = claripy.BVV(0, 32).add_annotation(your_annotation)
            y = x + zero

        Here, one of three things can happen:

            1. if your_annotation.eliminatable is True, the simplifiers will simply
               eliminate your_annotation along with `zero` and `y is x` will hold
            2. elif your_annotation.relocatable is False, the simplifier will abort
               and y will never be simplified
            3. elif your_annotation.relocatable is True, the simplifier will run,
               determine that the simplified result of `x + zero` will be `x`. It
               will then call your_annotation.relocate(zero, x) to move the annotation
               away from the AST that is about to be eliminated.

        :param src: the old AST that was eliminated in the simplification
        :param dst: the new AST (the result of a simplification)
        :return: the annotation that will be applied to `dst`
        """
        if self.relocatable:
            return self
        raise ClaripyOperationError("Annotation is not relocatable")


#
# Some built-in annotations
#


class SimplificationAvoidanceAnnotation(Annotation):
    """SimplificationAvoidanceAnnotation is an annotation that prevents simplification of an AST."""

    @property
    def eliminatable(self):
        return False

    @property
    def relocatable(self):
        return False


class StridedIntervalAnnotation(SimplificationAvoidanceAnnotation):
    """StridedIntervalAnnotation allows annotating a BVS to represent a strided interval."""

    stride: int
    lower_bound: int
    upper_bound: int

    def __init__(self, stride: int, lower_bound: int, upper_bound: int):
        self.stride = stride
        self.lower_bound = lower_bound
        self.upper_bound = upper_bound

    def __hash__(self):
        return hash((self.stride, self.lower_bound, self.upper_bound))

    def __eq__(self, other):
        return (
            isinstance(other, StridedIntervalAnnotation)
            and self.stride == other.stride
            and self.lower_bound == other.lower_bound
            and self.upper_bound == other
        )

    def __repr__(self):
        return f"<StridedIntervalAnnotation {self.stride}:{self.lower_bound} - {self.upper_bound}>"


class RegionAnnotation(SimplificationAvoidanceAnnotation):
    """
    Use RegionAnnotation to annotate ASTs. Normally, an AST annotated by
    RegionAnnotations is treated as a ValueSet.
    """

    def __init__(self, region_id, region_base_addr):
        self.region_id = region_id
        self.region_base_addr = region_base_addr

    #
    # Overriding base methods
    #

    def __hash__(self):
        return hash((self.region_id, self.region_base_addr))

    def __repr__(self):
        return f"<RegionAnnotation {self.region_id}@{self.region_base_addr:#08x}>"


class UninitializedAnnotation(Annotation):
    """
    Use UninitializedAnnotation to annotate ASTs that are uninitialized.
    """

    eliminatable = False
    relocatable = True

    def __hash__(self):
        return hash("uninitialized")

    def __eq__(self, other):
        return isinstance(other, UninitializedAnnotation)

    def __repr__(self):
        return "<UninitializedAnnotation>"
