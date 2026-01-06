import requests

from urllib.parse import urlparse


def wayback(host, page):
	payload = {
		'url': host,
		'matchType': 'host',
		'collapse': 'urlkey',
		'fl': 'original',
		'page': page,
		'limit': 10000
	}
	headers = {
		'User-Agent': 'Mozilla'
	}
	try:
		these_params = set()
		response = requests.get(
			'http://web.archive.org/cdx/search?filter=mimetype:text/html&filter=statuscode:200',
			params=payload,
			headers=headers,
			verify=False
		).text
		if not response:
			return (these_params, False, 'wayback')
		urls = filter(None, response.split('\n'))
		for url in urls:
			for param in urlparse(url).query.split('&'):
				these_params.add(param.split('=')[0])
		return (these_params, True, 'wayback')
	except requests.exceptions.ConnectionError:
		return (these_params, False, 'wayback')
