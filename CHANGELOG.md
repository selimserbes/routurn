# Changelog

All notable changes to Routurn will be documented in this file.

The project follows Semantic Versioning.

## [Unreleased]

## [0.4.0] - 2026-10-09

### Added
- Explicit SSH-free project execution via `[execution].mode = "local"` and `routurn init --local`, with local task output, run history, and artifact/result collection.
- OpenSSH `--jump` support for single or chained bastion routes, shared consistently across remote commands, synchronization, and artifact transfer; separate SSH multiplex pools for direct and jumped routes.
- Interactive registered-project selection for bare `exec`, `update`, and `result` outside project directories, without silently changing persistent defaults.
- Explicit project selection by directory path (`-p ./path`, absolute paths, or `~/path`) and `routurn project check` for offline configuration checks.
- Registration conflict protection requiring explicit `project add --replace` before reassigning an existing project name.

### Changed
- A missing `[execution].mode` continues to use remote SSH execution for compatibility with v0.3.0.
- Local `sync` is a no-op; `--detach` remains available for remote tasks but is not supported for local-mode tasks.

### Fixed
- Local artifact glob collection ignores Routurn-owned published result directories, preventing collection of prior results into new results; user-owned `results/` directories remain eligible.

## [0.3.0] - 2026-10-09

### Added
- Unified interactive terminal selection for discovered/saved tasks (`routurn exec`), update archives (`routurn update`), and available artifact results (`routurn result` without a task name).
- Arrow-key navigation, live `/` search, numeric task highlighting (`0` selects item 10), explicit Enter confirmation, Esc cancellation, and an in-place custom-command or path editor.
- Optional project-defined picker groups and recent-task ordering from run history; no special `stage` naming convention is required.
- Delayed loading feedback and opt-in startup timing diagnostics with `ROUTURN_STARTUP_TIMING=1`.

### Changed
- Remote endpoint probing for interactive `exec` happens after task selection, so opening and exiting the task picker need not wait for SSH routing.
- Interactive selection is optional: named tasks, direct `routurn exec -- <command>`, explicit update paths, and named result lookups remain available to scripts and advanced users.
- Refreshed installation and command documentation for v0.3.0 while preserving prior version history.

### Fixed
- Terminal redraw alignment while handling raw keyboard input, and robust handling of F1–F12/Insert and other unassigned escape sequences.
- Custom-command/path entry cancellation returns to the same task/update list without accidentally submitting input; clearer persistent selection feedback and readable no-color output.

## [0.2.0] - 2026-10-08
### Added
- Project entrypoint discovery now produces stable interpreter-based commands across Linux, macOS, and Windows local clients instead of depending on local executable permission bits.
- Native platform smoke and release builds use Go 1.26.8 for current macOS compatibility while the module retains Go 1.22 as its minimum language version.

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

[Unreleased]: https://github.com/selimserbes/routurn/compare/v0.4.0...HEAD
[0.4.0]: https://github.com/selimserbes/routurn/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/selimserbes/routurn/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/selimserbes/routurn/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/selimserbes/routurn/releases/tag/v0.1.0
