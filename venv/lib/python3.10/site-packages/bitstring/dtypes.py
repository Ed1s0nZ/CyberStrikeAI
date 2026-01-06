from __future__ import annotations

import functools
from typing import Optional, Dict, Any, Union, Tuple, Callable
import inspect
import bitstring
from bitstring import utils

CACHE_SIZE = 256


def scaled_get_fn(get_fn, s: Union[int, float]):
    def wrapper(*args, scale=s, **kwargs):
        return get_fn(*args, **kwargs) * scale
    return wrapper


def scaled_set_fn(set_fn, s: Union[int, float]):
    def wrapper(bs, value, *args, scale=s, **kwargs):
        return set_fn(bs, value / scale, *args, **kwargs)
    return wrapper


def scaled_read_fn(read_fn, s: Union[int, float]):
    def wrapper(*args, scale=s, **kwargs):
        val = read_fn(*args, **kwargs)
        if isinstance(val, tuple):
            val, pos = val
            return val * scale, pos
        return val * scale
    return wrapper


class Dtype:
    """A data type class, representing a concrete interpretation of binary data.

    Dtype instances are immutable. They are often created implicitly elsewhere via a token string.

    >>> u12 = Dtype('uint', 12)  # length separate from token string.
    >>> float16 = Dtype('float16')  # length part of token string.
    >>> mxfp = Dtype('e3m2mxfp', scale=2 ** 6)  # dtype with scaling factor

    """

    _name: str
    _read_fn: Callable
    _set_fn: Callable
    _get_fn: Callable
    _return_type: Any
    _is_signed: bool
    _set_fn_needs_length: bool
    _variable_length: bool
    _bitlength: Optional[int]
    _bits_per_item: int
    _length: Optional[int]
    _scale: Union[None, float, int]

    def __new__(cls, token: Union[str, Dtype], /, length: Optional[int] = None, scale: Union[None, float, int] = None) -> Dtype:
        if isinstance(token, cls):
            return token
        if length is None:
            x = cls._new_from_token(token, scale)
            return x
        else:
            x = dtype_register.get_dtype(token, length, scale)
            return x

    @property
    def scale(self) -> Union[int, float, None]:
        """The multiplicative scale applied when interpreting the data."""
        return self._scale

    @property
    def name(self) -> str:
        """A string giving the name of the data type."""
        return self._name

    @property
    def length(self) -> int:
        """The length of the data type in units of bits_per_item. Set to None for variable length dtypes."""
        return self._length

    @property
    def bitlength(self) -> Optional[int]:
        """The number of bits needed to represent a single instance of the data type. Set to None for variable length dtypes."""
        return self._bitlength

    @property
    def bits_per_item(self) -> int:
        """The number of bits for each unit of length. Usually 1, but equals 8 for bytes type."""
        return self._bits_per_item

    @property
    def variable_length(self) -> bool:
        """If True then the length of the data type depends on the data being interpreted, and must not be specified."""
        return self._variable_length

    @property
    def return_type(self) -> Any:
        """The type of the value returned by the parse method, such as int, float or str."""
        return self._return_type

    @property
    def is_signed(self) -> bool:
        """If True then the data type represents a signed quantity."""
        return self._is_signed

    @property
    def set_fn(self) -> Optional[Callable]:
        """A function to set the value of the data type."""
        return self._set_fn

    @property
    def get_fn(self) -> Callable:
        """A function to get the value of the data type."""
        return self._get_fn

    @property
    def read_fn(self) -> Callable:
        """A function to read the value of the data type."""
        return self._read_fn

    def _set_scale(self, value: Union[None, float, int]) -> None:
        self._scale = value
        if self._scale is None:
            return
        if self._scale == 0:
            raise ValueError("A Dtype's scale factor must not be zero.")
        if not hasattr(self, 'unscaled_get_fn'):
            self.unscaled_get_fn = self._get_fn
            self.unscaled_set_fn = self._set_fn
            self.unscaled_read_fn = self._read_fn
        self._get_fn = scaled_get_fn(self.unscaled_get_fn, self._scale)
        self._set_fn = scaled_set_fn(self.unscaled_set_fn, self._scale)
        self._read_fn = scaled_read_fn(self.unscaled_read_fn, self._scale)

    @classmethod
    @functools.lru_cache(CACHE_SIZE)
    def _new_from_token(cls, token: str, scale: Union[None, float, int] = None) -> Dtype:
        token = ''.join(token.split())
        return dtype_register.get_dtype(*utils.parse_name_length_token(token), scale=scale)

    def __hash__(self) -> int:
        return hash((self._name, self._length))

    @classmethod
    @functools.lru_cache(CACHE_SIZE)
    def _create(cls, definition: DtypeDefinition, length: Optional[int], scale: Union[None, float, int]) -> Dtype:
        x = super().__new__(cls)
        x._name = definition.name
        x._bitlength = x._length = length
        x._bits_per_item = definition.multiplier
        if x._bitlength is not None:
            x._bitlength *= x._bits_per_item
        x._set_fn_needs_length = definition.set_fn_needs_length
        x._variable_length = definition.variable_length
        if x._variable_length or dtype_register.names[x._name].allowed_lengths.only_one_value():
            x._read_fn = definition.read_fn
        else:
            x._read_fn = functools.partial(definition.read_fn, length=x._bitlength)
        if definition.set_fn is None:
            x._set_fn = None
        else:
            if x._set_fn_needs_length:
                x._set_fn = functools.partial(definition.set_fn, length=x._bitlength)
            else:
                x._set_fn = definition.set_fn
        x._get_fn = definition.get_fn
        x._return_type = definition.return_type
        x._is_signed = definition.is_signed
        x._set_scale(scale)
        return x

    def build(self, value: Any, /) -> bitstring.Bits:
        """Create a bitstring from a value.

        The value parameter should be of a type appropriate to the dtype.
        """
        b = bitstring.Bits()
        self._set_fn(b, value)
        if self.bitlength is not None and len(b) != self.bitlength:
            raise ValueError(f"Dtype has a length of {self.bitlength} bits, but value '{value}' has {len(b)} bits.")
        return b

    def parse(self, b: BitsType, /) -> Any:
        """Parse a bitstring to find its value.

        The b parameter should be a bitstring of the appropriate length, or an object that can be converted to a bitstring."""
        b = bitstring.Bits._create_from_bitstype(b)
        return self._get_fn(bitstring.Bits(b))

    def __str__(self) -> str:
        if self._scale is not None:
            return self.__repr__()
        hide_length = self._variable_length or dtype_register.names[self._name].allowed_lengths.only_one_value() or self._length is None
        length_str = '' if hide_length else str(self._length)
        return f"{self._name}{length_str}"

    def __repr__(self) -> str:
        hide_length = self._variable_length or dtype_register.names[self._name].allowed_lengths.only_one_value() or self._length is None
        length_str = '' if hide_length else ', ' + str(self._length)
        if self._scale is None:
            scale_str = ''
        else:
            try:
                # This will only succeed for powers of two from -127 to 127.
                e8m0 = bitstring.Bits(e8m0mxfp=self._scale)
            except ValueError:
                scale_str = f', scale={self._scale}'
            else:
                power_of_two = e8m0.uint - 127
                if power_of_two in [0, 1]:
                    scale_str = f', scale={self._scale}'
                else:
                    scale_str = f', scale=2 ** {power_of_two}'
        return f"{self.__class__.__name__}('{self._name}'{length_str}{scale_str})"

    def __eq__(self, other: Any) -> bool:
        if isinstance(other, Dtype):
            return self._name == other._name and self._length == other._length
        return False


class AllowedLengths:
    def __init__(self, value: Tuple[int, ...] = tuple()) -> None:
        if len(value) >= 3 and value[-1] is Ellipsis:
            step = value[1] - value[0]
            for i in range(1, len(value) - 1):
                if value[i] - value[i - 1] != step:
                    raise ValueError(f"Allowed length tuples must be equally spaced when final element is Ellipsis, but got {value}.")
            self.values = (value[0], value[1], Ellipsis)
        else:
            self.values = value

    def __str__(self) -> str:
        if self.values and self.values[-1] is Ellipsis:
            return f"({self.values[0]}, {self.values[1]}, ...)"
        return str(self.values)

    def __contains__(self, other: Any) -> bool:
        if not self.values:
            return True
        if self.values[-1] is Ellipsis:
            return (other - self.values[0]) % (self.values[1] - self.values[0]) == 0
        return other in self.values

    def only_one_value(self) -> bool:
        return self.values and len(self.values) == 1


class DtypeDefinition:
    """Represents a class of dtypes, such as uint or float, rather than a concrete dtype such as uint8.
    Not (yet) part of the public interface."""

    def __init__(self, name: str, set_fn, get_fn, return_type: Any = Any, is_signed: bool = False, bitlength2chars_fn=None,
                 variable_length: bool = False, allowed_lengths: Tuple[int, ...] = tuple(), multiplier: int = 1, description: str = ''):

        # Consistency checks
        if int(multiplier) != multiplier or multiplier <= 0:
            raise ValueError("multiplier must be an positive integer")
        if variable_length and allowed_lengths:
            raise ValueError("A variable length dtype can't have allowed lengths.")
        if variable_length and set_fn is not None and 'length' in inspect.signature(set_fn).parameters:
            raise ValueError("A variable length dtype can't have a set_fn which takes a length.")

        self.name = name
        self.description = description
        self.return_type = return_type
        self.is_signed = is_signed
        self.variable_length = variable_length
        self.allowed_lengths = AllowedLengths(allowed_lengths)

        self.multiplier = multiplier

        # Can work out if set_fn needs length based on its signature.
        self.set_fn_needs_length = set_fn is not None and 'length' in inspect.signature(set_fn).parameters
        self.set_fn = set_fn

        if self.allowed_lengths.values:
            def allowed_length_checked_get_fn(bs):
                if len(bs) not in self.allowed_lengths:
                    if self.allowed_lengths.only_one_value():
                        raise bitstring.InterpretError(f"'{self.name}' dtypes must have a length of {self.allowed_lengths.values[0]}, but received a length of {len(bs)}.")
                    else:
                        raise bitstring.InterpretError(f"'{self.name}' dtypes must have a length in {self.allowed_lengths}, but received a length of {len(bs)}.")
                return get_fn(bs)
            self.get_fn = allowed_length_checked_get_fn  # Interpret everything and check the length
        else:
            self.get_fn = get_fn  # Interpret everything

        # Create a reading function from the get_fn.
        if not self.variable_length:
            if self.allowed_lengths.only_one_value():
                def read_fn(bs, start):
                    return self.get_fn(bs[start:start + self.allowed_lengths.values[0]])
            else:
                def read_fn(bs, start, length):
                    if len(bs) < start + length:
                        raise bitstring.ReadError(f"Needed a length of at least {length} bits, but only {len(bs) - start} bits were available.")
                    return self.get_fn(bs[start:start + length])
            self.read_fn = read_fn
        else:
            # We only find out the length when we read/get.
            def length_checked_get_fn(bs):
                x, length = get_fn(bs)
                if length != len(bs):
                    raise ValueError
                return x
            self.get_fn = length_checked_get_fn

            def read_fn(bs, start):
                try:
                    x, length = get_fn(bs[start:])
                except bitstring.InterpretError:
                    raise bitstring.ReadError
                return x, start + length
            self.read_fn = read_fn
        self.bitlength2chars_fn = bitlength2chars_fn

    def get_dtype(self, length: Optional[int] = None, scale: Union[None, float, int] = None) -> Dtype:
        if self.allowed_lengths:
            if length is None:
                if self.allowed_lengths.only_one_value():
                    length = self.allowed_lengths.values[0]
            else:
                if length not in self.allowed_lengths:
                    if self.allowed_lengths.only_one_value():
                        raise ValueError(f"A length of {length} was supplied for the '{self.name}' dtype, but its only allowed length is {self.allowed_lengths.values[0]}.")
                    else:
                        raise ValueError(f"A length of {length} was supplied for the '{self.name}' dtype which is not one of its possible lengths (must be one of {self.allowed_lengths}).")
        if length is None:
            d = Dtype._create(self, None, scale)
            return d
        if self.variable_length:
            raise ValueError(f"A length ({length}) shouldn't be supplied for the variable length dtype '{self.name}'.")
        d = Dtype._create(self, length, scale)
        return d

    def __repr__(self) -> str:
        s = f"{self.__class__.__name__}(name='{self.name}', description='{self.description}', return_type={self.return_type.__name__}, "
        s += f"is_signed={self.is_signed}, set_fn_needs_length={self.set_fn_needs_length}, allowed_lengths={self.allowed_lengths!s}, multiplier={self.multiplier})"
        return s


class Register:
    """A singleton class that holds all the DtypeDefinitions. Not (yet) part of the public interface."""

    _instance: Optional[Register] = None
    names: Dict[str, DtypeDefinition] = {}

    def __new__(cls) -> Register:
        # Singleton. Only one Register instance can ever exist.
        if cls._instance is None:
            cls._instance = super(Register, cls).__new__(cls)
        return cls._instance

    @classmethod
    def add_dtype(cls, definition: DtypeDefinition):
        cls.names[definition.name] = definition
        if definition.get_fn is not None:
            setattr(bitstring.bits.Bits, definition.name, property(fget=definition.get_fn, doc=f"The bitstring as {definition.description}. Read only."))
        if definition.set_fn is not None:
            setattr(bitstring.bitarray_.BitArray, definition.name, property(fget=definition.get_fn, fset=definition.set_fn, doc=f"The bitstring as {definition.description}. Read and write."))

    @classmethod
    def add_dtype_alias(cls, name: str, alias: str):
        cls.names[alias] = cls.names[name]
        definition = cls.names[alias]
        if definition.get_fn is not None:
            setattr(bitstring.bits.Bits, alias, property(fget=definition.get_fn, doc=f"An alias for '{name}'. Read only."))
        if definition.set_fn is not None:
            setattr(bitstring.bitarray_.BitArray, alias, property(fget=definition.get_fn, fset=definition.set_fn, doc=f"An alias for '{name}'. Read and write."))

    @classmethod
    def get_dtype(cls, name: str, length: Optional[int], scale: Union[None, float, int] = None) -> Dtype:
        try:
            definition = cls.names[name]
        except KeyError:
            raise ValueError(f"Unknown Dtype name '{name}'. Names available: {list(cls.names.keys())}.")
        else:
            return definition.get_dtype(length, scale)

    @classmethod
    def __getitem__(cls, name: str) -> DtypeDefinition:
        return cls.names[name]

    @classmethod
    def __delitem__(cls, name: str) -> None:
        del cls.names[name]

    def __repr__(self) -> str:
        s = [f"{'key':<12}:{'name':^12}{'signed':^8}{'set_fn_needs_length':^23}{'allowed_lengths':^16}{'multiplier':^12}{'return_type':<13}"]
        s.append('-' * 85)
        for key in self.names:
            m = self.names[key]
            allowed = '' if not m.allowed_lengths else m.allowed_lengths
            ret = 'None' if m.return_type is None else m.return_type.__name__
            s.append(f"{key:<12}:{m.name:>12}{m.is_signed:^8}{m.set_fn_needs_length:^16}{allowed!s:^16}{m.multiplier:^12}{ret:<13} # {m.description}")
        return '\n'.join(s)


# Create the Register singleton
dtype_register = Register()
