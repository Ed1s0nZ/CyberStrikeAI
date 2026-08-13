# MCP v1.1.1 工具索引 + v0.9.0 迁移历史

> **说明**：当前可调用工具以 SKILL.md 核心层和 MCP 工具自身 docstring 为准；本文后半部分保留 v0.8.x → v0.9.0 的历史迁移映射。
>
> **版本**：v3.4.1（MCP v1.1.1）
>
> **当前 MCP 版本**：camoufox-reverse MCP v1.1.1（35 个工具）。v1.1.0 固定 `mcp>=1.29,<2` 并仅归一化安全的顶层可选参数 schema；v1.1.1 修复链式调用的重叠 AST 改写，并在插桩状态中暴露实际执行模式。

---

## 一、v0.9.0 历史工具分类表

> 本节用于理解旧文档和迁移来源，包含后续已移除的 Session/Assertion 工具，不应作为当前工具清单。当前清单见 SKILL.md「核心武器」。

### Browser — 浏览器与页面控制

| 工具名 | 一行描述 |
|--------|---------|
| `launch_browser` | 启动 Camoufox 反检测浏览器（C++ 引擎级指纹伪装） |
| `close_browser` | 关闭浏览器释放资源 |
| `navigate` | 导航到 URL，支持 pre_inject_hooks / wait_until / collect_response_chain |
| `reload` | 刷新当前页面 |
| `go_back` | 浏览器后退 |

### Page — 页面交互与信息

| 工具名 | 一行描述 |
|--------|---------|
| `click` | 点击页面元素 |
| `type_text` | 输入文本（带真实按键延迟） |
| `wait_for` | 等待元素出现或网络请求匹配 |
| `get_page_info` | 获取当前页面 URL、标题、视口尺寸 |
| `take_screenshot` | 截取页面截图 |
| `take_snapshot` | 获取页面无障碍树（token 高效） |
| `evaluate_js` | 在页面上下文执行任意 JS（最核心工具） |
| `evaluate_js_handle` | 执行 JS 并检查复杂对象属性 |
| `get_page_html` | 获取页面 HTML 或指定元素的 outerHTML |

### Network — 网络分析

| 工具名（v0.9.0） | 一行描述 |
|-------------------|---------|
| `network_capture(action='start'\|'stop')` <!-- v3.1.0: migrated from start_network_capture/stop_network_capture --> | 启停网络流量捕获，支持 capture_body=True |
| `list_network_requests` | 列出捕获的请求（支持 URL/method/type/status 过滤） |
| `get_network_request` | 获取请求完整详情（含响应体） |
| `get_request_initiator` | 获取发起请求的 JS 调用栈（黄金路径） |
| `intercept_request` | 拦截请求（log/block/modify/mock） |
| `stop_intercept` | 停止拦截 |
| `search_response_body` | 在所有已捕获响应体中全文搜索 |
| `get_response_body_page` | 分页读取大响应体 |
| `search_json_path` | 按 JSON 路径提取响应数据 |

### Scripts — 源码分析

| 工具名（v0.9.0） | 一行描述 |
|-------------------|---------|
| `scripts(action='list')` <!-- v3.1.0: migrated from list_scripts --> | 列出页面加载的所有 JS 脚本 |
| `scripts(action='get', url=...)` <!-- v3.1.0: migrated from get_script_source --> | 获取完整 JS 源码 |
| `scripts(action='save', url=..., save_path=...)` <!-- v3.1.0: migrated from save_script --> | 保存 JS 文件到本地 |
| `search_code` | 在所有已加载 JS 中搜索关键词（全局） |
| `search_code(keyword, script_url=url)` <!-- v3.1.0: migrated from search_code_in_script --> | 在指定脚本中搜索（大文件精确定位） |

### Hook — 调试与 Hook

| 工具名（v0.9.0） | 一行描述 |
|-------------------|---------|
| `add_init_script` | 注入页面加载前执行的脚本（支持 persistent） |
| `set_breakpoint_via_hook` | 通过 JS Hook 设置伪断点 |
| `get_breakpoint_data` | 获取伪断点捕获的数据 |
| `get_console_logs` | 获取页面控制台输出 |
| `hook_function(path, mode='trace', ...)` <!-- v3.1.0: migrated from trace_function --> | 追踪函数调用（不暂停执行） |
| `get_trace_data` | 获取追踪数据 |
| `hook_function(path, hook_code=..., position=...)` | 注入自定义 Hook（before/after/replace） |
| `hook_function(path, mode='intercept', non_overridable=True)` <!-- v3.1.0: migrated from freeze_prototype --> | 冻结原型方法防覆盖 |
| `inject_hook_preset` | 一键预设 Hook（xhr/fetch/crypto/websocket/debugger_bypass/cookie/runtime_probe） |
| `get_runtime_probe_log` | 拉取 runtime_probe 日志 |
| `remove_hooks` | 移除所有 Hook |
| `bypass_debugger_trap` | 一键绕过反调试陷阱 |

### JSVMP — 虚拟机专项分析

| 工具名（v0.9.0） | 一行描述 |
|-------------------|---------|
| `hook_jsvmp_interpreter` | 多路径通用探针（proxy/transparent 模式） |
| `hook_jsvmp_interpreter(mode='proxy', trackProps=True)` <!-- v3.1.0: migrated from trace_property_access --> | Proxy 级别属性访问追踪 |
| `get_jsvmp_log` | 获取 JSVMP 探针日志 |
| `dump_jsvmp_strings` | 提取 JSVMP 脚本中的字符串 |
| `compare_env` | 采集浏览器环境指纹基准 |
| `instrumentation(action='install', url_pattern=..., mode=..., tag=...)` <!-- v3.1.0: migrated from instrument_jsvmp_source --> | 源码级插桩（HTTP 层改写 VMP） |
| `instrumentation(action='log', tag_filter=..., type_filter=...)` <!-- v3.1.0: migrated from get_instrumentation_log --> | 获取源码插桩日志（hot_keys/hot_methods/hot_functions） |
| `instrumentation(action='status')` | 查看当前激活的源码插桩状态 |
| `instrumentation(action='stop', url_pattern=...)` <!-- v3.1.0: migrated from stop_instrumentation --> | 停止源码级插桩 |
| `instrumentation(action='reload')` <!-- v3.1.0: migrated from reload_with_hooks --> | 重载页面让 hooks 先于 VMP 生效 |
| `analyze_cookie_sources` | Cookie 归因分析（HTTP vs JS 来源） |

### Session — 域级 Session 档案

| 工具名（v0.9.0） | 一行描述 |
|-------------------|---------|
| `start_reverse_session` | 启动新的分析 run |
| `stop_reverse_session` | 结束当前 run |
| `get_session_snapshot` | 获取活跃 session 快照 |
| `list_sessions` | 列出所有域 session 档案 |
| `attach_domain_readonly` | 只读附加已有域档案 |
| `export_session` | 导出域档案为 zip |
| `import_session` | 导入域档案 zip |

### Assertion — 断言系统

| 工具名（v0.9.0） | 一行描述 |
|-------------------|---------|
| `add_assertion` | 注册域级断言 |
| `verify_assertion(assertion_id=...)` | 验证单条断言 |
| `verify_assertion(assertion_id='*')` <!-- v3.1.0: migrated from reverify_all_assertions_on_domain --> | 批量验证所有断言 |
| `list_assertions` | 列出域上的所有断言 |
| `remove_assertion` | 软删除断言 |

### Environment — 存储与指纹

| 工具名（v0.9.0） | 一行描述 |
|-------------------|---------|
| `cookies(action='get', ...)` <!-- v3.1.0: migrated from get_cookies --> | 获取 Cookie |
| `cookies(action='set', ...)` <!-- v3.1.0: migrated from set_cookies --> | 设置 Cookie |
| `delete_cookies` | 删除 Cookie |
| `get_storage` | 获取 localStorage/sessionStorage |
| `set_storage` | 设置 storage |
| `export_state` | 导出浏览器状态（Cookie + Storage） |
| `import_state` | 导入浏览器状态 |
| `get_fingerprint_info` | 检查当前浏览器指纹 |
| `check_detection` | 在反检测站点测试是否被识别 |
| `get_session_info` | 查看当前会话状态全貌 |

---

## 二、v0.8.x → v0.9.0 完整迁移表

> MCP v0.9.0 将 ~80 个工具合并为 ~50 个，核心变化是将同类操作合并为带 `action` 参数的统一接口。

### 19 个工具合并为 7 个统一接口

| v0.8.x 旧名 | v0.9.0 新名 | 说明 |
|-------------|-------------|------|
| `start_network_capture(capture_body=X)` | `network_capture(action='start', capture_body=X)` | 网络捕获启停合并 |
| `stop_network_capture()` | `network_capture(action='stop')` | |
| `search_code_in_script(url, keyword)` | `search_code(keyword, script_url=url)` | 合并到 search_code，script_url 可选参数 |
| `trace_function(path, ...)` | `hook_function(path, mode='trace', ...)` | 追踪合并到 hook_function |
| `instrument_jsvmp_source(url_pattern, ...)` | `instrumentation(action='install', url_pattern=url_pattern, ...)` | 源码插桩统一接口 |
| `get_instrumentation_log(tag=X)` | `instrumentation(action='log', tag_filter=X)` | |
| `get_instrumentation_status()` | `instrumentation(action='status')` | |
| `stop_instrumentation()` | `instrumentation(action='stop')` | |
| `reload_with_hooks()` | `instrumentation(action='reload')` | |
| `get_cookies(...)` | `cookies(action='get', ...)` | Cookie 管理统一接口 |
| `set_cookies(...)` | `cookies(action='set', ...)` | |
| `list_scripts(...)` | `scripts(action='list', ...)` | 脚本管理统一接口 |
| `get_script_source(url)` | `scripts(action='get', url=url)` | |
| `save_script(url, path)` | `scripts(action='save', url=url, save_path=path)` | |
| `reverify_all_assertions_on_domain()` | `verify_assertion(assertion_id='*')` | 批量验证合并到 verify_assertion |
| `find_dispatch_loops(url)` | `search_code(keyword='switch', script_url=url, context_chars=500)` | 合并到 search_code 的高级用法 |
| `trace_property_access(...)` | `hook_jsvmp_interpreter(mode='proxy', trackProps=True)` | 合并到 hook_jsvmp_interpreter |
| `freeze_prototype(class, method)` | `hook_function('class.prototype.method', mode='intercept', ...)` | 合并到 hook_function |
| `get_page_content()` | `evaluate_js("document.documentElement.outerHTML")` | 用 evaluate_js 替代 |

### 已删除工具及替代方案（12 个）

| 已删除工具 | 替代方案 |
|-----------|---------|
| `get_property_access_log` | `get_jsvmp_log(type_filter='prop_read')` |
| `get_page_content` | `evaluate_js("document.documentElement.outerHTML")` <!-- v3.1.0: migrated from get_page_content --> |
| `find_dispatch_loops` | `search_code(keyword='switch', script_url='<url>', context_chars=500)` |
| `trace_property_access` | `hook_jsvmp_interpreter(mode='proxy', trackProps=True)` |
| `freeze_prototype` | `hook_function('X.prototype.Y', mode='intercept', non_overridable=True)` |
| `check_csp_policy` | `evaluate_js("document.querySelector('meta[http-equiv=Content-Security-Policy]')?.content")` |
| `get_response_chain` | `navigate()` 返回值中的 `redirect_chain` 字段 |
| `get_dual_sign_data` | 组合 `list_network_requests` + `get_network_request` 手动提取 |
| `diff_hot_keys` | 本地 JSON diff 或 `evaluate_js` 实现 |
| `create_session` | `start_reverse_session` |
| `load_session` | `attach_domain_readonly` + `verify_assertion(assertion_id='*')` |
| `update_session` | `stop_reverse_session` + `add_assertion` |

---

## 三、6 个月 Shim 兼容期

> MCP v0.9.0 提供 6 个月的 shim 兼容期（至 2027-01-01）。

### Shim 行为

- 旧工具名仍可调用，MCP 内部自动映射到新接口
- 每次调用旧名会在 console 输出 deprecation warning
- Shim 期结束后旧名将返回 `ToolNotFound` 错误

### 建议迁移时间线

| 阶段 | 时间 | 行动 |
|------|------|------|
| 立即 | 现在 | 新代码/新文档全部使用 v0.9.0 名称 |
| 1 个月内 | 2026-08 | 更新所有 cases/*.md 中的工具名 |
| 3 个月内 | 2026-10 | 更新所有 references/*.md 中的工具名 |
| 6 个月内 | 2027-01 | 确认无旧名残留，shim 期结束 |

---

## 四、MCP v0.9.0 关键变化总结

### 工具数量

- v0.8.x：~80 个独立工具
- v0.9.0：~50 个工具（19 个合并为 7 个统一接口 + 12 个删除）

### 设计原则

1. **同类操作合并**：start/stop/get/list 合并为带 `action` 参数的统一接口
2. **功能重叠消除**：`trace_function` 和 `hook_function` 合并（trace 是 hook 的一种 mode）
3. **语义一致性**：`freeze_prototype` 本质是 hook 的 intercept 模式
4. **减少记忆负担**：7 个统一接口覆盖原来 19 个工具的功能

### 不变的核心工具

以下工具在 v0.9.0 中保持不变：

```
launch_browser / close_browser / navigate / reload / go_back
click / type_text / wait_for / get_page_info
take_screenshot / take_snapshot
evaluate_js / evaluate_js_handle
get_page_html / add_init_script
set_breakpoint_via_hook / get_breakpoint_data
get_console_logs / get_trace_data
hook_function (扩展了 mode 参数)
inject_hook_preset / remove_hooks / bypass_debugger_trap
hook_jsvmp_interpreter / get_jsvmp_log / dump_jsvmp_strings
compare_env / analyze_cookie_sources
get_runtime_probe_log
list_network_requests / get_network_request / get_request_initiator
intercept_request / stop_intercept
search_response_body / get_response_body_page / search_json_path
delete_cookies / get_storage / set_storage
export_state / import_state
get_fingerprint_info / check_detection / get_session_info
start_reverse_session / stop_reverse_session / get_session_snapshot
list_sessions / attach_domain_readonly
export_session / import_session
add_assertion / verify_assertion / list_assertions / remove_assertion
```

---

## 五、Session/Assertion 工具移除（v3.2.0 新增）

以下工具在当前 MCP 版本中**不存在**，skill v3.2.0 已移除所有引用：

| 移除的工具 | 替代 |
|---|---|
| `start_reverse_session` / `stop_reverse_session` | 无替代。经验沉淀到 cases/ 文件 |
| `get_session_snapshot` | 无替代。手动查看 list_network_requests / instrumentation(action='log') |
| `list_sessions` / `attach_domain_readonly` | 无替代。读 cases/README.md 查已有案例 |
| `export_session` / `import_session` | 无替代。case 文件本身是可 commit 的经验 |
| `add_assertion` / `verify_assertion` / `list_assertions` / `remove_assertion` | 在 case 文件的"可验证事实清单"段列事实，AI 手动核对 |
| `verify_against_session(signer_code, sample_filter=...)` | `verify_signer_offline(signer_code, samples=[...])` — 样本由用户提供 |

`verify_signer_offline` 迁移示例：

```python
# 步骤 1 — 手动提取样本
reqs = await list_network_requests(url_filter="api/search")
samples = []
for req in reqs["requests"][:10]:
    full = await get_network_request(req["id"])
    url = full["url"]
    xbogus = _parse_url_param(url, "X-Bogus")
    if xbogus:
        samples.append({
            "id": f"req_{req['id']}",
            "input": {"url": _strip_param(url, "X-Bogus")},
            "expected": {"X-Bogus": xbogus},
        })

# 步骤 2 — 验证
r = await verify_signer_offline(
    signer_code="(s) => ({'X-Bogus': mySign(s.input.url)})",
    samples=samples,
    compare_params=["X-Bogus"],
)
```

---

> **SKILL.md 核心层**有工具分类索引的精简版
> **各工具的详细用法**：查看工具自身的 docstring
