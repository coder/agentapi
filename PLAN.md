# Implementation Plan — AgentAPI++

**Module:** `github.com/coder/agentapi` (KooshaPari fork)
**Baseline:** AgentAPI upstream (coder/agentapi); fork adds multi-agent routing, Phenotype workspace init, token telemetry, benchmarks store

---

## Phase 1: Core HTTP Server and Single-Agent Control

**Goal**: Establish a working HTTP server that controls a single CLI agent process via PTY and exposes the four canonical AgentAPI endpoints.

| ID | Task | Description | Depends On | Effort |
|----|------|-------------|------------|--------|
| P1.1 | PTY harness | Implement `internal/harness/run_harness.go` — spawn agent subprocess in pseudo-terminal, relay stdin/stdout | — | M |
| P1.2 | Message parsing | Implement `internal/harness/parse.go` — parse agent terminal output into structured `Message` objects | P1.1 | M |
| P1.3 | Session state machine | Implement stable/running state transitions in `internal/harness/runner.go` | P1.2 | S |
| P1.4 | HTTP server skeleton | Implement `internal/server/server.go` with chi router, middleware, health endpoint on port 3284 | — | S |
| P1.5 | GET /messages endpoint | Return conversation message list from session state | P1.3, P1.4 | S |
| P1.6 | POST /message endpoint | Send message to agent, wait for processing acknowledgement, return 200 | P1.3, P1.4 | S |
| P1.7 | GET /status endpoint | Return current agent state (stable/running) | P1.3, P1.4 | XS |
| P1.8 | GET /events SSE stream | Server-sent events for message and status updates via `go-sse` | P1.3, P1.4 | M |
| P1.9 | cobra CLI | Implement `cmd/server/` with `agentapi server -- <agent-cmd>` subcommand and port/allowed-hosts flags | P1.4 | S |
| P1.10 | Allowed-hosts validation | Validate incoming `Host` header against configurable allow list | P1.4 | S |

---

## Phase 2: Multi-Agent Routing (AgentBifrost)

**Goal**: Route requests to the correct agent harness based on agent type, enabling simultaneous control of Claude, Cursor, Aider, Codex, Goose, and others from a single server instance.

| ID | Task | Description | Depends On | Effort |
|----|------|-------------|------------|--------|
| P2.1 | Agent type registry | Define `AgentType` enum covering Claude, Codex, Goose, Gemini, Copilot, Amp, Cursor, Aider, Opencode, Auggie in `internal/routing/base.go` | P1.1 | S |
| P2.2 | Agent-specific harnesses | Implement `claude_harness.go`, `codex_harness.go`, `generic_harness.go` with per-agent prompt formatting | P2.1 | L |
| P2.3 | AgentBifrost router | Implement `internal/routing/agent_bifrost.go` — select and instantiate correct harness from `--type` flag | P2.1, P2.2 | M |
| P2.4 | Agent handler | Implement `internal/server/agent_handler.go` — lifecycle endpoints (start, stop, status) per agent | P2.3, P1.4 | M |
| P2.5 | `/admin` management routes | Register agent lifecycle endpoints under `/admin` prefix in server router | P2.4 | S |
| P2.6 | Auto-detection | Detect agent type from command name when `--type` not specified, with fallback to generic harness | P2.3 | S |
| P2.7 | Bifrost routing tests | Unit tests for `agent_bifrost_test.go` covering type selection, fallback, and unknown agent handling | P2.3 | M |

---

## Phase 3: Token Telemetry and Cost Tracking

**Goal**: Parse token usage from agent output and persist cost/benchmark data so callers can track usage and compare agent efficiency.

| ID | Task | Description | Depends On | Effort |
|----|------|-------------|------------|--------|
| P3.1 | Token parser | Implement token count extraction from agent terminal output in `internal/harness/parse.go` (agent-specific patterns) | P1.2 | M |
| P3.2 | Telemetry middleware | Implement `internal/middleware/` — record token counts, request duration, agent type per request | P3.1, P1.4 | M |
| P3.3 | Benchmarks store | Implement `internal/benchmarks/` — persist per-session token/cost/latency records for comparison | P3.2 | M |
| P3.4 | Cost calculation | Map token counts to cost estimates per agent/model pricing table | P3.1 | S |
| P3.5 | Benchmark retrieval | Expose benchmark history via API endpoint for dashboard consumption | P3.3, P1.4 | S |

---

## Phase 4: Phenotype Workspace Integration

**Goal**: Integrate AgentAPI++ with the Phenotype platform — workspace initialization, config injection, and session context from Phenotype projects.

| ID | Task | Description | Depends On | Effort |
|----|------|-------------|------------|--------|
| P4.1 | Workspace init | Implement `internal/phenotype/` — detect Phenotype workspace (`agents.toml`, `CLAUDE.md`) and inject context | P2.3 | M |
| P4.2 | Agents.toml parsing | Parse `agents.toml` for agent role assignments and default model selection | P4.1 | S |
| P4.3 | Config injection | Inject workspace config into agent startup environment (env vars, working directory) | P4.1, P4.2 | S |
| P4.4 | Phenotype session metadata | Tag sessions with workspace name, project, and agent role for telemetry attribution | P4.1, P3.2 | S |

---

## Phase 5: Chat UI, OpenAPI, and Developer Experience

**Goal**: Ship a browser-accessible chat interface and machine-readable OpenAPI schema so developers can interact with and integrate AgentAPI++ without writing HTTP client code.

| ID | Task | Description | Depends On | Effort |
|----|------|-------------|------------|--------|
| P5.1 | OpenAPI schema generation | Generate `openapi.json` via `go:generate` directive in `main.go` | P1.4–P1.8 | S |
| P5.2 | Swagger/docs UI | Serve API documentation at `/docs` using huma v2 OpenAPI integration | P5.1 | S |
| P5.3 | Chat UI | Serve web-based chat interface at `/chat` directory from `chat/` static assets | P1.5–P1.8 | M |
| P5.4 | SDK (Go) | Publish `sdk/` package wrapping HTTP client calls for Go consumers | P1.4–P1.8 | M |
| P5.5 | Release automation | Implement `release.sh` for cross-platform binary releases (linux/darwin, amd64/arm64) | P1.9 | S |

---

## Phase 6: Testing, CI, and Quality Gates

**Goal**: Establish test coverage, integration tests, and CI pipeline meeting production-readiness standards.

| ID | Task | Description | Depends On | Effort |
|----|------|-------------|------------|--------|
| P6.1 | Harness unit tests | Unit tests for message parsing, state transitions in `test/` | P1.2, P1.3 | M |
| P6.2 | Server integration tests | HTTP integration tests for all 4 endpoints in `internal/server/server_test.go` | P1.5–P1.8 | M |
| P6.3 | E2E tests | End-to-end tests in `e2e/` spawning real agent processes (mocked) | P6.2 | L |
| P6.4 | Coverage gate | Enforce 80% coverage via `coverage.out` + golangci-lint in `Taskfile.yml` | P6.1, P6.2 | S |
| P6.5 | actionlint CI | Lint GitHub Actions workflows via `actionlint.yaml` | — | XS |
| P6.6 | Vendor lockfile | Maintain `vendor/` directory for reproducible builds in CI | P1–P5 | S |

---

## DAG Summary

```
Phase 1 (P1.1–P1.10)
    └─► Phase 2 (P2.1–P2.7)
            └─► Phase 3 (P3.1–P3.5)
            └─► Phase 4 (P4.1–P4.4)  [parallel with P3]
                    └─► Phase 5 (P5.1–P5.5)
                            └─► Phase 6 (P6.1–P6.6)
```

P3 and P4 are independent of each other and can run in parallel after Phase 2 completes.

---

## Effort Key

| Symbol | Agent Effort |
|--------|-------------|
| XS | 1-2 tool calls |
| S | 3-5 tool calls |
| M | 6-10 tool calls |
| L | 11-20 tool calls |
