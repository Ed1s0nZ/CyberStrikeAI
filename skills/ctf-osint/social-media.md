# Social Media OSINT

## Table of Contents
- [Twitter/X Account Tracking](#twitterx-account-tracking)
- [Tumblr Investigation](#tumblr-investigation)
- [BlueSky Advanced Search](#bluesky-advanced-search)
- [Username OSINT](#username-osint)
- [Platform False Positives](#platform-false-positives)
- [Social Media General Tips](#social-media-general-tips)
- [Multi-Platform OSINT Chain](#multi-platform-osint-chain)
- [Gaming Platform OSINT / MMO Character Lookup (CSAW CTF 2016)](#gaming-platform-osint--mmo-character-lookup-csaw-ctf-2016)
- [MetaCTF OSINT Challenge Patterns](#metactf-osint-challenge-patterns)
- [Unicode Homoglyph Steganography on BlueSky (MetaCTF 2026)](#unicode-homoglyph-steganography-on-bluesky-metactf-2026)
- [Strava Fitness Route OSINT (MidnightCTF 2026)](#strava-fitness-route-osint-midnightctf-2026)
- [Discord API Enumeration](#discord-api-enumeration)

---

## Twitter/X Account Tracking

**Persistent numeric User ID (key technique):**
- Every Twitter/X account has a permanent numeric ID that never changes
- Access any account by ID: `https://x.com/i/user/<numeric_id>` -- works even after username changes
- Find user ID from archived pages (JSON-LD `"author":{"identifier":"..."}`)
- Useful when username is deleted/changed but you have the ID from forensic artifacts

**Username rename detection:**
- Twitter User IDs persist across username changes; t.co shortlinks point to OLD usernames
- Wayback CDX API to find archived profiles: `http://web.archive.org/cdx/search/cdx?url=twitter.com/USERNAME*&output=json`
- Archived pages contain JSON-LD with user ID, creation date, follower/following counts
- t.co links in archived tweets reveal previous usernames (the redirect URL contains the username at time of posting)
- Same tweet ID accessible under different usernames = confirmed rename

**Alternative Twitter data sources:**
- Nitter instances (e.g., `nitter.poast.org/USERNAME`) show tweets without login
- Syndication API: `https://syndication.twitter.com/srv/timeline-profile/screen-name/USERNAME`
- Twitter Snowflake IDs encode timestamps: `(id >> 22) + 1288834974657` = Unix ms
- memory.lol and twitter.lolarchiver.com track username history

**Wayback Machine for Twitter:**
```bash
# Find all archived URLs for a username
curl "http://web.archive.org/cdx/search/cdx?url=twitter.com/USERNAME*&output=json&fl=timestamp,original,statuscode"

# Also check profile images
curl "http://web.archive.org/cdx/search/cdx?url=pbs.twimg.com/profile_images/*&output=json"

# Check t.co shortlinks
curl "http://web.archive.org/cdx/search/cdx?url=t.co/SHORTCODE&output=json"
```

## Tumblr Investigation

**Blog existence check:**
- `curl -sI "https://USERNAME.tumblr.com"` -> look for `x-tumblr-user` header (confirms blog exists even if API returns 401)
- Tumblr API may return 401 (Unauthorized) but the blog is still publicly viewable via browser

**Extracting post content from Tumblr HTML:**
- Tumblr embeds post data as JSON in the page HTML
- Search for `"content":[` to find post body data
- Posts contain `type: "text"` with `text` field, and `type: "image"` with media URLs
- Avatar URL pattern: `https://64.media.tumblr.com/HASH/HASH-XX/s512x512u_c1/FILENAME.jpg`

**Avatar as flag container:**
- Direct avatar endpoint: `https://api.tumblr.com/v2/blog/USERNAME.tumblr.com/avatar/512`
- Or simply: `https://USERNAME.tumblr.com/avatar/512` (redirects to CDN URL)
- Available sizes: 16, 24, 30, 40, 48, 64, 96, 128, 512
- Flags may be hidden as small text in avatar images (visual stego, not binary stego)
- Always download highest resolution (512) and zoom in on all areas

## BlueSky Advanced Search

**Pattern (Ms Blue Sky):** Find target's posts on BlueSky social media.

**Search filters:**
```text
from:username        # Posts from specific user
since:2025-01-01     # Date range
has:images           # Posts with images
```

**Reference:** https://bsky.social/about/blog/05-31-2024-search

## Username OSINT

- [namechk.com](https://namechk.com) - Check username across platforms
- [whatsmyname.app](https://whatsmyname.app) - Username enumeration (741+ sites)
- [Osint Industries](https://osint.industries) - Cross-platform people search (paid, covers fitness/niche platforms)
- Search `"username"` in quotes on major platforms

**Username metadata mining:**
Usernames often embed geographic or temporal signals in their structure. Extract and research numeric suffixes, prefixes, or embedded patterns:

| Pattern | Example | Signal |
|---------|---------|--------|
| Trailing digits = postal/ZIP code | `LinXiayu35170` | 35170 = Bruz, France |
| Birth year suffix | `jsmith1998` | Born 1998 |
| Area code | `user212nyc` | 212 = Manhattan |
| Country code | `player44uk` | +44 = United Kingdom |

Cross-reference extracted codes with postal code databases, phone number registries, or geographic gazetteers to narrow the subject's location. (MidnightCTF 2026)

**Username chain tracing (account renames):**
1. Start with known username -> find Wayback archives
2. Look for t.co links or cross-references to other usernames in archived pages
3. Discovered new username -> enumerate across ALL platforms again
4. Repeat until you find the platform with the flag

**Priority platforms for CTF username enumeration:**
- Twitter/X, Tumblr, GitHub, Reddit, Bluesky, Mastodon
- Spotify, SoundCloud, Steam, Keybase
- Strava, Garmin Connect, MapMyRun (fitness/GPS — leak physical locations)
- Pastebin, LinkedIn, YouTube, TikTok
- bio-link services (linktr.ee, bio.link, about.me)

## Platform False Positives

Platforms that return 200 but no real profile:
- Telegram (`t.me/USER`): Always returns 200 with "Contact @USER" page; check for "View" vs "Contact" in title
- TikTok: Returns 200 with "Couldn't find this account" in body
- Smule: Returns 200 with "Not Found" in page content
- linkin.bio: Redirects to Later.com product page for unclaimed names
- Instagram: Returns 200 but shows login wall (may or may not exist)

## Social Media General Tips

- Check Wayback Machine for deleted posts on Bluesky, Twitter, etc.
- Unlisted YouTube videos may be linked in deleted posts
- Bio links lead to itch.io, personal sites with more info
- Search `"username"` with quotes on platform-specific searches
- Challenge titles are often hints (e.g., "Linked Traces" -> LinkedIn / linked accounts)
- **Twitter strips EXIF** on upload - don't waste time on stego for Twitter-served images
- **Tumblr preserves more metadata** in avatars than in post images

## Multi-Platform OSINT Chain

**Pattern (Massive-Equipment393):** Reddit username -> Spotify social link -> Base58-encoded string -> Spotify playlist descriptions (base64) -> first-letter acrostic from song titles.

**Key techniques:**
- Base58 decoding for non-standard encodings
- Spotify playlists encode data in descriptions and song title initials
- Platform chaining: each platform links to the next

## Gaming Platform OSINT / MMO Character Lookup (CSAW CTF 2016)

CTF OSINT challenges may require looking up game characters, guilds, or profiles across MMO platforms.

```text
# World of Warcraft character/guild lookup:
# - Blizzard API: https://develop.battle.net/documentation/world-of-warcraft
# - WoW Progress: https://www.wowprogress.com
# - Raider.IO: https://raider.io
# Search: guild name + realm name (e.g., "Blackfathom Deep Dish" on US-Turalyon)

# Steam profile search:
# - steamcommunity.com/id/[username]
# - steamid.io for SteamID lookups

# Minecraft player lookup:
# - NameMC: https://namemc.com
# - Shows skin, name history, servers

# Discord user lookup:
# - discord.id for user/server lookups
# - Bot: UserInfo for detailed profiles

# Gaming OSINT chain pattern:
# 1. Blog/Twitter mentions guild or game name
# 2. Look up guild on game-specific tracker site
# 3. Find character name from guild roster
# 4. Character name may be used on other platforms
# 5. Cross-reference with other OSINT findings
```

**Key insight:** Gaming profiles are often overlooked in OSINT but contain rich metadata (play times, real names, linked accounts, server regions). Guild/clan trackers index public game APIs and cache historical data. Character names are frequently reused across platforms.

---

## MetaCTF OSINT Challenge Patterns

**Common flow:**
1. Start image with hidden EXIF/metadata -> extract username
2. Username enumeration (Sherlock/WhatsMyName) across platforms
3. Find profile on platform X with clues pointing to platform Y
4. Flag hidden on the final platform (Spotify bio, BlueSky post, Tumblr avatar, etc.)

**Platform-specific flag locations:**
- Spotify: playlist names, artist bio
- BlueSky: post content
- Tumblr: avatar image, post text
- Reddit: post/comment content
- Smule: song recordings or bio
- SoundCloud: track description

**Key techniques:**
- Account rename tracking via Wayback + t.co links
- Cross-platform username correlation
- Visual inspection of all profile images at max resolution
- Song lyric identification -> artist/song as flag component

## Unicode Homoglyph Steganography on BlueSky (MetaCTF 2026)

**Pattern (Skybound Secrets):** Flag hidden in a Bluesky post using Unicode homoglyph steganography — visually identical characters from different Unicode blocks encode binary data.

**Detection:**
- Post text looks normal but character-by-character analysis reveals non-ASCII codepoints
- Characters from Cyrillic (`а` U+0430 vs `a` U+0061), Greek, Armenian, Mathematical Monospace, etc.
- Each character encodes 1 bit: ASCII = 0, homoglyph = 1

**Bluesky API search workflow:**
```bash
# Search for posts about the CTF
curl -s "https://public.api.bsky.app/xrpc/app.bsky.feed.searchPosts?q=metactf+flash+ctf&sort=latest" | jq '.posts[].record.text'

# Search for specific accounts
curl -s "https://public.api.bsky.app/xrpc/app.bsky.actor.searchActors?q=metactf" | jq '.actors[].handle'

# Get profile
curl -s "https://public.api.bsky.app/xrpc/app.bsky.actor.getProfile?actor=metactf.bsky.social" | jq

# Get author feed (all posts)
curl -s "https://public.api.bsky.app/xrpc/app.bsky.feed.getAuthorFeed?actor=metactf.bsky.social&limit=50" | jq '.feed[].post.record.text'

# Get post thread (including replies)
curl -s "https://public.api.bsky.app/xrpc/app.bsky.feed.getPostThread?uri=at://did:plc:.../app.bsky.feed.post/..." | jq
```

**Decoding homoglyph steganography:**
```python
def decode_homoglyph_stego(text):
    bits = []
    for ch in text:
        if ch in ('\u2019',):  # Platform auto-inserted right single quote
            continue  # Skip, not intentional homoglyph
        if ord(ch) < 128:
            bits.append(0)  # Standard ASCII
        else:
            bits.append(1)  # Unicode homoglyph = 1 bit

    # Group into bytes (MSB first)
    flag = ''
    for i in range(0, len(bits) - 7, 8):
        byte_val = 0
        for j in range(8):
            byte_val = (byte_val << 1) | bits[i + j]
        flag += chr(byte_val)
    return flag
```

**Common homoglyph pairs:**
| ASCII | Homoglyph | Unicode Block |
|-------|-----------|---------------|
| `a` (U+0061) | `а` (U+0430) | Cyrillic |
| `o` (U+006F) | `о` (U+043E) | Cyrillic |
| `e` (U+0065) | `е` (U+0435) | Cyrillic |
| `s` (U+0073) | `ѕ` (U+0455) | Cyrillic DZE |
| `t` (U+0074) | `𝚝` (U+1D69D) | Math Monospace |
| `p` (U+0070) | `р` (U+0440) | Cyrillic |

**Key lessons:**
- Check ALL replies to official CTF posts, not just the main post
- Platform auto-formatting (smart quotes `'` → `'`) must be excluded from bit encoding
- Hints like "hype comes with its own secrets" suggest steganography in the social media posts themselves
- Bluesky public API requires no authentication — use `public.api.bsky.app`

---

## Strava Fitness Route OSINT (MidnightCTF 2026)

**Pattern (Where was Chine):** Target's physical location identified through fitness tracking data. Username discovered on Twitter → alias found in GitHub code → alias searched on Strava → running route endpoint reveals location.

**Strava public data exposure:**
- Public athlete profiles: `https://www.strava.com/athletes/<id>`
- Activity maps show GPS routes with start/end points
- Even "privacy zones" can be circumvented by analyzing route shapes outside the zone
- Segment leaderboards reveal athlete locations without following them

**Location extraction workflow:**
1. Find target's Strava profile via username enumeration (Whatsmyname, Osint Industries)
2. Check public activities for GPS route maps
3. Identify route start/end points or frequent locations
4. Search the endpoint location on Google Maps
5. Verify with Google Maps user-submitted photos (see [geolocation-and-media.md](geolocation-and-media.md#google-maps-crowd-sourced-photo-verification-midnightctf-2026))

**Key insight:** Fitness apps are high-value OSINT targets because users rarely restrict activity visibility. A single public run reveals home/work neighborhoods. Cross-reference GPS endpoints with Google Maps to identify specific parks, buildings, or landmarks.

**Detection:** Challenge mentions exercise, running, cycling, fitness, GPS, or health tracking. Target persona has an active/athletic profile.

---

## Discord API Enumeration

**Pattern (Insanity 1 & 2, 0xFun 2026):** Flags hidden in Discord server metadata not visible in normal UI.

**Hiding spots:**
- Role names
- Animated GIF emoji (flag in 2nd frame with tiny duration)
- Message embeds
- Server description, stickers, events

```bash
# Enumerate with user token
TOKEN="your_token"
# List roles
curl -H "Authorization: $TOKEN" "https://discord.com/api/v10/guilds/GUILD_ID/roles"
# List emojis
curl -H "Authorization: $TOKEN" "https://discord.com/api/v10/guilds/GUILD_ID/emojis"
# Search messages
curl -H "Authorization: $TOKEN" "https://discord.com/api/v10/guilds/GUILD_ID/messages/search?content=flag"
```

**Animated emoji:** Download GIF, extract frames -- hidden data in brief frames invisible at normal speed.
