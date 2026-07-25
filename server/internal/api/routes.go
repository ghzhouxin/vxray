package api

import (
	"v2ray-server/internal/service"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine, services *service.Container) {
	consoleHandler := NewConsoleHandler(services)
	subHandler := NewSubscriptionHandler(services)
	nodeHandler := NewNodeHandler(services)
	xrayHandler := NewXrayHandler(services)
	proxyHandler := NewProxyHandler(services)
	geoHandler := NewGeoHandler(services)
	configHandler := NewConfigHandler(services)
	logHandler := NewLogHandler(services)
	clashHandler := NewClashHandler(services)

	api := r.Group("/api")
	{
		api.GET("/console", consoleHandler.Get)

		subs := api.Group("/subscriptions")
		{
			subs.GET("", subHandler.List)
			subs.POST("", subHandler.Create)
			subs.PUT("/:id", subHandler.Update)
			subs.DELETE("/:id", subHandler.Delete)
			subs.POST("/refresh", subHandler.RefreshNodes)
		}

		nodes := api.Group("/nodes")
		{
			nodes.GET("", nodeHandler.List)
			nodes.GET("/speedtest/status", nodeHandler.SpeedTestStatus)
			nodes.POST("/speedtest", nodeHandler.SpeedTestByFilter)
			nodes.POST("/:id/activate", nodeHandler.SetActive)
			nodes.DELETE("/:id", nodeHandler.Delete)
			nodes.POST("/delete-failed", nodeHandler.DeleteFailed)
		}

		xray := api.Group("/xray")
		{
			xray.GET("/runtime", xrayHandler.Runtime)
			xray.POST("/runtime/start", xrayHandler.StartRuntime)
			xray.POST("/runtime/stop", xrayHandler.StopRuntime)
			xray.GET("/config", xrayHandler.GetConfig)
			xray.GET("/config/default", xrayHandler.GetDefaultConfig)
			xray.PUT("/config", xrayHandler.SaveConfig)
			xray.POST("/speedtest/websites", xrayHandler.SpeedTestWebsites)
		}

		proxy := api.Group("/proxy")
		{
			proxy.POST("/toggle", proxyHandler.Toggle)
		}

		geo := api.Group("/geo")
		{
			geo.GET("/status", geoHandler.GetStatus)
			geo.POST("/download/all", geoHandler.DownloadAll)
		}

		settings := api.Group("/settings")
		{
			settings.GET("", configHandler.Get)
			settings.PUT("", configHandler.Update)
			settings.GET("/default", configHandler.GetDefault)
		}

		logs := api.Group("/logs")
		{
			logs.GET("", logHandler.List)
			logs.DELETE("", logHandler.Clear)
		}

		api.GET("/clash", clashHandler.Subscription)
	}
}
