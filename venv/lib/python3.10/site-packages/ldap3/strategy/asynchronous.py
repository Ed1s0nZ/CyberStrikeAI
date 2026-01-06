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
# GNU Lesser General Public License for more details.
#
# You should have received a copy of the GNU Lesser General Public License
# along with ldap3 in the COPYING and COPYING.LESSER files.
# If not, see <http://www.gnu.org/licenses/>.

from threading import Thread, Lock, Event
import socket

from .. import get_config_parameter, DIGEST_MD5
from ..core.exceptions import LDAPSSLConfigurationError, LDAPStartTLSError, LDAPOperationResult, LDAPSignatureVerificationFailedError
from ..strategy.base import BaseStrategy, RESPONSE_COMPLETE
from ..protocol.rfc4511 import LDAPMessage
from ..utils.log import log, log_enabled, format_ldap_message, ERROR, NETWORK, EXTENDED
from ..utils.asn1 import decoder, decode_message_fast
from ..protocol.sasl.digestMd5 import md5_hmac


# noinspection PyProtectedMember
class AsyncStrategy(BaseStrategy):
    """
    This strategy is asynchronous. You send the request and get the messageId of the request sent
    Receiving data from socket is managed in a separated thread in a blocking mode
    Requests return an int value to indicate the messageId of the requested Operation
    You get the response with get_response, it has a timeout to wait for response to appear
    Connection.response will contain the whole LDAP response for the messageId requested in a dict form
    Connection.request will contain the result LDAP message in a dict form
    Response appear in strategy._responses dictionary
    """

    # noinspection PyProtectedMember
    class ReceiverSocketThread(Thread):
        """
        The thread that actually manage the receiver socket
        """

        def __init__(self, ldap_connection):
            Thread.__init__(self)
            self.connection = ldap_connection
            self.socket_size = get_config_parameter('SOCKET_SIZE')

        def run(self):
            """
            Waits for data on socket, computes the length of the message and waits for enough bytes to decode the message
            Message are appended to strategy._responses
            """
            unprocessed = b''
            get_more_data = True
            listen = True
            data = b''
            sasl_total_bytes_recieved = 0
            sasl_received_data = b''  # used to verify the signature, typo GC
            sasl_next_packet = b''
            sasl_buffer_length = -1
            # sasl_signature = b''  # not needed here GC
            # sasl_sec_num = b'' # used to verify the signature, not needed here GC

            while listen:
                if get_more_data:
                    try:
                        data = self.connection.socket.recv(self.socket_size)
                    except (OSError, socket.error, AttributeError):
                        if self.connection.receive_timeout:  # a receive timeout has been detected - keep kistening on the socket
                            continue
                    except Exception as e:
                        if log_enabled(ERROR):
                            log(ERROR, '<%s> for <%s>', str(e), self.connection)
                        raise  # unexpected exception - re-raise
                    if len(data) > 0:
                        # If we are using DIGEST-MD5 and LDAP signing is set : verify & remove the signature from the message
                        if self.connection.sasl_mechanism == DIGEST_MD5 and self.connection._digest_md5_kis and not self.connection.sasl_in_progress:
                            data = sasl_next_packet + data

                            if sasl_received_data == b'' or sasl_next_packet:
                                # Remove the sizeOf(encoded_message + signature + 0x0001 + secNum) from data.
                                sasl_buffer_length = int.from_bytes(data[0:4], "big")
                                data = data[4:]
                            sasl_next_packet = b''
                            sasl_total_bytes_recieved += len(data)
                            sasl_received_data += data

                            if sasl_total_bytes_recieved >= sasl_buffer_length:
                                # When the LDAP response is splitted accross multiple TCP packets, the SASL buffer length is equal to the MTU of each packet..Which is usually not equal to self.socket_size
                                # This means that the end of one SASL packet/beginning of one other....could be located in the middle of data
                                # We are using "sasl_received_data" instead of "data" & "unprocessed" for this reason

                                # structure of messages when LDAP signing is enabled : sizeOf(encoded_message + signature + 0x0001 + secNum) + encoded_message + signature + 0x0001 + secNum
                                sasl_signature = sasl_received_data[sasl_buffer_length - 16:sasl_buffer_length - 6]
                                sasl_sec_num = sasl_received_data[sasl_buffer_length - 4:sasl_buffer_length]
                                sasl_next_packet = sasl_received_data[sasl_buffer_length:]  # the last "data" variable may contain another sasl packet. We'll process it at the next iteration.
                                sasl_received_data = sasl_received_data[:sasl_buffer_length - 16]  # remove signature + 0x0001 + secNum + the next packet if any, from sasl_received_data

                                kis = self.connection._digest_md5_kis  # renamed to lowercase GC
                                calculated_signature = bytes.fromhex(md5_hmac(kis, sasl_sec_num + sasl_received_data)[0:20])
                                if sasl_signature != calculated_signature:
                                    raise LDAPSignatureVerificationFailedError("Signature verification failed for the recieved LDAP message number " + str(int.from_bytes(sasl_sec_num, 'big')) + ". Expected signature " + calculated_signature.hex() + " but got " + sasl_signature.hex() + ".")
                                sasl_total_bytes_recieved = 0
                                unprocessed += sasl_received_data
                                sasl_received_data = b''
                        else:
                            unprocessed += data
                        data = b''
                    else:
                        listen = False
                length = BaseStrategy.compute_ldap_message_size(unprocessed)
                if length == -1 or len(unprocessed) < length:
                    get_more_data = True
                elif len(unprocessed) >= length:  # add message to message list
                    if self.connection.usage:
                        self.connection._usage.update_received_message(length)
                        if log_enabled(NETWORK):
                            log(NETWORK, 'received %d bytes via <%s>', length, self.connection)
                    if self.connection.fast_decoder:
                        ldap_resp = decode_message_fast(unprocessed[:length])
                        dict_response = self.connection.strategy.decode_response_fast(ldap_resp)
                    else:
                        ldap_resp = decoder.decode(unprocessed[:length], asn1Spec=LDAPMessage())[0]
                        dict_response = self.connection.strategy.decode_response(ldap_resp)
                    message_id = int(ldap_resp['messageID'])
                    if log_enabled(NETWORK):
                        log(NETWORK, 'received 1 ldap message via <%s>', self.connection)
                    if log_enabled(EXTENDED):
                        log(EXTENDED, 'ldap message received via <%s>:%s', self.connection, format_ldap_message(ldap_resp, '<<'))
                    if dict_response['type'] == 'extendedResp' and (dict_response['responseName'] == '1.3.6.1.4.1.1466.20037' or hasattr(self.connection, '_awaiting_for_async_start_tls')):
                        if dict_response['result'] == 0:  # StartTls in progress
                            if self.connection.server.tls:
                                self.connection.server.tls._start_tls(self.connection)
                            else:
                                self.connection.last_error = 'no Tls object defined in Server'
                                if log_enabled(ERROR):
                                    log(ERROR, '<%s> for <%s>', self.connection.last_error, self.connection)
                                raise LDAPSSLConfigurationError(self.connection.last_error)
                        else:
                            self.connection.last_error = 'asynchronous StartTls failed'
                            if log_enabled(ERROR):
                                log(ERROR, '<%s> for <%s>', self.connection.last_error, self.connection)
                            raise LDAPStartTLSError(self.connection.last_error)
                        del self.connection._awaiting_for_async_start_tls
                    if message_id != 0:  # 0 is reserved for 'Unsolicited Notification' from server as per RFC4511 (paragraph 4.4)
                        with self.connection.strategy.async_lock:
                            if message_id in self.connection.strategy._responses:
                                self.connection.strategy._responses[message_id].append(dict_response)
                            else:
                                self.connection.strategy._responses[message_id] = [dict_response]
                            if dict_response['type'] not in ['searchResEntry', 'searchResRef', 'intermediateResponse']:
                                self.connection.strategy._responses[message_id].append(RESPONSE_COMPLETE)
                                self.connection.strategy.set_event_for_message(message_id)

                        if self.connection.strategy.can_stream:  # for AsyncStreamStrategy, used for PersistentSearch
                            self.connection.strategy.accumulate_stream(message_id, dict_response)
                        unprocessed = unprocessed[length:]
                        get_more_data = False if unprocessed else True
                        listen = True if self.connection.listening or unprocessed else False
                    else:  # Unsolicited Notification
                        if dict_response['responseName'] == '1.3.6.1.4.1.1466.20036':  # Notice of Disconnection as per RFC4511 (paragraph 4.4.1)
                            listen = False
                        else:
                            self.connection.last_error = 'unknown unsolicited notification from server'
                            if log_enabled(ERROR):
                                log(ERROR, '<%s> for <%s>', self.connection.last_error, self.connection)
                            raise LDAPStartTLSError(self.connection.last_error)
            self.connection.strategy.close()

    def __init__(self, ldap_connection):
        BaseStrategy.__init__(self, ldap_connection)
        self.sync = False
        self.no_real_dsa = False
        self.pooled = False
        self._responses = None
        self._requests = None
        self.can_stream = False
        self.receiver = None
        self.async_lock = Lock()
        self.event_lock = Lock()
        self._events = {}

    def open(self, reset_usage=True, read_server_info=True):
        """
        Open connection and start listen on the socket in a different thread
        """
        with self.connection.connection_lock:
            self._responses = dict()
            self._requests = dict()
            BaseStrategy.open(self, reset_usage, read_server_info)

        if read_server_info:
            try:
                self.connection.refresh_server_info()
            except LDAPOperationResult:  # catch errors from server if raise_exception = True
                self.connection.server._dsa_info = None
                self.connection.server._schema_info = None

    def close(self):
        """
        Close connection and stop socket thread
        """
        with self.connection.connection_lock:
            BaseStrategy.close(self)

    def _add_event_for_message(self, message_id):
        with self.event_lock:
            # Should have the check here because the receiver thread may has created it
            if message_id not in self._events:
                self._events[message_id] = Event()

    def set_event_for_message(self, message_id):
        with self.event_lock:
            # The receiver thread may receive the response before the sender set the event for the message_id,
            # so we have to check if the event exists
            if message_id not in self._events:
                self._events[message_id] = Event()
            self._events[message_id].set()

    def _get_event_for_message(self, message_id):
        with self.event_lock:
            if message_id not in self._events:
                raise RuntimeError('Event for message[{}] should have been created before accessing'.format(message_id))
            return self._events[message_id]

    def post_send_search(self, message_id):
        """
        Clears connection.response and returns messageId
        """
        self.connection.response = None
        self.connection.request = None
        self.connection.result = None
        self._add_event_for_message(message_id)
        return message_id

    def post_send_single_response(self, message_id):
        """
        Clears connection.response and returns messageId.
        """
        self.connection.response = None
        self.connection.request = None
        self.connection.result = None
        self._add_event_for_message(message_id)
        return message_id

    def _start_listen(self):
        """
        Start thread in daemon mode
        """
        if not self.connection.listening:
            self.receiver = AsyncStrategy.ReceiverSocketThread(self.connection)
            self.connection.listening = True
            self.receiver.daemon = True
            self.receiver.start()

    def _get_response(self, message_id, timeout):
        """
        Performs the capture of LDAP response for this strategy
        The response is only complete after the event been set
        """
        event = self._get_event_for_message(message_id)
        flag = event.wait(timeout)
        if not flag:
            # timeout
            return None

        # In this stage we could ensure the response is already there
        self._events.pop(message_id)
        with self.async_lock:
            return self._responses.pop(message_id)

    def receiving(self):
        raise NotImplementedError

    def get_stream(self):
        raise NotImplementedError

    def set_stream(self, value):
        raise NotImplementedError
