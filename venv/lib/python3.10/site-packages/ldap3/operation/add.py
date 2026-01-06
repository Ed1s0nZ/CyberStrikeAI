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

from .. import SEQUENCE_TYPES
from ..protocol.rfc4511 import AddRequest, LDAPDN, AttributeList, Attribute, AttributeDescription, ResultCode, Vals
from ..protocol.convert import referrals_to_list, attributes_to_dict, validate_attribute_value, prepare_for_sending


def add_operation(dn,
                  attributes,
                  auto_encode,
                  schema=None,
                  validator=None,
                  check_names=False):
    # AddRequest ::= [APPLICATION 8] SEQUENCE {
    #     entry           LDAPDN,
    #     attributes      AttributeList }
    #
    # attributes is a dictionary in the form 'attribute': ['val1', 'val2', 'valN']
    attribute_list = AttributeList()
    for pos, attribute in enumerate(attributes):
        attribute_list[pos] = Attribute()
        attribute_list[pos]['type'] = AttributeDescription(attribute)
        vals = Vals()  # changed from ValsAtLeast1() for allowing empty member value in groups
        if isinstance(attributes[attribute], SEQUENCE_TYPES):
            for index, value in enumerate(attributes[attribute]):
                vals.setComponentByPosition(index, prepare_for_sending(validate_attribute_value(schema, attribute, value, auto_encode, validator, check_names)))
        else:
            vals.setComponentByPosition(0, prepare_for_sending(validate_attribute_value(schema, attribute, attributes[attribute], auto_encode, validator, check_names)))

        attribute_list[pos]['vals'] = vals

    request = AddRequest()
    request['entry'] = LDAPDN(dn)
    request['attributes'] = attribute_list

    return request


def add_request_to_dict(request):
    return {'entry': str(request['entry']),
            'attributes': attributes_to_dict(request['attributes'])}


def add_response_to_dict(response):
    return {'result': int(response['resultCode']),
            'description': ResultCode().getNamedValues().getName(response['resultCode']),
            'dn': str(response['matchedDN']),
            'message': str(response['diagnosticMessage']),
            'referrals': referrals_to_list(response['referral'])}
