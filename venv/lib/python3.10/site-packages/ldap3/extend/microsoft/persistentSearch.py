"""
"""

# Created on 2016.07.08
#
# Author: Giovanni Cannata
#
# Copyright 2016 - 2020 Giovanni Cannata
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

try:
    from queue import Empty
except ImportError:  # Python 2
    # noinspection PyUnresolvedReferences
    from Queue import Empty

from ...core.exceptions import LDAPExtensionError
from ...utils.dn import safe_dn
from ...protocol.microsoft import persistent_search_control


class ADPersistentSearch(object):
    def __init__(self,
                 connection,
                 search_base,
                 search_scope,
                 attributes,
                 streaming,
                 callback
                 ):
        if connection.strategy.sync:
            raise LDAPExtensionError('Persistent Search needs an asynchronous streaming connection')

        if connection.check_names and search_base:
            search_base = safe_dn(search_base)

        self.connection = connection
        self.message_id = None
        self.base = search_base
        self.scope = search_scope
        self.attributes = attributes
        self.controls = [persistent_search_control()]

        # this is the only filter permitted by AD persistent search
        self.filter = '(objectClass=*)'

        self.connection.strategy.streaming = streaming
        if callback and callable(callback):
            self.connection.strategy.callback = callback
        elif callback:
            raise LDAPExtensionError('callback is not callable')

        self.start()

    def start(self):
        if self.message_id:  # persistent search already started
            return

        if not self.connection.bound:
            self.connection.bind()

        with self.connection.strategy.async_lock:
            self.message_id = self.connection.search(search_base=self.base,
                                                     search_filter=self.filter,
                                                     search_scope=self.scope,
                                                     attributes=self.attributes,
                                                     controls=self.controls)
            self.connection.strategy.persistent_search_message_id = self.message_id

    def stop(self, unbind=True):
        self.connection.abandon(self.message_id)
        if unbind:
            self.connection.unbind()
        if self.message_id in self.connection.strategy._responses:
            del self.connection.strategy._responses[self.message_id]
        if hasattr(self.connection.strategy, '_requests') and self.message_id in self.connection.strategy._requests:  # asynchronous strategy has a dict of request that could be returned by get_response()
            del self.connection.strategy._requests[self.message_id]
        self.connection.strategy.persistent_search_message_id = None
        self.message_id = None

    def next(self, block=False, timeout=None):
        if not self.connection.strategy.streaming and not self.connection.strategy.callback:
            try:
                return self.connection.strategy.events.get(block, timeout)
            except Empty:
                return None

        raise LDAPExtensionError('Persistent search is not accumulating events in queue')

    def funnel(self, block=False, timeout=None):
        done = False
        while not done:
            try:
                entry = self.connection.strategy.events.get(block, timeout)
            except Empty:
                yield None
            if entry['type'] == 'searchResEntry':
                yield entry
            else:
                done = True

        yield entry
