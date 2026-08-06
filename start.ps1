# CyberStrikeAI Windows quick start script
$RootDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $RootDir

# Add Git Bash to PATH (required for shell commands on Windows)
$gitBin = "C:\Program Files\Git\bin"
if ((Test-Path $gitBin) -and ($env:PATH -notmatch [regex]::Escape($gitBin))) {
    $env:PATH = "$gitBin;$env:PATH"
}

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  CyberStrikeAI Starting..." -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan

# Check binary
if (-not (Test-Path "CyberStrikeAI.exe")) {
    Write-Host "[*] Binary not found, building..." -ForegroundColor Yellow
    go build -o CyberStrikeAI.exe cmd/server/main.go
    if ($LASTEXITCODE -ne 0) {
        Write-Host "[X] Build failed, check Go env" -ForegroundColor Red
        exit 1
    }
}

# Parse args
$UseHttp = $false
$ForwardArgs = @()
foreach ($arg in $args) {
    if ($arg -eq "--http") { $UseHttp = $true }
    elseif ($arg -eq "--reset-admin-password") {
        & .\CyberStrikeAI.exe -config config.yaml --reset-admin-password
        exit $LASTEXITCODE
    }
    else { $ForwardArgs += $arg }
}

# Ensure Python subprocesses output UTF-8 (fix GBK UnicodeEncodeError in tool wrappers)
$env:PYTHONIOENCODING = "utf-8"

# Start (background, survives shell exit)
if ($UseHttp) {
    Write-Host "[*] HTTP mode: https://127.0.0.1:9090" -ForegroundColor Green
    $proc = Start-Process -FilePath ".\CyberStrikeAI.exe" -ArgumentList "-config config.yaml --http $ForwardArgs" -WindowStyle Hidden -PassThru
} else {
    Write-Host "[*] HTTPS mode: https://127.0.0.1:9090" -ForegroundColor Green
    Write-Host "[*] Self-signed cert, accept browser warning" -ForegroundColor DarkGray
    $proc = Start-Process -FilePath ".\CyberStrikeAI.exe" -ArgumentList "-config config.yaml --https $ForwardArgs" -WindowStyle Hidden -PassThru
}

Start-Sleep 3
$code = curl.exe -sk -o NUL -w '%{http_code}' https://127.0.0.1:9090/ 2>$null
if ($code -eq '200') {
    Write-Host "[OK] Service started (PID $($proc.Id))" -ForegroundColor Green
} else {
    Write-Host "[!] Service may still be starting..." -ForegroundColor Yellow
}
