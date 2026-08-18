package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cyberstrike-ai/internal/config"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func newWecomTestHandler(token string, aesKey string) *RobotHandler {
	return &RobotHandler{
		config: &config.Config{
			Robots: config.RobotsConfig{
				Wecom: config.RobotWecomConfig{
					Enabled:        true,
					Token:          token,
					EncodingAESKey: aesKey,
				},
			},
		},
		logger: zap.NewNop(),
	}
}

func TestHandleWecomPOST_rejectsWhenTokenEmpty(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := newWecomTestHandler("", "")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `<?xml version="1.0"?><xml><FromUserName>attacker</FromUserName><MsgType>text</MsgType><Content>hi</Content></xml>`
	c.Request = httptest.NewRequest(http.MethodPost, "/api/robot/wecom", strings.NewReader(body))

	h.HandleWecomPOST(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
	if w.Body.String() == "success" {
		t.Fatal("expected rejection, got success")
	}
}

func TestHandleWecomPOST_rejectsPlaintextWhenEncryptionConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := newWecomTestHandler("secret-token", "abcdefghijklmnopqrstuvwxyz0123456789ABCD")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := `<?xml version="1.0"?><xml><FromUserName>attacker</FromUserName><MsgType>text</MsgType><Content>hi</Content></xml>`
	c.Request = httptest.NewRequest(http.MethodPost, "/api/robot/wecom?timestamp=1&nonce=2&msg_signature=fake", strings.NewReader(body))

	h.HandleWecomPOST(c)

	if w.Body.String() == "success" {
		t.Fatal("expected rejection for plaintext in encryption mode, got success")
	}
}

func TestHandleWecomGET_rejectsWhenTokenEmpty(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := newWecomTestHandler("", "")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/robot/wecom?msg_signature=x&timestamp=1&nonce=2&echostr=abc", nil)

	h.HandleWecomGET(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}
