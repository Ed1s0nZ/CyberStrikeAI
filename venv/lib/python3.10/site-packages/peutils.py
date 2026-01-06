
"""peutils, Portable Executable utilities module


Copyright (c) 2005-2023 Ero Carrera <ero.carrera@gmail.com>

All rights reserved.
"""
import os
import re
import string
import urllib.error
import urllib.parse
import urllib.request

import pefile

__author__ = "Ero Carrera"
__version__ = pefile.__version__
__contact__ = "ero.carrera@gmail.com"


class SignatureDatabase:
    """This class loads and keeps a parsed PEiD signature database.

    Usage:

        sig_db = SignatureDatabase('/path/to/signature/file')

    and/or

        sig_db = SignatureDatabase()
        sig_db.load('/path/to/signature/file')

    Signature databases can be combined by performing multiple loads.

    The filename parameter can be a URL too. In that case the
    signature database will be downloaded from that location.
    """

    def __init__(self, filename=None, data=None):

        # RegExp to match a signature block
        #
        self.parse_sig = re.compile(
            r"\[(.*?)\]\s+?signature\s*=\s*(.*?)(\s+\?\?)*\s*ep_only\s*=\s*(\w+)(?:\s*section_start_only\s*=\s*(\w+)|)",
            re.S,
        )

        # Signature information
        #
        # Signatures are stored as trees using dictionaries
        # The keys are the byte values while the values for
        # each key are either:
        #
        # - Other dictionaries of the same form for further
        #   bytes in the signature
        #
        # - A dictionary with a string as a key (packer name)
        #   and None as value to indicate a full signature
        #
        self.signature_tree_eponly_true = dict()
        self.signature_count_eponly_true = 0
        self.signature_tree_eponly_false = dict()
        self.signature_count_eponly_false = 0
        self.signature_tree_section_start = dict()
        self.signature_count_section_start = 0

        # The depth (length) of the longest signature
        #
        self.max_depth = 0

        self.__load(filename=filename, data=data)

    def generate_section_signatures(self, pe, name, sig_length=512):
        """Generates signatures for all the sections in a PE file.

        If the section contains any data a signature will be created
        for it. The signature name will be a combination of the
        parameter 'name' and the section number and its name.
        """

        section_signatures = list()

        for idx, section in enumerate(pe.sections):

            if section.SizeOfRawData < sig_length:
                continue

            # offset = pe.get_offset_from_rva(section.VirtualAddress)
            offset = section.PointerToRawData

            sig_name = "%s Section(%d/%d,%s)" % (
                name,
                idx + 1,
                len(pe.sections),
                "".join([c for c in section.Name if c in string.printable]),
            )

            section_signatures.append(
                self.__generate_signature(
                    pe,
                    offset,
                    sig_name,
                    ep_only=False,
                    section_start_only=True,
                    sig_length=sig_length,
                )
            )

        return "\n".join(section_signatures) + "\n"

    def generate_ep_signature(self, pe, name, sig_length=512):
        """Generate signatures for the entry point of a PE file.

        Creates a signature whose name will be the parameter 'name'
        and the section number and its name.
        """

        offset = pe.get_offset_from_rva(pe.OPTIONAL_HEADER.AddressOfEntryPoint)

        return self.__generate_signature(
            pe, offset, name, ep_only=True, sig_length=sig_length
        )

    def __generate_signature(
        self, pe, offset, name, ep_only=False, section_start_only=False, sig_length=512
    ):

        data = pe.__data__[offset : offset + sig_length]

        signature_bytes = " ".join(["%02x" % ord(c) for c in data])

        if ep_only == True:
            ep_only = "true"
        else:
            ep_only = "false"

        if section_start_only == True:
            section_start_only = "true"
        else:
            section_start_only = "false"

        signature = "[%s]\nsignature = %s\nep_only = %s\nsection_start_only = %s\n" % (
            name,
            signature_bytes,
            ep_only,
            section_start_only,
        )

        return signature

    def match(self, pe, ep_only=True, section_start_only=False):
        """Matches and returns the exact match(es).

        If ep_only is True the result will be a string with
        the packer name. Otherwise it will be a list of the
        form (file_offset, packer_name) specifying where
        in the file the signature was found.
        """

        matches = self.__match(pe, ep_only, section_start_only)

        # The last match (the most precise) from the
        # list of matches (if any) is returned
        #
        if matches:
            if ep_only == False:
                # Get the most exact match for each list of matches
                # at a given offset
                #
                return [(match[0], match[1][-1]) for match in matches]

            return matches[1][-1]

        return None

    def match_all(self, pe, ep_only=True, section_start_only=False):
        """Matches and returns all the likely matches."""

        matches = self.__match(pe, ep_only, section_start_only)

        if matches:
            if ep_only == False:
                # Get the most exact match for each list of matches
                # at a given offset
                #
                return matches

            return matches[1]

        return None

    def __match(self, pe, ep_only, section_start_only):

        # Load the corresponding set of signatures
        # Either the one for ep_only equal to True or
        # to False
        #
        if section_start_only is True:

            # Fetch the data of the executable as it'd
            # look once loaded in memory
            #
            try:
                data = pe.__data__
            except Exception as excp:
                raise

            # Load the corresponding tree of signatures
            #
            signatures = self.signature_tree_section_start

            # Set the starting address to start scanning from
            #
            scan_addresses = [section.PointerToRawData for section in pe.sections]

        elif ep_only is True:

            # Fetch the data of the executable as it'd
            # look once loaded in memory
            #
            try:
                data = pe.get_memory_mapped_image()
            except Exception as excp:
                raise

            # Load the corresponding tree of signatures
            #
            signatures = self.signature_tree_eponly_true

            # Fetch the entry point of the PE file and the data
            # at the entry point
            #
            ep = pe.OPTIONAL_HEADER.AddressOfEntryPoint

            # Set the starting address to start scanning from
            #
            scan_addresses = [ep]

        else:

            data = pe.__data__

            signatures = self.signature_tree_eponly_false

            scan_addresses = range(len(data))

        # For each start address, check if any signature matches
        #
        matches = []
        for idx in scan_addresses:
            result = self.__match_signature_tree(
                signatures, data[idx : idx + self.max_depth]
            )
            if result:
                matches.append((idx, result))

        # Return only the matched items found at the entry point if
        # ep_only is True (matches will have only one element in that
        # case)
        #
        if ep_only is True:
            if matches:
                return matches[0]

        return matches

    def match_data(self, code_data, ep_only=True, section_start_only=False):

        data = code_data
        scan_addresses = [0]

        # Load the corresponding set of signatures
        # Either the one for ep_only equal to True or
        # to False
        #
        if section_start_only is True:

            # Load the corresponding tree of signatures
            #
            signatures = self.signature_tree_section_start

            # Set the starting address to start scanning from
            #

        elif ep_only is True:

            # Load the corresponding tree of signatures
            #
            signatures = self.signature_tree_eponly_true

        # For each start address, check if any signature matches
        #
        matches = []
        for idx in scan_addresses:
            result = self.__match_signature_tree(
                signatures, data[idx : idx + self.max_depth]
            )
            if result:
                matches.append((idx, result))

        # Return only the matched items found at the entry point if
        # ep_only is True (matches will have only one element in that
        # case)
        #
        if ep_only is True:
            if matches:
                return matches[0]

        return matches

    def __match_signature_tree(self, signature_tree, data, depth=0):
        """Recursive function to find matches along the signature tree.

        signature_tree  is the part of the tree left to walk
        data    is the data being checked against the signature tree
        depth   keeps track of how far we have gone down the tree
        """

        matched_names = list()
        match = signature_tree

        # Walk the bytes in the data and match them
        # against the signature
        #
        for idx, byte in enumerate([b if isinstance(b, int) else ord(b) for b in data]):

            # If the tree is exhausted...
            #
            if match is None:
                break

            # Get the next byte in the tree
            #
            match_next = match.get(byte, None)

            # If None is among the values for the key
            # it means that a signature in the database
            # ends here and that there's an exact match.
            #
            if None in list(match.values()):
                # idx represent how deep we are in the tree
                #
                # names = [idx+depth]
                names = list()

                # For each of the item pairs we check
                # if it has an element other than None,
                # if not then we have an exact signature
                #
                for item in list(match.items()):
                    if item[1] is None:
                        names.append(item[0])
                matched_names.append(names)

            # If a wildcard is found keep scanning the signature
            # ignoring the byte.
            #
            if "??" in match:
                match_tree_alternate = match.get("??", None)
                data_remaining = data[idx + 1 :]
                if data_remaining:
                    matched_names.extend(
                        self.__match_signature_tree(
                            match_tree_alternate, data_remaining, idx + depth + 1
                        )
                    )

            match = match_next

        # If we have any more packer name in the end of the signature tree
        # add them to the matches
        #
        if match is not None and None in list(match.values()):
            # names = [idx + depth + 1]
            names = list()
            for item in list(match.items()):
                if item[1] is None:
                    names.append(item[0])
            matched_names.append(names)

        return matched_names

    def load(self, filename=None, data=None):
        """Load a PEiD signature file.

        Invoking this method on different files combines the signatures.
        """

        self.__load(filename=filename, data=data)

    def __load(self, filename=None, data=None):

        if filename is not None:
            # If the path does not exist, attempt to open a URL
            #
            if not os.path.exists(filename):
                try:
                    sig_f = urllib.request.urlopen(filename)
                    sig_data = sig_f.read()
                    sig_f.close()
                except OSError:
                    # Let this be raised back to the user...
                    raise
            else:
                # Get the data for a file
                #
                try:
                    sig_f = open(filename, "rt")
                    sig_data = sig_f.read()
                    sig_f.close()
                except OSError:
                    # Let this be raised back to the user...
                    raise
        else:
            sig_data = data

        # If the file/URL could not be read or no "raw" data
        # was provided there's nothing else to do
        #
        if not sig_data:
            return

        # Helper function to parse the signature bytes
        #
        def to_byte(value):
            if "?" in value:
                return value
            return int(value, 16)

        # Parse all the signatures in the file
        #
        matches = self.parse_sig.findall(sig_data)

        # For each signature, get the details and load it into the
        # signature tree
        #
        for (
            packer_name,
            signature,
            superfluous_wildcards,
            ep_only,
            section_start_only,
        ) in matches:

            ep_only = ep_only.strip().lower()

            signature = signature.replace("\\n", "").strip()

            signature_bytes = [to_byte(b) for b in signature.split()]

            if ep_only == "true":
                ep_only = True
            else:
                ep_only = False

            if section_start_only == "true":
                section_start_only = True
            else:
                section_start_only = False

            depth = 0

            if section_start_only is True:

                tree = self.signature_tree_section_start
                self.signature_count_section_start += 1

            else:
                if ep_only is True:
                    tree = self.signature_tree_eponly_true
                    self.signature_count_eponly_true += 1
                else:
                    tree = self.signature_tree_eponly_false
                    self.signature_count_eponly_false += 1

            for idx, byte in enumerate(signature_bytes):

                if idx + 1 == len(signature_bytes):

                    tree[byte] = tree.get(byte, dict())
                    tree[byte][packer_name] = None

                else:

                    tree[byte] = tree.get(byte, dict())

                tree = tree[byte]
                depth += 1

            if depth > self.max_depth:
                self.max_depth = depth


def is_valid(pe):
    """"""
    pass


def is_suspicious(pe):
    """
    unusual locations of import tables
    non recognized section names
    presence of long ASCII strings
    """

    relocations_overlap_entry_point = False
    sequential_relocs = 0

    # If relocation data is found and the entries go over the entry point, and also are very
    # continuous or point outside section's boundaries => it might imply that an obfuscation
    # trick is being used or the relocations are corrupt (maybe intentionally)
    #
    if hasattr(pe, "DIRECTORY_ENTRY_BASERELOC"):
        for base_reloc in pe.DIRECTORY_ENTRY_BASERELOC:
            last_reloc_rva = None
            for reloc in base_reloc.entries:
                if reloc.rva <= pe.OPTIONAL_HEADER.AddressOfEntryPoint <= reloc.rva + 4:
                    relocations_overlap_entry_point = True

                if (
                    last_reloc_rva is not None
                    and last_reloc_rva <= reloc.rva <= last_reloc_rva + 4
                ):
                    sequential_relocs += 1

                last_reloc_rva = reloc.rva

    # If import tables or strings exist (are pointed to) to within the header or in the area
    # between the PE header and the first section that's suspicious
    #
    # IMPLEMENT

    warnings_while_parsing = False
    # If we have warnings, that's suspicious, some of those will be because of out-of-ordinary
    # values are found in the PE header fields
    # Things that are reported in warnings:
    # (parsing problems, special section characteristics i.e. W & X, uncommon values of fields,
    # unusual entrypoint, suspicious imports)
    #
    warnings = pe.get_warnings()
    if warnings:
        warnings_while_parsing

    # If there are few or none (should come with a standard "density" of strings/kilobytes of data) longer (>8)
    # ascii sequences that might indicate packed data, (this is similar to the entropy test in some ways but
    # might help to discard cases of legitimate installer or compressed data)

    # If compressed data (high entropy) and is_driver => uuuuhhh, nasty

    pass


def is_probably_packed(pe):
    """Returns True is there is a high likelihood that a file is packed or contains compressed data.

    The sections of the PE file will be analyzed, if enough sections
    look like containing compressed data and the data makes
    up for more than 20% of the total file size, the function will
    return True.
    """

    # Calculate the length of the data up to the end of the last section in the
    # file. Overlay data won't be taken into account
    #
    total_pe_data_length = len(pe.trim())
    # Assume that the file is packed when no data is available
    if not total_pe_data_length:
        return True
    has_significant_amount_of_compressed_data = False

    # If some of the sections have high entropy and they make for more than 20% of the file's size
    # it's assumed that it could be an installer or a packed file

    total_compressed_data = 0
    for section in pe.sections:
        s_entropy = section.get_entropy()
        s_length = len(section.get_data())
        # The value of 7.4 is empirical, based on looking at a few files packed
        # by different packers
        if s_entropy > 7.4:
            total_compressed_data += s_length

    if (total_compressed_data / total_pe_data_length) > 0.2:
        has_significant_amount_of_compressed_data = True

    return has_significant_amount_of_compressed_data
