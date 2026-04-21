# Ghidra Headless MCP - Interactive Binary Reverse Engineering

## Overview
- **Type**: External MCP server (standalone process, ~212 tools)
- **Binary formats**: ELF, PE, Mach-O, APK/DEX, .NET, firmware, shellcode, raw bytes - anything Ghidra supports
- **Transport**: stdio (auto-started by CyberStrikeAI) or TCP (manual start, port 8765)
- **Backed by**: [ghidra-headless-mcp](https://github.com/mrphrazer/ghidra-headless-mcp) + pyghidra + Ghidra
- **Auto-install**: `scripts/ghidra/start-ghidra-mcp.sh` installs all deps (Ghidra, JDK, pyghidra) if missing

## Architecture
```
LLM (agent)
    | MCP tool call
CyberStrikeAI (Go) - external MCP manager
    | stdio JSON-RPC
ghidra-headless-mcp (Python + JVM via pyghidra)
    | Ghidra Java API (JPype bridge)
Ghidra Analysis Engine
    |
Binary file (sandboxed Ghidra project in /tmp)
```

## Quick Start

### Manual start (TCP mode)
```bash
# First time - installs everything:
scripts/ghidra/start-ghidra-mcp.sh --tcp

# Subsequent runs:
scripts/ghidra/start-ghidra-mcp.sh --tcp --port 8765
```

### Auto-start via CyberStrikeAI
Enable in Settings → Ghidra Headless MCP → Enable checkbox, or set in config.yaml:
```yaml
external_mcp:
  servers:
    ghidra-headless-mcp:
      external_mcp_enable: true
      env:
        GHIDRA_INSTALL_DIR: "/opt/ghidra"
```

## Core Workflow (Essential Tools)

### Step 1: Open and Analyze a Binary
```
program.open  file_path="/path/to/binary"
→ Returns: session_id, program name, architecture, format

analysis.update_and_wait  session_id="..."
→ Runs Ghidra auto-analysis (decompiler, disassembler, type recovery, etc.)
→ Wait for this to complete before querying results!
```

### Step 2: Get Overview
```
program.summary  session_id="..."
→ Architecture, entry point, memory map, section count, function count, import/export counts

program.report  session_id="..."
→ Detailed program report with statistics
```

### Step 3: Explore Functions
```
function.list  session_id="..."
→ All discovered functions with addresses and sizes

function.at  session_id="..."  address="0x401000"
→ Function details at specific address

function.by_name  session_id="..."  name="main"
→ Find function by name
```

### Step 4: Decompile
```
decomp.function  session_id="..."  name="main"
→ Full C pseudocode decompilation

decomp.function  session_id="..."  address="0x401000"
→ Decompile by address

decomp.tokens  session_id="..."  address="0x401000"
→ Decompiler tokens (for precise analysis)
```

### Step 5: Search and Explore
```
search.defined_strings  session_id="..."
→ All defined strings (URLs, keys, passwords, C2 addresses)

external.imports.list  session_id="..."
→ Imported functions (spot crypto, network, process injection APIs)

external.exports.list  session_id="..."
→ Exported symbols

reference.to  session_id="..."  address="0x..."
→ Cross-references TO an address (who calls this?)

reference.from  session_id="..."  address="0x..."
→ Cross-references FROM an address (what does this call?)
```

### Step 6: Advanced Analysis
```
search.bytes  session_id="..."  hex_pattern="48 8b 05"
→ Search for byte patterns

search.text  session_id="..."  text="password"
→ Text search across the binary

graph.call_paths  session_id="..."  source="0x..."  target="0x..."
→ Find call paths between two functions

ghidra.eval  session_id="..."  code="..."
→ Execute arbitrary Ghidra/Python code for custom analysis

ghidra.script  session_id="..."  script_path="/path/to/script.py"
→ Run a Ghidra script file
```

### Step 7: Annotate and Patch
```
function.rename  session_id="..."  address="0x..."  new_name="decrypt_payload"
→ Rename a function

variable.rename  session_id="..."  function_address="0x..."  old_name="local_10"  new_name="encryption_key"
→ Rename a variable

type.define_c  session_id="..."  code="struct C2Config { char server[64]; int port; char key[32]; };"
→ Define custom types

patch.assemble  session_id="..."  address="0x..."  assembly="nop"
→ Patch instructions

patch.nop  session_id="..."  address="0x..."  length=5
→ NOP out instructions
```

### Step 8: Session Management
```
program.list_open
→ List all open sessions

program.close  session_id="..."
→ Close session and free resources

program.save  session_id="..."
→ Save analysis state

program.export_binary  session_id="..."  output_path="/tmp/patched_binary"
→ Export modified binary
```

## APK / DEX Reverse Engineering

Ghidra fully supports Android DEX bytecode:

```
1. program.open  file_path="/tmp/target.apk"
   → Ghidra extracts and analyzes classes.dex

2. analysis.update_and_wait  session_id="..."
   → Auto-analysis discovers Java classes and methods

3. function.list  session_id="..."
   → Lists all Java methods (com.target.app.MainActivity.onCreate, etc.)

4. decomp.function  session_id="..."  name="com.target.crypto.AESHelper.encrypt"
   → Decompiles to Java-like pseudocode showing encryption logic

5. search.defined_strings  session_id="..."
   → Find hardcoded API endpoints, encryption keys, server URLs

6. reference.to  session_id="..."  address="0x..."
   → Trace call chains: who calls the encryption function?

7. symbol.list  session_id="..."
   → All class and method symbols
```

### Key patterns to search in APKs:
- `http://` / `https://` - API endpoints and C2 servers
- `AES`, `DES`, `RSA`, `encrypt`, `decrypt` - crypto operations
- `password`, `token`, `secret`, `key` - credential handling
- `SharedPreferences`, `SQLiteDatabase` - local data storage
- `TrustManager`, `SSLSocketFactory` - certificate pinning
- `Runtime.exec`, `ProcessBuilder` - command execution
- `DexClassLoader`, `PathClassLoader` - dynamic code loading

## Malware Analysis Workflow

```
1. program.open  file_path="/tmp/suspicious_binary"
2. analysis.update_and_wait  session_id="..."
3. program.summary  session_id="..."  → architecture, packer detection
4. external.imports.list  session_id="..."  → suspicious APIs:
   - socket, connect, send, recv → network C2
   - CreateRemoteThread, VirtualAllocEx → process injection
   - CryptEncrypt, BCryptEncrypt → data exfiltration encryption
   - RegSetValueEx → persistence
5. search.defined_strings  session_id="..."  → C2 domains, IPs, mutexes
6. decomp.function  session_id="..."  name="main"  → entry point logic
7. graph.call_paths  session_id="..."  → trace from main to network functions
8. search.bytes  session_id="..."  hex_pattern="..."  → shellcode patterns
```

## Combined with Cuttlefish (Static + Dynamic)

```
# Static analysis with Ghidra
1. program.open  file_path="/tmp/drone_controller.apk"
2. analysis.update_and_wait → deep analysis
3. decomp.function → examine crypto, network, auth functions
4. search.defined_strings → find server URLs, keys

# Dynamic analysis with Cuttlefish
5. cuttlefish_launch → start Android VM
6. cuttlefish_install_apk → install the APK
7. droidrun_state → observe app UI
8. cuttlefish_frida_setup → deploy Frida
9. Hook functions discovered in Ghidra static analysis
10. Cross-reference: Ghidra decompilation + Frida runtime values
```

## All Tool Categories (212 tools)

| Category | Tools | Key Operations |
|----------|-------|----------------|
| Program | 12 | open, close, save, export, list sessions, get/set mode |
| Analysis | 9 | run analysis, status, options, analyzers, clear cache |
| Functions | 19 | list, decompile, rename, create, delete, signature, callers/callees |
| Decompiler | 12 | decompile, AST, tokens, high function, type tracing, writeback |
| Search | 7 | strings, bytes, text, constants, instructions, p-code, resolve |
| References | 12 | xrefs to/from, create/delete refs, clear, associations |
| Symbols | 7 | list, rename, create, delete, by_name, primary, namespace |
| Types | 12 | list, get, define C, parse, apply, rename, delete, categories |
| Memory | 5 | read, write, blocks, create block, remove block |
| Listing | 13 | code units, data, disassemble, clear |
| Patching | 3 | assemble, NOP, branch invert |
| Graph | 3 | basic blocks, CFG edges, call paths |
| P-Code | 4 | function p-code, ops, blocks, varnode uses |
| Layout | 17 | structs, unions, enums, fields, bitfields |
| Variables | 5 | rename, retype, local create/remove, comments |
| Parameters | 4 | add, remove, move, replace |
| External | 11 | imports, exports, libraries, locations, entrypoints |
| Comments | 4 | get, set, list, get_all |
| Bookmarks | 4 | add, list, remove, clear |
| Tags | 4 | add, list, remove, stats |
| Transactions | 6 | begin, commit, revert, undo, redo, status |
| Tasks | 4 | async analysis, status, result, cancel |
| Project | 7 | export, folders, files, info, open, search |
| Scripting | 4 | ghidra.eval, ghidra.call, ghidra.script, ghidra.info |
| Metadata | 2 | store, query |
| Source | 6 | file list/add/remove, map list/add/remove |
| Stack | 3 | variable create/clear, variables |
| Relocations | 2 | list, add |
| Context | 3 | get, set, ranges |

## Configuration

In `config.yaml` under `external_mcp.servers`:
```yaml
ghidra-headless-mcp:
  transport: stdio                    # stdio (auto-start) or tcp (manual)
  command: bash
  args: ["scripts/ghidra/start-ghidra-mcp.sh"]
  env:
    GHIDRA_INSTALL_DIR: "/opt/ghidra" # Path to Ghidra (auto-detected if empty)
    GHIDRA_MCP_HOME: ""               # ghidra-headless-mcp repo path (default ~/ghidra-headless-mcp)
  timeout: 600                        # 10 min timeout for heavy analysis
  external_mcp_enable: true           # Set true to enable
```

## Requirements
- **Ghidra**: 11.x+ (auto-installed by launcher script)
- **JDK**: 17+ (auto-installed)
- **Python**: 3.11+
- **pyghidra**: 3.0.2+ (auto-installed)
- **ghidra-headless-mcp**: Cloned from GitHub (auto-cloned)
- **RAM**: 4GB+ recommended for large binaries
- **Disk**: Ghidra installation ~1GB, analysis projects vary
