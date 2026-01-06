"""
"""

# Created on 2015.04.02
#
# Author: Giovanni Cannata
#
# Copyright 2015 - 2020 Giovanni Cannata
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

# NTLMv2 authentication as per [MS-NLMP] (https://msdn.microsoft.com/en-us/library/cc236621.aspx)

from struct import pack, unpack
from platform import system, version
from socket import gethostname
from time import time
import hmac
import hashlib
import binascii
from os import urandom

try:
    from locale import getpreferredencoding
    oem_encoding = getpreferredencoding()
except Exception:
    oem_encoding = 'utf-8'

from ..protocol.formatters.formatters import format_ad_timestamp

NTLM_SIGNATURE = b'NTLMSSP\x00'
NTLM_MESSAGE_TYPE_NTLM_NEGOTIATE = 1
NTLM_MESSAGE_TYPE_NTLM_CHALLENGE = 2
NTLM_MESSAGE_TYPE_NTLM_AUTHENTICATE = 3

FLAG_NEGOTIATE_56 = 31  # W
FLAG_NEGOTIATE_KEY_EXCH = 30  # V
FLAG_NEGOTIATE_128 = 29  # U
FLAG_NEGOTIATE_VERSION = 25  # T
FLAG_NEGOTIATE_TARGET_INFO = 23  # S
FLAG_REQUEST_NOT_NT_SESSION_KEY = 22  # R
FLAG_NEGOTIATE_IDENTIFY = 20  # Q
FLAG_NEGOTIATE_EXTENDED_SESSIONSECURITY = 19  # P
FLAG_TARGET_TYPE_SERVER = 17  # O
FLAG_TARGET_TYPE_DOMAIN = 16  # N
FLAG_NEGOTIATE_ALWAYS_SIGN = 15  # M
FLAG_NEGOTIATE_OEM_WORKSTATION_SUPPLIED = 13  # L
FLAG_NEGOTIATE_OEM_DOMAIN_SUPPLIED = 12  # K
FLAG_NEGOTIATE_ANONYMOUS = 11  # J
FLAG_NEGOTIATE_NTLM = 9  # H
FLAG_NEGOTIATE_LM_KEY = 7  # G
FLAG_NEGOTIATE_DATAGRAM = 6  # F
FLAG_NEGOTIATE_SEAL = 5  # E
FLAG_NEGOTIATE_SIGN = 4  # D
FLAG_REQUEST_TARGET = 2  # C
FLAG_NEGOTIATE_OEM = 1  # B
FLAG_NEGOTIATE_UNICODE = 0  # A

FLAG_TYPES = [FLAG_NEGOTIATE_56,
              FLAG_NEGOTIATE_KEY_EXCH,
              FLAG_NEGOTIATE_128,
              FLAG_NEGOTIATE_VERSION,
              FLAG_NEGOTIATE_TARGET_INFO,
              FLAG_REQUEST_NOT_NT_SESSION_KEY,
              FLAG_NEGOTIATE_IDENTIFY,
              FLAG_NEGOTIATE_EXTENDED_SESSIONSECURITY,
              FLAG_TARGET_TYPE_SERVER,
              FLAG_TARGET_TYPE_DOMAIN,
              FLAG_NEGOTIATE_ALWAYS_SIGN,
              FLAG_NEGOTIATE_OEM_WORKSTATION_SUPPLIED,
              FLAG_NEGOTIATE_OEM_DOMAIN_SUPPLIED,
              FLAG_NEGOTIATE_ANONYMOUS,
              FLAG_NEGOTIATE_NTLM,
              FLAG_NEGOTIATE_LM_KEY,
              FLAG_NEGOTIATE_DATAGRAM,
              FLAG_NEGOTIATE_SEAL,
              FLAG_NEGOTIATE_SIGN,
              FLAG_REQUEST_TARGET,
              FLAG_NEGOTIATE_OEM,
              FLAG_NEGOTIATE_UNICODE]

AV_END_OF_LIST = 0
AV_NETBIOS_COMPUTER_NAME = 1
AV_NETBIOS_DOMAIN_NAME = 2
AV_DNS_COMPUTER_NAME = 3
AV_DNS_DOMAIN_NAME = 4
AV_DNS_TREE_NAME = 5
AV_FLAGS = 6
AV_TIMESTAMP = 7
AV_SINGLE_HOST_DATA = 8
AV_TARGET_NAME = 9
AV_CHANNEL_BINDINGS = 10

AV_TYPES = [AV_END_OF_LIST,
            AV_NETBIOS_COMPUTER_NAME,
            AV_NETBIOS_DOMAIN_NAME,
            AV_DNS_COMPUTER_NAME,
            AV_DNS_DOMAIN_NAME,
            AV_DNS_TREE_NAME,
            AV_FLAGS,
            AV_TIMESTAMP,
            AV_SINGLE_HOST_DATA,
            AV_TARGET_NAME,
            AV_CHANNEL_BINDINGS]

AV_FLAG_CONSTRAINED = 0
AV_FLAG_INTEGRITY = 1
AV_FLAG_TARGET_SPN_UNTRUSTED = 2

AV_FLAG_TYPES = [AV_FLAG_CONSTRAINED,
                 AV_FLAG_INTEGRITY,
                 AV_FLAG_TARGET_SPN_UNTRUSTED]


def pack_windows_version(debug=False):
    if debug:
        if system().lower() == 'windows':
            try:
                major_release, minor_release, build = version().split('.')
                major_release = int(major_release)
                minor_release = int(minor_release)
                build = int(build)
            except Exception:
                major_release = 5
                minor_release = 1
                build = 2600
        else:
            major_release = 5
            minor_release = 1
            build = 2600
    else:
        major_release = 0
        minor_release = 0
        build = 0

    return pack('<B', major_release) + \
           pack('<B', minor_release) + \
           pack('<H', build) + \
           pack('<B', 0) + \
           pack('<B', 0) + \
           pack('<B', 0) + \
           pack('<B', 15)


def unpack_windows_version(version_message):
    if len(version_message) != 8:
        raise ValueError('version field must be 8 bytes long')

    if str is bytes:  # Python 2
        return (unpack('<B', version_message[0])[0],
                unpack('<B', version_message[1])[0],
                unpack('<H', version_message[2:4])[0],
                unpack('<B', version_message[7])[0])
    else:  # Python 3
        return (int(version_message[0]),
                int(version_message[1]),
                int(unpack('<H', version_message[2:4])[0]),
                int(version_message[7]))


class NtlmClient(object):
    def __init__(self, domain, user_name, password):
        self.client_config_flags = 0
        self.exported_session_key = None
        self.negotiated_flags = None
        self.user_name = user_name
        self.user_domain = domain
        self.no_lm_response_ntlm_v1 = None
        self.client_blocked = False
        self.client_block_exceptions = []
        self.client_require_128_bit_encryption = None
        self.max_life_time = None
        self.client_signing_key = None
        self.client_sealing_key = None
        self.sequence_number = None
        self.server_sealing_key = None
        self.server_signing_key = None
        self.integrity = False
        self.replay_detect = False
        self.sequence_detect = False
        self.confidentiality = False
        self.datagram = False
        self.identity = False
        self.client_supplied_target_name = None
        self.client_channel_binding_unhashed = None
        self.unverified_target_name = None
        self._password = password
        self.server_challenge = None
        self.server_target_name = None
        self.server_target_info = None
        self.server_version = None
        self.server_av_netbios_computer_name = None
        self.server_av_netbios_domain_name = None
        self.server_av_dns_computer_name = None
        self.server_av_dns_domain_name = None
        self.server_av_dns_forest_name = None
        self.server_av_target_name = None
        self.server_av_flags = None
        self.server_av_timestamp = None
        self.server_av_single_host_data = None
        self.server_av_channel_bindings = None
        self.server_av_flag_constrained = None
        self.server_av_flag_integrity = None
        self.server_av_flag_target_spn_untrusted = None
        self.current_encoding = None
        self.client_challenge = None
        self.server_target_info_raw = None

    def get_client_flag(self, flag):
        if not self.client_config_flags:
            return False

        if flag in FLAG_TYPES:
            return True if self.client_config_flags & (1 << flag) else False

        raise ValueError('invalid flag')

    def get_negotiated_flag(self, flag):
        if not self.negotiated_flags:
            return False

        if flag not in FLAG_TYPES:
            raise ValueError('invalid flag')

        return True if self.negotiated_flags & (1 << flag) else False

    def get_server_av_flag(self, flag):
        if not self.server_av_flags:
            return False

        if flag not in AV_FLAG_TYPES:
            raise ValueError('invalid AV flag')

        return True if self.server_av_flags & (1 << flag) else False

    def set_client_flag(self, flags):
        if type(flags) == int:
            flags = [flags]
        for flag in flags:
            if flag in FLAG_TYPES:
                self.client_config_flags |= (1 << flag)
            else:
                raise ValueError('invalid flag')

    def reset_client_flags(self):
        self.client_config_flags = 0

    def unset_client_flag(self, flags):
        if type(flags) == int:
            flags = [flags]
        for flag in flags:
            if flag in FLAG_TYPES:
                self.client_config_flags &= ~(1 << flag)
            else:
                raise ValueError('invalid flag')

    def create_negotiate_message(self):
        """
        Microsoft MS-NLMP 2.2.1.1
        """
        self.reset_client_flags()
        self.set_client_flag([FLAG_REQUEST_TARGET,
                              FLAG_NEGOTIATE_56,
                              FLAG_NEGOTIATE_128,
                              FLAG_NEGOTIATE_NTLM,
                              FLAG_NEGOTIATE_ALWAYS_SIGN,
                              FLAG_NEGOTIATE_OEM,
                              FLAG_NEGOTIATE_UNICODE,
                              FLAG_NEGOTIATE_EXTENDED_SESSIONSECURITY])

        message = NTLM_SIGNATURE  # 8 bytes
        message += pack('<I', NTLM_MESSAGE_TYPE_NTLM_NEGOTIATE)  # 4 bytes
        message += pack('<I', self.client_config_flags)  # 4 bytes
        message += self.pack_field('', 40)  # domain name field  # 8 bytes
        if self.get_client_flag(FLAG_NEGOTIATE_VERSION):  # version 8 bytes - used for debug in ntlm
            message += pack_windows_version(True)
        else:
            message += pack_windows_version(False)
        return message

    def parse_challenge_message(self, message):
        """
        Microsoft MS-NLMP 2.2.1.2
        """
        if len(message) < 56:  # minimum size of challenge message
            return False

        if message[0:8] != NTLM_SIGNATURE:  # NTLM signature - 8 bytes
            return False

        if int(unpack('<I', message[8:12])[0]) != NTLM_MESSAGE_TYPE_NTLM_CHALLENGE:  # type of message - 4 bytes
            return False

        target_name_len, _, target_name_offset = self.unpack_field(message[12:20])  # targetNameFields - 8 bytes
        self.negotiated_flags = unpack('<I', message[20:24])[0]  # negotiated flags - 4 bytes
        self.current_encoding = 'utf-16-le' if self.get_negotiated_flag(
            FLAG_NEGOTIATE_UNICODE) else oem_encoding  # set encoding

        self.server_challenge = message[24:32]  # server challenge - 8 bytes
        target_info_len, _, target_info_offset = self.unpack_field(message[40:48])  # targetInfoFields - 8 bytes
        self.server_version = unpack_windows_version(message[48:56])
        if self.get_negotiated_flag(FLAG_REQUEST_TARGET) and target_name_len:
            self.server_target_name = message[target_name_offset: target_name_offset + target_name_len].decode(
                self.current_encoding)
        if self.get_negotiated_flag(FLAG_NEGOTIATE_TARGET_INFO) and target_info_len:
            self.server_target_info_raw = message[target_info_offset: target_info_offset + target_info_len]
            self.server_target_info = self.unpack_av_info(self.server_target_info_raw)
            for attribute, value in self.server_target_info:
                if attribute == AV_NETBIOS_COMPUTER_NAME:
                    self.server_av_netbios_computer_name = value.decode('utf-16-le')  # always unicode
                elif attribute == AV_NETBIOS_DOMAIN_NAME:
                    self.server_av_netbios_domain_name = value.decode('utf-16-le')  # always unicode
                elif attribute == AV_DNS_COMPUTER_NAME:
                    self.server_av_dns_computer_name = value.decode('utf-16-le')  # always unicode
                elif attribute == AV_DNS_DOMAIN_NAME:
                    self.server_av_dns_domain_name = value.decode('utf-16-le')  # always unicode
                elif attribute == AV_DNS_TREE_NAME:
                    self.server_av_dns_forest_name = value.decode('utf-16-le')  # always unicode
                elif attribute == AV_FLAGS:
                    if self.get_server_av_flag(AV_FLAG_CONSTRAINED):
                        self.server_av_flag_constrained = True
                    if self.get_server_av_flag(AV_FLAG_INTEGRITY):
                        self.server_av_flag_integrity = True
                    if self.get_server_av_flag(AV_FLAG_TARGET_SPN_UNTRUSTED):
                        self.server_av_flag_target_spn_untrusted = True
                elif attribute == AV_TIMESTAMP:
                    self.server_av_timestamp = format_ad_timestamp(unpack('<Q', value)[0])
                elif attribute == AV_SINGLE_HOST_DATA:
                    self.server_av_single_host_data = value
                elif attribute == AV_TARGET_NAME:
                    self.server_av_target_name = value.decode('utf-16-le')  # always unicode
                elif attribute == AV_CHANNEL_BINDINGS:
                    self.server_av_channel_bindings = value
                else:
                    raise ValueError('unknown AV type')

    def create_authenticate_message(self):
        """
        Microsoft MS-NLMP 2.2.1.3
        """
        # 3.1.5.2
        if not self.client_config_flags and not self.negotiated_flags:
            return False

        # 3.1.5.2
        if self.get_client_flag(FLAG_NEGOTIATE_128) and not self.get_negotiated_flag(FLAG_NEGOTIATE_128):
            return False

        # 3.1.5.2
        if (not self.server_av_netbios_computer_name or not self.server_av_netbios_domain_name) and self.server_av_flag_integrity:
            return False

        message = NTLM_SIGNATURE  # 8 bytes
        message += pack('<I', NTLM_MESSAGE_TYPE_NTLM_AUTHENTICATE)  # 4 bytes
        pos = 88  # payload starts at 88
        # 3.1.5.2
        if self.server_target_info:
            lm_challenge_response = b''
        else:
            # computed LmChallengeResponse - todo
            lm_challenge_response = b''

        message += self.pack_field(lm_challenge_response, pos)  # LmChallengeResponseField field  # 8 bytes
        pos += len(lm_challenge_response)
        nt_challenge_response = self.compute_nt_response()
        message += self.pack_field(nt_challenge_response, pos)  # NtChallengeResponseField field  # 8 bytes
        pos += len(nt_challenge_response)
        domain_name = self.user_domain.encode(self.current_encoding)
        message += self.pack_field(domain_name, pos)  # DomainNameField field  # 8 bytes
        pos += len(domain_name)
        user_name = self.user_name.encode(self.current_encoding)
        message += self.pack_field(user_name, pos)  # UserNameField field  # 8 bytes
        pos += len(user_name)
        if self.get_negotiated_flag(FLAG_NEGOTIATE_OEM_WORKSTATION_SUPPLIED) or self.get_negotiated_flag(
                FLAG_NEGOTIATE_VERSION):
            workstation = gethostname().encode(self.current_encoding)
        else:
            workstation = b''
        message += self.pack_field(workstation, pos)  # empty WorkstationField field  # 8 bytes
        pos += len(workstation)
        encrypted_random_session_key = b''
        message += self.pack_field(encrypted_random_session_key, pos)  # EncryptedRandomSessionKeyField field  # 8 bytes
        pos += len(encrypted_random_session_key)
        message += pack('<I', self.negotiated_flags)  # negotiated flags - 4 bytes
        if self.get_negotiated_flag(FLAG_NEGOTIATE_VERSION):
            message += pack_windows_version(True)  # windows version - 8 bytes
        else:
            message += pack_windows_version()  # empty windows version - 8 bytes
        message += pack('<Q', 0)  # mic
        message += pack('<Q', 0)  # mic - total of 16 bytes
        # payload starts at 88
        message += lm_challenge_response
        message += nt_challenge_response
        message += domain_name
        message += user_name
        message += workstation
        message += encrypted_random_session_key

        return message

    @staticmethod
    def pack_field(value, offset):
        return pack('<HHI', len(value), len(value), offset)

    @staticmethod
    def unpack_field(field_message):
        if len(field_message) != 8:
            raise ValueError('ntlm field must be 8 bytes long')
        return unpack('<H', field_message[0:2])[0], \
               unpack('<H', field_message[2:4])[0], \
               unpack('<I', field_message[4:8])[0]

    @staticmethod
    def unpack_av_info(info):
        if info:
            avs = list()
            done = False
            pos = 0
            while not done:
                av_type = unpack('<H', info[pos: pos + 2])[0]
                if av_type not in AV_TYPES:
                    raise ValueError('unknown AV type')
                av_len = unpack('<H', info[pos + 2: pos + 4])[0]
                av_value = info[pos + 4: pos + 4 + av_len]
                pos += av_len + 4
                if av_type == AV_END_OF_LIST:
                    done = True
                else:
                    avs.append((av_type, av_value))
        else:
            return list()

        return avs

    @staticmethod
    def pack_av_info(avs):
        # avs is a list of tuples, each tuple is made of av_type and av_value
        info = b''
        for av_type, av_value in avs:
            if av_type(0) == AV_END_OF_LIST:
                continue
            info += pack('<H', av_type)
            info += pack('<H', len(av_value))
            info += av_value

        # add AV_END_OF_LIST
        info += pack('<H', AV_END_OF_LIST)
        info += pack('<H', 0)

        return info

    @staticmethod
    def pack_windows_timestamp():
        return pack('<Q', (int(time()) + 11644473600) * 10000000)

    def compute_nt_response(self):
        if not self.user_name and not self._password:  # anonymous authentication
            return b''

        self.client_challenge = urandom(8)
        temp = b''
        temp += pack('<B', 1)  # ResponseVersion - 1 byte
        temp += pack('<B', 1)  # HiResponseVersion - 1 byte
        temp += pack('<H', 0)  # Z(2)
        temp += pack('<I', 0)  # Z(4) - total Z(6)
        temp += self.pack_windows_timestamp()  # time - 8 bytes
        temp += self.client_challenge  # random client challenge - 8 bytes
        temp += pack('<I', 0)  # Z(4)
        temp += self.server_target_info_raw
        temp += pack('<I', 0)  # Z(4)
        response_key_nt = self.ntowf_v2()
        nt_proof_str = hmac.new(response_key_nt, self.server_challenge + temp, digestmod=hashlib.md5).digest()
        nt_challenge_response = nt_proof_str + temp
        return nt_challenge_response

    def ntowf_v2(self):
        passparts = self._password.split(':')
        if len(passparts) == 2 and len(passparts[0]) == 32 and len(passparts[1]) == 32:
            # The specified password is an LM:NTLM hash
            password_digest = binascii.unhexlify(passparts[1])
        else:
            try:
                password_digest = hashlib.new('MD4', self._password.encode('utf-16-le')).digest()
            except ValueError as e:
                try:
                    from Crypto.Hash import MD4  # try with the Crypto library if present
                    password_digest = MD4.new(self._password.encode('utf-16-le')).digest()
                except ImportError:
                    raise e  # raise original exception

        return hmac.new(password_digest, (self.user_name.upper() + self.user_domain).encode('utf-16-le'), digestmod=hashlib.md5).digest()
