from __future__ import absolute_import
from future.utils import PY3

from multiprocessing import *
if not PY3:
    __future_module__ = True
    from multiprocessing.queues import SimpleQueue
