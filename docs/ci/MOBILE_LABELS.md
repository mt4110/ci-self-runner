# MOBILE_LABELS

## 目的

mobile build を通常の `verify-full` と分離し、iOS / Android / fastlane の実行先をラベルで明示する。

## ラベル設計

| profile | 追加ラベル | 用途 |
|---|---|---|
| `none` | なし | 既定の verify runner |
| `ios` | `mobile,ios,fastlane,xcode` | Xcode / iOS Simulator / fastlane iOS |
| `android` | `mobile,android,fastlane,android-sdk` | Android SDK / Gradle / fastlane Android |
| `all` | `mobile,ios,android,fastlane,xcode,android-sdk` | 1台のMac build hostで両方を扱う |

既定ラベルは維持する。

```text
self-hosted,mac-mini,colima,verify-full
```

`--mobile-profile all` の場合は次になる。

```text
self-hosted,mac-mini,colima,verify-full,mobile,ios,android,fastlane,xcode,android-sdk
```

## 登録例

```bash
ci-self register --mobile-profile ios
ci-self up --mobile-profile all
go run ./cmd/runner_setup --apply --repo <owner/repo> --mobile-profile android
```

## workflow 側の要求ラベル

iOS:

```yaml
runs-on:
  - self-hosted
  - mac-mini
  - mobile
  - ios
  - fastlane
  - xcode
```

Android:

```yaml
runs-on:
  - self-hosted
  - mac-mini
  - mobile
  - android
  - fastlane
  - android-sdk
```

## 判断基準

- `mobile` は mobile build host の総称として使う
- `ios` / `android` は toolchain の存在を示す
- `fastlane` は Ruby fastlane 実行環境の存在を示す
- signing secret の有無を runner label で表現しない
- `release` / `production` のような配布先ラベルは workflow environment で管理する

## submodule について

最初から fastlane 共通処理を submodule にしない。

理由:

- signing / provisioning / Gradle / Xcode 設定はアプリ repo に密結合しやすい
- submodule は clone / checkout / review の手触りを悪くしやすい
- CIが壊れた時に「親repoの問題か、submoduleの参照か」の切り分けが増える

共通化が必要になったら、順番は次がよい。

1. この repo の workflow 雛形を各アプリ repo に展開する
2. 重複した fastlane lane だけを小さな Ruby helper / private gem に切り出す
3. 複数アプリで同じ release train を持つ段階で、専用 repo 化を検討する
