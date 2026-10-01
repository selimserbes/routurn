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
