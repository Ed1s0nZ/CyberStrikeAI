import sys
from bitstring.bits import Bits
from bitstring.dtypes import Register

dtype_register = Register()


def main() -> None:
    # check if final parameter is an interpretation string
    fp = sys.argv[-1]
    if fp in ['-h', '--help'] or len(sys.argv) == 1:
        print("""Create and interpret a bitstring from command-line parameters.

Command-line parameters are concatenated and a bitstring created
from them. If the final parameter is either an interpretation string
or ends with a '.' followed by an interpretation string then that
interpretation of the bitstring will be used when printing it.

Typical usage might be invoking the Python module from a console
as a one-off calculation:

$ python -m bitstring int:16=-400
0xfe70
$ python -m bitstring float:32=0.2 bin
00111110010011001100110011001101
$ python -m bitstring 0xff 3*0b01,0b11 uint
65367
$ python -m bitstring hex=01, uint:12=352.hex
01160
        """)
        return
    if fp in dtype_register.names:
        # concatenate all other parameters and interpret using the final one
        b1 = Bits(','.join(sys.argv[1: -1]))
        print(b1._readtoken(fp, 0, b1.__len__())[0])
    else:
        # does final parameter end with a dot then an interpretation string?
        interp = fp[fp.rfind('.') + 1:]
        if interp in dtype_register.names:
            sys.argv[-1] = fp[:fp.rfind('.')]
            b1 = Bits(','.join(sys.argv[1:]))
            print(b1._readtoken(interp, 0, b1.__len__())[0])
        else:
            # No interpretation - just use default print
            b1 = Bits(','.join(sys.argv[1:]))
            print(b1)


if __name__ == '__main__':
    main()
