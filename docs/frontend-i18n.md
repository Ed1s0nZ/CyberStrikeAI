## CyberStrikeAI Frontend Internationalization Plan

This document describes the i18n design and development conventions for the CyberStrikeAI web frontend (`web/templates/index.html` + `web/static/js/*.js`). The goal is a scalable, low-rework multi-language stack that does not introduce a bundler and does not change any backend routes.

Current goals:

- **English / Ukrainian / Chinese switching (`en-US` / `uk-UA` / `zh-CN`)**
- Easily extendable to more languages later (`ja-JP`, `ko-KR`, etc.)

---

## 1. Overall Design Principles

- **Frontend-driven client-side i18n**: every UI string is rendered in the browser according to the active language. The Go backend stays agnostic of locale and only serves structure and data.
- **Single HTML template**: keep one `index.html`; do not fork per-language templates.
- **Text separated from logic**: every visible string lives in a key/value catalog (per-language JSON). HTML / JS reference keys only - never hard-coded Chinese / English / Ukrainian literals.
- **Progressive migration**: cover header, login, sidebar, and system settings first, then migrate the remaining pages by module. Avoid one giant rewrite.
- **Fallback language**: if the target language is incomplete, fall back to the default language instead of exposing raw keys to users.

---

## 2. Technology Choice and Directory Layout

### 2.1 Technology Choice

- **i18n engine**: the browser UMD build of [i18next](https://www.i18next.com/), loaded via CDN - no bundler required.
- **Resource format**: one JSON file per language with a `domain.semantic` key hierarchy, e.g.
  - `common.ok`
  - `nav.dashboard`
  - `header.apiDocs`
  - `settings.robot.wecom.token`

### 2.2 Directory Layout

- `web/templates/index.html`
  - Page skeleton plus every static-text location, gradually annotated with `data-i18n` markers.
- `web/static/js/i18n.js`
  - Frontend i18n initialization and DOM application logic (introduced by this plan).
- `web/static/i18n/` (new directory)
  - `en-US.json` - English (default)
  - `uk-UA.json` - Ukrainian
  - `zh-CN.json` - Chinese
  - Future additions: `ja-JP.json`, `ko-KR.json`, etc.

---

## 3. String Organization Conventions

### 3.1 Key Naming

- Use `<module>.<semantic>` with at most 2–3 levels for readability:
  - Navigation: `nav.dashboard`, `nav.chat`, `nav.settings`
  - Header: `header.title`, `header.apiDocs`, `header.logout`
  - Login: `login.title`, `login.subtitle`, `login.passwordLabel`, `login.submit`
  - Dashboard: `dashboard.title`, `dashboard.refresh`, `dashboard.runningTasks`
  - System settings: `settings.title`, `settings.nav.basic`, `settings.nav.robot`, `settings.apply`
  - Chatbot configuration: `settings.robot.wecom.enabled`, `settings.robot.wecom.token`, etc.
- Group keys by **UI area** rather than by source-file name so non-developers can follow them.

### 3.2 JSON Example

`web/static/i18n/en-US.json`:

```json
{
  "common": {
    "ok": "OK",
    "cancel": "Cancel"
  },
  "nav": {
    "dashboard": "Dashboard",
    "chat": "Chat",
    "infoCollect": "Recon",
    "tasks": "Tasks",
    "vulnerabilities": "Vulnerabilities",
    "settings": "Settings"
  },
  "header": {
    "title": "CyberStrikeAI",
    "apiDocs": "API Docs",
    "logout": "Sign out",
    "language": "Interface language"
  },
  "login": {
    "title": "Sign in to CyberStrikeAI",
    "subtitle": "Enter the access password from config",
    "passwordLabel": "Password",
    "passwordPlaceholder": "Enter password",
    "submit": "Sign in"
  }
}
```

The Chinese file `zh-CN.json` keeps the same keys with different values:

```json
{
  "common": {
    "ok": "确定",
    "cancel": "取消"
  },
  "nav": {
    "dashboard": "仪表盘",
    "chat": "对话",
    "infoCollect": "信息收集",
    "tasks": "任务管理",
    "vulnerabilities": "漏洞管理",
    "settings": "系统设置"
  }
}
```

> Rule: **when adding a new UI element, define the i18n key first, then reference it from HTML/JS** - never hard-code a literal string.

---

## 4. HTML Markup Convention (`data-i18n`)

### 4.1 Basic Rules

- Bind an element's text to a key with `data-i18n`:

```html
<span data-i18n="nav.dashboard">Dashboard</span>
```

- Default behavior: the loader replaces the element's `textContent`.
- To translate attributes as well, add `data-i18n-attr` with a comma-separated list of attribute names:

```html
<button
  class="openapi-doc-btn"
  onclick="window.open('/api-docs', '_blank')"
  data-i18n="header.apiDocs"
  data-i18n-attr="title"
  title="API Docs">
  <span data-i18n="header.apiDocs">API Docs</span>
</button>
```

### 4.2 Purpose of the Default Text

- The literal text inside the HTML acts as a "no-JS / before-init" placeholder:
  - The page does not flash blank space or raw keys while JS is still loading.
  - After initialization, JS overwrites the placeholder with the active language's string.

---

## 5. JavaScript String Conventions

### 5.1 Global Translation Helper `t()`

`i18n.js` exposes the following globals:

- `window.t(key: string): string`
  - Returns the translation in the current language, falling back to the default language, and ultimately to the key itself if no translation exists.
- `window.changeLanguage(lang: string): Promise<void>`
  - Switches language and refreshes page strings in place - it does not reload the page.

Example (from `web/static/js/settings.js`):

```js
// Before
alert('Load config failed: ' + error.message);

// After
alert(t('settings.loadConfigFailed') + ': ' + error.message);
```

> Rule: **every user-facing alert, button label, and dialog title in JS goes through `t()`** - never hard-code a literal.

### 5.2 Progressive-Migration Guidance

- Prioritize:
  - Frequently-shown error / success toasts;
  - Login and system-settings strings.
- Lower priority:
  - Diagnostic output intended only for operators can temporarily stay as literal English or Chinese.

---

## 6. i18n Initialization and Language Switching

### 6.1 Language-Selection Strategy

- Default language: `en-US`.
- Priority (highest to lowest):
  1. User's explicit choice stored in `localStorage` (key: `csai_lang`).
  2. Browser `navigator.language` (`uk*` → `uk-UA`, `zh*` → `zh-CN`, otherwise `en-US`).
  3. Default `en-US`.

### 6.2 Initialization Flow (`i18n.js`)

1. Determine the initial language.
2. Initialize i18next:
   - `lng` is the current language;
   - `fallbackLng` is `en-US`;
   - Resources are empty initially - loaded on demand.
3. `fetch` `/static/i18n/{lng}.json` and call `i18next.addResources`.
4. Update:
   - The `<html lang="...">` attribute;
   - Every element carrying `data-i18n` / `data-i18n-attr`.
5. Expose `window.t` and `window.changeLanguage`.

### 6.3 DOM Application Logic

Pseudo-code:

```js
function applyTranslations(root = document) {
  const elements = root.querySelectorAll('[data-i18n]');
  elements.forEach(el => {
    const key = el.getAttribute('data-i18n');
    if (!key) return;
    const text = i18next.t(key);
    if (text) {
      el.textContent = text;
    }

    const attrList = el.getAttribute('data-i18n-attr');
    if (attrList) {
      attrList.split(',').map(s => s.trim()).forEach(attr => {
        if (!attr) return;
        const val = i18next.t(key);
        if (val) el.setAttribute(attr, val);
      });
    }
  });
}
```

> For elements that JS inserts dynamically, call `applyTranslations(newContainer)` again after insertion.

---

## 7. Language-Switcher UI Convention

### 7.1 Placement and Shape

- Location: `index.html` header, near the **API Docs** button and the user avatar on the right.
- Interaction:
  - A compact switcher, for example:
    - A `🌐` icon + current-language label (`English` / `Українська` / `中文`) in a dropdown button;
    - Dropdown lists every available language.

### 7.2 Example Structure

```html
<div class="lang-switcher">
  <button class="btn-secondary lang-switcher-btn" onclick="toggleLangDropdown()" data-i18n="header.language">
    <span class="lang-switcher-icon">🌐</span>
    <span id="current-lang-label">English</span>
  </button>
  <div id="lang-dropdown" class="lang-dropdown" style="display: none;">
    <div class="lang-option" data-lang="en-US" onclick="onLanguageSelect('en-US')">English</div>
    <div class="lang-option" data-lang="uk-UA" onclick="onLanguageSelect('uk-UA')">Українська</div>
    <div class="lang-option" data-lang="zh-CN" onclick="onLanguageSelect('zh-CN')">中文</div>
  </div>
</div>
```

Matching JS (in `i18n.js`):

```js
function onLanguageSelect(lang) {
  changeLanguage(lang).then(updateLangLabel).catch(console.error);
  closeLangDropdown();
}

function updateLangLabel() {
  const labelEl = document.getElementById('current-lang-label');
  if (!labelEl) return;
  const lang = i18next.language || 'en-US';
  if (lang.startsWith('uk')) { labelEl.textContent = 'Українська'; return; }
  if (lang.startsWith('zh')) { labelEl.textContent = '中文'; return; }
  labelEl.textContent = 'English';
}
```

> Rule: **language switch only updates strings** - no full-page reload, no URL-hash mutation.

---

## 8. Development Workflow

### 8.1 Adding or Changing a UI Surface

1. List every user-facing string while designing the surface.
2. Add / modify keys and translations in every language JSON.
3. In HTML, reference them via `data-i18n`; in JS, via `t('...')`.
4. Switch languages in the browser and confirm all locales render correctly.

### 8.2 Suggested Progressive-Migration Order

1. **Phase 1 (done)**
   - Introduce i18next and `i18n.js`.
   - Create `en-US.json` / `zh-CN.json` / `uk-UA.json` (header, login, left nav covered first).
   - Build the header language-switcher component.
2. **Phase 2 (done)**
   - All strings on the system-settings page (including the chatbot-configuration subpage) moved to i18n.
   - `settings.js` alerts and error toasts routed through `t()`.
3. **Phase 3 (in progress)**
   - Dashboard, Task Management, Vulnerability Management, MCP, Skills, Roles - migrated module by module.
4. **Phase 4**
   - Sweep out any remaining hard-coded strings in JS / HTML and route them through i18n.

---

## 9. Adding Another Language Later

When a new language is needed:

1. Add `web/static/i18n/{lang}.json`, copying the structure of an existing locale and filling in translations.
2. Add the matching option in the language-switcher dropdown, e.g.
   - `data-lang="ja-JP"` / label `日本語`.
3. No changes are required in `i18n.js` or in existing HTML/JS - the new language just works.

---

## 10. Gotchas and Pitfalls

- **Do not fork the HTML template** to implement multiple languages - maintenance cost explodes; the i18n layer is the single source of truth.
- **Never use Chinese/English sentences as keys** - keep `module.semantic` short keys for easy diffing and searching.
- Avoid hard-coded text in CSS (`content: "xxx"`). If you truly need it, set the text from JS via i18n.
- For backend-returned error messages (possibly localizable in the future), prefer letting the backend pick a language based on `Accept-Language` and let the frontend just display what it receives.
