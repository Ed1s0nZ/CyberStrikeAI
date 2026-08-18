package c2

import (
	"strings"
	"testing"
)

func TestDetectDownloadShellError(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
	}{
		{name: "empty ok", output: "", want: ""},
		{name: "base64 ok", output: "aGVsbG8=", want: ""},
		{name: "marker", output: "C2_DOWNLOAD_ERR: no such file or directory", want: "C2_DOWNLOAD_ERR: no such file or directory"},
		{name: "bash missing file", output: "bash: ../0: No such file or directory", want: "bash: ../0: No such file or directory"},
		{name: "permission denied", output: "C2_DOWNLOAD_ERR: permission denied", want: "C2_DOWNLOAD_ERR: permission denied"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectDownloadShellError(tt.output)
			if got != tt.want {
				t.Fatalf("detectDownloadShellError(%q) = %q, want %q", tt.output, got, tt.want)
			}
		})
	}
}

func TestBuildTCPCommandDownload(t *testing.T) {
	cmd, ok := buildTCPCommand(TaskTypeDownload, map[string]interface{}{
		"remote_path": "/tmp/demo.txt",
	})
	if !ok {
		t.Fatal("expected download command to be supported")
	}
	if want := "f='/tmp/demo.txt'"; !strings.Contains(cmd, want) {
		t.Fatalf("command %q should contain %q", cmd, want)
	}
	if !strings.Contains(cmd, "C2_DOWNLOAD_ERR") {
		t.Fatalf("command should validate file before base64: %q", cmd)
	}
}
