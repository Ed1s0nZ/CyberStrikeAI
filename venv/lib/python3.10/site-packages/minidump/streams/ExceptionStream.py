#!/usr/bin/env python3
#
# Author:
#  Tamas Jos (@skelsec)
#
# TODO: implement this better, the ExceptionInformation definition is missing on msdn :(

import io
import enum
from typing import List
from minidump.common_structs import *

# https://msdn.microsoft.com/en-us/library/windows/desktop/ms680368(v=vs.85).aspx
class MINIDUMP_EXCEPTION_STREAM:
	def __init__(self):
		self.ThreadId:int = None
		self.alignment:int = None
		self.ExceptionRecord:MINIDUMP_EXCEPTION = None
		self.ThreadContext:MINIDUMP_LOCATION_DESCRIPTOR = None

	@staticmethod
	def parse(buff):
		mes = MINIDUMP_EXCEPTION_STREAM()
		mes.ThreadId = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		mes.alignment = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		mes.ExceptionRecord = MINIDUMP_EXCEPTION.parse(buff)
		mes.ThreadContext = MINIDUMP_LOCATION_DESCRIPTOR.parse(buff)
		return mes

	def __str__(self):
		t  = '== MINIDUMP_EXCEPTION_STREAM ==\n'
		t += 'ThreadId: %s\n' % self.ThreadId
		# t += 'alignment: %s\n' % self.alignment
		t += 'ExceptionRecord:\n %s\n' % str(self.ExceptionRecord)
		t += 'ThreadContext: %s\n' % str(self.ThreadContext)
		return t

	@staticmethod
	def get_header():
		return [
			'ThreadId',
			*MINIDUMP_EXCEPTION.get_header()
		]


	def to_row(self):
		return [
			'0x%08x' % self.ThreadId,
			*self.ExceptionRecord.to_row()
		]


class ExceptionCode(enum.Enum):
	# Not a real exception code, it's just a placeholder to prevent the parser from raising an error
	EXCEPTION_UNKNOWN               = 'EXCEPTION_UNKNOWN_CHECK_RAW'
	EXCEPTION_NONE 					= 0x00

	# Linux SIG values (for crashpad generated dumps)
	EXCEPTION_SIGHUP    			= 0x00000001    # Hangup (POSIX)
	EXCEPTION_SIGINT    			= 0x00000002    # Terminal interrupt (ANSI)
	EXCEPTION_SIGQUIT   			= 0x00000003    # Terminal quit (POSIX)
	EXCEPTION_SIGILL    			= 0x00000004    # Illegal instruction (ANSI)
	EXCEPTION_SIGTRAP   			= 0x00000005    # Trace trap (POSIX)
	EXCEPTION_SIGIOT    			= 0x00000006    # IOT Trap (4.2 BSD)
	EXCEPTION_SIGBUS    			= 0x00000007    # BUS error (4.2 BSD)
	EXCEPTION_SIGFPE    			= 0x00000008    # Floating point exception (ANSI)
	EXCEPTION_SIGKILL   			= 0x00000009    # Kill(can't be caught or ignored) (POSIX)
	EXCEPTION_SIGUSR1   			= 0x0000000A   # User defined signal 1 (POSIX)
	EXCEPTION_SIGSEGV   			= 0x0000000B   # Invalid memory segment access (ANSI)
	EXCEPTION_SIGUSR2   			= 0x0000000C   # User defined signal 2 (POSIX)
	EXCEPTION_SIGPIPE   			= 0x0000000D   # Write on a pipe with no reader, Broken pipe (POSIX)
	EXCEPTION_SIGALRM   			= 0x0000000E   # Alarm clock (POSIX)
	EXCEPTION_SIGTERM   			= 0x0000000F   # Termination (ANSI)
	EXCEPTION_SIGSTKFLT 			= 0x00000010   # Stack fault
	EXCEPTION_SIGCHLD   			= 0x00000011   # Child process has stopped or exited, changed (POSIX)
	EXCEPTION_SIGCONTV  			= 0x00000012   # Continue executing, if stopped (POSIX)
	EXCEPTION_SIGSTOP   			= 0x00000013   # Stop executing(can't be caught or ignored) (POSIX)
	EXCEPTION_SIGTSTP   			= 0x00000014   # Terminal stop signal (POSIX)
	EXCEPTION_SIGTTIN   			= 0x00000015   # Background process trying to read, from TTY (POSIX)
	EXCEPTION_SIGTTOU   			= 0x00000016   # Background process trying to write, to TTY (POSIX)
	EXCEPTION_SIGURG    			= 0x00000017   # Urgent condition on socket (4.2 BSD)
	EXCEPTION_SIGXCPU   			= 0x00000018   # CPU limit exceeded (4.2 BSD)
	EXCEPTION_SIGXFSZ   			= 0x00000019   # File size limit exceeded (4.2 BSD)
	EXCEPTION_SIGVTALRM 			= 0x0000001A   # Virtual alarm clock (4.2 BSD)
	EXCEPTION_SIGPROF   			= 0x0000001B   # Profiling alarm clock (4.2 BSD)
	EXCEPTION_SIGWINCH  			= 0x0000001C   # Window size change (4.3 BSD, Sun)
	EXCEPTION_SIGIO     			= 0x0000001D   # I/O now possible (4.2 BSD)
	EXCEPTION_SIGPWR    			= 0x0000001E   # Power failure restart (System V)

	# Standard Windows exception values
	EXCEPTION_ACCESS_VIOLATION 		= 0xC0000005 	# The thread tried to read from or write to a virtual address for which it does not have the appropriate access.
	EXCEPTION_ARRAY_BOUNDS_EXCEEDED = 0xC000008C 	# The thread tried to access an array element that is out of bounds and the underlying hardware supports bounds checking.
	EXCEPTION_BREAKPOINT 			= 0x80000003 	# A breakpoint was encountered.
	EXCEPTION_DATATYPE_MISALIGNMENT = 0x80000002 	# The thread tried to read or write data that is misaligned on hardware that does not provide alignment. For example, 16-bit values must be aligned on 2-byte boundaries; 32-bit values on 4-byte boundaries, and so on.
	EXCEPTION_FLT_DENORMAL_OPERAND 	= 0xC000008D 	# One of the operands in a floating-point operation is denormal. A denormal value is one that is too small to represent as a standard floating-point value.
	EXCEPTION_FLT_DIVIDE_BY_ZERO 	= 0xC000008E	# The thread tried to divide a floating-point value by a floating-point divisor of zero.
	EXCEPTION_FLT_INEXACT_RESULT 	= 0xC000008F	# The result of a floating-point operation cannot be represented exactly as a decimal fraction.
	EXCEPTION_FLT_INVALID_OPERATION = 0xC0000090	# This exception represents any floating-point exception not included in this list.
	EXCEPTION_FLT_OVERFLOW 			= 0xC0000091 	# The exponent of a floating-point operation is greater than the magnitude allowed by the corresponding type.
	EXCEPTION_FLT_STACK_CHECK 		= 0xC0000092	# The stack overflowed or underflowed as the result of a floating-point operation.
	EXCEPTION_FLT_UNDERFLOW 		= 0xC0000093	# The exponent of a floating-point operation is less than the magnitude allowed by the corresponding type.
	EXCEPTION_ILLEGAL_INSTRUCTION 	= 0xC000001D	# The thread tried to execute an invalid instruction.
	EXCEPTION_IN_PAGE_ERROR 		= 0xC0000006	# The thread tried to access a page that was not present, and the system was unable to load the page. For example, this exception might occur if a network connection is lost while running a program over the network.
	EXCEPTION_INT_DIVIDE_BY_ZERO 	= 0xC0000094	# The thread tried to divide an integer value by an integer divisor of zero.
	EXCEPTION_INT_OVERFLOW 			= 0xC0000095	# The result of an integer operation caused a carry out of the most significant bit of the result.
	EXCEPTION_INVALID_DISPOSITION 	= 0xC0000026	# An exception handler returned an invalid disposition to the exception dispatcher. Programmers using a high-level language such as C should never encounter this exception.
	EXCEPTION_NONCONTINUABLE_EXCEPTION =0xC0000025  # The thread tried to continue execution after a noncontinuable exception occurred.
	EXCEPTION_PRIV_INSTRUCTION 		= 0xC0000096	# The thread tried to execute an instruction whose operation is not allowed in the current machine mode.
	EXCEPTION_SINGLE_STEP 			= 0x80000004	# A trace trap or other single-instruction mechanism signaled that one instruction has been executed.
	EXCEPTION_STACK_OVERFLOW 		= 0xC00000FD	# The thread used up its stack.

#https://msdn.microsoft.com/en-us/library/windows/desktop/ms680367(v=vs.85).aspx
class MINIDUMP_EXCEPTION:
	EXCEPTION_MAXIMUM_PARAMETERS = 15

	def __init__(self):
		self.ExceptionCode = None
		self.ExceptionFlags = None
		self.ExceptionRecord = None
		self.ExceptionAddress = None
		self.NumberParameters = None
		self.__unusedAlignment = None
		self.ExceptionInformation = []
		self.ExceptionCode_raw = None

	@staticmethod
	def parse(buff):
		me = MINIDUMP_EXCEPTION()
		me.ExceptionCode_raw = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		try:
			me.ExceptionCode = ExceptionCode(me.ExceptionCode_raw)
		except:
			me.ExceptionCode = ExceptionCode.EXCEPTION_UNKNOWN

		me.ExceptionFlags = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		me.ExceptionRecord = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)
		me.ExceptionAddress = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)
		me.NumberParameters = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		me.__unusedAlignment = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
		for _ in range(MINIDUMP_EXCEPTION.EXCEPTION_MAXIMUM_PARAMETERS):
			me.ExceptionInformation.append(int.from_bytes(buff.read(8), byteorder = 'little', signed = False))

		me.ExceptionInformation = me.ExceptionInformation[:me.NumberParameters]
		return me

	def __str__(self):
		t  = '== MINIDUExceptionInformationMP_EXCEPTION ==\n'
		t += "ExceptionCode : %s\n" % self.ExceptionCode
		t += "ExceptionFlags : %s\n" % self.ExceptionFlags
		t += "ExceptionRecord : %s\n" % self.ExceptionRecord
		t += "ExceptionAddress : 0x%x\n" % self.ExceptionAddress
		t += "NumberParameters : %s\n" % self.NumberParameters
		# t += "__unusedAlignment : %s\n" % self.__unusedAlignment
		t += "ExceptionInformation : %s\n" % ";".join("0x%x" % info for info in self.ExceptionInformation)
		return t

	@staticmethod
	def get_header():
		return [
			'ExceptionCode',
			'ExceptionFlags',
			'ExceptionRecord',
			'ExceptionAddress',
			'ExceptionInformation'
		]


	def to_row(self):
		return [
			str(self.ExceptionCode),
			'0x%08x' % self.ExceptionFlags,
			'0x%08x' % self.ExceptionRecord,
			'0x%08x' % self.ExceptionAddress,
			str(self.ExceptionInformation)
		]


class ExceptionList:
	def __init__(self):
		self.exception_records:List[MINIDUMP_EXCEPTION_STREAM] = []

	@staticmethod
	def parse(dir, buff):
		t = ExceptionList()

		buff.seek(dir.Location.Rva)
		chunk = io.BytesIO(buff.read(dir.Location.DataSize))

		# Unfortunately, we don't have a certain way to figure out how many exception records
		# there is in the stream, so we have to fallback on heuristics (EOF or bad data read)
		#
		# NB : 	some tool only read one exception record : https://github.com/GregTheDev/MinidumpExplorer/blob/a6dd974757c16142eefcfff7d99be10b14f87eaf/MinidumpExplorer/MinidumpExplorer/MainForm.cs#L257
		#		but it's incorrect since we can have an exception chain (double fault, exception catched and re-raised, etc.)
		while chunk.tell() < dir.Location.DataSize:
			mes = MINIDUMP_EXCEPTION_STREAM.parse(chunk)

			# a minidump exception stream is usally padded with zeroes
			# so whenever we parse an exception record with the code EXCEPTION_NONE
			# we can stop.
			if mes.ExceptionRecord.ExceptionCode == ExceptionCode.EXCEPTION_NONE:
				break

			t.exception_records.append(mes)

		return t

	@staticmethod
	async def aparse(dir, buff):
		t = ExceptionList()

		await buff.seek(dir.Location.Rva)
		chunk_data = await buff.read(dir.Location.DataSize)
		chunk = io.BytesIO(chunk_data)

		# Unfortunately, we don't have a certain way to figure out how many exception records
		# there is in the stream, so we have to fallback on heuristics (EOF or bad data read)
		#
		# NB : 	some tool only read one exception record : https://github.com/GregTheDev/MinidumpExplorer/blob/a6dd974757c16142eefcfff7d99be10b14f87eaf/MinidumpExplorer/MinidumpExplorer/MainForm.cs#L257
		#		but it's incorrect since we can have an exception chain (double fault, exception catched and re-raised, etc.)
		while chunk.tell() < dir.Location.DataSize:
			mes = MINIDUMP_EXCEPTION_STREAM.parse(chunk)

			# a minidump exception stream is usally padded with zeroes
			# so whenever we parse an exception record with the code EXCEPTION_NONE
			# we can stop.
			if mes.ExceptionRecord.ExceptionCode == ExceptionCode.EXCEPTION_NONE:
				break

			t.exception_records.append(mes)

		return t

	def to_table(self):
		t = []
		t.append(MINIDUMP_EXCEPTION_STREAM.get_header())
		for ex_record in self.exception_records:
			t.append(ex_record.to_row())
		return t

	def __str__(self):
		return '== ExceptionList ==\n' + construct_table(self.to_table())
