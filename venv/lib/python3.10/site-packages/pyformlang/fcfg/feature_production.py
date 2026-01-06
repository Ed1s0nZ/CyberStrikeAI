"""Production rules with features"""
from typing import List

from pyformlang.cfg import Production, Variable
from pyformlang.cfg.cfg_object import CFGObject
from pyformlang.fcfg.feature_structure import FeatureStructure


class FeatureProduction(Production):
    """ A feature production or rule of a FCFG

    Parameters
    ----------
    head : :class:`~pyformlang.cfg.Variable`
        The head of the production
    body : iterable of :class:`~pyformlang.cfg.CFGObject`
        The body of the production
    head_feature : :class:`~pyformlang.fcfg.FeatureStructure`
        The feature structure of the head
    body_features : Iterable of :class:`~pyformlang.fcfg.FeatureStructure`
        The feature structures of the elements of the body. Must be the same size as the body.
    """

    def __init__(self, head: Variable, body: List[CFGObject], head_feature, body_features, filtering=True):
        super().__init__(head, body, filtering)
        self._features = FeatureStructure()
        self._features.add_content("head", head_feature)
        for i, feature_structure in enumerate(body_features):
            self._features.add_content(str(i), feature_structure)

    @property
    def features(self):
        """The merged features of the production rules"""
        return self._features

    def __repr__(self):
        res = [self.head.to_text()]
        cond_head = str(self._features.get_feature_by_path(["head"]))
        if cond_head:
            res.append("[" + cond_head + "]")
        res.append("->")
        for i, body_part in enumerate(self.body):
            res.append(body_part.to_text())
            body_part_cond = str(self._features.get_feature_by_path([str(i)]))
            if body_part_cond:
                res.append("[" + body_part_cond + "]")
        return " ".join(res)
