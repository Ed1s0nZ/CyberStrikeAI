"""
"""

# Created on 2013.12.08
#
# Author: Giovanni Cannata
#
# Copyright 2013 - 2020 Giovanni Cannata
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

from base64 import b64encode
from datetime import datetime

from .. import STRING_TYPES
from ..core.exceptions import LDAPLDIFError, LDAPExtensionError
from ..protocol.persistentSearch import EntryChangeNotificationControl
from ..utils.asn1 import decoder
from ..utils.config import get_config_parameter

# LDIF converter RFC 2849 compliant

conf_ldif_line_length = get_config_parameter('LDIF_LINE_LENGTH')


def safe_ldif_string(bytes_value):
    if not bytes_value:
        return True

    # check SAFE-INIT-CHAR: < 127, not NUL, LF, CR, SPACE, COLON, LESS-THAN
    if bytes_value[0] > 127 or bytes_value[0] in [0, 10, 13, 32, 58, 60]:
        return False

    # check SAFE-CHAR: < 127 not NUL, LF, CR
    if 0 in bytes_value or 10 in bytes_value or 13 in bytes_value:
        return False

    # check last char for SPACE
    if bytes_value[-1] == 32:
        return False

    for byte in bytes_value:
        if byte > 127:
            return False

    return True


def _convert_to_ldif(descriptor, value, base64):
    if not value:
        value = ''
    if isinstance(value, STRING_TYPES):
        value = bytearray(value, encoding='utf-8')

    if base64 or not safe_ldif_string(value):
        try:
            encoded = b64encode(value)
        except TypeError:
            encoded = b64encode(str(value))  # patch for Python 2.6
        if not isinstance(encoded, str):  # in Python 3 b64encode returns bytes in Python 2 returns str
            encoded = str(encoded, encoding='ascii')  # Python 3
        line = descriptor + ':: ' + encoded
    else:
        if str is not bytes:  # Python 3
            value = str(value, encoding='ascii')
        else:  # Python 2
            value = str(value)
        line = descriptor + ': ' + value

    return line


def add_controls(controls, all_base64):
    lines = []
    if controls:
        for control in controls:
            line = 'control: ' + control[0]
            line += ' ' + ('true' if control[1] else 'false')
            if control[2]:
                lines.append(_convert_to_ldif(line, control[2], all_base64))

    return lines


def add_attributes(attributes, all_base64):
    lines = []
    oc_attr = None
    # objectclass first, even if this is not specified in the RFC
    for attr in attributes:
        if attr.lower() == 'objectclass':
            for val in attributes[attr]:
                lines.append(_convert_to_ldif(attr, val, all_base64))
            oc_attr = attr
            break

    # remaining attributes
    for attr in attributes:
        if attr != oc_attr and attr in attributes:
            for val in attributes[attr]:
                lines.append(_convert_to_ldif(attr, val, all_base64))

    return lines


def sort_ldif_lines(lines, sort_order):
    # sort lines as per custom sort_order
    # sort order is a list of descriptors, lines will be sorted following the same sequence
    return sorted(lines, key=lambda x: ldif_sort(x, sort_order)) if sort_order else lines


def search_response_to_ldif(entries, all_base64, sort_order=None):
    lines = []
    if entries:
        for entry in entries:
            if not entry or entry['type'] != 'searchResEntry':
                continue
            if 'dn' in entry:
                lines.append(_convert_to_ldif('dn', entry['dn'], all_base64))
                lines.extend(add_attributes(entry['raw_attributes'], all_base64))
            else:
                raise LDAPLDIFError('unable to convert to LDIF-CONTENT - missing DN')
            if sort_order:
                lines = sort_ldif_lines(lines, sort_order)
            lines.append('')

        if lines:
            lines.append('# total number of entries: ' + str(len(entries)))

    return lines


def add_request_to_ldif(entry, all_base64, sort_order=None):
    lines = []
    if 'entry' in entry:
        lines.append(_convert_to_ldif('dn', entry['entry'], all_base64))
        control_lines = add_controls(entry['controls'], all_base64)
        if control_lines:
            lines.extend(control_lines)
        lines.append('changetype: add')
        lines.extend(add_attributes(entry['attributes'], all_base64))
        if sort_order:
            lines = sort_ldif_lines(lines, sort_order)

    else:
        raise LDAPLDIFError('unable to convert to LDIF-CHANGE-ADD - missing DN ')

    return lines


def delete_request_to_ldif(entry, all_base64, sort_order=None):
    lines = []
    if 'entry' in entry:
        lines.append(_convert_to_ldif('dn', entry['entry'], all_base64))
        control_lines = add_controls(entry['controls'], all_base64)
        if control_lines:
            lines.extend(control_lines)
        lines.append('changetype: delete')
        if sort_order:
            lines = sort_ldif_lines(lines, sort_order)
    else:
        raise LDAPLDIFError('unable to convert to LDIF-CHANGE-DELETE - missing DN ')

    return lines


def modify_request_to_ldif(entry, all_base64, sort_order=None):
    lines = []
    if 'entry' in entry:
        lines.append(_convert_to_ldif('dn', entry['entry'], all_base64))
        control_lines = add_controls(entry['controls'], all_base64)
        if control_lines:
            lines.extend(control_lines)
        lines.append('changetype: modify')
        if 'changes' in entry:
            for change in entry['changes']:
                lines.append(['add', 'delete', 'replace', 'increment'][change['operation']] + ': ' + change['attribute']['type'])
                for value in change['attribute']['value']:
                    lines.append(_convert_to_ldif(change['attribute']['type'], value, all_base64))
                lines.append('-')
        if sort_order:
            lines = sort_ldif_lines(lines, sort_order)
    return lines


def modify_dn_request_to_ldif(entry, all_base64, sort_order=None):
    lines = []
    if 'entry' in entry:
        lines.append(_convert_to_ldif('dn', entry['entry'], all_base64))
        control_lines = add_controls(entry['controls'], all_base64)
        if control_lines:
            lines.extend(control_lines)
        lines.append('changetype: modrdn') if 'newSuperior' in entry and entry['newSuperior'] else lines.append('changetype: moddn')
        lines.append(_convert_to_ldif('newrdn', entry['newRdn'], all_base64))
        lines.append('deleteoldrdn: ' + ('1' if entry['deleteOldRdn'] else '0'))
        if 'newSuperior' in entry and entry['newSuperior']:
            lines.append(_convert_to_ldif('newsuperior', entry['newSuperior'], all_base64))
        if sort_order:
            lines = sort_ldif_lines(lines, sort_order)
    else:
        raise LDAPLDIFError('unable to convert to LDIF-CHANGE-MODDN - missing DN ')

    return lines


def operation_to_ldif(operation_type, entries, all_base64=False, sort_order=None):
    if operation_type == 'searchResponse':
        lines = search_response_to_ldif(entries, all_base64, sort_order)
    elif operation_type == 'addRequest':
        lines = add_request_to_ldif(entries, all_base64, sort_order)
    elif operation_type == 'delRequest':
        lines = delete_request_to_ldif(entries, all_base64, sort_order)
    elif operation_type == 'modifyRequest':
        lines = modify_request_to_ldif(entries, all_base64, sort_order)
    elif operation_type == 'modDNRequest':
        lines = modify_dn_request_to_ldif(entries, all_base64, sort_order)
    else:
        lines = []

    ldif_record = []
    # check max line length and split as per note 2 of RFC 2849
    for line in lines:
        if line:
            ldif_record.append(line[0:conf_ldif_line_length])
            ldif_record.extend([' ' + line[i: i + conf_ldif_line_length - 1] for i in range(conf_ldif_line_length, len(line), conf_ldif_line_length - 1)] if len(line) > conf_ldif_line_length else [])
        else:
            ldif_record.append('')

    return ldif_record


def add_ldif_header(ldif_lines):
    if ldif_lines:
        ldif_lines.insert(0, 'version: 1')

    return ldif_lines


def ldif_sort(line, sort_order):
    for i, descriptor in enumerate(sort_order):

        if line and line.startswith(descriptor):
            return i

    return len(sort_order) + 1


def decode_persistent_search_control(change):
    if 'controls' in change and '2.16.840.1.113730.3.4.7' in change['controls']:
        decoded = dict()
        decoded_control, unprocessed = decoder.decode(change['controls']['2.16.840.1.113730.3.4.7']['value'], asn1Spec=EntryChangeNotificationControl())
        if unprocessed:
            raise LDAPExtensionError('unprocessed value in EntryChangeNotificationControl')
        if decoded_control['changeType'] == 1:  # add
            decoded['changeType'] = 'add'
        elif decoded_control['changeType'] == 2:  # delete
            decoded['changeType'] = 'delete'
        elif decoded_control['changeType'] == 4:  # modify
            decoded['changeType'] = 'modify'
        elif decoded_control['changeType'] == 8:  # modify_dn
            decoded['changeType'] = 'modify dn'
        else:
            raise LDAPExtensionError('unknown Persistent Search changeType ' + str(decoded_control['changeType']))
        decoded['changeNumber'] = decoded_control['changeNumber'] if 'changeNumber' in decoded_control and decoded_control['changeNumber'] is not None and decoded_control['changeNumber'].hasValue() else None
        decoded['previousDN'] = decoded_control['previousDN'] if 'previousDN' in decoded_control and decoded_control['previousDN'] is not None and decoded_control['previousDN'].hasValue() else None
        return decoded

    return None


def persistent_search_response_to_ldif(change):
    ldif_lines = ['# ' + datetime.now().isoformat()]
    control = decode_persistent_search_control(change)
    if control:
        if control['changeNumber']:
            ldif_lines.append('# change number: ' + str(control['changeNumber']))
        ldif_lines.append(control['changeType'])
        if control['previousDN']:
            ldif_lines.append('# previous dn: ' + str(control['previousDN']))
    ldif_lines += operation_to_ldif('searchResponse', [change])

    return ldif_lines[:-1]  # removes "total number of entries"
