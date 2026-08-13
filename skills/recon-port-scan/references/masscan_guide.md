# Masscan Guide

High-performance port scanner for large-scale network reconnaissance.

## Table of Contents

- [Installation](#installation)
- [Basic Usage](#basic-usage)
- [Target Specification](#target-specification)
- [Port Ranges](#port-ranges)
- [Rate Limiting](#rate-limiting)
- [Output Formats](#output-formats)
- [Common Use Cases](#common-use-cases)
- [Integration with Nmap](#integration-with-nmap)

---

## Installation

### Linux (Debian/Ubuntu)

```bash
# From repository
sudo apt install masscan

# Build from source
git clone https://github.com/robertdavidgraham/masscan.git
cd masscan
make
sudo make install
```

### macOS

```bash
brew install masscan
```

**Note:** Masscan requires raw socket access, so run with `sudo` when needed.

---

## Basic Usage

```bash
masscan [options] {target ranges}
```

**Simple example:**
```bash
sudo masscan 192.168.1.0/24 -p80,443
```

---

## Target Specification

| Format | Description | Example |
|--------|-------------|---------|
| IP Range | CIDR notation | `192.168.1.0/24` |
| Single IP | Single host | `192.168.1.1` |
| Range | IP range | `192.168.1.1-100` |
| Comma | Multiple targets | `192.168.1.1,192.168.1.2` |
| File | Read from file | `masscan -iL targets.txt` |

**Examples:**
```bash
masscan 192.168.1.0/24 -p80
masscan 10.0.0.0/8,192.168.0.0/16 -p80,443,22
masscan -iL target_list.txt -p1-65535
```

---

## Port Ranges

| Option | Description |
|--------|-------------|
| `-p80` | Single port |
| `-p80,443,22` | Multiple ports |
| `-p1-1000` | Port range |
| `-p1-65535` | All ports |

**Examples:**
```bash
masscan 192.168.1.0/24 -p80
masscan 192.168.1.0/24 -p80,443,8080
masscan 192.168.1.0/24 -p1-65535
```

---

## Rate Limiting

**CRITICAL:** Always set `--rate` to avoid network issues.

| Option | Description |
|--------|-------------|
| `--rate 10000` | Packets per second |
| `--rate 100K` | 100,000 packets/sec |
| `--rate 1M` | 1 million packets/sec |

**Guidelines:**
```bash
# Slow/conservative
--rate 1000      # 1,000 pps

# Moderate
--rate 10000     # 10,000 pps (default)

# Fast (good connection)
--rate 100000    # 100,000 pps

# Very fast (datacenter)
--rate 1000000   # 1 million pps
```

**Calculate rate:**
- For /24 network (256 hosts): `--rate 1000` is safe
- For /16 network (65,536 hosts): Start with `--rate 10000`
- Consider bandwidth: ~100 bytes/packet → `--rate 10000` ≈ 1 MB/s

---

## Output Formats

| Option | Description | Format |
|--------|-------------|--------|
| `-oL file` | List format | Human-readable |
| `-oG file` | Grepable format | Easy to parse |
| `-oJ file` | JSON format | Structured data |
| `-oX file` | XML format | XML structure |
| `-oB file` | Binary format | Masscan native |

**Examples:**
```bash
masscan 192.168.1.0/24 -p80,443 --rate 10000 -oL results.txt
masscan 192.168.1.0/24 -p80,443 --rate 10000 -oJ results.json
masscan 192.168.1.0/24 -p80,443 --rate 10000 -oX results.xml
```

### Output Formats Details

**List format (-oL):**
```
open tcp 80 192.168.1.1 1234567890
open tcp 443 192.168.1.1 1234567890
```

**JSON format (-oJ):**
```json
{"ip": "192.168.1.1", "port": 80, "proto": "tcp", "status": "open", "timestamp": "1234567890"}
{"ip": "192.168.1.1", "port": 443, "proto": "tcp", "status": "open", "timestamp": "1234567890"}
```

---

## Common Use Cases

### Quick Web Service Discovery

```bash
sudo masscan 192.168.1.0/24 -p80,443,8080,8443 --rate 10000 -oL web_scan.txt
```

### Full Port Scan (Large Network)

```bash
sudo masscan 10.0.0.0/8 -p1-65535 --rate 100000 -oJ full_scan.json
```

### Internet-Scale Scan

```bash
# Scan entire internet for port 80
sudo masscan 0.0.0.0/0 -p80 --rate 10000000 -oX internet_80.xml
```

**Warning:** Internet-scale scans require significant bandwidth and time.

---

## Integration with Nmap

Masscan finds open ports quickly, then use nmap for detailed scanning.

### Workflow

```bash
# Step 1: Masscan to find open ports
sudo masscan 192.168.1.0/24 -p1-65535 --rate 10000 -oL masscan_results.txt

# Step 2: Convert to namp format
# Use masscan_to_nmap.py script or manually extract ports
grep "open" masscan_results.txt | awk '{print $4}' | sort -u > open_ports.txt

# Step 3: Nmap for service details
nmap -sV -sC -p $(cat open_ports.txt | tr '\n' ',' | sed 's/,$//') 192.168.1.100
```

### Automated with Provided Script

```bash
# Run masscan
sudo masscan 192.168.1.0/24 -p1-65535 --rate 10000 -oJ masscan.json

# Convert to nmap commands
python masscan_to_nmap.py masscan.json --nmap-cmd -t 192.168.1.100

# Output: nmap commands for each host with discovered ports
```

---

## Advanced Options

| Option | Description |
|--------|-------------|
| `--wait 5` | Seconds to wait after scan |
| `--retries 2` | Retry attempts |
| `--banners` | Grab banners |
| `--exclude 192.168.1.1` | Exclude IP |
| `--excludefile file.txt` | Exclude IPs from file |
| `--adapter eth0` | Use specific network interface |
| `--source-port 53` | Source port (firewall bypass) |
| `--spoof-mac 00:12:34:56:78:9A` | Spoof MAC address |

**Examples:**
```bash
# Banner grabbing
sudo masscan 192.168.1.0/24 -p80 --banners --rate 10000 -oB banners.bin

# Source port spoofing
sudo masscan 192.168.1.0/24 -p80 --source-port 53 --rate 10000

# Exclude hosts
sudo masscan 192.168.1.0/24 -p80 --exclude 192.168.1.1,192.168.1.254 --rate 10000
```

---

## Comparison: Masscan vs Nmap

| Feature | Masscan | Nmap |
|---------|---------|------|
| Speed | Extremely fast | Moderate |
| Port range | All 65535 | All 65535 |
| Service detection | Limited (--banners) | Comprehensive (-sV) |
| OS detection | No | Yes (-O) |
| Script engine | No | Yes (NSE) |
| Accuracy | Good | Excellent |
| Use case | Large networks | Detailed analysis |
| Transmission rate | Customizable | Fixed templates |

**Recommendation:** Use masscan for initial discovery on large networks, then use nmap for detailed analysis of discovered services.

---

## Tips and Best Practices

1. **Always set --rate**: Unrestricted masscan can saturate networks
2. **Use with caution**: Masscan is noisy and easily detected
3. **Test first**: Start with small ranges before large scans
4. **Combine with nmap**: Use masscan for discovery, nmap for details
5. **Check firewall rules**: Some networks block masscan's aggressive scanning
6. **Consider legal implications**: Get authorization before scanning

---

## Troubleshooting

### Permission Denied

```
error: masscan: cannot open packet adapter
```

**Solution:** Run with `sudo`
```bash
sudo masscan ...
```

### Network Saturation

Masscan caused network slowdown.

**Solution:** Lower rate
```bash
--rate 1000  # Reduce from 10000
```

### No Results Found

```bash
# Check if service is actually running
nmap -p80 target.com

# Try with different source port
masscan target.com -p80 --source-port 53 --rate 100
```
