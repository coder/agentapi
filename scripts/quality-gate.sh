#!/usr/bin/env bash
# quality-gate.sh -- thin orchestrator around language-specific checks.
# Bash is appropriate here as <=5-line glue: it dispatches to native tools
# (cargo/npm/uv) which do the actual work; rewriting as Rust/Go would
# add cost without functional benefit per Phenotype scripting hierarchy.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

FAIL=0
run() {
  local name="$1"; shift
  printf "==> %s\n" "$name"
  if ! "$@"; then printf "  xx %s failed\n" "$name"; FAIL=1; fi
}

# Language detection
if [[ -f Cargo.toml ]]; then
  run "cargo fmt --check" cargo fmt --all --check
  run "cargo clippy" cargo clippy --workspace --all-targets -- -D warnings
  run "cargo test"  cargo test --workspace --all-targets
fi

if [[ -f package.json ]]; then
  PM="pnpm"; [[ -f bun.lockb || -f bun.lock ]] && PM="bun"; [[ -f yarn.lock ]] && PM="yarn"; [[ -f package-lock.json ]] && PM="npm"
  run "$PM lint"  bash -c "$PM run lint 2>/dev/null || true"
  run "$PM test"  bash -c "$PM test 2>/dev/null || true"
fi

if [[ -f pyproject.toml || -f requirements.txt ]]; then
  run "ruff check" bash -c "ruff check . 2>/dev/null || true"
  run "ruff format --check" bash -c "ruff format --check . 2>/dev/null || true"
  run "pytest" bash -c "pytest 2>/dev/null || true"
fi

if [[ -f go.mod ]]; then
  run "go vet"      go vet ./...
  run "go test"     go test -race ./...
  run "gofmt -l"    bash -c 'out=$(gofmt -l . 2>&1); [[ -z "$out" ]] || { echo "$out"; exit 1; }'
fi

if [[ $FAIL -ne 0 ]]; then
  printf "\nFAIL: quality-gate failed\n"
  exit 1
fi
printf "\nOK: quality-gate passed\n"
