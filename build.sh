#!/usr/bin/env bash
# build.sh — builds Go backend (linux/windows amd64) and React frontend dist
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
BACKEND="$ROOT/backend"
FRONTEND="$ROOT/frontend"
OUT="$ROOT/dist"

echo "==> Cleaning output directory..."
rm -rf "$OUT"
mkdir -p "$OUT/linux" "$OUT/windows"

# ---------------------------------------------------------------------------
# Go backend
# ---------------------------------------------------------------------------
echo "==> Downloading Go dependencies..."
cd "$BACKEND"
go mod download

echo "==> Building backend for Linux amd64..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags "-s -w" \
  -o "$OUT/linux/server" \
  ./cmd/server

echo "==> Building backend for Windows amd64..."
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
  go build -trimpath -ldflags "-s -w" \
  -o "$OUT/windows/server.exe" \
  ./cmd/server

# ---------------------------------------------------------------------------
# React frontend
# ---------------------------------------------------------------------------
echo "==> Installing Node dependencies..."
cd "$FRONTEND"
npm ci

echo "==> Building frontend..."
npm run build

echo "==> Copying frontend dist..."
cp -r "$FRONTEND/dist" "$OUT/frontend"

# ---------------------------------------------------------------------------
# Copy config template
# ---------------------------------------------------------------------------
cp "$ROOT/server.yml" "$OUT/linux/server.yml"
cp "$ROOT/server.yml" "$OUT/windows/server.yml"

echo ""
echo "Build complete. Output:"
echo "  $OUT/linux/server          (Linux amd64 binary)"
echo "  $OUT/linux/server.yml      (config template)"
echo "  $OUT/windows/server.exe    (Windows amd64 binary)"
echo "  $OUT/windows/server.yml    (config template)"
echo "  $OUT/frontend/             (React static dist)"
