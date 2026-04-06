package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func GetHealthDashboard(c *gin.Context) {
	data, cacheHit, err := service.FetchHealthDashboard(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	common.ApiSuccess(c, gin.H{
		"dashboard": data,
		"cache_hit": cacheHit,
	})
}
