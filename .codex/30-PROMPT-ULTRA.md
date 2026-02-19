あなた（Codex）はこのリポジトリの実装補助。最初に以下を読み、厳守すること。

必読:
- .codex/00-RULES-READ-FIRST.md
- docs/ci/SYSTEM.md
- docs/ci/FLOW.md
- docs/ci/RUNNER_ISOLATION.md
- docs/ci/COLIMA_TUNING.md
- docs/ci/SHELL_POLICY.md
- docs/ci/RUNBOOK.md

絶対禁止（差し戻し）:
- リポジトリ名変更・勝手なrename
- codex/ ディレクトリ作成（禁止）
- k8s/k9s/nix など大規模導入
- Shellで分岐/ループ/パース/状態管理（SHELL_POLICY違反）
- 既存ファイルの全面置換、巨大リファクタ

最重要ポリシー:
- 止まらない設計。失敗は ERROR: を出して以降へ進まない（プロセスは落とさない）
- 判定は出力テキスト（OK/SKIP/ERROR）で行う
- Docker Desktopは使わず colima 前提
- 重い処理は分割して timebox を付ける

作業開始時の出力（必須）:
1) Plan（3〜7ステップ、各DONE条件）
2) 変更対象ファイル一覧（追加/更新）
3) 1ステップ=小差分
4) 各ステップの検証コマンド

Rust基盤の実装原則:
- Rustは司令塔（state・分割・順序・監査ログ）
- 実処理は外部コマンドでGo資産を呼ぶ
- stateは `.local/ci/state.json`
- ログは `.local/out/run/<run_id>/`
- 1行判定: `OK: <step> ...` / `SKIP: <step> reason=...` / `ERROR: <step> reason=...`
