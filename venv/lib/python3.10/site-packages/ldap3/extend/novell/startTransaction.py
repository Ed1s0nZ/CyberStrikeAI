"""
"""

# Created on 2016.04.14
#
# Author: Giovanni Cannata
#
# Copyright 2016 - 2020 Giovanni Cannata
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
from ...protocol.novell import CreateGroupTypeRequestValue, CreateGroupTypeResponseValue, GroupingControlValue
from ...protocol.controls import build_control


class StartTransaction(ExtendedOperation):
    def config(self):
        self.request_name = '2.16.840.1.113719.1.27.103.1'
        self.response_name = '2.16.840.1.113719.1.27.103.1'
        self.request_value = CreateGroupTypeRequestValue()
        self.asn1_spec = CreateGroupTypeResponseValue()

    def __init__(self, connection, controls=None):
        ExtendedOperation.__init__(self, connection, controls)  # calls super __init__()
        self.request_value['createGroupType'] = '2.16.840.1.113719.1.27.103.7'  # transactionGroupingType

    def populate_result(self):
        self.result['cookie'] = int(self.decoded_response['createGroupCookie'])
        try:
            self.result['value'] = self.decoded_response['createGroupValue']
        except TypeError:
            self.result['value'] = None

    def set_response(self):
        try:
            grouping_cookie_value = GroupingControlValue()
            grouping_cookie_value['groupingCookie'] = self.result['cookie']
            self.response_value = build_control('2.16.840.1.113719.1.27.103.7', True, grouping_cookie_value, encode_control_value=True)  # groupingControl
        except TypeError:
            self.response_value = None

