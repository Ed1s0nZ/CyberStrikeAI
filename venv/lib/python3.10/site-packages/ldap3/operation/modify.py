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

from .. import SEQUENCE_TYPES, MODIFY_ADD, MODIFY_DELETE, MODIFY_REPLACE, MODIFY_INCREMENT
from ..protocol.rfc4511 import ModifyRequest, LDAPDN, Changes, Change, Operation, PartialAttribute, AttributeDescription, Vals, ResultCode
from ..operation.bind import referrals_to_list
from ..protocol.convert import changes_to_list, validate_attribute_value, prepare_for_sending

# ModifyRequest ::= [APPLICATION 6] SEQUENCE {
#    object          LDAPDN,
#    changes         SEQUENCE OF change SEQUENCE {
#        operation       ENUMERATED {
#            add     (0),
#            delete  (1),
#            replace (2),
#            ...  },
#    modification    PartialAttribute } }

change_table = {MODIFY_ADD: 0,  # accepts actual values too
                MODIFY_DELETE: 1,
                MODIFY_REPLACE: 2,
                MODIFY_INCREMENT: 3,
                0: 0,
                1: 1,
                2: 2,
                3: 3}


def modify_operation(dn,
                     changes,
                     auto_encode,
                     schema=None,
                     validator=None,
                     check_names=False):
    # changes is a dictionary in the form {'attribute': [(operation, [val1, ...]), ...], ...}
    # operation is 0 (add), 1 (delete), 2 (replace), 3 (increment)
    # increment as per RFC4525

    change_list = Changes()
    pos = 0
    for attribute in changes:
        for change_operation in changes[attribute]:
            partial_attribute = PartialAttribute()
            partial_attribute['type'] = AttributeDescription(attribute)
            partial_attribute['vals'] = Vals()
            if isinstance(change_operation[1], SEQUENCE_TYPES):
                for index, value in enumerate(change_operation[1]):
                    partial_attribute['vals'].setComponentByPosition(index, prepare_for_sending(validate_attribute_value(schema, attribute, value, auto_encode, validator, check_names=check_names)))
            else:
                partial_attribute['vals'].setComponentByPosition(0, prepare_for_sending(validate_attribute_value(schema, attribute, change_operation[1], auto_encode, validator, check_names=check_names)))
            change = Change()
            change['operation'] = Operation(change_table[change_operation[0]])
            change['modification'] = partial_attribute

            change_list[pos] = change
            pos += 1

    request = ModifyRequest()
    request['object'] = LDAPDN(dn)
    request['changes'] = change_list
    return request


def modify_request_to_dict(request):
    return {'entry': str(request['object']),
            'changes': changes_to_list(request['changes'])}


def modify_response_to_dict(response):
    return {'result': int(response['resultCode']),
            'description': ResultCode().getNamedValues().getName(response['resultCode']),
            'message': str(response['diagnosticMessage']),
            'dn': str(response['matchedDN']),
            'referrals': referrals_to_list(response['referral'])}
