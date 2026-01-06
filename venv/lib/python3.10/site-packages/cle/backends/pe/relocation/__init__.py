from __future__ import annotations

import logging

from .arm import relocation_table_arm
from .generic import relocation_table_generic
from .mips import relocation_table_mips
from .riscv import relocation_table_riscv

ALL_RELOCATIONS = {
    "AMD64": relocation_table_generic,
    "arm": relocation_table_generic | relocation_table_arm,
    "X86": relocation_table_generic,
    "mips": relocation_table_generic | relocation_table_mips,
    "RISCV": relocation_table_generic | relocation_table_riscv,
}

log = logging.getLogger(name=__name__)
complaint_log = set()


def get_relocation(arch, r_type):
    if r_type == 0:
        return None
    try:
        return ALL_RELOCATIONS[arch][r_type]
    except KeyError:
        if (arch, r_type) not in complaint_log:
            complaint_log.add((arch, r_type))
            log.warning("Unknown reloc %d on %s", r_type, arch)
        return None
