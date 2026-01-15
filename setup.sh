#!/bin/bash

# Multi-platform Setup Script for Archive Duplicate Finder (Linux/macOS)
# Usage: chmod +x setup.sh && ./setup.sh

echo "🚀 Starting environment setup..."

prompt_install() {
    local name=$1
    echo -n "❓ $name is not installed. Would you like to try installing it automatically? (y/n): "
    read -r answer
    if [[ "$answer" =~ ^[Yy]$ ]]; then
        if [[ "$OSTYPE" == "darwin"* ]]; then
            if ! command -v brew &> /dev/null; then
                echo "❌ Homebrew is required for automatic installation on macOS. Install it from https://brew.sh/"
                return 1
            fi
            echo "📦 Installing $name via Homebrew..."
            brew install "$2"
        else
            if command -v apt-get &> /dev/null; then
                echo "📦 Installing $name via apt..."
                sudo apt-get update && sudo apt-get install -y "$3"
            else
                echo "❌ Automatic installation only supports macOS (Homebrew) or Debian/Ubuntu (apt). Please install $name manually."
                return 1
            fi
        fi
        return 0
    fi
    return 1
}

# 1. Check Go
if ! command -v go &> /dev/null; then
    if prompt_install "Go" "go" "golang-go"; then
        echo "✅ Go installed. Please restart your terminal and run this script again."
        exit 0
    else
        echo "❌ Go is required. Install it from https://go.dev/dl/"
        exit 1
    fi
else
    echo "✅ Go is installed: $(go version)"
fi

# 2. Check Node.js & NPM
if ! command -v node &> /dev/null; then
    if prompt_install "Node.js" "node" "nodejs npm"; then
        echo "✅ Node.js installed. Please restart your terminal and run this script again."
        exit 0
    else
        echo "❌ Node.js is required. Install it from https://nodejs.org/"
        exit 1
    fi
else
    echo "✅ Node.js $(node -v) and NPM $(npm -v) are installed."
fi

# 3. Install Backend Dependencies
echo "📦 Installing Go dependencies..."
go mod tidy

# 4. Install Frontend Dependencies
echo "📦 Installing UI dependencies..."
cd ui
npm install
cd ..

echo "✨ Setup complete! You can now build and run the project."
echo "   Build: go build -o archive-finder cmd/finder/main.go"
echo "   Run:   ./archive-finder"
echo "   UI Development: cd ui && npm run dev"
