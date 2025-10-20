# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

AgentAPI is an HTTP API wrapper that controls coding agents (Claude Code, Goose, Aider, Codex) via terminal emulation. It runs agents in an in-memory PTY, translates API calls to terminal keystrokes, and parses terminal output into structured messages.

**Tech Stack:**
- Backend: Go 1.23.2 (Cobra CLI, Chi router, Huma API framework)
- Frontend: Next.js 15 + React 19 + Tailwind CSS
- Build: Make + Bun (frontend is embedded in Go binary)

## Development Commands

### Building

```bash
# Build everything (frontend + backend)
make build

# Build frontend only
make embed

# Build with custom output path
make build BINPATH=./agentapi
```

The build process:
1. Builds Next.js frontend with Bun (`cd chat && bun run build`)
2. Copies frontend output to `lib/httpapi/chat/`
3. Embeds frontend in Go binary via go:embed
4. Compiles Go binary to `out/agentapi`

### Testing

```bash
# Run all Go tests
go test ./...

# Run tests with verbose output (as CI does)
go test -count=1 -v ./...

# Run specific package tests
go test ./lib/screentracker/...

# Frontend linting
cd chat && bun run lint
```

### Running Locally

```bash
# Run the server with Claude Code
./out/agentapi server -- claude

# Run with custom port
./out/agentapi server --port 8080 -- claude

# Run with Aider or Goose
./out/agentapi server -- aider --model sonnet
./out/agentapi server -- goose

# Attach to running session
./out/agentapi attach --url localhost:3284
```

## Architecture

### Core Components

**lib/termexec**: Terminal process execution
- Manages PTY (pseudo-terminal) using xpty library
- Starts agent processes (claude, goose, aider) in virtual terminal
- Handles terminal I/O and screen reading
- Sets TERM=vt100 for compatibility

**lib/screentracker**: Conversation tracking
- Monitors terminal screen changes to detect message boundaries
- Takes snapshots when terminal updates
- Determines when agent is "stable" (not typing) vs "running"
- Splits terminal output into user/agent message pairs
- Key insight: Diffs terminal snapshots to extract new agent responses

**lib/msgfmt**: Message formatting
- Strips TUI elements from agent messages (input boxes, user echoes)
- Agent-specific formatting logic (Claude Code vs Goose vs Aider)
- Handles whitespace normalization and user input detection
- Uses rune-based matching to find/remove user input echoes

**lib/httpapi**: HTTP server
- Chi router with Huma API framework for type-safe endpoints
- SSE (Server-Sent Events) for streaming message/status updates
- Embeds Next.js frontend via go:embed in `embed.go`
- OpenAPI schema auto-generated and served at `/openapi.json`

**cmd/server**: Server command
- Parses CLI args and agent command (everything after `--`)
- Configures terminal dimensions, timeouts, message formatting
- Starts HTTP server on port 3284 (default)

**cmd/attach**: Attach command
- Interactive terminal client to attach to running agent session
- Uses Bubble Tea TUI framework

### Frontend (chat/)

Next.js app with:
- `/chat`: Main chat interface
- `/chat/embed`: Embeddable version (used in demos)
- `/`: Redirects to `/chat/embed`
- SSE client for real-time message streaming
- Dark mode support via next-themes

### Message Flow

1. User sends POST to `/message`
2. `screentracker.Conversation.SendMessage()` writes to PTY
3. Terminal output changes → `screentracker` takes snapshots
4. Snapshots diffed to detect new agent message
5. `msgfmt` cleans TUI elements from message
6. Message emitted via SSE to `/events` subscribers
7. Frontend updates chat UI

### API Endpoints

- `GET /messages` - Full conversation history
- `POST /message` - Send message to agent
- `GET /status` - Agent status ("stable" or "running")
- `GET /events` - SSE stream of messages and status updates
- `GET /openapi.json` - OpenAPI schema
- `GET /docs` - API documentation UI

## Important Patterns

**Testing**: Use table-driven tests with `testify/assert`. See `lib/screentracker/conversation_test.go` for examples.

**Agent-specific logic**: When adding support for new agents, update `lib/msgfmt/msgfmt.go` to handle their TUI elements.

**Terminal stability**: The "stable" vs "running" status is determined by checking if the screen hasn't changed for `ScreenStabilityLength` (default 500ms). This is how we know the agent finished responding.

**Frontend embedding**: The frontend is built with `BASE_PATH=/magic-base-path-placeholder` which gets replaced at runtime. This allows serving from different paths (e.g., GitHub Pages at `/chat`).

## Common Development Scenarios

**Adding a new API endpoint**: Add route in `lib/httpapi/setup.go` and handler in `lib/httpapi/server.go`. Huma auto-generates OpenAPI schema.

**Modifying message parsing**: Edit `lib/msgfmt/msgfmt.go`. The message formatter strips user input echoes and TUI elements. Test changes against all supported agents.

**Debugging terminal output**: Use `agentapi attach` to see raw terminal state, or add logging in `lib/screentracker/conversation.go` to inspect snapshots.

**Frontend changes**: Work in `chat/` directory. Run `cd chat && bun run dev` for hot reload. Remember to `make embed` before testing with Go server.
