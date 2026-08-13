# Crypto, Stego, And Mobile Checklist

## Crypto

- Work in order: container -> compression -> encoding -> xor/substitution -> crypto -> integrity -> parse
- Recognition is not recovery; reproduce the actual plaintext or downstream artifact
- Keep exact parameters in one compact evidence block

## Stego

- Check metadata, chunk layout, palettes, alpha, LSBs, thumbnails, appended trailers
- Prefer evidence-driven decode attempts over blind brute force

## Mobile

- Check manifest/plist, exported components, deeplinks, native libs, shared prefs, local DBs, configs
- Hook the narrowest boundary: signer, crypto helper, protobuf edge, keystore access, WebView bridge
- Correlate static evidence and dynamic evidence before concluding

## Common Pitfalls

- Reporting only algorithm names with no reproduced artifact
- Scattering transform stages across many bullets
- Hooking too late and missing the trust boundary that matters
