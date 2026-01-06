#!/usr/bin/env python3
#
# Author:
#  Tamas Jos (@skelsec)
#
class MINIDUMP_USER_STREAM:
	def __init__(self):
		self.Type:int = None
		self.BufferSize:int = None
		self.Buffer = None

	@staticmethod
	def parse(buff):
		mus = MINIDUMP_USER_STREAM()
		mus.Type = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		mus.BufferSize = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)
		#this type is PVOID, not sure on the size
		mus.Buffer = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)

		return mus