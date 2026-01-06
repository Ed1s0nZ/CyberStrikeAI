"""
:mod:`pyformlang.fcfg`
=======================

This submodule implements functions related to feature-based grammars.


Available Classes
-----------------

Sources
-------

Daniel Jurafsky and James H. Martin, Speech and Language Processing

"""

__all__ = ["FCFG",
           "FeatureStructure",
           "FeatureProduction",
           "ContentAlreadyExistsException",
           "FeatureStructuresNotCompatibleException",
           "PathDoesNotExistsException"]

from pyformlang.fcfg.fcfg import FCFG
from pyformlang.fcfg.feature_production import FeatureProduction
from pyformlang.fcfg.feature_structure import FeatureStructure, ContentAlreadyExistsException, \
    FeatureStructuresNotCompatibleException, PathDoesNotExistsException
