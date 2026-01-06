#!/usr/bin/env python3
#
# Author:
#  Tamas Jos (@skelsec)
#
class CommentStreamA:
	def __init__(self):
		self.data:str = None

	def to_bytes(self):
		return self.data.encode('ascii')

	@staticmethod
	def parse(dir, buff):
		csa = CommentStreamA()
		buff.seek(dir.Location.Rva)
		csa.data = buff.read(dir.Location.DataSize).decode()
		return csa

	@staticmethod
	async def aparse(dir, buff):
		csa = CommentStreamA()
		await buff.seek(dir.Location.Rva)
		csdata = await buff.read(dir.Location.DataSize)
		csa.data = csdata.decode()
		return csa

	def __str__(self):
		return 'CommentA: %s' % self.data