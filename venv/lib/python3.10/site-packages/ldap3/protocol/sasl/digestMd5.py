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

from binascii import hexlify
import hashlib
import hmac

from ... import SEQUENCE_TYPES
from ...protocol.sasl.sasl import abort_sasl_negotiation, send_sasl_negotiation, random_hex_string


STATE_KEY = 0
STATE_VALUE = 1


def md5_h(value):
    if not isinstance(value, bytes):
        value = value.encode()

    return hashlib.md5(value).digest()


def md5_kd(k, s):
    if not isinstance(k, bytes):
        k = k.encode()

    if not isinstance(s, bytes):
        s = s.encode()

    return md5_h(k + b':' + s)


def md5_hex(value):
    if not isinstance(value, bytes):
        value = value.encode()

    return hexlify(value)


def md5_hmac(k, s):
    if not isinstance(k, bytes):
        k = k.encode()

    if not isinstance(s, bytes):
        s = s.encode()

    return hmac.new(k, s, digestmod=hashlib.md5).hexdigest()


def sasl_digest_md5(connection, controls):
    # sasl_credential must be a tuple made up of the following elements: (realm, user, password, authorization_id) or (realm, user, password, authorization_id, enable_signing)
    # if realm is None will be used the realm received from the server, if available
    if not isinstance(connection.sasl_credentials, SEQUENCE_TYPES) or not len(connection.sasl_credentials) in (4, 5):
        return None

    # step One of RFC2831
    result = send_sasl_negotiation(connection, controls, None)
    if 'saslCreds' in result and result['saslCreds'] is not None:
        server_directives = decode_directives(result['saslCreds'])
    else:
        return None

    if 'realm' not in server_directives or 'nonce' not in server_directives or 'algorithm' not in server_directives:  # mandatory directives, as per RFC2831
        abort_sasl_negotiation(connection, controls)
        return None

    # step Two of RFC2831
    charset = server_directives['charset'] if 'charset' in server_directives and server_directives['charset'].lower() == 'utf-8' else 'iso8859-1'
    user = connection.sasl_credentials[1].encode(charset)
    realm = (connection.sasl_credentials[0] if connection.sasl_credentials[0] else (server_directives['realm'] if 'realm' in server_directives else '')).encode(charset)
    password = connection.sasl_credentials[2].encode(charset)
    authz_id = connection.sasl_credentials[3].encode(charset) if connection.sasl_credentials[3] else b''
    nonce = server_directives['nonce'].encode(charset)
    cnonce = random_hex_string(16).encode(charset)
    uri = b'ldap/' + connection.server.host.encode(charset)
    qop = b'auth'
    if len(connection.sasl_credentials) == 5 and connection.sasl_credentials[4] == 'sign' and not connection.server.ssl:
        qop = b'auth-int'
        connection._digest_md5_sec_num = 0

    digest_response = b'username="' + user + b'",'
    digest_response += b'realm="' + realm + b'",'
    digest_response += (b'authzid="' + authz_id + b'",') if authz_id else b''
    digest_response += b'nonce="' + nonce + b'",'
    digest_response += b'cnonce="' + cnonce + b'",'
    digest_response += b'digest-uri="' + uri + b'",'
    digest_response += b'qop=' + qop + b','
    digest_response += b'nc=00000001' + b','
    if charset == 'utf-8':
        digest_response += b'charset="utf-8",'

    a0 = md5_h(b':'.join([user, realm, password]))
    a1 = b':'.join([a0, nonce, cnonce, authz_id]) if authz_id else b':'.join([a0, nonce, cnonce])
    a2 = b'AUTHENTICATE:' + uri + (b':00000000000000000000000000000000' if qop in [b'auth-int', b'auth-conf'] else b'')

    if qop == b'auth-int':
        connection._digest_md5_kis = md5_h(md5_h(a1) + b"Digest session key to server-to-client signing key magic constant")
        connection._digest_md5_kic = md5_h(md5_h(a1) + b"Digest session key to client-to-server signing key magic constant")

    digest_response += b'response="' + md5_hex(md5_kd(md5_hex(md5_h(a1)), b':'.join([nonce, b'00000001', cnonce, qop, md5_hex(md5_h(a2))]))) + b'"'

    result = send_sasl_negotiation(connection, controls, digest_response)
    return result


def decode_directives(directives_string):
    """
    converts directives to dict, unquote values
    """

    # old_directives = dict((attr[0], attr[1].strip('"')) for attr in [line.split('=') for line in directives_string.split(',')])
    state = STATE_KEY
    tmp_buffer = ''
    quoting = False
    key = ''
    directives = dict()
    for c in directives_string.decode('utf-8'):
        if state == STATE_KEY and c == '=':
            key = tmp_buffer
            tmp_buffer = ''
            state = STATE_VALUE
        elif state == STATE_VALUE and c == '"' and not quoting and not tmp_buffer:
            quoting = True
        elif state == STATE_VALUE and c == '"' and quoting:
            quoting = False
        elif state == STATE_VALUE and c == ',' and not quoting:
            directives[key] = tmp_buffer
            tmp_buffer = ''
            key = ''
            state = STATE_KEY
        else:
            tmp_buffer += c

    if key and tmp_buffer:
        directives[key] = tmp_buffer

    return directives
