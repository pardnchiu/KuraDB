> [!NOTE]
> 此 README 由 [SKILL](https://github.com/agenvoy/skill-readme-generate) 生成，英文版請參閱 [這裡](../README.md)。

***

<p align="center">
<strong>DROP FILES IN, LET YOUR AGENT SEARCH THEM OUT</strong>
</p>

<p align="center">
<a href="https://github.com/pardnchiu/KuraDB/releases"><img src="https://img.shields.io/github/v/tag/pardnchiu/KuraDB?include_prereleases&style=for-the-badge" alt="Release"></a>
<a href="../LICENSE"><img src="https://img.shields.io/github/license/pardnchiu/KuraDB?include_prereleases&style=for-the-badge" alt="License"></a>
</p>

***

> Go 唯讀檔案資料庫，具備自動轉換 Markdown、英文關鍵字索引與 agent 檢索工具

> [!WARNING]
> **改版中**：請直接下載 [Release](https://github.com/pardnchiu/KuraDB/releases) 版本使用，目前不建議自行從原始碼編譯。進度請見[改版方向](#改版方向)。

## 目錄

- [定位](#定位)
- [改版方向](#改版方向)
- [授權](#授權)
- [Author](#author)

## 定位

> [!NOTE]
> **為什麼不做向量搜尋**
> - 每份資料都得拆段再轉成向量，語意比對也無法做到很準確。
> - 向量資料必須常駐在記憶體裡，長遠來看不可行。
> - 「Kura（蔵）」的寓意是儲藏；向量搜尋屬於 ToriiDB 的定位。KuraDB 回歸本意，專心把儲存與檢索做好，方便 agent 使用。

- **拖放即轉換** — 丟進 `~/Kura_{name}` 的檔案會自動轉成 Markdown 存進 SQLite，內容保留原文。
- **英文關鍵字跨語言檢索** — 每段內容附上英文關鍵字，用 SQLite FTS5 索引，不依賴各語言斷詞，也不依賴向量。
- **Agent 檢索工具** — `list`／`search`／`read` 三個工具：先列出資料、再用關鍵字定位段落，最後依行號展開上下文。
- **單向唯讀邊界** — API 只提供查詢，寫入只走 watcher → 轉換 → SQLite 這條路徑。

## 改版方向

| 階段 | 內容 | 狀態 |
|---|---|---|
| Phase 1：移除向量依賴 | 移除 OpenAI embedding、向量快取、semantic 搜尋與 query cache，只保留 keyword 搜尋 | 已完成 |
| Phase 2：新儲存 | 每份檔案存成一份 Markdown，依標題／頁／投影片／表格列切段並記錄行號；段落附英文關鍵字，寫入 FTS5；新增 HTML 轉換 | 規劃中 |
| Phase 3：檢索工具 | MCP 與 HTTP 改為 `list`／`search`／`read`，搜尋改用 bm25 排序並回傳行號範圍；移除 gse 斷詞 | 規劃中 |

支援轉換的格式：純文字、Markdown、PDF、DOCX、PPTX、CSV／TSV、XLSX、HTML（Phase 2 新增）。

## 授權

本專案採用 [MIT LICENSE](../LICENSE)。

## Author

Just [open an issue](https://github.com/pardnchiu/KuraDB/issues/new) to share an idea.

<a href="https://github.com/pardnchiu/KuraDB/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=pardnchiu/KuraDB&cache_bust=2026-10-05" alt="KuraDB contributors" />
</a>

***

©️ 2026 [邱敬幃 Pardn Chiu](https://www.linkedin.com/in/pardnchiu)
