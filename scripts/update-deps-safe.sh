#!/usr/bin/env bash
set -euo pipefail

DAYS="${1:-14}"
shift || true

cd "$(dirname "${BASH_SOURCE[0]}")/.."
go run ./cmd/dependencies/main.go -days "$DAYS" "$@"
