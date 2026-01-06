"""
"""

# Created on 2015.01.07
#
# Author: Giovanni Cannata
#
# Copyright 2015 - 2020 Giovanni Cannata
#
# This file is part of ldap3.
#
# ldap3 is free software: you can redistribute it and/or modify
# it under the terms of the GNU Lesser General Public License as published
# by the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# ldap3 is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU Lesser General Public License for more details.
#
# You should have received a copy of the GNU Lesser General Public License
# along with ldap3 in the COPYING and COPYING.LESSER files.
# If not, see <http://www.gnu.org/licenses/>.

from datetime import timedelta, tzinfo


# from python standard library docs
class OffsetTzInfo(tzinfo):
    """Fixed offset in minutes east from UTC"""

    def __init__(self, offset, name):
        self.offset = offset
        self.name = name
        self._offset = timedelta(minutes=offset)

    def __str__(self):
        return self.name

    def __repr__(self):

        return 'OffsetTzInfo(offset={0.offset!r}, name={0.name!r})'.format(self)

    def utcoffset(self, dt):
        return self._offset

    def tzname(self, dt):
        return self.name

    # noinspection PyMethodMayBeStatic
    def dst(self, dt):
        return timedelta(0)

    def __getinitargs__(self):  # for pickling/unpickling
        return self.offset, self.name
