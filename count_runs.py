#!/usr/bin/env python3
import re, os
CN_RUN = re.compile(r'[\u4e00-\u9fff\u3000-\u303f\uff00-\uffef]+')
js_dir = '/home/badb/CyberStrikeAI/web/static/js'
files = ['chat.js','roles.js','knowledge.js','webshell.js','monitor.js','settings.js','tasks.js','vulnerability.js','skills.js','info-collect.js','api-docs.js','router.js','chat-files.js','terminal.js','auth.js','dashboard.js','agents.js']
cn_runs = set()
for fn in files:
    fp = os.path.join(js_dir, fn)
    if not os.path.exists(fp): continue
    with open(fp,'r') as f:
        for line in f:
            for m in CN_RUN.findall(line):
                cn_runs.add(m)
sorted_runs = sorted(cn_runs, key=len, reverse=True)
print(f'Total unique Chinese-only runs: {len(sorted_runs)}')
# Write them all to a file
with open('/home/badb/CyberStrikeAI/cn_runs.txt','w') as f:
    for r in sorted_runs:
        f.write(r + '\n')
print('Written to cn_runs.txt')
