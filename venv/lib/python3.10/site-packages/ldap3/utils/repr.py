"""
"""

# Created on 2015.07.09
#
# Author: Giovanni Cannata
#
# Copyright 2015 - 2020 Giovanni Cannata
#
# This file is part of ldap3.
#
# ldap3 is free software: you can redistribute it and/or modify
# it under the terms of the GNU Lesser General Public License as published
# by the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# ldap3 is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU Lesser General Public License for more details.
#
# You should have received a copy of the GNU Lesser General Public License
# along with ldap3 in the COPYING and COPYING.LESSER files.
# If not, see <http://www.gnu.org/licenses/>.
from binascii import hexlify

from .. import STRING_TYPES

try:
    from sys import stdout
    repr_encoding = stdout.encoding  # get the encoding of the stdout for printing (repr)
    if not repr_encoding:
        repr_encoding = 'ascii'  # default
except Exception:
    repr_encoding = 'ascii'  # default


def to_stdout_encoding(value):
    if not isinstance(value, STRING_TYPES):
        value = str(value)

    if str is bytes:  # Python 2
        try:
            return value.encode(repr_encoding, 'backslashreplace')
        except UnicodeDecodeError:  # Python 2.6
            return hexlify(value)
    else:  # Python 3
        try:
            return value.encode(repr_encoding, errors='backslashreplace').decode(repr_encoding, errors='backslashreplace')
        except UnicodeDecodeError:
            return hexlify(value)
