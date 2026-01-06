import argparse
import io
import re
import sys
from urllib.parse import urlparse

from uro.utils import *
from uro.filters import *

try:
	from signal import signal, SIGPIPE, SIG_DFL
	signal(SIGPIPE, SIG_DFL)
except ImportError:
	pass

parser = argparse.ArgumentParser()
parser.add_argument('-i', help='file containing urls', dest='input_file')
parser.add_argument('-o', help='output file', dest='output_file')
parser.add_argument('-w', '--whitelist', help='only keep these extensions and extensionless urls', dest='whitelist', nargs='+')
parser.add_argument('-b', '--blacklist', help='remove these extensions', dest='blacklist', nargs='+')
parser.add_argument('-f', '--filters', help='additional filters, read docs', dest='filters', nargs='+')
args = parser.parse_args()

filter_map = {
	'hasext': has_ext,
	'noext': no_ext,
	'hasparams': has_params,
	'noparams': no_params,
	'removecontent': remove_content,
	'blacklist': blacklisted,
	'whitelist': whitelisted,
	'vuln': has_vuln_param,
}

filters = clean_nargs(args.filters)
active_filters = ['removecontent']

if 'allexts' in filters:
	filters.remove('allexts')
else:
	if args.whitelist:
		active_filters.append('whitelist')
	else:
		active_filters.append('blacklist')

for i in filters:
	if i in filter_map or i in ('keepcontent', 'keepslash'):
		active_filters.append(i)
	elif i + 's' in filter_map:
		active_filters.append(i + 's')
	elif i[:-1] in filter_map:
		active_filters.append(i[:-1])
	else:
		print('[ERROR] Invalid filter:', i, file=sys.stderr)
		exit(1)

if 'keepcontent' in active_filters:
	active_filters.remove('removecontent')
	active_filters.remove('keepcontent')

keepslash = True if 'keepslash' in active_filters else False
if keepslash:
	active_filters.remove('keepslash')

urlmap = {}
params_seen = set()
patterns_seen = set()

re_int = re.compile(r'/\d+([?/]|$)')

ext_list = tuple(clean_nargs(args.blacklist)) if args.blacklist else tuple(('css', 'png', 'jpg', 'jpeg', 'svg',
	'ico','webp', 'scss','tif','tiff','ttf','otf','woff','woff2', 'gif',
	'pdf', 'bmp', 'eot', 'mp3', 'mp4', 'avi'
))

vuln_params = set(['file', 'document', 'folder', 'root', 'path', 'pg', 'style', 'pdf', 'template', 'php_path', 'doc', 'page', 'name', 'cat', 'dir', 'action', 'board', 'date', 'detail', 'download', 'prefix', 'include', 'inc', 'locate', 'show', 'site', 'type', 'view', 'content', 'layout', 'mod', 'conf', 'daemon', 'upload', 'log', 'ip', 'cli', 'cmd', 'exec', 'command', 'execute', 'ping', 'query', 'jump', 'code', 'reg', 'do', 'func', 'arg', 'option', 'load', 'process', 'step', 'read', 'function', 'req', 'feature', 'exe', 'module', 'payload', 'run', 'print', 'callback', 'checkout', 'checkout_url', 'continue', 'data', 'dest', 'destination', 'domain', 'feed', 'file_name', 'file_url', 'folder_url', 'forward', 'from_url', 'go', 'goto', 'host', 'html', 'image_url', 'img_url', 'load_file', 'load_url', 'login_url', 'logout', 'navigation', 'next', 'next_page', 'Open', 'out', 'page_url', 'port', 'redir', 'redirect', 'redirect_to', 'redirect_uri', 'redirect_url', 'reference', 'return', 'return_path', 'return_to', 'returnTo', 'return_url', 'rt', 'rurl', 'target', 'to', 'uri', 'url', 'val', 'validate', 'window', 'q', 's', 'search', 'lang', 'keyword', 'keywords', 'year', 'email', 'p', 'jsonp', 'api_key', 'api', 'password', 'emailto', 'token', 'username', 'csrf_token', 'unsubscribe_token', 'id', 'item', 'page_id', 'month', 'immagine', 'list_type', 'terms', 'categoryid', 'key', 'l', 'begindate', 'enddate', 'select', 'report', 'role', 'update', 'user', 'sort', 'where', 'params', 'row', 'table', 'from', 'sel', 'results', 'sleep', 'fetch', 'order', 'column', 'field', 'delete', 'string', 'number', 'filter', 'access', 'admin', 'dbg', 'debug', 'edit', 'grant', 'test', 'alter', 'clone', 'create', 'disable', 'enable', 'make', 'modify', 'rename', 'reset', 'shell', 'toggle', 'adm', 'cfg', 'open', 'img', 'filename', 'preview', 'activity'])

if args.whitelist:
	ext_list = tuple(clean_nargs(args.whitelist))


def create_pattern(path):
	"""
	creates patterns for urls with integers in them
	"""
	new_parts = []
	last_index = 0
	for i, part in enumerate(re.escape(path).split('/')):
		if part.isdigit():
			last_index = i
			new_parts.append('\\d+')
		else:
			new_parts.append(part)
	return re.compile('/'.join(new_parts[:last_index + 1]))


def apply_filters(path, params):
	"""
	apply filters to a url
	returns True if the url should be kept
	"""
	meta = {
		'strict': True if ('hasext' or 'noext') in filters else False,
		'ext_list': ext_list,
		'vuln_params': vuln_params,
	}
	for filter in active_filters:
		if not filter_map[filter](path, params, meta):
			return False
	return True


def process_url(url):
	"""
	processes a url
	"""
	host = url.scheme + '://' + url.netloc
	if host not in urlmap:
		urlmap[host] = {}
	path, params = url.path, params_to_dict(url.query)
	new_params = [] if not params else [param for param in params.keys() if param not in params_seen]
	keep_url = apply_filters(path, params)
	if not keep_url:
		return
	params_seen.update(new_params)
	new_path = path not in urlmap[host]
	if new_path:
		if re_int.search(path):
			pattern = create_pattern(path)
			if pattern in patterns_seen:
				return
			patterns_seen.add(pattern)
		urlmap[host][path] = []
		if params:
			urlmap[host][path].append(params)
	else:
		if new_params:
			urlmap[host][path].append(params)
		elif compare_params(urlmap[host][path], params):
			urlmap[host][path].append(params)


def process_line(line):
	"""
	processes a single line from input
	"""
	cleanline = line.strip() if keepslash else line.strip().rstrip('/')
	try:
		parsed_url = urlparse(cleanline)
		if parsed_url.netloc:
			process_url(parsed_url)
	except ValueError:
		pass

def main():
	if args.input_file:
		with open(args.input_file, 'r', encoding='utf-8', errors='ignore') as input_file:
			for line in input_file:
				process_line(line)
	elif not sys.stdin.isatty():
		for line in io.TextIOWrapper(sys.stdin.buffer, encoding='utf-8', errors='ignore'):
			process_line(line)
	else:
		print('[ERROR] No input file or stdin.', file=sys.stderr)
		exit(1)

	og_stdout = sys.stdout
	sys.stdout = open(args.output_file, 'a+') if args.output_file else sys.stdout
	for host, value in urlmap.items():
		for path, params in value.items():
			if params:
				for param in params:
					print(host + path + dict_to_params(param))
			else:
				print(host + path)
