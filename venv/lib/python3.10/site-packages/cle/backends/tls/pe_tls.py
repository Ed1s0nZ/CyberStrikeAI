from __future__ import annotations

import logging

from cle.address_translator import AT

from .tls_object import InternalTLSRelocation, ThreadManager, TLSObject

_l = logging.getLogger(__name__)


class PEThreadManager(ThreadManager):
    def register_object(self, obj):
        if not super().register_object(obj):
            return False

        # The PE TLS header says to write its index into a given address
        if hasattr(obj, "tls_index_address"):
            rva = AT.from_lva(obj.tls_index_address, obj).to_rva()
            if rva > 0 and rva in obj.memory:
                try:
                    obj.memory.pack_word(rva, obj.tls_module_id)
                except KeyError:
                    _l.warning(
                        "TLS index address %#x (RVA %#x) does not exist or is not writable.", obj.tls_index_address, rva
                    )
            else:
                # apparently RVA can be an absolute address pointing to another object. we don't handle it yet.
                _l.warning(
                    "TLS index address %#x (RVA %#x) is not mapped in the current object.", obj.tls_index_address, rva
                )
        return True

    @property
    def _thread_cls(self):
        return PETLSObject


class PETLSObject(TLSObject):
    """
    This class is used when parsing the Thread Local Storage of a PE binary. It
    represents both the TLS array and the TLS data area for a specific thread.

    In memory the ``PETLSObj`` is laid out as follows::

        +----------------------+---------------------------------------+
        | TLS array            | TLS data area                         |
        +----------------------+---------------------------------------+

    A more detailed description of the TLS array and TLS data areas is given
    below.

    **TLS array**

    The TLS array is an array of addresses that points into the TLS data area.
    In memory it is laid out as follows::

        +-----------+-----------+-----+-----------+
        |  address  |  address  | ... |  address  |
        +-----------+-----------+-----+-----------+
        | index = 0 | index = 1 |     | index = n |
        +-----------+-----------+-----+-----------+

    The size of each address is architecture independent (e.g. on X86 it is
    4 bytes). The number of addresses in the TLS array is equal to the number
    of modules that contain TLS data. At load time (i.e. in the ``finalize``
    method), each module is assigned an index into the TLS array. The address
    of this module's TLS data area is then stored at this location in the
    array.

    **TLS data area**

    The TLS data area directly follows the TLS array and contains the actual
    TLS data for each module. In memory it is laid out as follows::

        +----------+-----------+----------+-----------+-----+
        | TLS data | zero fill | TLS data | zero fill | ... |
        +----------+-----------+----------+-----------+-----+
        |       module a       |       module b       | ... |
        +---------------------------------------------------+

    The size of each module's TLS data area is variable and can be found in the
    module's ``tls_data_size`` property. The same applies to the zero fill. At
    load time (i.e in the ``finalize`` method), the initial TLS data values are
    copied into the TLS data area. Because a TLS index is also assigned to each
    module, we can access a module's TLS data area using this index into the
    TLS array to get the start address of the TLS data.
    """

    def __init__(self, thread_manager: PEThreadManager):
        super().__init__(loader=thread_manager.loader, arch=thread_manager.arch)

        self.used_modules = len(thread_manager.modules)
        self.data_start = self.arch.bytes * thread_manager.max_modules
        self.used_data = 0
        self.memory.add_backer(0, bytes(self.data_start))
        self.pic = True

        for obj in thread_manager.modules:
            image = thread_manager.initialization_image(obj)
            if image is None:
                continue
            image_offset = self.data_start + self.used_data
            index_offset = obj.tls_module_id * self.arch.bytes

            self.memory.pack_word(index_offset, image_offset)
            self.relocs.append(InternalTLSRelocation(image_offset, index_offset, self))
            self.memory.add_backer(image_offset, image)
            self.used_data += len(image)

    def get_tls_data_addr(self, tls_idx):
        """
        Get the start address of a module's TLS data area via the module's TLS
        index.

        From the PE/COFF spec:

            The code uses the TLS index and the TLS array location (multiplying
            the index by the word size and using it as an offset into the
            array) to get the address of the TLS data area for the given
            program and module.
        """
        if 0 <= tls_idx < self.used_modules:
            return self.memory.unpack_word(tls_idx * self.arch.bytes)
        else:
            raise IndexError("TLS index out of range")

    @property
    def max_addr(self):
        return self.mapped_base + self.data_start + self.used_data - 1

    # PE is MUCH simpler in terms of what's the pointer to the thread data. Add these properties for compatibility.

    @property
    def thread_pointer(self):
        return self.mapped_base

    @property
    def user_thread_pointer(self):
        return self.mapped_base
