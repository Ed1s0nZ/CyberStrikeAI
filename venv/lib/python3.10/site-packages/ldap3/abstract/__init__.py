"""
"""

# Created on 2016.08.31
#
# Author: Giovanni Cannata
#
# Copyright 2014 - 2020 Giovanni Cannata
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

STATUS_INIT = 'Initialized'  # The entry object is initialized
STATUS_VIRTUAL = 'Virtual'  # The entry is a new writable entry, still empty
STATUS_MANDATORY_MISSING = 'Missing mandatory attributes'  # The entry has some mandatory attributes missing
STATUS_READ = 'Read'  # The entry has been read
STATUS_WRITABLE = 'Writable'  # The entry has been made writable, still no changes
STATUS_PENDING_CHANGES = 'Pending changes'  # The entry has some changes to commit, mandatory attributes are present
STATUS_COMMITTED = 'Committed'  # The entry changes has been committed
STATUS_READY_FOR_DELETION = 'Ready for deletion'  # The entry is set to be deleted
STATUS_READY_FOR_MOVING = 'Ready for moving'  # The entry is set to be moved in the DIT
STATUS_READY_FOR_RENAMING = 'Ready for renaming'  # The entry is set to be renamed
STATUS_DELETED = 'Deleted'  # The entry has been deleted

STATUSES = [STATUS_INIT,
            STATUS_VIRTUAL,
            STATUS_MANDATORY_MISSING,
            STATUS_READ,
            STATUS_WRITABLE,
            STATUS_PENDING_CHANGES,
            STATUS_COMMITTED,
            STATUS_READY_FOR_DELETION,
            STATUS_READY_FOR_MOVING,
            STATUS_READY_FOR_RENAMING,
            STATUS_DELETED]

INITIAL_STATUSES = [STATUS_READ, STATUS_WRITABLE, STATUS_VIRTUAL]
