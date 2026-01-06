from __future__ import annotations


class PatchedStream:
    """
    An object that wraps a readable stream, performing passthroughs on seek and read operations,
    except to make it seem like the data has actually been patched by the given patches.
    """

    def __init__(self, stream, patches):
        """
        :param stream:      The stream to patch
        :param patches:     A list of tuples of (addr, patch data)
        """
        if type(stream) is PatchedStream:
            patches = stream.patches + patches
            stream = stream.stream

        self.stream = stream
        self.patches = patches
        self._pos = stream.tell()

    def read(self, *args, **kwargs):
        data = self.stream.read(*args, **kwargs)
        pos = self._pos
        newpos = pos + len(data)

        # AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
        for addr, patch in self.patches:
            if (addr >= self._pos and addr < newpos) or (self._pos >= addr and self._pos < addr + len(patch)):
                inject_start = max(0, addr - pos)
                inject_end = min(addr - pos + len(patch), len(data))
                patch_start = max(0, pos - addr)
                patch_end = min(newpos - addr, len(patch))
                data = data[:inject_start] + patch[patch_start:patch_end] + data[inject_end:]

        self._pos = newpos
        return data

    def seek(self, *args, **kwargs):
        self.stream.seek(*args, **kwargs)
        self._pos = self.stream.tell()

    def tell(self):
        return self._pos

    def close(self):
        return self.stream.close()
