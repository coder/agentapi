# agentapi-plusplus Ruleset Baseline

GitHub rulesets for protected branches should enforce:

- pull requests required before merge
- no force push
- no branch deletion
- linear history
- CODEOWNERS review
- conversation resolution before merge
- required status checks must pass
- stale approvals dismissed when new commits land

## Branch Policy

- Stacked PRs are preferred for multi-step changes.
- `stack/*`, `layer/*`, and `preview/*` branches are first-class.
- Direct `feat/*`, `fix/*`, `chore/*`, and `refactor/*` branches to `main`
  should be rare and should carry a documented exception in the PR body.
- Merge commits on active PR branches should be avoided.

## Required Checks

The repo already exposes these PR-facing checks and gates:

- `policy-gate`
- `quality-gate`
- `self-merge-gate`
- `security-guard`
- `lint-test`
- `CI`
- `Go Tests`

Protected-branch rules should require the non-billing checks to pass and should
block merge when review comments or `CHANGES_REQUESTED` remain unresolved.

## Billing Exception

Only billing-only CI failures may be exempted, and only when:

- the PR body explicitly documents the failure
- the PR is labeled for the exception
- all other checks are green
- review threads are resolved

## Direct Main Exception

Direct-to-`main` feature PRs are allowed only when:

- the PR body documents the base branch rationale in `## Stack Topology`
- all other governance requirements still pass
