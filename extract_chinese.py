#!/usr/bin/env python3
"""Extract all unique Chinese text segments from JS files."""
import re, os, json

CN_CHAR = re.compile(r'[\u4e00-\u9fff]')
# Match a run of text containing at least one Chinese character
# This grabs the full Chinese phrase including any interspersed ASCII
CN_SEGMENT = re.compile(r'(?:[\u4e00-\u9fff\u3000-\u303f\uff00-\uffef]+[\w\s\.\,\;\:\!\?\(\)\[\]\{\}\/\-\+\=\*\&\|\~\`\@\#\$\%\^\<\>\'\"\\\u4e00-\u9fff\u3000-\u303f\uff00-\uffef]*[\u4e00-\u9fff\u3000-\u303f\uff00-\uffef]+|[\u4e00-\u9fff])')

js_dir = '/home/badb/CyberStrikeAI/web/static/js'
files = ['chat.js', 'roles.js', 'knowledge.js', 'webshell.js', 'monitor.js',
         'settings.js', 'tasks.js', 'vulnerability.js', 'skills.js',
         'info-collect.js', 'api-docs.js', 'router.js', 'chat-files.js',
         'terminal.js', 'auth.js', 'dashboard.js', 'agents.js']

segments = set()
for fn in files:
    fp = os.path.join(js_dir, fn)
    if not os.path.exists(fp):
        continue
    with open(fp, 'r', encoding='utf-8') as f:
        for line in f:
            if CN_CHAR.search(line):
                # Extract complete line for context
                stripped = line.strip()
                if stripped.startswith('//'):
                    # Pure comment - take the whole comment text
                    comment_text = stripped[2:].strip()
                    if CN_CHAR.search(comment_text):
                        segments.add(comment_text)
                else:
                    # Mixed code+chinese - find chinese segments
                    matches = CN_SEGMENT.findall(stripped)
                    for m in matches:
                        segments.add(m.strip())

# Sort by length descending
sorted_segs = sorted(segments, key=len, reverse=True)
for s in sorted_segs:
    print(s)

print(f"\n--- Total unique segments: {len(sorted_segs)} ---")
