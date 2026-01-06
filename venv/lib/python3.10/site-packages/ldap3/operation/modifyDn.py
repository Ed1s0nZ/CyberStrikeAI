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

from ..protocol.rfc4511 import ModifyDNRequest, LDAPDN, RelativeLDAPDN, DeleteOldRDN, NewSuperior, ResultCode
from ..operation.bind import referrals_to_list

# ModifyDNRequest ::= [APPLICATION 12] SEQUENCE {
#     entry           LDAPDN,
#     newrdn          RelativeLDAPDN,
#     deleteoldrdn    BOOLEAN,
#     newSuperior     [0] LDAPDN OPTIONAL }


def modify_dn_operation(dn,
                        new_relative_dn,
                        delete_old_rdn=True,
                        new_superior=None):
    request = ModifyDNRequest()
    request['entry'] = LDAPDN(dn)
    request['newrdn'] = RelativeLDAPDN(new_relative_dn)
    request['deleteoldrdn'] = DeleteOldRDN(delete_old_rdn)
    if new_superior:
        request['newSuperior'] = NewSuperior(new_superior)

    return request


def modify_dn_request_to_dict(request):
    return {'entry': str(request['entry']),
            'newRdn': str(request['newrdn']),
            'deleteOldRdn': bool(request['deleteoldrdn']),
            'newSuperior': str(request['newSuperior']) if request['newSuperior'] is not None and request['newSuperior'].hasValue() else None}


def modify_dn_response_to_dict(response):
    return {'result': int(response['resultCode']),
            'description': ResultCode().getNamedValues().getName(response['resultCode']),
            'dn': str(response['matchedDN']),
            'referrals': referrals_to_list(response['referral']),
            'message': str(response['diagnosticMessage'])}
