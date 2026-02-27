# Changelog Process

This repository uses [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) with semantic versioning.

## Rules

1. Add all pending user-facing changes under `## [Unreleased]` in `CHANGELOG.md`.
2. Use standard sections when applicable: `Added`, `Changed`, `Deprecated`, `Removed`, `Fixed`, `Security`.
3. Keep entries concise and action-oriented.
4. Do not create a release section until the release is cut.

## Release Cut Workflow

1. Move finalized entries from `## [Unreleased]` into a new version section like `## [1.2.3]`.
2. Keep the `## [Unreleased]` section at the top for subsequent work.
3. Ensure `CHANGELOG.md` still includes the Keep a Changelog intro block.

## PR Guidance

1. Every PR with user-visible changes should include a changelog update.
2. Non-user-facing internal changes may be omitted.
