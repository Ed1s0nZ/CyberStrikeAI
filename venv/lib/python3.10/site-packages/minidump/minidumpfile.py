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
from typing import List

from minidump.header import MinidumpHeader
from minidump.minidumpreader import MinidumpFileReader
from minidump.streams import *
from minidump.common_structs import *
from minidump.constants import MINIDUMP_STREAM_TYPE
from minidump.directory import MINIDUMP_DIRECTORY
from minidump.streams.SystemInfoStream import PROCESSOR_ARCHITECTURE
from minidump.structures.peb import PEB


class MinidumpFile:
	def __init__(self):
		self.filename:str = None
		self.file_handle = None
		self.header:MinidumpHeader = None
		self.directories: List[MINIDUMP_DIRECTORY] = []

		self.threads_ex:MinidumpThreadExList = None
		self.threads:MinidumpThreadList = None
		self.modules:MinidumpModuleList = None
		self.memory_segments:MinidumpMemoryList = None
		self.memory_segments_64:MinidumpMemory64List = None
		self.sysinfo:MinidumpSystemInfo = None
		self.comment_a:CommentStreamA = None
		self.comment_w:CommentStreamW = None
		self.exception:ExceptionList = None
		self.handles:MinidumpHandleDataStream = None
		self.unloaded_modules:MinidumpUnloadedModuleList = None
		self.misc_info:MinidumpMiscInfo = None
		self.memory_info:MinidumpMemoryInfoList = None
		self.thread_info:MinidumpThreadInfoList = None

		self.peb:PEB = None

	@staticmethod
	def parse(filename):
		mf = MinidumpFile()
		mf.filename = filename
		mf.file_handle = open(filename, 'rb')
		mf._parse()
		return mf

	@staticmethod
	def parse_external(file_handle, filename = ''):
		"""
		External file handle must be an object that exposes basic file IO functionality
		that you'd get by python's file buffer (read, seek, tell etc.)
		"""
		mf = MinidumpFile()
		mf.filename = filename
		mf.file_handle = file_handle
		mf._parse()
		return mf

	@staticmethod
	def parse_bytes(data):
		return MinidumpFile.parse_buff(io.BytesIO(data))

	@staticmethod
	def parse_buff(buffer):
		mf = MinidumpFile()
		mf.file_handle = buffer
		mf._parse()
		return mf

	def get_reader(self):
		return MinidumpFileReader(self)

	def _parse(self):
		self.__parse_header()
		self.__parse_directories()
		try:
			self.__parse_peb()
		except Exception as e:
			logging.exception('PEB parsing error!')

	def __parse_header(self):
		self.header = MinidumpHeader.parse(self.file_handle)
		for i in range(0, self.header.NumberOfStreams):
			self.file_handle.seek(self.header.StreamDirectoryRva + i * 12, 0 )
			minidump_dir = MINIDUMP_DIRECTORY.parse(self.file_handle)

			if minidump_dir:
				self.directories.append(minidump_dir)
			else:
				self.file_handle.seek(self.header.StreamDirectoryRva + i * 12, 0 )
				user_stream_type_value = MINIDUMP_DIRECTORY.get_stream_type_value(self.file_handle)
				logging.debug('Found Unknown UserStream directory Type: %x' % (user_stream_type_value))

	def __parse_directories(self):

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
				self.threads = MinidumpThreadList.parse(dir, self.file_handle)
				continue
			elif dir.StreamType == MINIDUMP_STREAM_TYPE.ModuleListStream:
				logging.debug('Found ModuleListStream @%x Size: %d' % (dir.Location.Rva, dir.Location.DataSize))
				self.modules = MinidumpModuleList.parse(dir, self.file_handle)
				#logging.debug(str(modules_list))
				continue
			elif dir.StreamType == MINIDUMP_STREAM_TYPE.MemoryListStream:
				logging.debug('Found MemoryListStream @%x Size: %d' % (dir.Location.Rva, dir.Location.DataSize))
				self.memory_segments = MinidumpMemoryList.parse(dir, self.file_handle)
				#logging.debug(str(self.memory_segments))
				continue
			elif dir.StreamType == MINIDUMP_STREAM_TYPE.SystemInfoStream:
				logging.debug('Found SystemInfoStream @%x Size: %d' % (dir.Location.Rva, dir.Location.DataSize))
				self.sysinfo = MinidumpSystemInfo.parse(dir, self.file_handle)
				#logging.debug(str(self.sysinfo))
				continue
			elif dir.StreamType == MINIDUMP_STREAM_TYPE.ThreadExListStream:
				logging.debug('Found ThreadExListStream @%x Size: %d' % (dir.Location.Rva, dir.Location.DataSize))
				self.threads_ex = MinidumpThreadExList.parse(dir, self.file_handle)
				#logging.debug(str(self.threads_ex))
				continue
			elif dir.StreamType == MINIDUMP_STREAM_TYPE.Memory64ListStream:
				logging.debug('Found Memory64ListStream @%x Size: %d' % (dir.Location.Rva, dir.Location.DataSize))
				self.memory_segments_64 = MinidumpMemory64List.parse(dir, self.file_handle)
				#logging.debug(str(self.memory_segments_64))
				continue
			elif dir.StreamType == MINIDUMP_STREAM_TYPE.CommentStreamA:
				logging.debug('Found CommentStreamA @%x Size: %d' % (dir.Location.Rva, dir.Location.DataSize))
				self.comment_a = CommentStreamA.parse(dir, self.file_handle)
				#logging.debug(str(self.comment_a))
				continue
			elif dir.StreamType == MINIDUMP_STREAM_TYPE.CommentStreamW:
				logging.debug('Found CommentStreamW @%x Size: %d' % (dir.Location.Rva, dir.Location.DataSize))
				self.comment_w = CommentStreamW.parse(dir, self.file_handle)
				#logging.debug(str(self.comment_w))
				continue
			elif dir.StreamType == MINIDUMP_STREAM_TYPE.ExceptionStream:
				logging.debug('Found ExceptionStream @%x Size: %d' % (dir.Location.Rva, dir.Location.DataSize))
				self.exception = ExceptionList.parse(dir, self.file_handle)
				#logging.debug(str(self.comment_w))
				continue
			elif dir.StreamType == MINIDUMP_STREAM_TYPE.HandleDataStream:
				logging.debug('Found HandleDataStream @%x Size: %d' % (dir.Location.Rva, dir.Location.DataSize))
				self.handles = MinidumpHandleDataStream.parse(dir, self.file_handle)
				#logging.debug(str(self.handles))
				continue

			elif dir.StreamType == MINIDUMP_STREAM_TYPE.FunctionTableStream:
				logging.debug('Found FunctionTableStream @%x Size: %d' % (dir.Location.Rva, dir.Location.DataSize))
				logging.debug('Parsing of this stream type is not yet implemented!')
				continue

			elif dir.StreamType == MINIDUMP_STREAM_TYPE.UnloadedModuleListStream:
				logging.debug('Found UnloadedModuleListStream @%x Size: %d' % (dir.Location.Rva, dir.Location.DataSize))
				self.unloaded_modules = MinidumpUnloadedModuleList.parse(dir, self.file_handle)
				#logging.debug(str(self.unloaded_modules))
				continue
			elif dir.StreamType == MINIDUMP_STREAM_TYPE.MiscInfoStream:
				logging.debug('Found MiscInfoStream @%x Size: %d' % (dir.Location.Rva, dir.Location.DataSize))
				self.misc_info = MinidumpMiscInfo.parse(dir, self.file_handle)
				#logging.debug(str(self.misc_info))
				continue
			elif dir.StreamType == MINIDUMP_STREAM_TYPE.MemoryInfoListStream:
				logging.debug('Found MemoryInfoListStream @%x Size: %d' % (dir.Location.Rva, dir.Location.DataSize))
				self.memory_info = MinidumpMemoryInfoList.parse(dir, self.file_handle)
				#logging.debug(str(self.memory_info))
				continue
			elif dir.StreamType == MINIDUMP_STREAM_TYPE.ThreadInfoListStream:
				logging.debug('Found ThreadInfoListStream @%x Size: %d' % (dir.Location.Rva, dir.Location.DataSize))
				self.thread_info = MinidumpThreadInfoList.parse(dir, self.file_handle)
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
		try:
			self.__parse_thread_context()
		except Exception as e:
			logging.exception('Thread context parsing error!')

	def __parse_thread_context(self):
		if not self.sysinfo or not self.threads:
			return
		for thread in self.threads.threads:
			rva = thread.ThreadContext.Rva
			self.file_handle.seek(rva)
			if self.sysinfo.ProcessorArchitecture == PROCESSOR_ARCHITECTURE.AMD64:
				thread.ContextObject = CONTEXT.parse(self.file_handle)
			elif self.sysinfo.ProcessorArchitecture == PROCESSOR_ARCHITECTURE.INTEL:
				thread.ContextObject = WOW64_CONTEXT.parse(self.file_handle)

	def __parse_peb(self):
		if not self.sysinfo or not self.threads:
			return
		
		self.peb = PEB.from_minidump(self)
		


	def __str__(self):
		t = '== Minidump File ==\n'
		t += str(self.header)
		t += str(self.sysinfo)
		for dir in self.directories:
			t += str(dir) + '\n'
		for mod in self.modules.modules:
			t += str(mod) + '\n'
		if self.memory_segments is not None:
			for segment in self.memory_segments.memory_segments:
				t+= str(segment) + '\n'

		if self.memory_segments_64 is not None:
			for segment in self.memory_segments_64.memory_segments:
				t+= str(segment) + '\n'

		return t
