package multiagent

import (
	"encoding/json"
	"fmt"
	"strings"
)

// CairnOutputPayload 是 Cairn 模式 Worker（Reason/Explore/Bootstrap）的结构化输出。
type CairnOutputPayload struct {
	Accepted bool           `json:"accepted"`
	Reason   string         `json:"reason,omitempty"`
	Data     map[string]any `json:"data,omitempty"`
}

// CairnFactPayload 是 Bootstrap/Explore 的 fact 提交。
type CairnFactPayload struct {
	Description string `json:"description"`
}

// CairnCompletePayload 是 Reason/Bootstrap 的 complete 提交。
type CairnCompletePayload struct {
	From        []string `json:"from"`
	Description string   `json:"description"`
}

// CairnIntentPayload 是 Reason 的 intent 提交。
type CairnIntentPayload struct {
	From        []string `json:"from"`
	Description string   `json:"description"`
	// CanonicalID 是 SQLite canonical intent ID（提交 DB 后回填，不参与 JSON 序列化）。
	CanonicalID string `json:"-"`
}

// ParseCairnOutput 解析 Worker 的结构化输出（提取 JSON 对象）。
func ParseCairnOutput(stdout string) (*CairnOutputPayload, error) {
	// 提取第一个 JSON 对象（可能包裹在 markdown 代码块或其他文本中）
	start := strings.Index(stdout, "{")
	if start < 0 {
		return nil, fmt.Errorf("no JSON object found in output")
	}
	depth := 0
	end := -1
	for i := start; i < len(stdout); i++ {
		switch stdout[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = i + 1
				break
			}
		}
	}
	if end < 0 {
		return nil, fmt.Errorf("unterminated JSON object in output")
	}
	var p CairnOutputPayload
	if err := json.Unmarshal([]byte(stdout[start:end]), &p); err != nil {
		return nil, fmt.Errorf("cairn output parse: %w", err)
	}
	return &p, nil
}

// ValidateReasonPayload 校验 Reason 任务的输出。
// 返回：kind（complete / intents / noop / rejected）、数据、错误
func ValidateReasonPayload(p *CairnOutputPayload, openIntentsEmpty bool, maxIntents int) (string, any, error) {
	if p == nil {
		return "", nil, fmt.Errorf("payload is nil")
	}
	if !p.Accepted {
		return "rejected", nil, nil
	}
	data := p.Data
	if data == nil {
		if openIntentsEmpty {
			return "", nil, fmt.Errorf("intents is required when open_intents is empty")
		}
		return "noop", nil, nil
	}

	// complete 优先
	if completeRaw, ok := data["complete"]; ok {
		completeJSON, err := json.Marshal(completeRaw)
		if err != nil {
			return "", nil, fmt.Errorf("complete marshal: %w", err)
		}
		var complete CairnCompletePayload
		if err := json.Unmarshal(completeJSON, &complete); err != nil {
			return "", nil, fmt.Errorf("complete parse: %w", err)
		}
		if len(complete.From) == 0 || complete.Description == "" {
			return "", nil, fmt.Errorf("complete.from and complete.description are required")
		}
		return "complete", &complete, nil
	}

	// intents
	if intentsRaw, ok := data["intents"]; ok {
		intentsJSON, err := json.Marshal(intentsRaw)
		if err != nil {
			return "", nil, fmt.Errorf("intents marshal: %w", err)
		}
		var intents []CairnIntentPayload
		if err := json.Unmarshal(intentsJSON, &intents); err != nil {
			return "", nil, fmt.Errorf("intents parse: %w", err)
		}
		if len(intents) == 0 && openIntentsEmpty {
			return "", nil, fmt.Errorf("intents must not be empty when open_intents is empty")
		}
		if maxIntents > 0 && len(intents) > maxIntents {
			intents = intents[:maxIntents]
		}
		for i, it := range intents {
			if it.Description == "" {
				return "", nil, fmt.Errorf("intent[%d].description is required", i)
			}
		}
		if len(intents) == 0 {
			return "noop", nil, nil
		}
		return "intents", intents, nil
	}

	// 兼容单 intent
	if intentRaw, ok := data["intent"]; ok {
		intentJSON, err := json.Marshal(intentRaw)
		if err != nil {
			return "", nil, fmt.Errorf("intent marshal: %w", err)
		}
		var intent CairnIntentPayload
		if err := json.Unmarshal(intentJSON, &intent); err != nil {
			return "", nil, fmt.Errorf("intent parse: %w", err)
		}
		if intent.Description == "" {
			return "", nil, fmt.Errorf("intent.description is required")
		}
		return "intents", []CairnIntentPayload{intent}, nil
	}

	if openIntentsEmpty {
		return "", nil, fmt.Errorf("intents is required when open_intents is empty")
	}
	return "noop", nil, nil
}

// ValidateBootstrapPayload 校验 Bootstrap 任务的输出。
// 返回：kind（complete / fact / rejected）、fact、complete、错误
func ValidateBootstrapPayload(p *CairnOutputPayload) (string, *CairnFactPayload, *CairnCompletePayload, error) {
	if p == nil {
		return "", nil, nil, fmt.Errorf("payload is nil")
	}
	if !p.Accepted {
		return "rejected", nil, nil, nil
	}
	data := p.Data
	if data == nil {
		return "", nil, nil, fmt.Errorf("data is required")
	}

	var fact *CairnFactPayload
	if factRaw, ok := data["fact"]; ok {
		factJSON, err := json.Marshal(factRaw)
		if err != nil {
			return "", nil, nil, fmt.Errorf("fact marshal: %w", err)
		}
		var f CairnFactPayload
		if err := json.Unmarshal(factJSON, &f); err != nil {
			return "", nil, nil, fmt.Errorf("fact parse: %w", err)
		}
		if f.Description == "" {
			return "", nil, nil, fmt.Errorf("fact.description is required")
		}
		fact = &f
	}

	var complete *CairnCompletePayload
	if completeRaw, ok := data["complete"]; ok {
		completeJSON, err := json.Marshal(completeRaw)
		if err != nil {
			return "", nil, nil, fmt.Errorf("complete marshal: %w", err)
		}
		var c CairnCompletePayload
		if err := json.Unmarshal(completeJSON, &c); err != nil {
			return "", nil, nil, fmt.Errorf("complete parse: %w", err)
		}
		if len(c.From) == 0 || c.Description == "" {
			return "", nil, nil, fmt.Errorf("complete.from and complete.description are required")
		}
		complete = &c
	}

	if fact == nil {
		return "", nil, nil, fmt.Errorf("fact is required")
	}
	if complete != nil {
		return "complete", fact, complete, nil
	}
	return "fact", fact, nil, nil
}

// ValidateExplorePayload 校验 Explore 任务的输出。
// 返回：kind（fact / rejected）、fact、错误
func ValidateExplorePayload(p *CairnOutputPayload) (string, *CairnFactPayload, error) {
	if p == nil {
		return "", nil, fmt.Errorf("payload is nil")
	}
	if !p.Accepted {
		return "rejected", nil, nil
	}
	data := p.Data
	if data == nil {
		return "", nil, fmt.Errorf("data is required")
	}
	if descRaw, ok := data["description"]; ok {
		descJSON, err := json.Marshal(descRaw)
		if err != nil {
			return "", nil, fmt.Errorf("description marshal: %w", err)
		}
		var f CairnFactPayload
		if err := json.Unmarshal(descJSON, &f); err != nil {
			return "", nil, fmt.Errorf("description parse: %w", err)
		}
		if f.Description == "" {
			return "", nil, fmt.Errorf("description is required")
		}
		return "fact", &f, nil
	}
	return "", nil, fmt.Errorf("description is required")
}
