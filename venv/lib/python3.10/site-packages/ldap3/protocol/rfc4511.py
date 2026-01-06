"""
"""

# Created on 2013.05.15
#
# Author: Giovanni Cannata
#
# Copyright 2014 - 2020 Giovanni Cannata
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

#######################
# ldap ASN.1 Definition
# from RFC4511 - Appendix B
# extended with result codes from IANA ldap-parameters as of 2013.08.21
# extended with modify_increment from RFC4525

#########################################################
# Lightweight-Directory-Access-Protocol-V3 {1 3 6 1 1 18}
# -- Copyright (C) The Internet Society (2006).  This version of
# -- this ASN.1 module is part of RFC 4511; see the RFC itself
# -- for full legal notices.
# DEFINITIONS
# IMPLICIT TAGS
# EXTENSIBILITY IMPLIED

from pyasn1.type.univ import OctetString, Integer, Sequence, Choice, SequenceOf, Boolean, Null, Enumerated, SetOf
from pyasn1.type.namedtype import NamedTypes, NamedType, OptionalNamedType, DefaultedNamedType
from pyasn1.type.constraint import ValueRangeConstraint, SingleValueConstraint, ValueSizeConstraint
from pyasn1.type.namedval import NamedValues
from pyasn1.type.tag import tagClassApplication, tagFormatConstructed, Tag, tagClassContext, tagFormatSimple


# constants
# maxInt INTEGER ::= 2147483647 -- (2^^31 - 1) --
LDAP_MAX_INT = 2147483647
MAXINT = Integer(LDAP_MAX_INT)

# constraints
rangeInt0ToMaxConstraint = ValueRangeConstraint(0, MAXINT)
rangeInt1To127Constraint = ValueRangeConstraint(1, 127)
size1ToMaxConstraint = ValueSizeConstraint(1, MAXINT)
responseValueConstraint = SingleValueConstraint(0, 1, 2, 3, 4, 5, 6, 7, 8, 10, 11, 12, 13, 14, 16, 17, 18, 19, 20, 21, 32, 33, 34, 36, 48, 49, 50, 51, 52, 53, 54, 64, 65, 66, 67, 68, 69, 71, 80, 113, 114, 115, 116, 117, 118, 119, 120, 121, 122, 123,
                                                4096)

# custom constraints
numericOIDConstraint = None  # TODO
distinguishedNameConstraint = None  # TODO
nameComponentConstraint = None  # TODO
attributeDescriptionConstraint = None  # TODO
uriConstraint = None  # TODO
attributeSelectorConstraint = None  # TODO


class Integer0ToMax(Integer):
    subtypeSpec = Integer.subtypeSpec + rangeInt0ToMaxConstraint


class LDAPString(OctetString):
    # LDAPString ::= OCTET STRING -- UTF-8 encoded, -- [ISO10646] characters
    encoding = 'utf-8'


class MessageID(Integer0ToMax):
    # MessageID ::= INTEGER (0 ..  maxInt)
    pass


class LDAPOID(OctetString):
    # LDAPOID ::= OCTET STRING -- Constrained to <numericoid>
    #                             -- [RFC4512]

    # subtypeSpec = numericOIDConstraint
    pass


class LDAPDN(LDAPString):
    # LDAPDN ::= LDAPString -- Constrained to <distinguishedName>
    #                         -- [RFC4514]

    # subtypeSpec = distinguishedName
    pass


class RelativeLDAPDN(LDAPString):
    # RelativeLDAPDN ::= LDAPString -- Constrained to <name-component>
    #                         -- [RFC4514]

    # subtypeSpec = LDAPString.subtypeSpec + nameComponentConstraint
    pass


class AttributeDescription(LDAPString):
    # AttributeDescription ::= LDAPString -- Constrained to <attributedescription>
    #                         -- [RFC4512]

    # subtypeSpec = LDAPString.subtypeSpec + attributeDescriptionConstraint
    pass


class AttributeValue(OctetString):
    # AttributeValue ::= OCTET STRING
    encoding = 'utf-8'


class AssertionValue(OctetString):
    # AssertionValue ::= OCTET STRING
    encoding = 'utf-8'


class AttributeValueAssertion(Sequence):
    # AttributeValueAssertion ::= SEQUENCE {
    #     attributeDesc   AttributeDescription,
    #     assertionValue  AssertionValue }
    componentType = NamedTypes(NamedType('attributeDesc', AttributeDescription()),
                               NamedType('assertionValue', AssertionValue()))


class MatchingRuleId(LDAPString):
    # MatchingRuleId ::= LDAPString
    pass


class Vals(SetOf):
    # vals       SET OF value AttributeValue }
    componentType = AttributeValue()


class ValsAtLeast1(SetOf):
    # vals       SET OF value AttributeValue }
    componentType = AttributeValue()
    subtypeSpec = SetOf.subtypeSpec + size1ToMaxConstraint


class PartialAttribute(Sequence):
    # PartialAttribute ::= SEQUENCE {
    #     type       AttributeDescription,
    #     vals       SET OF value AttributeValue }
    componentType = NamedTypes(NamedType('type', AttributeDescription()),
                               NamedType('vals', Vals()))


class Attribute(Sequence):
    # Attribute ::= PartialAttribute(WITH COMPONENTS {
    #     ...,
    #     vals (SIZE(1..MAX))})
    componentType = NamedTypes(NamedType('type', AttributeDescription()),
                               # NamedType('vals', ValsAtLeast1()))
                               NamedType('vals', Vals()))  # changed from ValsAtLeast1() to allow empty member values in groups - this should not be as per rfc4511 4.1.7, but openldap accept it


class AttributeList(SequenceOf):
    # AttributeList ::= SEQUENCE OF attribute Attribute
    componentType = Attribute()


class Simple(OctetString):
    # simple                  [0] OCTET STRING,
    tagSet = OctetString.tagSet.tagImplicitly(Tag(tagClassContext, tagFormatSimple, 0))
    encoding = 'utf-8'


class Credentials(OctetString):
    # credentials             OCTET STRING
    encoding = 'utf-8'


class SaslCredentials(Sequence):
    # SaslCredentials ::= SEQUENCE {
    #     mechanism               LDAPString,
    #     credentials             OCTET STRING OPTIONAL }
    tagSet = Sequence.tagSet.tagImplicitly(Tag(tagClassContext, tagFormatConstructed, 3))
    componentType = NamedTypes(NamedType('mechanism', LDAPString()),
                               OptionalNamedType('credentials', Credentials()))


# not in RFC4511 but used by Microsoft to embed the NTLM protocol in the BindRequest (Sicily Protocol)
class SicilyPackageDiscovery(OctetString):
    # sicilyPackageDiscovery  [9] OCTET STRING,
    tagSet = OctetString.tagSet.tagImplicitly(Tag(tagClassContext, tagFormatSimple, 9))
    encoding = 'utf-8'


# not in RFC4511 but used by Microsoft to embed the NTLM protocol in the BindRequest (Sicily Protocol)
class SicilyNegotiate(OctetString):
    # sicilyNegotiate  [10] OCTET STRING,
    tagSet = OctetString.tagSet.tagImplicitly(Tag(tagClassContext, tagFormatSimple, 10))
    encoding = 'utf-8'


# not in RFC4511 but used by Microsoft to embed the NTLM protocol in the BindRequest (Sicily Protocol)
class SicilyResponse(OctetString):
    # sicilyResponse  [11] OCTET STRING,
    tagSet = OctetString.tagSet.tagImplicitly(Tag(tagClassContext, tagFormatSimple, 11))
    encoding = 'utf-8'


class AuthenticationChoice(Choice):
    # AuthenticationChoice ::= CHOICE {
    #     simple                  [0] OCTET STRING,
    #                             -- 1 and 2 reserved
    #     sasl                    [3] SaslCredentials,
    # ... }

    # from https://msdn.microsoft.com/en-us/library/cc223498.aspx  # legacy NTLM authentication for Windows Active Directory
    # sicilyPackageDiscovery [9]    OCTET STRING
    # sicilyNegotiate        [10]   OCTET STRING
    # sicilyResponse         [11]   OCTET STRING  }

    componentType = NamedTypes(NamedType('simple', Simple()),
                               NamedType('sasl', SaslCredentials()),
                               NamedType('sicilyPackageDiscovery', SicilyPackageDiscovery()),
                               NamedType('sicilyNegotiate', SicilyNegotiate()),
                               NamedType('sicilyResponse', SicilyResponse()),
                               )


class Version(Integer):
    # version                 INTEGER (1 ..  127),
    subtypeSpec = Integer.subtypeSpec + rangeInt1To127Constraint


class ResultCode(Enumerated):
    # resultCode         ENUMERATED {
    #     success                      (0),
    #     operationsError              (1),
    #     protocolError                (2),
    #     timeLimitExceeded            (3),
    #     sizeLimitExceeded            (4),
    #     compareFalse                 (5),
    #     compareTrue                  (6),
    #     authMethodNotSupported       (7),
    #     strongerAuthRequired         (8),
    #          -- 9 reserved --
    #     referral                     (10),
    #     adminLimitExceeded           (11),
    #     unavailableCriticalExtension (12),
    #     confidentialityRequired      (13),
    #     saslBindInProgress           (14),
    #     noSuchAttribute              (16),
    #     undefinedAttributeType       (17),
    #     inappropriateMatching        (18),
    #     constraintViolation          (19),
    #     attributeOrValueExists       (20),
    #     invalidAttributeSyntax       (21),
    #          -- 22-31 unused --
    #     noSuchObject                 (32),
    #     aliasProblem                 (33),
    #     invalidDNSyntax              (34),
    #          -- 35 reserved for undefined isLeaf --
    #     aliasDereferencingProblem    (36),
    #          -- 37-47 unused --
    #     inappropriateAuthentication  (48),
    #     invalidCredentials           (49),
    #     insufficientAccessRights     (50),
    #     busy                         (51),
    #     unavailable                  (52),
    #     unwillingToPerform           (53),
    #     loopDetect                   (54),
    #          -- 55-63 unused --
    #     namingViolation              (64),
    #     objectClassViolation         (65),
    #     notAllowedOnNonLeaf          (66),
    #     notAllowedOnRDN              (67),
    #     entryAlreadyExists           (68),
    #     objectClassModsProhibited    (69),
    #          -- 70 reserved for CLDAP --
    #     affectsMultipleDSAs          (71),
    #          -- 72-79 unused --
    #     other                        (80),
    #     ...  }
    #
    #     from IANA ldap-parameters:
    #     lcupResourcesExhausted        113        IESG                             [RFC3928]
    #     lcupSecurityViolation         114        IESG                             [RFC3928]
    #     lcupInvalidData               115        IESG                             [RFC3928]
    #     lcupUnsupportedScheme         116        IESG                             [RFC3928]
    #     lcupReloadRequired            117        IESG                             [RFC3928]
    #     canceled                      118        IESG                             [RFC3909]
    #     noSuchOperation               119        IESG                             [RFC3909]
    #     tooLate                       120        IESG                             [RFC3909]
    #     cannotCancel                  121        IESG                             [RFC3909]
    #     assertionFailed               122        IESG                             [RFC4528]
    #     authorizationDenied           123        WELTMAN                          [RFC4370]
    #     e-syncRefreshRequired         4096       [Kurt_Zeilenga] [Jong_Hyuk_Choi] [RFC4533]
    namedValues = NamedValues(('success', 0),
                              ('operationsError', 1),
                              ('protocolError', 2),
                              ('timeLimitExceeded', 3),
                              ('sizeLimitExceeded', 4),
                              ('compareFalse', 5),
                              ('compareTrue', 6),
                              ('authMethodNotSupported', 7),
                              ('strongerAuthRequired', 8),
                              ('referral', 10),
                              ('adminLimitExceeded', 11),
                              ('unavailableCriticalExtension', 12),
                              ('confidentialityRequired', 13),
                              ('saslBindInProgress', 14),
                              ('noSuchAttribute', 16),
                              ('undefinedAttributeType', 17),
                              ('inappropriateMatching', 18),
                              ('constraintViolation', 19),
                              ('attributeOrValueExists', 20),
                              ('invalidAttributeSyntax', 21),
                              ('noSuchObject', 32),
                              ('aliasProblem', 33),
                              ('invalidDNSyntax', 34),
                              ('aliasDereferencingProblem', 36),
                              ('inappropriateAuthentication', 48),
                              ('invalidCredentials', 49),
                              ('insufficientAccessRights', 50),
                              ('busy', 51),
                              ('unavailable', 52),
                              ('unwillingToPerform', 53),
                              ('loopDetected', 54),
                              ('namingViolation', 64),
                              ('objectClassViolation', 65),
                              ('notAllowedOnNonLeaf', 66),
                              ('notAllowedOnRDN', 67),
                              ('entryAlreadyExists', 68),
                              ('objectClassModsProhibited', 69),
                              ('affectMultipleDSAs', 71),
                              ('other', 80),
                              ('lcupResourcesExhausted', 113),
                              ('lcupSecurityViolation', 114),
                              ('lcupInvalidData', 115),
                              ('lcupUnsupportedScheme', 116),
                              ('lcupReloadRequired', 117),
                              ('canceled', 118),
                              ('noSuchOperation', 119),
                              ('tooLate', 120),
                              ('cannotCancel', 121),
                              ('assertionFailed', 122),
                              ('authorizationDenied', 123),
                              ('e-syncRefreshRequired', 4096))

    subTypeSpec = Enumerated.subtypeSpec + responseValueConstraint


class URI(LDAPString):
    # URI ::= LDAPString     -- limited to characters permitted in
    #                      -- URIs

    # subtypeSpec = LDAPString.subTypeSpec + uriConstrain
    pass


class Referral(SequenceOf):
    # Referral ::= SEQUENCE SIZE (1..MAX) OF uri URI
    tagSet = SequenceOf.tagSet.tagImplicitly(Tag(tagClassContext, tagFormatConstructed, 3))
    componentType = URI()


class ServerSaslCreds(OctetString):
    # serverSaslCreds    [7] OCTET STRING OPTIONAL
    tagSet = OctetString.tagSet.tagImplicitly(Tag(tagClassContext, tagFormatSimple, 7))
    encoding = 'utf-8'


class LDAPResult(Sequence):
    # LDAPResult ::= SEQUENCE {
    #     resultCode         ENUMERATED {
    #         success                      (0),
    #         operationsError              (1),
    #         protocolError                (2),
    #         timeLimitExceeded            (3),
    #         sizeLimitExceeded            (4),
    #         compareFalse                 (5),
    #         compareTrue                  (6),
    #         authMethodNotSupported       (7),
    #         strongerAuthRequired         (8),
    #              -- 9 reserved --
    #         referral                     (10),
    #         adminLimitExceeded           (11),
    #         unavailableCriticalExtension (12),
    #         confidentialityRequired      (13),
    #         saslBindInProgress           (14),
    #         noSuchAttribute              (16),
    #         undefinedAttributeType       (17),
    #         inappropriateMatching        (18),
    #         constraintViolation          (19),
    #         attributeOrValueExists       (20),
    #         invalidAttributeSyntax       (21),
    #              -- 22-31 unused --
    #         noSuchObject                 (32),
    #         aliasProblem                 (33),
    #         invalidDNSyntax              (34),
    #              -- 35 reserved for undefined isLeaf --
    #         aliasDereferencingProblem    (36),
    #              -- 37-47 unused --
    #         inappropriateAuthentication  (48),
    #         invalidCredentials           (49),
    #         insufficientAccessRights     (50),
    #         busy                         (51),
    #         unavailable                  (52),
    #         unwillingToPerform           (53),
    #         loopDetect                   (54),
    #              -- 55-63 unused --
    #         namingViolation              (64),
    #         objectClassViolation         (65),
    #         notAllowedOnNonLeaf          (66),
    #         notAllowedOnRDN              (67),
    #         entryAlreadyExists           (68),
    #         objectClassModsProhibited    (69),
    #              -- 70 reserved for CLDAP --
    #         affectsMultipleDSAs          (71),
    #              -- 72-79 unused --
    #         other                        (80),
    #         ...  },
    #     matchedDN          LDAPDN,
    #     diagnosticMessage  LDAPString,
    #     referral           [3] Referral OPTIONAL }
    componentType = NamedTypes(NamedType('resultCode', ResultCode()),
                               NamedType('matchedDN', LDAPDN()),
                               NamedType('diagnosticMessage', LDAPString()),
                               OptionalNamedType('referral', Referral()))


class Criticality(Boolean):
    # criticality             BOOLEAN DEFAULT FALSE
    defaultValue = False


class ControlValue(OctetString):
    # controlValue            OCTET STRING
    encoding = 'utf-8'


class Control(Sequence):
    # Control ::= SEQUENCE {
    #     controlType             LDAPOID,
    #     criticality             BOOLEAN DEFAULT FALSE,
    #     controlValue            OCTET STRING OPTIONAL }
    componentType = NamedTypes(NamedType('controlType', LDAPOID()),
                               DefaultedNamedType('criticality', Criticality()),
                               OptionalNamedType('controlValue', ControlValue()))


class Controls(SequenceOf):
    # Controls ::= SEQUENCE OF control Control
    tagSet = SequenceOf.tagSet.tagImplicitly(Tag(tagClassContext, tagFormatConstructed, 0))
    componentType = Control()


class Scope(Enumerated):
    # scope           ENUMERATED {
    #     baseObject              (0),
    #     singleLevel             (1),
    #     wholeSubtree            (2),
    namedValues = NamedValues(('baseObject', 0),
                              ('singleLevel', 1),
                              ('wholeSubtree', 2))


class DerefAliases(Enumerated):
    # derefAliases    ENUMERATED {
    #     neverDerefAliases       (0),
    #     derefInSearching        (1),
    #     derefFindingBaseObj     (2),
    #     derefAlways             (3) },
    namedValues = NamedValues(('neverDerefAliases', 0),
                              ('derefInSearching', 1),
                              ('derefFindingBaseObj', 2),
                              ('derefAlways', 3))


class TypesOnly(Boolean):
    # typesOnly       BOOLEAN
    pass


class Selector(LDAPString):
    #     -- The LDAPString is constrained to
    #     -- <attributeSelector> in Section 4.5.1.8

    # subtypeSpec = LDAPString.subtypeSpec + attributeSelectorConstraint
    pass


class AttributeSelection(SequenceOf):
    # AttributeSelection ::= SEQUENCE OF selector LDAPString
    #     -- The LDAPString is constrained to
    #     -- <attributeSelector> in Section 4.5.1.8
    componentType = Selector()


class MatchingRule(MatchingRuleId):
    # matchingRule    [1] MatchingRuleId
    tagSet = MatchingRuleId.tagSet.tagImplicitly(Tag(tagClassContext, tagFormatSimple, 1))


class Type(AttributeDescription):
    # type            [2] AttributeDescription
    tagSet = AttributeDescription.tagSet.tagImplicitly(Tag(tagClassContext, tagFormatSimple, 2))


class MatchValue(AssertionValue):
    # matchValue      [3] AssertionValue,
    tagSet = AssertionValue.tagSet.tagImplicitly(Tag(tagClassContext, tagFormatSimple, 3))


class DnAttributes(Boolean):
    # dnAttributes    [4] BOOLEAN DEFAULT FALSE }
    tagSet = Boolean.tagSet.tagImplicitly(Tag(tagClassContext, tagFormatSimple, 4))
    defaultValue = Boolean(False)


class MatchingRuleAssertion(Sequence):
    # MatchingRuleAssertion ::= SEQUENCE {
    #     matchingRule    [1] MatchingRuleId OPTIONAL,
    #     type            [2] AttributeDescription OPTIONAL,
    #     matchValue      [3] AssertionValue,
    #     dnAttributes    [4] BOOLEAN DEFAULT FALSE }
    componentType = NamedTypes(OptionalNamedType('matchingRule', MatchingRule()),
                               OptionalNamedType('type', Type()),
                               NamedType('matchValue', MatchValue()),
                               DefaultedNamedType('dnAttributes', DnAttributes()))


class Initial(AssertionValue):
    # initial [0] AssertionValue,  -- can occur at most once
    tagSet = AssertionValue.tagSet.tagImplicitly(Tag(tagClassContext, tagFormatSimple, 0))


class Any(AssertionValue):
    # any [1] AssertionValue,
    tagSet = AssertionValue.tagSet.tagImplicitly(Tag(tagClassContext, tagFormatSimple, 1))


class Final(AssertionValue):
    # final [1] AssertionValue,  -- can occur at most once
    tagSet = AssertionValue.tagSet.tagImplicitly(Tag(tagClassContext, tagFormatSimple, 2))


class Substring(Choice):
    # substring CHOICE {
    #     initial [0] AssertionValue,  -- can occur at most once
    #     any     [1] AssertionValue,
    #     final   [2] AssertionValue } -- can occur at most once
    #     }
    componentType = NamedTypes(NamedType('initial', Initial()),
                               NamedType('any', Any()),
                               NamedType('final', Final()))


class Substrings(SequenceOf):
    # substrings     SEQUENCE SIZE (1..MAX) OF substring CHOICE {
    # ...
    # }
    subtypeSpec = SequenceOf.subtypeSpec + size1ToMaxConstraint
    componentType = Substring()


class SubstringFilter(Sequence):
    #     SubstringFilter ::= SEQUENCE {
    #         type           AttributeDescription,
    #         substrings     SEQUENCE SIZE (1..MAX) OF substring CHOICE {
    #             initial [0] AssertionValue,  -- can occur at most once
    #             any     [1] AssertionValue,
    #             final   [2] AssertionValue } -- can occur at most once
    #             }
    tagSet = Sequence.tagSet.tagImplicitly(Tag(tagClassContext, tagFormatConstructed, 4))
    componentType = NamedTypes(NamedType('type', AttributeDescription()),
                               NamedType('substrings', Substrings()))


class And(SetOf):
    # and             [0] SET SIZE (1..MAX) OF filter Filter
    tagSet = SetOf.tagSet.tagImplicitly(Tag(tagClassContext, tagFormatConstructed, 0))
    subtypeSpec = SetOf.subtypeSpec + size1ToMaxConstraint


class Or(SetOf):
    # or              [1] SET SIZE (1..MAX) OF filter Filter
    tagSet = SetOf.tagSet.tagImplicitly(Tag(tagClassContext, tagFormatConstructed, 1))
    subtypeSpec = SetOf.subtypeSpec + size1ToMaxConstraint


class Not(Choice):
    # not             [2] Filter
    pass  # defined after Filter definition to allow recursion


class EqualityMatch(AttributeValueAssertion):
    # equalityMatch   [3] AttributeValueAssertion
    tagSet = AttributeValueAssertion.tagSet.tagImplicitly(Tag(tagClassContext, tagFormatConstructed, 3))


class GreaterOrEqual(AttributeValueAssertion):
    # greaterOrEqual  [5] AttributeValueAssertion
    tagSet = AttributeValueAssertion.tagSet.tagImplicitly(Tag(tagClassContext, tagFormatConstructed, 5))


class LessOrEqual(AttributeValueAssertion):
    # lessOrEqual     [6] AttributeValueAssertion
    tagSet = AttributeValueAssertion.tagSet.tagImplicitly(Tag(tagClassContext, tagFormatConstructed, 6))


class Present(AttributeDescription):
    # present         [7] AttributeDescription
    tagSet = AttributeDescription.tagSet.tagImplicitly(Tag(tagClassContext, tagFormatConstructed, 7))


class ApproxMatch(AttributeValueAssertion):
    # approxMatch     [8] AttributeValueAssertion
    tagSet = AttributeValueAssertion.tagSet.tagImplicitly(Tag(tagClassContext, tagFormatConstructed, 8))


class ExtensibleMatch(MatchingRuleAssertion):
    # extensibleMatch [9] MatchingRuleAssertion
    tagSet = MatchingRuleAssertion.tagSet.tagImplicitly(Tag(tagClassContext, tagFormatConstructed, 9))


class Filter(Choice):
    # Filter ::= CHOICE {
    #     and             [0] SET SIZE (1..MAX) OF filter Filter,
    #     or              [1] SET SIZE (1..MAX) OF filter Filter,
    #     not             [2] Filter,
    #     equalityMatch   [3] AttributeValueAssertion,
    #     substrings      [4] SubstringFilter,
    #     greaterOrEqual  [5] AttributeValueAssertion,
    #     lessOrEqual     [6] AttributeValueAssertion,
    #     present         [7] AttributeDescription,
    #     approxMatch     [8] AttributeValueAssertion,
    #     extensibleMatch [9] MatchingRuleAssertion,
    #          ...  }
    componentType = NamedTypes(NamedType('and', And()),
                               NamedType('or', Or()),
                               NamedType('notFilter', Not()),
                               NamedType('equalityMatch', EqualityMatch()),
                               NamedType('substringFilter', SubstringFilter()),
                               NamedType('greaterOrEqual', GreaterOrEqual()),
                               NamedType('lessOrEqual', LessOrEqual()),
                               NamedType('present', Present()),
                               NamedType('approxMatch', ApproxMatch()),
                               NamedType('extensibleMatch', ExtensibleMatch()))


And.componentType = Filter()
Or.componentType = Filter()
Not.componentType = NamedTypes(NamedType('innerNotFilter', Filter()))
Not.tagSet = Filter.tagSet.tagExplicitly(Tag(tagClassContext, tagFormatConstructed, 2))  # as per RFC4511 page 23


class PartialAttributeList(SequenceOf):
    # PartialAttributeList ::= SEQUENCE OF
    #     partialAttribute PartialAttribute
    componentType = PartialAttribute()


class Operation(Enumerated):
    # operation       ENUMERATED {
    #     add     (0),
    #     delete  (1),
    #     replace (2),
    #     ...  }
    namedValues = NamedValues(('add', 0),
                              ('delete', 1),
                              ('replace', 2),
                              ('increment', 3))


class Change(Sequence):
    # change SEQUENCE {
    #     operation       ENUMERATED {
    #         add     (0),
    #         delete  (1),
    #         replace (2),
    #         ...  },
    #     modification    PartialAttribute } }
    componentType = NamedTypes(NamedType('operation', Operation()),
                               NamedType('modification', PartialAttribute()))


class Changes(SequenceOf):
    # changes         SEQUENCE OF change SEQUENCE
    componentType = Change()


class DeleteOldRDN(Boolean):
    # deleteoldrdn    BOOLEAN
    pass


class NewSuperior(LDAPDN):
    # newSuperior     [0] LDAPDN
    tagSet = LDAPDN.tagSet.tagImplicitly(Tag(tagClassContext, tagFormatSimple, 0))


class RequestName(LDAPOID):
    # requestName      [0] LDAPOID
    tagSet = LDAPOID.tagSet.tagImplicitly(Tag(tagClassContext, tagFormatSimple, 0))


class RequestValue(OctetString):
    # requestValue     [1] OCTET STRING
    tagSet = OctetString.tagSet.tagImplicitly(Tag(tagClassContext, tagFormatSimple, 1))
    encoding = 'utf-8'


class ResponseName(LDAPOID):
    # responseName      [10] LDAPOID
    tagSet = LDAPOID.tagSet.tagImplicitly(Tag(tagClassContext, tagFormatSimple, 10))


class ResponseValue(OctetString):
    # responseValue     [11] OCTET STRING
    tagSet = OctetString.tagSet.tagImplicitly(Tag(tagClassContext, tagFormatSimple, 11))
    encoding = 'utf-8'


class IntermediateResponseName(LDAPOID):
    # responseName      [0] LDAPOID
    tagSet = LDAPOID.tagSet.tagImplicitly(Tag(tagClassContext, tagFormatSimple, 0))


class IntermediateResponseValue(OctetString):
    # responseValue     [1] OCTET STRING
    tagSet = OctetString.tagSet.tagImplicitly(Tag(tagClassContext, tagFormatSimple, 1))
    encoding = 'utf-8'


# operations
class BindRequest(Sequence):
    # BindRequest ::= [APPLICATION 0] SEQUENCE {
    #     version                 INTEGER (1 ..  127),
    #     name                    LDAPDN,
    #     authentication          AuthenticationChoice }
    tagSet = Sequence.tagSet.tagImplicitly(Tag(tagClassApplication, tagFormatConstructed, 0))
    componentType = NamedTypes(NamedType('version', Version()),
                               NamedType('name', LDAPDN()),
                               NamedType('authentication', AuthenticationChoice()))


class BindResponse(Sequence):
    # BindResponse ::= [APPLICATION 1] SEQUENCE {
    #     COMPONENTS OF LDAPResult,
    #     serverSaslCreds    [7] OCTET STRING OPTIONAL }
    tagSet = Sequence.tagSet.tagImplicitly(Tag(tagClassApplication, tagFormatConstructed, 1))
    componentType = NamedTypes(NamedType('resultCode', ResultCode()),
                               NamedType('matchedDN', LDAPDN()),
                               NamedType('diagnosticMessage', LDAPString()),
                               OptionalNamedType('referral', Referral()),
                               OptionalNamedType('serverSaslCreds', ServerSaslCreds()))


class UnbindRequest(Null):
    # UnbindRequest ::= [APPLICATION 2] NULL
    tagSet = Null.tagSet.tagImplicitly(Tag(tagClassApplication, tagFormatSimple, 2))


class SearchRequest(Sequence):
    # SearchRequest ::= [APPLICATION 3] SEQUENCE {
    #     baseObject      LDAPDN,
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
    tagSet = Sequence.tagSet.tagImplicitly(Tag(tagClassApplication, tagFormatConstructed, 3))
    componentType = NamedTypes(NamedType('baseObject', LDAPDN()),
                               NamedType('scope', Scope()),
                               NamedType('derefAliases', DerefAliases()),
                               NamedType('sizeLimit', Integer0ToMax()),
                               NamedType('timeLimit', Integer0ToMax()),
                               NamedType('typesOnly', TypesOnly()),
                               NamedType('filter', Filter()),
                               NamedType('attributes', AttributeSelection()))


class SearchResultReference(SequenceOf):
    # SearchResultReference ::= [APPLICATION 19] SEQUENCE
    #     SIZE (1..MAX) OF uri URI
    tagSet = SequenceOf.tagSet.tagImplicitly(Tag(tagClassApplication, tagFormatConstructed, 19))
    subtypeSpec = SequenceOf.subtypeSpec + size1ToMaxConstraint
    componentType = URI()


class SearchResultEntry(Sequence):
    # SearchResultEntry ::= [APPLICATION 4] SEQUENCE {
    #     objectName      LDAPDN,
    #     attributes      PartialAttributeList }
    tagSet = Sequence.tagSet.tagImplicitly(Tag(tagClassApplication, tagFormatConstructed, 4))
    componentType = NamedTypes(NamedType('object', LDAPDN()),
                               NamedType('attributes', PartialAttributeList()))


class SearchResultDone(LDAPResult):
    # SearchResultDone ::= [APPLICATION 5] LDAPResult
    tagSet = LDAPResult.tagSet.tagImplicitly(Tag(tagClassApplication, tagFormatConstructed, 5))


class ModifyRequest(Sequence):
    # ModifyRequest ::= [APPLICATION 6] SEQUENCE {
    #     object          LDAPDN,
    #     changes         SEQUENCE OF change SEQUENCE {
    #         operation       ENUMERATED {
    #             add     (0),
    #             delete  (1),
    #             replace (2),
    #             ...  },
    #         modification    PartialAttribute } }
    tagSet = Sequence.tagSet.tagImplicitly(Tag(tagClassApplication, tagFormatConstructed, 6))
    componentType = NamedTypes(NamedType('object', LDAPDN()),
                               NamedType('changes', Changes()))


class ModifyResponse(LDAPResult):
    # ModifyResponse ::= [APPLICATION 7] LDAPResult
    tagSet = LDAPResult.tagSet.tagImplicitly(Tag(tagClassApplication, tagFormatConstructed, 7))


class AddRequest(Sequence):
    # AddRequest ::= [APPLICATION 8] SEQUENCE {
    #     entry           LDAPDN,
    #     attributes      AttributeList }
    tagSet = Sequence.tagSet.tagImplicitly(Tag(tagClassApplication, tagFormatConstructed, 8))
    componentType = NamedTypes(NamedType('entry', LDAPDN()),
                               NamedType('attributes', AttributeList()))


class AddResponse(LDAPResult):
    # AddResponse ::= [APPLICATION 9] LDAPResult
    tagSet = LDAPResult.tagSet.tagImplicitly(Tag(tagClassApplication, tagFormatConstructed, 9))


class DelRequest(LDAPDN):
    # DelRequest ::= [APPLICATION 10] LDAPDN
    tagSet = LDAPDN.tagSet.tagImplicitly(Tag(tagClassApplication, tagFormatSimple, 10))


class DelResponse(LDAPResult):
    # DelResponse ::= [APPLICATION 11] LDAPResult
    tagSet = LDAPResult.tagSet.tagImplicitly(Tag(tagClassApplication, tagFormatConstructed, 11))


class ModifyDNRequest(Sequence):
    # ModifyDNRequest ::= [APPLICATION 12] SEQUENCE {
    #     entry           LDAPDN,
    #     newrdn          RelativeLDAPDN,
    #     deleteoldrdn    BOOLEAN,
    #     newSuperior     [0] LDAPDN OPTIONAL }
    tagSet = Sequence.tagSet.tagImplicitly(Tag(tagClassApplication, tagFormatConstructed, 12))
    componentType = NamedTypes(NamedType('entry', LDAPDN()),
                               NamedType('newrdn', RelativeLDAPDN()),
                               NamedType('deleteoldrdn', DeleteOldRDN()),
                               OptionalNamedType('newSuperior', NewSuperior()))


class ModifyDNResponse(LDAPResult):
    # ModifyDNResponse ::= [APPLICATION 13] LDAPResult
    tagSet = LDAPResult.tagSet.tagImplicitly(Tag(tagClassApplication, tagFormatConstructed, 13))


class CompareRequest(Sequence):
    # CompareRequest ::= [APPLICATION 14] SEQUENCE {
    #     entry           LDAPDN,
    #     ava             AttributeValueAssertion }
    tagSet = Sequence.tagSet.tagImplicitly(Tag(tagClassApplication, tagFormatConstructed, 14))
    componentType = NamedTypes(NamedType('entry', LDAPDN()),
                               NamedType('ava', AttributeValueAssertion()))


class CompareResponse(LDAPResult):
    # CompareResponse ::= [APPLICATION 15] LDAPResult
    tagSet = LDAPResult.tagSet.tagImplicitly(Tag(tagClassApplication, tagFormatConstructed, 15))


class AbandonRequest(MessageID):
    # AbandonRequest ::= [APPLICATION 16] MessageID
    tagSet = MessageID.tagSet.tagImplicitly(Tag(tagClassApplication, tagFormatSimple, 16))


class ExtendedRequest(Sequence):
    # ExtendedRequest ::= [APPLICATION 23] SEQUENCE {
    #     requestName      [0] LDAPOID,
    #     requestValue     [1] OCTET STRING OPTIONAL }
    tagSet = Sequence.tagSet.tagImplicitly(Tag(tagClassApplication, tagFormatConstructed, 23))
    componentType = NamedTypes(NamedType('requestName', RequestName()),
                               OptionalNamedType('requestValue', RequestValue()))


class ExtendedResponse(Sequence):
    # ExtendedResponse ::= [APPLICATION 24] SEQUENCE {
    #     COMPONENTS OF LDAPResult,
    #     responseName     [10] LDAPOID OPTIONAL,
    #     responseValue    [11] OCTET STRING OPTIONAL }
    tagSet = Sequence.tagSet.tagImplicitly(Tag(tagClassApplication, tagFormatConstructed, 24))
    componentType = NamedTypes(NamedType('resultCode', ResultCode()),
                               NamedType('matchedDN', LDAPDN()),
                               NamedType('diagnosticMessage', LDAPString()),
                               OptionalNamedType('referral', Referral()),
                               OptionalNamedType('responseName', ResponseName()),
                               OptionalNamedType('responseValue', ResponseValue()))


class IntermediateResponse(Sequence):
    # IntermediateResponse ::= [APPLICATION 25] SEQUENCE {
    #     responseName     [0] LDAPOID OPTIONAL,
    #     responseValue    [1] OCTET STRING OPTIONAL }
    tagSet = Sequence.tagSet.tagImplicitly(Tag(tagClassApplication, tagFormatConstructed, 25))
    componentType = NamedTypes(OptionalNamedType('responseName', IntermediateResponseName()),
                               OptionalNamedType('responseValue', IntermediateResponseValue()))


class ProtocolOp(Choice):
    # protocolOp      CHOICE {
    #     bindRequest           BindRequest,
    #     bindResponse          BindResponse,
    #     unbindRequest         UnbindRequest,
    #     searchRequest         SearchRequest,
    #     searchResEntry        SearchResultEntry,
    #     searchResDone         SearchResultDone,
    #     searchResRef          SearchResultReference,
    #     modifyRequest         ModifyRequest,
    #     modifyResponse        ModifyResponse,
    #     addRequest            AddRequest,
    #     addResponse           AddResponse,
    #     delRequest            DelRequest,
    #     delResponse           DelResponse,
    #     modDNRequest          ModifyDNRequest,
    #     modDNResponse         ModifyDNResponse,
    #     compareRequest        CompareRequest,
    #     compareResponse       CompareResponse,
    #     abandonRequest        AbandonRequest,
    #     extendedReq           ExtendedRequest,
    #     extendedResp          ExtendedResponse,
    #     ...,
    #     intermediateResponse  IntermediateResponse }
    componentType = NamedTypes(NamedType('bindRequest', BindRequest()),
                               NamedType('bindResponse', BindResponse()),
                               NamedType('unbindRequest', UnbindRequest()),
                               NamedType('searchRequest', SearchRequest()),
                               NamedType('searchResEntry', SearchResultEntry()),
                               NamedType('searchResDone', SearchResultDone()),
                               NamedType('searchResRef', SearchResultReference()),
                               NamedType('modifyRequest', ModifyRequest()),
                               NamedType('modifyResponse', ModifyResponse()),
                               NamedType('addRequest', AddRequest()),
                               NamedType('addResponse', AddResponse()),
                               NamedType('delRequest', DelRequest()),
                               NamedType('delResponse', DelResponse()),
                               NamedType('modDNRequest', ModifyDNRequest()),
                               NamedType('modDNResponse', ModifyDNResponse()),
                               NamedType('compareRequest', CompareRequest()),
                               NamedType('compareResponse', CompareResponse()),
                               NamedType('abandonRequest', AbandonRequest()),
                               NamedType('extendedReq', ExtendedRequest()),
                               NamedType('extendedResp', ExtendedResponse()),
                               NamedType('intermediateResponse', IntermediateResponse()))


class LDAPMessage(Sequence):
    # LDAPMessage ::= SEQUENCE {
    #     messageID       MessageID,
    #     protocolOp      CHOICE {
    #         bindRequest           BindRequest,
    #         bindResponse          BindResponse,
    #         unbindRequest         UnbindRequest,
    #         searchRequest         SearchRequest,
    #         searchResEntry        SearchResultEntry,
    #         searchResDone         SearchResultDone,
    #         searchResRef          SearchResultReference,
    #         modifyRequest         ModifyRequest,
    #         modifyResponse        ModifyResponse,
    #         addRequest            AddRequest,
    #         addResponse           AddResponse,
    #         delRequest            DelRequest,
    #         delResponse           DelResponse,
    #         modDNRequest          ModifyDNRequest,
    #         modDNResponse         ModifyDNResponse,
    #         compareRequest        CompareRequest,
    #         compareResponse       CompareResponse,
    #         abandonRequest        AbandonRequest,
    #         extendedReq           ExtendedRequest,
    #         extendedResp          ExtendedResponse,
    #         ...,
    #         intermediateResponse  IntermediateResponse },
    #     controls       [0] Controls OPTIONAL }
    componentType = NamedTypes(NamedType('messageID', MessageID()),
                               NamedType('protocolOp', ProtocolOp()),
                               OptionalNamedType('controls', Controls()))
