"""
Pythonic interface to SLEIGH
"""

from __future__ import annotations
import os.path
import xml.etree.ElementTree as ET
from enum import IntEnum
from typing import cast, Generator, Sequence, Mapping

from .__version__ import __version__

from .pypcode_native import __version__ as __pypcode_native_version__  # pylint:disable=no-name-in-module

assert __version__ == __pypcode_native_version__

from .pypcode_native import (  # pylint:disable=no-name-in-module
    Address,
    AddrSpace,
    BadDataError,
    DecoderError,
    Disassembly,
    Instruction,
    LowlevelError,
    OpCode,
    PcodeOp,
    TRANSLATE_FLAGS_BB_TERMINATING,
    Translation,
    UnimplError,
    Varnode,
)
from .pypcode_native import Context as _Context  # pylint:disable=no-name-in-module

from .printing import (
    OpFormat,
    OpFormatBinary,
    OpFormatFunc,
    OpFormatSpecial,
    OpFormatUnary,
    PcodePrettyPrinter,
)

__all__ = [
    "Address",
    "AddrSpace",
    "Arch",
    "ArchLanguage",
    "BadDataError",
    "Context",
    "DecoderError",
    "Disassembly",
    "Instruction",
    "LowlevelError",
    "OpCode",
    "OpFormat",
    "OpFormatBinary",
    "OpFormatFunc",
    "OpFormatSpecial",
    "OpFormatUnary",
    "PcodeOp",
    "PcodePrettyPrinter",
    "TranslateFlags",
    "Translation",
    "UnimplError",
    "Varnode",
]


class TranslateFlags(IntEnum):
    """
    Flags that can be passed to Context::translate
    """

    BB_TERMINATING = TRANSLATE_FLAGS_BB_TERMINATING


PKG_SRC_DIR = os.path.abspath(os.path.dirname(__file__))
SPECFILES_DIR = os.path.join(PKG_SRC_DIR, "processors")


class ArchLanguage:
    """
    A specific language for an architecture. Provides access to language, pspec, and cspecs.
    """

    __slots__ = (
        "archdir",
        "ldef",
        "_pspec",
        "_cspecs",
    )

    archdir: str
    ldef: ET.Element

    def __init__(self, archdir: str, ldef: ET.Element):
        self.archdir = archdir
        self.ldef = ldef
        self._pspec: ET.Element | None = None
        self._cspecs: dict[tuple[str, str], ET.Element] | None = None

    @property
    def pspec_path(self) -> str:
        return os.path.join(self.archdir, self.processorspec)

    @property
    def slafile_path(self) -> str:
        return os.path.join(self.archdir, self.slafile)

    @property
    def description(self) -> str:
        elem = self.ldef.find("description")
        if elem is not None:
            return elem.text or ""
        return ""

    def __getattr__(self, key):
        if key in self.ldef.attrib:
            return self.ldef.attrib[key]
        raise AttributeError(key)

    @property
    def pspec(self) -> ET.Element | None:
        if self._pspec is None:
            self._pspec = ET.parse(self.pspec_path).getroot()
        return self._pspec

    @property
    def cspecs(self) -> Mapping[tuple[str, str], ET.Element]:
        if self._cspecs is None:
            self._cspecs = {}
            for e in self.ldef.findall("compiler"):
                path = os.path.join(self.archdir, e.attrib["spec"])
                cspec = ET.parse(path).getroot()
                self._cspecs[(e.attrib["id"], e.attrib["name"])] = cspec
        return self._cspecs

    def init_context_from_pspec(self, ctx: Context) -> None:
        if self.pspec is None:
            return
        cd = self.pspec.find("context_data")
        if cd is None:
            return
        cs = cd.find("context_set")
        if cs is None:
            return
        for e in cs:
            assert e.tag == "set"
            ctx.setVariableDefault(e.attrib["name"], int(e.attrib["val"]))

    @classmethod
    def from_id(cls, langid: str) -> ArchLanguage | None:
        """
        Return language with given id, or None if the language could not be found.
        """
        for arch in Arch.enumerate():
            for lang in arch.languages:
                if lang.id == langid:
                    return lang
        return None


class Arch:
    """
    Main class representing an architecture describing available languages.
    """

    __slots__ = (
        "archpath",
        "archname",
        "ldefpath",
        "ldef",
        "languages",
    )

    archpath: str
    archname: str
    ldefpath: str
    ldef: ET.ElementTree[ET.Element[str]]
    languages: Sequence[ArchLanguage]

    def __init__(self, name: str, ldefpath: str):
        """
        Initialize the Arch.

        :param name: The name of the architecture
        :param ldefpath: Path to language definition files (.ldefs)
        """
        self.archpath = os.path.dirname(ldefpath)
        self.archname = name
        self.ldefpath = ldefpath
        self.ldef = ET.parse(ldefpath)
        self.languages = [ArchLanguage(self.archpath, e) for e in self.ldef.getroot()]

    @classmethod
    def enumerate(cls) -> Generator[Arch]:
        """
        Enumerate all available architectures and languages.

        Language definitions are sourced from definitions shipped with pypcode and
        can be found in processors/<architecture>/data/languages/<variant>.ldefs
        """
        for archname in os.listdir(SPECFILES_DIR):
            langdir = os.path.join(SPECFILES_DIR, archname, "data", "languages")
            if not (os.path.exists(langdir) and os.path.isdir(langdir)):
                continue
            for langname in os.listdir(langdir):
                if not langname.endswith(".ldefs"):
                    continue
                ldefpath = os.path.join(langdir, langname)
                yield Arch(archname, ldefpath)


class Context(_Context):
    """
    Context for translation.
    """

    __slots__ = (
        "language",
        "registers",
    )

    language: ArchLanguage
    registers: dict[str, Varnode]

    def __init__(self, language: ArchLanguage | str):
        """
        Initialize a context for translation or disassembly.

        :param language: The ``ArchLanguage`` to initialize the context with, or a language id ``str``
        """
        if isinstance(language, ArchLanguage):
            self.language = language
        elif isinstance(language, str):
            _l = ArchLanguage.from_id(cast(str, language))
            assert _l is not None
            self.language = _l
        else:
            raise TypeError("Context must be initialized with a language or language id")
        super().__init__(f"<sleigh>{self.language.slafile_path}</sleigh>")
        self.language.init_context_from_pspec(self)
        self.registers = {n: v for v, n in self.getAllRegisters().items()}
