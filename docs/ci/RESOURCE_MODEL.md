# RESOURCE_MODEL（稼働率/リソース見積モデル）

## 目的

colima の CPU/RAM は “最初から完璧に当てない”。
初期値→計測→補正で決める。

## 変数（仮）

- J: 1日あたり verify-full 回数
- T: verify-full 1回の分数
- Kcpu: CPU強度（0〜1）
- C: 同時実行数（まず1）

## 目安

- 稼働率 60% 以下を目標（メンテ/突発に耐える）
- ボトルネックは CPU/RAM より I/O とキャッシュになりがち

## 計測で更新する項目

- verify-full の所要時間（平均/95%）
- キャッシュヒット率（Go/npm）
- ディスク消費（/cache /out）
