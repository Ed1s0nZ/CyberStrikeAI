"""
"""

# Created on 2013.06.02
#
# Author: Giovanni Cannata
#
# Copyright 2013 - 2020 Giovanni Cannata
#
# This file is part of ldap3.
#
# ldap3 is free software: you can redistribute it and/or modify
# it under the terms of the GNU Lesser General Public License as published
# by the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# ldap3 is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU Lesser General Public License for more details.
#
# You should have received a copy of the GNU Lesser General Public License
# along with ldap3 in the COPYING and COPYING.LESSER files.
# If not, see <http://www.gnu.org/licenses/>.

from string import whitespace
from os import linesep

from .. import DEREF_NEVER, BASE, LEVEL, SUBTREE, DEREF_SEARCH, DEREF_BASE, DEREF_ALWAYS, NO_ATTRIBUTES, SEQUENCE_TYPES, get_config_parameter, STRING_TYPES

from ..core.exceptions import LDAPInvalidFilterError, LDAPAttributeError, LDAPInvalidScopeError, LDAPInvalidDereferenceAliasesError
from ..utils.ciDict import CaseInsensitiveDict
from ..protocol.rfc4511 import SearchRequest, LDAPDN, Scope, DerefAliases, Integer0ToMax, TypesOnly, \
    AttributeSelection, Selector, EqualityMatch, AttributeDescription, AssertionValue, Filter, \
    Not, And, Or, ApproxMatch, GreaterOrEqual, LessOrEqual, ExtensibleMatch, Present, SubstringFilter, \
    Substrings, Final, Initial, Any, ResultCode, Substring, MatchingRule, Type, MatchValue, DnAttributes
from ..operation.bind import referrals_to_list
from ..protocol.convert import ava_to_dict, attributes_to_list, search_refs_to_list, validate_assertion_value, prepare_filter_for_sending, search_refs_to_list_fast
from ..protocol.formatters.standard import format_attribute_values
from ..utils.conv import to_unicode, to_raw

ROOT = 0
AND = 1
OR = 2
NOT = 3
MATCH_APPROX = 4
MATCH_GREATER_OR_EQUAL = 5
MATCH_LESS_OR_EQUAL = 6
MATCH_EXTENSIBLE = 7
MATCH_PRESENT = 8
MATCH_SUBSTRING = 9
MATCH_EQUAL = 10

SEARCH_OPEN = 20
SEARCH_OPEN_OR_CLOSE = 21
SEARCH_MATCH_OR_CLOSE = 22
SEARCH_MATCH_OR_CONTROL = 23


class FilterNode(object):
    def __init__(self, tag=None, assertion=None):
        self.tag = tag
        self.parent = None
        self.assertion = assertion
        self.elements = []

    def append(self, filter_node):
        filter_node.parent = self
        self.elements.append(filter_node)
        return filter_node

    def __str__(self, pos=0):
        self.__repr__(pos)

    def __repr__(self, pos=0):
        node_tags = ['ROOT', 'AND', 'OR', 'NOT', 'MATCH_APPROX', 'MATCH_GREATER_OR_EQUAL', 'MATCH_LESS_OR_EQUAL', 'MATCH_EXTENSIBLE', 'MATCH_PRESENT', 'MATCH_SUBSTRING', 'MATCH_EQUAL']
        representation = ' ' * pos + 'tag: ' + node_tags[self.tag] + ' - assertion: ' + str(self.assertion)
        if self.elements:
            representation += ' - elements: ' + str(len(self.elements))
            for element in self.elements:
                representation += linesep + ' ' * pos + element.__repr__(pos + 2)
        return representation


def evaluate_match(match, schema, auto_escape, auto_encode, validator, check_names):
    left_part, equal_sign, right_part = match.strip().partition('=')
    if not equal_sign:
        raise LDAPInvalidFilterError('invalid matching assertion')
    if left_part.endswith('~'):  # approximate match '~='
        tag = MATCH_APPROX
        left_part = left_part[:-1].strip()
        right_part = right_part.strip()
        assertion = {'attr': left_part, 'value': validate_assertion_value(schema, left_part, right_part, auto_escape, auto_encode, validator, check_names)}
    elif left_part.endswith('>'):  # greater or equal match '>='
        tag = MATCH_GREATER_OR_EQUAL
        left_part = left_part[:-1].strip()
        right_part = right_part.strip()
        assertion = {'attr': left_part, 'value': validate_assertion_value(schema, left_part, right_part, auto_escape, auto_encode, validator, check_names)}
    elif left_part.endswith('<'):  # less or equal match '<='
        tag = MATCH_LESS_OR_EQUAL
        left_part = left_part[:-1].strip()
        right_part = right_part.strip()
        assertion = {'attr': left_part, 'value': validate_assertion_value(schema, left_part, right_part, auto_escape, auto_encode, validator, check_names)}
    elif left_part.endswith(':'):  # extensible match ':='
        tag = MATCH_EXTENSIBLE
        left_part = left_part[:-1].strip()
        right_part = right_part.strip()
        extended_filter_list = left_part.split(':')
        matching_rule = False
        dn_attributes = False
        attribute_name = False
        if extended_filter_list[0] == '':  # extensible filter format [:dn]:matchingRule:=assertionValue
            if len(extended_filter_list) == 2 and extended_filter_list[1].lower().strip() != 'dn':
                matching_rule = extended_filter_list[1]
            elif len(extended_filter_list) == 3 and extended_filter_list[1].lower().strip() == 'dn':
                dn_attributes = True
                matching_rule = extended_filter_list[2]
            else:
                raise LDAPInvalidFilterError('invalid extensible filter')
        elif len(extended_filter_list) <= 3:  # extensible filter format attr[:dn][:matchingRule]:=assertionValue
            if len(extended_filter_list) == 1:
                attribute_name = extended_filter_list[0]
            elif len(extended_filter_list) == 2:
                attribute_name = extended_filter_list[0]
                if extended_filter_list[1].lower().strip() == 'dn':
                    dn_attributes = True
                else:
                    matching_rule = extended_filter_list[1]
            elif len(extended_filter_list) == 3 and extended_filter_list[1].lower().strip() == 'dn':
                attribute_name = extended_filter_list[0]
                dn_attributes = True
                matching_rule = extended_filter_list[2]
            else:
                raise LDAPInvalidFilterError('invalid extensible filter')

        if not attribute_name and not matching_rule:
            raise LDAPInvalidFilterError('invalid extensible filter')
        attribute_name = attribute_name.strip() if attribute_name else False
        matching_rule = matching_rule.strip() if matching_rule else False
        assertion = {'attr': attribute_name, 'value': validate_assertion_value(schema, attribute_name, right_part, auto_escape, auto_encode, validator, check_names), 'matchingRule': matching_rule, 'dnAttributes': dn_attributes}
    elif right_part == '*':  # attribute present match '=*'
        tag = MATCH_PRESENT
        left_part = left_part.strip()
        assertion = {'attr': left_part}
    elif '*' in right_part:  # substring match '=initial*substring*substring*final'
        tag = MATCH_SUBSTRING
        left_part = left_part.strip()
        right_part = right_part.strip()
        substrings = right_part.split('*')
        initial = validate_assertion_value(schema, left_part, substrings[0], auto_escape, auto_encode, validator, check_names) if substrings[0] else None
        final = validate_assertion_value(schema, left_part, substrings[-1], auto_escape, auto_encode, validator, check_names) if substrings[-1] else None
        any_string = [validate_assertion_value(schema, left_part, substring, auto_escape, auto_encode, validator, check_names) for substring in substrings[1:-1] if substring]
        #assertion = {'attr': left_part, 'initial': initial, 'any': any_string, 'final': final}
        assertion = {'attr': left_part}
        if initial:
            assertion['initial'] =  initial
        if any_string:
            assertion['any'] = any_string
        if final:
            assertion['final'] =  final
    else:  # equality match '='
        tag = MATCH_EQUAL
        left_part = left_part.strip()
        right_part = right_part.strip()
        assertion = {'attr': left_part, 'value': validate_assertion_value(schema, left_part, right_part, auto_escape, auto_encode, validator, check_names)}

    return FilterNode(tag, assertion)


def parse_filter(search_filter, schema, auto_escape, auto_encode, validator, check_names):
    if str is not bytes and isinstance(search_filter, bytes):  # python 3 with byte filter
        search_filter = to_unicode(search_filter)
    search_filter = search_filter.strip()
    if search_filter and search_filter.count('(') == search_filter.count(')') and search_filter.startswith('(') and search_filter.endswith(')'):
        state = SEARCH_OPEN_OR_CLOSE
        root = FilterNode(ROOT)
        current_node = root
        start_pos = None
        skip_white_space = True
        just_closed = False
        for pos, c in enumerate(search_filter):
            if skip_white_space and c in whitespace:
                continue
            elif (state == SEARCH_OPEN or state == SEARCH_OPEN_OR_CLOSE) and c == '(':
                state = SEARCH_MATCH_OR_CONTROL
                just_closed = False
            elif state == SEARCH_MATCH_OR_CONTROL and c in '&!|':
                if c == '&':
                    current_node = current_node.append(FilterNode(AND))
                elif c == '|':
                    current_node = current_node.append(FilterNode(OR))
                elif c == '!':
                    current_node = current_node.append(FilterNode(NOT))
                state = SEARCH_OPEN
            elif (state == SEARCH_MATCH_OR_CLOSE or state == SEARCH_OPEN_OR_CLOSE) and c == ')':
                if just_closed:
                    current_node = current_node.parent
                else:
                    just_closed = True
                    skip_white_space = True
                    end_pos = pos
                    if start_pos:
                        if current_node.tag == NOT and len(current_node.elements) > 0:
                            raise LDAPInvalidFilterError('NOT (!) clause in filter cannot be multiple')
                        current_node.append(evaluate_match(search_filter[start_pos:end_pos], schema, auto_escape, auto_encode, validator, check_names))
                start_pos = None
                state = SEARCH_OPEN_OR_CLOSE
            elif (state == SEARCH_MATCH_OR_CLOSE or state == SEARCH_MATCH_OR_CONTROL) and c not in '()':
                skip_white_space = False
                if not start_pos:
                    start_pos = pos
                state = SEARCH_MATCH_OR_CLOSE
            else:
                raise LDAPInvalidFilterError('malformed filter')
        if len(root.elements) != 1:
            raise LDAPInvalidFilterError('missing boolean operator in filter')
        return root
    else:
        raise LDAPInvalidFilterError('invalid filter')


def compile_filter(filter_node):
    """Builds ASN1 structure for filter, converts from filter LDAP escaping to bytes"""
    compiled_filter = Filter()
    if filter_node.tag == AND:
        boolean_filter = And()
        pos = 0
        for element in filter_node.elements:
            boolean_filter[pos] = compile_filter(element)
            pos += 1
        compiled_filter['and'] = boolean_filter
    elif filter_node.tag == OR:
        boolean_filter = Or()
        pos = 0
        for element in filter_node.elements:
            boolean_filter[pos] = compile_filter(element)
            pos += 1
        compiled_filter['or'] = boolean_filter
    elif filter_node.tag == NOT:
        boolean_filter = Not()
        boolean_filter['innerNotFilter'] = compile_filter(filter_node.elements[0])
        compiled_filter.setComponentByName('notFilter', boolean_filter, verifyConstraints=False)  # do not verify constraints because of hack for recursive filters in rfc4511

    elif filter_node.tag == MATCH_APPROX:
        matching_filter = ApproxMatch()
        matching_filter['attributeDesc'] = AttributeDescription(filter_node.assertion['attr'])
        matching_filter['assertionValue'] = AssertionValue(prepare_filter_for_sending(filter_node.assertion['value']))
        compiled_filter['approxMatch'] = matching_filter
    elif filter_node.tag == MATCH_GREATER_OR_EQUAL:
        matching_filter = GreaterOrEqual()
        matching_filter['attributeDesc'] = AttributeDescription(filter_node.assertion['attr'])
        matching_filter['assertionValue'] = AssertionValue(prepare_filter_for_sending(filter_node.assertion['value']))
        compiled_filter['greaterOrEqual'] = matching_filter
    elif filter_node.tag == MATCH_LESS_OR_EQUAL:
        matching_filter = LessOrEqual()
        matching_filter['attributeDesc'] = AttributeDescription(filter_node.assertion['attr'])
        matching_filter['assertionValue'] = AssertionValue(prepare_filter_for_sending(filter_node.assertion['value']))
        compiled_filter['lessOrEqual'] = matching_filter
    elif filter_node.tag == MATCH_EXTENSIBLE:
        matching_filter = ExtensibleMatch()
        if filter_node.assertion['matchingRule']:
            matching_filter['matchingRule'] = MatchingRule(filter_node.assertion['matchingRule'])
        if filter_node.assertion['attr']:
            matching_filter['type'] = Type(filter_node.assertion['attr'])
        matching_filter['matchValue'] = MatchValue(prepare_filter_for_sending(filter_node.assertion['value']))
        matching_filter['dnAttributes'] = DnAttributes(filter_node.assertion['dnAttributes'])
        compiled_filter['extensibleMatch'] = matching_filter
    elif filter_node.tag == MATCH_PRESENT:
        matching_filter = Present(AttributeDescription(filter_node.assertion['attr']))
        compiled_filter['present'] = matching_filter
    elif filter_node.tag == MATCH_SUBSTRING:
        matching_filter = SubstringFilter()
        matching_filter['type'] = AttributeDescription(filter_node.assertion['attr'])
        substrings = Substrings()
        pos = 0
        if 'initial' in filter_node.assertion and filter_node.assertion['initial']:
            substrings[pos] = Substring().setComponentByName('initial', Initial(prepare_filter_for_sending(filter_node.assertion['initial'])))
            pos += 1
        if 'any' in filter_node.assertion and filter_node.assertion['any']:
            for substring in filter_node.assertion['any']:
                substrings[pos] = Substring().setComponentByName('any', Any(prepare_filter_for_sending(substring)))
                pos += 1
        if 'final' in filter_node.assertion and filter_node.assertion['final']:
            substrings[pos] = Substring().setComponentByName('final', Final(prepare_filter_for_sending(filter_node.assertion['final'])))
        matching_filter['substrings'] = substrings
        compiled_filter['substringFilter'] = matching_filter
    elif filter_node.tag == MATCH_EQUAL:
        matching_filter = EqualityMatch()
        matching_filter['attributeDesc'] = AttributeDescription(filter_node.assertion['attr'])
        matching_filter['assertionValue'] = AssertionValue(prepare_filter_for_sending(filter_node.assertion['value']))
        compiled_filter.setComponentByName('equalityMatch', matching_filter)
    else:
        raise LDAPInvalidFilterError('unknown filter node tag')

    return compiled_filter


def build_attribute_selection(attribute_list, schema):
    conf_attributes_excluded_from_check = [v.lower() for v in get_config_parameter('ATTRIBUTES_EXCLUDED_FROM_CHECK')]

    attribute_selection = AttributeSelection()
    for index, attribute in enumerate(attribute_list):
        if schema and schema.attribute_types:
            if ';' in attribute:  # exclude tags from validation
                if not attribute[0:attribute.index(';')] in schema.attribute_types and attribute.lower() not in conf_attributes_excluded_from_check:
                    raise LDAPAttributeError('invalid attribute type in attribute list: ' + attribute)
            else:
                if attribute not in schema.attribute_types and attribute.lower() not in conf_attributes_excluded_from_check:
                    raise LDAPAttributeError('invalid attribute type in attribute list: ' + attribute)
        attribute_selection[index] = Selector(attribute)

    return attribute_selection


def search_operation(search_base,
                     search_filter,
                     search_scope,
                     dereference_aliases,
                     attributes,
                     size_limit,
                     time_limit,
                     types_only,
                     auto_escape,
                     auto_encode,
                     schema=None,
                     validator=None,
                     check_names=False):
    # SearchRequest ::= [APPLICATION 3] SEQUENCE {
    # baseObject      LDAPDN,
    #     scope           ENUMERATED {
    #         baseObject              (0),
    #         singleLevel             (1),
    #         wholeSubtree            (2),
    #     ...  },
    #     derefAliases    ENUMERATED {
    #         neverDerefAliases       (0),
    #         derefInSearching        (1),
    #         derefFindingBaseObj     (2),
    #         derefAlways             (3) },
    #     sizeLimit       INTEGER (0 ..  maxInt),
    #     timeLimit       INTEGER (0 ..  maxInt),
    #     typesOnly       BOOLEAN,
    #     filter          Filter,
    #     attributes      AttributeSelection }
    request = SearchRequest()
    request['baseObject'] = LDAPDN(search_base)

    if search_scope == BASE or search_scope == 0:
        request['scope'] = Scope('baseObject')
    elif search_scope == LEVEL or search_scope == 1:
        request['scope'] = Scope('singleLevel')
    elif search_scope == SUBTREE or search_scope == 2:
        request['scope'] = Scope('wholeSubtree')
    else:
        raise LDAPInvalidScopeError('invalid scope type')

    if dereference_aliases == DEREF_NEVER or dereference_aliases == 0:
        request['derefAliases'] = DerefAliases('neverDerefAliases')
    elif dereference_aliases == DEREF_SEARCH or dereference_aliases == 1:
        request['derefAliases'] = DerefAliases('derefInSearching')
    elif dereference_aliases == DEREF_BASE or dereference_aliases == 2:
        request['derefAliases'] = DerefAliases('derefFindingBaseObj')
    elif dereference_aliases == DEREF_ALWAYS or dereference_aliases == 3:
        request['derefAliases'] = DerefAliases('derefAlways')
    else:
        raise LDAPInvalidDereferenceAliasesError('invalid dereference aliases type')

    request['sizeLimit'] = Integer0ToMax(size_limit)
    request['timeLimit'] = Integer0ToMax(time_limit)
    request['typesOnly'] = TypesOnly(True) if types_only else TypesOnly(False)
    request['filter'] = compile_filter(parse_filter(search_filter, schema, auto_escape, auto_encode, validator, check_names).elements[0])  # parse the searchFilter string and compile it starting from the root node
    if not isinstance(attributes, SEQUENCE_TYPES):
        attributes = [NO_ATTRIBUTES]

    request['attributes'] = build_attribute_selection(attributes, schema)

    return request


def decode_vals(vals):
    try:
        return [str(val) for val in vals if val] if vals else None
    except UnicodeDecodeError:
        return decode_raw_vals(vals)

def decode_vals_fast(vals):
    try:
        return [to_unicode(val[3], from_server=True) for val in vals if val] if vals else None
    except UnicodeDecodeError:
        return [val[3] for val in vals if val] if vals else None


def attributes_to_dict(attribute_list):
    conf_case_insensitive_attributes = get_config_parameter('CASE_INSENSITIVE_ATTRIBUTE_NAMES')
    attributes = CaseInsensitiveDict() if conf_case_insensitive_attributes else dict()
    for attribute in attribute_list:
            attributes[str(attribute['type'])] = decode_vals(attribute['vals'])
    return attributes


def attributes_to_dict_fast(attribute_list):
    conf_case_insensitive_attributes = get_config_parameter('CASE_INSENSITIVE_ATTRIBUTE_NAMES')
    attributes = CaseInsensitiveDict() if conf_case_insensitive_attributes else dict()
    for attribute in attribute_list:
        attributes[to_unicode(attribute[3][0][3], from_server=True)] = decode_vals_fast(attribute[3][1][3])

    return attributes


def decode_raw_vals(vals):
    return [bytes(val) for val in vals] if vals else None


def decode_raw_vals_fast(vals):
    return [bytes(val[3]) for val in vals] if vals else None


def raw_attributes_to_dict(attribute_list):
    conf_case_insensitive_attributes = get_config_parameter('CASE_INSENSITIVE_ATTRIBUTE_NAMES')

    attributes = CaseInsensitiveDict() if conf_case_insensitive_attributes else dict()
    for attribute in attribute_list:
        attributes[str(attribute['type'])] = decode_raw_vals(attribute['vals'])

    return attributes


def raw_attributes_to_dict_fast(attribute_list):
    conf_case_insensitive_attributes = get_config_parameter('CASE_INSENSITIVE_ATTRIBUTE_NAMES')
    attributes = CaseInsensitiveDict() if conf_case_insensitive_attributes else dict()
    for attribute in attribute_list:
        attributes[to_unicode(attribute[3][0][3], from_server=True)] = decode_raw_vals_fast(attribute[3][1][3])

    return attributes


def checked_attributes_to_dict(attribute_list, schema=None, custom_formatter=None):
    conf_case_insensitive_attributes = get_config_parameter('CASE_INSENSITIVE_ATTRIBUTE_NAMES')

    checked_attributes = CaseInsensitiveDict() if conf_case_insensitive_attributes else dict()
    for attribute in attribute_list:
        name = str(attribute['type'])
        checked_attributes[name] = format_attribute_values(schema, name, decode_raw_vals(attribute['vals']) or [], custom_formatter)
    return checked_attributes


def checked_attributes_to_dict_fast(attribute_list, schema=None, custom_formatter=None):
    conf_case_insensitive_attributes = get_config_parameter('CASE_INSENSITIVE_ATTRIBUTE_NAMES')

    checked_attributes = CaseInsensitiveDict() if conf_case_insensitive_attributes else dict()
    for attribute in attribute_list:
        name = to_unicode(attribute[3][0][3], from_server=True)
        checked_attributes[name] = format_attribute_values(schema, name, decode_raw_vals_fast(attribute[3][1][3]) or [], custom_formatter)
    return checked_attributes


def matching_rule_assertion_to_string(matching_rule_assertion):
    return str(matching_rule_assertion)


def filter_to_string(filter_object):
    filter_type = filter_object.getName()
    filter_string = '('
    if filter_type == 'and':
        filter_string += '&'
        for f in filter_object['and']:
            filter_string += filter_to_string(f)
    elif filter_type == 'or':
        filter_string += '|'
        for f in filter_object['or']:
            filter_string += filter_to_string(f)
    elif filter_type == 'notFilter':
        filter_string += '!' + filter_to_string(filter_object['notFilter']['innerNotFilter'])
    elif filter_type == 'equalityMatch':
        ava = ava_to_dict(filter_object['equalityMatch'])
        filter_string += ava['attribute'] + '=' + ava['value']
    elif filter_type == 'substringFilter':
        attribute = filter_object['substringFilter']['type']
        filter_string += str(attribute) + '='
        for substring in filter_object['substringFilter']['substrings']:
            component = substring.getName()
            if substring[component] is not None and substring[component].hasValue():
                if component == 'initial':
                    filter_string += str(substring['initial']) + '*'
                elif component == 'any':
                    filter_string += str(substring['any']) if filter_string.endswith('*') else '*' + str(substring['any'])
                    filter_string += '*'
                elif component == 'final':
                    filter_string += '*' + str(substring['final'])
    elif filter_type == 'greaterOrEqual':
        ava = ava_to_dict(filter_object['greaterOrEqual'])
        filter_string += ava['attribute'] + '>=' + ava['value']
    elif filter_type == 'lessOrEqual':
        ava = ava_to_dict(filter_object['lessOrEqual'])
        filter_string += ava['attribute'] + '<=' + ava['value']
    elif filter_type == 'present':
        filter_string += str(filter_object['present']) + '=*'
    elif filter_type == 'approxMatch':
        ava = ava_to_dict(filter_object['approxMatch'])
        filter_string += ava['attribute'] + '~=' + ava['value']
    elif filter_type == 'extensibleMatch':
        filter_string += matching_rule_assertion_to_string(filter_object['extensibleMatch'])
    else:
        raise LDAPInvalidFilterError('error converting filter to string')
    filter_string += ')'

    if str is bytes:  # Python2, forces conversion to Unicode
        filter_string = to_unicode(filter_string)

    return filter_string


def search_request_to_dict(request):
    return {'base': str(request['baseObject']),
            'scope': int(request['scope']),
            'dereferenceAlias': int(request['derefAliases']),
            'sizeLimit': int(request['sizeLimit']),
            'timeLimit': int(request['timeLimit']),
            'typesOnly': bool(request['typesOnly']),
            'filter': filter_to_string(request['filter']),
            'attributes': attributes_to_list(request['attributes'])}


def search_result_entry_response_to_dict(response, schema, custom_formatter, check_names):
    entry = dict()
    # entry['dn'] = str(response['object'])
    if response['object']:
        if isinstance(response['object'], STRING_TYPES):  # mock strategies return string not a PyAsn1 object
            entry['raw_dn'] = to_raw(response['object'])
            entry['dn'] = to_unicode(response['object'])
        else:
            entry['raw_dn'] = str(response['object'])
            entry['dn'] = to_unicode(bytes(response['object']), from_server=True)
    else:
        entry['raw_dn'] = b''
        entry['dn'] = ''
    entry['raw_attributes'] = raw_attributes_to_dict(response['attributes'])
    if check_names:
        entry['attributes'] = checked_attributes_to_dict(response['attributes'], schema, custom_formatter)
    else:
        entry['attributes'] = attributes_to_dict(response['attributes'])

    return entry


def search_result_done_response_to_dict(response):
    result = {'result': int(response['resultCode']),
            'description': ResultCode().getNamedValues().getName(response['resultCode']),
            'message': str(response['diagnosticMessage']),
            'dn': str(response['matchedDN']),
            'referrals': referrals_to_list(response['referral'])}

    if 'controls' in response:  # used for returning controls in Mock strategies
        result['controls'] = dict()
        for control in response['controls']:
            result['controls'][control[0]] = control[1]

    return result


def search_result_reference_response_to_dict(response):
    return {'uri': search_refs_to_list(response)}


def search_result_entry_response_to_dict_fast(response, schema, custom_formatter, check_names):
    entry_dict = dict()
    entry_dict['raw_dn'] = response[0][3]
    entry_dict['dn'] = to_unicode(response[0][3], from_server=True)
    entry_dict['raw_attributes'] = raw_attributes_to_dict_fast(response[1][3])  # attributes
    if check_names:
        entry_dict['attributes'] = checked_attributes_to_dict_fast(response[1][3], schema, custom_formatter)  # attributes
    else:
        entry_dict['attributes'] = attributes_to_dict_fast(response[1][3])  # attributes

    return entry_dict


def search_result_reference_response_to_dict_fast(response):
    return {'uri': search_refs_to_list_fast([r[3] for r in response])}
