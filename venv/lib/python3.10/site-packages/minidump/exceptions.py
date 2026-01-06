
class MinidumpException(Exception):
	"""Generic Exception from minidump module"""
	pass

class MinidumpHeaderSignatureMismatchException(Exception):
	"""Header signature was not correct"""
	pass

class MinidumpHeaderFlagsException(Exception):
	"""Header flags value was not correct"""
	pass