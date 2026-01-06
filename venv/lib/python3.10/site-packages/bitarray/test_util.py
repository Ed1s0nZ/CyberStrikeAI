# Copyright (c) 2019 - 2025, Ilan Schnell; All Rights Reserved
# bitarray is published under the PSF license.
#
# Author: Ilan Schnell
"""
Tests for bitarray.util module
"""
import os
import sys
import math
import array
import base64
import binascii
import operator
import struct
import shutil
import tempfile
import unittest
from io import StringIO
from functools import reduce
from random import (choice, choices, getrandbits, randrange, randint, random,
                    sample, seed)
from string import hexdigits, whitespace
from collections import Counter

from bitarray import (bitarray, frozenbitarray, decodetree, bits2bytes,
                      get_default_endian)
from bitarray.test_bitarray import Util, skipIf, is_pypy, urandom_2, PTRSIZE

from bitarray.util import (
    zeros, ones, urandom, random_k, random_p, pprint, strip, count_n,
    parity, gen_primes, sum_indices, xor_indices,
    count_and, count_or, count_xor, any_and, subset,
    correspond_all, byteswap, intervals,
    serialize, deserialize, ba2hex, hex2ba, ba2base, base2ba,
    ba2int, int2ba,
    sc_encode, sc_decode, vl_encode, vl_decode,
    _huffman_tree, huffman_code, canonical_huffman, canonical_decode,
)

from bitarray.util import _Random, _ssqi  # type: ignore

# ---------------------------  zeros()  ones()  -----------------------------

class ZerosOnesTests(unittest.TestCase):

    def test_basic(self):
        for _ in range(50):
            a = choice([zeros(0), zeros(0, None), zeros(0, endian=None),
                        ones(0), ones(0, None), ones(0, endian=None)])
            self.assertEqual(a, bitarray())
            self.assertEqual(a.endian, get_default_endian())
            self.assertEqual(type(a), bitarray)

            endian = choice(['little', 'big', None])
            n = randrange(100)

            a = choice([zeros(n, endian), zeros(n, endian=endian)])
            self.assertEqual(a.to01(), n * "0")
            self.assertEqual(a.endian, endian or get_default_endian())

            b = choice([ones(n, endian), ones(n, endian=endian)])
            self.assertEqual(b.to01(), n * "1")
            self.assertEqual(b.endian, endian or get_default_endian())

    def test_errors(self):
        for f in zeros, ones:
            self.assertRaises(TypeError, f) # no argument
            self.assertRaises(TypeError, f, '')
            self.assertRaises(TypeError, f, bitarray())
            self.assertRaises(TypeError, f, [])
            self.assertRaises(TypeError, f, 1.0)
            self.assertRaises(ValueError, f, -1)

            # endian not string
            for x in 0, 1, {}, [], False, True:
                self.assertRaises(TypeError, f, 0, x)
            # endian wrong string
            self.assertRaises(ValueError, f, 0, 'foo')

# -----------------------------  urandom()  ---------------------------------

class URandomTests(unittest.TestCase):

    def test_basic(self):
        for _ in range(20):
            a = choice([urandom(0), urandom(0, endian=None)])
            self.assertEqual(a, bitarray())
            self.assertEqual(a.endian, get_default_endian())

            endian = choice(['little', 'big', None])
            n = randrange(100)

            a = choice([urandom(n, endian), urandom(n, endian=endian)])
            self.assertEqual(len(a), n)
            self.assertEqual(a.endian, endian or get_default_endian())
            self.assertEqual(type(a), bitarray)

    def test_errors(self):
        U = urandom
        self.assertRaises(TypeError, U)
        self.assertRaises(TypeError, U, '')
        self.assertRaises(TypeError, U, bitarray())
        self.assertRaises(TypeError, U, [])
        self.assertRaises(TypeError, U, 1.0)
        self.assertRaises(ValueError, U, -1)
        self.assertRaises(TypeError, U, 0, 1)
        self.assertRaises(ValueError, U, 0, 'foo')

    def test_count(self):
        a = urandom(10_000_000)
        # see if population is within expectation
        self.assertTrue(abs(a.count() - 5_000_000) <= 15_811)

# ----------------------------  random_k()  ---------------------------------

class Random_K_Tests(unittest.TestCase):

    def test_basic(self):
        for _ in range(250):
            endian = choice(['little', 'big', None])
            n = randrange(120)
            k = randint(0, n)
            a = random_k(n, k, endian)
            self.assertTrue(type(a), bitarray)
            self.assertEqual(len(a), n)
            self.assertEqual(a.count(), k)
            self.assertEqual(a.endian, endian or get_default_endian())

    def test_inputs_and_edge_cases(self):
        R = random_k
        self.assertRaises(TypeError, R)
        self.assertRaises(TypeError, R, 4)
        self.assertRaises(TypeError, R, 1, "0.5")
        self.assertRaises(TypeError, R, 1, p=1)
        self.assertRaises(TypeError, R, 11, 5.5)  # see issue #239
        self.assertRaises(ValueError, R, -1, 0)
        for k in -1, 11:  # k is not 0 <= k <= n
            self.assertRaises(ValueError, R, 10, k)
        self.assertRaises(ValueError, R, 10, 7, 'foo')
        self.assertRaises(ValueError, R, 10, 7, endian='foo')
        for n in range(20):
            self.assertEqual(R(n, k=0), zeros(n))
            self.assertEqual(R(n, k=n), ones(n))

    def test_count(self):
        for n in range(10):  # test explicitly for small n
            for k in range(n + 1):
                a = random_k(n, k)
                self.assertEqual(len(a), n)
                self.assertEqual(a.count(), k)

        for _ in range(100):
            n = randrange(10_000)
            k = randint(0, n)
            a = random_k(n, k)
            self.assertEqual(len(a), n)
            self.assertEqual(a.count(), k)

    def test_active_bits(self):
        # test if all bits are active
        n = 240
        cum = zeros(n)
        for _ in range(1000):
            k = randint(30, 40)
            a = random_k(n, k)
            self.assertEqual(a.count(), k)
            cum |= a
            if cum.all():
                break
        else:
            self.fail()

    # test uses math.comb, added in 3.8
    @skipIf(sys.version_info[:2] < (3, 8))
    def test_combinations(self):
        # for entire range of 0 <= k <= n, validate that random_k()
        # generates all possible combinations
        n = 7
        total = 0
        for k in range(n + 1):
            expected = math.comb(n, k)
            combs = set()
            for _ in range(10_000):
                combs.add(frozenbitarray(random_k(n, k)))
                if len(combs) == expected:
                    total += expected
                    break
            else:
                self.fail()
        self.assertEqual(total, 2 ** n)

    def collect_code_branches(self):
        # return list of bitarrays from all code branches of random_k()
        res = []
        # test small k (no .combine_half())
        res.append(random_k(300, 10))
        # general cases
        for k in 100, 500, 2_500, 4_000:
            res.append(random_k(5_000, k))
        return res

    def test_seed(self):
        # We ensure that after setting a seed value, random_k() will
        # always return the same random bitarrays.  However, we do not ensure
        # that these results will not change in future versions of bitarray.
        a = []
        for val in 654321, 654322, 654321, 654322:
            seed(val)
            a.append(self.collect_code_branches())
        self.assertEqual(a[0], a[2])
        self.assertEqual(a[1], a[3])
        for item0, item1 in zip(a[0], a[1]):
            self.assertNotEqual(item0, item1)
        # initialize seed with current system time again
        seed()

    # ---------------- tests for internal _Random methods -------------------

    def test_op_seq(self):
        r = _Random()
        G = r.op_seq
        K = r.K
        M = r.M

        # special cases
        self.assertRaises(ValueError, G, 0)
        self.assertEqual(G(1), zeros(M - 1))
        self.assertEqual(G(K // 2), bitarray())
        self.assertEqual(G(K - 1), ones(M - 1))
        self.assertRaises(ValueError, G, K)

        # examples
        for p, s in [
                (0.15625, '0100'),
                (0.25,       '0'),  # 1/2   AND ->   1/4
                (0.375,     '10'),  # 1/2   OR ->   3/4   AND ->   3/8
                (0.5,         ''),
                (0.625,     '01'),  # 1/2   AND ->   1/4   OR ->   5/8
                (0.6875,   '101'),
                (0.75,       '1'),  # 1/2   OR ->   3/4
        ]:
            seq = G(int(p * K))
            self.assertEqual(seq.to01(), s)

        for i in range(1, K):
            seq = G(i)
            self.assertTrue(0 <= len(s) < M)
            q = 0.5                        # a = random_half()
            for k in seq:
                # k=0: AND    k=1: OR
                if k:
                    q += 0.5 * (1.0 - q)   # a |= random_half()
                else:
                    q *= 0.5               # a &= random_half()
            self.assertEqual(q, i / K)

    def test_combine_half(self):
        r = _Random(1_000_000)
        for seq, mean in [
                ([],     500_000),  # .random_half() itself
                ([0],    250_000),  # AND
                ([1],    750_000),  # OR
                ([1, 0], 375_000),  # OR followed by AND
        ]:
            a = r.combine_half(seq)
            self.assertTrue(abs(a.count() - mean) < 5_000)

# ----------------------------  random_p()  ---------------------------------

HAVE_BINOMIALVARIATE = sys.version_info[:2] >= (3, 12)

@skipIf(HAVE_BINOMIALVARIATE)
class Random_P_Not_Implemented(unittest.TestCase):

    def test_not_implemented(self):
        self.assertRaises(NotImplementedError, random_p, 100, 0.25)


@skipIf(not HAVE_BINOMIALVARIATE)
class Random_P_Tests(unittest.TestCase):

    def test_basic(self):
        for _ in range(250):
            endian = choice(['little', 'big', None])
            n = randrange(120)
            p = choice([0.0, 0.0001, 0.2, 0.5, 0.9, 1.0])
            a = random_p(n, p, endian)
            self.assertTrue(type(a), bitarray)
            self.assertEqual(len(a), n)
            self.assertEqual(a.endian, endian or get_default_endian())

    def test_inputs_and_edge_cases(self):
        R = random_p
        self.assertRaises(TypeError, R)
        self.assertRaises(TypeError, R, 0.25)
        self.assertRaises(TypeError, R, 1, "0.5")
        self.assertRaises(ValueError, R, -1)
        self.assertRaises(ValueError, R, 1, -0.5)
        self.assertRaises(ValueError, R, 1, p=1.5)
        self.assertRaises(ValueError, R, 1, 0.15, 'foo')
        self.assertRaises(ValueError, R, 10, 0.5, endian='foo')
        self.assertEqual(R(0), bitarray())
        for n in range(20):
            self.assertEqual(R(n, 0), zeros(n))
            self.assertEqual(len(R(n, 0.5)), n)
            self.assertEqual(R(n, p=1), ones(n))

    def test_default(self):
        a = random_p(10_000_000)  # p defaults to 0.5
        # see if population is within expectation
        self.assertTrue(abs(a.count() - 5_000_000) <= 15_811)

    def test_count(self):
        for _ in range(500):
            n = choice([randrange(4, 120), randrange(100, 1000)])
            p = choice([0.0001, 0.001, 0.01, 0.1, 0.25, 0.5, 0.9])
            sigma = math.sqrt(n * p * (1.0 - p))
            a = random_p(n, p)
            self.assertEqual(len(a), n)
            self.assertTrue(abs(a.count() - n * p) < max(4, 10 * sigma))

    def collect_code_branches(self):
        # return list of bitarrays from all code branches of random_p()
        res = []
        # for default p=0.5, random_p uses getrandbits
        res.append(random_p(32))
        # test small p
        res.append(random_p(5_000, 0.002))
        # small n (note that p=0.4 will call the "literal definition" case)
        res.append(random_p(15, 0.4))
        # general cases
        for p in 0.1, 0.2, 0.375, 0.4999, 0.7:
            res.append(random_p(150, p))
        return res

    def test_seed(self):
        # We ensure that after setting a seed value, random_p() will always
        # return the same random bitarrays.  However, we do not ensure that
        # these results will not change in future versions of bitarray.
        a = []
        for val in 123456, 123457, 123456, 123457:
            seed(val)
            a.append(self.collect_code_branches())
        self.assertEqual(a[0], a[2])
        self.assertEqual(a[1], a[3])
        for item0, item1 in zip(a[0], a[1]):
            self.assertNotEqual(item0, item1)
        # initialize seed with current system time again
        seed()

    def test_small_p_limit(self):
        # For understanding how the algorithm works, see ./doc/random_p.rst
        # Also, see VerificationTests in devel/test_random.py
        r = _Random()
        limit = 1.0 / (r.K + 1)  # lower limit for p
        self.assertTrue(r.SMALL_P > limit)

# ----------------------------  gen_primes()  -------------------------------

class PrimeTests(unittest.TestCase):

    primes = [
        2, 3, 5, 7, 11, 13, 17, 19, 23, 29, 31, 37, 41, 43, 47, 53, 59, 61,
        67, 71, 73, 79, 83, 89, 97, 101, 103, 107, 109, 113, 127, 131, 137,
        139, 149, 151, 157, 163, 167, 173, 179, 181, 191, 193, 197, 199, 211,
        223, 227, 229, 233, 239, 241, 251, 257, 263, 269, 271, 277, 281, 283,
        293, 307, 311, 313, 317, 331, 337, 347, 349, 353, 359, 367, 373, 379,
        383, 389, 397, 401, 409, 419, 421, 431, 433, 439, 443, 449, 457, 461,
    ]

    def test_errors(self):
        P = gen_primes
        self.assertRaises(TypeError, P, 3, 1)
        self.assertRaises(ValueError, P, "1.0")
        self.assertRaises(ValueError, P, -1)
        self.assertRaises(TypeError, P, 8, 4)
        self.assertRaises(TypeError, P, 8, foo="big")
        self.assertRaises(ValueError, P, 8, "foo")
        self.assertRaises(ValueError, P, 8, endian="foo")

    def test_explitcit(self):
        for n in range(230):
            endian = choice(["little", "big", None])
            odd = getrandbits(1)
            a = gen_primes(n, endian, odd)
            self.assertEqual(len(a), n)
            self.assertEqual(a.endian, endian or get_default_endian())
            if odd:
                lst = [2] + [2 * i + 1 for i in a.search(1)]
            else:
                lst = [i for i in a.search(1)]
            self.assertEqual(lst, self.primes[:len(lst)])

    def test_cmp(self):
        N = 10_000
        c = ones(N)
        c[:2] = 0
        for i in range(int(math.sqrt(N) + 1.0)):
            if c[i]:
                c[i * i :: i] = 0
        self.assertEqual(list(c.search(1, 0, 462)), self.primes)

        for _ in range(20):
            n = randrange(N)
            endian = choice(["little", "big"])
            a = gen_primes(n, endian=endian)
            self.assertEqual(a, c[:n])
            self.assertEqual(a.endian, endian)

            b = gen_primes(n // 2, endian, odd=True)
            self.assertEqual(b, a[1::2])
            self.assertEqual(b, c[1:n:2])

        for _ in range(20):
            i = randrange(10, 100)
            x = randint(-1, 1)
            n = i * i + x
            self.assertEqual(gen_primes(n), c[:n])
            self.assertEqual(gen_primes(n // 2, odd=1), c[1:n:2])

        self.assertEqual(gen_primes(N), c)
        self.assertEqual(gen_primes(N // 2, odd=1), c[1::2])

    def test_count(self):
        for n, count, sum_p, sum_sqr_p in [
                (    10,    4,        17,             87),
                (   100,   25,     1_060,         65_796),
                ( 1_000,  168,    76_127,     49_345_379),
                (10_000, 1229, 5_736_396, 37_546_387_960),
        ]:
            a = gen_primes(n)
            self.assertEqual(len(a), n)
            self.assertEqual(a.count(), count)
            self.assertEqual(sum_indices(a), sum_p)
            self.assertEqual(sum_indices(a, 2), sum_sqr_p)
            b = gen_primes(n // 2, odd=1)
            self.assertEqual(len(b), n // 2)
            self.assertEqual(b.count() + 1, count)  # +1 because of prime 2
            self.assertEqual(b, a[1::2])

# -----------------------------  pprint()  ----------------------------------

class PPrintTests(unittest.TestCase):

    @staticmethod
    def get_code_string(a):
        f = StringIO()
        pprint(a, stream=f)
        return f.getvalue()

    def round_trip(self, a):
        b = eval(self.get_code_string(a))
        self.assertEqual(b, a)
        self.assertEqual(type(b), type(a))

    def test_bitarray(self):
        a = bitarray('110')
        self.assertEqual(self.get_code_string(a), "bitarray('110')\n")
        self.round_trip(a)

    def test_frozenbitarray(self):
        a = frozenbitarray('01')
        self.assertEqual(self.get_code_string(a), "frozenbitarray('01')\n")
        self.round_trip(a)

    def test_formatting(self):
        a = bitarray(200)
        for width in range(40, 130, 10):
            for n in range(1, 10):
                f = StringIO()
                pprint(a, stream=f, group=n, width=width)
                r = f.getvalue()
                self.assertEqual(eval(r), a)
                s = r.strip("bitary(')\n")
                for group in s.split()[:-1]:
                    self.assertEqual(len(group), n)
                for line in s.split('\n'):
                    self.assertTrue(len(line) < width)

    def test_fallback(self):
        for a in None, 'asd', [1, 2], bitarray(), frozenbitarray('1'):
            self.round_trip(a)

    def test_subclass(self):
        class Foo(bitarray):
            pass

        a = Foo()
        code = self.get_code_string(a)
        self.assertEqual(code, "Foo()\n")
        b = eval(code)
        self.assertEqual(b, a)
        self.assertEqual(type(b), type(a))

    def test_random(self):
        for n in range(150):
            self.round_trip(urandom(n))

    def test_file(self):
        tmpdir = tempfile.mkdtemp()
        tmpfile = os.path.join(tmpdir, 'testfile')
        a = urandom_2(1000)
        try:
            with open(tmpfile, 'w') as fo:
                pprint(a, fo)
            with open(tmpfile, 'r') as fi:
                b = eval(fi.read())
            self.assertEqual(a, b)
        finally:
            shutil.rmtree(tmpdir)

# -----------------------------  strip()  -----------------------------------

class StripTests(unittest.TestCase, Util):

    def test_simple(self):
        self.assertRaises(TypeError, strip, '0110')
        self.assertRaises(TypeError, strip, bitarray(), 123)
        self.assertRaises(ValueError, strip, bitarray(), 'up')

        a = bitarray('00010110000')
        self.assertEQUAL(strip(a), bitarray('0001011'))
        self.assertEQUAL(strip(a, 'left'), bitarray('10110000'))
        self.assertEQUAL(strip(a, 'both'), bitarray('1011'))
        b = frozenbitarray('00010110000')
        c = strip(b, 'both')
        self.assertEqual(c, bitarray('1011'))
        self.assertEqual(type(c), frozenbitarray)

    def test_zeros_ones(self):
        for _ in range(50):
            n = randrange(10)
            mode = choice(['left', 'right', 'both'])
            a = zeros(n)
            c = strip(a, mode)
            self.assertEqual(type(c), bitarray)
            self.assertEqual(len(c), 0)
            self.assertEqual(a, zeros(n))

            b = frozenbitarray(a)
            c = strip(b, mode)
            self.assertEqual(type(c), frozenbitarray)
            self.assertEqual(len(c), 0)

            a.setall(1)
            c = strip(a, mode)
            self.assertEqual(c, ones(n))

    def test_random(self):
        for a in self.randombitarrays():
            b = a.copy()
            f = frozenbitarray(a)
            s = a.to01()
            for mode, res in [
                    ('left',  bitarray(s.lstrip('0'), a.endian)),
                    ('right', bitarray(s.rstrip('0'), a.endian)),
                    ('both',  bitarray(s.strip('0'),  a.endian)),
            ]:
                c = strip(a, mode)
                self.assertEQUAL(c, res)
                self.assertEqual(type(c), bitarray)
                self.assertEQUAL(a, b)

                c = strip(f, mode)
                self.assertEQUAL(c, res)
                self.assertEqual(type(c), frozenbitarray)
                self.assertEQUAL(f, b)

    def test_one_set(self):
        for _ in range(10):
            n = randint(1, 10000)
            a = bitarray(n)
            a.setall(0)
            a[randrange(n)] = 1
            self.assertEqual(strip(a, 'both'), bitarray('1'))
            self.assertEqual(len(a), n)

# -----------------------------  count_n()  ---------------------------------

class CountN_Tests(unittest.TestCase, Util):

    @staticmethod
    def count_n(a, n):
        "return lowest index i for which a[:i].count() == n"
        i, j = n, a.count(1, 0, n)
        while j < n:
            j += a[i]
            i += 1
        return i

    def check_result(self, a, n, i, v=1):
        self.assertEqual(a.count(v, 0, i), n)
        if i == 0:
            self.assertEqual(n, 0)
        else:
            self.assertEqual(a[i - 1], v)

    def test_empty(self):
        a = bitarray()
        self.assertEqual(count_n(a, 0), 0)
        self.assertEqual(count_n(a, 0, 0), 0)
        self.assertEqual(count_n(a, 0, 1), 0)
        self.assertRaises(ValueError, count_n, a, 1)
        self.assertRaises(TypeError, count_n, '', 0)
        self.assertRaises(TypeError, count_n, a, 7.0)
        self.assertRaises(ValueError, count_n, a, 0, 2)
        self.assertRaisesMessage(ValueError, "n = 1 larger than bitarray "
                                 "length 0", count_n, a, 1)

    def test_simple(self):
        a = bitarray('111110111110111110111110011110111110111110111000')
        b = a.copy()
        self.assertEqual(len(a), 48)
        self.assertEqual(a.count(), 37)
        self.assertEqual(a.count(0), 11)

        self.assertEqual(count_n(a, 0), 0)
        self.assertEqual(count_n(a, 0, 0), 0)
        self.assertEqual(count_n(a, 2, 0), 12)
        self.assertEqual(count_n(a, 10, 0), 47)
        self.assertEqual(count_n(a, 20), 23)
        self.assertEqual(count_n(a, 20, 1), 23)
        self.assertEqual(count_n(a, 37), 45)
        # n < 0
        self.assertRaisesMessage(ValueError, "non-negative integer expected",
                                 count_n, a, -1)
        # n > len(a)
        self.assertRaisesMessage(ValueError, "n = 49 larger than bitarray "
                                 "length 48", count_n, a, 49)
        # n > a.count(0)
        self.assertRaisesMessage(ValueError, "n = 12 exceeds total count "
                                 "(a.count(0) = 11)", count_n, a, 12, 0)
        # n > a.count(1)
        self.assertRaisesMessage(ValueError, "n = 38 exceeds total count "
                                 "(a.count(1) = 37)", count_n, a, 38, 1)

        for v in 0, 1:
            for n in range(a.count(v) + 1):
                i = count_n(a, n, v)
                self.check_result(a, n, i, v)
                self.assertEqual(a[:i].count(v), n)
                self.assertEqual(i, self.count_n(a if v else ~a, n))
        self.assertEQUAL(a, b)

    def test_frozenbitarray(self):
        a = frozenbitarray('001111101111101111101111100111100')
        self.assertEqual(len(a), 33)
        self.assertEqual(a.count(), 24)
        self.assertEqual(count_n(a, 0), 0)
        self.assertEqual(count_n(a, 10), 13)
        self.assertEqual(count_n(a, 24), 31)
        self.assertRaises(ValueError, count_n, a, -1) # n < 0
        self.assertRaises(ValueError, count_n, a, 25) # n > a.count()
        self.assertRaises(ValueError, count_n, a, 34) # n > len(a)
        for n in range(25):
            self.check_result(a, n, count_n(a, n))

    def test_ones(self):
        n = randint(1, 100_000)
        a = ones(n)
        self.assertEqual(count_n(a, n), n)
        self.assertRaises(ValueError, count_n, a, 1, 0)
        self.assertRaises(ValueError, count_n, a, n + 1)
        for _ in range(20):
            i = randint(0, n)
            self.assertEqual(count_n(a, i), i)

    def test_one_set(self):
        n = randint(1, 100_000)
        a = zeros(n)
        self.assertEqual(count_n(a, 0), 0)
        self.assertRaises(ValueError, count_n, a, 1)
        for _ in range(20):
            a.setall(0)
            i = randrange(n)
            a[i] = 1
            self.assertEqual(count_n(a, 1), i + 1)
            self.assertRaises(ValueError, count_n, a, 2)

    def test_last(self):
        for N in range(1, 1000):
            a = zeros(N)
            a[-1] = 1
            self.assertEqual(count_n(a, 1), N)
            if N == 1:
                msg = "n = 2 larger than bitarray length 1"
            else:
                msg = "n = 2 exceeds total count (a.count(1) = 1)"
            self.assertRaisesMessage(ValueError, msg, count_n, a, 2)

    def test_primes(self):
        a = gen_primes(10_000)
        # there are 1229 primes below 10,000
        self.assertEqual(a.count(), 1229)
        for n, p in [(  10,   29),   # the 10th prime number is 29
                     ( 100,  541),   # the 100th prime number is 541
                     (1000, 7919)]:  # the 1000th prime number is 7919
            self.assertEqual(count_n(a, n) - 1, p)

    def test_large(self):
        for _ in range(100):
            N = randint(100_000, 250_000)
            a = bitarray(N)
            v = getrandbits(1)
            a.setall(not v)
            for _ in range(randrange(100)):
                a[randrange(N)] = v
            tc = a.count(v)      # total count
            i = count_n(a, tc, v)
            self.check_result(a, tc, i, v)
            n = tc + 1
            self.assertRaisesMessage(ValueError, "n = %d exceeds total count "
                                     "(a.count(%d) = %d)" % (n, v, tc),
                                     count_n, a, n, v)
            for _ in range(20):
                n = randint(0, tc)
                i = count_n(a, n, v)
                self.check_result(a, n, i, v)

# ---------------------------------------------------------------------------

class BitwiseCountTests(unittest.TestCase, Util):

    def test_count_byte(self):
        for i in range(256):
            a = bitarray(bytearray([i]))
            cnt = a.count()
            self.assertEqual(count_and(a, zeros(8)), 0)
            self.assertEqual(count_and(a, ones(8)), cnt)
            self.assertEqual(count_and(a, a), cnt)
            self.assertEqual(count_or(a, zeros(8)), cnt)
            self.assertEqual(count_or(a, ones(8)), 8)
            self.assertEqual(count_or(a, a), cnt)
            self.assertEqual(count_xor(a, zeros(8)), cnt)
            self.assertEqual(count_xor(a, ones(8)), 8 - cnt)
            self.assertEqual(count_xor(a, a), 0)

    def test_1(self):
        a = bitarray('001111')
        aa = a.copy()
        b = bitarray('010011')
        bb = b.copy()
        self.assertEqual(count_and(a, b), 2)
        self.assertEqual(count_or(a, b), 5)
        self.assertEqual(count_xor(a, b), 3)
        for f in count_and, count_or, count_xor:
            # not two arguments
            self.assertRaises(TypeError, f)
            self.assertRaises(TypeError, f, a)
            self.assertRaises(TypeError, f, a, b, 3)
            # wrong argument types
            self.assertRaises(TypeError, f, a, '')
            self.assertRaises(TypeError, f, '1', b)
            self.assertRaises(TypeError, f, a, 4)
        self.assertEQUAL(a, aa)
        self.assertEQUAL(b, bb)

        b.append(1)
        for f in count_and, count_or, count_xor:
            self.assertRaises(ValueError, f, a, b)
            self.assertRaises(ValueError, f,
                              bitarray('110', 'big'),
                              bitarray('101', 'little'))

    def test_frozen(self):
        a = frozenbitarray('001111')
        b = frozenbitarray('010011')
        self.assertEqual(count_and(a, b), 2)
        self.assertEqual(count_or(a, b), 5)
        self.assertEqual(count_xor(a, b), 3)

    def test_random(self):
        for _ in range(100):
            n = randrange(1000)
            a = urandom_2(n)
            b = urandom(n, a.endian)
            self.assertEqual(count_and(a, b), (a & b).count())
            self.assertEqual(count_or(a, b),  (a | b).count())
            self.assertEqual(count_xor(a, b), (a ^ b).count())

    def test_misc(self):
        for a in self.randombitarrays():
            n = len(a)
            b = urandom(n, a.endian)
            # any and
            self.assertEqual(any(a & b), count_and(a, b) > 0)
            self.assertEqual(any_and(a, b), any(a & b))
            # any or
            self.assertEqual(any(a | b), count_or(a, b) > 0)
            self.assertEqual(any(a | b), any(a) or any(b))
            # any xor
            self.assertEqual(any(a ^ b), count_xor(a, b) > 0)
            self.assertEqual(any(a ^ b), a != b)

            # all and
            self.assertEqual(all(a & b), count_and(a, b) == n)
            self.assertEqual(all(a & b), all(a) and all(b))
            # all or
            self.assertEqual(all(a | b), count_or(a, b) == n)
            # all xor
            self.assertEqual(all(a ^ b), count_xor(a, b) == n)
            self.assertEqual(all(a ^ b), a == ~b)

# ---------------------------  any_and()  -----------------------------------

class BitwiseAnyTests(unittest.TestCase, Util):

    def test_basic(self):
        a = frozenbitarray('0101')
        b = bitarray('0111')
        self.assertTrue(any_and(a, b))
        self.assertRaises(TypeError, any_and)
        self.assertRaises(TypeError, any_and, a, 4)
        b.append(1)
        self.assertRaises(ValueError, any_and, a, b)
        self.assertRaises(ValueError, any_and,
                          bitarray('01', 'little'),
                          bitarray('11', 'big'))

    def test_overlap(self):
        n = 100
        for _ in range(500):
            i1 = randint(0, n)
            j1 = randint(i1, n)
            r1 = range(i1, j1)

            i2 = randint(0, n)
            j2 = randint(i2, n)
            r2 = range(i2, j2)

            # test if ranges r1 and r2 overlap
            res1 = bool(r1) and bool(r2) and (i2 in r1 or i1 in r2)
            res2 = bool(set(r1) & set(r2))
            self.assertEqual(res1, res2)

            a1, a2 = bitarray(n), bitarray(n)
            a1[i1:j1] = a2[i2:j2] = 1
            self.assertEqual(any_and(a1, a2), res1)

    def test_common(self):
        n = 100
        for _ in range(500):
            s1 = self.random_slice(n)
            s2 = self.random_slice(n)
            r1 = range(n)[s1]
            r2 = range(n)[s2]
            # test if ranges r1 and r2 have common items
            a1, a2 = bitarray(n), bitarray(n)
            a1[s1] = a2[s2] = 1
            self.assertEqual(any_and(a1, a2), bool(set(r1) & set(r2)))

    def check(self, a, b):
        r = any_and(a, b)
        self.assertEqual(type(r), bool)
        self.assertEqual(r, any_and(b, a))  # symmetry
        self.assertEqual(r, any(a & b))
        self.assertEqual(r, (a & b).any())
        self.assertEqual(r, count_and(a, b) > 0)

    def test_explitcit(self):
        for a, b , res in [
                ('', '', False),
                ('0', '1', False),
                ('0', '0', False),
                ('1', '1', True),
                ('00011', '11100', False),
                ('00001011 1', '01000100 1', True)]:
            a = bitarray(a)
            b = bitarray(b)
            self.assertTrue(any_and(a, b) is res)
            self.check(a, b)

    def test_random(self):
        for a in self.randombitarrays():
            n = len(a)
            b = urandom(n, a.endian)
            self.check(a, b)

    def test_one(self):
        for n in range(1, 300):
            a = zeros(n)
            b = urandom(n)
            i = randrange(n)
            a[i] = 1
            self.assertEqual(b[i], any_and(a, b))

# ----------------------------  subset()  -----------------------------------

class SubsetTests(unittest.TestCase, Util):

    def test_basic(self):
        a = frozenbitarray('0101')
        b = bitarray('0111')
        self.assertTrue(subset(a, b))
        self.assertFalse(subset(b, a))
        self.assertRaises(TypeError, subset)
        self.assertRaises(TypeError, subset, a, '')
        self.assertRaises(TypeError, subset, '1', b)
        self.assertRaises(TypeError, subset, a, 4)
        b.append(1)
        self.assertRaises(ValueError, subset, a, b)
        self.assertRaises(ValueError, subset,
                          bitarray('01', 'little'),
                          bitarray('11', 'big'))

    def check(self, a, b, res):
        r = subset(a, b)
        self.assertEqual(type(r), bool)
        self.assertEqual(r, res)
        self.assertEqual(a | b == b, res)
        self.assertEqual(a & b == a, res)

    def test_True(self):
        for a, b in [('', ''), ('0', '1'), ('0', '0'), ('1', '1'),
                     ('000', '111'), ('0101', '0111'),
                     ('000010111', '010011111')]:
            self.check(bitarray(a), bitarray(b), True)

    def test_False(self):
        for a, b in [('1', '0'), ('1101', '0111'),
                     ('0000101111', '0100111011')]:
            self.check(bitarray(a), bitarray(b), False)

    def test_random(self):
        for a in self.randombitarrays(start=1):
            b = a.copy()
            # we set one random bit in b to 1, so a is always a subset of b
            b[randrange(len(a))] = 1
            self.check(a, b, True)
            # but b is only a subset when they are equal
            self.check(b, a, a == b)
            # we set all bits in a, which ensures that b is a subset of a
            a.setall(1)
            self.check(b, a, True)

# -------------------------  correspond_all()  ------------------------------

class CorrespondAllTests(unittest.TestCase):

    def test_basic(self):
        a = frozenbitarray('0101')
        b = bitarray('0111')
        self.assertTrue(correspond_all(a, b), (1, 1, 1, 1))
        self.assertRaises(TypeError, correspond_all)
        b.append(1)
        self.assertRaises(ValueError, correspond_all, a, b)
        self.assertRaises(ValueError, correspond_all,
                          bitarray('01', 'little'),
                          bitarray('11', 'big'))

    def test_explitcit(self):
        for a, b, res in [
                ('', '', (0, 0, 0, 0)),
                ('0000011111',
                 '0000100111', (4, 1, 2, 3)),
            ]:
            self.assertEqual(correspond_all(bitarray(a), bitarray(b)), res)

    def test_random(self):
        for _ in range(100):
            n = randrange(3000)
            a = urandom_2(n)
            b = urandom(n, a.endian)
            res = correspond_all(a, b)
            self.assertEqual(res[0], count_and(~a, ~b))
            self.assertEqual(res[1], count_and(~a, b))
            self.assertEqual(res[2], count_and(a, ~b))
            self.assertEqual(res[3], count_and(a, b))

            self.assertEqual(res[0], n - count_or(a, b))
            self.assertEqual(res[1] + res[2], count_xor(a, b))
            self.assertEqual(sum(res), n)

# -----------------------------  byteswap()  --------------------------------

@skipIf(is_pypy)
class ByteSwapTests(unittest.TestCase):

    def test_basic_bytearray(self):
        a = bytearray(b"ABCD")
        byteswap(a, 2)
        self.assertEqual(a, bytearray(b"BADC"))
        byteswap(a)
        self.assertEqual(a, bytearray(b"CDAB"))

        a = bytearray(b"ABCDEF")
        byteswap(a, 3)
        self.assertEqual(a, bytearray(b"CBAFED"))
        byteswap(a, 1)
        self.assertEqual(a, bytearray(b"CBAFED"))

    def test_basic_bitarray(self):
        a = bitarray("11110000 01010101")
        byteswap(a)
        self.assertEqual(a, bitarray("01010101 11110000"))

        a = bitarray("01111000 1001")
        b = a.copy()
        a.tobytes()  # clear padbits
        byteswap(a)
        self.assertEqual(a, bitarray("10010000 0111"))
        byteswap(a)
        self.assertEqual(a, b)

    def test_basic_array(self):
        r = os.urandom(64)
        for typecode in array.typecodes:
            # type code 'u' is deprecated and will be removed in Python 3.16
            if typecode == 'u':
                continue
            a = array.array(typecode, r)
            self.assertEqual(len(a) * a.itemsize, 64)
            a.byteswap()
            byteswap(a, a.itemsize)
            self.assertEqual(a.tobytes(), r)

    def test_empty(self):
        a = bytearray()
        byteswap(a)
        self.assertEqual(a, bytearray())
        for n in range(10):
            byteswap(a, n)
            self.assertEqual(a, bytearray())

    def test_one_byte(self):
        a = bytearray(b'\xab')
        byteswap(a)
        self.assertEqual(a, bytearray(b'\xab'))
        for n in range(2):
            byteswap(a, n)
            self.assertEqual(a, bytearray(b'\xab'))

    def test_errors(self):
        # buffer not writable
        for a in b"AB", frozenbitarray(16):
            self.assertRaises(BufferError, byteswap, a)

        a = bytearray(b"ABCD")
        b = bitarray(32)
        for n in -1, 3, 5, 6:
            # byte size not multiple of n
            self.assertRaises(ValueError, byteswap, a, n)
            self.assertRaises(ValueError, byteswap, b, n)

    def test_range(self):
        for n in range(20):
            for m in range(20):
                r = os.urandom(m * n)
                a = bytearray(r)
                byteswap(a, n)
                lst = []
                for i in range(m):
                    x = r[i * n:i * n + n]
                    lst.extend(x[::-1])
                self.assertEqual(a, bytearray(lst))

    def test_reverse_bytearray(self):
        for n in range(100):
            r = os.urandom(n)
            a = bytearray(r)
            byteswap(a)
            self.assertEqual(a, bytearray(r[::-1]))

    def test_reverse_bitarray(self):
        for n in range(100):
            a = urandom(8 * n)
            b = a.copy()
            byteswap(a)
            a.bytereverse()
            self.assertEqual(a, b[::-1])

# ------------------------------  parity()  ---------------------------------

class ParityTests(unittest.TestCase):

    def test_explitcit(self):
        for s, res in [('', 0), ('1', 1), ('0010011', 1), ('10100110', 0)]:
            self.assertTrue(parity(bitarray(s)) is res)
            self.assertTrue(parity(frozenbitarray(s)) is res)

    def test_zeros_ones(self):
        for n in range(2000):
            self.assertEqual(parity(zeros(n)), 0)
            self.assertEqual(parity(ones(n)), n % 2)

    def test_random(self):
        endian = choice(["little", "big"])
        a = bitarray(endian=endian)
        par = 0
        for i in range(2000):
            self.assertEqual(parity(a), par)
            self.assertEqual(par, a.count() % 2)
            self.assertEqual(a.endian, endian)
            self.assertEqual(len(a), i)
            v = getrandbits(1)
            a.append(v)
            par ^= v

    def test_wrong_args(self):
        self.assertRaises(TypeError, parity, '')
        self.assertRaises(TypeError, parity, 1)
        self.assertRaises(TypeError, parity)
        self.assertRaises(TypeError, parity, bitarray("110"), 1)

# ----------------------------  sum_indices()  ------------------------------

class SumIndicesUtil(unittest.TestCase):

    def check_explicit(self, S):
        for s, r1, r2 in [
                ("", 0, 0), ("0", 0, 0), ("1", 0, 0), ("11", 1, 1),
                ("011", 3, 5), ("001", 2, 4), ("0001100", 7, 25),
                ("00001111", 22, 126), ("01100111 1101", 49, 381),
        ]:
            for a in [bitarray(s, choice(['little', 'big'])),
                      frozenbitarray(s, choice(['little', 'big']))]:
                self.assertEqual(S(a, 1), r1)
                self.assertEqual(S(a, 2), r2)
                self.assertEqual(a, bitarray(s))

    def check_wrong_args(self, S):
        self.assertRaises(TypeError, S, '')
        self.assertRaises(TypeError, S, 1.0)
        self.assertRaises(TypeError, S)
        for mode in -1, 0, 3, 4:
            self.assertRaises(ValueError, S, bitarray("110"), mode)

    def check_urandom(self, S, n):
        a = urandom_2(n)
        self.assertEqual(S(a, 1), sum(i for i, v in enumerate(a) if v))
        self.assertEqual(S(a, 2), sum(i * i for i, v in enumerate(a) if v))

    def check_sparse(self, S, n, k, mode=1, freeze=False, inv=False):
        a = zeros(n, choice(['little', 'big']))
        self.assertEqual(S(a, mode), 0)
        self.assertFalse(a.any())

        indices = sample(range(n), k)
        a[indices] = 1
        res = sum(indices) if mode == 1 else sum(i * i for i in indices)

        if inv:
            a.invert()
            sum_ones = 3 if mode == 1 else 2 * n - 1
            sum_ones *= n * (n - 1)
            sum_ones //= 6
            res = sum_ones - res

        if freeze:
            a = frozenbitarray(a)

        c = a.copy()
        self.assertEqual(a.count(), n - k if inv else k)
        self.assertEqual(S(a, mode), res)
        self.assertEqual(a, c)


class SSQI_Tests(SumIndicesUtil):

    # Additional tests for _ssqi() in: devel/test_sum_indices.py

    def test_explicit(self):
        self.check_explicit(_ssqi)

    def test_wrong_args(self):
        self.check_wrong_args(_ssqi)

    def test_small(self):
        a = bitarray()
        sm1 = sm2 = 0
        for i in range(100):
            v = getrandbits(1)
            a.append(v)
            if v:
                sm1 += i
                sm2 += i * i
            self.assertEqual(_ssqi(a, 1), sm1)
            self.assertEqual(_ssqi(a, 2), sm2)

    def test_urandom(self):
        self.check_urandom(_ssqi, 10_037)

    def test_sparse(self):
        for _ in range(5):
            mode = randint(1, 2)
            freeze = getrandbits(1)
            inv = getrandbits(1)
            self.check_sparse(_ssqi, n=1_000_003, k=400,
                              mode=mode, freeze=freeze, inv=inv)


class SumIndicesTests(SumIndicesUtil):

    # Additional tests in: devel/test_sum_indices.py

    def test_explicit(self):
        self.check_explicit(sum_indices)
        a = gen_primes(100)
        self.assertEqual(sum_indices(a, mode=1),  1_060)
        self.assertEqual(sum_indices(a, mode=2), 65_796)

    def test_wrong_args(self):
        self.check_wrong_args(sum_indices)

    def test_ones(self):
        for mode in 1, 2:
            self.check_sparse(sum_indices, n=1_600_037, k=0,
                              mode=mode, freeze=True, inv=True)

    def test_sparse(self):
        for _ in range(20):
            n = choice([500_029, 600_011])  # below and above block size
            k = randrange(1_000)
            mode = randint(1, 2)
            freeze = getrandbits(1)
            inv = getrandbits(1)
            self.check_sparse(sum_indices, n, k, mode, freeze, inv)

# ---------------------------------------------------------------------------

class XoredIndicesTests(unittest.TestCase, Util):

    def test_explicit(self):
        for s, r in [("", 0), ("0", 0), ("1", 0), ("11", 1),
                     ("011", 3), ("001", 2), ("0001100", 7),
                     ("01100111 1101", 13)]:
            for a in [bitarray(s, self.random_endian()),
                      frozenbitarray(s, self.random_endian())]:
                self.assertEqual(xor_indices(a), r)

    def test_wrong_args(self):
        X = xor_indices
        self.assertRaises(TypeError, X, '')
        self.assertRaises(TypeError, X, 1)
        self.assertRaises(TypeError, X)
        self.assertRaises(TypeError, X, bitarray("110"), 1)

    def test_ones(self):
        # OEIS A003815
        lst = [0, 1, 3, 0, 4, 1, 7, 0, 8, 1, 11, 0, 12, 1, 15, 0, 16, 1, 19]
        self.assertEqual([xor_indices(ones(i)) for i in range(1, 20)], lst)
        a = bitarray()
        x = 0
        for i in range(1000):
            a.append(1)
            x ^= i
            self.assertEqual(xor_indices(a), x)
            if i < 19:
                self.assertEqual(lst[i], x)

    def test_primes(self):
        # OEIS A126084
        lst = [0, 2, 1, 4, 3, 8, 5, 20, 7, 16, 13, 18, 55, 30, 53, 26, 47]
        primes = gen_primes(1000)
        x = 0
        for i, p in enumerate(primes.search(1)):
            self.assertEqual(xor_indices(primes[:p]), x)
            if i < 17:
                self.assertEqual(lst[i], x)
            x ^= p

    def test_large_random(self):
        n = 10_037
        for a in [urandom_2(n), frozenbitarray(urandom_2(n))]:
            res = reduce(operator.xor, (i for i, v in enumerate(a) if v))
            b = a.copy()
            self.assertEqual(xor_indices(a), res)
            self.assertEqual(a, b)

    def test_random(self):
        for a in self.randombitarrays():
            c = 0
            for i, v in enumerate(a):
                c ^= i * v
            self.assertEqual(xor_indices(a), c)

    def test_flips(self):
        a = bitarray(128)
        c = 0
        for _ in range(1000):
            self.assertEqual(xor_indices(a), c)
            i = randrange(len(a))
            a.invert(i)
            c ^= i

    def test_error_correct(self):
        parity_bits = [1, 2, 4, 8, 16, 32, 64, 128]  # parity bit positions
        a = urandom(256)
        a[parity_bits] = 0
        c = xor_indices(a)
        # set parity bits such that block is well prepared
        a[parity_bits] = int2ba(c, length=8, endian="little")
        for i in range(0, 256):
            self.assertEqual(xor_indices(a), 0)  # ensure well prepared
            a.invert(i)
            self.assertEqual(xor_indices(a), i)  # index of the flipped bit!
            a.invert(i)

# ------------------   intervals of uninterrupted runs   --------------------

def runs(a):
    "return number of uninterrupted intervals of 1s and 0s"
    n = len(a)
    if n < 2:
        return n
    return 1 + count_xor(a[:-1], a[1:])

class IntervalsTests(unittest.TestCase, Util):

    def test_explicit(self):
        for s, lst in [
                ('', []),
                ('0', [(0, 0, 1)]),
                ('1', [(1, 0, 1)]),
                ('00111100 0000011',
                 [(0, 0, 2), (1, 2, 6), (0, 6, 13), (1, 13, 15)]),
            ]:
            a = bitarray(s)
            self.assertEqual(list(intervals(a)), lst)
            self.assertEqual(runs(a), len(lst))

    def test_uniform(self):
        for n in range(1, 100):
            for v in 0, 1:
                a = n * bitarray([v], self.random_endian())
                self.assertEqual(list(intervals(a)), [(v, 0, n)])
                self.assertEqual(runs(a), 1)

    def test_random(self):
        for a in self.randombitarrays():
            n = len(a)
            b = urandom(n)
            for value, start, stop in intervals(a):
                self.assertFalse(isinstance(value, bool))
                self.assertTrue(0 <= start < stop <= n)
                b[start:stop] = value
            self.assertEqual(a, b)

    def test_list_runs(self):
        for a in self.randombitarrays():
            # list of length of runs of alternating bits
            alt_runs = [stop - start for _, start, stop in intervals(a)]
            self.assertEqual(len(alt_runs), runs(a))

            b = bitarray()
            v = a[0] if a else None  # value of first run
            for length in alt_runs:
                self.assertTrue(length > 0)
                b.extend(length * bitarray([v]))
                v = not v
            self.assertEqual(a, b)

# --------------------------  ba2hex()  hex2ba()  ---------------------------

class HexlifyTests(unittest.TestCase, Util):

    def test_explicit(self):
        data = [ #                  little   big
            ('',                    '',      ''),
            ('1000',                '1',     '8'),
            ('0101 0110',           'a6',    '56'),
            ('0100 1001 1101',      '29b',   '49d'),
            ('0000 1100 1110 1111', '037f',  '0cef'),
        ]
        for bs, hex_le, hex_be in data:
            a_be = bitarray(bs, 'big')
            a_le = bitarray(bs, 'little')
            self.assertEQUAL(hex2ba(hex_be, 'big'), a_be)
            self.assertEQUAL(hex2ba(hex_le, 'little'), a_le)
            self.assertEqual(ba2hex(a_be), hex_be)
            self.assertEqual(ba2hex(a_le), hex_le)

    def test_ba2hex_group(self):
        a = bitarray('1000 0000 0101 1111', 'little')
        self.assertEqual(ba2hex(a), "10af")
        self.assertEqual(ba2hex(a, 0), "10af")
        self.assertEqual(ba2hex(a, 1, ""), "10af")
        self.assertEqual(ba2hex(a, 1), "1 0 a f")
        self.assertEqual(ba2hex(a, group=2), "10 af")
        self.assertEqual(ba2hex(a, 2, "-"), "10-af")
        self.assertEqual(ba2hex(a, group=3, sep="_"), "10a_f")
        self.assertEqual(ba2hex(a, 3, sep=", "), "10a, f")

    def test_ba2hex_errors(self):
        self.assertRaises(TypeError, ba2hex)
        self.assertRaises(TypeError, ba2hex, None)
        self.assertRaises(TypeError, ba2hex, '101')

        # length not multiple of 4
        self.assertRaises(ValueError, ba2hex, bitarray('10'))

        a = bitarray('1000 0000 0101 1111', 'little')
        self.assertRaises(ValueError, ba2hex, a, -1)
        self.assertRaises(ValueError, ba2hex, a, group=-1)
        # sep not str
        self.assertRaises(TypeError, ba2hex, a, 1, b" ")
        # embedded null character in sep
        self.assertRaises(ValueError, ba2hex, a, 2, " \0")

    def test_hex2ba_whitespace(self):
        self.assertEqual(hex2ba("F1 FA %s f3 c0" % whitespace),
                         bitarray("11110001 11111010 11110011 11000000"))
        self.assertEQUAL(hex2ba(b' a F ', 'big'),
                         bitarray('1010 1111', 'big'))
        self.assertEQUAL(hex2ba(860 * " " + '0  1D' + 590 * " ", 'little'),
                         bitarray('0000 1000 1011', 'little'))

    def test_hex2ba_errors(self):
        self.assertRaises(TypeError, hex2ba, 0)
        self.assertRaises(TypeError, hex2ba, "F", 1)
        self.assertRaises(ValueError, hex2ba, "F", "foo")

        for s in '01a7g89', '0\u20ac', '0 \0', b'\x00':
            self.assertRaises(ValueError, hex2ba, s)

        for s in 'g', 'ag', 'aag' 'aaaga', 'ag':
            msg = "invalid digit found for base16, got 'g' (0x67)"
            self.assertRaisesMessage(ValueError, msg, hex2ba, s, 'big')

    def test_hex2ba_types(self):
        for c in 'e', 'E', b'e', b'E', bytearray(b'e'), bytearray(b'E'):
            a = hex2ba(c, "big")
            self.assertEqual(a.to01(), '1110')
            self.assertEqual(a.endian, 'big')
            self.assertEqual(type(a), bitarray)

    def test_random(self):
        for _ in range(100):
            endian = choice(["little", "big", None])
            a = urandom_2(4 * randrange(100), endian)
            s = ba2hex(a, group=randrange(10), sep=choice(whitespace))
            b = hex2ba(s, endian)
            self.assertEqual(b.endian, endian or get_default_endian())
            self.assertEqual(a, b)
            self.check_obj(b)

    def test_hexdigits(self):
        a = hex2ba(hexdigits)
        self.assertEqual(len(a), 4 * len(hexdigits))
        self.assertEqual(type(a), bitarray)
        self.check_obj(a)

        t = ba2hex(a)
        self.assertEqual(t, hexdigits.lower())
        self.assertEqual(type(t), str)
        self.assertEQUAL(a, hex2ba(t))

    def test_binascii(self):
        a = urandom(80, 'big')
        s = binascii.hexlify(a.tobytes()).decode()
        self.assertEqual(ba2hex(a), s)
        b = bitarray(binascii.unhexlify(s), endian='big')
        self.assertEQUAL(hex2ba(s, 'big'), b)

# --------------------------  ba2base()  base2ba()  -------------------------

class BaseTests(unittest.TestCase, Util):

    def test_explicit(self):
        data = [ #              n  little   big
            ('',                2, '',      ''),
            ('1 0 1',           2, '101',   '101'),
            ('11 01 00',        4, '320',   '310'),
            ('111 001',         8, '74',    '71'),
            ('1111 0001',      16, 'f8',    'f1'),
            ('11111 00001',    32, '7Q',    '7B'),
            ('111111 000001',  64, '/g',    '/B'),
        ]
        for bs, n, s_le, s_be in data:
            a_le = bitarray(bs, 'little')
            a_be = bitarray(bs, 'big')
            self.assertEQUAL(base2ba(n, s_le, 'little'), a_le)
            self.assertEQUAL(base2ba(n, s_be, 'big'),    a_be)
            self.assertEqual(ba2base(n, a_le), s_le)
            self.assertEqual(ba2base(n, a_be), s_be)

    def test_base2ba_types(self):
        for c in '7', b'7', bytearray(b'7'):
            a = base2ba(32, c)
            self.assertEqual(a.to01(), '11111')
            self.assertEqual(type(a), bitarray)

    def test_base2ba_whitespace(self):
        self.assertEqual(base2ba(8, bytearray(b"17 0"), "little"),
                         bitarray("100 111 000"))
        self.assertEqual(base2ba(32, "7 A"), bitarray("11111 00000"))
        self.assertEqual(base2ba(64, b"A /"), bitarray("000000 111111"))
        for n in 2, 4, 8, 16, 32, 64:
            a = base2ba(n, whitespace)
            self.assertEqual(a, bitarray())
            a = urandom(60)
            c = list(ba2base(n, a))
            for _ in range(randrange(80)):
                c.insert(randint(0, len(c)), choice(whitespace))
            s = ''.join(c)
            self.assertEqual(base2ba(n, s), a)

    def test_ba2base_group(self):
        a = bitarray("001 011 100 111", "little")
        self.assertEqual(ba2base(8, a, 3), "461 7")
        self.assertEqual(ba2base(8, a, group=2), "46 17")
        self.assertEqual(ba2base(8, a, sep="_", group=2), "46_17")
        self.assertEqual(ba2base(8, a, 2, sep="."), "46.17")
        for n, s, group, sep, res in [
                (2, '10100', 2, '-', '10-10-0'),
                (4, '10 11 00 01', 1, "_", "2_3_0_1"),
                (8, "101 100 011 101 001 010", 3, "  ", "543  512"),
                (8, "101 100 011 101 001 010", 3, "", "543512"),
                (16, '1011 0001 1101 1010 1111', 4, "+", "b1da+f"),
                (32, "10110 00111 01101 01111", 2, ", ", "WH, NP"),
                (64, "101100 011101 101011 111110 101110", 2, ".", "sd.r+.u"),
                ]:
            a = bitarray(s, "big")
            s = ba2base(n, a, group, sep)
            self.assertEqual(type(s), str)
            self.assertEqual(s, res)

    def test_empty(self):
        for n in 2, 4, 8, 16, 32, 64:
            a = base2ba(n, '')
            self.assertEqual(a, bitarray())
            self.assertEqual(ba2base(n, a), '')

    def test_invalid_characters(self):
        for n, s in ((2, '2'), (4, '4'), (8, '8'), (16, 'g'), (32, '8'),
                     (32, '1'), (32, 'a'), (64, '-'), (64, '_')):
            msg = ("invalid digit found for base%d, "
                   "got '%s' (0x%02x)" % (n, s, ord(s)))
            self.assertRaisesMessage(ValueError, msg, base2ba, n, s)

        for n in 2, 4, 8, 16, 32, 64:
            for s in '_', '@', '[', '\u20ac', '\0',  b'\0', b'\x80', b'\xff':
                self.assertRaises(ValueError, base2ba, n, s)
            msg = "invalid digit found for base%d, got '{' (0x7b)" % n
            self.assertRaisesMessage(ValueError, msg, base2ba, n, '{')

    def test_invalid_args(self):
        a = bitarray()
        self.assertRaises(TypeError, ba2base, None, a)
        self.assertRaises(TypeError, base2ba, None, '')
        self.assertRaises(TypeError, ba2base, 16.0, a)
        self.assertRaises(TypeError, base2ba, 16.0, '')
        self.assertRaises(TypeError, ba2base, 32, None)
        self.assertRaises(TypeError, base2ba, 32, None)

        for values, msg in [
                ([-1023, -16, -1, 0, 3, 5, 31, 48, 63, 129, 511, 4123],
                 "base must be a power of 2"),
                ([1, 128, 256, 512, 1024, 2048, 4096, 8192],
                 "base must be 2, 4, 8, 16, 32 or 64")]:
            for i in values:
                self.assertRaisesMessage(ValueError, msg, ba2base, i, a)
                self.assertRaisesMessage(ValueError, msg, base2ba, i, '')

        a = bitarray(29)
        for m in range(2, 7):
            msg = "bitarray length 29 not multiple of %d" % m
            self.assertRaisesMessage(ValueError, msg, ba2base, 1 << m, a)

    def test_hexadecimal(self):
        a = base2ba(16, 'F61', 'big')
        self.assertEqual(a, bitarray('1111 0110 0001'))
        self.assertEqual(ba2base(16, a), 'f61')

        for n in range(50):
            s = ''.join(choices(hexdigits, k=n))
            endian = self.random_endian()
            a = base2ba(16, s, endian)
            self.assertEQUAL(a, hex2ba(s, endian))
            self.assertEqual(ba2base(16, a), ba2hex(a))

    def test_base32(self):
        msg = os.urandom(randint(10, 100) * 5)
        s = base64.b32encode(msg).decode()
        a = base2ba(32, s, 'big')
        self.assertEqual(a.tobytes(), msg)
        self.assertEqual(ba2base(32, a), s)
        self.assertEqual(base64.b32decode(s), msg)

    def test_base64(self):
        msg = os.urandom(randint(10, 100) * 3)
        s = base64.standard_b64encode(msg).decode()
        a = base2ba(64, s, 'big')
        self.assertEqual(a.tobytes(), msg)
        self.assertEqual(ba2base(64, a), s)
        self.assertEqual(base64.standard_b64decode(s), msg)

    def test_primes(self):
        primes = gen_primes(60, odd=True)
        base_2 = primes.to01()
        for n, endian, rep in [
                ( 2, "little", base_2),
                ( 2, "big",    base_2),
                ( 4, "little", "232132030132012122122010132110"),
                ( 4, "big",    "131231030231021211211020231220"),
                ( 8, "little", "65554155441515405550"),
                ( 8, "big",    "35551455114545105550"),
                (16, "little", "e6bc4b46a921d61"),
                (16, "big",    "76d32d265948b68"),
                (32, "little", "O3SJLSJTSI3C"),
                (32, "big",    "O3JS2JSZJC3I"),
                (64, "little", "utMtkppEtF"),
                (64, "big",    "dtMtJllIto"),
        ]:
            a = bitarray(primes, endian)
            s = ba2base(n, a)
            self.assertEqual(type(s), str)
            self.assertEqual(s, rep)
            b = base2ba(n, rep, endian)
            self.assertEqual(b, a)
            self.assertEqual(type(b), bitarray)
            self.assertEqual(b.endian, endian)

    alphabets = [
    #    m   n  alphabet
        (1,  2, '01'),
        (2,  4, '0123'),
        (3,  8, '01234567'),
        (4, 16, '0123456789abcdef'),
        (4, 16, '0123456789ABCDEF'),
        (5, 32, 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567'),
        (6, 64, 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdef'
                'ghijklmnopqrstuvwxyz0123456789+/'),
    ]

    def test_alphabets(self):
        for m, n, alphabet in self.alphabets:
            self.assertEqual(1 << m, n)
            self.assertEqual(len(alphabet), n)
            for i, c in enumerate(alphabet):
                endian = self.random_endian()
                self.assertEqual(ba2int(base2ba(n, c, endian)), i)
                if m == 4 and c in "ABCDEF":
                    c = chr(ord(c) + 32)
                self.assertEqual(ba2base(n, int2ba(i, m, endian)), c)

    def test_not_alphabets(self):
        for m, n, alphabet in self.alphabets:
            for i in range(256):
                c = chr(i)
                if c in alphabet or c.isspace():
                    continue
                if n == 16 and c in hexdigits:
                    continue
                self.assertRaises(ValueError, base2ba, n, c)

    def test_random(self):
        for _ in range(100):
            m = randint(1, 6)
            a = urandom_2(m * randrange(100))
            n = 1 << m
            s = ba2base(n, a, group=randrange(10), sep=randrange(5) * " ")
            if m == 4 and getrandbits(1):
                s = s.upper()
            if getrandbits(1):
                s = s.encode()
            b = base2ba(n, s, a.endian)
            self.assertEQUAL(a, b)
            self.check_obj(b)

# --------------------------- sparse compression ----------------------------

class SC_Tests(unittest.TestCase, Util):

    def test_explicit(self):
        for b, bits, endian in [
                (b'\x00\0',                 '',                  'little'),
                (b'\x01\x03\x01\x03\0',     '110',               'little'),
                (b'\x01\x07\x01\x40\0',     '0000001',           'little'),
                (b'\x11\x07\x01\x02\0',     '0000001',           'big'),
                (b'\x01\x10\x02\xf0\x0f\0', '00001111 11110000', 'little'),
                (b'\x11\x10\xa1\x0c\0',     '00000000 00001000', 'big'),
                (b'\x11\x09\xa1\x08\0',     '00000000 1',        'big'),
                (b'\x01g\xa4abde\0',        97 * '0' + '110110', 'little'),
        ]:
            a = bitarray(bits, endian)
            self.assertEqual(sc_encode(a), b)
            self.assertEQUAL(sc_decode(b), a)

    def test_encode_types(self):
        for a in bitarray('1', 'big'), frozenbitarray('1', 'big'):
            b = sc_encode(a)
            self.assertEqual(type(b), bytes)
            self.assertEqual(b, b'\x11\x01\x01\x80\0')

        for a in None, [], 0, 123, b'', b'\x00', 3.14:
            self.assertRaises(TypeError, sc_encode, a)

    def test_decode_types(self):
        blob = b'\x11\x03\x01\x20\0'
        for b in blob, bytearray(blob), list(blob), array.array('B', blob):
            a = sc_decode(b)
            self.assertEqual(type(a), bitarray)
            self.assertEqual(a.endian, 'big')
            self.assertEqual(a.to01(), '001')

        a = [17, 3, 1, 32, 0]
        self.assertEqual(sc_decode(a), bitarray("001"))
        for x in 256, -1:
            a[-1] = x
            self.assertRaises(ValueError, sc_decode, a)

        self.assertRaises(TypeError, sc_decode, [0x02, None])
        for x in None, 3, 3.2, Ellipsis, 'foo':
            self.assertRaises(TypeError, sc_decode, x)

    def test_decode_header_nbits(self):
        for b, n in [
                (b'\x00\0', 0),
                (b'\x01\x00\0', 0),
                (b'\x01\x01\0', 1),
                (b'\x02\x00\x00\0', 0),
                (b'\x02\x00\x01\0', 256),
                (b'\x03\x00\x00\x00\0', 0),
                (b'\x03\x00\x00\x01\0', 65536),
        ]:
            a = sc_decode(b)
            self.assertEqual(len(a), n)
            self.assertFalse(a.any())

    def test_decode_untouch(self):
        stream = iter(b'\x01\x03\x01\x03\0XYZ')
        self.assertEqual(sc_decode(stream), bitarray('110'))
        self.assertEqual(next(stream), ord('X'))

        stream = iter([0x11, 0x05, 0x01, 0xff, 0, None, 'foo'])
        self.assertEqual(sc_decode(stream), bitarray('11111'))
        self.assertTrue(next(stream) is None)
        self.assertEqual(next(stream), 'foo')

    def test_decode_header_errors(self):
        # invalid header
        for c in 0x20, 0x21, 0x40, 0x80, 0xc0, 0xf0, 0xff:
            self.assertRaisesMessage(ValueError,
                                     "invalid header: 0x%02x" % c,
                                     sc_decode, [c])
        # invalid block head
        for c in 0xc0, 0xc1, 0xc5, 0xff:
            self.assertRaisesMessage(ValueError,
                                     "invalid block head: 0x%02x" % c,
                                     sc_decode, [0x01, 0x10, c])

    def test_decode_header_overflow(self):
        self.assertRaisesMessage(
            OverflowError,
            "sizeof(Py_ssize_t) = %d: cannot read 9 bytes" % PTRSIZE,
            sc_decode, b'\x09' + 9 * b'\x00')

        self.assertRaisesMessage(
            ValueError,
            "read %d bytes got negative value: -1" % PTRSIZE,
            sc_decode, [PTRSIZE] + PTRSIZE * [0xff])

        if PTRSIZE == 4:
            self.assertRaisesMessage(
                OverflowError,
                "sizeof(Py_ssize_t) = 4: cannot read 5 bytes",
                sc_decode, b'\x05' + 5 * b'\x00')

            self.assertRaisesMessage(
                ValueError,
                "read 4 bytes got negative value: -2147483648",
                sc_decode, b'\x04\x00\x00\x00\x80')

    def test_decode_errors(self):
        # too many raw bytes
        self.assertRaisesMessage(
            ValueError, "decode error (raw): 0 + 2 > 1",
            sc_decode, b"\x01\x05\x02\xff\xff\0")
        self.assertRaisesMessage(
            ValueError, "decode error (raw): 32 + 3 > 34",
            sc_decode, b"\x02\x0f\x01\xa0\x03\xff\xff\xff\0")

        # sparse index too high
        self.assertRaisesMessage(
            ValueError, "decode error (n=1): 128 >= 128",
            sc_decode, b"\x01\x80\xa1\x80\0")
        self.assertRaisesMessage(
            ValueError, "decode error (n=2): 512 >= 512",
            sc_decode, b"\x02\x00\x02\xc2\x01\x00\x02\0")
        self.assertRaisesMessage(
            ValueError, "decode error (n=3): 32768 >= 32768",
            sc_decode, b"\x02\x00\x80\xc3\x01\x00\x80\x00\0")

        msg = {4: "read 4 bytes got negative value: -2147483648",
               8: "decode error (n=4): 2147483648 >= 16"}
        self.assertRaisesMessage(
            ValueError, msg[PTRSIZE],
            sc_decode, b"\x01\x10\xc4\x01\x00\x00\x00\x80\0")

        msg = {4: "read 4 bytes got negative value: -1",
               8: "decode error (n=4): 4294967295 >= 16"}
        self.assertRaisesMessage(
            ValueError, msg[PTRSIZE],
            sc_decode, b"\x01\x10\xc4\x01\xff\xff\xff\xff\0")

    def test_decode_end_of_stream(self):
        for stream in [b'', b'\x00', b'\x01', b'\x02\x77',
                       b'\x01\x04\x01', b'\x01\x04\xa1', b'\x01\x04\xa0']:
            self.assertRaises(StopIteration, sc_decode, stream)

    def test_decode_ambiguity(self):
        for b in [
                # raw:
                b'\x11\x03\x01\x20\0',    # this is what sc_encode gives us
                b'\x11\x03\x01\x3f\0',    # but we can set the pad bits to 1
                # sparse:
                b'\x11\x03\xa1\x02\0',                  # block type 1
                b'\x11\x03\xc2\x01\x02\x00\0',          # block type 2
                b'\x11\x03\xc3\x01\x02\x00\x00\0',      # block type 3
                b'\x11\x03\xc4\x01\x02\x00\x00\x00\0',  # block type 4
        ]:
            a = sc_decode(b)
            self.assertEqual(a.to01(), '001')

    def test_block_type0(self):
        for k in range(0x01, 0xa0):
            nbytes = k if k <= 32 else 32 * (k - 31)
            nbits = 8 * nbytes
            a = ones(nbits, "little")
            b = bytearray([0x01, nbits] if nbits < 256 else
                          [0x02, nbits % 256, nbits // 256])
            b.append(k)
            b.extend(a.tobytes())
            b.append(0)  # stop byte

            self.assertEqual(sc_decode(b), a)
            self.assertEqual(sc_encode(a), b)

    def test_block_type1(self):
        a = bitarray(256, 'little')
        for n in range(1, 32):
            a[getrandbits(8)] = 1

            b = bytearray([0x02, 0x00, 0x01, 0xa0 + a.count()])
            b.extend(list(a.search(1)))  # sorted indices with no duplicates
            b.append(0)  # stop byte

            self.assertEqual(sc_decode(b), a)
            self.assertEqual(sc_encode(a), b)

    def test_block_type2(self):
        a = bitarray(65536, 'little')
        for n in range(1, 256):
            a[getrandbits(16)] = 1

            b = bytearray([0x03, 0x00, 0x00, 0x01, 0xc2, a.count()])
            for i in a.search(1):
                b.extend(struct.pack("<H", i))
            b.append(0)  # stop byte
            self.assertEqual(sc_decode(b), a)
            if n < 250:
                # We cannot compare for the highest populations, as for
                # such high values sc_encode() may find better compression
                # with type 1 blocks.
                self.assertEqual(sc_encode(a), b)
            else:
                self.assertTrue(len(sc_encode(a)) <= len(b))

    def test_block_type3(self):
        a = bitarray(16_777_216, 'little')
        a[choices(range(1 << 24), k=255)] = 1
        b = bytearray([0x04, 0x00, 0x00, 0x00, 0x01, 0xc3, a.count()])
        for i in a.search(1):
            b.extend(struct.pack("<I", i)[:3])
        b.append(0)  # stop byte
        self.assertEqual(sc_decode(b), a)
        self.assertEqual(sc_encode(a), b)

    def test_block_type4(self):
        a = bitarray(1 << 26, 'little')
        # To understand why we cannot have a population larger than 5 for
        # an array size 4 times the size of a type 3 block, take a look
        # at the cost comparison in sc_encode_block().  (2 + 6 >= 2 * 4)
        indices = sorted(set(choices(range(len(a)), k=5)))
        a[indices] = 1
        b = bytearray(b'\x04\x00\x00\x00\x04\xc4')
        b.append(len(indices))
        for i in indices:
            b.extend(struct.pack("<I", i))
        b.append(0)  # stop byte
        self.assertEqual(sc_decode(b), a)
        self.assertEqual(sc_encode(a), b)

    def test_decode_random_bytes(self):
        # ensure random input doesn't crash the decoder
        for _ in range(100):
            n = randrange(20)
            b = b'\x02\x00\x04' + os.urandom(n)
            try:
                a = sc_decode(b)
            except (StopIteration, ValueError):
                continue
            self.assertEqual(len(a), 1024)
            self.assertEqual(a.endian, 'little')

    def check_blob_length(self, a, m):
        blob = sc_encode(a)
        self.assertEqual(len(blob), m)
        self.assertEqual(sc_decode(blob), a)

    def test_encode_zeros(self):
        for i in range(26):
            n = 1 << i
            a = zeros(n)
            m = 2                            # head byte and stop byte
            m += bits2bytes(n.bit_length())  # size of n in bytes
            self.check_blob_length(a, m)

            a[0] = 1
            m += 2                  # block head byte and one index byte
            m += 2 * bool(i > 9)    # count byte and second index byte
            m += bool(i > 16)       # third index byte
            m += bool(i > 24)       # fourth index byte
            self.check_blob_length(a, m)

    def test_encode_ones(self):
        for _ in range(10):
            nbits = randrange(100_000)
            a = ones(nbits)
            m = 2                                # head byte and stop byte
            m += bits2bytes(nbits.bit_length())  # size bytes
            nbytes = bits2bytes(nbits)
            m += nbytes                       # actual raw bytes
            # number of head bytes, all of block type 0:
            m += bool(nbytes % 32)            # number in 0x01 .. 0x1f
            m += (nbytes // 32 + 127) // 128  # number in 0x20 .. 0xbf
            self.check_blob_length(a, m)

    def round_trip(self, a):
        c = a.copy()
        i = iter(sc_encode(a))
        b = sc_decode(i)
        self.assertTrue(a == b == c)
        self.assertTrue(a.endian == b.endian == c.endian)
        self.assertEqual(list(i), [])

    def test_random(self):
        for _ in range(10):
            n = randrange(100_000)
            endian = self.random_endian()
            a = ones(n, endian)
            while a.count():
                a &= urandom(n, endian)
                self.round_trip(a)

# ---------------------------------------------------------------------------

class VLFTests(unittest.TestCase, Util):

    def test_explicit(self):
        for blob, s in [
                (b'\x40', ''),
                (b'\x30', '0'),
                (b'\x38', '1'),
                (b'\x00', '0000'),
                (b'\x01', '0001'),
                (b'\xd3\x20', '001101'),
                (b'\xe0\x40', '0000 1'),
                (b'\x90\x02', '0000 000001'),
                (b'\xb5\xa7\x18', '0101 0100111 0011'),
                (b'\x95\xb7\x1c', '0101 0110111 001110'),
        ]:
            a = bitarray(s)
            self.assertEqual(vl_encode(a), blob)
            c = vl_decode(blob)
            self.assertEqual(c, a)
            self.assertEqual(c.endian, get_default_endian())

            for endian in 'big', 'little', None:
                a = bitarray(s, endian)
                c = vl_encode(a)
                self.assertEqual(type(c), bytes)
                self.assertEqual(c, blob)

                c = vl_decode(blob, endian)
                self.assertEqual(c, a)
                self.assertEqual(c.endian, endian or get_default_endian())

    def test_encode_types(self):
        s = "0011 01"
        for a in bitarray(s), frozenbitarray(s):
            b = vl_encode(a)
            self.assertEqual(type(b), bytes)
            self.assertEqual(b, b'\xd3\x20')

        for a in None, [], 0, 123, b'', b'\x00', 3.14:
            self.assertRaises(TypeError, vl_encode, a)

    def test_decode_types(self):
        blob = b'\xd3\x20'
        for s in (blob, iter(blob), memoryview(blob), iter([0xd3, 0x20]),
                  bytearray(blob)):
            a = vl_decode(s, endian=self.random_endian())
            self.assertEqual(type(a), bitarray)
            self.assertEqual(a, bitarray('0011 01'))

        # these objects are not iterable
        for arg in None, 0, 1, 0.0:
            self.assertRaises(TypeError, vl_decode, arg)
        # these items cannot be interpreted as ints
        for item in None, 2.34, Ellipsis, 'foo':
            self.assertRaises(TypeError, vl_decode, iter([0x95, item]))

    def test_decode_args(self):
        # item not integer
        self.assertRaises(TypeError, vl_decode, iter([b'\x40']))

        self.assertRaises(TypeError, vl_decode, b'\x40', 'big', 3)
        self.assertRaises(ValueError, vl_decode, b'\x40', 'foo')

    def test_decode_trailing(self):
        for s, bits in [(b'\x40ABC', ''),
                        (b'\xe0\x40A', '00001')]:
            stream = iter(s)
            self.assertEqual(vl_decode(stream), bitarray(bits))
            self.assertEqual(next(stream), 65)

    def test_decode_ambiguity(self):
        for s in b'\x40', b'\x4f', b'\x45':
            self.assertEqual(vl_decode(s), bitarray())
        for s in b'\x1e', b'\x1f':
            self.assertEqual(vl_decode(s), bitarray('111'))

    def test_decode_stream(self):
        stream = iter(b'\x40\x30\x38\x40\x2c\xe0\x40\xd3\x20')
        for bits in '', '0', '1', '', '11', '0000 1', '0011 01':
            self.assertEqual(vl_decode(stream), bitarray(bits))

        arrays = [urandom(randrange(30)) for _ in range(1000)]
        stream = iter(b''.join(vl_encode(a) for a in arrays))
        for a in arrays:
            self.assertEqual(vl_decode(stream), a)

    def test_decode_errors(self):
        # decode empty bytes
        self.assertRaises(StopIteration, vl_decode, b'')
        # invalid head byte
        for s in [
                b'\x70', b'\xf0',           # padding = 7
                b'\x50', b'\x60', b'\x70',  # no second byte, but padding > 4
        ]:
            self.assertRaisesMessage(ValueError,
                                     "invalid head byte: 0x%02x" % s[0],
                                     vl_decode, s)
        # high bit set, but no terminating byte
        for s in b'\x80', b'\x80\x80':
            self.assertRaises(StopIteration, vl_decode, s)
        # decode list with out of range items
        for i in -1, 256:
            self.assertRaises(ValueError, vl_decode, [i])
        # wrong type
        self.assertRaises(TypeError, vl_decode, [None])

    def test_decode_invalid_stream(self):
        N = 100
        s = iter(N * (3 * [0x80] + ['XX']) + ['end.'])
        for _ in range(N):
            a = None
            try:
                a = vl_decode(s)
            except TypeError:
                pass
            self.assertTrue(a is None)
        self.assertEqual(next(s), 'end.')

    def test_explicit_zeros(self):
        for n in range(100):
            a = zeros(4 + n * 7)
            s = n * b'\x80' + b'\x00'
            self.assertEqual(vl_encode(a), s)
            self.assertEqual(vl_decode(s), a)

    def round_trip(self, a):
        c = a.copy()
        s = vl_encode(a)
        b = vl_decode(s)
        self.check_obj(b)
        self.assertTrue(a == b == c)
        LEN_PAD_BITS = 3
        self.assertEqual(len(s), (len(a) + LEN_PAD_BITS + 6) // 7)

        head = s[0]
        padding = (head & 0x70) >> 4
        self.assertEqual(len(a) + padding, 7 * len(s) - LEN_PAD_BITS)

    def test_large(self):
        for _ in range(10):
            a = urandom(randrange(100_000))
            self.round_trip(a)

    def test_random(self):
        for a in self.randombitarrays():
            self.round_trip(a)

# ---------------------------------------------------------------------------

class IntegerizationTests(unittest.TestCase, Util):

    def test_ba2int(self):
        self.assertEqual(ba2int(bitarray('0')), 0)
        self.assertEqual(ba2int(bitarray('1')), 1)
        self.assertEqual(ba2int(bitarray('00101', 'big')), 5)
        self.assertEqual(ba2int(bitarray('00101', 'little')), 20)
        self.assertEqual(ba2int(frozenbitarray('11')), 3)
        self.assertRaises(ValueError, ba2int, bitarray())
        self.assertRaises(ValueError, ba2int, frozenbitarray())
        self.assertRaises(TypeError, ba2int, '101')
        a = bitarray('111')
        b = a.copy()
        self.assertEqual(ba2int(a), 7)
        # ensure original object wasn't altered
        self.assertEQUAL(a, b)

    def test_ba2int_frozen(self):
        for a in self.randombitarrays(start=1):
            b = frozenbitarray(a)
            self.assertEqual(ba2int(b), ba2int(a))
            self.assertEQUAL(a, b)

    def test_ba2int_random(self):
        for a in self.randombitarrays(start=1):
            b = bitarray(a, 'big')
            self.assertEqual(a, b)
            self.assertEqual(ba2int(b), int(b.to01(), 2))

    def test_ba2int_bytes(self):
        for n in range(1, 50):
            a = urandom_2(8 * n)
            c = bytearray(a.tobytes())
            i = 0
            for x in (c if a.endian == 'big' else reversed(c)):
                i <<= 8
                i |= x
            self.assertEqual(ba2int(a), i)

    def test_int2ba(self):
        self.assertEqual(int2ba(0), bitarray('0'))
        self.assertEqual(int2ba(1), bitarray('1'))
        self.assertEqual(int2ba(5), bitarray('101'))
        self.assertEQUAL(int2ba(6, endian='big'), bitarray('110', 'big'))
        self.assertEQUAL(int2ba(6, endian='little'),
                         bitarray('011', 'little'))
        self.assertRaises(TypeError, int2ba, 1.0)
        self.assertRaises(TypeError, int2ba, 1, 3.0)
        self.assertRaises(ValueError, int2ba, 1, 0)
        self.assertRaises(TypeError, int2ba, 1, 10, 123)
        self.assertRaises(ValueError, int2ba, 1, 10, 'asd')
        # signed integer requires length
        self.assertRaises(TypeError, int2ba, 100, signed=True)

    def test_signed(self):
        for s, i in [
                ('0',  0),
                ('1', -1),
                ('00',  0),
                ('10',  1),
                ('01', -2),
                ('11', -1),
                ('000',  0),
                ('100',  1),
                ('010',  2),
                ('110',  3),
                ('001', -4),
                ('101', -3),
                ('011', -2),
                ('111', -1),
                ('00000',   0),
                ('11110',  15),
                ('00001', -16),
                ('11111',  -1),
                ('00000000 0',    0),
                ('11111111 0',  255),
                ('00000000 1', -256),
                ('11111111 1',   -1),
        ]:
            self.assertEqual(ba2int(bitarray(s, 'little'), signed=1), i)
            self.assertEqual(ba2int(bitarray(s[::-1], 'big'), signed=1), i)

            len_s = len(bitarray(s))
            self.assertEQUAL(int2ba(i, len_s, 'little', signed=1),
                             bitarray(s, 'little'))
            self.assertEQUAL(int2ba(i, len_s, 'big', signed=1),
                             bitarray(s[::-1], 'big'))

    def test_zero(self):
        for endian in "little", "big":
            a = int2ba(0, endian=endian)
            self.assertEQUAL(a, bitarray('0', endian=endian))
            for n in range(1, 100):
                a = int2ba(0, length=n, endian=endian, signed=True)
                b = bitarray(n * '0', endian)
                self.assertEQUAL(a, b)
                for signed in 0, 1:
                    self.assertEqual(ba2int(b, signed=signed), 0)

    def test_negative_one(self):
        for endian in "little", "big":
            for n in range(1, 100):
                a = int2ba(-1, length=n, endian=endian, signed=True)
                b = bitarray(n * '1', endian)
                self.assertEQUAL(a, b)
                self.assertEqual(ba2int(b, signed=True), -1)

    def test_int2ba_overflow(self):
        self.assertRaises(OverflowError, int2ba, -1)
        self.assertRaises(OverflowError, int2ba, -1, 4)

        self.assertRaises(OverflowError, int2ba, 128, 7)
        self.assertRaises(OverflowError, int2ba, 64, 7, signed=1)
        self.assertRaises(OverflowError, int2ba, -65, 7, signed=1)

        for n in range(1, 20):
            self.assertRaises(OverflowError, int2ba, 1 << n, n)
            self.assertRaises(OverflowError, int2ba, 1 << (n - 1), n,
                              signed=1)
            self.assertRaises(OverflowError, int2ba, -(1 << (n - 1)) - 1, n,
                              signed=1)

    def test_int2ba_length(self):
        self.assertRaises(TypeError, int2ba, 0, 1.0)
        self.assertRaises(ValueError, int2ba, 0, 0)
        self.assertEqual(int2ba(5, length=6, endian='big'),
                         bitarray('000101'))
        for n in range(1, 100):
            ab = int2ba(1, n, 'big')
            al = int2ba(1, n, 'little')
            self.assertEqual(ab.endian, 'big')
            self.assertEqual(al.endian, 'little')
            self.assertEqual(len(ab), n),
            self.assertEqual(len(al), n)
            self.assertEqual(ab, bitarray((n - 1) * '0') + bitarray('1'))
            self.assertEqual(al, bitarray('1') + bitarray((n - 1) * '0'))

            ab = int2ba(0, n, 'big')
            al = int2ba(0, n, 'little')
            self.assertEqual(len(ab), n)
            self.assertEqual(len(al), n)
            self.assertEqual(ab, bitarray(n * '0', 'big'))
            self.assertEqual(al, bitarray(n * '0', 'little'))

            self.assertEqual(int2ba(2 ** n - 1), bitarray(n * '1'))
            self.assertEqual(int2ba(2 ** n - 1, endian='little'),
                             bitarray(n * '1'))

    def test_explicit(self):
        for i, sa in [( 0,     '0'),    (1,         '1'),
                      ( 2,    '10'),    (3,        '11'),
                      (25, '11001'),  (265, '100001001'),
                      (3691038, '1110000101001000011110')]:
            ab = bitarray(sa, 'big')
            al = bitarray(sa[::-1], 'little')
            self.assertEQUAL(int2ba(i), ab)
            self.assertEQUAL(int2ba(i, endian='big'), ab)
            self.assertEQUAL(int2ba(i, endian='little'), al)
            self.assertEqual(ba2int(ab), ba2int(al), i)

    def check_round_trip(self, i):
        for endian in 'big', 'little':
            a = int2ba(i, endian=endian)
            self.check_obj(a)
            self.assertEqual(a.endian, endian)
            self.assertTrue(len(a) > 0)
            # ensure we have no leading zeros
            if a.endian == 'big':
                self.assertTrue(len(a) == 1 or a.index(1) == 0)
            self.assertEqual(ba2int(a), i)
            if i > 0:
                self.assertEqual(i.bit_length(), len(a))
            # add a few trailing / leading zeros to bitarray
            if endian == 'big':
                a = zeros(randrange(4), endian) + a
            else:
                a = a + zeros(randrange(4), endian)
            self.assertEqual(a.endian, endian)
            self.assertEqual(ba2int(a), i)

    def test_many(self):
        for _ in range(20):
            self.check_round_trip(randrange(10 ** randint(3, 300)))

    @staticmethod
    def twos_complement(i, num_bits):
        # https://en.wikipedia.org/wiki/Two%27s_complement
        mask = 2 ** (num_bits - 1)
        return -(i & mask) + (i & ~mask)

    def test_random_signed(self):
        for a in self.randombitarrays(start=1):
            i = ba2int(a, signed=True)
            b = int2ba(i, len(a), a.endian, signed=True)
            self.assertEQUAL(a, b)

            j = ba2int(a, signed=False)  # unsigned
            if i >= 0:
                self.assertEqual(i, j)

            self.assertEqual(i, self.twos_complement(j, len(a)))

# ---------------------------------------------------------------------------

class MixedTests(unittest.TestCase, Util):

    def test_bin(self):
        for _ in range(20):
            i = randrange(1000)
            s = bin(i)
            self.assertEqual(s[:2], '0b')
            a = bitarray(s[2:], 'big')
            self.assertEqual(ba2int(a), i)
            t = a.to01()
            self.assertEqual(t, s[2:])
            self.assertEqual(int(t, 2), i)

    def test_oct(self):
        for _ in range(20):
            i = randrange(1000)
            s = oct(i)
            self.assertEqual(s[:2], '0o')
            a = base2ba(8, s[2:], 'big')
            self.assertEqual(ba2int(a), i)
            t = ba2base(8, a)
            self.assertEqual(t, s[2:])
            self.assertEqual(int(t, 8), i)

    def test_hex(self):
        for _ in range(20):
            i = randrange(1000)
            s = hex(i)
            self.assertEqual(s[:2], '0x')
            a = hex2ba(s[2:], 'big')
            self.assertEqual(ba2int(a), i)
            t = ba2hex(a)
            self.assertEqual(t, s[2:])
            self.assertEqual(int(t, 16), i)

    def test_bitwise(self):
        for a in self.randombitarrays(start=1):
            b = urandom(len(a), a.endian)
            aa = a.copy()
            bb = b.copy()
            i = ba2int(a)
            j = ba2int(b)
            self.assertEqual(ba2int(a & b), i & j)
            self.assertEqual(ba2int(a | b), i | j)
            self.assertEqual(ba2int(a ^ b), i ^ j)

            n = randint(0, len(a))
            if a.endian == 'big':
                self.assertEqual(ba2int(a >> n), i >> n)
                c = zeros(len(a), 'big') + a
                self.assertEqual(ba2int(c << n), i << n)

            self.assertEQUAL(a, aa)
            self.assertEQUAL(b, bb)

    def test_bitwise_inplace(self):
        for a in self.randombitarrays(start=1):
            b = urandom(len(a), a.endian)
            bb = b.copy()
            i = ba2int(a)
            j = ba2int(b)
            c = a.copy()
            c &= b
            self.assertEqual(ba2int(c), i & j)
            c = a.copy()
            c |= b
            self.assertEqual(ba2int(c), i | j)
            c = a.copy()
            c ^= b
            self.assertEqual(ba2int(c), i ^ j)
            self.assertEQUAL(b, bb)

            n = randint(0, len(a))
            if a.endian == 'big':
                c = a.copy()
                c >>= n
                self.assertEqual(ba2int(c), i >> n)
                c = zeros(len(a), 'big') + a
                c <<= n
                self.assertEqual(ba2int(c), i << n)

# ----------------------  serialize()  deserialize()  -----------------------

class SerializationTests(unittest.TestCase, Util):

    def test_explicit(self):
        for blob, endian, bits in [
                (b'\x00',         'little', ''),
                (b'\x07\x01',     'little', '1'),
                (b'\x17\x80',     'big',    '1'),
                (b'\x13\xf8',     'big',    '11111'),
                (b'\x00\x0f',     'little', '11110000'),
                (b'\x10\xf0',     'big',    '11110000'),
                (b'\x12\x87\xd8', 'big',    '10000111 110110')
        ]:
            a = bitarray(bits, endian)
            s = serialize(a)
            self.assertEqual(blob, s)
            self.assertEqual(type(s), bytes)

            b = deserialize(blob)
            self.assertEqual(b, a)
            self.assertEqual(b.endian, endian)
            self.assertEqual(type(b), bitarray)

    def test_serialize_args(self):
        for x in '0', 0, 1, b'\x00', 0.0, [0, 1], bytearray([0]):
            self.assertRaises(TypeError, serialize, x)
        # no arguments
        self.assertRaises(TypeError, serialize)
        # too many arguments
        self.assertRaises(TypeError, serialize, bitarray(), 1)

        for a in bitarray('0111', 'big'), frozenbitarray('0111', 'big'):
            self.assertEqual(serialize(a), b'\x14\x70')

    def test_deserialize_args(self):
        for x in 0, 1, False, True, None, '', '01', 0.0, [0, 1]:
            self.assertRaises(TypeError, deserialize, x)
        # no arguments
        self.assertRaises(TypeError, deserialize)
        # too many arguments
        self.assertRaises(TypeError, deserialize, b'\x00', 1)

        blob = b'\x03\x06'
        x = bitarray(blob)
        for s in blob, bytearray(blob), memoryview(blob), x:
            a = deserialize(s)
            self.assertEqual(a.to01(), '01100')
            self.assertEqual(a.endian, 'little')

    def test_invalid_bytes(self):
        self.assertRaises(ValueError, deserialize, b'')

        def check_msg(b):
            msg = "invalid header byte: 0x%02x" % b[0]
            self.assertRaisesMessage(ValueError, msg, deserialize, b)

        for i in range(256):
            b = bytearray([i])
            if i == 0 or i == 16:
                self.assertEqual(deserialize(b), bitarray())
            else:
                self.assertRaises(ValueError, deserialize, b)
                check_msg(b)

            b.append(0)
            if i < 32 and i % 16 < 8:
                self.assertEqual(deserialize(b), zeros(8 - i % 8))
            else:
                self.assertRaises(ValueError, deserialize, b)
                check_msg(b)

    def test_padbits_ignored(self):
        for blob, endian in [
                (b'\x07\x01', 'little'),
                (b'\x07\x03', 'little'),
                (b'\x07\xff', 'little'),
                (b'\x17\x80', 'big'),
                (b'\x17\xc0', 'big'),
                (b'\x17\xff', 'big'),
        ]:
            a = deserialize(blob)
            self.assertEqual(a.to01(), '1')
            self.assertEqual(a.endian, endian)

    def test_random(self):
        for a in self.randombitarrays():
            b = serialize(a)
            c = deserialize(b)
            self.assertEqual(a, c)
            self.assertEqual(a.endian, c.endian)
            self.check_obj(c)

# ---------------------------------------------------------------------------

class HuffmanTreeTests(unittest.TestCase):  # tests for _huffman_tree()

    def test_empty(self):
        freq = {}
        self.assertRaises(IndexError, _huffman_tree, freq)

    def test_one_symbol(self):
        freq = {"A": 1}
        tree = _huffman_tree(freq)
        self.assertEqual(tree.symbol, "A")
        self.assertEqual(tree.freq, 1)
        self.assertRaises(AttributeError, getattr, tree, 'child')

    def test_two_symbols(self):
        freq = {"A": 1, "B": 1}
        tree = _huffman_tree(freq)
        self.assertRaises(AttributeError, getattr, tree, 'symbol')
        self.assertEqual(tree.freq, 2)
        self.assertEqual(tree.child[0].symbol, "A")
        self.assertEqual(tree.child[0].freq, 1)
        self.assertEqual(tree.child[1].symbol, "B")
        self.assertEqual(tree.child[1].freq, 1)


class HuffmanTests(unittest.TestCase):

    def test_simple(self):
        freq = {0: 10, 'as': 2, None: 1.6}
        code = huffman_code(freq)
        self.assertEqual(len(code), 3)
        self.assertEqual(len(code[0]), 1)
        self.assertEqual(len(code['as']), 2)
        self.assertEqual(len(code[None]), 2)

    def test_endianness(self):
        freq = {'A': 10, 'B': 2, 'C': 5}
        for endian in 'big', 'little':
            code = huffman_code(freq, endian)
            self.assertEqual(len(code), 3)
            for v in code.values():
                self.assertEqual(v.endian, endian)

    def test_wrong_arg(self):
        self.assertRaises(TypeError, huffman_code, [('a', 1)])
        self.assertRaises(TypeError, huffman_code, 123)
        self.assertRaises(TypeError, huffman_code, None)
        # cannot compare 'a' with 1
        self.assertRaises(TypeError, huffman_code, {'A': 'a', 'B': 1})
        # frequency map cannot be empty
        self.assertRaises(ValueError, huffman_code, {})

    def test_one_symbol(self):
        cnt = {'a': 1}
        code = huffman_code(cnt)
        self.assertEqual(code, {'a': bitarray('0')})
        for n in range(4):
            msg = n * ['a']
            a = bitarray()
            a.encode(code, msg)
            self.assertEqual(a.to01(), n * '0')
            self.assertEqual(list(a.decode(code)), msg)
            a.append(1)
            self.assertRaises(ValueError, list, a.decode(code))

    def check_tree(self, code):
        n = len(code)
        tree = decodetree(code)
        self.assertEqual(tree.todict(), code)
        # ensure tree has 2n-1 nodes (n symbol nodes and n-1 internal nodes)
        self.assertEqual(tree.nodes(), 2 * n - 1)
        # a proper Huffman tree is complete
        self.assertTrue(tree.complete())

    def test_balanced(self):
        n = 6
        freq = {}
        for i in range(1 << n):
            freq[i] = 1
        code = huffman_code(freq)
        self.assertEqual(len(code), 1 << n)
        self.assertTrue(all(len(v) == n for v in code.values()))
        self.check_tree(code)

    def test_unbalanced(self):
        n = 27
        freq = {}
        for i in range(n):
            freq[i] = 1 << i
        code = huffman_code(freq)
        self.assertEqual(len(code), n)
        for i in range(n):
            self.assertEqual(len(code[i]), n - max(1, i))
        self.check_tree(code)

    def test_counter(self):
        message = 'the quick brown fox jumps over the lazy dog.'
        code = huffman_code(Counter(message))
        a = bitarray()
        a.encode(code, message)
        self.assertEqual(''.join(a.decode(code)), message)
        self.check_tree(code)

    def test_random_list(self):
        plain = choices(range(100), k=500)
        code = huffman_code(Counter(plain))
        a = bitarray()
        a.encode(code, plain)
        self.assertEqual(list(a.decode(code)), plain)
        self.check_tree(code)

    def test_random_freq(self):
        for n in 2, 3, 4, randint(5, 200):
            # create Huffman code for n symbols
            code = huffman_code({i: random() for i in range(n)})
            self.check_tree(code)

# ---------------------------------------------------------------------------

class CanonicalHuffmanTests(unittest.TestCase, Util):

    def test_basic(self):
        plain = bytearray(b'the quick brown fox jumps over the lazy dog.')
        chc, count, symbol = canonical_huffman(Counter(plain))
        self.assertEqual(type(chc), dict)
        self.assertEqual(type(count), list)
        self.assertEqual(type(symbol), list)
        a = bitarray()
        a.encode(chc, plain)
        self.assertEqual(bytearray(a.decode(chc)), plain)
        self.assertEqual(bytearray(canonical_decode(a, count, symbol)), plain)

    def test_example(self):
        cnt = {'a': 5, 'b': 3, 'c': 1, 'd': 1, 'r': 2}
        codedict, count, symbol = canonical_huffman(cnt)
        self.assertEqual(codedict, {'a': bitarray('0'),
                                    'b': bitarray('10'),
                                    'c': bitarray('1110'),
                                    'd': bitarray('1111'),
                                    'r': bitarray('110')})
        self.assertEqual(count, [0, 1, 1, 1, 2])
        self.assertEqual(symbol, ['a', 'b', 'r', 'c', 'd'])
        a = bitarray('01011001110011110101100')
        msg = "abracadabra"
        self.assertEqual(''.join(a.decode(codedict)), msg)
        self.assertEqual(''.join(canonical_decode(a, count, symbol)), msg)

    def test_canonical_huffman_errors(self):
        self.assertRaises(TypeError, canonical_huffman, [])
        # frequency map cannot be empty
        self.assertRaises(ValueError, canonical_huffman, {})
        self.assertRaises(TypeError, canonical_huffman)
        cnt = huffman_code(Counter('aabc'))
        self.assertRaises(TypeError, canonical_huffman, cnt, 'a')

    def test_one_symbol(self):
        cnt = {'a': 1}
        chc, count, symbol = canonical_huffman(cnt)
        self.assertEqual(chc, {'a': bitarray('0')})
        self.assertEqual(count, [0, 1])
        self.assertEqual(symbol, ['a'])
        for n in range(4):
            msg = n * ['a']
            a = bitarray()
            a.encode(chc, msg)
            self.assertEqual(a.to01(), n * '0')
            self.assertEqual(list(canonical_decode(a, count, symbol)), msg)
            a.append(1)
            self.assertRaises(ValueError, list,
                              canonical_decode(a, count, symbol))

    def test_canonical_decode_errors(self):
        a = bitarray('1101')
        s = ['a']
        # bitarray not of bitarray type
        self.assertRaises(TypeError, canonical_decode, '11', [0, 1], s)
        # count not sequence
        self.assertRaises(TypeError, canonical_decode, a, {0, 1}, s)
        # count element not an int
        self.assertRaises(TypeError, canonical_decode, a, [0, 1.0], s)
        # count element overflow
        self.assertRaises(OverflowError, canonical_decode, a, [0, 1 << 65], s)
        # symbol not sequence
        self.assertRaises(TypeError, canonical_decode, a, [0, 1], 43)

        symbol = ['a', 'b', 'c', 'd']
        # sum(count) != len(symbol)
        self.assertRaisesMessage(ValueError,
                                 "sum(count) = 3, but len(symbol) = 4",
                                 canonical_decode, a, [0, 1, 2], symbol)
        # count list too long
        self.assertRaisesMessage(ValueError,
                                 "len(count) cannot be larger than 32",
                                 canonical_decode, a, 33 * [0], symbol)

    def test_canonical_decode_count_range(self):
        a = bitarray()
        for i in range(1, 32):
            count = 32 * [0]
            # negative count
            count[i] = -1
            self.assertRaisesMessage(ValueError,
                "count[%d] not in [0..%d], got -1" % (i, 1 << i),
                canonical_decode, a, count, [])

            maxbits = 1 << i
            count[i] = maxbits
            if i == 31 and PTRSIZE == 4:
                self.assertRaises(OverflowError,
                                  canonical_decode, a, count, [])
                continue
            self.assertRaisesMessage(ValueError,
                "sum(count) = %d, but len(symbol) = 0" % maxbits,
                canonical_decode, a, count, [])

            count[i] = maxbits + 1
            self.assertRaisesMessage(ValueError,
                "count[%d] not in [0..%d], got %d" % (i, maxbits, count[i]),
                canonical_decode, a, count, [])

        iter = canonical_decode(a, 32 * [0], [])
        self.assertEqual(list(iter), [])

    def test_canonical_decode_simple(self):
        # symbols can be anything, they do not even have to be hashable here
        cnt = [0, 0, 4]
        s = ['A', 42, [1.2-3.7j, 4j], {'B': 6}]
        a = bitarray('00 01 10 11')
        # count can be a list
        self.assertEqual(list(canonical_decode(a, cnt, s)), s)
        # count can also be a tuple (any sequence object in fact)
        self.assertEqual(list(canonical_decode(a, (0, 0, 4), s)), s)
        self.assertEqual(list(canonical_decode(7 * a, cnt, s)), 7 * s)
        # the count list may have extra 0's at the end (but not too many)
        count = [0, 0, 4, 0, 0, 0, 0, 0]
        self.assertEqual(list(canonical_decode(a, count, s)), s)
        # the element count[0] is unused
        self.assertEqual(list(canonical_decode(a, [-47, 0, 4], s)), s)
        # in fact it can be anything, as it is entirely ignored
        self.assertEqual(list(canonical_decode(a, [None, 0, 4], s)), s)

        # the symbol argument can be any sequence object
        s = [65, 66, 67, 98]
        self.assertEqual(list(canonical_decode(a, cnt, s)), s)
        self.assertEqual(list(canonical_decode(a, cnt, bytearray(s))), s)
        self.assertEqual(list(canonical_decode(a, cnt, tuple(s))), s)
        self.assertEqual(list(canonical_decode(a, cnt, bytes(s))), s)
        # Implementation Note:
        #   The symbol can even be an iterable.  This was done because we
        #   want to use PySequence_Fast in order to convert sequence
        #   objects (like bytes and bytearray) to a list.  This is faster
        #   as all objects are now elements in an array of pointers (as
        #   opposed to having the object's __getitem__ method called on
        #   every iteration).
        self.assertEqual(list(canonical_decode(a, cnt, iter(s))), s)

    def test_canonical_decode_empty(self):
        a = bitarray()
        # count and symbol are empty, ok because sum([]) == len([])
        self.assertEqual(list(canonical_decode(a, [], [])), [])
        a.append(0)
        self.assertRaisesMessage(ValueError, "reached end of bitarray",
                                 list, canonical_decode(a, [], []))
        a = bitarray(31 * '0')
        self.assertRaisesMessage(ValueError, "ran out of codes",
                                 list, canonical_decode(a, [], []))

    def test_canonical_decode_one_symbol(self):
        symbols = ['A']
        count = [0, 1]
        a = bitarray('000')
        self.assertEqual(list(canonical_decode(a, count, symbols)),
                         3 * symbols)
        a.append(1)
        a.extend(bitarray(10 * '0'))
        iterator = canonical_decode(a, count, symbols)
        self.assertRaisesMessage(ValueError, "reached end of bitarray",
                                 list, iterator)

        a.extend(bitarray(20 * '0'))
        iterator = canonical_decode(a, count, symbols)
        self.assertRaisesMessage(ValueError, "ran out of codes",
                                 list, iterator)

    def test_canonical_decode_large(self):
        with open(__file__, 'rb') as f:
            msg = bytearray(f.read())
        self.assertTrue(len(msg) > 50000)
        codedict, count, symbol = canonical_huffman(Counter(msg))
        a = bitarray()
        a.encode(codedict, msg)
        self.assertEqual(bytearray(canonical_decode(a, count, symbol)), msg)
        self.check_code(codedict, count, symbol)

    def test_canonical_decode_symbol_change(self):
        msg = bytearray(b"Hello World!")
        codedict, count, symbol = canonical_huffman(Counter(msg))
        self.check_code(codedict, count, symbol)
        a = bitarray()
        a.encode(codedict, 10 * msg)

        it = canonical_decode(a, count, symbol)
        def decode_one_msg():
            return bytearray(next(it) for _ in range(len(msg)))

        self.assertEqual(decode_one_msg(), msg)
        symbol[symbol.index(ord("l"))] = ord("k")
        self.assertEqual(decode_one_msg(), bytearray(b"Hekko Workd!"))
        del symbol[:]
        self.assertRaises(IndexError, decode_one_msg)

    def ensure_sorted(self, chc, symbol):
        # ensure codes are sorted
        for i in range(len(symbol) - 1):
            a = chc[symbol[i]]
            b = chc[symbol[i + 1]]
            self.assertTrue(ba2int(a) < ba2int(b))

    def ensure_consecutive(self, chc, count, symbol):
        start = 0
        for nbits, cnt in enumerate(count):
            for i in range(start, start + cnt - 1):
                # ensure two consecutive codes (with same bit length) have
                # consecutive integer values
                a = chc[symbol[i]]
                b = chc[symbol[i + 1]]
                self.assertTrue(len(a) == len(b) == nbits)
                self.assertEqual(ba2int(a) + 1, ba2int(b))
            start += cnt

    def ensure_count(self, chc, count):
        # ensure count list corresponds to length counts from codedict
        maxbits = len(count) - 1
        self.assertEqual(maxbits, max(len(a) for a in chc.values()))
        my_count = (maxbits + 1) * [0]
        for a in chc.values():
            self.assertEqual(a.endian, 'big')
            my_count[len(a)] += 1
        self.assertEqual(my_count, count)

    def ensure_complete(self, count):
        # ensure code is complete and not oversubscribed
        len_c = len(count)
        x = sum(count[i] << (len_c - i) for i in range(1, len_c))
        self.assertEqual(x, 1 << len_c)

    def ensure_complete_2(self, chc):
        # ensure code is complete
        dt = decodetree(chc)
        self.assertTrue(dt.complete())

    def ensure_round_trip(self, chc, count, symbol):
        # create a short test message, encode and decode
        msg = choices(symbol, k=10)
        a = bitarray()
        a.encode(chc, msg)
        it = canonical_decode(a, count, symbol)
        # the iterator holds a reference to the bitarray and symbol list
        del a, count, symbol
        self.assertEqual(type(it).__name__, 'canonical_decodeiter')
        self.assertEqual(list(it), msg)

    def check_code(self, chc, count, symbol):
        self.assertTrue(len(chc) == len(symbol) == sum(count))
        self.assertEqual(count[0], 0)  # no codes have length 0
        self.assertTrue(set(chc) == set(symbol))
        # the code of the last symbol has all 1 bits
        self.assertTrue(chc[symbol[-1]].all())
        # the code of the first symbol starts with bit 0
        self.assertFalse(chc[symbol[0]][0])

        self.ensure_sorted(chc, symbol)
        self.ensure_consecutive(chc, count, symbol)
        self.ensure_count(chc, count)
        self.ensure_complete(count)
        self.ensure_complete_2(chc)
        self.ensure_round_trip(chc, count, symbol)

    def test_simple_counter(self):
        plain = bytearray(b'the quick brown fox jumps over the lazy dog.')
        cnt = Counter(plain)
        self.check_code(*canonical_huffman(cnt))

    def test_no_comp(self):
        freq = {None: 1, "A": 1}  # None and "A" are not comparable
        self.check_code(*canonical_huffman(freq))

    def test_balanced(self):
        n = 7
        freq = {}
        for i in range(1 << n):
            freq[i] = 1
        code, count, sym = canonical_huffman(freq)
        self.assertEqual(len(code), 1 << n)
        self.assertTrue(all(len(v) == n for v in code.values()))
        self.check_code(code, count, sym)

    def test_unbalanced(self):
        n = 32
        freq = {}
        for i in range(n):
            freq[i] = 1 << i
        code = canonical_huffman(freq)[0]
        self.assertEqual(len(code), n)
        for i in range(n):
            self.assertEqual(len(code[i]), n - max(1, i))
        self.check_code(*canonical_huffman(freq))

    def test_random_freq(self):
        for n in 2, 3, 4, randint(5, 200):
            freq = {i: random() for i in range(n)}
            self.check_code(*canonical_huffman(freq))

# ---------------------------------------------------------------------------

if __name__ == '__main__':
    unittest.main()
