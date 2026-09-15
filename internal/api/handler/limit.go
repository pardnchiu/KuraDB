package apiHandler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/pardnchiu/kuradb/internal/search"
)

func queryLimit(c *gin.Context) int {
	raw := c.Query("limit")
	if raw == "" {
		return search.DefaultLimit
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 || v > search.MaxLimit {
		return search.DefaultLimit
	}
	return v
}
