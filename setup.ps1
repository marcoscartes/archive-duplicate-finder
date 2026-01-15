# Multi-platform Setup Script for Archive Duplicate Finder (Windows)
# Usage: .\setup.ps1

Write-Host "🚀 Starting environment setup for Windows..." -ForegroundColor Cyan

# 1. Check Go
$goPath = Get-Command go -ErrorAction SilentlyContinue
if (-not $goPath) {
    Write-Host "❌ Go is not installed. Please install Go from https://go.dev/dl/" -ForegroundColor Red
    exit 1
} else {
    Write-Host "✅ Go is installed: $(go version)" -ForegroundColor Green
}

# 2. Check Node.js & NPM
$nodePath = Get-Command node -ErrorAction SilentlyContinue
$npmPath = Get-Command npm -ErrorAction SilentlyContinue
if (-not $nodePath -or -not $npmPath) {
    Write-Host "❌ Node.js or NPM not found. Please install from https://nodejs.org/" -ForegroundColor Red
    exit 1
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
