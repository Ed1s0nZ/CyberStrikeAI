"""
"""

# Created on 2013.09.11
#
# Author: Giovanni Cannata
#
# Copyright 2013 - 2020 Giovanni Cannata
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

import stringprep
from unicodedata import ucd_3_2_0 as unicode32
from os import urandom
from binascii import hexlify

from ... import SASL
from ...core.results import RESULT_AUTH_METHOD_NOT_SUPPORTED
from ...core.exceptions import LDAPSASLPrepError, LDAPPasswordIsMandatoryError


def sasl_prep(data):
    """
    implement SASLPrep profile as per RFC4013:
    it defines the "SASLprep" profile of the "stringprep" algorithm [StringPrep].
    The profile is designed for use in Simple Authentication and Security
    Layer ([SASL]) mechanisms, such as [PLAIN], [CRAM-MD5], and
    [DIGEST-MD5].  It may be applicable where simple user names and
    passwords are used.  This profile is not intended for use in
    preparing identity strings that are not simple user names (e.g.,
    email addresses, domain names, distinguished names), or where
    identity or password strings that are not character data, or require
    different handling (e.g., case folding).
    """

    # mapping
    prepared_data = ''
    for c in data:
        if stringprep.in_table_c12(c):
            # non-ASCII space characters [StringPrep, C.1.2] that can be mapped to SPACE (U+0020)
            prepared_data += ' '
        elif stringprep.in_table_b1(c):
            # the "commonly mapped to nothing" characters [StringPrep, B.1] that can be mapped to nothing.
            pass
        else:
            prepared_data += c

    # normalizing
    # This profile specifies using Unicode normalization form KC
    # The repertoire is Unicode 3.2 as per RFC 4013 (2)

    prepared_data = unicode32.normalize('NFKC', prepared_data)

    if not prepared_data:
        raise LDAPSASLPrepError('SASLprep error: unable to normalize string')

    # prohibit
    for c in prepared_data:
        if stringprep.in_table_c12(c):
            # Non-ASCII space characters [StringPrep, C.1.2]
            raise LDAPSASLPrepError('SASLprep error: non-ASCII space character present')
        elif stringprep.in_table_c21(c):
            # ASCII control characters [StringPrep, C.2.1]
            raise LDAPSASLPrepError('SASLprep error: ASCII control character present')
        elif stringprep.in_table_c22(c):
            # Non-ASCII control characters [StringPrep, C.2.2]
            raise LDAPSASLPrepError('SASLprep error: non-ASCII control character present')
        elif stringprep.in_table_c3(c):
            # Private Use characters [StringPrep, C.3]
            raise LDAPSASLPrepError('SASLprep error: private character present')
        elif stringprep.in_table_c4(c):
            # Non-character code points [StringPrep, C.4]
            raise LDAPSASLPrepError('SASLprep error: non-character code point present')
        elif stringprep.in_table_c5(c):
            # Surrogate code points [StringPrep, C.5]
            raise LDAPSASLPrepError('SASLprep error: surrogate code point present')
        elif stringprep.in_table_c6(c):
            # Inappropriate for plain text characters [StringPrep, C.6]
            raise LDAPSASLPrepError('SASLprep error: inappropriate for plain text character present')
        elif stringprep.in_table_c7(c):
            # Inappropriate for canonical representation characters [StringPrep, C.7]
            raise LDAPSASLPrepError('SASLprep error: inappropriate for canonical representation character present')
        elif stringprep.in_table_c8(c):
            # Change display properties or deprecated characters [StringPrep, C.8]
            raise LDAPSASLPrepError('SASLprep error: change display property or deprecated character present')
        elif stringprep.in_table_c9(c):
            # Tagging characters [StringPrep, C.9]
            raise LDAPSASLPrepError('SASLprep error: tagging character present')

    # check bidi
    # if a string contains any r_and_al_cat character, the string MUST NOT contain any l_cat character.
    flag_r_and_al_cat = False
    flag_l_cat = False
    for c in prepared_data:
        if stringprep.in_table_d1(c):
            flag_r_and_al_cat = True
        elif stringprep.in_table_d2(c):
            flag_l_cat = True

        if flag_r_and_al_cat and flag_l_cat:
            raise LDAPSASLPrepError('SASLprep error: string cannot contain (R or AL) and L bidirectional chars')

    # If a string contains any r_and_al_cat character, a r_and_al_cat character MUST be the first character of the string
    # and a r_and_al_cat character MUST be the last character of the string.
    if flag_r_and_al_cat and not stringprep.in_table_d1(prepared_data[0]) and not stringprep.in_table_d2(prepared_data[-1]):
        raise LDAPSASLPrepError('r_and_al_cat character present, must be first and last character of the string')

    return prepared_data


def validate_simple_password(password, accept_empty=False):
    """
    validate simple password as per RFC4013 using sasl_prep:
    """

    if accept_empty and not password:
        return password
    elif not password:
        raise LDAPPasswordIsMandatoryError("simple password can't be empty")

    if not isinstance(password, bytes):  # bytes are returned raw, as per RFC (4.2)
        password = sasl_prep(password)
        if not isinstance(password, bytes):
            password = password.encode('utf-8')

    return password


def abort_sasl_negotiation(connection, controls):
    from ...operation.bind import bind_operation

    request = bind_operation(connection.version, SASL, None, None, '', None)
    response = connection.post_send_single_response(connection.send('bindRequest', request, controls))
    if connection.strategy.sync:
        result = connection.result
    else:
        result = connection.get_response(response)[0][0]

    return True if result['result'] == RESULT_AUTH_METHOD_NOT_SUPPORTED else False


def send_sasl_negotiation(connection, controls, payload):
    from ...operation.bind import bind_operation

    request = bind_operation(connection.version, SASL, None, None, connection.sasl_mechanism, payload)
    response = connection.post_send_single_response(connection.send('bindRequest', request, controls))

    if connection.strategy.sync:
        result = connection.result
    else:
        _, result = connection.get_response(response)

    return result


def random_hex_string(size):
    return str(hexlify(urandom(size)).decode('ascii'))  # str fix for Python 2
