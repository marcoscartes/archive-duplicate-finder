# Multi-platform Setup Script for Archive Duplicate Finder (Windows)
# Usage: .\setup.ps1

Write-Host "🚀 Starting environment setup for Windows..." -ForegroundColor Cyan

function Prompt-Install($name, $command) {
    $title = "Missing Dependency"
    $message = "$name is not installed. Would you like to try installing it automatically via Winget?"
    $options = [System.Management.Automation.Host.ChoiceDescription[]] @(
        New-Object System.Management.Automation.Host.ChoiceDescription "&Yes", "Try automatic installation"
        New-Object System.Management.Automation.Host.ChoiceDescription "&No", "Skip and exit"
    )
    $result = $host.ui.PromptForChoice($title, $message, $options, 0)
    if ($result -eq 0) {
        Write-Host "📦 Attempting to install $name..." -ForegroundColor Yellow
        Invoke-Expression $command
        return $true
    }
    return $false
}

# 1. Check Go
$goPath = Get-Command go -ErrorAction SilentlyContinue
if (-not $goPath) {
    if (Prompt-Install "Go Programming Language" "winget install GoLang.Go") {
        Write-Host "✅ Installation triggered. Please RESTART your terminal after it finishes and run this script again." -ForegroundColor Green
        exit 0
    } else {
        Write-Host "❌ Go is required. Install it manually from https://go.dev/dl/" -ForegroundColor Red
        exit 1
    }
} else {
    Write-Host "✅ Go is installed: $(go version)" -ForegroundColor Green
}

# 2. Check Node.js & NPM
$nodePath = Get-Command node -ErrorAction SilentlyContinue
if (-not $nodePath) {
    if (Prompt-Install "Node.js (LTS)" "winget install OpenJS.NodeJS.LTS") {
        Write-Host "✅ Installation triggered. Please RESTART your terminal after it finishes and run this script again." -ForegroundColor Green
        exit 0
    } else {
        Write-Host "❌ Node.js is required. Install it from https://nodejs.org/" -ForegroundColor Red
        exit 1
    }
} else {
    Write-Host "✅ Node.js $((node -v)) and NPM $((npm -v)) are installed." -ForegroundColor Green
}

# 3. Install Backend Dependencies
Write-Host "📦 Installing Go dependencies..." -ForegroundColor Yellow
go mod tidy

# 4. Install Frontend Dependencies
Write-Host "📦 Installing UI dependencies (this may take a minute)..." -ForegroundColor Yellow
Set-Location ui
npm install
Set-Location ..

Write-Host "✨ Setup complete! You can now build and run the project." -ForegroundColor Cyan
Write-Host "   Build: go build -o archive-finder.exe cmd/finder/main.go"
Write-Host "   Run:   .\archive-finder.exe"
Write-Host "   UI Development: cd ui; npm run dev"
