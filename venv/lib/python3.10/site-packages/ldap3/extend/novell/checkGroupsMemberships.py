"""
"""

# Created on 2016.05.14
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


from .addMembersToGroups import edir_add_members_to_groups
from ...core.exceptions import LDAPInvalidDnError
from ... import SEQUENCE_TYPES, BASE, DEREF_NEVER
from ...utils.dn import safe_dn


def _check_members_have_memberships(connection,
                                    members_dn,
                                    groups_dn):
    """
    :param connection: a bound Connection object
    :param members_dn: the list of members to add to groups
    :param groups_dn: the list of groups where members are to be added
    :return: two booleans. The first when True means that all members have membership in all groups, The second when True means that
    there are inconsistences in the securityEquals attribute
    Checks user's group membership.
    Raises LDAPInvalidDNError if member is not found in the DIT.
    """
    if not isinstance(members_dn, SEQUENCE_TYPES):
        members_dn = [members_dn]

    if not isinstance(groups_dn, SEQUENCE_TYPES):
        groups_dn = [groups_dn]

    partial = False  # True when a member has groupMembership but doesn't have securityEquals
    for member in members_dn:
        result = connection.search(member, '(objectclass=*)', BASE, dereference_aliases=DEREF_NEVER, attributes=['groupMembership', 'securityEquals'])

        if not connection.strategy.sync:
            response, result = connection.get_response(result)
        else:
            if connection.strategy.thread_safe:
                _, result, response, _ = result
            else:
                result = connection.result
                response = connection.response

        if not result['description'] == 'success':  # member not found in DIT
            raise LDAPInvalidDnError(member + ' not found')

        existing_security_equals = response[0]['attributes']['securityEquals'] if 'securityEquals' in response[0]['attributes'] else []
        existing_group_membership = response[0]['attributes']['groupMembership'] if 'groupMembership' in response[0]['attributes'] else []
        existing_security_equals = [element.lower() for element in existing_security_equals]
        existing_group_membership = [element.lower() for element in existing_group_membership]

        for group in groups_dn:
            if group.lower() not in existing_group_membership:
                return False, False
            if group.lower() not in existing_security_equals:
                partial = True

    return True, partial


def _check_groups_contain_members(connection,
                                  groups_dn,
                                  members_dn):
    """
    :param connection: a bound Connection object
    :param members_dn: the list of members to add to groups
    :param groups_dn: the list of groups where members are to be added
    :return: two booleans. The first when True means that all members have membership in all groups, The second when True means that
    there are inconsistences in the EquivalentToMe attribute
    Checks if groups have members in their 'member' attribute.
    Raises LDAPInvalidDNError if member is not found in the DIT.
    """
    if not isinstance(groups_dn, SEQUENCE_TYPES):
        groups_dn = [groups_dn]

    if not isinstance(members_dn, SEQUENCE_TYPES):
        members_dn = [members_dn]

    partial = False  # True when a group has member but doesn't have equivalentToMe
    for group in groups_dn:
        result = connection.search(group, '(objectclass=*)', BASE, dereference_aliases=DEREF_NEVER, attributes=['member', 'equivalentToMe'])

        if not connection.strategy.sync:
            response, result = connection.get_response(result)
        else:
            if connection.strategy.thread_safe:
                _, result, response, _ = result
            else:
                result = connection.result
                response = connection.response

        if not result['description'] == 'success':
            raise LDAPInvalidDnError(group + ' not found')

        existing_members = response[0]['attributes']['member'] if 'member' in response[0]['attributes'] else []
        existing_equivalent_to_me = response[0]['attributes']['equivalentToMe'] if 'equivalentToMe' in response[0]['attributes'] else []
        existing_members = [element.lower() for element in existing_members]
        existing_equivalent_to_me = [element.lower() for element in existing_equivalent_to_me]
        for member in members_dn:
            if member.lower() not in existing_members:
                return False, False
            if member.lower() not in existing_equivalent_to_me:
                partial = True

    return True, partial


def edir_check_groups_memberships(connection,
                                  members_dn,
                                  groups_dn,
                                  fix,
                                  transaction):
    """
    :param connection: a bound Connection object
    :param members_dn: the list of members to check
    :param groups_dn: the list of groups to check
    :param fix: checks for inconsistences in the users-groups relation and fixes them
    :param transaction: activates an LDAP transaction when fixing
    :return: a boolean where True means that the operation was successful and False means an error has happened
    Checks and fixes users-groups relations following the eDirectory rules: groups are checked against 'groupMembership'
    attribute in the member object while members are checked against 'member' attribute in the group object.
    Raises LDAPInvalidDnError if members or groups are not found in the DIT.
    """
    if not isinstance(groups_dn, SEQUENCE_TYPES):
        groups_dn = [groups_dn]

    if not isinstance(members_dn, SEQUENCE_TYPES):
        members_dn = [members_dn]

    if connection.check_names:  # builds new lists with sanitized dn
        safe_members_dn = []
        safe_groups_dn = []
        for member_dn in members_dn:
            safe_members_dn.append(safe_dn(member_dn))
        for group_dn in groups_dn:
            safe_groups_dn.append(safe_dn(group_dn))

        members_dn = safe_members_dn
        groups_dn = safe_groups_dn

    try:
        members_have_memberships, partial_member_security = _check_members_have_memberships(connection, members_dn, groups_dn)
        groups_contain_members, partial_group_security = _check_groups_contain_members(connection, groups_dn, members_dn)
    except LDAPInvalidDnError:
        return False

    if not members_have_memberships and not groups_contain_members:
        return False

    if fix:  # fix any inconsistences
        if (members_have_memberships and not groups_contain_members) \
                or (groups_contain_members and not members_have_memberships) \
                or partial_group_security \
                or partial_member_security:

            for member in members_dn:
                for group in groups_dn:
                    edir_add_members_to_groups(connection, member, group, True, transaction)

    return True
