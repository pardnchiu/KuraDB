package apiHandler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/pardnchiu/kuradb/internal/database"
)

func List(reg *database.Registry, dbs map[string]*database.DB) gin.HandlerFunc {
	loaded := make([]string, 0, len(dbs))
	for name := range dbs {
		loaded = append(loaded, name)
	}
	return func(c *gin.Context) {
		entries, err := reg.Load()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if entries == nil {
			entries = []database.Entry{}
		}
		c.JSON(http.StatusOK, gin.H{
			"loaded":     loaded,
			"registered": entries,
		})
	}
}
