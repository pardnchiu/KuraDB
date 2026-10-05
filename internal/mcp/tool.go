package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/pardnchiu/kuradb/internal/database"
	"github.com/pardnchiu/kuradb/internal/search"
)

type listInput struct{}

type listOutput struct {
	Loaded     []string         `json:"loaded"`
	Registered []database.Entry `json:"registered"`
}

type searchInput struct {
	Mode  string `json:"mode"`
	DB    string `json:"db"`
	Q     string `json:"q"`
	Limit int    `json:"limit"`
}

type searchOutput struct {
	Keyword []search.Group `json:"keyword,omitempty"`
}

func addTools(server *mcpsdk.Server, src *store) {
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "list_rag",
		Description: "List RAG knowledge base databases. Call when the target database name is unknown; skip if the database name is already known.",
		InputSchema: &jsonschema.Schema{Type: "object"},
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, _ listInput) (*mcpsdk.CallToolResult, listOutput, error) {
		out, err := src.list(ctx)
		return nil, out, err
	})

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name: "search_rag",
		Description: `Search RAG knowledge base by keyword. Answer directly if results suffice.
Indexed files only — it holds no conversation history and no session memory.`,
		InputSchema: searchSchema(),
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in searchInput) (*mcpsdk.CallToolResult, searchOutput, error) {
		out, err := src.search(ctx, in)
		return nil, out, err
	})
}

func searchSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"mode": {
				Type:        "string",
				Description: "Search mode. Only keyword is supported; omit to use it.",
				Enum:        []any{search.TargetKeyword},
			},
			"db": {
				Type:        "string",
				Description: "Target RAG database name.",
			},
			"q": {
				Type:        "string",
				Description: "Search query.",
			},
			"limit": {
				Type:        "integer",
				Description: fmt.Sprintf("Max chunks to return (1-%d). Invalid values fall back to %d.", search.MaxLimit, search.DefaultLimit),
				Default:     json.RawMessage(fmt.Sprint(search.DefaultLimit)),
			},
		},
		Required: []string{"db", "q"},
	}
}
