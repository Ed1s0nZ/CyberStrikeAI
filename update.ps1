# CyberStrikeAI Auto Update Script
param(
    [switch]$Force
)
$RootDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $RootDir

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  CyberStrikeAI Auto Update" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

# 1. Stop service
Write-Host "[1/4] Stopping service..." -ForegroundColor Yellow
$oldPID = $null
$proc = Get-Process CyberStrikeAI -ErrorAction SilentlyContinue
if ($proc) {
    $oldPID = $proc.Id
    # Try Stop-Process first
    $proc | Stop-Process -Force -ErrorAction SilentlyContinue
    Start-Sleep 2
    if (Get-Process -Id $oldPID -ErrorAction SilentlyContinue) {
        # Fallback: elevate to admin
        Write-Host "  Stop-Process failed (access denied), trying elevated taskkill..." -ForegroundColor DarkYellow
        Start-Process -WindowStyle Hidden -Verb RunAs -FilePath taskkill -ArgumentList "/F /PID $oldPID" -Wait
        Start-Sleep 2
        if (Get-Process -Id $oldPID -ErrorAction SilentlyContinue) {
            Write-Host "  WARNING: Cannot kill PID $oldPID (admin process)." -ForegroundColor DarkYellow
        } else {
            Write-Host "  Service stopped (elevated)" -ForegroundColor Green
        }
    } else {
        Write-Host "  Service stopped" -ForegroundColor Green
    }
} else {
    Write-Host "  No running service" -ForegroundColor DarkGray
}

# 2. Pull latest code
Write-Host "[2/4] Pulling latest code..." -ForegroundColor Yellow
$pull = git pull origin main 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Host "  Git pull failed, check network" -ForegroundColor Red
    Write-Host $pull
    # 网络失败不阻断：允许 -Force 继续本地 rebuild
    if (-not $Force) { exit 1 }
}
if ($pull -match "Already up to date") {
    Write-Host "  Already up to date!" -ForegroundColor Green
    $alreadyLatest = $true
} else {
    Write-Host "  Code updated" -ForegroundColor Green
    $alreadyLatest = $false
}

# 2.5. Apply Windows compatibility fixes (idempotent, safe to run every time)
Write-Host "[2.5/4] Applying Windows fixes..." -ForegroundColor Yellow
# 重置 $LASTEXITCODE：fix-windows.ps1 内部只有 cmdlet，不会更新该变量，
# 若前面 git pull 失败过会残留非 0 导致误判
$LASTEXITCODE = 0
& "$RootDir\fix-windows.ps1"
if ($LASTEXITCODE -ne 0) {
    Write-Host "  Windows fixes failed!" -ForegroundColor Red
    exit 1
}

# 3. Rebuild (always when -Force is set, or on code changes, or on Windows local fixes)
$isWindows = [System.Runtime.InteropServices.RuntimeInformation]::IsOSPlatform([System.Runtime.InteropServices.OSPlatform]::Windows)
if ((-not $alreadyLatest) -or $Force -or $isWindows) {
    Write-Host "[3/4] Rebuilding..." -ForegroundColor Yellow
    go build -o CyberStrikeAI.exe cmd/server/main.go
    if ($LASTEXITCODE -ne 0) {
        Write-Host "  Build failed!" -ForegroundColor Red
        exit 1
    }
    Write-Host "  Build success" -ForegroundColor Green
} else {
    Write-Host "[3/4] No rebuild needed" -ForegroundColor DarkGray
}

# 4. Start
Write-Host "[4/4] Starting service..." -ForegroundColor Yellow

# Check if old process still occupies port (admin process we couldn't kill)
$portInUse = netstat -ano 2>$null | Select-String ":9090.*LISTENING"
if ($portInUse) {
    if ($alreadyLatest -and -not $Force) {
        Write-Host "  Port 9090 still occupied by running process, skipping restart (no code changes)" -ForegroundColor DarkGray
        Write-Host "  Service already running at https://127.0.0.1:9090/" -ForegroundColor DarkGray
        Write-Host "========================================" -ForegroundColor Cyan
        exit 0
    } else {
        Write-Host "  ERROR: Port 9090 occupied by admin process. Close it manually then re-run." -ForegroundColor Red
        exit 1
    }
}

Write-Host "========================================" -ForegroundColor Cyan
& .\start.ps1
