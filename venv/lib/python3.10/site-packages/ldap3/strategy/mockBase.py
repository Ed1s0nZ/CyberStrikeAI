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

import json
import re

from random import SystemRandom

from pyasn1.type.univ import OctetString

from .. import SEQUENCE_TYPES, ALL_ATTRIBUTES
from ..operation.bind import bind_request_to_dict
from ..operation.delete import delete_request_to_dict
from ..operation.add import add_request_to_dict
from ..operation.compare import compare_request_to_dict
from ..operation.modifyDn import modify_dn_request_to_dict
from ..operation.modify import modify_request_to_dict
from ..operation.extended import extended_request_to_dict
from ..operation.search import search_request_to_dict, parse_filter, ROOT, AND, OR, NOT, MATCH_APPROX, \
    MATCH_GREATER_OR_EQUAL, MATCH_LESS_OR_EQUAL, MATCH_EXTENSIBLE, MATCH_PRESENT,\
    MATCH_SUBSTRING, MATCH_EQUAL
from ..utils.conv import json_hook, to_unicode, to_raw
from ..core.exceptions import LDAPDefinitionError, LDAPPasswordIsMandatoryError, LDAPInvalidValueError, LDAPSocketOpenError
from ..core.results import RESULT_SUCCESS, RESULT_OPERATIONS_ERROR, RESULT_UNAVAILABLE_CRITICAL_EXTENSION, \
    RESULT_INVALID_CREDENTIALS, RESULT_NO_SUCH_OBJECT, RESULT_ENTRY_ALREADY_EXISTS, RESULT_COMPARE_TRUE, \
    RESULT_COMPARE_FALSE, RESULT_NO_SUCH_ATTRIBUTE, RESULT_UNWILLING_TO_PERFORM, RESULT_PROTOCOL_ERROR, RESULT_CONSTRAINT_VIOLATION, RESULT_NOT_ALLOWED_ON_RDN
from ..utils.ciDict import CaseInsensitiveDict
from ..utils.dn import to_dn, safe_dn, safe_rdn
from ..protocol.sasl.sasl import validate_simple_password
from ..protocol.formatters.standard import find_attribute_validator, format_attribute_values
from ..protocol.rfc2696 import paged_search_control
from ..utils.log import log, log_enabled, ERROR, BASIC
from ..utils.asn1 import encode
from ..utils.conv import ldap_escape_to_bytes
from ..strategy.base import BaseStrategy  # needed for decode_control() method
from ..protocol.rfc4511 import LDAPMessage, ProtocolOp, MessageID
from ..protocol.convert import build_controls_list


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

# noinspection PyProtectedMember,PyUnresolvedReferences

SEARCH_CONTROLS = ['1.2.840.113556.1.4.319'  # simple paged search [RFC 2696]
                   ]
SERVER_ENCODING = 'utf-8'


def random_cookie():
    return to_raw(SystemRandom().random())[-6:]


class PagedSearchSet(object):
    def __init__(self, response, size, criticality):
        self.size = size
        self.response = response
        self.cookie = None
        self.sent = 0
        self.done = False

    def next(self, size=None):
        if size:
            self.size=size

        message = ''
        response = self.response[self.sent: self.sent + self.size]
        self.sent += self.size
        if self.sent > len(self.response):
            self.done = True
            self.cookie = ''
        else:
            self.cookie = random_cookie()

        response_control = paged_search_control(False, len(self.response), self.cookie)
        result = {'resultCode': RESULT_SUCCESS,
                  'matchedDN': '',
                  'diagnosticMessage': to_unicode(message, SERVER_ENCODING),
                  'referral': None,
                  'controls': [BaseStrategy.decode_control(response_control)]
                  }
        return response, result


class MockBaseStrategy(object):
    """
    Base class for connection strategy
    """

    def __init__(self):
        if not hasattr(self.connection.server, 'dit'):  # create entries dict if not already present
            self.connection.server.dit = CaseInsensitiveDict()
        self.entries = self.connection.server.dit  # for simpler reference
        self.no_real_dsa = True
        self.bound = None
        self.custom_validators = None
        self.operational_attributes = ['entryDN']
        self.add_entry('cn=schema', [], validate=False)  # add default entry for schema
        self._paged_sets = []  # list of paged search in progress
        if log_enabled(BASIC):
            log(BASIC, 'instantiated <%s>: <%s>', self.__class__.__name__, self)

    def _start_listen(self):
        self.connection.listening = True
        self.connection.closed = False
        if self.connection.usage:
            self.connection._usage.open_sockets += 1

    def _stop_listen(self):
        self.connection.listening = False
        self.connection.closed = True
        if self.connection.usage:
            self.connection._usage.closed_sockets += 1

    def _prepare_value(self, attribute_type, value, validate=True):
        """
        Prepare a value for being stored in the mock DIT
        :param value: object to store
        :return: raw value to store in the DIT
        """
        if validate:  # if loading from json dump do not validate values:
            validator = find_attribute_validator(self.connection.server.schema, attribute_type, self.custom_validators)
            validated = validator(value)
            if validated is False:
                raise LDAPInvalidValueError('value non valid for attribute \'%s\'' % attribute_type)
            elif validated is not True:  # a valid LDAP value equivalent to the actual value
                value = validated
        raw_value = to_raw(value)
        if not isinstance(raw_value, bytes):
            raise LDAPInvalidValueError('The value "%s" of type %s for "%s" must be bytes or an offline schema needs to be provided when Mock strategy is used.' % (
                value,
                type(value),
                attribute_type,
            ))
        return raw_value

    def _update_attribute(self, dn, attribute_type, value):
        pass

    def add_entry(self, dn, attributes, validate=True):
        with self.connection.server.dit_lock:
            escaped_dn = safe_dn(dn)
            if escaped_dn not in self.connection.server.dit:
                new_entry = CaseInsensitiveDict()
                for attribute in attributes:
                    if attribute in self.operational_attributes:  # no restore of operational attributes, should be computed at runtime
                        continue
                    if not isinstance(attributes[attribute], SEQUENCE_TYPES):  # entry attributes are always lists of bytes values
                        attributes[attribute] = [attributes[attribute]]
                    if self.connection.server.schema and self.connection.server.schema.attribute_types[attribute].single_value and len(attributes[attribute]) > 1:  # multiple values in single-valued attribute
                        return False
                    if attribute.lower() == 'objectclass' and self.connection.server.schema:  # builds the objectClass hierarchy only if schema is present
                        class_set = set()
                        for object_class in attributes[attribute]:
                            if self.connection.server.schema.object_classes and object_class not in self.connection.server.schema.object_classes:
                                return False
                            # walkups the class hierarchy and buils a set of all classes in it
                            class_set.add(object_class)
                            class_set_size = 0
                            while class_set_size != len(class_set):
                                new_classes = set()
                                class_set_size = len(class_set)
                                for class_name in class_set:
                                    if self.connection.server.schema.object_classes[class_name].superior:
                                        new_classes.update(self.connection.server.schema.object_classes[class_name].superior)
                                class_set.update(new_classes)
                            new_entry['objectClass'] = [to_raw(value) for value in class_set]
                    else:
                        new_entry[attribute] = [self._prepare_value(attribute, value, validate) for value in attributes[attribute]]
                for rdn in safe_rdn(escaped_dn, decompose=True):  # adds rdns to entry attributes
                    if rdn[0] not in new_entry:  # if rdn attribute is missing adds attribute and its value
                        new_entry[rdn[0]] = [to_raw(rdn[1])]
                    else:
                        raw_rdn = to_raw(rdn[1])
                        if raw_rdn not in new_entry[rdn[0]]:  # add rdn value if rdn attribute is present but value is missing
                            new_entry[rdn[0]].append(raw_rdn)
                new_entry['entryDN'] = [to_raw(escaped_dn)]
                self.connection.server.dit[escaped_dn] = new_entry
                return True
            return False

    def remove_entry(self, dn):
        with self.connection.server.dit_lock:
            escaped_dn = safe_dn(dn)
            if escaped_dn in self.connection.server.dit:
                del self.connection.server.dit[escaped_dn]
                return True
            return False

    def entries_from_json(self, json_entry_file):
        target = open(json_entry_file, 'r')
        definition = json.load(target, object_hook=json_hook)
        if 'entries' not in definition:
            self.connection.last_error = 'invalid JSON definition, missing "entries" section'
            if log_enabled(ERROR):
                log(ERROR, '<%s> for <%s>', self.connection.last_error, self.connection)
            raise LDAPDefinitionError(self.connection.last_error)
        if not self.connection.server.dit:
            self.connection.server.dit = CaseInsensitiveDict()
        for entry in definition['entries']:
            if 'raw' not in entry:
                self.connection.last_error = 'invalid JSON definition, missing "raw" section'
                if log_enabled(ERROR):
                    log(ERROR, '<%s> for <%s>', self.connection.last_error, self.connection)
                raise LDAPDefinitionError(self.connection.last_error)
            if 'dn' not in entry:
                self.connection.last_error = 'invalid JSON definition, missing "dn" section'
                if log_enabled(ERROR):
                    log(ERROR, '<%s> for <%s>', self.connection.last_error, self.connection)
                raise LDAPDefinitionError(self.connection.last_error)
            self.add_entry(entry['dn'], entry['raw'], validate=False)
        target.close()

    def mock_bind(self, request_message, controls):
        # BindRequest ::= [APPLICATION 0] SEQUENCE {
        #     version                 INTEGER (1 ..  127),
        #     name                    LDAPDN,
        #     authentication          AuthenticationChoice }
        #
        # BindResponse ::= [APPLICATION 1] SEQUENCE {
        #     COMPONENTS OF LDAPResult,
        #     serverSaslCreds    [7] OCTET STRING OPTIONAL }
        #
        # request: version, name, authentication
        # response: LDAPResult + serverSaslCreds
        request = bind_request_to_dict(request_message)
        identity = request['name']
        if 'simple' in request['authentication']:
            try:
                password = validate_simple_password(request['authentication']['simple'])
            except LDAPPasswordIsMandatoryError:
                password = ''
                identity = '<anonymous>'
        else:
            self.connection.last_error = 'only Simple Bind allowed in Mock strategy'
            if log_enabled(ERROR):
                log(ERROR, '<%s> for <%s>', self.connection.last_error, self.connection)
            raise LDAPDefinitionError(self.connection.last_error)
        # checks userPassword for password. userPassword must be a text string or a list of text strings
        if identity in self.connection.server.dit:
            if 'userPassword' in self.connection.server.dit[identity]:
                # if self.connection.server.dit[identity]['userPassword'] == password or password in self.connection.server.dit[identity]['userPassword']:
                if self.equal(identity, 'userPassword', password):
                    result_code = RESULT_SUCCESS
                    message = ''
                    self.bound = identity
                else:
                    result_code = RESULT_INVALID_CREDENTIALS
                    message = 'invalid credentials'
            else:  # no user found, returns invalidCredentials
                result_code = RESULT_INVALID_CREDENTIALS
                message = 'missing userPassword attribute'
        elif identity == '<anonymous>':
            result_code = RESULT_SUCCESS
            message = ''
            self.bound = identity
        else:
            result_code = RESULT_INVALID_CREDENTIALS
            message = 'missing object'

        return {'resultCode': result_code,
                'matchedDN': '',
                'diagnosticMessage': to_unicode(message, SERVER_ENCODING),
                'referral': None,
                'serverSaslCreds': None
                }

    def mock_delete(self, request_message, controls):
        # DelRequest ::= [APPLICATION 10] LDAPDN
        #
        # DelResponse ::= [APPLICATION 11] LDAPResult
        #
        # request: entry
        # response: LDAPResult
        request = delete_request_to_dict(request_message)
        dn = safe_dn(request['entry'])
        if dn in self.connection.server.dit:
            del self.connection.server.dit[dn]
            result_code = RESULT_SUCCESS
            message = ''
        else:
            result_code = RESULT_NO_SUCH_OBJECT
            message = 'object not found'

        return {'resultCode': result_code,
                'matchedDN': '',
                'diagnosticMessage': to_unicode(message, SERVER_ENCODING),
                'referral': None
                }

    def mock_add(self, request_message, controls):
        # AddRequest ::= [APPLICATION 8] SEQUENCE {
        #     entry           LDAPDN,
        #     attributes      AttributeList }
        #
        # AddResponse ::= [APPLICATION 9] LDAPResult
        #
        # request: entry, attributes
        # response: LDAPResult
        request = add_request_to_dict(request_message)
        dn = safe_dn(request['entry'])
        attributes = request['attributes']
        # converts attributes values to bytes

        if dn not in self.connection.server.dit:
            if self.add_entry(dn, attributes):
                result_code = RESULT_SUCCESS
                message = ''
            else:
                result_code = RESULT_OPERATIONS_ERROR
                message = 'error adding entry'
        else:
            result_code = RESULT_ENTRY_ALREADY_EXISTS
            message = 'entry already exist'

        return {'resultCode': result_code,
                'matchedDN': '',
                'diagnosticMessage': to_unicode(message, SERVER_ENCODING),
                'referral': None
                }

    def mock_compare(self, request_message, controls):
        # CompareRequest ::= [APPLICATION 14] SEQUENCE {
        #     entry           LDAPDN,
        #     ava             AttributeValueAssertion }
        #
        # CompareResponse ::= [APPLICATION 15] LDAPResult
        #
        # request: entry, attribute, value
        # response: LDAPResult
        request = compare_request_to_dict(request_message)
        dn = safe_dn(request['entry'])
        attribute = request['attribute']
        value = to_raw(request['value'])
        if dn in self.connection.server.dit:
            if attribute in self.connection.server.dit[dn]:
                if self.equal(dn, attribute, value):
                    result_code = RESULT_COMPARE_TRUE
                    message = ''
                else:
                    result_code = RESULT_COMPARE_FALSE
                    message = ''
            else:
                result_code = RESULT_NO_SUCH_ATTRIBUTE
                message = 'attribute not found'
        else:
            result_code = RESULT_NO_SUCH_OBJECT
            message = 'object not found'

        return {'resultCode': result_code,
                'matchedDN': '',
                'diagnosticMessage': to_unicode(message, SERVER_ENCODING),
                'referral': None
                }

    def mock_modify_dn(self, request_message, controls):
        # ModifyDNRequest ::= [APPLICATION 12] SEQUENCE {
        #     entry           LDAPDN,
        #     newrdn          RelativeLDAPDN,
        #     deleteoldrdn    BOOLEAN,
        #     newSuperior     [0] LDAPDN OPTIONAL }
        #
        # ModifyDNResponse ::= [APPLICATION 13] LDAPResult
        #
        # request: entry, newRdn, deleteOldRdn, newSuperior
        # response: LDAPResult
        request = modify_dn_request_to_dict(request_message)
        dn = safe_dn(request['entry'])
        new_rdn = request['newRdn']
        delete_old_rdn = request['deleteOldRdn']
        new_superior = safe_dn(request['newSuperior']) if request['newSuperior'] else ''
        dn_components = to_dn(dn)
        if dn in self.connection.server.dit:
            if new_superior and new_rdn:  # performs move in the DIT
                new_dn = safe_dn(dn_components[0] + ',' + new_superior)
                self.connection.server.dit[new_dn] = self.connection.server.dit[dn].copy()
                moved_entry = self.connection.server.dit[new_dn]
                if delete_old_rdn:
                    del self.connection.server.dit[dn]
                result_code = RESULT_SUCCESS
                message = 'entry moved'
                moved_entry['entryDN'] = [to_raw(new_dn)]
            elif new_rdn and not new_superior:  # performs rename
                new_dn = safe_dn(new_rdn + ',' + safe_dn(dn_components[1:]))
                self.connection.server.dit[new_dn] = self.connection.server.dit[dn].copy()
                renamed_entry = self.connection.server.dit[new_dn]
                del self.connection.server.dit[dn]
                renamed_entry['entryDN'] = [to_raw(new_dn)]

                for rdn in safe_rdn(new_dn, decompose=True):  # adds rdns to entry attributes
                    renamed_entry[rdn[0]] = [to_raw(rdn[1])]

                result_code = RESULT_SUCCESS
                message = 'entry rdn renamed'
            else:
                result_code = RESULT_UNWILLING_TO_PERFORM
                message = 'newRdn or newSuperior missing'
        else:
            result_code = RESULT_NO_SUCH_OBJECT
            message = 'object not found'

        return {'resultCode': result_code,
                'matchedDN': '',
                'diagnosticMessage': to_unicode(message, SERVER_ENCODING),
                'referral': None
                }

    def mock_modify(self, request_message, controls):
        # ModifyRequest ::= [APPLICATION 6] SEQUENCE {
        #     object          LDAPDN,
        #     changes         SEQUENCE OF change SEQUENCE {
        #         operation       ENUMERATED {
        #             add     (0),
        #             delete  (1),
        #             replace (2),
        #             ...  },
        #         modification    PartialAttribute } }
        #
        # ModifyResponse ::= [APPLICATION 7] LDAPResult
        #
        # request: entry, changes
        # response: LDAPResult
        #
        # changes is a dictionary in the form {'attribute': [(operation, [val1, ...]), ...], ...}
        # operation is 0 (add), 1 (delete), 2 (replace), 3 (increment)
        request = modify_request_to_dict(request_message)
        dn = safe_dn(request['entry'])
        changes = request['changes']
        result_code = 0
        message = ''
        rdns = [rdn[0] for rdn in safe_rdn(dn, decompose=True)]
        if dn in self.connection.server.dit:
            entry = self.connection.server.dit[dn]
            original_entry = entry.copy()  # to preserve atomicity of operation
            for modification in changes:
                operation = modification['operation']
                attribute = modification['attribute']['type']
                elements = modification['attribute']['value']
                if operation == 0:  # add
                    if attribute not in entry and elements:  # attribute not present, creates the new attribute and add elements
                        if self.connection.server.schema and self.connection.server.schema.attribute_types and self.connection.server.schema.attribute_types[attribute].single_value and len(elements) > 1:  # multiple values in single-valued attribute
                            result_code = RESULT_CONSTRAINT_VIOLATION
                            message = 'attribute is single-valued'
                        else:
                            entry[attribute] = [to_raw(element) for element in elements]
                    else:  # attribute present, adds elements to current values
                        if self.connection.server.schema and self.connection.server.schema.attribute_types and self.connection.server.schema.attribute_types[attribute].single_value:  # multiple values in single-valued attribute
                            result_code = RESULT_CONSTRAINT_VIOLATION
                            message = 'attribute is single-valued'
                        else:
                            entry[attribute].extend([to_raw(element) for element in elements])
                elif operation == 1:  # delete
                    if attribute not in entry:  # attribute must exist
                        result_code = RESULT_NO_SUCH_ATTRIBUTE
                        message = 'attribute must exists for deleting its values'
                    elif attribute in rdns:  # attribute can't be used in dn
                        result_code = RESULT_NOT_ALLOWED_ON_RDN
                        message = 'cannot delete an rdn'
                    else:
                        if not elements:  # deletes whole attribute if element list is empty
                            del entry[attribute]
                        else:
                            for element in elements:
                                raw_element = to_raw(element)
                                if self.equal(dn, attribute, raw_element):  # removes single element
                                    entry[attribute].remove(raw_element)
                                else:
                                    result_code = 1
                                    message = 'value to delete not found'
                            if not entry[attribute]:  # removes the whole attribute if no elements remained
                                del entry[attribute]
                elif operation == 2:  # replace
                    if attribute not in entry and elements:  # attribute not present, creates the new attribute and add elements
                        if self.connection.server.schema and self.connection.server.schema.attribute_types and self.connection.server.schema.attribute_types[attribute].single_value and len(elements) > 1:  # multiple values in single-valued attribute
                            result_code = RESULT_CONSTRAINT_VIOLATION
                            message = 'attribute is single-valued'
                        else:
                            entry[attribute] = [to_raw(element) for element in elements]
                    elif not elements and attribute in rdns:  # attribute can't be used in dn
                        result_code = RESULT_NOT_ALLOWED_ON_RDN
                        message = 'cannot replace an rdn'
                    elif not elements:  # deletes whole attribute if element list is empty
                        if attribute in entry:
                            del entry[attribute]
                    else:  # substitutes elements
                        entry[attribute] = [to_raw(element) for element in elements]
                elif operation == 3:  # increment
                    if attribute not in entry:  # attribute must exist
                        result_code = RESULT_NO_SUCH_ATTRIBUTE
                        message = 'attribute must exists for incrementing its values'
                    else:
                        if len(elements) != 1:
                            result_code = RESULT_PROTOCOL_ERROR
                            message = 'only one increment value is allowed'
                        else:
                            try:
                                entry[attribute] = [bytes(str(int(value) + int(elements[0])), encoding='utf-8') for value in entry[attribute]]
                            except:
                                result_code = RESULT_UNWILLING_TO_PERFORM
                                message = 'unable to increment value'

            if result_code:  # an error has happened, restores the original dn
                self.connection.server.dit[dn] = original_entry
        else:
            result_code = RESULT_NO_SUCH_OBJECT
            message = 'object not found'

        return {'resultCode': result_code,
                'matchedDN': '',
                'diagnosticMessage': to_unicode(message, SERVER_ENCODING),
                'referral': None
                }

    def mock_search(self, request_message, controls):
        # SearchRequest ::= [APPLICATION 3] SEQUENCE {
        #     baseObject      LDAPDN,
        #     scope           ENUMERATED {
        #         baseObject              (0),
        #         singleLevel             (1),
        #         wholeSubtree            (2),
        #     ...  },
        #     derefAliases    ENUMERATED {
        #         neverDerefAliases       (0),
        #         derefInSearching        (1),
        #         derefFindingBaseObj     (2),
        #         derefAlways             (3) },
        #     sizeLimit       INTEGER (0 ..  maxInt),
        #     timeLimit       INTEGER (0 ..  maxInt),
        #     typesOnly       BOOLEAN,
        #     filter          Filter,
        #     attributes      AttributeSelection }
        #
        # SearchResultEntry ::= [APPLICATION 4] SEQUENCE {
        #     objectName      LDAPDN,
        #     attributes      PartialAttributeList }
        #
        #
        # SearchResultReference ::= [APPLICATION 19] SEQUENCE
        #     SIZE (1..MAX) OF uri URI
        #
        # SearchResultDone ::= [APPLICATION 5] LDAPResult
        #
        # request: base, scope, dereferenceAlias, sizeLimit, timeLimit, typesOnly, filter, attributes
        # response_entry: object, attributes
        # response_done: LDAPResult
        request = search_request_to_dict(request_message)
        if controls:
            decoded_controls = [self.decode_control(control) for control in controls if control]
            for decoded_control in decoded_controls:
                if decoded_control[1]['criticality'] and decoded_control[0] not in SEARCH_CONTROLS:
                    message = 'Critical requested control ' + str(decoded_control[0]) + ' not available'
                    result = {'resultCode': RESULT_UNAVAILABLE_CRITICAL_EXTENSION,
                              'matchedDN': '',
                              'diagnosticMessage': to_unicode(message, SERVER_ENCODING),
                              'referral': None
                              }
                    return [], result
                elif decoded_control[0] == '1.2.840.113556.1.4.319':  # Simple paged search
                    if not decoded_control[1]['value']['cookie']:  # new paged search
                        response, result =  self._execute_search(request)
                        if result['resultCode'] == RESULT_SUCCESS:  # success
                            paged_set = PagedSearchSet(response, int(decoded_control[1]['value']['size']), decoded_control[1]['criticality'])
                            response, result = paged_set.next()
                            if paged_set.done:  # paged search already completed, no need to store the set
                                del paged_set
                            else:
                                self._paged_sets.append(paged_set)
                            return response, result
                        else:
                            return [], result
                    else:
                        for paged_set in self._paged_sets:
                            if paged_set.cookie == decoded_control[1]['value']['cookie']: # existing paged set
                                response, result = paged_set.next()  # returns next bunch of entries as per paged set specifications
                                if paged_set.done:
                                    self._paged_sets.remove(paged_set)
                                return response, result
                        # paged set not found
                        message = 'Invalid cookie in simple paged search'
                        result = {'resultCode': RESULT_OPERATIONS_ERROR,
                                  'matchedDN': '',
                                  'diagnosticMessage': to_unicode(message, SERVER_ENCODING),
                                  'referral': None
                                  }
                        return [], result

        else:
            return self._execute_search(request)

    def _execute_search(self, request):
        responses = []
        base = safe_dn(request['base'])
        scope = request['scope']
        attributes = request['attributes']
        if '+' in attributes:  # operational attributes requested
            attributes.extend(self.operational_attributes)
            attributes.remove('+')

        attributes = [attr.lower() for attr in request['attributes']]

        filter_root = parse_filter(request['filter'], self.connection.server.schema, auto_escape=True, auto_encode=False, validator=self.connection.server.custom_validator, check_names=self.connection.check_names)
        candidates = []
        if scope == 0:  # base object
            if base in self.connection.server.dit or base.lower() == 'cn=schema':
                candidates.append(base)
        elif scope == 1:  # single level
            for entry in self.connection.server.dit:
                if entry.lower().endswith(base.lower()) and ',' not in entry[:-len(base) - 1]:  # only leafs without commas in the remaining dn
                    candidates.append(entry)
        elif scope == 2:  # whole subtree
            for entry in self.connection.server.dit:
                if entry.lower().endswith(base.lower()):
                    candidates.append(entry)

        if not candidates:  # incorrect base
            result_code = RESULT_NO_SUCH_OBJECT
            message = 'incorrect base object'
        else:
            matched = self.evaluate_filter_node(filter_root, candidates)
            if self.connection.raise_exceptions and 0 < request['sizeLimit'] < len(matched):
                result_code = 4
                message = 'size limit exceeded'
            else:
                for match in matched:
                    responses.append({
                        'object': match,
                        'attributes': [{'type': attribute,
                                        'vals': [] if request['typesOnly'] else self.connection.server.dit[match][attribute]}
                                       for attribute in self.connection.server.dit[match]
                                       if attribute.lower() in attributes or ALL_ATTRIBUTES in attributes]
                    })
                    if '+' not in attributes:  # remove operational attributes
                        for op_attr in self.operational_attributes:
                            if op_attr.lower() in attributes:
                                # if the op_attr was explicitly requested, then keep it
                                continue
                            for i, attr in enumerate(responses[len(responses)-1]['attributes']):
                                if attr['type'] == op_attr:
                                    del responses[len(responses)-1]['attributes'][i]
                result_code = 0
                message = ''

        result = {'resultCode': result_code,
                  'matchedDN': '',
                  'diagnosticMessage': to_unicode(message, SERVER_ENCODING),
                  'referral': None
                  }

        return responses[:request['sizeLimit']] if request['sizeLimit'] > 0 else responses, result

    def mock_extended(self, request_message, controls):
        # ExtendedRequest ::= [APPLICATION 23] SEQUENCE {
        #     requestName      [0] LDAPOID,
        #     requestValue     [1] OCTET STRING OPTIONAL }
        #
        # ExtendedResponse ::= [APPLICATION 24] SEQUENCE {
        #     COMPONENTS OF LDAPResult,
        #     responseName     [10] LDAPOID OPTIONAL,
        #     responseValue    [11] OCTET STRING OPTIONAL }
        #
        # IntermediateResponse ::= [APPLICATION 25] SEQUENCE {
        #     responseName     [0] LDAPOID OPTIONAL,
        #     responseValue    [1] OCTET STRING OPTIONAL }
        request = extended_request_to_dict(request_message)

        result_code = RESULT_UNWILLING_TO_PERFORM
        message = 'not implemented'
        response_name = None
        response_value = None
        if self.connection.server.info:
            for extension in self.connection.server.info.supported_extensions:
                if request['name'] == extension[0]:  # server can answer the extended request
                    if extension[0] == '2.16.840.1.113719.1.27.100.31':  # getBindDNRequest [NOVELL]
                        result_code = 0
                        message = ''
                        response_name = OctetString('2.16.840.1.113719.1.27.100.32')  # getBindDNResponse [NOVELL]
                        response_value = OctetString(self.bound)
                    elif extension[0] == '1.3.6.1.4.1.4203.1.11.3':  # WhoAmI [RFC4532]
                        result_code = 0
                        message = ''
                        response_name = OctetString('1.3.6.1.4.1.4203.1.11.3')  # WhoAmI [RFC4532]
                        response_value = OctetString(self.bound)
                    break

        return {'resultCode': result_code,
                'matchedDN': '',
                'diagnosticMessage': to_unicode(message, SERVER_ENCODING),
                'referral': None,
                'responseName': response_name,
                'responseValue': response_value
                }

    def evaluate_filter_node(self, node, candidates):
        """After evaluation each 2 sets are added to each MATCH node, one for the matched object and one for unmatched object.
        The unmatched object set is needed if a superior node is a NOT that reverts the evaluation. The BOOLEAN nodes mix the sets
        returned by the MATCH nodes"""
        node.matched = set()
        node.unmatched = set()

        if node.elements:
            for element in node.elements:
                self.evaluate_filter_node(element, candidates)

        if node.tag == ROOT:
            return node.elements[0].matched
        elif node.tag == AND:
            first_element = node.elements[0]
            node.matched.update(first_element.matched)
            node.unmatched.update(first_element.unmatched)

            for element in node.elements[1:]:
                node.matched.intersection_update(element.matched)
                node.unmatched.intersection_update(element.unmatched)
        elif node.tag == OR:
            for element in node.elements:
                node.matched.update(element.matched)
                node.unmatched.update(element.unmatched)
        elif node.tag == NOT:
            node.matched = node.elements[0].unmatched
            node.unmatched = node.elements[0].matched
        elif node.tag == MATCH_GREATER_OR_EQUAL:
            attr_name = node.assertion['attr']
            attr_value = node.assertion['value']
            for candidate in candidates:
                if attr_name in self.connection.server.dit[candidate]:
                    for value in self.connection.server.dit[candidate][attr_name]:
                        if value.isdigit() and attr_value.isdigit():  # int comparison
                            if int(value) >= int(attr_value):
                                node.matched.add(candidate)
                            else:
                                node.unmatched.add(candidate)
                        else:
                            if to_unicode(value, SERVER_ENCODING).lower() >= to_unicode(attr_value, SERVER_ENCODING).lower():  # case insensitive string comparison
                                node.matched.add(candidate)
                            else:
                                node.unmatched.add(candidate)
        elif node.tag == MATCH_LESS_OR_EQUAL:
            attr_name = node.assertion['attr']
            attr_value = node.assertion['value']
            for candidate in candidates:
                if attr_name in self.connection.server.dit[candidate]:
                    for value in self.connection.server.dit[candidate][attr_name]:
                        if value.isdigit() and attr_value.isdigit():  # int comparison
                            if int(value) <= int(attr_value):
                                node.matched.add(candidate)
                            else:
                                node.unmatched.add(candidate)
                        else:
                            if to_unicode(value, SERVER_ENCODING).lower() <= to_unicode(attr_value, SERVER_ENCODING).lower():  # case insentive string comparison
                                node.matched.add(candidate)
                            else:
                                node.unmatched.add(candidate)
        elif node.tag == MATCH_EXTENSIBLE:
            self.connection.last_error = 'Extensible match not allowed in Mock strategy'
            if log_enabled(ERROR):
                log(ERROR, '<%s> for <%s>', self.connection.last_error, self.connection)
            raise LDAPDefinitionError(self.connection.last_error)
        elif node.tag == MATCH_PRESENT:
            attr_name = node.assertion['attr']
            for candidate in candidates:
                if attr_name in self.connection.server.dit[candidate]:
                    node.matched.add(candidate)
                else:
                    node.unmatched.add(candidate)
        elif node.tag == MATCH_SUBSTRING:
            attr_name = node.assertion['attr']
            # rebuild the original substring filter
            if 'initial' in node.assertion and node.assertion['initial'] is not None:
                substring_filter = re.escape(to_unicode(node.assertion['initial'], SERVER_ENCODING))
            else:
                substring_filter = ''

            if 'any' in node.assertion and node.assertion['any'] is not None:
                for middle in node.assertion['any']:
                    substring_filter += '.*' + re.escape(to_unicode(middle, SERVER_ENCODING))

            if 'final' in node.assertion and node.assertion['final'] is not None:
                substring_filter += '.*' + re.escape(to_unicode(node.assertion['final'], SERVER_ENCODING))

            if substring_filter and not node.assertion.get('any', None) and not node.assertion.get('final', None):  # only initial, adds .*
                substring_filter += '.*'

            regex_filter = re.compile(substring_filter, flags=re.UNICODE | re.IGNORECASE)  # unicode AND ignorecase
            for candidate in candidates:
                if attr_name in self.connection.server.dit[candidate]:
                    for value in self.connection.server.dit[candidate][attr_name]:
                        if regex_filter.match(to_unicode(value, SERVER_ENCODING)):
                            node.matched.add(candidate)
                        else:
                            node.unmatched.add(candidate)
                else:
                    node.unmatched.add(candidate)
        elif node.tag == MATCH_EQUAL or node.tag == MATCH_APPROX:
            attr_name = node.assertion['attr']
            attr_value = node.assertion['value']
            for candidate in candidates:
                if attr_name in self.connection.server.dit[candidate] and self.equal(candidate, attr_name, attr_value):
                    node.matched.add(candidate)
                else:
                    node.unmatched.add(candidate)

    def equal(self, dn, attribute_type, value_to_check):
        # value is the value to match
        attribute_values = self.connection.server.dit[dn][attribute_type]
        if not isinstance(attribute_values, SEQUENCE_TYPES):
            attribute_values = [attribute_values]
        escaped_value_to_check = ldap_escape_to_bytes(value_to_check)
        for attribute_value in attribute_values:
            if self._check_equality(escaped_value_to_check, attribute_value):
                return True
            if self._check_equality(self._prepare_value(attribute_type, value_to_check), attribute_value):
                return True
        return False

    @staticmethod
    def _check_equality(value1, value2):
        if value1 == value2:  # exact matching
            return True
        if str(value1).isdigit() and str(value2).isdigit():
            if int(value1) == int(value2):  # int comparison
                return True
        try:
            if to_unicode(value1, SERVER_ENCODING).lower() == to_unicode(value2, SERVER_ENCODING).lower():  # case insensitive comparison
                return True
        except UnicodeError:
            pass

        return False

    def send(self, message_type, request, controls=None):
        self.connection.request = self.decode_request(message_type, request, controls)
        if self.connection.listening:
            message_id = self.connection.server.next_message_id()
            if self.connection.usage:  # ldap message is built for updating metrics only
                ldap_message = LDAPMessage()
                ldap_message['messageID'] = MessageID(message_id)
                ldap_message['protocolOp'] = ProtocolOp().setComponentByName(message_type, request)
                message_controls = build_controls_list(controls)
                if message_controls is not None:
                    ldap_message['controls'] = message_controls
                asn1_request = BaseStrategy.decode_request(message_type, request, controls)
                self.connection._usage.update_transmitted_message(asn1_request, len(encode(ldap_message)))
            return message_id, message_type, request, controls
        else:
            self.connection.last_error = 'unable to send message, connection is not open'
            if log_enabled(ERROR):
                log(ERROR, '<%s> for <%s>', self.connection.last_error, self.connection)
            raise LDAPSocketOpenError(self.connection.last_error)

