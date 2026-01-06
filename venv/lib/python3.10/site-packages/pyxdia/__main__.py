import argparse

from .pdb import PDB


def main():
    ap = argparse.ArgumentParser("pyxdia")
    ap.add_argument("pdb", help="Path to PDB file")
    args = ap.parse_args()

    pdb = PDB(args.pdb)
    pdb.pp()


if __name__ == "__main__":
    main()
