"""Feature Structure"""
from typing import Any, List, Dict


class ContentAlreadyExistsException(Exception):
    """Exception raised when we want to add a content that already exists"""


class PathDoesNotExistsException(Exception):
    """Raised when looking for a path that does not exist"""


class FeatureStructuresNotCompatibleException(Exception):
    """Raised when trying to unify uncompatible structures"""


class FeatureStructure:
    """ The feature structure containing constraints

    Parameters
    ----------
    value : Any, optional
        The value of the feature, if defined

    """

    def __init__(self, value=None):
        self._content = {}
        self._value = value
        self._pointer = None

    def copy(self, already_copied=None):
        """Copies the current feature structure

        Parameters
        ----------
        already_copied : dict
             A dictionary containing the parts already copied. For internal usage.

        Returns
        ----------
        fs : :class:`~pyformlang.fcfg.FeatureStructure`
            The copied feature structure
        """
        if already_copied is None:
            already_copied = {}
        if self in already_copied:
            return already_copied[self]
        new_fs = FeatureStructure(self.value)
        if self._pointer is not None:
            pointer_copy = self._pointer.copy(already_copied)
            new_fs.pointer = pointer_copy
        for feature, content in self._content.items():
            new_fs.content[feature] = content.copy(already_copied)
        already_copied[self] = new_fs
        return new_fs

    @property
    def content(self) -> Any:
        """Gets the content of the current node"""
        return self._content

    @property
    def pointer(self) -> Any:
        """Gets the pointer of the current node"""
        return self._pointer

    @pointer.setter
    def pointer(self, new_pointer):
        """Set the value of the pointer"""
        self._pointer = new_pointer


    @property
    def value(self) -> Any:
        """Gets the value associated to the current node"""
        return self._value if self.pointer is None else self.pointer.value

    @value.setter
    def value(self, new_value) -> Any:
        """Gets the value associated to the current node"""
        self._value = new_value

    def add_content(self, content_name: str, feature_structure: "FeatureStructure"):
        """Add content to the current feature structure.

        Parameters
        ----------
        content_name : str
             The name of the new feature
        feature_structure : :class:`~pyformlang.fcfg.FeatureStructure`
            The value of this new feature

        Raises
        ----------
        ContentAlreadyExistsException
            When the feature already exists
        """
        if content_name in self._content:
            raise ContentAlreadyExistsException()
        self._content[content_name] = feature_structure

    def add_content_path(self, content_name: str, feature_structure: "FeatureStructure", path: List[str]):
        """Add content to the current feature structure at a specific path

        Parameters
        ----------
        content_name : str
             The name of the new feature
        feature_structure : :class:`~pyformlang.fcfg.FeatureStructure`
            The value of this new feature
        path : Iterable of str
            The path where to add the new feature.

        Raises
        ----------
        ContentAlreadyExistsException
            When the feature already exists
        PathDoesNotExistsException
            When the path does not exist
        """
        to_modify = self.get_feature_by_path(path)
        to_modify.add_content(content_name, feature_structure)

    def get_dereferenced(self):
        """Get the dereferences version of the feature structure. For internal usage."""
        return self._pointer.get_dereferenced() if self._pointer is not None else self

    def get_feature_by_path(self, path: List[str] = None):
        """ Get a feature at a given path.

        Parameters
        -----------
        path : Iterable of str, optional
            The path to the new feature.

        Returns
        -------
        feature_structure : :class:`~pyformlang.fcfg.FeatureStructure`
            The feature structure at the end of the path.

        Raises
        ----------
        PathDoesNotExistsException
            When the path does not exist

        """
        if not path or path is None:
            return self
        current = self.get_dereferenced()
        if path[0] not in current.content:
            raise PathDoesNotExistsException()
        return current.content[path[0]].get_feature_by_path(path[1:])

    def unify(self, other: "FeatureStructure"):
        """Unify the current structure with another one.

        Modifies the current structure.

        Parameters
        ----------
        other : :class:`~pyformlang.fcfg.FeatureStructure`
            The other feature structure to unify.

        Raises
        ----------
        FeatureStructuresNotCompatibleException
            When the feature structure cannot be unified.
        """
        current_dereferenced = self.get_dereferenced()
        other_dereferenced = other.get_dereferenced()
        if current_dereferenced == other_dereferenced:
            return
        if len(current_dereferenced.content) == 0 and len(other_dereferenced.content) == 0:
            # We have a simple feature
            if current_dereferenced.value == other_dereferenced.value:
                current_dereferenced.pointer = other_dereferenced
            elif current_dereferenced.value is None:
                current_dereferenced.pointer = other_dereferenced
            elif other_dereferenced.value is None:
                other_dereferenced.pointer = current_dereferenced
            else:
                raise FeatureStructuresNotCompatibleException()
        else:
            other_dereferenced.pointer = current_dereferenced
            for feature in other_dereferenced.content:
                if feature not in current_dereferenced.content:
                    current_dereferenced.content[feature] = FeatureStructure()
                current_dereferenced.content[feature].unify(other_dereferenced.content[feature])

    def subsumes(self, other: "FeatureStructure"):
        """Check whether the current feature structure subsumes another one.

        Parameters
        ----------
        other : :class:`~pyformlang.fcfg.FeatureStructure`
            The other feature structure to unify.

        Returns
        ----------
        subsumes : bool
            Whether the current feature structure subsumes the one.
        """
        current_dereferenced = self.get_dereferenced()
        other_dereferenced = other.get_dereferenced()
        if current_dereferenced.value != other_dereferenced.value:
            return False
        for feature in current_dereferenced.content:
            if feature not in other_dereferenced.content:
                return False
            if not current_dereferenced.content[feature].subsumes(other_dereferenced.content[feature]):
                return False
        return True

    def get_all_paths(self):
        """ Get the list of all path in the feature structure

        Returns
        --------
        paths : Iterable of :class:`~pyformlang.fcfg.FeatureStructure`
            The paths

        """
        res = []
        for feature, content in self._content.items():
            paths = content.get_all_paths()
            for path in paths:
                res.append([feature] + path)
        if not res:
            res.append([])
        return res

    def __repr__(self):
        res = []
        for path in self.get_all_paths():
            if path:
                feature = self.get_feature_by_path(path)
                value = feature.value
                if value is None:
                    value = id(feature)
                res.append(".".join(path) + "=" + str(value))
        return " | ".join(res)

    @classmethod
    def from_text(cls, text: str, structure_variables: Dict[str, "FeatureStructure"] = None):
        """ Construct a feature structure from a text.

        Parameters
        -----------
        text : str
            The text to parse
        structure_variables : dict of (str, :class:`~pyformlang.fcfg.FeatureStructure`), optional
            Existing structure variables.

        Returns
        --------
        feature_structure : :class:`~pyformlang.fcfg.FeatureStructure`
            The parsed feature structure

        """
        if structure_variables is None:
            structure_variables = {}
        preprocessed_conditions = _preprocess_conditions(text)
        return _create_feature_structure(preprocessed_conditions, structure_variables)


def _find_closing_bracket(condition, start, opening="[", closing="]"):
    counter = 0
    pos = start
    for current_char in condition[start:]:
        if current_char == opening:
            counter += 1
        elif current_char == closing:
            counter -= 1
        if counter == 0:
            return pos
        pos += 1
    return -1


class ParsingException(Exception):
    """When there is a problem during parsing."""


def _preprocess_conditions(conditions, start=0, end=-1):
    conditions = conditions.replace("->", "=")
    conditions = conditions.strip()
    res = []
    reading_feature = True
    current_feature = ""
    current_value = ""
    reference = None
    pos = start
    end = len(conditions) if end == -1 else end
    while pos < end:
        current = conditions[pos]
        if current == "=":
            reading_feature = False
            pos += 1
        elif reading_feature:
            current_feature += current
            pos += 1
        elif current == "[":
            end_bracket = _find_closing_bracket(conditions, pos)
            if end_bracket == -1:
                raise ParsingException()
            current_value = _preprocess_conditions(conditions, pos + 1, end_bracket)
            pos = end_bracket + 1
        elif current == "(":
            end_bracket = _find_closing_bracket(conditions, pos, "(", ")")
            if end_bracket == -1:
                raise ParsingException()
            reference = conditions[pos+1: end_bracket]
            pos = end_bracket + 1
        elif current == ",":
            reading_feature = True
            if isinstance(current_value, str):
                current_value = current_value.strip()
            res.append((current_feature.strip(), current_value, reference))
            current_feature = ""
            current_value = ""
            reference = None
            pos += 1
        else:
            current_value += current
            pos += 1
    if current_feature.strip():
        if isinstance(current_value, str):
            current_value = current_value.strip()
        res.append((current_feature.strip(), current_value, reference))
    return res


def _create_feature_structure(conditions, structure_variables, existing_references=None, feature_structure=None):
    if existing_references is None:
        existing_references = {}
    if feature_structure is None:
        feature_structure = FeatureStructure()
    for feature, value, reference in conditions:
        if reference is not None:
            if reference not in existing_references:
                existing_references[reference] = FeatureStructure()
            new_fs = existing_references[reference]
        else:
            new_fs = FeatureStructure()
        if value and isinstance(value, str):
            if value[0] != "?":
                new_fs.value = value
                feature_structure.add_content(feature, new_fs)
            elif value[1:] in structure_variables:
                new_fs.pointer = structure_variables[value[1:]]
                feature_structure.add_content(feature, new_fs)
            else:
                feature_structure.add_content(feature, new_fs)
                structure_variables[value[1:]] = new_fs
        elif not isinstance(value, str):
            structure = _create_feature_structure(value, structure_variables, existing_references, new_fs)
            feature_structure.add_content(feature, structure)
        else:
            feature_structure.add_content(feature, new_fs)
    return feature_structure
