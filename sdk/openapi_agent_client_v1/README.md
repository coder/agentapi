# openapi_agent_client_v1

## Purpose
This module namespace is reserved for the v1 OpenAPI-derived agent client contract and implementation surface.

## Usage Boundaries
- Treat this module as the canonical client boundary for consumers integrating with `agentapi++` OpenAPI endpoints.
- Drive generated/request models from `openapi.json` as declared in `docs/contracts/openapi-agent-client.contract.json`.
- Preserve the SSE assumptions defined in the contract artifact for stream consumers.
- Do not introduce fallback clients, compatibility shims, or parallel `v2` namespace files.

## Migration Notes
- Existing consumers should migrate to this module boundary as the authoritative SDK surface once implementation lands.
- Breaking protocol or schema changes must increment major version per contract semver rules.
- Additive endpoint and optional SSE event changes are minor-version updates.
- Documentation-only clarifications are patch updates.
