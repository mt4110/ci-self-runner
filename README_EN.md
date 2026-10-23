# runner-kit (self-hosted runner + colima + docker)

**Nix is recommended for development and testing** to pin the controller tools
through `flake.lock`. Run `nix develop -c ci-self-test` to check this repository,
or run `sandbox --project-dir ...` from `nix develop` to execute a target workflow.
The target's dependencies remain controlled by its original workflow; they are
Nix-pinned only when that workflow invokes Nix.

**Native x86_64 Linux hardware has not been tested.** Runtime evidence covers
the ARM64 macOS development shell, Postal Converter's `launcher` job on ARM64
Linux, and the same job in an emulated x86_64 Ubuntu VM. That job uses
`actions/setup-go`; it does not validate the target's complete Nix job. The VM's
Nix job was interrupted during a long build and remains incomplete. Use ARM64
for daily preflight and GitHub Actions for final x86_64 validation.

## Test a project by path in Ubuntu + Nix

```bash
cd ~/dev/ci-self-runner
nix develop
ci-self sandbox --project-dir /path/to/project --workflow .github/workflows/ci.yml --list
ci-self sandbox --project-dir /path/to/project --workflow .github/workflows/ci.yml --job test --plan
ci-self sandbox --project-dir /path/to/project --workflow .github/workflows/ci.yml --job test
```

This copies the current Git worktree into act-managed Ubuntu + Nix containers,
including uncommitted changes. The original project is not mounted or modified.
Workflows control their own dependency setup; Nix is available without rewriting
their commands. A reachable Docker Engine is required. See
[the execution contract and limitations](docs/ci/SANDBOX.md).

## Native ARM64 preflight on an Apple Silicon Mac

With an existing ARM64 Colima instance:

```bash
nix develop
env -u DOCKER_HOST DOCKER_CONTEXT=colima bash ops/ci/ci_self.sh sandbox \
  --project-dir /path/to/project --workflow .github/workflows/ci.yml \
  --job test --arch linux/arm64 --timeout 10m
```

Project path, workflow and job remain generic parameters. The default architecture
is still x64; ARM64 is an explicit preflight choice, with final x64 acceptance on
GitHub Actions. The unchanged Postal Converter launcher job passed in about 51 s
first / 11 s repeat on this Mac. See [measurement conditions](docs/ci/SANDBOX.md#native-arm64-preflight-on-this-mac).

## Use a full x86_64 Ubuntu VM on this Mac

```bash
nix develop .#vm
ci-self vm start
ci-self sandbox --vm ci-self-x64 --nix-cache --project-dir /path/to/project --workflow .github/workflows/ci.yml --job test
ci-self vm status
ci-self vm stop
```

The dedicated Lima VM runs an x64 kernel, independently of Colima.
`--nix-cache` persists a project-specific Nix store/evaluation cache inside the
VM. The VM stays running after the workflow. See
[setup, measured results and limitations](docs/ci/X64_VM.md). CPU emulation has
an overhead; this does not promise hosted Actions parity or equal performance.

## Develop and test this project with Nix

With Nix installed and `nix-command` / `flakes` enabled, run from the checkout root:

```bash
nix develop -c ci-self-test
```

The lock file pins Nixpkgs, providing Go 1.25, Bash, Git, act, and Docker CLI. `ci-self-test`
checks formatting, runs `go vet ./...`, then `go test ./...`, stopping on failure.
The first run downloads dependencies. For an interactive shell, use `nix develop`.
The current working tree, including uncommitted changes, is tested in place.
While the new flake files are untracked, register their existence using
`git add -N flake.nix flake.lock` so Nix can see them (no commit required).
Go's normal caches are reused; automatic toolchain downloads are disabled.

This is a host development shell, not VM/container isolation or Linux emulation.
Definitions cover ARM64/x86_64 macOS and Linux; runtime verification covers ARM64
macOS only. Unit tests include mocks for SSH, GitHub, and Docker and do not verify
live runner registration, remote connections, or Docker/Colima operation.

`runner-kit` is a macOS-oriented toolkit for operating a self-hosted GitHub Actions runner with Colima and Docker.

## Quick Start

Install the CLI once:

```bash
cd ~/dev/ci-self-runner
bash ops/ci/install_cli.sh
```

Run it from the CI target repository:

```bash
cd ~/dev/<target-repo>
ci-self up
```

If you want to use `ci-self act` for rough local timing, install it first with `brew install act`.

`ci-self up` runs `register + run-focus` in sequence.

## Use A Remote CI Runner In One Command

- Machine A: the box that hosts the self-hosted runner, Colima, and Docker
- Machine B: the box where you normally write code

Think of Machine B as your desk, and Machine A as the workshop.
`remote-ci` moves your current worktree from the desk to the workshop, runs verification there, then brings back only the results.

```bash
ci-self remote-ci --host <user>@<machine-a-host> -i ~/.ssh/id_ed25519_for_ci_runner --project-dir '~/dev/<target-repo>' --repo <owner>/<repo>
```

What `remote-ci` does:

1. Verifies SSH public-key auth
2. Syncs the local worktree from Machine B to `--project-dir` on Machine A
3. Runs bootstrap on Machine A only when `--repo` is set and remote `gh auth status` succeeds
4. Runs the bundled verify wrapper over SSH on Machine A
5. Collects `verify-full.status` and `out/logs` into `out/remote/<host>/` on Machine B

Defaults during sync:

- Generated directories such as `target/`, `dist/`, `node_modules/`, `.venv/`, `coverage/`, and `.next/` are excluded
- `.git/` is excluded by default
- `rsync --info=progress2` is used when available
- If local `rsync` is too old, it falls back to `-h --progress`
- Use `--sync-git-dir` only when your build or tests need Git metadata directly

Notes:

- If remote `gh` is missing or not authenticated, bootstrap is skipped instead of treated as a hard failure
- Verification itself still continues even when bootstrap is skipped
- `remote-ci` syncs the current repository by default; use `--local-dir <path>` when you want to sync a different local path

## First-Time Setup For Machine B -> Machine A

Generate an SSH key on Machine B if needed:

```bash
ssh-keygen -t ed25519 -a 100
```

Append the public key to `~/.ssh/authorized_keys` on Machine A:

```bash
cat ~/.ssh/id_ed25519_for_ci_runner.pub | ssh -i ~/.ssh/id_ed25519_for_ci_runner <user>@<machine-a-host> 'mkdir -p ~/.ssh && chmod 700 ~/.ssh && cat >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys'
```

Confirm passwordless SSH:

```bash
ssh -i ~/.ssh/id_ed25519_for_ci_runner -o BatchMode=yes -o PasswordAuthentication=no -o KbdInteractiveAuthentication=no <user>@<machine-a-host> true
```

Then run:

```bash
ci-self remote-ci --host <user>@<machine-a-host> -i ~/.ssh/id_ed25519_for_ci_runner --project-dir '~/dev/<target-repo>' --repo <owner>/<repo>
```

## rsync Note

macOS ships an older `rsync` in some environments.

```bash
rsync --version
```

Recommended:

```bash
brew install rsync
```

An `alias rsync=...` is often not enough for the `ci-self` bash script. Prefer putting Homebrew first in `PATH`.

Apple Silicon example:

```bash
echo 'export PATH="/opt/homebrew/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
```

## Can I Use It Away From Home?

Sometimes, yes. The real requirement is not “same LAN”, but “SSH reachability”.

- Works when Machine A is reachable over SSH
- Common setups: Tailscale, VPN, port forwarding, fixed-IP home network
- Does not work when no SSH route exists

`remote-ci` does not create tunnels or expose the machine by itself.

## What It Can And Cannot Do

- Can: run any target repository on Machine A by changing `--project-dir` and `--repo`
- Can: sync uncommitted local changes and verify them remotely
- Can: collect `verify-full.status` and `out/logs` back to Machine B
- Can: use the same command both on a local network and remotely, as long as SSH works
- Cannot yet: run over password-based SSH
- Cannot yet: create SSH reachability for you when no route exists
- Cannot yet: auto-detect or auto-route multiple repositories without explicit config

## Optional Config File

`ci-self` auto-loads `.ci-self.env`.

Create it with:

```bash
ci-self config-init
```

Example:

```env
CI_SELF_REPO=<owner>/<repo>
CI_SELF_REF=main
CI_SELF_PROJECT_DIR=/Users/<you>/dev/<target-repo>
CI_SELF_REMOTE_HOST=<you>@ci-runner.local
CI_SELF_REMOTE_PROJECT_DIR=/Users/<you>/dev/<target-repo>
CI_SELF_REMOTE_IDENTITY=/Users/<you>/.ssh/id_ed25519_for_ci_runner
CI_SELF_PR_BASE=main
```

## Run Targeted Verify Jobs Locally With act

If you want to run only selected jobs locally without `gh workflow run` and get rough local timing, use the `act` path.

```bash
brew install act
cd ~/dev/<target-repo>
ci-self act
ci-self act --list
ci-self act --job <job-id>

# Or point at the repo explicitly from anywhere
ci-self act --project-dir ~/dev/<target-repo> --job <job-id>
```

**These timings are local estimates only. Actual duration on GitHub Actions, `remote-ci`, or a real self-hosted runner may differ.**

Notes:

- `ci-self act` looks at `.github/workflows/*.yml|*.yaml` inside the target repo
- If you omit `--workflow` and the repo has multiple workflows, it opens a shell prompt asking which workflow to run; press `q` to quit
- Start with `ci-self act --list`, then run `--job <job-id>`
- The workflow menu number is separate from `--job`; pass a real job id such as `verify` or `verify-lite`
- For `~/dev/maakie-brainlab`, use `ci-self act --project-dir ~/dev/maakie-brainlab --list` and then `ci-self act --project-dir ~/dev/maakie-brainlab --job verify`
- It prints `elapsed_sec` plus `benchmark_started_at` / `benchmark_finished_at`, and stores artifacts under `out/act-artifacts/`
- Live log lines are prefixed with `[YYYY MM/DD HH:MM:SS]`
- It does not require `SELF_HOSTED_OWNER` or `gh auth`
- If the repo has no workflow files yet, add `.github/workflows/*.yml` first
- If your existing workflow is old, refresh it with `bash ops/ci/scaffold_verify_workflow.sh --repo <target> --apply --force`
- When `scaffold_verify_workflow.sh --apply` runs from a TTY, it asks for `[y/N]` confirmation before creating or overwriting `verify.yml`

Keep in mind:

- `act` is useful for rough local timing and early failure detection, but it is not a full GitHub Actions replica
- If your workflow does not include a `github.event.act == true` bypass, owner guards may skip the job locally
- `verify-full-dryrun` still depends on local Docker/Colima reachability

## Main Commands

- `ci-self up`: fastest local path (`register + run-focus`)
- `ci-self act`: run a selected verify workflow/job locally via `act` for rough timing
- `ci-self focus`: runs `run-focus`, creates a PR if missing, then watches checks
- `ci-self mobile-workflow`: scaffolds a fastlane-based `mobile-build.yml`
- `ci-self remote-ci`: SSH-required sync + remote verify + result collection in one command
- `ci-self doctor --fix`: checks dependencies, `gh auth`, Colima, Docker, and runner health
- `ci-self update`: checks the runner and related tools such as `act`, `gh`, and Colima for available updates
- `ci-self update --apply`: explicitly upgrades already-installed Homebrew-managed tools only
- `ci-self remote-up`: older SSH path for `register + run-focus` without syncing
- `ci-self config-init`: generates a `.ci-self.env` template

Note: the GitHub Actions runner itself is left to its built-in auto-update path; `ci-self update --apply` does not force-upgrade it.

## Mobile Build (fastlane)

Use `--mobile-profile` to add iOS / Android runner capability labels.

```bash
ci-self register --mobile-profile ios
ci-self register --mobile-profile android
ci-self register --mobile-profile all
```

In a target mobile app repository, scaffold the `main` / `develop` workflow:

```bash
ci-self mobile-workflow --apply
```

See:

- `docs/ci/MOBILE_LABELS.md`
- `docs/ci/MOBILE_SECRETS_POLICY.md`
- `docs/ci/MAC_BUILD_HOST_RUNBOOK.md`

## Security Assumptions

- Intended for single-owner personal operation
- Self-hosted execution is gated by `SELF_HOSTED_OWNER`
- For external collaborators or fork PRs, review `docs/ci/SECURITY_HARDENING_TASK.md` first

## More

- `README.md`
- `docs/ci/QUICKSTART.md`
- `docs/ci/RUNBOOK.md`
- `docs/ci/MOBILE_LABELS.md`
- `docs/ci/MOBILE_SECRETS_POLICY.md`
- `docs/ci/MAC_BUILD_HOST_RUNBOOK.md`
- `docs/ci/SECURITY_HARDENING_TASK.md`
