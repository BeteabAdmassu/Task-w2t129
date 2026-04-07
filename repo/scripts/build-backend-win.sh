#!/bin/bash
# Cross-compile the Go backend for Windows x64 (for Electron packaging).
# Run this before `npm run dist:win` from the frontend directory.
#
# Usage: bash scripts/build-backend-win.sh
# Output: frontend/backend-dist/medops-server.exe
#         frontend/backend-dist/migrations/

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BACKEND_DIR="$REPO_ROOT/backend"
OUT_DIR="$REPO_ROOT/frontend/backend-dist"

echo "Building Go backend for Windows x64..."
mkdir -p "$OUT_DIR"

CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
  go build \
  -ldflags="-s -w" \
  -o "$OUT_DIR/medops-server.exe" \
  "$BACKEND_DIR/cmd/server"

echo "Copying migrations..."
cp -r "$BACKEND_DIR/migrations" "$OUT_DIR/"

echo ""
echo "Backend built successfully:"
ls -lh "$OUT_DIR/"
echo ""
echo "Next steps:"
echo "  cd frontend && npm run dist:win"
