"""
"""

# Created on 2016.07.09
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

from pyasn1.type.namedtype import NamedTypes, NamedType, OptionalNamedType
from pyasn1.type.namedval import NamedValues
from pyasn1.type.univ import Sequence, Integer, Boolean, Enumerated
from .rfc4511 import LDAPDN
from .controls import build_control


class PersistentSearchControl(Sequence):
    # PersistentSearch ::= SEQUENCE {
    #     changeTypes INTEGER,
    #     changesOnly BOOLEAN,
    #     returnECs BOOLEAN
    # }

    componentType = NamedTypes(NamedType('changeTypes', Integer()),
                               NamedType('changesOnly', Boolean()),
                               NamedType('returnECs', Boolean())
                               )


class ChangeType(Enumerated):
    # changeType ENUMERATED {
    #     add             (1),
    #     delete          (2),
    #     modify          (4),
    #     modDN           (8)
    #     }

    namedValues = NamedValues(('add', 1),
                              ('delete', 2),
                              ('modify', 4),
                              ('modDN', 8))


class EntryChangeNotificationControl(Sequence):
    # EntryChangeNotification ::= SEQUENCE {
    #     changeType ENUMERATED {
    #         add             (1),
    #         delete          (2),
    #         modify          (4),
    #         modDN           (8)
    #     },
    #     previousDN   LDAPDN OPTIONAL,     -- modifyDN ops. only
    #     changeNumber INTEGER OPTIONAL     -- if supported
    # }

    # tagSet = TagSet()
    # tagSet = Sequence.tagSet.tagImplicitly(Tag(tagClassUniversal, tagFormatConstructed, 16))
    componentType = NamedTypes(NamedType('changeType', ChangeType()),
                               OptionalNamedType('previousDN', LDAPDN()),
                               OptionalNamedType('changeNumber', Integer())
                               )


def persistent_search_control(change_types, changes_only=True, return_ecs=True, criticality=False):
    control_value = PersistentSearchControl()
    control_value.setComponentByName('changeTypes', Integer(change_types))
    control_value.setComponentByName('changesOnly', Boolean(changes_only))
    control_value.setComponentByName('returnECs', Boolean(return_ecs))
    return build_control('2.16.840.1.113730.3.4.3', criticality, control_value)
