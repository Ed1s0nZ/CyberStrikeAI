import enum

class MINIDUMP_STREAM_TYPE(enum.Enum):
	UnusedStream			   	= 0
	ReservedStream0				= 1
	ReservedStream1				= 2
	ThreadListStream		   	= 3
	ModuleListStream		   	= 4
	MemoryListStream		   	= 5
	ExceptionStream				= 6
	SystemInfoStream		   	= 7
	ThreadExListStream		 	= 8
	Memory64ListStream		 	= 9
	CommentStreamA			 	= 10
	CommentStreamW			 	= 11
	HandleDataStream		   	= 12
	FunctionTableStream			= 13
	UnloadedModuleListStream   	= 14
	MiscInfoStream			 	= 15
	MemoryInfoListStream	   	= 16
	ThreadInfoListStream	   	= 17
	HandleOperationListStream  	= 18
	TokenStream 				= 19
	JavaScriptDataStream 		= 20
	SystemMemoryInfoStream 		= 21
	ProcessVmCountersStream 	= 22
	ThreadNamesStream 			= 24
	ceStreamNull 				= 25
	ceStreamSystemInfo 			= 26
	ceStreamException 			= 27
	ceStreamModuleList 			= 28
	ceStreamProcessList 		= 29
	ceStreamThreadList 			= 30
	ceStreamThreadContextList 	= 31
	ceStreamThreadCallStackList = 32
	ceStreamMemoryVirtualList 	= 33
	ceStreamMemoryPhysicalList 	= 34
	ceStreamBucketParameters 	= 35
	ceStreamProcessModuleMap 	= 36
	ceStreamDiagnosisList 		= 37
	LastReservedStream		 	= 0xffff


class MINIDUMP_TYPE(enum.IntFlag):
	MiniDumpNormal                         = 0x00000000
	MiniDumpWithDataSegs                   = 0x00000001
	MiniDumpWithFullMemory                 = 0x00000002
	MiniDumpWithHandleData                 = 0x00000004
	MiniDumpFilterMemory                   = 0x00000008
	MiniDumpScanMemory                     = 0x00000010
	MiniDumpWithUnloadedModules            = 0x00000020
	MiniDumpWithIndirectlyReferencedMemory = 0x00000040
	MiniDumpFilterModulePaths              = 0x00000080
	MiniDumpWithProcessThreadData          = 0x00000100
	MiniDumpWithPrivateReadWriteMemory     = 0x00000200
	MiniDumpWithoutOptionalData            = 0x00000400
	MiniDumpWithFullMemoryInfo             = 0x00000800
	MiniDumpWithThreadInfo                 = 0x00001000
	MiniDumpWithCodeSegs                   = 0x00002000
	MiniDumpWithoutAuxiliaryState          = 0x00004000
	MiniDumpWithFullAuxiliaryState         = 0x00008000
	MiniDumpWithPrivateWriteCopyMemory     = 0x00010000
	MiniDumpIgnoreInaccessibleMemory       = 0x00020000
	MiniDumpWithTokenInformation           = 0x00040000
	MiniDumpWithModuleHeaders              = 0x00080000
	MiniDumpFilterTriage                   = 0x00100000
	MiniDumpValidTypeFlags                 = 0x001fffff

