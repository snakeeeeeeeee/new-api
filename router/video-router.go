package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"

	"github.com/gin-gonic/gin"
)

func SetVideoRouter(router *gin.Engine) {
	assetV1Router := router.Group("/v1")
	assetV1Router.Use(middleware.RouteTag("relay"))
	assetV1Router.Use(middleware.AssetKeyAuth())
	{
		assetV1Router.GET("/assets", controller.ListAssetsByAPIKey)
		assetV1Router.POST("/assets/query", controller.QueryAssetsByAPIKey)
		assetV1Router.POST("/assets/batch/urls", controller.GetAssetBatchURLsByAPIKey)
		assetV1Router.GET("/assets/export", controller.ExportAssetsByAPIKey)
		assetV1Router.GET("/assets/:asset_id", controller.GetAssetByAPIKey)
	}

	// Video proxy: accepts either session auth (dashboard) or token auth (API clients)
	videoProxyRouter := router.Group("/v1")
	videoProxyRouter.Use(middleware.RouteTag("relay"))
	videoProxyRouter.Use(middleware.TokenOrUserAuth())
	{
		videoProxyRouter.GET("/videos/:task_id/content", controller.VideoProxy)
		videoProxyRouter.GET("/assets/:asset_id/content", controller.VideoAssetContent)
	}

	imageTaskQueryRouter := router.Group("/v1")
	imageTaskQueryRouter.Use(middleware.RouteTag("relay"))
	imageTaskQueryRouter.Use(middleware.AssetKeyAuth())
	{
		imageTaskQueryRouter.GET("/image/tasks", controller.ListImageTasks)
		imageTaskQueryRouter.GET("/image/tasks/:task_id", controller.GetImageTask)
		imageTaskQueryRouter.POST("/image/tasks/query", controller.QueryImageTasks)
		imageTaskQueryRouter.POST("/image/uploads", controller.ProxyImageTaskUpload)
		imageTaskQueryRouter.POST("/image/uploads/base64", controller.ProxyImageTaskUpload)
	}

	videoTaskQueryRouter := router.Group("/v1")
	videoTaskQueryRouter.Use(middleware.RouteTag("relay"))
	videoTaskQueryRouter.Use(middleware.AssetKeyAuth())
	{
		videoTaskQueryRouter.GET("/video/tasks", controller.ListVideoTasks)
		videoTaskQueryRouter.GET("/video/tasks/:task_id", controller.GetVideoTask)
		videoTaskQueryRouter.POST("/video/tasks/query", controller.QueryVideoTasks)
	}

	videoCreateRouter := router.Group("/v1")
	videoCreateRouter.Use(middleware.RouteTag("relay"))
	videoCreateRouter.Use(middleware.TokenAuth(), middleware.Distribute())
	{
		videoCreateRouter.POST("/video/tasks", controller.PrepareVideoTaskRequest, controller.RelayTask)
		videoCreateRouter.POST("/video/generations", controller.RelayTask)
		videoCreateRouter.POST("/videos/:video_id/remix", controller.RelayTask)
		videoCreateRouter.POST("/videos/generations", controller.RelayTask)
		videoCreateRouter.POST("/videos/edits", controller.RelayTask)
		videoCreateRouter.POST("/videos/extensions", controller.RelayTask)
		videoCreateRouter.POST("/videos", controller.RelayTask)
	}

	videoQueryRouter := router.Group("/v1")
	videoQueryRouter.Use(middleware.RouteTag("relay"))
	videoQueryRouter.Use(middleware.AssetOrTokenAuth(), middleware.Distribute())
	{
		videoQueryRouter.GET("/video/generations/:task_id", controller.RelayTaskFetch)
		videoQueryRouter.GET("/videos/:task_id", controller.RelayTaskFetch)
	}

	// openai compatible API video routes
	// docs: https://platform.openai.com/docs/api-reference/videos/create

	imageTaskCreateRouter := router.Group("/v1")
	imageTaskCreateRouter.Use(middleware.RouteTag("relay"))
	imageTaskCreateRouter.Use(middleware.TokenAuth(), middleware.Distribute())
	{
		imageTaskCreateRouter.POST("/image/tasks", controller.PrepareImageTaskRequest, controller.RelayTask)
	}

	klingV1Router := router.Group("/kling/v1")
	klingV1Router.Use(middleware.RouteTag("relay"))
	klingV1Router.Use(middleware.KlingRequestConvert(), middleware.TokenAuth(), middleware.Distribute())
	{
		klingV1Router.POST("/videos/text2video", controller.RelayTask)
		klingV1Router.POST("/videos/image2video", controller.RelayTask)
		klingV1Router.GET("/videos/text2video/:task_id", controller.RelayTaskFetch)
		klingV1Router.GET("/videos/image2video/:task_id", controller.RelayTaskFetch)
	}

	// Jimeng official API routes - direct mapping to official API format
	jimengOfficialGroup := router.Group("jimeng")
	jimengOfficialGroup.Use(middleware.RouteTag("relay"))
	jimengOfficialGroup.Use(middleware.JimengRequestConvert(), middleware.TokenAuth(), middleware.Distribute())
	{
		// Maps to: /?Action=CVSync2AsyncSubmitTask&Version=2022-08-31 and /?Action=CVSync2AsyncGetResult&Version=2022-08-31
		jimengOfficialGroup.POST("/", controller.RelayTask)
	}
}
