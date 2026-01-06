"""
"""

# Created on 2014.05.14
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

from os import sep
from .results import RESULT_OPERATIONS_ERROR, RESULT_PROTOCOL_ERROR, RESULT_TIME_LIMIT_EXCEEDED, RESULT_SIZE_LIMIT_EXCEEDED, \
    RESULT_STRONGER_AUTH_REQUIRED, RESULT_REFERRAL, RESULT_ADMIN_LIMIT_EXCEEDED, RESULT_UNAVAILABLE_CRITICAL_EXTENSION, \
    RESULT_AUTH_METHOD_NOT_SUPPORTED, RESULT_UNDEFINED_ATTRIBUTE_TYPE, RESULT_NO_SUCH_ATTRIBUTE, \
    RESULT_SASL_BIND_IN_PROGRESS, RESULT_CONFIDENTIALITY_REQUIRED, RESULT_INAPPROPRIATE_MATCHING, \
    RESULT_CONSTRAINT_VIOLATION, \
    RESULT_ATTRIBUTE_OR_VALUE_EXISTS, RESULT_INVALID_ATTRIBUTE_SYNTAX, RESULT_NO_SUCH_OBJECT, RESULT_ALIAS_PROBLEM, \
    RESULT_INVALID_DN_SYNTAX, RESULT_ALIAS_DEREFERENCING_PROBLEM, RESULT_INVALID_CREDENTIALS, RESULT_LOOP_DETECTED, \
    RESULT_ENTRY_ALREADY_EXISTS, RESULT_LCUP_SECURITY_VIOLATION, RESULT_CANCELED, RESULT_E_SYNC_REFRESH_REQUIRED, \
    RESULT_NO_SUCH_OPERATION, RESULT_LCUP_INVALID_DATA, RESULT_OBJECT_CLASS_MODS_PROHIBITED, RESULT_NAMING_VIOLATION, \
    RESULT_INSUFFICIENT_ACCESS_RIGHTS, RESULT_OBJECT_CLASS_VIOLATION, RESULT_TOO_LATE, RESULT_CANNOT_CANCEL, \
    RESULT_LCUP_UNSUPPORTED_SCHEME, RESULT_BUSY, RESULT_AFFECT_MULTIPLE_DSAS, RESULT_UNAVAILABLE, \
    RESULT_NOT_ALLOWED_ON_NON_LEAF, \
    RESULT_UNWILLING_TO_PERFORM, RESULT_OTHER, RESULT_LCUP_RELOAD_REQUIRED, RESULT_ASSERTION_FAILED, \
    RESULT_AUTHORIZATION_DENIED, RESULT_LCUP_RESOURCES_EXHAUSTED, RESULT_NOT_ALLOWED_ON_RDN, \
    RESULT_INAPPROPRIATE_AUTHENTICATION
import socket


# LDAPException hierarchy
class LDAPException(Exception):
    pass


class LDAPOperationResult(LDAPException):
    def __new__(cls, result=None, description=None, dn=None, message=None, response_type=None, response=None):
        if cls is LDAPOperationResult and result and result in exception_table:
            exc = super(LDAPOperationResult, exception_table[result]).__new__(
                exception_table[result])  # create an exception of the required result error
            exc.result = result
            exc.description = description
            exc.dn = dn
            exc.message = message
            exc.type = response_type
            exc.response = response
        else:
            exc = super(LDAPOperationResult, cls).__new__(cls)
        return exc

    def __init__(self, result=None, description=None, dn=None, message=None, response_type=None, response=None):
        self.result = result
        self.description = description
        self.dn = dn
        self.message = message
        self.type = response_type
        self.response = response

    def __str__(self):
        s = [self.__class__.__name__,
             str(self.result) if self.result else None,
             self.description if self.description else None,
             self.dn if self.dn else None,
             self.message if self.message else None,
             self.type if self.type else None,
             self.response if self.response else None]

        return ' - '.join([str(item) for item in s if s is not None])

    def __repr__(self):
        return self.__str__()


class LDAPOperationsErrorResult(LDAPOperationResult):
    pass


class LDAPProtocolErrorResult(LDAPOperationResult):
    pass


class LDAPTimeLimitExceededResult(LDAPOperationResult):
    pass


class LDAPSizeLimitExceededResult(LDAPOperationResult):
    pass


class LDAPAuthMethodNotSupportedResult(LDAPOperationResult):
    pass


class LDAPStrongerAuthRequiredResult(LDAPOperationResult):
    pass


class LDAPReferralResult(LDAPOperationResult):
    pass


class LDAPAdminLimitExceededResult(LDAPOperationResult):
    pass


class LDAPUnavailableCriticalExtensionResult(LDAPOperationResult):
    pass


class LDAPConfidentialityRequiredResult(LDAPOperationResult):
    pass


class LDAPSASLBindInProgressResult(LDAPOperationResult):
    pass


class LDAPNoSuchAttributeResult(LDAPOperationResult):
    pass


class LDAPUndefinedAttributeTypeResult(LDAPOperationResult):
    pass


class LDAPInappropriateMatchingResult(LDAPOperationResult):
    pass


class LDAPConstraintViolationResult(LDAPOperationResult):
    pass


class LDAPAttributeOrValueExistsResult(LDAPOperationResult):
    pass


class LDAPInvalidAttributeSyntaxResult(LDAPOperationResult):
    pass


class LDAPNoSuchObjectResult(LDAPOperationResult):
    pass


class LDAPAliasProblemResult(LDAPOperationResult):
    pass


class LDAPInvalidDNSyntaxResult(LDAPOperationResult):
    pass


class LDAPAliasDereferencingProblemResult(LDAPOperationResult):
    pass


class LDAPInappropriateAuthenticationResult(LDAPOperationResult):
    pass


class LDAPInvalidCredentialsResult(LDAPOperationResult):
    pass


class LDAPInsufficientAccessRightsResult(LDAPOperationResult):
    pass


class LDAPBusyResult(LDAPOperationResult):
    pass


class LDAPUnavailableResult(LDAPOperationResult):
    pass


class LDAPUnwillingToPerformResult(LDAPOperationResult):
    pass


class LDAPLoopDetectedResult(LDAPOperationResult):
    pass


class LDAPNamingViolationResult(LDAPOperationResult):
    pass


class LDAPObjectClassViolationResult(LDAPOperationResult):
    pass


class LDAPNotAllowedOnNotLeafResult(LDAPOperationResult):
    pass


class LDAPNotAllowedOnRDNResult(LDAPOperationResult):
    pass


class LDAPEntryAlreadyExistsResult(LDAPOperationResult):
    pass


class LDAPObjectClassModsProhibitedResult(LDAPOperationResult):
    pass


class LDAPAffectMultipleDSASResult(LDAPOperationResult):
    pass


class LDAPOtherResult(LDAPOperationResult):
    pass


class LDAPLCUPResourcesExhaustedResult(LDAPOperationResult):
    pass


class LDAPLCUPSecurityViolationResult(LDAPOperationResult):
    pass


class LDAPLCUPInvalidDataResult(LDAPOperationResult):
    pass


class LDAPLCUPUnsupportedSchemeResult(LDAPOperationResult):
    pass


class LDAPLCUPReloadRequiredResult(LDAPOperationResult):
    pass


class LDAPCanceledResult(LDAPOperationResult):
    pass


class LDAPNoSuchOperationResult(LDAPOperationResult):
    pass


class LDAPTooLateResult(LDAPOperationResult):
    pass


class LDAPCannotCancelResult(LDAPOperationResult):
    pass


class LDAPAssertionFailedResult(LDAPOperationResult):
    pass


class LDAPAuthorizationDeniedResult(LDAPOperationResult):
    pass


class LDAPESyncRefreshRequiredResult(LDAPOperationResult):
    pass


exception_table = {RESULT_OPERATIONS_ERROR: LDAPOperationsErrorResult,
                   RESULT_PROTOCOL_ERROR: LDAPProtocolErrorResult,
                   RESULT_TIME_LIMIT_EXCEEDED: LDAPTimeLimitExceededResult,
                   RESULT_SIZE_LIMIT_EXCEEDED: LDAPSizeLimitExceededResult,
                   RESULT_AUTH_METHOD_NOT_SUPPORTED: LDAPAuthMethodNotSupportedResult,
                   RESULT_STRONGER_AUTH_REQUIRED: LDAPStrongerAuthRequiredResult,
                   RESULT_REFERRAL: LDAPReferralResult,
                   RESULT_ADMIN_LIMIT_EXCEEDED: LDAPAdminLimitExceededResult,
                   RESULT_UNAVAILABLE_CRITICAL_EXTENSION: LDAPUnavailableCriticalExtensionResult,
                   RESULT_CONFIDENTIALITY_REQUIRED: LDAPConfidentialityRequiredResult,
                   RESULT_SASL_BIND_IN_PROGRESS: LDAPSASLBindInProgressResult,
                   RESULT_NO_SUCH_ATTRIBUTE: LDAPNoSuchAttributeResult,
                   RESULT_UNDEFINED_ATTRIBUTE_TYPE: LDAPUndefinedAttributeTypeResult,
                   RESULT_INAPPROPRIATE_MATCHING: LDAPInappropriateMatchingResult,
                   RESULT_CONSTRAINT_VIOLATION: LDAPConstraintViolationResult,
                   RESULT_ATTRIBUTE_OR_VALUE_EXISTS: LDAPAttributeOrValueExistsResult,
                   RESULT_INVALID_ATTRIBUTE_SYNTAX: LDAPInvalidAttributeSyntaxResult,
                   RESULT_NO_SUCH_OBJECT: LDAPNoSuchObjectResult,
                   RESULT_ALIAS_PROBLEM: LDAPAliasProblemResult,
                   RESULT_INVALID_DN_SYNTAX: LDAPInvalidDNSyntaxResult,
                   RESULT_ALIAS_DEREFERENCING_PROBLEM: LDAPAliasDereferencingProblemResult,
                   RESULT_INAPPROPRIATE_AUTHENTICATION: LDAPInappropriateAuthenticationResult,
                   RESULT_INVALID_CREDENTIALS: LDAPInvalidCredentialsResult,
                   RESULT_INSUFFICIENT_ACCESS_RIGHTS: LDAPInsufficientAccessRightsResult,
                   RESULT_BUSY: LDAPBusyResult,
                   RESULT_UNAVAILABLE: LDAPUnavailableResult,
                   RESULT_UNWILLING_TO_PERFORM: LDAPUnwillingToPerformResult,
                   RESULT_LOOP_DETECTED: LDAPLoopDetectedResult,
                   RESULT_NAMING_VIOLATION: LDAPNamingViolationResult,
                   RESULT_OBJECT_CLASS_VIOLATION: LDAPObjectClassViolationResult,
                   RESULT_NOT_ALLOWED_ON_NON_LEAF: LDAPNotAllowedOnNotLeafResult,
                   RESULT_NOT_ALLOWED_ON_RDN: LDAPNotAllowedOnRDNResult,
                   RESULT_ENTRY_ALREADY_EXISTS: LDAPEntryAlreadyExistsResult,
                   RESULT_OBJECT_CLASS_MODS_PROHIBITED: LDAPObjectClassModsProhibitedResult,
                   RESULT_AFFECT_MULTIPLE_DSAS: LDAPAffectMultipleDSASResult,
                   RESULT_OTHER: LDAPOtherResult,
                   RESULT_LCUP_RESOURCES_EXHAUSTED: LDAPLCUPResourcesExhaustedResult,
                   RESULT_LCUP_SECURITY_VIOLATION: LDAPLCUPSecurityViolationResult,
                   RESULT_LCUP_INVALID_DATA: LDAPLCUPInvalidDataResult,
                   RESULT_LCUP_UNSUPPORTED_SCHEME: LDAPLCUPUnsupportedSchemeResult,
                   RESULT_LCUP_RELOAD_REQUIRED: LDAPLCUPReloadRequiredResult,
                   RESULT_CANCELED: LDAPCanceledResult,
                   RESULT_NO_SUCH_OPERATION: LDAPNoSuchOperationResult,
                   RESULT_TOO_LATE: LDAPTooLateResult,
                   RESULT_CANNOT_CANCEL: LDAPCannotCancelResult,
                   RESULT_ASSERTION_FAILED: LDAPAssertionFailedResult,
                   RESULT_AUTHORIZATION_DENIED: LDAPAuthorizationDeniedResult,
                   RESULT_E_SYNC_REFRESH_REQUIRED: LDAPESyncRefreshRequiredResult}


class LDAPExceptionError(LDAPException):
    pass


# configuration exceptions
class LDAPConfigurationError(LDAPExceptionError):
    pass


class LDAPUnknownStrategyError(LDAPConfigurationError):
    pass


class LDAPUnknownAuthenticationMethodError(LDAPConfigurationError):
    pass


class LDAPSSLConfigurationError(LDAPConfigurationError):
    pass


class LDAPDefinitionError(LDAPConfigurationError):
    pass


class LDAPPackageUnavailableError(LDAPConfigurationError, ImportError):
    pass


class LDAPConfigurationParameterError(LDAPConfigurationError):
    pass


# abstract layer exceptions
class LDAPKeyError(LDAPExceptionError, KeyError, AttributeError):
    pass


class LDAPObjectError(LDAPExceptionError, ValueError):
    pass


class LDAPAttributeError(LDAPExceptionError, ValueError, TypeError):
    pass


class LDAPCursorError(LDAPExceptionError):
    pass


class LDAPCursorAttributeError(LDAPCursorError, AttributeError):
    pass


class LDAPObjectDereferenceError(LDAPExceptionError):
    pass


# security exceptions
class LDAPSSLNotSupportedError(LDAPExceptionError, ImportError):
    pass


class LDAPInvalidTlsSpecificationError(LDAPExceptionError):
    pass


class LDAPInvalidHashAlgorithmError(LDAPExceptionError, ValueError):
    pass

class LDAPSignatureVerificationFailedError(LDAPExceptionError):
    pass


# connection exceptions
class LDAPBindError(LDAPExceptionError):
    pass


class LDAPInvalidServerError(LDAPExceptionError):
    pass


class LDAPSASLMechanismNotSupportedError(LDAPExceptionError):
    pass


class LDAPConnectionIsReadOnlyError(LDAPExceptionError):
    pass


class LDAPChangeError(LDAPExceptionError, ValueError):
    pass


class LDAPServerPoolError(LDAPExceptionError):
    pass


class LDAPServerPoolExhaustedError(LDAPExceptionError):
    pass


class LDAPInvalidPortError(LDAPExceptionError):
    pass


class LDAPStartTLSError(LDAPExceptionError):
    pass


class LDAPCertificateError(LDAPExceptionError):
    pass


class LDAPUserNameNotAllowedError(LDAPExceptionError):
    pass


class LDAPUserNameIsMandatoryError(LDAPExceptionError):
    pass


class LDAPPasswordIsMandatoryError(LDAPExceptionError):
    pass


class LDAPInvalidFilterError(LDAPExceptionError):
    pass


class LDAPInvalidScopeError(LDAPExceptionError, ValueError):
    pass


class LDAPInvalidDereferenceAliasesError(LDAPExceptionError, ValueError):
    pass


class LDAPInvalidValueError(LDAPExceptionError, ValueError):
    pass


class LDAPControlError(LDAPExceptionError, ValueError):
    pass


class LDAPExtensionError(LDAPExceptionError, ValueError):
    pass


class LDAPLDIFError(LDAPExceptionError):
    pass


class LDAPSchemaError(LDAPExceptionError):
    pass


class LDAPSASLPrepError(LDAPExceptionError):
    pass


class LDAPSASLBindInProgressError(LDAPExceptionError):
    pass


class LDAPMetricsError(LDAPExceptionError):
    pass


class LDAPObjectClassError(LDAPExceptionError):
    pass


class LDAPInvalidDnError(LDAPExceptionError):
    pass


class LDAPResponseTimeoutError(LDAPExceptionError):
    pass


class LDAPTransactionError(LDAPExceptionError):
    pass


class LDAPInfoError(LDAPExceptionError):
    pass


# communication exceptions
class LDAPCommunicationError(LDAPExceptionError):
    pass


class LDAPSocketOpenError(LDAPCommunicationError):
    pass


class LDAPSocketCloseError(LDAPCommunicationError):
    pass


class LDAPSocketReceiveError(LDAPCommunicationError, socket.error):
    pass


class LDAPSocketSendError(LDAPCommunicationError, socket.error):
    pass


class LDAPSessionTerminatedByServerError(LDAPCommunicationError):
    pass


class LDAPUnknownResponseError(LDAPCommunicationError):
    pass


class LDAPUnknownRequestError(LDAPCommunicationError):
    pass


class LDAPReferralError(LDAPCommunicationError):
    pass


# pooling exceptions
class LDAPConnectionPoolNameIsMandatoryError(LDAPExceptionError):
    pass


class LDAPConnectionPoolNotStartedError(LDAPExceptionError):
    pass


# restartable strategy
class LDAPMaximumRetriesError(LDAPExceptionError):
    def __str__(self):
        s = []
        if self.args:
            if isinstance(self.args, tuple):
                if len(self.args) > 0:
                    s.append('LDAPMaximumRetriesError: ' + str(self.args[0]))
                if len(self.args) > 1:
                    s.append('Exception history:')
                    prev_exc = ''
                    for i, exc in enumerate(self.args[1]):  # args[1] contains exception history
                        # if str(exc[1]) != prev_exc:
                        #     s.append((str(i).rjust(5) + ' ' + str(exc[0]) + ': ' + str(exc[1]) + ' - ' + str(exc[2])))
                        #     prev_exc = str(exc[1])
                        if str(exc) != prev_exc:
                            s.append((str(i).rjust(5) + ' ' + str(type(exc)) + ': ' + str(exc)))
                            prev_exc = str(exc)
                if len(self.args) > 2:
                    s.append('Maximum number of retries reached: ' + str(self.args[2]))
        else:
            s = [LDAPExceptionError.__str__(self)]

        return sep.join(s)


# exception factories
def communication_exception_factory(exc_to_raise, exc):
    """
    Generates a new exception class of the requested type (subclass of LDAPCommunication) merged with the exception raised by the interpreter
    """
    if exc_to_raise.__name__ in [cls.__name__ for cls in LDAPCommunicationError.__subclasses__()]:
        return type(exc_to_raise.__name__, (exc_to_raise, type(exc)), dict())
    else:
        raise LDAPExceptionError('unable to generate exception type ' + str(exc_to_raise))


def start_tls_exception_factory(exc):
    """
    Generates a new exception class of the requested type merged with the exception raised by the interpreter
    """
    return type(LDAPStartTLSError.__name__, (LDAPStartTLSError, type(exc)), dict())
