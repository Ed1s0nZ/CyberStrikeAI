from .gym import AARCH64Spotter, AMD64Spotter, ARMSpotter, X86Spotter
from .libvex import LIBVEX_SUPPORTED_ARCHES, LibVEXLifter
from .lift_function import lift, lifters, register
from .lifter import Lifter
from .post_processor import Postprocessor
from .zerodivision import ZeroDivisionPostProcessor

for arch in LIBVEX_SUPPORTED_ARCHES:
    register(LibVEXLifter, arch)
register(AARCH64Spotter, "AARCH64")
register(ARMSpotter, "ARM")
register(ARMSpotter, "ARMEL")
register(ARMSpotter, "ARMHF")
register(ARMSpotter, "ARMCortexM")
register(AMD64Spotter, "AMD64")
register(X86Spotter, "X86")

__all__ = ["Lifter", "Postprocessor", "lift", "register", "lifters", "ZeroDivisionPostProcessor"]
