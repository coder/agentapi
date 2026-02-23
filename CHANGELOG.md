# Changelog

All notable changes to this project will be documented in this file.

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
