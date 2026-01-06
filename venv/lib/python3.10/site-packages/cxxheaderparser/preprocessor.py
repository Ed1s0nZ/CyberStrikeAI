"""
Contains optional preprocessor support functions
"""

import io
import pathlib
import re
import os
import os.path
import subprocess
import sys
import tempfile
import typing

from .options import PreprocessorFunction


class PreprocessorError(Exception):
    pass


#
# GCC preprocessor support
#


def _gcc_filter(fname: str, fp: typing.TextIO) -> str:
    new_output = io.StringIO()
    keep = True
    fname = fname.replace("\\", "\\\\")

    for line in fp:
        if line.startswith("# "):
            last_quote = line.rfind('"')
            if last_quote != -1:
                keep = line[:last_quote].endswith(fname)

        if keep:
            new_output.write(line)

    new_output.seek(0)
    return new_output.read()


def make_gcc_preprocessor(
    *,
    defines: typing.List[str] = [],
    include_paths: typing.List[str] = [],
    retain_all_content: bool = False,
    encoding: typing.Optional[str] = None,
    gcc_args: typing.List[str] = ["g++"],
    print_cmd: bool = True,
    depfile: typing.Optional[pathlib.Path] = None,
    deptarget: typing.Optional[typing.List[str]] = None,
) -> PreprocessorFunction:
    """
    Creates a preprocessor function that uses g++ to preprocess the input text.

    gcc is a high performance and accurate precompiler, but if an #include
    directive can't be resolved or other oddity exists in your input it will
    throw an error.

    :param defines: list of #define macros specified as "key value"
    :param include_paths: list of directories to search for included files
    :param retain_all_content: If False, only the parsed file content will be retained
    :param encoding: If specified any include files are opened with this encoding
    :param gcc_args: This is the path to G++ and any extra args you might want
    :param print_cmd: Prints the gcc command as its executed
    :param depfile: If specified, will generate a preprocessor depfile that contains
                    a list of include files that were parsed. Must also specify deptarget.
    :param deptarget: List of targets to put in the depfile

    .. code-block:: python

        pp = make_gcc_preprocessor()
        options = ParserOptions(preprocessor=pp)

        parse_file(content, options=options)

    """

    if not encoding:
        encoding = "utf-8"

    def _preprocess_file(filename: str, content: typing.Optional[str]) -> str:
        cmd = gcc_args + ["-w", "-E", "-C"]

        for p in include_paths:
            cmd.append(f"-I{p}")
        for d in defines:
            cmd.append(f"-D{d.replace(' ', '=')}")

        kwargs = {"encoding": encoding}
        if filename == "<str>":
            cmd.append("-")
            filename = "<stdin>"
            if content is None:
                raise PreprocessorError("no content specified for stdin")
            kwargs["input"] = content
        else:
            cmd.append(filename)

        if depfile is not None:
            if deptarget is None:
                raise PreprocessorError(
                    "must specify deptarget if depfile is specified"
                )
            cmd.append("-MD")
            for target in deptarget:
                cmd += ["-MQ", target]
            cmd += ["-MF", str(depfile)]

        if print_cmd:
            print("+", " ".join(cmd), file=sys.stderr)

        result: str = subprocess.check_output(cmd, **kwargs)  # type: ignore
        if not retain_all_content:
            result = _gcc_filter(filename, io.StringIO(result))

        return result

    return _preprocess_file


#
# Microsoft Visual Studio preprocessor support
#


def _msvc_filter(fp: typing.TextIO) -> str:
    # MSVC outputs the original file as the very first #line directive
    # so we just use that
    new_output = io.StringIO()
    keep = True

    first = fp.readline()
    assert first.startswith("#line")
    fname = first[first.find('"') :]

    for line in fp:
        if line.startswith("#line"):
            keep = line.endswith(fname)

        if keep:
            new_output.write(line)

    new_output.seek(0)
    return new_output.read()


def make_msvc_preprocessor(
    *,
    defines: typing.List[str] = [],
    include_paths: typing.List[str] = [],
    retain_all_content: bool = False,
    encoding: typing.Optional[str] = None,
    msvc_args: typing.List[str] = ["cl.exe"],
    print_cmd: bool = True,
) -> PreprocessorFunction:
    """
    Creates a preprocessor function that uses cl.exe from Microsoft Visual Studio
    to preprocess the input text. cl.exe is not typically on the path, so you
    may need to open the correct developer tools shell or pass in the correct path
    to cl.exe in the `msvc_args` parameter.

    cl.exe will throw an error if a file referenced by an #include directive is not found.

    :param defines: list of #define macros specified as "key value"
    :param include_paths: list of directories to search for included files
    :param retain_all_content: If False, only the parsed file content will be retained
    :param encoding: If specified any include files are opened with this encoding
    :param msvc_args: This is the path to cl.exe and any extra args you might want
    :param print_cmd: Prints the command as its executed

    .. code-block:: python

        pp = make_msvc_preprocessor()
        options = ParserOptions(preprocessor=pp)

        parse_file(content, options=options)

    """

    if not encoding:
        encoding = "utf-8"

    def _preprocess_file(filename: str, content: typing.Optional[str]) -> str:
        cmd = msvc_args + ["/nologo", "/E", "/C"]

        for p in include_paths:
            cmd.append(f"/I{p}")
        for d in defines:
            cmd.append(f"/D{d.replace(' ', '=')}")

        tfpname = None

        try:
            kwargs = {"encoding": encoding}
            if filename == "<str>":
                if content is None:
                    raise PreprocessorError("no content specified for stdin")

                tfp = tempfile.NamedTemporaryFile(
                    mode="w", encoding=encoding, suffix=".h", delete=False
                )
                tfpname = tfp.name
                tfp.write(content)
                tfp.close()

                cmd.append(tfpname)
            else:
                cmd.append(filename)

            if print_cmd:
                print("+", " ".join(cmd), file=sys.stderr)

            result: str = subprocess.check_output(cmd, **kwargs)  # type: ignore
            if not retain_all_content:
                result = _msvc_filter(io.StringIO(result))
        finally:
            if tfpname:
                os.unlink(tfpname)

        return result

    return _preprocess_file


#
# PCPP preprocessor support (not installed by default)
#


try:
    import pcpp
    from pcpp import Preprocessor, OutputDirective, Action

    class _CustomPreprocessor(Preprocessor):
        def __init__(
            self,
            encoding: typing.Optional[str],
            passthru_includes: typing.Optional["re.Pattern"],
        ):
            Preprocessor.__init__(self)
            self.errors: typing.List[str] = []
            self.assume_encoding = encoding
            self.passthru_includes = passthru_includes

        def on_error(self, file, line, msg):
            self.errors.append(f"{file}:{line} error: {msg}")

        def on_include_not_found(self, *ignored):
            raise OutputDirective(Action.IgnoreAndPassThrough)

        def on_comment(self, *ignored):
            return True

except ImportError:
    pcpp = None


def _pcpp_filter(
    fname: str, fp: typing.TextIO, deps: typing.Optional[typing.Dict[str, bool]]
) -> str:
    # the output of pcpp includes the contents of all the included files, which
    # isn't what a typical user of cxxheaderparser would want, so we strip out
    # the line directives and any content that isn't in our original file

    line_ending = f'{fname}"\n'

    new_output = io.StringIO()
    keep = True

    for line in fp:
        if line.startswith("#line"):
            keep = line.endswith(line_ending)
            if deps is not None:
                start = line.find('"')
                deps[line[start + 1 : -2]] = True

        if keep:
            new_output.write(line)

    new_output.seek(0)
    return new_output.read()


def make_pcpp_preprocessor(
    *,
    defines: typing.List[str] = [],
    include_paths: typing.List[str] = [],
    retain_all_content: bool = False,
    encoding: typing.Optional[str] = None,
    passthru_includes: typing.Optional["re.Pattern"] = None,
    depfile: typing.Optional[pathlib.Path] = None,
    deptarget: typing.Optional[typing.List[str]] = None,
) -> PreprocessorFunction:
    """
    Creates a preprocessor function that uses pcpp (which must be installed
    separately) to preprocess the input text.

    If missing #include files are encountered, this preprocessor will ignore the
    error. This preprocessor is pure python so it's very portable, and is a good
    choice if performance isn't critical.

    :param defines: list of #define macros specified as "key value"
    :param include_paths: list of directories to search for included files
    :param retain_all_content: If False, only the parsed file content will be retained
    :param encoding: If specified any include files are opened with this encoding
    :param passthru_includes: If specified any #include directives that match the
                              compiled regex pattern will be part of the output.
    :param depfile: If specified, will generate a preprocessor depfile that contains
                    a list of include files that were parsed. Must also specify deptarget.
                    Not compatible with retain_all_content
    :param deptarget: List of targets to put in the depfile

    .. code-block:: python

        pp = make_pcpp_preprocessor()
        options = ParserOptions(preprocessor=pp)

        parse_file(content, options=options)

    """

    if pcpp is None:
        raise PreprocessorError("pcpp is not installed")

    def _preprocess_file(filename: str, content: typing.Optional[str]) -> str:
        pp = _CustomPreprocessor(encoding, passthru_includes)
        if include_paths:
            for p in include_paths:
                pp.add_path(p)

        for define in defines:
            pp.define(define)

        if not retain_all_content:
            pp.line_directive = "#line"
        elif depfile:
            raise PreprocessorError("retain_all_content and depfile not compatible")

        if content is None:
            with open(filename, "r", encoding=encoding) as cfp:
                content = cfp.read()

        pp.parse(content, filename)

        if pp.errors:
            raise PreprocessorError("\n".join(pp.errors))
        elif pp.return_code:
            raise PreprocessorError("failed with exit code %d" % pp.return_code)

        fp = io.StringIO()
        pp.write(fp)
        fp.seek(0)
        if retain_all_content:
            return fp.read()
        else:
            deps: typing.Optional[typing.Dict[str, bool]] = None
            target = None
            if depfile:
                deps = {}
                if not deptarget:
                    base, _ = os.path.splitext(filename)
                    target = f"{base}.o"
                else:
                    target = " ".join(deptarget)

            # pcpp emits the #line directive using the filename you pass in
            # but will rewrite it if it's on the include path it uses. This
            # is copied from pcpp:
            abssource = os.path.abspath(filename)
            for rewrite in pp.rewrite_paths:
                temp = re.sub(rewrite[0], rewrite[1], abssource)
                if temp != abssource:
                    filename = temp
                    if os.sep != "/":
                        filename = filename.replace(os.sep, "/")
                    break

            filtered = _pcpp_filter(filename, fp, deps)

            if depfile is not None:
                assert deps is not None
                with open(depfile, "w") as dfp:
                    dfp.write(f"{target}:")
                    for dep in reversed(list(deps.keys())):
                        dep = dep.replace("\\", "\\\\")
                        dep = dep.replace(" ", "\\ ")
                        dfp.write(f" \\\n  {dep}")
                    dfp.write("\n")

            return filtered

    return _preprocess_file
