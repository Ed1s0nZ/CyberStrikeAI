---
name: offensive-osint
description: Comprehensive OSINT methodology skill for offensive security, red team
  intelligence gathering, and bug bounty reconnaissance. Covers domain recon, email
  harvesting, social media profiling, GitHub/code leaks, Shodan/Censys enumeration,
  breach data lookup, employee profiling, infrastructure mapping, cryptocurrency tracing,
  geospatial intelligence, and AI-assisted analysis workflows. Use when performing
  reconnaissance against a target domain or organization, investigating a person or
  entity, tracing cryptocurrency flows, geolocating images or events, or building
  an attack-surface map.
---

# Offensive OSINT Methodology

## Workflow

1. Define target scope (domain, org, person, crypto address, or geo subject)
2. Select applicable categories below based on scope
3. Work top-down within each category; pivot on discovered artifacts
4. Archive every key artifact: URL + timestamp + screenshot (PNG) + hash (SHA-256)
5. Log findings in JSONL with a `run_id` and tool versions for reproducibility
6. Suggest next steps based on what each tool returns

---

## General OSINT

- [Bookmarks](https://tools.myosint.training/) — Comprehensive OSINT bookmarks
- [OSINT Framework](https://osintframework.com/) — Tool/resource directory
- [IntelTechniques Tools](https://inteltechniques.com/tools/) — Suite of investigative tools
- [Bellingcat Toolkit](https://www.bellingcat.com/resources/2024/09/24/bellingcat-online-investigations-toolkit/) — Investigative journalism tools
- [CyberSudo OSINT Toolkit](https://docs.google.com/spreadsheets/d/1EC0sKA_W9znzsxUt0wye9UYtyATXw5m8) — OSINT websites list
- [Google Dorks](https://dorksearch.com/) — Efficient Google searching
- [Distributed Denial of Secrets](https://ddosecrets.com/) — Leaked data
- [Country-Specific Resources](https://digitaldigging.org/osint/) — Country-targeted OSINT

### Search Engines

| Tool | Notes |
|------|-------|
| [Carrot2](https://search.carrot2.org/#/search/web) | Clusters results by topic |
| [etools](https://www.etools.ch/) | Metasearch engine |
| [Kagi](https://kagi.com/) | Privacy-first, non-personalized results |
| [Brave Search](https://search.brave.com/) | Independent index; Goggles for custom ranking |
| [PDF Search](https://www.pdfsearch.io/) | Search PDF files and view table of contents |
| [Google Fact Check Explorer](https://toolbox.google.com/factcheck/explorer) | Cross-site fact-check search |

---

## Username & Email Investigation

| Tool | Purpose |
|------|---------|
| [Sherlock](https://github.com/sherlock-project/sherlock) | Username search across social networks |
| [Maigret](https://github.com/soxoj/maigret) | Collect profiles by username from many sites |
| [What's My Name](https://whatsmyname.app/) | Username search across platforms |
| [Holehe](https://github.com/megadose/holehe) | Check if email is registered on platforms |
| [Epieos](https://epieos.com/) | Email address pivots and metadata |
| [OSINT Industries](https://osint.industries/) | Email/username/phone lookups |
| [Hunter.io](https://hunter.io/) | Find email addresses for a domain |
| [EmailRep](https://emailrep.io/) | Email reputation and associated data |
| [Emailable](https://emailable.com/) | Verify email existence |
| [Mugetsu](https://mugetsu.io/) | X/Twitter username history |
| [RocketReach](https://rocketreach.co/) / [Apollo](https://www.apollo.io/) | Email enrichment and pattern guessing |
| [PhoneInfoga](https://github.com/sundowndev/phoneinfoga) | Phone number intelligence framework |

**Browser extensions:** [GetProspect](https://chromewebstore.google.com/detail/email-finder-getprospect/bhbcbkonalnjkflmdkdodieehnmmeknp), [SignalHire](https://chrome.google.com/webstore/detail/signalhire-find-email-or/aeidadjdhppdffggfgjpanbafaedankd)

---

## People Search

- [TruePeopleSearch](https://www.truepeoplesearch.com/) — Free U.S. people search
- [WhitePages](https://www.whitepages.com/) — Contact information
- [Spokeo](https://www.spokeo.com/) — People search engine
- [Webmii](https://webmii.com/) — People search
- [Pipl](https://pipl.com/) — Deep web people search (paid)
- [Clearbit](https://clearbit.com/) — Company/individual data enrichment
- [FaceCheck](https://facecheck.id/) / [FaceSeek](https://faceseek.online/) — Reverse face search

---

## Phone Number OSINT

- [TrueCaller](https://www.truecaller.com/) — Caller ID and spam blocking
- [ThatsThem](https://thatsthem.com/) — Reverse phone search
- [Infobel](https://infobel.com/) — Phone search outside USA
- [FreeCarrierLookup](https://freecarrierlookup.com/) — Carrier/type lookup (US)
- [NumlookupAPI](https://numlookupapi.com/) [Freemium] — Programmatic carrier/line-type checks
- [CallerIDTest](https://calleridtest.com/) — Phone search
- [Advanced Background Checks](https://www.advancedbackgroundchecks.com/) — All people linked to a number

---

## Social Media

| Platform | Tool |
|----------|------|
| Instagram | [Picuki](https://www.picuki.com/) — view profiles without account |
| X/Twitter | [snscrape](https://github.com/snscrape/snscrape) — preferred CLI scraper; use Twint only as fallback |
| Facebook | [Graph Search](https://inteltechniques.com/tools/Facebook.html), [sowsearch.info](https://sowsearch.info/), [lookup-id.com](https://lookup-id.com/), [whopostedwhat.com](https://whopostedwhat.com/) |
| Facebook (research) | [Meta Content Library](https://transparency.meta.com/researcher) — CrowdTangle successor (researcher-gated) |
| YouTube/Twitch | [Social Blade](https://socialblade.com/) — analytics |
| TikTok | [Tokboard](https://tokboard.com/) — trend and profile analytics |
| Reddit | [Reveddit](https://www.reveddit.com/) — removed content; [RedTrack.social](https://redtrack.social/) — user history |
| Bluesky | [Firesky](https://firesky.tv/) — real-time firehose; [SkyView](https://bsky.jazco.dev/) — follower graphs |
| Mastodon | [FediSearch](https://fedisearch.skorpil.cz/) — cross-instance search; [Fedifinder](https://fedifinder.glitch.me/) — find Twitter users on Mastodon |
| Faces | [Search4Faces](https://search4faces.com/) |

---

## Public Records & Company Information

- [OpenCorporates](https://opencorporates.com/) — World's largest open company database
- [SEC EDGAR](https://www.sec.gov/edgar.shtml) — U.S. company filings
- [OpenOwnership Register](https://register.openownership.org/) — Beneficial ownership datasets
- [MuckRock](https://www.muckrock.com/) — FOIA repository and request tracking
- [EU Tenders (TED)](https://ted.europa.eu/) — EU procurement notices
- [World Bank Projects](https://projects.worldbank.org/) — Project and procurement records

### RU/CN Registries

**Russia:** [Rusprofile](https://www.rusprofile.ru/), [Kontur.Focus](https://focus.kontur.ru/) (freemium), [zakupki.gov.ru](https://zakupki.gov.ru/) (procurement), EGRUL/EGRIP (official, captcha-gated)

**China:** [GSXT](https://www.gsxt.gov.cn/) (National Enterprise Credit), [Qichacha](https://www.qcc.com/)/[Tianyancha](https://www.tianyancha.com/) (freemium), [MIIT ICP/Beian](https://beian.miit.gov.cn/) (ICP filings)

### Sanctions & Compliance

- [OFAC SDN List](https://sanctionssearch.ofac.treas.gov/)
- [EU Sanctions Map](https://www.sanctionsmap.eu/)
- [OpenSanctions](https://www.opensanctions.org/) — Aggregated persons/entities datasets
- [OCCRP Aleph](https://aleph.occrp.org/) — Investigative documents, leaks, company records

---

## Breach & Leak Data

- [Have I Been Pwned](https://haveibeenpwned.com/) — Breach lookup; Pwned Passwords API (k-anonymity)
- [Dehashed](https://dehashed.com/) — Credential search
- [IntelX](https://intelx.io/) — Data intelligence
- [LeakCheck](https://leakcheck.io/) — Breach lookups
- [Snusbase](https://snusbase.com/) — Database breach lookups
- [BreachDirectory](https://breachdirectory.org/) — Recent breach credentials
- [Scattered Secrets](https://scatteredsecrets.com/)
- [Cavalier (Hudson Rock)](https://cavalier.hudsonrock.com/) — Infostealer lookups
- [Phonebook](https://phonebook.cz/)
- [LeakPeek](https://leakpeek.com/)

---

## Infrastructure & Attack-Surface OSINT

- [Shodan](https://www.shodan.io/) — Internet-connected device/service search
- [Censys](https://search.censys.io/) — Host and certificate enumeration
- [GreyNoise](https://viz.greynoise.io/) — Distinguish background noise from targeted scans
- [SecurityTrails](https://securitytrails.com/) — Passive DNS and asset discovery
- [SpiderFoot](https://www.spiderfoot.net/) — Automated recon and correlation
- [theHarvester](https://github.com/laramies/theHarvester) — Subdomain, email, metadata harvesting
- [Recon-ng](https://github.com/lanmaster53/recon-ng) — Web recon framework
- [Amass](https://github.com/owasp-amass/amass) / [Subfinder](https://github.com/projectdiscovery/subfinder) — Passive subdomain discovery
- [BuiltWith](https://builtwith.com/) — Tech stack enumeration
- [Netlas](https://netlas.io/) — Large-scale HTTP/DNS/certificate pivots
- [BinaryEdge](https://www.binaryedge.io/) / [FOFA](https://fofa.so/) / [ZoomEye](https://www.zoomeye.org/) — Infra pivots complementing Shodan/Censys
- [RiskIQ PassiveTotal](https://community.riskiq.com/) — Passive DNS/cert/host pivots
- [Spur](https://spur.us/) — IP lookups and tracking
- [Robtex](https://www.robtex.com/) — Passive DNS and infrastructure pivots

### ASN/BGP & Internet Measurement

- [Hurricane Electric BGP Toolkit](https://bgp.he.net/) — ASN, prefix, peers, IRR data
- [RIPEstat](https://stat.ripe.net/) — IP/ASN history, routing, geolocation, abuse contacts
- [BGPView](https://bgpview.io/) — ASN and prefix explorer
- [bgp.tools](https://bgp.tools/) — Clean ASN/IX views, routing details
- [PeeringDB](https://www.peeringdb.com/) — Facility and peering info

### Certificates & CT Monitoring

- [crt.sh](https://crt.sh/) — Search Certificate Transparency logs
- [Censys Certificates](https://search.censys.io/certificates) — CT and x509 attribute pivots
- [CertStream](https://certstream.calidog.io/) — Real-time CT feed via WebSocket
- [Rapid7 Open Data](https://opendata.rapid7.com/) — Sonar DNS/HTTP/SSL datasets
- [Cert Spotter](https://sslmate.com/certspotter) [Freemium] — CT monitoring and alerts
- Favicon hash (mmh3): cluster infrastructure; pair with Shodan/Censys favicon search

---

## Threat Intel & IOCs

- Vendor/CERT advisories: CISA/NSA/CSA joint advisories, CERT-EU, NCSC-UK, JPCERT/CC, CERT-UA
- [MISP Project](https://www.misp-project.org/) and public MISP feeds
- [OpenCTI](https://www.opencti.io/) — CTI knowledge graph
- [Malpedia](https://malpedia.caad.fkie.fraunhofer.de/) — Malware families, YARA, references
- [ThreatFox](https://threatfox.abuse.ch/) / [URLHaus](https://urlhaus.abuse.ch/) / [SSLBL](https://sslbl.abuse.ch/)
- [MalwareBazaar](https://bazaar.abuse.ch/) — Hash-based sample sharing
- [PhishTank](https://www.phishtank.com/) / [OpenPhish](https://openphish.com/)

### Malware Analysis & Sandboxes

- Static analysis: [pefile](https://github.com/erocarrera/pefile), [FLOSS](https://github.com/mandiant/flare-floss), [capa](https://github.com/mandiant/capa)
- Similarity: SSDEEP, TLSH
- Sandboxes: [ANY.RUN](https://any.run/), [Hybrid Analysis](https://www.hybrid-analysis.com/), [CAPE](https://capesandbox.com/), [Tria.ge](https://tria.ge/)
- Intelligence: [Intezer](https://analyze.intezer.com/) (code reuse), [VirusTotal](https://www.virustotal.com/) (**caution**: uploads become public)
- TLS fingerprints: [JA3](https://github.com/salesforce/ja3), [JA4](https://github.com/FingerprinTLS/ja4)

---

## Cryptocurrency OSINT

### Blockchain Explorers

| Chain | Explorer |
|-------|---------|
| Bitcoin | [Blockchain.com](https://www.blockchain.com/explorer), [Blockchair](https://blockchair.com/) |
| Ethereum | [Etherscan](https://etherscan.io/) |
| BNB Chain | [BSCScan](https://bscscan.com/) |
| Polygon PoS | [PolygonScan](https://polygonscan.com/) |
| Solana | [Solscan](https://solscan.io/) |
| Multi-chain | [OKLink](https://www.oklink.com/) [Freemium], [Cielo](https://cielo.io/) |

**L2 Explorers:** [Arbiscan](https://arbiscan.io/), [Optimistic Etherscan](https://optimistic.etherscan.io/), [BaseScan](https://basescan.org/), [zkSync Era](https://explorer.zksync.io/), [L2Beat](https://l2beat.com/) (risk/TVL comparison)

### Transaction Tracking & Analytics

- [Arkham](https://www.arkhamintelligence.com/) — Multichain explorer, entity labels, graphs, alerts
- [TRM](https://www.trmlabs.com/) — Address/transaction graphs
- [MetaSleuth](https://metasleuth.io/) — Visual crypto flow analysis
- [Breadcrumbs](https://www.breadcrumbs.app/) [Freemium] — Visual graphing and labeling
- [Bubblemaps](https://bubblemaps.io/) — Holder concentration visualization
- [Whale Alert](https://whale-alert.io/) — Large transaction monitoring
- [Chainalysis](https://www.chainalysis.com/) / [Crystal Blockchain](https://crystalblockchain.com/) — Professional analytics
- [GraphSense](https://graphsense.info/) — Cryptocurrency analytics platform
- [Nansen](https://www.nansen.ai/) — Smart Money labels (paid)
- [Dune](https://dune.com/) — Custom blockchain data queries
- [Token Sniffer](https://tokensniffer.com/) — Honeypot and scam token detection

### NFT & Exchange Intelligence

- [OpenSea](https://opensea.io/) / [NFTScan](https://www.nftscan.com/) — NFT marketplace/explorer
- [DappRadar](https://dappradar.com/) — NFT sales and marketplace activity
- [CoinGecko](https://www.coingecko.com/) / [CoinMarketCap](https://coinmarketcap.com/) — Market data
- [Glassnode](https://glassnode.com/) — On-chain market intelligence

### Bridge Monitoring

- [Socketscan](https://socketscan.io/) — EVM bridge explorer
- [L2Beat Bridges](https://l2beat.com/bridges) — Bridge risk analysis
- [Pulsy](https://pulsy.io/) — Bridge explorer aggregator

---

## Media Intelligence

### Reverse Image & Facial Search

- [Google Images](https://images.google.com/) — General reverse image search
- [TinEye](https://tineye.com/) — Reverse image search
- [Yandex Images](https://yandex.com/images/) — Effective for Russian/Eastern European content
- [PimEyes](https://pimeyes.com/en) — Face-based image search
- [FaceCheck](https://facecheck.id/) — Find people by photo

### Image Forensics

- [Forensically](https://29a.ch/photo-forensics/) — Digital image forensics toolkit
- [ExifTool](https://exiftool.org/) — Read/write/edit metadata
- [Jimpl](https://jimpl.com/) — Online EXIF viewer
- [Jeffrey's EXIF viewer](http://exif.regex.info/exif.cgi) — Online metadata viewer
- [FOCA](https://www.elevenpaths.com/labstools/foca) — Metadata in documents
- [Metagoofil](https://www.edge-security.com/metagoofil.php) — Extract metadata from public documents
- [C2PA Verify](https://verify.contentauthenticity.org/) — Verify content credentials and AI provenance

### Video Analysis

- [YouTube Data Viewer](https://citizenevidence.amnestyusa.org/) — Extract YouTube metadata
- [InVID & WeVerify](https://www.invid-project.eu/tools-and-services/invid-verification-plugin/) — Video verification browser extension
- [YouTube Geo Tag](https://mattw.io/youtube-geofind/location) — Video geolocation via geo tags
- [MediaInfo](https://mediaarea.net/en/MediaInfo) — Technical/tag info for video/audio
- Snap Map (public stories) — Area/event context

### Browser Extensions for Media

- [Fake News Debunker by InVID & WeVerify](https://chrome.google.com/webstore/detail/fake-news-debunker-by-inv/mhccpoafgdgbhnjfhkcmgknndkeenfhe)
- [RevEye Reverse Image Search](https://chrome.google.com/webstore/detail/reveye-reverse-image-sear/kejaocbebojdmebagkjghljkeefgimdj)
- [EXIF Viewer Pro](https://chrome.google.com/webstore/detail/exif-viewer-pro/mmbhfeiddhndihdjeganjggkmjapkffm)
- [Wayback Machine Extension](https://chrome.google.com/webstore/detail/wayback-machine/fpnmgdkabkmnadcjpehmlllkndpkmiak)
- [Search by Image](https://chromewebstore.google.com/detail/search-by-image/cnojnbdhbhnkbcieeekonklommdnndci)

---

## Geospatial Intelligence

### Satellite Imagery & Mapping

- [Google Maps](https://www.google.com/maps) / [Bing Maps](https://www.bing.com/maps/) — General mapping
- [Sentinel Hub EO Browser](https://apps.sentinel-hub.com/eo-browser/) — Sentinel/Landsat satellite imagery
- [NASA Worldview](https://worldview.earthdata.nasa.gov/) — NASA satellite imagery
- [Zoom Earth](https://zoom.earth/) — Live satellite images and weather
- [Wayback Imagery](https://livingatlas.arcgis.com/wayback/) — Historical satellite images
- [NASA FIRMS](https://firms.modaps.eosdis.nasa.gov/map/) — Fire/hotspot data
- [Open Infrastructure Map](https://openinframap.org/) — Global infrastructure networks
- [Windy](https://www.windy.com/) — Live weather map

### Geolocation Tools

- [Mapillary](https://www.mapillary.com/app) — Crowdsourced street-level imagery
- [KartaView](https://kartaview.org/) — Open-source street-level imagery
- [Overpass Turbo](https://overpass-turbo.eu/) — Advanced OpenStreetMap queries
- [SunCalc](https://www.suncalc.org/) — Sun position for chronolocation
- [GeoNames](https://www.geonames.org/) — Geographical database
- [PeakVisor](https://peakvisor.com/) — Identify mountain peaks
- [GeoGuesser tips](https://somerandomstuff1.wordpress.com/2019/02/08/geoguessr-the-top-tips-tricks-and-techniques/) — Geolocation methodology

**Street View:** Google Street View, [Apple Maps](https://maps.apple.com/), [Yandex Maps](https://yandex.com/maps/), [Baidu Maps](https://map.baidu.com/)

### Flight OSINT

- [FlightRadar24](https://www.flightradar24.com/) / [FlightAware](https://www.flightaware.com/) / [RadarBox](https://www.radarbox.com/)
- [ADSBExchange](https://www.adsbexchange.com/) — Unfiltered community ADS-B feed
- [Planespotters](https://www.planespotters.net/) — Fleet/airframe history by tail number
- [AirFrames](https://www.airframes.org/) / [JetPhotos](https://www.jetphotos.com/) — Visual confirmation

### Maritime OSINT

- [MarineTraffic](https://www.marinetraffic.com/) — Live AIS vessel tracking
- [VesselFinder](https://www.vesselfinder.com/) — Global ship movements and port calls
- [FleetMon](https://www.fleetmon.com/) — Historical AIS data and analytics
- [Global Fishing Watch](https://globalfishingwatch.org/map/) — Fishing vessel behavior and AIS gap analysis

---

## AI-Assisted OSINT

> **Warning:** Never paste PII, sensitive IOCs, or unique pivots into cloud LLMs. They log inputs and may use them for training. Use local models (Ollama, LM Studio) for sensitive analysis.

| Tool | Strength |
|------|---------|
| [ChatGPT](https://chat.openai.com/) (paid) | Log parsing, dataset analysis, Code Interpreter for CSVs/JSON, GPT-4 Vision for image OCR |
| [Claude](https://claude.ai/) (paid) | 200K token context for large document dumps and report synthesis |
| [Gemini 1.5 Pro](https://gemini.google.com/) | 2M token context; Deep Research mode with citations |
| [Perplexity Pro](https://www.perplexity.ai/) (paid) | Real-time web search + reasoning; multi-query synthesis |

**Local/privacy-preserving:** [Ollama](https://ollama.com/) (Llama 3, Mistral), [LM Studio](https://lmstudio.ai/), [GPT4All](https://gpt4all.io/)

### Commercial AI OSINT Platforms

- [Cylect](https://www.cylect.io/) — AI entity extraction and link-analysis
- [Fivecast Matrix](https://www.fivecast.com/products/matrix/) — Generative-AI triage for social-media datasets
- [Recorded Future](https://www.recordedfuture.com/) — AI-driven threat intelligence
- [DarkOwl Vision](https://www.darkowl.com/) — AI-powered darknet data analysis

### Deepfake & Synthetic Media Detection

- [Sensity AI](https://sensity.ai/) — Deepfake detection
- [Reality Defender](https://realitydefender.com/) — AI-generated content detection
- [Adobe Content Credentials Verify](https://contentcredentials.org/verify) — C2PA verifier
- [CarNet](https://carnet.ai/) — AI car model identification (useful for geolocation)

---

## Archiving & Evidence Preservation

- [archive.today](https://archive.today/) — One-page content archiver with screenshot
- [URLScan.io](https://urlscan.io/) — On-demand webpage scan with resource map
- [ArchiveBox](https://archivebox.io/) — Self-hosted archiving (HTML, PDF, screenshots, media)
- [Hunchly](https://www.hunch.ly/) — Evidence capture for investigators (paid)
- Wayback SavePageNow API v3 — On-demand archiving with job IDs
- [SingleFileZ](https://github.com/gildas-lormeau/SingleFileZ) — Browser extension for offline HTML archives
- [Kasm Workspaces](https://kasmweb.com/) — Containerized OSINT workspace/browser isolation

**Evidence handling:**
- Capture: URL + timestamp + PNG screenshot + WARC/SingleFileZ archive
- Hash all downloaded files (SHA-256) and record in case notes
- Separate work profiles/containers per case; store evidence read-only
- Use JSONL (NDJSON) logs with `run_id` and tool versions for reproducibility

---

## Automation & Workflows

- [n8n](https://n8n.io/) — Self-hosted workflow automation (e.g., RSS → scrape → alert pipelines)
- [Huginn](https://github.com/huginn/huginn) — Agent-based monitoring, scraping, alerting
- [Playwright](https://playwright.dev/) — Headless browser automation with stealth plugins
- [Browsertrix Crawler](https://github.com/webrecorder/browsertrix-crawler) — Archival crawling with WARC export
- [Prefect](https://www.prefect.io/) / [Apache Airflow](https://airflow.apache.org/) — Workflow orchestration for data pipelines

---

## Regional Search Engines

- Russia/CIS: [Yandex](https://yandex.com/), [Mail.ru Search](https://go.mail.ru/)
- China: [Baidu](https://www.baidu.com/), [Sogou](https://www.sogou.com/), [360 Search](https://www.so.com/)
- Russia social: [VK](https://vk.com/), [OK.ru](https://ok.ru/)
- China social: [Weibo](https://weibo.com/), [Bilibili](https://www.bilibili.com/), [Zhihu](https://www.zhihu.com/), [Douyin](https://www.douyin.com/)

---

## Telegram & Messaging Intelligence

- [TGStat](https://tgstat.com/) — Channel analytics and search
- [Telemetr](https://telemetr.io/) — Channel growth, overlaps, forwards
- [Combot](https://combot.org/) — Group analytics (partially paid)
- [TelegramDB Search Bot](https://t.me/TGdb_bot) — Basic Telegram OSINT
- [Discord ID](https://discord.id/) — Basic Discord account information
- Sogou Weixin search — WeChat Official Accounts content search
- View public Telegram channels: `https://t.me/s/<channel>`
