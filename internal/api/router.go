package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/pardnchiu/KuraDB/internal/database"
	"github.com/pardnchiu/KuraDB/internal/openai"
	"github.com/pardnchiu/KuraDB/internal/segmenter"
	"github.com/pardnchiu/KuraDB/internal/vector"
)

func Router(dbName string, db *database.DB, cache *vector.Cache, embedder openai.Embedder, qCache *openai.Cache, seg *segmenter.Segmenter) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(gin.Recovery())

	api := r.Group("/api")
	api.GET("/health", Health(cache))
	api.GET("/semantic", requireDB(dbName), Semantic(db, cache, embedder, qCache))
	api.GET("/keyword", requireDB(dbName), Keyword(db, seg))

	return r
}

func requireDB(running string) gin.HandlerFunc {
	return func(c *gin.Context) {
		got := c.Query("db")
		if got == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "db is required"})
			return
		}
		if got != running {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("db mismatch: server runs %q, request asked %q", running, got),
			})
			return
		}
		c.Next()
	}
}
