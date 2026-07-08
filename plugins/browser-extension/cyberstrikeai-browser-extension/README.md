## CyberStrikeAI Browser Extension

**Version 0.3.8** — Full docs: **README.zh-CN.md**

Chromium DevTools extension: capture Network traffic and send it to CyberStrikeAI for AI-assisted security testing. Aligned with the Burp Suite plugin.

### Quick install

1. `chrome://extensions/` → Developer mode → **Load unpacked**
2. Select `plugins/browser-extension/cyberstrikeai-browser-extension/`
3. Open target page → **F12** → **CyberStrikeAI** tab → **Validate**
4. Select a captured request → **Send** → view **Output**

### Popup vs DevTools panel

| Location | Purpose |
|----------|---------|
| **DevTools panel** | Connection, Validate, capture, Send, Output (primary UI) |
| **Extension popup** | Read-only connection status + version + guide |

### Key features

- **Capture toggle**: **● Capturing** / **○ Paused** — pause stops `getContent` and list updates; Send still works on existing entries
- **Collapsible connection bar** — collapses to `https://host:port` after Validate
- **HTTP/1.1 normalization** — raw HAR stored; display and AI prompt strip HTTP/2 pseudo-headers (`:method`, etc.)
- **Test History** (50 runs) + **Captured Requests** (200/tab, XHR/Fetch filter)
- **SSE streaming** — Progress capped at 512KB; Final uncapped for active run
- **Deferred Markdown** — plain text while streaming; render after done; skip above 100KB
- **Stop** — abort local stream + server cancel via `conversationId`
- **Latest XHR**, **Copy**, project/role/agent send dialog
- Session token with **expires_at** tracking, 30s server probe, restart/unreachable detection

### Data limits (no unbounded growth)

| Data | Limit | Storage |
|------|-------|---------|
| Captures | 200 / tab | In-memory |
| Tabs tracked | 20 | In-memory |
| Test runs | 50 | Panel memory |
| Config / token | Small | `chrome.storage` |

Closing DevTools clears panel data. Closing the browser invalidates the session token.

### Performance

- **DevTools closed** → zero impact on page load
- **Capture paused** → near-zero overhead
- **Capturing + XHR only** → light overhead on matching requests only

### Troubleshooting

After reloading the extension, close DevTools completely and reopen (F12) if you see `chrome.runtime.connect` errors — the old panel context is invalidated.

### Package

```bash
bash package.sh
# → dist/cyberstrikeai-browser-extension.zip
```

### Layout

```text
manifest.json
background/service-worker.js
devtools.js
panel/          # main UI
popup/          # read-only status
lib/            # api, storage, capture, http-normalize, markdown, …
icons/
package.sh
```
