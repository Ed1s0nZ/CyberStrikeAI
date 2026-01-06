from dataclasses import dataclass
from typing import Callable, Optional

#: arguments are (filename, content)
PreprocessorFunction = Callable[[str, Optional[str]], str]


@dataclass
class ParserOptions:
    """
    Options that control parsing behaviors
    """

    #: If true, prints out
    verbose: bool = False

    #: If true, converts a single void parameter to zero parameters
    convert_void_to_zero_params: bool = True

    #: A function that will preprocess the header before parsing. See
    #: :py:mod:`cxxheaderparser.preprocessor` for available preprocessors
    preprocessor: Optional[PreprocessorFunction] = None
