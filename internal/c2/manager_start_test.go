package c2

import (
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"cyberstrike-ai/internal/database"

	"go.uber.org/zap"
)

// 回归：StartListener 返回的 rec 被 handler 脱敏清空 ImplantToken 后，运行中的 HTTP listener 仍能鉴权。
func TestStartListener_ImplantTokenSurvivesHandlerRedaction(t *testing.T) {
	tmp := t.TempDir()
	db, err := database.NewDB(filepath.Join(tmp, "c2.sqlite"), zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	lnPick, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := lnPick.Addr().(*net.TCPAddr).Port
	_ = lnPick.Close()

	mgr := NewManager(db, zap.NewNop(), tmp)
	mgr.Registry().Register(string(ListenerTypeHTTPBeacon), NewHTTPBeaconListener)
	rec, err := mgr.CreateListener(CreateListenerInput{
		Name:     "t",
		Type:     string(ListenerTypeHTTPBeacon),
		BindHost: "127.0.0.1",
		BindPort: port,
	})
	if err != nil {
		t.Fatal(err)
	}
	token := rec.ImplantToken

	rec, err = mgr.StartListener(rec.ID)
	if err != nil {
		t.Fatal(err)
	}
	// 模拟 internal/handler/c2.go StartListener 在 JSON 响应前的脱敏
	rec.ImplantToken = ""
	rec.EncryptionKey = ""

	time.Sleep(50 * time.Millisecond)

	body := `{"hostname":"n","username":"u","os":"Linux","arch":"amd64","internal_ip":"10.0.0.1","pid":42}`
	req, _ := http.NewRequest(http.MethodPost, "http://127.0.0.1:"+strconv.Itoa(port)+"/check_in", strings.NewReader(body))
	req.Header.Set("X-Implant-Token", token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.StatusCode, b)
	}
	if !strings.Contains(string(b), "session_id") {
		t.Fatalf("expected session_id in body: %s", b)
	}
	_ = mgr.StopListener(rec.ID)
}
