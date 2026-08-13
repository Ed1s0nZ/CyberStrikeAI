# Nmap Cheatsheet

Complete reference for nmap port scanner.

## Table of Contents

- [Basic Scan Syntax](#basic-scan-syntax)
- [Port Specifications](#port-specifications)
- [Scan Types](#scan-types)
- [Timing Templates](#timing-templates)
- [Service Detection](#service-detection)
- [OS Detection](#os-detection)
- [Output Formats](#output-formats)
- [Script Engine (NSE)](#script-engine-nse)
- [Firewall Evasion](#firewall-evasion)
- [Performance Tips](#performance-tips)

---

## Basic Scan Syntax

```bash
nmap [Scan Type(s)] [Options] {target specification}
```

### Target Specification

| Format | Description | Example |
|--------|-------------|---------|
| IP Address | Single host | `nmap 192.168.1.1` |
| Hostname | Domain name | `nmap example.com` |
| CIDR | Subnet | `nmap 192.168.1.0/24` |
| Range | IP range | `nmap 192.168.1.1-100` |
| Comma | Multiple hosts/ports | `nmap 192.168.1.1,2,3` |
| File | List from file | `nmap -iL targets.txt` |

**Exclude targets:**
```bash
nmap 192.168.1.0/24 --exclude 192.168.1.1
nmap 192.168.1.0/24 --excludefile exclude.txt
```

---

## Port Specifications

| Option | Description |
|--------|-------------|
| `-p 22,80,443` | Specific ports |
| `-p 1-1000` | Port range |
| `-p-` | All ports (1-65535) |
| `--top-ports 100` | Top 100 most common ports |
| `-F` | Fast mode (top 100 ports) |
| `-p U:53,137,T:80,443` | Mix UDP/TCP |

**Common port lists:**
```bash
-p 21,22,23,25,53,80,110,137,139,443,445,3306,3389,5432,5900,8080
```

---

## Scan Types

### TCP Scans

| Option | Name | Description | Privileges |
|--------|------|-------------|------------|
| `-sS` | SYN scan | Half-open, stealthy | Root required |
| `-sT` | TCP connect | Full connection | Not required |
| `-sA` | ACK scan | Maps firewall rules | Root required |
| `-sW` | Window scan | Similar to ACK | Root required |
| `-sM` | Maimon scan | Similar to ACK/-sS | Root required |

### UDP Scans

| Option | Description |
|--------|-------------|
| `-sU` | UDP scan |
| `--top-ports 100` | Scan top 100 UDP ports |

**Combined TCP + UDP:**
```bash
nmap -sS -sU target.com
```

### SCTP and IP

| Option | Description |
|--------|-------------|
| `-sY` | SCTP INIT scan |
| `-sZ` | SCTP COOKIE scan |
| `-sO` | IP protocol scan |

### Ping Scans (Host Discovery)

| Option | Description |
|--------|-------------|
| `-sn` | No port scan (ping only) |
| `-PE` | ICMP echo |
| `-PP` | ICMP timestamp |
| `-PM` | ICMP address mask |
| `-PR` | ARP ping (local network) |
| `--disable-arp-ping` | Skip ARP ping |

---

## Timing Templates

| Template | T-Level | Parallel | Timeout | Description |
|----------|---------|----------|---------|-------------|
| Paranoid | T0 | Serial | 5 min | IDS evasion |
| Sneaky | T1 | Serial | 15 sec | IDS evasion |
| Polite | T2 | Low | 1 sec | Reduced load |
| Normal | T3 | Medium | 1 sec | Default |
| Aggressive | T4 | High | 1 sec | Recommended |
| Insane | T5 | Very High | 0.75 sec | May miss ports |

**Usage:**
```bash
nmap -T4 target.com    # Recommended
nmap -T2 target.com    # Slower, stealthier
nmap -T5 target.com    # Fastest
```

**Fine-tuned timing:**
```bash
--min-rate 100        # Minimum packets/sec
--max-rate 1000       # Maximum packets/sec
--min-parallelism 10  # Minimum probes in parallel
--max-parallelism 100 # Maximum probes in parallel
--initial-rtt-timeout 500ms
--max-rtt-timeout 1000ms
--max-retries 2
```

---

## Service Detection

| Option | Description |
|--------|-------------|
| `-sV` | Probe open ports for service info |
| `--version-intensity 0-9` | Level 0 = light, 7 = default |
| `--version-all` | Try all probes (intensity 9) |
| `--version-trace` | Show scanning details |

**Examples:**
```bash
nmap -sV target.com
nmap -sV --version-intensity 7 target.com
nmap -sV --version-all target.com
```

---

## OS Detection

| Option | Description |
|--------|-------------|
| `-O` | Enable OS detection |
| `--osscan-limit` | Limit to promising targets |
| `--osscan-guess` | Guess OS more aggressively |

**Examples:**
```bash
nmap -O target.com
nmap -O --osscan-guess target.com
```

---

## Output Formats

| Option | Description |
|--------|-------------|
| `-oN file` | Normal output |
| `-oX file` | XML output |
| `-oG file` | Grepable output |
| `-oA file` | All formats (base name) |

**Append mode:**
```bash
-oA filename  # Creates filename.nmap, filename.xml, filename.gnmap
--append-output
```

**Verbosity:**
```bash
-v     # Increase verbosity
-vv    # More verbosity
-d     # Debugging
-dd    # More debugging
```

**Errors:**
```bash
--reason      # Show port state reason
--open        # Only show open ports
--packet-trace # Show all packets sent/received
```

---

## Script Engine (NSE)

### Script Categories

| Category | Description |
|----------|-------------|
| `auth` | Authentication bypass/brute force |
| `broadcast` | Broadcast discovery |
| `brute` | Brute force auditing |
| `default` | Default scripts (same as -sC) |
| `discovery` | Service enumeration |
| `dos` | Denial of service |
| `exploit` | Exploitation |
| `external` | External queries |
| `fuzzer` | Fuzzing |
| `intrusive` | Intrusive scripts |
| `malware` | Malware detection |
| `safe` | Safe/intrusive-free |
| `vuln` | Vulnerability detection |

### Usage

```bash
# Run default scripts
nmap -sC target.com

# Run specific scripts
nmap --script=vuln target.com
nmap --script=auth target.com
nmap --script=brute target.com

# Run specific script
nmap --script=http-title target.com
nmap --script=smb-vuln-ms17-010 target.com

# Run multiple scripts
nmap --script=http-title,http-headers target.com

# Run with arguments
nmap --script=http-title --script-args http.useragent="MyBot" target.com

# Run scripts in wildcard
nmap --script=http-* target.com
nmap --script=vuln,*-brute target.com

# Run scripts based on service
nmap --script+="not intrusive" target.com
nmap --script="default or safe" target.com
```

### Script Arguments

```bash
--script-args user=foo,pass=bar
--script-args-file args.txt
--script-trace        # Show script execution
--script-updatedb     # Update script database
--script-help         # Script help
```

---

## Firewall Evasion

### Fragmentation

```bash
-f                    # Fragment packets (8 bytes each)
-ff                   # Fragment into tiny 16-byte packets
--mtu 24             # Set MTU (implies -f)
--data-length 24      # Append random data
```

### Decoy

```bash
-D RND:10             # Random 10 decoys
-D ME,192.168.1.1,192.168.1.2  # Specific decoys
--source-port 53      # Source port (firewall bypass)
```

### Spoofing

```bash
-S 192.168.1.100      # Spoof source address
-e eth0               # Use specific interface
--spoof-mac 00:12:34:56:78:9A  # Spoof MAC address
```

### Other Evasion

```bash
--bad-sum             # Send bogus checksums
--ttl 10              # Set TTL value
--randomize-hosts     # Randomize scan order
```

---

## Performance Tips

### Speed Up Scans

```bash
# Use aggressive timing
nmap -T4 target.com

# Limit port range
nmap -p 1-1000 target.com

# Skip DNS resolution
nmap -n target.com

# Only scan open ports
nmap --open target.com

# Use parallelism
nmap --min-parallelism 50 target.com

# Skip host discovery
nmap -Pn target.com
```

### Slow Down Scans

```bash
# Use polite timing
nmap -T2 target.com

# Add delays
nmap --scan-delay 1s target.com

# Limit rate
nmap --max-rate 10 target.com
```

---

## Common Options

| Option | Description |
|--------|-------------|
| `-6` | IPv6 scanning |
| `-v` | Verbose |
| `-h` | Help |
| `-Pn` | Skip host discovery |
| `-n` | No DNS resolution |
| `-R` | Always DNS resolution |
| `--system-dns` | Use system DNS |
| `--traceroute` | Trace hop path to target |
| `--reason` | Show port state reason |
| `--open` | Show only open ports |
| `--iflist` | Show host interfaces |
| `-e eth0` | Use specific interface |

---

## Examples

### Quick Recon
```bash
nmap -T4 -F target.com
```

### Full Audit
```bash
nmap -sV -sC -O -p- target.com
```

### Stealth Scan
```bash
sudo nmap -sS -T2 -f target.com
```

### Large Network
```bash
nmap -sn 192.168.1.0/24    # Host discovery
nmap -T4 192.168.1.0/24    # Fast scan
```

### UDP Services
```bash
nmap -sU --top-ports 100 target.com
```

### Vulnerability Scan
```bash
nmap --script=vuln target.com
```

### Save Results
```bash
nmap -sV -oA scan_results target.com
# Creates: scan_results.nmap, scan_results.xml, scan_results.gnmap
```
