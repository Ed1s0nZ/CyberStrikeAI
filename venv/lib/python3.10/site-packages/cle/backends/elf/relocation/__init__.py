from __future__ import annotations

import logging

from .amd64 import relocation_table_amd64
from .arm import relocation_table_arm
from .arm64 import relocation_table_arm64
from .i386 import relocation_table_i386
from .mips import relocation_table_mips
from .ppc import relocation_table_ppc
from .ppc64 import relocation_table_ppc64
from .s390x import relocation_table_s390x
from .sparc import relocation_table_sparc

ALL_RELOCATIONS = {
    "AMD64": relocation_table_amd64,
    "ARMCortexM": relocation_table_arm,
    "ARM": relocation_table_arm,
    "AARCH64": relocation_table_arm64,
    "ARMEL": relocation_table_arm,
    "ARMHF": relocation_table_arm,
    "X86": relocation_table_i386,
    "MIPS32": relocation_table_mips,
    "MIPS64": relocation_table_mips,
    "PPC32": relocation_table_ppc,
    "PPC64": relocation_table_ppc64,
    "S390X": relocation_table_s390x,
    "sparc:BE:32:default": relocation_table_sparc,
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
