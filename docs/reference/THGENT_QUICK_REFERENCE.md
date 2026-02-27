# Thegent Quick Reference

Unified agent orchestration CLI for Factory skills.

## Install

```bash
# Option A: Install to PATH (recommended)
export THEGENT_ROOT=${THEGENT_ROOT:-/Users/kooshapari/temp-PRODVERCEL/485/kush/thegent}
uv tool install --editable "$THEGENT_ROOT"
# Then:
thegent run "prompt" gemini

thegent run -M gemini-3-flash "prompt" 
```

Use thegent CLI directly. No Makefile wrappers.

## Commands

| Command | Description |
|---------|-------------|
| `thegent run "<prompt>" [agent]` | Run agent in foreground |
| `thegent bg "<prompt>" [agent]` | Start background run, register session |
| `thegent ps [--owner X] [--all]` | List background sessions |
| `thegent status <session_id>` | Show one session status |
| `thegent inspect [--owner X] <session_id...>` | Get status + logs for sessions |
| `thegent logs [--follow] [--tail N] <session_id>` | Print session logs |
| `thegent wait [-t N] <session_id>` | Wait for session completion |
| `thegent stop [--force] <session_id>` | Stop a running session |
| `thegent list-agents` | List available agents |
| `thegent list-models [provider]` | List models from config and CLI providers |

## Run Options

| Option | Short | Default | Description |
|--------|-------|---------|-------------|
| `--cd` | `-d` | inferred | Working directory |
| `--mode` | `-m` | write | read-only, write, full |
| `--timeout` | `-t` | 90 | Timeout in seconds |
| `--full` | `-f` | false | Show full raw output |
| `--model` | `-M` | provider default | Model override (e.g. `gemini-3-flash`, `composer-1.5`, `auto` for cursor) |
| `--provider` | `-P` | none | Provider hint for model-first routing |

**Tip:** The CLI is Typer-based. Use options before/after positionals as needed and keep prompt quoted.

## CWD Inference

- If `--cd` is provided: uses it
- Else if current dir has `.git`, `.factory`, or `pyproject.toml`: uses cwd
- Else: exit with error and require explicit `--cd`

## Providers (Agents)

| Provider | Type | Backend |
|----------|------|---------|
| gemini | agent | Gemini CLI |
| codex | agent | OpenAI Codex |
| copilot | agent | GitHub Copilot |
| cursor-agent / cursor | agent | Cursor Agent |
| claude | agent | Claude Code |

```bash
thegent run "List top-level directories" gemini
thegent run "Summarize plan" cursor-agent
thegent run "Create WBS" gemini
```

## Examples

```bash
# Inferred cwd (from project root)
thegent run "List top-level directories" gemini

thegent run -d /path/to/project "Explore" gemini

# Read-only mode
thegent run -m read-only -t 120 "Analyze" gemini

# Model-first
thegent run -M gemini-3-flash "Deep analysis"

thegent run -M sonnet --provider claude "Architectural assessment"

# Other agent examples
thegent run "Expand idea X" minimax
thegent run "Create WBS for feature X" glm

# Background runs
thegent bg -d "$(pwd)" --owner "${USER}:trace" -f "Long lint task" cursor-agent
thegent bg -d "$(pwd)" --owner "${USER}:trace" -f minimax "Write docs"
thegent ps --owner "${USER}:trace"
thegent status <session_id>
thegent logs --follow <session_id>
thegent wait -t 120 <session_id>
thegent stop <session_id>
```

## Timeout / Tool-Call Budget

`-t` sets process timeout and injects a tool-call budget into the prompt (~2.3s per tool call). The agent is told to wrap up when approaching the limit, since process kill is inherently best-effort.

## Output (default: parsed)

Default output is stream-JSON where supported, with condensed final assistant output shown. Use `--full`/`-f` for raw logs.

## Output Control

Use `--full`, `--format json|md|rich` on session commands as needed.

## Environment

| Variable | Default | Description |
|----------|---------|-------------|
| `THGENT_FACTORY_SKILLS_DIR` | `~/.factory/skills` | Factory skills directory |
| `THGENT_CURSOR_AGENT_CMD` | `cursor-agent` | Override cursor cli/binary |
| `THGENT_DEFAULT_CURSOR_MODEL` | `gemini-3-flash` | Default model for cursor agent |
| `THGENT_DEFAULT_GEMINI_MODEL` | `gemini-3-flash` | Default model for gemini CLI |
| `THGENT_DEFAULT_COPILOT_MODEL` | `claude-haiku-4.5` | Default model for copilot |
| `THGENT_DEFAULT_CLAUDE_MODEL` | `haiku` | Default model for claude |
| `THGENT_DEFAULT_CODEX_MODEL` | `gpt-5.3-codex` | Default model for codex |
| `THGENT_DEFAULT_TIMEOUT` | 90 | Default timeout (seconds) |

**Cursor models:** `gemini-3-flash` (default), `composer-1.5`, `auto`. Use `thegent list-models cursor` or `thegent list-models cursor-agent` to see all.
