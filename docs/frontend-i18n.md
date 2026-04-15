## 能盾智御 前端国际化方案

本文档说明 能盾智御 Web 前端（`web/templates/index.html` + `web/static/js/*.js`）的国际化设计与开发规范，确保在不引入打包工具和不改动后端路由的前提下，实现可扩展、低返工的多语言支持。

当前目标：

- **支持中英文切换（zh-CN / en-US）**
- 后续可方便扩展更多语言（如 ja-JP、ko-KR 等）

---

## 一、总体设计原则

- **前端主导的客户端国际化**：所有 UI 文案在浏览器端根据当前语言动态渲染，后端 Go 仅负责结构和数据，不参与语言分发。
- **单一 HTML 模板**：继续使用一份 `index.html` 模板，不为不同语言复制模板文件。
- **文案与逻辑分离**：所有可见文本通过「键值表」管理（多语言 JSON），HTML / JS 只写 key，不直接写中文/英文常量。
- **渐进式改造**：先覆盖 header / 登录 / 侧边栏 / 系统设置等关键区域，其他页面按模块逐步迁移，避免一次性大改动。
- **可回退默认语言**：即使目标语言未完全翻译，也能回退到默认中文，不出现原始 key。

---

## 二、技术选型与目录结构

### 2.1 技术选型

- **i18n 引擎**：使用 [i18next](https://www.i18next.com/) 的浏览器 UMD 版本（通过 CDN 引入），无需打包器。
- **资源格式**：每种语言一份 JSON 文件，采用「域 + 语义」的层级 key 方案，例如：
  - `common.ok`
  - `nav.dashboard`
  - `header.apiDocs`
  - `settings.robot.wecom.token`

### 2.2 目录结构

- `web/templates/index.html`  
  - 页面骨架 + 所有静态文案位置，将逐步改为 `data-i18n` 标记。
- `web/static/js/i18n.js`  
  - 前端 i18n 初始化与 DOM 应用逻辑（本方案新增）。
- `web/static/i18n/`（新增目录）
  - `zh-CN.json`：中文文案（默认语言）
  - `en-US.json`：英文文案
  - 未来可新增：`ja-JP.json`、`ko-KR.json` 等。

---

## 三、文案组织规范

### 3.1 Key 命名约定

- 采用「**模块.语义**」形式，最多 2–3 级，确保可读性：
  - 导航：`nav.dashboard`、`nav.chat`、`nav.settings`
  - 头部：`header.title`、`header.apiDocs`、`header.logout`
  - 登录：`login.title`、`login.subtitle`、`login.passwordLabel`、`login.submit`
  - 仪表盘：`dashboard.title`、`dashboard.refresh`、`dashboard.runningTasks`
  - 系统设置：`settings.title`、`settings.nav.basic`、`settings.nav.robot`、`settings.apply`
  - 机器人配置：`settings.robot.wecom.enabled`、`settings.robot.wecom.token` 等。
- 尽量按「界面区域」而不是「文件名」划分域，便于非开发人员理解。

### 3.2 JSON 示例

`web/static/i18n/zh-CN.json` 示例：

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
  },
  "header": {
    "title": "能盾智御",
    "apiDocs": "API 文档",
    "logout": "退出登录",
    "language": "界面语言"
  },
  "login": {
    "title": "登录 能盾智御",
    "subtitle": "请输入配置中的访问密码",
    "passwordLabel": "密码",
    "passwordPlaceholder": "输入登录密码",
    "submit": "登录"
  }
}
```

英文文件 `en-US.json` 保持相同 key，不同 value：

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
    "title": "能盾智御",
    "apiDocs": "API Docs",
    "logout": "Sign out",
    "language": "Interface language"
  },
  "login": {
    "title": "Sign in to 能盾智御",
    "subtitle": "Enter the access password from config",
    "passwordLabel": "Password",
    "passwordPlaceholder": "Enter password",
    "submit": "Sign in"
  }
}
```

> 约定：**新增界面时，必须先定义 i18n key，再在 HTML/JS 中使用 key**，禁止直接写死中文/英文。

---

## 四、HTML 标记规范（data-i18n）

### 4.1 基本规则

- 使用 `data-i18n` 将元素文本与某个 key 绑定：

```html
<span data-i18n="nav.dashboard">仪表盘</span>
```

- 默认行为：脚本会替换元素的 `textContent`。
- 同时翻译属性时，额外使用 `data-i18n-attr`，逗号分隔多个属性名：

```html
<button
  class="openapi-doc-btn"
  onclick="window.open('/api-docs', '_blank')"
  data-i18n="header.apiDocs"
  data-i18n-attr="title"
  title="API 文档">
  <span data-i18n="header.apiDocs">API 文档</span>
</button>
```

### 4.2 默认文本的作用

- HTML 内的中文默认值作为「**无 JS / 初始化前**」的占位内容：
  - 页面在 JS 尚未加载完成时不会出现空白或 key。
  - JS 初始化后会用当前语言覆盖这些文本。

---

## 五、JavaScript 中的文案规范

### 5.1 全局翻译函数 `t()`

由 `i18n.js` 暴露以下全局函数：

- `window.t(key: string): string`  
  - 返回当前语言下的翻译文本，若缺失则回退到默认语言，再不行则返回 key 本身。
- `window.changeLanguage(lang: string): Promise<void>`  
  - 切换语言并刷新页面文案（不会刷新整页）。

示例（以 `web/static/js/settings.js` 为例）：

```js
// 之前
alert('加载配置失败: ' + error.message);

// 之后
alert(t('settings.loadConfigFailed') + ': ' + error.message);
```

> 规范：**JS 内所有面向用户的提示、按钮文字、对话框标题都应通过 `t()` 获取**，不直接写死中文/英文。

### 5.2 渐进迁移建议

- 优先改造：
  - 频繁弹出的错误提示 / 成功提示；
  - 登录相关、系统设置相关文案。
- 低优先级：
  - 仅面向运维人员的调试提示，可以暂时保留英文/中文常量。

---

## 六、i18n 初始化与语言切换实现

### 6.1 语言选择策略

- 默认语言：`zh-CN`。
- 优先级（从高到低）：
  1. `localStorage` 中的用户选择（key：`csai_lang`）。
  2. 浏览器 `navigator.language`（`zh` 开头 → `zh-CN`，否则 `en-US`）。
  3. 默认 `zh-CN`。

### 6.2 初始化流程（`i18n.js`）

1. 读取初始语言。
2. 初始化 i18next：
   - `lng` 为当前语言；
   - `fallbackLng` 为 `zh-CN`；
   - 资源先留空，采用按需加载。
3. 通过 `fetch` 拉取 `/static/i18n/{lng}.json` 并 `i18next.addResources`。
4. 更新：
   - `<html lang="...">` 属性；
   - 所有带 `data-i18n` / `data-i18n-attr` 的元素。
5. 暴露 `window.t` 与 `window.changeLanguage`。

### 6.3 DOM 应用逻辑

伪代码：

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

> 对于由 JS 动态插入的元素，需要在插入后再次调用 `applyTranslations(新容器)`。

---

## 七、语言切换 UI 规范

### 7.1 位置与形态

- 位置：`index.html` header 右侧 `API 文档` 按钮附近（靠近用户头像）。
- 交互形式：
  - 一个紧凑的语言切换组件，例如：
    - `🌐` 图标 + 当前语言文本（`中文` / `English`）的下拉按钮；
    - 下拉内容列出所有可用语言。

### 7.2 示例结构

```html
<div class="lang-switcher">
  <button class="btn-secondary lang-switcher-btn" onclick="toggleLangDropdown()" data-i18n="header.language">
    <span class="lang-switcher-icon">🌐</span>
    <span id="current-lang-label">中文</span>
  </button>
  <div id="lang-dropdown" class="lang-dropdown" style="display: none;">
    <div class="lang-option" data-lang="zh-CN" onclick="onLanguageSelect('zh-CN')">中文</div>
    <div class="lang-option" data-lang="en-US" onclick="onLanguageSelect('en-US')">English</div>
  </div>
</div>
```

对应 JS（在 `i18n.js` 中）：

```js
function onLanguageSelect(lang) {
  changeLanguage(lang).then(updateLangLabel).catch(console.error);
  closeLangDropdown();
}

function updateLangLabel() {
  const labelEl = document.getElementById('current-lang-label');
  if (!labelEl) return;
  const lang = i18next.language || 'zh-CN';
  labelEl.textContent = lang.startsWith('zh') ? '中文' : 'English';
}
```

> 规范：**语言切换只更新文案，不刷新整页，也不修改 URL hash**。

---

## 八、开发流程建议

### 8.1 新增 / 修改界面的流程

1. 设计界面时，先列出所有文案。
2. 在对应语言 JSON 中补充/修改 key 与翻译。
3. 在 HTML 中使用 `data-i18n`，在 JS 中使用 `t('...')`。
4. 在浏览器中切换中英文，确认两种语言显示都正确。

### 8.2 渐进式改造顺序（推荐）

1. **阶段 1（已规划）**
   - 引入 i18next 与 `i18n.js`。
   - 新建 `zh-CN.json` / `en-US.json`（先覆盖 header / 登录 / 左侧导航）。
   - 实现 header 区域语言切换组件。
2. **阶段 2**（已完成）
   - 系统设置页面（包括机器人配置页面）全部文案 i18n 化。
   - `settings.js` 中的提示与错误信息改用 `t()`。
3. **阶段 3**（进行中）
   - 仪表盘、任务管理、漏洞管理、MCP、Skills、Roles 等页面按模块逐步迁移。
4. **阶段 4**
   - 清理 JS / HTML 中残留的硬编码中文，统一通过 i18n。

---

## 九、后续扩展新语言

当需要新增语言时：

1. 在 `web/static/i18n/` 中新增 `{lang}.json`，复制现有英文/中文文件结构，补充对应翻译。
2. 在语言切换下拉中添加对应选项，例如：
   - `data-lang="ja-JP"` / 文本 `日本語`
3. 无需修改 `i18n.js` 或现有 HTML/JS 逻辑，即可支持新语言。

---

## 十、注意事项与坑点

- **不要复制多份 HTML 模板** 来做多语言，那样维护成本极高，本方案统一由前端 i18n 控制。
- **避免 key 直接用中文/英文句子**，统一采用「模块.语义」短 key，便于 diff 与搜索。
- 避免在 CSS 中写死文本（如 `content: "xxx"`），如确有需要，应通过 JS 设置并走 i18n。
- 对于后端返回的可本地化错误文本（未来可能支持），优先由后端根据 `Accept-Language` 返回对应语言，前端只负责展示。
