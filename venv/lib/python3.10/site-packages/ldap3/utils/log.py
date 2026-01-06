"""
"""

# Created on 2015.05.01
#
# Author: Giovanni Cannata
#
# Copyright 2015 - 2020 Giovanni Cannata
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

from logging import getLogger, DEBUG
from copy import deepcopy
from pprint import pformat
from ..protocol.rfc4511 import LDAPMessage

# logging levels
OFF = 0
ERROR = 10
BASIC = 20
PROTOCOL = 30
NETWORK = 40
EXTENDED = 50

_sensitive_lines = ('simple', 'credentials', 'serversaslcreds')  # must be a tuple, not a list, lowercase
_sensitive_args = ('simple', 'password', 'sasl_credentials', 'saslcreds', 'server_creds')
_sensitive_attrs = ('userpassword', 'unicodepwd')

_hide_sensitive_data = None

DETAIL_LEVELS = [OFF, ERROR, BASIC, PROTOCOL, NETWORK, EXTENDED]

_max_line_length = 4096
_logging_level = 0
_detail_level = 0
_logging_encoding = 'ascii'

try:
    from logging import NullHandler
except ImportError:  # NullHandler not present in Python < 2.7
    from logging import Handler

    class NullHandler(Handler):
        def handle(self, record):
            pass

        def emit(self, record):
            pass

        def createLock(self):
            self.lock = None


def _strip_sensitive_data_from_dict(d):
    if not isinstance(d, dict):
        return d

    try:
        d = deepcopy(d)
    except Exception:  # if deepcopy goes wrong gives up and returns the dict unchanged
        return d
    for k in d.keys():
        if isinstance(d[k], dict):
            d[k] = _strip_sensitive_data_from_dict(d[k])
        elif k.lower() in _sensitive_args and d[k]:
            d[k] = '<stripped %d characters of sensitive data>' % len(d[k])

    return d


def get_detail_level_name(level_name):
    if level_name == OFF:
        return 'OFF'
    elif level_name == ERROR:
        return 'ERROR'
    elif level_name == BASIC:
        return 'BASIC'
    elif level_name == PROTOCOL:
        return 'PROTOCOL'
    elif level_name == NETWORK:
        return 'NETWORK'
    elif level_name == EXTENDED:
        return 'EXTENDED'
    raise ValueError('unknown detail level')


def log(detail, message, *args):
    if detail <= _detail_level:
        if _hide_sensitive_data:
            args = tuple([_strip_sensitive_data_from_dict(arg) if isinstance(arg, dict) else arg for arg in args])

        if str is not bytes:  # Python 3
            encoded_message = (get_detail_level_name(detail) + ':' + message % args).encode(_logging_encoding, 'backslashreplace')
            encoded_message = encoded_message.decode()
        else:
            try:
                encoded_message = (get_detail_level_name(detail) + ':' + message % args).encode(_logging_encoding, 'replace')
            except Exception:
                encoded_message = (get_detail_level_name(detail) + ':' + message % args).decode(_logging_encoding, 'replace')

        if len(encoded_message) > _max_line_length:
            logger.log(_logging_level, encoded_message[:_max_line_length] + ' <removed %d remaining bytes in this log line>' % (len(encoded_message) - _max_line_length, ))
        else:
            logger.log(_logging_level, encoded_message)


def log_enabled(detail):
    if detail <= _detail_level:
        if logger.isEnabledFor(_logging_level):
            return True

    return False


def set_library_log_hide_sensitive_data(hide=True):
    global _hide_sensitive_data
    if hide:
        _hide_sensitive_data = True
    else:
        _hide_sensitive_data = False
    if log_enabled(ERROR):
        log(ERROR, 'hide sensitive data set to ' + str(_hide_sensitive_data))


def get_library_log_hide_sensitive_data():
    return True if _hide_sensitive_data else False


def set_library_log_activation_level(logging_level):
    if isinstance(logging_level, int):
        global _logging_level
        _logging_level = logging_level
    else:
        if log_enabled(ERROR):
            log(ERROR, 'invalid library log activation level <%s> ', logging_level)
        raise ValueError('invalid library log activation level')


def get_library_log_activation_lavel():
    return _logging_level


def set_library_log_max_line_length(length):
    if isinstance(length, int):
        global _max_line_length
        _max_line_length = length
    else:
        if log_enabled(ERROR):
            log(ERROR, 'invalid log max line length <%s> ', length)
        raise ValueError('invalid library log max line length')


def get_library_log_max_line_length():
    return _max_line_length


def set_library_log_detail_level(detail):
    if detail in DETAIL_LEVELS:
        global _detail_level
        _detail_level = detail
        if log_enabled(ERROR):
            log(ERROR, 'detail level set to ' + get_detail_level_name(_detail_level))
    else:
        if log_enabled(ERROR):
            log(ERROR, 'unable to set log detail level to <%s>', detail)
        raise ValueError('invalid library log detail level')


def get_library_log_detail_level():
    return _detail_level


def format_ldap_message(message, prefix):
    if isinstance(message, LDAPMessage):
        try:  # pyasn1 prettyprint raises exception in version 0.4.3
            formatted = message.prettyPrint().split('\n')  # pyasn1 pretty print
        except Exception as e:
            formatted = ['pyasn1 exception', str(e)]
    else:
        formatted = pformat(message).split('\n')

    prefixed = ''
    for line in formatted:
        if line:
            if _hide_sensitive_data and line.strip().lower().startswith(_sensitive_lines):  # _sensitive_lines is a tuple. startswith() method checks each tuple element
                tag, _, data = line.partition('=')
                if data.startswith("b'") and data.endswith("'") or data.startswith('b"') and data.endswith('"'):
                    prefixed += '\n' + prefix + tag + '=<stripped %d characters of sensitive data>' % (len(data) - 3, )
                else:
                    prefixed += '\n' + prefix + tag + '=<stripped %d characters of sensitive data>' % len(data)
            else:
                prefixed += '\n' + prefix + line
    return prefixed

# sets a logger for the library with NullHandler. It can be used by the application with its own logging configuration
logger = getLogger('ldap3')
logger.addHandler(NullHandler())

# sets defaults for the library logging
set_library_log_activation_level(DEBUG)
set_library_log_detail_level(OFF)
set_library_log_hide_sensitive_data(True)
