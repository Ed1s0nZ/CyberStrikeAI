---
name: recon-port-scan
description: Port scanning and service identification using nmap, masscan, and rustscan. Use this skill when user needs to discover open ports, identify running services, detect service versions, or fingerprint operating systems on target hosts.
---

# Port Scanning / Reconnaissance

## Authorization Warning

**IMPORTANT:** Port scanning without proper authorization is illegal. Always ensure you have:
- Written permission from the target system owner
- Defined scope of authorized testing
- Legal compliance with local regulations

## Prerequisites

Required tools that must be installed on your system:
- **nmap** - `sudo apt install nmap` (Debian/Ubuntu) or `brew install nmap` (macOS)

Optional tools:
- **masscan** - High-speed port scanner
- **rustscan** - Modern fast scanner with nmap integration

## Quick Start

Most commonly used commands for port scanning:

### Fast Common Port Scan
```bash
nmap -T4 -F <target>
```
Quick scan of top 100 common ports.

### Full Port Scan with Service Detection
```bash
nmap -sV -sC -p- <target>
```
Scan all 65535 ports with version detection and default scripts.

### Stealth SYN Scan
```bash
sudo nmap -sS -T2 -p- <target>
```
Stealthy scan (requires root).

### High-Speed Large Range Scan
```bash
masscan -p1-65535 <target/CIDR> --rate=10000
```
Fast scanning of large IP ranges.

## Common Scenarios

### Scenario 1: Quick Reconnaissance

When you need fast results on common ports:

```bash
nmap -T4 -F <target>
```

**Parameters:**
- `-T4` - Aggressive timing template (faster)
- `-F` - Fast mode, scan top 100 ports
- `<target>` - IP address, hostname, or CIDR range

**Example:**
```bash
nmap -T4 -F 192.168.1.100
nmap -T4 -F example.com
nmap -T4 -F 192.168.1.0/24
```

---

### Scenario 2: Full Port Range Discovery

When you need to find all open ports (1-65535):

```bash
nmap -p- <target>
```

**Parameters:**
- `-p-` - Scan all 65535 ports

**Example:**
```bash
nmap -p- 192.168.1.100
```

**With version detection:**
```bash
nmap -sV -p- <target>
```

---

### Scenario 3: Service Version Detection

When you need to identify running service versions:

```bash
nmap -sV -sC <target>
```

**Parameters:**
- `-sV` - Probe open ports for service/version info
- `-sC` - Run default NSE scripts

**Example:**
```bash
nmap -sV -sC 192.168.1.100
```

**More aggressive version detection:**
```bash
nmap -sV --version-intensity 7 <target>
```

---

### Scenario 4: Stealth Scanning

When you need to avoid detection:

```bash
sudo nmap -sS -T2 -f --data-length 24 <target>
```

**Parameters:**
- `-sS` - SYN scan (stealthier than connect scan)
- `-T2` - Polite timing (slower, less suspicious)
- `-f` - Fragment packets
- `--data-length 24` - Append random data to packets

**Example:**
```bash
sudo nmap -sS -T2 -f 192.168.1.100
```

**Decoy scan:**
```bash
sudo nmap -D RND:10 -sS <target>
```

---

### Scenario 5: UDP Port Scanning

When you need to discover UDP services:

```bash
nmap -sU --top-ports 100 <target>
```

**Parameters:**
- `-sU` - UDP scan
- `--top-ports 100` - Scan top 100 most common UDP ports

**Example:**
```bash
nmap -sU --top-ports 100 192.168.1.100
```

**Combined TCP + UDP scan:**
```bash
nmap -sS -sU <target>
```

---

### Scenario 6: OS Fingerprinting

When you need to identify the operating system:

```bash
sudo nmap -O <target>
```

**Parameters:**
- `-O` - Enable OS detection

**Example:**
```bash
sudo nmap -O 192.168.1.100
```

**Combined with version detection:**
```bash
sudo nmap -sV -O <target>
```

---

### Scenario 7: High-Speed Mass Scanning

When scanning large IP ranges:

```bash
masscan -p1-65535 <CIDR> --rate=10000 -oL output.txt
```

**Parameters:**
- `-p1-65535` - Port range
- `--rate=10000` - Packets per second (adjust based on bandwidth)
- `-oL output.txt` - Save results to file

**Example:**
```bash
masscan -p1-65535 192.168.1.0/24 --rate=10000 -oL scan_results.txt
```

**Follow-up with nmap for detailed scanning:**
```bash
# First, masscan to find open ports
masscan -p1-65535 192.168.1.0/24 --rate=10000 -oL - | grep open > open_ports.txt

# Then, nmap for service details on discovered ports
nmap -sV -p 80,443,22,3306 192.168.1.100
```

---

### Scenario 8: RustScan (Modern Fast Scanner)

```bash
rustscan -a <target> -- -sV
```

**Parameters:**
- `-a` - Target address
- `--` - Separator for nmap arguments (passed through to nmap)

**Example:**
```bash
rustscan -a 192.168.1.100 -- -sV -sC
```

---

### Scenario 9: Output Formats

Save results in different formats:

```bash
# All formats (normal, XML, grepable)
nmap -oA output <target>

# XML only (for parsing)
nmap -oX output.xml <target>

# Grepable format
nmap -oG output.gnmap <target>

# Normal output to file
nmap -oN output.txt <target>
```

**Example:**
```bash
nmap -sV -p- -oA scan_results 192.168.1.100
# Creates: scan_results.nmap, scan_results.xml, scan_results.gnmap
```

---

## Tool Selection Guide

| Scenario | Recommended Tool | Command |
|----------|------------------|---------|
| Quick common port scan | nmap | `nmap -T4 -F <target>` |
| Full port range | nmap | `nmap -p- <target>` |
| Service version detection | nmap | `nmap -sV -sC <target>` |
| Large network / speed critical | masscan | `masscan -p1-65535 <target> --rate=10000` |
| Stealth required | nmap | `sudo nmap -sS -T2 <target>` |
| UDP service discovery | nmap | `nmap -sU --top-ports 100 <target>` |
| OS fingerprinting | nmap | `sudo nmap -O <target>` |
| Modern fast workflow | rustscan | `rustscan -a <target> -- -sV` |

**Tool Comparison:**

| Tool | Speed | Accuracy | Features | Use Case |
|------|-------|----------|----------|----------|
| **nmap** | Medium | High | Most comprehensive | General purpose |
| **masscan** | Very High | Medium | Basic port scan | Large networks |
| **rustscan** | High | High | nmap integration | Modern workflows |

---

## Timing Templates

Adjust scan speed with timing templates (0-5):

| Level | Name | Description |
|-------|------|-------------|
| `-T0` | Paranoid | Very slow, IDS evasion |
| `-T1` | Sneaky | Slow, IDS evasion |
| `-T2` | Polite | Medium-slow, reduces load |
| `-T3` | Normal | Default speed |
| `-T4` | Aggressive | Fast, recommended |
| `-T5` | Insane | Very fast, may be inaccurate |

**Example:**
```bash
nmap -T4 <target>     # Fast scan (recommended)
nmap -T2 <target>     # Slower, more stealthy
nmap -T5 <target>     # Maximum speed (may miss ports)
```

---

## Script Categories

NSE (Nmap Scripting Engine) script categories:

```bash
# Vulnerability detection
nmap --script=vuln <target>

# Auth bypass detection
nmap --script=auth <target>

# Brute force
nmap --script=brute <target>

# Information gathering
nmap --script=discovery,info <target>
```

---

## Common Port Lists

Reference for common service ports:

| Ports | Services |
|-------|----------|
| 21 | FTP |
| 22 | SSH |
| 23 | Telnet |
| 25 | SMTP |
| 53 | DNS |
| 80, 8080, 8443 | HTTP |
| 110 | POP3 |
| 135, 139, 445 | SMB |
| 143, 993 | IMAP |
| 443, 8443 | HTTPS |
| 3306 | MySQL |
| 3389 | RDP |
| 5432 | PostgreSQL |
| 5900 | VNC |
| 6379 | Redis |
| 27017 | MongoDB |

---

## Resources

### Scripts

- `scripts/parse_nmap_xml.py` - Parse nmap XML output to structured JSON format
- `scripts/masscan_to_nmap.py` - Convert masscan results to nmap-compatible format
- `scripts/merge_scan_results.py` - Combine multiple scan result files

### References

- `references/nmap_cheatsheet.md` - Comprehensive nmap reference guide
- `references/masscan_guide.md` - Detailed masscan usage documentation
- `references/rustscan_guide.md` - RustScan quick reference
- `references/scanning_techniques.md` - Advanced scanning techniques and evasion methods

### Assets

- `assets/top-1000-ports.txt` - Top 1000 common ports list
- `assets/top-100-ports.txt` - Top 100 common ports list
- `assets/common-services.txt` - Common service fingerprint data
---

### Scenario 10: Persistent Storage of Scan Results

When you need to persist port scan results to the database for cross-session analysis and reporting:

```bash
# Generate XML scan output
nmap -sV -p- 192.168.1.0/24 -oX scan.xml

# Store to database (flat hierarchy - no subsystem)
python .claude/skills/recon-port-scan/scripts/port_scan_storage.py \
  --xml-file scan.xml

# Store to database with subsystem
python .claude/skills/recon-port-scan/scripts/port_scan_storage.py \
  --xml-file scan.xml \
  --subsystem "External Network"

# Or pipe directly (flat hierarchy)
nmap -sV -p- target.com -oX - | \
  python .claude/skills/recon-port-scan/scripts/port_scan_storage.py

# Or pipe directly (with subsystem)
nmap -sV -p- target.com -oX - | \
  python .claude/skills/recon-port-scan/scripts/port_scan_storage.py \
    --subsystem "DMZ"
```

**Parameters:**
- `--subsystem` - Subsystem name (optional, omit for flat hierarchy)
- `--scan-tool` - Scan tool used (default: nmap)
- `--xml-file` - Path to nmap XML file (optional, reads from stdin if not provided)

**Database location:** `./data/results.db`

**Related skills:** `results-storage` - Query data, generate reports
