# kuradb - Documentation

> Back to [README](../README.md)

## Prerequisites

- Go 1.25.1 or higher
- A C compiler with CGO enabled (required by `mattn/go-sqlite3`)
- OpenAI API key (`text-embedding-3-small`, 512 dimensions)
- macOS (APFS/HFS+) or Linux (ext4/xfs); Windows, SMB, NFS, and FUSE are unsupported

## Installation

### From Source

```bash
git clone https://github.com/agenvoy/kuradb.git
cd kuradb
make build
# outputs bin/kura
```

### Build and Install to /usr/local/bin

```bash
make app
# stop old daemon → build → sudo move to /usr/local/bin/kura → start
```

### Using go install

```bash
go install github.com/agenvoy/kuradb/cmd/app@latest
mv "$(go env GOPATH)/bin/app" "$(go env GOPATH)/bin/kura"
# go install names the binary after the app directory; rename it to kura
```

## Configuration

### API Key

| Name | Required | Description |
|------|----------|-------------|
| `OPENAI_API_KEY` | Yes | Resolved in order: system keychain (service `kuradb`) → environment variable; the daemon and `kura mcp` exit at startup without it |

System keychain lookup order:

| Platform | Order |
|----------|-------|
| macOS | `security find-generic-password -s kuradb -a OPENAI_API_KEY` → environment variable |
| Linux | `secret-tool lookup service kuradb account OPENAI_API_KEY` → `~/.config/kuradb/.secrets` (`OPENAI_API_KEY=...`) → environment variable |

A value stored in the keychain takes precedence; the environment variable is ignored when one exists.

### Config Directory `~/.config/kuradb/`

| Path | Purpose |
|------|---------|
| `db.json` | Database registry (`db`, `createAt`) |
| `config.json` | Server settings: `port` (fixed port), `remote` (mount `/mcp`) |
| `global.db` | Global SQLite holding `query_cache` |
| `{name}/data.db` | Per-database SQLite holding `file_data` |
| `{name}/inbox/` | Watched directory |
| `{name}/record.json` | Filesystem snapshot (size + mtime) |
| `endpoint` | HTTP address written once the daemon is ready |
| `runtime.uid` | Daemon PID and start time |
| `daemon.log` | Daemon stdout / stderr |

Each database gets a home-directory symlink `~/Kura_{name}` → `~/.config/kuradb/{name}/inbox/`.

### `config.json`

```json
{
  "port": 8080,
  "remote": true
}
```

| Field | Default | Description |
|-------|---------|-------------|
| `port` | Unset → random in 10000–65535 (up to 10 tries) | Binds to `127.0.0.1` |
| `remote` | `false` | When `true`, mounts `/mcp` on HTTP; read at startup, so changes require a restart |

## Usage

### Basic: Create a Database and Start

```bash
kura add my_docs
# db added: my_docs
#   dir:  ~/.config/kuradb/my_docs
#   link: ~/Kura_my_docs

kura
# http://localhost:12345
```

`kura` forks a background daemon, waits up to 10 seconds for `endpoint` to appear, and prints the address; on timeout it points to `daemon.log`. Restart the daemon after adding a database to load it.

### Index Files

```bash
cp spec.pdf notes.md ~/Kura_my_docs/
# within 10s the watcher detects the change → parses into chunks → upserts into SQLite
# every 5s the embedder takes up to 64 pending chunks → OpenAI → updates the vector cache
```

| Type | Extensions | Handling |
|------|------------|----------|
| PDF | `.pdf` | go-pkg parser |
| Word | `.docx` | go-pkg parser |
| PowerPoint | `.pptx` | go-pkg parser |
| Tabular | `.csv`, `.tsv`, `.xlsx` | First row as header, 5 rows per chunk (`[n] col=value, ...`) |
| Text | Any other extension | Parsed as Markdown only if the first 8 KiB is valid UTF-8 with no NUL bytes |
| Skipped | Images, media, archives, executables, fonts, database files, `.DS_Store`, etc. | Not indexed |

Subdirectories are scanned recursively; when a file is removed, its chunks are marked `dismiss = TRUE` and drop out of search results.

### Query the HTTP API

```bash
EP="$(cat ~/.config/kuradb/endpoint)"

curl "$EP/api/health"
# OK

curl "$EP/api/list"
# {"loaded":["my_docs"],"registered":[{"db":"my_docs","createAt":"2026-09-16T00:00:00Z"}]}

curl -G "$EP/api/search" --data-urlencode "db=my_docs" --data-urlencode "q=what is RAG" --data-urlencode "limit=5"
```

Response:

```json
{
  "keyword": [
    {"source": "/Users/me/.config/kuradb/my_docs/inbox/notes.md", "matches": [{"chunk": 1, "content": "RAG is retrieval-augmented generation..."}]}
  ],
  "semantic": [
    {"source": "/Users/me/.config/kuradb/my_docs/inbox/spec.pdf", "matches": [{"chunk": 3, "content": "..."}]}
  ]
}
```

Error handling:

```bash
curl -s -w "\n%{http_code}\n" "$EP/api/search?db=missing&q=x"
# {"error":"\"missing\" not exist"}
# 400
```

### Advanced: Fixed Port and Remote MCP

```bash
kura port set 8080      # writes config.json and restarts the daemon
kura remote enable      # mounts HTTP /mcp; restarts the daemon if running
kura remote disable
kura port clear         # applies on the next manual restart
kura stop               # SIGTERM, then SIGKILL after 5s
```

### Advanced: Connect an MCP Client over stdio

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

`kura mcp` opens SQLite and loads the vector cache on its own, read-only, without the watcher or embedder; restart the MCP session to see content the daemon indexed afterward.

## CLI Reference

### Commands

| Command | Syntax | Description |
|---------|--------|-------------|
| (none) | `kura` | Start the background daemon and print the endpoint |
| `add` | `kura add <name>` | Register a database and create its inbox and `~/Kura_{name}` link; whitespace in names becomes `_` |
| `list` | `kura list` | List registered databases as `name<TAB>createAt` |
| `remove` | `kura remove <name>` | Delete directory, link, and registry entry after typing `yes` |
| `edit` | `kura edit <old> <new>` | Rename directory, link, and registry entry |
| `stop` | `kura stop` | Stop the daemon |
| `port` | `kura port set <port>` \| `kura port clear` | Pin or unpin the HTTP port |
| `remote` | `kura remote enable` \| `kura remote disable` | Toggle HTTP `/mcp` |
| `mcp` | `kura mcp` | Serve MCP over stdio |
| `help` | `kura help` \| `-h` \| `--help` | Show usage |

### Makefile Targets

| Target | Description |
|--------|-------------|
| `make build` | `go build -o bin/kura ./cmd/app` |
| `make app` | Stop → build → install to `/usr/local/bin/kura` → start |
| `make stop` | Stop the daemon |
| `make test` | `go test -v -count=1 ./...` |
| `make add foo` / `make list` / `make remove foo` / `make edit old new` / `make port set 8080` / `make help` | Forward to the matching subcommand via `go run` |

### HTTP Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/health` | Returns plain text `OK` |
| `GET` | `/api/list` | `{loaded, registered}` |
| `GET` | `/api/search` | Parameters below |
| `GET` | `/api/semantic`, `/api/keyword` | Equivalent to `target=semantic` / `target=keyword`; removed in v1 |
| `ANY` | `/mcp` | Streamable HTTP MCP, mounted only when `remote: true` |

`/api/search` parameters:

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| `db` | Yes | — | Loaded database name |
| `q` | Yes | — | Query string |
| `limit` | No | `10` | 1–100; out-of-range values fall back to 10 |
| `target` | No | Both in parallel | `keyword` or `semantic`; the skipped field is omitted from the response |

### MCP Tools

| Tool | Parameters | Description |
|------|------------|-------------|
| `list_rag` | None | Returns `{loaded, registered}` |
| `search_rag` | `db` (required), `q` (required), `mode` (`keyword` \| `semantic`, omit for both), `limit` (default 10, 1–100) | Returns `{keyword, semantic}` with the same shape as HTTP |

### Search Behavior

| Item | Value |
|------|-------|
| Keyword | gse Simplified Chinese dictionary + stopword filter, lowercased and deduplicated, OR-matched with `LOWER(content) LIKE`, ranked by matched-term count |
| Semantic | Query vector looked up in the in-memory query cache (persisted in `global.db`); OpenAI is called only on a miss |
| Source candidates | `clamp(sources / 20, 20, sources)` |
| Similarity floor | Hits with cosine < 0.3 are cut |
| Input truncation | Inputs over 8000 characters are truncated before embedding |
| Soft delete | Both searches exclude `dismiss = TRUE` |

***

©️ 2026 [邱敬幃 Pardn Chiu](https://www.linkedin.com/in/pardnchiu)
