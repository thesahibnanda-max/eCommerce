#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

for tool in go npm node; do
  command -v "$tool" >/dev/null 2>&1 || { echo "build: $tool not found in PATH" >&2; exit 1; }
done

echo "==> frontend: npm ci + build"
go generate ./frontend

echo "==> backend: go build"
go build ./...
go build -trimpath -o "$root/eCommerce" .

echo "==> built $root/eCommerce"
