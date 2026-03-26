# Architecture Decision Records — AgentAPI++

**Module:** `github.com/coder/agentapi` (KooshaPari fork — `agentapi-plusplus`)
**Baseline commit:** `ddaedc2`

---

## ADR-001: Fork coder/agentapi and extend rather than build from scratch

**Status:** Accepted

**Context:**
The upstream `coder/agentapi` project already provides a battle-tested PTY/terminal-emulation layer for wrapping CLI agents over HTTP, SSE, and a structured message model. Building equivalent PTY handling from scratch is a significant engineering effort with high failure risk.

**Decision:**
Fork `coder/agentapi` as `agentapi-plusplus` and add Phenotype-specific extensions in separate packages (`internal/routing/`, `internal/harness/`, `internal/phenotype/`, `internal/benchmarks/`) without modifying upstream-derived code paths where avoidable.

**Consequences:**
- Upstream improvements can be merged periodically by rebasing.
- The fork maintains the upstream module path (`github.com/coder/agentapi`) to avoid breaking SDK-generated clients.
- Extensions are isolated in internal packages; upstream code lives in `agentapi/`, `chat/`, `lib/`, `sdk/`.

---

## ADR-002: Use AgentBifrost as the agent-routing middleware layer

**Status:** Accepted

**Context:**
Multiple AI coding agents (Claude, Codex, Gemini, Copilot, etc.) need to be addressable through a single HTTP endpoint. Each agent may have a preferred LLM model, fallback models, and per-agent retry policies. Encoding these routing concerns in the HTTP handler directly would conflate routing logic with transport concerns.

**Decision:**
Introduce `AgentBifrost` (`internal/routing/agent_bifrost.go`) as a dedicated routing struct. It owns:
- The `cliproxy+bifrost` HTTP client
- The per-agent `RoutingRule` map (guarded by `sync.RWMutex`)
- The per-agent `AgentSession` map (guarded by `sync.RWMutex`)
- The `benchmarks.Store` reference

The HTTP server delegates all routing decisions to `AgentBifrost.RouteRequest(ctx, agent, prompt)`.

**Consequences:**
- Routing rules and sessions can be inspected and mutated via `/admin` endpoints without touching transport code.
- Fallback model chaining is contained within `RouteRequest`; callers are unaware of retries.
- The `AgentBifrost` can be unit-tested independently of the HTTP server.
- Session state is in-memory only; a process restart loses all sessions (accepted trade-off for simplicity).

---

## ADR-003: Port agent subprocess harnesses from Python (thegent) to Go

**Status:** Accepted

**Context:**
The `thegent` Python project (`src/thegent/agents/base.py`, `direct_agents.py`) already encapsulates how to invoke each agent CLI correctly — flag sets, stdin vs argument delivery, ANSI stripping, token parsing. Duplicating this logic in an ad-hoc way in Go would create drift and maintenance burden.

**Decision:**
Implement the `harness` package (`internal/harness/`) as a direct Go port of the Python harness abstractions. The package defines:
- `Runner` interface (agent-agnostic contract)
- `baseRunner` (shared helpers: subprocess execution, ANSI stripping, token/cost parsing, timeout enforcement)
- `ClaudeHarness`, `CodexHarness`, `GenericHarness` (agent-specific CLI invocation)
- `RunHarness(agent, opts)` as the top-level dispatch

Document the Python origin in package-level comments (`// Ported from thegent src/thegent/agents/...`).

**Consequences:**
- Changes to upstream agent CLIs must be reflected in both Python and Go harnesses.
- The `harness` package can be used independently of the HTTP server for programmatic agent invocation.
- ANSI stripping depends on `github.com/acarl005/stripansi`.

---

## ADR-004: Use chi router with middleware.Recoverer and middleware.Logger

**Status:** Accepted

**Context:**
The project requires a lightweight HTTP router with good middleware composability. The upstream `coder/agentapi` already uses `go-chi/chi` for its server.

**Decision:**
Use `go-chi/chi/v5` for HTTP routing. Apply `middleware.Recoverer` (panic recovery) and `middleware.Logger` (request logging) globally. Use `go-chi/cors` for CORS headers.

**Alternatives considered:**
- `net/http` ServeMux — insufficient for route parameters and middleware chains.
- `gin` — heavier dependency, deviates from upstream.
- `echo` — same reason.

**Consequences:**
- Route group nesting (`r.Route("/admin", ...)`) cleanly separates public and admin endpoints.
- `middleware.Recoverer` prevents panics from crashing the server.
- Additional chi middleware can be added per-route without modifying unrelated handlers.

---

## ADR-005: In-memory session and rule state; no persistence layer

**Status:** Accepted

**Context:**
Persisting routing rules and session metadata to a database would add significant operational complexity (schema management, migrations, connection pooling) for a component that is designed to be stateless and restarted freely.

**Decision:**
Store both `RoutingRule` entries and `AgentSession` entries in `sync.RWMutex`-guarded maps within the `AgentBifrost` struct. No external store is required.

**Alternatives considered:**
- SQLite via `mattn/go-sqlite3` — CGo dependency, binary size increase, migration management.
- Redis — requires an additional service dependency.
- File-based JSON — introduces I/O failure modes and file-locking complexity.

**Consequences:**
- A process restart loses all dynamically-registered rules (use default rules or re-register at startup).
- Session history is not durable across restarts.
- No dependency on an external store simplifies deployment to a single binary.
- Rules that must survive restarts should be baked into the server startup configuration.

---

## ADR-006: Benchmark/telemetry store in-process; not exported to external sink

**Status:** Accepted

**Context:**
Token counts and cost estimates parsed from agent output are needed by `AgentBifrost` to inform routing decisions (e.g., avoid a model that is consistently expensive or slow). An external metrics sink would decouple collection from use, but adds operational overhead.

**Decision:**
Implement `benchmarks.Store` as an in-process ring-buffer or append-only slice. `AgentBifrost` holds a reference to the store and can read aggregate statistics when selecting models.

**Alternatives considered:**
- Prometheus metrics — useful for observability but not queryable for routing decisions at runtime without additional logic.
- OpenTelemetry spans — structured but not aggregable without an OTLP collector.

**Consequences:**
- Benchmark data is lost on restart (same trade-off as ADR-005).
- No additional infrastructure required.
- If external observability is needed in future, `benchmarks.Store` can be extended to emit to a Prometheus endpoint without changing the routing interface.

---

## ADR-007: Use go-sse for Server-Sent Events

**Status:** Accepted

**Context:**
The `/events` endpoint must stream agent `message` and `status` events to clients over SSE. Implementing SSE from scratch with `http.Flusher` is error-prone (keep-alive, event ID, retry fields).

**Decision:**
Use `github.com/tmaxmax/go-sse` which provides a well-tested SSE server and replay buffer.

**Consequences:**
- SSE replay on reconnect is available if configured.
- Dependency on an external library; pin to a specific version in `go.mod`.

---

## ADR-008: Phenotype SDK init as a lightweight directory bootstrap only

**Status:** Accepted

**Context:**
The Phenotype ecosystem expects a `.phenotype/` directory at the workspace root. The full Phenotype config SDK is a Rust library exposed via CGo, which introduces a CGo compile dependency. Not all callers of `agentapi-plusplus` will have the Rust toolchain available.

**Decision:**
Implement `internal/phenotype/init.go` as a pure-Go, CGo-free function that only creates the `.phenotype/` directory. Document in package comments that actual SDK calls require the CGo bindings (`phenoconfig` package). This init hook satisfies the minimum workspace integration contract without imposing a Rust build dependency.

**Consequences:**
- No CGo dependency for the core binary.
- Callers that need the full Phenotype SDK must link the CGo bindings separately.
- `phenotype.Init` is idempotent and safe to call unconditionally at startup.
