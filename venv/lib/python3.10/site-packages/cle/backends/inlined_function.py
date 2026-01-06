from __future__ import annotations

from dataclasses import dataclass, field


@dataclass
class InlinedFunction:
    """
    A representation of a piece of a function which is inlined from another function.
    """

    name: str | None = None
    ranges: list[tuple[int, int]] = field(default_factory=list)

    @property
    def low_pc(self):
        return min(x for x, _ in self.ranges)

    @property
    def high_pc(self):
        return max(x for _, x in self.ranges)
