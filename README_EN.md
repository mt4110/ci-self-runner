# runner-kit (self-hosted runner + colima + docker)

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

## Main Commands

- `ci-self up`: fastest local path (`register + run-focus`)
- `ci-self focus`: runs `run-focus`, creates a PR if missing, then watches checks
- `ci-self remote-ci`: SSH-required sync + remote verify + result collection in one command
- `ci-self doctor --fix`: checks dependencies, `gh auth`, Colima, Docker, and runner health
- `ci-self remote-up`: older SSH path for `register + run-focus` without syncing
- `ci-self config-init`: generates a `.ci-self.env` template

## Security Assumptions

- Intended for single-owner personal operation
- Self-hosted execution is gated by `SELF_HOSTED_OWNER`
- For external collaborators or fork PRs, review `docs/ci/SECURITY_HARDENING_TASK.md` first

## More

- `README.md`
- `docs/ci/QUICKSTART.md`
- `docs/ci/RUNBOOK.md`
- `docs/ci/SECURITY_HARDENING_TASK.md`
