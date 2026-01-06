""" An epsilon terminal """

from .terminal import Terminal


class Epsilon(Terminal):
    """ An epsilon terminal """
    # pylint: disable=too-few-public-methods

    def __init__(self):
        super().__init__("epsilon")

    def to_text(self) -> str:
        return "epsilon"
