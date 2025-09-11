# Changelog

## 0.7.0

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
