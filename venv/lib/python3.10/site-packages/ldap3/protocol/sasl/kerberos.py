"""
"""

# Created on 2015.04.08
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

# original code by Hugh Cole-Baker, modified by Peter Foley, modified again by Azaria Zornberg
# it needs the gssapi package
import base64
import socket

from ...core.exceptions import LDAPPackageUnavailableError, LDAPCommunicationError
from ...core.rdns import ReverseDnsSetting, get_hostname_by_addr, is_ip_addr

posix_gssapi_unavailable = True
try:
    # noinspection PyPackageRequirements,PyUnresolvedReferences
    import gssapi
    from gssapi.raw import ChannelBindings
    posix_gssapi_unavailable = False
except ImportError:
    pass

windows_gssapi_unavailable = True
# only attempt to import winkerberos if gssapi is unavailable
if posix_gssapi_unavailable:
    try:
        import winkerberos
        windows_gssapi_unavailable = False
    except ImportError:
        raise LDAPPackageUnavailableError('package gssapi (or winkerberos) missing')

from .sasl import send_sasl_negotiation, abort_sasl_negotiation


NO_SECURITY_LAYER = 1
INTEGRITY_PROTECTION = 2
CONFIDENTIALITY_PROTECTION = 4


def get_channel_bindings(ssl_socket):
    try:
        server_certificate = ssl_socket.getpeercert(True)
    except:
        # it is not SSL socket
        return None
    try:
        from cryptography import x509
        from cryptography.hazmat.backends import default_backend
        from cryptography.hazmat.primitives import hashes
    except ImportError:
        raise LDAPPackageUnavailableError('package cryptography missing')
    cert = x509.load_der_x509_certificate(server_certificate, default_backend())
    hash_algorithm = cert.signature_hash_algorithm
    # According to https://tools.ietf.org/html/rfc5929#section-4.1, we have to convert the the hash function for md5 and sha1
    if hash_algorithm.name in ('md5', 'sha1'):
        digest = hashes.Hash(hashes.SHA256(), default_backend())
    else:
        digest = hashes.Hash(hash_algorithm, default_backend())
    digest.update(server_certificate)
    application_data = b'tls-server-end-point:' + digest.finalize()
    # posix gssapi and windows winkerberos use different channel bindings classes
    if not posix_gssapi_unavailable:
        return ChannelBindings(application_data=application_data)
    else:
        return winkerberos.channelBindings(application_data=application_data)


def sasl_gssapi(connection, controls):
    """
    Performs a bind using the Kerberos v5 ("GSSAPI") SASL mechanism
    from RFC 4752. Does not support any security layers, only authentication!

    sasl_credentials can be empty or a tuple with one or two elements.
    The first element determines which service principal to request a ticket for and can be one of the following:

    - None or False, to use the hostname from the Server object
    - True to perform a reverse DNS lookup to retrieve the canonical hostname for the hosts IP address
    - A string containing the hostname

    The optional second element is what authorization ID to request.

    - If omitted or None, the authentication ID is used as the authorization ID
    - If a string, the authorization ID to use. Should start with "dn:" or "user:".

    The optional third element is a raw gssapi credentials structure which can be used over
    the implicit use of a krb ccache.
    """
    if not posix_gssapi_unavailable:
        return _posix_sasl_gssapi(connection, controls)
    else:
        return _windows_sasl_gssapi(connection, controls)


def _common_determine_target_name(connection):
    """ Common logic for determining our target name for kerberos negotiation, regardless of whether we are using
    gssapi on a posix system or winkerberos on windows.
    Returns a string in the form "ldap@" + a target hostname.
    The hostname can either be user specified, come from the connection, or resolved using reverse DNS based
    on parameters set in sasl_credentials.
    The default if no sasl_credentials are specified is to use the host in the connection server object.
    """
    # if we don't have any sasl_credentials specified, or the first entry is False (which is the legacy equivalent
    # to ReverseDnsSetting.OFF that has value 0) then our gssapi name is just
    if (not connection.sasl_credentials or len(connection.sasl_credentials) == 0
            or not connection.sasl_credentials[0]):
        return 'ldap@' + connection.server.host
    # older code will still be using a boolean True for the equivalent of
    # ReverseDnsSetting.REQUIRE_RESOLVE_ALL_ADDRESSES
    if connection.sasl_credentials[0] is True:
        hostname = get_hostname_by_addr(connection.socket.getpeername()[0])
        target_name = 'ldap@' + hostname
    elif connection.sasl_credentials[0] in ReverseDnsSetting.SUPPORTED_VALUES:
        rdns_setting = connection.sasl_credentials[0]
        # if the rdns_setting is OFF then we won't enter any branch here and will leave hostname as server host,
        # so we'll just use the server host, whatever it is
        peer_ip = connection.socket.getpeername()[0]
        hostname = connection.server.host
        if rdns_setting == ReverseDnsSetting.REQUIRE_RESOLVE_ALL_ADDRESSES:
            # resolve our peer ip and use it as our target name
            hostname = get_hostname_by_addr(peer_ip)
        elif rdns_setting == ReverseDnsSetting.REQUIRE_RESOLVE_IP_ADDRESSES_ONLY:
            # resolve our peer ip (if the server host is an ip address) and use it as our target name
            if is_ip_addr(hostname):
                hostname = get_hostname_by_addr(peer_ip)
        elif rdns_setting == ReverseDnsSetting.OPTIONAL_RESOLVE_ALL_ADDRESSES:
            # try to resolve our peer ip in dns and if we can, use it as our target name.
            # if not, just use the server host
            resolved_hostname = get_hostname_by_addr(peer_ip, success_required=False)
            if resolved_hostname is not None:
                hostname = resolved_hostname
        elif rdns_setting == ReverseDnsSetting.OPTIONAL_RESOLVE_IP_ADDRESSES_ONLY:
            # try to resolve our peer ip in dns if our server host is an ip. if we can, use it as our target
            #  name. if not, just use the server host
            if is_ip_addr(hostname):
                resolved_hostname = get_hostname_by_addr(peer_ip, success_required=False)
                if resolved_hostname is not None:
                    hostname = resolved_hostname
        # construct our target name
        target_name = 'ldap@' + hostname
    else:  # string hostname directly provided
        target_name = 'ldap@' + connection.sasl_credentials[0]
    return target_name


def _common_determine_authz_id_and_creds(connection):
    """ Given our connection, figure out the authorization id (i.e. the kerberos principal) and kerberos credentials
    being used for our SASL bind.
    On posix systems, we can actively negotiate with raw credentials and receive a kerberos client ticket during our
    SASL bind. So credentials can be specified.
    However, on windows systems, winkerberos expects to use the credentials cached by the system because windows
    machines are generally domain-joined. So no initiation is supported, as the TGT must already be valid prior
    to beginning the SASL bind, and the windows system credentials will be used as needed with that.
    """
    authz_id = b""
    creds = None
    # the default value for credentials is only something we should instantiate on systems using the
    # posix GSSAPI, as windows kerberos expects that a tgt already exists (i.e. the machine is domain-joined)
    # and does not support initiation from raw credentials
    if not posix_gssapi_unavailable:
        creds = gssapi.Credentials(name=gssapi.Name(connection.user), usage='initiate', store=connection.cred_store) if connection.user else None
    if connection.sasl_credentials:
        if len(connection.sasl_credentials) >= 2 and connection.sasl_credentials[1]:
            authz_id = connection.sasl_credentials[1].encode("utf-8")
        if len(connection.sasl_credentials) >= 3 and connection.sasl_credentials[2]:
            if posix_gssapi_unavailable:
                raise LDAPPackageUnavailableError('The winkerberos package does not support specifying raw  credentials'
                                                  'to initiate GSSAPI Kerberos communication. A ticket granting ticket '
                                                  'must have already been obtained for the user before beginning a '
                                                  'SASL bind.')
            raw_creds = connection.sasl_credentials[2]
            creds = gssapi.Credentials(base=raw_creds, usage='initiate', store=connection.cred_store)
    return authz_id, creds


def _common_process_end_token_get_security_layers(negotiated_token):
    """ Process the response we got at the end of our SASL negotiation wherein the server told us what
    minimum security layers we need, and return a bytearray for the client security layers we want.
    This function throws an error on a malformed token from the server.
    The ldap3 library does not support security layers, and only supports authentication with kerberos,
    so an error will be thrown for any tokens that indicate a security layer requirement.
    """
    if len(negotiated_token) != 4:
        raise LDAPCommunicationError("Incorrect response from server")

    server_security_layers = negotiated_token[0]
    if not isinstance(server_security_layers, int):
        server_security_layers = ord(server_security_layers)
    if server_security_layers in (0, NO_SECURITY_LAYER):
        if negotiated_token[1:] != '\x00\x00\x00':
            raise LDAPCommunicationError("Server max buffer size must be 0 if no security layer")
    if not (server_security_layers & NO_SECURITY_LAYER):
        raise LDAPCommunicationError("Server requires a security layer, but this is not implemented")

    # this is here to encourage anyone implementing client security layers to do it
    # for both windows and posix
    client_security_layers = bytearray([NO_SECURITY_LAYER, 0, 0, 0])
    return client_security_layers

def _posix_sasl_gssapi(connection, controls):
    """ Performs a bind using the Kerberos v5 ("GSSAPI") SASL mechanism
    from RFC 4752 using the gssapi package that works natively on most
    posix operating systems.
    """
    target_name = gssapi.Name(_common_determine_target_name(connection), gssapi.NameType.hostbased_service)
    authz_id, creds = _common_determine_authz_id_and_creds(connection)

    ctx = gssapi.SecurityContext(name=target_name, mech=gssapi.MechType.kerberos, creds=creds,
                                 channel_bindings=get_channel_bindings(connection.socket))
    in_token = None
    try:
        while True:
            out_token = ctx.step(in_token)
            if out_token is None:
                out_token = ''
            result = send_sasl_negotiation(connection, controls, out_token)
            in_token = result['saslCreds']
            try:
                # This raised an exception in gssapi<1.1.2 if the context was
                # incomplete, but was fixed in
                # https://github.com/pythongssapi/python-gssapi/pull/70
                if ctx.complete:
                    break
            except gssapi.exceptions.MissingContextError:
                pass

        unwrapped_token = ctx.unwrap(in_token)
        client_security_layers = _common_process_end_token_get_security_layers(unwrapped_token.message)
        out_token = ctx.wrap(bytes(client_security_layers)+authz_id, False)
        return send_sasl_negotiation(connection, controls, out_token.message)
    except (gssapi.exceptions.GSSError, LDAPCommunicationError):
        abort_sasl_negotiation(connection, controls)
        raise


def _windows_sasl_gssapi(connection, controls):
    """ Performs a bind using the Kerberos v5 ("GSSAPI") SASL mechanism
    from RFC 4752 using the winkerberos package that works natively on most
    windows operating systems.
    """
    target_name = _common_determine_target_name(connection)
    # initiation happens before beginning the SASL bind when using windows kerberos
    authz_id, _ = _common_determine_authz_id_and_creds(connection)
    gssflags = (
            winkerberos.GSS_C_MUTUAL_FLAG |
            winkerberos.GSS_C_SEQUENCE_FLAG |
            winkerberos.GSS_C_INTEG_FLAG |
            winkerberos.GSS_C_CONF_FLAG
    )
    _, ctx = winkerberos.authGSSClientInit(target_name, gssflags=gssflags)

    in_token = b''
    try:
        negotiation_complete = False
        while not negotiation_complete:
            # GSSAPI is a "client goes first" SASL mechanism. Send the first "response" to the server and
            # recieve its first challenge.
            # Despite this, we can get channel binding, which includes CBTs for windows environments computed from
            # the peer certificate, before starting.
            status = winkerberos.authGSSClientStep(ctx, base64.b64encode(in_token).decode('utf-8'),
                                                   channel_bindings=get_channel_bindings(connection.socket))
            # figure out if we're done with our sasl negotiation
            negotiation_complete = (status == winkerberos.AUTH_GSS_COMPLETE)
            out_token = winkerberos.authGSSClientResponse(ctx) or ''
            out_token_bytes = base64.b64decode(out_token)
            result = send_sasl_negotiation(connection, controls, out_token_bytes)
            in_token = result['saslCreds'] or b''

        winkerberos.authGSSClientUnwrap( ctx,base64.b64encode(in_token).decode('utf-8'))
        negotiated_token = ''
        if winkerberos.authGSSClientResponse(ctx):
            negotiated_token = base64.standard_b64decode(winkerberos.authGSSClientResponse(ctx))
        client_security_layers = _common_process_end_token_get_security_layers(negotiated_token)
        # manually construct a message indicating use of authorization-only layer
        # see winkerberos example: https://github.com/mongodb/winkerberos/blob/master/test/test_winkerberos.py
        authz_only_msg = base64.b64encode(bytes(client_security_layers) + authz_id).decode('utf-8')
        winkerberos.authGSSClientWrap(ctx, authz_only_msg)
        out_token = winkerberos.authGSSClientResponse(ctx) or ''

        return send_sasl_negotiation(connection, controls, base64.b64decode(out_token))
    except (winkerberos.GSSError, LDAPCommunicationError):
        abort_sasl_negotiation(connection, controls)
        raise
