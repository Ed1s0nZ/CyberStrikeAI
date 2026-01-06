"""
"""

# Created on 2014.07.04
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

from ..core.results import RESULT_SUCCESS
from ..core.exceptions import LDAPExtensionError
from ..utils.asn1 import decoder


class ExtendedOperation(object):
    def __init__(self, connection, controls=None):
        self.connection = connection
        self.decoded_response = None
        self.result = None
        self.asn1_spec = None  # if None the response_value is returned without encoding
        self.request_name = None
        self.response_name = None
        self.request_value = None
        self.response_value = None
        self.response_attribute = None
        self.controls = controls
        self.config()

    def send(self):
        if self.connection.check_names and self.connection.server.info is not None and self.connection.server.info.supported_extensions is not None:  # checks if extension is supported
            for request_name in self.connection.server.info.supported_extensions:
                if request_name[0] == self.request_name:
                    break
            else:
                raise LDAPExtensionError('extension not in DSA list of supported extensions')

        resp = self.connection.extended(self.request_name, self.request_value, self.controls)
        if not self.connection.strategy.sync:
            _, result = self.connection.get_response(resp)
        else:
            if self.connection.strategy.thread_safe:
                _, result, _, _ = resp
            else:
                result = self.connection.result
        self.result = result
        self.decode_response(result)
        self.populate_result()
        self.set_response()
        return self.response_value

    def populate_result(self):
        pass

    def decode_response(self, response=None):
        if not response:
            response = self.result
        if not response:
            return None
        if response['result'] not in [RESULT_SUCCESS]:
            if self.connection.raise_exceptions:
                raise LDAPExtensionError('extended operation error: ' + response['description'] + ' - ' + response['message'])
            else:
                return None
        if not self.response_name or response['responseName'] == self.response_name:
            if response['responseValue']:
                if self.asn1_spec is not None:
                    decoded, unprocessed = decoder.decode(response['responseValue'], asn1Spec=self.asn1_spec)
                    if unprocessed:
                        raise LDAPExtensionError('error decoding extended response value')
                    self.decoded_response = decoded
                else:
                    self.decoded_response = response['responseValue']
        else:
            raise LDAPExtensionError('invalid response name received')

    def set_response(self):
        self.response_value = self.result[self.response_attribute] if self.result and self.response_attribute in self.result else None
        if not self.connection.strategy.thread_safe:
            self.connection.response = self.response_value

    def config(self):
        pass
