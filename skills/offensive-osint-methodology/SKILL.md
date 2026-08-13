---
name: offensive-osint-methodology
description: 'Structured OSINT methodology framework: target definition, source selection,
  collection workflows, data correlation, timeline reconstruction, and reporting.
  Use to guide systematic OSINT campaigns or teach OSINT methodology.'
---

# SKILL: OSINT Methodology

## Metadata
- **Skill Name**: osint-methodology
- **Folder**: offensive-osint-methodology
- **Source**: https://github.com/SnailSploit/offensive-checklist/blob/main/osint-method.md

## Description
Structured OSINT methodology framework: target definition, source selection, collection workflows, data correlation, timeline reconstruction, and reporting. Use to guide systematic OSINT campaigns or teach OSINT methodology.

## Trigger Phrases
Use this skill when the conversation involves any of:
`OSINT methodology, open source intelligence, target profiling, data correlation, OSINT workflow, intelligence collection, OSINT campaign, recon methodology`

## Instructions for Claude

When this skill is active:
1. Load and apply the full methodology below as your operational checklist
2. Follow steps in order unless the user specifies otherwise
3. For each technique, consider applicability to the current target/context
4. Track which checklist items have been completed
5. Suggest next steps based on findings

---

## Full Methodology

# OSINT Methodology

## OpSec

### Create a Sock Puppet

- Fake account that cannot be linked to you
- Build a posting history (post stuff, etc.)
- Resources
  - [Effective Sock Puppets](https://medium.com/@unseeable06/creating-an-effective-sock-puppet-for-your-osint-investigation-95fdbb8b075a)
  - [Ultimate Guide to Sock Puppets](https://osintteam.blog/the-ultimate-guide-to-sockpuppets-in-osint-how-to-create-and-utilize-them-effectively-d088c2ed6e36)
  - [Fake Name Generator](https://www.fakenamegenerator.com/)
  - [This Person does not Exist](https://thispersondoesnotexist.com/)
  - Use separate browser profiles or isolation tools (e.g., **Firefox Multi‑Account Containers**) for any sock‑puppet activity.
  - Acquire disposable VoIP/SMS numbers (e.g., **Burner**, **Silent Link**) to satisfy platform verification without exposing real phone numbers.
  - Audit every browser extension before installation; supply‑chain attacks on popular add‑ons have targeted investigators since 2024.
  - Use dedicated browser profiles/containers per case and persona; avoid logging into personal accounts.
  - Prefer hardware‑backed passkeys for critical accounts; store recovery codes offline.
  - Maintain a minimal chain‑of‑custody: timestamp actions, hash key artifacts, and record tool versions per case.

## Cryptocurrency Investigation

### Transaction Analysis

- Track transaction flows between wallets
- Identify clusters of related addresses
- Monitor large transfers and whale activity
- Use block explorers to trace fund movements
- Tools:
  - Cielo: Multi-chain wallet tracking (EVM, Bitcoin, Solana, Tron)
  - TRM: Create relationship graphs for addresses/transactions
  - Arkham: Multichain explorer with entity labels, graph creation, and alerts
  - MetaSleuth: Transaction visualization for retail users
  - Range: CCTP bridge explorer
  - Socketscan: EVM bridge explorer
  - Pulsy: Bridge explorer aggregator
  - Chainalysis: **Horizon 2.0** cross‑chain tracing suite (paid)
  - Elliptic: **Lens** visual link explorer (launched Dec 2024)
  - Most compliance suites now provide **real‑time bridge‑risk scoring** dashboards (e.g., TRM, Chainalysis)

#### Layer 2 / Rollup Analysis

- **zkSync Era / Polygon zkEVM**: Zero-knowledge proofs hide transaction details on L2; only deposit/withdrawal bridge events visible on L1. Use [zkSync Era Block Explorer](https://explorer.zksync.io/) and [PolygonScan zkEVM](https://zkevm.polygonscan.com/).
- **Arbitrum / Optimism**: Transactions batched and compressed; L2 state reconstructed from L1 calldata. Use [Arbiscan](https://arbiscan.io/) and [Optimistic Etherscan](https://optimistic.etherscan.io/). Check [L2Beat](https://l2beat.com/) for risk framework and technology stack.
- **StarkNet**: Cairo VM with STARK proofs; different address derivation. Use [Voyager](https://voyager.online/) or [StarkScan](https://starkscan.co/).
- **Base / Blast / Scroll**: OP Stack or ZK-rollups; similar challenges to above.
- **Privacy protocols on L2**:
  - Aztec Network: Programmable privacy with noir circuits; limited block explorer visibility.
  - Railgun: Privacy system for DeFi on Ethereum/Polygon/BSC; shielded pools obscure sender/receiver/amount.
  - Privacy Pools: Proposed Tornado Cash successor with association sets; not yet deployed at scale.
- **Challenges**:
  - Bridge mixers (Hop Protocol, Across, Stargate) create synthetic liquidity pools that break direct tracing; funds enter/exit via pool swaps.
  - Cross-rollup transfers further obfuscate trails; requires tracking via bridge contracts and relayer infrastructure.
  - Many L2s lack mature analytics tools; explorers show transactions but relationship graphs are sparse.
- **Methodology**:
  - Start with L1 bridge events (deposits/withdrawals); these anchor L2 activity to known addresses.
  - Use L2-specific explorers to trace activity within the rollup.
  - For privacy protocols, focus on timing analysis, deposit/withdrawal clustering, and off-chain metadata (transaction memos, Tornado Cash-style notes).

#### Cautions (bridges and heuristics)

- Bridges/mixers/wrappers introduce mint/burn semantics; avoid assuming 1:1 flows without on‑chain proofs.
- MEV/sandwich and aggregator paths can create false "direct" trails; validate with multiple datasets.
- Cross‑label sanity: vendor labels can disagree; treat labels as hypotheses, not ground truth.
- **L2 finality**: Optimistic rollups have 7-day challenge periods; zkRollups finalize faster but proofs can be batched/delayed.

### Wallet Profiling

- Analyze wallet age and activity patterns
- Check for connections to known entities
- Monitor balance changes over time
- Identify associated exchange accounts

### Exchange Investigation

- Track deposits/withdrawals
- Monitor trading patterns
- Identify linked accounts
- Check for regulatory compliance

### NFT Investigation

- Track ownership history
- Monitor sales and transfers
- Analyze metadata and hidden content
- Identify connected wallets and marketplaces

## Image Analysis

- Contextual Analysis
  - Use multiple reverse image search engines to find matches or similar images:
    - [Google Images](https://images.google.com/) / **Google Lens** (note: Google Lens now requires authentication for some features; use incognito/sock-puppet account)
    - [Yandex Images](https://yandex.com/images/)
    - [Bing Image Match](https://www.bing.com/images/)
    - [TinEye](https://tineye.com/)
    - [Copyseeker](https://copyseeker.com/) AI‑based reverse‑image search engine
    - [Perplexity Pro](https://www.perplexity.ai/) with image upload: AI-powered contextual analysis and web search
  - Use browser extensions for quick searches:
    - [RevEye Reverse Image Search](https://chrome.google.com/webstore/detail/reveye-reverse-image-sear/kejaocbebojdmebagkjghljkeefgimdj)
    - [Search by Image](https://chromewebstore.google.com/detail/search-by-image/cnojnbdhbhnkbcieeekonklommdnndci) (multi-engine support)
  - Change search terms and time to narrow down the possible results
  - You can leverage [FakeNews Debunker Extension](https://chromewebstore.google.com/detail/fake-news-debunker-by-inv/mhccpoafgdgbhnjfhkcmgknndkeenfhe) as well
  - [Picarta](https://picarta.ai/) might help with geolocation as well
  - Check for embedded metadata (EXIF data) that may contain geolocation or device information:
    - [ExifTool](https://exiftool.org/)
    - [Jeffrey's Image Metadata Viewer](http://exif.regex.info/exif.cgi)
    - [EXIF Viewer Pro](https://chrome.google.com/webstore/detail/exif-viewer-pro/mmbhfeiddhndihdjeganjggkmjapkffm)
- Foreground
  - Signs, license plates, clothing styles, vegetation, and weather conditions.
- Background
  - Landmarks, unique buildings, mountains, bodies of water, and infrastructure.
- Map Markings
  - Flora and fauna types, which can indicate geographic regions.
  - Seasonal indicators like snow, foliage, or daylight hours.
- Trial and Error
  - Manually compare features from the image with maps and street views.
  - Use platforms like `Google Street View`, `Bing Streetside`, and `Yandex Panorama` to virtually explore locations.
  - Employ [Overpass Turbo](https://overpass-turbo.eu/)
  - Use Snap Map public stories for area‑based context pivots.
  - Consider Google Earth Studio for stabilized timelapse and bearing estimation.
- Pull Text from Image
  - you can use google or Yandex OCR to pull text from image
  - you can also search that text alongside your image for better results
  - Transcript extraction for video (YouTube): fetch captions to improve keyword and entity search.

### Image Forensics

- Analyze images for signs of manipulation or to uncover hidden details.
- Tools
  - [Forensically](https://29a.ch/photo-forensics/)
  - [FotoForensics](http://fotoforensics.com/)
  - [Bellingcat Photo Checker](https://photo-checker.bellingcat.com/)
  - [Sensity AI Deepfake Monitor](https://platform.sensity.ai/)
  - [Exposing.ai](https://exposing.ai/) facial‑dataset search
  - C2PA verification: [Adobe Content Credentials Verify](https://verify.contentauthenticity.org/) and `c2patool`
- Techniques
  - Error Level Analysis (ELA)
  - Metadata examination
  - Clone detection
  - Noise analysis

### Mountain Geolocation

- Use tools to identify mountain peaks and match them with the image.
- Tools
  - [PeakVisor](https://peakvisor.com/)
  - [Peakfinder](https://www.peakfinder.org/)
  - [PeakLens](https://peaklens.com/) AR mountain identifier
- Methodology
  - Align the silhouette of mountains in the image with the 3D models in the tools.
  - Adjust parameters like viewing angle and elevation.

### Fire Identification

- Identify fires, deforestation, or environmental changes.
- Tools
  - [NASA FIRMS](https://earthdata.nasa.gov/earth-observation-data/near-real-time/firms)
  - [Sentinel Hub Playground](https://apps.sentinel-hub.com/sentinel-playground/)
  - [Global Forest Watch](https://www.globalforestwatch.org/)
  - [Copernicus EFFIS](https://effis.jrc.ec.europa.eu/) EU wildfire monitoring portal

### Track and Find Planes

- Use [Apollo Hunter](https://imagehunter.apollomapping.com/) to find exact satellite image time
- Then use [FlightRadar](https://www.flightradar24.com/) to track that plane that you found
- Verify the size and plane features
- [ADS-B Exchange](https://www.adsbexchange.com/) – unfiltered global flight data

## Video Analysis

- Find context regarding the video
  - Signs, banners, and billboards.
  - Architectural styles and building materials.
  - Road markings and traffic signs.
  - License plates
  - Clothing styles and local customs.
  - Search for video snippets on platforms like YouTube, Twitter, or TikTok.
- Metadata Extraction
  - [YouTube Data Viewer](https://citizenevidence.amnestyusa.org/)
  - ExifTool: Extract metadata from downloaded video files.
- Platform-Specific Techniques
  - TikTok and Instagram
    - APIs change often; prefer platform exports when available
    - Sample cadence: 1–4 h for fast‑moving topics; keep a fixed persona and capture logs
    - Analyze user profiles for location tags; examine comments and hashtags for clues
  - **Bluesky AT Protocol**
    - Resolve handles via `https://bsky.social/xrpc/com.atproto.identity.resolveHandle?handle=<handle>` to get DID
    - Extract full identity document: `https://plc.directory/<did>` (returns PLC operations, handle history, PDS endpoint)
    - Real-time firehose: Use [Firesky](https://firesky.tv/) for live keyword/hashtag monitoring across entire network
    - Analytics: [SkyView](https://bsky.jazco.dev/) for follower graphs, post engagement, network analysis
    - Archive early: AT Protocol allows post deletion and handle migration; capture DIDs and post CIDs
    - Labelers and moderation: Check user's selected labelers (affects content visibility); different from centralized moderation
    - PDS (Personal Data Server): Users can self-host; identify via DID document to understand data custody
  - **Mastodon / Fediverse**
    - Instance matters: `@user@mastodon.social` vs `@user@infosec.exchange` - different jurisdictions, moderation policies, logging practices
    - WebFinger for discovery: `https://<instance>/.well-known/webfinger?resource=acct:<user>@<instance>` returns ActivityPub actor URL
    - Cross-instance search: [FediSearch](https://fedisearch.skorpil.cz/) aggregates public posts; not all instances are indexed
    - Instance enumeration: [Fediverse Observer](https://fediverse.observer/), [Fediverse.party](https://fediverse.party/) for instance lists, stats, software versions
    - Graph analysis: Follower/following lists are public by default; export via API for network mapping
    - Privacy considerations: Some instances (e.g., Pixelfed, PeerTube) federate differently; check instance software type
    - Archive via API: ActivityPub objects are JSON-LD; capture `id`, `published`, `content`, `attributedTo` fields
    - Deleted content: Federation is asynchronous; deletions may not propagate immediately; check caches and relay instances
- Auditory Clues
  - Languages or dialects spoken.
  - Background noises (train horns, call to prayer, wildlife).
  - Tools
    - [Audacity](https://www.audacityteam.org/): Audio editing software
    - [Sonic Visualiser](https://www.sonicvisualiser.org/): Visualize audio data
    - [SoundCMD](https://soundcmd.com/) crowd‑sourced sound‑matching engine
  - Methodology
    - Create spectrograms to identify unique sound patterns.
    - Use **Shazam** or **SoundHound** to identify music tracks.
- Extract Key Frames
  - Use tools like [FFmpeg](https://ffmpeg.org/) or [VLC Media Player](https://www.videolan.org/vlc/) to capture frames.
  - Extract frames at regular intervals or when significant changes occur.
  - Stitch frames together if the camera pans to create a panoramic image.
  - Create a panorama if the camera pans across a scene.
- Analyze frames using the same techniques as in image geolocation.
  - When possible, obtain the original upload (avoid re‑encodes) to retain metadata and audio clarity.
  - Decode platform snowflakes (e.g., Discord, Twitter/X) to infer server‑side timestamps for events.
  - **Threads by Instagram**: Similar to Instagram API limitations; use web scraping or official exports where available.
  - **Video stabilization**: Use FFmpeg `deshake` or Blender VSE to stabilize panning/shaky footage for better landmark identification.

## Chronolocation and Time Analysis

### Shadow Analysis

- Use shadows to estimate the time of day and date when the image or video was captured.
- Methodology
  - Determine the length and direction of shadows in the image.
  - Identify objects casting the shadows (e.g., poles, buildings).
- Calculate Sun Position
  - Use the object's height and shadow length to calculate the solar elevation angle.
  - Determine the azimuth (sun's compass direction).
- Tools
  - [SunCalc](https://www.suncalc.org/)
  - [ShadeMap](https://shademap.app/) – interactive 3‑D shadow simulator
  - Bellingcat **Shadow‑Finder** micro‑tool
    - Input location coordinates.
    - Adjust dates and times to match shadow lengths and directions.
  - **SunCalc.net**: Similar tool with additional features.
  - NOAA Solar Calculator for precise solar angles by date/time.
  - Use UTC consistently across all notes and screenshots.
  - OSM map‑compare sites and EOX Cloudless layers to cross‑check base imagery.

### Astronomical Calculations

- For night images, use celestial bodies to determine time and location.
- Tools
  - [Stellarium](https://stellarium.org/): Planetarium software
  - SkyMap: Mobile app for stargazing.
  - [MoonCalc](https://www.mooncalc.org/)
- Methodology
  - Identify visible stars, constellations, or the moon phase.
  - Use software to simulate the sky at different times and locations.
  - Match the celestial arrangement in the image to a specific date and time.

### Satellite Imagery Time

- Use historical satellite imagery to determine changes over time.
- Tools
  - **Google Earth Pro**:
    - Use the historical imagery slider to view images from different dates.
  - [Sentinel Hub EO Browser](https://apps.sentinel-hub.com/eo-browser/)
    - Access Sentinel and Landsat data.
    - Create TimeLapse animations.
- Methodology
  - Enter the location coordinates.
  - Select appropriate satellite datasets (Sentinel-2, Landsat 8).
  - Analyze changes in the environment to narrow down dates.
  - Record coordinates in WKT and hash cached tilesets for reproducibility where feasible.

## Threat Actor Investigation

### Actor‑Centric Workflow

- Scoping
  - Define the actor hypothesis (e.g., APT28, APT29, Turla, Sandworm; APT10, APT41, Mustang Panda, Volt Typhoon).
  - Collect seed reports from CERTs and vendors; extract indicators and TTPs.
- Indicator harvesting
  - Parse IOCs (domains, IPs, hashes, JA3/JA4, user‑agents) from advisories and reports; normalize and de‑duplicate.
  - Validate IOCs with passive DNS, CT logs, sandbox submissions, and open telemetry where possible.
- Infrastructure mapping
  - Build pivots from CT logs (SANs, issuer, serials), shared hosting, name‑server reuse, registrar accounts, and HTML/page fingerprints.
  - Enrich with ASN/WHOIS history, RPKI/ROA status, geolocation, and hosting provider relationships.
- Artifact profiling
  - Extract PE/ELF metadata (PDB paths, compile timestamps, Rich headers, resources language, code‑signing certs).
  - Cluster with fuzzy hashes (SSDEEP/TLSH) and identify packers/loaders; search YARA and sandboxes for near‑matches.
- Social and procurement pivots
  - Pivot on developer handles, code snippets, academic theses, job posts, and procurement records that imply capability or mandate.
- Falsification and reporting
  - Weigh each linkage (weak/medium/strong); document alternatives; avoid single‑source attribution.
  - Map TTPs to MITRE ATT&CK and cite sources with exact sections/pages.

### Attribution Discipline

- Separate capability from intent and sponsorship; avoid mirror‑imaging.
- Use a rule‑of‑three: require at least three independent weak signals, or one strong + one weak, before asserting linkage.
- Prefer durable pivots (registrar accounts, code‑signing cert reuse, build path idioms) over ephemeral ones (resolving IPs).
- Clearly mark uncertainty levels and confidence (e.g., low/medium/high) and distinguish correlation from control.

### Russia‑Specific Pivots

- Corporate/people
  - EGRUL/EGRIP extracts (official registry; captcha‑gated) and Rusprofile/Kontur.Focus summaries for entities and directors.
  - Government procurement: `zakupki.gov.ru` (tenders, contractors), regional portals, and grant listings.
  - Job boards (e.g., `hh.ru`) for role requirements, tech stacks, and office locations.
- Infrastructure
  - RU WHOIS: `whois.tcinet.ru`; check registrar accounts, nserver patterns, and RU‑center usage.
  - Telegram is widely used; analyze channels, admins, cross‑posts, and bot ecosystems.
- Media/platforms
  - VKontakte, Odnoklassniki, Rutube, and regional news portals; search in Russian and transliterations.

### China‑Specific Pivots

- Corporate/people
  - National Enterprise Credit Info System (`gsxt.gov.cn`) for registered entities; cross‑check with Tianyancha/Qichacha (paid/freemium).
  - ICP filings (`beian.miit.gov.cn`) to link domains to legal entities via Unified Social Credit Codes (USCC).
- Infrastructure
  - CNNIC WHOIS and hosting footprints; common domestic clouds (Aliyun, Tencent Cloud, Huawei Cloud) and registrar patterns.
- Media/platforms
  - Weibo, WeChat Official Accounts (via `weixin.sogou.com`), Zhihu, Bilibili, Douyin, Xiaohongshu; search in Chinese and Pinyin.

### Infrastructure & Internet Measurement

- Map IPs to ASNs (HE BGP Toolkit, RIPEstat, BGPView); observe peering and hosting ecosystems.
- Check CT logs (crt.sh) for certificate reuse and issuance cadence; pivot on subjects/issuers/serials.
- Use URLScan and similar crawlers to capture HTML fingerprints, favicons (mmh3), and script hashes for clustering.
- Monitor DNS over time (SecurityTrails PDNS, DNSDB) for subdomain churn and staging domains.

## People & Social Media Investigation

### Username Enumeration

- Tools:
  - [WhatsMyName](https://whatsmyname.app/)
  - [NameCheckup](https://namecheckup.com/)
  - [Sherlock](https://github.com/sherlock-project/sherlock)

### Profile Picture & Face Search

- Tools:
  - [PimEyes](https://pimeyes.com/)
  - [Exposing.ai](https://exposing.ai/)
  - Azure Face API (subject to compliance policies)

### Social Graph & Content Analysis

- Tools:
  - [Maltego](https://www.maltego.com/)
  - [snscrape](https://github.com/snscrape/snscrape)
  - [SocialBlade](https://socialblade.com/)
  - Bluesky/Mastodon: use instance explorers and handle resolvers; pivot across the Fediverse

## Infrastructure OSINT

### IP & Domain Discovery

- Tools:
  - [Shodan](https://www.shodan.io/)
  - [Censys](https://censys.io/)
  - [Onyphe](https://www.onyphe.io/)
  - [DNSDB](https://www.farsightsecurity.com/solutions/dnsdb/)

### Certificate & Passive DNS

- Tools:
  - [crt.sh](https://crt.sh/)
  - [SecurityTrails](https://securitytrails.com/)

### Malware & Artifact Analysis Workflow

- Static triage
  - Hash (SHA‑256), strings, import tables, PDB path, Rich header, resources; check VT/Malpedia family hints (do not rely solely on AV labels).
- Dynamic/sandbox
  - Execute in sandboxes (ANY.RUN, Hybrid Analysis, CAPE, Tria.ge) to collect network IOCs, mutexes, file drops, and C2 patterns.
- Clustering
  - Use SSDEEP/TLSH and YARA matches to find related samples; compare config schemas and protocol quirks.
- Reporting
  - Normalize IOCs (STIX 2.1 if possible), include ATT&CK technique IDs, and provide reproduction steps.

### Telegram/WeChat Investigation

- Telegram
  - Use public analytics (TGStat, Telemetr, Combot) for channel growth, overlaps, and forwarding graphs.
  - Export channels with Telegram Desktop; preserve message IDs, timestamps (UTC), and media hashes.
- WeChat
  - Search Official Accounts via `weixin.sogou.com`; archive articles (PNG + WARC); capture `__biz` IDs and publisher metadata.
  - Expect link rot and content takedowns—archive early.

## Automation & Case Management

- Tools:
  - [Hunchly](https://www.hunch.ly/) (browser evidence capture)
  - [Kasm Workspaces](https://kasmweb.com/) OSINT‑ready workspace images
  - [ArchiveBox](https://archivebox.io/) – self‑hosted web archiver
  - [SingleFileZ](https://github.com/gildas-lormeau/SingleFileZ)

## Synthetic Media Verification

- Tools:
  - [Sensity AI](https://sensity.ai/)
  - [Hive Moderation](https://hivemoderation.com/)
  - [Reality Defender](https://realitydefender.com/)
