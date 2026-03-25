# Workflow Guide

## Repository Configuration

- **origin**: Your fork (push target)
- **No upstream**: This repo has no upstream source

## Git Settings

```bash
pull.rebase = false    # Use merge (not rebase)
pull.ff = only         # Fast-forward only pulls
merge.ff = false       # Create merge commits
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

1. Use worktrees for features
2. No rebase on pushed branches
3. Merge commits preserve history
4. Keep branches small and focused
