from __future__ import annotations

import logging

from cle.backends.relocation import Relocation

log = logging.getLogger(name=__name__)


class ELFReloc(Relocation):
    def __init__(self, owner, symbol, relative_addr, addend=None):
        super().__init__(owner, symbol, relative_addr)

        if addend is not None:
            self.is_rela = True
            self._addend = addend
        else:
            self.is_rela = False
            self._addend = self.owner.memory.unpack_word(self.relative_addr)

    @property
    def addend(self):
        if self._addend is None:
            self._addend = self.owner.memory.unpack_word(self.relative_addr)
        return self._addend

    @property
    def value(self):  # pylint: disable=no-self-use
        log.error("Value property of Relocation must be overridden by subclass!")
        return 0
