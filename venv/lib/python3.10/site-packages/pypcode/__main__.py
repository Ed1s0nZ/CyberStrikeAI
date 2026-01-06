#!/usr/bin/env python
"""
Runs when invoking pypcode module from command line. Lists supported
architectures, and handles basic disassembly and translation to P-code of
supported binaries. Does not parse object files, the binary files must be plain
machine code bytes in a file.
"""

import argparse
import logging
import sys
from difflib import SequenceMatcher

from pypcode import Arch, BadDataError, Context, OpCode, TranslateFlags, UnimplError

log = logging.getLogger(__name__)
logging.basicConfig(level=logging.DEBUG, format="[%(name)s:%(levelname)s] %(message)s")


def main():
    ap = argparse.ArgumentParser(
        prog="pypcode",
        description="Disassemble and translate machine code to P-code using SLEIGH",
    )
    ap.add_argument(
        "-l",
        "--list",
        action="store_true",
        help="list supported architecture languages",
    )
    ap.add_argument("langid", help="architecture language id")
    ap.add_argument("binary", help="path to flat binary code")
    ap.add_argument("base", default="0", nargs="?", help="base address to load at")
    ap.add_argument("-o", "--offset", default="0", help="offset in binary file to load from")
    ap.add_argument("-s", "--length", default=None, help="length of code in bytes to load")
    ap.add_argument(
        "-i",
        "--max-instructions",
        default=0,
        type=int,
        help="maximum number of instructions to translate",
    )
    ap.add_argument(
        "-b",
        "--basic-block",
        action="store_true",
        default=False,
        help="stop translation at end of basic block",
    )

    # List supported languages
    langs = {lang.id: lang for arch in Arch.enumerate() for lang in arch.languages}
    if ("-l" in sys.argv) or ("--list" in sys.argv):
        for langid in sorted(langs):
            print("%-35s - %s" % (langid, langs[langid].description))
        return

    args = ap.parse_args()

    # Get requested language
    if args.langid not in langs:
        print(f'Language "{args.langid}" not found.')
        t = args.langid.upper()
        suggestions = [langid for langid in langs if SequenceMatcher(None, t, langid.split()[0].upper()).ratio() > 0.25]
        if len(suggestions):
            print("\nSuggestions:")
            for langid in sorted(suggestions):
                print("  %-35s - %s" % (langid, langs[langid].description))
            print("")
        print("Try `--list` for full list of architectures.")
        sys.exit(1)

    # Load target binary code
    base = int(args.base, 0)
    with open(args.binary, "rb") as f:
        f.seek(int(args.offset, 0))
        code = f.read(int(args.length, 0)) if args.length else f.read()

    # Translate
    ctx = Context(langs[args.langid])

    try:
        flags = TranslateFlags.BB_TERMINATING if args.basic_block else 0
        res = ctx.translate(code, base, max_instructions=args.max_instructions, flags=flags)

        last_imark_idx = 0
        for i, op in enumerate(res.ops):
            if op.opcode == OpCode.IMARK:
                last_imark_idx = i
                disas_addr = op.inputs[0].offset
                disas_offset = disas_addr - base
                disas_len = sum(vn.size for vn in op.inputs)
                disas_slice = code[disas_offset : disas_offset + disas_len]
                print(ctx.disassemble(disas_slice, disas_addr))
            else:
                print(f" {i - last_imark_idx - 1:3d}: {op}")
        print("")
    except (BadDataError, UnimplError) as e:
        print(f"An error occurred during translation: {e}")
        sys.exit(1)


if __name__ == "__main__":
    main()
