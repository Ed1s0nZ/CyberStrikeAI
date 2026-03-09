package handler

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"cyberstrike-ai/internal/config"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ReconHandler handles multi-engine recon search and key validation
type ReconHandler struct {
	cfg    *config.Config
	logger *zap.Logger
	client *http.Client
}

func NewReconHandler(cfg *config.Config, logger *zap.Logger) *ReconHandler {
	return &ReconHandler{
		cfg:    cfg,
		logger: logger,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// ── Validation endpoints ────────────────────────────────────────────────

type validateKeyResponse struct {
	Valid   bool   `json:"valid"`
	Error   string `json:"error,omitempty"`
	Info    string `json:"info,omitempty"`
	Engine  string `json:"engine"`
}

// ValidateFofaKey validates FOFA credentials by making a minimal query
func (h *ReconHandler) ValidateFofaKey(c *gin.Context) {
	var req struct {
		Email  string `json:"email"`
		APIKey string `json:"api_key"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, validateKeyResponse{Engine: "fofa", Error: "invalid request"})
		return
	}
	email := strings.TrimSpace(req.Email)
	apiKey := strings.TrimSpace(req.APIKey)
	if email == "" || apiKey == "" {
		c.JSON(http.StatusOK, validateKeyResponse{Engine: "fofa", Error: "email and api_key are required"})
		return
	}

	// Minimal FOFA query to validate credentials
	qb64 := base64.StdEncoding.EncodeToString([]byte("ip=1.1.1.1"))
	url := fmt.Sprintf("https://fofa.info/api/v1/search/all?email=%s&key=%s&qbase64=%s&size=1", email, apiKey, qb64)
	resp, err := h.client.Get(url)
	if err != nil {
		c.JSON(http.StatusOK, validateKeyResponse{Engine: "fofa", Error: "connection failed: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	var result struct {
		Error  bool   `json:"error"`
		ErrMsg string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.JSON(http.StatusOK, validateKeyResponse{Engine: "fofa", Error: "invalid response"})
		return
	}
	if result.Error {
		c.JSON(http.StatusOK, validateKeyResponse{Engine: "fofa", Error: result.ErrMsg})
		return
	}
	c.JSON(http.StatusOK, validateKeyResponse{Engine: "fofa", Valid: true, Info: "credentials verified"})
}

// ValidateZoomEyeKey validates a ZoomEye API key
func (h *ReconHandler) ValidateZoomEyeKey(c *gin.Context) {
	var req struct {
		APIKey string `json:"api_key"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, validateKeyResponse{Engine: "zoomeye", Error: "invalid request"})
		return
	}
	apiKey := strings.TrimSpace(req.APIKey)
	if apiKey == "" {
		c.JSON(http.StatusOK, validateKeyResponse{Engine: "zoomeye", Error: "api_key is required"})
		return
	}

	qb64 := base64.StdEncoding.EncodeToString([]byte("port:80"))
	body := fmt.Sprintf(`{"qbase64":"%s","page":1,"pagesize":1}`, qb64)
	httpReq, _ := http.NewRequestWithContext(c.Request.Context(), http.MethodPost,
		"https://api.zoomeye.ai/v2/search", strings.NewReader(body))
	httpReq.Header.Set("API-KEY", apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := h.client.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusOK, validateKeyResponse{Engine: "zoomeye", Error: "connection failed: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.JSON(http.StatusOK, validateKeyResponse{Engine: "zoomeye", Error: "invalid response"})
		return
	}
	if result.Code != 60000 {
		c.JSON(http.StatusOK, validateKeyResponse{Engine: "zoomeye", Error: result.Message})
		return
	}
	c.JSON(http.StatusOK, validateKeyResponse{Engine: "zoomeye", Valid: true, Info: "API key verified"})
}

// ValidateShodanKey validates a Shodan API key via /api-info
func (h *ReconHandler) ValidateShodanKey(c *gin.Context) {
	var req struct {
		APIKey string `json:"api_key"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, validateKeyResponse{Engine: "shodan", Error: "invalid request"})
		return
	}
	apiKey := strings.TrimSpace(req.APIKey)
	if apiKey == "" {
		c.JSON(http.StatusOK, validateKeyResponse{Engine: "shodan", Error: "api_key is required"})
		return
	}

	resp, err := h.client.Get("https://api.shodan.io/api-info?key=" + apiKey)
	if err != nil {
		c.JSON(http.StatusOK, validateKeyResponse{Engine: "shodan", Error: "connection failed: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		c.JSON(http.StatusOK, validateKeyResponse{Engine: "shodan", Error: "invalid API key"})
		return
	}
	if resp.StatusCode != 200 {
		c.JSON(http.StatusOK, validateKeyResponse{Engine: "shodan", Error: fmt.Sprintf("unexpected status: %d", resp.StatusCode)})
		return
	}

	var result struct {
		Plan         string `json:"plan"`
		QueryCredits int    `json:"query_credits"`
		ScanCredits  int    `json:"scan_credits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.JSON(http.StatusOK, validateKeyResponse{Engine: "shodan", Error: "invalid response"})
		return
	}
	c.JSON(http.StatusOK, validateKeyResponse{
		Engine: "shodan",
		Valid:  true,
		Info:   fmt.Sprintf("plan=%s, query_credits=%d, scan_credits=%d", result.Plan, result.QueryCredits, result.ScanCredits),
	})
}

// ValidateCensysKey validates Censys credentials via /api/v1/account
func (h *ReconHandler) ValidateCensysKey(c *gin.Context) {
	var req struct {
		APIID     string `json:"api_id"`
		APISecret string `json:"api_secret"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, validateKeyResponse{Engine: "censys", Error: "invalid request"})
		return
	}
	apiID := strings.TrimSpace(req.APIID)
	apiSecret := strings.TrimSpace(req.APISecret)
	if apiID == "" || apiSecret == "" {
		c.JSON(http.StatusOK, validateKeyResponse{Engine: "censys", Error: "api_id and api_secret are required"})
		return
	}

	httpReq, _ := http.NewRequestWithContext(c.Request.Context(), http.MethodGet,
		"https://search.censys.io/api/v1/account", nil)
	httpReq.SetBasicAuth(apiID, apiSecret)
	resp, err := h.client.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusOK, validateKeyResponse{Engine: "censys", Error: "connection failed: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		c.JSON(http.StatusOK, validateKeyResponse{Engine: "censys", Error: "invalid credentials"})
		return
	}
	if resp.StatusCode != 200 {
		c.JSON(http.StatusOK, validateKeyResponse{Engine: "censys", Error: fmt.Sprintf("unexpected status: %d", resp.StatusCode)})
		return
	}
	c.JSON(http.StatusOK, validateKeyResponse{Engine: "censys", Valid: true, Info: "credentials verified"})
}

// ── Search endpoints ────────────────────────────────────────────────────

type reconSearchResponse struct {
	Engine       string                   `json:"engine"`
	Query        string                   `json:"query"`
	Total        int                      `json:"total"`
	Page         int                      `json:"page"`
	ResultsCount int                      `json:"results_count"`
	Fields       []string                 `json:"fields"`
	Results      []map[string]interface{} `json:"results"`
}

// ZoomEyeSearch proxies a ZoomEye search request
func (h *ReconHandler) ZoomEyeSearch(c *gin.Context) {
	var req struct {
		Query    string `json:"query" binding:"required"`
		Page     int    `json:"page,omitempty"`
		PageSize int    `json:"pagesize,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	apiKey := strings.TrimSpace(h.cfg.ZoomEye.APIKey)
	if apiKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ZoomEye API key not configured"})
		return
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 10000 {
		req.PageSize = 10000
	}

	qb64 := base64.StdEncoding.EncodeToString([]byte(req.Query))
	payload := fmt.Sprintf(`{"qbase64":"%s","page":%d,"pagesize":%d}`, qb64, req.Page, req.PageSize)
	httpReq, _ := http.NewRequestWithContext(c.Request.Context(), http.MethodPost,
		"https://api.zoomeye.ai/v2/search", strings.NewReader(payload))
	httpReq.Header.Set("API-KEY", apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "ZoomEye request failed: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	var apiResp struct {
		Code    int                      `json:"code"`
		Message string                   `json:"message"`
		Query   string                   `json:"query"`
		Total   int                      `json:"total"`
		Data    []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(bodyBytes, &apiResp); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "invalid ZoomEye response"})
		return
	}
	if apiResp.Code != 60000 {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("ZoomEye error: %s (code %d)", apiResp.Message, apiResp.Code)})
		return
	}

	// Collect field names from first result
	fields := []string{"ip", "port", "domain", "update_time"}
	if len(apiResp.Data) > 0 {
		fieldSet := make(map[string]bool)
		for _, f := range fields {
			fieldSet[f] = true
		}
		for k := range apiResp.Data[0] {
			if !fieldSet[k] {
				fields = append(fields, k)
			}
		}
	}

	c.JSON(http.StatusOK, reconSearchResponse{
		Engine:       "zoomeye",
		Query:        apiResp.Query,
		Total:        apiResp.Total,
		Page:         req.Page,
		ResultsCount: len(apiResp.Data),
		Fields:       fields,
		Results:      apiResp.Data,
	})
}

// ShodanSearch proxies a Shodan search request
func (h *ReconHandler) ShodanSearch(c *gin.Context) {
	var req struct {
		Query string `json:"query" binding:"required"`
		Page  int    `json:"page,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	apiKey := strings.TrimSpace(h.cfg.Shodan.APIKey)
	if apiKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Shodan API key not configured"})
		return
	}

	if req.Page <= 0 {
		req.Page = 1
	}

	url := fmt.Sprintf("https://api.shodan.io/shodan/host/search?key=%s&query=%s&page=%d",
		apiKey, req.Query, req.Page)
	resp, err := h.client.Get(url)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Shodan request failed: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Shodan: invalid API key"})
		return
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	var apiResp struct {
		Total   int                      `json:"total"`
		Matches []map[string]interface{} `json:"matches"`
		Error   string                   `json:"error"`
	}
	if err := json.Unmarshal(bodyBytes, &apiResp); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "invalid Shodan response"})
		return
	}
	if apiResp.Error != "" {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Shodan: " + apiResp.Error})
		return
	}

	// Normalize results: flatten key fields for table display
	results := make([]map[string]interface{}, 0, len(apiResp.Matches))
	for _, m := range apiResp.Matches {
		row := make(map[string]interface{})
		row["ip_str"] = m["ip_str"]
		row["port"] = m["port"]
		row["org"] = m["org"]
		row["os"] = m["os"]
		row["product"] = m["product"]
		row["version"] = m["version"]
		row["transport"] = m["transport"]
		row["hostnames"] = m["hostnames"]
		row["domains"] = m["domains"]
		row["timestamp"] = m["timestamp"]
		if loc, ok := m["location"].(map[string]interface{}); ok {
			row["country"] = loc["country_name"]
			row["city"] = loc["city"]
		}
		// Include data preview
		if data, ok := m["data"].(string); ok {
			if len(data) > 200 {
				data = data[:200] + "..."
			}
			row["banner"] = data
		}
		results = append(results, row)
	}

	fields := []string{"ip_str", "port", "org", "product", "version", "os", "transport", "country", "city", "hostnames", "domains", "banner", "timestamp"}

	c.JSON(http.StatusOK, reconSearchResponse{
		Engine:       "shodan",
		Query:        req.Query,
		Total:        apiResp.Total,
		Page:         req.Page,
		ResultsCount: len(results),
		Fields:       fields,
		Results:      results,
	})
}

// CensysSearch proxies a Censys search request
func (h *ReconHandler) CensysSearch(c *gin.Context) {
	var req struct {
		Query   string `json:"query" binding:"required"`
		Page    int    `json:"page,omitempty"`
		PerPage int    `json:"per_page,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	apiID := strings.TrimSpace(h.cfg.Censys.APIID)
	apiSecret := strings.TrimSpace(h.cfg.Censys.APISecret)
	if apiID == "" || apiSecret == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Censys credentials not configured"})
		return
	}

	if req.PerPage <= 0 {
		req.PerPage = 25
	}
	if req.PerPage > 100 {
		req.PerPage = 100
	}

	// Censys v2 hosts search
	url := fmt.Sprintf("https://search.censys.io/api/v2/hosts/search?q=%s&per_page=%d",
		req.Query, req.PerPage)
	if req.Page > 1 {
		// Censys uses cursor-based pagination, but page number works for simple cases
		url += fmt.Sprintf("&page=%d", req.Page)
	}

	httpReq, _ := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, url, nil)
	httpReq.SetBasicAuth(apiID, apiSecret)
	httpReq.Header.Set("Accept", "application/json")

	resp, err := h.client.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Censys request failed: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Censys: invalid credentials"})
		return
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	var apiResp struct {
		Code   int    `json:"code"`
		Status string `json:"status"`
		Result struct {
			Query string                   `json:"query"`
			Total int                      `json:"total"`
			Hits  []map[string]interface{} `json:"hits"`
		} `json:"result"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(bodyBytes, &apiResp); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "invalid Censys response"})
		return
	}
	if apiResp.Error != "" {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Censys: " + apiResp.Error})
		return
	}

	// Normalize Censys hits
	results := make([]map[string]interface{}, 0, len(apiResp.Result.Hits))
	for _, hit := range apiResp.Result.Hits {
		row := make(map[string]interface{})
		row["ip"] = hit["ip"]
		if services, ok := hit["services"].([]interface{}); ok {
			ports := make([]interface{}, 0)
			serviceNames := make([]interface{}, 0)
			for _, s := range services {
				if svc, ok := s.(map[string]interface{}); ok {
					ports = append(ports, svc["port"])
					serviceNames = append(serviceNames, svc["service_name"])
				}
			}
			row["ports"] = ports
			row["services"] = serviceNames
		}
		if loc, ok := hit["location"].(map[string]interface{}); ok {
			row["country"] = loc["country"]
			row["city"] = loc["city"]
			row["continent"] = loc["continent"]
		}
		if as, ok := hit["autonomous_system"].(map[string]interface{}); ok {
			row["asn"] = as["asn"]
			row["as_name"] = as["name"]
		}
		row["last_updated"] = hit["last_updated_at"]
		results = append(results, row)
	}

	fields := []string{"ip", "ports", "services", "country", "city", "asn", "as_name", "last_updated"}

	c.JSON(http.StatusOK, reconSearchResponse{
		Engine:       "censys",
		Query:        apiResp.Result.Query,
		Total:        apiResp.Result.Total,
		Page:         req.Page,
		ResultsCount: len(results),
		Fields:       fields,
		Results:      results,
	})
}
