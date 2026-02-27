#!/usr/bin/env bash
set -euo pipefail
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_FILE="$ROOT_DIR/.generated/doc-index.json"
mkdir -p "$(dirname "$OUT_FILE")"

FILES=$(cd "$ROOT_DIR" && find . -type f \( -name "*.md" -o -name "*.mdx" \) \
  -not -path './.vitepress/*' -not -path './.generated/*' | sed 's#^\./##' | sort)

{
  echo '{'
  echo '  "generatedAt": "'"$(date -u +"%Y-%m-%dT%H:%M:%SZ")"'",'
  echo '  "files": ['
  first=1
  while IFS= read -r f; do
    [ -z "$f" ] && continue
    if [ $first -eq 0 ]; then
      echo ','
    fi
    printf '    "%s"' "$f"
    first=0
  done <<EOF2
$FILES
EOF2
  echo
  echo '  ]'
  echo '}'
} > "$OUT_FILE"
