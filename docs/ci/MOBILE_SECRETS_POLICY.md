# MOBILE_SECRETS_POLICY

## 原則

- signing material は repo に置かない
- PR では signing をしない。PR は `fastlane ios ci` / `fastlane android ci` のような検証 lane に寄せる
- 配布・署名・ストア連携は `push` to `main` / `develop` または手動 `workflow_dispatch` に限定する
- Secret は GitHub Actions Secrets / Environments で管理し、ログへ値を出さない
- runner label は能力だけを示す。secret の保有状態は label に含めない

## repo に置かないもの

| 種別 | 例 |
|---|---|
| iOS signing | `.p12`, `.mobileprovision`, `.provisionprofile`, App Store Connect API key `.p8` |
| Android signing | `.jks`, `.keystore`, `key.properties`, Play Console service account JSON |
| fastlane local env | `fastlane/.env`, `fastlane/.env.*`, `.env.mobile` |

## 推奨 Secret 名

iOS:

- `IOS_CERTIFICATE_BASE64`
- `IOS_CERTIFICATE_PASSWORD`
- `IOS_PROVISIONING_PROFILE_BASE64`
- `APP_STORE_CONNECT_API_KEY_ID`
- `APP_STORE_CONNECT_ISSUER_ID`
- `APP_STORE_CONNECT_API_KEY_BASE64`

Android:

- `ANDROID_KEYSTORE_BASE64`
- `ANDROID_KEYSTORE_PASSWORD`
- `ANDROID_KEY_ALIAS`
- `ANDROID_KEY_PASSWORD`
- `GOOGLE_PLAY_SERVICE_ACCOUNT_JSON`

fastlane:

- `FASTLANE_SESSION` は原則使わない。必要な場合は短命にして、使用後にローテーションする
- `MATCH_PASSWORD` を使う場合は protected branch / protected environment に閉じる

## workflow policy

- `pull_request` は build/test のみ
- `push` to `main` / `develop` は signing を許可できる
- `workflow_dispatch` は platform / lane を選べるが、release lane は environment approval を通す
- `pull_request_target` は使わない
- fork PR は self-hosted runner で実行しない

## 漏えい時の対応

1. 該当 Secret / certificate / key を失効または削除する
2. GitHub Secrets / Environments を更新する
3. Actions log / artifact / PR comment / chat の露出を確認する
4. 署名済み artifact がある場合は配布先で revoke / expire を確認する
5. `docs/ci/HOST_CHANGELOG_TEMPLATE.md` 形式で対応ログを残す

## verify-lite scan

`cmd/verify-lite` は次を検出対象にする。

- Discord / Slack webhook URL
- private key block
- Google service account JSON の private key
- mobile signing file names

検出された場合は、ファイルを repo から除去し、Secret 管理へ移す。
