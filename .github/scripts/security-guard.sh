#!/usr/bin/env bash
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

if command -v ggshield >/dev/null 2>&1; then
  GGSHIELD=(ggshield)
elif command -v uvx >/dev/null 2>&1; then
  GGSHIELD=(uvx ggshield)
elif command -v uv >/dev/null 2>&1; then
  GGSHIELD=(uv tool run ggshield)
else
  if [ -n "${CI-}" ] || [ -n "${GITHUB_ACTIONS-}" ]; then
    echo "[security-guard] ggshield not installed; skipping secret scan in CI." >&2
    GGSHIELD=()
  else
    echo "ERROR: ggshield not installed. Install with: pipx install ggshield or uv tool install ggshield" >&2
    exit 1
  fi
fi

scan_mode="${1:-pre-commit}"
if [ "${#GGSHIELD[@]}" -gt 0 ]; then
  if [ "${scan_mode}" != "pre-commit" ] && [ "${scan_mode}" != "pre-push" ] && [ "${scan_mode}" != "ci" ]; then
    echo "ERROR: unsupported ggshield mode '${scan_mode}'" >&2
    exit 1
  fi
  echo "[security-guard] Running ggshield secret scan (${scan_mode})"
  "${GGSHIELD[@]}" secret scan "${scan_mode}"
fi

if command -v codespell >/dev/null 2>&1; then
  changed_files_file="$(mktemp)"
  filtered_files_file="$(mktemp)"
  trap 'rm -f "$changed_files_file" "$filtered_files_file"' EXIT
  if [ "${scan_mode}" = "pre-push" ] && [ -n "${PRE_COMMIT_FROM_REF-}" ] && [ -n "${PRE_COMMIT_TO_REF-}" ]; then
    git diff -z --name-only "${PRE_COMMIT_FROM_REF}...${PRE_COMMIT_TO_REF}" 2>/dev/null | tr '\0' '\n' > "$changed_files_file" || true
  else
    git diff -z --cached --name-only --diff-filter=ACM 2>/dev/null | tr '\0' '\n' > "$changed_files_file" || true
  fi
  grep -E '\.(md|txt|py|ts|tsx|js|go|rs|kt|java|yaml|yml)$' "$changed_files_file" > "$filtered_files_file" || true
  if [ -s "$filtered_files_file" ]; then
    echo "[security-guard] Running optional codespell fast pass"
    while IFS= read -r file_path; do
      [ -n "$file_path" ] || continue
      codespell -q 2 -L "hte,teh" -- "$file_path" || true
    done < "$filtered_files_file"
  fi
fi
