#!/usr/bin/env python
# coding: utf-8

"""
Converts a Python dictionary or other native data type into a valid XML string.

Supports item (`int`, `float`, `long`, `decimal.Decimal`, `bool`, `str`, `unicode`, `datetime`, `none` and other number-like objects) and collection (`list`, `set`, `tuple` and `dict`, as well as iterable and dict-like objects) data types, with arbitrary nesting for the collections. Items with a `datetime` type are converted to ISO format strings. Items with a `None` type become empty XML elements.

This module works with both Python 2 and 3.
"""

from __future__ import unicode_literals

__version__ = '1.7.16'
version = __version__

from random import randint
try:
    from collections.abc import Iterable as iterable

except ImportError:
    from collections import Iterable as iterable

import numbers
import logging
from xml.dom.minidom import parseString


LOG = logging.getLogger("dicttoxml")

# python 3 doesn't have a unicode type
try:
    unicode
except:
    unicode = str

# python 3 doesn't have a long type
try:
    long
except:
    long = int


def set_debug(debug=False, filename='dicttoxml.log'):
    if debug:
        import datetime
        print('Debug mode is on. Events are logged at: %s' % (filename))
        logging.basicConfig(filename=filename, level=logging.INFO)
        LOG.info('\nLogging session starts: %s' % (
            str(datetime.datetime.today()))
        )
    else:
        logging.basicConfig(level=logging.WARNING)


def unicode_me(val):
    """Converts strings with non-ASCII characters to unicode for LOG.
    Python 3 doesn't have a `unicode()` function, so `unicode()` is an alias
    for `str()`, but `str()` doesn't take a second argument, hence this kludge.
    """
    LOG.info('Inside unicode_me(). val = "%s"' % (val, ))
    try:
        return unicode(val, 'utf-8')
    except:
        return unicode(val)


ids = [] # initialize list of unique ids

def make_id(element, start=100000, end=999999):
    """Returns a random integer"""
    LOG.info('Inside make_id(). element = "%s", start="%s", end="%s"' % (element, start, end))
    return '%s_%s' % (element, randint(start, end))


def get_unique_id(element):
    """Returns a unique id for a given element"""
    LOG.info('Inside get_unique_id(). element = "%s"' % (element, ))
    this_id = make_id(element)
    dup = True
    while dup:
        if this_id not in ids:
            dup = False
            ids.append(this_id)
        else:
            this_id = make_id(element)
    return ids[-1]


def get_xml_type(val):
    """Returns the data type for the xml type attribute"""
    LOG.info('Inside get_xml_type(). val = "%s", type(val) = "%s"' % (val, type(val).__name__))

    if type(val).__name__ == 'NoneType':
        LOG.info("type(val).__name__ == 'NoneType', returning 'null'")
        return 'null'

    elif type(val).__name__ == 'bool':
        LOG.info("type(val).__name__ == 'bool', returning 'bool'")
        return 'bool'

    elif type(val).__name__ in ('str', 'unicode'):
        LOG.info("type(val).__name__ in ('str', unicode'), returning 'str'")
        return 'str'

    elif type(val).__name__ in ('int', 'long'):
        LOG.info("type(val).__name__ in ('int', long'), returning 'int'")
        return 'int'

    elif type(val).__name__ == 'float':
        LOG.info("type(val).__name__ == 'float', returning 'float'")
        return 'float'

    elif isinstance(val, numbers.Number):
        LOG.info("isinstance(val, numbers.Number), returning 'number'")
        return 'number'

    elif isinstance(val, dict):
        LOG.info("isinstance(val, dict), returning 'dict'")
        return 'dict'

    elif isinstance(val, iterable):
        LOG.info("isinstance(val, iterable), returning 'list'")
        return 'list'

    LOG.info("type not found, returning '%s'" % (type(val).__name__))
    return type(val).__name__


def escape_xml(s):
    LOG.info('Inside escape_xml(). s = "%s" and type(s) = "%s"' % (s, type(s)))
    if type(s) in (str, unicode):
        s = unicode_me(s) # avoid UnicodeDecodeError
        s = s.replace('&', '&amp;')
        s = s.replace('"', '&quot;')
        s = s.replace('\'', '&apos;')
        s = s.replace('<', '&lt;')
        s = s.replace('>', '&gt;')
    return s


def make_attrstring(attr):
    """Returns an attribute string in the form key="val" """
    LOG.info('Inside make_attstring(). attr = "%s"' % (attr, ))
    attrstring = ' '.join(['%s="%s"' % (k, v) for k, v in attr.items()])
    return '%s%s' % (' ' if attrstring != '' else '', attrstring)


def key_is_valid_xml(key):
    """Checks that a key is a valid XML name"""
    LOG.info('Inside key_is_valid_xml(). Testing "%s"' % (unicode_me(key)))
    test_xml = '<?xml version="1.0" encoding="UTF-8" ?><%s>foo</%s>' % (key, key)
    try:
        parseString(test_xml)
        return True
    except Exception: # minidom does not implement exceptions well
        return False


def make_valid_xml_name(key, attr):
    """Tests an XML name and fixes it if invalid"""
    LOG.info('Inside make_valid_xml_name(). Testing key "%s" with attr "%s"' % (
        unicode_me(key), unicode_me(attr))
    )
    key = escape_xml(key)
    attr = escape_xml(attr)

    # pass through if key is already valid
    if key_is_valid_xml(key):
        return key, attr

    # prepend a lowercase n if the key is numeric
    # handle integers first
    if str(key).isdigit():
        return 'n%s' % (key), attr
    # now handle floats
    try:
        return 'n%s' % (float(str(key))), attr
    except ValueError:
        pass

    # replace spaces with underscores if that fixes the problem
    if key_is_valid_xml(key.replace(' ', '_')):
        return key.replace(' ', '_'), attr

    # key is still invalid - move it into a name attribute
    attr['name'] = key
    key = 'key'
    return key, attr


def wrap_cdata(val):
    """Wraps a string into CDATA sections"""
    LOG.info('Inside wrap_cdata(). val = "%s"' % (val, ))
    val = unicode_me(val).replace(']]>', ']]]]><![CDATA[>')
    return '<![CDATA[' + val + ']]>'


def default_item_func(parent):
    LOG.info('Inside default_item_func(). parent = "%s"' % (parent, ))
    return 'item'


def convert(obj, ids, attr_type, item_func, cdata, parent='root'):
    """Routes the elements of an object to the right function to convert them
    based on their data type"""
    LOG.info('Inside convert(). obj type is: "%s", obj="%s"' % (type(obj).__name__, unicode_me(obj)))

    item_name = item_func(parent)

    if type(obj) == bool:
        return convert_bool(item_name, obj, attr_type, cdata)

    if obj is None:
        return convert_none(item_name, obj, attr_type, cdata)

    if isinstance(obj, numbers.Number) or type(obj) in (str, unicode):
        return convert_kv(item_name, obj, attr_type, cdata)

    if hasattr(obj, 'isoformat'):
        return convert_kv(item_name, obj.isoformat(), attr_type, cdata)

    if isinstance(obj, dict):
        return convert_dict(obj, ids, parent, attr_type, item_func, cdata)

    if isinstance(obj, iterable):
        return convert_list(obj, ids, parent, attr_type, item_func, cdata)

    raise TypeError('Unsupported data type: %s (%s)' % (obj, type(obj).__name__))


def convert_dict(obj, ids, parent, attr_type, item_func, cdata):
    """Converts a dict into an XML string."""
    LOG.info('Inside convert_dict(): obj type is: "%s", obj="%s"' % (
        type(obj).__name__, unicode_me(obj))
    )
    output = []
    addline = output.append

    item_name = item_func(parent)

    for key, val in obj.items():
        LOG.info('Looping inside convert_dict(): key="%s", val="%s", type(val)="%s"' % (
            unicode_me(key), unicode_me(val), type(val).__name__)
        )

        attr = {} if not ids else {'id': '%s' % (get_unique_id(parent)) }

        key, attr = make_valid_xml_name(key, attr)

        if type(val) == bool:
            addline(convert_bool(key, val, attr_type, cdata, attr))

        elif isinstance(val, numbers.Number) or type(val) in (str, unicode):
            addline(convert_kv(key, val, attr_type, cdata, attr))

        elif hasattr(val, 'isoformat'): # datetime
            addline(convert_kv(key, val.isoformat(), attr_type, cdata, attr))

        elif type(val) == bool:
            addline(convert_bool(key, val, attr_type, cdata, attr))

        elif isinstance(val, dict):
            if attr_type:
                attr['type'] = get_xml_type(val)
            addline('<%s%s>%s</%s>' % (
                key, make_attrstring(attr),
                convert_dict(val, ids, key, attr_type, item_func, cdata),
                key
                )
            )

        elif isinstance(val, iterable):
            if attr_type:
                attr['type'] = get_xml_type(val)
            addline('<%s%s>%s</%s>' % (
                key,
                make_attrstring(attr),
                convert_list(val, ids, key, attr_type, item_func, cdata),
                key
                )
            )

        elif val is None:
            addline(convert_none(key, val, attr_type, cdata, attr))

        else:
            raise TypeError('Unsupported data type: %s (%s)' % (
                val, type(val).__name__)
            )

    return ''.join(output)


def convert_list(items, ids, parent, attr_type, item_func, cdata):
    """Converts a list into an XML string."""
    LOG.info('Inside convert_list()')
    output = []
    addline = output.append

    item_name = item_func(parent)

    if ids:
        this_id = get_unique_id(parent)

    for i, item in enumerate(items):
        LOG.info('Looping inside convert_list(): item="%s", item_name="%s", type="%s"' % (
            unicode_me(item), item_name, type(item).__name__)
        )
        attr = {} if not ids else { 'id': '%s_%s' % (this_id, i+1) }
        if isinstance(item, numbers.Number) or type(item) in (str, unicode):
            addline(convert_kv(item_name, item, attr_type, cdata, attr))

        elif hasattr(item, 'isoformat'): # datetime
            addline(convert_kv(item_name, item.isoformat(), attr_type, cdata, attr))

        elif type(item) == bool:
            addline(convert_bool(item_name, item, attr_type, cdata, attr))

        elif isinstance(item, dict):
            if not attr_type:
                addline('<%s>%s</%s>' % (
                    item_name,
                    convert_dict(item, ids, parent, attr_type, item_func, cdata),
                    item_name,
                    )
                )
            else:
                addline('<%s type="dict">%s</%s>' % (
                    item_name,
                    convert_dict(item, ids, parent, attr_type, item_func, cdata),
                    item_name,
                    )
                )

        elif isinstance(item, iterable):
            if not attr_type:
                addline('<%s %s>%s</%s>' % (
                    item_name, make_attrstring(attr),
                    convert_list(item, ids, item_name, attr_type, item_func, cdata),
                    item_name,
                    )
                )
            else:
                addline('<%s type="list"%s>%s</%s>' % (
                    item_name, make_attrstring(attr),
                    convert_list(item, ids, item_name, attr_type, item_func, cdata),
                    item_name,
                    )
                )

        elif item is None:
            addline(convert_none(item_name, None, attr_type, cdata, attr))

        else:
            raise TypeError('Unsupported data type: %s (%s)' % (
                item, type(item).__name__)
            )
    return ''.join(output)


def convert_kv(key, val, attr_type, cdata=False, attr=None):
    """Converts a number or string into an XML element"""
    LOG.info('Inside convert_kv(): key="%s", val="%s", type(val) is: "%s"' % (
        unicode_me(key), unicode_me(val), type(val).__name__)
    )

    if attr is None:
        attr = {}

    key, attr = make_valid_xml_name(key, attr)

    if attr_type:
        attr['type'] = get_xml_type(val)
    attrstring = make_attrstring(attr)
    return '<%s%s>%s</%s>' % (
        key, attrstring,
        wrap_cdata(val) if cdata == True else escape_xml(val),
        key
    )


def convert_bool(key, val, attr_type, cdata=False, attr=None):
    """Converts a boolean into an XML element"""
    LOG.info('Inside convert_bool(): key="%s", val="%s", type(val) is: "%s"' % (
        unicode_me(key),
        unicode_me(val),
        type(val).__name__
        )
    )

    if attr is None:
        attr = {}

    key, attr = make_valid_xml_name(key, attr)

    if attr_type:
        attr['type'] = get_xml_type(val)
    attrstring = make_attrstring(attr)
    return '<%s%s>%s</%s>' % (key, attrstring, unicode(val).lower(), key)


def convert_none(key, val, attr_type, cdata=False, attr=None):
    """Converts a null value into an XML element"""
    LOG.info('Inside convert_none(): key="%s". val="%s", attr_type="%s", attr=%s' % (
        unicode_me(key),
        unicode_me(val),
        unicode_me(attr_type),
        str(attr),
        )
    )

    if attr is None:
        attr = {}

    key, attr = make_valid_xml_name(key, attr)

    if attr_type:
        attr['type'] = get_xml_type(val)
    attrstring = make_attrstring(attr)
    return '<%s%s></%s>' % (key, attrstring, key)


def dicttoxml(
    obj,
    root = True,
    custom_root = 'root',
    xml_declaration = True,
    ids = False,
    attr_type = True,
    item_func = default_item_func,
    cdata = False,
    include_encoding = True,
    encoding = 'UTF-8',
    return_bytes = True,
    ):
    """Converts a python object into XML.
    Arguments:
    - root specifies whether the output is wrapped in an XML root element
      Default is True
    - custom_root allows you to specify a custom root element.
      Default is 'root'
    - ids specifies whether elements get unique ids.
      Default is False
    - attr_type specifies whether elements get a data type attribute.
      Default is True
    - item_func specifies what function should generate the element name for
      items in a list.
      Default is 'item'
    - cdata specifies whether string values should be wrapped in CDATA sections.
      Default is False
    """
    LOG.info('Inside dicttoxml(): type(obj) is: "%s", obj="%s"' % (type(obj).__name__, unicode_me(obj)))
    output = []
    addline = output.append
    if root == True:
        if xml_declaration == True:
            if include_encoding == False:
                addline('<?xml version="1.0" ?>')
            else:
                addline('<?xml version="1.0" encoding="%s" ?>' % (encoding))

        addline('<%s>%s</%s>' % (
        custom_root,
        convert(obj, ids, attr_type, item_func, cdata, parent=custom_root),
        custom_root,
        )
    )
    else:
        addline(convert(obj, ids, attr_type, item_func, cdata, parent=''))

    if return_bytes == False:
        return ''.join(output)
    return ''.join(output).encode('utf-8')
