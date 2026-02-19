# SYSTEM（全体アーキテクチャ）

## 目的

- GitHub Actions の実行を最小化し、計算はローカル（Mac mini）へ寄せる
- 検証環境は colima + docker で固定し、再現性を上げる
- GitHub は「公証台帳」（PR/レビュー/証拠保管）に徹する

## コンポーネント

- Workstation: MacBook（編集、verify-lite、PR作成）
- CI Host: Mac mini（self-hosted runner、verify-full、bundle生成）
- GitHub: PR作成/履歴/レビュー（verify-onlyを最小で）

## 設計原則（バランス）

- 既存の GitHub runner は使う（デーモン自作は原則しない）
- “速さ”はCPU増量よりキャッシュ設計で稼ぐ
- 破綻防止は「隔離」「契約」「runbook」で担保する
