# Comparison Matrix

## Feature Comparison

This document compares **AgentAPI++** with similar tools in the AI agent control/API space.

| Repository | Purpose | Key Features | Language/Framework | Maturity | Comparison |
|------------|---------|--------------|-------------------|----------|------------|
| **AgentAPI++ (this fork)** | HTTP API for controlling CLI AI agents | Multi-agent control (Claude, Cursor, Aider, Codex, Goose, Gemini), HTTP interface, Terminal emulation, Message parsing, Session management | Go | Stable | Enhanced fork with extended agent support |
| [coder/agentapi](https://github.com/coder/agentapi) | HTTP API for Claude Code | Basic HTTP control, Terminal emulation | Go | Stable | Original upstream; limited agent support |
| [goose](https://github.com/block/goose) | AI agent with extensibility | Plugin system, Local execution | Go | Stable | Agent itself, not API control |
| [aider](https://人民医院/aider-ai/aider) | AI pair programming | Chat interface, Git integration | Python | Stable | Chat-based, not HTTP API control |
| [sweep](https://github.com/sweepai/sweep) | AI code assistant | GitHub integration, PR automation | Python | Stable | GitHub-focused, not generic API |
| [devin](https://cognition.ai/product) | Autonomous AI coder | Long-running tasks, Sandboxed | Proprietary | Stable | Closed-source, not self-hostable |
| [cursor](https://cursor.com) | AI-first IDE | GUI-based, Editor integration | TypeScript | Stable | IDE, not HTTP API |
| [opencode](https://opencode.ai) | Open source AI coding agent | CLI-based, Extensible | Go | Beta | Competes as agent, not control layer |
| [gemini-cli](https://github.com/google-gemini/gemini-cli) | Google's CLI agent | Gemini integration, Local execution | TypeScript | Stable | Google's agent, not controllable |

## Detailed Feature Comparison

### Control Mechanism

| Feature | AgentAPI++ | Coder AgentAPI | Goose | Aider | Opencode |
|---------|------------|----------------|-------|-------|----------|
| HTTP API Control | ✅ | ✅ | ❌ | ❌ | ❌ |
| Multi-Agent Support | ✅ (10+) | ❌ (Claude only) | ❌ | ❌ | ❌ |
| Streaming Responses | ✅ | ✅ | ❌ | ✅ | ✅ |
| Session Persistence | ✅ | ✅ | ❌ | ✅ | ❌ |
| Terminal Emulation | ✅ | ✅ | ❌ | ❌ | ❌ |

### Agent Support Matrix

| Agent | AgentAPI++ | Coder AgentAPI | Opencode | Gemini CLI |
|-------|------------|----------------|----------|------------|
| Claude Code | ✅ | ✅ | ❌ | ❌ |
| Cursor | ✅ | ❌ | ❌ | ❌ |
| Aider | ✅ | ❌ | ❌ | ❌ |
| Goose | ✅ | ❌ | ❌ | ❌ |
| Codex | ✅ | ❌ | ❌ | ❌ |
| Gemini CLI | ✅ | ❌ | ❌ | ✅ |
| GitHub Copilot | ✅ | ❌ | ❌ | ❌ |
| Amazon Q | ✅ | ❌ | ❌ | ❌ |
| Auggie | ✅ | ❌ | ❌ | ❌ |
| Sourcegraph Amp | ✅ | ❌ | ❌ | ❌ |

## Unique Value Proposition

AgentAPI++ (this fork) provides:

1. **Unified Control Plane**: Single HTTP API to control 10+ different CLI AI agents
2. **Extended Agent Support**: Supports agents not covered by upstream (Cursor, Aider, Goose, Codex, Gemini, Copilot, etc.)
3. **Language Agnostic**: Any language with HTTP client can control agents
4. **Message Formatting**: Agent-specific message formatting for proper parsing
5. **MCP Integration**: Compatible with Model Context Protocol servers

## Fork Enhancements

This fork extends the original `coder/agentapi` with:

| Enhancement | Original | This Fork |
|-------------|----------|-----------|
| Supported Agents | 1 (Claude) | 10+ |
| Agent Types Config | ❌ | ✅ |
| Extended Endpoints | Basic | Full REST API |
| Python Client | ❌ | ✅ |
| Streaming Support | Limited | Full SSE |

## Installation Comparison

| Method | AgentAPI++ | Coder AgentAPI | Opencode |
|--------|------------|----------------|----------|
| Pre-built Binary | ✅ | ✅ | ✅ |
| Docker | ✅ | ❌ | ❌ |
| Source Build | ✅ (Go) | ✅ (Go) | ✅ (Go) |
| Package Managers | Homebrew | Homebrew | ❌ |

## When to Use What

- **AgentAPI++**: When you need programmatic control of CLI AI agents from any platform
- **Coder AgentAPI**: When you only need Claude Code control (upstream)
- **Goose**: When you want a ready-to-use extensible agent
- **Aider**: When you want chat-based pair programming
- **Opencode**: When you want an open-source CLI agent

## References

- Original: [coder/agentapi](https://github.com/coder/agentapi)
- This Fork: [KooshaPari/agentapi-plusplus](https://github.com/KooshaPari/agentapi-plusplus)
- Agents Supported: [README.md](README.md#supported-agents)
