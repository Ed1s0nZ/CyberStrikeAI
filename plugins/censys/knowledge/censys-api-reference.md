# Censys Platform API v3 Reference

## Migration Notice
Censys migrated from v1/v2 (search.censys.io) to Platform API v3 (api.platform.censys.io) in 2025.
- Old: HTTP Basic auth with API ID + Secret - DEPRECATED
- New: Bearer token (Personal Access Token / PAT)
- Generate PAT: https://accounts.censys.io/settings/personal-access-tokens

## Base URL
`https://api.platform.censys.io/v3/`

## Authentication
```
Authorization: Bearer censys_ex_token_...
Content-Type: application/json
```

## Endpoints

### Search (1 credit)
```
POST /v3/global/search/query
{"q": "port: 443 AND location.country: UA", "resource_type": "host", "limit": 50}
```

### Host Lookup (free)
```
GET /v3/global/asset/host/{ip_address}
```

### Host Names (free)
```
GET /v3/global/asset/host/{ip_address}/names
```

### Certificate Lookup (free)
```
GET /v3/global/asset/certificate/{sha256_fingerprint}
```

### Web Property (free)
```
GET /v3/global/asset/webproperty/{hostname}:{port}
```

### Aggregate (1 credit)
```
POST /v3/global/search/aggregate
{"q": "port: 443", "resource_type": "host", "agg_field": "autonomous_system.name", "num_buckets": 25}
```

### Credits Balance (free)
```
GET /v3/accounts/users/credits
```

## Credit Model
| Operation | Cost |
|-----------|------|
| Host/cert/web lookup | Free |
| Search query | 1 credit |
| Aggregate | 1 credit |
| CensEye pivot | 44 credits |

## Tiers
| Tier | Credits/month | Concurrent |
|------|--------------|------------|
| Free | 100 | 1 |
| Starter | 10,000 | 1 |
| Enterprise | Unlimited | 10 |
