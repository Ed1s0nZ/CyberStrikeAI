from __future__ import annotations

import logging
import time

from archinfo.arch_soot import ArchSoot, SootAddressDescriptor, SootMethodDescriptor

from cle.backends.backend import Backend
from cle.errors import CLEError

try:
    import pysoot
    from pysoot.lifter import Lifter
except ImportError:
    pysoot = None
    Lifter = None

log = logging.getLogger(name=__name__)


class Soot(Backend):
    """
    The basis backend for lifting and loading bytecode from JARs and APKs to Soot IR.

    Note that self.min_addr will be 0 and self.max_addr will be 1. Hopefully no other object will be mapped at address
    0.
    """

    def __init__(
        self,
        *args,
        entry_point=None,
        entry_point_params=(),
        input_format=None,
        additional_jars=None,
        additional_jar_roots=None,
        jni_libs_ld_path=None,
        jni_libs=None,
        android_sdk=None,
        **kwargs,
    ):
        if not pysoot:
            raise ImportError("Cannot import PySoot. The Soot backend requires PySoot.")

        if kwargs.get("has_memory", False):
            raise CLEError('The parameter "has_memory" must be False for Soot backend.')

        super().__init__(*args, has_memory=False, **kwargs)
        if self.binary is None:
            raise ValueError("Cannot use the Soot backend loading from a stream")

        # load the classes
        log.debug("Lifting to Soot IR ...")
        start_time = time.time()
        pysoot_lifter = Lifter(
            self.binary,
            input_format=input_format,
            android_sdk=android_sdk,
            additional_jars=additional_jars,
            additional_jar_roots=additional_jar_roots,
        )
        end_time = time.time()
        log.debug("Lifting completed in %ds", round(end_time - start_time, 2))
        self._classes = pysoot_lifter.classes

        # find entry method
        if entry_point:
            try:
                ep_method = self.get_soot_method(entry_point, params=entry_point_params)
                ep_method_descriptor = SootMethodDescriptor.from_soot_method(ep_method)
                self._entry = SootAddressDescriptor(ep_method_descriptor, 0, 0)
                log.debug("Entry point set to %s", self._entry)
            except CLEError:
                log.warning("Couldn't find entry point %s.", entry_point)
                self._entry = None

        self.os = "javavm"
        self.rebase_addr = None
        self.set_arch(ArchSoot())

        if jni_libs:
            # native libraries are getting loaded by adding them as a dependency of this object
            self.deps += [jni_libs] if type(jni_libs) in (str, bytes) else jni_libs
            # if available, add additional load path(s)
            if jni_libs_ld_path:
                path_list = [jni_libs_ld_path] if type(jni_libs_ld_path) in (str, bytes) else jni_libs_ld_path
                self.extra_load_path += path_list
            self.jni_support = True
        else:
            self.jni_support = False

    @property
    def max_addr(self):
        # FIXME: This is a hack to satisfy checks elsewhere that max_addr must be greater than min_addr
        return self.min_addr + 1

    @property
    def entry(self):
        return self._entry

    @property
    def classes(self):
        return self._classes

    def get_soot_class(self, cls_name, none_if_missing=False):
        """
        Get Soot class object.

        :param str cls_name: Name of the class.
        :return:             The class object.
        :rtype:              pysoot.soot.SootClass
        """
        try:
            return self._classes[cls_name]
        except KeyError:
            if none_if_missing:
                return None
            else:
                raise CLEError(f'Class "{cls_name}" does not exist.')

    def get_soot_method(self, thing, class_name=None, params=(), none_if_missing=False):
        """
        Get Soot method object.

        :param thing:           Descriptor or the method, or name of the method.
        :param str class_name:  Name of the class. If not specified, class name can be parsed from method_name.
        :return:                Soot method that satisfy the criteria.
        """

        # Step 1: Parse input
        if isinstance(thing, SootMethodDescriptor):
            method_description = {
                "class_name": thing.class_name,
                "name": thing.name,
                "params": thing.params,
            }

        elif isinstance(thing, str | bytes):
            method_name = thing

            # if class_name is not set, parse it from the method name
            if class_name is None:
                last_dot = method_name.rfind(".")
                if last_dot >= 0:
                    class_name = method_name[:last_dot]
                    method_name = method_name[last_dot + 1 :]
                else:
                    raise ValueError(f"Cannot parse class name from method {method_name}.")

            method_description = {
                "class_name": class_name,
                "name": method_name,
                "params": params,
            }

        else:
            raise TypeError(f'Unsupported type "{thing}" for the first argument.')

        # Step 2: Load class containing the method
        try:
            cls = self.get_soot_class(method_description["class_name"])
        except CLEError:
            if none_if_missing:
                return None
            else:
                raise

        # Step 3: Get all methods matching the description
        methods = [
            soot_method
            for soot_method in cls.methods
            if self._description_matches_soot_method(soot_method, **method_description)
        ]

        if not methods:
            if none_if_missing:
                return None
            else:
                raise CLEError(
                    "Method with description {} does not exist in class {}.".format(
                        method_description, method_description["class_name"]
                    )
                )

        if len(methods) > 1:
            # Warn if we found several matching methods
            log.warning(
                "Method with description %s is ambiguous in class %s.",
                method_description,
                method_description["class_name"],
            )

        return methods[0]

    @staticmethod
    def _description_matches_soot_method(soot_method, name=None, class_name=None, params=()):
        if name and soot_method.name != name:
            return False
        if class_name and soot_method.class_name != class_name:
            return False
        if soot_method.params != params:
            return False
        return True

    @property
    def main_methods(self):
        """
        Find all Main methods in this binary.

        :return: All main methods in each class.
        :rtype:  iterator
        """
        for cls in self.classes.values():
            for method in cls.methods:
                if method.name == "main":  # TODO: Check more attributes
                    yield method

    @staticmethod
    def is_zip_archive(stream):
        stream.seek(0)
        identstring = stream.read(4)
        stream.seek(0)
        return identstring.startswith(b"\x50\x4b\x03\x04")
