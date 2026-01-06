"""
"""

# Created on 2014.04.28
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

from pyasn1.type.univ import OctetString, Sequence
from pyasn1.type.namedtype import NamedTypes, OptionalNamedType
from pyasn1.type.tag import Tag, tagClassContext, tagFormatSimple

# Modify password extended operation
# passwdModifyOID OBJECT IDENTIFIER ::= 1.3.6.1.4.1.4203.1.11.1
# PasswdModifyRequestValue ::= SEQUENCE {
#    userIdentity [0] OCTET STRING OPTIONAL
#     oldPasswd [1] OCTET STRING OPTIONAL
#     newPasswd [2] OCTET STRING OPTIONAL }
#
# PasswdModifyResponseValue ::= SEQUENCE {
#     genPasswd [0] OCTET STRING OPTIONAL }


class UserIdentity(OctetString):
    """
    userIdentity [0] OCTET STRING OPTIONAL
    """
    tagSet = OctetString.tagSet.tagImplicitly(Tag(tagClassContext, tagFormatSimple, 0))
    encoding = 'utf-8'


class OldPasswd(OctetString):
    """
    oldPasswd [1] OCTET STRING OPTIONAL
    """
    tagSet = OctetString.tagSet.tagImplicitly(Tag(tagClassContext, tagFormatSimple, 1))
    encoding = 'utf-8'


class NewPasswd(OctetString):
    """
    newPasswd [2] OCTET STRING OPTIONAL
    """
    tagSet = OctetString.tagSet.tagImplicitly(Tag(tagClassContext, tagFormatSimple, 2))
    encoding = 'utf-8'


class GenPasswd(OctetString):
    """
    newPasswd [2] OCTET STRING OPTIONAL
    """
    tagSet = OctetString.tagSet.tagImplicitly(Tag(tagClassContext, tagFormatSimple, 0))
    encoding = 'utf-8'


class PasswdModifyRequestValue(Sequence):
    """
    PasswdModifyRequestValue ::= SEQUENCE {
        userIdentity [0] OCTET STRING OPTIONAL
        oldPasswd [1] OCTET STRING OPTIONAL
        newPasswd [2] OCTET STRING OPTIONAL }
    """
    componentType = NamedTypes(OptionalNamedType('userIdentity', UserIdentity()),
                               OptionalNamedType('oldPasswd', OldPasswd()),
                               OptionalNamedType('newPasswd', NewPasswd()))


class PasswdModifyResponseValue(Sequence):
    """
    PasswdModifyResponseValue ::= SEQUENCE {
       genPasswd [0] OCTET STRING OPTIONAL }
    """

    componentType = NamedTypes(OptionalNamedType('genPasswd', GenPasswd()))
