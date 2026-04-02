#!/usr/bin/env bash
set -euo pipefail

if [[ -z "${GITHUB_EVENT_PATH:-}" || ! -f "${GITHUB_EVENT_PATH:-}" ]]; then
  echo "GITHUB_EVENT_PATH is missing; cannot evaluate self-merge policy." >&2
  exit 1
fi

author=$(
  python3 - <<'PY'
import json
import os

with open(os.environ["GITHUB_EVENT_PATH"], "r", encoding="utf-8") as fh:
    event = json.load(fh)

print(event.get("pull_request", {}).get("user", {}).get("login", ""))
PY
)

reviewer=$(
  python3 - <<'PY'
import json
import os

with open(os.environ["GITHUB_EVENT_PATH"], "r", encoding="utf-8") as fh:
    event = json.load(fh)

print(event.get("review", {}).get("user", {}).get("login", ""))
PY
)

state=$(
  python3 - <<'PY'
import json
import os

with open(os.environ["GITHUB_EVENT_PATH"], "r", encoding="utf-8") as fh:
    event = json.load(fh)

print(event.get("review", {}).get("state", ""))
PY
)

echo "author=$author"
echo "reviewer=$reviewer"
echo "state=$state"

if [[ "$state" != "approved" ]]; then
  echo "Review is not an approval; nothing to block."
  exit 0
fi

if [[ -z "$author" || -z "$reviewer" ]]; then
  echo "Unable to determine PR author or reviewer." >&2
  exit 1
fi

if [[ "$author" == "$reviewer" ]]; then
  echo "ERROR: PR author cannot self-approve a protected merge." >&2
  exit 1
fi

echo "Self-merge gate passed."
