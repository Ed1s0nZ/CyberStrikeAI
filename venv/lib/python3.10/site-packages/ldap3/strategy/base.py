"""
"""

# Created on 2013.07.15
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
# GNU Lesser General Public License for more dectails.
#
# You should have received a copy of the GNU Lesser General Public License
# along with ldap3 in the COPYING and COPYING.LESSER files.
# If not, see <http://www.gnu.org/licenses/>.

import socket
try:  # try to discover if unix sockets are available for LDAP over IPC (ldapi:// scheme)
    # noinspection PyUnresolvedReferences
    from socket import AF_UNIX
    unix_socket_available = True
except ImportError:
    unix_socket_available = False
from struct import pack
from platform import system
from random import choice

from .. import SYNC, ANONYMOUS, get_config_parameter, BASE, ALL_ATTRIBUTES, ALL_OPERATIONAL_ATTRIBUTES, NO_ATTRIBUTES, DIGEST_MD5
from ..core.results import DO_NOT_RAISE_EXCEPTIONS, RESULT_REFERRAL
from ..core.exceptions import LDAPOperationResult, LDAPSASLBindInProgressError, LDAPSocketOpenError, LDAPSessionTerminatedByServerError,\
    LDAPUnknownResponseError, LDAPUnknownRequestError, LDAPReferralError, communication_exception_factory, LDAPStartTLSError, \
    LDAPSocketSendError, LDAPExceptionError, LDAPControlError, LDAPResponseTimeoutError, LDAPTransactionError
from ..utils.uri import parse_uri
from ..protocol.rfc4511 import LDAPMessage, ProtocolOp, MessageID, SearchResultEntry
from ..operation.add import add_response_to_dict, add_request_to_dict
from ..operation.modify import modify_request_to_dict, modify_response_to_dict
from ..operation.search import search_result_reference_response_to_dict, search_result_done_response_to_dict,\
    search_result_entry_response_to_dict, search_request_to_dict, search_result_entry_response_to_dict_fast,\
    search_result_reference_response_to_dict_fast, attributes_to_dict, attributes_to_dict_fast
from ..operation.bind import bind_response_to_dict, bind_request_to_dict, sicily_bind_response_to_dict, bind_response_to_dict_fast, \
    sicily_bind_response_to_dict_fast
from ..operation.compare import compare_response_to_dict, compare_request_to_dict
from ..operation.extended import extended_request_to_dict, extended_response_to_dict, intermediate_response_to_dict, extended_response_to_dict_fast, intermediate_response_to_dict_fast
from ..core.server import Server
from ..operation.modifyDn import modify_dn_request_to_dict, modify_dn_response_to_dict
from ..operation.delete import delete_response_to_dict, delete_request_to_dict
from ..protocol.convert import prepare_changes_for_request, build_controls_list
from ..operation.abandon import abandon_request_to_dict
from ..core.tls import Tls
from ..protocol.oid import Oids
from ..protocol.rfc2696 import RealSearchControlValue
from ..protocol.microsoft import DirSyncControlResponseValue
from ..utils.log import log, log_enabled, ERROR, BASIC, PROTOCOL, NETWORK, EXTENDED, format_ldap_message
from ..utils.asn1 import encode, decoder, ldap_result_to_dict_fast, decode_sequence
from ..utils.conv import to_unicode
from ..protocol.sasl.digestMd5 import md5_h, md5_hmac

SESSION_TERMINATED_BY_SERVER = 'TERMINATED_BY_SERVER'
TRANSACTION_ERROR = 'TRANSACTION_ERROR'
RESPONSE_COMPLETE = 'RESPONSE_FROM_SERVER_COMPLETE'


# noinspection PyProtectedMember
class BaseStrategy(object):
    """
    Base class for connection strategy
    """

    def __init__(self, ldap_connection):
        self.connection = ldap_connection
        self._outstanding = None
        self._referrals = []
        self.sync = None  # indicates a synchronous connection
        self.no_real_dsa = None  # indicates a connection to a fake LDAP server
        self.pooled = None  # Indicates a connection with a connection pool
        self.can_stream = None  # indicates if a strategy keeps a stream of responses (i.e. LdifProducer can accumulate responses with a single header). Stream must be initialized and closed in _start_listen() and _stop_listen()
        self.referral_cache = {}
        self.thread_safe = False  # Indicates that connection can be used in a multithread application
        if log_enabled(BASIC):
            log(BASIC, 'instantiated <%s>: <%s>', self.__class__.__name__, self)

    def __str__(self):
        s = [
            str(self.connection) if self.connection else 'None',
            'sync' if self.sync else 'async',
            'no real DSA' if self.no_real_dsa else 'real DSA',
            'pooled' if self.pooled else 'not pooled',
            'can stream output' if self.can_stream else 'cannot stream output',
        ]
        return ' - '.join(s)

    def open(self, reset_usage=True, read_server_info=True):
        """
        Open a socket to a server. Choose a server from the server pool if available
        """
        if log_enabled(NETWORK):
            log(NETWORK, 'opening connection for <%s>', self.connection)
        if self.connection.lazy and not self.connection._executing_deferred:
            self.connection._deferred_open = True
            self.connection.closed = False
            if log_enabled(NETWORK):
                log(NETWORK, 'deferring open connection for <%s>', self.connection)
        else:
            if not self.connection.closed and not self.connection._executing_deferred:  # try to close connection if still open
                self.close()

            self._outstanding = dict()
            if self.connection.usage:
                if reset_usage or not self.connection._usage.initial_connection_start_time:
                    self.connection._usage.start()

            if self.connection.server_pool:
                new_server = self.connection.server_pool.get_server(self.connection)  # get a server from the server_pool if available
                if self.connection.server != new_server:
                    self.connection.server = new_server
                    if self.connection.usage:
                        self.connection._usage.servers_from_pool += 1

            exception_history = []
            if not self.no_real_dsa:  # tries to connect to a real server
                for candidate_address in self.connection.server.candidate_addresses():
                    try:
                        if log_enabled(BASIC):
                            log(BASIC, 'try to open candidate address %s', candidate_address[:-2])
                        self._open_socket(candidate_address, self.connection.server.ssl, unix_socket=self.connection.server.ipc)
                        self.connection.server.current_address = candidate_address
                        self.connection.server.update_availability(candidate_address, True)
                        break
                    except Exception as e:
                        self.connection.server.update_availability(candidate_address, False)
                        # exception_history.append((datetime.now(), exc_type, exc_value, candidate_address[4]))
                        exception_history.append((type(e)(str(e)), candidate_address[4]))
                if not self.connection.server.current_address and exception_history:
                    if len(exception_history) == 1:  # only one exception, reraise
                        if log_enabled(ERROR):
                            log(ERROR, '<%s> for <%s>', str(exception_history[0][0]) + ' ' + str((exception_history[0][1])), self.connection)
                        raise exception_history[0][0]
                    else:
                        if log_enabled(ERROR):
                            log(ERROR, 'unable to open socket for <%s>', self.connection)
                        raise LDAPSocketOpenError('unable to open socket', exception_history)
                elif not self.connection.server.current_address:
                    if log_enabled(ERROR):
                        log(ERROR, 'invalid server address for <%s>', self.connection)
                    raise LDAPSocketOpenError('invalid server address')

            self.connection._deferred_open = False
            self._start_listen()
            if log_enabled(NETWORK):
                log(NETWORK, 'connection open for <%s>', self.connection)

    def close(self):
        """
        Close connection
        """
        if log_enabled(NETWORK):
            log(NETWORK, 'closing connection for <%s>', self.connection)
        if self.connection.lazy and not self.connection._executing_deferred and (self.connection._deferred_bind or self.connection._deferred_open):
            self.connection.listening = False
            self.connection.closed = True
            if log_enabled(NETWORK):
                log(NETWORK, 'deferred connection closed for <%s>', self.connection)
        else:
            if not self.connection.closed:
                self._stop_listen()
                if not self. no_real_dsa:
                    self._close_socket()
            if log_enabled(NETWORK):
                log(NETWORK, 'connection closed for <%s>', self.connection)

        self.connection.bound = False
        self.connection.request = None
        self.connection.response = None
        self.connection.tls_started = False
        self._outstanding = None
        self._referrals = []

        if not self.connection.strategy.no_real_dsa:
            self.connection.server.current_address = None
        if self.connection.usage:
            self.connection._usage.stop()

    def _open_socket(self, address, use_ssl=False, unix_socket=False):
        """
        Tries to open and connect a socket to a Server
        raise LDAPExceptionError if unable to open or connect socket
        """
        try:
            self.connection.socket = socket.socket(*address[:3])
        except Exception as e:
            self.connection.last_error = 'socket creation error: ' + str(e)
            if log_enabled(ERROR):
                log(ERROR, '<%s> for <%s>', self.connection.last_error, self.connection)
            # raise communication_exception_factory(LDAPSocketOpenError, exc)(self.connection.last_error)
            raise communication_exception_factory(LDAPSocketOpenError, type(e)(str(e)))(self.connection.last_error)
        # Try to bind the socket locally before connecting to the remote address
        # We go through our connection's source ports and try to bind our socket to our connection's source address
        # with them.
        # If no source address or ports were specified, this will have the same success/fail result as if we
        # tried to connect to the remote server without binding locally first.
        # This is actually a little bit better, as it lets us distinguish the case of "issue binding the socket
        # locally" from "remote server is unavailable" with more clarity, though this will only really be an
        # issue when no source address/port is specified if the system checking server availability is running
        # as a very unprivileged user.
        last_bind_exc = None
        if unix_socket_available and self.connection.socket.family != socket.AF_UNIX:
            socket_bind_succeeded = False
            for source_port in self.connection.source_port_list:
                try:
                    self.connection.socket.bind((self.connection.source_address, source_port))
                    socket_bind_succeeded = True
                    break
                except Exception as bind_ex:
                    last_bind_exc = bind_ex
                    # we'll always end up logging at error level if we cannot bind any ports to the address locally.
                    # but if some work and some don't you probably don't want the ones that don't at ERROR level
                    if log_enabled(NETWORK):
                        log(NETWORK, 'Unable to bind to local address <%s> with source port <%s> due to <%s>',
                            self.connection.source_address, source_port, bind_ex)
            if not socket_bind_succeeded:
                self.connection.last_error = 'socket connection error while locally binding: ' + str(last_bind_exc)
                if log_enabled(ERROR):
                    log(ERROR, 'Unable to locally bind to local address <%s> with any of the source ports <%s> for connection <%s due to <%s>',
                        self.connection.source_address, self.connection.source_port_list, self.connection, last_bind_exc)
                raise communication_exception_factory(LDAPSocketOpenError, type(last_bind_exc)(str(last_bind_exc)))(last_bind_exc)

        try:  # set socket timeout for opening connection
            if self.connection.server.connect_timeout:
                self.connection.socket.settimeout(self.connection.server.connect_timeout)
            self.connection.socket.connect(address[4])
        except socket.error as e:
            self.connection.last_error = 'socket connection error while opening: ' + str(e)
            if log_enabled(ERROR):
                log(ERROR, '<%s> for <%s>', self.connection.last_error, self.connection)
            # raise communication_exception_factory(LDAPSocketOpenError, exc)(self.connection.last_error)
            raise communication_exception_factory(LDAPSocketOpenError, type(e)(str(e)))(self.connection.last_error)

        # Set connection recv timeout (must be set after connect,
        # because socket.settimeout() affects both, connect() as
        # well as recv(). Set it before tls.wrap_socket() because
        # the recv timeout should take effect during the TLS
        # handshake.
        if self.connection.receive_timeout is not None:
            try:  # set receive timeout for the connection socket
                self.connection.socket.settimeout(self.connection.receive_timeout)
                if system().lower() == 'windows':
                    self.connection.socket.setsockopt(socket.SOL_SOCKET, socket.SO_RCVTIMEO, int(1000 * self.connection.receive_timeout))
                else:
                    self.connection.socket.setsockopt(socket.SOL_SOCKET, socket.SO_RCVTIMEO, pack('LL', self.connection.receive_timeout, 0))
            except socket.error as e:
                self.connection.last_error = 'unable to set receive timeout for socket connection: ' + str(e)

        # if exc:
        #     if log_enabled(ERROR):
        #         log(ERROR, '<%s> for <%s>', self.connection.last_error, self.connection)
        #     raise communication_exception_factory(LDAPSocketOpenError, exc)(self.connection.last_error)
                if log_enabled(ERROR):
                    log(ERROR, '<%s> for <%s>', self.connection.last_error, self.connection)
                raise communication_exception_factory(LDAPSocketOpenError, type(e)(str(e)))(self.connection.last_error)

        if use_ssl:
            try:
                self.connection.server.tls.wrap_socket(self.connection, do_handshake=True)
                if self.connection.usage:
                    self.connection._usage.wrapped_sockets += 1
            except Exception as e:
                self.connection.last_error = 'socket ssl wrapping error: ' + str(e)
                if log_enabled(ERROR):
                    log(ERROR, '<%s> for <%s>', self.connection.last_error, self.connection)
                raise communication_exception_factory(LDAPSocketOpenError, type(e)(str(e)))(self.connection.last_error)
        if self.connection.usage:
            self.connection._usage.open_sockets += 1

        self.connection.closed = False

    def _close_socket(self):
        """
        Try to close a socket
        don't raise exception if unable to close socket, assume socket is already closed
        """

        try:
            self.connection.socket.shutdown(socket.SHUT_RDWR)
        except Exception:
            pass

        try:
            self.connection.socket.close()
        except Exception:
            pass

        self.connection.socket = None
        self.connection.closed = True

        if self.connection.usage:
            self.connection._usage.closed_sockets += 1

    def _stop_listen(self):
        self.connection.listening = False

    def send(self, message_type, request, controls=None):
        """
        Send an LDAP message
        Returns the message_id
        """
        self.connection.request = None
        if self.connection.listening:
            if self.connection.sasl_in_progress and message_type not in ['bindRequest']:  # as per RFC4511 (4.2.1)
                self.connection.last_error = 'cannot send operation requests while SASL bind is in progress'
                if log_enabled(ERROR):
                    log(ERROR, '<%s> for <%s>', self.connection.last_error, self.connection)
                raise LDAPSASLBindInProgressError(self.connection.last_error)
            message_id = self.connection.server.next_message_id()
            ldap_message = LDAPMessage()
            ldap_message['messageID'] = MessageID(message_id)
            ldap_message['protocolOp'] = ProtocolOp().setComponentByName(message_type, request)
            message_controls = build_controls_list(controls)
            if message_controls is not None:
                ldap_message['controls'] = message_controls
            self.connection.request = BaseStrategy.decode_request(message_type, request, controls)
            self._outstanding[message_id] = self.connection.request
            self.sending(ldap_message)
        else:
            self.connection.last_error = 'unable to send message, socket is not open'
            if log_enabled(ERROR):
                log(ERROR, '<%s> for <%s>', self.connection.last_error, self.connection)
            raise LDAPSocketOpenError(self.connection.last_error)

        return message_id

    def get_response(self, message_id, timeout=None, get_request=False):
        """
        Get response LDAP messages
        Responses are returned by the underlying connection strategy
        Check if message_id LDAP message is still outstanding and wait for timeout to see if it appears in _get_response
        Result is stored in connection.result
        Responses without result is stored in connection.response
        A tuple (responses, result) is returned
        """
        if timeout is None:
            timeout = get_config_parameter('RESPONSE_WAITING_TIMEOUT')
        response = None
        result = None
        # request = None
        if self._outstanding and message_id in self._outstanding:
            responses = self._get_response(message_id, timeout)

            if not responses:
                if log_enabled(ERROR):
                    log(ERROR, 'socket timeout, no response from server for <%s>', self.connection)
                raise LDAPResponseTimeoutError('no response from server')

            if responses == SESSION_TERMINATED_BY_SERVER:
                try:  # try to close the session but don't raise any error if server has already closed the session
                    self.close()
                except (socket.error, LDAPExceptionError):
                    pass
                self.connection.last_error = 'session terminated by server'
                if log_enabled(ERROR):
                    log(ERROR, '<%s> for <%s>', self.connection.last_error, self.connection)
                raise LDAPSessionTerminatedByServerError(self.connection.last_error)
            elif responses == TRANSACTION_ERROR:  # Novell LDAP Transaction unsolicited notification
                self.connection.last_error = 'transaction error'
                if log_enabled(ERROR):
                    log(ERROR, '<%s> for <%s>', self.connection.last_error, self.connection)
                raise LDAPTransactionError(self.connection.last_error)

            # if referral in response opens a new connection to resolve referrals if requested

            if responses[-2]['result'] == RESULT_REFERRAL:
                if self.connection.usage:
                    self.connection._usage.referrals_received += 1
                if self.connection.auto_referrals:
                    ref_response, ref_result = self.do_operation_on_referral(self._outstanding[message_id], responses[-2]['referrals'])
                    if ref_response is not None:
                        responses = ref_response + [ref_result]
                        responses.append(RESPONSE_COMPLETE)
                    elif ref_result is not None:
                        responses = [ref_result, RESPONSE_COMPLETE]

                    self._referrals = []

            if responses:
                result = responses[-2]
                response = responses[:-2]
                self.connection.result = None
                self.connection.response = None

            if self.connection.raise_exceptions and result and result['result'] not in DO_NOT_RAISE_EXCEPTIONS:
                if log_enabled(PROTOCOL):
                    log(PROTOCOL, 'operation result <%s> for <%s>', result, self.connection)
                self._outstanding.pop(message_id)
                self.connection.result = result.copy()
                raise LDAPOperationResult(result=result['result'], description=result['description'], dn=result['dn'], message=result['message'], response_type=result['type'])

            # checks if any response has a range tag
            # self._auto_range_searching is set as a flag to avoid recursive searches
            if self.connection.auto_range and not hasattr(self, '_auto_range_searching') and any((True for resp in response if 'raw_attributes' in resp for name in resp['raw_attributes'] if ';range=' in name)):
                self._auto_range_searching = result.copy()
                temp_response = response[:]  # copy
                if self.do_search_on_auto_range(self._outstanding[message_id], response):
                    for resp in temp_response:
                        if resp['type'] == 'searchResEntry':
                            keys = [key for key in resp['raw_attributes'] if ';range=' in key]
                            for key in keys:
                                del resp['raw_attributes'][key]
                                del resp['attributes'][key]
                    response = temp_response
                    result = self._auto_range_searching
                del self._auto_range_searching

            if self.connection.empty_attributes:
                for entry in response:
                    if entry['type'] == 'searchResEntry':
                        for attribute_type in self._outstanding[message_id]['attributes']:
                            if attribute_type not in entry['raw_attributes'] and attribute_type not in (ALL_ATTRIBUTES, ALL_OPERATIONAL_ATTRIBUTES, NO_ATTRIBUTES):
                                entry['raw_attributes'][attribute_type] = list()
                                entry['attributes'][attribute_type] = list()
                                if log_enabled(PROTOCOL):
                                    log(PROTOCOL, 'attribute set to empty list for missing attribute <%s> in <%s>', attribute_type, self)
                        if not self.connection.auto_range:
                            attrs_to_remove = []
                            # removes original empty attribute in case a range tag is returned
                            for attribute_type in entry['attributes']:
                                if ';range' in attribute_type.lower():
                                    orig_attr, _, _ = attribute_type.partition(';')
                                    attrs_to_remove.append(orig_attr)
                            for attribute_type in attrs_to_remove:
                                if log_enabled(PROTOCOL):
                                    log(PROTOCOL, 'attribute type <%s> removed in response because of same attribute returned as range by the server in <%s>', attribute_type, self)
                                del entry['raw_attributes'][attribute_type]
                                del entry['attributes'][attribute_type]

            request = self._outstanding.pop(message_id)
        else:
            if log_enabled(ERROR):
                log(ERROR, 'message id not in outstanding queue for <%s>', self.connection)
            raise(LDAPResponseTimeoutError('message id not in outstanding queue'))

        if get_request:
            return response, result, request
        else:
            return response, result

    @staticmethod
    def compute_ldap_message_size(data):
        """
        Compute LDAP Message size according to BER definite length rules
        Returns -1 if too few data to compute message length
        """
        if isinstance(data, str):  # fix for Python 2, data is string not bytes
            data = bytearray(data)  # Python 2 bytearray is equivalent to Python 3 bytes

        ret_value = -1
        if len(data) > 2:
            if data[1] <= 127:  # BER definite length - short form. Highest bit of byte 1 is 0, message length is in the last 7 bits - Value can be up to 127 bytes long
                ret_value = data[1] + 2
            else:  # BER definite length - long form. Highest bit of byte 1 is 1, last 7 bits counts the number of following octets containing the value length
                bytes_length = data[1] - 128
                if len(data) >= bytes_length + 2:
                    value_length = 0
                    cont = bytes_length
                    for byte in data[2:2 + bytes_length]:
                        cont -= 1
                        value_length += byte * (256 ** cont)
                    ret_value = value_length + 2 + bytes_length

        return ret_value

    def decode_response(self, ldap_message):
        """
        Convert received LDAPMessage to a dict
        """
        message_type = ldap_message.getComponentByName('protocolOp').getName()
        component = ldap_message['protocolOp'].getComponent()
        controls = ldap_message['controls'] if ldap_message['controls'].hasValue() else None
        if message_type == 'bindResponse':
            if not bytes(component['matchedDN']).startswith(b'NTLM'):  # patch for microsoft ntlm authentication
                result = bind_response_to_dict(component)
            else:
                result = sicily_bind_response_to_dict(component)
        elif message_type == 'searchResEntry':
            result = search_result_entry_response_to_dict(component, self.connection.server.schema, self.connection.server.custom_formatter, self.connection.check_names)
        elif message_type == 'searchResDone':
            result = search_result_done_response_to_dict(component)
        elif message_type == 'searchResRef':
            result = search_result_reference_response_to_dict(component)
        elif message_type == 'modifyResponse':
            result = modify_response_to_dict(component)
        elif message_type == 'addResponse':
            result = add_response_to_dict(component)
        elif message_type == 'delResponse':
            result = delete_response_to_dict(component)
        elif message_type == 'modDNResponse':
            result = modify_dn_response_to_dict(component)
        elif message_type == 'compareResponse':
            result = compare_response_to_dict(component)
        elif message_type == 'extendedResp':
            result = extended_response_to_dict(component)
        elif message_type == 'intermediateResponse':
            result = intermediate_response_to_dict(component)
        else:
            if log_enabled(ERROR):
                log(ERROR, 'unknown response <%s> for <%s>', message_type, self.connection)
            raise LDAPUnknownResponseError('unknown response')
        result['type'] = message_type
        if controls:
            result['controls'] = dict()
            for control in controls:
                decoded_control = self.decode_control(control)
                result['controls'][decoded_control[0]] = decoded_control[1]
        return result

    def decode_response_fast(self, ldap_message):
        """
        Convert received LDAPMessage from fast ber decoder to a dict
        """
        if ldap_message['protocolOp'] == 1:  # bindResponse
            if not ldap_message['payload'][1][3].startswith(b'NTLM'):  # patch for microsoft ntlm authentication
                result = bind_response_to_dict_fast(ldap_message['payload'])
            else:
                result = sicily_bind_response_to_dict_fast(ldap_message['payload'])
            result['type'] = 'bindResponse'
        elif ldap_message['protocolOp'] == 4:  # searchResEntry'
            result = search_result_entry_response_to_dict_fast(ldap_message['payload'], self.connection.server.schema, self.connection.server.custom_formatter, self.connection.check_names)
            result['type'] = 'searchResEntry'
        elif ldap_message['protocolOp'] == 5:  # searchResDone
            result = ldap_result_to_dict_fast(ldap_message['payload'])
            result['type'] = 'searchResDone'
        elif ldap_message['protocolOp'] == 19:  # searchResRef
            result = search_result_reference_response_to_dict_fast(ldap_message['payload'])
            result['type'] = 'searchResRef'
        elif ldap_message['protocolOp'] == 7:  # modifyResponse
            result = ldap_result_to_dict_fast(ldap_message['payload'])
            result['type'] = 'modifyResponse'
        elif ldap_message['protocolOp'] == 9:  # addResponse
            result = ldap_result_to_dict_fast(ldap_message['payload'])
            result['type'] = 'addResponse'
        elif ldap_message['protocolOp'] == 11:  # delResponse
            result = ldap_result_to_dict_fast(ldap_message['payload'])
            result['type'] = 'delResponse'
        elif ldap_message['protocolOp'] == 13:  # modDNResponse
            result = ldap_result_to_dict_fast(ldap_message['payload'])
            result['type'] = 'modDNResponse'
        elif ldap_message['protocolOp'] == 15:  # compareResponse
            result = ldap_result_to_dict_fast(ldap_message['payload'])
            result['type'] = 'compareResponse'
        elif ldap_message['protocolOp'] == 24:  # extendedResp
            result = extended_response_to_dict_fast(ldap_message['payload'])
            result['type'] = 'extendedResp'
        elif ldap_message['protocolOp'] == 25:  # intermediateResponse
            result = intermediate_response_to_dict_fast(ldap_message['payload'])
            result['type'] = 'intermediateResponse'
        else:
            if log_enabled(ERROR):
                log(ERROR, 'unknown response <%s> for <%s>', ldap_message['protocolOp'], self.connection)
            raise LDAPUnknownResponseError('unknown response')
        if ldap_message['controls']:
            result['controls'] = dict()
            for control in ldap_message['controls']:
                decoded_control = self.decode_control_fast(control[3])
                result['controls'][decoded_control[0]] = decoded_control[1]
        return result

    @staticmethod
    def decode_control(control):
        """
        decode control, return a 2-element tuple where the first element is the control oid
        and the second element is a dictionary with description (from Oids), criticality and decoded control value
        """
        control_type = str(control['controlType'])
        criticality = bool(control['criticality'])
        control_value = bytes(control['controlValue'])
        unprocessed = None
        if control_type == '1.2.840.113556.1.4.319':  # simple paged search as per RFC2696
            control_resp, unprocessed = decoder.decode(control_value, asn1Spec=RealSearchControlValue())
            control_value = dict()
            control_value['size'] = int(control_resp['size'])
            control_value['cookie'] = bytes(control_resp['cookie'])
        elif control_type == '1.2.840.113556.1.4.841':  # DirSync AD
            control_resp, unprocessed = decoder.decode(control_value, asn1Spec=DirSyncControlResponseValue())
            control_value = dict()
            control_value['more_results'] = bool(control_resp['MoreResults'])  # more_result if nonzero
            control_value['cookie'] = bytes(control_resp['CookieServer'])
        elif control_type == '1.3.6.1.1.13.1' or control_type == '1.3.6.1.1.13.2':  # Pre-Read control, Post-Read Control as per RFC 4527
            control_resp, unprocessed = decoder.decode(control_value, asn1Spec=SearchResultEntry())
            control_value = dict()
            control_value['result'] = attributes_to_dict(control_resp['attributes'])
        if unprocessed:
            if log_enabled(ERROR):
                log(ERROR, 'unprocessed control response in substrate')
            raise LDAPControlError('unprocessed control response in substrate')
        return control_type, {'description': Oids.get(control_type, ''), 'criticality': criticality, 'value': control_value}

    @staticmethod
    def decode_control_fast(control, from_server=True):
        """
        decode control, return a 2-element tuple where the first element is the control oid
        and the second element is a dictionary with description (from Oids), criticality and decoded control value
        """
        control_type = str(to_unicode(control[0][3], from_server=from_server))
        criticality = False
        control_value = None
        for r in control[1:]:
            if r[2] == 4:  # controlValue
                control_value = r[3]
            else:
                criticality = False if r[3] == 0 else True  # criticality (booleand default to False)
        if control_type == '1.2.840.113556.1.4.319':  # simple paged search as per RFC2696
            control_resp = decode_sequence(control_value, 0, len(control_value))
            control_value = dict()
            control_value['size'] = int(control_resp[0][3][0][3])
            control_value['cookie'] = bytes(control_resp[0][3][1][3])
        elif control_type == '1.2.840.113556.1.4.841':  # DirSync AD
            control_resp = decode_sequence(control_value, 0, len(control_value))
            control_value = dict()
            control_value['more_results'] = True if control_resp[0][3][0][3] else False  # more_result if nonzero
            control_value['cookie'] = control_resp[0][3][2][3]
        elif control_type == '1.3.6.1.1.13.1' or control_type == '1.3.6.1.1.13.2':  # Pre-Read control, Post-Read Control as per RFC 4527
            control_resp = decode_sequence(control_value, 0, len(control_value))
            control_value = dict()
            control_value['result'] = attributes_to_dict_fast(control_resp[0][3][1][3])
        return control_type, {'description': Oids.get(control_type, ''), 'criticality': criticality, 'value': control_value}

    @staticmethod
    def decode_request(message_type, component, controls=None):
        # message_type = ldap_message.getComponentByName('protocolOp').getName()
        # component = ldap_message['protocolOp'].getComponent()
        if message_type == 'bindRequest':
            result = bind_request_to_dict(component)
        elif message_type == 'unbindRequest':
            result = dict()
        elif message_type == 'addRequest':
            result = add_request_to_dict(component)
        elif message_type == 'compareRequest':
            result = compare_request_to_dict(component)
        elif message_type == 'delRequest':
            result = delete_request_to_dict(component)
        elif message_type == 'extendedReq':
            result = extended_request_to_dict(component)
        elif message_type == 'modifyRequest':
            result = modify_request_to_dict(component)
        elif message_type == 'modDNRequest':
            result = modify_dn_request_to_dict(component)
        elif message_type == 'searchRequest':
            result = search_request_to_dict(component)
        elif message_type == 'abandonRequest':
            result = abandon_request_to_dict(component)
        else:
            if log_enabled(ERROR):
                log(ERROR, 'unknown request <%s>', message_type)
            raise LDAPUnknownRequestError('unknown request')
        result['type'] = message_type
        result['controls'] = controls

        return result

    def valid_referral_list(self, referrals):
        referral_list = []
        for referral in referrals:
            candidate_referral = parse_uri(referral)
            if candidate_referral:
                for ref_host in self.connection.server.allowed_referral_hosts:
                    if ref_host[0] == candidate_referral['host'] or ref_host[0] == '*':
                        if candidate_referral['host'] not in self._referrals:
                            candidate_referral['anonymousBindOnly'] = not ref_host[1]
                            referral_list.append(candidate_referral)
                            break

        return referral_list

    def do_next_range_search(self, request, response, attr_name):
        done = False
        current_response = response
        while not done:
            attr_type, _, returned_range = attr_name.partition(';range=')
            _, _, high_range = returned_range.partition('-')
            response['raw_attributes'][attr_type] += current_response['raw_attributes'][attr_name]
            response['attributes'][attr_type] += current_response['attributes'][attr_name]
            if high_range != '*':
                if log_enabled(PROTOCOL):
                    log(PROTOCOL, 'performing next search on auto-range <%s> via <%s>', str(int(high_range) + 1), self.connection)
                requested_range = attr_type + ';range=' + str(int(high_range) + 1) + '-*'
                result = self.connection.search(search_base=response['dn'],
                                                search_filter='(objectclass=*)',
                                                search_scope=BASE,
                                                dereference_aliases=request['dereferenceAlias'],
                                                attributes=[attr_type + ';range=' + str(int(high_range) + 1) + '-*'])
                if self.connection.strategy.thread_safe:
                    status, result, _response, _ = result
                else:
                    status = result
                    result = self.connection.result
                    _response = self.connection.response

                if self.connection.strategy.sync:
                    if status:
                        current_response = _response[0]
                    else:
                        done = True
                else:
                    current_response, _ = self.get_response(status)
                    current_response = current_response[0]

                if not done:
                    if requested_range in current_response['raw_attributes'] and len(current_response['raw_attributes'][requested_range]) == 0:
                        del current_response['raw_attributes'][requested_range]
                        del current_response['attributes'][requested_range]
                    attr_name = list(filter(lambda a: ';range=' in a, current_response['raw_attributes'].keys()))[0]
                    continue
            done = True

    def do_search_on_auto_range(self, request, response):
        for resp in [r for r in response if r['type'] == 'searchResEntry']:
            for attr_name in list(resp['raw_attributes'].keys()):  # generate list to avoid changing of dict size error
                if ';range=' in attr_name:
                    attr_type, _, range_values = attr_name.partition(';range=')
                    if range_values in ('1-1', '0-0'):  # DirSync returns these values for adding and removing members
                        return False
                    if attr_type not in resp['raw_attributes'] or resp['raw_attributes'][attr_type] is None:
                        resp['raw_attributes'][attr_type] = list()
                    if attr_type not in resp['attributes'] or resp['attributes'][attr_type] is None:
                        resp['attributes'][attr_type] = list()
                    self.do_next_range_search(request, resp, attr_name)
        return True

    def create_referral_connection(self, referrals):
        referral_connection = None
        selected_referral = None
        cachekey = None
        valid_referral_list = self.valid_referral_list(referrals)
        if valid_referral_list:
            preferred_referral_list = [referral for referral in valid_referral_list if
                                       referral['ssl'] == self.connection.server.ssl]
            selected_referral = choice(preferred_referral_list) if preferred_referral_list else choice(
                valid_referral_list)

            cachekey = (selected_referral['host'], selected_referral['port'] or self.connection.server.port, selected_referral['ssl'])
            if self.connection.use_referral_cache and cachekey in self.referral_cache:
                referral_connection = self.referral_cache[cachekey]
            else:
                referral_server = Server(host=selected_referral['host'],
                                         port=selected_referral['port'] or self.connection.server.port,
                                         use_ssl=selected_referral['ssl'],
                                         get_info=self.connection.server.get_info,
                                         formatter=self.connection.server.custom_formatter,
                                         connect_timeout=self.connection.server.connect_timeout,
                                         mode=self.connection.server.mode,
                                         allowed_referral_hosts=self.connection.server.allowed_referral_hosts,
                                         tls=Tls(local_private_key_file=self.connection.server.tls.private_key_file,
                                                 local_certificate_file=self.connection.server.tls.certificate_file,
                                                 validate=self.connection.server.tls.validate,
                                                 version=self.connection.server.tls.version,
                                                 ca_certs_file=self.connection.server.tls.ca_certs_file) if
                                         selected_referral['ssl'] else None)

                from ..core.connection import Connection

                referral_connection = Connection(server=referral_server,
                                                 user=self.connection.user if not selected_referral['anonymousBindOnly'] else None,
                                                 password=self.connection.password if not selected_referral['anonymousBindOnly'] else None,
                                                 version=self.connection.version,
                                                 authentication=self.connection.authentication if not selected_referral['anonymousBindOnly'] else ANONYMOUS,
                                                 client_strategy=SYNC,
                                                 auto_referrals=True,
                                                 read_only=self.connection.read_only,
                                                 check_names=self.connection.check_names,
                                                 raise_exceptions=self.connection.raise_exceptions,
                                                 fast_decoder=self.connection.fast_decoder,
                                                 receive_timeout=self.connection.receive_timeout,
                                                 sasl_mechanism=self.connection.sasl_mechanism,
                                                 sasl_credentials=self.connection.sasl_credentials)

                if self.connection.usage:
                    self.connection._usage.referrals_connections += 1

                referral_connection.open()
                referral_connection.strategy._referrals = self._referrals
                if self.connection.tls_started and not referral_server.ssl:  # if the original server was in start_tls mode and the referral server is not in ssl then start_tls on the referral connection
                    if not referral_connection.start_tls():
                        error = 'start_tls in referral not successful' + (' - ' + referral_connection.last_error if referral_connection.last_error else '')
                        if log_enabled(ERROR):
                            log(ERROR, '%s for <%s>', error, self)
                        self.unbind()
                        raise LDAPStartTLSError(error)

                if self.connection.bound:
                    referral_connection.bind()

            if self.connection.usage:
                self.connection._usage.referrals_followed += 1

        return selected_referral, referral_connection, cachekey

    def do_operation_on_referral(self, request, referrals):
        if log_enabled(PROTOCOL):
            log(PROTOCOL, 'following referral for <%s>', self.connection)
        selected_referral, referral_connection, cachekey = self.create_referral_connection(referrals)
        if selected_referral:
            if request['type'] == 'searchRequest':
                referral_connection.search(selected_referral['base'] or request['base'],
                                           selected_referral['filter'] or request['filter'],
                                           selected_referral['scope'] or request['scope'],
                                           request['dereferenceAlias'],
                                           selected_referral['attributes'] or request['attributes'],
                                           request['sizeLimit'],
                                           request['timeLimit'],
                                           request['typesOnly'],
                                           controls=request['controls'])
            elif request['type'] == 'addRequest':
                referral_connection.add(selected_referral['base'] or request['entry'],
                                        None,
                                        request['attributes'],
                                        controls=request['controls'])
            elif request['type'] == 'compareRequest':
                referral_connection.compare(selected_referral['base'] or request['entry'],
                                            request['attribute'],
                                            request['value'],
                                            controls=request['controls'])
            elif request['type'] == 'delRequest':
                referral_connection.delete(selected_referral['base'] or request['entry'],
                                           controls=request['controls'])
            elif request['type'] == 'extendedReq':
                referral_connection.extended(request['name'],
                                             request['value'],
                                             controls=request['controls'],
                                             no_encode=True
                                             )
            elif request['type'] == 'modifyRequest':
                referral_connection.modify(selected_referral['base'] or request['entry'],
                                           prepare_changes_for_request(request['changes']),
                                           controls=request['controls'])
            elif request['type'] == 'modDNRequest':
                referral_connection.modify_dn(selected_referral['base'] or request['entry'],
                                              request['newRdn'],
                                              request['deleteOldRdn'],
                                              request['newSuperior'],
                                              controls=request['controls'])
            else:
                self.connection.last_error = 'referral operation not permitted'
                if log_enabled(ERROR):
                    log(ERROR, '<%s> for <%s>', self.connection.last_error, self.connection)
                raise LDAPReferralError(self.connection.last_error)

            response = referral_connection.response
            result = referral_connection.result
            if self.connection.use_referral_cache:
                self.referral_cache[cachekey] = referral_connection
            else:
                referral_connection.unbind()
        else:
            response = None
            result = None

        return response, result

    def sending(self, ldap_message):
        if log_enabled(NETWORK):
            log(NETWORK, 'sending 1 ldap message for <%s>', self.connection)
        try:
            encoded_message = encode(ldap_message)
            if self.connection.sasl_mechanism == DIGEST_MD5 and self.connection._digest_md5_kic and not self.connection.sasl_in_progress:
                # If we are using DIGEST-MD5 and LDAP signing is enabled: add a signature to the message
                sec_num = self.connection._digest_md5_sec_num  # added underscore GC
                kic = self.connection._digest_md5_kic  # lowercase GC

                # RFC 2831 : encoded_message = sizeOf(encored_message + signature + 0x0001 + secNum) + encoded_message + signature + 0x0001 + secNum
                signature = bytes.fromhex(md5_hmac(kic, int(sec_num).to_bytes(4, 'big') + encoded_message)[0:20])
                encoded_message = int(len(encoded_message) + 4 + 2 + 10).to_bytes(4, 'big') + encoded_message + signature + int(1).to_bytes(2, 'big') + int(sec_num).to_bytes(4, 'big')
                self.connection._digest_md5_sec_num += 1

            self.connection.socket.sendall(encoded_message)
            if log_enabled(EXTENDED):
                log(EXTENDED, 'ldap message sent via <%s>:%s', self.connection, format_ldap_message(ldap_message, '>>'))
            if log_enabled(NETWORK):
                log(NETWORK, 'sent %d bytes via <%s>', len(encoded_message), self.connection)
        except socket.error as e:
            self.connection.last_error = 'socket sending error' + str(e)
            encoded_message = None
            if log_enabled(ERROR):
                log(ERROR, '<%s> for <%s>', self.connection.last_error, self.connection)
            # raise communication_exception_factory(LDAPSocketSendError, exc)(self.connection.last_error)
            raise communication_exception_factory(LDAPSocketSendError, type(e)(str(e)))(self.connection.last_error)
        if self.connection.usage:
            self.connection._usage.update_transmitted_message(self.connection.request, len(encoded_message))

    def _start_listen(self):
        # overridden on strategy class
        raise NotImplementedError

    def _get_response(self, message_id, timeout):
        # overridden in strategy class
        raise NotImplementedError

    def receiving(self):
        # overridden in strategy class
        raise NotImplementedError

    def post_send_single_response(self, message_id):
        # overridden in strategy class
        raise NotImplementedError

    def post_send_search(self, message_id):
        # overridden in strategy class
        raise NotImplementedError

    def get_stream(self):
        raise NotImplementedError

    def set_stream(self, value):
        raise NotImplementedError

    def unbind_referral_cache(self):
        while len(self.referral_cache) > 0:
            cachekey, referral_connection = self.referral_cache.popitem()
            referral_connection.unbind()
