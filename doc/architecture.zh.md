# kuradb - 架構

> 返回 [README](./README.zh.md)

## 概覽

```mermaid
graph TB
    subgraph 入口
        CLI[kura CLI]
        Daemon[背景 daemon]
        MCPStdio[kura mcp<br/>stdio 唯讀程序]
    end

    subgraph 索引管線
        Inbox[~/Kura_name 資料夾]
        Watcher[檔案監控器<br/>10s 輪詢]
        Parser[go-pkg parser<br/>PDF/DOCX/PPTX/CSV/XLSX/文字]
        Embedder[Embedding 排程器<br/>5s / 64 區塊]
    end

    subgraph 儲存
        Registry[(db.json)]
        PerDB[(每庫 data.db<br/>file_data)]
        GlobalDB[(global.db<br/>query_cache)]
    end

    subgraph 查詢
        Router[Gin Router]
        MCPServer[MCP Server<br/>list_rag / search_rag]
        Search[search.Search]
        Segmenter[gse 斷詞器]
        Vector[向量快取]
        QCache[查詢快取]
    end

    OpenAI[OpenAI Embeddings API]

    CLI --> Registry
    CLI --> Daemon
    Daemon --> Watcher
    Daemon --> Embedder
    Daemon --> Router
    Inbox --> Watcher
    Watcher --> Parser
    Parser --> PerDB
    PerDB --> Embedder
    Embedder --> OpenAI
    Embedder --> PerDB
    Embedder --> Vector

    Router -->|/api/*| Search
    Router -->|/mcp 需 remote| MCPServer
    MCPStdio --> MCPServer
    MCPServer --> Search
    Search --> Segmenter
    Segmenter --> PerDB
    Search --> QCache
    QCache --> OpenAI
    QCache --> GlobalDB
    Search --> Vector
    Vector --> PerDB
```

## 模組：cmd/app（入口與生命週期）

依參數分派 CLI 子指令、fork daemon，或以 stdio 啟動 MCP。

```mermaid
graph TB
    subgraph cmd/app
        Main["main()"] --> Switch{os.Args}
        Switch -->|add/list/remove/edit| Registry[database.Registry]
        Switch -->|port/remote| Config[config.json]
        Switch -->|stop| Stop[runtime.Stop]
        Switch -->|mcp| CmdMCP["cmdMCP()"]
        Switch -->|無參數| RunServer["runServer()<br/>fork --daemon，等待 endpoint"]
        Switch -->|--daemon| Daemon["runServerDaemon()"]

        Daemon --> RT[runtime.Init<br/>runtime.uid]
        Daemon --> Key[keychain → openai.New]
        Daemon --> QC[OpenGlobal + loadQueryCache<br/>註冊 OnSet 寫回]
        Daemon --> Loop[逐庫：inbox / symlink / OpenPerDB<br/>InitBucket / loadCache]
        Loop --> W["go runWatcher()"]
        Loop --> E["go runEmbedder()"]
        Daemon --> H["go runHTTP()"]
        H --> Sig[等待 SIGINT/SIGTERM → 清理]

        CmdMCP --> RO[OpenGlobal + loadQueryCache<br/>不註冊 OnSet]
        RO --> ROLoop[逐庫 OpenPerDB + loadCache]
        ROLoop --> Run["mcp.Run() stdio"]
    end
```

## 模組：internal/filesystem（監控與解析）

以 size + mtime 快照遞迴比對 inbox，變更檔依副檔名分派 parser，消失的檔案遞迴軟刪除。

```mermaid
graph TB
    subgraph filesystem
        Walk["WalkFiles()<br/>遞迴 os.ReadDir"] --> Cmp{與上次快照相同?}
        Cmp -->|是| Keep[保留節點]
        Cmp -->|否，檔案| Skip{略過名稱/副檔名?}
        Skip -->|是| Keep
        Skip -->|否| Ext{副檔名}
        Ext -->|.pdf/.docx/.pptx| Doc[go-pkg parser]
        Ext -->|.csv/.tsv/.xlsx| Tab["parseTabular()<br/>表頭 + 每 5 列"]
        Ext -->|其他| Sniff["looksLikeText()<br/>8 KiB UTF-8 / 無 NUL"]
        Sniff -->|文字| MD[Markdown parser]
        Doc --> Up
        Tab --> Up
        MD --> Up
        Walk -->|快照有、目錄無| Dis["dismissRemoved()"]
    end
    Up["databaseHandler.Upsert()"] --> DB[(data.db)]
    Dis --> DisH["databaseHandler.Dismiss()"] --> DB
    Walk --> Rec[record.json]
```

## 模組：internal/database（資料層）

`go-sqlkit` 管理讀寫分離連線；handler 為 SQLite 唯一寫入入口。

```mermaid
graph TB
    subgraph database
        DB["DB{Read, Write *sql.DB}"]
        OpenPerDB["OpenPerDB()<br/>file_data schema"]
        OpenGlobal["OpenGlobal()<br/>query_cache schema"]
        Reg["Registry<br/>Load/Has/Add/Remove/Rename"]
    end

    subgraph handler
        Upsert["Upsert()<br/>舊區塊先 dismiss<br/>內容相同保留 embedding"]
        Dismiss["Dismiss()"]
        ListPending["ListPending()<br/>is_embed=FALSE"]
        UpdateEmbedding["UpdateEmbedding()<br/>content 相符才套用"]
        LoadEmbedding["LoadEmbedding()"]
        GetByIDs["GetByIDs()"]
        SearchKeyword["SearchKeyword()<br/>LIKE OR + 命中數排序"]
        QCacheIO["Save/LoadQueryCache()"]
    end

    OpenPerDB --> DB
    OpenGlobal --> DB
    DB --> handler
    Reg --> JSON[(db.json)]
```

### file_data

| 欄位 | 型別 | 說明 |
|------|------|------|
| `id` | INTEGER PK | 主鍵 |
| `source` | TEXT | 來源檔絕對路徑 |
| `chunk` | INTEGER | 區塊序號 |
| `total` | INTEGER | 該檔總區塊數 |
| `content` | TEXT | 區塊內容 |
| `embedding` | BLOB | 512 維 float32 little-endian |
| `is_embed` | BOOLEAN | 是否已嵌入 |
| `dismiss` | BOOLEAN | 軟刪除 |
| `created_at` / `updated_at` | TIMESTAMP | 時間戳 |

約束 `UNIQUE (source, chunk)`；索引 `idx_file_data_source`、部分索引 `idx_file_data_pending`（`is_embed = FALSE AND dismiss = FALSE`）。

### query_cache

| 欄位 | 型別 | 說明 |
|------|------|------|
| `query` | TEXT PK | 查詢字串 |
| `embedding` | BLOB | 查詢向量 |
| `created_at` | TIMESTAMP | 寫入時間 |

## 模組：internal/openai（Embedding 客戶端與查詢快取）

```mermaid
graph TB
    subgraph openai
        New["New()<br/>keychain OPENAI_API_KEY"]
        Embed["EmbedBatch()<br/>超過 8000 字元截斷<br/>驗證數量與維度"]
        Codec["Encode() / Decode()"]
        Dim["Dim() = 512"]
        Cache["Cache<br/>Get / Set / Preload / OnSet"]
    end
    Embed --> API["POST /v1/embeddings<br/>text-embedding-3-small"]
    Cache -->|OnSet 僅 daemon 註冊| Save[SaveQueryCache → global.db]
```

## 模組：internal/vector（向量快取）

每庫一個 Bucket；由 SQLite 全量重建，非權威資料源。

```mermaid
graph TB
    subgraph vector
        C["Cache{dbBuckets}"] --> B["Bucket"]
        B --> IV["idVectors"]
        B --> IS["idSource"]
        B --> SC["sourceChunks"]
        B --> SV["sourceVectors<br/>正規化平均向量"]
        Set["Set()"] --> IV
        Set --> SC
        Rebuild["Rebuild() / RebuildAll()"] --> SV
        Search["Search()"] --> S1["階段一：cosine(query, sourceVectors)<br/>取 clamp(n/20, 20, n) 個來源"]
        S1 --> S2["階段二：候選區塊分 worker 平行 cosine<br/>NumCPU-1，每 worker ≥ 200 區塊"]
        S2 --> TopK[排序取 top-K]
        Search -->|無 sourceVectors| Full["searchLocked() 全量掃描"]
    end
```

## 模組：internal/search 與 internal/mcp（共用查詢核心）

HTTP 與 MCP 共用 `search.Search`，確保兩端行為一致。

```mermaid
graph TB
    subgraph api
        R["Router()"] --> H["/api/health"]
        R --> L["/api/list"]
        R --> QS["/api/search<br/>queryDB + withTarget"]
        R --> Legacy["/api/semantic、/api/keyword"]
        R -->|remote| MH["/mcp → mcp.Handler()"]
    end

    subgraph mcp
        MH --> Srv[MCP Server]
        Stdio["mcp.Run() stdio"] --> Srv
        Srv --> T1[list_rag → store.list]
        Srv --> T2[search_rag → store.search]
    end

    subgraph search
        S["Search()<br/>驗證 db/q/limit/target"]
        S --> KW["segmenter.Tokenize → SearchKeyword"]
        S --> SEM["getSemantic()<br/>qCache → EmbedBatch → vector.Search<br/>cosine ≥ 0.3 → GetByIDs"]
        KW --> G["group() 依 source 分組"]
        SEM --> G
    end

    QS --> S
    Legacy --> S
    T2 --> S
```

## 資料流：檔案索引

```mermaid
sequenceDiagram
    participant U as 使用者
    participant W as Watcher (10s)
    participant P as Parser
    participant DB as data.db
    participant E as Embedder (5s)
    participant O as OpenAI
    participant V as 向量快取

    U->>W: 檔案放入 ~/Kura_name
    W->>W: 比對 size + mtime 快照
    W->>P: 依副檔名解析
    P-->>W: chunks
    W->>DB: Upsert（舊區塊 dismiss，內容變更則清除 embedding）
    W->>W: 寫入 record.json
    E->>DB: ListPending（最多 64）
    DB-->>E: 待嵌入區塊
    E->>O: EmbedBatch
    O-->>E: 512 維向量
    E->>DB: UpdateEmbedding（content 相符才套用）
    E->>V: Set + Rebuild 受影響來源
```

## 資料流：搜尋

```mermaid
sequenceDiagram
    participant C as 客戶端 / MCP client
    participant R as Router / MCP Server
    participant S as search.Search
    participant Seg as 斷詞器
    participant Q as 查詢快取
    participant O as OpenAI
    participant V as 向量快取
    participant DB as data.db

    C->>R: /api/search 或 search_rag
    R->>S: db, q, target, limit
    par 關鍵字
        S->>Seg: Tokenize(q)
        Seg-->>S: 關鍵詞
        S->>DB: SearchKeyword（dismiss=FALSE）
        DB-->>S: rows
    and 語意
        S->>Q: Get(q)
        alt 未命中
            S->>O: EmbedBatch([q])
            O-->>S: 向量
            S->>Q: Set(q, v)
        end
        S->>V: Search(db, v, limit)
        V-->>S: hits（cosine ≥ 0.3）
        S->>DB: GetByIDs（dismiss=FALSE）
        DB-->>S: rows
    end
    S-->>R: {keyword, semantic} 依來源分組
    R-->>C: JSON
```

## 狀態機：區塊生命週期

```mermaid
stateDiagram-v2
    [*] --> Pending: Upsert 新區塊
    Pending --> Embedded: UpdateEmbedding 成功
    Embedded --> Embedded: 重新 Upsert 內容相同
    Embedded --> Pending: 重新 Upsert 內容變更
    Pending --> Dismissed: 檔案移除 / 區塊數減少
    Embedded --> Dismissed: 檔案移除 / 區塊數減少
    Dismissed --> Pending: 同 source+chunk 再次 Upsert
```

## 狀態機：daemon

```mermaid
stateDiagram-v2
    [*] --> Spawning: kura
    Spawning --> Running: endpoint 寫入（≤10s）
    Spawning --> Failed: 逾時，查看 daemon.log
    Running --> Restarting: port set / remote enable|disable
    Restarting --> Spawning
    Running --> Stopping: kura stop（SIGTERM）
    Stopping --> [*]: 結束或 5s 後 SIGKILL
```

***

©️ 2026 [邱敬幃 Pardn Chiu](https://www.linkedin.com/in/pardnchiu)
