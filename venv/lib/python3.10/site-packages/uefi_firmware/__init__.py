'''UEFI Firmware parser utils.
'''
import os

from . import uefi
from . import pfs
from . import me
from . import flash

from .misc import checker
from .base import FirmwareObject, RawObject, AutoRawObject
from .utils import search_firmware_volumes


class AutoParser(object):
    '''Automatically detect the file format type.

    The UEFI Firmware module supports several types of file formats:
      - UEFI Firmware Volumes
      - (U)EFI Capsule Updates
      - UEFI Firmware Files
      - Dell "PFS" Files
      - Intel ME Modules
      - Flash Descriptors and Regions

    If the input file format is unknown the AutoParser can attempt to discover
    the type by applying basic checks for known headers.
    '''

    def __init__(self, data, search=True):
        '''Create an AutoParser instance.

        Args:
            data (binary): The entire input file contents.
            search (Optional[bool]): Allow brute-force discovery of volumes.
        '''
        self.data_type = 'unknown'
        self.constructor = None
        self.firmware = None
        self.offset = 0

        if search:
            self.offset = 0
            while data[self.offset:self.offset + 1024] == b'\xFF' * 1024:
                self.offset += 1024
            if self.offset > 0:
                data = data[self.offset:]
        self.data = data

        header = data[:100]
        for tester in checker.TESTERS:
            if tester().match(header):
                self.data_type = tester().name
                self.constructor = tester().parser

    def type(self):
        '''Return the discovered file format type.

        If the type could not be matched, 'unknown' is returned.

        Return:
            string: Name of the file format type.
        '''
        return self.data_type

    def parse(self):
        '''Call the 'process' method for the discovered type using the input
        file contents. If the file type's parser returns False indicating a
        failure or exception while parsing this will return None.

        Return:
            object: The associated file object upon success, otherwise None.
        '''
        if self.constructor is None:
            return None
        if self.firmware is not None:
            return self.firmware

        # Instantiate an instance of the firmware object
        self.firmware = self.constructor(self.data)
        if not self.firmware.process():
            # Parsing failed, remove the object reference.
            self.firmware = None
            return None

        objs = [self.firmware]
        size = self.firmware.size

        while size < len(self.data):
            raw = AutoRawObject(self.data[size:])
            if raw.process():
                size += raw.object.size
                objs.append(raw.object)
            else:
                break

        mfc = MultiVolumeContainer(self.data[size:])
        if mfc.has_indexes():
            # Headers were discovered, attempt to process.
            if mfc.process():
                size += mfc.size
                # Add the base (first) firmware volume
                objs = objs + mfc.volumes

        if size < len(self.data):
            objs.append(RawObject(self.data[size:]))

        if self.offset > 0:
            objs = [RawObject('\xFF' * self.offset)] + objs

        if len(objs) == 1:
            return objs[0]
        return MultiObject(objs)


class MultiObject(FirmwareObject):
    def __init__(self, objs):
        self.objs = objs

    @property
    def size(self):
        return sum([obj.size for obj in self.objs])

    @property
    def objects(self):
        return self.objs

    def showinfo(self, ts='', index=None):
        for i in range(len(self.objs)):
            self.objs[i].showinfo(ts, i)

    def to_dict(self):
        volumes = []
        for i in range(len(self.objs)):
            obj = self.objs[i]
            if type(obj) is uefi.FirmwareVolume:
                volumes.append(obj.to_dict())
        return { 'regions': [
            {
                'type': 'bios',
                'data': { 'firmwareVolumes': volumes },
            }
        ] }

    def dump(self, parent='', index=None):
        for i in range(len(self.objs)):
            self.objs[i].dump(parent, i)


class MultiVolumeContainer(FirmwareObject):
    '''The AutoParser attempts to search for 'stacked' UEFIFirmwareVolumes.

    By default if a volume does not consist of the entire input content the
    AutoParser will attempt to search for additional volume headers. This can
    be disabled with the associated 'search' optional parameter.

    If additional volumes are discovered this MultiVolumeContainer is returned
    instead of a UEFIFirmwareVolume.
    '''

    def __init__(self, data):
        '''Initialize the container with the tail content from a volume.'''
        self.data = data
        self.indexes = search_firmware_volumes(data)
        self.volumes = []
        self.size = 0

    def has_indexes(self):
        '''Check if any indexes were discovered.'''
        return len(self.indexes) > 0

    def append_base(self, volume):
        '''Set the base volume as the first within the list.'''
        self.volumes = [volume] + self.volumes

    def process(self):
        for index in self.indexes:
            volume = uefi.FirmwareVolume(self.data[index - 40:], index)
            if volume.process():
                self.size += volume.size
                self.volumes.append(volume)
        valid = len(self.volumes) > 0
        return valid

    @property
    def objects(self):
        return self.volumes

    def showinfo(self, ts='', index=None):
        '''Write structure information to stdout.'''
        for volume in self.volumes:
            volume.showinfo(ts, index)

    def dump(self, parent, index=None):
        '''Allow a caller to dump the content of volumes.'''
        for i, volume in enumerate(self.volumes):
            path = os.path.join(parent, "volume-%d" % i)
            volume.dump(path)


__title__ = "uefi_firmware"
__version__ = "1.11"
__author__ = "Teddy Reed"
__license__ = "BSD"
