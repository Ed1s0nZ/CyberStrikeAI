import re
import sys
import json
import random
import requests

import concurrent.futures
from dicttoxml import dicttoxml
from urllib.parse import urlparse

from arjun.core.prompt import prompt
from arjun.core.importer import importer

from arjun.plugins.otx import otx
from arjun.plugins.wayback import wayback
from arjun.plugins.commoncrawl import commoncrawl

import arjun.core.config as mem
from arjun.core.colors import info


def extract_headers(headers):
    """
    parses headers provided through command line
    returns dict
    """
    headers = headers.replace('\\n', '\n')
    return parse_headers(headers)


def confirm(array_of_dicts, usable):
    """
    extracts the value from single valued dict from an array of dicts
    returns an array of dicts
    """
    param_groups = []
    for dic in array_of_dicts:
        if len(dic) == 1:
            usable.append(dic)
        else:
            param_groups.append(dic)
    return param_groups


def slicer(dic, n=2):
    """
    divides dict into n parts
    returns array containing n dicts
    """
    listed = list(dic.items())
    k, m = divmod(len(dic), n)
    return [dict(listed[i * k + min(i, m):(i + 1) * k + min(i + 1, m)]) for i in range(n)]


def populate(array):
    """
    converts a list of parameters into parameter and value pair
    returns dict
    """
    return {name: '1' * (6 - len(str(i))) + str(i) for i, name in enumerate(array)}


def stable_request(url, headers):
    """
    guarantees crash-proof HTTP(S) requests
    returns None in case of failure, returns a "response" object otherwise
    """
    parsed = urlparse(url)
    redirects_allowed = False if mem.var['disable_redirects'] else True
    scheme, host, path = parsed.scheme, parsed.netloc, parsed.path
    schemes = (['https', 'http'] if scheme == 'https' else ['http', 'https'])
    for scheme in schemes:
        try:
            response = requests.get(
                scheme + '://' + host + path,
                headers=headers,
                verify=False,
                timeout=10,
                allow_redirects=redirects_allowed)
            content = response.headers.get('Content-Type', '')
            if not ('text' in content or 'html' in content or 'json' in content or 'xml' in content):
                print('%s URL doesn\'t seem to be a webpage. Skipping.' % info)
                return None
            return response.url
        except Exception as e:
            if 'ConnectionError' not in str(e):
                continue
        return None


def remove_tags(html):
    """
    removes all the html from a webpage source
    """
    return re.sub(r'(?s)<.*?>', '', html)


def diff_map(body_1, body_2):
    """
    creates a list of lines that are common between two multi-line strings
    returns list
    """
    sig = []
    lines_1, lines_2 = body_1.split('\n'), body_2.split('\n')
    for line_1, line_2 in zip(lines_1, lines_2):
        if line_1 == line_2:
            sig.append(line_1)
    return sig


def random_str(n):
    """
    generates a random string of length n
    returns a string containing only digits
    """
    return ''.join(str(random.choice(range(10))) for i in range(n))


def get_params(include):
    """
    loads parameters from JSON/query string
    returns parameter dict
    """
    params = {}
    if include:
        if include.startswith('{'):
            try:
                params = json.loads(str(include).replace('\'', '"'))
                if type(params) != dict:
                    return {}
                return params
            except json.decoder.JSONDecodeError:
                return {}
        else:
            cleaned = include.split('?')[-1]
            parts = cleaned.split('&')
            for part in parts:
                each = part.split('=')
                try:
                    params[each[0]] = each[1]
                except IndexError:
                    params = {}
    return params


def create_query_string(params):
    """
    creates a query string from a list of parameters
    returns str
    """
    query_string = ''
    for param in params:
        pair = param + '=' + random_str(4) + '&'
        query_string += pair
    if query_string.endswith('&'):
        query_string = query_string[:-1]
    return '?' + query_string


def reader(path, mode='string'):
    """
    reads a file
    returns a string/array containing the content of the file
    """
    with open(path, 'r', encoding='utf-8') as file:
        if mode == 'lines':
            return list(filter(None, [line.rstrip('\n') for line in file]))
        else:
            return ''.join([line for line in file])


def extract_js(response):
    """
    extracts javascript from a given string
    """
    scripts = []
    for part in re.split('(?i)<script[> ]', response):
        actual_parts = re.split('(?i)</script>', part, maxsplit=2)
        if len(actual_parts) > 1:
            scripts.append(actual_parts[0])
    return scripts


def parse_headers(string):
    """
    parses headers
    returns dict
    """
    result = {}
    for line in string.split('\n'):
        if len(line) > 1:
            splitted = line.split(':')
            result[splitted[0]] = ':'.join(splitted[1:]).strip()
    return result


def parse_request(string):
    """
    parses http request
    returns dict
    """
    result = {}
    match = re.search(r'(?:([a-zA-Z0-9]+) ([^ ]+) [^ ]+\n)?([\s\S]+\n)\n?([\s\S]+)?', string)
    result['method'] = match.group(1)
    result['path'] = match.group(2)
    result['headers'] = parse_headers(match.group(3))
    result['data'] = match.group(4)
    return result


def http_import(path):
    """
    parses http request from a file
    returns dict
    """
    return parse_request(reader(path))


def fetch_params(host):
    """
    fetch parameters from passive sources
    returns list
    """
    available_plugins = {'commoncrawl': commoncrawl, 'otx': otx, 'wayback': wayback}
    page = 0
    progress = 0
    params = {}
    while len(available_plugins) > 0 and page <= 10:
        threadpool = concurrent.futures.ThreadPoolExecutor(max_workers=len(available_plugins))
        futures = (threadpool.submit(func, host, page) for func in available_plugins.values())
        for each in concurrent.futures.as_completed(futures):
            if progress < 98:
                progress += 3
            this_result = each.result()
            if not this_result[1]:
                progress += ((10 - page) * 10 / 3)
                del available_plugins[this_result[2]]
            if len(this_result[0]) > 1:
                if not params:
                    params = this_result[0]
                else:
                    params.update(this_result[0])
            print('%s Progress: %i%%' % (info, progress), end='\r')
        page += 1
    print('%s Progress: %i%%' % (info, 100), end='\r')
    return params


def prepare_requests(args):
    """
    creates a list of request objects used by Arjun from targets given by user
    returns list (of targets)
    """
    headers = {
        'User-Agent': 'Mozilla/5.0 (X11; Linux x86_64; rv:83.0) Gecko/20100101 Firefox/83.0',
        'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8',
        'Accept-Language': 'en-US,en;q=0.5',
        'Accept-Encoding': 'gzip, deflate',
        'Connection': 'close',
        'Upgrade-Insecure-Requests': '1'
    }
    result = []
    if type(args.headers) == str:
        headers = extract_headers(args.headers)
    elif args.headers:
        headers = extract_headers(prompt())
    if mem.var['method'] == 'JSON':
        headers['Content-type'] = 'application/json'
    if args.url:
        params = get_params(args.include)
        result.append(
            {
                'url': args.url,
                'method': mem.var['method'],
                'headers': headers,
                'include': params
            }
        )
    elif args.import_file:
        result = importer(args.import_file, mem.var['method'], headers, args.include)
    return result


def nullify(*args, **kwargs):
    """
    a function that does nothing
    """
    pass


def dict_to_xml(dict_obj):
    """
    converts dict to xml string
    returns str
    """
    return dicttoxml(dict_obj, root=False, attr_type=False).decode('utf-8')


def compatible_path(path):
    """
    converts filepaths to be compatible with the host OS
    returns str
    """
    if sys.platform.lower().startswith('win'):
        return path.replace('/', '\\')
    return path
