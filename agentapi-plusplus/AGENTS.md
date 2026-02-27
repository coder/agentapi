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

## Child-Agent and AI Review Governance

## Child-Agent and governance standard
- Use child agents as default for writing/reading/research tasks. Default operating cap is `syscap: 6` concurrent workers.
- Use a two-stage flow for larger changes: one agent performs standards/research first, then implementation agents execute; one lead agent coordinates and performs final synthesis/review.
- Cloud review pipeline expectation: run incremental review waves; target **5-10 review cycles** total when needed, with explicit stop criteria (all critical findings resolved, no open `request_changes`, and approval readiness on the last pass).
- Branch and release flow: `main` is production, `develop` is integration; create `release/*` branches for pre-releases (`-alpha`, `-beta`, `-rc`) and merge into `main` when green.
- Versioning standard: use SemVer (`MAJOR.MINOR.PATCH`) plus prerelease identifiers for pre-production (`x.y.z-alpha.n`, `x.y.z-beta.n`, `x.y.z-rc.n`).
- Tooling preference across workflows: use `uv` over `pip`, and `bun` over `npm` unless a project explicitly requires otherwise.
- Review config policy should inherit from `https://github.com/0xquinto/bcherny-claude/main/.coderabbit.yml` when available.


## Cloud Review Loop Contract

- Define one review gate family for all PRs: run cloud review after every pushed change and auto-request another pass until quality is stable or the max turn budget is reached.
- Standard target is **5-10 auto-review turns** per PR before manual final handoff.
- Do not pause for human review while critical findings exist or the stop condition (`all critical findings = 0`, no pending change requests, all required checks green) is unmet.
- For each PR include a machine review manifest section:
  - `review_turns`: integer count across cloud passes
  - `pending_actions`: list of unresolved agent findings
  - `stop_reason`: `max_turns`, `all_green`, or `human_hold`
- Branch/version rules for release channels:
  - `main` is production.
  - `develop` is integration.
  - Release channels: `alpha`, `beta`, `canary`, `release/*`, `staging`, `production`.
  - Tag format: `vX.Y.Z`, prereleases `vX.Y.Z-alpha.N`, `vX.Y.Z-beta.N`, `vX.Y.Z-rc.N`, `vX.Y.Z-canary.N`.
  - `alpha` -> `beta` -> `canary` -> `release/*` -> `main` promotion flow with evidence gates at each step.
- Cliproxy release requirement: release artifacts must include OS/arch binaries for `linux`, `darwin`, `windows` and both `amd64` and `arm64` unless there is an explicit platform exclusion reason in release notes.
- Tooling defaults in CI and release scripts: prefer `uv` over `pip`, `bun` over `npm` unless explicitly justified.
