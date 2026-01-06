"""
"""

# Created on 2016.12.26
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

from ... import SEQUENCE_TYPES, MODIFY_ADD, BASE, DEREF_NEVER
from ...core.exceptions import LDAPInvalidDnError, LDAPOperationsErrorResult
from ...utils.dn import safe_dn


def ad_add_members_to_groups(connection,
                             members_dn,
                             groups_dn,
                             fix=True,
                             raise_error=False):
    """
    :param connection: a bound Connection object
    :param members_dn: the list of members to add to groups
    :param groups_dn: the list of groups where members are to be added
    :param fix: checks for group existence and already assigned members
    :param raise_error: If the operation fails it raises an error instead of returning False
    :return: a boolean where True means that the operation was successful and False means an error has happened
    Establishes users-groups relations following the Active Directory rules: users are added to the member attribute of groups.
    Raises LDAPInvalidDnError if members or groups are not found in the DIT.
    """

    if not isinstance(members_dn, SEQUENCE_TYPES):
        members_dn = [members_dn]

    if not isinstance(groups_dn, SEQUENCE_TYPES):
        groups_dn = [groups_dn]

    if connection.check_names:  # builds new lists with sanitized dn
        members_dn = [safe_dn(member_dn) for member_dn in members_dn]
        groups_dn = [safe_dn(group_dn) for group_dn in groups_dn]

    error = False
    for group in groups_dn:
        if fix:  # checks for existance of group and for already assigned members
            result = connection.search(group, '(objectclass=*)', BASE, dereference_aliases=DEREF_NEVER, attributes=['member'])
            if not connection.strategy.sync:
                response, result = connection.get_response(result)
            else:
                if connection.strategy.thread_safe:
                    _, result, response, _ = result
                else:
                    response = connection.response
                    result = connection.result

            if not result['description'] == 'success':
                raise LDAPInvalidDnError(group + ' not found')

            existing_members = response[0]['attributes']['member'] if 'member' in response[0]['attributes'] else []
            existing_members = [element.lower() for element in existing_members]
        else:
            existing_members = []

        changes = dict()
        member_to_add = [element for element in members_dn if element.lower() not in existing_members]
        if member_to_add:
            changes['member'] = (MODIFY_ADD, member_to_add)
        if changes:
            result = connection.modify(group, changes)
            if not connection.strategy.sync:
                _, result = connection.get_response(result)
            else:
                if connection.strategy.thread_safe:
                    _, result, _, _ = result
                else:
                    result = connection.result
            if result['description'] != 'success':
                error = True
                result_error_params = ['result', 'description', 'dn', 'message']
                if raise_error:
                    raise LDAPOperationsErrorResult([(k, v) for k, v in result.items() if k in result_error_params])
                break

    return not error  # returns True if no error is raised in the LDAP operations
