"""
"""

# Created on 2015.08.19
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

from pyasn1 import __version__ as pyasn1_version
from pyasn1.codec.ber import decoder  # for usage in other modules
from pyasn1.codec.ber.encoder import Encoder # for monkeypatching of boolean value
from ..core.results import RESULT_CODES
from ..utils.conv import to_unicode
from ..protocol.convert import referrals_to_list

CLASSES = {(False, False): 0,  # Universal
           (False, True): 1,  # Application
           (True, False): 2,  # Context
           (True, True): 3}  # Private


# Monkeypatching of pyasn1 for encoding Boolean with the value 0xFF for TRUE
# THIS IS NOT PART OF THE FAST BER DECODER
if pyasn1_version == 'xxx0.2.3':
    from pyasn1.codec.ber.encoder import tagMap, BooleanEncoder, encode
    from pyasn1.type.univ import Boolean
    from pyasn1.compat.octets import ints2octs
    class BooleanCEREncoder(BooleanEncoder):
        _true = ints2octs((255,))

    tagMap[Boolean.tagSet] = BooleanCEREncoder()
else:
    from pyasn1.codec.ber.encoder import tagMap, typeMap, AbstractItemEncoder
    from pyasn1.type.univ import Boolean
    from copy import deepcopy

    class LDAPBooleanEncoder(AbstractItemEncoder):
        supportIndefLenMode = False
        if pyasn1_version <= '0.2.3':
            from pyasn1.compat.octets import ints2octs
            _true = ints2octs((255,))
            _false = ints2octs((0,))
            def encodeValue(self, encodeFun, value, defMode, maxChunkSize):
                return value and self._true or self._false, 0
        elif pyasn1_version <= '0.3.1':
            def encodeValue(self, encodeFun, value, defMode, maxChunkSize):
                return value and (255,) or (0,), False, False
        elif pyasn1_version <= '0.3.4':
            def encodeValue(self, encodeFun, value, defMode, maxChunkSize, ifNotEmpty=False):
                return value and (255,) or (0,), False, False
        elif pyasn1_version <= '0.3.7':
            def encodeValue(self, value, encodeFun, **options):
                return value and (255,) or (0,), False, False
        else:
            def encodeValue(self, value, asn1Spec, encodeFun, **options):
                return value and (255,) or (0,), False, False

    customTagMap = deepcopy(tagMap)
    customTypeMap = deepcopy(typeMap)
    customTagMap[Boolean.tagSet] = LDAPBooleanEncoder()
    customTypeMap[Boolean.typeId] = LDAPBooleanEncoder()

    encode = Encoder(customTagMap, customTypeMap)
# end of monkey patching

# a fast BER decoder for LDAP responses only
def compute_ber_size(data):
    """
    Compute size according to BER definite length rules
    Returns size of value and value offset
    """

    if data[1] <= 127:  # BER definite length - short form. Highest bit of byte 1 is 0, message length is in the last 7 bits - Value can be up to 127 bytes long
        return data[1], 2
    else:  # BER definite length - long form. Highest bit of byte 1 is 1, last 7 bits counts the number of following octets containing the value length
        bytes_length = data[1] - 128
        value_length = 0
        cont = bytes_length
        for byte in data[2: 2 + bytes_length]:
            cont -= 1
            value_length += byte * (256 ** cont)
        return value_length, bytes_length + 2


def decode_message_fast(message):
    ber_len, ber_value_offset = compute_ber_size(get_bytes(message[:10]))  # get start of sequence, at maximum 3 bytes for length
    decoded = decode_sequence(message, ber_value_offset, ber_len + ber_value_offset, LDAP_MESSAGE_CONTEXT)
    return {
        'messageID': decoded[0][3],
        'protocolOp': decoded[1][2],
        'payload': decoded[1][3],
        'controls': decoded[2][3] if len(decoded) == 3 else None
    }


def decode_sequence(message, start, stop, context_decoders=None):
    decoded = []
    while start < stop:
        octet = get_byte(message[start])
        ber_class = CLASSES[(bool(octet & 0b10000000), bool(octet & 0b01000000))]
        ber_constructed = bool(octet & 0b00100000)
        ber_type = octet & 0b00011111
        ber_decoder = DECODERS[(ber_class, octet & 0b00011111)] if ber_class < 2 else None
        ber_len, ber_value_offset = compute_ber_size(get_bytes(message[start: start + 10]))
        start += ber_value_offset
        if ber_decoder:
            value = ber_decoder(message, start, start + ber_len, context_decoders)  # call value decode function
        else:
            # try:
            value = context_decoders[ber_type](message, start, start + ber_len)  # call value decode function for context class
            # except KeyError:
            #     if ber_type == 3:  # Referral in result
            #         value = decode_sequence(message, start, start + ber_len)
            #     else:
            #         raise  # re-raise, should never happen
        decoded.append((ber_class, ber_constructed, ber_type, value))
        start += ber_len

    return decoded


def decode_integer(message, start, stop, context_decoders=None):
    first = message[start]
    value = -1 if get_byte(first) & 0x80 else 0
    for octet in message[start: stop]:
        value = value << 8 | get_byte(octet)

    return value


def decode_octet_string(message, start, stop, context_decoders=None):
    return message[start: stop]


def decode_boolean(message, start, stop, context_decoders=None):
    return False if message[start: stop] == 0 else True


def decode_bind_response(message, start, stop, context_decoders=None):
    return decode_sequence(message, start, stop, BIND_RESPONSE_CONTEXT)


def decode_extended_response(message, start, stop, context_decoders=None):
    return decode_sequence(message, start, stop, EXTENDED_RESPONSE_CONTEXT)


def decode_intermediate_response(message, start, stop, context_decoders=None):
    return decode_sequence(message, start, stop, INTERMEDIATE_RESPONSE_CONTEXT)


def decode_controls(message, start, stop, context_decoders=None):
    return decode_sequence(message, start, stop, CONTROLS_CONTEXT)


def ldap_result_to_dict_fast(response):
    response_dict = dict()
    response_dict['result'] = int(response[0][3])  # resultCode
    response_dict['description'] = RESULT_CODES[response_dict['result']]
    response_dict['dn'] = to_unicode(response[1][3], from_server=True)  # matchedDN
    response_dict['message'] = to_unicode(response[2][3], from_server=True)  # diagnosticMessage
    if len(response) == 4:
        response_dict['referrals'] = referrals_to_list([to_unicode(referral[3], from_server=True) for referral in response[3][3]])  # referrals
    else:
        response_dict['referrals'] = None

    return response_dict


######

if str is not bytes:  # Python 3
    def get_byte(x):
        return x

    def get_bytes(x):
        return x
else:  # Python 2
    def get_byte(x):
        return ord(x)

    def get_bytes(x):
        return bytearray(x)

DECODERS = {
    # Universal
    (0, 1): decode_boolean,  # Boolean
    (0, 2): decode_integer,  # Integer
    (0, 4): decode_octet_string,  # Octet String
    (0, 10): decode_integer,  # Enumerated
    (0, 16): decode_sequence,  # Sequence
    (0, 17): decode_sequence,  # Set
    # Application
    (1, 1): decode_bind_response,  # Bind response
    (1, 4): decode_sequence,  # Search result entry
    (1, 5): decode_sequence,  # Search result done
    (1, 7): decode_sequence,  # Modify response
    (1, 9): decode_sequence,  # Add response
    (1, 11): decode_sequence,  # Delete response
    (1, 13): decode_sequence,  # ModifyDN response
    (1, 15): decode_sequence,  # Compare response
    (1, 19): decode_sequence,  # Search result reference
    (1, 24): decode_extended_response,  # Extended response
    (1, 25): decode_intermediate_response,  # intermediate response
    (2, 3): decode_octet_string  #
}

BIND_RESPONSE_CONTEXT = {
    7: decode_octet_string  # SaslCredentials
}

EXTENDED_RESPONSE_CONTEXT = {
    10: decode_octet_string,  # ResponseName
    11: decode_octet_string  # Response Value
}

INTERMEDIATE_RESPONSE_CONTEXT = {
    0: decode_octet_string,  # IntermediateResponseName
    1: decode_octet_string  # IntermediateResponseValue
}

LDAP_MESSAGE_CONTEXT = {
    0: decode_controls,  # Controls
    3: decode_sequence  # Referral
}

CONTROLS_CONTEXT = {
    0: decode_sequence  # Control
}
