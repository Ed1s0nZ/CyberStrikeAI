# fix-windows.ps1 — 每次 git pull 后自动应用 Windows 兼容修复，无需手动重做
# 在 update.ps1 的 git pull 后、go build 前自动调用

$ErrorActionPreference = "Stop"
$root = Split-Path $PSCommandPath -Parent
$utf8 = [System.Text.UTF8Encoding]::new($false)

Write-Host "[fix-windows] Checking Windows compatibility fixes..." -ForegroundColor Cyan

# ── 1. 修复 tools/*.yaml: python3 → python ──
Get-ChildItem "$root\tools\*.yaml" | ForEach-Object {
    $content = Get-Content $_.FullName -Raw -Encoding UTF8
    if ($content -match 'command:\s*"python3"') {
        $content = $content -replace 'command:\s*"python3"', 'command: "python"'
        [System.IO.File]::WriteAllText($_.FullName, $content, $utf8)
        Write-Host "  python3→python: $($_.Name)" -ForegroundColor Green
    }
}

# ── 2. 修复 execute-python-script.yaml & install-python-package.yaml ──
@("execute-python-script.yaml", "install-python-package.yaml") | ForEach-Object {
    $path = "$root\tools\$_"
    if (Test-Path $path) {
        $content = Get-Content $path -Raw -Encoding UTF8
        $changed = $false
        if ($content -match 'command:\s*"/bin/bash"') {
            $content = $content -replace 'command:\s*"/bin/bash"', 'command: "powershell"'
            $changed = $true
        }
        if ($content -match 'python3') {
            $content = $content -replace 'python3', 'python'
            $changed = $true
        }
        if ($changed) {
            [System.IO.File]::WriteAllText($path, $content, $utf8)
            Write-Host "  bash→powershell+python3→python: $_" -ForegroundColor Green
        }
    }
}

# ── 3. 修复 shell_execute_stream.go ──
$ssePath = "$root\internal\security\shell_execute_stream.go"
if (Test-Path $ssePath) {
    $content = Get-Content $ssePath -Raw -Encoding UTF8
    $changed = $false

    # 3a. 如果缺少 getSystemShell 函数，加回 import 和函数
    if ($content -notmatch 'func getSystemShell') {
        $content = $content -replace '("os/exec"\r?\n)(\s+)"sync"',
            ('$1' + "`t`"runtime`"`r`n" + '$2"sync"')
        $funcDef = @'

// getSystemShell returns the shell binary and argument to execute a command string.
func getSystemShell() (string, string) {
	if runtime.GOOS == "windows" {
		return "powershell.exe", "-Command"
	}
	return "/bin/sh", "-c"
}
'@
        $content = $content -replace '(// ConfigureShellCmdForAgentExecute)', ($funcDef + "`r`n" + '$1')
        $changed = $true
    }

    # 3b. 修复硬编码的 /bin/sh → getSystemShell()
    if ($content -match 'exec\.CommandContext\(ctx, "/bin/sh", "-c", command\)') {
        $replacement = @'
	shell, shellArg := getSystemShell()
	cmd := exec.CommandContext(ctx, shell, shellArg, command)
'@
        $content = $content -replace (
            'cmd := exec\.CommandContext\(ctx, "/bin/sh", "-c", command\)',
            $replacement
        )
        $changed = $true
    }

    if ($changed) {
        [System.IO.File]::WriteAllText($ssePath, $content, $utf8)
        Write-Host "  getSystemShell + /bin/sh fix: shell_execute_stream.go" -ForegroundColor Green
    }
}

# ── 4. 修复 executor.go ──
$execPath = "$root\internal\security\executor.go"
if (Test-Path $execPath) {
    $content = Get-Content $execPath -Raw -Encoding UTF8
    $changed = $false

    # 4a. 修复 shell 默认值
    if ($content -match 'shell := "sh"') {
        $old = @'
	shell := "sh"
	if s, ok := args["shell"].(string); ok && s != "" {
		shell = s
	}
'@
        $new = @'
	shell, shellArg := getSystemShell()
	if s, ok := args["shell"].(string); ok && s != "" {
		shell, shellArg = s, "-c"
	}
'@
        $content = $content -replace ([regex]::Escape($old), $new)
        $changed = $true
    }

    # 4b. 修复 exec.CommandContext 硬编码 "-c"
    if ($content -match 'exec\.CommandContext\(ctx, shell, "-c", command\)') {
        $content = $content -replace (
            'exec\.CommandContext\(ctx, shell, "-c", command\)',
            'exec.CommandContext(ctx, shell, shellArg, command)'
        )
        $changed = $true
    }

    # 4c. 恢复 rewritePythonInlineScriptToTemp 调用
    if ($content -notmatch 'rewritePythonInlineScriptToTemp') {
        $old = "`t// 执行命令`r`n`tcmd := exec.CommandContext(ctx, toolConfig.Command, cmdArgs...)"
        $newStr = "`t// 执行命令`r`n`t// Windows 命令行长度限制 ~8191 字符：python -c 携带超长内联脚本时自动写入临时 .py 文件`r`n`tcmdArgs = rewritePythonInlineScriptToTemp(toolConfig.Command, cmdArgs)`r`n`tcmd := exec.CommandContext(ctx, toolConfig.Command, cmdArgs...)"
        $content = $content -replace ([regex]::Escape($old), $newStr)
        $changed = $true
    }

    # 4d. 恢复 rewritePythonInlineScriptToTemp 函数定义
    if ($content -notmatch 'func rewritePythonInlineScriptToTemp') {
        $func = @'

// rewritePythonInlineScriptToTemp 检测 python -c 超长脚本，超过 Windows 命令行限制时写入临时 .py 文件
func rewritePythonInlineScriptToTemp(command string, cmdArgs []string) []string {
	if runtime.GOOS != "windows" {
		return cmdArgs
	}
	if !strings.HasSuffix(command, "python") && !strings.HasSuffix(command, "python.exe") {
		return cmdArgs
	}
	for i := 0; i < len(cmdArgs)-1; i++ {
		if cmdArgs[i] == "-c" && i+1 < len(cmdArgs) {
			script := cmdArgs[i+1]
			if len(script) < 3000 {
				return cmdArgs
			}
			tmpFile, err := os.CreateTemp("", "cyberstrike-tool-*.py")
			if err != nil {
				return cmdArgs
			}
			if _, err := tmpFile.WriteString(script); err != nil {
				tmpFile.Close()
				os.Remove(tmpFile.Name())
				return cmdArgs
			}
			tmpFile.Close()
			newArgs := make([]string, 0, len(cmdArgs)-1)
			newArgs = append(newArgs, cmdArgs[:i]...)
			newArgs = append(newArgs, tmpFile.Name())
			newArgs = append(newArgs, cmdArgs[i+2:]...)
			return newArgs
		}
	}
	return cmdArgs
}
'@
        $anchor = '// getExitCode'
        $content = $content -replace ($anchor, ($func + "`r`n" + $anchor))
        $changed = $true
    }

    if ($changed) {
        [System.IO.File]::WriteAllText($execPath, $content, $utf8)
        Write-Host "  shell+rewritePython fix: executor.go" -ForegroundColor Green
    }
}

Write-Host "[fix-windows] Done." -ForegroundColor Cyan
