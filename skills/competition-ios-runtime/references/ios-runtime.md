# iOS Runtime Checklist

## Static Targets To Map First

- `Info.plist`, entitlements, URL schemes, universal links, embedded frameworks, provisioning clues
- Keychain access groups, app groups, local plist files, SQLite DBs, cache directories
- Certificate checks, jailbreak checks, request builders, crypto helpers, device-binding logic

## Preferred Hook Boundaries

1. Request builder input and final signed headers
2. Crypto helper plaintext and ciphertext
3. Trust evaluator or pinning decision point
4. Keychain read or write boundary
5. Objective-C selector or Swift method that gates the accepted branch

## Evidence To Keep Together

- Static location: class, selector, framework, plist key, entitlement, or bundle path
- Dynamic proof: hook log, returned value, header, request body, or accepted response
- State dependency: Keychain item, plist value, DB row, nonce, device flag, or local token

## Common Pitfalls

- Hooking only UI handlers and missing the real signer or trust evaluator
- Capturing a signed request without the plaintext or local state that generated it
- Mixing original IPA, decrypted bundle, and patched runtime output without labeling them separately
