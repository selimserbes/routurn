# Routurn

**Agentless remote iteration CLI for syncing changes, running tasks over SSH, and collecting results.**

Routurn is designed for development loops where the code is edited locally but the real build, test, simulation, benchmark, or training environment lives on another machine.

The remote machine does **not** need Routurn, a daemon, or a privileged service. Routurn stays on the user's machine and uses standard SSH-based workflows.

## Status

Early development. The current bootstrap implements the configuration and remote-execution foundation. Safe sync/apply, artifact collection, transactional runs, and `exec` are next.

## Core idea

```text
local project / AI update
        │
        ▼
      Routurn
        │ SSH
        ▼
 remote workstation
        │
        ├─ build / test / run / train
        │
        ▼
 logs + artifacts
        │
        ▼
      Routurn
        │
        ▼
 local developer / AI
```

## Current commands

```bash
routurn init
routurn doctor
routurn target add <name> --host <host> [--user <user>]
routurn target list
routurn target remove <name>
routurn status
routurn run <task>
routurn -p <project> run <task>
```

`routurn run` streams the remote task's stdout/stderr directly into the user's terminal and returns a non-zero exit status when the remote command fails.

## Project configuration

`routurn init` creates a `routurn.toml` in the project and registers the local project path. This lets Routurn work either from inside the repository or from any directory with `-p`.

Example:

```toml
version = 1
name = "my-project"

[remote]
target = "remote-dev"
path = "/home/user/workspace/my-project"

[sync]
exclude = [
  ".git/**",
  ".routurn/**",
  ".venv/**",
  "**/__pycache__/**",
  "node_modules/**",
  "target/**",
]

[tasks.test]
command = "go test ./..."
artifacts = []

[tasks.train]
command = "python scripts/train.py"
interactive = true
artifacts = ["outputs/**"]
```

SSH targets are user-level configuration and are intentionally kept outside the repository:

```bash
routurn target add remote-dev --host dev.example.com --user developer
```

On Linux, the global configuration is stored under `~/.config/routurn/`.

## Why Routurn?

Routurn is not tied to a language or framework. A task is simply a command on a remote project:

- Go: `go test ./...`
- Rust: `cargo test`
- Python: `pytest`
- Next.js: `npm test`
- C/C++: `cmake`, `ninja`, `ctest`
- ML/AI: training and evaluation scripts
- Robotics/simulation: ROS, Isaac Sim, Isaac Lab, Jetson workflows

## Planned v0.1 workflow

```bash
routurn exec test
```

will become the complete iteration transaction:

```text
inspect changes
→ create remote snapshot
→ sync/apply safely
→ run task with live output
→ capture exit status and logs
→ collect declared artifacts
→ save a local run manifest
```

Incoming update bundles from chat-based AI workflows will also be supported without requiring an AI agent on either machine.

## Development

```bash
go mod tidy
go test ./...
go run ./cmd/routurn version
```
