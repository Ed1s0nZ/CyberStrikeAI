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
import traceback
import calendar
import time
import re
from impacket.dcerpc.v5 import transport, samr, srvs, lsat, lsad, nrpc, wkst, scmr, tsch, rrp
from impacket.dcerpc.v5.rpcrt import DCERPCException, RPC_C_AUTHN_LEVEL_PKT_INTEGRITY
from impacket.dcerpc.v5.ndr import NULL
from impacket.dcerpc.v5.dtypes import RPC_SID, MAXIMUM_ALLOWED
from impacket.nmb import NetBIOSTimeout, NetBIOSError
from bloodhound.ad.utils import ADUtils, AceResolver
from bloodhound.enumeration.acls import parse_binary_acl
from bloodhound.ad.structures import LDAP_SID
from impacket.smb3 import SMB3
from impacket.smb import SMB
from impacket.smbconnection import SessionError
from impacket import smb
from impacket.smb3structs import SMB2_DIALECT_21
# Try to import exceptions here, if this does not succeed, then impacket version is too old
try:
    HostnameValidationExceptions = (SMB3.HostnameValidationException, SMB.HostnameValidationException)
except AttributeError:
    HostnameValidationExceptions = ()

class ADComputer(object):
    """
    Computer connected to Active Directory
    """
    def __init__(self, hostname=None, samname=None, ad=None, addc=None, objectsid=None):
        self.ad = ad
        self.addc = addc
        self.samname = samname
        self.rpc = None
        self.dce = None
        self.admins = []
        self.dcom = []
        self.rdp = []
        self.psremote = []
        self.trusts = []
        self.services = []
        self.sessions = []
        self.loggedon = []
        self.registry_sessions = []
        self.addr = None
        self.smbconnection = None
        self.TGS = None
        # The SID of the local domain
        self.sid = None
        # The SID within the domain
        self.objectsid = objectsid
        self.primarygroup = None
        if addc:
            self.aceresolver = AceResolver(ad, ad.objectresolver)
            # Which auth methods to try for this host
            self.auth_method = self.ad.auth.auth_method
        # Did connecting to this host fail before?
        self.permanentfailure = False
        # Process invalid hosts
        if not hostname:
            self.hostname = '%s.%s' % (samname[:-1].upper(), self.ad.domain.upper())
        else:
            self.hostname = hostname

    def get_bloodhound_data(self, entry, collect, skip_acl=False):
        data = {
            'ObjectIdentifier': self.objectsid,
            'AllowedToAct': [],
            'PrimaryGroupSID': self.primarygroup,
            'LocalAdmins': {
                'Collected': 'localadmin' in collect and not self.permanentfailure,
                'FailureReason': None,
                'Results': self.admins,
            },
            'PSRemoteUsers': {
                'Collected': 'psremote' in collect and not self.permanentfailure,
                'FailureReason': None,
                'Results': self.psremote
            },
            'Properties': {
                'name': self.hostname.upper(),
                'domainsid': self.ad.domain_object.sid,
                'domain': self.ad.domain.upper(),
                'distinguishedname': ADUtils.get_entry_property(entry, 'distinguishedName').upper()
            },
            'RemoteDesktopUsers': {
                'Collected': 'rdp' in collect and not self.permanentfailure,
                'FailureReason': None,
                'Results': self.rdp
            },
            'DcomUsers': {
                'Collected': 'dcom' in collect and not self.permanentfailure,
                'FailureReason': None,
                'Results': self.dcom
            },
            'AllowedToDelegate': [],
            'Sessions': {
                'Collected': 'session' in collect and not self.permanentfailure,
                'FailureReason': None,
                'Results': self.sessions
            },
            'PrivilegedSessions': {
                'Collected': 'loggedon' in collect and not self.permanentfailure,
                'FailureReason': None,
                'Results': self.loggedon
            },
            'RegistrySessions': {
                'Collected': 'loggedon' in collect and not self.permanentfailure,
                'FailureReason': None,
                'Results': self.registry_sessions
            },
            'Aces': [],
            'HasSIDHistory': [],
            'IsDeleted': ADUtils.get_entry_property(entry, 'isDeleted', default=False),
            'Status': None
        }

        props = data['Properties']
        # via the TRUSTED_FOR_DELEGATION (0x00080000) flag in UAC
        props['unconstraineddelegation'] = ADUtils.get_entry_property(entry, 'userAccountControl', default=0) & 0x00080000 == 0x00080000
        props['enabled'] = ADUtils.get_entry_property(entry, 'userAccountControl', default=0) & 2 == 0
        props['trustedtoauth'] = ADUtils.get_entry_property(entry, 'userAccountControl', default=0) & 0x01000000 == 0x01000000
        props['samaccountname'] = ADUtils.get_entry_property(entry, 'sAMAccountName')

        if 'objectprops' in collect or 'acl' in collect:
            props['haslaps'] = bool(ADUtils.get_entry_property(entry, 'ms-mcs-admpwdexpirationtime', 0) or ADUtils.get_entry_property(entry, 'mslaps-passwordexpirationtime', 0))

        if 'objectprops' in collect:
            props['lastlogon'] = ADUtils.win_timestamp_to_unix(
                ADUtils.get_entry_property(entry, 'lastlogon', default=0, raw=True)
            )
            props['lastlogontimestamp'] = ADUtils.win_timestamp_to_unix(
                ADUtils.get_entry_property(entry, 'lastlogontimestamp', default=0, raw=True)
            )
            if props['lastlogontimestamp'] == 0:
                props['lastlogontimestamp'] = -1
            props['pwdlastset'] = ADUtils.win_timestamp_to_unix(
                ADUtils.get_entry_property(entry, 'pwdLastSet', default=0, raw=True)
            )
            whencreated = ADUtils.get_entry_property(entry, 'whencreated', default=0)
            if not isinstance(whencreated, int):
                whencreated = calendar.timegm(whencreated.timetuple())
            props['whencreated'] = whencreated
            props['serviceprincipalnames'] = ADUtils.get_entry_property(entry, 'servicePrincipalName', [])
            props['description'] = ADUtils.get_entry_property(entry, 'description')

            osname = ADUtils.get_entry_property(entry, 'operatingSystem')
            osservicepack = ADUtils.get_entry_property(entry, 'operatingSystemServicePack')
            osversion = ADUtils.get_entry_property(entry, 'operatingSystemVersion')
            # Add SP to OS if specified
            props['operatingsystem'] = '%s %s' % (osname, osservicepack) if osservicepack else osname
            props['operatingsystemname'] = osname
            props['operatingsystemservicepack'] = osservicepack
            props['operatingsystemversion'] = osversion

            props['sidhistory'] = [LDAP_SID(bsid).formatCanonical() for bsid in ADUtils.get_entry_property(entry, 'sIDHistory', [])]
            delegatehosts = ADUtils.get_entry_property(entry, 'msDS-AllowedToDelegateTo', [])
            delegatehosts_cache = []
            for host in delegatehosts:
                try:
                    target = host.split('/')[1].split(':')[0]
                except IndexError:
                    logging.warning('Invalid delegation target: %s', host)
                    continue
                try:
                    object_sid = self.ad.computersidcache.get(target.lower())
                    data['AllowedToDelegate'].append({
                        'ObjectIdentifier': object_sid,
                        'ObjectType': ADUtils.resolve_ad_entry(
                            self.ad.objectresolver.resolve_sid(object_sid)
                        )['type'],
                    })
                except KeyError:
                    object_sam = target.upper().split(".")[0].split("\\")[0]
                    if object_sam in delegatehosts_cache:
                        continue
                    delegatehosts_cache.append(object_sam)
                    object_entry = self.ad.objectresolver.resolve_samname(object_sam)
                    if not object_entry:
                        object_entry = self.ad.objectresolver.resolve_samname(object_sam + '$')
                    if not object_entry:
                        object_entry = self.ad.objectresolver.resolve_samname(object_sam + '*', allow_filter=True)
                    if object_entry:
                        if len(object_entry) > 1:
                            object_resolved = None
                            for object_entry_instance in object_entry:
                                sam = ADUtils.get_entry_property(object_entry_instance, 'sAMAccountName')
                                if not sam:
                                    continue
                                if sam.lower() == object_sam.lower() or f"{sam}$".lower() == object_sam.lower():
                                    # Best match
                                    object_resolved = ADUtils.resolve_ad_entry(object_entry_instance)
                                    break
                            # No match? Then pick first one and hope for the best
                            if not object_resolved:
                                object_resolved = ADUtils.resolve_ad_entry(object_entry[0])
                        else:
                            object_resolved = ADUtils.resolve_ad_entry(object_entry[0])
                        data['AllowedToDelegate'].append({
                            'ObjectIdentifier': object_resolved['objectid'],
                            'ObjectType': object_resolved['type'],
                        })
            if len(delegatehosts) > 0:
                props['allowedtodelegate'] = delegatehosts

            # Process resource-based constrained delegation
            _, aces = parse_binary_acl(data,
                                       'computer',
                                       ADUtils.get_entry_property(entry,
                                                                  'msDS-AllowedToActOnBehalfOfOtherIdentity',
                                                                  raw=True),
                                       self.addc.objecttype_guid_map)
            outdata = self.aceresolver.resolve_aces(aces)
            for delegated in outdata:
                if delegated['RightName'] == 'Owner':
                    continue
                if delegated['RightName'] == 'GenericAll':
                    data['AllowedToAct'].append({'ObjectIdentifier': delegated['PrincipalSID'], 'ObjectType': delegated['PrincipalType']})

        # Run ACL collection if this was not already done centrally
        if 'acl' in collect and not skip_acl:
            _, aces = parse_binary_acl(data,
                                       'computer',
                                       ADUtils.get_entry_property(entry,
                                                                  'nTSecurityDescriptor',
                                                                  raw=True),
                                       self.addc.objecttype_guid_map)
            # Parse aces
            data['Aces'] = self.aceresolver.resolve_aces(aces)

        return data

    def try_connect(self):
        addr = None
        try:
            addr = self.ad.dnscache.get(self.hostname)
        except KeyError:
            try:
                q = self.ad.dnsresolver.query(self.hostname, 'A', tcp=self.ad.dns_tcp)
                for r in q:
                    addr = r.address

                if addr == None:
                    return False
            # Do exit properly on keyboardinterrupts
            except KeyboardInterrupt:
                raise
            except Exception as e:
                # Doesn't exist
                if "None of DNS query names exist" in str(e):
                    logging.info('Skipping enumeration for %s since it could not be resolved.', self.hostname)
                else:
                    logging.warning('Could not resolve: %s: %s', self.hostname, e)
                return False

            logging.debug('Resolved: %s' % addr)

            self.ad.dnscache.put(self.hostname, addr)

        self.addr = addr

        logging.debug('Trying connecting to computer: %s', self.hostname)
        # We ping the host here, this adds a small overhead for setting up an extra socket
        # but saves us from constructing RPC Objects for non-existing hosts. Also RPC over
        # SMB does not support setting a connection timeout, so we catch this here.
        return ADUtils.tcp_ping(addr, 445)


    def dce_rpc_connect(self, binding, uuid, integrity=False):
        if self.permanentfailure:
            logging.debug('Skipping connection because of previous failure')
            return None
        logging.debug('DCE/RPC binding: %s', binding)

        try:
            self.rpc = transport.DCERPCTransportFactory(binding)
            self.rpc.set_connect_timeout(1.0)

            # Set name/host explicitly
            self.rpc.setRemoteName(self.hostname)
            self.rpc.setRemoteHost(self.addr)

            # Use Kerberos if we have a TGT
            if hasattr(self.rpc, 'set_kerberos') and self.ad.auth.tgt and self.auth_method in ('auto', 'kerberos'):
                self.rpc.set_kerberos(True, self.ad.auth.kdc)
                if not self.TGS:
                    try:
                        self.TGS = self.ad.auth.get_tgs_for_smb(self.hostname)
                    except Exception as exc:
                        logging.debug(traceback.format_exc())
                        if self.auth_method == 'auto':
                            logging.warning('Failed to get service ticket for %s, falling back to NTLM auth', self.hostname)
                            self.auth_method = 'ntlm'
                        else:
                            logging.warning('Failed to get service ticket for %s, skipping host', self.hostname)
                            self.permanentfailure = True
                            return None
                if hasattr(self.rpc, 'set_credentials'):
                    if self.auth_method == 'auto':
                        # Set all we have
                        self.rpc.set_credentials(self.ad.auth.username, self.ad.auth.password,
                                                 domain=self.ad.auth.userdomain,
                                                 lmhash=self.ad.auth.lm_hash,
                                                 nthash=self.ad.auth.nt_hash,
                                                 aesKey=self.ad.auth.aeskey,
                                                 TGS=self.TGS)
                    elif self.auth_method == 'kerberos':
                        # Kerberos only
                        self.rpc.set_credentials(self.ad.auth.username, '',
                                                 domain=self.ad.auth.userdomain,
                                                 TGS=self.TGS)
                    else:
                        # NTLM fallback triggered
                        self.rpc.set_credentials(self.ad.auth.username, self.ad.auth.password,
                                                 domain=self.ad.auth.userdomain,
                                                 lmhash=self.ad.auth.lm_hash,
                                                 nthash=self.ad.auth.nt_hash)
            # Else set the required stuff for NTLM
            elif hasattr(self.rpc, 'set_credentials'):
                self.rpc.set_credentials(self.ad.auth.username, self.ad.auth.password,
                                         domain=self.ad.auth.userdomain,
                                         lmhash=self.ad.auth.lm_hash,
                                         nthash=self.ad.auth.nt_hash)

            # Use strict validation if possible
            if hasattr(self.rpc, 'set_hostname_validation'):
                self.rpc.set_hostname_validation(True, False, self.hostname)

            # Uncomment to force SMB2 (especially for development to prevent encryption)
            # will break clients only supporting SMB1 ofc
            # self.rpc.preferred_dialect(smb3structs.SMB2_DIALECT_21)

            # Re-use the SMB connection if possible
            if self.smbconnection:
                self.rpc.set_smb_connection(self.smbconnection)
            dce = self.rpc.get_dce_rpc()

            # Some interfaces require integrity (such as scheduled tasks)
            # others don't support it at all and error out.
            if integrity:
                dce.set_auth_level(RPC_C_AUTHN_LEVEL_PKT_INTEGRITY)

            # Try connecting, catch hostname validation
            try:
                dce.connect()
            except HostnameValidationExceptions as exc:
                logging.info('Ignoring host %s since its hostname does not match: %s', self.hostname, str(exc))
                self.permanentfailure = True
                return None
            except SessionError as exc:
                if ('STATUS_PIPE_NOT_AVAILABLE' in str(exc) or 'STATUS_OBJECT_NAME_NOT_FOUND' in str(exc)) and 'winreg' in binding.lower():
                    # This can happen, silently ignore
                    return None
                if 'STATUS_MORE_PROCESSING_REQUIRED' in str(exc):
                    if self.auth_method == 'kerberos':
                        logging.warning('Kerberos auth failed and no more auth methods to try.')
                    elif self.auth_method == 'auto':
                        logging.debug('Kerberos auth failed. Falling back to NTLM')
                        self.auth_method = 'ntlm'
                        # Close connection and retry
                        try:
                            self.rpc.get_smb_connection().close()
                        except:
                            pass
                        # Try again!
                        return self.dce_rpc_connect(binding, uuid, integrity)
                # Else, just log it
                logging.debug(traceback.format_exc())
                logging.warning('DCE/RPC connection failed: %s', str(exc))
                return None

            if self.smbconnection is None:
                self.smbconnection = self.rpc.get_smb_connection()
                # We explicity set the smbconnection back to the rpc object
                # this way it won't be closed when we call disconnect()
                self.rpc.set_smb_connection(self.smbconnection)

            # Hostname validation
            authname = self.smbconnection.getServerName()
            if authname and authname.lower() != self.hostname.split('.')[0].lower():
                logging.info('Ignoring host %s since its reported name %s does not match', self.hostname, authname)
                self.permanentfailure = True
                return None

            dce.bind(uuid)
        except DCERPCException as e:
            logging.debug(traceback.format_exc())
            logging.warning('DCE/RPC connection failed: %s', str(e))
            return None
        except KeyboardInterrupt:
            raise
        except Exception as e:
            logging.debug(traceback.format_exc())
            logging.warning('DCE/RPC connection failed: %s', e)
            return None
        except:
            logging.warning('DCE/RPC connection failed (unknown error)')
            return None

        return dce

    def rpc_get_loggedon(self):
        """
        Query logged on users via RPC.
        Requires admin privs
        """
        binding = r'ncacn_np:%s[\PIPE\wkssvc]' % self.addr
        loggedonusers = set()
        dce = self.dce_rpc_connect(binding, wkst.MSRPC_UUID_WKST)
        if dce is None:
            logging.warning('Connection failed: %s', binding)
            return
        try:
            # 1 means more detail, including the domain
            resp = wkst.hNetrWkstaUserEnum(dce, 1)
            for record in resp['UserInfo']['WkstaUserInfo']['Level1']['Buffer']:
                # Skip computer accounts
                if record['wkui1_username'][-2] == '$':
                    continue
                # Skip sessions for local accounts
                if record['wkui1_logon_domain'][:-1].upper() == self.samname[:-1].upper():
                    continue
                domain = record['wkui1_logon_domain'][:-1].upper()
                domain_entry = self.ad.get_domain_by_name(domain)
                if domain_entry is not None:
                    domain = ADUtils.ldap2domain(domain_entry['attributes']['distinguishedName'])
                logging.debug('Found logged on user at %s: %s@%s' % (self.hostname, record['wkui1_username'][:-1], domain))
                loggedonusers.add((record['wkui1_username'][:-1], domain))
        except DCERPCException as e:
            if 'rpc_s_access_denied' in str(e):
                logging.debug('Access denied while enumerating LoggedOn on %s, probably no admin privs', self.hostname)
            else:
                logging.debug('Exception connecting to RPC: %s', e)
        except Exception as e:
            if 'connection reset' in str(e):
                logging.debug('Connection was reset: %s', e)
            else:
                raise e

        dce.disconnect()
        return list(loggedonusers)

    def rpc_close(self):
        if self.smbconnection:
            self.smbconnection.logoff()

    def rpc_get_sessions(self):
        binding = r'ncacn_np:%s[\PIPE\srvsvc]' % self.addr

        dce = self.dce_rpc_connect(binding, srvs.MSRPC_UUID_SRVS)

        if dce is None:
            return

        try:
            resp = srvs.hNetrSessionEnum(dce, '\x00', NULL, 10)
        except DCERPCException as e:
            if 'rpc_s_access_denied' in str(e):
                logging.debug('Access denied while enumerating Sessions on %s, likely a patched OS', self.hostname)
                return []
            else:
                raise
        except Exception as e:
            if str(e).find('Broken pipe') >= 0:
                return
            else:
                raise

        sessions = []

        for session in resp['InfoStruct']['SessionInfo']['Level10']['Buffer']:
            userName = session['sesi10_username'][:-1]
            ip = session['sesi10_cname'][:-1]
            # Strip \\ from IPs
            if ip[:2] == '\\\\':
                ip = ip[2:]
            # Skip empty IPs
            if ip == '':
                continue
            # Skip our connection
            if userName == self.ad.auth.username:
                continue
            # Skip empty usernames
            if len(userName) == 0:
                continue
            # Skip machine accounts
            if userName[-1] == '$':
                continue
            # Skip local connections
            if ip in ['127.0.0.1', '[::1]']:
                continue
            # IPv6 address
            if ip[0] == '[' and ip[-1] == ']':
                ip = ip[1:-1]

            logging.info('User %s is logged in on %s from %s' % (userName, self.hostname, ip))

            sessions.append({'user': userName, 'source': ip, 'target': self.hostname})

        dce.disconnect()

        return sessions

    def rpc_get_registry_sessions(self):
        binding = r'ncacn_np:%s[\pipe\winreg]' % self.addr

        # Try to bind to the Remote Registry RPC interface, if it fails try again once.
        binding_attempts = 2
        while binding_attempts > 0:
            dce = self.dce_rpc_connect(binding, rrp.MSRPC_UUID_RRP)
            if dce is None:
                # If the Remote Registry is not yet started, the named pipe '\pipe\winreg' does not
                # exist and therefore the following exception is expected: STATUS_PIPE_NOT_AVAILABLE.
                # But this initial attempt should trigger it. Wait 1s and hope the service had enough
                # time to start.
                time.sleep(1)
            else:
                # We could connect to the Remote Registry, so exit the loop.
                break
            binding_attempts -= 1

        # If the two binding attempts failed, silently return.
        if dce is None:
            logging.debug('Failed opening remote registry after 2 attempts')
            return

        registry_sessions = []

        # Impacket's 'hOpenUsers' will allow us to open the remote HKU hive.
        try:
            resp = rrp.hOpenUsers(dce)
        except DCERPCException as e:
            if 'rpc_s_access_denied' in str(e):
                logging.debug('Access denied while enumerating Registry Sessions on %s', self.hostname)
                return []
            else:
                logging.debug('Exception connecting to RPC: %s', e)
        except Exception as e:
            if str(e).find('Broken pipe') >= 0:
                return
            else:
                raise

        # Once we have a handle on the remote HKU hive, we can call 'BaseRegEnumKey' in a loop in
        # order to enumerate the subkeys which names are the SIDs of the logged in users.
        key_handle = resp['phKey']
        index = 1
        sid_filter = "^S-1-5-21-[0-9]+-[0-9]+-[0-9]+-[0-9]+$"
        while True:
            try:
                resp = rrp.hBaseRegEnumKey(dce, key_handle, index)
                sid = resp['lpNameOut'].rstrip('\0')
                if re.match(sid_filter, sid):
                    logging.info('User with SID %s is logged in on %s' % (sid, self.hostname))
                    # Ignore local accounts (best effort, self.sid is only
                    # populated if we enumerated a group before)
                    if self.sid and sid.startswith(self.sid):
                        index += 1
                        continue
                    registry_sessions.append({'user': sid})
                index += 1
            except:
                break

        rrp.hBaseRegCloseKey(dce, key_handle)
        dce.disconnect()

        return registry_sessions

    """
    """
    def rpc_get_domain_trusts(self):
        binding = r'ncacn_np:%s[\PIPE\netlogon]' % self.addr

        dce = self.dce_rpc_connect(binding, nrpc.MSRPC_UUID_NRPC)

        if dce is None:
            return

        try:
            req = nrpc.DsrEnumerateDomainTrusts()
            req['ServerName'] = NULL
            req['Flags'] = 1
            resp = dce.request(req)
        except Exception as e:
            raise e

        for domain in resp['Domains']['Domains']:
            logging.info('Found domain trust from %s to %s', self.hostname, domain['NetbiosDomainName'])
            self.trusts.append({'domain': domain['DnsDomainName'],
                                'type': domain['TrustType'],
                                'flags': domain['Flags']})

        dce.disconnect()


    def rpc_get_services(self):
        """
        Query services with stored credentials via RPC.
        These credentials can be dumped with mimikatz via lsadump::secrets or via secretsdump.py
        """
        binding = r'ncacn_np:%s[\PIPE\svcctl]' % self.addr
        serviceusers = []
        dce = self.dce_rpc_connect(binding, scmr.MSRPC_UUID_SCMR)
        if dce is None:
            return serviceusers
        try:
            resp = scmr.hROpenSCManagerW(dce)
            scManagerHandle = resp['lpScHandle']
            # TODO: Figure out if filtering out service types makes sense
            resp = scmr.hREnumServicesStatusW(dce,
                                              scManagerHandle,
                                              dwServiceType=scmr.SERVICE_WIN32_OWN_PROCESS,
                                              dwServiceState=scmr.SERVICE_STATE_ALL)
            # TODO: Skip well-known services to save on traffic
            for i in range(len(resp)):
                try:
                    ans = scmr.hROpenServiceW(dce, scManagerHandle, resp[i]['lpServiceName'][:-1])
                    serviceHandle = ans['lpServiceHandle']
                    svcresp = scmr.hRQueryServiceConfigW(dce, serviceHandle)
                    svc_user = svcresp['lpServiceConfig']['lpServiceStartName'][:-1]
                    if '@' in svc_user:
                        logging.info("Found user service: %s running as %s on %s",
                                     resp[i]['lpServiceName'][:-1],
                                     svc_user,
                                     self.hostname)
                        serviceusers.append(svc_user)
                except DCERPCException as e:
                    if 'rpc_s_access_denied' not in str(e):
                        logging.debug('Exception querying service %s via RPC: %s', resp[i]['lpServiceName'][:-1], e)
        except DCERPCException as e:
            logging.debug('Exception connecting to RPC: %s', e)
        except Exception as e:
            if 'connection reset' in str(e):
                logging.debug('Connection was reset: %s', e)
            else:
                raise e

        dce.disconnect()
        return serviceusers


    def rpc_get_schtasks(self):
        """
        Query the scheduled tasks via RPC. Requires admin privileges.
        These credentials can be dumped with mimikatz via vault::cred
        """
        # Blacklisted folders (Default ones)
        blacklist = [u'Microsoft\x00']
        # Start with the root folder
        folders = ['\\']
        tasks = []
        schtaskusers = []
        binding = r'ncacn_np:%s[\PIPE\atsvc]' % self.addr
        try:
            dce = self.dce_rpc_connect(binding, tsch.MSRPC_UUID_TSCHS, True)
            if dce is None:
                return schtaskusers
            # Get root folder
            resp = tsch.hSchRpcEnumFolders(dce, '\\')
            for item in resp['pNames']:
                data = item['Data']
                if data not in blacklist:
                    folders.append('\\'+data)

            # Enumerate the folders we found
            # subfolders not supported yet
            for folder in folders:
                try:
                    resp = tsch.hSchRpcEnumTasks(dce, folder)
                    for item in resp['pNames']:
                        data = item['Data']
                        if folder != '\\':
                            # Make sure to strip the null byte
                            tasks.append(folder[:-1]+'\\'+data)
                        else:
                            tasks.append(folder+data)
                except DCERPCException as e:
                    logging.debug('Error enumerating task folder %s: %s', folder, e)
            for task in tasks:
                try:
                    resp = tsch.hSchRpcRetrieveTask(dce, task)
                    # This returns a tuple (sid, logontype) or None
                    userinfo = ADUtils.parse_task_xml(resp['pXml'])
                    if userinfo:
                        if userinfo[1] == u'Password':
                            # Convert to byte string because our cache format is in bytes
                            schtaskusers.append(str(userinfo[0]))
                            logging.info('Found scheduled task %s on %s with stored credentials for SID %s',
                                         task,
                                         self.hostname,
                                         userinfo[0])
                except DCERPCException as e:
                    logging.debug('Error querying task %s: %s', task, e)
        except DCERPCException as e:
            logging.debug('Exception enumerating scheduled tasks: %s', e)

        dce.disconnect()
        return schtaskusers


    """
    This magic is mostly borrowed from impacket/examples/netview.py
    """
    def rpc_get_group_members(self, group_rid, resultlist):
        binding = r'ncacn_np:%s[\PIPE\samr]' % self.addr
        unresolved = []
        dce = self.dce_rpc_connect(binding, samr.MSRPC_UUID_SAMR)

        if dce is None:
            return

        try:
            resp = samr.hSamrConnect(dce)
            serverHandle = resp['ServerHandle']
            # Attempt to get the SID from this computer to filter local accounts later
            try:
                resp = samr.hSamrLookupDomainInSamServer(dce, serverHandle, self.samname[:-1])
                self.sid = resp['DomainId'].formatCanonical()
            # This doesn't always work (for example on DCs)
            except DCERPCException as e:
                # Make it a string which is guaranteed not to match a SID
                self.sid = 'UNKNOWN'


            # Enumerate the domains known to this computer
            resp = samr.hSamrEnumerateDomainsInSamServer(dce, serverHandle)
            domains = resp['Buffer']['Buffer']

            # Query the builtin domain (derived from this SID)
            sid = RPC_SID()
            sid.fromCanonical('S-1-5-32')

            logging.debug('Opening domain handle')
            # Open a handle to this domain
            resp = samr.hSamrOpenDomain(dce,
                                        serverHandle=serverHandle,
                                        desiredAccess=samr.DOMAIN_LOOKUP | MAXIMUM_ALLOWED,
                                        domainId=sid)
            domainHandle = resp['DomainHandle']
            try:
                resp = samr.hSamrOpenAlias(dce,
                                           domainHandle,
                                           desiredAccess=samr.ALIAS_LIST_MEMBERS | MAXIMUM_ALLOWED,
                                           aliasId=group_rid)
            except samr.DCERPCSessionError as error:
                # Group does not exist
                if 'STATUS_NO_SUCH_ALIAS' in str(error):
                    logging.debug('No group with RID %d exists', group_rid)
                    return
            resp = samr.hSamrGetMembersInAlias(dce,
                                               aliasHandle=resp['AliasHandle'])
            for member in resp['Members']['Sids']:
                sid_string = member['SidPointer'].formatCanonical()

                logging.debug('Found %d SID: %s', group_rid, sid_string)
                if not sid_string.startswith(self.sid):
                    # If the sid is known, we can add the admin value directly
                    try:
                        siddata = self.ad.sidcache.get(sid_string)
                        if siddata is None:
                            unresolved.append(sid_string)
                        else:
                            logging.debug('Sid is cached: %s', siddata['principal'])
                            resultlist.append({'ObjectIdentifier': sid_string,
                                               'ObjectType': siddata['type'].capitalize()})
                    except KeyError:
                        # Append it to the list of unresolved SIDs
                        unresolved.append(sid_string)
                else:
                    logging.debug('Ignoring local group %s', sid_string)
        except DCERPCException as e:
            if 'rpc_s_access_denied' in str(e):
                logging.debug('Access denied while enumerating groups on %s, likely a patched OS', self.hostname)
            else:
                raise
        except Exception as e:
            if 'connection reset' in str(e):
                logging.debug('Connection was reset: %s', e)
            else:
                raise e

        dce.disconnect()
        return unresolved


    def rpc_resolve_sids(self, sids, resultlist):
        """
        Resolve any remaining unknown SIDs for local accounts.
        """
        # If all sids were already cached, we can just return
        if sids is None or len(sids) == 0:
            return
        binding = r'ncacn_np:%s[\PIPE\lsarpc]' % self.addr

        dce = self.dce_rpc_connect(binding, lsat.MSRPC_UUID_LSAT)

        if dce is None:
            return

        try:
            resp = lsad.hLsarOpenPolicy2(dce, lsat.POLICY_LOOKUP_NAMES | MAXIMUM_ALLOWED)
        except Exception as e:
            if str(e).find('Broken pipe') >= 0:
                return
            else:
                raise

        policyHandle = resp['PolicyHandle']

        # We could look up the SIDs all at once, but if not all SIDs are mapped, we don't know which
        # ones were resolved and which not, making it impossible to map them in the cache.
        # Therefor we use more SAMR calls at the start, but after a while most SIDs will be reliable
        # in our cache and this function doesn't even need to get called anymore.
        for sid_string in sids:
            try:
                resp = lsat.hLsarLookupSids(dce, policyHandle, [sid_string], lsat.LSAP_LOOKUP_LEVEL.enumItems.LsapLookupWksta)
            except DCERPCException as e:
                if str(e).find('STATUS_NONE_MAPPED') >= 0:
                    logging.warning('SID %s lookup failed, return status: STATUS_NONE_MAPPED', sid_string)
                    # Try next SID
                    continue
                elif str(e).find('STATUS_SOME_NOT_MAPPED') >= 0:
                    # Not all could be resolved, work with the ones that could
                    resp = e.get_packet()
                else:
                    raise
            except NetBIOSTimeout as e:
                logging.warning('Connection timed out while resolving sids')
                continue

            domains = []
            for entry in resp['ReferencedDomains']['Domains']:
                domains.append(entry['Name'])

            for entry in resp['TranslatedNames']['Names']:
                domain = domains[entry['DomainIndex']]
                domain_entry = self.ad.get_domain_by_name(domain)
                if domain_entry is not None:
                    domain = ADUtils.ldap2domain(domain_entry['attributes']['distinguishedName'])
                # TODO: what if it isn't? Should we fall back to LDAP?

                if entry['Name'] != '':
                    resolved_entry = ADUtils.resolve_sid_entry(entry, domain)
                    logging.debug('Resolved SID to name: %s', resolved_entry['principal'])
                    resultlist.append({'ObjectIdentifier': sid_string,
                                       'ObjectType': resolved_entry['type'].capitalize()})
                    # Add it to our cache
                    self.ad.sidcache.put(sid_string, resolved_entry)
                else:
                    logging.warning('Resolved name is empty [%s]', entry)
        try:
            dce.disconnect()
        except NetBIOSError:
            pass
