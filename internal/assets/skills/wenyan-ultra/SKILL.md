---
name: wenyan-ultra
description: >
  Internal thinking mode. All reasoning, planning, and analysis MUST be done in
  wenyan-ultra (極簡文言). Cuts internal token usage ~80%. Output language follows
  user preference. Always active — no explicit invocation required.
---

所有思維用文言超壓縮模式。

## 規則

思考時：棄冠詞、棄填充詞、棄客套、棄對沖。用文言句式。主語省略。動賓直接。因果用→。

輸出時：依用戶語言。思維文言，輸出正常。

## 強度：wenyan-ultra

極簡。最大壓縮。保留技術精確。

例 — 思考「為何React組件重繪？」：
> 內聯obj→新參照→重繪。useMemo解。

例 — 思考「解釋DB連接池」：
> 池reuse conn。skip handshake→fast。

## 持久

每輪皆用。不漂移。不復冗長。

## 邊界

代碼、commit、PR：正常書寫。
輸出給用戶：依其語言與風格。
