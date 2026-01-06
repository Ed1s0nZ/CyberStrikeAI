#!/usr/bin/env python3

# https://www.vergiliusproject.com/kernels/x64/Windows%2010%20%7C%202016/1507%20Threshold%201/_M128A
class M128A:
    def __init__(self):
        self.Low = 0                                        # 0x0 ULONGLONG
        self.High = 0                                       # 0x8 LONGLONG

    @classmethod
    def parse(cls, buff):
        m128a = cls()

        m128a.Low = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)
        m128a.High = int.from_bytes(buff.read(8), byteorder = 'little', signed = True)

        return m128a

    @classmethod
    def parse_array(cls, buff, length):
        arr = []
        for i in range(length):
            arr.append(cls.parse(buff))
        return arr

    def __str__(self):
        s = ""
        s += "Low: %x (%d)" % (self.Low, self.Low)
        s += "High: %x (%d)\n" % (self.High, self.High)
        return s


# https://doxygen.reactos.org/df/d06/sdk_2include_2xdk_2arm_2ke_8h_source.html#l00229
class NEON128(M128A):
    # looks to be the same as M128A
    pass


# https://www.vergiliusproject.com/kernels/x64/Windows%20Vista%20%7C%202008/SP2/_XMM_SAVE_AREA32
class XMM_SAVE_AREA32:
    def __init__(self):
        self.ControlWord = 0                               # 0x0 USHORT
        self.StatusWord = 0                                # 0x2 USHORT
        self.TagWord = 0                                   # 0x4 UCHAR
        self.Reserved1 = 0                                 # 0x5 UCHAR
        self.ErrorOpcode = 0                               # 0x6 USHORT
        self.ErrorOffset = 0                               # 0x8 ULONG
        self.ErrorSelector = 0                             # 0xc USHORT
        self.Reserved2 = 0                                 # 0xe USHORT
        self.DataOffset = 0                                # 0x10 ULONG
        self.DataSelector = 0                              # 0x14 USHORT
        self.Reserved3 = 0                                 # 0x16 USHORT
        self.MxCsr = 0                                     # 0x18 ULONG
        self.MxCsr_Mask = 0                                # 0x1c ULONG
        self.FloatRegisters = []                           # 0x20 struct M128A[8]
        self.XmmRegisters = []                             # 0xa0 struct M128A[16]
        self.Reserved4 = []                                # 0x1a0 UCHAR[96]

    @classmethod
    def parse(cls, buff):
        xmm = cls()

        xmm.ControlWord = int.from_bytes(buff.read(2), byteorder = 'little', signed = False)
        xmm.StatusWord = int.from_bytes(buff.read(2), byteorder = 'little', signed = False)
        xmm.TagWord = chr(int.from_bytes(buff.read(1), byteorder = 'little', signed = False))
        xmm.Reserved1 = chr(int.from_bytes(buff.read(1), byteorder = 'little', signed = False))
        xmm.ErrorOpcode = int.from_bytes(buff.read(2), byteorder = 'little', signed = False)
        xmm.ErrorOffset = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
        xmm.ErrorSelector = int.from_bytes(buff.read(2), byteorder = 'little', signed = False)
        xmm.Reserved2 = int.from_bytes(buff.read(2), byteorder = 'little', signed = False)
        xmm.DataOffset = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
        xmm.DataSelector = int.from_bytes(buff.read(2), byteorder = 'little', signed = False)
        xmm.Reserved3 = int.from_bytes(buff.read(2), byteorder = 'little', signed = False)
        xmm.MxCsr = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
        xmm.MxCsr_Mask = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
        xmm.FloatRegisters = M128A.parse_array(buff, 8)
        xmm.XmmRegisters = M128A.parse_array(buff, 16)
        xmm.Reserved4 = [
            chr(int.from_bytes(buff.read(1), byteorder = 'little', signed = False))
            for i in range(96)
        ]

        return xmm

    def __str__(self):
        s = ""
        s += "%s: %x (%d)\n" % ("ControlWord", self.ControlWord, self.ControlWord)
        s += "%s: %x (%d)\n" % ("StatusWord", self.StatusWord, self.StatusWord)
        s += "%s: %s\n" % ("TagWord", self.TagWord)
        s += "%s: %s\n" % ("Reserved1", self.Reserved1)
        s += "%s: %x (%d)\n" % ("ErrorOpcode", self.ErrorOpcode, self.ErrorOpcode)
        s += "%s: %x (%d)\n" % ("ErrorOffset", self.ErrorOffset, self.ErrorOffset)
        s += "%s: %x (%d)\n" % ("ErrorSelector", self.ErrorSelector, self.ErrorSelector)
        s += "%s: %x (%d)\n" % ("Reserved2", self.Reserved2, self.Reserved2)
        s += "%s: %x (%d)\n" % ("DataOffset", self.DataOffset, self.DataOffset)
        s += "%s: %x (%d)\n" % ("DataSelector", self.DataSelector, self.DataSelector)
        s += "%s: %x (%d)\n" % ("Reserved3", self.Reserved3, self.Reserved3)
        s += "%s: %x (%d)\n" % ("MxCsr", self.MxCsr, self.MxCsr)
        s += "%s: %x (%d)\n" % ("MxCsr_Mask", self.MxCsr_Mask, self.MxCsr_Mask)
        s += "%s:\n" % ("FloatRegisters:")
        for freg in self.FloatRegisters:
            s += "\t%s" % (freg)
        s += "%s:\n" % ("XmmRegisters")
        for xreg in self.XmmRegisters:
            s += "\t%s" % (xreg)
        s += "%s: %s\n" % ("Reserved4", "".join(self.Reserved4))

        return s


class CTX_DUMMYSTRUCTNAME:
    def __init__(self):
        # all are M128A
        self.Header = []                # [2]
        self.Legacy = []                # [8]
        self.Xmm0 = 0
        self.Xmm1 = 0
        self.Xmm2 = 0
        self.Xmm3 = 0
        self.Xmm4 = 0
        self.Xmm5 = 0
        self.Xmm6 = 0
        self.Xmm7 = 0
        self.Xmm8 = 0
        self.Xmm9 = 0
        self.Xmm10 = 0
        self.Xmm11 = 0
        self.Xmm12 = 0
        self.Xmm13 = 0
        self.Xmm14 = 0
        self.Xmm15 = 0

    @classmethod
    def parse(cls, buff):
        dsn = cls()

        dsn.Header = M128A.parse_array(buff, 2)
        dsn.Legacy = M128A.parse_array(buff, 8)
        dsn.Xmm0 = M128A.parse(buff)
        dsn.Xmm1 = M128A.parse(buff)
        dsn.Xmm2 = M128A.parse(buff)
        dsn.Xmm3 = M128A.parse(buff)
        dsn.Xmm4 = M128A.parse(buff)
        dsn.Xmm5 = M128A.parse(buff)
        dsn.Xmm6 = M128A.parse(buff)
        dsn.Xmm7 = M128A.parse(buff)
        dsn.Xmm8 = M128A.parse(buff)
        dsn.Xmm9 = M128A.parse(buff)
        dsn.Xmm10 = M128A.parse(buff)
        dsn.Xmm11 = M128A.parse(buff)
        dsn.Xmm12 = M128A.parse(buff)
        dsn.Xmm13 = M128A.parse(buff)
        dsn.Xmm14 = M128A.parse(buff)
        dsn.Xmm15 = M128A.parse(buff)

        return dsn

    def __str__(self):
        s = ""
        s += "%s:\n" % ("Header")
        for head in self.Header:
            s += "\t%s" % (head)
        s += "%s:\n" % ("Legacy")
        for leg in self.Legacy:
            s += "\t%s" % (leg)
        s += "%s: %s" % ("Xmm0", self.Xmm0)
        s += "%s: %s" % ("Xmm1", self.Xmm1)
        s += "%s: %s" % ("Xmm2", self.Xmm2)
        s += "%s: %s" % ("Xmm3", self.Xmm3)
        s += "%s: %s" % ("Xmm4", self.Xmm4)
        s += "%s: %s" % ("Xmm5", self.Xmm5)
        s += "%s: %s" % ("Xmm6", self.Xmm6)
        s += "%s: %s" % ("Xmm7", self.Xmm7)
        s += "%s: %s" % ("Xmm8", self.Xmm8)
        s += "%s: %s" % ("Xmm9", self.Xmm9)
        s += "%s: %s" % ("Xmm10", self.Xmm10)
        s += "%s: %s" % ("Xmm11", self.Xmm11)
        s += "%s: %s" % ("Xmm12", self.Xmm12)
        s += "%s: %s" % ("Xmm13", self.Xmm13)
        s += "%s: %s" % ("Xmm14", self.Xmm14)
        s += "%s: %s" % ("Xmm15", self.Xmm15)

        return s


class CTX_DUMMYUNIONNAME:
    def __init__(self):
        self.FltSave = []                  # XMM_SAVE_AREA32
        self.Q = []                        # NEON128 [16]
        self.D = []                        # ULONGLONG [32]
        self.DUMMYSTRUCTNAME = []
        self.S = []                        # DWORD [32]

    @classmethod
    def parse(cls, buff):
        dun = cls()

        dun.FltSave = XMM_SAVE_AREA32.parse(buff)
        dun.Q = NEON128.parse_array(buff, 16)
        dun.D = [
            int.from_bytes(buff.read(8), byteorder = 'little', signed = False)
            for i in range(32)
        ]
        dun.DUMMYSTRUCTNAME = CTX_DUMMYSTRUCTNAME.parse(buff)
        dun.S = [
            int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
            for i in range(32)
        ]
        return dun

    def __str__(self):
        s = ""
        s += "%s: %s\n" % ("FltSave", self.FltSave)
        s += "%s:\n" % ("Q")
        for q in self.Q:
            s += "\t%s" % (q.__str__())
        for d in self.D:
            s += "\t%d" % (d)
        s += "%s: %s" % ("DUMMYSTRUCTNAME", self.DUMMYSTRUCTNAME)
        s += "%s:\n" %("S")
        for e in self.S:
            s += "\t%d" % (e)

        return s


# https:# docs.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-context
class CONTEXT:
    def __init__(self):
        self.P1Home = 0   # DWORD64
        self.P2Home = 0   # DWORD64
        self.P3Home = 0   # DWORD64
        self.P4Home = 0   # DWORD64
        self.P5Home = 0   # DWORD64
        self.P6Home = 0   # DWORD64
        self.ContextFlags = 0   # DWORD
        self.MxCsr = 0   # DWORD
        self.SegCs = 0   # WORD
        self.SegDs = 0   # WORD
        self.SegEs = 0   # WORD
        self.SegFs = 0   # WORD
        self.SegGs = 0   # WORD
        self.SegSs = 0   # WORD
        self.EFlags = 0   # DWORD
        self.Dr0 = 0   # DWORD64
        self.Dr1 = 0   # DWORD64
        self.Dr2 = 0   # DWORD64
        self.Dr3 = 0   # DWORD64
        self.Dr6 = 0   # DWORD64
        self.Dr7 = 0   # DWORD64
        self.Rax = 0   # DWORD64
        self.Rcx = 0   # DWORD64
        self.Rdx = 0   # DWORD64
        self.Rbx = 0   # DWORD64
        self.Rsp = 0   # DWORD64
        self.Rbp = 0   # DWORD64
        self.Rsi = 0   # DWORD64
        self.Rdi = 0   # DWORD64
        self.R8 = 0    # DWORD64
        self.R9 = 0    # DWORD64
        self.R10 = 0   # DWORD64
        self.R11 = 0   # DWORD64
        self.R12 = 0   # DWORD64
        self.R13 = 0   # DWORD64
        self.R14 = 0   # DWORD64
        self.R15 = 0   # DWORD64
        self.Rip = 0   # DWORD64
        self.DUMMYUNIONNAME = None

        self.VectorRegister = []         # M128A   [26]
        self.VectorControl = 0           # DWORD64
        self.DebugControl = 0            # DWORD64
        self.LastBranchToRip = 0         # DWORD64
        self.LastBranchFromRip = 0       # DWORD64
        self.LastExceptionToRip = 0      # DWORD64
        self.LastExceptionFromRip = 0    # DWORD64

    @classmethod
    def parse(cls, buff):
        ctx = cls()

        ctx.P1Home = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)   # DWORD64
        ctx.P2Home = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)   # DWORD64
        ctx.P3Home = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)   # DWORD64
        ctx.P4Home = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)   # DWORD64
        ctx.P5Home = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)   # DWORD64
        ctx.P6Home = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)   # DWORD64
        ctx.ContextFlags = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)   # DWORD
        ctx.MxCsr = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)   # DWORD
        ctx.SegCs = int.from_bytes(buff.read(2), byteorder = 'little', signed = False)   # WORD
        ctx.SegDs = int.from_bytes(buff.read(2), byteorder = 'little', signed = False)   # WORD
        ctx.SegEs = int.from_bytes(buff.read(2), byteorder = 'little', signed = False)   # WORD
        ctx.SegFs = int.from_bytes(buff.read(2), byteorder = 'little', signed = False)   # WORD
        ctx.SegGs = int.from_bytes(buff.read(2), byteorder = 'little', signed = False)   # WORD
        ctx.SegSs = int.from_bytes(buff.read(2), byteorder = 'little', signed = False)   # WORD
        ctx.EFlags = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)   # DWORD
        ctx.Dr0 = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)   # DWORD64
        ctx.Dr1 = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)   # DWORD64
        ctx.Dr2 = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)   # DWORD64
        ctx.Dr3 = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)   # DWORD64
        ctx.Dr6 = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)   # DWORD64
        ctx.Dr7 = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)   # DWORD64
        ctx.Rax = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)   # DWORD64
        ctx.Rcx = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)   # DWORD64
        ctx.Rdx = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)   # DWORD64
        ctx.Rbx = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)   # DWORD64
        ctx.Rsp = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)   # DWORD64
        ctx.Rbp = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)   # DWORD64
        ctx.Rsi = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)   # DWORD64
        ctx.Rdi = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)   # DWORD64
        ctx.R8 = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)   # DWORD64
        ctx.R9 = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)   # DWORD64
        ctx.R10 = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)   # DWORD64
        ctx.R11 = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)   # DWORD64
        ctx.R12 = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)   # DWORD64
        ctx.R13 = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)   # DWORD64
        ctx.R14 = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)   # DWORD64
        ctx.R15 = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)   # DWORD64
        ctx.Rip = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)   # DWORD64
        ctx.DUMMYUNIONNAME = CTX_DUMMYUNIONNAME.parse(buff)

        ctx.VectorRegister = M128A.parse_array(buff, 26)         # M128A   [26]
        ctx.VectorControl =  int.from_bytes(buff.read(8), byteorder = 'little', signed = False)       # DWORD64
        ctx.DebugControl = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)         # DWORD64
        ctx.LastBranchToRip = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)      # DWORD64
        ctx.LastBranchFromRip = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)    # DWORD64
        ctx.LastExceptionToRip = int.from_bytes(buff.read(8), byteorder = 'little', signed = False)   # DWORD64
        ctx.LastExceptionFromRip = int.from_bytes(buff.read(8), byteorder = 'little', signed = False) # DWORD64

        return ctx

    def __str__(self):
        s = ""
        s += "%s: 0x%x (%d)\n" % ("P1Home",self.P1Home,self.P1Home)
        s += "%s: 0x%x (%d)\n" % ("P2Home",self.P2Home,self.P2Home)
        s += "%s: 0x%x (%d)\n" % ("P3Home",self.P3Home,self.P3Home)
        s += "%s: 0x%x (%d)\n" % ("P4Home",self.P4Home,self.P4Home)
        s += "%s: 0x%x (%d)\n" % ("P5Home",self.P5Home,self.P5Home)
        s += "%s: 0x%x (%d)\n" % ("P6Home",self.P6Home,self.P6Home)
        s += "%s: 0x%x (%d)\n" % ("ContextFlags",self.ContextFlags,self.ContextFlags)
        s += "%s: 0x%x (%d)\n" % ("MxCsr",self.MxCsr,self.MxCsr)
        s += "%s: 0x%x (%d)\n" % ("SegCs",self.SegCs,self.SegCs)
        s += "%s: 0x%x (%d)\n" % ("SegDs",self.SegDs,self.SegDs)
        s += "%s: 0x%x (%d)\n" % ("SegEs",self.SegEs,self.SegEs)
        s += "%s: 0x%x (%d)\n" % ("SegFs",self.SegFs,self.SegFs)
        s += "%s: 0x%x (%d)\n" % ("SegGs",self.SegGs,self.SegGs)
        s += "%s: 0x%x (%d)\n" % ("SegSs",self.SegSs,self.SegSs)
        s += "%s: 0x%x (%d)\n" % ("EFlags",self.EFlags,self.EFlags)
        s += "%s: 0x%x (%d)\n" % ("Dr0",self.Dr0,self.Dr0)
        s += "%s: 0x%x (%d)\n" % ("Dr1",self.Dr1,self.Dr1)
        s += "%s: 0x%x (%d)\n" % ("Dr2",self.Dr2,self.Dr2)
        s += "%s: 0x%x (%d)\n" % ("Dr3",self.Dr3,self.Dr3)
        s += "%s: 0x%x (%d)\n" % ("Dr6",self.Dr6,self.Dr6)
        s += "%s: 0x%x (%d)\n" % ("Dr7",self.Dr7,self.Dr7)
        s += "%s: 0x%x (%d)\n" % ("Rax",self.Rax,self.Rax)
        s += "%s: 0x%x (%d)\n" % ("Rcx",self.Rcx,self.Rcx)
        s += "%s: 0x%x (%d)\n" % ("Rdx",self.Rdx,self.Rdx)
        s += "%s: 0x%x (%d)\n" % ("Rbx",self.Rbx,self.Rbx)
        s += "%s: 0x%x (%d)\n" % ("Rsp",self.Rsp,self.Rsp)
        s += "%s: 0x%x (%d)\n" % ("Rbp",self.Rbp,self.Rbp)
        s += "%s: 0x%x (%d)\n" % ("Rsi",self.Rsi,self.Rsi)
        s += "%s: 0x%x (%d)\n" % ("Rdi",self.Rdi,self.Rdi)
        s += "%s: 0x%x (%d)\n" % ("R8",self.R8,self.R8)
        s += "%s: 0x%x (%d)\n" % ("R9",self.R9,self.R9)
        s += "%s: 0x%x (%d)\n" % ("R10",self.R10,self.R10)
        s += "%s: 0x%x (%d)\n" % ("R11",self.R11,self.R11)
        s += "%s: 0x%x (%d)\n" % ("R12",self.R12,self.R12)
        s += "%s: 0x%x (%d)\n" % ("R13",self.R13,self.R13)
        s += "%s: 0x%x (%d)\n" % ("R14",self.R14,self.R14)
        s += "%s: 0x%x (%d)\n" % ("R15",self.R15,self.R15)
        s += "%s: 0x%x (%d)\n" % ("Rip",self.Rip,self.Rip)
        s += "%s:" % ("DUMMYUNIONNAME")
        s += self.DUMMYUNIONNAME.__str__()

        return s


# https:# docs.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-wow64_floating_save_area
class WOW64_FLOATING_SAVE_AREA:
    def __init__(self):
        self.ControlWord = 0  # DWORD
        self.StatusWord = 0   # DWORD
        self.TagWord = 0      # DWORD
        self.ErrorOffset = 0  # DWORD
        self.ErrorSelector = 0  # DWORD
        self.DataOffset = 0  # DWORD
        self.DataSelector = 0 # DWORD
        self.RegisterArea = []  # BYTE
        self.Cr0NpxState = 0  # DWORD

    @classmethod
    def parse(cls, buff):
        ctx = cls()
        ctx.ControlWord = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
        ctx.StatusWord = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
        ctx.TagWord = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
        ctx.ErrorOffset = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
        ctx.ErrorSelector = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
        ctx.DataOffset = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
        ctx.DataSelector = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
        ctx.RegisterArea = int.from_bytes(buff.read(80), byteorder = 'little', signed = False)
        ctx.Cr0NpxState = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
        return ctx

    def __str__(self):
        s = ''
        s += "ControlWord: %x (%d)\n" % (self.ControlWord, self.ControlWord)
        s += "StatusWord: %x (%d)\n" % (self.StatusWord, self.StatusWord)
        s += "TagWord: %x (%d)\n" % (self.TagWord, self.TagWord)
        s += "ErrorOffset: %x (%d)\n" % (self.ErrorOffset, self.ErrorOffset)
        s += "ErrorSelector: %x (%d)\n" % (self.ErrorSelector, self.ErrorSelector)
        s += "DataOffset: %x (%d)\n" % (self.DataOffset, self.DataOffset)
        s += "DataSelector: %x (%d)\n" % (self.DataSelector, self.DataSelector)
        s += "RegisterArea: %s\n" % str(self.RegisterArea)
        s += "Cr0NpxState: %x (%d)" % (self.Cr0NpxState, self.Cr0NpxState)
        return s

# https:# docs.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-wow64_context
class WOW64_CONTEXT:
    def __init__(self):
        self.ContextFlags = 0   # DWORD
        self.Dr0 = 0   # DWORD
        self.Dr1 = 0   # DWORD
        self.Dr2 = 0   # DWORD
        self.Dr3 = 0   # DWORD
        self.Dr6 = 0   # DWORD
        self.Dr7 = 0   # DWORD
        self.FloatSave = 0   # WOW64_FLOATING_SAVE_AREA
        self.SegGs = 0   # DWORD
        self.SegFs = 0   # DWORD
        self.SegEs = 0   # DWORD
        self.SegDs = 0   # DWORD
        self.Edi = 0   # DWORD
        self.Esi = 0   # DWORD
        self.Ebx = 0   # DWORD
        self.Edx = 0   # DWORD
        self.Ecx = 0   # DWORD
        self.Eax = 0   # DWORD
        self.Ebp = 0   # DWORD
        self.Eip = 0   # DWORD
        self.SegCs = 0   # DWORD
        self.EFlags = 0   # DWORD
        self.Esp = 0   # DWORD
        self.SegSs = 0   # DWORD
        self.ExtendedRegisters = []   # BYTE

    @classmethod
    def parse(cls, buff):
        ctx = cls()

        ctx.ContextFlags = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
        ctx.Dr0 = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
        ctx.Dr1 = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
        ctx.Dr2 = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
        ctx.Dr3 = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
        ctx.Dr6 = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
        ctx.Dr7 = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
        ctx.FloatSave = WOW64_FLOATING_SAVE_AREA.parse(buff)
        ctx.SegGs = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
        ctx.SegFs = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
        ctx.SegEs = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
        ctx.SegDs = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
        ctx.Edi = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
        ctx.Esi = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
        ctx.Ebx = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
        ctx.Edx = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
        ctx.Ecx = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
        ctx.Eax = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
        ctx.Ebp = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
        ctx.Eip = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
        ctx.SegCs = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
        ctx.EFlags = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
        ctx.Esp = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
        ctx.SegSs = int.from_bytes(buff.read(4), byteorder = 'little', signed = False)
        ctx.ExtendedRegisters = [
            int.from_bytes(buff.read(1), byteorder = 'little', signed = False)
            for i in range(512)
        ]
        return ctx

    def __str__(self):
        s = ''
        s += "%s: %x (%d)\n" % ("ContextFlags", self.ContextFlags, self.ContextFlags)
        s += "%s: %x (%d)\n" % ("Dr0", self.Dr0, self.Dr0)
        s += "%s: %x (%d)\n" % ("Dr1", self.Dr1, self.Dr1)
        s += "%s: %x (%d)\n" % ("Dr2", self.Dr2, self.Dr2)
        s += "%s: %x (%d)\n" % ("Dr3", self.Dr3, self.Dr3)
        s += "%s: %x (%d)\n" % ("Dr6", self.Dr6, self.Dr6)
        s += "%s: %x (%d)\n" % ("Dr7", self.Dr7, self.Dr7)
        s += "%s: %s\n" % ("FloatSave", self.FloatSave.__str__())
        s += "%s: %x (%d)\n" % ("SegGs", self.SegGs, self.SegGs)
        s += "%s: %x (%d)\n" % ("SegFs", self.SegFs, self.SegFs)
        s += "%s: %x (%d)\n" % ("SegEs", self.SegEs, self.SegEs)
        s += "%s: %x (%d)\n" % ("SegDs", self.SegDs, self.SegDs)
        s += "%s: %x (%d)\n" % ("Edi", self.Edi, self.Edi)
        s += "%s: %x (%d)\n" % ("Esi", self.Esi, self.Esi)
        s += "%s: %x (%d)\n" % ("Ebx", self.Ebx, self.Ebx)
        s += "%s: %x (%d)\n" % ("Edx", self.Edx, self.Edx)
        s += "%s: %x (%d)\n" % ("Ecx", self.Ecx, self.Ecx)
        s += "%s: %x (%d)\n" % ("Eax", self.Eax, self.Eax)
        s += "%s: %x (%d)\n" % ("Ebp", self.Ebp, self.Ebp)
        s += "%s: %x (%d)\n" % ("Eip", self.Eip, self.Eip)
        s += "%s: %x (%d)\n" % ("SegCs", self.SegCs, self.SegCs)
        s += "%s: %x (%d)\n" % ("EFlags", self.EFlags, self.EFlags)
        s += "%s: %x (%d)\n" % ("Esp", self.Esp, self.Esp)
        s += "%s: %x (%d)\n" % ("SegSs", self.SegSs, self.SegSs)
        s += "%s: %s\n" % ("ExtendedRegisters", str(self.ExtendedRegisters))

        return s
