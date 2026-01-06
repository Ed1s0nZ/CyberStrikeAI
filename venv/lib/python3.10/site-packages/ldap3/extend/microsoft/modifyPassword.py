"""
"""

# Created on 2015.11.27
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


from ... import MODIFY_REPLACE, MODIFY_DELETE, MODIFY_ADD
from ...utils.log import log, log_enabled, PROTOCOL
from ...core.results import RESULT_SUCCESS
from ...utils.dn import safe_dn
from ...utils.conv import to_unicode


def ad_modify_password(connection, user_dn, new_password, old_password, controls=None):
    # old password must be None to reset password with sufficient privileges
    if connection.check_names:
        user_dn = safe_dn(user_dn)
    if str is bytes:  # python2, converts to unicode
        new_password = to_unicode(new_password)
        if old_password:
            old_password = to_unicode(old_password)

    encoded_new_password = ('"%s"' % new_password).encode('utf-16-le')

    if old_password:  # normal users must specify old and new password
        encoded_old_password = ('"%s"' % old_password).encode('utf-16-le')
        result = connection.modify(user_dn,
                                   {'unicodePwd': [(MODIFY_DELETE, [encoded_old_password]),
                                                   (MODIFY_ADD, [encoded_new_password])]},
                                   controls)
    else:  # admin users can reset password without sending the old one
        result = connection.modify(user_dn,
                                   {'unicodePwd': [(MODIFY_REPLACE, [encoded_new_password])]},
                                   controls)

    if not connection.strategy.sync:
        _, result = connection.get_response(result)
    else:
        if connection.strategy.thread_safe:
            _, result, _, _ = result
        else:
            result = connection.result

    # change successful, returns True
    if result['result'] == RESULT_SUCCESS:
        return True

    # change was not successful, raises exception if raise_exception = True in connection or returns the operation result, error code is in result['result']
    if connection.raise_exceptions:
        from ...core.exceptions import LDAPOperationResult
        if log_enabled(PROTOCOL):
            log(PROTOCOL, 'operation result <%s> for <%s>', result, connection)
        raise LDAPOperationResult(result=result['result'], description=result['description'], dn=result['dn'], message=result['message'], response_type=result['type'])

    return False
