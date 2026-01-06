from ._core import (
    NODEFAULT,
    UNSET,
    DecodeError,
    EncodeError,
    Field as _Field,
    Meta,
    MsgspecError,
    Raw,
    Struct,
    StructMeta,
    UnsetType,
    ValidationError,
    convert,
    defstruct,
    to_builtins,
)


def field(*, default=NODEFAULT, default_factory=NODEFAULT, name=None):
    return _Field(default=default, default_factory=default_factory, name=name)


field.__doc__ = _Field.__doc__


from . import inspect, json, msgpack, structs, toml, yaml
from ._version import __version__
