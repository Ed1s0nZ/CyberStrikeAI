from __future__ import annotations

import bitstring
from bitstring.bitstream import BitStream
from bitstring.utils import tokenparser
from bitstring.exceptions import CreationError
from typing import Union, List
from bitstring.bitstore import BitStore
from bitstring.bitstore_helpers import bitstore_from_token


def pack(fmt: Union[str, List[str]], *values, **kwargs) -> BitStream:
    """Pack the values according to the format string and return a new BitStream.

    fmt -- A single string or a list of strings with comma separated tokens
           describing how to create the BitStream.
    values -- Zero or more values to pack according to the format.
    kwargs -- A dictionary or keyword-value pairs - the keywords used in the
              format string will be replaced with their given value.

    Token examples: 'int:12'    : 12 bits as a signed integer
                    'uint:8'    : 8 bits as an unsigned integer
                    'float:64'  : 8 bytes as a big-endian float
                    'intbe:16'  : 2 bytes as a big-endian signed integer
                    'uintbe:16' : 2 bytes as a big-endian unsigned integer
                    'intle:32'  : 4 bytes as a little-endian signed integer
                    'uintle:32' : 4 bytes as a little-endian unsigned integer
                    'floatle:64': 8 bytes as a little-endian float
                    'intne:24'  : 3 bytes as a native-endian signed integer
                    'uintne:24' : 3 bytes as a native-endian unsigned integer
                    'floatne:32': 4 bytes as a native-endian float
                    'hex:80'    : 80 bits as a hex string
                    'oct:9'     : 9 bits as an octal string
                    'bin:1'     : single bit binary string
                    'ue' / 'uie': next bits as unsigned exp-Golomb code
                    'se' / 'sie': next bits as signed exp-Golomb code
                    'bits:5'    : 5 bits as a bitstring object
                    'bytes:10'  : 10 bytes as a bytes object
                    'bool'      : 1 bit as a bool
                    'pad:3'     : 3 zero bits as padding

    >>> s = pack('uint:12, bits', 100, '0xffe')
    >>> t = pack(['bits', 'bin:3'], s, '111')
    >>> u = pack('uint:8=a, uint:8=b, uint:55=a', a=6, b=44)

    """
    tokens = []
    if isinstance(fmt, str):
        fmt = [fmt]
    try:
        for f_item in fmt:
            _, tkns = tokenparser(f_item, tuple(sorted(kwargs.keys())))
            tokens.extend(tkns)
    except ValueError as e:
        raise CreationError(*e.args)
    value_iter = iter(values)
    bsl: List[BitStore] = []
    try:
        for name, length, value in tokens:
            # If the value is in the kwd dictionary then it takes precedence.
            value = kwargs.get(value, value)
            # If the length is in the kwd dictionary then use that too.
            length = kwargs.get(length, length)
            # Also if we just have a dictionary name then we want to use it
            if name in kwargs and length is None and value is None:
                bsl.append(BitStream(kwargs[name])._bitstore)
                continue
            if length is not None:
                length = int(length)
            if value is None and name != 'pad':
                # Take the next value from the ones provided
                value = next(value_iter)
            if name == 'bits':
                value = bitstring.bits.Bits(value)
                if length is not None and length != len(value):
                    raise CreationError(f"Token with length {length} packed with value of length {len(value)}.")
                bsl.append(value._bitstore)
                continue
            bsl.append(bitstore_from_token(name, length, value))
    except StopIteration:
        raise CreationError(f"Not enough parameters present to pack according to the "
                            f"format. {len(tokens)} values are needed.")

    try:
        next(value_iter)
    except StopIteration:
        # Good, we've used up all the *values.
        s = BitStream()
        if bitstring.options.lsb0:
            bsl.reverse()
        for b in bsl:
            s._bitstore += b
        return s

    raise CreationError(f"Too many parameters present to pack according to the format. Only {len(tokens)} values were expected.")
