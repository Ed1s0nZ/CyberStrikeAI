"""
"""

# Created on 2016.04.30
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

from .. import ALL_ATTRIBUTES, ALL_OPERATIONAL_ATTRIBUTES, NO_ATTRIBUTES
from .mockBase import MockBaseStrategy
from .asynchronous import AsyncStrategy
from ..operation.search import search_result_done_response_to_dict, search_result_entry_response_to_dict
from ..core.results import DO_NOT_RAISE_EXCEPTIONS
from ..utils.log import log, log_enabled, ERROR, PROTOCOL
from ..core.exceptions import LDAPResponseTimeoutError, LDAPOperationResult
from ..operation.bind import bind_response_to_dict
from ..operation.delete import delete_response_to_dict
from ..operation.add import add_response_to_dict
from ..operation.compare import compare_response_to_dict
from ..operation.modifyDn import modify_dn_response_to_dict
from ..operation.modify import modify_response_to_dict
from ..operation.search import search_result_done_response_to_dict, search_result_entry_response_to_dict
from ..operation.extended import extended_response_to_dict

# LDAPResult ::= SEQUENCE {
#     resultCode         ENUMERATED {
#         success                      (0),
#         operationsError              (1),
#         protocolError                (2),
#         timeLimitExceeded            (3),
#         sizeLimitExceeded            (4),
#         compareFalse                 (5),
#         compareTrue                  (6),
#         authMethodNotSupported       (7),
#         strongerAuthRequired         (8),
#              -- 9 reserved --
#         referral                     (10),
#         adminLimitExceeded           (11),
#         unavailableCriticalExtension (12),
#         confidentialityRequired      (13),
#         saslBindInProgress           (14),
#         noSuchAttribute              (16),
#         undefinedAttributeType       (17),
#         inappropriateMatching        (18),
#         constraintViolation          (19),
#         attributeOrValueExists       (20),
#         invalidAttributeSyntax       (21),
#              -- 22-31 unused --
#         noSuchObject                 (32),
#         aliasProblem                 (33),
#         invalidDNSyntax              (34),
#              -- 35 reserved for undefined isLeaf --
#         aliasDereferencingProblem    (36),
#              -- 37-47 unused --
#         inappropriateAuthentication  (48),
#         invalidCredentials           (49),
#         insufficientAccessRights     (50),
#         busy                         (51),
#         unavailable                  (52),
#         unwillingToPerform           (53),
#         loopDetect                   (54),
#              -- 55-63 unused --
#         namingViolation              (64),
#         objectClassViolation         (65),
#         notAllowedOnNonLeaf          (66),
#         notAllowedOnRDN              (67),
#         entryAlreadyExists           (68),
#         objectClassModsProhibited    (69),
#              -- 70 reserved for CLDAP --
#         affectsMultipleDSAs          (71),
#              -- 72-79 unused --
#         other                        (80),
#         ...  },
#     matchedDN          LDAPDN,
#     diagnosticMessage  LDAPString,
#     referral           [3] Referral OPTIONAL }


class MockAsyncStrategy(MockBaseStrategy, AsyncStrategy):  # class inheritance sequence is important, MockBaseStrategy must be the first one
    """
    This strategy create a mock LDAP server, with asynchronous access
    It can be useful to test LDAP without accessing a real Server
    """
    def __init__(self, ldap_connection):
        AsyncStrategy.__init__(self, ldap_connection)
        MockBaseStrategy.__init__(self)
        #outstanding = dict()  # a dictionary with the message id as key and a tuple (result, response) as value

    def post_send_search(self, payload):
        message_id, message_type, request, controls = payload
        async_response = []
        async_result = dict()
        if message_type == 'searchRequest':
            responses, result = self.mock_search(request, controls)
            result['type'] = 'searchResDone'
            for entry in responses:
                response = search_result_entry_response_to_dict(entry, self.connection.server.schema, self.connection.server.custom_formatter, self.connection.check_names)
                response['type'] = 'searchResEntry'

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

                async_response.append(response)
            async_result = search_result_done_response_to_dict(result)
            async_result['type'] = 'searchResDone'
        self._responses[message_id] = (request, async_result, async_response)
        return message_id

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
        responses.append(result)
        if self.connection.raise_exceptions and result and result['result'] not in DO_NOT_RAISE_EXCEPTIONS:
            if log_enabled(PROTOCOL):
                log(PROTOCOL, 'operation result <%s> for <%s>', result, self.connection)
            raise LDAPOperationResult(result=result['result'], description=result['description'], dn=result['dn'], message=result['message'], response_type=result['type'])
        self._responses[message_id] = (request, result, responses)
        return message_id


    def get_response(self, message_id, timeout=None, get_request=False):
        if message_id in self._responses:
            request, result, response = self._responses.pop(message_id)
        else:
            raise(LDAPResponseTimeoutError('message id not in outstanding queue'))

        if self.connection.raise_exceptions and result and result['result'] not in DO_NOT_RAISE_EXCEPTIONS:
            if log_enabled(PROTOCOL):
                log(PROTOCOL, 'operation result <%s> for <%s>', result, self.connection)
            raise LDAPOperationResult(result=result['result'], description=result['description'], dn=result['dn'], message=result['message'], response_type=result['type'])

        if get_request:
            return response, result, request
        else:
            return response, result
