# RustScan Guide

Modern, high-performance port scanner written in Rust.

## Table of Contents

- [Installation](#installation)
- [Basic Usage](#basic-usage)
- [Nmap Integration](#nmap-integration)
- [Common Patterns](#common-patterns)
- [Configuration](#configuration)

---

## Installation

### Linux

```bash
# Install script
curl -L https://github.com/RustScan/RustScan/releases/download/2.0.1/rustscan_2.0.1_x86_64-unknown-linux-gnu.tar.gz | tar xz
sudo mv rustscan /usr/local/bin/

# Or from crates.io (requires Rust)
cargo install rustscan
```

### macOS

```bash
brew install rustscan
```

### Check Installation

```bash
rustscan --version
```

---

## Basic Usage

```bash
rustscan -a <target> [options]
```

**Simple scan:**
```bash
rustscan -a 192.168.1.100
```

**With custom ports:**
```bash
rustscan -a 192.168.1.100 --ports 1-1000
```

---

## Nmap Integration

RustScan's key feature is seamless nmap integration.

### Basic Pattern

```bash
rustscan -a <target> -- <nmap_arguments>
```

**The `--` separator passes all following arguments to nmap.**

### Examples

**Service version detection:**
```bash
rustscan -a 192.168.1.100 -- -sV
```

**Full audit scan:**
```bash
rustscan -a 192.168.1.100 -- -sV -sC -O
```

**Specific ports with nmap scripts:**
```bash
rustscan -a 192.168.1.100 -p 80,443,22 -- --script vuln
```

**Custom nmap options:**
```bash
rustscan -a 192.168.1.100 -- -T4 -sV -p- -oN scan.txt
```

---

## Options

| Option | Description | Example |
|--------|-------------|---------|
| `-a <addr>` | Target IP/hostname | `-a 192.168.1.100` |
| `-p <ports>` | Port range | `-p 1-1000` |
| `-b`, `--batch` | Batch size (parallelism) | `-b 5000` |
| `-t`, `--timeout` | Timeout in ms | `-t 1000` |
| `-u`, `--udp` | UDP scan | `-u` |
| `-w`, `--wordlist` | Custom port wordlist | `-w ports.txt` |
| `--no-banner` | Hide banner | `--no-banner` |

---

## Common Patterns

### Fast Discovery

```bash
rustscan -a 192.168.1.100
```

### With Service Details

```bash
rustscan -a 192.168.1.100 -- -sV -sC
```

### UDP Scan

```bash
rustscan -a 192.168.1.100 -u -- -sU
```

### Batch Size (Speed Control)

```bash
# More parallelism (faster but noisier)
rustscan -a 192.168.1.100 -b 10000

# Less parallelism (slower but stealthier)
rustscan -a 192.168.1.100 -b 1000
```

### Multiple Targets

```bash
# CIDR notation
rustscan -a 192.168.1.0/24

# Domain
rustscan -a example.com
```

---

## Configuration File

Create `~/.rustscan.toml` for default settings:

```toml
[default]
batch_size = 5000
timeout = 1500
ports = "1-65535"
```

---

## Comparison: RustScan vs Alternatives

| Feature | RustScan | Nmap | Masscan |
|---------|----------|------|---------|
| Speed | Very fast | Moderate | Extremely fast |
| Language | Rust | C | C |
| Nmap integration | Native | Native | Manual |
| Ease of use | Simple | Complex | Moderate |
| Accuracy | Good | Excellent | Good |
| Best for | Quick + detailed | Full audit | Large networks |

**When to use RustScan:**
- Want fast port discovery with immediate detailed analysis
- Prefer simple command syntax
- Need nmap integration out of the box
- Scanning individual hosts or small networks

**When to use alternatives:**
- Masscan: Large network ranges (/16, /8)
- Nmap: Maximum accuracy and options

---

## Examples by Use Case

### Web Application Recon

```bash
rustscan -a target.com -- -sV -p 80,443,8080,8443 --script http-title
```

### Internal Network Assessment

```bash
# Quick sweep
rustscan -a 192.168.1.0/24

# Detailed analysis of found hosts
rustscan -a 192.168.1.100 -- -sV -sC -O
```

### Stealthy Scan

```bash
rustscan -a target.com -b 500 -- -sS -T2
```

### Custom Port List

```bash
# Create ports.txt with desired ports
rustscan -a target.com -w ports.txt -- -sV
```

---

## Tips

1. **Adjust batch size**: `-b 10000` for fast scans, `-b 1000` for stealth
2. **Use with nmap**: Always pass nmap args after `--`
3. **Combine with scripts**: `--script vuln` for vulnerability detection
4. **Output formats**: Pass nmap output options like `-oA results`

---

## Troubleshooting

### Permission Issues

RustScan uses raw sockets, may require sudo:
```bash
sudo rustscan -a target.com
```

### Nmap Not Found

Ensure nmap is installed:
```bash
# Debian/Ubuntu
sudo apt install nmap

# macOS
brew install nmap
```
