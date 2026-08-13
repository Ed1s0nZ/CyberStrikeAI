---
name: ctf-osint
description: Provides open source intelligence techniques for CTF challenges. Use when gathering information from public sources, social media, geolocation, DNS records, username enumeration, reverse image search, Google dorking, Wayback Machine, Tor relays, FEC filings, or identifying unknown data like hashes and coordinates.
license: MIT
compatibility: Requires filesystem-based agent (Claude Code or similar) with bash, Python 3, and internet access for OSINT lookups.
allowed-tools: Bash Read Write Edit Glob Grep Task WebFetch WebSearch
metadata:
  user-invocable: "false"
---

# CTF OSINT

Quick reference for OSINT CTF challenges. Each technique has a one-liner here; see supporting files for full details.

## Prerequisites

**Python packages (all platforms):**
```bash
pip install shodan Pillow
```

**Linux (apt):**
```bash
apt install whois dnsutils nmap libimage-exiftool-perl imagemagick curl
```

**macOS (Homebrew):**
```bash
brew install whois bind nmap exiftool imagemagick curl
```

## Additional Resources

- [social-media.md](social-media.md) - Twitter/X (user IDs, Snowflake timestamps, Nitter, memory.lol, Wayback CDX), Tumblr (blog checks, post JSON, avatars), BlueSky search + API, Unicode homoglyph steganography, Discord API, username OSINT (namechk, whatsmyname, Osint Industries), username metadata mining (postal codes), platform false positives, multi-platform chains, Strava fitness route OSINT
- [geolocation-and-media.md](geolocation-and-media.md) - Image analysis, reverse image search (including Baidu for China), Google Lens cropped region search, reflected/mirrored text reading, geolocation techniques (railroad signs, infrastructure maps, MGRS), Google Plus Codes, EXIF/metadata, hardware identification, newspaper archives, IP geolocation, Google Street View panorama matching, What3Words micro-landmark matching, Google Maps crowd-sourced photo verification, Overpass Turbo spatial queries, music-themed landmark geolocation with key encoding
- [web-and-dns.md](web-and-dns.md) - Google dorking (including TBS image filters), Google Docs/Sheets enumeration, DNS recon (TXT, zone transfers), Wayback Machine, FEC research, Tor relay lookups, GitHub repository analysis, Telegram bot investigation, WHOIS investigation (reverse WHOIS, historical WHOIS, IP/ASN lookup), fake service banner detection via nmap fingerprinting

---

## When to Pivot

- If you already have the files or packets locally and now need extraction or carving, switch to `/ctf-forensics`.
- If the task becomes active exploitation of a live HTTP service, switch to `/ctf-web`.
- If you uncover malware samples, beacons, or suspicious binaries during attribution, switch to `/ctf-malware`.

## Quick Start Commands

```bash
# DNS recon
dig -t any target.com
dig -t txt target.com
dig axfr @ns.target.com target.com
whois target.com

# Image metadata
exiftool image.jpg
identify -verbose image.jpg | head -30

# Web archive
curl "https://web.archive.org/web/20230101*/target.com"

# Username lookup
curl -s "https://whatsmyname.app/api/lookup?username=<user>"

# Shodan
shodan search "hostname:target.com"
shodan host <ip>
```

## String Identification

- 40 hex chars -> SHA-1 (Tor fingerprint)
- 64 hex chars -> SHA-256
- 32 hex chars -> MD5

## Twitter/X Account Tracking

- Persistent numeric User ID: `https://x.com/i/user/<id>` works even after renames.
- Snowflake timestamps: `(id >> 22) + 1288834974657` = Unix ms.
- Wayback CDX, Nitter, memory.lol for historical data. See [social-media.md](social-media.md).

## Tumblr Investigation

- Blog check: `curl -sI` for `x-tumblr-user` header. Avatar at `/avatar/512`. See [social-media.md](social-media.md).

## Username OSINT

- [whatsmyname.app](https://whatsmyname.app) (741+ sites), [namechk.com](https://namechk.com). Watch for platform false positives. See [social-media.md](social-media.md).

## Image Analysis & Reverse Image Search

- Google Lens (crop to region of interest), Google Images, TinEye, Yandex (faces). Check corners for visual stego. Twitter strips EXIF. See [geolocation-and-media.md](geolocation-and-media.md).
- **Cropped region search:** Isolate distinctive elements (shop signs, building facades) and search via Google Lens for better results than full-scene search. See [geolocation-and-media.md](geolocation-and-media.md).
- **Reflected text:** Flip mirrored/reflected text (water, glass) horizontally; search partial text with quoted strings. See [geolocation-and-media.md](geolocation-and-media.md).

## Geolocation

- Railroad signs, infrastructure maps (OpenRailwayMap, OpenInfraMap), process of elimination. See [geolocation-and-media.md](geolocation-and-media.md).
- **Street View panorama matching:** Feature extraction + multi-metric image similarity ranking against candidate panoramas. Useful when challenge image is a crop of a Street View photo. See [geolocation-and-media.md](geolocation-and-media.md).
- **Road sign OCR:** Extract text from directional signs (town names, route numbers) to pinpoint road corridors. Driving side + sign style + script identify the country. See [geolocation-and-media.md](geolocation-and-media.md).
- **Architecture + brand identification:** Post-Soviet concrete = Russia/CIS; named businesses → search locations/branches → cross-reference with coastline/terrain. See [geolocation-and-media.md](geolocation-and-media.md).
- **Music-themed landmark geolocation:** Multiple images of music-related landmarks worldwide; each yields a piano key number encoding one flag character. Identify all locations first, then decode the key sequence. See [geolocation-and-media.md](geolocation-and-media.md).

## MGRS Coordinates

- Grid format "4V FH 246 677" -> online converter -> lat/long -> Google Maps. See [geolocation-and-media.md](geolocation-and-media.md).

## Google Plus Codes

- Format `XXXX+XXX` (chars: `23456789CFGHJMPQRVWX`). Drop a pin on Google Maps → Plus Code appears in details. Free, no API key needed. See [geolocation-and-media.md](geolocation-and-media.md).

## Metadata Extraction

```bash
exiftool image.jpg           # EXIF data
pdfinfo document.pdf         # PDF metadata
mediainfo video.mp4          # Video metadata
```

## Google Dorking

```text
site:example.com filetype:pdf
intitle:"index of" password
```

**Image TBS filters:** Append `&tbs=itp:face` to Google Image URLs to filter for faces only (strips logos/banners). See [web-and-dns.md](web-and-dns.md).

## Google Docs/Sheets

- Try `/export?format=csv`, `/pub`, `/gviz/tq?tqx=out:csv`, `/htmlview`. See [web-and-dns.md](web-and-dns.md).

## DNS Reconnaissance

```bash
dig -t txt subdomain.ctf.domain.com
dig axfr @ns.domain.com domain.com  # Zone transfer
```

Always check TXT, CNAME, MX for CTF domains. See [web-and-dns.md](web-and-dns.md).

## Tor Relay Lookups

- `https://metrics.torproject.org/rs.html#simple/<FINGERPRINT>` -- check family, sort by "first seen". See [web-and-dns.md](web-and-dns.md).

## GitHub Repository Analysis

- Check issue comments, PR reviews, commit messages, wiki edits via `gh api`. See [web-and-dns.md](web-and-dns.md).

## Telegram Bot Investigation

- Find bot references in browser history, interact via `/start`, answer verification questions. See [web-and-dns.md](web-and-dns.md).

## FEC Political Donation Research

- FEC.gov for committee receipts; 501(c)(4) orgs obscure original funders. See [web-and-dns.md](web-and-dns.md).

## IP Geolocation

```bash
curl "http://ip-api.com/json/103.150.68.150"
```

See [geolocation-and-media.md](geolocation-and-media.md).

## Unicode Homoglyph Steganography

**Pattern:** Visually-identical Unicode characters from different blocks (Cyrillic, Greek, Math) encode binary data in social media posts. ASCII = 0, homoglyph = 1. Group bits into bytes for flag. See [social-media.md](social-media.md#unicode-homoglyph-steganography-on-bluesky-metactf-2026).

## BlueSky Public API

No auth needed. Endpoints: `public.api.bsky.app/xrpc/app.bsky.feed.searchPosts?q=...`, `app.bsky.actor.searchActors`, `app.bsky.feed.getAuthorFeed`. Check all replies to official posts. See [social-media.md](social-media.md#unicode-homoglyph-steganography-on-bluesky-metactf-2026).

## Fake Service Banner Detection

**Pattern:** Port appears open on a standard service port (22/SSH, 80/HTTP) but runs a fake service. `nmap -sV` or `nc host port` reveals the flag in the banner. Never trust port numbers alone -- always fingerprint the service. See [web-and-dns.md](web-and-dns.md#fake-service-banner-detection-via-fingerprinting-metactf-flash-2026).

## Shodan SSH Fingerprint Lookup

Search Shodan by SSH host key fingerprint to identify servers: `shodan search "fingerprint:AA:BB:CC:..."`. See [web-and-dns.md](web-and-dns.md#shodan-ssh-fingerprint-lookup-ekoparty-ctf-2016).

## Gaming Platform OSINT

Lookup usernames across gaming platforms (Steam, Xbox, PSN, MMOs) for character profiles, activity, and linked accounts. See [social-media.md](social-media.md#gaming-platform-osint--mmo-character-lookup-csaw-ctf-2016).

## Resources

- **Shodan** - Internet-connected devices
- **Censys** - Certificate and host search
- **VirusTotal** - File/URL reputation
- **WHOIS** - Domain registration
- **Wayback Machine** - Historical snapshots
