---
name: frida-kahlo-mcp
description: Dynamic Android instrumentation via Frida/Kahlo MCP — attach or spawn processes, inject Frida scripts as isolated jobs, stream telemetry, dump artifacts, hook Java and native methods, unpin TLS, extract crypto keys, and iterate via drafts→modules. TRIGGER when the user asks to reverse-engineer Android apps at runtime, hook methods, unpin SSL/TLS, extract keys at runtime, trace API calls, bypass root/debugger detection, inspect Intents, dump memory from a running process, or mentions Frida/Java hooks/runtime instrumentation. Covers the 26 tools exposed by the `frida-kahlo` MCP server plus the full `ctx.stdlib.*` reference (9 namespaces, 50+ helpers).
version: 1.0.0
---

# Frida Kahlo MCP — Operational Reference

This skill documents the `frida-kahlo` MCP server integrated with CyberStrikeAI. Kahlo is a Frida-based Android dynamic instrumentation server that exposes **26 structured tools** over MCP stdio. It manages devices, spawns/attaches to processes, injects an in-process orchestrator agent, runs isolated jobs as Frida scripts, streams cursor-paginated telemetry, and stores artifacts and reusable code modules on disk.

Authoritative design document: [github.com/FuzzySecurity/kahlo-mcp](https://github.com/FuzzySecurity/kahlo-mcp) (the upstream project) and the live `kahlo_mcp_about` tool output. Background reading: the blog post at [knifecoat.com/Posts/Scalable+research+tooling+for+agent+systems](https://knifecoat.com/Posts/Scalable+research+tooling+for+agent+systems).

## Required tools (verify before use)

Before acting on anything in this skill, the agent MUST verify each item is present AND functional. Failed hooks ten minutes into an analysis cost more than a 30-second precheck. Report missing items to the operator with the remediation command rather than proceeding blind.

- **`frida-kahlo` MCP server is connected.**
  - Call `mcp__frida-kahlo__kahlo_mcp_about` — expect `{ok: true, data: {...}}` with the toolkit contract.
  - If the tool is not in the available set at all, the server isn't registered. Remediation: enable the `frida-kahlo` entry under `external_mcp.servers` in CyberStrikeAI's `config.yaml` (template block lives in `config.example.yaml`; see [docs/mcp-setup.md](../../docs/mcp-setup.md)).
  - If the tool is listed but `kahlo_mcp_about` errors out, the server is registered but unhealthy. Run `scripts/frida/start-frida-kahlo-mcp.sh --install` from the CyberStrikeAI repo root to verify/repair, then restart CyberStrikeAI.

- **ADB is reachable and a device is visible.**
  - `mcp__frida-kahlo__kahlo_devices_list` → expect a non-empty `devices` array. Empty array means ADB is running but no device is attached (check cable / emulator / `adb connect`).
  - If the call fails entirely, `adb` isn't on the server's PATH. Install it (Debian/Ubuntu: `sudo apt-get install android-tools-adb`; macOS: `brew install android-platform-tools`), or set `KAHLO_ADB_PATH` in the `env:` block of the `frida-kahlo` entry in `config.yaml`.

- **frida-server is running on the target device.**
  - Call `mcp__frida-kahlo__kahlo_devices_health` for the device. Status `healthy` = good. `degraded` / `unavailable` = frida-server missing or wrong version.
  - Remediation: push `frida-server` matching the `frida` npm version the kahlo-mcp build uses (see `package.json` in the kahlo-mcp checkout) to `/data/local/tmp/frida-server` and launch as root. Downloads: [frida/frida releases](https://github.com/frida/frida/releases).

- **Target process / package is installed and runnable.** For `mode="attach"` the process must be running (`kahlo_processes_list` will show it). For `mode="spawn"` the package must be installed (`adb shell pm list packages | grep <pkg>`).

If any precondition fails, stop and report to the operator. Do not proceed with hooks against an unhealthy target — the failures will cascade and waste the operator's time.

## When To Use This Skill

Invoke this reference whenever any of the following is true:

- The operator hands the agent an Android app and asks for **runtime analysis** (`.apk` is installed, APK already deployed, "running on my phone", "hook at runtime").
- The request mentions **attach**, **spawn**, **hook**, **trace**, **intercept**, **unpin TLS**, **bypass root detection**, **dump memory**, **extract keys**, **log Intents**, **sniff API calls**, **inspect class loaders**, or **find live heap instances**.
- Frida, frida-server, frida-compile, frida-java-bridge, `Java.use`, `Interceptor.attach`, `Interceptor.replace`, `Module.findExportByName`, NativePointer work, or `send()/recv()` come up by name.
- A larger mobile-forensics or malware-triage workflow needs **live** evidence that static decompilation (jadx, Ghidra) cannot give you: runtime-generated URLs, TLS certificates held only in memory, decrypted strings constructed at runtime, KeyStore-held keys, Bundle extras sent via broadcasts.

**Use this instead of shelling out to `frida-cli`/`frida-trace`/`frida -U -n`** because: (1) jobs are fully isolated per Frida script so cancelling cleans up all hooks automatically, (2) events flow through a cursor-paginated buffer with back-pressure, (3) artifacts handle large payloads without filling tool output, (4) the orchestrator survives app crashes and supports reinjection, (5) the built-in `ctx.stdlib` replaces ~50 common boilerplate snippets with hardened helpers.

Use `ghidra-mcp` (sibling skill under `skills/ghidra-mcp/`) alongside this one for static reverse engineering of native libs or compiled code. Use `apk-analysis` (sibling skill) for decompilation of app Java bytecode with jadx. Kahlo is the **dynamic half** of the analysis loop.

## Server Configuration

The server runs under CyberStrikeAI's external MCP federation. Its lifecycle is managed by `scripts/frida/start-frida-kahlo-mcp.sh` — CyberStrikeAI spawns that script via stdio when the `frida-kahlo` entry in `config.yaml` has `external_mcp_enable: true`.

Relevant environment variables (set them in the `env:` block of the config entry, not in the shell — `config.yaml` values take precedence):

- `KAHLO_MCP_HOME` — path to the `kahlo-mcp` checkout. Defaults to `~/kahlo-mcp` (auto-cloned on first `--install`).
- `KAHLO_ADB_PATH` — full path to the `adb` binary. Auto-detected via `which adb` when left empty.
- `KAHLO_DATA_DIR` — where kahlo-mcp stores runs/modules/drafts/snapshots/artifacts. Default: `<KAHLO_MCP_HOME>/data/`.
- `KAHLO_LOG_LEVEL` — `info` by default.

The launcher script auto-installs Node.js (if missing), clones `FuzzySecurity/kahlo-mcp`, runs `npm install` + `npm run build`, and writes a default `config.json`. On first run it prompts the operator if `adb` is missing (NOT auto-installed — platform-specific udev/USB permissions make that a bad idea silently).

**Data layout** (created on first use under `KAHLO_DATA_DIR`):

- `runs/` — per-job telemetry streams
- `modules/` — versioned frozen modules
- `drafts/` — mutable drafts for iteration
- `snapshots/` — persisted snapshot payloads
- `artifacts/*.bin` + `artifacts.jsonl` — artifact storage (max 10 MB each)

**Reload config at runtime**: edit `<KAHLO_MCP_HOME>/config.json` and restart CyberStrikeAI (the MCP server is respawned with the new config on the next external-MCP start).

For interactive smoke-testing without CyberStrikeAI in the loop:

```bash
./scripts/frida/start-frida-kahlo-mcp.sh
# Ctrl+C to stop. Feed JSON-RPC manually with mcp-cli or @modelcontextprotocol/inspector.
```

## MCP Response Envelope (shared by all 26 tools)

Every tool returns the Kahlo standard envelope:

```json
{
  "ok": true,
  "data": { ... }
}
```
or
```json
{
  "ok": false,
  "error": {
    "code": "NOT_IMPLEMENTED|INVALID_ARGUMENT|NOT_FOUND|UNAVAILABLE|INTERNAL|TIMEOUT",
    "message": "...",
    "tool": "kahlo_...",
    "retryable": true|false,
    "details": { ... },
    "suggestion": "..."
  }
}
```

Always check `ok` before reading `data`. The `suggestion` field on errors tells you which tool to call next to recover (e.g., `"Re-attach to the target using kahlo_targets_ensure"`). **Treat the suggestion as the first thing to try on retry.**

Responses appear in both `content[0].text` (human-readable JSON) and `structuredContent` (canonical machine-readable JSON). Prefer `structuredContent` when chaining calls.

## Core Concepts (read this before running anything)

Kahlo models five first-class entities. Understanding them prevents 90% of mistakes.

### Target = one instrumented process

A **target** is an Android process hosting the in-process orchestrator agent. Identity: `target_id`. Key fields:

- `device_id` — which device
- `package` — for `attach`, the *process name* from `kahlo_processes_list` (e.g., `LINE`, `Chrome`); for `spawn`, the Android package id (e.g., `jp.naver.line.android`, `com.android.chrome`). **These are usually different.**
- `pid` — set when attached
- `mode` — `spawn` | `attach`
- `gating` — `none` | `spawn` | `child`
- `state` — `pending` | `running` | `dead` | `detached`
- `agent_state` — `not_injected` | `ready` | `crashed` | `reinjecting`
- `parent_target_id` — set when captured by `gating='child'` (Android: not used)

**Invariants**:

- You must create/ensure a target before starting jobs.
- Targets can die (app crash, OOM, anti-debug kills). Crashes are normal. Reattach via `kahlo_targets_ensure`.
- `gating='child'` **does not work on Android** — Android uses zygote to fork processes, so Frida's spawn-gating child capture is blind. For multi-process apps, enumerate child process names via `kahlo_processes_list` (look for `com.example.app:background`, `com.example.app:service`, etc.) and attach to each separately.

### Job = one Frida script instance running inside a target

A **job** is a unit of instrumentation code. Identity: `job_id`. Key fields:

- `target_id`
- `type` — `oneshot` | `interactive` | `daemon`
  - `oneshot`: runs `start(params, ctx)` once; when it returns, job completes with `result`. Use for "dump X and exit".
  - `daemon`: long-lived. MUST call `ctx.heartbeat()` periodically (once every few seconds) or kahlo will flag it as stalled. Use for "hook this and emit events forever until cancelled".
  - `interactive`: reserved for future two-way RPC; treat as daemon for now.
- `state` — `queued` | `starting` | `running` | `completed` | `failed` | `cancelled`
- `heartbeat` — ISO 8601 timestamp
- `metrics` — `{ events_emitted, hooks_installed, errors }` (`hooks_installed` is auto-tracked when you use `ctx.stdlib.hook.*`)
- `is_bootstrap` — true for jobs created by `gating='spawn'` bootstrap

**Invariant — isolation**: each job runs in its own Frida script. Cancelling a job unloads the script, and Frida automatically removes every `Interceptor.attach`, method replacement, timer, and piece of state it installed. **You never need manual cleanup code**. When iterating, `kahlo_jobs_cancel` the old job before starting the new one — the next job sees a clean target with no leftover hooks.

Multiple jobs can run concurrently in the same target without interfering. This is how you layer instrumentation (e.g., one job for TLS unpinning, another for URL logging, a third for crypto key extraction).

### Module = a versioned, reusable instrumentation bundle

A **module** is a frozen, semver-versioned blob of JS source (plus manifest) stored on disk. Identity: `module_id@version` (aka `module_ref`, e.g., `tls.unpin@1.0.0`). Modules live in `<KAHLO_DATA_DIR>/modules/` and are immutable once promoted.

### Draft = a mutable, iteration-friendly module

A **draft** is a temporary module you can edit and re-test. Identity: `draft_id`. Drafts live in `<KAHLO_DATA_DIR>/drafts/`. Typical flow:

```
createDraft(source) → start job from draft → observe events → updateDraft → cancel + start fresh → promoteDraft → versioned module
```

### Event = small structured telemetry

Jobs emit **events** via `ctx.emit(kind, payload, level)`. Events are buffered per-target/per-job and fetched with cursor pagination via `kahlo_events_fetch`. Keys: `ts`, `target_id`, `pid`, `job_id`, `kind`, `level`, `payload`. The buffer drops events (with dropped markers) under overload — it will never crash the host.

Filter shape on `kahlo_events_fetch`: `{ kind?: string, level?: 'debug'|'info'|'warn'|'error' }`. Both optional, combined with AND. Exact match only — no glob.

### Artifact = large output stored on disk

For anything bigger than a few KB (memory dumps, file dumps, trace logs, PCAPs, screenshots), jobs call `ctx.emitArtifact({type, mime?, name?, metadata?}, bytes)`. The host persists to `<KAHLO_DATA_DIR>/artifacts/<id>.bin` and records in `artifacts.jsonl`. Fetch metadata with `kahlo_artifacts_list` and payload (inline if ≤ 32 KB, else `storage_ref`) with `kahlo_artifacts_get`. **Max 10 MB per artifact**. For bigger payloads, split before emitting.

## The 26 Tools (organized by workflow phase)

All tools are prefixed `kahlo_` and appear as `mcp__frida-kahlo__kahlo_<name>` in the agent tool namespace (mentions below drop the MCP prefix for readability).

### Phase 0 — Introspection

- **`kahlo_mcp_about`** — returns the full operational contract (concepts, workflows, failure modes, stdlib reference). **Call this when you haven't used Kahlo in a while** — it's self-documenting and re-grounds you in one call. The payload is ~30 KB structured JSON.

### Phase 1 — Devices and ADB

- **`kahlo_devices_list`** — list connected devices with `device_id`, `model`, `transport` (`USB`|`TCP`). Empty array means ADB is running but nothing is attached.
- **`kahlo_devices_get({device_id})`** — device details plus `android_version`, `availability` (`available`|`busy`|`offline`), `frida_server_present`, `frida_server_running`.
- **`kahlo_devices_health({device_id})`** — one-call health check. Status: `healthy` | `degraded` | `unavailable`. Use before attempting target creation.
- **`kahlo_adb_command({command: string[], device_id?, timeout_ms?=30000})`** — run any `adb` command. `command` is an argv array, NOT a shell string (no shell parsing, no quoting traps). Root-gated paths: wrap with `su -c`:
  ```json
  {"command": ["shell", "su", "-c", "ls /data/data/com.example.app"]}
  {"command": ["shell", "su", "-c", "cat /data/data/com.example.app/shared_prefs/auth.xml"]}
  ```
  Common recipes:
  - List installed packages: `["shell", "pm", "list", "packages", "-3"]` (third-party only)
  - Package info: `["shell", "dumpsys", "package", "com.example.app"]`
  - Start activity: `["shell", "am", "start", "-n", "com.example.app/.MainActivity"]`
  - Android version: `["shell", "getprop", "ro.build.version.release"]`
  - SELinux mode: `["shell", "getenforce"]`
  - Force-stop: `["shell", "am", "force-stop", "com.example.app"]`

### Phase 2 — Processes

- **`kahlo_processes_list({device_id, scope?})`** — enumerate running processes as `{pid, name, parameters?}`. Scope values:
  - `"minimal"` (default) — just pid + name
  - `"metadata"` — includes bundle metadata
  - `"full"` — includes icons and rich metadata (expensive)
  
  Use this **before** `mode="attach"` to find the exact Frida process name. **Important**: Frida process names are display names (e.g., `"LINE"`, `"Chrome Beta"`) and are NOT the Android package id. When spawning, you need the package id instead.

### Phase 3 — Targets (lifecycle)

- **`kahlo_targets_ensure`** — create or re-use a target. This is the single most complex tool; get it right and everything else falls into place.

  Parameters:
  - `device_id` — from `kahlo_devices_list`
  - `package` — process name (attach) or package id (spawn)
  - `mode` — `"attach"` | `"spawn"`
  - `gating?` — `"none"` (default) | `"spawn"` | `"child"`
  - `bootstrap?` — a module selector (`{kind: "source", source: "..."}` | `{kind: "draft_id", draft_id: "..."}` | `{kind: "module_ref", module_ref: "name@version"}`). **Required** when `gating="spawn"` or `"child"`.
  - `bootstrap_params?` — JSON passed to the bootstrap job's `start(params, ctx)`
  - `bootstrap_type?` — job type for the bootstrap (defaults to `oneshot`)
  - `child_bootstrap?`, `child_bootstrap_params?`, `child_bootstrap_type?` — only relevant for `gating="child"` (non-functional on Android; leave unset).

  **Decision matrix**:
  
  | Goal | mode | gating | bootstrap |
  |---|---|---|---|
  | Observe a running app mid-session | `attach` | `none` | — |
  | Inspect an app that keeps backgrounded | `attach` | `none` | — |
  | Hook `Application.onCreate` or earliest init | `spawn` | `spawn` | required (installs hooks while suspended) |
  | Spawn fresh without early hooks | `spawn` | `none` | — |
  | Multi-process (e.g., foreground+service) | attach to each process name separately | `none` | — |

  When `gating="spawn"` is used, the bootstrap job runs **while the app is suspended**; the app auto-resumes when the bootstrap's `start()` returns (or its promise resolves). No separate resume step is required. Install early hooks inside the bootstrap, then either leave it as a daemon or return from it to let the app run.

- **`kahlo_targets_status({target_id})`** — `state` + `agent_state` + recent crash/reinjection info. Use to diagnose why jobs are failing before calling `kahlo_targets_ensure` again.
- **`kahlo_targets_detach({target_id})`** — clean detach; cancels all jobs for that target. Always call this at the end of a session.

### Phase 4 — Jobs (instrumentation code)

- **`kahlo_jobs_start`** — start a job inside an existing target.
  
  Parameters:
  - `target_id`
  - `type?` — `"oneshot"` (default) | `"interactive"` | `"daemon"`
  - `ttl?` — optional time-to-live in milliseconds; job auto-cancels after TTL expires
  - `params?` — JSON passed to the job's `start(params, ctx)`
  - `module` — one of:
    - `{ kind: "source", source: "module.exports = { start(params, ctx) { ... } }" }` — inline JS for fast iteration
    - `{ kind: "draft_id", draft_id: "..." }`
    - `{ kind: "module_ref", module_ref: "name@version" }`

  For iteration, **always use `kind: "source"`** first — it's the fastest loop. Once the code is stable, promote it via `kahlo_modules_createDraftFromJob` → `kahlo_modules_promoteDraft`.

- **`kahlo_jobs_status({job_id})`** — lifecycle + last heartbeat + metrics + terminal result/error. Poll this to decide whether to cancel a stuck daemon.
- **`kahlo_jobs_list({target_id})`** — all jobs for a target (active + historical). Historical entries retain metrics and final state.
- **`kahlo_jobs_cancel({job_id})`** — cancel. Frida unloads the script; all hooks/timers/state installed by that job are automatically removed. This is the intended cleanup path — there is no "stop hook" tool.

### Phase 5 — Events and Snapshots

- **`kahlo_events_fetch({target_id? | job_id?, cursor?, limit?=50, filters?})`** — cursor-paginated event stream. Either `target_id` or `job_id` scopes the query. Pass `cursor` from the previous response to continue.
  
  Filter shape: `{ kind?: string, level?: 'debug'|'info'|'warn'|'error' }` — both optional, exact match only, combined with AND.
  
  Responses include `{events: [...], next_cursor: "...", dropped?: N}`. Drop markers indicate buffer overflow. On drop, lower emit rate in the job or tighten your filters and poll more frequently.

- **`kahlo_snapshots_get({target_id, kind, options?})`** — on-demand synchronous state query via the orchestrator agent. Supported kinds:
  - `"native.modules"` — enumerate loaded native modules (name, base, size)
  - `"process.info"` — process details (pid, tid, env, etc.)
  
  Snapshots have a 10-second timeout and can be **expensive** — use sparingly and prefer events for continuous data.

### Phase 6 — Artifacts

- **`kahlo_artifacts_list({target_id? | job_id?})`** — metadata-only listing of artifacts produced by the scope.
- **`kahlo_artifacts_get({artifact_id})`** — full metadata plus inline base64 payload (for ≤ 32 KB artifacts) or a `storage_ref` path for larger ones. Read the file at `storage_ref` via a shell tool (`cat`, `xxd`, etc.) or the operator's file manager.

### Phase 7 — Modules and Drafts

- **`kahlo_modules_list`** — list versioned modules in the store (name, versions, manifest summary).
- **`kahlo_modules_get({module_ref})`** — full source + manifest + provenance for a specific `name@version`.
- **`kahlo_modules_createDraft({source, name?, manifest?})`** — create a mutable draft from JS source.
- **`kahlo_modules_createDraftFromJob({job_id, name?})`** — **save-working-code flow**: take a running or completed `source`-kind job and snapshot its source into a draft.
- **`kahlo_modules_listDrafts`** — metadata for all drafts.
- **`kahlo_modules_getDraft({draft_id})`** — full source + metadata.
- **`kahlo_modules_updateDraft({draft_id, source})`** — replace source in place.
- **`kahlo_modules_promoteDraft({draft_id, name, version_strategy, notes?})`** — freeze draft into `name@version`. `version_strategy` is `"patch"` | `"minor"` | `"major"` and bumps from the last promoted version.
- **`kahlo_modules_promoteFromJob({job_id, name, version_strategy, notes?})`** — same as promoteDraft but skips the draft step; directly freezes a job's source into a versioned module.

## The Module Contract (writing job code)

### Required format

Jobs are **CommonJS** modules with one required export: `start(params, ctx)`. Optional: `init(ctx)`.

```js
module.exports = {
  // Optional — runs once before start; setup that doesn't depend on params.
  init: function(ctx) {
    ctx.log('info', 'Job initializing');
  },

  // Required — main entry point.
  start: function(params, ctx) {
    // params is the JSON object passed to kahlo_jobs_start
    // Return a value (oneshot job.result) or a Promise (for async)
    ctx.log('info', 'Starting with params', params);
    // ... installation code ...
    return { installed: true };
  }
};
```

**Common format mistakes** (every one of these is a wrong pattern to avoid):

1. **IIFE instead of module.exports** — ❌ `(function(){...})()` — ✅ `module.exports = { start(params, ctx) { ... } }`
2. **Wrong function signature** — ❌ `start(ctx, params)` — ✅ `start(params, ctx)` (params is first)
3. **Wrong emit signature** — ❌ `ctx.emit({kind:'x', payload:{}, level:'info'})` — ✅ `ctx.emit('x', {}, 'info')` (three positional args)
4. **Manual boilerplate instead of stdlib** — ❌ `Java.use('java.lang.Thread').currentThread().getStackTrace()` — ✅ `ctx.stdlib.stack.capture()`
5. **Wrong sign on Java byte arrays** — ❌ `javaBytes[i]` (gives signed −128..127) — ✅ `ctx.stdlib.bytes.fromJavaBytes(javaBytes)` (gives unsigned 0..255)
6. **Unsafe Java.use** — ❌ `Java.use('may.not.Exist')` wrapped in try/catch — ✅ `ctx.stdlib.safe.tryUse('may.not.Exist')` returns `null` if missing
7. **Array args to safe.invoke** — ❌ `ctx.stdlib.safe.invoke(obj, 'method', [a, b])` — ✅ `ctx.stdlib.safe.invoke(obj, 'method', a, b)` (variadic)
8. **String pattern for suffix search** — ❌ `classes.find('Activity')` (string means prefix-only) — ✅ `classes.find(/.*Activity$/)` (use RegExp for anything other than a prefix)

### The `ctx` API (available inside every job)

| Member | Purpose |
|---|---|
| `ctx.job_id` | Unique identifier for this job |
| `ctx.emit(kind, payload, level)` | Emit a telemetry event. `level` is `'debug'`\|`'info'`\|`'warn'`\|`'error'`. |
| `ctx.log(level, message, extra?)` | Shortcut for `ctx.emit('log', {message, extra}, level)`. |
| `ctx.heartbeat()` | Update liveness timestamp. Required every few seconds in daemon jobs. |
| `ctx.sleep(ms)` | Promise-based sleep; timer auto-cleared on cancel. |
| `ctx.setTimeout` / `ctx.setInterval` / `ctx.clearTimeout` / `ctx.clearInterval` | Frida-safe timers, auto-cleared on cancel. |
| `ctx.Java` | Java bridge handle, or `null` in non-Java processes. Prefer this over global `Java`. |
| `ctx.javaAvailable()` | Returns true if the target has a Java runtime. |
| `ctx.requireJava(hint?)` | Returns the Java bridge, or emits an error event and throws. |
| `ctx.getJavaStackTrace(options?)` | Shortcut for `ctx.stdlib.stack.capture()`. |
| `ctx.getJavaStackTraceString(options?)` | Shortcut for `ctx.stdlib.stack.toString()`. |
| `ctx.inspectObject(obj, options?)` | Shortcut for `ctx.stdlib.inspect.toJson()`. |
| `ctx.newArtifactId()` | Generate a fresh artifact id (low-level; prefer `emitArtifact`). |
| `ctx.emitArtifact(opts, bytes)` | Send a binary artifact. `opts = {type, mime?, name?, metadata?}`. `bytes` is `ArrayBuffer`\|`Uint8Array`. Max 10 MB. Returns `{artifact_id, size_bytes}` or `{error}`. |
| `ctx.stdlib.*` | The 9-namespace standard library (see below). |

## `ctx.stdlib.*` — The Standard Library

`ctx.stdlib` is pre-loaded into every job. Canonical reference: the upstream kahlo-mcp repo's `src/backend/jobs/jobScriptStdlib.js`, or the live `kahlo_mcp_about` tool output. Use these instead of writing your own; they handle sign bugs, null safety, overload resolution, and JS/Java type coercion correctly.

### `ctx.stdlib.stack` — Java stack traces

- `capture({skip?, limit?})` → `[{className, methodName, fileName, lineNumber, isNative}]`
- `toString({skip?, limit?, separator?})` → formatted string
- `filter(frames, pattern)` / `findFirst(frames, pattern)` — string prefix or RegExp
- `getCaller()` → best-effort caller frame, skipping internal/runtime noise
- `getException(throwable)` → formatted exception string with cause chain

### `ctx.stdlib.inspect` — Java object introspection

- `className(obj)` / `simpleClassName(obj)` — FQ name / bare name
- `fields(obj, {includeInherited?, includeStatic?}?)` → `[{name, type, declaringClass, modifiers}]` (includes privates)
- `methods(obj, {includeInherited?, includeStatic?}?)` → method list
- `getField(obj, fieldName)` → `{ok, value?, error?}`
- `toJson(obj, {maxDepth?, maxArrayLength?, maxStringLength?}?)` → JSON-safe representation. **Use this for logging Java objects**; don't rely on Java `toString()`.
- `isInstance(obj, className)` — instanceof check
- `superclassChain(obj)` / `interfaces(obj, includeInherited?)`

### `ctx.stdlib.classes` — class discovery and loading

- `find(pattern, {limit?}?)` — string = prefix match, RegExp = full match
- `enumerate(callbackOrOptions?)` — stream via callback or return array
- `load(className)` — safe `Java.use`, returns `null` on failure
- `isLoaded(className)` — fast check
- `instances(className, {limit?}?)` — enumerate live heap instances (**expensive**, budget your use)
- `getClassLoader(className)` — the `ClassLoader` for a loaded class

### `ctx.stdlib.bytes` — binary helpers

- `toHex(data, {uppercase?, separator?}?)` / `fromHex(hex)` — hex ↔ `Uint8Array`
- `toBase64(data)` / `fromBase64(b64)` — base64 ↔ `Uint8Array`
- `fromJavaBytes(javaByteArray)` / `toJavaBytes(uint8Array)` — fixes the Java signed-byte trap
- `equals(a, b)` / `concat(...arrays)` / `slice(data, start, end?)`

### `ctx.stdlib.strings`

- `fromJava(javaString)` / `toJava(jsString)` — null-safe conversion
- `fromUtf8(bytes)` / `toUtf8(str)` — UTF-8 ↔ `Uint8Array`
- `truncate(str, maxLen, ellipsis?)` / `matches(str, pattern)` / `safeToString(obj, maxLength?)`

### `ctx.stdlib.intent` — Android Intents

- `parse(intent)` → `{action, data, type, categories, extras, flags, component}` — **one call gives you everything**
- `getExtras(intent)` → JS object, recurses on nested Bundles
- `create({action?, data?, type?, flags?, packageName?, className?, extras?})`
- `getComponent(intent)` / `isExplicit(intent)`
- `flagsToStrings(flags)` — decode the bitmask to flag names

### `ctx.stdlib.hook` — Java and native hooks (all of these auto-increment `metrics.hooks_installed`)

- `method(className, methodName, {onEnter?, onLeave?})` — hooks the **first overload**; use this when you know there's only one
- `methodWithSignature(className, methodName, paramTypes[], {onEnter?, onLeave?})` — hooks a specific overload; `paramTypes` uses JNI-ish names like `['java.lang.String', 'int']`
- `allOverloads(className, methodName, handler)` — hooks every overload with one handler; use for methods like `getBytes()` that are overloaded with `Charset`, `String`, etc.
- `constructor(className, handler)` — hooks all `$init` overloads
- `onClassLoad(className, callback)` — fires once when the class becomes available; use for classes loaded by custom ClassLoaders (DexClassLoader, plugin frameworks, dynamic feature modules)
- `native(addressOrSymbol, {onEnter?, onLeave?})` — `Interceptor.attach` wrapper
- `replace(classWrapper, methodName, replacement)` — full implementation replacement; returns a restore function (manual restore is optional — Frida cleans it on job cancel anyway)

### `ctx.stdlib.safe` — fail-closed wrappers

- `call(fn)` → `{ok, result?, error?}`
- `java(fn)` → same, but wraps in `Java.performNow`
- `timeout(fn, timeoutMs)` → same, but promise-based with timeout (does NOT cancel underlying work)
- `tryUse(className)` → `Java.use` that returns `null` instead of throwing — **use this whenever the class may not exist** (e.g., optional dependencies, version-gated classes)
- `invoke(obj, methodName, ...args)` — safe method invocation, variadic, returns `null` on failure
- `get(obj, dotPath, defaultValue?)` — safe nested property access (`'a.b.c'`)

### `ctx.stdlib.time`

- `now()` / `nowMs()` / `hrNow()` — ISO / epoch ms / high-res
- `format(ms)` — human duration (`3661000 → '1h 1m 1s'`)
- `stopwatch()` → `{elapsed(), reset()}`
- `sleep(ms)` / `measure(fn)` → `{result, durationMs}`
- `debounce(fn, delayMs)` / `throttle(fn, intervalMs)`

## Ready-to-Use Recipes

These are the bread-and-butter. All examples use `kind: "source"` inline JS — promote to a draft/module once validated.

### Recipe A: Log every Activity created (quick sanity hook)

```js
module.exports = {
  start: function (params, ctx) {
    var Java = ctx.requireJava('Activity logger');
    Java.perform(function () {
      ctx.stdlib.hook.method('android.app.Activity', 'onCreate', {
        onEnter: function (args) {
          var cls = ctx.stdlib.inspect.simpleClassName(this);
          ctx.emit('activity.onCreate', { activity: cls }, 'info');
        }
      });
    });
    ctx.log('info', 'Activity hook installed');
    return { installed: true };
  }
};
```

Run as `type: "daemon"`, poll with `kahlo_events_fetch({target_id, filters: {kind: 'activity.onCreate'}})`.

### Recipe B: TLS unpinning (works for OkHttp CertificatePinner, X509TrustManager)

```js
module.exports = {
  start: function (params, ctx) {
    var Java = ctx.requireJava('TLS unpin');
    Java.perform(function () {
      // 1. Neutralize OkHttp CertificatePinner.check
      var pinner = ctx.stdlib.safe.tryUse('okhttp3.CertificatePinner');
      if (pinner) {
        ctx.stdlib.hook.allOverloads('okhttp3.CertificatePinner', 'check', function (args, original) {
          ctx.emit('tls.pinner.bypassed', { hostname: args[0] }, 'info');
          return; // do nothing — pin check passes
        });
      }

      // 2. Neutralize X509TrustManager.checkServerTrusted on any custom TM
      var X509TM = ctx.stdlib.safe.tryUse('javax.net.ssl.X509TrustManager');
      if (X509TM) {
        ctx.stdlib.hook.allOverloads('javax.net.ssl.X509TrustManager', 'checkServerTrusted', function (args, original) {
          ctx.emit('tls.trustmanager.bypassed', {}, 'info');
          return;
        });
      }

      // 3. Platform-level SSLContext.init neutralization (optional, breaks pinning for all engines)
      var SSLContext = ctx.stdlib.safe.tryUse('javax.net.ssl.SSLContext');
      if (SSLContext) {
        ctx.stdlib.hook.method('javax.net.ssl.SSLContext', 'init', {
          onEnter: function (args) {
            ctx.emit('tls.sslcontext.init', {}, 'debug');
          }
        });
      }

      ctx.log('info', 'TLS unpinning installed');
    });
    return { installed: true };
  }
};
```

Run as `daemon`, keep running while you drive the app. Combine with mitmproxy/burp CA on the device for full MitM.

### Recipe C: Dump every HTTPS URL the app hits (OkHttp)

```js
module.exports = {
  start: function (params, ctx) {
    ctx.requireJava('URL dumper');
    Java.perform(function () {
      var Builder = ctx.stdlib.safe.tryUse('okhttp3.Request$Builder');
      if (!Builder) { ctx.log('warn', 'OkHttp not loaded'); return; }
      ctx.stdlib.hook.method('okhttp3.Request$Builder', 'build', {
        onLeave: function (retval) {
          var url = ctx.stdlib.safe.invoke(retval, 'url');
          var method = ctx.stdlib.safe.invoke(retval, 'method');
          ctx.emit('http.request', {
            url: ctx.stdlib.strings.safeToString(url),
            method: ctx.stdlib.strings.safeToString(method)
          }, 'info');
        }
      });
    });
    return { installed: true };
  }
};
```

### Recipe D: Extract AES keys at runtime (SecretKeySpec)

```js
module.exports = {
  start: function (params, ctx) {
    ctx.requireJava('Key extractor');
    Java.perform(function () {
      ctx.stdlib.hook.constructor('javax.crypto.spec.SecretKeySpec', function (args) {
        // args[0] = byte[] key, args[1] = algorithm string
        var keyBytes = ctx.stdlib.bytes.fromJavaBytes(args[0]);
        var hex = ctx.stdlib.bytes.toHex(keyBytes);
        var algo = ctx.stdlib.strings.fromJava(args[1]);
        var stack = ctx.stdlib.stack.toString({ limit: 8 });
        ctx.emit('crypto.key', { algo: algo, hex: hex, length: keyBytes.length, stack: stack }, 'info');
      });
    });
    return { installed: true };
  }
};
```

Similarly hook `Cipher.init(int, Key, ...)` to capture IVs.

### Recipe E: Intent Spy — log every Intent going through startActivity/sendBroadcast

```js
module.exports = {
  start: function (params, ctx) {
    ctx.requireJava('Intent spy');
    Java.perform(function () {
      ctx.stdlib.hook.method('android.content.Context', 'startActivity', {
        onEnter: function (args) {
          ctx.emit('intent.startActivity', ctx.stdlib.intent.parse(args[0]), 'info');
        }
      });
      ctx.stdlib.hook.method('android.content.Context', 'sendBroadcast', {
        onEnter: function (args) {
          ctx.emit('intent.sendBroadcast', ctx.stdlib.intent.parse(args[0]), 'info');
        }
      });
      ctx.stdlib.hook.method('android.content.Context', 'startService', {
        onEnter: function (args) {
          ctx.emit('intent.startService', ctx.stdlib.intent.parse(args[0]), 'info');
        }
      });
    });
    return { installed: true };
  }
};
```

### Recipe F: Dump a Java heap region as an artifact

```js
module.exports = {
  start: function (params, ctx) {
    ctx.requireJava('Memory dump');
    var className = params.className || 'java.lang.String';
    Java.perform(function () {
      var found = ctx.stdlib.classes.instances(className, { limit: 20 });
      var payload = [];
      for (var i = 0; i < found.length; i++) {
        payload.push(ctx.stdlib.inspect.toJson(found[i], { maxDepth: 2, maxStringLength: 256 }));
      }
      var json = JSON.stringify(payload, null, 2);
      var bytes = ctx.stdlib.strings.toUtf8(json);
      ctx.emitArtifact({ type: 'custom', mime: 'application/json', name: className + '-instances.json' }, bytes);
    });
    return { count: 'dumped' };
  }
};
```

### Recipe G: Bootstrap for `gating="spawn"` — hook Application.onCreate

```js
// Pass as `bootstrap: { kind: "source", source: "<this>" }` to kahlo_targets_ensure
module.exports = {
  start: function (params, ctx) {
    ctx.requireJava('Early bootstrap');
    Java.perform(function () {
      ctx.stdlib.hook.method('android.app.Application', 'onCreate', {
        onEnter: function () {
          ctx.emit('app.onCreate', {}, 'info');
          // Install further hooks here — they will be in place before the app runs any user code
        }
      });
    });
    // When this oneshot start() returns, the host auto-resumes the suspended app.
    return { ready: true };
  }
};
```

After the target is ensured with this bootstrap, launch additional jobs with `kahlo_jobs_start` for the main instrumentation work.

### Recipe H: Native hook — Interceptor.attach on `open()`

```js
module.exports = {
  start: function (params, ctx) {
    var openPtr = Module.findExportByName('libc.so', 'open');
    if (!openPtr) {
      ctx.log('error', 'libc!open not found');
      return { installed: false };
    }
    ctx.stdlib.hook.native(openPtr, {
      onEnter: function (args) {
        this.path = args[0].readUtf8String();
        this.flags = args[1].toInt32();
      },
      onLeave: function (retval) {
        ctx.emit('syscall.open', { path: this.path, flags: this.flags, fd: retval.toInt32() }, 'debug');
      }
    });
    return { installed: true };
  }
};
```

## Canonical Workflows

### Workflow 1 — "Running app, I want to observe it" (attach)

```
kahlo_devices_list                                     # pick device_id
kahlo_devices_health(device_id)                        # verify ADB + frida-server
kahlo_processes_list(device_id)                        # find the exact process name
kahlo_targets_ensure(device_id, package=<name>, mode="attach")
kahlo_jobs_start(target_id, type="daemon", module={kind:"source", source:"..."})
kahlo_events_fetch(target_id, filters={kind:"http.request"})   # poll with cursor
# When done:
kahlo_jobs_cancel(job_id)
kahlo_targets_detach(target_id)
```

### Workflow 2 — "I need to hook earliest init" (spawn with bootstrap)

```
kahlo_devices_list
kahlo_targets_ensure(
  device_id,
  package="com.example.app",               # package id, not process name
  mode="spawn",
  gating="spawn",
  bootstrap={kind:"source", source:"<Recipe G>"}
)
# App auto-resumes after bootstrap start() returns
kahlo_jobs_start(target_id, module={kind:"source", source:"<main instrumentation>"})
kahlo_events_fetch(target_id, cursor="...")
```

### Workflow 3 — "Iterate on instrumentation until it works, then save"

```
# Iterate with inline source
j1 = kahlo_jobs_start(target_id, module={kind:"source", source:v1})
kahlo_events_fetch(job_id=j1, ...)
kahlo_jobs_cancel(j1)                        # clean slate
j2 = kahlo_jobs_start(target_id, module={kind:"source", source:v2})
# ... keep iterating ...
# Once it works, save it:
draft = kahlo_modules_createDraftFromJob(job_id=jN, name="http.tracer")
kahlo_modules_promoteDraft(draft_id=draft, name="http.tracer", version_strategy="patch")
# Next time:
kahlo_jobs_start(target_id, module={kind:"module_ref", module_ref:"http.tracer@0.0.1"})
```

### Workflow 4 — "Multi-process app" (Android has no gating=child)

```
kahlo_processes_list(device_id)
# Look for e.g. "com.example.app", "com.example.app:background", "com.example.app:service"
t1 = kahlo_targets_ensure(device_id, package="com.example.app", mode="attach")
t2 = kahlo_targets_ensure(device_id, package="com.example.app:background", mode="attach")
# Install instrumentation in each target independently
```

### Workflow 5 — "App crashed, reinject and resume"

```
kahlo_targets_status(target_id)              # confirm state=dead or agent_state=crashed
# re-run ensure with the same parameters
kahlo_targets_ensure(device_id, package=..., mode=...)
# re-start jobs
kahlo_jobs_start(...)
```

### Workflow 6 — "Event flood, my polling is losing data"

```
# Symptom: kahlo_events_fetch returns dropped markers
# Fix 1: reduce emission at source
kahlo_jobs_cancel(job_id); kahlo_jobs_start(..., params={sample_rate:0.1})   # sample inside the job
# Fix 2: tighter filters
kahlo_events_fetch(target_id, filters={kind:"http.request", level:"info"}, limit:500)
# Fix 3: move bulk data to artifacts
# (rewrite the job to call ctx.emitArtifact() for heavy payloads)
```

## Human-in-the-Loop Handoff

CyberStrikeAI's RE workflow is audit-oriented, not autopwn. When Kahlo surfaces something worth a human's eyes (captured crypto key, sensitive URL, unexpected Intent, a suspicious native export), emit a **review block** in the response instead of immediately auto-patching or pivoting:

```
### Runtime finding
Target:   com.example.app on device emulator-5554
Job:      tls-unpin@src (daemon, 14s uptime)
Evidence: kahlo_events_fetch showed 47× tls.pinner.bypassed events in 3s
          followed by http.request to https://api.example.com/auth/token
Artifact: auth-response.bin (1.2 KB, inspect via kahlo_artifacts_get)
Hypothesis: App normally refuses MitM; pin bypass reveals the hidden auth endpoint.
```

Let the operator review artifacts and decide whether to escalate (deeper hooking, combined static analysis via `ghidra-mcp`, etc.). The agent runs breadth; the operator runs depth.

## Operational Guidelines (keep these close)

- **Crashes are normal.** Always design workflows to reattach and resume. `kahlo_targets_status` → `kahlo_targets_ensure` is the happy-path recovery.
- **Poll, don't push.** Prefer `kahlo_events_fetch` with cursor + limit. Don't try to stuff big payloads into tool results.
- **Cancel early, cancel often.** When iterating on a hook, always `kahlo_jobs_cancel` the previous job before starting the next one. This gives you a clean slate — no accumulated hooks from previous iterations.
- **No cleanup code in modules.** Frida auto-removes every `Interceptor.attach`, method replacement, timer, and piece of state when the script unloads on cancel.
- **Artifacts for anything big.** Events should be small and structured. Anything > a few KB should go to `ctx.emitArtifact`.
- **Use `ctx.stdlib.*` instead of boilerplate.** Stacks: `ctx.stdlib.stack.capture()`, not `Thread.currentThread().getStackTrace()`. Bytes: `ctx.stdlib.bytes.fromJavaBytes(...)`, not raw `[]` indexing (the sign bug bites). Classes: `ctx.stdlib.safe.tryUse(...)`, not `try/catch` around `Java.use`.
- **Attach vs spawn semantics**:
  - `mode="attach"` — `package` is a process *name* (from `kahlo_processes_list`)
  - `mode="spawn"` — `package` is a package *id* (Android manifest)
  - These are usually different strings
- **Gating**:
  - `"none"` — fastest, may miss earliest init
  - `"spawn"` — early hooks via bootstrap; bootstrap runs while suspended, app auto-resumes after `start()` returns
  - `"child"` — **DOES NOT WORK ON ANDROID** (zygote forks). Enumerate child process names and attach to each.
- **For root-only paths** via ADB, wrap with `su -c`: `["shell", "su", "-c", "cat /data/data/<pkg>/databases/app.db"]`. Requires Magisk or similar.

## Failure Modes and Recovery

| Failure | Symptom | Fix |
|---|---|---|
| App crash/restart | `state=dead`, no heartbeat, no events | `kahlo_targets_status` → `kahlo_targets_ensure` (same params) → restart jobs |
| Orchestrator agent crash | `agent_state=crashed` | `kahlo_targets_ensure` again (reinjects agent) |
| Job deadloop / event spam | stalled heartbeat, CPU spike, buffer drops | `kahlo_jobs_cancel(job_id)` immediately; reduce emit rate before re-running |
| Event buffer overflow | dropped markers | tighten filters, smaller limits, move bulk → artifacts |
| Disk / permission issues | tool errors on write | check `df` on `<KAHLO_DATA_DIR>`, `ls -la` on the data dir |
| frida-server missing/wrong version | `kahlo_devices_health` says `unavailable`; `kahlo_targets_ensure` hangs or errors | push a matching frida-server (must match npm `frida` version in kahlo-mcp's `package.json`) and run as root |
| `kahlo_processes_list` validation error | Empty process names cause Zod validation failure | Known upstream bug — some system processes have empty names. Use ADB `ps -A | grep <name>` as fallback. |
| `attach` mode "Process not found" | Frida can't enumerate the process despite it running | Use `spawn` mode instead with `gating="spawn"` + bootstrap. Or find PID via `adb shell pidof <package>`. |
| CLI `frida -f` "Failed to spawn: connection closed" | App crashes on Frida spawn injection | Use `kahlo_targets_ensure` with `mode="spawn"` + `gating="spawn"` instead — MCP handles the spawn lifecycle more robustly than CLI. |
| "class not found" in job | hook install silently fails | use `ctx.stdlib.classes.find(pattern)` to verify the class is loaded; for late-loaded classes use `ctx.stdlib.hook.onClassLoad(name, cb)` |

## Operational Lessons — Kotlin Coroutine Apps (Trepet-class targets)

### Calling Kotlin suspend functions from Frida

Kotlin suspend functions take a hidden `Continuation` parameter. To call them from Frida:

1. **Create a Continuation**: Use `Java.registerClass` implementing `kotlin.coroutines.Continuation` with `getContext()` returning `Dispatchers.IO` and `resumeWith()` logging the result.
2. **Use the app's own continuation classes**: Find `*$ensureConnected$1` or similar continuation classes on the heap and instantiate them with the proper parent continuation.
3. **Return COROUTINE_SUSPENDED from hooks**: When hooking a suspend function and wanting to prevent its continuation chain from executing, return `IntrinsicsKt.getCOROUTINE_SUSPENDED()`. This tells the coroutine machinery "still running, don't resume."

```javascript
// Example: calling a suspend function
var Continuation = Java.use('kotlin.coroutines.Continuation');
var cont = Java.registerClass({
    name: 'com.hook.MyCont',
    implements: [Continuation],
    methods: {
        getContext: function() { return Java.use('kotlinx.coroutines.Dispatchers').getIO(); },
        resumeWith: function(r) { console.log('Result:', r); }
    }
}).$new();

// For methods needing ContinuationImpl (abstract class), use the app's own continuation:
var EnsureConnected1 = Java.use('com.example.SomeClass$methodName$1');
var ec1 = EnsureConnected1.$new(targetInstance, cont);
targetInstance.methodName(ec1);
```

### Bypassing Compose activation/login screens

Compose apps use `SnapshotMutableStateImpl` for UI state. Key patterns:

- **Don't block all setValue()**: Blocking `SnapshotMutableStateImpl.setValue` globally freezes the entire Compose UI. Only intercept specific values.
- **COROUTINE_SUSPENDED trick**: Hook the login handler (`handleLoginAttempt`), force success states, call `onLaunch()`, then return `COROUTINE_SUSPENDED` to prevent the continuation from resetting state to UNKNOWN.
- **Anti-kill hooks**: Block `System.exit`, `Process.killProcess`, `Runtime.exit`, `Activity.finish` to prevent activation timer self-termination.
- **Mode switching**: `LastModeLoaded` enum vs String — some MutableStates hold the enum, others hold the String representation. Check `.getClass().getName()` before setting.

### Accessing obfuscated fields

Frida's `instance.fieldName` syntax often fails with obfuscated Kotlin code. Use Java reflection:
```javascript
var field = instance.getClass().getDeclaredField('a');
field.setAccessible(true);
var value = field.get(instance);
field.set(instance, newValue);
```

### StateFlow manipulation

```javascript
var StateFlowImpl = Java.use('kotlinx.coroutines.flow.StateFlowImpl');
var flow = Java.cast(fieldRef, StateFlowImpl);
flow.getValue();     // read
flow.setValue(val);   // write — triggers recomposition
```

## Further reading

- [docs/mcp-setup.md](../../docs/mcp-setup.md) — full install / enablement / troubleshoot guide for all bundled external MCPs.
- Upstream repo: [FuzzySecurity/kahlo-mcp](https://github.com/FuzzySecurity/kahlo-mcp) — canonical source, README with tool reference, and stdlib implementation.
- Upstream blog post: [Scalable research tooling for agent systems](https://knifecoat.com/Posts/Scalable+research+tooling+for+agent+systems) — design rationale.
- [frida.re/docs/](https://frida.re/docs/) — upstream Frida docs for JavaScript API (`Java.*`, `Interceptor.*`, `Module.*`, `Memory.*`, `NativePointer`).
