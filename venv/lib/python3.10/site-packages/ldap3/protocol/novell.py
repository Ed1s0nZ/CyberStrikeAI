"""
"""

# Created on 2014.06.27
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

from pyasn1.type.univ import OctetString, Integer, Sequence, SequenceOf
from pyasn1.type.namedtype import NamedType, NamedTypes, OptionalNamedType
from pyasn1.type.tag import Tag, tagFormatSimple, tagClassUniversal, TagSet

NMAS_LDAP_EXT_VERSION = 1


class Identity(OctetString):
    encoding = 'utf-8'


class LDAPDN(OctetString):
    tagSet = OctetString.tagSet.tagImplicitly(Tag(tagClassUniversal, tagFormatSimple, 4))
    encoding = 'utf-8'


class Password(OctetString):
    tagSet = OctetString.tagSet.tagImplicitly(Tag(tagClassUniversal, tagFormatSimple, 4))
    encoding = 'utf-8'


class LDAPOID(OctetString):
    tagSet = OctetString.tagSet.tagImplicitly(Tag(tagClassUniversal, tagFormatSimple, 4))
    encoding = 'utf-8'


class GroupCookie(Integer):
    tagSet = Integer.tagSet.tagImplicitly(Tag(tagClassUniversal, tagFormatSimple, 2))


class NmasVer(Integer):
    tagSet = Integer.tagSet.tagImplicitly(Tag(tagClassUniversal, tagFormatSimple, 2))


class Error(Integer):
    tagSet = Integer.tagSet.tagImplicitly(Tag(tagClassUniversal, tagFormatSimple, 2))


class NmasGetUniversalPasswordRequestValue(Sequence):
    componentType = NamedTypes(NamedType('nmasver', NmasVer()),
                               NamedType('reqdn', Identity())
                               )


class NmasGetUniversalPasswordResponseValue(Sequence):
    componentType = NamedTypes(NamedType('nmasver', NmasVer()),
                               NamedType('err', Error()),
                               OptionalNamedType('passwd', Password())
                               )


class NmasSetUniversalPasswordRequestValue(Sequence):
    componentType = NamedTypes(NamedType('nmasver', NmasVer()),
                               NamedType('reqdn', Identity()),
                               NamedType('new_passwd', Password())
                               )


class NmasSetUniversalPasswordResponseValue(Sequence):
    componentType = NamedTypes(NamedType('nmasver', NmasVer()),
                               NamedType('err', Error())
                               )


class ReplicaList(SequenceOf):
    componentType = OctetString()


class ReplicaInfoRequestValue(Sequence):
    tagSet = TagSet()
    componentType = NamedTypes(NamedType('server_dn', LDAPDN()),
                               NamedType('partition_dn', LDAPDN())
                               )


class ReplicaInfoResponseValue(Sequence):
    # tagSet = Sequence.tagSet.tagImplicitly(Tag(tagClassContext, tagFormatConstructed, 3))
    tagSet = TagSet()
    componentType = NamedTypes(NamedType('partition_id', Integer()),
                               NamedType('replica_state', Integer()),
                               NamedType('modification_time', Integer()),
                               NamedType('purge_time', Integer()),
                               NamedType('local_partition_id', Integer()),
                               NamedType('partition_dn', LDAPDN()),
                               NamedType('replica_type', Integer()),
                               NamedType('flags', Integer())
                               )


class CreateGroupTypeRequestValue(Sequence):
    componentType = NamedTypes(NamedType('createGroupType', LDAPOID()),
                               OptionalNamedType('createGroupValue', OctetString())
                               )


class CreateGroupTypeResponseValue(Sequence):
    componentType = NamedTypes(NamedType('createGroupCookie', GroupCookie()),
                               OptionalNamedType('createGroupValue', OctetString())
                               )


class EndGroupTypeRequestValue(Sequence):
    componentType = NamedTypes(NamedType('endGroupCookie', GroupCookie()),
                               OptionalNamedType('endGroupValue', OctetString())
                               )


class EndGroupTypeResponseValue(Sequence):
    componentType = NamedTypes(OptionalNamedType('endGroupValue', OctetString())
                               )


class GroupingControlValue(Sequence):
    componentType = NamedTypes(NamedType('groupingCookie', GroupCookie()),
                               OptionalNamedType('groupValue', OctetString())
                               )
