#!/usr/bin/env python3
#
# Author:
#  Tamas Jos (@skelsec)
#
import io
import enum

#https://msdn.microsoft.com/en-us/library/windows/desktop/ms680388(v=vs.85).aspx
class MinidumpMiscInfo2Flags1(enum.IntFlag):
	MINIDUMP_MISC1_PROCESS_ID = 0x00000001 #ProcessId is used.
	MINIDUMP_MISC1_PROCESS_TIMES = 0x00000002 #ProcessCreateTime, ProcessKernelTime, and ProcessUserTime are used.
	MINIDUMP_MISC1_PROCESSOR_POWER_INFO = 0x00000004 #ProcessorMaxMhz, ProcessorCurrentMhz, ProcessorMhzLimit, ProcessorMaxIdleState, and ProcessorCurrentIdleState are used.

# https://msdn.microsoft.com/en-us/library/windows/desktop/ms680389(v=vs.85).aspx
class MinidumpMiscInfoFlags1(enum.IntFlag):
	MINIDUMP_MISC1_PROCESS_ID = 0x00000001 #ProcessId is used.
	MINIDUMP_MISC1_PROCESS_TIMES = 0x00000002 #ProcessCreateTime, ProcessKernelTime, and ProcessUserTime are used.

# https://msdn.microsoft.com/en-us/library/windows/desktop/ms680389(v=vs.85).aspx
class MINIDUMP_MISC_INFO:
	size = 24
	def __init__(self):
		self.SizeOfInfo = None
		self.Flags1 = None
		self.ProcessId = None
		self.ProcessCreateTime = None
		self.ProcessUserTime = None
		self.ProcessKernelTime = None

	@staticmethod
	def parse(buff):
		mmi = MINIDUMP_MISC_INFO()
		mmi.SizeOfInfo = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		mmi.Flags1 = MinidumpMiscInfoFlags1(int.from_bytes(buff.read(4), byteorder = 'little', signed = False))
		if mmi.Flags1 & MinidumpMiscInfoFlags1.MINIDUMP_MISC1_PROCESS_ID:
			mmi.ProcessId = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		else:
			buff.read(4)
		if mmi.Flags1 & MinidumpMiscInfoFlags1.MINIDUMP_MISC1_PROCESS_TIMES:
			mmi.ProcessCreateTime = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
			mmi.ProcessUserTime = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
			mmi.ProcessKernelTime = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		else:
			buff.read(12)

		return mmi

#https://msdn.microsoft.com/en-us/library/windows/desktop/ms680388(v=vs.85).aspx
class MINIDUMP_MISC_INFO_2:
	size = 44
	def __init__(self):
		self.SizeOfInfo = None
		self.Flags1 = None
		self.ProcessId = None
		self.ProcessCreateTime = None
		self.ProcessUserTime = None
		self.ProcessKernelTime = None
		self.ProcessorMaxMhz = None
		self.ProcessorCurrentMhz = None
		self.ProcessorMhzLimit = None
		self.ProcessorMaxIdleState = None
		self.ProcessorCurrentIdleState = None

	@staticmethod
	def parse(buff):
		mmi = MINIDUMP_MISC_INFO_2()
		mmi.SizeOfInfo = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		mmi.Flags1 = MinidumpMiscInfo2Flags1(int.from_bytes(buff.read(4), byteorder = 'little', signed = False))
		if mmi.Flags1 & MinidumpMiscInfo2Flags1.MINIDUMP_MISC1_PROCESS_ID:
			mmi.ProcessId = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		else:
			buff.read(4)
		if mmi.Flags1 & MinidumpMiscInfo2Flags1.MINIDUMP_MISC1_PROCESS_TIMES:
			mmi.ProcessCreateTime = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
			mmi.ProcessUserTime = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
			mmi.ProcessKernelTime = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		else:
			buff.read(12)
		if mmi.Flags1 & MinidumpMiscInfo2Flags1.MINIDUMP_MISC1_PROCESSOR_POWER_INFO:
			mmi.ProcessorMaxMhz = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
			mmi.ProcessorCurrentMhz = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
			mmi.ProcessorMhzLimit = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
			mmi.ProcessorMaxIdleState = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
			mmi.ProcessorCurrentIdleState = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		else:
			buff.read(20)

		return mmi

class MinidumpMiscInfo:
	def __init__(self):
		self.ProcessId = None
		self.ProcessCreateTime = None
		self.ProcessUserTime = None
		self.ProcessKernelTime = None
		self.ProcessorMaxMhz = None
		self.ProcessorCurrentMhz = None
		self.ProcessorMhzLimit = None
		self.ProcessorMaxIdleState = None
		self.ProcessorCurrentIdleState = None

	@staticmethod
	def parse(dir, buff):
		t = MinidumpMiscInfo()
		buff.seek(dir.Location.Rva)
		chunk = io.BytesIO(buff.read(dir.Location.DataSize))
		if dir.Location.DataSize == MINIDUMP_MISC_INFO.size:
			misc = MINIDUMP_MISC_INFO.parse(chunk)
			t.ProcessId = misc.ProcessId
			t.ProcessCreateTime = misc.ProcessCreateTime
			t.ProcessUserTime = misc.ProcessUserTime
			t.ProcessKernelTime = misc.ProcessKernelTime
		else:
			misc = MINIDUMP_MISC_INFO_2.parse(chunk)
			t.ProcessId = misc.ProcessId
			t.ProcessCreateTime = misc.ProcessCreateTime
			t.ProcessUserTime = misc.ProcessUserTime
			t.ProcessKernelTime = misc.ProcessKernelTime
			t.ProcessorMaxMhz = misc.ProcessorMaxMhz
			t.ProcessorCurrentMhz = misc.ProcessorCurrentMhz
			t.ProcessorMhzLimit = misc.ProcessorMhzLimit
			t.ProcessorMaxIdleState = misc.ProcessorMaxIdleState
			t.ProcessorCurrentIdleState = misc.ProcessorCurrentIdleState
		return t

	@staticmethod
	async def aparse(dir, buff):
		t = MinidumpMiscInfo()
		await buff.seek(dir.Location.Rva)
		chunk_data = await buff.read(dir.Location.DataSize)
		chunk = io.BytesIO(chunk_data)
		if dir.Location.DataSize == MINIDUMP_MISC_INFO.size:
			misc = MINIDUMP_MISC_INFO.parse(chunk)
			t.ProcessId = misc.ProcessId
			t.ProcessCreateTime = misc.ProcessCreateTime
			t.ProcessUserTime = misc.ProcessUserTime
			t.ProcessKernelTime = misc.ProcessKernelTime
		else:
			misc = MINIDUMP_MISC_INFO_2.parse(chunk)
			t.ProcessId = misc.ProcessId
			t.ProcessCreateTime = misc.ProcessCreateTime
			t.ProcessUserTime = misc.ProcessUserTime
			t.ProcessKernelTime = misc.ProcessKernelTime
			t.ProcessorMaxMhz = misc.ProcessorMaxMhz
			t.ProcessorCurrentMhz = misc.ProcessorCurrentMhz
			t.ProcessorMhzLimit = misc.ProcessorMhzLimit
			t.ProcessorMaxIdleState = misc.ProcessorMaxIdleState
			t.ProcessorCurrentIdleState = misc.ProcessorCurrentIdleState
		return t

	def __str__(self):
		t  = '== MinidumpMiscInfo ==\n'
		t += 'ProcessId %s\n' % self.ProcessId
		t += 'ProcessCreateTime %s\n' % self.ProcessCreateTime
		t += 'ProcessUserTime %s\n' % self.ProcessUserTime
		t += 'ProcessKernelTime %s\n' % self.ProcessKernelTime
		t += 'ProcessorMaxMhz %s\n' % self.ProcessorMaxMhz
		t += 'ProcessorCurrentMhz %s\n' % self.ProcessorCurrentMhz
		t += 'ProcessorMhzLimit %s\n' % self.ProcessorMhzLimit
		t += 'ProcessorMaxIdleState %s\n' % self.ProcessorMaxIdleState
		t += 'ProcessorCurrentIdleState %s\n' % self.ProcessorCurrentIdleState
		return t