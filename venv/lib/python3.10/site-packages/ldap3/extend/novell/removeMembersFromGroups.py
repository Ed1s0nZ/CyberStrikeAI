"""
"""

# Created on 2016.04.17
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
from ...core.exceptions import LDAPInvalidDnError
from ... import SEQUENCE_TYPES, MODIFY_DELETE, BASE, DEREF_NEVER
from ...utils.dn import safe_dn


def edir_remove_members_from_groups(connection,
                                    members_dn,
                                    groups_dn,
                                    fix,
                                    transaction):
    """
    :param connection: a bound Connection object
    :param members_dn: the list of members to remove from groups
    :param groups_dn: the list of groups where members are to be removed
    :param fix: checks for inconsistences in the users-groups relation and fixes them
    :param transaction: activates an LDAP transaction
    :return: a boolean where True means that the operation was successful and False means an error has happened
    Removes users-groups relations following the eDirectory rules: groups are removed from securityEquals and groupMembership
    attributes in the member object while members are removed from member and equivalentToMe attributes in the group object.
    Raises LDAPInvalidDnError if members or groups are not found in the DIT.

    """
    if not isinstance(members_dn, SEQUENCE_TYPES):
        members_dn = [members_dn]

    if not isinstance(groups_dn, SEQUENCE_TYPES):
        groups_dn = [groups_dn]

    if connection.check_names:  # builds new lists with sanitized dn
        safe_members_dn = []
        safe_groups_dn = []
        for member_dn in members_dn:
            safe_members_dn.append(safe_dn(member_dn))
        for group_dn in groups_dn:
            safe_groups_dn.append(safe_dn(group_dn))

        members_dn = safe_members_dn
        groups_dn = safe_groups_dn

    transaction_control = None
    error = False

    if transaction:
        transaction_control = connection.extend.novell.start_transaction()

    if not error:
        for member in members_dn:
            if fix:  # checks for existance of member and for already assigned groups
                result = connection.search(member, '(objectclass=*)', BASE, dereference_aliases=DEREF_NEVER, attributes=['securityEquals', 'groupMembership'])

                if not connection.strategy.sync:
                    response, result = connection.get_response(result)
                else:
                    if connection.strategy.thread_safe:
                        _, result, response, _ = result
                    else:
                        response = connection.response
                        result = connection.result

                if not result['description'] == 'success':
                    raise LDAPInvalidDnError(member + ' not found')

                existing_security_equals = response[0]['attributes']['securityEquals'] if 'securityEquals' in response[0]['attributes'] else []
                existing_group_membership = response[0]['attributes']['groupMembership'] if 'groupMembership' in response[0]['attributes'] else []
            else:
                existing_security_equals = groups_dn
                existing_group_membership = groups_dn
            existing_security_equals = [element.lower() for element in existing_security_equals]
            existing_group_membership = [element.lower() for element in existing_group_membership]

            changes = dict()
            security_equals_to_remove = [element for element in groups_dn if element.lower() in existing_security_equals]
            group_membership_to_remove = [element for element in groups_dn if element.lower() in existing_group_membership]
            if security_equals_to_remove:
                changes['securityEquals'] = (MODIFY_DELETE, security_equals_to_remove)
            if group_membership_to_remove:
                changes['groupMembership'] = (MODIFY_DELETE, group_membership_to_remove)
            if changes:
                result = connection.modify(member, changes, controls=[transaction_control] if transaction else None)
                if not connection.strategy.sync:
                    _, result = connection.get_response(result)
                else:
                    if connection.strategy.thread_safe:
                        _, result, _, _ = result
                    else:
                        result = connection.result
                if result['description'] != 'success':
                    error = True
                    break

    if not error:
        for group in groups_dn:
            if fix:  # checks for existance of group and for already assigned members
                result = connection.search(group, '(objectclass=*)', BASE, dereference_aliases=DEREF_NEVER, attributes=['member', 'equivalentToMe'])

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
                existing_equivalent_to_me = response[0]['attributes']['equivalentToMe'] if 'equivalentToMe' in response[0]['attributes'] else []
            else:
                existing_members = members_dn
                existing_equivalent_to_me = members_dn

            existing_members = [element.lower() for element in existing_members]
            existing_equivalent_to_me = [element.lower() for element in existing_equivalent_to_me]

            changes = dict()
            member_to_remove = [element for element in members_dn if element.lower() in existing_members]
            equivalent_to_me_to_remove = [element for element in members_dn if element.lower() in existing_equivalent_to_me]
            if member_to_remove:
                changes['member'] = (MODIFY_DELETE, member_to_remove)
            if equivalent_to_me_to_remove:
                changes['equivalentToMe'] = (MODIFY_DELETE, equivalent_to_me_to_remove)
            if changes:
                result = connection.modify(group, changes, controls=[transaction_control] if transaction else None)
                if not connection.strategy.sync:
                    _, result = connection.get_response(result)
                else:
                    if connection.strategy.thread_safe:
                        _, result, _, _ = result
                    else:
                        result = connection.result
                if result['description'] != 'success':
                    error = True
                    break

    if transaction:
        if error:  # aborts transaction in case of error in the modify operations
            result = connection.extend.novell.end_transaction(commit=False, controls=[transaction_control])
        else:
            result = connection.extend.novell.end_transaction(commit=True, controls=[transaction_control])

        if result['description'] != 'success':
            error = True

    return not error  # return True if no error is raised in the LDAP operations
