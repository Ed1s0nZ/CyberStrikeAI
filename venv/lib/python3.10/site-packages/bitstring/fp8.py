"""
The 8-bit float formats used here are from a proposal supported by Graphcore, AMD and Qualcomm.
See https://arxiv.org/abs/2206.02915

"""

import struct
import zlib
import array
import bitarray
from bitstring.luts import binary8_luts_compressed
import math


class Binary8Format:
    """8-bit floating point formats based on draft IEEE binary8"""

    def __init__(self, exp_bits: int, bias: int):
        self.exp_bits = exp_bits
        self.bias = bias
        self.pos_clamp_value = 0b01111111
        self.neg_clamp_value = 0b11111111

    def __str__(self):
        return f"Binary8Format(exp_bits={self.exp_bits}, bias={self.bias})"

    def decompress_luts(self):
        binary8_to_float_compressed, float16_to_binary8_compressed = binary8_luts_compressed[(self.exp_bits, self.bias)]
        self.lut_float16_to_binary8 = zlib.decompress(float16_to_binary8_compressed)
        dec = zlib.decompress(binary8_to_float_compressed)
        self.lut_binary8_to_float = struct.unpack(f'<{len(dec) // 4}f', dec)

    def create_luts(self):
        self.lut_binary8_to_float = self.createLUT_for_binary8_to_float()
        self.lut_float16_to_binary8 = self.createLUT_for_float16_to_binary8()

    def float_to_int8(self, f: float) -> int:
        """Given a Python float convert to the best float8 (expressed as an integer in 0-255 range)."""
        # First convert the float to a float16, then a 16 bit uint
        try:
            b = struct.pack('>e', f)
        except (OverflowError, struct.error):
            # Return the largest representable positive or negative value
            return self.pos_clamp_value if f > 0 else self.neg_clamp_value
        f16_int = int.from_bytes(b, byteorder='big')
        # Then use this as an index to our large LUT
        return self.lut_float16_to_binary8[f16_int]

    def createLUT_for_float16_to_binary8(self) -> bytes:
        # Used to create the LUT that was compressed and stored for the fp8 code
        import gfloat
        fi = gfloat.formats.format_info_p3109(8 - self.exp_bits)
        fp16_to_fp8 = bytearray(1 << 16)
        for i in range(1 << 16):
            b = struct.pack('>H', i)
            f, = struct.unpack('>e', b)
            fp = gfloat.round_float(fi, f)
            if math.isnan(fp):
                fp8_i = 0b10000000
            else:
                fp8_i = self.lut_binary8_to_float.index(fp)
            fp16_to_fp8[i] = fp8_i
        return bytes(fp16_to_fp8)

    def createLUT_for_binary8_to_float(self):
        """Create a LUT to convert an int in range 0-255 representing a float8 into a Python float"""
        i2f = []
        for i in range(256):
            b = bitarray.util.int2ba(i, length=8, endian='big', signed=False)
            sign = b[0]
            exponent = bitarray.util.ba2int(b[1:1 + self.exp_bits])
            significand = b[1 + self.exp_bits:]
            if exponent == 0:
                significand = bitarray.bitarray('0') + significand
                exponent = -self.bias + 1
            else:
                significand = bitarray.bitarray('1') + significand
                exponent -= self.bias
            f = float(bitarray.util.ba2int(significand)) / (2.0 ** (7 - self.exp_bits))
            f *= 2 ** exponent
            i2f.append(f if not sign else -f)
        # One special case for minus zero
        i2f[0b10000000] = float('nan')
        # and for plus and minus infinity
        i2f[0b01111111] = float('inf')
        i2f[0b11111111] = float('-inf')
        return array.array('f', i2f)


# We create the 1.5.2 and 1.4.3 formats.
p4binary_fmt = Binary8Format(exp_bits=4, bias=8)
p3binary_fmt = Binary8Format(exp_bits=5, bias=16)


def decompress_luts():
    p4binary_fmt.decompress_luts()
    p3binary_fmt.decompress_luts()
