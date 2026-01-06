import io
import os
from typing import AsyncIterable, BinaryIO, Union


class Peeker:
    def __init__(self, fileobj: BinaryIO):
        self.fileobj = fileobj
        self.buf = io.BytesIO()

    def _append_to_buf(self, contents: io.BytesIO):
        oldpos = self.buf.tell()
        self.buf.seek(0, os.SEEK_END)
        self.buf.write(contents)
        self.buf.seek(oldpos)

    def peek(self, size: int) -> bytes:
        contents = self.fileobj.read(size)
        self._append_to_buf(contents)
        return contents

    def read(self, size: Union[int, None] = None) -> bytes:
        if size is None:
            return self.buf.read() + self.fileobj.read()
        contents = self.buf.read(size)
        if len(contents) < size:
            contents += self.fileobj.read(size - len(contents))
        return contents

    def readline(self) -> bytes:
        line = self.buf.readline()
        if not line.endswith(b"\n"):
            line += self.fileobj.readline()
        return line


class AsyncReader:
    """Wraps an asynchronous stream to allow reading a fixed number of bytes at a time."""

    def __init__(self, async_stream: AsyncIterable):
        self.async_stream = async_stream
        # bytes left over from last read.
        self._left_over_bytes: bytes = b""

    async def read(self, bytes_to_read: int = 1) -> Union[bytes, None]:
        """Read the requested number of bytes or to the and of the async iterable."""
        if self._left_over_bytes is None:
            return None

        read_bytes: list[bytes] = [self._left_over_bytes]
        bytes_read_count = 0
        async for d_chunk in self.async_stream:
            read_bytes.append(d_chunk)
            bytes_read_count += len(d_chunk)
            if bytes_read_count >= bytes_to_read:
                break

        ret_bytes = b"".join(read_bytes)
        self._left_over_bytes = ret_bytes[bytes_to_read:]

        # Iterable has been exhausted return None
        if len(ret_bytes) == 0:
            self._left_over_bytes = None
            return None
        return ret_bytes[:bytes_to_read]
