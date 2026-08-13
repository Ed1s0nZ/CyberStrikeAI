# JSVMP-RS6+Cookie生成+412挑战+sdenv补环境

> 难度：★★★★★
> 还原方案：E: jsdom 环境模拟（sdenv 魔改 jsdom + C++ V8 Addon）
> 实现语言：Node.js
> 最后验证日期：2026-04-17

---

## 技术指纹（供 Phase 0.5 自动匹配）

### JS 特征
- [x] 单文件 230KB+，变量名为 `_$` 前缀加2-3位随机字母（如 `_$bV`, `_$ku`），存在 `if($_ts.cd){` 入口判断，函数名每次请求动态变化
- [x] 页面底部存在 `<script>_$xx();</script>` 格式的入口函数调用，函数名每次不同
- [x] HTML 中包含 `<meta id="固定ID" content="动态token" r='m'>` 标签，content 值约 100 字符，每次请求变化
- [x] 内联脚本设置 `$_ts` 全局变量，包含 `nsd`（数字种子）和 `cd`（约 1800 字符的配置字符串），每次请求动态变化

### 参数特征
- [x] Cookie 名称格式为 `XxxYyyZzz2Aaaa` + `S`/`T` 后缀（如 `NfBCSins2OywS` 和 `NfBCSins2OywT`），S 由服务端 Set-Cookie 下发，T 由客户端 JS 生成

### 请求特征
- [x] 首次请求返回 HTTP 412（非标准状态码），响应体为精简 HTML（含 meta + 内联 $_ts 配置 + 外部 JS 引用 + 入口函数调用）
- [x] 412 响应的 Set-Cookie 头同时下发 `acw_tc`（会话标识）和 `XxxS`（服务端标识，HttpOnly，过期时间 10 年）
- [x] 成功请求需携带 3 个 Cookie：`acw_tc` + `XxxS`（服务端下发）+ `XxxT`（客户端生成）
- [x] RS JS 文件 URL 路径包含随机目录名和文件名，但版本号后缀（如 `.e17ed02.js`）在一段时间内固定

### 反调试特征
- [x] JSVMP 内部使用函数表+直接调用（非 `Function.prototype.apply/call`），导致标准 JSVMP Hook 工具无法拦截
- [x] `$_ts` 全局变量在 JS 执行完毕后被清理，运行时无法通过 console 访问
- [x] 检测 `typeof document.all` 必须为 `"undefined"`（浏览器特有行为，纯 JS 无法模拟）

### 混淆类型
- [x] JSVMP（三层嵌套虚拟机），外层解析配置生成 eval 代码，中层执行 Cookie 生成和 XHR 劫持，内层执行 AES/CRC32/Huffman/Base64 加密

### 指纹检测规则（Agent 执行）

```
快速检测命令（30秒内完成）：
  - search_code(keyword="$_ts") → 命中 + 存在 nsd/cd 字段 → 高置信度
  - search_code(keyword="_$") → 大量 _$ 前缀变量 → 辅助确认
  - list_scripts → 存在 230KB+ 文件 → 辅助确认
  - list_network_requests → 首次请求返回 412 → 高置信度
  - 检查 Set-Cookie 头 → 存在 acw_tc + XxxS 格式 Cookie → 直接定位本案例
  匹配判定：412 响应 + $_ts 配置 + _$ 前缀变量 → RS6 高置信度匹配
```

---

## 加密方案

- **算法**：Huffman 编码 → XOR → AES-128-CBC → CRC32 校验 → AES-128-CBC → Base64（URL-safe 变体）
- **密钥来源**：从 `$_ts.cd` 配置字符串中通过 XOR offset 推导提取 45 组密钥，密钥随每次 412 响应动态变化
- **加密流程**：
  1. 收集浏览器指纹（Canvas、WebGL、UA、屏幕尺寸、navigator 属性等）
  2. 组装 basearr（约 154 字节 TLV 结构，包含 8 种 type 的环境数据）
  3. basearr → Huffman 编码 → XOR 加密 → AES-128-CBC 加密 → 追加 CRC32 校验 → 再次 AES-128-CBC → Base64 编码
  4. Cookie T = `"0"` + Base64 结果
  5. 写入 `document.cookie`，然后通过 `location.replace` 或 XHR 重新请求页面
- **签名公式**：Cookie T 不是简单的签名，而是完整的加密数据包，包含浏览器指纹、时间戳、随机数等信息，服务端解密后验证指纹一致性

---

## 已验证定位路径（Phase 0.5 命中后直接执行）

```
步骤 1: Camoufox 反检测浏览器 + 网络捕获
  启动 Camoufox（C++ 引擎级指纹伪装），注入 XHR/Fetch/Crypto 持久化 Hook，导航到目标页面。
  通过 list_network_requests 发现请求链路：
    请求 #1 返回 412 + Set-Cookie + HTML body
    请求 #2 加载 230KB JS 文件
    请求 #3 携带 3 个 Cookie 返回 200

步骤 2: 412 响应体分析
  通过 get_network_request(id=1, include_body=true) 获取完整的 412 响应。
  发现 HTML body 结构为：
    <meta> 标签（动态 token）+ $_ts 内联配置（nsd + cd）+ 外部 JS 引用 + 入口函数调用 _$xx()
  同时从 Set-Cookie 头提取到 acw_tc 和 XxxS 两个服务端 Cookie。

步骤 3: 成功请求对比
  通过 get_network_request(id=3, include_headers=true) 对比成功请求的 Cookie 头，
  发现多了一个 XxxT Cookie — 这就是客户端 JS 生成的。
  同时发现请求头中 Referer 和 Sec-Fetch-Site: same-origin 是必需的。

步骤 4: Cookie setter Hook 验证
  注入 document.cookie setter Hook，发现 Cookie 日志为空 —
  说明RS JS 不是通过标准 document.cookie setter 写入的
  （可能是通过 jsdom 内部机制或 location.replace 触发的重新请求时由浏览器自动携带）。

步骤 5: JSVMP 插桩尝试
  使用 hook_jsvmp_interpreter 对RS JS 进行插桩，发现日志为空 —
  确认RS6使用内部函数表+直接调用，不经过 Function.prototype.apply/call，
  标准 JSVMP Hook 无效。

步骤 6: 环境对比
  使用 compare_env 采集 Camoufox 中的真实浏览器环境数据，
  发现 document.cookie 中包含 enable_XxxYyy=true 标记，
  说明RS JS 执行成功后会设置一个启用标记。

步骤 7: RS JS 保存
  通过 save_script 将 230KB 的RS JS 文件保存到本地，分析其结构：
  以 if($_ts.cd){ 开头，内部是 JSVMP 解释器，
  window 仅出现 3 次（环境访问全部通过 JSVMP 字节码间接进行）。

步骤 8: 补环境方案选型
  参考开源社区的手动补环境方案和 sdenv/纯算/JsRpc 多方案，
  确定 sdenv（魔改 jsdom + C++ V8 Addon）是RS6的最优纯 Node.js 方案。
```

---

## 还原代码模板

### 核心函数：RS6 Cookie 生成 — 基于 sdenv

```javascript
/**
 * RS6 Cookie 生成 — 基于 sdenv (魔改 jsdom + C++ V8 Addon)
 * 依赖: npm install sdenv (需要 pnpm 安装 + node-gyp 编译原生模块)
 * 原理:
 *   sdenv 的核心是 documentAll.node (51行C++)，用 V8 的
 *   ObjectTemplate::MarkAsUndetectable() 实现 document.all 的浏览器特有行为
 *   (typeof === "undefined" 但可调用)，加上完整的浏览器环境模拟，
 *   让RS JSVMP 在 Node.js 中真实执行并生成有效 Cookie。
 */

process.env.NODE_TLS_REJECT_UNAUTHORIZED = "0";

const { jsdomFromUrl } = require('sdenv');
const https = require('https');

class RuishuClient {
  constructor(config = {}) {
    this.host = config.host;           // 目标主机名
    this.entryPath = config.entryPath; // 入口页面路径（返回 412 的页面）
    this.ua = config.userAgent || 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36';
    this.dom = null;
    this.cookies = '';
    this.ready = false;
  }

  async init() {
    const url = `https://${this.host}${this.entryPath}`;
    this.dom = await jsdomFromUrl(url, {
      userAgent: this.ua,
      consoleConfig: { error: () => {} },
    });

    // 等待RS JS 执行完毕
    await new Promise(resolve => {
      this.dom.window.addEventListener('sdenv:exit', () => resolve());
      setTimeout(resolve, 8000);
    });

    this.cookies = this.dom.cookieJar.getCookieStringSync(`https://${this.host}`);
    this.ready = true;
    return this;
  }

  get(path) {
    if (!this.ready) throw new Error('请先调用 init()');
    return new Promise((resolve, reject) => {
      https.request({
        hostname: this.host, port: 443, path, method: 'GET',
        headers: {
          'User-Agent': this.ua,
          'Host': this.host,
          'Cookie': this.cookies,
          'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8',
          'Referer': `https://${this.host}/`,
          'Sec-Fetch-Site': 'same-origin',
          'Sec-Fetch-Mode': 'navigate',
          'Sec-Fetch-Dest': 'document',
        },
      }, res => {
        let body = '';
        res.on('data', chunk => body += chunk);
        res.on('end', () => resolve({ status: res.statusCode, body }));
      }).on('error', reject).end();
    });
  }

  close() {
    if (this.dom) {
      try { this.dom.window.close(); } catch(e) {}
      this.dom = null;
      this.ready = false;
    }
  }
}
```

---

## 踩坑记录

| # | 坑 | 现象 | 解决方法 |
|---|---|------|---------|
| 1 | 误判反爬类型 | 通过 webFetch 直接请求返回 412，猜测是 JSL 加速乐（`__jsl_clearance_s` cookie），实际是RS信息（RS） | 不要猜，用 Camoufox 实际抓包看 412 响应体结构和 Set-Cookie 头 |
| 2 | Cookie 有效但请求返回 400 | Camoufox 获取的 Cookie 有效（curl 测试返回 200），但 axios 请求返回 400 | RS不仅检查 Cookie，还检查请求头指纹 — UA 必须与生成 Cookie 时一致，且必须包含 `Sec-Fetch-Site: same-origin` 等 Fetch Metadata 头 |
| 3 | jsdom 执行RS JS 卡死（超时） | 原生 jsdom + `runScripts: 'dangerously'` 加载 412 页面，RS JS 执行后进入死循环 | `typeof document.all` 在 jsdom 中返回 `"object"` 而非 `"undefined"`，RS检测到非浏览器环境后故意卡死。必须用 sdenv（C++ 层面实现 `MarkAsUndetectable()`）或真实浏览器 |
| 4 | V8 `%GetUndetectable()` 不够 | 用 V8 内部函数获取的 undetectable 对象通过了 `typeof === "undefined"` 检测，但它是空对象，没有 `HTMLAllCollection` 的方法 | RS JSVMP 后续尝试调用该对象的方法时报错 `_$xx[_$yy[49]] is not a function`。`document.all` 不仅要通过 typeof 检测，还要有完整的集合行为 |
| 5 | RS入口函数名每次不同 | 412 响应体底部的入口函数调用（如 `_$bV()`, `_$l1()`, `_$jH()`）每次请求都不同 | 正则匹配时需要用 `_\$[a-zA-Z0-9]+\(\)` 而非固定函数名 |
| 6 | Node.js v24 的 Navigator 原型有只读属性 | `Navigator.prototype` 上的 `language` 等属性已经有了 getter，用 `Object.assign` 赋值报错 | 必须用 `Object.defineProperty` 逐个覆盖 |
| 7 | sdenv 原生模块编译 | sdenv 的核心 `documentAll.node` 是 C++ 原生模块，需要 node-gyp 编译 | pnpm 安装时默认不执行 build scripts，需要手动 `pnpm approve-builds` 或 `npx node-gyp rebuild` |
| 8 | JSVMP Hook 工具对RS6无效 | Camoufox 的 `hook_jsvmp_interpreter` 通过 Hook `Function.prototype.apply/call` 追踪，但RS6使用内部函数表+直接调用，日志为空 | 不是所有 JSVMP 都能用标准 Hook 工具分析，需要根据具体实现选择分析方法 |

---

## 变体说明

| 变体 | 差异点 | 影响 |
|------|--------|------|
| RS 4/5 代 vs 6 代 | RS6 JS 文件约 230KB，比 4/5 代（约 200KB）更大，环境检测项更多 | 4/5 代可通过手动补环境成功，6 代手动补环境极其困难，建议直接用 sdenv |
| Cookie-only vs Cookie+URL后缀 | 大部分RS站点只需要 Cookie 即可访问。少数站点还需要 URL 后缀签名 | sdenv 只能生成 Cookie，不能生成 URL 后缀。需要后缀的站点建议用 JsRpc 方案 |
| HTTP vs HTTPS | 部分RS站点使用 HTTP，其他使用 HTTPS | sdenv 的 `jsdomFromUrl` 两种协议都支持，但 HTTPS 站点需要设置 `NODE_TLS_REJECT_UNAUTHORIZED=0`（某些政府站点的证书链不完整） |
| 静态页面 vs API 接口 | 列表页和详情页可能是纯静态 HTML（服务端渲染），用 cheerio 解析即可。其他站点可能有 JSON API 接口 | API 请求可能需要额外的签名参数（如 `sign = MD5(itemId + searchValue + timestamp)`） |
| `$_ts` 配置差异 | 不同站点的 `$_ts` 配置结构可能不同，有的只有 `nsd` 和 `cd`，其他可能还有 `cp`、`aebi` 等字段 | sdenv 方案不需要关心这些差异，因为它让RS JS 自己解析配置 |
| 服务端动态更新 | RS服务端可以随时更新 `basearr` 中的环境检测值 | 纯算方案会因此失效，sdenv 方案通常不受影响（除非RS新增了 sdenv 未模拟的检测点） |
