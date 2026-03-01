#!/usr/bin/env bash
set -euo pipefail

required_files=(
  "docs/changes/shared-modules/openapi-agent-client-v1/proposal.md"
  "docs/changes/shared-modules/openapi-agent-client-v1/tasks.md"
  "docs/contracts/openapi-agent-client.contract.json"
  "sdk/openapi_agent_client_v1/README.md"
  "scripts/validate_openapi_agent_client_module.sh"
)

fail() {
  echo "VALIDATION FAILED: $1" >&2
  exit 1
}

for path in "${required_files[@]}"; do
  [[ -f "$path" ]] || fail "missing required artifact: $path"
done

contract="docs/contracts/openapi-agent-client.contract.json"

if ! command -v jq >/dev/null 2>&1; then
  fail "jq is required to validate contract json"
fi

jq -e . "$contract" >/dev/null || fail "invalid json in $contract"

jq -e '.openapi_schema_source.path == "openapi.json"' "$contract" >/dev/null \
  || fail "contract must declare openapi_schema_source.path as openapi.json"

declared_openapi_schema_path="$(jq -r '.openapi_schema_source.path' "$contract")"
[[ -n "$declared_openapi_schema_path" && "$declared_openapi_schema_path" != "null" ]] \
  || fail "contract openapi_schema_source.path must be a non-empty string"
[[ -f "$declared_openapi_schema_path" ]] \
  || fail "declared openapi schema path does not exist: $declared_openapi_schema_path"

jq -e '.versioning.policy == "semver"' "$contract" >/dev/null \
  || fail "contract must declare versioning.policy as semver"

jq -e '.sse_event_contract_assumptions.required_event_types | index("message") != null' "$contract" >/dev/null \
  || fail "contract missing required SSE event type: message"

jq -e '.sse_event_contract_assumptions.required_event_types | index("error") != null' "$contract" >/dev/null \
  || fail "contract missing required SSE event type: error"

jq -e '.sse_event_contract_assumptions.required_event_types | index("done") != null' "$contract" >/dev/null \
  || fail "contract missing required SSE event type: done"

echo "VALIDATION PASSED: openapi_agent_client_v1 artifacts are present and contract checks passed"
