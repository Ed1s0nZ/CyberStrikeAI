"""
"""

# Created on 2014.04.30
#
# Author: Giovanni Cannata
#
# Copyright 2014 - 2020 Giovanni Cannata
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

from ... import HASHED_NONE
from ...extend.operation import ExtendedOperation
from ...protocol.rfc3062 import PasswdModifyRequestValue, PasswdModifyResponseValue
from ...utils.hashed import hashed
from ...protocol.sasl.sasl import validate_simple_password
from ...utils.dn import safe_dn
from ...core.results import RESULT_SUCCESS

# implements RFC3062


class ModifyPassword(ExtendedOperation):
    def config(self):
        self.request_name = '1.3.6.1.4.1.4203.1.11.1'
        self.request_value = PasswdModifyRequestValue()
        self.asn1_spec = PasswdModifyResponseValue()
        self.response_attribute = 'new_password'

    def __init__(self, connection, user=None, old_password=None, new_password=None, hash_algorithm=None, salt=None, controls=None):
        ExtendedOperation.__init__(self, connection, controls)  # calls super __init__()
        if user:
            if connection.check_names:
                user = safe_dn(user)
            self.request_value['userIdentity'] = user
        if old_password:
            if not isinstance(old_password, bytes):  # bytes are returned raw, as per RFC (4.2)
                old_password = validate_simple_password(old_password, True)
            self.request_value['oldPasswd'] = old_password
        if new_password:
            if not isinstance(new_password, bytes):  # bytes are returned raw, as per RFC (4.2)
                new_password = validate_simple_password(new_password, True)
            if hash_algorithm is None or hash_algorithm == HASHED_NONE:
                self.request_value['newPasswd'] = new_password
            else:
                self.request_value['newPasswd'] = hashed(hash_algorithm, new_password, salt)

    def populate_result(self):
        try:
            self.result[self.response_attribute] = str(self.decoded_response['genPasswd'])
        except TypeError:  # optional field can be absent, so returns True if operation is successful else False
            if self.result['result'] == RESULT_SUCCESS:
                self.result[self.response_attribute] = True
            else:  # change was not successful, raises exception if raise_exception = True in connection or returns the operation result, error code is in result['result']
                self.result[self.response_attribute] = False
                if self.connection.raise_exceptions:
                    from ...core.exceptions import LDAPOperationResult
                    raise LDAPOperationResult(result=self.result['result'], description=self.result['description'], dn=self.result['dn'], message=self.result['message'], response_type=self.result['type'])
