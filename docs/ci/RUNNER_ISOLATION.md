# RUNNER_ISOLATION（隔離設計）

## 目的

self-hosted runner は “状態が溜まる” のが弱点。汚染を防ぐ。

## 推奨（最低ライン）

- runner 専用ユーザ `ci` を作る（作業ユーザと分離）
- runner の作業領域は固定パスに閉じる（例: /Users/ci/ci-root）
- 検証は docker コンテナ内で行い、ホスト依存を減らす

## 禁止（事故が増える）

- dev環境（brew更新、IDE等）を runner ユーザに混ぜる
- 外部PRを runner で実行する（fork PR）
