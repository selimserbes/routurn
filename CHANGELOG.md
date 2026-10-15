# Changelog

All notable changes to Routurn will be documented in this file.
The project follows Semantic Versioning.

## [Unreleased]

## [1.2.2] - 2026-10-15

### Added
- User-level completion installation for Bash, Zsh, Fish, and PowerShell.
- Automatic completion setup in supported release installations.
- Dynamic suggestions for targets, projects, tasks, and endpoints.

### Improved
- Standard shell integration and repeatable completion installation.
- Local-only completion without remote connections or task execution.


## [1.2.1] - 2026-10-14

### Added
- Guided update-type selection for ZIP archives whose completeness cannot be verified.
- Safe non-deleting partial update as the default interactive choice.
- Complete-source review with a preview of obsolete managed-file removals.

### Improved
- Preserve automatic handling of verified complete-source bundles.
- Require deliberate selection and separate confirmation before applying changes that remove files.
- Preserve existing noninteractive update behavior and rollback support.

## [1.2.0] - 2026-10-14

### Added
- Guided home menu for task execution, project updates, results, and rollback.
- Automatic recognition of verified, compatible complete-source update bundles.
- Optional review of compatible updates before task execution.
- Interactive rollback selection without requiring an update ID.

### Improved
- Show successful and failed task history in the results menu, including runs without collected artifacts.
- Preserve safe non-deleting behavior for archives whose complete-source status cannot be verified.

## [1.1.0] - 2026-10-14

### Added
- Opt-in `--full-snapshot` for complete update archives: track managed source
  files, preview obsolete files, safely remove only unchanged previously
  managed files, and restore removed files with rollback. Partial archives
  keep the original non-deleting behavior.

### Fixed
- Select the most recent update using precise application timestamps for reliable rollback ordering. Reject ambiguous legacy update ordering instead of choosing an arbitrary rollback target.

## [1.0.0] - 2026-10-13

### Features
- Agentless command execution in local projects and on Unix-like hosts over SSH, with ProxyJump support.
- Project registry and selection by name or path; interactive discovery and reusable task shortcuts.
- Command discovery for common development ecosystems and direct execution of arbitrary commands.
- Remote synchronization, endpoint routing and failover, live output, and detached remote tasks.
- Result and artifact collection, execution history, and bounded retention of managed data.
- ZIP/TAR and optional 7-Zip-compatible update archive import with review, dry-run, rollback, and source safeguards.
- Evidence-based detection of nested archive roots and fail-closed handling of ambiguous updates.
- Linux, macOS and Windows client builds, checksums, and shell completion support.

[Unreleased]: https://github.com/selimserbes/routurn/compare/v1.2.2...HEAD
[1.2.2]: https://github.com/selimserbes/routurn/compare/v1.2.1...v1.2.2
[1.2.1]: https://github.com/selimserbes/routurn/compare/v1.2.0...v1.2.1
[1.2.0]: https://github.com/selimserbes/routurn/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/selimserbes/routurn/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/selimserbes/routurn/releases/tag/v1.0.0
