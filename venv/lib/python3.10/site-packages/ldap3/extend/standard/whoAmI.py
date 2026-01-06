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

# implements RFC4532
from ...extend.operation import ExtendedOperation
from ...utils.conv import to_unicode


class WhoAmI(ExtendedOperation):
    def config(self):
        self.request_name = '1.3.6.1.4.1.4203.1.11.3'
        self.response_attribute = 'authzid'

    def populate_result(self):
        try:
            self.result['authzid'] = to_unicode(self.decoded_response) if self.decoded_response else None
        except TypeError:
            self.result['authzid'] = self.decoded_response if self.decoded_response else None
