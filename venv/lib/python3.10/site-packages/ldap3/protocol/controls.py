"""
"""

# Created on 2015.10.20
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

from .rfc4511 import Control, Criticality, LDAPOID
from ..utils.asn1 import encode


def build_control(oid, criticality, value, encode_control_value=True):
    control = Control()
    control.setComponentByName('controlType', LDAPOID(oid))
    control.setComponentByName('criticality', Criticality(criticality))
    if value is not None:
        if encode_control_value:
            control.setComponentByName('controlValue', encode(value))
        else:
            control.setComponentByName('controlValue', value)

    return control
