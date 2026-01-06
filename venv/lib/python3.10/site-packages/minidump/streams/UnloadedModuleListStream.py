#!/usr/bin/env python3
#
# Author:
#  Tamas Jos (@skelsec)
#

import io
from minidump.common_structs import *

# https://msdn.microsoft.com/en-us/library/windows/desktop/ms680521(v=vs.85).aspx
class MINIDUMP_UNLOADED_MODULE_LIST:
	def __init__(self):
		self.SizeOfHeader = None
		self.SizeOfEntry = None
		self.NumberOfEntries = None

	def to_bytes(self):
		t  = self.SizeOfHeader.value.to_bytes(4, byteorder = 'little', signed = False)
		t += self.SizeOfEntry.to_bytes(4, byteorder = 'little', signed = False)
		t += self.NumberOfEntries.to_bytes(4, byteorder = 'little', signed = False)
		return t

	@staticmethod
	def parse(buff):
		muml = MINIDUMP_UNLOADED_MODULE_LIST()
		muml.SizeOfHeader = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		muml.SizeOfEntry = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		muml.NumberOfEntries = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		return muml

# https://msdn.microsoft.com/en-us/library/windows/desktop/ms680523(v=vs.85).aspx
class MINIDUMP_UNLOADED_MODULE:
	def __init__(self):
		self.BaseOfImage = None
		self.SizeOfImage = None
		self.CheckSum = None
		self.TimeDateStamp = None
		self.ModuleNameRva = None

	def to_bytes(self):
		t  = self.BaseOfImage.value.to_bytes(8, byteorder = 'little', signed = False)
		t += self.SizeOfImage.to_bytes(4, byteorder = 'little', signed = False)
		t += self.CheckSum.to_bytes(4, byteorder = 'little', signed = False)
		t += self.TimeDateStamp.to_bytes(4, byteorder = 'little', signed = False)
		t += self.ModuleNameRva.to_bytes(4, byteorder = 'little', signed = False)
		return t

	@staticmethod
	def parse(buff):
		mum = MINIDUMP_UNLOADED_MODULE()
		mum.BaseOfImage = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)
		mum.SizeOfImage = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		mum.CheckSum = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		mum.TimeDateStamp = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		mum.ModuleNameRva = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		return mum

class MinidumpUnloadedModule:
	def __init__(self):
		self.name = None
		self.baseaddress = None
		self.size = None
		self.endaddress = None
		self.memorysegments = [] #list of memory segments the module takes place in

		self.checksum = None
		self.timestamp = None

	@staticmethod
	def parse(mod, buff):
		"""
		mod: MINIDUMP_MODULE
		buff: file handle
		"""
		mm = MinidumpUnloadedModule()
		mm.baseaddress = mod.BaseOfImage
		mm.size = mod.SizeOfImage
		mm.checksum = mod.CheckSum
		mm.timestamp = mod.TimeDateStamp
		mm.name = MINIDUMP_STRING.get_from_rva(mod.ModuleNameRva, buff)
		mm.endaddress = mm.baseaddress + mm.size
		return mm

	@staticmethod
	async def aparse(mod, buff):
		"""
		mod: MINIDUMP_MODULE
		buff: file handle
		"""
		mm = MinidumpUnloadedModule()
		mm.baseaddress = mod.BaseOfImage
		mm.size = mod.SizeOfImage
		mm.checksum = mod.CheckSum
		mm.timestamp = mod.TimeDateStamp
		mm.name = await MINIDUMP_STRING.aget_from_rva(mod.ModuleNameRva, buff)
		mm.endaddress = mm.baseaddress + mm.size
		return mm

	def assign_memory_regions(self, segments):
		for segment in segments:
			if self.baseaddress <= segment.start_virtual_address < self.endaddress:
				self.memorysegments.append(segment)

	def __str__(self):
		return 'Unloaded Module name: %s Size: %s BaseAddress: %s' % (self.name, hex(self.size), hex(self.baseaddress))

	@staticmethod
	def get_header():
		return [
			'Module name',
			'BaseAddress',
			'Size',
			'Endaddress',
		]

	def to_row(self):
		return [
			str(self.name),
			'0x%08x' % self.baseaddress,
			hex(self.size),
			'0x%08x' % self.endaddress,
		]


class MinidumpUnloadedModuleList:
	def __init__(self):
		self.modules = []

	@staticmethod
	def parse(dir, buff):
		t = MinidumpUnloadedModuleList()
		buff.seek(dir.Location.Rva)
		chunk = io.BytesIO(buff.read(dir.Location.DataSize))
		muml = MINIDUMP_UNLOADED_MODULE_LIST.parse(chunk)
		for _ in range(muml.NumberOfEntries):
			mod = MINIDUMP_UNLOADED_MODULE.parse(chunk)
			t.modules.append(MinidumpUnloadedModule.parse(mod, buff))

		return t

	@staticmethod
	async def aparse(dir, buff):
		t = MinidumpUnloadedModuleList()
		await buff.seek(dir.Location.Rva)
		chunk_data = await buff.read(dir.Location.DataSize)
		chunk = io.BytesIO(chunk_data)
		muml = MINIDUMP_UNLOADED_MODULE_LIST.parse(chunk)
		for _ in range(muml.NumberOfEntries):
			mod = MINIDUMP_UNLOADED_MODULE.parse(chunk)
			dr = await MinidumpUnloadedModule.aparse(mod, buff)
			t.modules.append(dr)

		return t

	def to_table(self):
		t = []
		t.append(MinidumpUnloadedModule.get_header())
		for mod in self.modules:
			t.append(mod.to_row())
		return t

	def __str__(self):
		t  = '== UnloadedModuleList ==\n' + construct_table(self.to_table())
		return t