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

from ...protocol.novell import Identity
from ...extend.operation import ExtendedOperation


class GetBindDn(ExtendedOperation):
    def config(self):
        self.request_name = '2.16.840.1.113719.1.27.100.31'
        self.response_name = '2.16.840.1.113719.1.27.100.32'
        self.response_attribute = 'identity'
        self.asn1_spec = Identity()

    def populate_result(self):
        try:
            self.result['identity'] = str(self.decoded_response) if self.decoded_response else None
        except TypeError:
            self.result['identity'] = None
