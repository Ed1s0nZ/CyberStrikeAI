####################
#
# Copyright (c) 2022 Fox-IT
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
from multiprocessing import Pool
from impacket.uuid import string_to_bin, bin_to_string
from bloodhound.lib import cstruct
from io import BytesIO

# Extended rights and property GUID mapping, converted to binary so we don't have to do this
# for every comparison.
# Source: https://msdn.microsoft.com/en-us/library/cc223512.aspx
EXTRIGHTS_GUID_MAPPING = {
    "GetChanges": string_to_bin("1131f6aa-9c07-11d1-f79f-00c04fc2dcd2"),
    "GetChangesAll": string_to_bin("1131f6ad-9c07-11d1-f79f-00c04fc2dcd2"),
    "GetChangesInFilteredSet": string_to_bin("89e95b76-444d-4c62-991a-0facbeda640c"),
    "WriteMember": string_to_bin("bf9679c0-0de6-11d0-a285-00aa003049e2"),
    "UserForceChangePassword": string_to_bin("00299570-246d-11d0-a768-00aa006e0529"),
    "AllowedToAct": string_to_bin("3f78c3e5-f79a-46bd-a0b8-9d18116ddc79"),
    "UserAccountRestrictionsSet": string_to_bin("4c164200-20c0-11d0-a768-00aa006e0529"),
    "WriteGPLink": string_to_bin("f30e3bbe-9ff0-11d1-b603-0000f80367c1")
}

def parse_binary_acl(entry, entrytype, acl, objecttype_guid_map):
    """
    Main ACL structure parse function.
    This is offloaded to subprocesses and takes the current entry and the
    acl data as argument. This is then returned and processed back in the main process
    """
    if not acl:
        return entry, []
    sd = SecurityDescriptor(BytesIO(acl))

    # Check for protected DACL flag
    entry['IsACLProtected'] = sd.has_control(sd.PD)
    relations = []

    # Parse owner
    osid = str(sd.owner_sid)
    ignoresids = ["S-1-3-0", "S-1-5-18", "S-1-5-10"]
    # Ignore Creator Owner or Local System
    if osid not in ignoresids:
        relations.append(build_relation(osid, 'Owns', inherited=False))
    for ace_object in sd.dacl.aces:
        if ace_object.ace.AceType != 0x05 and ace_object.ace.AceType != 0x00:
            # These are the only two aces we care about currently
            logging.debug('Don\'t care about acetype %d', ace_object.ace.AceType)
            continue
        # Check if sid is ignored
        sid = str(ace_object.acedata.sid)
        # Ignore Creator Owner or Local System
        if sid in ignoresids:
            continue

        # For debugging, print specific aces
        # if sid == "S-1-5-21-2895268558-4179327395-2773671012-513":
        #     print(ace_object)
        if ace_object.ace.AceType == 0x05:
            is_inherited = ace_object.has_flag(ACE.INHERITED_ACE)
            # ACCESS_ALLOWED_OBJECT_ACE
            if not ace_object.has_flag(ACE.INHERITED_ACE) and ace_object.has_flag(ACE.INHERIT_ONLY_ACE):
                # ACE is set on this object, but only inherited, so not applicable to us
                continue

            # Check if the ACE has restrictions on object type (inherited case)
            if ace_object.has_flag(ACE.INHERITED_ACE) \
                and ace_object.acedata.has_flag(ACCESS_ALLOWED_OBJECT_ACE.ACE_INHERITED_OBJECT_TYPE_PRESENT):
                # Verify if the ACE applies to this object type
                if not ace_applies(ace_object.acedata.get_inherited_object_type().lower(), entrytype, objecttype_guid_map):
                    continue

            mask = ace_object.acedata.mask
            # Now the magic, we have to check all the rights BloodHound cares about

            # Check generic access masks first
            if mask.has_priv(ACCESS_MASK.GENERIC_ALL) or mask.has_priv(ACCESS_MASK.WRITE_DACL) \
                or mask.has_priv(ACCESS_MASK.WRITE_OWNER) or mask.has_priv(ACCESS_MASK.GENERIC_WRITE):
                # For all generic rights we should check if it applies to our object type
                if ace_object.acedata.has_flag(ACCESS_ALLOWED_OBJECT_ACE.ACE_OBJECT_TYPE_PRESENT) \
                    and not ace_applies(ace_object.acedata.get_object_type().lower(), entrytype, objecttype_guid_map):
                    # If it does not apply, break out of the loop here in order to
                    # avoid individual rights firing later on
                    continue
                # Check from high to low, ignore lower privs which may also match the bitmask,
                # even though this shouldn't happen since we check for exact matches currently
                if mask.has_priv(ACCESS_MASK.GENERIC_ALL):
                    # Report this as LAPS rights if it's a computer object AND laps is enabled
                    if entrytype == 'computer' and \
                    ace_object.acedata.has_flag(ACCESS_ALLOWED_OBJECT_ACE.ACE_OBJECT_TYPE_PRESENT) and \
                    entry['Properties']['haslaps']:
                        if ace_object.acedata.get_object_type().lower() == objecttype_guid_map['ms-mcs-admpwd']:
                            relations.append(build_relation(sid, 'ReadLAPSPassword', inherited=is_inherited))
                    else:
                        relations.append(build_relation(sid, 'GenericAll', inherited=is_inherited))
                    continue
                if mask.has_priv(ACCESS_MASK.GENERIC_WRITE):
                    relations.append(build_relation(sid, 'GenericWrite', inherited=is_inherited))
                    # Don't skip this if it's the domain object, since BloodHound reports duplicate
                    # rights as well, and this might influence some queries
                    if entrytype != 'domain' and entrytype != 'computer':
                        continue

                # These are specific bitmasks so don't break the loop from here
                if mask.has_priv(ACCESS_MASK.WRITE_DACL):
                    relations.append(build_relation(sid, 'WriteDacl', inherited=is_inherited))

                if mask.has_priv(ACCESS_MASK.WRITE_OWNER):
                    relations.append(build_relation(sid, 'WriteOwner', inherited=is_inherited))

            # Property write privileges
            writeprivs = ace_object.acedata.mask.has_priv(ACCESS_MASK.ADS_RIGHT_DS_WRITE_PROP)
            if writeprivs:
                # GenericWrite
                if entrytype in ['user', 'group', 'computer', 'gpo', 'organizational-unit'] and not ace_object.acedata.has_flag(ACCESS_ALLOWED_OBJECT_ACE.ACE_OBJECT_TYPE_PRESENT):
                    relations.append(build_relation(sid, 'GenericWrite', inherited=is_inherited))
                if entrytype == 'group' and can_write_property(ace_object, EXTRIGHTS_GUID_MAPPING['WriteMember']):
                    relations.append(build_relation(sid, 'AddMember', '', inherited=is_inherited))
                if entrytype == 'computer' and can_write_property(ace_object, EXTRIGHTS_GUID_MAPPING['AllowedToAct']):
                    relations.append(build_relation(sid, 'AddAllowedToAct', '', inherited=is_inherited))
                # Property set, but ignore Domain Admins since they already have enough privileges anyway
                if entrytype in ['computer', 'user'] and can_write_property(ace_object, EXTRIGHTS_GUID_MAPPING['UserAccountRestrictionsSet']) and not sid.endswith('-512'):
                    relations.append(build_relation(sid, 'WriteAccountRestrictions', '', inherited=is_inherited))
                if entrytype == 'organizational-unit' and can_write_property(ace_object, EXTRIGHTS_GUID_MAPPING['WriteGPLink']):
                    relations.append(build_relation(sid, 'WriteGPLink', '', inherited=is_inherited))


                # Since 4.0
                # Key credential link property write rights
                if entrytype in ['user', 'computer'] and ace_object.acedata.has_flag(ACCESS_ALLOWED_OBJECT_ACE.ACE_OBJECT_TYPE_PRESENT) \
                and 'ms-ds-key-credential-link' in objecttype_guid_map and ace_object.acedata.get_object_type().lower() == objecttype_guid_map['ms-ds-key-credential-link']:
                    relations.append(build_relation(sid, 'AddKeyCredentialLink', inherited=is_inherited))

                # ServicePrincipalName property write rights (exclude generic rights)
                if entrytype in ['user', 'computer'] and ace_object.acedata.has_flag(ACCESS_ALLOWED_OBJECT_ACE.ACE_OBJECT_TYPE_PRESENT) \
                and ace_object.acedata.get_object_type().lower() == objecttype_guid_map['service-principal-name']:
                    relations.append(build_relation(sid, 'WriteSPN', inherited=is_inherited))

            elif ace_object.acedata.mask.has_priv(ACCESS_MASK.ADS_RIGHT_DS_SELF):
                # Self add - since 4.0
                if entrytype == 'group' and ace_object.acedata.data.ObjectType == EXTRIGHTS_GUID_MAPPING['WriteMember']:
                    relations.append(build_relation(sid, 'AddSelf', '', inherited=is_inherited))

            # Property read privileges
            if ace_object.acedata.mask.has_priv(ACCESS_MASK.ADS_RIGHT_DS_READ_PROP) and \
                ace_object.acedata.mask.has_priv(ACCESS_MASK.ADS_RIGHT_DS_CONTROL_ACCESS):
                if entrytype == 'computer' and \
                ace_object.acedata.has_flag(ACCESS_ALLOWED_OBJECT_ACE.ACE_OBJECT_TYPE_PRESENT) and \
                entry['Properties']['haslaps']:
                    if ace_object.acedata.get_object_type().lower() in (
                        objecttype_guid_map.get("ms-mcs-admpwd"),
                        objecttype_guid_map.get("ms-laps-password"),
                        objecttype_guid_map.get("ms-laps-encryptedpassword"),
                    ):
                        relations.append(build_relation(sid, 'ReadLAPSPassword', inherited=is_inherited))

            # Extended rights
            control_access = ace_object.acedata.mask.has_priv(ACCESS_MASK.ADS_RIGHT_DS_CONTROL_ACCESS)
            if control_access:
                # All Extended
                if entrytype in ['user', 'domain'] and not ace_object.acedata.has_flag(ACCESS_ALLOWED_OBJECT_ACE.ACE_OBJECT_TYPE_PRESENT):
                    relations.append(build_relation(sid, 'AllExtendedRights', '', inherited=is_inherited))
                # Also report all extended if no laps
                if entrytype == 'computer' and not ace_object.acedata.has_flag(ACCESS_ALLOWED_OBJECT_ACE.ACE_OBJECT_TYPE_PRESENT):
                    relations.append(build_relation(sid, 'AllExtendedRights', '', inherited=is_inherited))
                if entrytype == 'domain' and has_extended_right(ace_object, EXTRIGHTS_GUID_MAPPING['GetChanges']):
                    relations.append(build_relation(sid, 'GetChanges', '', inherited=is_inherited))
                if entrytype == 'domain' and has_extended_right(ace_object, EXTRIGHTS_GUID_MAPPING['GetChangesAll']):
                    relations.append(build_relation(sid, 'GetChangesAll', '', inherited=is_inherited))
                if entrytype == 'domain' and has_extended_right(ace_object, EXTRIGHTS_GUID_MAPPING['GetChangesInFilteredSet']):
                    relations.append(build_relation(sid, 'GetChangesInFilteredSet', '', inherited=is_inherited))
                if entrytype in ['user', 'computer'] and has_extended_right(ace_object, EXTRIGHTS_GUID_MAPPING['UserForceChangePassword']):
                    relations.append(build_relation(sid, 'ForceChangePassword', '', inherited=is_inherited))

        if ace_object.ace.AceType == 0x00:
            is_inherited = ace_object.has_flag(ACE.INHERITED_ACE)
            mask = ace_object.acedata.mask
            # ACCESS_ALLOWED_ACE
            if not ace_object.has_flag(ACE.INHERITED_ACE) and ace_object.has_flag(ACE.INHERIT_ONLY_ACE):
                # ACE is set on this object, but only inherited, so not applicable to us
                continue
            
            if mask.has_priv(ACCESS_MASK.GENERIC_ALL):
                # Generic all includes all other rights, so skip from here
                relations.append(build_relation(sid, 'GenericAll', inherited=is_inherited))
                continue

            if mask.has_priv(ACCESS_MASK.ADS_RIGHT_DS_WRITE_PROP):
                # Genericwrite is only for properties, don't skip after
                if entrytype in ['user', 'group', 'computer', 'gpo', 'organizational-unit']:
                    relations.append(build_relation(sid, 'GenericWrite', inherited=is_inherited))

            if mask.has_priv(ACCESS_MASK.WRITE_OWNER):
                relations.append(build_relation(sid, 'WriteOwner', inherited=is_inherited))

            # For users and domain, check extended rights
            if entrytype in ['user', 'domain'] and mask.has_priv(ACCESS_MASK.ADS_RIGHT_DS_CONTROL_ACCESS):
                relations.append(build_relation(sid, 'AllExtendedRights', '', inherited=is_inherited))

            # Also report all extended if no laps
            if entrytype == 'computer' and mask.has_priv(ACCESS_MASK.ADS_RIGHT_DS_CONTROL_ACCESS) and \
                sid != "S-1-5-32-544" and not sid.endswith('-512'):
                relations.append(build_relation(sid, 'AllExtendedRights', '', inherited=is_inherited))

            if mask.has_priv(ACCESS_MASK.WRITE_DACL):
                relations.append(build_relation(sid, 'WriteDacl', inherited=is_inherited))

            # Self add, also possible ad ACCESS_ALLOWED_ACE
            if mask.has_priv(ACCESS_MASK.ADS_RIGHT_DS_SELF) and \
                sid != "S-1-5-32-544" and not sid.endswith('-512') and not sid.endswith('-519'):
                if entrytype == 'group':
                    relations.append(build_relation(sid, 'AddSelf', '', inherited=is_inherited))

    # pprint.pprint(entry)
        # pprint.pprint(relations)
    return entry, relations

def can_write_property(ace_object, binproperty):
    '''
    Checks if the access is sufficient to write to a specific property.
    This can either be because we have the right ADS_RIGHT_DS_WRITE_PROP and the correct GUID
    is set in ObjectType, or if we have the ADS_RIGHT_DS_WRITE_PROP right and the ObjectType
    is empty, in which case we can write to any property. This is documented in
    [MS-ADTS] section 5.1.3.2: https://msdn.microsoft.com/en-us/library/cc223511.aspx
    '''
    if not ace_object.acedata.mask.has_priv(ACCESS_MASK.ADS_RIGHT_DS_WRITE_PROP):
        return False
    if not ace_object.acedata.has_flag(ACCESS_ALLOWED_OBJECT_ACE.ACE_OBJECT_TYPE_PRESENT):
        # No ObjectType present - we have generic access on all properties
        return True
    # Both are binary here
    if ace_object.acedata.data.ObjectType == binproperty:
        return True
    return False

def has_extended_right(ace_object, binrightguid):
    '''
    Checks if the access is sufficient to control the right with the given GUID.
    This can either be because we have the right ADS_RIGHT_DS_CONTROL_ACCESS and the correct GUID
    is set in ObjectType, or if we have the ADS_RIGHT_DS_CONTROL_ACCESS right and the ObjectType
    is empty, in which case we have all extended rights. This is documented in
    [MS-ADTS] section 5.1.3.2: https://msdn.microsoft.com/en-us/library/cc223511.aspx
    '''
    if not ace_object.acedata.mask.has_priv(ACCESS_MASK.ADS_RIGHT_DS_CONTROL_ACCESS):
        return False
    if not ace_object.acedata.has_flag(ACCESS_ALLOWED_OBJECT_ACE.ACE_OBJECT_TYPE_PRESENT):
        # No ObjectType present - we have all extended rights
        return True
    # Both are binary here
    if ace_object.acedata.data.ObjectType == binrightguid:
        return True
    return False

def ace_applies(ace_guid, object_class, objecttype_guid_map):
    '''
    Checks if an ACE applies to this object (based on object classes).
    Note that this function assumes you already verified that InheritedObjectType is set (via the flag).
    If this is not set, the ACE applies to all object types.
    '''
    if ace_guid == objecttype_guid_map[object_class]:
        return True
    # If none of these match, the ACE does not apply to this object
    return False

def build_relation(sid, relation, acetype='', inherited=False):
    if acetype != '':
        raise ValueError("BH 4.0 incompatible output called")
    return {'rightname': relation, 'sid': sid, 'inherited': inherited}

class AclEnumerator(object):
    """
    Helper class for ACL parsing.
    """
    def __init__(self, addomain, addc, collect):
        self.addomain = addomain
        self.addc = addc
        # Store collection methods specified
        self.collect = collect
        self.pool = None

    def init_pool(self):
        self.pool = Pool()

"""
The following is Security Descriptor parsing using cstruct
Thanks to Erik Schamper for helping me implement this!
"""
cdef = """
struct SECURITY_DESCRIPTOR {
    uint8   Revision;
    uint8   Sbz1;
    uint16  Control;
    uint32  OffsetOwner;
    uint32  OffsetGroup;
    uint32  OffsetSacl;
    uint32  OffsetDacl;
};

struct LDAP_SID_IDENTIFIER_AUTHORITY {
    char    Value[6];
};

struct LDAP_SID {
    uint8   Revision;
    uint8   SubAuthorityCount;
    LDAP_SID_IDENTIFIER_AUTHORITY   IdentifierAuthority;
    uint32  SubAuthority[SubAuthorityCount];
};

struct ACL {
    uint8   AclRevision;
    uint8   Sbz1;
    uint16  AclSize;
    uint16  AceCount;
    uint16  Sbz2;
    char    Data[AclSize - 8];
};

struct ACE {
    uint8   AceType;
    uint8   AceFlags;
    uint16  AceSize;
    char    Data[AceSize - 4];
};

struct ACCESS_ALLOWED_ACE {
    uint32  Mask;
    LDAP_SID Sid;
};

struct ACCESS_ALLOWED_OBJECT_ACE {
    uint32  Mask;
    uint32  Flags;
    char    ObjectType[Flags & 1 * 16];
    char    InheritedObjectType[Flags & 2 * 8];
    LDAP_SID Sid;
};
"""
c_secd = cstruct()
c_secd.load(cdef, compiled=True)


class SecurityDescriptor(object):
    # Control indexes in bit field
    SR = 0  # Self-Relative
    RM = 1  # RM Control Valid
    PS = 2  # SACL Protected
    PD = 3  # DACL Protected
    SI = 4  # SACL Auto-Inherited
    DI = 5  # DACL Auto-Inherited
    SC = 6  # SACL Computed Inheritance Required
    DC = 7  # DACL Computed Inheritance Required
    SS = 8  # Server Security
    DT = 9  # DACL Trusted
    SD = 10 # SACL Defaulted
    SP = 11 # SACL Present
    DD = 12 # DACL Defaulted
    DP = 13 # DACL Present
    GD = 14 # Group Defaulted
    OD = 15 # Owner Defaulted

    def has_control(self, control):
        # Convert to bin representation and
        # look up index. Slice off 0b
        return bin(self.control)[2:][control] == '1'

    def __init__(self, fh):
        self.fh = fh
        self.descriptor = c_secd.SECURITY_DESCRIPTOR(fh)

        self.control = self.descriptor.Control
        self.owner_sid = b''
        self.group_sid = b''
        self.sacl = b''
        self.dacl = b''

        if self.descriptor.OffsetOwner != 0:
            fh.seek(self.descriptor.OffsetOwner)
            self.owner_sid = LdapSid(fh=fh)

        if self.descriptor.OffsetGroup != 0:
            fh.seek(self.descriptor.OffsetGroup)
            self.group_sid = LdapSid(fh=fh)

        if self.descriptor.OffsetSacl != 0:
            fh.seek(self.descriptor.OffsetSacl)
            self.sacl = ACL(fh)

        if self.descriptor.OffsetDacl != 0:
            fh.seek(self.descriptor.OffsetDacl)
            self.dacl = ACL(fh)


class LdapSid(object):
    def __init__(self, fh=None, in_obj=None):
        if fh:
            self.fh = fh
            self.ldap_sid = c_secd.LDAP_SID(fh)
        else:
            self.ldap_sid = in_obj

    def __repr__(self):
        return "S-{}-{}-{}".format(self.ldap_sid.Revision, bytearray(self.ldap_sid.IdentifierAuthority.Value)[5], "-".join(['{:d}'.format(v) for v in self.ldap_sid.SubAuthority]))


class ACL(object):
    def __init__(self, fh):
        self.fh = fh
        self.acl = c_secd.ACL(fh)
        self.aces = []

        buf = BytesIO(self.acl.Data)
        for i in range(self.acl.AceCount):
            self.aces.append(ACE(buf))


class ACCESS_ALLOWED_ACE(object):
    def __init__(self, fh):
        self.fh = fh
        self.data = c_secd.ACCESS_ALLOWED_ACE(fh)
        self.sid = LdapSid(in_obj=self.data.Sid)
        self.mask = ACCESS_MASK(self.data.Mask)

    def __repr__(self):
        return "<ACCESS_ALLOWED_OBJECT_ACE Sid=%s Mask=%s>" % (str(self.sid), str(self.mask))

class ACCESS_DENIED_ACE(ACCESS_ALLOWED_ACE):
    pass


class ACCESS_ALLOWED_OBJECT_ACE(object):
    # Flag constants
    ACE_OBJECT_TYPE_PRESENT             = 0x01
    ACE_INHERITED_OBJECT_TYPE_PRESENT   = 0x02

    def __init__(self, fh):
        self.fh = fh
        self.data = c_secd.ACCESS_ALLOWED_OBJECT_ACE(fh)
        self.sid = LdapSid(in_obj=self.data.Sid)
        self.mask = ACCESS_MASK(self.data.Mask)

    def has_flag(self, flag):
        return self.data.Flags & flag == flag

    def get_object_type(self):
        if self.has_flag(self.ACE_OBJECT_TYPE_PRESENT):
            return bin_to_string(self.data.ObjectType)
        return None

    def get_inherited_object_type(self):
        if self.has_flag(self.ACE_INHERITED_OBJECT_TYPE_PRESENT):
            return bin_to_string(self.data.InheritedObjectType)
        return None

    def __repr__(self):
        out = []
        for name, value in vars(ACCESS_ALLOWED_OBJECT_ACE).items():
            if not name.startswith('_') and type(value) is int and self.has_flag(value):
                out.append(name)
        data = (' | '.join(out),
                str(self.sid),
                str(self.mask),
                self.get_object_type(),
                self.get_inherited_object_type())
        return "<ACCESS_ALLOWED_OBJECT_ACE Flags=%s Sid=%s \n\t\tMask=%s \n\t\tObjectType=%s InheritedObjectType=%s>" % data

class ACCESS_DENIED_OBJECT_ACE(ACCESS_ALLOWED_OBJECT_ACE):
    pass


"""
ACCESS_MASK as described in 2.4.3
https://msdn.microsoft.com/en-us/library/cc230294.aspx
"""
class ACCESS_MASK(object):
    # Flag constants

    # These constants are only used when WRITING
    # and are then translated into their actual rights
    SET_GENERIC_READ        = 0x80000000
    SET_GENERIC_WRITE       = 0x04000000
    SET_GENERIC_EXECUTE     = 0x20000000
    SET_GENERIC_ALL         = 0x10000000
    # When reading, these constants are actually represented by
    # the following for Active Directory specific Access Masks
    # Reference: https://docs.microsoft.com/en-us/dotnet/api/system.directoryservices.activedirectoryrights?view=netframework-4.7.2
    GENERIC_READ            = 0x00020094
    GENERIC_WRITE           = 0x00020028
    GENERIC_EXECUTE         = 0x00020004
    GENERIC_ALL             = 0x000F01FF

    # These are actual rights (for all ACE types)
    MAXIMUM_ALLOWED         = 0x02000000
    ACCESS_SYSTEM_SECURITY  = 0x01000000
    SYNCHRONIZE             = 0x00100000
    WRITE_OWNER             = 0x00080000
    WRITE_DACL              = 0x00040000
    READ_CONTROL            = 0x00020000
    DELETE                  = 0x00010000

    # ACE type specific mask constants (for ACCESS_ALLOWED_OBJECT_ACE)
    # Note that while not documented, these also seem valid
    # for ACCESS_ALLOWED_ACE types
    ADS_RIGHT_DS_CONTROL_ACCESS         = 0x00000100
    ADS_RIGHT_DS_CREATE_CHILD           = 0x00000001
    ADS_RIGHT_DS_DELETE_CHILD           = 0x00000002
    ADS_RIGHT_DS_READ_PROP              = 0x00000010
    ADS_RIGHT_DS_WRITE_PROP             = 0x00000020
    ADS_RIGHT_DS_SELF                   = 0x00000008

    def __init__(self, mask):
        self.mask = mask

    def has_priv(self, priv):
        return self.mask & priv == priv

    def set_priv(self, priv):
        self.mask |= priv

    def remove_priv(self, priv):
        self.mask ^= priv

    def __repr__(self):
        out = []
        for name, value in vars(ACCESS_MASK).items():
            if not name.startswith('_') and type(value) is int and self.has_priv(value):
                out.append(name)
        return "<ACCESS_MASK RawMask=%d Flags=%s>" % (self.mask, ' | '.join(out))



class ACE(object):
    CONTAINER_INHERIT_ACE       = 0x02
    FAILED_ACCESS_ACE_FLAG      = 0x80
    INHERIT_ONLY_ACE            = 0x08
    INHERITED_ACE               = 0x10
    NO_PROPAGATE_INHERIT_ACE    = 0x04
    OBJECT_INHERIT_ACE          = 0x01
    SUCCESSFUL_ACCESS_ACE_FLAG  = 0x04

    def __init__(self, fh):
        self.fh = fh
        self.ace = c_secd.ACE(fh)
        self.acedata = None
        buf = BytesIO(self.ace.Data)
        if self.ace.AceType == 0x00:
            # ACCESS_ALLOWED_ACE
            self.acedata = ACCESS_ALLOWED_ACE(buf)
        elif self.ace.AceType == 0x05:
            # ACCESS_ALLOWED_OBJECT_ACE
            self.acedata = ACCESS_ALLOWED_OBJECT_ACE(buf)
        elif self.ace.AceType == 0x01:
            # ACCESS_DENIED_ACE
            self.acedata = ACCESS_DENIED_ACE(buf)
        elif self.ace.AceType == 0x06:
            # ACCESS_DENIED_OBJECT_ACE
            self.acedata = ACCESS_DENIED_OBJECT_ACE(buf)
        # else:
        #     print 'Unsupported type %d' % self.ace.AceType

        if self.acedata:
            self.mask = ACCESS_MASK(self.acedata.data.Mask)

    def __repr__(self):
        out = []
        for name, value in vars(ACE).items():
            if not name.startswith('_') and type(value) is int and self.has_flag(value):
                out.append(name)
        return "<ACE Type=%s Flags=%s RawFlags=%d \n\tAce=%s>" % (self.ace.AceType, ' | '.join(out), self.ace.AceFlags, str(self.acedata))

    def has_flag(self, flag):
        return self.ace.AceFlags & flag == flag
