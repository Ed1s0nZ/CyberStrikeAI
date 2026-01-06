import re
import requests

import arjun.core.config as mem

from urllib.parse import urlparse
from arjun.core.utils import diff_map, remove_tags


def define(response_1, response_2, param, value, wordlist):
    """
    defines a rule list for detecting anomalies by comparing two HTTP response
    returns dict
    """
    factors = {
        'same_code': None, # if http status code is same, contains that code
        'same_body': None, # if http body is same, contains that body
        'same_plaintext': None, # if http body isn't same but is same after removing html, contains that non-html text
        'lines_num': None, # if number of lines in http body is same, contains that number
        'lines_diff': None, # if http-body or plaintext aren't and there are more than two lines, contain which lines are same
        'same_headers': None, # if the headers are same, contains those headers
        'same_redirect': None, # if both requests redirect in similar manner, contains that redirection
        'param_missing': None, # if param name is missing from the body, contains words that are already there
        'value_missing': None # contains whether param value is missing from the body
    }
    if type(response_1) == type(response_2) == requests.models.Response:
        body_1, body_2 = response_1.text, response_2.text
        if response_1.status_code == response_2.status_code:
            factors['same_code'] = response_1.status_code
        if response_1.headers.keys() == response_2.headers.keys():
            factors['same_headers'] = list(response_1.headers.keys())
            factors['same_headers'].sort()
        if mem.var['disable_redirects']:
            if response_1.headers.get('Location', '') == response_2.headers.get('Location', ''):
                factors['same_redirect'] = urlparse(response_1.headers.get('Location', '')).path
        elif urlparse(response_1.url).path == urlparse(response_2.url).path:
            factors['same_redirect'] = urlparse(response_1.url).path
        else:
            factors['same_redirect'] = ''
        if response_1.text == response_2.text:
            factors['same_body'] = response_1.text
        elif response_1.text.count('\n') == response_2.text.count('\n'):
            factors['lines_num'] = response_1.text.count('\n')
        elif remove_tags(body_1) == remove_tags(body_2):
            factors['same_plaintext'] = remove_tags(body_1)
        elif body_1 and body_2 and body_1.count('\\n') == body_2.count('\\n'):
            factors['lines_diff'] = diff_map(body_1, body_2)
        if param not in response_2.text:
            factors['param_missing'] = [word for word in wordlist if word in response_2.text]
        if value not in response_2.text:
            factors['value_missing'] = True
    return factors


def compare(response, factors, params):
    """
    detects anomalies by comparing a HTTP response against a rule list
    returns string, list (anomaly, list of parameters that caused it)
    """
    if response == '' or type(response) == str:
        return ('', [], '')
    these_headers = list(response.headers.keys())
    these_headers.sort()
    if factors['same_code'] is not None and response.status_code != factors['same_code']:
        return ('http code', params, 'same_code')
    if factors['same_headers'] is not None and these_headers != factors['same_headers']:
        return ('http headers', params, 'same_headers')
    if mem.var['disable_redirects']:
        if factors['same_redirect'] is not None and urlparse(response.headers.get('Location', '')).path != factors['same_redirect']:
            return ('redirection', params, 'same_redirect')
    elif factors['same_redirect'] is not None and 'Location' in response.headers:
        if urlparse(response.headers.get('Location', '')).path != factors['same_redirect']:
            return ('redirection', params, 'same_redirect')
    if factors['same_body'] is not None and response.text != factors['same_body']:
        return ('body length', params, 'same_body')
    if factors['lines_num'] is not None and response.text.count('\n') != factors['lines_num']:
        return ('number of lines', params, 'lines_num')
    if factors['same_plaintext'] is not None and remove_tags(response.text) != factors['same_plaintext']:
        return ('text length', params, 'same_plaintext')
    if factors['lines_diff'] is not None:
        for line in factors['lines_diff']:
            if line not in response.text:
                return ('lines', params, 'lines_diff')
    if factors['param_missing'] is not None:
        for param in params.keys():
            if len(param) < 5:
                continue
            if param not in factors['param_missing'] and re.search(r'[\'"\s]%s[\'"\s]' % re.escape(param), response.text):
                return ('param name reflection', params, 'param_missing')
    if factors['value_missing'] is not None:
        for value in params.values():
            if type(value) != str or len(value) != 6:
                continue
            if value in response.text and re.search(r'[\'"\s]%s[\'"\s]' % re.escape(value), response.text):
                return ('param value reflection', params, 'value_missing')
    return ('', [], '')
