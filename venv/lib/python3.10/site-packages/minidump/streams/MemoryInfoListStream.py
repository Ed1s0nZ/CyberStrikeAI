#!/usr/bin/env python3
#
# Author:
#  Tamas Jos (@skelsec)
#
import io
import enum
from minidump.common_structs import *

class AllocationProtect(enum.Enum):
	NONE = 0
	PAGE_EXECUTE = 0x10 #Enables execute access to the committed region of pages. An attempt to write to the committed region results in an access violation.
						#This flag is not supported by the CreateFileMapping function.

	PAGE_EXECUTE_READ = 0x20 #Enables execute or read-only access to the committed region of pages. An attempt to write to the committed region results in an access violation.
							 #Windows Server 2003 and Windows XP:  This attribute is not supported by the CreateFileMapping function until Windows XP with SP2 and Windows Server 2003 with SP1.

	PAGE_EXECUTE_READWRITE = 0x40 #Enables execute, read-only, or read/write access to the committed region of pages.#Windows Server 2003 and Windows XP:  This attribute is not supported by the CreateFileMapping function until Windows XP with SP2 and Windows Server 2003 with SP1.
	PAGE_EXECUTE_WRITECOPY = 0x80 #Enables execute, read-only, or copy-on-write access to a mapped view of a file mapping object. An attempt to write to a committed copy-on-write page results in a private copy of the page being made for the process. The private page is marked as PAGE_EXECUTE_READWRITE, and the change is written to the new page.
	#This flag is not supported by the VirtualAlloc or VirtualAllocEx functions.
	#Windows Vista, Windows Server 2003 and Windows XP:  This attribute is not supported by the CreateFileMapping function until Windows Vista with SP1 and Windows Server 2008.

	PAGE_NOACCESS = 0x01 #Disables all access to the committed region of pages. An attempt to read from, write to, or execute the committed region results in an access violation.
	#This flag is not supported by the CreateFileMapping function.

	PAGE_READONLY = 0x02 #Enables read-only access to the committed region of pages. An attempt to write to the committed region results in an access violation. If Data Execution Prevention is enabled, an attempt to execute code in the committed region results in an access violation.
	PAGE_READWRITE = 0x04 #Enables read-only or read/write access to the committed region of pages. If Data Execution Prevention is enabled, attempting to execute code in the committed region results in an access violation.
	PAGE_WRITECOPY = 0x08 #Enables read-only or copy-on-write access to a mapped view of a file mapping object. An attempt to write to a committed copy-on-write page results in a private copy of the page being made for the process. The private page is marked as PAGE_READWRITE, and the change is written to the new page. If Data Execution Prevention is enabled, attempting to execute code in the committed region results in an access violation.
							#This flag is not supported by the VirtualAlloc or VirtualAllocEx functions.

	PAGE_TARGETS_INVALID = 0x40000000
	#Sets all locations in the pages as invalid targets for CFG. Used along with any execute page protection like PAGE_EXECUTE, PAGE_EXECUTE_READ, PAGE_EXECUTE_READWRITE and PAGE_EXECUTE_WRITECOPY. Any indirect call to locations in those pages will fail CFG checks and the process will be terminated. The default behavior for executable pages allocated is to be marked valid call targets for CFG.
	#This flag is not supported by the VirtualProtect or CreateFileMapping functions.

	PAGE_TARGETS_NO_UPDATE = 0x40000000 #Pages in the region will not have their CFG information updated while the protection changes for VirtualProtect. For example, if the pages in the region was allocated using PAGE_TARGETS_INVALID, then the invalid information will be maintained while the page protection changes. This flag is only valid when the protection changes to an executable type like PAGE_EXECUTE, PAGE_EXECUTE_READ, PAGE_EXECUTE_READWRITE and PAGE_EXECUTE_WRITECOPY. The default behavior for VirtualProtect protection change to executable is to mark all locations as valid call targets for CFG.
	#The following are modifiers that can be used in addition to the options provided in the previous table, except as noted.
	#Constant/value	Description

	PAGE_GUARD = 0x100 #Pages in the region become guard pages. Any attempt to access a guard page causes the system to raise a STATUS_GUARD_PAGE_VIOLATION exception and turn off the guard page status. Guard pages thus act as a one-time access alarm. For more information, see Creating Guard Pages.
	#When an access attempt leads the system to turn off guard page status, the underlying page protection takes over.
	#If a guard page exception occurs during a system service, the service typically returns a failure status indicator.
	#This value cannot be used with PAGE_NOACCESS.
	#This flag is not supported by the CreateFileMapping function.

	PAGE_NOCACHE = 0x200
	#Sets all pages to be non-cachable. Applications should not use this attribute except when explicitly required for a device. Using the interlocked functions with memory that is mapped with SEC_NOCACHE can result in an EXCEPTION_ILLEGAL_INSTRUCTION exception.
	#The PAGE_NOCACHE flag cannot be used with the PAGE_GUARD, PAGE_NOACCESS, or PAGE_WRITECOMBINE flags.
	#The PAGE_NOCACHE flag can be used only when allocating private memory with the VirtualAlloc, VirtualAllocEx, or VirtualAllocExNuma functions. To enable non-cached memory access for shared memory, specify the SEC_NOCACHE flag when calling the CreateFileMapping function.
	PAGE_WRITECOMBINE = 0x400 #Sets all pages to be write-combined.
	#Applications should not use this attribute except when explicitly required for a device. Using the interlocked functions with memory that is mapped as write-combined can result in an EXCEPTION_ILLEGAL_INSTRUCTION exception.
	#The PAGE_WRITECOMBINE flag cannot be specified with the PAGE_NOACCESS, PAGE_GUARD, and PAGE_NOCACHE flags.
	#The PAGE_WRITECOMBINE flag can be used only when allocating private memory with the VirtualAlloc, VirtualAllocEx, or VirtualAllocExNuma functions. To enable write-combined memory access for shared memory, specify the SEC_WRITECOMBINE flag when calling the CreateFileMapping function.
	#Windows Server 2003 and Windows XP:  This flag is not supported until Windows Server 2003 with SP1.

class MemoryType(enum.Enum):
	MEM_IMAGE = 0x1000000 #Indicates that the memory pages within the region are mapped into the view of an image section.
	MEM_MAPPED = 0x40000 #Indicates that the memory pages within the region are mapped into the view of a section.
	MEM_PRIVATE = 0x20000 #Indicates that the memory pages within the region are private (that is, not shared by other processes).
class MemoryState(enum.Enum):
	MEM_COMMIT = 0x1000 #Indicates committed pages for which physical storage has been allocated, either in memory or in the paging file on disk.
	MEM_FREE = 0x10000 #Indicates free pages not accessible to the calling process and available to be allocated. For free pages, the information in the AllocationBase, AllocationProtect, Protect, and Type members is undefined.
	MEM_RESERVE = 0x2000 #Indicates reserved pages where a range of the process's virtual address space is reserved without any physical storage being allocated. For reserved pages, the information in the Protect member is undefined.


# https://msdn.microsoft.com/en-us/library/windows/desktop/ms680385(v=vs.85).aspx
class MINIDUMP_MEMORY_INFO_LIST:
	def __init__(self):
		self.SizeOfHeader:int = 16
		self.SizeOfEntry:int = 48
		self.NumberOfEntries:int = None
		self.entries = []

	def get_size(self):
		return self.SizeOfHeader + len(self.entries)*MINIDUMP_MEMORY_INFO().get_size()

	def to_bytes(self):
		t  = self.SizeOfHeader.to_bytes(4, byteorder = 'little', signed = False)
		t += self.SizeOfEntry.to_bytes(4, byteorder = 'little', signed = False)
		t += len(self.entries).to_bytes(8, byteorder = 'little', signed = False)
		return t

	@staticmethod
	def parse(buff):
		mhds = MINIDUMP_MEMORY_INFO_LIST()
		mhds.SizeOfHeader = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		mhds.SizeOfEntry = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		mhds.NumberOfEntries = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)

		return mhds

# https://msdn.microsoft.com/en-us/library/windows/desktop/ms680386(v=vs.85).aspx
class MINIDUMP_MEMORY_INFO:
	def __init__(self):
		self.BaseAddress = None
		self.AllocationBase = None
		self.AllocationProtect = None
		self.__alignment1 = 0
		self.RegionSize = None
		self.State = None
		self.Protect = None
		self.Type = None
		self.__alignment2 = 0

	def get_size(self):
		return 8+8+4+4+8+4+4+4+4

	def __str__(self):
		t = ''
		for k in self.__dict__:
			t += '%s : %s\r\n' % (k, str(self.__dict__[k]))
		return t

	def to_bytes(self):
		t = self.BaseAddress.to_bytes(8, byteorder = 'little', signed = False)
		t += self.AllocationBase.to_bytes(8, byteorder = 'little', signed = False)
		t += self.AllocationProtect.to_bytes(4, byteorder = 'little', signed = False)
		t += self.__alignment1.to_bytes(4, byteorder = 'little', signed = False)
		t += self.RegionSize.to_bytes(8, byteorder = 'little', signed = False)
		t += self.State.value.to_bytes(4, byteorder = 'little', signed = False)
		t += self.Protect.value.to_bytes(4, byteorder = 'little', signed = False)
		t += self.Type.value.to_bytes(4, byteorder = 'little', signed = False)
		t += self.__alignment2.to_bytes(4, byteorder = 'little', signed = False)
		return t

	@staticmethod
	def parse(buff):
		mmi = MINIDUMP_MEMORY_INFO()
		mmi.BaseAddress = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)
		mmi.AllocationBase = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)
		mmi.AllocationProtect = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		mmi.__alignment1 = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		mmi.RegionSize = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)
		try:
			mmi.State = MemoryState(int.from_bytes(buff.read(4), byteorder = 'little', signed = False))
		except:
			pass
		try:
			mmi.Protect = AllocationProtect(int.from_bytes(buff.read(4), byteorder = 'little', signed = False))
		except:
			pass
		try:
			mmi.Type = MemoryType(int.from_bytes(buff.read(4), byteorder = 'little', signed = False))
		except:
			pass
		mmi.__alignment2 = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)

		return mmi

class MinidumpMemoryInfo:
	def __init__(self):
		self.BaseAddress = None
		self.AllocationBase = None
		self.AllocationProtect = None
		self.RegionSize = None
		self.State = None
		self.Protect = None
		self.Type = None

	@staticmethod
	def parse(t, buff):
		mmi = MinidumpMemoryInfo()
		mmi.BaseAddress = t.BaseAddress
		mmi.AllocationBase = t.AllocationBase
		mmi.AllocationProtect = t.AllocationProtect
		mmi.RegionSize = t.RegionSize
		mmi.State = t.State
		mmi.Protect = t.Protect
		mmi.Type = t.Type
		return mmi

	@staticmethod
	def get_header():
		t = [
			'BaseAddress',
			'AllocationBase',
			'AllocationProtect',
			'RegionSize',
			'State',
			'Protect',
			'Type',
		]
		return t

	def to_row(self):
		t = [
			hex(self.BaseAddress),
			hex(self.AllocationBase),
			str(self.AllocationProtect),
			hex(self.RegionSize),
			self.State.name if self.State else 'N/A',
			self.Protect.name if self.Protect else 'N/A',
			self.Type.name if self.Type else 'N/A',
		]
		return t


class MinidumpMemoryInfoList:
	def __init__(self):
		self.header = None
		self.infos = []

	@staticmethod
	def parse(dir, buff):
		t = MinidumpMemoryInfoList()
		buff.seek(dir.Location.Rva)
		data = buff.read(dir.Location.DataSize)
		chunk = io.BytesIO(data)
		t.header = MINIDUMP_MEMORY_INFO_LIST.parse(chunk)
		for _ in range(t.header.NumberOfEntries):
			mi = MINIDUMP_MEMORY_INFO.parse(chunk)
			t.infos.append(MinidumpMemoryInfo.parse(mi, buff))

		return t

	@staticmethod
	async def aparse(dir, buff):
		t = MinidumpMemoryInfoList()
		await buff.seek(dir.Location.Rva)
		data = await buff.read(dir.Location.DataSize)
		chunk = io.BytesIO(data)
		t.header = MINIDUMP_MEMORY_INFO_LIST.parse(chunk)
		for _ in range(t.header.NumberOfEntries):
			mi = MINIDUMP_MEMORY_INFO.parse(chunk)
			t.infos.append(MinidumpMemoryInfo.parse(mi, None))

		return t

	def to_table(self):
		t = []
		t.append(MinidumpMemoryInfo.get_header())
		for info in self.infos:
			t.append(info.to_row())
		return t

	def __str__(self):
		return '== MinidumpMemoryInfoList ==\n' + construct_table(self.to_table())