"""
This module is used when parsing the Thread Local Storage of an ELF binary. It heavily uses the TLSArchInfo
namedtuple from archinfo.

ELF TLS is implemented based on the following documents:

    - https://www.uclibc.org/docs/tls.pdf
    - https://www.uclibc.org/docs/tls-ppc.txt
    - https://www.uclibc.org/docs/tls-ppc64.txt
    - https://www.linux-mips.org/wiki/NPTL
"""

from __future__ import annotations

from .tls_object import InternalTLSRelocation, ThreadManager, TLSObject

TLS_BLOCK_ALIGN = 0x10
TLS_TOTAL_HEAD_SIZE = 0x4000
TLS_HEAD_ALIGN = 0x10000


def roundup(val, to=TLS_BLOCK_ALIGN):
    return val - 1 + (to - ((val - 1) % to))


class ELFThreadManager(ThreadManager):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.used_data = 0

    def register_object(self, obj):
        if not super().register_object(obj):
            return False

        # only track tls_block_offset for modules registered before the first thread
        # ??? is this right
        if not self.threads:
            # tls_block_offset has these semantics: the thread pointer plus the block offset equals the address of the
            # module's TLS data in memory
            if self.arch.elf_tls.variant == 1:
                obj.tls_block_offset = self.used_data
            else:
                obj.tls_block_offset = -self.used_data - roundup(obj.tls_block_size)

            self.used_data += roundup(obj.tls_block_size)
        return True

    @property
    def _thread_cls(self):
        if self.arch.elf_tls.variant == 1:
            return ELFTLSObjectV1
        else:
            return ELFTLSObjectV2


class ELFTLSObject(TLSObject):
    def __init__(self, thread_manager: ELFThreadManager):
        super().__init__(loader=thread_manager.loader, arch=thread_manager.arch)
        self.tcb_offset: int = None
        self.dtv_offset: int = None
        self.tp_offset: int = None
        self.head_offset: int = None
        self._max_addr: int = None
        self.tlsoffsets = [obj.tls_block_offset for obj in thread_manager.modules]
        self.pic = True

        self._calculate_pointers(thread_manager.used_data, thread_manager.max_modules)

        # add backer for header
        self.memory.add_backer(self.head_offset, bytes(TLS_TOTAL_HEAD_SIZE))

        # add backer for dtv
        self.memory.add_backer(
            self.dtv_offset - 2 * self.arch.bytes,
            bytes(2 * self.arch.bytes * thread_manager.max_modules + 2 * self.arch.bytes),
        )

        # Set the appropriate pointers in the tcbhead
        for off in self.arch.elf_tls.head_offsets:
            self._drop_int(off + self.tcb_offset, self.tp_offset, True)
        for off in self.arch.elf_tls.dtv_offsets:
            self._drop_int(off + self.tcb_offset, self.dtv_offset, True)
        for off in self.arch.elf_tls.pthread_offsets:
            self._drop_int(off + self.tcb_offset, self.tp_offset, True)  # ?????

        # tid. feel free to move this code wherever
        # this only matters if you're not running the libc initializers... hm.
        # tid = len(thread_manager.threads) + 1
        # if self.arch.name == 'AMD64':
        #    self._drop_int(self.tcb_offset + 0x2d0, tid, False, size=4)

        # Set up the DTV
        # at dtv[-1] there's capacity, at dtv[0] there's count (technically generation number?)
        self._drop_int(self.dtv_offset - 2 * self.arch.bytes, thread_manager.max_modules - 1)
        self._drop_int(self.dtv_offset, len(thread_manager.modules))

        # set up each module
        for obj in thread_manager.modules:
            # dtv entry
            dtv_entry_offset = self.dtv_offset + 2 * self.arch.bytes * obj.tls_module_id
            self._drop_int(
                dtv_entry_offset, self.tp_offset + obj.tls_block_offset + self.arch.elf_tls.dtv_entry_offset, True
            )
            self._drop_int(dtv_entry_offset + self.arch.bytes, 1)

            # initialization image
            image = thread_manager.initialization_image(obj)
            if image is None:
                continue
            self.memory.add_backer(self.tp_offset + obj.tls_block_offset, image)

    def _calculate_pointers(self, used_data, max_modules):
        raise NotImplementedError

    def _drop_int(self, offset, num, needs_relocation=False, **kwargs):
        if needs_relocation:
            self.relocs.append(InternalTLSRelocation(num, offset, self))
        self.memory.pack_word(offset, num, **kwargs)

    @property
    def thread_pointer(self):
        """
        The thread pointer. This is a technical term that refers to a specific location in the TLS segment.
        """
        return self.mapped_base + self.tp_offset

    @property
    def user_thread_pointer(self):
        """
        The thread pointer that is exported to the user
        """
        return self.thread_pointer + self.arch.elf_tls.tp_offset

    @property
    def max_addr(self):
        return self.mapped_base + self._max_addr

    def get_addr(self, module_id, offset):
        """
        basically ``__tls_get_addr``.
        """
        return self.memory.unpack_word(self.dtv_offset + module_id * self.arch.bytes) + offset


class ELFTLSObjectV1(ELFTLSObject):
    # variant 1: memory is laid out like so:
    # [header][module data]
    #         ^ thread pointer
    def _calculate_pointers(self, used_data, max_modules):
        self.tcb_offset = TLS_TOTAL_HEAD_SIZE - self.arch.elf_tls.tcbhead_size
        # CRITICAL DIFFERENCE FROM THE DOC - variant 1 seems to expect the thread pointer points to the end of the TCB
        self.tp_offset = TLS_TOTAL_HEAD_SIZE
        self.dtv_offset = TLS_TOTAL_HEAD_SIZE + used_data + 2 * self.arch.bytes
        self.head_offset = 0  # ^^ that's the point of this field
        self._max_addr = self.dtv_offset + 2 * self.arch.bytes * max_modules - 1


class ELFTLSObjectV2(ELFTLSObject):
    # variant 2: memory is laid out like so:
    # [module data][header]
    #              ^ thread pointer
    def _calculate_pointers(self, used_data, max_modules):
        self.tcb_offset = roundup(used_data, TLS_HEAD_ALIGN)
        self.tp_offset = roundup(used_data, TLS_HEAD_ALIGN)
        self.dtv_offset = self.tp_offset + TLS_TOTAL_HEAD_SIZE + 2 * self.arch.bytes
        self.head_offset = self.tp_offset
        self._max_addr = self.dtv_offset + 2 * self.arch.bytes * max_modules - 1
