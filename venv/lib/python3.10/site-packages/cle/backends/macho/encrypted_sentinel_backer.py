from __future__ import annotations

from cle.memory import Clemory


class CryptSentinel(Clemory):
    """
    Mach-O binaries are often encrypted, and some area of memory is only decrypted at runtime later in the loading
    process. This decryption process can't easily be implemented in CLE and is typically done with separate tools
    But not all data is encrypted, and various metadata is still accessible.

    This Clemory serves as a shim that allows us to notice accesses to encrypted areas of memory and raise an exception
    This means that all code that was written will loudly fail on access to encrypted memory, instead of silently
    reading garbage data.
    """

    def __init__(self, arch, root=False):
        super().__init__(arch, root)
        self._crypt_start = None
        self._crypt_end = None
        self._is_encrypted: bool = False

    def load(self, addr, n):
        self._assert_unencrypted_access(addr, n)
        return super().load(addr, n)

    def store(self, addr, data):
        self._assert_unencrypted_access(addr, len(data))
        return super().store(addr, data)

    def find(self, data, search_min=None, search_max=None):
        if self._is_encrypted:
            raise EncryptedDataAccessException("Cannot search encrypted memory region", self._crypt_start)
        return super().find(data, search_min, search_max)

    def set_crypt_info(self, cryptid, start, size):
        self._is_encrypted = cryptid != 0 and size > 0
        self._crypt_start = start
        self._crypt_end = start + size

    def backers(self, addr=0):
        if self._is_encrypted:
            if self._crypt_start <= addr < self._crypt_end:
                raise EncryptedDataAccessException("Accessing encrypted memory region", addr)
        return super().backers(addr)

    def _assert_unencrypted_access(self, addr, size):
        """
        Make sure that the access does not cover encrypted memory regions
        If it does, raise an error

        Cases:
        - Access starts before encrypted region and ends after it
        - Access starts within encrypted region
        - Access ends within encrypted region

        :param addr:
        :param size:
        :return:
        """
        if not self._is_encrypted:
            return

        encrypted_range = range(self._crypt_start, self._crypt_end)
        if addr in encrypted_range or (addr + size) in encrypted_range or (addr < self._crypt_start < addr + size):
            raise EncryptedDataAccessException("Accessing encrypted memory region", addr)


class EncryptedDataAccessException(Exception):
    """
    Special exception to be raised when access to encrypted memory is attempted
    """

    def __init__(self, message, addr):
        super().__init__(message)
        self.addr = addr
