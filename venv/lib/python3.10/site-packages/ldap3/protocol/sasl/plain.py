"""
"""

# Created on 2014.01.04
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

# payload for PLAIN mechanism
# message   = [authzid] UTF8NUL authcid UTF8NUL passwd
# authcid   = 1*SAFE ; MUST accept up to 255 octets
# authzid   = 1*SAFE ; MUST accept up to 255 octets
# passwd    = 1*SAFE ; MUST accept up to 255 octets
# UTF8NUL   = %x00 ; UTF-8 encoded NUL character
#
# SAFE      = UTF1 / UTF2 / UTF3 / UTF4
#             ;; any UTF-8 encoded Unicode character except NUL
#
# UTF1      = %x01-7F ;; except NUL
# UTF2      = %xC2-DF UTF0
# UTF3      = %xE0 %xA0-BF UTF0 / %xE1-EC 2(UTF0) /
#             %xED %x80-9F UTF0 / %xEE-EF 2(UTF0)
# UTF4      = %xF0 %x90-BF 2(UTF0) / %xF1-F3 3(UTF0) /
#             %xF4 %x80-8F 2(UTF0)
# UTF0      = %x80-BF

from ...protocol.sasl.sasl import send_sasl_negotiation
from .sasl import sasl_prep
from ...utils.conv import to_raw, to_unicode


def sasl_plain(connection, controls):
    authzid = connection.sasl_credentials[0]
    authcid = connection.sasl_credentials[1]
    passwd = connection.sasl_credentials[2]

    payload = b''
    if authzid:
        payload += to_raw(sasl_prep(to_unicode(authzid)))

    payload += b'\0'

    if authcid:
        payload += to_raw(sasl_prep(to_unicode(authcid)))

    payload += b'\0'

    if passwd:
        payload += to_raw(sasl_prep(to_unicode(passwd)))

    result = send_sasl_negotiation(connection, controls, payload)

    return result
