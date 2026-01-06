import array
import math
import struct
import bitarray
from bitstring.luts import mxfp_luts_compressed
import zlib
from typing import Optional


def round_to_nearest_ties_to_even(lut_int_to_float, lower: int, f: float) -> Optional[int]:
    upper = lower + 1
    # Special case for LUTs without a negative zero.
    lower_float = 0.0 if lower == 128 else lut_int_to_float[lower]
    upper_float = lut_int_to_float[upper]
    if upper_float < lower_float:
        lower, upper = upper, lower
        lower_float, upper_float = upper_float, lower_float
    if f == lower_float:
        return lower
    if f == upper_float:
        return upper
    if lower_float < f < upper_float:
        d1 = f - lower_float
        d2 = upper_float - f
        if d1 < d2:
            return lower
        if d2 < d1:
            return upper
        return lower if lower % 2 == 0 else upper
    return None


class MXFPFormat:
    """Defining an MXFP micro-scaling floating point format"""

    def __init__(self, exp_bits: int, mantissa_bits: int, bias: int, mxfp_overflow: str):
        self.exp_bits = exp_bits
        self.mantissa_bits = mantissa_bits
        self.bias = bias
        self.mxfp_overflow = mxfp_overflow

        self.pos_clamp_value = (1 << (self.exp_bits + self.mantissa_bits)) - 1
        self.neg_clamp_value = (1 << (1 + self.exp_bits + self.mantissa_bits)) - 1

        # Special cases for e4m3 and e5m2
        if self.exp_bits == 4 and self.mantissa_bits == 3:
            if self.mxfp_overflow == 'saturate':
                self.pos_clamp_value = 0b01111110  # 448
                self.neg_clamp_value = 0b11111110  # -448
            else:
                self.pos_clamp_value = self.neg_clamp_value = 0b11111111  # NaN
        if self.exp_bits == 5 and self.mantissa_bits == 2:
            if self.mxfp_overflow == 'saturate':
                self.pos_clamp_value = 0b01111011  # 57344
                self.neg_clamp_value = 0b11111011  # -57344
            else:
                self.pos_clamp_value = 0b01111100  # +inf
                self.neg_clamp_value = 0b11111100  # -inf

        # If we calculate these LUTs now it creates a bootstrap problem in generate_luts.py.
        self.lut_float16_to_mxfp = None
        self.lut_int_to_float = None

    def __str__(self):
        return f"MXFPFormat(exp_bits={self.exp_bits}, mantissa_bits={self.mantissa_bits}, bias={self.bias}, mxfp_overflow='{self.mxfp_overflow}')"

    def decompress_luts(self):
        int_to_float_compressed, float16_to_mxfp_compressed = mxfp_luts_compressed[(self.exp_bits, self.mantissa_bits, self.bias, self.mxfp_overflow)]
        self.lut_float16_to_mxfp = zlib.decompress(float16_to_mxfp_compressed)
        dec = zlib.decompress(int_to_float_compressed)
        self.lut_int_to_float = struct.unpack(f'<{len(dec) // 4}f', dec)

    def create_luts(self):
        self.lut_int_to_float = self.createLUT_for_int_to_float()
        self.lut_float16_to_mxfp = self.createLUT_for_float16_to_mxfp()

    def float_to_int(self, f: float) -> int:
        """Given a Python float convert to the best mxfp float (expressed as an int) that represents it."""
        # First convert the float to a float16, then a 16 bit uint
        try:
            b = struct.pack('>e', f)
        except (OverflowError, struct.error):
            # Return the largest representable positive or negative value
            return self.pos_clamp_value if f > 0 else self.neg_clamp_value

        f16_int = int.from_bytes(b, byteorder='big')
        # Then use this as an index to our large LUT
        return self.lut_float16_to_mxfp[f16_int]

    def slow_float_to_int(self, f: float) -> int:
        # Slow, but easier to follow than the faster version.
        # The output int has the binary sequence needed for the float.
        length = 1 + self.exp_bits + self.mantissa_bits
        values = 1 << length
        # First get the NaN case out of the way
        if math.isnan(f):
            if length == 8:
                return 0xff  # Works for both e5m2 and e4m3
            # For smaller lengths, NaN isn't supported so we instead return an invalid value to detect later
            return 0xff
        # This is so we can distinguish between 0.0 and -0.0
        is_positive = math.copysign(1.0, f) == 1.0
        if is_positive:
            # Positive, so top bit is not set
            for i in range(values // 2 - 1):
                upper = self.lut_int_to_float[i + 1]
                if upper == float('inf'):
                    break
                x = round_to_nearest_ties_to_even(self.lut_int_to_float, i, f)
                if x is not None:
                    return x
            return self.pos_clamp_value
        else:
            # Negative, so top bit is set
            for i in range(values // 2, values - 1):
                lower = self.lut_int_to_float[i + 1]
                if lower == float('-inf'):
                    break
                x = round_to_nearest_ties_to_even(self.lut_int_to_float, i, f)
                if x is not None:
                    return x
            # Clip to negative max
            return self.neg_clamp_value

    def createLUT_for_int_to_float(self) -> array.array:
        """Create a LUT to convert an int in representing a MXFP float into a Python float"""
        i2f = []
        length = 1 + self.exp_bits + self.mantissa_bits
        for i in range(1 << length):
            b = bitarray.util.int2ba(i, length=length, endian='big', signed=False)
            sign = b[0]
            exponent = bitarray.util.ba2int(b[1:1 + self.exp_bits])
            significand = b[1 + self.exp_bits:]
            if exponent == 0:
                significand = bitarray.bitarray('0') + significand
                exponent = -self.bias + 1
            else:
                significand = bitarray.bitarray('1') + significand
                exponent -= self.bias
            f = float(bitarray.util.ba2int(significand)) / (2.0 ** self.mantissa_bits)
            f *= 2 ** exponent
            if length == 8:
                # Some special cases
                if self.exp_bits == 5:
                    if i in [0b01111100, 0b11111100]:
                        f = float('inf')
                    if i in [0b01111101, 0b11111101, 0b01111110, 0b11111110, 0b01111111, 0b11111111]:
                        f = float('nan')
                if self.exp_bits == 4:
                    if i in [0b01111111, 0b11111111]:
                        f = float('nan')
            i2f.append(f if not sign else -f)
        return array.array('f', i2f)

    def createLUT_for_float16_to_mxfp(self) -> bytes:
        """Create a LUT to convert a float16 into a MXFP format"""
        # Used to create the LUT that was compressed and stored for the fp8 code
        length = 1 + self.exp_bits + self.mantissa_bits
        if length == 8:
            import gfloat
            from gfloat.formats import format_info_ocp_e5m2, format_info_ocp_e4m3
            fi = format_info_ocp_e5m2 if self.exp_bits == 5 else format_info_ocp_e4m3

            fp16_to_fp8 = bytearray(1 << 16)
            for i in range(1 << 16):
                b = struct.pack('>H', i)
                f, = struct.unpack('>e', b)
                fp = gfloat.round_float(fi, f, sat=self.mxfp_overflow == 'saturate')
                if math.isnan(fp):
                    fp8_i = 0b11111111
                else:
                    # Special case for negative zero
                    if fp == 0.0 and math.copysign(1.0, fp) == -1.0:
                        fp8_i = 0b10000000
                    else:
                        fp8_i = self.lut_int_to_float.index(fp)
                fp16_to_fp8[i] = fp8_i
            return bytes(fp16_to_fp8)
        else:
            assert length in [4, 6]
            fp16_to_fp8 = bytearray(1 << 16)
            for i in range(1 << 16):
                b = struct.pack('>H', i)
                f, = struct.unpack('>e', b)
                fp8_i = self.slow_float_to_int(f)
                fp16_to_fp8[i] = fp8_i
            return bytes(fp16_to_fp8)


e2m1mxfp_fmt = MXFPFormat(exp_bits=2, mantissa_bits=1, bias=1, mxfp_overflow='saturate')
e2m3mxfp_fmt = MXFPFormat(exp_bits=2, mantissa_bits=3, bias=1, mxfp_overflow='saturate')
e3m2mxfp_fmt = MXFPFormat(exp_bits=3, mantissa_bits=2, bias=3, mxfp_overflow='saturate')
e4m3mxfp_saturate_fmt = MXFPFormat(exp_bits=4, mantissa_bits=3, bias=7, mxfp_overflow='saturate')
e5m2mxfp_saturate_fmt = MXFPFormat(exp_bits=5, mantissa_bits=2, bias=15, mxfp_overflow='saturate')
e4m3mxfp_overflow_fmt = MXFPFormat(exp_bits=4, mantissa_bits=3, bias=7, mxfp_overflow='overflow')
e5m2mxfp_overflow_fmt = MXFPFormat(exp_bits=5, mantissa_bits=2, bias=15, mxfp_overflow='overflow')


def decompress_luts():
    e2m1mxfp_fmt.decompress_luts()
    e2m3mxfp_fmt.decompress_luts()
    e3m2mxfp_fmt.decompress_luts()
    e4m3mxfp_saturate_fmt.decompress_luts()
    e5m2mxfp_saturate_fmt.decompress_luts()
    e4m3mxfp_overflow_fmt.decompress_luts()
    e5m2mxfp_overflow_fmt.decompress_luts()
