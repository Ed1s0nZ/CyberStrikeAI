# 部署指南

本文说明 CyberStrikeAI 的常见部署方式。生产环境部署前，请先阅读 [安全模型](security-model.md)，确认授权范围、认证、HITL、审计和高风险功能开关。

## 部署前准备

基础依赖：

- Go：用于源码运行或构建二进制。
- Python：部分 MCP 服务或工具脚本需要 Python 运行环境。
- SQLite：默认使用文件型数据库，无需单独服务。
- 安全工具：`tools/` 中的 YAML 只是工具定义，实际命令如 `nmap`、`sqlmap`、`nuclei` 仍需安装到系统 PATH。
- 模型服务：至少配置一个 `ai.channels` 通道；`provider: openai_compatible` 适用于 OpenAI 兼容 API，`provider: claude` 使用 Eino 原生 Claude 组件。

建议目录：

```text
CyberStrikeAI-main/
  config.yaml
  data/
  tools/
  roles/
  skills/
  agents/
  knowledge_base/
```

`data/`、`config.yaml`、自定义 `tools/roles/skills/agents/knowledge_base` 是最重要的持久化内容，升级前应备份。

## 快速启动

仓库提供 `run.sh`，适合本地体验和小规模部署：

```bash
chmod +x run.sh && ./run.sh
```

默认配置中 `server.tls_enabled: true` 且 `tls_auto_self_sign: true`，访问地址通常是：

```text
https://127.0.0.1:8080/
```

自签证书会触发浏览器安全提示，这是本地测试的正常现象。生产环境建议配置真实证书。

`run.sh` 是最常用的启动入口，适合：

- 本机体验。
- 开发调试。
- 小团队临时内网使用。
- 升级后快速验证新版是否能启动。

如果需要长期运行、开机自启、日志托管或进程崩溃自动恢复，建议改用 systemd 托管二进制。

## 源码运行

适合开发调试：

```bash
go run ./cmd/server --config config.yaml
```

如果依赖下载较慢，可以先配置 Go 代理：

```bash
go env -w GOPROXY=https://goproxy.cn,direct
```

## 构建二进制

```bash
go build -o cyberstrike-ai ./cmd/server
./cyberstrike-ai --config config.yaml
```

交付二进制时仍需要携带：

- `web/templates/`
- `web/static/`
- `tools/`
- `roles/`
- `skills/`
- `agents/`
- `config.yaml`

## HTTPS

本地测试可以使用自签：

```yaml
server:
  tls_enabled: true
  tls_auto_self_sign: true
```

生产环境建议使用证书文件：

```yaml
server:
  host: 0.0.0.0
  port: 8080
  tls_enabled: true
  tls_cert_path: /etc/letsencrypt/live/example.com/fullchain.pem
  tls_key_path: /etc/letsencrypt/live/example.com/privkey.pem
```

启用 TLS 后，同端口 HTTP 请求默认会 308 跳转到 HTTPS。若前面有反向代理负责 TLS，可以关闭应用内 TLS，在代理层处理 HTTPS。

## 反向代理

Nginx 示例：

```nginx
server {
    listen 443 ssl http2;
    server_name cyberstrike.example.com;

    ssl_certificate /etc/letsencrypt/live/cyberstrike.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/cyberstrike.example.com/privkey.pem;

    client_max_body_size 200m;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_buffering off;
    }
}
```

`proxy_buffering off` 对 SSE 流式输出和 WebSocket 终端更友好。

## systemd

示例服务：

```ini
[Unit]
Description=CyberStrikeAI
After=network.target

[Service]
Type=simple
WorkingDirectory=/opt/CyberStrikeAI
ExecStart=/opt/CyberStrikeAI/cyberstrike-ai --config /opt/CyberStrikeAI/config.yaml
Restart=on-failure
RestartSec=5
Environment=GIN_MODE=release

[Install]
WantedBy=multi-user.target
```

启用：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now cyberstrikeai
sudo journalctl -u cyberstrikeai -f
```

## 数据与备份

重点备份：

- `config.yaml`
- `data/conversations.db`
- `data/knowledge.db`
- `data/eino-checkpoints/`
- 自定义 `tools/roles/skills/agents/knowledge_base`
- 上传文件目录 `chat_uploads/`

SQLite 热备份时最好先停止服务，或至少复制 `*.db`、`*.db-wal`、`*.db-shm` 三类文件。

## 升级

推荐流程：

1. 停止服务。
2. 备份 `config.yaml`、`data/` 和自定义目录。
3. 拉取或替换新版代码/二进制。
4. 保留原配置，按新版 `config.yaml` 示例补新增字段。
5. 启动服务，检查登录、模型测试、工具列表、知识库状态。

仓库提供 `upgrade.sh`，适合无兼容性问题的快速升级；生产环境仍建议先备份再运行。

## 回滚

回滚时同时恢复：

- 上一版本二进制或代码。
- 升级前的 `config.yaml`。
- 升级前的 `data/`。

如果新版已经写入数据库结构变更，单独回滚二进制可能不够，建议整体恢复备份。

## 生产部署决策表

| 场景 | 推荐部署 | 关键配置 | 不建议 |
| --- | --- | --- | --- |
| 单人本机测试 | `./run.sh` + 自签 HTTPS | `tls_auto_self_sign: true` | 暴露公网 |
| 小团队内网 | 二进制 + systemd + 内网 HTTPS | 强密码、审计、备份、限制来源 IP | 所有人共用弱密码 |
| 生产红队平台 | 反向代理 + 独立运行用户 + 日志采集 | 真实证书、反向代理认证、C2 按需启用 | Web 管理面直连公网 |
| 只做知识库/对话 | 关闭 C2，禁用不需要的外部 MCP | `c2.enabled: false` | 默认开启所有高风险模块 |
| 多工具自动化 | 独立工作目录 + HITL + 工具白名单 | `workspace_root_dir`、`hitl`、`monitor` | 让 Agent 拥有全局 Shell 权限且免审批 |

## 运行时文件分层

部署时最容易出问题的是“代码、配置、运行数据混在一起”。建议按下面方式理解：

- 可替换：二进制、`web/`、默认 `tools/roles/skills/agents/docs`。
- 必须保留：`config.yaml`、`data/`、自定义工具/角色/技能/子代理、`knowledge_base/`、`chat_uploads/`。
- 可清理但要谨慎：`data/eino-checkpoints/`、临时 workspace、旧 payload、旧工具执行记录。

升级时如果覆盖整个目录，应先把自定义目录和 `data/` 移出或备份。很多“升级后配置丢了”的问题，本质是把运行态文件当成发布包的一部分覆盖掉。

## 启动后验收清单

启动成功不代表可用，至少做下面检查：

1. 打开 `/`，确认 HTTPS/反向代理没有跳转循环。
2. 登录后访问 `/api/auth/validate`，确认会话可用。
3. 系统设置中执行模型测试。
4. 打开工具列表，确认 `tools/` 被加载，核心工具 schema 正常。
5. 若启用知识库，访问知识库页，确认 `index-status` 正常。
6. 若启用外部 MCP，查看外部 MCP 状态和工具是否出现在对话侧。
7. 若启用 C2，只在授权网络启动一个测试 listener，并确认停止/删除正常。
8. 查看审计页面，确认登录和配置读取有记录。

## 反向代理容易踩的坑

- SSE 被缓冲：表现为 Agent 一直不输出，结束时一次性吐出。关闭 `proxy_buffering`。
- WebSocket 失败：终端或事件流异常。检查 `Upgrade` 和 `Connection` 头。
- HTTPS 混用：应用内 TLS 和 Nginx TLS 同时启用时，`proxy_pass` 协议必须匹配。
- 上传失败：调大 `client_max_body_size`，并检查应用侧上传限制。
- 308 循环：如果应用启用同端口 HTTPS 跳转，而代理又用 HTTP 回源，需要关闭应用 TLS 或改代理回源 HTTPS。

## 源码锚点

- 服务组装和路由：`internal/app/app.go`
- HTTPS 和自签证书：`internal/app/main_server_tls.go`
- HTTP 到 HTTPS 跳转：`internal/app/main_server_http_redirect.go`
- 配置结构和默认值：`internal/config/config.go`
- 配置应用逻辑：`internal/handler/config.go`
