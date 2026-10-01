# Security

Routurn is designed to move project changes to SSH-accessible machines and execute configured commands there. That makes predictable trust boundaries important.

## Supported versions

Before the first stable release, security fixes are applied to the latest release only.

## Reporting a vulnerability

Please do not open a public issue for a vulnerability that could lead to arbitrary file writes, path traversal, command injection, credential exposure, or unsafe remote execution.

Use GitHub's private vulnerability reporting for this repository when available. If private reporting is unavailable, contact the maintainer privately before publishing technical details.

## Security model

Routurn is local-only and agentless on remote targets. It relies on the user's existing SSH trust and permissions.

Routurn intentionally:

- rejects archive path traversal and protected `.git/` / `.routurn/` writes;
- rejects archive symlinks and special files during update application;
- rejects writes through symlinked local parent directories;
- snapshots tracked remote files before overwriting or deleting them;
- identifies managed update bundles by SHA-256 rather than filenames;
- removes an original selected update archive only after a verified managed copy is safely registered, and never performs broad cleanup of unrelated user files;
- never installs a daemon or privileged service on the remote machine;
- does not require `sudo` for normal operation.

Routurn does **not** sandbox the commands configured in `routurn.toml`. A task runs with the permissions of the configured SSH user. Review project configuration and update bundles before running code from untrusted sources.
