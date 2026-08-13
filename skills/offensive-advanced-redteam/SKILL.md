---
name: offensive-advanced-redteam
description: 'Practical advanced red team operations guide: OPSEC discipline, C2 infrastructure
  design, living-off-the-land techniques, lateral movement, persistence, data exfiltration,
  and evading modern defenses. Use for planning advanced red team engagements or understanding
  APT TTPs.'
---

# SKILL: Advanced Redteam Ops

## Metadata
- **Skill Name**: advanced-redteam-ops
- **Folder**: offensive-advanced-redteam
- **Source**: https://github.com/SnailSploit/offensive-checklist/blob/main/Advanced%20red-team%20operations%20for%20dummies.md

## Description
Practical advanced red team operations guide: OPSEC discipline, C2 infrastructure design, living-off-the-land techniques, lateral movement, persistence, data exfiltration, and evading modern defenses. Use for planning advanced red team engagements or understanding APT TTPs.

## Trigger Phrases
Use this skill when the conversation involves any of:
`advanced red team, red team operations, OPSEC, C2 infrastructure, living off the land, LOTL, lateral movement, persistence, exfiltration, APT, advanced threat, red team for dummies`

## Instructions for Claude

When this skill is active:
1. Load and apply the full methodology below as your operational checklist
2. Follow steps in order unless the user specifies otherwise
3. For each technique, consider applicability to the current target/context
4. Track which checklist items have been completed
5. Suggest next steps based on findings

---

## Full Methodology

## Redirectors
Your CStrike, BRC4, etc., team server should ONLY bind locally. NEVER bind to 0.0.0.0 or an external-facing interface; always bind locally and have a redirector/tunnel expose it to the outside world.

On Cloudflare, you can use `Zero Trust` to create a tunnel.
Here's how to host your CStrike **teamserver** behind a redirector.
1. Start your server on your VPS:
	`./TeamServerImage -Dcobaltstrike.server_port=50050 -Dcobaltstrike.server_bindto=127.0.0.1 -Djavax.net.ssl.keyStore=./cobaltstrike.store -Djavax.net.ssl.keyStorePassword=0123456 teamserver 127.0.0.100 lovestrange` ; change lovestrange with your PW. This will bind CS to 127.0.0.1:50050.
	
2. CStrike teamservers and clients (not beacons) use raw TCP. We can't host that directly behind Cloudflare, so we smuggle it within WebSocket traffic with `websocat`:
	1. `websocat -E -b ws-l:SOURCE tcp:DESTINATION &`
	2. source = where `websocat` will listen/where you will point your tunnel. i.e., 127.0.0.1:40000
	3. destination = teamserver's IP + port
	4. final cmd: `websocat -E -b ws-l:127.0.0.1:40000 tcp:127.0.0.1:50050 &`
3. Now, point your tunnel at this address.
	1. For a temporary tunnel, use `cloudflared`:
		1. `cloudflared tunnel --url http://127.0.0.1:40000 --no-autoupdate`
	2. Otherwise, use a named Cloudflare tunnel within Zero Trust, point it to your domain + a specific path (long UUIDs are best), and point it to `http://127.0.0.1:40000`.
**VPS part is done!** Do the following on your machine before starting Cobalt Strike:
4. `websocat -E -b tcp-l:127.0.0.1:2222 ws://mytunnel.domain.com/lovestrange &` ; replace `/lovestrange` with the path you just set OR replace the domain with the link `cloudflared` gave you.
5. Start your Cobalt Strike client and connect to 127.0.0.1:2222.
6. Done :)

If your origin traffic is HTTPS you can skip the websocat part and directly point your Cloudflare tunnel to your service. you can either use Cloudflare's TLS certificate (best) or tell CF not to check origin's TLS cert

Benefits of this: much better OPSEC. A lot of team servers get taken down because they listen eternally and get scanned through Shodan then taken down. You can also use `ngrok` temporary tunnels. Tunneling HTTP is the easiest thing to do.
- The **teamserver ↔ operator** channel is raw TCP (hence `websocat`).
- The **beacon ↔ teamserver** channel is whatever your Malleable profile says (HTTP/S, DNS, etc.) and can live behind the same domain; just use different subdomains/paths.
    Diagram:
```
Operator ──(raw TCP)──► websocat ──(WS)──► cloudflared ──► Internet ──► Cloudflare edge ──► teamserver 127.0.0.1:50050

Beacon ──(HTTPS Malleable)──► same domain/different path ──► Cloudflare edge ─(TLS terminates)─► nginx ──► teamserver 127.0.0.1:443
```

## Beacon Profiles
EDIT THE BASE PROFILE! (you can text me on TG for a good profile)

Never use the default CStrike profile; always edit it as much as you can.
- **Disable Staging:** Unless absolutely necessary for standard shellcode injection, set `host_stage = false`
- Staged payloads are noisy, easier to signature and unnecessary if you are using loaders.
	- CStrike's staged payload is super detected anyway.
- **Mimic Real Traffic:** If you can, don't just randomize; profile legitimate traffic (e.g., Microsoft Teams, standard Azure API chatter) and clone it closely—matching URIs, headers, and User-Agents.
- **Memory Obfuscation:** Ensure your profile includes `set sleep_mask "true";` (encrypts heap while sleeping) and `set obfuscate "true";` (to avoid generic signature scanning in memory). Look at the sleep mask guides for CStrike.
- **Certificate Opsec:** If using HTTPS, never use the default self-signed certs. Use valid certificates (Let's Encrypt is fine) and ensure your C2 profile's `https-certificate` block matches the real certificate exactly (especially the Java Keystore specifics) to avoid fingerprinting.
	- This doesn't matter if your team server is behind an HTTP(S) redirector like `cloudflared` because TLS terminates there anyway. Be sure to set your Cloudflare tunnel to allow self-signed TLS certs.
	- I have seen some orgs ban Let's Encrypt certs altogether, but they are extremely large corporations. If you care about these targets, maybe you can invest in a real TLS cert, no? ;)

## Infrastructure Segregation
Never run all operations from one VPS or domain; use a tiered approach so that burning one asset doesn't kill the engagement:

- **Tier 1 - Phishing/Delivery:** High-reputation domains, typically short-lived. Used ONLY to get the initial payload to the target. Once an email is flagged, this tier is burned. Warm up the domain for ~2 weeks.
    
- **Tier 2 - Interactive C2 (Short-haul):** Used for active hands-on-keyboard work. Higher risk of detection due to frequent traffic.
    
- **Tier 3 - Long-haul C2 (Persistence):** Low and slow. Connects back once a day/week. Used only to respawn Tier 2 access if it gets burned. NEVER run active commands through this if avoidable. Uses DNS or another stealthy protocol.

For more mature operations, you could use different C2 frameworks for different tiers. A lightweight, lesser-known C2 could be used for long-haul persistence, while a more feature-rich framework like Cobalt Strike could be reserved for active, short-haul operations.

## Malwareful Operations
**Phish or vish** -> get an employee to run your payload.

Your stage 0 payload is an extremely light, self-contained loader (>30kb). 
- It should NOT be a .exe because these are very often blocked.
	- I won't reveal our TTPs or anything that could help EDR vendors, so figure this out on your own ))
		- Know your target's stack tho: will your payload not get blocked?
- It HAS to be FUD (Fully Undetectable).
- It should NOT do anything other than download/extract/inject stage 1.

- Stage 1 is a very minimal implant (like Merlin or, my excellent product aimed at EDR evasion, [Shikine](https://t.me/teamkavkaz25)) that can:
	- Act as a minimal reverse shell (support 5-6 commands: ls, whoami, pwd, download, upload, execute). You make this persistent until an operator uses it to download stage 2.
	- It SHOULD be FUD because it might be written to disk.

- Stage 2 is a post-exploitation beacon like Cobalt Strike that you only load after killing AV/EDR or having a strong foothold on the victim.
	- You can now remove stage 1 from persistence and make stage 2 persistent with registry keys or something similar.
	- You shouldn't write it to disk; have it be extracted and injected at runtime on reboot, etc.
- Stage 2s are usually heavily signatured. Think: **do you really need Cobalt Strike/BRC4**?

- Stage 2 is the most signatured type because a lot of people use it as stage 0.

- Goal is to attack for persistence rather than attack for command execution.
- Every stage should be redundant. If your stage 0 downloads stage 1 over HTTPS and your target has a whitelist-based firewall, you'll be in trouble unless you use DNS as a backup.
	- This requires on-the-clock R&D time that you could avoid by baking in this redundancy and making it try many different methods/protocols when developing it.

- Many EDRs flag a process as suspicious if it's spawned by an unusual parent (`winword.exe` spawning `powershell.exe`). Your Malleable C2 profile or post-exploitation jobs can be configured to spawn processes from more legitimate-looking parents like `explorer.exe`.
	- Most AVs stop tracking after two generations. For EDRs, aim at spoofing three generations if you can.

## General Infrastructure Rules/Tips
- Have a list of TTPs you **blacklist** (like using `powershell.exe -Command [...]`, `rundll32.exe`, etc.) for better OPSEC.
- *Always* use encryption, even when your communications are internal.
	- One time during an operation, I had root access to a box and was able to escalate privileges to the whole network by listening to TCP traffic because all machines on the network sent my machine credentials in clear-text/raw HTTP. The traffic was local, but still. If they had used SSL, I wouldn't have been able to do much. ;)
- Have two types of beacons when you can:
	- A long-haul one that uses DNS or other covert channels to exfiltrate data, sleeps a lot, and can be used for the long-term or as a backup solution.
	- A short-haul beacon that uses HTTP, used for hands-on operations.
- When you infect a network, use SMB listeners. That is, set machines (and beacons) B, C, and D to use SMB listeners to relay traffic to machine A, where A is a short-haul beacon. They are much stealthier.
- Don't just forward all traffic to your team server. Blue teams will scan your infrastructure.
	- **Dumb Redirector:** `iptables`/`websocat` just forwarding port X to team server 443. (Bad OPSEC, easily fingerprinted).
	- **Smart Redirector:** Use Nginx or Apache with specific rules.
    - Filtering: Only forward traffic to the team server if it matches your specific Malleable C2 User-Agent and URIs *(hint: use a custom HTTP header like `session_id` to transmit your data and filter beacons from blue-teamers.)*
    - Deflection: If traffic doesn't match (e.g., a Shodan scanner or blue team hitting your domain), proxy pass it to a legitimate site (like generic Microsoft or Amazon pages).
- **Avoid "Easy" Built-ins:** `psexec` (even the Cobalt Strike version) creates very predictable service installation artifacts (Event ID 7045). Avoid it unless you absolutely know logging is disabled. 
- **WinRM Preferred:** If you have credentials, WinRM is generally cleaner and blends in better with admin traffic than SMB service creation.
- **Bring Your Own Tools (BYOT) Carefully:** Don't drop a standard compiled Mimikatz to disk. Use sleep-masked in-memory execution, or better yet, use alternative credential dumping methods (like dumping LSASS via legitimate Microsoft binaries (`comsvcs.dll`), though this is heavily watched now too).
	- Ideally, NEVER write any tooling to disk.
- **Jitter and Sleep:** NEVER use 0 sleep unless actively working on a box. Even then, consider 1-3s sleep. For standard beacons, use a high jitter (e.g., sleep 60 with jitter 37) to avoid mathematically predictable beacon intervals that NDRs easily spot.
- **Kill Dates:** Always set a kill date on your beacons. You don't want a forgotten zombie beacon calling back to your infrastructure 3 years later during someone else's audit.
- **Timestomping:** When dropping files to disk, match the creation/modification timestamps of legitimate files in the same directory (`timestomp` command) to blend in during casual forensic reviews.
- **USE AS MANY LOLBINS AS YOU CAN!**: I've had many cases where using an API to do something (like downloading a payload through WinSock) was blocked/checked by an EDR, but using a LOLBin for it was completely fine (`curl.exe`).
	- Live off the land (LOTL) as much as you can. Use built-in tools as much as you can to avoid bringing your own stuff.

#### Data Exfiltration

- **Stealthy Data Transfer:** You can use DNS tunneling for small, important files like passwords or smuggling data in seemingly normal HTTP traffic.
- **Know Your Target's Stack:** If they just have Splunk and ClamAV, you can be bold and do whatever you want.
	- Once in an operation, my target only had Splunk and no AV, so I zipped important files through a reverse shell, hosted a basic HTTP server with Python, and used an HTTP tunnel (`cloudflared`) to exfiltrate 80GB of data after office hours. )


hackerz 4 lyfe
***Lovestrange | TEAM KAVKAZ***
TG @ **lovestrangekz**
