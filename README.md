# AgentAPI

Control [Claude Code](https://github.com/anthropics/claude-code), [Goose](https://github.com/block/goose), [Aider](https://github.com/Aider-AI/aider), and [Codex](https://github.com/openai/codex) with an HTTP API.

![chat demo](https://github.com/user-attachments/assets/11685cf3-324b-4e72-b8e9-5bb8ceede785)

You can use AgentAPI:

- to build a unified chat interface for coding agents
- as a backend in an MCP server that lets one agent control another coding agent
- to create a tool that submits pull request reviews to an agent
- and much more!

## Quickstart

Install `agentapi`:

1. Download the latest release binary from the [releases page](https://github.com/coder/agentapi/releases)
2. Rename it (e.g. `mv agentapi-darwin-arm64 agentapi`)
3. Make it executable (`chmod +x agentapi`)
4. Put it in your PATH (e.g. `sudo mv agentapi /usr/local/bin`)
5. Verify the installation (`agentapi --help`)
6. (macOS) If you're prompted that macOS was unable to verify the binary, go to `System Settings -> Privacy & Security`, click "Open Anyway", and run the command again.

Run a Claude Code server (assumes `claude` is installed on your system):

```bash
agentapi server -- claude
```

Send a message to the agent:

```bash
curl -X POST localhost:3284/message \
  -H "Content-Type: application/json" \
  -d '{"content": "Hello, agent!", "type": "user"}'
```

Get the conversation history:

```bash
curl localhost:3284/messages
```

We host a demo web chat interface at https://coder.github.io/agentapi/chat which will connect to your local AgentAPI server and let you test it out.

## CLI Commands

### `agentapi server`

Run an HTTP server that lets you control an agent. If you'd like to start an agent with additional arguments, pass the full agent command after the `--` flag.

```bash
agentapi server -- claude --allowedTools "Bash(git*) Edit Replace"
```

You may also use `agentapi` to run the Aider and Goose agents:

```bash
agentapi server -- aider --model sonnet --api-key anthropic=sk-ant-apio3-XXX
agentapi server -- goose
```

The OpenAPI schema is available in this repository: [openapi.json](openapi.json).

By default, the server runs on port 3284. Additionally, the server exposes an OpenAPI schema at http://localhost:3284/openapi.json. You may also inspect the available endpoints at http://localhost:3284/docs.

There are 4 endpoints:

- GET `/messages` - returns a list of all messages in the conversation with the agent
- POST `/message` - sends a message to the agent. When a 200 response is returned, AgentAPI has detected that the agent started processing the message
- GET `/status` - returns the current status of the agent, either "stable" or "running"
- GET `/events` - an SSE stream of events from the agent: message and status updates

### `agentapi attach`

Attach to a running agent's terminal session.

```bash
agentapi attach --url localhost:3284
```

Press `ctrl+c` to detach from the session.

## How it works

AgentAPI runs an in-memory terminal emulator. It translates API calls into appropriate terminal keystrokes and parses the agent's outputs into individual messages.

### Splitting terminal output into messages

There are 2 types of messages:

- User messages: sent by the user to the agent
- Agent messages: sent by the agent to the user

To parse individual messages from the terminal output, we take the following steps:

1. The initial terminal output, before any user messages are sent, is treated as the agent's first message.
2. When the user sends a message through the API, a snapshot of the terminal is taken before any keystrokes are sent.
3. The user message is then submitted to the agent. From this point on, any time the terminal output changes, a new snapshot is taken. It's diffed against the initial snapshot, and any new text that appears below the initial content is treated as the agent's next message.
4. If the terminal output changes again before a new user message is sent, the agent message is updated.

This lets us split the terminal output into a sequence of messages.

### Removing TUI elements from agent messages

Each agent message contains some extra bits that aren't useful to the end user:

- The user's input at the beginning of the message. Coding agents often echo the input back to the user to make it visible in the terminal.
- An input box at the end of the message. This is where the user usually types their input.

AgentAPI automatically removes these.

- For user input, we strip the lines that contain the text from the user's last message.
- For the input box, we look for lines at the end of the message that contain common TUI elements, like `>` or `------`.

### What will happen when Claude Code, Goose, Aider, or Codex update their TUI?

Splitting the terminal output into a sequence of messages should still work, since it doesn't depend on the TUI structure. The logic for removing extra bits may need to be updated to account for new elements. AgentAPI will still be usable, but some extra TUI elements may become visible in the agent messages.

## Long-term vision

In the short term, AgentAPI solves the problem of how to programmatically control coding agents. As time passes, we hope to see the major agents release proper SDKs. One might wonder whether AgentAPI will still be needed then. We think that depends on whether agent vendors decide to standardize on a common API, or each sticks with a proprietary format.

In the former case, we'll deprecate AgentAPI in favor of the official SDKs. In the latter case, our goal will be to make AgentAPI a universal adapter to control any coding agent, so a developer using AgentAPI can switch between agents without changing their code.
