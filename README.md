# Routurn

**Agentless remote iteration CLI for syncing changes, running tasks over SSH, and collecting results.**

Routurn is for development loops where code is edited locally but the real build, test, simulation, benchmark, training, or runtime environment lives on another machine.

The remote machine does **not** need Routurn, a daemon, or a privileged service. Routurn stays on the user's machine and works over standard SSH using common remote tools (`sh`, `tar`, and `find`).

## Core loop

```text
local project / AI update
        │
        ▼
      Routurn
        │ SSH
        ▼
 remote development target
        │
        ├─ build / test / run / train
        │
        ▼
 live terminal output + artifacts
        │
        ▼
      Routurn
        │
        ▼
 local developer / AI
```

Routurn is language- and framework-independent. A remote task is simply a configured command.

## Current commands

```bash
routurn init
routurn doctor

routurn target add <name> --host <host> [--user <user>]
routurn target list
routurn target remove <name>

routurn status
routurn sync [--dry-run]
routurn run <task>
routurn fetch <task>
routurn exec <task>
routurn runs
routurn runs show <run-id|latest> [--json]
```

All project commands can also be used outside the project directory with a registered project name:

```bash
routurn -p example-project exec test
```

## Project configuration

`routurn init` creates `routurn.toml` and registers the local project path in the user's Routurn config.

Example:

```toml
version = 1
name = "example-project"

[remote]
target = "remote-dev"
path = "/home/developer/projects/example-project"

[sync]
exclude = [
  ".git/**",
  ".routurn/**",
  ".venv/**",
  "venv/**",
  "**/__pycache__/**",
  "node_modules/**",
  "target/**",
]

[tasks.test]
command = "go test ./..."
artifacts = ["reports/**"]

[tasks.train]
command = "python scripts/train.py"
interactive = true
artifacts = ["outputs/**"]
```

SSH targets are user-level configuration and stay outside the repository:

```bash
routurn target add remote-dev \
  --host dev.example.com \
  --user developer
```

`dev.example.com` is documentation-only. In real use, `--host` can be a hostname, IP address, or an alias from `~/.ssh/config`.

## Sync safety

Routurn does not blindly mirror and delete an entire remote directory.

It maintains a local content-hash manifest under `.routurn/` and synchronizes only files it tracks. Before overwriting or deleting tracked remote files, Routurn creates a project-local remote snapshot:

```text
<remote-project>/.routurn/snapshots/<run-id>.tar
```

`.git/` and `.routurn/` are always excluded from project synchronization.

Preview a sync without changing the remote target:

```bash
routurn sync --dry-run
```

Then apply it:

```bash
routurn sync
```

## Live remote execution

```bash
routurn run test
```

Remote stdout and stderr are streamed live into the local terminal. Routurn also records them locally for the run.

A failed remote process produces a failed Routurn command and a non-zero exit status.

## One-command iteration

```bash
routurn exec test
```

`exec` performs the main Routurn loop:

```text
scan local project
→ calculate changed/deleted tracked files
→ create remote snapshot
→ sync changes over SSH
→ run task with live terminal output
→ collect declared artifacts
→ save run logs + manifest locally
```

Local run data is stored under:

```text
.routurn/runs/<run-id>/
├── run.json
├── stdout.log
├── stderr.log
└── artifacts/
```

Inspect recent runs:

```bash
routurn runs
routurn runs show latest
routurn runs show latest --json
```

## Artifact collection

Tasks declare the files Routurn should bring back:

```toml
[tasks.benchmark]
command = "./scripts/benchmark.sh"
artifacts = [
  "reports/**",
  "results/*.json",
]
```

Run and collect in one operation:

```bash
routurn exec benchmark
```

Or collect from the current remote project separately:

```bash
routurn fetch benchmark
```

## Works with any stack

Examples include:

- Go: `go test ./...`
- Rust: `cargo test`
- Python: `pytest`
- Next.js / Node.js: `npm test`
- C / C++: `cmake`, `ninja`, `ctest`
- ML / AI: training and evaluation scripts
- Robotics / simulation: ROS, Isaac Sim, Isaac Lab, Jetson workflows
- GUI applications launched on a remote workstation

Routurn does not need to understand the programming language. It only needs a project, an SSH target, and the commands you define.

## Requirements

Local machine:

- Routurn
- OpenSSH client (`ssh`)

Remote machine:

- SSH access
- `sh`
- `tar`
- `find`
- the project's own runtime/toolchain

No Routurn binary is installed on the remote machine.

## Development

```bash
go mod tidy
go test ./...
go run ./cmd/routurn version
```

## Roadmap

The next v0.1 pieces include:

- safe incoming update bundles (`routurn apply`)
- rollback from remote snapshots
- detached long-running tasks and log reattachment
- structured `--json` output for AI/automation workflows
- release packaging and install helpers
