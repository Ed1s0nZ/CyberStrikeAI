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

import os, sys, logging, argparse, getpass, time, re, datetime
from zipfile import ZipFile
from bloodhound.ad.domain import AD, ADDC
from bloodhound.ad.authentication import ADAuthentication
from bloodhound.enumeration.computers import ComputerEnumerator
from bloodhound.enumeration.memberships import MembershipEnumerator
from bloodhound.enumeration.domains import DomainEnumerator

"""
BloodHound.py is a Python port of BloodHound, designed to run on Linux and Windows.
"""
class BloodHound(object):
    def __init__(self, ad):
        self.ad = ad
        self.ldap = None
        self.pdc = None
        self.sessions = []


    def connect(self):
        if len(self.ad.dcs()) == 0:
            logging.error('Could not find a domain controller. Consider specifying a domain and/or DNS server.')
            sys.exit(1)

        if not self.ad.baseDN:
            logging.error('Could not figure out the domain to query. Please specify this manually with -d')
            sys.exit(1)

        pdc = self.ad.dcs()[0]
        logging.debug('Using LDAP server: %s', pdc)
        logging.debug('Using base DN: %s', self.ad.baseDN)

        if len(self.ad.kdcs()) > 0:
            kdc = self.ad.auth.kdc
            logging.debug('Using kerberos KDC: %s', kdc)
            logging.debug('Using kerberos realm: %s', self.ad.realm())

        # Create a domain controller object
        self.pdc = ADDC(pdc, self.ad)
        # Create an object resolver
        self.ad.create_objectresolver(self.pdc)


    def run(self, collect, num_workers=10, disable_pooling=False, timestamp="", computerfile="", cachefile=None, exclude_dcs=False, fileNamePrefix=""):
        start_time = time.time()
        if cachefile:
            self.ad.load_cachefile(cachefile)

        # Check early if we should enumerate computers as well
        do_computer_enum = any(method in collect for method in ['localadmin', 'session', 'loggedon', 'experimental', 'rdp', 'dcom', 'psremote'])

        if 'group' in collect or 'objectprops' in collect or 'acl' in collect:
            # Fetch domains for later, computers if needed
            self.pdc.prefetch_info('objectprops' in collect, 'acl' in collect, cache_computers=do_computer_enum)
            # Initialize enumerator
            membership_enum = MembershipEnumerator(self.ad, self.pdc, collect, disable_pooling)
            membership_enum.enumerate_memberships(timestamp=timestamp, fileNamePrefix=fileNamePrefix)
        elif 'container' in collect:
            # Fetch domains for later, computers if needed
            self.pdc.prefetch_info('objectprops' in collect, 'acl' in collect, cache_computers=do_computer_enum)
            # Initialize enumerator
            membership_enum = MembershipEnumerator(self.ad, self.pdc, collect, disable_pooling)
            membership_enum.do_container_collection(timestamp=timestamp)
        elif do_computer_enum:
            # We need to know which computers to query regardless
            # We also need the domains to have a mapping from NETBIOS -> FQDN for local admins
            self.pdc.prefetch_info('objectprops' in collect, 'acl' in collect, cache_computers=True)
        elif 'trusts' in collect:
            # Prefetch domains
            self.pdc.get_domains('acl' in collect)
        if 'trusts' in collect or 'acl' in collect or 'objectprops' in collect:
            trusts_enum = DomainEnumerator(self.ad, self.pdc)
            trusts_enum.dump_domain(collect,timestamp=timestamp,fileNamePrefix=fileNamePrefix)
        if do_computer_enum:
            # If we don't have a GC server, don't use it for deconflictation
            have_gc = len(self.ad.gcs()) > 0
            computer_enum = ComputerEnumerator(self.ad, self.pdc, collect, do_gc_lookup=have_gc, computerfile=computerfile, exclude_dcs=exclude_dcs)
            computer_enum.enumerate_computers(self.ad.computers, num_workers=num_workers, timestamp=timestamp, fileNamePrefix=fileNamePrefix)
        end_time = time.time()
        minutes, seconds = divmod(int(end_time-start_time),60)
        logging.info('Done in %02dM %02dS' % (minutes, seconds))

def resolve_collection_methods(methods):
    """
    Convert methods (string) to list of validated methods to resolve
    """
    valid_methods = ['group', 'localadmin', 'session', 'trusts', 'default', 'all', 'loggedon',
                     'objectprops', 'experimental', 'acl', 'dcom', 'rdp', 'psremote', 'dconly',
                     'container']
    default_methods = ['group', 'localadmin', 'session', 'trusts']
    # Similar to SharpHound, All is not really all, it excludes loggedon
    all_methods = ['group', 'localadmin', 'session', 'trusts', 'objectprops', 'acl', 'dcom', 'rdp', 'psremote', 'container']
    # DC only, does not collect to computers
    dconly_methods = ['group', 'trusts', 'objectprops', 'acl', 'container']
    if ',' in methods:
        method_list = [method.lower() for method in methods.split(',')]
        validated_methods = []
        for method in method_list:
            if method not in valid_methods:
                logging.error('Invalid collection method specified: %s', method)
                return False

            if method == 'default':
                validated_methods += default_methods
            elif method == 'all':
                validated_methods += all_methods
            elif method == 'dconly':
                validated_methods += dconly_methods
            else:
                validated_methods.append(method)
        return set(validated_methods)
    else:
        validated_methods = []
        # It is only one
        method = methods.lower()
        if method in valid_methods:
            if method == 'default':
                validated_methods += default_methods
            elif method == 'all':
                validated_methods += all_methods
            elif method == 'dconly':
                validated_methods += dconly_methods
            else:
                validated_methods.append(method)
            return set(validated_methods)
        else:
            logging.error('Invalid collection method specified: %s', method)
            return False

def main():
#    logging.basicConfig(stream=sys.stderr, level=logging.INFO)

    logger = logging.getLogger()
    logger.setLevel(logging.INFO)
    stream = logging.StreamHandler(sys.stderr)
    stream.setLevel(logging.DEBUG)
    formatter = logging.Formatter('%(levelname)s: %(message)s')
#    formatter = logging.Formatter('%(asctime)s %(levelname)s: %(message)s')
    stream.setFormatter(formatter)
    logger.addHandler(stream)

    parser = argparse.ArgumentParser(add_help=True, description='Python based ingestor for BloodHound LEGACY\nFor help or reporting issues, visit https://github.com/dirkjanm/BloodHound.py', formatter_class=argparse.RawDescriptionHelpFormatter)

    parser.add_argument('-c',
                        '--collectionmethod',
                        action='store',
                        default='Default',
                        help='Which information to collect. Supported: Group, LocalAdmin, Session, '
                             'Trusts, Default (all previous), DCOnly (no computer connections), DCOM, RDP,'
                             'PSRemote, LoggedOn, Container, ObjectProps, ACL, All (all except LoggedOn). '
                             'You can specify more than one by separating them with a comma. (default: Default)')
    parser.add_argument('-d',
                        '--domain',
                        action='store',
                        default='',
                        help='Domain to query.')
    parser.add_argument('-v',
                        action='store_true',
                        help='Enable verbose output')
    helptext = 'Specify one or more authentication options. \n' \
               'By default Kerberos authentication is used and NTLM is used as fallback. \n' \
               'Kerberos tickets are automatically requested if a password or hashes are specified.'
    auopts = parser.add_argument_group('authentication options', description=helptext)
    auopts.add_argument('-u',
                        '--username',
                        action='store',
                        help='Username. Format: username[@domain]; If the domain is unspecified, the current domain is used.')
    auopts.add_argument('-p',
                        '--password',
                        action='store',
                        help='Password')
    auopts.add_argument('-k',
                        '--kerberos',
                        action='store_true',
                        help='Use kerberos ccache file')
    auopts.add_argument('--hashes',
                        action='store',
                        help='LM:NLTM hashes')
    auopts.add_argument('-no-pass', action="store_true", help='don\'t ask for password (useful for -k)')
    auopts.add_argument('-aesKey',
                        action="store",
                        metavar="hex key",
                        help='AES key to use for Kerberos Authentication (128 or 256 bits)')
    auopts.add_argument('--auth-method',
                        choices=('auto','ntlm','kerberos'),
                        default='auto',
                        action='store',
                        help='Authentication methods. Force Kerberos or NTLM only or use auto for Kerberos with NTLM fallback')
    coopts = parser.add_argument_group('collection options')
    coopts.add_argument('-ns',
                        '--nameserver',
                        action='store',
                        help='Alternative name server to use for queries')
    coopts.add_argument('--dns-tcp',
                        action='store_true',
                        help='Use TCP instead of UDP for DNS queries')
    coopts.add_argument('--dns-timeout',
                        action='store',
                        type=int,
                        default=3,
                        help='DNS query timeout in seconds (default: 3)')
    coopts.add_argument('-dc',
                        '--domain-controller',
                        metavar='HOST',
                        action='store',
                        help='Override which DC to query (hostname)')
    coopts.add_argument('-gc',
                        '--global-catalog',
                        metavar='HOST',
                        action='store',
                        help='Override which GC to query (hostname)')
    coopts.add_argument('-w',
                        '--workers',
                        action='store',
                        type=int,
                        default=10,
                        help='Number of workers for computer enumeration (default: 10)')
    coopts.add_argument('--exclude-dcs',
                        action='store_true',
                        help='Skip DCs during computer enumeration')
    coopts.add_argument('--disable-pooling',
                        action='store_true',
                        help='Don\'t use subprocesses for ACL parsing (only for debugging purposes)')
    coopts.add_argument('--disable-autogc',
                        action='store_true',
                        help='Don\'t automatically select a Global Catalog (use only if it gives errors)')
    coopts.add_argument('--zip',
                        action='store_true',
                        help='Compress the JSON output files into a zip archive')
    coopts.add_argument('--computerfile',
                        action='store',
                        help='File containing computer FQDNs to use as allowlist for any computer based methods')
    coopts.add_argument('--cachefile',
                        action='store',
                        help='Cache file (experimental)')
    coopts.add_argument('--ldap-channel-binding',
                        action='store_true',
                        help='Use LDAP Channel Binding (will force ldaps protocol to be used)')
    coopts.add_argument('--use-ldaps',
                        action='store_true',
                        help='Use LDAP over TLS on port 636 by default')
    coopts.add_argument('-op',
                        '--outputprefix',
                        metavar='PREFIX_NAME',
                        action='store',
                        help='String to prepend to output file names')

    args = parser.parse_args()
    logging.info('BloodHound.py for BloodHound LEGACY (BloodHound 4.2 and 4.3)')

    if args.v is True:
        logger.setLevel(logging.DEBUG)

    if args.username is not None and args.password is not None:
        logging.debug('Authentication: username/password')
        auth = ADAuthentication(username=args.username, password=args.password, domain=args.domain, auth_method=args.auth_method, ldap_channel_binding=args.ldap_channel_binding)
    elif args.username is not None and args.password is None and args.hashes is None and args.aesKey is None and not args.no_pass:
        args.password = getpass.getpass()
        auth = ADAuthentication(username=args.username, password=args.password, domain=args.domain, auth_method=args.auth_method, ldap_channel_binding=args.ldap_channel_binding)
    elif args.username is None and (args.password is not None or args.hashes is not None):
        logging.error('Authentication: password or hashes provided without username')
        sys.exit(1)
    elif args.hashes is not None and args.username is not None:
        logging.debug('Authentication: NT hash')
        lm, nt = args.hashes.split(":")
        auth = ADAuthentication(lm_hash=lm, nt_hash=nt, username=args.username, domain=args.domain, auth_method=args.auth_method, ldap_channel_binding=args.ldap_channel_binding)
    elif args.aesKey is not None and args.username is not None:
        logging.debug('Authentication: Kerberos AES')
        auth = ADAuthentication(username=args.username, domain=args.domain, aeskey=args.aesKey, auth_method=args.auth_method, ldap_channel_binding=args.ldap_channel_binding)
    else:
        if not args.kerberos:
            parser.print_help()
            sys.exit(1)
        else:
            if not args.username:
                logging.error('Specifying the username explicitly is required for Kerberos ccache authentication')
                sys.exit(1)
            logging.debug('Authentication: Kerberos ccache')
            args.auth_method = 'kerberos'
            auth = ADAuthentication(username=args.username, password=args.password, domain=args.domain, auth_method=args.auth_method, ldap_channel_binding=args.ldap_channel_binding)

    ad = AD(auth=auth, domain=args.domain, nameserver=args.nameserver, dns_tcp=args.dns_tcp, dns_timeout=args.dns_timeout, use_ldaps=args.use_ldaps)
    # Resolve collection methods
    collect = resolve_collection_methods(args.collectionmethod)
    if not collect:
        return
    logging.debug('Resolved collection methods: %s', ', '.join(list(collect)))

    logging.debug('Using DNS to retrieve domain information')
    ad.dns_resolve(domain=args.domain, options=args)

    # Override the detected DC / GC if specified
    if args.domain_controller:
        if re.match(r'\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}', args.domain_controller):
            logging.error('The specified domain controller %s looks like an IP address, but requires a hostname (FQDN).\n'\
                          'Use the -ns flag to specify a DNS server IP if the hostname does not resolve on your default nameserver.',
                          args.domain_controller)
            sys.exit(1)
        ad.override_dc(args.domain_controller)
        logging.debug('Using supplied domain controller as KDC')
        auth.set_kdc(args.domain_controller)

    if args.global_catalog:
        if re.match(r'\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}', args.global_catalog):
            logging.error('The specified global catalog server %s looks like an IP address, but requires a hostname (FQDN).\n'\
                          'Use the -ns flag to specify a DNS server IP if the hostname does not resolve on your default nameserver.',
                          args.global_catalog)
            sys.exit(1)
        ad.override_gc(args.global_catalog)

    if args.auth_method in ('auto', 'kerberos'):
        if args.kerberos is True:
            if not auth.load_ccache():
                logging.debug('Could not load ticket from ccache, trying to request a TGT instead')
                auth.get_tgt()
        else:
            auth.get_tgt()

    # For adding timestamp prefix to the outputfiles 
    timestamp = datetime.datetime.fromtimestamp(time.time()).strftime('%Y%m%d%H%M%S') + "_"
    bloodhound = BloodHound(ad)
    bloodhound.connect()
    bloodhound.run(collect=collect,
                   num_workers=args.workers,
                   disable_pooling=args.disable_pooling,
                   timestamp=timestamp,
                   computerfile=args.computerfile,
                   cachefile=args.cachefile,
                   exclude_dcs=args.exclude_dcs,
                   fileNamePrefix=args.outputprefix)
    #If args --zip is true, the compress output  
    if args.zip:
        logging.info("Compressing output into " + timestamp + "bloodhound.zip")
        # Get a list of files in the current dir
        list_of_files = os.listdir(os.getcwd())
        # Create handle to zip file with timestamp prefix
        if(args.outputprefix!=None):
            with ZipFile(args.outputprefix + "_" + timestamp + "bloodhound.zip",'w') as zip:
                # For each of those files we fetched
                for each_file in list_of_files:
                    # If the files starts with the current timestamp and ends in json
                    if each_file.startswith(args.outputprefix) and each_file.endswith("json"):
                        # Write it to the zip
                        zip.write(each_file)
                        # Remove it from disk
                        os.remove(each_file)
        else:
            with ZipFile(timestamp + "bloodhound.zip",'w') as zip:
                # For each of those files we fetched
                for each_file in list_of_files:
                    # If the files starts with the current timestamp and ends in json
                    if each_file.startswith(timestamp) and each_file.endswith("json"):
                        # Write it to the zip
                        zip.write(each_file)
                        # Remove it from disk
                        os.remove(each_file)



if __name__ == '__main__':
    main()
