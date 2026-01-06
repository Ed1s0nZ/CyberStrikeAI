from __future__ import annotations

import logging
from zipfile import ZipFile

from cle.backends.backend import register_backend

from .soot import Soot

log = logging.getLogger(name=__name__)


class Jar(Soot):
    """
    Backend for lifting JARs to Soot.
    """

    is_default = True  # let CLE automatically use this backend

    def __init__(
        self,
        jar_path,
        binary_stream,
        entry_point=None,
        entry_point_params=("java.lang.String[]",),
        jni_libs=None,
        jni_libs_ld_path=None,
        **kwargs,
    ):
        """
        :param jar_path:                Path to JAR.

        The following parameters are optional

        :param entry_point:             Fully qualified name of method that should be used as the entry point.
                                        If not specified, we try to parse it from the manifest.
        :param additional_jars:         Additional JARs.
        :param additional_jar_roots:    Additional JAR roots.
        :param jni_libs:                Name(s) of JNI libs to load (if any).
        :param jni_libs_ld_path:        Path(s) where to find libs defined by param jni_libs.
                                        Note: Directory of the JAR is added by default.
        """

        log.debug("Loading JAR from %s ...", jar_path)

        if not entry_point:
            # try to parse main class from manifest
            self.manifest = self.get_manifest(jar_path)
            main_class = self.manifest.get("Main-Class", None)
            if main_class:
                entry_point = main_class + "." + "main"

        # the actual lifting is done by the Soot superclass
        super().__init__(
            jar_path,
            binary_stream,
            input_format="jar",
            entry_point=entry_point,
            entry_point_params=entry_point_params,
            jni_libs=jni_libs,
            jni_libs_ld_path=jni_libs_ld_path,
            **kwargs,
        )

    @staticmethod
    def is_compatible(stream):
        # check if stream is an archive
        if not Soot.is_zip_archive(stream):
            return False
        # get filelist
        with ZipFile(stream) as jar:
            filelist = jar.namelist()
        # check for manifest and if a least one java class
        # file is available
        if "META-INF/MANIFEST.MF" not in filelist:
            return False
        class_files = [f for f in filelist if f.endswith(".class")]
        if len(class_files) == 0:
            return False
        return True

    def get_manifest(self, binary_path=None):
        """
        Load the MANIFEST.MF file

        :return: A dict of meta info
        :rtype:  dict
        """
        path = binary_path if binary_path else self.binary
        with ZipFile(path) as jar:
            manifest = jar.open("META-INF/MANIFEST.MF", "r")
            data = {}
            for line in manifest.readlines():
                if b":" in line:
                    key, value = line.split(b":")
                    data[key.strip().decode("utf-8")] = value.strip().decode("utf-8")
            return data


register_backend("jar", Jar)
