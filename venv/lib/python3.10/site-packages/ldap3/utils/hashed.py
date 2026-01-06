"""
"""

# Created on 2015.07.16
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

from .. import HASHED_NONE, HASHED_MD5, HASHED_SALTED_MD5, HASHED_SALTED_SHA, HASHED_SALTED_SHA256, \
    HASHED_SALTED_SHA384, HASHED_SALTED_SHA512, HASHED_SHA, HASHED_SHA256, HASHED_SHA384, HASHED_SHA512

import hashlib
from os import urandom
from base64 import b64encode

from ..core.exceptions import LDAPInvalidHashAlgorithmError

# each tuple: (the string to include between braces in the digest, the name of the algorithm to invoke with the new() function)

algorithms_table = {
    HASHED_MD5: ('md5', 'MD5'),
    HASHED_SHA: ('sha', 'SHA1'),
    HASHED_SHA256: ('sha256', 'SHA256'),
    HASHED_SHA384: ('sha384', 'SHA384'),
    HASHED_SHA512: ('sha512', 'SHA512')
}


salted_table = {
    HASHED_SALTED_MD5: ('smd5', HASHED_MD5),
    HASHED_SALTED_SHA: ('ssha', HASHED_SHA),
    HASHED_SALTED_SHA256: ('ssha256', HASHED_SHA256),
    HASHED_SALTED_SHA384: ('ssha384', HASHED_SHA384),
    HASHED_SALTED_SHA512: ('ssha512', HASHED_SHA512)
}


def hashed(algorithm, value, salt=None, raw=False, encoding='utf-8'):
    if str is not bytes and not isinstance(value, bytes):  # Python 3
        value = value.encode(encoding)

    if algorithm is None or algorithm == HASHED_NONE:
        return value

    # algorithm name can be already coded in the ldap3 constants or can be any value passed in the 'algorithm' parameter

    if algorithm in algorithms_table:
        try:
            digest = hashlib.new(algorithms_table[algorithm][1], value).digest()
        except ValueError:
            raise LDAPInvalidHashAlgorithmError('Hash algorithm ' + str(algorithm) + ' not available')

        if raw:
            return digest
        return ('{%s}' % algorithms_table[algorithm][0]) + b64encode(digest).decode('ascii')
    elif algorithm in salted_table:
        if not salt:
            salt = urandom(8)
        digest = hashed(salted_table[algorithm][1], value + salt, raw=True) + salt
        if raw:
            return digest
        return ('{%s}' % salted_table[algorithm][0]) + b64encode(digest).decode('ascii')
    else:
        # if an unknown (to the library) algorithm is requested passes the name as the string in braces and as the algorithm name
        # if salt is present uses it to salt the digest
        try:
            if not salt:
                digest = hashlib.new(algorithm, value).digest()
            else:
                digest = hashlib.new(algorithm, value + salt).digest() + salt
        except ValueError:
            raise LDAPInvalidHashAlgorithmError('Hash algorithm ' + str(algorithm) + ' not available')

        if raw:
            return digest
        return ('{%s}' % algorithm) + b64encode(digest).decode('ascii')

