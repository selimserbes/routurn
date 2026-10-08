# Contributing to Routurn

Thanks for helping improve Routurn.

## Development setup

Requirements:

- Go version declared in `go.mod`
- OpenSSH client for manual integration testing

Run the local quality gate before opening a pull request:

```bash
gofmt -w .
go vet ./...
go test ./...
go build ./cmd/routurn
sh -n install.sh
sh -n scripts/release-notes.sh
```

## Pull requests

Keep changes focused and include tests for behavior that can be tested without a real SSH host. For remote behavior, describe the target setup and the commands used to validate it.

Public examples must stay generic. Use values such as:

```text
remote-dev
dev.example.com
developer
example-project
```

Do not commit real usernames, hostnames, IP addresses, SSH keys, tokens, `.env` files, or private project paths.

## CLI conventions

Routurn follows these conventions:

```text
-h, --help       help
help             help command
-V, --version    version
-v, --verbose    detailed diagnostics
```

New commands should provide concise `Use`, `Short`, and helpful error messages, return non-zero exit codes on failure, and preserve script-friendly behavior.

## Release process

Routurn uses Semantic Versioning. Release notes are generated from `CHANGELOG.md`; Git commit messages are not used as the release body.

Before creating a release tag:

1. Move the relevant entries from `[Unreleased]` into a versioned section such as `## [0.3.0] - YYYY-MM-DD`.
2. Run the normal quality gate.
3. Preview the release body with `sh scripts/release-notes.sh v0.3.0`.
4. Commit and push the changelog update and wait for CI to pass.
5. Create an annotated tag such as `git tag -a v0.3.0 -m "Routurn v0.3.0"` and push it.

The tag-driven release workflow validates the tag and changelog, rebuilds and tests Routurn, creates cross-platform archives, verifies the embedded version, generates SHA-256 checksums, and publishes the GitHub Release using the changelog-derived notes.

Tags containing a suffix such as `v0.2.0-rc.1` are published as prereleases. Stable tags are marked as the latest release.

## Update bundle fixtures

New user-facing update bundles should include a root-level `routurn-bundle.toml` when the project identity and base state are known:

```toml
schema = 1

[bundle]
name = "example-fix"

[project]
name = "example-project"

[base]
fingerprint = "sha256:..."
```

The manifest is metadata and must not be applied into the project tree. Tests that change update intake behavior should cover project mismatch, base-state mismatch, SHA-256 deduplication, source-consumption safety, and legacy bundles without a manifest.
