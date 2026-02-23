# agentapi

**Agent API Layer** - Sits between thegent and cliproxy+bifrost for intelligent request routing and governance.

## Problem

Multiple AI coding agents (claude, cursor, codex, droid) need unified routing, session-aware load balancing, and agent-specific governance policies.

## Solution

agentapi provides an intermediary layer that:
- Routes requests to appropriate LLM providers
- Applies custom routing rules per agent
- Manages session-aware load balancing
- Enforces agent-specific governance

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌────────────────┐
│   thegent   │────▶│   agentapi    │────▶│ cliproxy/bifrost│
│             │     │  (this repo)  │     │                │
└─────────────┘     └──────────────┘     └────────────────┘
                          │
                          ▼
                   ┌──────────────┐
                   │   Rules      │
                   │   Engine     │
                   └──────────────┘
```

## Quick Start

```bash
# Build
go build -o agentapi ./cmd/agentapi

# Run
./agentapi --port 8318 --cliproxy http://127.0.0.1:8317
```

## Configuration

```yaml
port: 8318

cliproxy:
  url: http://localhost:8317

routing:
  default_model: claude-3-5-sonnet-20241022
  fallback_models:
    - gpt-4o
    - gemini-1.5-pro

agents:
  claude:
    preferred_model: claude-3-5-sonnet-20241022
    max_retries: 3
    timeout_seconds: 30
  cursor:
    preferred_model: gpt-4o
    max_retries: 2
    timeout_seconds: 60
```

## Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |
| `/v1/chat/completions` | POST | Chat with agent routing |
| `/admin/rules` | GET | List routing rules |
| `/admin/rules` | POST | Set routing rule |
| `/admin/sessions` | GET | List active sessions |

## Routing Rules

```json
{
  "agent": "claude",
  "preferred_model": "claude-3-5-sonnet-20241022",
  "fallback_models": ["gpt-4o", "gemini-1.5-pro"],
  "max_retries": 3,
  "timeout_seconds": 30,
  "rate_limit": {
    "requests_per_minute": 60,
    "tokens_per_minute": 100000
  }
}
```

## Agent Support

| Agent | Preferred Model | Fallback |
|-------|----------------|----------|
| claude | claude-3-5-sonnet | gpt-4o |
| cursor | gpt-4o | claude-3-5-sonnet |
| codex | gpt-4o | claude-3-opus |
| droid | claude-3-haiku | gpt-4o-mini |

## Governance

agentapi integrates with the Kush governance system:

- **Session tracking** - Monitor active agent sessions
- **Rate limiting** - Per-agent quotas
- **Cost routing** - Route to cheaper providers when available
- **Fallback logic** - Automatic failover on errors

## Integration

### With thegent

```python
from thegent import Agent

agent = Agent(
    provider="agentapi",
    base_url="http://localhost:8318"
)
```

### With cliproxy

```
agentapi --cliproxy http://cliproxy:8317
```

## Metrics

Prometheus metrics available at `/metrics`:

- `agentapi_requests_total` - Total requests by agent
- `agentapi_latency_seconds` - Request latency
- `agentapi_errors_total` - Error count by type

## Development

```bash
# Test
go test ./...

# Lint
go fmt ./...
go vet ./...
```

## License

MIT License - see LICENSE file
