# -*- coding: utf-8 -*-
from __future__ import print_function

import os
import sys
import struct
from builtins import bytes

nocolor = False

def blue(msg):
    '''Return the input string as console-escaped blue.'''
    if nocolor:
        return msg
    else:
        return "\033[1;36m%s\033[1;m" % msg


def red(msg):
    '''Return the input string as console-escaped red.'''
    if nocolor:
        return msg
    else:
        return "\033[31m%s\033[1;m" % msg


def green(msg):
    '''Return the input string as console-escaped green.'''
    if nocolor:
        return msg
    else:
        return "\033[32m%s\033[1;m" % msg


def purple(msg):
    '''Return the input string as console-escaped purple.'''
    if nocolor:
        return msg
    else:
        return "\033[1;35m%s\033[1;m" % msg


def print_error(msg):
    '''Write the input string to stderr.'''
    print(msg, file=sys.stderr)


def ascii_char(c):
    '''Return the ASCII or (.) representation of the input character.'''
    if ord(c) >= 32 and ord(c) <= 126:
        return c
    return '.'


def hex_dump(data, size=16):
    '''Print a debug view of binary data similar to a hex editor

    Args:
        data (binary): Data to be printed.
        size (Optional[int]): Length of each line.
    '''
    def print_line(line):
        print("%s | %s" % (
            line.encode("hex"),
            "".join([ascii_char(c) for c in line])
        ))
        pass

    for i in range(0, len(data) / size):
        data_line = data[i * size:i * size + size]
        print_line(data_line)

    if not len(data) % size == 0:
        print_line(data[(len(data) % size) * -1:])


def sguid(b, big=False):
    '''RFC4122 binary GUID as string.'''
    if b is None or len(b) != 16:
        return ""
    a, b, c, d = struct.unpack("%sIHH8s" % (">" if big else "<"), b)
    d = ''.join('%02x' % c for c in bytes(d))
    return "%08x-%04x-%04x-%s-%s" % (a, b, c, d[:4], d[4:])


def s2aguid(s):
    '''RFC4122 string GUID as int array.'''
    guid = [s[:8], s[8 + 1:9 + 4], s[13 + 1:14 + 4],
            s[18 + 1:19 + 4] + s[-12:]]
    return aguid(b"".join([bytes.fromhex(part) for part in guid]), True)


def a2sguid(a):
    '''RFC4122 int array GUID as string.'''
    guid = ""
    for value in a:
        value = format(value, 'x')
        guid += value.zfill(len(value) + len(value) % 2).decode("hex")
    guid = guid.zfill(len(guid) + len(guid) % 2)
    return sguid(guid, True)


def aguid(b, big=False):
    '''RFC4122 binary GUID as int array.'''
    a, b, c, d = struct.unpack("%sIHH8s" % (">" if big else "<"), b)
    return [a, b, c] + [_c for _c in d]


def bit_set(field, bit):
    '''Check if bit is set (1) in field.'''
    return (field & bit == bit)


def dump_data(name, data):
    '''Write binary data to name.

    Args:
        name (string): Path to output file, created if it does not exist.
        data (binary): Content to be written.
    '''
    try:
        if os.path.dirname(name) != '':
            if not os.path.exists(os.path.dirname(name)):
                os.makedirs(os.path.dirname(name))
        with open(name, 'wb') as fh:
            fh.write(data)
        print("Wrote: %s" % (red(name)))
    except Exception as e:
        print("Error: could not write (%s), (%s)." % (name, str(e)))


def search_firmware_volumes(data, byte_align=16, limit=None):
    '''"Search a blob for '_FVH' magics, related to firmware volume headers.'''
    potential_volumes = []
    for aligned in range(32, len(data), byte_align):
        if data[aligned:aligned + 4] == b'_FVH':
            potential_volumes.append(aligned)
            if limit and limit == len(potential_volumes):
                return potential_volumes
        magic = data[(aligned + byte_align // 2):(aligned + byte_align // 2 + 4)]
        if magic == b'_FVH':
            potential_volumes.append(aligned + byte_align // 2)
            if limit and limit == len(potential_volumes):
                return potential_volumes
    return potential_volumes


def flatten_firmware_objects(base_objects):
    '''Flatten the parent-child relations between firmware objects.

    It is often nice to remove the structured relationship between firmware
    objects. Each object maintains a pointer to its parent and will
    maintain pointers to children. The output will contain a flattened list
    of all children.

    Args:
        base_objects (list): Nested list of firmware objects.

    Returns:
        list: Non-Nested list of firmware objects.
    '''
    objects = []
    for _object in base_objects:
        objects.append(_object)
        if "objects" in _object and len(_object["objects"]) > 0:
            objects += flatten_firmware_objects(_object["objects"])
    return objects
