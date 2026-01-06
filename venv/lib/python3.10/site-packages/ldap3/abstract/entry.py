"""
"""

# Created on 2016.08.19
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
try:
    from collections import OrderedDict
except ImportError:
    from ..utils.ordDict import OrderedDict  # for Python 2.6

from os import linesep
from copy import deepcopy

from .. import STRING_TYPES, SEQUENCE_TYPES, MODIFY_ADD, MODIFY_REPLACE
from .attribute import WritableAttribute
from .objectDef import ObjectDef
from .attrDef import AttrDef
from ..core.exceptions import LDAPKeyError, LDAPCursorError, LDAPCursorAttributeError
from ..utils.conv import check_json_dict, format_json, prepare_for_stream
from ..protocol.rfc2849 import operation_to_ldif, add_ldif_header
from ..utils.dn import safe_dn, safe_rdn, to_dn
from ..utils.repr import to_stdout_encoding
from ..utils.ciDict import CaseInsensitiveWithAliasDict
from ..utils.config import get_config_parameter
from . import STATUS_VIRTUAL, STATUS_WRITABLE, STATUS_PENDING_CHANGES, STATUS_COMMITTED, STATUS_DELETED,\
    STATUS_INIT, STATUS_READY_FOR_DELETION, STATUS_READY_FOR_MOVING, STATUS_READY_FOR_RENAMING, STATUS_MANDATORY_MISSING, STATUSES, INITIAL_STATUSES
from ..core.results import RESULT_SUCCESS
from ..utils.log import log, log_enabled, ERROR, BASIC, PROTOCOL, EXTENDED


class EntryState(object):
    """Contains data on the status of the entry. Does not pollute the Entry __dict__.

    """

    def __init__(self, dn, cursor):
        self.dn = dn
        self._initial_status = None
        self._to = None  # used for move and rename
        self.status = STATUS_INIT
        self.attributes = CaseInsensitiveWithAliasDict()
        self.raw_attributes = CaseInsensitiveWithAliasDict()
        self.response = None
        self.cursor = cursor
        self.origin = None  # reference to the original read-only entry (set when made writable). Needed to update attributes in read-only when modified (only if both refer the same server)
        self.read_time = None
        self.changes = OrderedDict()  # includes changes to commit in a writable entry
        if cursor.definition:
            self.definition = cursor.definition
        else:
            self.definition = None

    def __repr__(self):
        if self.__dict__ and self.dn is not None:
            r = 'DN: ' + to_stdout_encoding(self.dn) + ' - STATUS: ' + ((self._initial_status + ', ') if self._initial_status != self.status else '') + self.status + ' - READ TIME: ' + (self.read_time.isoformat() if self.read_time else '<never>') + linesep
            r += 'attributes: ' + ', '.join(sorted(self.attributes.keys())) + linesep
            r += 'object def: ' + (', '.join(sorted(self.definition._object_class)) if self.definition._object_class else '<None>') + linesep
            r += 'attr defs: ' + ', '.join(sorted(self.definition._attributes.keys())) + linesep
            r += 'response: ' + ('present' if self.response else '<None>') + linesep
            r += 'cursor: ' + (self.cursor.__class__.__name__ if self.cursor else '<None>') + linesep
            return r
        else:
            return object.__repr__(self)

    def __str__(self):
        return self.__repr__()

    def __getstate__(self):
        cpy = dict(self.__dict__)
        cpy['cursor'] = None
        return cpy

    def set_status(self, status):
        conf_ignored_mandatory_attributes_in_object_def = [v.lower() for v in get_config_parameter('IGNORED_MANDATORY_ATTRIBUTES_IN_OBJECT_DEF')]
        if status not in STATUSES:
            error_message = 'invalid entry status ' + str(status)
            if log_enabled(ERROR):
                log(ERROR, '%s for <%s>', error_message, self)
            raise LDAPCursorError(error_message)
        if status in INITIAL_STATUSES:
            self._initial_status = status
        self.status = status
        if status == STATUS_DELETED:
            self._initial_status = STATUS_VIRTUAL
        if status == STATUS_COMMITTED:
            self._initial_status = STATUS_WRITABLE
        if self.status == STATUS_VIRTUAL or (self.status == STATUS_PENDING_CHANGES and self._initial_status == STATUS_VIRTUAL):  # checks if all mandatory attributes are present in new entries
            for attr in self.definition._attributes:
                if self.definition._attributes[attr].mandatory and attr.lower() not in conf_ignored_mandatory_attributes_in_object_def:
                    if (attr not in self.attributes or self.attributes[attr].virtual) and attr not in self.changes:
                        self.status = STATUS_MANDATORY_MISSING
                        break

    @property
    def entry_raw_attributes(self):
        return self.raw_attributes


class EntryBase(object):
    """The Entry object contains a single LDAP entry.
    Attributes can be accessed either by sequence, by assignment
    or as dictionary keys. Keys are not case sensitive.

    The Entry object is read only

    - The DN is retrieved by entry_dn
    - The cursor reference is in _cursor
    - Raw attributes values are retrieved with _raw_attributes and the _raw_attribute() methods
    """

    def __init__(self, dn, cursor):
        self._state = EntryState(dn, cursor)

    def __repr__(self):
        if self.__dict__ and self.entry_dn is not None:
            r = 'DN: ' + to_stdout_encoding(self.entry_dn) + ' - STATUS: ' + ((self._state._initial_status + ', ') if self._state._initial_status != self.entry_status else '') + self.entry_status + ' - READ TIME: ' + (self.entry_read_time.isoformat() if self.entry_read_time else '<never>') + linesep
            if self._state.attributes:
                for attr in sorted(self._state.attributes):
                    if self._state.attributes[attr] or (hasattr(self._state.attributes[attr], 'changes') and self._state.attributes[attr].changes):
                        r += '    ' + repr(self._state.attributes[attr]) + linesep
            return r
        else:
            return object.__repr__(self)

    def __str__(self):
        return self.__repr__()

    def __iter__(self):
        for attribute in self._state.attributes:
            yield self._state.attributes[attribute]
        # raise StopIteration  # deprecated in PEP 479
        return

    def __contains__(self, item):
        try:
            self.__getitem__(item)
            return True
        except LDAPKeyError:
            return False

    def __getattr__(self, item):
        if isinstance(item, STRING_TYPES):
            if item == '_state':
                return object.__getattr__(self, item)
            item = ''.join(item.split()).lower()
            attr_found = None
            for attr in self._state.attributes.keys():
                if item == attr.lower():
                    attr_found = attr
                    break
            if not attr_found:
                for attr in self._state.attributes.aliases():
                    if item == attr.lower():
                        attr_found = attr
                        break
            if not attr_found:
                for attr in self._state.attributes.keys():
                    if item + ';binary' == attr.lower():
                        attr_found = attr
                        break
            if not attr_found:
                for attr in self._state.attributes.aliases():
                    if item + ';binary' == attr.lower():
                        attr_found = attr
                        break
            if not attr_found:
                for attr in self._state.attributes.keys():
                    if item + ';range' in attr.lower():
                        attr_found = attr
                        break
            if not attr_found:
                for attr in self._state.attributes.aliases():
                    if item + ';range' in attr.lower():
                        attr_found = attr
                        break
            if not attr_found:
                error_message = 'attribute \'%s\' not found' % item
                if log_enabled(ERROR):
                    log(ERROR, '%s for <%s>', error_message, self)
                raise LDAPCursorAttributeError(error_message)
            return self._state.attributes[attr]
        error_message = 'attribute name must be a string'
        if log_enabled(ERROR):
            log(ERROR, '%s for <%s>', error_message, self)
        raise LDAPCursorAttributeError(error_message)

    def __setattr__(self, item, value):
        if item == '_state':
            object.__setattr__(self, item, value)
        elif item in self._state.attributes:
            error_message = 'attribute \'%s\' is read only' % item
            if log_enabled(ERROR):
                log(ERROR, '%s for <%s>', error_message, self)
            raise LDAPCursorAttributeError(error_message)
        else:
            error_message = 'entry is read only, cannot add \'%s\'' % item
            if log_enabled(ERROR):
                log(ERROR, '%s for <%s>', error_message, self)
            raise LDAPCursorAttributeError(error_message)

    def __getitem__(self, item):
        if isinstance(item, STRING_TYPES):
            item = ''.join(item.split()).lower()
            attr_found = None
            for attr in self._state.attributes.keys():
                if item == attr.lower():
                    attr_found = attr
                    break
            if not attr_found:
                for attr in self._state.attributes.aliases():
                    if item == attr.lower():
                        attr_found = attr
                        break
            if not attr_found:
                for attr in self._state.attributes.keys():
                    if item + ';binary' == attr.lower():
                        attr_found = attr
                        break
            if not attr_found:
                for attr in self._state.attributes.aliases():
                    if item + ';binary' == attr.lower():
                        attr_found = attr
                        break
            if not attr_found:
                error_message = 'key \'%s\' not found' % item
                if log_enabled(ERROR):
                    log(ERROR, '%s for <%s>', error_message, self)
                raise LDAPKeyError(error_message)
            return self._state.attributes[attr]

        error_message = 'key must be a string'
        if log_enabled(ERROR):
            log(ERROR, '%s for <%s>', error_message, self)
        raise LDAPKeyError(error_message)

    def __eq__(self, other):
        if isinstance(other, EntryBase):
            return self.entry_dn == other.entry_dn

        return False

    def __lt__(self, other):
        if isinstance(other, EntryBase):
            return self.entry_dn <= other.entry_dn

        return False

    @property
    def entry_dn(self):
        return self._state.dn

    @property
    def entry_cursor(self):
        return self._state.cursor

    @property
    def entry_status(self):
        return self._state.status

    @property
    def entry_definition(self):
        return self._state.definition

    @property
    def entry_raw_attributes(self):
        return self._state.raw_attributes

    def entry_raw_attribute(self, name):
        """

        :param name: name of the attribute
        :return: raw (unencoded) value of the attribute, None if attribute is not found
        """
        return self._state.raw_attributes[name] if name in self._state.raw_attributes else None

    @property
    def entry_mandatory_attributes(self):
        return [attribute for attribute in self.entry_definition._attributes if self.entry_definition._attributes[attribute].mandatory]

    @property
    def entry_attributes(self):
        return list(self._state.attributes.keys())

    @property
    def entry_attributes_as_dict(self):
        return dict((attribute_key, deepcopy(attribute_value.values)) for (attribute_key, attribute_value) in self._state.attributes.items())

    @property
    def entry_read_time(self):
        return self._state.read_time

    @property
    def _changes(self):
        return self._state.changes

    def entry_to_json(self, raw=False, indent=4, sort=True, stream=None, checked_attributes=True, include_empty=True):
        json_entry = dict()
        json_entry['dn'] = self.entry_dn
        if checked_attributes:
            if not include_empty:
                # needed for python 2.6 compatibility
                json_entry['attributes'] = dict((key, self.entry_attributes_as_dict[key]) for key in self.entry_attributes_as_dict if self.entry_attributes_as_dict[key])
            else:
                json_entry['attributes'] = self.entry_attributes_as_dict
        if raw:
            if not include_empty:
                # needed for python 2.6 compatibility
                json_entry['raw'] = dict((key, self.entry_raw_attributes[key]) for key in self.entry_raw_attributes if self.entry_raw_attributes[key])
            else:
                json_entry['raw'] = dict(self.entry_raw_attributes)

        if str is bytes:  # Python 2
            check_json_dict(json_entry)

        json_output = json.dumps(json_entry,
                                 ensure_ascii=True,
                                 sort_keys=sort,
                                 indent=indent,
                                 check_circular=True,
                                 default=format_json,
                                 separators=(',', ': '))

        if stream:
            stream.write(json_output)

        return json_output

    def entry_to_ldif(self, all_base64=False, line_separator=None, sort_order=None, stream=None):
        ldif_lines = operation_to_ldif('searchResponse', [self._state.response], all_base64, sort_order=sort_order)
        ldif_lines = add_ldif_header(ldif_lines)
        line_separator = line_separator or linesep
        ldif_output = line_separator.join(ldif_lines)
        if stream:
            if stream.tell() == 0:
                header = add_ldif_header(['-'])[0]
                stream.write(prepare_for_stream(header + line_separator + line_separator))
            stream.write(prepare_for_stream(ldif_output + line_separator + line_separator))
        return ldif_output


class Entry(EntryBase):
    """The Entry object contains a single LDAP entry.
    Attributes can be accessed either by sequence, by assignment
    or as dictionary keys. Keys are not case sensitive.

    The Entry object is read only

    - The DN is retrieved by entry_dn
    - The Reader reference is in _cursor()
    - Raw attributes values are retrieved by the _ra_attributes and
      _raw_attribute() methods

    """
    def entry_writable(self, object_def=None, writer_cursor=None, attributes=None, custom_validator=None, auxiliary_class=None):
        conf_operational_attribute_prefix = get_config_parameter('ABSTRACTION_OPERATIONAL_ATTRIBUTE_PREFIX')
        if not self.entry_cursor.schema:
            error_message = 'schema must be available to make an entry writable'
            if log_enabled(ERROR):
                log(ERROR, '%s for <%s>', error_message, self)
            raise LDAPCursorError(error_message)
        # returns a new WritableEntry and its Writer cursor
        if object_def is None:
            if self.entry_cursor.definition._object_class:
                object_def = self.entry_definition._object_class
                auxiliary_class = self.entry_definition._auxiliary_class + (auxiliary_class if isinstance(auxiliary_class, SEQUENCE_TYPES) else [])
            elif 'objectclass' in self:
                object_def = self.objectclass.values

        if not object_def:
            error_message = 'object class must be specified to make an entry writable'
            if log_enabled(ERROR):
                log(ERROR, '%s for <%s>', error_message, self)
            raise LDAPCursorError(error_message)

        if not isinstance(object_def, ObjectDef):
                object_def = ObjectDef(object_def, self.entry_cursor.schema, custom_validator, auxiliary_class)

        if attributes:
            if isinstance(attributes, STRING_TYPES):
                attributes = [attributes]

            if isinstance(attributes, SEQUENCE_TYPES):
                for attribute in attributes:
                    if attribute not in object_def._attributes:
                        error_message = 'attribute \'%s\' not in schema for \'%s\'' % (attribute, object_def)
                        if log_enabled(ERROR):
                            log(ERROR, '%s for <%s>', error_message, self)
                        raise LDAPCursorError(error_message)
        else:
            attributes = []

        if not writer_cursor:
            from .cursor import Writer  # local import to avoid circular reference in import at startup
            writable_cursor = Writer(self.entry_cursor.connection, object_def)
        else:
            writable_cursor = writer_cursor

        if attributes:  # force reading of attributes
            writable_entry = writable_cursor._refresh_object(self.entry_dn, list(attributes) + self.entry_attributes)
        else:
            writable_entry = writable_cursor._create_entry(self._state.response)
            writable_cursor.entries.append(writable_entry)
            writable_entry._state.read_time = self.entry_read_time
        writable_entry._state.origin = self  # reference to the original read-only entry
        # checks original entry for custom definitions in AttrDefs
        attr_to_add = []
        attr_to_remove = []
        object_def_to_add = []
        object_def_to_remove = []
        for attr in writable_entry._state.origin.entry_definition._attributes:
            original_attr = writable_entry._state.origin.entry_definition._attributes[attr]
            if attr != original_attr.name and (attr not in writable_entry._state.attributes or conf_operational_attribute_prefix + original_attr.name not in writable_entry._state.attributes):
                old_attr_def = writable_entry.entry_definition._attributes[original_attr.name]
                new_attr_def = AttrDef(original_attr.name,
                                       key=attr,
                                       validate=original_attr.validate,
                                       pre_query=original_attr.pre_query,
                                       post_query=original_attr.post_query,
                                       default=original_attr.default,
                                       dereference_dn=original_attr.dereference_dn,
                                       description=original_attr.description,
                                       mandatory=old_attr_def.mandatory,  # keeps value read from schema
                                       single_value=old_attr_def.single_value,  # keeps value read from schema
                                       alias=original_attr.other_names)
                od = writable_entry.entry_definition
                object_def_to_remove.append(old_attr_def)
                object_def_to_add.append(new_attr_def)
                # updates attribute name in entry attributes
                new_attr = WritableAttribute(new_attr_def, writable_entry, writable_cursor)
                if original_attr.name in writable_entry._state.attributes:
                    new_attr.other_names = writable_entry._state.attributes[original_attr.name].other_names
                    new_attr.raw_values = writable_entry._state.attributes[original_attr.name].raw_values
                    new_attr.values = writable_entry._state.attributes[original_attr.name].values
                    new_attr.response = writable_entry._state.attributes[original_attr.name].response
                attr_to_add.append((attr, new_attr))
                attr_to_remove.append(original_attr.name)
                # writable_entry._state.attributes[attr] = new_attr
                ## writable_entry._state.attributes.set_alias(attr, new_attr.other_names)
                # del writable_entry._state.attributes[original_attr.name]
        for attr, new_attr in attr_to_add:
            writable_entry._state.attributes[attr] = new_attr
        for attr in attr_to_remove:
            del writable_entry._state.attributes[attr]
        for object_def in object_def_to_remove:
            o = writable_entry.entry_definition
            o -= object_def
        for object_def in object_def_to_add:
            o = writable_entry.entry_definition
            o += object_def

        writable_entry._state.set_status(STATUS_WRITABLE)
        return writable_entry


class WritableEntry(EntryBase):
    def __setitem__(self, key, value):
        if value is not Ellipsis:  # hack for using implicit operators in writable attributes
            self.__setattr__(key, value)

    def __setattr__(self, item, value):
        conf_attributes_excluded_from_object_def = [v.lower() for v in get_config_parameter('ATTRIBUTES_EXCLUDED_FROM_OBJECT_DEF')]
        if item == '_state' and isinstance(value, EntryState):
            self.__dict__['_state'] = value
            return

        if value is not Ellipsis:  # hack for using implicit operators in writable attributes
            # checks if using an alias
            if item in self.entry_cursor.definition._attributes or item.lower() in conf_attributes_excluded_from_object_def:
                if item not in self._state.attributes:  # setting value to an attribute still without values
                    new_attribute = WritableAttribute(self.entry_cursor.definition._attributes[item], self, cursor=self.entry_cursor)
                    self._state.attributes[str(item)] = new_attribute  # force item to a string for key in attributes dict
                self._state.attributes[item].set(value)  # try to add to new_values
            else:
                error_message = 'attribute \'%s\' not defined' % item
                if log_enabled(ERROR):
                    log(ERROR, '%s for <%s>', error_message, self)
                raise LDAPCursorAttributeError(error_message)

    def __getattr__(self, item):
        if isinstance(item, STRING_TYPES):
            if item == '_state':
                return self.__dict__['_state']
            item = ''.join(item.split()).lower()
            for attr in self._state.attributes.keys():
                if item == attr.lower():
                    return self._state.attributes[attr]
            for attr in self._state.attributes.aliases():
                if item == attr.lower():
                    return self._state.attributes[attr]
            if item in self.entry_definition._attributes:  # item is a new attribute to commit, creates the AttrDef and add to the attributes to retrive
                self._state.attributes[item] = WritableAttribute(self.entry_definition._attributes[item], self, self.entry_cursor)
                self.entry_cursor.attributes.add(item)
                return self._state.attributes[item]
            error_message = 'attribute \'%s\' not defined' % item
            if log_enabled(ERROR):
                log(ERROR, '%s for <%s>', error_message, self)
            raise LDAPCursorAttributeError(error_message)
        else:
            error_message = 'attribute name must be a string'
            if log_enabled(ERROR):
                log(ERROR, '%s for <%s>', error_message, self)
            raise LDAPCursorAttributeError(error_message)

    @property
    def entry_virtual_attributes(self):
        return [attr for attr in self.entry_attributes if self[attr].virtual]

    def entry_commit_changes(self, refresh=True, controls=None, clear_history=True):
        if clear_history:
            self.entry_cursor._reset_history()

        if self.entry_status == STATUS_READY_FOR_DELETION:
            result = self.entry_cursor.connection.delete(self.entry_dn, controls)
            if not self.entry_cursor.connection.strategy.sync:
                response, result, request = self.entry_cursor.connection.get_response(result, get_request=True)
            else:
                if self.entry_cursor.connection.strategy.thread_safe:
                    _, result, response, request = result
                else:
                    response = self.entry_cursor.connection.response
                    result = self.entry_cursor.connection.result
                    request = self.entry_cursor.connection.request
            self.entry_cursor._store_operation_in_history(request, result, response)
            if result['result'] == RESULT_SUCCESS:
                dn = self.entry_dn
                if self._state.origin and self.entry_cursor.connection.server == self._state.origin.entry_cursor.connection.server:  # deletes original read-only Entry
                    cursor = self._state.origin.entry_cursor
                    self._state.origin.__dict__.clear()
                    self._state.origin.__dict__['_state'] = EntryState(dn, cursor)
                    self._state.origin._state.set_status(STATUS_DELETED)
                cursor = self.entry_cursor
                self.__dict__.clear()
                self._state = EntryState(dn, cursor)
                self._state.set_status(STATUS_DELETED)
                return True
            return False
        elif self.entry_status == STATUS_READY_FOR_MOVING:
            result = self.entry_cursor.connection.modify_dn(self.entry_dn, '+'.join(safe_rdn(self.entry_dn)), new_superior=self._state._to)
            if not self.entry_cursor.connection.strategy.sync:
                response, result, request = self.entry_cursor.connection.get_response(result, get_request=True)
            else:
                if self.entry_cursor.connection.strategy.thread_safe:
                    _, result, response, request = result
                else:
                    response = self.entry_cursor.connection.response
                    result = self.entry_cursor.connection.result
                    request = self.entry_cursor.connection.request
            self.entry_cursor._store_operation_in_history(request, result, response)
            if result['result'] == RESULT_SUCCESS:
                self._state.dn = safe_dn('+'.join(safe_rdn(self.entry_dn)) + ',' + self._state._to)
                if refresh:
                    if self.entry_refresh():
                        if self._state.origin and self.entry_cursor.connection.server == self._state.origin.entry_cursor.connection.server:  # refresh dn of origin
                            self._state.origin._state.dn = self.entry_dn
                self._state.set_status(STATUS_COMMITTED)
                self._state._to = None
                return True
            return False
        elif self.entry_status == STATUS_READY_FOR_RENAMING:
            rdn = '+'.join(safe_rdn(self._state._to))
            result = self.entry_cursor.connection.modify_dn(self.entry_dn, rdn)
            if not self.entry_cursor.connection.strategy.sync:
                response, result, request = self.entry_cursor.connection.get_response(result, get_request=True)
            else:
                if self.entry_cursor.connection.strategy.thread_safe:
                    _, result, response, request = result
                else:
                    response = self.entry_cursor.connection.response
                    result = self.entry_cursor.connection.result
                    request = self.entry_cursor.connection.request
            self.entry_cursor._store_operation_in_history(request, result, response)
            if result['result'] == RESULT_SUCCESS:
                self._state.dn = rdn + ',' + ','.join(to_dn(self.entry_dn)[1:])
                if refresh:
                    if self.entry_refresh():
                        if self._state.origin and self.entry_cursor.connection.server == self._state.origin.entry_cursor.connection.server:  # refresh dn of origin
                            self._state.origin._state.dn = self.entry_dn
                self._state.set_status(STATUS_COMMITTED)
                self._state._to = None
                return True
            return False
        elif self.entry_status in [STATUS_VIRTUAL, STATUS_MANDATORY_MISSING]:
            missing_attributes = []
            for attr in self.entry_mandatory_attributes:
                if (attr not in self._state.attributes or self._state.attributes[attr].virtual) and attr not in self._changes:
                    missing_attributes.append('\'' + attr + '\'')
            error_message = 'mandatory attributes %s missing in entry %s' % (', '.join(missing_attributes), self.entry_dn)
            if log_enabled(ERROR):
                log(ERROR, '%s for <%s>', error_message, self)
            raise LDAPCursorError(error_message)
        elif self.entry_status == STATUS_PENDING_CHANGES:
            if self._changes:
                if self.entry_definition._auxiliary_class:  # checks if an attribute is from an auxiliary class and adds it to the objectClass attribute if not present
                    for attr in self._changes:
                        # checks schema to see if attribute is defined in one of the already present object classes
                        attr_classes = self.entry_cursor.schema.attribute_types[attr].mandatory_in + self.entry_cursor.schema.attribute_types[attr].optional_in
                        for object_class in self.objectclass:
                            if object_class in attr_classes:
                                break
                        else:  # executed only if the attribute class is not present in the objectClass attribute
                            # checks if attribute is defined in one of the possible auxiliary classes
                            for aux_class in self.entry_definition._auxiliary_class:
                                if aux_class in attr_classes:
                                    if self._state._initial_status == STATUS_VIRTUAL:  # entry is new, there must be a pending objectClass MODIFY_REPLACE
                                        self._changes['objectClass'][0][1].append(aux_class)
                                    else:
                                        self.objectclass += aux_class
                if self._state._initial_status == STATUS_VIRTUAL:
                    new_attributes = dict()
                    for attr in self._changes:
                        new_attributes[attr] = self._changes[attr][0][1]
                    result = self.entry_cursor.connection.add(self.entry_dn, None, new_attributes, controls)
                else:
                    result = self.entry_cursor.connection.modify(self.entry_dn, self._changes, controls)

                if not self.entry_cursor.connection.strategy.sync:  # asynchronous request
                    response, result, request = self.entry_cursor.connection.get_response(result, get_request=True)
                else:
                    if self.entry_cursor.connection.strategy.thread_safe:
                        _, result, response, request = result
                    else:
                        response = self.entry_cursor.connection.response
                        result = self.entry_cursor.connection.result
                        request = self.entry_cursor.connection.request
                self.entry_cursor._store_operation_in_history(request, result, response)

                if result['result'] == RESULT_SUCCESS:
                    if refresh:
                        if self.entry_refresh():
                            if self._state.origin and self.entry_cursor.connection.server == self._state.origin.entry_cursor.connection.server:  # updates original read-only entry if present
                                for attr in self:  # adds AttrDefs from writable entry to origin entry definition if some is missing
                                    if attr.key in self.entry_definition._attributes and attr.key not in self._state.origin.entry_definition._attributes:
                                        self._state.origin.entry_cursor.definition.add_attribute(self.entry_cursor.definition._attributes[attr.key])  # adds AttrDef from writable entry to original entry if missing
                                temp_entry = self._state.origin.entry_cursor._create_entry(self._state.response)
                                self._state.origin.__dict__.clear()
                                self._state.origin.__dict__['_state'] = temp_entry._state
                                for attr in self:  # returns the whole attribute object
                                    if not hasattr(attr,'virtual'):
                                        self._state.origin.__dict__[attr.key] = self._state.origin._state.attributes[attr.key]
                                self._state.origin._state.read_time = self.entry_read_time
                    else:
                        self.entry_discard_changes()  # if not refreshed remove committed changes
                    self._state.set_status(STATUS_COMMITTED)
                    return True
        return False

    def entry_discard_changes(self):
        self._changes.clear()
        self._state.set_status(self._state._initial_status)

    def entry_delete(self):
        if self.entry_status not in [STATUS_WRITABLE, STATUS_COMMITTED, STATUS_READY_FOR_DELETION]:
            error_message = 'cannot delete entry, invalid status: ' + self.entry_status
            if log_enabled(ERROR):
                log(ERROR, '%s for <%s>', error_message, self)
            raise LDAPCursorError(error_message)
        self._state.set_status(STATUS_READY_FOR_DELETION)

    def entry_refresh(self, tries=4, seconds=2):
        """

        Refreshes the entry from the LDAP Server
        """
        if self.entry_cursor.connection:
            if self.entry_cursor.refresh_entry(self, tries, seconds):
                return True

        return False

    def entry_move(self, destination_dn):
        if self.entry_status not in [STATUS_WRITABLE, STATUS_COMMITTED, STATUS_READY_FOR_MOVING]:
            error_message = 'cannot move entry, invalid status: ' + self.entry_status
            if log_enabled(ERROR):
                log(ERROR, '%s for <%s>', error_message, self)
            raise LDAPCursorError(error_message)
        self._state._to = safe_dn(destination_dn)
        self._state.set_status(STATUS_READY_FOR_MOVING)

    def entry_rename(self, new_name):
        if self.entry_status not in [STATUS_WRITABLE, STATUS_COMMITTED, STATUS_READY_FOR_RENAMING]:
            error_message = 'cannot rename entry, invalid status: ' + self.entry_status
            if log_enabled(ERROR):
                log(ERROR, '%s for <%s>', error_message, self)
            raise LDAPCursorError(error_message)
        self._state._to = new_name
        self._state.set_status(STATUS_READY_FOR_RENAMING)

    @property
    def entry_changes(self):
        return self._changes
