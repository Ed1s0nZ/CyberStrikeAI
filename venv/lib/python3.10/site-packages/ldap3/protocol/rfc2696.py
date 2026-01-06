"""
"""

# Created on 2013.10.15
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

from pyasn1.type.univ import OctetString, Integer, Sequence
from pyasn1.type.namedtype import NamedTypes, NamedType
from pyasn1.type.constraint import ValueRangeConstraint
from .controls import build_control

# constants
# maxInt INTEGER ::= 2147483647 -- (2^^31 - 1) --

MAXINT = Integer(2147483647)

# constraints
rangeInt0ToMaxConstraint = ValueRangeConstraint(0, MAXINT)


class Integer0ToMax(Integer):
    subtypeSpec = Integer.subtypeSpec + rangeInt0ToMaxConstraint


class Size(Integer0ToMax):
    # Size INTEGER (0..maxInt)
    pass


class Cookie(OctetString):
    # cookie          OCTET STRING
    pass


class RealSearchControlValue(Sequence):
    # realSearchControlValue ::= SEQUENCE {
    #     size            INTEGER (0..maxInt),
    #                             -- requested page size from client
    #                             -- result set size estimate from server
    #     cookie          OCTET STRING

    componentType = NamedTypes(NamedType('size', Size()),
                               NamedType('cookie', Cookie()))


def paged_search_control(criticality=False, size=10, cookie=None):
    control_value = RealSearchControlValue()
    control_value.setComponentByName('size', Size(size))
    control_value.setComponentByName('cookie', Cookie(cookie if cookie else ''))

    return build_control('1.2.840.113556.1.4.319', criticality, control_value)
