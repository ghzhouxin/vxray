package api

import (
	"github.com/gin-gonic/gin"
	"v2ray-server/internal/constants"
	"v2ray-server/internal/service"
	"v2ray-server/pkg/response"
)

type GeoHandler struct {
	services *service.Container
	logger   *service.TaggedLogger
}

func NewGeoHandler(services *service.Container) *GeoHandler {
	return &GeoHandler{
		services: services,
		logger:   services.Log.NewTaggedLogger(constants.TagGeo),
	}
}

func (h *GeoHandler) GetStatus(c *gin.Context) {
	response.Success(c, h.services.Geo.GetFileInfo())
}

func (h *GeoHandler) DownloadAll(c *gin.Context) {
	h.download(c, "all geo files")
}

func (h *GeoHandler) download(c *gin.Context, name string) {
	h.logger.Info("开始更新 Geo 文件", map[string]any{"target": name})
	if handleError(c, h.services.Geo.DownloadAll(c.Request.Context())) {
		h.logger.Error("更新 Geo 文件失败", map[string]any{"target": name})
		return
	}
	h.logger.Info("Geo 文件已更新", map[string]any{"target": name})
	response.SuccessMessage(c, name+" downloaded successfully")
}
