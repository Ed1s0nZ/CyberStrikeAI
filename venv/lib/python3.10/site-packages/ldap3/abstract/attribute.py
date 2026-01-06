"""
"""

# Created on 2014.01.06
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

from os import linesep

from .. import MODIFY_ADD, MODIFY_REPLACE, MODIFY_DELETE, SEQUENCE_TYPES
from ..core.exceptions import LDAPCursorError
from ..utils.repr import to_stdout_encoding
from . import STATUS_PENDING_CHANGES, STATUS_VIRTUAL, STATUS_READY_FOR_DELETION, STATUS_READY_FOR_MOVING, STATUS_READY_FOR_RENAMING
from ..utils.log import log, log_enabled, ERROR, BASIC, PROTOCOL, EXTENDED


# noinspection PyUnresolvedReferences
class Attribute(object):
    """Attribute/values object, it includes the search result (after post_query transformation) of each attribute in an entry

    Attribute object is read only

    - values: contain the processed attribute values
    - raw_values': contain the unprocessed attribute values


    """

    def __init__(self, attr_def, entry, cursor):
        self.key = attr_def.key
        self.definition = attr_def
        self.values = []
        self.raw_values = []
        self.response = None
        self.entry = entry
        self.cursor = cursor
        other_names = [name for name in attr_def.oid_info.name if self.key.lower() != name.lower()] if attr_def.oid_info else None
        self.other_names = set(other_names) if other_names else None  # self.other_names is None if there are no short names, else is a set of secondary names

    def __repr__(self):
        if len(self.values) == 1:
            r = to_stdout_encoding(self.key) + ': ' + to_stdout_encoding(self.values[0])
        elif len(self.values) > 1:
            r = to_stdout_encoding(self.key) + ': ' + to_stdout_encoding(self.values[0])
            filler = ' ' * (len(self.key) + 6)
            for value in self.values[1:]:
                r += linesep + filler + to_stdout_encoding(value)
        else:
            r = to_stdout_encoding(self.key) + ': ' + to_stdout_encoding('<no value>')

        return r

    def __str__(self):
        if len(self.values) == 1:
            return to_stdout_encoding(self.values[0])
        else:
            return to_stdout_encoding(self.values)

    def __len__(self):
        return len(self.values)

    def __iter__(self):
        return self.values.__iter__()

    def __getitem__(self, item):
        return self.values[item]

    def __getstate__(self):
        cpy = dict(self.__dict__)
        cpy['cursor'] = None
        return cpy

    def __eq__(self, other):
        try:
            if self.value == other:
                return True
        except Exception:
            return False

    def __ne__(self, other):
        return not self == other

    @property
    def value(self):
        """
        :return: The single value or a list of values of the attribute.
        """
        if not self.values:
            return None

        return self.values[0] if len(self.values) == 1 else self.values


class OperationalAttribute(Attribute):
    """Operational attribute/values object. Include the search result of an
    operational attribute in an entry

    OperationalAttribute object is read only

    - values: contains the processed attribute values
    - raw_values: contains the unprocessed attribute values

    It may not have an AttrDef

    """

    def __repr__(self):
        if len(self.values) == 1:
            r = to_stdout_encoding(self.key) + ' [OPERATIONAL]: ' + to_stdout_encoding(self.values[0])
        elif len(self.values) > 1:
            r = to_stdout_encoding(self.key) + ' [OPERATIONAL]: ' + to_stdout_encoding(self.values[0])
            filler = ' ' * (len(self.key) + 6)
            for value in sorted(self.values[1:]):
                r += linesep + filler + to_stdout_encoding(value)
        else:
            r = ''

        return r


class WritableAttribute(Attribute):
    def __repr__(self):
        filler = ' ' * (len(self.key) + 6)
        if len(self.values) == 1:
            r = to_stdout_encoding(self.key) + ': ' + to_stdout_encoding(self.values[0])
        elif len(self.values) > 1:
            r = to_stdout_encoding(self.key) + ': ' + to_stdout_encoding(self.values[0])
            for value in self.values[1:]:
                r += linesep + filler + to_stdout_encoding(value)
        else:
            r = to_stdout_encoding(self.key) + to_stdout_encoding(': <Virtual>')
        if self.definition.name in self.entry._changes:
            r += linesep + filler + 'CHANGES: ' + str(self.entry._changes[self.definition.name])
        return r

    def __iadd__(self, other):
        self.add(other)
        return Ellipsis  # hack to avoid calling set() in entry __setattr__

    def __isub__(self, other):
        self.delete(other)
        return Ellipsis  # hack to avoid calling set_value in entry __setattr__

    def _update_changes(self, changes, remove_old=False):
        # checks for friendly key in AttrDef and uses the real attribute name
        if self.definition and self.definition.name:
            key = self.definition.name
        else:
            key = self.key

        if key not in self.entry._changes or remove_old:  # remove old changes (for removing attribute)
            self.entry._changes[key] = []

        self.entry._changes[key].append(changes)
        if log_enabled(PROTOCOL):
            log(PROTOCOL, 'updated changes <%r> for <%s> attribute in <%s> entry', changes, self.key, self.entry.entry_dn)
        self.entry._state.set_status(STATUS_PENDING_CHANGES)

    def add(self, values):
        if log_enabled(PROTOCOL):
            log(PROTOCOL, 'adding %r to <%s> attribute in <%s> entry', values, self.key, self.entry.entry_dn)
        # new value for attribute to commit with a MODIFY_ADD
        if self.entry._state._initial_status == STATUS_VIRTUAL:
            error_message = 'cannot perform a modify operation in a new entry'
            if log_enabled(ERROR):
                log(ERROR, '%s for <%s>', error_message, self)
            raise LDAPCursorError(error_message)
        if self.entry.entry_status in [STATUS_READY_FOR_DELETION, STATUS_READY_FOR_MOVING, STATUS_READY_FOR_RENAMING]:
            error_message = self.entry.entry_status + ' - cannot add attributes'
            if log_enabled(ERROR):
                log(ERROR, '%s for <%s>', error_message, self)
            raise LDAPCursorError(error_message)
        if values is None:
            error_message = 'value to add cannot be None'
            if log_enabled(ERROR):
                log(ERROR, '%s for <%s>', error_message, self)
            raise LDAPCursorError(error_message)
        if values is not None:
            validated = self.definition.validate(values)  # returns True, False or a value to substitute to the actual values
            if validated is False:
                error_message = 'value \'%s\' non valid for attribute \'%s\'' % (values, self.key)
                if log_enabled(ERROR):
                    log(ERROR, '%s for <%s>', error_message, self)
                raise LDAPCursorError(error_message)
            elif validated is not True:  # a valid LDAP value equivalent to the actual values
                values = validated
        self._update_changes((MODIFY_ADD, values if isinstance(values, SEQUENCE_TYPES) else [values]))

    def set(self, values):
        # new value for attribute to commit with a MODIFY_REPLACE, old values are deleted
        if log_enabled(PROTOCOL):
            log(PROTOCOL, 'setting %r to <%s> attribute in <%s> entry', values, self.key, self.entry.entry_dn)
        if self.entry.entry_status in [STATUS_READY_FOR_DELETION, STATUS_READY_FOR_MOVING, STATUS_READY_FOR_RENAMING]:
            error_message = self.entry.entry_status + ' - cannot set attributes'
            if log_enabled(ERROR):
                log(ERROR, '%s for <%s>', error_message, self)
            raise LDAPCursorError(error_message)
        if values is None:
            error_message = 'new value cannot be None'
            if log_enabled(ERROR):
                log(ERROR, '%s for <%s>', error_message, self)
            raise LDAPCursorError(error_message)
        validated = self.definition.validate(values)  # returns True, False or a value to substitute to the actual values
        if validated is False:
            error_message = 'value \'%s\' non valid for attribute \'%s\'' % (values, self.key)
            if log_enabled(ERROR):
                log(ERROR, '%s for <%s>', error_message, self)
            raise LDAPCursorError(error_message)
        elif validated is not True:  # a valid LDAP value equivalent to the actual values
            values = validated
        self._update_changes((MODIFY_REPLACE, values if isinstance(values, SEQUENCE_TYPES) else [values]), remove_old=True)

    def delete(self, values):
        # value for attribute to delete in commit with a MODIFY_DELETE
        if log_enabled(PROTOCOL):
            log(PROTOCOL, 'deleting %r from <%s> attribute in <%s> entry', values, self.key, self.entry.entry_dn)
        if self.entry._state._initial_status == STATUS_VIRTUAL:
            error_message = 'cannot delete an attribute value in a new entry'
            if log_enabled(ERROR):
                log(ERROR, '%s for <%s>', error_message, self)
            raise LDAPCursorError(error_message)
        if self.entry.entry_status in [STATUS_READY_FOR_DELETION, STATUS_READY_FOR_MOVING, STATUS_READY_FOR_RENAMING]:
            error_message = self.entry.entry_status + ' - cannot delete attributes'
            if log_enabled(ERROR):
                log(ERROR, '%s for <%s>', error_message, self)
            raise LDAPCursorError(error_message)
        if values is None:
            error_message = 'value to delete cannot be None'
            if log_enabled(ERROR):
                log(ERROR, '%s for <%s>', error_message, self)
            raise LDAPCursorError(error_message)
        if not isinstance(values, SEQUENCE_TYPES):
            values = [values]
        for single_value in values:
            if single_value not in self.values:
                error_message = 'value \'%s\' not present in \'%s\'' % (single_value, ', '.join(self.values))
                if log_enabled(ERROR):
                    log(ERROR, '%s for <%s>', error_message, self)
                raise LDAPCursorError(error_message)
        self._update_changes((MODIFY_DELETE, values))

    def remove(self):
        if log_enabled(PROTOCOL):
            log(PROTOCOL, 'removing <%s> attribute in <%s> entry', self.key, self.entry.entry_dn)
        if self.entry._state._initial_status == STATUS_VIRTUAL:
            error_message = 'cannot remove an attribute in a new entry'
            if log_enabled(ERROR):
                log(ERROR, '%s for <%s>', error_message, self)
            raise LDAPCursorError(error_message)
        if self.entry.entry_status in [STATUS_READY_FOR_DELETION, STATUS_READY_FOR_MOVING, STATUS_READY_FOR_RENAMING]:
            error_message = self.entry.entry_status + ' - cannot remove attributes'
            if log_enabled(ERROR):
                log(ERROR, '%s for <%s>', error_message, self)
            raise LDAPCursorError(error_message)
        self._update_changes((MODIFY_REPLACE, []), True)

    def discard(self):
        if log_enabled(PROTOCOL):
            log(PROTOCOL, 'discarding <%s> attribute in <%s> entry', self.key, self.entry.entry_dn)
        del self.entry._changes[self.key]
        if not self.entry._changes:
            self.entry._state.set_status(self.entry._state._initial_status)

    @property
    def virtual(self):
        return False if len(self.values) else True

    @property
    def changes(self):
        if self.key in self.entry._changes:
            return self.entry._changes[self.key]
        return None
