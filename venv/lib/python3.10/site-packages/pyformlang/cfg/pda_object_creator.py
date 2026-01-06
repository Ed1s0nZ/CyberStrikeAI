"""Creation of objects for PDA"""

from pyformlang import cfg
from pyformlang import pda


class PDAObjectCreator:
    """Creates Objects for a PDA"""

    def __init__(self, terminals, variables):
        self._inverse_symbol = {}
        self._inverse_stack_symbol = {}
        for terminal in terminals:
            self._inverse_symbol[terminal] = None
            self._inverse_stack_symbol[terminal] = None
        for variable in variables:
            self._inverse_stack_symbol[variable] = None

    def get_symbol_from(self, symbol):
        """Get a symbol"""
        if isinstance(symbol, cfg.Epsilon):
            return pda.Epsilon()
        if self._inverse_symbol[symbol] is None:
            value = str(symbol.value)
            temp = pda.Symbol(value)
            self._inverse_symbol[symbol] = temp
            return temp
        return self._inverse_symbol[symbol]

    def get_stack_symbol_from(self, stack_symbol):
        """Get a stack symbol"""
        if isinstance(stack_symbol, cfg.Epsilon):
            return pda.Epsilon()
        if self._inverse_stack_symbol[stack_symbol] is None:
            value = str(stack_symbol.value)
            if isinstance(stack_symbol, cfg.Terminal):
                value = "#TERM#" + value
            temp = pda.StackSymbol(value)
            self._inverse_stack_symbol[stack_symbol] = temp
            return temp
        return self._inverse_stack_symbol[stack_symbol]
