####################
#
# Copyright (c) 2017 Dirk-jan Mollema
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
import sys, os, re, codecs, json, argparse, getpass, base64
# import class and constants
from datetime import datetime, timedelta
from urllib.parse import quote_plus
import ldap3
from ldap3 import Server, Connection, SIMPLE, SYNC, ALL, SASL, NTLM
from ldap3.core.exceptions import LDAPKeyError, LDAPAttributeError, LDAPCursorError, LDAPInvalidDnError
from ldap3.abstract import attribute, attrDef
from ldap3.utils import dn
from ldap3.protocol.formatters.formatters import format_sid

# dnspython, for resolving hostnames
import dns.resolver


# User account control flags
# From: https://blogs.technet.microsoft.com/askpfeplat/2014/01/15/understanding-the-useraccountcontrol-attribute-in-active-directory/
uac_flags = {'ACCOUNT_DISABLED':0x00000002,
             'ACCOUNT_LOCKED':0x00000010,
             'PASSWD_NOTREQD':0x00000020,
             'PASSWD_CANT_CHANGE': 0x00000040,
             'PASSWORD_STORE_CLEARTEXT': 0x00000080,
             'NORMAL_ACCOUNT': 0x00000200,
             'WORKSTATION_ACCOUNT':0x00001000,
             'SERVER_TRUST_ACCOUNT': 0x00002000,
             'DONT_EXPIRE_PASSWD': 0x00010000,
             'SMARTCARD_REQUIRED': 0x00040000,
             'TRUSTED_FOR_DELEGATION': 0x00080000,
             'NOT_DELEGATED': 0x00100000,
             'USE_DES_KEY_ONLY': 0x00200000,
             'DONT_REQ_PREAUTH': 0x00400000,
             'PASSWORD_EXPIRED': 0x00800000,
             'TRUSTED_TO_AUTH_FOR_DELEGATION': 0x01000000,
             'PARTIAL_SECRETS_ACCOUNT': 0x04000000
            }

# Password policy flags
pwd_flags = {'PASSWORD_COMPLEX':0x01,
             'PASSWORD_NO_ANON_CHANGE': 0x02,
             'PASSWORD_NO_CLEAR_CHANGE': 0x04,
             'LOCKOUT_ADMINS': 0x08,
             'PASSWORD_STORE_CLEARTEXT': 0x10,
             'REFUSE_PASSWORD_CHANGE': 0x20}

# Domain trust flags
# From: https://msdn.microsoft.com/en-us/library/cc223779.aspx
trust_flags = {'NON_TRANSITIVE':0x00000001,
               'UPLEVEL_ONLY':0x00000002,
               'QUARANTINED_DOMAIN':0x00000004,
               'FOREST_TRANSITIVE':0x00000008,
               'CROSS_ORGANIZATION':0x00000010,
               'WITHIN_FOREST':0x00000020,
               'TREAT_AS_EXTERNAL':0x00000040,
               'USES_RC4_ENCRYPTION':0x00000080,
               'CROSS_ORGANIZATION_NO_TGT_DELEGATION':0x00000200,
               'CROSS_ORGANIZATION_ENABLE_TGT_DELEGATION':0x00000800,
               'PIM_TRUST':0x00000400}

# Domain trust direction
# From: https://msdn.microsoft.com/en-us/library/cc223768.aspx
trust_directions = {'INBOUND':0x01,
                    'OUTBOUND':0x02,
                    'BIDIRECTIONAL':0x03}
# Domain trust types
trust_type = {'DOWNLEVEL':0x01,
              'UPLEVEL':0x02,
              'MIT':0x03}

# Common attribute pretty translations
attr_translations = {'sAMAccountName':'SAM Name',
                     'cn':'CN',
                     'operatingSystem':'Operating System',
                     'operatingSystemServicePack':'Service Pack',
                     'operatingSystemVersion':'OS Version',
                     'userAccountControl':'Flags',
                     'objectSid':'SID',
                     'memberOf':'Member of groups',
                     'primaryGroupId':'Primary group',
                     'dNSHostName':'DNS Hostname',
                     'whenCreated':'Created on',
                     'whenChanged':'Changed on',
                     'IPv4':'IPv4 Address',
                     'lockOutObservationWindow':'Lockout time window',
                     'lockoutDuration':'Lockout Duration',
                     'lockoutThreshold':'Lockout Threshold',
                     'maxPwdAge':'Max password age',
                     'minPwdAge':'Min password age',
                     'minPwdLength':'Min password length',
                     'pwdHistoryLength':'Password history length',
                     'pwdProperties':'Password properties',
                     'ms-DS-MachineAccountQuota':'Machine Account Quota',
                     'flatName':'NETBIOS Domain name'}

MINIMAL_COMPUTERATTRIBUTES = ['cn', 'sAMAccountName', 'dNSHostName', 'operatingSystem', 'operatingSystemServicePack', 'operatingSystemVersion', 'lastLogon', 'userAccountControl', 'whenCreated', 'objectSid', 'description', 'objectClass']
MINIMAL_USERATTRIBUTES = ['cn', 'name', 'sAMAccountName', 'memberOf', 'primaryGroupId', 'whenCreated', 'whenChanged', 'lastLogon', 'userAccountControl', 'pwdLastSet', 'objectSid', 'description', 'servicePrincipalName', 'objectClass']
MINIMAL_GROUPATTRIBUTES = ['cn', 'name', 'sAMAccountName', 'memberOf', 'description', 'whenCreated', 'whenChanged', 'objectSid', 'distinguishedName', 'objectClass']

#Class containing the default config
class domainDumpConfig():
    def __init__(self):
        #Base path
        self.basepath = '.'

        #Output files basenames
        self.groupsfile = 'domain_groups' #Groups
        self.usersfile = 'domain_users' #User accounts
        self.computersfile = 'domain_computers' #Computer accounts
        self.policyfile = 'domain_policy' #General domain attributes
        self.trustsfile = 'domain_trusts' #Domain trusts attributes

        #Combined files basenames
        self.users_by_group = 'domain_users_by_group' #Users sorted by group
        self.computers_by_os = 'domain_computers_by_os' #Computers sorted by OS

        #Output formats
        self.outputhtml = True
        self.outputjson = True
        self.outputgrep = True

        #Output json for groups
        self.groupedjson = False

        #Default field delimiter for greppable format is a tab
        self.grepsplitchar = '\t'

        #Other settings
        self.lookuphostnames = False #Look up hostnames of computers to get their IP address
        self.dnsserver = '' #Addres of the DNS server to use, if not specified default DNS will be used
        self.minimal = False #Only query minimal list of attributes

#Domaindumper main class
class domainDumper():
    def __init__(self, server, connection, config, root=None):
        self.server = server
        self.connection = connection
        self.config = config
        #Unless the root is specified we get it from the server
        if root is None:
            self.root = self.getRoot()
        else:
            self.root = root
        self.users = None #Domain users
        self.groups = None #Domain groups
        self.computers = None #Domain computers
        self.policy = None #Domain policy
        self.groups_dnmap = None #CN map for group IDs to CN
        self.groups_dict = None #Dictionary of groups by CN
        self.trusts = None #Domain trusts

    #Get the server root from the default naming context
    def getRoot(self):
        return self.server.info.other['defaultNamingContext'][0]

    #Query the groups of the current user
    def getCurrentUserGroups(self, username, domainsid=None):
        self.connection.search(self.root, '(&(objectCategory=person)(objectClass=user)(sAMAccountName=%s))' % username, attributes=['cn', 'memberOf', 'primaryGroupId'])
        try:
            groups = self.connection.entries[0]['memberOf'].values
            if domainsid is not None:
                groups.append(self.getGroupDNfromID(domainsid, self.connection.entries[0]['primaryGroupId'].value))
            return groups
        except LDAPKeyError:
            #No groups, probably just member of the primary group
            if domainsid is not None:
                primarygroup = self.getGroupDNfromID(domainsid, self.connection.entries[0]['primaryGroupId'].value)
                return [primarygroup]
            else:
                return []
        except IndexError:
            #The username does not exist (might be a computer account)
            return []

    #Check if the user is part of the Domain Admins or Enterprise Admins group, or any of their subgroups
    def isDomainAdmin(self, username):
        domainsid = self.getRootSid()
        groups = self.getCurrentUserGroups(username, domainsid)
        #Get DA and EA group DNs
        dagroupdn = self.getDAGroupDN(domainsid)
        eagroupdn = self.getEAGroupDN(domainsid)
        #First, simple checks
        for group in groups:
            if 'CN=Administrators' in group or 'CN=Domain Admins' in group or dagroupdn == group:
                return True
            #Also for enterprise admins if applicable
            if 'CN=Enterprise Admins' in group or (eagroupdn is not False and eagroupdn == group):
                return True
        #Now, just do a recursive check in both groups and their subgroups using LDAP_MATCHING_RULE_IN_CHAIN
        self.connection.search(self.root, '(&(objectCategory=person)(objectClass=user)(sAMAccountName=%s)(memberOf:1.2.840.113556.1.4.1941:=%s))' % (username, dagroupdn), attributes=['cn', 'sAMAccountName'])
        if len(self.connection.entries) > 0:
            return True
        self.connection.search(self.root, '(&(objectCategory=person)(objectClass=user)(sAMAccountName=%s)(memberOf:1.2.840.113556.1.4.1941:=%s))' % (username, eagroupdn), attributes=['cn', 'sAMAccountName'])
        if len(self.connection.entries) > 0:
            return True
        #At last, check the users primary group ID
        return False

    #Get all users
    def getAllUsers(self):
        if self.config.minimal:
            self.connection.extend.standard.paged_search('%s' % (self.root), '(&(objectCategory=person)(objectClass=user))', attributes=MINIMAL_USERATTRIBUTES, paged_size=500, generator=False)
        else:
            self.connection.extend.standard.paged_search('%s' % (self.root), '(&(objectCategory=person)(objectClass=user))', attributes=ldap3.ALL_ATTRIBUTES, paged_size=500, generator=False)
        return self.connection.entries

    #Get all computers in the domain
    def getAllComputers(self):
        if self.config.minimal:
            self.connection.extend.standard.paged_search('%s' % (self.root), '(&(objectClass=computer)(objectClass=user))', attributes=MINIMAL_COMPUTERATTRIBUTES, paged_size=500, generator=False)
        else:
            self.connection.extend.standard.paged_search('%s' % (self.root), '(&(objectClass=computer)(objectClass=user))', attributes=ldap3.ALL_ATTRIBUTES, paged_size=500, generator=False)
        return self.connection.entries

    #Get all user SPNs
    def getAllUserSpns(self):
        if self.config.minimal:
            self.connection.extend.standard.paged_search('%s' % (self.root), '(&(objectCategory=person)(objectClass=user)(servicePrincipalName=*))', attributes=MINIMAL_USERATTRIBUTES, paged_size=500, generator=False)
        else:
            self.connection.extend.standard.paged_search('%s' % (self.root), '(&(objectCategory=person)(objectClass=user)(servicePrincipalName=*))', attributes=ldap3.ALL_ATTRIBUTES, paged_size=500, generator=False)
        return self.connection.entries

    #Get all defined groups
    def getAllGroups(self):
        if self.config.minimal:
            self.connection.extend.standard.paged_search(self.root, '(objectClass=group)', attributes=MINIMAL_GROUPATTRIBUTES, paged_size=500, generator=False)
        else:
            self.connection.extend.standard.paged_search(self.root, '(objectClass=group)', attributes=ldap3.ALL_ATTRIBUTES, paged_size=500, generator=False)
        return self.connection.entries

    #Get the domain policies (such as lockout policy)
    def getDomainPolicy(self):
        self.connection.search(self.root, '(objectClass=domain)', attributes=ldap3.ALL_ATTRIBUTES)
        return self.connection.entries

    #Get domain trusts
    def getTrusts(self):
        self.connection.search(self.root, '(objectClass=trustedDomain)', attributes=ldap3.ALL_ATTRIBUTES)
        return self.connection.entries

    #Get all defined security groups
    #Syntax from:
    #https://ldapwiki.willeke.com/wiki/Active%20Directory%20Group%20Related%20Searches
    def getAllSecurityGroups(self):
        self.connection.search(self.root, '(groupType:1.2.840.113556.1.4.803:=2147483648)', attributes=ldap3.ALL_ATTRIBUTES)
        return self.connection.entries

    #Get the SID of the root object
    def getRootSid(self):
        self.connection.search(self.root, '(objectClass=domain)', attributes=['objectSid'])
        try:
            sid = self.connection.entries[0].objectSid
        except (LDAPAttributeError, LDAPCursorError, IndexError):
            return False
        return sid

    #Get group members recursively using LDAP_MATCHING_RULE_IN_CHAIN (1.2.840.113556.1.4.1941)
    def getRecursiveGroupmembers(self, groupdn):
        self.connection.extend.standard.paged_search(self.root, '(&(objectCategory=person)(objectClass=user)(memberOf:1.2.840.113556.1.4.1941:=%s))' % groupdn, attributes=MINIMAL_USERATTRIBUTES, paged_size=500, generator=False)
        return self.connection.entries

    #Resolve group ID to DN
    def getGroupDNfromID(self, domainsid, gid):
        self.connection.search(self.root, '(objectSid=%s-%d)' % (domainsid, gid), attributes=['distinguishedName'])
        return self.connection.entries[0]['distinguishedName'].value

    #Get Domain Admins group DN
    def getDAGroupDN(self, domainsid):
        return self.getGroupDNfromID(domainsid, 512)

    #Get Enterprise Admins group DN
    def getEAGroupDN(self, domainsid):
        try:
            return self.getGroupDNfromID(domainsid, 519)
        except (LDAPAttributeError, LDAPCursorError, IndexError):
            #This does not exist, could be in a parent domain
            return False


    #Lookup all computer DNS names to get their IP
    def lookupComputerDnsNames(self):
        dnsresolver = dns.resolver.Resolver()
        dnsresolver.lifetime = 2
        ipdef = attrDef.AttrDef('ipv4')
        if self.config.dnsserver != '':
            dnsresolver.nameservers = [self.config.dnsserver]
        for computer in self.computers:
            try:
                answers = dnsresolver.query(computer.dNSHostName.values[0], 'A')
                ip = str(answers.response.answer[0][0])
            except dns.resolver.NXDOMAIN:
                ip = 'error.NXDOMAIN'
            except dns.resolver.Timeout:
                ip = 'error.TIMEOUT'
            except (LDAPAttributeError, LDAPCursorError):
                ip = 'error.NOHOSTNAME'
            #Construct a custom attribute as workaround
            ipatt = attribute.Attribute(ipdef, computer, None)
            ipatt.__dict__['_response'] = ip
            ipatt.__dict__['raw_values'] = [ip]
            ipatt.__dict__['values'] = [ip]
            #Add the attribute to the entry's dictionary
            computer._state.attributes['IPv4'] = ipatt

    #Create a dictionary of all operating systems with the computer accounts that are associated
    def sortComputersByOS(self, items):
        osdict = {}
        for computer in items:
            try:
                cos = computer.operatingSystem.value or 'Unknown'
            except (LDAPAttributeError, LDAPCursorError):
                cos = 'Unknown'
            try:
                osdict[cos].append(computer)
            except KeyError:
                #New OS
                osdict[cos] = [computer]
        return osdict

    #Map all groups on their ID (taken from their SID) to CNs
    #This is used for getting the primary group of a user
    def mapGroupsIdsToDns(self):
        dnmap = {}
        for group in self.groups:
            gid = int(group.objectSid.value.split('-')[-1])
            dnmap[gid] = group.distinguishedName.values[0]
        self.groups_dnmap = dnmap
        return dnmap

    #Create a dictionary where a groups CN returns the full object
    def createGroupsDictByCn(self):
        gdict = {grp.cn.values[0]:grp for grp in self.groups}
        self.groups_dict = gdict
        return gdict

    #Get CN from DN
    def getGroupCnFromDn(self, dnin):
        cn = self.unescapecn(dn.parse_dn(dnin)[0][1])
        return cn

    #Unescape special DN characters from a CN (only needed if it comes from a DN)
    def unescapecn(self, cn):
        for c in ' "#+,;<=>\\\00':
            cn = cn.replace('\\'+c, c)
        return cn

    #Sort users by group they belong to
    def sortUsersByGroup(self, items):
        groupsdict = {}
        #Make sure the group CN mapping already exists
        if self.groups_dnmap is None:
            self.mapGroupsIdsToDns()
        for user in items:
            try:
                ugroups = [self.getGroupCnFromDn(group) for group in user.memberOf.values]
            #If the user is only in the default group, its memberOf property wont exist
            except (LDAPAttributeError, LDAPCursorError):
                ugroups = []
            #Add the user default group
            try:
                ugroups.append(self.getGroupCnFromDn(self.groups_dnmap[user.primaryGroupId.value]))
            # Sometimes we can't query this group or it doesn't exist
            except KeyError:
                pass
            for group in ugroups:
                try:
                    groupsdict[group].append(user)
                except KeyError:
                    #Group is not yet in dict
                    groupsdict[group] = [user]

        #Append any groups that are members of groups
        for group in self.groups:
            try:
                for parentgroup in group.memberOf.values:
                    try:
                        groupsdict[self.getGroupCnFromDn(parentgroup)].append(group)
                    except KeyError:
                        #Group is not yet in dict
                        groupsdict[self.getGroupCnFromDn(parentgroup)] = [group]
            #Without subgroups this attribute does not exist
            except (LDAPAttributeError, LDAPCursorError):
                pass

        return groupsdict

    #Main function
    def domainDump(self):
        self.users = self.getAllUsers()
        self.computers = self.getAllComputers()
        self.groups = self.getAllGroups()
        if self.config.lookuphostnames:
            self.lookupComputerDnsNames()
        self.policy = self.getDomainPolicy()
        self.trusts = self.getTrusts()
        rw = reportWriter(self.config)
        rw.generateUsersReport(self)
        rw.generateGroupsReport(self)
        rw.generateComputersReport(self)
        rw.generatePolicyReport(self)
        rw.generateTrustsReport(self)
        rw.generateComputersByOsReport(self)
        rw.generateUsersByGroupReport(self)

class reportWriter():
    def __init__(self, config):
        self.config = config
        self.dd = None
        if self.config.lookuphostnames:
            self.computerattributes = ['cn', 'sAMAccountName', 'dNSHostName', 'IPv4', 'operatingSystem', 'operatingSystemServicePack', 'operatingSystemVersion', 'lastLogon', 'userAccountControl', 'whenCreated', 'objectSid', 'description']
        else:
            self.computerattributes = ['cn', 'sAMAccountName', 'dNSHostName', 'operatingSystem', 'operatingSystemServicePack', 'operatingSystemVersion', 'lastLogon', 'userAccountControl', 'whenCreated', 'objectSid', 'description']
        self.userattributes = ['cn', 'name', 'sAMAccountName', 'memberOf', 'primaryGroupId', 'whenCreated', 'whenChanged', 'lastLogon', 'userAccountControl', 'pwdLastSet', 'objectSid', 'description', 'servicePrincipalName']
        #In grouped view, don't include the memberOf property to reduce output size
        self.userattributes_grouped = ['cn', 'name', 'sAMAccountName', 'whenCreated', 'whenChanged', 'lastLogon', 'userAccountControl', 'pwdLastSet', 'objectSid', 'description', 'servicePrincipalName']
        self.groupattributes = ['cn', 'sAMAccountName', 'memberOf', 'description', 'whenCreated', 'whenChanged', 'objectSid']
        self.policyattributes = ['distinguishedName', 'lockOutObservationWindow', 'lockoutDuration', 'lockoutThreshold', 'maxPwdAge', 'minPwdAge', 'minPwdLength', 'pwdHistoryLength', 'pwdProperties', 'ms-DS-MachineAccountQuota']
        self.trustattributes = ['cn', 'flatName', 'securityIdentifier', 'trustAttributes', 'trustDirection', 'trustType']

    #Escape HTML special chars
    def htmlescape(self, html):
        return (html.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;").replace("'", "&#39;").replace('"', "&quot;"))

    #Unescape special DN characters from a CN (only needed if it comes from a DN)
    def unescapecn(self, cn):
        for c in ' "#+,;<=>\\\00':
            cn = cn.replace('\\'+c, c)
        return cn

    #Convert password max age (in 100 nanoseconds), to days
    def nsToDays(self, length):
        # ldap3 >= 2.6 returns timedelta
        if isinstance(length, timedelta):
            return length.total_seconds() / 86400
        else:
            return abs(length) * .0000001 / 86400

    def nsToMinutes(self, length):
        # ldap3 >= 2.6 returns timedelta
        if isinstance(length, timedelta):
            return length.total_seconds() / 60
        else:
            return abs(length) * .0000001 / 60

    #Parse bitwise flags into a list
    def parseFlags(self, attr, flags_def):
        outflags = []
        if attr is None or attr.value is None:
            return outflags
        for flag, val in flags_def.items():
            if int(attr.value) & val:
                outflags.append(flag)
        return outflags

    #Parse bitwise trust direction - only one flag applies here, 0x03 overlaps
    def parseSingleFlag(self, attr, flags_def):
        outflags = []
        if attr is None:
            return outflags
        for flag, val in flags_def.items():
            if int(attr.value) == val:
                outflags.append(flag)
        return outflags

    #Generate a HTML table from a list of entries, with the specified attributes as column
    def generateHtmlTable(self, listable, attributes, header='', firstTable=True, specialGroupsFormat=False):
        of = []
        #Only if this is the first table it is an actual table, the others are just bodies of the first table
        #This makes sure that multiple tables have their columns aligned to make it less messy
        if firstTable:
            of.append('<table>')
        #Table header
        if header != '':
            of.append('<thead><tr><td colspan="%d" id="cn_%s">%s</td></tr></thead>' % (len(attributes), self.formatId(header), header))
        of.append('<tbody><tr>')
        for hdr in attributes:
            try:
                #Print alias of this attribute if there is one
                of.append('<th>%s</th>' % self.htmlescape(attr_translations[hdr]))
            except KeyError:
                of.append('<th>%s</th>' % self.htmlescape(hdr))
        of.append('</tr>\n')
        for li in listable:
            #Whether we should format group objects separately
            if specialGroupsFormat and 'group' in li['objectClass'].values:
                #Give it an extra class and pass it to the function below to make sure the CN is a link
                liIsGroup = True
                of.append('<tr class="group">')
            else:
                liIsGroup = False
                of.append('<tr>')
            for att in attributes:
                try:
                    of.append('<td>%s</td>' % self.formatAttribute(li[att], liIsGroup))
                except (LDAPKeyError, LDAPCursorError):
                    of.append('<td>&nbsp;</td>')
            of.append('</tr>\n')
        of.append('</tbody>\n')
        return ''.join(of)

    #Generate several HTML tables for grouped reports
    def generateGroupedHtmlTables(self, groups, attributes):
        first = True
        for groupname, members in groups.items():
            yield self.generateHtmlTable(members, attributes, groupname, first, specialGroupsFormat=True)
            if first:
                first = False

    #Write generated HTML to file
    def writeHtmlFile(self, rel_outfile, body, genfunc=None, genargs=None, closeTable=True):
        if not os.path.exists(self.config.basepath):
            os.makedirs(self.config.basepath)
        outfile = os.path.join(self.config.basepath, rel_outfile)
        with codecs.open(outfile, 'w', 'utf8') as of:
            of.write('<!DOCTYPE html>\n<html>\n<head><meta charset="UTF-8">')
            #Include the style
            try:
                with open(os.path.join(os.path.dirname(__file__), 'style.css'), 'r') as sf:
                    of.write('<style type="text/css">')
                    of.write(sf.read())
                    of.write('</style>')
            except IOError:
                log_warn('style.css not found in package directory, styling will be skipped')
            of.write('</head><body>')
            #If the generator is not specified, we should write the HTML blob directly
            if genfunc is None:
                of.write(body)
            else:
                for tpart in genfunc(*genargs):
                    of.write(tpart)
            #Does the body contain an open table?
            if closeTable:
                of.write('</table>')
            of.write('</body></html>')

    #Write generated JSON to file
    def writeJsonFile(self, rel_outfile, jsondata, genfunc=None, genargs=None):
        if not os.path.exists(self.config.basepath):
            os.makedirs(self.config.basepath)
        outfile = os.path.join(self.config.basepath, rel_outfile)
        with codecs.open(outfile, 'w', 'utf8') as of:
            #If the generator is not specified, we should write the JSON blob directly
            if genfunc is None:
                of.write(jsondata)
            else:
                for jpart in genfunc(*genargs):
                    of.write(jpart)

    #Write generated Greppable stuff to file
    def writeGrepFile(self, rel_outfile, body):
        if not os.path.exists(self.config.basepath):
            os.makedirs(self.config.basepath)
        outfile = os.path.join(self.config.basepath, rel_outfile)
        with codecs.open(outfile, 'w', 'utf8') as of:
            of.write(body)

    #Format a value for HTML
    def formatString(self, value):
        if type(value) is datetime:
            try:
                return value.strftime('%x %X')
            except ValueError:
                #Invalid date
                return '0'
        # Make sure it's a unicode string
        if type(value) is bytes:
            return value.encode('utf8')
        if type(value) is str:
            return value#.encode('utf8')
        if type(value) is int:
            return str(value)
        if value is None:
            return ''
        #Other type: just return it
        return value

    #Format an attribute to a human readable format
    def formatAttribute(self, att, formatCnAsGroup=False):
        aname = att.key.lower()
        #User flags
        if aname == 'useraccountcontrol':
            return ', '.join(self.parseFlags(att, uac_flags))
        #List of groups
        if aname == 'member' or aname == 'memberof' and type(att.values) is list:
            return self.formatGroupsHtml(att.values)
        if aname == 'serviceprincipalname' and type(att.values) is list:
            return self.formatSPNsHtml(att.values)
        #Primary group
        if aname == 'primarygroupid':
            try:
                return self.formatGroupsHtml([self.dd.groups_dnmap[att.value]])
            except KeyError:
                return 'NOT FOUND!'
        if aname == 'description' and type(att.values) is list:
             return " ".join(att.values)
        #Pwd flags
        if aname == 'pwdproperties':
            return ', '.join(self.parseFlags(att, pwd_flags))
        #Domain trust flags
        if aname == 'trustattributes':
            return ', '.join(self.parseFlags(att, trust_flags))
        if aname == 'trustdirection':
            if  att.value == 0:
                return 'DISABLED'
            else:
                return ', '.join(self.parseSingleFlag(att, trust_directions))
        if aname == 'trusttype':
            return ', '.join(self.parseSingleFlag(att, trust_type))
        if aname == 'securityidentifier':
            return format_sid(att.raw_values[0])
        if aname == 'minpwdage' or  aname == 'maxpwdage':
            return '%.2f days' % self.nsToDays(att.value)
        if aname == 'lockoutobservationwindow' or  aname == 'lockoutduration':
            return '%.1f minutes' % self.nsToMinutes(att.value)
        if aname == 'objectsid':
            return '<abbr title="%s">%s</abbr>' % (att.value, att.value.split('-')[-1])
        #Special case where the attribute is a CN and it should be made clear its a group
        if aname == 'cn' and formatCnAsGroup:
            return self.formatCnWithGroupLink(att.value)
        #Other
        return self.htmlescape(self.formatString(att.value))


    def formatCnWithGroupLink(self, cn):
        return 'Group: <a href="#cn_%s" title="%s">%s</a>' % (self.formatId(cn), self.htmlescape(cn), self.htmlescape(cn))

    #Convert a CN to a valid HTML id by replacing all non-ascii characters with a _
    def formatId(self, cn):
        return re.sub(r'[^a-zA-Z0-9_\-]+', '_', cn)

    # Fallback function for dirty DN parsing in case ldap3 functions error out
    def parseDnFallback(self, dn):
        try:
            indcn = dn[3:].index(',CN=')
            indou = dn[3:].index(',OU=')
            if indcn < indou:
                cn = dn[3:].split(',CN=')[0]
            else:
                cn = dn[3:].split(',OU=')[0]
        except ValueError:
            cn = dn
        return cn

    #Format groups to readable HTML
    def formatGroupsHtml(self, grouplist):
        outcache = []
        for group in grouplist:
            try:
                cn = self.unescapecn(dn.parse_dn(group)[0][1])
            except LDAPInvalidDnError:
                # Parsing failed, do it manually
                cn = self.unescapecn(self.parseDnFallback(group))
            outcache.append('<a href="%s.html#cn_%s" title="%s">%s</a>' % (self.config.users_by_group, quote_plus(self.formatId(cn)), self.htmlescape(group), self.htmlescape(cn)))
        return ', '.join(outcache)

    #Format groups to greppable format
    def formatGroupsGrep(self, grouplist):
        outcache = []
        for group in grouplist:
            try:
                cn = self.unescapecn(dn.parse_dn(group)[0][1])
            except LDAPInvalidDnError:
                # Parsing failed, do it manually
                cn = self.unescapecn(self.parseDnFallback(group))
            outcache.append(cn)
        return ', '.join(outcache)

    #Format SPNs to readable HTML
    def formatSPNsHtml(self, spnlist):
        return '<br />'.join(spnlist)

    #Format SPNs to greppable format
    def formatSPNsGrep(self, spnlist):
        return ','.join(spnlist)

    #Format attribute for grepping
    def formatGrepAttribute(self, att):
        aname = att.key.lower()
        #User flags
        if aname == 'useraccountcontrol':
            return ', '.join(self.parseFlags(att, uac_flags))
        #List of groups
        if aname == 'member' or aname == 'memberof' and type(att.values) is list:
            return self.formatGroupsGrep(att.values)
        if aname == 'serviceprincipalname' and type(att.values) is list:
            return self.formatSPNsGrep(att.values)
        if aname == 'primarygroupid':
            try:
                return self.formatGroupsGrep([self.dd.groups_dnmap[att.value]])
            except KeyError:
                return 'NOT FOUND!'
        if aname == 'description' and type(att.values) is list:
            return " ".join(att.values)
        #Domain trust flags
        if aname == 'trustattributes':
            return ', '.join(self.parseFlags(att, trust_flags))
        if aname == 'trustdirection':
            if att.value == 0:
                return 'DISABLED'
            else:
                return ', '.join(self.parseSingleFlag(att, trust_directions))
        if aname == 'trusttype':
            return ', '.join(self.parseSingleFlag(att, trust_type))
        if aname == 'securityidentifier':
            return format_sid(att.raw_values[0])
        #Pwd flags
        if aname == 'pwdproperties':
            return ', '.join(self.parseFlags(att, pwd_flags))
        if aname == 'minpwdage' or  aname == 'maxpwdage':
            return '%.2f days' % self.nsToDays(att.value)
        if aname == 'lockoutobservationwindow' or  aname == 'lockoutduration':
            return '%.1f minutes' % self.nsToMinutes(att.value)
        return self.formatString(att.value)

    #Generate grep/awk/cut-able output
    def generateGrepList(self, entrylist, attributes):
        hdr = self.config.grepsplitchar.join(attributes)
        out = [hdr]
        for entry in entrylist:
            eo = []
            for attr in attributes:
                try:
                    eo.append(self.formatGrepAttribute(entry[attr]) or '')
                except (LDAPKeyError, LDAPCursorError):
                    eo.append('')
            out.append(self.config.grepsplitchar.join(eo))
        return '\n'.join(out)

    #Convert a list of entities to a JSON string
    #String concatenation is used here since the entities have their own json generate
    #method and converting the string back to json just to process it would be inefficient
    def generateJsonList(self, entrylist):
        out = '[' + ','.join([entry.entry_to_json() for entry in entrylist]) + ']'
        return out

    #Convert a group key/value pair to json
    #Same methods as previous function are used
    def generateJsonGroup(self, group):
        out = '{%s:%s}' % (json.dumps(group[0]), self.generateJsonList(group[1]))
        return out

    #Convert a list of group dicts with entry lists to JSON string
    #Same methods as previous functions are used, except that text is returned
    #from a generator rather than allocating everything in memory
    def generateJsonGroupedList(self, groups):
        #Start of the list
        yield '['
        firstGroup = True
        for group in groups.items():
            if not firstGroup:
                #Separate items
                yield ','
            else:
                firstGroup = False
            yield self.generateJsonGroup(group)
        yield ']'

    #Generate report of all computers grouped by OS family
    def generateComputersByOsReport(self, dd):
        grouped = dd.sortComputersByOS(dd.computers)
        if self.config.outputhtml:
            #Use the generator approach to save memory
            self.writeHtmlFile('%s.html' % self.config.computers_by_os, None, genfunc=self.generateGroupedHtmlTables, genargs=(grouped, self.computerattributes))
        if self.config.outputjson and self.config.groupedjson:
            self.writeJsonFile('%s.json' % self.config.computers_by_os, None, genfunc=self.generateJsonGroupedList, genargs=(grouped, ))

    #Generate report of all groups and detailled user info
    def generateUsersByGroupReport(self, dd):
        grouped = dd.sortUsersByGroup(dd.users)
        if self.config.outputhtml:
            #Use the generator approach to save memory
            self.writeHtmlFile('%s.html' % self.config.users_by_group, None, genfunc=self.generateGroupedHtmlTables, genargs=(grouped, self.userattributes_grouped))
        if self.config.outputjson and self.config.groupedjson:
            self.writeJsonFile('%s.json' % self.config.users_by_group, None, genfunc=self.generateJsonGroupedList, genargs=(grouped, ))

    #Generate report with just a table of all users
    def generateUsersReport(self, dd):
        #Copy dd to this object, to be able to reference it
        self.dd = dd
        dd.mapGroupsIdsToDns()
        if self.config.outputhtml:
            html = self.generateHtmlTable(dd.users, self.userattributes, 'Domain users')
            self.writeHtmlFile('%s.html' % self.config.usersfile, html)
        if self.config.outputgrep:
            grepout = self.generateGrepList(dd.users, self.userattributes)
            self.writeGrepFile('%s.grep' % self.config.usersfile, grepout)
        if self.config.outputjson:
            jsonout = self.generateJsonList(dd.users)
            self.writeJsonFile('%s.json' % self.config.usersfile, jsonout)

    #Generate report with just a table of all computer accounts
    def generateComputersReport(self, dd):
        if self.config.outputhtml:
            html = self.generateHtmlTable(dd.computers, self.computerattributes, 'Domain computer accounts')
            self.writeHtmlFile('%s.html' % self.config.computersfile, html)
        if self.config.outputgrep:
            grepout = self.generateGrepList(dd.computers, self.computerattributes)
            self.writeGrepFile('%s.grep' % self.config.computersfile, grepout)
        if self.config.outputjson:
            jsonout = self.generateJsonList(dd.computers)
            self.writeJsonFile('%s.json' % self.config.computersfile, jsonout)

    #Generate report with just a table of all computer accounts
    def generateGroupsReport(self, dd):
        if self.config.outputhtml:
            html = self.generateHtmlTable(dd.groups, self.groupattributes, 'Domain groups')
            self.writeHtmlFile('%s.html' % self.config.groupsfile, html)
        if self.config.outputgrep:
            grepout = self.generateGrepList(dd.groups, self.groupattributes)
            self.writeGrepFile('%s.grep' % self.config.groupsfile, grepout)
        if self.config.outputjson:
            jsonout = self.generateJsonList(dd.groups)
            self.writeJsonFile('%s.json' % self.config.groupsfile, jsonout)

    #Generate policy report
    def generatePolicyReport(self, dd):
        if self.config.outputhtml:
            html = self.generateHtmlTable(dd.policy, self.policyattributes, 'Domain policy')
            self.writeHtmlFile('%s.html' % self.config.policyfile, html)
        if self.config.outputgrep:
            grepout = self.generateGrepList(dd.policy, self.policyattributes)
            self.writeGrepFile('%s.grep' % self.config.policyfile, grepout)
        if self.config.outputjson:
            jsonout = self.generateJsonList(dd.policy)
            self.writeJsonFile('%s.json' % self.config.policyfile, jsonout)

    #Generate policy report
    def generateTrustsReport(self, dd):
        if self.config.outputhtml:
            html = self.generateHtmlTable(dd.trusts, self.trustattributes, 'Domain trusts')
            self.writeHtmlFile('%s.html' % self.config.trustsfile, html)
        if self.config.outputgrep:
            grepout = self.generateGrepList(dd.trusts, self.trustattributes)
            self.writeGrepFile('%s.grep' % self.config.trustsfile, grepout)
        if self.config.outputjson:
            jsonout = self.generateJsonList(dd.trusts)
            self.writeJsonFile('%s.json' % self.config.trustsfile, jsonout)

#Some quick logging helpers
def log_warn(text):
    print('[!] %s' % text)
def log_info(text):
    print('[*] %s' % text)
def log_success(text):
    print('[+] %s' % text)

def main():
    parser = argparse.ArgumentParser(description='Domain information dumper via LDAP. Dumps users/computers/groups and OS/membership information to HTML/JSON/greppable output.')
    parser._optionals.title = "Main options"
    parser._positionals.title = "Required options"

    #Main parameters
    #maingroup = parser.add_argument_group("Main options")
    parser.add_argument("host", type=str, metavar='HOSTNAME', help="Hostname/ip or ldap://host:port connection string to connect to (use ldaps:// to use SSL)")
    parser.add_argument("-u", "--user", type=str, metavar='USERNAME', help="DOMAIN\\username for authentication, leave empty for anonymous authentication")
    parser.add_argument("-p", "--password", type=str, metavar='PASSWORD', help="Password or LM:NTLM hash, will prompt if not specified")
    parser.add_argument("-at", "--authtype", type=str, choices=['NTLM', 'SIMPLE'], default='NTLM', help="Authentication type (NTLM or SIMPLE, default: NTLM)")

    #Output parameters
    outputgroup = parser.add_argument_group("Output options")
    outputgroup.add_argument("-o", "--outdir", type=str, metavar='DIRECTORY', help="Directory in which the dump will be saved (default: current)")
    outputgroup.add_argument("--no-html", action='store_true', help="Disable HTML output")
    outputgroup.add_argument("--no-json", action='store_true', help="Disable JSON output")
    outputgroup.add_argument("--no-grep", action='store_true', help="Disable Greppable output")
    outputgroup.add_argument("--grouped-json", action='store_true', default=False, help="Also write json files for grouped files (default: disabled)")
    outputgroup.add_argument("-d", "--delimiter", help="Field delimiter for greppable output (default: tab)")

    #Additional options
    miscgroup = parser.add_argument_group("Misc options")
    miscgroup.add_argument("-r", "--resolve", action='store_true', help="Resolve computer hostnames (might take a while and cause high traffic on large networks)")
    miscgroup.add_argument("-n", "--dns-server", help="Use custom DNS resolver instead of system DNS (try a domain controller IP)")
    miscgroup.add_argument("-m", "--minimal", action='store_true', default=False, help="Only query minimal set of attributes to limit memmory usage")

    args = parser.parse_args()
    #Create default config
    cnf = domainDumpConfig()
    #Dns lookups?
    if args.resolve:
        cnf.lookuphostnames = True
    #Custom dns server?
    if args.dns_server is not None:
        cnf.dnsserver = args.dns_server
    #Minimal attributes?
    if args.minimal:
        cnf.minimal = True
    #Custom separator?
    if args.delimiter is not None:
        cnf.grepsplitchar = args.delimiter
    #Disable html?
    if args.no_html:
        cnf.outputhtml = False
    #Disable json?
    if args.no_json:
        cnf.outputjson = False
    #Disable grep?
    if args.no_grep:
        cnf.outputgrep = False
    #Custom outdir?
    if args.outdir is not None:
        cnf.basepath = args.outdir
    #Do we really need grouped json files?
    cnf.groupedjson = args.grouped_json

    #Prompt for password if not set
    authentication = None
    if args.user is not None:
        if args.authtype == 'SIMPLE':
            authentication = 'SIMPLE'
        else:
            authentication = NTLM
        if not '\\' in args.user:
            log_warn('Username must include a domain, use: DOMAIN\\username')
            sys.exit(1)
        if args.password is None:
            args.password = getpass.getpass()
    else:
        log_info('Connecting as anonymous user, dumping will probably fail. Consider specifying a username/password to login with')
    # define the server and the connection
    s = Server(args.host, get_info=ALL)
    log_info('Connecting to host...')

    c = Connection(s, user=args.user, password=args.password, authentication=authentication)
    log_info('Binding to host')
    # perform the Bind operation
    if not c.bind():
        log_warn('Could not bind with specified credentials')
        log_warn(c.result)
        sys.exit(1)
    log_success('Bind OK')
    log_info('Starting domain dump')
    #Create domaindumper object
    dd = domainDumper(s, c, cnf)

    #Do the actual dumping
    dd.domainDump()
    log_success('Domain dump finished')

if __name__ == '__main__':
    main()
