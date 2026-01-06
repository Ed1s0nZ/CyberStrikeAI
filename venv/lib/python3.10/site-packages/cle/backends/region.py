from __future__ import annotations


class Region:
    """
    A region of memory that is mapped in the object's file.

    :ivar offset:       The offset into the file the region starts.
    :ivar vaddr:        The virtual address.
    :ivar filesize:     The size of the region in the file.
    :ivar memsize:      The size of the region when loaded into memory.

    The prefix `v-` on a variable or parameter name indicates that it refers to the virtual, loaded memory space,
    while a corresponding variable without the `v-` refers to the flat zero-based memory of the file.

    When used next to each other, `addr` and `offset` refer to virtual memory address and file offset, respectively.
    """

    vaddr: int
    memsize: int
    filesize: int

    def __init__(self, offset, vaddr, filesize, memsize):
        self.vaddr = vaddr
        self.memsize = memsize
        self.filesize = filesize
        self.offset = offset

    def _rebase(self, delta):
        """
        Does region rebasing to other base address.
        Intended for usage by loader's add_object to reflect the rebasing.

        :param int delta: Delta offset between an old and a new image bases
        """
        self.vaddr += delta

    def contains_addr(self, addr):
        """
        Does this region contain this virtual address?
        """
        return self.vaddr <= addr < self.vaddr + self.memsize

    def contains_offset(self, offset):
        """
        Does this region contain this offset into the file?
        """
        return self.offset <= offset < self.offset + self.filesize

    def addr_to_offset(self, addr):
        """
        Convert a virtual memory address into a file offset
        """
        offset = addr - self.vaddr + self.offset
        if not self.contains_offset(offset):
            return None
        return offset

    def offset_to_addr(self, offset):
        """
        Convert a file offset into a virtual memory address
        """
        addr = offset - self.offset + self.vaddr
        if not self.contains_addr(addr):
            return None
        return addr

    def __repr__(self):
        return "<{} {}>".format(
            self.__class__.__name__,
            ", ".join([f"{k}={hex(v) if v is not None else 'None'}" for k, v in self.__dict__.items()]),
        )

    @property
    def max_addr(self):
        """
        The maximum virtual address of this region
        """
        return self.vaddr + self.memsize - 1

    @property
    def min_addr(self):
        """
        The minimum virtual address of this region
        """
        return self.vaddr

    @property
    def max_offset(self):
        """
        The maximum file offset of this region
        """
        return self.offset + self.filesize - 1

    def min_offset(self):
        """
        The minimum file offset of this region
        """
        return self.offset

    # EDG says: Blobs now have segments, and SimOS will get upset if these don't exist.  See simos.py line 107 for
    # some code you should probably fix if you don't like it.
    @property
    def is_readable(self) -> bool:
        # pylint: disable=no-self-use
        return True

    @property
    def is_writable(self) -> bool:
        # pylint: disable=no-self-use
        return True

    @property
    def is_executable(self) -> bool:
        # pylint: disable=no-self-use
        return True


class Segment(Region):
    pass


class EmptySegment(Segment):
    """
    A segment with no static content, and permissions
    """

    def __init__(self, vaddr, memsize, is_readable=True, is_writable=True, is_executable=False):
        super().__init__(0, vaddr, 0, memsize)
        self._is_readable = is_readable
        self._is_writable = is_writable
        self._is_executable = is_executable

    @property
    def is_executable(self):
        return self._is_executable

    @property
    def is_writable(self):
        return self._is_writable

    @property
    def is_readable(self):
        return self._is_readable

    @property
    def only_contains_uninitialized_data(self):
        """
        Whether this section is initialized to zero after the executable is loaded.
        """
        return True


class Section(Region):
    """
    Simple representation of a loaded section.

    :ivar str name:     The name of the section
    """

    def __init__(self, name, offset, vaddr, size):
        """
        :param str name:    The name of the section
        :param int offset:  The offset into the binary file this section begins
        :param int vaddr:   The address in virtual memory this section begins
        :param int size:    How large this section is
        """
        super().__init__(offset, vaddr, size, size)
        self.name = name

    @property
    def is_readable(self):
        """
        Whether this section has read permissions
        """
        raise NotImplementedError()

    @property
    def is_writable(self):
        """
        Whether this section has write permissions
        """
        raise NotImplementedError()

    @property
    def is_executable(self):
        """
        Whether this section has execute permissions
        """
        raise NotImplementedError()

    @property
    def only_contains_uninitialized_data(self):
        """
        Whether this section is initialized to zero after the executable is loaded.
        """
        raise NotImplementedError()

    def __repr__(self):
        return "<{} | offset {:#x}, vaddr {:#x}, size {:#x}>".format(
            self.name if self.name else "Unnamed", self.offset, self.vaddr, self.memsize
        )
