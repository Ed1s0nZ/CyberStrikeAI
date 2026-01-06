"""
"""

# Created on 2016.08.09
#
# Author: Giovanni Cannata
#
# Copyright 2016 - 2020 Giovanni Cannata
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
from binascii import a2b_hex, hexlify
from datetime import datetime
from calendar import timegm
from uuid import UUID
from struct import pack


from ... import SEQUENCE_TYPES, STRING_TYPES, NUMERIC_TYPES, INTEGER_TYPES
from .formatters import format_time, format_ad_timestamp
from ...utils.conv import to_raw, to_unicode, ldap_escape_to_bytes, escape_bytes

# Validators return True if value is valid, False if value is not valid,
# or a value different from True and False that is a valid value to substitute to the input value


def check_backslash(value):
    if isinstance(value, (bytearray, bytes)):
        if b'\\' in value:
            value = value.replace(b'\\', b'\\5C')
    elif isinstance(value, STRING_TYPES):
        if '\\' in value:
            value = value.replace('\\', '\\5C')
    return value


def check_type(input_value, value_type):
    if isinstance(input_value, value_type):
        return True

    if isinstance(input_value, SEQUENCE_TYPES):
        for value in input_value:
            if not isinstance(value, value_type):
                return False
        return True

    return False


# noinspection PyUnusedLocal
def always_valid(input_value):
    return True


def validate_generic_single_value(input_value):
    if not isinstance(input_value, SEQUENCE_TYPES):
        return True

    try:  # object couldn't have a __len__ method
        if len(input_value) == 1:
            return True
    except Exception:
        pass

    return False


def validate_zero_and_minus_one_and_positive_int(input_value):
    """Accept -1 and 0 only (used by pwdLastSet in AD)
    """
    if not isinstance(input_value, SEQUENCE_TYPES):
        if isinstance(input_value, NUMERIC_TYPES) or isinstance(input_value, STRING_TYPES):
            return True if int(input_value) >= -1 else False
        return False
    else:
        if len(input_value) == 1 and (isinstance(input_value[0], NUMERIC_TYPES) or isinstance(input_value[0], STRING_TYPES)):
            return True if int(input_value[0]) >= -1 else False

    return False


def validate_integer(input_value):
    if check_type(input_value, (float, bool)):
        return False
    if check_type(input_value, INTEGER_TYPES):
        return True

    if not isinstance(input_value, SEQUENCE_TYPES):
        sequence = False
        input_value = [input_value]
    else:
        sequence = True  # indicates if a sequence must be returned

    valid_values = []  # builds a list of valid int values
    from decimal import Decimal, InvalidOperation
    for element in input_value:
        try:  #try to convert any type to int, an invalid conversion raise TypeError or ValueError, doublecheck with Decimal type, if both are valid and equal then then int() value is used
            value = to_unicode(element) if isinstance(element, bytes) else element
            decimal_value = Decimal(value)
            int_value = int(value)
            if decimal_value == int_value:
                valid_values.append(int_value)
            else:
                return False
        except (ValueError, TypeError, InvalidOperation):
            return False

    if sequence:
        return valid_values
    else:
        return valid_values[0]


def validate_bytes(input_value):
    return check_type(input_value, bytes)


def validate_boolean(input_value):
    # it could be a real bool or the string TRUE or FALSE, # only a single valued is allowed
    if validate_generic_single_value(input_value):  # valid only if a single value or a sequence with a single element
        if isinstance(input_value, SEQUENCE_TYPES):
            input_value = input_value[0]
        if isinstance(input_value, bool):
            if input_value:
                return 'TRUE'
            else:
                return 'FALSE'
        if str is not bytes and isinstance(input_value, bytes):  # python3 try to converts bytes to string
            input_value = to_unicode(input_value)
        if isinstance(input_value, STRING_TYPES):
            if input_value.lower() == 'true':
                return 'TRUE'
            elif input_value.lower() == 'false':
                return 'FALSE'
    return False


def validate_time_with_0_year(input_value):
    # validates generalized time but accept a 0000 year too
    # if datetime object doesn't have a timezone it's considered local time and is adjusted to UTC
    if not isinstance(input_value, SEQUENCE_TYPES):
        sequence = False
        input_value = [input_value]
    else:
        sequence = True  # indicates if a sequence must be returned

    valid_values = []
    changed = False
    for element in input_value:
        if str is not bytes and isinstance(element, bytes):  # python3 try to converts bytes to string
            element = to_unicode(element)
        if isinstance(element, STRING_TYPES):  # tries to check if it is already be a Generalized Time
            if element.startswith('0000') or isinstance(format_time(to_raw(element)), datetime):  # valid Generalized Time string
                valid_values.append(element)
            else:
                return False
        elif isinstance(element, datetime):
            changed = True
            if element.tzinfo:  # a datetime with a timezone
                valid_values.append(element.strftime('%Y%m%d%H%M%S%z'))
            else:  # datetime without timezone, assumed local and adjusted to UTC
                offset = datetime.now() - datetime.utcnow()
                valid_values.append((element - offset).strftime('%Y%m%d%H%M%SZ'))
        else:
            return False

    if changed:
        if sequence:
            return valid_values
        else:
            return valid_values[0]
    else:
        return True


def validate_time(input_value):
    # if datetime object doesn't have a timezone it's considered local time and is adjusted to UTC
    if not isinstance(input_value, SEQUENCE_TYPES):
        sequence = False
        input_value = [input_value]
    else:
        sequence = True  # indicates if a sequence must be returned

    valid_values = []
    changed = False
    for element in input_value:
        if str is not bytes and isinstance(element, bytes):  # python3 try to converts bytes to string
            element = to_unicode(element)
        if isinstance(element, STRING_TYPES):  # tries to check if it is already be a Generalized Time
            if isinstance(format_time(to_raw(element)), datetime):  # valid Generalized Time string
                valid_values.append(element)
            else:
                return False
        elif isinstance(element, datetime):
            changed = True
            if element.tzinfo:  # a datetime with a timezone
                valid_values.append(element.strftime('%Y%m%d%H%M%S%z'))
            else:  # datetime without timezone, assumed local and adjusted to UTC
                offset = datetime.now() - datetime.utcnow()
                valid_values.append((element - offset).strftime('%Y%m%d%H%M%SZ'))
        else:
            return False

    if changed:
        if sequence:
            return valid_values
        else:
            return valid_values[0]
    else:
        return True


def validate_ad_timestamp(input_value):
    """
    Active Directory stores date/time values as the number of 100-nanosecond intervals
    that have elapsed since the 0 hour on January 1, 1601 till the date/time that is being stored.
    The time is always stored in Greenwich Mean Time (GMT) in the Active Directory.
    """
    if not isinstance(input_value, SEQUENCE_TYPES):
        sequence = False
        input_value = [input_value]
    else:
        sequence = True  # indicates if a sequence must be returned

    valid_values = []
    changed = False
    for element in input_value:
        if str is not bytes and isinstance(element, bytes):  # python3 try to converts bytes to string
            element = to_unicode(element)
        if isinstance(element, NUMERIC_TYPES):
            if 0 <= element <= 9223372036854775807:  # min and max for the AD timestamp starting from 12:00 AM January 1, 1601
                valid_values.append(element)
            else:
                return False
        elif isinstance(element, STRING_TYPES):  # tries to check if it is already be a AD timestamp
            if isinstance(format_ad_timestamp(to_raw(element)), datetime):  # valid Generalized Time string
                valid_values.append(element)
            else:
                return False
        elif isinstance(element, datetime):
            changed = True
            if element.tzinfo:  # a datetime with a timezone
                valid_values.append(to_raw((timegm(element.utctimetuple()) + 11644473600) * 10000000, encoding='ascii'))
            else:  # datetime without timezone, assumed local and adjusted to UTC
                offset = datetime.now() - datetime.utcnow()
                valid_values.append(to_raw((timegm((element - offset).timetuple()) + 11644473600) * 10000000, encoding='ascii'))
        else:
            return False

    if changed:
        if sequence:
            return valid_values
        else:
            return valid_values[0]
    else:
        return True


def validate_ad_timedelta(input_value):
    """
    Should be validated like an AD timestamp except that since it is a time
    delta, it is stored as a negative number.
    """
    if not isinstance(input_value, INTEGER_TYPES) or input_value > 0:
        return False
    return validate_ad_timestamp(input_value * -1)


def validate_guid(input_value):
    """
    object guid in uuid format (Novell eDirectory)
    """
    if not isinstance(input_value, SEQUENCE_TYPES):
        sequence = False
        input_value = [input_value]
    else:
        sequence = True  # indicates if a sequence must be returned

    valid_values = []
    changed = False
    for element in input_value:
        if isinstance(element,  STRING_TYPES):
            try:
                valid_values.append(UUID(element).bytes)
                changed = True
            except ValueError: # try if the value is an escaped ldap byte sequence
                try:
                    x = ldap_escape_to_bytes(element)
                    valid_values.append(UUID(bytes=x).bytes)
                    changed = True
                    continue
                except ValueError:
                    if str is not bytes:  # python 3
                        pass
                    else:
                        valid_values.append(element)
                        continue
                return False
        elif isinstance(element, (bytes, bytearray)):  # assumes bytes are valid
            valid_values.append(element)
        else:
            return False

    if changed:
        # valid_values = [check_backslash(value) for value in valid_values]
        if sequence:
            return valid_values
        else:
            return valid_values[0]
    else:
        return True


def validate_uuid(input_value):
    """
    object entryUUID in uuid format
    """
    if not isinstance(input_value, SEQUENCE_TYPES):
        sequence = False
        input_value = [input_value]
    else:
        sequence = True  # indicates if a sequence must be returned

    valid_values = []
    changed = False
    for element in input_value:
        if isinstance(element,  STRING_TYPES):
            try:
                valid_values.append(str(UUID(element)))
                changed = True
            except ValueError: # try if the value is an escaped byte sequence
                try:
                    valid_values.append(str(UUID(element.replace('\\', ''))))
                    changed = True
                    continue
                except ValueError:
                    if str is not bytes:  # python 3
                        pass
                    else:
                        valid_values.append(element)
                        continue
                return False
        elif isinstance(element, (bytes, bytearray)):  # assumes bytes are valid
            valid_values.append(element)
        else:
            return False

    if changed:
        # valid_values = [check_backslash(value) for value in valid_values]
        if sequence:
            return valid_values
        else:
            return valid_values[0]
    else:
        return True


def validate_uuid_le(input_value):
    r"""
    Active Directory stores objectGUID in uuid_le format, follows RFC4122 and MS-DTYP:
    "{07039e68-4373-264d-a0a7-07039e684373}": string representation big endian, converted to little endian (with or without brace curles)
    "689e030773434d26a7a007039e684373": packet representation, already in little endian
    "\68\9e\03\07\73\43\4d\26\a7\a0\07\03\9e\68\43\73": bytes representation, already in little endian
    byte sequence: already in little endian

    """
    if not isinstance(input_value, SEQUENCE_TYPES):
        sequence = False
        input_value = [input_value]
    else:
        sequence = True  # indicates if a sequence must be returned

    valid_values = []
    changed = False
    for element in input_value:
        error = False
        if isinstance(element, STRING_TYPES):
            if element[0] == '{' and element[-1] == '}':
                try:
                    valid_values.append(UUID(hex=element).bytes_le)  # string representation, value in big endian, converts to little endian
                    changed = True
                except ValueError:
                    error = True
            elif '-' in element:
                try:
                    valid_values.append(UUID(hex=element).bytes_le)  # string representation, value in big endian, converts to little endian
                    changed = True
                except ValueError:
                    error = True
            elif '\\' in element:
                try:
                    valid_values.append(UUID(bytes_le=ldap_escape_to_bytes(element)).bytes_le)  # byte representation, value in little endian
                    changed = True
                except ValueError:
                    error = True
            elif '-' not in element:  # value in little endian
                try:
                    valid_values.append(UUID(bytes_le=a2b_hex(element)).bytes_le)  # packet representation, value in little endian, converts to little endian
                    changed = True
                except ValueError:
                    error = True
            if error and (str is bytes):  # python2 only assume value is bytes and valid
                valid_values.append(element)  # value is untouched, must be in little endian
        elif isinstance(element, (bytes, bytearray)):  # assumes bytes are valid uuid
            valid_values.append(element)  # value is untouched, must be in little endian
        else:
            return False

    if changed:
        # valid_values = [check_backslash(value) for value in valid_values]
        if sequence:
            return valid_values
        else:
            return valid_values[0]
    else:
        return True


def validate_sid(input_value):
    """
        SID= "S-1-" IdentifierAuthority 1*SubAuthority
               IdentifierAuthority= IdentifierAuthorityDec / IdentifierAuthorityHex
                  ; If the identifier authority is < 2^32, the
                  ; identifier authority is represented as a decimal
                  ; number
                  ; If the identifier authority is >= 2^32,
                  ; the identifier authority is represented in
                  ; hexadecimal
                IdentifierAuthorityDec =  1*10DIGIT
                  ; IdentifierAuthorityDec, top level authority of a
                  ; security identifier is represented as a decimal number
                IdentifierAuthorityHex = "0x" 12HEXDIG
                  ; IdentifierAuthorityHex, the top-level authority of a
                  ; security identifier is represented as a hexadecimal number
                SubAuthority= "-" 1*10DIGIT
                  ; Sub-Authority is always represented as a decimal number
                  ; No leading "0" characters are allowed when IdentifierAuthority
                  ; or SubAuthority is represented as a decimal number
                  ; All hexadecimal digits must be output in string format,
                  ; pre-pended by "0x"

        Revision (1 byte): An 8-bit unsigned integer that specifies the revision level of the SID. This value MUST be set to 0x01.
        SubAuthorityCount (1 byte): An 8-bit unsigned integer that specifies the number of elements in the SubAuthority array. The maximum number of elements allowed is 15.
        IdentifierAuthority (6 bytes): A SID_IDENTIFIER_AUTHORITY structure that indicates the authority under which the SID was created. It describes the entity that created the SID. The Identifier Authority value {0,0,0,0,0,5} denotes SIDs created by the NT SID authority.
        SubAuthority (variable): A variable length array of unsigned 32-bit integers that uniquely identifies a principal relative to the IdentifierAuthority. Its length is determined by SubAuthorityCount.

        If you have a SID like S-a-b-c-d-e-f-g-...

        Then the bytes are
        a 	(revision)
        N 	(number of dashes minus two)
        bbbbbb 	(six bytes of "b" treated as a 48-bit number in big-endian format)
        cccc 	(four bytes of "c" treated as a 32-bit number in little-endian format)
        dddd 	(four bytes of "d" treated as a 32-bit number in little-endian format)
        eeee 	(four bytes of "e" treated as a 32-bit number in little-endian format)
        ffff 	(four bytes of "f" treated as a 32-bit number in little-endian format)

    """
    if not isinstance(input_value, SEQUENCE_TYPES):
        sequence = False
        input_value = [input_value]
    else:
        sequence = True  # indicates if a sequence must be returned

    valid_values = []
    changed = False
    for element in input_value:
        if isinstance(element, STRING_TYPES):
            if element.startswith('S-'):
                parts = element.split('-')
                sid_bytes = pack('<q', int(parts[1]))[0:1]  # revision number
                sid_bytes += pack('<q', len(parts[3:]))[0:1]  # number of sub authorities
                if len(parts[2]) <= 10:
                    sid_bytes += pack('>q', int(parts[2]))[2:]  # authority (in dec)
                else:
                    sid_bytes += pack('>q', int(parts[2], 16))[2:]  # authority (in hex)
                for sub_auth in parts[3:]:
                    sid_bytes += pack('<q', int(sub_auth))[0:4]  # sub-authorities
                valid_values.append(sid_bytes)
                changed = True

    if changed:
        # valid_values = [check_backslash(value) for value in valid_values]
        if sequence:
            return valid_values
        else:
            return valid_values[0]
    else:
        return True
