from __future__ import unicode_literals
import argparse
import os
import logging
import json
import codecs
import re
from ldapdomaindump import trust_flags, trust_directions

logging.basicConfig()
logger = logging.getLogger('ldd2bloodhound')

class Utils():
    @staticmethod
    def ldap_to_domain(ldap):
        return re.sub(',DC=', '.', ldap[ldap.find('DC='):], flags=re.I)[3:]

    @staticmethod
    def get_group_object(groupo, domain):
        return {
            'dn': groupo['dn'],
            'sid': groupo['attributes']['objectSid'][0],
            'type': 'group',
            'principal': '%s@%s' % (groupo['attributes']['sAMAccountName'][0].upper(), domain.upper()),
            'memberOf': groupo['attributes']['memberOf'] if 'memberOf' in groupo['attributes'] else []
        }

class BloodHoundConverter():
    def __init__(self):
        # Input files
        self.computers_files = []
        self.trust_files = []
        self.group_files = []
        self.user_files = []

        # Caches
        self.groups_by_dn = {}
        self.groups_by_sid = {}
        self.domaincache = {}

    # Get domain from sid and dn - use cache if possible
    def get_domain(self, sid, dn):
        dsid = sid.rsplit('-', 1)[0]
        try:
            return self.domaincache[dsid]
        except KeyError:
            self.domaincache[dsid] = Utils.ldap_to_domain(dn)
            return self.domaincache[dsid]

    def build_mappings(self):
        # Parse groups, build DN and SID index
        for file in self.group_files:
            with codecs.open(file, 'r', 'utf-8') as infile:
                data = json.load(infile)
            # data is now a list of groups (objects)
            for group in data:
                groupattrs = Utils.get_group_object(group, self.get_domain(group['attributes']['objectSid'][0], group['dn']))
                self.groups_by_dn[group['dn']] = groupattrs
                self.groups_by_sid[group['attributes']['objectSid'][0]] = groupattrs
        return

    def write_users(self):
        # Read user mapping - write to csv
        with codecs.open('group_membership.csv', 'w', 'utf-8') as outfile:
            outfile.write('GroupName,AccountName,AccountType\n')
            for file in self.user_files:
                with codecs.open(file, 'r', 'utf-8') as infile:
                    data = json.load(infile)
                # data is now a list of users (objects)
                for user in data:
                    self.write_entry_memberships(user, outfile)

    def write_computers(self):
        # Read computer mapping - write to csv
        # file is already created here, we just append
        with codecs.open('group_membership.csv', 'a', 'utf-8') as outfile:
            for file in self.computers_files:
                with codecs.open(file, 'r', 'utf-8') as infile:
                    data = json.load(infile)
                # data is now a list of computers (objects)
                for computer in data:
                    self.write_entry_memberships(computer, outfile, 'computer')

    def write_groups(self):
        # Read group mapping - write to csv
        # file is already created here, we just append
        with codecs.open('group_membership.csv', 'a', 'utf-8') as outfile:
            for group in self.groups_by_dn.values():
                for membergroup in group['memberOf']:
                    try:
                        outfile.write('%s,%s,%s\n' % (self.groups_by_dn[membergroup]['principal'], group['principal'], 'group'))
                    except KeyError:
                        logger.warning('Unknown group %s. Not found in groups cache!', membergroup)

    def write_trusts(self):
        direction_map = {flag:meaning.capitalize() for meaning, flag in trust_directions.items()}
        # open output file first
        with codecs.open('trusts.csv', 'w', 'utf-8') as outfile:
            outfile.write('SourceDomain,TargetDomain,TrustDirection,TrustType,Transitive\n')
            for file in self.trust_files:
                # load the trusts from file
                with codecs.open(file, 'r', 'utf-8') as infile:
                    data = json.load(infile)
                # data is now a list of trusts (objects)
                for trust in data:
                    # process flags similar to BloodHound.py
                    flags = trust['attributes']['trustAttributes'][0]
                    if flags & trust_flags['WITHIN_FOREST']:
                        trustType = 'ParentChild'
                    else:
                        trustType = 'External'
                    if flags & trust_flags['NON_TRANSITIVE']:
                        isTransitive = False
                    else:
                        isTransitive = True
                    out = [
                        Utils.ldap_to_domain(trust['dn']),
                        trust['attributes']['name'][0],
                        direction_map[trust['attributes']['trustDirection'][0]],
                        trustType,
                        str(isTransitive)
                    ]
                    outfile.write(','.join(out) + '\n')

    def write_entry_memberships(self, entry, outfile, entry_type='user'):
        domain = self.get_domain(entry['attributes']['objectSid'][0], entry['dn'])
        if entry_type == 'user':
            principal = '%s@%s' % (entry['attributes']['sAMAccountName'][0].upper(), domain.upper())
        else:
            principal = '%s.%s' % (entry['attributes']['sAMAccountName'][0][:-1].upper(), domain.upper())
        if 'memberOf' in entry['attributes']:
            for group in entry['attributes']['memberOf']:
                try:
                    rgroup = self.groups_by_dn[group]
                    outfile.write('%s,%s,%s\n' % (rgroup['principal'], principal, entry_type))
                except KeyError:
                    logger.warning('Unknown group %s. Not found in groups cache!', group)
        # Now process primary group id
        dsid = entry['attributes']['objectSid'][0].rsplit('-', 1)[0]
        try:
            rgroup = self.groups_by_sid['%s-%d' % (dsid, entry['attributes']['primaryGroupID'][0])]
            outfile.write('%s,%s,%s\n' % (rgroup['principal'], principal, entry_type))
        except KeyError:
            logger.warning('Unknown rid %d. Not found in groups cache!', entry['attributes']['primaryGroupID'][0])

    def parse_files(self, infiles):
        filemap = {
            'domain_users.json': self.user_files,
            'domain_groups.json': self.group_files,
            'domain_trusts.json': self.trust_files,
            'domain_computers.json': self.computers_files,
        }
        for file in infiles:
            # Get the filename
            filename = file.split(os.sep)[-1]
            try:
                filemap[filename.lower()].append(file)
            except KeyError:
                logger.debug('Unknown input file: %s', filename)
        return

def ldd2bloodhound():
    parser = argparse.ArgumentParser(description='LDAPDomainDump to BloodHound CSV converter utility. Supports users/computers/trusts conversion.')

    #Main parameters
    parser.add_argument("files", type=str, nargs='+', metavar='FILENAME', help="The ldapdomaindump json files to load. Required files: domain_users.json and domain_groups.json")
    parser.add_argument("-d", "--debug", action='store_true', help="Enable debug logger")

    args = parser.parse_args()
    if args.debug:
        logger.setLevel(logging.DEBUG)

    converter = BloodHoundConverter()
    converter.parse_files(args.files)
    if len(converter.group_files) == 0:
        logger.error('No domain_groups.json files were specified. Need at least one to perform conversion.')
        return
    if len(converter.user_files) == 0:
        logger.error('No domain_users.json files were specified. Need at least one to perform conversion.')
        return
    logger.debug('Mapping groups...')
    converter.build_mappings()
    logger.debug('Processing users')
    converter.write_users()
    logger.debug('Processing groups')
    converter.write_groups()
    logger.debug('Processing computers')
    converter.write_computers()
    logger.debug('Processing trusts')
    converter.write_trusts()
    print('Done!')

if __name__ == '__main__':
    ldd2bloodhound()
