# AgentAPI (KooshaPari Fork)

**Forked from [coder/agentapi](https://github.com/coder/agentapi)** - HTTP API for controlling AI coding agents.

## Overview

This fork adds customizations for the Kush ecosystem integration:
- Enhanced agent message formatting
- Custom MCP server integration
- Integration with cliproxy for LLM routing

## Supported Agents

Control these AI coding agents via HTTP API:

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

## Architecture

```
┌──────────────┐     ┌──────────────┐     ┌─────────────┐
│   HTTP API   │────▶│  AgentAPI    │────▶│ Agent CLI   │
│  (this repo) │     │  (terminal  │     │ (claude,    │
│              │     │   emulator)  │     │  cursor)    │
└──────────────┘     └──────────────┘     └─────────────┘
                            │
                            ▼
                     ┌──────────────┐
                     │   Message   │
                     │  Formatter  │
                     │ (custom)    │
                     └──────────────┘
```

## Quick Start

### Install

```bash
# Binary
OS=$(uname -s | tr "[:upper:]" "[:lower:]")
ARCH=$(uname -m | sed "s/x86_64/amd64/;s/aarch64/arm64/")
curl -fsSL "https://github.com/KooshaPari/agentapi/releases/latest/download/agentapi-${OS}-${ARCH}" -o agentapi
chmod +x agentapi

# Or build from source
go build -o out/agentapi main.go
```

### Run

```bash
# Start with Claude Code
./agentapi server -- claude

# Start with specific agent
./agentapi server -- cursor
./agentapi server -- aider
```

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |
| `/chat` | GET | Chat UI |
| `/api/v0` | POST | Send message |
| `/api/v0/agents` | GET | List agents |
| `/api/v0/sessions` | GET | List sessions |

## Configuration

```yaml
# config.yaml
server:
  port: 3284

agents:
  claude:
    preferred_model: claude-3-5-sonnet-20241022
  cursor:
    preferred_model: gpt-4o
```

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

The agentapi routes LLM requests through cliproxy for:
- Cost optimization
- Rate limiting
- Multi-provider fallback

## Development

```bash
# Build
make build

# Test
go test ./...

# Lint
go fmt ./...
go vet ./...
```

## Fork Differences

This fork includes:
- ✅ Custom message formatters for Kush agents
- ✅ MCP server integration
- ✅ cliproxy routing integration
- ✅ Enhanced session management

## License

MIT License - see LICENSE file
