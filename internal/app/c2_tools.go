package app

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"cyberstrike-ai/internal/agent"
	"cyberstrike-ai/internal/authctx"
	"cyberstrike-ai/internal/c2"
	"cyberstrike-ai/internal/database"
	"cyberstrike-ai/internal/mcp"
	"cyberstrike-ai/internal/mcp/builtin"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// registerC2Tools 注册所有 C2 MCP 工具（合并同类项，减少工具数量以节省上下文 token）。
// webListenPort 为本进程 Web/API 监听端口（配置 server.port，启动时已加载），用于 MCP 描述中提示勿与 C2 bind_port 冲突。
func registerC2Tools(mcpServer *mcp.Server, c2Manager *c2.Manager, logger *zap.Logger, webListenPort int) {
	registerC2ListenerTool(mcpServer, c2Manager, logger, webListenPort)
	registerC2SessionTool(mcpServer, c2Manager, logger)
	registerC2TaskTool(mcpServer, c2Manager, logger)
	registerC2TaskManageTool(mcpServer, c2Manager, logger)
	registerC2PayloadTool(mcpServer, c2Manager, logger, webListenPort)
	registerC2EventTool(mcpServer, c2Manager, logger)
	registerC2ProfileTool(mcpServer, c2Manager, logger)
	registerC2FileTool(mcpServer, c2Manager, logger)
	logger.Debug("C2 MCP tools registered (8 unified tools)")
}

func makeC2Result(data interface{}, err error) (*mcp.ToolResult, error) {
	if err != nil {
		return &mcp.ToolResult{
			Content: []mcp.Content{{Type: "text", Text: err.Error()}},
			IsError: true,
		}, nil
	}
	text, _ := json.Marshal(data)
	return &mcp.ToolResult{
		Content: []mcp.Content{{Type: "text", Text: string(text)}},
	}, nil
}

// ============================================================================
// c2_listener — 监听器统一工具
// ============================================================================

func registerC2ListenerTool(s *mcp.Server, m *c2.Manager, l *zap.Logger, webListenPort int) {
	s.RegisterTool(mcp.Tool{
		Name: builtin.ToolC2Listener,
		Description: fmt.Sprintf(`C2 监听器管理。通过 action 参数选择操作：
- list: 列出所有监听器
- get: 获取监听器详情（需 listener_id）
- create: 创建监听器（需 name, type, bind_port）。成功时除 listener 外会返回 implant_token（仅此一次，用于 X-Implant-Token / oneliner；list/get/start 不再返回）
- update: 更新监听器配置（需 listener_id，可改 name/bind_host/bind_port/remark/config/callback_host）
- start: 启动监听器（需 listener_id）
- stop: 停止监听器（需 listener_id）
- delete: 删除监听器（需 listener_id）
监听器类型: tcp_reverse, http_beacon, https_beacon, websocket
tcp_reverse 默认仅接受 CSB1 加密 Beacon（AES-GCM + ImplantToken）才登记会话；经典 bash/nc 反弹需在 config.allow_legacy_shell=true（公网不推荐）。
端口约束：create/update 的 bind_port 禁止与本平台 Web/API 所用端口相同。当前本服务该端口为 %d（配置项 server.port，随进程启动从配置文件加载）。若 bind_port 与此相同会导致本服务或监听器 bind 失败、Beacon/oneliner 误连到 Web 而非 C2。请为监听器另选空闲端口。`, webListenPort),
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action":        map[string]interface{}{"type": "string", "description": "操作: list/get/create/update/start/stop/delete", "enum": []string{"list", "get", "create", "update", "start", "stop", "delete"}},
				"listener_id":   map[string]interface{}{"type": "string", "description": "监听器 ID（get/update/start/stop/delete 需要）"},
				"name":          map[string]interface{}{"type": "string", "description": "监听器名称（create/update）"},
				"type":          map[string]interface{}{"type": "string", "description": "监听器类型（create）", "enum": []string{"tcp_reverse", "http_beacon", "https_beacon", "websocket"}},
				"bind_host":     map[string]interface{}{"type": "string", "description": "绑定地址，默认 127.0.0.1；外网监听常用 0.0.0.0"},
				"callback_host": map[string]interface{}{"type": "string", "description": "可选：植入端/Payload 回连主机名（公网 IP 或域名）。写入 config_json；生成 oneliner/beacon 时优先于 bind_host。update 时传入空字符串可清除"},
				"bind_port":     map[string]interface{}{"type": "integer", "description": fmt.Sprintf("绑定端口（create 必填）。须 ≠ %d（当前本服务 Web/API 端口，配置 server.port）", webListenPort), "minimum": 1, "maximum": 65535},
				"project_id":    map[string]interface{}{"type": "string", "description": "所属项目 ID。create 省略时默认使用当前对话绑定项目；未绑定项目的对话则创建未绑定监听器"},
				"profile_id":    map[string]interface{}{"type": "string", "description": "Malleable Profile ID"},
				"remark":        map[string]interface{}{"type": "string", "description": "备注"},
				"config":        map[string]interface{}{"type": "object", "description": "高级配置（beacon 路径/TLS/OPSEC 等），create/update 可用。tcp_reverse 可选 allow_legacy_shell:true 允许未加密经典 shell（默认 false）"},
			},
			"required": []string{"action"},
		},
	}, func(ctx context.Context, params map[string]interface{}) (*mcp.ToolResult, error) {
		action := getString(params, "action")
		id := getString(params, "listener_id")

		switch action {
		case "list":
			listeners, err := m.DB().ListC2ListenersForAccess(c2ToolAccess(ctx), mcpEffectiveProjectFilter(ctx, m.DB()))
			if err != nil {
				return makeC2Result(nil, err)
			}
			for _, li := range listeners {
				li.EncryptionKey = ""
				li.ImplantToken = ""
			}
			return makeC2Result(map[string]interface{}{"listeners": listeners, "count": len(listeners)}, nil)

		case "get":
			listener, err := m.DB().GetC2Listener(id)
			if err != nil {
				return makeC2Result(nil, err)
			}
			if listener == nil {
				return makeC2Result(nil, fmt.Errorf("listener not found"))
			}
			listener.EncryptionKey = ""
			listener.ImplantToken = ""
			return makeC2Result(map[string]interface{}{"listener": listener}, nil)

		case "create":
			var cfg *c2.ListenerConfig
			if cfgRaw, ok := params["config"]; ok && cfgRaw != nil {
				cfgBytes, _ := json.Marshal(cfgRaw)
				cfg = &c2.ListenerConfig{}
				_ = json.Unmarshal(cfgBytes, cfg)
			}
			projectID := strings.TrimSpace(getString(params, "project_id"))
			if projectID == "" {
				projectID = mcpEffectiveProjectFilter(ctx, m.DB())
				if projectID == database.ProjectFilterUnbound {
					projectID = ""
				}
			}
			input := c2.CreateListenerInput{
				Name:         getString(params, "name"),
				Type:         getString(params, "type"),
				BindHost:     getString(params, "bind_host"),
				BindPort:     int(getFloat64(params, "bind_port")),
				ProfileID:    getString(params, "profile_id"),
				Remark:       getString(params, "remark"),
				ProjectID:    projectID,
				Config:       cfg,
				CallbackHost: getString(params, "callback_host"),
			}
			listener, err := m.CreateListener(input)
			if err != nil {
				return makeC2Result(nil, err)
			}
			if principal, ok := authctx.PrincipalFromContext(ctx); ok {
				_ = m.DB().SetResourceOwner("c2_listener", listener.ID, principal.UserID)
				_ = m.DB().AssignResourceToUser(principal.UserID, "c2_listener", listener.ID)
			}
			implantToken := listener.ImplantToken
			listener.EncryptionKey = ""
			listener.ImplantToken = ""
			return makeC2Result(map[string]interface{}{
				"listener":      listener,
				"implant_token": implantToken,
			}, nil)

		case "update":
			listener, err := m.DB().GetC2Listener(id)
			if err != nil {
				return makeC2Result(nil, err)
			}
			if listener == nil {
				return makeC2Result(nil, fmt.Errorf("listener not found"))
			}
			if m.IsListenerRunning(id) {
				newHost := getString(params, "bind_host")
				newPort := int(getFloat64(params, "bind_port"))
				if (newHost != "" && newHost != listener.BindHost) || (newPort > 0 && newPort != listener.BindPort) {
					return makeC2Result(nil, fmt.Errorf("cannot modify bind address while listener is running"))
				}
			}
			if v := getString(params, "name"); v != "" {
				listener.Name = v
			}
			if v := getString(params, "bind_host"); v != "" {
				listener.BindHost = v
			}
			if v := int(getFloat64(params, "bind_port")); v > 0 {
				listener.BindPort = v
			}
			if v := getString(params, "profile_id"); v != "" {
				listener.ProfileID = v
			}
			if v, ok := params["remark"]; ok {
				listener.Remark, _ = v.(string)
			}
			if cfgRaw, ok := params["config"]; ok && cfgRaw != nil {
				cfgBytes, _ := json.Marshal(cfgRaw)
				listener.ConfigJSON = string(cfgBytes)
			}
			if _, ok := params["callback_host"]; ok {
				pcfg := &c2.ListenerConfig{}
				raw := strings.TrimSpace(listener.ConfigJSON)
				if raw == "" {
					raw = "{}"
				}
				_ = json.Unmarshal([]byte(raw), pcfg)
				pcfg.CallbackHost = strings.TrimSpace(getString(params, "callback_host"))
				pcfg.ApplyDefaults()
				cfgBytes, err := json.Marshal(pcfg)
				if err != nil {
					return makeC2Result(nil, err)
				}
				listener.ConfigJSON = string(cfgBytes)
			}
			if err := m.DB().UpdateC2Listener(listener); err != nil {
				return makeC2Result(nil, err)
			}
			listener.EncryptionKey = ""
			listener.ImplantToken = ""
			return makeC2Result(map[string]interface{}{"listener": listener}, nil)

		case "start":
			listener, err := m.StartListener(id)
			if err != nil {
				return makeC2Result(nil, err)
			}
			listener.EncryptionKey = ""
			listener.ImplantToken = ""
			return makeC2Result(map[string]interface{}{"listener": listener}, nil)

		case "stop":
			err := m.StopListener(id)
			return makeC2Result(map[string]interface{}{"stopped": err == nil}, err)

		case "delete":
			err := m.DeleteListener(id)
			return makeC2Result(map[string]interface{}{"deleted": err == nil}, err)

		default:
			return makeC2Result(nil, fmt.Errorf("unknown action: %s", action))
		}
	})
}

// ============================================================================
// c2_session — 会话统一工具
// ============================================================================

func registerC2SessionTool(s *mcp.Server, m *c2.Manager, l *zap.Logger) {
	s.RegisterTool(mcp.Tool{
		Name: builtin.ToolC2Session,
		Description: `C2 会话管理。通过 action 参数选择操作：
- list: 列出会话（可按 listener_id/status/os/search/suspicious 过滤）
- get: 获取会话详情及最近任务历史（需 session_id）
- set_sleep: 设置心跳间隔（需 session_id）
- kill: 下发 exit 任务让 implant 退出（需 session_id）
- delete: 删除单个会话记录（需 session_id）
- delete_batch: 批量删除会话（需 session_ids 数组）`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action":         map[string]interface{}{"type": "string", "description": "操作: list/get/set_sleep/kill/delete/delete_batch", "enum": []string{"list", "get", "set_sleep", "kill", "delete", "delete_batch"}},
				"session_id":     map[string]interface{}{"type": "string", "description": "会话 ID（get/set_sleep/kill/delete 需要）"},
				"session_ids":    map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "会话 ID 列表（delete_batch）"},
				"listener_id":    map[string]interface{}{"type": "string", "description": "按监听器过滤（list）"},
				"status":         map[string]interface{}{"type": "string", "description": "按状态过滤: active/sleeping/dead/killed（list）"},
				"os":             map[string]interface{}{"type": "string", "description": "按 OS 过滤: linux/windows/darwin（list）"},
				"search":         map[string]interface{}{"type": "string", "description": "模糊搜索 hostname/username/IP（list）"},
				"suspicious":     map[string]interface{}{"type": "boolean", "description": "仅疑似误报：离线且 tcp_* / unknown / PID 0（list）"},
				"limit":          map[string]interface{}{"type": "integer", "description": "返回数量上限（list）"},
				"sleep_seconds":  map[string]interface{}{"type": "integer", "description": "心跳间隔秒数（set_sleep）"},
				"jitter_percent": map[string]interface{}{"type": "integer", "description": "抖动百分比 0-100（set_sleep）"},
			},
			"required": []string{"action"},
		},
	}, func(ctx context.Context, params map[string]interface{}) (*mcp.ToolResult, error) {
		action := getString(params, "action")
		id := getString(params, "session_id")

		switch action {
		case "list":
			filter := database.ListC2SessionsFilter{
				ListenerID: getString(params, "listener_id"),
				ProjectID:  mcpEffectiveProjectFilter(ctx, m.DB()),
				Status:     getString(params, "status"),
				OS:         getString(params, "os"),
				Search:     getString(params, "search"),
			}
			if limit := int(getFloat64(params, "limit")); limit > 0 {
				filter.Limit = limit
			}
			if v, ok := params["suspicious"].(bool); ok && v {
				filter.Suspicious = true
			}
			sessions, err := m.DB().ListC2SessionsForAccess(filter, c2ToolAccess(ctx))
			return makeC2Result(map[string]interface{}{"sessions": sessions, "count": len(sessions)}, err)

		case "get":
			session, err := m.DB().GetC2Session(id)
			if err != nil {
				return makeC2Result(nil, err)
			}
			if session == nil {
				return makeC2Result(nil, fmt.Errorf("session not found"))
			}
			tasks, _ := m.DB().ListC2Tasks(database.ListC2TasksFilter{SessionID: id, Limit: 10})
			return makeC2Result(map[string]interface{}{"session": session, "tasks": tasks}, nil)

		case "set_sleep":
			sleep := int(getFloat64(params, "sleep_seconds"))
			jitter := int(getFloat64(params, "jitter_percent"))
			task, err := m.SetSessionSleep(id, sleep, jitter)
			out := map[string]interface{}{
				"updated":        err == nil,
				"sleep_seconds":  sleep,
				"jitter_percent": jitter,
			}
			if task != nil {
				out["task_id"] = task.ID
			}
			return makeC2Result(out, err)

		case "kill":
			task, err := m.EnqueueTask(c2.EnqueueTaskInput{
				SessionID:      id,
				TaskType:       c2.TaskTypeExit,
				Payload:        map[string]interface{}{},
				Source:         "ai",
				ConversationID: agent.ConversationIDFromContext(ctx),
				UserCtx:        ctx,
			})
			return makeC2Result(map[string]interface{}{"task": task}, err)

		case "delete":
			err := m.DB().DeleteC2Session(id)
			return makeC2Result(map[string]interface{}{"deleted": err == nil}, err)

		case "delete_batch":
			rawIDs, _ := params["session_ids"].([]interface{})
			ids := make([]string, 0, len(rawIDs))
			for _, v := range rawIDs {
				if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
					ids = append(ids, strings.TrimSpace(s))
				}
			}
			n, err := m.DB().DeleteC2SessionsByIDs(ids)
			return makeC2Result(map[string]interface{}{"deleted": n}, err)

		default:
			return makeC2Result(nil, fmt.Errorf("unknown action: %s", action))
		}
	})
}

// ============================================================================
// c2_task — 任务下发统一工具（合并所有 task 类型）
// ============================================================================

func registerC2TaskTool(s *mcp.Server, m *c2.Manager, l *zap.Logger) {
	s.RegisterTool(mcp.Tool{
		Name: builtin.ToolC2Task,
		Description: `在 C2 会话上下发任务。所有任务类型通过 task_type 参数指定：
- exec: 执行命令（需 command）
- shell: 交互式命令，保持 cwd（需 command）
- pwd/ps/screenshot/socks_stop: 无额外参数
- cd/ls: 需 path
- kill_proc: 需 pid
- upload: 需 remote_path + file_id
- download: 需 remote_path
- port_fwd: 需 action(start/stop) + local_port + remote_host + remote_port
- socks_start: 需 port（默认 1080）
- load_assembly: 需 data(base64) 或 file_id，可选 args
- persist: 可选 method(auto/cron/bashrc/launchagent/registry/schtasks)
返回 task_id，用 c2_task_manage 的 wait/get_result 获取结果。`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"session_id":      map[string]interface{}{"type": "string", "description": "C2 会话 ID（s_xxx）"},
				"task_type":       map[string]interface{}{"type": "string", "description": "任务类型", "enum": []string{"exec", "shell", "pwd", "cd", "ls", "ps", "kill_proc", "upload", "download", "screenshot", "port_fwd", "socks_start", "socks_stop", "load_assembly", "persist"}},
				"command":         map[string]interface{}{"type": "string", "description": "命令（exec/shell）"},
				"path":            map[string]interface{}{"type": "string", "description": "路径（cd/ls）"},
				"pid":             map[string]interface{}{"type": "integer", "description": "进程 ID（kill_proc）"},
				"remote_path":     map[string]interface{}{"type": "string", "description": "远程路径（upload/download）"},
				"file_id":         map[string]interface{}{"type": "string", "description": "服务端文件 ID（upload/load_assembly）"},
				"data":            map[string]interface{}{"type": "string", "description": "base64 数据（load_assembly）"},
				"args":            map[string]interface{}{"type": "string", "description": "命令行参数（load_assembly）"},
				"action":          map[string]interface{}{"type": "string", "description": "start/stop（port_fwd）"},
				"local_port":      map[string]interface{}{"type": "integer", "description": "本地端口（port_fwd）"},
				"remote_host":     map[string]interface{}{"type": "string", "description": "远程主机（port_fwd）"},
				"remote_port":     map[string]interface{}{"type": "integer", "description": "远程端口（port_fwd）"},
				"port":            map[string]interface{}{"type": "integer", "description": "SOCKS5 端口（socks_start），默认 1080"},
				"method":          map[string]interface{}{"type": "string", "description": "持久化方法（persist）: auto/cron/bashrc/launchagent/registry/schtasks"},
				"timeout_seconds": map[string]interface{}{"type": "integer", "description": "超时秒数，默认 60"},
			},
			"required": []string{"session_id", "task_type"},
		},
	}, func(ctx context.Context, params map[string]interface{}) (*mcp.ToolResult, error) {
		sessionID := getString(params, "session_id")
		taskTypeStr := getString(params, "task_type")
		taskType := c2.TaskType(taskTypeStr)
		timeout := getFloat64(params, "timeout_seconds")

		payload := map[string]interface{}{"timeout_seconds": timeout}

		switch taskType {
		case c2.TaskTypeExec, c2.TaskTypeShell:
			payload["command"] = getString(params, "command")
		case c2.TaskTypeCd, c2.TaskTypeLs:
			payload["path"] = getString(params, "path")
		case c2.TaskTypeKillProc:
			payload["pid"] = params["pid"]
		case c2.TaskTypeUpload:
			payload["remote_path"] = getString(params, "remote_path")
			payload["file_id"] = getString(params, "file_id")
		case c2.TaskTypeDownload:
			payload["remote_path"] = getString(params, "remote_path")
		case c2.TaskTypePortFwd:
			payload["action"] = getString(params, "action")
			payload["local_port"] = params["local_port"]
			payload["remote_host"] = getString(params, "remote_host")
			payload["remote_port"] = params["remote_port"]
		case c2.TaskTypeSocksStart:
			payload["port"] = params["port"]
		case c2.TaskTypeLoadAssembly:
			payload["data"] = getString(params, "data")
			payload["file_id"] = getString(params, "file_id")
			payload["args"] = getString(params, "args")
		case c2.TaskTypePersist:
			payload["method"] = getString(params, "method")
		case c2.TaskTypePwd, c2.TaskTypePs, c2.TaskTypeScreenshot, c2.TaskTypeSocksStop:
			// no extra params
		default:
			return makeC2Result(nil, fmt.Errorf("unsupported task_type: %s", taskTypeStr))
		}

		input := c2.EnqueueTaskInput{
			SessionID:      sessionID,
			TaskType:       taskType,
			Payload:        payload,
			Source:         "ai",
			ConversationID: agent.ConversationIDFromContext(ctx),
			UserCtx:        ctx,
		}
		task, err := m.EnqueueTask(input)
		if err != nil {
			return makeC2Result(nil, err)
		}
		return makeC2Result(map[string]interface{}{"task_id": task.ID, "status": task.Status}, nil)
	})
}

// ============================================================================
// c2_task_manage — 任务管理工具（查询/等待/取消）
// ============================================================================

func registerC2TaskManageTool(s *mcp.Server, m *c2.Manager, l *zap.Logger) {
	s.RegisterTool(mcp.Tool{
		Name: builtin.ToolC2TaskManage,
		Description: `C2 任务管理。通过 action 参数选择操作：
- get_result: 获取任务详情和结果（需 task_id）
- wait: 阻塞等待任务完成并返回结果（需 task_id）
- list: 列出任务（可按 session_id/status 过滤）
- cancel: 取消排队中的任务（需 task_id）`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action":          map[string]interface{}{"type": "string", "description": "操作: get_result/wait/list/cancel", "enum": []string{"get_result", "wait", "list", "cancel"}},
				"task_id":         map[string]interface{}{"type": "string", "description": "任务 ID（get_result/wait/cancel 需要）"},
				"session_id":      map[string]interface{}{"type": "string", "description": "按会话过滤（list）"},
				"status":          map[string]interface{}{"type": "string", "description": "按状态过滤: queued/sent/running/success/failed/cancelled（list）"},
				"limit":           map[string]interface{}{"type": "integer", "description": "返回数量上限（list）"},
				"timeout_seconds": map[string]interface{}{"type": "integer", "description": "等待超时秒数（wait），默认 60"},
			},
			"required": []string{"action"},
		},
	}, func(ctx context.Context, params map[string]interface{}) (*mcp.ToolResult, error) {
		action := getString(params, "action")

		switch action {
		case "get_result":
			id := getString(params, "task_id")
			task, err := m.DB().GetC2Task(id)
			if err != nil {
				return makeC2Result(nil, err)
			}
			if task == nil {
				return makeC2Result(nil, fmt.Errorf("task not found"))
			}
			return makeC2Result(map[string]interface{}{"task": task}, nil)

		case "wait":
			id := getString(params, "task_id")
			timeout := int(getFloat64(params, "timeout_seconds"))
			if timeout <= 0 {
				timeout = 60
			}
			deadline := time.Now().Add(time.Duration(timeout) * time.Second)
			for time.Now().Before(deadline) {
				task, err := m.DB().GetC2Task(id)
				if err != nil {
					return makeC2Result(nil, err)
				}
				if task == nil {
					return makeC2Result(nil, fmt.Errorf("task not found"))
				}
				if task.Status == "success" || task.Status == "failed" || task.Status == "cancelled" {
					return makeC2Result(map[string]interface{}{"task": task}, nil)
				}
				select {
				case <-time.After(500 * time.Millisecond):
				case <-ctx.Done():
					return makeC2Result(nil, ctx.Err())
				}
			}
			return makeC2Result(nil, fmt.Errorf("timeout waiting for task completion"))

		case "list":
			filter := database.ListC2TasksFilter{
				SessionID: getString(params, "session_id"),
				ProjectID: mcpEffectiveProjectFilter(ctx, m.DB()),
				Status:    getString(params, "status"),
			}
			if limit := int(getFloat64(params, "limit")); limit > 0 {
				filter.Limit = limit
			}
			tasks, err := m.DB().ListC2TasksForAccess(filter, c2ToolAccess(ctx))
			return makeC2Result(map[string]interface{}{"tasks": tasks, "count": len(tasks)}, err)

		case "cancel":
			id := getString(params, "task_id")
			err := m.CancelTask(id)
			return makeC2Result(map[string]interface{}{"cancelled": err == nil}, err)

		default:
			return makeC2Result(nil, fmt.Errorf("unknown action: %s", action))
		}
	})
}

// ============================================================================
// c2_payload — Payload 统一工具
// ============================================================================

func registerC2PayloadTool(s *mcp.Server, m *c2.Manager, l *zap.Logger, webListenPort int) {
	s.RegisterTool(mcp.Tool{
		Name: builtin.ToolC2Payload,
		Description: fmt.Sprintf(`C2 Payload 生成。通过 action 参数选择操作：
- oneliner: 生成单行 payload。kind 必须与监听器协议一致，否则会失败：
  • tcp_reverse：默认仅支持 build 加密 Beacon；若监听器 config.allow_legacy_shell=true，才可用 kind: bash, nc, nc_mkfifo, python, perl, powershell。
  • http_beacon / https_beacon / websocket：仅 HTTP(S) Beacon 轮询，oneliner 只能用 kind: curl_beacon（脚本内用 bash+curl，与「tcp 的 bash」不同）。curl_beacon 返回串末尾含「 &」用于把整个 bash -c 放后台；若用 exec/execute 同步执行，必须整段原样复制（含末尾 &）。若删掉 &，内部 while 死循环占满前台，调用会一直阻塞到超时/杀进程。
  • 公网部署 tcp_reverse 请用 build 生成加密 Beacon，勿开启 allow_legacy_shell。
  • 省略 kind 时，会按监听器类型自动选第一个兼容类型（HTTP 系默认为 curl_beacon）。
- build: 交叉编译 beacon 二进制。支持 http_beacon / https_beacon / websocket / tcp_reverse（tcp_reverse 植入端回连后先发魔数 CSB1，再经 AES-GCM 解密且校验 ImplantToken 后才登记会话）。
依赖的监听器 bind_port 须避开本服务 Web 端口 %d（配置 server.port，与 c2_listener 描述一致），否则 Beacon 无法正确回连。`, webListenPort),
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action":         map[string]interface{}{"type": "string", "description": "操作: oneliner/build", "enum": []string{"oneliner", "build"}},
				"listener_id":    map[string]interface{}{"type": "string", "description": "监听器 ID（必填）。oneliner 前请确认该监听器的 type，再选兼容的 kind"},
				"kind":           map[string]interface{}{"type": "string", "description": "仅 action=oneliner 需要。tcp_reverse: bash|nc|nc_mkfifo|python|perl|powershell；http_beacon|https_beacon|websocket: 仅 curl_beacon"},
				"host":           map[string]interface{}{"type": "string", "description": "oneliner/build 可选覆盖：非空则强制用作植入回连主机。留空时顺序为：监听器 callback_host（create/update 的 callback_host 参数写入）→ bind_host（0.0.0.0 时尝试本机对外 IP 探测）"},
				"os":             map[string]interface{}{"type": "string", "description": "目标 OS（build）: linux/windows/darwin", "default": "linux"},
				"arch":           map[string]interface{}{"type": "string", "description": "目标架构（build）: amd64/arm64/386/arm", "default": "amd64"},
				"sleep_seconds":  map[string]interface{}{"type": "integer", "description": "默认心跳间隔（build）"},
				"jitter_percent": map[string]interface{}{"type": "integer", "description": "默认抖动百分比（build）"},
			},
			"required": []string{"action", "listener_id"},
		},
	}, func(ctx context.Context, params map[string]interface{}) (*mcp.ToolResult, error) {
		action := getString(params, "action")
		listenerID := getString(params, "listener_id")

		switch action {
		case "oneliner":
			listener, err := m.DB().GetC2Listener(listenerID)
			if err != nil {
				return makeC2Result(nil, err)
			}
			if listener == nil {
				return makeC2Result(nil, fmt.Errorf("listener not found"))
			}
			host := c2.ResolveBeaconDialHost(listener, getString(params, "host"), l, listenerID)
			kind := c2.OnelinerKind(getString(params, "kind"))
			if kind == "" {
				compatible := c2.OnelinerKindsForListener(listener.Type)
				if len(compatible) > 0 {
					kind = compatible[0]
				}
			}
			if !c2.IsOnelinerCompatible(listener.Type, kind) {
				compatible := c2.OnelinerKindsForListener(listener.Type)
				names := make([]string, len(compatible))
				for i, k := range compatible {
					names[i] = string(k)
				}
				return makeC2Result(nil, fmt.Errorf("监听器类型 %s 不支持 %s，兼容类型: %v", listener.Type, kind, names))
			}
			if err := c2.ValidateOnelinerForListener(listener, kind); err != nil {
				return makeC2Result(nil, err)
			}
			input := c2.OnelinerInput{
				Kind:         kind,
				Host:         host,
				Port:         listener.BindPort,
				HTTPBaseURL:  fmt.Sprintf("http://%s:%d", host, listener.BindPort),
				ImplantToken: listener.ImplantToken,
			}
			oneliner, err := c2.GenerateOneliner(input)
			if err != nil {
				return makeC2Result(nil, err)
			}
			out := map[string]interface{}{
				"oneliner": oneliner, "kind": input.Kind, "host": host, "port": listener.BindPort,
			}
			if kind == c2.OnelinerCurl {
				out["usage_note"] = "同步 exec/execute：整段原样执行（末尾须有「 &」）。去掉则 while 永不结束，工具会一直卡住。"
			}
			return makeC2Result(out, nil)

		case "build":
			builder := c2.NewPayloadBuilder(m, l, "", "")
			input := c2.PayloadBuilderInput{
				ListenerID:    listenerID,
				OS:            getString(params, "os"),
				Arch:          getString(params, "arch"),
				SleepSeconds:  int(getFloat64(params, "sleep_seconds")),
				JitterPercent: int(getFloat64(params, "jitter_percent")),
				Host:          strings.TrimSpace(getString(params, "host")),
			}
			result, err := builder.BuildBeacon(input)
			if err != nil {
				return makeC2Result(nil, err)
			}
			if principal, ok := authctx.PrincipalFromContext(ctx); ok {
				_ = m.DB().RecordC2PayloadArtifact(filepath.Base(result.OutputPath), result.PayloadID, result.ListenerID, principal.UserID)
			}
			return makeC2Result(map[string]interface{}{
				"payload_id": result.PayloadID, "download_path": result.DownloadPath,
				"os": result.OS, "arch": result.Arch, "size_bytes": result.SizeBytes,
			}, nil)

		default:
			return makeC2Result(nil, fmt.Errorf("unknown action: %s", action))
		}
	})
}

// ============================================================================
// c2_event — 事件查询工具
// ============================================================================

func registerC2EventTool(s *mcp.Server, m *c2.Manager, l *zap.Logger) {
	s.RegisterTool(mcp.Tool{
		Name:        builtin.ToolC2Event,
		Description: "获取 C2 事件（上线/掉线/任务/错误），支持按级别/类别/会话/任务/时间过滤",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"level":      map[string]interface{}{"type": "string", "description": "级别过滤: info/warn/critical"},
				"category":   map[string]interface{}{"type": "string", "description": "类别过滤: listener/session/task/payload/opsec"},
				"session_id": map[string]interface{}{"type": "string", "description": "按会话过滤"},
				"task_id":    map[string]interface{}{"type": "string", "description": "按任务过滤"},
				"since":      map[string]interface{}{"type": "string", "description": "起始时间（RFC3339 格式，如 2025-01-01T00:00:00Z）"},
				"limit":      map[string]interface{}{"type": "integer", "default": 50, "description": "返回数量"},
			},
		},
	}, func(ctx context.Context, params map[string]interface{}) (*mcp.ToolResult, error) {
		filter := database.ListC2EventsFilter{
			Level:     getString(params, "level"),
			Category:  getString(params, "category"),
			ProjectID: mcpEffectiveProjectFilter(ctx, m.DB()),
			SessionID: getString(params, "session_id"),
			TaskID:    getString(params, "task_id"),
			Limit:     int(getFloat64(params, "limit")),
		}
		if filter.Limit <= 0 {
			filter.Limit = 50
		}
		if since := getString(params, "since"); since != "" {
			if t, err := time.Parse(time.RFC3339, since); err == nil {
				filter.Since = &t
			}
		}
		events, err := m.DB().ListC2EventsForAccess(filter, c2ToolAccess(ctx))
		return makeC2Result(map[string]interface{}{"events": events, "count": len(events)}, err)
	})
}

func c2ToolAccess(ctx context.Context) database.RBACListAccess {
	principal, ok := authctx.PrincipalFromContext(ctx)
	if !ok {
		return database.RBACListAccess{Scope: database.RBACScopeAssigned}
	}
	return database.RBACListAccess{UserID: principal.UserID, Scope: principal.ScopeFor("c2:read")}
}

// ============================================================================
// c2_profile — Malleable Profile 管理工具（新增）
// ============================================================================

func registerC2ProfileTool(s *mcp.Server, m *c2.Manager, l *zap.Logger) {
	s.RegisterTool(mcp.Tool{
		Name: builtin.ToolC2Profile,
		Description: `C2 Malleable Profile 管理（控制 beacon 通信伪装）。通过 action 参数选择操作：
- list: 列出所有 Profile
- get: 获取 Profile 详情（需 profile_id）
- create: 创建 Profile（需 name，可选 user_agent/uris/request_headers/response_headers/body_template/jitter_min_ms/jitter_max_ms）
- update: 更新 Profile（需 profile_id）
- delete: 删除 Profile（需 profile_id）`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action":           map[string]interface{}{"type": "string", "description": "操作: list/get/create/update/delete", "enum": []string{"list", "get", "create", "update", "delete"}},
				"profile_id":       map[string]interface{}{"type": "string", "description": "Profile ID（get/update/delete 需要）"},
				"name":             map[string]interface{}{"type": "string", "description": "Profile 名称"},
				"user_agent":       map[string]interface{}{"type": "string", "description": "User-Agent 字符串"},
				"uris":             map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "beacon 请求的 URI 列表"},
				"request_headers":  map[string]interface{}{"type": "object", "description": "自定义请求头"},
				"response_headers": map[string]interface{}{"type": "object", "description": "自定义响应头"},
				"body_template":    map[string]interface{}{"type": "string", "description": "响应体模板"},
				"jitter_min_ms":    map[string]interface{}{"type": "integer", "description": "最小抖动（毫秒）"},
				"jitter_max_ms":    map[string]interface{}{"type": "integer", "description": "最大抖动（毫秒）"},
			},
			"required": []string{"action"},
		},
	}, func(ctx context.Context, params map[string]interface{}) (*mcp.ToolResult, error) {
		action := getString(params, "action")
		id := getString(params, "profile_id")

		switch action {
		case "list":
			profiles, err := m.DB().ListC2Profiles()
			return makeC2Result(map[string]interface{}{"profiles": profiles, "count": len(profiles)}, err)

		case "get":
			profile, err := m.DB().GetC2Profile(id)
			if err != nil {
				return makeC2Result(nil, err)
			}
			if profile == nil {
				return makeC2Result(nil, fmt.Errorf("profile not found"))
			}
			return makeC2Result(map[string]interface{}{"profile": profile}, nil)

		case "create":
			profile := &database.C2Profile{
				ID:           "p_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:14],
				Name:         getString(params, "name"),
				UserAgent:    getString(params, "user_agent"),
				BodyTemplate: getString(params, "body_template"),
				JitterMinMS:  int(getFloat64(params, "jitter_min_ms")),
				JitterMaxMS:  int(getFloat64(params, "jitter_max_ms")),
				CreatedAt:    time.Now(),
			}
			if uris, ok := params["uris"]; ok {
				if arr, ok := uris.([]interface{}); ok {
					for _, u := range arr {
						if s, ok := u.(string); ok {
							profile.URIs = append(profile.URIs, s)
						}
					}
				}
			}
			if rh, ok := params["request_headers"]; ok {
				if m, ok := rh.(map[string]interface{}); ok {
					profile.RequestHeaders = make(map[string]string)
					for k, v := range m {
						profile.RequestHeaders[k], _ = v.(string)
					}
				}
			}
			if rh, ok := params["response_headers"]; ok {
				if m, ok := rh.(map[string]interface{}); ok {
					profile.ResponseHeaders = make(map[string]string)
					for k, v := range m {
						profile.ResponseHeaders[k], _ = v.(string)
					}
				}
			}
			if err := m.DB().CreateC2Profile(profile); err != nil {
				return makeC2Result(nil, err)
			}
			return makeC2Result(map[string]interface{}{"profile": profile}, nil)

		case "update":
			profile, err := m.DB().GetC2Profile(id)
			if err != nil {
				return makeC2Result(nil, err)
			}
			if profile == nil {
				return makeC2Result(nil, fmt.Errorf("profile not found"))
			}
			if v := getString(params, "name"); v != "" {
				profile.Name = v
			}
			if v := getString(params, "user_agent"); v != "" {
				profile.UserAgent = v
			}
			if v := getString(params, "body_template"); v != "" {
				profile.BodyTemplate = v
			}
			if v := int(getFloat64(params, "jitter_min_ms")); v > 0 {
				profile.JitterMinMS = v
			}
			if v := int(getFloat64(params, "jitter_max_ms")); v > 0 {
				profile.JitterMaxMS = v
			}
			if uris, ok := params["uris"]; ok {
				if arr, ok := uris.([]interface{}); ok {
					profile.URIs = nil
					for _, u := range arr {
						if s, ok := u.(string); ok {
							profile.URIs = append(profile.URIs, s)
						}
					}
				}
			}
			if rh, ok := params["request_headers"]; ok {
				if mp, ok := rh.(map[string]interface{}); ok {
					profile.RequestHeaders = make(map[string]string)
					for k, v := range mp {
						profile.RequestHeaders[k], _ = v.(string)
					}
				}
			}
			if rh, ok := params["response_headers"]; ok {
				if mp, ok := rh.(map[string]interface{}); ok {
					profile.ResponseHeaders = make(map[string]string)
					for k, v := range mp {
						profile.ResponseHeaders[k], _ = v.(string)
					}
				}
			}
			if err := m.DB().UpdateC2Profile(profile); err != nil {
				return makeC2Result(nil, err)
			}
			return makeC2Result(map[string]interface{}{"profile": profile}, nil)

		case "delete":
			err := m.DB().DeleteC2Profile(id)
			return makeC2Result(map[string]interface{}{"deleted": err == nil}, err)

		default:
			return makeC2Result(nil, fmt.Errorf("unknown action: %s", action))
		}
	})
}

// ============================================================================
// c2_file — 文件管理工具（新增）
// ============================================================================

func registerC2FileTool(s *mcp.Server, m *c2.Manager, l *zap.Logger) {
	s.RegisterTool(mcp.Tool{
		Name: builtin.ToolC2File,
		Description: `C2 文件管理。通过 action 参数选择操作：
- list: 列出会话的文件传输记录（需 session_id）
- get_result: 获取任务结果文件路径（截图等，需 task_id）`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action":     map[string]interface{}{"type": "string", "description": "操作: list/get_result", "enum": []string{"list", "get_result"}},
				"session_id": map[string]interface{}{"type": "string", "description": "会话 ID（list 需要）"},
				"task_id":    map[string]interface{}{"type": "string", "description": "任务 ID（get_result 需要）"},
			},
			"required": []string{"action"},
		},
	}, func(ctx context.Context, params map[string]interface{}) (*mcp.ToolResult, error) {
		action := getString(params, "action")

		switch action {
		case "list":
			sessionID := getString(params, "session_id")
			if sessionID == "" {
				return makeC2Result(nil, fmt.Errorf("session_id required"))
			}
			files, err := m.DB().ListC2FilesBySession(sessionID)
			return makeC2Result(map[string]interface{}{"files": files, "count": len(files)}, err)

		case "get_result":
			taskID := getString(params, "task_id")
			task, err := m.DB().GetC2Task(taskID)
			if err != nil {
				return makeC2Result(nil, err)
			}
			if task == nil {
				return makeC2Result(nil, fmt.Errorf("task not found"))
			}
			if task.ResultBlobPath == "" {
				return makeC2Result(map[string]interface{}{"has_file": false, "task_id": taskID}, nil)
			}
			return makeC2Result(map[string]interface{}{
				"has_file":  true,
				"task_id":   taskID,
				"file_path": task.ResultBlobPath,
			}, nil)

		default:
			return makeC2Result(nil, fmt.Errorf("unknown action: %s", action))
		}
	})
}

// ============================================================================
// 工具函数
// ============================================================================

func getString(params map[string]interface{}, key string) string {
	if v, ok := params[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getFloat64(params map[string]interface{}, key string) float64 {
	if v, ok := params[key]; ok {
		switch n := v.(type) {
		case float64:
			return n
		case int:
			return float64(n)
		case string:
			if f, err := strconv.ParseFloat(n, 64); err == nil {
				return f
			}
		}
	}
	return 0
}
