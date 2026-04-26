# AgentAPI++ (KooshaPari Fork)

Multi-model AI routing gateway extending Anthropic's agentapi — Phenotype org fork (74 commits ahead, 7 fixes staged for upstream).

**Status:** active

Control [Claude Code](https://github.com/anthropics/claude-code), [AmazonQ](https://aws.amazon.com/developer/learning/q-developer-cli/), [Opencode](https://opencode.ai/), [Goose](https://github.com/block/goose), [Aider](https://github.com/Aider-AI/aider), [Gemini](https://github.com/google-gemini/gemini-cli), [GitHub Copilot](https://github.com/github/copilot-cli), [Sourcegraph Amp](https://ampcode.com/), [Codex](https://github.com/openai/codex), [Auggie](https://docs.augmentcode.com/cli/overview), and [Cursor CLI](https://cursor.com/en/cli) with an HTTP API.

---

## What is AgentAPI++?

AgentAPI++ provides an HTTP API to programmatically control CLI-based AI coding agents:

```
HTTP Request → AgentAPI++ → Terminal Emulator → Claude Code, Cursor, Aider, etc.
```

### Key Capabilities

| Capability | Description |
|------------|-------------|
| **Multi-Agent Control** | Claude Code, Cursor, Aider, Codex, Goose, Gemini, Copilot, and more |
| **HTTP Interface** | REST API for any language/platform |
| **Terminal Emulation** | In-memory PTY for agent interaction |
| **Message Parsing** | Structured responses from agent output |
| **Session Management** | Persistent conversations with state |

---

## Quick Start

### Install Binary

```bash
OS=$(uname -s | tr "[:upper:]" "[:lower:]")
ARCH=$(uname -m | sed "s/x86_64/amd64/;s/aarch64/arm64/")
curl -fsSL "https://github.com/KooshaPari/agentapi/releases/latest/download/agentapi-${OS}-${ARCH}" -o agentapi
chmod +x agentapi
```

> [!NOTE]
> Always specify the agent type explicitly using the `--type` flag (e.g: `agentapi server --type claude -- claude`), otherwise message formatting may break.

An OpenAPI schema is available in [openapi.json](openapi.json).

By default, the server runs on port 3284. The server exposes the OpenAPI schema at http://localhost:3284/openapi.json.

### Available Endpoints

The server provides the following REST endpoints:

- GET `/messages` - returns a list of all messages in the conversation with the agent
- POST `/message` - sends a message to the agent. When a 200 response is returned, AgentAPI has detected that the agent started processing the message
- GET `/status` - returns the current status of the agent, either "stable" or "running"
- GET `/events` - an SSE stream of events from the agent: message and status updates
- GET `/health` - health check endpoint
- GET `/version` - server version information
- GET `/info` - server and agent information

#### Allowed hosts

By default, the server only allows requests with the host header set to `localhost`. If you'd like to host AgentAPI elsewhere, you can change this by using the `AGENTAPI_ALLOWED_HOSTS` environment variable or the `--allowed-hosts` flag. Hosts must be hostnames only (no ports); the server ignores the port portion of incoming requests when authorizing.

To allow requests from any host, use `*` as the allowed host.

```bash
go build -o agentapi main.go
```

### Run

```bash
# Start with Claude Code (REQUIRED: specify --type)
./agentapi server --type claude -- claude

# Start with specific agent (REQUIRED: specify --type)
./agentapi server --type cursor -- cursor
./agentapi server --type aider -- aider

# With specific model
./agentapi server --type claude -- claude --model claude-3-5-sonnet-20241022
```

Server runs on port 3284. The chat interface is available at http://localhost:3284/chat (static HTML served from root, redirects to `/chat`).

---

## Supported Agents

| Agent | Type Flag | Description |
|-------|-----------|-------------|
| Claude Code | `claude` | Anthropic's CLI (default) |
| Cursor | `cursor` | Cursor IDE agent |
| Aider | `aider` | AI pair programming |
| Goose | `goose` | Independent agent |
| Codex | `codex` | OpenAI's coding agent |
| Gemini CLI | `gemini` | Google's CLI |
| GitHub Copilot | `copilot` | GitHub's CLI |
| Sourcegraph Amp | `amp` | Sourcegraph's agent |
| Amazon Q | `q` | AWS's developer agent |
| Auggie | `auggie` | Augment Code's agent |

---

## API Examples

### Send a Message

```bash
curl -X POST http://localhost:3284/message \
  -H "Content-Type: application/json" \
  -d '{"type":"user","content":"Hello, help me with Python"}'
```

### Get Conversation History

```bash
curl http://localhost:3284/messages
```

### Check Agent Status

```bash
curl http://localhost:3284/status
```

### Subscribe to Events (SSE)

```bash
curl http://localhost:3284/events
```

---

## Python Client

```python
import requests

url = "http://localhost:3284/message"
payload = {
    "type": "user",
    "content": "Hello, help me with Python"
}

response = requests.post(url, json=payload)
print(response.json())

# Get history
history = requests.get("http://localhost:3284/messages").json()
print(history)
```

### With Events (SSE)

```python
import requests
import sseclient

url = "http://localhost:3284/events"
response = requests.get(url, stream=True)
client = sseclient.SSEClient(response)

for event in client.events():
    print(f"{event.event}: {event.data}")
```

---

## Architecture

```
┌──────────────┐     ┌─────────────────┐     ┌────────────┐
│   HTTP API   │────▶│    AgentAPI     │────▶│    Agent   │
│  (this repo) │     │ (terminal emu)  │     │ (claude,   │
│              │     │                 │     │  cursor)   │
└──────────────┘     └─────────────────┘     └────────────┘
                            │
                            ▼
                     ┌─────────────────┐
                     │   Message       │
                     │   Formatter     │
                     │ (claude,goose,  │
                     │  aider, etc)    │
                     └─────────────────┘
```

### Components

| Component | Description |
|-----------|-------------|
| `cmd/` | CLI commands (server, attach) |
| `lib/httpapi/` | HTTP server and routes |
| `lib/termexec/` | Terminal process execution |
| `lib/screentracker/` | Output parsing |
| `lib/msgfmt/` | Agent-specific message formatting |
| `chat/` | Next.js web UI |

---

## Configuration

### Environment Variables

```bash
export AGENTAPI_PORT=3284
export AGENTAPI_ALLOWED_HOSTS="localhost,127.0.0.1"
export AGENTAPI_ALLOWED_ORIGINS="http://localhost:3284"
```

### Config File

```yaml
server:
  port: 3284
  timeout: 300

agents:
  claude:
    model: claude-3-5-sonnet-20241022
    tools_enabled: true
  cursor:
    model: gpt-4o
```

---

## Integration

### With thegent (MCP)

```yaml
mcp:
  servers:
    agentapi:
      command: agentapi
      args: ["server", "--type", "claude", "--", "claude"]
```

### With cliproxy++

Route LLM requests through cliproxy++ for cost optimization:

```bash
./agentapi server --type claude -- claude --llm-provider http://localhost:8317
```

---

## Documentation

- [API Reference](docs/api/) - Full API documentation
- [Tutorials](docs/tutorials/) - Step-by-step guides
- [How-to Guides](docs/how-to/) - Common tasks
- [Architecture](docs/explanation/) - Deep dives

---

## Development Philosophy

### Extend, Never Duplicate
- NEVER create a v2 file. Refactor the original.
- NEVER create a new class if an existing one can be made generic.
- NEVER create custom implementations when an OSS library exists.

### Primitives First
- Build generic building blocks before application logic.
- A provider interface + registry is better than N isolated classes.

### Research Before Implementing
- Check pkg.go.dev for existing libraries.
- Search GitHub for 80%+ implementations to fork/adapt.

---

## License

MIT License - see LICENSE file
