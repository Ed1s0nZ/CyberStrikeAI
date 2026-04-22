---
name: apk-analysis
description: Static APK analysis workflow — hash, decompile with jadx, extract IOCs (network endpoints, API paths, hardcoded secrets, data-collection APIs, crypto primitives), identify suspicious files, and produce per-APK + master IOC reports. TRIGGER when the user hands over one or more .apk files, asks for malware-triage of Android apps, wants to enumerate endpoints/keys/permissions statically, or mentions jadx/apktool/APK IOC extraction. Scoped to static work only — for runtime hooks use the `frida-kahlo-mcp` skill; for deep binary analysis of native libs use `ghidra-mcp`.
version: 1.0.0
---

# APK Analysis — Static Triage Workflow

This skill is a pattern library for **static** analysis of one or more APKs: hashing, decompilation, IOC extraction, report writing. It is deliberately CLI-shaped (jadx + grep + shell) so the agent can parallelize work across dozens of APKs cheaply.

For the **dynamic** half of the loop (attach Frida, hook Java methods, capture runtime URLs/keys/Intents), call the `frida-kahlo-mcp` sibling skill. For deep **native** binary analysis (`.so` files inside the APK's `lib/`, JNI bridges, packer/obfuscator unpacking), call the `ghidra-mcp` sibling skill. APK analysis typically cycles between all three.

## Required tools (verify before use)

Before running the workflow, the agent MUST verify these CLI tools exist and work. Report missing ones to the operator with exact install commands — a failed decompile halfway through 200 APKs wastes more time than 30 seconds of prechecks.

- **`jadx`** — primary DEX → Java decompiler. Version ≥ 1.4.x.
  - Verify: `jadx --version` should succeed.
  - Install: `pip install jadx` (Python wrapper), or download release JAR from [github.com/skylot/jadx](https://github.com/skylot/jadx), or on Debian/Ubuntu `sudo apt-get install jadx`.
  - This fork's `skills/android-reverse-engineering/scripts/install-dep.sh jadx` automates the install.

- **`sha256sum`** — coreutils on Linux / macOS. Usually present.
- **`file`** — coreutils. Usually present.
- **`grep` with `-P` (PCRE) support** — GNU grep on Linux has this by default. macOS default `grep` is BSD and doesn't support `-P`; install `ggrep` via Homebrew and use `ggrep -P` or `grep -E` (extended-regex fallback; some patterns need adjustment).

- **Optional for deeper passes** (flag but don't block if missing):
  - `apktool` — resource/AndroidManifest extraction (covered by the `android-reverse-engineering` skill's scripts).
  - `aapt` / `aapt2` — `getprop`-style APK metadata extraction.
  - `keytool` — cert chain inspection (usually ships with JDK).

If any required tool is missing, surface the gap with a one-liner install command and stop. Don't silently skip steps — the report will have holes the operator can't see.

## Step 1: Hash and inventory

```bash
sha256sum *.apk
file *.apk
ls -lh *.apk
```

Record each APK's SHA256 in the per-APK report header. Same-hash duplicates across a sample set are common (repackaged malware families) — deduplicate before spending compute.

Pull package metadata with `aapt` if available:

```bash
aapt dump badging <apk> | head -20
# package name, versionCode, versionName, targetSdkVersion, permissions
```

## Step 2: Decompile

```bash
# Single APK
jadx --no-res -d decompiled/PKG_NAME apk_file.apk

# Batch (parallel — uses all cores)
for apk in *.apk; do
  name=$(basename "$apk" .apk)
  jadx --no-res -d "decompiled/$name" "$apk" &
done
wait
```

`--no-res` skips resource decompilation to cut time roughly in half. Re-run selected APKs without it when you need `res/`, `assets/`, or the decoded `AndroidManifest.xml`.

If jadx chokes on a particular APK (packed / obfuscated / custom DEX format), escalate to `apktool d` for a raw smali dump, or to Ghidra-mcp's `program.open` on the native libs in `lib/*/`.

## Step 3: Extract IOCs

Run the greps below across `decompiled/*/sources/`. Each produces a noisy first pass — filter by eyeballing the results and tightening patterns.

### Network endpoints

```bash
grep -rhoP 'https?://[a-zA-Z0-9._/-]+' decompiled/*/sources/ | sort -u | \
  grep -v "schemas.android\|xmlns\|w3.org\|apache.org\|google.com/android"
```

Ignore-list expands over time — add hosts that keep appearing in legit platform code (e.g., `googleapis.com/auth`, `crashlytics-sync.googleapis.com`) once you've confirmed they're benign.

### API paths

```bash
grep -rhoP '"(/api/[^"]+|/v[0-9]+/[^"]+)"' decompiled/*/sources/ | sort -u
```

Good for reconstructing a REST surface without touching the app at runtime.

### Hardcoded keys / secrets

```bash
grep -rn "api_key\|apikey\|secret\|token\|password\|Bearer" decompiled/*/sources/ | \
  grep -v "test\|example\|TODO"
```

High false-positive rate — triage each hit. Look for entropic-looking strings (`[A-Za-z0-9+/=]{20,}`) assigned to fields near the matches.

### Data collection APIs

```bash
grep -rn "getDeviceId\|getSubscriberId\|IMEI\|IMSI\|TelephonyManager\|LocationManager\|getLastKnownLocation\|SmsManager\|sendTextMessage\|getInstalledPackages" decompiled/*/sources/ | head -20
```

### Encryption primitives

```bash
grep -rn "AES\|RSA\|DES\|encrypt\|decrypt\|cipher\|SecretKey\|wbcrypto" decompiled/*/sources/ | head -20
```

Hardcoded keys often travel with these — grep again in the immediate file for `byte[]` literals or `getBytes()` calls.

### Firebase / Analytics / Third-party SDKs

```bash
grep -rn "firebase\|analytics\|crashlytics\|measurement\|appsflyer\|adjust\|amplitude\|mixpanel" decompiled/*/sources/ | head -20
```

Useful for attribution (which SDKs ship in this app?) and for finding the hosts those SDKs talk to, which won't show up in the main app code.

### Native entry points

```bash
grep -rn "System.loadLibrary\|loadLibrary(" decompiled/*/sources/ | head
```

Each `loadLibrary("foo")` means there's a `lib*/libfoo.so` inside the APK — hand those to `ghidra-mcp` for further analysis.

## Step 4: Analyze suspicious files

Pick the files flagged by Step 3. Read them completely (not just the grep-matched line). Focus on:

- **Command execution classes** — `Runtime.exec`, `ProcessBuilder`, remote install, force-execute patterns.
- **Data exfiltration** — `HttpURLConnection` / `OkHttpClient` / `Retrofit` POSTs carrying device-identifying data.
- **Persistence** — `BOOT_COMPLETED` receivers, `AlarmManager`, `JobScheduler`, accessibility services, device-admin receivers.
- **Obfuscation markers** — single-letter class names, reflection-driven string decode, inlined base64 strings, `DexClassLoader` / `PathClassLoader` for dynamic-loaded payloads.
- **Permissions/capabilities** — `AndroidManifest.xml` in the decoded resources (re-decompile with `--no-res` removed if needed). Permissions of interest: `SEND_SMS`, `READ_SMS`, `READ_CONTACTS`, `RECEIVE_BOOT_COMPLETED`, `BIND_ACCESSIBILITY_SERVICE`, `BIND_DEVICE_ADMIN`, `REQUEST_INSTALL_PACKAGES`, `SYSTEM_ALERT_WINDOW`.

## Step 5: Per-APK report

For each APK, write a findings report with the following shape:

```
## <package.name> (<versionName>, vc=<versionCode>)

- SHA256:        <hash>
- Size:          <bytes>
- targetSdk:     <n>
- Permissions:   <comma-separated dangerous perms>
- Signer CN:     <from keytool>

### Network IOCs
- <URL 1>
- <URL 2>
- ...

### API surface (reconstructed)
- POST /api/v1/login           { username, password, imei }
- GET  /api/v1/commands        (polled every 60s)

### Data collection
- Reads IMEI (TelephonyManager.getDeviceId() at com/.../DeviceInfo.java:42)
- Reads SMS log (READ_SMS + ContentResolver on content://sms)

### Encryption
- AES/CBC/PKCS5Padding with hardcoded 16-byte key at com/.../Crypto.java:18 (hex: ...)

### Persistence
- BOOT_COMPLETED receiver at com/.../BootReceiver.java

### Severity
<critical | high | medium | low | benign>  — <one-line reason>

### Recommended action
<block hostnames / alert on installs / deeper Frida analysis / submit to threat intel>
```

Write these into `reports/<package.name>.md` so the master report step can concatenate them.

## Step 6: Master IOC report

Compile per-APK findings into `reports/IOC-master.md` with:

- **Domain summary table** — host → count of distinct APKs that reach it → WHOIS + hosting info → severity.
- **IP summary table** — where distinct IP hits resolve (some APKs skip DNS).
- **Data-flow sketch** — rough diagram of which data categories flow to which hosts. A text table is fine; nobody needs Graphviz here.
- **Cross-APK patterns** — shared signer, same hardcoded key, same endpoint family → likely same actor / repackaged family.
- **Prior-art** — grep the top hostnames against public threat-intel feeds (VirusTotal, URLhaus) and note hits.
- **Recommendations** — ordered by impact: block X domains at the perimeter, flag Y signer certs, alert on installs of these package names.

## Parallel analysis via sub-agents

When analyzing more than a handful of APKs, parallelize. CyberStrikeAI's multi-agent orchestrator can hand off each decompiled APK directory to a sub-agent:

1. Use the orchestrator's `task` tool to spawn one sub-agent per APK, each scoped to that APK's `decompiled/<pkg>/` directory.
2. Each sub-agent runs Steps 3–5 and writes `reports/<package.name>.md`.
3. When all sub-agents complete, the orchestrator runs Step 6 to merge.

This maps naturally to the Markdown-agent system (`agents/*.md`) — see the `agents/orchestrator.md` body for the task-delegation shape. Keep sub-agents stateless (everything they need is in their APK directory) so restart is cheap.

## Human-in-the-Loop Handoff

Deep findings (suspicious C2 domain with certainty, novel evasion technique, hardcoded key for a known service) deserve explicit operator review before pivoting to response actions. Emit a handoff block like:

```
### Review target: <pkg.name>
Hash:     <sha256>
Finding:  Hardcoded Firebase admin token in com/.../Prod.java:118 — grants write to
          `firestore://<proj>.firebaseio.com` according to JWT iss/aud.
Evidence: decompiled/<pkg>/sources/com/.../Prod.java lines 108-130
Next-step options:
  1. Validate token scope against Firebase project (operator decides whether this is in scope)
  2. Hand APK to `frida-kahlo-mcp` for runtime confirmation of auth flow
  3. Submit to threat intel if actor attribution is the goal
```

Let the operator pick the next step. The agent does the breadth (pattern-hunt across the sample set); the operator does the depth on the flagged items.

## Further reading

- `skills/android-reverse-engineering/` — companion skill covering the install scripts (`install-dep.sh jadx|vineflower|dex2jar`), the decompile.sh wrapper, and find-api-calls.sh helper.
- `skills/frida-kahlo-mcp/` — dynamic hooks; pair with static findings.
- `skills/ghidra-mcp/` — native library analysis (`.so` files inside `lib/`).
- [jadx user guide](https://github.com/skylot/jadx) — upstream CLI reference.
