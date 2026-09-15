package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	apiHandler "github.com/agenvoy/kuradb/internal/api/handler"
	"github.com/agenvoy/kuradb/internal/database"
	"github.com/agenvoy/kuradb/internal/mcp"
	"github.com/agenvoy/kuradb/internal/openai"
)

func Router(reg *database.Registry, dbs map[string]*database.DB, embedder openai.Embedder, qCache *openai.Cache, remote bool) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	router := gin.New()
	router.Use(gin.Recovery())

	if remote {
		router.Any("/mcp", gin.WrapH(mcp.Handler(reg, dbs, embedder, qCache)))
	}

	api := router.Group("/api")
	api.GET("/health", apiHandler.Health())
	api.GET("/list", apiHandler.List(reg, dbs))
	api.GET("/search", queryDB(dbs), withTarget(""), apiHandler.Search(dbs, embedder, qCache))

	// * will deprecate in v1.*.*, keep this for ensuring Agenvoy won't break.
	api.GET("/semantic", queryDB(dbs), withTarget("semantic"), apiHandler.Search(dbs, embedder, qCache))
	api.GET("/keyword", queryDB(dbs), withTarget("keyword"), apiHandler.Search(dbs, embedder, qCache))

	return router
}

func queryDB(dbs map[string]*database.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		db := c.Query("db")
		if db == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "db is required",
			})
			return
		}
		if _, ok := dbs[db]; !ok {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("%q not exist", db),
			})
			return
		}

		c.Set("db", db)
		c.Next()
	}
}

func withTarget(target string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if target == "" {
			target = c.Query("target")
		}
		c.Set("target", target)
		c.Next()
	}
}
