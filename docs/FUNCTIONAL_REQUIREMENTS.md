# Functional Requirements

This document defines the functional requirements currently traced by the test suite.
Tests reference these IDs with `// Traces to: FR-...` comments.

## HTTP API

| FR ID | Requirement |
|-------|-------------|
| FR-HTTP-001 | The service can initialize an HTTP server on the configured port. |
| FR-HTTP-002 | The service supports `POST /v1/chat/completions`. |
| FR-HTTP-003 | Successful OpenAI-compatible requests return a success response. |
| FR-HTTP-004 | Invalid JSON requests return a client error. |
| FR-HTTP-005 | Downstream or retry-exhaustion failures are surfaced as server errors. |
| FR-HTTP-007 | The service exposes a status endpoint. |
| FR-HTTP-008 | The service exposes a server-sent events stream. |
| FR-HTTP-009 | The service exposes a health endpoint. |
| FR-HTTP-011 | The service supports `POST /message`. |

## Routing And Sessions

| FR ID | Requirement |
|-------|-------------|
| FR-ROUTE-001 | AgentBifrost can be initialized with a base URL. |
| FR-ROUTE-002 | AgentBifrost provides default routing rules. |
| FR-ROUTE-003 | AgentBifrost supports custom rule configuration. |
| FR-ROUTE-004 | Agent routing selects a valid target. |
| FR-SESS-001 | Session lookup creates a session when none exists. |

## Telemetry And Harness

| FR ID | Requirement |
|-------|-------------|
| FR-TELE-001 | Token usage can be parsed from agent output. |
| FR-TELE-002 | Cost values can be parsed from agent output. |
| FR-TELE-003 | Agent metrics can be collected and represented. |
| FR-HARN-001 | The harness rejects unknown agents deterministically. |
| FR-HARN-002 | Agent timeout configuration is represented and testable. |

## Security

| FR ID | Requirement |
|-------|-------------|
| FR-SEC-001 | Host authorization permits explicitly allowed hosts. |

## Traceability

Every test covering one of these requirements should include a trace marker:

```go
// Traces to: FR-HTTP-001
func TestExample(t *testing.T) { }
```

See `docs/reference/fr_coverage_matrix.md` for the current FR-to-test mapping.
