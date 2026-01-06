"""
"""

# Created on 2015.03.27
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

import ctypes

from pyasn1.type.namedtype import NamedTypes, NamedType
from pyasn1.type.tag import Tag, tagClassApplication, tagFormatConstructed
from pyasn1.type.univ import Sequence, OctetString, Integer
from .rfc4511 import ResultCode, LDAPString
from .controls import build_control


class SicilyBindResponse(Sequence):
    # SicilyBindResponse ::= [APPLICATION 1] SEQUENCE {
    #
    #     resultCode   ENUMERATED {
    #                      success                     (0),
    #                      protocolError               (2),
    #                      adminLimitExceeded          (11),
    #                      inappropriateAuthentication (48),
    #                      invalidCredentials          (49),
    #                      busy                        (51),
    #                      unavailable                 (52),
    #                      unwillingToPerform          (53),
    #                      other                       (80) },
    #
    #     serverCreds  OCTET STRING,
    #     errorMessage LDAPString }
    # BindResponse ::= [APPLICATION 1] SEQUENCE {
    #     COMPONENTS OF LDAPResult,
    #     serverSaslCreds    [7] OCTET STRING OPTIONAL }
    tagSet = Sequence.tagSet.tagImplicitly(Tag(tagClassApplication, tagFormatConstructed, 1))
    componentType = NamedTypes(NamedType('resultCode', ResultCode()),
                               NamedType('serverCreds', OctetString()),
                               NamedType('errorMessage', LDAPString())
                               )


class DirSyncControlRequestValue(Sequence):
    # DirSyncRequestValue  ::= SEQUENCE {
    #    Flags      integer
    #    MaxBytes   integer
    #    Cookie     OCTET STRING }
    componentType = NamedTypes(NamedType('Flags', Integer()),
                               NamedType('MaxBytes', Integer()),
                               NamedType('Cookie', OctetString())
                               )


class DirSyncControlResponseValue(Sequence):
    # DirSyncResponseValue ::= SEQUENCE {
    #    MoreResults     INTEGER
    #    unused          INTEGER
    #    CookieServer    OCTET STRING
    #     }
    componentType = NamedTypes(NamedType('MoreResults', Integer()),
                               NamedType('unused', Integer()),
                               NamedType('CookieServer', OctetString())
                               )


class SdFlags(Sequence):
    # SDFlagsRequestValue ::= SEQUENCE {
    #     Flags    INTEGER
    # }
    componentType = NamedTypes(NamedType('Flags', Integer())
                               )


class ExtendedDN(Sequence):
    # A flag value 0 specifies that the GUID and SID values be returned in hexadecimal string
    # A flag value of 1 will return the GUID and SID values in standard string format
    componentType = NamedTypes(NamedType('option', Integer())
                               )


def dir_sync_control(criticality, object_security, ancestors_first, public_data_only, incremental_values, max_length, cookie):
    control_value = DirSyncControlRequestValue()
    flags = 0x0
    if object_security:
        flags |= 0x00000001

    if ancestors_first:
        flags |= 0x00000800

    if public_data_only:
        flags |= 0x00002000

    if incremental_values:
        flags |= 0x80000000
        # converts flags to signed 32 bit (AD expects a 4 bytes long unsigned integer, but ASN.1 Integer type is signed
        # so the BER encoder gives back a 5 bytes long signed integer
        flags = ctypes.c_long(flags & 0xFFFFFFFF).value

    control_value.setComponentByName('Flags', flags)
    control_value.setComponentByName('MaxBytes', max_length)
    if cookie:
        control_value.setComponentByName('Cookie', cookie)
    else:
        control_value.setComponentByName('Cookie', OctetString(''))
    return build_control('1.2.840.113556.1.4.841', criticality, control_value)


def extended_dn_control(criticality=False, hex_format=False):
    control_value = ExtendedDN()
    control_value.setComponentByName('option', Integer(not hex_format))
    return build_control('1.2.840.113556.1.4.529', criticality, control_value)


def show_deleted_control(criticality=False):
    return build_control('1.2.840.113556.1.4.417', criticality, value=None)


def security_descriptor_control(criticality=False, sdflags=0x0F):
    sdcontrol = SdFlags()
    sdcontrol.setComponentByName('Flags', sdflags)
    return [build_control('1.2.840.113556.1.4.801', criticality, sdcontrol)]

def persistent_search_control(criticality=False):
    return build_control('1.2.840.113556.1.4.528', criticality, value=None)