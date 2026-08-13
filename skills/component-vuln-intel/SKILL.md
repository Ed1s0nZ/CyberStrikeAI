---
name: component-vuln-intel
description: >-
  联网情报收集:识别组件后查询根组件与组件家族的CVE/搜索引擎/GitHub PoC/即时情报，并在受阻时换路。Use when a framework, component, product, or version is identified and must be searched before exploit.
metadata:
  tags: [渗透测试, penetration-testing, 红队]
---

## 联网情报收集（识别组件→先查结构化 CVE 情报；结果=线索/tentative，验证前不是 confirmed Fact）

```
用 {C}=组件名、{V}=精确版本、{VENDOR}=厂商、{PRODUCT}=产品名替换占位符。先确认组件指纹；版本未知不阻塞首次查询，但结果只能记为候选，不能宣称 CVE 适用。

🔴CVE 查询优先使用 cve-search MCP。若目标工具未出现在当前工具 schema，先用 tool_search 搜索 `cve-search`；tool_search 不可用或未找到时直接进入降级路径。

- 每个 `task_id + target + root_component` 初始化 `cve_lookup_state=pending`，通过 `upsert_project_fact` 保存隔离状态；切题、目标变化或发现新的根组件时重置，不能继承其他组件的 `queried`。
- 已知 CVE ID → 调用 cve-search__vul_cve_search(cve_id)。
- 已知厂商与产品 → 在组件指纹出现后最多 3 次非控制面工具调用内调用 cve-search__vul_vendor_product_cve(vendor, product)，再按 {V}、受影响范围和修复版本过滤结果；本地验证可与查询并行。
- 用户要求近期/最新 CVE → 调用 cve-search__vul_last_cves(number)，number 默认 5，按任务需要设置有界数量。
- 厂商或产品规范名不确定 → 调用 cve-search__vul_vendor_products(vendor)；数据库新鲜度可疑 → 调用 cve-search__vul_db_update_status()。cve-search__vul_vendors() 返回量大，只在明确需要厂商目录时调用。
- MCP 返回的数据是 observed 情报；只有版本范围匹配且完成目标侧验证后才能标为 confirmed。
- MCP 报错、超时、空结果、缺少目标工具，或需要独立交叉验证时，执行下面的本地与公开渠道降级路径。

🔴根组件与组件家族必须同批检索：

1. 以根组件为第一个查询对象。
2. 从安装目录、插件清单、API/路由、进程、依赖文件和可信生态映射生成最多 3 个家族候选，覆盖管理器、官方插件、扩展和紧耦合子项目。
3. 根组件与家族候选的 vendor/product 查询可并行。例如识别 `ComfyUI` 后，必须同时把 `ComfyUI-Manager` 作为候选查询。
4. 每个候选记录 `family_of/addon_of` 关系和 `candidate|observed_installed` 状态；没有目标侧安装证据时，其漏洞不得标为适用或 confirmed。
5. 根组件及有界家族查询完成后置 `cve_lookup_state=queried`；MCP 与降级渠道均失败时置 `blocked` 并记录失败签名；无可识别组件时置 `not_applicable`。

以下阶段按证据缺口执行，不做无目的全量重复查询：

1.CVE漏洞库(必做,找已知漏洞):
  MCP: 按上面的 ID / vendor+product / recent 路由调用 cve-search
  terminal: searchsploit {C} {V}
  terminal: curl -s "https://cve.circl.lu/api/search/{C}/{V}" | python3 -c "import sys,json;[print(x['id'],x.get('summary','')[:80]) for x in json.load(sys.stdin)[:10]]"
  browser_navigate: https://github.com/advisories?query={C}+{V}
  browser_navigate: https://www.cvedetails.com/google-search-results.php?q={C}+{V}&sa=Search

2.搜索引擎(至少执行3个,找漏洞分析+PoC):
  browser_navigate: https://www.google.com/search?q={C}+{V}+exploit+PoC+RCE+site:github.com
  browser_navigate: https://www.google.com/search?q={C}+{V}+漏洞+利用+复现
  browser_navigate: https://www.baidu.com/s?wd={C}+{V}+漏洞+利用+poc+getshell
  browser_navigate: https://www.bing.com/search?q={C}+{V}+CVE+exploit+poc
  browser_navigate: https://duckduckgo.com/?q={C}+{V}+vulnerability+exploit

3.中文安全社区(必做,中文首发多且深度分析好):
  browser_navigate: https://xz.aliyun.com/search?keyword={C}+漏洞
  browser_navigate: https://www.seebug.org/search/?keywords={C}
  browser_navigate: https://paper.seebug.org/search/?keyword={C}
  browser_navigate: https://www.freebuf.com/search?search={C}+{V}
  browser_navigate: https://ti.qianxin.com/vulnerability?keyword={C}
  browser_navigate: https://www.anquanke.com/search?s={C}

4.GitHub搜PoC/exploit代码(必做,最直接拿利用代码):
  terminal: curl -s "https://api.github.com/search/repositories?q={C}+{V}+exploit+OR+poc+OR+CVE&sort=updated&per_page=10" | python3 -c "import sys,json;d=json.load(sys.stdin);[print(x['full_name'],x['html_url'],x.get('description','')[:60]) for x in d.get('items',[])]"
  terminal: curl -s "https://api.github.com/search/code?q={C}+RCE+OR+shell+OR+exploit+language:python&per_page=5" | python3 -c "import sys,json;d=json.load(sys.stdin);[print(x['html_url']) for x in d.get('items',[])]"
  terminal: curl -s "https://api.github.com/search/repositories?q={C}+CVE&sort=stars&per_page=5" | python3 -c "import sys,json;d=json.load(sys.stdin);[print(x['full_name'],x['stargazers_count'],'★',x.get('description','')[:50]) for x in d.get('items',[])]"
  找到仓库后: curl -s "https://api.github.com/repos/{owner}/{repo}/readme" | python3 -c "import sys,json,base64;print(base64.b64decode(json.load(sys.stdin)['content']).decode())"

5.资产引擎(找同类目标/暴露面):
  browser_navigate: https://fofa.info/result?qbase64=$(echo -n 'app="{C}"' | base64)
  browser_navigate: https://www.shodan.io/search?query={C}+{V}
  browser_navigate: https://www.zoomeye.org/searchResult?q={C}
  browser_navigate: https://search.censys.io/search?resource=hosts&q=services.software.product:{C}

6.即时情报(最新0day/在野利用):
  browser_navigate: https://x.com/search?q={C}+CVE+OR+0day+OR+exploit&f=live
  browser_navigate: https://www.reddit.com/r/netsec/search/?q={C}&sort=new&t=month
  browser_navigate: https://www.exploit-db.com/search?q={C}

7.扩展链(有界): 搜完{C}后提取依赖清单(package.json/pom.xml/requirements.txt/go.mod)，按可达性和影响选择最多3个家族/依赖候选重复1-6

🔴搜索受阻处理序列(碰到403/验证码/空结果/超时→按序执行不放弃):
  ①换UA: curl -H "User-Agent: Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)" "{URL}"
  ②Jina读取器: browser_navigate: https://r.jina.ai/{原始URL}
  ③Google缓存: browser_navigate: https://webcache.googleusercontent.com/search?q=cache:{域名}+{关键词}
  ④Archive: browser_navigate: https://web.archive.org/web/{URL}
  ⑤GitHub API替代(GitHub页面拦但API不拦): 用上面第4步的curl命令
  ⑥换引擎: Google拦→执行Bing/DuckDuckGo/百度; 百度拦→执行Google/Bing
  ⑦走代理: 按 `proxy-tool-bootstrap` 序列获取SOCKS5代理后重试
  全部受阻仍无结果→写负Fact"已搜{C} {V}全渠道无公开漏洞"→转 `zero-day-discovery`
```
