import re

burp_regex = re.compile(r'''(?m)^    <url><!\[CDATA\[(.+?)\]\]></url>
    <host ip="[^"]*">[^<]+</host>
    <port>[^<]*</port>
    <protocol>[^<]*</protocol>
    <method><!\[CDATA\[(.+?)\]\]></method>
    <path>.*</path>
    <extension>(.*)</extension>
    <request base64="(?:false|true)"><!\[CDATA\[([\s\S]+?)]]></request>
    <status>([^<]*)</status>
    <responselength>([^<]*)</responselength>
    <mimetype>([^<]*)</mimetype>''')


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
    result['url'] = 'http://' + result['headers']['Host'] + result['path']
    result['data'] = match.group(4)
    return result


def parse_headers(string):
    """
    parses headers
    return dict
    """
    result = {}
    for line in string.split('\n'):
        if len(line) > 1:
            splitted = line.split(':')
            result[splitted[0]] = ':'.join(splitted[1:]).strip()
    return result


def burp_import(path):
    """
    imports targets from burp suite
    returns list (of request objects)
    """
    requests = []
    content = reader(path)
    matches = re.finditer(burp_regex, content)
    for match in matches:
        request = parse_request(match.group(4))
        headers = request['headers']
        if match.group(7) in ('HTML', 'JSON'):
            requests.append({
                'url': match.group(1),
                'method': match.group(2),
                'extension': match.group(3),
                'headers': headers,
                'include': request['data'],
                'code': match.group(5),
                'length': match.group(6),
                'mime': match.group(7)
            })
    return requests


def urls_import(path, method, headers, include):
    """
    imports urls from a newline delimited text file
    returns list (of request objects)
    """
    requests = []
    urls = reader(path, mode='lines')
    for url in urls:
        requests.append({
            'url': url,
            'method': method,
            'headers': headers,
            'data': include
        })
    return requests


def request_import(path):
    """
    imports request from a raw request file
    returns list
    """
    result = []
    result.append(parse_request(reader(path)))
    return result


def importer(path, method, headers, include):
    """
    main importer function that calls other import functions
    """
    with open(path, 'r', encoding='utf-8') as file:
        for line in file:
            if line.startswith('<?xml'):
                return burp_import(path)
            elif line.startswith(('http://', 'https://')):
                return urls_import(path, method, headers, include)
            elif line.startswith(('GET', 'POST')):
                return request_import(path)
            return []
