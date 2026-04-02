# Workflow Guide

## Repository Configuration

- **origin**: Your fork (push target)
- **No upstream**: This repo has no upstream source

## Git Settings

```bash
pull.rebase = false    # Do not rewrite shared history
pull.ff = only         # Fast-forward only pulls
merge.ff = false       # Preserve explicit merge commits when they are intentional
```

## Worktree Setup

```bash
# Create feature worktree
git worktree add worktrees/agentapi/feat/my-feature -b feat/my-feature

# List worktrees
git worktree list

# Prune stale
git worktree prune
```

## Branch Naming

- `feat/*` - Features
- `fix/*` - Fixes
- `chore/*` - Maintenance
- `refactor/*` - Refactoring
- `stack/*` - stacked PR chain roots
- `layer/*` - stacked PR chain layers
- `preview/*` - preview or release-lane branches

## Protected Branch Policy

- `main` and `master` must not be force-pushed.
- Shared protected branches require pull requests, resolved review threads, and
  green non-billing CI before merge.
- `CHANGES_REQUESTED` and unresolved review comments block merge.
- Billing-only CI exceptions must be explicitly documented in the PR body and
  labeled before they can be accepted.

## Daily Workflow

```bash
git worktree add worktrees/agentapi/feat/my-feature -b feat/my-feature
cd worktrees/agentapi/feat/my-feature
# ... changes ...
git add . && git commit -m "feat: my feature"
git push origin feat/my-feature
gh pr create --base main --title "feat: my feature"
```

## Sync Rules

1. Use worktrees for features.
2. Prefer stacked PRs for multi-step work.
3. Do not force-push protected branches.
4. Do not merge with unresolved comments or `CHANGES_REQUESTED`.
5. Keep branches small and focused.
