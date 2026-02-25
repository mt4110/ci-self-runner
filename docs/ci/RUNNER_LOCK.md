# RUNNER_LOCK（Runner バージョン固定 SOT）

## 固定バージョン

- **actions/runner v2.321.0**
- リリース日: 2024-10-01
- GitHub 最低要件: v2.317.0 以上

## SHA256 ハッシュ

| アーキテクチャ | ファイル名 | SHA256 |
|---|---|---|
| osx-x64 | `actions-runner-osx-x64-2.321.0.tar.gz` | `(リリースページで取得して記入)` |
| osx-arm64 | `actions-runner-osx-arm64-2.321.0.tar.gz` | `(リリースページで取得して記入)` |

## ダウンロード URL

```
https://github.com/actions/runner/releases/download/v2.321.0/actions-runner-osx-x64-2.321.0.tar.gz
https://github.com/actions/runner/releases/download/v2.321.0/actions-runner-osx-arm64-2.321.0.tar.gz
```

## 更新ポリシー

- Runner バージョンはこのファイルで固定する（SOT）
- 更新時は sha256 ハッシュも同時に更新すること
- `cmd/runner_setup` はこのファイルの値をハードコードで参照する
- GitHub が最低バージョンを引き上げた場合のみ更新を検討する

## 検証コマンド

```bash
# ダウンロード後の検証
shasum -a 256 actions-runner-osx-arm64-2.321.0.tar.gz
# 上記の出力がこのファイルの SHA256 と一致すること
```

## 参考

- [actions/runner releases](https://github.com/actions/runner/releases)
- [GitHub の runner 最低要件](https://docs.github.com/en/actions/hosting-your-own-runners)
