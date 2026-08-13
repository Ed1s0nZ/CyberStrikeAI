<!-- source: Claude-Red Skills/web/offensive-graphql -->
<!-- fused into host package: api-security -->

# SKILL: GraphQL Vulnerabilities

## Metadata
- **Skill Name**: graphql-security
- **Folder**: offensive-graphql
- **Source**: https://github.com/SnailSploit/offensive-checklist/blob/main/graphql.md

## Description
GraphQL security testing checklist: introspection abuse, batching attacks, query depth/complexity DoS, field suggestion enumeration, IDOR via GraphQL, injection through arguments, authorization bypass. Use when assessing GraphQL endpoints in web app tests or bug bounty.

## Trigger Phrases
Use this skill when the conversation involves any of:
`GraphQL, introspection, batching attack, query depth, GraphQL injection, GraphQL IDOR, field suggestion, GraphQL auth bypass, GraphQL DoS, GraphQL security`

## Instructions for Claude

When this skill is active:
1. Load and apply the full methodology below as your operational checklist
2. Follow steps in order unless the user specifies otherwise
3. For each technique, consider applicability to the current target/context
4. Track which checklist items have been completed
5. Suggest next steps based on findings

---

## Full Methodology

# GraphQL Vulnerabilities

## Shortcut

1.  Identify GraphQL Endpoint: Look for common paths like `/graphql`, `/graphiql`, `/graphql.php`, `/graphql/console`. Check network requests in browser developer tools.
2.  Introspection Query: Send an introspection query to fetch the schema. Tools like GraphiQL or Postman can help. `query={__schema{types{name}}}`
3.  Analyze Schema: Look for sensitive types, fields, mutations, and subscriptions. Pay attention to authorization logic.
4.  Test Queries/Mutations:
    - Check for Information Disclosure (e.g., user data, configuration).
    - Test for Authorization Bypass (IDOR, insufficient permission checks).
    - Look for Injection (SQLi, NoSQLi, Command Injection) in input fields.
    - Test for Denial of Service (complex/deeply nested queries, batching abuse).
    - Explore Mutations for unintended state changes.
    - Check Subscriptions for data leakage.
    - Verify persisted/signed queries enforced in production; depth/complexity limits.
5.  No Introspection? Try common field/type guessing (e.g., `user`, `admin`, `query`, `mutation`). Use tools like `clairvoyance` or `inql`.

## Mechanisms

- Over-Fetching: Clients can request excessive data, potentially leading to DoS or information disclosure if not properly limited.
- Under-Fetching/N+1 Problem: Primarily a performance issue—poorly designed resolvers make dozens of backend calls (N+1). While not a direct data‑exposure risk, extreme latency can create timing side‑channels an attacker could measure.
- Insecure Direct Object References (IDOR): Exposing internal IDs allows attackers to potentially access unauthorized data by guessing/enumerating IDs.
- Insufficient Authorization: Missing or flawed checks on types, fields, mutations, or subscriptions.
- Input Validation Issues: Failure to sanitize or validate user input can lead to injection attacks (SQLi, NoSQLi, XSS, SSRF) if resolvers interact with backend systems insecurely.
- Introspection Enabled in Production: Exposes the entire schema, simplifying reconnaissance for attackers.
- Batching Abuse: Sending multiple queries/mutations in a single request can overwhelm the server (DoS) or bypass rate limiting.
- Lack of Depth/Complexity Limiting: Allows excessively nested or complex queries, leading to DoS.
- Directive Flooding: Sending thousands of `@include`/`@skip` directives in a single query can exhaust parser and validation phases, triggering DoS (e.g., CVE‑2024‑47614 in async‑graphql).
- Incremental Delivery: `@defer`/`@stream` can multiply work and leak partial data; must be guarded by cost and auth checks on deferred subtrees.
- File Uploads: Implementations using `graphql-upload` or custom multipart handling can inherit classic upload bugs (path traversal, content-type trust, temp file exposure).
- Federation/Gateway: Cross-subgraph authorization gaps, entity resolver overfetching, and inconsistent role enforcement at the router vs. subgraphs.
- CSRF Considerations: If cookie‑based auth is used, enforce header + `Origin` validation; prefer Authorization header.
- WebSocket Security: GraphQL subscriptions over WebSocket often lack proper authorization on long-lived connections; auth tokens in connection params may not be re-validated after expiry.
- Field Suggestions: Error messages that suggest valid field names when invalid ones are queried can leak schema information even with introspection disabled.
- Relay Global IDs: Base64-encoded `Type:ID` patterns (e.g., `base64("User:123")`) are commonly used and can be decoded to reveal internal IDs.
- Apollo/Hasura Leaks: Production Apollo Server instances may leak schema via query extensions; Hasura permissions misconfiguration can expose direct DB access.
- Header Injection: `x-hasura-*` headers or custom auth headers may be trusted without validation, enabling privilege escalation.

## Hunt

### Preparation

- Identify the GraphQL endpoint(s).
- Obtain the schema via introspection or guessing.
- Understand the application context and potential sensitive data/actions.

### Techniques

- Schema Analysis: Use tools like `GraphQL Voyager` or manually review the schema for sensitive keywords (`admin`, `password`, `config`, `secret`), authorization directives, and complex relationships.
- Query Fuzzing: Use tools like `inql` or custom scripts to fuzz queries, mutations, and arguments.
- Authorization Testing:
  - Try accessing data/mutations meant for higher-privileged users.
  - Test IDOR by replacing IDs in queries/mutations.
  - Check if different roles see different schema subsets (if applicable).
  - Verify router and subgraphs enforce identical authz decisions.
- Injection Testing: Inject payloads (SQL, NoSQL, OS command, XSS, SSRF) into string arguments.
- DoS Testing:
  - Deeply nested queries (`query { user { friends { friends { ... } } } }`).
  - Large limits in list arguments (`query { users(limit: 99999) { id } }`).
  - Query batching abuse.
  - Field duplication/aliases (`query { u1: user(id:1){id} u2: user(id:1){id} ... }`).
  - Directive flooding by attaching a very long chain of `@include` or `@skip` directives to safe fields.
- Incremental delivery pressure: attach many `@defer`/`@stream` segments to expand compute and memory footprint.
- Business Logic Flaws: Test mutations for race conditions, logical errors, or unintended side effects.
- Upload testing: multipart spec edge cases (path traversal via `map`, temp file exposure) and file‑type checks.
- WebSocket Subscription Testing:
  - Tamper with `connection_init` payload (JWT in `connectionParams`)
  - Test subscription flooding without rate limiting
  - Verify auth token expiry is enforced on long-lived WS connections
  - Test for cross-user subscription leaks via predictable subscription IDs
- Field Suggestion Probing: Send invalid field names and analyze error messages for schema hints ("Did you mean...?" responses)
- Relay ID Decoding: Identify base64-encoded global IDs (e.g., `id: "VXNlcjoxMjM="`), decode to extract type and numeric ID, test IDOR
- Apollo Extensions: Try `?extensions={"persistedQuery":{...}}` or check for `apollo-server-testing` header in responses
- Hasura Header Injection: Test `x-hasura-role`, `x-hasura-user-id`, `x-hasura-org-id` headers for authorization bypass

### Advanced Testing

- Reverse engineer client-side code making GraphQL requests.
- Analyze traffic between microservices if GraphQL is used internally.
- Test subscription endpoints for authorization issues and data leakage over time.

## Bypass Techniques

### Introspection Disabled

Use wordlists (SecLists has GraphQL lists) with tools like `clairvoyance` or `GraphQLmap` to guess types, fields, and arguments. Analyze client-side code for hints.

- Quick probe: `query { __typename }` often succeeds even when full introspection is disabled and confirms a GraphQL endpoint.

- Use wordlists (SecLists has GraphQL lists) with tools like `clairvoyance` or `GraphQLmap` to guess types, fields, and arguments. Analyze client-side code for hints.

### Rate Limiting/Complexity Limits

- Use aliases to request the same field multiple times within limits.
- Split complex queries into multiple smaller ones.
- Abuse batching if not properly limited.

### Web Application Firewalls (WAFs)

- Use GraphQL query variations (aliases, fragments, different whitespace).
- Encode payloads within strings.
- Leverage nested input objects if WAF only inspects top-level arguments.
- Abuse incremental delivery: place sensitive fields under `@defer` to evade naive complexity calculators.
- Persisted queries reduce WAF reliance; prefer signature enforcement at edge.

## Vulnerabilities

### Common Patterns

- Publicly exposed GraphiQL interface with introspection enabled.
- Mutations lacking proper authorization checks.
- Resolvers directly using user input in database queries or system commands.
- Fields returning sensitive information not intended for the user's role.
- Lack of query depth/complexity/limit controls.

### Specific Functions/Areas:

- `user`, `account`, `profile` types/queries (Information Disclosure, IDOR).
- `admin`, `settings`, `config` types/queries (Privilege Escalation).
- Mutations involving payments, data modification, or user management.
- Search functionalities (Injection).
- File upload mechanisms via mutations.
- Subscription endpoints.

## Methodologies

### Tools

- Automated Scanners: `StackHawk`, `Invicti`, **Escape** (free SaaS tier), `Nuclei` (GraphQL templates).
- Introspection & Interaction: `GraphiQL`, `Postman`, `Altair GraphQL Client`, `Insomnia`.
- Schema Exploration: `GraphQL Voyager`.
- Exploitation/Fuzzing: `inql` (Burp Suite Extension), `GraphQLmap`, `clairvoyance`, **CrackQL** (JWT extraction from errors), **BatchQL** (batch query fuzzing), custom Python scripts (`requests` library).
- Proxy: Burp Suite, OWASP ZAP (to intercept and modify requests).
- Security Middleware: **GraphQL Armor** – production‑ready depth, alias and complexity limits for Apollo Server, Yoga, Envelop and more.
- Fingerprinting / Recon: **graphw00f** – identifies the underlying GraphQL implementation (Apollo, Yoga, Hasura, etc.) to tailor attacks.
- Security Auditing: **graphql-cop** – security auditing and configuration checking.
- Endpoint Discovery: `Graphinder` and wordlists for path guessing.
- Linters/Policy: `graphql-schema-linter`, `eslint-plugin-graphql`, and custom auth directives unit tests.

### Systematic Process

1.  Reconnaissance (Endpoint discovery, Schema retrieval/guessing).
2.  Schema Analysis (Identify key types, fields, mutations, auth).
3.  Authorization Testing (Role-based access, IDOR).
4.  Input Vulnerability Testing (Injection, XSS, SSRF in arguments).
5.  DoS Testing (Query complexity, batching, limits).
6.  Business Logic Testing (Mutation side-effects, race conditions).
7.  Subscription Testing (if applicable).

### High-Impact Targets

Mutations changing state (user roles, passwords, settings), queries accessing sensitive user data, administrative endpoints.

## Chaining and Escalation

- **IDOR + Mutation**: Discover an IDOR in a query, then use the leaked ID to modify another user's data via a mutation (e.g., change email/password).
- **Information Disclosure + Injection**: Leak database structure/version via a verbose error, then use that info to craft a targeted SQLi payload.
- **SSRF + Internal Endpoint**: Use an SSRF vulnerability in a resolver to interact with internal GraphQL endpoints or other services not directly accessible.
- **Authorization Bypass + Admin Mutation**: Gain access to an administrative mutation (e.g., `updateUserRole`) through flawed authorization, then escalate privileges.
- **XSS + Token Theft**: Inject XSS payload via a vulnerable field, steal authentication tokens from other users viewing the data.

## Remediation Recommendations

- Disable Introspection in Production: Prevent easy schema discovery.
- Implement Strict Authorization: Apply checks at the schema level (directives) and within resolvers for every field, type, mutation, and subscription based on user roles/permissions. Use context passed to resolvers.
- Input Validation & Sanitization: Validate all arguments against expected types, formats, and lengths. Sanitize input before using it in downstream systems (databases, commands). Use parameterized queries.
- Query Cost Analysis: Implement limits on query depth, complexity (e.g., maximum nodes or calculated cost), and amount (limit number of results).
- Rate Limiting: Limit the number of requests per user/IP, including batched queries.
- Persisted & Signed Queries: Enforce automatic persisted queries (APQ) with operation signatures to whitelist allowed operations and block unknown or modified queries.
- Secure Federation Gateways: Keep Apollo Router (or your GraphQL gateway) patched, validate supergraph composition, and enforce authorization at the gateway layer to prevent cross‑subgraph data leaks.
- Caching & CDN Hardening: If responses are cached, partition caches by the `Authorization` header (or disable caching) to avoid shared‑cache data leakage.
- Specific Field Exposure: Avoid exposing sensitive fields (`password`, `internal tokens`). Use dedicated Data Transfer Objects (DTOs) if necessary.
- Error Handling: Return generic error messages; avoid leaking stack traces or internal details.
- Regular Audits & Testing: Perform regular security reviews and penetration tests specifically targeting the GraphQL API.
- Use Security Headers: Apply standard web security headers (CSP, HSTS, etc.).
- Keep Libraries Updated: Ensure GraphQL server libraries and dependencies are patched.
- Incremental Delivery Controls: enforce cost accounting for `@defer`/`@stream`; ensure deferred subtrees still run full auth/visibility checks.
- File Upload Hygiene: if using GraphQL upload, re‑encode images, validate content by signature, and store outside web root; apply all controls from `file-upload.md`.
- Federation RBAC: centralize auth policy in schema directives evaluated at the gateway and in subgraphs; avoid trusting upstream filtering blindly.
