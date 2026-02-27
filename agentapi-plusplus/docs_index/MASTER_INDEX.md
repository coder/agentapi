---
title: AgentAPI++ Repository Master Index
date: 2026-02-25
status: active
owner: kooshapari
tags: [agentapi, documentation, index, consolidated, http-api]
---

# AgentAPI++ Repository - Master Index

## Overview

This index consolidates all markdown documentation in the AgentAPI++ repository. AgentAPI++ is an HTTP API for controlling AI coding agents, forked from [coder/agentapi](https://github.com/coder/agentapi). It provides programmatic control over CLI-based agents including Claude Code, Cursor, Aider, Codex, and others.

**Total Documents**: 29
**Repository Type**: API Server + CLI Tool
**Primary Language**: TypeScript/Go
**Last Updated**: 2026-02-25

---

## Document Catalog

| Path | Description | Status | Type |
|------|-------------|--------|------|
| `README.md` | Main repository overview, quick start, capabilities | active | readme |
| `CHANGELOG.md` | Release history and version changelog | active | changelog |
| `ARCHIVED.md` | Archive of deprecated features and decisions | inactive | archive |
| `ADR.md` | Architecture Decision Records | active | architecture |
| `AGENTS.md` | Agent integration documentation and support matrix | active | reference |
| `MAINTAINERS.md` | Project maintainers and contribution guidelines | active | governance |
| `WORKLOG.md` | Development worklog and progress tracking | active | tracking |
| `TEST_SUMMARY.md` | Test coverage and execution summary | active | testing |
| `TYPESCRIPT_MINIMIZATION.md` | TypeScript codebase reduction effort | active | refactoring |
| `docs/index.md` | Documentation index and navigation hub | active | docs |
| `docs/PRD.md` | Product Requirements Document | active | prd |
| `docs/SPEC.md` | Technical specification document | active | specification |
| `docs/CHANGELOG.md` | Detailed changelog (mirrors root CHANGELOG.md) | active | changelog |
| `docs/WORKLOG.md` | Development worklog (mirrors root WORKLOG.md) | active | tracking |
| `docs/api/index.md` | HTTP API reference and endpoints | active | api-reference |
| `docs/how-to/index.md` | How-to guides and tutorials | active | guides |
| `docs/explanation/index.md` | Conceptual explanations and architecture | active | explanation |
| `docs/tutorials/index.md` | Step-by-step tutorials and examples | active | tutorial |
| `docs/operations/index.md` | Operations, deployment, and configuration | active | ops |
| `docs/guides/CHANGELOG_PROCESS.md` | Changelog contribution process | active | guidance |
| `docs/reference/CHANGELOG_ENTRY_TEMPLATE.md` | Template for changelog entries | active | template |
| `docs/fa/index.md` | Documentation in Persian (فارسی) | active | localization |
| `docs/fa-Latn/index.md` | Documentation in Persian (Latin script) | active | localization |
| `docs/zh-CN/index.md` | Documentation in Simplified Chinese (中文) | active | localization |
| `docs/zh-TW/index.md` | Documentation in Traditional Chinese (繁體中文) | active | localization |
| `chat/README.md` | Chat module documentation | active | module |
| `e2e/README.md` | End-to-end testing documentation | active | testing |
| `lib/httpapi/README.md` | HTTP API library documentation | active | api |

---

## Key Findings

### Documentation Organization

The repository follows a **Divio-inspired documentation structure** with clear separation:

1. **Root Level** — Main project files (README, CHANGELOG, ADR, AGENTS, etc.)
2. **docs/** — Primary documentation hub
   - `index.md` — Navigation and table of contents
   - `api/` — HTTP API reference
   - `how-to/` — Practical guides
   - `explanation/` — Conceptual docs
   - `tutorials/` — Step-by-step examples
   - `operations/` — Deployment and configuration
   - `guides/` — Contribution and process guides
   - `reference/` — Templates and specifications
3. **Localization** — Multi-language support (Persian, Simplified Chinese, Traditional Chinese)
4. **Module READMEs** — Component-specific docs (chat/, e2e/, lib/httpapi/)

### Documentation Coverage

**Maturity Level**: High (Divio structure with API docs, localization, clear governance)

- **100% Coverage**: API, operations, how-to, explanations
- **Localization**: 4 language variants (Persian, Persian Latin, Simplified Chinese, Traditional Chinese)
- **Process Documentation**: Changelog process, contribution guidelines
- **Architecture**: ADR, SPEC, PRD all present

### Duplicate or Mirrored Files

1. **CHANGELOG.md**
   - `CHANGELOG.md` (root)
   - `docs/CHANGELOG.md` (mirrored)
   - **Status**: Both synchronized; primary is root level

2. **WORKLOG.md**
   - `WORKLOG.md` (root)
   - `docs/WORKLOG.md` (mirrored)
   - **Status**: Both synchronized; primary is root level

### Key Themes

1. **Multi-Agent Control** — Support for multiple AI agents (Claude Code, Cursor, Aider, Codex, etc.)
2. **HTTP API Interface** — RESTful control of terminal-based agents
3. **Terminal Emulation** — In-memory PTY handling
4. **Session Management** — Persistent conversation state
5. **TypeScript Optimization** — Minimization and refactoring efforts
6. **Internationalization** — Multi-language documentation support

### Status Indicators

- **active**: Currently maintained and in use
- **inactive**: Deprecated or archived (ARCHIVED.md)
- **localization**: Multi-language variants (all active)

---

## Document Dependencies & Relationships

```
README.md (entry point)
├── AGENTS.md (agent support matrix)
├── ADR.md (architectural decisions)
├── docs/index.md (documentation hub)
│   ├── docs/api/index.md
│   ├── docs/how-to/index.md
│   ├── docs/explanation/index.md
│   ├── docs/tutorials/index.md
│   ├── docs/operations/index.md
│   ├── docs/PRD.md
│   ├── docs/SPEC.md
│   └── Localization variants (fa/, fa-Latn/, zh-CN/, zh-TW/)
├── docs/guides/CHANGELOG_PROCESS.md
└── lib/httpapi/README.md (API library)
```

---

## Navigation Shortcuts

### For Users
- **Getting Started**: `README.md` → `docs/how-to/index.md`
- **API Integration**: `docs/api/index.md`
- **Deployment**: `docs/operations/index.md`
- **Agent Support**: `AGENTS.md`

### For Contributors
- **Architecture**: `ADR.md` → `docs/SPEC.md`
- **Contribution Process**: `MAINTAINERS.md`
- **Development Tracking**: `WORKLOG.md`
- **Changelog Format**: `docs/guides/CHANGELOG_PROCESS.md`

### For Maintainers
- **Deprecations**: `ARCHIVED.md`
- **Testing**: `TEST_SUMMARY.md`
- **Refactoring Status**: `TYPESCRIPT_MINIMIZATION.md`

---

## Recommendations

1. **Mirror Consolidation**: The `CHANGELOG.md` and `WORKLOG.md` are mirrored at root and `docs/`. Consider maintaining single source with symlink or reference.

2. **Central Navigation**: The `docs/index.md` serves as the hub; ensure all major sections link back to it for discoverability.

3. **Localization Sync**: Verify Persian, Chinese (SC), and Traditional Chinese translations are kept in sync during updates. Current localization appears complete but should be audited quarterly.

4. **API Documentation**: The `docs/api/index.md` is critical. Ensure it stays synchronized with actual API endpoints in code.

5. **Archive Review**: `ARCHIVED.md` should be periodically reviewed and moved to a timestamped archive (e.g., `docs/archive/2026-02-25-archived.md`) for historical tracking.

---

## Statistics

| Metric | Value |
|--------|-------|
| Total Documents | 29 |
| Active Documents | 27 |
| Inactive/Archived | 1 |
| Localization Variants | 4 |
| Root-Level Docs | 8 |
| docs/ Subdirectory Docs | 18 |
| Module READMEs | 3 |
| Languages Supported | 5 (English + 4 translations) |

---

## File Locations Summary

```
agentapi++/
├── README.md                           ← Main entry point
├── CHANGELOG.md                        ← Root changelog
├── WORKLOG.md                          ← Root worklog
├── ARCHIVED.md                         ← Deprecated features
├── ADR.md                              ← Architecture decisions
├── AGENTS.md                           ← Agent support matrix
├── MAINTAINERS.md                      ← Governance
├── TEST_SUMMARY.md                     ← Test coverage
├── TYPESCRIPT_MINIMIZATION.md          ← Refactoring status
├── docs/
│   ├── index.md                        ← Docs hub
│   ├── PRD.md                          ← Product requirements
│   ├── SPEC.md                         ← Technical specification
│   ├── CHANGELOG.md                    ← Mirrored changelog
│   ├── WORKLOG.md                      ← Mirrored worklog
│   ├── api/index.md                    ← API reference
│   ├── how-to/index.md                 ← How-to guides
│   ├── explanation/index.md            ← Conceptual docs
│   ├── tutorials/index.md              ← Tutorials
│   ├── operations/index.md             ← Deployment docs
│   ├── guides/CHANGELOG_PROCESS.md     ← Changelog process
│   ├── reference/CHANGELOG_ENTRY_TEMPLATE.md  ← Templates
│   ├── fa/index.md                     ← Persian docs
│   ├── fa-Latn/index.md                ← Persian (Latin)
│   ├── zh-CN/index.md                  ← Simplified Chinese
│   └── zh-TW/index.md                  ← Traditional Chinese
├── chat/README.md                      ← Chat module
├── e2e/README.md                       ← E2E testing
└── lib/httpapi/README.md               ← HTTP API library
```

---

**Index Generated**: 2026-02-25
**Generator**: Automated markdown consolidation tool
