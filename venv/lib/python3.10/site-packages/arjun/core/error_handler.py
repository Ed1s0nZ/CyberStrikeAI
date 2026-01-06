import time

import arjun.core.config as mem

from arjun.core.colors import bad


def connection_refused():
	"""
	checks if a request should be retried if the server refused connection
	returns str
	"""
	if mem.var['stable']:
		print('%s Hit rate limit, stabilizing the connection' % bad)
		mem.var['kill'] = False
		time.sleep(30)
		return 'retry'
	print('%s Target has rate limiting in place, please use --stable switch' % bad)
	return 'kill'


def error_handler(response, factors):
	"""
	decides what to do after performing a HTTP request
		'ok': continue normally
		'retry': retry this request
		'kill': stop processing this target
	returns str
	"""
	if type(response) != str and response.status_code in (400, 413, 418, 429, 503):
		if not mem.var['healthy_url']:
			return 'ok'
		if response.status_code == 503:
			mem.var['kill'] = True
			print('%s Target is unable to process requests, try --stable switch' % bad)
			return 'kill'
		elif response.status_code in (429, 418):
			print('%s Target has a rate limit in place, try --stable switch' % bad)
			return 'kill'
		else:
			if factors['same_code'] != response.status_code:
				mem.var['bad_req_count'] = mem.var.get('bad_req_count', 0) + 1
				if mem.var['bad_req_count'] > 20:
					mem.var['kill'] = True
					print('%s Server received a bad request. Try decreasing the chunk size with -c option' % bad)
					return 'kill'
			else:
				return 'ok'
	else:
		if 'Timeout' in response:
			if mem.var['timeout'] > 20:
				mem.var['kill'] = True
				print('%s Connection timed out, unable to increase timeout further' % bad)
				print('%s Target might have a rate limit in place, try --stable switch' % bad)
				return 'kill'
			else:
				print('%s Connection timed out, increased timeout by 5 seconds' % bad)
				mem.var['timeout'] += 5
				return 'retry'
		elif 'ConnectionRefused' in response:
			return connection_refused()
		elif type(response) == str:
			if '\'' in response:
				print('%s Encountered an error: %s' % (bad, response.split('\'')[1]))
			return 'kill'
	return 'ok'
