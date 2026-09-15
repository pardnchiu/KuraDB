package mcp

import (
	"context"
	"net/http"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/agenvoy/kuradb/internal/database"
	"github.com/agenvoy/kuradb/internal/openai"
)

const (
	serverName    = "kuradb"
	serverVersion = "0.5.1"

	instructions = `KuraDB is a read-only retrieval source over local file collections ("db"), each backed by a watched folder
of static files the user put there: notes, documents, specs, reference material.

Call list_rag when the target database name is unknown, then search_rag to retrieve file chunks from it.
search_rag runs keyword matching and semantic vector similarity in parallel; results are chunks grouped by their source file.
Use it whenever an answer depends on the content of those files rather than on general knowledge.

It stores nothing but the indexed files: no conversation history, no session memory, no user profile.

This server never writes: indexing happens only when files are dropped into a db inbox folder.`
)

func newServer(src *store) *mcpsdk.Server {
	server := mcpsdk.NewServer(
		&mcpsdk.Implementation{Name: serverName, Version: serverVersion},
		&mcpsdk.ServerOptions{Instructions: instructions},
	)
	addTools(server, src)
	return server
}

func Handler(reg *database.Registry, dbs map[string]*database.DB, embedder openai.Embedder, qCache *openai.Cache) http.Handler {
	server := newServer(newStore(reg, dbs, embedder, qCache))
	return mcpsdk.NewStreamableHTTPHandler(
		func(*http.Request) *mcpsdk.Server { return server },
		nil,
	)
}

func Run(ctx context.Context, reg *database.Registry, dbs map[string]*database.DB, embedder openai.Embedder, qCache *openai.Cache) error {
	server := newServer(newStore(reg, dbs, embedder, qCache))
	return server.Run(ctx, &mcpsdk.StdioTransport{})
}
