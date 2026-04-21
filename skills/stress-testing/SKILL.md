# Infrastructure Stress Testing

## Overview
Test resilience of your own infrastructure against DDoS attack patterns. Uses containerized stress testing tools to validate CDN protection, WAF rules, rate limiting, and failover mechanisms.

## Prerequisites
- Docker installed on attack host
- Target infrastructure is YOUR OWN (authorized testing only)
- Monitoring set up on target side BEFORE testing
- Notify your CDN provider (Cloudflare, AWS) of planned test

## Pre-Test Monitoring Setup

### On target server
```bash
# Watch connections
watch -n 1 'ss -s'
# Watch HTTP status codes
tail -f /var/log/nginx/access.log | awk '{print $9}' | sort | uniq -c | sort -rn

# Elasticsearch monitoring query
curl -s localhost:9200/_cat/indices?v
```

### Cloudflare dashboard
- Security > Events - watch for blocked requests
- Analytics > Traffic - monitor request rate
- Under Attack Mode - toggle during test

## Gorgon Stress Tool

### Load Docker image
```bash
docker load < gorgon-stress-3.0.0.5.tar.gz
docker images | grep gorgon
```

### Run container
```bash
docker run -it --rm gorgon-stress:3.0.0.5
# Access web panel at the Tor .onion address shown in logs
```

### Configuration
The tool exposes a web panel with:
- **Target**: IP/domain to test
- **Method**: Attack method (see below)
- **Duration**: Test duration in seconds
- **Data Size**: Payload size (32-512 bytes or "memory")
- **Threads/Workers**: Concurrency level
- **Proxy list**: Optional proxy rotation
- **Pause mode**: Auto-cycle or manual

### Attack Methods for Testing

#### Layer 4 Tests
| Method | Tests | Expected Defense |
|--------|-------|-----------------|
| TCP | TCP connection handling | SYN cookies, connection limits |
| UDP | UDP flood handling | Rate limiting, BCP38 |
| SYN | Half-open connection handling | SYN proxy, SYN cookies |
| TLS | TLS handshake capacity | TLS offload, rate limiting |

#### Layer 7 Tests
| Method | Tests | Expected Defense |
|--------|-------|-----------------|
| HTTP | Basic HTTP flood handling | WAF, rate limiting |
| HTTPS | Encrypted flood handling | TLS inspection, CDN |
| BROWSER | Bot detection effectiveness | JS challenges, CAPTCHA |
| CF variants | Cloudflare bypass resistance | Managed rules, Under Attack Mode |

### Test Phases
1. **Baseline** (5 min) - measure normal performance metrics
2. **Low intensity** (5 min) - single method, low concurrency
3. **Medium intensity** (5 min) - increase workers
4. **High intensity** (5 min) - full concurrency
5. **Multi-vector** (5 min) - combine L4 + L7
6. **Bypass test** (5 min) - CF/browser methods
7. **Recovery** (10 min) - measure time to normal after stop

### Metrics to Capture Each Phase
- Requests per second (RPS) at CDN
- Origin server CPU/memory
- Response time (p50, p95, p99)
- Error rate (4xx, 5xx)
- CDN cache hit ratio
- Blocked vs passed requests
- Connection queue depth

## Post-Test Analysis

### Generate report
```bash
# Collect Cloudflare analytics
# Collect server metrics
# Compare blocked vs passed per method
# Identify which methods bypassed defenses
# Document remediation for gaps
```

### Defense Assessment Matrix
| Method | Blocked? | Bypassed? | Remediation |
|--------|----------|-----------|-------------|
| TCP flood | | | |
| HTTP flood | | | |
| BROWSER | | | |
| CF bypass | | | |

### Remediation Priority
1. CRITICAL: Any L7 bypass that reaches origin
2. HIGH: Any method causing elevated error rate
3. MEDIUM: Methods partially mitigated but causing latency
4. LOW: Fully blocked but worth monitoring
