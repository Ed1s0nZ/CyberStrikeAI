import json
import requests

import arjun.core.config as mem
from arjun.core.utils import populate

from arjun.core.utils import create_query_string


def json_export(result):
    """
    exports result to a file in JSON format
    """
    with open(mem.var['json_file'], 'w+', encoding='utf8') as json_output:
        json.dump(result, json_output, sort_keys=True, indent=4)


def burp_export(result):
    """
    exports results to Burp Suite by sending request to Burp proxy
    """
    proxy = ('' if ':' in mem.var['burp_proxy'] else '127.0.0.1:') + mem.var['burp_proxy']
    proxies = {
        'http': 'http://' + proxy,
        'https': 'https://' + proxy
    }
    for url, data in result.items():
        if data['method'] == 'GET':
            requests.get(url, params=populate(data['params']), headers=data['headers'], proxies=proxies, verify=False)
        elif data['method'] == 'POST':
            requests.post(url, data=populate(data['params']), headers=data['headers'], proxies=proxies, verify=False)
        elif data['method'] == 'JSON':
            requests.post(url, json=populate(data['params']), headers=data['headers'], proxies=proxies, verify=False)


def text_export(result):
    """
    exports results to a text file, one url per line
    """
    with open(mem.var['text_file'], 'a+', encoding='utf8') as text_file:
        for url, data in result.items():
            clean_url = url.lstrip('/')
            if data['method'] == 'JSON':
                text_file.write(clean_url + '\t' + json.dumps(populate(data['params'])) + '\n')
            else:
                query_string = create_query_string(data['params'])
                if '?' in clean_url:
                    query_string = query_string.replace('?', '&', 1)
                if data['method'] == 'GET':
                    text_file.write(clean_url + query_string + '\n')
                elif data['method'] == 'POST':
                    text_file.write(clean_url + '\t' + query_string + '\n')


def exporter(result):
    """
    main exporter function that calls other export functions
    """
    if mem.var['json_file']:
        json_export(result)
    if mem.var['text_file']:
        text_export(result)
    if mem.var['burp_proxy']:
        burp_export(result)
