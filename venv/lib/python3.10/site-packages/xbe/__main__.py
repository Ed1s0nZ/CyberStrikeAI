#!/usr/bin/env python
# Copyright (c) 2020-2023 Matt Borgerson
#
# Permission is hereby granted, free of charge, to any person obtaining a copy
# of this software and associated documentation files (the "Software"), to deal
# in the Software without restriction, including without limitation the rights
# to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
# copies of the Software, and to permit persons to whom the Software is
# furnished to do so, subject to the following conditions:
#
# The above copyright notice and this permission notice shall be included in all
# copies or substantial portions of the Software.
#
# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
# FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
# AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
# LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
# OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
# SOFTWARE.
import logging
import sys
import argparse
import os.path

from xbe import decode_logo, decode_xpr_image, encode_bmp, Xbe


logging.basicConfig(format="%(message)s", level=logging.DEBUG, stream=sys.stdout)


def extract_images(xbe_path: str, xbe: Xbe) -> None:
    """
    Extract title image and default title save image as BMP files
    """
    out_dir = os.path.dirname(xbe_path)
    xbe_filename = os.path.basename(xbe_path)
    xbe_name = os.path.splitext(xbe_filename)[0]

    for section_name, file_name in [
        ("$$XTIMAGE", "title_image"),
        ("$$XSIMAGE", "save_image"),
    ]:
        if section_name not in xbe.sections:
            print("XBE does not contain '%s' section" % section_name)
            continue

        out_path = os.path.join(out_dir, xbe_name + "_" + file_name + ".bmp")
        print(
            "Extracting XBE image in section '{}' to '{}'".format(
                section_name, out_path
            )
        )

        bmp = encode_bmp(*decode_xpr_image(xbe.sections[section_name].data))
        with open(out_path, "wb") as f:
            f.write(bmp)

    bmp = encode_bmp(*decode_logo(xbe.logo))
    with open(os.path.join(out_dir, xbe_name + "_logo_image.bmp"), "wb") as f:
        f.write(bmp)


def xbx_to_bmp(xbx_path: str) -> None:
    """
    Convert a .xbx image file to a standard BMP file
    """
    out_dir = os.path.dirname(xbx_path)
    xbx_filename = os.path.basename(xbx_path)
    xbx_name = os.path.splitext(xbx_filename)[0]
    out_path = os.path.join(out_dir, xbx_name + ".bmp")

    print(f"Converting XBX file '{xbx_path}' to '{out_path}'")

    with open(xbx_path, "rb") as f:
        data = f.read()

    with open(out_path, "wb") as f:
        f.write(encode_bmp(*decode_xpr_image(data)))


def main() -> None:
    ap = argparse.ArgumentParser(
        "xbe", description="Tool to work with original Xbox executable and data files"
    )

    # XBE file options
    ap.add_argument("xbefile", nargs="?", help=".xbe file")
    ap.add_argument(
        "--extract-images",
        action="store_true",
        help="extract title and save images as bmp files",
    )

    # Auxillary file processing
    ap.add_argument(
        "--xbx-to-bmp",
        nargs="+",
        metavar="xbxfile",
        help="convert an xbx image to a bmp",
    )
    args = ap.parse_args()

    if args.xbefile:
        xbe = Xbe.from_file(args.xbefile)
        if args.extract_images:
            extract_images(args.xbefile, xbe)

    if args.xbx_to_bmp:
        for path in args.xbx_to_bmp:
            xbx_to_bmp(path)


if __name__ == "__main__":
    main()
