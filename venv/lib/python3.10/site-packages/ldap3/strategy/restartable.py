"""
"""

# Created on 2014.03.04
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

from time import sleep
import socket

from .. import get_config_parameter
from .sync import SyncStrategy
from ..core.exceptions import LDAPSocketOpenError, LDAPOperationResult, LDAPMaximumRetriesError, LDAPStartTLSError
from ..utils.log import log, log_enabled, ERROR, BASIC


# noinspection PyBroadException,PyProtectedMember
class RestartableStrategy(SyncStrategy):
    def __init__(self, ldap_connection):
        SyncStrategy.__init__(self, ldap_connection)
        self.sync = True
        self.no_real_dsa = False
        self.pooled = False
        self.can_stream = False
        self.restartable_sleep_time = get_config_parameter('RESTARTABLE_SLEEPTIME')
        self.restartable_tries = get_config_parameter('RESTARTABLE_TRIES')
        self._restarting = False
        self._last_bind_controls = None
        self._current_message_type = None
        self._current_request = None
        self._current_controls = None
        self._restart_tls = None
        self.exception_history = []

    def open(self, reset_usage=False, read_server_info=True):
        SyncStrategy.open(self, reset_usage, read_server_info)

    def _open_socket(self, address, use_ssl=False, unix_socket=False):
        """
        Try to open and connect a socket to a Server
        raise LDAPExceptionError if unable to open or connect socket
        if connection is restartable tries for the number of restarting requested or forever
        """
        try:
            SyncStrategy._open_socket(self, address, use_ssl, unix_socket)  # try to open socket using SyncWait
            self._reset_exception_history()
            return
        except Exception as e:  # machinery for restartable connection
            if log_enabled(ERROR):
                log(ERROR, '<%s> while restarting <%s>', e, self.connection)
                self._add_exception_to_history(type(e)(str(e)))

        if not self._restarting:  # if not already performing a restart
            self._restarting = True
            counter = self.restartable_tries
            while counter > 0:  # includes restartable_tries == True
                if log_enabled(BASIC):
                    log(BASIC, 'try #%d to open Restartable connection <%s>', self.restartable_tries - counter, self.connection)
                sleep(self.restartable_sleep_time)
                if not self.connection.closed:
                    try:  # resetting connection
                        self.connection.unbind()
                    except (socket.error, LDAPSocketOpenError):  # don't trace catch socket errors because socket could already be closed
                        pass
                    except Exception as e:
                        if log_enabled(ERROR):
                            log(ERROR, '<%s> while restarting <%s>', e, self.connection)
                        self._add_exception_to_history(type(e)(str(e)))
                try:  # reissuing same operation
                    if self.connection.server_pool:
                        new_server = self.connection.server_pool.get_server(self.connection)  # get a server from the server_pool if available
                        if self.connection.server != new_server:
                            self.connection.server = new_server
                            if self.connection.usage:
                                self.connection._usage.servers_from_pool += 1
                    SyncStrategy._open_socket(self, address, use_ssl, unix_socket)  # calls super (not restartable) _open_socket()
                    if self.connection.usage:
                        self.connection._usage.restartable_successes += 1
                    self.connection.closed = False
                    self._restarting = False
                    self._reset_exception_history()
                    return
                except Exception as e:
                    if log_enabled(ERROR):
                        log(ERROR, '<%s> while restarting <%s>', e, self.connection)
                    self._add_exception_to_history(type(e)(str(e)))
                    if self.connection.usage:
                        self.connection._usage.restartable_failures += 1
                if not isinstance(self.restartable_tries, bool):
                    counter -= 1
            self._restarting = False
            self.connection.last_error = 'restartable connection strategy failed while opening socket'
            if log_enabled(ERROR):
                log(ERROR, '<%s> for <%s>', self.connection.last_error, self.connection)
            raise LDAPMaximumRetriesError(self.connection.last_error, self.exception_history, self.restartable_tries)

    def send(self, message_type, request, controls=None):
        self._current_message_type = message_type
        self._current_request = request
        self._current_controls = controls
        if not self._restart_tls:  # RFCs doesn't define how to stop tls once started
            self._restart_tls = self.connection.tls_started
        if message_type == 'bindRequest':  # stores controls used in bind operation to be used again when restarting the connection
            self._last_bind_controls = controls

        try:
            message_id = SyncStrategy.send(self, message_type, request, controls)  # tries to send using SyncWait
            self._reset_exception_history()
            return message_id
        except Exception as e:
            if log_enabled(ERROR):
                log(ERROR, '<%s> while restarting <%s>', e, self.connection)
            self._add_exception_to_history(type(e)(str(e)))
        if not self._restarting:  # machinery for restartable connection
            self._restarting = True
            counter = self.restartable_tries
            while counter > 0:
                if log_enabled(BASIC):
                    log(BASIC, 'try #%d to send in Restartable connection <%s>', self.restartable_tries - counter, self.connection)
                sleep(self.restartable_sleep_time)
                if not self.connection.closed:
                    try:  # resetting connection
                        self.connection.unbind()
                    except (socket.error, LDAPSocketOpenError):  # don't trace socket errors because socket could already be closed
                        pass
                    except Exception as e:
                        if log_enabled(ERROR):
                            log(ERROR, '<%s> while restarting <%s>', e, self.connection)
                        self._add_exception_to_history(type(e)(str(e)))
                failure = False
                try:  # reopening connection
                    self.connection.open(reset_usage=False, read_server_info=False)
                    if self._restart_tls:  # restart tls if start_tls was previously used
                        if not self.connection.start_tls(read_server_info=False):
                            error = 'restart tls in restartable not successful' + (' - ' + self.connection.last_error if self.connection.last_error else '')
                            if log_enabled(ERROR):
                                log(ERROR, '%s for <%s>', error, self)
                            self.connection.unbind()
                            raise LDAPStartTLSError(error)
                    if message_type != 'bindRequest':
                        self.connection.bind(read_server_info=False, controls=self._last_bind_controls)  # binds with previously used controls unless the request is already a bindRequest
                    if not self.connection.server.schema and not self.connection.server.info:
                        self.connection.refresh_server_info()
                    else:
                        self.connection._fire_deferred(read_info=False)   # in case of lazy connection, not open by the refresh_server_info
                except Exception as e:
                    if log_enabled(ERROR):
                        log(ERROR, '<%s> while restarting <%s>', e, self.connection)
                    self._add_exception_to_history(type(e)(str(e)))
                    failure = True

                if not failure:
                    try:  # reissuing same operation
                        ret_value = self.connection.send(message_type, request, controls)
                        if self.connection.usage:
                            self.connection._usage.restartable_successes += 1
                        self._restarting = False
                        self._reset_exception_history()
                        return ret_value  # successful send
                    except Exception as e:
                        if log_enabled(ERROR):
                            log(ERROR, '<%s> while restarting <%s>', e, self.connection)
                        self._add_exception_to_history(type(e)(str(e)))
                        failure = True

                if failure and self.connection.usage:
                    self.connection._usage.restartable_failures += 1

                if not isinstance(self.restartable_tries, bool):
                    counter -= 1

            self._restarting = False

        self.connection.last_error = 'restartable connection failed to send'
        if log_enabled(ERROR):
            log(ERROR, '<%s> for <%s>', self.connection.last_error, self.connection)
        raise LDAPMaximumRetriesError(self.connection.last_error, self.exception_history, self.restartable_tries)

    def post_send_single_response(self, message_id):
        try:
            ret_value = SyncStrategy.post_send_single_response(self, message_id)
            self._reset_exception_history()
            return ret_value
        except Exception as e:
            if log_enabled(ERROR):
                log(ERROR, '<%s> while restarting <%s>', e, self.connection)
            self._add_exception_to_history(type(e)(str(e)))

        # if an LDAPExceptionError is raised then resend the request
        try:
            ret_value = SyncStrategy.post_send_single_response(self, self.send(self._current_message_type, self._current_request, self._current_controls))
            self._reset_exception_history()
            return ret_value
        except Exception as e:
            if log_enabled(ERROR):
                log(ERROR, '<%s> while restarting <%s>', e, self.connection)
            self._add_exception_to_history(type(e)(str(e)))
            if not isinstance(e, LDAPOperationResult):
                self.connection.last_error = 'restartable connection strategy failed in post_send_single_response'
            if log_enabled(ERROR):
                log(ERROR, '<%s> for <%s>', self.connection.last_error, self.connection)
            raise

    def post_send_search(self, message_id):
        try:
            ret_value = SyncStrategy.post_send_search(self, message_id)
            self._reset_exception_history()
            return ret_value
        except Exception as e:
            if log_enabled(ERROR):
                log(ERROR, '<%s> while restarting <%s>', e, self.connection)
            self._add_exception_to_history(type(e)(str(e)))

        # if an LDAPExceptionError is raised then resend the request
        try:
            ret_value = SyncStrategy.post_send_search(self, self.connection.send(self._current_message_type, self._current_request, self._current_controls))
            self._reset_exception_history()
            return ret_value
        except Exception as e:
            if log_enabled(ERROR):
                log(ERROR, '<%s> while restarting <%s>', e, self.connection)
            self._add_exception_to_history(type(e)(str(e)))
            if not isinstance(e, LDAPOperationResult):
                self.connection.last_error = e.args
            if log_enabled(ERROR):
                log(ERROR, '<%s> for <%s>', self.connection.last_error, self.connection)
            raise e

    def _add_exception_to_history(self, exc):
        if not isinstance(self.restartable_tries, bool):  # doesn't accumulate when restarting forever
            if not isinstance(exc, LDAPMaximumRetriesError):  # doesn't add the LDAPMaximumRetriesError exception
                self.exception_history.append(exc)

    def _reset_exception_history(self):
        if self.exception_history:
            self.exception_history = []

    def get_stream(self):
        raise NotImplementedError

    def set_stream(self, value):
        raise NotImplementedError
