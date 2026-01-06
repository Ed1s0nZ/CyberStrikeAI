"""
"""

# Created on 2014.08.07
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

from datetime import datetime

from pyasn1.type.univ import Integer

from ...core.exceptions import LDAPExtensionError
from ...protocol.novell import LDAPDN, ReplicaInfoRequestValue
from ..operation import ExtendedOperation
from ...utils.asn1 import decoder
from ...utils.dn import safe_dn


class ReplicaInfo(ExtendedOperation):
    def config(self):
        self.request_name = '2.16.840.1.113719.1.27.100.17'
        self.response_name = '2.16.840.1.113719.1.27.100.18'
        # self.asn1_spec = ReplicaInfoResponseValue()
        self.request_value = ReplicaInfoRequestValue()
        self.response_attribute = 'partition_dn'

    def __init__(self, connection, server_dn, partition_dn, controls=None):
        if connection.check_names:
            if server_dn:
                server_dn = safe_dn(server_dn)
            if partition_dn:
                partition_dn = safe_dn(partition_dn)

        ExtendedOperation.__init__(self, connection, controls)  # calls super __init__()
        self.request_value['server_dn'] = server_dn
        self.request_value['partition_dn'] = partition_dn

    def populate_result(self):
        substrate = self.decoded_response
        try:
            decoded, substrate = decoder.decode(substrate, asn1Spec=Integer())
            self.result['partition_id'] = int(decoded)
            decoded, substrate = decoder.decode(substrate, asn1Spec=Integer())
            self.result['replica_state'] = int(decoded)
            decoded, substrate = decoder.decode(substrate, asn1Spec=Integer())
            self.result['modification_time'] = datetime.utcfromtimestamp(int(decoded))
            decoded, substrate = decoder.decode(substrate, asn1Spec=Integer())
            self.result['purge_time'] = datetime.utcfromtimestamp(int(decoded))
            decoded, substrate = decoder.decode(substrate, asn1Spec=Integer())
            self.result['local_partition_id'] = int(decoded)
            decoded, substrate = decoder.decode(substrate, asn1Spec=LDAPDN())
            self.result['partition_dn'] = str(decoded)
            decoded, substrate = decoder.decode(substrate, asn1Spec=Integer())
            self.result['replica_type'] = int(decoded)
            decoded, substrate = decoder.decode(substrate, asn1Spec=Integer())
            self.result['flags'] = int(decoded)
        except Exception:
            raise LDAPExtensionError('unable to decode substrate')

        if substrate:
            raise LDAPExtensionError('unknown substrate remaining')
