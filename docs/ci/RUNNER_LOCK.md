# RUNNER_LOCK（Runner バージョン固定 SOT）

## 固定バージョン

- **actions/runner v2.334.0**
- リリース日: 2026-04-21
- GitHub 最低要件: v2.329.0 以上を目安に監視（GitHub.com の登録時要件は変更される可能性あり）

## SHA256 ハッシュ

| アーキテクチャ | ファイル名 | SHA256 |
|---|---|---|
| osx-x64 | `actions-runner-osx-x64-2.334.0.tar.gz` | `73a979ff7e9ce8a70244f3a959d896870be486fac92bb08ed90684f961474e0d` |
| osx-arm64 | `actions-runner-osx-arm64-2.334.0.tar.gz` | `760899b29fd4e942076bcd1160a662bf83c15d9ce8a8cc466763aec7e582b21b` |

## ダウンロード URL

```
https://github.com/actions/runner/releases/download/v2.334.0/actions-runner-osx-x64-2.334.0.tar.gz
https://github.com/actions/runner/releases/download/v2.334.0/actions-runner-osx-arm64-2.334.0.tar.gz
```

## 更新ポリシー

- Runner バージョンはこのファイルで固定する（SOT）
- 更新時は sha256 ハッシュも同時に更新すること
- `cmd/runner_setup` はこのファイルの値をハードコードで参照し、未設定なら失敗させる
- GitHub runner 本体の自動更新は有効のままにする
- `ci-self update` で既存 runner と周辺ツールの更新候補を確認する
- GitHub が最低バージョンを引き上げた場合、または最新との差分が出た場合は更新を検討する

## 検証コマンド

```bash
# ダウンロード後の検証
shasum -a 256 actions-runner-osx-arm64-2.334.0.tar.gz
# 上記の出力がこのファイルの SHA256 と一致すること
```

## 参考

- [actions/runner releases](https://github.com/actions/runner/releases)
- [GitHub の runner 最低要件](https://docs.github.com/en/actions/hosting-your-own-runners)
