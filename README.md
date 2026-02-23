# AgentAPI (KooshaPari Fork)

HTTP API for controlling AI coding agents (Claude Code, Cursor, Aider, etc.)

This repository works with Claude and other AI agents as autonomous software engineers.

## Quick Start

```bash
# Install binary
OS=$(uname -s | tr "[:upper:]" "[:lower:]")
ARCH=$(uname -m | sed "s/x86_64/amd64/;s/aarch64/arm64/")
curl -fsSL "https://github.com/KooshaPari/agentapi/releases/latest/download/agentapi-${OS}-${ARCH}" -o agentapi
chmod +x agentapi

# Or build from source
go build -o out/agentapi main.go

# Run with agent
./agentapi server -- claude
```

## Environment

```bash
# Optional environment variables
export AGENTAPI_PORT=3284
export AGENTAPI_MODEL=claude-3-5-sonnet-20241022
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
| CLI | cobra | manual flag parsing |
| Logging | zerolog | fmt.Print |
| Terminal emulation | tty | raw os/exec |

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

## Supported Agents

| Agent | Type | Status |
|-------|------|--------|
| Claude Code | claude | ✅ |
| Amazon Q | amazon-q | ✅ |
| Opencode | opencode | ✅ |
| Goose | goose | ✅ |
| Aider | aider | ✅ |
| Gemini CLI | gemini | ✅ |
| GitHub Copilot | github-copilot | ✅ |
| Sourcegraph Amp | amp | ✅ |
| Codex | codex | ✅ |
| Auggie | auggie | ✅ |
| Cursor | cursor | ✅ |

---

## Integration

### With thegent

```python
# thegent config
mcp:
  servers:
    agentapi:
      command: agentapi
      args: ["server", "--", "claude"]
```

### With cliproxy

The agentapi routes LLM requests through cliproxy for cost optimization and rate limiting.

---

## Fork Differences

This fork includes:
- ✅ Custom message formatters for Kush agents
- ✅ MCP server integration
- ✅ cliproxy routing integration
- ✅ Enhanced session management

---

## License

MIT License - see LICENSE file
