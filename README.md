# agentapi

Agent API Layer - Sits between thegent and cliproxy+bifrost for intelligent request routing.

This repository works with Claude and other AI agents as autonomous software engineers.

## Quick Start

```bash
# Build
go build -o agentapi ./cmd/agentapi

# Run
./agentapi --port 8318 --cliproxy http://127.0.0.1:8317
```

## Environment

```bash
# Required environment variables
export CLIPROXY_URL="http://localhost:8317"
export AGENTAPI_PORT="8318"
```

---

## Development Philosophy

### Extend, Never Duplicate

- NEVER create a v2 file. Refactor the original.
- NEVER create a new class if an existing one can be made generic.
- NEVER create custom implementations when an OSS library exists.
- Before writing ANY new code: search the codebase for existing patterns.

### Primitives First

- Build generic building blocks before application logic.
- A provider interface + registry is better than N isolated classes.
- Template strings > hardcoded messages. Config-driven > code-driven.

### Research Before Implementing

- Check pkg.go.dev for existing libraries.
- Search GitHub for 80%+ implementations to fork/adapt.

---

## Library Preferences (DO NOT REINVENT)

| Need | Use | NOT |
|------|-----|-----|
| HTTP router | chi | custom router |
| Logging | zerolog | fmt.Print |
| CLI | cobra | manual flag parsing |
| Config | viper | manual env parsing |

---

## Code Quality Non-Negotiables

- Zero new lint suppressions without inline justification
- All new code must pass: go fmt, go vet, golint
- Max function: 40 lines
- No placeholder TODOs in committed code

---

## Verifiable Constraints

| Metric | Threshold | Enforcement |
|--------|-----------|-------------|
| Tests | 80% coverage | CI gate |
| Lint | 0 errors | golangci-lint |

---

## Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |
| `/v1/chat/completions` | POST | Chat with agent routing |
| `/admin/rules` | GET | List routing rules |
| `/admin/rules` | POST | Set routing rule |
| `/admin/sessions` | GET | List active sessions |

---

## Agent Support

| Agent | Preferred Model | Fallback |
|-------|----------------|----------|
| claude | claude-3-5-sonnet | gpt-4o |
| cursor | gpt-4o | claude-3-5-sonnet |
| codex | gpt-4o | claude-3-opus |
| droid | claude-3-haiku | gpt-4o-mini |

---

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

---

## License

MIT License - see LICENSE file
