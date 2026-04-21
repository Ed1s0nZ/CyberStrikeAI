# CyberStrikeAI Plugin System

## Overview

The plugin system allows extending CyberStrikeAI with new tools, skills, agents, roles, and knowledge - **without modifying any Go source code**. Each plugin is a self-contained directory with a manifest, and gets hot-loaded at runtime.

Plugins can provide:
- **Tools** - security tool integrations (nmap wrappers, API clients, custom scanners)
- **Skills** - methodology documents the AI agent loads on demand
- **Agents** - multi-agent sub-agent definitions for the orchestrator
- **Roles** - predefined security testing roles with tool/skill configurations
- **Knowledge** - documents indexed into the RAG knowledge base
- **Scripts** - Python/shell scripts executed by plugin tools
- **MCP Servers** - standalone MCP protocol servers (stdio/SSE)

## Quick Start

### 1. Create a plugin

```bash
mkdir -p plugins/my-plugin/tools plugins/my-plugin/scripts
```

### 2. Write the manifest

`plugins/my-plugin/plugin.yaml`:
```yaml
name: my-plugin
version: "1.0.0"
description: "My custom security tool integration"
author: "your-name"

config:
  - name: MY_API_KEY
    description: "API key for the service"
    required: true
```

### 3. Add a tool

`plugins/my-plugin/tools/my-tool.yaml`:
```yaml
name: "my-tool"
command: "python3"
args: ["{{PLUGIN_DIR}}/scripts/my_tool.py"]
enabled: true
short_description: "One-line description for the AI agent"
description: |
  Detailed description of what this tool does.
  The AI reads this to decide when to use the tool.
parameters:
  - name: "target"
    type: "string"
    description: "Target IP, domain, or URL"
    required: true
    position: 0
    format: "positional"
```

### 4. Write the script

`plugins/my-plugin/scripts/my_tool.py`:
```python
#!/usr/bin/env python3
import os, sys, json

api_key = os.environ.get("MY_API_KEY", "")
if not api_key:
    print(json.dumps({"error": "MY_API_KEY not configured"}))
    sys.exit(1)

target = sys.argv[1] if len(sys.argv) > 1 else ""
# ... your tool logic here ...
print(json.dumps({"status": "ok", "results": [...]}))
```

### 5. Enable via API

```bash
# Configure API key
curl -X POST http://localhost:8080/api/plugins/my-plugin/config \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"MY_API_KEY": "your-key-here"}'

# Enable (hot-loads tools immediately)
curl -X POST http://localhost:8080/api/plugins/my-plugin/enable \
  -H "Authorization: Bearer TOKEN"
```

The tool is now available to the AI agent - no restart needed.

---

## Plugin Directory Structure

```
plugins/
└── my-plugin/
    ├── plugin.yaml          # Manifest (required)
    ├── requirements.txt     # Python dependencies (optional)
    ├── tools/               # Tool YAML definitions
    │   ├── tool-a.yaml
    │   └── tool-b.yaml
    ├── scripts/             # Scripts referenced by tools
    │   ├── tool_a.py
    │   └── lib/
    │       └── helpers.py
    ├── skills/              # Skill methodology documents
    │   └── my-skill/
    │       └── SKILL.md
    ├── agents/              # Multi-agent sub-agent definitions
    │   └── my-agent.md
    ├── roles/               # Role configurations
    │   └── My_Role.yaml
    ├── knowledge/           # Knowledge base documents
    │   └── reference.md
    └── mcp-server/          # MCP server (optional)
        ├── server.py
        └── config.yaml
```

All subdirectories are optional - a plugin can provide any combination.

---

## Manifest Reference (plugin.yaml)

```yaml
# Required fields
name: "plugin-name"              # Unique identifier (lowercase, hyphens)
version: "1.0.0"                 # Semantic version
description: "What this plugin does"

# Optional metadata
author: "author-name"
url: "https://github.com/..."    # Source repository

# What this plugin provides (auto-detected from directories if omitted)
provides:
  tools: true                    # Has tools/ directory
  skills: true                   # Has skills/ directory
  agents: false                  # Has agents/ directory
  roles: false                   # Has roles/ directory
  knowledge: true                # Has knowledge/ directory

# Plugin configuration variables
# These become environment variables available to plugin scripts
config:
  - name: MY_API_KEY             # Env var name (UPPER_SNAKE_CASE)
    description: "Human-readable description"
    required: true               # Must be set before enabling

  - name: MY_OPTION
    description: "Optional setting"
    required: false

# Python dependencies file (relative to plugin dir)
requirements: "requirements.txt"
```

### Auto-detection

If `provides` is omitted, the plugin manager auto-detects by checking for directories:
- `tools/` exists → `provides.tools = true`
- `skills/` exists and contains `*/SKILL.md` → `provides.skills = true`
- `agents/` exists and contains `*.md` → `provides.agents = true`
- `roles/` exists and contains `*.yaml` → `provides.roles = true`
- `knowledge/` exists and contains files → `provides.knowledge = true`

---

## Tool Definitions

Tool YAML files in `plugins/<name>/tools/` follow the same format as `tools/*.yaml` in the main project.

### Special Placeholder: `{{PLUGIN_DIR}}`

Use `{{PLUGIN_DIR}}` in `command` or `args` to reference files within the plugin directory. It gets replaced with the absolute path at load time.

```yaml
name: "my-scanner"
command: "python3"
args: ["{{PLUGIN_DIR}}/scripts/scanner.py"]
# Becomes: args: ["/home/user/CyberStrikeAI/plugins/my-plugin/scripts/scanner.py"]
```

### Environment Variables

All plugin `config` variables are automatically injected as environment variables when the tool runs. You don't need to handle env var passing - the plugin system does it.

```python
# In your script - just read the env var
import os
api_key = os.environ["MY_API_KEY"]  # Set by plugin system
```

### Parameter Types

```yaml
parameters:
  - name: "target"
    type: "string"           # string, int, bool, array
    description: "..."
    required: true
    position: 0              # Positional arg index
    format: "positional"     # positional, flag, or auto

  - name: "verbose"
    type: "bool"
    flag: "-v"               # Passed as -v when true
    required: false

  - name: "ports"
    type: "string"
    flag: "-p"               # Passed as -p <value>
    required: false
```

---

## Skills

Place skill documents in `plugins/<name>/skills/<skill-name>/SKILL.md`. These are automatically available to the AI agent via the `list_skills` / `read_skill` tools.

```markdown
# My Security Testing Methodology

## Overview
Description of when and how to use this skill.

## Techniques
1. Step one
2. Step two

## Tools
- tool-a: for this purpose
- tool-b: for that purpose

## Examples
```
Example commands and expected output
```
```

---

## Agents

Place multi-agent sub-agent definitions in `plugins/<name>/agents/<agent-name>.md`. Format:

```markdown
---
id: my-agent
name: My Specialist Agent
description: Handles specific security testing tasks
tools: []
max_iterations: 0
---

You are a specialist sub-agent for [domain].
Use available tools to complete assigned tasks.
Always respond in English only.
```

---

## Roles

Place role configurations in `plugins/<name>/roles/<Role_Name>.yaml`:

```yaml
name: My Role
description: "Custom role for specific testing scenarios"
user_prompt: "Additional context for the AI when using this role"
icon: "\U0001F50D"
tools:
  - my-tool
  - nmap
  - nuclei
skills:
  - my-skill
enabled: true
```

---

## Knowledge

Place markdown documents in `plugins/<name>/knowledge/`. These are automatically indexed into the RAG knowledge base when the plugin is enabled (requires knowledge base to be enabled in config).

Good for: API references, vulnerability databases, methodology guides, tool documentation.

---

## Python Dependencies

If your plugin uses Python packages, list them in `requirements.txt`:

```
requests>=2.28
beautifulsoup4>=4.11
lxml>=4.9
```

Install via API:
```bash
curl -X POST http://localhost:8080/api/plugins/my-plugin/install \
  -H "Authorization: Bearer TOKEN"
```

Dependencies install into a `.venv` directory inside the plugin folder, keeping the system clean. Scripts should be invoked with the system Python - the venv is activated automatically if present.

---

## API Reference

All endpoints require authentication (`Authorization: Bearer <token>`).

### List Plugins

```
GET /api/plugins
```

Response:
```json
{
  "count": 2,
  "plugins": [
    {
      "manifest": {
        "name": "censys",
        "version": "1.0.0",
        "description": "Censys internet search",
        "config": [
          {"name": "CENSYS_API_ID", "description": "...", "required": true},
          {"name": "CENSYS_API_SECRET", "description": "...", "required": true}
        ]
      },
      "enabled": true,
      "installed": true,
      "config_set": true,
      "tool_count": 1,
      "skill_count": 1,
      "agent_count": 0
    }
  ]
}
```

### Enable Plugin

```
POST /api/plugins/:name/enable
```

Hot-loads plugin tools into the MCP server. The AI agent can use them immediately.

### Disable Plugin

```
POST /api/plugins/:name/disable
```

Removes plugin tools from the MCP server.

### Get Plugin Config

```
GET /api/plugins/:name/config
```

Returns config variables with values masked (`********`).

### Set Plugin Config

```
POST /api/plugins/:name/config
Content-Type: application/json

{"CENSYS_API_ID": "your-id", "CENSYS_API_SECRET": "your-secret"}
```

### Install Requirements

```
POST /api/plugins/:name/install
```

Installs Python dependencies from `requirements.txt` into a `.venv` in the plugin directory.

### Upload Plugin (ZIP)

```
POST /api/plugins/upload
Content-Type: multipart/form-data

file: plugin.zip
```

Extracts the ZIP into `plugins/`. The ZIP should contain a directory with `plugin.yaml` at its root.

### Delete Plugin

```
DELETE /api/plugins/:name
```

Removes the plugin directory and all stored state/config.

---

## Plugin Lifecycle

```
 Drop plugin dir into plugins/
            │
            ▼
 Server scans plugins/ at startup
 (or GET /api/plugins triggers re-scan)
            │
            ▼
 Plugin discovered (disabled by default)
            │
 POST /api/plugins/:name/install     ← Install Python deps
            │
 POST /api/plugins/:name/config      ← Set API keys
            │
 POST /api/plugins/:name/enable      ← Hot-load tools
            │
            ▼
 ✅ AI agent can now use plugin tools
            │
 POST /api/plugins/:name/disable     ← Hot-unload (reversible)
            │
 DELETE /api/plugins/:name            ← Remove completely
```

---

## Example: Censys Plugin

Complete working example in `plugins/censys/`:

```
plugins/censys/
├── plugin.yaml              # Manifest with API key config
├── requirements.txt         # requests>=2.28
├── tools/
│   └── censys-search.yaml   # Tool definition
├── scripts/
│   └── censys_search.py     # Search implementation
├── skills/
│   └── censys-osint/
│       └── SKILL.md         # OSINT methodology
└── knowledge/
    └── censys-api-reference.md
```

Setup:
```bash
# 1. Install dependencies
curl -X POST http://localhost:8080/api/plugins/censys/install -H "Authorization: Bearer TOKEN"

# 2. Configure API keys
curl -X POST http://localhost:8080/api/plugins/censys/config \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"CENSYS_API_ID": "your-id", "CENSYS_API_SECRET": "your-secret"}'

# 3. Enable
curl -X POST http://localhost:8080/api/plugins/censys/enable -H "Authorization: Bearer TOKEN"

# 4. Test - the AI agent now has access to censys-search tool
```

---

## Creating Plugins for Third-Party Services

### Pattern: API wrapper tool

Most plugins follow this pattern:

1. **Script** reads API key from env var, takes arguments, calls API, returns JSON
2. **Tool YAML** defines the command, parameters, and description for the AI
3. **Skill** teaches the AI when and how to use the tool effectively
4. **Knowledge** provides API reference for advanced queries

### Common services to wrap:

| Service | Env Vars | Use Case |
|---------|----------|----------|
| Censys | `CENSYS_API_ID`, `CENSYS_API_SECRET` | Internet-wide host/service search |
| Shodan | `SHODAN_API_KEY` | IoT/infrastructure search |
| VirusTotal | `VT_API_KEY` | Malware/URL/IP reputation |
| SecurityTrails | `ST_API_KEY` | DNS history, subdomains |
| Hunter.io | `HUNTER_API_KEY` | Email discovery |
| BuiltWith | `BW_API_KEY` | Technology profiling |
| URLScan.io | `URLSCAN_API_KEY` | URL analysis/screenshots |
| Netlas | `NETLAS_API_KEY` | Network intelligence |
| BinaryEdge | `BE_API_KEY` | Internet scanner |
| GreyNoise | `GREYNOISE_API_KEY` | IP noise/threat classification |

### Pattern: Custom scanner tool

For tools that run local binaries:

```yaml
name: "my-scanner"
command: "/usr/local/bin/my-scanner"
args: ["--json"]
enabled: true
parameters:
  - name: "target"
    type: "string"
    required: true
    flag: "--target"
```

No `{{PLUGIN_DIR}}` needed - just reference the binary directly.

---

## Security Notes

- Plugin config values (API keys) are stored in the database, never logged
- Config values are masked with `********` in all API responses
- Plugin scripts run with the same permissions as the CyberStrikeAI process
- Python dependencies install into isolated `.venv` per plugin
- All plugin API endpoints require authentication
- Plugin upload (ZIP) should be restricted to trusted sources - a malicious plugin has full system access

---

## Troubleshooting

### Plugin not discovered
- Check `plugin.yaml` exists in `plugins/<name>/plugin.yaml`
- Verify YAML syntax: `python3 -c "import yaml; yaml.safe_load(open('plugins/my-plugin/plugin.yaml'))"`

### Tool not working after enable
- Check config is set: `GET /api/plugins/<name>/config`
- Check requirements installed: `POST /api/plugins/<name>/install`
- Test script directly: `MY_API_KEY=xxx python3 plugins/my-plugin/scripts/tool.py arg1`
- Check logs: `tail -f logs/cyberstrike_*.log | grep plugin`

### Permission denied on script
```bash
chmod +x plugins/my-plugin/scripts/*.py
```

### Import errors in Python script
```bash
# Install deps into plugin venv
curl -X POST http://localhost:8080/api/plugins/my-plugin/install -H "Authorization: Bearer TOKEN"

# Or manually:
cd plugins/my-plugin && python3 -m venv .venv && .venv/bin/pip install -r requirements.txt
```

---

## Plugin Frontend Development

Plugins can extend the CyberStrikeAI web UI with custom pages, navigation items, JavaScript, CSS, and translations - all without modifying core code.

### Frontend Directory Structure

```
plugins/my-plugin/
├── plugin.yaml
├── i18n/                        # Translations (merged into global i18n)
│   ├── en-US.json
│   └── uk-UA.json
└── web/                         # Frontend assets
    ├── pages/                   # HTML page fragments
    │   └── my-page.html
    ├── js/                      # JavaScript modules
    │   └── my-plugin.js
    └── css/                     # Stylesheets
        └── my-plugin.css
```

### Manifest: frontend section

Declare frontend assets in `plugin.yaml`:

```yaml
name: my-plugin
version: "1.0.0"
description: "My plugin with custom UI"

frontend:
  nav_items:
    - id: "my-plugin-dashboard"      # Page ID (must be unique)
      label: "My Plugin"             # Display text in sidebar
      icon: "🔌"                     # Emoji or SVG string
      i18n: "myPlugin.navLabel"      # i18n key (optional, overrides label)

  pages:
    - "my-page.html"                 # Files in web/pages/

  scripts:
    - "my-plugin.js"                 # Files in web/js/

  styles:
    - "my-plugin.css"                # Files in web/css/
```

### Loading Order

When CyberStrikeAI starts and a plugin is enabled:

1. **CSS** loaded first (prevents FOUC)
2. **i18n** translations merged into global bundle
3. **Nav items** injected into sidebar (before Settings)
4. **Page HTML** loaded and appended to main content area
5. **JavaScript** loaded last (DOM is ready)

### Custom Pages

`plugins/my-plugin/web/pages/my-page.html`:

```html
<!-- Page content fragment - no <html>/<head>/<body> wrappers needed -->
<!-- The page ID is derived from filename: my-page.html → page-my-page -->

<div class="page-header">
    <h2 data-i18n="myPlugin.title">My Plugin Dashboard</h2>
</div>

<div class="page-content" style="padding: 20px;">
    <div id="my-plugin-results"></div>
    <button class="btn-primary" onclick="myPluginSearch()">
        <span data-i18n="myPlugin.searchBtn">Search</span>
    </button>
</div>
```

**Page ID mapping**: filename `my-page.html` becomes container `id="page-my-page"`. The `nav_items[].id` must match this (without the `page-` prefix): `id: "my-page"`.

### Custom JavaScript

`plugins/my-plugin/web/js/my-plugin.js`:

```javascript
// Plugin JS runs after the page HTML is in the DOM.
// You have access to all CyberStrikeAI globals:
//   - apiFetch(url, opts)    - authenticated API calls
//   - showNotification(msg, type) - toast notifications
//   - switchPage(pageId)     - navigate to a page
//   - window.t(key)          - i18n translation
//   - CyberStrikePlugins     - plugin loader API

(function() {
    'use strict';

    // Your plugin initialization
    console.log('My plugin loaded');

    // Example: custom API call
    window.myPluginSearch = async function() {
        const resultsEl = document.getElementById('my-plugin-results');
        resultsEl.innerHTML = '<p>Searching...</p>';

        try {
            // Call your plugin's backend tool via the agent API,
            // or call any custom endpoint you've added
            const resp = await apiFetch('/api/plugins/my-plugin/web/data/results.json');
            const data = await resp.json();
            resultsEl.innerHTML = `<pre>${JSON.stringify(data, null, 2)}</pre>`;
        } catch (err) {
            resultsEl.innerHTML = `<p style="color:red;">${err.message}</p>`;
        }
    };

    // Example: hook into page navigation
    const origSwitchPage = window.switchPage;
    const wrappedSwitchPage = function(pageId) {
        origSwitchPage(pageId);
        if (pageId === 'my-page') {
            // Page became active - refresh data
            myPluginSearch();
        }
    };
    window.switchPage = wrappedSwitchPage;
})();
```

### Custom CSS

`plugins/my-plugin/web/css/my-plugin.css`:

```css
/* Plugin styles - use specific selectors to avoid conflicts */
#page-my-page .page-header {
    border-bottom: 1px solid var(--border-color);
    padding-bottom: 12px;
    margin-bottom: 16px;
}

#page-my-page .result-card {
    background: var(--card-bg, var(--bg-secondary));
    border: 1px solid var(--border-color);
    border-radius: 8px;
    padding: 16px;
    margin-bottom: 12px;
}
```

**Tip**: Always scope styles to your page ID (`#page-my-page`) to avoid CSS conflicts with the core UI.

### Translations (i18n)

`plugins/my-plugin/i18n/en-US.json`:

```json
{
  "myPlugin": {
    "navLabel": "My Plugin",
    "title": "My Plugin Dashboard",
    "searchBtn": "Search",
    "noResults": "No results found",
    "error": "An error occurred"
  }
}
```

`plugins/my-plugin/i18n/uk-UA.json`:

```json
{
  "myPlugin": {
    "navLabel": "Мій плагін",
    "title": "Панель мого плагіна",
    "searchBtn": "Пошук",
    "noResults": "Результатів не знайдено",
    "error": "Виникла помилка"
  }
}
```

Translations merge into the global i18n bundle at load time. Use `data-i18n="myPlugin.title"` in HTML or `window.t('myPlugin.title')` in JavaScript.

### Static File Serving

Plugin assets are served at:
```
/api/plugins/<name>/web/<path>    - CSS, JS, HTML, images, data files
/api/plugins/<name>/i18n/<lang>   - Translation JSON files
```

These routes do NOT require authentication (they load before login on page init).

**Security**: path traversal is blocked - `..` in paths returns 403.

### Accessing Plugin Config in Frontend

Plugin config vars (API keys) are not directly accessible in the frontend for security reasons. If your plugin needs to make authenticated API calls from the browser:

1. Create a backend tool or script that reads the env var and proxies the call
2. Call the tool through the agent API: `POST /api/agent-loop/stream`
3. Or create a custom MCP endpoint

### Plugin JS API Reference

The `CyberStrikePlugins` global provides:

```javascript
// Check if a plugin is loaded
CyberStrikePlugins.loaded['my-plugin']  // true/false

// Manually load a plugin (normally automatic)
await CyberStrikePlugins.loadPlugin(pluginState)

// Reload i18n for a plugin (e.g., after language switch)
await CyberStrikePlugins.loadI18n('my-plugin')

// Add a nav item programmatically
CyberStrikePlugins.addNavItem('my-plugin', {
    id: 'my-page',
    label: 'My Page',
    icon: '🔌',
    i18n: 'myPlugin.navLabel'
})
```

### Core UI Globals Available to Plugins

| Function | Description |
|----------|-------------|
| `apiFetch(url, opts)` | Authenticated fetch (adds Bearer token) |
| `showNotification(msg, type)` | Toast notification (`'success'`, `'error'`, `'info'`) |
| `switchPage(pageId)` | Navigate to a page |
| `window.t(key)` / `window.t(key, params)` | i18n translation |
| `i18next.language` | Current language code (`'en-US'`, `'uk-UA'`) |
| `currentConversationId` | Active conversation ID |

---

## Full Plugin Example with Frontend

Complete example: a plugin that adds a custom recon dashboard page.

### Directory structure

```
plugins/recon-dashboard/
├── plugin.yaml
├── i18n/
│   ├── en-US.json
│   └── uk-UA.json
├── web/
│   ├── pages/
│   │   └── recon-dashboard.html
│   ├── js/
│   │   └── recon-dashboard.js
│   └── css/
│       └── recon-dashboard.css
├── tools/
│   └── recon-summary.yaml
└── scripts/
    └── recon_summary.py
```

### plugin.yaml

```yaml
name: recon-dashboard
version: "1.0.0"
description: "Visual recon dashboard with aggregated scan results"
author: "cybersecua"

provides:
  tools: true

frontend:
  nav_items:
    - id: "recon-dashboard"
      label: "Recon Dashboard"
      icon: "🗺️"
      i18n: "reconDashboard.nav"
  pages:
    - "recon-dashboard.html"
  scripts:
    - "recon-dashboard.js"
  styles:
    - "recon-dashboard.css"
```

### i18n/en-US.json

```json
{
  "reconDashboard": {
    "nav": "Recon Dashboard",
    "title": "Reconnaissance Dashboard",
    "subtitle": "Aggregated scan results and attack surface overview",
    "scanTarget": "Scan Target",
    "runScan": "Run Full Recon",
    "results": "Results",
    "noData": "No recon data. Run a scan to populate."
  }
}
```

### web/pages/recon-dashboard.html

```html
<div class="page-header" style="display:flex; justify-content:space-between; align-items:center; padding: 16px 24px; border-bottom: 1px solid var(--border-color);">
    <div>
        <h2 data-i18n="reconDashboard.title">Reconnaissance Dashboard</h2>
        <p style="color: var(--text-muted); font-size: 13px;" data-i18n="reconDashboard.subtitle">Aggregated scan results</p>
    </div>
    <div style="display:flex; gap:8px; align-items:center;">
        <input type="text" id="recon-target-input" placeholder="example.com" style="width:250px;" />
        <button class="btn-primary" onclick="reconDashboardScan()" data-i18n="reconDashboard.runScan">Run Full Recon</button>
    </div>
</div>
<div id="recon-dashboard-content" style="padding: 24px;">
    <p style="color: var(--text-muted); text-align:center; padding:40px;" data-i18n="reconDashboard.noData">No recon data</p>
</div>
```

### web/js/recon-dashboard.js

```javascript
(function() {
    'use strict';

    window.reconDashboardScan = async function() {
        const target = document.getElementById('recon-target-input').value.trim();
        if (!target) {
            showNotification('Enter a target domain', 'error');
            return;
        }

        const content = document.getElementById('recon-dashboard-content');
        content.innerHTML = '<p style="text-align:center; padding:40px;">Running recon on ' + target + '...</p>';

        // Trigger the agent to run recon via the chat API
        try {
            const resp = await apiFetch('/api/agent-loop/stream', {
                method: 'POST',
                headers: {'Content-Type': 'application/json'},
                body: JSON.stringify({
                    message: 'Run full reconnaissance on ' + target + '. Use subfinder, dnsenum, nmap, and nuclei. Report all findings.',
                    role: 'Information_Gathering'
                })
            });
            // The agent runs async - switch to chat to see progress
            switchPage('chat');
            showNotification('Recon started for ' + target + ' - see chat for progress', 'success');
        } catch (err) {
            content.innerHTML = '<p style="color:red;">' + err.message + '</p>';
        }
    };
})();
```

This example demonstrates: a plugin with its own sidebar page, custom UI, i18n support, and integration with the agent API - all without touching any Go source code.

---

## Developer Checklist

When creating a new plugin:

- [ ] Create `plugins/<name>/plugin.yaml` with name, version, description
- [ ] Add tool YAMLs in `tools/` using `{{PLUGIN_DIR}}` for script paths
- [ ] Write Python scripts in `scripts/` that read config from env vars
- [ ] Add `requirements.txt` if using Python packages
- [ ] Define `config` vars in manifest for any API keys needed
- [ ] Write a `skills/*/SKILL.md` teaching the AI when to use your tool
- [ ] Add `knowledge/*.md` reference docs for the RAG knowledge base
- [ ] (Optional) Add `frontend` section with pages, JS, CSS, nav items
- [ ] (Optional) Add `i18n/en-US.json` + `i18n/uk-UA.json` translations
- [ ] Test: install deps, set config, enable, verify tool appears in agent
- [ ] Test: script runs standalone with env vars set manually
- [ ] Test: frontend page loads, navigation works, i18n displays correctly

## Plugin Distribution

### As a directory (local development)

Just copy the plugin folder into `plugins/`:
```bash
cp -r ~/my-plugin plugins/
```

### As a ZIP (remote install)

```bash
# Create a ZIP with the plugin directory at root
cd plugins && zip -r my-plugin.zip my-plugin/

# Upload via API
curl -X POST http://localhost:8080/api/plugins/upload \
  -H "Authorization: Bearer TOKEN" \
  -F "file=@my-plugin.zip"
```

### As a Git repository

```bash
cd plugins && git clone https://github.com/user/cyberstrike-plugin-example.git example
```

### Plugin naming conventions

- Use lowercase with hyphens: `censys`, `virus-total`, `recon-dashboard`
- Prefix tool names with the plugin name to avoid collisions: `censys-search`, `vt-lookup`
- Prefix i18n keys with a unique namespace: `censys.title`, `reconDashboard.nav`
- Prefix page IDs: `censys-page`, `recon-dashboard`
