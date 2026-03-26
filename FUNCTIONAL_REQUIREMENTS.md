# Functional Requirements - AgentAPI++

**Project**: AgentAPI++ (HTTP Gateway for AI Coding Agents)  
**Document Version**: 1.0  
**Last Updated**: 2026-03-25

---

## Chat & Message Handling

### FR-CHAT-001: Send Message to Agent
**Description**: Client sends message to agent via HTTP and receives response  
**Acceptance Criteria**:
- Endpoint: `POST /api/v0/chat`
- Request includes: `messages` (array of {role, content}), `agent` (type), optional `model`, `temperature`
- System spawns agent process if not already running
- Sends message to agent stdin via PTY
- Captures agent stdout and parses response
- Response includes: `role: "assistant"`, `content: <agent_response>`, `stop_reason`
- HTTP 200 response when agent finishes processing

### FR-CHAT-002: Stream Agent Response
**Description**: Client streams agent response in real-time via SSE  
**Acceptance Criteria**:
- Endpoint: `POST /api/v0/chat/stream`
- Request format same as FR-CHAT-001
- Server opens SSE connection and streams events as agent outputs
- Event types: `message-start`, `message-content` (delta), `message-stop`, `error`
- Client can close connection at any time (graceful disconnect)
- Server resumes from last message if client reconnects

### FR-CHAT-003: Multi-Message Conversations
**Description**: Support stateful conversations with multiple messages  
**Acceptance Criteria**:
- Each session maintains conversation history
- New message appended to history (user + assistant messages)
- Agent context includes previous messages for continuity
- Conversation state preserved in memory
- Sessions auto-expire after 1 hour of inactivity

### FR-CHAT-004: Agent-Specific Message Formatting
**Description**: Format user input according to agent CLI requirements  
**Acceptance Criteria**:
- ClaudeFormatter: wraps input in Claude CLI format
- CursorFormatter: formats for Cursor agent expectations
- AiderFormatter: formats for Aider-specific input
- Each formatter handles: prompt injection, model selection, tool enablement
- Formatting transparent to client (all agents accept same message format)
- Formatters testable with unit tests

### FR-CHAT-005: Agent-Specific Response Parsing
**Description**: Parse agent output and extract structured responses  
**Acceptance Criteria**:
- Each agent outputs different format (JSON vs text vs mixed)
- Parser detects agent type and applies correct parser
- Extracts: message content, tool calls (if any), stop reason
- Handles incomplete output gracefully (partial messages)
- Error detection: agent errors parsed and returned as error response
- Parsing tested with real agent output samples

---

## Session Management

### FR-SESSION-001: Create Session
**Description**: Establish new conversation session  
**Acceptance Criteria**:
- Implicit creation on first message (no explicit session creation needed)
- Session ID generated (UUID format)
- Session includes: agent type, messages array (starts empty), created timestamp
- Session status: "active"
- Client receives session ID in response

### FR-SESSION-002: List Active Sessions
**Description**: Retrieve list of all active sessions  
**Acceptance Criteria**:
- Endpoint: `GET /api/v0/sessions`
- Returns array of session objects with: ID, agent type, created timestamp, last message timestamp, message count
- Pagination support (default 50, max 100)
- Filter by agent type optional
- Response time <500ms for 1000+ sessions

### FR-SESSION-003: Get Session Details
**Description**: Retrieve full conversation history for session  
**Acceptance Criteria**:
- Endpoint: `GET /api/v0/sessions/{id}`
- Returns: session metadata + complete message history
- Messages include: timestamp, role, content, tokens (estimated)
- Useful for auditing and debugging
- Response time <200ms

### FR-SESSION-004: Close Session
**Description**: Terminate session and cleanup resources  
**Acceptance Criteria**:
- Endpoint: `DELETE /api/v0/sessions/{id}`
- Terminates agent process
- Cleans up memory (releases PTY, kills subprocess)
- Session marked as "completed"
- Subsequent messages to closed session return 404
- Graceful: waits for in-flight messages before killing process

### FR-SESSION-005: Session Persistence
**Description**: Optionally persist sessions to storage for recovery  
**Acceptance Criteria**:
- Configuration option to enable/disable persistence
- Persisted to: memory (default) or database
- On server restart, reload persisted sessions
- Conversation history fully recovered
- Useful for long-lived agent orchestrations

---

## Agent Control

### FR-AGENT-001: Agent Detection & Availability
**Description**: Detect if agent is installed and available  
**Acceptance Criteria**:
- System checks if agent CLI exists in PATH on startup
- Endpoint: `GET /api/v0/agents` lists all supported agents
- Returns: agent type, status (available/unavailable), version, supported models
- Graceful handling if agent not installed (error message clear)
- Optional pre-flight checks when starting agent

### FR-AGENT-002: Agent Process Spawning
**Description**: Spawn agent CLI process with PTY  
**Acceptance Criteria**:
- Use `termexec.Process` to start agent in pseudo-terminal
- PTY supports: interactive input, ANSI escape codes, signals
- Process memory limit: configurable (default 1GB)
- Timeout: kill process if hung >300s (configurable)
- Signal handling: SIGTERM (graceful), SIGKILL (forced)

### FR-AGENT-003: Agent Output Capture
**Description**: Capture agent stdout/stderr in real-time  
**Acceptance Criteria**:
- Screen tracker captures terminal output at 40fps (25ms interval)
- Supports: ANSI colors, formatting, special characters
- Output buffered in memory (default 10MB)
- Snapshot contains: timestamp, buffer content
- Useful for debugging and output analysis

### FR-AGENT-004: Tool Execution Handling
**Description**: Detect and handle tool calls made by agents  
**Acceptance Criteria**:
- Agent may call tools (e.g., "run_shell_command")
- Tool calls detected in agent output (JSON or text format)
- Tool name, input parameters extracted
- Tool execution delegated to client or executor service
- Result returned to agent for continuation
- Support for: shell commands, file operations, API calls

### FR-AGENT-005: Error Recovery
**Description**: Handle agent crashes and errors gracefully  
**Acceptance Criteria**:
- If agent crashes, return error response with crash details
- Cleanup: kill process, release resources
- Session marked as "error" state
- Error message includes: crash reason, stdout/stderr capture, timestamp
- Client can retry with same or different agent

---

## API Endpoints & Protocols

### FR-API-001: POST /api/v0/chat
**Description**: Send message and get response (blocking)  
**Acceptance Criteria**:
- Request schema: { "messages": [{"role", "content"}], "agent": "claude" | ..., optional "model", "temperature" }
- Response schema: { "role": "assistant", "content": string, "stop_reason": "end_turn" | "tool_use" | "error" }
- Timeout: 300s (configurable per agent)
- Error responses include: error_code, message, details
- Idempotent: same input always returns same output (session independent)

### FR-API-002: POST /api/v0/chat/stream
**Description**: Send message and stream response via SSE  
**Acceptance Criteria**:
- Request schema same as FR-API-001
- Response: Server-Sent Events stream
- Events: `event: message-start`, `event: message-content`, `event: message-stop`, `event: error`
- Each event has: `data: <JSON>` with payload
- Client can close connection at any time
- Server handles disconnect gracefully

### FR-API-003: GET /api/v0/agents
**Description**: List all supported agents  
**Acceptance Criteria**:
- Response: array of { type, status, version, models: [], capabilities: [] }
- Example: { type: "claude", status: "available", version: "1.0.0", models: ["claude-3-5-sonnet"], capabilities: ["tools", "streaming"] }
- Update frequency: static at server startup
- Response time <100ms

### FR-API-004: GET /health
**Description**: Health check for server  
**Acceptance Criteria**:
- Response: { "status": "ok", "uptime": <seconds>, "version": <version> }
- HTTP 200 when healthy
- HTTP 503 when unhealthy (graceful shutdown in progress)
- Used by load balancers

### FR-API-005: GET /metrics
**Description**: Prometheus metrics endpoint  
**Acceptance Criteria**:
- Response: Prometheus format text
- Metrics:
  - `agentapi_requests_total{agent,method}` - request count
  - `agentapi_request_duration_seconds{agent}` - latency histogram
  - `agentapi_errors_total{agent,error_type}` - error count
  - `agentapi_active_sessions` - current session count
- Update frequency: real-time
- Used by Prometheus scraper

### FR-API-006: GET /openapi.json
**Description**: OpenAPI 3.0 specification  
**Acceptance Criteria**:
- Complete API specification in OpenAPI 3.0 format
- Includes: endpoints, request/response schemas, security, examples
- Generated from code or hand-maintained
- Used by: Swagger UI, API clients, documentation generators

---

## Host Authorization & Security

### FR-SEC-001: Host Validation
**Description**: Restrict API access to authorized hosts  
**Acceptance Criteria**:
- By default, allow only localhost (127.0.0.1, ::1)
- Environment variable `AGENTAPI_ALLOWED_HOSTS` configures allowed hosts
- Format: comma-separated hostnames or `*` for any
- Hostname extracted from Host header
- Reject requests from unauthorized hosts with HTTP 403

### FR-SEC-002: Input Validation
**Description**: Validate all HTTP inputs  
**Acceptance Criteria**:
- Validate message length (max 100KB)
- Validate agent type from whitelist
- Validate session ID format (UUID)
- Reject invalid inputs with HTTP 400
- Error messages don't leak system details

### FR-SEC-003: Secrets Handling
**Description**: Protect API keys and credentials  
**Acceptance Criteria**:
- API keys for agents passed via environment variables
- Never log keys or credentials
- Keys not transmitted in request bodies
- Support OAuth2 for agent authentication
- TLS support for encrypted communication

---

## Terminal Emulation & Output

### FR-TERM-001: Pseudo-Terminal (PTY) Support
**Description**: Run agent in interactive terminal environment  
**Acceptance Criteria**:
- Use Go PTY library (github.com/creack/pty)
- Allocate pseudo-terminal for agent process
- Support: interactive input, terminal signals, ANSI colors
- Terminal size: 80x24 (or configurable)
- Useful for agents needing interactive features

### FR-TERM-002: Output Buffering
**Description**: Buffer agent output for client consumption  
**Acceptance Criteria**:
- Buffer terminal output in memory
- Snapshot interval: 25ms (40fps)
- Buffer size: configurable (default 10MB)
- Snapshots include: timestamp, content, truncation flag
- Useful for: streaming, logging, debugging

### FR-TERM-003: ANSI Escape Code Handling
**Description**: Support terminal colors and formatting  
**Acceptance Criteria**:
- Preserve ANSI escape codes in captured output
- Support: colors (16, 256, 24-bit), bold, italic, underline
- Client can strip ANSI codes if needed (optional)
- Useful for: preserving terminal appearance, debugging

---

## Integration & Extensibility

### FR-INTEG-001: cliproxy++ Integration
**Description**: Route LLM requests through cliproxy++ for cost optimization  
**Acceptance Criteria**:
- Agents can be started with `--llm-provider http://localhost:8317`
- Requests routed through cliproxy++ instead of direct API
- Reduces costs by routing to cheaper models
- Transparent to client

### FR-INTEG-002: MCP Integration
**Description**: Support Model Context Protocol for agentic systems  
**Acceptance Criteria**:
- AgentAPI can act as MCP server
- Register tool handlers for agents
- Receive context updates from parent MCP client
- Enable orchestration of agent chains

### FR-INTEG-003: Custom Message Handlers
**Description**: Support custom formatters for new agents  
**Acceptance Criteria**:
- Plugin system for new agent formatters
- Implement interface: Input formatter + Output parser
- Register formatter with type name
- New agents can be added without rebuilding server

---

## Observability & Debugging

### FR-OBS-001: Structured Logging
**Description**: Log all requests and errors in structured format  
**Acceptance Criteria**:
- Log format: JSON with: timestamp, request_id, agent, method, status, duration
- Log level: INFO (requests), ERROR (failures), DEBUG (output traces)
- Logs useful for debugging and monitoring
- Secrets (API keys) never logged

### FR-OBS-002: Request Tracing
**Description**: Support distributed tracing via OpenTelemetry  
**Acceptance Criteria**:
- Each request assigned unique trace ID
- Trace spans: HTTP handler, agent process, output parsing
- Exportable to Jaeger or other OTEL collectors
- Useful for debugging latency issues

### FR-OBS-003: Debug Endpoint
**Description**: Expose debugging information for developers  
**Acceptance Criteria**:
- Endpoint: `GET /debug/sessions/{id}` (admin only)
- Returns: session state, message history, process info
- Endpoint: `GET /debug/agents` - detailed agent info
- Useful for development and troubleshooting

---

## Performance & Reliability

### FR-PERF-001: Startup Time
**Description**: Agent startup time should be acceptable  
**Acceptance Criteria**:
- Claude startup: <5s
- Cursor startup: <5s
- Aider startup: <5s
- Other agents: <10s
- Timeouts configured per agent

### FR-PERF-002: Message Latency
**Description**: Message processing latency should be low  
**Acceptance Criteria**:
- Simple messages: <200ms
- Complex messages with tools: <2s
- Streaming latency: <100ms per event
- Measured end-to-end (HTTP request to response)

### FR-REL-001: Graceful Shutdown
**Description**: Server shutdown should clean up resources properly  
**Acceptance Criteria**:
- Receive SIGTERM, wait for in-flight requests (10s timeout)
- Close all active sessions gracefully
- Kill agent processes
- Close database/cache connections
- Exit cleanly

### FR-REL-002: Session Recovery
**Description**: Recover sessions after unexpected shutdown  
**Acceptance Criteria**:
- Sessions persisted to disk (optional)
- On restart, reload persisted sessions
- Resume conversations from last message
- Agent processes restarted as needed

---

## Success Criteria

### Integration
- [ ] All 10 agents successfully integrated
- [ ] Each agent integration tested with real agent
- [ ] Message formatting roundtrip verified
- [ ] Output parsing handles all response types

### Reliability
- [ ] Agent crashes handled gracefully
- [ ] No message loss (persisted sessions)
- [ ] Server uptime >99.9%
- [ ] Error recovery tested

### Performance
- [ ] Startup time <5s for all agents
- [ ] Message latency <200ms (simple)
- [ ] Streaming latency <100ms per event
- [ ] Support 100+ concurrent sessions

### Developer Experience
- [ ] API fully documented (OpenAPI)
- [ ] Examples for all 10 agents
- [ ] Error messages clear and actionable
- [ ] Debugging support (logs, traces, endpoints)

---

**Total Functional Requirements**: 21  
**Implementation Status**: IN PROGRESS  
**Last Updated**: 2026-03-25
