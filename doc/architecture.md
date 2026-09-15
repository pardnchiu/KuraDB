# kuradb - Architecture

> Back to [README](../README.md)

## Overview

```mermaid
graph TB
    subgraph Entry
        CLI[kura CLI]
        Daemon[Background daemon]
        MCPStdio[kura mcp<br/>read-only stdio process]
    end

    subgraph Indexing Pipeline
        Inbox[~/Kura_name folder]
        Watcher[File Watcher<br/>10s poll]
        Parser[go-pkg parser<br/>PDF/DOCX/PPTX/CSV/XLSX/text]
        Embedder[Embedding Scheduler<br/>5s / 64 chunks]
    end

    subgraph Storage
        Registry[(db.json)]
        PerDB[(Per-DB data.db<br/>file_data)]
        GlobalDB[(global.db<br/>query_cache)]
    end

    subgraph Query
        Router[Gin Router]
        MCPServer[MCP Server<br/>list_rag / search_rag]
        Search[search.Search]
        Segmenter[gse Segmenter]
        Vector[Vector Cache]
        QCache[Query Cache]
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
    Router -->|/mcp when remote| MCPServer
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

## Module: cmd/app (Entry and Lifecycle)

Dispatches CLI subcommands, forks the daemon, or starts MCP over stdio based on arguments.

```mermaid
graph TB
    subgraph cmd/app
        Main["main()"] --> Switch{os.Args}
        Switch -->|add/list/remove/edit| Registry[database.Registry]
        Switch -->|port/remote| Config[config.json]
        Switch -->|stop| Stop[runtime.Stop]
        Switch -->|mcp| CmdMCP["cmdMCP()"]
        Switch -->|no args| RunServer["runServer()<br/>fork --daemon, wait for endpoint"]
        Switch -->|--daemon| Daemon["runServerDaemon()"]

        Daemon --> RT[runtime.Init<br/>runtime.uid]
        Daemon --> Key[keychain → openai.New]
        Daemon --> QC[OpenGlobal + loadQueryCache<br/>register OnSet write-back]
        Daemon --> Loop[Per DB: inbox / symlink / OpenPerDB<br/>InitBucket / loadCache]
        Loop --> W["go runWatcher()"]
        Loop --> E["go runEmbedder()"]
        Daemon --> H["go runHTTP()"]
        H --> Sig[Wait SIGINT/SIGTERM → cleanup]

        CmdMCP --> RO[OpenGlobal + loadQueryCache<br/>no OnSet]
        RO --> ROLoop[Per DB OpenPerDB + loadCache]
        ROLoop --> Run["mcp.Run() stdio"]
    end
```

## Module: internal/filesystem (Watching and Parsing)

Recursively diffs the inbox against a size + mtime snapshot, dispatches changed files to parsers by extension, and recursively soft-deletes vanished files.

```mermaid
graph TB
    subgraph filesystem
        Walk["WalkFiles()<br/>recursive os.ReadDir"] --> Cmp{Same as last snapshot?}
        Cmp -->|Yes| Keep[Keep node]
        Cmp -->|No, file| Skip{Skipped name/ext?}
        Skip -->|Yes| Keep
        Skip -->|No| Ext{Extension}
        Ext -->|.pdf/.docx/.pptx| Doc[go-pkg parser]
        Ext -->|.csv/.tsv/.xlsx| Tab["parseTabular()<br/>header + 5 rows"]
        Ext -->|other| Sniff["looksLikeText()<br/>8 KiB UTF-8 / no NUL"]
        Sniff -->|text| MD[Markdown parser]
        Doc --> Up
        Tab --> Up
        MD --> Up
        Walk -->|in snapshot, gone from dir| Dis["dismissRemoved()"]
    end
    Up["databaseHandler.Upsert()"] --> DB[(data.db)]
    Dis --> DisH["databaseHandler.Dismiss()"] --> DB
    Walk --> Rec[record.json]
```

## Module: internal/database (Data Layer)

`go-sqlkit` manages split read/write connections; handlers are the only SQLite write entry point.

```mermaid
graph TB
    subgraph database
        DB["DB{Read, Write *sql.DB}"]
        OpenPerDB["OpenPerDB()<br/>file_data schema"]
        OpenGlobal["OpenGlobal()<br/>query_cache schema"]
        Reg["Registry<br/>Load/Has/Add/Remove/Rename"]
    end

    subgraph handler
        Upsert["Upsert()<br/>dismiss old chunks first<br/>keep embedding if content unchanged"]
        Dismiss["Dismiss()"]
        ListPending["ListPending()<br/>is_embed=FALSE"]
        UpdateEmbedding["UpdateEmbedding()<br/>apply only if content matches"]
        LoadEmbedding["LoadEmbedding()"]
        GetByIDs["GetByIDs()"]
        SearchKeyword["SearchKeyword()<br/>LIKE OR + hit-count ranking"]
        QCacheIO["Save/LoadQueryCache()"]
    end

    OpenPerDB --> DB
    OpenGlobal --> DB
    DB --> handler
    Reg --> JSON[(db.json)]
```

### file_data

| Column | Type | Description |
|--------|------|-------------|
| `id` | INTEGER PK | Primary key |
| `source` | TEXT | Absolute source file path |
| `chunk` | INTEGER | Chunk index |
| `total` | INTEGER | Total chunks for the file |
| `content` | TEXT | Chunk content |
| `embedding` | BLOB | 512-dim float32, little-endian |
| `is_embed` | BOOLEAN | Embedded flag |
| `dismiss` | BOOLEAN | Soft-delete flag |
| `created_at` / `updated_at` | TIMESTAMP | Timestamps |

Constraint `UNIQUE (source, chunk)`; index `idx_file_data_source` and partial index `idx_file_data_pending` (`is_embed = FALSE AND dismiss = FALSE`).

### query_cache

| Column | Type | Description |
|--------|------|-------------|
| `query` | TEXT PK | Query string |
| `embedding` | BLOB | Query vector |
| `created_at` | TIMESTAMP | Write time |

## Module: internal/openai (Embedding Client and Query Cache)

```mermaid
graph TB
    subgraph openai
        New["New()<br/>keychain OPENAI_API_KEY"]
        Embed["EmbedBatch()<br/>truncate over 8000 chars<br/>validate count and dim"]
        Codec["Encode() / Decode()"]
        Dim["Dim() = 512"]
        Cache["Cache<br/>Get / Set / Preload / OnSet"]
    end
    Embed --> API["POST /v1/embeddings<br/>text-embedding-3-small"]
    Cache -->|OnSet registered by daemon only| Save[SaveQueryCache → global.db]
```

## Module: internal/vector (Vector Cache)

One bucket per database; fully rebuilt from SQLite and never authoritative.

```mermaid
graph TB
    subgraph vector
        C["Cache{dbBuckets}"] --> B["Bucket"]
        B --> IV["idVectors"]
        B --> IS["idSource"]
        B --> SC["sourceChunks"]
        B --> SV["sourceVectors<br/>normalized mean vectors"]
        Set["Set()"] --> IV
        Set --> SC
        Rebuild["Rebuild() / RebuildAll()"] --> SV
        Search["Search()"] --> S1["Stage 1: cosine(query, sourceVectors)<br/>take clamp(n/20, 20, n) sources"]
        S1 --> S2["Stage 2: parallel cosine over candidate chunks<br/>NumCPU-1 workers, ≥ 200 chunks each"]
        S2 --> TopK[Sort, take top-K]
        Search -->|no sourceVectors| Full["searchLocked() full scan"]
    end
```

## Module: internal/search and internal/mcp (Shared Query Core)

HTTP and MCP share `search.Search`, keeping behavior identical across both surfaces.

```mermaid
graph TB
    subgraph api
        R["Router()"] --> H["/api/health"]
        R --> L["/api/list"]
        R --> QS["/api/search<br/>queryDB + withTarget"]
        R --> Legacy["/api/semantic, /api/keyword"]
        R -->|remote| MH["/mcp → mcp.Handler()"]
    end

    subgraph mcp
        MH --> Srv[MCP Server]
        Stdio["mcp.Run() stdio"] --> Srv
        Srv --> T1[list_rag → store.list]
        Srv --> T2[search_rag → store.search]
    end

    subgraph search
        S["Search()<br/>validate db/q/limit/target"]
        S --> KW["segmenter.Tokenize → SearchKeyword"]
        S --> SEM["getSemantic()<br/>qCache → EmbedBatch → vector.Search<br/>cosine ≥ 0.3 → GetByIDs"]
        KW --> G["group() by source"]
        SEM --> G
    end

    QS --> S
    Legacy --> S
    T2 --> S
```

## Data Flow: File Indexing

```mermaid
sequenceDiagram
    participant U as User
    participant W as Watcher (10s)
    participant P as Parser
    participant DB as data.db
    participant E as Embedder (5s)
    participant O as OpenAI
    participant V as Vector Cache

    U->>W: Drop file into ~/Kura_name
    W->>W: Diff size + mtime snapshot
    W->>P: Parse by extension
    P-->>W: chunks
    W->>DB: Upsert (dismiss old chunks, clear embedding on content change)
    W->>W: Write record.json
    E->>DB: ListPending (up to 64)
    DB-->>E: Pending chunks
    E->>O: EmbedBatch
    O-->>E: 512-dim vectors
    E->>DB: UpdateEmbedding (only if content matches)
    E->>V: Set + Rebuild affected sources
```

## Data Flow: Search

```mermaid
sequenceDiagram
    participant C as Client / MCP client
    participant R as Router / MCP Server
    participant S as search.Search
    participant Seg as Segmenter
    participant Q as Query Cache
    participant O as OpenAI
    participant V as Vector Cache
    participant DB as data.db

    C->>R: /api/search or search_rag
    R->>S: db, q, target, limit
    par Keyword
        S->>Seg: Tokenize(q)
        Seg-->>S: keywords
        S->>DB: SearchKeyword (dismiss=FALSE)
        DB-->>S: rows
    and Semantic
        S->>Q: Get(q)
        alt Miss
            S->>O: EmbedBatch([q])
            O-->>S: vector
            S->>Q: Set(q, v)
        end
        S->>V: Search(db, v, limit)
        V-->>S: hits (cosine ≥ 0.3)
        S->>DB: GetByIDs (dismiss=FALSE)
        DB-->>S: rows
    end
    S-->>R: {keyword, semantic} grouped by source
    R-->>C: JSON
```

## State Machine: Chunk Lifecycle

```mermaid
stateDiagram-v2
    [*] --> Pending: Upsert new chunk
    Pending --> Embedded: UpdateEmbedding succeeds
    Embedded --> Embedded: Re-upsert with same content
    Embedded --> Pending: Re-upsert with changed content
    Pending --> Dismissed: File removed / fewer chunks
    Embedded --> Dismissed: File removed / fewer chunks
    Dismissed --> Pending: Same source+chunk upserted again
```

## State Machine: Daemon

```mermaid
stateDiagram-v2
    [*] --> Spawning: kura
    Spawning --> Running: endpoint written (≤10s)
    Spawning --> Failed: Timeout, check daemon.log
    Running --> Restarting: port set / remote enable|disable
    Restarting --> Spawning
    Running --> Stopping: kura stop (SIGTERM)
    Stopping --> [*]: Exit, or SIGKILL after 5s
```

***

©️ 2026 [邱敬幃 Pardn Chiu](https://www.linkedin.com/in/pardnchiu)
