# CTFd Platform Navigation (No Browser)

Programmatic interaction with CTFd-based CTF platforms via REST API. Eliminates browser dependency during competitions.

## Table of Contents

- [Detect CTFd](#detect-ctfd)
- [Authentication](#authentication)
- [List Challenges](#list-challenges)
- [Challenge Details](#challenge-details)
- [Download Challenge Files](#download-challenge-files)
- [Submit Flags](#submit-flags)
- [Scoreboard](#scoreboard)
- [Hints and Unlocks](#hints-and-unlocks)
- [Notifications](#notifications)
- [User and Team Info](#user-and-team-info)
- [Full Competition Workflow](#full-competition-workflow)
- [Python CTFd Client](#python-ctfd-client)
- [Troubleshooting](#troubleshooting)

---

## Detect CTFd

CTFd fingerprints in HTTP responses:

```bash
# Check for CTFd signatures in response headers and body
curl -sI "$CTF_URL" | grep -i 'ctfd\|powered-by'

# Check for CTFd API endpoint (returns Swagger UI or JSON)
curl -s "$CTF_URL/api/v1/" | head -20

# Check for CTFd static assets
curl -s "$CTF_URL" | grep -oE '(ctfd|CTFd|/themes/core)'

# Check for CTFd login page structure
curl -s "$CTF_URL/login" | grep -oE 'name="nonce"'
```

**Key indicators:**
- `/api/v1/` returns Swagger/RESTX documentation
- HTML contains `/themes/core/` asset paths
- Login form includes a `nonce` hidden field
- Response headers may include `CTFd` in `Server` or `X-Powered-By`

---

## Authentication

CTFd supports two auth methods: session cookies (login flow) and API tokens (recommended).

**Important:** When CTFd is detected, **ask the user for their API token**. Tokens are not provided by default — the user must generate one from the CTFd web UI (Settings > Access Tokens) before API access works. If the user doesn't have a token yet, guide them: log in to CTFd in a browser, go to Settings > Access Tokens, create a token, and paste it back.

### Method 1: API Token (Recommended)

Generate a token from the CTFd web UI (Settings > Access Tokens), or if you already have session cookies:

```bash
# Generate token via API (requires session auth first)
curl -s -X POST "$CTF_URL/api/v1/tokens" \
  -H "Content-Type: application/json" \
  -b cookies.txt \
  -d '{"expiration": "2026-12-31", "description": "CLI access"}' | jq .
```

Use the token for all subsequent requests:

```bash
export CTF_URL="https://ctf.example.com"
export CTF_TOKEN="ctfd_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"

# Test authentication
curl -s -H "Authorization: Token $CTF_TOKEN" "$CTF_URL/api/v1/users/me" | jq .
```

### Method 2: Session Login (Cookie-Based)

```bash
# Step 1: Get CSRF nonce from login page
NONCE=$(curl -sc cookies.txt "$CTF_URL/login" | grep 'name="nonce"' | grep -oE 'value="[^"]*"' | cut -d'"' -f2)

# Step 2: Login with credentials
curl -sb cookies.txt -c cookies.txt -X POST "$CTF_URL/login" \
  -d "name=username&password=password&nonce=$NONCE" \
  -L -o /dev/null -w '%{http_code}'

# Step 3: Use cookies for API calls
curl -s -b cookies.txt "$CTF_URL/api/v1/users/me" | jq .
```

**Key insight:** The nonce is a CSRF token required for form-based login. API token auth bypasses this entirely — always prefer tokens when available.

---

## List Challenges

```bash
# All visible challenges
curl -s -H "Authorization: Token $CTF_TOKEN" "$CTF_URL/api/v1/challenges" | jq .

# Filter by category
curl -s -H "Authorization: Token $CTF_TOKEN" "$CTF_URL/api/v1/challenges?category=web" | jq .

# Compact listing: id, name, category, value, solves
curl -s -H "Authorization: Token $CTF_TOKEN" "$CTF_URL/api/v1/challenges" | \
  jq -r '.data[] | "\(.id)\t\(.value)pts\t\(.category)\t\(.name)\t(\(.solves) solves)"' | \
  sort -t$'\t' -k3,3 -k2,2rn | column -t -s$'\t'
```

**Response structure:**
```json
{
  "success": true,
  "data": [
    {
      "id": 1,
      "type": "standard",
      "name": "Challenge Name",
      "value": 100,
      "solves": 42,
      "solved_by_me": false,
      "category": "web",
      "tags": [],
      "template": "...",
      "script": "..."
    }
  ]
}
```

---

## Challenge Details

```bash
# Full challenge details (description, files, hints, tags)
curl -s -H "Authorization: Token $CTF_TOKEN" "$CTF_URL/api/v1/challenges/$CHALL_ID" | jq .

# Extract just the description (HTML)
curl -s -H "Authorization: Token $CTF_TOKEN" "$CTF_URL/api/v1/challenges/$CHALL_ID" | \
  jq -r '.data.description'

# Strip HTML tags for readable description
curl -s -H "Authorization: Token $CTF_TOKEN" "$CTF_URL/api/v1/challenges/$CHALL_ID" | \
  jq -r '.data.description' | sed 's/<[^>]*>//g'

# List files attached to challenge
curl -s -H "Authorization: Token $CTF_TOKEN" "$CTF_URL/api/v1/challenges/$CHALL_ID" | \
  jq -r '.data.files[]'

# Get connection info (if present in description)
curl -s -H "Authorization: Token $CTF_TOKEN" "$CTF_URL/api/v1/challenges/$CHALL_ID" | \
  jq -r '.data.description' | grep -oE '(nc |ssh |https?://)[^ <"]+' | head -5
```

---

## Download Challenge Files

CTFd serves files with token-signed URLs. Extract them from challenge details and download:

```bash
# Get file URLs from challenge
FILES=$(curl -s -H "Authorization: Token $CTF_TOKEN" "$CTF_URL/api/v1/challenges/$CHALL_ID" | \
  jq -r '.data.files[]')

# Download all challenge files
mkdir -p "chall_$CHALL_ID"
for f in $FILES; do
  # File paths are relative — prepend base URL
  URL="${CTF_URL}${f}"
  FILENAME=$(basename "$f" | sed 's/?.*//')
  curl -s -H "Authorization: Token $CTF_TOKEN" -o "chall_$CHALL_ID/$FILENAME" "$URL"
  echo "Downloaded: $FILENAME"
done
```

**Key insight:** File URLs include a query-string token (`?token=...`) that authenticates the download. The token is time-limited — re-fetch the challenge details if downloads return 403.

---

## Submit Flags

```bash
# Submit a flag
curl -s -X POST -H "Authorization: Token $CTF_TOKEN" \
  -H "Content-Type: application/json" \
  "$CTF_URL/api/v1/challenges/attempt" \
  -d "{\"challenge_id\": $CHALL_ID, \"submission\": \"flag{example}\"}" | jq .
```

**Response statuses:**
| Status | Meaning |
|--------|---------|
| `correct` | Flag accepted |
| `incorrect` | Wrong flag |
| `already_solved` | Previously solved by you/team |
| `ratelimited` | Too many attempts (default: 10/min) |
| `paused` | CTF is paused |

**Key insight:** Rate limit is 10 incorrect submissions per minute per user. Space out brute-force attempts or you get locked out temporarily.

---

## Scoreboard

```bash
# Full scoreboard
curl -s -H "Authorization: Token $CTF_TOKEN" "$CTF_URL/api/v1/scoreboard" | jq .

# Top 10
curl -s -H "Authorization: Token $CTF_TOKEN" "$CTF_URL/api/v1/scoreboard/top/10" | jq .

# Compact scoreboard
curl -s -H "Authorization: Token $CTF_TOKEN" "$CTF_URL/api/v1/scoreboard/top/20" | \
  jq -r '.data | to_entries[] | "\(.value.pos)\t\(.value.name)\t\(.value.score)pts"' | \
  column -t -s$'\t'
```

**Note:** Scoreboard is cached server-side for 60 seconds.

---

## Hints and Unlocks

```bash
# List hints for a challenge (from challenge details)
curl -s -H "Authorization: Token $CTF_TOKEN" "$CTF_URL/api/v1/challenges/$CHALL_ID" | \
  jq '.data.hints'

# Get hint content (if free or already unlocked)
curl -s -H "Authorization: Token $CTF_TOKEN" "$CTF_URL/api/v1/hints/$HINT_ID" | jq .

# Unlock a paid hint (costs points)
curl -s -X POST -H "Authorization: Token $CTF_TOKEN" \
  -H "Content-Type: application/json" \
  "$CTF_URL/api/v1/unlocks" \
  -d "{\"target\": $HINT_ID, \"type\": \"hints\"}" | jq .
```

---

## Notifications

```bash
# Get all notifications (announcements from organizers)
curl -s -H "Authorization: Token $CTF_TOKEN" "$CTF_URL/api/v1/notifications" | jq .

# Get notification count (HEAD request)
curl -sI -H "Authorization: Token $CTF_TOKEN" "$CTF_URL/api/v1/notifications" | \
  grep -i 'x-total'

# Poll for new notifications since last seen
curl -s -H "Authorization: Token $CTF_TOKEN" \
  "$CTF_URL/api/v1/notifications?since_id=$LAST_ID" | jq .
```

---

## User and Team Info

```bash
# Current user profile
curl -s -H "Authorization: Token $CTF_TOKEN" "$CTF_URL/api/v1/users/me" | jq .

# My solves
curl -s -H "Authorization: Token $CTF_TOKEN" "$CTF_URL/api/v1/users/me/solves" | \
  jq -r '.data[] | "\(.challenge.name)\t\(.challenge.value)pts\t\(.date)"'

# My failed attempts
curl -s -H "Authorization: Token $CTF_TOKEN" "$CTF_URL/api/v1/users/me/fails" | jq .

# Current team (teams mode)
curl -s -H "Authorization: Token $CTF_TOKEN" "$CTF_URL/api/v1/teams/me" | jq .

# Team solves
TEAM_ID=$(curl -s -H "Authorization: Token $CTF_TOKEN" "$CTF_URL/api/v1/teams/me" | jq '.data.id')
curl -s -H "Authorization: Token $CTF_TOKEN" "$CTF_URL/api/v1/teams/$TEAM_ID/solves" | jq .
```

---

## Full Competition Workflow

End-to-end CTFd interaction from the terminal:

```bash
#!/usr/bin/env bash
# CTFd CLI workflow — set these two variables and go
export CTF_URL="https://ctf.example.com"
export CTF_TOKEN="ctfd_your_token_here"
AUTH="-H 'Authorization: Token $CTF_TOKEN'"

# 1. Verify auth
echo "=== Logged in as ==="
curl -s -H "Authorization: Token $CTF_TOKEN" "$CTF_URL/api/v1/users/me" | jq -r '.data | "\(.name) (id: \(.id))"'

# 2. List all challenges grouped by category
echo -e "\n=== Challenges ==="
curl -s -H "Authorization: Token $CTF_TOKEN" "$CTF_URL/api/v1/challenges" | \
  jq -r '.data | sort_by(.category, -.value) | .[] |
    "\(.solved_by_me | if . then "✓" else " " end) \(.id)\t\(.value)pts\t\(.category)\t\(.name)"' | \
  column -t -s$'\t'

# 3. Read a specific challenge
read -p "Challenge ID: " CID
curl -s -H "Authorization: Token $CTF_TOKEN" "$CTF_URL/api/v1/challenges/$CID" | \
  jq -r '.data | "Name: \(.name)\nCategory: \(.category)\nValue: \(.value)\nSolves: \(.solves)\n\nDescription:\n\(.description)"' | \
  sed 's/<[^>]*>//g'

# 4. Download files
mkdir -p "chall_$CID"
curl -s -H "Authorization: Token $CTF_TOKEN" "$CTF_URL/api/v1/challenges/$CID" | \
  jq -r '.data.files[]' | while read -r f; do
    curl -s -H "Authorization: Token $CTF_TOKEN" -o "chall_$CID/$(basename "$f" | sed 's/?.*//')" "${CTF_URL}${f}"
  done
echo "Files downloaded to chall_$CID/"

# 5. Submit flag
read -p "Flag: " FLAG
curl -s -X POST -H "Authorization: Token $CTF_TOKEN" \
  -H "Content-Type: application/json" \
  "$CTF_URL/api/v1/challenges/attempt" \
  -d "{\"challenge_id\": $CID, \"submission\": \"$FLAG\"}" | \
  jq -r '.data | "\(.status): \(.message)"'
```

---

## Python CTFd Client

Reusable class for scripted interaction:

```python
import requests
import os
import re
from pathlib import Path


class CTFdClient:
    """Minimal CTFd API client for competition use."""

    def __init__(self, url, token):
        self.url = url.rstrip('/')
        self.s = requests.Session()
        self.s.headers['Authorization'] = f'Token {token}'

    def _get(self, path, **kwargs):
        r = self.s.get(f'{self.url}/api/v1{path}', **kwargs)
        r.raise_for_status()
        return r.json()

    def _post(self, path, json=None):
        r = self.s.post(f'{self.url}/api/v1{path}', json=json)
        r.raise_for_status()
        return r.json()

    # --- Challenges ---

    def challenges(self, category=None):
        """List all visible challenges."""
        params = {'category': category} if category else {}
        return self._get('/challenges', params=params)['data']

    def challenge(self, cid):
        """Get full challenge details."""
        return self._get(f'/challenges/{cid}')['data']

    def unsolved(self):
        """List challenges not yet solved by current user."""
        return [c for c in self.challenges() if not c.get('solved_by_me')]

    # --- Files ---

    def download_files(self, cid, dest='.'):
        """Download all files for a challenge."""
        info = self.challenge(cid)
        dest = Path(dest)
        dest.mkdir(parents=True, exist_ok=True)
        paths = []
        for f in info.get('files', []):
            url = f'{self.url}{f}' if f.startswith('/') else f
            fname = re.sub(r'\?.*', '', f.split('/')[-1])
            out = dest / fname
            r = self.s.get(url)
            r.raise_for_status()
            out.write_bytes(r.content)
            paths.append(str(out))
        return paths

    # --- Flag Submission ---

    def submit(self, cid, flag):
        """Submit a flag. Returns (status, message)."""
        resp = self._post('/challenges/attempt',
                          json={'challenge_id': cid, 'submission': flag})
        d = resp['data']
        return d['status'], d['message']

    # --- Scoreboard ---

    def scoreboard(self, top=10):
        """Get top N scoreboard entries."""
        return self._get(f'/scoreboard/top/{top}')['data']

    # --- User/Team ---

    def me(self):
        """Current user info."""
        return self._get('/users/me')['data']

    def my_solves(self):
        """Challenges solved by current user."""
        return self._get('/users/me/solves')['data']

    # --- Hints ---

    def hint(self, hint_id):
        """Get hint content (if unlocked or free)."""
        return self._get(f'/hints/{hint_id}')['data']

    def unlock_hint(self, hint_id):
        """Unlock a hint (costs points)."""
        return self._post('/unlocks', json={'target': hint_id, 'type': 'hints'})

    # --- Notifications ---

    def notifications(self, since_id=None):
        """Get announcements. Optionally filter since a notification ID."""
        params = {'since_id': since_id} if since_id else {}
        return self._get('/notifications', params=params)['data']


# --- Usage ---

if __name__ == '__main__':
    c = CTFdClient(os.environ['CTF_URL'], os.environ['CTF_TOKEN'])

    # Dashboard
    print(f"Logged in as: {c.me()['name']}")
    print(f"\nUnsolved challenges:")
    for ch in c.unsolved():
        print(f"  [{ch['id']}] {ch['category']}/{ch['name']} ({ch['value']}pts, {ch['solves']} solves)")

    # Download and submit workflow
    # files = c.download_files(1, dest='chall_1')
    # status, msg = c.submit(1, 'flag{...}')
    # print(f"{status}: {msg}")
```

---

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| 401 Unauthorized | Token expired or invalid | Re-generate token via web UI or session login |
| 403 on file download | File token expired | Re-fetch challenge details to get fresh file URLs |
| 403 on challenges | CTF not started or email unverified | Check `/api/v1/users/me` for `verified` field |
| 429 Rate Limited | Too many wrong flag submissions | Wait 60 seconds; default is 10 incorrect/min |
| Empty challenge list | CTF hasn't started | Check CTF start time in notifications or config |
| `nonce` missing | Login page changed or anti-bot | Try API token auth instead of session login |
| Connection info not in API | Some CTFs use dynamic instances | Check for challenge-specific instance API or Docker endpoints |
