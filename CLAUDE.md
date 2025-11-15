# CLAUDE.md - AgentAPI Documentation for AI Assistants

This document provides comprehensive guidance for AI assistants working on the AgentAPI codebase.

## Table of Contents

1. [Project Overview](#project-overview)
2. [Codebase Structure](#codebase-structure)
3. [Architecture & Key Components](#architecture--key-components)
4. [Development Setup](#development-setup)
5. [Code Conventions & Patterns](#code-conventions--patterns)
6. [Testing Strategy](#testing-strategy)
7. [Making Changes](#making-changes)
8. [Common Tasks](#common-tasks)
9. [CI/CD & Release Process](#cicd--release-process)
10. [Troubleshooting](#troubleshooting)

---

## Project Overview

**AgentAPI** is a unified HTTP API that controls multiple AI coding agents through terminal emulation. It provides a standardized interface to interact with Claude Code, Goose, Aider, Gemini, Sourcegraph Amp, Codex, and Cursor CLI.

### Purpose
- Enables programmatic control of coding agents that typically run in terminal UIs
- Provides a REST API with OpenAPI schema
- Offers real-time updates via Server-Sent Events (SSE)
- Includes a web-based chat interface

### Tech Stack
- **Backend**: Go 1.23.2
- **Frontend**: Next.js 15.2.4, React 19, Tailwind CSS 4
- **Build**: Make, Bun (for frontend)
- **Key Libraries**:
  - `termtest/xpty` - PTY (pseudo-terminal) emulation
  - `huma/v2` - REST API framework with OpenAPI
  - `chi/v5` - HTTP router
  - `bubbletea` - Terminal UI for attach command

### Current Version
v0.5.0 - Latest release adds Cursor CLI support

---

## Codebase Structure

```
agentapi/
├── main.go                    # Entry point (8 lines)
├── cmd/                       # CLI commands (Cobra)
│   ├── root.go               # Root command setup
│   ├── server/               # Main server command
│   │   ├── server.go         # Server implementation & flags
│   │   └── server_test.go    # Configuration tests
│   └── attach/               # Terminal viewer command
│       └── attach.go         # TUI for viewing agent terminal
├── lib/                      # Core packages
│   ├── httpapi/              # HTTP server & API
│   │   ├── server.go         # API routes & handlers (482 lines)
│   │   ├── events.go         # SSE event streaming (220 lines)
│   │   ├── models.go         # Request/response types
│   │   ├── setup.go          # Process initialization
│   │   ├── embed.go          # Chat UI embedding
│   │   ├── claude.go         # Claude-specific helpers
│   │   ├── chat/             # Embedded Next.js static files
│   │   └── README.md         # API documentation
│   ├── termexec/             # Terminal execution wrapper
│   │   └── termexec.go       # PTY process management (181 lines)
│   ├── msgfmt/               # Message formatting per agent
│   │   ├── msgfmt.go         # Agent-specific formatters (276 lines)
│   │   ├── message_box.go    # TUI element detection
│   │   ├── msgfmt_test.go    # Formatting tests
│   │   └── testdata/         # Test cases per agent
│   ├── screentracker/        # Terminal state tracking
│   │   ├── conversation.go   # Message history & snapshots (411 lines)
│   │   ├── ringbuffer.go     # Circular snapshot buffer
│   │   └── conversation_test.go
│   ├── logctx/               # Context-based logging
│   │   └── logctx.go         # slog wrapper (26 lines)
│   └── util/                 # Common utilities
│       ├── util.go           # Helpers (waitFor, OpenAPI schema)
│       └── unsafe.go         # Reflection-based field access
├── chat/                     # Next.js web UI
│   ├── src/
│   │   ├── app/              # Next.js app router
│   │   │   ├── page.tsx      # Main page
│   │   │   ├── layout.tsx    # Root layout
│   │   │   └── embed/        # Embed variant
│   │   ├── components/       # React components
│   │   │   ├── chat-provider.tsx  # State management (291 lines)
│   │   │   ├── message-input.tsx  # Auto-resize input (283 lines)
│   │   │   ├── message-list.tsx   # Message display
│   │   │   └── ui/           # Radix UI components
│   │   └── lib/              # Utilities
│   ├── package.json
│   ├── next.config.ts
│   └── tailwind.config.ts
├── .github/
│   └── workflows/
│       ├── go-test.yml       # Test & lint on PR/push
│       └── release.yml       # Build cross-platform binaries
├── Makefile                  # Build automation
├── openapi.json              # OpenAPI 3.1 schema
├── CHANGELOG.md              # Version history
└── README.md                 # User documentation
```

### File Statistics
- **Total Go files**: 22
- **Total lines of Go**: ~3,000
- **Total React components**: ~1,400 lines
- **Test coverage**: 5 test files

---

## Architecture & Key Components

### System Flow

```
User Request → HTTP API → Conversation Tracker → Terminal Executor → Agent Process
                ↓                ↓                      ↓
            SSE Events    Message Parser      Screen Snapshots
```

### Component Responsibilities

#### 1. **termexec** - Terminal Process Execution
**Location**: `lib/termexec/termexec.go`

Manages PTY (pseudo-terminal) processes:
- Creates in-memory terminal emulator (80x1000 default)
- Runs agent as subprocess with VT100 terminal type
- Provides thread-safe screen reading
- Handles process signals and cleanup

**Key Methods**:
```go
StartProcess(cfg ProcessConfig) (*Process, error)  // Start agent
ReadScreen() (string, error)                        // Get terminal state
Write(data []byte) (int, error)                     // Send input
Signal(sig os.Signal) error                         // Send signal (e.g., SIGINT)
Wait() error                                        // Wait for exit
Close() error                                       // Graceful shutdown
```

**Implementation Note**: Uses reflection (`GetUnexportedField`) to access xpty's internal state for concurrent screen updates.

#### 2. **msgfmt** - Agent Message Formatting
**Location**: `lib/msgfmt/msgfmt.go`

Cleans agent output by removing:
- Echoed user input (agents often repeat the user's message)
- TUI elements (input boxes, prompts, decorative lines)

**Supported Agents**:
```go
const (
    AgentTypeClaude      = "claude"         // Claude Code
    AgentTypeGoose       = "goose"          // Goose
    AgentTypeAider       = "aider"          // Aider
    AgentTypeCodex       = "codex"          // OpenAI Codex
    AgentTypeGemini      = "gemini"         // Google Gemini
    AgentTypeAmp         = "amp"            // Sourcegraph Amp
    AgentTypeCursorAgent = "cursor-agent"   // Cursor CLI
    AgentTypeCursor      = "cursor"         // Cursor
    AgentTypeCustom      = "custom"         // Fallback
)
```

**Key Functions**:
```go
FormatAgentMessage(agentType AgentType, message string) string
RemoveUserInput(message, userInput string, agentType AgentType) (string, error)
```

**Matching Strategy**: Uses fuzzy rune-based matching to handle character transformations by agents (normalization, truncation, Unicode handling).

#### 3. **screentracker** - Conversation State Management
**Location**: `lib/screentracker/conversation.go`

Tracks terminal changes and builds conversation history:

**State Machine**:
```
Initializing → Stable → Changing → Stable → ...
```

**Message Types**:
```go
type ConversationRole string
const (
    ConversationRoleUser  ConversationRole = "user"
    ConversationRoleAgent ConversationRole = "agent"
)
```

**Key Mechanisms**:
- **Snapshots**: Periodic terminal screen captures with timestamps
- **RingBuffer**: Stores last 128 snapshots for stability detection
- **Diffing**: Compares screens before/after user input to extract agent response
- **Stability Detection**: Uses exponential backoff (16ms intervals, 2s max) to detect when agent finishes

**Key Methods**:
```go
NewConversation(process, agentType) *Conversation
SendMessage(message, messageType) error           // Submit to agent
Messages() []ConversationMessage                  // Get history
Status() ConversationStatus                       // "stable" or "running"
FindNewMessage(oldScreen, newScreen) string       // Extract new content
```

#### 4. **httpapi** - REST API Server
**Location**: `lib/httpapi/server.go`

Provides HTTP interface with OpenAPI schema:

**API Endpoints**:

| Method | Path | Description | Response |
|--------|------|-------------|----------|
| GET | `/status` | Agent status | `{"status": "stable"/"running"}` |
| GET | `/messages` | Conversation history | Array of messages |
| POST | `/message` | Send message | `{"ok": true}` |
| GET | `/events` | SSE event stream | Event stream |
| GET | `/internal/screen` | Raw terminal SSE | Screen updates |

**Request/Response Types**:
```go
type MessageRequest struct {
    Content string `json:"content"`
    Type    string `json:"type"`  // "user" or "raw"
}

type ConversationMessage struct {
    Id       int       `json:"id"`
    Content  string    `json:"content"`
    Role     string    `json:"role"`      // "user" or "agent"
    Timestamp time.Time `json:"timestamp"`
}
```

**Security Middleware**:
- **Host validation**: Checks `Host` header (default: localhost only)
- **CORS**: Configurable allowed origins
- **Default allowed hosts**: `localhost`, `127.0.0.1`, `[::1]`
- **Default allowed origins**: `http://localhost:3284`, `http://localhost:3000`, `http://localhost:3001`

**SSE Event Types**:
```go
// Sent via /events endpoint
"message_update"   // New or updated message
"status_change"    // Agent status changed
"screen_update"    // Terminal screen changed (for attach)
```

#### 5. **logctx** - Context Logging
**Location**: `lib/logctx/logctx.go`

Simple wrapper for passing `*slog.Logger` through context:
```go
WithLogger(ctx context.Context, logger *slog.Logger) context.Context
From(ctx context.Context) *slog.Logger  // Panics if missing
```

#### 6. **Frontend (Chat UI)**
**Location**: `chat/src/`

**Architecture**:
- **State Management**: React Context API (`chat-provider.tsx`)
- **Real-time Updates**: SSE subscription to `/events`
- **Optimistic UI**: Draft messages shown immediately
- **Auto-resize**: Textarea expands with content
- **Theme**: Dark/light mode support via `next-themes`

**Key Components**:
```tsx
<ChatProvider url="http://localhost:3284">  // Context wrapper
  <Chat />                                   // Main UI
    <MessageList />                          // Message display
    <MessageInput />                         // Input field
</ChatProvider>
```

**Backend Communication**:
```typescript
// Initial state: GET /messages, GET /status
// Send message: POST /message
// Live updates: EventSource on /events
```

---

## Development Setup

### Prerequisites
- Go 1.23.2 or later
- Bun (for frontend development)
- Make

### Initial Setup

```bash
# Clone repository
git clone https://github.com/coder/agentapi.git
cd agentapi

# Install Go dependencies (automatic on build)
go mod download

# Install frontend dependencies
cd chat && bun install && cd ..

# Build everything
make build

# Binary will be in: out/agentapi
```

### Development Workflow

#### Backend Development

```bash
# Run server directly (no build needed)
go run main.go server -- claude

# Run with specific agent
go run main.go server -- aider --model sonnet

# Override agent type detection
go run main.go server --type=codex -- codex

# Custom port and security settings
go run main.go server --port 8080 --allowed-hosts '*' -- claude

# Run tests
CGO_ENABLED=0 go test -count=1 -v ./...

# Run linter
golangci-lint run
```

#### Frontend Development

```bash
# Start backend first
go run main.go server -- claude

# In separate terminal, start frontend dev server
cd chat
bun run dev

# Access UI at: http://localhost:3000/chat/?url=http://localhost:3284

# Build frontend only
cd chat && bun run build

# Frontend will be output to: chat/out/
```

#### Full Build

```bash
# Build everything (frontend + backend)
make build

# Just build/embed frontend
make embed

# Output binary location
./out/agentapi
```

### Environment Variables

Configuration via environment variables (prefix: `AGENTAPI_`):

```bash
export AGENTAPI_TYPE=claude
export AGENTAPI_PORT=3284
export AGENTAPI_TERM_WIDTH=80
export AGENTAPI_TERM_HEIGHT=1000
export AGENTAPI_ALLOWED_HOSTS="localhost 127.0.0.1"     # Space-separated
export AGENTAPI_ALLOWED_ORIGINS="http://localhost:3284" # Space-separated
export AGENTAPI_CHAT_BASE_PATH=/chat
export AGENTAPI_PRINT_OPENAPI=false
```

**Precedence**: CLI flags > Environment variables > Defaults

---

## Code Conventions & Patterns

### Go Style

1. **Error Handling**: Use `golang.org/x/xerrors` for wrapping
   ```go
   if err != nil {
       return xerrors.Errorf("failed to do thing: %w", err)
   }
   ```

2. **Logging**: Use context-based `log/slog`
   ```go
   logger := logctx.From(ctx)
   logger.Info("message", "key", value)
   logger.Error("error occurred", "err", err)
   ```

3. **Concurrency**: Prefer mutexes for shared state
   ```go
   type SafeThing struct {
       mu    sync.Mutex
       state int
   }

   func (t *SafeThing) Update() {
       t.mu.Lock()
       defer t.mu.Unlock()
       t.state++
   }
   ```

4. **Testing**: Use testify for assertions
   ```go
   import "github.com/stretchr/testify/assert"

   assert.Equal(t, expected, actual)
   assert.NoError(t, err)
   ```

5. **Exhaustiveness**: Use golangci-lint's exhaustive checker
   ```go
   // All cases must be handled in switch on AgentType
   switch agentType {
   case AgentTypeClaude:
   case AgentTypeGoose:
   // ... all types must be covered
   default:
       // Required for custom types
   }
   ```

### File Organization

- **Tests**: Place `*_test.go` files in same package as code
- **Test data**: Use `testdata/` subdirectories
- **Package docs**: Add package comment in main file
- **Internal packages**: Use `lib/` for internal code (not exposed)

### Naming Conventions

- **Interfaces**: Not commonly used in this codebase
- **Structs**: PascalCase (exported), camelCase (unexported)
- **Constants**: PascalCase for exported, camelCase for unexported
- **Functions**: PascalCase for exported, camelCase for unexported
- **Files**: snake_case.go

### Frontend (TypeScript/React)

1. **Components**: PascalCase filenames (e.g., `ChatProvider.tsx`)
2. **Hooks**: Use custom hooks for reusable logic
3. **State**: React Context for global, useState for local
4. **Styling**: Tailwind utility classes
5. **Types**: Define interfaces for props

---

## Testing Strategy

### Test Coverage

Current test files:
- `lib/httpapi/server_test.go` - HTTP server & configuration
- `lib/httpapi/events_test.go` - SSE event system
- `lib/msgfmt/msgfmt_test.go` - Message formatting
- `lib/screentracker/conversation_test.go` - Conversation tracking
- `cmd/server/server_test.go` - CLI flag parsing

### Running Tests

```bash
# Run all tests
CGO_ENABLED=0 go test -count=1 -v ./...

# Run specific package
go test -v ./lib/msgfmt

# Run specific test
go test -v ./lib/msgfmt -run TestRemoveUserInput

# With coverage
go test -cover ./...

# Run linter
golangci-lint run
```

### Test Data Structure

Message formatting tests use fixtures:
```
lib/msgfmt/testdata/
├── format/                    # Full message formatting
│   ├── aider/
│   │   ├── first_message/
│   │   │   ├── input.txt
│   │   │   └── output.txt
│   │   └── second_message/
│   └── claude/
└── remove-user-input/         # User input removal
    └── [test-name]/
        ├── agent-output.txt   # Raw agent response
        ├── user-input.txt     # User's message
        └── expected.txt       # Cleaned output
```

### Writing New Tests

1. **Unit tests**: Test individual functions
   ```go
   func TestFormatMessage(t *testing.T) {
       result := FormatAgentMessage(AgentTypeClaude, input)
       assert.Equal(t, expected, result)
   }
   ```

2. **Table-driven tests**: For multiple scenarios
   ```go
   tests := []struct {
       name     string
       input    string
       expected string
   }{
       {"case1", "input1", "output1"},
       {"case2", "input2", "output2"},
   }
   for _, tt := range tests {
       t.Run(tt.name, func(t *testing.T) {
           result := Function(tt.input)
           assert.Equal(t, tt.expected, result)
       })
   }
   ```

3. **Test data files**: For complex inputs
   ```go
   input := readTestFile(t, "testdata/format/claude/input.txt")
   expected := readTestFile(t, "testdata/format/claude/output.txt")
   ```

### CI Testing

Tests run automatically on:
- Every push to `main`
- Every pull request
- Uses GitHub Actions (`.github/workflows/go-test.yml`)

**Required checks**:
- All tests pass: `go test ./...`
- Linter passes: `golangci-lint run`
- CGO disabled: `CGO_ENABLED=0`

---

## Making Changes

### Adding a New Agent

**Files to modify**:
1. `lib/msgfmt/msgfmt.go` - Add agent type constant and formatting logic
2. `cmd/server/server.go` - Add to `agentTypeMap`
3. Create test data in `lib/msgfmt/testdata/format/[agent-name]/`

**Steps**:

```go
// 1. Add constant in lib/msgfmt/msgfmt.go
const AgentTypeNewAgent AgentType = "newagent"

// 2. Add to server.go agentTypeMap
var agentTypeMap = map[AgentType]bool{
    // ... existing types
    AgentTypeNewAgent: true,
}

// 3. Add formatting logic in msgfmt.go
func FormatAgentMessage(agentType AgentType, message string) string {
    switch agentType {
    // ... existing cases
    case AgentTypeNewAgent:
        return removeMessageBox(message, []string{"custom-prompt-pattern"})
    }
}

// 4. Create test data
// lib/msgfmt/testdata/format/newagent/first_message/input.txt
// lib/msgfmt/testdata/format/newagent/first_message/output.txt
```

**Test**:
```bash
go test -v ./lib/msgfmt -run TestFormat
agentapi server --type=newagent -- newagent-cli
```

### Adding a New API Endpoint

**File**: `lib/httpapi/server.go`

```go
// 1. Define request/response types in models.go
type NewRequest struct {
    Field string `json:"field" example:"value"`
}

type NewResponse struct {
    Result string `json:"result"`
}

// 2. Add handler function
func (s *Server) handleNewEndpoint(
    ctx context.Context,
    req *NewRequest,
) (*NewResponse, error) {
    // Implementation
    return &NewResponse{Result: "success"}, nil
}

// 3. Register route in registerRoutes()
huma.Register(s.api, huma.Operation{
    OperationID: "new-endpoint",
    Method:      http.MethodPost,
    Path:        "/new-endpoint",
    Summary:     "Description",
}, s.handleNewEndpoint)

// 4. Update openapi.json
go run main.go server --print-openapi > openapi.json
```

### Modifying Message Formatting

**File**: `lib/msgfmt/msgfmt.go`

When agents update their TUI, you may need to adjust formatters:

```go
// Example: Update input box detection patterns
func removeMessageBox(message string, patterns []string) string {
    // Add new pattern to detect input box
    patterns = append(patterns, "new-pattern-to-remove")
    // ... rest of logic
}
```

**Test changes**:
1. Create test case in `testdata/format/[agent]/`
2. Run: `go test -v ./lib/msgfmt`
3. Test with real agent: `agentapi server -- [agent]`

### Frontend Changes

**Common tasks**:

1. **Modify chat UI**:
   - Edit `chat/src/components/chat.tsx`
   - Changes hot-reload at `http://localhost:3000`

2. **Change API interaction**:
   - Edit `chat/src/components/chat-provider.tsx`
   - Update fetch calls or SSE handling

3. **Add new component**:
   ```bash
   cd chat/src/components
   # Create new-component.tsx
   # Import and use in chat.tsx
   ```

4. **Update styles**:
   - Modify Tailwind classes in components
   - Or edit `chat/tailwind.config.ts` for theme

**Build for production**:
```bash
cd chat
NEXT_PUBLIC_BASE_PATH="/chat" bun run build
```

---

## Common Tasks

### Task 1: Debug Agent Communication

**Symptoms**: Agent not responding, messages malformed

**Steps**:
1. Check terminal output:
   ```bash
   # Use attach command to see raw terminal
   agentapi attach --url localhost:3284
   ```

2. Enable debug logging:
   ```go
   // In server.go, add
   logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
       Level: slog.LevelDebug,
   }))
   ```

3. Check message formatting:
   ```bash
   # Test with raw message type
   curl -X POST localhost:3284/message \
     -H "Content-Type: application/json" \
     -d '{"content": "test", "type": "raw"}'
   ```

### Task 2: Update OpenAPI Schema

**When**: After changing API endpoints or types

```bash
# Generate new schema
go run main.go server --print-openapi > openapi.json

# Commit the updated schema
git add openapi.json
git commit -m "docs: update OpenAPI schema"
```

### Task 3: Add Host/Origin for Deployment

**Scenario**: Deploying AgentAPI on custom domain

```bash
# Allow specific host
agentapi server --allowed-hosts 'example.com' -- claude

# Allow any host (less secure)
agentapi server --allowed-hosts '*' -- claude

# Multiple hosts
agentapi server --allowed-hosts 'example.com,example.org' -- claude

# CORS origins (must include protocol)
agentapi server --allowed-origins 'https://example.com,https://*.example.com' -- claude
```

### Task 4: Reproduce Test Failure

```bash
# Run specific test multiple times
go test -v ./lib/msgfmt -run TestRemoveUserInput -count=10

# Run with race detector
go test -race ./...

# Run with coverage
go test -cover -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Task 5: Update Frontend Dependencies

```bash
cd chat

# Update all dependencies
bun update

# Update specific package
bun update next

# Check for outdated packages
bun outdated
```

### Task 6: Cross-Platform Build

```bash
# Linux AMD64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 make build

# Linux ARM64
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 make build

# macOS Apple Silicon
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 make build

# macOS Intel
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 make build

# Windows
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 BINPATH=out/agentapi.exe make build
```

---

## CI/CD & Release Process

### Continuous Integration

**On every push/PR** (`.github/workflows/go-test.yml`):
1. Checkout code
2. Setup Go (stable version)
3. Run tests: `CGO_ENABLED=0 go test -count=1 -v ./...`
4. Run linter: `golangci-lint` v2.1

**Required for merge**:
- All tests pass
- Linter passes
- No golangci-lint warnings

### Release Process

**Triggering builds** (`.github/workflows/release.yml`):
- On release published (GitHub UI)
- On push to `main` (creates "preview" release)
- Manual workflow dispatch

**Build matrix**:
```
Platform           | Binary Name
-------------------|-------------------------
Linux AMD64        | agentapi-linux-amd64
Linux ARM64        | agentapi-linux-arm64
macOS AMD64        | agentapi-darwin-amd64
macOS ARM64        | agentapi-darwin-arm64
Windows AMD64      | agentapi-windows-amd64.exe
```

**Build steps**:
1. Checkout code
2. Setup Go + Bun
3. Install frontend dependencies: `cd chat && bun install`
4. Build all platforms: `CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch make build`
5. Upload to GitHub release

**Creating a new release**:

```bash
# 1. Update CHANGELOG.md
vim CHANGELOG.md
# Add new version section with features/fixes

# 2. Commit changelog
git add CHANGELOG.md
git commit -m "chore: prepare v0.x.x release"
git push

# 3. Create and push tag
git tag v0.x.x
git push origin v0.x.x

# 4. Create GitHub release (via UI or gh CLI)
gh release create v0.x.x \
  --title "v0.x.x" \
  --notes "See CHANGELOG.md for details"

# 5. CI automatically builds and uploads binaries
```

### Version Numbering

Follow semantic versioning:
- **Major** (v1.0.0): Breaking changes
- **Minor** (v0.5.0): New features, backward compatible
- **Patch** (v0.4.1): Bug fixes, backward compatible

---

## Troubleshooting

### Common Issues

#### Issue: "agent not found" or "command not found"

**Cause**: Agent binary not in PATH

**Solution**:
```bash
# Find full path to agent
which claude  # or goose, aider, etc.

# Use full path
agentapi server -- /full/path/to/claude
```

#### Issue: Terminal output garbled or messages malformed

**Cause**: Incorrect agent type detection or outdated formatting logic

**Solution**:
```bash
# Explicitly set agent type
agentapi server --type=claude -- claude

# Check raw terminal output
agentapi attach --url localhost:3284

# Check if agent updated their TUI (may need msgfmt update)
```

#### Issue: CORS errors in browser

**Cause**: Origin not in allowed list

**Solution**:
```bash
# Add your frontend origin
agentapi server --allowed-origins 'http://localhost:3000' -- claude

# Or allow all origins (development only!)
agentapi server --allowed-origins '*' -- claude
```

#### Issue: "Host not allowed" error

**Cause**: Custom hostname not in allowed hosts

**Solution**:
```bash
# Add your hostname
agentapi server --allowed-hosts 'example.com' -- claude

# Or allow all hosts
agentapi server --allowed-hosts '*' -- claude
```

#### Issue: Frontend shows "Connection error"

**Cause**: Backend not running or wrong URL

**Solution**:
```bash
# Check backend is running
curl http://localhost:3284/status

# Access UI with correct URL parameter
http://localhost:3000/chat/?url=http://localhost:3284

# Check browser console for detailed error
```

#### Issue: Tests fail with "terminal not responding"

**Cause**: Race condition in terminal emulation

**Solution**:
```bash
# Run with race detector to identify
go test -race ./...

# Add synchronization if needed (mutex, channels)
```

#### Issue: Build fails with "chat sources changed"

**Cause**: Frontend needs rebuilding

**Solution**:
```bash
# Rebuild frontend
make embed

# Or clean and rebuild
rm -rf lib/httpapi/chat chat/out chat/.sources.stamp
make build
```

### Debugging Tools

1. **Attach command**: View raw terminal
   ```bash
   agentapi attach --url localhost:3284
   ```

2. **curl commands**: Test API manually
   ```bash
   # Get status
   curl http://localhost:3284/status

   # Get messages
   curl http://localhost:3284/messages

   # Send message
   curl -X POST http://localhost:3284/message \
     -H "Content-Type: application/json" \
     -d '{"content": "hello", "type": "user"}'

   # Watch events
   curl http://localhost:3284/events
   ```

3. **SSE debugging**: Use browser EventSource
   ```javascript
   const events = new EventSource('http://localhost:3284/events');
   events.onmessage = (e) => console.log(JSON.parse(e.data));
   ```

4. **Go debugger**: Use delve
   ```bash
   # Install delve
   go install github.com/go-delve/delve/cmd/dlv@latest

   # Debug server
   dlv debug main.go -- server -- claude

   # Set breakpoint
   (dlv) break lib/httpapi/server.go:100
   (dlv) continue
   ```

### Getting Help

- **Issues**: Check existing issues at https://github.com/coder/agentapi/issues
- **Discussions**: For questions, use GitHub Discussions
- **Documentation**: Refer to README.md and this CLAUDE.md

---

## Key Principles for AI Assistants

When working on this codebase:

1. **Preserve backward compatibility**: API changes should be additive
2. **Test thoroughly**: Add tests for new features
3. **Update OpenAPI schema**: After API changes
4. **Follow agent patterns**: New agents should match existing structure
5. **Keep formatters resilient**: Use fuzzy matching, handle edge cases
6. **Document breaking changes**: Update CHANGELOG.md
7. **Respect security boundaries**: Don't bypass host/origin validation
8. **Use CGO_ENABLED=0**: For maximum compatibility
9. **Keep frontend embedded**: Don't require separate frontend deployment
10. **Maintain clean separation**: Terminal execution ← Message parsing ← HTTP API

---

## Quick Reference

### Build Commands
```bash
make build              # Full build (frontend + backend)
make embed              # Just build frontend
go run main.go server   # Run without building
go test ./...           # Run all tests
golangci-lint run       # Run linter
```

### Server Commands
```bash
agentapi server -- claude                           # Start server
agentapi server --port 8080 -- claude               # Custom port
agentapi server --type=goose -- aider               # Override type
agentapi server --allowed-hosts '*' -- claude       # Allow any host
agentapi attach --url localhost:3284                # View terminal
```

### API Endpoints
```
GET  /status              # Agent status
GET  /messages            # Message history
POST /message             # Send message
GET  /events              # SSE stream
GET  /chat                # Web UI (embedded)
GET  /docs                # API documentation UI
GET  /openapi.json        # OpenAPI schema
```

### File Reference
| Task | File |
|------|------|
| Add agent | `lib/msgfmt/msgfmt.go`, `cmd/server/server.go` |
| Add endpoint | `lib/httpapi/server.go` |
| Modify CLI | `cmd/server/server.go` |
| Change formatting | `lib/msgfmt/msgfmt.go` |
| Update UI | `chat/src/components/` |
| Fix terminal | `lib/termexec/termexec.go` |
| Change tracking | `lib/screentracker/conversation.go` |

---

**Last Updated**: 2025-11-15
**AgentAPI Version**: v0.5.0
**Go Version**: 1.23.2
**Maintainer**: Coder (https://github.com/coder)
