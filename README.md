# AgentAPI++ (KooshaPari Fork)

This repository works with Claude and other AI agents as autonomous software engineers.

## Quick Start

```bash
# Install binary
OS=$(uname -s | tr "[:upper:]" "[:lower:]")
ARCH=$(uname -m | sed "s/x86_64/amd64/;s/aarch64/arm64/")
curl -fsSL "https://github.com/KooshaPari/agentapi/releases/latest/download/agentapi-${OS}-${ARCH}" -o agentapi
chmod +x agentapi

# Or build from source
go build -o agentapi main.go

# Run with Claude Code
./agentapi server -- claude

# Run with Cursor
./agentapi server -- cursor
```

## External CLI Agent Control

Control Claude Code, Cursor, Aider, Codex and other agents via HTTP API:

```bash
# Send message
curl -X POST http://localhost:3284/api/v0/chat \
  -H "Content-Type: application/json" \
  -d '{"messages": [{"role": "user", "content": "Hello"}], "agent": "claude"}'

# List agents
curl http://localhost:3284/api/v0/agents

# Get session
curl http://localhost:3284/api/v0/sessions/{id}
```

### Agent Selection

```bash
# Claude Code (default)
./agentapi server -- claude

# Cursor
./agentapi server -- cursor

# Aider
./agentapi server -- aider

# Codex (requires --type flag)
./agentapi server --type=codex -- codex
```

## Supported Agents

| Agent | Flag | Type | Description |
|-------|------|------|-------------|
| Claude Code | `claude` | auto | Anthropic's CLI |
| Cursor | `cursor` | auto | Cursor IDE agent |
| Aider | `aider` | auto | AI pair programming |
| Codex | `codex` | explicit | OpenAI's coding agent |
| Goose | `goose` | auto | Independent agent |
| Gemini CLI | `gemini` | explicit | Google's CLI |
| GitHub Copilot | `github-copilot` | explicit | GitHub's CLI |
| Amazon Q | `amazon-q` | explicit | AWS developer agent |
| Sourcegraph Amp | `amp` | explicit | Sourcegraph's agent |
| Auggie | `auggie` | explicit | Augment Code's agent |

## Documentation

- `docs/api/` - API endpoint reference
- `docs/tutorials/` - Step-by-step guides
- `docs/how-to/` - Common tasks
- `docs/explanation/` - Architecture deep dives

## Environment

```bash
export AGENTAPI_PORT=3284
export AGENTAPI_MODEL=claude-3-5-sonnet-20241022
export AGENTAPI_TIMEOUT=300
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
| Testing | testify | manual assertions |

---

## Code Quality Non-Negotiables

- Zero new lint suppressions without inline justification
- All new code must pass: go fmt, go vet, golint
- Max function: 40 lines
- No placeholder TODOs in committed code

### Go-Specific Rules

- Use `go fmt` for formatting
- Use `go vet` for linting
- Use `golangci-lint` for comprehensive linting
- All public APIs must have godoc comments

---

## Verifiable Constraints

| Metric | Threshold | Enforcement |
|--------|-----------|-------------|
| Tests | 80% coverage | CI gate |
| Lint | 0 errors | golangci-lint |
| Security | 0 critical | trivy scan |

---

## Domain-Specific Patterns

### What AgentAPI++ Is

AgentAPI++ is an **HTTP API gateway** for controlling CLI-based AI coding agents. The core domain is: provide a unified HTTP interface to spawn, control, and interact with any CLI agent through terminal emulation.

### Key Interfaces

| Interface | Responsibility | Location |
|-----------|---------------|----------|
| **HTTP Server** | REST API for agent control | `lib/httpapi/` |
| **Terminal Emulator** | PTY management | `lib/termexec/` |
| **Output Parser** | Agent message extraction | `lib/screentracker/` |
| **Message Formatter** | Agent-specific formatting | `lib/msgfmt/` |

### Message Flow

```
1. HTTP Request → API Handler
2. API Handler → Terminal Input
3. Terminal Emulator → Agent Process
4. Output Parser ← Agent Output
5. SSE/Response ← Formatted Message
```

### Common Anti-Patterns to Avoid

- **Blocking on agent output** -- Use streaming/SSE instead
- **Hardcoded timeouts** -- Use configurable timeouts with env vars
- **Missing agent type handling** -- Each agent has different output formats
- **No session state** -- Agents maintain stateful conversations

---

## Kush Ecosystem

This project is part of the Kush multi-repo system:

```
kush/
├── thegent/         # Agent orchestration
├── agentapi++/      # HTTP API for coding agents (this repo)
├── cliproxy++/      # LLM proxy with multi-provider support
├── tokenledger/     # Token and cost tracking
├── 4sgm/           # Python tooling workspace
├── civ/             # Deterministic simulation
├── parpour/         # Spec-first planning
└── pheno-sdk/       # Python SDK
```

---

## License

MIT License - see LICENSE file
