package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func GetRequestDumpStatus(c *gin.Context) {
	common.ApiSuccess(c, service.GetRequestDumpStatus())
}

func StartRequestDump(c *gin.Context) {
	rule := service.DefaultRequestDumpRule()
	if err := common.DecodeJson(c.Request.Body, &rule); err != nil {
		common.ApiError(c, err)
		return
	}
	status, err := service.StartRequestDump(rule)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, status)
}

func StopRequestDump(c *gin.Context) {
	common.ApiSuccess(c, service.StopRequestDump())
}

func ClearRequestDumpEvents(c *gin.Context) {
	common.ApiSuccess(c, service.ClearRequestDumpEvents())
}

func GetRequestDumpEvents(c *gin.Context) {
	afterID, limit := service.ParseRequestDumpEventQuery(c.Query("after_id"), c.Query("limit"))
	common.ApiSuccess(c, gin.H{
		"events": service.GetRequestDumpEvents(afterID, limit),
		"status": service.GetRequestDumpStatus(),
	})
}
