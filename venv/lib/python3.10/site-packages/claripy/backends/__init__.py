from __future__ import annotations

from .backend import Backend
from .backend_any import BackendAny
from .backend_concrete import BackendConcrete
from .backend_vsa import BackendVSA
from .backend_z3 import BackendZ3

concrete = BackendConcrete()
z3 = BackendZ3()
vsa = BackendVSA()
any_backend = BackendAny()

all_backends = [concrete, z3, vsa]
backends_by_type = {b.__class__.__name__: b for b in all_backends}

__all__ = (
    "Backend",
    "BackendConcrete",
    "BackendVSA",
    "BackendZ3",
    "all_backends",
    "any_backend",
    "backends_by_type",
    "concrete",
    "vsa",
    "z3",
)
