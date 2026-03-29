#!/usr/bin/env bash
set -euo pipefail
if [ -f package-lock.json ] || [ -f yarn.lock ] || [ -f pnpm-lock.yaml ]; then
  echo "ERROR: Non-bun lockfile found. Use bun exclusively."
  exit 1
fi
if [ -n "${npm_execpath:-}" ] && echo "$npm_execpath" | grep -qv "bun"; then
  echo "ERROR: Use bun, not npm/yarn/pnpm."
  exit 1
fi
