---
name: ctf-writeup
description: Generates a single standardized submission-style CTF writeup for competition handoff and organizer review. Use after solving a CTF challenge to document the solution steps, tools used, and lessons learned in a structured format.
license: MIT
compatibility: Requires filesystem-based agent (Claude Code or similar) with bash and Python 3.
allowed-tools: Bash Read Write Edit Glob Grep Task WebFetch WebSearch
metadata:
  user-invocable: "true"
  argument-hint: "[challenge-name]"
---

# CTF Write-up Generator

Generate a standardized submission-style CTF writeup for a solved challenge.

Default behavior:

- During an active competition, optimize for speed, clarity, and reproducibility
- Keep writeups short enough that a teammate or organizer can validate the solve quickly
- Always produce a `submission`-style writeup
- Prefer one complete solve script from challenge data to final flag

## Workflow

### Step 1: Gather Information

Collect the following from the current session, challenge files, and user input:

1. **Challenge metadata** — name, CTF event, category, difficulty, points, flag format
2. **Solution artifacts** — exploit scripts, payloads, screenshots, command output
3. **Timeline** — key steps taken, dead ends, pivots

```bash
# Scan for exploit scripts and artifacts
find . -name '*.py' -o -name '*.sh' -o -name 'exploit*' -o -name 'solve*' | head -20
# Check for flags in output files
grep -rniE '(flag|ctf|eno|htb|pico)\{' . 2>/dev/null
```

### Step 2: Generate Write-up

Write the writeup file as `writeup.md` (or `writeup-<challenge-name>.md`) using the submission template below.

---

## Templates

### Submission Format

```markdown
---
title: "<Challenge Name>"
ctf: "<CTF Event Name>"
date: YYYY-MM-DD
category: web|pwn|crypto|reverse|forensics|osint|malware|misc
difficulty: easy|medium|hard
points: <number>
flag_format: "flag{...}"
author: "<your name or team>"
---

# <Challenge Name>

## Summary

<1-2 sentences: what the challenge was and the core technique. Keep it direct.>

## Solution

### Step 1: <Action>

<Explain the key observation in 3-8 short lines. Keep it direct.>

\`\`\`python
<one complete solving script from provided challenge data to printing the final flag>
\`\`\`

### Step 2: <Action> (optional)

<Only add this when a second short step genuinely helps readability, such as separating the core observation from final verification.>

### Step 3: <Action> (optional)

<Use only if the challenge really needs it. Keep the total number of steps small.>

## Flag

\`\`\`
flag{example_flag_here}
\`\`\`
```

Guidance:

- Prefer 1-3 short steps total
- Keep code to the smallest complete solving script
- Do not split "recover secret", "derive key", and "decrypt flag" into separate partial snippets
- The script should start from the challenge data and end by printing the flag
- Avoid long background sections
- Avoid dead ends unless they explain a key pivot
- Avoid multiple alternative solves; pick one clean path
- Redact the flag only if the user explicitly asks for redaction

---

## Best Practices Checklist

Before finalizing the writeup, verify:

- [ ] **Metadata complete** — title, CTF, date, category, difficulty, points, author all filled
- [ ] **Flag handling matches request** — keep the real flag unless the user asked for redaction
- [ ] **Reproducible steps** — a reader can follow your writeup and reproduce the solution
- [ ] **Code is runnable** — exploit scripts include all imports, correct variable names, and comments
- [ ] **No sensitive data** — no real credentials, API keys, or private infrastructure details
- [ ] **Length stays concise** — the writeup is short enough for fast review
- [ ] **Tools and versions noted** — mention specific tool versions if behavior depends on them
- [ ] **Proper attribution** — credit teammates, referenced writeups, or tools that were essential
- [ ] **Grammar and formatting** — consistent heading levels, code blocks have language tags

## Quality Guidelines

**DO:**
- Explain just enough for fast verification
- Include one complete solving path, not multiple alternative routes
- Include one complete script that goes all the way to the final flag
- Show actual output (truncated if very long) to prove the approach worked
- Tag code blocks with language (`python`, `bash`, `sql`, etc.)
- Keep the main path front-loaded so a reader can validate it quickly

**DON'T:**
- Copy-paste raw terminal dumps without explanation
- Paste several partial snippets that force the reader to reconstruct the final solve
- Leave placeholder text in the final writeup
- Include irrelevant tangents that don't contribute to the solution
- Assume the reader knows the specific challenge setup

## Challenge

$ARGUMENTS
