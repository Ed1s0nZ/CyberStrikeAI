package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"cyberstrike-ai/internal/config"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestFofaSearchUsesAPIKeyWithoutEmail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("FOFA_API_KEY", "")
	t.Setenv("FOFA_EMAIL", "legacy@example.com")

	var receivedEmail string
	var receivedKey string
	fofaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedEmail = r.URL.Query().Get("email")
		receivedKey = r.URL.Query().Get("key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":false,"size":1,"page":1,"results":[["https://example.com"]]}`))
	}))
	defer fofaServer.Close()

	h := NewFofaHandler(&config.Config{
		FOFA: config.FofaConfig{
			BaseURL: fofaServer.URL,
			APIKey:  "test-api-key",
		},
	}, zap.NewNop())

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	body := `{"query":"domain=\"example.com\"","fields":"host"}`
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/fofa/search", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	h.Search(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("Search() status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if receivedEmail != "" {
		t.Fatalf("FOFA request unexpectedly included email = %q", receivedEmail)
	}
	if receivedKey != "test-api-key" {
		t.Fatalf("FOFA request key = %q, want %q", receivedKey, "test-api-key")
	}

	var response fofaSearchResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ResultsCount != 1 {
		t.Fatalf("results_count = %d, want 1", response.ResultsCount)
	}
}

func TestSafeFofaRequestErrorDoesNotExposeURLOrAPIKey(t *testing.T) {
	const secretURL = "https://fofa.info/api/v1/search/all?key=secret-api-key"
	err := &url.Error{
		Op:  http.MethodGet,
		URL: secretURL,
		Err: context.DeadlineExceeded,
	}

	status, message, timeout := safeFofaRequestError(err)

	if status != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want %d", status, http.StatusGatewayTimeout)
	}
	if !timeout {
		t.Fatal("timeout = false, want true")
	}
	if strings.Contains(message, "secret-api-key") || strings.Contains(message, secretURL) {
		t.Fatalf("safe error exposed request URL or API key: %q", message)
	}
}

func TestShodanSearchReportsShortfallWhenTotalExceedsMatches(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("SHODAN_API_KEY", "")

	shodanServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/shodan/host/search" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("key"); got != "test-shodan-key" {
			t.Fatalf("Shodan key = %q, want test-shodan-key", got)
		}
		page := r.URL.Query().Get("page")
		count := 0
		switch page {
		case "1":
			count = 100
		case "2":
			count = 3
		default:
			count = 0
		}
		matches := make([]map[string]interface{}, 0, count)
		for i := 0; i < count; i++ {
			matches = append(matches, map[string]interface{}{
				"ip_str": fmt.Sprintf("192.0.2.%d", i+1),
				"port":   80,
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"total":   104,
			"matches": matches,
		})
	}))
	defer shodanServer.Close()

	h := NewFofaHandler(&config.Config{
		Shodan: config.SpaceSearchConfig{
			BaseURL: shodanServer.URL,
			APIKey:  "test-shodan-key",
		},
	}, zap.NewNop())

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	body := `{"provider":"shodan","query":"product:nginx","fields":"ip_str,port","size":1000,"page":1}`
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/fofa/search", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	h.Search(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("Search() status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response fofaSearchResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Total != 104 || response.ResultsCount != 103 {
		t.Fatalf("counts: total=%d results_count=%d, want 104/103", response.Total, response.ResultsCount)
	}
	if response.ExpectedCount != 104 || response.Shortfall != 1 {
		t.Fatalf("shortfall: expected=%d shortfall=%d, want 104/1", response.ExpectedCount, response.Shortfall)
	}
	if response.Warning == "" {
		t.Fatal("warning should explain shortfall")
	}
}

func TestQuakeSearchHandlesStringErrorCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("QUAKE_API_KEY", "")

	quakeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-QuakeToken"); got != "test-quake-key" {
			t.Fatalf("Quake token = %q, want test-quake-key", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"q5000","message":"查询语法错误","data":{}}`))
	}))
	defer quakeServer.Close()

	h := NewFofaHandler(&config.Config{
		Quake: config.SpaceSearchConfig{
			BaseURL: quakeServer.URL,
			APIKey:  "test-quake-key",
		},
	}, zap.NewNop())

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	body := `{"provider":"quake","query":"bad query","fields":"ip,port","size":10,"page":1}`
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/fofa/search", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	h.Search(ctx)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("Search() status = %d, want %d, body = %s", recorder.Code, http.StatusBadGateway, recorder.Body.String())
	}
	bodyText := recorder.Body.String()
	if !strings.Contains(bodyText, "查询语法错误") {
		t.Fatalf("response should include Quake error message, got %s", bodyText)
	}
	if strings.Contains(bodyText, "cannot unmarshal") {
		t.Fatalf("response exposed JSON type decoding failure: %s", bodyText)
	}
}

func TestQuakeSearchAcceptsArrayData(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("QUAKE_API_KEY", "")

	quakeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"Successful.","data":[{"ip":"1.1.1.1","port":53}],"meta":{"pagination":{"total":1}}}`))
	}))
	defer quakeServer.Close()

	h := NewFofaHandler(&config.Config{
		Quake: config.SpaceSearchConfig{
			BaseURL: quakeServer.URL,
			APIKey:  "test-quake-key",
		},
	}, zap.NewNop())

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	body := `{"provider":"quake","query":"ip:\"1.1.1.1\"","fields":"ip,port","size":10,"page":1}`
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/fofa/search", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	h.Search(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("Search() status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response fofaSearchResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ResultsCount != 1 || response.Total != 1 {
		t.Fatalf("results_count=%d total=%d, want 1/1", response.ResultsCount, response.Total)
	}
}

func TestZoomEyeSearchHandlesObjectDataError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("ZOOMEYE_API_KEY", "")

	zoomeyeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("API-KEY"); got != "test-zoomeye-key" {
			t.Fatalf("ZoomEye API-KEY = %q, want test-zoomeye-key", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":40001,"message":"invalid api key","data":{}}`))
	}))
	defer zoomeyeServer.Close()

	h := NewFofaHandler(&config.Config{
		ZoomEye: config.SpaceSearchConfig{
			BaseURL: zoomeyeServer.URL,
			APIKey:  "test-zoomeye-key",
		},
	}, zap.NewNop())

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	body := `{"provider":"zoomeye","query":"ip=\"1.1.1.1\"","fields":"ip,port","size":10,"page":1}`
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/fofa/search", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	h.Search(ctx)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("Search() status = %d, want %d, body = %s", recorder.Code, http.StatusBadGateway, recorder.Body.String())
	}
	bodyText := recorder.Body.String()
	if !strings.Contains(bodyText, "invalid api key") {
		t.Fatalf("response should include ZoomEye error message, got %s", bodyText)
	}
	if strings.Contains(bodyText, "cannot unmarshal") {
		t.Fatalf("response exposed JSON type decoding failure: %s", bodyText)
	}
}

func TestShodanSearchSurfacesJSONErrorOnUnauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("SHODAN_API_KEY", "")

	shodanServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"Invalid API key"}`))
	}))
	defer shodanServer.Close()

	h := NewFofaHandler(&config.Config{
		Shodan: config.SpaceSearchConfig{
			BaseURL: shodanServer.URL,
			APIKey:  "test-shodan-key",
		},
	}, zap.NewNop())

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	body := `{"provider":"shodan","query":"product:nginx","fields":"ip_str,port","size":10,"page":1}`
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/fofa/search", strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	h.Search(ctx)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("Search() status = %d, want %d, body = %s", recorder.Code, http.StatusBadGateway, recorder.Body.String())
	}
	bodyText := recorder.Body.String()
	if !strings.Contains(bodyText, "Invalid API key") {
		t.Fatalf("response should include Shodan error message, got %s", bodyText)
	}
	if strings.Contains(bodyText, "非 2xx") {
		t.Fatalf("response should not hide Shodan error behind generic status, got %s", bodyText)
	}
}

func TestCanonicalizeSpaceSearchBaseURLMigratesLegacyHosts(t *testing.T) {
	t.Parallel()
	cases := []struct {
		provider string
		in       string
		want     string
	}{
		{provider: "zoomeye", in: "https://api.zoomeye.org/v2/search", want: "https://api.zoomeye.ai/v2/search"},
		{provider: "quake", in: "https://quake.360.cn/api/v3/search/quake_service", want: "https://quake.360.net/api/v3/search/quake_service"},
		{provider: "shodan", in: "https://api.shodan.io", want: "https://api.shodan.io"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.provider, func(t *testing.T) {
			t.Parallel()
			got := canonicalizeSpaceSearchBaseURL(tc.provider, tc.in)
			if got != tc.want {
				t.Fatalf("canonicalizeSpaceSearchBaseURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestExtractInfoCollectJSONObject(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "plain json",
			in:   `{"query":"title:\"CyberStrikeAI\"","warnings":[]}`,
			want: `{"query":"title:\"CyberStrikeAI\"","warnings":[]}`,
		},
		{
			name: "fenced json",
			in:   "```json\n{\"query\":\"product:nginx\"}\n```",
			want: `{"query":"product:nginx"}`,
		},
		{
			name: "prefixed explanation",
			in:   "解析结果如下：\n{\"query\":\"ssl.cert.subject.cn:example.com\",\"explanation\":\"ok\"}\n请确认。",
			want: `{"query":"ssl.cert.subject.cn:example.com","explanation":"ok"}`,
		},
		{
			name: "braces inside string",
			in:   "结果：{\"query\":\"title:\\\"{admin}\\\"\",\"warnings\":[\"check\"]}",
			want: `{"query":"title:\"{admin}\"","warnings":["check"]}`,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := extractInfoCollectJSONObject(tc.in)
			if err != nil {
				t.Fatalf("extractInfoCollectJSONObject() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("extractInfoCollectJSONObject() = %q, want %q", got, tc.want)
			}
		})
	}
}
