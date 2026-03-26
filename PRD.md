# Product Requirements Document — AgentAPI++

**Module:** `github.com/coder/agentapi` (KooshaPari fork — `agentapi-plusplus`)
**Baseline commit:** `ddaedc2`
**Status:** Active Development

---

## 1. Overview

AgentAPI++ is an HTTP-API layer that wraps CLI-based AI coding agents (Claude Code, Codex, Aider, Gemini CLI, GitHub Copilot, Cursor CLI, Goose, Sourcegraph Amp, Amazon Q, Augment Code) in a unified, programmable interface. It removes the need to manually drive a terminal emulator and exposes agent interaction over REST + SSE so any language or automation platform can programmatically control agents.

The `++` additions over the upstream `coder/agentapi` codebase are:

- `AgentBifrost` routing layer (`internal/routing/`) — per-agent model selection, fallback chaining, and session-aware load balancing sitting between callers and the `cliproxy+bifrost` proxy.
- `harness` package (`internal/harness/`) — Go port of the agent subprocess abstractions from `thegent` (Python), enabling direct subprocess control of Claude Code, Codex, and generic CLIs with stdin injection, ANSI stripping, timeout enforcement, and token/cost telemetry parsing.
- Phenotype SDK init hook (`internal/phenotype/`) — lightweight `.phenotype/` directory bootstrap for Phenotype-org workspace integration.
- Benchmark telemetry store (`internal/benchmarks/`) — token counts and estimated cost from every harness run, surfaced to the `AgentBifrost` for routing decisions.

---

## 2. Goals

| ID | Goal |
|----|------|
| G-1 | Provide a stable HTTP API to send prompts to any supported CLI agent and receive structured responses. |
| G-2 | Maintain per-agent routing rules (preferred model, fallback chain, retry policy) without requiring callers to change. |
| G-3 | Track agent sessions with metadata so multi-turn conversations and load-balancing decisions are session-aware. |
| G-4 | Enable subprocess-level agent control via the `harness` package for callers that need direct process access rather than the HTTP proxy path. |
| G-5 | Capture benchmark telemetry (token counts, cost, duration) from every run for downstream routing and cost accounting. |
| G-6 | Integrate into the Phenotype ecosystem workspace via the `phenotype` SDK init hook. |

---

## 3. Non-Goals

- AgentAPI++ does not implement a model provider itself; it delegates all LLM calls to agents via the `cliproxy+bifrost` proxy or direct CLI subprocess.
- It does not persist conversation history to a database; session state is in-memory.
- It does not provide a user-facing product UI beyond the read-only `/chat` debug interface served on port 3284.

---

## 4. Epics and User Stories

### E1 — HTTP Agent Control

**Description:** External callers (CI pipelines, orchestrators, thegent) send prompts via HTTP and receive structured agent output.

| Story ID | Story | Acceptance Criteria |
|----------|-------|---------------------|
| E1.S1 | As an orchestrator, I can POST a prompt to `/v1/chat/completions` and receive the agent's response. | HTTP 200 with agent output JSON on success; HTTP 4xx/5xx on bad input or agent error. |
| E1.S2 | As a caller, I can GET `/messages` to see all messages in the current session. | Returns ordered list of messages with role and content fields. |
| E1.S3 | As a caller, I can GET `/status` to determine whether the agent is idle or processing. | Returns `{"status": "stable" | "running"}`. |
| E1.S4 | As a caller, I can GET `/events` to receive a real-time SSE stream of message and status events. | SSE stream delivers `message` and `status` event types; connection held open until agent is stable. |

### E2 — Multi-Agent Routing (AgentBifrost)

**Description:** The `AgentBifrost` component selects the correct model and proxy path for each named agent.

| Story ID | Story | Acceptance Criteria |
|----------|-------|---------------------|
| E2.S1 | As an admin, I can GET `/admin/rules` to inspect current per-agent routing rules. | Returns all configured `RoutingRule` entries as JSON. |
| E2.S2 | As an admin, I can POST `/admin/rules` to set or update a routing rule for a named agent. | Rule persisted in memory; subsequent requests for that agent use the new rule immediately. |
| E2.S3 | As a caller, when a preferred model fails, the system retries with fallback models in configured order. | After exhausting `max_retries` across the fallback chain, the final error is returned to the caller. |
| E2.S4 | Requests for unknown agents use a safe default rule: Claude Sonnet as preferred, GPT-4o and Gemini as fallbacks. | Default `RoutingRule` applied when no rule is registered for the given agent name. |

### E3 — Session Management

**Description:** Session state tracks which agent is active, which models have been used, and arbitrary metadata per session.

| Story ID | Story | Acceptance Criteria |
|----------|-------|---------------------|
| E3.S1 | Sessions are created automatically on first request for a given agent name. | `AgentSession` with a unique ID exists in the sessions map after the first `RouteRequest` call. |
| E3.S2 | As an admin, I can GET `/admin/sessions` to see all active sessions. | Returns list of `AgentSession` structs with ID, agent name, start time, and models used. |
| E3.S3 | Sessions are safe for concurrent goroutine access. | `sync.RWMutex` guards the sessions map; no data races under `go test -race`. |

### E4 — Subprocess Agent Harness

**Description:** The `harness` package runs agent CLIs as managed subprocesses.

| Story ID | Story | Acceptance Criteria |
|----------|-------|---------------------|
| E4.S1 | I can invoke `RunHarness("claude", opts)` and receive a `RunResult` with stdout, stderr, exit code, token counts, and cost. | `RunResult.ExitCode == 0` on success; `PromptTokens`, `CompletionTokens`, `CostUSD` populated by parsing agent output. |
| E4.S2 | A timeout-exceeded run sets `RunResult.TimedOut = true` and terminates the subprocess cleanly. | Process killed within 2 seconds of `Timeout` expiry; `TimedOut` field is `true` in the returned result. |
| E4.S3 | Claude harness invokes `claude --print --output-format stream-json --verbose [--model <m>] < <prompt>` with correct flags per `Mode`. | `ModeReadOnly` omits `--dangerously-skip-permissions`; `ModeWrite`/`ModeFull` add it. |
| E4.S4 | Codex harness invokes `codex --full-auto [--model <m>] <prompt>` passing the prompt as an argument rather than stdin. | `RunResult` populated with parsed Codex output; `usesStdin == false` for Codex. |
| E4.S5 | Generic harness handles cursor-agent, copilot, gemini, and opencode via a configurable command template. | Template substitution (model, prompt, mode flags) is applied correctly; `RunResult` populated with parsed output. |

### E5 — Telemetry and Benchmarks

**Description:** Token counts and estimated cost from every harness run are stored and made available for routing decisions.

| Story ID | Story | Acceptance Criteria |
|----------|-------|---------------------|
| E5.S1 | `benchmarks.Store` accumulates `RunResult` telemetry entries without blocking callers. | `Store.Record(result)` returns immediately; data is queryable after the call. |
| E5.S2 | `AgentBifrost` reads benchmark data when making model selection decisions. | Routing logic can factor in average latency and cost per agent/model pair from the store. |

### E6 — Phenotype Workspace Integration

**Description:** The `phenotype` package initialises the `.phenotype/` directory at server startup.

| Story ID | Story | Acceptance Criteria |
|----------|-------|---------------------|
| E6.S1 | `phenotype.Init(repoRoot)` creates `.phenotype/` if absent, without error. | Directory exists after the call; function is idempotent on repeated invocations. |
| E6.S2 | If `repoRoot` is empty, `Init` falls back to the process working directory. | `os.Getwd()` used as fallback; no panic on empty string input. |

---

## 5. Constraints and Assumptions

- Agents must be installed and accessible on `PATH` (or supplied via explicit `cliPath` parameter to harness constructors).
- The `cliproxy+bifrost` URL is required for the `AgentBifrost` HTTP routing path; the harness path bypasses it.
- Session state is ephemeral (in-memory); a process restart loses all session history.
- The default listen port is `3284`; configurable via CLI flag.
- Allowed-hosts enforcement defaults to `localhost` only; overridable via `--allowed-hosts` flag or `AGENTAPI_ALLOWED_HOSTS` environment variable.
- GitHub Actions CI is blocked by billing limits on the KooshaPari account; quality must be verified locally.

AgentAPI++ is an HTTP API gateway that enables programmatic control of AI coding agents (Claude Code, Cursor, Aider, Codex, Gemini CLI, Copilot, Amazon Q, Auggie, Goose, etc.) via RESTful endpoints. It provides a unified interface for spawning agent processes, sending messages, capturing output, parsing responses, and managing persistent agent sessions—enabling orchestration systems, CI/CD pipelines, and IDE integrations to control multiple AI coding agents without CLI knowledge.

**Core Problem**: Each AI coding agent has different CLI interfaces, output formats, and session management approaches. Orchestration systems must implement custom logic for each agent. No standardized API exists.

**Solution**: AgentAPI++ abstracts agent control behind a unified HTTP API:
- Single endpoint to control 10+ different agents (Claude, Cursor, Aider, etc.)
- Message routing with agent-specific formatting and response parsing
- Persistent session management (stateful conversations)
- Real-time event streaming via Server-Sent Events (SSE)
- HTTP/REST interface (no CLI knowledge required)

## 2. Core Architecture

### Design: Terminal Emulator + Message Formatter
- **Terminal Emulator**: In-memory PTY (pseudo-terminal) runs agent CLI process
- **Message Formatter**: Agent-specific protocol handlers for input/output parsing
- **Session Manager**: Maintain conversation state across multiple messages
- **HTTP Gateway**: REST API endpoints + SSE streaming

### Agent Support Matrix
| Agent | Type | CLI Entry | Format Handler | Status |
|-------|------|-----------|-----------------|--------|
| Claude Code | Anthropic | `claude` | ClaudeFormatter | Supported |
| Cursor | IDE-based | `cursor` | CursorFormatter | Supported |
| Aider | OSS | `aider` | AiderFormatter | Supported |
| Codex | OpenAI | `codex` | CodexFormatter | Supported |
| Goose | Independent | `goose` | GooseFormatter | Supported |
| Gemini CLI | Google | `gemini` | GeminiFormatter | Supported |
| GitHub Copilot | GitHub | `github-copilot` | CopilotFormatter | Supported |
| Sourcegraph Amp | Sourcegraph | `amp` | AmpFormatter | Supported |
| Amazon Q | AWS | `amazon-q` | AmazonQFormatter | Supported |
| Auggie | Augment Code | `auggie` | AuggieFormatter | Supported |

## 3. Core Features

### Feature 1: Multi-Agent Control
- Start agent via HTTP API: `POST /api/v0/agents/{type}/start`
- Supported agent types: claude, cursor, aider, codex, gemini, github-copilot, amp, amazon-q, auggie, goose
- Each request spawns isolated agent process with separate session
- Agent process runs in-memory PTY with full terminal capabilities
- Graceful shutdown via API when done

### Feature 2: Stateful Chat Interface
- Send message: `POST /api/v0/chat` with message content and agent type
- Agent receives message, processes asynchronously
- Server responds with 200 when agent starts processing
- Client polls status or uses streaming endpoint for output
- Support persistent sessions across multiple messages in conversation

### Feature 3: Real-Time Streaming (SSE)
- Endpoint `/api/v0/chat/stream` streams agent output as Server-Sent Events
- Client subscribes to events, receives updates in real-time
- Event types: `message-start`, `message-content`, `message-stop`, `tool-call`, `error`
- Streaming continues until agent finishes or client disconnects
- Useful for real-time UI updates and long-running operations

### Feature 4: Session Management
- List active sessions: `GET /api/v0/sessions` returns active conversation IDs
- Get session details: `GET /api/v0/sessions/{id}` returns conversation history
- Sessions persist in-memory for duration of server uptime
- Each session has: unique ID, agent type, message history, created timestamp
- Optional: persist sessions to database for recovery after server restart

### Feature 5: Message Formatting & Parsing
- Each agent has custom formatting for input/output
- Input formatter: convert HTTP request to agent CLI input
- Output parser: capture agent stdout/stderr and parse structured responses
- Formatters handle agent-specific quirks (e.g., Claude response format vs Cursor format)
- Error handling: detect and report agent errors gracefully

### Feature 6: Health Check & Status
- Endpoint `GET /health` returns server health status
- Endpoint `GET /api/v0/agents` lists all supported agents with version info
- Status responses include: server version, agent availability, uptime
- Useful for load balancers and health monitoring

### Feature 7: Authorization & Allowed Hosts
- By default, accept requests only from `localhost`
- Environment variable `AGENTAPI_ALLOWED_HOSTS` configures allowed hosts
- Format: comma-separated hostnames or `*` for any
- Prevents unauthorized remote access
- Host header validation on every request

### Feature 8: Configuration & Customization
- Environment variables configure:
  - `AGENTAPI_PORT` (default 3284)
  - `AGENTAPI_ALLOWED_HOSTS` (default "localhost")
  - `AGENTAPI_TIMEOUT` (default 300s)
  - `AGENTAPI_LOG_LEVEL` (default "info")
- YAML config file support for complex setups
- Per-agent model configuration (e.g., claude-3-5-sonnet for Claude, gpt-4o for Cursor)
- Tool enablement/disablement per agent

### Feature 9: Terminal Output Capture
- Screen tracker captures agent terminal output in real-time
- Supports ANSI escape codes, colors, and formatting
- Snapshot interval: 25ms (40 FPS refresh rate)
- Output stored in memory with configurable buffer size
- Useful for debugging and logging agent behavior

### Feature 10: Integration with cliproxy++ & MCP
- Can route LLM requests through cliproxy++ for cost optimization
- Example: `./agentapi server -- claude --llm-provider http://localhost:8317`
- MCP (Model Context Protocol) integration support
- Allows AgentAPI to work as MCP server within larger agentic systems

## 4. Non-Functional Requirements

### Performance
- Agent startup time: <5s for most agents
- Message processing latency: <200ms for simple messages
- Streaming latency: <100ms per event
- WebSocket support optional but recommended for real-time UIs

### Reliability
- Handle agent crashes gracefully (cleanup, error reporting)
- Support session recovery (persist sessions to disk)
- Timeout handling (kill agent if hung >300s)
- Graceful shutdown (wait for in-flight messages, cleanup)

### Scalability
- Support 100+ concurrent sessions
- Memory usage: ~50MB per session (based on message history)
- CPU usage: negligible when agent not processing

### Security
- Host validation (no remote access without configuration)
- Input validation on all endpoints
- No execution of arbitrary code
- Secrets (API keys) passed via environment, not config files
- TLS support for inter-service communication

### Observability
- Structured logging (request ID, duration, agent type)
- Prometheus metrics (request count, latency, errors)
- Tracing support (OpenTelemetry)
- Debug endpoint to inspect session state

## 5. Data Models

### Session
```
ID: string (UUID)
AgentType: string ("claude" | "cursor" | "aider" | ...)
Messages: [Message]
CreatedAt: timestamp
LastMessageAt: timestamp
Status: "active" | "completed" | "error"
```

### Message
```
Role: "user" | "assistant"
Content: string (message text)
Timestamp: timestamp
Tokens: number (estimated token count)
Model: string (agent model used, if available)
Tools: [ToolCall] (optional, if agent used tools)
```

### ToolCall
```
Name: string (tool name, e.g., "run_shell_command")
Input: object (tool input parameters)
Output: string (tool execution result)
```

### Agent
```
Type: string ("claude" | "cursor" | ...)
Status: "available" | "unavailable"
Version: string
Models: [string] (supported models)
CapabilityFlags: string[] (e.g., "tools", "streaming", "vision")
```

## 6. API Endpoints

### Core Chat
- `POST /api/v0/chat` - Send message, get response
  - Params: `messages` (array), `agent` (string), optional `model`, `temperature`
  - Returns: conversation response
  
- `POST /api/v0/chat/stream` - Send message, stream response via SSE
  - Params: same as above
  - Returns: Server-Sent Events stream

### Sessions
- `GET /api/v0/sessions` - List active sessions
- `GET /api/v0/sessions/{id}` - Get session details and history
- `DELETE /api/v0/sessions/{id}` - Close session

### Agents
- `GET /api/v0/agents` - List supported agents
- `GET /api/v0/agents/{type}` - Get agent details

### Health
- `GET /health` - Server health check
- `GET /metrics` - Prometheus metrics

### OpenAPI
- `GET /openapi.json` - OpenAPI 3.0 spec
- `GET /docs` - Interactive API documentation

## 7. Success Metrics

### Adoption
- Support 10+ different AI coding agents
- Used by 50+ organizations for agent orchestration
- 1000+ daily API requests

### Quality
- Agent integration test coverage >90%
- Message format roundtrip tests for all 10 agents
- Integration tests with real agents (optional in CI)

### Reliability
- Uptime >99.9%
- Handle agent crashes without server crash
- No message loss (persisted sessions)

### Developer Experience
- <5 min to integrate into existing system
- Clear error messages when integration fails
- Comprehensive API documentation

## 8. Roadmap

### Phase 1: Core Gateway (Current)
- HTTP API with 10+ agent support
- Message formatting and parsing
- Session management in-memory
- SSE streaming for real-time output

### Phase 2: Persistence & Resilience
- Session persistence to database
- Agent recovery after crash
- Distributed session storage for multi-instance setup

### Phase 3: Advanced Features
- Tool use execution and result handling
- Vision/image support for agents
- Custom prompt injection per agent
- Rate limiting and quotas per API key

### Phase 4: Enterprise
- Enterprise authentication (SAML, OIDC)
- Audit logging for compliance
- Cost tracking per agent usage
- Multi-tenancy support

---

**Status**: ACTIVE | **Owner**: Engineering Team | **Last Updated**: 2026-03-25
