# AGENTS.md

This file provides guidance to AI agents working with code in this repository.

## Quick Start

```bash
# Development
go build -o agentapi ./cmd/agentapi

# Testing
go test ./...

# Linting
go fmt ./...
go vet ./...
```

## Environment

```bash
# Required environment variables
export CLIPROXY_URL="http://localhost:8317"
export AGENTAPI_PORT="8318"
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
| Logging | zerolog | fmt.Print |
| CLI | cobra | manual flag parsing |
| Config | viper | manual env parsing |

---

## Code Quality Non-Negotiables

- Zero new lint suppressions without inline justification
- All new code must pass: go fmt, go vet, golint
- Max function: 40 lines
- No placeholder TODOs in committed code

---

## Verifiable Constraints

| Metric | Threshold | Enforcement |
|--------|-----------|-------------|
| Tests | 80% coverage | CI gate |
| Lint | 0 errors | golangci-lint |

---

## Supported Agents

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
├── parpour/        # Spec-first planning
└── pheno-sdk/       # Python SDK
```
