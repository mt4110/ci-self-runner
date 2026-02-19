# v0.1.0 Release Notes

## Title

v0.1.0 Initial Public Release

## 日本語本文

self-hosted runner + colima + docker 前提の CI 運用キットを初回公開しました。

主な内容:
- Go中心実装（Shellは極薄ラッパ）
- verify-lite / verify-full の実行契約（`/repo`, `/out`, `/cache`）
- `OK:` / `SKIP:` / `ERROR:` ログ契約
- Discord通知（失敗時）
- review-pack（core/optional）による ChatGPT/Gemini 向け証跡パック生成
- out配下の自動GC（最新5件保持）
- docs/ci 一式（SYSTEM/FLOW/RUNBOOK/COLIMA_TUNING/SHELL_POLICY/SECRETS_POLICY）

注意:
- Docker Desktop は使用せず、colima を利用
- self-hosted runner で fork/external PR は実行しない

## English Body

Initial public release of the CI runner kit for self-hosted macOS runners using colima + docker.

Highlights:
- Go-first implementation (shell scripts are thin wrappers only)
- verify-lite / verify-full execution contract (`/repo`, `/out`, `/cache`)
- Unified log contract with `OK:` / `SKIP:` / `ERROR:` lines
- Discord notifications on workflow failures
- review-pack (core/optional) for ChatGPT/Gemini review bundles
- Automatic GC for runtime artifacts (keep latest 5)
- Full docs under `docs/ci` (SYSTEM/FLOW/RUNBOOK/COLIMA_TUNING/SHELL_POLICY/SECRETS_POLICY)

Notes:
- Uses colima (no Docker Desktop)
- No fork/external PR execution on self-hosted runners
