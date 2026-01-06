"""
"""

# Created on 2013.05.31
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

from pyasn1.type.univ import OctetString
from pyasn1.type.base import Asn1Item

from ..core.results import RESULT_CODES
from ..protocol.rfc4511 import ExtendedRequest, RequestName, ResultCode, RequestValue
from ..protocol.convert import referrals_to_list
from ..utils.asn1 import encode
from ..utils.conv import to_unicode

# ExtendedRequest ::= [APPLICATION 23] SEQUENCE {
#     requestName      [0] LDAPOID,
#     requestValue     [1] OCTET STRING OPTIONAL }


def extended_operation(request_name,
                       request_value=None,
                       no_encode=None):
    request = ExtendedRequest()
    request['requestName'] = RequestName(request_name)
    if request_value and isinstance(request_value, Asn1Item):
        request['requestValue'] = RequestValue(encode(request_value))
    elif str is not bytes and isinstance(request_value, (bytes, bytearray)):  # in Python 3 doesn't try to encode a byte value
        request['requestValue'] = request_value
    elif request_value and no_encode:  # doesn't encode the value
        request['requestValue'] = request_value
    elif request_value:  # tries to encode as a octet string
        request['requestValue'] = RequestValue(encode(OctetString(str(request_value))))

    # elif request_value is not None:
    #     raise LDAPExtensionError('unable to encode value for extended operation')
    return request


def extended_request_to_dict(request):
    # return {'name': str(request['requestName']), 'value': bytes(request['requestValue']) if request['requestValue'] else None}
    return {'name': str(request['requestName']), 'value': bytes(request['requestValue']) if 'requestValue' in request and request['requestValue'] is not None and request['requestValue'].hasValue()  else None}

def extended_response_to_dict(response):
    return {'result': int(response['resultCode']),
            'dn': str(response['matchedDN']),
            'message': str(response['diagnosticMessage']),
            'description': ResultCode().getNamedValues().getName(response['resultCode']),
            'referrals': referrals_to_list(response['referral']),
            'responseName': str(response['responseName']) if response['responseName'] is not None and response['responseName'].hasValue() else str(),
            'responseValue': bytes(response['responseValue']) if response['responseValue'] is not None and response['responseValue'].hasValue() else bytes()}


def intermediate_response_to_dict(response):
    return {'responseName': str(response['responseName']),
            'responseValue': bytes(response['responseValue']) if response['responseValue'] else bytes()}


def extended_response_to_dict_fast(response):
    response_dict = dict()
    response_dict['result'] = int(response[0][3])  # resultCode
    response_dict['description'] = RESULT_CODES[response_dict['result']]
    response_dict['dn'] = to_unicode(response[1][3], from_server=True)  # matchedDN
    response_dict['message'] = to_unicode(response[2][3], from_server=True)  # diagnosticMessage
    response_dict['referrals'] = None  # referrals
    response_dict['responseName'] = None  # referrals
    response_dict['responseValue'] = None  # responseValue

    for r in response[3:]:
        if r[2] == 3:  # referrals
            response_dict['referrals'] = referrals_to_list(r[3])  # referrals
        elif r[2] == 10:  # responseName
            response_dict['responseName'] = to_unicode(r[3], from_server=True)
            response_dict['responseValue'] = b''  # responseValue could be empty

        else:  # responseValue (11)
            response_dict['responseValue'] = bytes(r[3])

    return response_dict


def intermediate_response_to_dict_fast(response):
    response_dict = dict()
    for r in response:
        if r[2] == 0:  # responseName
            response_dict['responseName'] = to_unicode(r[3], from_server=True)
        else:  # responseValue (1)
            response_dict['responseValue'] = bytes(r[3])

    return response_dict
