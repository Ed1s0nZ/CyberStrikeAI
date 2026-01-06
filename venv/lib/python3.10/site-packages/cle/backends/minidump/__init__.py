from __future__ import annotations

import ntpath
import struct

import archinfo
from minidump import minidumpfile
from minidump.streams import SystemInfoStream

from cle.backends.backend import Backend, register_backend
from cle.backends.region import Section, Segment
from cle.errors import CLEError, CLEInvalidBinaryError


class MinidumpMissingStreamError(Exception):
    def __init__(self, stream, message=None):
        super().__init__()
        self.message = message
        self.stream = stream


class Minidump(Backend):
    is_default = True

    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.os = "windows"
        self.supports_nx = True
        if self.binary is None:
            self._mdf = minidumpfile.MinidumpFile.parse_bytes(self._binary_stream.read())
        else:
            self._mdf = minidumpfile.MinidumpFile.parse(self.binary)

        self.wow64 = False

        if self._arch is None:
            if getattr(self._mdf, "sysinfo", None) is None:
                raise MinidumpMissingStreamError("SystemInfo", "The architecture was not specified")
            arch = self._mdf.sysinfo.ProcessorArchitecture
            if arch == SystemInfoStream.PROCESSOR_ARCHITECTURE.AMD64:
                if any(module.name.endswith("wow64.dll") for module in self._mdf.modules.modules):
                    self.wow64 = True
                    self.set_arch(archinfo.ArchX86())
                else:
                    self.set_arch(archinfo.ArchAMD64())
            elif arch == SystemInfoStream.PROCESSOR_ARCHITECTURE.INTEL:
                self.set_arch(archinfo.ArchX86())
            else:
                # has not been tested with other architectures
                raise CLEError("Loading minidumps is not implemented for this architecture")

        if self._mdf.memory_segments_64 is not None:
            segments = self._mdf.memory_segments_64.memory_segments
        elif self._mdf.memory_segments is not None:
            segments = self._mdf.memory_segments.memory_segments
        else:
            raise MinidumpMissingStreamError("MemoryList", "The memory segments were not defined")

        for segment in segments:
            data = segment.read(segment.start_virtual_address, segment.size, self._mdf.file_handle)
            self.segments.append(
                Segment(segment.start_file_address, segment.start_virtual_address, segment.size, segment.size)
            )
            self.memory.add_backer(segment.start_virtual_address, data)

        for module in self._mdf.modules.modules:
            for segment in segments:
                if segment.start_virtual_address == module.baseaddress:
                    break
            else:
                raise CLEInvalidBinaryError("Missing segment for loaded module: " + module.name)
            section = Section(module.name, segment.start_file_address, module.baseaddress, module.size)
            self.sections.append(section)
            self.sections_map[ntpath.basename(section.name)] = section

        self._thread_data = {}

        for thread in self._mdf.threads.threads:
            tid = thread.ThreadId
            teb = thread.Teb
            self._binary_stream.seek(thread.ThreadContext.Rva)  # pylint: disable=undefined-loop-variable
            data = self._binary_stream.read(thread.ThreadContext.DataSize)  # pylint: disable=undefined-loop-variable
            self._binary_stream.seek(0)
            self._thread_data[tid] = (teb, data)

    def close(self):
        super().close()
        self._mdf.file_handle.close()
        del self._mdf

    @staticmethod
    def is_compatible(stream):
        identstring = stream.read(4)
        stream.seek(0)
        return identstring == b"MDMP"

    @property
    def threads(self):
        return list(self._thread_data)

    def thread_registers(self, thread=None):
        if thread is None:
            thread = self.threads[0]

        teb, data = self._thread_data[thread]

        if self.arch.name == "AMD64" or self.wow64:
            fmt = "QQQQQQIIHHHHHHIQQQQQQQQQQQQQQQQQQQQQQQ"
            fmt_registers = {
                #'fs':     11, 'gs':  12,
                "eflags": 14,
                "rax": 21,
                "rcx": 22,
                "rdx": 23,
                "rbx": 24,
                "rsp": 25,
                "rbp": 26,
                "rsi": 27,
                "rdi": 28,
                "r8": 29,
                "r9": 30,
                "r10": 31,
                "r11": 32,
                "r12": 33,
                "r13": 34,
                "r14": 35,
                "r15": 36,
                "rip": 37,
            }
        elif self.arch.name == "X86":
            fmt = "IIIIIII112xIIIIIIIIIIIIIIII512x"
            fmt_registers = {
                #'gs':     7,  'fs':  8,
                "edi": 11,
                "esi": 12,
                "ebx": 13,
                "edx": 14,
                "ecx": 15,
                "eax": 16,
                "ebp": 17,
                "eip": 18,
                "eflags": 20,
                "esp": 21,
            }
        else:
            raise CLEError("Deserializing minidump registers is not implemented for this architecture")
        members = struct.unpack_from(fmt, data)
        thread_registers = {}
        for register, position in fmt_registers.items():
            thread_registers[register] = members[position]

        if self.arch.name == "AMD64" or self.wow64:
            gs_base = self.memory.unpack_word(teb + 0x30)
            thread_registers["gs_const"] = gs_base
            if self.arch.name == "AMD64":
                NUM_XMM_REGS = 16
                xmms = struct.unpack_from("16s" * NUM_XMM_REGS, data, offset=0x1A0)
                thread_registers |= {
                    f"xmm{i}": int.from_bytes(xmm, "little", signed=False) for i, xmm in enumerate(xmms)
                }
        elif self.arch.name == "X86":
            fs_base = self.memory.unpack_word(teb + 0x18)
            thread_registers["fs"] = fs_base

        if self.wow64:
            register_translation = [
                ("edi", "rdi"),
                ("esi", "rsi"),
                ("ebx", "rbx"),
                ("edx", "rdx"),
                ("ecx", "rcx"),
                ("eax", "rax"),
                ("ebp", "rbp"),
                ("eip", "rip"),
                ("eflags", "eflags"),
                ("esp", "rsp"),
                ("fs", "gs_const"),  # ???
            ]

            thread_registers = {ereg: thread_registers[rreg] & 0xFFFFFFFF for ereg, rreg in register_translation}

        return thread_registers

    def get_thread_registers_by_id(self, thread_id):
        return self.thread_registers(thread_id)


register_backend("minidump", Minidump)
