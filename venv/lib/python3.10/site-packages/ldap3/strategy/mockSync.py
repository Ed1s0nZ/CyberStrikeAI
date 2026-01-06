"""
"""

# Created on 2014.11.17
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

from ..core.results import DO_NOT_RAISE_EXCEPTIONS
from .mockBase import MockBaseStrategy
from .. import ALL_ATTRIBUTES, ALL_OPERATIONAL_ATTRIBUTES, NO_ATTRIBUTES
from .sync import SyncStrategy
from ..operation.bind import bind_response_to_dict
from ..operation.delete import delete_response_to_dict
from ..operation.add import add_response_to_dict
from ..operation.compare import compare_response_to_dict
from ..operation.modifyDn import modify_dn_response_to_dict
from ..operation.modify import modify_response_to_dict
from ..operation.search import search_result_done_response_to_dict, search_result_entry_response_to_dict
from ..operation.extended import extended_response_to_dict
from ..core.exceptions import LDAPSocketOpenError, LDAPOperationResult
from ..utils.log import log, log_enabled, ERROR, PROTOCOL


class MockSyncStrategy(MockBaseStrategy, SyncStrategy):  # class inheritance sequence is important, MockBaseStrategy must be the first one
    """
    This strategy create a mock LDAP server, with synchronous access
    It can be useful to test LDAP without accessing a real Server
    """
    def __init__(self, ldap_connection):
        SyncStrategy.__init__(self, ldap_connection)
        MockBaseStrategy.__init__(self)

    def post_send_search(self, payload):
        message_id, message_type, request, controls = payload
        self.connection.response = []
        self.connection.result = dict()
        if message_type == 'searchRequest':
            responses, result = self.mock_search(request, controls)
            for entry in responses:
                response = search_result_entry_response_to_dict(entry, self.connection.server.schema, self.connection.server.custom_formatter, self.connection.check_names)
                response['type'] = 'searchResEntry'
                ###
                if self.connection.empty_attributes:
                    for attribute_type in request['attributes']:
                        attribute_name = str(attribute_type)
                        if attribute_name not in response['raw_attributes'] and attribute_name not in (ALL_ATTRIBUTES, ALL_OPERATIONAL_ATTRIBUTES, NO_ATTRIBUTES):
                            response['raw_attributes'][attribute_name] = list()
                            response['attributes'][attribute_name] = list()
                            if log_enabled(PROTOCOL):
                                log(PROTOCOL, 'attribute set to empty list for missing attribute <%s> in <%s>',
                                    attribute_type, self)
                    if not self.connection.auto_range:
                        attrs_to_remove = []
                        # removes original empty attribute in case a range tag is returned
                        for attribute_type in response['attributes']:
                            attribute_name = str(attribute_type)
                            if ';range' in attribute_name.lower():
                                orig_attr, _, _ = attribute_name.partition(';')
                                attrs_to_remove.append(orig_attr)
                        for attribute_type in attrs_to_remove:
                            if log_enabled(PROTOCOL):
                                log(PROTOCOL,
                                    'attribute type <%s> removed in response because of same attribute returned as range by the server in <%s>',
                                    attribute_type, self)
                            del response['raw_attributes'][attribute_type]
                            del response['attributes'][attribute_type]
                ###
                self.connection.response.append(response)
            result = search_result_done_response_to_dict(result)
            result['type'] = 'searchResDone'
            self.connection.result = result
            if self.connection.raise_exceptions and result and result['result'] not in DO_NOT_RAISE_EXCEPTIONS:
                if log_enabled(PROTOCOL):
                    log(PROTOCOL, 'operation result <%s> for <%s>', result, self.connection)
                raise LDAPOperationResult(result=result['result'], description=result['description'], dn=result['dn'], message=result['message'], response_type=result['type'])

        return self.connection.response

    def post_send_single_response(self, payload):  # payload is a tuple sent by self.send() made of message_type, request, controls
        message_id, message_type, request, controls = payload
        responses = []
        result = None
        if message_type == 'bindRequest':
            result = bind_response_to_dict(self.mock_bind(request, controls))
            result['type'] = 'bindResponse'
        elif message_type == 'unbindRequest':
            self.bound = None
        elif message_type == 'abandonRequest':
            pass
        elif message_type == 'delRequest':
            result = delete_response_to_dict(self.mock_delete(request, controls))
            result['type'] = 'delResponse'
        elif message_type == 'addRequest':
            result = add_response_to_dict(self.mock_add(request, controls))
            result['type'] = 'addResponse'
        elif message_type == 'compareRequest':
            result = compare_response_to_dict(self.mock_compare(request, controls))
            result['type'] = 'compareResponse'
        elif message_type == 'modDNRequest':
            result = modify_dn_response_to_dict(self.mock_modify_dn(request, controls))
            result['type'] = 'modDNResponse'
        elif message_type == 'modifyRequest':
            result = modify_response_to_dict(self.mock_modify(request, controls))
            result['type'] = 'modifyResponse'
        elif message_type == 'extendedReq':
            result = extended_response_to_dict(self.mock_extended(request, controls))
            result['type'] = 'extendedResp'
        self.connection.result = result
        responses.append(result)
        if self.connection.raise_exceptions and result and result['result'] not in DO_NOT_RAISE_EXCEPTIONS:
            if log_enabled(PROTOCOL):
                log(PROTOCOL, 'operation result <%s> for <%s>', result, self.connection)
            raise LDAPOperationResult(result=result['result'], description=result['description'], dn=result['dn'], message=result['message'], response_type=result['type'])
        return responses

