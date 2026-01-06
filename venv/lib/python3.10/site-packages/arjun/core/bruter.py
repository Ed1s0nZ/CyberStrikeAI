import arjun.core.config as mem

from arjun.core.anomaly import compare
from arjun.core.requester import requester
from arjun.core.error_handler import error_handler


def bruter(request, factors, params, mode='bruteforce'):
    """
    returns anomaly detection result for a chunk of parameters
    returns list
    """
    if mem.var['kill']:
        return []
    response = requester(request, params)
    conclusion = error_handler(response, factors)
    if conclusion == 'retry':
        return bruter(request, factors, params, mode=mode)
    elif conclusion == 'kill':
        mem.var['kill'] = True
        return []
    comparison_result = compare(response, factors, params)
    if mode == 'verify':
        return comparison_result[0]
    return comparison_result[1]
