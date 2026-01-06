import struct
from collections.abc import Callable
from typing import Any

try:
    import _md5 as md5lib
except ImportError:
    import hashlib as md5lib


md5_unpacker = struct.Struct("4I")


def stable_hash(t: tuple) -> int:
    cnt = _dump_tuple(t)
    hd = md5lib.md5(cnt).digest()
    return md5_unpacker.unpack(hd)[0]  # 32 bits


def _dump_tuple(t: tuple) -> bytes:
    cnt = b""
    for item in t:
        if item is not None:
            type_ = type(item)
            if type_ in _DUMP_BY_TYPE:
                cnt += _DUMP_BY_TYPE[type_](item)
            else:
                cnt += struct.pack("<Q", hash(item) & 0xFFFF_FFFF_FFFF_FFFF)
        cnt += b"\xf0"
    return cnt


def _dump_str(t: str) -> bytes:
    return t.encode("ascii")


def _dump_int(t: int) -> bytes:
    prefix = b"" if t >= 0 else b"-"
    t = abs(t)
    if t <= 0xFFFF:
        return prefix + struct.pack("<H", t)
    elif t <= 0xFFFF_FFFF:
        return prefix + struct.pack("<I", t)
    elif t <= 0xFFFF_FFFF_FFFF_FFFF:
        return prefix + struct.pack("<Q", t)
    else:
        cnt = b""
        while t > 0:
            cnt += _dump_int(t & 0xFFFF_FFFF_FFFF_FFFF)
            t >>= 64
        return prefix + cnt


def _dump_type(t: type) -> bytes:
    return t.__name__.encode("ascii")


_DUMP_BY_TYPE: dict[type, Callable[[Any], bytes]] = {
    tuple: _dump_tuple,
    str: _dump_str,
    int: _dump_int,
    type: _dump_type,
}
