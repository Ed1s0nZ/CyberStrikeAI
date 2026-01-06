"""
"""

# Created on 2014.03.15
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

from datetime import datetime, timedelta
from os import linesep

from .exceptions import LDAPMetricsError
from ..utils.log import log, log_enabled, ERROR, BASIC


class ConnectionUsage(object):
    """
    Collect statistics on connection usage
    """

    def reset(self):
        self.open_sockets = 0
        self.closed_sockets = 0
        self.wrapped_sockets = 0
        self.bytes_transmitted = 0
        self.bytes_received = 0
        self.messages_transmitted = 0
        self.messages_received = 0
        self.operations = 0
        self.abandon_operations = 0
        self.add_operations = 0
        self.bind_operations = 0
        self.compare_operations = 0
        self.delete_operations = 0
        self.extended_operations = 0
        self.modify_operations = 0
        self.modify_dn_operations = 0
        self.search_operations = 0
        self.unbind_operations = 0
        self.referrals_received = 0
        self.referrals_followed = 0
        self.referrals_connections = 0
        self.restartable_failures = 0
        self.restartable_successes = 0
        self.servers_from_pool = 0
        if log_enabled(BASIC):
            log(BASIC, 'reset usage metrics')

    def __init__(self):
        self.initial_connection_start_time = None
        self.open_socket_start_time = None
        self.connection_stop_time = None
        self.last_transmitted_time = None
        self.last_received_time = None
        self.open_sockets = 0
        self.closed_sockets = 0
        self.wrapped_sockets = 0
        self.bytes_transmitted = 0
        self.bytes_received = 0
        self.messages_transmitted = 0
        self.messages_received = 0
        self.operations = 0
        self.abandon_operations = 0
        self.add_operations = 0
        self.bind_operations = 0
        self.compare_operations = 0
        self.delete_operations = 0
        self.extended_operations = 0
        self.modify_operations = 0
        self.modify_dn_operations = 0
        self.search_operations = 0
        self.unbind_operations = 0
        self.referrals_received = 0
        self.referrals_followed = 0
        self.referrals_connections = 0
        self.restartable_failures = 0
        self.restartable_successes = 0
        self.servers_from_pool = 0

        if log_enabled(BASIC):
            log(BASIC, 'instantiated Usage object')

    def __repr__(self):
        r = 'Connection Usage:' + linesep
        r += '  Time: [elapsed:          ' + str(self.elapsed_time) + ']' + linesep
        r += '    Initial start time:    ' + (str(self.initial_connection_start_time.isoformat()) if self.initial_connection_start_time else '') + linesep
        r += '    Open socket time:      ' + (str(self.open_socket_start_time.isoformat()) if self.open_socket_start_time else '') + linesep
        r += '    Last transmitted time: ' + (str(self.last_transmitted_time.isoformat()) if self.last_transmitted_time else '') + linesep
        r += '    Last received time:    ' + (str(self.last_received_time.isoformat()) if self.last_received_time else '') + linesep
        r += '    Close socket time:     ' + (str(self.connection_stop_time.isoformat()) if self.connection_stop_time else '') + linesep
        r += '  Server:' + linesep
        r += '    Servers from pool:     ' + str(self.servers_from_pool) + linesep
        r += '    Sockets open:          ' + str(self.open_sockets) + linesep
        r += '    Sockets closed:        ' + str(self.closed_sockets) + linesep
        r += '    Sockets wrapped:       ' + str(self.wrapped_sockets) + linesep
        r += '  Bytes:                   ' + str(self.bytes_transmitted + self.bytes_received) + linesep
        r += '    Transmitted:           ' + str(self.bytes_transmitted) + linesep
        r += '    Received:              ' + str(self.bytes_received) + linesep
        r += '  Messages:                ' + str(self.messages_transmitted + self.messages_received) + linesep
        r += '    Transmitted:           ' + str(self.messages_transmitted) + linesep
        r += '    Received:              ' + str(self.messages_received) + linesep
        r += '  Operations:              ' + str(self.operations) + linesep
        r += '    Abandon:               ' + str(self.abandon_operations) + linesep
        r += '    Bind:                  ' + str(self.bind_operations) + linesep
        r += '    Add:                   ' + str(self.add_operations) + linesep
        r += '    Compare:               ' + str(self.compare_operations) + linesep
        r += '    Delete:                ' + str(self.delete_operations) + linesep
        r += '    Extended:              ' + str(self.extended_operations) + linesep
        r += '    Modify:                ' + str(self.modify_operations) + linesep
        r += '    ModifyDn:              ' + str(self.modify_dn_operations) + linesep
        r += '    Search:                ' + str(self.search_operations) + linesep
        r += '    Unbind:                ' + str(self.unbind_operations) + linesep
        r += '  Referrals:               ' + linesep
        r += '    Received:              ' + str(self.referrals_received) + linesep
        r += '    Followed:              ' + str(self.referrals_followed) + linesep
        r += '    Connections:           ' + str(self.referrals_connections) + linesep
        r += '  Restartable tries:       ' + str(self.restartable_failures + self.restartable_successes) + linesep
        r += '    Failed restarts:       ' + str(self.restartable_failures) + linesep
        r += '    Successful restarts:   ' + str(self.restartable_successes) + linesep
        return r

    def __str__(self):
        return self.__repr__()

    def __iadd__(self, other):
        if not isinstance(other, ConnectionUsage):
            raise LDAPMetricsError('unable to add to ConnectionUsage')

        self.open_sockets += other.open_sockets
        self.closed_sockets += other.closed_sockets
        self.wrapped_sockets += other.wrapped_sockets
        self.bytes_transmitted += other.bytes_transmitted
        self.bytes_received += other.bytes_received
        self.messages_transmitted += other.messages_transmitted
        self.messages_received += other.messages_received
        self.operations += other.operations
        self.abandon_operations += other.abandon_operations
        self.add_operations += other.add_operations
        self.bind_operations += other.bind_operations
        self.compare_operations += other.compare_operations
        self.delete_operations += other.delete_operations
        self.extended_operations += other.extended_operations
        self.modify_operations += other.modify_operations
        self.modify_dn_operations += other.modify_dn_operations
        self.search_operations += other.search_operations
        self.unbind_operations += other.unbind_operations
        self.referrals_received += other.referrals_received
        self.referrals_followed += other.referrals_followed
        self.referrals_connections += other.referrals_connections
        self.restartable_failures += other.restartable_failures
        self.restartable_successes += other.restartable_successes
        self.servers_from_pool += other.servers_from_pool
        return self

    def update_transmitted_message(self, message, length):
        self.last_transmitted_time = datetime.now()
        self.bytes_transmitted += length
        self.operations += 1
        self.messages_transmitted += 1
        if message['type'] == 'abandonRequest':
            self.abandon_operations += 1
        elif message['type'] == 'addRequest':
            self.add_operations += 1
        elif message['type'] == 'bindRequest':
            self.bind_operations += 1
        elif message['type'] == 'compareRequest':
            self.compare_operations += 1
        elif message['type'] == 'delRequest':
            self.delete_operations += 1
        elif message['type'] == 'extendedReq':
            self.extended_operations += 1
        elif message['type'] == 'modifyRequest':
            self.modify_operations += 1
        elif message['type'] == 'modDNRequest':
            self.modify_dn_operations += 1
        elif message['type'] == 'searchRequest':
            self.search_operations += 1
        elif message['type'] == 'unbindRequest':
            self.unbind_operations += 1
        else:
            if log_enabled(ERROR):
                log(ERROR, 'unable to collect usage for unknown message type <%s>', message['type'])
            raise LDAPMetricsError('unable to collect usage for unknown message type')

    def update_received_message(self, length):
        self.last_received_time = datetime.now()
        self.bytes_received += length
        self.messages_received += 1

    def start(self, reset=True):
        if reset:
            self.reset()
        self.open_socket_start_time = datetime.now()
        self.connection_stop_time = None
        if not self.initial_connection_start_time:
            self.initial_connection_start_time = self.open_socket_start_time

        if log_enabled(BASIC):
            log(BASIC, 'start collecting usage metrics')

    def stop(self):
        if self.open_socket_start_time:
            self.connection_stop_time = datetime.now()
            if log_enabled(BASIC):
                log(BASIC, 'stop collecting usage metrics')

    @property
    def elapsed_time(self):
        if self.connection_stop_time:
            return self.connection_stop_time - self.open_socket_start_time
        else:
            return (datetime.now() - self.open_socket_start_time) if self.open_socket_start_time else timedelta(0)
