# COLIMA_TUNING（32GB確定版 / ホストLLM推論前提 / Docker Desktop不使用）

## 前提（このファイルの固定条件）

- Mac mini RAM: **32GiB**
- LLM推論は **ホスト(macOS)** で行う（= ホスト側にメモリ余白が必要）
- Docker Desktop は使わない。**colima + docker** を使う
- 速さはCPU/RAM盛りより **キャッシュ設計 + I/O** で稼ぐ
- 数値は “固定 → 計測 → 最小限の補正”。むやみに動かさない

---

## 採用プロファイル（固定）

### ✅ CIデフォ（常用・安定優先）

- Colima: **CPU=10 / Memory=16GiB / Disk=120GiB**
- 同時実行: **1（原則固定）**
- 目的: ホストに余白を残し、LLM推論中でもスワップ地獄を避ける

### ⚠️ 例外プロファイル（短時間のみ・記録必須）

- Colima: **CPU=10 / Memory=20GiB / Disk=120GiB**
- 使用条件:
  - コンテナ内の重い処理（ビルド/テスト）が明確にメモリ不足で落ちる
  - もしくは “分割してもなお” 20GiB が必要と判断できる
- 運用ルール:
  - 例外で切り替えたら **docs/ci/RUNBOOK.md に理由を1行追記**してから行う
  - 恒常運用にしない（原則 16GiB に戻す）

---

## 起動（推奨・CIデフォ）

> `vz` が使える前提。使えない場合は `--vm-type=vz` を外す。
> `virtiofs` は bind mount I/O 改善のために推奨。

colima stop || true
colima start --cpu 10 --memory 16 --disk 120 --vm-type=vz --mount-type=virtiofs

## 例外起動（20GiB）

colima stop || true
colima start --cpu 10 --memory 20 --disk 120 --vm-type=vz --mount-type=virtiofs

---

## 状態確認（最小）

colima status
colima list
docker info | sed -n '1,80p'

---

## “遅い/重い”の診断順（順番厳守）

> 先にCPU/RAMを盛るのは禁止。まず原因を見る。

1) **キャッシュ（/cache）**
   - Go build cache / npm cache が volume に乗っているか
   - 毎回フルビルドしていないか（キャッシュが効いていない兆候）

2) **bind mount のI/O**
   - 大量小ファイル（例: node_modules）が足を引っ張っていないか
   - 可能なら node_modules をコンテナ内に閉じる/分離する

3) **処理分割**
   - “重いと察知したら分割” を優先
   - 例: build/test/bundle を段階化、テストをスコープ分割

4) **最後に Colima 補正**
   - OOMが出るなら 20GiB へ（例外運用として記録）
   - それでも厳しいなら “並列を増やす” ではなく “分割” を選ぶ

---

## 補正ルール（固定運用）

### 16GiBで運用できているかの基準

- verify-full が安定して完走
- スワップが暴れない（ホストLLM実行中でも破綻しない）

### 20GiBに上げる条件（例外）

- コンテナ内で明確な OOM / memory pressure 由来の失敗が出る
- 分割しても改善しない（= 分割余地が薄い）

---

## ディスク運用（破綻防止）

docker system df

### 注意

- 肥大化を見たら、まず原因を1行メモしてから対処する
- “無思考 prune” はしない（やるなら RUNBOOK に理由を残す）

```bash
docker system prune -f  # ←実行するなら Runbook に理由を残す
```
