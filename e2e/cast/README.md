# Cast-Based E2E Testing

This directory contains a cast-based end-to-end (E2E) test for the AgentAPI project. The framework simulates realistic agent interactions by replaying asciicast v2 recordings.

## TL;DR

```shell
go test ./e2e/cast
```

## How it Works

The testing framework (`cast_test.go`) does the following:
- Starts the AgentAPI server with a fake agent (`cmd/cast_agent.go`).
- The fake agent replays a `.cast` file (asciicast v2 format), writing terminal output to stdout and validating stdin against the recorded input events.
- The testing framework sends messages to the fake agent via the AgentAPI and validates the responses.

## Adding or Updating Fixtures

### Step 1: Record with asciinema

The recording captures both agentapi's output (what it sends to the agent) and the agent's responses.
Use `asciinema` to record agentapi wrapping the agent. Below is an example for Claude:

```shell
# Set terminal to minimum size Claude supports (80 columns)
stty cols 80 rows 1000

# Build agentapi (or use an existing known-good version)
agentapi server -t claude -- asciinema rec --stdin testdata/claude.cast --command 'echo hello | claude'

# Then interact with Claude via AgentAPI (either web UI or API):
# 1. Wait for Claude to respond to the initial prompt
# 2. Type a test message (e.g., "This is just a test.")
# 3. Wait for Claude's reply
# 4. Press Ctrl+C to exit
```

**Important notes:**
- The terminal dimensions (80x1000) must match the `--term-width` and `--term-height` flags in the test (see `defaultCmdFn` in `cast_test.go`).
- Recording agentapi (not Claude directly) captures the exact byte sequences that agentapi sends, including bracketed paste mode escape sequences.
- The `--stdin` flag captures your input, which the test uses to validate that agentapi sends the correct bytes.
- To overwrite an existing fixture, add `--overwrite` to the `asciinema` invocation.

### Step 2: Create the sidecar script file

Create a matching sidecar file `testdata/my-fixture.txt` that lists the expected conversation in order, one entry per line:

```
user	hello
agent	Hello! How can I help you today?
user	This is just a test.
agent	Got it! Let me know if you need anything.
```

Each line is `<role><TAB><message>`. Valid roles are `agent` or `user`.

**Flow explanation:**
- Line 1: The initial prompt passed to the agent at startup
- Line 2: The agent's reply to the initial prompt
- Line 3: The test message sent via the AgentAPI
- Line 4: The agent's reply to the test message

Note: The AgentAPI merges the startup sequence (welcome screen + initial prompt + initial reply) into the first agent message. The test expects 3 messages from the API, even though the script file has 4 entries.

### Step 3: Review and update the test

> **Caution:** Review the recording before committing. Remove or redact any API keys, tokens, or other sensitive data that may appear in the terminal output (stdout events), recorded keystrokes (`"i"` events from `--stdin`), or environment variables captured in the cast header.

To use a new fixture, update the `castFile` and `scriptFile` constants in `cast_test.go` to reference the new fixture files, then update the assertions in `TestE2E` to match the new conversation.
