from __future__ import annotations

from logging import LogRecord


class UniqueLogFilter:
    _logged: set[str]

    def __init__(self):
        self._logged = set()

    def filter(self, record: LogRecord):
        msg = record.getMessage()
        if msg in self._logged:
            return False
        self._logged.add(msg)
        return True
