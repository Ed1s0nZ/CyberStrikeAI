from __future__ import annotations

import math
import numbers
from collections.abc import Sized
from bitstring.exceptions import CreationError
from typing import Union, List, Iterable, Any, Optional, BinaryIO, overload, TextIO
from bitstring.bits import Bits, BitsType
from bitstring.bitarray_ import BitArray
from bitstring.dtypes import Dtype, dtype_register
from bitstring import utils
from bitstring.bitstring_options import Options, Colour
import copy
import array
import operator
import io
import sys

# The possible types stored in each element of the Array
ElementType = Union[float, str, int, bytes, bool, Bits]

options = Options()


class Array:
    """Return an Array whose elements are initialised according to the fmt string.
    The dtype string can be typecode as used in the struct module or any fixed-length bitstring
    format.

    a = Array('>H', [1, 15, 105])
    b = Array('int5', [-9, 0, 4])

    The Array data is stored compactly as a BitArray object and the Array behaves very like
    a list of items of the given format. Both the Array data and fmt properties can be freely
    modified after creation. If the data length is not a multiple of the fmt length then the
    Array will have 'trailing_bits' which will prevent some methods from appending to the
    Array.

    Methods:

    append() -- Append a single item to the end of the Array.
    byteswap() -- Change byte endianness of all items.
    count() -- Count the number of occurences of a value.
    extend() -- Append new items to the end of the Array from an iterable.
    fromfile() -- Append items read from a file object.
    insert() -- Insert an item at a given position.
    pop() -- Remove and return an item.
    pp() -- Pretty print the Array.
    reverse() -- Reverse the order of all items.
    tobytes() -- Return Array data as bytes object, padding with zero bits at the end if needed.
    tofile() -- Write Array data to a file, padding with zero bits at the end if needed.
    tolist() -- Return Array items as a list.

    Special methods:

    Also available are the operators [], ==, !=, +, *, <<, >>, &, |, ^,
    plus the mutating operators [], +=, *=, <<=, >>=, &=, |=, ^=.

    Properties:

    data -- The BitArray binary data of the Array. Can be freely modified.
    dtype -- The format string or typecode. Can be freely modified.
    itemsize -- The length *in bits* of a single item. Read only.
    trailing_bits -- If the data length is not a multiple of the fmt length, this BitArray
                     gives the leftovers at the end of the data.


    """

    def __init__(self, dtype: Union[str, Dtype], initializer: Optional[Union[int, Array, array.array, Iterable, Bits, bytes, bytearray, memoryview, BinaryIO]] = None,
                 trailing_bits: Optional[BitsType] = None) -> None:
        self.data = BitArray()
        if isinstance(dtype, Dtype) and dtype.scale == 'auto':
            if isinstance(initializer, (int, Bits, bytes, bytearray, memoryview, BinaryIO)):
                raise TypeError("An Array with an 'auto' scale factor can only be created from an iterable of values.")
            auto_scale = self._calculate_auto_scale(initializer, dtype.name, dtype.length)
            dtype = Dtype(dtype.name, dtype.length, scale=auto_scale)
        try:
            self._set_dtype(dtype)
        except ValueError as e:
            raise CreationError(e)

        if isinstance(initializer, numbers.Integral):
            self.data = BitArray(initializer * self._dtype.bitlength)
        elif isinstance(initializer, (Bits, bytes, bytearray, memoryview)):
            self.data += initializer
        elif isinstance(initializer, io.BufferedReader):
            self.fromfile(initializer)
        elif initializer is not None:
            self.extend(initializer)

        if trailing_bits is not None:
            self.data += BitArray._create_from_bitstype(trailing_bits)

    _largest_values = None

    @staticmethod
    def _calculate_auto_scale(initializer, name: str, length: Optional[int]) -> float:
        # Now need to find the largest power of 2 representable with this format.
        if Array._largest_values is None:
            Array._largest_values = {
                'mxint8': Bits('0b01111111').mxint8,  # 1.0 + 63.0/64.0,
                'e2m1mxfp4': Bits('0b0111').e2m1mxfp4,  # 6.0
                'e2m3mxfp6': Bits('0b011111').e2m3mxfp6,  # 7.5
                'e3m2mxfp6': Bits('0b011111').e3m2mxfp6,  # 28.0
                'e4m3mxfp8': Bits('0b01111110').e4m3mxfp8,  # 448.0
                'e5m2mxfp8': Bits('0b01111011').e5m2mxfp8,  # 57344.0
                'p4binary8': Bits('0b01111110').p4binary8,  # 224.0
                'p3binary8': Bits('0b01111110').p3binary8,  # 49152.0
                'float16': Bits('0x7bff').float16,  # 65504.0
                # The bfloat range is so large the scaling algorithm doesn't work well, so I'm disallowing it.
                # 'bfloat16': Bits('0x7f7f').bfloat16,  # 3.38953139e38,
            }
        if f'{name}{length}' in Array._largest_values.keys():
            float_values = Array('float64', initializer).tolist()
            if not float_values:
                raise ValueError("Can't calculate an 'auto' scale with an empty Array initializer.")
            max_float_value = max(abs(x) for x in float_values)
            if max_float_value == 0:
                # This special case isn't covered in the standard. I'm choosing to return no scale.
                return 1.0
            # We need to find the largest power of 2 that is less than the max value
            log2 = math.floor(math.log2(max_float_value))
            lp2 = math.floor(math.log2(Array._largest_values[f'{name}{length}']))
            lg_scale = log2 - lp2
            # Saturate at values representable in E8M0 format.
            if lg_scale > 127:
                lg_scale = 127
            elif lg_scale < -127:
                lg_scale = -127
            return 2 ** lg_scale
        else:
            raise ValueError(f"Can't calculate auto scale for format '{name}{length}'. "
                             f"This feature is only available for these formats: {list(Array._largest_values.keys())}.")

    @property
    def itemsize(self) -> int:
        return self._dtype.length

    @property
    def trailing_bits(self) -> BitArray:
        trailing_bit_length = len(self.data) % self._dtype.bitlength
        return BitArray() if trailing_bit_length == 0 else self.data[-trailing_bit_length:]

    @property
    def dtype(self) -> Dtype:
        return self._dtype

    @dtype.setter
    def dtype(self, new_dtype: Union[str, Dtype]) -> None:
        self._set_dtype(new_dtype)

    def _set_dtype(self, new_dtype: Union[str, Dtype]) -> None:
        if isinstance(new_dtype, Dtype):
            self._dtype = new_dtype
        else:
            try:
                dtype = Dtype(new_dtype)
            except ValueError:
                name_length = utils.parse_single_struct_token(new_dtype)
                if name_length is not None:
                    dtype = Dtype(name_length[0], name_length[1])
                else:
                    raise ValueError(f"Inappropriate Dtype for Array: '{new_dtype}'.")
            if dtype.length is None:
                raise ValueError(f"A fixed length format is needed for an Array, received '{new_dtype}'.")
            self._dtype = dtype
        if self._dtype.scale == 'auto':
            raise ValueError("A Dtype with an 'auto' scale factor can only be used when creating a new Array.")

    def _create_element(self, value: ElementType) -> Bits:
        """Create Bits from value according to the token_name and token_length"""
        b = self._dtype.build(value)
        if len(b) != self._dtype.length:
            raise ValueError(f"The value {value!r} has the wrong length for the format '{self._dtype}'.")
        return b

    def __len__(self) -> int:
        return len(self.data) // self._dtype.length

    @overload
    def __getitem__(self, key: slice) -> Array:
        ...

    @overload
    def __getitem__(self, key: int) -> ElementType:
        ...

    def __getitem__(self, key: Union[slice, int]) -> Union[Array, ElementType]:
        if isinstance(key, slice):
            start, stop, step = key.indices(len(self))
            if step != 1:
                d = BitArray()
                for s in range(start * self._dtype.length, stop * self._dtype.length, step * self._dtype.length):
                    d.append(self.data[s: s + self._dtype.length])
                a = self.__class__(self._dtype)
                a.data = d
                return a
            else:
                a = self.__class__(self._dtype)
                a.data = self.data[start * self._dtype.length: stop * self._dtype.length]
                return a
        else:
            if key < 0:
                key += len(self)
            if key < 0 or key >= len(self):
                raise IndexError(f"Index {key} out of range for Array of length {len(self)}.")
            return self._dtype.read_fn(self.data, start=self._dtype.length * key)

    @overload
    def __setitem__(self, key: slice, value: Iterable[ElementType]) -> None:
        ...

    @overload
    def __setitem__(self, key: int, value: ElementType) -> None:
        ...

    def __setitem__(self, key: Union[slice, int], value: Union[Iterable[ElementType], ElementType]) -> None:
        if isinstance(key, slice):
            start, stop, step = key.indices(len(self))
            if not isinstance(value, Iterable):
                raise TypeError("Can only assign an iterable to a slice.")
            if step == 1:
                new_data = BitArray()
                for x in value:
                    new_data += self._create_element(x)
                self.data[start * self._dtype.length: stop * self._dtype.length] = new_data
                return
            items_in_slice = len(range(start, stop, step))
            if not isinstance(value, Sized):
                value = list(value)
            if len(value) == items_in_slice:
                for s, v in zip(range(start, stop, step), value):
                    self.data.overwrite(self._create_element(v), s * self._dtype.length)
            else:
                raise ValueError(f"Can't assign {len(value)} values to an extended slice of length {items_in_slice}.")
        else:
            if key < 0:
                key += len(self)
            if key < 0 or key >= len(self):
                raise IndexError(f"Index {key} out of range for Array of length {len(self)}.")
            start = self._dtype.length * key
            self.data.overwrite(self._create_element(value), start)
            return

    def __delitem__(self, key: Union[slice, int]) -> None:
        if isinstance(key, slice):
            start, stop, step = key.indices(len(self))
            if step == 1:
                self.data.__delitem__(slice(start * self._dtype.length, stop * self._dtype.length))
                return
            # We need to delete from the end or the earlier positions will change
            r = reversed(range(start, stop, step)) if step > 0 else range(start, stop, step)
            for s in r:
                self.data.__delitem__(slice(s * self._dtype.length, (s + 1) * self._dtype.length))
        else:
            if key < 0:
                key += len(self)
            if key < 0 or key >= len(self):
                raise IndexError
            start = self._dtype.length * key
            del self.data[start: start + self._dtype.length]

    def __repr__(self) -> str:
        list_str = f"{self.tolist()}"
        trailing_bit_length = len(self.data) % self._dtype.length
        final_str = "" if trailing_bit_length == 0 else ", trailing_bits=" + repr(
            self.data[-trailing_bit_length:])
        return f"Array('{self._dtype}', {list_str}{final_str})"

    def astype(self, dtype: Union[str, Dtype]) -> Array:
        """Return Array with elements of new dtype, initialised from current Array."""
        new_array = self.__class__(dtype, self.tolist())
        return new_array

    def tolist(self) -> List[ElementType]:
        return [self._dtype.read_fn(self.data, start=start)
                for start in range(0, len(self.data) - self._dtype.length + 1, self._dtype.length)]

    def append(self, x: ElementType) -> None:
        if len(self.data) % self._dtype.length != 0:
            raise ValueError("Cannot append to Array as its length is not a multiple of the format length.")
        self.data += self._create_element(x)

    def extend(self, iterable: Union[Array, array.array, Iterable[Any]]) -> None:
        if len(self.data) % self._dtype.length != 0:
            raise ValueError(f"Cannot extend Array as its data length ({len(self.data)} bits) is not a multiple of the format length ({self._dtype.length} bits).")
        if isinstance(iterable, Array):
            if self._dtype.name != iterable._dtype.name or self._dtype.length != iterable._dtype.length:
                raise TypeError(
                    f"Cannot extend an Array with format '{self._dtype}' from an Array of format '{iterable._dtype}'.")
            # No need to iterate over the elements, we can just append the data
            self.data.append(iterable.data)
        elif isinstance(iterable, array.array):
            # array.array types are always native-endian, hence the '='
            name_value = utils.parse_single_struct_token('=' + iterable.typecode)
            if name_value is None:
                raise ValueError(f"Cannot extend from array with typecode {iterable.typecode}.")
            other_dtype = dtype_register.get_dtype(*name_value, scale=None)
            if self._dtype.name != other_dtype.name or self._dtype.length != other_dtype.length:
                raise ValueError(
                    f"Cannot extend an Array with format '{self._dtype}' from an array with typecode '{iterable.typecode}'.")
            self.data += iterable.tobytes()
        else:
            if isinstance(iterable, str):
                raise TypeError("Can't extend an Array with a str.")
            for item in iterable:
                self.data += self._create_element(item)

    def insert(self, i: int, x: ElementType) -> None:
        """Insert a new element into the Array at position i.

        """
        i = min(i, len(self))  # Inserting beyond len of array inserts at the end (copying standard behaviour)
        self.data.insert(self._create_element(x), i * self._dtype.length)

    def pop(self, i: int = -1) -> ElementType:
        """Return and remove an element of the Array.

        Default is to return and remove the final element.

        """
        if len(self) == 0:
            raise IndexError("Can't pop from an empty Array.")
        x = self[i]
        del self[i]
        return x

    def byteswap(self) -> None:
        """Change the endianness in-place of all items in the Array.

        If the Array format is not a whole number of bytes a ValueError will be raised.

        """
        if self._dtype.length % 8 != 0:
            raise ValueError(
                f"byteswap can only be used for whole-byte elements. The '{self._dtype}' format is {self._dtype.length} bits long.")
        self.data.byteswap(self.itemsize // 8)

    def count(self, value: ElementType) -> int:
        """Return count of Array items that equal value.

        value -- The quantity to compare each Array element to. Type should be appropriate for the Array format.

        For floating point types using a value of float('nan') will count the number of elements that are NaN.

        """
        if math.isnan(value):
            return sum(math.isnan(i) for i in self)
        else:
            return sum(i == value for i in self)

    def tobytes(self) -> bytes:
        """Return the Array data as a bytes object, padding with zero bits if needed.

        Up to seven zero bits will be added at the end to byte align.

        """
        return self.data.tobytes()

    def tofile(self, f: BinaryIO) -> None:
        """Write the Array data to a file object, padding with zero bits if needed.

        Up to seven zero bits will be added at the end to byte align.

        """
        self.data.tofile(f)

    def fromfile(self, f: BinaryIO, n: Optional[int] = None) -> None:
        trailing_bit_length = len(self.data) % self._dtype.bitlength
        if trailing_bit_length != 0:
            raise ValueError(f"Cannot extend Array as its data length ({len(self.data)} bits) is not a multiple of the format length ({self._dtype.bitlength} bits).")

        new_data = Bits(f)
        max_items = len(new_data) // self._dtype.length
        items_to_append = max_items if n is None else min(n, max_items)
        self.data += new_data[0: items_to_append * self._dtype.bitlength]
        if n is not None and items_to_append < n:
            raise EOFError(f"Only {items_to_append} were appended, not the {n} items requested.")

    def reverse(self) -> None:
        trailing_bit_length = len(self.data) % self._dtype.length
        if trailing_bit_length != 0:
            raise ValueError(f"Cannot reverse the items in the Array as its data length ({len(self.data)} bits) is not a multiple of the format length ({self._dtype.length} bits).")
        for start_bit in range(0, len(self.data) // 2, self._dtype.length):
            start_swap_bit = len(self.data) - start_bit - self._dtype.length
            temp = self.data[start_bit: start_bit + self._dtype.length]
            self.data[start_bit: start_bit + self._dtype.length] = self.data[
                                                               start_swap_bit: start_swap_bit + self._dtype.length]
            self.data[start_swap_bit: start_swap_bit + self._dtype.length] = temp

    def pp(self, fmt: Optional[str] = None, width: int = 120,
           show_offset: bool = True, stream: TextIO = sys.stdout) -> None:
        """Pretty-print the Array contents.

        fmt -- Data format string. Defaults to current Array dtype.
        width -- Max width of printed lines in characters. Defaults to 120. A single group will always
                 be printed per line even if it exceeds the max width.
        show_offset -- If True shows the element offset in the first column of each line.
        stream -- A TextIO object with a write() method. Defaults to sys.stdout.

        """
        colour = Colour(not options.no_color)
        sep = ' '
        dtype2 = None
        tidy_fmt = None
        if fmt is None:
            fmt = self.dtype
            dtype1 = self.dtype
            tidy_fmt = "dtype='" + colour.purple + str(self.dtype) + "'" + colour.off
        else:
            token_list = utils.preprocess_tokens(fmt)
            if len(token_list) not in [1, 2]:
                raise ValueError(f"Only one or two tokens can be used in an Array.pp() format - '{fmt}' has {len(token_list)} tokens.")
            name1, length1 = utils.parse_name_length_token(token_list[0])
            dtype1 = Dtype(name1, length1)
            if len(token_list) == 2:
                name2, length2 = utils.parse_name_length_token(token_list[1])
                dtype2 = Dtype(name2, length2)

        token_length = dtype1.bitlength
        if dtype2 is not None:
            # For two types we're OK as long as they don't have different lengths given.
            if dtype1.bitlength is not None and dtype2.bitlength is not None and dtype1.bitlength != dtype2.bitlength:
                raise ValueError(f"Two different format lengths specified ('{fmt}'). Either specify just one, or two the same length.")
            if token_length is None:
                token_length = dtype2.bitlength
        if token_length is None:
            token_length = self.itemsize

        trailing_bit_length = len(self.data) % token_length
        format_sep = " : "  # String to insert on each line between multiple formats
        if tidy_fmt is None:
            tidy_fmt = colour.purple + str(dtype1) + colour.off
            if dtype2 is not None:
                tidy_fmt += ', ' + colour.blue + str(dtype2) + colour.off
            tidy_fmt = "fmt='" + tidy_fmt + "'"
        data = self.data if trailing_bit_length == 0 else self.data[0: -trailing_bit_length]
        length = len(self.data) // token_length
        len_str = colour.green + str(length) + colour.off
        stream.write(f"<{self.__class__.__name__} {tidy_fmt}, length={len_str}, itemsize={token_length} bits, total data size={(len(self.data) + 7) // 8} bytes> [\n")
        data._pp(dtype1, dtype2, token_length, width, sep, format_sep, show_offset, stream, False, token_length)
        stream.write("]")
        if trailing_bit_length != 0:
            stream.write(" + trailing_bits = " + str(self.data[-trailing_bit_length:]))
        stream.write("\n")

    def equals(self, other: Any) -> bool:
        """Return True if format and all Array items are equal."""
        if isinstance(other, Array):
            if self._dtype.length != other._dtype.length:
                return False
            if self._dtype.name != other._dtype.name:
                return False
            if self.data != other.data:
                return False
            return True
        elif isinstance(other, array.array):
            # Assume we are comparing with an array type
            if self.trailing_bits:
                return False
            # array's itemsize is in bytes, not bits.
            if self.itemsize != other.itemsize * 8:
                return False
            if len(self) != len(other):
                return False
            if self.tolist() != other.tolist():
                return False
            return True
        return False

    def __iter__(self) -> Iterable[ElementType]:
        start = 0
        for _ in range(len(self)):
            yield self._dtype.read_fn(self.data, start=start)
            start += self._dtype.length

    def __copy__(self) -> Array:
        a_copy = self.__class__(self._dtype)
        a_copy.data = copy.copy(self.data)
        return a_copy

    def _apply_op_to_all_elements(self, op, value: Union[int, float, None], is_comparison: bool = False) -> Array:
        """Apply op with value to each element of the Array and return a new Array"""
        new_array = self.__class__('bool' if is_comparison else self._dtype)
        new_data = BitArray()
        failures = index = 0
        msg = ''
        if value is not None:
            def partial_op(a):
                return op(a, value)
        else:
            def partial_op(a):
                return op(a)
        for i in range(len(self)):
            v = self._dtype.read_fn(self.data, start=self._dtype.length * i)
            try:
                new_data.append(new_array._create_element(partial_op(v)))
            except (CreationError, ZeroDivisionError, ValueError) as e:
                if failures == 0:
                    msg = str(e)
                    index = i
                failures += 1
        if failures != 0:
            raise ValueError(f"Applying operator '{op.__name__}' to Array caused {failures} errors. "
                             f'First error at index {index} was: "{msg}"')
        new_array.data = new_data
        return new_array

    def _apply_op_to_all_elements_inplace(self, op, value: Union[int, float]) -> Array:
        """Apply op with value to each element of the Array in place."""
        # This isn't really being done in-place, but it's simpler and faster for now?
        new_data = BitArray()
        failures = index = 0
        msg = ''
        for i in range(len(self)):
            v = self._dtype.read_fn(self.data, start=self._dtype.length * i)
            try:
                new_data.append(self._create_element(op(v, value)))
            except (CreationError, ZeroDivisionError, ValueError) as e:
                if failures == 0:
                    msg = str(e)
                    index = i
                failures += 1
        if failures != 0:
            raise ValueError(f"Applying operator '{op.__name__}' to Array caused {failures} errors. "
                             f'First error at index {index} was: "{msg}"')
        self.data = new_data
        return self

    def _apply_bitwise_op_to_all_elements(self, op, value: BitsType) -> Array:
        """Apply op with value to each element of the Array as an unsigned integer and return a new Array"""
        a_copy = self[:]
        a_copy._apply_bitwise_op_to_all_elements_inplace(op, value)
        return a_copy

    def _apply_bitwise_op_to_all_elements_inplace(self, op, value: BitsType) -> Array:
        """Apply op with value to each element of the Array as an unsigned integer in place."""
        value = BitArray._create_from_bitstype(value)
        if len(value) != self._dtype.length:
            raise ValueError(f"Bitwise op needs a bitstring of length {self._dtype.length} to match format {self._dtype}.")
        for start in range(0, len(self) * self._dtype.length, self._dtype.length):
            self.data[start: start + self._dtype.length] = op(self.data[start: start + self._dtype.length], value)
        return self

    def _apply_op_between_arrays(self, op, other: Array, is_comparison: bool = False) -> Array:
        if len(self) != len(other):
            msg = f"Cannot operate element-wise on Arrays with different lengths ({len(self)} and {len(other)})."
            if op in [operator.add, operator.iadd]:
                msg += " Use extend() method to concatenate Arrays."
            if op in [operator.eq, operator.ne]:
                msg += " Use equals() method to compare Arrays for a single boolean result."
            raise ValueError(msg)
        if is_comparison:
            new_type = dtype_register.get_dtype('bool', 1)
        else:
            new_type = self._promotetype(self._dtype, other._dtype)
        new_array = self.__class__(new_type)
        new_data = BitArray()
        failures = index = 0
        msg = ''
        for i in range(len(self)):
            a = self._dtype.read_fn(self.data, start=self._dtype.length * i)
            b = other._dtype.read_fn(other.data, start=other._dtype.length * i)
            try:
                new_data.append(new_array._create_element(op(a, b)))
            except (CreationError, ValueError, ZeroDivisionError) as e:
                if failures == 0:
                    msg = str(e)
                    index = i
                failures += 1
        if failures != 0:
            raise ValueError(f"Applying operator '{op.__name__}' between Arrays caused {failures} errors. "
                             f'First error at index {index} was: "{msg}"')
        new_array.data = new_data
        return new_array

    @classmethod
    def _promotetype(cls, type1: Dtype, type2: Dtype) -> Dtype:
        """When combining types which one wins?

        1. We only deal with types representing floats or integers.
        2. One of the two types gets returned. We never create a new one.
        3. Floating point types always win against integer types.
        4. Signed integer types always win against unsigned integer types.
        5. Longer types win against shorter types.
        6. In a tie the first type wins against the second type.

        """
        def is_float(x): return x.return_type is float
        def is_int(x): return x.return_type is int or x.return_type is bool
        if is_float(type1) + is_int(type1) + is_float(type2) + is_int(type2) != 2:
            raise ValueError(f"Only integer and floating point types can be combined - not '{type1}' and '{type2}'.")
        # If same type choose the widest
        if type1.name == type2.name:
            return type1 if type1.length > type2.length else type2
        # We choose floats above integers, irrespective of the widths
        if is_float(type1) and is_int(type2):
            return type1
        if is_int(type1) and is_float(type2):
            return type2
        if is_float(type1) and is_float(type2):
            return type2 if type2.length > type1.length else type1
        assert is_int(type1) and is_int(type2)
        if type1.is_signed and not type2.is_signed:
            return type1
        if type2.is_signed and not type1.is_signed:
            return type2
        return type2 if type2.length > type1.length else type1

    # Operators between Arrays or an Array and scalar value

    def __add__(self, other: Union[int, float, Array]) -> Array:
        """Add int or float to all elements."""
        if isinstance(other, Array):
            return self._apply_op_between_arrays(operator.add, other)
        return self._apply_op_to_all_elements(operator.add, other)

    def __iadd__(self, other: Union[int, float, Array]) -> Array:
        if isinstance(other, Array):
            return self._apply_op_between_arrays(operator.add, other)
        return self._apply_op_to_all_elements_inplace(operator.add, other)

    def __isub__(self, other: Union[int, float, Array]) -> Array:
        if isinstance(other, Array):
            return self._apply_op_between_arrays(operator.sub, other)
        return self._apply_op_to_all_elements_inplace(operator.sub, other)

    def __sub__(self, other: Union[int, float, Array]) -> Array:
        if isinstance(other, Array):
            return self._apply_op_between_arrays(operator.sub, other)
        return self._apply_op_to_all_elements(operator.sub, other)

    def __mul__(self, other: Union[int, float, Array]) -> Array:
        if isinstance(other, Array):
            return self._apply_op_between_arrays(operator.mul, other)
        return self._apply_op_to_all_elements(operator.mul, other)

    def __imul__(self, other: Union[int, float, Array]) -> Array:
        if isinstance(other, Array):
            return self._apply_op_between_arrays(operator.mul, other)
        return self._apply_op_to_all_elements_inplace(operator.mul, other)

    def __floordiv__(self, other: Union[int, float, Array]) -> Array:
        if isinstance(other, Array):
            return self._apply_op_between_arrays(operator.floordiv, other)
        return self._apply_op_to_all_elements(operator.floordiv, other)

    def __ifloordiv__(self, other: Union[int, float, Array]) -> Array:
        if isinstance(other, Array):
            return self._apply_op_between_arrays(operator.floordiv, other)
        return self._apply_op_to_all_elements_inplace(operator.floordiv, other)

    def __truediv__(self, other: Union[int, float, Array]) -> Array:
        if isinstance(other, Array):
            return self._apply_op_between_arrays(operator.truediv, other)
        return self._apply_op_to_all_elements(operator.truediv, other)

    def __itruediv__(self, other: Union[int, float, Array]) -> Array:
        if isinstance(other, Array):
            return self._apply_op_between_arrays(operator.truediv, other)
        return self._apply_op_to_all_elements_inplace(operator.truediv, other)

    def __rshift__(self, other: Union[int, Array]) -> Array:
        if isinstance(other, Array):
            return self._apply_op_between_arrays(operator.rshift, other)
        return self._apply_op_to_all_elements(operator.rshift, other)

    def __lshift__(self, other: Union[int, Array]) -> Array:
        if isinstance(other, Array):
            return self._apply_op_between_arrays(operator.lshift, other)
        return self._apply_op_to_all_elements(operator.lshift, other)

    def __irshift__(self, other: Union[int, Array]) -> Array:
        if isinstance(other, Array):
            return self._apply_op_between_arrays(operator.rshift, other)
        return self._apply_op_to_all_elements_inplace(operator.rshift, other)

    def __ilshift__(self, other: Union[int, Array]) -> Array:
        if isinstance(other, Array):
            return self._apply_op_between_arrays(operator.lshift, other)
        return self._apply_op_to_all_elements_inplace(operator.lshift, other)

    def __mod__(self, other: Union[int, Array]) -> Array:
        if isinstance(other, Array):
            return self._apply_op_between_arrays(operator.mod, other)
        return self._apply_op_to_all_elements(operator.mod, other)

    def __imod__(self, other: Union[int, Array]) -> Array:
        if isinstance(other, Array):
            return self._apply_op_between_arrays(operator.mod, other)
        return self._apply_op_to_all_elements_inplace(operator.mod, other)

    # Bitwise operators

    def __and__(self, other: BitsType) -> Array:
        return self._apply_bitwise_op_to_all_elements(operator.iand, other)

    def __iand__(self, other: BitsType) -> Array:
        return self._apply_bitwise_op_to_all_elements_inplace(operator.iand, other)

    def __or__(self, other: BitsType) -> Array:
        return self._apply_bitwise_op_to_all_elements(operator.ior, other)

    def __ior__(self, other: BitsType) -> Array:
        return self._apply_bitwise_op_to_all_elements_inplace(operator.ior, other)

    def __xor__(self, other: BitsType) -> Array:
        return self._apply_bitwise_op_to_all_elements(operator.ixor, other)

    def __ixor__(self, other: BitsType) -> Array:
        return self._apply_bitwise_op_to_all_elements_inplace(operator.ixor, other)

    # Reverse operators between a scalar value and an Array

    def __rmul__(self, other: Union[int, float]) -> Array:
        return self._apply_op_to_all_elements(operator.mul, other)

    def __radd__(self, other: Union[int, float]) -> Array:
        return self._apply_op_to_all_elements(operator.add, other)

    def __rsub__(self, other: Union[int, float]) -> Array:
        # i - A == (-A) + i
        neg = self._apply_op_to_all_elements(operator.neg, None)
        return neg._apply_op_to_all_elements(operator.add, other)

    # Reverse operators between a scalar and something that can be a BitArray.

    def __rand__(self, other: BitsType) -> Array:
        return self._apply_bitwise_op_to_all_elements(operator.iand, other)

    def __ror__(self, other: BitsType) -> Array:
        return self._apply_bitwise_op_to_all_elements(operator.ior, other)

    def __rxor__(self, other: BitsType) -> Array:
        return self._apply_bitwise_op_to_all_elements(operator.ixor, other)

    # Comparison operators

    def __lt__(self, other: Union[int, float, Array]) -> Array:
        if isinstance(other, Array):
            return self._apply_op_between_arrays(operator.lt, other, is_comparison=True)
        return self._apply_op_to_all_elements(operator.lt, other, is_comparison=True)

    def __gt__(self, other: Union[int, float, Array]) -> Array:
        if isinstance(other, Array):
            return self._apply_op_between_arrays(operator.gt, other, is_comparison=True)
        return self._apply_op_to_all_elements(operator.gt, other, is_comparison=True)

    def __ge__(self, other: Union[int, float, Array]) -> Array:
        if isinstance(other, Array):
            return self._apply_op_between_arrays(operator.ge, other, is_comparison=True)
        return self._apply_op_to_all_elements(operator.ge, other, is_comparison=True)

    def __le__(self, other: Union[int, float, Array]) -> Array:
        if isinstance(other, Array):
            return self._apply_op_between_arrays(operator.le, other, is_comparison=True)
        return self._apply_op_to_all_elements(operator.le, other, is_comparison=True)

    def _eq_ne(self, op, other: Any) -> Array:
        if isinstance(other, (int, float, str, Bits)):
            return self._apply_op_to_all_elements(op, other, is_comparison=True)
        other = self.__class__(self.dtype, other)
        return self._apply_op_between_arrays(op, other, is_comparison=True)

    def __eq__(self, other: Any) -> Array:
        return self._eq_ne(operator.eq, other)

    def __ne__(self, other: Any) -> Array:
        return self._eq_ne(operator.ne, other)

    # Unary operators

    def __neg__(self):
        return self._apply_op_to_all_elements(operator.neg, None)

    def __abs__(self):
        return self._apply_op_to_all_elements(operator.abs, None)