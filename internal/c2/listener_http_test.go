package c2

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"cyberstrike-ai/internal/database"

	"go.uber.org/zap"
)

// 集成验证：路由、鉴权伪装 404、明文 check-in JSON 回包。
func TestHTTPBeaconListener_CheckInMatrix(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "c2.sqlite")
	db, err := database.NewDB(dbPath, zap.NewNop())
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

	keyB64, err := GenerateAESKey()
	if err != nil {
		t.Fatal(err)
	}
	token := "test-implant-token-fixed"

	lid := "l_testhttpbeacon01"
	rec := &database.C2Listener{
		ID:            lid,
		Name:          "t",
		Type:          string(ListenerTypeHTTPBeacon),
		BindHost:      "127.0.0.1",
		BindPort:      port,
		EncryptionKey: keyB64,
		ImplantToken:  token,
		Status:        "stopped",
		ConfigJSON:    `{"beacon_check_in_path":"/check_in"}`,
		CreatedAt:     time.Now(),
	}
	if err := db.CreateC2Listener(rec); err != nil {
		t.Fatal(err)
	}

	m := NewManager(db, zap.NewNop(), filepath.Join(tmp, "c2store"))
	m.Registry().Register(string(ListenerTypeHTTPBeacon), NewHTTPBeaconListener)
	if _, err := m.StartListener(lid); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.StopListener(lid) })

	base := "http://127.0.0.1:" + strconv.Itoa(port)
	client := &http.Client{Timeout: 5 * time.Second}

	t.Run("wrong_path_go_default_404", func(t *testing.T) {
		resp, err := client.Post(base+"/nope", "application/json", strings.NewReader(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status=%d body=%q", resp.StatusCode, b)
		}
		if !strings.Contains(string(b), "404") || !strings.Contains(strings.ToLower(string(b)), "not found") {
			t.Fatalf("unexpected body: %q", b)
		}
	})

	t.Run("check_in_wrong_token_disguised_html_404", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, base+"/check_in", bytes.NewBufferString(`{"hostname":"h"}`))
		req.Header.Set("X-Implant-Token", "wrong-token")
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status=%d", resp.StatusCode)
		}
		ct := resp.Header.Get("Content-Type")
		if !strings.Contains(ct, "text/html") {
			t.Fatalf("content-type=%q body=%q", ct, b)
		}
		if !strings.Contains(string(b), "404 Not Found") {
			t.Fatalf("expected disguised HTML, got: %q", b)
		}
	})

	t.Run("check_in_ok_plaintext_json", func(t *testing.T) {
		body := `{"hostname":"n","username":"u","os":"Linux","arch":"amd64","internal_ip":"10.0.0.1","pid":42}`
		req, _ := http.NewRequest(http.MethodPost, base+"/check_in", strings.NewReader(body))
		req.Header.Set("X-Implant-Token", token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status=%d body=%s", resp.StatusCode, b)
		}
		var out ImplantCheckInResponse
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatalf("json: %v body=%s", err, b)
		}
		if out.SessionID == "" || out.NextSleep <= 0 {
			t.Fatalf("bad response: %+v", out)
		}
	})
}

func TestHTTPBeaconListener_HandleFileServe(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "c2.sqlite")
	db, err := database.NewDB(dbPath, zap.NewNop())
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

	keyB64, err := GenerateAESKey()
	if err != nil {
		t.Fatal(err)
	}
	token := "test-implant-token-file"

	lid := "l_testhttpfile01"
	rec := &database.C2Listener{
		ID:            lid,
		Name:          "t",
		Type:          string(ListenerTypeHTTPBeacon),
		BindHost:      "127.0.0.1",
		BindPort:      port,
		EncryptionKey: keyB64,
		ImplantToken:  token,
		Status:        "stopped",
		ConfigJSON:    `{"beacon_file_path":"/file/"}`,
		CreatedAt:     time.Now(),
	}
	if err := db.CreateC2Listener(rec); err != nil {
		t.Fatal(err)
	}

	store := filepath.Join(tmp, "c2store")
	m := NewManager(db, zap.NewNop(), store)
	m.Registry().Register(string(ListenerTypeHTTPBeacon), NewHTTPBeaconListener)
	if _, err := m.StartListener(lid); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.StopListener(lid) })

	fileID := "f_testfile123"
	downDir := filepath.Join(store, "downstream")
	if err := os.MkdirAll(downDir, 0o755); err != nil {
		t.Fatal(err)
	}
	want := []byte("upload-payload-bytes")
	if err := os.WriteFile(filepath.Join(downDir, fileID+".bin"), want, 0o644); err != nil {
		t.Fatal(err)
	}

	base := "http://127.0.0.1:" + strconv.Itoa(port)
	client := &http.Client{Timeout: 5 * time.Second}

	for _, path := range []string{"/file/" + fileID, "/file/" + fileID + ".bin"} {
		t.Run(path, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, base+path, nil)
			req.Header.Set("X-Implant-Token", token)
			resp, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				b, _ := io.ReadAll(resp.Body)
				t.Fatalf("status=%d body=%q", resp.StatusCode, b)
			}
			raw, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			plain, err := DecryptAESGCM(keyB64, string(raw))
			if err != nil {
				t.Fatal(err)
			}
			var out struct {
				FileData string `json:"file_data"`
			}
			if err := json.Unmarshal(plain, &out); err != nil {
				t.Fatal(err)
			}
			got, err := base64.StdEncoding.DecodeString(out.FileData)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("got %q want %q", got, want)
			}
		})
	}
}

func TestHTTPBeaconListener_HandleUploadConfinesTaskID(t *testing.T) {
	tmp := t.TempDir()
	store := filepath.Join(tmp, "c2store")
	keyB64, err := GenerateAESKey()
	if err != nil {
		t.Fatal(err)
	}
	token := "test-implant-token-upload"
	l := &HTTPBeaconListener{
		rec: &database.C2Listener{
			EncryptionKey: keyB64,
			ImplantToken:  token,
		},
		manager: NewManager(nil, zap.NewNop(), store),
		logger:  zap.NewNop(),
	}

	encrypted, err := EncryptAESGCM(keyB64, []byte("safe upload"))
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/upload?task_id=t_safe123", strings.NewReader(encrypted))
	req.Header.Set("X-Implant-Token", token)
	rr := httptest.NewRecorder()

	l.handleUpload(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", rr.Code, rr.Body.String())
	}
	got, err := os.ReadFile(filepath.Join(store, "uploads", "t_safe123.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "safe upload" {
		t.Fatalf("content=%q", got)
	}

	evilBody, err := EncryptAESGCM(keyB64, []byte("owned"))
	if err != nil {
		t.Fatal(err)
	}
	evilReq := httptest.NewRequest(http.MethodPost, "/upload?task_id=..%2Fowned", strings.NewReader(evilBody))
	evilReq.Header.Set("X-Implant-Token", token)
	evilRR := httptest.NewRecorder()

	l.handleUpload(evilRR, evilReq)

	if evilRR.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%q", evilRR.Code, evilRR.Body.String())
	}
	if _, err := os.Stat(filepath.Join(store, "owned.bin")); !os.IsNotExist(err) {
		t.Fatalf("outside file exists or stat failed unexpectedly: %v", err)
	}
}

func TestHTTPBeaconListener_HandleResultRejectsPlaintextJSON(t *testing.T) {
	keyB64, err := GenerateAESKey()
	if err != nil {
		t.Fatal(err)
	}
	l := &HTTPBeaconListener{
		rec: &database.C2Listener{
			EncryptionKey: keyB64,
			ImplantToken:  "test-implant-token-result",
		},
		logger: zap.NewNop(),
	}

	req := httptest.NewRequest(http.MethodPost, "/result", strings.NewReader(`{"task_id":"t_test","success":true}`))
	req.Header.Set("X-Implant-Token", "test-implant-token-result")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	l.handleResult(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%q", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "404 Not Found") {
		t.Fatalf("expected disguised 404 body, got %q", rr.Body.String())
	}
}
