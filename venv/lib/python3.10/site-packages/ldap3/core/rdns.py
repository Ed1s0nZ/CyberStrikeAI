""" Utilities and constants related to reverse dns lookup """

# Created on 2020.09.16
#
# Author: Azaria Zornberg
#
# Copyright 2020 Giovanni Cannata
# Copyright 2020 Azaria Zornberg
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

import socket


class ReverseDnsSetting(object):
    OFF = 0,
    REQUIRE_RESOLVE_ALL_ADDRESSES = 1,
    REQUIRE_RESOLVE_IP_ADDRESSES_ONLY = 2,
    OPTIONAL_RESOLVE_ALL_ADDRESSES = 3,
    OPTIONAL_RESOLVE_IP_ADDRESSES_ONLY = 4,
    SUPPORTED_VALUES = {OFF, REQUIRE_RESOLVE_ALL_ADDRESSES, REQUIRE_RESOLVE_IP_ADDRESSES_ONLY,
                        OPTIONAL_RESOLVE_ALL_ADDRESSES, OPTIONAL_RESOLVE_IP_ADDRESSES_ONLY}


def get_hostname_by_addr(addr, success_required=True):
    """ Resolve the hostname for an ip address. If success is required, raise an exception if a hostname cannot
    be resolved for the address.
    Returns the hostname resolved for the address.
    If success is not required, returns None for addresses that do not resolve to hostnames.
    """
    try:
        return socket.gethostbyaddr(addr)[0]
    except Exception as ex:
        # if we need to succeed, just re-raise
        if success_required:
            raise
    return None


def is_ip_addr(addr):
    """Returns True if an address is an ipv4 address or an ipv6 address based on format. False otherwise."""
    for addr_type in [socket.AF_INET, socket.AF_INET6]:
        try:
            socket.inet_pton(addr_type, addr)  # not present on python 2.7 on Windows
            return True
        except AttributeError:   # not present on Windows, always valid
            if '.' in addr and any([c.isalpha() for c in addr.replace('.', '')]):  # not an IPv4 address, probably an hostname
                return False
            else:  # do not check for ipv6
                return True
        except OSError:
            pass

    return False
