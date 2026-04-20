#!/bin/bash

set -e

mkdir -p dist

echo "Building TransForward..."

# Clean old builds
rm -rf dist/*

# Get version
VERSION=${VERSION:-"1.0.0"}

# Build for multiple platforms
echo "Building for Windows..."
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -X main.version=$VERSION" -o dist/transforward-windows-amd64.exe .

echo "Building for Linux..."
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X main.version=$VERSION" -o dist/transforward-linux-amd64 .

echo "Building for Linux ARM64..."
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w -X main.version=$VERSION" -o dist/transforward-linux-arm64 .

echo "Building for macOS..."
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w -X main.version=$VERSION" -o dist/transforward-darwin-amd64 .

echo "Building for macOS ARM64..."
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w -X main.version=$VERSION" -o dist/transforward-darwin-arm64 .

echo ""
echo "Build complete! Output in dist/:"
ls -la dist/
