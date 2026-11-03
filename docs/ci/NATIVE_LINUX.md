# Native Linux workflow checks

Use Docker Engine directly on a Linux machine. Colima/Lima are optional Mac VM
routes, not prerequisites for this route. The controller's Nix shell pins Go,
act, Bash, Git, and Docker CLI; it does not install or start a Docker daemon.

## Environment selection

| Controller host | Current runtime evidence | Job execution choice |
| --- | --- | --- |
| macOS ARM64 | Controller checks passed | ARM64 Colima for ARM preflight; a separate x64 engine for x64 checks |
| Ubuntu x86_64 | Controller checks, original launcher and Nix security job passed | Local native Docker Engine, no extra VM |
| macOS x86_64 | Flake evaluation only | Runtime not measured |
| Linux ARM64 | Flake evaluation only for the controller | ARM64 job execution was measured in Colima with a Mac controller |
| Windows | No native controller definition or runtime validation | WSL2 is a possible Linux route, not verified by these results |

Use the target job's architecture for acceptance; ARM preflight does not replace
an x64 check. These are explicit measured scopes, not a promise that every Linux
distribution, Windows shell, runner label, or workflow feature is supported.

## Setup and normal execution

Install Nix with flakes enabled and a Docker Engine appropriate for your Linux
distribution. On the measured Ubuntu host, `docker.io` from its apt repository
was used. Installation and service startup require administrator authorization.
Neither the CLI nor entering `nix develop` grants Docker access or starts it.

From a checkout of ci-self-runner:

```bash
nix develop
ci-self-test
docker info
bash ops/ci/ci_self.sh sandbox --project-dir /path/to/project \
  --workflow .github/workflows/ci.yml --job test --arch linux/amd64 --plan
bash ops/ci/ci_self.sh sandbox --project-dir /path/to/project \
  --workflow .github/workflows/ci.yml --job test --arch linux/amd64 \
  --nix-cache --timeout 45m
```

Use `--arch linux/arm64` for a deliberate ARM64 run. Paths, workflow and job IDs
are parameters, not project-specific generated configuration. Select a job with
no deployment or destructive side effects for the first run. See
[the execution contract](SANDBOX.md) for unsupported workflow features and
differences from hosted GitHub Actions.

`docker info` must succeed for the identity running the controller. Docker access
grants control of the Docker host; do not automatically add users to a group or
change socket permissions. If an administrator explicitly chooses sudo instead,
run only trusted projects. On the measured host, the controller was built inside
`nix develop`, then invoked with `sudo -n env PATH="$PATH" ...`; no Docker group
membership was added. A single `safe.directory` Git setting was passed through
`GIT_CONFIG_COUNT=1`, `GIT_CONFIG_KEY_0=safe.directory`, and
`GIT_CONFIG_VALUE_0=/absolute/project/path` for that invocation, because the
project copy belonged to the ordinary user. No wildcard global Git trust was set.
Root-run evidence and caches belong to root; use sudo for inspection as necessary.
For that explicitly authorized sudo route, a complete example inside `nix develop`
is:

```bash
project="$(realpath /path/to/project)"
mkdir -p out
go build -o out/ci-self-sandbox ./cmd/sandbox
sudo -n env PATH="$PATH" \
  GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=safe.directory GIT_CONFIG_VALUE_0="$project" \
  "$PWD/out/ci-self-sandbox" --project-dir "$project" \
  --workflow .github/workflows/ci.yml --job test --arch linux/amd64 \
  --nix-cache --timeout 45m --out /var/tmp/ci-self-runs
```

This does not grant sudo authorization; `-n` fails when noninteractive sudo is
not already permitted. Prefer an existing administrator-approved Docker access
method rather than changing host privileges to make this example work.

## Storage and retained data

Before a large test, check the actual filesystem for each storage location:

```bash
findmnt -T /nix
sudo docker info --format '{{.DockerRootDir}}'
findmnt -T /var/lib/docker
df -h /nix /var/lib/docker "$HOME"
lsblk -o NAME,TRAN,SIZE,FSTYPE,MOUNTPOINTS
```

Adjust the Docker path check if `DockerRootDir` differs. A system booted from an
external SSD already stores ordinary `/nix`, `/var/lib/docker`, and home data on
that external root filesystem unless separate mounts override them. A 1 TB disk
can expose a smaller logical root volume; inspect LVM allocation before assuming
the remaining space is lost. No partition expansion, formatting, mount changes,
or cleanup is performed automatically.

Docker images, named Nix volumes, and act tool/action caches are separate from
`--out` evidence. Changing `--out` does not move Docker data. `--cache-dir` moves
only the controller's act cache parent, not Docker volumes or the Nix daemon
store. `--nix-cache` retains store/evaluation data on the selected Docker engine.
Keep the same project path and architecture to reuse the same project cache.
Do not switch or unplug a boot SSD while Ubuntu is running.

## Measured native x86_64 launcher (2026-11-03 JST)

Host: Intel Core i7-9700, 8 cores, about 61 GiB usable RAM, native x86_64 Ubuntu
26.04.1. Storage: ELECOM ESD-EPK 1 TB external SSD, ext4 root on LVM; USB link
reported 480 Mbps. Internal Windows disks remained unmounted. No Colima VM was
created. Tools: Nix 2.33.1, pinned Go 1.25.14 / act 0.2.88 / Docker CLI 29.9.1,
Docker Engine 29.1.3. The job uses Ubuntu 24.04 containers; the host kernel and
OS remain different from GitHub's runner VM.

Target: Postal Converter source `635e176fa7763063639620ef0419776c92975d32`, original
`.github/workflows/ci.yml`, job `launcher`. The retained sanitized snapshot's 211
files matched the current source before copying. The controller snapshotted that
copy again for each run; the original Mac project was unchanged.

| Sandbox monotonic measurement | First | Repeat |
| --- | ---: | ---: |
| Internal total | 258.95 s | 27.03 s |
| Images | 182.53 s | 10.09 s |
| Workflow including cleanup | 76.10 s | 15.76 s |
| Go build step within workflow | 6.68 s | 6.67 s |

OS/tool installation, controller build, Nix development-shell downloads, and
source transfer are outside these internal totals. The first run fetched the
job images; the repeat reused image layers and the Go toolchain. setup-go did
not generate a dependency-cache key; this is not a warm Go dependency-cache
restore. This job uses setup-go, not a project Nix build. Different host CPU,
USB storage, cache state, and timing scope prevent a normalized speed comparison
with the ARM Mac or emulated VM results.

Raw evidence is local only: `out/native-x64-20261103/`, results
`run-3804477378` / `run-267192864`. Both returned `OK`. No running job containers
remained after those two runs; named caches were retained. No data was pruned.

Controller validation: `nix develop -c ci-self-test` passed on macOS ARM64 and
native Ubuntu x86_64 after fixing the missing-Docker test fixture. The former
fixture assumed `/usr/bin:/bin` contained no Docker; it now builds a minimal PATH
with only its required utilities. The expected error/status assertions remain.
These controller tests include mocks and do not establish full hosted parity.

An opt-in native Docker integration test also passed: a dedicated labelled sleep
container was stopped after a cancelled context and independently observed as
`exited`. Its data was retained. The test used production cleanup code and did
not stop the concurrently running target Nix job. Evidence:
`cleanup-integration.log`; fixture ID
`4b46a369ea7ad4955f412056edcc0bae5e3d4c79bfe41802922157b1f8ca5a44`.

A second opt-in test verified the startup guard using a real running `act-`
fixture. The guard returned its ID and independent inspection confirmed that
it was still running. Test cleanup stopped and renamed only that fixture to a
non-act name, retaining its data without blocking later real jobs. The target
Node build continued throughout. Evidence: `guard-integration.log`.

While the Nix job held its project cache, a second controller targeting the same
path/architecture/engine returned nonzero with `Nix cache is in use by another
controller` before running a workflow. The original job continued. This verifies
the writer guard on the native engine for the same controller user, not shared
locking between different hosts or users.
Evidence: `cache-concurrency.log`.

## Nix job acquisition failure and retry

The first native `nix-node-security` attempt (`run-1408257098`) returned `ERROR`
after 177.31 s total / 167.26 s workflow. It failed during source expansion with
`Truncated tar archive detected while reading data`, before the Node build.
The labelled leftover was initially stopped and retained. The later retry reused
act's deterministic job name and act removed that old container; logs, result
JSON, source snapshots, and named Nix caches remained. This observation exposed
a preservation gap in the earlier implementation. The controller now refuses
to start when any `act-` container exists on the engine, before snapshots or image
pulls, instead of allowing act to remove or reuse it. This is a failed attempt, not
a passed job or part of a successful warm timing.

The official GitHub archive at the pinned Nixpkgs revision was subsequently
downloaded independently; gzip integrity and full tar listing succeeded. A
workflow retry with the original lock file, signed-cache settings, source and
commands then passed evaluation. The underlying transport/expansion fault was
not isolated; no source hash check was bypassed, cache was deleted, or workflow
step was changed. Evidence: `nix-first.log`, `archive-download.log`.

## Native source-build findings

The next attempt (`run-2527172352`) completed all 1,971 compile steps but returned
`ERROR` after 2576.54 s total / 2566.62 s workflow. Node's own tests reported a
directory-permission failure (`test-fs-rmSync-special-char-additional-error`) and
two cluster-primary timeouts. The permission test expected a write-denied
directory to resist removal, which a root builder bypassed. The cluster tests
waited for exited workers to disappear; inspection found orphaned zombie
processes under the job's tail PID 1. These failures were not skipped or accepted.

The corrected image keeps workflow commands as root but gives Nix a dedicated group
of unprivileged builders. `ci/nix/build-user-smoke.nix` was built in the image:
UID 999 was nonzero and directory write denial was enforced. This is a
container-specific diagnostic using Ubuntu's shell/utilities, not a portable Nix
package. The job image still does not provide an additional Nix build sandbox.
Syscall filters and signed-cache checks were not disabled.

The controller now passes Docker `--init` through act's container options. A
real Docker test verified that an orphaned child was reaped; the fixture was
retained stopped with `HostConfig.Init=true`. These probes verify the isolated
fixes; the complete original-workflow results are recorded below.
Evidence: `build-user-image.log`, `build-user-smoke.log`,
`reaper-integration.log`, `nix-retry.log`.

Failed-run metadata, the stopped container filesystem, and its two act job
volumes were archived under `failed-root-archive/` on the external SSD. Nix and
tool caches are separate retained volumes. The cleanup plan records exact
resource names and archive hashes; deleting any of those resources requires
explicit approval. Existing act containers are not silently removed by this CLI.

After these fixes, `ci-self-test` again passed on both controller hosts. A real
CLI invocation against the stopped failed container returned its ID before
creating the requested output directory (`guard-cli.log`). An attempted updated-image ARM64
launcher run stopped at Docker connection setup because Colima was not running.
No Mac job ran and no service restart was attempted in that probe. The historical
ARM measurements above remain separate from this unperformed updated-image run.

The user subsequently authorized removal of exactly the old stopped container
and its two act job volumes after backup verification. That cleanup completed;
all three cache volumes (Nix store, Nix evaluation, toolcache) were checked to
remain. The corrected workflow uses a new recipe-keyed store. Independent
inspection confirmed `HostConfig.Init=true`, `build-users-group=nixbld`,
`filter-syscalls=true`, `require-sigs=true`, and compiler/ninja processes running
as UID 999. Evidence: `cleanup-execution.json`, `fixed-runtime-config.log`,
`fixed-builder-processes.log`.

## Corrected original Nix workflow: first successful build and repeat

With the same source HEAD and unmodified `.github/workflows/ci.yml`, job
`nix-node-security` passed in `run-230799280` and `run-1812653538`.
The former used an empty project Nix store for the corrected recipe, with base
images already downloaded. All Node package tests completed, including the
previous directory-permission and two cluster-primary failures. The final
project check printed Node **24.22.1**, Brotli **1.2.1**, OpenSSL **3.6.5**, and
passed the Brotli round-trip assertion under the development shell's shared
library search path. No test expectations, lock files, or workflow commands were
changed to obtain this result.

| Host monotonic measurement | First successful build | Persistent-cache repeat |
| --- | ---: | ---: |
| Sandbox internal total | 2577.48 s (42 min 57.48 s) | 15.83 s |
| Image preparation | 15.05 s | 9.52 s |
| Workflow including cleanup | 2562.12 s | 6.02 s |
| Nix evaluation step | 58.08 s | 1.53 s |
| Nix build/check step | 2494.25 s | 0.22 s |

These totals exclude OS/tool provisioning, controller compilation/startup, source
transfer, diagnostic probes, and preceding failed attempts. The first successful
run is a cold project-store build, not an empty-machine cold benchmark. The repeat
reused the already successful check derivation, including its Node runtime test
result; it did not repeat all Node compilation or assertions from scratch.

Both runs used the same base/Nix digests and Dockerfile recipe, and the same
store/evaluation volumes with cache key `8c0802e08fb9d20ae9aceac5cd3be3ce`.
BuildKit export logs showed the same platform manifest
`sha256:ea8e301039683da9826761d0c0315b73306ae0c90cf23ad0b3c12c7f13f558fa`
and image config; top-level image index IDs differed on export and are recorded
separately in results. The earlier root-build cache remains a separate volume.
Evidence: `nix-fixed-first.log`, `nix-fixed-repeat.log`, corresponding result JSON.

The corrected controller also ran `launcher` from the relative path
`../project with spaces` successfully (`run-1207640545`): 24.53 s total,
16.74 s workflow. This used generic path selection and the original workflow;
no per-project configuration was generated. This case did not opt into a Nix
store cache. Evidence: `launcher-space-path.log`.

This validates the controller and the selected launcher/Nix jobs on this native
host. Frontend, API, DB integration, releases/deployments, updated-image ARM64
runtime, Windows, and hosted GitHub Actions parity are outside these results.

Implementation references:
[act 0.2.88 job naming and lifecycle](https://github.com/nektos/act/blob/v0.2.88/pkg/runner/run_context.go),
[act container option parsing](https://github.com/nektos/act/blob/v0.2.88/pkg/container/docker_run.go),
[Node 24.22.1 directory-permission test](https://github.com/nodejs/node/blob/v24.22.1/test/parallel/test-fs-rmSync-special-char-additional-error.js).
