package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func GetLogsDashboard(c *gin.Context) {
	window := c.DefaultQuery("window", service.LogDashboardWindow1h)
	data, err := service.GetLogDashboard(c.Request.Context(), window)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    data,
	})
}
