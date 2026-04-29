package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/pardnchiu/AgenvoyRAG/internal/vector"
)

func Health(cache *vector.Cache) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":     "ok",
			"cache_size": cache.Len(),
		})
	}
}
