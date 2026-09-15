package apiHandler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/pardnchiu/kuradb/internal/database"
	"github.com/pardnchiu/kuradb/internal/openai"
	"github.com/pardnchiu/kuradb/internal/search"
)

func Search(dbs map[string]*database.DB, embedder openai.Embedder, qCache *openai.Cache) gin.HandlerFunc {
	return func(c *gin.Context) {
		dic, err := search.Search(c.Request.Context(), dbs, embedder, qCache,
			c.GetString("db"), c.Query("q"), c.GetString("target"), queryLimit(c))
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, search.ErrInvalidArgument) {
				status = http.StatusBadRequest
			}
			c.JSON(status, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, dic)
	}
}
