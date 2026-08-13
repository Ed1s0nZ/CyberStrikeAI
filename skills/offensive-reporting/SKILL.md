---
name: offensive-reporting
description: Penetration test and red team report writing methodology. Covers executive
  summary structuring (risk-led narrative for non-technical readers), technical finding
  format (title, severity, affected scope, narrative, reproduction steps, impact,
  remediation, references), CVSS v3.1 / v4.0 scoring with vector justification, OWASP
  risk rating, evidence hygiene (redacting credentials, hashing client data, time-stamping
  every action), screenshot and PoC artifact management, finding chain narratives,
  scope/limitations/assumptions documentation, retest evidence and remediation tracking,
  deliverable formats (PDF, DOCX, HTML, JSON for SIEM ingestion), client-customer-deliverable
  separation, and common report mistakes (over-CVSSing, undermining the triager, missing
  the 'so what'). Use at the end of an engagement when authoring a deliverable, when
  restructuring a draft for executive readability, or when establishing a reusable
  report template for a consulting practice.
---

# Penetration Test Reporting — Professional Methodology

A great finding lost in a bad report is a wasted finding. Reports are the artifact the client pays for, the auditor reads, and the developer fixes from. Treat the report with the same rigor as the exploit.

## Quick Workflow

1. Capture evidence as you exploit — never reconstruct after the fact
2. Draft each finding immediately while context is fresh; one finding = one numbered file
3. Build the executive summary last, after all findings are scored
4. Two-pass review: technical accuracy first, then read-as-CISO for narrative
5. Hand off with a retest plan and a JSON/CSV index for the client's tracking system

---

## Report Structure (Standard)

```
1. Executive Summary             ← Last to write, first read
2. Engagement Overview
   2.1 Scope
   2.2 Methodology
   2.3 Limitations / Assumptions
   2.4 Timeline
   2.5 Team
3. Risk Summary                  ← Heatmap, finding count by severity
4. Technical Findings            ← One per finding, sorted by severity
5. Attack Narratives / Chains    ← Critical chains called out separately
6. Strategic Recommendations     ← Programmatic, not finding-by-finding
7. Appendices
   A. Tools Used
   B. Indicators of Compromise (for blue team)
   C. Raw Evidence Pointers
   D. Glossary
```

---

## Executive Summary — The 90-Second Read

The executive summary is for the CISO, the GRC officer, and the board member. They read this and nothing else.

**Structure (one page max):**

1. **Engagement context** — what was tested, when, by whom (1 sentence)
2. **Headline finding** — the worst thing you found, in business terms (2–3 sentences)
3. **Risk verdict** — overall posture in plain language (1 paragraph)
4. **Counts** — number of findings by severity, in a small table
5. **Top 3 strategic recommendations** — programmatic fixes, not "patch CVE-X"

**Words to avoid in the executive summary:**
`payload`, `RCE`, `XSS`, `LDAP`, `SMB`, `kerberos`, `injection`. Translate every one. ("An attacker could run arbitrary commands on the server" not "RCE via deserialization gadget chain.")

**Words to include:**
Business impact (`customer data`, `regulatory exposure`, `operational disruption`, `financial loss`). Anchor every finding to a business consequence.

---

## Technical Finding Template

```markdown
## Finding ID — Short Descriptive Title

**Severity:** Critical (CVSS 9.8 — vector below)
**Affected Scope:** <hosts/URLs/components, with version where relevant>
**Status:** Open / Fixed in retest / Accepted Risk
**CWE:** CWE-89 (SQL Injection)
**OWASP:** A03:2021 — Injection

### Summary
One paragraph. What is the finding, why does it matter, what's the worst case.

### Background
What technology is involved and why this class of bug exists. Two paragraphs max.
Skip if obvious (e.g. don't explain XSS to an XSS shop).

### Description
Detailed walkthrough of the issue. The root cause, not just the symptom.

### Reproduction Steps
1. Numbered, copy-paste ready.
2. Include the exact request/response, redacted.
3. A reader with no engagement context should reproduce in <15 minutes.

### Evidence
- `screenshots/finding-007/01-payload.png`
- `requests/finding-007/initial-poc.http`
- `evidence-log.csv` line 142 (timestamp 2025-04-12 14:33:07Z)

### Impact
Concrete. Quantified where possible.
- "Read access to the entire customer table (~2.3M records)"
- "Authenticate as any user; verified for sample ID 1, 2, 999, 1000000"
- "Cross-tenant access — verified by reading data from acquired-tenant ABC"

### Remediation
Specific, actionable, ordered by precedence:
1. **Fix the bug** — exact code change or config flag
2. **Defense in depth** — secondary control (WAF rule, input validation)
3. **Detection** — log line / SIEM rule that would have caught the exploit

### References
- CWE / OWASP / CAPEC
- Vendor advisory if known CVE
- Blog posts only if directly relevant

### Notes for Retest
What you'd do to verify the fix. Specific request, specific expected response.
```

---

## Severity Scoring

### CVSS v3.1 Discipline

CVSS is a tool, not a verdict. Score it, then sanity-check against business impact.

```
CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H = 9.8 Critical
```

For every metric, justify the choice in one sentence:
- `AV:N` — exposed to internet (port 443)
- `AC:L` — no special preconditions
- `PR:N` — no authentication needed
- `UI:N` — no user interaction
- `S:U` — does not cross security scope
- `C:H` `I:H` `A:H` — full read/write/availability impact on the database

If two reasonable people would score it differently, document why you chose what you chose.

### When CVSS Lies

CVSS doesn't capture business context. A "Medium" CVSS XSS in the customer support chat panel that authenticated agents use to handle PII is more dangerous than an unauthenticated "High" SSRF on a metadata-less internal service. Use CVSS as the floor, not the ceiling.

In those cases, score CVSS honestly and then **add a "Business Impact Adjustment"** paragraph that argues for higher reporting severity. Don't lie with CVSS.

### CVSS v4.0 (where required)

CVSS v4.0 adds environmental and threat metrics that better capture real-world risk. Use it when the client mandates it (PCI DSS 4.0 trends this way) — otherwise v3.1 stays the lingua franca.

### OWASP Risk Rating (alternative)

For web-app-only engagements where CVSS feels stretched, OWASP's risk rating (likelihood × impact across multiple factors) often communicates better.

---

## Evidence Discipline

### What to Capture

For every finding, every action:

1. **Timestamp** (UTC, ISO 8601)
2. **Source IP** (yours, including any pivot)
3. **Target** (host, URL, RPC interface)
4. **Action** (what request was sent)
5. **Result** (response, what you got)
6. **Hash** of any data extracted (so you can prove what you saw)

```csv
timestamp,operator,src_ip,target,action,result_hash,notes
2025-04-12T14:33:07Z,KA,10.10.10.5,app.client.com,SQLi probe ' OR 1=1--,sha256:abc...,initial detection
```

This is the audit trail. Clients with mature security teams will ask for it.

### Redaction Rules

Before any artifact leaves your secure environment:

- **Replace credentials** with placeholders: `<REDACTED-PASSWORD>`, `<TOKEN-A1>`
- **Hash extracted PII** — never include real names, emails, SSNs in screenshots
- **Crop screenshots** to the relevant area; check for browser tab leaks (other tabs visible)
- **Strip EXIF** from images; auto-redact via `exiftool -all= *.png`
- **Remove debug toolbars** from screenshots that reveal client infrastructure paths
- **Verify URLs in screenshots** don't include session tokens

### Storage & Chain of Custody

- Encrypted volume during the engagement (LUKS, FileVault, BitLocker)
- Per-engagement key, not a master operator key
- Wipe to client-spec at end of engagement (typically 30–90 days post-delivery)
- Retain only the report and a hash manifest of evidence, deletable on request

---

## Scope, Limitations, and Assumptions

These three sections protect both you and the client. Be explicit.

### Scope
- IPs / domains / repos / accounts in scope, with start/end of engagement window
- Excluded: third-party SaaS used by the client (they don't own it)
- Out of scope by request: physical, social engineering against staff, DoS

### Limitations
- "Testing was conducted from the internet only; no internal network access provided"
- "Source code review was not in scope"
- "Production database mutations were avoided per ROE"
- "No coordinated downtime — testing windows were 22:00–06:00 UTC"

### Assumptions
- "We assumed the staging environment mirrors production"
- "We assumed the WAF in front of app.client.com is the same as production"
- "Service accounts with admin rights were assumed pre-existing"

---

## Risk Summary & Heatmap

Show, don't tell. A visual summary every executive can read in 5 seconds:

```
Severity   Count   Top Example
Critical     3     RCE via deserialization (Finding #2)
High         7     ADCS ESC1 → Domain Admin (Finding #11)
Medium      14     Stored XSS in customer support panel (Finding #4)
Low         22     TLS 1.0 still enabled on api.client.com (Finding #29)
Info        11     —
```

A simple bar chart or stoplight grid converts this to a one-glance summary. Put it on page 2 (after exec summary).

---

## Attack Chains / Narratives

Critical findings rarely matter in isolation. The chain is the story:

```
1. Phishing email → user runs HTA payload (Finding #1, Medium)
2. Local UAC bypass via Token Manipulation (Finding #5, Low)
3. Kerberoast service account (Finding #11, High)
4. Crack TGS offline → service account password (Finding #11)
5. ACL abuse: service account has WriteDacl on Domain Users (Finding #14, High)
6. Grant DCSync, dump krbtgt → Golden Ticket → Domain Admin (Finding #15, Critical)

Total time: 4 hours. Detection points missed: 3 (see Appendix B).
```

Highlight chains separately because the *combination* often warrants higher severity than any individual finding.

---

## Strategic Recommendations

Below the per-finding remediations, write 3–5 programmatic recommendations:

- "Adopt SAST in CI for Java services" (addresses 12 findings)
- "Roll out tier-0 admin model for AD" (addresses entire AD attack chain)
- "Centralize secrets in HashiCorp Vault; rotate hardcoded creds" (addresses 9 findings)

This is what the CISO presents to the board. Make it memorable.

---

## Deliverable Formats

| Format | Use |
|--------|-----|
| **PDF** | Executive read, formal record, contractual deliverable |
| **DOCX** | If the client wants to redact or extend |
| **HTML** | Internal portal upload, searchable via grep |
| **JSON** | SIEM / GRC tool ingestion (DefectDojo, Faraday, ServiceNow) |
| **CSV** | Quick import into Jira / Asana for tracking |
| **Markdown source** | The single source of truth that generates all the above |

Build all formats from one Markdown source via Pandoc / a static site generator. Never maintain parallel formats by hand.

```bash
# Markdown → polished PDF via Pandoc + LaTeX template
pandoc report.md -o report.pdf \
  --template=client-template.tex \
  --pdf-engine=xelatex \
  --metadata=title:"Penetration Test Report — Client Co." \
  --toc --number-sections
```

---

## Common Report Mistakes

| Mistake | Fix |
|---------|-----|
| CVSS 9.0 on every finding ("over-CVSSing") | Score honestly; clients lose trust if everything is critical |
| Marketing language ("revolutionary attack") | Plain professional tone |
| Tool output dumped as evidence | Curate; show the relevant 5 lines |
| Generic remediation ("validate input") | Specific code/config changes |
| Missing reproduction steps | If they can't reproduce, they can't fix |
| Untimed evidence | Every action gets a UTC timestamp |
| Confusing identical findings | Group by class, list affected items in a table |
| Forgotten retest plan | Each finding includes how you'll verify the fix |
| Failure to separate scope from limitations | Scope = what we tested; Limitations = what blocked us |
| Treating informational findings as filler | Either drop them or write them well |

---

## Reporting for Bug Bounty (Different Audience)

Bug bounty triagers are time-pressured and skeptical. Adjust:

- **Title**: include the bug class + endpoint + impact in 80 chars
- **Reproduction**: a single curl command if possible, plus the expected vs actual response
- **Impact**: anchor to the program's threat model (read PII? auth bypass? cross-account?)
- **Avoid**: walls of text, screenshots without a request log, claims without reproduction

A good bounty report is read in 2 minutes and reproduced in 5. A bad one bounces with "more info."

---

## Retest & Closeout

```markdown
### Retest Summary

| Finding | Original Severity | Retest Status | Verification Date |
|---------|-------------------|---------------|-------------------|
| #1 | Critical | ✓ Fixed (verified) | 2025-05-10 |
| #2 | High | ✓ Fixed | 2025-05-10 |
| #5 | Medium | ⚠ Partially fixed — see notes | 2025-05-10 |
| #11 | High | ✗ Not fixed — finding stands | 2025-05-10 |
| #14 | Low | • Accepted Risk (client decision) | 2025-05-10 |
```

For each finding, include the exact verification request/response showing the fix. Without proof, "fixed" is hearsay.

---

## Sample CVSS Vectors (Reference)

| Class | Typical Vector | Score |
|-------|---------------|-------|
| Unauth RCE | `AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H` | 9.8 |
| Authed RCE | `AV:N/AC:L/PR:L/UI:N/S:U/C:H/I:H/A:H` | 8.8 |
| Stored XSS | `AV:N/AC:L/PR:L/UI:R/S:C/C:L/I:L/A:N` | 5.4 |
| IDOR (PII read) | `AV:N/AC:L/PR:L/UI:N/S:U/C:H/I:N/A:N` | 6.5 |
| SSRF (cloud meta) | `AV:N/AC:L/PR:L/UI:N/S:C/C:H/I:H/A:N` | 9.0 |
| Open Redirect | `AV:N/AC:L/PR:N/UI:R/S:U/C:N/I:L/A:N` | 4.3 |

Use these as starting points; adjust per environment.

---

## Tooling

| Tool | Use |
|------|-----|
| Pandoc + LaTeX | Markdown → polished PDF |
| Sphinx / mkdocs | Markdown → HTML portal |
| DefectDojo | Finding tracking, JSON export |
| Faraday | Multi-engagement aggregation |
| Dradis | Collaborative report drafting |
| serpico (legacy but still used) | Pentest report templates |
| Plextrac | Commercial reporting platform |

---

## Key References

- NIST SP 800-115 (technical security testing reporting)
- PTES — Penetration Testing Execution Standard, reporting section
- OWASP Testing Guide — reporting chapter
- FIRST CVSS v3.1 / v4.0 specifications
- CREST Cyber Security Incident Response and Penetration Testing reporting standards
- Source: https://github.com/SnailSploit/offensive-checklist/blob/main/reporting.md
