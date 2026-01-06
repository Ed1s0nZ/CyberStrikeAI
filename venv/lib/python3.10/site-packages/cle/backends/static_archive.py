from __future__ import annotations

import logging

import arpy

from .backend import Backend, register_backend

log = logging.getLogger(__name__)


class StaticArchive(Backend):
    @classmethod
    def is_compatible(cls, stream):
        stream.seek(0)
        return stream.read(8) == b"!<arch>\n"

    is_default = True
    is_outer = True

    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)

        # hack: we are using a loader internal method in a non-kosher way which will cause our children to be
        # marked as the main binary if we are also the main binary
        # work around this by setting ourself here:
        if self.loader._main_object is None:
            self.loader._main_object = self

        ar = arpy.Archive(fileobj=self._binary_stream)
        ar.read_all_headers()
        for name, stream in ar.archived_files.items():
            child = self.loader._load_object_isolated(stream)
            child.binary = child.binary_basename = name.decode()
            child.parent_object = self
            self.child_objects.append(child)

        if self.child_objects:
            self._arch = self.child_objects[0].arch
        else:
            log.warning("Loaded empty static archive?")
        self.has_memory = False
        self.pic = True

        # hack pt. 2
        if self.loader._main_object is self:
            self.loader._main_object = None


register_backend("AR", StaticArchive)
