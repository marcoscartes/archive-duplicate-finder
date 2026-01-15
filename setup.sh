#!/bin/bash

# Multi-platform Setup Script for Archive Duplicate Finder (Linux/macOS)
# Usage: chmod +x setup.sh && ./setup.sh

echo "🚀 Starting environment setup..."

# 1. Check Go
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go from https://go.dev/dl/"
    exit 1
else
    echo "✅ Go is installed: $(go version)"
fi

# 2. Check Node.js & NPM
if ! command -v node &> /dev/null; then
    echo "❌ Node.js is not installed. Please install it from https://nodejs.org/"
    exit 1
fi
if ! command -v npm &> /dev/null; then
    echo "❌ NPM is not installed."
    exit 1
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

# 5. Check GitHub CLI (Optional but recommended for releases)
if ! command -v gh &> /dev/null; then
    echo "⚠️  GitHub CLI (gh) not found. Required if you want to use the release script."
else
    echo "✅ GitHub CLI is installed."
fi

echo "✨ Setup complete! You can now run the project."
echo "   Development: cd ui && npm run dev"
echo "   Build: ./make_release (if on Windows) or manual go build"
