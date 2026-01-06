
import cmd
from minidump.minidumpfile import *
from minidump.common_structs import hexdump

def args2int(x):
	if isinstance(x, int):
		return x
	elif isinstance(x, str):
		if x[:2].lower() == '0x':
			return int(x[2:], 16)
		elif x[:2].lower() == '0b':
			return int(x[2:], 2)
		else:
			return int(x)

	else:
		raise Exception('Unknown integer format! %s' % type(x))

class MinidumpShell(cmd.Cmd):
	intro  = 'Welcome to the minidump shell.   Type help or ? to list commands.\n'
	prompt = '[minidump] '
	mini   = None
	reader = None
	hexdump_size = 16

	def do_open(self, filename):
		"""Opens minidump file"""
		self.mini = MinidumpFile.parse(filename)
		self.reader = self.mini.get_reader().get_buffered_reader()
	
	def do_peb(self, args):
		"""Shows PEB information (if available)"""
		if self.mini.peb is not None:
			print(str(self.mini.peb))

	def do_threads(self, args):
		"""Lists all thread information (if available)"""
		if self.mini.threads is not None:
			print(str(self.mini.threads))
		if self.mini.threads_ex is not None:
			print(str(self.mini.threads_ex))
		if self.mini.thread_info is not None:
			print(str(self.mini.thread_info))

	def do_memory(self, args):
		"""Lists all memory segments"""
		if self.mini.memory_segments is not None:
			print(str(self.mini.memory_segments))
		if self.mini.memory_segments_64 is not None:
			print(str(self.mini.memory_segments_64))
		if self.mini.memory_info is not None:
			print(str(self.mini.memory_info))

	def do_modules(self, args):
		"""Lists all loaded and unloaded module information (if available)"""
		if self.mini.modules is not None:
			print(str(self.mini.modules))
		if self.mini.unloaded_modules is not None:
			print(str(self.mini.unloaded_modules))

	def do_sysinfo(self, args):
		"""Shows sysinfo (if available)"""
		if self.mini.sysinfo is not None:
			print(str(self.mini.sysinfo))

	def do_exception(self, args):
		"""Shows exception information (if available)"""
		if self.mini.exception is not None:
			print(str(self.mini.exception))

	def do_comments(self, args):
		"""Lists all comments (if any)"""
		if self.mini.comment_a is not None:
			print(str(self.mini.comment_a))
		if self.mini.comment_w is not None:
			print(str(self.mini.comment_w))

	def do_handles(self, args):
		"""Lists all handles (if available)"""
		if self.mini.handles is not None:
			print(str(self.mini.handles))

	def do_misc(self, args):
		"""Lists all miscellaneous info (if available)"""
		if self.mini.misc_info is not None:
			print(str(self.mini.misc_info))

	#### Exit aliases
	def do_quit(self, args):
		"""Quit"""
		return True
	def do_exit(self, args):
		"""Quit"""
		return self.do_quit(None)
	def do_q(self, args):
		"""Quit"""
		return self.do_quit(None)

	###### READER
	def do_printsize(self, printsize):
		"""Changes the hexdump print size to the given bytes/line size (default: 16)"""
		self.hexdump_size = args2int(printsize)

	def update_prompt(self, args):
		pos = self.reader.tell()
		current_segment_start = self.reader.current_segment.start_address
		segment_relative_position = pos - current_segment_start
		self.prompt = "[%s %s+%s] " % (hex(pos), hex(current_segment_start), hex(segment_relative_position))

	def do_tell(self, args):
		"""Shows/refreshes the current position in the process' virtual memory space"""
		x = self.reader.tell()
		if x is None:
			print('Reader not yet positioned! Issue a "move" command with the desired memory address!')
		print(hex(x))

	def do_move(self, position):
		"""Sets the current position in the process' virtual memory space"""
		pos = args2int(position)
		self.reader.move(pos)
		self.update_prompt(None)

	def do_read(self, count):
		"""Performs a read of 'count' bytes from the current position and updates the cursor with the bytes read"""
		count = args2int(count)
		pos_before = self.reader.tell()
		data = self.reader.read(count)
		print(hexdump( data, length=self.hexdump_size, sep='.', start = pos_before))
		self.update_prompt(None)

	def do_readi(self, args):
		"""Reads a signed integer starting the current position and updates the cursor with the bytes read. The integer size is determined automatically by the processor architecture information from the dump file"""
		data = self.reader.read_int()
		print('D: %s' % data)
		print('H: %s' % hex(data))
		self.update_prompt(None)

	def do_readui(self, args):
		"""Reads an unsigned integer starting the current position and updates the cursor with the bytes read. The integer size is determined automatically by the processor architecture information from the dump file"""
		data = self.reader.read_uint()
		print('D: %s' % data)
		print('H: %s' % hex(data))
		self.update_prompt(None)

	def do_peek(self, count):
		"""Performs a read of 'count' bytes from the current position but doesn't update the cursor."""
		count = args2int(count)
		pos_before = self.reader.tell()
		data = self.reader.peek(count)
		print(hexdump( data, length=self.hexdump_size, sep='.', start = pos_before))
		self.update_prompt(None)


def main():
	import argparse

	parser = argparse.ArgumentParser(description='A parser for minidumnp files')
	parser.add_argument('-f', '--minidumpfile', help='path to the minidump file of lsass.exe')
	args = parser.parse_args()

	shell = MinidumpShell()
	if args.minidumpfile:
		shell.do_open(args.minidumpfile)
	shell.cmdloop()

if __name__ == '__main__':
	main()