package handler

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type DockerHandler struct {
	rootDir    string
	scriptPath string
	logger     *zap.Logger
}

type DockerActionRequest struct {
	Action       string `json:"action"`
	ProxyMode    string `json:"proxy_mode"`
	ProxyURL     string `json:"proxy_url"`
	VPNContainer string `json:"vpn_container"`
	GitRef       string `json:"git_ref"`
}

func NewDockerHandler(rootDir string, logger *zap.Logger) *DockerHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &DockerHandler{
		rootDir:    rootDir,
		scriptPath: filepath.Join(rootDir, "run_docker.sh"),
		logger:     logger,
	}
}

func (h *DockerHandler) GetStatus(c *gin.Context) {
	inDocker := isInDocker()
	dockerInstalled := commandExists("docker")
	composeInstalled := false
	composeVersion := ""
	if dockerInstalled {
		if out, err := runShortCmd(h.rootDir, 5*time.Second, "docker", "compose", "version"); err == nil {
			composeInstalled = true
			composeVersion = strings.TrimSpace(out)
		} else if out, err = runShortCmd(h.rootDir, 5*time.Second, "docker-compose", "version"); err == nil {
			composeInstalled = true
			composeVersion = strings.TrimSpace(out)
		}
	}

	containerStatus := "not_found"
	containerImage := ""
	containerName := "cyberstrikeai"
	if inDocker {
		if hn := strings.TrimSpace(os.Getenv("HOSTNAME")); hn != "" {
			containerName = hn
		}
	}
	if dockerInstalled {
		if out, err := runShortCmd(h.rootDir, 8*time.Second, "docker", "ps", "-a", "--filter", "name=^/cyberstrikeai$", "--format", "{{.Status}}|{{.Image}}|{{.Names}}"); err == nil {
			line := strings.TrimSpace(out)
			if line != "" {
				parts := strings.SplitN(line, "|", 3)
				containerStatus = parts[0]
				if len(parts) > 1 {
					containerImage = parts[1]
				}
				if len(parts) > 2 && strings.TrimSpace(parts[2]) != "" {
					containerName = parts[2]
				}
			}
		}
	} else if inDocker {
		containerStatus = "running (self)"
		containerImage = "unknown (docker cli unavailable in container)"
	}

	scriptExists := false
	if st, err := os.Stat(h.scriptPath); err == nil && !st.IsDir() {
		scriptExists = true
	}

	httpStatus := map[string]interface{}{
		"app_18080": probeHTTP("http://127.0.0.1:18080/"),
		"app_8080":  probeHTTP("http://127.0.0.1:8080/"),
	}

	c.JSON(http.StatusOK, gin.H{
		"in_docker":         inDocker,
		"docker_installed":  dockerInstalled,
		"compose_installed": composeInstalled,
		"compose_version":   composeVersion,
		"container_name":    containerName,
		"container_status":  containerStatus,
		"container_image":   containerImage,
		"script_exists":     scriptExists,
		"script_path":       h.scriptPath,
		"http":              httpStatus,
		"checked_at":        time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *DockerHandler) GetLogs(c *gin.Context) {
	lines := 200
	if raw := strings.TrimSpace(c.Query("lines")); raw != "" {
		if n, err := parsePositiveInt(raw, 5000); err == nil {
			lines = n
		}
	}

	// Prefer container logs when Docker is available.
	if commandExists("docker") {
		out, err := runShortCmd(h.rootDir, 15*time.Second, "docker", "logs", "--tail", fmt.Sprintf("%d", lines), "cyberstrikeai")
		if err == nil {
			c.JSON(http.StatusOK, gin.H{
				"source": "docker",
				"lines":  lines,
				"log":    out,
			})
			return
		}
	}

	// Fallback to local suite log file.
	logPath := filepath.Join(h.rootDir, "logs", "suite.log")
	content, err := tailFile(logPath, lines)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"source": "none",
			"lines":  lines,
			"log":    "",
			"error":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"source": "file",
		"path":   logPath,
		"lines":  lines,
		"log":    content,
	})
}

func (h *DockerHandler) RunAction(c *gin.Context) {
	var req DockerActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	action := strings.TrimSpace(strings.ToLower(req.Action))
	if action == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "action is required"})
		return
	}

	allowed := map[string]struct{}{
		"install": {}, "deploy": {}, "update": {}, "remove": {}, "status": {},
		"logs": {}, "start": {}, "stop": {}, "restart": {}, "test": {},
	}
	if _, ok := allowed[action]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported action"})
		return
	}

	args := []string{action}
	if m := strings.TrimSpace(strings.ToLower(req.ProxyMode)); m != "" {
		args = append(args, "--proxy-mode", m)
	}
	if p := strings.TrimSpace(req.ProxyURL); p != "" {
		args = append(args, "--proxy-url", p)
	}
	if v := strings.TrimSpace(req.VPNContainer); v != "" {
		args = append(args, "--vpn-container", v)
	}
	if g := strings.TrimSpace(req.GitRef); g != "" {
		args = append(args, "--git-ref", g)
	}

	timeout := 45 * time.Minute
	if action == "status" || action == "logs" {
		timeout = 2 * time.Minute
	}

	output, err := runScript(h.rootDir, h.scriptPath, timeout, args...)
	exitCode := 0
	if err != nil {
		exitCode = 1
	}
	statusCode := http.StatusOK
	if err != nil {
		statusCode = http.StatusInternalServerError
	}
	c.JSON(statusCode, gin.H{
		"action":   action,
		"success":  err == nil,
		"exitCode": exitCode,
		"output":   output,
		"error":    errString(err),
	})
}

func runScript(workDir, scriptPath string, timeout time.Duration, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmdArgs := append([]string{scriptPath}, args...)
	cmd := exec.CommandContext(ctx, "bash", cmdArgs...)
	cmd.Dir = workDir

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return out.String(), fmt.Errorf("action timed out after %s", timeout)
	}
	return out.String(), err
}

func runShortCmd(workDir string, timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = workDir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func isInDocker() bool {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("CYBERSTRIKE_DOCKER")), "true") {
		return true
	}
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		txt := strings.ToLower(string(data))
		return strings.Contains(txt, "docker") || strings.Contains(txt, "containerd")
	}
	return false
}

func probeHTTP(url string) map[string]interface{} {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return map[string]interface{}{"ok": false, "error": err.Error()}
	}
	defer resp.Body.Close()
	return map[string]interface{}{"ok": resp.StatusCode >= 200 && resp.StatusCode < 500, "status_code": resp.StatusCode}
}

func parsePositiveInt(raw string, max int) (int, error) {
	n := 0
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("not a number")
		}
		n = n*10 + int(ch-'0')
		if n > max {
			return max, nil
		}
	}
	if n <= 0 {
		return 0, fmt.Errorf("must be > 0")
	}
	return n, nil
}

func tailFile(path string, lines int) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	chunks := strings.Split(string(data), "\n")
	if lines <= 0 || len(chunks) <= lines {
		return string(data), nil
	}
	return strings.Join(chunks[len(chunks)-lines:], "\n"), nil
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
