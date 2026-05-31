package handler

import (
	"strings"
	"testing"

	"go.uber.org/zap"
)

func TestBuildFileCommandWindowsNormalizesSlashPaths(t *testing.T) {
	h := NewWebShellHandler(zap.NewNop(), nil)

	tests := []struct {
		name   string
		input  fileCommandInput
		expect string
	}{
		{
			name: "list drive path",
			input: fileCommandInput{
				Action: "list",
				Path:   "C:/inetpub/wwwroot",
				OS:     "windows",
			},
			expect: `dir /a "C:\inetpub\wwwroot"`,
		},
		{
			name: "read nested file",
			input: fileCommandInput{
				Action: "read",
				Path:   "D:/site/uploads/a.txt",
				OS:     "windows",
			},
			expect: `powershell -NoProfile -NonInteractive -Command "[Console]::Write('__CSAI_FILE_B64__:');[Console]::Write([Convert]::ToBase64String([IO.File]::ReadAllBytes('D:\site\uploads\a.txt')))"`,
		},
		{
			name: "rename both paths",
			input: fileCommandInput{
				Action:     "rename",
				Path:       "C:/tmp/old.txt",
				TargetPath: "C:/tmp/new.txt",
				OS:         "windows",
			},
			expect: `move /y "C:\tmp\old.txt" "C:\tmp\new.txt"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := h.buildFileCommand(tt.input)
			if err != nil {
				t.Fatalf("buildFileCommand returned error: %v", err)
			}
			if got != tt.expect {
				t.Fatalf("command mismatch:\n got: %s\nwant: %s", got, tt.expect)
			}
		})
	}
}

func TestBuildFileCommandWindowsPowerShellWriteNormalizesSlashPath(t *testing.T) {
	h := NewWebShellHandler(zap.NewNop(), nil)
	got, err := h.buildFileCommand(fileCommandInput{
		Action:  "write",
		Path:    "C:/inetpub/wwwroot/a.txt",
		Content: "hello",
		OS:      "windows",
	})
	if err != nil {
		t.Fatalf("buildFileCommand returned error: %v", err)
	}
	if !strings.Contains(got, `[IO.File]::WriteAllBytes('C:\inetpub\wwwroot\a.txt',$b)`) {
		t.Fatalf("PowerShell write command did not normalize path: %s", got)
	}
}
