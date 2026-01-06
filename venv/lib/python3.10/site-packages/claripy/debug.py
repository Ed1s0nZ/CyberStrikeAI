# to enable debug mode, please set _DEBUG to True via the set_debug method.
from __future__ import annotations

_DEBUG = True


def set_debug(enabled):
    """
    Enable or disable the debug mode. In debug mode, a bunch of extra checks in claripy will be executed. You'll want to
    disable debug mode if you are running performance critical code.
    """

    global _DEBUG
    _DEBUG = enabled
