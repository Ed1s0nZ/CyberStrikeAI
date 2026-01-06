
# https://msdn.microsoft.com/en-us/library/windows/desktop/ms680383(v=vs.85).aspx
class MINIDUMP_LOCATION_DESCRIPTOR:
	def __init__(self):
		self.DataSize = None
		self.Rva = None

	def get_size(self):
		return 8

	def to_bytes(self):
		t = self.DataSize.to_bytes(4, byteorder = 'little', signed = False)
		t += self.Rva.to_bytes(4, byteorder = 'little', signed = False)
		return t

	@staticmethod
	def parse(buff):
		mld = MINIDUMP_LOCATION_DESCRIPTOR()
		mld.DataSize = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		mld.Rva = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		return mld

	@staticmethod
	async def aparse(buff):
		mld = MINIDUMP_LOCATION_DESCRIPTOR()
		t = await buff.read(4)
		mld.DataSize = int.from_bytes(t, byteorder = 'little', signed = False)
		t = await buff.read(4)
		mld.Rva = int.from_bytes(t, byteorder = 'little', signed = False)
		return mld

	def __str__(self):
		t = 'Size: %s File offset: %s' % (self.DataSize, self.Rva)
		return t

class MINIDUMP_LOCATION_DESCRIPTOR64:
	def __init__(self):
		self.DataSize = None
		self.Rva = None

	def get_size(self):
		return 16

	def to_bytes(self):
		t = self.DataSize.to_bytes(8, byteorder = 'little', signed = False)
		t += self.Rva.to_bytes(8, byteorder = 'little', signed = False)
		return t

	@staticmethod
	def parse(buff):
		mld = MINIDUMP_LOCATION_DESCRIPTOR64()
		mld.DataSize = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)
		mld.Rva = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)
		return mld

	def __str__(self):
		t = 'Size: %s File offset: %s' % (self.DataSize, self.Rva)
		return t

class MINIDUMP_STRING:
	def __init__(self):
		self.Length = None
		self.Buffer = None

	@staticmethod
	def parse(buff):
		ms = MINIDUMP_STRING()
		ms.Length = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		ms.Buffer = buff.read(ms.Length)
		return ms

	@staticmethod
	async def aparse(buff):
		ms = MINIDUMP_STRING()
		t = await buff.read(4)
		ms.Length = int.from_bytes(t, byteorder = 'little', signed = False)
		ms.Buffer = await buff.read(ms.Length)
		return ms

	@staticmethod
	def get_from_rva(rva, buff):
		pos = buff.tell()
		buff.seek(rva, 0)
		ms = MINIDUMP_STRING.parse(buff)
		buff.seek(pos, 0)
		try:
			return ms.Buffer.decode('utf-16-le')
		except:
			return '<STRING_DECODE_FAILED>'

	@staticmethod
	async def aget_from_rva(rva, buff):
		pos = buff.tell()
		await buff.seek(rva, 0)
		ms = await MINIDUMP_STRING.aparse(buff)
		await buff.seek(pos, 0)
		try:
			return ms.Buffer.decode('utf-16-le')
		except:
			return '<STRING_DECODE_FAILED>'

class MinidumpMemorySegment:
	def __init__(self):
		self.start_virtual_address = None
		self.size = None
		self.end_virtual_address = None
		self.start_file_address = None

	@staticmethod
	def parse_mini(memory_decriptor, buff):
		"""
		memory_descriptor: MINIDUMP_MEMORY_DESCRIPTOR
		buff: file_handle
		"""
		mms = MinidumpMemorySegment()
		mms.start_virtual_address = memory_decriptor.StartOfMemoryRange
		mms.size = memory_decriptor.DataSize
		mms.start_file_address = memory_decriptor.Rva
		mms.end_virtual_address = mms.start_virtual_address + mms.size
		return mms

	@staticmethod
	def parse_full(memory_decriptor, rva):
		mms = MinidumpMemorySegment()
		mms.start_virtual_address = memory_decriptor.StartOfMemoryRange
		mms.size = memory_decriptor.DataSize
		mms.start_file_address = rva
		mms.end_virtual_address = mms.start_virtual_address + mms.size
		return mms

	def inrange(self, virt_addr):
		if virt_addr >= self.start_virtual_address and virt_addr < self.end_virtual_address:
			return True
		return False

	def read(self, virtual_address, size, file_handler):
		if virtual_address > self.end_virtual_address or virtual_address < self.start_virtual_address:
			raise Exception('Reading from wrong segment!')

		if virtual_address+size > self.end_virtual_address:
			raise Exception('Read would cross boundaries!')

		pos = file_handler.tell()
		offset = virtual_address - self.start_virtual_address
		file_handler.seek(self.start_file_address + offset, 0)
		data = file_handler.read(size)
		file_handler.seek(pos, 0)
		return data

	async def aread(self, virtual_address, size, file_handler):
		if virtual_address > self.end_virtual_address or virtual_address < self.start_virtual_address:
			raise Exception('Reading from wrong segment!')

		if virtual_address+size > self.end_virtual_address:
			raise Exception('Read would cross boundaries!')

		pos = file_handler.tell()
		offset = virtual_address - self.start_virtual_address
		await file_handler.seek(self.start_file_address + offset, 0)
		data = await file_handler.read(size)
		await file_handler.seek(pos, 0)
		return data

	def search(self, pattern, file_handler, find_first = False, chunksize = 50*1024):
		if len(pattern) > self.size:
			return []
		pos = file_handler.tell()
		file_handler.seek(self.start_file_address, 0)
		fl = []
		if find_first is True:
			chunksize = min(chunksize, self.size)
			data = b''
			i = 0
			while len(data) < self.size:
				i += 1
				if chunksize > (self.size - len(data)):
					chunksize = (self.size - len(data))
				data += file_handler.read(chunksize)
				marker = data.find(pattern)
				if marker != -1:
					#print('FOUND! size: %s i: %s read: %s perc: %s' % (self.size, i, i*chunksize, 100*((i*chunksize)/self.size)))
					file_handler.seek(pos, 0)
					return [self.start_virtual_address + marker]


			#print('NOTFOUND! size: %s i: %s read: %s perc %s' % (self.size, i, len(data), 100*(len(data)/self.size) ))

		else:
			data = file_handler.read(self.size)
			file_handler.seek(pos, 0)

			offset = 0
			while len(data) > len(pattern):
				marker = data.find(pattern)
				if marker == -1:
					return fl
				fl.append(marker + offset + self.start_virtual_address)
				data = data[marker+1:]
				offset += marker + 1
				if find_first is True:
					return fl

		file_handler.seek(pos, 0)
		return fl

	async def asearch(self, pattern, file_handler, find_first = False, chunksize = 50*1024):
		if len(pattern) > self.size:
			return []
		pos = file_handler.tell()
		await file_handler.seek(self.start_file_address, 0)
		fl = []

		if find_first is True:
			chunksize = min(chunksize, self.size)
			data = b''
			i = 0
			while len(data) < self.size:
				i += 1
				if chunksize > (self.size - len(data)):
					chunksize = (self.size - len(data))
				data += await file_handler.read(chunksize)
				marker = data.find(pattern)
				if marker != -1:
					#print('FOUND! size: %s i: %s read: %s perc: %s' % (self.size, i, i*chunksize, 100*((i*chunksize)/self.size)))
					await file_handler.seek(pos, 0)
					return [self.start_virtual_address + marker]


			#print('NOTFOUND! size: %s i: %s read: %s perc %s' % (self.size, i, len(data), 100*(len(data)/self.size) ))

		else:
			offset = 0
			data = await file_handler.read(self.size)
			await file_handler.seek(pos, 0)
			while len(data) > len(pattern):
				marker = data.find(pattern)
				if marker == -1:
					return fl
				fl.append(marker + offset + self.start_virtual_address)
				data = data[marker+1:]
				offset += marker + 1
				if find_first is True:
					return fl

		await file_handler.seek(pos, 0)
		return fl


	@staticmethod
	def get_header():
		t = [
			'VA Start',
			'RVA',
			'Size',
		]
		return t

	def to_row(self):
		t = [
			hex(self.start_virtual_address),
			hex(self.start_file_address),
			hex(self.size)
		]
		return t

	def __str__(self):
		t = 'VA Start: %s, RVA: %s, Size: %s' % (hex(self.start_virtual_address), hex(self.start_file_address), hex(self.size))
		return t



def hexdump( src, length=16, sep='.', start = 0):
	'''
	@brief Return {src} in hex dump.
	@param[in] length	{Int} Nb Bytes by row.
	@param[in] sep		{Char} For the text part, {sep} will be used for non ASCII char.
	@return {Str} The hexdump

	@note Full support for python2 and python3 !
	'''
	result = [];

	# Python3 support
	try:
		xrange(0,1);
	except NameError:
		xrange = range;

	for i in xrange(0, len(src), length):
		subSrc = src[i:i+length];
		hexa = '';
		isMiddle = False;
		for h in xrange(0,len(subSrc)):
			if h == length/2:
				hexa += ' ';
			h = subSrc[h];
			if not isinstance(h, int):
				h = ord(h);
			h = hex(h).replace('0x','');
			if len(h) == 1:
				h = '0'+h;
			hexa += h+' ';
		hexa = hexa.strip(' ');
		text = '';
		for c in subSrc:
			if not isinstance(c, int):
				c = ord(c);
			if 0x20 <= c < 0x7F:
				text += chr(c);
			else:
				text += sep;
		if start == 0:
			result.append(('%08x:  %-'+str(length*(2+1)+1)+'s  |%s|') % (i, hexa, text));
		else:
			result.append(('%08x(+%04x):  %-'+str(length*(2+1)+1)+'s  |%s|') % (start+i, i, hexa, text));
	return '\n'.join(result);

def construct_table(lines, separate_head=True):
	"""Prints a formatted table given a 2 dimensional array"""
	#Count the column width
	widths = []
	for line in lines:
			for i,size in enumerate([len(x) for x in line]):
					while i >= len(widths):
							widths.append(0)
					if size > widths[i]:
							widths[i] = size

	#Generate the format string to pad the columns
	print_string = ""
	for i,width in enumerate(widths):
			print_string += "{" + str(i) + ":" + str(width) + "} | "
	if (len(print_string) == 0):
			return
	print_string = print_string[:-3]

	#Print the actual data
	t = ''
	for i,line in enumerate(lines):
			t += print_string.format(*line) + '\n'
			if (i == 0 and separate_head):
					t += "-"*(sum(widths)+3*(len(widths)-1)) + '\n'

	return t
