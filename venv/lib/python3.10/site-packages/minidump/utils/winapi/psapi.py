
from minidump.utils.winapi.defines import *


# typedef struct _MODULEINFO {
#   LPVOID lpBaseOfDll;
#   DWORD  SizeOfImage;
#   LPVOID EntryPoint;
# } MODULEINFO, *LPMODULEINFO;
class MODULEINFO(Structure):
    _fields_ = [
        ("lpBaseOfDll",     LPVOID),    # remote pointer
        ("SizeOfImage",     DWORD),
        ("EntryPoint",      LPVOID),    # remote pointer
]
LPMODULEINFO = POINTER(MODULEINFO)

# BOOL WINAPI EnumProcessModules(
#   __in   HANDLE hProcess,
#   __out  HMODULE *lphModule,
#   __in   DWORD cb,
#   __out  LPDWORD lpcbNeeded
# );
def EnumProcessModules(hProcess):
    _EnumProcessModules = windll.psapi.EnumProcessModules
    _EnumProcessModules.argtypes = [HANDLE, LPVOID, DWORD, LPDWORD]
    _EnumProcessModules.restype = bool
    _EnumProcessModules.errcheck = RaiseIfZero

    size = 0x1000
    lpcbNeeded = DWORD(size)
    unit = sizeof(HMODULE)
    while 1:
        lphModule = (HMODULE * (size // unit))()
        _EnumProcessModules(hProcess, byref(lphModule), lpcbNeeded, byref(lpcbNeeded))
        needed = lpcbNeeded.value
        if needed <= size:
            break
        size = needed
    return [ lphModule[index] for index in range(0, int(needed // unit)) ]

def GetModuleFileNameExW(hProcess, hModule = None):
    _GetModuleFileNameExW = ctypes.windll.psapi.GetModuleFileNameExW
    _GetModuleFileNameExW.argtypes = [HANDLE, HMODULE, LPWSTR, DWORD]
    _GetModuleFileNameExW.restype = DWORD

    nSize = MAX_PATH
    while 1:
        lpFilename = ctypes.create_unicode_buffer(u"", nSize)
        nCopied = _GetModuleFileNameExW(hProcess, hModule, lpFilename, nSize)
        if nCopied == 0:
            raise ctypes.WinError()
        if nCopied < (nSize - 1):
            break
        nSize = nSize + MAX_PATH
    return lpFilename.value

# BOOL WINAPI GetModuleInformation(
#   __in   HANDLE hProcess,
#   __in   HMODULE hModule,
#   __out  LPMODULEINFO lpmodinfo,
#   __in   DWORD cb
# );
def GetModuleInformation(hProcess, hModule, lpmodinfo = None):
    _GetModuleInformation = windll.psapi.GetModuleInformation
    _GetModuleInformation.argtypes = [HANDLE, HMODULE, LPMODULEINFO, DWORD]
    _GetModuleInformation.restype = bool
    _GetModuleInformation.errcheck = RaiseIfZero

    if lpmodinfo is None:
        lpmodinfo = MODULEINFO()
    _GetModuleInformation(hProcess, hModule, byref(lpmodinfo), sizeof(lpmodinfo))
    return lpmodinfo