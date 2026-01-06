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
import codecs
import json
import ldap3
import ipaddress

from uuid import UUID
from dns import resolver
from ldap3 import ALL_ATTRIBUTES, BASE, SUBTREE, LEVEL
from ldap3.core.exceptions import LDAPKeyError, LDAPAttributeError, LDAPCursorError, LDAPNoSuchObjectResult, LDAPSocketReceiveError, LDAPSocketSendError, LDAPCommunicationError
from ldap3.protocol.microsoft import security_descriptor_control
from bloodhound.ad.utils import ADUtils, DNSCache, SidCache, SamCache, CollectionException
from bloodhound.ad.computer import ADComputer
from bloodhound.enumeration.objectresolver import ObjectResolver

"""
Active Directory Domain Controller
"""
class ADDC(ADComputer):
    def __init__(self, hostname=None, ad=None):
        ADComputer.__init__(self, hostname)
        self.ad = ad
        # Primary LDAP connection
        self.ldap = None
        # Secondary LDAP connection
        self.resolverldap = None
        # GC LDAP connection
        self.gcldap = None
        # Initialize GUID map
        self.objecttype_guid_map = dict()

    def ldap_connect(self, protocol=None, resolver=False):
        """
        Connect to the LDAP service
        """
        if not protocol:
            protocol = self.ad.ldap_default_protocol

        logging.info('Connecting to LDAP server: %s' % self.hostname)
        logging.debug('Using protocol %s' % protocol)


        ip = None
        # Convert the hostname to an IP, this prevents ldap3 from doing it
        # which doesn't use our custom nameservers
        try:
            q = self.ad.dnsresolver.resolve(self.hostname, 'A', tcp=self.ad.dns_tcp)
            for rdata in q:
                logging.debug('LDAP server IPv4: %s', str(rdata.address))
                logging.debug('Testing resolved hostname connectivity %s', rdata.address)
                try:
                    _tmp_serv = ldap3.Server(rdata.address, connect_timeout=5)
                    _conn = ldap3.Connection(_tmp_serv)
                    _bind = _conn.bind()
                except ldap3.core.exceptions.LDAPSocketOpenError:
                    continue
                else:
                    if _conn:
                        logging.debug('Successful connection to LDAP server on IP: %s', rdata.address)
                        ip = rdata.address
        except:
            logging.debug('No A records found')

        try:
            q6 = self.ad.dnsresolver.resolve(self.hostname, 'AAAA', tcp=self.ad.dns_tcp)
            for rdata in q6:
                logging.debug('LDAP server IPv6: %s', str(rdata.address))
                logging.info('Testing resolved hostname connectivity %s', rdata.address)
                try:
                    logging.info('Trying LDAP connection to %s', rdata.address)
                    _tmp_serv = ldap3.Server(rdata.address, connect_timeout=5)
                    _conn = ldap3.Connection(_tmp_serv)
                    _bind = _conn.bind()
                except ldap3.core.exceptions.LDAPSocketOpenError:
                    continue
                except ldap3.core.exceptions.LDAPInvalidServerError:
                    logging.error('LDAP3 did not like address: %s', rdata.address)
                    continue
                else:
                    if _conn:
                        logging.debug('Successful connection to: %s', rdata.address)
                        ip = f'[{rdata.address}]'
        except:
            logging.debug('No AAAA records found')

        if not ip:
            logging.error('Failed to resolve LDAP server IP')
            return False

        ldap = self.ad.auth.getLDAPConnection(hostname=self.hostname, ip=ip,
                                              baseDN=self.ad.baseDN, protocol=protocol)
        if resolver:
            self.resolverldap = ldap
        else:
            self.ldap = ldap
        return ldap is not None

    def gc_connect(self, protocol='ldap'):
        """
        Connect to the global catalog
        """
        if self.hostname in self.ad.gcs():
            # This server is a Global Catalog
            initial_server = self.hostname
        else:
            # Pick the first GC server
            try:
                initial_server = self.ad.gcs()[0]
            except IndexError:
                logging.error('Could not find a Global Catalog in this domain!'\
                              ' Resolving will be unreliable in forests with multiple domains')
                return False
        try:
            # Convert the hostname to an IP, this prevents ldap3 from doing it
            # which doesn't use our custom nameservers
            logging.info('Connecting to GC LDAP server: %s' % initial_server)
            q = self.ad.dnsresolver.query(initial_server, tcp=self.ad.dns_tcp)
            for r in q:
                ip = r.address
        except (resolver.NXDOMAIN, resolver.Timeout):
            for server in self.ad.gcs():
                # Skip the one we already tried
                if server == initial_server:
                    continue
                try:
                    # Convert the hostname to an IP, this prevents ldap3 from doing it
                    # which doesn't use our custom nameservers
                    logging.info('Connecting to GC LDAP server: %s' % server)
                    q = self.ad.dnsresolver.query(server, tcp=self.ad.dns_tcp)
                    for r in q:
                        ip = r.address
                        break
                except (resolver.NXDOMAIN, resolver.Timeout):
                    continue

        self.gcldap = self.ad.auth.getLDAPConnection(hostname=self.hostname, ip=ip, gc=True,
                                                     baseDN=self.ad.baseDN, protocol=protocol)
        return self.gcldap is not None

    def search(self, search_filter='(objectClass=*)',attributes=None, search_base=None, generator=True, use_gc=False, use_resolver=False, query_sd=False, is_retry=False,  search_scope=SUBTREE,):
        """
        Search for objects in LDAP or Global Catalog LDAP.
        """
        if self.ldap is None and not use_resolver:
            self.ldap_connect(resolver=use_resolver)
        if self.resolverldap is None and use_resolver:
            self.ldap_connect(resolver=use_resolver)
        if search_base is None:
            search_base = self.ad.baseDN
        if attributes is None or attributes == []:
            attributes = ALL_ATTRIBUTES
        if query_sd:
            # Set SD flags to only query for DACL and Owner
            controls = security_descriptor_control(sdflags=0x05)
        else:
            controls = None

        # If we don't query the GC, don't accept an empty search base
        if search_base == "" and not use_gc:
            search_base = self.ad.baseDN

        # Use the GC if this is requested
        if use_gc:
            searcher = self.gcldap
        else:
            # If this request comes from the resolver thread, use that
            if use_resolver:
                searcher = self.resolverldap
            else:
                searcher = self.ldap

        hadresults = False
        sresult = searcher.extend.standard.paged_search(search_base,
                                                        search_filter,
                                                        attributes=attributes,
                                                        paged_size=200,
                                                        search_scope=search_scope,
                                                        controls=controls,
                                                        generator=generator)
        try:
            # Use a generator for the result regardless of if the search function uses one
            for e in sresult:
                if e['type'] != 'searchResEntry':
                    continue
                if not hadresults:
                    hadresults = True
                yield e
        except LDAPNoSuchObjectResult:
            # This may indicate the object doesn't exist or access is denied
            logging.warning('LDAP Server reported that the search in %s for %s does not exist.', search_base, search_filter)
        except (LDAPSocketReceiveError, LDAPSocketSendError, LDAPCommunicationError) as e:
            if is_retry:
                logging.error('Connection to LDAP server lost during data gathering - reconnect failed - giving up on query %s', search_filter)
            else:
                if hadresults:
                    logging.error('Connection to LDAP server lost during data gathering. Query was cut short. Data may be inaccurate for query %s', search_filter)
                    if use_gc:
                        self.gc_connect()
                    else:
                        self.ldap_connect(resolver=use_resolver)
                else:
                    logging.warning('Re-establishing connection with server')
                    if use_gc:
                        self.gc_connect()
                    else:
                        self.ldap_connect(resolver=use_resolver)
                    # Try again
                    yield from self.search(search_filter, attributes, search_base, generator, use_gc, use_resolver, query_sd, is_retry=True)


    def ldap_get_single(self, qobject, attributes=None, use_gc=False, use_resolver=False, is_retry=False):
        """
        Get a single object, requires full DN to object.
        This function supports searching both in the local directory and the Global Catalog.
        The connection to the GC should already be established before calling this function.
        """
        if use_gc:
            searcher = self.gcldap
        else:
            # If this request comes from the resolver thread, use that
            if use_resolver:
                searcher = self.resolverldap
            else:
                searcher = self.ldap
        if attributes is None or attributes == []:
            attributes = ALL_ATTRIBUTES
        try:
            sresult = searcher.extend.standard.paged_search(qobject,
                                                            '(objectClass=*)',
                                                            search_scope=BASE,
                                                            attributes=attributes,
                                                            paged_size=10,
                                                            generator=False)
        except (LDAPSocketReceiveError, LDAPSocketSendError, LDAPCommunicationError) as e:
            if is_retry:
                logging.error('Connection to LDAP server lost during object resolving - reconnect failed - giving up on resolving %s', qobject)
                return None
            else:
                logging.warning('Re-establishing connection with server')
                if use_gc:
                    self.gc_connect()
                else:
                    self.ldap_connect(resolver=use_resolver)
                # Try again
                return self.ldap_get_single(qobject, attributes, use_gc, use_resolver, is_retry=True)
        except LDAPNoSuchObjectResult:
            # This may indicate the object doesn't exist or access is denied
            logging.warning('LDAP Server reported that the object %s does not exist.', qobject)
            return None
        for e in sresult:
            if e['type'] != 'searchResEntry':
                continue
            return e

    def get_domain_controllers(self):
        entries = self.search('(userAccountControl:1.2.840.113556.1.4.803:=8192)',
                              ['dnshostname', 'samaccounttype', 'samaccountname',
                               'serviceprincipalname', 'objectSid'])

        return entries


    def get_netbios_name(self, context):
        try:
            entries = self.search('(ncname=%s)' % context,
                                  ['nETBIOSName'],
                                  search_base="CN=Partitions,%s" % self.ldap.server.info.other['configurationNamingContext'][0])
        except (LDAPAttributeError, LDAPCursorError) as e:
            logging.warning('Could not determine NetBiosname of the domain: %s', str(e))
        return next(entries)

    def get_objecttype(self):
        """
        Function to get objecttype GUID
        """
        self.objecttype_guid_map = dict()

        if self.ldap is None:
            self.ldap_connect()

        sresult = self.ldap.extend.standard.paged_search(self.ldap.server.info.other['schemaNamingContext'][0],
                                                         '(objectClass=*)',
                                                         attributes=['name', 'schemaidguid'])
        for res in sresult:
            if res['attributes']['schemaIDGUID']:
                guid = str(UUID(bytes_le=res['attributes']['schemaIDGUID']))
                self.objecttype_guid_map[res['attributes']['name'].lower()] = guid

        if 'ms-mcs-admpwdexpirationtime' in self.objecttype_guid_map:
            logging.debug('Found LAPS attributes in schema')
            self.ad.has_laps = True
        if 'ms-laps-passwordexpirationtime' in self.objecttype_guid_map:
            logging.debug('Found LAPSv2 attributes in schema')
            self.ad.has_lapsv2 = True
        if not self.ad.has_laps and not self.ad.has_lapsv2:
            logging.debug('No LAPS attributes found in schema')

        if 'ms-ds-key-credential-link' in self.objecttype_guid_map:
            logging.debug('Found KeyCredentialLink attributes in schema')
            self.ad.has_keycredlink = True
        else:
            logging.debug('No KeyCredentialLink attributes found in schema')

    def get_domains(self, acl=False):
        """
        Function to get domains. This should only return the current domain.
        """
        entries = self.search('(objectClass=domain)',
                              [],
                              generator=True,
                              query_sd=acl)

        entriesNum = 0
        for entry in entries:
            entriesNum += 1
            # Todo: actually use these objects instead of discarding them
            # means rewriting other functions
            domain_object = ADDomain.fromLDAP(entry['attributes']['distinguishedName'], entry['attributes']['objectSid'])
            self.ad.domain_object = domain_object
            self.ad.domains[entry['attributes']['distinguishedName']] = entry
            try:
                nbentry = self.get_netbios_name(entry['attributes']['distinguishedName'])
                self.ad.nbdomains[nbentry['attributes']['nETBIOSName']] = entry
            except IndexError:
                pass

        if entriesNum == 0:
            # Raise exception if we somehow managed to authenticate but the domain is wrong
            # prevents confusing exceptions later
            actualdn = self.ldap.server.info.other['defaultNamingContext'][0]
            actualdomain = ADUtils.ldap2domain(actualdn)
            logging.error('Could not find the requested domain %s on this DC, LDAP server reports is domain as %s (you may want to try that?)', self.ad.domain, actualdomain)
            raise CollectionException("Specified domain was not found in LDAP")

        logging.info('Found %u domains', entriesNum)

        return entries

    def get_forest_domains(self):
        """
        Function which searches the LDAP references in order to find domains.
        I'm not sure if this is the best function but couldn't find anything better.

        This searches the configuration, which is present only once in the forest but is replicated
        to every DC.
        """
        entries = self.search('(objectClass=crossRef)',
                              ['nETBIOSName', 'systemFlags', 'nCName', 'name'],
                              search_base="CN=Partitions,%s" % self.ldap.server.info.other['configurationNamingContext'][0],
                              generator=True)

        entriesNum = 0
        for entry in entries:
            # Ensure systemFlags entry is not empty before running the naming context check.
            if not entry['attributes']['systemFlags']:
                continue
            # This is a naming context, but not a domain
            if not entry['attributes']['systemFlags'] & 2:
                continue
            entry['attributes']['distinguishedName'] = entry['attributes']['nCName']
            entriesNum += 1
            # Todo: actually use these objects instead of discarding them
            # means rewriting other functions
            d = ADDomain.fromLDAP(entry['attributes']['nCName'])
            # We don't want to add our own domain since this entry doesn't contain the sid
            # which we need later on
            if entry['attributes']['nCName'] not in self.ad.domains:
                self.ad.domains[entry['attributes']['nCName']] = entry
                self.ad.nbdomains[entry['attributes']['nETBIOSName']] = entry

        # Store this number so we can easily determine if we are in a multi-domain
        # forest later on.
        self.ad.num_domains = entriesNum
        logging.info('Found %u domains in the forest', entriesNum)

    def get_cache_items(self):
        self.get_objecttype()
        self.get_forest_domains()
        self.gc_connect()
        sidcache = {}
        dncache = {}
        for nc, domain in self.ad.domains.items():
            logging.info('Processing domain %s', domain['attributes']['name'])
            query = '(|(&(objectCategory=person)(objectClass=user))(objectClass=group)(&(sAMAccountType=805306369)(!(UserAccountControl:1.2.840.113556.1.4.803:=2))))'
            entries = self.search(query,
                                  use_gc=True,
                                  use_resolver=True,
                                  attributes=['sAMAccountName', 'distinguishedName', 'sAMAccountType', 'objectSid', 'name'],
                                  search_base=nc,
                                  generator=True)
            for lentry in entries:
                resolved_entry = ADUtils.resolve_ad_entry(lentry)
                cacheitem = {
                    "ObjectIdentifier": resolved_entry['objectid'],
                    "ObjectType": resolved_entry['type'].capitalize()
                }
                sidcache[resolved_entry['objectid']] = cacheitem
                dncache[ADUtils.get_entry_property(lentry, 'distinguishedName').upper()] = cacheitem
        return dncache, sidcache

    def get_groups(self, include_properties=False, acl=False):
        properties = ['distinguishedName', 'samaccountname', 'samaccounttype', 'objectsid', 'member']
        if include_properties:
            properties += ['adminCount', 'description', 'whencreated']
        if acl:
            properties += ['nTSecurityDescriptor']
        entries = self.search('(objectClass=group)',
                              properties,
                              generator=True,
                              query_sd=acl)
        return entries

    def get_gpos(self, include_properties=False, acl=False):
        properties = ['distinguishedName', 'name', 'objectGUID', 'gPCFileSysPath', 'displayName']
        if include_properties:
            properties += ['description', 'whencreated']
        if acl:
            properties += ['nTSecurityDescriptor']
        entries = self.search('(objectCategory=groupPolicyContainer)',
                              properties,
                              generator=True,
                              query_sd=acl)
        return entries

    def get_ous(self, include_properties=False, acl=False):
        properties = ['distinguishedName', 'name', 'objectGUID', 'gPLink', 'gPOptions']
        if include_properties:
            properties += ['description', 'whencreated']
        if acl:
            properties += ['nTSecurityDescriptor']
        entries = self.search('(objectCategory=organizationalUnit)',
                              properties,
                              generator=True,
                              query_sd=acl)
        return entries

    def get_containers(self, include_properties=False, acl=False, dn=''):
        properties = ['distinguishedName', 'name', 'objectGUID', 'isCriticalSystemObject','objectClass', 'objectCategory']
        if include_properties:
            properties += ['description', 'whencreated']
        if acl:
            properties += ['nTSecurityDescriptor']
        entries = self.search('(&(objectCategory=container)(objectClass=container))',
                              properties,
                              generator=True,
                              query_sd=acl,
                              search_base=dn)
        return entries

    def get_users(self, include_properties=False, acl=False):

        properties = ['sAMAccountName', 'distinguishedName', 'sAMAccountType',
                      'objectSid', 'primaryGroupID', 'isDeleted', 'objectClass']
        if 'ms-DS-GroupMSAMembership'.lower() in self.objecttype_guid_map:
            properties.append('msDS-GroupMSAMembership')

        if include_properties:
            properties += ['servicePrincipalName', 'userAccountControl', 'displayName',
                           'lastLogon', 'lastLogonTimestamp', 'pwdLastSet', 'mail', 'title', 'homeDirectory',
                           'description', 'userPassword', 'adminCount', 'msDS-AllowedToDelegateTo', 'sIDHistory',
                           'whencreated', 'unicodepwd', 'scriptpath']
            if 'unixuserpassword' in self.objecttype_guid_map:
                properties.append('unixuserpassword')
        if acl:
            properties.append('nTSecurityDescriptor')

        # Query for MSA only if server supports it
        if 'msDS-GroupManagedServiceAccount' in self.ldap.server.schema.object_classes:
            gmsa_filter = '(objectClass=msDS-GroupManagedServiceAccount)'
        else:
            logging.debug('No support for GMSA, skipping in query')
            gmsa_filter = ''

        if 'msDS-ManagedServiceAccount' in self.ldap.server.schema.object_classes:
            smsa_filter = '(objectClass=msDS-ManagedServiceAccount)'
        else:
            logging.debug('No support for SMSA, skipping in query')
            smsa_filter = ''

        if gmsa_filter or smsa_filter:
            query = '(|(&(objectCategory=person)(objectClass=user)){}{})'.format(gmsa_filter, smsa_filter)
        else:
            query = '(&(objectCategory=person)(objectClass=user))'
        entries = self.search(query,
                              properties,
                              generator=True,
                              query_sd=acl)
        return entries


    def get_computers(self, include_properties=False, acl=False):
        """
        Get all computer objects. This purely gets them using LDAP. This function is used directly in case of DCOnly enum,
        or used to create a cache in case of computer enumeration later on.
        """
        properties = ['samaccountname', 'userAccountControl', 'distinguishedname',
                      'dnshostname', 'samaccounttype', 'objectSid', 'primaryGroupID',
                      'isDeleted']
        if include_properties:
            properties += ['servicePrincipalName', 'msDS-AllowedToDelegateTo', 'sIDHistory', 'whencreated',
                           'lastLogon', 'lastLogonTimestamp', 'pwdLastSet', 'operatingSystem', 'description',
                           'operatingSystemServicePack', 'operatingSystemVersion']
            # Difference between guid map which maps the lowercase schema object name and the property name itself
            if 'ms-DS-Allowed-To-Act-On-Behalf-Of-Other-Identity'.lower() in self.objecttype_guid_map:
                properties.append('msDS-AllowedToActOnBehalfOfOtherIdentity')
            if self.ad.has_laps:
                properties.append('ms-mcs-admpwdexpirationtime')
            if self.ad.has_lapsv2:
                properties.append('mslaps-passwordexpirationtime')
        if acl:
            # Also collect LAPS expiration time since this matters for reporting (no LAPS = no ACL reported)
            if self.ad.has_laps:
                properties.append('ms-mcs-admpwdexpirationtime')
            if self.ad.has_lapsv2:
                properties.append('mslaps-passwordexpirationtime')
            properties.append('nTSecurityDescriptor')

        # Exclude MSA only if server supports it
        if 'msDS-GroupManagedServiceAccount' in self.ldap.server.schema.object_classes:
            gmsa_filter = '(!(objectClass=msDS-GroupManagedServiceAccount))'
        else:
            gmsa_filter = ''

        if 'msDS-ManagedServiceAccount' in self.ldap.server.schema.object_classes:
            smsa_filter = '(!(objectClass=msDS-ManagedServiceAccount))'
        else:
            smsa_filter = ''

        if gmsa_filter or smsa_filter:
            query = '(&(sAMAccountType=805306369){}{})'.format(gmsa_filter, smsa_filter)
        else:
            query = '(&(sAMAccountType=805306369))'

        entries = self.search(query,
                              properties,
                              generator=True,
                              query_sd=acl)

        return entries

    def get_computers_withcache(self, include_properties=False, acl=False):
        """
        Get all computer objects. This is the function used when we do computer enumeration, in which case these
        items are cached first. Calls the function with generator above to create the list.
        """

        entries = self.get_computers(include_properties, acl)

        entriesNum = 0
        for entry in entries:
            entriesNum += 1
            # Resolve it first for DN cache
            resolved_entry = ADUtils.resolve_ad_entry(entry)
            cacheitem = {
                "ObjectIdentifier": resolved_entry['objectid'],
                "ObjectType": resolved_entry['type'].capitalize()
            }
            self.ad.dncache[ADUtils.get_entry_property(entry, 'distinguishedName', '').upper()] = cacheitem
            # This list is used to process computers later on
            self.ad.computers[ADUtils.get_entry_property(entry, 'distinguishedName', '')] = entry
            self.ad.computersidcache.put(ADUtils.get_entry_property(entry, 'dNSHostname', '').lower(), entry['attributes']['objectSid'])

        logging.info('Found %u computers', entriesNum)

        return entries

    def get_memberships(self):
        entries = self.search('(|(memberof=*)(primarygroupid=*))',
                              ['samaccountname', 'distinguishedname',
                               'dnshostname', 'samaccounttype', 'primarygroupid',
                               'memberof'],
                              generator=False)
        return entries

    def get_sessions(self):
        entries = self.search('(&(samAccountType=805306368)(!(userAccountControl:1.2.840.113556.1.4.803:=2))(|(homedirectory=*)(scriptpath=*)(profilepath=*)))',
                              ['homedirectory', 'scriptpath', 'profilepath'])
        return entries

    def get_childobjects(self, dn, use_resolver=True):
        entries = self.search('(|(objectClass=container)(objectClass=organizationalUnit)(sAMAccountType=805306369)(objectClass=group)(&(objectCategory=person)(objectClass=user)))',
                              attributes=['objectSid', 'objectClass', 'objectGUID', 'distinguishedName', 'sAMAccountName', 'sAMAccountType'],
                              search_base=dn,
                              search_scope=LEVEL,
                              use_resolver=use_resolver)
                              
        return entries

    def get_trusts(self):
        entries = self.search('(objectClass=trustedDomain)',
                              attributes=['flatName', 'name', 'securityIdentifier', 'trustAttributes', 'trustDirection', 'trustType'],
                              generator=True)
        return entries

    def prefetch_info(self, props=False, acls=False, cache_computers=False):
        self.get_objecttype()
        self.get_domains(acl=acls)
        self.get_forest_domains()
        if cache_computers:
            self.get_computers_withcache(include_properties=props, acl=acls)

    def get_root_domain(self):
        return ADUtils.ldap2domain(self.ldap.server.info.other['configurationNamingContext'][0])


"""
Active Directory data and cache
"""
class AD(object):

    def __init__(self, domain=None, auth=None, nameserver=None, dns_tcp=False, dns_timeout=3.0, use_ldaps=False):
        self.domain = domain
        # Object of type ADDomain, added later
        self.domain_object = None
        self.auth = auth
        # List of DCs for this domain. Contains just one DC since
        # we query for the primary DC specifically
        self._dcs = []
        # Kerberos servers
        self._kdcs = []
        # Global catalog servers
        self._gcs = []

        self.domains = {}
        self.nbdomains = {}
        self.groups = {} # Groups by DN
        self.groups_dnmap = {} # Group mapping from gid to DN
        self.computers = {}
        self.users = {} # Users by DN

        # Create a resolver object
        self.dnsresolver = resolver.Resolver()
        if nameserver:
            self.dnsresolver.nameservers = [nameserver]
        # Resolve DNS over TCP?
        self.dns_tcp = dns_tcp
        # Give it a cache to prevent duplicate lookups
        self.dnsresolver.cache = resolver.Cache()
        # Default timeout after 3 seconds if the DNS servers
        # do not come up with an answer
        self.dnsresolver.timeout = float(dns_timeout)
        self.dnsresolver.lifetime = float(dns_timeout)
        # Also create a custom cache for both forward and backward lookups
        # this cache is thread-safe
        self.dnscache = DNSCache()
        # Create a thread-safe SID lookup cache
        self.sidcache = SidCache()
        # Create a thread-safe SAM lookup cache
        self.samcache = SamCache()
        # DN cache dict - use generic cache for all objects
        # holds only direct bloodhound output
        self.dncache = {}
        self.newsidcache = SidCache()
        # Create SID cache for computer accounts
        self.computersidcache = SidCache()
        # Object Resolver, initialized later
        self.objectresolver = None
        # Number of domains within the forest
        self.num_domains = 1
        # Does the schema have laps properties
        self.has_laps = False
        # Does the schema have lapsv2 properties
        self.has_lapsv2 = False
        # Does the schema have msDS-KeyCredentialLink
        self.has_keycredlink = False
        if domain is not None:
            self.baseDN = ADUtils.domain2ldap(domain)
        else:
            self.baseDN = None
        if use_ldaps:
            self.ldap_default_protocol = 'ldaps'
        else:
            self.ldap_default_protocol = 'ldap'

    def realm(self):
        if self.domain is not None:
            return self.domain.upper()
        else:
            return None

    def override_dc(self, dcname):
        self._dcs = [dcname]

    def override_gc(self, gcname):
        self._gcs = [gcname]

    def dcs(self):
        return self._dcs

    def gcs(self):
        return self._gcs

    def kdcs(self):
        return self._kdcs

    def create_objectresolver(self, addc):
        self.objectresolver = ObjectResolver(addomain=self, addc=addc)

    def load_cachefile(self, cachefile):
        with codecs.open(cachefile, 'r', 'utf-8') as cfile:
            cachedata = json.load(cfile)
        self.dncache = cachedata['dncache']
        self.newsidcache.load(cachedata['sidcache'])
        logging.info('Loaded cached DNs and SIDs from cachefile')

    def save_cachefile(self, cachefile):
        pass

    def dns_resolve(self, domain=None, options=None):
        logging.debug('Querying domain controller information from DNS')

        basequery = '_ldap._tcp.pdc._msdcs'
        ad_domain = None

        if domain is not None:
            logging.debug('Using domain hint: %s' % str(domain))
            query = '_ldap._tcp.pdc._msdcs.%s.' % domain
        else:
            # Assume a DNS search domain is (correctly) configured on the host
            # in which case the resolver will autocomplete our request
            query = basequery

        try:

            q = self.dnsresolver.query(query, 'SRV', tcp=self.dns_tcp)

            if str(q.qname).lower().startswith('_ldap._tcp.pdc._msdcs'):
                ad_domain = str(q.qname).lower()[len(basequery):].strip('.')
                logging.info('Found AD domain: %s' % ad_domain)

                self.domain = ad_domain
                if self.auth.domain is None:
                    self.auth.domain = ad_domain
                self.baseDN = ADUtils.domain2ldap(ad_domain)

            for r in q:
                dc = str(r.target).rstrip('.')
                logging.debug('Found primary DC: %s' % dc)
                if dc not in self._dcs:
                    self._dcs.append(dc)

        except resolver.NXDOMAIN:
            pass

        try:
            q = self.dnsresolver.query(query.replace('pdc','gc'), 'SRV', tcp=self.dns_tcp)
            for r in q:
                gc = str(r.target).rstrip('.')
                logging.debug('Found Global Catalog server: %s' % gc)
                if gc not in self._gcs:
                    self._gcs.append(gc)

        except resolver.NXDOMAIN:
            # Only show warning if we don't already have a GC specified manually
            if options and not options.global_catalog:
                if not options.disable_autogc:
                    logging.warning('Could not find a global catalog server, assuming the primary DC has this role\n'
                                    'If this gives errors, either specify a hostname with -gc or disable gc resolution with --disable-autogc')
                    self._gcs = self._dcs
                else:
                    logging.warning('Could not find a global catalog server. Please specify one with -gc')

        try:
            kquery = query.replace('pdc','dc').replace('_ldap','_kerberos')
            q = self.dnsresolver.query(kquery, 'SRV', tcp=self.dns_tcp)
            # TODO: Get the additional records here to get the DC ip immediately
            for r in q:
                kdc = str(r.target).rstrip('.')
                logging.debug('Found KDC for enumeration domain: %s' % str(r.target).rstrip('.'))
                if kdc not in self._kdcs:
                    self._kdcs.append(kdc)
                    self.auth.kdc = self._kdcs[0]
        except resolver.NXDOMAIN:
            pass

        # Fix for below in case domain was not found via DNS
        if not ad_domain:
            if domain:
                ad_domain = domain
            else:
                ad_domain = 'unknown'

        if self.auth.userdomain.lower() != ad_domain.lower():
            # Resolve KDC for user auth domain
            kquery = '_kerberos._tcp.dc._msdcs.%s.' % self.auth.userdomain
            q = self.dnsresolver.query(kquery, 'SRV', tcp=self.dns_tcp)
            for r in q:
                kdc = str(r.target).rstrip('.')
                logging.debug('Found KDC for user: %s' % str(r.target).rstrip('.'))
                self.auth.userdomain_kdc = kdc
        else:
            self.auth.userdomain_kdc = self.auth.kdc

        return True


    def get_domain_by_name(self, name):
        for domain, entry in self.domains.items():
            if 'name' in entry['attributes']:
                if entry['attributes']['name'].upper() == name.upper():
                    return entry
        # Also try domains by NETBIOS definition
        for domain, entry in self.nbdomains.items():
            if domain.upper() == name.upper():
                return entry
        return None


    def get_dn_from_cache_or_ldap(self, distinguishedname):
        try:
            linkentry = self.dncache[distinguishedname.upper()]
        except KeyError:
            use_gc = ADUtils.ldap2domain(distinguishedname).lower() != self.domain.lower()
            qobject = self.objectresolver.resolve_distinguishedname(distinguishedname, use_gc=use_gc)
            if qobject is None:
                return None
            resolved_entry = ADUtils.resolve_ad_entry(qobject)
            if not resolved_entry['objectid']:
                return None
            linkentry = {
                "ObjectIdentifier": resolved_entry['objectid'],
                "ObjectType": resolved_entry['type'].capitalize()
            }
            self.dncache[distinguishedname.upper()] = linkentry
        return linkentry

"""
Active Directory Domain
"""
class ADDomain(object):
    def __init__(self, name=None, netbios_name=None, sid=None, distinguishedname=None):
        self.name = name
        self.netbios_name = netbios_name
        self.sid = sid
        self.distinguishedname = distinguishedname


    @staticmethod
    def fromLDAP(identifier, sid=None):
        dns_name = ADUtils.ldap2domain(identifier)
        return ADDomain(name=dns_name, sid=sid, distinguishedname=identifier)
