from __future__ import annotations

import warnings


def deprecated(new: str, old: str | None = None):
    """
    Warn a user once that a function has been deprecated
    If old is not specified, func.__name__ is used as the old name
    """

    def outer(func):
        old_name = old.__name__ if old is None else old

        def inner(*args, **kwargs):
            warnings.warn(f"Use {new} instead of {old_name}", DeprecationWarning, stacklevel=2)
            return func(*args, **kwargs)

        return inner

    return outer
