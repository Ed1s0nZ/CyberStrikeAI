#!/usr/bin/env python3
#
# Author:
#  Tamas Jos (@skelsec)
#

import io
import sys
import enum
import struct
import logging

from minidump.header import MinidumpHeader
from minidump.aminidumpreader import AMinidumpFileReader
from minidump.streams import *
from minidump.common_structs import *
from minidump.constants import MINIDUMP_STREAM_TYPE
from minidump.directory import MINIDUMP_DIRECTORY


class AsyncFile:
	def __init__(self, filename):
		self.filename = filename
		self.fhandle = open(filename, 'rb')
	
	async def read(self, n = -1):
		return self.fhandle.read(n)

	async def seek(self, n, beg = 0):
		return self.fhandle.seek(n, beg)

	def tell(self):
		return self.fhandle.tell()

class AMinidumpFile:
	def __init__(self):
		self.filename = None
		self.file_handle = None
		self.header = None
		self.directories = []

		self.threads_ex = None
		self.threads = None
		self.modules = None
		self.memory_segments = None
		self.memory_segments_64 = None
		self.sysinfo = None
		self.comment_a = None
		self.comment_w = None
		self.exception = None
		self.handles = None
		self.unloaded_modules = None
		self.misc_info = None
		self.memory_info = None
		self.thread_info = None

	@staticmethod
	async def parse(filename):
		mf = AMinidumpFile()
		mf.filename = filename
		mf.file_handle = AsyncFile(filename)
		await mf._parse()
		return mf

	@staticmethod
	async def parse_external(file_handle, filename = ''):
		"""
		External file handle must be an object that exposes basic file IO functionality
		that you'd get by python's file buffer (read, seek, tell etc.)
		"""
		mf = AMinidumpFile()
		mf.filename = filename
		mf.file_handle = file_handle
		await mf._parse()
		return mf

	@staticmethod
	async def parse_bytes(data):
		return await AMinidumpFile.parse_buff(io.BytesIO(data))

	@staticmethod
	def parse_buff(buffer):
		mf = AMinidumpFile()
		mf.file_handle = buffer
		mf._parse()
		return mf

	def get_reader(self):
		return AMinidumpFileReader(self)

	async def _parse(self):
		await self.__parse_header()
		await self.__parse_directories()

	async def __parse_header(self):
		self.header = await MinidumpHeader.aparse(self.file_handle)
		for i in range(0, self.header.NumberOfStreams):
			await self.file_handle.seek(self.header.StreamDirectoryRva + i * 12, 0 )
			minidump_dir = await MINIDUMP_DIRECTORY.aparse(self.file_handle)
			
			if minidump_dir:
				self.directories.append(minidump_dir)
			else:
				await self.file_handle.seek(self.header.StreamDirectoryRva + i * 12, 0 )
				t = await self.file_handle.read(4)
				user_stream_type_value = int.from_bytes(t, byteorder = 'little', signed = False)
				logging.debug('Found Unknown UserStream directory Type: %x' % (user_stream_type_value))

	async def __parse_directories(self):

		for dir in self.directories:
			if dir.StreamType == MINIDUMP_STREAM_TYPE.UnusedStream:
				logging.debug('Found UnusedStream @%x Size: %d' % (dir.Location.Rva, dir.Location.DataSize))
				continue # Reserved. Do not use this enumeration value.
			elif dir.StreamType == MINIDUMP_STREAM_TYPE.ReservedStream0:
				logging.debug('Found ReservedStream0 @%x Size: %d' % (dir.Location.Rva, dir.Location.DataSize))
				continue # Reserved. Do not use this enumeration value.
			elif dir.StreamType == MINIDUMP_STREAM_TYPE.ReservedStream1:
				logging.debug('Found ReservedStream1 @%x Size: %d' % (dir.Location.Rva, dir.Location.DataSize))
				continue # Reserved. Do not use this enumeration value.
			elif dir.StreamType == MINIDUMP_STREAM_TYPE.ThreadListStream:
				logging.debug('Found ThreadListStream @%x Size: %d' % (dir.Location.Rva, dir.Location.DataSize))
				self.threads = await MinidumpThreadList.aparse(dir, self.file_handle)
				continue
			elif dir.StreamType == MINIDUMP_STREAM_TYPE.ModuleListStream:
				logging.debug('Found ModuleListStream @%x Size: %d' % (dir.Location.Rva, dir.Location.DataSize))
				self.modules = await MinidumpModuleList.aparse(dir, self.file_handle)
				#logging.debug(str(modules_list))
				continue
			elif dir.StreamType == MINIDUMP_STREAM_TYPE.MemoryListStream:
				logging.debug('Found MemoryListStream @%x Size: %d' % (dir.Location.Rva, dir.Location.DataSize))
				self.memory_segments = await MinidumpMemoryList.aparse(dir, self.file_handle)
				#logging.debug(str(self.memory_segments))
				continue
			elif dir.StreamType == MINIDUMP_STREAM_TYPE.SystemInfoStream:
				logging.debug('Found SystemInfoStream @%x Size: %d' % (dir.Location.Rva, dir.Location.DataSize))
				self.sysinfo = await MinidumpSystemInfo.aparse(dir, self.file_handle)
				#logging.debug(str(self.sysinfo))
				continue
			elif dir.StreamType == MINIDUMP_STREAM_TYPE.ThreadExListStream:
				logging.debug('Found ThreadExListStream @%x Size: %d' % (dir.Location.Rva, dir.Location.DataSize))
				self.threads_ex = await MinidumpThreadExList.aparse(dir, self.file_handle)
				#logging.debug(str(self.threads_ex))
				continue
			elif dir.StreamType == MINIDUMP_STREAM_TYPE.Memory64ListStream:
				logging.debug('Found Memory64ListStream @%x Size: %d' % (dir.Location.Rva, dir.Location.DataSize))
				self.memory_segments_64 = await MinidumpMemory64List.aparse(dir, self.file_handle)
				#logging.debug(str(self.memory_segments_64))
				continue
			elif dir.StreamType == MINIDUMP_STREAM_TYPE.CommentStreamA:
				logging.debug('Found CommentStreamA @%x Size: %d' % (dir.Location.Rva, dir.Location.DataSize))
				self.comment_a = await CommentStreamA.aparse(dir, self.file_handle)
				#logging.debug(str(self.comment_a))
				continue
			elif dir.StreamType == MINIDUMP_STREAM_TYPE.CommentStreamW:
				logging.debug('Found CommentStreamW @%x Size: %d' % (dir.Location.Rva, dir.Location.DataSize))
				self.comment_w = await CommentStreamW.aparse(dir, self.file_handle)
				#logging.debug(str(self.comment_w))
				continue
			elif dir.StreamType == MINIDUMP_STREAM_TYPE.ExceptionStream:
				logging.debug('Found ExceptionStream @%x Size: %d' % (dir.Location.Rva, dir.Location.DataSize))
				self.exception = await ExceptionList.aparse(dir, self.file_handle)
				#logging.debug(str(self.comment_w))
				continue
			elif dir.StreamType == MINIDUMP_STREAM_TYPE.HandleDataStream:
				logging.debug('Found HandleDataStream @%x Size: %d' % (dir.Location.Rva, dir.Location.DataSize))
				self.handles = await MinidumpHandleDataStream.aparse(dir, self.file_handle)
				#logging.debug(str(self.handles))
				continue

			elif dir.StreamType == MINIDUMP_STREAM_TYPE.FunctionTableStream:
				logging.debug('Found FunctionTableStream @%x Size: %d' % (dir.Location.Rva, dir.Location.DataSize))
				logging.debug('Parsing of this stream type is not yet implemented!')
				continue

			elif dir.StreamType == MINIDUMP_STREAM_TYPE.UnloadedModuleListStream:
				logging.debug('Found UnloadedModuleListStream @%x Size: %d' % (dir.Location.Rva, dir.Location.DataSize))
				self.unloaded_modules = await MinidumpUnloadedModuleList.aparse(dir, self.file_handle)
				#logging.debug(str(self.unloaded_modules))
				continue
			elif dir.StreamType == MINIDUMP_STREAM_TYPE.MiscInfoStream:
				logging.debug('Found MiscInfoStream @%x Size: %d' % (dir.Location.Rva, dir.Location.DataSize))
				self.misc_info = await MinidumpMiscInfo.aparse(dir, self.file_handle)
				#logging.debug(str(self.misc_info))
				continue
			elif dir.StreamType == MINIDUMP_STREAM_TYPE.MemoryInfoListStream:
				logging.debug('Found MemoryInfoListStream @%x Size: %d' % (dir.Location.Rva, dir.Location.DataSize))
				self.memory_info = await MinidumpMemoryInfoList.aparse(dir, self.file_handle)
				#logging.debug(str(self.memory_info))
				continue
			elif dir.StreamType == MINIDUMP_STREAM_TYPE.ThreadInfoListStream:
				logging.debug('Found ThreadInfoListStream @%x Size: %d' % (dir.Location.Rva, dir.Location.DataSize))
				self.thread_info = await MinidumpThreadInfoList.aparse(dir, self.file_handle)
				logging.debug(str(self.thread_info))
				continue
			elif dir.StreamType == MINIDUMP_STREAM_TYPE.SystemMemoryInfoStream:
				logging.debug('Found SystemMemoryInfoStream @%x Size: %d' % (dir.Location.Rva, dir.Location.DataSize))
				logging.debug('SystemMemoryInfoStream parsing is not implemented (Missing documentation)')
				continue

			elif dir.StreamType == MINIDUMP_STREAM_TYPE.JavaScriptDataStream:
				logging.debug('Found JavaScriptDataStream @%x Size: %d' % (dir.Location.Rva, dir.Location.DataSize))
				logging.debug('JavaScriptDataStream parsing is not implemented (Missing documentation)')

			elif dir.StreamType == MINIDUMP_STREAM_TYPE.ProcessVmCountersStream:
				logging.debug('Found ProcessVmCountersStream @%x Size: %d' % (dir.Location.Rva, dir.Location.DataSize))
				logging.debug('ProcessVmCountersStream parsing is not implemented (Missing documentation)')

			elif dir.StreamType == MINIDUMP_STREAM_TYPE.TokenStream:
				logging.debug('Found TokenStream @%x Size: %d' % (dir.Location.Rva, dir.Location.DataSize))
				logging.debug('TokenStream parsing is not implemented (Missing documentation)')

			else:
				logging.debug('Found Unknown Stream! Type: %s @%x Size: %d' % (dir.StreamType.name, dir.Location.Rva, dir.Location.DataSize))

			"""
			elif dir.StreamType == MINIDUMP_STREAM_TYPE.HandleOperationListStream:
			elif dir.StreamType == MINIDUMP_STREAM_TYPE.LastReservedStream:
			
			"""

	def __str__(self):
		t = '== Minidump File ==\n'
		t += str(self.header)
		t += str(self.sysinfo)
		for dir in self.directories:
			t += str(dir) + '\n'
		for mod in self.modules:
			t += str(mod) + '\n'
		if self.memory_segments is not None:
			for segment in self.memory_segments:
				t+= str(segment) + '\n'

		if self.memory_segments_64 is not None:
			for segment in self.memory_segments_64:
				t+= str(segment) + '\n'

		return t
