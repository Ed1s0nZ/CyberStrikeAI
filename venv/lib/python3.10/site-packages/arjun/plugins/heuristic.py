import re

from arjun.core.colors import info
import arjun.core.config as mem
from arjun.core.utils import extract_js

# TODO: for map keys, javascript tolerates { param: "value" }
re_words = re.compile(r'[A-Za-z][A-Za-z0-9_]*')
re_not_junk = re.compile(r'^[A-Za-z0-9_]+$')
re_inputs = re.compile(r'''(?i)<(?:input|textarea)[^>]+?(?:id|name)=["']?([^"'\s>]+)''')
re_empty_vars = re.compile(r'''(?:[;\n]|\bvar|\blet)(\w+)\s*=\s*(?:['"`]{1,2}|true|false|null)''')
re_map_keys = re.compile(r'''['"](\w+?)['"]\s*:\s*['"`]''')


def is_not_junk(param):
    return (re_not_junk.match(param) is not None)


def heuristic(raw_response, wordlist):
    words_exist = False
    potential_params = []

    headers, response = raw_response.headers, raw_response.text
    if headers.get('content-type', '').startswith(('application/json', 'text/plain')):
        if len(response) < 200:
            if ('required' or 'missing' or 'not found' or 'requires') in response.lower() and ('param' or 'parameter' or 'field') in response.lower():
                if not mem.var['quiet']:
                    print('%s The endpoint seems to require certain parameters to function. Check the response and use the --include option appropriately for better results.' % info)
            words_exist = True
            potential_params = re_words.findall(response)
    # Parse Inputs
    input_names = re_inputs.findall(response)
    potential_params += input_names

    # Parse Scripts
    for script in extract_js(response):
        empty_vars = re_empty_vars.findall(script)
        potential_params += empty_vars

        map_keys = re_map_keys.findall(script)
        potential_params += map_keys

    if len(potential_params) == 0:
        return [], words_exist

    found = set()
    for word in potential_params:
        if is_not_junk(word) and (word not in found):
            found.add(word)

            if word in wordlist:
                wordlist.remove(word)
            wordlist.insert(0, word)

    return list(found), words_exist
