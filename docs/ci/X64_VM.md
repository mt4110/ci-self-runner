# x86_64 Ubuntu VM on this Mac

This route runs a full x64 Linux kernel with QEMU TCG on an ARM Mac, then uses
Docker + act inside the VM for job execution. Nix is installed both on the VM
and in the job image. The host controller snapshots a project selected by path
and streams that copy through Docker; the VM has no host filesystem mounts.

## Reuse the prepared environment

```bash
cd ~/dev/ci-self-runner
nix develop .#vm
bash ops/ci/ci_self.sh vm start
bash ops/ci/ci_self.sh sandbox --vm ci-self-x64 \
  --project-dir /path/to/postal_converter_ja \
  --workflow .github/workflows/ci.yml --job launcher --nix-cache
bash ops/ci/ci_self.sh vm shell -- uname -m
bash ops/ci/ci_self.sh vm status
bash ops/ci/ci_self.sh vm stop
```

`ci-self` can replace `bash ops/ci/ci_self.sh` after installing the CLI symlink.
Start reuses an existing instance without replacing its configuration. `--name`
selects another `ci-self-*` instance. This CLI has no VM deletion operation.
`--vm` uses a temporary SSH-forwarded Docker socket and leaves global Docker
configuration unchanged. Run one workflow at a time because act's artifact
server uses its default port. Local workflows have network access.

The VM is 4 vCPU / 8 GiB RAM / 64 GiB thin disk. The host is an Apple M4 Pro,
14 logical CPUs / 64 GiB RAM. CPU translation is required; this is not hardware
virtualization of x64 instructions. Other preexisting VMs were left running.

## Recreate on another machine

Install Nix with flakes, Lima, and enter `nix develop .#vm` for the pinned QEMU.
The checked-in `ci/vm/ubuntu-x64.yaml` pins the official Ubuntu 24.04 amd64 cloud
image URL and SHA-256. Ubuntu package installation uses current apt repositories;
those package versions are not a reproducible snapshot.

The existing Homebrew Lima 2.2.0 lacked the x64 guest agent. This Mac therefore
uses an independent, matching official distribution without upgrading shared
Homebrew tools. Download these files from the [Lima v2.2.0 release](https://github.com/lima-vm/lima/releases/tag/v2.2.0),
verify SHA256SUMS before extraction, and place `bin/limactl`, `share/lima`, and
the additional x86_64 guest agent under one prefix:

| Archive | SHA-256 |
| --- | --- |
| lima-2.2.0-Darwin-arm64.tar.gz | bbdef91774885a0d05f7b048c4eb89ae2bcf3a0c252ae7ca7934e63df76d93c3 |
| lima-additional-guestagents-2.2.0-Darwin-arm64.tar.gz | 3aff4453eb3c359eb4f3b458056db24f2c5c15019232531292f49e04050554ed |

The prepared prefix is `~/.cache/ci-self-runner/lima-2.2.0`. `vm` checks that
location and the platform's user cache before PATH. `CI_SELF_LIMACTL` explicitly
selects another binary. This preparation is not an automatic downloader.
The small POSIX shell provision block is limited to standard apt/systemd/user
setup; CLI management, validation, and workflow execution stay in Go.

## Evidence and scope (2026-10-23 JST)

Local raw evidence is retained under `out/vm-evidence/` (not for publication).
VM identity: `ci-self-x64`, Ubuntu 24.04, kernel `6.8.0-147-generic`, x86_64,
QEMU 10.2.4, guest Nix 2.18.1, Docker 29.1.3. Job image Nix is 2.33.1.
`ci/vm/seccomp-smoke.nix` built in the VM with both Nix `sandbox` and
`filter-syscalls` true: guest `/usr/bin/time` reported 1.39 s first / 0.19 s
repeat. This tiny local derivation is a kernel/Nix compatibility proof, not a
project performance benchmark.

A subsequent normal stop took 24.25 s and a start from the existing disk took
70.11 s (host monotonic time). After restart the `ci` user had Docker/Nix group
access, the Nix smoke succeeded with sandbox/syscall filtering enabled, and no
job containers were running. Both interrupted containers remained stopped with
their data retained. The VM is left running for immediate use. Evidence:
`warm-stop.log`, `warm-start.log`, `reboot-poststate.log`.

The initial installation needed a missing `nix-setup-systemd` package and user
group corrections. Those are included in the template. Its approximately
6.5-minute first start is not a clean unattended provisioning benchmark.
Initial image attempts and failures are retained, including an interrupted
whole-store copy and a rejected unsigned-store copy. The final image fetches
only the runtime closure from the signed official Nix cache; signature checks
remain enabled.

Project: `postal_converter_ja`, source HEAD
`635e176fa7763063639620ef0419776c92975d32`, original `.github/workflows/ci.yml`.
The source worktree was preserved, including untracked `docs/articles/`.
Common `.env` files were excluded from the snapshot. No deployment/release jobs
or database teardown workflows were selected. Host monotonic phase timings in
`result.json` separate image preparation from workflow execution.

| Project job | Images | Workflow | Total | Result |
| --- | ---: | ---: | ---: | --- |
| launcher, first successful image build | 196.49 s | 165.31 s | 364.38 s | OK |
| launcher, repeat with same action cache | 5.67 s | 100.35 s | 108.96 s | OK |
| nix-node-security, bounded trial | 48.88 s | 323.40 s | 375.81 s | manually cancelled; incomplete |

These launcher runs used host act 0.2.89. Go compilation itself took 84.67 s /
84.59 s. `actions/setup-go` did not generate a dependency-cache key with this
workflow's current configuration, so the repeat still rebuilt packages. This is
an image/action-cache repeat, not a warm Go build. The first successful run
also followed interrupted/failed image preparation, so it is not a completely
empty-machine cold benchmark. Raw logs: `postal-launcher-signed-runtime.log`,
`postal-launcher-repeat.log`; result directories: `runs/run-528042885`,
`runs/run-957097592`. The Nix job uses act 0.2.88 and Docker CLI 29.9.1 from the pinned development
shell. Its image preparation used BuildKit, separately from the launcher trials.

`nix flake check --no-build` succeeded in 206.00 s. The next step requested 181
cached paths (357.5 MiB download / 1.1 GiB unpacked) and nine local derivations,
including Brotli 1.2.1 and Node.js 24.22.1 itself. Brotli compilation was observed;
the Node runtime check did not finish. The controller was manually interrupted
at 375.81 s total to bound the emulated source-build experiment. This does not
measure full Node build duration and is not a project test failure diagnosis.
The run remains `ERROR` with a failed act outcome, not a passing acceptance.
Evidence: `postal-nix-node-security.log`, `runs/run-2879276512/result.json`.

Cancellation exposed an SSH-forward lifetime bug: forwarding closed before act's
cleanup, leaving the job container running. That owned container was stopped
without deleting it or its data. Forwarding now remains alive until child cleanup
finishes; a regression test covers cancellation versus forwarding lifetime.
A real cancellation fixture (`cancellation-fixed.log`) confirmed that Docker
connection errors disappeared, but act 0.2.88 still left its job container running.
That owned container was also stopped. These early trials preceded the owned-container
stop added in the persistent-cache follow-up below. The current controller stops
its labelled running leftovers after act exits; this does not delete their data.
No container/image/VM data was pruned.

The measured Go subset is useful for local checks. This project's uncached Nix
source builds are a poor fit for rapid feedback on ARM-to-x64 TCG. Use native x64
compute or a trusted cache containing the project's exact outputs for that
workload; neither is provisioned by this change.

Full application CI,
frontend E2E, database integration, and hosted GitHub parity are outside this
acceptance subset.

## Remaining differences from GitHub Actions

The x64 kernel resolves the ARM-kernel/x64-Nix seccomp mismatch seen with Colima
userspace emulation. The jobs still run through act in Ubuntu containers,
not the exact hosted runner VM image. Installed packages, systemd/services,
permissions, OIDC, environments, concurrency, and timeout handling can differ.
Nix dependencies are used where the original workflow invokes Nix; other setup
actions run unchanged. Project Nix stores persist across fresh jobs when `--nix-cache` is enabled.
Persistent act caches only help where a workflow successfully saves/restores
its cache. See [the execution contract](SANDBOX.md) for flags and limitations.

## Persistent-cache follow-up

Use `--nix-cache` for the same project and architecture on subsequent runs:

```bash
nix develop -c bash ops/ci/ci_self.sh sandbox --vm ci-self-x64 --nix-cache \
  --project-dir /path/to/postal_converter_ja \
  --workflow .github/workflows/ci.yml --job nix-node-security --timeout 120m
```

The named store and evaluation-cache volumes live on the VM disk and are listed
in each result's `plan.nix_cache`. This is separate from host-side act caches.
The VM remains running. Named caches, snapshots, logs, and artifacts are retained.
New runs attach an invocation label; after act exits the controller stops that
invocation's running leftovers without deleting containers or volumes.

Proposed automatic deletion policy (not enabled): allow opt-in removal only of
stopped containers matching an exact recorded run label after evidence is saved.
First show IDs, labels, mounts and retained volumes for approval. Do not enable
engine-wide prune, VM deletion/shutdown, or cache/evidence removal. This respects
the target-specific deletion approval rule while making a future cleanup scope
reviewable. act's existing successful-job removal behavior is unchanged.

Validation of persistent-cache controls:

- A second real controller using the same project cache was rejected by the
  exclusive lock before job execution (`postal-nix-cache-concurrency.log`).
- An opt-in Docker integration test created a dedicated labelled sleep container,
  passed a cancelled context to the production cleanup function, and verified
  that the exact container was stopped and retained (`persistent-cache-cleanup-integration.log`).
  Independent inspection recorded both the retained stopped test container and
  the still-running real Nix job (`persistent-cache-cleanup-poststate.log`).
- The integration test is normally skipped. To run it intentionally, set
  `CI_SELF_TEST_DOCKER_HOST` to an explicit engine endpoint and
  `CI_SELF_TEST_DOCKER_IMAGE` to an existing Linux/amd64 image with `/bin/sleep`,
  then `go test -v ./cmd/sandbox -run TestStopRunContainersIntegration -count=1`.
  It creates a stopped container for inspection and does not remove it.

## Persistent-cache experiment stopped by user (2026-10-23 JST)

The project Nix cache was implemented and attached successfully. The first
persistent-store run (`runs/run-2271444980/result.json`) was stopped at the user's
request after 5273.81 s (87 min 53.81 s), before the earlier 120-minute ceiling.
The original `nix flake check --no-build` passed in 183.57 s. Brotli and ngtcp2
completed, but Node.js itself remained in its 1971-step source build. The final
Node runtime check and a full warm-job timing were not completed. Do not report
this run as a passing job or a measured cache speedup.

The labelled job container was stopped, the temporary cutoff guard exited, and
the VM stayed running. Both project Nix volumes and local run evidence remain.
See `postal-nix-user-stop.log` and `postal-nix-stopped-poststate.log`.
The intended rapid-local-feedback use case does not justify continuing this
full x64-emulation source-build route. Alternative execution plans are proposals;
no new VM, remote builder, cloud run, or deletion was started.
