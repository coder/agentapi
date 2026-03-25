# Changelog

## v0.11.9

### Features
- Integrate shared phenotype-go-kit packages (#328, #329)
- Modernize tooling with oxc linter and bun (#330)
- Add pagination support for messages endpoint (#335)

### Fixes
- Resolve PR303 contract and gate blockers (#333)
- Stabilize policy federation and docs CI (#336)
- Normalize OpenAPI contract hygiene (#334)
- Fix TypeScript lint issues (#332)

## v0.11.8

### Fix
- Update message box formatting detection for Claude

## v0.11.7

### Features
- format codex messages to skip the coder_report_task tool call

## v0.11.6

### Features
- Bump Next.js to 15.4.10

## v0.11.5

### Features
- Add tool call logging.
- Improve parsing/detection of tool call messages.

## v0.11.4

### Features
- Temporarily remove coder report_task tool-call logs

## v0.11.3

### Features
- format claude messages to skip the coder_report_task tool call

## v0.11.2

### Features
- Improved handling of initial prompt

## v0.11.1

### Features
- Add tooltips for buttons
- Autofocus message box on user's turn
- Add msgfmt logic for amp module
- Update msgfmt for latest version in opencode

## v0.11.0

### Features
- Support sending initial prompt via stdin

## v0.10.2

### Features
- Improve autoscroll UX

## v0.10.1

### Features
- Visual indicator for agent name in the UI (not in embed)
- Downgrade openapi version to v3.0.3
- Add CLI installation instructions in README.md

## v0.10.0

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

### Changed

### Fixed

## [0.10.0]

### Added
- Feature to upload files to agentapi
- Introduced clickable links
- Added e2e tests

### Fixed
- Fixed the resizing scroll issue

## [0.9.0]

### Added
- Add support for initial prompt via `-I` flag

## [0.8.0]

### Added
- Add support for GitHub Copilot

### Fixed
- Fix inconsistent OpenAPI generation

## [0.7.1]

### Fixed
- Adds headers to prevent proxies buffering SSE connections

## [0.7.0]

### Added
- Add support for Opencode
- Add support for agent aliases
- Explicitly support AmazonQ

### Changed
- Bump Next.js version

## [0.6.3]

### Fixed
- CI fixes

## [0.6.2]

### Fixed
- Fix incorrect version string

## [0.6.1]

### Added
- Handle animation on Amp CLI start screen

## [0.6.0]

### Added
- Add support for Auggie CLI

## [0.5.0]

### Added
- Add support for Cursor CLI

## [0.4.1]

### Fixed
- Set `CGO_ENABLED=0` in build process to improve compatibility with older Linux versions

## [0.4.0]

### Added
- Sourcegraph Amp support
- Added a new `--allowed-hosts` flag to the `server` command

### Changed
- If running agentapi behind a reverse proxy, set the `--allowed-hosts` flag. See [README](./README.md) for details.

### Fixed
- Updated Codex support after its TUI was updated in a recent version
