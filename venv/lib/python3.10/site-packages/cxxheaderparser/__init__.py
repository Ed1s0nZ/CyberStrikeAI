try:
    from .version import __version__  # type: ignore
except ImportError:
    __version__ = "master"
