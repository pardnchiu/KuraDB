package agenvoy

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	goUtils_filesystem "github.com/pardnchiu/go-utils/filesystem"
)

const (
	timeoutSeconds = 15
	defaultLimit   = 10
	maxLimit       = 100

	keywordToolName  = "rag_search_keyword"
	semanticToolName = "rag_search_semantic"
)

type endpoint struct {
	URL         string `json:"url"`
	Method      string `json:"method"`
	ContentType string `json:"content_type"`
	Timeout     int    `json:"timeout"`
}

type parameter struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Default     any    `json:"default,omitempty"`
}

type response struct {
	Format string `json:"format"`
}

type tool struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Endpoint    endpoint             `json:"endpoint"`
	Parameters  map[string]parameter `json:"parameters"`
	Response    response             `json:"response"`
}

func Register(dbName, baseURL string) error {
	if dbName == "" {
		return errors.New("dbName is required")
	}
	if baseURL == "" {
		return errors.New("baseURL is required")
	}

	dir, err := toolsDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("os.MkdirAll %s: %w", dir, err)
	}

	tools := []tool{
		keywordTool(dbName, baseURL),
		semanticTool(dbName, baseURL),
	}
	for _, t := range tools {
		path := filepath.Join(dir, t.Name+".json")
		if err := goUtils_filesystem.WriteJSON(path, t, true); err != nil {
			return fmt.Errorf("goUtils_filesystem.WriteJSON %s: %w", path, err)
		}
	}
	return nil
}

func Unregister() error {
	dir, err := toolsDir()
	if err != nil {
		return err
	}

	var firstErr error
	for _, name := range []string{keywordToolName, semanticToolName} {
		path := filepath.Join(dir, name+".json")
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			if firstErr == nil {
				firstErr = fmt.Errorf("os.Remove %s: %w", path, err)
			}
		}
	}
	return firstErr
}

func toolsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("os.UserHomeDir: %w", err)
	}
	if home == "" {
		return "", errors.New("home directory is empty")
	}
	return filepath.Join(home, ".config", "Agenvoy", "api_tools"), nil
}

func dbParam(dbName string) parameter {
	return parameter{
		Type:        "string",
		Description: fmt.Sprintf("Target RAG db name. Must match the running instance (%q); mismatched values return HTTP 400.", dbName),
		Required:    true,
		Default:     dbName,
	}
}

func limitParam() parameter {
	return parameter{
		Type:        "integer",
		Description: fmt.Sprintf("Max chunks to return (1-%d). Invalid values fall back to %d.", maxLimit, defaultLimit),
		Required:    false,
		Default:     defaultLimit,
	}
}

func keywordTool(dbName, baseURL string) tool {
	return tool{
		Name:        keywordToolName,
		Description: fmt.Sprintf("Keyword search over the RAG index served by this process (db=%q). Tokenizes the query (Chinese-aware via gse), runs case-insensitive SQL LIKE matching, and returns matching chunks grouped by source file ranked by hit count.", dbName),
		Endpoint: endpoint{
			URL:         baseURL + "/api/keyword",
			Method:      "GET",
			ContentType: "json",
			Timeout:     timeoutSeconds,
		},
		Parameters: map[string]parameter{
			"db": dbParam(dbName),
			"q": {
				Type:        "string",
				Description: "Search query. Natural-language input is tokenized into keywords; stopwords are removed.",
				Required:    true,
			},
			"limit": limitParam(),
		},
		Response: response{Format: "json"},
	}
}

func semanticTool(dbName, baseURL string) tool {
	return tool{
		Name:        semanticToolName,
		Description: fmt.Sprintf("Semantic search over the RAG index served by this process (db=%q). Embeds the query with OpenAI text-embedding-3-small (1536-dim) and returns the top cosine-similarity chunks (min score 0.3) grouped by source file.", dbName),
		Endpoint: endpoint{
			URL:         baseURL + "/api/semantic",
			Method:      "GET",
			ContentType: "json",
			Timeout:     timeoutSeconds,
		},
		Parameters: map[string]parameter{
			"db": dbParam(dbName),
			"q": {
				Type:        "string",
				Description: "Natural-language query; semantic similarity is computed against indexed chunk embeddings.",
				Required:    true,
			},
			"limit": limitParam(),
		},
		Response: response{Format: "json"},
	}
}
