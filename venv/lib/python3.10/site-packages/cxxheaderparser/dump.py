import argparse
import dataclasses
import json
import pathlib
import pprint
import subprocess
import sys

from .options import ParserOptions
from .simple import parse_file


def dumpmain() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("header")
    parser.add_argument(
        "-w",
        "--width",
        default=80,
        type=int,
        help="Width of output when in pprint mode",
    )
    parser.add_argument("-v", "--verbose", default=False, action="store_true")
    parser.add_argument(
        "--mode",
        choices=["json", "pprint", "repr", "brepr", "pponly"],
        default="pprint",
    )
    parser.add_argument(
        "--pcpp", default=False, action="store_true", help="Use pcpp preprocessor"
    )
    parser.add_argument(
        "--gcc", default=False, action="store_true", help="Use GCC as preprocessor"
    )
    parser.add_argument(
        "--depfile",
        default=None,
        type=pathlib.Path,
        help="Generate a depfile (requires preprocessor)",
    )
    parser.add_argument(
        "--deptarget", default=[], action="append", help="depfile target"
    )
    parser.add_argument(
        "--encoding", default=None, help="Use this encoding to open the file"
    )

    args = parser.parse_args()

    pp_kwargs = dict(encoding=args.encoding)

    if args.depfile:
        if not (args.pcpp or args.gcc):
            parser.error("--depfile requires either --pcpp or --gcc")

        pp_kwargs["depfile"] = args.depfile
        pp_kwargs["deptarget"] = args.deptarget

    preprocessor = None
    if args.gcc:
        from .preprocessor import make_gcc_preprocessor

        preprocessor = make_gcc_preprocessor(**pp_kwargs)

    if args.pcpp or (args.mode == "pponly" and preprocessor is None):
        from .preprocessor import make_pcpp_preprocessor

        preprocessor = make_pcpp_preprocessor(**pp_kwargs)

    if args.mode == "pponly":
        assert preprocessor is not None
        with open(args.header, "r", encoding=args.encoding) as fp:
            pp_content = preprocessor(args.header, fp.read())
        sys.stdout.write(pp_content)
        sys.exit(0)

    options = ParserOptions(verbose=args.verbose, preprocessor=preprocessor)
    data = parse_file(args.header, encoding=args.encoding, options=options)

    if args.mode == "pprint":
        ddata = dataclasses.asdict(data)
        pprint.pprint(ddata, width=args.width, compact=True)

    elif args.mode == "json":
        ddata = dataclasses.asdict(data)
        json.dump(ddata, sys.stdout, indent=2)

    elif args.mode == "brepr":
        stmt = repr(data)
        stmt = subprocess.check_output(
            ["black", "-", "-q"], input=stmt.encode("utf-8")
        ).decode("utf-8")

        print(stmt)

    elif args.mode == "repr":
        print(data)

    else:
        parser.error("Invalid mode")
