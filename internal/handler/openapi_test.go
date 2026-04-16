package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func TestOpenAPI_IncludesWebUserRbacEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewOpenAPIHandler(nil, zap.NewNop(), nil, nil, nil)
	router.GET("/openapi.json", handler.GetOpenAPISpec)

	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	req.Host = "localhost:8080"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	components, ok := body["components"].(map[string]any)
	if !ok {
		t.Fatalf("expected OpenAPI components object")
	}
	schemas, ok := components["schemas"].(map[string]any)
	if !ok {
		t.Fatalf("expected OpenAPI schemas object")
	}
	loginRequest, ok := schemas["LoginRequest"].(map[string]any)
	if !ok {
		t.Fatalf("expected LoginRequest schema")
	}
	required, ok := loginRequest["required"].([]any)
	if !ok {
		t.Fatalf("expected LoginRequest.required to be an array")
	}

	hasUsername := false
	for _, item := range required {
		if value, ok := item.(string); ok && value == "username" {
			hasUsername = true
			break
		}
	}
	if !hasUsername {
		t.Fatal("expected LoginRequest to require username")
	}

	paths, ok := body["paths"].(map[string]any)
	if !ok {
		t.Fatalf("expected OpenAPI paths object")
	}
	if _, ok := paths["/api/security/web-users"]; !ok {
		t.Fatal("expected /api/security/web-users path in OpenAPI output")
	}
	if _, ok := paths["/api/security/web-access-roles"]; !ok {
		t.Fatal("expected /api/security/web-access-roles path in OpenAPI output")
	}
	if _, ok := paths["/api/security/web-access-roles/permission-catalog"]; !ok {
		t.Fatal("expected /api/security/web-access-roles/permission-catalog path in OpenAPI output")
	}

	validatePath, ok := paths["/api/auth/validate"].(map[string]any)
	if !ok {
		t.Fatalf("expected /api/auth/validate path in OpenAPI output")
	}
	getOp, ok := validatePath["get"].(map[string]any)
	if !ok {
		t.Fatalf("expected GET operation for /api/auth/validate")
	}
	responses, ok := getOp["responses"].(map[string]any)
	if !ok {
		t.Fatalf("expected responses for /api/auth/validate")
	}
	response200, ok := responses["200"].(map[string]any)
	if !ok {
		t.Fatalf("expected 200 response for /api/auth/validate")
	}
	content, ok := response200["content"].(map[string]any)
	if !ok {
		t.Fatalf("expected content for /api/auth/validate 200 response")
	}
	jsonContent, ok := content["application/json"].(map[string]any)
	if !ok {
		t.Fatalf("expected application/json schema for /api/auth/validate")
	}
	schema, ok := jsonContent["schema"].(map[string]any)
	if !ok {
		t.Fatalf("expected schema for /api/auth/validate")
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected properties in /api/auth/validate schema")
	}
	if _, ok := properties["permissions"]; !ok {
		t.Fatal("expected permissions field in /api/auth/validate schema")
	}
}
