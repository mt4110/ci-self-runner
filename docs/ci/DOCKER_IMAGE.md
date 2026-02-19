# DOCKER_IMAGE（Dockerfile契約）

## 目的

決定論・再現性・汚染防止。

## 契約

- /repo : リポジトリ（bind mount）
- /out  : 生成物（logs/bundle等）
- /cache: ビルドキャッシュ（named volume）

## 方針

- マルチステージ（builder / runtime）
- ツールバージョン固定は versions.lock で宣言
- “大きい処理”は分割しやすい entrypoint を用意する

## mise を使う場合の必須手順

- Dockerfile で `mise` を利用する場合は、`mise install` の前に必ず `mise trust` を実行する
- 推奨順序: `mise trust` -> `mise install` -> バージョン確認
