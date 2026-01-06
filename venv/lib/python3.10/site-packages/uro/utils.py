def clean_nargs(args):
	"""
	cleans nargs to prevent user errors
	"""
	if not args:
		return []
	new_args = []
	if len(args) == 1:
		if "," in args[0]:
			new_args = [arg.lower() for arg in args[0].strip().split(',')]
		elif " " in args[0]:
			new_args = [arg.lower() for arg in args[0].split(' ')]
		else:
			new_args.append(args[0].lower())
	else:
		for arg in args:
			cleaner = clean_nargs([arg])
			if cleaner:
				new_args.extend(cleaner)
			else:
				new_args.append(arg)
	return list(set(filter(None, new_args)))

def params_to_dict(params):
	"""
	converts query string to dict
	"""
	the_dict = {}
	if params:
		for pair in params.split('&'):
			parts = pair.split('=')
			try:
				the_dict[parts[0]] = parts[1]
			except IndexError:
				pass
	return the_dict


def dict_to_params(params):
	"""
	converts dict of params to query string
	"""
	stringed = [name + '=' + value for name, value in params.items()]
	return '?' + '&'.join(stringed)


def compare_params(og_params, new_params):
	"""
	checks if new_params contain a param
	that doesn't exist in og_params
	"""
	og_set = set([])
	for each in og_params:
		for key in each.keys():
			og_set.add(key)
	return set(new_params.keys()) - og_set
