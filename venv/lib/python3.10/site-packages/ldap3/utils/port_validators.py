""" Some helper functions for validation of ports and lists of ports. """


def check_port(port):
    """ Check if a port is valid. Return an error message indicating what is invalid if something isn't valid. """
    if isinstance(port, int):
        if port not in range(0, 65535):
            return 'Source port must in range from 0 to 65535'
    else:
        return 'Source port must be an integer'
    return None


def check_port_and_port_list(port, port_list):
    """ Take in a port and a port list and check that at most one is non-null. Additionally check that if either
    is non-null, it is valid.
    Return an error message indicating what is invalid if something isn't valid.
    """
    if port is not None and port_list is not None:
        return 'Cannot specify both a source port and a source port list'
    elif port is not None:
        if isinstance(port, int):
            if port not in range(0, 65535):
                return 'Source port must in range from 0 to 65535'
        else:
            return 'Source port must be an integer'
    elif port_list is not None:
        try:
            _ = iter(port_list)
        except TypeError:
            return 'Source port list must be an iterable {}'.format(port_list)

        for candidate_port in port_list:
            err = check_port(candidate_port)
            if err:
                return err
    return None
