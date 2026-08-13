# Advanced Scanning Techniques

Advanced port scanning methods for evasion and detection.

## Table of Contents

- [Scan Evasion Techniques](#scan-evasion-techniques)
- [IDS/IPS Evasion](#idsips-evasion)
- [Firewall Bypass](#firewall-bypass)
- [Timing Strategies](#timing-strategies)
- [Detection Signatures](#detection-signatures)

---

## Scan Evasion Techniques

### 1. Fragmentation

Split packets into smaller fragments to evade pattern matching.

**Nmap:**
```bash
# Fragment into 8-byte chunks
sudo nmap -f target.com

# Fragment with specific MTU
sudo nmap --mtu 24 target.com

# Additional random data
sudo nmap --data-length 24 target.com
```

### 2. Decoy Scanning

Hide scan among decoy traffic.

**Nmap:**
```bash
# Random decoys
sudo nmap -D RND:10 target.com

# Specific decoys
sudo nmap -D 192.168.1.1,192.168.1.2,192.168.1.3,ME target.com

# Zombie scan (advanced idle scan)
sudo nmap -sI <zombie_host> target.com
```

### 3. Source Port Manipulation

Set common source ports to bypass firewalls.

**Nmap:**
```bash
# DNS source port (often allowed)
sudo nmap --source-port 53 target.com

# HTTP source port
sudo nmap --source-port 80 target.com

# FTP source port
sudo nmap --source-port 20 target.com
```

**Masscan:**
```bash
sudo masscan target.com -p80 --source-port 53 --rate 1000
```

### 4. MAC Address Spoofing

**Nmap:**
```bash
# Random MAC
sudo nmap --spoof-mac 0 target.com

# Specific MAC
sudo nmap --spoof-mac 00:12:34:56:78:9A target.com

# Vendor MAC (e.g., Apple)
sudo nmap --spoof-mac Apple target.com
```

### 5. IP Spoofing

**Warning:** Requires advanced network setup and may not receive responses.

```bash
# Specify source IP
sudo nmap -S 192.168.1.50 target.com

# Use specific interface
sudo nmap -e eth0 target.com
```

---

## IDS/IPS Evasion

### Timing-Based Evasion

Slow down scanning to avoid rate-based detection.

```bash
# Very slow timing
sudo nmap -T0 target.com

# Custom delay
sudo nmap --scan-delay 10s target.com

# Randomize scan order
sudo nmap --randomize-hosts target.com
```

### Serial Scanning

Send probes sequentially (slower but harder to detect).

```bash
# Paranoid mode
sudo nmap -T0 --min-parallelism 1 target.com
```

### Avoid Common Signatures

Use less common scan types:

```bash
# FIN scan
sudo nmap -sF target.com

# NULL scan
sudo nmap -sN target.com

# XMAS scan
sudo nmap -sX target.com

# Maimon scan
sudo nmap -sM target.com
```

---

## Firewall Bypass

### ACK Scan

Map firewall rules without completing TCP handshake.

```bash
sudo nmap -sA target.com
```

### Window Scan

Similar to ACK but uses TCP window field.

```bash
sudo nmap -sW target.com
```

### FTP Bounce Scan

Use FTP server to proxy the scan.

```bash
sudo nmap -b ftp://ftp.example.com:21 target.com
```

### HTTP Proxy Scan

Scan through HTTP proxy (with proxychains).

```bash
proxychains nmap -sT -Pn -p80,443 target.com
```

---

## Timing Strategies

### Scenario-Based Timing

| Scenario | Recommended Settings |
|----------|---------------------|
| Quick sweep | `-T4 -F` |
| Full audit | `-T3 -p-` |
| Stealth | `-T0 -sS -f` |
| IDS evasion | `-T0 --scan-delay 30s` |
| Remote network | `-T3 --min-rate 100` |
| Local network | `-T5 --min-rate 1000` |

### Adaptive Timing

```bash
# Start slow, increase if no detection
nmap -T2 target.com
# If no response or alert:
nmap -T3 target.com
# Gradually increase...
```

---

## Detection Signatures

### What IDS/IPS Detect

| Technique | Detection Method |
|-----------|------------------|
| SYN scan | Half-open connections |
| Connect scan | Full TCP handshake |
| Same order ports | Sequential pattern |
| Same timing | Consistent intervals |
| OS fingerprint | Unusual packet flags |

### Evading Detection

```bash
# Randomize hosts
nmap --randomize-hosts 192.168.1.0/24

# Randomize ports (use multiple scans)
nmap -p 80,443,22,3306,3389 target.com
nmap -p 8080,8443,9090 target.com

# Vary timing
nmap --scan-delay 1-10s target.com  # Note: random range not supported, use fixed
```

---

## Advanced Nmap Patterns

### 1. Idle Scan (Zombie)

Use a zombie host to hide your IP.

```bash
# Find zombie host
sudo nmap -p 1-1023 <potential_zombie>

# Run idle scan
sudo nmap -sI <zombie_host>:<zombie_port> target.com
```

### 2. SCTP Scanning

For SCTP-enabled systems.

```bash
# SCTP INIT scan
sudo nmap -sY target.com

# SCTP COOKIE scan
sudo nmap -sZ target.com
```

### 3. IP Protocol Scan

Scan for IP protocols, not just ports.

```bash
sudo nmap -sO target.com
```

### 4. FTP Bounce Attack

Use FTP PORT command to scan through FTP server.

```bash
sudo nmap -b ftp://anonymous:password@ftp.server.com:21 target.com
```

---

## Counter-Detection

### Recognizing Detection

Signs you've been detected:
- Port scan stops returning results
- Firewall blocks all further packets
- Security team contacts
- IDS alerts in logs

### Response to Detection

1. **Stop immediately**
2. **Change IP address** if possible
3. **Wait** before resuming
4. **Reduce aggressiveness** if continuing
5. **Reconsider authorization** if not properly authorized

---

## Legal and Ethical Considerations

### Authorization

Always have:
- Written permission
- Defined scope
- Point of contact
- Emergency response plan

### Responsible Disclosure

If vulnerabilities found:
1. Report to system owner
2. Provide time to fix
3. Follow responsible disclosure
4. Don't publish without permission

---

## Detection Tools

Tools that detect port scanning:

| Tool | Type |
|------|------|
| Snort | IDS/IPS |
| Suricata | IDS/IPS |
| OSSEC | HIDS |
| Zeek (Bro) | Network monitoring |
| Fail2ban | IP banning |

### Testing Detection

```bash
# Test if your scanning is detected
# Run on your own lab environment

# Start snort
snort -i eth0 -A console -q -c snort.conf

# Then scan from another machine
nmap -T4 target.com
```

---

## Summary

**Key evasion techniques:**
1. Fragmentation (`-f`, `--mtu`)
2. Decoys (`-D`)
3. Timing adjustment (`-T0`, `--scan-delay`)
4. Source port manipulation (`--source-port`)
5. Randomization (`--randomize-hosts`)

**Remember:** No technique guarantees evasion. Best defense is proper authorization.
