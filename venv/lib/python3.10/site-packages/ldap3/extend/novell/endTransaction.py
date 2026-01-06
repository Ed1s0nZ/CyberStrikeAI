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
from ...protocol.novell import EndGroupTypeRequestValue, EndGroupTypeResponseValue, Sequence
from ...utils.asn1 import decoder


class EndTransaction(ExtendedOperation):
    def config(self):
        self.request_name = '2.16.840.1.113719.1.27.103.2'
        self.response_name = '2.16.840.1.113719.1.27.103.2'
        self.request_value = EndGroupTypeRequestValue()
        self.asn1_spec = EndGroupTypeResponseValue()

    def __init__(self, connection, commit=True, controls=None):
        if controls and len(controls) == 1:
            group_cookie = decoder.decode(controls[0][2], asn1Spec=Sequence())[0][0]  # get the cookie from the built groupingControl
        else:
            group_cookie = None
        controls = None

        ExtendedOperation.__init__(self, connection, controls)  # calls super __init__()
        if group_cookie:
            self.request_value['endGroupCookie'] = group_cookie  # transactionGroupingType
            if not commit:
                self.request_value['endGroupValue'] = ''  # an empty endGroupValue means abort transaction

    def populate_result(self):
        try:
            self.result['value'] = self.decoded_response['endGroupValue']
        except TypeError:
            self.result['value'] = None

    def set_response(self):
        self.response_value = self.result
