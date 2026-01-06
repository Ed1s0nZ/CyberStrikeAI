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
import queue
import threading
import calendar
from bloodhound.ad.utils import ADUtils, AceResolver
from bloodhound.ad.computer import ADComputer
from bloodhound.ad.structures import LDAP_SID
from bloodhound.enumeration.acls import AclEnumerator, parse_binary_acl
from bloodhound.enumeration.outputworker import OutputWorker

class MembershipEnumerator(object):
    """
    Class to enumerate memberships in the domain.
    Contains the dumping functions which
    methods from the bloodhound.ad module.
    """
    def __init__(self, addomain, addc, collect, disable_pooling):
        """
        Membership enumeration. Enumerates all groups/users/other memberships.
        """
        self.addomain = addomain
        self.addc = addc
        # Store collection methods specified
        self.collect = collect
        self.disable_pooling = disable_pooling
        self.aclenumerator = AclEnumerator(addomain, addc, collect)
        self.aceresolver = AceResolver(addomain, addomain.objectresolver)
        self.result_q = None

    def get_membership(self, member):
        """
        Attempt to resolve the membership (DN) of a group to an object
        Moved to addomain logic since we need DN resolving in other files
        """
        return self.addomain.get_dn_from_cache_or_ldap(member)

    @staticmethod
    def get_primary_membership(entry):
        """
        Construct primary membership from RID to SID (BloodHound 3.0 only)
        """
        try:
            primarygroupid = int(entry['attributes']['primaryGroupID'])
        except (TypeError, KeyError):
            # Doesn't have a primarygroupid, means it is probably a Group instead of a user
            return None
        return '%s-%d' % ('-'.join(entry['attributes']['objectSid'].split('-')[:-1]), primarygroupid)

    @staticmethod
    def add_user_properties(user, entry):
        """
        Resolve properties for user objects
        """
        props = user['Properties']
        # print entry
        # Is user enabled? Checked by seeing if the UAC flag 2 (ACCOUNT_DISABLED) is not set
        props['enabled'] = ADUtils.get_entry_property(entry, 'userAccountControl', default=0) & 2 == 0
        props['lastlogon'] = ADUtils.win_timestamp_to_unix(
            ADUtils.get_entry_property(entry, 'lastLogon', default=0, raw=True)
        )
        props['lastlogontimestamp'] = ADUtils.win_timestamp_to_unix(
            ADUtils.get_entry_property(entry, 'lastlogontimestamp', default=0, raw=True)
        )
        if props['lastlogontimestamp'] == 0:
            props['lastlogontimestamp'] = -1
        props['pwdlastset'] = ADUtils.win_timestamp_to_unix(
            ADUtils.get_entry_property(entry, 'pwdLastSet', default=0, raw=True)
        )
        props['dontreqpreauth'] = ADUtils.get_entry_property(entry, 'userAccountControl', default=0) & 0x00400000 == 0x00400000
        props['pwdneverexpires'] = ADUtils.get_entry_property(entry, 'userAccountControl', default=0) & 0x00010000 == 0x00010000
        props['sensitive'] = ADUtils.get_entry_property(entry, 'userAccountControl', default=0) & 0x00100000 == 0x00100000
        props['serviceprincipalnames'] = ADUtils.get_entry_property(entry, 'servicePrincipalName', [])
        props['hasspn'] = len(props['serviceprincipalnames']) > 0
        props['displayname'] = ADUtils.get_entry_property(entry, 'displayName')
        props['email'] = ADUtils.get_entry_property(entry, 'mail')
        props['title'] = ADUtils.get_entry_property(entry, 'title')
        props['homedirectory'] = ADUtils.get_entry_property(entry, 'homeDirectory')
        props['description'] = ADUtils.get_entry_property(entry, 'description')
        props['userpassword'] = ADUtils.ensure_string(ADUtils.get_entry_property(entry, 'userPassword'))
        props['admincount'] = ADUtils.get_entry_property(entry, 'adminCount', 0) == 1
        if len(ADUtils.get_entry_property(entry, 'msDS-AllowedToDelegateTo', [])) > 0:
            props['allowedtodelegate'] = ADUtils.get_entry_property(entry, 'msDS-AllowedToDelegateTo', [])
        props['sidhistory'] = [LDAP_SID(bsid).formatCanonical() for bsid in ADUtils.get_entry_property(entry, 'sIDHistory', [])]
        # v4 props
        whencreated = ADUtils.get_entry_property(entry, 'whencreated', default=0)
        if isinstance(whencreated, int):
            props['whencreated'] = whencreated
        else:
            props['whencreated'] = calendar.timegm(whencreated.timetuple())
        props['unixpassword'] = ADUtils.ensure_string(ADUtils.get_entry_property(entry, 'unixuserpassword'))
        props['unicodepassword'] = ADUtils.ensure_string(ADUtils.get_entry_property(entry, 'unicodepwd'))
        props['logonscript'] = ADUtils.ensure_string(ADUtils.get_entry_property(entry, 'scriptpath'))
        props['samaccountname'] = ADUtils.ensure_string(ADUtils.get_entry_property(entry, 'sAMAccountName'))
        # Non-default schema?
        # props['sfupassword'] = ADUtils.ensure_string(ADUtils.get_entry_property(entry, 'msSFU30Password'))
        props['sfupassword'] = None

    def enumerate_users(self, timestamp="", fileNamePrefix=""):
        if (fileNamePrefix != None):
            filename = fileNamePrefix + "_" + timestamp + 'users.json'
        else:
            filename = timestamp + 'users.json'

        # Should we include extra properties in the query?
        with_properties = 'objectprops' in self.collect
        acl = 'acl' in self.collect
        entries = self.addc.get_users(include_properties=with_properties, acl=acl)

        logging.debug('Writing users to file: %s', filename)

        # Use a separate queue for processing the results
        self.result_q = queue.Queue()
        results_worker = threading.Thread(target=OutputWorker.membership_write_worker, args=(self.result_q, 'users', filename))
        results_worker.daemon = True
        results_worker.start()

        if acl and not self.disable_pooling:
            self.aclenumerator.init_pool()

        # This loops over a generator, results are fetched from LDAP on the go
        for entry in entries:
            resolved_entry = ADUtils.resolve_ad_entry(entry)
            # Skip trust objects
            if resolved_entry['type'] == 'trustaccount':
                continue
            user = {
                "AllowedToDelegate": [],
                "ObjectIdentifier": ADUtils.get_entry_property(entry, 'objectSid'),
                "PrimaryGroupSID": MembershipEnumerator.get_primary_membership(entry),
                "Properties": {
                    "name": resolved_entry['principal'],
                    "domain": self.addomain.domain.upper(),
                    "domainsid": self.addomain.domain_object.sid,
                    "distinguishedname":ADUtils.get_entry_property(entry, 'distinguishedName').upper(),
                    "unconstraineddelegation": ADUtils.get_entry_property(entry, 'userAccountControl', default=0) & 0x00080000 == 0x00080000,
                    "trustedtoauth": ADUtils.get_entry_property(entry, 'userAccountControl', default=0) & 0x01000000 == 0x01000000,
                    "passwordnotreqd": ADUtils.get_entry_property(entry, 'userAccountControl', default=0) & 0x00000020 == 0x00000020
                },
                "Aces": [],
                "SPNTargets": [],
                "HasSIDHistory": [],
                "IsDeleted": ADUtils.get_entry_property(entry, 'isDeleted', default=False)
            }

            if with_properties:
                MembershipEnumerator.add_user_properties(user, entry)
                if 'allowedtodelegate' in user['Properties']:
                    delegatehosts_cache = []
                    for host in user['Properties']['allowedtodelegate']:
                        try:
                            target = host.split('/')[1]
                        except IndexError:
                            logging.warning('Invalid delegation target: %s', host)
                            continue
                        try:
                            object_sid = self.addomain.computersidcache.get(target.lower())
                            user['AllowedToDelegate'].append({
                                'ObjectIdentifier': object_sid,
                                'ObjectType': ADUtils.resolve_ad_entry(
                                    self.addomain.objectresolver.resolve_sid(object_sid)
                                )['type'],
                            })
                        except KeyError:
                            object_sam = target.upper().split(".")[0].split("\\")[0]
                            if object_sam in delegatehosts_cache: continue
                            delegatehosts_cache.append(object_sam)
                            object_entry = self.addomain.objectresolver.resolve_samname(object_sam + '*', allow_filter=True)
                            if object_entry:
                                object_resolved = ADUtils.resolve_ad_entry(object_entry[0])
                                user['AllowedToDelegate'].append({
                                    'ObjectIdentifier': object_resolved['objectid'],
                                    'ObjectType': object_resolved['type'],
                                })
                # Parse SID history
                if len(user['Properties']['sidhistory']) > 0:
                    for historysid in user['Properties']['sidhistory']:
                        user['HasSIDHistory'].append(self.aceresolver.resolve_sid(historysid))

            # If this is a GMSA, process it's ACL. We don't bother with threads/processes here
            # since these accounts shouldn't be that common and neither should they have very complex
            # DACLs which control who can read their password
            if ADUtils.get_entry_property(entry, 'msDS-GroupMSAMembership', default=b'', raw=True) != b'':
                self.parse_gmsa(user, entry)

            # Cache link entry for membership resolution
            linkentry = {
                "ObjectIdentifier": resolved_entry['objectid'],
                "ObjectType": resolved_entry['type'].capitalize()
            }
            self.addomain.dncache[entry['dn'].upper()] = linkentry

            # If we are enumerating ACLs, we break out of the loop here
            # this is because parsing ACLs is computationally heavy and therefor is done in subprocesses
            if acl:
                if self.disable_pooling:
                    # Debug mode, don't run this pooled since it hides exceptions
                    self.process_acldata(parse_binary_acl(user, 'user', ADUtils.get_entry_property(entry, 'nTSecurityDescriptor', raw=True), self.addc.objecttype_guid_map))
                else:
                    # Process ACLs in separate processes, then call the processing function to resolve entries and write them to file
                    self.aclenumerator.pool.apply_async(parse_binary_acl, args=(user, 'user', ADUtils.get_entry_property(entry, 'nTSecurityDescriptor', raw=True), self.addc.objecttype_guid_map), callback=self.process_acldata)
            else:
                # Write it to the queue -> write to file in separate thread
                # this is solely for consistency with acl parsing, the performance improvement is probably minimal
                self.result_q.put(user)

        self.write_default_users()

        # If we are parsing ACLs, close the parsing pool first
        # then close the result queue and join it
        if acl and not self.disable_pooling:
            self.aclenumerator.pool.close()
            self.aclenumerator.pool.join()
            self.result_q.put(None)
        else:
            self.result_q.put(None)
        self.result_q.join()

        logging.debug('Finished writing users')

    def enumerate_groups(self, timestamp="", fileNamePrefix=""):

        highvalue = ["S-1-5-32-544", "S-1-5-32-550", "S-1-5-32-549", "S-1-5-32-551", "S-1-5-32-548"]

        def is_highvalue(sid):
            if sid.endswith("-512") or sid.endswith("-516") or sid.endswith("-519"):
                return True
            if sid in highvalue:
                return True
            return False

        # Should we include extra properties in the query?
        with_properties = 'objectprops' in self.collect
        acl = 'acl' in self.collect
        if (fileNamePrefix != None):
            filename = fileNamePrefix + "_" + timestamp + 'groups.json'
        else:
            filename = timestamp + 'groups.json'
        entries = self.addc.get_groups(include_properties=with_properties, acl=acl)

        logging.debug('Writing groups to file: %s', filename)

        # Use a separate queue for processing the results
        self.result_q = queue.Queue()
        results_worker = threading.Thread(target=OutputWorker.membership_write_worker, args=(self.result_q, 'groups', filename))
        results_worker.daemon = True
        results_worker.start()

        if acl and not self.disable_pooling:
            self.aclenumerator.init_pool()

        for entry in entries:
            resolved_entry = ADUtils.resolve_ad_entry(entry)
            try:
                sid = entry['attributes']['objectSid']
            except KeyError:
                #Somehow we found a group without a sid?
                logging.warning('Could not determine SID for group %s', entry['attributes']['distinguishedName'])
                continue
            group = {
                "ObjectIdentifier": sid,
                "Properties": {
                    "domain": self.addomain.domain.upper(),
                    "domainsid": self.addomain.domain_object.sid,
                    "highvalue": is_highvalue(sid),
                    "name": resolved_entry['principal'],
                    "distinguishedname": ADUtils.get_entry_property(entry, 'distinguishedName').upper()
                },
                "Members": [],
                "Aces": [],
                "IsDeleted": ADUtils.get_entry_property(entry, 'isDeleted', default=False)
            }
            if sid in ADUtils.WELLKNOWN_SIDS:
                # Prefix it with the domain
                group['ObjectIdentifier'] = '%s-%s' % (self.addomain.domain.upper(), sid)
            if with_properties:
                group['Properties']['admincount'] = ADUtils.get_entry_property(entry, 'adminCount', default=0) == 1
                group['Properties']['description'] = ADUtils.get_entry_property(entry, 'description')
                group['Properties']['samaccountname'] = ADUtils.get_entry_property(entry, 'sAMAccountName')
                whencreated = ADUtils.get_entry_property(entry, 'whencreated', default=0)
                if isinstance(whencreated, int):
                    group['Properties']['whencreated'] = whencreated
                else:
                    group['Properties']['whencreated'] = calendar.timegm(whencreated.timetuple())

            for member in entry['attributes']['member']:
                resolved_member = self.get_membership(member)
                if resolved_member:
                    group['Members'].append(resolved_member)

            # Create cache entry for links
            link_output = {
                "ObjectIdentifier": group['ObjectIdentifier'],
                "ObjectType": 'Group'
            }
            self.addomain.dncache[ADUtils.get_entry_property(entry, 'distinguishedName').upper()] = link_output

            # If we are enumerating ACLs, we break out of the loop here
            # this is because parsing ACLs is computationally heavy and therefor is done in subprocesses
            if acl:
                if self.disable_pooling:
                    # Debug mode, don't run this pooled since it hides exceptions
                    self.process_acldata(parse_binary_acl(group, 'group', ADUtils.get_entry_property(entry, 'nTSecurityDescriptor', raw=True), self.addc.objecttype_guid_map))
                else:
                    # Process ACLs in separate processes, then call the processing function to resolve entries and write them to file
                    self.aclenumerator.pool.apply_async(parse_binary_acl, args=(group, 'group', ADUtils.get_entry_property(entry, 'nTSecurityDescriptor', raw=True), self.addc.objecttype_guid_map), callback=self.process_acldata)
            else:
                # Write it to the queue -> write to file in separate thread
                # this is solely for consistency with acl parsing, the performance improvement is probably minimal
                self.result_q.put(group)

        self.write_default_groups()

        # If we are parsing ACLs, close the parsing pool first
        # then close the result queue and join it
        if acl and not self.disable_pooling:
            self.aclenumerator.pool.close()
            self.aclenumerator.pool.join()
            self.result_q.put(None)
        else:
            self.result_q.put(None)
        self.result_q.join()

        logging.debug('Finished writing groups')

    def enumerate_computers_dconly(self,timestamp ="", fileNamePrefix=""):
        '''
        Enumerate computer objects. This function is only used if no
        collection was requested that required connecting to computers anyway.
        '''
        if (fileNamePrefix != None):
            filename = fileNamePrefix + "_" + timestamp + 'computers.json'
        else:
            filename = timestamp + 'computers.json'
        # Should we include extra properties in the query?
        with_properties = 'objectprops' in self.collect
        acl = 'acl' in self.collect

        entries = self.addc.get_computers(include_properties=with_properties, acl=acl)

        logging.debug('Writing computers ACL to file: %s', filename)

        # Use a separate queue for processing the results
        self.result_q = queue.Queue()
        results_worker = threading.Thread(target=OutputWorker.membership_write_worker, args=(self.result_q, 'computers', filename))
        results_worker.daemon = True
        results_worker.start()

        if acl and not self.disable_pooling:
            self.aclenumerator.init_pool()

        # This loops over the cached entries
        for entry in entries:
            if not 'attributes' in entry:
                continue

            hostname = ADUtils.get_entry_property(entry, 'dNSHostName')
            samname = ADUtils.get_entry_property(entry, 'sAMAccountName')
            if not hostname:
                logging.debug('Invalid computer object without hostname: %s', samname)
                hostname = ''

            cobject = ADComputer(hostname=hostname, samname=samname, ad=self.addomain, addc=self.addc, objectsid=entry['attributes']['objectSid'])
            cobject.primarygroup = MembershipEnumerator.get_primary_membership(entry)
            computer = cobject.get_bloodhound_data(entry, self.collect, skip_acl=True)

            # If we are enumerating ACLs, we break out of the loop here
            # this is because parsing ACLs is computationally heavy and therefor is done in subprocesses
            if acl:
                if self.disable_pooling:
                    # Debug mode, don't run this pooled since it hides exceptions
                    self.process_acldata(parse_binary_acl(computer, 'computer', ADUtils.get_entry_property(entry, 'nTSecurityDescriptor', raw=True), self.addc.objecttype_guid_map))
                else:
                    # Process ACLs in separate processes, then call the processing function to resolve entries and write them to file
                    self.aclenumerator.pool.apply_async(parse_binary_acl, args=(computer, 'computer', ADUtils.get_entry_property(entry, 'nTSecurityDescriptor', raw=True), self.addc.objecttype_guid_map), callback=self.process_acldata)
            else:
                # Write it to the queue -> write to file in separate thread
                # this is solely for consistency with acl parsing, the performance improvement is probably minimal
                self.result_q.put(computer)

        # If we are parsing ACLs, close the parsing pool first
        # then close the result queue and join it
        if acl and not self.disable_pooling:
            self.aclenumerator.pool.close()
            self.aclenumerator.pool.join()
            self.result_q.put(None)
        else:
            self.result_q.put(None)
        self.result_q.join()

        logging.debug('Finished writing computers')

    def enumerate_gpos(self, timestamp ="", fileNamePrefix=""):
        if (fileNamePrefix != None):
            filename = fileNamePrefix + "_" + timestamp + 'gpos.json'
        else:
            filename = timestamp + 'gpos.json'

        with_properties = 'objectprops' in self.collect
        acl = 'acl' in self.collect
        entries = self.addc.get_gpos(include_properties=with_properties, acl=acl)

        logging.debug('Writing GPOs to file: %s', filename)

        # Use a separate queue for processing the results
        self.result_q = queue.Queue()
        results_worker = threading.Thread(target=OutputWorker.membership_write_worker, args=(self.result_q, 'gpos', filename))
        results_worker.daemon = True
        results_worker.start()

        if acl and not self.disable_pooling:
            self.aclenumerator.init_pool()

        for entry in entries:
            resolved_entry = ADUtils.resolve_ad_entry(entry)
            try:
                guid = entry['attributes']['objectGUID'][1:-1].upper()
            except KeyError:
                #Somehow we found an OU without a guid?
                logging.warning('Could not determine GUID for GPO %s', entry['attributes']['distinguishedName'])
                continue
            gpo = {
                "ObjectIdentifier": guid,
                "Properties": {
                    "domain": self.addomain.domain.upper(),
                    "name": '%s@%s' % (ADUtils.get_entry_property(entry, 'displayName', '').upper(), self.addomain.domain.upper()),
                    "distinguishedname": ADUtils.get_entry_property(entry, 'distinguishedName', '').upper(),
                    "domainsid": self.addomain.domain_object.sid,
                    "highvalue": False,
                    "gpcpath": ADUtils.get_entry_property(entry, 'gPCFileSysPath', '').upper(),
                },
                "IsDeleted": False,
                "IsACLProtected": False,
                "Aces": [],
            }
            
            if with_properties:
                gpo["Properties"]["description"] = ADUtils.get_entry_property(entry, 'description', '')
                whencreated = ADUtils.get_entry_property(entry, 'whencreated', default=0)
                if isinstance(whencreated, int):
                    gpo['Properties']['whencreated'] = whencreated
                else:
                    gpo['Properties']['whencreated'] = calendar.timegm(whencreated.timetuple())

            # Create cache entry for links
            link_output = {
                "ObjectIdentifier": gpo['ObjectIdentifier'],
                "ObjectType": "GPO",
            }
            self.addomain.dncache[ADUtils.get_entry_property(entry, 'distinguishedName').upper()] = link_output

            # If we are enumerating ACLs, we break out of the loop here
            # this is because parsing ACLs is computationally heavy and therefor is done in subprocesses
            if acl:
                if self.disable_pooling:
                    # Debug mode, don't run this pooled since it hides exceptions
                    self.process_acldata(parse_binary_acl(gpo, 'gpo', ADUtils.get_entry_property(entry, 'nTSecurityDescriptor', raw=True), self.addc.objecttype_guid_map))
                else:
                    # Process ACLs in separate processes, then call the processing function to resolve entries and write them to file
                    self.aclenumerator.pool.apply_async(parse_binary_acl, args=(gpo, 'gpo', ADUtils.get_entry_property(entry, 'nTSecurityDescriptor', raw=True), self.addc.objecttype_guid_map), callback=self.process_acldata)
            else:
                # Write it to the queue -> write to file in separate thread
                # this is solely for consistency with acl parsing, the performance improvement is probably minimal
                self.result_q.put(gpo)

            # self.write_default_groups()

        # If we are parsing ACLs, close the parsing pool first
        # then close the result queue and join it
        if acl and not self.disable_pooling:
            self.aclenumerator.pool.close()
            self.aclenumerator.pool.join()
            self.result_q.put(None)
        else:
            self.result_q.put(None)
        self.result_q.join()

        logging.debug('Finished writing GPO')

    def enumerate_ous(self, timestamp ="", fileNamePrefix=""):
        if (fileNamePrefix != None):
            filename = fileNamePrefix + "_" + timestamp + 'ous.json'
        else:
            filename = timestamp + 'ous.json'
        with_properties = 'objectprops' in self.collect
        acl = 'acl' in self.collect
        entries = self.addc.get_ous(include_properties=with_properties, acl=acl)

        logging.debug('Writing OU to file: %s', filename)

        # Use a separate queue for processing the results
        self.result_q = queue.Queue()
        results_worker = threading.Thread(target=OutputWorker.membership_write_worker, args=(self.result_q, 'ous', filename))
        results_worker.daemon = True
        results_worker.start()

        if acl and not self.disable_pooling:
            self.aclenumerator.init_pool()

        for entry in entries:
            resolved_entry = ADUtils.resolve_ad_entry(entry)
            try:
                guid = entry['attributes']['objectGUID'][1:-1].upper()
            except KeyError:
                #Somehow we found an OU without a guid?
                logging.warning('Could not determine GUID for OU %s', entry['attributes']['distinguishedName'])
                continue
            ou = {
                "ObjectIdentifier": guid,
                "Properties": {
                    "domain": self.addomain.domain.upper(),
                    "name": '%s@%s' % (ADUtils.get_entry_property(entry, 'name').upper(), self.addomain.domain.upper()),
                    "distinguishedname": ADUtils.get_entry_property(entry, 'distinguishedName').upper(),
                    "domainsid": self.addomain.domain_object.sid,
                    "highvalue": False,
                    # Ref: https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-gpol/08090b22-bc16-49f4-8e10-f27a8fb16d18
                    "blocksinheritance": ADUtils.get_entry_property(entry, 'gPOptions', 0) == 1,
                },
                "IsDeleted": False,
                "IsACLProtected": False,
                "Aces": [],
                "Links": [],
                "ChildObjects": [],
                "GPOChanges": {
                    "AffectedComputers": [],
                    "DcomUsers": [],
                    "LocalAdmins": [],
                    "PSRemoteUsers": [],
                    "RemoteDesktopUsers": []
                },
            }

            
            if with_properties:
                ou["Properties"]["description"] = ADUtils.get_entry_property(entry, 'description')
                whencreated = ADUtils.get_entry_property(entry, 'whencreated', default=0)
                if isinstance(whencreated, int):
                    ou['Properties']['whencreated'] = whencreated
                else:
                    ou['Properties']['whencreated'] = calendar.timegm(whencreated.timetuple())
            
            for childentry in self.addc.get_childobjects(ou["Properties"]["distinguishedname"]):
                resolved_childentry = ADUtils.resolve_ad_entry(childentry)
                # Skip children with empty ID because it breaks ingestion
                if not resolved_childentry['objectid']:
                    continue
                out_object = {
                    "ObjectIdentifier":resolved_childentry['objectid'],
                    "ObjectType":resolved_childentry['type']
                }
                ou["ChildObjects"].append(out_object)
            
            for gplink_dn, option in ADUtils.parse_gplink_string(ADUtils.get_entry_property(entry, 'gPLink', '')):
                if option == 0 or option == 2:
                    link = dict()
                    link['IsEnforced'] = option == 2
                    try:
                        link['GUID'] = self.get_membership(gplink_dn.upper())['ObjectIdentifier']
                        if not link['GUID']:
                            continue
                        ou['Links'].append(link)
                    except TypeError:
                        logging.warning('Could not resolve GPO link to {0}'.format(gplink_dn))
            
            # Create cache entry for links
            link_output = {
                "ObjectIdentifier": ou['ObjectIdentifier'],
                "ObjectType": 'OU'
            }
            self.addomain.dncache[ADUtils.get_entry_property(entry, 'distinguishedName').upper()] = link_output

            # If we are enumerating ACLs, we break out of the loop here
            # this is because parsing ACLs is computationally heavy and therefor is done in subprocesses
            if acl:
                if self.disable_pooling:
                    # Debug mode, don't run this pooled since it hides exceptions
                    self.process_acldata(parse_binary_acl(ou, 'organizational-unit', ADUtils.get_entry_property(entry, 'nTSecurityDescriptor', raw=True), self.addc.objecttype_guid_map))
                else:
                    # Process ACLs in separate processes, then call the processing function to resolve entries and write them to file
                    self.aclenumerator.pool.apply_async(parse_binary_acl, args=(ou, 'organizational-unit', ADUtils.get_entry_property(entry, 'nTSecurityDescriptor', raw=True), self.addc.objecttype_guid_map), callback=self.process_acldata)
            else:
                # Write it to the queue -> write to file in separate thread
                # this is solely for consistency with acl parsing, the performance improvement is probably minimal
                self.result_q.put(ou)

            # self.write_default_groups()

        # If we are parsing ACLs, close the parsing pool first
        # then close the result queue and join it
        if acl and not self.disable_pooling:
            self.aclenumerator.pool.close()
            self.aclenumerator.pool.join()
            self.result_q.put(None)
        else:
            self.result_q.put(None)
        self.result_q.join()

        logging.debug('Finished writing OU')

    def enumerate_containers(self, timestamp ="", fileNamePrefix=""):
        if (fileNamePrefix != None):
            filename = fileNamePrefix + "_" + timestamp + 'containers.json'
        else:
            filename = timestamp + 'containers.json'
        with_properties = 'objectprops' in self.collect
        acl = 'acl' in self.collect
        entries = self.addc.get_containers(include_properties=with_properties, acl=acl)

        logging.debug('Writing containers to file: %s', filename)

        # Use a separate queue for processing the results
        self.result_q = queue.Queue()
        results_worker = threading.Thread(target=OutputWorker.membership_write_worker, args=(self.result_q, 'containers', filename))
        results_worker.daemon = True
        results_worker.start()

        if acl and not self.disable_pooling:
            self.aclenumerator.init_pool()

        for entry in entries:
            if ADUtils.is_filtered_container(ADUtils.get_entry_property(entry, 'distinguishedName')):
                continue
            resolved_entry = ADUtils.resolve_ad_entry(entry)
            try:
                guid = entry['attributes']['objectGUID'][1:-1].upper()
            except KeyError:
                #Somehow we found an container without a guid?
                logging.warning('Could not determine GUID for container %s', entry['attributes']['distinguishedName'])
                continue
            container = {
                "ObjectIdentifier": guid,
                "Properties": {
                    "domain": self.addomain.domain.upper(),
                    "name": '%s@%s' % (ADUtils.get_entry_property(entry, 'name').upper(), self.addomain.domain.upper()),
                    "distinguishedname": ADUtils.get_entry_property(entry, 'distinguishedName').upper(),
                    "domainsid": self.addomain.domain_object.sid,
                    "highvalue": False,
                },
                "IsDeleted": False,
                "IsACLProtected": False,
                "Aces": [],
                "ChildObjects": [],
            }

            
            if with_properties:
                container["Properties"]["description"] = ADUtils.get_entry_property(entry, 'description', '')
                whencreated = ADUtils.get_entry_property(entry, 'whencreated', default=0)
                container["Properties"]["whencreated"] =  calendar.timegm(whencreated.timetuple())
            
            for childentry in self.addc.get_childobjects(container["Properties"]["distinguishedname"]):
                if ADUtils.is_filtered_container_child(ADUtils.get_entry_property(childentry, 'distinguishedName')):
                    continue
                resolved_childentry = ADUtils.resolve_ad_entry(childentry)
                # Skip children with empty ID because it breaks ingestion
                if not resolved_childentry['objectid']:
                    continue
                childobject = {
                    "ObjectIdentifier":resolved_childentry['objectid'],
                    "ObjectType":resolved_childentry['type']
                }
                container["ChildObjects"].append(childobject)
            
            # Create cache entry for links
            link_output = {
                "ObjectIdentifier": container['ObjectIdentifier'],
                "ObjectType": 'container'
            }
            self.addomain.dncache[ADUtils.get_entry_property(entry, 'distinguishedName').upper()] = link_output

            # If we are enumerating ACLs, we break out of the loop here
            # this is because parsing ACLs is computationally heavy and therefor is done in subprocesses
            if acl:
                if self.disable_pooling:
                    # Debug mode, don't run this pooled since it hides exceptions
                    self.process_acldata(parse_binary_acl(container, 'container', ADUtils.get_entry_property(entry, 'nTSecurityDescriptor', raw=True), self.addc.objecttype_guid_map))
                else:
                    # Process ACLs in separate processes, then call the processing function to resolve entries and write them to file
                    self.aclenumerator.pool.apply_async(parse_binary_acl, args=(container, 'container', ADUtils.get_entry_property(entry, 'nTSecurityDescriptor', raw=True), self.addc.objecttype_guid_map), callback=self.process_acldata)
            else:
                # Write it to the queue -> write to file in separate thread
                # this is solely for consistency with acl parsing, the performance improvement is probably minimal
                self.result_q.put(container)

        # If we are parsing ACLs, close the parsing pool first
        # then close the result queue and join it
        if acl and not self.disable_pooling:
            self.aclenumerator.pool.close()
            self.aclenumerator.pool.join()
            self.result_q.put(None)
        else:
            self.result_q.put(None)
        self.result_q.join()

        logging.debug('Finished writing containers')

    def parse_gmsa(self, user, entry):
        """
        Parse GMSA DACL which states which users can read the password
        """
        _, aces = parse_binary_acl(user, 'user', ADUtils.get_entry_property(entry, 'msDS-GroupMSAMembership', raw=True), self.addc.objecttype_guid_map)
        processed_aces = self.aceresolver.resolve_aces(aces)
        for ace in processed_aces:
            if ace['RightName'] == 'Owner':
                continue
            ace['RightName'] = 'ReadGMSAPassword'
            user['Aces'].append(ace)

    def process_acldata(self, result):
        """
        Process ACLs that resulted from parsing with cstruct
        """
        data, aces = result
        # Parse aces
        data['Aces'] += self.aceresolver.resolve_aces(aces)
        self.result_q.put(data)

    def write_default_users(self):
        """
        Write built-in users to users.json file
        """

        domainsid = self.addomain.domain_object.sid
        domainname = self.addomain.domain.upper()

        user = {
            "AllowedToDelegate": [],
            "ObjectIdentifier": "%s-S-1-5-20" % domainname,
            "PrimaryGroupSID": None,
            "Properties": {
                "domain": domainname,
                "domainsid": self.addomain.domain_object.sid,
                "name": "NT AUTHORITY@%s" % domainname,
            },
            "Aces": [],
            "SPNTargets": [],
            "HasSIDHistory": [],
            "IsDeleted": False,
            "IsACLProtected": False,
        }
        self.result_q.put(user)


    def write_default_groups(self):
        """
        Put default groups in the groups.json file
        """

        # Domain controllers
        rootdomain = self.addc.get_root_domain().upper()
        entries = self.addc.get_domain_controllers()

        group = {
            "IsDeleted": False,
            "IsACLProtected": False,
            "ObjectIdentifier": "%s-S-1-5-9" % rootdomain,
            "Properties": {
                "domain": rootdomain.upper(),
                "name": "ENTERPRISE DOMAIN CONTROLLERS@%s" % rootdomain,
            },
            "Members": [],
            "Aces": []
        }
        for entry in entries:
            resolved_entry = ADUtils.resolve_ad_entry(entry)
            memberdata = {
                "ObjectIdentifier": resolved_entry['objectid'],
                "ObjectType": resolved_entry['type'].capitalize()
            }
            group["Members"].append(memberdata)
        self.result_q.put(group)

        domainsid = self.addomain.domain_object.sid
        domainname = self.addomain.domain.upper()

        # Everyone
        evgroup = {
            "IsDeleted": False,
            "IsACLProtected": False,
            "ObjectIdentifier": "%s-S-1-1-0" % domainname,
            "Properties": {
                "domain": domainname,
                "domainsid": self.addomain.domain_object.sid,
                "name": "EVERYONE@%s" % domainname,
            },
            "Members": [],
            "Aces": []
        }
        self.result_q.put(evgroup)

        # Authenticated users
        augroup = {
            "IsDeleted": False,
            "IsACLProtected": False,
            "ObjectIdentifier": "%s-S-1-5-11" % domainname,
            "Properties": {
                "domain": domainname,
                "domainsid": self.addomain.domain_object.sid,
                "name": "AUTHENTICATED USERS@%s" % domainname,
            },
            "Members": [],
            "Aces": []
        }
        self.result_q.put(augroup)

        # Interactive
        iugroup = {
            "IsDeleted": False,
            "IsACLProtected": False,
            "ObjectIdentifier": "%s-S-1-5-4" % domainname,
            "Properties": {
                "domain": domainname,
                "domainsid": self.addomain.domain_object.sid,
                "name": "INTERACTIVE@%s" % domainname,
            },
            "Members": [],
            "Aces": []
        }
        self.result_q.put(iugroup)

    def do_container_collection(self, timestamp="", fileNamePrefix=""):
        self.enumerate_gpos(timestamp, fileNamePrefix)
        self.enumerate_ous(timestamp, fileNamePrefix)
        self.enumerate_containers(timestamp, fileNamePrefix)

    def enumerate_memberships(self, timestamp="", fileNamePrefix=""):
        """
        Run appropriate enumeration tasks
        """
        self.enumerate_users(timestamp, fileNamePrefix)
        self.enumerate_groups(timestamp, fileNamePrefix)
        if 'container' in self.collect:
            self.do_container_collection(timestamp, fileNamePrefix)
        if not ('localadmin' in self.collect
                or 'session' in self.collect
                or 'loggedon' in self.collect
                or 'experimental' in self.collect):
            self.enumerate_computers_dconly(timestamp, fileNamePrefix)
