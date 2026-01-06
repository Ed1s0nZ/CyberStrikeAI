from __future__ import annotations

import logging

import archinfo

from .tls_object import ThreadManager

log = logging.getLogger(__name__)


class ELFCoreThreadManager(ThreadManager):
    def __init__(self, loader, arch, **kwargs):  # pylint: disable=unused-argument
        self.loader = loader
        self.arch = arch
        self.threads = [ELFCoreThread(loader, arch, threadinfo) for threadinfo in loader.main_object._threads]
        if arch.name not in ("AMD64", "X86"):
            log.warning("TLS for coredumps won't be right for this arch - idk how to do it")
        self.modules = []  # ???

    def new_thread(self, insert=False):  # pylint: disable=no-self-use
        raise TypeError("Cannot create new threads from a core file... for now")

    def register_object(self, obj):
        pass


class ELFCoreThread:
    def __init__(self, loader, arch: archinfo.Arch, threadinfo):
        self.loader = loader
        self.arch = arch
        self._threadinfo = threadinfo
        if arch.name == "AMD64":
            self.thread_pointer = threadinfo["registers"]["fs_base"]
        elif arch.name == "X86":
            gs = threadinfo["registers"]["gs"]
            if gs == 0:
                # I have no idea why this happens
                gs = next(iter(threadinfo["segments"].keys())) << 3
            self.thread_pointer = threadinfo["segments"][gs >> 3][0]
        else:
            self.thread_pointer = 0

        self.user_thread_pointer = self.thread_pointer + arch.elf_tls.tp_offset

    @property
    def dtv(self):
        return self.loader.memory.unpack_word(self.thread_pointer + self.arch.elf_tls.dtv_offsets[0])

    def get_addr(self, module_id, offset):
        """
        basically ``__tls_get_addr``.
        """
        return self.loader.memory.unpack_word(self.dtv + module_id * self.arch.bytes) + offset
