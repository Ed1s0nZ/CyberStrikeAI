from __future__ import annotations

import logging
import os
import tempfile
from zipfile import ZipFile

from cle.backends.backend import register_backend

from .android_lifecycle import callback
from .soot import Soot

try:
    from pyaxmlparser import APK as APKParser

    PYAXMLPARSER_INSTALLED = True
except ImportError:
    PYAXMLPARSER_INSTALLED = False

try:
    from pysoot.sootir.soot_class import SootClass
    from pysoot.sootir.soot_method import SootMethod
except ImportError:
    SootMethod = None
    SootClass = None

log = logging.getLogger(name=__name__)

# Default list of JNI archs (in descending order of preference)
# => specifies which arch should be used for loading native libs from the APK
default_jni_archs = ["x86", "armeabi", "armeabi-v7a", "x86_64", "arm64-v8a"]


class Apk(Soot):
    """
    Backend for lifting Apk's to Soot.
    """

    is_default = True  # let CLE automatically use this backend

    def __init__(
        self,
        apk_path,
        binary_stream,
        entry_point=None,
        entry_point_params=(),
        android_sdk=None,
        supported_jni_archs=None,
        jni_libs=None,
        jni_libs_ld_path=None,
        **options,
    ):
        """
        :param apk_path:                Path to APK.
        :param android_sdk:             Path to Android SDK folder (e.g. "/home/angr/android/platforms")

        The following parameters are optional

        :param entry_point:             Fully qualified name of method that should be used as the entry point.
        :param supported_jni_archs:     List of supported JNI architectures (ABIs) in descending order of preference.
        :param jni_libs:                Name(s) of JNI libs to load (if any). If not specified, we try to extract
                                        JNI libs from the APK.
        :param jni_libs_ld_path:        Path(s) where to find libs defined by param jni_libs.
                                        Note: Directory of the APK is added by default.
        """

        log.info("Loading APK from %s ...", apk_path)

        if not android_sdk:
            raise ValueError(
                "\nPath to Android SDK must be specified explicitly, e.g.\n"
                '    loading_opts = { "android_sdk" : "/home/angr/android/platforms" }\n'
                '    proj = angr.Project("/path/to/apk/target.apk", main_opts=loading_opts)'
            )

        if not supported_jni_archs:
            supported_jni_archs = default_jni_archs

        # if jni libs are not defined by the user, we try to extract them from the APK
        if not jni_libs:
            log.info("No JNI libs provided. Trying to parse them from the APK.")
            jni_libs, jni_libs_ld_path = self._extract_jni_libs(apk_path, supported_jni_archs)
        else:
            log.info("Using user defined JNI lib(s) %s (load path(s) %s)", jni_libs, jni_libs_ld_path)

        apk_parser = APKParser(apk_path) if PYAXMLPARSER_INSTALLED else None

        if not entry_point:
            if apk_parser:
                main_activity = apk_parser.get_main_activity()
                entry_point = main_activity + "." + "onCreate"
                entry_point_params = ("android.os.Bundle",)
            else:
                log.error("Install pyaxmlparser to identify APK entry point.")
                raise ImportError

        # the actual lifting is done by the Soot superclass
        super().__init__(
            apk_path,
            binary_stream,
            input_format="apk",
            android_sdk=android_sdk,
            entry_point=entry_point,
            entry_point_params=entry_point_params,
            jni_libs=jni_libs,
            jni_libs_ld_path=jni_libs_ld_path,
            **options,
        )

        # the lifecycle needs to support of pyaxmlparser
        if apk_parser:
            self.components = {"activity": [], "service": [], "receiver": [], "provider": []}
            self.callbacks = {"activity": [], "service": [], "receiver": [], "provider": []}
            self._set_lifecycle(apk_parser)
        else:
            self.components = None
            self.callbacks = None
            log.warning("Install pyaxmlparser, if you want to identify components with callbacks.")

    def _set_lifecycle(self, apk_parser):
        """
        Set components with callbacks of APK lifecycle.

        :param pyaxmlparser apk_parser: XML Parser of the APK.
        """

        component_getter = {
            "activity": apk_parser.get_activities,
            "service": apk_parser.get_services,
            "receiver": apk_parser.get_receivers,
            "provider": apk_parser.get_providers,
        }

        for key, getter in component_getter.items():
            class_names = getter()
            self.components[key], self.callbacks[key] = self._extract_lifecycle(class_names, key)

    def _extract_lifecycle(self, cls_name: list[str], component_kind: str) -> tuple[list[SootClass], list[SootMethod]]:
        """
        Extract components with callbacks from class names and component kind.
        Use general callback name for each component by component kind

        :param cls_name:        Name of the class.
        :param component_kind:  Kind of the component. (activity, service, receiver, provider)
        :return components:     The list of class objects which are components.
        :return callbacks:      The list of method objects which are callbacks.
        """

        components = []
        callbacks = []

        for cls in cls_name:
            components.append(self.classes[cls])
            callbacks.extend(self.get_callbacks(cls, callback[component_kind]))

        return components, callbacks

    def get_callbacks(self, class_name: str, callback_names: list[str]) -> list[SootMethod]:
        """
        Get callback methods from the name of callback methods.

        :param class_name:      Name of the class.
        :param callback_names:  Name list of the callbacks.
        :return:                The method object which is callback.
        :rtype:                 list[pysoot.sootir.soot_method.SootMethod]
        """

        callback_methods = []

        for callback_name in callback_names:
            split_str = callback_name.split("(")
            method_name = split_str[0]
            param_str = split_str[1].rstrip(")")

            if param_str == "":
                params = ()
            else:
                params = tuple(param.strip() for param in param_str.split(","))

            soot_method = self.get_soot_method(method_name, class_name=class_name, params=params, none_if_missing=True)
            if soot_method is not None:
                callback_methods.append(soot_method)

        return callback_methods

    @staticmethod
    def _extract_jni_libs(apk_path, supported_jni_archs):
        """
        Extract JNI libs from APK.

        If an APK uses native libraries via JNI, the APK usually include the libs compiled for
        various architectures. This method first matches the available archs with the list of
        supported archs and then extracts the JNI libs using one of the matched archs.

        :return: Name of all extracted JNI libs together with the path to the directory used for
                 extracting.
        :rtype: tuple
        """
        with ZipFile(apk_path) as apk:
            # Step 1: get filelist from APK
            # => structure follows this schema:
            #    AndroidManifest.xml
            #    META-INF/MANIFEST.MF
            #    classes.dex
            #    lib/armeabi-v7a/libnative-lib.so
            #    lib/x86/libnative-lib.so
            filelist = apk.namelist()

            # Step 2: parse name of available libs and archs
            #         from lib paths "/lib/<jni_arch>/lib<name>.so"
            lib_filelist = [list(filter(None, f.split("/"))) for f in filelist if f.startswith("lib")]
            jni_libs = {lib_path[2] for lib_path in lib_filelist if len(lib_path) > 2}
            available_jni_archs = {lib_path[1] for lib_path in lib_filelist if len(lib_path) > 2}

            if not jni_libs:
                log.info("No JNI libs found.")
                return None, None
            log.info("Found JNI lib(s): %s", ", ".join(jni_libs))

            # Step 3: get the first supported jni arch that is available in the APK
            jni_archs = [arch for arch in supported_jni_archs if arch in available_jni_archs]
            if not jni_archs:
                raise ValueError(
                    f"Couldn't find a supported JNI arch. Available {available_jni_archs}. "
                    "Supported {supported_jni_archs}."
                )
            jni_arch = jni_archs[0]
            log.info("Libs are available with arch(s): %s. Picking %s.", ", ".join(available_jni_archs), jni_arch)

            # Step 4: extract all used libaries from the APK
            # TODO: implement this w/o the need of actually writing files to disk
            #       see https://github.com/angr/cle/issues/123
            tmp_dir = tempfile.mkdtemp()
            for lib in jni_libs:
                apk_file = f"lib/{jni_arch}/{lib}"
                apk.extract(apk_file, path=tmp_dir)
            jni_libs_ld_path = os.path.join(tmp_dir, "lib", jni_arch)

            log.info("Extracted lib(s) to %s", jni_libs_ld_path)
            return jni_libs, jni_libs_ld_path

    @staticmethod
    def is_compatible(stream):
        # check if stream is an archive
        if not Soot.is_zip_archive(stream):
            return False
        # get filelist
        with ZipFile(stream) as apk:
            filelist = apk.namelist()
        # check for manifest and the .dex bytecode file
        if "AndroidManifest.xml" not in filelist:
            return False
        if "classes.dex" not in filelist:
            return False
        return True


register_backend("apk", Apk)
