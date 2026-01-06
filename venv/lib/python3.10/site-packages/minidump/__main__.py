#!/usr/bin/env python3
#
# Author:
#  Tamas Jos (@skelsec)
#

import logging
from minidump.minidumpfile import MinidumpFile
from minidump.common_structs import hexdump
from minidump.minidumpshell import MinidumpShell
from minidump._version import __banner__


def run():
	import argparse

	parser = argparse.ArgumentParser(description='A parser for minidumnp files')
	parser.add_argument('minidumpfile', help='path to the minidump file of lsass.exe')
	parser.add_argument('-v', '--verbose', action='count', default=0)
	parser.add_argument('-i', '--interactive', action='store_true', help='Interactive minidump shell')
	parser.add_argument('--header', action='store_true', help='File header info')
	parser.add_argument('--modules', action='store_true', help='List modules')
	parser.add_argument('--threads', action='store_true', help='List threads')
	parser.add_argument('--memory', action='store_true', help='List memory')
	parser.add_argument('--sysinfo', action='store_true', help='Show sysinfo')
	parser.add_argument('--comments', action='store_true', help='Show comments')
	parser.add_argument('--exception', action='store_true', help='Show exception records')
	parser.add_argument('--handles', action='store_true', help='List handles')
	parser.add_argument('--misc', action='store_true', help='Show misc info')
	parser.add_argument('--peb', action='store_true', help='Show PEB info')
	parser.add_argument('--all', action='store_true', help='Show all info')
	parser.add_argument('-r', '--read-addr', type=lambda x: int(x,0), help='Dump a memory region from the process\'s addres space')
	parser.add_argument('-s', '--read-size', type=lambda x: int(x,0), default = 0x20, help='Dump a memory region from the process\'s addres space')

	args = parser.parse_args()
	if args.verbose == 0:
		logging.basicConfig(level=logging.INFO)
	elif args.verbose == 1:
		logging.basicConfig(level=logging.DEBUG)
	else:
		logging.basicConfig(level=1)

	print(__banner__)

	if args.interactive:
		shell = MinidumpShell()
		shell.do_open(args.minidumpfile)
		shell.cmdloop()

	else:

		mf = MinidumpFile.parse(args.minidumpfile)
		reader = mf.get_reader()

		if args.all or args.threads:
			if mf.threads is not None:
				print(str(mf.threads))
			if mf.threads_ex is not None:
				print(str(mf.threads_ex))
			if mf.thread_info is not None:
				print(str(mf.thread_info))
		if args.all or args.modules:
			if mf.modules is not None:
				print(str(mf.modules))
			if mf.unloaded_modules is not None:
				print(str(mf.unloaded_modules))
		if args.all or args.memory:
			if mf.memory_segments is not None:
				print(str(mf.memory_segments))
			if mf.memory_segments_64 is not None:
				print(str(mf.memory_segments_64))
			if mf.memory_info is not None:
				print(str(mf.memory_info))
		if args.all or args.sysinfo:
			if mf.sysinfo is not None:
				print(str(mf.sysinfo))
		if args.all or args.exception:
			if mf.exception is not None:
				print(str(mf.exception))
		if args.all or args.comments:
			if mf.comment_a is not None:
				print(str(mf.comment_a))
			if mf.comment_w is not None:
				print(str(mf.comment_w))
		if args.all or args.handles:
			if mf.handles is not None:
				print(str(mf.handles))
		if args.all or args.misc:
			if mf.misc_info is not None:
				print(str(mf.misc_info))
		if args.all or args.header:
			print(str(mf.header))
		
		if args.all or args.peb:
			if mf.peb is not None:
				print(str(mf.peb))

		if args.read_addr:
			buff_reader = reader.get_buffered_reader()
			buff_reader.move(args.read_addr)
			data = buff_reader.peek(args.read_size)
			print(hexdump(data, start = args.read_addr))


if __name__ == '__main__':
	run()