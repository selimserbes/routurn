# Routurn

[![CI](https://github.com/selimserbes/routurn/actions/workflows/ci.yml/badge.svg)](https://github.com/selimserbes/routurn/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

**Agentless project iteration CLI for running tasks locally or over SSH, applying updates, and collecting results.**

Routurn supports local builds, tests, and development workflows as well as workflows where code is edited locally but simulation, benchmarking, training, or another runtime lives on a remote machine.

Remote machines do **not** need Routurn, a daemon, or a privileged service. Routurn stays on the user's machine and uses standard SSH with common remote tools (`sh`, `tar`, and `find`) when remote execution is configured.



## v0.4.0: local execution, SSH jump, and project selection

To use Routurn without SSH, run `routurn init --local` in a new project, or add
the following to an existing `routurn.toml`:

```toml
[execution]
mode = "local"

[tasks.smoke]
command = "echo local-run"
artifacts = ["outputs/**/*.json"]
```

`routurn exec`, `routurn run smoke`, `routurn fetch`, `routurn update`, and
`routurn result` use the existing TUI/history/output conventions. Local exec
runs from the project root, without SSH or upload. Artifact patterns must be
project-relative and symlinks are not followed. Generated Routurn result views
are excluded from local artifact collection, including with broad patterns;
unrelated user-owned `results/` directories are not excluded. Local detached
execution (`routurn exec --detach`, `routurn run --detach`) is **not supported**.

Without `[execution] mode = "local"`, existing projects keep the v0.3.0 SSH
behavior. `mode = "remote"` is also accepted explicitly.

### SSH jump hosts

Use `routurn target add <name> ... --jump bastion` (or a comma-separated
jump chain). Jump routes are shared by remote command execution, sync, and
artifact transfer. Existing targets without `--jump` remain direct. A local
SSH `Host` alias can be used as a jump hop. Offline argument/configuration
tests cannot prove a bastion is reachable; test actual connectivity separately.

### Choosing and checking projects

A registered name and an explicit directory path are both accepted by the
project flag; paths may be absolute, `./relative`, `../relative`, or `~/path`.
When given a nested directory, Routurn finds the nearest `routurn.toml`
ancestor. A name that matches a registered project takes priority over an
ambiguous relative path.

```bash
routurn project add ~/workspace/python/example --name example
routurn project list
routurn project show example
routurn project check example                   # local configuration only; no SSH
routurn project select                          # interactive; prints a CLI hint
routurn -p example exec
routurn -p ~/workspace/python/example exec
routurn -p ./my-other-project result smoke
```

When started **interactively outside any Routurn project**, bare `routurn exec`,
`routurn update`, and `routurn result` ask which registered project to use
*before* showing their normal TUI. This choice is for the current command
only: no persistent default is written and selecting a project never runs a
command or applies an update. In a project (including subdirectories), that
project remains the default. Explicit actions and noninteractive usage require
a project directory or `-p`; Routurn never guesses a project for a script.

Missing or invalid project entries are omitted from the interactive picker but
are still visible with `routurn project list`. Registering a different folder
with an already-used name is rejected unless you explicitly pass
`routurn project add ... --name NAME --replace`; `routurn init` also refuses
to overwrite another project's registry entry.


## Install

Routurn is distributed as a single local binary. Nothing is installed on remote targets.

### Go install

With Go installed:

```bash
go install github.com/selimserbes/routurn/cmd/routurn@latest
```

### Release installer

Linux and macOS users can install the latest release binary with:

```bash
curl -fsSL https://raw.githubusercontent.com/selimserbes/routurn/main/install.sh | sh
```

The installer verifies the release archive against the published SHA-256 checksum and installs `routurn` to `~/.local/bin` by default. Override the destination with `ROUTURN_INSTALL_DIR`.

Install a specific release with:

```bash
curl -fsSL https://raw.githubusercontent.com/selimserbes/routurn/v0.4.0/install.sh | ROUTURN_VERSION=v0.4.0 sh
```

Release binaries are built for Linux, macOS, and Windows on amd64 and arm64.

### Windows install

The shell installer above is for Linux and macOS. On Windows, download the matching `routurn_windows_amd64.zip` or `routurn_windows_arm64.zip` asset from the [latest GitHub release](https://github.com/selimserbes/routurn/releases/latest), verify it against `checksums.txt`, extract `routurn.exe`, and place it in a directory on `PATH`.

### Platform scope

Routurn runs as a local client on Linux, macOS, and Windows. Native CI exercises the Go client on all three operating systems.

Remote execution currently targets Unix-like hosts reachable over SSH. Remote hosts are expected to provide standard tools such as `sh`, `tar`, and `find`.

Native Windows remote targets are not part of the current support contract.

## Quick start

Create or enter a local project, then initialize Routurn:

```bash
cd example-project
routurn init
```

Register an SSH target locally:

```bash
routurn target add remote-dev --host dev.example.com --user developer
```

Point the project at the remote target/path once:

```toml
[remote]
target = "remote-dev"
path = "/home/developer/projects/example-project"
```

Then let Routurn discover common project commands:

```bash
routurn exec
```

Routurn recognizes common Go, Rust, Node/web, Python, Make, Docker Compose, and executable project entrypoints. Nothing is locked to those ecosystems: any command can always be run directly.

The v0.3.0 task chooser supports arrow keys, numbered selection (including `0` for the tenth entry), `/` search, and `Enter` to confirm. Shortcuts switch views immediately; selecting a task or update does not execute it merely because it was highlighted. `Esc` cancels an editor or goes back. Recent commands can appear first based on actual run history; the picker never assumes that a name such as `stage3` means "latest". Projects may optionally define their own picker groups.

In terminals without interactive keyboard support (and in scripted/CI usage), continue to use the explicit CLI forms shown below. The terminal interface is convenience, not a required protocol.

```bash
routurn exec -- go test ./...
routurn exec --detach -- cargo run --release
```

Frequently used commands can be remembered without editing `routurn.toml`:

```bash
routurn task save test -- go test ./...
routurn exec test
```

Routurn syncs changes, streams remote output live, preserves run metadata, and brings declared artifacts back for configured tasks.

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

Routurn is language- and framework-independent. Remote execution is command-first. Saved local tasks are lightweight command shortcuts, while configured tasks can additionally declare artifact and interactive-run metadata.

## Current commands

```bash
routurn init
routurn doctor

routurn target add <name> --host <host> [--user <user>]
routurn target list
routurn target endpoint add <target> <endpoint> --host <host> [--user <user>]
routurn target endpoint list <target>
routurn target endpoint remove <target> <endpoint>
routurn target merge <target> <other-target> --primary-name <name> --as <name>
routurn target route <target> [auto|endpoint]
routurn target test [target]
routurn target remove <name>

routurn project add [path] [--name <name>] [--replace]
routurn project list
routurn project show <name>
routurn project select
routurn project check [name-or-path]
routurn project remove <name>

routurn status [task|run-id|latest] [--check]
routurn bundle inspect <update-archive> [--json]
routurn bundle fingerprint [--json]
routurn update [archive] [--recent] [--dry-run] [-y]
routurn apply <update-archive> [--dry-run] [-y]
routurn updates
routurn rollback <update-id|latest|previous>
routurn sync [--dry-run]
routurn run <task> [--detach]
routurn logs <task|run-id|latest> [--follow]
routurn stop <task|run-id|latest> [--force]
routurn fetch [task|run-id|latest]
routurn result [task]
routurn task list
routurn task save <name> -- <command>
routurn task remove <name>
routurn exec
routurn exec <task> [--update [recent|latest|path]] [--detach]
routurn exec [--detach] -- <command>
routurn runs
routurn runs show <run-id|latest> [--json]
routurn clean [--dry-run]
routurn completion <bash|zsh|fish|powershell>
routurn completion install <bash|zsh|fish|powershell>

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
--endpoint <name>  one-command endpoint override
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

### SSH jump hosts / bastion routing (v0.4.0 development)

Routurn uses OpenSSH's `ProxyJump` (`ssh -J`) for one or multiple SSH hops.
The jump route is stored **per endpoint**, so `sync`, `exec`, `run`, `fetch`,
`status --check`, and route probing use the same chain throughout a workflow:

```bash
# Single-route target
routurn target add gpu --host 10.0.0.42 --user mss --jump dev@bastion.example.com

# Named route on an existing target
routurn target endpoint add gpu via-bastion \
  --host 10.0.0.42 --user mss --jump dev@bastion.example.com --priority 20

# Two jumps (OpenSSH reaches each hop in sequence)
routurn target endpoint add gpu two-hops \
  --host 10.0.0.42 --user mss --jump gateway1,gateway2:2222 --priority 30

routurn target endpoint list gpu
routurn target test gpu
```

An alternate no-Routurn-config approach is to define a `Host` with `ProxyJump`
in `~/.ssh/config` and use its alias as `--host`. Both approaches use your
normal SSH keys and host-key verification; no credentials or agent forwarding
are configured by Routurn. Jump-enabled endpoints explicitly disable agent
forwarding for the destination connection and get separate multiplex sockets
from direct connections. To clear a configured jump, update with `--jump ''`.
**No bastion is required for local projects or existing direct SSH routes.**

### Multiple routes to the same remote machine

A logical Routurn target can have more than one named SSH endpoint. This is useful when the same workstation is reachable through a fast local network while on site and a VPN, overlay network, bastion route, or alternate address while away.

Existing single-route targets remain valid. Two existing target records can be explicitly grouped as routes to the same logical machine:

```bash
routurn target add remote-dev --host dev-lan.example.com --user developer
routurn target add remote-dev-vpn --host dev-vpn.example.com --user developer

routurn target merge remote-dev remote-dev-vpn \
  --primary-name lan \
  --as vpn \
  --remove-source
```

The merge is an explicit declaration that both routes reach the same remote machine. Routurn never falls back to an unrelated target merely because the configured target is unreachable.

Use automatic routing:

```bash
routurn target route remote-dev auto
routurn status --check
```

In `auto` mode Routurn uses a short non-interactive SSH probe, prefers the recently successful endpoint during rapid iteration, then falls back through endpoint priority order. One endpoint is resolved and pinned for the whole command workflow, so a single `exec` does not switch addresses midway through sync, run, and artifact fetch.

Choose a persistent route interactively:

```bash
routurn target route remote-dev
```

or explicitly:

```bash
routurn target route remote-dev vpn
```

Override the saved route for only one command:

```bash
routurn --endpoint vpn exec test
```

Add another endpoint directly when the logical target already exists:

```bash
routurn target endpoint add remote-dev backup \
  --host dev-backup.example.com \
  --user developer \
  --priority 30
```

Lower priority numbers are preferred by automatic routing. `routurn target test remote-dev` checks every configured endpoint; `routurn status --check` also reports the endpoint Routurn would select for the current project.


## Zero-config command discovery

`routurn exec` does not require a predeclared task. It inspects the local project and offers high-confidence runnable commands from supported ecosystems, plus project entrypoints discovered without assuming a `scripts/` directory. The v0.3.0 picker supports searching, paging, recent-run prioritization, and an **All tasks** view to reach discovered maintenance commands such as `install_*`, `setup_*`, `migrate_*`, `bootstrap_*`, `generate_*`, `register_*`, and `resolve_*`. Saved tasks and discovered entrypoints are deduplicated when they represent the same command. Any task grouping is optional project configuration: no framework-specific or `stage` naming convention is required. Initial built-in discovery covers:

- Go (`go.mod`, root/cmd main packages, build/test)
- Rust (`Cargo.toml`, default/named binaries, build/test)
- Node/web (`package.json` scripts with npm/pnpm/yarn/bun lockfile hints)
- Python (`pyproject.toml` entry points, common app/test entrypoints)
- Make targets
- Docker Compose
- executable, shebang-based, and conventional runnable entrypoints anywhere near the project root

Unsupported languages and tools are never blocked. Use `routurn exec -- <command>` for arbitrary commands. Non-executable entrypoints with a shebang, or conventional names such as `run_*`, `train_*`, `test_*`, `smoke_*`, `seed_*`, and `telemetry_*`, can still be offered with an appropriate interpreter. Local shortcuts created with `routurn task save` are stored under `.routurn/tasks.toml`, merged automatically at runtime, and do not require editing or syncing `routurn.toml`.

The design principle is **command first, task second**: discovery is convenience, arbitrary commands are the universal fallback, and saved tasks are optional shortcuts.

### Interactive selection (v0.3.0)

The same keyboard conventions are used for `exec`, `update`, and `result` when an interactive terminal is available:

| Input | Behavior |
| --- | --- |
| `↑` / `↓` | Move the highlighted selection |
| `←` / `→` | Switch pages when paging is available |
| `1`–`9`, `0` | Highlight the matching item on the current page (`0` = tenth item) |
| `Enter` | Confirm the highlighted selection or submit an entered command/path |
| `/` | Open live search |
| `Esc` | Cancel the active editor or return to the previous view |
| `q` | Exit the selector when in the task/list view |

In the task chooser, `e` opens a custom-command editor without hiding the task list. `Esc` returns to the same selection without executing a command. In the update chooser, a selected archive still goes through the existing review and apply-confirmation checks; the picker does not bypass update safety. `routurn result` with no task argument selects from previously materialized results, while `routurn result <task> --path` remains suitable for scripting. No interactive menu is required when an explicit task or command is supplied.

The terminal picker is available where terminal keyboard handling is supported; otherwise Routurn keeps non-interactive/line-based CLI paths. For debugging startup costs, run `ROUTURN_STARTUP_TIMING=1 routurn exec` and exit the menu; it prints local discovery/history timings. Automatic SSH route checking is deferred until after choosing a task, so simply opening the menu need not connect to a remote target.

Optional grouping is project-owned rather than guessed from names. For example, a project with tasks named `build` and `test` can add this to `routurn.toml`:

```toml
[picker]
default_group = "Daily"

[[picker.groups]]
name = "Daily"
tasks = ["build", "test"]
```

This configuration is not required for task discovery or search; projects without it get the general task list. Groups do not change how tasks are run or how updates are applied.

## AI/chat update archives

Routurn can safely consume an update archive received from a chat-based AI or another developer without asking the user to manually extract, rename, or clean up the download. The AI does not need to know Routurn: ordinary supported archives without a `routurn-bundle.toml` are still detected interactively as `[generic archive] [review required]`. They are never eligible for automatic `--recent` selection, and Routurn shows the apply plan/confirmation before changing the project. The local project remains the source of truth.

### Human-friendly update selection

The normal interactive workflow is:

```bash
routurn update
```

Routurn presents a terminal menu with recently detected Routurn bundles, a terminal file browser, direct path entry, and the latest previously managed update. Safe matches are grouped under `Recommended updates`; bundles with stale state, missing verification, or a different/missing manifest project name remain visible under `Other detected updates` instead of disappearing. State labels include `[compatible]`, `[scoped ok]`, `[state differs]`, and `[unverified]`; identity warnings such as `[project name differs]` are shown separately. Discovery and safety validation are intentionally separate: filenames and human-readable project names help the UI, but they are not trusted as the apply-time safety boundary. Complete target-file preconditions or an exact whole-project fingerprint provide that boundary. The file can live anywhere the user can access; Routurn does not require a hard-coded Downloads directory.

An explicit path is always supported:

```bash
routurn update /path/to/update.zip
```

For a safe automatic choice:

```bash
routurn update --recent
```

Interactive discovery is intentionally bounded but user-visible. Routurn checks a small set of normal user locations (including the last directory used, current directory, standard user directories, and the system temporary directory), never recursively scans the whole disk, deduplicates candidates by SHA-256, and shows valid Routurn bundles even when the manifest project name is missing or different. Such bundles are never silently promoted to the automatic path. `routurn update --recent` remains conservative: it requires a matching manifest project name plus either an exact whole-project base fingerprint or complete target-file preconditions that match the current local files. If multiple distinct safe candidates remain, Routurn asks instead of guessing.

### Managed ownership and download cleanup

`routurn update` imports the selected archive into Routurn-managed persistent storage before applying it. The copy is verified by SHA-256. After the managed copy is safely registered, the original selected archive is removed by default so browser downloads such as `update.zip`, `update (1).zip`, and `update (2).zip` do not accumulate indefinitely.

Use `--keep-source` when the original archive should remain in place:

```bash
routurn update /mnt/share/update.zip --keep-source
```

Routurn never deletes unrelated files from Downloads, Desktop, or any other directory. It only removes the exact archive the user selected or explicitly supplied, and only after a verified managed copy exists.

Managed bundles use content-addressed storage under the platform's user data directory. Identical archives are stored once even when the browser gave them different filenames.

### Bundle identity and compatibility

New Routurn bundles can include a root-level `routurn-bundle.toml`:

```toml
schema = 1

[bundle]
name = "example-fix"

[project]
name = "example-project"

[base]
fingerprint = "sha256:..."

[base.files]
"source/example.go" = "sha256:..."
"scripts/new-helper.sh" = "missing"
```

The manifest is metadata and is not written into the project. The project name is a human-readable identity hint, not the sole safety boundary: a bundle that accidentally uses a different project name is still discoverable. Routurn only permits that bundle to continue when an exact whole-project fingerprint or complete matching target-file preconditions independently verify the local base state; otherwise it rejects the update. AI/chat updates can include **complete target-file preconditions** under `[base.files]`: every payload path must have either the SHA-256 of the file state the update was authored against or `missing` for a newly-added file. If unrelated project files changed but all target-file preconditions still match, Routurn can safely apply the bundle without requiring the user to provide a fresh whole-project fingerprint. If any target file changed, Routurn rejects the bundle and reports the conflicting path. Legacy archives without a manifest remain available through manual selection/path workflows, but Routurn warns that project/base identity cannot be verified and they are not eligible for automatic `--recent` selection.

Filenames are not identities. `update.zip` and `update (7).zip` are equivalent when their content hash is identical.

### Safety and rollback

Routurn rejects path traversal, archive symlinks/special files, `.git/`, `.routurn/`, and writes through symlinked parent directories. Existing files are backed up under `.routurn/updates/` before replacement.

Rollback the latest applied update to the previous local state:

```bash
routurn rollback previous
```

`routurn rollback latest` remains equivalent. Show the human-readable update history with:

```bash
routurn updates
```

After rollback, `routurn sync` sends the restored local state back to the remote target.

Archive support:

- `.zip` — native
- `.tar` — native
- `.tar.gz` / `.tgz` — native
- `.7z` — validated local 7-Zip importer
- `.rar` — validated local 7-Zip importer

`.7z` and `.rar` require a local 7-Zip-compatible CLI named `7zz`, `7z`, or `7za`. This is a **local-only optional dependency**; nothing extra is installed on the remote target. Routurn lists and validates entries first, rejects links/special files/encrypted entries/unsafe paths, then streams selected file contents through stdout instead of extracting the archive directly into the project.

Inspect any supported bundle without modifying a project:

```bash
routurn bundle inspect /path/to/update.7z
routurn bundle inspect /path/to/update.rar --json
```

`routurn doctor` reports whether the optional `.7z`/`.rar` importer is available. ZIP/TAR support does not depend on it.

Archives that contain one extra top-level directory can be handled explicitly:

```bash
routurn update /path/to/update.zip --strip-components 1
```

### Update + remote task in one command

Open the interactive update chooser, apply the selected bundle, sync, run, and fetch artifacts:

```bash
routurn exec test --update
```

Use the safe recent-bundle discovery mode:

```bash
routurn exec test --update recent
```

Or supply a path directly:

```bash
routurn exec test --update /path/to/update.zip
```

`--update=/path/to/update.zip` is also accepted. The older positional `routurn exec test update.zip` form remains for compatibility, but the managed `--update` workflow is recommended.

Routurn then performs:

```text
validate bundle identity + safety
→ import/deduplicate into managed storage
→ optionally remove the original download
→ apply local update + create rollback backup
→ sync changed local files to remote
→ create remote snapshot
→ run the configured task with live terminal output
→ fetch declared artifacts
→ materialize the latest successful task result
→ prune bounded local history
```

For scripted automation, confirmation can be explicitly disabled:

```bash
routurn exec test --update /path/to/update.zip --yes
```

## Sync safety

Routurn does not blindly mirror and delete an entire remote directory.

It maintains a local content-hash manifest under `.routurn/` and synchronizes only files it tracks. Before overwriting or deleting tracked remote files, Routurn creates a project-local remote snapshot:

```text
<remote-project>/.routurn/snapshots/<run-id>.tar
```

Routurn automatically keeps the newest 10 remote sync snapshots so this internal safety history does not grow without bound. `.git/` and `.routurn/` are always excluded from project synchronization.

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

Routurn starts the task in the background on the remote machine. No Routurn daemon or binary is installed remotely. Internal run IDs remain available for audit/debug history, but normal commands can address the task by name. The remote run is represented by small state and log files under the remote project's `.routurn/runs/<run-id>/` directory. Routurn preserves running jobs and automatically bounds completed detached-run state history.

Check whether it is still running:

```bash
routurn status train
routurn status latest
```

Reconnect to its logs without stopping it:

```bash
routurn logs train --follow
```

`Ctrl+C` disconnects the log viewer; it does not stop the remote task.

Request a graceful stop:

```bash
routurn stop train
```

Or force-stop it when necessary:

```bash
routurn stop train --force
```

The complete sync + run loop can also be detached:

```bash
routurn exec train --detach
```

Because the local Routurn process exits immediately after launching a detached task, artifact collection is intentionally deferred. After the task finishes, collect its configured outputs with:

```bash
routurn fetch train
```

Interactive tasks cannot be detached.

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
→ materialize latest successful task result
→ save bounded run history
```

Normal users do not need to copy internal run IDs. Routurn keeps its canonical latest successful artifact set under `.routurn/results/<task>/`, and publishes a short user-facing view in the project root. For a task with one artifact:

```text
results/<task>/latest.<ext>
```

For a task with multiple artifacts, the common remote artifact prefix is stripped and the files are exposed directly under `results/<task>/`. The user-facing view is Routurn-managed, replaced atomically after each successful fetch, excluded from Routurn sync/fingerprinting, and locally excluded from Git when possible. If the project already has a user-owned `results/` directory, Routurn does not claim it and uses `routurn-results/` instead.

Show a result interactively, or use a named task directly:

```bash
routurn result
routurn result test
routurn result test --path
```

`--path` prints only the stable user-facing file or directory path, which is convenient for scripts and desktop upload dialogs without making Routurn depend on a GUI. Existing results created by older Routurn versions are published into the visible view lazily the first time `routurn result <task>` is run.

Detailed history remains available under `.routurn/runs/<run-id>/` for debugging and audit purposes. By default Routurn automatically keeps the newest 10 local run histories and 10 update backups, plus the newest 20 managed bundles globally. Inspect or override cleanup explicitly with:

```bash
routurn clean --dry-run
routurn clean
routurn clean --keep-runs 20 --keep-updates 20 --keep-bundles 30
routurn clean --keep-snapshots 20 --keep-remote-runs 20
```

Normal `clean` also prunes old remote snapshots and completed detached-run state when the configured target is reachable. Use `--local-only` to skip remote cleanup. `--dry-run` never performs remote deletions.

Inspect recent runs when needed:

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

The stable latest result is then available through:

```bash
routurn result benchmark
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

Routurn does not need to understand the programming language. It needs a project and a command that is discovered, selected, saved, or supplied directly; SSH targets are optional for projects configured with `[execution] mode = "local"`.


## Shell completion

Generate completion for your shell:

```bash
routurn completion bash
routurn completion zsh
routurn completion fish
routurn completion powershell
```

To see the recommended installation command for a shell:

```bash
routurn completion install zsh
```

## Requirements

Local machine:

- Routurn
- OpenSSH client (`ssh`) for remote mode only

Remote machine (remote mode only):

- SSH access
- `sh`
- `tar`
- `find`
- the project's own runtime/toolchain

No Routurn binary is installed on the remote machine.

## Remote task environment

Routurn runs remote tasks through non-interactive SSH sessions. Environment variables that are configured only by an interactive shell startup file may therefore be unavailable. If a task requires environment-specific proxy settings, credentials, SDK paths, or runtime configuration, define them in the task command or provide them through the remote environment using the deployment mechanism appropriate for that system.

Do not commit secrets or machine-specific credentials to public `routurn.toml` files. Machine-specific values should remain local/private.

## Development

```bash
go mod tidy
go test ./...
go run ./cmd/routurn version
```

## Roadmap

Near-term work includes:

- structured `--json` output for AI/automation workflows
- stronger end-to-end integration tests over disposable SSH targets
- richer machine-readable run/event output
- finish and harden v0.4.0 local and multi-hop SSH workflows, including real-host integration tests

## Releases

Every `v*` Git tag triggers the release workflow. CI tests and vets the code, then the release workflow publishes cross-platform archives and `checksums.txt` to GitHub Releases. The release tag is embedded into the binary, so all of these report the same version:

```bash
routurn version
routurn --version
routurn -V
```

## License

Routurn is licensed under the Apache License 2.0. See [LICENSE](LICENSE).
