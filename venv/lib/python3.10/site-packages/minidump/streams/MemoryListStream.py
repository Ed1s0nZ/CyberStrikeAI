#!/usr/bin/env python3
#
# Author:
#  Tamas Jos (@skelsec)
#
import io
from minidump.common_structs import *

# https://docs.microsoft.com/en-us/windows/win32/api/minidumpapiset/ns-minidumpapiset-minidump_memory_list
class MINIDUMP_MEMORY_LIST:
	def __init__(self):
		self.NumberOfMemoryRanges = None
		self.MemoryRanges = []

	def to_bytes(self):
		t = len(self.MemoryRanges).to_bytes(4, byteorder = 'little', signed = False)
		for memrange in self.MemoryRanges:
			t += memrange.to_bytes()
		return t

	@staticmethod
	def parse(buff):
		mml = MINIDUMP_MEMORY_LIST()
		mml.NumberOfMemoryRanges = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		for _ in range(mml.NumberOfMemoryRanges):
			mml.MemoryRanges.append(MINIDUMP_MEMORY_DESCRIPTOR.parse(buff))

		return mml

	def __str__(self):
		t  = '== MINIDUMP_MEMORY_LIST ==\n'
		t += 'NumberOfMemoryRanges: %s\n' % self.NumberOfMemoryRanges
		for range in self.MemoryRanges:
			t+= str(range)
		return t

# https://msdn.microsoft.com/en-us/library/windows/desktop/ms680384(v=vs.85).aspx
class MINIDUMP_MEMORY_DESCRIPTOR:
	def __init__(self):
		self.StartOfMemoryRange = None
		self.MemoryLocation = None

		#we do not use MemoryLocation but immediately store its fields in this object for easy access
		self.DataSize = None
		self.Rva = None

	def to_bytes(self):
		t = self.StartOfMemoryRange.to_bytes(4, byteorder = 'little', signed = False)
		t += self.MemoryLocation.to_bytes()
		return t

	@staticmethod
	def parse(buff):
		md = MINIDUMP_MEMORY_DESCRIPTOR()
		md.StartOfMemoryRange = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)

		#TODO: figure out what the documentation says, the person writign it was probably high...
		# The deal is: RVA sizes differ on where in the file the memory data is stored. but it's not possible to know it up front if we need to read 32 or 64 bytes...
		#
		#if md.StartOfMemoryRange < 0x100000000:
		#	md.MemoryLocation = MINIDUMP_LOCATION_DESCRIPTOR.parse(buff)
		#else:
		#	md.MemoryLocation = MINIDUMP_LOCATION_DESCRIPTOR64.parse(buff)

		md.MemoryLocation = MINIDUMP_LOCATION_DESCRIPTOR.parse(buff)
		md.DataSize = md.MemoryLocation.DataSize
		md.Rva = md.MemoryLocation.Rva
		return md

	def __str__(self):
		t =  'Start: %s' % hex(self.StartOfMemoryRange)
		t += 'Size: %s' % self.DataSize
		t += 'Rva: %s' % self.Rva
		return t

class MinidumpMemoryList:
	def __init__(self):
		self.memory_segments = []

	@staticmethod
	def parse(dir, buff):
		t = MinidumpMemoryList()
		buff.seek(dir.Location.Rva)
		chunk = io.BytesIO(buff.read(dir.Location.DataSize))
		mtl = MINIDUMP_MEMORY_LIST.parse(chunk)
		for mod in mtl.MemoryRanges:
			t.memory_segments.append(MinidumpMemorySegment.parse_mini(mod, buff))
		return t

	@staticmethod
	async def aparse(dir, buff):
		t = MinidumpMemoryList()
		await buff.seek(dir.Location.Rva)
		chunk_data = await buff.read(dir.Location.DataSize)
		chunk = io.BytesIO(chunk_data)
		mtl = MINIDUMP_MEMORY_LIST.parse(chunk)
		for mod in mtl.MemoryRanges:
			t.memory_segments.append(MinidumpMemorySegment.parse_mini(mod, buff))
		return t

	def __str__(self):
		t  = '== MinidumpMemoryList ==\n'
		for mod in self.memory_segments:
			t+= str(mod) + '\n'
		return t