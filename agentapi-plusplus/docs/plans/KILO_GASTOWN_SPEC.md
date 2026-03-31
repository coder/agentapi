# Kilo Gastown Methodology Specification

## Overview

Kilo Gastown is the orchestration layer for agent-driven development within the Kilo ecosystem. It provides the infrastructure for agent coordination, work item management, and multi-repo synchronization. Gastown handles the mechanics of agent orchestration while the process methodology (AgilePlus) defines quality gates and delivery standards.

## Core Concepts

### 1. Beads (Work Items)

Beads are the fundamental work unit in Gastown. Each bead represents a discrete task, issue, or convoy tracked within a rig.

| Bead Type | Purpose | Lifecycle |
|-----------|---------|-----------|
| `issue` | Single task or bug | open → in_progress → in_review → closed |
| `convoy` | Multi-repo feature coordinated across repos | open → in_progress → merged → closed |
| `triage` | Intake queue for new requests | open → resolved → closed |

### 2. Bead Lifecycle

```
┌─────────┐    ┌──────────────┐    ┌─────────────┐    ┌────────┐
│  open   │───▶│ in_progress  │───▶│  in_review  │───▶│ closed │
└─────────┘    └──────────────┘    └─────────────┘    └────────┘
                    │                    │
                    ▼                    ▼
              ┌──────────┐         ┌──────────┐
              │ escalated │         │ rework   │
              └──────────┘         └──────────┘
```

**State Transitions:**
- `open` → `in_progress`: Agent picks up the bead
- `in_progress` → `in_review`: Agent completes work and calls `gt_done`
- `in_review` → `closed`: Refinery approves and merges
- `in_review` → `rework`: Refinery requests changes (bead returns to `in_progress`)
- Any state → `escalated`: Agent cannot resolve; requires human intervention

### 3. Convoys

Convoys are meta-beads that coordinate work across multiple repositories. They ensure related changes in different repos are developed in parallel and merged together.

| Convoy Property | Description |
|-----------------|-------------|
| `feature_branch` | Shared branch name across all participant repos |
| `ready_to_land` | Flag indicating convoy is ready for merge |
| `participant_beads` | Child beads in each repo belonging to the convoy |

**Convoy Workflow:**
1. Create convoy bead with shared `feature_branch` pattern: `convoy/<name>/<convoy_id>/head`
2. Create child beads in each repo with `convoy_id` metadata
3. All participant agents work on their respective repos using the shared branch
4. When all beads in convoy are `ready_to_land`, convoy merges atomically

### 4. Delegation: gt_sling and gt_sling_batch

**gt_sling** sends a single bead to another agent for execution.

**gt_sling_batch** distributes multiple related beads to sub-agents in parallel.

| Command | Use Case | Behavior |
|---------|----------|----------|
| `gt_sling` | Single bead delegation | Transfers one bead to another agent |
| `gt_sling_batch` | Mass delegation | Distributes N beads to parallel workers |

**Batch Delegation Pattern:**
```
gt_sling_batch --bead-ids=<id1,id2,id3> --agent-type=polecat --parallel=3
```

### 5. Merge Modes

Gastown supports different merge strategies for convoys:

| Mode | Description | Use When |
|------|-------------|----------|
| `auto-merge` | Automatically merges when all participants ready | Hotfixes, synchronous releases |
| `staged` | Merges go to `develop` first | Feature development |
| `rolling` | Continuous merge to `main` | Canary releases |

### 6. Progress Tracking: gt_list_convoys

The `gt_list_convoys` command provides visibility into convoy status:

```bash
gt_list_convoys --rig=<rig_id>
```

**Output Fields:**
| Field | Description |
|-------|-------------|
| `convoy_id` | Unique identifier |
| `title` | Convoy description |
| `status` | open / in_progress / merged |
| `participant_repos` | List of repos with active beads |
| `ready_to_land` | Boolean merge readiness |
| `blocked_by` | Upstream blocking dependencies |

## Gastown Architecture

### Rig Structure

```
Town (78a8d430-a206-4a25-96c0-5cd9f5caf984)
└── Rig (81bdd033-d575-4597-b098-f3a06d0fd7f7)
    ├── Agent: Polecat-40 (polecat role)
    ├── Agent: Refinery (merge gate)
    ├── Worktree: gt__polecat-40__<bead_id>
    └── Convoys: [bead_id_1, bead_id_2, ...]
```

### Worktree Isolation

Each agent operates in an isolated worktree:
```
worktrees/gt__polecat-40__<bead_id>/
```

Benefits:
- Agent can make commits without affecting `main` or other agents
- Branch is automatically created and tracked
- Push triggers Refinery review pipeline

## Integration with AgilePlus

Gastown handles orchestration; AgilePlus defines process:

| Concern | Gastown | AgilePlus |
|---------|---------|-----------|
| Work tracking | Beads, convoys | User stories, PRDs |
| Agent routing | gt_sling, gt_sling_batch | Task decomposition |
| Branch strategy | Convoy branches | `release/*`, `develop` |
| Quality gates | Enforcement via Refinery | Lint, tests, coverage |
| Progress visibility | gt_list_convoys | Sprint boards |

### Agent Roles in Gastown

| Role | Type | Responsibilities |
|------|------|------------------|
| Polecat | Implementation | Executes bead work, writes code |
| Architect/PM | Planning | Creates beads, manages convoys |
| Refinery | Merge gate | Reviews, approves, merges |

## Workflow for agentapi-plusplus

### Starting a New Feature (Convoy)

1. **Architect creates convoy bead**
   ```bash
   gt_bead_create --type=convoy --title="Feature: X" --rig=<rig_id>
   ```

2. **Architect creates participant beads** in each repo with `convoy_id` metadata

3. **Polecat agents pick up beads** via `gt_prime` (auto-injected context)

4. **Agents work in isolation** using worktree branches

5. **Push and gt_done** triggers Refinery review

6. **Convoy merges** when all participants are `ready_to_land`

### Bead Lifecycle Example

```
Polecat-40 receives bead 252ac72c (this bead)
  │
  ├─▶ worktree: gt__polecat-40__252ac72c
  │
  ├─▶ Implement changes
  │
  ├─▶ git add . && git commit -m "feat: add Kilo Gastown spec"
  │
  ├─▶ git push origin gt__polecat-40__252ac72c
  │
  └─▶ gt_done --branch=gt__polecat-40__252ac72c
         │
         ├─▶ Bead status → in_review
         └─▶ Refinery picks up for merge review
```

## Anti-Patterns (Forbidden)

- ❌ **Silent degradation**: Required dependencies must fail loudly, not gracefully
- ❌ **Human checkpoints in plans**: Agents own execution, not humans
- ❌ **Cross-rig coordination without convoy**: Individual bead work across repos must use convoys
- ❌ **Bypass Refinery**: Never merge without going through the review pipeline
- ❌ **Shared branch per agent**: Use worktree isolation, not shared dev branches

## Success Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| Bead cycle time | < 4h | `gt_bead_status` timestamps |
| Convoy merge time | < 24h | `gt_list_convoys` status |
| Refinery turnaround | < 1h | Review request to merge |
| Escalation rate | < 5% | Escalated beads / total |
| Retry rate | < 10% | Rework beads / total |

## Key Commands Reference

| Command | Purpose |
|---------|---------|
| `gt_prime` | Get context: identity, hooked bead, mail, open beads |
| `gt_done` | Complete bead, push branch, transition to `in_review` |
| `gt_bead_status <id>` | Inspect bead state and metadata |
| `gt_bead_close <id>` | Mark bead as completed |
| `gt_list_convoys` | List all convoys with progress |
| `gt_sling <bead_id> --to=<agent>` | Delegate single bead |
| `gt_sling_batch --beads=<ids>` | Delegate multiple beads |
| `gt_escalate` | Create escalation for blocked work |
| `gt_checkpoint --data=<json>` | Save crash-recovery state |
| `gt_mail_send --to=<agent>` | Inter-agent coordination |
| `gt_status --message=<text>` | Dashboard status update |

---

**Document Version**: 1.0  
**Last Updated**: 2026-03-31  
**Owners**: Platform Team, DevEx  
**Related**: [AGILEPLUS_SPEC.md](./AGILEPLUS_SPEC.md)
