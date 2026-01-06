#!/usr/bin/env python3
#
# Author:
#  Tamas Jos (@skelsec)
#

import io
import enum
import logging
from minidump.common_structs import *

MINIDUMP_WIN_BUILDNO_TO_VERSION = {
	103: 'Windows 3.1',
	102: 'Windows 3.1 - Sparta',
	528: 'Windiws NT 3.1',
	300: 'Windows NT 3.11 - Snowball',
	153: 'Windows 3.2',
	807: 'Windows NT 3.5',
	1057: 'Windows NT 3.51',
	1381: 'Windows NT 4.0',
	950: 'Windows 95',
	1998 : 'Windows 98',
	2222: 'Windows 98 SE',
	2195: 'Windows 2000',
	3000: 'Windows ME',
	2600: 'Windows XP',
	2700: 'Windows XP - Media Center Edition 2005',
	2710: 'Windows XP - Media Center Edition 2005 Update Rollup 2',
	3790: 'Windows XP x64 / Server 2003 / Server 2003 R2',
	6002: 'Windows Vista / Server 2008',
	7601: 'Windows 7 / Server 2008 R2',
	9200: 'Windows 8 / Server 2012',
	9600: 'Windows 8.1 / Server 2012 R2',
	10240: 'Windows 10 - 1507',
	10586: 'Windows 10 - 1511',
	14393: 'Windows 10 - 1607 / Server 2016',
	15063: 'Windows 10 - 1703',
	16299: 'Windows 10 - 1709 / Server 2016',
	17134: 'Windows 10 - 1803 / Server 2016',
	17763: 'Windows 10 - 1809 / Server 2019',
	18362: 'Windows 10 - 1903 / Server 2019',
	18363: 'Windows 10 - 1909 / Server 2019',
	19041: 'Windows 10 - 2004 / Server 2019',
	19042: 'Windows 10 - 20H2 / Server 2019',
	19043: 'Windows 10 - 21H1',
	19044: 'Windows 10 - 21H2',
	19045: 'Windows 10 - 22H2',
	22000: 'Windows 11 - 21H2',
	20348: 'Windows Server 2022',
	22621: 'Windows 11 - 22H2',
}


# https://msdn.microsoft.com/en-us/library/windows/desktop/ms680396(v=vs.85).aspx
class PROCESSOR_ARCHITECTURE(enum.Enum):
	AMD64 = 9 #x64 (AMD or Intel)
	ARM = 5 #ARM
	IA64 = 6 #Intel Itanium
	INTEL = 0 #x86
	AARCH64 = 0x8003 #ARM64
	UNKNOWN = 0xffff #Unknown processor
# https://msdn.microsoft.com/en-us/library/windows/desktop/ms680396(v=vs.85).aspx
class PROCESSOR_LEVEL(enum.Enum):
	INTEL_80386 = 3
	INTEL_80486 = 4
	INTEL_PENTIUM = 5
	INTEL_PENTIUM_PRO = 6 #or Pentium II
# https://msdn.microsoft.com/en-us/library/windows/desktop/ms680396(v=vs.85).aspx
class PRODUCT_TYPE(enum.Enum):
	VER_UNIDENTIFIED_PRODUCT 	= 0x0000000 # Crashpad des not set ProductType value on non-Windows systems
	VER_NT_WORKSTATION 			= 0x0000001 # The system is running Windows XP, Windows Vista, Windows 7, or Windows 8.
	VER_NT_DOMAIN_CONTROLLER 	= 0x0000002 # The system is a domain controller.
	VER_NT_SERVER 				= 0x0000003 # The system is a server.

# https://msdn.microsoft.com/en-us/library/windows/desktop/ms680396(v=vs.85).aspx
class PLATFORM_ID(enum.Enum):
	VER_PLATFORM_WIN32s = 0 #Not supported
	VER_PLATFORM_WIN32_WINDOWS = 1 #Not supported.
	VER_PLATFORM_WIN32_NT = 2 #The operating system platform is Windows.

	# source : https://github.com/chromium/crashpad/blob/4b05be4265c0ffacfce26d7db7644ffbf9037696/minidump/minidump_extensions.h#L239
	VER_PLATFORM_CRASHPAD_MAC       = 0x8101
	VER_PLATFORM_CRASHPAD_IOS       = 0x8102
	VER_PLATFORM_CRASHPAD_LINUX 	= 0x8201
	VER_PLATFORM_CRASHPAD_SOLARIS 	= 0x8202
	VER_PLATFORM_CRASHPAD_ANDROID 	= 0x8203
	VER_PLATFORM_CRASHPAD_PS3       = 0x8204
	VER_PLATFORM_CRASHPAD_NACL      = 0x8205
	VER_PLATFORM_CRASHPAD_FUSCHIA 	= 0x8206
	VER_PLATFORM_CRASHPAD_UNKNOWN 	= 0xffffffff

# https://msdn.microsoft.com/en-us/library/windows/desktop/ms680396(v=vs.85).aspx
class SUITE_MASK(enum.IntFlag):
	VER_SUITE_BACKOFFICE = 0x00000004 #Microsoft BackOffice components are installed.
	VER_SUITE_BLADE = 0x00000400 #Windows Server 2003, Web Edition is installed.
	VER_SUITE_COMPUTE_SERVER = 0x00004000 #Windows Server 2003, Compute Cluster Edition is installed.
	VER_SUITE_DATACENTER = 0x00000080 #Windows Server 2008 R2 Datacenter, Windows Server 2008 Datacenter, or Windows Server 2003, Datacenter Edition is installed.
	VER_SUITE_ENTERPRISE = 0x00000002 #Windows Server 2008 R2 Enterprise, Windows Server 2008 Enterprise, or Windows Server 2003, Enterprise Edition is installed.
	VER_SUITE_EMBEDDEDNT = 0x00000040 #Windows Embedded is installed.
	VER_SUITE_PERSONAL = 0x00000200 #Windows XP Home Edition is installed.
	VER_SUITE_SINGLEUSERTS = 0x00000100 #Remote Desktop is supported, but only one interactive session is supported. This value is set unless the system is running in application server mode.
	VER_SUITE_SMALLBUSINESS = 0x00000001 #Microsoft Small Business Server was once installed on the system, but may have been upgraded to another version of Windows.
	VER_SUITE_SMALLBUSINESS_RESTRICTED = 0x00000020 #Microsoft Small Business Server is installed with the restrictive client license in force.
	VER_SUITE_STORAGE_SERVER = 0x00002000 #Windows Storage Server is installed.
	VER_SUITE_TERMINAL = 0x00000010 # Terminal Services is installed. This value is always set. If VER_SUITE_TERMINAL is set but VER_SUITE_SINGLEUSERTS is not set, the system is running in application server mode.


# https://msdn.microsoft.com/en-us/library/windows/desktop/ms680396(v=vs.85).aspx
class MINIDUMP_SYSTEM_INFO:
	def __init__(self):
		self.ProcessorArchitecture = None
		self.ProcessorLevel = None
		self.ProcessorRevision = None
		self.Reserved0 = None
		self.NumberOfProcessors = None
		self.ProductType = None
		self.MajorVersion = None
		self.MinorVersion = None
		self.BuildNumber = None
		self.PlatformId = None
		self.CSDVersionRva = None
		self.Reserved1 = 0
		self.SuiteMask = None
		self.Reserved2 = 0
		self.VendorId = []
		self.VersionInformation = None
		self.FeatureInformation = None
		self.AMDExtendedCpuFeatures = None
		self.ProcessorFeatures = []

		#for wrtier
		self.CSDVersion = None

	def get_size(self):
		# here we cannot tell upfront what the size will be :(
		return len(self.to_bytes())

	def to_bytes(self, data_buffer = None):
		t = self.ProcessorArchitecture.value.to_bytes(2, byteorder = 'little', signed = False)
		t += self.ProcessorLevel.to_bytes(2, byteorder = 'little', signed = False)
		t += self.ProcessorRevision.to_bytes(2, byteorder = 'little', signed = False)
		#missing filed here?
		t += self.NumberOfProcessors.to_bytes(1, byteorder = 'little', signed = False)
		t += self.ProductType.value.to_bytes(1, byteorder = 'little', signed = False)
		t += self.MajorVersion.to_bytes(4, byteorder = 'little', signed = False)
		t += self.MinorVersion.to_bytes(4, byteorder = 'little', signed = False)
		t += self.BuildNumber.to_bytes(4, byteorder = 'little', signed = False)
		t += self.PlatformId.to_bytes(4, byteorder = 'little', signed = False)
		if data_buffer is None:
			t += self.CSDVersionRva.to_bytes(4, byteorder = 'little', signed = False)
		else:
			pos = data_buffer.tell()
			data_buffer.write(100*b'\x00')
			self.CSDVersionRva = data_buffer.tell()
			data_buffer.write(self.CSDVersion.encode('ascii') + b'\x00')
			pos_end = data_buffer.tell()
			data_buffer.seek(pos,0)
			t += self.CSDVersionRva.to_bytes(4, byteorder = 'little', signed = False)
		#missing filed here?
		t += self.SuiteMask.to_bytes(2, byteorder = 'little', signed = False)
		t += self.Reserved2.to_bytes(2, byteorder = 'little', signed = False)
		if self.ProcessorArchitecture == PROCESSOR_ARCHITECTURE.INTEL:
			for vid in self.VendorId:
				t += vid.to_bytes(4, byteorder = 'little', signed = False)
			t += self.VersionInformation.value.to_bytes(4, byteorder = 'little', signed = False)
			t += self.FeatureInformation.value.to_bytes(4, byteorder = 'little', signed = False)
			t += self.AMDExtendedCpuFeatures.value.to_bytes(4, byteorder = 'little', signed = False)
		else:
			for pf in self.ProcessorFeatures:
				t += pf.to_bytes(8, byteorder = 'little', signed = False)

		if data_buffer is None:
			return t
		else:
			data_buffer.write(t)

	@staticmethod
	def parse(buff):
		msi = MINIDUMP_SYSTEM_INFO()
		msi.ProcessorArchitecture = PROCESSOR_ARCHITECTURE(int.from_bytes(buff.read(2), byteorder = 'little', signed = False))
		msi.ProcessorLevel = int.from_bytes(buff.read(2), byteorder = 'little', signed = False)
		msi.ProcessorRevision = int.from_bytes(buff.read(2), byteorder = 'little', signed = False)
		#the below field is present in the documentation from MSDN, however is not present in the actual dump
		#msi.Reserved0 = int.from_bytes(buff.read(2), byteorder = 'little', signed = False)
		msi.NumberOfProcessors = int.from_bytes(buff.read(1), byteorder = 'little', signed = False)
		msi.ProductType = PRODUCT_TYPE(int.from_bytes(buff.read(1), byteorder = 'little', signed = False))
		msi.MajorVersion = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		msi.MinorVersion = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		msi.BuildNumber = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		msi.PlatformId = PLATFORM_ID(int.from_bytes(buff.read(4), byteorder = 'little', signed = False))
		msi.CSDVersionRva = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		#msi.Reserved1 = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		msi.SuiteMask = SUITE_MASK(int.from_bytes(buff.read(2), byteorder = 'little', signed = False))
		msi.Reserved2 = int.from_bytes(buff.read(2), byteorder = 'little', signed = False)
		if msi.ProcessorArchitecture == PROCESSOR_ARCHITECTURE.INTEL:
			for _ in range(3):
				msi.VendorId.append(int.from_bytes(buff.read(4), byteorder = 'little', signed = False))
			msi.VersionInformation = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
			msi.FeatureInformation = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
			msi.AMDExtendedCpuFeatures = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		else:
			for _ in range(2):
				msi.ProcessorFeatures.append(int.from_bytes(buff.read(8), byteorder = 'little', signed = False))

		return msi

	def __str__(self):
		t = ''
		for k in self.__dict__:
			t += '%s : %s\r\n' % (k, str(self.__dict__[k]))
		return t

class MinidumpSystemInfo:
	def __init__(self):
		self.ProcessorArchitecture = None
		self.ProcessorLevel = None
		self.ProcessorRevision = None
		self.NumberOfProcessors = None
		self.ProductType = None
		self.MajorVersion = None
		self.MinorVersion = None
		self.BuildNumber = None
		self.PlatformId = None
		self.CSDVersion = None
		self.SuiteMask = None
		self.VendorId = None
		self.VersionInformation = None
		self.FeatureInformation = None
		self.AMDExtendedCpuFeatures = None
		self.ProcessorFeatures = None

		#extra
		self.OperatingSystem = None

	def guess_os(self):
		if self.BuildNumber in MINIDUMP_WIN_BUILDNO_TO_VERSION:
			self.OperatingSystem = MINIDUMP_WIN_BUILDNO_TO_VERSION[self.BuildNumber]
		elif self.MajorVersion == 10 and self.MinorVersion == 0 and self.ProductType == PRODUCT_TYPE.VER_NT_WORKSTATION:
			self.OperatingSystem = "Windows 10"
		elif self.MajorVersion == 10 and self.MinorVersion == 0 and self.ProductType != self.ProductType.VER_NT_WORKSTATION:
			self.OperatingSystem =  "Windows Server 2016 Technical Preview"
		elif self.MajorVersion == 6 and self.MinorVersion == 3 and self.ProductType == self.ProductType.VER_NT_WORKSTATION:
			self.OperatingSystem =  "Windows 8.1"
		elif self.MajorVersion == 6 and self.MinorVersion == 3 and self.ProductType != self.ProductType.VER_NT_WORKSTATION:
			self.OperatingSystem =  "Windows Server 2012 R2"
		elif self.MajorVersion == 6 and self.MinorVersion == 2 and self.ProductType == self.ProductType.VER_NT_WORKSTATION:
			self.OperatingSystem =  "Windows 8"
		elif self.MajorVersion == 6 and self.MinorVersion == 2 and self.ProductType != self.ProductType.VER_NT_WORKSTATION:
			self.OperatingSystem =  "Windows Server 2012"
		elif self.MajorVersion == 6 and self.MinorVersion == 1 and self.ProductType == self.ProductType.VER_NT_WORKSTATION:
			self.OperatingSystem =  "Windows 7"
		elif self.MajorVersion == 6 and self.MinorVersion == 1 and self.ProductType != self.ProductType.VER_NT_WORKSTATION:
			self.OperatingSystem =  "Windows Server 2008 R2"
		elif self.MajorVersion == 6 and self.MinorVersion == 0 and self.ProductType == self.ProductType.VER_NT_WORKSTATION:
			self.OperatingSystem =  "Windows Vista"
		elif self.MajorVersion == 6 and self.MinorVersion == 0 and self.ProductType != self.ProductType.VER_NT_WORKSTATION:
			self.OperatingSystem =  "Windows Server 2008"
		# Can't accurately report on Windows Server 2003/R2
		# elif (MajorVersion == 5 and MinorVersion == 2 and ProductType == self.ProductType.VER_NT_WORKSTATION)
		#	self.OperatingSystem =  "Windows Vista"
		#elif (MajorVersion == 5 and MinorVersion == 2 and ProductType != self.ProductType.VER_NT_WORKSTATION)
		#	self.OperatingSystem =  "Windows Server 2008"
		elif self.MajorVersion == 5 and self.MinorVersion == 1:
			self.OperatingSystem =  "Windows XP"
		elif self.MajorVersion == 5 and self.MinorVersion == 0:
			self.OperatingSystem =  "Windows 2000"

	@staticmethod
	def parse(dir, buff):
		t = MinidumpSystemInfo()
		buff.seek(dir.Location.Rva)
		chunk = io.BytesIO(buff.read(dir.Location.DataSize))
		si = MINIDUMP_SYSTEM_INFO.parse(chunk)
		t.ProcessorArchitecture = si.ProcessorArchitecture
		t.ProcessorLevel = si.ProcessorLevel
		t.ProcessorRevision = si.ProcessorRevision
		t.NumberOfProcessors = si.NumberOfProcessors
		t.ProductType = si.ProductType
		t.MajorVersion = si.MajorVersion
		t.MinorVersion = si.MinorVersion
		t.BuildNumber = si.BuildNumber
		t.PlatformId = si.PlatformId
		t.CSDVersion = MINIDUMP_STRING.get_from_rva(si.CSDVersionRva, buff)
		t.SuiteMask = si.SuiteMask
		t.VendorId = si.VendorId
		t.VersionInformation = si.VersionInformation
		t.FeatureInformation = si.FeatureInformation
		t.AMDExtendedCpuFeatures = si.AMDExtendedCpuFeatures
		t.ProcessorFeatures = si.ProcessorFeatures
		try:
			t.guess_os()
		except Exception as e:
			logging.log(1, 'Failed to guess OS! MajorVersion: %s MinorVersion %s BuildNumber %s ProductType: %s' % (t.MajorVersion, t.MinorVersion, t.BuildNumber, t.ProductType ))
			t.OperatingSystem = None
		return t

	@staticmethod
	async def aparse(dir, buff):
		t = MinidumpSystemInfo()
		await buff.seek(dir.Location.Rva)
		chunk_data = await buff.read(dir.Location.DataSize)
		chunk = io.BytesIO(chunk_data)
		si = MINIDUMP_SYSTEM_INFO.parse(chunk)
		t.ProcessorArchitecture = si.ProcessorArchitecture
		t.ProcessorLevel = si.ProcessorLevel
		t.ProcessorRevision = si.ProcessorRevision
		t.NumberOfProcessors = si.NumberOfProcessors
		t.ProductType = si.ProductType
		t.MajorVersion = si.MajorVersion
		t.MinorVersion = si.MinorVersion
		t.BuildNumber = si.BuildNumber
		t.PlatformId = si.PlatformId
		t.CSDVersion = await MINIDUMP_STRING.aget_from_rva(si.CSDVersionRva, buff)
		t.SuiteMask = si.SuiteMask
		t.VendorId = si.VendorId
		t.VersionInformation = si.VersionInformation
		t.FeatureInformation = si.FeatureInformation
		t.AMDExtendedCpuFeatures = si.AMDExtendedCpuFeatures
		t.ProcessorFeatures = si.ProcessorFeatures
		try:
			t.guess_os()
		except Exception as e:
			logging.log(1, 'Failed to guess OS! MajorVersion: %s MinorVersion %s BuildNumber %s ProductType: %s' % (t.MajorVersion, t.MinorVersion, t.BuildNumber, t.ProductType ))
			t.OperatingSystem = None
		return t


	def __str__(self):
		t = '== System Info ==\n'
		t += 'ProcessorArchitecture %s\n' % self.ProcessorArchitecture
		t += 'OperatingSystem -guess- %s\n' % self.OperatingSystem
		t += 'ProcessorLevel %s\n' % self.ProcessorLevel
		t += 'ProcessorRevision %s\n' % hex(self.ProcessorRevision)
		t += 'NumberOfProcessors %s\n' % self.NumberOfProcessors
		t += 'ProductType %s\n' % self.ProductType
		t += 'MajorVersion %s\n' % self.MajorVersion
		t += 'MinorVersion %s\n' % self.MinorVersion
		t += 'BuildNumber %s\n' % self.BuildNumber
		t += 'PlatformId %s\n' % self.PlatformId
		t += 'CSDVersion: %s\n' % self.CSDVersion
		t += 'SuiteMask %s\n' % self.SuiteMask
		t += 'VendorId %s\n' % ' '.join( [hex(x) for x in self.VendorId] )
		t += 'VersionInformation %s\n' % self.VersionInformation
		t += 'FeatureInformation %s\n' % self.FeatureInformation
		t += 'AMDExtendedCpuFeatures %s\n' % self.AMDExtendedCpuFeatures
		t += 'ProcessorFeatures %s\n' % ' '.join( [hex(x) for x in self.ProcessorFeatures] )

		return t
