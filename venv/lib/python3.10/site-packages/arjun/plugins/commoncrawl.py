import requests

from urllib.parse import urlparse


def commoncrawl(host, page=0):
	these_params = set()
	response = requests.get('http://index.commoncrawl.org/CC-MAIN-2024-42-index?url=*.%s&fl=url&page=%s&limit=10000' % (host, page), verify=False).text
	if response.startswith('<!DOCTYPE html>'):
		return ([], False, 'commoncrawl')
	urls = response.split('\n')
	for url in urls:
		for param in urlparse(url).query.split('&'):
			these_params.add(param.split('=')[0])
	return (these_params, True, 'commoncrawl')
