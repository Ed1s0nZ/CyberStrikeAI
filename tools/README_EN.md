# Tool Configuration Guide

## Overview

Each tool ships with its own YAML configuration placed in the `tools/` directory. This keeps definitions modular, easier to review, and simple to extend. The runtime automatically loads every `.yaml` / `.yml` file in that directory.

## File Structure

The table below enumerates every supported top-level field. Double-check each entry before adding a new tool:

| Field | Required | Type | Description |
|-------|----------|------|-------------|
| `name` | ✅ | string | Unique identifier. Prefer lowercase letters, digits, and hyphens. |
| `command` | ✅ | string | Executable or script name. Must exist in `$PATH` or be an absolute path. |
| `enabled` | ✅ | bool | Controls MCP registration. Disabled tools are ignored by the loader. |
| `description` | ✅ | string | Full Markdown description for MCP `resources/read` and AI comprehension. |
| `short_description` | Optional | string | 20–50 character summary shown in tool lists. When omitted, the loader extracts the start of `description`. |
| `args` | Optional | string[] | Static arguments prepended to every invocation—useful for default scan profiles. |
| `parameters` | Optional | array | Runtime parameter definitions. See **Parameter Definition** for details. |
| `arg_mapping` | Optional | string | Mapping strategy (`auto`/`manual`/`template`). Defaults to `auto`; override only for legacy tooling. |

> If a required field is missing or malformed, the loader skips that tool and logs a warning without blocking the service.

## Tool Descriptions

### Short Description (`short_description`)

- **Purpose**: compact summary for tool listings and to minimise language model context usage.  
- **Guideline**: one concise sentence (20–50 Chinese characters or English equivalents).  
- **Example**: `"Network scanner for discovering hosts, open ports, and services"`

### Detailed Description (`description`)

Supports multi-line Markdown. Recommended contents:

1. **Capabilities** – what the tool does.  
2. **Usage scenarios** – when to prefer this tool.  
3. **Warnings** – permissions, runtime risks, side-effects.  
4. **Examples** – optional walkthroughs or sample commands.

**Important**:
- Tool menus and MCP summaries use `short_description` when available.  
- Without `short_description`, the loader trims the first line or first 100 characters of `description`.  
- Full descriptions are accessible through the MCP `resources/read` endpoint (`tool://<tool_name>`).

## Parameter Definition

Each parameter object accepts the fields below:

- `name` *(required)* – parameter key used in CLI construction and MCP schema.  
- `type` *(required)* – `string`, `int`/`integer`, `bool`/`boolean`, `array`, etc.  
- `description` *(required)* – Markdown-friendly explanation including purpose, format rules, example values, and safety notes.  
- `required` – boolean; when `true`, missing values cause the executor to return an error.  
- `default` – fallback value applied if the caller omits the argument.  
- `flag` – CLI switch such as `-u` or `--url`.  
- `position` – zero-based index for positional arguments.  
- `format` – rendering strategy:
  - `flag` *(default)* → `--flag value` / `-f value`
  - `combined` → `--flag=value`
  - `positional` → appended according to `position`
  - `template` → uses the `template` string
- `template` – placeholder string (supports `{flag}`, `{value}`, `{name}`) when `format: "template"`.
- `options` – array of allowed values; surfaced as `enum` entries in the MCP schema.

### Format Reference

- **`flag`**: pass the flag and the value separately.  
  Example: `flag: "-u"` → `-u https://example.com`

- **`positional`**: insert according to `position`.  
  Example: `position: 0` → becomes the first positional argument.

- **`combined`**: join flag and value in one token.  
  Example: `flag: "--level"`, `format: "combined"` → `--level=3`

- **`template`**: custom rendering.  
  Example: `template: "{flag} {value}"` → fully manual control.

### Reserved Parameters

- `additional_args` – allows users to append arbitrary CLI fragments. The executor tokenises the string (preserving quoted groups) and appends the resulting list to the command.  
- `scan_type` – for scanners like `nmap`, replacing default scan switches (e.g., `-sV -sC`).  
- `action` – consumed by server-side branching logic and intentionally not forwarded to the command line.

## Parameter Description Checklist

When documenting a parameter, include:

1. **Purpose** – what the value controls.  
2. **Format rules** – accepted patterns (URL, CIDR, path, etc.).  
3. **Example values** – list several realistic samples.  
4. **Notes** – permissions, performance impact, or other caveats.

Suggested style: Markdown lists, bold emphasis for key cautions, and code blocks for complex examples.

### Example

```yaml
description: |
  Target IP address or domain. Accepts single IPs, ranges, CIDR blocks, or hostnames.

  **Example values**
  - Single IP: "192.168.1.1"
  - Range: "192.168.1.1-100"
  - CIDR: "192.168.1.0/24"
  - Domain: "example.com"

  **Notes**
  - Required; cannot be empty.
  - Validate address format before running to avoid false positives.
```

## Parameter Types

### Boolean
- `true` → adds only the flag (no value).  
- `false` → suppresses the flag.  
- Accepts `true`/`false`, `1`/`0`, and `"true"`/`"false"`.

```yaml
- name: "verbose"
  type: "bool"
  description: "Enable verbose output"
  required: false
  default: false
  flag: "-v"
  format: "flag"
```

### String
Most common parameter type; accepts any string value.

### Integer
Use for numeric inputs (ports, levels, limits).

```yaml
- name: "level"
  type: "int"
  description: "Level of detail, 1-5"
  required: false
  default: 3
  flag: "--level"
  format: "combined"  # --level=3
```

### Array
Automatically converted to a comma-separated string.

```yaml
- name: "ports"
  type: "array"
  item_type: "number"
  description: "List of ports to scan"
  required: false
  # Input: [80, 443, 8080]
  # Output: "80,443,8080"
```

## Special Parameters

### `additional_args`

```yaml
- name: "additional_args"
  type: "string"
  description: "Extra CLI arguments; separate multiple options with spaces"
  required: false
  format: "positional"
```

Examples:
- `additional_args: "--script vuln -O"` → `["--script", "vuln", "-O"]`
- `additional_args: "-T4 --max-retries 3"` → `["-T4", "--max-retries", "3"]`

Notes:
- Quoted strings are preserved.  
- Validate user input to avoid command injection.  
- Appended at the end of the final command.

### `scan_type`

```yaml
- name: "scan_type"
  type: "string"
  description: "Overrides default scan switches"
  required: false
  format: "positional"
```

Examples:
- `scan_type: "-sV -sC"`  
- `scan_type: "-A"`

Notes:
- Replaces default entries in the tool’s `args` list.  
- Separate multiple flags with spaces.

## Complete Example (`nmap`)

```yaml
name: "nmap"
command: "nmap"
args: ["-sT", "-sV", "-sC"]
enabled: true

short_description: "Network scanner for discovering hosts, open ports, and services"

description: |
  Network mapping and port scanning utility.

  **Highlights**
  - Host discovery
  - Port scanning
  - Service identification
  - OS fingerprinting
  - NSE-based vulnerability checks

parameters:
  - name: "target"
    type: "string"
    description: "Target IP or domain"
    required: true
    position: 0
    format: "positional"

  - name: "ports"
    type: "string"
    description: "Port range, e.g., 1-1000"
    required: false
    flag: "-p"
    format: "flag"

  - name: "scan_type"
    type: "string"
    description: "Override scan switches, e.g., '-sV -sC'"
    required: false
    format: "positional"

  - name: "additional_args"
    type: "string"
    description: "Extra nmap arguments, e.g., '--script vuln -O'"
    required: false
    format: "positional"
```

## Adding a New Tool

1. Create a YAML file in `tools/` (e.g., `tools/mytool.yaml`).  
2. Fill out the top-level fields and parameter list.  
3. Provide defaults and rich descriptions wherever possible.  
4. Run `go run cmd/test-config/main.go` to validate the configuration.  
5. Restart the service (or trigger a reload) so the UI and MCP registry pick up the change.

### Template

```yaml
name: "tool_name"
command: "command"
enabled: true

short_description: "One-line summary"

description: |
  Detailed description with Markdown formatting.

parameters:
  - name: "target"
    type: "string"
    description: "Explain the expected value, format, examples, and caveats"
    required: true
    position: 0
    format: "positional"

  - name: "option"
    type: "string"
    description: "Optional flag parameter"
    required: false
    flag: "--option"
    format: "flag"

  - name: "verbose"
    type: "bool"
    description: "Enable verbose mode"
    required: false
    default: false
    flag: "-v"
    format: "flag"

  - name: "additional_args"
    type: "string"
    description: "Extra CLI options separated by spaces"
    required: false
    format: "positional"
```

## Validation & Troubleshooting

- ✅ Verify required fields: `name`, `command`, `enabled`, `description`.  
- ✅ Ensure parameter definitions use supported types and formats.  
- ✅ Watch server logs for warnings when a tool fails to load.  
- ✅ Use `go run cmd/test-config/main.go` to inspect parsed tool metadata.

## Best Practices

1. **Parameter design** – expose common flags individually; leverage `additional_args` for advanced scenarios.  
2. **Documentation** – combine `short_description` with thorough `description` to balance brevity and clarity.  
3. **Defaults** – provide sensible `default` values, especially for frequently used options.  
4. **Validation prompts** – describe expected formats and highlight constraints to help the AI and users avoid mistakes.  
5. **Safety** – warn about privileged commands, destructive actions, or high-impact scans.

## Disabling a Tool

Set `enabled: false` or remove/rename the YAML file. Disabled tools disappear from the UI and MCP inventory.

## Related Documents

- Main project README: `../README.md`  
- Tool list samples: `tools/*.yaml`  
- API overview: see the main README

