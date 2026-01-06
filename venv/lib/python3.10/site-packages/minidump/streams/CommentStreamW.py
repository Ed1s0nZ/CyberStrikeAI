#!/usr/bin/env python3
#
# Author:
#  Tamas Jos (@skelsec)
#
class CommentStreamW:
	def __init__(self):
		self.data:str = None

	def to_bytes(self):
		return self.data.encode('utf-16-le')

	@staticmethod
	def parse(dir, buff):
		csa = CommentStreamW()
		buff.seek(dir.Location.Rva)
		csa.data = buff.read(dir.Location.DataSize).decode('utf-16-le')
		return csa

	@staticmethod
	async def aparse(dir, buff):
		csa = CommentStreamW()
		await buff.seek(dir.Location.Rva)
		csdata = await buff.read(dir.Location.DataSize)
		csa.data = csdata.decode('utf-16-le')
		return csa

	def __str__(self):
		return 'CommentW: %s' % self.data