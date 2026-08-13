<#
.SYNOPSIS
Start IDA Pro MCP HTTP server (background, non-blocking)

.DESCRIPTION
1. Kill old process
2. Start idalib-mcp HTTP server in hidden window mode
3. Wait for service ready (max 15 seconds)
4. Output result

Usage: run without parameters
#>

param(
    [string]$IdaDir,
    [int]$Port = 13337,
    [string]$ServerPath
)

function Get-InstalledIdaDir {
    $registryPaths = @(
        'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\*',
        'HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*',
        'HKCU:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\*'
    )

    $fromRegistry = $registryPaths |
        ForEach-Object {
            Get-ItemProperty $_ -ErrorAction SilentlyContinue
        } |
        Where-Object {
            ($_.DisplayName -match 'IDA|Hex-Rays') -and
            -not [string]::IsNullOrWhiteSpace($_.InstallLocation) -and
            (Test-Path -LiteralPath $_.InstallLocation)
        } |
        Select-Object -ExpandProperty InstallLocation -First 1

    if ($fromRegistry) {
        return $fromRegistry
    }

    $idaCandidates = @(
        'C:\Program Files\IDA Pro',
        'C:\Program Files\IDA',
        'C:\IDA Pro',
        'C:\IDA',
        'D:\IDA',
        'E:\Program Files\IDA',
        (Join-Path $env:USERPROFILE 'Tools\IDA')
    ) | Where-Object { -not [string]::IsNullOrWhiteSpace($_) }

    return $idaCandidates | Where-Object { Test-Path -LiteralPath $_ } | Select-Object -First 1
}

if ([string]::IsNullOrWhiteSpace($IdaDir)) {
    if (-not [string]::IsNullOrWhiteSpace($env:IDADIR)) {
        $IdaDir = $env:IDADIR
    } else {
        $persistedIdaDir = [Environment]::GetEnvironmentVariable('IDADIR', 'User')
        if ([string]::IsNullOrWhiteSpace($persistedIdaDir)) {
            $persistedIdaDir = [Environment]::GetEnvironmentVariable('IDADIR', 'Machine')
        }

        if (-not [string]::IsNullOrWhiteSpace($persistedIdaDir) -and (Test-Path -LiteralPath $persistedIdaDir)) {
            $IdaDir = $persistedIdaDir
        } else {
            # Search registry install records and common fallback paths
            $foundIda = Get-InstalledIdaDir
            if ($foundIda) {
                $IdaDir = $foundIda
            } else {
                Write-Output "ERR:IDADIR not set and IDA Pro not found. Set IDADIR environment variable."
                exit 1
            }
        }
    }
}
$env:IDADIR = $IdaDir

if ([string]::IsNullOrWhiteSpace($ServerPath)) {
    # Try both possible executable names (idalib-mcp is the HTTP server, ida-pro-mcp is the installer CLI)
    $resolved = Get-Command idalib-mcp -ErrorAction SilentlyContinue
    if (-not $resolved) {
        $resolved = Get-Command ida-pro-mcp -ErrorAction SilentlyContinue
    }
    if ($resolved) {
        $ServerPath = $resolved.Source
    }
    else {
        $roamingPython = Join-Path $env:APPDATA 'Python'
        if (Test-Path -LiteralPath $roamingPython) {
            $candidate = Get-ChildItem -LiteralPath $roamingPython -Directory -ErrorAction SilentlyContinue |
                ForEach-Object {
                    $scripts = Join-Path $_.FullName 'Scripts'
                    @('idalib-mcp.exe', 'ida-pro-mcp.exe') | ForEach-Object { Join-Path $scripts $_ }
                } |
                Where-Object { Test-Path -LiteralPath $_ } |
                Select-Object -First 1
            if ($candidate) {
                $ServerPath = $candidate
            }
        }
    }
}

# Auto-bootstrap if still not found
if ([string]::IsNullOrWhiteSpace($ServerPath)) {
    $bootstrapScript = Join-Path $PSScriptRoot '..\..\scripts\bootstrap-reverse.ps1'
    if (Test-Path -LiteralPath $bootstrapScript) {
        Write-Output "INFO: ida-pro-mcp not found, attempting auto-bootstrap (installing mrexodia/ida-pro-mcp)..."
        & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $bootstrapScript -Capability @('idalib-mcp') -SkipRefresh
        $resolved = Get-Command ida-pro-mcp -ErrorAction SilentlyContinue
        if (-not $resolved) {
            $resolved = Get-Command idalib-mcp -ErrorAction SilentlyContinue
        }
        if ($resolved) {
            $ServerPath = $resolved.Source
        }
        else {
            $roamingPython = Join-Path $env:APPDATA 'Python'
            if (Test-Path -LiteralPath $roamingPython) {
                $candidate = Get-ChildItem -LiteralPath $roamingPython -Directory -ErrorAction SilentlyContinue |
                    ForEach-Object {
                        $scripts = Join-Path $_.FullName 'Scripts'
                        @('ida-pro-mcp.exe', 'idalib-mcp.exe') | ForEach-Object { Join-Path $scripts $_ }
                    } |
                    Where-Object { Test-Path -LiteralPath $_ } |
                    Select-Object -First 1
                if ($candidate) {
                    $ServerPath = $candidate
                }
            }
        }
    }
}

if ([string]::IsNullOrWhiteSpace($ServerPath)) {
    throw 'Missing required CLI tool: ida-pro-mcp — auto-bootstrap failed. Install manually: pip install git+https://github.com/mrexodia/ida-pro-mcp.git && ida-pro-mcp --install'
}

# 清理旧进程（杀进程树，包括 worker 子进程）
$old = Get-Process -Name "ida-pro-mcp" -ErrorAction SilentlyContinue
if (-not $old) { $old = Get-Process -Name "idalib-mcp" -ErrorAction SilentlyContinue }
if ($old) { taskkill /F /T /PID $old.Id 2>$null | Out-Null; Start-Sleep 2 }

# 后台启动
Start-Process -WindowStyle Hidden -FilePath $ServerPath -ArgumentList "--host 127.0.0.1 --port $Port"

# 等待就绪
$ready = $false
for ($i = 0; $i -lt 15; $i++) {
    Start-Sleep -Seconds 1
    try {
        $r = Invoke-RestMethod "http://127.0.0.1:$Port/mcp" -Method Post `
            -Body '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' `
            -ContentType "application/json" -ErrorAction Stop
        if ($r.result.tools.Count -gt 0) {
            Write-Output "OK:$($r.result.tools.Count)"
            $ready = $true
            break
        }
    } catch {}
}
if (-not $ready) {
    Write-Output "ERR:timeout"
}
