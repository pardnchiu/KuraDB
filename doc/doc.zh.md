# kuradb - 技術文件

> 返回 [README](./README.zh.md)

## 前置需求

- Go 1.25.1 或更高版本
- CGO 可用的 C 編譯器（`mattn/go-sqlite3` 需要）
- OpenAI API Key（`text-embedding-3-small`，512 維）
- macOS（APFS/HFS+）或 Linux（ext4/xfs）；不支援 Windows / SMB / NFS / FUSE

## 安裝

### 從原始碼建置

```bash
git clone https://github.com/pardnchiu/KuraDB.git
cd KuraDB
make build
# 輸出 bin/kura
```

### 建置並安裝至 /usr/local/bin

```bash
make app
# 停止舊 daemon → 建置 → sudo 移至 /usr/local/bin/kura → 啟動
```

### 使用 go install

```bash
go install github.com/pardnchiu/kuradb/cmd/app@latest
mv "$(go env GOPATH)/bin/app" "$(go env GOPATH)/bin/kura"
# go install 以目錄名 app 命名，需自行更名為 kura
```

## 設定

### API Key

| 名稱 | 必要 | 說明 |
|------|------|------|
| `OPENAI_API_KEY` | 是 | 依序讀取：系統 keychain（service `kuradb`）→ 環境變數；缺少時 daemon 與 `kura mcp` 啟動即失敗 |

系統 keychain 查找順序：

| 平台 | 順序 |
|------|------|
| macOS | `security find-generic-password -s kuradb -a OPENAI_API_KEY` → 環境變數 |
| Linux | `secret-tool lookup service kuradb account OPENAI_API_KEY` → `~/.config/kuradb/.secrets`（`OPENAI_API_KEY=...`）→ 環境變數 |

keychain 內已有值時，環境變數不會生效。

### 設定目錄 `~/.config/kuradb/`

| 路徑 | 用途 |
|------|------|
| `db.json` | 資料庫註冊表（`db`、`createAt`） |
| `config.json` | 伺服器設定：`port`（固定埠）、`remote`（是否掛載 `/mcp`） |
| `global.db` | 全域 SQLite，存放 `query_cache` |
| `{name}/data.db` | 每庫 SQLite，存放 `file_data` |
| `{name}/inbox/` | 監控目錄 |
| `{name}/record.json` | 檔案系統快照（size + mtime） |
| `endpoint` | daemon 就緒後寫入的 HTTP 位址 |
| `runtime.uid` | daemon 的 PID 與啟動時間 |
| `daemon.log` | daemon stdout / stderr |

每個資料庫在家目錄建立符號連結 `~/Kura_{name}` → `~/.config/kuradb/{name}/inbox/`。

### `config.json`

```json
{
  "port": 8080,
  "remote": true
}
```

| 欄位 | 預設 | 說明 |
|------|------|------|
| `port` | 未設定 → 10000–65535 隨機（最多嘗試 10 次） | 綁定 `127.0.0.1` |
| `remote` | `false` | `true` 時在 HTTP 掛載 `/mcp`；啟動時決定，變更需重啟 |

## 使用方式

### 基礎：建立資料庫並啟動

```bash
kura add my_docs
# db added: my_docs
#   dir:  ~/.config/kuradb/my_docs
#   link: ~/Kura_my_docs

kura
# http://localhost:12345
```

`kura` 會 fork 背景 daemon，於 10 秒內等待 `endpoint` 寫入後印出位址；逾時則提示查看 `daemon.log`。新增資料庫後需重啟 daemon 才會載入。

### 索引檔案

```bash
cp spec.pdf notes.md ~/Kura_my_docs/
# 10 秒內 watcher 偵測變更 → 解析分段 → Upsert 至 SQLite
# 每 5 秒 embedder 取最多 64 個待處理區塊 → OpenAI → 更新向量快取
```

| 類型 | 副檔名 | 處理方式 |
|------|--------|----------|
| PDF | `.pdf` | go-pkg parser |
| Word | `.docx` | go-pkg parser |
| PowerPoint | `.pptx` | go-pkg parser |
| 表格 | `.csv`、`.tsv`、`.xlsx` | 首列為表頭，每 5 列一個區塊（`[n] col=value, ...`） |
| 文字 | 其他副檔名 | 前 8 KiB 為合法 UTF-8 且無 NUL 才以 Markdown 解析器處理 |
| 略過 | 圖片、影音、壓縮檔、執行檔、字型、資料庫檔、`.DS_Store` 等 | 不索引 |

子目錄會遞迴掃描；檔案移除後其區塊標記 `dismiss = TRUE`，不再出現在搜尋結果。

### 查詢 HTTP API

```bash
EP="$(cat ~/.config/kuradb/endpoint)"

curl "$EP/api/health"
# OK

curl "$EP/api/list"
# {"loaded":["my_docs"],"registered":[{"db":"my_docs","createAt":"2026-09-16T00:00:00Z"}]}

curl -G "$EP/api/search" --data-urlencode "db=my_docs" --data-urlencode "q=什麼是RAG" --data-urlencode "limit=5"
```

回應：

```json
{
  "keyword": [
    {"source": "/Users/me/.config/kuradb/my_docs/inbox/notes.md", "matches": [{"chunk": 1, "content": "RAG 是檢索增強生成..."}]}
  ],
  "semantic": [
    {"source": "/Users/me/.config/kuradb/my_docs/inbox/spec.pdf", "matches": [{"chunk": 3, "content": "..."}]}
  ]
}
```

錯誤處理：

```bash
curl -s -w "\n%{http_code}\n" "$EP/api/search?db=missing&q=x"
# {"error":"\"missing\" not exist"}
# 400
```

### 進階：固定埠與遠端 MCP

```bash
kura port set 8080      # 寫入 config.json 並重啟 daemon
kura remote enable      # 掛載 HTTP /mcp；daemon 執行中則自動重啟
kura remote disable
kura port clear         # 下次手動重啟後生效
kura stop               # SIGTERM，5 秒未結束則 SIGKILL
```

### 進階：以 stdio 接入 MCP client

```json
{
  "mcpServers": {
    "kuradb": {
      "command": "kura",
      "args": ["mcp"]
    }
  }
}
```

`kura mcp` 自行開啟 SQLite 並載入向量快取，只讀、不跑 watcher / embedder；daemon 之後新索引的內容需重啟該 MCP session 才看得到。

## 命令列參考

### 指令

| 指令 | 語法 | 說明 |
|------|------|------|
| （無） | `kura` | 啟動背景 daemon 並印出 endpoint |
| `add` | `kura add <name>` | 註冊資料庫，建立 inbox 與 `~/Kura_{name}` 連結；名稱內空白轉為 `_` |
| `list` | `kura list` | 以 `name<TAB>createAt` 列出已註冊資料庫 |
| `remove` | `kura remove <name>` | 輸入 `yes` 確認後刪除目錄、連結與註冊 |
| `edit` | `kura edit <old> <new>` | 重新命名目錄、連結與註冊 |
| `stop` | `kura stop` | 停止 daemon |
| `port` | `kura port set <port>` \| `kura port clear` | 固定／解除固定 HTTP 埠 |
| `remote` | `kura remote enable` \| `kura remote disable` | 開關 HTTP `/mcp` |
| `mcp` | `kura mcp` | 以 stdio 提供 MCP |
| `help` | `kura help` \| `-h` \| `--help` | 顯示用法 |

### Makefile 目標

| 目標 | 說明 |
|------|------|
| `make build` | `go build -o bin/kura ./cmd/app` |
| `make app` | 停止 → 建置 → 安裝至 `/usr/local/bin/kura` → 啟動 |
| `make stop` | 停止 daemon |
| `make test` | `go test -v -count=1 ./...` |
| `make add foo` / `make list` / `make remove foo` / `make edit old new` / `make port set 8080` / `make help` | 以 `go run` 轉呼叫對應子指令 |

### HTTP 端點

| 方法 | 路徑 | 說明 |
|------|------|------|
| `GET` | `/api/health` | 回傳純文字 `OK` |
| `GET` | `/api/list` | `{loaded, registered}` |
| `GET` | `/api/search` | 參數見下表 |
| `GET` | `/api/semantic`、`/api/keyword` | 等同 `target=semantic` / `target=keyword`，將於 v1 移除 |
| `ANY` | `/mcp` | 僅 `remote: true` 時掛載，Streamable HTTP MCP |

`/api/search` 參數：

| 參數 | 必要 | 預設 | 說明 |
|------|------|------|------|
| `db` | 是 | — | 已載入的資料庫名稱 |
| `q` | 是 | — | 查詢字串 |
| `limit` | 否 | `10` | 1–100，超出範圍回退為 10 |
| `target` | 否 | 兩者並行 | `keyword` 或 `semantic`；未執行的欄位從回應省略 |

### MCP 工具

| 工具 | 參數 | 說明 |
|------|------|------|
| `list_rag` | 無 | 回傳 `{loaded, registered}` |
| `search_rag` | `db`（必要）、`q`（必要）、`mode`（`keyword` \| `semantic`，省略則兩者）、`limit`（預設 10，1–100） | 回傳 `{keyword, semantic}`，結構同 HTTP |

### 搜尋行為

| 項目 | 值 |
|------|----|
| 關鍵字 | gse 簡中字典斷詞 + 停用詞過濾，小寫去重後以 `LOWER(content) LIKE` OR 比對，依命中詞數排序 |
| 語意 | 查詢向量先查記憶體 query cache（持久化於 `global.db`），未命中才呼叫 OpenAI |
| 來源候選數 | `clamp(來源數 / 20, 20, 來源數)` |
| 相似度門檻 | cosine < 0.3 截斷 |
| 輸入截斷 | 單段超過 8000 字元截斷後送 embedding |
| 軟刪除 | 兩種搜尋皆排除 `dismiss = TRUE` |

***

©️ 2026 [邱敬幃 Pardn Chiu](https://www.linkedin.com/in/pardnchiu)
