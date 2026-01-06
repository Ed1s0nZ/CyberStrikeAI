# Copyright (c) 2008 - 2025, Ilan Schnell; All Rights Reserved
# bitarray is published under the PSF license.
#
# Author: Ilan Schnell
"""
Tests for bitarray

Author: Ilan Schnell
"""
import re
import os
import sys
import platform
import unittest
import shutil
import tempfile
from io import BytesIO, UnsupportedOperation
from random import choice, choices, getrandbits, randrange, randint, shuffle
from string import whitespace

# imports needed inside tests
import array
import copy
import itertools
import mmap
import pickle
import shelve
import weakref


pyodide = bool(platform.machine() == 'wasm32')
is_pypy = bool(platform.python_implementation() == 'PyPy')


from bitarray import (bitarray, frozenbitarray, bits2bytes, decodetree,
                      get_default_endian,
                      _bitarray_reconstructor, _sysinfo as sysinfo,
                      BufferInfo, __version__)

def skipIf(condition):
    "Skip a test if the condition is true."
    if condition:
        return lambda f: None
    return lambda f: f

PTRSIZE = sysinfo("void*")  # pointer size in bytes

# avoid importing from bitarray.util
zeros = bitarray

def ones(n, endian=None):
    a = bitarray(n, endian)
    a.setall(1)
    return a

def urandom_2(n, endian="<RAND>"):
    if endian == "<RAND>":
        endian = choice(['little', 'big'])
    a = bitarray(os.urandom(bits2bytes(n)), endian)
    del a[n:]
    return a


class Util(object):

    @staticmethod
    def random_endian():
        return choice(['little', 'big'])

    @staticmethod
    def randombitarrays(start=0):
        for n in range(start, 10):
            yield urandom_2(n)
        for _ in range(3):
            yield urandom_2(randrange(start, 1000))

    def randomlists(self):
        for a in self.randombitarrays():
            yield a.tolist()

    @staticmethod
    def opposite_endian(endian):
        t = {'little': 'big',
             'big': 'little'}
        return t[endian]

    @staticmethod
    def random_slice(n, step=None):
        return slice(randint(-n - 2, n + 2),
                     randint(-n - 2, n + 2),
                     step or randint(-5, 5) or 1)

    def check_obj(self, a):
        self.assertIsInstance(a, bitarray)

        self.assertEqual(a.nbytes, bits2bytes(len(a)))
        self.assertTrue(0 <= a.padbits < 8)
        self.assertEqual(len(a) + a.padbits, 8 * a.nbytes)

        info = a.buffer_info()
        if info.imported:
            # imported buffer implies that no extra memory is allocated
            self.assertEqual(info.alloc, 0)
            # an imported buffer will always have a multiple of 8 bits
            self.assertEqual(len(a), 8 * a.nbytes)
            self.assertEqual(a.padbits, 0)
        else:
            # the allocated memory is always larger than the buffer size
            self.assertTrue(info.alloc >= a.nbytes)

        if info.address == 0:
            # the buffer being a NULL pointer implies that the buffer size
            # and the allocated memory size are 0
            self.assertEqual(a.nbytes, 0)
            self.assertEqual(info.alloc, 0)

        if type(a) == frozenbitarray:
            # frozenbitarray have read-only memory
            self.assertEqual(a.readonly, 1)
            if a.padbits:  # ensure padbits are zero
                b = bitarray(bytes(a)[-1:], endian=a.endian)[-a.padbits:]
                self.assertEqual(len(b), a.padbits)
                self.assertEqual(b.count(), 0)
        elif not info.imported:
            # otherwise, unless the buffer is imported, it is writable
            self.assertFalse(a.readonly)

    def assertEQUAL(self, a, b):
        self.assertEqual(a, b)
        self.assertEqual(a.endian, b.endian)

    def assertIsType(self, a, b):
        self.assertEqual(type(a).__name__, b)
        self.assertEqual(repr(type(a)), "<class 'bitarray.%s'>" % b)

    @staticmethod
    def assertRaisesMessage(excClass, msg, callable, *args, **kwargs):
        try:
            callable(*args, **kwargs)
            raise AssertionError("%s not raised" % excClass.__name__)
        except excClass as e:
            if msg != str(e):
                raise AssertionError("message: %s\n got: %s" % (msg, e))

# --------------------------  Module Functions  -----------------------------

class ModuleFunctionsTests(unittest.TestCase):

    def test_version_string(self):
        # the version string is not a function, but test it here anyway
        self.assertEqual(type(__version__), str)

    def test_sysinfo(self):
        for key in ["void*", "size_t", "bitarrayobject", "decodetreeobject",
                    "binode", "HAVE_BUILTIN_BSWAP64",
                    "PY_LITTLE_ENDIAN", "PY_BIG_ENDIAN",
                    "Py_GIL_DISABLED", "Py_DEBUG", "DEBUG"]:
            res = sysinfo(key)
            self.assertEqual(type(res), int)

    @skipIf(sys.version_info[:2] < (3, 14))
    def test_gil_disabled(self):
        self.assertEqual(sysinfo("Py_GIL_DISABLED"),
                         "free-threading" in sys.version)
        # see if GIL is actually disabled in free-threading build
        if sysinfo("Py_GIL_DISABLED"):
            self.assertFalse(sys._is_gil_enabled())

    def test_sysinfo_errors(self):
        self.assertRaises(TypeError, sysinfo)
        self.assertRaises(TypeError, sysinfo, b"void*")
        self.assertRaises(KeyError, sysinfo, "foo")

    def test_sysinfo_pointer_size(self):
        self.assertEqual(sysinfo("void*"), PTRSIZE)
        self.assertEqual(sysinfo("size_t"), PTRSIZE)
        self.assertEqual(sys.maxsize, 2 ** (8 * PTRSIZE - 1) - 1)
        if not is_pypy:  # PyPy doesn't have tuple.__itemsize__
            self.assertEqual(PTRSIZE, tuple.__itemsize__)

    def test_sysinfo_byteorder(self):
        self.assertEqual(sys.byteorder == "little",
                         sysinfo("PY_LITTLE_ENDIAN"))
        self.assertEqual(sys.byteorder == "big",
                         sysinfo("PY_BIG_ENDIAN"))

    def test_get_default_endian(self):
        endian = get_default_endian()
        self.assertTrue(endian in ('little', 'big'))
        self.assertEqual(type(endian), str)
        a = bitarray()
        self.assertEqual(a.endian, endian)

    def test_get_default_endian_errors(self):
        # takes no arguments
        self.assertRaises(TypeError, get_default_endian, 'big')

    def test_bits2bytes(self):
        for n, res in (0, 0), (1, 1), (7, 1), (8, 1), (9, 2):
            self.assertEqual(bits2bytes(n), res)

        for n in range(1, 200):
            m = bits2bytes(n)
            self.assertEqual(m, (n - 1) // 8 + 1)
            self.assertEqual(type(m), int)

            k = (1 << n) + randrange(1000)
            self.assertEqual(bits2bytes(k), (k - 1) // 8 + 1)

    def test_bits2bytes_errors(self):
        for arg in 'foo', [], None, {}, 187.0, -4.0:
            self.assertRaises(TypeError, bits2bytes, arg)

        self.assertRaises(TypeError, bits2bytes)
        self.assertRaises(TypeError, bits2bytes, 1, 2)

        self.assertRaises(ValueError, bits2bytes, -1)
        self.assertRaises(ValueError, bits2bytes, -924)

# ---------------------------------------------------------------------------

class CreateObjectTests(unittest.TestCase, Util):

    def test_noInitializer(self):
        a = bitarray()
        self.assertEqual(len(a), 0)
        self.assertEqual(a.tolist(), [])
        self.assertEqual(type(a), bitarray)
        self.check_obj(a)

    def test_endian(self):
        a = bitarray(b"ABC", endian='little')
        self.assertEqual(a.endian, 'little')
        self.assertEqual(type(a.endian), str)
        self.check_obj(a)

        b = bitarray(b"ABC", endian='big')
        self.assertEqual(b.endian, 'big')
        self.assertEqual(type(a.endian), str)
        self.check_obj(b)

        self.assertNotEqual(a, b)
        self.assertEqual(a.tobytes(), b.tobytes())

    def test_endian_wrong(self):
        self.assertRaises(TypeError, bitarray, endian=0)
        self.assertRaises(ValueError, bitarray, endian='')
        self.assertRaisesMessage(
            ValueError,
            "bit-endianness must be either 'little' or 'big', not 'foo'",
            bitarray, endian='foo')
        self.assertRaisesMessage(TypeError,
                                 "'ellipsis' object is not iterable",
                                 bitarray, Ellipsis)

    def test_buffer_endian(self):
        for endian in 'big', 'little':
            a = bitarray(buffer=b'', endian=endian)
            self.assertEQUAL(a, bitarray(0, endian))

        a = bitarray(buffer=b'A')
        self.assertEqual(a.endian, "big")
        self.assertEqual(len(a), 8)

    def test_buffer_readonly(self):
        a = bitarray(buffer=b'\xf0', endian='little')
        self.assertTrue(a.readonly)
        self.assertRaises(TypeError, a.clear)
        self.assertRaises(TypeError, a.__setitem__, 3, 1)
        self.assertEQUAL(a, bitarray('00001111', 'little'))
        self.check_obj(a)

    @skipIf(is_pypy)
    def test_buffer_writable(self):
        a = bitarray(buffer=bytearray([65]))
        self.assertFalse(a.readonly)
        a[6] = 1

    def test_buffer_args(self):
        # buffer requires no initial argument
        self.assertRaises(TypeError, bitarray, 5, buffer=b'DATA\0')

        # positinal arguments
        a = bitarray(None, 'big', bytearray([15]))
        self.assertEQUAL(a, bitarray('00001111', 'big'))
        a = bitarray(None, 'little', None)
        self.assertEQUAL(a, bitarray(0, 'little'))

    def test_none(self):
        for a in [bitarray(None),
                  bitarray(None, buffer=None),
                  bitarray(None, buffer=Ellipsis),
                  bitarray(None, None, None),
                  bitarray(None, None, Ellipsis)]:
            self.assertEqual(len(a), 0)

    def test_int(self):
        for n in range(50):
            a = bitarray(n)
            self.assertEqual(len(a), n)
            self.assertFalse(a.any())
            self.assertEqual(a.to01(), n * '0')
            self.check_obj(a)

            # uninitialized buffer
            a = bitarray(n, buffer=Ellipsis)
            self.assertEqual(len(a), n)
            self.check_obj(a)

        self.assertRaises(ValueError, bitarray, -1)
        self.assertRaises(ValueError, bitarray, -924)

    def test_list(self):
        a = bitarray([0, 1, False, True])
        self.assertEqual(a, bitarray('0101'))
        self.check_obj(a)

        self.assertRaises(ValueError, bitarray, [0, 1, 2])
        self.assertRaises(TypeError, bitarray, [0, 1, None])

        for n in range(50):
            lst = [bool(getrandbits(1)) for d in range(n)]
            a = bitarray(lst)
            self.assertEqual(a.tolist(), lst)
            self.check_obj(a)

    def test_sequence(self):
        lst = [0, 1, 1, 1, 0]
        for x in lst, tuple(lst), array.array('i', lst):
            self.assertEqual(len(x), 5)  # sequences have len, iterables not
            a = bitarray(x)
            self.assertEqual(a, bitarray('01110'))
            self.check_obj(a)

    def test_iter1(self):
        for n in range(50):
            lst = [bool(getrandbits(1)) for d in range(n)]
            a = bitarray(iter(lst))
            self.assertEqual(a.tolist(), lst)
            self.check_obj(a)

    def test_iter2(self):
        for lst in self.randomlists():
            def foo():
                for x in lst:
                    yield x
            a = bitarray(foo())
            self.assertEqual(a, bitarray(lst))
            self.check_obj(a)

    def test_iter3(self):
        a = bitarray(itertools.repeat(False, 10))
        self.assertEqual(a, zeros(10))
        a = bitarray(itertools.repeat(1, 10))
        self.assertEqual(a, bitarray(10 * '1'))

    def test_range(self):
        self.assertEqual(bitarray(range(2)), bitarray('01'))
        self.assertRaises(ValueError, bitarray, range(0, 3))

    def test_string01(self):
        for s in '0010111', '0010 111', '0010_111':
            a = bitarray(s)
            self.assertEqual(a.tolist(), [0, 0, 1, 0, 1, 1, 1])
            self.check_obj(a)

        for lst in self.randomlists():
            s = ''.join([['0', '1'][x] for x in lst])
            a = bitarray(s)
            self.assertEqual(a.tolist(), lst)
            self.check_obj(a)

        self.assertRaises(ValueError, bitarray, '01021')        # UCS1
        self.assertRaises(ValueError, bitarray, '1\u26050')     # UCS2
        self.assertRaises(ValueError, bitarray, '0\U00010348')  # UCS4

    def test_string01_whitespace(self):
        a = bitarray(whitespace)
        self.assertEqual(a, bitarray())

        for c in whitespace:
            a = bitarray(c + '1101110001')
            self.assertEqual(a, bitarray('1101110001'))

        a = bitarray(' 0\n1\r0\t1\v0 ')
        self.assertEqual(a, bitarray('01010'))

    def test_bytes_bytearray(self):
        for x in b'\x80', bytearray(b'\x80'):
            a = bitarray(x, 'little')
            self.assertEqual(len(a), 8 * len(x))
            self.assertEqual(a.tobytes(), x)
            self.assertEqual(a.to01(), '00000001')
            self.check_obj(a)

        for n in range(100):
            x = os.urandom(n)
            a = bitarray(x)
            self.assertEqual(len(a), 8 * n)
            self.assertEqual(memoryview(a), x)

    def test_byte(self):
        for byte, endian, res in [
                (b'\x01', 'little', '10000000'),
                (b'\x80', 'little', '00000001'),
                (b'\x80', 'big',    '10000000'),
                (b'\x01', 'big',    '00000001')]:
            a = bitarray(byte, endian)
            self.assertEqual(a.to01(), res)

    def test_bitarray_simple(self):
        for n in range(10):
            a = bitarray(n)
            b = bitarray(a, endian=None)
            self.assertFalse(a is b)
            self.assertEQUAL(a, b)

    def test_bitarray_endian(self):
        # Test creating a new bitarray with different endianness from an
        # existing bitarray.
        for endian in 'little', 'big':
            a = bitarray(endian=endian)
            b = bitarray(a)
            self.assertFalse(a is b)
            self.assertEQUAL(a, b)

            endian2 = self.opposite_endian(endian)
            b = bitarray(a, endian2)
            self.assertEqual(b.endian, endian2)
            self.assertEqual(a, b)

        for a in self.randombitarrays():
            endian2 = self.opposite_endian(a.endian)
            b = bitarray(a, endian2)
            self.assertEqual(a, b)
            self.assertEqual(b.endian, endian2)
            self.assertNotEqual(a.endian, b.endian)

    def test_bitarray_endianness(self):
        a = bitarray('11100001', endian='little')
        b = bitarray(a, endian='big')
        self.assertEqual(a, b)
        self.assertNotEqual(a.tobytes(), b.tobytes())

        b.bytereverse()
        self.assertNotEqual(a, b)
        self.assertEqual(a.tobytes(), b.tobytes())

        c = bitarray('11100001', endian='big')
        self.assertEqual(a, c)

    def test_frozenbitarray(self):
        a = bitarray(frozenbitarray())
        self.assertEQUAL(a, bitarray())
        self.assertEqual(type(a), bitarray)

        for endian in 'little', 'big':
            a = bitarray(frozenbitarray('011', endian=endian))
            self.assertEQUAL(a, bitarray('011', endian))
            self.assertEqual(type(a), bitarray)

    def test_create_empty(self):
        for x in (None, 0, '', list(), tuple(), set(), dict(), bytes(),
                  bytearray(), bitarray(), frozenbitarray()):
            a = bitarray(x)
            self.assertEqual(len(a), 0)
            self.assertEQUAL(a, bitarray())

    def test_wrong_args(self):
        # wrong types
        for x in False, True, Ellipsis, slice(0), 0.0, 0 + 0j:
            self.assertRaises(TypeError, bitarray, x)
        # wrong values
        for x in -1, 'A', '\0', '010\0 11':
            self.assertRaises(ValueError, bitarray, x)
        # test second (endian) argument
        self.assertRaises(TypeError, bitarray, 0, 0)
        self.assertRaises(ValueError, bitarray, 0, 'foo')
        # too many args
        self.assertRaises(TypeError, bitarray, 0, 'big', 0)

    @skipIf(is_pypy)
    def test_weakref(self):
        a = bitarray('0100')
        b = weakref.proxy(a)
        self.assertEqual(b.to01(), a.to01())
        a = None
        self.assertRaises(ReferenceError, len, b)

# ---------------------------------------------------------------------------

class ToObjectsTests(unittest.TestCase, Util):

    def test_numeric(self):
        a = bitarray()
        self.assertRaises(Exception, int, a)
        self.assertRaises(Exception, float, a)
        self.assertRaises(Exception, complex, a)

    def test_list(self):
        for a in self.randombitarrays():
            self.assertEqual(list(a), a.tolist())

    def test_tuple(self):
        for a in self.randombitarrays():
            self.assertEqual(tuple(a), tuple(a.tolist()))

    def test_bytes_bytearray(self):
        for n in range(20):
            a = urandom_2(8 * n)
            self.assertEqual(a.padbits, 0)
            self.assertEqual(bytes(a), a.tobytes())
            self.assertEqual(bytearray(a), a.tobytes())

    def test_bytes_bytearray_pad(self):
        for endian, res in ("little", b"\x01"), ("big", b"\x80"):
            a = bitarray("1", endian)
            self.assertEqual(a.tobytes(), res)
            a = bitarray("1", endian)
            self.assertEqual(bytes(a), res)

    def test_set(self):
        for a in self.randombitarrays():
            self.assertEqual(set(a), set(a.tolist()))

# -------------------------- (Number) index tests ---------------------------

class GetItemTests(unittest.TestCase, Util):

    def test_explicit(self):
        a = bitarray()
        self.assertRaises(IndexError, a.__getitem__,  0)
        a.append(True)
        self.assertEqual(a[0], 1)
        self.assertEqual(a[-1], 1)
        self.assertRaises(IndexError, a.__getitem__,  1)
        self.assertRaises(IndexError, a.__getitem__, -2)
        a.append(False)
        self.assertEqual(a[1], 0)
        self.assertEqual(a[-1], 0)
        self.assertRaises(IndexError, a.__getitem__,  2)
        self.assertRaises(IndexError, a.__getitem__, -3)
        self.assertRaises(TypeError, a.__getitem__, 1.5)
        self.assertRaises(TypeError, a.__getitem__, None)
        self.assertRaises(TypeError, a.__getitem__, 'A')

    def test_basic(self):
        a = bitarray('1100010')
        for i, v in enumerate(a):
            self.assertEqual(a[i], v)
            self.assertEqual(type(a[i]), int)
            self.assertEqual(a[-7 + i], v)
        self.assertRaises(IndexError, a.__getitem__,  7)
        self.assertRaises(IndexError, a.__getitem__, -8)

    def test_range(self):
        for a in self.randombitarrays():
            aa = a.tolist()
            for i in range(len(a)):
                self.assertEqual(a[i], aa[i])

class SetItemTests(unittest.TestCase, Util):

    def test_explicit(self):
        a = bitarray('0')
        a[0] = 1
        self.assertEqual(a, bitarray('1'))

        a = bitarray(2)
        a[0] = 0
        a[1] = 1
        self.assertEqual(a, bitarray('01'))
        a[-1] = 0
        a[-2] = 1
        self.assertEqual(a, bitarray('10'))

        self.assertRaises(ValueError, a.__setitem__, 0, -1)
        self.assertRaises(TypeError, a.__setitem__, 1, None)

        self.assertRaises(IndexError, a.__setitem__,  2, True)
        self.assertRaises(IndexError, a.__setitem__, -3, False)
        self.assertRaises(TypeError, a.__setitem__, 1.5, 1)  # see issue 114
        self.assertRaises(TypeError, a.__setitem__, None, 0)
        self.assertRaises(TypeError, a.__setitem__, 'a', True)
        self.assertEqual(a, bitarray('10'))

    def test_random(self):
        for a in self.randombitarrays(start=1):
            i = randrange(len(a))
            aa = a.tolist()
            v = getrandbits(1)
            a[i] = v
            aa[i] = v
            self.assertEqual(a.tolist(), aa)
            self.check_obj(a)

    @skipIf(is_pypy)
    def test_imported(self):
        a = bytearray([5, 1, 2, 3])
        b = bitarray(endian="little", buffer=a)
        self.assertFalse(b.readonly)
        # operate on imported (writable) buffer
        b[7] = 1
        self.assertEqual(a, bytearray([0x85, 1, 2, 3]))
        b[-2] = 1
        self.assertEqual(a, bytearray([0x85, 1, 2, 0x43]))

class DelItemTests(unittest.TestCase, Util):

    def test_simple(self):
        a = bitarray('100110')
        del a[1]
        self.assertEqual(len(a), 5)
        del a[3], a[-2]
        self.assertEqual(a, bitarray('100'))
        self.assertRaises(IndexError, a.__delitem__,  3)
        self.assertRaises(IndexError, a.__delitem__, -4)

    def test_random(self):
        for a in self.randombitarrays(start=1):
            n = len(a)
            b = a.copy()
            i = randrange(n)
            del b[i]
            self.assertEQUAL(b, a[:i] + a[i + 1:])
            self.assertEqual(len(b), n - 1)
            self.check_obj(b)

    @skipIf(is_pypy)
    def test_imported(self):
        a = bytearray([5, 11, 7])
        b = bitarray(buffer=a)
        self.assertFalse(b.readonly)
        self.assertRaises(BufferError, b.__delitem__, 13)

# -------------------------- Slice index tests ------------------------------

class GetSliceTests(unittest.TestCase, Util):

    def test_slice(self):
        a = bitarray('01001111 00001')
        self.assertEQUAL(a[:], a)
        self.assertFalse(a[:] is a)
        self.assertEQUAL(a[13:2:-3], bitarray('1010'))
        self.assertEQUAL(a[2:-1:4], bitarray('010'))
        self.assertEQUAL(a[::2], bitarray('0011001'))
        self.assertEQUAL(a[8:], bitarray('00001'))
        self.assertEQUAL(a[7:], bitarray('100001'))
        self.assertEQUAL(a[:8], bitarray('01001111'))
        self.assertEQUAL(a[::-1], bitarray('10000111 10010'))
        self.assertEQUAL(a[:8:-1], bitarray('1000'))
        self.assertRaises(ValueError, a.__getitem__, slice(None, None, 0))

    def test_reverse(self):
        for _ in range(20):
            n = randrange(200)
            a = urandom_2(n)
            b = a.copy()
            b.reverse()
            self.assertEQUAL(a[::-1], b)

    def test_random_step1(self):
        for n in range(200):
            a = urandom_2(n)
            i = randint(0, n)
            j = randint(0, n)
            b = a[i:j]
            self.assertEqual(b.to01(), a.to01()[i:j])
            self.assertEqual(len(b), max(j - i, 0))
            self.assertEqual(b.endian, a.endian)

    def test_random(self):
        for n in range(200):
            a = urandom_2(n)
            s = self.random_slice(n)
            b = a[s]
            self.assertEqual(len(b), len(range(n)[s]))  # slicelength
            self.assertEqual(list(b), a.tolist()[s])
            self.assertEqual(b.endian, a.endian)

class SetSliceTests(unittest.TestCase, Util):

    def test_simple(self):
        for a in self.randombitarrays(start=1):
            n = len(a)
            b = bitarray(n)
            b[0:n] = bitarray(a)
            self.assertEqual(a, b)
            self.assertFalse(a is b)

            b = bitarray(n)
            b[:] = bitarray(a)
            self.assertEqual(a, b)
            self.assertFalse(a is b)

            b = bitarray(n)
            b[::-1] = a
            self.assertEqual(b.tolist(), a.tolist()[::-1])

    def test_random(self):
        for a in self.randombitarrays(start=1):
            len_a = len(a)
            for _ in range(10):
                s = self.random_slice(len_a)
                len_b = randrange(10) if s.step == 1 else len(range(len_a)[s])
                b = bitarray(len_b)
                c = bitarray(a)
                c[s] = b
                self.check_obj(c)
                cc = a.tolist()
                cc[s] = b.tolist()
                self.assertEqual(c, bitarray(cc))

    def test_self_random(self):
        for a in self.randombitarrays():
            n = len(a)
            for step in -1, 1:
                s = slice(None, None, step)
                # ensure slicelength equals len(a)
                self.assertEqual(len(range(n)[s]), n)
                aa = a.tolist()
                a[s] = a
                aa[s] = aa
                self.assertEqual(a, bitarray(aa))

    def test_special(self):
        for n in 0, 1, 10, 87:
            a = urandom_2(n)
            for m in 0, 1, 10, 99:
                x = urandom_2(m)
                b = a.copy()
                b[n:n] = x  # insert at end - extend
                self.assertEqual(b, a + x)
                self.assertEqual(len(b), len(a) + len(x))
                b[0:0] = x  # insert at 0 - prepend
                self.assertEqual(b, x + a + x)
                self.check_obj(b)
                self.assertEqual(len(b), len(a) + 2 * len(x))

    def test_range(self):
        # tests C function insert_n()
        for _ in range(100):
            n = randrange(200)
            a = urandom_2(n)
            p = randint(0, n)
            m = randint(0, 500)
            x = urandom_2(m)
            b = a.copy()
            b[p:p] = x
            self.assertEQUAL(b, a[:p] + x + a[p:])
            self.assertEqual(len(b), len(a) + m)
            self.check_obj(b)

    def test_resize(self):
        for _ in range(100):
            n = randint(0, 200)
            a = urandom_2(n)
            p1 = randint(0, n)
            p2 = randint(0, n)
            m = randint(0, 300)
            x = urandom_2(m)
            b = a.copy()
            b[p1:p2] = x
            b_lst = a.tolist()
            b_lst[p1:p2] = x.tolist()
            self.assertEqual(b.tolist(), b_lst)
            if p1 <= p2:
                self.assertEQUAL(b, a[:p1] + x + a[p2:])
                self.assertEqual(len(b), n + p1 - p2 + len(x))
            else:
                self.assertEqual(b, a[:p1] + x + a[p1:])
                self.assertEqual(len(b), n + len(x))
            self.check_obj(b)

    def test_self(self):
        a = bitarray('1100111')
        a[::-1] = a
        self.assertEqual(a, bitarray('1110011'))
        a[4:] = a
        self.assertEqual(a, bitarray('11101110011'))
        a[:-5] = a
        self.assertEqual(a, bitarray('1110111001110011'))

        a = bitarray('01001')
        a[:-1] = a
        self.assertEqual(a, bitarray('010011'))
        a[2::] = a
        self.assertEqual(a, bitarray('01010011'))
        a[2:-2:1] = a
        self.assertEqual(a, bitarray('010101001111'))

        a = bitarray('011')
        a[2:2] = a
        self.assertEqual(a, bitarray('010111'))
        a[:] = a
        self.assertEqual(a, bitarray('010111'))

    def test_self_shared_buffer(self):
        # This is a special case.  We have two bitarrays which share the
        # same buffer, and then do a slice assignment.  The bitarray is
        # copied onto itself in reverse order.  So we need to make a copy
        # in setslice_bitarray().  However, since a and b are two distinct
        # objects, it is not enough to check for self == other, but rather
        # check whether their buffers overlap.
        a = bitarray('11100000')
        b = bitarray(buffer=a)
        b[::-1] = a
        self.assertEqual(a, b)
        self.assertEqual(a, bitarray('00000111'))

    def test_self_shared_buffer_2(self):
        # This is an even more special case.  We have a bitarrays which
        # shares part of anothers bitarray buffer.  So in setslice_bitarray(),
        # we need to make a copy of other if:
        #
        #   self->ob_item <= other->ob_item <= self->ob_item + Py_SIZE(self)
        #
        # In words: Is the other buffer inside the self buffer (which inclues
        #           the previous case)
        a = bitarray('11111111 11000000 00000000')
        b = bitarray(buffer=memoryview(a)[1:2])
        self.assertEqual(b, bitarray('11000000'))
        a[15:7:-1] = b
        self.assertEqual(a, bitarray('11111111 00000011 00000000'))

    @skipIf(is_pypy)
    def test_self_shared_buffer_3(self):
        # Requires to check for (in setslice_bitarray()):
        #
        #   other->ob_item <= self->ob_item <= other->ob_item + Py_SIZE(other)
        #
        a = bitarray('11111111 11000000 00000000')
        b = bitarray(buffer=memoryview(a)[:2])
        c = bitarray(buffer=memoryview(a)[1:])
        self.assertEqual(b, bitarray('11111111 11000000'))
        self.assertEqual(c, bitarray('11000000 00000000'))
        c[::-1] = b
        self.assertEqual(c, bitarray('00000011 11111111'))
        self.assertEqual(a, bitarray('11111111 00000011 11111111'))

    def test_setslice_bitarray(self):
        a = ones(12)
        a[2:6] = bitarray('0010')
        self.assertEqual(a, bitarray('11001011 1111'))
        a.setall(0)
        a[::2] = bitarray('111001')
        self.assertEqual(a, bitarray('10101000 0010'))
        a.setall(0)
        a[3:] = bitarray('111')
        self.assertEqual(a, bitarray('000111'))

        a = zeros(12)
        a[1:11:2] = bitarray('11101')
        self.assertEqual(a, bitarray('01010100 0100'))
        a.setall(0)
        a[5:2] = bitarray('111')  # make sure inserts before 5 (not 2)
        self.assertEqual(a, bitarray('00000111 0000000'))

        a = zeros(12)
        a[:-6:-1] = bitarray('10111')
        self.assertEqual(a, bitarray('00000001 1101'))

    def test_bitarray_2(self):
        a = bitarray('1111')
        a[3:3] = bitarray('000')  # insert
        self.assertEqual(a, bitarray('1110001'))
        a[2:5] = bitarray()  # remove
        self.assertEqual(a, bitarray('1101'))

        a = bitarray('1111')
        a[1:3] = bitarray('0000')
        self.assertEqual(a, bitarray('100001'))
        a[:] = bitarray('010')  # replace all values
        self.assertEqual(a, bitarray('010'))

        # assign slice to bitarray with different length
        a = bitarray('111111')
        a[3:4] = bitarray('00')
        self.assertEqual(a, bitarray('1110011'))
        a[2:5] = bitarray('0')  # remove
        self.assertEqual(a, bitarray('11011'))

    def test_frozenbitarray(self):
        a = bitarray('11111111 1111')
        b = frozenbitarray('0000')
        a[2:6] = b
        self.assertEqual(a, bitarray('11000011 1111'))
        self.assertEqual(type(b), frozenbitarray)
        self.assertEqual(b, bitarray('0000'))

        b = frozenbitarray('011100')
        a[::2] = b
        self.assertEqual(a, bitarray('01101011 0101'))
        self.check_obj(a)
        self.assertEqual(type(b), frozenbitarray)
        self.assertEqual(b, bitarray('011100'))

    def test_setslice_bitarray_random_same_length(self):
        for _ in range(100):
            n = randrange(200)
            a = urandom_2(n)
            lst_a = a.tolist()
            b = urandom_2(randint(0, n))
            lst_b = b.tolist()
            i = randint(0, n - len(b))
            j = i + len(b)
            self.assertEqual(j - i, len(b))
            a[i:j] = b
            lst_a[i:j] = lst_b
            self.assertEqual(a.tolist(), lst_a)
            # a didn't change length
            self.assertEqual(len(a), n)
            self.check_obj(a)

    def test_bitarray_random_step1(self):
        for _ in range(50):
            n = randrange(300)
            a = urandom_2(n)
            lst_a = a.tolist()
            b = urandom_2(randrange(100))
            lst_b = b.tolist()
            s = self.random_slice(n, step=1)
            a[s] = b
            lst_a[s] = lst_b
            self.assertEqual(a.tolist(), lst_a)
            self.check_obj(a)

    def test_bitarray_random(self):
        for _ in range(100):
            n = randrange(50)
            a = urandom_2(n)
            lst_a = a.tolist()
            b = urandom_2(randrange(50))
            lst_b = b.tolist()
            s = self.random_slice(n)
            try:
                a[s] = b
            except ValueError:
                a = None

            try:
                lst_a[s] = lst_b
            except ValueError:
                lst_a = None

            if a is None:
                self.assertTrue(lst_a is None)
            else:
                self.assertEqual(a.tolist(), lst_a)
                self.check_obj(a)

    def test_bool_explicit(self):
        a = bitarray('11111111')
        a[::2] = False
        self.assertEqual(a, bitarray('01010101'))
        a[4::] = True #                   ^^^^
        self.assertEqual(a, bitarray('01011111'))
        a[-2:] = False #                    ^^
        self.assertEqual(a, bitarray('01011100'))
        a[:2:] = True #               ^^
        self.assertEqual(a, bitarray('11011100'))
        a[:] = True #                 ^^^^^^^^
        self.assertEqual(a, bitarray('11111111'))
        a[2:5] = False #                ^^^
        self.assertEqual(a, bitarray('11000111'))
        a[1::3] = False #              ^  ^  ^
        self.assertEqual(a, bitarray('10000110'))
        a[1:6:2] = True #              ^ ^ ^
        self.assertEqual(a, bitarray('11010110'))
        a[3:3] = False # zero slicelength
        self.assertEqual(a, bitarray('11010110'))
        a[:] = False #                ^^^^^^^^
        self.assertEqual(a, bitarray('00000000'))
        a[-2:2:-1] = 1 #                 ^^^^
        self.assertEqual(a, bitarray('00011110'))

    def test_bool_step1(self):
        for _ in range(100):
            n = randrange(1000)
            start = randint(0, n)
            stop = randint(start, n)
            a = bitarray(n)
            a[start:stop] = 1
            self.assertEqual(a.count(1), stop - start)
            b = bitarray(n)
            b[range(start, stop)] = 1
            self.assertEqual(a, b)

    def test_setslice_bool_random_slice(self):
        for _ in range(200):
            n = randrange(100)
            a = urandom_2(n)
            aa = a.tolist()
            s = self.random_slice(n)
            slicelength = len(range(n)[s])
            v = getrandbits(1)
            a[s] = v
            aa[s] = slicelength * [v]
            self.assertEqual(a.tolist(), aa)

            a.setall(0)
            a[s] = 1
            self.assertEqual(a.count(1), slicelength)

    def test_setslice_bool_step(self):
        # this test exercises set_range() when stop is much larger than start
        cnt = 0
        for _ in range(500):
            n = randrange(3000, 4000)
            a = urandom_2(n)
            aa = a.tolist()
            s = slice(randrange(1000), randrange(1000, n), randint(1, 100))
            self.assertTrue(s.stop - s.start >= 0)
            cnt += s.stop - s.start >= 1024
            slicelength = len(range(n)[s])
            self.assertTrue(slicelength > 0)
            v = getrandbits(1)
            a[s] = v
            aa[s] = slicelength * [v]
            self.assertEqual(a.tolist(), aa)
        self.assertTrue(cnt > 300)

    def test_to_int(self):
        a = bitarray('11111111')
        a[::2] = 0 #  ^ ^ ^ ^
        self.assertEqual(a, bitarray('01010101'))
        a[4::] = 1 #                      ^^^^
        self.assertEqual(a, bitarray('01011111'))
        a.__setitem__(slice(-2, None, None), 0)
        self.assertEqual(a, bitarray('01011100'))
        self.assertRaises(ValueError, a.__setitem__, slice(None, None, 2), 3)
        self.assertRaises(ValueError, a.__setitem__, slice(None, 2, None), -1)
        # a[:2:] = '0'
        self.assertRaises(TypeError, a.__setitem__, slice(None, 2, None), '0')

    def test_invalid(self):
        a = bitarray('11111111')
        s = slice(2, 6, None)
        self.assertRaises(TypeError, a.__setitem__, s, 1.2)
        self.assertRaises(TypeError, a.__setitem__, s, None)
        self.assertRaises(TypeError, a.__setitem__, s, "0110")
        a[s] = False
        self.assertEqual(a, bitarray('11000011'))
        # step != 1 and slicelen != length of assigned bitarray
        self.assertRaisesMessage(
            ValueError,
            "attempt to assign sequence of size 3 to extended slice of size 4",
            a.__setitem__, slice(None, None, 2), bitarray('000'))
        self.assertRaisesMessage(
            ValueError,
            "attempt to assign sequence of size 3 to extended slice of size 2",
            a.__setitem__, slice(None, None, 4), bitarray('000'))
        self.assertRaisesMessage(
            ValueError,
            "attempt to assign sequence of size 7 to extended slice of size 8",
            a.__setitem__, slice(None, None, -1), bitarray('0001000'))
        self.assertEqual(a, bitarray('11000011'))

    @skipIf(is_pypy)
    def test_imported(self):
        a = bytearray([5, 1, 2, 3])
        b = bitarray(endian="little", buffer=a)
        self.assertFalse(b.readonly)
        # operate on imported (writable) buffer
        b[8:-8] = 1
        self.assertEqual(a, bytearray([5, 0xff, 0xff, 3]))
        b[8:-8:2] = 0
        self.assertEqual(a, bytearray([5, 0xaa, 0xaa, 3]))
        b[8:20] = bitarray('11110000 0011')
        self.assertEqual(a, bytearray([5, 0x0f, 0xac, 3]))

class DelSliceTests(unittest.TestCase, Util):

    def test_explicit(self):
        a = bitarray('10101100 10110')
        del a[3:9] #     ^^^^^ ^
        self.assertEqual(a, bitarray('1010110'))
        del a[::3] #                  ^  ^  ^
        self.assertEqual(a, bitarray('0111'))
        a = bitarray('10101100 101101111')
        del a[5:-3:3] #    ^   ^  ^
        self.assertEqual(a, bitarray('1010100 0101111'))
        a = bitarray('10101100 1011011')
        del a[:-9:-2] #        ^ ^ ^ ^
        self.assertEqual(a, bitarray('10101100 011'))
        del a[3:3] # zero slicelength
        self.assertEqual(a, bitarray('10101100 011'))
        self.assertRaises(ValueError, a.__delitem__, slice(None, None, 0))
        self.assertEqual(len(a), 11)
        del a[:]
        self.assertEqual(a, bitarray())

    def test_special(self):
        for n in 0, 1, 10, 73:
            a = urandom_2(n)
            b = a.copy()
            del b[:0]
            del b[n:]
            self.assertEqual(b, a)
            del b[10:]  # delete everything after 10th item
            self.assertEqual(b, a[:10])
            del b[:]  # clear
            self.assertEqual(len(b), 0)
            self.check_obj(b)

    def test_random(self):
        for _ in range(100):
            n = randrange(500)
            a = urandom_2(n)
            s = self.random_slice(n)
            slicelength = len(range(n)[s])
            c = a.copy()
            del c[s]
            self.assertEqual(len(c), n - slicelength)
            self.check_obj(c)
            c_lst = a.tolist()
            del c_lst[s]
            self.assertEQUAL(c, bitarray(c_lst, endian=c.endian))

    def test_range(self):
        # tests C function delete_n()
        for n in range(200):
            a = urandom_2(n)
            p = randint(0, n)
            m = randint(0, 200)

            b = a.copy()
            del b[p:p + m]
            self.assertEQUAL(b, a[:p] + a[p + m:])
            self.check_obj(b)

    def test_range_step(self):
        n = 200
        for step in range(-n - 1, n):
            if step == 0:
                continue
            a = urandom_2(n)
            lst = a.tolist()
            del a[::step]
            del lst[::step]
            self.assertEqual(a.tolist(), lst)

    @skipIf(is_pypy)
    def test_imported(self):
        a = bytearray([5, 1, 2, 3])
        b = bitarray(buffer=a)
        self.assertFalse(b.readonly)
        self.assertRaises(BufferError, b.__delitem__, slice(3, 21))
        # even though we don't delete anything, raise error
        self.assertRaises(BufferError, b.__delitem__, slice(3, 3))

# ----------------------------- Masked index tests --------------------------

class GetMaskedIndexTests(unittest.TestCase, Util):

    def test_basic(self):
        a =    bitarray('1001001')
        mask = bitarray('1010111')
        self.assertEqual(a[mask], bitarray('10001'))
        self.assertRaises(IndexError, a.__getitem__, bitarray('1011'))

    def test_random(self):
        for a in self.randombitarrays():
            n = len(a)
            # select items from a wehre a is 1  ->  all 1 items
            self.assertEqual(a[a], a.count() * bitarray('1'))

            mask = zeros(n)
            self.assertEqual(a[mask], bitarray())

            mask = ones(n)
            self.assertEqual(a[mask], a)

            mask = urandom_2(n)
            self.assertEqual(list(a[mask]),
                             [a[i] for i in range(n) if mask[i]])

    def test_random_slice_mask(self):
        for n in range(100):
            s = self.random_slice(n, step=randint(1, 5))
            a = urandom_2(n)
            mask = zeros(n)
            mask[s] = 1
            self.assertEQUAL(a[mask], a[s])

class SetMaskedIndexTests(unittest.TestCase, Util):

    def test_basic(self):
        a =    bitarray('1001001')
        mask = bitarray('1010111')
        val =  bitarray("0 1 110")
        res =  bitarray("0011110")
        self.assertRaises(NotImplementedError, a.__setitem__, mask, 1)
        self.assertRaises(ValueError, a.__setitem__, mask, 2)
        a[mask] = val
        self.assertEqual(a, res)
        b = bitarray('0111')
        self.assertRaisesMessage(
            IndexError,
            "attempt to assign mask of size 5 to bitarray of size 4",
            a.__setitem__, mask, b)

    def test_issue225(self):
        # example from issue #225
        a = bitarray('0000000')
        b = bitarray('1100110')
        c = bitarray('10  10 ')
        a[b] = c
        self.assertEqual(a,
            bitarray('1000100'))

    def test_zeros_mask(self):
        for a in self.randombitarrays():
            b = a.copy()
            mask = zeros(len(a))
            a[mask] = bitarray()
            self.assertEqual(a, b)

    def test_ones_mask(self):
        for a in self.randombitarrays():
            n = len(a)
            mask = ones(n)
            c = urandom_2(n)
            a[mask] = c
            self.assertEqual(a, c)

    def test_random_mask_set_random(self):
        for a in self.randombitarrays():
            b = a.copy()
            mask = urandom_2(len(a))
            other = urandom_2(mask.count())
            a[mask] = other
            b[list(mask.search(1))] = other
            self.assertEqual(a, b)

    def test_random_slice_mask(self):
        for n in range(100):
            s = self.random_slice(n, randint(1, 5))
            slicelength = len(range(n)[s])
            a = urandom_2(n)
            b = a.copy()
            mask = zeros(n)
            mask[s] = 1
            other = urandom_2(slicelength)
            a[mask] = b[s] = other
            self.assertEQUAL(a, b)

    def test_random_mask_set_zeros(self):
        for a in self.randombitarrays():
            mask = urandom_2(len(a), endian=a.endian)
            b = a.copy()
            self.assertRaisesMessage(
                NotImplementedError,
                "mask assignment to bool not implemented;\n"
                "`a[mask] = 0` equivalent to `a &= ~mask`",
                a.__setitem__, mask, 0)
            a[mask] = zeros(mask.count())
            b &= ~mask
            self.assertEqual(a, b)

    def test_random_mask_set_ones(self):
        for a in self.randombitarrays():
            mask = urandom_2(len(a), endian=a.endian)
            b = a.copy()
            self.assertRaisesMessage(
                NotImplementedError,
                "mask assignment to bool not implemented;\n"
                "`a[mask] = 1` equivalent to `a |= mask`",
                a.__setitem__, mask, 1)
            a[mask] = ones(mask.count())
            b |= mask
            self.assertEqual(a, b)

    @skipIf(is_pypy)
    def test_imported(self):
        a = bytearray([0, 0xff])
        #             00000000 11111111
        b = bitarray(endian="big", buffer=a)
        c = bitarray('00001111 00110011')
        b[c] = bitarray(' 1001   01  10')
        self.assertEqual(a, bytearray([0b00001001, 0b11011110]))

class DelMaskedIndexTests(unittest.TestCase, Util):

    def test_basic(self):
        a =    bitarray('1001001')
        mask = bitarray('1010111')
        del a[mask]
        self.assertEqual(a, bitarray('01'))
        self.assertRaises(IndexError, a.__delitem__, bitarray('101'))

    def test_zeros_mask(self):
        for a in self.randombitarrays():
            b = a.copy()
            # mask has only zeros - nothing will be removed
            mask = zeros(len(a))
            del b[mask]
            self.assertEqual(b, a)

    def test_ones_mask(self):
        for a in self.randombitarrays():
            # mask has only ones - everything will be removed
            mask = ones(len(a))
            del a[mask]
            self.assertEqual(a, bitarray())

    def test_self_mask(self):
        for a in self.randombitarrays():
            cnt0 = a.count(0)
            # mask is bitarray itself - all 1 items are removed -
            # only all the 0's remain
            del a[a]
            self.assertEqual(a, zeros(cnt0))

    def test_random_mask(self):
        for a in self.randombitarrays():
            n = len(a)
            b = a.copy()
            mask = urandom_2(n)
            del b[mask]
            self.assertEqual(b,
                             bitarray(a[i] for i in range(n) if not mask[i]))
            # `del a[mask]` is equivalent to the in-place version of
            # selecting the inverted mask `a = a[~mask]`
            self.assertEqual(b, a[~mask])

    def test_random_slice_mask(self):
        for n in range(100):
            s = self.random_slice(n)
            a = urandom_2(n)
            b = a.copy()
            mask = zeros(n)
            mask[s] = 1
            del a[mask], b[s]
            self.assertEQUAL(a, b)

    @skipIf(is_pypy)
    def test_imported(self):
        a = bytearray([5, 3])
        b = bitarray(buffer=a)
        self.assertFalse(b.readonly)
        self.assertRaises(BufferError, b.__delitem__,
                          bitarray('00001111 00110011'))
        # even though we don't delete anything, raise error
        self.assertRaises(BufferError, b.__delitem__, bitarray(16))

# ------------------------- Sequence index tests ----------------------------

class CommonSequenceIndexTests(unittest.TestCase, Util):

    def test_type_messages(self):
        for item, msg in [
                (tuple([1, 2]), "multiple dimensions not supported"),
                (None, "bitarray indices must be integers, slices or "
                       "sequences, not 'NoneType'"),
                (0.12, "bitarray indices must be integers, slices or "
                       "sequences, not 'float'"),
        ]:
            a = bitarray('10111')
            self.assertRaisesMessage(TypeError, msg, a.__getitem__, item)
            self.assertRaisesMessage(TypeError, msg, a.__setitem__, item, 1)
            self.assertRaisesMessage(TypeError, msg, a.__delitem__, item)

class GetSequenceIndexTests(unittest.TestCase, Util):

    def test_basic(self):
        a = bitarray('00110101 00')
        self.assertEqual(a[[2, 4, -3, 9]], bitarray('1010'))
        self.assertEqual(a[71 * [2, 4, 7]], 71 * bitarray('101'))
        self.assertEqual(a[[-1]], bitarray('0'))
        self.assertEqual(a[[]], bitarray())
        self.assertRaises(IndexError, a.__getitem__, [1, 10])
        self.assertRaises(IndexError, a.__getitem__, [-11])

    def test_types(self):
        a = bitarray('11001101 01')
        lst = [1, 3, -2]
        for b in lst, array.array('i', lst):
            self.assertEqual(a[b], bitarray('100'))
        lst[2] += len(a)
        self.assertEqual(a[bytearray(lst)], bitarray('100'))
        self.assertEqual(a[bytes(lst)], bitarray('100'))

        self.assertRaises(TypeError, a.__getitem__, [2, "B"])
        self.assertRaises(TypeError, a.__getitem__, [2, 1.2])
        self.assertRaises(TypeError, a.__getitem__, tuple(lst))

    def test_random(self):
        for a in self.randombitarrays():
            n = len(a)
            lst = choices(range(n), k=n//2)
            b = a[lst]
            self.assertEqual(b, bitarray(a[i] for i in lst))
            self.assertEqual(b.endian, a.endian)

    def test_range(self):
        for n in range(100):
            s = self.random_slice(n)
            r = range(n)[s]
            a = urandom_2(n)
            self.assertEQUAL(a[r], a[s])

class SetSequenceIndexTests(unittest.TestCase, Util):

    def test_bool_basic(self):
        a = zeros(10)
        a[[2, 3, 5, 7]] = 1
        self.assertEqual(a, bitarray('00110101 00'))
        a[[]] = 1
        self.assertEqual(a, bitarray('00110101 00'))
        a[[-1]] = True
        self.assertEqual(a, bitarray('00110101 01'))
        a[[3, -1]] = 0
        self.assertEqual(a, bitarray('00100101 00'))
        self.assertRaises(IndexError, a.__setitem__, [1, 10], 0)
        self.assertRaises(ValueError, a.__setitem__, [1], 2)
        self.assertRaises(TypeError, a.__setitem__, [1], "A")
        self.assertRaises(TypeError, a.__setitem__, (3, -1))
        self.assertRaises(TypeError, a.__setitem__, a)

    def test_bool_random(self):
        for a in self.randombitarrays():
            n = len(a)
            lst = choices(range(n), k=n//2)
            b = a.copy()
            v = getrandbits(1)
            a[lst] = v
            for i in lst:
                b[i] = v
            self.assertEqual(a, b)

    def test_bool_range(self):
        for n in range(100):
            s = self.random_slice(n)
            r = range(n)[s]
            a = urandom_2(n)
            b = a.copy()
            a[s] = b[r] = getrandbits(1)
            self.assertEQUAL(a, b)

    def test_bitarray_basic(self):
        a = zeros(10)
        a[[2, 3, 5, 7]] = bitarray('1101')
        self.assertEqual(a, bitarray('00110001 00'))
        a[[]] = bitarray()
        self.assertEqual(a, bitarray('00110001 00'))
        a[[5, -1]] = bitarray('11')
        self.assertEqual(a, bitarray('00110101 01'))
        self.assertRaises(IndexError, a.__setitem__, [1, 10], bitarray('11'))
        self.assertRaises(ValueError, a.__setitem__, [1], bitarray())
        msg = "attempt to assign sequence of size 2 to bitarray of size 3"
        self.assertRaisesMessage(ValueError, msg,
                                 a.__setitem__, [1, 2], bitarray('001'))

    def test_bitarray_random(self):
        for a in self.randombitarrays():
            n = len(a)
            lst = choices(range(n), k=n//2)
            c = urandom_2(len(lst))
            b = a.copy()

            a[lst] = c
            for i, j in enumerate(lst):
                b[j] = c[i]
            self.assertEqual(a, b)

    def test_bitarray_range(self):
        for n in range(100):
            s = self.random_slice(n)
            r = range(n)[s]
            a = urandom_2(n)
            b = a.copy()
            # note that len(r) is slicelength
            a[s] = b[r] = urandom_2(len(r))
            self.assertEQUAL(a, b)

    def test_bitarray_random_self(self):
        for a in self.randombitarrays():
            lst = list(range(len(a)))
            shuffle(lst)
            b = a.copy()
            c = a.copy()

            a[lst] = a
            for i, j in enumerate(lst):
                b[j] = c[i]
            self.assertEqual(a, b)

    @skipIf(is_pypy)
    def test_imported(self):
        a = bytearray([0, 1, 2, 3])
        b = bitarray(endian="big", buffer=a)
        self.assertFalse(b.readonly)
        # operate on imported (writable) buffer
        b[range(0, 32, 8)] = 1
        self.assertEqual(a, bytearray([0x80, 0x81, 0x82, 0x83]))
        b[range(0, 10)] = bitarray("00001111 01", "little")
        self.assertEqual(a, bytearray([0x0f, 0x41, 0x82, 0x83]))

class DelSequenceIndexTests(unittest.TestCase, Util):

    def test_basic(self):
        a = bitarray('00110101 00')
        #               ^ ^  ^  ^
        del a[[2, 4, 7, 9]]
        self.assertEqual(a, bitarray('001100'))
        del a[[]]  # delete nothing
        self.assertEqual(a, bitarray('001100'))
        del a[[2]]
        self.assertEqual(a, bitarray('00100'))
        a = bitarray('00110101 00')
        # same as earlier, but list is not ordered and has repeated indices
        del a[[7, 9, 2, 2, 4, 7]]
        self.assertEqual(a, bitarray('001100'))
        self.assertRaises(IndexError, a.__delitem__, [1, 10])
        self.assertRaises(IndexError, a.__delitem__, [10])
        self.assertRaises(TypeError, a.__delitem__, (1, 3))

    def test_delete_one(self):
        for a in self.randombitarrays(start=1):
            b = a.copy()
            i = randrange(len(a))
            del a[i], b[[i]]
            self.assertEqual(a, b)

    def test_random(self):
        for n in range(100):
            a = urandom_2(n)
            lst = choices(range(n), k=randint(0, n))
            b = a.copy()
            del a[lst]
            self.assertEqual(len(a), n - len(set(lst)))
            for i in sorted(set(lst), reverse=True):
                del b[i]
            self.assertEqual(a, b)

    def test_shuffle(self):
        for a in self.randombitarrays():
            lst = list(range(len(a)))
            shuffle(lst)
            del a[lst]
            self.assertEqual(len(a), 0)

    def test_range(self):
        for n in range(100):
            s = self.random_slice(n)
            r = range(n)[s]
            a = urandom_2(n)
            b = a.copy()
            del a[s], b[r]
            self.assertEQUAL(a, b)

    @skipIf(is_pypy)
    def test_imported(self):
        a = bytearray([0, 1, 2, 3])
        b = bitarray(buffer=a)
        self.assertFalse(b.readonly)
        # operate on imported (writable) buffer
        self.assertRaises(BufferError, b.__delitem__, range(0, 32, 8))
        # even though we don't delete anything, raise error
        self.assertRaises(BufferError, b.__delitem__, range(0))

# ---------------------------------------------------------------------------

class MiscTests(unittest.TestCase, Util):

    def test_instancecheck(self):
        a = bitarray('011')
        self.assertIsInstance(a, bitarray)
        self.assertFalse(isinstance(a, str))

    def test_booleanness(self):
        self.assertEqual(bool(bitarray('')), False)
        self.assertEqual(bool(bitarray('0')), True)
        self.assertEqual(bool(bitarray('1')), True)

    def test_iterate(self):
        for lst in self.randomlists():
            acc = []
            for b in bitarray(lst):
                acc.append(b)
            self.assertEqual(acc, lst)

    def test_iter1(self):
        it = iter(bitarray('011'))
        self.assertIsType(it, 'bitarrayiterator')
        for res in 0, 1, 1:
            item = next(it)
            self.assertEqual(type(item), int)
            self.assertEqual(item, res)
        self.assertRaises(StopIteration, next, it)

    def test_iter2(self):
        for a in self.randombitarrays():
            aa = a.tolist()
            self.assertEqual(list(a), aa)
            self.assertEqual(list(iter(a)), aa)

    def test_assignment(self):
        a = bitarray('00110111001')
        a[1:3] = a[7:9]
        a[-1:] = a[:1]
        b = bitarray('01010111000')
        self.assertEqual(a, b)

    def test_subclassing(self):
        class ExaggeratingBitarray(bitarray):

            def __new__(cls, data, offset):
                return bitarray.__new__(cls, data)

            def __init__(self, data, offset):
                self.offset = offset

            def __getitem__(self, i):
                return bitarray.__getitem__(self, i - self.offset)

        for a in self.randombitarrays():
            b = ExaggeratingBitarray(a, 1234)
            for i in range(len(a)):
                self.assertEqual(a[i], b[i + 1234])

    @skipIf(is_pypy)
    def test_overflow(self):
        a = bitarray(1)
        for i in 0, 1:
            n = (1 << 63) + i
            self.assertRaises(OverflowError, a.__imul__, n)
            self.assertRaises(OverflowError, bitarray, n)

        a = bitarray(1 << 10)
        self.assertRaises(OverflowError, a.__imul__, 1 << 53)

    @skipIf(PTRSIZE != 4 or is_pypy)
    def test_overflow_32bit(self):
        a = bitarray(1000_000)
        self.assertRaises(OverflowError, a.__imul__, 17180)
        for i in 0, 1:
            self.assertRaises(OverflowError, bitarray, (1 << 31) + i)
        try:
            a = bitarray((1 << 31) - 1);
        except MemoryError:
            return
        self.assertRaises(OverflowError, bitarray.append, a, True)

    def test_unhashable(self):
        a = bitarray()
        self.assertRaises(TypeError, hash, a)
        self.assertRaises(TypeError, dict, [(a, 'foo')])

    def test_abc(self):
        from collections import abc

        a = bitarray('001')
        self.assertIsInstance(a, abc.Iterable)
        self.assertIsInstance(a, abc.Sized)
        self.assertIsInstance(a, abc.Sequence)
        self.assertIsInstance(a, abc.MutableSequence)
        if sys.version_info[:2] >= (3, 12):
            self.assertIsInstance(a, abc.Buffer)
        if sys.platform != "win32":
            self.assertFalse(isinstance(a, abc.Hashable))

# ---------------------------------------------------------------------------

class PickleTests(unittest.TestCase, Util):

    def test_attributes(self):
        a = frozenbitarray("00110")
        # as a is a subclass of bitarray, we can have attributes
        a.x = "bar"
        a.y = "baz"

        b = pickle.loads(pickle.dumps(a))
        self.assertEqual(b, a)
        self.assertEqual(b.x, "bar")
        self.assertEqual(b.y, "baz")

    def test_readonly(self):
        a = bitarray(buffer=b'A')
        # readonly (because buffer is readonly), but not frozenbitarray
        self.assertTrue(a.readonly)
        self.assertEqual(type(a), bitarray)

        b = pickle.loads(pickle.dumps(a))
        self.assertTrue(b.readonly)
        self.assertEqual(type(b), bitarray)

    def test_endian(self):
        for endian in 'little', 'big':
            a = bitarray(endian=endian)
            b = pickle.loads(pickle.dumps(a))
            self.assertEqual(b.endian, endian)

    def test_reduce_explicit(self):
        a = frozenbitarray('11001111 01001', 'little')
        a.quux = 12
        res = (_bitarray_reconstructor,
               (frozenbitarray, b'\xf3\x12', 'little', 3, 1),
               {'quux': 12})
        self.assertEqual(a.__reduce__(), res)

    def check_reduce(self, a):
        try:
            attrs = a.__dict__
        except AttributeError:
            attrs = None

        res = (
            _bitarray_reconstructor,
            (
                type(a),         # type object
                a.tobytes(),     # buffer
                a.endian,        # endianness
                a.padbits,       # number of pad bits
                int(a.readonly)  # readonly
            ),
            attrs)  # __dict__ or None
        self.assertEqual(a.__reduce__(), res)

        b = _bitarray_reconstructor(*res[1])
        self.assertEqual(a, b)
        self.assertEqual(type(a), type(b))
        self.assertEqual(a.endian, b.endian)
        self.assertEqual(a.readonly, b.readonly)
        self.check_obj(b)

    @skipIf(is_pypy)
    def test_reduce_random(self):
        for a in self.randombitarrays():
            self.check_reduce(a)
            b = frozenbitarray(a)
            self.check_reduce(b)
            b.foo = 42
            self.check_reduce(b)

    def test_reconstructor_explicit(self):
        a = _bitarray_reconstructor(bitarray, b'', 'little', 0, 0)
        self.assertEqual(len(a), 0)
        self.assertEqual(a.endian, 'little')
        self.check_obj(a)

        a = _bitarray_reconstructor(bitarray, b'\x0f', 'big', 1, 0)
        self.assertEqual(a, bitarray("0000111"))
        self.assertEqual(a.endian, 'big')
        self.check_obj(a)

    def test_reconstructor_invalid_args(self):
        # argument 1 - type object
        self.assertRaisesMessage(
            TypeError, "first argument must be a type object, got 'str'",
            _bitarray_reconstructor, "foo", b'', 'big', 0, 0)

        self.assertRaisesMessage(
            TypeError, "'list' is not a subtype of bitarray",
            _bitarray_reconstructor, list, b'', 'big', 0, 0)

        # argument 2 - buffer
        self.assertRaisesMessage(
            TypeError, "second argument must be bytes, got 'int'",
            _bitarray_reconstructor, bitarray, 123, 'big', 0, 0)

        # argument 3 - bit-endianness
        self.assertRaises(TypeError, _bitarray_reconstructor,
                          bitarray, b'\x0f', 123, 1, 0)
        self.assertRaisesMessage(
            ValueError,
            "bit-endianness must be either 'little' or 'big', not 'small'",
            _bitarray_reconstructor, bitarray, b"", "small", 0, 0)

        # argument 4 - number of pad bits
        self.assertRaises(TypeError, _bitarray_reconstructor,
                          bitarray, b'\x0f', 'big', 0.0, 0)
        self.assertRaisesMessage(
            ValueError, "invalid number of pad bits: 8",
            _bitarray_reconstructor, bitarray, b"A", "big", 8, 0)
        self.assertRaisesMessage(
            # the number of bytes is 0 zero, so pad bits cannot be 1
            ValueError, "invalid number of pad bits: 1",
            _bitarray_reconstructor, bitarray, b"", "big", 1, 0)

        # argument 5 - readonly
        self.assertRaises(TypeError, _bitarray_reconstructor,
                          bitarray, b'\x0f', 'big', 1, 'foo')

    def check_file(self, fn):
        path = os.path.join(os.path.dirname(__file__), fn)
        with open(path, 'rb') as fi:
            d = pickle.load(fi)

        for i, (s, end) in enumerate([
                # 0x03
                ('110', 'little'),
                # 0x60
                ('011', 'big'),
                # 0x07    0x12    0x00    0x40
                ('1110000001001000000000000000001', 'little'),
                # 0x27    0x80    0x00    0x02
                ('0010011110000000000000000000001', 'big'),
        ]):
            b = d['b%d' % i]
            self.assertEqual(b.to01(), s)
            self.assertEqual(b.endian, end)
            self.assertEqual(type(b), bitarray)
            self.assertFalse(b.readonly)
            self.check_obj(b)

            f = d['f%d' % i]
            self.assertEqual(f.to01(), s)
            self.assertEqual(f.endian, end)
            self.assertEqual(type(f), frozenbitarray)
            self.assertTrue(f.readonly)
            self.check_obj(f)

    def test_load(self):
        # using bitarray 2.8.1 / Python 3.5.5 (_bitarray_reconstructor)
        self.check_file('test_281.pickle')

    def test_random(self):
        for a in self.randombitarrays():
            b = pickle.loads(pickle.dumps(a))
            self.assertFalse(b.readonly)
            self.assertFalse(b is a)
            self.assertEQUAL(a, b)
            self.check_obj(b)

# ---------------------------- Richcompare tests ----------------------------

class RichCompareTests(unittest.TestCase, Util):

    def test_wrong_types(self):
        a = bitarray()
        for x in None, 7, 'A':
            self.assertEqual(a.__eq__(x), NotImplemented)
            self.assertEqual(a.__ne__(x), NotImplemented)
            self.assertEqual(a.__ge__(x), NotImplemented)
            self.assertEqual(a.__gt__(x), NotImplemented)
            self.assertEqual(a.__le__(x), NotImplemented)
            self.assertEqual(a.__lt__(x), NotImplemented)

    def test_explicit(self):
        for sa, sb, res in [
                ('',   '',   '101010'),
                ('0',  '0',  '101010'),
                ('1',  '1',  '101010'),
                ('0',  '',   '011100'),
                ('1',  '',   '011100'),
                ('1',  '0',  '011100'),
                ('11', '10', '011100'),
                ('01', '00', '011100'),
                ('0',  '1',  '010011'),
                ('',   '0',  '010011'),
                ('',   '1',  '010011'),
        ]:
            a = bitarray(sa, self.random_endian())
            b = bitarray(sb, self.random_endian())
            r = bitarray(res)
            self.assertEqual(a == b, r[0])
            self.assertEqual(a != b, r[1])
            self.assertEqual(a >= b, r[2])
            self.assertEqual(a >  b, r[3])
            self.assertEqual(a <= b, r[4])
            self.assertEqual(a <  b, r[5])

    def test_eq_ne(self):
        for _ in range(5):
            self.assertTrue(urandom_2(0) == urandom_2(0))
            self.assertFalse(urandom_2(0) != urandom_2(0))

        for n in range(1, 20):
            a = ones(n, self.random_endian())
            b = bitarray(a, self.random_endian())
            self.assertTrue(a == b)
            self.assertFalse(a != b)
            b[-1] = 0
            self.assertTrue(a != b)
            self.assertFalse(a == b)

    def test_eq_ne_random(self):
        for a in self.randombitarrays(start=1):
            b = bitarray(a, self.random_endian())
            self.assertTrue(a == b)
            self.assertFalse(a != b)
            b.invert(randrange(len(a)))
            self.assertTrue(a != b)
            self.assertFalse(a == b)

    def check(self, a, b, c, d):
        self.assertEqual(a == b, c == d)
        self.assertEqual(a != b, c != d)
        self.assertEqual(a <= b, c <= d)
        self.assertEqual(a <  b, c <  d)
        self.assertEqual(a >= b, c >= d)
        self.assertEqual(a >  b, c >  d)

    def test_invert_random_element(self):
        for a in self.randombitarrays(start=1):
            n = len(a)
            b = bitarray(a, self.random_endian())
            i = randrange(n)
            b.invert(i)
            self.check(a, b, a[i], b[i])

    def test_size(self):
        for _ in range(10):
            a = bitarray(randrange(20), self.random_endian())
            b = bitarray(randrange(20), self.random_endian())
            self.check(a, b, len(a), len(b))

    def test_random(self):
        for a in self.randombitarrays():
            aa = a.tolist()
            if getrandbits(1):
                a = frozenbitarray(a)
            for b in self.randombitarrays():
                bb = b.tolist()
                if getrandbits(1):
                    b = frozenbitarray(b)
                self.check(a, b, aa, bb)
                self.check(a, b, aa, bb)

# ---------------------------------------------------------------------------

class SpecialMethodTests(unittest.TestCase, Util):

    def test_repr(self):
        r = repr(bitarray())
        self.assertEqual(r, "bitarray()")
        self.assertEqual(type(r), str)

        r = repr(bitarray('10111'))
        self.assertEqual(r, "bitarray('10111')")
        self.assertEqual(type(r), str)

        for a in self.randombitarrays():
            self.assertEqual(repr(a), str(a))
            b = eval(repr(a))
            self.assertFalse(b is a)
            self.assertEqual(a, b)
            self.check_obj(b)

    def test_copy(self):
        for a in self.randombitarrays():
            b = a.copy()
            self.assertFalse(b is a)
            self.assertEQUAL(a, b)

            b = copy.copy(a)
            self.assertFalse(b is a)
            self.assertEQUAL(a, b)

            b = copy.deepcopy(a)
            self.assertFalse(b is a)
            self.assertEQUAL(a, b)

    @skipIf(is_pypy)
    def test_sizeof(self):
        a = bitarray()
        size = sys.getsizeof(a)
        self.assertEqual(size, a.__sizeof__())
        self.assertEqual(type(size), int)
        self.assertTrue(size < 200)
        a = bitarray(8000)
        self.assertTrue(sys.getsizeof(a) > 1000)

# -------------------------- Sequence methods tests -------------------------

class SequenceTests(unittest.TestCase, Util):

    def test_len(self):
        for n in range(100):
            a = bitarray(n)
            self.assertEqual(len(a), n)

    def test_concat(self):
        a = bitarray('001')
        b = a + bitarray('110')
        self.assertEQUAL(b, bitarray('001110'))
        b = a + [0, 1, True]
        self.assertEQUAL(b, bitarray('001011'))
        b = a + '100'
        self.assertEQUAL(b, bitarray('001100'))
        b = a + (1, 0, True)
        self.assertEQUAL(b, bitarray('001101'))
        self.assertRaises(ValueError, a.__add__, (0, 1, 2))
        self.assertEQUAL(a, bitarray('001'))

        self.assertRaises(TypeError, a.__add__, 42)
        self.assertRaises(ValueError, a.__add__, b'1101')

        for a in self.randombitarrays():
            aa = a.copy()
            for b in self.randombitarrays():
                bb = b.copy()
                c = a + b
                self.assertEqual(c, bitarray(a.tolist() + b.tolist()))
                self.assertEqual(c.endian, a.endian)
                self.check_obj(c)

                self.assertEQUAL(a, aa)
                self.assertEQUAL(b, bb)

    def test_inplace_concat(self):
        a = bitarray('001')
        a += bitarray('110')
        self.assertEqual(a, bitarray('001110'))
        a += [0, 1, True]
        self.assertEqual(a, bitarray('001110011'))
        a += '100'
        self.assertEqual(a, bitarray('001110011100'))
        a += (1, 0, True)
        self.assertEqual(a, bitarray('001110011100101'))

        a = bitarray('110')
        self.assertRaises(ValueError, a.__iadd__, [0, 1, 2])
        self.assertEqual(a, bitarray('110'))

        self.assertRaises(TypeError, a.__iadd__, 42)
        self.assertRaises(ValueError, a.__iadd__, b'101')

        for a in self.randombitarrays():
            for b in self.randombitarrays():
                c = bitarray(a)
                d = c
                d += b
                self.assertEqual(d, a + b)
                self.assertTrue(c is d)
                self.assertEQUAL(c, d)
                self.assertEqual(d.endian, a.endian)
                self.check_obj(d)

    def test_repeat_explicit(self):
        for m, s, r in [
                ( 0,        '',      ''),
                ( 0, '1001111',      ''),
                (-1,  '100110',      ''),
                (11,        '',      ''),
                ( 1,     '110',   '110'),
                ( 2,      '01',  '0101'),
                ( 5,       '1', '11111'),
        ]:
            a = bitarray(s)
            self.assertEqual(a * m, bitarray(r))
            self.assertEqual(m * a, bitarray(r))
            c = a.copy()
            c *= m
            self.assertEqual(c, bitarray(r))

    def test_repeat_wrong_args(self):
        a = bitarray()
        self.assertRaises(TypeError, a.__mul__, None)
        self.assertRaises(TypeError, a.__mul__, 2.0)
        self.assertRaises(TypeError, a.__imul__, None)
        self.assertRaises(TypeError, a.__imul__, 3.0)

    def test_repeat_random(self):
        for a in self.randombitarrays():
            b = a.copy()
            for m in list(range(-3, 5)) + [randint(5, 200)]:
                res = bitarray(m * a.to01(), endian=a.endian)
                self.assertEqual(len(res), len(a) * max(0, m))

                self.assertEQUAL(a * m, res)
                self.assertEQUAL(m * a, res)

                c = a.copy()
                c *= m
                self.assertEQUAL(c, res)
                self.check_obj(c)

            self.assertEQUAL(a, b)

    def test_contains_simple(self):
        a = bitarray()
        self.assertFalse(0 in a)
        self.assertFalse(1 in a)
        self.assertTrue(bitarray() in a)
        a.append(1)
        self.assertTrue(1 in a)
        self.assertFalse(0 in a)
        a = bitarray([0])
        self.assertTrue(0 in a)
        self.assertFalse(1 in a)
        a.append(1)
        self.assertTrue(0 in a)
        self.assertTrue(1 in a)

    def test_contains_errors(self):
        a = bitarray()
        self.assertEqual(a.__contains__(1), False)
        a.append(1)
        self.assertEqual(a.__contains__(1), True)
        a = bitarray('0011')
        self.assertEqual(a.__contains__(bitarray('01')), True)
        self.assertEqual(a.__contains__(bitarray('10')), False)
        self.assertRaises(TypeError, a.__contains__, 'asdf')
        self.assertRaises(ValueError, a.__contains__, 2)
        self.assertRaises(ValueError, a.__contains__, -1)

    def test_contains_range(self):
        for n in range(2, 50):
            a = zeros(n)
            self.assertTrue(0 in a)
            self.assertFalse(1 in a)
            a[randrange(n)] = 1
            self.assertTrue(1 in a)
            self.assertTrue(0 in a)
            a.setall(1)
            self.assertTrue(True in a)
            self.assertFalse(False in a)
            a[randrange(n)] = 0
            self.assertTrue(True in a)
            self.assertTrue(False in a)

    def test_contains_explicit(self):
        a = bitarray('011010000001')
        for s, r in [('', True), # every bitarray contains an empty one
                     ('1', True), ('11', True), ('111', False),
                     ('011', True), ('0001', True), ('00011', False)]:
            c = bitarray(s) in a
            self.assertTrue(c is r)

# -------------------------- Number methods tests ---------------------------

class NumberTests(unittest.TestCase, Util):

    def test_misc(self):
        for a in self.randombitarrays():
            b = ~a
            c = a & b
            self.assertEqual(c.any(), False)
            self.assertEqual(a, a ^ c)
            d = a ^ b
            self.assertEqual(d.all(), True)
            b &= d
            self.assertEqual(~b, a)

    def test_bool(self):
        a = bitarray()
        self.assertTrue(bool(a) is False)
        a.append(0)
        self.assertTrue(bool(a) is True)
        a.append(1)
        self.assertTrue(bool(a) is True)

    def test_size_error(self):
        a = bitarray('11001')
        b = bitarray('100111')
        self.assertRaises(ValueError, lambda: a & b)
        self.assertRaises(ValueError, lambda: a | b)
        self.assertRaises(ValueError, lambda: a ^ b)
        for x in (a.__and__, a.__or__, a.__xor__,
                  a.__iand__, a.__ior__, a.__ixor__):
            self.assertRaises(ValueError, x, b)

    def test_endianness_error(self):
        a = bitarray('11001', 'big')
        b = bitarray('10011', 'little')
        self.assertRaises(ValueError, lambda: a & b)
        self.assertRaises(ValueError, lambda: a | b)
        self.assertRaises(ValueError, lambda: a ^ b)
        for x in (a.__and__, a.__or__, a.__xor__,
                  a.__iand__, a.__ior__, a.__ixor__):
            self.assertRaises(ValueError, x, b)

    def test_and(self):
        a = bitarray('11001')
        b = bitarray('10011')
        c = a & b
        self.assertEqual(c, bitarray('10001'))
        self.check_obj(c)

        self.assertRaises(TypeError, lambda: a & 1)
        self.assertRaises(TypeError, lambda: 1 & a)
        self.assertEqual(a, bitarray('11001'))
        self.assertEqual(b, bitarray('10011'))

    def test_or(self):
        a = bitarray('11001')
        b = bitarray('10011')
        c = a | b
        self.assertEqual(c, bitarray('11011'))
        self.check_obj(c)

        self.assertRaises(TypeError, lambda: a | 1)
        self.assertRaises(TypeError, lambda: 1 | a)
        self.assertEqual(a, bitarray('11001'))
        self.assertEqual(b, bitarray('10011'))

    def test_xor(self):
        a = bitarray('11001')
        b = bitarray('10011')
        c = a ^ b
        self.assertEQUAL(c, bitarray('01010'))
        self.check_obj(c)

        self.assertRaises(TypeError, lambda: a ^ 1)
        self.assertRaises(TypeError, lambda: 1 ^ a)
        self.assertEqual(a, bitarray('11001'))
        self.assertEqual(b, bitarray('10011'))

    def test_iand(self):
        a = bitarray('110010110')
        b = bitarray('100110011')
        a &= b
        self.assertEqual(a, bitarray('100010010'))
        self.assertEqual(b, bitarray('100110011'))
        self.check_obj(a)
        self.check_obj(b)
        try:
            a &= 1
        except TypeError:
            error = 1
        self.assertEqual(error, 1)

    def test_ior(self):
        a = bitarray('110010110')
        b = bitarray('100110011')
        a |= b
        self.assertEQUAL(a, bitarray('110110111'))
        self.assertEQUAL(b, bitarray('100110011'))
        try:
            a |= 1
        except TypeError:
            error = 1
        self.assertEqual(error, 1)

    def test_ixor(self):
        a = bitarray('110010110')
        b = bitarray('100110011')
        a ^= b
        self.assertEQUAL(a, bitarray('010100101'))
        self.assertEQUAL(b, bitarray('100110011'))
        try:
            a ^= 1
        except TypeError:
            error = 1
        self.assertEqual(error, 1)

    def test_bitwise_self(self):
        for a in self.randombitarrays():
            aa = a.copy()
            self.assertEQUAL(a & a, aa)
            self.assertEQUAL(a | a, aa)
            self.assertEQUAL(a ^ a, zeros(len(aa), aa.endian))
            self.assertEQUAL(a, aa)

    def test_bitwise_inplace_self(self):
        for a in self.randombitarrays():
            aa = a.copy()
            a &= a
            self.assertEQUAL(a, aa)
            a |= a
            self.assertEQUAL(a, aa)
            a ^= a
            self.assertEqual(a, zeros(len(aa), aa.endian))

    def test_invert(self):
        a = bitarray('11011')
        b = ~a
        self.assertEQUAL(b, bitarray('00100'))
        self.assertEQUAL(a, bitarray('11011'))
        self.assertFalse(a is b)
        self.check_obj(b)

        for a in self.randombitarrays():
            b = bitarray(a)
            b.invert()
            for i in range(len(a)):
                self.assertEqual(b[i], not a[i])
            self.check_obj(b)
            self.assertEQUAL(~a, b)

    @staticmethod
    def shift(a, n, direction):
        if n >= len(a):
            return zeros(len(a), a.endian)

        if direction == 'right':
            return zeros(n, a.endian) + a[:len(a)-n]
        elif direction == 'left':
            return a[n:] + zeros(n, a.endian)
        else:
            raise ValueError("invalid direction: %s" % direction)

    def test_lshift(self):
        a = bitarray('11011')
        b = a << 2
        self.assertEQUAL(b, bitarray('01100'))
        self.assertRaises(TypeError, lambda: a << 1.2)
        self.assertRaises(TypeError, a.__lshift__, 1.2)
        self.assertRaises(ValueError, lambda: a << -1)
        self.assertRaises(OverflowError, a.__lshift__, 1 << 63)

        for a in self.randombitarrays():
            c = a.copy()
            n = randrange(len(a) + 4)
            b = a << n
            self.assertEqual(len(b), len(a))
            self.assertEQUAL(b, self.shift(a, n, 'left'))
            self.assertEQUAL(a, c)

    def test_rshift(self):
        a = bitarray('1101101')
        b = a >> 1
        self.assertEQUAL(b, bitarray('0110110'))
        self.assertRaises(TypeError, lambda: a >> 1.2)
        self.assertRaises(TypeError, a.__rshift__, 1.2)
        self.assertRaises(ValueError, lambda: a >> -1)

        for a in self.randombitarrays():
            c = a.copy()
            n = randrange(len(a) + 4)
            b = a >> n
            self.assertEqual(len(b), len(a))
            self.assertEQUAL(b, self.shift(a, n, 'right'))
            self.assertEQUAL(a, c)

    def test_ilshift(self):
        a = bitarray('110110101')
        a <<= 7
        self.assertEQUAL(a, bitarray('010000000'))
        self.assertRaises(TypeError, a.__ilshift__, 1.2)
        self.assertRaises(ValueError, a.__ilshift__, -3)

        for a in self.randombitarrays():
            b = a.copy()
            n = randrange(len(a) + 4)
            b <<= n
            self.assertEqual(len(b), len(a))
            self.assertEQUAL(b, self.shift(a, n, 'left'))

    def test_irshift(self):
        a = bitarray('110110111')
        a >>= 3
        self.assertEQUAL(a, bitarray('000110110'))
        self.assertRaises(TypeError, a.__irshift__, 1.2)
        self.assertRaises(ValueError, a.__irshift__, -4)

        for a in self.randombitarrays():
            b = a.copy()
            n = randrange(len(a) + 4)
            b >>= n
            self.assertEqual(len(b), len(a))
            self.assertEQUAL(b, self.shift(a, n, 'right'))

    def check_random(self, n, endian, n_shift, direction):
        a = urandom_2(n, endian)
        self.assertEqual(len(a), n)

        b = a.copy()
        if direction == 'left':
            b <<= n_shift
        else:
            b >>= n_shift
        self.assertEQUAL(b, self.shift(a, n_shift, direction))

    def test_shift_range(self):
        for endian in 'little', 'big':
            for direction in 'left', 'right':
                for n in range(0, 200):
                    self.check_random(n, endian, 1, direction)
                    self.check_random(n, endian, randint(0, n), direction)
                for n_shift in range(0, 100):
                    self.check_random(100, endian, n_shift, direction)

    def test_zero_shift(self):
        for a in self.randombitarrays():
            aa = a.copy()
            self.assertEQUAL(a << 0, aa)
            self.assertEQUAL(a >> 0, aa)
            a <<= 0
            self.assertEQUAL(a, aa)
            a >>= 0
            self.assertEQUAL(a, aa)

    def test_len_or_larger_shift(self):
        # ensure shifts with len(a) (or larger) result in all zero bitarrays
        for a in self.randombitarrays():
            c = a.copy()
            z = zeros(len(a), a.endian)
            n = randint(len(a), len(a) + 10)
            self.assertEQUAL(a << n, z)
            self.assertEQUAL(a >> n, z)
            self.assertEQUAL(a, c)
            a <<= n
            self.assertEQUAL(a, z)
            a = bitarray(c)
            a >>= n
            self.assertEQUAL(a, z)

    def test_shift_example(self):
        a = bitarray('0010011')
        self.assertEqual(a << 3, bitarray('0011000'))
        a >>= 4
        self.assertEqual(a, bitarray('0000001'))

    def test_frozenbitarray(self):
        a = frozenbitarray('0010011')
        b = a << 3
        self.assertEqual(b, bitarray('0011000'))
        self.assertEqual(type(b), frozenbitarray)
        self.assertRaises(TypeError, a.__ilshift__, 4)

    @skipIf(is_pypy)
    def test_imported(self):
        a = bytearray([0xf0, 0x01, 0x02, 0x0f])
        b = bitarray(buffer=a)
        self.assertFalse(b.readonly)
        # operate on imported (writable) buffer
        b[8:24] <<= 3
        self.assertEqual(a, bytearray([0xf0, 0x08, 0x10, 0x0f]))
        b[0:9] |= bitarray("0000 1100 1")
        self.assertEqual(a, bytearray([0xfc, 0x88, 0x10, 0x0f]))
        b[23:] ^= bitarray("1 1110 1110")
        self.assertEqual(a, bytearray([0xfc, 0x88, 0x11, 0xe1]))
        b[16:] &= bitarray("1111 0000 1111 0000")
        self.assertEqual(a, bytearray([0xfc, 0x88, 0x10, 0xe0]))
        b >>= 8
        self.assertEqual(a, bytearray([0x00, 0xfc, 0x88, 0x10]))

# --------------------------------   .extend()   ----------------------------

class ExtendTests(unittest.TestCase, Util):

    def test_wrong_args(self):
        a = bitarray()
        self.assertRaises(TypeError, a.extend)
        for x in None, 1, True, 24, 1.0:
            self.assertRaises(TypeError, a.extend, x)
        self.assertEqual(len(a), 0)
        self.check_obj(a)

    def test_bitarray(self):
        a = bitarray()
        a.extend(bitarray())
        self.assertEqual(a, bitarray())
        a.extend(bitarray('110'))
        self.assertEqual(a, bitarray('110'))
        a.extend(bitarray('1110'))
        self.assertEqual(a, bitarray('110 1110'))

        a = bitarray('00001111', endian='little')
        a.extend(bitarray('00100111', endian='big'))
        self.assertEqual(a, bitarray('00001111 00100111'))

    def test_bitarray_random(self):
        for a in self.randombitarrays():
            sa = a.to01()
            for b in self.randombitarrays():
                bb = b.copy()
                c = bitarray(a)
                c.extend(b)
                self.assertEqual(c.to01(), sa + bb.to01())
                self.assertEqual(c.endian, a.endian)
                self.assertEqual(len(c), len(a) + len(b))
                self.check_obj(c)
                # ensure b hasn't changed
                self.assertEQUAL(b, bb)

    def test_list(self):
        a = bitarray()
        a.extend([])
        self.assertEqual(a, bitarray())
        a.extend([0, 1, True, False])
        self.assertEqual(a, bitarray('0110'))
        self.assertRaises(ValueError, a.extend, [0, 1, 2])
        self.assertRaises(TypeError, a.extend, [0, 1, 'a'])
        self.assertEqual(a, bitarray('0110'))

        for a in self.randomlists():
            for b in self.randomlists():
                c = bitarray(a)
                c.extend(b)
                self.assertEqual(c.tolist(), a + b)
                self.check_obj(c)

    def test_range(self):
        a = bitarray()
        a.extend(range(2))
        self.assertEqual(a, bitarray('01'))
        self.check_obj(a)

    def test_sequence(self):
        lst = [0, 1, 0, 1, 1]
        for x in [lst, tuple(lst), bytes(lst), bytearray(lst),
                  array.array('b', lst)]:
            self.assertEqual(len(x), 5)  # sequences have len, iterables not
            a = bitarray()
            a.extend(x)
            self.assertEqual(a, bitarray("01011"))
            self.check_obj(a)

        lst.append(2)  # will raise ValueError
        for x in [lst, tuple(lst), bytes(lst), bytearray(lst),
                  array.array('b', lst)]:
            self.assertEqual(len(x), 6)
            a = bitarray()
            self.assertRaises(ValueError, a.extend, x)
            self.assertEqual(len(a), 0)
            self.check_obj(a)

    def test_generator_1(self):
        def gen(lst):
            for x in lst:
                yield x
        a = bitarray('0011')
        a.extend(gen([0, 1, False, True, 0]))
        self.assertEqual(a, bitarray('0011 01010'))
        self.assertRaises(ValueError, a.extend, gen([0, 1, 2]))
        self.assertRaises(TypeError, a.extend, gen([1, 0, None]))
        self.assertEqual(a, bitarray('0011 01010'))

        a = bytearray()
        a.extend(gen([0, 1, 255]))
        self.assertEqual(a, b'\x00\x01\xff')
        self.assertRaises(ValueError, a.extend, gen([0, 1, 256]))
        self.assertRaises(TypeError, a.extend, gen([1, 0, None]))
        self.assertEqual(a, b'\x00\x01\xff')

        for a in self.randomlists():
            def foo():
                for e in a:
                    yield e
            b = bitarray()
            b.extend(foo())
            self.assertEqual(b.tolist(), a)
            self.check_obj(b)

    def test_generator_2(self):
        def gen():
            for i in range(10):
                if i == 4:
                    raise KeyError
                yield i % 2

        for a in bitarray(), []:
            self.assertRaises(KeyError, a.extend, gen())
            self.assertEqual(list(a), [0, 1, 0, 1])

    def test_iterator_1(self):
        a = bitarray()
        a.extend(iter([]))
        self.assertEqual(a, bitarray())
        a.extend(iter([1, 1, 0, True, False]))
        self.assertEqual(a, bitarray('11010'))
        self.assertRaises(ValueError, a.extend, iter([1, 1, 0, 0, 2]))
        self.assertEqual(a, bitarray('11010'))

        for a in self.randomlists():
            for b in self.randomlists():
                c = bitarray(a)
                c.extend(iter(b))
                self.assertEqual(c.tolist(), a + b)
                self.check_obj(c)

    def test_iterator_2(self):
        a = bitarray()
        a.extend(itertools.repeat(True, 23))
        self.assertEqual(a, bitarray(23 * '1'))
        self.check_obj(a)

    def test_iterator_change(self):
        a = bitarray(1000)
        c = 0
        for i, x in enumerate(a):
            if i == 10:
                a.clear()
            c += 1
        self.assertEqual(c, 11)
        self.check_obj(a)

    def test_string01(self):
        a = bitarray()
        a.extend(str())
        a.extend('')
        self.assertEqual(a, bitarray())
        a.extend('0110111')
        self.assertEqual(a, bitarray('0110111'))
        self.assertRaises(ValueError, a.extend, '0011201')
        # ensure no bits got added after error was raised
        self.assertEqual(a, bitarray('0110111'))

        a = bitarray()
        self.assertRaises(ValueError, a.extend, 100 * '01' + '.')
        self.assertRaises(ValueError, a.extend, 100 * '01' + '\0')
        self.assertEqual(a, bitarray())

        for a in self.randomlists():
            for b in self.randomlists():
                c = bitarray(a)
                c.extend(''.join(str(x) for x in b))
                self.assertEqual(c, bitarray(a + b))
                self.check_obj(c)

    def test_string01_whitespace(self):
        a = bitarray()
        a.extend(whitespace)
        self.assertEqual(len(a), 0)
        a.extend('0 1\n0\r1\t0\v1_')
        self.assertEqual(a, bitarray('010101'))
        a += '_ 1\n0\r1\t0\v'
        self.assertEqual(a, bitarray('010101 1010'))
        self.check_obj(a)

    def test_self(self):
        for s in '', '1', '110', '00110111':
            a = bitarray(s)
            a.extend(a)
            self.assertEqual(a, bitarray(2 * s))

        for a in self.randombitarrays():
            endian = a.endian
            s = a.to01()
            a.extend(a)
            self.assertEqual(a.to01(), 2 * s)
            self.assertEqual(a.endian, endian)
            self.assertEqual(len(a), 2 * len(s))
            self.check_obj(a)

# ------------------------ Tests for bitarray methods -----------------------

class AllAnyTests(unittest.TestCase, Util):

    def test_all(self):
        a = bitarray()
        self.assertTrue(a.all())
        for s, r in ('0', False), ('1', True), ('01', False):
            self.assertTrue(bitarray(s).all() is r)

        for a in self.randombitarrays():
            self.assertTrue(a.all() is all(a))

        N = randint(1000, 2000)
        a = ones(N)
        self.assertTrue(a.all())
        a[N - 1] = 0
        self.assertFalse(a.all())

    def test_any(self):
        a = bitarray()
        self.assertFalse(a.any())
        for s, r in ('0', False), ('1', True), ('01', True):
            self.assertTrue(bitarray(s).any() is r)

        for a in self.randombitarrays():
            self.assertTrue(a.any() is any(a))

        N = randint(1000, 2000)
        a = zeros(N)
        self.assertFalse(a.any())
        a[N - 1] = 1
        self.assertTrue(a.any())

class AppendTests(unittest.TestCase, Util):

    def test_simple(self):
        a = bitarray()
        a.append(True)
        a.append(False)
        a.append(False)
        self.assertEQUAL(a, bitarray('100'))
        a.append(0)
        a.append(1)
        self.assertEQUAL(a, bitarray('10001'))
        self.check_obj(a)

    def test_wrong_args(self):
        a = bitarray("10001")
        self.assertRaises(ValueError, a.append, 2)
        self.assertRaises(TypeError, a.append, None)
        self.assertRaises(TypeError, a.append, '')
        self.assertEQUAL(a, bitarray('10001'))
        self.check_obj(a)

    def test_random(self):
        a = urandom_2(1000)
        b = bitarray(endian=a.endian)
        for i in range(len(a)):
            b.append(a[i])
            self.assertEQUAL(b, a[:i+1])
        self.check_obj(b)

class BufferInfoTests(unittest.TestCase):

    def test_values(self):
        for a, views, res in [

                (bitarray(11, endian='little'), 0,
                 (2, 'little', 5, 2, False, False, 0)),

                (bitarray(endian='big', buffer=b"ABC"), 2,
                 (3, 'big', 0, 0, True, True, 2)),

                (frozenbitarray("00100", 'big'), 5,
                 (1, 'big', 3, 1, True, False, 5)),
        ]:
            d = {}
            for i in range(views):
                d[i] = memoryview(a)
            self.assertEqual(len(d), views)

            info = a.buffer_info()
            self.assertEqual(info[1:8], res)
            self.assertEqual(info.nbytes, a.nbytes)
            self.assertEqual(info.endian, a.endian)
            self.assertEqual(info.padbits, a.padbits)
            self.assertEqual(info.readonly, a.readonly)
            self.assertEqual(info.exports, views)

    def test_types(self):
        a = urandom_2(57)
        info = a.buffer_info()
        self.assertTrue(isinstance(info, tuple))
        self.assertEqual(type(info), BufferInfo)
        self.assertEqual(len(info), 8)

        for i, (item, tp) in enumerate([
                (info.address, int),
                (info.nbytes, int),
                (info.endian, str),
                (info.padbits, int),
                (info.alloc, int),
                (info.readonly, bool),
                (info.imported, bool),
                (info.exports, int),
        ]):
            self.assertEqual(type(item), tp)
            self.assertTrue(info[i] is item)

class InsertTests(unittest.TestCase, Util):

    def test_basic(self):
        a = bitarray('00111')
        a.insert(0, 1)
        self.assertEqual(a, bitarray('1 00111'))
        a.insert(0, 0)
        self.assertEqual(a, bitarray('01 00111'))
        a.insert(2, 1)
        self.assertEqual(a, bitarray('011 00111'))

    def test_errors(self):
        a = bitarray('111100')
        self.assertRaises(ValueError, a.insert, 0, 2)
        self.assertRaises(TypeError, a.insert, 0, None)
        self.assertRaises(TypeError, a.insert)
        self.assertRaises(TypeError, a.insert, None)
        self.assertEqual(a, bitarray('111100'))
        self.check_obj(a)

    def test_random(self):
        for a in self.randombitarrays():
            aa = a.tolist()
            for _ in range(20):
                item = getrandbits(1)
                pos = randint(-len(a) - 5, len(a) + 5)
                a.insert(pos, item)
                aa.insert(pos, item)
            self.assertEqual(a.tolist(), aa)
            self.check_obj(a)

class FillTests(unittest.TestCase, Util):

    def test_simple(self):
        a = bitarray(endian=self.random_endian())
        self.assertEqual(a.fill(), 0)
        self.assertEqual(len(a), 0)

        a = bitarray('101', self.random_endian())
        self.assertEqual(a.fill(), 5)
        self.assertEqual(a, bitarray('10100000'))
        self.assertEqual(a.fill(), 0)
        self.assertEqual(a, bitarray('10100000'))
        self.check_obj(a)

    def test_exported(self):
        a = bitarray('11101')
        b = bitarray(buffer=a)
        v = memoryview(a)
        self.assertEqual(a.fill(), 3)
        self.assertEqual(a, b)
        self.assertEqual(v.nbytes, 1)

    def test_random(self):
        for a in self.randombitarrays():
            b = a.copy()
            res = b.fill()
            self.assertTrue(0 <= res < 8)
            self.assertTrue(b.padbits == 0)
            self.assertEqual(len(b) % 8, 0)
            self.assertEqual(b, a + zeros(res))
            self.assertEqual(b.endian, a.endian)
            self.check_obj(b)

class InvertTests(unittest.TestCase, Util):

    def test_simple(self):
        a = bitarray()
        a.invert()
        self.assertEQUAL(a, bitarray())
        self.check_obj(a)

        a = bitarray('11011')
        a.invert()
        self.assertEQUAL(a, bitarray('00100'))
        for i, res in [( 0, '10100'),
                       ( 4, '10101'),
                       ( 2, '10001'),
                       (-1, '10000'),
                       (-5, '00000')]:
            a.invert(i)
            self.assertEqual(a.to01(), res)

    def test_errors(self):
        a = bitarray(5)
        self.assertRaises(IndexError, a.invert, 5)
        self.assertRaises(IndexError, a.invert, -6)
        self.assertRaises(TypeError, a.invert, "A")
        self.assertRaises(TypeError, a.invert, 0, 1)
        self.assertFalse(a.any())
        self.check_obj(a)

    def test_random(self):
        for a in self.randombitarrays(start=1):
            n = len(a)
            b = a.copy()
            i = randint(-n, n - 1)
            a[i] = not a[i]
            b.invert(i)
            self.assertEQUAL(b, a)
            self.check_obj(b)

    def test_all(self):
        for a in self.randombitarrays():
            b = a.copy()
            a.invert()
            self.assertEqual(a, bitarray([not v for v in b]))
            self.assertEqual(a.endian, b.endian)
            self.check_obj(a)
            self.assertEQUAL(b, ~a)

    def test_span(self):
        for a in self.randombitarrays():
            n = len(a)
            b = a.copy()
            for _ in range(10):
                i = randint(0, n)
                j = randint(i, n)
                a.invert(slice(i, j))
                b[i:j] = ~b[i:j]
                self.assertEqual(a, b)

    def test_random_slice(self):
        for a in self.randombitarrays():
            n = len(a)
            b = a.copy()
            for _ in range(10):
                s = self.random_slice(n)
                a.invert(s)
                b[s] = ~b[s]
                self.assertEQUAL(a, b)

    @skipIf(is_pypy)
    def test_imported(self):
        a = bytearray([0, 1, 2, 3, 32, 255])
        b = bitarray(buffer=a)
        # operate on imported (writable) buffer
        self.assertFalse(b.readonly)
        b.invert()
        self.assertEqual(a, bytearray([255, 254, 253, 252, 223, 0]))

class SortTests(unittest.TestCase, Util):

    def test_simple(self):
        a = bitarray('1101000')
        a.sort()
        self.assertEqual(a, bitarray('0000111'))
        self.check_obj(a)

        a = bitarray('1101000')
        a.sort(reverse=True)
        self.assertEqual(a, bitarray('1110000'))
        a.sort(reverse=False)
        self.assertEqual(a, bitarray('0000111'))
        a.sort(True)
        self.assertEqual(a, bitarray('1110000'))
        a.sort(False)
        self.assertEqual(a, bitarray('0000111'))

        self.assertRaises(TypeError, a.sort, 'A')

    def test_random(self):
        for rev in False, True, 0, 1, 7, -1, -7, None:
            for a in self.randombitarrays():
                lst = a.tolist()
                if rev is None:
                    lst.sort()
                    a.sort()
                else:
                    lst.sort(reverse=rev)
                    a.sort(reverse=rev)
                self.assertEqual(a, bitarray(lst))
                self.check_obj(a)

    @skipIf(is_pypy)
    def test_imported(self):
        a = bytearray([0x6f, 0xa5])
        b = bitarray(endian="big", buffer=a)
        self.assertFalse(b.readonly)
        # operate on imported (writable) buffer
        b.sort()
        self.assertEqual(b.count(), 10)
        self.assertEqual(a, bytearray([0x03, 0xff]))

# -----------------------   .pack()   .unpack()   ---------------------------

class PackTests(unittest.TestCase, Util):

    def test_pack_simple(self):
        a = bitarray()
        a.pack(bytes())
        self.assertEQUAL(a, bitarray())
        a.pack(b'\x00')
        self.assertEQUAL(a, bitarray('0'))
        a.pack(b'\xff')
        self.assertEQUAL(a, bitarray('01'))
        a.pack(b'\x01\x00\x7a')
        self.assertEQUAL(a, bitarray('01101'))
        a.pack(bytearray([0x01, 0x00, 0xff, 0xa7]))
        self.assertEQUAL(a, bitarray('01101 1011'))
        self.check_obj(a)

    def test_pack_types(self):
        a = bitarray()
        a.pack(b'\0\x01')                        # bytes
        self.assertEqual(a, bitarray('01'))
        a.pack(bytearray([0, 2]))                # bytearray
        self.assertEqual(a, bitarray('01 01'))
        a.pack(memoryview(b'\x02\0'))            # memoryview
        self.assertEqual(a, bitarray('01 01 10'))

        a.pack(array.array('B', [0, 255, 192]))
        self.assertEqual(a, bitarray('01 01 10 011'))
        self.check_obj(a)

    def test_pack_bitarray(self):
        b = bitarray("00000000 00000001 10000000 11111111 00000000")
        a = bitarray()
        a.pack(bitarray(b))
        self.assertEqual(a, bitarray('01110'))
        self.check_obj(a)

    def test_pack_self(self):
        a = bitarray()
        self.assertRaisesMessage(
            BufferError,
            "cannot resize bitarray that is exporting buffers",
            a.pack, a)

    def test_pack_allbytes(self):
        a = bitarray()
        a.pack(bytearray(range(256)))
        self.assertEqual(a.to01(), '0' + 255 * '1')
        self.check_obj(a)

    def test_pack_errors(self):
        a = bitarray()
        self.assertRaises(TypeError, a.pack, 0)
        self.assertRaises(TypeError, a.pack, '1')
        self.assertRaises(TypeError, a.pack, [1, 3])

    def test_unpack_simple(self):
        a = bitarray('01')
        self.assertEqual(type(a.unpack()), bytes)
        self.assertEqual(a.unpack(), b'\x00\x01')
        self.assertEqual(a.unpack(b'A'), b'A\x01')
        self.assertEqual(a.unpack(b'0', b'1'), b'01')
        self.assertEqual(a.unpack(one=b'\xff'), b'\x00\xff')
        self.assertEqual(a.unpack(zero=b'A'), b'A\x01')
        self.assertEqual(a.unpack(one=b't', zero=b'f'), b'ft')

    def test_unpack_random(self):
        for a in self.randombitarrays():
            self.assertEqual(a.unpack(b'0', b'1'), a.to01().encode())
            # round trip
            b = bitarray()
            b.pack(a.unpack())
            self.assertEqual(b, a)
            # round trip with invert
            b = bitarray()
            b.pack(a.unpack(b'\x01', b'\x00'))
            b.invert()
            self.assertEqual(b, a)
            # use .extend() to pack
            b = bitarray()
            b.extend(a.unpack())
            self.assertEqual(b, a)

    def test_unpack_errors(self):
        a = bitarray('01')
        self.assertRaises(TypeError, a.unpack, b'')
        self.assertRaises(TypeError, a.unpack, b'0', b'')
        self.assertRaises(TypeError, a.unpack, b'a', zero=b'b')
        self.assertRaises(TypeError, a.unpack, foo=b'b')
        self.assertRaises(TypeError, a.unpack, one=b'aa', zero=b'b')
        self.assertRaises(TypeError, a.unpack, '0')
        self.assertRaises(TypeError, a.unpack, one='a')
        self.assertRaises(TypeError, a.unpack, b'0', '1')

class PopTests(unittest.TestCase, Util):

    def test_basic(self):
        a = bitarray('01')
        self.assertRaisesMessage(IndexError, "pop index out of range",
                                 a.pop, 2)
        self.assertEqual(a.pop(), True)
        self.assertEqual(a.pop(), False)
        self.assertEqual(a, bitarray())
        # pop from empty bitarray
        self.assertRaisesMessage(IndexError, "pop from empty bitarray", a.pop)

    def test_simple(self):
        for x, n, r, y in [('1',       0, 1, ''),
                           ('0',      -1, 0, ''),
                           ('0011100', 3, 1, '001100')]:
            a = bitarray(x)
            self.assertTrue(a.pop(n) is r)
            self.assertEqual(a, bitarray(y))
            self.check_obj(a)

    def test_reverse(self):
        for a in self.randombitarrays():
            c = a.copy()
            b = bitarray()
            while a:
                b.append(a.pop())
            self.assertEqual(a, bitarray())
            b.reverse()
            self.assertEqual(b, c)

    def test_random_1(self):
        for a in self.randombitarrays():
            self.assertRaises(IndexError, a.pop, len(a))
            self.assertRaises(IndexError, a.pop, -len(a) - 1)
            if len(a) == 0:
                continue
            aa = a.tolist()
            enda = a.endian
            self.assertEqual(a.pop(), aa[-1])
            self.check_obj(a)
            self.assertEqual(a.endian, enda)

    def test_random_2(self):
        for a in self.randombitarrays(start=1):
            n = randrange(-len(a), len(a))
            aa = a.tolist()
            x = a.pop(n)
            self.assertEqual(x, aa[n])
            self.assertEqual(type(x), int)
            y = aa.pop(n)
            self.assertEqual(a, bitarray(aa))
            self.assertEqual(x, y)
            self.check_obj(a)

class ReverseTests(unittest.TestCase, Util):

    def test_explicit(self):
        for x, y in [('', ''), ('1', '1'), ('10', '01'), ('001', '100'),
                     ('1110', '0111'), ('11100', '00111'),
                     ('011000', '000110'), ('1101100', '0011011'),
                     ('11110000', '00001111'),
                     ('11111000011', '11000011111')]:
            a = bitarray(x)
            a.reverse()
            self.assertEQUAL(a, bitarray(y))
            self.check_obj(a)

    def test_argument(self):
        a = bitarray(3)
        self.assertRaises(TypeError, a.reverse, 42)

    def test_random(self):
        for a in self.randombitarrays():
            b = a.copy()
            a.reverse()
            self.assertEqual(a.to01(), b.to01()[::-1])
            self.assertEQUAL(a, bitarray(reversed(b), endian=a.endian))
            self.assertEQUAL(a, b[::-1])
            self.check_obj(a)

    @skipIf(is_pypy)
    def test_imported(self):
        a = bytearray([0, 1, 2, 3, 255])
        b = bitarray(buffer=a)
        # reversing an imported (writable) buffer
        self.assertFalse(b.readonly)
        b.reverse()
        self.assertEqual(a, bytearray([255, 192, 64, 128, 0]))

class RemoveTests(unittest.TestCase, Util):

    def test_explicit(self):
        a = bitarray('1010110')
        for val, res in [(False, '110110'), (True, '10110'),
                         (1, '0110'), (1, '010'), (0, '10'),
                         (0, '1'), (1, '')]:
            a.remove(val)
            self.assertEQUAL(a, bitarray(res))
            self.check_obj(a)

    def test_errors(self):
        a = bitarray('0010011')
        a.remove(1)
        self.assertEQUAL(a, bitarray('000011'))
        self.assertRaises(TypeError, a.remove, 'A')
        self.assertRaises(ValueError, a.remove, 21)
        self.assertEQUAL(a, bitarray('000011'))

        a = bitarray()
        for i in (True, False, 1, 0):
            self.assertRaises(ValueError, a.remove, i)

        a = zeros(21)
        self.assertRaises(ValueError, a.remove, 1)
        a.setall(1)
        self.assertRaises(ValueError, a.remove, 0)

    def test_random(self):
        for a in self.randombitarrays():
            b = a.tolist()
            v = getrandbits(1)
            if v not in a:
                continue
            a.remove(v)
            b.remove(v)
            self.assertEqual(a.tolist(), b)
            self.check_obj(a)

class SetAllTests(unittest.TestCase, Util):

    def test_explicit(self):
        a = urandom_2(5)
        a.setall(True)
        self.assertRaises(ValueError, a.setall, -1)
        self.assertRaises(TypeError, a.setall, None)
        self.assertEqual(a.to01(), '11111')
        a.setall(0)
        self.assertEqual(a.to01(), '00000')
        self.check_obj(a)

    def test_empty(self):
        a = bitarray()
        for v in 0, 1:
            a.setall(v)
            self.assertEqual(len(a), 0)
            self.check_obj(a)

    def test_random(self):
        for a in self.randombitarrays():
            endian = a.endian
            val = getrandbits(1)
            a.setall(val)
            self.assertEqual(a.to01(), len(a) * str(val))
            self.assertEqual(a.endian, endian)
            self.check_obj(a)

    @skipIf(is_pypy)
    def test_imported(self):
        a = bytearray([0, 1, 2, 3])
        b = bitarray(buffer=a)
        self.assertFalse(b.readonly)
        # operate on imported (writable) buffer
        b.setall(1)
        self.assertEqual(a, bytearray([0xff, 0xff, 0xff, 0xff]))

class To01Tests(unittest.TestCase, Util):

    def test_no_grouping(self):
        a = bitarray()
        self.assertEqual(a.to01(1), "")

        a = bitarray("100011110")
        for s in [a.to01(), a.to01(0), a.to01(0, "X"), a.to01(1, ""),
                  a.to01(group=0), a.to01(sep="X"), a.to01(group=2, sep="")]:
            self.assertEqual(type(s), str)
            self.assertEqual(len(s), len(a))
            self.assertEqual(s, "100011110")

    def test_examples(self):
        a = bitarray("0000 1111 0011 0101")
        self.assertEqual(a.to01(1, "-"), "0-0-0-0-1-1-1-1-0-0-1-1-0-1-0-1")
        self.assertEqual(a.to01(2, sep='+'), "00+00+11+11+00+11+01+01")
        self.assertEqual(a.to01(3), "000 011 110 011 010 1")
        self.assertEqual(a.to01(group=4, sep="_"), "0000_1111_0011_0101")
        self.assertEqual(a.to01(group=5, sep='.'), "00001.11100.11010.1")
        self.assertEqual(a.to01(group=6), "000011 110011 0101")
        self.assertEqual(a.to01(7), "0000111 1001101 01")
        self.assertEqual(a.to01(8, ", "), "00001111, 00110101")
        self.assertEqual(a.to01(9, "ABC"), "000011110ABC0110101")

    def test_wrong_args(self):
        a = bitarray("1101100")
        self.assertRaises(TypeError, a.to01, None)
        self.assertRaises(ValueError, a.to01, -1)
        self.assertRaises(TypeError, a.to01, foo=4)
        self.assertRaises(TypeError, a.to01, 2, None)
        self.assertRaises(TypeError, a.to01, 4, b"_")

    def test_sep(self):
        for a in self.randombitarrays():
            sep = "".join(chr(randint(32, 126))
                          for _ in range(randrange(10)))
            self.assertEqual(a.to01(1, sep), sep.join(str(v) for v in a))

        a = bitarray("11100111")
        # use unicode character black star as separator
        s = a.to01(3, "\u2605")
        self.assertEqual(s, "111\u2605001\u260511")

    def test_random(self):
        for a in self.randombitarrays():
            n = len(a)
            group = randrange(10)
            nsep = randrange(6)
            s = a.to01(group, nsep * " ")
            self.assertEqual(a, bitarray(s))
            nspace = s.count(" ")
            self.assertEqual(len(s), n + nspace)
            self.assertEqual(nspace,
                             nsep * ((n - 1) // group) if group and n else 0)

class ByteReverseTests(unittest.TestCase, Util):

    def test_explicit_all(self):
        for x, y in [('', ''),
                     ('11101101', '10110111'),
                     ('00000001', '10000000'),
                     ('11011111 00100000 00011111',
                      '11111011 00000100 11111000')]:
            a = bitarray(x)
            a.bytereverse()
            self.assertEqual(a, bitarray(y))

    def test_explicit_range(self):
        a = bitarray('11100000 00000011 00111111 11111000')
        a.bytereverse(0, 1)  # reverse byte 0
        self.assertEqual(a, bitarray('00000111 00000011 00111111 11111000'))
        a.bytereverse(1, -1)  # reverse bytes 1 and 2
        self.assertEqual(a, bitarray('00000111 11000000 11111100 11111000'))
        a.bytereverse(2)  # reverse bytes 2 till end of buffer
        self.assertEqual(a, bitarray('00000111 11000000 00111111 00011111'))
        a.bytereverse(-1)  # reverse last byte
        self.assertEqual(a, bitarray('00000111 11000000 00111111 11111000'))
        a.bytereverse(3, 1)  # start > stop (nothing to reverse)
        self.assertEqual(a, bitarray('00000111 11000000 00111111 11111000'))
        a.bytereverse(0, 4)  # reverse all bytes
        self.assertEqual(a, bitarray('11100000 00000011 11111100 00011111'))
        a.bytereverse(-2)  # last two bytes
        self.assertEqual(a, bitarray('11100000 00000011 00111111 11111000'))

        self.assertRaises(IndexError, a.bytereverse, -5)
        self.assertRaises(IndexError, a.bytereverse, 0, -5)
        self.assertRaises(IndexError, a.bytereverse, 5)
        self.assertRaises(IndexError, a.bytereverse, 0, 5)

    def test_byte(self):
        for i in range(256):
            a = bitarray(bytearray([i]))
            self.assertEqual(len(a), 8)
            b = a.copy()
            b.bytereverse()
            self.assertEqual(b, a[::-1])
            a.reverse()
            self.assertEqual(b, a)
            self.check_obj(b)

    def test_consecutive(self):
        for a in self.randombitarrays():
            b = a.copy()
            # two consecutive calls to .bytereverse() leave the bitarray
            # unchanged (even when the length is not a multiple of 8).
            a.bytereverse()
            a.bytereverse()
            self.assertEQUAL(a, b)

    def test_random(self):
        t = bitarray(bytearray(range(256)), self.random_endian())
        t.bytereverse()
        table = t.tobytes()  # translation table
        self.assertEqual(table[:9], b'\x00\x80\x40\xc0\x20\xa0\x60\xe0\x10')

        for n in range(100):
            a = urandom_2(8 * n)
            i = randint(0, n)  # start
            j = randint(0, n)  # stop
            b = a.copy()
            memoryview(b)[i:j] = b.tobytes()[i:j].translate(table)
            a.bytereverse(i, j)
            self.assertEQUAL(a, b)
            self.check_obj(a)

    def test_endian(self):
        for n in range(20):
            a = urandom_2(8 * n)
            b = a.copy()
            a.bytereverse()
            a = bitarray(a, self.opposite_endian(a.endian))
            self.assertEqual(a.tobytes(), b.tobytes())

    @skipIf(is_pypy)
    def test_imported(self):
        a = bytearray([0, 1, 2, 3, 255])
        b = bitarray(buffer=a)
        # operate on imported (writable) buffer
        self.assertFalse(b.readonly)
        b.bytereverse()
        self.assertEqual(a, bytearray([0, 128, 64, 192, 255]))

class ToListTests(unittest.TestCase, Util):

    def test_empty(self):
        a = bitarray()
        self.assertEqual(a.tolist(), [])

    def test_simple(self):
        a = bitarray('110')
        lst = a.tolist()
        self.assertEqual(type(lst), list)
        self.assertEqual(lst, [1, 1, 0])
        for item in lst:
            self.assertEqual(type(item), int)

    def test_random(self):
        for a in self.randombitarrays():
            res = a.tolist()
            self.assertEqual(res, list(a))
            self.assertEqual(res, [int(v) for v in a.to01()])

class ClearTests(unittest.TestCase, Util):

    def test_simple(self):
        a = bitarray("1110000001001000011111")
        a.clear()
        self.assertEqual(len(a), 0)

    def test_random(self):
        for a in self.randombitarrays():
            endian = a.endian
            a.clear()
            self.assertFalse(a)
            self.assertEqual(len(a), 0)
            self.assertEqual(a.endian, endian)
            self.check_obj(a)

# -------------------------------- .count() ---------------------------------

class CountTests(unittest.TestCase, Util):

    def test_basic(self):
        a = bitarray('10011')
        self.assertEqual(a.count(), 3)
        self.assertEqual(a.count(True), 3)
        self.assertEqual(a.count(False), 2)
        self.assertEqual(a.count(1), 3)
        self.assertEqual(a.count(0), 2)
        self.assertEqual(a.count(0, 5, 0, -1), 2)
        self.assertEqual(a.count(bitarray('0')), 2)
        self.assertEqual(a.count(bitarray('00')), 1)
        self.assertRaises(ValueError, a.count, 2)
        self.assertRaises(ValueError, a.count, 1, 0, 5, 0)
        self.assertRaises(TypeError, a.count, '')
        self.assertRaises(TypeError, a.count, 'A')
        self.assertRaises(TypeError, a.count, 1, 2.0)
        self.assertRaises(TypeError, a.count, 1, 2, 4.0)
        self.assertRaises(TypeError, a.count, 0, 'A')
        self.assertRaises(TypeError, a.count, 0, 0, 'A')

    def test_sub(self):
        a = bitarray('10011000 1110000')
        self.assertEqual(len(a), 15)
        self.assertEqual(a.count(bitarray('')), 16)
        self.assertEqual(a.count(bitarray('00')), 4)
        self.assertEqual(a.count(bitarray('11')), 2)
        self.assertEqual(a.count(bitarray('000')), 2)
        self.assertEqual(a.count(bitarray('000'), 8), 1)
        self.assertEqual(a.count(bitarray('000'), -3), 1)
        self.assertEqual(a.count(bitarray('000'), -4), 1)
        self.assertEqual(a.count(bitarray('000'), 4, -1), 2)
        self.assertEqual(a.count(bitarray('00'), -3), 1)
        self.assertEqual(a.count(bitarray('00'), -4), 2)
        self.assertRaises(ValueError, a.count, bitarray(''), 0, 15, 2)
        self.assertRaises(ValueError, a.count, bitarray('11'), 0, 15, 2)
        self.assertRaises(ValueError, a.count, bitarray('11'), 15, 0, -1)

    def test_random_sub(self):
        for _ in range(1000):
            n = randrange(100)
            a = urandom_2(n)
            s = a.to01()
            b = urandom_2(randrange(8))
            t = b.to01()
            i = randint(-n - 10, n + 10)
            j = randint(-n - 10, n + 10)
            self.assertEqual(a.count(b, i, j), s.count(t, i, j))

    def test_byte(self):
        for i in range(256):
            a = bitarray(bytearray([i]))
            self.assertEqual(len(a), 8)
            self.assertEqual(a.count(), bin(i)[2:].count('1'))

    def test_whole_range(self):
        for n in range(500):
            a = urandom_2(n)
            s = a.to01()
            for v in 0, 1:
                ref = s.count(str(v))
                self.assertEqual(a.count(v), ref)
                self.assertEqual(a.count(v, n, -n - 1, -1), ref)

    def test_sparse(self):
        n = 65536
        a = bitarray(n)
        indices = set(choices(range(n), k=256))
        a[list(indices)] = 1
        self.assertEqual(a.count(1), len(indices))
        self.assertEqual(a.count(0), n - len(indices))

        for _ in range(100):
            i = randrange(n)
            j = randrange(i, n)
            cnt = sum(1 for k in indices if i <= k < j)
            self.assertEqual(a.count(1, i, j), cnt)
            self.assertEqual(a.count(0, i, j), j - i - cnt)

    def test_step1(self):
        n = 300
        a = urandom_2(n)
        s = a.to01()
        for _ in range(1000):
            i = randrange(n)
            j = randrange(i, n)

            t = s[i:j]
            c0 = t.count('0')
            c1 = t.count('1')
            self.assertEqual(c0 + c1, j - i)

            self.assertEqual(a.count(0, i, j), c0)
            self.assertEqual(a.count(1, i, j), c1)

            b = a[i:j]
            self.assertEqual(b.count(0), c0)
            self.assertEqual(b.count(1), c1)

    def test_explicit(self):
        a = bitarray('01001100 01110011 01')
        self.assertEqual(a.count(), 9)
        self.assertEqual(a.count(0, 12), 3)
        self.assertEqual(a.count(1, 1, 18, 2), 6)
        self.assertEqual(a.count(1, 0, 18, 3), 2)
        self.assertEqual(a.count(1, 15, 4, -3), 2)
        self.assertEqual(a.count(1, -5), 3)
        self.assertEqual(a.count(1, 2, 17), 7)
        self.assertEqual(a.count(1, 6, 11), 2)
        self.assertEqual(a.count(0, 7, -3), 4)
        self.assertEqual(a.count(1, 1, -1), 8)
        self.assertEqual(a.count(1, 17, 14), 0)

    def test_random_slice(self):
        for n in range(500):
            a = urandom_2(n)
            v = randrange(2)
            s = self.random_slice(n)
            self.assertEqual(a.count(v, s.start, s.stop, s.step),
                             a[s].count(v))

    def test_offest_buffer(self):
        # this tests if words are aligned in popcnt_words()
        N = 1 << 16
        for i in range(20):
            a = urandom_2(N, 'little')
            b = bitarray(buffer=memoryview(a)[i:], endian='little')
            self.assertEqual(b.count(), a.count(1, 8 * i))

# -------------------------- .find() and .index() ---------------------------

class IndexTests(unittest.TestCase, Util):

    def test_errors(self):
        a = bitarray()
        for i in True, False, 1, 0:
            self.assertEqual(a.find(i), -1)
            self.assertRaises(ValueError, a.index, i)

        a = zeros(100)
        self.assertRaises(TypeError, a.find)
        self.assertRaises(TypeError, a.find, 1, 'a')
        self.assertRaises(TypeError, a.find, 1, 0, 'a')
        self.assertRaises(TypeError, a.find, 1, 0, 100, 'a')
        self.assertEqual(a.find(1, right=True), -1)

        self.assertRaises(ValueError, a.index, True)
        self.assertRaises(TypeError, a.index)
        self.assertRaises(TypeError, a.index, 1, 'a')
        self.assertRaises(TypeError, a.index, 1, 0, 'a')
        self.assertRaises(TypeError, a.index, 1, 0, 100, 'a')

    def test_explicit(self):
        a = bitarray('10011000 101000')
        for sub, start, stop, right, res in [
                ('',       7, 13, 0,  7),
                ('',      15, 99, 0, -1),
                ('0',      0, 99, 0,  1),
                ('1',      8, 12, 1, 10),
                ('1',    -99, -4, 1,  8),
                ('11',     0, 99, 0,  3),
                ('11',     4, 99, 0, -1),
                ('111',    0, 99, 1, -1),
                ('101',    0, 99, 1,  8),
                (a.to01(), 0, 99, 0,  0),
        ]:
            b = bitarray(sub, self.random_endian())
            self.assertEqual(a.find(b, start, stop, right), res)
            if res >= 0:
                self.assertEqual(a.index(b, start, stop, right), res)
            else:
                self.assertRaises(ValueError, a.index, start, stop, right)

            if len(b) == 1:
                self.assertEqual(a.find(b[0], start, stop, right), res)

    @staticmethod
    def find_empty(n, start=0, stop=sys.maxsize, right=0):
        """
        Return first (or rightmost (right=1)) index of an empty sequence
        inside a sequence S of length n with S[start:stop], or -1 when no
        empty sequence is found.
        """
        if start > n:
            return -1
        s = slice(start, stop, 1)
        start, stop, stride = s.indices(n)
        if start > stop:
            return -1
        return stop if right else start

    def test_find_empty(self):
        # test staticmethod .find_empty() against Python builtins
        for x in bytearray([0]), b"\0", "A":
            empty = 0 * x  # empty sequence
            self.assertEqual(len(empty), 0)
            for _ in range(50):
                n = randint(0, 5)
                z = n * x  # sequence of length n
                self.assertEqual(len(z), n)
                self.assertTrue(type(x) == type(empty) == type(z))

                self.assertEqual(z.find(empty), self.find_empty(n))
                self.assertEqual(z.rfind(empty), self.find_empty(n, right=1))

                start = randint(-5, 5)
                self.assertEqual(z.find(empty, start),
                                 self.find_empty(n, start))
                self.assertEqual(z.rfind(empty, start),
                                 self.find_empty(n, start, right=1))

                stop = randint(-5, 5)
                self.assertEqual(z.find(empty, start, stop),
                                 self.find_empty(n, start, stop))
                self.assertEqual(z.rfind(empty, start, stop),
                                 self.find_empty(n, start, stop, 1))

    def test_empty(self):
        # now that we have the established .find_empty(), we use it to
        # test .find() with an empty bitarray
        empty = bitarray()
        for _ in range(50):
            n = randint(0, 5)
            z = bitarray(n)
            right = getrandbits(1)
            self.assertEqual(z.find(empty, right=right),
                             self.find_empty(n, right=right))

            start = randint(-5, 5)
            self.assertEqual(z.find(empty, start, right=right),
                             self.find_empty(n, start, right=right))

            stop = randint(-5, 5)
            self.assertEqual(z.find(empty, start, stop, right),
                             self.find_empty(n, start, stop, right))

    def test_range_explicit(self):
        n = 150
        a = bitarray(n)
        for m in range(n):
            a.setall(0)
            self.assertRaises(ValueError, a.index, 1)
            self.assertEqual(a.find(1), -1)
            a[m] = 1
            self.assertEqual(a.index(1), m)
            self.assertEqual(a.find(1), m)

            a.setall(1)
            self.assertRaises(ValueError, a.index, 0)
            self.assertEqual(a.find(0), -1)
            a[m] = 0
            self.assertEqual(a.index(0), m)
            self.assertEqual(a.find(0), m)

    def test_random_start_stop(self):
        for _ in range(500):
            n = randrange(1, 200)
            a = zeros(n)
            plst = sorted(choices(range(n), k=9))
            a[plst] = 1
            # test without start and stop
            self.assertEqual(a.find(1, right=0), plst[0])
            self.assertEqual(a.find(1, right=1), plst[-1])
            start = randint(0, n)
            stop = randint(0, n)

            plst2 = [i for i in plst if start <= i < stop]
            if plst2:
                self.assertEqual(a.find(1, start, stop, 0), plst2[0])
                self.assertEqual(a.find(1, start, stop, 1), plst2[-1])
            else:
                right = getrandbits(1)
                self.assertEqual(a.find(1, start, stop, right), -1)

    def test_random_sub(self):  # test finding sub_bitarray
        for _ in range(200):
            n = randrange(1, 100)
            a = urandom_2(n)
            s = a.to01()
            self.assertEqual(a.find(a), 0)

            n = len(a)
            b = bitarray(randrange(10), self.random_endian())
            t = b.to01()
            self.assertEqual(a.find(b), s.find(t))

            i = randint(-n - 5, n + 5)
            j = randint(-n - 5, n + 5)
            ref_l = s.find(t, i, j)
            ref_r = s.rfind(t, i, j)

            self.assertEqual(ref_l == -1, ref_r == -1)
            self.assertEqual(a.find(b, i, j, 0), ref_l)
            self.assertEqual(a.find(b, i, j, 1), ref_r)

            if len(b) == 1:  # test finding int
                v = b[0]
                self.assertTrue(v in range(2))
                self.assertEqual(a.find(v, i, j, 0), ref_l)
                self.assertEqual(a.find(v, i, j, 1), ref_r)

# ----------------------------- .search() -----------------------------------

class SearchTests(unittest.TestCase, Util):

    def test_no_itersearch(self):
        a = bitarray()
        # removed in bitarray 3.0
        self.assertRaises(AttributeError, a.__getattribute__, 'itersearch')

    def test_simple(self):
        a = bitarray()
        for s in 0, 1, False, True, bitarray('0'), bitarray('1'):
            self.assertEqual(list(a.search(s)), [])

        a = bitarray('00100')
        for s in 1, True, bitarray('1'), bitarray('10'):
            self.assertEqual(list(a.search(s)), [2])

        a = 100 * bitarray('1')
        self.assertEqual(list(a.search(0)), [])
        self.assertEqual(list(a.search(1)), list(range(100)))

        self.assertRaises(TypeError, a.search, '010')

    def test_search_next(self):
        a = bitarray('10011')
        self.assertRaises(TypeError, a.search, '')
        it = a.search(1)
        self.assertIsType(it, 'searchiterator')
        self.assertEqual(next(it), 0)
        self.assertEqual(next(it), 3)
        self.assertEqual(next(it), 4)
        self.assertRaises(StopIteration, next, it)
        x = bitarray('11')
        it = a.search(x)
        del a, x
        self.assertEqual(next(it), 3)

    def test_search_empty(self):
        a = bitarray('10011')
        empty = bitarray()
        self.assertEqual(list(a.search(empty)), [0, 1, 2, 3, 4, 5])
        for start, stop, right, res in [
                (-9,  9, 0, [0, 1, 2, 3, 4, 5]),
                ( 1,  4, 0, [1, 2, 3, 4]),
                (-3, -2, 0, [2, 3]),
                (-1,  0, 1, []),
                ( 3,  3, 0, [3]),
                ( 4,  3, 0, []),
                ( 2,  2, 1, [2]),
                ( 2,  1, 1, []),
        ]:
            self.assertEqual(list(a.search(empty, start, stop, right)),
                             res)

    def test_explicit_1(self):
        a = bitarray('10011', self.random_endian())
        for s, res in [('0',     [1, 2]),  ('1', [0, 3, 4]),
                       ('01',    [2]),     ('11', [3]),
                       ('000',   []),      ('1001', [0]),
                       ('011',   [2]),     ('0011', [1]),
                       ('10011', [0]),     ('100111', [])]:
            b = bitarray(s, self.random_endian())
            self.assertEqual(list(a.search(b)), res)

    def test_explicit_2(self):
        a = bitarray('10010101 11001111 1001011')
        for s, res in [('011', [6, 11, 20]),
                       ('111', [7, 12, 13, 14]),  # note the overlap
                       ('1011', [5, 19]),
                       ('100', [0, 9, 16])]:
            b = bitarray(s)
            self.assertEqual(list(a.search(b)), res)

    def test_bool_random(self):
        for a in self.randombitarrays():
            b = a.copy()
            b.setall(0)
            b[list(a.search(1))] = 1
            self.assertEQUAL(b, a)

            b.setall(1)
            b[list(a.search(0))] = 0
            self.assertEQUAL(b, a)

            s = set(a.search(0)) | set(a.search(1))
            self.assertEqual(len(s), len(a))

    def test_random(self):
        for a in self.randombitarrays():
            if a:
                # search for a in itself
                self.assertEqual(list(a.search(a)), [0])
                self.assertEqual(list(a.search(a, right=1)), [0])

            for sub in '0', '1', '01', '01', '11', '101', '1101', '01100':
                b = bitarray(sub, self.random_endian())
                plst = [i for i in range(len(a)) if a[i:i + len(b)] == b]
                self.assertEqual(list(a.search(b)), plst)

                for p in a.search(b):
                    self.assertEqual(a[p:p + len(b)], b)
                self.assertEqual(list(a.search(b)), plst)

                for p in a.search(b, right=1):
                    self.assertEqual(a[p:p + len(b)], b)
                self.assertEqual(list(a.search(b, right=1)), plst[::-1])

    def test_search_random(self):
        for _ in range(500):
            n = randrange(1, 50)
            a = urandom_2(n)
            b = urandom_2(randrange(10))
            i = randrange(n)
            j = randrange(n)
            aa = a[i:j]
            # list of positions
            if b:
                plst = [i + k for k in range(len(aa))
                        if aa[k:k + len(b)] == b]
            else:  # empty sub-bitarray
                plst = list(range(i, j + 1))

            self.assertEqual(sorted(plst), plst)
            self.assertEqual(list(a.search(b, i, j)), plst)

            if len(b) == 1:  # test sub-bitarray being int
                self.assertEqual(list(a.search(b[0], i, j)), plst)

            if plst:  # test first and last using .find()
                self.assertEqual(a.find(b, i, j, 0), plst[0])
                self.assertEqual(a.find(b, i, j, 1), plst[-1])

            plst.reverse()
            self.assertEqual(list(a.search(b, i, j, 1)), plst)

            if len(b) == 1:  # test sub-bitarray being int
                self.assertEqual(list(a.search(b[0], i, j, 1)), plst)

            # test contains
            self.assertEqual(b in aa, bool(plst) if b else True)

            if not plst:  # test .find() not found
                right = getrandbits(1)
                self.assertEqual(a.find(b, i, j, right), -1)

    def test_iterator_change(self):
        for right in 0, 1:
            a = zeros(100)
            b = zeros(10)
            c = 0
            for i, x in enumerate(a.search(b, right=right)):
                if i == 40:
                    a.clear()
                c += 1
            self.assertEqual(c, 41)

    def test_iterator_change_sub(self):
        for right in 0, 1:
            a = zeros(100)
            b = zeros(0)
            c = 0
            for i, x in enumerate(a.search(b, right=right)):
                if i == 20:
                    b.append(1)
                c += 1
            self.assertEqual(c, 21)

# ------------------------ .frombytes() and .tobytes() ----------------------

class BytesTests(unittest.TestCase, Util):

    def test_frombytes_simple(self):
        a = bitarray("110", "big")
        a.frombytes(b'A')
        self.assertEqual(a, bitarray('110 01000001'))

        a.frombytes(b'BC')
        self.assertEQUAL(a, bitarray('110 01000001 01000010 01000011',
                                     endian='big'))

    def test_frombytes_types(self):
        a = bitarray(endian='big')
        a.frombytes(b'A')                           # bytes
        self.assertEqual(a, bitarray('01000001'))
        a.frombytes(bytearray([254]))               # bytearray
        self.assertEqual(a, bitarray('01000001 11111110'))
        a.frombytes(memoryview(b'C'))               # memoryview
        self.assertEqual(a, bitarray('01000001 11111110 01000011'))

        a.clear()
        arr = array.array('H', [0x010f, 0xff])      # array
        self.assertEqual(arr.itemsize, 2)
        if sys.byteorder == "big":
            arr.byteswap()
        a.frombytes(arr)
        self.assertEqual(a, bitarray("00001111 00000001 11111111 00000000"))
        self.check_obj(a)

        for x in '', 0, 1, False, True, None, []:
            self.assertRaises(TypeError, a.frombytes, x)

    def test_frombytes_bitarray(self):
        # endianness doesn't matter here as we're writting the buffer
        # from bytes, and then extend from buffer bytes again
        b = bitarray(0, self.random_endian())
        b.frombytes(b'ABC')

        a = bitarray(0, 'big')
        a.frombytes(bitarray(b))  # get bytes from bitarray buffer
        self.assertEqual(a.endian, 'big')
        self.assertEqual(a.tobytes(), b'ABC')
        self.check_obj(a)

    def test_frombytes_self(self):
        a = bitarray()
        self.assertRaisesMessage(
            BufferError, "cannot resize bitarray that is exporting buffers",
            a.frombytes, a)

    def test_frombytes_empty(self):
        for a in self.randombitarrays():
            b = a.copy()
            a.frombytes(b'')
            a.frombytes(bytearray())
            self.assertEQUAL(a, b)
            self.assertFalse(a is b)
            self.check_obj(a)

    def test_frombytes_errors(self):
        a = bitarray()
        self.assertRaises(TypeError, a.frombytes)
        self.assertRaises(TypeError, a.frombytes, b'', b'')
        self.assertRaises(TypeError, a.frombytes, 1)
        self.check_obj(a)

    def test_frombytes_random(self):
        for n in range(20):
            s = os.urandom(n)
            b = bitarray(0, self.random_endian())
            b.frombytes(s)
            self.assertEqual(len(b), 8 * n)
            for a in self.randombitarrays():
                c = bitarray(a, b.endian)
                c.frombytes(s)
                self.assertEqual(len(c), len(a) + 8 * n)
                self.assertEqual(c, a + b)
                self.check_obj(c)

    def test_tobytes_empty(self):
        a = bitarray()
        self.assertEqual(a.tobytes(), b'')

    def test_tobytes_endian(self):
        a = bitarray(endian=self.random_endian())
        a.frombytes(b'foo')
        self.assertEqual(a.tobytes(), b'foo')

        for n in range(20):
            s = os.urandom(n)
            a = bitarray(s, endian=self.random_endian())
            self.assertEqual(len(a), 8 * n)
            self.assertEqual(a.tobytes(), s)
            self.check_obj(a)

    def test_tobytes_explicit_ones(self):
        for n, s in [(1, b'\x01'), (2, b'\x03'), (3, b'\x07'), (4, b'\x0f'),
                     (5, b'\x1f'), (6, b'\x3f'), (7, b'\x7f'), (8, b'\xff'),
                     (12, b'\xff\x0f'), (15, b'\xff\x7f'), (16, b'\xff\xff'),
                     (17, b'\xff\xff\x01'), (24, b'\xff\xff\xff')]:
            a = ones(n, endian='little')
            self.assertEqual(a.tobytes(), s)

# -------------------------- test attributes --------------------------------

class DescriptorTests(unittest.TestCase, Util):

    def test_endian(self):
        for endian in "little", "big":
            a = bitarray('1101100', endian)
            self.assertEqual(a.endian, endian)
            self.assertEqual(type(a.endian), str)

    def test_nbytes_padbits(self):
        for n in range(50):
            a = bitarray(n)
            # .nbytes
            self.assertEqual(a.nbytes, bits2bytes(n))
            self.assertEqual(type(a.nbytes), int)
            # .padbits
            self.assertEqual(a.padbits, 8 * a.nbytes - n)
            self.assertTrue(0 <= a.padbits < 8)
            self.assertEqual(type(a.padbits), int)

    def test_readonly(self):
        a = bitarray('110')
        self.assertFalse(a.readonly)
        self.assertEqual(type(a.readonly), bool)

        b = frozenbitarray(a)
        self.assertTrue(b.readonly)
        self.assertEqual(type(b.readonly), bool)

# ---------------------------------------------------------------------------

class FileTests(unittest.TestCase, Util):

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        self.tmpfname = os.path.join(self.tmpdir, 'testfile')

    def tearDown(self):
        shutil.rmtree(self.tmpdir)

    def read_file(self):
        with open(self.tmpfname, 'rb') as fi:
            return fi.read()

    def assertFileSize(self, size):
        self.assertEqual(os.path.getsize(self.tmpfname), size)

    def test_pickle(self):
        d1 = {i: a for i, a in enumerate(self.randombitarrays())}
        with open(self.tmpfname, 'wb') as fo:
            pickle.dump(d1, fo)
        with open(self.tmpfname, 'rb') as fi:
            d2 = pickle.load(fi)
        for key in d1.keys():
            self.assertEQUAL(d1[key], d2[key])

    # pyodide has no dbm module
    @skipIf(pyodide)
    def test_shelve(self):
        d1 = shelve.open(self.tmpfname)
        stored = []
        for i, a in enumerate(self.randombitarrays()):
            key = str(i)
            d1[key] = a
            stored.append((key, a))
        d1.close()

        d2 = shelve.open(self.tmpfname)
        for k, v in stored:
            self.assertEQUAL(d2[k], v)
        d2.close()

    def test_fromfile_empty(self):
        with open(self.tmpfname, 'wb') as fo:
            pass
        self.assertFileSize(0)

        a = bitarray()
        with open(self.tmpfname, 'rb') as fi:
            a.fromfile(fi)
        self.assertEqual(a, bitarray())
        self.check_obj(a)

    def test_fromfile_Foo(self):
        with open(self.tmpfname, 'wb') as fo:
            fo.write(b'Foo')
        self.assertFileSize(3)

        a = bitarray(endian='big')
        with open(self.tmpfname, 'rb') as fi:
            a.fromfile(fi)
        self.assertEqual(a, bitarray('01000110 01101111 01101111'))

        a = bitarray(endian='little')
        with open(self.tmpfname, 'rb') as fi:
            a.fromfile(fi)
        self.assertEqual(a, bitarray('01100010 11110110 11110110'))

    def test_fromfile_wrong_args(self):
        a = bitarray()
        self.assertRaises(TypeError, a.fromfile)
        self.assertRaises(AttributeError, a.fromfile, 42)
        self.assertRaises(AttributeError, a.fromfile, 'bar')

        with open(self.tmpfname, 'wb') as fo:
            fo.write(b"ABC")
        with open(self.tmpfname, 'rb') as fi:
            self.assertRaises(TypeError, a.fromfile, fi, None)

    def test_fromfile_erros(self):
        with open(self.tmpfname, 'wb') as fo:
            fo.write(b'0123456789')
        self.assertFileSize(10)

        a = bitarray()
        with open(self.tmpfname, 'wb') as fi:
            self.assertRaisesMessage(UnsupportedOperation, "read",
                                     a.fromfile, fi)

        with open(self.tmpfname, 'r') as fi:
            self.assertRaisesMessage(TypeError, ".read() did not return "
                                     "'bytes', got 'str'", a.fromfile, fi)

    def test_frombytes_invalid_reader(self):
        class Reader:
            def read(self, n):
                return 12
        a = bitarray()
        f = Reader()
        self.assertRaisesMessage(TypeError, ".read() did not return "
                                 "'bytes', got 'int'", a.fromfile, f)

    def test_from_large_files(self):
        for N in range(65534, 65538):
            data = os.urandom(N)
            with open(self.tmpfname, 'wb') as fo:
                fo.write(data)

            a = bitarray()
            with open(self.tmpfname, 'rb') as fi:
                a.fromfile(fi)
            self.assertEqual(len(a), 8 * N)
            self.assertEqual(a.nbytes, N)
            self.assertEqual(a.tobytes(), data)
            self.check_obj(a)

    def test_fromfile_extend_existing(self):
        with open(self.tmpfname, 'wb') as fo:
            fo.write(b'Foo')

        foo_le = '01100010 11110110 11110110'

        for n in range(20):
            a = bitarray(n * '1', endian='little')
            with open(self.tmpfname, 'rb') as fi:
                a.fromfile(fi)
            self.assertEqual(a, bitarray(n * '1' + foo_le))
            self.check_obj(a)

    def test_fromfile_n(self):
        a = bitarray(b'ABCDEFGHIJ')
        with open(self.tmpfname, 'wb') as fo:
            a.tofile(fo)
        self.assertFileSize(10)

        with open(self.tmpfname, 'rb') as f:
            a = bitarray()
            a.fromfile(f, 0);  self.assertEqual(a.tobytes(), b'')
            a.fromfile(f, 1);  self.assertEqual(a.tobytes(), b'A')
            f.read(1)  # skip B
            a.fromfile(f, 1);  self.assertEqual(a.tobytes(), b'AC')
            a = bitarray()
            a.fromfile(f, 2);  self.assertEqual(a.tobytes(), b'DE')
            a.fromfile(f, 1);  self.assertEqual(a.tobytes(), b'DEF')
            a.fromfile(f, 0);  self.assertEqual(a.tobytes(), b'DEF')
            a.fromfile(f);     self.assertEqual(a.tobytes(), b'DEFGHIJ')
            a.fromfile(f);     self.assertEqual(a.tobytes(), b'DEFGHIJ')
            self.check_obj(a)

        a = bitarray()
        with open(self.tmpfname, 'rb') as f:
            f.read(1)
            self.assertRaises(EOFError, a.fromfile, f, 10)
        # check that although we received an EOFError, the bytes were read
        self.assertEqual(a.tobytes(), b'BCDEFGHIJ')

        a = bitarray()
        with open(self.tmpfname, 'rb') as f:
            # negative values - like ommiting the argument
            a.fromfile(f, -1)
            self.assertEqual(a.tobytes(), b'ABCDEFGHIJ')
            self.assertRaises(EOFError, a.fromfile, f, 1)

    def test_fromfile_BytesIO(self):
        f = BytesIO(b'somedata')
        a = bitarray()
        a.fromfile(f, 4)
        self.assertEqual(len(a), 32)
        self.assertEqual(a.tobytes(), b'some')
        a.fromfile(f)
        self.assertEqual(len(a), 64)
        self.assertEqual(a.tobytes(), b'somedata')
        self.check_obj(a)

    def test_tofile_empty(self):
        a = bitarray()
        with open(self.tmpfname, 'wb') as f:
            a.tofile(f)

        self.assertFileSize(0)

    def test_tofile_Foo(self):
        a = bitarray('0100011 001101111 01101111', endian='big')
        b = a.copy()
        with open(self.tmpfname, 'wb') as f:
            a.tofile(f)
        self.assertEQUAL(a, b)

        self.assertFileSize(3)
        self.assertEqual(self.read_file(), b'Foo')

    def test_tofile_random(self):
        for a in self.randombitarrays():
            with open(self.tmpfname, 'wb') as fo:
                a.tofile(fo)
            n = a.nbytes
            self.assertFileSize(n)
            raw = self.read_file()
            self.assertEqual(len(raw), n)
            self.assertEqual(raw, a.tobytes())

    def test_tofile_errors(self):
        n = 100
        a = bitarray(8 * n)
        self.assertRaises(TypeError, a.tofile)

        with open(self.tmpfname, 'wb') as f:
            a.tofile(f)
        self.assertFileSize(n)
        # write to closed file
        self.assertRaises(ValueError, a.tofile, f)

        with open(self.tmpfname, 'w') as f:
            self.assertRaises(TypeError, a.tofile, f)

        with open(self.tmpfname, 'rb') as f:
            self.assertRaises(Exception, a.tofile, f)

    def test_tofile_large(self):
        n = 100_000
        a = zeros(8 * n)
        a[2::37] = 1
        with open(self.tmpfname, 'wb') as f:
            a.tofile(f)
        self.assertFileSize(n)

        raw = self.read_file()
        self.assertEqual(len(raw), n)
        self.assertEqual(raw, a.tobytes())

    def test_tofile_ones(self):
        for n in range(20):
            a = n * bitarray('1', endian='little')
            with open(self.tmpfname, 'wb') as fo:
                a.tofile(fo)

            raw = self.read_file()
            self.assertEqual(len(raw), a.nbytes)
            # when we fill the pad bits in a, we can compare
            a.fill()
            b = bitarray(raw, endian='little')
            self.assertEqual(a, b)

    def test_tofile_BytesIO(self):
        for n in list(range(10)) + list(range(65534, 65538)):
            data = os.urandom(n)
            a = bitarray(data, 'big')
            self.assertEqual(a.nbytes, n)
            f = BytesIO()
            a.tofile(f)
            self.assertEqual(f.getvalue(), data)

    @skipIf(is_pypy)
    def test_mmap(self):
        with open(self.tmpfname, 'wb') as fo:
            fo.write(1000 * b'\0')

        with open(self.tmpfname, 'r+b') as f:  # see issue #141
            with mmap.mmap(f.fileno(), 0) as mapping:
                a = bitarray(buffer=mapping, endian='little')
                info = a.buffer_info()
                self.assertFalse(info.readonly)
                self.assertTrue(info.imported)
                self.assertEqual(a, zeros(8000))
                a[::2] = True
                # not sure this is necessary, without 'del a', I get:
                # BufferError: cannot close exported pointers exist
                del a

        self.assertEqual(self.read_file(), 1000 * b'\x55')

    # pyodide hits emscripten mmap bug
    @skipIf(pyodide or is_pypy)
    def test_mmap_2(self):
        with open(self.tmpfname, 'wb') as fo:
            fo.write(1000 * b'\x22')

        with open(self.tmpfname, 'r+b') as f:
            a = bitarray(buffer=mmap.mmap(f.fileno(), 0), endian='little')
            info = a.buffer_info()
            self.assertFalse(info.readonly)
            self.assertTrue(info.imported)
            self.assertEqual(a, 1000 * bitarray('0100 0100'))
            a[::4] = 1

        self.assertEqual(self.read_file(), 1000 * b'\x33')

    @skipIf(is_pypy)
    def test_mmap_readonly(self):
        with open(self.tmpfname, 'wb') as fo:
            fo.write(994 * b'\x89' + b'Veedon')

        with open(self.tmpfname, 'rb') as fi:  # readonly
            m = mmap.mmap(fi.fileno(), 0, access=mmap.ACCESS_READ)
            a = bitarray(buffer=m, endian='big')
            info = a.buffer_info()
            self.assertTrue(info.readonly)
            self.assertTrue(info.imported)
            self.assertRaisesMessage(TypeError,
                                     "cannot modify read-only memory",
                                     a.__setitem__, 0, 1)
            self.assertEqual(a[:8 * 994], 994 * bitarray('1000 1001'))
            self.assertEqual(a[8 * 994:].tobytes(), b'Veedon')

# ----------------------------- Decode Tree ---------------------------------

alphabet_code = {
    ' ': bitarray('001'),         '.': bitarray('0101010'),
    'a': bitarray('0110'),        'b': bitarray('0001100'),
    'c': bitarray('000011'),      'd': bitarray('01011'),
    'e': bitarray('111'),         'f': bitarray('010100'),
    'g': bitarray('101000'),      'h': bitarray('00000'),
    'i': bitarray('1011'),        'j': bitarray('0111101111'),
    'k': bitarray('00011010'),    'l': bitarray('01110'),
    'm': bitarray('000111'),      'n': bitarray('1001'),
    'o': bitarray('1000'),        'p': bitarray('101001'),
    'q': bitarray('00001001101'), 'r': bitarray('1101'),
    's': bitarray('1100'),        't': bitarray('0100'),
    'u': bitarray('000100'),      'v': bitarray('0111100'),
    'w': bitarray('011111'),      'x': bitarray('0000100011'),
    'y': bitarray('101010'),      'z': bitarray('00011011110')
}

class DecodeTreeTests(unittest.TestCase, Util):

    def test_create(self):
        dt = decodetree(alphabet_code)
        self.assertEqual(type(dt), decodetree)
        self.assertRaises(TypeError, decodetree, None)
        self.assertRaises(TypeError, decodetree, 'foo')
        d = dict(alphabet_code)
        d['-'] = bitarray()
        self.assertRaises(ValueError, decodetree, d)

    def test_ambiguous_code(self):
        for d in [
            {'a': bitarray('0'), 'b': bitarray('0'), 'c': bitarray('1')},
            {'a': bitarray('01'), 'b': bitarray('01'), 'c': bitarray('1')},
            {'a': bitarray('0'), 'b': bitarray('01')},
            {'a': bitarray('0'), 'b': bitarray('11'), 'c': bitarray('111')},
        ]:
            self.assertRaises(ValueError, decodetree, d)

    @skipIf(is_pypy)
    def test_sizeof(self):
        dt = decodetree({'.': bitarray('1')})
        self.assertTrue(0 < sys.getsizeof(dt) < 100)

        dt = decodetree({'a': zeros(20)})
        self.assertTrue(sys.getsizeof(dt) > 200)

    def test_nodes(self):
        for n in range(1, 20):
            dt = decodetree({'a': zeros(n)})
            self.assertEqual(dt.nodes(), n + 1)
            self.assertFalse(dt.complete())

        dt = decodetree({'I': bitarray('1'),   'l': bitarray('01'),
                         'a': bitarray('001'), 'n': bitarray('000')})
        self.assertEqual(dt.nodes(), 7)
        dt = decodetree(alphabet_code)
        self.assertEqual(dt.nodes(), 70)

    def test_complete(self):
        dt = decodetree({'.': bitarray('1')})
        self.assertEqual(type(dt.complete()), bool)
        self.assertFalse(dt.complete())

        dt = decodetree({'a': bitarray('0'),
                         'b': bitarray('1')})
        self.assertTrue(dt.complete())

        dt = decodetree({'a': bitarray('0'),
                         'b': bitarray('11')})
        self.assertFalse(dt.complete())

        dt = decodetree({'a': bitarray('0'),
                         'b': bitarray('11'),
                         'c': bitarray('10')})
        self.assertTrue(dt.complete())

    def test_todict(self):
        t = decodetree(alphabet_code)
        d = t.todict()
        self.assertEqual(type(d), dict)
        self.assertEqual(d, alphabet_code)

    def test_decode(self):
        t = decodetree(alphabet_code)
        a = bitarray('1011 01110 0110 1001')
        self.assertEqual(list(a.decode(t)), ['i', 'l', 'a', 'n'])
        self.assertEqual(''.join(a.decode(t)), 'ilan')
        a = bitarray()
        self.assertEqual(list(a.decode(t)), [])
        self.check_obj(a)

    @skipIf(is_pypy)
    def test_large(self):
        d = {i: bitarray(bool((1 << j) & i) for j in range(10))
             for i in range(1024)}
        t = decodetree(d)
        self.assertEqual(t.todict(), d)
        self.assertEqual(t.nodes(), 2047)
        self.assertTrue(t.complete())
        self.assertTrue(sys.getsizeof(t) > 10000)

# ------------------ variable length encoding and decoding ------------------

class PrefixCodeTests(unittest.TestCase, Util):

    def test_encode_string(self):
        a = bitarray()
        a.encode(alphabet_code, '')
        self.assertEqual(a, bitarray())
        a.encode(alphabet_code, 'a')
        self.assertEqual(a, bitarray('0110'))

    def test_encode_list(self):
        a = bitarray()
        a.encode(alphabet_code, [])
        self.assertEqual(a, bitarray())
        a.encode(alphabet_code, ['e'])
        self.assertEqual(a, bitarray('111'))

    def test_encode_iter(self):
        a = bitarray()
        d = {0: bitarray('0'), 1: bitarray('1')}
        a.encode(d, iter([0, 1, 1, 0]))
        self.assertEqual(a, bitarray('0110'))

        def foo():
            for c in 1, 1, 0, 0, 1, 1:
                yield c

        a.clear()
        a.encode(d, foo())
        a.encode(d, range(2))
        self.assertEqual(a, bitarray('11001101'))
        self.assertEqual(d, {0: bitarray('0'), 1: bitarray('1')})

    def test_encode_symbol_not_in_code(self):
        d = dict(alphabet_code)
        a = bitarray()
        a.encode(d, 'is')
        self.assertEqual(a, bitarray('1011 1100'))
        self.assertRaises(ValueError, a.encode, d, 'ilAn')
        msg = "symbol not defined in prefix code: None"
        self.assertRaisesMessage(ValueError, msg, a.encode, d, [None, 2])

    def test_encode_not_iterable(self):
        d = {'a': bitarray('0'), 'b': bitarray('1')}
        a = bitarray()
        a.encode(d, 'abba')
        self.assertRaises(TypeError, a.encode, d, 42)
        self.assertRaises(TypeError, a.encode, d, 1.3)
        self.assertRaises(TypeError, a.encode, d, None)
        self.assertEqual(a, bitarray('0110'))

    def test_check_codedict_encode(self):
        a = bitarray()
        self.assertRaises(TypeError, a.encode, None, '')
        self.assertRaises(ValueError, a.encode, {}, '')
        self.assertRaises(TypeError, a.encode, {'a': 'b'}, 'a')
        self.assertRaises(ValueError, a.encode, {'a': bitarray()}, 'a')
        self.assertEqual(len(a), 0)

    def test_check_codedict_decode(self):
        a = bitarray('1100101')
        self.assertRaises(TypeError, a.decode, 0)
        self.assertRaises(ValueError, a.decode, {})
        self.assertRaises(TypeError, a.decode, {'a': 42})
        self.assertRaises(TypeError, a.decode, {'a': []})
        self.assertRaises(ValueError, a.decode, {'a': bitarray()})
        self.assertEqual(a, bitarray('1100101'))

    def test_no_iterdecode(self):
        a = bitarray()
        # removed in bitarray 3.0
        self.assertRaises(AttributeError, a.__getattribute__, 'iterdecode')

    def test_decode_simple(self):
        d = {'I': bitarray('1'),   'l': bitarray('01'),
             'a': bitarray('001'), 'n': bitarray('000')}
        dcopy = dict(d)
        a = bitarray('101001000')
        res = list("Ilan")
        self.assertEqual(list(a.decode(d)), res)
        self.assertEqual(d, dcopy)
        self.assertEqual(a, bitarray('101001000'))

    def test_decode_type(self):
        a = bitarray('0110')
        it = a.decode(alphabet_code)
        self.assertIsType(it, 'decodeiterator')
        self.assertEqual(list(it), ['a'])

    def test_decode_remove(self):
        d = {'I': bitarray('1'),   'l': bitarray('01'),
             'a': bitarray('001'), 'n': bitarray('000')}
        t = decodetree(d)
        a = bitarray('101001000')
        it = a.decode(t)
        del t  # remove tree
        self.assertEqual(''.join(it), "Ilan")

        it = a.decode(d)
        del a
        self.assertEqual(''.join(it), "Ilan")

    def test_decode_empty(self):
        d = {'a': bitarray('1')}
        a = bitarray()
        self.assertEqual(list(a.decode(d)), [])
        self.assertEqual(d, {'a': bitarray('1')})
        self.assertEqual(len(a), 0)

    def test_decode_incomplete(self):
        d = {'a': bitarray('0'), 'b': bitarray('111')}
        a = bitarray('00011')
        msg = "incomplete prefix code at position 3"
        self.assertRaisesMessage(ValueError, msg, list, a.decode(d))
        it = a.decode(d)
        self.assertIsType(it, 'decodeiterator')
        self.assertRaisesMessage(ValueError, msg, list, it)
        t = decodetree(d)
        self.assertRaisesMessage(ValueError, msg, list, a.decode(t))

        self.assertEqual(a, bitarray('00011'))
        self.assertEqual(d, {'a': bitarray('0'), 'b': bitarray('111')})
        self.assertEqual(t.todict(), d)

    def test_decode_incomplete_2(self):
        a = bitarray()
        a.encode(alphabet_code, "now we rise")
        x = len(a)
        a.extend('00')
        msg = "incomplete prefix code at position %d" % x
        self.assertRaisesMessage(ValueError, msg,
                                 list, a.decode(alphabet_code))

    def test_decode_no_term(self):
        d = {'a': bitarray('0'), 'b': bitarray('111')}
        a = bitarray('011')
        it = a.decode(d)
        self.assertEqual(next(it), 'a')
        self.assertRaisesMessage(ValueError,
                                 "incomplete prefix code at position 1",
                                 next, it)
        self.assertEqual(a, bitarray('011'))

    def test_decode_buggybitarray(self):
        d = dict(alphabet_code)
        #             i    s    t
        a = bitarray('1011 1100 0100 011110111001101001')
        msg = "prefix code unrecognized in bitarray at position 12 .. 21"
        self.assertRaisesMessage(ValueError, msg, list, a.decode(d))
        t = decodetree(d)
        self.assertRaisesMessage(ValueError, msg, list, a.decode(t))

        self.check_obj(a)
        self.assertEqual(t.todict(), d)

    def test_decode_buggybitarray2(self):
        d = {'a': bitarray('0')}
        a = bitarray('1')
        it = a.decode(d)
        self.assertRaises(ValueError, next, it)
        self.assertEqual(a, bitarray('1'))
        self.assertEqual(d, {'a': bitarray('0')})

    def test_decode_buggybitarray3(self):
        d = {'a': bitarray('00'), 'b': bitarray('01')}
        a = bitarray('1')
        self.assertRaises(ValueError, next, a.decode(d))

        t = decodetree(d)
        self.assertRaises(ValueError, next, a.decode(t))

        self.assertEqual(a, bitarray('1'))
        self.assertEqual(d, {'a': bitarray('00'), 'b': bitarray('01')})
        self.assertEqual(t.todict(), d)

    def test_decode_random(self):
        pat1 = re.compile(r'incomplete prefix code.+\s(\d+)')
        pat2 = re.compile(r'prefix code unrecognized.+\s(\d+)\s*\.\.\s*(\d+)')
        t = decodetree(alphabet_code)
        for a in self.randombitarrays():
            try:
                a.decode(t)
            except ValueError as e:
                msg = str(e)
                m1 = pat1.match(msg)
                m2 = pat2.match(msg)
                self.assertFalse(m1 and m2)
                if m1:
                    i = int(m1.group(1))
                if m2:
                    i, j = int(m2.group(1)), int(m2.group(2))
                    self.assertFalse(a[i:j] in alphabet_code.values())
                a[:i].decode(t)

    def test_decode_ambiguous_code(self):
        for d in [
            {'a': bitarray('0'), 'b': bitarray('0'), 'c': bitarray('1')},
            {'a': bitarray('01'), 'b': bitarray('01'), 'c': bitarray('1')},
            {'a': bitarray('0'), 'b': bitarray('01')},
            {'a': bitarray('0'), 'b': bitarray('11'), 'c': bitarray('111')},
        ]:
            a = bitarray()
            self.assertRaises(ValueError, a.decode, d)
            self.check_obj(a)

    def test_miscitems(self):
        d = {None : bitarray('00'),
             0    : bitarray('110'),
             1    : bitarray('111'),
             ''   : bitarray('010'),
             2    : bitarray('011')}
        a = bitarray()
        a.encode(d, [None, 0, 1, '', 2])
        self.assertEqual(a, bitarray('00110111010011'))
        self.assertEqual(list(a.decode(d)), [None, 0, 1, '', 2])
        # iterator
        it = a.decode(d)
        self.assertTrue(next(it) is None)
        self.assertEqual(next(it), 0)
        self.assertEqual(next(it), 1)
        self.assertEqual(next(it), '')
        self.assertEqual(next(it), 2)
        self.assertRaises(StopIteration, next, it)

    def test_quick_example(self):
        a = bitarray()
        message = 'the quick brown fox jumps over the lazy dog.'
        a.encode(alphabet_code, message)
        self.assertEqual(a, bitarray(
            # t    h     e       q           u      i    c      k
            '0100 00000 111 001 00001001101 000100 1011 000011 00011010 001'
            # b       r    o    w      n        f      o    x
            '0001100 1101 1000 011111 1001 001 010100 1000 0000100011 001'
            # j          u      m      p      s        o    v       e   r
            '0111101111 000100 000111 101001 1100 001 1000 0111100 111 1101'
            #     t    h     e       l     a    z           y
            '001 0100 00000 111 001 01110 0110 00011011110 101010 001'
            # d     o    g      .
            '01011 1000 101000 0101010'))
        self.assertEqual(''.join(a.decode(alphabet_code)), message)
        t = decodetree(alphabet_code)
        self.assertEqual(''.join(a.decode(t)), message)
        self.check_obj(a)

# --------------------------- Buffer Import ---------------------------------

class BufferImportTests(unittest.TestCase, Util):

    def test_bytes(self):
        b = 100 * b'\0'
        a = bitarray(buffer=b)

        info = a.buffer_info()
        self.assertEqual(info.alloc, 0)
        self.assertTrue(info.readonly)
        self.assertTrue(info.imported)

        self.assertRaises(TypeError, a.setall, 1)
        self.assertRaises(TypeError, a.clear)
        self.assertEqual(a, zeros(800))
        self.check_obj(a)

    @skipIf(is_pypy)
    def test_bytearray(self):
        b = bytearray(100 * [0])
        a = bitarray(buffer=b, endian='little')

        info = a.buffer_info()
        self.assertEqual(info.alloc, 0)
        self.assertFalse(info.readonly)
        self.assertTrue(info.imported)

        a[0] = 1
        self.assertEqual(b[0], 1)
        a[7] = 1
        self.assertEqual(b[0], 129)
        a[:] = 1
        self.assertEqual(b, bytearray(100 * [255]))
        self.assertRaises(BufferError, a.pop)
        a[8:16] = bitarray('10000010', endian='big')
        self.assertEqual(b, bytearray([255, 65] + 98 * [255]))
        self.assertEqual(a.tobytes(), b)
        for n in 7, 9:
            self.assertRaises(BufferError, a.__setitem__, slice(8, 16),
                              bitarray(n))
        b[1] = b[2] = 255
        self.assertEqual(b, bytearray(100 * [255]))
        self.assertEqual(a, 800 * bitarray('1'))
        self.check_obj(a)

    @skipIf(is_pypy)
    def test_array(self):
        a = array.array('B', [0, 255, 64])
        b = bitarray(None, 'little', a)
        self.assertEqual(b, bitarray('00000000 11111111 00000010'))
        a[1] = 32
        self.assertEqual(b, bitarray('00000000 00000100 00000010'))
        b[3] = 1
        self.assertEqual(a.tolist(), [8, 32, 64])
        self.check_obj(b)

    def test_bitarray(self):
        a = urandom_2(10000, 'little')
        b = bitarray(endian='little', buffer=a)
        # a and b are two distinct bitarrays that share the same buffer now
        self.assertFalse(a is b)

        a_info = a.buffer_info()
        self.assertFalse(a_info.imported)
        self.assertEqual(a_info.exports, 1)
        b_info = b.buffer_info()
        self.assertTrue(b_info.imported)
        self.assertEqual(b_info.exports, 0)
        # buffer address is the same
        self.assertEqual(a_info.address, b_info.address)

        self.assertFalse(a is b)
        self.assertEqual(a, b)
        b[437:461] = 0
        self.assertEqual(a, b)
        a[327:350] = 1
        self.assertEqual(a, b)
        b[101:1187] <<= 79
        self.assertEqual(a, b)
        a[100:9800:5] = 1
        self.assertEqual(a, b)

        self.assertRaisesMessage(
            BufferError, "cannot resize bitarray that is exporting buffers",
            a.pop)
        self.assertRaisesMessage(
            BufferError, "cannot resize imported buffer",
            b.pop)
        self.check_obj(a)
        self.check_obj(b)

    def test_copy(self):
        a = bitarray(buffer=b'XA')
        self.assertTrue(a.readonly)
        for b in [a.copy(), 3 * a, 5 * a, a & bitarray(16),
                  a >> 2, ~a, a + bitarray(8*'1'),
                  a[:], a[::2], a[[0, 1]], a[bitarray(16)]]:
            self.assertFalse(b.readonly)
            self.check_obj(b)

    @skipIf(is_pypy)
    def test_bitarray_shared_sections(self):
        a = urandom_2(0x2000, 'big')
        b = bitarray(buffer=memoryview(a)[0x100:0x300])
        self.assertEqual(b.buffer_info().address,
                         a.buffer_info().address + 0x100)
        c = bitarray(buffer=memoryview(a)[0x200:0x800])
        self.assertEqual(c.buffer_info().address,
                         a.buffer_info().address + 0x200)
        self.assertEqual(a[8 * 0x100 : 8 * 0x300], b)
        self.assertEqual(a[8 * 0x200 : 8 * 0x800], c)
        a.setall(0)
        b.setall(1)
        c.setall(0)

        d = bitarray(0x2000)
        d.setall(0)
        d[8 * 0x100 : 8 * 0x200] = 1
        self.assertEqual(a, d)

    def test_bitarray_range(self):
        for n in range(100):
            a = urandom_2(n)
            b = bitarray(buffer=a, endian=a.endian)
            # an imported buffer will never have any pad bits
            self.assertEqual(b.padbits, 0)
            self.assertEqual(len(b) % 8, 0)
            self.assertEQUAL(b[:n], a)
            self.check_obj(a)
            self.check_obj(b)

    def test_bitarray_chain(self):
        d = [urandom_2(64, 'big')]
        for n in range(1, 100):
            d.append(bitarray(endian='big', buffer=d[n - 1]))

        self.assertEqual(d[99], d[0])
        d[0].setall(1)
        for a in d:
            self.assertEqual(len(a), 64)
            self.assertTrue(a.all())
            self.check_obj(a)

    def test_frozenbitarray(self):
        a = frozenbitarray('10011011 011')
        self.assertTrue(a.readonly)
        self.check_obj(a)

        b = bitarray(buffer=a)
        self.assertTrue(b.readonly)  # also readonly
        self.assertRaises(TypeError, b.__setitem__, 1, 0)
        self.check_obj(b)

    def test_invalid_buffer(self):
        # these objects do not expose a buffer
        for arg in (123, 1.23, [1, 2, 3], (1, 2, 3), {1: 2},
                    set([1, 2, 3]),):
            self.assertRaises(TypeError, bitarray, buffer=arg)

    @skipIf(is_pypy)
    def test_del_import_object(self):
        b = bytearray(100 * [0])
        a = bitarray(buffer=b)
        del b
        self.assertEqual(a, zeros(800))
        a.setall(1)
        self.assertTrue(a.all())
        self.check_obj(a)

    @skipIf(is_pypy)
    def test_readonly_errors(self):
        a = bitarray(buffer=b'A')
        info = a.buffer_info()
        self.assertTrue(info.readonly)
        self.assertTrue(info.imported)

        self.assertRaises(TypeError, a.append, True)
        self.assertRaises(TypeError, a.bytereverse)
        self.assertRaises(TypeError, a.clear)
        self.assertRaises(TypeError, a.encode, {'a': bitarray('0')}, 'aa')
        self.assertRaises(TypeError, a.extend, [0, 1, 0])
        self.assertRaises(TypeError, a.fill)
        self.assertRaises(TypeError, a.frombytes, b'')
        self.assertRaises(TypeError, a.insert, 0, 1)
        self.assertRaises(TypeError, a.invert)
        self.assertRaises(TypeError, a.pack, b'\0\0\xff')
        self.assertRaises(TypeError, a.pop)
        self.assertRaises(TypeError, a.remove, 1)
        self.assertRaises(TypeError, a.reverse)
        self.assertRaises(TypeError, a.setall, 0)
        self.assertRaises(TypeError, a.sort)
        self.assertRaises(TypeError, a.__delitem__, 0)
        self.assertRaises(TypeError, a.__delitem__, slice(None, None, 2))
        self.assertRaises(TypeError, a.__setitem__, 0, 0)
        self.assertRaises(TypeError, a.__iadd__, bitarray(8))
        self.assertRaises(TypeError, a.__ior__, bitarray(8))
        self.assertRaises(TypeError, a.__ixor__, bitarray(8))
        self.assertRaises(TypeError, a.__irshift__, 1)
        self.assertRaises(TypeError, a.__ilshift__, 1)
        self.check_obj(a)

    @skipIf(is_pypy)
    def test_resize_errors(self):
        a = bitarray(buffer=bytearray([123]))
        info = a.buffer_info()
        self.assertFalse(info.readonly)
        self.assertTrue(info.imported)

        self.assertRaises(BufferError, a.append, True)
        self.assertRaises(BufferError, a.clear)
        self.assertRaises(BufferError, a.encode, {'a': bitarray('0')}, 'aa')
        self.assertRaises(BufferError, a.extend, [0, 1, 0])
        self.assertRaises(BufferError, a.frombytes, b'a')
        self.assertRaises(BufferError, a.insert, 0, 1)
        self.assertRaises(BufferError, a.pack, b'\0\0\xff')
        self.assertRaises(BufferError, a.pop)
        self.assertRaises(BufferError, a.remove, 1)
        self.assertRaises(BufferError, a.__delitem__, 0)
        self.check_obj(a)

# --------------------------- Buffer Export ---------------------------------

class BufferExportTests(unittest.TestCase, Util):

    def test_read_simple(self):
        a = bitarray('01000001 01000010 01000011', endian='big')
        v = memoryview(a)
        self.assertFalse(v.readonly)
        self.assertEqual(a.buffer_info().exports, 1)
        self.assertEqual(len(v), 3)
        self.assertEqual(v[0], 65)
        self.assertEqual(v.tobytes(), b'ABC')
        a[13] = 1
        self.assertEqual(v.tobytes(), b'AFC')

        w = memoryview(a)  # a second buffer export
        self.assertFalse(w.readonly)
        self.assertEqual(a.buffer_info().exports, 2)
        self.check_obj(a)

    def test_many_exports(self):
        a = bitarray('01000111 01011111')
        d = {}  # put bitarrays in dict to key object around
        for n in range(1, 20):
            d[n] = bitarray(buffer=a)
            self.assertEqual(a.buffer_info().exports, n)
            self.assertEqual(len(d[n]), 16)
        self.check_obj(a)

    def test_range(self):
        for n in range(100):
            a = bitarray(n)
            v = memoryview(a)
            self.assertEqual(len(v), a.nbytes)
            info = a.buffer_info()
            self.assertFalse(info.readonly)
            self.assertFalse(info.imported)
            self.assertEqual(info.exports, 1)
            self.check_obj(a)

    def test_read_random(self):
        a = bitarray(os.urandom(100))
        v = memoryview(a)
        self.assertEqual(len(v), 100)
        b = a[34 * 8 : 67 * 8]
        self.assertEqual(v[34:67].tobytes(), b.tobytes())
        self.assertEqual(v.tobytes(), a.tobytes())
        self.check_obj(a)

    def test_resize(self):
        a = bitarray('011', endian='big')
        v = memoryview(a)
        self.assertFalse(v.readonly)
        self.assertRaises(BufferError, a.append, 1)
        self.assertRaises(BufferError, a.clear)
        self.assertRaises(BufferError, a.encode, {'a': bitarray('0')}, 'aa')
        self.assertRaises(BufferError, a.extend, '0')
        self.assertRaises(BufferError, a.frombytes, b'\0')
        self.assertRaises(BufferError, a.insert, 0, 1)
        self.assertRaises(BufferError, a.pack, b'\0')
        self.assertRaises(BufferError, a.pop)
        self.assertRaises(BufferError, a.remove, 1)
        self.assertRaises(BufferError, a.__delitem__, slice(0, 8))
        a.fill()
        self.assertEqual(v.tobytes(), a.tobytes())
        self.check_obj(a)

    def test_frozenbitarray(self):
        a = frozenbitarray(40)
        v = memoryview(a)
        self.assertTrue(v.readonly)
        self.assertEqual(len(v), 5)
        self.assertEqual(v.tobytes(), a.tobytes())
        self.check_obj(a)

    def test_write(self):
        a = zeros(8000)
        v = memoryview(a)
        self.assertFalse(v.readonly)
        v[500] = 255
        self.assertEqual(a[3999:4009], bitarray('0111111110'))
        a[4003] = 0
        self.assertEqual(a[3999:4009], bitarray('0111011110'))
        v[301:304] = b'ABC'
        self.assertEqual(a[300 * 8 : 305 * 8].tobytes(), b'\x00ABC\x00')
        self.check_obj(a)

    def test_write_memoryview_slice(self):
        a = zeros(40)
        m = memoryview(a)
        v = m[1:4]
        v[0] = 65
        v[1] = 66
        v[2] = 67
        self.assertEqual(a.tobytes(), b'\x00ABC\x00')
        m[1:4] = b'XYZ'
        self.assertEqual(a.tobytes(), b'\x00XYZ\x00')
        self.check_obj(a)

# ---------------------------------------------------------------------------

class FrozenbitarrayTests(unittest.TestCase, Util):

    def test_init(self):
        a = frozenbitarray('110')
        self.assertEqual(a, bitarray('110'))
        self.assertEqual(a.to01(), '110')
        self.assertIsInstance(a, bitarray)
        self.assertEqual(type(a), frozenbitarray)
        self.assertTrue(a.readonly)
        self.check_obj(a)

        a = frozenbitarray(bitarray())
        self.assertEQUAL(a, frozenbitarray())
        self.assertEqual(type(a), frozenbitarray)

        for endian in 'big', 'little':
            a = frozenbitarray(0, endian)
            self.assertEqual(a.endian, endian)
            self.assertEqual(type(a), frozenbitarray)

            a = frozenbitarray(bitarray(0, endian))
            self.assertEqual(a.endian, endian)
            self.assertEqual(type(a), frozenbitarray)

    def test_methods(self):
        # test a few methods which do not raise the TypeError
        a = frozenbitarray('1101100')
        self.assertEqual(a[2], 0)
        self.assertEqual(a[:4].to01(), '1101')
        self.assertEqual(a.count(), 4)
        self.assertEqual(a.index(0), 2)
        b = a.copy()
        self.assertEqual(b, a)
        self.assertEqual(type(b), frozenbitarray)
        self.assertEqual(len(b), 7)
        self.assertFalse(b.all())
        self.assertTrue(b.any())
        self.check_obj(a)

    def test_init_from_bitarray(self):
        for a in self.randombitarrays():
            b = frozenbitarray(a)
            self.assertFalse(b is a)
            self.assertEQUAL(b, a)
            c = frozenbitarray(b)
            self.assertFalse(c is b)
            self.assertEQUAL(c, b)
            self.assertEqual(hash(c), hash(b))
            self.check_obj(b)

    def test_init_from_misc(self):
        tup = 0, 1, 0, 1, 1, False, True
        for obj in list(tup), tup, iter(tup), bitarray(tup):
            a = frozenbitarray(obj)
            self.assertEqual(type(a), frozenbitarray)
            self.assertEqual(a, bitarray(tup))
        a = frozenbitarray(b'AB', "big")
        self.assertEqual(a.to01(), "0100000101000010")
        self.assertEqual(a.endian, "big")

    def test_init_bytes_bytearray(self):
        for x in b'\x80ABCD', bytearray(b'\x80ABCD'):
            a = frozenbitarray(x, 'little')
            self.assertEqual(type(a), frozenbitarray)
            self.assertEqual(len(a), 8 * len(x))
            self.assertEqual(a.tobytes(), x)
            self.assertEqual(a[:8].to01(), '00000001')
            self.check_obj(a)

    def test_repr(self):
        a = frozenbitarray()
        self.assertEqual(repr(a), "frozenbitarray()")
        self.assertEqual(str(a), "frozenbitarray()")
        a = frozenbitarray('10111')
        self.assertEqual(repr(a), "frozenbitarray('10111')")
        self.assertEqual(str(a), "frozenbitarray('10111')")

    def test_immutable(self):
        a = frozenbitarray('111')
        self.assertRaises(TypeError, a.append, True)
        self.assertRaises(TypeError, a.bytereverse)
        self.assertRaises(TypeError, a.clear)
        self.assertRaises(TypeError, a.encode, {'a': bitarray('0')}, 'aa')
        self.assertRaises(TypeError, a.extend, [0, 1, 0])
        self.assertRaises(TypeError, a.fill)
        self.assertRaises(TypeError, a.frombytes, b'')
        self.assertRaises(TypeError, a.insert, 0, 1)
        self.assertRaises(TypeError, a.invert)
        self.assertRaises(TypeError, a.pack, b'\0\0\xff')
        self.assertRaises(TypeError, a.pop)
        self.assertRaises(TypeError, a.remove, 1)
        self.assertRaises(TypeError, a.reverse)
        self.assertRaises(TypeError, a.setall, 0)
        self.assertRaises(TypeError, a.sort)
        self.assertRaises(TypeError, a.__delitem__, 0)
        self.assertRaises(TypeError, a.__delitem__, slice(None, None, 2))
        self.assertRaises(TypeError, a.__setitem__, 0, 0)
        self.assertRaises(TypeError, a.__iadd__, bitarray('010'))
        self.assertRaises(TypeError, a.__ior__, bitarray('100'))
        self.assertRaises(TypeError, a.__ixor__, bitarray('110'))
        self.assertRaises(TypeError, a.__irshift__, 1)
        self.assertRaises(TypeError, a.__ilshift__, 1)
        self.check_obj(a)

    def test_copy(self):
        a = frozenbitarray('101')
        # not only .copy() creates new frozenbitarray which are read-only
        for b in [a, a.copy(), 3 * a, 5 * a, a & bitarray('110'),
                  a >> 2, ~a, a + bitarray(8*'1'),
                  a[:], a[::2], a[[0, 1]], a[bitarray('011')]]:
            self.assertEqual(type(b), frozenbitarray)
            self.assertTrue(b.readonly)
            self.check_obj(b)

    def test_freeze(self):
        # not so much a test for frozenbitarray, but how it is initialized
        a = bitarray(78)
        self.assertFalse(a.readonly)  # not readonly
        a._freeze()
        self.assertTrue(a.readonly)   # readonly

    def test_memoryview(self):
        a = frozenbitarray('01000001 01000010', 'big')
        v = memoryview(a)
        self.assertEqual(v.tobytes(), b'AB')
        self.assertRaises(TypeError, v.__setitem__, 0, 0x7c)

    def test_buffer_import_readonly(self):
        b = bytes([15, 95, 128])
        a = frozenbitarray(buffer=b, endian='big')
        self.assertEQUAL(a, bitarray('00001111 01011111 10000000', 'big'))
        info = a.buffer_info()
        self.assertTrue(info.readonly)
        self.assertTrue(info.imported)

    @skipIf(is_pypy)
    def test_buffer_import_writable(self):
        c = bytearray([15, 95])
        self.assertRaisesMessage(
            TypeError,
            "cannot import writable buffer into frozenbitarray",
            frozenbitarray, buffer=c)

    def test_as_set(self):
        a = frozenbitarray('1')
        b = frozenbitarray('11')
        c = frozenbitarray('01')
        d = frozenbitarray('011')
        s = set([a, b, c, d])
        self.assertEqual(len(s), 4)
        self.assertTrue(d in s)
        self.assertFalse(frozenbitarray('0') in s)

    def test_as_dictkey(self):
        a = frozenbitarray('01')
        b = frozenbitarray('1001')
        d = {a: 123, b: 345}
        self.assertEqual(d[frozenbitarray('01')], 123)
        self.assertEqual(d[frozenbitarray(b)], 345)

    def test_as_dictkey2(self):  # taken slightly modified from issue #74
        a1 = frozenbitarray("10")
        a2 = frozenbitarray("00")
        dct = {a1: "one", a2: "two"}
        a3 = frozenbitarray("10")
        self.assertEqual(a3, a1)
        self.assertEqual(dct[a3], 'one')

    def test_mix(self):
        a = bitarray('110')
        b = frozenbitarray('0011')
        self.assertEqual(a + b, bitarray('1100011'))
        a.extend(b)
        self.assertEqual(a, bitarray('1100011'))

    def test_hash_endianness_simple(self):
        a = frozenbitarray('1', 'big')
        b = frozenbitarray('1', 'little')
        self.assertEqual(a, b)
        self.assertEqual(hash(a), hash(b))
        d = {a: 'value'}
        self.assertEqual(d[b], 'value')
        self.assertEqual(len(set([a, b])), 1)

    def test_hash_endianness_random(self):
        for a in self.randombitarrays():
            a = frozenbitarray(a)
            b = frozenbitarray(a, self.opposite_endian(a.endian))
            self.assertEqual(a, b)
            self.assertNotEqual(a.endian, b.endian)
            self.assertEqual(hash(a), hash(b))
            d = {a: 1, b: 2}
            self.assertEqual(len(d), 1)

    def test_pickle(self):
        for a in self.randombitarrays():
            f = frozenbitarray(a)
            f.foo = 42  # unlike bitarray itself, we can have attributes
            g = pickle.loads(pickle.dumps(f))
            self.assertEqual(f, g)
            self.assertEqual(f.endian, g.endian)
            self.assertTrue(str(g).startswith('frozenbitarray'))
            self.assertTrue(g.readonly)
            self.check_obj(a)
            self.check_obj(f)
            self.check_obj(g)
            self.assertEqual(g.foo, 42)

    def test_bytes_bytearray(self):
        for a in self.randombitarrays():
            a = frozenbitarray(a)
            self.assertEqual(bytes(a), a.tobytes())
            self.assertEqual(bytearray(a), a.tobytes())
            self.check_obj(a)

# ---------------------------------------------------------------------------

def run(verbosity=1):
    import bitarray.test_util

    print('bitarray is installed in: %s' % os.path.dirname(__file__))
    print('bitarray version: %s' % __version__)
    print('sys.version: %s' % sys.version)
    print('sys.prefix: %s' % sys.prefix)
    if sys.version_info[:2] >= (3, 14):
        print('sys._is_gil_enabled(): %s' % sys._is_gil_enabled())
    print('pointer size: %d bit' % (8 * PTRSIZE))
    print('sizeof(size_t): %d' % sysinfo("size_t"));
    print('sizeof(bitarrayobject): %d' % sysinfo("bitarrayobject"))
    print('HAVE_BUILTIN_BSWAP64: %d' % sysinfo("HAVE_BUILTIN_BSWAP64"))
    print('default bit-endianness: %s' % get_default_endian())
    print('machine byte-order: %s' % sys.byteorder)
    print('Py_GIL_DISABLED: %s' % sysinfo("Py_GIL_DISABLED"))
    print('Py_DEBUG: %s' % sysinfo("Py_DEBUG"))
    print('DEBUG: %s' % sysinfo("DEBUG"))
    loader = unittest.TestLoader()
    suite = unittest.TestSuite()
    suite.addTests(loader.loadTestsFromModule(sys.modules[__name__]))
    suite.addTests(loader.loadTestsFromModule(bitarray.test_util))

    runner = unittest.TextTestRunner(verbosity=verbosity)
    result = runner.run(suite)
    return result

if __name__ == '__main__':
    unittest.main()
