from minidump.utils.winapi.defines import *
import enum


# DWORD WINAPI GetLastError(void);
def GetLastError():
    _GetLastError = windll.kernel32.GetLastError
    _GetLastError.argtypes = []
    _GetLastError.restype  = DWORD
    return _GetLastError()

class WindowsMinBuild(enum.Enum):
	WIN_XP = 2500
	WIN_2K3 = 3000
	WIN_VISTA = 5000
	WIN_7 = 7000
	WIN_8 = 8000
	WIN_BLUE = 9400
	WIN_10 = 9800

#utter microsoft bullshit commencing..
def getWindowsBuild():
    class OSVersionInfo(ctypes.Structure):
        _fields_ = [
            ("dwOSVersionInfoSize" , ctypes.c_int),
            ("dwMajorVersion"      , ctypes.c_int),
            ("dwMinorVersion"      , ctypes.c_int),
            ("dwBuildNumber"       , ctypes.c_int),
            ("dwPlatformId"        , ctypes.c_int),
            ("szCSDVersion"        , ctypes.c_char*128)];
    GetVersionEx = getattr( ctypes.windll.kernel32 , "GetVersionExA")
    version  = OSVersionInfo()
    version.dwOSVersionInfoSize = ctypes.sizeof(OSVersionInfo)
    GetVersionEx( ctypes.byref(version) )
    return version.dwBuildNumber

def get_all_access_flags():
    DELETE = 0x00010000
    READ_CONTROL = 0x00020000
    WRITE_DAC = 0x00040000
    WRITE_OWNER = 0x00080000

    SYNCHRONIZE = 0x00100000

    STANDARD_RIGHTS_REQUIRED = DELETE | READ_CONTROL | WRITE_DAC | WRITE_OWNER
    STANDARD_RIGHTS_ALL = STANDARD_RIGHTS_REQUIRED | SYNCHRONIZE

    if getWindowsBuild() >= WindowsMinBuild.WIN_VISTA.value:
        return STANDARD_RIGHTS_REQUIRED | SYNCHRONIZE | 0xFFFF
    else:
        return STANDARD_RIGHTS_REQUIRED | SYNCHRONIZE | 0xFFF

PROCESS_ALL_ACCESS = get_all_access_flags()

# Process access rights for OpenProcess
PROCESS_TERMINATE                 = 0x0001
PROCESS_CREATE_THREAD             = 0x0002
PROCESS_SET_SESSIONID             = 0x0004
PROCESS_VM_OPERATION              = 0x0008
PROCESS_VM_READ                   = 0x0010
PROCESS_VM_WRITE                  = 0x0020
PROCESS_DUP_HANDLE                = 0x0040
PROCESS_CREATE_PROCESS            = 0x0080
PROCESS_SET_QUOTA                 = 0x0100
PROCESS_SET_INFORMATION           = 0x0200
PROCESS_QUERY_INFORMATION         = 0x0400
PROCESS_SUSPEND_RESUME            = 0x0800
PROCESS_QUERY_LIMITED_INFORMATION = 0x1000

# Thread access rights for OpenThread
THREAD_TERMINATE                 = 0x0001
THREAD_SUSPEND_RESUME            = 0x0002
THREAD_ALERT                     = 0x0004
THREAD_GET_CONTEXT               = 0x0008
THREAD_SET_CONTEXT               = 0x0010
THREAD_SET_INFORMATION           = 0x0020
THREAD_QUERY_INFORMATION         = 0x0040
THREAD_SET_THREAD_TOKEN          = 0x0080
THREAD_IMPERSONATE               = 0x0100
THREAD_DIRECT_IMPERSONATION      = 0x0200
THREAD_SET_LIMITED_INFORMATION   = 0x0400
THREAD_QUERY_LIMITED_INFORMATION = 0x0800

# typedef struct DECLSPEC_ALIGN(16) _MEMORY_BASIC_INFORMATION64 {
#     ULONGLONG BaseAddress;
#     ULONGLONG AllocationBase;
#     DWORD     AllocationProtect;
#     DWORD     __alignment1;
#     ULONGLONG RegionSize;
#     DWORD     State;
#     DWORD     Protect;
#     DWORD     Type;
#     DWORD     __alignment2;
# } MEMORY_BASIC_INFORMATION64, *PMEMORY_BASIC_INFORMATION64;
class MEMORY_BASIC_INFORMATION64(Structure):
    _fields_ = [
        ('BaseAddress',         ULONGLONG),     # remote pointer
        ('AllocationBase',      ULONGLONG),     # remote pointer
        ('AllocationProtect',   DWORD),
        ('__alignment1',        DWORD),
        ('RegionSize',          ULONGLONG),
        ('State',               DWORD),
        ('Protect',             DWORD),
        ('Type',                DWORD),
        ('__alignment2',        DWORD),
    ]

# typedef struct _MEMORY_BASIC_INFORMATION {
#     PVOID BaseAddress;
#     PVOID AllocationBase;
#     DWORD AllocationProtect;
#     SIZE_T RegionSize;
#     DWORD State;
#     DWORD Protect;
#     DWORD Type;
# } MEMORY_BASIC_INFORMATION, *PMEMORY_BASIC_INFORMATION;
class MEMORY_BASIC_INFORMATION(Structure):
    _fields_ = [
        ('BaseAddress',         SIZE_T),    # remote pointer
        ('AllocationBase',      SIZE_T),    # remote pointer
        ('AllocationProtect',   DWORD),
        ('RegionSize',          SIZE_T),
        ('State',               DWORD),
        ('Protect',             DWORD),
        ('Type',                DWORD),
    ]
PMEMORY_BASIC_INFORMATION = POINTER(MEMORY_BASIC_INFORMATION)


# HANDLE WINAPI OpenProcess(
#   __in  DWORD dwDesiredAccess,
#   __in  BOOL bInheritHandle,
#   __in  DWORD dwProcessId
# );
def OpenProcess(dwDesiredAccess, bInheritHandle, dwProcessId):
    _OpenProcess = windll.kernel32.OpenProcess
    _OpenProcess.argtypes = [DWORD, BOOL, DWORD]
    _OpenProcess.restype  = HANDLE

    hProcess = _OpenProcess(dwDesiredAccess, bool(bInheritHandle), dwProcessId)
    if hProcess == NULL:
        raise ctypes.WinError()
    return hProcess

# SIZE_T WINAPI VirtualQueryEx(
#   __in      HANDLE hProcess,
#   __in_opt  LPCVOID lpAddress,
#   __out     PMEMORY_BASIC_INFORMATION lpBuffer,
#   __in      SIZE_T dwLength
# );
def VirtualQueryEx(hProcess, lpAddress):
    _VirtualQueryEx = windll.kernel32.VirtualQueryEx
    _VirtualQueryEx.argtypes = [HANDLE, LPVOID, PMEMORY_BASIC_INFORMATION, SIZE_T]
    _VirtualQueryEx.restype  = SIZE_T

    lpBuffer  = MEMORY_BASIC_INFORMATION()
    dwLength  = sizeof(MEMORY_BASIC_INFORMATION)
    success   = _VirtualQueryEx(hProcess, lpAddress, byref(lpBuffer), dwLength)
    if success == 0:
        raise ctypes.WinError()
    return lpBuffer


# BOOL WINAPI ReadProcessMemory(
#   __in   HANDLE hProcess,
#   __in   LPCVOID lpBaseAddress,
#   __out  LPVOID lpBuffer,
#   __in   SIZE_T nSize,
#   __out  SIZE_T* lpNumberOfBytesRead
# );
def ReadProcessMemory(hProcess, lpBaseAddress, nSize):
    _ReadProcessMemory = windll.kernel32.ReadProcessMemory
    _ReadProcessMemory.argtypes = [HANDLE, LPVOID, LPVOID, SIZE_T, POINTER(SIZE_T)]
    _ReadProcessMemory.restype  = bool

    lpBuffer            = ctypes.create_string_buffer(b'', nSize)
    lpNumberOfBytesRead = SIZE_T(0)
    success = _ReadProcessMemory(hProcess, lpBaseAddress, lpBuffer, nSize, byref(lpNumberOfBytesRead))
    if not success and GetLastError() != ERROR_PARTIAL_COPY:
        raise ctypes.WinError()
    return str(lpBuffer.raw)[:lpNumberOfBytesRead.value]