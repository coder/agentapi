# AGENTS.md

This file provides guidance to AI agents working with code in this repository.

## Build Commands

- `make build` - Build the binary to `out/agentapi` (includes chat UI build)
- `make embed` - Build the chat UI and embed it into Go
- `go build -o out/agentapi main.go` - Direct Go build without chat UI
- `go generate ./...` - Generate OpenAPI schema and version info

## Testing

- `go test ./...` - Run all Go tests
- Tests are located alongside source files (e.g., `lib/httpapi/server_test.go`)

## Development Commands

- `agentapi server -- claude` - Start server with Claude Code agent
- `agentapi server -- aider --model sonnet` - Start server with Aider agent
- `agentapi server -- goose` - Start server with Goose agent
- `agentapi server --type=codex -- codex` - Start server with Codex (requires explicit type)
- `agentapi server --type=gemini -- gemini` - Start server with Gemini (requires explicit type)
- `agentapi attach --url localhost:3284` - Attach to running agent terminal
- Server runs on port 3284 by default
- Chat UI available at http://localhost:3284/chat
- API documentation at http://localhost:3284/docs

## Architecture

This is a Go HTTP API server that controls coding agents (Claude Code, Aider, Goose, etc.) through terminal emulation.

**Core Components:**
- `main.go` - Entry point using cobra CLI framework
- `cmd/` - CLI command definitions (server, attach)
- `lib/httpapi/` - HTTP server, routes, and OpenAPI schema
- `lib/screentracker/` - Terminal output parsing and message splitting
- `lib/termexec/` - Terminal process execution and management
- `lib/msgfmt/` - Message formatting for different agent types (claude, goose, aider, codex, gemini, amp, cursor-agent, cursor, auggie, custom)
- `chat/` - Next.js web UI (embedded into Go binary)

**Key Architecture:**
- Runs agents in an in-memory terminal emulator
- Translates API calls to terminal keystrokes
- Parses terminal output into structured messages
- Supports multiple agent types with different message formats
- Embeds Next.js chat UI as static assets in Go binary

**Message Flow:**
1. User sends message via HTTP API
2. Server takes terminal snapshot
3. Message sent to agent as terminal input
4. Terminal output changes tracked and parsed
5. New content becomes agent response message
6. SSE events stream updates to clients

## API Endpoints

- GET `/messages` - Get all messages in conversation
- POST `/message` - Send message to agent (content, type fields)
- GET `/status` - Get agent status ("stable" or "running")
- GET `/events` - SSE stream of agent events
- GET `/openapi.json` - OpenAPI schema
- GET `/docs` - API documentation UI
- GET `/chat` - Web chat interface

## Supported Agents

Agents with explicit type requirement (use `--type=<agent>`):
- `codex` - OpenAI Codex
- `gemini` - Google Gemini CLI
- `amp` - Sourcegraph Amp CLI
- `cursor` - Cursor CLI

Agents with auto-detection:
- `claude` - Claude Code (default)
- `goose` - Goose
- `aider` - Aider

## Project Structure

- Go module with standard layout
- Chat UI in `chat/` directory (Next.js + TypeScript)
- OpenAPI schema auto-generated to `openapi.json`
- Version managed via `version.sh` script
