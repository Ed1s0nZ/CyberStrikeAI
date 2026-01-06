from __future__ import annotations

import io
import logging
import mmap
from dataclasses import dataclass
from functools import singledispatchmethod
from uuid import UUID

import archinfo
import uefi_firmware

from cle.errors import CLEUnknownFormatError

from . import Backend, register_backend
from .pe import PE
from .te import TE

log = logging.getLogger(__name__)


class UefiDriverLoadError(Exception):
    """
    This error is raised (and caught internally) if the data contained in the UEFI entity tree doesn't make sense.
    """


class UefiFirmware(Backend):
    """
    A UEFI firmware blob loader. Support is provided by the ``uefi_firmware`` package.
    """

    is_default = True

    @classmethod
    def _to_bytes(cls, fileobj: io.IOBase):
        try:
            fileno = fileobj.fileno()
        except io.UnsupportedOperation:
            pass
        else:
            return mmap.mmap(fileno, 0, access=mmap.ACCESS_READ)

        if isinstance(fileobj, io.BytesIO):
            return fileobj.getbuffer()

        # fuck it, we'll do it live
        fileobj.seek(0)
        return fileobj.read()

    @classmethod
    def is_compatible(cls, stream):
        buffer = cls._to_bytes(stream)
        parser = uefi_firmware.AutoParser(buffer)
        return parser.type() != "unknown"

    def __init__(self, *args, **kwargs) -> None:
        super().__init__(*args, **kwargs)

        # hack: we are using a loader internal method in a non-kosher way which will cause our children to be
        # marked as the main binary if we are also the main binary
        # work around this by setting ourself here:
        if self.loader._main_object is None:
            self.loader._main_object = self

        self._drivers: dict[UUID, UefiModuleMixin] = {}
        self._drivers_pending: dict[UUID, UefiModulePending] = {}
        self._current_file: UUID | None = None

        self.set_arch(archinfo.arch_from_id("x86_64"))  # TODO: ???

        buffer = self._to_bytes(self._binary_stream)
        parser = uefi_firmware.AutoParser(buffer)
        firmware = parser.parse()
        self._load(firmware)

        while self._drivers_pending:
            uuid, pending = self._drivers_pending.popitem()
            try:
                child = pending.build(self, uuid)
            except UefiDriverLoadError as e:
                log.warning("Failed to load %s: %s", uuid, e.args[0])
            else:
                self._drivers[uuid] = child
                child.parent_object = self
                self.child_objects.append(child)

        if self.child_objects:
            self._arch = self.child_objects[0].arch
        else:
            log.warning("Loaded empty UEFI firmware?")
        self.has_memory = False
        self.pic = True

        # hack pt. 2
        if self.loader._main_object is self:
            self.loader._main_object = None

    @singledispatchmethod
    def _load(self, uefi_obj):  # pylint: disable=no-self-use
        raise CLEUnknownFormatError(f"Can't load firmware object: {uefi_obj}")

    @_load.register
    def _load_generic(self, uefi_obj: uefi_firmware.FirmwareObject):
        for obj in uefi_obj.objects:
            self._load(obj)

    @_load.register
    def _load_none(self, uefi_obj: None):
        pass

    @_load.register
    def _load_firmwarefile(self, uefi_obj: uefi_firmware.uefi.FirmwareFile):
        old_uuid = self._current_file
        if uefi_obj.type == 7:  # driver
            uuid = UUID(bytes=uefi_obj.guid)
            self._drivers_pending[uuid] = UefiModulePending()
            self._current_file = uuid
        self._load_generic(uefi_obj)
        self._current_file = old_uuid

    @_load.register
    def _load_firmwarefilesection(self, uefi_obj: uefi_firmware.uefi.FirmwareFileSystemSection):
        pending = self._drivers_pending.get(self._current_file, None)
        if pending is not None:
            if uefi_obj.type == 16:  # pe32 image
                pending.pe_image = uefi_obj.content
            elif uefi_obj.type == 18:  # te image
                pending.te_image = uefi_obj.content
            elif uefi_obj.type == 21:  # user interface name
                pending.name = uefi_obj.content.decode("utf-16").strip("\0")
        self._load_generic(uefi_obj)


@dataclass
class UefiModulePending:
    """
    A worklist entry for the UEFI firmware loader.
    """

    name: str | None = None
    pe_image: bytes | None = None
    te_image: bytes | None = None
    # version
    # dependencies

    def build(self, parent: UefiFirmware, guid: UUID) -> UefiModuleMixin:
        count = (self.pe_image is not None) + (self.te_image is not None)
        if count > 1:
            raise UefiDriverLoadError("Multiple image sections")
        cls: type[UefiModuleMixin]
        if self.pe_image is not None:
            cls = UefiPE
            data = self.pe_image
        elif self.te_image is not None:
            cls = UefiTE
            data = self.te_image
        else:
            raise UefiDriverLoadError("Missing PE or TE image section")
        return cls(None, io.BytesIO(data), is_main_bin=False, loader=parent.loader, name=self.name, guid=guid)


class UefiModuleMixin(Backend):
    """
    A mixin to make other kinds of backends load as UEFI modules.
    """

    def __init__(self, *args, guid: UUID, name: str | None, **kwargs):
        super().__init__(*args, **kwargs)
        self.guid = guid
        self.user_interface_name = name

        if self.linked_base == 0:
            self.pic = True

    def __repr__(self):
        return (
            f"<{type(self).__name__} Object "
            f'{self.guid}{f" {self.user_interface_name}" if self.user_interface_name else ""}, '
            f"maps [{self.min_addr:#x}:{self.max_addr:#x}]>"
        )


class UefiPE(UefiModuleMixin, PE):
    """
    A PE file contained in a UEFI image.
    """


class UefiTE(UefiModuleMixin, TE):
    """
    A TE file contained in a UEFI image.
    """


register_backend("uefi", UefiFirmware)
