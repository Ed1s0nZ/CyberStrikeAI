package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"cyberstrike-ai/internal/config"
	openaiClient "cyberstrike-ai/internal/openai"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type FofaHandler struct {
	cfg          *config.Config
	logger       *zap.Logger
	client       *http.Client
	openAIClient *openaiClient.Client
}

func NewFofaHandler(cfg *config.Config, logger *zap.Logger) *FofaHandler {
	// LLM requests are generally slower than FOFA queries; give it a more lenient timeout.
	llmHTTPClient := &http.Client{Timeout: 2 * time.Minute}
	var llmCfg *config.OpenAIConfig
	if cfg != nil {
		llmCfg = &cfg.OpenAI
	}
	return &FofaHandler{
		cfg:          cfg,
		logger:       logger,
		client:       &http.Client{Timeout: 30 * time.Second},
		openAIClient: openaiClient.NewClient(llmCfg, llmHTTPClient, logger),
	}
}

type fofaSearchRequest struct {
	Query  string `json:"query" binding:"required"`
	Size   int    `json:"size,omitempty"`
	Page   int    `json:"page,omitempty"`
	Fields string `json:"fields,omitempty"`
	Full   bool   `json:"full,omitempty"`
}

type fofaParseRequest struct {
	Text string `json:"text" binding:"required"`
}

type fofaParseResponse struct {
	Query       string   `json:"query"`
	Explanation string   `json:"explanation,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
}

type fofaAPIResponse struct {
	Error   bool            `json:"error"`
	ErrMsg  string          `json:"errmsg"`
	Size    int             `json:"size"`
	Page    int             `json:"page"`
	Total   int             `json:"total"`
	Mode    string          `json:"mode"`
	Query   string          `json:"query"`
	Results [][]interface{} `json:"results"`
}

type fofaSearchResponse struct {
	Query        string                   `json:"query"`
	Size         int                      `json:"size"`
	Page         int                      `json:"page"`
	Total        int                      `json:"total"`
	Fields       []string                 `json:"fields"`
	ResultsCount int                      `json:"results_count"`
	Results      []map[string]interface{} `json:"results"`
}

func (h *FofaHandler) resolveCredentials() (email, apiKey string) {
	// Prefer environment variables (convenient for container deployment), then config file
	email = strings.TrimSpace(os.Getenv("FOFA_EMAIL"))
	apiKey = strings.TrimSpace(os.Getenv("FOFA_API_KEY"))
	if email != "" && apiKey != "" {
		return email, apiKey
	}
	if h.cfg != nil {
		if email == "" {
			email = strings.TrimSpace(h.cfg.FOFA.Email)
		}
		if apiKey == "" {
			apiKey = strings.TrimSpace(h.cfg.FOFA.APIKey)
		}
	}
	return email, apiKey
}

func (h *FofaHandler) resolveBaseURL() string {
	if h.cfg != nil {
		if v := strings.TrimSpace(h.cfg.FOFA.BaseURL); v != "" {
			return v
		}
	}
	return "https://fofa.info/api/v1/search/all"
}

// ParseNaturalLanguage parses natural language into FOFA query syntax (generates only, does not execute the query)
func (h *FofaHandler) ParseNaturalLanguage(c *gin.Context) {
	var req fofaParseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request parameters: " + err.Error()})
		return
	}
	req.Text = strings.TrimSpace(req.Text)
	if req.Text == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "text cannot be empty"})
		return
	}

	if h.cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "system configuration not initialized"})
		return
	}
	if strings.TrimSpace(h.cfg.OpenAI.APIKey) == "" || strings.TrimSpace(h.cfg.OpenAI.Model) == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "AI model not configured: please fill in openai.api_key and openai.model in system settings (supports OpenAI-compatible APIs such as DeepSeek)",
			"need":  []string{"openai.api_key", "openai.model"},
		})
		return
	}
	if h.openAIClient == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI client not initialized"})
		return
	}

	systemPrompt := strings.TrimSpace(`
You are a "FOFA Query Syntax Generator". Task: convert the natural language search intent provided by the user into FOFA query syntax.

Output requirements (very important):
1) Output JSON only (no markdown, no code blocks, no extra explanatory text)
2) JSON structure must be:
{
  "query": "string, FOFA query syntax (can be pasted directly into FOFA or the query box of this system)",
  "explanation": "string, optional, explains how you mapped fields/logic",
  "warnings": ["string"...] optional, lists ambiguities/risks/points requiring manual confirmation
}
3) If the user input is already FOFA query syntax (or an expression very close to FOFA syntax), return it "as-is" as query:
   - Do not rewrite field names, operators, or bracket structure
   - Do not rewrite any string values (especially geographic values); do not abbreviate/substitute synonyms/translate/transliterate

Query syntax essentials (from FOFA syntax reference):
- Logical connectors: && (AND), || (OR); use () around sub-expressions when necessary to confirm precedence (parentheses have highest precedence)
- When && and || appear at the same level (mixed), use () to clarify precedence (avoid ambiguity)
- Comparison/matching:
  - =  match; when field="" can query cases where "the field does not exist" or "value is empty"
  - == exact match; when field=="" can query cases where "the field exists and value is empty"
  - != does not match; when field!="" can query cases where "value is not empty"
  - *= fuzzy match; * or ? can be used for searching
- Entering a keyword directly (without a field) searches title, HTML content, HTTP headers, and URL fields; but when intent is clear, prefer field expressions (more controllable, more precise)

Field quick-reference examples (from user-provided cases, can be used directly/concatenated):
- Advanced search operator examples:
  - title="beijing"                    (= match)
  - title==""                          (== exact match, field exists and value is empty)
  - title=""                           (= match, may indicate field does not exist or value is empty)
  - title!=""                          (!= does not match, can be used for non-empty values)
  - title*="*Home*"                    (*= fuzzy match, use * or ?)
  - (app="Apache" || app="Nginx") && country="CN"   (use parentheses when mixing && / ||)
- General:
  - ip="1.1.1.1"
  - ip="220.181.111.1/24"
  - ip="2600:9000:202a:2600:18:4ab7:f600:93a1"
  - port="6379"
  - domain="qq.com"
  - host=".fofa.info"
  - os="centos"
  - server="Microsoft-IIS/10"
  - asn="19551"
  - org="LLC Baxet"
  - is_domain=true / is_domain=false
  - is_ipv6=true / is_ipv6=false
- Special Label:
  - app="Microsoft-Exchange"
  - fid="sSXXGNUO2FefBTcCLIT/2Q=="
  - product="NGINX"
  - product="Roundcube-Webmail" && product.version="1.6.10"
  - category="service"
  - type="service" / type="subdomain"
  - cloud_name="Aliyundun"
  - is_cloud=true / is_cloud=false
  - is_fraud=true / is_fraud=false
  - is_honeypot=true / is_honeypot=false
- Protocol (type=service):
  - protocol="quic"
  - banner="users"
  - banner_hash="7330105010150477363"
  - banner_fid="zRpqmn0FXQRjZpH8MjMX55zpMy9SgsW8"
  - base_protocol="udp" / base_protocol="tcp"
- Website (type=subdomain):
  - title="beijing"
  - header="elastic"
  - header_hash="1258854265"
  - body="cyberspace mapping"
  - body_hash="-2090962452"
  - js_name="js/jquery.js"
  - js_md5="82ac3f14327a8b7ba49baa208d4eaa15"
  - cname="customers.spektrix.com"
  - cname_domain="siteforce.com"
  - icon_hash="-247388890"
  - status_code="402"
  - icp="ICP030173"
  - sdk_hash="Are3qNnP2Eqn7q5kAoUO3l+w3mgVIytO"
- Location:
  - country="CN" or country="China"
  - region="Zhejiang" (only Chinese region names for China are supported)
  - city="Hangzhou"
- Certificate:
  - cert="baidu"
  - cert.subject="Oracle Corporation"
  - cert.issuer="DigiCert"
  - cert.subject.org="Oracle Corporation"
  - cert.subject.cn="baidu.com"
  - cert.issuer.org="cPanel, Inc."
  - cert.issuer.cn="Synology Inc. CA"
  - cert.domain="huawei.com"
  - cert.is_equal=true / cert.is_equal=false
  - cert.is_valid=true / cert.is_valid=false
  - cert.is_match=true / cert.is_match=false
  - cert.is_expired=true / cert.is_expired=false
  - jarm="2ad2ad0002ad2ad22c2ad2ad2ad2ad2eac92ec34bcc0cf7520e97547f83e81"
  - tls.version="TLS 1.3"
  - tls.ja3s="15af977ce25de452b96affa2addb1036"
  - cert.sn="356078156165546797850343536942784588840297"
  - cert.not_after.after="2025-03-01" / cert.not_after.before="2025-03-01"
  - cert.not_before.after="2025-03-01" / cert.not_before.before="2025-03-01"
- Last update time:
  - after="2023-01-01"
  - before="2023-12-01"
  - after="2023-01-01" && before="2023-12-01"
- Standalone IP syntax (requires ip_filter / ip_exclude):
  - ip_filter(banner="SSH-2.0-OpenSSH_6.7p2") && ip_filter(icon_hash="-1057022626")
  - ip_filter(banner="SSH-2.0-OpenSSH_6.7p2" && asn="3462") && ip_exclude(title="EdgeOS")
  - port_size="6" / port_size_gt="6" / port_size_lt="12"
  - ip_ports="80,161"
  - ip_country="CN"
  - ip_region="Zhejiang"
  - ip_city="Hangzhou"
  - ip_after="2021-03-18"
  - ip_before="2019-09-09"

Generation constraints and notes:
- Always wrap string values in English double quotes, e.g. title="login", country="CN"
- Keep string values literally consistent: do not abbreviate (e.g. city="beijing" should not become city="BJ"), do not use aliases (e.g. Beijing/Peking), do not arbitrarily translate/transliterate/change case
- Geographic fields (country/region/city) should prefer "outputting the value as given by the user"; when unsure of valid values, do not guess-put alternatives in warnings
- Do not fabricate non-existent FOFA fields; when unsure, put uncertain points in warnings and output a conservative query
- When the user's description has "multiple AND/OR conditions", prefer adding () to clarify precedence, e.g.: (app="Apache" || app="Nginx") && country="CN"
- When the user is missing key conditions causing the scope to be too broad or ambiguous (e.g. location/protocol/port/service type not specified), allow query to be an empty string and clearly state in warnings what information needs to be supplemented
`)

	userPrompt := fmt.Sprintf("Natural language intent: %s", req.Text)

	requestBody := map[string]interface{}{
		"model": h.cfg.OpenAI.Model,
		"messages": []map[string]interface{}{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.1,
		"max_tokens":  1200,
	}

	// OpenAI response structure: only need choices[0].message.content
	var apiResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 90*time.Second)
	defer cancel()

	if err := h.openAIClient.ChatCompletion(ctx, requestBody, &apiResponse); err != nil {
		var apiErr *openaiClient.APIError
		if errors.As(err, &apiErr) {
			h.logger.Warn("FOFA natural language parsing: LLM returned error", zap.Int("status", apiErr.StatusCode))
			c.JSON(http.StatusBadGateway, gin.H{"error": "AI parsing failed (upstream returned non-200), please check model configuration or try again later"})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": "AI parsing failed: " + err.Error()})
		return
	}
	if len(apiResponse.Choices) == 0 {
		c.JSON(http.StatusBadGateway, gin.H{"error": "AI did not return a valid result"})
		return
	}

	content := strings.TrimSpace(apiResponse.Choices[0].Message.Content)
	// handle cases where the model occasionally returns ```json ... ```
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var parsed fofaParseResponse
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		// pass back a snippet of the raw text for debugging, but avoid too much data
		snippet := content
		if len(snippet) > 1200 {
			snippet = snippet[:1200]
		}
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "AI response could not be parsed as JSON, please try again later or rephrase your description",
			"snippet": snippet,
		})
		return
	}
	parsed.Query = strings.TrimSpace(parsed.Query)
	if parsed.Query == "" {
		// query is allowed to be empty (indicates unclear requirements), but the frontend needs a clear prompt
		if len(parsed.Warnings) == 0 {
			parsed.Warnings = []string{"Insufficient information to generate a valid FOFA query syntax. Please provide key conditions (such as country/port/product/domain, etc.)."}
		}
	}

	c.JSON(http.StatusOK, parsed)
}

// Search FOFA query (backend proxy to avoid exposing the key on the frontend)
func (h *FofaHandler) Search(c *gin.Context) {
	var req fofaSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request parameters: " + err.Error()})
		return
	}

	req.Query = strings.TrimSpace(req.Query)
	if req.Query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "query cannot be empty"})
		return
	}
	if req.Size <= 0 {
		req.Size = 100
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	// FOFA API size limit is related to account permissions; apply a reasonable cap here
	if req.Size > 10000 {
		req.Size = 10000
	}
	if req.Fields == "" {
		req.Fields = "host,ip,port,domain,title,protocol,country,province,city,server"
	}

	email, apiKey := h.resolveCredentials()
	if email == "" || apiKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "FOFA not configured: please fill in FOFA Email/API Key in system settings, or set environment variables FOFA_EMAIL/FOFA_API_KEY",
			"need":    []string{"fofa.email", "fofa.api_key"},
			"env_key": []string{"FOFA_EMAIL", "FOFA_API_KEY"},
		})
		return
	}

	baseURL := h.resolveBaseURL()
	qb64 := base64.StdEncoding.EncodeToString([]byte(req.Query))

	u, err := url.Parse(baseURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "FOFA base_url is invalid: " + err.Error()})
		return
	}

	params := u.Query()
	params.Set("email", email)
	params.Set("key", apiKey)
	params.Set("qbase64", qb64)
	params.Set("size", fmt.Sprintf("%d", req.Size))
	params.Set("page", fmt.Sprintf("%d", req.Page))
	params.Set("fields", strings.TrimSpace(req.Fields))
	if req.Full {
		params.Set("full", "true")
	} else {
		// explicitly pass false for easier debugging
		params.Set("full", "false")
	}
	u.RawQuery = params.Encode()

	httpReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, u.String(), nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create request: " + err.Error()})
		return
	}

	resp, err := h.client.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "FOFA request failed: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("FOFA returned non-2xx: %d", resp.StatusCode)})
		return
	}

	var apiResp fofaAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to parse FOFA response: " + err.Error()})
		return
	}
	if apiResp.Error {
		msg := strings.TrimSpace(apiResp.ErrMsg)
		if msg == "" {
			msg = "FOFA returned an error"
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": msg})
		return
	}

	fields := splitAndCleanCSV(req.Fields)
	results := make([]map[string]interface{}, 0, len(apiResp.Results))
	for _, row := range apiResp.Results {
		item := make(map[string]interface{}, len(fields))
		for i, f := range fields {
			if i < len(row) {
				item[f] = row[i]
			} else {
				item[f] = nil
			}
		}
		results = append(results, item)
	}

	c.JSON(http.StatusOK, fofaSearchResponse{
		Query:        req.Query,
		Size:         apiResp.Size,
		Page:         apiResp.Page,
		Total:        apiResp.Total,
		Fields:       fields,
		ResultsCount: len(results),
		Results:      results,
	})
}

func splitAndCleanCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
