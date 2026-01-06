from __future__ import annotations

from collections.abc import Iterator
from typing import Generic, TypeVar

from cle.utils import key_bisect_find, key_bisect_insort_left

from .region import Region

R = TypeVar("R", bound=Region)


class Regions(Generic[R]):
    """
    A container class acting as a list of regions (sections or segments). Additionally, it keeps an sorted list of
    all regions that are mapped into memory to allow fast lookups.

    We assume none of the regions overlap with others.
    """

    def __init__(self, lst: list[R] | None = None):
        self._list = lst if lst is not None else []

        if self._list:
            self._sorted_list: list[R] = self._make_sorted(self._list)
        else:
            self._sorted_list = []

    @property
    def raw_list(self) -> list[R]:
        """
        Get the internal list. Any change to it is not tracked, and therefore _sorted_list will not be updated.
        Therefore you probably does not want to modify the list.

        :return:  The internal list container.
        :rtype:   list
        """

        return self._list

    @property
    def max_addr(self) -> int | None:
        """
        Get the highest address of all regions.

        :return: The highest address of all regions, or None if there is no region available.
        :rtype:   int or None
        """

        if self._sorted_list:
            return self._sorted_list[-1].max_addr
        return None

    def __getitem__(self, idx: int) -> R:
        return self._list[idx]

    def __setitem__(self, idx: int, item: R) -> None:
        self._list[idx] = item

        # update self._sorted_list
        self._sorted_list = self._make_sorted(self._list)

    def __iter__(self) -> Iterator[R]:
        return iter(self._list)

    def __len__(self) -> int:
        return len(self._list)

    def __repr__(self):
        return f"<Regions: {repr(self._list)}>"

    def _rebase(self, delta):
        """
        Does regions rebasing to other base address.
        Modifies state of each internal object, so the list reference doesn't need to be updated,
        the same is also valid for sorted list as operation preserves the ordering.

        :param delta: Delta offset between an old and a new image bases
        :type delta: int
        """
        for x in self._list:
            x._rebase(delta)

    def append(self, region: R):
        """
        Append a new Region instance into the list.

        :param region: The region to append.
        """
        self._list.append(region)

        if self._is_region_mapped(region):
            key_bisect_insort_left(self._sorted_list, region, keyfunc=lambda x: x.vaddr)

    def remove(self, region: R) -> None:
        """
        Remove an existing Region instance from the list.

        :param region: The region to remove.
        """
        if self._is_region_mapped(region):
            self._sorted_list.remove(region)
        self._list.remove(region)

    def find_region_containing(self, addr: int) -> R | None:
        """
        Find the region that contains a specific address. Returns None if none of the regions covers the address.

        :param addr:    The address.
        :return:        The region that covers the specific address, or None if no such region is found.
        """

        pos = key_bisect_find(
            self._sorted_list, addr, keyfunc=lambda x: x if isinstance(x, int) else x.vaddr + x.memsize
        )
        if pos >= len(self._sorted_list):
            return None
        region = self._sorted_list[pos]
        if region.contains_addr(addr):
            return region
        return None

    def find_region_next_to(self, addr: int) -> R | None:
        """
        Find the next region after the given address.

        :param addr: The address to test.
        :return:         The next region that goes after the given address, or None if there is no section after the
                         address,
        """

        pos = key_bisect_find(
            self._sorted_list, addr, keyfunc=lambda x: x if isinstance(x, int) else x.vaddr + x.memsize
        )
        if pos >= len(self._sorted_list):
            return None

        return self._sorted_list[pos]

    @staticmethod
    def _is_region_mapped(region: R) -> bool:
        # delayed import
        # pylint: disable=import-outside-toplevel
        from .elf.regions import ELFSection

        mapped = True
        if region.memsize == 0:
            mapped = False
        elif isinstance(region, ELFSection) and not region.occupies_memory:
            mapped = False
        return mapped

    @staticmethod
    def _make_sorted(lst: list[R]) -> list[R]:
        """
        Return a sorted list of regions that are mapped into memory.

        :param lst:       A list of regions.
        :return:          A sorted list of regions.
        :rtype:           list
        """

        return sorted([r for r in lst if Regions._is_region_mapped(r)], key=lambda x: x.vaddr)
