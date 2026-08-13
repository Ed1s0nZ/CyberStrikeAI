# Android Hooking Checklist

## Static Targets To Map First

- `AndroidManifest.xml`, exported components, deeplinks, intent filters, bundled configs
- Native libraries, JNI registration points, crypto helpers, request builders, protobuf models
- Shared prefs, SQLite DBs, WebView assets, root checks, SSL pinning logic

## Preferred Hook Boundaries

1. Request signer input string and output signature
2. Crypto helper plaintext and ciphertext
3. JNI boundary arguments and return values
4. Keystore access or device-binding checks
5. WebView bridge messages or JS interface calls

## Evidence To Keep Together

- Static location: class, method, symbol, or asset path
- Dynamic proof: hook log, returned value, request header, or accepted response
- State dependency: local token, DB row, pref key, nonce, or device flag

## Common Pitfalls

- Hooking too high in the UI layer and missing the real signer boundary
- Capturing a signed header without the plaintext that produced it
- Ignoring local state prerequisites such as prefs, DB rows, or keystore material
