"""
"""

# Created on 2013.08.05
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

from .exceptions import LDAPSSLNotSupportedError, LDAPSSLConfigurationError, LDAPCertificateError, start_tls_exception_factory, LDAPStartTLSError
from .. import SEQUENCE_TYPES
from ..utils.log import log, log_enabled, ERROR, BASIC, NETWORK

try:
    # noinspection PyUnresolvedReferences
    import ssl
except ImportError:
    if log_enabled(ERROR):
        log(ERROR, 'SSL not supported in this Python interpreter')
    raise LDAPSSLNotSupportedError('SSL not supported in this Python interpreter')

try:
    from ssl import match_hostname, CertificateError  # backport for python2 missing ssl functionalities
except ImportError:
    from ..utils.tls_backport import CertificateError
    from ..utils.tls_backport import match_hostname
    if log_enabled(BASIC):
        log(BASIC, 'using tls_backport')

try:  # try to use SSLContext
    # noinspection PyUnresolvedReferences
    from ssl import create_default_context, Purpose  # defined in Python 3.4 and Python 2.7.9
    use_ssl_context = True
except ImportError:
    use_ssl_context = False
    if log_enabled(BASIC):
        log(BASIC, 'SSLContext unavailable')

from os import path


# noinspection PyProtectedMember
class Tls(object):
    """
    tls/ssl configuration for Server object
    Starting from python 2.7.9 and python 3.4 uses the SSLContext object
    that tries to read the CAs defined at system level
    ca_certs_path and ca_certs_data are valid only when using SSLContext
    local_private_key_password is valid only when using SSLContext
    ssl_options is valid only when using SSLContext
    sni is the server name for Server Name Indication (when available)
    """

    def __init__(self,
                 local_private_key_file=None,
                 local_certificate_file=None,
                 validate=ssl.CERT_NONE,
                 version=None,
                 ssl_options=None,
                 ca_certs_file=None,
                 valid_names=None,
                 ca_certs_path=None,
                 ca_certs_data=None,
                 local_private_key_password=None,
                 ciphers=None,
                 sni=None):
        if ssl_options is None:
            ssl_options = []
        self.ssl_options = ssl_options
        if validate in [ssl.CERT_NONE, ssl.CERT_OPTIONAL, ssl.CERT_REQUIRED]:
            self.validate = validate
        elif validate:
            if log_enabled(ERROR):
                log(ERROR, 'invalid validate parameter <%s>', validate)
            raise LDAPSSLConfigurationError('invalid validate parameter')
        if ca_certs_file and path.exists(ca_certs_file):
            self.ca_certs_file = ca_certs_file
        elif ca_certs_file:
            if log_enabled(ERROR):
                log(ERROR, 'invalid CA public key file <%s>', ca_certs_file)
            raise LDAPSSLConfigurationError('invalid CA public key file')
        else:
            self.ca_certs_file = None

        if ca_certs_path and use_ssl_context and path.exists(ca_certs_path):
            self.ca_certs_path = ca_certs_path
        elif ca_certs_path and not use_ssl_context:
            if log_enabled(ERROR):
                log(ERROR, 'cannot use CA public keys path, SSLContext not available')
            raise LDAPSSLNotSupportedError('cannot use CA public keys path, SSLContext not available')
        elif ca_certs_path:
            if log_enabled(ERROR):
                log(ERROR, 'invalid CA public keys path <%s>', ca_certs_path)
            raise LDAPSSLConfigurationError('invalid CA public keys path')
        else:
            self.ca_certs_path = None

        if ca_certs_data and use_ssl_context:
            self.ca_certs_data = ca_certs_data
        elif ca_certs_data:
            if log_enabled(ERROR):
                log(ERROR, 'cannot use CA data, SSLContext not available')
            raise LDAPSSLNotSupportedError('cannot use CA data, SSLContext not available')
        else:
            self.ca_certs_data = None

        if local_private_key_password and use_ssl_context:
            self.private_key_password = local_private_key_password
        elif local_private_key_password:
            if log_enabled(ERROR):
                log(ERROR, 'cannot use local private key password, SSLContext not available')
            raise LDAPSSLNotSupportedError('cannot use local private key password, SSLContext is not available')
        else:
            self.private_key_password = None

        self.version = version
        self.private_key_file = local_private_key_file
        self.certificate_file = local_certificate_file
        self.valid_names = valid_names
        self.ciphers = ciphers
        self.sni = sni

        if log_enabled(BASIC):
            log(BASIC, 'instantiated Tls: <%r>' % self)

    def __str__(self):
        s = [
            'protocol: ' + str(self.version),
            'client private key: ' + ('present ' if self.private_key_file else 'not present'),
            'client certificate: ' + ('present ' if self.certificate_file else 'not present'),
            'private key password: ' + ('present ' if self.private_key_password else 'not present'),
            'CA certificates file: ' + ('present ' if self.ca_certs_file else 'not present'),
            'CA certificates path: ' + ('present ' if self.ca_certs_path else 'not present'),
            'CA certificates data: ' + ('present ' if self.ca_certs_data else 'not present'),
            'verify mode: ' + str(self.validate),
            'valid names: ' + str(self.valid_names),
            'ciphers: ' + str(self.ciphers),
            'sni: ' + str(self.sni)
        ]
        return ' - '.join(s)

    def __repr__(self):
        r = '' if self.private_key_file is None else ', local_private_key_file={0.private_key_file!r}'.format(self)
        r += '' if self.certificate_file is None else ', local_certificate_file={0.certificate_file!r}'.format(self)
        r += '' if self.validate is None else ', validate={0.validate!r}'.format(self)
        r += '' if self.version is None else ', version={0.version!r}'.format(self)
        r += '' if self.ca_certs_file is None else ', ca_certs_file={0.ca_certs_file!r}'.format(self)
        r += '' if self.ca_certs_path is None else ', ca_certs_path={0.ca_certs_path!r}'.format(self)
        r += '' if self.ca_certs_data is None else ', ca_certs_data={0.ca_certs_data!r}'.format(self)
        r += '' if self.ciphers is None else ', ciphers={0.ciphers!r}'.format(self)
        r += '' if self.sni is None else ', sni={0.sni!r}'.format(self)
        r = 'Tls(' + r[2:] + ')'
        return r

    def wrap_socket(self, connection, do_handshake=False):
        """
        Adds TLS to the connection socket
        """
        if use_ssl_context:
            if self.version is None:  # uses the default ssl context for reasonable security
                ssl_context = create_default_context(purpose=Purpose.SERVER_AUTH,
                                                     cafile=self.ca_certs_file,
                                                     capath=self.ca_certs_path,
                                                     cadata=self.ca_certs_data)
            else:  # code from create_default_context in the Python standard library 3.5.1, creates a ssl context with the specificd protocol version
                ssl_context = ssl.SSLContext(self.version)
                if self.ca_certs_file or self.ca_certs_path or self.ca_certs_data:
                    ssl_context.load_verify_locations(self.ca_certs_file, self.ca_certs_path, self.ca_certs_data)
                elif self.validate != ssl.CERT_NONE:
                    ssl_context.load_default_certs(Purpose.SERVER_AUTH)

            if self.certificate_file:
                ssl_context.load_cert_chain(self.certificate_file, keyfile=self.private_key_file, password=self.private_key_password)
            ssl_context.check_hostname = False
            ssl_context.verify_mode = self.validate
            for option in self.ssl_options:
                ssl_context.options |= option

            if self.ciphers:
                try:
                    ssl_context.set_ciphers(self.ciphers)
                except ssl.SSLError:
                    pass

            if self.sni:
                wrapped_socket = ssl_context.wrap_socket(connection.socket, server_side=False, do_handshake_on_connect=do_handshake, server_hostname=self.sni)
            else:
                wrapped_socket = ssl_context.wrap_socket(connection.socket, server_side=False, do_handshake_on_connect=do_handshake)
            if log_enabled(NETWORK):
                log(NETWORK, 'socket wrapped with SSL using SSLContext for <%s>', connection)
        else:
            if self.version is None and hasattr(ssl, 'PROTOCOL_SSLv23'):
                self.version = ssl.PROTOCOL_SSLv23
            if self.ciphers:
                try:

                    wrapped_socket = ssl.wrap_socket(connection.socket,
                                                     keyfile=self.private_key_file,
                                                     certfile=self.certificate_file,
                                                     server_side=False,
                                                     cert_reqs=self.validate,
                                                     ssl_version=self.version,
                                                     ca_certs=self.ca_certs_file,
                                                     do_handshake_on_connect=do_handshake,
                                                     ciphers=self.ciphers)
                except ssl.SSLError:
                    raise
                except TypeError:  # in python2.6 no ciphers argument is present, failback to self.ciphers=None
                    self.ciphers = None

            if not self.ciphers:
                wrapped_socket = ssl.wrap_socket(connection.socket,
                                                 keyfile=self.private_key_file,
                                                 certfile=self.certificate_file,
                                                 server_side=False,
                                                 cert_reqs=self.validate,
                                                 ssl_version=self.version,
                                                 ca_certs=self.ca_certs_file,
                                                 do_handshake_on_connect=do_handshake)
            if log_enabled(NETWORK):
                log(NETWORK, 'socket wrapped with SSL for <%s>', connection)

        if do_handshake and (self.validate == ssl.CERT_REQUIRED or self.validate == ssl.CERT_OPTIONAL):
            check_hostname(wrapped_socket, connection.server.host, self.valid_names)

        connection.socket = wrapped_socket
        return

    def start_tls(self, connection):
        if connection.server.ssl:  # ssl already established at server level
            return False

        if (connection.tls_started and not connection._executing_deferred) or connection.strategy._outstanding or connection.sasl_in_progress:
            # Per RFC 4513 (3.1.1)
            if log_enabled(ERROR):
                log(ERROR, "can't start tls because operations are in progress for <%s>", self)
            return False
        connection.starting_tls = True
        if log_enabled(BASIC):
            log(BASIC, 'starting tls for <%s>', connection)
        if not connection.strategy.sync:
            connection._awaiting_for_async_start_tls = True  # some flaky servers (OpenLDAP) doesn't return the extended response name in response
        result = connection.extended('1.3.6.1.4.1.1466.20037')
        if not connection.strategy.sync:
            # asynchronous - _start_tls must be executed by the strategy
            response = connection.get_response(result)
            if response != (None, None):
                if log_enabled(BASIC):
                    log(BASIC, 'tls started for <%s>', connection)
                return True
            else:
                if log_enabled(BASIC):
                    log(BASIC, 'tls not started for <%s>', connection)
                return False
        else:
            if connection.result['description'] not in ['success']:
                # startTLS failed
                connection.last_error = 'startTLS failed - ' + str(connection.result['description'])
                if log_enabled(ERROR):
                    log(ERROR, '%s for <%s>', connection.last_error, connection)
                raise LDAPStartTLSError(connection.last_error)
            if log_enabled(BASIC):
                log(BASIC, 'tls started for <%s>', connection)
            return self._start_tls(connection)

    def _start_tls(self, connection):
        try:
            self.wrap_socket(connection, do_handshake=True)
        except Exception as e:
            connection.last_error = 'wrap socket error: ' + str(e)
            if log_enabled(ERROR):
                log(ERROR, 'error <%s> wrapping socket for TLS in <%s>', connection.last_error, connection)
            raise start_tls_exception_factory(e)(connection.last_error)
        finally:
            connection.starting_tls = False

        if connection.usage:
            connection._usage.wrapped_sockets += 1
        connection.tls_started = True
        return True


def check_hostname(sock, server_name, additional_names):
    server_certificate = sock.getpeercert()
    if log_enabled(NETWORK):
        log(NETWORK, 'certificate found for %s: %s', sock, server_certificate)
    if additional_names:
        host_names = [server_name] + (additional_names if isinstance(additional_names, SEQUENCE_TYPES) else [additional_names])
    else:
        host_names = [server_name]

    for host_name in host_names:
        if not host_name:
            continue
        elif host_name == '*':
            if log_enabled(NETWORK):
                log(NETWORK, 'certificate matches * wildcard')
            return  # valid

        try:
            match_hostname(server_certificate, host_name)  # raise CertificateError if certificate doesn't match server name
            if log_enabled(NETWORK):
                log(NETWORK, 'certificate matches host name <%s>', host_name)
            return  # valid
        except CertificateError as e:
            if log_enabled(NETWORK):
                log(NETWORK, str(e))

    if log_enabled(ERROR):
        log(ERROR, "hostname doesn't match certificate")
    raise LDAPCertificateError("certificate %s doesn't match any name in %s " % (server_certificate, str(host_names)))
