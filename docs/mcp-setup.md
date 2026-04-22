# External MCP Setup Guide

CyberStrikeAI talks to external MCP (Model Context Protocol) servers through the
`external_mcp.servers` block in `config.yaml`. This is how the agent gets access
to capabilities CyberStrikeAI itself doesn't implement — third-party analysis
tools, specialized scanners, or vendor-built MCP wrappers around existing
tooling.

This guide covers the two MCP servers that ship with CyberStrikeAI as
**first-class integrations** — launcher scripts under `scripts/`, config
templates under `config.example.yaml`, and recommended skill packs under
`skills/`. Every other MCP server works the same way (point at a binary / URL /
stdio command from `config.yaml`), but these two have extra glue so the
out-of-the-box experience is "run one script, flip a boolean, done."

| MCP | Role | Upstream | Launcher | Skill pack |
|---|---|---|---|---|
| **ghidra-headless-mcp** | Static binary RE: decompile, disassemble, xrefs, type recovery, patching, scripting | [`mrphrazer/ghidra-headless-mcp`](https://github.com/mrphrazer/ghidra-headless-mcp) | `scripts/ghidra/start-ghidra-mcp.sh` | `skills/ghidra-mcp/` |
| **frida-kahlo** | Android dynamic instrumentation: attach/spawn, script injection, telemetry streaming, stdlib for Java hooks | [`FuzzySecurity/kahlo-mcp`](https://github.com/FuzzySecurity/kahlo-mcp) | `scripts/frida/start-frida-kahlo-mcp.sh` | `skills/frida-kahlo-mcp/` |

The two servers are complementary. Ghidra gives the agent **static reasoning**
over any binary Ghidra can open — ELF, PE, Mach-O, APK/DEX, firmware, shellcode.
Frida-Kahlo gives it **live runtime reasoning** on Android — hook a method, dump
a stack trace, parse an `Intent`, observe telemetry from an actual running
process. Neither duplicates the other, and most serious RE sessions benefit from
having both wired up.

---

## Ghidra Headless MCP

### What it does

Exposes ~212 tools covering the full Ghidra programming API: project
management, function listing and decompilation, pcode-level analysis, xref
traversal, struct/enum/union/type recovery, memory reads/writes, symbol table
manipulation, patching (NOP / branch invert / assemble), bookmark management,
and arbitrary `ghidra_call` / `ghidra_script` escape hatches for anything the
typed API doesn't cover.

Everything runs **headless** (no Ghidra GUI). Ghidra projects live on disk as
`.gzf` files, so the agent's analysis state persists across sessions and is
cheap to snapshot or share with a human reviewer in the regular Ghidra GUI.

### Prerequisites

- **Java 17+** (OpenJDK 17 or 21 — the launcher installs one if it's missing on Debian/Ubuntu/Fedora/Arch)
- **Python 3.11+** (also auto-installed on supported distros)
- **Ghidra 11.x** (auto-downloaded if no existing installation is detected under `/opt/ghidra`, `$HOME/ghidra`, or `/usr/share/ghidra`)
- **~2 GB disk** for Ghidra + one project per binary you import
- The `ghidra-headless-mcp` repo (auto-cloned)

### Installation

One command, no environment variables required:

```bash
./scripts/ghidra/start-ghidra-mcp.sh --install
```

The `--install` flag runs every dependency check and exits without starting the
server. On first run you'll see it download and unpack Ghidra, set up a Python
virtual environment, `pip install pyghidra`, clone `ghidra-headless-mcp`, and
verify that `analyzeHeadless` is callable. Subsequent runs are fast — the
script detects the existing install and skips straight to the launch step.

If you already have Ghidra installed somewhere unusual, either set
`GHIDRA_INSTALL_DIR` before running the script or pass `--ghidra-dir`:

```bash
GHIDRA_INSTALL_DIR=/opt/my-ghidra ./scripts/ghidra/start-ghidra-mcp.sh --install
# or
./scripts/ghidra/start-ghidra-mcp.sh --ghidra-dir /opt/my-ghidra --install
```

### Enable in CyberStrikeAI

In `config.yaml`, find `external_mcp.servers.ghidra-headless-mcp` and flip
`external_mcp_enable: false` to `true`. The block is already templated in
`config.example.yaml` so you can also just copy that block into your
`config.yaml` and edit:

```yaml
external_mcp:
  servers:
    ghidra-headless-mcp:
      transport: stdio
      command: bash
      args: ["scripts/ghidra/start-ghidra-mcp.sh"]
      env:
        GHIDRA_INSTALL_DIR: ""  # optional override
        GHIDRA_MCP_HOME: ""     # optional override
      timeout: 600
      external_mcp_enable: true   # ← flip this
      tool_enabled: {}
```

Restart CyberStrikeAI. Verify the server is up from the Web UI:
**Settings → External MCP** should show `ghidra-headless-mcp` in `connected`
state with a tool count in the low 200s.

### Troubleshooting

- **`analyzeHeadless not found` on startup** — Ghidra's download silently
  failed. Delete `$HOME/ghidra_*_PUBLIC` and re-run with `--install`. On
  restricted networks point to a local mirror with
  `GHIDRA_DOWNLOAD_URL=<url>` (see the header of `start-ghidra-mcp.sh` for
  the full env-var list).
- **`Java version too old`** — Ghidra 11.x requires JDK 17+. The script will
  try to install it on Debian/Ubuntu/Fedora/Arch. On other distros, install
  manually and set `JAVA_HOME` before running.
- **Tool count shows 0 after enabling** — the stdio subprocess is alive but
  isn't registering tools. Most common cause: `pyghidra` installed into a
  Python version that doesn't match the one the launcher is using. Run
  `./scripts/ghidra/start-ghidra-mcp.sh --tcp` once interactively to see the
  actual error on stderr.
- **Timeouts during decompilation of large binaries** — bump `timeout: 600`
  in `config.yaml` to `1200` or more. Per-tool timeouts can also be
  overridden from the UI.

### Run interactively for debugging

When you need to see the server's logs directly (not piped through
CyberStrikeAI), start it in TCP mode on another terminal:

```bash
./scripts/ghidra/start-ghidra-mcp.sh --tcp --port 8765
```

and point a temporary `external_mcp` entry at `http://127.0.0.1:8765/mcp` with
`transport: simple_http`. Switch back to stdio mode for normal operation.

---

## Frida Kahlo MCP

### What it does

Wraps Frida's Android dynamic instrumentation capabilities in a structured
tool interface. The server manages the full lifecycle of an instrumented
session:

- **Discovery** — list connected devices, running processes, device health
- **Targeting** — attach to or spawn a package with configurable gating
  (`none` / `spawn` / `child`) and optional bootstrap scripts
- **Execution** — run Frida scripts in per-script isolation with automatic
  cleanup on cancellation
- **Observation** — cursor-paginated event streaming, binary artifact storage
  (dumped classes, captured buffers, intercepted files)
- **Standard library** — prebuilt primitives for Java object inspection, stack
  traces, Intent parsing, and method hooking so the agent doesn't have to
  rewrite the same hook logic on every job

kahlo-mcp's stdlib is the practical bit that matters for agent use — without
it every Frida script would have to reinvent reflection-walking boilerplate.

### Prerequisites

- **Node.js 18+** (the launcher auto-installs via apt/dnf/pacman/brew; on
  Debian/Ubuntu it will pull from NodeSource for a current version)
- **adb** (Android Debug Bridge) — **not** auto-installed because it needs
  platform-specific USB permissions / udev rules. On Debian/Ubuntu:
  `sudo apt-get install android-tools-adb`. On macOS:
  `brew install android-platform-tools`.
- **A rooted Android device or emulator** (required for arbitrary process
  attach; Magisk works, rooted AVDs work, Genymotion works).
- **`frida-server` running on the device** at a version matching the `frida`
  package pinned in `kahlo-mcp/package.json`. Download the matching
  `frida-server-*-android-<arch>` release from
  [Frida releases](https://github.com/frida/frida/releases), push to
  `/data/local/tmp/`, `chmod +x`, and run as root (typically
  `su -c /data/local/tmp/frida-server &`).
- The kahlo-mcp repo (auto-cloned into `~/kahlo-mcp` by the launcher).

### Installation

```bash
./scripts/frida/start-frida-kahlo-mcp.sh --install
```

On first run this installs Node (if missing), clones `FuzzySecurity/kahlo-mcp`
into `~/kahlo-mcp`, runs `npm install` + `npm run build`, auto-detects the
`adb` path via `which adb`, and writes a default `config.json`. Subsequent
runs skip all of that.

If adb is at a non-standard path or you have multiple SDK installs, pass it
explicitly:

```bash
./scripts/frida/start-frida-kahlo-mcp.sh --adb-path /opt/android-sdk/platform-tools/adb --install
```

### Enable in CyberStrikeAI

The template is already in `config.example.yaml`; mirror or copy the block
into your `config.yaml`:

```yaml
external_mcp:
  servers:
    frida-kahlo:
      transport: stdio
      command: bash
      args: ["scripts/frida/start-frida-kahlo-mcp.sh"]
      env:
        KAHLO_MCP_HOME: ""    # optional — default ~/kahlo-mcp
        KAHLO_ADB_PATH: ""    # optional — default: auto-detect
        KAHLO_LOG_LEVEL: "info"
      timeout: 300
      external_mcp_enable: true   # ← flip this
      tool_enabled: {}
```

Restart. **Settings → External MCP** should list `frida-kahlo` as `connected`.
Verify the device is reachable by asking the agent (in any chat using a role
with frida-kahlo access):

> *"List devices visible to kahlo-mcp and run a health check on each."*

The agent will call `kahlo_devices_list` and `kahlo_devices_health` and report
back whether adb is reachable and frida-server is responsive on the device.

### Troubleshooting

- **`adb not found on PATH`** — install from your distro's Android tools
  package (see Prerequisites above). The launcher falls back to
  `/usr/bin/adb` as a placeholder so the server can still start and list its
  tool inventory, but real device interaction will fail until adb is
  installed.
- **Device not listed by `kahlo_devices_list`** — run `adb devices` manually
  first; if empty, USB debugging isn't enabled on the device or
  `~/.android/adbkey.pub` isn't trusted. If the device shows up in `adb
  devices` but not in kahlo, check `$KAHLO_MCP_HOME/config.json` has the
  correct `adbPath`.
- **"Failed to spawn process"** — frida-server isn't running on the device,
  or its version doesn't match the `frida` npm package. Check with
  `adb shell ps -A | grep frida` and make sure the frida-server binary
  matches the device arch (arm64 / armv7 / x86_64).
- **`npm run build` fails during install** — typically a Node < 18 issue.
  The launcher checks and upgrades, but if you have a pinned older Node
  elsewhere on PATH it may win. Set `PATH` explicitly before running the
  launcher.

### Run interactively for debugging

Run the launcher without `--install` to see kahlo-mcp's stderr directly:

```bash
./scripts/frida/start-frida-kahlo-mcp.sh
# Ctrl+C to stop
```

kahlo-mcp doesn't currently have a TCP/SSE mode (stdio only); to talk to it
manually, use an MCP client like `mcp-cli` or `@modelcontextprotocol/inspector`.

---

## Adding your own MCP server

The two integrations above are just convenience wrappers. Any MCP server that
speaks stdio, HTTP, or SSE can be registered in `external_mcp.servers` using
the same shape:

```yaml
external_mcp:
  servers:
    my-server:
      # Pick one transport:
      transport: stdio           # OR "http" OR "sse" OR "simple_http"
      command: /path/to/binary   # for stdio
      args: ["--some-flag"]
      env:
        MY_SECRET: "${MY_SECRET}"  # env-var refs resolved at connect time, not persisted to disk
      # OR for http/sse:
      # url: "https://example.com/mcp"
      # headers:
      #   Authorization: "Bearer ${MY_TOKEN}"
      description: "One-line for the settings UI"
      timeout: 30
      external_mcp_enable: true
      tool_enabled: {}            # or {"tool_x": true, "tool_y": false}
```

`${VAR}` and `${VAR:-default}` syntax in `env`, `command`, `args`, `url`, and
`headers` is resolved lazily at the moment CyberStrikeAI spawns the server, so
secrets can live in the environment instead of in committed config files. See
`internal/config/envexpand.go` for the expansion semantics.

If you're working on an MCP server you'd like bundled with CyberStrikeAI the
way Ghidra and Frida-Kahlo are, the pattern is:

1. Launcher shell script under `scripts/<tool>/start-<tool>-mcp.sh` with
   auto-install logic following the same structure as the existing two.
2. Template block under `external_mcp.servers.<tool>` in `config.example.yaml`
   with `external_mcp_enable: false` by default.
3. Section in this document covering prereqs / install / enable /
   troubleshooting.
4. Optional: a skill pack in `skills/<tool>/SKILL.md` describing how the agent
   should use the tool's inventory.
