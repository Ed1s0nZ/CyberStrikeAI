package config

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Version     string                `yaml:"version,omitempty" json:"version,omitempty"` // 前端显示的版本号，如 v1.3.3
	Server      ServerConfig          `yaml:"server"`
	Log         LogConfig             `yaml:"log"`
	MCP         MCPConfig             `yaml:"mcp"`
	OpenAI      OpenAIConfig          `yaml:"openai"`
	FOFA        FofaConfig            `yaml:"fofa,omitempty" json:"fofa,omitempty"`
	Agent       AgentConfig           `yaml:"agent"`
	Hitl        HitlConfig            `yaml:"hitl,omitempty" json:"hitl,omitempty"`
	Security    SecurityConfig        `yaml:"security"`
	Database    DatabaseConfig        `yaml:"database"`
	Auth        AuthConfig            `yaml:"auth"`
	ExternalMCP ExternalMCPConfig     `yaml:"external_mcp,omitempty"`
	Knowledge   KnowledgeConfig       `yaml:"knowledge,omitempty"`
	C2          C2Config              `yaml:"c2,omitempty" json:"c2,omitempty"` // 内置 C2 总开关；未配置时默认启用
	Robots      RobotsConfig          `yaml:"robots,omitempty" json:"robots,omitempty"`         // 企业微信/钉钉/飞书等机器人配置
	RolesDir    string                `yaml:"roles_dir,omitempty" json:"roles_dir,omitempty"`   // 角色配置文件目录（新方式）
	Roles       map[string]RoleConfig `yaml:"roles,omitempty" json:"roles,omitempty"`           // 向后兼容：支持在主配置文件中定义角色
	SkillsDir   string                `yaml:"skills_dir,omitempty" json:"skills_dir,omitempty"` // Skills配置文件目录
	AgentsDir   string                `yaml:"agents_dir,omitempty" json:"agents_dir,omitempty"` // 多代理子 Agent Markdown 定义目录（*.md，YAML front matter）
	MultiAgent  MultiAgentConfig      `yaml:"multi_agent,omitempty" json:"multi_agent,omitempty"`
}

// MultiAgentConfig 基于 CloudWeGo Eino adk/prebuilt 的多代理编排（deep | plan_execute | supervisor，与单 Agent /agent-loop 并存）。
type MultiAgentConfig struct {
	Enabled            bool `yaml:"enabled" json:"enabled"`
	RobotUseMultiAgent bool `yaml:"robot_use_multi_agent" json:"robot_use_multi_agent"` // 为 true 时钉钉/飞书/企微机器人走 Eino 多代理
	BatchUseMultiAgent bool `yaml:"batch_use_multi_agent" json:"batch_use_multi_agent"` // 为 true 时批量任务队列中每子任务走 Eino 多代理
	// Orchestration 已弃用：保留仅兼容旧版 config.yaml；编排由聊天/WebShell 请求体 orchestration 决定，未传时按 deep。
	Orchestration string `yaml:"orchestration,omitempty" json:"orchestration,omitempty"`
	MaxIteration  int    `yaml:"max_iteration" json:"max_iteration"` // 主代理 / 执行器最大推理轮次（Deep、Supervisor、plan_execute 的 Executor）
	// PlanExecuteLoopMaxIterations plan_execute 模式下 execute↔replan 外层循环上限；0 表示用 Eino 默认 10。
	PlanExecuteLoopMaxIterations int    `yaml:"plan_execute_loop_max_iterations,omitempty" json:"plan_execute_loop_max_iterations,omitempty"`
	SubAgentMaxIterations        int    `yaml:"sub_agent_max_iterations" json:"sub_agent_max_iterations"`
	WithoutGeneralSubAgent       bool   `yaml:"without_general_sub_agent" json:"without_general_sub_agent"`
	WithoutWriteTodos            bool   `yaml:"without_write_todos" json:"without_write_todos"`
	OrchestratorInstruction      string `yaml:"orchestrator_instruction" json:"orchestrator_instruction"`
	// OrchestratorInstructionPlanExecute plan_execute 主代理（规划侧）系统提示；非空且 agents/orchestrator-plan-execute.md 正文为空或未存在时生效。不与 Deep 的 orchestrator_instruction 混用。
	OrchestratorInstructionPlanExecute string `yaml:"orchestrator_instruction_plan_execute,omitempty" json:"orchestrator_instruction_plan_execute,omitempty"`
	// OrchestratorInstructionSupervisor supervisor 主代理系统提示（transfer/exit 说明仍由运行追加）；非空且 agents/orchestrator-supervisor.md 正文为空或未存在时生效。
	OrchestratorInstructionSupervisor string                `yaml:"orchestrator_instruction_supervisor,omitempty" json:"orchestrator_instruction_supervisor,omitempty"`
	SubAgents                         []MultiAgentSubConfig `yaml:"sub_agents" json:"sub_agents"`
	// SubAgentUserContextMaxRunes caps the user-context supplement appended to task descriptions for sub-agents.
	// 0 (default) uses the built-in default of 2000 runes; negative value disables injection entirely.
	SubAgentUserContextMaxRunes int `yaml:"sub_agent_user_context_max_runes,omitempty" json:"sub_agent_user_context_max_runes,omitempty"`
	// EinoSkills configures CloudWeGo Eino ADK skill middleware + optional local filesystem/execute on DeepAgent.
	EinoSkills MultiAgentEinoSkillsConfig `yaml:"eino_skills,omitempty" json:"eino_skills,omitempty"`
	// EinoMiddleware wires optional ADK middleware (patchtoolcalls, toolsearch, plantask, reduction) and Deep extras.
	EinoMiddleware MultiAgentEinoMiddlewareConfig `yaml:"eino_middleware,omitempty" json:"eino_middleware,omitempty"`
}

// MultiAgentEinoMiddlewareConfig optional Eino ADK middleware and Deep / supervisor tuning.
type MultiAgentEinoMiddlewareConfig struct {
	// PatchToolCalls inserts placeholder tool results for dangling assistant tool_calls (nil = enabled).
	PatchToolCalls *bool `yaml:"patch_tool_calls,omitempty" json:"patch_tool_calls,omitempty"`
	// ToolSearch enables dynamictool/toolsearch: hide tail tools until model calls tool_search (reduces prompt tools).
	ToolSearchEnable        bool `yaml:"tool_search_enable,omitempty" json:"tool_search_enable,omitempty"`
	ToolSearchMinTools      int  `yaml:"tool_search_min_tools,omitempty" json:"tool_search_min_tools,omitempty"`           // default 20; applies when len(tools) >= this
	ToolSearchAlwaysVisible int  `yaml:"tool_search_always_visible,omitempty" json:"tool_search_always_visible,omitempty"` // default 12; first N tools stay always visible
	// ToolSearchAlwaysVisibleTools keeps specified tool names always visible (never hidden by tool_search).
	ToolSearchAlwaysVisibleTools []string `yaml:"tool_search_always_visible_tools,omitempty" json:"tool_search_always_visible_tools,omitempty"`
	// Plantask adds TaskCreate/Get/Update/List (file-backed under skills dir); requires eino_skills + local backend.
	PlantaskEnable bool `yaml:"plantask_enable,omitempty" json:"plantask_enable,omitempty"`
	// PlantaskRelDir relative to skills_dir for per-conversation task boards (default .eino/plantask).
	PlantaskRelDir string `yaml:"plantask_rel_dir,omitempty" json:"plantask_rel_dir,omitempty"`
	// Reduction truncates/offloads large tool outputs (requires eino local backend for Write).
	ReductionEnable       bool     `yaml:"reduction_enable,omitempty" json:"reduction_enable,omitempty"`
	ReductionRootDir      string   `yaml:"reduction_root_dir,omitempty" json:"reduction_root_dir,omitempty"` // default: os temp + conversation id
	ReductionMaxLengthForTrunc int `yaml:"reduction_max_length_for_trunc,omitempty" json:"reduction_max_length_for_trunc,omitempty"` // default 12000
	ReductionMaxTokensForClear int `yaml:"reduction_max_tokens_for_clear,omitempty" json:"reduction_max_tokens_for_clear,omitempty"` // default 50000
	ReductionClearExclude []string `yaml:"reduction_clear_exclude,omitempty" json:"reduction_clear_exclude,omitempty"`
	ReductionSubAgents    bool     `yaml:"reduction_sub_agents,omitempty" json:"reduction_sub_agents,omitempty"` // also attach to sub-agents
	// SummarizationTriggerRatio controls summarization trigger threshold as max_total_tokens * ratio (default 0.8).
	SummarizationTriggerRatio float64 `yaml:"summarization_trigger_ratio,omitempty" json:"summarization_trigger_ratio,omitempty"`
	// SummarizationEmitInternalEvents controls middleware internal event emission (default true).
	SummarizationEmitInternalEvents *bool `yaml:"summarization_emit_internal_events,omitempty" json:"summarization_emit_internal_events,omitempty"`
	// HistoryInputBudgetRatio caps pre-agent history tokens as max_total_tokens * ratio (default 0.35).
	HistoryInputBudgetRatio float64 `yaml:"history_input_budget_ratio,omitempty" json:"history_input_budget_ratio,omitempty"`
	// PlanExecuteUserInputBudgetRatio caps planner/replanner/executor userInput prompt budget ratio (default 0.35).
	PlanExecuteUserInputBudgetRatio float64 `yaml:"plan_execute_user_input_budget_ratio,omitempty" json:"plan_execute_user_input_budget_ratio,omitempty"`
	// PlanExecuteExecutedStepsBudgetRatio caps executed_steps prompt budget ratio (default 0.2).
	PlanExecuteExecutedStepsBudgetRatio float64 `yaml:"plan_execute_executed_steps_budget_ratio,omitempty" json:"plan_execute_executed_steps_budget_ratio,omitempty"`
	// PlanExecuteMaxStepResultRunes caps each executed step result length for prompt view (default 4000).
	PlanExecuteMaxStepResultRunes int `yaml:"plan_execute_max_step_result_runes,omitempty" json:"plan_execute_max_step_result_runes,omitempty"`
	// PlanExecuteKeepLastSteps keeps only the tail steps in prompt view (default 8).
	PlanExecuteKeepLastSteps int `yaml:"plan_execute_keep_last_steps,omitempty" json:"plan_execute_keep_last_steps,omitempty"`
	// CheckpointDir when non-empty enables adk.Runner CheckPointStore (file-backed) for interrupt/resume persistence.
	CheckpointDir string `yaml:"checkpoint_dir,omitempty" json:"checkpoint_dir,omitempty"`
	// DeepOutputKey passed to deep.Config OutputKey (session final text); empty = off.
	DeepOutputKey string `yaml:"deep_output_key,omitempty" json:"deep_output_key,omitempty"`
	// DeepModelRetryMaxRetries > 0 enables deep.Config ModelRetryConfig (framework-level chat model retries).
	DeepModelRetryMaxRetries int `yaml:"deep_model_retry_max_retries,omitempty" json:"deep_model_retry_max_retries,omitempty"`
	// TaskToolDescriptionPrefix when non-empty sets deep.Config TaskToolDescriptionGenerator (sub-agent names appended).
	TaskToolDescriptionPrefix string `yaml:"task_tool_description_prefix,omitempty" json:"task_tool_description_prefix,omitempty"`
}

func (c MultiAgentEinoMiddlewareConfig) SummarizationTriggerRatioEffective() float64 {
	v := c.SummarizationTriggerRatio
	if v <= 0 {
		return 0.8
	}
	if v < 0.5 {
		return 0.5
	}
	if v > 0.95 {
		return 0.95
	}
	return v
}

func (c MultiAgentEinoMiddlewareConfig) SummarizationEmitInternalEventsEffective() bool {
	if c.SummarizationEmitInternalEvents != nil {
		return *c.SummarizationEmitInternalEvents
	}
	return true
}

func (c MultiAgentEinoMiddlewareConfig) HistoryInputBudgetRatioEffective() float64 {
	v := c.HistoryInputBudgetRatio
	if v <= 0 {
		return 0.35
	}
	if v < 0.15 {
		return 0.15
	}
	if v > 0.6 {
		return 0.6
	}
	return v
}

func (c MultiAgentEinoMiddlewareConfig) PlanExecuteUserInputBudgetRatioEffective() float64 {
	v := c.PlanExecuteUserInputBudgetRatio
	if v <= 0 {
		return 0.35
	}
	if v < 0.1 {
		return 0.1
	}
	if v > 0.6 {
		return 0.6
	}
	return v
}

func (c MultiAgentEinoMiddlewareConfig) PlanExecuteExecutedStepsBudgetRatioEffective() float64 {
	v := c.PlanExecuteExecutedStepsBudgetRatio
	if v <= 0 {
		return 0.2
	}
	if v < 0.08 {
		return 0.08
	}
	if v > 0.5 {
		return 0.5
	}
	return v
}

func (c MultiAgentEinoMiddlewareConfig) PlanExecuteMaxStepResultRunesEffective() int {
	if c.PlanExecuteMaxStepResultRunes > 0 {
		return c.PlanExecuteMaxStepResultRunes
	}
	return 4000
}

func (c MultiAgentEinoMiddlewareConfig) PlanExecuteKeepLastStepsEffective() int {
	if c.PlanExecuteKeepLastSteps > 0 {
		return c.PlanExecuteKeepLastSteps
	}
	return 8
}

func (c MultiAgentEinoMiddlewareConfig) ReductionMaxLengthForTruncEffective() int {
	if c.ReductionMaxLengthForTrunc > 0 {
		return c.ReductionMaxLengthForTrunc
	}
	return 12000
}

func (c MultiAgentEinoMiddlewareConfig) ReductionMaxTokensForClearEffective() int {
	if c.ReductionMaxTokensForClear > 0 {
		return c.ReductionMaxTokensForClear
	}
	return 50000
}

// MultiAgentEinoSkillsConfig toggles Eino official skill progressive disclosure and host filesystem tools.
type MultiAgentEinoSkillsConfig struct {
	// Disable skips skill middleware (and does not attach local FS tools for Deep).
	Disable bool `yaml:"disable" json:"disable"`
	// FilesystemTools registers read_file/glob/grep/write/edit/execute (eino-ext local backend). Nil/omitted = true.
	FilesystemTools *bool `yaml:"filesystem_tools,omitempty" json:"filesystem_tools,omitempty"`
	// SkillToolName overrides the default Eino tool name "skill".
	SkillToolName string `yaml:"skill_tool_name,omitempty" json:"skill_tool_name,omitempty"`
}

// EinoSkillFilesystemToolsEffective returns whether Deep/sub-agents should attach local filesystem + streaming shell.
func (c MultiAgentEinoSkillsConfig) EinoSkillFilesystemToolsEffective() bool {
	if c.FilesystemTools != nil {
		return *c.FilesystemTools
	}
	return true
}

// PatchToolCallsEffective returns whether patchtoolcalls middleware should run (default true).
func (c MultiAgentEinoMiddlewareConfig) PatchToolCallsEffective() bool {
	if c.PatchToolCalls != nil {
		return *c.PatchToolCalls
	}
	return true
}

// MultiAgentSubConfig 子代理（Eino ChatModelAgent）：deep 下由 task 调度；supervisor 下由 transfer 委派；plan_execute 不使用子代理列表。
type MultiAgentSubConfig struct {
	ID            string   `yaml:"id" json:"id"`
	Name          string   `yaml:"name" json:"name"`
	Description   string   `yaml:"description" json:"description"`
	Instruction   string   `yaml:"instruction" json:"instruction"`
	BindRole      string   `yaml:"bind_role,omitempty" json:"bind_role,omitempty"` // 可选：关联主配置 roles 中的角色名；未配 role_tools 时沿用该角色的 tools
	RoleTools     []string `yaml:"role_tools" json:"role_tools"`                   // 与单 Agent 角色工具相同 key；空表示全部工具（bind_role 可补全 tools）
	MaxIterations int      `yaml:"max_iterations" json:"max_iterations"`
	Kind          string   `yaml:"kind,omitempty" json:"kind,omitempty"` // 仅 Markdown：kind=orchestrator 表示 Deep 主代理（与 orchestrator.md 二选一约定）
}

// MultiAgentPublic 返回给前端的精简信息（不含子代理指令全文）。
type MultiAgentPublic struct {
	Enabled                      bool   `json:"enabled"`
	RobotUseMultiAgent           bool   `json:"robot_use_multi_agent"`
	BatchUseMultiAgent           bool   `json:"batch_use_multi_agent"`
	SubAgentCount                int    `json:"sub_agent_count"`
	Orchestration                string `json:"orchestration,omitempty"`
	PlanExecuteLoopMaxIterations int    `json:"plan_execute_loop_max_iterations"`
	ToolSearchAlwaysVisibleTools []string `json:"tool_search_always_visible_tools,omitempty"`
	ToolSearchAlwaysVisibleEffectiveTools []string `json:"tool_search_always_visible_effective_tools,omitempty"`
}

// NormalizeMultiAgentOrchestration 返回 deep、plan_execute 或 supervisor。
func NormalizeMultiAgentOrchestration(s string) string {
	v := strings.TrimSpace(strings.ToLower(s))
	switch v {
	case "plan_execute", "plan-execute", "planexecute", "pe":
		return "plan_execute"
	case "supervisor", "super", "sv":
		return "supervisor"
	default:
		return "deep"
	}
}

// MultiAgentAPIUpdate 设置页/API 仅更新多代理标量字段；写入 YAML 时不覆盖 sub_agents 等块。
type MultiAgentAPIUpdate struct {
	Enabled                      bool `json:"enabled"`
	RobotUseMultiAgent           bool `json:"robot_use_multi_agent"`
	BatchUseMultiAgent           bool `json:"batch_use_multi_agent"`
	PlanExecuteLoopMaxIterations *int `json:"plan_execute_loop_max_iterations,omitempty"`
	ToolSearchAlwaysVisibleTools []string `json:"tool_search_always_visible_tools,omitempty"`
}

// RobotsConfig 机器人配置（企业微信、钉钉、飞书等）
type RobotsConfig struct {
	Session  RobotSessionConfig  `yaml:"session,omitempty" json:"session,omitempty"`   // 机器人会话隔离策略
	Wecom    RobotWecomConfig    `yaml:"wecom,omitempty" json:"wecom,omitempty"`       // 企业微信
	Dingtalk RobotDingtalkConfig `yaml:"dingtalk,omitempty" json:"dingtalk,omitempty"` // 钉钉
	Lark     RobotLarkConfig     `yaml:"lark,omitempty" json:"lark,omitempty"`         // 飞书
}

// RobotSessionConfig 机器人会话隔离策略
type RobotSessionConfig struct {
	StrictUserIdentity *bool `yaml:"strict_user_identity,omitempty" json:"strict_user_identity,omitempty"` // true 时只允许真实用户标识，不允许会话/群 ID 兜底
}

// StrictUserIdentityEnabled 返回是否启用严格用户身份模式；未配置时默认 true。
func (c RobotSessionConfig) StrictUserIdentityEnabled() bool {
	if c.StrictUserIdentity == nil {
		return true
	}
	return *c.StrictUserIdentity
}

// RobotWecomConfig 企业微信机器人配置
type RobotWecomConfig struct {
	Enabled        bool   `yaml:"enabled" json:"enabled"`
	Token          string `yaml:"token" json:"token"`                       // 回调 URL 校验 Token
	EncodingAESKey string `yaml:"encoding_aes_key" json:"encoding_aes_key"` // EncodingAESKey
	CorpID         string `yaml:"corp_id" json:"corp_id"`                   // 企业 ID
	Secret         string `yaml:"secret" json:"secret"`                     // 应用 Secret
	AgentID        int64  `yaml:"agent_id" json:"agent_id"`                 // 应用 AgentId
}

// RobotDingtalkConfig 钉钉机器人配置
type RobotDingtalkConfig struct {
	Enabled                    bool   `yaml:"enabled" json:"enabled"`
	ClientID                   string `yaml:"client_id" json:"client_id"`                                       // 应用 Key (AppKey)
	ClientSecret               string `yaml:"client_secret" json:"client_secret"`                               // 应用 Secret
	AllowConversationIDFallback bool   `yaml:"allow_conversation_id_fallback" json:"allow_conversation_id_fallback"` // sender_id 缺失时是否允许回退到会话 ID
}

// RobotLarkConfig 飞书机器人配置
type RobotLarkConfig struct {
	Enabled                 bool   `yaml:"enabled" json:"enabled"`
	AppID                   string `yaml:"app_id" json:"app_id"`                                 // 应用 App ID
	AppSecret               string `yaml:"app_secret" json:"app_secret"`                         // 应用 App Secret
	VerifyToken             string `yaml:"verify_token" json:"verify_token"`                     // 事件订阅 Verification Token（可选）
	AllowChatIDFallback     bool   `yaml:"allow_chat_id_fallback" json:"allow_chat_id_fallback"` // 用户 ID 缺失时是否允许回退到 chat_id
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type LogConfig struct {
	Level  string `yaml:"level"`
	Output string `yaml:"output"`
}

type MCPConfig struct {
	Enabled         bool   `yaml:"enabled"`
	Host            string `yaml:"host"`
	Port            int    `yaml:"port"`
	AuthHeader      string `yaml:"auth_header,omitempty"`       // 鉴权 header 名，留空表示不鉴权
	AuthHeaderValue string `yaml:"auth_header_value,omitempty"` // 鉴权 header 值，需与请求中该 header 一致
}

type OpenAIConfig struct {
	Provider       string `yaml:"provider,omitempty" json:"provider,omitempty"` // API 提供商: "openai"(默认) 或 "claude"，claude 时自动桥接为 Anthropic Messages API
	APIKey         string `yaml:"api_key" json:"api_key"`
	BaseURL        string `yaml:"base_url" json:"base_url"`
	Model          string `yaml:"model" json:"model"`
	MaxTotalTokens int    `yaml:"max_total_tokens,omitempty" json:"max_total_tokens,omitempty"`
}

type FofaConfig struct {
	// Email 为 FOFA 账号邮箱；APIKey 为 FOFA API Key（建议使用只读权限的 Key）
	Email   string `yaml:"email,omitempty" json:"email,omitempty"`
	APIKey  string `yaml:"api_key,omitempty" json:"api_key,omitempty"`
	BaseURL string `yaml:"base_url,omitempty" json:"base_url,omitempty"` // 默认 https://fofa.info/api/v1/search/all
}

type SecurityConfig struct {
	Tools               []ToolConfig `yaml:"tools,omitempty"`                 // 向后兼容：支持在主配置文件中定义工具
	ToolsDir            string       `yaml:"tools_dir,omitempty"`             // 工具配置文件目录（新方式）
	ToolDescriptionMode string       `yaml:"tool_description_mode,omitempty"` // 工具描述模式: "short" | "full"，默认 short
}

type DatabaseConfig struct {
	Path            string `yaml:"path"`                        // 会话数据库路径
	KnowledgeDBPath string `yaml:"knowledge_db_path,omitempty"` // 知识库数据库路径（可选，为空则使用会话数据库）
}

type AgentConfig struct {
	MaxIterations        int    `yaml:"max_iterations" json:"max_iterations"`
	LargeResultThreshold int    `yaml:"large_result_threshold" json:"large_result_threshold"` // 大结果阈值（字节），默认50KB
	ResultStorageDir     string `yaml:"result_storage_dir" json:"result_storage_dir"`         // 结果存储目录，默认tmp
	ToolTimeoutMinutes   int    `yaml:"tool_timeout_minutes" json:"tool_timeout_minutes"`     // 单次工具执行最大时长（分钟），超时自动终止，防止长时间挂起；0 表示不限制（不推荐）
	// SystemPromptPath 单代理系统提示 Markdown/文本文件路径（相对 config.yaml 所在目录，或可写绝对路径）。非空且可读时替换内置单代理提示；留空用内置。
	SystemPromptPath string `yaml:"system_prompt_path,omitempty" json:"system_prompt_path,omitempty"`
}

// HitlConfig 人机协同全局选项；与会话侧栏/API 中的白名单合并为并集后参与判定。
// tool_whitelist 可在侧栏「应用」时合并写入 config.yaml 并立即生效；其他字段若仅改文件仍需重启。
type HitlConfig struct {
	// ToolWhitelist 全局免审批工具名（与每条会话配置的 sensitiveTools 语义相同：白名单内工具不触发 HITL）。
	ToolWhitelist []string `yaml:"tool_whitelist,omitempty" json:"tool_whitelist,omitempty"`
}

type AuthConfig struct {
	Password                    string `yaml:"password" json:"password"`
	SessionDurationHours        int    `yaml:"session_duration_hours" json:"session_duration_hours"`
	GeneratedPassword           string `yaml:"-" json:"-"`
	GeneratedPasswordPersisted  bool   `yaml:"-" json:"-"`
	GeneratedPasswordPersistErr string `yaml:"-" json:"-"`
}

// ExternalMCPConfig 外部MCP配置
type ExternalMCPConfig struct {
	Servers map[string]ExternalMCPServerConfig `yaml:"servers,omitempty" json:"servers,omitempty"`
}

// ExternalMCPServerConfig 外部MCP服务器配置（遵循官方 MCP 配置格式，兼容 Claude Desktop / Cursor / VS Code）。
// 所有字符串字段均支持 ${VAR} 和 ${VAR:-default} 环境变量展开语法。
type ExternalMCPServerConfig struct {
	// 传输类型: "stdio" | "sse" | "http"（Streamable HTTP）。
	// stdio 模式可省略，有 command 字段时自动推断。
	Type string `yaml:"type,omitempty" json:"type,omitempty"`

	// stdio 模式配置
	Command string            `yaml:"command,omitempty" json:"command,omitempty"`
	Args    []string          `yaml:"args,omitempty" json:"args,omitempty"`
	Env     map[string]string `yaml:"env,omitempty" json:"env,omitempty"`

	// HTTP/SSE 模式配置
	URL     string            `yaml:"url,omitempty" json:"url,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`

	// 官方标准字段
	Disabled    bool     `yaml:"disabled,omitempty" json:"disabled,omitempty"`       // 禁用服务器（官方字段）
	AutoApprove []string `yaml:"autoApprove,omitempty" json:"autoApprove,omitempty"` // 自动批准的工具列表（官方字段）

	// SDK 高级配置（对应 MCP Go SDK 传输层参数）
	MaxRetries        int `yaml:"max_retries,omitempty" json:"max_retries,omitempty"`               // Streamable HTTP 断线重连次数（默认 5）
	TerminateDuration int `yaml:"terminate_duration,omitempty" json:"terminate_duration,omitempty"` // stdio 进程优雅关闭等待秒数（默认 5）
	KeepAlive         int `yaml:"keep_alive,omitempty" json:"keep_alive,omitempty"`                 // 客户端心跳间隔秒数（0 = 禁用）

	// 通用配置
	Description       string          `yaml:"description,omitempty" json:"description,omitempty"`
	Timeout           int             `yaml:"timeout,omitempty" json:"timeout,omitempty"`                         // 连接超时（秒）
	ExternalMCPEnable bool            `yaml:"external_mcp_enable,omitempty" json:"external_mcp_enable,omitempty"` // 是否启用
	ToolEnabled       map[string]bool `yaml:"tool_enabled,omitempty" json:"tool_enabled,omitempty"`               // 每个工具的启用状态
}

// GetTransportType 返回实际传输类型。优先读 Type，否则根据 Command/URL 自动推断。
func (c ExternalMCPServerConfig) GetTransportType() string {
	if c.Type != "" {
		return c.Type
	}
	if c.Command != "" {
		return "stdio"
	}
	if c.URL != "" {
		return "http"
	}
	return ""
}

type ToolConfig struct {
	Name             string            `yaml:"name"`
	Command          string            `yaml:"command"`
	Args             []string          `yaml:"args,omitempty"`              // 固定参数（可选）
	ShortDescription string            `yaml:"short_description,omitempty"` // 简短描述（用于工具列表，减少token消耗）
	Description      string            `yaml:"description"`                 // 详细描述（用于工具文档）
	Enabled          bool              `yaml:"enabled"`
	Parameters       []ParameterConfig `yaml:"parameters,omitempty"`         // 参数定义（可选）
	ArgMapping       string            `yaml:"arg_mapping,omitempty"`        // 参数映射方式: "auto", "manual", "template"（可选）
	AllowedExitCodes []int             `yaml:"allowed_exit_codes,omitempty"` // 允许的退出码列表（某些工具在成功时也返回非零退出码）
}

// ParameterConfig 参数配置
type ParameterConfig struct {
	Name        string      `yaml:"name"`                // 参数名称
	Type        string      `yaml:"type"`                // 参数类型: string, int, bool, array
	Description string      `yaml:"description"`         // 参数描述
	Required    bool        `yaml:"required,omitempty"`  // 是否必需
	Default     interface{} `yaml:"default,omitempty"`   // 默认值
	ItemType    string      `yaml:"item_type,omitempty"` // 当 type 为 array 时，数组元素类型，如 string, number, object
	Flag        string      `yaml:"flag,omitempty"`      // 命令行标志，如 "-u", "--url", "-p"
	Position    *int        `yaml:"position,omitempty"`  // 位置参数的位置（从0开始）
	Format      string      `yaml:"format,omitempty"`    // 参数格式: "flag", "positional", "combined" (flag=value), "template"
	Template    string      `yaml:"template,omitempty"`  // 模板字符串，如 "{flag} {value}" 或 "{value}"
	Options     []string    `yaml:"options,omitempty"`   // 可选值列表（用于枚举）
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	if cfg.Auth.SessionDurationHours <= 0 {
		cfg.Auth.SessionDurationHours = 12
	}
	if strings.TrimSpace(cfg.Auth.Password) == "" {
		password, err := generateStrongPassword(24)
		if err != nil {
			return nil, fmt.Errorf("生成默认密码失败: %w", err)
		}

		cfg.Auth.Password = password
		cfg.Auth.GeneratedPassword = password

		if err := PersistAuthPassword(path, password); err != nil {
			cfg.Auth.GeneratedPasswordPersisted = false
			cfg.Auth.GeneratedPasswordPersistErr = err.Error()
		} else {
			cfg.Auth.GeneratedPasswordPersisted = true
		}
	}

	// 如果配置了工具目录，从目录加载工具配置
	if cfg.Security.ToolsDir != "" {
		configDir := filepath.Dir(path)
		toolsDir := cfg.Security.ToolsDir

		// 如果是相对路径，相对于配置文件所在目录
		if !filepath.IsAbs(toolsDir) {
			toolsDir = filepath.Join(configDir, toolsDir)
		}

		tools, err := LoadToolsFromDir(toolsDir)
		if err != nil {
			return nil, fmt.Errorf("从工具目录加载工具配置失败: %w", err)
		}

		// 合并工具配置：目录中的工具优先，主配置中的工具作为补充
		existingTools := make(map[string]bool)
		for _, tool := range tools {
			existingTools[tool.Name] = true
		}

		// 添加主配置中不存在于目录中的工具（向后兼容）
		for _, tool := range cfg.Security.Tools {
			if !existingTools[tool.Name] {
				tools = append(tools, tool)
			}
		}

		cfg.Security.Tools = tools
	}

	// 外部 MCP：迁移 + 环境变量展开
	if cfg.ExternalMCP.Servers != nil {
		for name, serverCfg := range cfg.ExternalMCP.Servers {
			// 官方 disabled 字段 → ExternalMCPEnable
			if serverCfg.Disabled {
				serverCfg.ExternalMCPEnable = false
			} else if !serverCfg.ExternalMCPEnable {
				// 默认启用
				serverCfg.ExternalMCPEnable = true
			}

			// 展开所有 ${VAR} / ${VAR:-default} 环境变量引用
			ExpandConfigEnv(&serverCfg)

			cfg.ExternalMCP.Servers[name] = serverCfg
		}
	}

	// 从角色目录加载角色配置
	if cfg.RolesDir != "" {
		configDir := filepath.Dir(path)
		rolesDir := cfg.RolesDir

		// 如果是相对路径，相对于配置文件所在目录
		if !filepath.IsAbs(rolesDir) {
			rolesDir = filepath.Join(configDir, rolesDir)
		}

		roles, err := LoadRolesFromDir(rolesDir)
		if err != nil {
			return nil, fmt.Errorf("从角色目录加载角色配置失败: %w", err)
		}

		cfg.Roles = roles
	} else {
		// 如果未配置 roles_dir，初始化为空 map
		if cfg.Roles == nil {
			cfg.Roles = make(map[string]RoleConfig)
		}
	}

	return &cfg, nil
}

func generateStrongPassword(length int) (string, error) {
	if length <= 0 {
		length = 24
	}

	bytesLen := length
	randomBytes := make([]byte, bytesLen)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}

	password := base64.RawURLEncoding.EncodeToString(randomBytes)
	if len(password) > length {
		password = password[:length]
	}
	return password, nil
}

func PersistAuthPassword(path, password string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	inAuthBlock := false
	authIndent := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inAuthBlock {
			if strings.HasPrefix(trimmed, "auth:") {
				inAuthBlock = true
				authIndent = len(line) - len(strings.TrimLeft(line, " "))
			}
			continue
		}

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		leadingSpaces := len(line) - len(strings.TrimLeft(line, " "))
		if leadingSpaces <= authIndent {
			// 离开 auth 块
			inAuthBlock = false
			authIndent = -1
			// 继续寻找其它 auth 块（理论上没有）
			if strings.HasPrefix(trimmed, "auth:") {
				inAuthBlock = true
				authIndent = leadingSpaces
			}
			continue
		}

		if strings.HasPrefix(strings.TrimSpace(line), "password:") {
			prefix := line[:len(line)-len(strings.TrimLeft(line, " "))]
			comment := ""
			if idx := strings.Index(line, "#"); idx >= 0 {
				comment = strings.TrimRight(line[idx:], " ")
			}

			newLine := fmt.Sprintf("%spassword: %s", prefix, password)
			if comment != "" {
				if !strings.HasPrefix(comment, " ") {
					newLine += " "
				}
				newLine += comment
			}
			lines[i] = newLine
			break
		}
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}

func PrintGeneratedPasswordWarning(password string, persisted bool, persistErr string) {
	if strings.TrimSpace(password) == "" {
		return
	}

	if persisted {
		fmt.Println("[CyberStrikeAI] ✅ 已为您自动生成并写入 Web 登录密码。")
	} else {
		if persistErr != "" {
			fmt.Printf("[CyberStrikeAI] ⚠️ 无法自动写入配置文件中的密码: %s\n", persistErr)
		} else {
			fmt.Println("[CyberStrikeAI] ⚠️ 无法自动写入配置文件中的密码。")
		}
		fmt.Println("请手动将以下随机密码写入 config.yaml 的 auth.password：")
	}

	fmt.Println("----------------------------------------------------------------")
	fmt.Println("CyberStrikeAI Auto-Generated Web Password")
	fmt.Printf("Password: %s\n", password)
	fmt.Println("WARNING: Anyone with this password can fully control CyberStrikeAI.")
	fmt.Println("Please store it securely and change it in config.yaml as soon as possible.")
	fmt.Println("警告：持有此密码的人将拥有对 CyberStrikeAI 的完全控制权限。")
	fmt.Println("请妥善保管，并尽快在 config.yaml 中修改 auth.password！")
	fmt.Println("----------------------------------------------------------------")
}

// generateRandomToken 生成用于 MCP 鉴权的随机字符串（64 位十六进制）
func generateRandomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// persistMCPAuth 将 MCP 的 auth_header / auth_header_value 写回配置文件
func persistMCPAuth(path string, mcp *MCPConfig) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	inMcpBlock := false
	mcpIndent := -1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inMcpBlock {
			if strings.HasPrefix(trimmed, "mcp:") {
				inMcpBlock = true
				mcpIndent = len(line) - len(strings.TrimLeft(line, " "))
			}
			continue
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		leadingSpaces := len(line) - len(strings.TrimLeft(line, " "))
		if leadingSpaces <= mcpIndent {
			inMcpBlock = false
			mcpIndent = -1
			if strings.HasPrefix(trimmed, "mcp:") {
				inMcpBlock = true
				mcpIndent = leadingSpaces
			}
			continue
		}

		prefix := line[:leadingSpaces]
		rest := strings.TrimSpace(line[leadingSpaces:])
		comment := ""
		if idx := strings.Index(line, "#"); idx >= 0 {
			comment = strings.TrimRight(line[idx:], " ")
		}
		withComment := ""
		if comment != "" {
			if !strings.HasPrefix(comment, " ") {
				withComment = " "
			}
			withComment += comment
		}

		if strings.HasPrefix(rest, "auth_header_value:") {
			lines[i] = fmt.Sprintf("%sauth_header_value: %q%s", prefix, mcp.AuthHeaderValue, withComment)
		} else if strings.HasPrefix(rest, "auth_header:") {
			lines[i] = fmt.Sprintf("%sauth_header: %q%s", prefix, mcp.AuthHeader, withComment)
		}
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}

// EnsureMCPAuth 在 MCP 启用且 auth_header_value 为空时，自动生成随机密钥并写回配置
func EnsureMCPAuth(path string, cfg *Config) error {
	if !cfg.MCP.Enabled || strings.TrimSpace(cfg.MCP.AuthHeaderValue) != "" {
		return nil
	}
	token, err := generateRandomToken()
	if err != nil {
		return fmt.Errorf("生成 MCP 鉴权密钥失败: %w", err)
	}
	cfg.MCP.AuthHeaderValue = token
	if strings.TrimSpace(cfg.MCP.AuthHeader) == "" {
		cfg.MCP.AuthHeader = "X-MCP-Token"
	}
	return persistMCPAuth(path, &cfg.MCP)
}

// PrintMCPConfigJSON 向终端输出 MCP 配置的 JSON，可直接复制到 Cursor / Claude Code 的 mcp 配置中使用
func PrintMCPConfigJSON(mcp MCPConfig) {
	if !mcp.Enabled {
		return
	}
	hostForURL := strings.TrimSpace(mcp.Host)
	if hostForURL == "" || hostForURL == "0.0.0.0" {
		hostForURL = "localhost"
	}
	url := fmt.Sprintf("http://%s:%d/mcp", hostForURL, mcp.Port)
	headers := map[string]string{}
	if mcp.AuthHeader != "" {
		headers[mcp.AuthHeader] = mcp.AuthHeaderValue
	}
	serverEntry := map[string]interface{}{
		"url": url,
	}
	if len(headers) > 0 {
		serverEntry["headers"] = headers
	}
	// Claude Code 需要 type: "http"
	serverEntry["type"] = "http"
	out := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"cyberstrike-ai": serverEntry,
		},
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println("[CyberStrikeAI] MCP 配置（可复制到 Cursor / Claude Code 使用）：")
	fmt.Println("  Cursor: 放入 ~/.cursor/mcp.json 的 mcpServers，或项目 .cursor/mcp.json")
	fmt.Println("  Claude Code: 放入 .mcp.json 或 ~/.claude.json 的 mcpServers")
	fmt.Println("----------------------------------------------------------------")
	fmt.Println(string(b))
	fmt.Println("----------------------------------------------------------------")
}

// LoadToolsFromDir 从目录加载所有工具配置文件
func LoadToolsFromDir(dir string) ([]ToolConfig, error) {
	var tools []ToolConfig

	// 检查目录是否存在
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return tools, nil // 目录不存在时返回空列表，不报错
	}

	// 读取目录中的所有 .yaml 和 .yml 文件
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("读取工具目录失败: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		filePath := filepath.Join(dir, name)
		tool, err := LoadToolFromFile(filePath)
		if err != nil {
			// 记录错误但继续加载其他文件
			fmt.Printf("警告: 加载工具配置文件 %s 失败: %v\n", filePath, err)
			continue
		}

		tools = append(tools, *tool)
	}

	return tools, nil
}

// LoadToolFromFile 从单个文件加载工具配置
func LoadToolFromFile(path string) (*ToolConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	var tool ToolConfig
	if err := yaml.Unmarshal(data, &tool); err != nil {
		return nil, fmt.Errorf("解析工具配置失败: %w", err)
	}

	// 验证必需字段
	if tool.Name == "" {
		return nil, fmt.Errorf("工具名称不能为空")
	}
	if tool.Command == "" {
		return nil, fmt.Errorf("工具命令不能为空")
	}

	return &tool, nil
}

// LoadRolesFromDir 从目录加载所有角色配置文件
func LoadRolesFromDir(dir string) (map[string]RoleConfig, error) {
	roles := make(map[string]RoleConfig)

	// 检查目录是否存在
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return roles, nil // 目录不存在时返回空map，不报错
	}

	// 读取目录中的所有 .yaml 和 .yml 文件
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("读取角色目录失败: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		filePath := filepath.Join(dir, name)
		role, err := LoadRoleFromFile(filePath)
		if err != nil {
			// 记录错误但继续加载其他文件
			fmt.Printf("警告: 加载角色配置文件 %s 失败: %v\n", filePath, err)
			continue
		}

		// 使用角色名称作为key
		roleName := role.Name
		if roleName == "" {
			// 如果角色名称为空，使用文件名（去掉扩展名）作为名称
			roleName = strings.TrimSuffix(strings.TrimSuffix(name, ".yaml"), ".yml")
			role.Name = roleName
		}

		roles[roleName] = *role
	}

	return roles, nil
}

// LoadRoleFromFile 从单个文件加载角色配置
func LoadRoleFromFile(path string) (*RoleConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	var role RoleConfig
	if err := yaml.Unmarshal(data, &role); err != nil {
		return nil, fmt.Errorf("解析角色配置失败: %w", err)
	}

	// 处理 icon 字段：如果包含 Unicode 转义格式（\U0001F3C6），转换为实际的 Unicode 字符
	// Go 的 yaml 库可能不会自动解析 \U 转义序列，需要手动转换
	if role.Icon != "" {
		icon := role.Icon
		// 去除可能的引号
		icon = strings.Trim(icon, `"`)

		// 检查是否是 Unicode 转义格式 \U0001F3C6（8位十六进制）或 \uXXXX（4位十六进制）
		if len(icon) >= 3 && icon[0] == '\\' {
			if icon[1] == 'U' && len(icon) >= 10 {
				// \U0001F3C6 格式（8位十六进制）
				if codePoint, err := strconv.ParseInt(icon[2:10], 16, 32); err == nil {
					role.Icon = string(rune(codePoint))
				}
			} else if icon[1] == 'u' && len(icon) >= 6 {
				// \uXXXX 格式（4位十六进制）
				if codePoint, err := strconv.ParseInt(icon[2:6], 16, 32); err == nil {
					role.Icon = string(rune(codePoint))
				}
			}
		}
	}

	// 验证必需字段
	if role.Name == "" {
		// 如果名称为空，尝试从文件名获取
		baseName := filepath.Base(path)
		role.Name = strings.TrimSuffix(strings.TrimSuffix(baseName, ".yaml"), ".yml")
	}

	return &role, nil
}

func Default() *Config {
	strictRobotIdentity := true
	return &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Log: LogConfig{
			Level:  "info",
			Output: "stdout",
		},
		MCP: MCPConfig{
			Enabled: true,
			Host:    "0.0.0.0",
			Port:    8081,
		},
		OpenAI: OpenAIConfig{
			BaseURL:        "https://api.openai.com/v1",
			Model:          "gpt-4",
			MaxTotalTokens: 120000,
		},
		Agent: AgentConfig{
			MaxIterations:      30, // 默认最大迭代次数
			ToolTimeoutMinutes: 10, // 单次工具执行默认最多 10 分钟，避免异常长时间占用
		},
		Security: SecurityConfig{
			Tools:    []ToolConfig{}, // 工具配置应该从 config.yaml 或 tools/ 目录加载
			ToolsDir: "tools",        // 默认工具目录
		},
		Database: DatabaseConfig{
			Path:            "data/conversations.db",
			KnowledgeDBPath: "data/knowledge.db", // 默认知识库数据库路径
		},
		Auth: AuthConfig{
			SessionDurationHours: 12,
		},
		Robots: RobotsConfig{
			Session: RobotSessionConfig{
				StrictUserIdentity: &strictRobotIdentity,
			},
		},
		Knowledge: KnowledgeConfig{
			Enabled:  true,
			BasePath: "knowledge_base",
			Embedding: EmbeddingConfig{
				Provider: "openai",
				Model:    "text-embedding-3-small",
				BaseURL:  "https://api.openai.com/v1",
			},
			Retrieval: RetrievalConfig{
				TopK:                5,
				SimilarityThreshold: 0.65, // 降低阈值到 0.65，减少漏检
			},
			Indexing: IndexingConfig{
				ChunkStrategy:         "markdown_then_recursive",
				RequestTimeoutSeconds: 120,
				ChunkSize:             768, // 增加到 768，更好的上下文保持
				ChunkOverlap:          50,
				MaxChunksPerItem:      20, // 限制单个知识项最多 20 个块，避免消耗过多配额
				BatchSize:             64,
				PreferSourceFile:      false,
				MaxRPM:                100, // 默认 100 RPM，避免 429 错误
				RateLimitDelayMs:      600, // 600ms 间隔，对应 100 RPM
				MaxRetries:            3,
				RetryDelayMs:          1000,
				SubIndexes:            nil,
			},
		},
	}
}

// C2Config 内置 C2 模块开关（与知识库 enabled 语义一致：关闭后不初始化监听器、不注册 C2 MCP 工具）。
type C2Config struct {
	// Enabled 为 nil 表示未写配置，按 true 处理（兼容旧 config.yaml）
	Enabled *bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`
}

// EnabledEffective 返回是否启用 C2；未显式配置时默认启用。
func (c C2Config) EnabledEffective() bool {
	if c.Enabled == nil {
		return true
	}
	return *c.Enabled
}

// C2Public 返回给前端的 C2 状态（仅标量）。
type C2Public struct {
	Enabled bool `json:"enabled"`
}

// Public 将内部配置转为 API 响应。
func (c C2Config) Public() C2Public {
	return C2Public{Enabled: c.EnabledEffective()}
}

// C2APIUpdate 设置页/API 更新 C2 开关。
type C2APIUpdate struct {
	Enabled bool `json:"enabled"`
}

// KnowledgeConfig 知识库配置
type KnowledgeConfig struct {
	Enabled   bool            `yaml:"enabled" json:"enabled"`     // 是否启用知识检索
	BasePath  string          `yaml:"base_path" json:"base_path"` // 知识库路径
	Embedding EmbeddingConfig `yaml:"embedding" json:"embedding"`
	Retrieval RetrievalConfig `yaml:"retrieval" json:"retrieval"`
	Indexing  IndexingConfig  `yaml:"indexing,omitempty" json:"indexing,omitempty"` // 索引构建配置
}

// IndexingConfig 索引构建配置（用于控制知识库索引构建时的行为）
type IndexingConfig struct {
	// ChunkStrategy: "markdown_then_recursive"（默认，Eino Markdown 标题切分后再递归切）或 "recursive"（仅递归切分）
	ChunkStrategy string `yaml:"chunk_strategy,omitempty" json:"chunk_strategy,omitempty"`
	// RequestTimeoutSeconds 嵌入 HTTP 客户端超时（秒），0 表示使用默认 120
	RequestTimeoutSeconds int `yaml:"request_timeout_seconds,omitempty" json:"request_timeout_seconds,omitempty"`
	// 分块配置
	ChunkSize        int `yaml:"chunk_size,omitempty" json:"chunk_size,omitempty"`                   // 每个块的最大 token 数（估算），默认 512
	ChunkOverlap     int `yaml:"chunk_overlap,omitempty" json:"chunk_overlap,omitempty"`             // 块之间的重叠 token 数，默认 50
	MaxChunksPerItem int `yaml:"max_chunks_per_item,omitempty" json:"max_chunks_per_item,omitempty"` // 单个知识项的最大块数量，0 表示不限制

	// PreferSourceFile 为 true 时优先用 Eino FileLoader 从 file_path 读原文再索引（与库内 content 不一致时以磁盘为准）
	PreferSourceFile bool `yaml:"prefer_source_file,omitempty" json:"prefer_source_file,omitempty"`

	// 速率限制配置（用于避免 API 速率限制）
	RateLimitDelayMs int `yaml:"rate_limit_delay_ms,omitempty" json:"rate_limit_delay_ms,omitempty"` // 请求间隔时间（毫秒），0 表示不使用固定延迟
	MaxRPM           int `yaml:"max_rpm,omitempty" json:"max_rpm,omitempty"`                         // 每分钟最大请求数，0 表示不限制

	// 重试配置（用于处理临时错误）
	MaxRetries   int `yaml:"max_retries,omitempty" json:"max_retries,omitempty"`       // 最大重试次数，默认 3
	RetryDelayMs int `yaml:"retry_delay_ms,omitempty" json:"retry_delay_ms,omitempty"` // 重试间隔（毫秒），默认 1000

	// BatchSize 嵌入批大小（SQLite 索引写入），0 表示默认 64
	BatchSize int `yaml:"batch_size,omitempty" json:"batch_size,omitempty"`
	// SubIndexes 传入 Eino indexer.WithSubIndexes（逻辑分区标记，随 Document 元数据传递）
	SubIndexes []string `yaml:"sub_indexes,omitempty" json:"sub_indexes,omitempty"`
}

// EmbeddingConfig 嵌入配置
type EmbeddingConfig struct {
	Provider string `yaml:"provider" json:"provider"` // 嵌入模型提供商
	Model    string `yaml:"model" json:"model"`       // 模型名称
	BaseURL  string `yaml:"base_url" json:"base_url"` // API Base URL
	APIKey   string `yaml:"api_key" json:"api_key"`   // API Key（从OpenAI配置继承）
}

// PostRetrieveConfig 检索后处理：固定对正文做规范化去重（最佳实践）、上下文预算截断；PrefetchTopK 用于多取候选再收敛到 top_k。
type PostRetrieveConfig struct {
	// PrefetchTopK 向量检索阶段最多保留的候选数（余弦序），应 ≥ top_k，0 表示与 top_k 相同；上限见知识库包内常量。
	PrefetchTopK int `yaml:"prefetch_top_k,omitempty" json:"prefetch_top_k,omitempty"`
	// MaxContextChars 返回文档内容总 Unicode 字符数上限（整段 chunk，不截断半段）；0 表示不限制。
	MaxContextChars int `yaml:"max_context_chars,omitempty" json:"max_context_chars,omitempty"`
	// MaxContextTokens 返回文档内容总 token 上限（tiktoken，按嵌入模型名映射，失败则 cl100k_base）；0 表示不限制。
	MaxContextTokens int `yaml:"max_context_tokens,omitempty" json:"max_context_tokens,omitempty"`
}

// RetrievalConfig 检索配置
type RetrievalConfig struct {
	TopK                int     `yaml:"top_k" json:"top_k"`                               // 检索Top-K
	SimilarityThreshold float64 `yaml:"similarity_threshold" json:"similarity_threshold"` // 余弦相似度阈值
	// SubIndexFilter 非空时仅保留 sub_indexes 含该标签（逗号分隔之一）的行；sub_indexes 为空的旧行仍返回。
	SubIndexFilter string `yaml:"sub_index_filter,omitempty" json:"sub_index_filter,omitempty"`
	// PostRetrieve 检索后处理（去重、预算截断）；重排通过代码注入 [knowledge.DocumentReranker]。
	PostRetrieve PostRetrieveConfig `yaml:"post_retrieve,omitempty" json:"post_retrieve,omitempty"`
}

// RolesConfig 角色配置（已废弃，使用 map[string]RoleConfig 替代）
// 保留此类型以兼容旧代码，但建议直接使用 map[string]RoleConfig
type RolesConfig struct {
	Roles map[string]RoleConfig `yaml:"roles,omitempty" json:"roles,omitempty"`
}

// RoleConfig 单个角色配置
type RoleConfig struct {
	Name        string   `yaml:"name" json:"name"`                       // 角色名称
	Description string   `yaml:"description" json:"description"`         // 角色描述
	UserPrompt  string   `yaml:"user_prompt" json:"user_prompt"`         // 用户提示词(追加到用户消息前)
	Icon        string   `yaml:"icon,omitempty" json:"icon,omitempty"`   // 角色图标（可选）
	Tools       []string `yaml:"tools,omitempty" json:"tools,omitempty"` // 关联的工具列表（toolKey格式，如 "toolName" 或 "mcpName::toolName"）
	MCPs        []string `yaml:"mcps,omitempty" json:"mcps,omitempty"`   // 向后兼容：关联的MCP服务器列表（已废弃，使用tools替代）
	Enabled     bool     `yaml:"enabled" json:"enabled"`                 // 是否启用
}
