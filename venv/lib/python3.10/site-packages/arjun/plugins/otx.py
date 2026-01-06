import requests

from urllib.parse import urlparse


def otx(host, page):
	these_params = set()
	data = requests.get('https://otx.alienvault.com/api/v1/indicators/hostname/%s/url_list?limit=50&page=%d' % (host, page+1), verify=False).json()
	if 'url_list' not in data:
		return (these_params, False, 'otx')
	for obj in data['url_list']:
		for param in urlparse(obj['url']).query.split('&'):
			these_params.add(param.split('=')[0])
	return (these_params, data['has_next'], 'otx')
