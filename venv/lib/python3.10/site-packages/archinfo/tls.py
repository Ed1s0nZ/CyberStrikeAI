from collections import namedtuple

# REFERENCES:
# [1] https://www.uclibc.org/docs/tls.pdf
# [2] https://www.uclibc.org/docs/tls-ppc.txt
# [3] https://www.uclibc.org/docs/tls-ppc64.txt
# [4] https://www.linux-mips.org/wiki/NPTL

TLSArchInfo = namedtuple(
    "TLSArchInfo",
    ("variant", "tcbhead_size", "head_offsets", "dtv_offsets", "pthread_offsets", "tp_offset", "dtv_entry_offset"),
)
# variant: 1 or 2 based on the memory layout scheme
# tcbhead_size: the size of the thread control block head struct
# head_offsets: the offsets in the the tcb struct where a pointer to the head is kept
# dtv_offsets: the offsets in the tcb struct where a pointer to the dtv is kept
# pthread_offsets: the offsets in the tcb struct where a pointer to libc's pthread data is kept, I guess?
# tp_offset: the offset between the thread pointer from [1] and the thread pointer given to the program, see [2],[3]
# dtv_entry_offset: the offset between the value stored in the dtv and the actual start of the given dtv entry
