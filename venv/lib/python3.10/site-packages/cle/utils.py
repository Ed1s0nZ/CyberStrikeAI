from __future__ import annotations

import contextlib
import os

import elftools

from .errors import CLEError, CLEFileNotFoundError


# https://code.woboq.org/userspace/glibc/include/libc-pointer-arith.h.html#43
def ALIGN_DOWN(base, size):
    return base & -size


# https://code.woboq.org/userspace/glibc/include/libc-pointer-arith.h.html#50
def ALIGN_UP(base, size):
    return ALIGN_DOWN(base + size - 1, size)


# To verify the mmap behavior you can compile and run the following program. Fact is that mmap file mappings
# always map in the entire page into memory from the file if available. If not, it gets zero padded
# pylint: disable=pointless-string-statement
# #include <stdio.h>
# #include <sys/types.h>
# #include <sys/stat.h>
# #include <unistd.h>
# #include <fcntl.h>
# #include <sys/mman.h>
#
# void make_test_file()
# {
#     void* data = (void*)0xdead0000;
#     int fd = open("./test.data", O_CREAT | O_RDWR, S_IRUSR | S_IWUSR);
#     for (int i = 0; i < 0x1800; i += sizeof(void*)) // Only write 1 1/2 pages worth
#     {
#         write(fd, &data, sizeof(void*));
#         data += sizeof(void*);
#     }
#     close(fd);
# }
# int main(int argc, char* argv[])
# {
#     make_test_file();
#
#     int fd = open("./test.data", O_RDONLY);
#     unsigned char* mapping = mmap(NULL, 0x123, PROT_READ, MAP_PRIVATE, fd, 4096);
#
#     for (int i=0; i < 0x1000; i++)
#     {
#         printf("%02x ", mapping[i]);
#         if (i % sizeof(void*) == (sizeof(void*) - 1))
#             printf("| ");
#         if (i % 16 == 15)
#             printf("\n");
#     }
# }


def get_mmaped_data(stream, offset, length, page_size):
    if offset % page_size != 0:
        raise CLEError(
            "libc helper for mmap: Invalid page offset, should be multiple of page size! "
            f"Stream {stream}, offset {offset}, length: {length}"
        )

    read_length = ALIGN_UP(length, page_size)
    stream.seek(offset)
    data = stream.read(read_length)
    return data.ljust(read_length, b"\0")


@contextlib.contextmanager
def stream_or_path(obj, perms="rb"):
    if hasattr(obj, "read") and hasattr(obj, "seek"):
        obj.seek(0)
        yield obj
    else:
        if not os.path.exists(obj):
            raise CLEFileNotFoundError(f"{obj!r} is not a valid path")

        with open(obj, perms) as f:
            yield f


def key_bisect_floor_key(lst, key, lo=0, hi=None, keyfunc=lambda x: x):
    if lo < 0:
        raise ValueError("lo must be non-negative")
    if hi is None:
        hi = len(lst)
    while lo < hi:
        mid = (lo + hi) // 2
        if keyfunc(lst[mid]) <= key:
            lo = mid + 1
        else:
            hi = mid
    if lo <= len(lst) and lo > 0:
        return lst[lo - 1]
    return None


def key_bisect_find(lst, item, lo=0, hi=None, keyfunc=lambda x: x):
    if lo < 0:
        raise ValueError("lo must be non-negative")
    if hi is None:
        hi = len(lst)
    while lo < hi:
        mid = (lo + hi) // 2
        if keyfunc(lst[mid]) <= keyfunc(item):
            lo = mid + 1
        else:
            hi = mid
    return lo


def key_bisect_insort_left(lst, item, lo=0, hi=None, keyfunc=lambda x: x):
    if lo < 0:
        raise ValueError("lo must be non-negative")
    if hi is None:
        hi = len(lst)
    while lo < hi:
        mid = (lo + hi) // 2
        if keyfunc(lst[mid]) < keyfunc(item):
            lo = mid + 1
        else:
            hi = mid
    lst.insert(lo, item)


def key_bisect_insort_right(lst, item, lo=0, hi=None, keyfunc=lambda x: x):
    if lo < 0:
        raise ValueError("lo must be non-negative")
    if hi is None:
        hi = len(lst)
    while lo < hi:
        mid = (lo + hi) // 2
        if keyfunc(lst[mid]) <= keyfunc(item):
            lo = mid + 1
        else:
            hi = mid
    lst.insert(lo, item)


def get_text_offset(path):
    """
    Offset of text section in the binary.
    """
    with stream_or_path(path) as f:
        e = elftools.elf.elffile.ELFFile(f)
        return e.get_section_by_name(".text").header.sh_offset


def extract_null_terminated_bytestr(data: bytes | bytearray, offset: int = 0, sentinel_value: bytes = b"\x00") -> bytes:
    """
    Return an exclusive null-terminated sequence of bytes at `offset` in `data`.
    """
    return bytes(data[offset : data.find(sentinel_value, offset)])
