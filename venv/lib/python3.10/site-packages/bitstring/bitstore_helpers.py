from __future__ import annotations

import struct
import math
import functools
from typing import Union, Optional, Dict, Callable
import bitarray
from bitstring.bitstore import BitStore
import bitstring
from bitstring.fp8 import p4binary_fmt, p3binary_fmt
from bitstring.mxfp import (e3m2mxfp_fmt, e2m3mxfp_fmt, e2m1mxfp_fmt, e4m3mxfp_saturate_fmt,
                            e5m2mxfp_saturate_fmt, e4m3mxfp_overflow_fmt, e5m2mxfp_overflow_fmt)

# The size of various caches used to improve performance
CACHE_SIZE = 256


def tidy_input_string(s: str) -> str:
    """Return string made lowercase and with all whitespace and underscores removed."""
    try:
        t = s.split()
    except (AttributeError, TypeError):
        raise ValueError(f"Expected str object but received a {type(s)} with value {s}.")
    return ''.join(t).lower().replace('_', '')


@functools.lru_cache(CACHE_SIZE)
def str_to_bitstore(s: str) -> BitStore:
    _, tokens = bitstring.utils.tokenparser(s)
    bs = BitStore()
    for token in tokens:
        bs += bitstore_from_token(*token)
    bs.immutable = True
    return bs


def bin2bitstore(binstring: str) -> BitStore:
    binstring = tidy_input_string(binstring)
    binstring = binstring.replace('0b', '')
    try:
        return BitStore(binstring)
    except ValueError:
        raise bitstring.CreationError(f"Invalid character in bin initialiser {binstring}.")


def bin2bitstore_unsafe(binstring: str) -> BitStore:
    return BitStore(binstring)


def hex2bitstore(hexstring: str) -> BitStore:
    hexstring = tidy_input_string(hexstring)
    hexstring = hexstring.replace('0x', '')
    try:
        ba = bitarray.util.hex2ba(hexstring)
    except ValueError:
        raise bitstring.CreationError("Invalid symbol in hex initialiser.")
    return BitStore(ba)


def oct2bitstore(octstring: str) -> BitStore:
    octstring = tidy_input_string(octstring)
    octstring = octstring.replace('0o', '')
    try:
        ba = bitarray.util.base2ba(8, octstring)
    except ValueError:
        raise bitstring.CreationError("Invalid symbol in oct initialiser.")
    return BitStore(ba)


def ue2bitstore(i: Union[str, int]) -> BitStore:
    i = int(i)
    if i < 0:
        raise bitstring.CreationError("Cannot use negative initialiser for unsigned exponential-Golomb.")
    if i == 0:
        return BitStore('1')
    tmp = i + 1
    leadingzeros = -1
    while tmp > 0:
        tmp >>= 1
        leadingzeros += 1
    remainingpart = i + 1 - (1 << leadingzeros)
    return BitStore('0' * leadingzeros + '1') + int2bitstore(remainingpart, leadingzeros, False)


def se2bitstore(i: Union[str, int]) -> BitStore:
    i = int(i)
    if i > 0:
        u = (i * 2) - 1
    else:
        u = -2 * i
    return ue2bitstore(u)


def uie2bitstore(i: Union[str, int]) -> BitStore:
    i = int(i)
    if i < 0:
        raise bitstring.CreationError("Cannot use negative initialiser for unsigned interleaved exponential-Golomb.")
    return BitStore('1' if i == 0 else '0' + '0'.join(bin(i + 1)[3:]) + '1')


def sie2bitstore(i: Union[str, int]) -> BitStore:
    i = int(i)
    if i == 0:
        return BitStore('1')
    else:
        return uie2bitstore(abs(i)) + (BitStore('1') if i < 0 else BitStore('0'))


def bfloat2bitstore(f: Union[str, float], big_endian: bool) -> BitStore:
    f = float(f)
    fmt = '>f' if big_endian else '<f'
    try:
        b = struct.pack(fmt, f)
    except OverflowError:
        # For consistency we overflow to 'inf'.
        b = struct.pack(fmt, float('inf') if f > 0 else float('-inf'))
    return BitStore.frombytes(b[0:2]) if big_endian else BitStore.frombytes(b[2:4])


def p4binary2bitstore(f: Union[str, float]) -> BitStore:
    f = float(f)
    u = p4binary_fmt.float_to_int8(f)
    return int2bitstore(u, 8, False)


def p3binary2bitstore(f: Union[str, float]) -> BitStore:
    f = float(f)
    u = p3binary_fmt.float_to_int8(f)
    return int2bitstore(u, 8, False)


def e4m3mxfp2bitstore(f: Union[str, float]) -> BitStore:
    f = float(f)
    if bitstring.options.mxfp_overflow == 'saturate':
        u = e4m3mxfp_saturate_fmt.float_to_int(f)
    else:
        u = e4m3mxfp_overflow_fmt.float_to_int(f)
    return int2bitstore(u, 8, False)


def e5m2mxfp2bitstore(f: Union[str, float]) -> BitStore:
    f = float(f)
    if bitstring.options.mxfp_overflow == 'saturate':
        u = e5m2mxfp_saturate_fmt.float_to_int(f)
    else:
        u = e5m2mxfp_overflow_fmt.float_to_int(f)
    return int2bitstore(u, 8, False)


def e3m2mxfp2bitstore(f: Union[str, float]) -> BitStore:
    f = float(f)
    if math.isnan(f):
        raise ValueError("Cannot convert float('nan') to e3m2mxfp format as it has no representation for it.")
    u = e3m2mxfp_fmt.float_to_int(f)
    return int2bitstore(u, 6, False)


def e2m3mxfp2bitstore(f: Union[str, float]) -> BitStore:
    f = float(f)
    if math.isnan(f):
        raise ValueError("Cannot convert float('nan') to e2m3mxfp format as it has no representation for it.")
    u = e2m3mxfp_fmt.float_to_int(f)
    return int2bitstore(u, 6, False)


def e2m1mxfp2bitstore(f: Union[str, float]) -> BitStore:
    f = float(f)
    if math.isnan(f):
        raise ValueError("Cannot convert float('nan') to e2m1mxfp format as it has no representation for it.")
    u = e2m1mxfp_fmt.float_to_int(f)
    return int2bitstore(u, 4, False)


e8m0mxfp_allowed_values = [float(2 ** x) for x in range(-127, 128)]


def e8m0mxfp2bitstore(f: Union[str, float]) -> BitStore:
    f = float(f)
    if math.isnan(f):
        return BitStore('11111111')
    try:
        i = e8m0mxfp_allowed_values.index(f)
    except ValueError:
        raise ValueError(f"{f} is not a valid e8m0mxfp value. It must be exactly 2 ** i, for -127 <= i <= 127 or float('nan') as no rounding will be done.")
    return int2bitstore(i, 8, False)


def mxint2bitstore(f: Union[str, float]) -> BitStore:
    f = float(f)
    if math.isnan(f):
        raise ValueError("Cannot convert float('nan') to mxint format as it has no representation for it.")
    f *= 2 ** 6  # Remove the implicit scaling factor
    if f > 127:  # 1 + 63/64
        return BitStore('01111111')
    if f <= -128:  # -2
        return BitStore('10000000')
    # Want to round to nearest, so move by 0.5 away from zero and round down by converting to int
    if f >= 0.0:
        f += 0.5
        i = int(f)
        # For ties-round-to-even
        if f - i == 0.0 and i % 2:
            i -= 1
    else:
        f -= 0.5
        i = int(f)
        if f - i == 0.0 and i % 2:
            i += 1
    return int2bitstore(i, 8, True)


def int2bitstore(i: int, length: int, signed: bool) -> BitStore:
    i = int(i)
    try:
        x = BitStore(bitarray.util.int2ba(i, length=length, endian='big', signed=signed))
    except OverflowError as e:
        if signed:
            if i >= (1 << (length - 1)) or i < -(1 << (length - 1)):
                raise bitstring.CreationError(f"{i} is too large a signed integer for a bitstring of length {length}. "
                                    f"The allowed range is [{-(1 << (length - 1))}, {(1 << (length - 1)) - 1}].")
        else:
            if i >= (1 << length):
                raise bitstring.CreationError(f"{i} is too large an unsigned integer for a bitstring of length {length}. "
                                    f"The allowed range is [0, {(1 << length) - 1}].")
            if i < 0:
                raise bitstring.CreationError("uint cannot be initialised with a negative number.")
        raise e
    return x


def intle2bitstore(i: int, length: int, signed: bool) -> BitStore:
    x = int2bitstore(i, length, signed).tobytes()
    return BitStore.frombytes(x[::-1])


def float2bitstore(f: Union[str, float], length: int, big_endian: bool) -> BitStore:
    f = float(f)
    fmt = {16: '>e', 32: '>f', 64: '>d'}[length] if big_endian else {16: '<e', 32: '<f', 64: '<d'}[length]
    try:
        b = struct.pack(fmt, f)
    except OverflowError:
        # If float64 doesn't fit it automatically goes to 'inf'. This reproduces that behaviour for other types.
        b = struct.pack(fmt, float('inf') if f > 0 else float('-inf'))
    return BitStore.frombytes(b)


literal_bit_funcs: Dict[str, Callable[..., BitStore]] = {
    '0x': hex2bitstore,
    '0X': hex2bitstore,
    '0b': bin2bitstore,
    '0B': bin2bitstore,
    '0o': oct2bitstore,
    '0O': oct2bitstore,
}


def bitstore_from_token(name: str, token_length: Optional[int], value: Optional[str]) -> BitStore:
    if name in literal_bit_funcs:
        return literal_bit_funcs[name](value)
    try:
        d = bitstring.dtypes.Dtype(name, token_length)
    except ValueError as e:
        raise bitstring.CreationError(f"Can't parse token: {e}")
    if value is None and name != 'pad':
        raise ValueError(f"Token {name} requires a value.")
    bs = d.build(value)._bitstore
    if token_length is not None and len(bs) != d.bitlength:
        raise bitstring.CreationError(f"Token with length {token_length} packed with value of length {len(bs)} "
                                      f"({name}:{token_length}={value}).")
    return bs
