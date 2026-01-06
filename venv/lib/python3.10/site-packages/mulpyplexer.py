from __future__ import print_function

from functools import reduce # pylint: disable=redefined-builtin

class MP(object):
    def __init__(self, items):
        super(MP, self).__setattr__('mp_items', items)

    @staticmethod
    def _resolve_object(a, n):
        return a if not isinstance(a, MP) else a.mp_items[n]

    def _expand(self, o):
        expanded = [ ]

        for n in range(len(self.mp_items)):
            if isinstance(o, dict):
                e = { k:self._resolve_object(a, n) for (k,a) in o.items() }
            elif isinstance(o, list):
                e = [ self._resolve_object(a, n) for a in o ]
            elif isinstance(o, tuple):
                e = tuple(self._resolve_object(a, n) for a in o)
            else:
                e = self._resolve_object(o, n)

            expanded.append(e)

        return expanded

    def __repr__(self):
        return "MP(%s)" % (self.mp_items,)

    #
    # Plex-throughs!
    #

    def __getattr__(self, k):
        keys = self._expand(k)
        return MP([ getattr(i, k) for i,k in zip(self.mp_items,keys) ])

    def __setattr__(self, k, v):
        keys = self._expand(k)
        values = self._expand(v)
        return MP([ setattr(i, k, v) for i,k,v in zip(self.mp_items,keys,values) ])

    def __call__(self, *args, **kwargs):
        expanded_args, expanded_kwargs = self._expand(args), self._expand(kwargs)
        return MP([ i(*a, **k) for i,a,k in zip(self.mp_items,expanded_args,expanded_kwargs) ])

    def __getitem__(self, k):
        keys = self._expand(k)
        return MP([ i[k] for i,k in zip(self.mp_items,keys) ])

    def __setitem__(self, k, v):
        keys = self._expand(k)
        values = self._expand(v)
        return MP([ i.__setitem__(k, v) for i,k,v in zip(self.mp_items,keys,values) ])

    #
    # Plexionality
    #

    def mp_len(self):
        return [ len(i) for i in self.mp_items ]

    def mp_map(self, func):
        return MP([ func(i) for i in self.mp_items ])

    def mp_filter(self, func):
        return MP([ i for i in self.mp_items if func(i) ])

    def mp_flatten(self):
        items = [ ]
        for i in self.mp_items:
            if isinstance(i, (list, tuple, set)):
                items += list(i)
        return MP(items)

    def mp_union(self):
        items = set()
        for i in self.mp_items:
            if isinstance(i, (list, tuple, set)):
                items |= set(i)
        return MP(list(items))

    @property
    def mp_first(self):
        return self.mp_items[0]

    @property
    def mp_last(self):
        return self.mp_items[-1]

    def mp_sorted(self, key=None, reverse=False):
        return MP(sorted(self.mp_items, key=key, reverse=reverse))

    def mp_reduce(self, function, initial=None):
        reduce_args = [ function, self.mp_items ] if initial is None else [ function, self.mp_items, initial ]
        return reduce(*reduce_args)

    def __dir__(self):
        attrs = frozenset.intersection(*[frozenset(dir(i)) for i in self.mp_items])
        return list(sorted(attrs | { 'mp_items', 'mp_len', 'mp_map', 'mp_flatten', 'mp_union', 'mp_filter', 'mp_sorted', 'mp_reduce', 'mp_first', 'mp_last' } ))

def test():
    class A(object):
        def __init__(self, i, h=None):
            self.i = i
            self.h = [] if h is None else h + [i]

        def add(self, j):
            return A(self.i + (j.i if isinstance(j, A) else j), self.h)

        def sub(self, j):
            return A(self.i - (j.i if isinstance(j, A) else j), self.h)

        def __repr__(self):
            return "<A %d>" % self.i

        def str(self):
            return str(self.i)

        def __eq__(self, o):
            return self.i == o.i

    one = MP([ A(10), A(20), A(30) ])

    # test getattr
    ga = one.i
    assert ga.mp_items == [ 10, 20, 30 ]

    # test call
    two = one.add(5)
    assert two.mp_items == [ A(15), A(25), A(35) ]

    three = two.sub(10)
    assert three.mp_items == [ A(5), A(15), A(25) ]

    four = three.add(one)
    assert four.mp_items == [ A(15), A(35), A(55) ]

    five = four.str()
    assert five.mp_items == [ "15", "35", "55" ]

    six = four.i
    assert six.mp_items == [ 15, 35, 55 ]

    # test setattr
    four.i = one.add(5).i
    assert four.i.mp_items == [ 15, 25, 35 ]

    assert four.i.mp_sorted(reverse=True).mp_items == [ 35, 25, 15 ]

    import operator
    assert four.i.mp_reduce(operator.__add__, initial=10) == 35 + 25 + 15 + 10

    print("TESTS SUCCEEDED")

if __name__ == '__main__':
    test()
