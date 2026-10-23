# Local workflow execution with Ubuntu and Nix

`ci-self sandbox --project-dir PATH` snapshots a Git checkout, then copies it
into act-managed Linux job containers. It does not generate or replace the target
workflow. If there are multiple workflows, specify `--workflow`.

```bash
cd ~/dev/ci-self-runner
nix develop
ci-self sandbox --project-dir /path/to/project --workflow .github/workflows/ci.yml --list
ci-self sandbox --project-dir /path/to/project --workflow .github/workflows/ci.yml --job test --plan
ci-self sandbox --project-dir /path/to/project --workflow .github/workflows/ci.yml --job test
```

Without an installed `ci-self` symlink, use `bash ops/ci/ci_self.sh` instead.
For the dedicated full-system x64 VM use `--vm ci-self-x64`; see
[the VM runbook](X64_VM.md).
Docker Engine must be reachable; the command does not restart host services.
For a stale default Colima Unix socket, it can open a process-scoped SSH forward
to that same running local VM using `colima ssh-config`. The forward ends when
the command returns. Global Docker settings are unchanged; explicit `DOCKER_HOST`
endpoints are never silently substituted. Temporary forwarding files are kept in
`/tmp/ci-self-docker-*`; they contain SSH configuration paths, not private keys.

## Native ARM64 preflight on this Mac

Use the existing ARM64 Colima engine for an explicit ARM preflight. This does not
create another VM, rewrite the target workflow, or change global Docker context.
The same command accepts other Git project paths and supported workflow/job IDs.

```bash
cd ~/dev/ci-self-runner
nix develop
env -u DOCKER_HOST DOCKER_CONTEXT=colima bash ops/ci/ci_self.sh sandbox \
  --project-dir /path/to/project --workflow .github/workflows/ci.yml \
  --job test --arch linux/arm64 --timeout 10m
```

For Postal Converter, substitute
`--project-dir /path/to/postal_converter_ja --job launcher`.
ARM64 is an intentional architecture substitution; the tool warns and records it.
Keep GitHub Actions as final x64 acceptance. Jobs explicitly requiring x64, such
as this project's `checks.x86_64-linux.nodejs-security`, are not validated by this
ARM launcher result. Other projects may need different jobs or supported runners.

Measured on 2026-10-23 JST: Apple M4 Pro (14 logical host CPUs, 64 GiB RAM), existing
Colima using macOS Virtualization.Framework, Linux aarch64, 6 guest CPUs, Docker
reported 12,515,446,784 bytes RAM. Existing database/search services and the x64 VM
stayed running. This is a practical shared-machine measurement, not an isolated
CPU benchmark. Controller: pinned act 0.2.88 / Docker CLI 29.9.1. Target Go:
1.24.10 linux/arm64, verified in setup-go output.

The unmodified `.github/workflows/ci.yml`, job `launcher`, at source HEAD
`635e176fa7763063639620ef0419776c92975d32` passed twice:

| Measurement | First | Repeat |
| --- | ---: | ---: |
| Whole CLI wall time, including nix develop / go run | 50.70 s | 10.83 s |
| Sandbox internal total | 50.24 s | 10.23 s |
| Image preparation | 19.20 s | 3.32 s |
| Workflow, including Go build | 30.09 s | 6.07 s |
| Go build step within workflow | 4.80 s | 3.83 s |

The CLI uses host `/usr/bin/time -p`; internal phases and act step durations use
host monotonic clocks. The first row includes shell/controller startup overhead
outside `result.json`. This first run reused existing base layers/action caches
and fetched the ARM Go toolchain; it is not an empty-machine cold run. The repeat
reused image build layers and the Go toolchain. setup-go could not generate a
Go dependency-cache key because its default search did not find a root `go.sum`
(the project has `launcher/go.sum`). The workflow was preserved; do not describe
the repeat as a successful Go dependency-cache restore.

Both runs used the same immutable base/Nix digests, platform image manifest
`sha256:d1cd6297acc89b2f27b79c28d0da927d4285385b443ec03abe5a98a818abc537`,
and image config. The top-level image index IDs changed on re-export and are
recorded separately. Platform manifest/config equality was verified from BuildKit
export logs; the first top-level index was no longer inspectable after retagging.
The source workflow, launcher/go.mod, and launcher/go.sum matched both snapshots.
All six preexisting running container IDs/names were unchanged after the runs.
The original source worktree retained only its preexisting untracked docs/articles/.

Local evidence (not published):
`out/vm-evidence/postal-launcher-arm64-measurements.json`,
`postal-launcher-arm64-first.log`, `postal-launcher-arm64-repeat.log`,
and results in `runs/run-2039772803` / `runs/run-4270535666`.
Only the launcher build is accepted here: frontend, API, database integration,
Nix Node runtime security, releases/deployments, and hosted x64 acceptance remain
outside this measurement. No project-specific runner configuration was added.

## Execution contract

- Plan: read workflow YAML, resolve the selected job and its `needs`, check runner
  compatibility, print a JSON plan. No image downloads or file writes.
- List: let act list jobs, without requiring supported runners or Docker.
- Snapshot: clone Git metadata without copying local config/hooks or using hard
  links. Overlay current tracked files and non-ignored untracked files. Deleted
  files stay absent. Retain source history for versioning tools.
- Exclusions: `.actrc`, `.env`, `.env.*` except `.env.example`, `.secrets`, `.vars`,
  `.input`. Record excluded filenames. Other credentials and committed history
  are not scanned or sanitized. External symlinks and submodules fail explicitly.
- Images: pull the selected Ubuntu base and official Nix image, resolve immutable
  digests, build a derived image from those digests, record its ID. Copy only
  the Nix runtime closure, fetched from the signed official binary cache. Nix retains
  the official container configuration, using a single-user store. This does not
  provide an additional Nix build sandbox inside Docker.
- Workflow: pass the original YAML to act. Use copy mode, never a source bind
  mount or a forced remote checkout. No automatic host secrets or `.actrc`.
  Original setup actions and commands remain in control of project dependencies.
  A project flake is used when its workflow invokes Nix, not inferred automatically.
- Results: retain `result.json`, `run.log`, `artifacts/`, and the snapshot under
  the user's cache directory (`ci-self-runner/sandbox/run-*`). `--out` may change
  the parent, which must be outside the project. The tool does not prune runs.
- Result meaning: `OK` means act exited successfully with at least one successful
  selected job variant. Individual recorded outcomes still matter. Zero completed
  jobs, including an entirely skipped workflow, returns nonzero. This is not a
  hosted Actions acceptance result.

## Compatibility and resources

Default bases are `ghcr.io/catthehacker/ubuntu:act-22.04` / `act-24.04`.
`ubuntu-latest` currently maps to 24.04 and emits a warning; update this mapping
when GitHub changes it. The default architecture is `linux/amd64`, even on ARM
Macs. This can require emulation. Use `--arch linux/arm64` deliberately for ARM
jobs, rather than treating their results or timing as x64 evidence.

On the tested ARM64 Colima host, an x64 container can run workflow commands and
evaluate Nix, but `nix develop` fails with `unable to load seccomp BPF program`.
This is a cross-architecture Nix/emulation limitation, not a project test failure.
The tool warns when the Docker engine and requested architecture differ and does
not disable syscall filters. The dedicated `--vm ci-self-x64` full-system VM supplies an x64 kernel and
passes a Nix build without disabling syscall filters. Use an x64 Docker engine
for x64 Nix builds, or an
ARM workflow/engine deliberately. See [Nix issue #15153](https://github.com/NixOS/nix/issues/15153).

Static `matrix.KEY` runner expressions are expanded for compatibility checks.
Matrix `exclude` is not evaluated by this preflight; all candidate runners are
checked conservatively. act remains responsible for actual scheduling/matrices.
Dynamic expressions, reusable workflows, job `container:`, and unsupported
OS/runner labels fail explicitly. `--job` restricts checks to the dependency
closure, allowing a Linux job in a multi-OS workflow to run separately.

For more preinstalled tools, explicitly select a larger Linux base:

```bash
ci-self sandbox --project-dir /path/to/project --workflow .github/workflows/ci.yml \
  --platform ubuntu-24.04=ghcr.io/catthehacker/ubuntu:full-24.04
```

This may consume substantial bandwidth/disk. It still cannot reproduce VM
services, the hosted kernel, GitHub permission enforcement, OIDC, environments,
or concurrency. macOS and Windows cannot be reproduced by a Linux image.
Images and their build layers are cached by Docker; each run re-resolves base
tags and records the actual digests. Without `--nix-cache`, project Nix build stores are not shared
between job containers. Actions and the act cache server use a persistent user cache
(`ci-self-runner/act`); `--cache-dir PATH` overrides its parent. Cache data is local
workflow-controlled state, separate from the project snapshot. Existing workflow
cache configuration still determines whether dependency caches are saved/restored.
`result.json` includes host monotonic elapsed seconds for setup, snapshot, images,
workflow and total time. Image preparation and workflow duration are distinct.

### Persistent Nix cache (`--nix-cache`)

Opt in with `--nix-cache` to keep `/nix` (store, database, profiles, and fetched
flake sources) and `/root/.cache/nix` on named Docker volumes. With `--vm`, these
volumes live on that VM's disk, not on macOS. The volume identity includes the
canonical project path, architecture, immutable Nix image references, and runtime
Dockerfile. A changed runtime gets a separate cache instead of obscuring its
binaries with an older store. Moving a project path creates a different cache.
The normal CLI snapshot and workflow remain unchanged.

Docker initializes an empty store volume from the job image on first attachment;
subsequent job containers reuse the store. These are single-user root stores,
separate from the VM host's daemon store. Do not run GC while jobs use them.
Project cache data is mutable, workflow-controlled local state: only reuse it
for trusted workflows from that project. Host-controller file locking prevents
concurrent cache writers on this Mac. A running-container check also rejects
reuse of a cache left mounted after an interrupted run. `result.json` records
volume names and cache setup timing. No automatic cache or evidence pruning.

```bash
ci-self sandbox --vm ci-self-x64 --nix-cache --project-dir /path/to/project \
  --workflow .github/workflows/ci.yml --job nix-node-security --timeout 120m
```

Each job container carries `ci-self.run=run-...` for read-only inventory and
future targeted cleanup. The VM stays running after a workflow completes.
After act exits, this invocation stops any still-running containers carrying its
exact run label, including after cancellation. Stopped IDs are recorded in
`result.json`. Automatic deletion of failed/interrupted containers is not enabled. Before
approving deletion, inspect the exact container ID, stopped state, run label,
and mounts; retain named Nix/action caches and local logs/snapshots. Use no
engine-wide prune and no implicit VM shutdown/deletion. act's existing normal
successful-job cleanup is unchanged.

`--timeout` defaults to 45 minutes across setup and workflow execution, because
act does not enforce `job.timeout-minutes`. Cancellation sends an interrupt to
the active command, then bounds the wait; if act cannot finish cleanup, inspect
remaining containers before the next run. No automatic service restart/pruning.

## Credentials, networking, and side effects

Use `--secret-file`, `--var-file`, and `--event-file` only when necessary. The
default event is `push` with `{"act":true}`. An explicit event file is unchanged.
act also sets `ACT`, so workflows using that variable may skip or change steps.
No owner-guard or permissions check is rewritten to make a job pass.

Network access is enabled. act's artifact HTTP server starts for the duration of
the workflow using act's default bind address/port. Workflow code can still send
data over the network; this tool is not a hostile-code sandbox. It excludes common
local credential files but cannot prove that a project contains no secrets.

Docker socket access is disabled by default. `--docker-socket` explicitly grants
the workflow Docker-host control, for trusted workflows needing services, Docker
builds, or container actions. These paths require separate runtime validation.
Secrets are subject to act's masking, not a guarantee against arbitrary workflow
logging. Run directories should be treated as local evidence, not published.

## Implementation dependencies

Workflow YAML uses the YAML organization's maintained Go v3 fork,
`go.yaml.in/yaml/v3 v3.0.4` (MIT/Apache-2.0). It is the only added Go runtime
dependency. Parsing workflow structure with a real YAML parser avoids treating
quoted values, arrays, or anchors as ad-hoc shell text. Versions/checksums are
recorded in `go.mod` and `go.sum`.

References: [act runner images](https://nektosact.com/usage/runners.html),
[act limitations](https://nektosact.com/not_supported.html),
[GitHub runner images](https://github.com/actions/runner-images),
[official Nix image definition](https://github.com/NixOS/nix/blob/2.33.1/docker.nix).
