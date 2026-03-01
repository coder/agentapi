# OpenAPI Agent Client v1 Tasks

## Task List
- [x] Create rollout proposal document.
- [x] Create rollout task document.
- [x] Add contract artifact at `docs/contracts/openapi-agent-client.contract.json`.
- [x] Add module scaffold at `sdk/openapi_agent_client_v1/README.md`.
- [x] Add validator script at `scripts/validate_openapi_agent_client_module.sh`.
- [x] Execute validator script once and confirm pass/fail output.

## Validation Command
```bash
bash scripts/validate_openapi_agent_client_module.sh
```

## Expected Outcome
Validation exits non-zero on missing/invalid artifacts and prints explicit failure reasons.
