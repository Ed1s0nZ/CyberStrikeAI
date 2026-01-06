from __future__ import annotations

import logging
import os
import struct
from io import BytesIO

import cart

from cle.errors import CLEInvalidEncryptionError, CLEInvalidFileFormatError

from .backend import Backend, register_backend

log = logging.getLogger(__name__)


class CARTFile(Backend):
    """
    The CaRT file format is used to store/transfer malware and its associated metadata. It neuters the malware so it
    cannot be executed and encrypts it so anti-virus software cannot flag the CaRT file as malware.

    Ref: https://github.com/CybercentreCanada/cart

    CART file does not store the name of the unpacked file. This backend automatically creates one and stores at
    self.unpacked_name.
    """

    is_default = True
    is_outer = True

    def __init__(self, binary, binary_stream, *args, arc4_key=None, **kwargs):
        super().__init__(binary, binary_stream, *args, **kwargs)
        self.set_load_args(arc4_key=arc4_key)

        # hack: we are using a loader internal method in a non-kosher way which will cause our children to be
        # marked as the main binary if we are also the main binary
        # work around this by setting ourself here:
        ostream = BytesIO()
        try:
            _ = cart.unpack_stream(
                binary_stream,
                ostream,
                arc4_key_override=arc4_key,
            )
        except cart.cart.InvalidARC4KeyException as ex:
            raise CLEInvalidEncryptionError(backend=CARTFile, enckey_argname="arc4_key") from ex
        except cart.cart.InvalidCARTException as ex:
            raise CLEInvalidFileFormatError() from ex

        self.unpacked_name = self.get_unpacked_name(binary)

        if self.loader._main_object is None:
            self.loader._main_object = self
        child = self.loader._load_object_isolated(ostream, obj_ident=self.unpacked_name)
        self.child_objects.append(child)
        self.has_memory = False

        if self.loader._main_object is self:
            # clean up the main_object after use
            self.loader._main_object = None

        self.force_main_object = child  # the loader will pick it up

    @classmethod
    def is_compatible(cls, stream) -> bool:
        stream.seek(0)
        header = stream.read(6)
        stream.seek(0)
        if len(header) != 6:
            return False
        (magic, version) = struct.unpack("4sh", header)
        return magic == b"CART" and version == 1

    @staticmethod
    def get_unpacked_name(file_path: str) -> str:
        if isinstance(file_path, str):
            return os.path.basename(file_path) + ".unpacked"
        return "cart.unpacked"


register_backend("cart", CARTFile)
