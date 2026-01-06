from __future__ import annotations

import bitarray
from bitstring.exceptions import CreationError
from typing import Union, Iterable, Optional, overload, Iterator, Any

if bitarray.__version__.startswith("2."):
    raise ImportError(f"bitstring version 4.3 requires bitarray version 3 or higher. Found version {bitarray.__version__}.")

def indices(s: slice, length: int) -> tuple[int, int | None, int]:
    """A better implementation of slice.indices such that a
    slice made from [start:stop:step] will actually equal the original slice."""
    if s.step is None or s.step > 0:
        return s.indices(length)
    assert s.step < 0
    start, stop, step = s.indices(length)
    if stop < 0:
        stop = None
    return start, stop, step

def offset_slice_indices_lsb0(key: slice, length: int) -> slice:
    start, stop, step = indices(key, length)
    if step is not None and step < 0:
        if stop is None:
            new_start = start + 1
            new_stop = None
        else:
            first_element = start
            last_element = start + ((stop + 1 - start) // step) * step
            new_start = length - last_element
            new_stop = length - first_element - 1
    else:
        first_element = start
        # The last element will usually be stop - 1, but needs to be adjusted if step != 1.
        last_element = start + ((stop - 1 - start) // step) * step
        new_start = length - last_element - 1
        new_stop = length - first_element
    return slice(new_start, new_stop, key.step)


class BitStore:
    """A light wrapper around bitarray that does the LSB0 stuff"""

    __slots__ = ('_bitarray', 'modified_length', 'immutable')

    def __init__(self, initializer: Union[int, bitarray.bitarray, str, None] = None,
                 immutable: bool = False) -> None:
        self._bitarray = bitarray.bitarray(initializer)
        self.immutable = immutable
        self.modified_length = None

    @classmethod
    def frombytes(cls, b: Union[bytes, bytearray, memoryview], /) -> BitStore:
        x = super().__new__(cls)
        x._bitarray = bitarray.bitarray()
        x._bitarray.frombytes(b)
        x.immutable = False
        x.modified_length = None
        return x

    @classmethod
    def frombuffer(cls, buffer, /, length: Optional[int] = None) -> BitStore:
        x = super().__new__(cls)
        x._bitarray = bitarray.bitarray(buffer=buffer)
        x.immutable = True
        x.modified_length = length
        # Here 'modified' means it shouldn't be changed further, so setting, deleting etc. are disallowed.
        if x.modified_length is not None:
            if x.modified_length < 0:
                raise CreationError("Can't create bitstring with a negative length.")
            if x.modified_length > len(x._bitarray):
                raise CreationError(
                    f"Can't create bitstring with a length of {x.modified_length} from {len(x._bitarray)} bits of data.")
        return x

    def setall(self, value: int, /) -> None:
        self._bitarray.setall(value)

    def tobytes(self) -> bytes:
        if self.modified_length is not None:
            return self._bitarray[:self.modified_length].tobytes()
        return self._bitarray.tobytes()

    def slice_to_uint(self, start: Optional[int] = None, end: Optional[int] = None) -> int:
        return bitarray.util.ba2int(self.getslice(start, end)._bitarray, signed=False)

    def slice_to_int(self, start: Optional[int] = None, end: Optional[int] = None) -> int:
        return bitarray.util.ba2int(self.getslice(start, end)._bitarray, signed=True)

    def slice_to_hex(self, start: Optional[int] = None, end: Optional[int] = None) -> str:
        return bitarray.util.ba2hex(self.getslice(start, end)._bitarray)

    def slice_to_bin(self, start: Optional[int] = None, end: Optional[int] = None) -> str:
        return self.getslice(start, end)._bitarray.to01()

    def slice_to_oct(self, start: Optional[int] = None, end: Optional[int] = None) -> str:
        return bitarray.util.ba2base(8, self.getslice(start, end)._bitarray)

    def __iadd__(self, other: BitStore, /) -> BitStore:
        self._bitarray += other._bitarray
        return self

    def __add__(self, other: BitStore, /) -> BitStore:
        bs = self._copy()
        bs += other
        return bs

    def __eq__(self, other: Any, /) -> bool:
        return self._bitarray == other._bitarray

    def __and__(self, other: BitStore, /) -> BitStore:
        return BitStore(self._bitarray & other._bitarray)

    def __or__(self, other: BitStore, /) -> BitStore:
        return BitStore(self._bitarray | other._bitarray)

    def __xor__(self, other: BitStore, /) -> BitStore:
        return BitStore(self._bitarray ^ other._bitarray)

    def __iand__(self, other: BitStore, /) -> BitStore:
        self._bitarray &= other._bitarray
        return self

    def __ior__(self, other: BitStore, /) -> BitStore:
        self._bitarray |= other._bitarray
        return self

    def __ixor__(self, other: BitStore, /) -> BitStore:
        self._bitarray ^= other._bitarray
        return self

    def find(self, bs: BitStore, start: int, end: int, bytealigned: bool = False) -> int:
        if not bytealigned:
            return self._bitarray.find(bs._bitarray, start, end)
        try:
            return next(self.findall_msb0(bs, start, end, bytealigned))
        except StopIteration:
            return -1

    def rfind(self, bs: BitStore, start: int, end: int, bytealigned: bool = False):
        if not bytealigned:
            return self._bitarray.find(bs._bitarray, start, end, right=True)
        try:
            return next(self.rfindall_msb0(bs, start, end, bytealigned))
        except StopIteration:
            return -1

    def findall_msb0(self, bs: BitStore, start: int, end: int, bytealigned: bool = False) -> Iterator[int]:
        if bytealigned is True and len(bs) % 8 == 0:
            # Special case, looking for whole bytes on whole byte boundaries
            bytes_ = bs.tobytes()
            # Round up start byte to next byte, and round end byte down.
            # We're only looking for whole bytes, so can ignore bits at either end.
            start_byte = (start + 7) // 8
            end_byte = end // 8
            b = self._bitarray[start_byte * 8: end_byte * 8].tobytes()
            byte_pos = 0
            bytes_to_search = end_byte - start_byte
            while byte_pos < bytes_to_search:
                byte_pos = b.find(bytes_, byte_pos)
                if byte_pos == -1:
                    break
                yield (byte_pos + start_byte) * 8
                byte_pos = byte_pos + 1
            return
        # General case
        i = self._bitarray.search(bs._bitarray, start, end)
        if not bytealigned:
            for p in i:
                yield p
        else:
            for p in i:
                if (p % 8) == 0:
                    yield p

    def rfindall_msb0(self, bs: BitStore, start: int, end: int, bytealigned: bool = False) -> Iterator[int]:
        i = self._bitarray.search(bs._bitarray, start, end, right=True)
        if not bytealigned:
            for p in i:
                yield p
        else:
            for p in i:
                if (p % 8) == 0:
                    yield p

    def count(self, value, /) -> int:
        return self._bitarray.count(value)

    def clear(self) -> None:
        self._bitarray.clear()

    def reverse(self) -> None:
        self._bitarray.reverse()

    def __iter__(self) -> Iterable[bool]:
        for i in range(len(self)):
            yield self.getindex(i)

    def _copy(self) -> BitStore:
        """Always creates a copy, even if instance is immutable."""
        return BitStore(self._bitarray)

    def copy(self) -> BitStore:
        return self if self.immutable else self._copy()

    def __getitem__(self, item: Union[int, slice], /) -> Union[int, BitStore]:
        # Use getindex or getslice instead
        raise NotImplementedError

    def getindex_msb0(self, index: int, /) -> bool:
        return bool(self._bitarray.__getitem__(index))

    def getslice_withstep_msb0(self, key: slice, /) -> BitStore:
        if self.modified_length is not None:
            key = slice(*key.indices(self.modified_length))
        return BitStore(self._bitarray.__getitem__(key))

    def getslice_withstep_lsb0(self, key: slice, /) -> BitStore:
        key = offset_slice_indices_lsb0(key, len(self))
        return BitStore(self._bitarray.__getitem__(key))

    def getslice_msb0(self, start: Optional[int], stop: Optional[int], /) -> BitStore:
        if self.modified_length is not None:
            key = slice(*slice(start, stop, None).indices(self.modified_length))
            start = key.start
            stop = key.stop
        return BitStore(self._bitarray[start:stop])

    def getslice_lsb0(self, start: Optional[int], stop: Optional[int], /) -> BitStore:
        s = offset_slice_indices_lsb0(slice(start, stop, None), len(self))
        return BitStore(self._bitarray[s.start:s.stop])

    def getindex_lsb0(self, index: int, /) -> bool:
        return bool(self._bitarray.__getitem__(-index - 1))

    @overload
    def setitem_lsb0(self, key: int, value: int, /) -> None:
        ...

    @overload
    def setitem_lsb0(self, key: slice, value: BitStore, /) -> None:
        ...

    def setitem_lsb0(self, key: Union[int, slice], value: Union[int, BitStore], /) -> None:
        if isinstance(key, slice):
            new_slice = offset_slice_indices_lsb0(key, len(self))
            self._bitarray.__setitem__(new_slice, value._bitarray)
        else:
            self._bitarray.__setitem__(-key - 1, value)

    def delitem_lsb0(self, key: Union[int, slice], /) -> None:
        if isinstance(key, slice):
            new_slice = offset_slice_indices_lsb0(key, len(self))
            self._bitarray.__delitem__(new_slice)
        else:
            self._bitarray.__delitem__(-key - 1)

    def invert_msb0(self, index: Optional[int] = None, /) -> None:
        if index is not None:
            self._bitarray.invert(index)
        else:
            self._bitarray.invert()

    def invert_lsb0(self, index: Optional[int] = None, /) -> None:
        if index is not None:
            self._bitarray.invert(-index - 1)
        else:
            self._bitarray.invert()

    def any_set(self) -> bool:
        return self._bitarray.any()

    def all_set(self) -> bool:
        return self._bitarray.all()

    def __len__(self) -> int:
        return self.modified_length if self.modified_length is not None else len(self._bitarray)

    def setitem_msb0(self, key, value, /):
        if isinstance(value, BitStore):
            self._bitarray.__setitem__(key, value._bitarray)
        else:
            self._bitarray.__setitem__(key, value)

    def delitem_msb0(self, key, /):
        self._bitarray.__delitem__(key)
