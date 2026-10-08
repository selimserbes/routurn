# Changelog

All notable changes to Routurn will be documented in this file.

The project follows Semantic Versioning.

## [Unreleased]


## [0.2.0] - 2026-10-08
### Added

- Native macOS and Windows CI smoke coverage for local-client test/build validation.
- Command-first remote execution: `routurn exec` discovers runnable project commands, while `routurn exec -- <command>` works for any language/framework without predefined tasks.
- Initial ecosystem discovery for Go, Rust, Node/web, Python, Make, Docker Compose, and executable text entrypoints without assuming a `scripts/` directory.
- Cleaner command picker with saved-task/project-entrypoint deduplication, hidden maintenance helpers, shebang/conventional entrypoint discovery, and a `Show all detected commands` fallback.
- Routurn-managed local task shortcuts with `routurn task save/list/remove` stored under `.routurn/tasks.toml`.
- Interactive discovery of ordinary AI/developer archives without Routurn metadata, labeled as generic review-required updates rather than hidden.
- `routurn bundle inspect` for validating and inspecting update archives without modifying a project.
- Optional `.7z` and `.rar` update support through a validated local 7-Zip-compatible importer (`7zz`, `7z`, or `7za`).
- `routurn update` with terminal-based update selection, direct path input, permissive recent-bundle discovery, recommended/other grouping, and separate state/identity labels.
- Routurn bundle manifests (`routurn-bundle.toml`) for project identity, optional whole-project fingerprints, and scoped target-file preconditions.
- `routurn bundle fingerprint` for generating the current project-state identity used by compatible update bundles.
- Content-addressed managed update storage with SHA-256 deduplication and verified source consumption.
- Stable latest successful task results under `.routurn/results/<task>/` plus `routurn result <task>`.
- Short user-facing task result views under `results/<task>/` (or collision-safe `routurn-results/<task>/`), including `latest.<ext>` for single-artifact tasks and `routurn result <task> --path`.
- Human-friendly task selectors for status, logs, stop, and fetch workflows so internal run IDs are normally optional.
- `routurn updates`, `routurn rollback previous`, and `routurn clean --dry-run`.
- Automatic bounded retention for local run history, update backups, managed update bundles, remote sync snapshots, and completed detached-run state.
- Logical SSH targets with multiple named endpoints for alternate routes to the same remote machine.
- Automatic endpoint failover with priority ordering and short-lived last-success routing cache.
- Persistent interactive route selection with `routurn target route`, one-command `--endpoint` overrides, endpoint management, target merging, and `routurn target test`.
- `routurn status --check` for endpoint reachability and selected-route reporting.

### Security

- Reject control characters, Windows drive/ADS/reserved-name paths, and Windows trailing-dot/space aliases before applying updates.
- External archive import streams file contents through stdout and rejects links, special files, encrypted entries, duplicate normalized paths, and size-limit violations before project writes.
- Automatic `--recent` selection requires a matching manifest project name plus either an exact whole-project fingerprint or complete matching target-file preconditions. Interactive discovery does not hide bundles solely because a human-readable project name is wrong or missing; mismatched identities are shown explicitly and can proceed only when base-state verification independently proves safety.
- Source update archives are removed only after a verified managed copy is safely registered; unrelated files are never cleaned from user directories.
- Automatic SSH failover is restricted to endpoints explicitly grouped under the same logical target; Routurn never guesses that an unrelated target is a safe substitute.

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

[Unreleased]: https://github.com/selimserbes/routurn/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/selimserbes/routurn/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/selimserbes/routurn/releases/tag/v0.1.0
