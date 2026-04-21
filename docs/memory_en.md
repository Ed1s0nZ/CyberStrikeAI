# Persistent Memory - Guide

CyberStrikeAI can remember facts, credentials, targets, and notes **across conversation compressions and server restarts**. This page explains how memory works, how agents use it, and how to manage it through the web UI.

---

## How It Works

Every time the agent completes a tool call it can call `store_memory` to save a fact to a small SQLite-backed store (`agent_memories` table in the main database). In addition, **tool results are automatically persisted** as `tool_run` category memory entries without any explicit agent action - so completed scans, output summaries, and execution metadata are always available for future sessions.

On every subsequent request all stored entries are injected into the system prompt inside a `<persistent_memory>` block, grouped by category. This means the agent never forgets credentials, targets, or key findings even when conversation history is compressed.

### Agent Introspection (Memory Similarity)

Before every major new action the agent runs a mandatory introspection pass:

1. **Memory similarity check** - queries memories semantically similar to the current user input (`retrieve_memory` with the raw query), surfacing relevant prior findings.
2. **Entity-based lookup** - extracts IP addresses and domain names from the user input and fetches all memories tagged with those entities (`list_memories` with entity filter).
3. **Knowledge-base preflight** - runs a focused KB query combining the user input with penetration-testing terminology to retrieve relevant exploitation techniques and tool guidance.

The results are injected into the system prompt as a `<memory_similarity_context>` block:

```
<memory_similarity_context>
Similar memory entries related to the current task. Reuse this context before launching new scans:
- Query matches:
  • [active][tool_run] nmap_192.168.1.1 => open ports: 22/tcp ssh, 80/tcp http, 443/tcp https
- Entity matches:
  • [entity:192.168.1.1][vulnerability] sqli_login => /login.php?id= is injectable (union-based)
</memory_similarity_context>
```

This prevents duplicate scans and ensures the agent builds on prior findings rather than restarting from scratch.

### Memory Categories

| Category | What to store |
|---|---|
| `credential` | Discovered passwords, API keys, tokens, hashes |
| `target` | IPs, domains, hostnames, service versions |
| `vulnerability` | CVEs, exploit notes, proof-of-concept details |
| `fact` | General observations and context |
| `note` | Operational reminders and planning notes |
| `tool_run` | Automatically stored tool execution results (scan output summaries, execution metadata) |

---

## Agent Tools

The agent has four dedicated memory tools:

| Tool | Purpose |
|---|---|
| `store_memory` | Persist a key/value fact, upserts by key |
| `retrieve_memory` | Full-text search across stored memories |
| `list_memories` | List all entries, optionally filtered by category |
| `delete_memory` | Remove an entry by UUID |

---

## Memory UI Panel

Open the **Memory** page from the left sidebar (below Skills, above System Settings).

### Stats Strip

The top bar shows the total entry count plus per-category breakdowns. Click a category chip to instantly filter the list to that category.

### Search

Type in the search box to filter entries by key or value (case-insensitive). Press **Escape** or click the × button to clear.

### Category Filters

Use the colour-coded filter buttons to narrow entries to a single category. Click the active button again to clear the filter.

### Add Entry

Click **+ Add Entry** to open the create dialog. Fill in:

- **Key** - a short, unique label (e.g. `admin_password`)
- **Value** - the fact to remember
- **Category** - choose from the dropdown
- **Conversation ID** - optional; links the entry to a specific conversation

Click **Save** to store the entry. If a memory with the same key already exists it is updated in-place.

### Edit Entry

Click the pencil icon on any entry to open the edit dialog. The key, value, and category can all be changed. Conversation ID is not editable after creation.

### Delete Entry

Click the trash icon on any entry to delete it. You will be prompted to confirm.

### Bulk Delete

Click **Delete All** in the toolbar to remove all visible entries (respects the active category filter). You will be prompted to confirm with the exact count.

---

## Configuration

Memory is enabled by default. To adjust it in `config.yaml`:

```yaml
agent:
  memory:
    enabled: true       # Set to false to disable entirely
    max_entries: 200    # Hard cap (0 = unlimited)
```

When `enabled` is `false` the Memory UI panel will display a disabled notice and no API calls will store or retrieve data.

---

## API Endpoints

All endpoints require authentication (Bearer token from login).

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/memories` | List entries; supports `?search=`, `?category=`, `?limit=` |
| `GET` | `/api/memories/stats` | Count totals and per-category breakdown |
| `POST` | `/api/memories` | Create a new entry |
| `PUT` | `/api/memories/:id` | Update key, value, and category by UUID |
| `DELETE` | `/api/memories/:id` | Delete a single entry |
| `DELETE` | `/api/memories` | Bulk delete; supports `?category=` filter |
