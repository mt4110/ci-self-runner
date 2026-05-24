# MAC_BUILD_HOST_RUNBOOK

## 目的

Mac build host を iOS / Android mobile build 用に使うときの初期化、点検、復旧手順を固定する。

## 前提

- runner は専用ユーザで動かす
- self-hosted job は owner guard 付き workflow だけで実行する
- mobile secrets は `docs/ci/MOBILE_SECRETS_POLICY.md` に従う
- runner labels は `docs/ci/MOBILE_LABELS.md` に従う

## 初期セットアップ

1. Xcode をインストールする
2. Command Line Tools を有効化する
3. Android Studio / Android SDK / platform-tools を入れる
4. Ruby は system Ruby 直叩きではなく、mise / rbenv / asdf のどれかで固定する
5. Bundler と fastlane は repo の `Gemfile.lock` に寄せる

確認:

```bash
xcodebuild -version
xcrun simctl list devices available
java -version
sdkmanager --list
ruby -v
bundle -v
bundle exec fastlane --version
```

## runner 登録

iOS only:

```bash
ci-self register --mobile-profile ios
```

Android only:

```bash
ci-self register --mobile-profile android
```

1台で両方:

```bash
ci-self register --mobile-profile all
```

## workflow 雛形

対象アプリ repo で生成する。

```bash
ci-self mobile-workflow --apply
```

既存の `mobile-build.yml` を更新する場合:

```bash
ci-self mobile-workflow --apply --force
```

## fastlane lane の目安

- `ios ci`: build / test / lint。signing しない
- `ios beta`: signing あり。TestFlight / internal 配布向け
- `android ci`: assemble / test / lint。release keystore を使わない
- `android beta`: signing あり。internal track / artifact 配布向け

PR は `ci` lane だけを通す。`beta` / `release` lane は protected branch または environment approval に寄せる。

## 日常点検

```bash
ci-self doctor --fix
df -h
xcrun simctl delete unavailable
brew outdated
bundle check
```

月1回:

- Xcode / Android SDK / Ruby / fastlane の更新候補を確認
- 更新する場合は1つずつ変更し、失敗時に戻せるログを残す
- `docs/ci/HOST_CHANGELOG_TEMPLATE.md` に変更履歴を残す

## よくある復旧

### Xcode の選択が壊れた

```bash
sudo xcode-select -s /Applications/Xcode.app/Contents/Developer
sudo xcodebuild -license accept
```

### Simulator が詰まった

```bash
xcrun simctl shutdown all || true
xcrun simctl erase all
xcrun simctl delete unavailable
```

### Android SDK が見つからない

```bash
export ANDROID_HOME="$HOME/Library/Android/sdk"
export PATH="$ANDROID_HOME/platform-tools:$ANDROID_HOME/cmdline-tools/latest/bin:$PATH"
sdkmanager --list
```

### Bundler / fastlane が壊れた

```bash
rm -rf vendor/bundle
bundle install
bundle exec fastlane --version
```

## 緊急停止

1. GitHub UI で runner を disable にする
2. Mac 側で runner service を停止する
3. signing 関連 Secret / certificate / key を失効する
4. Actions logs / artifacts を確認する
5. runbook に時刻、影響範囲、復旧手順を記録する
