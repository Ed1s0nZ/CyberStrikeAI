#!/usr/bin/env python3
import re, os
CN_RUN = re.compile(r'[\u4e00-\u9fff]+')
js_dir = '/home/badb/CyberStrikeAI/web/static/js'
files = ['chat.js','roles.js','knowledge.js','webshell.js','monitor.js','settings.js','tasks.js','vulnerability.js','skills.js','info-collect.js','api-docs.js','router.js','chat-files.js','terminal.js','auth.js','dashboard.js','agents.js']
runs = {}
for fn in files:
    fp = os.path.join(js_dir, fn)
    if not os.path.exists(fp): continue
    with open(fp,'r') as f:
        for m in CN_RUN.findall(f.read()):
            if len(m) >= 2:
                runs[m] = runs.get(m,0)+1

with open('/home/badb/CyberStrikeAI/do_translate.py','r') as f:
    script = f.read()

missing = {k:v for k,v in runs.items() if ("'" + k + "'") not in script and ("'" + k) not in script}
sorted_missing = sorted(missing.items(), key=lambda x: x[1], reverse=True)
for word, count in sorted_missing[:200]:
    print(f'{count:4d}  {word}')
print(f'\nTotal missing 2+ char words: {len(missing)}')
