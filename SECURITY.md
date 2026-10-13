# Security

Routurn is designed to move project changes to SSH-accessible machines and execute configured commands there. That makes predictable trust boundaries important.

## Supported versions

Until Routurn reaches 1.0.0, security fixes are applied to the latest published 0.x minor line only. Older 0.x minor lines are not supported once a newer minor release is available.

## Reporting a vulnerability

Please do not open a public issue for a vulnerability that could lead to arbitrary file writes, path traversal, command injection, credential exposure, or unsafe remote execution.

Use GitHub's private vulnerability reporting for this repository when available. If private reporting is unavailable, contact the maintainer privately before publishing technical details.

## Security model

Routurn is local-only and agentless on remote targets. It relies on the user's existing SSH trust and permissions.

Routurn intentionally:

- rejects archive path traversal and protected `.git/` / `.routurn/` writes;
- rejects Windows drive/ADS/reserved-name paths and trailing-dot/space aliases before update writes;
- rejects archive symlinks, special files, encrypted external-archive entries, duplicate normalized paths, and configured size-limit violations;
- rejects writes through symlinked local parent directories;
- requires exact or scoped base-state verification before automatic `--recent` update selection;
- snapshots tracked remote files before overwriting or deleting them;
- identifies managed update bundles by SHA-256 rather than filenames;
- removes an original selected update archive only after a verified managed copy is safely registered, and never performs broad cleanup of unrelated user files;
- restricts automatic SSH failover to endpoints explicitly grouped under the same logical target;
- never installs a daemon or privileged service on the remote machine;
- does not require `sudo` for normal operation.

Routurn does **not** sandbox commands it executes. Configured tasks, local saved shortcuts, discovered project commands, and arbitrary `routurn exec -- <command>` invocations run with the permissions of the configured SSH user. Review project configuration, selected commands, and update bundles before running code from untrusted sources.
