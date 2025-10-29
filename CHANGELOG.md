# Changelog

## v0.10.2

### Features
- Improve autoscroll UX

## v0.10.1

### Features
- Visual indicator for agent name in the UI (not in embed)
- Downgrade openapi version to v3.0.3
- Add CLI installation instructions in README.md

## v0.10.0

### Features
- Feature to upload files to agentapi
- Introduced clickable links
- Added e2e tests
- Fixed the resizing scroll issue

## v0.9.0

### Features
- Add support for initial prompt via `-I` flag

## v0.8.0

### Features
- Add Support for GitHub Copilot
- Fix inconsistent openapi generation

## v0.7.1

### Fixes

- Adds headers to prevent proxies buffering SSE connections

## v0.7.0

### Features
- Add Support for Opencode.
- Add support for Agent aliases
- Explicitly support AmazonQ
- Bump NEXT.JS version

## v0.6.3

- CI fixes.

## v0.6.2

- Fix incorrect version string.

## v0.6.1

### Features
- Handle animation on Amp cli start screen.

## v0.6.0

### Features

- Adds support for Auggie CLI.

## v0.5.0

### Features

- Adds support for Cursor CLI.

## v0.4.1

### Fixes

- Sets `CGO_ENABLED=0` in build process to improve compatibility with older Linux versions.

## v0.4.0

### Breaking changes

- If you're running agentapi behind a reverse proxy, you'll now likely need to set the `--allowed-hosts` flag. See the [README](./README.md) for more details.

### New features

- Sourcegraph Amp support
- Added a new `--allowed-hosts` flag to the `server` command.

### Fixes

- Updated Codex support after its TUI has been updated in a recent version.
