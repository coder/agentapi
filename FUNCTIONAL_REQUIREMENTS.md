# Functional Requirements — AgentAPI++

**Module:** `github.com/coder/agentapi` (KooshaPari fork — `agentapi-plusplus`)
**Baseline commit:** `ddaedc2`
**Traces to:** PRD.md epics E1–E6

---

## Category Index

| Category | Prefix | Scope |
|----------|--------|-------|
| HTTP API | FR-HTTP | REST endpoints, SSE, allowed-hosts |
| Routing | FR-ROUTE | AgentBifrost, rules, fallback |
| Session | FR-SESS | Session lifecycle and concurrency |
| Harness | FR-HARN | Subprocess control, CLI invocation |
| Telemetry | FR-TELE | Token parsing, cost, benchmarks store |
| Phenotype | FR-PHENO | Workspace init |
| Security | FR-SEC | Allowed-hosts, input validation |

---

## FR-HTTP — HTTP API

### FR-HTTP-001
**SHALL** start an HTTP server on port `3284` by default when `agentapi server` is invoked.
- Traces to: E1.S1
- Code: `internal/server/server.go` `Server.Start()`

### FR-HTTP-002
**SHALL** accept `POST /v1/chat/completions` with a JSON body containing `agent` (string), `model` (string), and `prompt` (string) fields.
- Traces to: E1.S1
- Code: `internal/server/server.go` `chatCompletions()`

### FR-HTTP-003
**SHALL** return HTTP 200 with a JSON response body on successful agent routing.
- Traces to: E1.S1

### FR-HTTP-004
**SHALL** return HTTP 400 when the request body cannot be decoded as valid JSON.
- Traces to: E1.S1

### FR-HTTP-005
**SHALL** return HTTP 500 when the underlying `AgentBifrost.RouteRequest` call returns a non-nil error after all retries are exhausted.
- Traces to: E1.S1, E2.S3

### FR-HTTP-006
**SHALL** expose `GET /messages` returning an ordered list of all messages in the current session.
- Traces to: E1.S2
- Code: `internal/server/agent_handler.go`

### FR-HTTP-007
**SHALL** expose `GET /status` returning a JSON object with a `status` field whose value is either `"stable"` or `"running"`.
- Traces to: E1.S3

### FR-HTTP-008
**SHALL** expose `GET /events` as a Server-Sent Events stream delivering `message` and `status` event types.
- Traces to: E1.S4
- Dependency: `github.com/tmaxmax/go-sse`

### FR-HTTP-009
**SHALL** expose `GET /health` returning `{"status":"ok"}` with HTTP 200; this endpoint is not subject to allowed-hosts enforcement.
- Traces to: G-1
- Code: `internal/server/server.go` `health()`

### FR-HTTP-010
**SHALL** expose `GET /admin/rules`, `POST /admin/rules`, and `GET /admin/sessions` under the `/admin` route group.
- Traces to: E2.S1, E2.S2, E3.S2

### FR-HTTP-011
**SHALL** expose `POST /message` to send a message to the agent; response indicates the agent has begun processing (HTTP 200).
- Traces to: E1.S1
- Code: `internal/server/agent_handler.go`

---

## FR-ROUTE — Multi-Agent Routing (AgentBifrost)

### FR-ROUTE-001
**SHALL** create an `AgentBifrost` instance configured with the `cliproxy+bifrost` base URL at server startup.
- Traces to: G-2
- Code: `internal/routing/agent_bifrost.go` `NewAgentBifrost()`

### FR-ROUTE-002
**SHALL** apply a per-agent `RoutingRule` when forwarding a request; the rule specifies `preferred_model`, `fallback_models`, `max_retries`, and `timeout_seconds`.
- Traces to: E2.S2, E2.S3
- Code: `RoutingRule` struct, `getRule()`

### FR-ROUTE-003
**SHALL** apply the following default `RoutingRule` when no rule is registered for the given agent name:
  - `preferred_model`: `claude-3-5-sonnet-20241022`
  - `fallback_models`: `["gpt-4o", "gemini-1.5-pro"]`
  - `max_retries`: 3
  - `timeout_seconds`: 30
- Traces to: E2.S4
- Code: `getRule()` default branch

### FR-ROUTE-004
**SHALL** forward requests to the `cliproxy+bifrost` URL at path `/v1/chat/completions` using `POST` with `Content-Type: application/json`.
- Traces to: E2.S1
- Code: `forwardToCliproxy()`

### FR-ROUTE-005
**SHALL** retry with each fallback model in order when the preferred model request returns an error; stop on the first successful response.
- Traces to: E2.S3
- Code: `RouteRequest()` fallback loop

### FR-ROUTE-006
**SHALL** allow `POST /admin/rules` to register or replace a `RoutingRule` for a named agent; the update MUST be reflected in all subsequent requests for that agent.
- Traces to: E2.S2
- Code: `SetRule()`

### FR-ROUTE-007
**SHALL** protect the `rules` map with `sync.RWMutex`; reads use `RLock`, writes use `Lock`.
- Traces to: E2.S2 (concurrency safety)
- Code: `rulesMut` in `AgentBifrost`

### FR-ROUTE-008
**SHALL** include the session ID in the request body forwarded to `cliproxy+bifrost` under the `session` key.
- Traces to: E3.S1
- Code: `RouteRequest()` body assembly

---

## FR-SESS — Session Management

### FR-SESS-001
**SHALL** create an `AgentSession` record the first time a request is routed for a given agent name if no session yet exists.
- Traces to: E3.S1
- Code: `getOrCreateSession()`

### FR-SESS-002
**SHALL** assign each session a unique string identifier.
- Traces to: E3.S1

### FR-SESS-003
**SHALL** record the session start time (`Started time.Time`) on creation.
- Traces to: E3.S2

### FR-SESS-004
**SHALL** track which models have been used in the session under the `Models []string` field.
- Traces to: E3.S2

### FR-SESS-005
**SHALL** protect the `sessions` map with `sync.RWMutex`; reads use `RLock`, writes use `Lock`.
- Traces to: E3.S3
- Code: `sessionsMut` in `AgentBifrost`

### FR-SESS-006
**SHALL** return all active `AgentSession` records when `GET /admin/sessions` is called.
- Traces to: E3.S2

---

## FR-HARN — Subprocess Agent Harness

### FR-HARN-001
**SHALL** define a `Runner` interface with methods `AgentName() string` and `Run(ctx context.Context, opts RunOptions) (RunResult, error)`.
- Traces to: E4.S1
- Code: `internal/harness/base.go`

### FR-HARN-002
**SHALL** implement `ClaudeHarness` that resolves the `claude` binary from `PATH` when no explicit path is given, and invokes it as:
  `claude --print [--dangerously-skip-permissions] [--add-dir <workdir>] --output-format stream-json --verbose [--model <model>]`
  with the prompt written to stdin.
- Traces to: E4.S3
- Code: `internal/harness/claude_harness.go`

### FR-HARN-003
**SHALL** add `--dangerously-skip-permissions` to the Claude invocation for `ModeWrite` and `ModeFull`; omit it for `ModeReadOnly`.
- Traces to: E4.S3

### FR-HARN-004
**SHALL** implement `CodexHarness` that invokes the `codex` binary as:
  `codex --full-auto [--model <model>] <prompt>`
  passing the prompt as a command-line argument, not via stdin (`usesStdin == false`).
- Traces to: E4.S4
- Code: `internal/harness/codex_harness.go`

### FR-HARN-005
**SHALL** implement `GenericHarness` that handles cursor-agent, copilot, gemini, and opencode via a configurable command template with model, prompt, and mode-flag substitution.
- Traces to: E4.S5
- Code: `internal/harness/generic_harness.go`

### FR-HARN-006
**SHALL** strip ANSI escape codes from captured stdout and stderr before storing them in `RunResult`.
- Traces to: E4.S1
- Dependency: `github.com/acarl005/stripansi`

### FR-HARN-007
**SHALL** enforce the `RunOptions.Timeout` duration; if the subprocess does not exit within the timeout, it MUST be killed and `RunResult.TimedOut` set to `true`.
- Traces to: E4.S2

### FR-HARN-008
**SHALL** parse `PromptTokens`, `CompletionTokens`, and `CostUSD` from agent output (stdout or stderr) and populate the corresponding `RunResult` fields.
- Traces to: E4.S1, E5.S1
- Code: `internal/harness/parse.go`

### FR-HARN-009
**SHALL** expose `RunHarness(agent string, opts RunOptions) (RunResult, error)` as the top-level dispatch function that selects the correct harness by agent name.
- Traces to: E4.S1
- Code: `internal/harness/run_harness.go`

### FR-HARN-010
**SHALL** propagate the `context.Context` cancellation signal to the subprocess; a cancelled context MUST terminate the subprocess.
- Traces to: E4.S2

---

## FR-TELE — Telemetry and Benchmarks

### FR-TELE-001
**SHALL** provide a `benchmarks.Store` type with a `Record(result RunResult)` method that stores the run result for later querying.
- Traces to: E5.S1
- Code: `internal/benchmarks/`

### FR-TELE-002
**SHALL** initialise a `benchmarks.Store` inside `AgentBifrost` and make it accessible to routing decision logic.
- Traces to: E5.S2
- Code: `AgentBifrost.benchmarks` field

### FR-TELE-003
**SHALL** store at minimum: agent name, model used, `PromptTokens`, `CompletionTokens`, `CostUSD`, and `Duration` per recorded run.
- Traces to: E5.S1

---

## FR-PHENO — Phenotype Workspace Integration

### FR-PHENO-001
**SHALL** provide `phenotype.Init(repoRoot string) error` that creates the `.phenotype/` directory at `repoRoot` with permissions `0o755` if it does not exist.
- Traces to: E6.S1
- Code: `internal/phenotype/init.go`

### FR-PHENO-002
**SHALL** use `os.Getwd()` as the fallback root when the `repoRoot` argument is an empty string.
- Traces to: E6.S2

### FR-PHENO-003
**SHALL** be idempotent: calling `Init` on a directory where `.phenotype/` already exists MUST return `nil` without error.
- Traces to: E6.S1

---

## FR-SEC — Security

### FR-SEC-001
**SHALL** reject HTTP requests whose `Host` header does not match a configured allowed-host entry, returning HTTP 403, unless `--allowed-hosts "*"` is set.
- Traces to: G-1
- Code: `internal/middleware/`

### FR-SEC-002
**SHALL** support configuring allowed hosts via the `--allowed-hosts` CLI flag or the `AGENTAPI_ALLOWED_HOSTS` environment variable.
- Traces to: FR-SEC-001

### FR-SEC-003
**SHALL** default to allowing only `localhost` as the allowed host when neither flag nor environment variable is set.
- Traces to: FR-SEC-001

### FR-SEC-004
**SHALL** use `go-chi/cors` middleware to set appropriate CORS headers on all API responses.
- Traces to: G-1
- Dependency: `github.com/go-chi/cors`

### FR-SEC-005
**SHALL NOT** log the content of agent prompts or responses at `INFO` level or below; only metadata (agent name, session ID, model, duration) may be logged at `INFO`.
- Traces to: G-1
