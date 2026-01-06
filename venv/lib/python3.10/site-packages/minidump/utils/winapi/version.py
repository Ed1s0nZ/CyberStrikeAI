from minidump.utils.winapi.defines import *
import ctypes
from ctypes import windll, byref, Structure

class _SYSTEM_INFO_OEM_ID_STRUCT(Structure):
    _fields_ = [
        ("wProcessorArchitecture",  WORD),
        ("wReserved",               WORD),
]

class _SYSTEM_INFO_OEM_ID(Union):
    _fields_ = [
        ("dwOemId",  DWORD),
        ("w",        _SYSTEM_INFO_OEM_ID_STRUCT),
]


class SYSTEM_INFO(Structure):
    _fields_ = [
        ("id",                              _SYSTEM_INFO_OEM_ID),
        ("dwPageSize",                      DWORD),
        ("lpMinimumApplicationAddress",     LPVOID),
        ("lpMaximumApplicationAddress",     LPVOID),
        ("dwActiveProcessorMask",           DWORD_PTR),
        ("dwNumberOfProcessors",            DWORD),
        ("dwProcessorType",                 DWORD),
        ("dwAllocationGranularity",         DWORD),
        ("wProcessorLevel",                 WORD),
        ("wProcessorRevision",              WORD),
    ]

    def __get_dwOemId(self):
        return self.id.dwOemId
    def __set_dwOemId(self, value):
        self.id.dwOemId = value
    dwOemId = property(__get_dwOemId, __set_dwOemId)

    def __get_wProcessorArchitecture(self):
        return self.id.w.wProcessorArchitecture
    def __set_wProcessorArchitecture(self, value):
        self.id.w.wProcessorArchitecture = value
    wProcessorArchitecture = property(__get_wProcessorArchitecture, __set_wProcessorArchitecture)

LPSYSTEM_INFO = ctypes.POINTER(SYSTEM_INFO)

class OSVERSIONINFOW(Structure):
    _fields_ = [
        ("dwOSVersionInfoSize", DWORD),
        ("dwMajorVersion",      DWORD),
        ("dwMinorVersion",      DWORD),
        ("dwBuildNumber",       DWORD),
        ("dwPlatformId",        DWORD),
        ("szCSDVersion",        WCHAR * 128),
    ]

class OSVERSIONINFOEXW(Structure):
    _fields_ = [
        ("dwOSVersionInfoSize", DWORD),
        ("dwMajorVersion",      DWORD),
        ("dwMinorVersion",      DWORD),
        ("dwBuildNumber",       DWORD),
        ("dwPlatformId",        DWORD),
        ("szCSDVersion",        WCHAR * 128),
        ("wServicePackMajor",   WORD),
        ("wServicePackMinor",   WORD),
        ("wSuiteMask",          WORD),
        ("wProductType",        BYTE),
        ("wReserved",           BYTE),
    ]


def GetSystemInfo():
    _GetSystemInfo = windll.kernel32.GetSystemInfo
    _GetSystemInfo.argtypes = [LPSYSTEM_INFO]
    _GetSystemInfo.restype  = None

    sysinfo = SYSTEM_INFO()
    _GetSystemInfo(byref(sysinfo))
    return sysinfo

def GetVersionExW():
    _GetVersionExW = windll.kernel32.GetVersionExW
    _GetVersionExW.argtypes = [POINTER(OSVERSIONINFOEXW)]
    _GetVersionExW.restype  = bool
    _GetVersionExW.errcheck = RaiseIfZero

    osi = OSVERSIONINFOEXW()
    osi.dwOSVersionInfoSize = sizeof(osi)
    try:
        _GetVersionExW(byref(osi))
    except WindowsError:
        osi = OSVERSIONINFOW()
        osi.dwOSVersionInfoSize = sizeof(osi)
        _GetVersionExW.argtypes = [POINTER(OSVERSIONINFOW)]
        _GetVersionExW(byref(osi))
    return osi

def GetFileVersionInfoW(lptstrFilename):
    _GetFileVersionInfoW = windll.version.GetFileVersionInfoW
    _GetFileVersionInfoW.argtypes = [LPWSTR, DWORD, DWORD, LPVOID]
    _GetFileVersionInfoW.restype  = bool
    _GetFileVersionInfoW.errcheck = RaiseIfZero

    _GetFileVersionInfoSizeW = windll.version.GetFileVersionInfoSizeW
    _GetFileVersionInfoSizeW.argtypes = [LPWSTR, LPVOID]
    _GetFileVersionInfoSizeW.restype  = DWORD
    _GetFileVersionInfoSizeW.errcheck = RaiseIfZero

    dwLen = _GetFileVersionInfoSizeW(lptstrFilename, None)
    lpData = ctypes.create_string_buffer(dwLen)  # not a string!
    _GetFileVersionInfoW(lptstrFilename, 0, dwLen, byref(lpData))
    return lpData