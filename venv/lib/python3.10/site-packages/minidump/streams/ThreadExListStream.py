#!/usr/bin/env python3
#
# Author:
#  Tamas Jos (@skelsec)
#

import io
from minidump.common_structs import *
from minidump.streams.MemoryListStream import MINIDUMP_MEMORY_DESCRIPTOR

# https://msdn.microsoft.com/en-us/library/windows/desktop/ms680399(v=vs.85).aspx
class MINIDUMP_THREAD_EX_LIST:
	def __init__(self):
		self.NumberOfThreads = None
		self.Threads = []

	@staticmethod
	def parse(buff):
		mtel = MINIDUMP_THREAD_EX_LIST()
		mtel.NumberOfThreads = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		for _ in range(mtel.NumberOfThreads):
			mtel.Threads.append(MINIDUMP_THREAD_EX.parse(buff))

		return mtel

# https://msdn.microsoft.com/en-us/library/windows/desktop/ms680400(v=vs.85).aspx
class MINIDUMP_THREAD_EX:
	def __init__(self):
		self.ThreadId = None
		self.SuspendCount = None
		self.PriorityClass = None
		self.Priority = None
		self.Teb = None
		self.Stack = None
		self.ThreadContext = None
		self.BackingStore = None

	@staticmethod
	def parse(buff):
		mte = MINIDUMP_THREAD_EX()
		mte.ThreadId = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		mte.SuspendCount = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		mte.PriorityClass = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		mte.Priority = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		mte.Teb = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)
		mte.Stack = MINIDUMP_MEMORY_DESCRIPTOR.parse(buff)
		mte.ThreadContext = MINIDUMP_LOCATION_DESCRIPTOR.parse(buff)
		mte.BackingStore = MINIDUMP_MEMORY_DESCRIPTOR.parse(buff)
		return mte

	@staticmethod
	def get_header():
		return [
			'ThreadId',
			'SuspendCount',
			'PriorityClass',
			'Priority',
			'Teb',
			#'Stack',
			#'ThreadContext',
		]

	def to_row(self):
		return [
			hex(self.ThreadId),
			str(self.SuspendCount),
			str(self.PriorityClass),
			str(self.Priority),
			hex(self.Teb),
			#self.Stack,
			#self.ThreadContext,
		]


class MinidumpThreadExList:
	def __init__(self):
		self.threads = []

	@staticmethod
	def parse(dir, buff):
		t = MinidumpThreadExList()
		buff.seek(dir.Location.Rva)
		chunk = io.BytesIO(buff.read(dir.Location.DataSize))
		mtl = MINIDUMP_THREAD_EX_LIST.parse(chunk)
		t.threads = mtl.Threads
		return t

	@staticmethod
	async def aparse(dir, buff):
		t = MinidumpThreadExList()
		await buff.seek(dir.Location.Rva)
		chunk_data = await buff.read(dir.Location.DataSize)
		chunk = io.BytesIO(chunk_data)
		mtl = MINIDUMP_THREAD_EX_LIST.parse(chunk)
		t.threads = mtl.Threads
		return t

	def to_table(self):
		t = []
		t.append(MINIDUMP_THREAD_EX.get_header())
		for thread in self.threads:
			t.append(thread.to_row())
		return t

	def __str__(self):
		return '== ThreadExList ==\n' + construct_table(self.to_table())
