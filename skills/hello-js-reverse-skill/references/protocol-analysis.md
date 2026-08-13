# 协议层分析与对抗

## 概述

有些网站不仅在 JS 层面做加密保护，还在协议层面进行检测（TLS 指纹、HTTP/2 强制等）。
当算法全部还原正确但请求仍然失败时，需要考虑协议层问题。

## 1. TLS 指纹检测

### 原理

服务器通过分析 TLS Client Hello 报文中的特征来识别客户端类型：
- Cipher Suites 列表和顺序
- TLS 扩展列表和顺序
- 支持的曲线（Elliptic Curves）
- 签名算法
- ALPN 协议列表

不同客户端（Chrome、Firefox、curl、Node.js）的 TLS 指纹不同。

### 检测表现

- 请求返回 `403 Forbidden`
- 返回空响应或错误页面
- 返回 `token failed`、`access denied` 等自定义错误
- 同样的参数在浏览器中正常，在 Node.js 中失败

### 解决方案

#### 方案 A：使用 Camoufox 反检测浏览器（最可靠）

```javascript
// 使用 camoufox-reverse MCP 在 Camoufox 浏览器环境中执行请求
// [camoufox-reverse] evaluate_js
const response = await fetch('https://target.com/api/data', {
    method: 'GET',
    headers: { 'Cookie': cookieValue },
    credentials: 'include'
});
const data = await response.json();
```

```
MCP 操作：
[camoufox-reverse] launch_browser()
[camoufox-reverse] navigate(url="https://target.com")
[camoufox-reverse] evaluate_js(expression="fetch(url, options).then(r=>r.json()).then(d=>JSON.stringify(d))")
```

#### 方案 B：curl-impersonate

```bash
# 使用 curl-impersonate 模拟 Chrome 的 TLS 指纹
curl_chrome116 https://target.com/api/data \
    -H "Cookie: session=xxx" \
    -H "User-Agent: Mozilla/5.0 ..."
```

Node.js 中通过子进程调用：
```javascript
const { execSync } = require('child_process');

function fetchWithChromeTLS(url, headers = {}) {
    const headerArgs = Object.entries(headers)
        .map(([k, v]) => `-H "${k}: ${v}"`)
        .join(' ');
    const result = execSync(`curl_chrome116 "${url}" ${headerArgs} -s`, {
        encoding: 'utf-8',
        timeout: 30000,
    });
    return JSON.parse(result);
}
```

#### 方案 C：自定义 TLS（高级）

使用支持自定义 TLS 指纹的库（如 `tls-client` 的 Node.js 绑定）。

## 2. HTTP/2 协议

### 检测表现

- HTTP/1.1 请求返回错误
- HTTP/2 请求正常返回数据
- 抓包可以看到服务器仅接受 h2 连接

### Node.js HTTP/2 请求

```javascript
const http2 = require('http2');

function http2Request(url, options = {}) {
    return new Promise((resolve, reject) => {
        const urlObj = new URL(url);
        const client = http2.connect(`${urlObj.protocol}//${urlObj.host}`);
        
        client.on('error', reject);
        
        const headers = {
            ':method': options.method || 'GET',
            ':path': urlObj.pathname + urlObj.search,
            ':scheme': urlObj.protocol.replace(':', ''),
            ':authority': urlObj.host,
            'user-agent': options.userAgent || 'Mozilla/5.0 ...',
            'accept': 'application/json',
            ...options.headers,
        };
        
        if (options.cookie) {
            headers['cookie'] = options.cookie;
        }
        
        const req = client.request(headers);
        
        let data = '';
        req.on('data', (chunk) => { data += chunk; });
        req.on('end', () => {
            client.close();
            resolve(data);
        });
        req.on('error', reject);
        
        if (options.body) {
            req.write(options.body);
        }
        req.end();
    });
}

// 使用示例
async function main() {
    const data = await http2Request('https://target.com/api/data?page=1', {
        headers: {
            'cookie': 'm=xxx; sessionid=yyy',
            'referer': 'https://target.com/',
        }
    });
    console.log(JSON.parse(data));
}
```

## 3. User-Agent 校验

### 常见场景

- 某些页面要求特定 UA
- 不同 UA 返回不同内容
- 空 UA 或非浏览器 UA 被拒绝

### Node.js 设置

```javascript
const axios = require('axios');

const client = axios.create({
    headers: {
        'User-Agent': 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
    }
});
```

### MCP 侧验证

```
[camoufox-reverse] launch_browser(os_type="macos")
[camoufox-reverse] get_fingerprint_info
→ 验证当前浏览器环境中的 UA、平台和指纹是否与目标站点要求一致
```

## 4. Referer 校验

### 常见场景

- API 接口检查 Referer 头
- 必须来自特定页面的请求
- 缺少 Referer 返回 403

### 解决方案

```javascript
headers: {
    'Referer': 'https://target.com/page',
    'Origin': 'https://target.com',
}
```

## 5. CORS 与跨域

### 预检请求 (Preflight)

```javascript
// 某些请求会先发 OPTIONS 预检
// 确保模拟请求时也处理 CORS 相关头
headers: {
    'Origin': 'https://target.com',
    'Access-Control-Request-Method': 'POST',
    'Access-Control-Request-Headers': 'content-type',
}
```

## 6. 请求频率限制

### 常见策略

- IP 维度限流
- Cookie/Session 维度限流
- 请求间隔检测
- 滑动窗口计数

### 对抗方式

```javascript
// 请求间隔控制
function sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
}

async function fetchWithRateLimit(urls, delayMs = 1000) {
    const results = [];
    for (const url of urls) {
        const data = await fetch(url);
        results.push(data);
        await sleep(delayMs + Math.random() * 500); // 随机化延迟
    }
    return results;
}
```

## 7. IP 封禁对抗

### 代理轮换

```javascript
const HttpsProxyAgent = require('https-proxy-agent');

const proxies = [
    'http://proxy1:port',
    'http://proxy2:port',
];

function getRandomProxy() {
    return proxies[Math.floor(Math.random() * proxies.length)];
}

async function fetchWithProxy(url, options = {}) {
    const agent = new HttpsProxyAgent(getRandomProxy());
    return fetch(url, { ...options, agent });
}
```

## 诊断流程

```
请求失败？
    │
    ├─ 检查 HTTP 状态码
    │   ├─ 403 → TLS 指纹 / Referer / UA / IP 封禁
    │   ├─ 412 → 前置条件失败（Cookie / Token 过期）
    │   ├─ 429 → 频率限制
    │   └─ 200 但数据异常 → 参数错误 / 加密不对
    │
    ├─ 同样的请求在浏览器中成功？
    │   ├─ 是 → TLS 指纹问题，切换到浏览器方案
    │   └─ 否 → 参数/Cookie 确实有问题，重新分析
    │
    ├─ curl 命令测试
    │   ├─ curl 成功 → Node.js 请求头/参数构造问题
    │   └─ curl 也失败 → TLS / 协议 / IP 问题
    │
    └─ 使用 HTTP/2？
        ├─ HTTP/2 成功 → 切换到 http2 模块
        └─ HTTP/2 也失败 → TLS 指纹问题
```
