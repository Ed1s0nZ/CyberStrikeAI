####################
#
# Copyright (c) 2018 Fox-IT
#
# Permission is hereby granted, free of charge, to any person obtaining a copy
# of this software and associated documentation files (the "Software"), to deal
# in the Software without restriction, including without limitation the rights
# to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
# copies of the Software, and to permit persons to whom the Software is
# furnished to do so, subject to the following conditions:
#
# The above copyright notice and this permission notice shall be included in all
# copies or substantial portions of the Software.
#
# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
# FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
# AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
# LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
# OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
# SOFTWARE.
#
####################

import logging
import ssl
import os
import traceback
from hashlib import sha256, md5
from bloodhound.ad.utils import CollectionException
from binascii import unhexlify
from cryptography import x509
from cryptography.hazmat.backends import default_backend
from cryptography.hazmat.primitives import hashes
import ldap3
from ldap3 import Server, Connection, NTLM, ALL, SASL, KERBEROS, Tls
from ldap3.core.results import RESULT_STRONGER_AUTH_REQUIRED
from ldap3.operation.bind import bind_operation
from impacket.krb5.ccache import CCache
from impacket.krb5.types import Principal, KerberosTime, Ticket
from pyasn1.codec.der import decoder, encoder
from impacket.krb5.asn1 import AP_REQ, AS_REP, TGS_REQ, Authenticator, TGS_REP, seq_set, seq_set_iter, PA_FOR_USER_ENC, \
    Ticket as TicketAsn1, EncTGSRepPart
from impacket.krb5 import constants
from impacket.krb5.kerberosv5 import getKerberosTGT, getKerberosTGS, sendReceive
from impacket.krb5.gssapi import CheckSumField, GSS_C_SEQUENCE_FLAG, GSS_C_REPLAY_FLAG, GSS_C_MUTUAL_FLAG
import datetime
from pyasn1.type.univ import noValue
from impacket.spnego import SPNEGO_NegTokenInit, TypesMech

"""
Active Directory authentication helper
"""
class ADAuthentication(object):
    def __init__(self, username='', password='', domain='',
                 lm_hash='', nt_hash='', aeskey='', kdc=None, auth_method='auto', ldap_channel_binding=False):
        self.username = username
        # Assume user domain and enum domain are same
        self.domain = domain.lower()
        self.userdomain = domain.lower()
        # If not, override userdomain
        if '@' in self.username:
            self.username, self.userdomain = self.username.lower().rsplit('@', 1)
        self.password = password
        self.lm_hash = lm_hash
        self.nt_hash = nt_hash
        self.aeskey = aeskey
        # KDC for domain we query
        self.kdc = kdc
        # KDC for domain of the user - fill with domain first, will be resolved later
        self.userdomain_kdc = self.domain
        self.auth_method = auth_method
        self.ldap_channel_binding = ldap_channel_binding

        # Kerberos
        self.tgt = None

    def set_aeskey(self, aeskey):
        self.aeskey = aeskey

    def set_kdc(self, kdc):
        # Set KDC
        self.kdc = kdc
        if self.userdomain == self.domain:
            # Also set it for user domain if this is equal
            self.userdomain_kdc = kdc

    def getLDAPConnection(self, hostname='', ip='', baseDN='', protocol='ldaps', gc=False):
        if gc:
            # Global Catalog connection
            if protocol == 'ldaps' or self.ldap_channel_binding is True:
                if self.ldap_channel_binding is True:
                    if not hasattr(ldap3, 'TLS_CHANNEL_BINDING'):
                        raise Exception("To use LDAP channel binding, install the patched ldap3 module: pip3 install git+https://github.com/ly4k/ldap3 or pip3 install ldap3-bleeding-edge")
                    logging.debug("Using LDAPS channel binding")
                    protocol = 'ldaps'
                    version=ssl.PROTOCOL_TLSv1_2
                    tls = Tls(validate=ssl.CERT_NONE, version=version, ciphers='ALL:@SECLEVEL=0')
                    server = Server(
                        "%s://%s:3269" % (protocol,ip),
                        use_ssl=True,
                        get_info=ALL,
                        tls=tls
                    )
                else:
                    # Ldap SSL (no channel binding)
                    server = Server("%s://%s:3269" % (protocol, ip), get_info=ALL)
            else:
                # Plain LDAP
                server = Server("%s://%s:3268" % (protocol, ip), get_info=ALL)
        else: # no GC specified
            if self.ldap_channel_binding is True:
                if not hasattr(ldap3, 'TLS_CHANNEL_BINDING'):
                    raise Exception("To use LDAP channel binding, install the patched ldap3 module: pip3 install git+https://github.com/ly4k/ldap3 or pip3 install ldap3-bleeding-edge")
                logging.debug("Using LDAPS channel binding")
                protocol = 'ldaps'
                version=ssl.PROTOCOL_TLSv1_2
                tls = Tls(validate=ssl.CERT_NONE, version=version, ciphers='ALL:@SECLEVEL=0')
                server = Server(
                    "%s://%s" % (protocol,ip),
                    use_ssl=True,
                    get_info=ALL,
                    tls=tls
                )
            else: # No LDAP Channel Binding
                server = Server("%s://%s" % (protocol, ip), get_info=ALL)
        # ldap3 supports auth with the NT hash. LM hash is actually ignored since only NTLMv2 is used.
        if self.nt_hash != '':
            if self.lm_hash != '':
                ldappass = self.lm_hash + ':' + self.nt_hash
            else:
                # ldap3 requires a 32-character long string for LM hash in order to use the NT hash
                ldappass = 'aad3b435b51404eeaad3b435b51404ee:' + self.nt_hash
        else:
            ldappass = self.password
        ldaplogin = '%s\\%s' % (self.userdomain, self.username)

        bound = False
        if self.tgt is not None and self.auth_method in ('kerberos', 'auto'):
            conn = Connection(server, user=ldaplogin, auto_referrals=False, password=ldappass, authentication=SASL, sasl_mechanism=KERBEROS, receive_timeout=60, auto_range=True)
            logging.debug('Authenticating to LDAP server with Kerberos')
            try:
                bound = self.ldap_kerberos(conn, hostname)
            except Exception as exc:
                if self.auth_method == 'auto':
                    logging.debug(traceback.format_exc())
                    logging.warning('Kerberos auth to LDAP failed, trying NTLM')
                    bound = False
                else:
                    logging.debug(traceback.format_exc())
                    logging.critical('Kerberos auth to LDAP failed, no authentication methods left')
                    raise CollectionException('Could not authenticate to LDAP. Check your credentials and LDAP server requirements.')
        if not bound and self.auth_method in ('auto','ntlm'):
            conn = Connection(server, user=ldaplogin, auto_referrals=False, password=ldappass, authentication=NTLM, receive_timeout=60, auto_range=True)
            logging.debug('Authenticating to LDAP server with NTLM')
            if self.ldap_channel_binding:
                from ldap3 import TLS_CHANNEL_BINDING
                logging.debug("Using LDAPS channel binding")
                protocol = 'ldaps'
                channel_binding = {"channel_binding": TLS_CHANNEL_BINDING}
                conn = Connection(server, user=ldaplogin, password=ldappass, authentication=NTLM, auto_referrals=False, receive_timeout=60, auto_range=True, **channel_binding)
            else:
                conn = Connection(server, user=ldaplogin, auto_referrals=False, password=ldappass, authentication=NTLM, receive_timeout=60, auto_range=True)
            bound = conn.bind()

        if not bound:
            result = conn.result
            if result['result'] == RESULT_STRONGER_AUTH_REQUIRED and protocol == 'ldaps':
                logging.warning('LDAP Authentication is refused because LDAP Channel Binding is likely enabled. '
                                'Trying to connect using LDAP Channel Binding')
                self.ldap_channel_binding = True
                return self.getLDAPConnection(hostname, ip, baseDN, 'ldaps')
            if result['result'] == RESULT_STRONGER_AUTH_REQUIRED and protocol == 'ldap':
                logging.warning('LDAP Authentication is refused because LDAP signing is enabled. '
                                'Trying to connect over LDAPS instead...')
                return self.getLDAPConnection(hostname, ip, baseDN, 'ldaps')
            else:
                logging.error('Failure to authenticate with LDAP! Error %s : Code: %s' % (result['message'], result['result']))
                raise CollectionException('Could not authenticate to LDAP. Check your credentials and LDAP server requirements.')
        return conn

    def ldap_kerberos(self, connection, hostname):
        # Hackery to authenticate with ldap3 using impacket Kerberos stack

        # Open ldap3 socket because we need it
        connection.open(read_server_info=False)

        username = Principal(self.username, type=constants.PrincipalNameType.NT_PRINCIPAL.value)
        servername = Principal('ldap/%s' % hostname, type=constants.PrincipalNameType.NT_SRV_INST.value)
        tgs, cipher, _, sessionkey = getKerberosTGS(servername, self.domain, self.kdc,
                                                                self.tgt['KDC_REP'], self.tgt['cipher'], self.tgt['sessionKey'])

        # Let's build a NegTokenInit with a Kerberos AP_REQ
        blob = SPNEGO_NegTokenInit()

        # Kerberos
        blob['MechTypes'] = [TypesMech['MS KRB5 - Microsoft Kerberos 5']]

        # Let's extract the ticket from the TGS
        tgs = decoder.decode(tgs, asn1Spec=TGS_REP())[0]
        ticket = Ticket()
        ticket.from_asn1(tgs['ticket'])

        # Now let's build the AP_REQ
        apReq = AP_REQ()
        apReq['pvno'] = 5
        apReq['msg-type'] = int(constants.ApplicationTagNumbers.AP_REQ.value)

        opts = []
        apReq['ap-options'] = constants.encodeFlags(opts)
        seq_set(apReq, 'ticket', ticket.to_asn1)

        authenticator = Authenticator()
        authenticator['authenticator-vno'] = 5
        authenticator['crealm'] = self.userdomain
        seq_set(authenticator, 'cname', username.components_to_asn1)
        now = datetime.datetime.utcnow()

        authenticator['cusec'] = now.microsecond
        authenticator['ctime'] = KerberosTime.to_asn1(now)

        peercert = None
        bindings = None
        try:
            peercert = connection.socket.getpeercert(binary_form=True)
        except AttributeError:
            # No TLS, skip
            pass
        if peercert:
            # Do TLS channel binding
            # The logic here is heavly inspired by "msldap", "minikerberos" and "asysocks" projects by @skelsec.
            # Adapted from ldap3 contributions by ThePirateWhoSmellsOfSunflowers
            peer_certificate = x509.load_der_x509_certificate(peercert, default_backend())
            peer_certificate_hash_algorithm = peer_certificate.signature_hash_algorithm

            # RFC 5929 section 4.1 hashes list
            rfc5929_hashes_list = (hashes.MD5, hashes.SHA1)

            # section 4.1 hash function selection
            if isinstance(peer_certificate_hash_algorithm, rfc5929_hashes_list):
                digest = hashes.Hash(hashes.SHA256(), default_backend())
            else:
                digest = hashes.Hash(peer_certificate_hash_algorithm, default_backend())
            digest.update(peercert)
            peer_certificate_digest = digest.finalize()

            # https://datatracker.ietf.org/doc/html/rfc2744#section-3.11
            channel_binding_struct = bytes()
            initiator_address = b'\x00'*8
            acceptor_address = b'\x00'*8

            # https://datatracker.ietf.org/doc/html/rfc5929#section-4
            application_data_raw = b'tls-server-end-point:' + peer_certificate_digest
            len_application_data = len(application_data_raw).to_bytes(4, byteorder='little', signed = False)
            application_data = len_application_data
            application_data += application_data_raw
            channel_binding_struct += initiator_address
            channel_binding_struct += acceptor_address
            channel_binding_struct += application_data
            bindings = md5(channel_binding_struct).digest()

        # Add checksum to authenticator
        authenticator['cksum'] = noValue
        authenticator['cksum']['cksumtype'] = 0x8003

        chkField = CheckSumField()
        chkField['Lgth'] = 16
        if bindings:
            chkField['Bnd'] = bindings
        chkField['Flags'] = 0
        authenticator['cksum']['checksum'] = chkField.getData()

        encodedAuthenticator = encoder.encode(authenticator)

        # Key Usage 11
        # AP-REQ Authenticator (includes application authenticator
        # subkey), encrypted with the application session key
        # (Section 5.5.1)
        encryptedEncodedAuthenticator = cipher.encrypt(sessionkey, 11, encodedAuthenticator, None)

        apReq['authenticator'] = noValue
        apReq['authenticator']['etype'] = cipher.enctype
        apReq['authenticator']['cipher'] = encryptedEncodedAuthenticator

        blob['MechToken'] = encoder.encode(apReq)

        # From here back to ldap3
        request = bind_operation(connection.version, SASL, None, None, connection.sasl_mechanism, blob.getData())
        response = connection.post_send_single_response(connection.send('bindRequest', request, None))[0]
        connection.result = response
        if response['result'] == 0:
            connection.bound = True
            connection.refresh_server_info()
        return response['result'] == 0

    def get_tgt(self):
        """
        Request a Kerberos TGT given our provided inputs.
        """
        username = Principal(self.username, type=constants.PrincipalNameType.NT_PRINCIPAL.value)
        logging.info('Getting TGT for user')

        try:
            tgt, cipher, _, session_key = getKerberosTGT(username, self.password, self.userdomain,
                                                         unhexlify(self.lm_hash), unhexlify(self.nt_hash),
                                                         self.aeskey,
                                                         self.userdomain_kdc)
        except Exception as exc:
            logging.debug(traceback.format_exc())
            if self.auth_method == 'auto':
                logging.warning('Failed to get Kerberos TGT. Falling back to NTLM authentication. Error: %s', str(exc))
                return
            else:
                # No other auth methods, so raise exception
                logging.error('Failed to get Kerberos TGT.')
                raise

        if self.userdomain != self.domain:
            # Try to get inter-realm TGT
            username = Principal(self.username, type=constants.PrincipalNameType.NT_PRINCIPAL.value)
            servername = Principal('krbtgt/%s' % self.domain, type=constants.PrincipalNameType.NT_SRV_INST.value)
            # Get referral TGT
            tgs, cipher, _, sessionkey = getKerberosTGS(servername, self.userdomain, self.userdomain_kdc,
                                                                    tgt, cipher, session_key)
            # See if this is a ticket for the correct domain
            refneeded = True
            while refneeded:
                decoded_tgs = decoder.decode(tgs, asn1Spec = TGS_REP())[0]
                next_realm = str(decoded_tgs['ticket']['sname']['name-string'][1])
                if next_realm.upper() == self.domain.upper():
                    refneeded = False
                else:
                    # Get next referral TGT
                    logging.debug('Following referral across trust to get next TGT')
                    servername = Principal('krbtgt/%s' % self.domain, type=constants.PrincipalNameType.NT_SRV_INST.value)
                    tgs, cipher, _, sessionkey = getKerberosTGS(servername, next_realm, next_realm,
                                                                            tgs, cipher, sessionkey)

            # Get foreign domain TGT
            servername = Principal('krbtgt/%s' % self.domain, type=constants.PrincipalNameType.NT_SRV_INST.value)
            tgs, cipher, _, sessionkey = getKerberosTGS(servername, self.domain, self.kdc,
                                                                    tgs, cipher, sessionkey)
            # Store this as our TGT
            self.tgt = {
                'KDC_REP': tgs,
                'cipher': cipher,
                'sessionKey': sessionkey
            }
        else:
            TGT = dict()
            TGT['KDC_REP'] = tgt
            TGT['cipher'] = cipher
            TGT['sessionKey'] = session_key
            self.tgt = TGT

    def get_tgs_for_smb(self, hostname):
        """
        Get a TGS for use with SMB Connection. We do this here to make sure the realms are correct,
        since impacket doesn't support cross-realm TGT usage and we don't want it to do its own Kerberos
        """
        username = Principal(self.username, type=constants.PrincipalNameType.NT_PRINCIPAL.value)
        servername = Principal('cifs/%s' % hostname, type=constants.PrincipalNameType.NT_SRV_INST.value)
        tgs, cipher, _, sessionkey = getKerberosTGS(servername, self.domain, self.kdc,
                                                                self.tgt['KDC_REP'], self.tgt['cipher'], self.tgt['sessionKey'])
        return {
            'KDC_REP': tgs,
            'cipher': cipher,
            'sessionKey': sessionkey
        }

    def load_ccache(self):
        """
        Extract a TGT from a ccache file.
        """
        # If the kerberos credential cache is known, use that.
        krb5cc = os.getenv('KRB5CCNAME')

        # Otherwise, guess it.
        if krb5cc is None:
            try:
                krb5cc = '/tmp/krb5cc_%u' % os.getuid()
            except AttributeError:
                # This fails on Windows
                krb5cc = 'nonexistingfile'

        if os.path.isfile(krb5cc):
            logging.debug('Using kerberos credential cache: %s', krb5cc)
        else:
            logging.debug('No Kerberos credential cache file found, manually requesting TGT')
            return False

        # Load TGT for our domain
        ccache = CCache.loadFile(krb5cc)
        principal = 'krbtgt/%s@%s' % (self.domain.upper(), self.domain.upper())
        creds = ccache.getCredential(principal, anySPN=False)
        if creds is not None:
            TGT = creds.toTGT()
            # This we store for later
            self.tgt = TGT
            tgt, cipher, session_key = TGT['KDC_REP'], TGT['cipher'], TGT['sessionKey']
            logging.info('Using TGT from cache')
        else:
            logging.debug("No valid credentials found in cache. ")
            return False

        # Verify if this ticket is actually for the specified user
        ticket = Ticket()
        decoded_tgt = decoder.decode(tgt, asn1Spec = AS_REP())[0]
        ticket.from_asn1(decoded_tgt['ticket'])

        tgt_principal = Principal()
        tgt_principal.from_asn1(decoded_tgt, 'crealm', 'cname')
        expected_principal = '%s@%s' % (self.username.lower(), self.domain.upper())
        if expected_principal.upper() != str(tgt_principal).upper():
            logging.warning('Username in ccache file does not match supplied username! %s != %s', tgt_principal, expected_principal)
            return False
        else:
            logging.info('Found TGT with correct principal in ccache file.')
        return True
