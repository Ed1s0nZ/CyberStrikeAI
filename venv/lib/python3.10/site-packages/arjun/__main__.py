#!/usr/bin/env python3
# -*- coding: utf-8 -*-

from arjun.core.colors import green, end, info, bad, good, run, res

import argparse
import json

from urllib.parse import urlparse
import arjun.core.config as mem
from arjun.core.exporter import exporter
from arjun.core.anomaly import define, compare
from arjun.core.utils import fetch_params, stable_request, random_str, slicer, confirm, populate, reader, nullify, prepare_requests, compatible_path

from arjun.plugins.heuristic import heuristic
from arjun.plugins.wl import detect_casing, covert_to_case

arjun_dir = compatible_path(mem.__file__.replace(compatible_path('/core/config.py'), ''))

parser = argparse.ArgumentParser() # defines the parser
# Arguments that can be supplied
parser.add_argument('-u', help='Target URL', dest='url')
parser.add_argument('-o', '-oJ', help='Path for json output file.', dest='json_file')
parser.add_argument('-oT', help='Path for text output file.', dest='text_file')
parser.add_argument('-oB', help='Output to Burp Suite Proxy. Default is 127.0.0.1:8080.', dest='burp_proxy', nargs='?', const='127.0.0.1:8080')
parser.add_argument('-d', help='Delay between requests in seconds. (default: 0)', dest='delay', type=float, default=0)
parser.add_argument('-t', help='Number of concurrent threads. (default: 5)', dest='threads', type=int, default=5)
parser.add_argument('-w', help='Wordlist file path. (default: {arjundir}/db/large.txt)', dest='wordlist', default=arjun_dir+'/db/large.txt')
parser.add_argument('-m', help='Request method to use: GET/POST/XML/JSON. (default: GET)', dest='method', default='GET')
parser.add_argument('-i', help='Import target URLs from file.', dest='import_file', nargs='?', const=True)
parser.add_argument('-T', help='HTTP request timeout in seconds. (default: 15)', dest='timeout', type=float, default=15)
parser.add_argument('-c', help='Chunk size. The number of parameters to be sent at once', type=int, dest='chunks', default=250)
parser.add_argument('-q', help='Quiet mode. No output.', dest='quiet', action='store_true')
parser.add_argument('--rate-limit', help='Max number of requests to be sent out per second (default: 9999)', dest='rate_limit', type=int, default=9999)
parser.add_argument('--headers', help='Add headers. Separate multiple headers with a new line.', dest='headers', nargs='?', const=True)
parser.add_argument('--passive', help='Collect parameter names from passive sources like wayback, commoncrawl and otx.', dest='passive', nargs='?', const='-')
parser.add_argument('--stable', help='Prefer stability over speed.', dest='stable', action='store_true')
parser.add_argument('--include', help='Include this data in every request.', dest='include', default={})
parser.add_argument('--disable-redirects', help='disable redirects', dest='disable_redirects', action='store_true')
parser.add_argument('--casing', help='casing style for params e.g. like_this, likeThis, likethis', dest='casing')
args = parser.parse_args() # arguments to be parsed

if args.quiet:
    print = nullify

print('''%s    _
   /_| _ '
  (  |/ /(//) v%s
      _/      %s
''' % (green, __import__('arjun').__version__, end))

try:
    from concurrent.futures import ThreadPoolExecutor, as_completed
except ImportError:
    print('%s Please use Python > 3.2 to run Arjun.' % bad)
    quit()

mem.var = vars(args)

mem.var['method'] = mem.var['method'].upper()

if mem.var['method'] != 'GET':
    mem.var['chunks'] = 500

if mem.var['stable'] or mem.var['delay']:
    mem.var['threads'] = 1
if mem.var['wordlist'] in ('large', 'medium', 'small'):
    mem.var['wordlist'] = f'{arjun_dir}/db/{mem.var["wordlist"]}.txt'

try:
    wordlist_file = arjun_dir + '/db/small.txt' if args.wordlist == 'small' else args.wordlist
    wordlist_file = compatible_path(wordlist_file)
    wordlist = set(reader(wordlist_file, mode='lines'))
    if mem.var['passive']:
        host = mem.var['passive']
        if host == '-':
            host = urlparse(args.url).netloc
        print('%s Collecting parameter names from passive sources for %s, it may take a while' % (run, host))
        passive_params = fetch_params(host)
        wordlist.update(passive_params)
        print('%s Collected %s parameters, added to the wordlist' % (info, len(passive_params)))
    if args.casing:
        delimiter, casing = detect_casing(args.casing)
        wordlist = [covert_to_case(word, delimiter, casing) for word in wordlist]
    else:
        wordlist = list(wordlist)
except FileNotFoundError:
    exit('%s The specified file for parameters doesn\'t exist' % bad)

if len(wordlist) < mem.var['chunks']:
    mem.var['chunks'] = int(len(wordlist)/2)

if not args.url and not args.import_file:
    exit('%s No target(s) specified' % bad)

from arjun.core.requester import requester
from arjun.core.bruter import bruter

def narrower(request, factors, param_groups):
    """
    takes a list of parameters and narrows it down to parameters that cause anomalies
    returns list
    """
    anomalous_params = []
    threadpool = ThreadPoolExecutor(max_workers=mem.var['threads'])
    futures = (threadpool.submit(bruter, request, factors, params) for params in param_groups)
    for i, result in enumerate(as_completed(futures)):
        if result.result():
            anomalous_params.extend(slicer(result.result()))
        if mem.var['kill']:
            return anomalous_params
        print('%s Processing chunks: %i/%-6i' % (info, i + 1, len(param_groups)), end='\r')
    return anomalous_params


def initialize(request, wordlist, single_url=False):
    """
    handles parameter finding process for a single request object
    returns 'skipped' (on error), list on success
    """
    url = request['url']
    if not url.startswith('http'):
        print('%s %s is not a valid URL' % (bad, url))
        return 'skipped'
    print('%s Probing the target for stability' % run)
    request['url'] = stable_request(url, request['headers'])
    mem.var['healthy_url'] = True
    if not request['url']:
        return 'skipped'
    else:
        fuzz = "z" + random_str(6)
        response_1 = requester(request, {fuzz[:-1]: fuzz[::-1][:-1]})
        mem.var['healthy_url'] = response_1.status_code not in (400, 413, 418, 429, 503)
        if not mem.var['healthy_url']:
            print('%s Target returned HTTP %i, this may cause problems.' % (bad, request.status_code))
        if single_url:
            print('%s Analysing HTTP response for anomalies' % run)
        response_2 = requester(request, {fuzz[:-1]: fuzz[::-1][:-1]})
        if type(response_1) == str or type(response_2) == str:
            return 'skipped'

        # params from response must be extracted before factors but displayed later
        found, words_exist = heuristic(response_1, wordlist)

        factors = define(response_1, response_2, fuzz, fuzz[::-1], wordlist)
        zzuf = "z" + random_str(6)
        response_3 = requester(request, {zzuf[:-1]: zzuf[::-1][:-1]})
        while True:
            reason = compare(response_3, factors, {zzuf[:-1]: zzuf[::-1][:-1]})[2]
            if not reason:
                break
            factors[reason] = None
        if found:
            num = len(found)
            if words_exist:
                print('%s Extracted %i parameters from response for testing' % (good, num))
            else:
                s = 's' if num > 1 else ''
                print('%s Extracted %i parameter%s from response for testing: %s' % (good, num, s, ', '.join(found)))
        if single_url:
            print('%s Logicforcing the URL endpoint' % run)
        populated = populate(wordlist)
        with open(f'{arjun_dir}/db/special.json', 'r') as f:
            populated.update(json.load(f))
        param_groups = slicer(populated, int(len(wordlist)/mem.var['chunks']))
        prev_chunk_count = len(param_groups)
        last_params = []
        while True:
            param_groups = narrower(request, factors, param_groups)
            if len(param_groups) > prev_chunk_count:
                response_3 = requester(request, {zzuf[:-1]: zzuf[::-1][:-1]})
                if compare(response_3, factors, {zzuf[:-1]: zzuf[::-1][:-1]})[0] != '':
                    print('%s Webpage is returning different content on each request. Skipping.' % bad)
                    return []
            if mem.var['kill']:
                return 'skipped'
            param_groups = confirm(param_groups, last_params)
            prev_chunk_count = len(param_groups)
            if not param_groups:
                break
        confirmed_params = []
        for param in last_params:
            reason = bruter(request, factors, param, mode='verify')
            if reason:
                name = list(param.keys())[0]
                confirmed_params.append(name)
                if single_url:
                    print('%s parameter detected: %s, based on: %s' % (res, name, reason))
        return confirmed_params


def main():
    requests = prepare_requests(args)

    final_result = {}
    is_single = False if args.import_file else True

    try:
        mem.var['kill'] = False
        count = 0
        for request in requests:
            url = request['url']
            print('%s Scanning %d/%d: %s' % (run, count, len(requests), url))
            these_params = initialize(request, wordlist, single_url=is_single)
            count += 1
            mem.var['kill'] = False
            mem.var['bad_req_count'] = 0
            if these_params == 'skipped':
                print('%s Skipped %s due to errors' % (bad, url))
            elif these_params:
                final_result[url] = {}
                final_result[url]['params'] = these_params
                final_result[url]['method'] = request['method']
                final_result[url]['headers'] = request['headers']
                exporter(final_result)
                print('%s Parameters found: %-4s\n' % (good, ', '.join(final_result[url]['params'])))
                if not mem.var['json_file']:
                    final_result = {}
                    continue
            else:
                print('%s No parameters were discovered.\n' % info)
    except KeyboardInterrupt:
        exit()


if __name__ == '__main__':
    main()
