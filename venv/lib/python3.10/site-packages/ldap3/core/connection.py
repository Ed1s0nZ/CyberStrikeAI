"""
"""

# Created on 2014.05.31
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
from copy import deepcopy, copy
from os import linesep
from threading import RLock
from functools import reduce
import json

from .. import ANONYMOUS, SIMPLE, SASL, MODIFY_ADD, MODIFY_DELETE, MODIFY_REPLACE, get_config_parameter, DEREF_ALWAYS, \
    SUBTREE, ASYNC, SYNC, NO_ATTRIBUTES, ALL_ATTRIBUTES, ALL_OPERATIONAL_ATTRIBUTES, MODIFY_INCREMENT, LDIF, ASYNC_STREAM, \
    RESTARTABLE, ROUND_ROBIN, REUSABLE, AUTO_BIND_DEFAULT, AUTO_BIND_NONE, AUTO_BIND_TLS_BEFORE_BIND, SAFE_SYNC, SAFE_RESTARTABLE, \
    AUTO_BIND_TLS_AFTER_BIND, AUTO_BIND_NO_TLS, STRING_TYPES, SEQUENCE_TYPES, MOCK_SYNC, MOCK_ASYNC, NTLM, EXTERNAL,\
    DIGEST_MD5, GSSAPI, PLAIN, DSA, SCHEMA, ALL

from .results import RESULT_SUCCESS, RESULT_COMPARE_TRUE, RESULT_COMPARE_FALSE
from ..extend import ExtendedOperationsRoot
from .pooling import ServerPool
from .server import Server
from ..operation.abandon import abandon_operation, abandon_request_to_dict
from ..operation.add import add_operation, add_request_to_dict
from ..operation.bind import bind_operation, bind_request_to_dict
from ..operation.compare import compare_operation, compare_request_to_dict
from ..operation.delete import delete_operation, delete_request_to_dict
from ..operation.extended import extended_operation, extended_request_to_dict
from ..operation.modify import modify_operation, modify_request_to_dict
from ..operation.modifyDn import modify_dn_operation, modify_dn_request_to_dict
from ..operation.search import search_operation, search_request_to_dict
from ..protocol.rfc2849 import operation_to_ldif, add_ldif_header
from ..protocol.sasl.digestMd5 import sasl_digest_md5
from ..protocol.sasl.external import sasl_external
from ..protocol.sasl.plain import sasl_plain
from ..strategy.sync import SyncStrategy
from ..strategy.safeSync import SafeSyncStrategy
from ..strategy.safeRestartable import SafeRestartableStrategy
from ..strategy.mockAsync import MockAsyncStrategy
from ..strategy.asynchronous import AsyncStrategy
from ..strategy.reusable import ReusableStrategy
from ..strategy.restartable import RestartableStrategy
from ..strategy.ldifProducer import LdifProducerStrategy
from ..strategy.mockSync import MockSyncStrategy
from ..strategy.asyncStream import AsyncStreamStrategy
from ..operation.unbind import unbind_operation
from ..protocol.rfc2696 import paged_search_control
from .usage import ConnectionUsage
from .tls import Tls
from .exceptions import LDAPUnknownStrategyError, LDAPBindError, LDAPUnknownAuthenticationMethodError, \
    LDAPSASLMechanismNotSupportedError, LDAPObjectClassError, LDAPConnectionIsReadOnlyError, LDAPChangeError, LDAPExceptionError, \
    LDAPObjectError, LDAPSocketReceiveError, LDAPAttributeError, LDAPInvalidValueError, LDAPInvalidPortError, LDAPStartTLSError

from ..utils.conv import escape_bytes, prepare_for_stream, check_json_dict, format_json, to_unicode
from ..utils.log import log, log_enabled, ERROR, BASIC, PROTOCOL, EXTENDED, get_library_log_hide_sensitive_data
from ..utils.dn import safe_dn
from ..utils.port_validators import check_port_and_port_list


SASL_AVAILABLE_MECHANISMS = [EXTERNAL,
                             DIGEST_MD5,
                             GSSAPI,
                             PLAIN]

CLIENT_STRATEGIES = [SYNC,
                     SAFE_SYNC,
                     SAFE_RESTARTABLE,
                     ASYNC,
                     LDIF,
                     RESTARTABLE,
                     REUSABLE,
                     MOCK_SYNC,
                     MOCK_ASYNC,
                     ASYNC_STREAM]


def _format_socket_endpoint(endpoint):
    if endpoint and len(endpoint) == 2:  # IPv4
        return str(endpoint[0]) + ':' + str(endpoint[1])
    elif endpoint and len(endpoint) == 4:  # IPv6
        return '[' + str(endpoint[0]) + ']:' + str(endpoint[1])

    try:
        return str(endpoint)
    except Exception:
        return '?'


def _format_socket_endpoints(sock):
    if sock:
        try:
            local = sock.getsockname()
        except Exception:
            local = (None, None, None, None)
        try:
            remote = sock.getpeername()
        except Exception:
            remote = (None, None, None, None)

        return '<local: ' + _format_socket_endpoint(local) + ' - remote: ' + _format_socket_endpoint(remote) + '>'
    return '<no socket>'


class Connection(object):
    """Main ldap connection class.

    Controls, if used, must be a list of tuples. Each tuple must have 3
    elements, the control OID, a boolean meaning if the control is
    critical, a value.

    If the boolean is set to True the server must honor the control or
    refuse the operation

    Mixing controls must be defined in controls specification (as per
    RFC 4511)

    :param server: the Server object to connect to
    :type server: Server, str
    :param user: the user name for simple authentication
    :type user: str
    :param password: the password for simple authentication
    :type password: str
    :param auto_bind: specify if the bind will be performed automatically when defining the Connection object
    :type auto_bind: int, can be one of AUTO_BIND_DEFAULT, AUTO_BIND_NONE, AUTO_BIND_NO_TLS, AUTO_BIND_TLS_BEFORE_BIND, AUTO_BIND_TLS_AFTER_BIND as specified in ldap3
    :param version: LDAP version, default to 3
    :type version: int
    :param authentication: type of authentication
    :type authentication: int, can be one of ANONYMOUS, SIMPLE or SASL, as specified in ldap3
    :param client_strategy: communication strategy used in the Connection
    :type client_strategy: can be one of SYNC, ASYNC, LDIF, RESTARTABLE, REUSABLE as specified in ldap3
    :param auto_referrals: specify if the connection object must automatically follow referrals
    :type auto_referrals: bool
    :param sasl_mechanism: mechanism for SASL authentication, can be one of 'EXTERNAL', 'DIGEST-MD5', 'GSSAPI', 'PLAIN'
    :type sasl_mechanism: str
    :param sasl_credentials: credentials for SASL mechanism
    :type sasl_credentials: tuple
    :param check_names: if True the library will check names of attributes and object classes against the schema. Also values found in entries will be formatted as indicated by the schema
    :type check_names: bool
    :param collect_usage: collect usage metrics in the usage attribute
    :type collect_usage: bool
    :param read_only: disable operations that modify data in the LDAP server
    :type read_only: bool
    :param lazy: open and bind the connection only when an actual operation is performed
    :type lazy: bool
    :param raise_exceptions: raise exceptions when operations are not successful, if False operations return False if not successful but not raise exceptions
    :type raise_exceptions: bool
    :param pool_name: pool name for pooled strategies
    :type pool_name: str
    :param pool_size: pool size for pooled strategies
    :type pool_size: int
    :param pool_lifetime: pool lifetime for pooled strategies
    :type pool_lifetime: int
    :param cred_store: credential store for gssapi
    :type cred_store: dict
    :param use_referral_cache: keep referral connections open and reuse them
    :type use_referral_cache: bool
    :param auto_escape: automatic escaping of filter values
    :type auto_escape: bool
    :param auto_encode: automatic encoding of attribute values
    :type auto_encode: bool
    :param source_address: the ip address or hostname to use as the source when opening the connection to the server
    :type source_address: str
    :param source_port: the source port to use when opening the connection to the server. Cannot be specified with source_port_list
    :type source_port: int
    :param source_port_list: a list of source ports to choose from when opening the connection to the server. Cannot be specified with source_port
    :type source_port_list: list
    """
    def __init__(self,
                 server,
                 user=None,
                 password=None,
                 auto_bind=AUTO_BIND_DEFAULT,
                 version=3,
                 authentication=None,
                 client_strategy=SYNC,
                 auto_referrals=True,
                 auto_range=True,
                 sasl_mechanism=None,
                 sasl_credentials=None,
                 check_names=True,
                 collect_usage=False,
                 read_only=False,
                 lazy=False,
                 raise_exceptions=False,
                 pool_name=None,
                 pool_size=None,
                 pool_lifetime=None,
                 cred_store=None,
                 fast_decoder=True,
                 receive_timeout=None,
                 return_empty_attributes=True,
                 use_referral_cache=False,
                 auto_escape=True,
                 auto_encode=True,
                 pool_keepalive=None,
                 source_address=None,
                 source_port=None,
                 source_port_list=None):

        conf_default_pool_name = get_config_parameter('DEFAULT_THREADED_POOL_NAME')
        self.connection_lock = RLock()  # re-entrant lock to ensure that operations in the Connection object are executed atomically in the same thread
        with self.connection_lock:
            if client_strategy not in CLIENT_STRATEGIES:
                self.last_error = 'unknown client connection strategy'
                if log_enabled(ERROR):
                    log(ERROR, '%s for <%s>', self.last_error, self)
                raise LDAPUnknownStrategyError(self.last_error)

            self.strategy_type = client_strategy
            self.user = user
            self.password = password

            if not authentication and self.user:
                self.authentication = SIMPLE
            elif not authentication:
                self.authentication = ANONYMOUS
            elif authentication in [SIMPLE, ANONYMOUS, SASL, NTLM]:
                self.authentication = authentication
            else:
                self.last_error = 'unknown authentication method'
                if log_enabled(ERROR):
                    log(ERROR, '%s for <%s>', self.last_error, self)
                raise LDAPUnknownAuthenticationMethodError(self.last_error)

            self.version = version
            self.auto_referrals = True if auto_referrals else False
            self.request = None
            self.response = None
            self.result = None
            self.bound = False
            self.listening = False
            self.closed = True
            self.last_error = None
            if auto_bind is False:  # compatibility with older version where auto_bind was a boolean
                self.auto_bind = AUTO_BIND_DEFAULT
            elif auto_bind is True:
                self.auto_bind = AUTO_BIND_NO_TLS
            else:
                self.auto_bind = auto_bind
            self.sasl_mechanism = sasl_mechanism
            self.sasl_credentials = sasl_credentials
            self._usage = ConnectionUsage() if collect_usage else None
            self.socket = None
            self.tls_started = False
            self.sasl_in_progress = False
            self.read_only = read_only
            self._context_state = []
            self._deferred_open = False
            self._deferred_bind = False
            self._deferred_start_tls = False
            self._bind_controls = None
            self._executing_deferred = False
            self.lazy = lazy
            self.pool_name = pool_name if pool_name else conf_default_pool_name
            self.pool_size = pool_size
            self.cred_store = cred_store
            self.pool_lifetime = pool_lifetime
            self.pool_keepalive = pool_keepalive
            self.starting_tls = False
            self.check_names = check_names
            self.raise_exceptions = raise_exceptions
            self.auto_range = True if auto_range else False
            self.extend = ExtendedOperationsRoot(self)
            self._entries = []
            self.fast_decoder = fast_decoder
            self.receive_timeout = receive_timeout
            self.empty_attributes = return_empty_attributes
            self.use_referral_cache = use_referral_cache
            self.auto_escape = auto_escape
            self.auto_encode = auto_encode
            self._digest_md5_kic = None
            self._digest_md5_kis = None
            self._digest_md5_sec_num = 0

            port_err = check_port_and_port_list(source_port, source_port_list)
            if port_err:
                if log_enabled(ERROR):
                    log(ERROR, port_err)
                raise LDAPInvalidPortError(port_err)
            # using an empty string to bind a socket means "use the default as if this wasn't provided" because socket
            # binding requires that you pass something for the ip if you want to pass a specific port
            self.source_address = source_address if source_address is not None else ''
            # using 0 as the source port to bind a socket means "use the default behavior of picking a random port from
            # all ports as if this wasn't provided" because socket binding requires that you pass something for the port
            # if you want to pass a specific ip
            self.source_port_list = [0]
            if source_port is not None:
                self.source_port_list = [source_port]
            elif source_port_list is not None:
                self.source_port_list = source_port_list[:]

            if isinstance(server, STRING_TYPES):
                server = Server(server)
            if isinstance(server, SEQUENCE_TYPES):
                server = ServerPool(server, ROUND_ROBIN, active=True, exhaust=True)

            if isinstance(server, ServerPool):
                self.server_pool = server
                self.server_pool.initialize(self)
                self.server = self.server_pool.get_current_server(self)
            else:
                self.server_pool = None
                self.server = server

            # if self.authentication == SIMPLE and self.user and self.check_names:
            #     self.user = safe_dn(self.user)
            #     if log_enabled(EXTENDED):
            #         log(EXTENDED, 'user name sanitized to <%s> for simple authentication via <%s>', self.user, self)

            if self.strategy_type == SYNC:
                self.strategy = SyncStrategy(self)
            elif self.strategy_type == SAFE_SYNC:
                self.strategy = SafeSyncStrategy(self)
            elif self.strategy_type == SAFE_RESTARTABLE:
                self.strategy = SafeRestartableStrategy(self)
            elif self.strategy_type == ASYNC:
                self.strategy = AsyncStrategy(self)
            elif self.strategy_type == LDIF:
                self.strategy = LdifProducerStrategy(self)
            elif self.strategy_type == RESTARTABLE:
                self.strategy = RestartableStrategy(self)
            elif self.strategy_type == REUSABLE:
                self.strategy = ReusableStrategy(self)
                self.lazy = False
            elif self.strategy_type == MOCK_SYNC:
                self.strategy = MockSyncStrategy(self)
            elif self.strategy_type == MOCK_ASYNC:
                self.strategy = MockAsyncStrategy(self)
            elif self.strategy_type == ASYNC_STREAM:
                self.strategy = AsyncStreamStrategy(self)
            else:
                self.last_error = 'unknown strategy'
                if log_enabled(ERROR):
                    log(ERROR, '%s for <%s>', self.last_error, self)
                raise LDAPUnknownStrategyError(self.last_error)

            # maps strategy functions to connection functions
            self.send = self.strategy.send
            self.open = self.strategy.open
            self.get_response = self.strategy.get_response
            self.post_send_single_response = self.strategy.post_send_single_response
            self.post_send_search = self.strategy.post_send_search

            if not self.strategy.no_real_dsa:
                self._do_auto_bind()
            # else:  # for strategies with a fake server set get_info to NONE if server hasn't a schema
            #     if self.server and not self.server.schema:
            #         self.server.get_info = NONE
            if log_enabled(BASIC):
                if get_library_log_hide_sensitive_data():
                    log(BASIC, 'instantiated Connection: <%s>', self.repr_with_sensitive_data_stripped())
                else:
                    log(BASIC, 'instantiated Connection: <%r>', self)

    def _prepare_return_value(self, status, response=False):
        if self.strategy.thread_safe:
            temp_response = self.response
            self.response = None
            temp_request = self.request
            self.request = None
            return status, deepcopy(self.result), deepcopy(temp_response) if response else None, copy(temp_request)
        return status

    def _do_auto_bind(self):
        if self.auto_bind and self.auto_bind not in [AUTO_BIND_NONE, AUTO_BIND_DEFAULT]:
            if log_enabled(BASIC):
                log(BASIC, 'performing automatic bind for <%s>', self)
            if self.closed:
                self.open(read_server_info=False)
            if self.auto_bind == AUTO_BIND_NO_TLS:
                self.bind(read_server_info=True)
            elif self.auto_bind == AUTO_BIND_TLS_BEFORE_BIND:
                if self.start_tls(read_server_info=False):
                    self.bind(read_server_info=True)
                else:
                    error = 'automatic start_tls befored bind not successful' + (' - ' + self.last_error if self.last_error else '')
                    if log_enabled(ERROR):
                        log(ERROR, '%s for <%s>', error, self)
                    self.unbind()  # unbind anyway to close connection
                    raise LDAPStartTLSError(error)
            elif self.auto_bind == AUTO_BIND_TLS_AFTER_BIND:
                self.bind(read_server_info=False)
                if not self.start_tls(read_server_info=True):
                    error = 'automatic start_tls after bind not successful' + (' - ' + self.last_error if self.last_error else '')
                    if log_enabled(ERROR):
                        log(ERROR, '%s for <%s>', error, self)
                    self.unbind()
                    raise LDAPStartTLSError(error)
            if not self.bound:
                error = 'automatic bind not successful' + (' - ' + self.last_error if self.last_error else '')
                if log_enabled(ERROR):
                    log(ERROR, '%s for <%s>', error, self)
                self.unbind()
                raise LDAPBindError(error)

    def __str__(self):
        s = [
            str(self.server) if self.server else 'None',
            'user: ' + str(self.user),
            'lazy' if self.lazy else 'not lazy',
            'unbound' if not self.bound else ('deferred bind' if self._deferred_bind else 'bound'),
            'closed' if self.closed else ('deferred open' if self._deferred_open else 'open'),
            _format_socket_endpoints(self.socket),
            'tls not started' if not self.tls_started else('deferred start_tls' if self._deferred_start_tls else 'tls started'),
            'listening' if self.listening else 'not listening',
            self.strategy.__class__.__name__ if hasattr(self, 'strategy') else 'No strategy',
            'internal decoder' if self.fast_decoder else 'pyasn1 decoder'
        ]
        return ' - '.join(s)

    def __repr__(self):
        conf_default_pool_name = get_config_parameter('DEFAULT_THREADED_POOL_NAME')
        if self.server_pool:
            r = 'Connection(server={0.server_pool!r}'.format(self)
        else:
            r = 'Connection(server={0.server!r}'.format(self)
        r += '' if self.user is None else ', user={0.user!r}'.format(self)
        r += '' if self.password is None else ', password={0.password!r}'.format(self)
        r += '' if self.auto_bind is None else ', auto_bind={0.auto_bind!r}'.format(self)
        r += '' if self.version is None else ', version={0.version!r}'.format(self)
        r += '' if self.authentication is None else ', authentication={0.authentication!r}'.format(self)
        r += '' if self.strategy_type is None else ', client_strategy={0.strategy_type!r}'.format(self)
        r += '' if self.auto_referrals is None else ', auto_referrals={0.auto_referrals!r}'.format(self)
        r += '' if self.sasl_mechanism is None else ', sasl_mechanism={0.sasl_mechanism!r}'.format(self)
        r += '' if self.sasl_credentials is None else ', sasl_credentials={0.sasl_credentials!r}'.format(self)
        r += '' if self.check_names is None else ', check_names={0.check_names!r}'.format(self)
        r += '' if self.usage is None else (', collect_usage=' + ('True' if self.usage else 'False'))
        r += '' if self.read_only is None else ', read_only={0.read_only!r}'.format(self)
        r += '' if self.lazy is None else ', lazy={0.lazy!r}'.format(self)
        r += '' if self.raise_exceptions is None else ', raise_exceptions={0.raise_exceptions!r}'.format(self)
        r += '' if (self.pool_name is None or self.pool_name == conf_default_pool_name) else ', pool_name={0.pool_name!r}'.format(self)
        r += '' if self.pool_size is None else ', pool_size={0.pool_size!r}'.format(self)
        r += '' if self.pool_lifetime is None else ', pool_lifetime={0.pool_lifetime!r}'.format(self)
        r += '' if self.pool_keepalive is None else ', pool_keepalive={0.pool_keepalive!r}'.format(self)
        r += '' if self.cred_store is None else (', cred_store=' + repr(self.cred_store))
        r += '' if self.fast_decoder is None else (', fast_decoder=' + ('True' if self.fast_decoder else 'False'))
        r += '' if self.auto_range is None else (', auto_range=' + ('True' if self.auto_range else 'False'))
        r += '' if self.receive_timeout is None else ', receive_timeout={0.receive_timeout!r}'.format(self)
        r += '' if self.empty_attributes is None else (', return_empty_attributes=' + ('True' if self.empty_attributes else 'False'))
        r += '' if self.auto_encode is None else (', auto_encode=' + ('True' if self.auto_encode else 'False'))
        r += '' if self.auto_escape is None else (', auto_escape=' + ('True' if self.auto_escape else 'False'))
        r += '' if self.use_referral_cache is None else (', use_referral_cache=' + ('True' if self.use_referral_cache else 'False'))
        r += ')'

        return r

    def repr_with_sensitive_data_stripped(self):
        conf_default_pool_name = get_config_parameter('DEFAULT_THREADED_POOL_NAME')
        if self.server_pool:
            r = 'Connection(server={0.server_pool!r}'.format(self)
        else:
            r = 'Connection(server={0.server!r}'.format(self)
        r += '' if self.user is None else ', user={0.user!r}'.format(self)
        r += '' if self.password is None else ", password='{0}'".format('<stripped %d characters of sensitive data>' % len(self.password))
        r += '' if self.auto_bind is None else ', auto_bind={0.auto_bind!r}'.format(self)
        r += '' if self.version is None else ', version={0.version!r}'.format(self)
        r += '' if self.authentication is None else ', authentication={0.authentication!r}'.format(self)
        r += '' if self.strategy_type is None else ', client_strategy={0.strategy_type!r}'.format(self)
        r += '' if self.auto_referrals is None else ', auto_referrals={0.auto_referrals!r}'.format(self)
        r += '' if self.sasl_mechanism is None else ', sasl_mechanism={0.sasl_mechanism!r}'.format(self)
        if self.sasl_mechanism == DIGEST_MD5:
            r += '' if self.sasl_credentials is None else ", sasl_credentials=({0!r}, {1!r}, '{2}', {3!r})".format(self.sasl_credentials[0], self.sasl_credentials[1], '*' * len(self.sasl_credentials[2]), self.sasl_credentials[3])
        else:
            r += '' if self.sasl_credentials is None else ', sasl_credentials={0.sasl_credentials!r}'.format(self)
        r += '' if self.check_names is None else ', check_names={0.check_names!r}'.format(self)
        r += '' if self.usage is None else (', collect_usage=' + 'True' if self.usage else 'False')
        r += '' if self.read_only is None else ', read_only={0.read_only!r}'.format(self)
        r += '' if self.lazy is None else ', lazy={0.lazy!r}'.format(self)
        r += '' if self.raise_exceptions is None else ', raise_exceptions={0.raise_exceptions!r}'.format(self)
        r += '' if (self.pool_name is None or self.pool_name == conf_default_pool_name) else ', pool_name={0.pool_name!r}'.format(self)
        r += '' if self.pool_size is None else ', pool_size={0.pool_size!r}'.format(self)
        r += '' if self.pool_lifetime is None else ', pool_lifetime={0.pool_lifetime!r}'.format(self)
        r += '' if self.pool_keepalive is None else ', pool_keepalive={0.pool_keepalive!r}'.format(self)
        r += '' if self.cred_store is None else (', cred_store=' + repr(self.cred_store))
        r += '' if self.fast_decoder is None else (', fast_decoder=' + 'True' if self.fast_decoder else 'False')
        r += '' if self.auto_range is None else (', auto_range=' + ('True' if self.auto_range else 'False'))
        r += '' if self.receive_timeout is None else ', receive_timeout={0.receive_timeout!r}'.format(self)
        r += '' if self.empty_attributes is None else (', return_empty_attributes=' + 'True' if self.empty_attributes else 'False')
        r += '' if self.auto_encode is None else (', auto_encode=' + ('True' if self.auto_encode else 'False'))
        r += '' if self.auto_escape is None else (', auto_escape=' + ('True' if self.auto_escape else 'False'))
        r += '' if self.use_referral_cache is None else (', use_referral_cache=' + ('True' if self.use_referral_cache else 'False'))
        r += ')'

        return r

    @property
    def stream(self):
        """Used by the LDIFProducer strategy to accumulate the ldif-change operations with a single LDIF header
        :return: reference to the response stream if defined in the strategy.
        """
        return self.strategy.get_stream() if self.strategy.can_stream else None

    @stream.setter
    def stream(self, value):
        with self.connection_lock:
            if self.strategy.can_stream:
                self.strategy.set_stream(value)

    @property
    def usage(self):
        """Usage statistics for the connection.
        :return: Usage object
        """
        if not self._usage:
            return None
        if self.strategy.pooled:  # update master connection usage from pooled connections
            self._usage.reset()
            for worker in self.strategy.pool.workers:
                self._usage += worker.connection.usage
            self._usage += self.strategy.pool.terminated_usage
        return self._usage

    def __enter__(self):
        with self.connection_lock:
            self._context_state.append((self.bound, self.closed))  # save status out of context as a tuple in a list
            if self.auto_bind != AUTO_BIND_NONE:
                if self.auto_bind == AUTO_BIND_DEFAULT:
                    self.auto_bind = AUTO_BIND_NO_TLS
                if self.closed:
                    self.open()
                if not self.bound:
                    if not self.bind():
                        raise LDAPBindError('unable to bind')

            return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        with self.connection_lock:
            context_bound, context_closed = self._context_state.pop()
            if (not context_bound and self.bound) or self.stream:  # restore status prior to entering context
                try:
                    self.unbind()
                except LDAPExceptionError:
                    pass

            if not context_closed and self.closed:
                self.open()

            if exc_type is not None:
                if log_enabled(ERROR):
                    log(ERROR, '%s for <%s>', exc_type, self)
                return False  # re-raise LDAPExceptionError

    def bind(self,
             read_server_info=True,
             controls=None):
        """Bind to ldap Server with the authentication method and the user defined in the connection

        :param read_server_info: reads info from server
        :param controls: LDAP controls to send along with the bind operation
        :type controls: list of tuple
        :return: bool

        """
        if log_enabled(BASIC):
            log(BASIC, 'start BIND operation via <%s>', self)
        self.last_error = None
        with self.connection_lock:
            if self.lazy and not self._executing_deferred:
                if self.strategy.pooled:
                    self.strategy.validate_bind(controls)
                self._deferred_bind = True
                self._bind_controls = controls
                self.bound = True
                if log_enabled(BASIC):
                    log(BASIC, 'deferring bind for <%s>', self)
            else:
                self._deferred_bind = False
                self._bind_controls = None
                if self.closed:  # try to open connection if closed
                    self.open(read_server_info=False)
                if self.authentication == ANONYMOUS:
                    if log_enabled(PROTOCOL):
                        log(PROTOCOL, 'performing anonymous BIND for <%s>', self)
                    if not self.strategy.pooled:
                        request = bind_operation(self.version, self.authentication, self.user, '', auto_encode=self.auto_encode)
                        if log_enabled(PROTOCOL):
                            log(PROTOCOL, 'anonymous BIND request <%s> sent via <%s>', bind_request_to_dict(request), self)
                        response = self.post_send_single_response(self.send('bindRequest', request, controls))
                    else:
                        response = self.strategy.validate_bind(controls)  # only for REUSABLE
                elif self.authentication == SIMPLE:
                    if log_enabled(PROTOCOL):
                        log(PROTOCOL, 'performing simple BIND for <%s>', self)
                    if not self.strategy.pooled:
                        request = bind_operation(self.version, self.authentication, self.user, self.password, auto_encode=self.auto_encode)
                        if log_enabled(PROTOCOL):
                            log(PROTOCOL, 'simple BIND request <%s> sent via <%s>', bind_request_to_dict(request), self)
                        response = self.post_send_single_response(self.send('bindRequest', request, controls))
                    else:
                        response = self.strategy.validate_bind(controls)  # only for REUSABLE
                elif self.authentication == SASL:
                    if self.sasl_mechanism in SASL_AVAILABLE_MECHANISMS:
                        if log_enabled(PROTOCOL):
                            log(PROTOCOL, 'performing SASL BIND for <%s>', self)
                        if not self.strategy.pooled:
                            response = self.do_sasl_bind(controls)
                        else:
                            response = self.strategy.validate_bind(controls)  # only for REUSABLE
                    else:
                        self.last_error = 'requested SASL mechanism not supported'
                        if log_enabled(ERROR):
                            log(ERROR, '%s for <%s>', self.last_error, self)
                        raise LDAPSASLMechanismNotSupportedError(self.last_error)
                elif self.authentication == NTLM:
                    if self.user and self.password and len(self.user.split('\\')) == 2:
                        if log_enabled(PROTOCOL):
                            log(PROTOCOL, 'performing NTLM BIND for <%s>', self)
                        if not self.strategy.pooled:
                            response = self.do_ntlm_bind(controls)
                        else:
                            response = self.strategy.validate_bind(controls)  # only for REUSABLE
                    else:  # user or password missing
                        self.last_error = 'NTLM needs domain\\username and a password'
                        if log_enabled(ERROR):
                            log(ERROR, '%s for <%s>', self.last_error, self)
                        raise LDAPUnknownAuthenticationMethodError(self.last_error)
                else:
                    self.last_error = 'unknown authentication method'
                    if log_enabled(ERROR):
                        log(ERROR, '%s for <%s>', self.last_error, self)
                    raise LDAPUnknownAuthenticationMethodError(self.last_error)

                if not self.strategy.sync and not self.strategy.pooled and self.authentication not in (SASL, NTLM):  # get response if asynchronous except for SASL and NTLM that return the bind result even for asynchronous strategy
                    _, result = self.get_response(response)
                    if log_enabled(PROTOCOL):
                        log(PROTOCOL, 'async BIND response id <%s> received via <%s>', result, self)
                elif self.strategy.sync:
                    result = self.result
                    if log_enabled(PROTOCOL):
                        log(PROTOCOL, 'BIND response <%s> received via <%s>', result, self)
                elif self.strategy.pooled or self.authentication in (SASL, NTLM):  # asynchronous SASL and NTLM or reusable strtegy get the bind result synchronously
                    result = response
                else:
                    self.last_error = 'unknown authentication method'
                    if log_enabled(ERROR):
                        log(ERROR, '%s for <%s>', self.last_error, self)
                    raise LDAPUnknownAuthenticationMethodError(self.last_error)

                if result is None:
                    # self.bound = True if self.strategy_type == REUSABLE else False
                    self.bound = False
                elif result is True:
                    self.bound = True
                elif result is False:
                    self.bound = False
                else:
                    self.bound = True if result['result'] == RESULT_SUCCESS else False
                    if not self.bound and result and result['description'] and not self.last_error:
                        self.last_error = result['description']

                if read_server_info and self.bound:
                    self.refresh_server_info()
            self._entries = []

            if log_enabled(BASIC):
                log(BASIC, 'done BIND operation, result <%s>', self.bound)

            return self._prepare_return_value(self.bound, self.result)

    def rebind(self,
               user=None,
               password=None,
               authentication=None,
               sasl_mechanism=None,
               sasl_credentials=None,
               read_server_info=True,
               controls=None
               ):

        if log_enabled(BASIC):
            log(BASIC, 'start (RE)BIND operation via <%s>', self)
        self.last_error = None
        with self.connection_lock:
            if user:
                self.user = user
            if password is not None:
                self.password = password
            if not authentication and user:
                self.authentication = SIMPLE
            if authentication in [SIMPLE, ANONYMOUS, SASL, NTLM]:
                self.authentication = authentication
            elif authentication is not None:
                self.last_error = 'unknown authentication method'
                if log_enabled(ERROR):
                    log(ERROR, '%s for <%s>', self.last_error, self)
                raise LDAPUnknownAuthenticationMethodError(self.last_error)
            if sasl_mechanism:
                self.sasl_mechanism = sasl_mechanism
            if sasl_credentials:
                self.sasl_credentials = sasl_credentials

            # if self.authentication == SIMPLE and self.user and self.check_names:
            #     self.user = safe_dn(self.user)
            #     if log_enabled(EXTENDED):
            #         log(EXTENDED, 'user name sanitized to <%s> for rebind via <%s>', self.user, self)

            if not self.strategy.pooled:
                try:
                    return self.bind(read_server_info, controls)
                except LDAPSocketReceiveError:
                    self.last_error = 'Unable to rebind as a different user, furthermore the server abruptly closed the connection'
                    if log_enabled(ERROR):
                        log(ERROR, '%s for <%s>', self.last_error, self)
                    raise LDAPBindError(self.last_error)
            else:
                self.strategy.pool.rebind_pool()
                return self._prepare_return_value(True, self.result)

    def unbind(self,
               controls=None):
        """Unbind the connected user. Unbind implies closing session as per RFC4511 (4.3)

        :param controls: LDAP controls to send along with the bind operation

        """
        if log_enabled(BASIC):
            log(BASIC, 'start UNBIND operation via <%s>', self)

        if self.use_referral_cache:
            self.strategy.unbind_referral_cache()

        self.last_error = None
        with self.connection_lock:
            if self.lazy and not self._executing_deferred and (self._deferred_bind or self._deferred_open):  # _clear deferred status
                self.strategy.close()
                self._deferred_open = False
                self._deferred_bind = False
                self._deferred_start_tls = False
            elif not self.closed:
                request = unbind_operation()
                if log_enabled(PROTOCOL):
                    log(PROTOCOL, 'UNBIND request sent via <%s>', self)
                self.send('unbindRequest', request, controls)
                self.strategy.close()

            if log_enabled(BASIC):
                log(BASIC, 'done UNBIND operation, result <%s>', True)

            return self._prepare_return_value(True)

    def search(self,
               search_base,
               search_filter,
               search_scope=SUBTREE,
               dereference_aliases=DEREF_ALWAYS,
               attributes=None,
               size_limit=0,
               time_limit=0,
               types_only=False,
               get_operational_attributes=False,
               controls=None,
               paged_size=None,
               paged_criticality=False,
               paged_cookie=None,
               auto_escape=None):
        """
        Perform an ldap search:

        - If attributes is empty noRFC2696 with the specified size
        - If paged is 0 and cookie is present the search is abandoned on
          server attribute is returned
        - If attributes is ALL_ATTRIBUTES all attributes are returned
        - If paged_size is an int greater than 0 a simple paged search
          is tried as described in
        - Cookie is an opaque string received in the last paged search
          and must be used on the next paged search response
        - If lazy == True open and bind will be deferred until another
          LDAP operation is performed
        - If mssing_attributes == True then an attribute not returned by the server is set to None
        - If auto_escape is set it overrides the Connection auto_escape
        """
        conf_attributes_excluded_from_check = [v.lower() for v in get_config_parameter('ATTRIBUTES_EXCLUDED_FROM_CHECK')]
        if log_enabled(BASIC):
            log(BASIC, 'start SEARCH operation via <%s>', self)

        if self.check_names and search_base:
            search_base = safe_dn(search_base)
            if log_enabled(EXTENDED):
                log(EXTENDED, 'search base sanitized to <%s> for SEARCH operation via <%s>', search_base, self)

        with self.connection_lock:
            self._fire_deferred()
            if not attributes:
                attributes = [NO_ATTRIBUTES]
            elif attributes == ALL_ATTRIBUTES:
                attributes = [ALL_ATTRIBUTES]

            if isinstance(attributes, STRING_TYPES):
                attributes = [attributes]

            if get_operational_attributes and isinstance(attributes, list):
                attributes.append(ALL_OPERATIONAL_ATTRIBUTES)
            elif get_operational_attributes and isinstance(attributes, tuple):
                attributes += (ALL_OPERATIONAL_ATTRIBUTES, )  # concatenate tuple

            if isinstance(paged_size, int):
                if log_enabled(PROTOCOL):
                    log(PROTOCOL, 'performing paged search for %d items with cookie <%s> for <%s>', paged_size, escape_bytes(paged_cookie), self)

                if controls is None:
                    controls = []
                else:
                    # Copy the controls to prevent modifying the original object
                    controls = list(controls)
                controls.append(paged_search_control(paged_criticality, paged_size, paged_cookie))

            if self.server and self.server.schema and self.check_names:
                for attribute_name in attributes:
                    if ';' in attribute_name:  # remove tags
                        attribute_name_to_check = attribute_name.split(';')[0]
                    else:
                        attribute_name_to_check = attribute_name
                    if self.server.schema and attribute_name_to_check.lower() not in conf_attributes_excluded_from_check and attribute_name_to_check not in self.server.schema.attribute_types:
                        self.last_error = 'invalid attribute type ' + attribute_name_to_check
                        if log_enabled(ERROR):
                            log(ERROR, '%s for <%s>', self.last_error, self)
                        raise LDAPAttributeError(self.last_error)

            request = search_operation(search_base,
                                       search_filter,
                                       search_scope,
                                       dereference_aliases,
                                       attributes,
                                       size_limit,
                                       time_limit,
                                       types_only,
                                       self.auto_escape if auto_escape is None else auto_escape,
                                       self.auto_encode,
                                       self.server.schema if self.server else None,
                                       validator=self.server.custom_validator,
                                       check_names=self.check_names)
            if log_enabled(PROTOCOL):
                log(PROTOCOL, 'SEARCH request <%s> sent via <%s>', search_request_to_dict(request), self)
            response = self.post_send_search(self.send('searchRequest', request, controls))
            self._entries = []

            if isinstance(response, int):  # asynchronous strategy
                return_value = response
                if log_enabled(PROTOCOL):
                    log(PROTOCOL, 'async SEARCH response id <%s> received via <%s>', return_value, self)
            else:
                return_value = True if self.result['type'] == 'searchResDone' and len(response) > 0 else False
                if not return_value and self.result['result'] not in [RESULT_SUCCESS] and not self.last_error:
                    self.last_error = self.result['description']

                if log_enabled(PROTOCOL):
                    for entry in response:
                        if entry['type'] == 'searchResEntry':
                            log(PROTOCOL, 'SEARCH response entry <%s> received via <%s>', entry, self)
                        elif entry['type'] == 'searchResRef':
                            log(PROTOCOL, 'SEARCH response reference <%s> received via <%s>', entry, self)

            if log_enabled(BASIC):
                log(BASIC, 'done SEARCH operation, result <%s>', return_value)

            return self._prepare_return_value(return_value, response=True)

    def compare(self,
                dn,
                attribute,
                value,
                controls=None):
        """
        Perform a compare operation
        """
        conf_attributes_excluded_from_check = [v.lower() for v in get_config_parameter('ATTRIBUTES_EXCLUDED_FROM_CHECK')]

        if log_enabled(BASIC):
            log(BASIC, 'start COMPARE operation via <%s>', self)
        self.last_error = None
        if self.check_names:
            dn = safe_dn(dn)
            if log_enabled(EXTENDED):
                log(EXTENDED, 'dn sanitized to <%s> for COMPARE operation via <%s>', dn, self)

        if self.server and self.server.schema and self.check_names:
            if ';' in attribute:  # remove tags for checking
                attribute_name_to_check = attribute.split(';')[0]
            else:
                attribute_name_to_check = attribute

            if self.server.schema.attribute_types and attribute_name_to_check.lower() not in conf_attributes_excluded_from_check and attribute_name_to_check not in self.server.schema.attribute_types:
                self.last_error = 'invalid attribute type ' + attribute_name_to_check
                if log_enabled(ERROR):
                    log(ERROR, '%s for <%s>', self.last_error, self)
                raise LDAPAttributeError(self.last_error)

        if isinstance(value, SEQUENCE_TYPES):  # value can't be a sequence
            self.last_error = 'value cannot be a sequence'
            if log_enabled(ERROR):
                log(ERROR, '%s for <%s>', self.last_error, self)
            raise LDAPInvalidValueError(self.last_error)

        with self.connection_lock:
            self._fire_deferred()
            request = compare_operation(dn, attribute, value, self.auto_encode, self.server.schema if self.server else None, validator=self.server.custom_validator if self.server else None, check_names=self.check_names)
            if log_enabled(PROTOCOL):
                log(PROTOCOL, 'COMPARE request <%s> sent via <%s>', compare_request_to_dict(request), self)
            response = self.post_send_single_response(self.send('compareRequest', request, controls))
            self._entries = []
            if isinstance(response, int):
                return_value = response
                if log_enabled(PROTOCOL):
                    log(PROTOCOL, 'async COMPARE response id <%s> received via <%s>', return_value, self)
            else:
                return_value = True if self.result['type'] == 'compareResponse' and self.result['result'] == RESULT_COMPARE_TRUE else False
                if not return_value and self.result['result'] not in [RESULT_COMPARE_TRUE, RESULT_COMPARE_FALSE] and not self.last_error:
                    self.last_error = self.result['description']

                if log_enabled(PROTOCOL):
                    log(PROTOCOL, 'COMPARE response <%s> received via <%s>', response, self)

            if log_enabled(BASIC):
                log(BASIC, 'done COMPARE operation, result <%s>', return_value)

            return self._prepare_return_value(return_value)

    def add(self,
            dn,
            object_class=None,
            attributes=None,
            controls=None):
        """
        Add dn to the DIT, object_class is None, a class name or a list
        of class names.

        Attributes is a dictionary in the form 'attr': 'val' or 'attr':
        ['val1', 'val2', ...] for multivalued attributes
        """
        conf_attributes_excluded_from_check = [v.lower() for v in get_config_parameter('ATTRIBUTES_EXCLUDED_FROM_CHECK')]
        conf_classes_excluded_from_check = [v.lower() for v in get_config_parameter('CLASSES_EXCLUDED_FROM_CHECK')]
        if log_enabled(BASIC):
            log(BASIC, 'start ADD operation via <%s>', self)
        self.last_error = None
        _attributes = deepcopy(attributes)  # dict could change when adding objectClass values
        if self.check_names:
            dn = safe_dn(dn)
            if log_enabled(EXTENDED):
                log(EXTENDED, 'dn sanitized to <%s> for ADD operation via <%s>', dn, self)

        with self.connection_lock:
            self._fire_deferred()
            attr_object_class = []
            if object_class is None:
                parm_object_class = []
            else:
                parm_object_class = list(object_class) if isinstance(object_class, SEQUENCE_TYPES) else [object_class]

            object_class_attr_name = ''
            if _attributes:
                for attr in _attributes:
                    if attr.lower() == 'objectclass':
                        object_class_attr_name = attr
                        attr_object_class = list(_attributes[object_class_attr_name]) if isinstance(_attributes[object_class_attr_name], SEQUENCE_TYPES) else [_attributes[object_class_attr_name]]
                        break
            else:
                _attributes = dict()

            if not object_class_attr_name:
                object_class_attr_name = 'objectClass'

            attr_object_class = [to_unicode(object_class) for object_class in attr_object_class]  # converts objectclass to unicode in case of bytes value
            _attributes[object_class_attr_name] = reduce(lambda x, y: x + [y] if y not in x else x, parm_object_class + attr_object_class, [])  # remove duplicate ObjectClasses

            if not _attributes[object_class_attr_name]:
                self.last_error = 'objectClass attribute is mandatory'
                if log_enabled(ERROR):
                    log(ERROR, '%s for <%s>', self.last_error, self)
                raise LDAPObjectClassError(self.last_error)

            if self.server and self.server.schema and self.check_names:
                for object_class_name in _attributes[object_class_attr_name]:
                    if object_class_name.lower() not in conf_classes_excluded_from_check and object_class_name not in self.server.schema.object_classes:
                        self.last_error = 'invalid object class ' + str(object_class_name)
                        if log_enabled(ERROR):
                            log(ERROR, '%s for <%s>', self.last_error, self)
                        raise LDAPObjectClassError(self.last_error)

                for attribute_name in _attributes:
                    if ';' in attribute_name:  # remove tags for checking
                        attribute_name_to_check = attribute_name.split(';')[0]
                    else:
                        attribute_name_to_check = attribute_name

                    if attribute_name_to_check.lower() not in conf_attributes_excluded_from_check and attribute_name_to_check not in self.server.schema.attribute_types:
                        self.last_error = 'invalid attribute type ' + attribute_name_to_check
                        if log_enabled(ERROR):
                            log(ERROR, '%s for <%s>', self.last_error, self)
                        raise LDAPAttributeError(self.last_error)

            request = add_operation(dn, _attributes, self.auto_encode, self.server.schema if self.server else None, validator=self.server.custom_validator if self.server else None, check_names=self.check_names)
            if log_enabled(PROTOCOL):
                log(PROTOCOL, 'ADD request <%s> sent via <%s>', add_request_to_dict(request), self)
            response = self.post_send_single_response(self.send('addRequest', request, controls))
            self._entries = []

            if isinstance(response, STRING_TYPES + (int, )):
                return_value = response
                if log_enabled(PROTOCOL):
                    log(PROTOCOL, 'async ADD response id <%s> received via <%s>', return_value, self)
            else:
                if log_enabled(PROTOCOL):
                    log(PROTOCOL, 'ADD response <%s> received via <%s>', response, self)
                return_value = True if self.result['type'] == 'addResponse' and self.result['result'] == RESULT_SUCCESS else False
                if not return_value and self.result['result'] not in [RESULT_SUCCESS] and not self.last_error:
                    self.last_error = self.result['description']

            if log_enabled(BASIC):
                log(BASIC, 'done ADD operation, result <%s>', return_value)

            return self._prepare_return_value(return_value)

    def delete(self,
               dn,
               controls=None):
        """
        Delete the entry identified by the DN from the DIB.
        """
        if log_enabled(BASIC):
            log(BASIC, 'start DELETE operation via <%s>', self)
        self.last_error = None
        if self.check_names:
            dn = safe_dn(dn)
            if log_enabled(EXTENDED):
                log(EXTENDED, 'dn sanitized to <%s> for DELETE operation via <%s>', dn, self)

        with self.connection_lock:
            self._fire_deferred()
            if self.read_only:
                self.last_error = 'connection is read-only'
                if log_enabled(ERROR):
                    log(ERROR, '%s for <%s>', self.last_error, self)
                raise LDAPConnectionIsReadOnlyError(self.last_error)

            request = delete_operation(dn)
            if log_enabled(PROTOCOL):
                log(PROTOCOL, 'DELETE request <%s> sent via <%s>', delete_request_to_dict(request), self)
            response = self.post_send_single_response(self.send('delRequest', request, controls))
            self._entries = []

            if isinstance(response, STRING_TYPES + (int, )):
                return_value = response
                if log_enabled(PROTOCOL):
                    log(PROTOCOL, 'async DELETE response id <%s> received via <%s>', return_value, self)
            else:
                if log_enabled(PROTOCOL):
                    log(PROTOCOL, 'DELETE response <%s> received via <%s>', response, self)
                return_value = True if self.result['type'] == 'delResponse' and self.result['result'] == RESULT_SUCCESS else False
                if not return_value and self.result['result'] not in [RESULT_SUCCESS] and not self.last_error:
                    self.last_error = self.result['description']

            if log_enabled(BASIC):
                log(BASIC, 'done DELETE operation, result <%s>', return_value)

            return self._prepare_return_value(return_value)

    def modify(self,
               dn,
               changes,
               controls=None):
        """
        Modify attributes of entry

        - changes is a dictionary in the form {'attribute1': change), 'attribute2': [change, change, ...], ...}
        - change is (operation, [value1, value2, ...])
        - operation is 0 (MODIFY_ADD), 1 (MODIFY_DELETE), 2 (MODIFY_REPLACE), 3 (MODIFY_INCREMENT)
        """
        conf_attributes_excluded_from_check = [v.lower() for v in get_config_parameter('ATTRIBUTES_EXCLUDED_FROM_CHECK')]

        if log_enabled(BASIC):
            log(BASIC, 'start MODIFY operation via <%s>', self)
        self.last_error = None
        if self.check_names:
            dn = safe_dn(dn)
            if log_enabled(EXTENDED):
                log(EXTENDED, 'dn sanitized to <%s> for MODIFY operation via <%s>', dn, self)

        with self.connection_lock:
            self._fire_deferred()
            if self.read_only:
                self.last_error = 'connection is read-only'
                if log_enabled(ERROR):
                    log(ERROR, '%s for <%s>', self.last_error, self)
                raise LDAPConnectionIsReadOnlyError(self.last_error)

            if not isinstance(changes, dict):
                self.last_error = 'changes must be a dictionary'
                if log_enabled(ERROR):
                    log(ERROR, '%s for <%s>', self.last_error, self)
                raise LDAPChangeError(self.last_error)

            if not changes:
                self.last_error = 'no changes in modify request'
                if log_enabled(ERROR):
                    log(ERROR, '%s for <%s>', self.last_error, self)
                raise LDAPChangeError(self.last_error)

            changelist = dict()
            for attribute_name in changes:
                if self.server and self.server.schema and self.check_names:
                    if ';' in attribute_name:  # remove tags for checking
                        attribute_name_to_check = attribute_name.split(';')[0]
                    else:
                        attribute_name_to_check = attribute_name

                    if self.server.schema.attribute_types and attribute_name_to_check.lower() not in conf_attributes_excluded_from_check and attribute_name_to_check not in self.server.schema.attribute_types:
                        self.last_error = 'invalid attribute type ' + attribute_name_to_check
                        if log_enabled(ERROR):
                            log(ERROR, '%s for <%s>', self.last_error, self)
                        raise LDAPAttributeError(self.last_error)
                change = changes[attribute_name]
                if isinstance(change, SEQUENCE_TYPES) and change[0] in [MODIFY_ADD, MODIFY_DELETE, MODIFY_REPLACE, MODIFY_INCREMENT, 0, 1, 2, 3]:
                    if len(change) != 2:
                        self.last_error = 'malformed change'
                        if log_enabled(ERROR):
                            log(ERROR, '%s for <%s>', self.last_error, self)
                        raise LDAPChangeError(self.last_error)

                    changelist[attribute_name] = [change]  # insert change in a list
                else:
                    for change_operation in change:
                        if len(change_operation) != 2 or change_operation[0] not in [MODIFY_ADD, MODIFY_DELETE, MODIFY_REPLACE, MODIFY_INCREMENT, 0, 1, 2, 3]:
                            self.last_error = 'invalid change list'
                            if log_enabled(ERROR):
                                log(ERROR, '%s for <%s>', self.last_error, self)
                            raise LDAPChangeError(self.last_error)
                    changelist[attribute_name] = change
            request = modify_operation(dn, changelist, self.auto_encode, self.server.schema if self.server else None, validator=self.server.custom_validator if self.server else None, check_names=self.check_names)
            if log_enabled(PROTOCOL):
                log(PROTOCOL, 'MODIFY request <%s> sent via <%s>', modify_request_to_dict(request), self)
            response = self.post_send_single_response(self.send('modifyRequest', request, controls))
            self._entries = []

            if isinstance(response, STRING_TYPES + (int, )):
                return_value = response
                if log_enabled(PROTOCOL):
                    log(PROTOCOL, 'async MODIFY response id <%s> received via <%s>', return_value, self)
            else:
                if log_enabled(PROTOCOL):
                    log(PROTOCOL, 'MODIFY response <%s> received via <%s>', response, self)
                return_value = True if self.result['type'] == 'modifyResponse' and self.result['result'] == RESULT_SUCCESS else False
                if not return_value and self.result['result'] not in [RESULT_SUCCESS] and not self.last_error:
                    self.last_error = self.result['description']

            if log_enabled(BASIC):
                log(BASIC, 'done MODIFY operation, result <%s>', return_value)

            return self._prepare_return_value(return_value)

    def modify_dn(self,
                  dn,
                  relative_dn,
                  delete_old_dn=True,
                  new_superior=None,
                  controls=None):
        """
        Modify DN of the entry or performs a move of the entry in the
        DIT.
        """
        if log_enabled(BASIC):
            log(BASIC, 'start MODIFY DN operation via <%s>', self)
        self.last_error = None
        if self.check_names:
            dn = safe_dn(dn)
            if log_enabled(EXTENDED):
                log(EXTENDED, 'dn sanitized to <%s> for MODIFY DN operation via <%s>', dn, self)
            relative_dn = safe_dn(relative_dn)
            if log_enabled(EXTENDED):
                log(EXTENDED, 'relative dn sanitized to <%s> for MODIFY DN operation via <%s>', relative_dn, self)

        with self.connection_lock:
            self._fire_deferred()
            if self.read_only:
                self.last_error = 'connection is read-only'
                if log_enabled(ERROR):
                    log(ERROR, '%s for <%s>', self.last_error, self)
                raise LDAPConnectionIsReadOnlyError(self.last_error)

            # if new_superior and not dn.startswith(relative_dn):  # as per RFC4511 (4.9)
            #     self.last_error = 'DN cannot change while performing moving'
            #     if log_enabled(ERROR):
            #         log(ERROR, '%s for <%s>', self.last_error, self)
            #     raise LDAPChangeError(self.last_error)

            request = modify_dn_operation(dn, relative_dn, delete_old_dn, new_superior)
            if log_enabled(PROTOCOL):
                log(PROTOCOL, 'MODIFY DN request <%s> sent via <%s>', modify_dn_request_to_dict(request), self)
            response = self.post_send_single_response(self.send('modDNRequest', request, controls))
            self._entries = []

            if isinstance(response, STRING_TYPES + (int, )):
                return_value = response
                if log_enabled(PROTOCOL):
                    log(PROTOCOL, 'async MODIFY DN response id <%s> received via <%s>', return_value, self)
            else:
                if log_enabled(PROTOCOL):
                    log(PROTOCOL, 'MODIFY DN response <%s> received via <%s>', response, self)
                return_value = True if self.result['type'] == 'modDNResponse' and self.result['result'] == RESULT_SUCCESS else False
                if not return_value and self.result['result'] not in [RESULT_SUCCESS] and not self.last_error:
                    self.last_error = self.result['description']

            if log_enabled(BASIC):
                log(BASIC, 'done MODIFY DN operation, result <%s>', return_value)

            return self._prepare_return_value(return_value)

    def abandon(self,
                message_id,
                controls=None):
        """
        Abandon the operation indicated by message_id
        """
        if log_enabled(BASIC):
            log(BASIC, 'start ABANDON operation via <%s>', self)
        self.last_error = None
        with self.connection_lock:
            self._fire_deferred()
            return_value = False
            if self.strategy._outstanding or message_id == 0:
                # only current  operation should be abandoned, abandon, bind and unbind cannot ever be abandoned,
                # messagiId 0 is invalid and should be used as a "ping" to keep alive the connection
                if (self.strategy._outstanding and message_id in self.strategy._outstanding and self.strategy._outstanding[message_id]['type'] not in ['abandonRequest', 'bindRequest', 'unbindRequest']) or message_id == 0:
                    request = abandon_operation(message_id)
                    if log_enabled(PROTOCOL):
                        log(PROTOCOL, 'ABANDON request: <%s> sent via <%s>', abandon_request_to_dict(request), self)
                    self.send('abandonRequest', request, controls)
                    self.result = None
                    self.response = None
                    self._entries = []
                    return_value = True
                else:
                    if log_enabled(ERROR):
                        log(ERROR, 'cannot abandon a Bind, an Unbind or an Abandon operation or message ID %s not found via <%s>', str(message_id), self)

            if log_enabled(BASIC):
                log(BASIC, 'done ABANDON operation, result <%s>', return_value)

            return self._prepare_return_value(return_value)

    def extended(self,
                 request_name,
                 request_value=None,
                 controls=None,
                 no_encode=None):
        """
        Performs an extended operation
        """
        if log_enabled(BASIC):
            log(BASIC, 'start EXTENDED operation via <%s>', self)
        self.last_error = None
        with self.connection_lock:
            self._fire_deferred()
            request = extended_operation(request_name, request_value, no_encode=no_encode)
            if log_enabled(PROTOCOL):
                log(PROTOCOL, 'EXTENDED request <%s> sent via <%s>', extended_request_to_dict(request), self)
            response = self.post_send_single_response(self.send('extendedReq', request, controls))
            self._entries = []
            if isinstance(response, int):
                return_value = response
                if log_enabled(PROTOCOL):
                    log(PROTOCOL, 'async EXTENDED response id <%s> received via <%s>', return_value, self)
            else:
                if log_enabled(PROTOCOL):
                    log(PROTOCOL, 'EXTENDED response <%s> received via <%s>', response, self)
                return_value = True if self.result['type'] == 'extendedResp' and self.result['result'] == RESULT_SUCCESS else False
                if not return_value and self.result['result'] not in [RESULT_SUCCESS] and not self.last_error:
                    self.last_error = self.result['description']

            if log_enabled(BASIC):
                log(BASIC, 'done EXTENDED operation, result <%s>', return_value)

            return self._prepare_return_value(return_value, response=True)

    def start_tls(self, read_server_info=True):  # as per RFC4511. Removal of TLS is defined as MAY in RFC4511 so the client can't implement a generic stop_tls method0
        if log_enabled(BASIC):
            log(BASIC, 'start START TLS operation via <%s>', self)

        with self.connection_lock:
            return_value = False
            self.result = None

            if not self.server.tls:
                self.server.tls = Tls()

            if self.lazy and not self._executing_deferred:
                self._deferred_start_tls = True
                self.tls_started = True
                return_value = True
                if log_enabled(BASIC):
                    log(BASIC, 'deferring START TLS for <%s>', self)
            else:
                self._deferred_start_tls = False
                if self.closed:
                    self.open()
                if self.server.tls.start_tls(self) and self.strategy.sync:  # for asynchronous connections _start_tls is run by the strategy
                    if read_server_info:
                        self.refresh_server_info()  # refresh server info as per RFC4515 (3.1.5)
                    return_value = True
                elif not self.strategy.sync:
                    return_value = True

            if log_enabled(BASIC):
                log(BASIC, 'done START TLS operation, result <%s>', return_value)

            return self._prepare_return_value(return_value)

    def do_sasl_bind(self,
                     controls):
        if log_enabled(BASIC):
            log(BASIC, 'start SASL BIND operation via <%s>', self)
        self.last_error = None
        with self.connection_lock:
            result = None

            if not self.sasl_in_progress:
                self.sasl_in_progress = True
                try:
                    if self.sasl_mechanism == EXTERNAL:
                        result = sasl_external(self, controls)
                    elif self.sasl_mechanism == DIGEST_MD5:
                        result = sasl_digest_md5(self, controls)
                    elif self.sasl_mechanism == GSSAPI:
                        from ..protocol.sasl.kerberos import sasl_gssapi  # needs the gssapi package
                        result = sasl_gssapi(self, controls)
                    elif self.sasl_mechanism == 'PLAIN':
                        result = sasl_plain(self, controls)
                finally:
                    self.sasl_in_progress = False

            if log_enabled(BASIC):
                log(BASIC, 'done SASL BIND operation, result <%s>', result)

            return result

    def do_ntlm_bind(self,
                     controls):
        if log_enabled(BASIC):
            log(BASIC, 'start NTLM BIND operation via <%s>', self)
        self.last_error = None
        with self.connection_lock:
            if not self.sasl_in_progress:
                self.sasl_in_progress = True  # ntlm is same of sasl authentication
                try:
                    # additional import for NTLM
                    from ..utils.ntlm import NtlmClient
                    domain_name, user_name = self.user.split('\\', 1)
                    ntlm_client = NtlmClient(user_name=user_name, domain=domain_name, password=self.password)

                    # as per https://msdn.microsoft.com/en-us/library/cc223501.aspx
                    # send a sicilyPackageDiscovery request (in the bindRequest)
                    request = bind_operation(self.version, 'SICILY_PACKAGE_DISCOVERY', ntlm_client)
                    if log_enabled(PROTOCOL):
                        log(PROTOCOL, 'NTLM SICILY PACKAGE DISCOVERY request sent via <%s>', self)
                    response = self.post_send_single_response(self.send('bindRequest', request, controls))
                    if not self.strategy.sync:
                        _, result = self.get_response(response)
                    else:
                        result = response[0]
                    if 'server_creds' in result:
                        sicily_packages = result['server_creds'].decode('ascii').split(';')
                        if 'NTLM' in sicily_packages:  # NTLM available on server
                            request = bind_operation(self.version, 'SICILY_NEGOTIATE_NTLM', ntlm_client)
                            if log_enabled(PROTOCOL):
                                log(PROTOCOL, 'NTLM SICILY NEGOTIATE request sent via <%s>', self)
                            response = self.post_send_single_response(self.send('bindRequest', request, controls))
                            if not self.strategy.sync:
                                _, result = self.get_response(response)
                            else:
                                if log_enabled(PROTOCOL):
                                    log(PROTOCOL, 'NTLM SICILY NEGOTIATE response <%s> received via <%s>', response[0],
                                        self)
                                result = response[0]

                            if result['result'] == RESULT_SUCCESS:
                                request = bind_operation(self.version, 'SICILY_RESPONSE_NTLM', ntlm_client,
                                                         result['server_creds'])
                                if log_enabled(PROTOCOL):
                                    log(PROTOCOL, 'NTLM SICILY RESPONSE NTLM request sent via <%s>', self)
                                response = self.post_send_single_response(self.send('bindRequest', request, controls))
                                if not self.strategy.sync:
                                    _, result = self.get_response(response)
                                else:
                                    if log_enabled(PROTOCOL):
                                        log(PROTOCOL, 'NTLM BIND response <%s> received via <%s>', response[0], self)
                                    result = response[0]
                    else:
                        result = None
                finally:
                    self.sasl_in_progress = False

                if log_enabled(BASIC):
                    log(BASIC, 'done SASL NTLM operation, result <%s>', result)

                return result

    def refresh_server_info(self):
        # if self.strategy.no_real_dsa:  # do not refresh for mock strategies
        #     return

        if not self.strategy.pooled:
            with self.connection_lock:
                if not self.closed:
                    if log_enabled(BASIC):
                        log(BASIC, 'refreshing server info for <%s>', self)
                    previous_response = self.response
                    previous_result = self.result
                    previous_entries = self._entries
                    self.server.get_info_from_server(self)
                    self.response = previous_response
                    self.result = previous_result
                    self._entries = previous_entries
        else:
            if log_enabled(BASIC):
                log(BASIC, 'refreshing server info from pool for <%s>', self)
            self.strategy.pool.get_info_from_server()

    def response_to_ldif(self,
                         search_result=None,
                         all_base64=False,
                         line_separator=None,
                         sort_order=None,
                         stream=None):
        with self.connection_lock:
            if search_result is None:
                search_result = self.response

            if isinstance(search_result, SEQUENCE_TYPES):
                ldif_lines = operation_to_ldif('searchResponse', search_result, all_base64, sort_order=sort_order)
                ldif_lines = add_ldif_header(ldif_lines)
                line_separator = line_separator or linesep
                ldif_output = line_separator.join(ldif_lines)
                if stream:
                    if stream.tell() == 0:
                        header = add_ldif_header(['-'])[0]
                        stream.write(prepare_for_stream(header + line_separator + line_separator))
                    stream.write(prepare_for_stream(ldif_output + line_separator + line_separator))
                if log_enabled(BASIC):
                    log(BASIC, 'building LDIF output <%s> for <%s>', ldif_output, self)
                return ldif_output

            return None

    def response_to_json(self,
                         raw=False,
                         search_result=None,
                         indent=4,
                         sort=True,
                         stream=None,
                         checked_attributes=True,
                         include_empty=True):

        with self.connection_lock:
            if search_result is None:
                search_result = self.response

            if isinstance(search_result, SEQUENCE_TYPES):
                json_dict = dict()
                json_dict['entries'] = []

                for response in search_result:
                    if response['type'] == 'searchResEntry':
                        entry = dict()

                        entry['dn'] = response['dn']
                        if checked_attributes:
                            if not include_empty:
                                # needed for python 2.6 compatibility
                                entry['attributes'] = dict((key, response['attributes'][key]) for key in response['attributes'] if response['attributes'][key])
                            else:
                                entry['attributes'] = dict(response['attributes'])
                        if raw:
                            if not include_empty:
                                # needed for python 2.6 compatibility
                                entry['raw_attributes'] = dict((key, response['raw_attributes'][key]) for key in response['raw_attributes'] if response['raw:attributes'][key])
                            else:
                                entry['raw'] = dict(response['raw_attributes'])
                        json_dict['entries'].append(entry)

                if str is bytes:  # Python 2
                    check_json_dict(json_dict)

                json_output = json.dumps(json_dict, ensure_ascii=True, sort_keys=sort, indent=indent, check_circular=True, default=format_json, separators=(',', ': '))

                if log_enabled(BASIC):
                    log(BASIC, 'building JSON output <%s> for <%s>', json_output, self)
                if stream:
                    stream.write(json_output)

                return json_output

    def response_to_file(self,
                         target,
                         raw=False,
                         indent=4,
                         sort=True):
        with self.connection_lock:
            if self.response:
                if isinstance(target, STRING_TYPES):
                    target = open(target, 'w+')

                if log_enabled(BASIC):
                    log(BASIC, 'writing response to file for <%s>', self)

                target.writelines(self.response_to_json(raw=raw, indent=indent, sort=sort))
                target.close()

    def _fire_deferred(self, read_info=None):
        # if read_info is None reads the schema and server info if not present, if False doesn't read server info, if True reads always server info
        with self.connection_lock:
            if self.lazy and not self._executing_deferred:
                self._executing_deferred = True

                if log_enabled(BASIC):
                    log(BASIC, 'executing deferred (open: %s, start_tls: %s, bind: %s) for <%s>', self._deferred_open, self._deferred_start_tls, self._deferred_bind, self)
                try:
                    if self._deferred_open:
                        self.open(read_server_info=False)
                    if self._deferred_start_tls:
                        if not self.start_tls(read_server_info=False):
                            error = 'deferred start_tls not successful' + (' - ' + self.last_error if self.last_error else '')
                            if log_enabled(ERROR):
                                log(ERROR, '%s for <%s>', error, self)
                            self.unbind()
                            raise LDAPStartTLSError(error)
                    if self._deferred_bind:
                        self.bind(read_server_info=False, controls=self._bind_controls)
                    if (read_info is None and (not self.server.info and self.server.get_info in [DSA, ALL]) or (not self.server.schema and self.server.get_info in [SCHEMA, ALL])) or read_info:
                        self.refresh_server_info()
                except LDAPExceptionError as e:
                    if log_enabled(ERROR):
                        log(ERROR, '%s for <%s>', e, self)
                    raise  # re-raise LDAPExceptionError
                finally:
                    self._executing_deferred = False

    @property
    def entries(self):
        if self.response:
            if not self._entries:
                self._entries = self._get_entries(self.response, self.request)
        return self._entries

    def _get_entries(self, search_response, search_request):
        with self.connection_lock:
            from .. import ObjectDef, Reader

            # build a table of ObjectDefs, grouping the entries found in search_response for their attributes set, subset will be included in superset
            attr_sets = []
            for response in search_response:
                if response['type'] == 'searchResEntry':
                    resp_attr_set = set(response['attributes'].keys())
                    if resp_attr_set not in attr_sets:
                        attr_sets.append(resp_attr_set)
            attr_sets.sort(key=lambda x: -len(x))  # sorts the list in descending length order
            unique_attr_sets = []
            for attr_set in attr_sets:
                for unique_set in unique_attr_sets:
                    if unique_set >= attr_set:  # checks if unique set is a superset of attr_set
                        break
                else:  # the attr_set is not a subset of any element in unique_attr_sets
                    unique_attr_sets.append(attr_set)
            object_defs = []
            for attr_set in unique_attr_sets:
                object_def = ObjectDef(schema=self.server.schema)
                object_def += list(attr_set)  # converts the set in a list to be added to the object definition
                object_defs.append((attr_set,
                                    object_def,
                                    Reader(self, object_def, search_request['base'], search_request['filter'], attributes=attr_set) if self.strategy.sync else Reader(self, object_def, '', '', attributes=attr_set))
                                   )  # objects_defs contains a tuple with the set, the ObjectDef and a cursor

            entries = []
            for response in search_response:
                if response['type'] == 'searchResEntry':
                    resp_attr_set = set(response['attributes'].keys())
                    for object_def in object_defs:
                        if resp_attr_set <= object_def[0]:  # finds the ObjectDef for the attribute set of this entry
                            entry = object_def[2]._create_entry(response)
                            entries.append(entry)
                            break
                    else:
                        self.last_error = 'attribute set not found for ' + str(resp_attr_set)
                        if log_enabled(ERROR):
                            log(ERROR, self.last_error, self)
                        raise LDAPObjectError(self.last_error)

        return entries
