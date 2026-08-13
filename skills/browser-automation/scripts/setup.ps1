#requires -Version 5

<#
.SYNOPSIS
Bootstrap script for agent-browser and Playwright.
Installs Node.js (if missing), agent-browser CLI, and Playwright browsers.
#>

[CmdletBinding()]
param(
    [switch]$SkipBrowserInstall
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

. (Join-Path $PSScriptRoot '..\..\scripts\lib\ToolDiscovery.ps1')

function Write-Step {
    param([string]$Message)
    Write-Host "[$((Get-Date).ToString('HH:mm:ss'))] $Message" -ForegroundColor Cyan
}

# Step 1: Ensure Node.js
Write-Step "Checking Node.js..."
$node = Get-Command node -ErrorAction SilentlyContinue
if (-not $node) {
    Write-Step "Node.js not found, attempting install via winget..."
    $winget = Get-Command winget -ErrorAction SilentlyContinue
    if ($winget) {
        & winget install --id OpenJS.NodeJS.22 --source winget --accept-package-agreements --accept-source-agreements --disable-interactivity
        if ($LASTEXITCODE -ne 0) {
            throw "Failed to install Node.js via winget. Install manually: https://nodejs.org/"
        }
        # Refresh PATH
        $env:PATH = [System.Environment]::GetEnvironmentVariable('PATH', 'Machine') + ';' + [System.Environment]::GetEnvironmentVariable('PATH', 'User')
    }
    else {
        throw "Node.js not found and winget not available. Install Node.js manually: https://nodejs.org/"
    }
}
$nodeVersion = & node -v
Write-Step "Node.js: $nodeVersion"

# Step 2: Ensure npm
$npm = Get-Command npm -ErrorAction SilentlyContinue
if (-not $npm) {
    throw "npm not found. Reinstall Node.js: https://nodejs.org/"
}

# Step 3: Install agent-browser globally
Write-Step "Checking agent-browser..."
$agentBrowser = Get-Command agent-browser -ErrorAction SilentlyContinue
if (-not $agentBrowser) {
    Write-Step "Installing agent-browser globally..."
    & npm install -g agent-browser
    if ($LASTEXITCODE -ne 0) {
        Write-Warning "agent-browser global install failed. Trying npx fallback..."
    }
    else {
        Write-Step "agent-browser installed."
    }
}
else {
    Write-Step "agent-browser already available: $($agentBrowser.Source)"
}

# Step 4: Install Playwright and browsers
if (-not $SkipBrowserInstall) {
    Write-Step "Installing Playwright browsers (chromium)..."
    & npx playwright install chromium
    if ($LASTEXITCODE -ne 0) {
        Write-Warning "Playwright browser install failed. You may need to run: npx playwright install chromium"
    }
    else {
        Write-Step "Playwright chromium installed."
    }

    # Record browser location
    $playwrightPath = $null
    try {
        $envBrowsersPath = $env:PLAYWRIGHT_BROWSERS_PATH
        if ([string]::IsNullOrWhiteSpace($envBrowsersPath)) {
            # Default Playwright browser cache location on Windows
            $defaultPath = Join-Path $env:LOCALAPPDATA 'ms-playwright'
            if (Test-Path -LiteralPath $defaultPath) {
                $playwrightPath = $defaultPath
            }
        }
        else {
            $playwrightPath = $envBrowsersPath
        }
    }
    catch {}

    if ($playwrightPath) {
        Write-Step "Playwright browsers location: $playwrightPath"
        "playwright_browsers_path=$playwrightPath"
    }
}

# Step 5: Verify
Write-Step "Verification:"
$finalCheck = Get-Command agent-browser -ErrorAction SilentlyContinue
if ($finalCheck) {
    "agent-browser=$($finalCheck.Source)"
    "status=ready"
}
else {
    # Fallback: can still use via npx
    $npxCheck = Get-Command npx -ErrorAction SilentlyContinue
    if ($npxCheck) {
        "agent-browser=npx agent-browser"
        "status=ready-via-npx"
    }
    else {
        "status=failed"
        throw "agent-browser installation failed. Install manually: npm install -g agent-browser"
    }
}
