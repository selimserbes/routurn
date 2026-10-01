# Changelog

All notable changes to Routurn will be documented in this file.

The project follows Semantic Versioning.

## [Unreleased]

### Added

- `routurn bundle inspect` for validating and inspecting update archives without modifying a project.
- Optional `.7z` and `.rar` update support through a validated local 7-Zip-compatible importer (`7zz`, `7z`, or `7za`).

### Security

- Reject control characters, Windows drive/ADS/reserved-name paths, and Windows trailing-dot/space aliases before applying updates.
- External archive import streams file contents through stdout and rejects links, special files, encrypted entries, duplicate normalized paths, and size-limit violations before project writes.

## [0.1.0] - 2026-10-01

### Added

- Agentless SSH target management.
- Project registry for running commands from any local directory.
- Content-aware project synchronization with remote snapshots.
- Live remote task execution and artifact collection.
- Detached long-running tasks with status, logs, and stop controls.
- Safe AI/chat update archive application with rollback.
- ZIP, TAR, TAR.GZ, and TGZ update bundle support.
- SSH connection reuse for multi-step workflows.
- Shell completion generation for bash, zsh, fish, and PowerShell.
- CI and tag-driven cross-platform GitHub release automation.

[Unreleased]: https://github.com/selimserbes/routurn/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/selimserbes/routurn/releases/tag/v0.1.0
