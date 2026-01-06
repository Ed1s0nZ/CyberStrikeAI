'''Base provides basic firmware object structures.
'''


import os
import ctypes

from .utils import dump_data, sguid, blue


class BaseObject(object):
    '''A base object can be used to access direct content.'''


class FirmwareObject(object):
    '''A pseudo-abstract type providing common firmware member facilities.'''
    def __init__(self):
        self.data = None
        self._name = None
        self.attrs = None
        self.guid = None

    @property
    def name(self):
        return self._name

    @name.setter
    def name(self, name):
        if isinstance(name, bytes):
            try:
                name = name.decode("utf-8")
            except UnicodeDecodeError:
                name = "0x[{}]".format(name.hex())
        self._name = name

    @property
    def content(self):
        '''The object content is the 'data' stream.'''
        if hasattr(self, "data") and self.data is not None:
            return self.data
        return ""

    @property
    def objects(self):
        '''Objects are the child firmware objects found via 'processing'.'''
        return []

    @property
    def label(self):
        '''An overload for an object 'name'.'''
        if hasattr(self, "name") and self.name is not None:
            if self.name is None:
                return ""
            return self.name
        return ""

    @property
    def guid_label(self):
        '''A string representation of an optional 'guid' field.'''
        if not hasattr(self, "guid") or self.guid is None:
            return ""
        return sguid(self.guid)

    @property
    def type_label(self):
        '''The string representation of the object's class name.'''
        return self.__class__.__name__

    @property
    def attrs_label(self):
        '''An overload for the 'attrs' field.'''
        if hasattr(self, "attrs") and self.attrs is not None:
            return self.attrs
        return {}

    def info(self, include_content=False):
        '''Firmwae objects define a common interface for information.

        This defines: label, guid, type, content, attrs-- as common between
        most firmware objects.

        Args:
            include_content (Optional[bool]): Include a pointer to the 'data'
            or content stream.

        Return:
            dict: Return a pointer to this object "_self" and the defines listed
                above with an optional pointer to the data stream.
        '''
        return {
            "_self": self,
            "label": self.label,
            "guid": self.guid_label,
            "type": self.type_label,
            "content": self.content if include_content else "",
            "attrs": self.attrs_label
        }

    def iterate_objects(self, include_content=False):
        '''Flatten this object's children into a list.

        This mis-named as an interation. Each object within the children list
        is recursively 'iterated', meaning its 'iterate_objects' method is
        called. The object is represented via the 'info' method. Access to the
        object is possible via the "_self" key.

        The output list does not include this object but each entry sets a
        "_parent" key with a pointer to this object.

        Return:
            list: flattened list of firmware objects.
        '''
        objects = []
        for _object in self.objects:
            if _object is None:
                continue
            _info = _object.info(include_content)
            _info["objects"] = _object.iterate_objects(include_content)
            for _object in _info["objects"]:
                _object["parent"] = _info
            objects.append(_info)
        return objects


class StructuredObject(object):
    def __init__(self):
        self.fields = []

    def parse_structure(self, data, structure):
        '''Construct an instance object of the provided structure.'''
        struct_instance = structure()
        struct_size = ctypes.sizeof(struct_instance)

        struct_data = data[:struct_size]
        struct_length = min(len(struct_data), struct_size)
        ctypes.memmove(
            ctypes.addressof(struct_instance), struct_data, struct_length)
        self.structure = struct_instance
        self.structure_data = struct_data
        self.structure_fields = [field[0] for field in structure._fields_]
        self.structure_size = struct_size

    def show_structure(self):
        for field in self.fields:
            print("%s: %s" % (field, getattr(self.structure, field, None)))


class RawObject(FirmwareObject, BaseObject):

    def __init__(self, data):
        self.data = data
        self.size = len(data)

    def build(self, generate_checksum, debug=False):
        return self.data

    def showinfo(self, ts='', index=None):
        print("%s%s size= %d " % (
            ts, blue("RawObject:"), len(self.data)
        ))

    def to_dict(self):
        return {
            'size': len(self.data),
        }

    def dump(self, parent='', index=0):
        path = os.path.join(parent, "object-%s.raw" % (str(index)))
        dump_data(path, self.data)


class AutoRawObject(RawObject):
    '''A RawObject that applies AutoParser logic for embedded object discovery.
    '''

    def __init__(self, data):
        self.object = None
        self.data = data

    @property
    def objects(self):
        if self.object is not None:
            return [self.object]
        return []

    def process(self):
        from . import AutoParser
        parser = AutoParser(self.data)
        self.object = parser.parse()
        return self.object is not None

    def showinfo(self, ts='', index=None):
        if self.object is None:
            print("%s%s size= %d " % (
                ts, blue("RawObject:"), len(self.data)
            ))
            return
        self.object.showinfo(ts)

    def to_dict(self):
        return {
            'size': len(self.data),
        }

    def dump(self, parent='', index=None):
        if self.object is None:
            path = os.path.join(parent, "object.raw")
            dump_data(path, self.data)
            return
        self.object.dump(parent)
