#!/usr/bin/env python3
"""Extract all unique lines containing Chinese from target JS files, output as JSON for translation."""
import re, os, json

CN_RE = re.compile(r'[\u4e00-\u9fff]')

js_dir = '/home/badb/CyberStrikeAI/web/static/js'
files = ['chat.js', 'roles.js', 'knowledge.js', 'webshell.js', 'monitor.js',
         'settings.js', 'tasks.js', 'vulnerability.js', 'skills.js',
         'info-collect.js', 'api-docs.js', 'router.js', 'chat-files.js',
         'terminal.js', 'auth.js', 'dashboard.js', 'agents.js']

lines_set = set()
for fn in files:
    fp = os.path.join(js_dir, fn)
    if not os.path.exists(fp):
        continue
    with open(fp, 'r', encoding='utf-8') as f:
        for line in f:
            stripped = line.rstrip('\n')
            if CN_RE.search(stripped):
                lines_set.add(stripped)

# Sort by length for readability
sorted_lines = sorted(lines_set, key=len)
with open('/home/badb/CyberStrikeAI/chinese_lines.json', 'w', encoding='utf-8') as f:
    json.dump(sorted_lines, f, ensure_ascii=False, indent=2)

print(f"Total unique lines with Chinese: {len(sorted_lines)}")
