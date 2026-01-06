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
from bloodhound.ad.structures import LDAP_SID
import logging
"""
Domain trust
"""
class ADDomainTrust(object):
    # Flags copied from ldapdomaindump
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
                   'CROSS_ORGANIZATION_ENABLE_TGT_DELEGATION': 0x00000800,
                   'PIM_TRUST':0x00000400}

    # Domain trust direction
    # From: https://msdn.microsoft.com/en-us/library/cc223768.aspx
    trust_directions = {'INBOUND':0x01,
                        'OUTBOUND':0x02,
                        'BIDIRECTIONAL':0x03}

    # Mapping used to generate output
    direction_map = {flag:meaning.capitalize() for meaning, flag in trust_directions.items()}

    # Domain trust types
    trust_type = {'DOWNLEVEL':0x01,
                  'UPLEVEL':0x02,
                  'MIT':0x03}

    # BloodHound trust types - undeprecated again
    bh_trust_type = {
        'ParentChild': 0,
        'CrossLink': 1,
        'Forest': 2,
        'External': 3,
        'Unknown':4
    }
    def __init__(self, destination, direction, trust_type, flags, domainsid):
        self.destination_domain = destination
        self.direction = direction
        self.type = trust_type
        self.flags = flags
        # Try catching empty SID
        if domainsid:
            self.domainsid = LDAP_SID(domainsid).formatCanonical()
        else:
            logging.debug('Domain %s has empty domain SID', self.destination_domain)
            self.domainsid = ''

    def has_flag(self, flag):
        if self.flags is None or self.trust_flags.get(flag) is None:
            return False  # Handle the case where either self.flags or trust_flags[flag] is None
    
        return self.flags & self.trust_flags[flag] == self.trust_flags[flag]

    def to_output(self):
        trusttype = 'Unknown'
        is_transitive = False
        sid_filtering = True

        if self.has_flag('WITHIN_FOREST'):
            trusttype = 'ParentChild'
            is_transitive = True
            sid_filtering = self.has_flag('QUARANTINED_DOMAIN')
        elif self.has_flag('FOREST_TRANSITIVE'):
            trusttype = 'Forest'
            is_transitive = True
            # SID filtering is disabled if this flag is set
            sid_filtering = not self.has_flag('TREAT_AS_EXTERNAL')
        elif self.has_flag('TREAT_AS_EXTERNAL') or self.has_flag('CROSS_ORGANIZATION'):
            trusttype = 'External'
            is_transitive = False
            sid_filtering = True
        else:
            is_transitive = not self.has_flag('NON_TRANSITIVE')

        # Make sure this is an int
        trust_direction = int(self.direction)
        trusttype_out = self.bh_trust_type.get(trusttype, 4)

        out = {
            "TargetDomainName": self.destination_domain.upper(),
            "TargetDomainSid": self.domainsid,
            "IsTransitive": is_transitive,
            "TrustDirection": trust_direction,
            "TrustType": trusttype_out,
            "SidFilteringEnabled": sid_filtering
        }
        return out
