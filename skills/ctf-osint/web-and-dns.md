# Web and DNS OSINT

## Table of Contents
- [Google Dorking](#google-dorking)
- [Google Docs/Sheets in OSINT](#google-docssheets-in-osint)
- [DNS Reconnaissance](#dns-reconnaissance)
- [DNS TXT Record OSINT](#dns-txt-record-osint)
- [Tor Relay Lookups](#tor-relay-lookups)
- [GitHub Repository Comments](#github-repository-comments)
- [Telegram Bot Investigation](#telegram-bot-investigation)
- [FEC Political Donation Research](#fec-political-donation-research)
- [Wayback Machine](#wayback-machine)
- [WHOIS Investigation](#whois-investigation)
- [Shodan SSH Fingerprint Lookup (EKOPARTY CTF 2016)](#shodan-ssh-fingerprint-lookup-ekoparty-ctf-2016)
- [Fake Service Banner Detection via Fingerprinting (MetaCTF Flash 2026)](#fake-service-banner-detection-via-fingerprinting-metactf-flash-2026)
- [Git Commit Author Mining for Credentials (Hackover 2018)](#git-commit-author-mining-for-credentials-hackover-2018)
- [.DS_Store Directory Enumeration with Python-dsstore (35C3 2018)](#ds_store-directory-enumeration-with-python-dsstore-35c3-2018)
- [TTF Glyph Contour Diffing for Obfuscated CAPTCHA (Square CTF 2018)](#ttf-glyph-contour-diffing-for-obfuscated-captcha-square-ctf-2018)
- [Cross-Challenge Container IP Reuse (RITSEC 2018)](#cross-challenge-container-ip-reuse-ritsec-2018)
- [Resources](#resources)

---

## Google Dorking

```text
site:example.com filetype:pdf
intitle:"index of" password
inurl:admin
"confidential" filetype:doc
```

**Google Image TBS (To Be Searched) parameters:**

Append `&tbs=` filters to Google Image search URLs for precision filtering:

| Filter | Parameter | Example |
|--------|-----------|---------|
| Faces only | `itp:face` | Find profile photos |
| Clipart | `itp:clipart` | Logos, icons |
| Animated GIF | `itp:animated` | Animated images |
| Specific color | `ic:specific,isc:green` | Dominant color filter |
| Transparent BG | `ic:trans` | PNGs with transparency |
| Large images | `isz:l` | High resolution only |
| Min resolution | `isz:lt,islt:2mp` | Greater than 2 megapixels |

**Combined example:** Search LinkedIn for face photos of interns at a company:
```text
https://www.google.com/search?q="orange"+"alternant"+site:linkedin.com&tbm=isch&tbs=itp:face
```

**Key insight:** The `itp:face` filter is especially useful for OSINT — it strips out logos, banners, and UI screenshots from results, leaving only profile photos. Combine with `site:` and date range (`after:YYYY-MM-DD`) for targeted reconnaissance.

## Google Docs/Sheets in OSINT

- Suspects may link to Google Sheets/Docs in tweets or posts
- Try public access URLs:
  - `/export?format=csv` - Export as CSV
  - `/pub` - Published version
  - `/gviz/tq?tqx=out:csv` - Visualization API CSV export
  - `/htmlview` - HTML view
- Private sheets require authentication; flag may be in the sheet itself
- Sheet IDs are stable identifiers even if sharing settings change

## DNS Reconnaissance

Flags often in TXT records of subdomains, not root domain:
```bash
dig -t txt subdomain.ctf.domain.com
dig -t any domain.com
dig axfr @ns.domain.com domain.com  # Zone transfer
```

## DNS TXT Record OSINT

```bash
dig TXT ctf.domain.org
dig TXT _dmarc.domain.org
dig ANY domain.org
```

**Lesson:** DNS TXT records are publicly queryable. Always check TXT, CNAME, MX for CTF domains and subdomains.

## Tor Relay Lookups

```text
https://metrics.torproject.org/rs.html#simple/<FINGERPRINT>
```
Check family members and sort by "first seen" date for ordered flags.

## GitHub Repository Comments

**Pattern (Rogue, VuwCTF 2025):** Hidden information in GitHub repo comments (issue comments, PR reviews, commit messages, wiki edits).

**Check:** `gh api repos/OWNER/REPO/issues/comments`, `gh api repos/OWNER/REPO/commits`, wiki edit history.

## Telegram Bot Investigation

**Pattern:** Forensic artifacts (browser history, chat logs) may reference Telegram bots that require active interaction.

**Finding bot references in forensics:**
```python
# Search browser history for Telegram URLs
import sqlite3
conn = sqlite3.connect("History")  # Edge/Chrome history DB
cur = conn.cursor()
cur.execute("SELECT url FROM urls WHERE url LIKE '%t.me/%'")
# Example: https://t.me/comrade404_bot
```

**Bot interaction workflow:**
1. Visit `https://t.me/<botname>` -> Opens in Telegram
2. Start conversation with `/start` or bot's custom command
3. Bot may require verification (CTF-style challenges)
4. Answers often require knowledge from forensic analysis

**Verification question patterns:**
- "Which user account did you use for X?" -> Check browser history, login records
- "Which account was modified?" -> Check Security.evtx Event 4781 (rename)
- "What file did you access?" -> Check MRU, Recent files, Shellbags

**Example bot flow:**
```text
Bot: "TIER 1: Which account used for online search?"
-> Answer from Edge history showing Bing/Google searches

Bot: "TIER 2: Which account name did you change?"
-> Answer from Security event log (account rename events)

Bot: [Grants access] "Website: http://x.x.x.x:5000, Username: mehacker, Password: flaghere"
```

**Key insight:** Bot responses may reveal:
- Attacker's real identity/handle
- Credentials to secondary systems
- Direct flag components
- Links to hidden web services

## FEC Political Donation Research

**Pattern (Shell Game):** Track organizational donors through FEC filings.

**Key resources:**
- [FEC.gov](https://www.fec.gov/data/) - Committee receipts and expenditures
- 501(c)(4) organizations can donate to Super PACs without disclosing original funders
- Look for largest organizational donors, then research org leadership (CEO/President)

## Wayback Machine

```bash
# Find all archived URLs for a site
curl "http://web.archive.org/cdx/search/cdx?url=example.com*&output=json&fl=timestamp,original,statuscode"
```

- Check for deleted posts, old profiles, cached pages
- CDX API for programmatic access to archive index

## WHOIS Investigation

```bash
# Basic WHOIS lookup
whois example.com

# Key fields to extract:
# - Registrant name/email/org (often redacted by privacy services)
# - Creation/expiration dates (timeline correlation)
# - Name servers (shared hosting identification)
# - Registrar (can indicate sophistication level)

# Historical WHOIS (before privacy was enabled)
# Use SecurityTrails, WhoisXML API, or DomainTools
curl "https://api.securitytrails.com/v1/domain/example.com/whois" \
  -H "APIKEY: YOUR_KEY"

# Reverse WHOIS — find all domains registered by same entity
# Search by registrant email, org name, or phone number
curl "https://reverse-whois-api.whoisxmlapi.com/api/v2" \
  -d '{"searchType":"current","mode":"purchase","basicSearchTerms":{"include":["target@email.com"]}}'

# IP WHOIS (find network owner)
whois 1.2.3.4
# Look for: NetName, OrgName, CIDR range, abuse contact

# ASN lookup
whois -h whois.radb.net AS12345
# Or use bgp.tools: https://bgp.tools/as/12345
```

**Key insight:** WHOIS data is most useful for timeline correlation (when was the domain registered relative to CTF events?), reverse lookups (what other domains share the same registrant?), and identifying shared infrastructure. Historical WHOIS via SecurityTrails or Wayback Machine can reveal pre-privacy registrant details.

---

## Shodan SSH Fingerprint Lookup (EKOPARTY CTF 2016)

Discover the real IP behind a Tor hidden service or CDN by searching Shodan for the service's SSH fingerprint.

```bash
# Step 1: Get SSH fingerprint from target
ssh-keyscan -t rsa target.onion 2>/dev/null | ssh-keygen -lf - -E md5
# Or use a dedicated scanner:
# pip install ssh-audit
ssh-audit target.onion

# Step 2: Extract the fingerprint hash
# e.g., MD5:ab:cd:ef:12:34:56:78:90:ab:cd:ef:12:34:56:78:90

# Step 3: Search Shodan for matching fingerprint
# Via API:
import shodan
api = shodan.Shodan('YOUR_API_KEY')
results = api.search('ssh.fingerprint:"ab:cd:ef:12:34:56:78:90:ab:cd:ef:12:34:56:78:90"')
for result in results['matches']:
    print(f"IP: {result['ip_str']}")
    print(f"Port: {result['port']}")
    print(f"Banner: {result['data'][:200]}")

# Via Shodan CLI:
shodan search 'ssh.fingerprint:"ab:cd:ef:12:34:56:78:90"'

# Via web: https://www.shodan.io/search?query=ssh.fingerprint:%22...%22

# Also works with TLS certificate fingerprints:
# shodan search 'ssl.cert.fingerprint:"SHA256_HASH"'
```

**Key insight:** SSH host keys are unique per server. If a hidden service runs SSH, its fingerprint can be searched on Shodan/Censys to find the real IP. This technique also works to de-anonymize services behind CloudFlare or other CDNs. Search both SSH fingerprints and TLS certificate fingerprints.

---

## Fake Service Banner Detection via Fingerprinting (MetaCTF Flash 2026)

**Pattern (O-Syn-T):** A port appears open on a standard service port (e.g., 22/SSH), but the service behind it is not what it claims. A basic SYN scan reports the port as open, but service version detection reveals a fake or custom banner containing the flag.

```bash
# Step 1: Basic port scan finds port 22 open
nmap -sS target.ctf
# PORT   STATE SERVICE
# 22/tcp open  ssh

# Step 2: Service version fingerprinting reveals the deception
nmap -sV -sC target.ctf -p 22
# PORT   STATE SERVICE VERSION
# 22/tcp open  ssh?
# |_banner: MetaCTF{fake_banner_flag_here}

# Step 3: Or simply connect with netcat to read the banner
nc target.ctf 22
# MetaCTF{fake_banner_flag_here}

# Alternative: use curl or openssl for TLS-wrapped banners
echo "" | timeout 3 nc -w 3 target.ctf 22
```

**Key insight:** Never trust port numbers alone. A SYN scan only confirms the port is open, not what service is running. Always run `nmap -sV` (version detection) or connect with `nc` to read the actual banner. CTF challenges exploit the assumption that port 22 = SSH, port 80 = HTTP, etc. Custom banner services on standard ports are a common OSINT/network recon trick.

**When to recognize:** Challenge name hints at network scanning or reconnaissance ("SYN", "scan", "port"). The expected approach is to enumerate open ports, but the flag is in the service banner itself rather than requiring exploitation.

---

## Git Commit Author Mining for Credentials (Hackover 2018)

**Pattern:** A challenge mentions a username with no credentials and expects the attacker to pivot to a public repository (GitHub/GitLab/Bitbucket) owned by that user. `git shortlog -sne` or `git log --format="%an <%ae>"` extracts every author email from the commit history — that address is often the valid login username the target service expects, before you attempt any password-reset or SQL-injection flow.

```bash
# Clone the target's public repo and list every contributor email
git clone https://github.com/<target-user>/<repo>.git
cd repo
git shortlog -sne
# 23  John Doe <[email protected]>
#  5  John Doe <[email protected]>     ← often the real login

# Pull every historic author at once:
git log --format="%an <%ae>%n%cn <%ce>" | sort -u
```

```bash
# GitHub-wide enumeration — list every event from a user
gh api "users/<target-user>/events/public" --paginate \
   | jq -r '.[] | .payload.commits[]?.author.email' | sort -u
```

**Key insight:** A git repo is a signed audit log of every author, committer, and co-author who has ever touched it. Even after someone rotates an email, the history keeps the old addresses. Mine both `author.email` and `committer.email`, and also look at `.mailmap`, `CONTRIBUTORS`, and GPG-signed commits (`git log --show-signature`). Treat each recovered email as a candidate login for the target service — many CTF web boxes, HR portals, and password-reset flows accept author emails straight from a public repo.

**References:** Hackover CTF 2018 — who knows john dows?, writeups 11537, 11646

---

## .DS_Store Directory Enumeration with Python-dsstore (35C3 2018)

**Pattern:** macOS `.DS_Store` files leak directory listings even when the web server hides them behind `robots.txt` or obscured paths. Download `.DS_Store` wherever possible (root, `/uploads/`, `/static/`) and parse it to enumerate filenames that are otherwise un-guessable.

```bash
curl -sO https://target/.DS_Store
python3 -m dsstore .DS_Store
# prints every file the Finder ever saw in that directory
```

**Key insight:** `.DS_Store` is generated automatically by macOS and often pushed to production by accident. It exposes filenames, not contents, but that is enough to find hidden admin panels, backup files, and uploaded flags.

**References:** 35C3 CTF 2018 — McDonald, writeup 12763

---

## TTF Glyph Contour Diffing for Obfuscated CAPTCHA (Square CTF 2018)

**Pattern:** A CAPTCHA serves obfuscated characters by remapping glyph IDs to random `cmap` entries, so the browser still displays "5" but the underlying Unicode codepoint is `U+E042`. Extract the TTF, dump each glyph's contours with `ttx`, build a reference library of known digit/letter contours, and `diff` incoming glyphs against the library to recover the true character.

```bash
ttx -t glyf -g -d glyph_out font.ttf
# glyph_out/font.glyf/<glyph_name>.ttx holds contour XML
diff glyph_out/font.glyf/zero.ttx reference/zero.ttx
```

**Key insight:** Visual CAPTCHAs that rely on custom fonts are trivially defeatable because the glyph *shapes* are invariant under cmap remapping. Build the reference once from any standard font and reuse it across every challenge variant.

**References:** Square CTF 2018 — C8, writeup 12161

---

## Cross-Challenge Container IP Reuse (RITSEC 2018)

**Pattern:** In Docker-hosted CTF infrastructures, all challenges in the same subnet often share an internal IP range. Leak the container's `REMOTE_ADDR` or routing table from one challenge (typically via a command-injection or SSRF), then apply that leaked IP to any other challenge that gates on `REMOTE_ADDR` hashes, `X-Forwarded-For` checks, or MD5(IP)-based upload paths.

```text
# Challenge A leaks REMOTE_ADDR = 10.0.10.254
# Challenge B expects upload at /uploads/md5(10.0.10.254)/md5(time()).ext
```

**Key insight:** Multi-challenge CTFs often leak infrastructure details cross-challenge. Always map the shared subnet first, then pivot info from the weakest challenge to the most constrained one.

**References:** RITSEC CTF 2018 — Lazy Dev → Archivr, writeups 12234-12235

---

## Resources

- **Shodan** - Internet-connected devices
- **Censys** - Certificate and host search
- **VirusTotal** - File/URL reputation
- **WHOIS** - Domain registration
- **Wayback Machine** - Historical snapshots
