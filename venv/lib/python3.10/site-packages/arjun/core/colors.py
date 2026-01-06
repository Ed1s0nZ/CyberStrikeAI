import sys

colors = True # Output should be colored
machine = sys.platform # Detecting the os of current system
if machine.lower().startswith(('os', 'win', 'darwin', 'ios')):
    colors = False # Colors shouldn't be displayed in mac & windows
if not colors:
    white = green = red = yellow = end = back = info = que = bad = good = run = res = ''
else:
    white = '\033[97m'
    green = '\033[92m'
    red = '\033[91m'
    yellow = '\033[93m'
    end = '\033[0m'
    back = '\033[7;91m'
    info = '\033[1;93m[!]\033[0m'
    que = '\033[1;94m[?]\033[0m'
    bad = '\033[1;91m[-]\033[0m'
    good = '\033[1;32m[+]\033[0m'
    run = '\033[1;97m[*]\033[0m'
    res = '\033[1;92m[✓]\033[0m'
