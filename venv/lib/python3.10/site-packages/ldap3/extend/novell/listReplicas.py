"""
"""

# Created on 2014.07.03
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

from ...extend.operation import ExtendedOperation
from ...protocol.novell import ReplicaList
from ...protocol.rfc4511 import LDAPDN
from ...utils.dn import safe_dn


class ListReplicas(ExtendedOperation):
    def config(self):
        self.request_name = '2.16.840.1.113719.1.27.100.19'
        self.response_name = '2.16.840.1.113719.1.27.100.20'
        self.request_value = LDAPDN()
        self.asn1_spec = ReplicaList()
        self.response_attribute = 'replicas'

    def __init__(self, connection, server_dn, controls=None):
        ExtendedOperation.__init__(self, connection, controls)  # calls super __init__()
        if connection.check_names:
            server_dn = safe_dn(server_dn)
        self.request_value = LDAPDN(server_dn)

    def populate_result(self):
        try:
            self.result['replicas'] = [str(replica) for replica in self.decoded_response] if self.decoded_response else None
        except TypeError:
            self.result['replicas'] = None
