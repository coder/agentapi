#!/usr/bin/env bash
set -euo pipefail

mode="${1:-verify}"

if [[ "$mode" != "verify" ]]; then
  echo "Unsupported mode: $mode" >&2
  exit 1
fi

if ! command -v go >/dev/null 2>&1; then
  echo "go is required for quality-gate verification." >&2
  exit 1
fi

echo "==> gofmt check"
fmt_out="$(gofmt -l .)"
if [[ -n "$fmt_out" ]]; then
  echo "Formatting drift detected:"
  echo "$fmt_out"
  exit 1
fi

echo "==> go build"
go build ./...

echo "==> go test"
go test ./...

echo "Quality gate passed."
