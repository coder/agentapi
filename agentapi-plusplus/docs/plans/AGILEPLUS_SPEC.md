# AgilePlus Methodology Specification

## Overview

AgilePlus is an enhanced Agile framework designed for AI-native software development. It builds on core Agile principles while addressing the unique challenges of agent-driven, multi-repo software systems. AgilePlus emphasizes rapid iteration, comprehensive quality gates, and seamless human-agent collaboration.

## Core Principles

### 1. Iterative Delivery with AI Augmentation
- **Short Cycles**: 1-2 week iterations with concrete, shippable outcomes
- **AI-First**: Leverage AI agents for code generation, review, and research tasks
- **Human Oversight**: Strategic decisions and quality gates require human judgment

### 2. Phased Work Breakdown Structure (WBS)
All work follows a phased WBS with explicit DAG-style dependencies:

| Phase | Focus | Key Outputs |
|-------|-------|-------------|
| Discovery | Scope, requirements | PRD, user stories |
| Design | Architecture, approach | ADRs, technical specs |
| Build | Implementation | Code, unit tests |
| Test | Validation | Integration tests, QA reports |
| Deploy | Release | Deployed artifacts, release notes |

### 3. Agent-Led Execution
- Agents own decomposition, execution, and handoffs
- Humans provide prompts and basic elicitation—not task execution
- No "human checkpoint" or "schedule external audit" tasks in plans

### 4. Quality Gates (Non-Negotiable)
All changes must pass:
- **Lint**: Zero golangci-lint errors
- **Tests**: 80%+ code coverage
- **Format**: gofmt compliance
- **Vet**: go vet clean
- **Review**: 5-10 cloud review cycles before human handoff

## Documentation Standards

### Required Artifacts by Phase

| Artifact | Location | Template |
|----------|----------|----------|
| PRD | `docs/PRD.md` | PRD_TEMPLATE.md |
| Spec | `docs/plans/*.md` | PLAN_TEMPLATE.md |
| ADR | `docs/context/*/adr/*.md` | ADR_TEMPLATE.md |
| Test Plan | `docs/context/*/tests/*.md` | TEST_TEMPLATE.md |
| Changelog | `CHANGELOG.md` | CHANGELOG_ENTRY_TEMPLATE.md |

### File Organization (Governance)

```
docs/
├── guides/              # Implementation guides
├── reports/             # Completion reports, summaries
├── research/            # Research summaries
├── reference/           # Quick references
├── checklists/          # Implementation checklists
└── plans/               # Specs and planning documents
```

**Rules:**
- Never create `.md` files in project root (except README.md, CHANGELOG.md, AGENTS.md)
- All documentation goes in `docs/` subdirectories
- Use `*_SPEC.md`, `*_GUIDE.md`, `*_QUICK_START*.md` naming conventions

## Workflow Standards

### Branch Strategy

| Branch | Purpose | Lifecycle |
|--------|---------|-----------|
| `main` | Production | Protected |
| `develop` | Integration | Protected |
| `release/*` | Pre-releases | Temporary |
| `convoy/*` | Feature convoys | Auto-merged |

### Versioning

- **Standard**: SemVer (`MAJOR.MINOR.PATCH`)
- **Pre-release**: `x.y.z-alpha.n`, `x.y.z-beta.n`, `x.y.z-rc.n`

### Commit Hygiene
- Commit after every meaningful unit (new function, passing test, config change)
- Push after every commit (disk is ephemeral)
- Descriptive commit messages referencing bead/task ID

### Review Pipeline

1. **Cloud Review**: 5-10 automated cycles per PR
2. **Stop Criteria**: All critical findings resolved, no open `request_changes`
3. **Human Handoff**: Only after stop criteria met

## Quality Enforcement

### Opinionated Rules

| Rule | Enforcement |
|------|-------------|
| No lint suppressions | CI gate |
| No TODO placeholders | golangci-lint |
| Max function length | 40 lines (golangci-lint) |
| Test coverage | 80% minimum (CI gate) |
| Documentation required | PR cannot merge without |

### Violation Handling

| Violation | Fix Strategy |
|-----------|-------------|
| `revive` (unused params) | Rename to `_paramName` |
| `goconst` (repeated strings) | Extract to named constants |
| `mnd` (magic numbers) | Extract to named constants with units |
| `gocognit` (complexity) | Extract helper functions |
| `funlen` (long functions) | Split into focused helpers |
| `gosec` (security) | Fix immediately, no exceptions |

## Multi-Actor Coordination

### Command Debouncing
High-impact commands (`make lint`, `make test`, `make quality`) MUST use `smart-command.sh` to prevent conflicts.

### Shared Services
- Use `process-compose` for orchestration
- Use `make dev-status` / `make dev-restart` for service management
- DO NOT force-kill shared resources

### Lock Files
- Active locks stored in `.process-compose/locks/`
- Check for existing locks before heavy tasks

## Integration with Kilo Gastown

AgilePlus pairs with Kilo Gastown for orchestration:
- **Gastown**: Handles agent orchestration, bead management, routing
- **AgilePlus**: Defines process, quality gates, delivery standards

### Agent Roles

| Role | Responsibility |
|------|----------------|
| Polecat | Implementation, code execution |
| Architect/PM | Planning, dependency management |
| Refinery | Code review, merge decisions |

## Anti-Patterns (Forbidden)

- ❌ "Schedule external security audit" in plans
- ❌ "Get approval from X" as a task step
- ❌ "Human checkpoint" gates
- ❌ Silent degradation or graceful failures for required deps
- ❌ v2 files (refactor instead)
- ❌ Duplicate implementations (extend existing)

## Success Metrics

| Metric | Target |
|--------|--------|
| PR cycle time | < 24h (agent-only phases) |
| Cloud review cycles | 5-10 per PR |
| Critical findings at merge | 0 |
| Test coverage | ≥ 80% |
| Lint errors | 0 |

---

**Document Version**: 1.0  
**Last Updated**: 2026-03-31  
**Owners**: Architecture Team, DevEx
