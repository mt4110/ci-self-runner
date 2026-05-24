# RUNNER_LOCK（Runner バージョン固定 SOT）

## 固定バージョン

- **actions/runner v2.321.0**
- リリース日: 2024-11-13
- GitHub 最低要件: v2.317.0 以上

## SHA256 ハッシュ

| アーキテクチャ | ファイル名 | SHA256 |
|---|---|---|
| osx-x64 | `actions-runner-osx-x64-2.321.0.tar.gz` | `b2c91416b3e4d579ae69fc2c381fc50dbda13f1b3fcc283187e2c75d1b173072` |
| osx-arm64 | `actions-runner-osx-arm64-2.321.0.tar.gz` | `fbee07e42a134645d4f04f8146b0a3d0b3c948f0d6b2b9fa61f4318c1192ff79` |

## ダウンロード URL

```
https://github.com/actions/runner/releases/download/v2.321.0/actions-runner-osx-x64-2.321.0.tar.gz
https://github.com/actions/runner/releases/download/v2.321.0/actions-runner-osx-arm64-2.321.0.tar.gz
```

## 更新ポリシー

- Runner バージョンはこのファイルで固定する（SOT）
- 更新時は sha256 ハッシュも同時に更新すること
- `cmd/runner_setup` はこのファイルの値をハードコードで参照し、未設定なら失敗させる
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
