---
name: ghidra-mcp
description: Headless Ghidra reverse engineering via MCP — decompile, patch, rename, retype, xref, search, and script real binaries without the GUI. TRIGGER when the user asks to reverse-engineer, decompile, analyze, patch, or understand native binaries (ELF, PE, Mach-O, raw firmware, DLL, .so, kernel modules, drivers, embedded blobs, DJI IMAH, Android native libs), or mentions Ghidra/pyghidra/decompiler tasks. Covers the 212 tools across 34 feature groups exposed by the `ghidra-headless-mcp` external MCP server, including scripting escape hatches (ghidra.eval, ghidra.call, ghidra.script).
version: 1.0.0
---

# Ghidra Headless MCP — Operational Reference

This skill documents the `ghidra-headless-mcp` external MCP server integrated with CyberStrikeAI. When the server is registered and started (see **Required tools** below), the agent has access to **212 tools across 34 feature groups** exposed over MCP stdio. Tools are reachable in-agent as `mcp__ghidra_headless_mcp__<tool.name>` (dots in tool names stay as dots).

## Required tools (verify before use)

Before acting on anything in this skill, the agent MUST verify each item is present AND functional. Report missing tools to the operator with the exact remediation command rather than proceeding blind — a failed tool call ten minutes into an analysis costs more than a 30-second precheck.

- **`ghidra-headless-mcp` MCP server is connected.**
  - Call `mcp__ghidra_headless_mcp__health.ping` — expect `{status: "ok", message: "pong"}`.
  - If the tool is not in the available set at all, the server isn't registered. Remediation: add / enable the `ghidra-headless-mcp` entry under `external_mcp.servers` in the CyberStrikeAI `config.yaml` (template block lives in `config.example.yaml`; see [docs/mcp-setup.md](../../docs/mcp-setup.md)).
  - If the tool is listed but `health.ping` fails, the server is registered but the subprocess is not healthy. Run `scripts/ghidra/start-ghidra-mcp.sh --install` from the CyberStrikeAI repo root to re-verify / repair the install, then restart CyberStrikeAI.

- **Ghidra install directory is resolvable.**
  - `mcp__ghidra_headless_mcp__ghidra.info` returns the install dir the server is using. If empty / `null`, set `GHIDRA_INSTALL_DIR` in the `env:` block of the `ghidra-headless-mcp` entry in `config.yaml` and restart.

- **Disk space.** Ghidra projects are ~100MB per imported binary. Before opening a large firmware image, check free space — `df -h` via an available shell tool is enough.

If any precondition fails, stop and report to the operator. Do not try workarounds that would silently degrade analysis quality (no `objdump` / `r2` fallback unless the user explicitly approves — Ghidra's typed decompiler output is not interchangeable with those).

## When To Use This Skill

Invoke this reference whenever any of the following is true:

- The operator hands the agent a native binary and asks for analysis (`ELF`, `PE`, `Mach-O`, `.so`, `.dll`, `.sys`, `.ko`, firmware blobs, bootloaders, kernel modules, DJI IMAH images, Android native libs).
- The request mentions **decompile**, **disassemble**, **patch**, **rename**, **retype**, **xref**, **find callers/callees**, **trace a function**, **define a struct**, **read/write memory**, **apply a type**, **search for strings/bytes/constants/instructions**, or **extract a CFG**.
- Ghidra, pyghidra, Sleigh, p-code, decompiler writeback, or Clang markup come up by name.
- A larger workflow (APK analysis, firmware RE, malware triage, CVE root-causing, DJI reverse engineering, C2 extraction) needs a native-binary deep dive.

Prefer MCP tools over shell invocations (`objdump`, `r2`, `radare2`, `ghidra_run` scripts) because: (1) the server keeps programs analyzed in memory across calls, (2) `structuredContent` gives you machine-readable JSON with no parser work, (3) transactions give you safe undo, and (4) `decomp.*` is the same Ghidra decompiler with high-level type propagation, not a partial rewrite.

Fall back to shell tools only when you need something outside Ghidra's scope (stripping, packing, running the binary, symbolic execution, fuzzing).

## Server Configuration

The server runs under CyberStrikeAI's external MCP federation. Its lifecycle is managed by `scripts/ghidra/start-ghidra-mcp.sh` — CyberStrikeAI spawns that script via stdio when the `ghidra-headless-mcp` entry in `config.yaml` has `external_mcp_enable: true`.

Relevant environment variables (set them in the `env:` block of the config entry, not in the shell — `config.yaml` values take precedence):

- `GHIDRA_INSTALL_DIR` — path to the Ghidra installation. Auto-detected if left empty; the launcher searches `/opt/ghidra`, `$HOME/ghidra`, `/usr/share/ghidra`, etc.
- `GHIDRA_MCP_HOME` — path to the `ghidra-headless-mcp` repo checkout. Defaults to `~/ghidra-headless-mcp` (auto-cloned on first `--install`).
- `GHIDRA_MCP_PORT` — TCP port when the launcher is run manually with `--tcp` for debugging. Not used in stdio mode.

Manual install & verification (`scripts/ghidra/start-ghidra-mcp.sh --install` handles this, but useful to know for troubleshooting):

1. Clone `github.com/mrphrazer/ghidra-headless-mcp` to `GHIDRA_MCP_HOME`.
2. Create a Python 3.11+ venv in `$GHIDRA_MCP_HOME/.venv`.
3. `pip install pyghidra` (the server wraps pyghidra).
4. Ensure `GHIDRA_INSTALL_DIR/support/analyzeHeadless` exists and is executable.

For an interactive smoke test without CyberStrikeAI in the picture, run the launcher in TCP mode:

```bash
./scripts/ghidra/start-ghidra-mcp.sh --tcp --port 8765
```

Then point a temporary `external_mcp` entry at `http://127.0.0.1:8765/mcp` with `transport: simple_http`.

## MCP Response Format (important — saves a lot of context)

Every tool call returns both:

- `structuredContent` — the canonical full JSON payload. **Read this programmatically.**
- `content[0].text` — a short human-readable summary. Do NOT re-parse this as JSON; it is deliberately lossy.

This split is intentional: use the summary when streaming progress to the operator, use `structuredContent` when feeding results back into another tool call. Never duplicate both in your reply.

Pagination: list/search tools accept `offset` and `limit` and return `{offset, limit, total, has_more, next_offset?, notice?}`. Start with `limit: 50` for exploration and raise to `500` when you know the upper bound. When `has_more=true`, always follow `next_offset` rather than guessing.

## Core Operational Model

### Sessions, Not Global State

Every analysis begins with opening a program, which creates a **session**. Sessions hold the analyzed program database, the decompiler cache, and the active transaction context. Closing a session releases memory. Many tools accept an optional `session` argument; when omitted, they target the most recently active session.

```
program.open            → opens binary from filesystem path
program.open_bytes      → opens from base64 bytes (useful for in-memory blobs)
project.program.open    → opens a program already inside the current project
project.program.open_existing → opens from a different project by name
program.list_open       → shows currently held sessions
program.close           → releases a session
```

After opening, run `program.summary` (metadata, architecture, entry, image base) and `program.report` (counts + samples of functions, strings, imports, memory blocks) to orient yourself before drilling in.

### Read-Only Is The Default

`program.open` opens in **read-only mode**. Any mutating call (rename, retype, patch, comment, struct edits, analysis toggles) will fail until you switch the session:

```
program.mode.set(mode="read_write")
```

Switch back to `read_only` before closing if you don't want to keep changes in the project. Use `program.mode.get` to check.

### Transactions Wrap Every Mutation

Mutating tools auto-wrap themselves in a minimal transaction if none is active. For multi-step edits you want to commit (or revert) atomically, start an explicit transaction:

```
transaction.begin(name="refactor_auth_flow")
# ... many edits ...
transaction.commit      → makes the whole block undoable as one unit
transaction.revert      → discards the whole block without touching history
transaction.undo        → rewinds the most recently committed transaction
transaction.redo        → replays a previously undone transaction
transaction.status      → shows active transaction + undo/redo depth
```

Rule of thumb: one high-level task (e.g., "fill out this parser struct", "patch all bounds checks in this function") = one transaction. This gives the operator a clean single-step rollback if they reject the result.

### Saving Changes

In-memory edits do not persist to the Ghidra project until you call:

```
program.save           → writes back to the project file the session was opened from
program.save_as        → writes to a new project path (use for branching analysis states)
program.export_binary  → dumps as original-format OR raw bytes to disk
project.export         → exports the whole project to a directory
```

Patching workflows typically: `mode.set read_write` → `transaction.begin` → patch → `transaction.commit` → `program.save` → `program.export_binary` (for the patched binary on disk).

## The 34 Feature Groups (Tool Catalog)

### 1. Core and Infrastructure

#### `health.*` (1 tool)
- `health.ping` — always start here when debugging. Returns `{status: "ok", message: "pong"}`. Use it to confirm the server is alive before blaming your tool call.

#### `mcp.*` (1 tool)
- `mcp.response_format` — returns a static explanation of structuredContent vs content[0].text. Call this once when you forget the convention.

#### `analysis.*` (9 tools) — control Ghidra's auto-analyzer
- `analysis.status` — is auto-analysis running, idle, or cancelled?
- `analysis.update` — kick off analysis in background, returns immediately.
- `analysis.update_and_wait` — kick off and block until done. Use this before decomp workflows when opening a fresh binary.
- `analysis.analyzers.list` / `analysis.analyzers.set` — toggle individual boolean analyzers by name (e.g. `DWARF`, `PDB Universal`, `Function Start Search`, `Decompiler Parameter ID`). Disable noisy analyzers on obfuscated binaries.
- `analysis.options.list` / `analysis.options.get` / `analysis.options.set` — the full typed option surface (every slider/dropdown from the Ghidra GUI).
- `analysis.clear_cache` — drop decompiler cache. Use after manual edits that didn't propagate.

#### `task.*` (4 tools) — asynchronous work tracking
- `task.analysis_update` — start auto-analysis as a tracked background task, returns `task_id`.
- `task.status` / `task.result` / `task.cancel` — poll or abort long work.

Use `task.*` instead of `analysis.update_and_wait` when you need to do other work (search, decompile already-analyzed functions) while a reanalysis pass runs.

### 2. Project, Program, and Transactions

#### `program.*` (12 tools)
See "Sessions" above. Additional useful ones:
- `program.image_base.set` — rebase a program. Needed for relocatable blobs (bootloaders, shellcode, ROMs) loaded with wrong base.
- `program.mode.get` / `program.mode.set` — switch `read_only` ↔ `read_write`.

#### `project.*` (7 tools)
- `project.folders.list` — walk project folders (supports recursive).
- `project.files.list` — list files with content-type + query + pagination.
- `project.file.info` — per-file metadata (size, content type, readOnly, checksums).
- `project.search.programs` — name/path search across the project. Fast way to find already-imported versions.
- `project.program.open` / `project.program.open_existing` — open an already-imported program by project path (much faster than re-importing).
- `project.export` — whole-project export.

#### `transaction.*` (6 tools)
See "Transactions Wrap Every Mutation" above.

### 3. Listing, Memory, Disassembly, Patching

#### `listing.*` (12 tools) — the assembly view, programmatic
- `listing.code_units.list` — paginated code-unit sweep over a range, supports direction (`forward`/`backward`).
- `listing.code_unit.at|before|after|containing` — navigate one code unit at a time.
- `listing.clear(address_range, clear_symbols, clear_comments, clear_references, clear_functions, clear_context)` — selectively wipe listing state. Use to prepare dirty regions before re-disassembly.
- `listing.disassemble.function(function)` — disassemble a whole function body.
- `listing.disassemble.range(start, end)` — disassemble an address range.
- `listing.disassemble.seed(address, follow_flows=True)` — kick off flow-following disassembly from a seed. Best for regions the auto-analyzer missed.
- `listing.data.list|at|create|clear` — defined data items (strings, structs, arrays) in the listing.

#### `memory.*` (5 tools)
- `memory.read(address, length)` — raw bytes (base64-encoded in JSON).
- `memory.write(address, bytes_b64)` — direct bytes patch (no assembly).
- `memory.blocks.list` — every memory block with perms + sizes.
- `memory.block.create|remove` — add or drop blocks (needed for loading firmware overlays, MMIO regions, or stripped section recovery).

#### `patch.*` (3 tools) — the clean-patch surface
- `patch.assemble(address, instruction_text)` — write new instruction bytes from Sleigh assembly text, e.g. `"JMP 0x4012a0"`, `"XOR EAX,EAX"`. **Preferred** over `memory.write` for instruction patches because the assembler validates the result.
- `patch.nop(start, end)` — replace an instruction range with NOPs (processor-specific NOP bytes).
- `patch.branch_invert(address)` — flip `JZ ↔ JNZ`, `JG ↔ JLE`, etc., in place. Fastest way to bypass a check.

#### `context.*` (3 tools) — processor context register (ARM Thumb mode, x86 segments, MIPS ISA_MODE, etc.)
- `context.get(address, register)` — read context.
- `context.set(range, register, value)` — set for a range (e.g., force `TMode=1` to disassemble an ARM region as Thumb).
- `context.ranges(register, value)` — find every range where a context value applies.

### 4. Symbols, Namespaces, Externals, References

#### `symbol.*` (7 tools)
- `symbol.list(filter, offset, limit)` — paginated symbol listing with filters (prefix, namespace, type).
- `symbol.by_name(name)` — exact lookup.
- `symbol.create(address, name, namespace?)` — add a label.
- `symbol.rename(address, old_name, new_name)` — rename.
- `symbol.delete(address, name?)` — remove.
- `symbol.primary.set(address, name)` — choose which symbol wins when multiple exist at one address.
- `symbol.namespace.move(address, name, target_namespace)` — reparent.

#### `namespace.create` / `class.create`
- `namespace.create(name, parent?)` — create a plain namespace.
- `class.create(name, parent?)` — create a class namespace (holds vtables, methods).

#### `external.*` (11 tools) — imports, exports, libraries
- `external.imports.list` / `external.exports.list` — the public DLL/SO interface.
- `external.library.list|create|set_path` — register DLLs/SOs with filesystem paths so Ghidra can resolve them.
- `external.location.get|create` — specific symbols inside an external library.
- `external.function.create` — create an external function symbol (type-it like an in-program function).
- `external.entrypoint.add|remove|list` — the program's declared external entry points.

Use case: when analyzing a stripped Linux binary with unresolved `puts`, `strcpy`, `malloc`, register `libc.so.6` via `external.library.create`, point it at the real libc file with `external.library.set_path`, then `external.function.create` the individual functions with correct prototypes.

#### `reference.*` (12 tools) — xrefs are central to RE
- `reference.to(address, range?)` — "who references this" (supports range sweeps for bulk xref extraction).
- `reference.from(address, range?)` — "what does this reference".
- `reference.create.memory(from, to, operand_index, type)` — memory xref.
- `reference.create.stack|register|external` — typed reference creators.
- `reference.delete` / `reference.clear_from` / `reference.clear_to` — removal.
- `reference.primary.set` — mark a reference as primary (for tables of pointers).
- `reference.association.set|remove` — attach a symbol to a reference (used for call tables, vtables).

Tip: `reference.to(range=...)` is *the* way to bulk-harvest callers of a region. Combine with `function.at` on each `from_address` to get unique callers.

#### `equate.*` (4 tools) — symbolic constants at operand level
- `equate.create(address, operand_index, name, value)` — attach a name to an operand constant (e.g. rename `0x80000000` to `GENERIC_WRITE` on a WinAPI call).
- `equate.list|delete|clear_range` — management.

### 5. Annotations

#### `comment.*` (4 tools)
- `comment.set(address, type, text)` — types: `PRE`, `POST`, `EOL`, `PLATE`, `REPEATABLE`.
- `comment.get(address, type)` — read one type.
- `comment.get_all(address, include_function_comments=True)` — all types at once.
- `comment.list(range?, type?, query?, offset?, limit?)` — paginated + text search.

#### `bookmark.*` (4 tools)
- `bookmark.add(address, type, category, comment)` — types: `Note`, `Analysis`, `Error`, custom.
- `bookmark.list|remove|clear` — management.

#### `tag.*` (4 tools) — function-level labels
- `tag.add(function, tag_name)` — tag a function (auto-creates the tag if new).
- `tag.list|remove|stats` — query. `tag.stats` shows tag→function counts across the program.

#### `metadata.*` (2 tools) — your own JSON scratch space, scoped to the program
- `metadata.store(key, value_json)` — store arbitrary JSON under a key.
- `metadata.query(key?, prefix?)` — retrieve.

Use for analysis notes, IoC lists, threat model flags that should travel with the program.

#### `source.*` (6 tools) — source file line → address mapping (DWARF/PDB glue)
- `source.file.list|add|remove` — the file-level source records.
- `source.map.list|add|remove` — line-to-address entries.

#### `relocation.*` (2 tools)
- `relocation.list(range?)` — all relocation entries.
- `relocation.add` — useful for reconstructing relocations in stripped kernel modules.

### 6. Functions, Variables, Types, Layouts

#### `function.*` (22 tools) — the densest group
- `function.list(filter?, offset, limit)` — paginated enumeration. Filters by name/namespace/tag.
- `function.at(address)` — function containing an address.
- `function.by_name(name)` — exact name lookup.
- `function.report(function)` — **the single most valuable exploration tool** — returns signature, variables, callers/callees, xrefs, AND the decompilation output in one shot. Use this first when drilling into a function.
- `function.create(address, name?)` — promote code to a function (when analyzer missed it).
- `function.delete(address)` — de-function.
- `function.rename(address, new_name)` — rename (propagates to decomp).
- `function.body.set(address, new_body_range)` — manual body adjustment.
- `function.signature.get|set` — full C-style signature read/apply. `function.signature.set` takes one C string like `"int parse_packet(uint8_t *buf, size_t len, packet_hdr *out)"`.
- `function.return_type.set` — just the return type.
- `function.calling_conventions.list|set` — per-arch calling conventions (`__stdcall`, `__cdecl`, `__fastcall`, `__thiscall`, custom).
- `function.flags.set` — varargs, inline, noreturn, custom-storage flags.
- `function.thunk.set` — mark as thunk to another function (used for GOT/PLT stubs).
- `function.variables(address)` — all parameters and locals.
- `function.callers(address)` / `function.callees(address)` — one-hop call graph edges.
- `function.batch.run(filter, action, params)` — run one supported action across a filtered set of functions. Use for bulk renames, bulk tag applications, bulk signature applications. Huge time-saver.

#### `variable.*` (5 tools) — local variables and parameters
- `variable.rename(function, old_name, new_name)` — decompiler-backed rename when a high symbol exists.
- `variable.retype(function, name, new_type)` — decompiler-backed retype.
- `variable.local.create|remove` — explicit local variable management.
- `variable.comment.set` — per-variable comment.

#### `parameter.*` (4 tools)
- `parameter.add(function, type, name?, ordinal?, storage?)` — new param.
- `parameter.remove` / `parameter.move(function, from, to)` / `parameter.replace(function, ordinal_or_name, new_definition)` — structural edits.

#### `stackframe.*` (3 tools) — stack frame vars (pre-decomp level)
- `stackframe.variable.create(function, stack_offset, type, name?)`
- `stackframe.variable.clear(function, stack_offset)`
- `stackframe.variables(function)` — enumerate.

#### `type.*` (12 tools) — data type management (Ghidra's type system)
- `type.list(category?, filter?, offset, limit)` — paginated.
- `type.get(name_or_path)` / `type.get_by_id(id)` — lookups.
- `type.define_c(c_declaration, category?)` — **the main way** to add types. Accepts any valid C declaration: `struct foo { int a; char b[16]; };`, `typedef uint32_t handle_t;`, enum, union, function signature.
- `type.parse_c(c_declaration)` — parse without committing (validation-only).
- `type.rename|delete|apply_at(address, type_name)` — applies a defined type to a memory location (creates defined data).
- `type.category.list|create` — organize types by category paths (e.g. `/net/protocol/`).
- `type.archives.list` / `type.source_archives.list` — which type libraries are loaded (`generic_clib.gdt`, `windows_vs12_64.gdt`, etc.).

#### `layout.*` (17 tools) — struct/union/enum editing
Struct:
- `layout.struct.create(name, size?, category?)` — empty struct.
- `layout.struct.get(name)` — current definition + components.
- `layout.struct.resize(name, new_size)` — expand/shrink.
- `layout.struct.field.add(name, offset?, type, field_name?, comment?)` — add at specific offset or append.
- `layout.struct.field.rename|replace|clear|comment.set` — field-level edits.
- `layout.struct.bitfield.add(name, byte_offset, bit_offset, bit_size, type, field_name)` — bitfields.
- **`layout.struct.fill_from_decompiler(variable_ref)`** — the killer feature: takes a decompiler variable that is accessed as a struct and auto-fills fields by observed usage patterns. Start here when you see `arg1->field_0x8 = foo` in decomp and want a real struct.

Union:
- `layout.union.create(name, category?)` — empty union.
- `layout.union.member.add|remove` — members.

Enum:
- `layout.enum.create(name, size, category?)` — create (size = 1/2/4/8 bytes).
- `layout.enum.member.add(name, member_name, value)` / `layout.enum.member.remove`.

Inspection:
- `layout.inspect.components(type_name)` — full component breakdown including nested types.

### 7. Decompiler and P-code (the high-level abstraction layer)

#### `decomp.*` (12 tools) — the decompiler is your best friend
- `decomp.function(function)` — return recovered C source code as text. This is what you read first.
- `decomp.tokens(function)` — tokenized Clang markup (each token has type, color, link target — use for structured programmatic analysis).
- `decomp.ast(function)` — full Clang markup tree.
- `decomp.high_function.summary(function)` — high-function view: locals, globals, basic blocks, jump tables, PcodeOp stats. Use before writeback.
- **`decomp.writeback.params(function)`** — commit decompiler-recovered parameter names/types back into the program database.
- **`decomp.writeback.locals(function)`** — same for locals. Run this after rename-heavy work so edits persist to database, not just the decomp cache.
- `decomp.override.get|set(callsite_address)` — override the decompiler's inferred call signature at a specific callsite (useful for indirect calls through function pointers where the type isn't propagated).
- `decomp.trace_type.forward(symbol, function)` / `decomp.trace_type.backward(symbol, function)` — trace type propagation through data flow. When you don't know where a type came from, trace backward. When you want to know where a type goes next, trace forward.
- `decomp.global.rename(symbol, new_name)` / `decomp.global.retype(symbol, new_type)` — rename/retype globals via high-symbol path. Preferred over plain `symbol.rename` because it uses decomp-aware identity.

`variable.rename` and `variable.retype` automatically use decomp writeback when a high symbol is available, so you don't need to call writeback explicitly for single edits — only for bulk operations.

#### `pcode.*` (4 tools) — the low-level IR
- `pcode.function(function)` — per-instruction p-code for the whole function.
- `pcode.block(address)` — p-code for the basic block containing an address.
- `pcode.op.at(address)` — p-code ops for one instruction.
- `pcode.varnode_uses(function, varnode)` — p-code reads and writes that match a varnode (Ghidra's SSA-level variable).

Use p-code when the decompilation output is too high-level to answer a specific question ("does this instruction read EAX before writing it?", "what registers does this basic block clobber?"). It is also the only layer where you can match semantically on operations regardless of encoding — `search.pcode` uses this.

### 8. Search, Graph, Bulk Query

#### `search.*` (7 tools)
- `search.resolve(expression)` — turn `"0x401000 + 0x24"`, `"main+0x10"`, `"module.dll:CreateFileA"` into an absolute address.
- `search.text(query, encoding?, defined_strings_only?, start?, end?)` — text in defined strings **and** raw memory. Encodings: `ascii`, `utf8`, `utf16le`, `utf16be`.
- `search.defined_strings(filter?, offset, limit)` — just the string table.
- `search.bytes(hex_pattern, start?, end?)` — exact byte pattern search. Wildcards supported as `"48 8b ?? 24"` style masks.
- `search.constants(value, start?, end?)` — find scalar constant operands in instructions.
- `search.instructions(mnemonic_or_pattern, start?, end?)` — mnemonic or rendered-instruction pattern. Much faster than grepping listing output.
- `search.pcode(op_mnemonic_or_pattern, start?, end?)` — p-code op search (encoding-agnostic — finds `CALL`, `BRANCH`, `LOAD` regardless of which x86/ARM/MIPS instruction emitted them).

**Rule of thumb**: always scope searches with `start`/`end` when you know the region. Full-program searches on large firmware can take seconds.

#### `graph.*` (3 tools)
- `graph.basic_blocks(function)` — list basic blocks.
- `graph.cfg.edges(function)` — control-flow edges.
- `graph.call_paths(from_function, to_function, max_depth=5)` — find call graph paths. Use to answer "is `main` ever reachable from this handler?" or "what's the shortest path from `recv()` to `system()`?".

### 9. The Escape Hatches — `ghidra.*`

When the explicit tool catalog does not cover something, drop into scripting:

- `ghidra.info` — runtime info about Ghidra, PyGhidra, versions, install dir, loaded modules.
- `ghidra.call(method_path, args_json)` — invoke a Ghidra/Java API directly. E.g. `ghidra.call("currentProgram.getFunctionManager().getFunctionCount", [])`. Use when the MCP surface does not expose a specific Java method.
- `ghidra.eval(python_code)` — evaluate arbitrary Python against the live PyGhidra runtime. `currentProgram`, `currentAddress`, `currentSelection`, `currentHighlight` are in scope. Anything `jython`-ish from Ghidra GUI scripts works here.
- `ghidra.script(script_path_or_text, args?)` — run a full Ghidra script (Python/Java). Use for long-running or reusable logic.

**Warning**: `ghidra.eval` and `ghidra.call` run arbitrary code in the Ghidra JVM context. They can also write the program database — always start with `mode.set read_write` and an active transaction if you plan to mutate. For read-only investigation, stay with explicit tools.

## Common Workflows

### Workflow A: Fresh Binary Triage

```
health.ping                                 # sanity
program.open(path="/path/to/binary")       # opens read-only
analysis.update_and_wait                    # complete auto-analysis before touching anything
program.summary                             # arch, entry, image base
program.report                              # counts + samples (strings, imports, functions)
external.imports.list                       # what APIs does it use?
search.defined_strings(limit=200)           # any interesting literals?
function.list(filter="main", limit=10)      # entrypoint candidates
function.report(address=<main>)             # decomp + xrefs + variables in one shot
```

### Workflow B: Drilling Into A Specific Function

```
search.resolve(expression="<name-or-addr>")    # get an address
function.at(address)                            # get the function
function.report(function=<addr>)                # full context (sig, vars, xrefs, decomp)
decomp.function(function)                       # just the decomp
function.callers(address) / function.callees    # call graph edges
```

### Workflow C: Rename + Retype Cleanup

```
program.mode.set(mode="read_write")
transaction.begin(name="rename_parse_packet")
function.rename(address, new_name="parse_packet")
function.signature.set(address, signature="int parse_packet(pkt_t *in, size_t len, hdr_t *out)")
variable.rename(function, old_name="iVar1", new_name="magic")
variable.retype(function, name="magic", new_type="uint32_t")
transaction.commit
program.save
```

### Workflow D: Struct Reconstruction From Decompiler

```
program.mode.set(mode="read_write")
transaction.begin(name="recover_pkt_hdr")
# Pick a variable used as `arg1->field_0x8` etc. in decomp
decomp.function(function)                   # eyeball the usage
layout.struct.create(name="pkt_hdr", size=0, category="/net")
layout.struct.fill_from_decompiler(
    type_name="pkt_hdr",
    function=<addr>,
    variable="arg1",
)
# Review and hand-tune
layout.struct.get(name="pkt_hdr")
layout.struct.field.rename(type="pkt_hdr", field="field_0x8", new_name="magic")
layout.struct.field.replace(type="pkt_hdr", field="field_0x8", new_type="uint32_t")
# Apply the type to the parameter
variable.retype(function, name="arg1", new_type="pkt_hdr *")
decomp.writeback.params(function)
transaction.commit
program.save
```

### Workflow E: Patching A Check

```
program.mode.set(mode="read_write")
transaction.begin(name="bypass_signature_check")
# Option 1: flip the branch
patch.branch_invert(address=<jnz-addr>)
# Option 2: NOP the call
patch.nop(start=<call-addr>, end=<call-end>)
# Option 3: rewrite an instruction
patch.assemble(address=<addr>, instruction_text="XOR EAX,EAX")
# Verify
listing.disassemble.range(start=<addr-8>, end=<addr+16>)
transaction.commit
program.save
program.export_binary(path="/tmp/patched.bin", format="original")
```

### Workflow F: Xref Sweep For Callers Of A Dangerous API

```
search.resolve(expression="strcpy")         # or any imported API
reference.to(address=<strcpy-addr>)         # direct callers
# For each caller:
function.at(caller_addr)                    # enclosing function
function.report(enclosing_function)         # see it in context
# Or bulk:
function.batch.run(
    filter={"calls": "strcpy"},
    action="tag.add",
    params={"tag_name": "unsafe_string"}
)
```

### Workflow G: Bulk Comment Harvest

```
comment.list(type="PLATE", query="TODO", limit=500)  # find all TODO plate comments
comment.list(range={"start": 0x400000, "end": 0x500000}, type="EOL")
```

### Workflow H: CFG + Call-Path Analysis

```
graph.basic_blocks(function=<addr>)
graph.cfg.edges(function=<addr>)
graph.call_paths(
    from_function=<main>,
    to_function=<system>,
    max_depth=8
)
```

### Workflow I: When MCP Doesn't Cover Something → Scripting

```
ghidra.info                                 # check Ghidra version, loaded extensions
ghidra.eval(code="""
from ghidra.program.model.symbol import RefType
count = 0
for ref in currentProgram.getReferenceManager().getReferenceIterator(currentProgram.getMinAddress()):
    if ref.getReferenceType() == RefType.COMPUTED_CALL:
        count += 1
count
""")
ghidra.script(path="/path/to/my_analyzer.py")
```

## Human-in-the-Loop Handoff

CyberStrikeAI's RE workflow is audit-oriented, not autopwn. When the agent has narrowed down a finding worth a human's eyes (a suspicious function, a concrete vulnerability, a patching decision), emit an **IDA-handoff block** in the response instead of deep-auto-editing:

```
### Review target
Binary:      /path/to/binary
Function:    FUN_00401a20   (parse_packet)
File offset: 0x1a20         (RVA from base 0x400000)
Hypothesis:  Missing bounds check on len parameter before memcpy
Ghidra says:
    <decomp snippet from decomp.function>
```

This gives the operator an exact `g <address>` jump in IDA (or Ghidra GUI, or whatever they prefer) to review with human-grade UX. Ghidra does the breadth; the operator does the depth on the flagged items. Do not auto-apply aggressive patches across many functions without explicit operator confirmation — pattern-hunt widely, but keep mutation narrow.

## Operational Tips and Gotchas

- **Always call `analysis.update_and_wait` after `program.open` on a fresh binary** before running any decomp/xref/search. Pre-analysis results will be incomplete and wrong in subtle ways.
- **Large binaries**: set `limit` on all list/search tools and iterate via `next_offset`. Default pagination is 50 and that's usually what you want for interactive exploration.
- **When decomp output looks wrong**: check the calling convention, then run `decomp.writeback.params`, then `analysis.clear_cache`, then `decomp.function` again.
- **When rename doesn't propagate**: you probably ran `symbol.rename` on a local that has a high-symbol. Use `variable.rename` instead, or call `decomp.writeback.locals`.
- **Searching for specific instruction sequences**: `search.pcode` beats `search.instructions` when you care about semantics, not encoding. Example: finding all indirect calls regardless of whether they're `CALL [RAX]`, `CALL [RBX+8]`, `JMP EAX`, etc. — search p-code op `CALLIND`.
- **Byte searches with wildcards**: `search.bytes(pattern="48 89 ?5 ?? ?? ?? ??")` lets you nail `mov [rip+disp32], reg64` shapes.
- **Running multiple programs at once**: `program.list_open` gives all sessions. Tools take optional `session` to disambiguate. Useful for cross-version diffing.
- **Read-only mode is sticky**: if you forgot `mode.set read_write` and your first edit fails, subsequent retries often still fail due to cached state. Close and reopen the session if things get weird.
- **Transactions don't persist across sessions**: always `program.save` before `program.close`, or the transaction you committed lives only in the in-memory session.
- **Decomp writeback is required for persistence**: renames in decomp alone (without writeback) will NOT show up in the next session unless you save.
- **P-code varnode notation**: `(register, offset, size)` or `(unique, id, size)`. Use `pcode.op.at` to get the exact varnode strings you need for `pcode.varnode_uses`.
- **Assembler errors**: `patch.assemble` will reject instructions that don't fit the available byte space at the address. For longer replacements, `patch.nop` the tail region first or pick a different encoding.

## Further reading

- [docs/mcp-setup.md](../../docs/mcp-setup.md) — full install / enablement / troubleshoot guide for all bundled external MCPs.
- Upstream repo: [mrphrazer/ghidra-headless-mcp](https://github.com/mrphrazer/ghidra-headless-mcp) — the full tool catalog and the Python/Java backend.
- Ghidra project docs: `$GHIDRA_INSTALL_DIR/docs/` — the underlying API this server wraps.
