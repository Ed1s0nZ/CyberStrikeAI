"""Methods as suggested in book.  Hackers Delight."""

from __future__ import annotations


def min_or(a, b, c, d, w):
    """
    Lower bound of result of ORing 2-intervals.

    :param a: Lower bound of first interval
    :param b: Upper bound of first interval
    :param c: Lower bound of second interval
    :param d: Upper bound of second interval
    :param w: bit width
    :return: Lower bound of ORing 2-intervals
    """
    m = 1 << (w - 1)
    while m != 0:
        if ((~a) & c & m) != 0:
            temp = (a | m) & -m
            if temp <= b:
                a = temp
                break
        elif (a & (~c) & m) != 0:
            temp = (c | m) & -m
            if temp <= d:
                c = temp
                break
        m >>= 1
    return a | c


def max_or(a, b, c, d, w):
    """
    Upper bound of result of ORing 2-intervals.

    :param a: Lower bound of first interval
    :param b: Upper bound of first interval
    :param c: Lower bound of second interval
    :param d: Upper bound of second interval
    :param w: bit width
    :return: Upper bound of ORing 2-intervals
    """
    m = 1 << (w - 1)
    while m != 0:
        if (b & d & m) != 0:
            temp = (b - m) | (m - 1)
            if temp >= a:
                b = temp
                break
            temp = (d - m) | (m - 1)
            if temp >= c:
                d = temp
                break
        m >>= 1
    return b | d


def min_and(a, b, c, d, w):
    """
    Lower bound of result of ANDing 2-intervals.

    :param a: Lower bound of first interval
    :param b: Upper bound of first interval
    :param c: Lower bound of second interval
    :param d: Upper bound of second interval
    :param w: bit width
    :return: Lower bound of ANDing 2-intervals
    """
    m = 1 << (w - 1)
    while m != 0:
        if (~a & ~c & m) != 0:
            temp = (a | m) & -m
            if temp <= b:
                a = temp
                break
            temp = (c | m) & -m
            if temp <= d:
                c = temp
                break
        m >>= 1
    return a & c


def max_and(a, b, c, d, w):
    """
    Upper bound of result of ANDing 2-intervals.

    :param a: Lower bound of first interval
    :param b: Upper bound of first interval
    :param c: Lower bound of second interval
    :param d: Upper bound of second interval
    :param w: bit width
    :return: Upper bound of ANDing 2-intervals
    """
    m = 1 << (w - 1)
    while m != 0:
        if ((~d) & b & m) != 0:
            temp = (b & ~m) | (m - 1)
            if temp >= a:
                b = temp
                break
        elif (d & (~b) & m) != 0:
            temp = (d & ~m) | (m - 1)
            if temp >= c:
                d = temp
                break
        m >>= 1
    return b & d


def min_xor(a, b, c, d, w):
    """
    Lower bound of result of XORing 2-intervals.

    :param a: Lower bound of first interval
    :param b: Upper bound of first interval
    :param c: Lower bound of second interval
    :param d: Upper bound of second interval
    :param w: bit width
    :return: Lower bound of XORing 2-intervals
    """
    m = 1 << (w - 1)
    while m != 0:
        if ((~a) & c & m) != 0:
            temp = (a | m) & -m
            if temp <= b:
                a = temp
        elif (a & (~c) & m) != 0:
            temp = (c | m) & -m
            if temp <= d:
                c = temp
        m >>= 1
    return a ^ c


def max_xor(a, b, c, d, w):
    """
    Upper bound of result of XORing 2-intervals.

    :param a: Lower bound of first interval
    :param b: Upper bound of first interval
    :param c: Lower bound of second interval
    :param d: Upper bound of second interval
    :param w: bit width
    :return: Upper bound of XORing 2-intervals
    """
    m = 1 << (w - 1)
    while m != 0:
        if (b & d & m) != 0:
            temp = (b - m) | (m - 1)
            if temp >= a:
                b = temp
            else:
                temp = (d - m) | (m - 1)
                if temp >= c:
                    d = temp
        m >>= 1
    return b ^ d
