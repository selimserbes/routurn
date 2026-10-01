# Routurn

**Agentless remote iteration CLI for syncing changes, running tasks over SSH, and collecting results.**

Routurn is for development loops where code is edited locally but the real build, test, simulation, benchmark, training, or runtime environment lives on another machine.

The remote machine does **not** need Routurn, a daemon, or a privileged service. Routurn stays on the user's machine and works over standard SSH using common remote tools (`sh`, `tar`, and `find`).


## Install

Routurn is distributed as a single local binary. Nothing is installed on remote targets.

Linux and macOS:

```bash
curl -fsSL https://raw.githubusercontent.com/selimserbes/routurn/main/install.sh | sh
```

The installer downloads the matching GitHub Release asset, verifies its SHA-256 checksum, and installs `routurn` to `~/.local/bin` by default. Override the destination with `ROUTURN_INSTALL_DIR`.

A specific release can be installed with:

```bash
curl -fsSL https://raw.githubusercontent.com/selimserbes/routurn/main/install.sh | ROUTURN_VERSION=v0.1.0 sh
```

Go users can also install from source:

```bash
go install github.com/selimserbes/routurn/cmd/routurn@latest
```

Release binaries are produced for Linux, macOS, and Windows on amd64 and arm64.

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

routurn project add [path] [--name <name>]
routurn project list
routurn project show <name>
routurn project remove <name>

routurn status [run-id|latest]
routurn apply <update-archive> [--dry-run] [-y]
routurn rollback <update-id|latest>
routurn sync [--dry-run]
routurn run <task> [--detach]
routurn logs <run-id|latest> [--follow]
routurn stop <run-id|latest> [--force]
routurn fetch [task|run-id|latest]
routurn exec <task> [update-archive] [--detach]
routurn runs
routurn runs show <run-id|latest> [--json]

routurn version
routurn --version
routurn -V
```

CLI conventions:

```text
-h, --help     help
help           help command
-V, --version  version
-v, --verbose  detailed diagnostics
```

All project commands can also be used outside the project directory with a registered project name:

```bash
routurn -p example-project exec test
```


## Project registry

`routurn init` automatically registers the project locally. The registry lets project commands run from any directory:

```bash
routurn -p example-project exec test
```

Manage the registry explicitly when moving or cloning a project:

```bash
routurn project add /path/to/example-project
routurn project list
routurn project show example-project
routurn project remove example-project
```

The registry is user-local configuration; absolute local paths are never written to `routurn.toml` or required in the public repository.

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


## AI/chat update archives

Routurn can safely consume an update archive received from a chat-based AI or another developer without manually extracting it into the project. The local project remains the source of truth.

Preview an update:

```bash
routurn apply ~/Downloads/update.zip --dry-run
```

Apply it after reviewing the file plan:

```bash
routurn apply ~/Downloads/update.zip
```

Routurn rejects path traversal, archive symlinks/special files, `.git/`, `.routurn/`, and writes through symlinked parent directories. Existing files are backed up under `.routurn/updates/<update-id>/` before replacement.

Rollback the most recent applied update:

```bash
routurn rollback latest
```

After rollback, `routurn sync` sends the restored local state back to the remote target.

Native archive formats in this release:

- `.zip`
- `.tar`
- `.tar.gz`
- `.tgz`

RAR and 7z are intentionally not extracted through an unsafe shell fallback yet. They can be added later as validated import backends while keeping the same `apply` interface.

Archives that contain one extra top-level directory can be handled explicitly:

```bash
routurn apply update.zip --strip-components 1
```

### Update + remote test in one command

A chat workflow can collapse the complete loop into one command:

```bash
routurn exec test ~/Downloads/update.zip
```

Routurn then performs:

```text
inspect + apply local update
→ create local backup
→ sync changed local files to remote
→ create remote snapshot
→ run the configured task with live terminal output
→ fetch declared artifacts
→ save the run manifest and logs
```

For scripted or AI-controlled local automation, confirmation can be explicitly disabled:

```bash
routurn exec test ~/Downloads/update.zip --yes
```

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

## Detached long-running tasks

Long training, build, simulation, and benchmark jobs can continue after the local terminal disconnects:

```bash
routurn run train --detach
```

Routurn starts the task in the background on the remote machine and returns a run ID. No Routurn daemon or binary is installed remotely. The run is represented by small state and log files under the remote project's `.routurn/runs/<run-id>/` directory.

Check whether it is still running:

```bash
routurn status latest
routurn status <run-id>
```

Reconnect to its logs without stopping it:

```bash
routurn logs latest --follow
```

`Ctrl+C` disconnects the log viewer; it does not stop the remote task.

Request a graceful stop:

```bash
routurn stop latest
```

Or force-stop it when necessary:

```bash
routurn stop latest --force
```

The complete sync + run loop can also be detached:

```bash
routurn exec train --detach
```

Because the local Routurn process exits immediately after launching a detached task, artifact collection is intentionally deferred. After the task finishes, collect its configured outputs with:

```bash
routurn fetch latest
```

Interactive tasks cannot be detached.

## One-command iteration

```bash
routurn exec test
```

Or include a local update archive in the same iteration:

```bash
routurn exec test ~/Downloads/update.zip
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

- structured `--json` output for AI/automation workflows
- release packaging and install helpers
- optional validated 7z/RAR import backends
- stronger end-to-end integration tests over disposable SSH targets

## Releases

Every `v*` Git tag triggers the release workflow. CI tests and vets the code, then the release workflow publishes cross-platform archives and `checksums.txt` to GitHub Releases. The release tag is embedded into the binary, so all of these report the same version:

```bash
routurn version
routurn --version
routurn -V
```
