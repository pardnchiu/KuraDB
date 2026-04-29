package api

import (
	"github.com/gin-gonic/gin"

	"github.com/pardnchiu/AgenvoyRAG/internal/database"
	"github.com/pardnchiu/AgenvoyRAG/internal/openai"
	"github.com/pardnchiu/AgenvoyRAG/internal/segmenter"
	"github.com/pardnchiu/AgenvoyRAG/internal/vector"
)

func Router(db *database.DB, cache *vector.Cache, embedder openai.Embedder, qCache *openai.Cache, seg *segmenter.Segmenter) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(gin.Recovery())

	api := r.Group("/api")
	api.GET("/health", Health(cache))
	api.GET("/semantic", Semantic(db, cache, embedder, qCache))
	api.GET("/keyword", Keyword(db, seg))

	return r
}
