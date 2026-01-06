""" An epsilon symbol """

from .symbol import Symbol


class Epsilon(Symbol):
    """ An epsilon symbol """
    # pylint: disable=too-few-public-methods

    def __init__(self):
        super().__init__("epsilon")
