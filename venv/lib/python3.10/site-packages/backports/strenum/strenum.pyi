from enum import Enum

from typing_extensions import Self

class StrEnum(str, Enum):
    def __new__(cls, value: str) -> Self: ...
    _value_: str
    @property
    def value(self) -> str: ...
    @staticmethod
    def _generate_next_value_(
        name: str, start: int, count: int, last_values: list[str]
    ) -> str: ...
